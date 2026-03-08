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

	"github.com/chrixbedardcad/GhostType/assets"
	"github.com/chrixbedardcad/GhostType/clipboard"
	"github.com/chrixbedardcad/GhostType/config"
	"github.com/chrixbedardcad/GhostType/gui"
	"github.com/chrixbedardcad/GhostType/hotkey"
	"github.com/chrixbedardcad/GhostType/keyboard"
	"github.com/chrixbedardcad/GhostType/llm"
	"github.com/chrixbedardcad/GhostType/mode"
	"github.com/chrixbedardcad/GhostType/sound"
	"github.com/chrixbedardcad/GhostType/tray"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// captureMethod indicates which strategy was used to capture text.
// This determines the paste strategy in processMode.
type captureMethod int

const (
	captureViaAXAPI      captureMethod = iota // AX API read — paste via AX write
	captureViaCGEvent                         // CGEventPost + clipboard — paste via CGEventPost
	captureViaAXKeystroke                     // AXUIElementPostKeyboardEvent + clipboard — paste via AX keystroke
	captureViaScript                          // osascript System Events + clipboard — paste via osascript
)

// captureText reads text from the focused UI element.
// Strategy:
//  1. Try macOS Accessibility API (kAXSelectedTextAttribute / kAXValueAttribute)
//     — instant, no clipboard pollution, no keyboard simulation needed.
//  2. Fall back to CGEventPost Cmd+C / Cmd+A+Cmd+C clipboard approach.
//  3. Fall back to AXUIElementPostKeyboardEvent Cmd+A+Cmd+C (for Chrome/browsers).
//  4. Fall back to osascript / System Events (true last resort).
//
// Returns the captured text, whether the user had an active selection,
// the capture method used (for paste strategy), and any error.
func captureText(
	promptName string,
	cb *clipboard.Clipboard,
	kb keyboard.Simulator,
) (text string, hadSelection bool, method captureMethod, err error) {
	// Wait for the user to release hotkey modifier keys (e.g. Ctrl from Ctrl+G).
	// On macOS, CGEventPost at kCGHIDEventTap merges with hardware state —
	// if Ctrl is still held, our Cmd+A/C/V become Ctrl+Cmd+A/C/V which apps ignore.
	kb.WaitForModifierRelease()

	// Log the frontmost app for diagnostics.
	if appName := kb.FrontAppName(); appName != "" {
		slog.Debug("captureText: frontmost app", "app", appName)
	}

	// --- Strategy 1: Accessibility API (macOS) ---
	// Try reading selected text directly — no keyboard simulation, no clipboard.
	if selected := kb.ReadSelectedText(); selected != "" {
		slog.Info("captureText: got selection via Accessibility API", "len", len(selected))
		return selected, true, captureViaAXAPI, nil
	}
	// No selection — try reading all text from the focused element.
	if allText := kb.ReadAllText(); allText != "" {
		slog.Info("captureText: got all text via Accessibility API", "len", len(allText))
		return allText, false, captureViaAXAPI, nil
	}
	slog.Debug("captureText: Accessibility API returned no text, falling back to clipboard")

	// --- Strategy 2: Clipboard (Cmd+C / Cmd+A+Cmd+C) ---
	// Clear clipboard so we can detect whether Ctrl+C actually grabbed something.
	slog.Debug("captureText: clearing clipboard...")
	if err := cb.Clear(); err != nil {
		return "", false, captureViaCGEvent, fmt.Errorf("clear clipboard: %w", err)
	}
	time.Sleep(50 * time.Millisecond)

	// Try copying the current selection (if any).
	slog.Debug("captureText: sending Copy keystroke...")
	if err := kb.Copy(); err != nil {
		return "", false, captureViaCGEvent, fmt.Errorf("copy: %w", err)
	}
	time.Sleep(100 * time.Millisecond)

	slog.Debug("captureText: reading clipboard...")
	text, err = cb.Read()
	if err != nil {
		return "", false, captureViaCGEvent, fmt.Errorf("read clipboard: %w", err)
	}
	slog.Debug("captureText: clipboard read", "len", len(text), "empty", text == "")

	if text != "" {
		slog.Info("Selection detected", "prompt", promptName, "len", len(text))
		return text, true, captureViaCGEvent, nil
	}

	// No selection — fall back to select-all + copy.
	slog.Debug("No selection detected, falling back to select-all", "prompt", promptName)
	if err := kb.SelectAll(); err != nil {
		return "", false, captureViaCGEvent, fmt.Errorf("select all: %w", err)
	}
	time.Sleep(50 * time.Millisecond)

	slog.Debug("captureText: sending Copy after select-all...")
	if err := kb.Copy(); err != nil {
		return "", false, captureViaCGEvent, fmt.Errorf("copy after select-all: %w", err)
	}
	time.Sleep(100 * time.Millisecond)

	slog.Debug("captureText: reading clipboard after select-all...")
	text, err = cb.Read()
	if err != nil {
		return "", false, captureViaCGEvent, fmt.Errorf("read clipboard after select-all: %w", err)
	}
	slog.Debug("captureText: final clipboard read", "len", len(text))

	if text != "" {
		return text, false, captureViaCGEvent, nil
	}

	// --- Strategy 3: AXUIElementPostKeyboardEvent (macOS, for Chrome/browsers) ---
	// CGEventPost keystrokes didn't reach the app (common with Chrome's
	// multi-process sandbox). Try AXUIElementPostKeyboardEvent which routes
	// through the Accessibility framework instead of the HID event tap.
	slog.Debug("captureText: CGEventPost clipboard empty, trying AX keystroke fallback")

	if err := cb.Clear(); err != nil {
		return "", false, captureViaAXKeystroke, fmt.Errorf("clear clipboard (AX fallback): %w", err)
	}
	time.Sleep(50 * time.Millisecond)

	if err := kb.SelectAllAX(); err != nil {
		slog.Debug("captureText: SelectAllAX not available", "error", err)
		return "", false, captureViaCGEvent, nil
	}
	time.Sleep(100 * time.Millisecond)

	if err := kb.CopyAX(); err != nil {
		slog.Debug("captureText: CopyAX not available", "error", err)
		return "", false, captureViaCGEvent, nil
	}
	time.Sleep(150 * time.Millisecond)

	text, err = cb.Read()
	if err != nil {
		return "", false, captureViaAXKeystroke, fmt.Errorf("read clipboard (AX fallback): %w", err)
	}
	if text != "" {
		slog.Info("captureText: got text via AX keystroke fallback", "len", len(text))
		return text, false, captureViaAXKeystroke, nil
	}
	slog.Debug("captureText: AX keystroke fallback also returned empty")

	// --- Strategy 4: osascript / System Events (macOS, true last resort) ---
	// Both CGEventPost and AXUIElementPostKeyboardEvent failed. Try osascript
	// which routes through System Events — a completely different mechanism.
	slog.Debug("captureText: trying osascript fallback")

	if err := cb.Clear(); err != nil {
		return "", false, captureViaScript, fmt.Errorf("clear clipboard (script fallback): %w", err)
	}
	time.Sleep(50 * time.Millisecond)

	if err := kb.SelectAllScript(); err != nil {
		slog.Debug("captureText: SelectAllScript not available", "error", err)
		return "", false, captureViaScript, nil
	}
	time.Sleep(150 * time.Millisecond)

	if err := kb.CopyScript(); err != nil {
		slog.Debug("captureText: CopyScript not available", "error", err)
		return "", false, captureViaScript, nil
	}
	time.Sleep(200 * time.Millisecond)

	text, err = cb.Read()
	if err != nil {
		return "", false, captureViaScript, fmt.Errorf("read clipboard (script fallback): %w", err)
	}
	if text != "" {
		slog.Info("captureText: got text via osascript fallback", "len", len(text))
	} else {
		slog.Debug("captureText: osascript fallback also returned empty")
	}

	return text, false, captureViaScript, nil
}

// processingGuard prevents concurrent processMode execution.
// macOS Carbon RegisterEventHotKey can send multiple Keydown events for a
// single press, launching parallel goroutines that corrupt clipboard state.
// TryLock rejects duplicates; Unlock happens when the current call finishes.
var processingGuard sync.Mutex

// processMode captures text from the active window, sends it through the LLM
// with the given prompt, and pastes the result back.
func processMode(
	promptName string,
	promptIdx int,
	cfg *config.Config,
	router *mode.Router,
	cb *clipboard.Clipboard,
	kb keyboard.Simulator,
	mu *sync.Mutex,
	cancelLLM *context.CancelFunc,
) {
	if !processingGuard.TryLock() {
		slog.Debug("Hotkey ignored (already processing)")
		return
	}
	defer processingGuard.Unlock()
	slog.Info(promptName + " triggered")
	slog.Debug("Playing working sound...")
	sound.PlayWorking()

	// Save original clipboard.
	slog.Debug("Saving clipboard...")
	if err := cb.Save(); err != nil {
		slog.Error("Failed to save clipboard", "prompt", promptName, "error", err)
		return
	}
	slog.Debug("Clipboard saved")

	// Capture text — detects existing selection or falls back to select-all.
	slog.Debug("Capturing text...")
	text, hadSelection, capMethod, err := captureText(promptName, cb, kb)
	if err != nil {
		slog.Error("Failed to capture text", "prompt", promptName, "error", err)
		cb.Restore()
		return
	}
	if text == "" {
		slog.Warn("Nothing to process (empty text)", "prompt", promptName)
		cb.Restore()
		return
	}

	slog.Info("Captured text", "prompt", promptName, "len", len(text), "selection", hadSelection, "method", capMethod, "text", text)
	fmt.Printf("[%s] Captured: %q\n", promptName, text)

	// Create cancellable context with per-provider timeout.
	timeout := time.Duration(router.TimeoutForPrompt(promptIdx)) * time.Millisecond
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
	slog.Debug("Sending to LLM...", "prompt", promptName, "text_len", len(text))
	result, err := router.Process(ctx, promptIdx, text)
	if err != nil {
		slog.Error("LLM processing failed", "prompt", promptName, "error", err)
		// Paste error indicator so the user sees something went wrong
		// directly in their text. They can Ctrl+Z to undo.
		cb.Write("\U0001F47B\u274C") // 👻❌
		kb.Paste()
		time.Sleep(300 * time.Millisecond)
		cb.Restore()
		sound.PlayError()
		return
	}

	// --- Write result back ---
	written := false

	// Strategy 1: AX API write — for apps where we read via AX API (Cocoa apps).
	// Bypasses clipboard and CGEventPost entirely.
	if capMethod == captureViaAXAPI {
		if hadSelection {
			written = kb.WriteSelectedText(result)
			if written {
				slog.Info("Wrote result via AX API (selected text)", "prompt", promptName)
			} else {
				slog.Debug("AX WriteSelectedText failed, falling back to clipboard paste", "prompt", promptName)
			}
		} else {
			written = kb.WriteAllText(result)
			if written {
				slog.Info("Wrote result via AX API (all text)", "prompt", promptName)
			} else {
				slog.Debug("AX WriteAllText failed, falling back to clipboard paste", "prompt", promptName)
			}
		}
	}

	// Strategy 2 & 3: Clipboard-based paste.
	if !written {
		if err := cb.Write(result); err != nil {
			slog.Error("Failed to write result to clipboard", "prompt", promptName, "error", err)
			cb.Restore()
			return
		}
		slog.Debug("Result written to clipboard", "prompt", promptName, "result_len", len(result))
		time.Sleep(50 * time.Millisecond)

		switch capMethod {
		case captureViaAXKeystroke:
			// Strategy 3: AX keystroke paste.
			if !hadSelection {
				if err := kb.SelectAllAX(); err != nil {
					slog.Error("SelectAllAX (paste prep) failed", "prompt", promptName, "error", err)
				}
				time.Sleep(50 * time.Millisecond)
			}
			if err := kb.PasteAX(); err != nil {
				slog.Error("PasteAX failed", "prompt", promptName, "error", err)
				cb.Restore()
				return
			}
		case captureViaScript:
			// Strategy 4: osascript paste.
			if !hadSelection {
				if err := kb.SelectAllScript(); err != nil {
					slog.Error("SelectAllScript (paste prep) failed", "prompt", promptName, "error", err)
				}
				time.Sleep(100 * time.Millisecond)
			}
			if err := kb.PasteScript(); err != nil {
				slog.Error("PasteScript failed", "prompt", promptName, "error", err)
				cb.Restore()
				return
			}
		default:
			// Strategy 2: CGEventPost paste — for native apps.
			if !hadSelection {
				if err := kb.SelectAll(); err != nil {
					slog.Error("SelectAll (paste prep) failed", "prompt", promptName, "error", err)
					cb.Restore()
					return
				}
				time.Sleep(50 * time.Millisecond)
			}
			if err := kb.Paste(); err != nil {
				slog.Error("Paste failed", "prompt", promptName, "error", err)
				cb.Restore()
				return
			}
		}
		// Wait for the target app to process the paste event.
		time.Sleep(300 * time.Millisecond)
	}

	// Restore original clipboard.
	cb.Restore()

	sound.PlaySuccess()
	slog.Info(promptName+" complete", "result", result)
	fmt.Printf("[%s] Result: %q\n", promptName, result)
}


func runApp(cfg *config.Config, router *mode.Router, configPath string, needsSetup bool) {
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
	}

	// Create the shared Wails application used by both the tray, wizard, and settings.
	// The SettingsService is pre-registered so its JS bindings are available
	// whenever a settings or wizard window is created on this app.
	settingsSvc := gui.NewSettingsService()

	// Wire debug callbacks so the Settings GUI can control debug logging.
	if debugState != nil {
		settingsSvc.DebugEnableFn = debugState.Enable
		settingsSvc.DebugDisableFn = debugState.Disable
		settingsSvc.DebugEnabledFn = debugState.Enabled
		settingsSvc.DebugLogPathFn = debugState.LogPath
		settingsSvc.DebugTailFn = debugState.Tail
	}

	// Wire permission callbacks for the Settings GUI.
	settingsSvc.CheckAccessibilityFn = checkAccessibility
	settingsSvc.CheckPostEventAccessFn = checkPostEventAccess
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
		Name: "GhostType",
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

	trayCfg := tray.Config{
		IconPNG:         assets.TrayIcon64,
		TemplateIconPNG: assets.TrayIconMacOS,
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
				*cfg = *newCfg
				mu.Unlock()
				if router != nil {
					router.ResetClients()
				}
				slog.Info("Live config reloaded after settings save")
				refreshHotkeys()
			})
		},
		OnModelSelect: func(label string) {
			mu.Lock()
			cfg.DefaultLLM = label
			mu.Unlock()
			config.WriteDefault(configPath, cfg)
			slog.Info("Default model changed", "label", label)
			sound.PlayToggle()
			scheduleHotkeyRecovery()
		},
		OnExit: func() {
			slog.Info("Exit requested via tray menu")
			fmt.Println("\nGhostType exiting (tray menu).")
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
	}

	var dismissTrayMenu func()
	trayRun, stopTrayFn, dismissTrayMenu = tray.Start(trayCfg, wailsApp)

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
			dismissTrayMenu()
			time.Sleep(50 * time.Millisecond)

			mu.Lock()
			promptIdx := cfg.ActivePrompt
			mu.Unlock()

			promptName := "Prompt"
			if promptIdx >= 0 && promptIdx < len(cfg.Prompts) {
				promptName = cfg.Prompts[promptIdx].Name
			}
			processMode(promptName, promptIdx, cfg, router, cb, kb, &mu, &cancelLLM)
		}); err != nil {
			slog.Error("Failed to register action hotkey", "key", cfg.Hotkeys.Action, "error", err)
			fmt.Fprintf(os.Stderr, "Error: failed to register hotkey %s: %v\n", cfg.Hotkeys.Action, err)
			return err
		}

		// Optional cycle-prompt hotkey.
		if cfg.Hotkeys.CyclePrompt != "" {
			if err := mgr.Register("cycle_prompt", cfg.Hotkeys.CyclePrompt, func() {
				if router == nil {
					return
				}
				idx, name := router.CyclePrompt()
				slog.Info("Prompt cycled", "index", idx, "name", name)
				fmt.Printf("Active prompt: %s\n", name)
				sound.PlayToggle()
			}); err != nil {
				slog.Error("Failed to register cycle-prompt hotkey", "key", cfg.Hotkeys.CyclePrompt, "error", err)
				fmt.Fprintf(os.Stderr, "Error: failed to register hotkey %s: %v\n", cfg.Hotkeys.CyclePrompt, err)
				return err
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

		// On macOS, GhostType needs Accessibility + Input Monitoring.
		// We check and log — but don't block. The user can check permission
		// status in Settings > General and fix it manually.
		axOK := checkAccessibility()
		postOK := checkPostEventAccess()
		slog.Info("Permission check", "accessibility", axOK, "postEventAccess", postOK)
		fmt.Printf("Accessibility: %v | PostEvent: %v\n", axOK, postOK)

		if !axOK || !postOK {
			fmt.Println("")
			if axOK && !postOK {
				fmt.Println("  WARNING: Accessibility is checked but event posting is BLOCKED.")
				fmt.Println("  Fix: toggle GhostType OFF then ON in Accessibility settings.")
				slog.Warn("Stale TCC: AXIsProcessTrusted=true but CGPreflightPostEventAccess=false")
			} else {
				fmt.Println("  WARNING: macOS permissions missing — hotkeys or keyboard simulation may not work.")
			}
			fmt.Println("  Grant Accessibility + Input Monitoring in System Settings > Privacy & Security.")
			fmt.Println("  Check permission status in GhostType Settings > General.")
			fmt.Println("")
		}

		fmt.Println("GhostType is ready. Waiting for hotkey input...")
		fmt.Println("Press Ctrl+C to exit.")

		if err := doRegister(hk); err != nil {
			return err
		}

		hkMu.Lock()
		registeredHotkeys = cfg.Hotkeys
		hotkeyReady = true
		hkMu.Unlock()

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
