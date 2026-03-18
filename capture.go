package main

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/chrixbedardcad/GhostSpell/clipboard"
	"github.com/chrixbedardcad/GhostSpell/keyboard"
)

// captureMethod indicates which strategy was used to capture text.
// This determines the paste strategy in processMode.
type captureMethod int

const (
	captureViaAXAPI      captureMethod = iota // AX API read — paste via AX write
	captureViaCGEvent                         // CGEventPost + clipboard — paste via CGEventPost
	captureViaAXKeystroke                     // AXUIElementPostKeyboardEvent + clipboard — paste via AX keystroke
	captureViaScript                          // osascript System Events + clipboard — paste via osascript
)

// captureResult bundles the return values of captureText into a single struct
// for readability.
type captureResult struct {
	Text               string
	FullContext         string // full document text for context-aware processing (#192)
	ContextViaClipboard bool  // true if FullContext was captured via Ctrl+A+C (selection is now "all")
	HasAX              bool
	Method             captureMethod
	Err                error
}

// captureText reads text from the focused UI element.
// Strategy:
//  1. Try macOS Accessibility API (kAXSelectedTextAttribute / kAXValueAttribute)
//     — instant, no clipboard pollution, no keyboard simulation needed.
//  2. Fall back to CGEventPost Cmd+C / Cmd+A+Cmd+C clipboard approach.
//  3. Fall back to AXUIElementPostKeyboardEvent Cmd+A+Cmd+C (for Chrome/browsers).
//  4. Fall back to osascript / System Events (true last resort).
//
// Returns a captureResult with the captured text, whether the user had an
// active selection, the capture method used (for paste strategy), and any error.
func captureText(
	promptName string,
	cb *clipboard.Clipboard,
	kb keyboard.Simulator,
) captureResult {
	// Wait for the user to release hotkey modifier keys (e.g. Ctrl from Ctrl+G).
	// On macOS, CGEventPost at kCGHIDEventTap merges with hardware state —
	// if Ctrl is still held, our Cmd+A/C/V become Ctrl+Cmd+A/C/V which apps ignore.
	kb.WaitForModifierRelease()

	// Log the frontmost app for diagnostics.
	// If a GhostSpell window (Settings, Wizard, Update) is focused,
	// skip capture — keyboard simulation would go to our own window.
	// Use IsForegroundOwnProcess() instead of checking the window title,
	// because on Windows FrontAppName() returns the window title which can
	// false-match browser tabs containing "GhostSpell" in the page title.
	appName := kb.FrontAppName()
	slog.Info("captureText: foreground window", "app", appName, "empty", appName == "")
	if kb.IsForegroundOwnProcess() {
		slog.Warn("captureText: GhostSpell window is focused, skipping capture", "window", appName)
		return captureResult{Method: captureViaCGEvent, Err: fmt.Errorf("cannot capture from GhostSpell window — switch to another app first")}
	}

	// --- Strategy 1: Accessibility API (macOS) ---
	// Try reading selected text directly — no keyboard simulation, no clipboard.
	if selected := kb.ReadSelectedText(); selected != "" {
		slog.Info("captureText: got selection via Accessibility API", "len", len(selected))
		// Context-aware (#192): also read the full document text for context.
		// AX API reads are silent — no clipboard change, no selection change.
		fullCtx := ""
		if allText := kb.ReadAllText(); allText != "" && allText != selected {
			fullCtx = allText
			slog.Info("captureText: got full context via Accessibility API", "len", len(fullCtx))
		}
		return captureResult{Text: selected, FullContext: fullCtx, HasAX: true, Method: captureViaAXAPI}
	}
	// No selection — try reading all text from the focused element.
	if allText := kb.ReadAllText(); allText != "" {
		slog.Info("captureText: got all text via Accessibility API", "len", len(allText))
		return captureResult{Text: allText, Method: captureViaAXAPI}
	}
	slog.Debug("captureText: Accessibility API returned no text, falling back to clipboard")

	// --- Strategy 2: Clipboard (Cmd+C / Cmd+A+Cmd+C) ---
	// Clear clipboard so we can detect whether Ctrl+C actually grabbed something.
	slog.Debug("captureText: clearing clipboard...")
	if err := cb.Clear(); err != nil {
		return captureResult{Method: captureViaCGEvent, Err: fmt.Errorf("clear clipboard: %w", err)}
	}
	time.Sleep(50 * time.Millisecond)

	// Try copying the current selection (if any). Some apps are slow to
	// process Copy (especially games, Electron apps, remote desktop). Retry
	// clipboard read up to 3 times with increasing delays to catch late copies.
	slog.Debug("captureText: sending Copy keystroke...")
	if err := kb.Copy(); err != nil {
		return captureResult{Method: captureViaCGEvent, Err: fmt.Errorf("copy: %w", err)}
	}

	var text string
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		delay := time.Duration(100+attempt*50) * time.Millisecond // 100ms, 150ms, 200ms
		time.Sleep(delay)
		var readErr error
		text, readErr = cb.Read()
		if readErr != nil {
			return captureResult{Method: captureViaCGEvent, Err: fmt.Errorf("read clipboard: %w", readErr)}
		}
		if text != "" {
			break
		}
		slog.Debug("captureText: clipboard still empty, retrying", "attempt", attempt+1)
	}
	slog.Info("captureText: clipboard after Copy", "len", len(text), "empty", text == "")

	if text != "" {
		slog.Info("Selection detected", "prompt", promptName, "len", len(text))
		// Context-aware (#192): try to capture full document for context.
		// Uses Ctrl+A + Ctrl+C — visible flash but works in any editable field.
		fullCtx := captureFullContextViaClipboard(text, cb, kb)
		return captureResult{Text: text, FullContext: fullCtx, ContextViaClipboard: fullCtx != "", HasAX: true, Method: captureViaCGEvent}
	}

	// --- Strategy 2.5: AX-routed Copy (browsers/sandboxed apps) ---
	// CGEventPost Copy failed — before falling back to SelectAll, try
	// AXUIElementPostKeyboardEvent which routes Cmd+C through the
	// Accessibility framework by PID. This captures the selection on
	// browsers where CGEventPost is blocked by the sandbox.
	slog.Debug("captureText: CGEventPost Copy empty, trying AX Copy for selection")
	if err := cb.Clear(); err == nil {
		time.Sleep(30 * time.Millisecond)
		if err := kb.CopyAX(); err == nil {
			time.Sleep(150 * time.Millisecond)
			axText, readErr := cb.Read()
			if readErr == nil && axText != "" {
				slog.Info("Selection detected via AX Copy", "prompt", promptName, "len", len(axText))
				fullCtx := captureFullContextViaClipboard(axText, cb, kb)
				return captureResult{Text: axText, FullContext: fullCtx, ContextViaClipboard: fullCtx != "", HasAX: true, Method: captureViaAXKeystroke}
			}
			slog.Debug("captureText: AX Copy also empty")
		} else {
			slog.Debug("captureText: AX Copy not available", "error", err)
		}
	}

	// --- Strategy 2.6: osascript Copy (macOS, Chrome/browsers) ---
	// CGEventPost and AX both failed. Try System Events which routes through
	// the macOS scripting layer — this is the only mechanism that reliably
	// reaches Chrome's renderer process on some macOS versions.
	slog.Debug("captureText: trying osascript Copy for selection")
	if err := cb.Clear(); err == nil {
		time.Sleep(30 * time.Millisecond)
		if err := kb.CopyScript(); err == nil {
			time.Sleep(200 * time.Millisecond)
			scriptText, readErr := cb.Read()
			if readErr == nil && scriptText != "" {
				slog.Info("Selection detected via osascript Copy", "prompt", promptName, "len", len(scriptText))
				fullCtx := captureFullContextViaClipboard(scriptText, cb, kb)
				return captureResult{Text: scriptText, FullContext: fullCtx, ContextViaClipboard: fullCtx != "", HasAX: true, Method: captureViaScript}
			}
			slog.Debug("captureText: osascript Copy also empty")
		} else {
			slog.Debug("captureText: osascript Copy not available", "error", err)
		}
	}

	// No selection detected by any Copy method — fall back to select-all.
	slog.Debug("No selection detected, falling back to select-all", "prompt", promptName)
	if err := kb.SelectAll(); err != nil {
		return captureResult{Method: captureViaCGEvent, Err: fmt.Errorf("select all: %w", err)}
	}
	time.Sleep(50 * time.Millisecond)

	slog.Debug("captureText: sending Copy after select-all...")
	if err := kb.Copy(); err != nil {
		return captureResult{Method: captureViaCGEvent, Err: fmt.Errorf("copy after select-all: %w", err)}
	}
	time.Sleep(100 * time.Millisecond)

	slog.Debug("captureText: reading clipboard after select-all...")
	text, err = cb.Read()
	if err != nil {
		return captureResult{Method: captureViaCGEvent, Err: fmt.Errorf("read clipboard after select-all: %w", err)}
	}
	slog.Info("captureText: clipboard after SelectAll+Copy", "len", len(text))

	if text != "" {
		return captureResult{Text: text, Method: captureViaCGEvent}
	}

	// --- Strategy 3: AXUIElementPostKeyboardEvent (macOS, for Chrome/browsers) ---
	// CGEventPost keystrokes didn't reach the app (common with Chrome's
	// multi-process sandbox). Try AXUIElementPostKeyboardEvent which routes
	// through the Accessibility framework instead of the HID event tap.
	slog.Debug("captureText: CGEventPost clipboard empty, trying AX keystroke fallback")

	if err := cb.Clear(); err != nil {
		return captureResult{Method: captureViaAXKeystroke, Err: fmt.Errorf("clear clipboard (AX fallback): %w", err)}
	}
	time.Sleep(50 * time.Millisecond)

	if err := kb.SelectAllAX(); err != nil {
		slog.Debug("captureText: SelectAllAX not available", "error", err)
		return captureResult{Method: captureViaCGEvent}
	}
	time.Sleep(100 * time.Millisecond)

	if err := kb.CopyAX(); err != nil {
		slog.Debug("captureText: CopyAX not available", "error", err)
		return captureResult{Method: captureViaCGEvent}
	}
	time.Sleep(150 * time.Millisecond)

	text, err = cb.Read()
	if err != nil {
		return captureResult{Method: captureViaAXKeystroke, Err: fmt.Errorf("read clipboard (AX fallback): %w", err)}
	}
	if text != "" {
		slog.Info("captureText: got text via AX keystroke fallback", "len", len(text))
		return captureResult{Text: text, Method: captureViaAXKeystroke}
	}
	slog.Debug("captureText: AX keystroke fallback also returned empty")

	// --- Strategy 4: osascript / System Events (macOS, true last resort) ---
	// Both CGEventPost and AXUIElementPostKeyboardEvent failed. Try osascript
	// which routes through System Events — a completely different mechanism.
	slog.Debug("captureText: trying osascript fallback")

	if err := cb.Clear(); err != nil {
		return captureResult{Method: captureViaScript, Err: fmt.Errorf("clear clipboard (script fallback): %w", err)}
	}
	time.Sleep(50 * time.Millisecond)

	if err := kb.SelectAllScript(); err != nil {
		slog.Debug("captureText: SelectAllScript not available", "error", err)
		return captureResult{Method: captureViaScript}
	}
	time.Sleep(150 * time.Millisecond)

	if err := kb.CopyScript(); err != nil {
		slog.Debug("captureText: CopyScript not available", "error", err)
		return captureResult{Method: captureViaScript}
	}
	time.Sleep(200 * time.Millisecond)

	text, err = cb.Read()
	if err != nil {
		return captureResult{Method: captureViaScript, Err: fmt.Errorf("read clipboard (script fallback): %w", err)}
	}
	if text != "" {
		slog.Info("captureText: got text via osascript fallback", "len", len(text))
	} else {
		slog.Info("captureText: all strategies returned empty", "foreground", appName)
	}

	return captureResult{Text: text, Method: captureViaScript}
}

// captureFullContextViaClipboard attempts to read the full document text
// using Ctrl+A + Ctrl+C. This provides context for smarter LLM corrections.
// Works in editable fields (Notepad, VS Code, browser textareas).
// Returns empty string if context capture fails or isn't useful.
func captureFullContextViaClipboard(selectedText string, cb *clipboard.Clipboard, kb keyboard.Simulator) string {
	slog.Info("captureFullContext: attempting clipboard-based context capture")

	// Clear clipboard before select-all.
	if err := cb.Clear(); err != nil {
		slog.Warn("captureFullContext: clear clipboard failed", "error", err)
		return ""
	}

	// Select All + Copy to get the full document.
	if err := kb.SelectAll(); err != nil {
		slog.Info("captureFullContext: SelectAll failed — no context available", "error", err)
		return ""
	}
	time.Sleep(50 * time.Millisecond)

	if err := kb.Copy(); err != nil {
		slog.Info("captureFullContext: Copy after SelectAll failed", "error", err)
		return ""
	}
	time.Sleep(100 * time.Millisecond)

	fullText, err := cb.Read()
	if err != nil || fullText == "" {
		slog.Info("captureFullContext: clipboard empty after SelectAll+Copy — no context")
		return ""
	}

	// If full text equals the selection, there's no additional context.
	if fullText == selectedText {
		slog.Info("captureFullContext: full text equals selection, no extra context")
		// Restore selection text to clipboard for paste-back.
		cb.Write(selectedText)
		return ""
	}

	slog.Info("captureFullContext: got full document via clipboard", "full_len", len(fullText), "selected_len", len(selectedText))

	// Restore selected text to clipboard so paste-back works correctly.
	cb.Write(selectedText)

	return fullText
}
