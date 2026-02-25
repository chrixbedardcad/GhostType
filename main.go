package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

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

	// Set up logging
	logLevel := slog.LevelInfo
	switch cfg.LogLevel {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	}

	logFile, err := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not open log file %s: %v\n", cfg.LogFile, err)
		logFile = os.Stderr
	} else {
		defer logFile.Close()
	}

	logger := slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)

	slog.Info("GhostType starting",
		"provider", cfg.LLMProvider,
		"model", cfg.Model,
		"target_window", cfg.TargetWindow,
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
	fmt.Printf("Target window: %s\n", cfg.TargetWindow)

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
	fmt.Println("GhostType is ready. Waiting for hotkey input...")
	fmt.Println("(Platform-specific hotkey hooks will be added in future builds)")
	fmt.Println("Press Ctrl+C to exit.")

	slog.Info("GhostType ready",
		"hotkey_correct", cfg.Hotkeys.Correct,
		"hotkey_translate", cfg.Hotkeys.Translate,
		"hotkey_rewrite", cfg.Hotkeys.Rewrite,
	)

	// Keep the process alive, wait for termination signal
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_ = ctx // Will be used for hotkey handlers
	_ = router // Will be used for hotkey handlers

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nGhostType shutting down.")
	slog.Info("GhostType shutting down")
}
