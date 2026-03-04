package tray

import (
	"fmt"
	"log/slog"
	"runtime"

	"github.com/chrixbedardcad/GhostType/internal/version"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// ModelLabel describes a configured provider for the tray Models menu section.
type ModelLabel struct {
	Label     string // e.g. "claude"
	Provider  string // e.g. "Anthropic"
	Model     string // e.g. "claude-sonnet-4-6"
	IsDefault bool
}

// Config holds tray configuration and callbacks.
type Config struct {
	// IconPNG is the raw PNG bytes for the tray icon.
	IconPNG []byte

	// Callbacks — called on the tray thread.
	OnModeChange   func(modeName string) // "correct", "translate", "rewrite"
	OnTargetSelect func(idx int)
	OnTemplSelect  func(idx int)
	OnSoundToggle  func(enabled bool)
	OnCancel       func()
	OnSettings     func()
	OnModelSelect  func(label string)
	OnExit         func()

	// State readers — called to build the menu.
	GetActiveMode   func() string // returns "correct", "translate", or "rewrite"
	GetTargetIdx    func() int
	GetTemplateIdx  func() int
	GetSoundEnabled func() bool
	GetIsProcessing func() bool
	GetModelLabels  func() []ModelLabel

	// Static data for building menu items.
	TargetLabels  []string // translate target display labels
	TemplateNames []string // rewrite template display names
}

// trayState holds the runtime state of the system tray.
type trayState struct {
	cfg     Config
	app     *application.App
	systray *application.SystemTray
}

// Start launches the system tray icon in a background goroutine.
// The provided app must be a fully-configured Wails application (with services
// and assets already registered). Start sets up the system tray and calls
// app.Run() in a goroutine. Returns a stop function that quits the app.
func Start(cfg Config, app *application.App) (stop func()) {
	slog.Info("[tray] Start() called",
		"os", runtime.GOOS,
		"icon_bytes", len(cfg.IconPNG),
		"target_labels", len(cfg.TargetLabels),
		"template_names", len(cfg.TemplateNames),
	)
	fmt.Printf("[tray] Start() called on %s, icon=%d bytes\n", runtime.GOOS, len(cfg.IconPNG))

	ts := &trayState{cfg: cfg, app: app}
	slog.Info("[tray] Using provided Wails application", "app_nil", ts.app == nil)
	fmt.Printf("[tray] Using provided Wails application (nil=%v)\n", ts.app == nil)

	slog.Info("[tray] Creating SystemTray...")
	fmt.Println("[tray] Creating SystemTray...")
	ts.systray = ts.app.SystemTray.New()
	slog.Info("[tray] SystemTray created", "systray_nil", ts.systray == nil)
	fmt.Printf("[tray] SystemTray created (nil=%v)\n", ts.systray == nil)

	if len(cfg.IconPNG) > 0 {
		ts.systray.SetIcon(cfg.IconPNG)
		slog.Info("[tray] Icon set", "bytes", len(cfg.IconPNG))
	} else {
		slog.Warn("[tray] No icon PNG provided!")
		fmt.Println("[tray] WARNING: No icon PNG provided!")
	}
	ts.systray.SetTooltip(fmt.Sprintf("GhostType v%s", version.Version))

	// Build and set the initial menu.
	slog.Info("[tray] Building initial menu...")
	fmt.Println("[tray] Building initial menu...")
	ts.refreshMenu()

	// On Windows/macOS, both left and right click should refresh state and
	// show the menu popup. Wails' default only opens the menu on right-click.
	// We must call OpenMenu() explicitly after refreshing — just calling
	// refreshMenu() (which rebuilds via SetMenu) is not enough.
	// On Linux, the DBus DE handles menu display natively; overriding
	// handlers emits LayoutUpdated signals that interfere with the DE.
	if runtime.GOOS != "linux" {
		ts.systray.OnClick(func() {
			ts.refreshMenu()
			ts.systray.OpenMenu()
		})
		ts.systray.OnRightClick(func() {
			ts.refreshMenu()
			ts.systray.OpenMenu()
		})
	}

	slog.Info("[tray] Starting Wails app.Run() in goroutine...")
	fmt.Println("[tray] Starting Wails app.Run() in goroutine...")
	go func() {
		runtime.LockOSThread()
		slog.Info("[tray] goroutine: calling app.Run()")
		fmt.Println("[tray] goroutine: calling app.Run()")
		if err := ts.app.Run(); err != nil {
			slog.Error("[tray] Wails app.Run() FAILED", "error", err)
			fmt.Printf("[tray] ERROR: Wails app.Run() failed: %v\n", err)
		} else {
			slog.Info("[tray] Wails app.Run() returned successfully")
			fmt.Println("[tray] Wails app.Run() returned successfully")
		}
	}()

	slog.Info("[tray] Start() complete — tray is running")
	fmt.Println("[tray] Start() complete — tray is running")

	return func() {
		slog.Info("[tray] Stop function called — quitting app")
		fmt.Println("[tray] Stop function called — quitting app")
		ts.app.Quit()
	}
}

// refreshMenu rebuilds the tray context menu from current state.
func (ts *trayState) refreshMenu() {
	menu := application.NewMenu()

	// Version header (disabled).
	menu.Add(fmt.Sprintf("GhostType v%s", version.Version)).SetEnabled(false)
	menu.AddSeparator()

	// Mode selection (radio group).
	activeMode := "correct"
	if ts.cfg.GetActiveMode != nil {
		activeMode = ts.cfg.GetActiveMode()
	}

	correctItem := menu.AddRadio("Correct", activeMode == "correct")
	translateItem := menu.AddRadio("Translate", activeMode == "translate")
	rewriteItem := menu.AddRadio("Rewrite", activeMode == "rewrite")

	correctItem.OnClick(func(ctx *application.Context) {
		ts.uncheckModes(correctItem, translateItem, rewriteItem)
		correctItem.SetChecked(true)
		if ts.cfg.OnModeChange != nil {
			ts.cfg.OnModeChange("correct")
		}
	})
	translateItem.OnClick(func(ctx *application.Context) {
		ts.uncheckModes(correctItem, translateItem, rewriteItem)
		translateItem.SetChecked(true)
		if ts.cfg.OnModeChange != nil {
			ts.cfg.OnModeChange("translate")
		}
	})
	rewriteItem.OnClick(func(ctx *application.Context) {
		ts.uncheckModes(correctItem, translateItem, rewriteItem)
		rewriteItem.SetChecked(true)
		if ts.cfg.OnModeChange != nil {
			ts.cfg.OnModeChange("rewrite")
		}
	})

	// Models section.
	menu.AddSeparator()
	menu.Add("Models:").SetEnabled(false)

	modelCount := 0
	if ts.cfg.GetModelLabels != nil {
		models := ts.cfg.GetModelLabels()
		modelCount = len(models)
		if len(models) > 0 {
			for _, ml := range models {
				displayName := ml.Label
				if displayName == "" {
					displayName = ml.Model
				}
				item := menu.AddRadio("  "+displayName, ml.IsDefault)
				label := ml.Label // capture for closure
				item.OnClick(func(ctx *application.Context) {
					if ts.cfg.OnModelSelect != nil {
						ts.cfg.OnModelSelect(label)
					}
					ts.refreshMenu()
				})
			}
		} else {
			menu.Add("  Add a model in Settings...").SetEnabled(false)
		}
	}

	settingsItem := menu.Add("  Settings...")
	settingsItem.OnClick(func(ctx *application.Context) {
		if ts.cfg.OnSettings != nil {
			ts.cfg.OnSettings()
		}
	})

	// Language targets.
	if len(ts.cfg.TargetLabels) > 0 {
		menu.AddSeparator()
		menu.Add("Language:").SetEnabled(false)

		targetIdx := 0
		if ts.cfg.GetTargetIdx != nil {
			targetIdx = ts.cfg.GetTargetIdx()
		}

		for i, name := range ts.cfg.TargetLabels {
			item := menu.AddRadio("  "+name, i == targetIdx)
			idx := i // capture for closure
			item.OnClick(func(ctx *application.Context) {
				if ts.cfg.OnTargetSelect != nil {
					ts.cfg.OnTargetSelect(idx)
				}
				ts.refreshMenu()
			})
		}
	}

	// Rewrite templates.
	if len(ts.cfg.TemplateNames) > 0 {
		menu.AddSeparator()
		menu.Add("Template:").SetEnabled(false)

		templIdx := 0
		if ts.cfg.GetTemplateIdx != nil {
			templIdx = ts.cfg.GetTemplateIdx()
		}

		for i, name := range ts.cfg.TemplateNames {
			item := menu.AddRadio("  "+name, i == templIdx)
			idx := i // capture for closure
			item.OnClick(func(ctx *application.Context) {
				if ts.cfg.OnTemplSelect != nil {
					ts.cfg.OnTemplSelect(idx)
				}
				ts.refreshMenu()
			})
		}
	}

	// Sound toggle.
	menu.AddSeparator()
	soundEnabled := false
	if ts.cfg.GetSoundEnabled != nil {
		soundEnabled = ts.cfg.GetSoundEnabled()
	}
	soundItem := menu.AddCheckbox("Sound", soundEnabled)
	soundItem.OnClick(func(ctx *application.Context) {
		if ts.cfg.OnSoundToggle != nil {
			ts.cfg.OnSoundToggle(ctx.IsChecked())
		}
	})

	// Cancel LLM.
	isProcessing := false
	if ts.cfg.GetIsProcessing != nil {
		isProcessing = ts.cfg.GetIsProcessing()
	}
	cancelItem := menu.Add("Cancel LLM")
	cancelItem.SetEnabled(isProcessing)
	cancelItem.OnClick(func(ctx *application.Context) {
		if ts.cfg.OnCancel != nil {
			ts.cfg.OnCancel()
		}
	})

	// Exit.
	menu.AddSeparator()
	exitItem := menu.Add("Exit")
	exitItem.OnClick(func(ctx *application.Context) {
		if ts.cfg.OnExit != nil {
			ts.cfg.OnExit()
		}
	})

	ts.systray.SetMenu(menu)

	// Count items for debug.
	itemCount := 3 + modelCount + 3 // modes + models + sound/cancel/exit + separators
	slog.Info("[tray] Menu built and set",
		"active_mode", activeMode,
		"models", modelCount,
		"targets", len(ts.cfg.TargetLabels),
		"templates", len(ts.cfg.TemplateNames),
		"approx_items", itemCount,
	)
}

// uncheckModes unchecks all mode radio items.
func (ts *trayState) uncheckModes(items ...*application.MenuItem) {
	for _, item := range items {
		item.SetChecked(false)
	}
}
