package stt

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	// voiceIdleTimeout is how long the daemon stays alive after the last
	// transcription when keep_alive is false. Matches Ghost-AI's pattern.
	voiceIdleTimeout = 5 * time.Minute
)

// GhostVoiceClient implements Transcriber using the ghostvoice helper binary.
// Supports two modes:
//   - Daemon (default): ghostvoice stays running with model loaded. Subsequent
//     transcriptions skip the model load (~500ms-2s savings).
//   - When keep_alive=false, the daemon auto-exits after 5 minutes of idle.
//   - When keep_alive=true, the daemon stays alive until the app exits.
type GhostVoiceClient struct {
	modelPath string
	modelName string
	cliPath   string
	keepAlive bool

	mu        sync.Mutex
	proc      *exec.Cmd
	stdin     io.WriteCloser
	stdout    *bufio.Scanner
	running   bool
	idleTimer *time.Timer
}

// NewGhostVoiceClient creates a local STT client.
func NewGhostVoiceClient(modelName, modelsDir string, keepAlive bool) (*GhostVoiceClient, error) {
	model := findVoiceModel(modelName)
	if model == nil {
		return nil, fmt.Errorf("ghost-voice: unknown model %q", modelName)
	}

	modelPath := filepath.Join(modelsDir, model.FileName)
	fi, err := os.Stat(modelPath)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("ghost-voice: model file not found: %s (download it first in Settings)", modelPath)
	}
	if err != nil {
		return nil, fmt.Errorf("ghost-voice: cannot stat model file: %w", err)
	}
	fileSizeMB := fi.Size() / (1024 * 1024)
	expectedMB := model.Size / (1024 * 1024)
	slog.Info("[ghost-voice] model file", "path", modelPath, "size_mb", fileSizeMB, "expected_mb", expectedMB)
	if fileSizeMB < expectedMB/2 {
		return nil, fmt.Errorf("ghost-voice: model file appears truncated (%dMB, expected ~%dMB) — re-download in Settings", fileSizeMB, expectedMB)
	}

	cliPath, err := findGhostVoice()
	if err != nil {
		return nil, err
	}

	mode := "daemon"
	if !keepAlive {
		mode = "on-demand"
	}
	slog.Info("[ghost-voice] client ready", "model", modelName, "helper", cliPath, "mode", mode)
	return &GhostVoiceClient{
		modelPath: modelPath,
		modelName: modelName,
		cliPath:   cliPath,
		keepAlive: keepAlive,
	}, nil
}

func (c *GhostVoiceClient) Name() string           { return "Ghost Voice" }
func (c *GhostVoiceClient) SupportsStreaming() bool { return false }

func (c *GhostVoiceClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stopDaemonLocked()
}

// ensureDaemon starts the ghostvoice daemon if not running, and resets the idle timer.
func (c *GhostVoiceClient) ensureDaemon() error {
	if c.running {
		// Reset idle timer on each use.
		if c.idleTimer != nil {
			c.idleTimer.Reset(voiceIdleTimeout)
		}
		return nil
	}

	slog.Info("[ghost-voice] starting daemon", "model", c.modelName)
	cmd := exec.Command(c.cliPath, "--daemon", "-m", c.modelPath, "-t", "4")
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("ghost-voice: stdin pipe: %w", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return fmt.Errorf("ghost-voice: stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		stdin.Close()
		return fmt.Errorf("ghost-voice: start daemon: %w", err)
	}

	scanner := bufio.NewScanner(stdoutPipe)
	scanner.Buffer(make([]byte, 256*1024), 256*1024) // 256KB line buffer

	// Wait for {"ready":true} response.
	if !scanner.Scan() {
		cmd.Process.Kill()
		return fmt.Errorf("ghost-voice: daemon failed to start (no ready response)")
	}
	ready := scanner.Text()
	if !strings.Contains(ready, `"ready":true`) {
		cmd.Process.Kill()
		return fmt.Errorf("ghost-voice: daemon failed: %s", ready)
	}

	c.proc = cmd
	c.stdin = stdin
	c.stdout = scanner
	c.running = true

	// Start idle timer (unless keep-alive).
	if !c.keepAlive {
		c.idleTimer = time.AfterFunc(voiceIdleTimeout, func() {
			c.mu.Lock()
			defer c.mu.Unlock()
			if c.running {
				slog.Info("[ghost-voice] idle timeout — stopping daemon")
				c.stopDaemonLocked()
			}
		})
	}

	slog.Info("[ghost-voice] daemon started (model loaded)", "model", c.modelName)
	return nil
}

func (c *GhostVoiceClient) stopDaemonLocked() {
	if !c.running {
		return
	}

	if c.idleTimer != nil {
		c.idleTimer.Stop()
		c.idleTimer = nil
	}

	// Send quit command.
	fmt.Fprintf(c.stdin, "{\"quit\":true}\n")
	c.stdin.Close()

	// Wait for process to exit (with timeout).
	done := make(chan struct{})
	go func() {
		c.proc.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		c.proc.Process.Kill()
	}

	c.running = false
	c.proc = nil
	c.stdin = nil
	c.stdout = nil
	slog.Info("[ghost-voice] daemon stopped")
}

// daemonRequest is the JSON command sent to ghostvoice stdin.
type daemonRequest struct {
	File string `json:"file"`
	Lang string `json:"lang,omitempty"`
}

// daemonResponse is the JSON response from ghostvoice stdout.
type daemonResponse struct {
	Text  string `json:"text,omitempty"`
	Error string `json:"error,omitempty"`
}

func (c *GhostVoiceClient) Transcribe(ctx context.Context, wavData []byte, language string) (string, error) {
	// Write WAV to temp file.
	tmp, err := os.CreateTemp("", "ghostvoice-*.wav")
	if err != nil {
		return "", fmt.Errorf("ghost-voice: temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(wavData); err != nil {
		tmp.Close()
		return "", fmt.Errorf("ghost-voice: write temp: %w", err)
	}
	tmp.Close()

	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.ensureDaemon(); err != nil {
		return "", err
	}

	slog.Info("[ghost-voice] transcribing via daemon", "model", c.modelName, "wav_bytes", len(wavData))

	// Send transcribe command.
	req := daemonRequest{File: tmpPath, Lang: language}
	reqBytes, _ := json.Marshal(req)
	if _, err := fmt.Fprintf(c.stdin, "%s\n", reqBytes); err != nil {
		// Daemon died — stop and retry once.
		slog.Warn("[ghost-voice] daemon write failed, restarting", "error", err)
		c.stopDaemonLocked()
		if err := c.ensureDaemon(); err != nil {
			return "", err
		}
		if _, err := fmt.Fprintf(c.stdin, "%s\n", reqBytes); err != nil {
			c.stopDaemonLocked()
			return "", fmt.Errorf("ghost-voice: daemon write: %w", err)
		}
	}

	// Read response (with context cancellation).
	type result struct {
		resp daemonResponse
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		if c.stdout.Scan() {
			var resp daemonResponse
			if err := json.Unmarshal([]byte(c.stdout.Text()), &resp); err != nil {
				ch <- result{err: fmt.Errorf("ghost-voice: parse response: %w", err)}
			} else {
				ch <- result{resp: resp}
			}
		} else {
			ch <- result{err: fmt.Errorf("ghost-voice: daemon closed unexpectedly")}
		}
	}()

	select {
	case <-ctx.Done():
		// Context cancelled — kill daemon to abort transcription.
		c.stopDaemonLocked()
		return "", ctx.Err()
	case r := <-ch:
		if r.err != nil {
			c.stopDaemonLocked()
			return "", r.err
		}
		if r.resp.Error != "" {
			return "", fmt.Errorf("ghost-voice: %s", r.resp.Error)
		}
		text := strings.TrimSpace(r.resp.Text)
		slog.Info("[ghost-voice] transcription complete", "text_len", len(text), "text", text)
		return text, nil
	}
}

// findGhostVoice locates the ghostvoice helper binary.
func findGhostVoice() (string, error) {
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}

	// Check multiple possible names next to the main executable.
	names := []string{
		"ghostvoice" + ext,
		fmt.Sprintf("ghostvoice-%s-%s%s", runtime.GOOS, runtime.GOARCH, ext),
	}

	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		for _, name := range names {
			path := filepath.Join(dir, name)
			if _, err := os.Stat(path); err == nil {
				return path, nil
			}
		}
	}

	// Check PATH.
	for _, name := range names {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("ghost-voice: ghostvoice%s not found — run _build.bat to build it", ext)
}

// VoiceModel describes a downloadable whisper model.
type VoiceModel struct {
	Name     string
	FileName string
	URL      string
	Size     int64
	Tag      string
	Desc     string
}

// AvailableVoiceModels returns the curated list of downloadable whisper models.
func AvailableVoiceModels() []VoiceModel {
	return []VoiceModel{
		{
			Name:     "whisper-tiny",
			FileName: "ggml-tiny.bin",
			URL:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-tiny.bin",
			Size:     75_000_000,
			Tag:      "fast",
			Desc:     "Fastest, English-focused. 39M params. Good for quick dictation.",
		},
		{
			Name:     "whisper-base",
			FileName: "ggml-base.bin",
			URL:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.bin",
			Size:     142_000_000,
			Tag:      "recommended",
			Desc:     "Good balance of speed and accuracy. 74M params. Supports 99 languages.",
		},
		{
			Name:     "whisper-small",
			FileName: "ggml-small.bin",
			URL:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.bin",
			Size:     466_000_000,
			Tag:      "best",
			Desc:     "High accuracy, 244M params. Great for multilingual use.",
		},
		{
			Name:     "whisper-medium",
			FileName: "ggml-medium.bin",
			URL:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-medium.bin",
			Size:     1_500_000_000,
			Tag:      "heavy",
			Desc:     "High accuracy, 769M params. Needs 2GB+ RAM.",
		},
		{
			Name:     "whisper-large-v3-turbo",
			FileName: "ggml-large-v3-turbo.bin",
			URL:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-large-v3-turbo.bin",
			Size:     1_600_000_000,
			Tag:      "best",
			Desc:     "Best accuracy + speed. Turbo variant of large-v3. 809M params.",
		},
		{
			Name:     "whisper-large-v3",
			FileName: "ggml-large-v3.bin",
			URL:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-large-v3.bin",
			Size:     3_100_000_000,
			Tag:      "heavy",
			Desc:     "Maximum accuracy, 1.55B params. Needs 4GB+ RAM. Slowest.",
		},
	}
}

func findVoiceModel(name string) *VoiceModel {
	for _, m := range AvailableVoiceModels() {
		if m.Name == name {
			return &m
		}
	}
	return nil
}
