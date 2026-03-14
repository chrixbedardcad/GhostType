package main

import (
	"fmt"
	"log/slog"
	"strings"
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
	Text   string
	HasAX  bool
	Method captureMethod
	Err    error
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
	appName := kb.FrontAppName()
	slog.Info("captureText: foreground window", "app", appName, "empty", appName == "")
	if appName != "" && strings.Contains(appName, "GhostSpell") {
		slog.Warn("captureText: GhostSpell window is focused, skipping capture", "window", appName)
		return captureResult{Method: captureViaCGEvent, Err: fmt.Errorf("cannot capture from GhostSpell window — switch to another app first")}
	}

	// --- Strategy 1: Accessibility API (macOS) ---
	// Try reading selected text directly — no keyboard simulation, no clipboard.
	if selected := kb.ReadSelectedText(); selected != "" {
		slog.Info("captureText: got selection via Accessibility API", "len", len(selected))
		return captureResult{Text: selected, HasAX: true, Method: captureViaAXAPI}
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

	// Try copying the current selection (if any).
	slog.Debug("captureText: sending Copy keystroke...")
	if err := kb.Copy(); err != nil {
		return captureResult{Method: captureViaCGEvent, Err: fmt.Errorf("copy: %w", err)}
	}
	time.Sleep(100 * time.Millisecond)

	slog.Debug("captureText: reading clipboard...")
	text, err := cb.Read()
	if err != nil {
		return captureResult{Method: captureViaCGEvent, Err: fmt.Errorf("read clipboard: %w", err)}
	}
	slog.Info("captureText: clipboard after Copy", "len", len(text), "empty", text == "")

	if text != "" {
		slog.Info("Selection detected", "prompt", promptName, "len", len(text))
		return captureResult{Text: text, HasAX: true, Method: captureViaCGEvent}
	}

	// No selection — fall back to select-all + copy.
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
