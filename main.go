package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/chrixbedardcad/GhostType/config"
	"github.com/chrixbedardcad/GhostType/llm"
	"github.com/chrixbedardcad/GhostType/mode"
	"github.com/chrixbedardcad/GhostType/sound"
)

func main() {
	fmt.Printf("GhostType v%s - AI-powered multilingual auto-correction\n", Version)
	fmt.Println("====================================================")

	// Determine config path (same directory as executable, then CWD).
	execPath, err := os.Executable()
	if err != nil {
		execPath = "."
	}
	configPath := filepath.Join(filepath.Dir(execPath), "config.json")

	// Fall back to current working directory.
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		configPath = "config.json"
	}

	// Resolve to absolute so log messages always show the full path.
	if absPath, err := filepath.Abs(configPath); err == nil {
		configPath = absPath
	}

	// Load configuration.
	fmt.Printf("Loading config from: %s\n", configPath)
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		fmt.Println("A default config.json has been created. Please add your API key and restart.")
		os.Exit(1)
	}

	// Derive the base directory from the config file for resolving relative paths.
	configDir := filepath.Dir(configPath)

	// Set up logging. Empty log_level disables all logging.
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
		} else {
			defer logFile.Close()
		}

		logger := slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{Level: logLevel}))
		slog.SetDefault(logger)
		fmt.Printf("Logging enabled: level=%s file=%s\n", cfg.LogLevel, logPath)
	} else {
		// Disabled: set a no-op logger that discards everything.
		slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError + 1})))
		fmt.Println("Logging disabled (set log_level in config.json to enable)")
	}

	slog.Info("GhostType starting",
		"version", Version,
		"default_llm", cfg.DefaultLLM,
		"llm_providers", len(cfg.LLMProviders),
	)

	// Initialize LLM client
	var client llm.Client
	if cfg.DefaultLLM != "" {
		def := cfg.LLMProviders[cfg.DefaultLLM]
		client, err = llm.NewClientFromDef(def)
	} else {
		client, err = llm.NewClient(cfg)
	}
	if err != nil {
		slog.Error("Failed to initialize LLM client", "error", err)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

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

	// Initialize mode router
	router := mode.NewRouter(cfg, client)

	// Initialize sound system and play startup sound.
	sound.Init(*cfg.SoundEnabled)
	sound.PlayStart()

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

	slog.Info("GhostType ready",
		"version", Version,
		"active_mode", cfg.ActiveMode,
		"hotkey_action", cfg.Hotkeys.Correct,
	)

	runApp(cfg, router, configPath)
}
