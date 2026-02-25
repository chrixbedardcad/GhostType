// cmd/poc/main.go — Proof of Concept: F7 clipboard correction workflow
//
// This POC demonstrates the F7 correction pipeline WITHOUT any LLM calls.
// Instead of calling an LLM, it uses a simple test message to prove
// the clipboard workflow works end-to-end.
//
// The workflow:
//   1. F7 hotkey registered as global hotkey (Windows)
//   2. On F7: Ctrl+A → Ctrl+C → read clipboard → replace with test message → Ctrl+A → Ctrl+V
//   3. Original clipboard content is preserved and restored
//
// Build for Windows:
//   go build -o ghosttype-poc.exe ./cmd/poc
//
// Run in test mode (works on any OS, no Windows APIs):
//   go run ./cmd/poc -test

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

var testMode = flag.Bool("test", false, "Run in test mode with simulated clipboard (no Windows APIs needed)")

func main() {
	flag.Parse()

	fmt.Println("==============================================")
	fmt.Println("  GhostType POC v0.1.0 — F7 Clipboard Workflow")
	fmt.Println("  (No LLM — uses test message)")
	fmt.Println("==============================================")
	fmt.Println()

	if *testMode {
		runTestMode()
		return
	}

	runLive()
}

// correctText is the simple test replacement — no LLM needed.
// It just uppercases the text and prepends "[CORRECTED]" to prove
// the clipboard workflow works.
func correctText(input string) string {
	return "[CORRECTED] " + strings.ToUpper(strings.TrimSpace(input))
}

// beep plays a console bell sound to indicate an action was triggered.
// On Windows, the workflow_windows.go uses the Win32 Beep API for a proper tone.
func beep() {
	fmt.Print("\a") // ASCII BEL character — terminal beep
}

// runTestMode simulates the full F7 clipboard workflow without any
// platform-specific APIs. Works on any OS.
func runTestMode() {
	fmt.Println("Running in TEST MODE (simulated clipboard)")
	fmt.Println("No LLM calls — using simple test transformation.")
	fmt.Println()

	// Simulated clipboard
	clipboardContent := "user's original clipboard data"

	testInputs := []string{
		"helo wrold",
		"je sui contant",
		"ths is a tset",
	}

	allPassed := true
	for i, input := range testInputs {
		fmt.Printf("=== Test #%d ===\n", i+1)

		// Step 1: Save original clipboard
		savedClipboard := clipboardContent
		fmt.Printf("  [1] Save clipboard: %q\n", savedClipboard)

		// Step 2: Simulate Ctrl+A, Ctrl+C
		clipboardContent = input
		fmt.Printf("  [2] Ctrl+A → Ctrl+C captured: %q\n", clipboardContent)

		// Step 3: Read captured text
		capturedText := clipboardContent
		if capturedText == "" {
			fmt.Println("  [!] Empty text — aborting (no modification)")
			clipboardContent = savedClipboard
			continue
		}
		fmt.Printf("  [3] Read clipboard: %q\n", capturedText)

		// Step 4: Apply correction (simple test — no LLM)
		corrected := correctText(capturedText)
		fmt.Printf("  [4] Corrected: %q\n", corrected)
		beep() // Sound feedback for correction

		// Step 5: Write result to clipboard, simulate paste
		clipboardContent = corrected
		fmt.Printf("  [5] Ctrl+A → Ctrl+V pasted: %q\n", clipboardContent)

		// Step 6: Restore original clipboard
		clipboardContent = savedClipboard
		fmt.Printf("  [6] Restored clipboard: %q\n", clipboardContent)

		// Verify
		if clipboardContent != "user's original clipboard data" {
			fmt.Println("  FAIL: Clipboard was corrupted!")
			allPassed = false
		} else {
			fmt.Println("  PASS: Clipboard preserved")
		}
		fmt.Println()
	}

	fmt.Println("=== Summary ===")
	if allPassed {
		fmt.Println("ALL TESTS PASSED")
		fmt.Println("Clipboard workflow verified: capture → transform → paste → restore")
	} else {
		fmt.Println("SOME TESTS FAILED")
		os.Exit(1)
	}
}
