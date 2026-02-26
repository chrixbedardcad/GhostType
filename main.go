package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/chrixbedardcad/GhostType/config"
	"github.com/chrixbedardcad/GhostType/llm"
	"github.com/chrixbedardcad/GhostType/mode"
)

func main() {
	fmt.Printf("GhostType v%s - AI-powered multilingual auto-correction\n", Version)
	fmt.Println("====================================================")

	// Determine config path (same directory as executable)
	execPath, err := os.Executable()
	if err != nil {
		execPath = "."
	}
	configPath := filepath.Join(filepath.Dir(execPath), "config.json")

	// Also check current working directory
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		configPath = "config.json"
	}

	// Load configuration
	fmt.Printf("Loading config from: %s\n", configPath)
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		fmt.Println("A default config.json has been created. Please add your API key and restart.")
		os.Exit(1)
	}

	// Set up logging. Empty log_level disables all logging.
	if cfg.LogLevel != "" {
		logLevel := slog.LevelInfo
		switch cfg.LogLevel {
		case "debug":
			logLevel = slog.LevelDebug
		case "warn":
			logLevel = slog.LevelWarn
		case "error":
			logLevel = slog.LevelError
		}

		// Resolve log file path relative to the executable directory.
		logPath := cfg.LogFile
		if !filepath.IsAbs(logPath) {
			logPath = filepath.Join(filepath.Dir(execPath), logPath)
		}

		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not open log file %s: %v\n", logPath, err)
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
		fmt.Println("Logging disabled (log_level is empty)")
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
	fmt.Printf("Current translate target: %s\n", router.CurrentTranslateTarget())
	fmt.Printf("Current rewrite template: %s\n", router.CurrentTemplateName())
	fmt.Println("")
	fmt.Println("Hotkeys:")
	fmt.Printf("  %s - Correct spelling/grammar\n", cfg.Hotkeys.Correct)
	fmt.Printf("  %s - Toggle translation language\n", cfg.Hotkeys.ToggleLanguage)
	fmt.Printf("  %s - Translate\n", cfg.Hotkeys.Translate)
	fmt.Printf("  %s - Cycle rewrite template\n", cfg.Hotkeys.CycleTemplate)
	fmt.Printf("  %s - Rewrite\n", cfg.Hotkeys.Rewrite)
	fmt.Printf("  %s - Cancel\n", cfg.Hotkeys.Cancel)
	fmt.Println("")

	slog.Info("GhostType ready",
		"hotkey_correct", cfg.Hotkeys.Correct,
		"hotkey_translate", cfg.Hotkeys.Translate,
		"hotkey_rewrite", cfg.Hotkeys.Rewrite,
	)

	runApp(cfg, router)
}
