//go:build !windows

package gui

import (
	"os/exec"
	"syscall"
)

// detachProcess sets up the command to run in a new session so it
// survives the parent process exiting.
func detachProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
