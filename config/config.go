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
	Name        string `json:"name"`
	Prompt      string `json:"prompt"`
	LLM         string `json:"llm,omitempty"`
	Icon        string `json:"icon,omitempty"`         // emoji icon shown in tray menu (e.g. "✏️")
	TimeoutMs   int    `json:"timeout_ms,omitempty"`    // per-prompt timeout override (0 = use model default)
	DisplayMode string `json:"display_mode,omitempty"`  // "replace" (default) or "popup" — how to show the LLM result
	Vision      bool   `json:"vision,omitempty"`        // capture screenshot instead of text
	Voice       bool   `json:"voice,omitempty"`         // record microphone instead of capture text
	VoiceMode   string `json:"voice_mode,omitempty"`    // "skill" (default) or "dictation"
}

// LLMProviderDef defines a named LLM provider configuration.
// This is an internal merge type used by the LLM package to create clients.
// It is NOT stored in config JSON directly; instead, the LLM layer
// synthesises it from Providers + Models at runtime.
type LLMProviderDef struct {
	Provider     string `json:"provider"`
	APIKey       string `json:"api_key,omitempty"`
	Model        string `json:"model"`
	APIEndpoint  string `json:"api_endpoint,omitempty"`
	MaxTokens    int    `json:"max_tokens,omitempty"`
	TimeoutMs    int    `json:"timeout_ms,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"` // OAuth: used to auto-refresh API key
	KeepAlive    bool   `json:"keep_alive,omitempty"`    // local: keep llama-server running (no idle timeout)
}

// ProviderConfig holds connection credentials for one AI provider.
// Keys: "anthropic", "openai", "chatgpt", "gemini", "xai", "deepseek", "ollama", "local", "lmstudio"
type ProviderConfig struct {
	APIKey       string `json:"api_key,omitempty"`
	APIEndpoint  string `json:"api_endpoint,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	KeepAlive    bool   `json:"keep_alive,omitempty"`
	TimeoutMs    int    `json:"timeout_ms,omitempty"`
	Disabled     bool   `json:"disabled,omitempty"`
}

// ModelEntry defines a named model configuration that references a provider.
type ModelEntry struct {
	Provider  string `json:"provider"`             // key into Providers map
	Model     string `json:"model"`                // model identifier
	MaxTokens int    `json:"max_tokens,omitempty"`
	TimeoutMs int    `json:"timeout_ms,omitempty"` // overrides provider timeout
}

// Hotkeys defines the configurable hotkey bindings.
type Hotkeys struct {
	Action      string `json:"action"`
	CyclePrompt string `json:"cycle_prompt"`
}

// VoiceConfig holds voice input settings (#236).
type VoiceConfig struct {
	Enabled  bool   `json:"enabled,omitempty"`
	Language string `json:"language,omitempty"` // BCP-47 code or empty for auto-detect
	Model    string `json:"model,omitempty"`    // e.g. "whisper-base"
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

// Config is the top-level configuration for GhostSpell.
type Config struct {
	Providers    map[string]ProviderConfig `json:"providers,omitempty"`
	Models       map[string]ModelEntry     `json:"models,omitempty"`
	DefaultModel string                    `json:"default_model,omitempty"`

	ActivePrompt int           `json:"active_prompt"`
	Prompts      []PromptEntry `json:"prompts"`
	Hotkeys      Hotkeys       `json:"hotkeys"`
	Overlay      Overlay       `json:"overlay"`

	MaxTokens         int    `json:"max_tokens"`
	TimeoutMs         int    `json:"timeout_ms"`
	MaxInputChars     int    `json:"max_input_chars"`
	PreserveClipboard bool   `json:"preserve_clipboard"`
	SoundEnabled      *bool  `json:"sound_enabled"`
	IndicatorPosition string `json:"indicator_position,omitempty"` // center, top-right, top-left, bottom-right, bottom-left, hidden
	IndicatorMode     string `json:"indicator_mode,omitempty"`     // "processing" (default), "always", "hidden" (#211)
	IndicatorX        int    `json:"indicator_x,omitempty"`        // saved drag position X
	IndicatorY        int    `json:"indicator_y,omitempty"`        // saved drag position Y

	Voice VoiceConfig `json:"voice,omitempty"` // voice input settings (#236)

	LogLevel          string `json:"log_level"`
	LogFile           string `json:"log_file"`
	LastSeenVersion   string `json:"last_seen_version,omitempty"`
}

// NeedsSetup returns true if no usable provider is configured.
func NeedsSetup(cfg *Config) bool {
	return len(cfg.Providers) == 0
}

func boolPtr(v bool) *bool { return &v }

// Default prompt texts.
const (
	DefaultCorrectPrompt   = "Fix only spelling and grammar errors. Do not rewrite, rephrase, or restructure the sentence. Keep the text in its original language — never translate it. Preserve slang, abbreviations, acronyms, and informal tone exactly as written. Only fix what is objectively incorrect. Return ONLY the corrected text with no explanation."
	DefaultPolishPrompt    = "Improve the following text to make it clearer, more natural, and better structured while preserving its original meaning and tone. Fix grammar, punctuation, and awkward phrasing. Smooth out rough sentences into polished, ready-to-send prose. Keep the text in its original language — never translate it. Return ONLY the improved text with no explanation."
	DefaultFunnyPrompt     = "Rewrite the following text to be funny, witty, and entertaining while preserving the original meaning and key information. Add humor, clever wordplay, or a lighthearted twist. Keep the text in its original language — never translate it. Return ONLY the funny version with no explanation."
	DefaultElaboratePrompt = "Expand the following text by adding relevant detail, context, and completeness while preserving the original meaning and intent. Flesh out terse or incomplete points into well-developed statements. Maintain the same tone and style as the original. Keep the text in its original language — never translate it. Return ONLY the elaborated text with no explanation."
	DefaultShortenPrompt   = "Condense the following text to be as concise as possible while preserving all essential meaning and key information. Remove redundancy, filler words, and unnecessary qualifiers. Keep the same tone and intent. Keep the text in its original language — never translate it. Return ONLY the shortened text with no explanation."
	DefaultTranslatePrompt = "Translate the following text to English regardless of its source language. Return ONLY the translated text with no explanation."
	DefaultAskPrompt              = "Answer this question clearly and concisely. Return the question and then the answer."
	DefaultDefinePrompt           = "Define the following word or phrase. Provide a clear, concise definition. If applicable, include the part of speech and a brief example of usage. Keep it short and helpful."
	DefaultDescribeScreenPrompt   = "Describe what you see in this image. Be concise."
	DefaultScreenshotOCRPrompt    = "Extract all text from this image. Return only the text, preserving formatting."
	DefaultDictatePrompt          = "Transcribe the following speech accurately. Preserve the speaker's words exactly, only fixing obvious speech-to-text errors. Do not rephrase or summarize."
	DefaultVoiceNotePrompt        = "The following is a voice transcription. Clean it up into well-structured text. Fix grammar and remove filler words, but preserve the meaning."
)

// DefaultPrompts returns the default prompt list.
func DefaultPrompts() []PromptEntry {
	return []PromptEntry{
		{Name: "Correct", Prompt: DefaultCorrectPrompt, Icon: "\u270F\uFE0F"},
		{Name: "Polish", Prompt: DefaultPolishPrompt, Icon: "\U0001F48E"},
		{Name: "Funny", Prompt: DefaultFunnyPrompt, Icon: "\U0001F604"},
		{Name: "Elaborate", Prompt: DefaultElaboratePrompt, Icon: "\U0001F4DD"},
		{Name: "Shorten", Prompt: DefaultShortenPrompt, Icon: "\u2702\uFE0F"},
		{Name: "Translate", Prompt: DefaultTranslatePrompt, Icon: "\U0001F310"},
		{Name: "Ask", Prompt: DefaultAskPrompt, Icon: "\u2753"},
		{Name: "Define", Prompt: DefaultDefinePrompt, Icon: "\U0001F4D6", DisplayMode: "popup"},
		{Name: "Describe Screenshot", Prompt: DefaultDescribeScreenPrompt, Icon: "\U0001F4F8", Vision: true, DisplayMode: "popup"},
		{Name: "Screenshot OCR", Prompt: DefaultScreenshotOCRPrompt, Icon: "\U0001F441\uFE0F", Vision: true, DisplayMode: "popup"},
		{Name: "Dictate", Prompt: DefaultDictatePrompt, Icon: "\U0001F399\uFE0F", Voice: true, VoiceMode: "dictation"},
		{Name: "Voice Note", Prompt: DefaultVoiceNotePrompt, Icon: "\U0001F4DD", Voice: true, VoiceMode: "skill"},
	}
}

// DefaultConfig returns a Config populated with default values.
func DefaultConfig() Config {
	return Config{
		Providers: make(map[string]ProviderConfig),
		Models:    make(map[string]ModelEntry),
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
		LogLevel:          "debug",
		LogFile:           "ghostspell.log",
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

// unmarshalWithMigration parses config JSON, detecting and migrating old formats:
//   - Old prompts format (JSON object with correct/translate/rewrite_templates)
//   - Old LLM fields (llm_providers map or llm_provider flat fields)
//
// to the new providers + models structure.
func unmarshalWithMigration(data []byte) (*Config, error) {
	// First, try parsing into a raw map to detect the prompts format.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	promptsRaw, hasPrompts := raw["prompts"]
	needsPromptMigration := false

	if hasPrompts && len(promptsRaw) > 0 {
		// Trim whitespace to find the first meaningful character.
		trimmed := strings.TrimSpace(string(promptsRaw))
		if len(trimmed) > 0 && trimmed[0] == '{' {
			needsPromptMigration = true
		}
	}

	if needsPromptMigration {
		return migrateOldConfig(data)
	}

	// Check for legacy llm_providers or llm_provider fields that need migration.
	_, hasLLMProviders := raw["llm_providers"]
	_, hasLLMProvider := raw["llm_provider"]
	if hasLLMProviders || hasLLMProvider {
		return migrateLLMFields(data)
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
	Correct          string               `json:"correct"`
	Translate        string               `json:"translate"`
	TranslateSingle  string               `json:"translate_single"`
	RewriteTemplates []oldRewriteTemplate `json:"rewrite_templates"`
}

type oldRewriteTemplate struct {
	Name   string `json:"name"`
	Prompt string `json:"prompt"`
	LLM    string `json:"llm,omitempty"`
}

// oldConfig is a partial struct for deserializing the oldest format
// (object-style prompts with correct/translate/rewrite_templates).
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

// legacyConfig is a partial struct for deserializing configs with old-style
// llm_providers / llm_provider flat fields (but new-style prompts array).
type legacyConfig struct {
	LLMProvider  string                    `json:"llm_provider,omitempty"`
	APIKey       string                    `json:"api_key,omitempty"`
	Model        string                    `json:"model,omitempty"`
	APIEndpoint  string                    `json:"api_endpoint,omitempty"`
	LLMProviders map[string]LLMProviderDef `json:"llm_providers,omitempty"`
	DefaultLLM   string                    `json:"default_llm,omitempty"`

	ActivePrompt      int           `json:"active_prompt"`
	Prompts           []PromptEntry `json:"prompts"`
	Hotkeys           Hotkeys       `json:"hotkeys"`
	Overlay           Overlay       `json:"overlay"`
	MaxTokens         int           `json:"max_tokens"`
	TimeoutMs         int           `json:"timeout_ms"`
	MaxInputChars     int           `json:"max_input_chars"`
	PreserveClipboard bool          `json:"preserve_clipboard"`
	SoundEnabled      *bool         `json:"sound_enabled"`
	LogLevel          string        `json:"log_level"`
	LogFile           string        `json:"log_file"`
}

// migrateLegacyLLM converts old llm_providers/llm_provider fields into the
// new Providers + Models maps.
func migrateLegacyLLM(
	llmProvider string, apiKey string, model string, apiEndpoint string,
	llmProviders map[string]LLMProviderDef, defaultLLM string,
	maxTokens int,
) (map[string]ProviderConfig, map[string]ModelEntry, string) {
	providers := make(map[string]ProviderConfig)
	models := make(map[string]ModelEntry)
	defaultModel := ""

	if len(llmProviders) > 0 {
		for label, def := range llmProviders {
			provKey := def.Provider
			// Add provider credentials (dedup by provider key).
			if _, exists := providers[provKey]; !exists {
				providers[provKey] = ProviderConfig{
					APIKey:       def.APIKey,
					APIEndpoint:  def.APIEndpoint,
					RefreshToken: def.RefreshToken,
					KeepAlive:    def.KeepAlive,
					TimeoutMs:    def.TimeoutMs,
				}
			}
			models[label] = ModelEntry{
				Provider:  provKey,
				Model:     def.Model,
				MaxTokens: def.MaxTokens,
				TimeoutMs: def.TimeoutMs,
			}
		}
		if defaultLLM != "" {
			defaultModel = defaultLLM
		}
	} else if llmProvider != "" {
		provKey := llmProvider
		providers[provKey] = ProviderConfig{
			APIKey:      apiKey,
			APIEndpoint: apiEndpoint,
		}
		models["default"] = ModelEntry{
			Provider:  provKey,
			Model:     model,
			MaxTokens: maxTokens,
		}
		defaultModel = "default"
	}

	return providers, models, defaultModel
}

// migrateLLMFields converts a config with old-style llm_providers/llm_provider
// fields (but new-style prompts array) to the new providers + models structure.
func migrateLLMFields(data []byte) (*Config, error) {
	var old legacyConfig
	if err := json.Unmarshal(data, &old); err != nil {
		return nil, fmt.Errorf("migration: %w", err)
	}

	slog.Info("Migrating config from llm_providers to providers+models format")

	providers, models, defaultModel := migrateLegacyLLM(
		old.LLMProvider, old.APIKey, old.Model, old.APIEndpoint,
		old.LLMProviders, old.DefaultLLM, old.MaxTokens,
	)

	return &Config{
		Providers:         providers,
		Models:            models,
		DefaultModel:      defaultModel,
		ActivePrompt:      old.ActivePrompt,
		Prompts:           old.Prompts,
		Hotkeys:           old.Hotkeys,
		Overlay:           old.Overlay,
		MaxTokens:         old.MaxTokens,
		TimeoutMs:         old.TimeoutMs,
		MaxInputChars:     old.MaxInputChars,
		PreserveClipboard: old.PreserveClipboard,
		SoundEnabled:      old.SoundEnabled,
		LogLevel:          old.LogLevel,
		LogFile:           old.LogFile,
	}, nil
}

// migrateOldConfig converts an old-format config (object-style prompts) to the new format.
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

	// Migrate LLM fields.
	providers, models, defaultModel := migrateLegacyLLM(
		old.LLMProvider, old.APIKey, old.Model, old.APIEndpoint,
		old.LLMProviders, old.DefaultLLM, old.MaxTokens,
	)

	return &Config{
		Providers:    providers,
		Models:       models,
		DefaultModel: defaultModel,
		ActivePrompt: activePrompt,
		Prompts:      prompts,
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
	}, nil
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
	return os.WriteFile(path, data, 0600) // #200: restrict to owner-only (contains API keys)
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
		cfg.LogFile = "ghostspell.log"
	}
	if cfg.SoundEnabled == nil {
		cfg.SoundEnabled = boolPtr(true)
	}
	if cfg.IndicatorPosition == "" {
		cfg.IndicatorPosition = "top-right"
	}
	if cfg.Hotkeys.Action == "" {
		cfg.Hotkeys.Action = defaultActionHotkey
	}
	if cfg.Hotkeys.CyclePrompt == "" {
		cfg.Hotkeys.CyclePrompt = defaultCycleHotkey
	}

	// Default prompts if empty.
	if len(cfg.Prompts) == 0 {
		cfg.Prompts = DefaultPrompts()
	}

	// Migrate icons: assign default emoji icons to built-in prompts that lack
	// them. Existing configs created before the icon field was added will get
	// the standard icons without losing any custom prompt data.
	defaultIcons := map[string]string{
		"Correct":   "\u270F\uFE0F",
		"Polish":    "\U0001F48E",
		"Funny":     "\U0001F604",
		"Elaborate": "\U0001F4DD",
		"Shorten":   "\u2702\uFE0F",
		"Translate": "\U0001F310",
		"Ask":       "\u2753",
		"Define":    "\U0001F4D6",
	}
	for i := range cfg.Prompts {
		if cfg.Prompts[i].Icon == "" {
			if icon, ok := defaultIcons[cfg.Prompts[i].Name]; ok {
				cfg.Prompts[i].Icon = icon
			}
		}
	}

	// Migrate: add Define prompt if missing (added in v0.36.0).
	hasDefine := false
	for _, p := range cfg.Prompts {
		if p.Name == "Define" {
			hasDefine = true
			break
		}
	}
	if !hasDefine {
		cfg.Prompts = append(cfg.Prompts, PromptEntry{
			Name:        "Define",
			Prompt:      DefaultDefinePrompt,
			Icon:        "\U0001F4D6",
			DisplayMode: "popup",
		})
	}

	// Migrate: add vision prompts if missing (added in v0.43.0).
	hasDescribeScreen := false
	hasScreenshotOCR := false
	for _, p := range cfg.Prompts {
		if p.Name == "Describe Screenshot" {
			hasDescribeScreen = true
		}
		if p.Name == "Screenshot OCR" {
			hasScreenshotOCR = true
		}
	}
	if !hasDescribeScreen {
		cfg.Prompts = append(cfg.Prompts, PromptEntry{
			Name:        "Describe Screenshot",
			Prompt:      DefaultDescribeScreenPrompt,
			Icon:        "\U0001F4F8",
			Vision:      true,
			DisplayMode: "popup",
		})
	}
	if !hasScreenshotOCR {
		cfg.Prompts = append(cfg.Prompts, PromptEntry{
			Name:        "Screenshot OCR",
			Prompt:      DefaultScreenshotOCRPrompt,
			Icon:        "\U0001F441\uFE0F",
			Vision:      true,
			DisplayMode: "popup",
		})
	}

	// Migrate: add voice prompts if missing (added in v0.56.0).
	hasDictate := false
	hasVoiceNote := false
	for _, p := range cfg.Prompts {
		if p.Name == "Dictate" {
			hasDictate = true
		}
		if p.Name == "Voice Note" {
			hasVoiceNote = true
		}
	}
	if !hasDictate {
		cfg.Prompts = append(cfg.Prompts, PromptEntry{
			Name:      "Dictate",
			Prompt:    DefaultDictatePrompt,
			Icon:      "\U0001F399\uFE0F",
			Voice:     true,
			VoiceMode: "dictation",
		})
	}
	if !hasVoiceNote {
		cfg.Prompts = append(cfg.Prompts, PromptEntry{
			Name:      "Voice Note",
			Prompt:    DefaultVoiceNotePrompt,
			Icon:      "\U0001F4DD",
			Voice:     true,
			VoiceMode: "skill",
		})
	}

	// Clamp active_prompt to valid range.
	if cfg.ActivePrompt < 0 || cfg.ActivePrompt >= len(cfg.Prompts) {
		cfg.ActivePrompt = 0
	}

	// Initialize maps if nil.
	if cfg.Providers == nil {
		cfg.Providers = make(map[string]ProviderConfig)
	}
	if cfg.Models == nil {
		cfg.Models = make(map[string]ModelEntry)
	}

	// Migrate "ghost-ai" model entries that incorrectly point to "ollama".
	// Before v0.15.0, the settings page could create a "ghost-ai" model with
	// provider "ollama" instead of "local". Fix it and rename to "GhostSpell Local".
	if me, ok := cfg.Models["ghost-ai"]; ok && me.Provider == "ollama" {
		if _, hasLocal := cfg.Providers["local"]; hasLocal {
			slog.Info("Migrating ghost-ai model from ollama to local provider")
			delete(cfg.Models, "ghost-ai")
			// Convert Ollama model name (colon) to Ghost-AI name (dash).
			// e.g. "qwen3.5:4b" → "qwen3.5-4b"
			model := strings.ReplaceAll(me.Model, ":", "-")
			cfg.Models["GhostSpell Local"] = ModelEntry{
				Provider:  "local",
				Model:     model,
				MaxTokens: me.MaxTokens,
				TimeoutMs: me.TimeoutMs,
			}
			if cfg.DefaultModel == "ghost-ai" {
				cfg.DefaultModel = "GhostSpell Local"
			}
		}
	}

	// Migrate "GhostAI" model label to "GhostSpell Local".
	if me, ok := cfg.Models["GhostAI"]; ok {
		delete(cfg.Models, "GhostAI")
		cfg.Models["GhostSpell Local"] = me
		if cfg.DefaultModel == "GhostAI" {
			cfg.DefaultModel = "GhostSpell Local"
		}
		slog.Info("Migrated model label GhostAI → GhostSpell Local")
	}

	// Migrate OAuth refresh_token from providers.openai to providers.chatgpt.
	// Before v0.20.0, both API key and OAuth shared providers.openai.
	if openai, ok := cfg.Providers["openai"]; ok && openai.RefreshToken != "" {
		if _, hasChatGPT := cfg.Providers["chatgpt"]; !hasChatGPT {
			slog.Info("Migrating OAuth refresh_token from openai to chatgpt provider")
			cfg.Providers["chatgpt"] = ProviderConfig{
				RefreshToken: openai.RefreshToken,
			}
			// Clear refresh_token from openai (keep API key if present).
			openai.RefreshToken = ""
			if openai.APIKey == "" && openai.APIEndpoint == "" {
				delete(cfg.Providers, "openai")
			} else {
				cfg.Providers["openai"] = openai
			}
			// Update model entries that point to "openai" but were created via OAuth.
			for name, me := range cfg.Models {
				if me.Provider == "openai" && (name == "chatgpt" || strings.Contains(strings.ToLower(name), "chatgpt")) {
					me.Provider = "chatgpt"
					cfg.Models[name] = me
				}
			}
		}
	}

	// Fill missing TimeoutMs per model from provider or global value.
	for name, me := range cfg.Models {
		if me.TimeoutMs == 0 {
			if prov, ok := cfg.Providers[me.Provider]; ok && prov.TimeoutMs != 0 {
				me.TimeoutMs = prov.TimeoutMs
			} else if me.Provider == "ollama" || me.Provider == "local" {
				me.TimeoutMs = 120000
			} else {
				me.TimeoutMs = cfg.TimeoutMs
			}
			cfg.Models[name] = me
		}
	}
}

// Validate checks that the config has all required fields.
func Validate(cfg *Config) error {
	validProviders := map[string]bool{
		"anthropic": true,
		"openai":    true,
		"chatgpt":   true,
		"gemini":    true,
		"xai":       true,
		"deepseek":  true,
		"ollama":    true,
		"local":     true,
		"lmstudio":  true,
	}

	if len(cfg.Providers) == 0 {
		return fmt.Errorf("at least one provider is required")
	}

	// Validate each provider key.
	for key := range cfg.Providers {
		if !validProviders[key] {
			return fmt.Errorf("providers[%s]: unsupported provider key (valid: anthropic, openai, gemini, xai, deepseek, ollama, local, lmstudio)", key)
		}
	}

	// Validate each model entry.
	for name, me := range cfg.Models {
		if me.Provider == "" {
			return fmt.Errorf("models[%s]: provider is required", name)
		}
		if _, ok := cfg.Providers[me.Provider]; !ok {
			return fmt.Errorf("models[%s]: provider %q not found in providers", name, me.Provider)
		}
		if me.Model == "" {
			return fmt.Errorf("models[%s]: model is required", name)
		}
	}

	// DefaultModel must reference an existing model entry.
	if cfg.DefaultModel != "" {
		if _, ok := cfg.Models[cfg.DefaultModel]; !ok {
			return fmt.Errorf("default_model %q not found in models", cfg.DefaultModel)
		}
	}

	// Validate prompt LLM label references point to defined models.
	if len(cfg.Models) > 0 {
		for _, p := range cfg.Prompts {
			if p.LLM != "" {
				if _, ok := cfg.Models[p.LLM]; !ok {
					return fmt.Errorf("prompt[%s].llm %q not found in models", p.Name, p.LLM)
				}
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
