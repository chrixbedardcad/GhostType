package gui

import (
	"io/fs"
	"sync"

	"github.com/chrixbedardcad/GhostType/config"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

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

// ShowSettings opens the settings window on the already-running tray Wails app.
// Non-blocking — creates a window and returns immediately.
// svc must be the SettingsService that was pre-registered on the app before Run().
func ShowSettings(svc *SettingsService, cfg *config.Config, configPath string, onSaved func()) {
	guiLog("[GUI] ShowSettings (async) called")
	settingsOpenMu.Lock()
	if settingsOpen {
		settingsOpenMu.Unlock()
		guiLog("[GUI] ShowSettings: window already open, skipping")
		return
	}
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
		Title:  "GhostType Settings",
		Width:  720,
		Height: 580,
		URL:    "/index.html",
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:               application.MacBackdropTranslucent,
		},
	})
	svc.window = win

	// Reset the guard when the window closes so it can be reopened later.
	win.OnWindowEvent(events.Common.WindowClosing, func(event *application.WindowEvent) {
		guiLog("[GUI] Settings window closing event received")
		settingsOpenMu.Lock()
		settingsOpen = false
		settingsOpenMu.Unlock()
	})

	guiLog("[GUI] Settings window created on running app")
}

// showStandaloneWindow creates a standalone Wails app with a settings window
// and blocks until closed. Used for first-launch setup before the tray exists.
func showStandaloneWindow(cfg *config.Config, configPath string, onSaved func()) *config.Config {
	guiLog("[GUI] showStandaloneWindow entered")

	// Work on a copy so cancelled edits don't corrupt the live config.
	cfgCopy := *cfg
	if cfg.LLMProviders != nil {
		cfgCopy.LLMProviders = make(map[string]config.LLMProviderDef, len(cfg.LLMProviders))
		for k, v := range cfg.LLMProviders {
			cfgCopy.LLMProviders[k] = v
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
		Name: "GhostType Settings",
		Services: []application.Service{
			application.NewService(svc),
		},
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(subFS),
		},
	})
	svc.app = app

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "GhostType Setup",
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
