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
		"provider", cfg.LLMProvider,
		"model", cfg.Model,
	)

	// Initialize LLM client
	client, err := llm.NewClient(cfg)
	if err != nil {
		slog.Error("Failed to initialize LLM client", "error", err)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Provider: %s\n", client.Provider())
	fmt.Printf("Model: %s\n", cfg.Model)

	// Initialize mode router
	router := mode.NewRouter(cfg, client)

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
	fmt.Printf("  %s - Cancel\n", cfg.Hotkeys.Cancel)
	fmt.Println("")

	slog.Info("GhostType ready",
		"active_mode", cfg.ActiveMode,
		"hotkey_action", cfg.Hotkeys.Correct,
	)

	runApp(cfg, router)
}
