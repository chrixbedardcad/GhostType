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
	indicatorMu  sync.Mutex
)

// No off-screen positioning needed — opacity:0 in CSS handles visibility.

func CreateIndicator(app *application.App) {
	indicatorMu.Lock()
	defer indicatorMu.Unlock()

	bgType := application.BackgroundTypeTransparent
	ignoreMouse := true
	if runtime.GOOS == "windows" {
		bgType = application.BackgroundTypeTranslucent
		ignoreMouse = false
	}

	indicatorWin = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:              "ghostspell-indicator",
		Title:             "",
		Width:             260,
		Height:            52,
		Frameless:         true,
		AlwaysOnTop:       true,
		BackgroundType:    bgType,
		BackgroundColour:  application.RGBA{Red: 0, Green: 0, Blue: 0, Alpha: 0},
		DisableResize:     true,
		Hidden:            false, // must stay visible for WebView2 to render
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
	// Window is visible (Hidden:false) but content is invisible (CSS opacity:0).
	// WebView2 renders the page, ExecJS works, but user sees nothing.
	slog.Info("[gui] Indicator overlay window created (off-screen)")
}

// indicatorPos stores the configured position. Set by the app at startup.
var indicatorPos = "center"

// SetIndicatorPosition sets the configured position for the indicator.
func SetIndicatorPosition(pos string) {
	indicatorMu.Lock()
	indicatorPos = pos
	indicatorMu.Unlock()
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
	indicatorMu.Lock()
	win := indicatorWin
	indicatorMu.Unlock()
	if win == nil {
		return
	}

	slog.Debug("[indicator] ShowIndicator called", "prompt", promptName, "icon", promptIcon, "model", modelLabel)

	// Check if indicator is hidden.
	indicatorMu.Lock()
	pos := indicatorPos
	indicatorMu.Unlock()
	if pos == "hidden" {
		return
	}

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
	// Navigate to bare URL (no params) → page renders with opacity:0 (invisible).
	win.SetURL("/indicator.html")
}

func PopIndicator(promptIcon, promptName string) {
	indicatorMu.Lock()
	win := indicatorWin
	indicatorMu.Unlock()
	if win == nil {
		return
	}

	slog.Debug("[indicator] PopIndicator called", "prompt", promptName, "icon", promptIcon)

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
			w.SetURL("/indicator.html")
		}
	}()
}
