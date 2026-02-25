// ghosttype_test.go — Integration-level tests for the GhostType prototype.
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

	"github.com/chrixbedardcad/GhostType/clipboard"
	"github.com/chrixbedardcad/GhostType/config"
	"github.com/chrixbedardcad/GhostType/llm"
	"github.com/chrixbedardcad/GhostType/mode"
)

// TestFullCorrectionPipeline tests the complete F7 correction workflow
// with a mock LLM server.
func TestFullCorrectionPipeline(t *testing.T) {
	// 1. Set up mock LLM server
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

	// 2. Create config pointing to mock server
	cfg := &config.Config{
		LLMProvider: "anthropic",
		APIKey:      "test-key",
		Model:       "claude-sonnet-4-5-20250929",
		APIEndpoint: server.URL,
		Languages:   []string{"en", "fr"},
		LanguageNames: map[string]string{
			"en": "English",
			"fr": "French",
		},
		DefaultTranslateTarget: "en",
		Prompts: config.Prompts{
			Correct:   "Fix all spelling and grammar errors. Return ONLY the corrected text.",
			Translate: "Translate to {target_language}. Return ONLY the translation.",
			RewriteTemplates: []config.RewriteTemplate{
				{Name: "funny", Prompt: "Rewrite as funny. Return ONLY the text."},
			},
		},
		MaxTokens: 256,
		TimeoutMs: 5000,
	}

	// 3. Create LLM client
	client, err := llm.NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create LLM client: %v", err)
	}

	// 4. Create mode router
	router := mode.NewRouter(cfg, client)

	// 5. Simulate F7 correction
	inputText := "Helo, how are yu tday?"
	result, err := router.Process(context.Background(), mode.ModeCorrect, inputText)
	if err != nil {
		t.Fatalf("Correction failed: %v", err)
	}

	if result == "" {
		t.Fatal("Expected non-empty correction result")
	}

	t.Logf("Input:  %s", inputText)
	t.Logf("Output: %s", result)
}

// TestFullTranslationPipeline tests the F8 translation workflow.
func TestFullTranslationPipeline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": "Bonjour, comment allez-vous?"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.Config{
		LLMProvider: "anthropic",
		APIKey:      "test-key",
		Model:       "claude-sonnet-4-5-20250929",
		APIEndpoint: server.URL,
		Languages:   []string{"en", "fr"},
		LanguageNames: map[string]string{
			"en": "English",
			"fr": "French",
		},
		DefaultTranslateTarget: "fr",
		Prompts: config.Prompts{
			Correct:   "Fix errors.",
			Translate: "Translate to {target_language}. Return ONLY the translation.",
			RewriteTemplates: []config.RewriteTemplate{
				{Name: "funny", Prompt: "Rewrite as funny."},
			},
		},
		MaxTokens: 256,
		TimeoutMs: 5000,
	}

	client, err := llm.NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create LLM client: %v", err)
	}

	router := mode.NewRouter(cfg, client)

	result, err := router.Process(context.Background(), mode.ModeTranslate, "Hello, how are you?")
	if err != nil {
		t.Fatalf("Translation failed: %v", err)
	}

	if result == "" {
		t.Fatal("Expected non-empty translation result")
	}

	t.Logf("Input:  Hello, how are you?")
	t.Logf("Output: %s", result)
}

// TestFullRewritePipeline tests the F9 rewrite workflow.
func TestFullRewritePipeline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"content": []map[string]string{
				{"type": "text", "text": "LOL, you won't believe what just happened!"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.Config{
		LLMProvider: "anthropic",
		APIKey:      "test-key",
		Model:       "claude-sonnet-4-5-20250929",
		APIEndpoint: server.URL,
		Languages:   []string{"en", "fr"},
		LanguageNames: map[string]string{
			"en": "English",
			"fr": "French",
		},
		DefaultTranslateTarget: "en",
		Prompts: config.Prompts{
			Correct:   "Fix errors.",
			Translate: "Translate to {target_language}.",
			RewriteTemplates: []config.RewriteTemplate{
				{Name: "funny", Prompt: "Rewrite as funny. Return ONLY the text."},
				{Name: "formal", Prompt: "Rewrite formally. Return ONLY the text."},
			},
		},
		MaxTokens: 256,
		TimeoutMs: 5000,
	}

	client, err := llm.NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create LLM client: %v", err)
	}

	router := mode.NewRouter(cfg, client)

	result, err := router.Process(context.Background(), mode.ModeRewrite, "Something happened today.")
	if err != nil {
		t.Fatalf("Rewrite failed: %v", err)
	}

	if result == "" {
		t.Fatal("Expected non-empty rewrite result")
	}

	t.Logf("Input:  Something happened today.")
	t.Logf("Output: %s", result)
}

// TestClipboardPreservation tests clipboard save/restore behavior.
func TestClipboardPreservation(t *testing.T) {
	var store string = "user's important clipboard data"
	cb := clipboard.New(
		func() (string, error) { return store, nil },
		func(text string) error { store = text; return nil },
	)

	// Save original clipboard
	if err := cb.Save(); err != nil {
		t.Fatalf("Failed to save clipboard: %v", err)
	}

	// Simulate GhostType writing corrected text
	if err := cb.Write("corrected text from LLM"); err != nil {
		t.Fatalf("Failed to write clipboard: %v", err)
	}

	current, _ := cb.Read()
	if current != "corrected text from LLM" {
		t.Errorf("Expected 'corrected text from LLM', got '%s'", current)
	}

	// Restore original clipboard
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

	// Load should create a default
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Failed to load/create config: %v", err)
	}

	if cfg.LLMProvider != "anthropic" {
		t.Errorf("Expected default provider 'anthropic', got '%s'", cfg.LLMProvider)
	}

	// Verify the file was created and can be read
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read created config: %v", err)
	}

	var reloaded config.Config
	if err := json.Unmarshal(data, &reloaded); err != nil {
		t.Fatalf("Created config is invalid JSON: %v", err)
	}

	if reloaded.Model != "claude-sonnet-4-5-20250929" {
		t.Errorf("Expected default model in file, got '%s'", reloaded.Model)
	}
}

// TestOpenAIPipeline tests the full pipeline using the OpenAI provider.
func TestOpenAIPipeline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify OpenAI-style auth header
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
		LLMProvider: "openai",
		APIKey:      "test-openai-key",
		Model:       "gpt-4o",
		APIEndpoint: server.URL,
		Languages:   []string{"en", "fr"},
		LanguageNames: map[string]string{
			"en": "English",
			"fr": "French",
		},
		DefaultTranslateTarget: "en",
		Prompts: config.Prompts{
			Correct:   "Fix errors. Return ONLY corrected text.",
			Translate: "Translate to {target_language}.",
			RewriteTemplates: []config.RewriteTemplate{
				{Name: "funny", Prompt: "Rewrite as funny."},
			},
		},
		MaxTokens: 256,
		TimeoutMs: 5000,
	}

	client, err := llm.NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create OpenAI client: %v", err)
	}

	router := mode.NewRouter(cfg, client)

	result, err := router.Process(context.Background(), mode.ModeCorrect, "Helo wrold")
	if err != nil {
		t.Fatalf("OpenAI correction failed: %v", err)
	}

	if result != "Corrected text from GPT." {
		t.Errorf("Expected 'Corrected text from GPT.', got '%s'", result)
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
		LLMProvider: "anthropic",
		APIKey:      "test-key",
		Model:       "claude-sonnet-4-5-20250929",
		APIEndpoint: server.URL,
		Languages:   []string{"en"},
		LanguageNames: map[string]string{
			"en": "English",
		},
		DefaultTranslateTarget: "en",
		Prompts: config.Prompts{
			Correct:   "Fix errors.",
			Translate: "Translate.",
			RewriteTemplates: []config.RewriteTemplate{
				{Name: "funny", Prompt: "Rewrite."},
			},
		},
		MaxTokens: 256,
		TimeoutMs: 5000,
	}

	client, err := llm.NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	router := mode.NewRouter(cfg, client)

	originalText := "This is the original text that should not be replaced"
	_, err = router.Process(context.Background(), mode.ModeCorrect, originalText)
	if err == nil {
		t.Fatal("Expected error from failing LLM, but got none — text would have been replaced!")
	}

	t.Logf("Correctly received error: %v", err)
	t.Log("Original text preserved (not replaced) — safety check passed")
}
