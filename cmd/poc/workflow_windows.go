//go:build windows

package main

import (
	"fmt"
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

func init() {
	// On Windows, override main to use live hotkey mode
	// This is called before main() when built on Windows
}

// RunWindowsLive registers F6 as a global hotkey and runs the clipboard
// workflow with a simple test message (no LLM).
func RunWindowsLive() {
	cb := clipboard.NewWindowsClipboard()
	kb := keyboard.NewWindowsSimulator()
	hk := hotkey.NewWindowsManager()

	err := hk.Register("correct", "F6", func() {
		fmt.Println("[F6] Correction triggered!")
		winBeep(800, 100) // Short beep to confirm F6 press

		// Step 1: Save original clipboard
		if err := cb.Save(); err != nil {
			fmt.Printf("[ERROR] Failed to save clipboard: %v\n", err)
			return
		}

		// Step 2: Select all text in active window
		kb.SelectAll()
		time.Sleep(50 * time.Millisecond)

		// Step 3: Copy selected text
		kb.Copy()
		time.Sleep(100 * time.Millisecond)

		// Step 4: Read clipboard to get input text
		text, err := cb.Read()
		if err != nil {
			fmt.Printf("[ERROR] Failed to read clipboard: %v\n", err)
			cb.Restore()
			return
		}

		if text == "" {
			fmt.Println("[WARN] Nothing to correct (empty text)")
			cb.Restore()
			return
		}

		fmt.Printf("[INPUT]  %q\n", text)

		// Step 5: Apply simple correction (no LLM)
		corrected := correctText(text)
		fmt.Printf("[OUTPUT] %q\n", corrected)

		// Step 6: Write result to clipboard
		if err := cb.Write(corrected); err != nil {
			fmt.Printf("[ERROR] Failed to write clipboard: %v\n", err)
			cb.Restore()
			return
		}

		// Step 7: Select all and paste
		kb.SelectAll()
		time.Sleep(50 * time.Millisecond)
		kb.Paste()
		time.Sleep(50 * time.Millisecond)

		// Step 8: Restore original clipboard
		cb.Restore()

		winBeep(1200, 150) // Higher beep to confirm correction complete
		fmt.Println("[OK] Done!")
	})
	if err != nil {
		fmt.Printf("Failed to register F6: %v\n", err)
		return
	}

	fmt.Println("F6 registered! Press F6 in any text field to test.")
	fmt.Println("Text will be uppercased with [CORRECTED] prefix.")
	fmt.Println("Press Ctrl+C in this console to exit.")

	hk.Listen()
}
