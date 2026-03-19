package stt

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/chrixbedardcad/GhostSpell/stt/ghostvoice"
)

// GhostVoiceClient implements Transcriber using the local whisper.cpp engine.
// It manages model loading/unloading and provides the same interface as cloud STT.
type GhostVoiceClient struct {
	mu        sync.Mutex
	engine    *ghostvoice.Engine
	modelPath string
	modelName string
}

// NewGhostVoiceClient creates a local STT client.
// modelName is the friendly name (e.g. "whisper-base").
// modelsDir is the directory containing downloaded GGML models.
func NewGhostVoiceClient(modelName, modelsDir string) (*GhostVoiceClient, error) {
	if !ghostvoice.Available() {
		return nil, fmt.Errorf("ghost-voice: engine not available (build with -tags ghostvoice)")
	}

	model := findVoiceModel(modelName)
	if model == nil {
		return nil, fmt.Errorf("ghost-voice: unknown model %q", modelName)
	}

	modelPath := filepath.Join(modelsDir, model.FileName)
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("ghost-voice: model file not found: %s (download it first in Settings)", modelPath)
	}

	engine := ghostvoice.New(0) // auto-detect threads
	if err := engine.Load(modelPath); err != nil {
		engine.Close()
		return nil, err
	}

	slog.Info("[ghost-voice] client ready", "model", modelName, "path", modelPath)
	return &GhostVoiceClient{
		engine:    engine,
		modelPath: modelPath,
		modelName: modelName,
	}, nil
}

func (c *GhostVoiceClient) Name() string { return "Ghost Voice" }

func (c *GhostVoiceClient) Transcribe(ctx context.Context, wavData []byte, language string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.engine == nil {
		return "", fmt.Errorf("ghost-voice: engine not initialized")
	}

	return c.engine.Transcribe(ctx, wavData, language)
}

// Close shuts down the engine and frees resources.
func (c *GhostVoiceClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.engine != nil {
		c.engine.Close()
		c.engine = nil
	}
}

// VoiceModel describes a downloadable whisper model.
type VoiceModel struct {
	Name     string // e.g. "whisper-base"
	FileName string // e.g. "ggml-base.bin"
	URL      string // HuggingFace download URL
	Size     int64  // file size in bytes
	Tag      string // "fast", "recommended", "best", "heavy"
	Desc     string // human description
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
			Desc:     "Highest accuracy, 769M params. Needs 2GB+ RAM.",
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
