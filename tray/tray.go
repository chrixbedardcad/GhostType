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
	// TemplateIconPNG is a macOS template icon (used via SetTemplateIcon on darwin).
	// If set and running on macOS, this takes precedence over IconPNG.
	TemplateIconPNG []byte

	// Callbacks — called on the tray thread.
	OnPromptSelect func(idx int)
	OnModelSelect  func(label string)
	OnSettings     func()
	OnExit         func()

	// IsProcessing returns true when the hotkey handler is actively capturing
	// or processing text. Used on macOS to suppress menu opening, since the
	// NSMenu modal event loop blocks keyboard simulation to the target app.
	IsProcessing func() bool

	// State readers — called to build the menu.
	GetActivePrompt func() int
	GetPromptNames  func() []string
	GetModelLabels  func() []ModelLabel
}

// trayState holds the runtime state of the system tray.
type trayState struct {
	cfg     Config
	app     *application.App
	systray *application.SystemTray
}

// Start configures the system tray icon and menu on the given Wails application.
// It returns a run function that starts the Cocoa/GTK/Win32 event loop (blocking),
// a stop function that quits the app, and a dismissMenu function that cancels any
// currently tracking tray menu. The caller decides which goroutine calls run —
// this is critical on macOS where Cocoa must run on the main thread.
func Start(cfg Config, app *application.App) (run func() error, stop func(), dismissMenu func() bool) {
	slog.Info("[tray] Start() called",
		"os", runtime.GOOS,
		"icon_bytes", len(cfg.IconPNG),
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

	if runtime.GOOS == "darwin" && len(cfg.TemplateIconPNG) > 0 {
		ts.systray.SetTemplateIcon(cfg.TemplateIconPNG)
		slog.Info("[tray] macOS template icon set", "bytes", len(cfg.TemplateIconPNG))
	} else if len(cfg.IconPNG) > 0 {
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
			if ts.cfg.IsProcessing != nil && ts.cfg.IsProcessing() {
				return // Don't open menu while hotkey is processing
			}
			ts.refreshMenu()
			ts.systray.OpenMenu()
		})
		ts.systray.OnRightClick(func() {
			if ts.cfg.IsProcessing != nil && ts.cfg.IsProcessing() {
				return
			}
			ts.refreshMenu()
			ts.systray.OpenMenu()
		})
	}

	slog.Info("[tray] Start() setup complete — returning run/stop/setIcon")
	fmt.Println("[tray] Start() setup complete — returning run/stop/setIcon")

	run = func() error {
		slog.Info("[tray] run: calling app.Run()")
		fmt.Println("[tray] run: calling app.Run()")
		return ts.app.Run()
	}

	stop = func() {
		slog.Info("[tray] Stop function called — quitting app")
		fmt.Println("[tray] Stop function called — quitting app")
		ts.app.Quit()
	}

	dismissMenu = func() bool {
		return ts.systray.DismissMenu()
	}

	return run, stop, dismissMenu
}

// refreshMenu rebuilds the tray context menu from current state.
func (ts *trayState) refreshMenu() {
	menu := application.NewMenu()

	// Version header (disabled).
	menu.Add(fmt.Sprintf("GhostType v%s", version.Version)).SetEnabled(false)
	menu.AddSeparator()

	// Prompt selection (radio group).
	activePrompt := 0
	if ts.cfg.GetActivePrompt != nil {
		activePrompt = ts.cfg.GetActivePrompt()
	}

	var promptNames []string
	if ts.cfg.GetPromptNames != nil {
		promptNames = ts.cfg.GetPromptNames()
	}

	for i, name := range promptNames {
		item := menu.AddRadio(name, i == activePrompt)
		idx := i // capture for closure
		item.OnClick(func(ctx *application.Context) {
			if ts.cfg.OnPromptSelect != nil {
				ts.cfg.OnPromptSelect(idx)
			}
			ts.refreshMenu()
		})
	}

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
		}
	}

	// Settings and Exit — together, no separator between them.
	menu.AddSeparator()
	settingsItem := menu.Add("Settings")
	settingsItem.OnClick(func(ctx *application.Context) {
		if ts.cfg.OnSettings != nil {
			ts.cfg.OnSettings()
		}
	})

	exitItem := menu.Add("Exit")
	exitItem.OnClick(func(ctx *application.Context) {
		if ts.cfg.OnExit != nil {
			ts.cfg.OnExit()
		}
	})

	ts.systray.SetMenu(menu)

	slog.Info("[tray] Menu built and set",
		"active_prompt", activePrompt,
		"prompts", len(promptNames),
		"models", modelCount,
	)
}
