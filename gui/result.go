package gui

import (
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// Result popup window state.
var (
	resultWin   *application.WebviewWindow
	resultMu    sync.Mutex
	resultOpen  bool
	resultText  string // current result text
	resultMeta  resultMetadata
)

type resultMetadata struct {
	Prompt string `json:"prompt"`
	Icon   string `json:"icon"`
	Model  string `json:"model"`
}

// ShowResult opens a popup window displaying the LLM result.
// Used for prompts with DisplayMode="popup" (e.g. Define, Explain).
func ShowResult(text, promptName, promptIcon, modelLabel string) {
	resultMu.Lock()

	// Store result for JS to fetch via service methods.
	resultText = text
	resultMeta = resultMetadata{
		Prompt: promptName,
		Icon:   promptIcon,
		Model:  modelLabel,
	}

	// Close previous result window if open.
	if resultWin != nil && resultOpen {
		prev := resultWin
		resultWin = nil
		resultOpen = false
		resultMu.Unlock()
		prev.Close()
		resultMu.Lock()
	}

	resultMu.Unlock()

	app := application.Get()
	if app == nil {
		slog.Error("[gui] ShowResult: no running Wails app")
		return
	}

	slog.Info("[gui] ShowResult: opening popup", "prompt", promptName, "text_len", len(text))

	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "GhostSpell — " + promptName,
		Width:  500,
		Height: 420,
		URL:    "/result.html",
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:               application.MacBackdropTranslucent,
		},
	})

	resultMu.Lock()
	resultWin = win
	resultOpen = true
	resultMu.Unlock()

	win.Focus()

	win.OnWindowEvent(events.Common.WindowClosing, func(event *application.WindowEvent) {
		resultMu.Lock()
		resultOpen = false
		resultWin = nil
		resultMu.Unlock()
	})
}

// CloseResultWindow closes the result popup (called from JS).
func CloseResultWindow() {
	resultMu.Lock()
	win := resultWin
	resultWin = nil
	resultOpen = false
	resultMu.Unlock()

	if win != nil {
		win.Close()
	}
}

// GetResultText returns the current popup result text (called from JS).
func GetResultText() string {
	resultMu.Lock()
	defer resultMu.Unlock()
	return resultText
}

// GetResultMeta returns JSON metadata about the current result (called from JS).
func GetResultMeta() string {
	resultMu.Lock()
	defer resultMu.Unlock()
	data, _ := json.Marshal(resultMeta)
	return string(data)
}
