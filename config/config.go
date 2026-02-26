package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// RewriteTemplate defines a named rewrite prompt template.
type RewriteTemplate struct {
	Name   string `json:"name"`
	Prompt string `json:"prompt"`
}

// Hotkeys defines the configurable hotkey bindings.
type Hotkeys struct {
	Correct        string `json:"correct"`
	Translate      string `json:"translate"`
	ToggleLanguage string `json:"toggle_language"`
	Rewrite        string `json:"rewrite"`
	CycleTemplate  string `json:"cycle_template"`
	Cancel         string `json:"cancel"`
}

// Prompts defines the LLM prompt configuration.
type Prompts struct {
	Correct          string            `json:"correct"`
	Translate        string            `json:"translate"`
	RewriteTemplates []RewriteTemplate `json:"rewrite_templates"`
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
	LLMProvider            string            `json:"llm_provider"`
	APIKey                 string            `json:"api_key"`
	Model                  string            `json:"model"`
	APIEndpoint            string            `json:"api_endpoint"`
	Languages              []string          `json:"languages"`
	LanguageNames          map[string]string `json:"language_names"`
	DefaultTranslateTarget string            `json:"default_translate_target"`
	Hotkeys                Hotkeys           `json:"hotkeys"`
	Prompts                Prompts           `json:"prompts"`
	Overlay                Overlay           `json:"overlay"`
	MaxTokens              int               `json:"max_tokens"`
	TimeoutMs              int               `json:"timeout_ms"`
	PreserveClipboard      bool              `json:"preserve_clipboard"`
	LogLevel               string            `json:"log_level"`
	LogFile                string            `json:"log_file"`
}

// DefaultConfig returns a Config populated with default values.
func DefaultConfig() Config {
	return Config{
		LLMProvider: "anthropic",
		APIKey:      "",
		Model:       "claude-sonnet-4-5-20250929",
		APIEndpoint: "",
		Languages:   []string{"en", "fr"},
		LanguageNames: map[string]string{
			"en": "English",
			"fr": "French",
		},
		DefaultTranslateTarget: "en",
		Hotkeys: Hotkeys{
			Correct:        "Ctrl+G",
			Translate:      "Ctrl+J",
			ToggleLanguage: "Ctrl+F8",
			Rewrite:        "F9",
			CycleTemplate:  "Ctrl+F9",
			Cancel:         "Escape",
		},
		Prompts: Prompts{
			Correct:   "Detect the language of the following text (French or English). Fix all spelling and grammar errors while preserving the original meaning and language. Return ONLY the corrected text with no explanation.",
			Translate: "The two configured languages are {language_a} and {language_b}. Detect the language of the following text and translate it to the OTHER language. If it is {language_a}, translate to {language_b}. If it is {language_b}, translate to {language_a}. Preserve the tone and intent. Return ONLY the translation with no explanation.",
			RewriteTemplates: []RewriteTemplate{
				{Name: "funny", Prompt: "Rewrite this as a funny, witty reply. Keep it short and punchy. Return ONLY the rewritten text."},
				{Name: "formal", Prompt: "Rewrite this in a formal, professional tone. Return ONLY the rewritten text."},
				{Name: "sarcastic", Prompt: "Rewrite this with heavy sarcasm. Return ONLY the rewritten text."},
				{Name: "flirty", Prompt: "Rewrite this in a playful, flirty tone. Return ONLY the rewritten text."},
				{Name: "poetic", Prompt: "Rewrite this as if you were a romantic poet. Return ONLY the rewritten text."},
			},
		},
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
		TimeoutMs:         5000,
		PreserveClipboard: true,
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
			return &cfg, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	applyDefaults(&cfg)

	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
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
	if cfg.TimeoutMs == 0 {
		cfg.TimeoutMs = 5000
	}
	// LogLevel: empty means disabled (no logging). No default applied.
	// LogFile: only default if logging is enabled.
	if cfg.LogLevel != "" && cfg.LogFile == "" {
		cfg.LogFile = "ghosttype.log"
	}
	if cfg.Hotkeys.Correct == "" {
		cfg.Hotkeys.Correct = "Ctrl+G"
	}
	if cfg.Hotkeys.Translate == "" {
		cfg.Hotkeys.Translate = "Ctrl+J"
	}
	if cfg.Hotkeys.ToggleLanguage == "" {
		cfg.Hotkeys.ToggleLanguage = "Ctrl+F8"
	}
	if cfg.Hotkeys.Rewrite == "" {
		cfg.Hotkeys.Rewrite = "F9"
	}
	if cfg.Hotkeys.CycleTemplate == "" {
		cfg.Hotkeys.CycleTemplate = "Ctrl+F9"
	}
	if cfg.Hotkeys.Cancel == "" {
		cfg.Hotkeys.Cancel = "Escape"
	}
	if len(cfg.Languages) == 0 {
		cfg.Languages = []string{"en", "fr"}
	}
	if cfg.LanguageNames == nil {
		cfg.LanguageNames = map[string]string{"en": "English", "fr": "French"}
	}
}

// validate checks that the config has all required fields.
func validate(cfg *Config) error {
	if cfg.LLMProvider == "" {
		return fmt.Errorf("llm_provider is required")
	}

	validProviders := map[string]bool{
		"anthropic": true,
		"openai":    true,
		"gemini":    true,
		"xai":       true,
		"ollama":    true,
	}
	if !validProviders[cfg.LLMProvider] {
		return fmt.Errorf("unsupported llm_provider: %s (valid: anthropic, openai, gemini, xai, ollama)", cfg.LLMProvider)
	}

	// API key is required for all providers except ollama
	if cfg.LLMProvider != "ollama" && cfg.APIKey == "" {
		return fmt.Errorf("api_key is required for provider %s", cfg.LLMProvider)
	}

	if cfg.Model == "" {
		return fmt.Errorf("model is required")
	}

	if cfg.Prompts.Correct == "" {
		return fmt.Errorf("prompts.correct is required")
	}

	return nil
}
