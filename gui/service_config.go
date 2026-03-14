package gui

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/chrixbedardcad/GhostSpell/config"
	"github.com/chrixbedardcad/GhostSpell/llm"
)

// validateAndSave writes the config to disk, removing any provider entries that
// are clearly invalid (no API key for non-ollama/local providers) and ensuring
// DefaultLLM points to a valid provider. Legacy flat fields are omitted
// automatically by their omitempty JSON tags.
func (s *SettingsService) validateAndSave() error {
	// Remove invalid provider entries (e.g. phantom "default" from legacy synthesis).
	for label, def := range s.cfgCopy.LLMProviders {
		if def.Provider != "ollama" && def.Provider != "local" && def.APIKey == "" && def.RefreshToken == "" {
			delete(s.cfgCopy.LLMProviders, label)
			if s.cfgCopy.DefaultLLM == label {
				s.cfgCopy.DefaultLLM = ""
			}
		}
	}
	// Ensure DefaultLLM points to a valid provider.
	if s.cfgCopy.DefaultLLM == "" && len(s.cfgCopy.LLMProviders) > 0 {
		for k := range s.cfgCopy.LLMProviders {
			s.cfgCopy.DefaultLLM = k
			break
		}
	}

	if err := config.WriteDefault(s.configPath, s.cfgCopy); err != nil {
		return err
	}
	s.saved = true
	if s.onSaved != nil {
		s.onSaved()
	}
	return nil
}

// GetConfig returns the current config as JSON.
func (s *SettingsService) GetConfig() string {
	guiLog("[GUI] JS called: GetConfig")
	data, err := json.Marshal(s.cfgCopy)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// SaveProvider saves or updates a named LLM provider.
func (s *SettingsService) SaveProvider(label, provider, apiKey, model, endpoint, originalLabel string) string {
	guiLog("[GUI] JS called: SaveProvider(%s, %s)", label, provider)
	if label == "" {
		return "error: label is required"
	}

	if originalLabel != "" && originalLabel != label {
		delete(s.cfgCopy.LLMProviders, originalLabel)
		if s.cfgCopy.DefaultLLM == originalLabel {
			s.cfgCopy.DefaultLLM = label
		}
	}

	if s.cfgCopy.LLMProviders == nil {
		s.cfgCopy.LLMProviders = make(map[string]config.LLMProviderDef)
	}

	s.cfgCopy.LLMProviders[label] = config.LLMProviderDef{
		Provider:    provider,
		APIKey:      apiKey,
		Model:       model,
		APIEndpoint: endpoint,
	}

	if len(s.cfgCopy.LLMProviders) == 1 || s.cfgCopy.DefaultLLM == "" {
		s.cfgCopy.DefaultLLM = label
	}

	if err := s.validateAndSave(); err != nil {
		return fmt.Sprintf("error: %v", err)
	}

	guiLog("[GUI] Provider saved: label=%s provider=%s", label, provider)
	return "ok"
}

// DeleteProvider removes a provider by label.
func (s *SettingsService) DeleteProvider(label string) string {
	guiLog("[GUI] JS called: DeleteProvider(%s)", label)
	delete(s.cfgCopy.LLMProviders, label)
	if s.cfgCopy.DefaultLLM == label {
		s.cfgCopy.DefaultLLM = ""
		for k := range s.cfgCopy.LLMProviders {
			s.cfgCopy.DefaultLLM = k
			break
		}
	}

	if err := s.validateAndSave(); err != nil {
		return fmt.Sprintf("error: %v", err)
	}

	return "ok"
}

// SetDefault sets the default provider.
func (s *SettingsService) SetDefault(label string) string {
	guiLog("[GUI] JS called: SetDefault(%s)", label)
	if _, ok := s.cfgCopy.LLMProviders[label]; !ok {
		return "error: provider not found"
	}
	s.cfgCopy.DefaultLLM = label

	if err := s.validateAndSave(); err != nil {
		return fmt.Sprintf("error: %v", err)
	}

	return "ok"
}

// TestConnection tests provider credentials.
func (s *SettingsService) TestConnection(provider, apiKey, model, endpoint string) string {
	guiLog("[GUI] JS called: TestConnection(provider=%s, model=%s, endpoint=%q)", provider, model, endpoint)

	// Ollama and local need much longer timeout — first request loads model into memory.
	// They also need more max_tokens because thinking models (Qwen3/3.5, DeepSeek)
	// can consume 200-400 tokens on <think> blocks even with /no_think — larger
	// models like qwen3.5-4b are especially heavy thinkers.
	timeout := 10 * time.Second
	maxTokens := 64
	timeoutMs := 10000
	if provider == "ollama" || provider == "local" {
		timeout = 120 * time.Second
		maxTokens = 512
		timeoutMs = 120000
		guiLog("[GUI] %s detected — using %s timeout, %d max_tokens", provider, timeout, maxTokens)
	}

	def := config.LLMProviderDef{
		Provider:    provider,
		APIKey:      apiKey,
		Model:       model,
		APIEndpoint: endpoint,
		MaxTokens:   maxTokens,
		TimeoutMs:   timeoutMs,
	}

	client, err := llm.NewClientFromDef(def)
	if err != nil {
		guiLog("[GUI] TestConnection: NewClientFromDef failed: %v", err)
		return fmt.Sprintf("error: %v", err)
	}
	defer client.Close()

	guiLog("[GUI] TestConnection: sending test request (timeout=%s)...", timeout)
	tctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()
	_, err = client.Send(tctx, llm.Request{
		Prompt:    "Reply with OK",
		Text:      "test",
		MaxTokens: maxTokens,
	})
	elapsed := time.Since(start)
	if err != nil {
		guiLog("[GUI] TestConnection FAILED after %s: %v", elapsed, err)
		return fmt.Sprintf("error: %v", err)
	}

	guiLog("[GUI] TestConnection OK (elapsed=%s)", elapsed)
	return "ok"
}

// TestProvider tests a saved provider by label.
func (s *SettingsService) TestProvider(label string) string {
	guiLog("[GUI] JS called: TestProvider(%s)", label)
	def, ok := s.cfgCopy.LLMProviders[label]
	if !ok {
		guiLog("[GUI] TestProvider: provider %q not found in config", label)
		return "error: provider not found"
	}
	guiLog("[GUI] TestProvider: provider=%s model=%s endpoint=%q", def.Provider, def.Model, def.APIEndpoint)

	// Ollama and local need much longer timeout — first request loads model into memory.
	// They also need more max_tokens because thinking models (Qwen3/3.5, DeepSeek)
	// can consume 200-400 tokens on <think> blocks even with /no_think — larger
	// models like qwen3.5-4b are especially heavy thinkers.
	timeout := 10 * time.Second
	maxTokens := 64
	timeoutMs := 10000
	if def.Provider == "ollama" || def.Provider == "local" {
		timeout = 120 * time.Second
		maxTokens = 512
		timeoutMs = 120000
		guiLog("[GUI] %s detected — using %s timeout, %d max_tokens", def.Provider, timeout, maxTokens)
	}

	def.MaxTokens = maxTokens
	def.TimeoutMs = timeoutMs

	client, err := llm.NewClientFromDef(def)
	if err != nil {
		guiLog("[GUI] TestProvider: NewClientFromDef failed: %v", err)
		return fmt.Sprintf("error: %v", err)
	}
	defer client.Close()

	guiLog("[GUI] TestProvider: sending test request (timeout=%s)...", timeout)
	tctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()
	_, err = client.Send(tctx, llm.Request{
		Prompt:    "Reply with OK",
		Text:      "test",
		MaxTokens: maxTokens,
	})
	elapsed := time.Since(start)
	if err != nil {
		guiLog("[GUI] TestProvider FAILED after %s: %v", elapsed, err)
		return fmt.Sprintf("error: %v", err)
	}

	guiLog("[GUI] TestProvider OK (elapsed=%s)", elapsed)
	return "ok"
}

// SetRefreshToken stores an OAuth refresh token for a saved provider.
func (s *SettingsService) SetRefreshToken(label, token string) string {
	guiLog("[GUI] JS called: SetRefreshToken(%s)", label)
	def, ok := s.cfgCopy.LLMProviders[label]
	if !ok {
		return "error: provider not found"
	}
	def.RefreshToken = token
	s.cfgCopy.LLMProviders[label] = def
	if err := s.validateAndSave(); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return "ok"
}

// --- Prompt management -----------------------------------------------------

// SavePrompt updates an existing prompt at the given index, or appends if index == -1.
func (s *SettingsService) SavePrompt(index int, name, prompt, llmLabel, icon string) string {
	guiLog("[GUI] JS called: SavePrompt(idx=%d, name=%s, icon=%s)", index, name, icon)
	if name == "" || prompt == "" {
		return "error: name and prompt are required"
	}
	entry := config.PromptEntry{Name: name, Prompt: prompt, LLM: llmLabel, Icon: icon}
	if index < 0 || index >= len(s.cfgCopy.Prompts) {
		s.cfgCopy.Prompts = append(s.cfgCopy.Prompts, entry)
	} else {
		s.cfgCopy.Prompts[index] = entry
	}
	if err := s.validateAndSave(); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return "ok"
}

// DeletePrompt removes the prompt at the given index.
func (s *SettingsService) DeletePrompt(index int) string {
	guiLog("[GUI] JS called: DeletePrompt(%d)", index)
	if index < 0 || index >= len(s.cfgCopy.Prompts) {
		return "error: invalid index"
	}
	s.cfgCopy.Prompts = append(s.cfgCopy.Prompts[:index], s.cfgCopy.Prompts[index+1:]...)
	// Adjust active_prompt if needed.
	if s.cfgCopy.ActivePrompt >= len(s.cfgCopy.Prompts) {
		s.cfgCopy.ActivePrompt = 0
	}
	if err := s.validateAndSave(); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return "ok"
}

// MovePrompt moves a prompt from one index to another.
func (s *SettingsService) MovePrompt(fromIdx, toIdx int) string {
	guiLog("[GUI] JS called: MovePrompt(%d -> %d)", fromIdx, toIdx)
	ps := s.cfgCopy.Prompts
	if fromIdx < 0 || fromIdx >= len(ps) || toIdx < 0 || toIdx >= len(ps) {
		return "error: invalid index"
	}
	item := ps[fromIdx]
	ps = append(ps[:fromIdx], ps[fromIdx+1:]...)
	newPS := make([]config.PromptEntry, 0, len(ps)+1)
	newPS = append(newPS, ps[:toIdx]...)
	newPS = append(newPS, item)
	newPS = append(newPS, ps[toIdx:]...)
	s.cfgCopy.Prompts = newPS
	if err := s.validateAndSave(); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return "ok"
}

// ResetPrompts restores the default prompts (Correct, Polish, Translate).
func (s *SettingsService) ResetPrompts() string {
	guiLog("[GUI] JS called: ResetPrompts")
	s.cfgCopy.Prompts = config.DefaultPrompts()
	s.cfgCopy.ActivePrompt = 0
	if err := s.validateAndSave(); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return "ok"
}

// --- General settings ------------------------------------------------------

// SetSoundEnabled toggles sound.
func (s *SettingsService) SetSoundEnabled(enabled bool) string {
	guiLog("[GUI] JS called: SetSoundEnabled(%v)", enabled)
	s.cfgCopy.SoundEnabled = &enabled
	if err := s.validateAndSave(); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return "ok"
}

// SetPreserveClipboard toggles clipboard preservation.
func (s *SettingsService) SetPreserveClipboard(enabled bool) string {
	guiLog("[GUI] JS called: SetPreserveClipboard(%v)", enabled)
	s.cfgCopy.PreserveClipboard = enabled
	if err := s.validateAndSave(); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return "ok"
}

// SetMaxInputChars updates the max input character limit.
func (s *SettingsService) SetMaxInputChars(limit int) string {
	guiLog("[GUI] JS called: SetMaxInputChars(%d)", limit)
	if limit < 0 {
		limit = 0
	}
	s.cfgCopy.MaxInputChars = limit
	if err := s.validateAndSave(); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return "ok"
}

// SetHotkey updates a named hotkey binding.
func (s *SettingsService) SetHotkey(name, binding string) string {
	guiLog("[GUI] JS called: SetHotkey(%s, %s)", name, binding)
	switch name {
	case "action":
		s.cfgCopy.Hotkeys.Action = binding
	case "cycle_prompt":
		s.cfgCopy.Hotkeys.CyclePrompt = binding
	default:
		return "error: unknown hotkey name"
	}
	if err := s.validateAndSave(); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return "ok"
}

// SetLogLevel updates the log level.
func (s *SettingsService) SetLogLevel(level string) string {
	guiLog("[GUI] JS called: SetLogLevel(%s)", level)
	s.cfgCopy.LogLevel = level
	if err := s.validateAndSave(); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return "ok"
}

// OpenConfigFile opens the config file in the OS default editor.
func (s *SettingsService) OpenConfigFile() string {
	guiLog("[GUI] JS called: OpenConfigFile")
	OpenFile(s.configPath)
	return "ok"
}
