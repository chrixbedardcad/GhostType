//go:build !windows

package main

import (
	"os"
	"syscall"
)

// isProcessRunning checks if a process with the given PID exists.
// On Unix, FindProcess always succeeds — send signal 0 to probe.
func isProcessRunning(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}
