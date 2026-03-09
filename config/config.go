package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// PromptEntry defines a named prompt with an optional per-prompt LLM override.
type PromptEntry struct {
	Name   string `json:"name"`
	Prompt string `json:"prompt"`
	LLM    string `json:"llm,omitempty"`
}

// LLMProviderDef defines a named LLM provider configuration.
type LLMProviderDef struct {
	Provider    string `json:"provider"`
	APIKey      string `json:"api_key,omitempty"`
	Model       string `json:"model"`
	APIEndpoint string `json:"api_endpoint,omitempty"`
	MaxTokens   int    `json:"max_tokens,omitempty"`
	TimeoutMs   int    `json:"timeout_ms,omitempty"`
}

// Hotkeys defines the configurable hotkey bindings.
type Hotkeys struct {
	Action      string `json:"action"`
	CyclePrompt string `json:"cycle_prompt"`
}

// Overlay defines overlay display settings.
type Overlay struct {
	Enabled            bool    `json:"enabled"`
	Position           string  `json:"position"`
	Opacity            float64 `json:"opacity"`
	AutoDismissSeconds int     `json:"auto_dismiss_seconds"`
	ShowModeIndicator  bool    `json:"show_mode_indicator"`
	HighlightChanges   bool    `json:"highlight_changes"`
	FontSize           int     `json:"font_size"`
}

// Config is the top-level configuration for GhostType.
type Config struct {
	// Legacy flat fields — kept for backward-compat migration only.
	LLMProvider string `json:"llm_provider,omitempty"`
	APIKey      string `json:"api_key,omitempty"`
	Model       string `json:"model,omitempty"`
	APIEndpoint string `json:"api_endpoint,omitempty"`

	LLMProviders map[string]LLMProviderDef `json:"llm_providers,omitempty"`
	DefaultLLM   string                    `json:"default_llm,omitempty"`

	ActivePrompt int           `json:"active_prompt"`
	Prompts      []PromptEntry `json:"prompts"`
	Hotkeys      Hotkeys       `json:"hotkeys"`
	Overlay      Overlay       `json:"overlay"`

	MaxTokens         int    `json:"max_tokens"`
	TimeoutMs         int    `json:"timeout_ms"`
	MaxInputChars     int    `json:"max_input_chars"`
	PreserveClipboard bool   `json:"preserve_clipboard"`
	SoundEnabled      *bool  `json:"sound_enabled"`
	LogLevel          string `json:"log_level"`
	LogFile           string `json:"log_file"`
}

func boolPtr(v bool) *bool { return &v }

// Default prompt texts.
const (
	DefaultCorrectPrompt    = "Fix only spelling and grammar errors. Do not rewrite, rephrase, or restructure the sentence. Keep the text in its original language — never translate it. Preserve slang, abbreviations, acronyms, and informal tone exactly as written. Only fix what is objectively incorrect. Return ONLY the corrected text with no explanation."
	DefaultPolishPrompt     = "Improve the following text to make it clearer, more natural, and better structured while preserving its original meaning and tone. Fix grammar, punctuation, and awkward phrasing. Smooth out rough sentences into polished, ready-to-send prose. Keep the text in its original language — never translate it. Return ONLY the improved text with no explanation."
	DefaultFunnyPrompt      = "Rewrite the following text to be funny, witty, and entertaining while preserving the original meaning and key information. Add humor, clever wordplay, or a lighthearted twist. Keep the text in its original language — never translate it. Return ONLY the funny version with no explanation."
	DefaultElaboratePrompt  = "Expand the following text by adding relevant detail, context, and completeness while preserving the original meaning and intent. Flesh out terse or incomplete points into well-developed statements. Maintain the same tone and style as the original. Keep the text in its original language — never translate it. Return ONLY the elaborated text with no explanation."
	DefaultShortenPrompt    = "Condense the following text to be as concise as possible while preserving all essential meaning and key information. Remove redundancy, filler words, and unnecessary qualifiers. Keep the same tone and intent. Keep the text in its original language — never translate it. Return ONLY the shortened text with no explanation."
	DefaultTranslatePrompt  = "Translate the following text to English regardless of its source language. Return ONLY the translated text with no explanation."
)

// DefaultPrompts returns the default prompt list.
func DefaultPrompts() []PromptEntry {
	return []PromptEntry{
		{Name: "Correct", Prompt: DefaultCorrectPrompt},
		{Name: "Polish", Prompt: DefaultPolishPrompt},
		{Name: "Funny", Prompt: DefaultFunnyPrompt},
		{Name: "Elaborate", Prompt: DefaultElaboratePrompt},
		{Name: "Shorten", Prompt: DefaultShortenPrompt},
		{Name: "Translate", Prompt: DefaultTranslatePrompt},
	}
}

// DefaultConfig returns a Config populated with default values.
func DefaultConfig() Config {
	return Config{
		Hotkeys: Hotkeys{
			Action: defaultActionHotkey,
		},
		Prompts:      DefaultPrompts(),
		ActivePrompt: 0,
		Overlay: Overlay{
			Enabled:            true,
			Position:           "above_chat",
			Opacity:            0.85,
			AutoDismissSeconds: 10,
			ShowModeIndicator:  true,
			HighlightChanges:   true,
			FontSize:           14,
		},
		MaxTokens:         256,
		TimeoutMs:         30000,
		MaxInputChars:     2000,
		PreserveClipboard: true,
		SoundEnabled:      boolPtr(true),
		LogLevel:          "info",
		LogFile:           "ghosttype.log",
	}
}

// Load reads a config from the given JSON file path.
// If the file does not exist, it creates a default config file and returns the defaults.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cfg := DefaultConfig()
			if writeErr := WriteDefault(path, &cfg); writeErr != nil {
				return nil, fmt.Errorf("failed to create default config: %w", writeErr)
			}
			applyDefaults(&cfg)
			return &cfg, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg, err := unmarshalWithMigration(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	applyDefaults(cfg)

	if err := Validate(cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

// LoadRaw reads a config from the given JSON file path without validation.
// It applies defaults but skips Validate(), allowing the GUI wizard to run
// before an API key is configured.
func LoadRaw(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cfg := DefaultConfig()
			if writeErr := WriteDefault(path, &cfg); writeErr != nil {
				return nil, fmt.Errorf("failed to create default config: %w", writeErr)
			}
			applyDefaults(&cfg)
			return &cfg, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg, err := unmarshalWithMigration(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	applyDefaults(cfg)
	return cfg, nil
}

// unmarshalWithMigration parses config JSON, detecting and migrating old format
// where "prompts" was a JSON object (with correct/translate/rewrite_templates fields)
// to the new format where "prompts" is a JSON array of PromptEntry.
func unmarshalWithMigration(data []byte) (*Config, error) {
	// First, try parsing into a raw map to detect the prompts format.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	promptsRaw, hasPrompts := raw["prompts"]
	needsMigration := false

	if hasPrompts && len(promptsRaw) > 0 {
		// Trim whitespace to find the first meaningful character.
		trimmed := strings.TrimSpace(string(promptsRaw))
		if len(trimmed) > 0 && trimmed[0] == '{' {
			needsMigration = true
		}
	}

	if needsMigration {
		return migrateOldConfig(data)
	}

	// New format — parse directly.
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// oldPrompts represents the old prompts format.
type oldPrompts struct {
	Correct          string              `json:"correct"`
	Translate        string              `json:"translate"`
	TranslateSingle  string              `json:"translate_single"`
	RewriteTemplates []oldRewriteTemplate `json:"rewrite_templates"`
}

type oldRewriteTemplate struct {
	Name   string `json:"name"`
	Prompt string `json:"prompt"`
	LLM    string `json:"llm,omitempty"`
}

// oldConfig is a partial struct for deserializing the old format.
type oldConfig struct {
	LLMProvider  string                    `json:"llm_provider,omitempty"`
	APIKey       string                    `json:"api_key,omitempty"`
	Model        string                    `json:"model,omitempty"`
	APIEndpoint  string                    `json:"api_endpoint,omitempty"`
	LLMProviders map[string]LLMProviderDef `json:"llm_providers,omitempty"`
	DefaultLLM   string                    `json:"default_llm,omitempty"`
	ActiveMode   string                    `json:"active_mode"`
	OldHotkeys   struct {
		Correct        string `json:"correct"`
		Translate      string `json:"translate"`
		ToggleLanguage string `json:"toggle_language"`
		Rewrite        string `json:"rewrite"`
		CycleTemplate  string `json:"cycle_template"`
	} `json:"hotkeys"`
	Prompts           oldPrompts `json:"prompts"`
	Overlay           Overlay    `json:"overlay"`
	MaxTokens         int        `json:"max_tokens"`
	TimeoutMs         int        `json:"timeout_ms"`
	PreserveClipboard bool       `json:"preserve_clipboard"`
	SoundEnabled      *bool      `json:"sound_enabled"`
	LogLevel          string     `json:"log_level"`
	LogFile           string     `json:"log_file"`
}

// migrateOldConfig converts an old-format config to the new format.
func migrateOldConfig(data []byte) (*Config, error) {
	var old oldConfig
	if err := json.Unmarshal(data, &old); err != nil {
		return nil, fmt.Errorf("migration: %w", err)
	}

	slog.Info("Migrating config from old 3-mode format to unified prompts format")

	// Build new prompt list from old prompts.
	var prompts []PromptEntry

	// Correction prompt
	if old.Prompts.Correct != "" {
		prompts = append(prompts, PromptEntry{Name: "Correct", Prompt: old.Prompts.Correct})
	}

	// Rewrite templates become top-level prompts.
	for _, t := range old.Prompts.RewriteTemplates {
		prompts = append(prompts, PromptEntry{Name: t.Name, Prompt: t.Prompt, LLM: t.LLM})
	}

	// Map old active_mode to active_prompt index.
	activePrompt := 0
	// "correct" → 0 (default), others we leave at 0

	cfg := &Config{
		LLMProvider:   old.LLMProvider,
		APIKey:        old.APIKey,
		Model:         old.Model,
		APIEndpoint:   old.APIEndpoint,
		LLMProviders:  old.LLMProviders,
		DefaultLLM:    old.DefaultLLM,
		ActivePrompt:  activePrompt,
		Prompts:       prompts,
		Hotkeys: Hotkeys{
			Action:      old.OldHotkeys.Correct,
			CyclePrompt: old.OldHotkeys.CycleTemplate,
		},
		Overlay:           old.Overlay,
		MaxTokens:         old.MaxTokens,
		TimeoutMs:         old.TimeoutMs,
		PreserveClipboard: old.PreserveClipboard,
		SoundEnabled:      old.SoundEnabled,
		LogLevel:          old.LogLevel,
		LogFile:           old.LogFile,
	}

	return cfg, nil
}

// WriteDefault writes a default config file to the given path.
func WriteDefault(path string, cfg *Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// applyDefaults fills in zero-value fields with sensible defaults.
func applyDefaults(cfg *Config) {
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 256
	}
	// Migrate old default: 5000ms was too short for cloud LLM APIs and caused
	// frequent "context deadline exceeded" errors. Bump to 30000ms.
	if cfg.TimeoutMs == 0 || cfg.TimeoutMs == 5000 {
		cfg.TimeoutMs = 30000
	}
	// Normalize log_level to lowercase so "Debug", "INFO", etc. all work.
	cfg.LogLevel = strings.ToLower(strings.TrimSpace(cfg.LogLevel))
	// LogLevel: empty means disabled (no logging). No default applied.
	// LogFile: only default if logging is enabled.
	if cfg.LogLevel != "" && cfg.LogFile == "" {
		cfg.LogFile = "ghosttype.log"
	}
	if cfg.SoundEnabled == nil {
		cfg.SoundEnabled = boolPtr(true)
	}
	if cfg.Hotkeys.Action == "" {
		cfg.Hotkeys.Action = "Ctrl+G"
	}

	// Default prompts if empty.
	if len(cfg.Prompts) == 0 {
		cfg.Prompts = DefaultPrompts()
	}

	// Clamp active_prompt to valid range.
	if cfg.ActivePrompt < 0 || cfg.ActivePrompt >= len(cfg.Prompts) {
		cfg.ActivePrompt = 0
	}

	// Synthesize LLMProviders from legacy flat fields if not set.
	if len(cfg.LLMProviders) == 0 && cfg.LLMProvider != "" {
		if cfg.LLMProvider == "ollama" || cfg.APIKey != "" {
			cfg.LLMProviders = map[string]LLMProviderDef{
				"default": {
					Provider:    cfg.LLMProvider,
					APIKey:      cfg.APIKey,
					Model:       cfg.Model,
					APIEndpoint: cfg.APIEndpoint,
					MaxTokens:   cfg.MaxTokens,
					TimeoutMs:   cfg.TimeoutMs,
				},
			}
			if cfg.DefaultLLM == "" {
				cfg.DefaultLLM = "default"
			}
		}
	}

	// Fill missing TimeoutMs per provider from global value.
	for label, def := range cfg.LLMProviders {
		if def.TimeoutMs == 0 {
			if def.Provider == "ollama" {
				def.TimeoutMs = 120000
			} else {
				def.TimeoutMs = cfg.TimeoutMs
			}
		}
		cfg.LLMProviders[label] = def
	}
}

// Validate checks that the config has all required fields.
func Validate(cfg *Config) error {
	validProviders := map[string]bool{
		"anthropic": true,
		"openai":    true,
		"gemini":    true,
		"xai":       true,
		"deepseek":  true,
		"ollama":    true,
	}

	// Flat-field validation only when llm_providers was not provided directly.
	if cfg.LLMProvider != "" {
		if !validProviders[cfg.LLMProvider] {
			return fmt.Errorf("unsupported llm_provider: %s (valid: anthropic, openai, gemini, xai, ollama)", cfg.LLMProvider)
		}
		if cfg.LLMProvider != "ollama" && cfg.APIKey == "" {
			return fmt.Errorf("api_key is required for provider %s", cfg.LLMProvider)
		}
		if cfg.Model == "" {
			return fmt.Errorf("model is required")
		}
	} else if len(cfg.LLMProviders) == 0 {
		return fmt.Errorf("either llm_provider or llm_providers is required")
	}

	// DefaultLLM required when llm_providers is set.
	if len(cfg.LLMProviders) > 0 && cfg.DefaultLLM == "" {
		return fmt.Errorf("default_llm is required when llm_providers is defined")
	}

	// Validate each provider def in llm_providers.
	for label, def := range cfg.LLMProviders {
		if !validProviders[def.Provider] {
			return fmt.Errorf("llm_providers[%s]: unsupported provider %q (valid: anthropic, openai, gemini, xai, ollama)", label, def.Provider)
		}
		if def.Provider != "ollama" && def.APIKey == "" {
			return fmt.Errorf("llm_providers[%s]: api_key is required for provider %s", label, def.Provider)
		}
		if def.Model == "" {
			return fmt.Errorf("llm_providers[%s]: model is required", label)
		}
	}

	// Validate LLM label references point to defined providers.
	if len(cfg.LLMProviders) > 0 {
		checkLabel := func(field, label string) error {
			if label == "" {
				return nil
			}
			if _, ok := cfg.LLMProviders[label]; !ok {
				return fmt.Errorf("%s %q not found in llm_providers", field, label)
			}
			return nil
		}
		if err := checkLabel("default_llm", cfg.DefaultLLM); err != nil {
			return err
		}
		for _, p := range cfg.Prompts {
			if err := checkLabel(fmt.Sprintf("prompt[%s].llm", p.Name), p.LLM); err != nil {
				return err
			}
		}
	}

	// Must have at least one prompt.
	if len(cfg.Prompts) == 0 {
		return fmt.Errorf("at least one prompt is required")
	}
	for i, p := range cfg.Prompts {
		if p.Name == "" {
			return fmt.Errorf("prompts[%d]: name is required", i)
		}
		if p.Prompt == "" {
			return fmt.Errorf("prompts[%d]: prompt text is required", i)
		}
	}

	// Validate log_level if set.
	if cfg.LogLevel != "" {
		validLevels := map[string]bool{
			"debug": true,
			"info":  true,
			"warn":  true,
			"error": true,
		}
		if !validLevels[cfg.LogLevel] {
			return fmt.Errorf("unsupported log_level: %q (valid: debug, info, warn, error, or empty to disable)", cfg.LogLevel)
		}
	}

	return nil
}
