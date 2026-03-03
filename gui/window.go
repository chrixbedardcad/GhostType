package gui

import (
	"io/fs"
	"sync"

	"github.com/chrixbedardcad/GhostType/config"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// settingsGuard prevents multiple settings windows.
var (
	settingsOpen   bool
	settingsOpenMu sync.Mutex
)

// ShowSettingsBlocking opens the settings window and blocks until it closes.
// Returns the (potentially updated) config. Used by main.go on first launch.
func ShowSettingsBlocking(cfg *config.Config, configPath string) *config.Config {
	guiLog("[GUI] ShowSettingsBlocking called, configPath=%s", configPath)
	updated := showWindow(cfg, configPath, nil)
	if updated != nil {
		return updated
	}
	return cfg
}

// ShowSettings opens the settings window. Non-blocking (for tray).
// onSaved is called after each save so the caller can reload the live config.
func ShowSettings(cfg *config.Config, configPath string, onSaved func()) {
	guiLog("[GUI] ShowSettings (async) called")
	settingsOpenMu.Lock()
	if settingsOpen {
		settingsOpenMu.Unlock()
		guiLog("[GUI] ShowSettings: window already open, skipping")
		return
	}
	settingsOpen = true
	settingsOpenMu.Unlock()
	guiLog("[GUI] ShowSettings: launching goroutine")

	go func() {
		defer func() {
			if r := recover(); r != nil {
				guiLog("[GUI] PANIC in ShowSettings goroutine: %v", r)
			}
			settingsOpenMu.Lock()
			settingsOpen = false
			settingsOpenMu.Unlock()
			guiLog("[GUI] ShowSettings goroutine exited")
		}()
		showWindow(cfg, configPath, onSaved)
	}()
}

// showWindow creates a Wails app with a settings window and blocks until closed.
// Returns the updated config if any saves occurred, or nil if no changes were made.
func showWindow(cfg *config.Config, configPath string, onSaved func()) *config.Config {
	guiLog("[GUI] showWindow entered")

	// Work on a copy so cancelled edits don't corrupt the live config.
	cfgCopy := *cfg
	if cfg.LLMProviders != nil {
		cfgCopy.LLMProviders = make(map[string]config.LLMProviderDef, len(cfg.LLMProviders))
		for k, v := range cfg.LLMProviders {
			cfgCopy.LLMProviders[k] = v
		}
	}

	// Create the settings service with all state.
	svc := &SettingsService{
		cfgCopy:    &cfgCopy,
		configPath: configPath,
		onSaved:    onSaved,
	}

	// Sub-filesystem strips the "frontend/" prefix so URL can be "/index.html".
	subFS, err := fs.Sub(frontendFS, "frontend")
	if err != nil {
		guiLog("[GUI] ERROR: fs.Sub failed: %v", err)
		return nil
	}

	guiLog("[GUI] Creating Wails app...")
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

	// Create the settings window.
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "GhostType Settings",
		Width:  720,
		Height: 580,
		URL:    "/index.html",
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:               application.MacBackdropTranslucent,
		},
	})

	guiLog("[GUI] Wails app created, calling Run (blocks until window closes)...")

	// Run blocks until the app quits (window closed or CloseWindow called).
	if err := app.Run(); err != nil {
		guiLog("[GUI] Wails app.Run error: %v", err)
	}

	// Reset the Wails singleton so the tray (or a later settings window)
	// can create a fresh App. Without this, New() returns the stale app
	// and Run() fails with "application is running or a previous run has failed".
	application.ResetGlobal()

	guiLog("[GUI] Run returned, window closed")
	if svc.saved {
		return &cfgCopy
	}
	return nil
}
