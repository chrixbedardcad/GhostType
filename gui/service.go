package gui

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/chrixbedardcad/GhostType/config"
	"github.com/chrixbedardcad/GhostType/internal/version"
	"github.com/chrixbedardcad/GhostType/llm"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// guiLog logs to both slog (log file) and fmt (stdout, if console attached).
func guiLog(msg string, args ...any) {
	formatted := fmt.Sprintf(msg, args...)
	fmt.Println(formatted)
	slog.Info(formatted)
}

// SettingsService exposes all Settings GUI bindings as public methods.
// Wails v3 auto-binds all public methods for JS access via wails.Call().
type SettingsService struct {
	cfgCopy    *config.Config
	configPath string
	onSaved    func()
	app        *application.App
	window     *application.WebviewWindow
	saved      bool
	standalone bool // true for first-launch (app.Quit on close), false when tray app is running

	// Debug callbacks — set by app.go to access the debugState.
	DebugEnableFn  func() (string, error)
	DebugDisableFn func()
	DebugEnabledFn func() bool
	DebugLogPathFn func() string
	DebugTailFn    func() (string, error)

	// Permission callbacks — set by app.go for macOS permission checks.
	CheckAccessibilityFn      func() bool
	CheckPostEventAccessFn    func() bool
	OpenPermissionsFn         func()
	OpenAccessibilityPaneFn   func()
	OpenInputMonitoringPaneFn func()
}

// Reset reinitializes the service for a new settings session. Called each time
// the settings window is opened from the tray so the service operates on a
// fresh copy of the live config.
func (s *SettingsService) Reset(cfg *config.Config, configPath string, onSaved func()) {
	cfgCopy := *cfg
	if cfg.LLMProviders != nil {
		cfgCopy.LLMProviders = make(map[string]config.LLMProviderDef, len(cfg.LLMProviders))
		for k, v := range cfg.LLMProviders {
			cfgCopy.LLMProviders[k] = v
		}
	}
	if cfg.Prompts != nil {
		cfgCopy.Prompts = make([]config.PromptEntry, len(cfg.Prompts))
		copy(cfgCopy.Prompts, cfg.Prompts)
	}
	s.cfgCopy = &cfgCopy
	s.configPath = configPath
	s.onSaved = onSaved
	s.window = nil
	s.saved = false
}

// clearLegacyAndSave writes the config to disk, clearing legacy flat fields
// and removing any provider entries that are clearly invalid (no API key for
// non-ollama providers). This prevents phantom entries synthesized from stale
// legacy fields from poisoning the saved config.
func (s *SettingsService) clearLegacyAndSave() error {
	s.cfgCopy.LLMProvider = ""
	s.cfgCopy.APIKey = ""
	s.cfgCopy.Model = ""
	s.cfgCopy.APIEndpoint = ""

	// Remove invalid provider entries (e.g. phantom "default" from legacy synthesis).
	for label, def := range s.cfgCopy.LLMProviders {
		if def.Provider != "ollama" && def.APIKey == "" {
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

// GetVersion returns the app version string.
func (s *SettingsService) GetVersion() string {
	return version.Version
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

// GetKnownModels returns a curated model list for the given provider.
func (s *SettingsService) GetKnownModels(provider string) string {
	guiLog("[GUI] JS called: GetKnownModels(%s)", provider)
	models := KnownModels(provider)
	data, _ := json.Marshal(models)
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

	if err := s.clearLegacyAndSave(); err != nil {
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

	if err := s.clearLegacyAndSave(); err != nil {
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

	if err := s.clearLegacyAndSave(); err != nil {
		return fmt.Sprintf("error: %v", err)
	}

	return "ok"
}

// TestConnection tests provider credentials.
func (s *SettingsService) TestConnection(provider, apiKey, model, endpoint string) string {
	guiLog("[GUI] JS called: TestConnection(provider=%s, model=%s, endpoint=%q)", provider, model, endpoint)

	// Ollama needs much longer timeout — first request loads model into memory.
	timeout := 10 * time.Second
	timeoutMs := 10000
	if provider == "ollama" {
		timeout = 120 * time.Second
		timeoutMs = 120000
		guiLog("[GUI] Ollama detected — using %s timeout", timeout)
	}

	def := config.LLMProviderDef{
		Provider:    provider,
		APIKey:      apiKey,
		Model:       model,
		APIEndpoint: endpoint,
		MaxTokens:   32,
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
		MaxTokens: 32,
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

	// Ollama needs much longer timeout — first request loads model into memory.
	timeout := 10 * time.Second
	timeoutMs := 10000
	if def.Provider == "ollama" {
		timeout = 120 * time.Second
		timeoutMs = 120000
		guiLog("[GUI] Ollama detected — using %s timeout", timeout)
	}

	def.MaxTokens = 32
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
		MaxTokens: 32,
	})
	elapsed := time.Since(start)
	if err != nil {
		guiLog("[GUI] TestProvider FAILED after %s: %v", elapsed, err)
		return fmt.Sprintf("error: %v", err)
	}

	guiLog("[GUI] TestProvider OK (elapsed=%s)", elapsed)
	return "ok"
}

// OpenConfigFile opens the config file in the OS default editor.
func (s *SettingsService) OpenConfigFile() string {
	guiLog("[GUI] JS called: OpenConfigFile")
	OpenFile(s.configPath)
	return "ok"
}

// CloseWindow terminates the settings window.
func (s *SettingsService) CloseWindow() string {
	guiLog("[GUI] JS called: CloseWindow (standalone=%v)", s.standalone)
	if s.standalone && s.app != nil {
		// First-launch standalone app — quit the whole app to unblock Run().
		s.app.Quit()
	} else if s.window != nil {
		// Tray-running mode — just close the settings window, keep app alive.
		s.window.Close()
	}
	return "ok"
}

// OllamaStatus checks if Ollama is running and returns status + models in one call.
func (s *SettingsService) OllamaStatus(endpoint string) string {
	guiLog("[GUI] JS called: OllamaStatus(%s)", endpoint)
	st := ollamaGetStatus(endpoint)

	result := map[string]interface{}{
		"status": st["status"],
	}
	if v, ok := st["version"]; ok {
		result["version"] = v
	}

	// If running, include models in the same response.
	if st["status"] == "running" {
		base := ollamaBaseURL(endpoint)
		models, err := ollamaFetchModels(base)
		if err != nil {
			guiLog("[GUI] OllamaStatus model fetch error: %v", err)
			result["models"] = []ollamaModelInfo{}
		} else {
			result["models"] = models
		}
	}

	data, _ := json.Marshal(result)
	return string(data)
}

// OllamaListModels returns the list of locally-installed models.
func (s *SettingsService) OllamaListModels(endpoint string) string {
	guiLog("[GUI] JS called: OllamaListModels(%s)", endpoint)
	base := ollamaBaseURL(endpoint)
	models, err := ollamaFetchModels(base)
	if err != nil {
		guiLog("[GUI] OllamaListModels error: %v", err)
		return "[]"
	}
	data, _ := json.Marshal(models)
	return string(data)
}

// OllamaPullModel runs "ollama pull" synchronously (up to 10 minutes).
func (s *SettingsService) OllamaPullModel(model string) string {
	guiLog("[GUI] JS called: OllamaPullModel(%s)", model)
	if err := ollamaPullModelSync(model); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return "ok"
}

// OllamaDownloadInstaller downloads the Ollama installer (platform-specific).
func (s *SettingsService) OllamaDownloadInstaller() string {
	guiLog("[GUI] JS called: OllamaDownloadInstaller")
	if err := ollamaDownloadInstallerPlatform(); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return "ok"
}

// --- OpenRouter OAuth -------------------------------------------------------

// StartOpenRouterOAuth initiates the OpenRouter PKCE OAuth flow in the background.
// Returns "started" immediately. Use PollOAuthResult to check for completion.
func (s *SettingsService) StartOpenRouterOAuth() string {
	guiLog("[GUI] JS called: StartOpenRouterOAuth")
	startOpenRouterOAuthAsync()
	return "started"
}

// PollOAuthResult checks the status of the OAuth flow.
// Returns "pending", "error: ...", or "ok:sk-or-v1-..."
func (s *SettingsService) PollOAuthResult() string {
	return getOAuthResult()
}

// --- Prompt management -----------------------------------------------------

// SavePrompt updates an existing prompt at the given index, or appends if index == -1.
func (s *SettingsService) SavePrompt(index int, name, prompt, llmLabel string) string {
	guiLog("[GUI] JS called: SavePrompt(idx=%d, name=%s)", index, name)
	if name == "" || prompt == "" {
		return "error: name and prompt are required"
	}
	entry := config.PromptEntry{Name: name, Prompt: prompt, LLM: llmLabel}
	if index < 0 || index >= len(s.cfgCopy.Prompts) {
		s.cfgCopy.Prompts = append(s.cfgCopy.Prompts, entry)
	} else {
		s.cfgCopy.Prompts[index] = entry
	}
	if err := s.clearLegacyAndSave(); err != nil {
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
	if err := s.clearLegacyAndSave(); err != nil {
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
	if err := s.clearLegacyAndSave(); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return "ok"
}

// ResetPrompts restores the default prompts (Correct, Polish, Translate).
func (s *SettingsService) ResetPrompts() string {
	guiLog("[GUI] JS called: ResetPrompts")
	s.cfgCopy.Prompts = config.DefaultPrompts()
	s.cfgCopy.ActivePrompt = 0
	if err := s.clearLegacyAndSave(); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return "ok"
}

// --- General settings ------------------------------------------------------

// SetSoundEnabled toggles sound.
func (s *SettingsService) SetSoundEnabled(enabled bool) string {
	guiLog("[GUI] JS called: SetSoundEnabled(%v)", enabled)
	s.cfgCopy.SoundEnabled = &enabled
	if err := s.clearLegacyAndSave(); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return "ok"
}

// SetPreserveClipboard toggles clipboard preservation.
func (s *SettingsService) SetPreserveClipboard(enabled bool) string {
	guiLog("[GUI] JS called: SetPreserveClipboard(%v)", enabled)
	s.cfgCopy.PreserveClipboard = enabled
	if err := s.clearLegacyAndSave(); err != nil {
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
	if err := s.clearLegacyAndSave(); err != nil {
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
	if err := s.clearLegacyAndSave(); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return "ok"
}

// SetLogLevel updates the log level.
func (s *SettingsService) SetLogLevel(level string) string {
	guiLog("[GUI] JS called: SetLogLevel(%s)", level)
	s.cfgCopy.LogLevel = level
	if err := s.clearLegacyAndSave(); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return "ok"
}

// CheckForUpdate checks GitHub for a newer version and returns JSON.
func (s *SettingsService) CheckForUpdate() string {
	guiLog("[GUI] JS called: CheckForUpdate")
	current := version.Version

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	type ghRelease struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}

	apiURL := "https://api.github.com/repos/chrixbedardcad/GhostType/releases/latest"
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return fmt.Sprintf(`{"current":"%s","error":"%v"}`, current, err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf(`{"current":"%s","error":"%v"}`, current, err)
	}
	defer resp.Body.Close()

	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return fmt.Sprintf(`{"current":"%s","error":"%v"}`, current, err)
	}

	latest := rel.TagName
	if len(latest) > 0 && latest[0] == 'v' {
		latest = latest[1:]
	}
	hasUpdate := latest != current

	result := map[string]interface{}{
		"current":    current,
		"latest":     latest,
		"has_update": hasUpdate,
		"url":        rel.HTMLURL,
	}
	data, _ := json.Marshal(result)
	return string(data)
}

// UpdateNow launches the platform install script in a detached process and exits
// the app. The install script handles killing the old process, downloading the
// latest release, and relaunching.
func (s *SettingsService) UpdateNow() string {
	guiLog("[GUI] JS called: UpdateNow")

	const repo = "chrixbedardcad/GhostType"
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		script := fmt.Sprintf("irm https://raw.githubusercontent.com/%s/main/scripts/install.ps1 | iex", repo)
		cmd = exec.Command("powershell", "-NoProfile", "-Command", script)
	case "darwin", "linux":
		script := fmt.Sprintf("curl -fsSL https://raw.githubusercontent.com/%s/main/scripts/install.sh | bash", repo)
		cmd = exec.Command("bash", "-c", script)
	default:
		return "error: unsupported platform"
	}

	// Detach the process so it survives our exit.
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	detachProcess(cmd)

	if err := cmd.Start(); err != nil {
		guiLog("[GUI] UpdateNow: failed to start installer: %v", err)
		return fmt.Sprintf("error: %v", err)
	}

	guiLog("[GUI] UpdateNow: installer launched (PID %d), exiting app...", cmd.Process.Pid)

	// Give the install script a moment to start, then exit.
	go func() {
		time.Sleep(1 * time.Second)
		os.Exit(0)
	}()

	return "ok"
}

// --- Debug tools -----------------------------------------------------------

// EnableDebug activates debug-level logging. Returns the log file path.
func (s *SettingsService) EnableDebug() string {
	guiLog("[GUI] JS called: EnableDebug")
	if s.DebugEnableFn == nil {
		return "error: debug not available"
	}
	path, err := s.DebugEnableFn()
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return path
}

// DisableDebug deactivates debug logging.
func (s *SettingsService) DisableDebug() string {
	guiLog("[GUI] JS called: DisableDebug")
	if s.DebugDisableFn == nil {
		return "error: debug not available"
	}
	s.DebugDisableFn()
	return "ok"
}

// GetDebugEnabled returns whether debug logging is active.
func (s *SettingsService) GetDebugEnabled() bool {
	if s.DebugEnabledFn == nil {
		return false
	}
	return s.DebugEnabledFn()
}

// GetDebugLogPath returns the path to the debug log file.
func (s *SettingsService) GetDebugLogPath() string {
	if s.DebugLogPathFn == nil {
		return ""
	}
	return s.DebugLogPathFn()
}

// OpenLogFile opens the log file in the OS default editor/viewer.
func (s *SettingsService) OpenLogFile() string {
	guiLog("[GUI] JS called: OpenLogFile")
	path := s.GetDebugLogPath()
	if path == "" {
		return "error: no log path"
	}
	OpenFile(path)
	return "ok"
}

// ClearDebugLog truncates the log file.
func (s *SettingsService) ClearDebugLog() string {
	guiLog("[GUI] JS called: ClearDebugLog")
	path := s.GetDebugLogPath()
	if path == "" {
		return "error: no log path"
	}
	if err := os.Truncate(path, 0); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return "ok"
}

// TailDebugLog returns the last ~200 lines of the log file.
func (s *SettingsService) TailDebugLog() string {
	guiLog("[GUI] JS called: TailDebugLog")
	if s.DebugTailFn == nil {
		return "error: debug not available"
	}
	tail, err := s.DebugTailFn()
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return tail
}

// CheckPermissions returns a JSON object with macOS permission status.
// On non-macOS platforms, all permissions return true.
func (s *SettingsService) CheckPermissions() string {
	guiLog("[GUI] JS called: CheckPermissions")
	ax := true
	post := true
	if s.CheckAccessibilityFn != nil {
		ax = s.CheckAccessibilityFn()
	}
	if s.CheckPostEventAccessFn != nil {
		post = s.CheckPostEventAccessFn()
	}
	result := map[string]bool{
		"accessibility": ax,
		"postEvent":     post,
		"isMac":         runtime.GOOS == "darwin",
	}
	data, _ := json.Marshal(result)
	slog.Info("Permission check from GUI", "accessibility", ax, "postEvent", post)
	return string(data)
}

// OpenPermissions opens both macOS System Settings permission panes.
func (s *SettingsService) OpenPermissions() string {
	guiLog("[GUI] JS called: OpenPermissions")
	if s.OpenPermissionsFn != nil {
		s.OpenPermissionsFn()
	}
	return "ok"
}

// OpenAccessibilityPane opens the macOS Accessibility privacy pane.
func (s *SettingsService) OpenAccessibilityPane() string {
	guiLog("[GUI] JS called: OpenAccessibilityPane")
	if s.OpenAccessibilityPaneFn != nil {
		s.OpenAccessibilityPaneFn()
	}
	return "ok"
}

// OpenInputMonitoringPane opens the macOS Input Monitoring privacy pane.
func (s *SettingsService) OpenInputMonitoringPane() string {
	guiLog("[GUI] JS called: OpenInputMonitoringPane")
	if s.OpenInputMonitoringPaneFn != nil {
		s.OpenInputMonitoringPaneFn()
	}
	return "ok"
}

// OpenFile opens a file using the platform's default handler.
func OpenFile(path string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		// Use rundll32 to open files without flashing a cmd.exe console window.
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	if err := cmd.Start(); err != nil {
		guiLog("[GUI] ERROR: Failed to open file: %v", err)
	}
}
