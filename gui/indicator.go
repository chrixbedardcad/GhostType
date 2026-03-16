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

// offScreenX is a position that hides the window off-screen.
const offScreenX = -9999

// CreateIndicator creates a small, frameless overlay window showing an
// animated ghost. The window is always "visible" (from WebView2's perspective)
// but positioned off-screen. ShowIndicator moves it on-screen, HideIndicator
// moves it back off-screen. This ensures ExecJS always works on Windows.
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
		Hidden:            false, // Always "visible" — positioned off-screen when hidden
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

	// Move off-screen immediately.
	indicatorWin.SetPosition(offScreenX, 0)

	slog.Info("[gui] Indicator overlay window created (off-screen)")
}

// getIndicatorPosition returns the bottom-right position for the indicator.
func getIndicatorPosition() (int, int) {
	app := application.Get()
	if app != nil {
		screen := app.Screen.GetPrimary()
		if screen != nil {
			x := screen.WorkArea.X + screen.WorkArea.Width - 276
			y := screen.WorkArea.Y + screen.WorkArea.Height - 68
			return x, y
		}
	}
	return 100, 100 // fallback
}

// ShowIndicator displays the floating ghost in the bottom-right corner of the
// primary screen. Updates prompt icon, name, and starts the timer via ExecJS.
func ShowIndicator(promptIcon, promptName string) {
	indicatorMu.Lock()
	win := indicatorWin
	indicatorMu.Unlock()
	if win == nil {
		return
	}

	slog.Debug("[indicator] ShowIndicator called", "prompt", promptName, "icon", promptIcon)

	// Update content via ExecJS FIRST (window is "visible" off-screen, so
	// ExecJS works reliably). Then move on-screen.
	js := fmt.Sprintf(
		`document.getElementById('pi').textContent=%q;`+
			`document.getElementById('pn').textContent=%q;`+
			`document.getElementById('sep').style.display='';`+
			`document.getElementById('t').style.display='';`+
			`document.getElementById('t').textContent='0s';`+
			`var _s=Date.now();if(window._iv)clearInterval(window._iv);`+
			`window._iv=setInterval(function(){document.getElementById('t').textContent=Math.floor((Date.now()-_s)/1000)+'s'},1000);`,
		promptIcon, promptName)
	win.ExecJS(js)
	time.Sleep(50 * time.Millisecond) // let JS execute

	// Move on-screen.
	x, y := getIndicatorPosition()
	win.SetPosition(x, y)
}

// HideIndicator moves the indicator off-screen and stops the timer.
func HideIndicator() {
	indicatorMu.Lock()
	win := indicatorWin
	indicatorMu.Unlock()
	if win == nil {
		return
	}

	slog.Debug("[indicator] HideIndicator called")
	win.SetPosition(offScreenX, 0)
	win.ExecJS(`if(window._iv)clearInterval(window._iv);document.getElementById('t').textContent='0s';`)
}

// PopIndicator briefly shows the indicator pill with the prompt icon and name
// (no timer), then auto-hides after a short delay.
func PopIndicator(promptIcon, promptName string) {
	indicatorMu.Lock()
	win := indicatorWin
	indicatorMu.Unlock()
	if win == nil {
		return
	}

	slog.Debug("[indicator] PopIndicator called", "prompt", promptName, "icon", promptIcon)

	// Update content (no timer for pop mode).
	js := fmt.Sprintf(
		`document.getElementById('pi').textContent=%q;`+
			`document.getElementById('pn').textContent=%q;`+
			`document.getElementById('sep').style.display='none';`+
			`document.getElementById('t').style.display='none';`+
			`if(window._iv)clearInterval(window._iv);`,
		promptIcon, promptName)
	win.ExecJS(js)
	time.Sleep(50 * time.Millisecond)

	// Move on-screen.
	x, y := getIndicatorPosition()
	win.SetPosition(x, y)

	// Auto-hide after 1.5 seconds.
	go func() {
		time.Sleep(1500 * time.Millisecond)
		indicatorMu.Lock()
		w := indicatorWin
		indicatorMu.Unlock()
		if w != nil {
			w.SetPosition(offScreenX, 0)
		}
	}()
}
