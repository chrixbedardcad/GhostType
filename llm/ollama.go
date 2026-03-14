package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/chrixbedardcad/GhostSpell/config"
)

const defaultOllamaEndpoint = "http://localhost:11434/api/generate"

// OllamaClient implements the Client interface for local Ollama.
type OllamaClient struct {
	model      string
	endpoint   string
	maxTokens  int
	timeoutMs  int
	httpClient *http.Client
}

// NewOllamaClient creates a new Ollama client from config.
func NewOllamaClient(cfg *config.Config) *OllamaClient {
	endpoint := normalizeOllamaEndpoint(cfg.APIEndpoint)

	return &OllamaClient{
		model:      cfg.Model,
		endpoint:   endpoint,
		maxTokens:  cfg.MaxTokens,
		timeoutMs:  cfg.TimeoutMs,
		httpClient: newPooledHTTPClient(cfg.TimeoutMs),
	}
}

// normalizeOllamaEndpoint ensures the endpoint includes the /api/generate path.
// Users often enter just the base URL (e.g. http://localhost:11434).
func normalizeOllamaEndpoint(endpoint string) string {
	if endpoint == "" {
		slog.Debug("[ollama] endpoint empty, using default", "endpoint", defaultOllamaEndpoint)
		fmt.Printf("[ollama] endpoint empty → using default: %s\n", defaultOllamaEndpoint)
		return defaultOllamaEndpoint
	}
	original := endpoint
	endpoint = strings.TrimRight(endpoint, "/")
	if !strings.HasSuffix(endpoint, "/api/generate") {
		endpoint += "/api/generate"
	}
	if original != endpoint {
		slog.Info("[ollama] endpoint normalized", "original", original, "normalized", endpoint)
		fmt.Printf("[ollama] endpoint normalized: %q → %q\n", original, endpoint)
	}
	return endpoint
}

// newOllamaFromDef creates a new Ollama client from a provider definition.
func newOllamaFromDef(def config.LLMProviderDef) *OllamaClient {
	slog.Info("[ollama] newOllamaFromDef", "model", def.Model, "raw_endpoint", def.APIEndpoint, "timeout_ms", def.TimeoutMs)
	fmt.Printf("[ollama] Creating client: model=%s endpoint=%q timeout=%dms\n", def.Model, def.APIEndpoint, def.TimeoutMs)
	endpoint := normalizeOllamaEndpoint(def.APIEndpoint)
	maxTokens := def.MaxTokens
	if maxTokens == 0 {
		maxTokens = 256
	}
	timeoutMs := def.TimeoutMs
	if timeoutMs == 0 {
		timeoutMs = 30000
	}

	return &OllamaClient{
		model:      def.Model,
		endpoint:   endpoint,
		maxTokens:  maxTokens,
		timeoutMs:  timeoutMs,
		httpClient: newPooledHTTPClient(timeoutMs),
	}
}

func (c *OllamaClient) Provider() string {
	return "ollama"
}

func (c *OllamaClient) Close() {
	c.httpClient.CloseIdleConnections()
}

// ollamaRequest is the request body for the Ollama generate API.
type ollamaRequest struct {
	Model   string         `json:"model"`
	System  string         `json:"system,omitempty"`
	Prompt  string         `json:"prompt"`
	Stream  bool           `json:"stream"`
	Options ollamaOptions  `json:"options,omitempty"`
}

// ollamaOptions maps to Ollama's model options (num_predict, etc.).
type ollamaOptions struct {
	NumPredict int `json:"num_predict,omitempty"`
}

// ollamaResponse is the response body from the Ollama generate API.
type ollamaResponse struct {
	Response string `json:"response"`
	Error    string `json:"error,omitempty"`
}

func (c *OllamaClient) Send(ctx context.Context, req Request) (*Response, error) {
	maxTok := c.maxTokens
	if req.MaxTokens > 0 {
		maxTok = req.MaxTokens
	}

	// Use the system field for instructions, user text as prompt.
	// Prepend /no_think to suppress chain-of-thought on thinking models
	// (qwen3, deepseek-r1, etc.).
	body := ollamaRequest{
		Model:  c.model,
		System: "/no_think\n" + req.Prompt,
		Prompt: req.Text,
		Stream: false,
		Options: ollamaOptions{
			NumPredict: maxTok,
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	slog.Info("[ollama] sending request", "model", c.model, "endpoint", c.endpoint, "body_len", len(jsonBody))
	fmt.Printf("[ollama] POST %s model=%s body=%d bytes\n", c.endpoint, c.model, len(jsonBody))

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := c.httpClient.Do(httpReq)
	elapsed := time.Since(start)
	if err != nil {
		slog.Error("[ollama] request failed", "endpoint", c.endpoint, "elapsed", elapsed, "error", err)
		fmt.Printf("[ollama] FAILED after %s: %v\n", elapsed, err)
		return nil, fmt.Errorf("API request failed (is Ollama running at %s?): %w", c.endpoint, err)
	}
	defer resp.Body.Close()

	slog.Info("[ollama] response received", "status", resp.StatusCode, "elapsed", elapsed)
	fmt.Printf("[ollama] Response: status=%d elapsed=%s\n", resp.StatusCode, elapsed)

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		slog.Error("[ollama] HTTP error", "status", resp.StatusCode, "body", string(respBody))
		fmt.Printf("[ollama] HTTP ERROR %d: %s\n", resp.StatusCode, string(respBody))
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var apiResp ollamaResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if apiResp.Error != "" {
		return nil, fmt.Errorf("Ollama error: %s", apiResp.Error)
	}

	// Strip <think>...</think> blocks from thinking models (qwen3, deepseek-r1)
	// that may still emit them despite /no_think.
	text := stripThinkingTags(apiResp.Response)

	if text == "" {
		return nil, fmt.Errorf("empty response from Ollama")
	}

	return &Response{
		Text:     text,
		Provider: "ollama",
		Model:    c.model,
	}, nil
}

// stripThinkingTags removes <think>...</think> blocks from model output.
// Thinking models like qwen3 and deepseek-r1 sometimes emit these even when
// told not to think.
func stripThinkingTags(s string) string {
	for {
		start := strings.Index(s, "<think>")
		if start == -1 {
			break
		}
		end := strings.Index(s, "</think>")
		if end == -1 {
			// Unclosed tag — remove from <think> to end.
			s = s[:start]
			break
		}
		s = s[:start] + s[end+len("</think>"):]
	}
	return strings.TrimSpace(s)
}
