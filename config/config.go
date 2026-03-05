package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// TranslateTarget represents a parsed translate target.
// A pair (e.g. "en|fr") has both LangA and LangB set.
// A single target (e.g. "es") has only LangA set.
type TranslateTarget struct {
	LangA string // always set
	LangB string // empty for single-target mode
}

// IsPair returns true if this target is a bilingual pair.
func (t TranslateTarget) IsPair() bool { return t.LangB != "" }

// RewriteTemplate defines a named rewrite prompt template.
type RewriteTemplate struct {
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
	Correct        string `json:"correct"`
	Translate      string `json:"translate"`
	ToggleLanguage string `json:"toggle_language"`
	Rewrite        string `json:"rewrite"`
	CycleTemplate  string `json:"cycle_template"`
}

// Prompts defines the LLM prompt configuration.
type Prompts struct {
	Correct          string            `json:"correct"`
	Translate        string            `json:"translate"`
	TranslateSingle  string            `json:"translate_single"`
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
	APIEndpoint            string                     `json:"api_endpoint"`
	LLMProviders           map[string]LLMProviderDef  `json:"llm_providers,omitempty"`
	DefaultLLM             string                     `json:"default_llm,omitempty"`
	CorrectLLM             string                     `json:"correct_llm,omitempty"`
	TranslateLLM           string                     `json:"translate_llm,omitempty"`
	Languages              []string                   `json:"languages"`
	LanguageNames          map[string]string `json:"language_names"`
	TranslateTargets       []string          `json:"translate_targets"`
	ParsedTargets          []TranslateTarget `json:"-"`
	DefaultTranslateTarget string            `json:"default_translate_target"`
	ActiveMode             string            `json:"active_mode"`
	Hotkeys                Hotkeys           `json:"hotkeys"`
	Prompts                Prompts           `json:"prompts"`
	Overlay                Overlay           `json:"overlay"`
	MaxTokens              int               `json:"max_tokens"`
	TimeoutMs              int               `json:"timeout_ms"`
	PreserveClipboard      bool              `json:"preserve_clipboard"`
	SoundEnabled           *bool             `json:"sound_enabled"`
	LogLevel               string            `json:"log_level"`
	LogFile                string            `json:"log_file"`
}

func boolPtr(v bool) *bool { return &v }

// DefaultConfig returns a Config populated with default values.
func DefaultConfig() Config {
	return Config{
		LLMProvider: "",
		APIKey:      "",
		Model:       "",
		APIEndpoint: "",
		Languages:   []string{"en", "fr"},
		LanguageNames: map[string]string{
			"en": "English",
			"fr": "French",
		},
		DefaultTranslateTarget: "en",
		ActiveMode:             "correct",
		Hotkeys: Hotkeys{
			Correct:        "Ctrl+G",
			Translate:      "",
			ToggleLanguage: "",
			Rewrite:        "",
			CycleTemplate:  "",
		},
		Prompts: Prompts{
			Correct:         "Detect the language of the following text ({languages}). Fix all spelling and grammar errors while preserving the original meaning and language. Return ONLY the corrected text with no explanation.",
			Translate:       "The two configured languages are {language_a} and {language_b}. Detect the language of the following text and translate it to the OTHER language. If it is {language_a}, translate to {language_b}. If it is {language_b}, translate to {language_a}. Preserve the tone and intent. Return ONLY the translation with no explanation.",
			TranslateSingle: "Translate the following text to {target_language}. Preserve the tone and intent. Return ONLY the translation with no explanation.",
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
		TimeoutMs:         30000,
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

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	applyDefaults(&cfg)

	if err := Validate(&cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
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

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	applyDefaults(&cfg)
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
	if cfg.ActiveMode == "" {
		cfg.ActiveMode = "correct"
	}
	if cfg.Hotkeys.Correct == "" {
		cfg.Hotkeys.Correct = "Ctrl+G"
	}
	// Translate, ToggleLanguage, Rewrite, CycleTemplate default to empty
	// (not registered). Users can add dedicated hotkeys in config if desired.
	if len(cfg.Languages) == 0 {
		cfg.Languages = []string{"en", "fr"}
	}
	if cfg.LanguageNames == nil {
		cfg.LanguageNames = map[string]string{"en": "English", "fr": "French"}
	}

	// Derive translate_targets from languages if not explicitly set.
	if len(cfg.TranslateTargets) == 0 {
		if len(cfg.Languages) >= 2 {
			cfg.TranslateTargets = []string{cfg.Languages[0] + "|" + cfg.Languages[1]}
		} else if len(cfg.Languages) == 1 {
			cfg.TranslateTargets = []string{cfg.Languages[0]}
		}
	}

	// Parse translate_targets into ParsedTargets.
	cfg.ParsedTargets = make([]TranslateTarget, len(cfg.TranslateTargets))
	for i, raw := range cfg.TranslateTargets {
		parts := strings.SplitN(raw, "|", 2)
		cfg.ParsedTargets[i].LangA = strings.TrimSpace(parts[0])
		if len(parts) == 2 {
			cfg.ParsedTargets[i].LangB = strings.TrimSpace(parts[1])
		}
	}

	// Synthesize LLMProviders from legacy flat fields if not set.
	if len(cfg.LLMProviders) == 0 && cfg.LLMProvider != "" {
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

	// Fill missing TimeoutMs per provider from global value.
	// MaxTokens is NOT filled here — each LLM constructor applies its own
	// sensible default (e.g. 2048 for OpenAI reasoning models vs 256 for others).
	for label, def := range cfg.LLMProviders {
		if def.TimeoutMs == 0 {
			if def.Provider == "ollama" {
				// Ollama needs a longer timeout: the first request loads the
				// model into memory which can take 30-60+ seconds.
				def.TimeoutMs = 120000
			} else {
				def.TimeoutMs = cfg.TimeoutMs
			}
		}
		cfg.LLMProviders[label] = def
	}

	// Substitute {languages} in the correct prompt using configured language names.
	if strings.Contains(cfg.Prompts.Correct, "{languages}") {
		names := make([]string, 0, len(cfg.Languages))
		for _, code := range cfg.Languages {
			if name, ok := cfg.LanguageNames[code]; ok {
				names = append(names, name)
			} else {
				names = append(names, code)
			}
		}
		cfg.Prompts.Correct = strings.ReplaceAll(cfg.Prompts.Correct, "{languages}", strings.Join(names, " or "))
	}
}

// Validate checks that the config has all required fields.
func Validate(cfg *Config) error {
	validProviders := map[string]bool{
		"anthropic": true,
		"openai":    true,
		"gemini":    true,
		"xai":       true,
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
		if err := checkLabel("correct_llm", cfg.CorrectLLM); err != nil {
			return err
		}
		if err := checkLabel("translate_llm", cfg.TranslateLLM); err != nil {
			return err
		}
		for _, tmpl := range cfg.Prompts.RewriteTemplates {
			if err := checkLabel(fmt.Sprintf("rewrite_template[%s].llm", tmpl.Name), tmpl.LLM); err != nil {
				return err
			}
		}
	}

	if cfg.Prompts.Correct == "" {
		return fmt.Errorf("prompts.correct is required")
	}

	validModes := map[string]bool{
		"correct":   true,
		"translate": true,
		"rewrite":   true,
	}
	if !validModes[cfg.ActiveMode] {
		return fmt.Errorf("unsupported active_mode: %q (valid: correct, translate, rewrite)", cfg.ActiveMode)
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

	// Validate translate target language codes exist in LanguageNames.
	for _, target := range cfg.ParsedTargets {
		if _, ok := cfg.LanguageNames[target.LangA]; !ok {
			return fmt.Errorf("translate target language %q not found in language_names", target.LangA)
		}
		if target.LangB != "" {
			if _, ok := cfg.LanguageNames[target.LangB]; !ok {
				return fmt.Errorf("translate target language %q not found in language_names", target.LangB)
			}
		}
	}

	return nil
}

// TranslateTargetLabels returns display labels for each parsed target.
// Pairs are shown as "English ↔ French", singles as "Spanish".
func (cfg *Config) TranslateTargetLabels() []string {
	labels := make([]string, len(cfg.ParsedTargets))
	for i, t := range cfg.ParsedTargets {
		nameA := cfg.LanguageNames[t.LangA]
		if nameA == "" {
			nameA = t.LangA
		}
		if t.IsPair() {
			nameB := cfg.LanguageNames[t.LangB]
			if nameB == "" {
				nameB = t.LangB
			}
			labels[i] = nameA + " ↔ " + nameB
		} else {
			labels[i] = nameA
		}
	}
	return labels
}
