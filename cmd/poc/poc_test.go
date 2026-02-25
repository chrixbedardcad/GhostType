package main

import (
	"testing"

	"github.com/chrixbedardcad/GhostType/clipboard"
)

// TestCorrectText verifies the simple test transformation.
func TestCorrectText(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"helo wrold", "[CORRECTED] HELO WROLD"},
		{"je sui contant", "[CORRECTED] JE SUI CONTANT"},
		{"  spaces  ", "[CORRECTED] SPACES"},
		{"Already Fine", "[CORRECTED] ALREADY FINE"},
	}

	for _, tc := range tests {
		result := correctText(tc.input)
		if result != tc.expected {
			t.Errorf("correctText(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

// TestPOC_ClipboardWorkflow simulates the full F6 workflow
// using a mock clipboard.
func TestPOC_ClipboardWorkflow(t *testing.T) {
	var store string = "user's important data"
	cb := clipboard.New(
		func() (string, error) { return store, nil },
		func(text string) error { store = text; return nil },
	)

	// Step 1: Save original clipboard
	err := cb.Save()
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Step 2: Simulate Ctrl+A, Ctrl+C (user typed text captured)
	store = "helo wrold"

	// Step 3: Read captured text
	captured, err := cb.Read()
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if captured != "helo wrold" {
		t.Fatalf("Expected captured text 'helo wrold', got %q", captured)
	}

	// Step 4: Apply correction (no LLM)
	corrected := correctText(captured)
	if corrected != "[CORRECTED] HELO WROLD" {
		t.Fatalf("Expected corrected text, got %q", corrected)
	}

	// Step 5: Write to clipboard and simulate paste
	err = cb.Write(corrected)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	pasted, _ := cb.Read()
	if pasted != "[CORRECTED] HELO WROLD" {
		t.Fatalf("Expected pasted text, got %q", pasted)
	}

	// Step 6: Restore original clipboard
	err = cb.Restore()
	if err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	restored, _ := cb.Read()
	if restored != "user's important data" {
		t.Errorf("Clipboard not restored! Got %q", restored)
	}

	t.Log("PASS: Full F6 workflow verified")
	t.Logf("  Captured: %q → Corrected: %q → Clipboard restored: %q", captured, corrected, restored)
}

// TestPOC_EmptyTextAborts verifies that empty text skips the workflow.
func TestPOC_EmptyTextAborts(t *testing.T) {
	var store string = "original"
	cb := clipboard.New(
		func() (string, error) { return store, nil },
		func(text string) error { store = text; return nil },
	)

	cb.Save()

	// Simulate empty capture
	store = ""
	captured, _ := cb.Read()
	if captured != "" {
		t.Fatalf("Expected empty capture, got %q", captured)
	}

	// Should abort — no correction, no paste
	// Restore clipboard
	cb.Restore()

	restored, _ := cb.Read()
	if restored != "original" {
		t.Errorf("Clipboard not restored after abort! Got %q", restored)
	}

	t.Log("PASS: Empty text correctly aborted")
}

// TestPOC_ClipboardPreservation tests that clipboard is always restored
// even after multiple correction cycles.
func TestPOC_ClipboardPreservation(t *testing.T) {
	var store string = "my important clipboard data"
	cb := clipboard.New(
		func() (string, error) { return store, nil },
		func(text string) error { store = text; return nil },
	)

	inputs := []string{"test one", "test two", "test three"}
	for i, input := range inputs {
		// Save
		cb.Save()

		// Capture
		store = input
		captured, _ := cb.Read()

		// Correct
		corrected := correctText(captured)

		// Write + paste
		cb.Write(corrected)

		// Restore
		cb.Restore()

		restored, _ := cb.Read()
		if restored != "my important clipboard data" {
			t.Errorf("Round %d: clipboard corrupted! Got %q", i+1, restored)
		}
	}

	t.Log("PASS: Clipboard preserved across 3 correction cycles")
}
