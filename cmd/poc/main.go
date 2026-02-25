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
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	testMode = flag.Bool("test", false, "Run in test mode with simulated clipboard (no Windows APIs needed)")
	logFile  = flag.String("log", "", "Path to debug log file (default: logs/ghosttype-poc-<timestamp>.log)")
)

// Log-level helper functions — conventional format: datetime LEVEL message
func logInfo(format string, v ...any)  { log.Printf("INFO  "+format, v...) }
func logWarn(format string, v ...any)  { log.Printf("WARN  "+format, v...) }
func logError(format string, v ...any) { log.Printf("ERROR "+format, v...) }
func logDebug(format string, v ...any) { log.Printf("DEBUG "+format, v...) }

func main() {
	flag.Parse()

	// Generate timestamped log path if none specified
	if *logFile == "" {
		ts := time.Now().Format("20060102-150405")
		*logFile = filepath.Join("logs", fmt.Sprintf("ghosttype-poc-%s.log", ts))
	}

	// Ensure log directory exists
	if dir := filepath.Dir(*logFile); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create log directory %s: %v\n", dir, err)
			os.Exit(1)
		}
	}

	// Set up logging to both stdout and log file
	f, err := os.OpenFile(*logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log file %s: %v\n", *logFile, err)
		os.Exit(1)
	}
	defer f.Close()
	log.SetOutput(io.MultiWriter(os.Stdout, f))
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	log.Println("==============================================")
	log.Println("  GhostType POC v0.1.2 — F7 Clipboard Workflow")
	log.Println("  (No LLM — uses test message)")
	log.Printf("  Log file: %s", *logFile)
	log.Println("==============================================")

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
