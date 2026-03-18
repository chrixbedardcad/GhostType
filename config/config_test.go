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
	if len(cfg.Prompts) != 8 {
		t.Errorf("expected 8 default prompts, got %d", len(cfg.Prompts))
	}
	expectedNames := []string{"Correct", "Polish", "Funny", "Elaborate", "Shorten", "Translate"}
	for i, name := range expectedNames {
		if i < len(cfg.Prompts) && cfg.Prompts[i].Name != name {
			t.Errorf("expected prompt %d '%s', got '%s'", i, name, cfg.Prompts[i].Name)
		}
	}
	if cfg.ActivePrompt != 0 {
		t.Errorf("expected default active_prompt 0, got %d", cfg.ActivePrompt)
	}
	if !cfg.PreserveClipboard {
		t.Error("expected preserve_clipboard to be true by default")
	}
}

func TestDefaultConfigHasEmptyMaps(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Providers == nil {
		t.Error("expected Providers to be initialized (not nil)")
	}
	if len(cfg.Providers) != 0 {
		t.Errorf("expected empty Providers map, got %d entries", len(cfg.Providers))
	}
	if cfg.Models == nil {
		t.Error("expected Models to be initialized (not nil)")
	}
	if len(cfg.Models) != 0 {
		t.Errorf("expected empty Models map, got %d entries", len(cfg.Models))
	}
}

func TestNeedsSetupNoProviders(t *testing.T) {
	cfg := DefaultConfig()
	if !NeedsSetup(&cfg) {
		t.Error("expected NeedsSetup=true with no providers")
	}
}

func TestNeedsSetupWithProvider(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Providers["openai"] = ProviderConfig{APIKey: "sk-test"}
	if NeedsSetup(&cfg) {
		t.Error("expected NeedsSetup=false with at least one provider")
	}
}

func TestLoadCreatesDefaultWhenMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg, err := LoadRaw(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Prompts) != 8 {
		t.Errorf("expected 8 default prompts, got %d", len(cfg.Prompts))
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
		"providers": map[string]interface{}{
			"openai": map[string]interface{}{
				"api_key": "sk-test-key-12345",
			},
		},
		"models": map[string]interface{}{
			"gpt4o": map[string]interface{}{
				"provider": "openai",
				"model":    "gpt-4o",
			},
		},
		"default_model": "gpt4o",
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

	if _, ok := loaded.Providers["openai"]; !ok {
		t.Error("expected 'openai' in providers")
	}
	if loaded.Providers["openai"].APIKey != "sk-test-key-12345" {
		t.Errorf("expected api_key, got '%s'", loaded.Providers["openai"].APIKey)
	}
	if len(loaded.Prompts) != 2 {
		t.Fatalf("expected 2 prompts, got %d", len(loaded.Prompts))
	}
	if loaded.Prompts[0].Name != "Correct" {
		t.Errorf("expected prompt name 'Correct', got '%s'", loaded.Prompts[0].Name)
	}
	if loaded.DefaultModel != "gpt4o" {
		t.Errorf("expected default_model 'gpt4o', got '%s'", loaded.DefaultModel)
	}
}

func TestLoadInvalidProvider(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := map[string]interface{}{
		"providers": map[string]interface{}{
			"invalid_provider": map[string]interface{}{
				"api_key": "sk-test",
			},
		},
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

func TestLoadNoProviders(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := map[string]interface{}{
		"prompts": []map[string]interface{}{
			{"name": "Correct", "prompt": "Fix errors."},
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(path, data, 0644)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error for no providers")
	}
}

func TestLoadOllamaNoAPIKeyRequired(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := map[string]interface{}{
		"providers": map[string]interface{}{
			"ollama": map[string]interface{}{},
		},
		"models": map[string]interface{}{
			"mistral": map[string]interface{}{
				"provider": "ollama",
				"model":    "mistral",
			},
		},
		"default_model": "mistral",
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

	if _, ok := loaded.Providers["ollama"]; !ok {
		t.Error("expected 'ollama' in providers")
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
		"providers": map[string]interface{}{
			"openai": map[string]interface{}{
				"api_key": "sk-test",
			},
		},
		"models": map[string]interface{}{
			"gpt4o": map[string]interface{}{
				"provider": "openai",
				"model":    "gpt-4o",
			},
		},
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
		"providers": map[string]interface{}{
			"openai": map[string]interface{}{
				"api_key": "sk-test",
			},
		},
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
		"providers": map[string]interface{}{
			"openai": map[string]interface{}{
				"api_key": "sk-test",
			},
		},
		"log_level": "debug",
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
	if loaded.LogFile != "ghostspell.log" {
		t.Errorf("expected default log_file 'ghostspell.log', got '%s'", loaded.LogFile)
	}
}

func TestLoadInvalidLogLevel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := map[string]interface{}{
		"providers": map[string]interface{}{
			"openai": map[string]interface{}{
				"api_key": "sk-test",
			},
		},
		"log_level": "verbose",
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
		"providers": map[string]interface{}{
			"openai": map[string]interface{}{
				"api_key": "sk-test",
			},
		},
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
	// CyclePrompt should have default
	if loaded.Hotkeys.CyclePrompt != "Ctrl+Shift+T" {
		t.Errorf("expected cycle_prompt hotkey 'Ctrl+Shift+T', got '%s'", loaded.Hotkeys.CyclePrompt)
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
	if len(loaded.Prompts) != 8 {
		t.Errorf("expected 8 prompts in written file, got %d", len(loaded.Prompts))
	}
}

func TestMarshalUnmarshalRoundTrip(t *testing.T) {
	cfg := Config{
		Providers: map[string]ProviderConfig{
			"anthropic": {APIKey: "sk-ant-test"},
			"openai":    {APIKey: "sk-openai-test"},
		},
		Models: map[string]ModelEntry{
			"claude": {Provider: "anthropic", Model: "claude-sonnet-4-5-20250929", MaxTokens: 512},
			"gpt4o":  {Provider: "openai", Model: "gpt-4o"},
		},
		DefaultModel: "claude",
		Prompts: []PromptEntry{
			{Name: "Correct", Prompt: "Fix errors."},
		},
		Hotkeys:   Hotkeys{Action: "Ctrl+G"},
		MaxTokens: 256,
		TimeoutMs: 30000,
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var loaded Config
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(loaded.Providers) != 2 {
		t.Errorf("expected 2 providers, got %d", len(loaded.Providers))
	}
	if len(loaded.Models) != 2 {
		t.Errorf("expected 2 models, got %d", len(loaded.Models))
	}
	if loaded.DefaultModel != "claude" {
		t.Errorf("expected default_model 'claude', got %q", loaded.DefaultModel)
	}
	me, ok := loaded.Models["claude"]
	if !ok {
		t.Fatal("expected 'claude' model entry")
	}
	if me.Provider != "anthropic" {
		t.Errorf("expected provider 'anthropic', got %q", me.Provider)
	}
	if me.Model != "claude-sonnet-4-5-20250929" {
		t.Errorf("expected model 'claude-sonnet-4-5-20250929', got %q", me.Model)
	}
	if me.MaxTokens != 512 {
		t.Errorf("expected max_tokens 512, got %d", me.MaxTokens)
	}
}

func TestLLMProviderDefStandalone(t *testing.T) {
	// LLMProviderDef is still used by the LLM package as a merge type.
	def := LLMProviderDef{
		Provider:     "openai",
		APIKey:       "sk-test",
		Model:        "gpt-4o",
		APIEndpoint:  "https://api.openai.com/v1",
		MaxTokens:    1024,
		TimeoutMs:    30000,
		RefreshToken: "rt-test",
		KeepAlive:    true,
	}

	data, err := json.Marshal(def)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var loaded LLMProviderDef
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if loaded.Provider != "openai" {
		t.Errorf("expected provider 'openai', got %q", loaded.Provider)
	}
	if loaded.APIKey != "sk-test" {
		t.Errorf("expected api_key 'sk-test', got %q", loaded.APIKey)
	}
	if loaded.Model != "gpt-4o" {
		t.Errorf("expected model 'gpt-4o', got %q", loaded.Model)
	}
	if loaded.RefreshToken != "rt-test" {
		t.Errorf("expected refresh_token 'rt-test', got %q", loaded.RefreshToken)
	}
	if !loaded.KeepAlive {
		t.Error("expected keep_alive true")
	}
}

func TestMigrateLegacyFlatFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	// Old flat-field format with llm_provider + api_key
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

	loaded, err := LoadRaw(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have migrated to new providers+models format.
	if len(loaded.Providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(loaded.Providers))
	}
	prov, ok := loaded.Providers["openai"]
	if !ok {
		t.Fatal("expected 'openai' in providers")
	}
	if prov.APIKey != "sk-test-key" {
		t.Errorf("expected api_key 'sk-test-key', got %q", prov.APIKey)
	}

	if loaded.DefaultModel != "default" {
		t.Errorf("expected default_model 'default', got %q", loaded.DefaultModel)
	}
	me, ok := loaded.Models["default"]
	if !ok {
		t.Fatal("expected 'default' model entry")
	}
	if me.Provider != "openai" {
		t.Errorf("expected provider 'openai', got %q", me.Provider)
	}
	if me.Model != "gpt-4o" {
		t.Errorf("expected model 'gpt-4o', got %q", me.Model)
	}
}

func TestMigrateLegacyLLMProviders(t *testing.T) {
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

	loaded, err := LoadRaw(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have migrated to new providers+models.
	if len(loaded.Providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(loaded.Providers))
	}
	if _, ok := loaded.Providers["anthropic"]; !ok {
		t.Error("expected 'anthropic' in providers")
	}
	if _, ok := loaded.Providers["openai"]; !ok {
		t.Error("expected 'openai' in providers")
	}

	if len(loaded.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(loaded.Models))
	}
	if loaded.DefaultModel != "claude" {
		t.Errorf("expected default_model 'claude', got %q", loaded.DefaultModel)
	}

	me, ok := loaded.Models["claude"]
	if !ok {
		t.Fatal("expected 'claude' model entry")
	}
	if me.Provider != "anthropic" {
		t.Errorf("expected provider 'anthropic', got %q", me.Provider)
	}
}

func TestValidateInvalidDefaultModel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := map[string]interface{}{
		"providers": map[string]interface{}{
			"anthropic": map[string]interface{}{
				"api_key": "sk-ant-test",
			},
		},
		"models": map[string]interface{}{
			"claude": map[string]interface{}{
				"provider": "anthropic",
				"model":    "claude-sonnet-4-5-20250929",
			},
		},
		"default_model": "nonexistent",
		"prompts": []map[string]interface{}{
			{"name": "Correct", "prompt": "Fix errors."},
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(path, data, 0644)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error for invalid default_model label")
	}
}

func TestValidateModelReferencesProvider(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := map[string]interface{}{
		"providers": map[string]interface{}{
			"openai": map[string]interface{}{
				"api_key": "sk-test",
			},
		},
		"models": map[string]interface{}{
			"claude": map[string]interface{}{
				"provider": "anthropic",
				"model":    "claude-sonnet-4-5-20250929",
			},
		},
		"prompts": []map[string]interface{}{
			{"name": "Correct", "prompt": "Fix errors."},
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(path, data, 0644)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error: model references provider not in providers map")
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

	// Should have migrated: correction prompt + 2 rewrite templates + Define = 4 prompts
	if len(loaded.Prompts) != 4 {
		t.Fatalf("expected 4 migrated prompts, got %d", len(loaded.Prompts))
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

	// LLM fields should have been migrated to providers+models
	if _, ok := loaded.Providers["ollama"]; !ok {
		t.Error("expected 'ollama' in providers after old config migration")
	}
	if loaded.DefaultModel != "default" {
		t.Errorf("expected default_model 'default', got %q", loaded.DefaultModel)
	}
}

func TestNewFormatLoadsDirect(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := map[string]interface{}{
		"providers": map[string]interface{}{
			"ollama": map[string]interface{}{},
		},
		"models": map[string]interface{}{
			"mistral": map[string]interface{}{
				"provider": "ollama",
				"model":    "mistral",
			},
		},
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

	if len(loaded.Prompts) != 3 {
		t.Fatalf("expected 3 prompts, got %d", len(loaded.Prompts))
	}
	if loaded.Prompts[0].Name != "Correct" {
		t.Errorf("expected 'Correct', got '%s'", loaded.Prompts[0].Name)
	}
	if loaded.Prompts[1].Name != "Polish" {
		t.Errorf("expected 'Polish', got '%s'", loaded.Prompts[1].Name)
	}
	if loaded.Prompts[2].Name != "Define" {
		t.Errorf("expected 'Define', got '%s'", loaded.Prompts[2].Name)
	}
}

func TestActivePromptClamped(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := map[string]interface{}{
		"providers": map[string]interface{}{
			"ollama": map[string]interface{}{},
		},
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

func TestModelTimeoutInheritsFromProvider(t *testing.T) {
	cfg := Config{
		Providers: map[string]ProviderConfig{
			"openai": {APIKey: "sk-test", TimeoutMs: 45000},
		},
		Models: map[string]ModelEntry{
			"gpt4o": {Provider: "openai", Model: "gpt-4o"},
		},
		Prompts: []PromptEntry{
			{Name: "Correct", Prompt: "Fix errors."},
		},
		TimeoutMs: 30000,
	}

	applyDefaults(&cfg)

	me := cfg.Models["gpt4o"]
	if me.TimeoutMs != 45000 {
		t.Errorf("expected model timeout 45000 (from provider), got %d", me.TimeoutMs)
	}
}

func TestModelTimeoutDefaultsOllama(t *testing.T) {
	cfg := Config{
		Providers: map[string]ProviderConfig{
			"ollama": {},
		},
		Models: map[string]ModelEntry{
			"mistral": {Provider: "ollama", Model: "mistral"},
		},
		Prompts: []PromptEntry{
			{Name: "Correct", Prompt: "Fix errors."},
		},
	}

	applyDefaults(&cfg)

	me := cfg.Models["mistral"]
	if me.TimeoutMs != 120000 {
		t.Errorf("expected ollama model timeout 120000, got %d", me.TimeoutMs)
	}
}
