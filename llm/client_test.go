package llm

import (
	"testing"

	"github.com/chrixbedardcad/GhostType/config"
)

func TestNewClient_Anthropic(t *testing.T) {
	cfg := &config.Config{
		LLMProvider: "anthropic",
		APIKey:      "test-key",
		Model:       "claude-sonnet-4-5-20250929",
		MaxTokens:   256,
		TimeoutMs:   5000,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.Provider() != "anthropic" {
		t.Errorf("expected provider 'anthropic', got '%s'", client.Provider())
	}
}

func TestNewClient_OpenAI(t *testing.T) {
	cfg := &config.Config{
		LLMProvider: "openai",
		APIKey:      "test-key",
		Model:       "gpt-4o",
		MaxTokens:   256,
		TimeoutMs:   5000,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.Provider() != "openai" {
		t.Errorf("expected provider 'openai', got '%s'", client.Provider())
	}
}

func TestNewClient_Ollama(t *testing.T) {
	cfg := &config.Config{
		LLMProvider: "ollama",
		Model:       "mistral",
		MaxTokens:   256,
		TimeoutMs:   5000,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.Provider() != "ollama" {
		t.Errorf("expected provider 'ollama', got '%s'", client.Provider())
	}
}

func TestNewClient_Unsupported(t *testing.T) {
	cfg := &config.Config{
		LLMProvider: "unsupported",
		APIKey:      "test",
		Model:       "test",
		MaxTokens:   256,
		TimeoutMs:   5000,
	}

	_, err := NewClient(cfg)
	if err == nil {
		t.Fatal("expected error for unsupported provider")
	}
}

func TestNewClientFromDef_Anthropic(t *testing.T) {
	def := config.LLMProviderDef{
		Provider:  "anthropic",
		APIKey:    "test-key",
		Model:     "claude-sonnet-4-5-20250929",
		MaxTokens: 256,
		TimeoutMs: 5000,
	}

	client, err := NewClientFromDef(def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.Provider() != "anthropic" {
		t.Errorf("expected provider 'anthropic', got '%s'", client.Provider())
	}
}

func TestNewClientFromDef_OpenAI(t *testing.T) {
	def := config.LLMProviderDef{
		Provider:  "openai",
		APIKey:    "test-key",
		Model:     "gpt-4o",
		MaxTokens: 256,
		TimeoutMs: 5000,
	}

	client, err := NewClientFromDef(def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.Provider() != "openai" {
		t.Errorf("expected provider 'openai', got '%s'", client.Provider())
	}
}

func TestNewClientFromDef_Ollama(t *testing.T) {
	def := config.LLMProviderDef{
		Provider:  "ollama",
		Model:     "mistral",
		MaxTokens: 256,
		TimeoutMs: 5000,
	}

	client, err := NewClientFromDef(def)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.Provider() != "ollama" {
		t.Errorf("expected provider 'ollama', got '%s'", client.Provider())
	}
}

func TestNewClientFromDef_Unsupported(t *testing.T) {
	def := config.LLMProviderDef{
		Provider: "unsupported",
		APIKey:   "test",
		Model:    "test",
	}

	_, err := NewClientFromDef(def)
	if err == nil {
		t.Fatal("expected error for unsupported provider")
	}
}
