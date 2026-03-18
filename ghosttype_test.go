// ghostspell_test.go — Integration-level tests for the GhostSpell prototype.
// Run with: go test -v ./...
// These tests verify the full pipeline: config loading, LLM client creation,
// mode routing with mock LLM, and clipboard operations.

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/chrixbedardcad/GhostSpell/clipboard"
	"github.com/chrixbedardcad/GhostSpell/config"
	"github.com/chrixbedardcad/GhostSpell/mode"
)

// TestFullCorrectionPipeline tests the complete correction workflow
// with a mock LLM server.
func TestFullCorrectionPipeline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": "Hello, how are you today?"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.Config{
		Providers: map[string]config.ProviderConfig{
			"anthropic": {APIKey: "test-key", APIEndpoint: server.URL},
		},
		Models: map[string]config.ModelEntry{
			"default": {Provider: "anthropic", Model: "claude-sonnet-4-5-20250929", MaxTokens: 256},
		},
		DefaultModel: "default",
		Prompts: []config.PromptEntry{
			{Name: "Correct", Prompt: "Fix all spelling and grammar errors. Return ONLY the corrected text."},
		},
		ActivePrompt: 0,
		MaxTokens:    256,
		TimeoutMs:    5000,
	}

	client, err := newClientFromConfig(cfg, cfg.DefaultModel)
	if err != nil {
		t.Fatalf("Failed to create LLM client: %v", err)
	}

	router := mode.NewRouter(cfg, client)

	inputText := "Helo, how are yu tday?"
	resp, err := router.Process(context.Background(), 0, inputText)
	if err != nil {
		t.Fatalf("Correction failed: %v", err)
	}

	if resp.Text == "" {
		t.Fatal("Expected non-empty correction result")
	}

	t.Logf("Input:  %s", inputText)
	t.Logf("Output: %s", resp.Text)
}

// TestFullPolishPipeline tests the polish prompt workflow.
func TestFullPolishPipeline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": "This is a polished and refined version."},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.Config{
		Providers: map[string]config.ProviderConfig{
			"anthropic": {APIKey: "test-key", APIEndpoint: server.URL},
		},
		Models: map[string]config.ModelEntry{
			"default": {Provider: "anthropic", Model: "claude-sonnet-4-5-20250929", MaxTokens: 256},
		},
		DefaultModel: "default",
		Prompts: []config.PromptEntry{
			{Name: "Correct", Prompt: "Fix errors."},
			{Name: "Polish", Prompt: "Improve the text."},
		},
		ActivePrompt: 1,
		MaxTokens:    256,
		TimeoutMs:    5000,
	}

	client, err := newClientFromConfig(cfg, cfg.DefaultModel)
	if err != nil {
		t.Fatalf("Failed to create LLM client: %v", err)
	}

	router := mode.NewRouter(cfg, client)

	resp, err := router.Process(context.Background(), 1, "some rough text here")
	if err != nil {
		t.Fatalf("Polish failed: %v", err)
	}

	if resp.Text == "" {
		t.Fatal("Expected non-empty polish result")
	}

	t.Logf("Output: %s", resp.Text)
}

// TestClipboardPreservation tests clipboard save/restore behavior.
func TestClipboardPreservation(t *testing.T) {
	var store string = "user's important clipboard data"
	cb := clipboard.New(
		func() (string, error) { return store, nil },
		func(text string) error { store = text; return nil },
	)

	if err := cb.Save(); err != nil {
		t.Fatalf("Failed to save clipboard: %v", err)
	}

	if err := cb.Write("corrected text from LLM"); err != nil {
		t.Fatalf("Failed to write clipboard: %v", err)
	}

	current, _ := cb.Read()
	if current != "corrected text from LLM" {
		t.Errorf("Expected 'corrected text from LLM', got '%s'", current)
	}

	if err := cb.Restore(); err != nil {
		t.Fatalf("Failed to restore clipboard: %v", err)
	}

	restored, _ := cb.Read()
	if restored != "user's important clipboard data" {
		t.Errorf("Expected original clipboard restored, got '%s'", restored)
	}
}

// TestConfigLoadAndCreateDefault verifies config creates a default file
// and that it can be re-loaded.
func TestConfigLoadAndCreateDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg, err := config.LoadRaw(path)
	if err != nil {
		t.Fatalf("Failed to load/create config: %v", err)
	}

	if len(cfg.Prompts) != 8 {
		t.Errorf("Expected 8 default prompts, got %d", len(cfg.Prompts))
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read created config: %v", err)
	}

	var reloaded config.Config
	if err := json.Unmarshal(data, &reloaded); err != nil {
		t.Fatalf("Created config is invalid JSON: %v", err)
	}

	if reloaded.Hotkeys.Action != "Ctrl+G" {
		t.Errorf("Expected default hotkey 'Ctrl+G' in file, got '%s'", reloaded.Hotkeys.Action)
	}
}

// TestOpenAIPipeline tests the full pipeline using the OpenAI provider.
func TestOpenAIPipeline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-openai-key" {
			t.Errorf("Expected OpenAI auth header, got '%s'", auth)
		}

		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "Corrected text from GPT."}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.Config{
		Providers: map[string]config.ProviderConfig{
			"openai": {APIKey: "test-openai-key", APIEndpoint: server.URL},
		},
		Models: map[string]config.ModelEntry{
			"default": {Provider: "openai", Model: "gpt-4o", MaxTokens: 256},
		},
		DefaultModel: "default",
		Prompts: []config.PromptEntry{
			{Name: "Correct", Prompt: "Fix errors. Return ONLY corrected text."},
		},
		ActivePrompt: 0,
		MaxTokens:    256,
		TimeoutMs:    5000,
	}

	client, err := newClientFromConfig(cfg, cfg.DefaultModel)
	if err != nil {
		t.Fatalf("Failed to create OpenAI client: %v", err)
	}

	router := mode.NewRouter(cfg, client)

	resp, err := router.Process(context.Background(), 0, "Helo wrold")
	if err != nil {
		t.Fatalf("OpenAI correction failed: %v", err)
	}

	if resp.Text != "Corrected text from GPT." {
		t.Errorf("Expected 'Corrected text from GPT.', got '%s'", resp.Text)
	}
}

// TestLLMErrorDoesNotReplaceText verifies the critical safety rule:
// NEVER replace text if API call fails.
func TestLLMErrorDoesNotReplaceText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"server error"}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		Providers: map[string]config.ProviderConfig{
			"anthropic": {APIKey: "test-key", APIEndpoint: server.URL},
		},
		Models: map[string]config.ModelEntry{
			"default": {Provider: "anthropic", Model: "claude-sonnet-4-5-20250929", MaxTokens: 256},
		},
		DefaultModel: "default",
		Prompts: []config.PromptEntry{
			{Name: "Correct", Prompt: "Fix errors."},
		},
		ActivePrompt: 0,
		MaxTokens:    256,
		TimeoutMs:    5000,
	}

	client, err := newClientFromConfig(cfg, cfg.DefaultModel)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	router := mode.NewRouter(cfg, client)

	originalText := "This is the original text that should not be replaced"
	_, err = router.Process(context.Background(), 0, originalText)
	if err == nil {
		t.Fatal("Expected error from failing LLM, but got none — text would have been replaced!")
	}

	t.Logf("Correctly received error: %v", err)
	t.Log("Original text preserved (not replaced) — safety check passed")
}
