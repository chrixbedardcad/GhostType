//go:build windows

package gui

import "os/exec"

// detachProcess is a no-op on Windows — child processes already
// survive parent exit by default.
func detachProcess(cmd *exec.Cmd) {}
