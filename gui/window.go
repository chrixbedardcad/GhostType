package gui

import (
	"fmt"
	"io/fs"
	"runtime"
	"sync"

	"github.com/chrixbedardcad/GhostSpell/assets"
	"github.com/chrixbedardcad/GhostSpell/config"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// windowAppIcon returns the best icon bytes for the current platform.
// On Windows, the multi-resolution .ico gives crisp taskbar/Start Menu icons.
func windowAppIcon() []byte {
	if runtime.GOOS == "windows" {
		return assets.AppIconICO
	}
	return assets.AppIcon512
}

// settingsGuard prevents multiple settings windows.
var (
	settingsOpen   bool
	settingsOpenMu sync.Mutex
)

// FrontendSubFS returns the embedded frontend sub-filesystem with the
// "frontend/" prefix stripped, suitable for use as a Wails asset handler.
func FrontendSubFS() (fs.FS, error) {
	return fs.Sub(frontendFS, "frontend")
}

// NewSettingsService creates a SettingsService instance. The caller should
// register it on the Wails app before calling app.Run(). Call Reset() before
// each settings window open to reinitialize the service state.
func NewSettingsService() *SettingsService {
	return &SettingsService{}
}

// ShowSettingsBlocking opens the settings window and blocks until it closes.
// Creates a standalone Wails app (used on first launch before the tray exists).
// Returns the (potentially updated) config.
func ShowSettingsBlocking(cfg *config.Config, configPath string) *config.Config {
	guiLog("[GUI] ShowSettingsBlocking called, configPath=%s", configPath)
	updated := showStandaloneWindow(cfg, configPath, nil)
	if updated != nil {
		return updated
	}
	return cfg
}

// ShowWizardOnApp creates the first-launch wizard window on the tray Wails app
// instead of creating a standalone app. This avoids the goroutine leak caused by
// running two sequential Wails apps in the same process.
// onSaved is called when the user saves a provider. onCancel is called when the
// user closes the wizard without saving.
func ShowWizardOnApp(svc *SettingsService, app *application.App, cfg *config.Config, configPath string, onSaved func(), onCancel func()) {
	guiLog("[GUI] ShowWizardOnApp: creating wizard window on tray app")

	svc.Reset(cfg, configPath, onSaved)
	svc.app = app

	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "GhostSpell Setup",
		Width:  620,
		Height: 750,
		URL:    "/wizard.html",
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:               application.MacBackdropTranslucent,
		},
	})
	svc.window = win

	win.OnWindowEvent(events.Common.WindowClosing, func(event *application.WindowEvent) {
		guiLog("[GUI] Wizard window closing (saved=%v)", svc.saved)
		if !svc.saved && onCancel != nil {
			fmt.Println("Setup cancelled — no provider configured.")
			onCancel()
		}
	})
}

// ShowSettings opens the settings window on the already-running tray Wails app.
// Non-blocking — creates a window and returns immediately.
// svc must be the SettingsService that was pre-registered on the app before Run().
func ShowSettings(svc *SettingsService, cfg *config.Config, configPath string, onSaved func()) {
	guiLog("[GUI] ShowSettings (async) called")

	// Always close any previous settings window before creating a new one.
	// This covers normal re-opens, but also edge cases where the closing
	// event was missed or the window ended up behind other windows.
	settingsOpenMu.Lock()
	prev := svc.window
	svc.window = nil
	settingsOpen = false
	settingsOpenMu.Unlock()

	if prev != nil {
		guiLog("[GUI] ShowSettings: closing previous window before reopening")
		prev.Close()
	}

	settingsOpenMu.Lock()
	settingsOpen = true
	settingsOpenMu.Unlock()

	// Reset service with a fresh copy of the live config.
	svc.Reset(cfg, configPath, onSaved)

	app := application.Get()
	if app == nil {
		guiLog("[GUI] ERROR: no running Wails app for settings window")
		settingsOpenMu.Lock()
		settingsOpen = false
		settingsOpenMu.Unlock()
		return
	}
	svc.app = app

	guiLog("[GUI] Creating settings window on running tray app...")
	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "GhostSpell Settings",
		Width:     760,
		Height:    660,
		URL:       "/dist/react.html?window=settings",
		Frameless: runtime.GOOS == "windows",
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:               application.MacBackdropTranslucent,
		},
	})
	svc.window = win

	// Bring window to front — on macOS and Windows the new window can appear
	// behind other windows if the app is a background-only (accessory) process (#139).
	win.Focus()

	// Reset the guard when the window closes so it can be reopened later.
	win.OnWindowEvent(events.Common.WindowClosing, func(event *application.WindowEvent) {
		guiLog("[GUI] Settings window closing event received")
		settingsOpenMu.Lock()
		settingsOpen = false
		svc.window = nil
		settingsOpenMu.Unlock()
	})

	guiLog("[GUI] Settings window created on running app")
}

// updateGuard prevents multiple update windows.
var (
	updateOpen   bool
	updateOpenMu sync.Mutex
)

// ShowUpdateWindow opens the update popup window on the running tray Wails app.
func ShowUpdateWindow(svc *SettingsService, cfg *config.Config, configPath string) {
	updateOpenMu.Lock()
	if updateOpen {
		updateOpenMu.Unlock()
		return
	}
	updateOpen = true
	updateOpenMu.Unlock()

	svc.Reset(cfg, configPath, nil)

	app := application.Get()
	if app == nil {
		updateOpenMu.Lock()
		updateOpen = false
		updateOpenMu.Unlock()
		return
	}
	svc.app = app

	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "GhostSpell Update",
		Width:  400,
		Height: 420,
		URL:    "/update.html",
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:               application.MacBackdropTranslucent,
		},
	})

	win.OnWindowEvent(events.Common.WindowClosing, func(event *application.WindowEvent) {
		updateOpenMu.Lock()
		updateOpen = false
		updateOpenMu.Unlock()
	})
}

// showStandaloneWindow creates a standalone Wails app with a settings window
// and blocks until closed. Used for first-launch setup before the tray exists.
func showStandaloneWindow(cfg *config.Config, configPath string, onSaved func()) *config.Config {
	guiLog("[GUI] showStandaloneWindow entered")

	// Work on a copy so cancelled edits don't corrupt the live config.
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

	svc := &SettingsService{
		cfgCopy:    &cfgCopy,
		configPath: configPath,
		onSaved:    onSaved,
		standalone: true,
	}

	subFS, err := fs.Sub(frontendFS, "frontend")
	if err != nil {
		guiLog("[GUI] ERROR: fs.Sub failed: %v", err)
		return nil
	}

	guiLog("[GUI] Creating standalone Wails app for first-launch settings...")
	app := application.New(application.Options{
		Name: "GhostSpell Settings",
		Icon: windowAppIcon(),
		Services: []application.Service{
			application.NewService(svc),
		},
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(subFS),
		},
	})
	svc.app = app

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "GhostSpell Setup",
		Width:  600,
		Height: 640,
		URL:    "/wizard.html",
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:               application.MacBackdropTranslucent,
		},
	})

	guiLog("[GUI] Standalone Wails app created, calling Run (blocks until window closes)...")
	if err := app.Run(); err != nil {
		guiLog("[GUI] Wails app.Run error: %v", err)
	} else {
		guiLog("[GUI] Wails app.Run completed without error")
	}

	// Reset the Wails singleton so the tray can create a fresh App.
	guiLog("[GUI] Calling ResetGlobal() to clear Wails singleton...")
	application.ResetGlobal()
	guiLog("[GUI] ResetGlobal() done (globalApplication should be nil now: %v)", application.Get() == nil)

	guiLog("[GUI] Run returned, window closed")
	if svc.saved {
		return &cfgCopy
	}
	return nil
}
