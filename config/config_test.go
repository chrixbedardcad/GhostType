package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxTokens != 256 {
		t.Errorf("expected default max_tokens 256, got %d", cfg.MaxTokens)
	}
	if cfg.TimeoutMs != 30000 {
		t.Errorf("expected default timeout_ms 30000, got %d", cfg.TimeoutMs)
	}
	if cfg.Hotkeys.Action != "Ctrl+G" {
		t.Errorf("expected default action hotkey 'Ctrl+G', got '%s'", cfg.Hotkeys.Action)
	}
	if len(cfg.Prompts) != 3 {
		t.Errorf("expected 3 default prompts, got %d", len(cfg.Prompts))
	}
	if cfg.Prompts[0].Name != "Correct" {
		t.Errorf("expected first prompt 'Correct', got '%s'", cfg.Prompts[0].Name)
	}
	if cfg.Prompts[1].Name != "Polish" {
		t.Errorf("expected second prompt 'Polish', got '%s'", cfg.Prompts[1].Name)
	}
	if cfg.Prompts[2].Name != "Translate to En" {
		t.Errorf("expected third prompt 'Translate to En', got '%s'", cfg.Prompts[2].Name)
	}
	if cfg.ActivePrompt != 0 {
		t.Errorf("expected default active_prompt 0, got %d", cfg.ActivePrompt)
	}
	if !cfg.PreserveClipboard {
		t.Error("expected preserve_clipboard to be true by default")
	}
}

func TestLoadCreatesDefaultWhenMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg, err := LoadRaw(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Prompts) != 3 {
		t.Errorf("expected 3 default prompts, got %d", len(cfg.Prompts))
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
		"prompts": []map[string]interface{}{
			{"name": "Correct", "prompt": "Fix spelling errors."},
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
		t.Errorf("expected api_key, got '%s'", loaded.APIKey)
	}
	if len(loaded.Prompts) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(loaded.Prompts))
	}
	if loaded.Prompts[0].Name != "Correct" {
		t.Errorf("expected prompt name 'Correct', got '%s'", loaded.Prompts[0].Name)
	}
}

func TestLoadInvalidProvider(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := map[string]interface{}{
		"llm_provider": "invalid_provider",
		"api_key":      "sk-test",
		"model":        "some-model",
		"prompts": []map[string]interface{}{
			{"name": "Correct", "prompt": "Fix errors."},
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
		"prompts": []map[string]interface{}{
			{"name": "Correct", "prompt": "Fix errors."},
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
		"prompts": []map[string]interface{}{
			{"name": "Correct", "prompt": "Fix errors."},
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

	cfg := map[string]interface{}{
		"llm_provider": "openai",
		"api_key":      "sk-test",
		"model":        "gpt-4o",
		"prompts": []map[string]interface{}{
			{"name": "Correct", "prompt": "Fix errors."},
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
	if loaded.TimeoutMs != 30000 {
		t.Errorf("expected default timeout_ms 30000, got %d", loaded.TimeoutMs)
	}
}

func TestLoadLoggingDisabledByDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := map[string]interface{}{
		"llm_provider": "openai",
		"api_key":      "sk-test",
		"model":        "gpt-4o",
		"prompts": []map[string]interface{}{
			{"name": "Correct", "prompt": "Fix errors."},
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(path, data, 0644)

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if loaded.LogLevel != "" {
		t.Errorf("expected empty log_level when not set, got '%s'", loaded.LogLevel)
	}
	if loaded.LogFile != "" {
		t.Errorf("expected empty log_file when logging disabled, got '%s'", loaded.LogFile)
	}
}

func TestLoadLoggingEnabledSetsLogFileDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := map[string]interface{}{
		"llm_provider": "openai",
		"api_key":      "sk-test",
		"model":        "gpt-4o",
		"log_level":    "debug",
		"prompts": []map[string]interface{}{
			{"name": "Correct", "prompt": "Fix errors."},
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(path, data, 0644)

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if loaded.LogLevel != "debug" {
		t.Errorf("expected log_level 'debug', got '%s'", loaded.LogLevel)
	}
	if loaded.LogFile != "ghosttype.log" {
		t.Errorf("expected default log_file 'ghosttype.log', got '%s'", loaded.LogFile)
	}
}

func TestLoadInvalidLogLevel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := map[string]interface{}{
		"llm_provider": "openai",
		"api_key":      "sk-test",
		"model":        "gpt-4o",
		"log_level":    "verbose",
		"prompts": []map[string]interface{}{
			{"name": "Correct", "prompt": "Fix errors."},
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(path, data, 0644)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error for invalid log_level")
	}
}

func TestLoadOptionalHotkeysEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := map[string]interface{}{
		"llm_provider": "openai",
		"api_key":      "sk-test",
		"model":        "gpt-4o",
		"prompts": []map[string]interface{}{
			{"name": "Correct", "prompt": "Fix errors."},
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(path, data, 0644)

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Action hotkey should have a default
	if loaded.Hotkeys.Action != "Ctrl+G" {
		t.Errorf("expected action hotkey 'Ctrl+G', got '%s'", loaded.Hotkeys.Action)
	}
	// CyclePrompt should remain empty
	if loaded.Hotkeys.CyclePrompt != "" {
		t.Errorf("expected cycle_prompt hotkey empty, got '%s'", loaded.Hotkeys.CyclePrompt)
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

	if loaded.Hotkeys.Action != "Ctrl+G" {
		t.Errorf("expected hotkey 'Ctrl+G' in written file, got '%s'", loaded.Hotkeys.Action)
	}
	if len(loaded.Prompts) != 3 {
		t.Errorf("expected 3 prompts in written file, got %d", len(loaded.Prompts))
	}
}

func TestLLMProvidersSynthesizedFromFlat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := map[string]interface{}{
		"llm_provider": "openai",
		"api_key":      "sk-test-key",
		"model":        "gpt-4o",
		"prompts": []map[string]interface{}{
			{"name": "Correct", "prompt": "Fix errors."},
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(path, data, 0644)

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if loaded.DefaultLLM != "default" {
		t.Errorf("expected default_llm 'default', got %q", loaded.DefaultLLM)
	}
	if len(loaded.LLMProviders) != 1 {
		t.Fatalf("expected 1 synthesized provider, got %d", len(loaded.LLMProviders))
	}
	def, ok := loaded.LLMProviders["default"]
	if !ok {
		t.Fatal("expected llm_providers to contain 'default' key")
	}
	if def.Provider != "openai" {
		t.Errorf("expected provider 'openai', got %q", def.Provider)
	}
}

func TestLoadLLMProviders(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := map[string]interface{}{
		"llm_providers": map[string]interface{}{
			"claude": map[string]interface{}{
				"provider": "anthropic",
				"api_key":  "sk-ant-test",
				"model":    "claude-sonnet-4-5-20250929",
			},
			"gpt": map[string]interface{}{
				"provider": "openai",
				"api_key":  "sk-openai-test",
				"model":    "gpt-4o",
			},
		},
		"default_llm": "claude",
		"prompts": []map[string]interface{}{
			{"name": "Correct", "prompt": "Fix errors."},
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(path, data, 0644)

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(loaded.LLMProviders) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(loaded.LLMProviders))
	}
	if loaded.DefaultLLM != "claude" {
		t.Errorf("expected default_llm 'claude', got %q", loaded.DefaultLLM)
	}
}

func TestValidateInvalidLLMLabel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := map[string]interface{}{
		"llm_providers": map[string]interface{}{
			"claude": map[string]interface{}{
				"provider": "anthropic",
				"api_key":  "sk-ant-test",
				"model":    "claude-sonnet-4-5-20250929",
			},
		},
		"default_llm": "nonexistent",
		"prompts": []map[string]interface{}{
			{"name": "Correct", "prompt": "Fix errors."},
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(path, data, 0644)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error for invalid default_llm label")
	}
}

func TestMigrateOldFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	// Old format: prompts as a JSON object with correct/translate/rewrite_templates
	cfg := map[string]interface{}{
		"llm_provider": "ollama",
		"model":        "mistral",
		"active_mode":  "correct",
		"hotkeys": map[string]interface{}{
			"correct":        "Ctrl+G",
			"cycle_template": "Ctrl+T",
		},
		"prompts": map[string]interface{}{
			"correct":          "Fix all errors.",
			"translate":        "Translate between {language_a} and {language_b}.",
			"translate_single": "Translate to {target_language}.",
			"rewrite_templates": []map[string]interface{}{
				{"name": "funny", "prompt": "Make it funny."},
				{"name": "formal", "prompt": "Make it formal.", "llm": "gpt"},
			},
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(path, data, 0644)

	loaded, err := LoadRaw(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have migrated: correction prompt + 2 rewrite templates = 3 prompts
	if len(loaded.Prompts) != 3 {
		t.Fatalf("expected 3 migrated prompts, got %d", len(loaded.Prompts))
	}
	if loaded.Prompts[0].Name != "Correct" {
		t.Errorf("expected first prompt 'Correct', got '%s'", loaded.Prompts[0].Name)
	}
	if loaded.Prompts[0].Prompt != "Fix all errors." {
		t.Errorf("expected old correct prompt preserved, got '%s'", loaded.Prompts[0].Prompt)
	}
	if loaded.Prompts[1].Name != "funny" {
		t.Errorf("expected second prompt 'funny', got '%s'", loaded.Prompts[1].Name)
	}
	if loaded.Prompts[2].LLM != "gpt" {
		t.Errorf("expected formal template LLM 'gpt', got '%s'", loaded.Prompts[2].LLM)
	}

	// Hotkeys should be migrated
	if loaded.Hotkeys.Action != "Ctrl+G" {
		t.Errorf("expected action hotkey 'Ctrl+G', got '%s'", loaded.Hotkeys.Action)
	}
	if loaded.Hotkeys.CyclePrompt != "Ctrl+T" {
		t.Errorf("expected cycle_prompt hotkey 'Ctrl+T', got '%s'", loaded.Hotkeys.CyclePrompt)
	}
}

func TestNewFormatLoadsDirect(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := map[string]interface{}{
		"llm_provider": "ollama",
		"model":        "mistral",
		"prompts": []map[string]interface{}{
			{"name": "Correct", "prompt": "Fix errors."},
			{"name": "Polish", "prompt": "Polish the text."},
		},
		"hotkeys": map[string]interface{}{
			"action": "Ctrl+G",
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(path, data, 0644)

	loaded, err := LoadRaw(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(loaded.Prompts) != 2 {
		t.Fatalf("expected 2 prompts, got %d", len(loaded.Prompts))
	}
	if loaded.Prompts[0].Name != "Correct" {
		t.Errorf("expected 'Correct', got '%s'", loaded.Prompts[0].Name)
	}
	if loaded.Prompts[1].Name != "Polish" {
		t.Errorf("expected 'Polish', got '%s'", loaded.Prompts[1].Name)
	}
}

func TestActivePromptClamped(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := map[string]interface{}{
		"llm_provider":  "ollama",
		"model":         "mistral",
		"active_prompt": 99,
		"prompts": []map[string]interface{}{
			{"name": "Correct", "prompt": "Fix errors."},
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(path, data, 0644)

	loaded, err := LoadRaw(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if loaded.ActivePrompt != 0 {
		t.Errorf("expected active_prompt clamped to 0, got %d", loaded.ActivePrompt)
	}
}
