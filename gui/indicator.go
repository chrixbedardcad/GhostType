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
		Width:             260,
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
// primary screen (above the taskbar on Windows). The prompt icon and name are
// displayed alongside the ghost and elapsed timer.
func ShowIndicator(promptIcon, promptName string) {
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
			x := screen.WorkArea.X + screen.WorkArea.Width - 276
			y := screen.WorkArea.Y + screen.WorkArea.Height - 68
			win.SetPosition(x, y)
		}
	}

	slog.Debug("[indicator] ShowIndicator called", "prompt", promptName, "icon", promptIcon)

	// Show the window FIRST, then set prompt data via ExecJS. On Windows
	// WebView2, ExecJS on a hidden window gets silently dropped.
	// We sleep 100ms after Show to let WebView2 fully transition to visible
	// before calling ExecJS. This is synchronous (not a goroutine) so that
	// process.go's RestoreForegroundWindow() runs AFTER the ExecJS lands —
	// a goroutine would race with RestoreForegroundWindow stealing focus,
	// causing WebView2 to drop the ExecJS calls.
	// The JS side also stores prompt data in variables and renders them
	// from the focus/visibilitychange handlers as a fallback.
	win.Show()
	time.Sleep(100 * time.Millisecond)
	js := fmt.Sprintf(`setPrompt(%q,%q);startTimer()`, promptIcon, promptName)
	win.ExecJS(js)
}

// HideIndicator hides the floating ghost overlay.
func HideIndicator() {
	indicatorMu.Lock()
	win := indicatorWin
	indicatorMu.Unlock()
	if win == nil {
		return
	}

	slog.Debug("[indicator] HideIndicator called")
	win.Hide()
}
