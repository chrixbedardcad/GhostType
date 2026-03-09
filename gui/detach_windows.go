//go:build windows

package gui

import (
	"os/exec"
	"syscall"
)

// detachProcess gives the child its own console on Windows.
// ghosttype.exe is built with -H windowsgui (no console), so child
// processes like PowerShell need CREATE_NEW_CONSOLE to function.
func detachProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | 0x00000010, // CREATE_NEW_CONSOLE
	}
}
