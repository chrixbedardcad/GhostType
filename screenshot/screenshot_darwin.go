//go:build darwin

package screenshot

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// CaptureActiveWindow captures the frontmost window as PNG bytes on macOS.
// Uses the built-in screencapture CLI which works on all macOS versions
// without deprecated CGWindowListCreateImage or kUTTypePNG APIs.
func CaptureActiveWindow() ([]byte, error) {
	// Create a temp file for the screenshot.
	tmp := filepath.Join(os.TempDir(), "ghostspell_screenshot.png")
	defer os.Remove(tmp)

	// -w = interactive window selection (captures front window when non-interactive)
	// -o = no shadow
	// -x = no sound
	// -l <wid> would be ideal but requires window ID; -w captures the frontmost.
	cmd := exec.Command("screencapture", "-x", "-o", "-w", tmp)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("screencapture failed: %w", err)
	}

	data, err := os.ReadFile(tmp)
	if err != nil {
		return nil, fmt.Errorf("failed to read screenshot: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("screenshot file is empty")
	}

	return data, nil
}
