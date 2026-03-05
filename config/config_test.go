package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Legacy flat fields are empty — providers are configured via llm_providers map.
	if cfg.LLMProvider != "" {
		t.Errorf("expected default provider empty (wizard handles setup), got '%s'", cfg.LLMProvider)
	}
	if cfg.Model != "" {
		t.Errorf("expected default model empty (wizard handles setup), got '%s'", cfg.Model)
	}
	if cfg.MaxTokens != 256 {
		t.Errorf("expected default max_tokens 256, got %d", cfg.MaxTokens)
	}
	if cfg.TimeoutMs != 30000 {
		t.Errorf("expected default timeout_ms 30000, got %d", cfg.TimeoutMs)
	}
	if cfg.ActiveMode != "correct" {
		t.Errorf("expected default active_mode 'correct', got '%s'", cfg.ActiveMode)
	}
	if cfg.Hotkeys.Correct != "Ctrl+G" {
		t.Errorf("expected default correct hotkey 'Ctrl+G', got '%s'", cfg.Hotkeys.Correct)
	}
	if cfg.Hotkeys.Translate != "" {
		t.Errorf("expected default translate hotkey empty, got '%s'", cfg.Hotkeys.Translate)
	}
	if cfg.Hotkeys.Rewrite != "" {
		t.Errorf("expected default rewrite hotkey empty, got '%s'", cfg.Hotkeys.Rewrite)
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

	cfg, err := LoadRaw(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Default config has empty flat fields — wizard sets up llm_providers.
	if cfg.LLMProvider != "" {
		t.Errorf("expected empty default provider, got '%s'", cfg.LLMProvider)
	}
	if cfg.ActiveMode != "correct" {
		t.Errorf("expected default active_mode 'correct', got '%s'", cfg.ActiveMode)
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
	if loaded.TimeoutMs != 30000 {
		t.Errorf("expected default timeout_ms 30000, got %d", loaded.TimeoutMs)
	}
	if loaded.LogLevel != "" {
		t.Errorf("expected empty log_level (disabled by default), got '%s'", loaded.LogLevel)
	}
}

func TestLoadLoggingDisabledByDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := map[string]interface{}{
		"llm_provider": "openai",
		"api_key":      "sk-test",
		"model":        "gpt-4o",
		"prompts":      map[string]interface{}{"correct": "Fix errors."},
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
		"prompts":      map[string]interface{}{"correct": "Fix errors."},
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

func TestLoadLogLevelCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := map[string]interface{}{
		"llm_provider": "openai",
		"api_key":      "sk-test",
		"model":        "gpt-4o",
		"log_level":    "DEBUG",
		"prompts":      map[string]interface{}{"correct": "Fix errors."},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(path, data, 0644)

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if loaded.LogLevel != "debug" {
		t.Errorf("expected normalized log_level 'debug', got '%s'", loaded.LogLevel)
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
		"prompts":      map[string]interface{}{"correct": "Fix errors."},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(path, data, 0644)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error for invalid log_level")
	}
}

func TestLoadActiveModeDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := map[string]interface{}{
		"llm_provider": "openai",
		"api_key":      "sk-test",
		"model":        "gpt-4o",
		"prompts":      map[string]interface{}{"correct": "Fix errors."},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(path, data, 0644)

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if loaded.ActiveMode != "correct" {
		t.Errorf("expected default active_mode 'correct', got '%s'", loaded.ActiveMode)
	}
}

func TestLoadActiveModeCustom(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := map[string]interface{}{
		"llm_provider": "openai",
		"api_key":      "sk-test",
		"model":        "gpt-4o",
		"active_mode":  "translate",
		"prompts":      map[string]interface{}{"correct": "Fix errors."},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(path, data, 0644)

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if loaded.ActiveMode != "translate" {
		t.Errorf("expected active_mode 'translate', got '%s'", loaded.ActiveMode)
	}
}

func TestLoadActiveModeInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := map[string]interface{}{
		"llm_provider": "openai",
		"api_key":      "sk-test",
		"model":        "gpt-4o",
		"active_mode":  "invalid",
		"prompts":      map[string]interface{}{"correct": "Fix errors."},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(path, data, 0644)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error for invalid active_mode")
	}
}

func TestLoadOptionalHotkeysEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := map[string]interface{}{
		"llm_provider": "openai",
		"api_key":      "sk-test",
		"model":        "gpt-4o",
		"prompts":      map[string]interface{}{"correct": "Fix errors."},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(path, data, 0644)

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only action (correct) should have a default
	if loaded.Hotkeys.Correct != "Ctrl+G" {
		t.Errorf("expected action hotkey 'Ctrl+G', got '%s'", loaded.Hotkeys.Correct)
	}
	// Optional hotkeys should remain empty
	if loaded.Hotkeys.Translate != "" {
		t.Errorf("expected translate hotkey empty, got '%s'", loaded.Hotkeys.Translate)
	}
	if loaded.Hotkeys.Rewrite != "" {
		t.Errorf("expected rewrite hotkey empty, got '%s'", loaded.Hotkeys.Rewrite)
	}
	if loaded.Hotkeys.ToggleLanguage != "" {
		t.Errorf("expected toggle_language hotkey empty, got '%s'", loaded.Hotkeys.ToggleLanguage)
	}
	if loaded.Hotkeys.CycleTemplate != "" {
		t.Errorf("expected cycle_template hotkey empty, got '%s'", loaded.Hotkeys.CycleTemplate)
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

	// Default config writes empty flat fields — wizard populates llm_providers.
	if loaded.ActiveMode != "correct" {
		t.Errorf("expected active_mode 'correct' in written file, got '%s'", loaded.ActiveMode)
	}
	if loaded.Hotkeys.Correct != "Ctrl+G" {
		t.Errorf("expected hotkey 'Ctrl+G' in written file, got '%s'", loaded.Hotkeys.Correct)
	}
}

func TestLLMProvidersSynthesizedFromFlat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := map[string]interface{}{
		"llm_provider": "openai",
		"api_key":      "sk-test-key",
		"model":        "gpt-4o",
		"prompts":      map[string]interface{}{"correct": "Fix errors."},
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
	if def.APIKey != "sk-test-key" {
		t.Errorf("expected api_key 'sk-test-key', got %q", def.APIKey)
	}
	if def.Model != "gpt-4o" {
		t.Errorf("expected model 'gpt-4o', got %q", def.Model)
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
		"prompts":     map[string]interface{}{"correct": "Fix errors."},
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
	if loaded.LLMProviders["gpt"].Model != "gpt-4o" {
		t.Errorf("expected gpt model 'gpt-4o', got %q", loaded.LLMProviders["gpt"].Model)
	}
}

func TestCorrectPromptLanguagesSubstitution(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := map[string]interface{}{
		"llm_provider": "ollama",
		"model":        "mistral",
		"languages":    []string{"en", "fr", "es"},
		"language_names": map[string]string{
			"en": "English",
			"fr": "French",
			"es": "Spanish",
		},
		"prompts": map[string]interface{}{
			"correct": "Fix errors in {languages}.",
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(path, data, 0644)

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "Fix errors in English or French or Spanish."
	if loaded.Prompts.Correct != expected {
		t.Errorf("expected prompt %q, got %q", expected, loaded.Prompts.Correct)
	}
}

func TestCustomCorrectPromptNotOverwritten(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := map[string]interface{}{
		"llm_provider": "ollama",
		"model":        "mistral",
		"prompts": map[string]interface{}{
			"correct": "Just fix the text please.",
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(path, data, 0644)

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if loaded.Prompts.Correct != "Just fix the text please." {
		t.Errorf("expected custom prompt unchanged, got %q", loaded.Prompts.Correct)
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
		"prompts":     map[string]interface{}{"correct": "Fix errors."},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(path, data, 0644)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error for invalid default_llm label")
	}
}
