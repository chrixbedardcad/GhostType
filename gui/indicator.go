package gui

import (
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"time"

	"github.com/chrixbedardcad/GhostSpell/config"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

var (
	indicatorWin   *application.WebviewWindow
	indicatorApp   *application.App
	indicatorMu    sync.Mutex
	popGeneration  uint64 // incremented on each PopIndicator to cancel stale hide timers
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
	bgColour := application.RGBA{Red: 0, Green: 0, Blue: 0, Alpha: 0}
	ignoreMouse := false // must receive clicks for drag + context menu
	if runtime.GOOS == "windows" {
		// Translucent uses WS_EX_NOREDIRECTIONBITMAP (DWM composition) which
		// gives true visual transparency. Clicks work because Wails runtime
		// is loaded via indicator-react.html and --wails-draggable handles drag.
		bgType = application.BackgroundTypeTranslucent
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

	fmt.Println("[indicator] Window created (React hybrid) URL=/indicator-react.html?window=indicator")
	slog.Info("[gui] Indicator window created (React hybrid)", "url", "/indicator-react.html?window=indicator")

	// Save position on user drag. Programmatic moves set indicatorMoving=true
	// so they're ignored. Only actual Wails-native drag triggers a save.
	indicatorWin.OnWindowEvent(events.Windows.WindowDidMove, func(e *application.WindowEvent) {
		indicatorMu.Lock()
		moving := indicatorMoving
		indicatorMu.Unlock()
		if moving {
			return
		}
		x, y := indicatorWin.Position()
		slog.Debug("[indicator] User drag detected", "x", x, "y", y)
		indicatorMu.Lock()
		indicatorPos = "custom"
		indicatorSavedX = x
		indicatorSavedY = y
		indicatorMu.Unlock()
		if indicatorConfigSaver != nil {
			indicatorConfigSaver(x, y)
		}
	})

	// Block until React has time to mount and register event listeners.
	fmt.Println("[indicator] Waiting 800ms for React to mount...")
	time.Sleep(800 * time.Millisecond)
	indicatorReady = true
	fmt.Println("[indicator] React page assumed ready")
}

// indicatorConfigSaver persists drag position to config file.
var indicatorConfigSaver func(x, y int)

// SetIndicatorConfigSaver sets the callback to persist position and mode to config.
func SetIndicatorConfigSaver(cfg *config.Config, configPath string) {
	indicatorConfigSaver = func(x, y int) {
		cfg.IndicatorPosition = "custom"
		cfg.IndicatorX = x
		cfg.IndicatorY = y
		config.WriteDefault(configPath, cfg)
		slog.Info("[indicator] Position saved to config (custom)", "x", x, "y", y)
	}
	indicatorModeSaver = func(mode string) {
		cfg.IndicatorMode = mode
		config.WriteDefault(configPath, cfg)
		slog.Info("[indicator] Mode saved to config", "mode", mode)
	}
}

// currentPromptVoice/Vision track the active prompt's mode flags.
// Updated by SetCurrentPromptFlags() when the prompt changes.
// Injected into every indicator event by emitIndicatorEvent().
var currentPromptVoice bool
var currentPromptVision bool

// SetCurrentPromptFlags updates the voice/vision flags for the active prompt.
func SetCurrentPromptFlags(voice, vision bool) {
	indicatorMu.Lock()
	currentPromptVoice = voice
	currentPromptVision = vision
	indicatorMu.Unlock()
}

// indicatorModeSaver persists indicator mode to config file.
var indicatorModeSaver func(mode string)

// SaveIndicatorMode persists the indicator mode via the registered saver.
func SaveIndicatorMode(mode string) {
	if indicatorModeSaver != nil {
		indicatorModeSaver(mode)
	}
}

// indicatorPos stores the configured position.
var indicatorPos = "top-right"

// indicatorMode stores the configured mode: "processing" (default), "always", "hidden".
var indicatorMode = "processing"

// indicatorSavedX/Y stores the user's dragged position.
var indicatorSavedX, indicatorSavedY int

// indicatorMoving is true during programmatic SetPosition calls.
// WindowDidMove only saves position when this is false (user drag).
var indicatorMoving bool


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

// moveIndicatorWindow sets position programmatically without triggering save.
func moveIndicatorWindow(win *application.WebviewWindow, x, y int) {
	indicatorMu.Lock()
	indicatorMoving = true
	indicatorMu.Unlock()
	moveIndicatorWindow(win, x, y)
	// Small delay to let WindowDidMove fire and be ignored before clearing flag.
	go func() {
		time.Sleep(50 * time.Millisecond)
		indicatorMu.Lock()
		indicatorMoving = false
		indicatorMu.Unlock()
	}()
}


// emitIndicatorEvent sends a state update to the React indicator.
func emitIndicatorEvent(data map[string]any) {
	// Inject current prompt flags into every event so React always has them.
	indicatorMu.Lock()
	data["voice"] = currentPromptVoice
	data["vision"] = currentPromptVision
	indicatorMu.Unlock()
	app := application.Get()
	if app != nil {
		app.Event.Emit("indicatorState", data)
	}
}

// getIndicatorPositionForSize calculates position based on configured corner.
// Preset positions (top-right, top-left, etc.) always compute from screen geometry.
// Custom drag position (indicator_position="custom") uses saved X/Y coordinates.
func getIndicatorPositionForSize(w, h int) (int, int) {
	indicatorMu.Lock()
	pos := indicatorPos
	sx, sy := indicatorSavedX, indicatorSavedY
	indicatorMu.Unlock()

	// Custom position: user has dragged the indicator to a specific spot.
	if pos == "custom" && (sx > 0 || sy > 0) {
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

	// Preset positions — always computed from screen, never from saved X/Y.
	switch pos {
	case "top-left":
		return screen.WorkArea.X + 20, screen.WorkArea.Y + 20
	case "center-top":
		return screen.WorkArea.X + (screen.WorkArea.Width-w)/2, screen.WorkArea.Y + 20
	case "bottom-left":
		return screen.WorkArea.X + 20, screen.WorkArea.Y + screen.WorkArea.Height - h - 20
	case "bottom-right":
		return screen.WorkArea.X + screen.WorkArea.Width - w - 20, screen.WorkArea.Y + screen.WorkArea.Height - h - 20
	case "center":
		return screen.WorkArea.X + (screen.WorkArea.Width-w)/2, screen.WorkArea.Y + screen.WorkArea.Height/3
	default: // "top-right" and any unknown value
		return screen.WorkArea.X + screen.WorkArea.Width - w - 20, screen.WorkArea.Y + 20
	}
}

// getIndicatorPosition returns the pill (300x52) position.
func getIndicatorPosition() (int, int) { return getIndicatorPositionForSize(300, 52) }

// getIdlePosition returns the idle circle (48x48) position.
// For presets, the idle circle is positioned so the ghost image aligns with
// where it appears in the pill — same left edge for left/center anchors,
// same right edge for right anchors.
func getIdlePosition() (int, int) {
	px, py := getIndicatorPositionForSize(300, 52)
	indicatorMu.Lock()
	pos := indicatorPos
	indicatorMu.Unlock()
	// Right-anchored: align right edges. Idle right = pill right.
	if pos == "top-right" || pos == "bottom-right" {
		return px + (300 - 48), py
	}
	// Center: center the circle within the pill span.
	if pos == "center-top" || pos == "center" {
		return px + (300-48)/2, py
	}
	// Left-anchored + custom: same left edge.
	return px, py
}

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

	win.SetSize(300, 52)
	x, y := getIndicatorPosition()
	moveIndicatorWindow(win, x, y)
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
	moveIndicatorWindow(win, x, y)
	emitIndicatorEvent(map[string]any{"state": "idle"})
}

// ClearIndicatorProcessing clears the processing flag without hiding.
func ClearIndicatorProcessing() {
	indicatorMu.Lock()
	indicatorProcessing = false
	indicatorMu.Unlock()
}

// ForceHideIndicator hides the indicator regardless of processing state.
// Called from processMode's defer when the task truly ends.
func ForceHideIndicator() {
	indicatorMu.Lock()
	indicatorProcessing = false
	indicatorMu.Unlock()
	HideIndicator()
}

// ShowRecordingIndicator shows the indicator in recording mode with the recording flag.
// Uses pill size so the timer is visible, with ghost pulse + red dot.
func ShowRecordingIndicator() {
	slog.Debug("[indicator] ShowRecordingIndicator called")

	indicatorMu.Lock()
	pos := indicatorPos
	if pos == "hidden" {
		indicatorMu.Unlock()
		return
	}
	indicatorProcessing = true
	ensureIndicatorWindow()
	win := indicatorWin
	indicatorMu.Unlock()
	if win == nil {
		return
	}

	win.SetSize(300, 52)
	x, y := getIndicatorPosition()
	moveIndicatorWindow(win, x, y)
	emitIndicatorEvent(map[string]any{
		"state": "processing", "icon": "\U0001F399\uFE0F", "name": "Recording...",
		"recording": true,
	})
}

// UpdateTranscript sends partial transcription text to the indicator during recording.
func UpdateTranscript(text string) {
	indicatorMu.Lock()
	win := indicatorWin
	indicatorMu.Unlock()
	if win == nil {
		return
	}
	// Widen the pill to fit transcript text.
	win.SetSize(400, 72)
	x, y := getIndicatorPosition()
	moveIndicatorWindow(win, x, y)
	emitIndicatorEvent(map[string]any{
		"state": "processing", "recording": true,
		"icon": "\U0001F399\uFE0F", "name": "Recording...",
		"transcript": text,
	})
}

// EmitAudioLevel sends the current mic level to the indicator for visual feedback.
func EmitAudioLevel(level float32) {
	app := application.Get()
	if app != nil {
		app.Event.Emit("audioLevel", map[string]any{"level": level})
	}
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
	indicatorProcessing = true
	ensureIndicatorWindow()
	win := indicatorWin
	indicatorMu.Unlock()
	if win == nil {
		return
	}

	win.SetSize(300, 52)
	x, y := getIndicatorPosition()
	slog.Info("[indicator] ShowIndicator: positioning", "size", "260x52", "x", x, "y", y, "prompt", promptName, "model", modelLabel)
	moveIndicatorWindow(win, x, y)
	emitIndicatorEvent(map[string]any{
		"state": "processing", "icon": promptIcon, "name": promptName, "model": modelLabel,
	})
}

// indicatorProcessing tracks whether the indicator is in an active processing state.
// Set to true by ShowIndicator/ShowRecordingIndicator, cleared by explicit hide calls
// from processMode's defer. Prevents pop auto-hide timers from collapsing during work.
var indicatorProcessing bool

// HideIndicator hides the indicator or returns to idle in "always" mode.
// Skips if the indicator is in an active processing state (recording, transcribing, etc.)
// unless forceHide is used internally.
func HideIndicator() {
	indicatorMu.Lock()
	win := indicatorWin
	mode := indicatorMode
	processing := indicatorProcessing
	indicatorMu.Unlock()
	if win == nil {
		return
	}

	// Don't collapse to idle while actively processing (recording, transcribing, LLM).
	// The processMode defer calls ForceHideIndicator when the task truly ends.
	if processing {
		slog.Debug("[indicator] HideIndicator: skipped (processing active)")
		return
	}

	slog.Debug("[indicator] HideIndicator called", "mode", mode)

	if mode == "always" {
		win.SetSize(48, 48)
		x, y := getIdlePosition()
		slog.Debug("[indicator] HideIndicator: returning to idle", "x", x, "y", y)
		moveIndicatorWindow(win, x, y)
		emitIndicatorEvent(map[string]any{"state": "idle"})
		return
	}

	// Move off-screen. Keep 48x48 so WebView2 stays renderable.
	slog.Debug("[indicator] HideIndicator: moving off-screen")
	moveIndicatorWindow(win, -9999, -9999)
	win.SetSize(48, 48)
	emitIndicatorEvent(map[string]any{"state": "hidden"})
}

// PopIndicatorDone shows the completion summary with prompt, model, and elapsed time.
func PopIndicatorDone(promptIcon, promptName, modelName string, elapsedSec float64) {
	indicatorMu.Lock()
	ensureIndicatorWindow()
	win := indicatorWin
	popGeneration++
	gen := popGeneration
	indicatorMu.Unlock()
	if win == nil {
		return
	}

	slog.Info("[indicator] PopIndicatorDone called", "prompt", promptName, "model", modelName, "elapsed", elapsedSec)

	win.SetSize(300, 52)
	x, y := getIndicatorPosition()
	moveIndicatorWindow(win, x, y)
	emitIndicatorEvent(map[string]any{
		"state": "done", "icon": promptIcon, "name": promptName,
		"model": modelName, "elapsed": elapsedSec,
	})

	go func() {
		time.Sleep(8 * time.Second)
		indicatorMu.Lock()
		current := popGeneration
		indicatorMu.Unlock()
		if current == gen {
			HideIndicator()
		}
	}()
}

// PopIndicatorWithModel shows prompt name + model briefly, then auto-hides.
func PopIndicatorWithModel(promptIcon, promptName, modelName string) {
	popIndicatorInner(promptIcon, promptName, modelName)
}

// PopIndicator shows prompt name briefly, then auto-hides.
func PopIndicator(promptIcon, promptName string) {
	popIndicatorInner(promptIcon, promptName, "")
}

func popIndicatorInner(promptIcon, promptName, modelName string) {
	indicatorMu.Lock()
	ensureIndicatorWindow()
	win := indicatorWin
	popGeneration++ // cancel any previous pop's hide timer
	gen := popGeneration
	indicatorMu.Unlock()
	if win == nil {
		return
	}

	slog.Info("[indicator] PopIndicator called", "prompt", promptName, "icon", promptIcon)

	win.SetSize(300, 52)
	x, y := getIndicatorPosition()
	slog.Info("[indicator] PopIndicator: positioning", "size", "260x52", "x", x, "y", y)
	moveIndicatorWindow(win, x, y)
	evt := map[string]any{"state": "pop", "icon": promptIcon, "name": promptName}
	if modelName != "" {
		evt["model"] = modelName
	}
	emitIndicatorEvent(evt)

	go func() {
		time.Sleep(8 * time.Second)
		// Only hide if no newer pop has started since.
		indicatorMu.Lock()
		current := popGeneration
		indicatorMu.Unlock()
		if current == gen {
			HideIndicator()
		}
	}()
}

// SaveIndicatorPosition saves the drag position (called from React JS).
// Sets position to "custom" so preset corners don't override the drag position.
// This is the ONLY place position is saved — never from programmatic SetPosition.
func (s *SettingsService) SaveIndicatorPosition(x, y int) string {
	slog.Debug("[GUI] SaveIndicatorPosition", "x", x, "y", y)
	indicatorMu.Lock()
	indicatorPos = "custom"
	indicatorMu.Unlock()
	SetIndicatorSavedPosition(x, y)
	if s.cfgCopy != nil {
		s.cfgCopy.IndicatorX = x
		s.cfgCopy.IndicatorY = y
		s.validateAndSave()
	}
	return "ok"
}
