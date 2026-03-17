package gui

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// UpdateProgress reports the current state of an in-progress update.
type UpdateProgress struct {
	Phase   string  `json:"phase"` // "downloading", "verifying", "installing", "restarting", "error"
	Percent float64 `json:"percent,omitempty"`
	Error   string  `json:"error,omitempty"`
}

// ghRelease is the GitHub releases API response.
type ghReleaseInfo struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int64  `json:"size"`
	} `json:"assets"`
}

// updateAssetName returns the expected release asset filename for the
// current platform.
func updateAssetName() string {
	switch runtime.GOOS {
	case "windows":
		return "ghostspell-windows-amd64.exe"
	case "darwin":
		if runtime.GOARCH == "arm64" {
			return "ghostspell-darwin-arm64"
		}
		return "ghostspell-darwin-amd64"
	case "linux":
		return "ghostspell-linux-amd64"
	default:
		return ""
	}
}

// fetchReleaseInfo queries the GitHub releases API for the latest release.
func fetchReleaseInfo(ctx context.Context) (*ghReleaseInfo, error) {
	const apiURL = "https://api.github.com/repos/chrixbedardcad/GhostSpell/releases/latest"

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release info: %w", err)
	}
	defer resp.Body.Close()

	var rel ghReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("failed to parse release info: %w", err)
	}
	return &rel, nil
}

// downloadToFile downloads a URL to a local file with progress reporting.
func downloadToFile(ctx context.Context, url, destPath string, expectedSize int64, progressCb func(UpdateProgress)) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned HTTP %d", resp.StatusCode)
	}

	total := resp.ContentLength
	if total <= 0 && expectedSize > 0 {
		total = expectedSize
	}

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	var downloaded int64
	buf := make([]byte, 256*1024) // 256KB chunks
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := f.Write(buf[:n]); writeErr != nil {
				f.Close()
				os.Remove(destPath)
				return fmt.Errorf("write error: %w", writeErr)
			}
			downloaded += int64(n)
			if progressCb != nil && total > 0 {
				progressCb(UpdateProgress{
					Phase:   "downloading",
					Percent: float64(downloaded) / float64(total) * 100,
				})
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			f.Close()
			os.Remove(destPath)
			return fmt.Errorf("download read error: %w", readErr)
		}
	}

	if err := f.Close(); err != nil {
		os.Remove(destPath)
		return fmt.Errorf("failed to close file: %w", err)
	}

	// Verify size.
	info, err := os.Stat(destPath)
	if err != nil || info.Size() == 0 {
		os.Remove(destPath)
		return fmt.Errorf("downloaded file is empty or unreadable")
	}
	if expectedSize > 0 && info.Size() != expectedSize {
		os.Remove(destPath)
		return fmt.Errorf("size mismatch: expected %d, got %d", expectedSize, info.Size())
	}

	return nil
}

// swapBinary replaces the current binary with a new one, keeping a .bak backup.
func swapBinary(currentPath, newPath string) error {
	bakPath := currentPath + ".bak"

	// Remove any previous .bak file.
	os.Remove(bakPath)

	// Rename current → .bak (on Windows, a running .exe can be renamed).
	if err := os.Rename(currentPath, bakPath); err != nil {
		return fmt.Errorf("failed to backup current binary: %w", err)
	}

	// Move new → current.
	if err := os.Rename(newPath, currentPath); err != nil {
		// Rollback: restore .bak → current.
		os.Rename(bakPath, currentPath)
		return fmt.Errorf("failed to install new binary: %w", err)
	}

	// Preserve executable permissions on Unix.
	if runtime.GOOS != "windows" {
		os.Chmod(currentPath, 0755)
	}

	return nil
}

// launchAndExit spawns a detached process that waits then relaunches the app.
func launchAndExit(binaryPath string) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		// Wait 2 seconds for the old process to exit, then launch the new binary.
		// Use PowerShell for reliable path handling (cmd /c mangles backslashes).
		script := fmt.Sprintf(`Start-Sleep -Seconds 2; Start-Process -FilePath '%s'`, binaryPath)
		cmd = exec.Command("powershell", "-NoProfile", "-WindowStyle", "Hidden", "-Command", script)
	case "darwin":
		// On macOS .app bundles, use 'open' to launch the .app directory.
		appPath := binaryPath
		if idx := strings.Index(binaryPath, ".app/"); idx != -1 {
			appPath = binaryPath[:idx+4]
		}
		cmd = exec.Command("sh", "-c", fmt.Sprintf("sleep 2 && open '%s'", appPath))
	default:
		cmd = exec.Command("sh", "-c", fmt.Sprintf("sleep 2 && '%s'", binaryPath))
	}

	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	detachProcess(cmd)

	if err := cmd.Start(); err != nil {
		slog.Error("[updater] failed to launch relaunch process", "error", err)
	} else {
		slog.Info("[updater] relaunch scheduled", "pid", cmd.Process.Pid)
	}

	time.Sleep(300 * time.Millisecond)
	os.Exit(0)
}

// CleanupUpdateBackup removes .bak files left by a previous self-update.
// Called from main.go on startup after a successful launch.
func CleanupUpdateBackup() {
	execPath, err := os.Executable()
	if err != nil {
		return
	}
	execPath, _ = filepath.EvalSymlinks(execPath)
	bakPath := execPath + ".bak"
	if _, err := os.Stat(bakPath); err == nil {
		os.Remove(bakPath)
		slog.Info("[updater] cleaned up update backup", "path", bakPath)
	}
}
