//go:build windows

package main

import (
	"os"
	"runtime"
	"syscall"
	"time"

	"github.com/chrixbedardcad/GhostType/clipboard"
	"github.com/chrixbedardcad/GhostType/hotkey"
	"github.com/chrixbedardcad/GhostType/keyboard"
)

var (
	kernel32Win  = syscall.NewLazyDLL("kernel32.dll")
	procBeep     = kernel32Win.NewProc("Beep")
)

// winBeep plays a short tone using the Windows Beep API.
// freq is in Hz, duration in milliseconds.
func winBeep(freq, durationMs uint32) {
	procBeep.Call(uintptr(freq), uintptr(durationMs))
}

func runLive() {
	RunWindowsLive()
}

// RunWindowsLive registers Ctrl+G as a global hotkey and runs the clipboard
// workflow with a simple test message (no LLM).
func RunWindowsLive() {
	// Windows RegisterHotKey and GetMessageW must run on the same OS thread.
	// Without this, Go's scheduler can move the goroutine between threads,
	// causing the message loop to stop receiving hotkey events after the first press.
	runtime.LockOSThread()

	logInfo("Creating clipboard, keyboard, hotkey managers")
	cb := clipboard.NewWindowsClipboard()
	kb := keyboard.NewWindowsSimulator()
	hk := hotkey.NewWindowsManager()

	logInfo("Registering Ctrl+G hotkey...")
	err := hk.Register("correct", "Ctrl+G", func() {
		logInfo("---- Correction triggered! ----")
		winBeep(800, 100) // Short beep to confirm hotkey press

		// Step 1: Save original clipboard
		logDebug("Step 1: Saving original clipboard...")
		if err := cb.Save(); err != nil {
			logError("Failed to save clipboard: %v", err)
			return
		}
		logDebug("Step 1: Clipboard saved OK")

		// Step 2: Select all text in active window
		logDebug("Step 2: Sending Ctrl+A (SelectAll)...")
		if err := kb.SelectAll(); err != nil {
			logError("SelectAll failed: %v", err)
			cb.Restore()
			return
		}
		logDebug("Step 2: SelectAll sent, sleeping 50ms")
		time.Sleep(50 * time.Millisecond)

		// Step 3: Copy selected text
		logDebug("Step 3: Sending Ctrl+C (Copy)...")
		if err := kb.Copy(); err != nil {
			logError("Copy failed: %v", err)
			cb.Restore()
			return
		}
		logDebug("Step 3: Copy sent, sleeping 100ms")
		time.Sleep(100 * time.Millisecond)

		// Step 4: Read clipboard to get input text
		logDebug("Step 4: Reading clipboard...")
		text, err := cb.Read()
		if err != nil {
			logError("Failed to read clipboard: %v", err)
			cb.Restore()
			return
		}

		if text == "" {
			logWarn("Nothing to correct (empty text)")
			cb.Restore()
			return
		}

		logInfo("Captured text (%d chars): %q", len(text), text)

		// Step 5: Apply simple correction (no LLM)
		corrected := correctText(text)
		logInfo("Corrected text: %q", corrected)

		// Step 6: Write result to clipboard
		logDebug("Step 6: Writing corrected text to clipboard...")
		if err := cb.Write(corrected); err != nil {
			logError("Failed to write clipboard: %v", err)
			cb.Restore()
			return
		}
		logDebug("Step 6: Clipboard written OK")

		// Step 7: Select all and paste
		logDebug("Step 7: Sending Ctrl+A (SelectAll)...")
		if err := kb.SelectAll(); err != nil {
			logError("SelectAll (paste prep) failed: %v", err)
			cb.Restore()
			return
		}
		time.Sleep(50 * time.Millisecond)

		logDebug("Step 7: Sending Ctrl+V (Paste)...")
		if err := kb.Paste(); err != nil {
			logError("Paste failed: %v", err)
			cb.Restore()
			return
		}
		logDebug("Step 7: Paste sent, sleeping 50ms")
		time.Sleep(50 * time.Millisecond)

		// Step 8: Restore original clipboard
		logDebug("Step 8: Restoring original clipboard...")
		cb.Restore()
		logDebug("Step 8: Clipboard restored")

		winBeep(1200, 150) // Higher beep to confirm correction complete
		logInfo("---- Correction complete! ----")
	})
	if err != nil {
		logError("Failed to register Ctrl+G: %v", err)
		return
	}
	logInfo("Ctrl+G hotkey registered OK")

	logInfo("Registering Ctrl+B diagnostic ping hotkey...")
	err = hk.Register("ping", "Ctrl+B", func() {
		logInfo("Ctrl+B ping — hotkey delivery confirmed")
		winBeep(600, 200)
	})
	if err != nil {
		logError("Failed to register Ctrl+B: %v", err)
		return
	}
	logInfo("Ctrl+B hotkey registered OK")

	logInfo("Registering Escape hotkey...")
	err = hk.Register("quit", "Escape", func() {
		logInfo("Escape pressed — exiting cleanly.")
		hk.Unregister("correct")
		hk.Unregister("ping")
		hk.Unregister("quit")
		os.Exit(0)
	})
	if err != nil {
		logError("Failed to register Escape: %v", err)
		return
	}
	logInfo("Escape hotkey registered OK")

	logInfo("Ctrl+G registered! Press Ctrl+G in any text field to test.")
	logInfo("Text will be uppercased with [CORRECTED] prefix.")
	logInfo("Press Ctrl+B to test hotkey delivery (ping)")
	logInfo("Press Escape to exit.")

	hk.Listen()
}
