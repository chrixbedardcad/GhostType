package gui

import (
	"log/slog"
	"runtime"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

var (
	indicatorWin *application.WebviewWindow
	indicatorApp *application.App
	indicatorMu  sync.Mutex
)

// CreateIndicator stores the app reference for lazy window creation.
func CreateIndicator(app *application.App) {
	indicatorMu.Lock()
	defer indicatorMu.Unlock()
	indicatorApp = app
	slog.Info("[gui] Indicator lazy-init registered (window created on first use)")
}

// ensureIndicatorWindow lazily creates the fixed-size indicator window.
// The window is 320x400, transparent, always on top, and never resizes.
// All visual changes happen via Wails events → React CSS transitions.
func ensureIndicatorWindow() {
	if indicatorWin != nil || indicatorApp == nil {
		return
	}

	bgType := application.BackgroundTypeTransparent
	ignoreMouse := false // must receive clicks for drag + context menu
	if runtime.GOOS == "windows" {
		bgType = application.BackgroundTypeTranslucent
	}

	// Calculate initial position.
	x, y := getDefaultIndicatorPos()

	// Use saved position if available.
	indicatorMu.Unlock() // temporarily unlock to avoid deadlock with config access
	indicatorMu.Lock()
	if indicatorSavedX > 0 || indicatorSavedY > 0 {
		x, y = indicatorSavedX, indicatorSavedY
	}

	indicatorWin = indicatorApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:              "ghostspell-indicator",
		Title:             "",
		X:                 x,
		Y:                 y,
		Width:             320,
		Height:            400,
		Frameless:         true,
		AlwaysOnTop:       true,
		BackgroundType:    bgType,
		BackgroundColour:  application.RGBA{Red: 0, Green: 0, Blue: 0, Alpha: 0},
		DisableResize:     true,
		Hidden:            false,
		IgnoreMouseEvents: ignoreMouse,
		URL:               "/dist/react.html?window=indicator",
		Windows: application.WindowsWindow{
			HiddenOnTaskbar:                  true,
			DisableFramelessWindowDecorations: true,
		},
		Mac: application.MacWindow{
			TitleBar:    application.MacTitleBar{Hide: true},
			Backdrop:    application.MacBackdropTransparent,
			WindowLevel: application.MacWindowLevelFloating,
		},
	})
	slog.Info("[gui] Indicator window created (fixed 320x400)", "x", x, "y", y)
}

// indicatorPos stores the configured position. Set by the app at startup.
var indicatorPos = "top-right"

// indicatorMode stores the configured mode: "processing" (default), "always", "hidden".
var indicatorMode = "processing"

// indicatorSavedX/Y stores the user's dragged position.
var indicatorSavedX, indicatorSavedY int

// SetIndicatorPosition sets the configured position for the indicator.
func SetIndicatorPosition(pos string) {
	indicatorMu.Lock()
	indicatorPos = pos
	indicatorMu.Unlock()
}

// SetIndicatorMode sets the indicator mode (#211).
func SetIndicatorMode(mode string) {
	indicatorMu.Lock()
	indicatorMode = mode
	indicatorMu.Unlock()
}

// SetIndicatorSavedPosition sets the saved drag position.
func SetIndicatorSavedPosition(x, y int) {
	indicatorMu.Lock()
	indicatorSavedX = x
	indicatorSavedY = y
	indicatorMu.Unlock()
}

// emitIndicatorEvent sends a state update to the React indicator.
func emitIndicatorEvent(data map[string]any) {
	app := application.Get()
	if app != nil {
		app.Event.Emit("indicatorState", data)
	}
}

// getDefaultIndicatorPos calculates the default ghost position based on config.
func getDefaultIndicatorPos() (int, int) {
	app := application.Get()
	if app == nil {
		return 100, 100
	}
	screen := app.Screen.GetPrimary()
	if screen == nil {
		return 100, 100
	}

	pos := indicatorPos
	// Ghost appears at window origin; use 48px as ghost size for edge offset.
	switch pos {
	case "top-left":
		return screen.WorkArea.X + 20, screen.WorkArea.Y + 20
	case "top-right":
		return screen.WorkArea.X + screen.WorkArea.Width - 68, screen.WorkArea.Y + 20
	case "bottom-left":
		return screen.WorkArea.X + 20, screen.WorkArea.Y + screen.WorkArea.Height - 68
	case "bottom-right":
		return screen.WorkArea.X + screen.WorkArea.Width - 68, screen.WorkArea.Y + screen.WorkArea.Height - 68
	default: // "center"
		return screen.WorkArea.X + (screen.WorkArea.Width-48)/2, screen.WorkArea.Y + screen.WorkArea.Height/3
	}
}

// PreviewIndicatorPosition shows the indicator briefly at the configured position.
func PreviewIndicatorPosition() {
	indicatorMu.Lock()
	pos := indicatorPos
	if pos == "hidden" {
		indicatorMu.Unlock()
		return
	}
	ensureIndicatorWindow()
	win := indicatorWin
	indicatorMu.Unlock()
	if win == nil {
		return
	}

	// Move to default position (ignoring saved drag position for preview).
	x, y := getDefaultIndicatorPos()
	win.SetPosition(x, y)

	// Show as pop
	emitIndicatorEvent(map[string]any{
		"state": "pop",
		"icon":  "✏️",
		"name":  "Preview",
	})

	go func() {
		time.Sleep(2 * time.Second)
		emitIndicatorEvent(map[string]any{"state": "hidden"})
	}()
}

// ShowIdle displays the indicator in idle mode.
func ShowIdle() {
	indicatorMu.Lock()
	mode := indicatorMode
	if mode != "always" {
		indicatorMu.Unlock()
		return
	}
	ensureIndicatorWindow()
	indicatorMu.Unlock()

	slog.Debug("[indicator] ShowIdle: displaying idle ghost")
	emitIndicatorEvent(map[string]any{"state": "idle"})
}

// ShowIndicator shows the processing state with prompt info.
func ShowIndicator(promptIcon, promptName, modelLabel string) {
	slog.Debug("[indicator] ShowIndicator called", "prompt", promptName, "icon", promptIcon, "model", modelLabel)

	indicatorMu.Lock()
	pos := indicatorPos
	if pos == "hidden" {
		indicatorMu.Unlock()
		return
	}
	ensureIndicatorWindow()
	indicatorMu.Unlock()

	emitIndicatorEvent(map[string]any{
		"state": "processing",
		"icon":  promptIcon,
		"name":  promptName,
		"model": modelLabel,
	})
}

// HideIndicator hides the indicator or returns to idle in "always" mode.
func HideIndicator() {
	indicatorMu.Lock()
	mode := indicatorMode
	indicatorMu.Unlock()

	slog.Debug("[indicator] HideIndicator called", "mode", mode)

	if mode == "always" {
		emitIndicatorEvent(map[string]any{"state": "idle"})
		return
	}

	emitIndicatorEvent(map[string]any{"state": "hidden"})
}

// PopIndicator shows prompt name briefly, then auto-hides/returns to idle.
func PopIndicator(promptIcon, promptName string) {
	slog.Debug("[indicator] PopIndicator called", "prompt", promptName, "icon", promptIcon)

	indicatorMu.Lock()
	ensureIndicatorWindow()
	indicatorMu.Unlock()

	emitIndicatorEvent(map[string]any{
		"state": "pop",
		"icon":  promptIcon,
		"name":  promptName,
	})

	go func() {
		time.Sleep(2500 * time.Millisecond)
		HideIndicator()
	}()
}

// SaveIndicatorPosition saves the drag position for the indicator.
func (s *SettingsService) SaveIndicatorPosition(x, y int) string {
	slog.Debug("[GUI] SaveIndicatorPosition", "x", x, "y", y)
	SetIndicatorSavedPosition(x, y)
	if s.cfgCopy != nil {
		s.cfgCopy.IndicatorX = x
		s.cfgCopy.IndicatorY = y
		s.validateAndSave()
	}
	return "ok"
}
