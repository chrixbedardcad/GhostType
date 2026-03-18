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

	slog.Info("Captured text", "prompt", promptName, "len", len(cap.Text), "selection", cap.HasAX, "method", cap.Method, "text", cap.Text)
	fmt.Printf("[%s] Captured: %q\n", promptName, cap.Text)

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
	gui.ShowIndicator(promptIcon, promptName, modelLabel)

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
	textToSend := cap.Text
	if cap.FullContext != "" {
		// Truncate context to ~2000 chars around the selection to avoid
		// sending huge documents and burning tokens.
		ctx2000 := cap.FullContext
		if len([]rune(ctx2000)) > 2000 {
			// Find the selection position and extract ~2000 chars around it.
			pos := strings.Index(ctx2000, cap.Text)
			if pos < 0 {
				pos = 0
			}
			start := pos - 800
			if start < 0 {
				start = 0
			}
			end := pos + len(cap.Text) + 800
			if end > len(ctx2000) {
				end = len(ctx2000)
			}
			ctx2000 = ctx2000[start:end]
		}
		textToSend = "=== FULL DOCUMENT (for context only — do NOT include this in your response) ===\n" +
			ctx2000 +
			"\n\n=== SELECTED TEXT (apply your instructions ONLY to this portion) ===\n" +
			cap.Text
		slog.Info("Context-aware mode: sending selected text with document context", "selected_len", len(cap.Text), "context_len", len(ctx2000))
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

	slog.Info(promptName+" complete", "result", result)
	fmt.Printf("[%s] Result: %q\n", promptName, result)
}
