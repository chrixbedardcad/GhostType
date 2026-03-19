package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/chrixbedardcad/GhostSpell/assets"
	"github.com/chrixbedardcad/GhostSpell/config"
	"github.com/chrixbedardcad/GhostSpell/gui"
	"github.com/chrixbedardcad/GhostSpell/hotkey"
	"github.com/chrixbedardcad/GhostSpell/internal/version"
	"github.com/chrixbedardcad/GhostSpell/mode"
	"github.com/chrixbedardcad/GhostSpell/sound"
	"github.com/chrixbedardcad/GhostSpell/stats"
	"github.com/chrixbedardcad/GhostSpell/tray"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

func runApp(cfg *config.Config, router *mode.Router, configPath string, needsSetup bool, initError error) {
	cb := newClipboard()
	kb := newKeyboard()
	hk := newHotkeyManager()

	// Mutex-protected cancellation context for in-progress LLM calls.
	var mu sync.Mutex
	var cancelLLM context.CancelFunc

	// Hotkey lifecycle — the manager can be stopped and replaced when the
	// user changes hotkey bindings in Settings.
	var hkMu sync.Mutex
	var registeredHotkeys config.Hotkeys
	var hotkeyReady bool
	var refreshHotkeys func()
	var reRegisterHotkeys func() // force re-register (after tray menu disrupts Carbon events)

	// setActivePrompt changes the active prompt at runtime.
	setActivePrompt := func(idx int) {
		mu.Lock()
		if idx >= 0 && idx < len(cfg.Prompts) {
			cfg.ActivePrompt = idx
		}
		mu.Unlock()
		name := ""
		if idx >= 0 && idx < len(cfg.Prompts) {
			name = cfg.Prompts[idx].Name
		}
		slog.Info("Active prompt changed", "index", idx, "name", name)
		fmt.Printf("Active prompt: %s\n", name)
		sound.PlayToggle()
		// Notify settings UI if open.
		gui.EmitConfigChanged()
	}

	// refreshTrayMenu is set after tray.Start() and called after settings save.
	var refreshTrayMenu func()

	// Create the shared Wails application used by both the tray, wizard, and settings.
	// The SettingsService is pre-registered so its JS bindings are available
	// whenever a settings or wizard window is created on this app.
	settingsSvc := gui.NewSettingsService()

	// Wire debug callbacks so the Settings GUI can control debug logging.
	if debugState != nil {
		settingsSvc.GetStatsFn = func() string { return appStats.GetSummary() }
		settingsSvc.ClearStatsFn = func() { appStats.Clear() }
		settingsSvc.RecordStatFn = func(prompt, promptIcon, provider, model, label, status, errMsg, output string, inputChars, durationMs int) {
			appStats.Record(stats.Entry{
				Timestamp:  time.Now(),
				Prompt:     prompt,
				PromptIcon: promptIcon,
				Provider:   provider,
				Model:      model,
				ModelLabel: label,
				InputChars: inputChars,
				OutputChars: len(output),
				DurationMs: int64(durationMs),
				Status:     status,
				Error:      errMsg,
				Changed:    output != "" && status == "success",
			})
		}
		settingsSvc.DebugEnableFn = debugState.Enable
		settingsSvc.DebugDisableFn = debugState.Disable
		settingsSvc.DebugEnabledFn = debugState.Enabled
		settingsSvc.DebugLogPathFn = debugState.LogPath
		settingsSvc.DebugTailFn = debugState.Tail
	}

	// Wire permission callbacks for the Settings GUI.
	settingsSvc.CheckAccessibilityFn = checkAccessibility
	settingsSvc.CheckPostEventAccessFn = checkInputMonitoring
	settingsSvc.OpenPermissionsFn = openAccessibilitySettings
	settingsSvc.OpenAccessibilityPaneFn = openAccessibilityPane
	settingsSvc.OpenInputMonitoringPaneFn = openInputMonitoringPane

	subFS, err := gui.FrontendSubFS()
	if err != nil {
		slog.Error("Failed to load frontend assets", "error", err)
		fmt.Fprintf(os.Stderr, "Error: failed to load frontend assets: %v\n", err)
		return
	}
	wailsApp := application.New(application.Options{
		Name: "GhostSpell",
		Icon: appIcon(),
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

	// scheduleHotkeyRecovery re-registers hotkeys after a tray menu interaction.
	// Only needed on macOS where the Cocoa modal event loop (NSMenu popup) can
	// break the Carbon hotkey event handler. On Windows/Linux, tray menus don't
	// interfere with hotkey listeners, so re-registering is unnecessary and
	// harmful (Windows RegisterHotKey fails if the old registration hasn't been
	// cleaned up by the exiting thread yet).
	scheduleHotkeyRecovery := func() {
		if runtime.GOOS != "darwin" {
			return
		}
		go func() {
			time.Sleep(200 * time.Millisecond)
			reRegisterHotkeys()
		}()
	}

	// initErr tracks the LLM init error so the tray can display it.
	// Cleared when the user fixes the model via settings.
	var initErr error = initError

	trayCfg := tray.Config{
		IconPNG:         assets.TrayIcon64,
		TemplateIconPNG: assets.TrayIconMacOS,
		WorkingFrames:      [][]byte{assets.TrayWorking1, assets.TrayWorking2, assets.TrayWorking3, assets.TrayWorking4},
		WorkingFramesMacOS: [][]byte{assets.TrayWorkingMacOS1, assets.TrayWorkingMacOS2, assets.TrayWorkingMacOS3, assets.TrayWorkingMacOS4},
		IsProcessing:    func() bool { return processingActive.Load() },
		OnPromptSelect: func(idx int) {
			slog.Debug("OnPromptSelect callback entered", "index", idx)
			setActivePrompt(idx)
			if router != nil {
				router.SetPrompt(idx)
			}
			scheduleHotkeyRecovery()
			slog.Debug("OnPromptSelect callback exiting", "index", idx)
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
				oldDefault := cfg.DefaultModel
				*cfg = *newCfg
				slog.Info("Config reloaded", "old_default", oldDefault, "new_default", cfg.DefaultModel, "models_count", len(cfg.Models))
				if router != nil {
					router.ResetClients()
				}
				// If we had an init error and the user fixed the config, try to init the router.
				if router == nil && initErr != nil && cfg.DefaultModel != "" {
					client, clientErr := newClientFromConfig(cfg, cfg.DefaultModel)
					if clientErr == nil {
						router = mode.NewRouter(cfg, client)
						initErr = nil
						slog.Info("Model error resolved after settings save")
					}
				}
				mu.Unlock()
				sound.SetEnabled(*cfg.SoundEnabled)
				gui.SetIndicatorPosition(cfg.IndicatorPosition)
				gui.SetIndicatorMode(cfg.IndicatorMode)
				// Show or hide idle indicator based on mode change (#211).
				if cfg.IndicatorMode == "always" {
					go gui.ShowIdle()
				} else {
					go gui.HideIndicator()
				}
				slog.Info("Live config reloaded after settings save")
				refreshTrayMenu()
				refreshHotkeys()
			})
		},
		OnModelSelect: func(label string) {
			mu.Lock()
			cfg.DefaultModel = label
			mu.Unlock()
			config.WriteDefault(configPath, cfg)
			slog.Info("Default model changed", "label", label)
			sound.PlayToggle()
			scheduleHotkeyRecovery()
		},
		OnUpdateClick: func() {
			gui.ShowUpdateWindow(settingsSvc, cfg, configPath)
		},
		GetInitError: func() string {
			if initErr != nil {
				return initErr.Error()
			}
			return ""
		},
		OnReportBug: func() {
			settingsSvc.SubmitBugReport("")
		},
		OnExit: func() {
			slog.Info("Exit requested via tray menu")
			fmt.Println("\nGhostSpell exiting (tray menu).")
			hkMu.Lock()
			hk.Stop()
			hkMu.Unlock()
			go func() {
				time.Sleep(2 * time.Second)
				os.Exit(0)
			}()
		},
		GetActivePrompt: func() int {
			mu.Lock()
			defer mu.Unlock()
			return cfg.ActivePrompt
		},
		GetPromptNames: func() []string {
			mu.Lock()
			defer mu.Unlock()
			names := make([]string, len(cfg.Prompts))
			for i, p := range cfg.Prompts {
				names[i] = p.Name
			}
			return names
		},
		GetPromptIcons: func() []string {
			mu.Lock()
			defer mu.Unlock()
			icons := make([]string, len(cfg.Prompts))
			for i, p := range cfg.Prompts {
				icons[i] = p.Icon
			}
			return icons
		},
		GetDefaultModelName: func() string {
			mu.Lock()
			defer mu.Unlock()
			if cfg.DefaultModel == "" {
				return ""
			}
			if me, ok := cfg.Models[cfg.DefaultModel]; ok {
				return cfg.DefaultModel + " (" + me.Model + ")"
			}
			return cfg.DefaultModel
		},
		GetModelLabels: func() []tray.ModelLabel {
			mu.Lock()
			defer mu.Unlock()
			var labels []tray.ModelLabel
			for label, me := range cfg.Models {
				labels = append(labels, tray.ModelLabel{
					Label:     label,
					Provider:  me.Provider,
					Model:     me.Model,
					IsDefault: label == cfg.DefaultModel,
				})
			}
			sort.Slice(labels, func(i, j int) bool { return labels[i].Label < labels[j].Label })
			return labels
		},
	}

	var dismissTrayMenu func() bool
	var trayStartAnim func()
	var trayStopAnim func()
	var setUpdateAvailable func(string)
	trayRun, stopTrayFn, dismissTrayMenu, trayStartAnim, trayStopAnim, setUpdateAvailable, refreshTrayMenu = tray.Start(trayCfg, wailsApp)

	// Background update checker — checks after 60s, then every 24h.
	go func() {
		time.Sleep(60 * time.Second)
		checkAndNotifyUpdate(setUpdateAvailable)
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			checkAndNotifyUpdate(setUpdateAvailable)
		}
	}()

	// Floating ghost overlay — lazy-initialized on first Ctrl+G press.
	// The window is NOT created at startup to avoid blocking clicks during
	// wizard, OAuth, and first-launch setup (AlwaysOnTop + IgnoreMouseEvents
	// =false on Windows was blocking the entire UI).
	gui.CreateIndicator(wailsApp)
	gui.SetIndicatorPosition(cfg.IndicatorPosition)
	gui.SetIndicatorMode(cfg.IndicatorMode)

	// Wire indicator → settings callback (#211).
	gui.OnIndicatorOpenSettings = func() {
		mu.Lock()
		c := cfg
		mu.Unlock()
		gui.ShowSettings(settingsSvc, c, configPath, func() {
			mu.Lock()
			if router != nil {
				router.ResetClients()
			}
			mu.Unlock()
		})
	}

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
	var wizardInitDone bool // guards against double-close of wizardDone
	if needsSetup {
		gui.ShowWizardOnApp(settingsSvc, wailsApp, cfg, configPath,
			func() {
				// onSaved fires for each wizard save call (SaveProviderConfig,
				// SaveModel, SetDefaultModel). Only init the LLM client once
				// the full config is ready (provider + model + default set).
				slog.Info("Wizard: config saved, checking readiness...")

				newCfg, err := config.LoadRaw(configPath)
				if err != nil {
					slog.Error("Failed to reload config after wizard", "error", err)
					return
				}
				mu.Lock()
				*cfg = *newCfg
				mu.Unlock()

				if debugState != nil {
					debugState.InitFromConfig(cfg.LogLevel)
				}

				// Wait until all 3 pieces are in place before initialising.
				if cfg.DefaultModel == "" || len(cfg.Models) == 0 || len(cfg.Providers) == 0 {
					slog.Debug("Wizard: config not fully ready yet", "default_model", cfg.DefaultModel, "models", len(cfg.Models), "providers", len(cfg.Providers))
					return
				}

				if wizardInitDone {
					return // already initialised on a previous onSaved call
				}

				client, err := newClientFromConfig(cfg, cfg.DefaultModel)
				if err != nil {
					slog.Error("Failed to init LLM client after wizard", "error", err)
					return
				}

				router = mode.NewRouter(cfg, client)
				sound.Init(*cfg.SoundEnabled)
				sound.PlayStart()

				wizardInitDone = true
				slog.Info("Wizard: init complete, unblocking hotkeys")
				close(wizardDone)
			},
			func() {
				// onCancel — user closed wizard without saving.
				if settingsSvc.Restarting {
					return // restart is handling the exit
				}
				os.Exit(1)
			},
		)
	} else {
		// No wizard needed — router is already initialised from main().
		close(wizardDone)
	}

	// doRegister registers all configured hotkeys on the given manager.
	// Used both at startup and when re-registering after a settings change.
	doRegister := func(mgr hotkey.Manager) error {
		slog.Info("Registering hotkeys", "action", cfg.Hotkeys.Action)

		// Main action hotkey — dispatches based on active prompt.
		if err := mgr.Register("action", cfg.Hotkeys.Action, func() {
			slog.Debug("Hotkey callback fired")

			// On macOS, the tray menu's NSMenu modal event loop intercepts
			// all keyboard events (CGEventPost, AX, osascript). Dismiss any
			// open tray menu before text capture so keystrokes reach the app.
			if dismissTrayMenu() {
				slog.Debug("Dismissed open tray menu before capture")
				time.Sleep(50 * time.Millisecond)
			}

			// Snapshot cfg fields and router under lock to avoid races with
			// settings save which replaces *cfg and router concurrently.
			mu.Lock()
			promptIdx := cfg.ActivePrompt
			promptName := "Prompt"
			promptLLM := ""
			if promptIdx >= 0 && promptIdx < len(cfg.Prompts) {
				promptName = cfg.Prompts[promptIdx].Name
				promptLLM = cfg.Prompts[promptIdx].LLM
			}
			localRouter := router
			slog.Info("Hotkey: config snapshot", "default_model", cfg.DefaultModel, "prompt", promptName, "prompt_llm", promptLLM, "prompt_idx", promptIdx)
			mu.Unlock()

			if localRouter == nil {
				slog.Warn("Hotkey pressed but no active model — open Settings to configure")
				sound.PlayError()
				return
			}

			processMode(promptName, promptIdx, cfg, localRouter, cb, kb, &mu, &cancelLLM, trayStartAnim, trayStopAnim)
		}); err != nil {
			slog.Error("Failed to register action hotkey", "key", cfg.Hotkeys.Action, "error", err)
			fmt.Fprintf(os.Stderr, "Error: failed to register hotkey %s: %v\n", cfg.Hotkeys.Action, err)
			return err
		}

		// Optional cycle-prompt hotkey. Non-fatal if registration fails
		// (e.g. Ctrl+Shift+T is taken by Chrome on Windows).
		if cfg.Hotkeys.CyclePrompt != "" {
			if err := mgr.Register("cycle_prompt", cfg.Hotkeys.CyclePrompt, func() {
				mu.Lock()
				localRouter := router
				mu.Unlock()
				if localRouter == nil {
					return
				}
				idx, name := localRouter.CyclePrompt()
				slog.Info("Prompt cycled", "index", idx, "name", name)
				fmt.Printf("Active prompt: %s\n", name)
				sound.PlayToggle()
				// Show a brief pop of the indicator pill with the new prompt.
				mu.Lock()
				icon := ""
				if idx >= 0 && idx < len(cfg.Prompts) {
					icon = cfg.Prompts[idx].Icon
				}
				mu.Unlock()
				gui.PopIndicator(icon, name)
			}); err != nil {
				slog.Warn("Cycle-prompt hotkey registration failed (non-fatal, may be taken by another app)", "key", cfg.Hotkeys.CyclePrompt, "error", err)
				fmt.Fprintf(os.Stderr, "Warning: cycle-prompt hotkey %s unavailable: %v\n", cfg.Hotkeys.CyclePrompt, err)
				// Don't return err — primary hotkey still works.
			}
		}

		return nil
	}

	// refreshHotkeys re-registers hotkeys when the user changes them in Settings.
	// It stops the current hotkey manager, creates a new one, and starts listening.
	refreshHotkeys = func() {
		hkMu.Lock()
		if !hotkeyReady {
			hkMu.Unlock()
			return
		}
		mu.Lock()
		newHotkeys := cfg.Hotkeys
		mu.Unlock()
		if newHotkeys == registeredHotkeys {
			hkMu.Unlock()
			return
		}

		slog.Info("Hotkey config changed, re-registering",
			"old_action", registeredHotkeys.Action,
			"new_action", newHotkeys.Action)

		old := hk
		hk = newHotkeyManager()
		newMgr := hk
		registeredHotkeys = newHotkeys
		hkMu.Unlock()

		old.Stop()

		restartHotkeyListener(newMgr, func() error {
			return doRegister(newMgr)
		})
	}

	// reRegisterHotkeys forces a hotkey re-registration even when the config
	// hasn't changed. On macOS, the Cocoa modal event loop (NSMenu popup from
	// the tray) can disrupt Carbon's RegisterEventHotKey handler, causing
	// hotkey events to stop being delivered. Re-registering restores them.
	reRegisterHotkeys = func() {
		hkMu.Lock()
		if !hotkeyReady {
			hkMu.Unlock()
			return
		}

		slog.Debug("Force re-registering hotkeys (post-menu recovery)")
		mgr := hk
		hkMu.Unlock()

		if err := mgr.Reregister(); err != nil {
			slog.Error("Failed to re-register hotkeys", "error", err)
		}
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

		// On macOS, GhostSpell needs Accessibility + Input Monitoring.
		// We check and log — but don't block. The user can check permission
		// status in Settings > General and fix it manually.
		axOK := checkAccessibility()
		postOK := checkPostEventAccess()
		slog.Info("Permission check", "accessibility", axOK, "postEventAccess", postOK)
		fmt.Printf("Accessibility: %v | PostEvent: %v\n", axOK, postOK)

		if !axOK || !postOK {
			if axOK && !postOK {
				fmt.Println("")
				fmt.Println("  WARNING: Accessibility is checked but event posting is BLOCKED.")
				fmt.Println("  Fix: toggle GhostSpell OFF then ON in Accessibility settings.")
				fmt.Println("")
				slog.Warn("Stale TCC: AXIsProcessTrusted=true but CGPreflightPostEventAccess=false")
			} else {
				fmt.Println("")
				fmt.Println("  WARNING: macOS permissions missing — hotkeys or keyboard simulation may not work.")
				fmt.Println("  Grant Accessibility + Input Monitoring in System Settings > Privacy & Security.")
				fmt.Println("")
			}
		}

		fmt.Println("GhostSpell is ready. Waiting for hotkey input...")
		fmt.Println("Press Ctrl+C to exit.")

		if err := doRegister(hk); err != nil {
			return err
		}

		hkMu.Lock()
		registeredHotkeys = cfg.Hotkeys
		hotkeyReady = true
		hkMu.Unlock()

		// Show idle indicator if always-on mode is enabled (#211).
		if cfg.IndicatorMode == "always" {
			go func() {
				time.Sleep(500 * time.Millisecond) // brief delay for event loop to stabilize
				gui.ShowIdle()
			}()
		}

		// Auto-open Settings after an update to show the "What's New" popup.
		// Don't auto-open if the wizard just ran (needsSetup was true) — the
		// wizard already showed everything the user needs.
		if !needsSetup && cfg.LastSeenVersion != version.Version && cfg.LastSeenVersion != "" {
			slog.Info("Version changed, auto-opening Settings for What's New", "last", cfg.LastSeenVersion, "current", version.Version)
			go func() {
				time.Sleep(1500 * time.Millisecond)
				gui.ShowSettings(settingsSvc, cfg, configPath, func() {
					mu.Lock()
					if router != nil {
						router.ResetClients()
					}
					mu.Unlock()
				})
			}()
		}

		return nil
	}

	// SIGINT handler — clean shutdown.
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		fmt.Println("\nGhostSpell shutting down.")
		slog.Info("GhostSpell shutting down (signal)")
		stopTrayFn()
		hkMu.Lock()
		hk.Stop()
		hkMu.Unlock()
		go func() {
			time.Sleep(2 * time.Second)
			os.Exit(0)
		}()
	}()

	// Resolve keyboard layout key codes on the main thread BEFORE the event
	// loop starts. On macOS the TIS API (TISCopyCurrentKeyboardInputSource)
	// is not thread-safe and crashes/hangs when called from a goroutine.
	initKeyboard()

	// Platform-specific main loop: controls which thread runs the Cocoa/GTK
	// event loop vs the hotkey listener. On macOS this runs app.Run() on the
	// main thread so the Carbon hotkey API doesn't deadlock.
	// On all platforms, the event loop starts BEFORE registerHotkeys so that
	// the wizard window (if needed) can render while hotkeys wait on wizardDone.
	startMainLoop(trayRun, registerHotkeys, hk)
}
