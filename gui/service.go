package gui

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"io"
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

// updateProgress tracks download progress for the UI.
var updateProgress atomic.Value // *updateDLProgress

type updateDLProgress struct {
	Downloaded int64   `json:"downloaded"`
	Total      int64   `json:"total"`
	Percent    float64 `json:"percent"`
	Status     string  `json:"status"` // "downloading", "installing", "done", "error"
	Error      string  `json:"error,omitempty"`
}

// UpdateNow downloads the latest release with progress tracking, then installs
// and restarts. The Settings window stays open during download.
func (s *SettingsService) UpdateNow() string {
	guiLog("[GUI] JS called: UpdateNow")

	// Get the latest version tag.
	raw := s.CheckForUpdate()
	var info struct {
		Latest    string `json:"latest"`
		HasUpdate bool   `json:"has_update"`
	}
	json.Unmarshal([]byte(raw), &info)
	if !info.HasUpdate || info.Latest == "" {
		return "error: no update available"
	}

	tag := "v" + info.Latest
	const repo = "chrixbedardcad/GhostSpell"

	// Determine the asset filename for this platform.
	var assetName string
	switch runtime.GOOS {
	case "darwin":
		assetName = fmt.Sprintf("GhostSpell-darwin-%s.dmg", runtime.GOARCH)
	case "windows":
		assetName = "ghostspell-windows-amd64.exe"
	case "linux":
		assetName = fmt.Sprintf("ghostspell-linux-%s", runtime.GOARCH)
	default:
		return "error: unsupported platform"
	}

	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, tag, assetName)
	guiLog("[GUI] UpdateNow: downloading %s", url)

	// Reset progress.
	updateProgress.Store(&updateDLProgress{Status: "downloading"})

	// Download in background.
	go func() {
		tmpDir := os.TempDir()
		tmpPath := filepath.Join(tmpDir, assetName)

		err := downloadWithProgress(url, tmpPath, func(downloaded, total int64) {
			pct := 0.0
			if total > 0 {
				pct = float64(downloaded) / float64(total) * 100
			}
			updateProgress.Store(&updateDLProgress{
				Downloaded: downloaded,
				Total:      total,
				Percent:    pct,
				Status:     "downloading",
			})
		})

		if err != nil {
			guiLog("[GUI] UpdateNow: download failed: %v", err)
			updateProgress.Store(&updateDLProgress{Status: "error", Error: err.Error()})
			return
		}

		guiLog("[GUI] UpdateNow: download complete, installing from %s", tmpPath)
		updateProgress.Store(&updateDLProgress{Status: "installing", Percent: 100})

		// Launch platform-specific install from the downloaded file.
		if err := installFromFile(tmpPath); err != nil {
			guiLog("[GUI] UpdateNow: install failed: %v", err)
			updateProgress.Store(&updateDLProgress{Status: "error", Error: err.Error()})
			return
		}

		updateProgress.Store(&updateDLProgress{Status: "done", Percent: 100})

		// Exit after a brief delay.
		go func() {
			time.Sleep(1 * time.Second)
			os.Exit(0)
		}()
	}()

	return "ok"
}

// GetUpdateProgress returns the current update download progress as JSON.
func (s *SettingsService) GetUpdateProgress() string {
	p, _ := updateProgress.Load().(*updateDLProgress)
	if p == nil {
		return ""
	}
	data, _ := json.Marshal(p)
	return string(data)
}

// downloadWithProgress downloads a URL to a file, calling progressFn periodically.
func downloadWithProgress(url, dest string, progressFn func(downloaded, total int64)) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	total := resp.ContentLength
	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer out.Close()

	buf := make([]byte, 32*1024)
	var downloaded int64
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := out.Write(buf[:n]); writeErr != nil {
				return fmt.Errorf("write file: %w", writeErr)
			}
			downloaded += int64(n)
			if progressFn != nil {
				progressFn(downloaded, total)
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return fmt.Errorf("read response: %w", readErr)
		}
	}
	return nil
}

// installFromFile installs the update from a pre-downloaded file and relaunches.
func installFromFile(path string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		// Mount DMG, copy .app, unmount, relaunch.
		script := fmt.Sprintf(
			`hdiutil attach '%s' -nobrowse -quiet && `+
				`cp -R /Volumes/GhostSpell/GhostSpell.app /Applications/ && `+
				`hdiutil detach /Volumes/GhostSpell -quiet && `+
				`xattr -dr com.apple.quarantine /Applications/GhostSpell.app 2>/dev/null; `+
				`sleep 1 && open /Applications/GhostSpell.app`,
			path)
		cmd = exec.Command("bash", "-c", script)

	case "windows":
		// Write a small batch script that waits, copies, and relaunches.
		exePath := filepath.Join(os.Getenv("LOCALAPPDATA"), "GhostSpell", "ghostspell.exe")
		batPath := filepath.Join(os.TempDir(), "ghostspell-update.bat")
		bat := fmt.Sprintf("@echo off\r\ntimeout /t 2 /nobreak >nul\r\ncopy /y \"%s\" \"%s\"\r\nstart \"\" \"%s\"\r\ndel \"%%~f0\"\r\n", path, exePath, exePath)
		if err := os.WriteFile(batPath, []byte(bat), 0644); err != nil {
			return fmt.Errorf("write bat: %w", err)
		}
		cmd = exec.Command("cmd", "/c", "start", "/b", batPath)

	case "linux":
		// Copy binary, chmod, relaunch.
		script := fmt.Sprintf(
			`sleep 1 && cp '%s' /usr/local/bin/ghostspell && `+
				`chmod +x /usr/local/bin/ghostspell && `+
				`nohup ghostspell >/dev/null 2>&1 &`,
			path)
		cmd = exec.Command("bash", "-c", script)

	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	detachProcess(cmd)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start installer: %w", err)
	}
	guiLog("[GUI] installFromFile: installer launched (PID %d)", cmd.Process.Pid)
	return nil
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
