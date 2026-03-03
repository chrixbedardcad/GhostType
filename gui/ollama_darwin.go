//go:build darwin

package gui

import (
	"fmt"
	"os/exec"
)

// ollamaDownloadInstallerPlatform installs Ollama on macOS via brew (fallback: curl script).
func ollamaDownloadInstallerPlatform() error {
	// Try Homebrew first.
	if _, err := exec.LookPath("brew"); err == nil {
		cmd := exec.Command("brew", "install", "ollama")
		if out, err := cmd.CombinedOutput(); err != nil {
			guiLog("[GUI] brew install ollama failed: %s", string(out))
		} else {
			return nil
		}
	}

	// Fallback: official install script.
	cmd := exec.Command("bash", "-c", "curl -fsSL https://ollama.com/install.sh | sh")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("install failed: %s", string(out))
	}
	return nil
}
