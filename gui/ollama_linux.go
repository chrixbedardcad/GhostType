//go:build linux

package gui

import (
	"fmt"
	"os/exec"
)

// ollamaDownloadInstallerPlatform installs Ollama on Linux via the official install script.
func ollamaDownloadInstallerPlatform() error {
	cmd := exec.Command("bash", "-c", "curl -fsSL https://ollama.com/install.sh | sh")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("install failed: %s", string(out))
	}
	return nil
}
