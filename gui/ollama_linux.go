//go:build linux

package gui

import "os/exec"

// ollamaDownloadInstallerPlatform opens the Ollama download page in the default browser.
func ollamaDownloadInstallerPlatform() error {
	return exec.Command("xdg-open", "https://ollama.com/download").Start()
}
