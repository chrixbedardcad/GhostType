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
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/chrixbedardcad/GhostSpell/config"
	"github.com/chrixbedardcad/GhostSpell/internal/version"
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
<h3>What's New in GhostSpell</h3>
<ul>
<li><strong>ChatGPT Login is now independent</strong> — fully separate provider from OpenAI API Key. No more conflicts. Both can coexist.</li>
<li><strong>Ask prompt</strong> — new default prompt for answering questions (❓)</li>
<li><strong>Cycle prompt pop</strong> — indicator pill briefly shows the new prompt when you cycle with Ctrl+Shift+T</li>
<li><strong>macOS permissions redesign</strong> — Accessibility and Input Monitoring are now two clear separate steps</li>
<li><strong>Emoji picker</strong> — full categorized emoji picker for prompt icons</li>
<li><strong>GhostAI improvements</strong> — thinking models use full context window, better answer extraction</li>
<li><strong>What's New popup</strong> — this popup! Shows automatically after each update</li>
<li><strong>Provider logos</strong> — all setup pages and model entries show provider logos</li>
<li><strong>Clean external links</strong> — standardized pill buttons for all links that open a browser</li>
</ul>
`

// GetKnownModels returns a curated model list for the given provider.
func (s *SettingsService) GetKnownModels(provider string) string {
	guiLog("[GUI] JS called: GetKnownModels(%s)", provider)
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

// UpdateNow launches the proven platform install script in a detached process
// and exits the app. The install script handles downloading, killing the old
// process, installing, and relaunching. This is the battle-tested approach.
func (s *SettingsService) UpdateNow() string {
	guiLog("[GUI] JS called: UpdateNow")

	const repo = "chrixbedardcad/GhostSpell"
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
		time.Sleep(2 * time.Second)
		os.Exit(0)
	}()

	return "ok"
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

// GetPlatform returns the current OS (darwin, windows, linux).
func (s *SettingsService) GetPlatform() string {
	return runtime.GOOS
}

// --- Permissions -----------------------------------------------------------

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
