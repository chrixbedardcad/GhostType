//go:build windows

package gui

import (
	"os/exec"
	"syscall"
)

// ollamaDownloadInstallerPlatform opens the Ollama download page in the default browser.
func ollamaDownloadInstallerPlatform() error {
	cmd := exec.Command("cmd", "/c", "start", "", "https://ollama.com/download")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Start()
}
