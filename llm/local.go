package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	localIdleTimeout    = 5 * time.Minute
	localStartupTimeout = 60 * time.Second
)

// LocalClient implements the Client interface using a managed llama-server
// subprocess. It starts the subprocess on first request, delegates to the
// OpenAI-compatible /v1/chat/completions endpoint, and auto-kills after
// 5 minutes of idle.
type LocalClient struct {
	mu         sync.Mutex
	modelPath  string // absolute path to GGUF file
	modelName  string // display name (e.g. "qwen3-0.6b")
	serverPath string // path to llama-server binary
	port       int
	cmd        *exec.Cmd
	idleTimer  *time.Timer
	httpClient *http.Client
	maxTokens  int
	timeoutMs  int
}

// newLocalFromDef creates a LocalClient from a provider definition.
func newLocalFromDef(def LLMProviderDefCompat) (*LocalClient, error) {
	maxTokens := def.MaxTokens
	if maxTokens == 0 {
		maxTokens = 256
	}
	timeoutMs := def.TimeoutMs
	if timeoutMs == 0 {
		timeoutMs = 120000 // local models need time for cold start
	}

	// Resolve model name to GGUF file path.
	modelPath, err := resolveLocalModel(def.Model)
	if err != nil {
		return nil, fmt.Errorf("local model %q: %w", def.Model, err)
	}

	serverPath, err := LlamaServerPath()
	if err != nil {
		return nil, fmt.Errorf("llama-server: %w", err)
	}

	return &LocalClient{
		modelPath:  modelPath,
		modelName:  def.Model,
		serverPath: serverPath,
		maxTokens:  maxTokens,
		timeoutMs:  timeoutMs,
		httpClient: newPooledHTTPClient(),
	}, nil
}

// LLMProviderDefCompat mirrors config.LLMProviderDef fields needed here,
// avoiding a circular import (config → llm would be fine, but llm → config
// is already established; we just accept the fields we need).
type LLMProviderDefCompat struct {
	Model     string
	MaxTokens int
	TimeoutMs int
}

func (c *LocalClient) Provider() string { return "local" }

func (c *LocalClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stopLocked()
}

func (c *LocalClient) Send(ctx context.Context, req Request) (*Response, error) {
	if err := c.ensureRunning(); err != nil {
		return nil, fmt.Errorf("local LLM not ready: %w", err)
	}

	c.resetIdleTimer()

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = c.maxTokens
	}

	endpoint := fmt.Sprintf("http://127.0.0.1:%d/v1/chat/completions", c.port)

	body := openaiRequest{
		Model: c.modelName,
		Messages: []openaiMessage{
			{Role: "system", Content: "/no_think\n" + req.Prompt},
			{Role: "user", Content: req.Text},
		},
		MaxTokens: maxTokens,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	slog.Debug("[local] sending request", "model", c.modelName, "port", c.port, "body_len", len(jsonBody))

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := c.httpClient.Do(httpReq)
	elapsed := time.Since(start)
	if err != nil {
		slog.Error("[local] request failed", "elapsed", elapsed, "error", err)
		return nil, fmt.Errorf("local LLM request failed: %w", err)
	}
	defer resp.Body.Close()

	slog.Debug("[local] response received", "status", resp.StatusCode, "elapsed", elapsed)

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		slog.Error("[local] HTTP error", "status", resp.StatusCode, "body", string(respBody))
		return nil, fmt.Errorf("local LLM returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var apiResp openaiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if apiResp.Error != nil {
		return nil, fmt.Errorf("local LLM error (%s): %s", apiResp.Error.Type, apiResp.Error.Message)
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("empty response from local LLM")
	}

	text := stripThinkingTags(apiResp.Choices[0].Message.Content)
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("local LLM returned empty content")
	}

	slog.Info("[local] inference complete", "elapsed", elapsed, "text_len", len(text))

	return &Response{
		Text:     text,
		Provider: "local",
		Model:    c.modelName,
	}, nil
}

// ensureRunning starts the llama-server subprocess if it's not already running.
func (c *LocalClient) ensureRunning() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if already running and healthy.
	if c.cmd != nil && c.cmd.Process != nil {
		if c.healthCheckLocked() {
			return nil
		}
		// Process exists but not healthy — clean up and restart.
		slog.Warn("[local] subprocess unhealthy, restarting")
		c.stopLocked()
	}

	// Find a free port.
	port, err := freePort()
	if err != nil {
		return fmt.Errorf("find free port: %w", err)
	}
	c.port = port

	// Build llama-server command.
	args := []string{
		"--model", c.modelPath,
		"--port", fmt.Sprintf("%d", port),
		"--host", "127.0.0.1",
		"--ctx-size", "2048",
		"--n-gpu-layers", "-1", // auto: offload everything to GPU if available
		"--log-disable",
	}

	slog.Info("[local] starting llama-server", "path", c.serverPath, "model", c.modelPath, "port", port)
	fmt.Printf("[local] Starting llama-server on port %d...\n", port)

	cmd := exec.Command(c.serverPath, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start llama-server: %w", err)
	}
	c.cmd = cmd

	// Monitor subprocess exit in background.
	go func() {
		if err := cmd.Wait(); err != nil {
			slog.Debug("[local] llama-server exited", "error", err)
		}
	}()

	// Wait for the server to become healthy.
	if err := c.waitForHealthy(localStartupTimeout); err != nil {
		c.stopLocked()
		return fmt.Errorf("llama-server failed to start: %w", err)
	}

	slog.Info("[local] llama-server ready", "port", port, "model", c.modelName)
	fmt.Printf("[local] llama-server ready on port %d\n", port)

	// Start idle timer.
	c.idleTimer = time.AfterFunc(localIdleTimeout, func() {
		slog.Info("[local] idle timeout reached, shutting down llama-server")
		fmt.Println("[local] Idle timeout — shutting down llama-server")
		c.mu.Lock()
		c.stopLocked()
		c.mu.Unlock()
	})

	return nil
}

// waitForHealthy polls the /health endpoint until it responds OK or timeout.
func (c *LocalClient) waitForHealthy(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	url := fmt.Sprintf("http://127.0.0.1:%d/health", c.port)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("timeout after %s waiting for llama-server health", timeout)
}

// healthCheckLocked checks if the subprocess is still responding.
// Caller must hold c.mu.
func (c *LocalClient) healthCheckLocked() bool {
	if c.port == 0 {
		return false
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/health", c.port)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// stopLocked kills the subprocess and resets state. Caller must hold c.mu.
func (c *LocalClient) stopLocked() {
	if c.idleTimer != nil {
		c.idleTimer.Stop()
		c.idleTimer = nil
	}
	if c.cmd != nil && c.cmd.Process != nil {
		slog.Info("[local] killing llama-server", "pid", c.cmd.Process.Pid)
		c.cmd.Process.Kill()
		c.cmd = nil
	}
	c.port = 0
}

// resetIdleTimer resets the 5-minute idle shutdown timer.
func (c *LocalClient) resetIdleTimer() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.idleTimer != nil {
		c.idleTimer.Reset(localIdleTimeout)
	}
}

// freePort asks the OS for a free TCP port.
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port, nil
}

// resolveLocalModel maps a friendly model name to the GGUF file path.
func resolveLocalModel(name string) (string, error) {
	modelsDir, err := LocalModelsDir()
	if err != nil {
		return "", err
	}

	// Map friendly names to filenames.
	for _, m := range AvailableLocalModels() {
		if m.Name == name {
			path := filepath.Join(modelsDir, m.FileName)
			if _, err := os.Stat(path); err != nil {
				return "", fmt.Errorf("model file not found: %s (download it first in Settings)", path)
			}
			return path, nil
		}
	}

	// Assume it's already a filename.
	path := filepath.Join(modelsDir, name)
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("model file not found: %s", path)
	}
	return path, nil
}

// LocalModelsDir returns the path to the local models directory.
func LocalModelsDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "GhostSpell", "models")
	os.MkdirAll(dir, 0755)
	return dir, nil
}

// localBinDir returns the path to the local binaries directory.
func localBinDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "GhostSpell", "bin")
	os.MkdirAll(dir, 0755)
	return dir, nil
}

// LlamaServerPath returns the path to the llama-server binary.
func LlamaServerPath() (string, error) {
	dir, err := localBinDir()
	if err != nil {
		return "", err
	}
	name := "llama-server"
	if runtime.GOOS == "windows" {
		name = "llama-server.exe"
	}
	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("llama-server not found at %s (download it first in Settings)", path)
	}
	return path, nil
}
