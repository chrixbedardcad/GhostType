package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/chrixbedardcad/GhostType/assets"
	"github.com/chrixbedardcad/GhostType/clipboard"
	"github.com/chrixbedardcad/GhostType/config"
	"github.com/chrixbedardcad/GhostType/gui"
	"github.com/chrixbedardcad/GhostType/internal/sysinfo"
	"github.com/chrixbedardcad/GhostType/keyboard"
	"github.com/chrixbedardcad/GhostType/llm"
	"github.com/chrixbedardcad/GhostType/mode"
	"github.com/chrixbedardcad/GhostType/sound"
	"github.com/chrixbedardcad/GhostType/tray"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// captureText detects whether the user has an active text selection. It clears
// the clipboard, copies, and checks. If text was copied the user had a selection.
// Otherwise it falls back to select-all + copy.
// Returns the captured text, whether the user had an active selection, and any error.
func captureText(
	modeName string,
	cb *clipboard.Clipboard,
	kb keyboard.Simulator,
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
	kb keyboard.Simulator,
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

	// Create cancellable context with per-provider timeout.
	timeout := time.Duration(router.TimeoutForMode(m)) * time.Millisecond
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
		// Paste error indicator so the user sees something went wrong
		// directly in their text. They can Ctrl+Z to undo.
		cb.Write("\U0001F47B\u274C") // 👻❌
		kb.Paste()
		time.Sleep(50 * time.Millisecond)
		cb.Restore()
		sound.PlayError()
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

func runApp(cfg *config.Config, router *mode.Router, configPath string, needsSetup bool) {
	cb := newClipboard()
	kb := newKeyboard()
	hk := newHotkeyManager()

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

	// Create the shared Wails application used by both the tray, wizard, and settings.
	// The SettingsService is pre-registered so its JS bindings are available
	// whenever a settings or wizard window is created on this app.
	settingsSvc := gui.NewSettingsService()
	subFS, err := gui.FrontendSubFS()
	if err != nil {
		slog.Error("Failed to load frontend assets", "error", err)
		fmt.Fprintf(os.Stderr, "Error: failed to load frontend assets: %v\n", err)
		return
	}
	wailsApp := application.New(application.Options{
		Name: "GhostType",
		Services: []application.Service{
			application.NewService(settingsSvc),
		},
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(subFS),
		},
		Mac: application.MacOptions{
			ActivationPolicy:                               application.ActivationPolicyAccessory,
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
		Windows: application.WindowsOptions{
			DisableQuitOnLastWindowClosed: true,
		},
		Linux: application.LinuxOptions{
			DisableQuitOnLastWindowClosed: true,
		},
		// Prevent auto-quit when the settings/wizard window closes; the tray keeps running.
		ShouldQuit: func() bool { return false },
	})

	// appReady is closed when the Wails/Cocoa/GTK event loop has started.
	// On macOS this is critical: the Carbon hotkey API calls dispatch_sync to the
	// main queue, which deadlocks unless the Cocoa event loop is running.
	// Using this event-based signal replaces the fragile time.Sleep approach.
	appReady := make(chan struct{})
	var appReadyOnce sync.Once
	wailsApp.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(event *application.ApplicationEvent) {
		appReadyOnce.Do(func() { close(appReady) })
	})

	// Pre-declare so closures in trayCfg can reference these before assignment.
	var stopTrayFn func()
	var trayRun func() error

	// wizardDone is closed when the wizard completes (or immediately if no wizard needed).
	// registerHotkeys blocks on this channel so hotkeys are only registered after
	// the LLM client and router are ready.
	wizardDone := make(chan struct{})

	trayCfg := tray.Config{
		IconPNG:         assets.TrayIcon64,
		TemplateIconPNG: assets.TrayIconMacOS,
		OnModeChange: func(modeName string) {
			setActiveMode(modeName)
		},
		OnTargetSelect: func(idx int) {
			if router == nil {
				return
			}
			label := router.SetTranslateTarget(idx)
			setActiveMode("translate")
			slog.Info("Translation target changed", "target", label)
			fmt.Printf("Translation target: %s\n", label)
		},
		OnTemplSelect: func(idx int) {
			if router == nil {
				return
			}
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
		OnSettings: func() {
			gui.ShowSettings(settingsSvc, cfg, configPath, func() {
				// Reload config from disk after settings save.
				newCfg, err := config.LoadRaw(configPath)
				if err != nil {
					slog.Error("Failed to reload config after settings save", "error", err)
					return
				}
				mu.Lock()
				*cfg = *newCfg
				mu.Unlock()
				if router != nil {
					router.ResetClients()
				}
				slog.Info("Live config reloaded after settings save")
			})
		},
		OnModelSelect: func(label string) {
			mu.Lock()
			cfg.DefaultLLM = label
			mu.Unlock()
			config.WriteDefault(configPath, cfg)
			slog.Info("Default model changed", "label", label)
			sound.PlayToggle()
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
		OnDebugToggle: func(enabled bool) {
			if debugState == nil {
				return
			}
			if enabled {
				logPath, err := debugState.Enable()
				if err != nil {
					fmt.Fprintf(os.Stderr, "Failed to enable debug logging: %v\n", err)
					return
				}
				logSysInfo(cfg)
				fmt.Printf("Debug logging enabled: %s\n", logPath)
			} else {
				debugState.Disable()
				fmt.Println("Debug logging disabled")
			}
		},
		OnOpenLogFile: func() {
			if debugState == nil {
				return
			}
			logPath := debugState.LogPath()
			if _, err := os.Stat(logPath); os.IsNotExist(err) {
				// No log file yet — enable debug logging first so there's something to open.
				fmt.Println("No log file yet — enabling debug logging first")
				if _, enableErr := debugState.Enable(); enableErr != nil {
					fmt.Fprintf(os.Stderr, "Failed to enable debug logging: %v\n", enableErr)
					return
				}
				logSysInfo(cfg)
			}
			gui.OpenFile(logPath)
		},
		OnCopyLog: func() {
			if debugState == nil {
				return
			}
			tail, err := debugState.Tail()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to read log: %v\n", err)
				return
			}
			info := sysinfo.Collect()
			header := fmt.Sprintf("GhostType v%s | %s %s (%s) | Locale: %s | Keyboard: %s\n---\n",
				Version, info.OS, info.OSVersion, info.Arch, info.Locale, info.KeyboardLayout)
			cb.Write(header + tail)
			fmt.Println("Log copied to clipboard (last 200 lines)")
		},
		OnExit: func() {
			slog.Info("Exit requested via tray menu")
			fmt.Println("\nGhostType exiting (tray menu).")
			hk.Stop()
			go func() {
				time.Sleep(2 * time.Second)
				os.Exit(0)
			}()
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
		GetModelLabels: func() []tray.ModelLabel {
			mu.Lock()
			defer mu.Unlock()
			var labels []tray.ModelLabel
			for label, def := range cfg.LLMProviders {
				labels = append(labels, tray.ModelLabel{
					Label:     label,
					Provider:  def.Provider,
					Model:     def.Model,
					IsDefault: label == cfg.DefaultLLM,
				})
			}
			sort.Slice(labels, func(i, j int) bool { return labels[i].Label < labels[j].Label })
			return labels
		},
		GetDebugEnabled: func() bool {
			return debugState != nil && debugState.Enabled()
		},
		GetTargetIdx: func() int {
			if router == nil {
				return 0
			}
			return router.CurrentTranslateIdx()
		},
		GetTemplateIdx: func() int {
			if router == nil {
				return 0
			}
			return router.CurrentTemplateIdx()
		},
		TargetLabels:  targetLabels,
		TemplateNames: templNames,
	}

	trayRun, stopTrayFn = tray.Start(trayCfg, wailsApp)

	// When debug auto-disables after 30min, log it.
	if debugState != nil {
		debugState.SetOnAutoStop(func() {
			fmt.Println("Debug logging auto-disabled after 30 minutes")
		})
	}

	// If first launch, show the wizard on this same Wails app (no separate app).
	// The wizard window appears alongside the tray icon. When the user saves a
	// provider, the LLM client + router are initialised and wizardDone is closed
	// to unblock hotkey registration.
	if needsSetup {
		gui.ShowWizardOnApp(settingsSvc, wailsApp, cfg, configPath,
			func() {
				// onSaved — provider was saved to disk. Initialise everything.
				slog.Info("Wizard: provider saved, initialising LLM client...")
				fmt.Println("Wizard: provider saved, initialising...")

				newCfg, err := config.LoadRaw(configPath)
				if err != nil {
					slog.Error("Failed to reload config after wizard", "error", err)
					fmt.Fprintf(os.Stderr, "Error reloading config: %v\n", err)
					return
				}
				mu.Lock()
				*cfg = *newCfg
				mu.Unlock()

				// Re-init logging in case the reload changed settings.
				if debugState != nil {
					debugState.InitFromConfig(cfg.LogLevel)
				}

				if err := config.Validate(cfg); err != nil {
					slog.Error("Config validation failed after wizard", "error", err)
					fmt.Fprintf(os.Stderr, "Config validation failed: %v\n", err)
					return
				}

				var client llm.Client
				if cfg.DefaultLLM != "" {
					def := cfg.LLMProviders[cfg.DefaultLLM]
					client, err = llm.NewClientFromDef(def)
				} else {
					client, err = llm.NewClient(cfg)
				}
				if err != nil {
					slog.Error("Failed to init LLM client after wizard", "error", err)
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					return
				}

				router = mode.NewRouter(cfg, client)
				sound.Init(*cfg.SoundEnabled)
				sound.PlayStart()

				slog.Info("Wizard: init complete, unblocking hotkeys")
				fmt.Println("Wizard: setup complete, starting GhostType...")
				close(wizardDone)
			},
			func() {
				// onCancel — user closed wizard without saving.
				os.Exit(1)
			},
		)
	} else {
		// No wizard needed — router is already initialised from main().
		close(wizardDone)
	}

	// registerHotkeys is called by startMainLoop at the right time for each
	// platform (deferred on macOS so the Cocoa event loop is running first).
	// It blocks on wizardDone so hotkeys are only registered after the LLM
	// client and router are ready.
	registerHotkeys := func() error {
		// Wait for the event loop to be ready — required on macOS where the
		// Carbon hotkey API dispatches to the main queue. If the event loop
		// never starts, proceeding would cause a deadlock (SIGTRAP), so we
		// exit with a clear error after 30s rather than crashing.
		select {
		case <-appReady:
			slog.Debug("Event loop ready (ApplicationStarted)")
		case <-time.After(30 * time.Second):
			slog.Error("ApplicationStarted event not received within 30s — cannot register hotkeys safely")
			fmt.Fprintln(os.Stderr, "Error: application event loop did not start within 30s. Exiting.")
			return fmt.Errorf("event loop did not start within 30s")
		}
		<-wizardDone

		// On macOS, verify Accessibility permission before registering hotkeys.
		// Without it, the Carbon API deadlocks (SIGTRAP) and keyboard simulation
		// silently fails.
		if !checkAccessibility() {
			slog.Error("Accessibility permission not granted — hotkeys and keyboard simulation will not work")
			fmt.Fprintln(os.Stderr, "Error: GhostType requires Accessibility permission.")
			fmt.Fprintln(os.Stderr, "Please grant access in System Settings → Privacy & Security → Accessibility.")
			// Attempt to open the Accessibility pane for the user.
			openAccessibilitySettings()
			return fmt.Errorf("accessibility permission not granted")
		}

		fmt.Println("GhostType is ready. Waiting for hotkey input...")
		fmt.Println("Press Ctrl+C to exit.")

		// Main action hotkey — dispatches based on active mode.
		if err := hk.Register("action", cfg.Hotkeys.Correct, func() {
			mu.Lock()
			currentMode := activeMode
			mu.Unlock()

			m, displayName := modeFromString(currentMode)
			processMode(displayName, m, cfg, router, cb, kb, &mu, &cancelLLM)
		}); err != nil {
			slog.Error("Failed to register action hotkey", "key", cfg.Hotkeys.Correct, "error", err)
			fmt.Fprintf(os.Stderr, "Error: failed to register hotkey %s: %v\n", cfg.Hotkeys.Correct, err)
			return err
		}

		// Optional dedicated hotkeys (only if configured).
		if cfg.Hotkeys.Translate != "" {
			if err := hk.Register("translate", cfg.Hotkeys.Translate, func() {
				processMode("Translation", mode.ModeTranslate, cfg, router, cb, kb, &mu, &cancelLLM)
			}); err != nil {
				slog.Error("Failed to register translate hotkey", "key", cfg.Hotkeys.Translate, "error", err)
				fmt.Fprintf(os.Stderr, "Error: failed to register hotkey %s: %v\n", cfg.Hotkeys.Translate, err)
				return err
			}
		}

		if cfg.Hotkeys.ToggleLanguage != "" {
			if err := hk.Register("toggle_language", cfg.Hotkeys.ToggleLanguage, func() {
				label := router.ToggleTranslateTarget()
				slog.Info("Translation target toggled", "target", label)
				fmt.Printf("Translation target: %s\n", label)
				sound.PlayToggle()
			}); err != nil {
				slog.Error("Failed to register toggle-language hotkey", "key", cfg.Hotkeys.ToggleLanguage, "error", err)
				fmt.Fprintf(os.Stderr, "Error: failed to register hotkey %s: %v\n", cfg.Hotkeys.ToggleLanguage, err)
				return err
			}
		}

		if cfg.Hotkeys.Rewrite != "" {
			if err := hk.Register("rewrite", cfg.Hotkeys.Rewrite, func() {
				processMode("Rewrite", mode.ModeRewrite, cfg, router, cb, kb, &mu, &cancelLLM)
			}); err != nil {
				slog.Error("Failed to register rewrite hotkey", "key", cfg.Hotkeys.Rewrite, "error", err)
				fmt.Fprintf(os.Stderr, "Error: failed to register hotkey %s: %v\n", cfg.Hotkeys.Rewrite, err)
				return err
			}
		}

		if cfg.Hotkeys.CycleTemplate != "" {
			if err := hk.Register("cycle_template", cfg.Hotkeys.CycleTemplate, func() {
				name := router.CycleTemplate()
				slog.Info("Rewrite template cycled", "template", name)
				fmt.Printf("Rewrite template: %s\n", name)
				sound.PlayToggle()
			}); err != nil {
				slog.Error("Failed to register cycle-template hotkey", "key", cfg.Hotkeys.CycleTemplate, "error", err)
				fmt.Fprintf(os.Stderr, "Error: failed to register hotkey %s: %v\n", cfg.Hotkeys.CycleTemplate, err)
				return err
			}
		}

		return nil
	}

	// SIGINT handler — clean shutdown.
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		fmt.Println("\nGhostType shutting down.")
		slog.Info("GhostType shutting down (signal)")
		stopTrayFn()
		hk.Stop()
		go func() {
			time.Sleep(2 * time.Second)
			os.Exit(0)
		}()
	}()

	// Platform-specific main loop: controls which thread runs the Cocoa/GTK
	// event loop vs the hotkey listener. On macOS this runs app.Run() on the
	// main thread so the Carbon hotkey API doesn't deadlock.
	// On all platforms, the event loop starts BEFORE registerHotkeys so that
	// the wizard window (if needed) can render while hotkeys wait on wizardDone.
	startMainLoop(trayRun, registerHotkeys, hk)
}
