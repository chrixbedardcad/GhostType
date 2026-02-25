package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chrixbedardcad/GhostType/config"
)

func TestAnthropicClient_Send_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request headers
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("expected x-api-key 'test-key', got '%s'", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Errorf("expected anthropic-version '2023-06-01', got '%s'", r.Header.Get("anthropic-version"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type 'application/json', got '%s'", r.Header.Get("Content-Type"))
		}

		// Verify request body
		var reqBody anthropicRequest
		json.NewDecoder(r.Body).Decode(&reqBody)
		if reqBody.Model != "claude-sonnet-4-5-20250929" {
			t.Errorf("expected model 'claude-sonnet-4-5-20250929', got '%s'", reqBody.Model)
		}
		if len(reqBody.Messages) != 1 {
			t.Fatalf("expected 1 message, got %d", len(reqBody.Messages))
		}

		// Return success response
		resp := anthropicResponse{
			Content: []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}{
				{Type: "text", Text: "Hello, how are you?"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.Config{
		APIKey:      "test-key",
		Model:       "claude-sonnet-4-5-20250929",
		APIEndpoint: server.URL,
		MaxTokens:   256,
		TimeoutMs:   5000,
	}
	client := NewAnthropicClient(cfg)

	resp, err := client.Send(context.Background(), Request{
		Prompt: "Fix spelling errors.",
		Text:   "Helo, how are yu?",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Text != "Hello, how are you?" {
		t.Errorf("expected 'Hello, how are you?', got '%s'", resp.Text)
	}
	if resp.Provider != "anthropic" {
		t.Errorf("expected provider 'anthropic', got '%s'", resp.Provider)
	}
	if resp.Model != "claude-sonnet-4-5-20250929" {
		t.Errorf("expected model 'claude-sonnet-4-5-20250929', got '%s'", resp.Model)
	}
}

func TestAnthropicClient_Send_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"type":"authentication_error","message":"invalid api key"}}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		APIKey:      "bad-key",
		Model:       "claude-sonnet-4-5-20250929",
		APIEndpoint: server.URL,
		MaxTokens:   256,
		TimeoutMs:   5000,
	}
	client := NewAnthropicClient(cfg)

	_, err := client.Send(context.Background(), Request{
		Prompt: "Fix errors.",
		Text:   "test",
	})
	if err == nil {
		t.Fatal("expected error for unauthorized request")
	}
}

func TestAnthropicClient_Send_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := anthropicResponse{
			Content: []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.Config{
		APIKey:      "test-key",
		Model:       "claude-sonnet-4-5-20250929",
		APIEndpoint: server.URL,
		MaxTokens:   256,
		TimeoutMs:   5000,
	}
	client := NewAnthropicClient(cfg)

	_, err := client.Send(context.Background(), Request{
		Prompt: "Fix errors.",
		Text:   "test",
	})
	if err == nil {
		t.Fatal("expected error for empty response")
	}
}

func TestAnthropicClient_Provider(t *testing.T) {
	cfg := &config.Config{
		APIKey:    "test",
		Model:     "test",
		MaxTokens: 256,
		TimeoutMs: 5000,
	}
	client := NewAnthropicClient(cfg)
	if client.Provider() != "anthropic" {
		t.Errorf("expected 'anthropic', got '%s'", client.Provider())
	}
}

func TestAnthropicClient_DefaultEndpoint(t *testing.T) {
	cfg := &config.Config{
		APIKey:      "test",
		Model:       "test",
		APIEndpoint: "",
		MaxTokens:   256,
		TimeoutMs:   5000,
	}
	client := NewAnthropicClient(cfg)
	if client.endpoint != defaultAnthropicEndpoint {
		t.Errorf("expected default endpoint '%s', got '%s'", defaultAnthropicEndpoint, client.endpoint)
	}
}
