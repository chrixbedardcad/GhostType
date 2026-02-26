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

// processMode captures text from the active window, sends it through the LLM
// with the given mode, and pastes the result back. This is the shared workflow
// for correction, translation, and rewrite hotkeys.
func processMode(
	modeName string,
	m mode.Mode,
	cfg *config.Config,
	router *mode.Router,
	cb *clipboard.WindowsClipboard,
	kb *keyboard.WindowsSimulator,
	mu *sync.Mutex,
	cancelLLM *context.CancelFunc,
) {
	slog.Info(modeName + " triggered")
	winBeep(800, 100)

	// Save original clipboard.
	if err := cb.Save(); err != nil {
		slog.Error("Failed to save clipboard", "mode", modeName, "error", err)
		return
	}

	// Select all + copy.
	if err := kb.SelectAll(); err != nil {
		slog.Error("SelectAll failed", "mode", modeName, "error", err)
		cb.Restore()
		return
	}
	time.Sleep(50 * time.Millisecond)

	if err := kb.Copy(); err != nil {
		slog.Error("Copy failed", "mode", modeName, "error", err)
		cb.Restore()
		return
	}
	time.Sleep(100 * time.Millisecond)

	// Read captured text.
	text, err := cb.Read()
	if err != nil {
		slog.Error("Failed to read clipboard", "mode", modeName, "error", err)
		cb.Restore()
		return
	}
	if text == "" {
		slog.Warn("Nothing to process (empty text)", "mode", modeName)
		cb.Restore()
		return
	}

	slog.Info("Captured text", "mode", modeName, "len", len(text), "text", text)
	fmt.Printf("[%s] Captured: %q\n", modeName, text)

	// Create cancellable context with timeout.
	timeout := time.Duration(cfg.TimeoutMs) * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	mu.Lock()
	*cancelLLM = cancel
	mu.Unlock()

	defer func() {
		cancel()
		mu.Lock()
		*cancelLLM = nil
		mu.Unlock()
	}()

	// Send to LLM via mode router.
	result, err := router.Process(ctx, m, text)
	if err != nil {
		slog.Error("LLM processing failed", "mode", modeName, "error", err)
		cb.Restore()
		return
	}

	// Write result, select all, paste.
	if err := cb.Write(result); err != nil {
		slog.Error("Failed to write result to clipboard", "mode", modeName, "error", err)
		cb.Restore()
		return
	}

	if err := kb.SelectAll(); err != nil {
		slog.Error("SelectAll (paste prep) failed", "mode", modeName, "error", err)
		cb.Restore()
		return
	}
	time.Sleep(50 * time.Millisecond)

	if err := kb.Paste(); err != nil {
		slog.Error("Paste failed", "mode", modeName, "error", err)
		cb.Restore()
		return
	}
	time.Sleep(50 * time.Millisecond)

	// Restore original clipboard.
	cb.Restore()

	winBeep(1200, 150)
	slog.Info(modeName+" complete", "result", result)
	fmt.Printf("[%s] Result: %q\n", modeName, result)
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

	// Register correction hotkey.
	err := hk.Register("correct", cfg.Hotkeys.Correct, func() {
		processMode("Correction", mode.ModeCorrect, cfg, router, cb, kb, &mu, &cancelLLM)
	})
	if err != nil {
		slog.Error("Failed to register correction hotkey", "key", cfg.Hotkeys.Correct, "error", err)
		fmt.Fprintf(os.Stderr, "Error: failed to register hotkey %s: %v\n", cfg.Hotkeys.Correct, err)
		return
	}

	// Register translation hotkey.
	err = hk.Register("translate", cfg.Hotkeys.Translate, func() {
		processMode("Translation", mode.ModeTranslate, cfg, router, cb, kb, &mu, &cancelLLM)
	})
	if err != nil {
		slog.Error("Failed to register translate hotkey", "key", cfg.Hotkeys.Translate, "error", err)
		fmt.Fprintf(os.Stderr, "Error: failed to register hotkey %s: %v\n", cfg.Hotkeys.Translate, err)
		return
	}

	// Register toggle-language hotkey — cycles translation target, no LLM call.
	err = hk.Register("toggle_language", cfg.Hotkeys.ToggleLanguage, func() {
		name := router.ToggleTranslateTarget()
		slog.Info("Translation target toggled", "target", name)
		fmt.Printf("Translation target: %s\n", name)
		winBeep(600, 80)
	})
	if err != nil {
		slog.Error("Failed to register toggle-language hotkey", "key", cfg.Hotkeys.ToggleLanguage, "error", err)
		fmt.Fprintf(os.Stderr, "Error: failed to register hotkey %s: %v\n", cfg.Hotkeys.ToggleLanguage, err)
		return
	}

	// Register rewrite hotkey.
	err = hk.Register("rewrite", cfg.Hotkeys.Rewrite, func() {
		processMode("Rewrite", mode.ModeRewrite, cfg, router, cb, kb, &mu, &cancelLLM)
	})
	if err != nil {
		slog.Error("Failed to register rewrite hotkey", "key", cfg.Hotkeys.Rewrite, "error", err)
		fmt.Fprintf(os.Stderr, "Error: failed to register hotkey %s: %v\n", cfg.Hotkeys.Rewrite, err)
		return
	}

	// Register cycle-template hotkey — cycles rewrite template, no LLM call.
	err = hk.Register("cycle_template", cfg.Hotkeys.CycleTemplate, func() {
		name := router.CycleTemplate()
		slog.Info("Rewrite template cycled", "template", name)
		fmt.Printf("Rewrite template: %s\n", name)
		winBeep(600, 80)
	})
	if err != nil {
		slog.Error("Failed to register cycle-template hotkey", "key", cfg.Hotkeys.CycleTemplate, "error", err)
		fmt.Fprintf(os.Stderr, "Error: failed to register hotkey %s: %v\n", cfg.Hotkeys.CycleTemplate, err)
		return
	}

	// Register cancel hotkey — cancels in-progress LLM call, does NOT exit.
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
