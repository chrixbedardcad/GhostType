//go:build windows

package gui

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// ollamaDownloadInstallerPlatform downloads OllamaSetup.exe to %TEMP% and launches it.
func ollamaDownloadInstallerPlatform() error {
	const url = "https://ollama.com/download/OllamaSetup.exe"

	tmpDir := os.TempDir()
	dest := filepath.Join(tmpDir, "OllamaSetup.exe")

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned HTTP %d", resp.StatusCode)
	}

	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return fmt.Errorf("write file: %w", err)
	}
	f.Close()

	// Launch the installer via cmd /c start so the user sees the standard setup wizard.
	cmd := exec.Command("cmd", "/c", "start", "", dest)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch installer: %w", err)
	}
	return nil
}
