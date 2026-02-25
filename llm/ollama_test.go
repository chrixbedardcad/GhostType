package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chrixbedardcad/GhostType/config"
)

func TestOllamaClient_Send_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request body
		var reqBody ollamaRequest
		json.NewDecoder(r.Body).Decode(&reqBody)
		if reqBody.Model != "mistral" {
			t.Errorf("expected model 'mistral', got '%s'", reqBody.Model)
		}
		if reqBody.Stream != false {
			t.Error("expected stream to be false")
		}

		resp := ollamaResponse{
			Response: "Hello, how are you?",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.Config{
		LLMProvider: "ollama",
		Model:       "mistral",
		APIEndpoint: server.URL,
		MaxTokens:   256,
		TimeoutMs:   5000,
	}
	client := NewOllamaClient(cfg)

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
	if resp.Provider != "ollama" {
		t.Errorf("expected provider 'ollama', got '%s'", resp.Provider)
	}
}

func TestOllamaClient_Send_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ollamaResponse{
			Error: "model 'nonexistent' not found",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.Config{
		LLMProvider: "ollama",
		Model:       "nonexistent",
		APIEndpoint: server.URL,
		MaxTokens:   256,
		TimeoutMs:   5000,
	}
	client := NewOllamaClient(cfg)

	_, err := client.Send(context.Background(), Request{
		Prompt: "Fix errors.",
		Text:   "test",
	})
	if err == nil {
		t.Fatal("expected error for model not found")
	}
}

func TestOllamaClient_Provider(t *testing.T) {
	cfg := &config.Config{
		Model:     "mistral",
		MaxTokens: 256,
		TimeoutMs: 5000,
	}
	client := NewOllamaClient(cfg)
	if client.Provider() != "ollama" {
		t.Errorf("expected 'ollama', got '%s'", client.Provider())
	}
}

func TestOllamaClient_DefaultEndpoint(t *testing.T) {
	cfg := &config.Config{
		Model:       "mistral",
		APIEndpoint: "",
		MaxTokens:   256,
		TimeoutMs:   5000,
	}
	client := NewOllamaClient(cfg)
	if client.endpoint != defaultOllamaEndpoint {
		t.Errorf("expected default endpoint '%s', got '%s'", defaultOllamaEndpoint, client.endpoint)
	}
}
