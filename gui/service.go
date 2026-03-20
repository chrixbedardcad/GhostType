package gui

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/chrixbedardcad/GhostSpell/llm"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chrixbedardcad/GhostSpell/config"
	"github.com/chrixbedardcad/GhostSpell/internal/version"
	"github.com/chrixbedardcad/GhostSpell/sound"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// EmitConfigChanged notifies the settings UI that config state has changed.
// Replaces the 5-second polling loop with event-driven updates.
func EmitConfigChanged() {
	app := application.Get()
	if app != nil {
		app.Event.Emit("configChanged")
	}
}

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
	liveCfg    *config.Config // live config from app.go — used by indicator when cfgCopy is nil
	liveMu     *sync.Mutex    // protects liveCfg reads (shared with app.go)
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

	// ResetClientsFn is called before deleting a local model to unload the
	// Ghost-AI engine and release the file lock on Windows.
	ResetClientsFn func()

	// RefreshTrayMenuFn refreshes the tray menu to reflect prompt changes.
	RefreshTrayMenuFn func()

	// SetActivePromptFn sets the active prompt index with proper synchronization.
	// Wired by app.go to call setActivePrompt + router.SetPrompt under mutex.
	SetActivePromptFn func(idx int)

	// ReloadSTTFn re-initializes the speech-to-text engine after the voice model changes.
	ReloadSTTFn func()

	// Stats callbacks.
	GetStatsFn    func() string
	ClearStatsFn  func()
	RecordStatFn  func(prompt, promptIcon, provider, model, label, status, errMsg, output string, inputChars, durationMs int)

	// Permission callbacks — set by app.go for macOS permission checks.
	CheckAccessibilityFn      func() bool
	CheckPostEventAccessFn    func() bool
	OpenPermissionsFn         func()
	OpenAccessibilityPaneFn   func()
	OpenInputMonitoringPaneFn func()

	// Restarting is set when the user requests a restart for permissions.
	// Prevents the onCancel handler from treating this as a user-cancelled wizard.
	Restarting bool

	downloadProgress atomic.Value // stores *llm.DownloadProgress
}

// Reset reinitializes the service for a new settings session. Called each time
// the settings window is opened from the tray so the service operates on a
// fresh copy of the live config.
func (s *SettingsService) Reset(cfg *config.Config, configPath string, onSaved func()) {
	cfgCopy := *cfg
	if cfg.Providers != nil {
		cfgCopy.Providers = make(map[string]config.ProviderConfig, len(cfg.Providers))
		for k, v := range cfg.Providers {
			cfgCopy.Providers[k] = v
		}
	}
	if cfg.Models != nil {
		cfgCopy.Models = make(map[string]config.ModelEntry, len(cfg.Models))
		for k, v := range cfg.Models {
			cfgCopy.Models[k] = v
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

// SetLiveConfig stores a reference to the live config from app.go.
// The indicator uses this when cfgCopy is nil (Settings window not open).
func (s *SettingsService) SetLiveConfig(cfg *config.Config, mu *sync.Mutex) {
	s.liveCfg = cfg
	s.liveMu = mu
}

// indicatorCfg returns the best available config for indicator methods.
// Prefers cfgCopy (settings session copy) when available, falls back to liveCfg.
func (s *SettingsService) indicatorCfg() *config.Config {
	if s.cfgCopy != nil {
		return s.cfgCopy
	}
	if s.liveMu != nil {
		s.liveMu.Lock()
		defer s.liveMu.Unlock()
	}
	return s.liveCfg
}

// GetVersion returns the app version string.
func (s *SettingsService) GetVersion() string {
	return version.Version
}

// GetWhatsNew returns the changelog for the current version if the user
// hasn't seen it yet. Returns empty string if already dismissed.
func (s *SettingsService) GetWhatsNew() string {
	if s.cfgCopy.LastSeenVersion == version.Version {
		return "" // already seen
	}
	return whatsNewHTML
}

// GetWhatsNewAlways returns the changelog regardless of whether it was dismissed.
func (s *SettingsService) GetWhatsNewAlways() string {
	return whatsNewHTML
}

// DismissWhatsNew marks the current version as seen so the popup won't show again.
func (s *SettingsService) DismissWhatsNew() string {
	s.cfgCopy.LastSeenVersion = version.Version
	if err := s.validateAndSave(); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return "ok"
}

// whatsNewHTML is the changelog shown after an update. Update this with each release.
const whatsNewHTML = `
<ul>
<li><strong>🔒 macOS permissions preserved</strong> — updates no longer require re-granting Accessibility and Input Monitoring</li>
<li><strong>Bulletproof self-updater</strong> — downloads signed DMG on macOS, binary swap on Windows/Linux</li>
<li><strong>Usage stats &amp; benchmark</strong> — track model performance, run benchmarks, compare response times</li>
<li><strong>LM Studio support</strong> — auto-detects loaded models, setup guide in settings</li>
<li><strong>Faster local models</strong> — Qwen3.5 thinking properly disabled, response times cut from minutes to seconds</li>
<li><strong>Per-prompt timeout</strong> — set custom timeouts per prompt (e.g. Ask needs more time than Correct)</li>
<li><strong>Provider test status</strong> — cards show green "Connected" or red "Error" after testing</li>
<li><strong>Indicator position fix</strong> — no longer blocks clicks at screen center on Windows</li>
</ul>
`

// GetKnownModels returns a curated model list for the given provider.
// For LM Studio, dynamically queries the server for loaded models.
func (s *SettingsService) GetKnownModels(provider string) string {
	guiLog("[GUI] JS called: GetKnownModels(%s)", provider)

	// LM Studio: query the server for real loaded models.
	if provider == "lmstudio" {
		endpoint := ""
		if prov, ok := s.cfgCopy.Providers["lmstudio"]; ok {
			endpoint = prov.APIEndpoint
		}
		if _, modelNames, err := llm.LMStudioStatus(endpoint); err == nil && len(modelNames) > 0 {
			var models []ModelInfo
			for _, name := range modelNames {
				models = append(models, ModelInfo{Name: name})
			}
			data, _ := json.Marshal(models)
			return string(data)
		}
		// Fallback to static list if server not reachable.
	}

	models := KnownModels(provider)
	data, _ := json.Marshal(models)
	return string(data)
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

// --- ChatGPT OAuth ----------------------------------------------------------

// StartChatGPTOAuth initiates the OpenAI OAuth PKCE flow in the background.
// Returns "started" immediately. Use PollOAuthResult to check for completion.
func (s *SettingsService) StartChatGPTOAuth() string {
	guiLog("[GUI] JS called: StartChatGPTOAuth")
	startOpenAIOAuthAsync()
	return "started"
}

// PollOAuthResult checks the status of the OAuth flow.
// Returns "pending", "error: ...", or "ok:{...json...}"
func (s *SettingsService) PollOAuthResult() string {
	return getOAuthResult()
}

// --- Update management -----------------------------------------------------

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

	apiURL := "https://api.github.com/repos/chrixbedardcad/GhostSpell/releases/latest"
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
	// Compare versions properly: only show update if latest > current.
	// Simple != comparison falsely triggers when the user is ahead of
	// the latest GitHub release (e.g. rapid release cycles).
	hasUpdate := versionGreater(latest, current)

	result := map[string]interface{}{
		"current":    current,
		"latest":     latest,
		"has_update": hasUpdate,
		"url":        rel.HTMLURL,
	}
	data, _ := json.Marshal(result)
	return string(data)
}

// UpdateNow downloads the new binary in-process, verifies it, swaps the
// old binary with the new one, and relaunches. No external scripts needed.
// Progress is reported via GetUpdateProgress() for JS polling.
func (s *SettingsService) UpdateNow() string {
	guiLog("[GUI] JS called: UpdateNow")

	assetName := updateAssetName()
	if assetName == "" {
		return "error: unsupported platform"
	}

	// Backup config before anything else.
	if s.cfgCopy != nil && s.configPath != "" {
		_ = config.WriteDefault(s.configPath, s.cfgCopy)
		backupPath := s.configPath + ".bak"
		if data, err := os.ReadFile(s.configPath); err == nil {
			os.WriteFile(backupPath, data, 0600)
			guiLog("[GUI] UpdateNow: config backed up to %s", backupPath)
		}
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		setProgress := func(p UpdateProgress) {
			s.downloadProgress.Store(&p)
			if p.Error != "" {
				guiLog("[GUI] UpdateNow error: %s", p.Error)
			}
		}

		// 1. Fetch release info.
		setProgress(UpdateProgress{Phase: "downloading", Percent: 0})
		rel, err := fetchReleaseInfo(ctx)
		if err != nil {
			setProgress(UpdateProgress{Phase: "error", Error: err.Error()})
			return
		}

		// Find the asset for this platform.
		var assetURL string
		var assetSize int64
		for _, a := range rel.Assets {
			if a.Name == assetName {
				assetURL = a.BrowserDownloadURL
				assetSize = a.Size
				break
			}
		}
		if assetURL == "" {
			setProgress(UpdateProgress{Phase: "error", Error: fmt.Sprintf("release %s has no asset %s", rel.TagName, assetName)})
			return
		}

		// 2. Resolve current binary path.
		execPath, err := os.Executable()
		if err != nil {
			setProgress(UpdateProgress{Phase: "error", Error: fmt.Sprintf("cannot find executable: %v", err)})
			return
		}
		execPath, _ = filepath.EvalSymlinks(execPath)

		// Use temp dir for DMGs (large files), binary dir for binary swaps.
		var tmpPath string
		if strings.HasSuffix(assetName, ".dmg") {
			tmpPath = filepath.Join(os.TempDir(), assetName)
		} else {
			tmpPath = execPath + ".tmp"
		}

		guiLog("[GUI] UpdateNow: downloading %s (%d bytes) to %s", assetName, assetSize, tmpPath)

		// 3. Download to temp file.
		if err := downloadToFile(ctx, assetURL, tmpPath, assetSize, func(p UpdateProgress) {
			setProgress(p)
		}); err != nil {
			setProgress(UpdateProgress{Phase: "error", Error: err.Error()})
			return
		}

		// 4. Install the update.
		setProgress(UpdateProgress{Phase: "installing", Percent: 100})
		if runtime.GOOS == "darwin" && strings.HasSuffix(assetName, ".dmg") {
			// macOS: install from signed DMG to preserve code signature.
			// This keeps TCC grants (Accessibility, Input Monitoring) intact (#193).
			guiLog("[GUI] UpdateNow: installing from DMG %s", tmpPath)
			if err := installFromDMG(tmpPath, execPath); err != nil {
				setProgress(UpdateProgress{Phase: "error", Error: err.Error()})
				return
			}
			os.Remove(tmpPath) // Clean up downloaded DMG.
		} else {
			// Windows/Linux: swap the binary directly.
			guiLog("[GUI] UpdateNow: swapping binary %s", execPath)
			if err := swapBinary(execPath, tmpPath); err != nil {
				setProgress(UpdateProgress{Phase: "error", Error: err.Error()})
				return
			}
		}

		// 5. Relaunch and exit.
		setProgress(UpdateProgress{Phase: "restarting", Percent: 100})
		guiLog("[GUI] UpdateNow: update complete, relaunching...")
		launchAndExit(execPath)
	}()

	return "ok"
}

// GetUpdateProgress returns the current update progress as JSON.
func (s *SettingsService) GetUpdateProgress() string {
	v := s.downloadProgress.Load()
	if v == nil {
		return ""
	}
	p, ok := v.(*UpdateProgress)
	if !ok || p == nil {
		return ""
	}
	data, _ := json.Marshal(p)
	return string(data)
}

// --- Debug tools -----------------------------------------------------------

// EnableDebug activates debug-level logging. Returns the log file path.
// GetStats returns usage statistics as JSON.
func (s *SettingsService) GetStats() string {
	if s.GetStatsFn != nil {
		return s.GetStatsFn()
	}
	return "{}"
}

// ClearStats resets all usage statistics.
func (s *SettingsService) ClearStats() string {
	if s.ClearStatsFn != nil {
		s.ClearStatsFn()
	}
	return "ok"
}

// OpenStatsFile opens the stats.json file in the OS default editor.
func (s *SettingsService) OpenStatsFile() string {
	guiLog("[GUI] JS called: OpenStatsFile")
	dir := filepath.Dir(s.configPath)
	path := filepath.Join(dir, "stats.json")
	if _, err := os.Stat(path); err != nil {
		return "error: stats file not found"
	}
	OpenFile(path)
	return "ok"
}

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
	// Write a header entry so the cleared log starts with context.
	header := fmt.Sprintf("=== Log cleared at %s ===\nGhostSpell v%s | %s/%s\n",
		time.Now().Format("2006-01-02 15:04:05"),
		version.Version,
		runtime.GOOS, runtime.GOARCH)
	os.WriteFile(path, []byte(header), 0644)
	slog.Info("Debug log cleared", "version", version.Version, "os", runtime.GOOS, "arch", runtime.GOARCH)
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

// --- Result popup ----------------------------------------------------------

// GetResultText returns the current popup result text (called from result.html JS).
func (s *SettingsService) GetResultText() string {
	return GetResultText()
}

// GetResultMeta returns JSON metadata about the current result.
func (s *SettingsService) GetResultMeta() string {
	return GetResultMeta()
}

// CopyResultText copies the current result text to the system clipboard.
func (s *SettingsService) CopyResultText() string {
	text := GetResultText()
	if text == "" {
		return "error: no result text"
	}
	cmd := exec.Command("sh", "-c", "echo -n '' | pbcopy") // will be overridden below
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
		cmd.Stdin = strings.NewReader(text)
	case "linux":
		cmd = exec.Command("xclip", "-selection", "clipboard")
		cmd.Stdin = strings.NewReader(text)
	case "windows":
		cmd = exec.Command("powershell", "-NoProfile", "-Command", "Set-Clipboard -Value $input")
		cmd.Stdin = strings.NewReader(text)
	}
	if err := cmd.Run(); err != nil {
		guiLog("[GUI] CopyResultText: clipboard write failed: %v", err)
		return fmt.Sprintf("error: %v", err)
	}
	return "ok"
}

// CloseResultWindow closes the result popup window.
func (s *SettingsService) CloseResultWindow() string {
	CloseResultWindow()
	return "ok"
}

// GetPlatform returns the current OS (darwin, windows, linux).
func (s *SettingsService) GetPlatform() string {
	return runtime.GOOS
}

// --- Indicator interactions (#211) ------------------------------------------

// ShowCurrentPrompt shows the current active prompt as a pop indicator (no cycling).
func (s *SettingsService) ShowCurrentPrompt() string {
	slog.Info("[GUI] ShowCurrentPrompt called")
	cfg := s.indicatorCfg()
	if cfg == nil || len(cfg.Prompts) == 0 {
		return "error: no prompts"
	}
	idx := cfg.ActivePrompt
	if idx < 0 || idx >= len(cfg.Prompts) {
		idx = 0
	}
	p := cfg.Prompts[idx]
	go sound.PlayClick()
	SetCurrentPromptFlags(p.Voice, p.Vision)
	PopIndicatorWithModel(p.Icon, p.Name, cfg.DefaultModel)
	return "ok"
}

// CyclePromptFromIndicator cycles to the next enabled prompt (called from indicator click).
// Disabled prompts are skipped.
func (s *SettingsService) CyclePromptFromIndicator() string {
	slog.Info("[GUI] CyclePromptFromIndicator called")
	cfg := s.indicatorCfg()
	if cfg == nil || len(cfg.Prompts) == 0 {
		slog.Warn("[GUI] CyclePromptFromIndicator: no config or prompts available")
		return "error: no prompts"
	}
	next, found := config.NextEnabledPrompt(cfg.Prompts, cfg.ActivePrompt)
	if !found {
		slog.Warn("[GUI] CyclePromptFromIndicator: all prompts disabled")
		return "error: all prompts disabled"
	}
	// Use the app.go callback for proper mutex + router sync.
	if s.SetActivePromptFn != nil {
		s.SetActivePromptFn(next)
	} else {
		cfg.ActivePrompt = next
	}
	p := cfg.Prompts[next]
	slog.Info("[GUI] CyclePromptFromIndicator: cycled", "index", next, "name", p.Name)
	SetCurrentPromptFlags(p.Voice, p.Vision)
	PopIndicatorWithModel(p.Icon, p.Name, cfg.DefaultModel)
	EmitConfigChanged()
	return "ok"
}

// OpenSettingsFromIndicator opens the settings window (called from indicator double-click).
func (s *SettingsService) OpenSettingsFromIndicator() string {
	slog.Debug("[GUI] OpenSettingsFromIndicator called")
	// Trigger settings via the tray menu callback pattern.
	// The actual ShowSettings call requires the live config from app.go,
	// so we use the OnIndicatorOpenSettings callback if set.
	if OnIndicatorOpenSettings != nil {
		OnIndicatorOpenSettings()
	}
	return "ok"
}

// QuitFromIndicator quits the app (called from indicator context menu).
func (s *SettingsService) QuitFromIndicator() string {
	slog.Info("[GUI] QuitFromIndicator called")
	go func() {
		time.Sleep(200 * time.Millisecond)
		os.Exit(0)
	}()
	return "ok"
}

// GetIndicatorMenu returns the context menu data for the indicator as JSON.
// Disabled prompts are excluded from the menu.
func (s *SettingsService) GetIndicatorMenu() string {
	slog.Info("[GUI] GetIndicatorMenu called")
	type menuPrompt struct {
		Name   string `json:"name"`
		Icon   string `json:"icon"`
		Active bool   `json:"active"`
		Index  int    `json:"index"` // real index into cfg.Prompts
	}
	type menuData struct {
		Prompts       []menuPrompt `json:"prompts"`
		Version       string       `json:"version"`
		ActiveModel   string       `json:"activeModel"`
		IndicatorMode string       `json:"indicatorMode"`
	}
	var data menuData
	data.Version = version.Version
	indicatorMu.Lock()
	data.IndicatorMode = indicatorMode
	indicatorMu.Unlock()
	cfg := s.indicatorCfg()
	if cfg != nil {
		for i, p := range cfg.Prompts {
			if p.Disabled {
				continue
			}
			data.Prompts = append(data.Prompts, menuPrompt{
				Name:   p.Name,
				Icon:   p.Icon,
				Active: i == cfg.ActivePrompt,
				Index:  i,
			})
		}
		data.ActiveModel = cfg.DefaultModel
	} else {
		slog.Warn("[GUI] GetIndicatorMenu: no config available")
	}
	j, _ := json.Marshal(data)
	slog.Info("[GUI] GetIndicatorMenu: returning", "prompts", len(data.Prompts))
	return string(j)
}

// SetActivePromptFromIndicator sets the active prompt by index (called from indicator menu).
func (s *SettingsService) SetActivePromptFromIndicator(idx int) string {
	slog.Info("[GUI] SetActivePromptFromIndicator called", "idx", idx)
	cfg := s.indicatorCfg()
	if cfg == nil || idx < 0 || idx >= len(cfg.Prompts) {
		slog.Warn("[GUI] SetActivePromptFromIndicator: invalid", "cfg_nil", cfg == nil, "idx", idx)
		return "error: invalid index"
	}
	// Use the app.go callback for proper mutex + router sync.
	if s.SetActivePromptFn != nil {
		s.SetActivePromptFn(idx)
	} else {
		cfg.ActivePrompt = idx
	}
	p := cfg.Prompts[idx]
	slog.Info("[GUI] SetActivePromptFromIndicator: set", "index", idx, "name", p.Name)
	SetCurrentPromptFlags(p.Voice, p.Vision)
	PopIndicator(p.Icon, p.Name)
	EmitConfigChanged()
	return "ok"
}

// SetIndicatorModeFromIndicator changes the indicator display mode from the context menu.
func (s *SettingsService) SetIndicatorModeFromIndicator(mode string) string {
	slog.Info("[GUI] SetIndicatorModeFromIndicator called", "mode", mode)
	if mode != "always" && mode != "processing" && mode != "hidden" {
		return "error: invalid mode"
	}
	SetIndicatorMode(mode)
	cfg := s.indicatorCfg()
	if cfg != nil {
		cfg.IndicatorMode = mode
		if mode == "always" {
			go ShowIdle()
		} else {
			go HideIndicator()
		}
	}
	SaveIndicatorMode(mode)
	return "ok"
}

// GetActivePromptInfo returns the active prompt name, icon, and mode flags as JSON.
func (s *SettingsService) GetActivePromptInfo() string {
	type info struct {
		Name   string `json:"name"`
		Icon   string `json:"icon"`
		Voice  bool   `json:"voice"`
		Vision bool   `json:"vision"`
	}
	cfg := s.indicatorCfg()
	if cfg == nil || len(cfg.Prompts) == 0 {
		return "{}"
	}
	idx := cfg.ActivePrompt
	if idx < 0 || idx >= len(cfg.Prompts) {
		idx = 0
	}
	p := cfg.Prompts[idx]
	j, _ := json.Marshal(info{Name: p.Name, Icon: p.Icon, Voice: p.Voice, Vision: p.Vision})
	return string(j)
}

// IndicatorReady is called by the React indicator when Wails runtime is confirmed ready.
// Emits the current state so React syncs up with Go (fixes event race condition).
func (s *SettingsService) IndicatorReady() string {
	slog.Info("[GUI] IndicatorReady: React confirmed Wails runtime available")
	// Set voice/vision flags for the active prompt so the first event has them.
	cfg := s.indicatorCfg()
	if cfg != nil && len(cfg.Prompts) > 0 {
		idx := cfg.ActivePrompt
		if idx >= 0 && idx < len(cfg.Prompts) {
			p := cfg.Prompts[idx]
			SetCurrentPromptFlags(p.Voice, p.Vision)
		}
	}
	indicatorMu.Lock()
	mode := indicatorMode
	indicatorMu.Unlock()
	// Re-emit current state so React picks it up.
	if mode == "always" {
		emitIndicatorEvent(map[string]any{"state": "idle"})
	}
	return "ok"
}

// MoveIndicatorWindow moves the indicator window to the given screen position.
// Called from JS drag handler since window.moveTo() doesn't work in WebView2.
func (s *SettingsService) MoveIndicatorWindow(x, y int) string {
	indicatorMu.Lock()
	win := indicatorWin
	indicatorMu.Unlock()
	if win != nil {
		win.SetPosition(x, y)
	}
	return "ok"
}

// GetIndicatorWindowPosition returns the current indicator window position as JSON.
func (s *SettingsService) GetIndicatorWindowPosition() string {
	indicatorMu.Lock()
	win := indicatorWin
	indicatorMu.Unlock()
	if win == nil {
		return `{"x":0,"y":0}`
	}
	x, y := win.Position()
	return fmt.Sprintf(`{"x":%d,"y":%d}`, x, y)
}

// ResizeIndicatorForMenu temporarily resizes the indicator window for the context menu (#214).
func (s *SettingsService) ResizeIndicatorForMenu(width, height int) string {
	indicatorMu.Lock()
	win := indicatorWin
	indicatorMu.Unlock()
	if win != nil {
		win.SetSize(width, height)
	}
	return "ok"
}

// OnIndicatorOpenSettings is a callback set by app.go to open settings from the indicator.
var OnIndicatorOpenSettings func()

// GetSystemRAMGB returns the approximate total system RAM in gigabytes.
// Used by the wizard to recommend an appropriate local model (#191).
func (s *SettingsService) GetSystemRAMGB() int {
	return getSystemRAMGB()
}

// --- Permissions -----------------------------------------------------------

// CheckPermissions returns a JSON object with macOS permission status.
// On non-macOS platforms, all permissions return true.
func (s *SettingsService) CheckPermissions() string {
	// Debug level — this is polled every 2 seconds by the wizard (#209).
	slog.Debug("[GUI] CheckPermissions called")
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
	slog.Debug("Permission check", "accessibility", ax, "postEvent", post)
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

// QuitForRestart closes the app and relaunches it so macOS permissions take effect.
func (s *SettingsService) QuitForRestart() string {
	guiLog("[GUI] JS called: QuitForRestart")
	s.Restarting = true

	execPath, err := os.Executable()
	if err != nil {
		guiLog("[GUI] QuitForRestart: could not get executable path: %v", err)
		go func() {
			time.Sleep(300 * time.Millisecond)
			os.Exit(0)
		}()
		return "ok"
	}

	// On macOS .app bundles, derive the .app path for `open` command.
	// execPath is like /Applications/GhostSpell.app/Contents/MacOS/GhostSpell
	appPath := execPath
	if idx := strings.Index(execPath, ".app/"); idx != -1 {
		appPath = execPath[:idx+4] // "/Applications/GhostSpell.app"
	}

	// Launch a shell process that waits 1 second then reopens the app.
	// The shell process is independent — it survives os.Exit(0).
	var relaunchCmd *exec.Cmd
	if runtime.GOOS == "darwin" && strings.HasSuffix(appPath, ".app") {
		relaunchCmd = exec.Command("sh", "-c", "sleep 1 && open '"+appPath+"'")
	} else {
		relaunchCmd = exec.Command("sh", "-c", "sleep 1 && '"+execPath+"'")
	}
	if err := relaunchCmd.Start(); err != nil {
		guiLog("[GUI] QuitForRestart: failed to schedule relaunch: %v", err)
	} else {
		guiLog("[GUI] QuitForRestart: relaunch scheduled (PID %d)", relaunchCmd.Process.Pid)
	}

	// Quit the current instance after a short delay for the JS response.
	go func() {
		time.Sleep(300 * time.Millisecond)
		os.Exit(0)
	}()

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

// versionGreater returns true if a > b using semantic versioning.
// Compares each numeric segment left to right (e.g. "0.26.13" > "0.26.11").
func versionGreater(a, b string) bool {
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	for i := 0; i < len(as) || i < len(bs); i++ {
		var ai, bi int
		if i < len(as) {
			fmt.Sscan(as[i], &ai)
		}
		if i < len(bs) {
			fmt.Sscan(bs[i], &bi)
		}
		if ai > bi {
			return true
		}
		if ai < bi {
			return false
		}
	}
	return false
}
