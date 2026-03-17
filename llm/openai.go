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

	"github.com/chrixbedardcad/GhostSpell/config"
)

const defaultOpenAIEndpoint = "https://api.openai.com/v1/chat/completions"

// OpenAIClient implements the Client interface for OpenAI and OpenAI-compatible
// providers (Gemini, xAI). The providerName field controls the name reported
// in Provider() and Response.Provider.
type OpenAIClient struct {
	apiKey       string
	model        string
	endpoint     string
	maxTokens    int
	timeoutMs    int
	providerName string
	httpClient   *http.Client
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
		apiKey:       def.APIKey,
		model:        def.Model,
		endpoint:     endpoint,
		maxTokens:    maxTokens,
		timeoutMs:    timeoutMs,
		providerName: def.Provider,
		httpClient:   newPooledHTTPClient(timeoutMs),
	}
}

func (c *OpenAIClient) Provider() string {
	return c.providerName
}

func (c *OpenAIClient) Close() {
	c.httpClient.CloseIdleConnections()
}

// openaiRequest is the request body for the OpenAI Chat Completions API.
// OpenAI uses max_completion_tokens (required for reasoning models like o1).
// Gemini and xAI use the standard max_tokens field via their OpenAI-compatible
// endpoints — they ignore max_completion_tokens.
type openaiRequest struct {
	Model               string          `json:"model"`
	Messages            []openaiMessage `json:"messages"`
	MaxCompletionTokens int             `json:"max_completion_tokens,omitempty"`
	MaxTokens           int             `json:"max_tokens,omitempty"`
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
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		CompletionTokensDetails *struct {
			ReasoningTokens int `json:"reasoning_tokens"`
		} `json:"completion_tokens_details"`
	} `json:"usage"`
	Error   json.RawMessage `json:"error,omitempty"` // OpenAI: {"message":...}, LM Studio: "string"
	ErrorOK bool            `json:"-"`               // set after parsing
	ErrorMsg string         `json:"-"`               // extracted error message
}

func (c *OpenAIClient) Send(ctx context.Context, req Request) (*Response, error) {
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = c.maxTokens
	}

	body := openaiRequest{
		Model: c.model,
		Messages: []openaiMessage{
			{Role: "system", Content: req.Prompt},
			{Role: "user", Content: req.Text},
		},
	}
	// OpenAI uses max_completion_tokens (required for reasoning models).
	// Gemini and xAI use the standard max_tokens field.
	if c.providerName == "openai" || c.providerName == "chatgpt" {
		body.MaxCompletionTokens = maxTokens
	} else {
		body.MaxTokens = maxTokens
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	slog.Debug(c.providerName+": sending request", "model", c.model, "endpoint", c.endpoint, "body_len", len(jsonBody))

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

	slog.Debug(c.providerName+": response received", "status", resp.StatusCode)

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		slog.Debug(c.providerName+": HTTP error", "status", resp.StatusCode, "body", string(respBody))
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	slog.Debug(c.providerName+": raw response body", "body", string(respBody))

	var apiResp openaiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Parse error field — OpenAI uses {"message":..., "type":...},
	// LM Studio uses a plain string. Handle both formats.
	if len(apiResp.Error) > 0 && string(apiResp.Error) != "null" {
		// Try OpenAI format first.
		var errObj struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		}
		if json.Unmarshal(apiResp.Error, &errObj) == nil && errObj.Message != "" {
			return nil, fmt.Errorf("API error (%s): %s", errObj.Type, errObj.Message)
		}
		// Fall back to plain string (LM Studio format).
		var errStr string
		if json.Unmarshal(apiResp.Error, &errStr) == nil && errStr != "" {
			return nil, fmt.Errorf("API error: %s", errStr)
		}
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("empty response from API")
	}

	text := apiResp.Choices[0].Message.Content
	slog.Debug(c.providerName+": parsed response", "text_len", len(text), "choices", len(apiResp.Choices))

	if strings.TrimSpace(text) == "" {
		// Detect reasoning models that exhaust tokens on chain-of-thought.
		finishReason := apiResp.Choices[0].FinishReason
		reasoningTokens := 0
		if apiResp.Usage != nil && apiResp.Usage.CompletionTokensDetails != nil {
			reasoningTokens = apiResp.Usage.CompletionTokensDetails.ReasoningTokens
		}
		if finishReason == "length" && reasoningTokens > 0 {
			return nil, fmt.Errorf("model %s used all %d tokens for reasoning with no visible output — increase max_tokens in model settings", c.model, reasoningTokens)
		}
		return nil, fmt.Errorf("API returned empty content (model=%s, finish_reason=%s)", c.model, finishReason)
	}

	return &Response{
		Text:     text,
		Provider: c.providerName,
		Model:    c.model,
	}, nil
}
