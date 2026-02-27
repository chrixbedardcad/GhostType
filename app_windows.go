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

	"github.com/chrixbedardcad/GhostType/assets"
	"github.com/chrixbedardcad/GhostType/clipboard"
	"github.com/chrixbedardcad/GhostType/config"
	"github.com/chrixbedardcad/GhostType/hotkey"
	"github.com/chrixbedardcad/GhostType/keyboard"
	"github.com/chrixbedardcad/GhostType/mode"
	"github.com/chrixbedardcad/GhostType/sound"
	"github.com/chrixbedardcad/GhostType/tray"
)

// captureText detects whether the user has an active text selection. It clears
// the clipboard, copies, and checks. If text was copied the user had a selection.
// Otherwise it falls back to select-all + copy.
// Returns the captured text, whether the user had an active selection, and any error.
func captureText(
	modeName string,
	cb *clipboard.Clipboard,
	kb *keyboard.WindowsSimulator,
) (text string, hadSelection bool, err error) {
	// Clear clipboard so we can detect whether Ctrl+C actually grabbed something.
	if err := cb.Clear(); err != nil {
		return "", false, fmt.Errorf("clear clipboard: %w", err)
	}
	time.Sleep(50 * time.Millisecond)

	// Try copying the current selection (if any).
	if err := kb.Copy(); err != nil {
		return "", false, fmt.Errorf("copy: %w", err)
	}
	time.Sleep(100 * time.Millisecond)

	text, err = cb.Read()
	if err != nil {
		return "", false, fmt.Errorf("read clipboard: %w", err)
	}

	if text != "" {
		slog.Info("Selection detected", "mode", modeName, "len", len(text))
		return text, true, nil
	}

	// No selection — fall back to select-all + copy.
	slog.Debug("No selection detected, falling back to select-all", "mode", modeName)
	if err := kb.SelectAll(); err != nil {
		return "", false, fmt.Errorf("select all: %w", err)
	}
	time.Sleep(50 * time.Millisecond)

	if err := kb.Copy(); err != nil {
		return "", false, fmt.Errorf("copy after select-all: %w", err)
	}
	time.Sleep(100 * time.Millisecond)

	text, err = cb.Read()
	if err != nil {
		return "", false, fmt.Errorf("read clipboard after select-all: %w", err)
	}

	return text, false, nil
}

// processMode captures text from the active window, sends it through the LLM
// with the given mode, and pastes the result back. This is the shared workflow
// for correction, translation, and rewrite hotkeys.
func processMode(
	modeName string,
	m mode.Mode,
	cfg *config.Config,
	router *mode.Router,
	cb *clipboard.Clipboard,
	kb *keyboard.WindowsSimulator,
	mu *sync.Mutex,
	cancelLLM *context.CancelFunc,
) {
	slog.Info(modeName + " triggered")
	sound.PlayWorking()

	// Save original clipboard.
	if err := cb.Save(); err != nil {
		slog.Error("Failed to save clipboard", "mode", modeName, "error", err)
		return
	}

	// Capture text — detects existing selection or falls back to select-all.
	text, hadSelection, err := captureText(modeName, cb, kb)
	if err != nil {
		slog.Error("Failed to capture text", "mode", modeName, "error", err)
		cb.Restore()
		return
	}
	if text == "" {
		slog.Warn("Nothing to process (empty text)", "mode", modeName)
		cb.Restore()
		return
	}

	slog.Info("Captured text", "mode", modeName, "len", len(text), "selection", hadSelection, "text", text)
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
		sound.PlayError()
		cb.Restore()
		return
	}

	// Write result to clipboard.
	if err := cb.Write(result); err != nil {
		slog.Error("Failed to write result to clipboard", "mode", modeName, "error", err)
		cb.Restore()
		return
	}

	// Paste-prep: only select-all before paste when we used select-all to capture.
	// If the user had a selection, it's still active after Ctrl+C — Ctrl+V replaces it.
	// If we did select-all, re-do it in case the user clicked during the LLM call.
	if !hadSelection {
		if err := kb.SelectAll(); err != nil {
			slog.Error("SelectAll (paste prep) failed", "mode", modeName, "error", err)
			cb.Restore()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}

	if err := kb.Paste(); err != nil {
		slog.Error("Paste failed", "mode", modeName, "error", err)
		cb.Restore()
		return
	}
	time.Sleep(50 * time.Millisecond)

	// Restore original clipboard.
	cb.Restore()

	sound.PlaySuccess()
	slog.Info(modeName+" complete", "result", result)
	fmt.Printf("[%s] Result: %q\n", modeName, result)
}

// modeFromString converts a mode name string to a mode.Mode value.
func modeFromString(name string) (mode.Mode, string) {
	switch name {
	case "translate":
		return mode.ModeTranslate, "Translation"
	case "rewrite":
		return mode.ModeRewrite, "Rewrite"
	default:
		return mode.ModeCorrect, "Correction"
	}
}

func runApp(cfg *config.Config, router *mode.Router, configPath string) {
	// Windows RegisterHotKey and GetMessageW must run on the same OS thread.
	runtime.LockOSThread()

	cb := clipboard.NewWindowsClipboard()
	kb := keyboard.NewWindowsSimulator()
	hk := hotkey.NewWindowsManager()

	// Mutex-protected cancellation context for in-progress LLM calls.
	var mu sync.Mutex
	var cancelLLM context.CancelFunc

	// Active mode state — determines what the action hotkey (Ctrl+G) does.
	// Protected by mu. Can be changed at runtime (e.g., from tray menu).
	activeMode := cfg.ActiveMode

	// setActiveMode changes the active mode at runtime.
	setActiveMode := func(modeName string) {
		mu.Lock()
		activeMode = modeName
		mu.Unlock()
		slog.Info("Active mode changed", "mode", modeName)
		fmt.Printf("Active mode: %s\n", modeName)
		sound.PlayToggle()
	}

	// Build target labels for the tray menu.
	targetLabels := cfg.TranslateTargetLabels()

	// Build template name list for the tray menu.
	templNames := make([]string, len(cfg.Prompts.RewriteTemplates))
	for i, t := range cfg.Prompts.RewriteTemplates {
		templNames[i] = t.Name
	}

	// Pointer indirection so OnExit can reference stopTray before it's assigned.
	var stopTrayFn func()

	trayCfg := tray.Config{
		IconPNG: assets.AppIcon512,
		OnModeChange: func(modeName string) {
			setActiveMode(modeName)
		},
		OnTargetSelect: func(idx int) {
			label := router.SetTranslateTarget(idx)
			setActiveMode("translate")
			slog.Info("Translation target changed", "target", label)
			fmt.Printf("Translation target: %s\n", label)
		},
		OnTemplSelect: func(idx int) {
			name := router.SetTemplate(idx)
			setActiveMode("rewrite")
			slog.Info("Rewrite template changed", "template", name)
			fmt.Printf("Rewrite template: %s\n", name)
		},
		OnSoundToggle: func(enabled bool) {
			sound.SetEnabled(enabled)
			cfg.SoundEnabled = &enabled
			if err := config.WriteDefault(configPath, cfg); err != nil {
				slog.Error("Failed to save config", "error", err)
			} else {
				slog.Info("Sound toggled", "enabled", enabled)
				fmt.Printf("Sound: %v\n", enabled)
			}
			if enabled {
				sound.PlayToggle()
			}
		},
		OnCancel: func() {
			mu.Lock()
			fn := cancelLLM
			mu.Unlock()

			if fn != nil {
				slog.Info("Cancel requested via tray — aborting LLM call")
				fn()
			}
		},
		OnExit: func() {
			hk.Stop()
			if stopTrayFn != nil {
				stopTrayFn()
			}
		},
		GetActiveMode: func() string {
			mu.Lock()
			defer mu.Unlock()
			return activeMode
		},
		GetSoundEnabled: func() bool {
			return cfg.SoundEnabled != nil && *cfg.SoundEnabled
		},
		GetIsProcessing: func() bool {
			mu.Lock()
			defer mu.Unlock()
			return cancelLLM != nil
		},
		GetTargetIdx:   router.CurrentTranslateIdx,
		GetTemplateIdx: router.CurrentTemplateIdx,
		TargetLabels:   targetLabels,
		TemplateNames:  templNames,
	}

	stopTrayFn = tray.Start(trayCfg)

	// Register main action hotkey — dispatches based on active mode.
	err := hk.Register("action", cfg.Hotkeys.Correct, func() {
		mu.Lock()
		currentMode := activeMode
		mu.Unlock()

		m, displayName := modeFromString(currentMode)
		processMode(displayName, m, cfg, router, cb, kb, &mu, &cancelLLM)
	})
	if err != nil {
		slog.Error("Failed to register action hotkey", "key", cfg.Hotkeys.Correct, "error", err)
		fmt.Fprintf(os.Stderr, "Error: failed to register hotkey %s: %v\n", cfg.Hotkeys.Correct, err)
		return
	}

	// Register optional dedicated hotkeys (only if configured).
	if cfg.Hotkeys.Translate != "" {
		err = hk.Register("translate", cfg.Hotkeys.Translate, func() {
			processMode("Translation", mode.ModeTranslate, cfg, router, cb, kb, &mu, &cancelLLM)
		})
		if err != nil {
			slog.Error("Failed to register translate hotkey", "key", cfg.Hotkeys.Translate, "error", err)
			fmt.Fprintf(os.Stderr, "Error: failed to register hotkey %s: %v\n", cfg.Hotkeys.Translate, err)
			return
		}
	}

	if cfg.Hotkeys.ToggleLanguage != "" {
		err = hk.Register("toggle_language", cfg.Hotkeys.ToggleLanguage, func() {
			label := router.ToggleTranslateTarget()
			slog.Info("Translation target toggled", "target", label)
			fmt.Printf("Translation target: %s\n", label)
			sound.PlayToggle()
		})
		if err != nil {
			slog.Error("Failed to register toggle-language hotkey", "key", cfg.Hotkeys.ToggleLanguage, "error", err)
			fmt.Fprintf(os.Stderr, "Error: failed to register hotkey %s: %v\n", cfg.Hotkeys.ToggleLanguage, err)
			return
		}
	}

	if cfg.Hotkeys.Rewrite != "" {
		err = hk.Register("rewrite", cfg.Hotkeys.Rewrite, func() {
			processMode("Rewrite", mode.ModeRewrite, cfg, router, cb, kb, &mu, &cancelLLM)
		})
		if err != nil {
			slog.Error("Failed to register rewrite hotkey", "key", cfg.Hotkeys.Rewrite, "error", err)
			fmt.Fprintf(os.Stderr, "Error: failed to register hotkey %s: %v\n", cfg.Hotkeys.Rewrite, err)
			return
		}
	}

	if cfg.Hotkeys.CycleTemplate != "" {
		err = hk.Register("cycle_template", cfg.Hotkeys.CycleTemplate, func() {
			name := router.CycleTemplate()
			slog.Info("Rewrite template cycled", "template", name)
			fmt.Printf("Rewrite template: %s\n", name)
			sound.PlayToggle()
		})
		if err != nil {
			slog.Error("Failed to register cycle-template hotkey", "key", cfg.Hotkeys.CycleTemplate, "error", err)
			fmt.Fprintf(os.Stderr, "Error: failed to register hotkey %s: %v\n", cfg.Hotkeys.CycleTemplate, err)
			return
		}
	}

	// SIGINT handler — clean shutdown.
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		fmt.Println("\nGhostType shutting down.")
		slog.Info("GhostType shutting down")
		stopTrayFn()
		hk.Stop()
	}()

	fmt.Println("GhostType is ready. Waiting for hotkey input...")
	fmt.Println("Press Ctrl+C to exit.")

	// Block on Windows message loop.
	hk.Listen()
}
