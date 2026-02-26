//go:build windows

package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/chrixbedardcad/GhostType/clipboard"
	"github.com/chrixbedardcad/GhostType/config"
	"github.com/chrixbedardcad/GhostType/hotkey"
	"github.com/chrixbedardcad/GhostType/keyboard"
	"github.com/chrixbedardcad/GhostType/mode"
)

var (
	kernel32Win = syscall.NewLazyDLL("kernel32.dll")
	procBeep    = kernel32Win.NewProc("Beep")
)

// winBeep plays a short tone using the Windows Beep API.
func winBeep(freq, durationMs uint32) {
	procBeep.Call(uintptr(freq), uintptr(durationMs))
}

func runApp(cfg *config.Config, router *mode.Router) {
	// Windows RegisterHotKey and GetMessageW must run on the same OS thread.
	runtime.LockOSThread()

	cb := clipboard.NewWindowsClipboard()
	kb := keyboard.NewWindowsSimulator()
	hk := hotkey.NewWindowsManager()

	// Mutex-protected cancellation context for in-progress LLM calls.
	var mu sync.Mutex
	var cancelLLM context.CancelFunc

	// Register correction hotkey (Ctrl+G).
	err := hk.Register("correct", cfg.Hotkeys.Correct, func() {
		slog.Info("Correction triggered")
		winBeep(800, 100)

		// Save original clipboard.
		if err := cb.Save(); err != nil {
			slog.Error("Failed to save clipboard", "error", err)
			return
		}

		// Select all + copy.
		if err := kb.SelectAll(); err != nil {
			slog.Error("SelectAll failed", "error", err)
			cb.Restore()
			return
		}
		time.Sleep(50 * time.Millisecond)

		if err := kb.Copy(); err != nil {
			slog.Error("Copy failed", "error", err)
			cb.Restore()
			return
		}
		time.Sleep(100 * time.Millisecond)

		// Read captured text.
		text, err := cb.Read()
		if err != nil {
			slog.Error("Failed to read clipboard", "error", err)
			cb.Restore()
			return
		}
		if text == "" {
			slog.Warn("Nothing to correct (empty text)")
			cb.Restore()
			return
		}

		slog.Info("Captured text", "len", len(text), "text", text)
		fmt.Printf("Captured: %q\n", text)

		// Create cancellable context with timeout.
		timeout := time.Duration(cfg.TimeoutMs) * time.Millisecond
		ctx, cancel := context.WithTimeout(context.Background(), timeout)

		mu.Lock()
		cancelLLM = cancel
		mu.Unlock()

		defer func() {
			cancel()
			mu.Lock()
			cancelLLM = nil
			mu.Unlock()
		}()

		// Send to LLM via mode router.
		corrected, err := router.Process(ctx, mode.ModeCorrect, text)
		if err != nil {
			slog.Error("LLM correction failed", "error", err)
			cb.Restore()
			return
		}

		// Write corrected text, select all, paste.
		if err := cb.Write(corrected); err != nil {
			slog.Error("Failed to write corrected text to clipboard", "error", err)
			cb.Restore()
			return
		}

		if err := kb.SelectAll(); err != nil {
			slog.Error("SelectAll (paste prep) failed", "error", err)
			cb.Restore()
			return
		}
		time.Sleep(50 * time.Millisecond)

		if err := kb.Paste(); err != nil {
			slog.Error("Paste failed", "error", err)
			cb.Restore()
			return
		}
		time.Sleep(50 * time.Millisecond)

		// Restore original clipboard.
		cb.Restore()

		winBeep(1200, 150)
		slog.Info("Correction complete", "corrected", corrected)
		fmt.Printf("Corrected: %q\n", corrected)
	})
	if err != nil {
		slog.Error("Failed to register correction hotkey", "key", cfg.Hotkeys.Correct, "error", err)
		fmt.Fprintf(os.Stderr, "Error: failed to register hotkey %s: %v\n", cfg.Hotkeys.Correct, err)
		return
	}

	// Register cancel hotkey (Escape) — cancels in-progress LLM call, does NOT exit.
	err = hk.Register("cancel", cfg.Hotkeys.Cancel, func() {
		mu.Lock()
		fn := cancelLLM
		mu.Unlock()

		if fn != nil {
			slog.Info("Cancel requested — aborting LLM call")
			fn()
		} else {
			slog.Debug("Cancel pressed but no LLM call in progress")
		}
	})
	if err != nil {
		slog.Error("Failed to register cancel hotkey", "key", cfg.Hotkeys.Cancel, "error", err)
		fmt.Fprintf(os.Stderr, "Error: failed to register hotkey %s: %v\n", cfg.Hotkeys.Cancel, err)
		return
	}

	// SIGINT handler — clean shutdown.
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		fmt.Println("\nGhostType shutting down.")
		slog.Info("GhostType shutting down")
		hk.Stop()
	}()

	fmt.Println("GhostType is ready. Waiting for hotkey input...")
	fmt.Println("Press Ctrl+C to exit.")

	// Block on Windows message loop.
	hk.Listen()
}
