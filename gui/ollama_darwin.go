//go:build darwin

package gui

import "os/exec"

// ollamaDownloadInstallerPlatform opens the Ollama download page in the default browser.
func ollamaDownloadInstallerPlatform() error {
	return exec.Command("open", "https://ollama.com/download").Start()
}
