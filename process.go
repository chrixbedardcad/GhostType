package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chrixbedardcad/GhostSpell/clipboard"
	"github.com/chrixbedardcad/GhostSpell/config"
	"github.com/chrixbedardcad/GhostSpell/gui"
	"github.com/chrixbedardcad/GhostSpell/keyboard"
	"github.com/chrixbedardcad/GhostSpell/mode"
	"github.com/chrixbedardcad/GhostSpell/screenshot"
	"github.com/chrixbedardcad/GhostSpell/sound"
	"github.com/chrixbedardcad/GhostSpell/stats"
)

// appStats is the global stats tracker, set from app.go.
var appStats *stats.Stats

// processingGuard prevents concurrent processMode execution.
// macOS Carbon RegisterEventHotKey can send multiple Keydown events for a
// single press, launching parallel goroutines that corrupt clipboard state.
// TryLock rejects duplicates; Unlock happens when the current call finishes.
var processingGuard sync.Mutex

// processingActive is set while processMode is running. Used by the tray to
// suppress menu opening on macOS (the NSMenu modal event loop blocks keyboard
// simulation to the target app).
var processingActive atomic.Bool

// processMode captures text from the active window, sends it through the LLM
// with the given prompt, and pastes the result back.
func processMode(
	promptName string,
	promptIdx int,
	cfg *config.Config,
	router *mode.Router,
	cb *clipboard.Clipboard,
	kb keyboard.Simulator,
	mu *sync.Mutex,
	cancelLLM *context.CancelFunc,
	startAnim func(),
	stopAnim func(),
) {
	if !processingGuard.TryLock() {
		// Second press while processing — cancel the active LLM request.
		slog.Info("Hotkey pressed again — cancelling active request")
		mu.Lock()
		if *cancelLLM != nil {
			(*cancelLLM)()
		}
		mu.Unlock()
		sound.StopWorkingLoop()
		sound.PlayCancel() // Play cancel sound immediately — before GUI cleanup for sharp feedback.
		if stopAnim != nil {
			stopAnim()
		}
		gui.HideIndicator()
		gui.PopIndicator("\U0001F6D1", "Cancelled")
		return
	}
	processingActive.Store(true)

	// Create cancel context immediately so a second Ctrl+G can cancel at
	// any point — including during text capture, not only during LLM processing.
	cancelCtx, cancelFn := context.WithCancel(context.Background())
	mu.Lock()
	*cancelLLM = cancelFn
	mu.Unlock()

	defer func() {
		cancelFn()
		mu.Lock()
		*cancelLLM = nil
		mu.Unlock()
		sound.StopWorkingLoop()
		if stopAnim != nil {
			stopAnim()
		}
		gui.HideIndicator()
		processingActive.Store(false)
		processingGuard.Unlock()
	}()
	slog.Info(promptName + " triggered")
	slog.Debug("Playing working sound loop...")
	sound.StartWorkingLoop()
	if startAnim != nil {
		startAnim()
	}

	// --- Vision path: capture screenshot instead of text ---
	if promptIdx >= 0 && promptIdx < len(cfg.Prompts) && cfg.Prompts[promptIdx].Vision {
		processVision(promptName, promptIdx, cfg, router, kb, cancelCtx, startAnim, stopAnim)
		return
	}

	// NOTE: ShowIndicator() is deferred until AFTER captureText(). On Windows,
	// the indicator overlay (AlwaysOnTop, IgnoreMouseEvents=false) steals focus
	// from the target app, causing SendInput(Ctrl+C) to go to the indicator
	// instead of Notepad/Chrome/etc. The tray animation still provides visual
	// feedback during capture; the indicator appears once LLM processing starts.

	// Save original clipboard (only if PreserveClipboard is enabled).
	// When disabled, the LLM result stays in the clipboard after paste.
	preserveClipboard := cfg.PreserveClipboard
	restoreClipboard := func() {
		if preserveClipboard {
			cb.Restore()
		}
	}
	if preserveClipboard {
		slog.Debug("Saving clipboard...")
		if err := cb.Save(); err != nil {
			slog.Error("Failed to save clipboard", "prompt", promptName, "error", err)
			return
		}
		slog.Debug("Clipboard saved")
	}

	// Capture text — detects existing selection or falls back to select-all.
	slog.Debug("Capturing text...")
	cap := captureText(promptName, cb, kb)
	// Bail out immediately if cancelled during capture (second Ctrl+G).
	if cancelCtx.Err() != nil {
		slog.Info("Request cancelled during capture", "prompt", promptName)
		restoreClipboard()
		return
	}
	if cap.Err != nil {
		slog.Error("Failed to capture text", "prompt", promptName, "error", cap.Err)
		sound.StopWorkingLoop()
		if werr := cb.Write("\U0001F47B\u274C"); werr == nil { // 👻❌
			kb.Paste()
			time.Sleep(150 * time.Millisecond)
		}
		restoreClipboard()
		sound.PlayError()
		return
	}
	if cap.Text == "" {
		slog.Warn("Nothing to process (empty text)", "prompt", promptName)
		sound.StopWorkingLoop()
		if werr := cb.Write("\U0001F47B\U0001FAE5"); werr == nil { // 👻🫥
			kb.Paste()
			time.Sleep(150 * time.Millisecond)
		}
		restoreClipboard()
		sound.PlayError()
		return
	}

	// Check input length limit.
	if cfg.MaxInputChars > 0 && len([]rune(cap.Text)) > cfg.MaxInputChars {
		slog.Warn("Text exceeds max input limit", "prompt", promptName, "chars", len([]rune(cap.Text)), "limit", cfg.MaxInputChars)
		// Stop working loop first so it doesn't kill the error sound.
		sound.StopWorkingLoop()
		// Collapse selection to end (Right arrow) so text is preserved,
		// then paste the warning indicator after the text.
		kb.PressRight()
		time.Sleep(50 * time.Millisecond)
		if werr := cb.Write("\U0001F47B\u26A0\uFE0F"); werr != nil { // 👻⚠️
			slog.Error("Failed to write warning indicator to clipboard", "error", werr)
		}
		kb.Paste()
		time.Sleep(150 * time.Millisecond)
		restoreClipboard()
		sound.PlayError()
		return
	}

	slog.Info("Captured text", "prompt", promptName, "len", len(cap.Text), "selection", cap.HasAX, "method", cap.Method)
	slog.Debug("Captured text content", "text", cap.Text) // #200: user text at Debug only (privacy)
	fmt.Printf("[%s] Captured %d chars (method=%d)\n", promptName, len(cap.Text), cap.Method)

	// Save the foreground window HWND before showing the indicator.
	// On Windows the indicator overlay (AlwaysOnTop, no WS_EX_NOACTIVATE)
	// steals focus when shown. Without this, all subsequent SendInput calls
	// (Ctrl+A, Ctrl+V) would go to the indicator instead of the target app.
	kb.SaveForegroundWindow()

	// Text captured — now safe to show the indicator overlay. It won't
	// interfere with keyboard simulation since capture is complete.
	// Pass prompt icon and name so the indicator pill shows which prompt is active.
	promptIcon := ""
	if promptIdx >= 0 && promptIdx < len(cfg.Prompts) {
		promptIcon = cfg.Prompts[promptIdx].Icon
	}
	modelLabel := cfg.DefaultModel
	if promptIdx >= 0 && promptIdx < len(cfg.Prompts) && cfg.Prompts[promptIdx].LLM != "" {
		modelLabel = cfg.Prompts[promptIdx].LLM
	}
	// Resolve the actual model identifier for the indicator (#204).
	// modelLabel is the config key (e.g. "GhostSpell Local") — show the
	// actual model name (e.g. "qwen3.5-2b") which is more informative.
	indicatorModel := modelLabel
	if me, ok := cfg.Models[modelLabel]; ok && me.Model != "" {
		indicatorModel = me.Model
	}
	gui.ShowIndicator(promptIcon, promptName, indicatorModel)

	// Immediately restore focus to the target app. The indicator is visible
	// (visual feedback) but the target app has focus for keyboard simulation.
	kb.RestoreForegroundWindow()

	// Create timeout context as child of cancelCtx — inherits cancel from
	// second Ctrl+G press, plus its own per-provider deadline.
	timeout := time.Duration(router.TimeoutForPrompt(promptIdx)) * time.Millisecond
	ctx, cancel := context.WithTimeout(cancelCtx, timeout)
	defer cancel()

	// Build text to send — if we have full document context, wrap the selected
	// text with context so the LLM can make better corrections (#192).
	// Cap total size to avoid crashes with small local models (#216).
	textToSend := cap.Text
	maxInput := cfg.MaxInputChars
	if maxInput <= 0 {
		maxInput = 2000 // safe default
	}
	if cap.FullContext != "" {
		// Skip context-aware mode for very large documents — the write-back
		// (Ctrl+A+V of the entire document) is unreliable and can crash/hang
		// the target app when the document exceeds a reasonable size (#216).
		maxFullDoc := maxInput * 5
		if maxFullDoc < 10000 {
			maxFullDoc = 10000
		}
		if len([]rune(cap.FullContext)) > maxFullDoc {
			slog.Warn("Context-aware: document too large, skipping context mode", "full_len", len(cap.FullContext), "limit", maxFullDoc)
			cap.FullContext = ""
			cap.ContextViaClipboard = false
		}
	}
	if cap.FullContext != "" {
		// Context budget: total text must fit within maxInput.
		// Reserve space for markers (~200 chars) and the selection itself.
		markerOverhead := 200
		maxContextChars := maxInput - len([]rune(cap.Text)) - markerOverhead
		if maxContextChars < 200 {
			// Not enough room for meaningful context — skip it.
			slog.Debug("Context-aware: skipping context, not enough budget", "maxInput", maxInput, "selectedLen", len(cap.Text))
		} else {
			ctx := cap.FullContext
			if len([]rune(ctx)) > maxContextChars {
				// Truncate around the selection position.
				pos := strings.Index(ctx, cap.Text)
				if pos < 0 {
					pos = 0
				}
				half := maxContextChars / 2
				start := pos - half
				if start < 0 {
					start = 0
				}
				end := start + maxContextChars
				if end > len(ctx) {
					end = len(ctx)
					start = end - maxContextChars
					if start < 0 {
						start = 0
					}
				}
				ctx = ctx[start:end]
			}
			textToSend = "=== FULL DOCUMENT (for context only — do NOT include this in your response) ===\n" +
				ctx +
				"\n\n=== SELECTED TEXT (apply your instructions ONLY to this portion) ===\n" +
				cap.Text
			slog.Info("Context-aware mode: sending selected text with document context", "selected_len", len(cap.Text), "context_len", len(ctx), "total_len", len(textToSend))
		}
	}

	// Send to LLM via mode router.
	slog.Debug("Sending to LLM...", "prompt", promptName, "text_len", len(textToSend))
	llmStart := time.Now()
	resp, err := router.Process(ctx, promptIdx, textToSend)
	llmElapsed := time.Since(llmStart)

	// LLM call complete — hide the overlay and restore focus to the target
	// window before any keyboard simulation (paste, select-all, etc).
	// This is defense-in-depth: RestoreForegroundWindow was already called
	// after ShowIndicator, but the user may have clicked elsewhere during
	// processing, or the indicator may have recaptured focus.
	gui.HideIndicator()
	kb.RestoreForegroundWindow()

	// Extract provider/model metadata from response (if available).
	respProvider, respModel := "", ""
	if resp != nil {
		respProvider = resp.Provider
		respModel = resp.Model
	}

	// Resolve the model label for this prompt (per-prompt LLM override or default).
	llmLabel := cfg.DefaultModel
	if promptIdx >= 0 && promptIdx < len(cfg.Prompts) && cfg.Prompts[promptIdx].LLM != "" {
		llmLabel = cfg.Prompts[promptIdx].LLM
	}

	// Helper to record stats non-intrusively.
	recordStat := func(status, errMsg, output string) {
		if appStats == nil {
			return
		}
		outWords := len(strings.Fields(output))
		appStats.Record(stats.Entry{
			Timestamp:  time.Now(),
			Prompt:     promptName,
			PromptIcon: promptIcon,
			Provider:   respProvider,
			Model:      respModel,
			ModelLabel:  llmLabel,
			InputChars:  len(cap.Text),
			InputWords:  len(strings.Fields(cap.Text)),
			OutputChars: len(output),
			OutputWords: outWords,
			DurationMs:  llmElapsed.Milliseconds(),
			Status:      status,
			Error:       errMsg,
			Changed:     output != "" && strings.TrimSpace(output) != strings.TrimSpace(cap.Text),
		})
	}

	if err != nil {
		slog.Error("LLM processing failed", "prompt", promptName, "error", err)
		sound.StopWorkingLoop()

		// User cancelled with second Ctrl+G — deselect text, show cancel
		// indicator, and restore clipboard to initial state.
		if ctx.Err() == context.Canceled && !strings.Contains(err.Error(), "deadline exceeded") {
			slog.Info("Request cancelled by user", "prompt", promptName)
			kb.PressRight()
			time.Sleep(50 * time.Millisecond)
			gui.PopIndicator("\U0001F6D1", "Cancelled")
			recordStat("cancelled", "", "")
			restoreClipboard()
			return
		}

		// Distinguish timeout from other errors.
		status := "error"
		indicator := "\U0001F47B\u274C" // 👻❌ (generic error)
		if ctx.Err() == context.DeadlineExceeded || strings.Contains(err.Error(), "deadline exceeded") {
			indicator = "\U0001F47B\u23F3" // 👻⏳ (timeout)
			status = "timeout"
		}
		recordStat(status, err.Error(), "")
		// Collapse selection to end (Right arrow) so the original text is
		// preserved — the emoji is appended after it, not replacing it.
		kb.PressRight()
		time.Sleep(50 * time.Millisecond)
		if werr := cb.Write(indicator); werr != nil {
			slog.Error("Failed to write error indicator to clipboard", "error", werr)
		}
		kb.Paste()
		time.Sleep(150 * time.Millisecond)
		restoreClipboard()
		sound.PlayError()
		return
	}

	// Result received — stop working loop.
	result := resp.Text
	sound.StopWorkingLoop()

	// If the LLM returned identical text, signal "no changes needed"
	// instead of replacing with the same content.
	if strings.TrimSpace(result) == strings.TrimSpace(cap.Text) {
		slog.Info("LLM returned identical text (no changes needed)", "prompt", promptName)
		recordStat("identical", "", result)
		kb.PressRight()
		time.Sleep(50 * time.Millisecond)
		if werr := cb.Write("\U0001F47B\u2705"); werr == nil { // 👻✅
			kb.Paste()
			time.Sleep(150 * time.Millisecond)
		}
		restoreClipboard()
		sound.PlaySuccess()
		return
	}

	sound.PlaySuccess()
	recordStat("success", "", result)

	// --- Popup mode: show result in a window instead of pasting ---
	// Used for lookup prompts (Define, Explain) where the source text may be
	// non-editable (browser, PDF viewer) and pasting would fail or be unwanted.
	if promptIdx >= 0 && promptIdx < len(cfg.Prompts) && cfg.Prompts[promptIdx].DisplayMode == "popup" {
		slog.Info("DisplayMode=popup — showing result in popup window", "prompt", promptName)
		kb.PressRight() // deselect text in source app
		time.Sleep(50 * time.Millisecond)
		gui.ShowResult(result, promptName, promptIcon, modelLabel)
		restoreClipboard()
		return
	}

	// --- Write result back ---

	// Context-aware full-document replacement (#192): when context was captured
	// via clipboard (Ctrl+A+C), the selection is now "all". Instead of pasting
	// just the result, compute the full document with the selected portion
	// replaced and paste the entire thing. Works for Notepad, VS Code, browser
	// textareas — any editable field where Ctrl+A+V works.
	if cap.ContextViaClipboard && cap.FullContext != "" {
		newFull := strings.Replace(cap.FullContext, cap.Text, result, 1)
		if newFull != cap.FullContext {
			slog.Info("Context-aware write-back: replacing selection within full document", "prompt", promptName, "full_len", len(newFull))
			if err := cb.Write(newFull); err != nil {
				slog.Error("Failed to write full document to clipboard", "prompt", promptName, "error", err)
				restoreClipboard()
				return
			}
			time.Sleep(50 * time.Millisecond)
			// Re-select all and paste the full document with the fix applied.
			if err := kb.SelectAll(); err != nil {
				slog.Error("SelectAll (context write-back) failed", "prompt", promptName, "error", err)
			}
			time.Sleep(50 * time.Millisecond)
			if err := kb.Paste(); err != nil {
				slog.Error("Paste (context write-back) failed", "prompt", promptName, "error", err)
				restoreClipboard()
				return
			}
			time.Sleep(150 * time.Millisecond)
			restoreClipboard()
			slog.Info(promptName+" complete (context-aware)", "result_len", len(result))
			return
		}
		slog.Debug("Context-aware: Replace() didn't change the document, falling back to normal paste")
	}

	written := false

	// Strategy 1: AX API write — for apps where we read via AX API (Cocoa apps).
	// Bypasses clipboard and CGEventPost entirely.
	if cap.Method == captureViaAXAPI {
		if cap.HasAX {
			written = kb.WriteSelectedText(result)
			if written {
				slog.Info("Wrote result via AX API (selected text)", "prompt", promptName)
			} else {
				slog.Debug("AX WriteSelectedText failed, falling back to clipboard paste", "prompt", promptName)
			}
		} else {
			written = kb.WriteAllText(result)
			if written {
				slog.Info("Wrote result via AX API (all text)", "prompt", promptName)
			} else {
				slog.Debug("AX WriteAllText failed, falling back to clipboard paste", "prompt", promptName)
			}
		}
	}

	// Strategy 2 & 3: Clipboard-based paste.
	if !written {
		if err := cb.Write(result); err != nil {
			slog.Error("Failed to write result to clipboard", "prompt", promptName, "error", err)
			restoreClipboard()
			return
		}
		slog.Debug("Result written to clipboard", "prompt", promptName, "result_len", len(result))
		time.Sleep(50 * time.Millisecond)

		switch cap.Method {
		case captureViaAXKeystroke:
			// Strategy 3: AX keystroke paste.
			if !cap.HasAX {
				if err := kb.SelectAllAX(); err != nil {
					slog.Error("SelectAllAX (paste prep) failed", "prompt", promptName, "error", err)
				}
				time.Sleep(50 * time.Millisecond)
			}
			if err := kb.PasteAX(); err != nil {
				slog.Error("PasteAX failed", "prompt", promptName, "error", err)
				restoreClipboard()
				return
			}
		case captureViaScript:
			// Strategy 4: osascript paste.
			if !cap.HasAX {
				if err := kb.SelectAllScript(); err != nil {
					slog.Error("SelectAllScript (paste prep) failed", "prompt", promptName, "error", err)
				}
				time.Sleep(100 * time.Millisecond)
			}
			if err := kb.PasteScript(); err != nil {
				slog.Error("PasteScript failed", "prompt", promptName, "error", err)
				restoreClipboard()
				return
			}
		default:
			// Strategy 2: CGEventPost paste — for native apps.
			if !cap.HasAX {
				if err := kb.SelectAll(); err != nil {
					slog.Error("SelectAll (paste prep) failed", "prompt", promptName, "error", err)
					restoreClipboard()
					return
				}
				time.Sleep(50 * time.Millisecond)
			}
			if err := kb.Paste(); err != nil {
				slog.Error("Paste failed", "prompt", promptName, "error", err)
				restoreClipboard()
				return
			}
		}
		// Wait for the target app to process the paste event.
		time.Sleep(150 * time.Millisecond)
	}

	// Restore original clipboard.
	restoreClipboard()

	slog.Info(promptName+" complete", "result_len", len(result))
	slog.Debug("Result content", "result", result) // #200: user text at Debug only (privacy)
	fmt.Printf("[%s] Complete (%d chars)\n", promptName, len(result))
}

// processVision handles the vision prompt path: capture screenshot → send to LLM → show popup.
// Called from processMode when the active prompt has Vision: true.
func processVision(
	promptName string,
	promptIdx int,
	cfg *config.Config,
	router *mode.Router,
	kb keyboard.Simulator,
	cancelCtx context.Context,
	startAnim func(),
	stopAnim func(),
) {
	// Small delay to let the user's window fully come to focus after hotkey press.
	time.Sleep(100 * time.Millisecond)

	// Capture the active window screenshot.
	slog.Info("Vision mode: capturing active window screenshot", "prompt", promptName)
	imgData, err := screenshot.CaptureActiveWindow()
	if err != nil {
		slog.Error("Vision: screenshot capture failed", "prompt", promptName, "error", err)
		sound.StopWorkingLoop()
		gui.PopIndicator("\U0001F47B\u274C", "Screenshot failed")
		sound.PlayError()
		return
	}
	slog.Info("Vision: screenshot captured", "prompt", promptName, "size_bytes", len(imgData))

	// Check if cancelled during capture.
	if cancelCtx.Err() != nil {
		slog.Info("Vision: cancelled during capture", "prompt", promptName)
		return
	}

	// Show indicator with prompt info.
	promptIcon := ""
	if promptIdx >= 0 && promptIdx < len(cfg.Prompts) {
		promptIcon = cfg.Prompts[promptIdx].Icon
	}
	modelLabel := cfg.DefaultModel
	if promptIdx >= 0 && promptIdx < len(cfg.Prompts) && cfg.Prompts[promptIdx].LLM != "" {
		modelLabel = cfg.Prompts[promptIdx].LLM
	}
	indicatorModel := modelLabel
	if me, ok := cfg.Models[modelLabel]; ok && me.Model != "" {
		indicatorModel = me.Model
	}
	gui.ShowIndicator(promptIcon, promptName, indicatorModel)

	// Create timeout context.
	timeout := time.Duration(router.TimeoutForPrompt(promptIdx)) * time.Millisecond
	ctx, cancel := context.WithTimeout(cancelCtx, timeout)
	defer cancel()

	// Send screenshot + prompt to LLM.
	llmStart := time.Now()
	resp, err := router.ProcessWithImages(ctx, promptIdx, "", [][]byte{imgData})
	llmElapsed := time.Since(llmStart)

	gui.HideIndicator()

	// Record stats.
	respProvider, respModel := "", ""
	if resp != nil {
		respProvider = resp.Provider
		respModel = resp.Model
	}
	llmLabel := cfg.DefaultModel
	if promptIdx >= 0 && promptIdx < len(cfg.Prompts) && cfg.Prompts[promptIdx].LLM != "" {
		llmLabel = cfg.Prompts[promptIdx].LLM
	}
	recordVisionStat := func(status, errMsg, output string) {
		if appStats == nil {
			return
		}
		appStats.Record(stats.Entry{
			Timestamp:   time.Now(),
			Prompt:      promptName,
			PromptIcon:  promptIcon,
			Provider:    respProvider,
			Model:       respModel,
			ModelLabel:  llmLabel,
			InputChars:  len(imgData),
			InputWords:  0,
			OutputChars: len(output),
			OutputWords: len(strings.Fields(output)),
			DurationMs:  llmElapsed.Milliseconds(),
			Status:      status,
			Error:       errMsg,
		})
	}

	if err != nil {
		slog.Error("Vision: LLM processing failed", "prompt", promptName, "error", err)
		sound.StopWorkingLoop()

		if ctx.Err() == context.Canceled && !strings.Contains(err.Error(), "deadline exceeded") {
			gui.PopIndicator("\U0001F6D1", "Cancelled")
			recordVisionStat("cancelled", "", "")
			return
		}

		status := "error"
		if ctx.Err() == context.DeadlineExceeded || strings.Contains(err.Error(), "deadline exceeded") {
			status = "timeout"
		}
		recordVisionStat(status, err.Error(), "")
		gui.PopIndicator("\U0001F47B\u274C", "Vision failed")
		sound.PlayError()
		return
	}

	result := resp.Text
	sound.StopWorkingLoop()
	sound.PlaySuccess()
	recordVisionStat("success", "", result)

	// Always show vision results in a popup window.
	slog.Info("Vision: showing result in popup", "prompt", promptName, "result_len", len(result))
	gui.ShowResult(result, promptName, promptIcon, modelLabel)
	fmt.Printf("[%s] Vision complete (%d chars)\n", promptName, len(result))
}
