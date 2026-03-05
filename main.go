package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chrixbedardcad/GhostType/config"
	"github.com/chrixbedardcad/GhostType/gui"
	"github.com/chrixbedardcad/GhostType/llm"
	"github.com/chrixbedardcad/GhostType/mode"
	"github.com/chrixbedardcad/GhostType/sound"
)

// logStartupError writes a fatal startup error to a crash log file next to the
// config so that errors are visible even in windowless builds.
func logStartupError(dir, msg string, err error) {
	crashPath := filepath.Join(dir, "ghosttype_crash.log")
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

// setupLogging initialises the slog default logger from the loaded config.
// It creates the log file immediately so a version stamp is recorded as early
// as possible. Safe to call more than once (e.g. after settings change).
func setupLogging(cfg *config.Config, configDir string) {
	// Normalize to lowercase so "Debug", "DEBUG", etc. all work.
	cfg.LogLevel = strings.ToLower(strings.TrimSpace(cfg.LogLevel))

	if cfg.LogLevel != "" {
		logLevel := slog.LevelInfo
		switch cfg.LogLevel {
		case "debug":
			logLevel = slog.LevelDebug
		case "info":
			logLevel = slog.LevelInfo
		case "warn":
			logLevel = slog.LevelWarn
		case "error":
			logLevel = slog.LevelError
		}

		// Resolve log file path relative to the config file directory.
		logPath := cfg.LogFile
		if !filepath.IsAbs(logPath) {
			logPath = filepath.Join(configDir, logPath)
		}

		// Ensure the parent directory exists.
		if dir := filepath.Dir(logPath); dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not create log directory %s: %v\n", dir, err)
			}
		}

		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: could not open log file %s: %v\n", logPath, err)
			fmt.Fprintf(os.Stderr, "Logs will be written to stderr instead.\n")
			logFile = os.Stderr
		}

		logger := slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{Level: logLevel}))
		slog.SetDefault(logger)
		fmt.Printf("Logging enabled: level=%s file=%s\n", cfg.LogLevel, logPath)
	} else {
		// Disabled: set a no-op logger that discards everything.
		slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError + 1})))
		fmt.Println("Logging disabled (set log_level in config.json to enable)")
	}
}

func main() {
	fmt.Printf("GhostType v%s - AI-powered multilingual auto-correction\n", Version)
	fmt.Println("====================================================")

	// Determine config path (same directory as executable, then CWD).
	execPath, err := os.Executable()
	if err != nil {
		execPath = "."
	}
	execDir := filepath.Dir(execPath)
	configPath := filepath.Join(execDir, "config.json")

	// Fall back to current working directory.
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		configPath = "config.json"
	}

	// Resolve to absolute so log messages always show the full path.
	if absPath, err := filepath.Abs(configPath); err == nil {
		configPath = absPath
	}

	// Load configuration (without validation so the wizard can run first).
	fmt.Printf("Loading config from: %s\n", configPath)
	cfg, err := config.LoadRaw(configPath)
	if err != nil {
		logStartupError(filepath.Dir(configPath), "Failed to load config", err)
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		fmt.Println("A default config.json has been created. Please add your API key and restart.")
		os.Exit(1)
	}

	// Derive the base directory from the config file for resolving relative paths.
	configDir := filepath.Dir(configPath)

	// Set up logging early so the log file is created and the version is
	// stamped before anything else (including the settings GUI).
	setupLogging(cfg, configDir)

	slog.Info("GhostType starting",
		"version", Version,
		"default_llm", cfg.DefaultLLM,
		"llm_providers", len(cfg.LLMProviders),
	)

	// First-launch check: if no provider is configured, the wizard will
	// run on the tray Wails app (no separate app to avoid goroutine leaks).
	needsSetup := gui.NeedsSetup(cfg)
	slog.Info("First-launch check", "needs_setup", needsSetup, "providers", len(cfg.LLMProviders), "default_llm", cfg.DefaultLLM)
	fmt.Printf("First-launch check: needs_setup=%v providers=%d default_llm=%q\n", needsSetup, len(cfg.LLMProviders), cfg.DefaultLLM)

	var router *mode.Router
	if !needsSetup {
		// Validate the config — if invalid, fall back to the wizard instead of crashing.
		if err := config.Validate(cfg); err != nil {
			slog.Warn("Config invalid, will show setup wizard", "error", err)
			fmt.Fprintf(os.Stderr, "Config invalid: %v — opening setup wizard\n", err)
			needsSetup = true
		}
	}
	if !needsSetup {
		// Initialize LLM client — if it fails, fall back to the wizard.
		var client llm.Client
		if cfg.DefaultLLM != "" {
			def := cfg.LLMProviders[cfg.DefaultLLM]
			client, err = llm.NewClientFromDef(def)
		} else {
			client, err = llm.NewClient(cfg)
		}
		if err != nil {
			slog.Warn("LLM init failed, will show setup wizard", "error", err)
			fmt.Fprintf(os.Stderr, "LLM init failed: %v — opening setup wizard\n", err)
			needsSetup = true
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

	slog.Info("GhostType launching",
		"version", Version,
		"needs_setup", needsSetup,
	)

	runApp(cfg, router, configPath, needsSetup)
}

// printStatus prints provider, mode, and hotkey info to stdout.
func printStatus(cfg *config.Config, client llm.Client, router *mode.Router) {
	if len(cfg.LLMProviders) > 0 {
		fmt.Println("")
		fmt.Println("LLM Providers:")
		for label, def := range cfg.LLMProviders {
			suffix := ""
			if label == cfg.DefaultLLM {
				suffix = " (default)"
			}
			fmt.Printf("  %s: %s / %s%s\n", label, def.Provider, def.Model, suffix)
		}
		if cfg.CorrectLLM != "" {
			fmt.Printf("  correct  → %s\n", cfg.CorrectLLM)
		}
		if cfg.TranslateLLM != "" {
			fmt.Printf("  translate → %s\n", cfg.TranslateLLM)
		}
		for _, tmpl := range cfg.Prompts.RewriteTemplates {
			if tmpl.LLM != "" {
				fmt.Printf("  rewrite/%s → %s\n", tmpl.Name, tmpl.LLM)
			}
		}
	} else {
		fmt.Printf("Provider: %s\n", client.Provider())
		fmt.Printf("Model: %s\n", cfg.Model)
	}

	fmt.Println("")
	fmt.Printf("Active mode: %s\n", cfg.ActiveMode)
	targetLabels := cfg.TranslateTargetLabels()
	if idx := router.CurrentTranslateIdx(); idx < len(targetLabels) {
		fmt.Printf("Translate target: %s\n", targetLabels[idx])
	}
	fmt.Printf("Rewrite template: %s\n", router.CurrentTemplateName())
	fmt.Println("")
	fmt.Println("Hotkeys:")
	fmt.Printf("  %s - Action (%s)\n", cfg.Hotkeys.Correct, cfg.ActiveMode)
	if cfg.Hotkeys.Translate != "" {
		fmt.Printf("  %s - Translate\n", cfg.Hotkeys.Translate)
	}
	if cfg.Hotkeys.ToggleLanguage != "" {
		fmt.Printf("  %s - Toggle translation language\n", cfg.Hotkeys.ToggleLanguage)
	}
	if cfg.Hotkeys.Rewrite != "" {
		fmt.Printf("  %s - Rewrite\n", cfg.Hotkeys.Rewrite)
	}
	if cfg.Hotkeys.CycleTemplate != "" {
		fmt.Printf("  %s - Cycle rewrite template\n", cfg.Hotkeys.CycleTemplate)
	}
	fmt.Println("")
}
