package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.LLMProvider != "anthropic" {
		t.Errorf("expected default provider 'anthropic', got '%s'", cfg.LLMProvider)
	}
	if cfg.Model != "claude-sonnet-4-5-20250929" {
		t.Errorf("expected default model 'claude-sonnet-4-5-20250929', got '%s'", cfg.Model)
	}
	if cfg.MaxTokens != 256 {
		t.Errorf("expected default max_tokens 256, got %d", cfg.MaxTokens)
	}
	if cfg.TimeoutMs != 5000 {
		t.Errorf("expected default timeout_ms 5000, got %d", cfg.TimeoutMs)
	}
	if cfg.Hotkeys.Correct != "F6" {
		t.Errorf("expected default correct hotkey 'F6', got '%s'", cfg.Hotkeys.Correct)
	}
	if cfg.Hotkeys.Translate != "F7" {
		t.Errorf("expected default translate hotkey 'F7', got '%s'", cfg.Hotkeys.Translate)
	}
	if cfg.Hotkeys.Rewrite != "F8" {
		t.Errorf("expected default rewrite hotkey 'F8', got '%s'", cfg.Hotkeys.Rewrite)
	}
	if cfg.TargetWindow != "Firestorm" {
		t.Errorf("expected default target_window 'Firestorm', got '%s'", cfg.TargetWindow)
	}
	if len(cfg.Languages) != 2 {
		t.Errorf("expected 2 default languages, got %d", len(cfg.Languages))
	}
	if len(cfg.Prompts.RewriteTemplates) != 5 {
		t.Errorf("expected 5 default rewrite templates, got %d", len(cfg.Prompts.RewriteTemplates))
	}
	if !cfg.PreserveClipboard {
		t.Error("expected preserve_clipboard to be true by default")
	}
}

func TestLoadCreatesDefaultWhenMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.LLMProvider != "anthropic" {
		t.Errorf("expected default provider, got '%s'", cfg.LLMProvider)
	}

	// Verify file was created
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("expected config file to be created")
	}
}

func TestLoadValidConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := map[string]interface{}{
		"llm_provider": "openai",
		"api_key":      "sk-test-key-12345",
		"model":        "gpt-4o",
		"prompts": map[string]interface{}{
			"correct": "Fix spelling errors. Return ONLY corrected text.",
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(path, data, 0644)

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if loaded.LLMProvider != "openai" {
		t.Errorf("expected provider 'openai', got '%s'", loaded.LLMProvider)
	}
	if loaded.APIKey != "sk-test-key-12345" {
		t.Errorf("expected api_key 'sk-test-key-12345', got '%s'", loaded.APIKey)
	}
	if loaded.Model != "gpt-4o" {
		t.Errorf("expected model 'gpt-4o', got '%s'", loaded.Model)
	}
}

func TestLoadInvalidProvider(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := map[string]interface{}{
		"llm_provider": "invalid_provider",
		"api_key":      "sk-test",
		"model":        "some-model",
		"prompts": map[string]interface{}{
			"correct": "Fix errors.",
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(path, data, 0644)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error for invalid provider")
	}
}

func TestLoadMissingAPIKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := map[string]interface{}{
		"llm_provider": "anthropic",
		"api_key":      "",
		"model":        "claude-sonnet-4-5-20250929",
		"prompts": map[string]interface{}{
			"correct": "Fix errors.",
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(path, data, 0644)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error for missing API key")
	}
}

func TestLoadOllamaNoAPIKeyRequired(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := map[string]interface{}{
		"llm_provider": "ollama",
		"api_key":      "",
		"model":        "mistral",
		"prompts": map[string]interface{}{
			"correct": "Fix errors.",
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(path, data, 0644)

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if loaded.LLMProvider != "ollama" {
		t.Errorf("expected provider 'ollama', got '%s'", loaded.LLMProvider)
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	os.WriteFile(path, []byte("not valid json{{{"), 0644)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	// Minimal config, should get defaults applied
	cfg := map[string]interface{}{
		"llm_provider": "openai",
		"api_key":      "sk-test",
		"model":        "gpt-4o",
		"prompts": map[string]interface{}{
			"correct": "Fix errors.",
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(path, data, 0644)

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if loaded.MaxTokens != 256 {
		t.Errorf("expected default max_tokens 256, got %d", loaded.MaxTokens)
	}
	if loaded.TimeoutMs != 5000 {
		t.Errorf("expected default timeout_ms 5000, got %d", loaded.TimeoutMs)
	}
	if loaded.LogLevel != "info" {
		t.Errorf("expected default log_level 'info', got '%s'", loaded.LogLevel)
	}
	if loaded.TargetWindow != "Firestorm" {
		t.Errorf("expected default target_window 'Firestorm', got '%s'", loaded.TargetWindow)
	}
}

func TestWriteDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "config.json")

	cfg := DefaultConfig()
	err := WriteDefault(path, &cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}

	var loaded Config
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("written file is not valid JSON: %v", err)
	}

	if loaded.LLMProvider != "anthropic" {
		t.Errorf("expected provider 'anthropic' in written file, got '%s'", loaded.LLMProvider)
	}
}
