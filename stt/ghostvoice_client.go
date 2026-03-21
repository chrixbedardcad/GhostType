package stt

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// GhostVoiceClient implements Transcriber by spawning the ghostvoice helper process.
// Each transcription spawns a fresh process — no ggml symbol collision with Ghost-AI.
type GhostVoiceClient struct {
	modelPath  string
	modelName  string
	helperPath string
}

// NewGhostVoiceClient creates a local STT client.
// modelName is the friendly name (e.g. "whisper-base").
// modelsDir is the directory containing downloaded GGML models.
func NewGhostVoiceClient(modelName, modelsDir string) (*GhostVoiceClient, error) {
	model := findVoiceModel(modelName)
	if model == nil {
		return nil, fmt.Errorf("ghost-voice: unknown model %q", modelName)
	}

	modelPath := filepath.Join(modelsDir, model.FileName)
	fi, err := os.Stat(modelPath)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("ghost-voice: model file not found: %s (download it first in Settings)", modelPath)
	}
	if err != nil {
		return nil, fmt.Errorf("ghost-voice: cannot stat model file: %w", err)
	}
	fileSizeMB := fi.Size() / (1024 * 1024)
	expectedMB := model.Size / (1024 * 1024)
	slog.Info("[ghost-voice] model file", "path", modelPath, "size_mb", fileSizeMB, "expected_mb", expectedMB)
	if fileSizeMB < expectedMB/2 {
		return nil, fmt.Errorf("ghost-voice: model file appears truncated (%dMB, expected ~%dMB) — re-download in Settings", fileSizeMB, expectedMB)
	}

	// Find the ghostvoice helper binary next to the main executable.
	helperPath, err := findHelper()
	if err != nil {
		return nil, err
	}

	slog.Info("[ghost-voice] client ready (subprocess mode)", "model", modelName, "helper", helperPath)
	return &GhostVoiceClient{
		modelPath:  modelPath,
		modelName:  modelName,
		helperPath: helperPath,
	}, nil
}

func (c *GhostVoiceClient) Name() string             { return "Ghost Voice" }
func (c *GhostVoiceClient) SupportsStreaming() bool { return false }
func (c *GhostVoiceClient) Close()                  {}

func (c *GhostVoiceClient) Transcribe(ctx context.Context, wavData []byte, language string) (string, error) {
	// Write WAV to temp file.
	tmp, err := os.CreateTemp("", "ghostvoice-*.wav")
	if err != nil {
		return "", fmt.Errorf("ghost-voice: temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(wavData); err != nil {
		tmp.Close()
		return "", fmt.Errorf("ghost-voice: write temp: %w", err)
	}
	tmp.Close()

	// Build command.
	args := []string{"transcribe", "--model", c.modelPath}
	if language != "" {
		args = append(args, "--language", language)
	}
	args = append(args, tmpPath)

	slog.Info("[ghost-voice] spawning helper", "helper", c.helperPath, "model", c.modelName, "wav_bytes", len(wavData))

	// Spawn helper — context cancellation kills the process instantly.
	cmd := exec.CommandContext(ctx, c.helperPath, args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr

	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return "", fmt.Errorf("ghost-voice: %s", errMsg)
		}
		return "", fmt.Errorf("ghost-voice: %w", err)
	}

	text := strings.TrimSpace(string(output))
	slog.Info("[ghost-voice] transcription complete", "text_len", len(text))
	return text, nil
}

// findHelper locates the ghostvoice helper binary.
func findHelper() (string, error) {
	name := "ghostvoice"
	if runtime.GOOS == "windows" {
		name = "ghostvoice.exe"
	}

	// Look next to the main executable.
	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exe)
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// Look in PATH.
	path, err := exec.LookPath(name)
	if err == nil {
		return path, nil
	}

	return "", fmt.Errorf("ghost-voice: helper binary %q not found (build it with: go build -tags ghostvoice -o %s ./cmd/ghostvoice/)", name, name)
}

// VoiceModel describes a downloadable whisper model.
type VoiceModel struct {
	Name     string
	FileName string
	URL      string
	Size     int64
	Tag      string
	Desc     string
}

// AvailableVoiceModels returns the curated list of downloadable whisper models.
func AvailableVoiceModels() []VoiceModel {
	return []VoiceModel{
		{
			Name:     "whisper-tiny",
			FileName: "ggml-tiny.bin",
			URL:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-tiny.bin",
			Size:     75_000_000,
			Tag:      "fast",
			Desc:     "Fastest, English-focused. 39M params. Good for quick dictation.",
		},
		{
			Name:     "whisper-base",
			FileName: "ggml-base.bin",
			URL:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.bin",
			Size:     142_000_000,
			Tag:      "recommended",
			Desc:     "Good balance of speed and accuracy. 74M params. Supports 99 languages.",
		},
		{
			Name:     "whisper-small",
			FileName: "ggml-small.bin",
			URL:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.bin",
			Size:     466_000_000,
			Tag:      "best",
			Desc:     "High accuracy, 244M params. Great for multilingual use.",
		},
		{
			Name:     "whisper-medium",
			FileName: "ggml-medium.bin",
			URL:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-medium.bin",
			Size:     1_500_000_000,
			Tag:      "heavy",
			Desc:     "High accuracy, 769M params. Needs 2GB+ RAM.",
		},
		{
			Name:     "whisper-large-v3-turbo",
			FileName: "ggml-large-v3-turbo.bin",
			URL:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-large-v3-turbo.bin",
			Size:     1_600_000_000,
			Tag:      "best",
			Desc:     "Best accuracy + speed. Turbo variant of large-v3. 809M params.",
		},
		{
			Name:     "whisper-large-v3",
			FileName: "ggml-large-v3.bin",
			URL:      "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-large-v3.bin",
			Size:     3_100_000_000,
			Tag:      "heavy",
			Desc:     "Maximum accuracy, 1.55B params. Needs 4GB+ RAM. Slowest.",
		},
	}
}

func findVoiceModel(name string) *VoiceModel {
	for _, m := range AvailableVoiceModels() {
		if m.Name == name {
			return &m
		}
	}
	return nil
}
