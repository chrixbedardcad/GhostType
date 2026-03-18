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
// The actual window is NOT created until ShowIndicator is first called.
// This prevents the AlwaysOnTop + IgnoreMouseEvents=false window from
// blocking clicks during the wizard, OAuth flows, and first-launch setup.
func CreateIndicator(app *application.App) {
	indicatorMu.Lock()
	defer indicatorMu.Unlock()
	indicatorApp = app
	slog.Info("[gui] Indicator lazy-init registered (window created on first use)")
}

// ensureIndicatorWindow lazily creates the indicator window on first use.
// Must be called with indicatorMu held.
func ensureIndicatorWindow() {
	if indicatorWin != nil || indicatorApp == nil {
		return
	}

	bgType := application.BackgroundTypeTransparent
	ignoreMouse := true
	if runtime.GOOS == "windows" {
		bgType = application.BackgroundTypeTranslucent
		ignoreMouse = false
	}

	indicatorWin = indicatorApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:              "ghostspell-indicator",
		Title:             "",
		X:                 -9999,
		Y:                 -9999,
		Width:             1,
		Height:            1,
		Frameless:         true,
		AlwaysOnTop:       true,
		BackgroundType:    bgType,
		BackgroundColour:  application.RGBA{Red: 0, Green: 0, Blue: 0, Alpha: 0},
		DisableResize:     true,
		Hidden:            false,
		IgnoreMouseEvents: ignoreMouse,
		URL:               "/indicator.html",
		Windows: application.WindowsWindow{
			HiddenOnTaskbar:                   true,
			DisableFramelessWindowDecorations: true,
		},
		Mac: application.MacWindow{
			TitleBar:    application.MacTitleBar{Hide: true},
			Backdrop:    application.MacBackdropTransparent,
			WindowLevel: application.MacWindowLevelFloating,
		},
	})
	slog.Info("[gui] Indicator window created (lazy, first use)")
}

// indicatorPos stores the configured position. Set by the app at startup.
var indicatorPos = "top-right"

// SetIndicatorPosition sets the configured position for the indicator.
func SetIndicatorPosition(pos string) {
	indicatorMu.Lock()
	indicatorPos = pos
	indicatorMu.Unlock()
}

// PreviewIndicatorPosition briefly shows the indicator at the current position
// so the user can see where it will appear. Auto-hides after 2 seconds.
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
	u := "/indicator.html?i=%E2%9C%8F%EF%B8%8F&n=Preview&pop=1"
	win.SetURL(u)
	time.Sleep(150 * time.Millisecond)

	x, y := getIndicatorPosition()
	win.SetPosition(x, y)

	go func() {
		time.Sleep(2 * time.Second)
		indicatorMu.Lock()
		w := indicatorWin
		indicatorMu.Unlock()
		if w != nil {
			w.SetPosition(-9999, -9999)
			w.SetURL("/indicator.html")
			w.SetSize(1, 1)
		}
	}()
}

func getIndicatorPosition() (int, int) {
	app := application.Get()
	if app == nil {
		return 100, 100
	}
	screen := app.Screen.GetPrimary()
	if screen == nil {
		return 100, 100
	}

	w := 260
	h := 52
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

func ShowIndicator(promptIcon, promptName, modelLabel string) {
	slog.Debug("[indicator] ShowIndicator called", "prompt", promptName, "icon", promptIcon, "model", modelLabel)

	indicatorMu.Lock()
	pos := indicatorPos
	if pos == "hidden" {
		indicatorMu.Unlock()
		return
	}
	// Lazy-create the window on first actual use.
	ensureIndicatorWindow()
	win := indicatorWin
	indicatorMu.Unlock()
	if win == nil {
		return
	}

	// Restore full size (was shrunk to 1x1 in HideIndicator to avoid blocking clicks).
	win.SetSize(260, 52)

	u := "/indicator.html?i=" + url.QueryEscape(promptIcon) + "&n=" + url.QueryEscape(promptName) + "&m=" + url.QueryEscape(modelLabel)
	win.SetURL(u)
	time.Sleep(150 * time.Millisecond) // let page load

	// Move on-screen.
	x, y := getIndicatorPosition()
	win.SetPosition(x, y)
}

func HideIndicator() {
	indicatorMu.Lock()
	win := indicatorWin
	indicatorMu.Unlock()
	if win == nil {
		return
	}

	slog.Debug("[indicator] HideIndicator called")
	// Move off-screen FIRST to stop blocking clicks immediately,
	// then shrink and clear content. Moving off-screen is the only
	// reliable way to prevent click interception on Windows where
	// IgnoreMouseEvents=false and AlwaysOnTop=true.
	win.SetPosition(-9999, -9999)
	win.SetURL("/indicator.html")
	win.SetSize(1, 1)
}

func PopIndicator(promptIcon, promptName string) {
	indicatorMu.Lock()
	win := indicatorWin
	indicatorMu.Unlock()
	if win == nil {
		return
	}

	slog.Debug("[indicator] PopIndicator called", "prompt", promptName, "icon", promptIcon)

	win.SetSize(260, 52)

	u := "/indicator.html?i=" + url.QueryEscape(promptIcon) + "&n=" + url.QueryEscape(promptName) + "&pop=1"
	win.SetURL(u)
	time.Sleep(150 * time.Millisecond)

	x, y := getIndicatorPosition()
	win.SetPosition(x, y)

	go func() {
		time.Sleep(1500 * time.Millisecond)
		indicatorMu.Lock()
		w := indicatorWin
		indicatorMu.Unlock()
		if w != nil {
			w.SetPosition(-9999, -9999)
			w.SetURL("/indicator.html")
			w.SetSize(1, 1)
		}
	}()
}
