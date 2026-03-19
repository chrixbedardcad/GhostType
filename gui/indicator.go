package gui

import (
	"log/slog"
	"net/url"
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
// Uses React frontend (320x400 transparent) for smooth CSS transitions.
// All other windows (settings, wizard, etc.) remain on old HTML.
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
	slog.Info("[gui] Indicator window created (React, fixed 320x400)", "x", x, "y", y)
}

// indicatorPos stores the configured position.
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

// SetIndicatorMode sets the indicator mode.
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

// emitIndicatorEvent sends a state update to the React indicator via Wails events.
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

// --- Legacy compatibility for old HTML windows ---
// These functions are still called by settings (index.html) for preview.

func getIndicatorPositionForSize(w, h int) (int, int) {
	app := application.Get()
	if app == nil {
		return 100, 100
	}
	screen := app.Screen.GetPrimary()
	if screen == nil {
		return 100, 100
	}
	indicatorMu.Lock()
	pos := indicatorPos
	indicatorMu.Unlock()
	switch pos {
	case "top-left":
		return screen.WorkArea.X + 20, screen.WorkArea.Y + 20
	case "top-right":
		return screen.WorkArea.X + screen.WorkArea.Width - w - 20, screen.WorkArea.Y + 20
	case "bottom-left":
		return screen.WorkArea.X + 20, screen.WorkArea.Y + screen.WorkArea.Height - h - 20
	case "bottom-right":
		return screen.WorkArea.X + screen.WorkArea.Width - w - 20, screen.WorkArea.Y + screen.WorkArea.Height - h - 20
	default:
		return screen.WorkArea.X + (screen.WorkArea.Width-w)/2, screen.WorkArea.Y + screen.WorkArea.Height/3
	}
}

func getIndicatorPosition() (int, int) {
	return getIndicatorPositionForSize(260, 52)
}

func getIdlePosition() (int, int) {
	return getIndicatorPositionForSize(48, 48)
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

	// Move to default position for preview.
	x, y := getDefaultIndicatorPos()
	win.SetPosition(x, y)

	emitIndicatorEvent(map[string]any{
		"state": "pop",
		"icon":  "✏️",
		"name":  "Preview",
	})

	go func() {
		time.Sleep(2 * time.Second)
		HideIndicator()
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

// SaveIndicatorPosition saves the drag position for the indicator (called from React JS).
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

// --- Legacy wrappers for old indicator.html (no longer used but kept for
// ResizeIndicatorForMenu which settings JS still calls) ---

// showIndicatorOldHTML is the old SetURL-based approach. Not called anymore
// but the function signature exists for any stray references.
func showIndicatorURLBased(promptIcon, promptName, modelLabel string) {
	indicatorMu.Lock()
	win := indicatorWin
	indicatorMu.Unlock()
	if win == nil {
		return
	}
	win.SetSize(260, 52)
	u := "/indicator.html?i=" + url.QueryEscape(promptIcon) + "&n=" + url.QueryEscape(promptName) + "&m=" + url.QueryEscape(modelLabel)
	win.SetURL(u)
	time.Sleep(150 * time.Millisecond)
	x, y := getIndicatorPosition()
	win.SetPosition(x, y)
}
