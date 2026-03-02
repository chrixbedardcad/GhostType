package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/chrixbedardcad/GhostType/config"
)

const defaultOpenAIEndpoint = "https://api.openai.com/v1/chat/completions"

// OpenAIClient implements the Client interface for OpenAI.
type OpenAIClient struct {
	apiKey     string
	model      string
	endpoint   string
	maxTokens  int
	timeoutMs  int
	httpClient *http.Client
}

// NewOpenAIClient creates a new OpenAI client from config.
func NewOpenAIClient(cfg *config.Config) *OpenAIClient {
	endpoint := cfg.APIEndpoint
	if endpoint == "" {
		endpoint = defaultOpenAIEndpoint
	}

	return &OpenAIClient{
		apiKey:     cfg.APIKey,
		model:      cfg.Model,
		endpoint:   endpoint,
		maxTokens:  cfg.MaxTokens,
		timeoutMs:  cfg.TimeoutMs,
		httpClient: &http.Client{},
	}
}

// newOpenAIFromDef creates a new OpenAI client from a provider definition.
// Default max_completion_tokens is 2048 (higher than other providers) because
// OpenAI reasoning models (gpt-5-nano, o1, etc.) consume tokens for internal
// chain-of-thought in addition to the visible output.
func newOpenAIFromDef(def config.LLMProviderDef) *OpenAIClient {
	endpoint := def.APIEndpoint
	if endpoint == "" {
		endpoint = defaultOpenAIEndpoint
	}
	maxTokens := def.MaxTokens
	if maxTokens == 0 {
		maxTokens = 2048
	}
	timeoutMs := def.TimeoutMs
	if timeoutMs == 0 {
		timeoutMs = 30000
	}

	return &OpenAIClient{
		apiKey:     def.APIKey,
		model:      def.Model,
		endpoint:   endpoint,
		maxTokens:  maxTokens,
		timeoutMs:  timeoutMs,
		httpClient: &http.Client{},
	}
}

func (c *OpenAIClient) Provider() string {
	return "openai"
}

// openaiRequest is the request body for the OpenAI Chat Completions API.
type openaiRequest struct {
	Model              string          `json:"model"`
	Messages           []openaiMessage `json:"messages"`
	MaxCompletionTokens int            `json:"max_completion_tokens"`
}

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// openaiResponse is the response body from the OpenAI Chat Completions API.
type openaiResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

func (c *OpenAIClient) Send(ctx context.Context, req Request) (*Response, error) {
	fullPrompt := req.Prompt + "\n\n" + req.Text

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = c.maxTokens
	}

	body := openaiRequest{
		Model: c.model,
		Messages: []openaiMessage{
			{Role: "user", Content: fullPrompt},
		},
		MaxCompletionTokens: maxTokens,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	slog.Debug("openai: sending request", "model", c.model, "endpoint", c.endpoint, "body_len", len(jsonBody))

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	slog.Debug("openai: response received", "status", resp.StatusCode)

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		slog.Debug("openai: HTTP error", "status", resp.StatusCode, "body", string(respBody))
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	slog.Debug("openai: raw response body", "body", string(respBody))

	var apiResp openaiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if apiResp.Error != nil {
		return nil, fmt.Errorf("API error (%s): %s", apiResp.Error.Type, apiResp.Error.Message)
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("empty response from API")
	}

	text := apiResp.Choices[0].Message.Content
	slog.Debug("openai: parsed response", "text_len", len(text), "choices", len(apiResp.Choices))

	return &Response{
		Text:     text,
		Provider: "openai",
		Model:    c.model,
	}, nil
}
