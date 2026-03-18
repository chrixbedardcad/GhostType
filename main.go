package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/chrixbedardcad/GhostSpell/config"
	"github.com/chrixbedardcad/GhostSpell/gui"
	"github.com/chrixbedardcad/GhostSpell/internal/debuglog"
	"github.com/chrixbedardcad/GhostSpell/internal/sysinfo"
	"github.com/chrixbedardcad/GhostSpell/llm"
	"github.com/chrixbedardcad/GhostSpell/mode"
	"github.com/chrixbedardcad/GhostSpell/sound"
	"github.com/chrixbedardcad/GhostSpell/stats"
)

// appDataDir returns the OS-standard directory for GhostSpell's config, logs,
// and other persistent data.
//
//	macOS:   ~/Library/Application Support/GhostSpell/
//	Windows: %APPDATA%\GhostSpell\
//	Linux:   ~/.config/GhostSpell/  (or $XDG_CONFIG_HOME/GhostSpell/)
func appDataDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("os.UserConfigDir: %w", err)
	}
	return filepath.Join(base, "GhostSpell"), nil
}

// migrateConfigFromExeDir checks whether a config.json exists next to the
// executable (the old storage location). If it does and the target path does
// not yet exist, it copies the old config to the new app data directory and
// renames the original to config.json.bak so it isn't loaded again.
func migrateConfigFromExeDir(newConfigPath string) {
	// Already have a config in the new location — nothing to migrate.
	if _, err := os.Stat(newConfigPath); err == nil {
		return
	}

	execPath, err := os.Executable()
	if err != nil {
		slog.Debug("Config migration: cannot resolve executable path", "error", err)
		return
	}
	oldPath := filepath.Join(filepath.Dir(execPath), "config.json")
	if _, err := os.Stat(oldPath); err != nil {
		return // no old config
	}

	data, err := os.ReadFile(oldPath)
	if err != nil {
		slog.Warn("Config migration: failed to read old config", "path", oldPath, "error", err)
		return
	}
	if err := os.WriteFile(newConfigPath, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to migrate config to %s: %v\n", newConfigPath, err)
		return
	}

	// Rename old file so it won't be picked up again.
	if err := os.Rename(oldPath, oldPath+".bak"); err != nil {
		slog.Warn("Config migration: failed to rename old config", "path", oldPath, "error", err)
	}
	fmt.Printf("Migrated config from %s to %s\n", oldPath, newConfigPath)
}

// logStartupError writes a fatal startup error to a crash log file next to the
// config so that errors are visible even in windowless builds.
func logStartupError(dir, msg string, err error) {
	crashPath := filepath.Join(dir, "ghostspell_crash.log")
	f, ferr := os.OpenFile(crashPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if ferr != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "time=%s level=ERROR version=%s msg=%q error=%q\n",
		time.Now().Format(time.RFC3339), Version, msg, err)
	// Also log via slog in case the logger is already set up.
	slog.Error(msg, "error", err)
}

// debugState is the global debug log state, initialised in main().
var debugState *debuglog.State

// logSysInfo writes a system information block to the log.
func logSysInfo(cfg *config.Config) {
	info := sysinfo.Collect()
	slog.Info("=== GhostSpell Debug Session ===",
		"version", Version,
		"os", info.OS,
		"os_version", info.OSVersion,
		"arch", info.Arch,
		"locale", info.Locale,
		"keyboard", info.KeyboardLayout,
		"hotkey", cfg.Hotkeys.Action,
		"default_model", cfg.DefaultModel,
		"providers", len(cfg.Providers),
		"models", len(cfg.Models),
	)
}

// recoverPanic writes a crash log if a panic occurs.
func recoverPanic(configDir string) {
	if r := recover(); r != nil {
		crashPath := filepath.Join(configDir, "ghostspell_crash.log")
		f, err := os.OpenFile(crashPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "PANIC: %v\n%s\n", r, debug.Stack())
			return
		}
		defer f.Close()
		fmt.Fprintf(f, "=== CRASH %s (v%s) ===\n%v\n%s\n\n",
			time.Now().Format(time.RFC3339), Version, r, debug.Stack())
		fmt.Fprintf(os.Stderr, "GhostSpell crashed. Details written to: %s\n", crashPath)
	}
}

func main() {
	fmt.Printf("GhostSpell v%s - AI-powered multilingual auto-correction\n", Version)
	fmt.Println("====================================================")

	// Determine the app data directory using the OS-standard location:
	//   macOS:   ~/Library/Application Support/GhostSpell/
	//   Windows: %APPDATA%\GhostSpell\
	//   Linux:   ~/.config/GhostSpell/  (XDG_CONFIG_HOME)
	appDir, err := appDataDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not determine app data directory: %v\n", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(appDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not create app directory %s: %v\n", appDir, err)
		os.Exit(1)
	}

	// Single-instance check — exit if another GhostSpell is already running.
	removeLock := acquireSingleInstance(appDir)
	defer removeLock()

	// Panic recovery — writes stack trace to crash log.
	defer recoverPanic(appDir)

	configPath := filepath.Join(appDir, "config.json")

	// Migration: if a config exists next to the executable (old behavior) but
	// not in the new app directory, move it over so existing users don't lose
	// their settings.
	migrateConfigFromExeDir(configPath)

	// Clean up .bak files left by a successful self-update.
	gui.CleanupUpdateBackup()

	// Load configuration (without validation so the wizard can run first).
	fmt.Printf("App data: %s\n", appDir)
	cfg, err := config.LoadRaw(configPath)
	if err != nil {
		// Config corrupted — try restoring from backup (created before updates).
		backupPath := configPath + ".bak"
		if bdata, berr := os.ReadFile(backupPath); berr == nil {
			fmt.Println("Config corrupted, restoring from backup...")
			os.WriteFile(configPath, bdata, 0644)
			cfg, err = config.LoadRaw(configPath)
		}
	}
	if err != nil {
		logStartupError(filepath.Dir(configPath), "Failed to load config", err)
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		fmt.Println("A default config.json has been created. Please add your API key and restart.")
		os.Exit(1)
	}

	// Check if config lost providers (e.g., empty after partial write).
	// Restore from backup if the backup has more providers.
	backupPath := configPath + ".bak"
	if len(cfg.Providers) == 0 {
		if bdata, berr := os.ReadFile(backupPath); berr == nil {
			var backupCfg config.Config
			if json.Unmarshal(bdata, &backupCfg) == nil && len(backupCfg.Providers) > 0 {
				fmt.Printf("Config has no providers but backup has %d — restoring\n", len(backupCfg.Providers))
				slog.Info("Restoring providers from config backup", "backup_providers", len(backupCfg.Providers))
				os.WriteFile(configPath, bdata, 0644)
				cfg, _ = config.LoadRaw(configPath)
			}
		}
	}

	// Derive the base directory from the config file for resolving relative paths.
	configDir := filepath.Dir(configPath)

	// Initialise the debug log system. Honours config's log_level on startup.
	debugState = debuglog.New(configDir)

	// Initialize usage stats tracker.
	appStats = stats.New(configDir)
	debugState.InitFromConfig(cfg.LogLevel)

	slog.Info("GhostSpell starting",
		"version", Version,
		"default_model", cfg.DefaultModel,
		"providers", len(cfg.Providers),
		"models", len(cfg.Models),
	)
	logSysInfo(cfg)

	// Register the OpenAI OAuth token refresh hook so the LLM layer can
	// auto-refresh expired tokens without importing the gui package.
	llm.RefreshOpenAIKeyFunc = func(refreshToken string) (string, error) {
		apiKey, _, err := gui.RefreshOpenAITokens(refreshToken)
		return apiKey, err
	}

	// First-launch check: if no provider is configured, the wizard will
	// run on the tray Wails app (no separate app to avoid goroutine leaks).
	needsSetup := config.NeedsSetup(cfg)

	// On macOS, check Accessibility and Input Monitoring permissions.
	// These are mandatory — the app cannot function without them.
	//
	// Strategy (per Apple best practices):
	// 1. Call AXIsProcessTrustedWithOptions(prompt:YES) to trigger the native
	//    macOS permission dialog. This pre-lists GhostSpell in System Settings
	//    and is the recommended approach for Accessibility.
	// 2. Validate with real CGEventCreateKeyboardEvent/CGEventTapCreate tests
	//    (not just API checks which can be stale after binary updates).
	// 3. If permissions are missing, show the wizard for guided setup.
	//
	// The #1 fix for permission-loss-after-update is proper Developer ID
	// code signing (not ad-hoc). With a stable signing identity, TCC carries
	// permissions across updates automatically.
	if runtime.GOOS == "darwin" {
		// Trigger native Accessibility prompt (fires once per launch, pre-lists app).
		requestAccessibility()

		axOK := checkAccessibility()
		imOK := checkInputMonitoring()
		slog.Info("macOS permissions", "accessibility", axOK, "inputMonitoring", imOK)
		if !axOK || !imOK {
			fmt.Printf("macOS permissions: accessibility=%v inputMonitoring=%v — opening wizard\n", axOK, imOK)
			needsSetup = true
		}
	}

	slog.Info("First-launch check", "needs_setup", needsSetup, "providers", len(cfg.Providers), "default_model", cfg.DefaultModel)
	fmt.Printf("First-launch check: needs_setup=%v providers=%d default_model=%q\n", needsSetup, len(cfg.Providers), cfg.DefaultModel)

	var router *mode.Router
	if !needsSetup {
		// Validate the config — if invalid, fall back to the wizard instead of crashing.
		if err := config.Validate(cfg); err != nil {
			slog.Warn("Config invalid, will show setup wizard", "error", err)
			fmt.Fprintf(os.Stderr, "Config invalid: %v — opening setup wizard\n", err)
			needsSetup = true
		}
	}
	var initError error
	if !needsSetup {
		// Initialize LLM client — if it fails and no providers exist, show wizard.
		// If providers exist but model fails (e.g. legacy model removed), start
		// the app with an error state instead of forcing the wizard.
		var client llm.Client
		if cfg.DefaultModel != "" {
			client, err = newClientFromConfig(cfg, cfg.DefaultModel)
		} else {
			err = fmt.Errorf("no default_model configured")
		}
		if err != nil {
			if len(cfg.Providers) > 0 {
				// Providers configured but model failed — don't force wizard.
				slog.Warn("LLM init failed (model error), starting without active model", "error", err)
				fmt.Fprintf(os.Stderr, "LLM init failed: %v — open Settings to fix\n", err)
				initError = err
				sound.Init(*cfg.SoundEnabled)
			} else {
				slog.Warn("LLM init failed, will show setup wizard", "error", err)
				fmt.Fprintf(os.Stderr, "LLM init failed: %v — opening setup wizard\n", err)
				needsSetup = true
			}
		} else {
			router = mode.NewRouter(cfg, client)
			sound.Init(*cfg.SoundEnabled)
			sound.PlayStart()
			printStatus(cfg, client, router)
		}
	}
	if needsSetup {
		fmt.Println("Setup needed — wizard will open...")
	}

	slog.Info("GhostSpell launching",
		"version", Version,
		"needs_setup", needsSetup,
	)

	runApp(cfg, router, configPath, needsSetup, initError)
}

// newClientFromConfig builds an LLM client by merging a named model entry
// with its provider credentials from the config.
func newClientFromConfig(cfg *config.Config, label string) (llm.Client, error) {
	model, ok := cfg.Models[label]
	if !ok {
		return nil, fmt.Errorf("model %q not found", label)
	}
	prov, ok := cfg.Providers[model.Provider]
	if !ok {
		return nil, fmt.Errorf("provider %q not configured", model.Provider)
	}
	def := config.LLMProviderDef{
		Provider:     model.Provider,
		APIKey:       prov.APIKey,
		Model:        model.Model,
		APIEndpoint:  prov.APIEndpoint,
		RefreshToken: prov.RefreshToken,
		KeepAlive:    prov.KeepAlive,
		TimeoutMs:    prov.TimeoutMs,
		MaxTokens:    model.MaxTokens,
	}
	if model.TimeoutMs > 0 {
		def.TimeoutMs = model.TimeoutMs
	}
	return llm.NewClientFromDef(def)
}

// printStatus prints provider, mode, and hotkey info to stdout.
func printStatus(cfg *config.Config, _ llm.Client, _ *mode.Router) {
	fmt.Println("")
	fmt.Println("Models:")
	for label, me := range cfg.Models {
		suffix := ""
		if label == cfg.DefaultModel {
			suffix = " (default)"
		}
		fmt.Printf("  %s: %s / %s%s\n", label, me.Provider, me.Model, suffix)
	}
	for _, p := range cfg.Prompts {
		if p.LLM != "" {
			fmt.Printf("  prompt/%s → %s\n", p.Name, p.LLM)
		}
	}

	fmt.Println("")
	activePromptName := "Correct"
	if cfg.ActivePrompt >= 0 && cfg.ActivePrompt < len(cfg.Prompts) {
		activePromptName = cfg.Prompts[cfg.ActivePrompt].Name
	}
	fmt.Printf("Active prompt: %s\n", activePromptName)
	fmt.Println("")
	fmt.Println("Prompts:")
	for i, p := range cfg.Prompts {
		marker := "  "
		if i == cfg.ActivePrompt {
			marker = "* "
		}
		fmt.Printf("  %s%s\n", marker, p.Name)
	}
	fmt.Println("")
	fmt.Println("Hotkeys:")
	fmt.Printf("  %s - Action (%s)\n", cfg.Hotkeys.Action, activePromptName)
	if cfg.Hotkeys.CyclePrompt != "" {
		fmt.Printf("  %s - Cycle prompt\n", cfg.Hotkeys.CyclePrompt)
	}
	fmt.Println("")
}
