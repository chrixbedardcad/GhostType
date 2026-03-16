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

func getIndicatorPosition() (int, int) {
	app := application.Get()
	if app != nil {
		screen := app.Screen.GetPrimary()
		if screen != nil {
			// Center horizontally, position in the upper third vertically.
			x := screen.WorkArea.X + (screen.WorkArea.Width-260)/2
			y := screen.WorkArea.Y + screen.WorkArea.Height/3
			return x, y
		}
	}
	return 100, 100
}

func ShowIndicator(promptIcon, promptName string) {
	indicatorMu.Lock()
	win := indicatorWin
	indicatorMu.Unlock()
	if win == nil {
		return
	}

	slog.Debug("[indicator] ShowIndicator called", "prompt", promptName, "icon", promptIcon)

	// Navigate to the indicator page with prompt data in URL params.
	// The window is always "visible" (off-screen), so SetURL + render
	// works reliably on both Windows WebView2 and macOS WebKit.
	u := "/indicator.html?i=" + url.QueryEscape(promptIcon) + "&n=" + url.QueryEscape(promptName)
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
