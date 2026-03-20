// Package ghostvoice provides local speech-to-text via embedded whisper.cpp.
// Build with -tags ghostvoice to enable. Without the tag, a stub is used.
//
// This follows the same pattern as llm/ghostai/ (embedded llama.cpp):
// - CGo bridge wraps whisper.h
// - Build script downloads whisper.cpp and compiles static libs
// - Engine loads GGML model files downloaded from HuggingFace
package ghostvoice

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"
	"time"
)

// engine is the internal interface — either cgoEngine or stubEngine.
type engine interface {
	load(modelPath string) error
	transcribe(pcmFloat []float32, language string) (text string, lang string, err error)
	abort() // request cancellation of in-progress transcription
	isLoaded() bool
	unload()
	close()
}

// Engine is the public Ghost Voice engine.
type Engine struct {
	mu     sync.Mutex
	be     engine
	closed bool
}

// New creates a new Ghost Voice engine.
// threads: number of CPU threads (0 = auto, typically 4).
func New(threads int) *Engine {
	if threads <= 0 {
		threads = 4
	}
	return &Engine{be: newEngine(threads)}
}

// Available returns true if Ghost Voice was compiled in (build with -tags ghostvoice).
func Available() bool {
	return engineAvailable()
}

// Load loads a whisper GGML model from the given path.
func (e *Engine) Load(modelPath string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return fmt.Errorf("ghost-voice: engine closed")
	}

	start := time.Now()
	slog.Info("[ghost-voice] loading model", "path", modelPath)

	if err := e.be.load(modelPath); err != nil {
		return fmt.Errorf("ghost-voice load: %w", err)
	}

	slog.Info("[ghost-voice] model loaded", "elapsed", time.Since(start))
	return nil
}

// Transcribe converts PCM audio (16-bit signed, mono, 16kHz WAV) to text.
// The wavData should include the WAV header (44 bytes) followed by PCM samples.
// language is a BCP-47 code (e.g. "en", "fr") or empty for auto-detect.
// Respects context cancellation — aborts whisper.cpp mid-inference via abort callback.
func (e *Engine) Transcribe(ctx context.Context, wavData []byte, language string) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return "", fmt.Errorf("ghost-voice: engine closed")
	}
	if !e.be.isLoaded() {
		return "", fmt.Errorf("ghost-voice: model not loaded")
	}

	// Check context before starting.
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	// Parse WAV and convert 16-bit PCM to float32 (whisper.cpp expects float).
	pcmFloat, err := wavToFloat32(wavData)
	if err != nil {
		return "", fmt.Errorf("ghost-voice: %w", err)
	}

	slog.Info("[ghost-voice] transcribing", "samples", len(pcmFloat), "duration_sec", float64(len(pcmFloat))/16000.0)
	start := time.Now()

	// Monitor context — abort whisper inference if cancelled.
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			slog.Info("[ghost-voice] context cancelled — aborting inference")
			e.be.abort()
		case <-done:
		}
	}()

	text, detectedLang, err := e.be.transcribe(pcmFloat, language)
	close(done)

	// If context was cancelled, return the cancel error regardless of whisper result.
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	if err != nil {
		return "", fmt.Errorf("ghost-voice transcribe: %w", err)
	}

	text = strings.TrimSpace(text)
	slog.Info("[ghost-voice] transcription complete",
		"elapsed", time.Since(start),
		"text_len", len(text),
		"language", detectedLang,
	)

	return text, nil
}

// IsLoaded returns true if a model is loaded.
func (e *Engine) IsLoaded() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.be.isLoaded()
}

// Unload frees the model from memory.
func (e *Engine) Unload() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.be.unload()
	slog.Info("[ghost-voice] model unloaded")
}

// Close destroys the engine.
func (e *Engine) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return
	}
	e.be.close()
	e.closed = true
	slog.Info("[ghost-voice] engine closed")
}

// wavToFloat32 parses a WAV file and returns the PCM data as float32 samples
// normalized to [-1.0, 1.0]. Expects 16-bit signed, mono, 16kHz.
func wavToFloat32(wav []byte) ([]float32, error) {
	if len(wav) < 44 {
		return nil, fmt.Errorf("WAV data too short (%d bytes)", len(wav))
	}

	// Verify RIFF header.
	if string(wav[0:4]) != "RIFF" || string(wav[8:12]) != "WAVE" {
		return nil, fmt.Errorf("not a valid WAV file")
	}

	// Find data chunk.
	dataOffset := 12
	for dataOffset < len(wav)-8 {
		chunkID := string(wav[dataOffset : dataOffset+4])
		chunkSize := int(binary.LittleEndian.Uint32(wav[dataOffset+4 : dataOffset+8]))
		if chunkID == "data" {
			dataOffset += 8
			pcmBytes := wav[dataOffset:]
			if len(pcmBytes) > chunkSize {
				pcmBytes = pcmBytes[:chunkSize]
			}

			// Convert 16-bit signed PCM to float32.
			nSamples := len(pcmBytes) / 2
			floats := make([]float32, nSamples)
			for i := 0; i < nSamples; i++ {
				sample := int16(binary.LittleEndian.Uint16(pcmBytes[i*2 : i*2+2]))
				floats[i] = float32(sample) / float32(math.MaxInt16)
			}
			return floats, nil
		}
		dataOffset += 8 + chunkSize
		if chunkSize%2 != 0 {
			dataOffset++ // padding byte
		}
	}

	return nil, fmt.Errorf("WAV data chunk not found")
}
