package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chrixbedardcad/GhostType/config"
)

func TestOpenAIClient_Send_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request headers
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected Authorization 'Bearer test-key', got '%s'", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type 'application/json', got '%s'", r.Header.Get("Content-Type"))
		}

		// Verify request body
		var reqBody openaiRequest
		json.NewDecoder(r.Body).Decode(&reqBody)
		if reqBody.Model != "gpt-4o" {
			t.Errorf("expected model 'gpt-4o', got '%s'", reqBody.Model)
		}

		// Return success response
		resp := openaiResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{Message: struct {
					Content string `json:"content"`
				}{Content: "Hello, how are you?"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.Config{
		APIKey:      "test-key",
		Model:       "gpt-4o",
		APIEndpoint: server.URL,
		MaxTokens:   256,
		TimeoutMs:   5000,
	}
	client := NewOpenAIClient(cfg)

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
	if resp.Provider != "openai" {
		t.Errorf("expected provider 'openai', got '%s'", resp.Provider)
	}
	if resp.Model != "gpt-4o" {
		t.Errorf("expected model 'gpt-4o', got '%s'", resp.Model)
	}
}

func TestOpenAIClient_Send_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"message":"invalid api key","type":"invalid_request_error"}}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		APIKey:      "bad-key",
		Model:       "gpt-4o",
		APIEndpoint: server.URL,
		MaxTokens:   256,
		TimeoutMs:   5000,
	}
	client := NewOpenAIClient(cfg)

	_, err := client.Send(context.Background(), Request{
		Prompt: "Fix errors.",
		Text:   "test",
	})
	if err == nil {
		t.Fatal("expected error for unauthorized request")
	}
}

func TestOpenAIClient_Send_EmptyChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openaiResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.Config{
		APIKey:      "test-key",
		Model:       "gpt-4o",
		APIEndpoint: server.URL,
		MaxTokens:   256,
		TimeoutMs:   5000,
	}
	client := NewOpenAIClient(cfg)

	_, err := client.Send(context.Background(), Request{
		Prompt: "Fix errors.",
		Text:   "test",
	})
	if err == nil {
		t.Fatal("expected error for empty choices")
	}
}

func TestOpenAIClient_Provider(t *testing.T) {
	cfg := &config.Config{
		APIKey:    "test",
		Model:     "test",
		MaxTokens: 256,
		TimeoutMs: 5000,
	}
	client := NewOpenAIClient(cfg)
	if client.Provider() != "openai" {
		t.Errorf("expected 'openai', got '%s'", client.Provider())
	}
}

func TestOpenAIClient_DefaultEndpoint(t *testing.T) {
	cfg := &config.Config{
		APIKey:      "test",
		Model:       "test",
		APIEndpoint: "",
		MaxTokens:   256,
		TimeoutMs:   5000,
	}
	client := NewOpenAIClient(cfg)
	if client.endpoint != defaultOpenAIEndpoint {
		t.Errorf("expected default endpoint '%s', got '%s'", defaultOpenAIEndpoint, client.endpoint)
	}
}
