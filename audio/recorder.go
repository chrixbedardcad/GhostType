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

// CommandRecorder records audio using OS command-line tools.
// This is a simple cross-platform implementation that doesn't require
// CGo audio libraries. It records to a temp WAV file.
//
// Tools used:
//   - macOS: sox (brew install sox) or afrecord
//   - Windows: PowerShell + .NET AudioCapture
//   - Linux: arecord (ALSA) or parecord (PulseAudio)
type CommandRecorder struct {
	cmd     *exec.Cmd
	tmpFile string
}

// NewRecorder creates a new audio recorder.
func NewRecorder() *CommandRecorder {
	return &CommandRecorder{}
}

func (r *CommandRecorder) Available() bool {
	switch runtime.GOOS {
	case "darwin":
		_, err := exec.LookPath("sox")
		return err == nil
	case "linux":
		_, err1 := exec.LookPath("arecord")
		_, err2 := exec.LookPath("parecord")
		return err1 == nil || err2 == nil
	case "windows":
		// PowerShell is always available on Windows 10+
		return true
	}
	return false
}

func (r *CommandRecorder) Record(ctx context.Context, stop <-chan struct{}) ([]byte, time.Duration, error) {
	tmpDir := os.TempDir()
	r.tmpFile = filepath.Join(tmpDir, "ghostspell_voice.wav")
	defer os.Remove(r.tmpFile)

	start := time.Now()

	switch runtime.GOOS {
	case "darwin":
		r.cmd = exec.CommandContext(ctx, "sox", "-d", "-r", "16000", "-c", "1", "-b", "16", r.tmpFile)
	case "linux":
		if _, err := exec.LookPath("parecord"); err == nil {
			r.cmd = exec.CommandContext(ctx, "parecord", "--format=s16le", "--rate=16000", "--channels=1", r.tmpFile)
		} else {
			r.cmd = exec.CommandContext(ctx, "arecord", "-f", "S16_LE", "-r", "16000", "-c", "1", r.tmpFile)
		}
	case "windows":
		// Use ffmpeg if available, otherwise PowerShell
		if _, err := exec.LookPath("ffmpeg"); err == nil {
			r.cmd = exec.CommandContext(ctx, "ffmpeg", "-f", "dshow", "-i", "audio=default", "-ar", "16000", "-ac", "1", "-y", r.tmpFile)
		} else {
			return nil, 0, fmt.Errorf("ffmpeg not found — install ffmpeg for voice recording on Windows")
		}
	default:
		return nil, 0, fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	slog.Info("[audio] Recording started", "tool", r.cmd.Path, "file", r.tmpFile)
	if err := r.cmd.Start(); err != nil {
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
	if r.cmd.Process != nil {
		r.cmd.Process.Kill()
	}
	r.cmd.Wait() // ignore error — killed processes return non-zero

	duration := time.Since(start)

	// Read the recorded file.
	data, err := os.ReadFile(r.tmpFile)
	if err != nil {
		return nil, duration, fmt.Errorf("failed to read recording: %w", err)
	}
	if len(data) < 100 {
		return nil, duration, fmt.Errorf("recording too short or empty (%d bytes)", len(data))
	}

	slog.Info("[audio] Recording complete", "duration", duration, "size", len(data))
	return data, duration, nil
}

// Stop kills the recording process.
func (r *CommandRecorder) Stop() {
	if r.cmd != nil && r.cmd.Process != nil {
		r.cmd.Process.Kill()
	}
}
