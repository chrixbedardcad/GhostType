package gui

import (
	"log/slog"
	"runtime"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
)

var (
	indicatorWin *application.WebviewWindow
	indicatorMu  sync.Mutex
)

// CreateIndicator creates a small, hidden, frameless overlay window showing an
// animated ghost. Call this before app.Run(). Use ShowIndicator / HideIndicator
// to toggle visibility when processing starts/stops.
func CreateIndicator(app *application.App) {
	indicatorMu.Lock()
	defer indicatorMu.Unlock()

	// On Windows, BackgroundTypeTransparent + Frameless causes WS_EX_LAYERED
	// which is incompatible with WebView2 (window renders invisible).
	// Use BackgroundTypeTranslucent instead — it triggers WS_EX_NOREDIRECTIONBITMAP
	// which works with WebView2's DirectComposition renderer.
	// Similarly, IgnoreMouseEvents adds WS_EX_LAYERED on Windows, so skip it.
	bgType := application.BackgroundTypeTransparent
	ignoreMouse := true
	if runtime.GOOS == "windows" {
		bgType = application.BackgroundTypeTranslucent
		ignoreMouse = false
	}

	indicatorWin = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:              "ghostspell-indicator",
		Title:             "",
		Width:             148,
		Height:            52,
		Frameless:         true,
		AlwaysOnTop:       true,
		BackgroundType:    bgType,
		BackgroundColour:  application.RGBA{Red: 0, Green: 0, Blue: 0, Alpha: 0},
		DisableResize:     true,
		Hidden:            true,
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
	slog.Info("[gui] Indicator overlay window created (hidden)")
}

// ShowIndicator displays the floating ghost in the bottom-right corner of the
// primary screen (above the taskbar on Windows).
func ShowIndicator() {
	indicatorMu.Lock()
	win := indicatorWin
	indicatorMu.Unlock()
	if win == nil {
		return
	}

	// Position bottom-right of the primary screen's work area.
	app := application.Get()
	if app != nil {
		screen := app.Screen.GetPrimary()
		if screen != nil {
			x := screen.WorkArea.X + screen.WorkArea.Width - 164
			y := screen.WorkArea.Y + screen.WorkArea.Height - 68
			win.SetPosition(x, y)
		}
	}
	// Show window FIRST so the WebView is active, then reset timer.
	// On Windows, WebView2 silently drops ExecJS calls when the window
	// is hidden, so we must show before executing JS.
	win.Show()
	win.ExecJS("resetTimer()")
}

// HideIndicator hides the floating ghost overlay and stops the elapsed timer.
// ExecJS runs BEFORE Hide() so the WebView is still active and processes the
// JS. This ensures the DOM reads "0s" when the window is next shown.
func HideIndicator() {
	indicatorMu.Lock()
	win := indicatorWin
	indicatorMu.Unlock()
	if win == nil {
		return
	}
	win.ExecJS("stopTimer()")
	win.Hide()
}
