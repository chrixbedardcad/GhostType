package gui

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/chrixbedardcad/GhostSpell/config"
	"github.com/chrixbedardcad/GhostSpell/llm"
)

// validateAndSave writes the config to disk, validating that model entries
// reference existing providers and ensuring DefaultModel points to a valid model.
func (s *SettingsService) validateAndSave() error {
	// Remove model entries whose provider is missing from Providers map.
	for label, me := range s.cfgCopy.Models {
		if _, ok := s.cfgCopy.Providers[me.Provider]; !ok {
			// Provider missing — keep the model but log it; the UI shows a warning.
			_ = label
		}
	}

	// Ensure DefaultModel points to a valid model entry.
	if s.cfgCopy.DefaultModel != "" {
		if _, ok := s.cfgCopy.Models[s.cfgCopy.DefaultModel]; !ok {
			s.cfgCopy.DefaultModel = ""
		}
	}
	if s.cfgCopy.DefaultModel == "" && len(s.cfgCopy.Models) > 0 {
		for k := range s.cfgCopy.Models {
			s.cfgCopy.DefaultModel = k
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

// SaveProviderConfig saves or updates a provider's connection credentials.
func (s *SettingsService) SaveProviderConfig(providerType, apiKey, endpoint, refreshToken string, keepAlive bool) string {
	guiLog("[GUI] JS called: SaveProviderConfig(%s)", providerType)
	if providerType == "" {
		return "error: providerType is required"
	}

	if s.cfgCopy.Providers == nil {
		s.cfgCopy.Providers = make(map[string]config.ProviderConfig)
	}

	s.cfgCopy.Providers[providerType] = config.ProviderConfig{
		APIKey:       apiKey,
		APIEndpoint:  endpoint,
		RefreshToken: refreshToken,
		KeepAlive:    keepAlive,
	}

	if err := s.validateAndSave(); err != nil {
		return fmt.Sprintf("error: %v", err)
	}

	guiLog("[GUI] Provider config saved: %s", providerType)
	return "ok"
}

// SaveModel saves or updates a named model entry.
func (s *SettingsService) SaveModel(label, providerType, modelName, originalLabel string) string {
	guiLog("[GUI] JS called: SaveModel(%s, %s, %s, original=%s)", label, providerType, modelName, originalLabel)
	if label == "" {
		return "error: label is required"
	}

	if originalLabel != "" && originalLabel != label {
		delete(s.cfgCopy.Models, originalLabel)
		if s.cfgCopy.DefaultModel == originalLabel {
			s.cfgCopy.DefaultModel = label
		}
	}

	if s.cfgCopy.Models == nil {
		s.cfgCopy.Models = make(map[string]config.ModelEntry)
	}

	s.cfgCopy.Models[label] = config.ModelEntry{
		Provider: providerType,
		Model:    modelName,
	}

	// Auto-set DefaultModel if this is the first model.
	if len(s.cfgCopy.Models) == 1 || s.cfgCopy.DefaultModel == "" {
		s.cfgCopy.DefaultModel = label
	}

	if err := s.validateAndSave(); err != nil {
		return fmt.Sprintf("error: %v", err)
	}

	guiLog("[GUI] Model saved: label=%s provider=%s model=%s", label, providerType, modelName)
	return "ok"
}

// SaveProvider is a convenience method that saves both provider config and a model entry
// in a single call. Used by the wizard and legacy code paths.
func (s *SettingsService) SaveProvider(label, provider, apiKey, model, endpoint, originalLabel string) string {
	guiLog("[GUI] JS called: SaveProvider(%s, %s)", label, provider)
	if label == "" {
		return "error: label is required"
	}

	// Save provider credentials.
	if s.cfgCopy.Providers == nil {
		s.cfgCopy.Providers = make(map[string]config.ProviderConfig)
	}

	// Preserve existing RefreshToken and KeepAlive if provider already exists.
	existing := s.cfgCopy.Providers[provider]
	s.cfgCopy.Providers[provider] = config.ProviderConfig{
		APIKey:       apiKey,
		APIEndpoint:  endpoint,
		RefreshToken: existing.RefreshToken,
		KeepAlive:    existing.KeepAlive,
	}

	// Handle model rename.
	if originalLabel != "" && originalLabel != label {
		delete(s.cfgCopy.Models, originalLabel)
		if s.cfgCopy.DefaultModel == originalLabel {
			s.cfgCopy.DefaultModel = label
		}
	}

	if s.cfgCopy.Models == nil {
		s.cfgCopy.Models = make(map[string]config.ModelEntry)
	}

	s.cfgCopy.Models[label] = config.ModelEntry{
		Provider: provider,
		Model:    model,
	}

	// Auto-set DefaultModel if first model or none set.
	if len(s.cfgCopy.Models) == 1 || s.cfgCopy.DefaultModel == "" {
		s.cfgCopy.DefaultModel = label
	}

	if err := s.validateAndSave(); err != nil {
		return fmt.Sprintf("error: %v", err)
	}

	guiLog("[GUI] Provider+model saved: label=%s provider=%s model=%s", label, provider, model)
	return "ok"
}

// RemoveProvider removes a provider's credentials from the config.
// Does NOT auto-remove models that reference it (they show a warning in the UI).
func (s *SettingsService) RemoveProvider(providerType string) string {
	guiLog("[GUI] JS called: RemoveProvider(%s)", providerType)
	delete(s.cfgCopy.Providers, providerType)

	if err := s.validateAndSave(); err != nil {
		return fmt.Sprintf("error: %v", err)
	}

	return "ok"
}

// DeleteModel removes a model entry by label.
func (s *SettingsService) DeleteModel(label string) string {
	guiLog("[GUI] JS called: DeleteModel(%s)", label)
	delete(s.cfgCopy.Models, label)
	if s.cfgCopy.DefaultModel == label {
		s.cfgCopy.DefaultModel = ""
		for k := range s.cfgCopy.Models {
			s.cfgCopy.DefaultModel = k
			break
		}
	}

	if err := s.validateAndSave(); err != nil {
		return fmt.Sprintf("error: %v", err)
	}

	return "ok"
}

// DeleteProvider removes a model entry by label (legacy compatibility alias for DeleteModel).
func (s *SettingsService) DeleteProvider(label string) string {
	guiLog("[GUI] JS called: DeleteProvider(%s)", label)
	return s.DeleteModel(label)
}

// SetDefaultModel sets the default model.
func (s *SettingsService) SetDefaultModel(label string) string {
	guiLog("[GUI] JS called: SetDefaultModel(%s)", label)
	if _, ok := s.cfgCopy.Models[label]; !ok {
		return "error: model not found"
	}
	s.cfgCopy.DefaultModel = label

	if err := s.validateAndSave(); err != nil {
		return fmt.Sprintf("error: %v", err)
	}

	return "ok"
}

// SetDefault sets the default model (legacy compatibility alias for SetDefaultModel).
func (s *SettingsService) SetDefault(label string) string {
	return s.SetDefaultModel(label)
}

// TestConnection tests provider credentials by sending a test request.
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
		maxTokens = 128
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
		Prompt:    "Fix spelling. Return ONLY the corrected text.",
		Text:      "helo wrld",
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

// defaultTestModel returns a sensible default model name for testing a provider connection.
func defaultTestModel(providerType string) string {
	models := KnownModels(providerType)
	if len(models) > 0 {
		return models[0].Name
	}
	return ""
}

// TestProviderConnection tests a saved provider by provider type.
func (s *SettingsService) TestProviderConnection(providerType string) string {
	guiLog("[GUI] JS called: TestProviderConnection(%s)", providerType)
	prov, ok := s.cfgCopy.Providers[providerType]
	if !ok {
		guiLog("[GUI] TestProviderConnection: provider %q not found in config", providerType)
		return "error: provider not found"
	}

	// Pick a default model for this provider type.
	model := defaultTestModel(providerType)
	if model == "" {
		return "error: no known test model for provider"
	}

	guiLog("[GUI] TestProviderConnection: provider=%s model=%s endpoint=%q", providerType, model, prov.APIEndpoint)

	// Ollama and local need much longer timeout — first request loads model into memory.
	timeout := 10 * time.Second
	maxTokens := 64
	timeoutMs := 10000
	if providerType == "ollama" || providerType == "local" {
		timeout = 120 * time.Second
		maxTokens = 512
		timeoutMs = 120000
		guiLog("[GUI] %s detected — using %s timeout, %d max_tokens", providerType, timeout, maxTokens)
	}

	def := config.LLMProviderDef{
		Provider:     providerType,
		APIKey:       prov.APIKey,
		Model:        model,
		APIEndpoint:  prov.APIEndpoint,
		RefreshToken: prov.RefreshToken,
		MaxTokens:    maxTokens,
		TimeoutMs:    timeoutMs,
	}

	client, err := llm.NewClientFromDef(def)
	if err != nil {
		guiLog("[GUI] TestProviderConnection: NewClientFromDef failed: %v", err)
		return fmt.Sprintf("error: %v", err)
	}
	defer client.Close()

	guiLog("[GUI] TestProviderConnection: sending test request (timeout=%s)...", timeout)
	tctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()
	_, err = client.Send(tctx, llm.Request{
		Prompt:    "Fix spelling. Return ONLY the corrected text.",
		Text:      "helo wrld",
		MaxTokens: maxTokens,
	})
	elapsed := time.Since(start)
	if err != nil {
		guiLog("[GUI] TestProviderConnection FAILED after %s: %v", elapsed, err)
		return fmt.Sprintf("error: %v", err)
	}

	guiLog("[GUI] TestProviderConnection OK (elapsed=%s)", elapsed)
	return "ok"
}

// TestProvider tests a saved model entry by label.
func (s *SettingsService) TestProvider(label string) string {
	guiLog("[GUI] JS called: TestProvider(%s)", label)
	me, ok := s.cfgCopy.Models[label]
	if !ok {
		guiLog("[GUI] TestProvider: model %q not found in config", label)
		return "error: model not found"
	}

	prov, ok := s.cfgCopy.Providers[me.Provider]
	if !ok {
		guiLog("[GUI] TestProvider: provider %q not found for model %q", me.Provider, label)
		return "error: provider not found"
	}

	guiLog("[GUI] TestProvider: provider=%s model=%s endpoint=%q", me.Provider, me.Model, prov.APIEndpoint)

	// Ollama and local need much longer timeout — first request loads model into memory.
	timeout := 10 * time.Second
	maxTokens := 64
	timeoutMs := 10000
	if me.Provider == "ollama" || me.Provider == "local" {
		timeout = 120 * time.Second
		maxTokens = 512
		timeoutMs = 120000
		guiLog("[GUI] %s detected — using %s timeout, %d max_tokens", me.Provider, timeout, maxTokens)
	}

	def := config.LLMProviderDef{
		Provider:     me.Provider,
		APIKey:       prov.APIKey,
		Model:        me.Model,
		APIEndpoint:  prov.APIEndpoint,
		RefreshToken: prov.RefreshToken,
		MaxTokens:    maxTokens,
		TimeoutMs:    timeoutMs,
	}

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
		Prompt:    "Fix spelling. Return ONLY the corrected text.",
		Text:      "helo wrld",
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

// SetRefreshToken stores an OAuth refresh token for a provider type.
func (s *SettingsService) SetRefreshToken(providerType, token string) string {
	guiLog("[GUI] JS called: SetRefreshToken(%s)", providerType)
	prov, ok := s.cfgCopy.Providers[providerType]
	if !ok {
		return "error: provider not found"
	}
	prov.RefreshToken = token
	s.cfgCopy.Providers[providerType] = prov
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
