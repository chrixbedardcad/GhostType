package gui

import (
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

var (
	indicatorWin  *application.WebviewWindow
	indicatorApp  *application.App
	indicatorMu   sync.Mutex
	indicatorReady bool // true once the React page has loaded
)

// CreateIndicator stores the app reference for lazy window creation.
func CreateIndicator(app *application.App) {
	indicatorMu.Lock()
	defer indicatorMu.Unlock()
	indicatorApp = app
	slog.Info("[gui] Indicator lazy-init registered (window created on first use)")
}

// ensureIndicatorWindow lazily creates the indicator window on first use.
// Loads the React indicator page ONCE. All subsequent state changes come
// via Wails events — no more SetURL page reloads.
func ensureIndicatorWindow() {
	if indicatorWin != nil || indicatorApp == nil {
		return
	}

	bgType := application.BackgroundTypeTransparent
	// On Windows, transparent/translucent backgrounds make the WebView2 window
	// fully click-through (DWM composition with WS_EX_NOREDIRECTIONBITMAP).
	// Use a solid background matching the indicator's dark theme instead.
	// The window is frameless + rounded via CSS, so the small rectangular
	// corners are barely visible.
	bgColour := application.RGBA{Red: 0, Green: 0, Blue: 0, Alpha: 0}
	ignoreMouse := false // must receive clicks for drag + context menu
	if runtime.GOOS == "windows" {
		bgType = application.BackgroundTypeSolid
		bgColour = application.RGBA{Red: 30, Green: 30, Blue: 46, Alpha: 255}
	}

	indicatorWin = indicatorApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:              "ghostspell-indicator",
		Title:             "",
		X:                 -9999,
		Y:                 -9999,
		Width:             48,
		Height:            48,
		Frameless:         true,
		AlwaysOnTop:       true,
		BackgroundType:    bgType,
		BackgroundColour:  bgColour,
		DisableResize:     true,
		Hidden:            false,
		IgnoreMouseEvents: ignoreMouse,
		URL:               "/indicator-react.html?window=indicator",
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

	fmt.Println("[indicator] Window created (React hybrid) URL=/dist/react.html?window=indicator")
	slog.Info("[gui] Indicator window created (React hybrid)", "url", "/dist/react.html?window=indicator")

	// Block until React has time to mount and register event listeners.
	fmt.Println("[indicator] Waiting 800ms for React to mount...")
	time.Sleep(800 * time.Millisecond)
	indicatorReady = true
	fmt.Println("[indicator] React page assumed ready")
}

// indicatorPos stores the configured position.
var indicatorPos = "top-right"

// indicatorMode stores the configured mode: "processing" (default), "always", "hidden".
var indicatorMode = "processing"

// indicatorSavedX/Y stores the user's dragged position.
var indicatorSavedX, indicatorSavedY int

func SetIndicatorPosition(pos string) {
	indicatorMu.Lock()
	indicatorPos = pos
	indicatorMu.Unlock()
}

func SetIndicatorMode(mode string) {
	indicatorMu.Lock()
	indicatorMode = mode
	indicatorMu.Unlock()
}

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

// getIndicatorPositionForSize calculates position based on configured corner.
func getIndicatorPositionForSize(w, h int) (int, int) {
	// Use saved drag position if available.
	indicatorMu.Lock()
	sx, sy := indicatorSavedX, indicatorSavedY
	indicatorMu.Unlock()
	if sx > 0 || sy > 0 {
		return sx, sy
	}

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
	default: // "center"
		return screen.WorkArea.X + (screen.WorkArea.Width-w)/2, screen.WorkArea.Y + screen.WorkArea.Height/3
	}
}

func getIndicatorPosition() (int, int) { return getIndicatorPositionForSize(260, 52) }
func getIdlePosition() (int, int)      { return getIndicatorPositionForSize(48, 48) }

// PreviewIndicatorPosition shows the indicator briefly.
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

	win.SetSize(260, 52)
	x, y := getIndicatorPosition()
	win.SetPosition(x, y)
	emitIndicatorEvent(map[string]any{"state": "pop", "icon": "✏️", "name": "Preview"})

	go func() {
		time.Sleep(2 * time.Second)
		HideIndicator()
	}()
}

// ShowIdle displays the indicator in idle mode (small ghost circle).
func ShowIdle() {
	indicatorMu.Lock()
	mode := indicatorMode
	if mode != "always" {
		slog.Debug("[indicator] ShowIdle: skipped (mode is not always)", "mode", mode)
		indicatorMu.Unlock()
		return
	}
	ensureIndicatorWindow()
	win := indicatorWin
	indicatorMu.Unlock()
	if win == nil {
		slog.Warn("[indicator] ShowIdle: window is nil")
		return
	}

	win.SetSize(48, 48)
	x, y := getIdlePosition()
	slog.Info("[indicator] ShowIdle", "size", "48x48", "x", x, "y", y)
	fmt.Printf("[indicator] ShowIdle: size=48x48 pos=%d,%d\n", x, y)
	win.SetPosition(x, y)
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
	win := indicatorWin
	indicatorMu.Unlock()
	if win == nil {
		return
	}

	win.SetSize(260, 52)
	x, y := getIndicatorPosition()
	slog.Info("[indicator] ShowIndicator: positioning", "size", "260x52", "x", x, "y", y, "prompt", promptName, "model", modelLabel)
	win.SetPosition(x, y)
	emitIndicatorEvent(map[string]any{
		"state": "processing", "icon": promptIcon, "name": promptName, "model": modelLabel,
	})
}

// HideIndicator hides the indicator or returns to idle in "always" mode.
func HideIndicator() {
	indicatorMu.Lock()
	win := indicatorWin
	mode := indicatorMode
	indicatorMu.Unlock()
	if win == nil {
		return
	}

	slog.Debug("[indicator] HideIndicator called", "mode", mode)

	if mode == "always" {
		win.SetSize(48, 48)
		x, y := getIdlePosition()
		slog.Debug("[indicator] HideIndicator: returning to idle", "x", x, "y", y)
		win.SetPosition(x, y)
		emitIndicatorEvent(map[string]any{"state": "idle"})
		return
	}

	// Move off-screen. Keep 48x48 so WebView2 stays renderable.
	slog.Debug("[indicator] HideIndicator: moving off-screen")
	win.SetPosition(-9999, -9999)
	win.SetSize(48, 48)
	emitIndicatorEvent(map[string]any{"state": "hidden"})
}

// PopIndicator shows prompt name briefly, then auto-hides.
func PopIndicator(promptIcon, promptName string) {
	indicatorMu.Lock()
	ensureIndicatorWindow()
	win := indicatorWin
	indicatorMu.Unlock()
	if win == nil {
		return
	}

	slog.Info("[indicator] PopIndicator called", "prompt", promptName, "icon", promptIcon)

	win.SetSize(260, 52)
	x, y := getIndicatorPosition()
	slog.Info("[indicator] PopIndicator: positioning", "size", "260x52", "x", x, "y", y)
	win.SetPosition(x, y)
	emitIndicatorEvent(map[string]any{"state": "pop", "icon": promptIcon, "name": promptName})

	go func() {
		time.Sleep(2500 * time.Millisecond)
		HideIndicator()
	}()
}

// SaveIndicatorPosition saves the drag position (called from React JS).
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
