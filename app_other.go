//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/chrixbedardcad/GhostType/config"
	"github.com/chrixbedardcad/GhostType/mode"
)

func runApp(cfg *config.Config, router *mode.Router, configPath string) {
	fmt.Println("GhostType requires Windows for hotkey support.")
	fmt.Println("On Windows, build and run:")
	fmt.Println("  go build -o ghosttype.exe .")
	fmt.Println("  ghosttype.exe")
	fmt.Println()
	fmt.Println("Press Ctrl+C to exit.")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nGhostType shutting down.")
}
