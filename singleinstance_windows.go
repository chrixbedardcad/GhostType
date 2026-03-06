//go:build windows

package main

import (
	"os/exec"
	"strconv"
	"strings"
)

// isProcessRunning checks if a process with the given PID exists.
// On Windows, we use tasklist to check if the PID is active.
func isProcessRunning(pid int) bool {
	cmd := exec.Command("tasklist", "/FI", "PID eq "+strconv.Itoa(pid), "/NH")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	// tasklist returns "INFO: No tasks are running..." if PID not found.
	return len(out) > 0 && !strings.Contains(string(out), "No tasks")
}
