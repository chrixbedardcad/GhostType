package tray

import (
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"time"

	"github.com/chrixbedardcad/GhostSpell/internal/version"
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

	// WorkingFrames holds the animation frames for the working indicator.
	// On macOS these should be template icons (white silhouettes).
	WorkingFrames [][]byte
	// WorkingFramesMacOS holds macOS template variants of the working frames.
	WorkingFramesMacOS [][]byte

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

	// OnUpdateClick is called when the "Update available" menu item is clicked.
	OnUpdateClick func()
}

// trayState holds the runtime state of the system tray.
type trayState struct {
	cfg     Config
	app     *application.App
	systray *application.SystemTray

	// Animation state.
	animMu   sync.Mutex
	animStop chan struct{}

	// Update state.
	updateMu      sync.Mutex
	updateVersion string // non-empty when an update is available
}

// Start configures the system tray icon and menu on the given Wails application.
// It returns a run function that starts the Cocoa/GTK/Win32 event loop (blocking),
// a stop function that quits the app, a dismissMenu function that cancels any
// currently tracking tray menu, startAnim/stopAnim for working animation, and
// a setUpdateAvailable function that sets the update version and rebuilds the menu.
func Start(cfg Config, app *application.App) (run func() error, stop func(), dismissMenu func() bool, startAnim func(), stopAnim func(), setUpdateAvailable func(version string)) {
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
	ts.systray.SetTooltip(fmt.Sprintf("GhostSpell v%s", version.Version))

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

	slog.Info("[tray] Start() setup complete — returning run/stop/anim")
	fmt.Println("[tray] Start() setup complete — returning run/stop/anim")

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

	startAnim = func() { ts.StartAnimation() }
	stopAnim = func() { ts.StopAnimation() }

	setUpdateAvailable = func(ver string) {
		ts.updateMu.Lock()
		ts.updateVersion = ver
		ts.updateMu.Unlock()
		ts.refreshMenu()
		slog.Info("[tray] Update available", "version", ver)
	}

	return run, stop, dismissMenu, startAnim, stopAnim, setUpdateAvailable
}

// StartAnimation begins cycling through working animation frames.
func (ts *trayState) StartAnimation() {
	ts.animMu.Lock()
	if ts.animStop != nil {
		ts.animMu.Unlock()
		return // already animating
	}
	stop := make(chan struct{})
	ts.animStop = stop
	ts.animMu.Unlock()

	// Select the right frames for the platform.
	var frames [][]byte
	if runtime.GOOS == "darwin" && len(ts.cfg.WorkingFramesMacOS) > 0 {
		frames = ts.cfg.WorkingFramesMacOS
	} else {
		frames = ts.cfg.WorkingFrames
	}
	if len(frames) == 0 {
		return
	}

	ts.systray.SetTooltip("GhostSpell — Processing...")

	go func() {
		// Linear cycle: 0 → 1 → 2 → 3 → 0 → 1 → ...
		// Frames are designed to loop seamlessly:
		//   up-faded → center-bright → down-faded → center-dim → repeat
		idx := 0
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				fi := idx % len(frames)
				if runtime.GOOS == "darwin" {
					ts.systray.SetTemplateIcon(frames[fi])
				} else {
					ts.systray.SetIcon(frames[fi])
				}
				idx++
			}
		}
	}()
}

// StopAnimation stops the icon animation and restores the default icon.
func (ts *trayState) StopAnimation() {
	ts.animMu.Lock()
	if ts.animStop != nil {
		close(ts.animStop)
		ts.animStop = nil
	}
	ts.animMu.Unlock()

	// Restore default icon.
	if runtime.GOOS == "darwin" && len(ts.cfg.TemplateIconPNG) > 0 {
		ts.systray.SetTemplateIcon(ts.cfg.TemplateIconPNG)
	} else if len(ts.cfg.IconPNG) > 0 {
		ts.systray.SetIcon(ts.cfg.IconPNG)
	}
	ts.systray.SetTooltip(fmt.Sprintf("GhostSpell v%s", version.Version))
}

// refreshMenu rebuilds the tray context menu from current state.
func (ts *trayState) refreshMenu() {
	menu := application.NewMenu()

	// Version header (disabled).
	menu.Add(fmt.Sprintf("GhostSpell v%s", version.Version)).SetEnabled(false)

	// Update available item.
	ts.updateMu.Lock()
	updateVer := ts.updateVersion
	ts.updateMu.Unlock()
	if updateVer != "" {
		updateItem := menu.Add(fmt.Sprintf("Update available → v%s", updateVer))
		updateItem.OnClick(func(ctx *application.Context) {
			if ts.cfg.OnUpdateClick != nil {
				ts.cfg.OnUpdateClick()
			}
		})
	}

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
