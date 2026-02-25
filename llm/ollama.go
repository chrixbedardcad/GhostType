package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/chrixbedardcad/GhostType/config"
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
	endpoint := cfg.APIEndpoint
	if endpoint == "" {
		endpoint = defaultOllamaEndpoint
	}

	return &OllamaClient{
		model:    cfg.Model,
		endpoint: endpoint,
		maxTokens: cfg.MaxTokens,
		timeoutMs: cfg.TimeoutMs,
		httpClient: &http.Client{
			Timeout: time.Duration(cfg.TimeoutMs) * time.Millisecond,
		},
	}
}

func (c *OllamaClient) Provider() string {
	return "ollama"
}

// ollamaRequest is the request body for the Ollama generate API.
type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

// ollamaResponse is the response body from the Ollama generate API.
type ollamaResponse struct {
	Response string `json:"response"`
	Error    string `json:"error,omitempty"`
}

func (c *OllamaClient) Send(ctx context.Context, req Request) (*Response, error) {
	fullPrompt := req.Prompt + "\n\n" + req.Text

	body := ollamaRequest{
		Model:  c.model,
		Prompt: fullPrompt,
		Stream: false,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("API request failed (is Ollama running?): %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var apiResp ollamaResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if apiResp.Error != "" {
		return nil, fmt.Errorf("Ollama error: %s", apiResp.Error)
	}

	if apiResp.Response == "" {
		return nil, fmt.Errorf("empty response from Ollama")
	}

	return &Response{
		Text:     apiResp.Response,
		Provider: "ollama",
		Model:    c.model,
	}, nil
}
