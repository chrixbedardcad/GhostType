//go:build !windows

package audio

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

func (r *CommandRecorder) Available() bool {
	switch runtime.GOOS {
	case "darwin":
		_, err := exec.LookPath("sox")
		return err == nil
	case "linux":
		_, err1 := exec.LookPath("arecord")
		_, err2 := exec.LookPath("parecord")
		return err1 == nil || err2 == nil
	}
	return false
}

func (r *CommandRecorder) Record(ctx context.Context, stop <-chan struct{}) ([]byte, time.Duration, error) {
	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, "ghostspell_voice.wav")
	defer os.Remove(tmpFile)

	start := time.Now()

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(ctx, "sox", "-d", "-r", "16000", "-c", "1", "-b", "16", tmpFile)
	case "linux":
		if _, err := exec.LookPath("parecord"); err == nil {
			cmd = exec.CommandContext(ctx, "parecord", "--format=s16le", "--rate=16000", "--channels=1", tmpFile)
		} else {
			cmd = exec.CommandContext(ctx, "arecord", "-f", "S16_LE", "-r", "16000", "-c", "1", tmpFile)
		}
	default:
		return nil, 0, fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	slog.Info("[audio] Recording started", "tool", cmd.Path, "file", tmpFile)
	if err := cmd.Start(); err != nil {
		return nil, 0, fmt.Errorf("failed to start recording: %w", err)
	}

	// Wait for stop signal or context cancellation.
	select {
	case <-stop:
		slog.Info("[audio] Recording stopped by user")
	case <-ctx.Done():
		slog.Info("[audio] Recording stopped by context")
	}

	// Kill the recording process.
	if cmd.Process != nil {
		cmd.Process.Kill()
	}
	cmd.Wait()

	duration := time.Since(start)

	// Read the recorded file.
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		return nil, duration, fmt.Errorf("failed to read recording: %w", err)
	}
	if len(data) < 100 {
		return nil, duration, fmt.Errorf("recording too short or empty (%d bytes)", len(data))
	}

	slog.Info("[audio] Recording complete", "duration", duration, "size", len(data))
	return data, duration, nil
}
