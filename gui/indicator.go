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

	// Show the window FIRST, then burst ExecJS calls. On Windows WebView2,
	// ExecJS on a hidden window gets silently dropped. By showing first, the
	// WebView is active and ready to receive JS calls. We burst 5 times with
	// short delays to ensure at least one call lands during the transition.
	win.Show()
	js := fmt.Sprintf(`setPrompt(%q,%q);startTimer()`, promptIcon, promptName)
	go func() {
		for i := 0; i < 5; i++ {
			win.ExecJS(js)
			time.Sleep(50 * time.Millisecond)
		}
	}()
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
