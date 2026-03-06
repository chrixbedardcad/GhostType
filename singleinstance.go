package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const lockFileName = ".ghosttype.lock"

// acquireSingleInstance ensures only one GhostType process runs at a time.
// It writes the current PID to a lock file. If a lock file already exists,
// it checks whether the PID inside is still alive. If alive, it exits.
// If stale (process no longer running), it reclaims the lock.
// Returns a cleanup function that removes the lock file on shutdown.
func acquireSingleInstance(appDir string) func() {
	lockPath := filepath.Join(appDir, lockFileName)

	// Check if lock file exists.
	data, err := os.ReadFile(lockPath)
	if err == nil {
		// Lock file exists — check if the PID is still alive.
		pidStr := strings.TrimSpace(string(data))
		if pid, err := strconv.Atoi(pidStr); err == nil && pid > 0 && pid != os.Getpid() {
			if isProcessRunning(pid) {
				fmt.Fprintf(os.Stderr, "GhostType is already running (PID %d).\n", pid)
				fmt.Fprintln(os.Stderr, "Look for the GhostType icon in your system tray.")
				os.Exit(0)
			}
		}
		// Stale lock — process is gone. Remove and reclaim.
	}

	// Write our PID.
	pidData := []byte(strconv.Itoa(os.Getpid()))
	if err := os.WriteFile(lockPath, pidData, 0644); err != nil {
		// Non-fatal — continue without single-instance protection.
		fmt.Fprintf(os.Stderr, "Warning: could not create lock file: %v\n", err)
		return func() {}
	}

	return func() {
		os.Remove(lockPath)
	}
}
