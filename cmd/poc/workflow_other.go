//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func runLive() {
	fmt.Println("This POC requires Windows for live hotkey support.")
	fmt.Println("Use '-test' flag to run the simulated demo:")
	fmt.Println("  go run ./cmd/poc -test")
	fmt.Println()
	fmt.Println("On Windows, build and run:")
	fmt.Println("  go build -o ghosttype-poc.exe ./cmd/poc")
	fmt.Println("  ghosttype-poc.exe")
	fmt.Println()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
}
