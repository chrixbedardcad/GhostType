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

	// Verify audio data is real (not zeros).
	var maxAbs float32
	for _, s := range pcmFloat {
		if s > maxAbs {
			maxAbs = s
		}
		if -s > maxAbs {
			maxAbs = -s
		}
	}
	slog.Info("[ghost-voice] transcribing", "samples", len(pcmFloat), "duration_sec", float64(len(pcmFloat))/16000.0, "max_amplitude", maxAbs, "wav_bytes", len(wavData))
	start := time.Now()

	text, detectedLang, err := e.be.transcribe(pcmFloat, language)

	// Check if cancelled during inference — return cancel error.
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	if err != nil {
		return "", fmt.Errorf("ghost-voice transcribe: %w", err)
	}

	slog.Info("[ghost-voice] transcription raw result",
		"elapsed", time.Since(start),
		"raw_text", text,
		"raw_len", len(text),
		"language", detectedLang,
	)
	text = strings.TrimSpace(text)
	slog.Info("[ghost-voice] transcription complete",
		"text_len", len(text),
		"text", text,
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

// wavToFloat32 parses a WAV file and returns mono 16kHz float32 samples
// normalized to [-1.0, 1.0]. Handles stereo→mono and resampling automatically.
func wavToFloat32(wav []byte) ([]float32, error) {
	if len(wav) < 44 {
		return nil, fmt.Errorf("WAV data too short (%d bytes)", len(wav))
	}
	if string(wav[0:4]) != "RIFF" || string(wav[8:12]) != "WAVE" {
		return nil, fmt.Errorf("not a valid WAV file")
	}

	// Parse fmt and data chunks.
	var channels, bitsPerSample int
	var sampleRate int
	var pcmBytes []byte
	offset := 12
	for offset < len(wav)-8 {
		chunkID := string(wav[offset : offset+4])
		chunkSize := int(binary.LittleEndian.Uint32(wav[offset+4 : offset+8]))
		if chunkID == "fmt " && chunkSize >= 16 {
			channels = int(binary.LittleEndian.Uint16(wav[offset+10 : offset+12]))
			sampleRate = int(binary.LittleEndian.Uint32(wav[offset+12 : offset+16]))
			bitsPerSample = int(binary.LittleEndian.Uint16(wav[offset+22 : offset+24]))
		} else if chunkID == "data" {
			pcmBytes = wav[offset+8:]
			if len(pcmBytes) > chunkSize {
				pcmBytes = pcmBytes[:chunkSize]
			}
		}
		offset += 8 + chunkSize
		if chunkSize%2 != 0 {
			offset++
		}
	}
	if pcmBytes == nil {
		return nil, fmt.Errorf("WAV data chunk not found")
	}
	if channels == 0 {
		channels = 1
	}
	if sampleRate == 0 {
		sampleRate = 16000
	}
	if bitsPerSample == 0 {
		bitsPerSample = 16
	}

	slog.Info("[ghost-voice] WAV format", "channels", channels, "sampleRate", sampleRate, "bitsPerSample", bitsPerSample, "dataBytes", len(pcmBytes))

	// Convert to float32 mono.
	bytesPerSample := bitsPerSample / 8
	stride := channels * bytesPerSample
	nFrames := len(pcmBytes) / stride

	monoFloat := make([]float32, nFrames)
	for i := 0; i < nFrames; i++ {
		var sum float64
		for ch := 0; ch < channels; ch++ {
			pos := i*stride + ch*bytesPerSample
			if pos+1 < len(pcmBytes) {
				s := int16(binary.LittleEndian.Uint16(pcmBytes[pos : pos+2]))
				sum += float64(s)
			}
		}
		monoFloat[i] = float32(sum / float64(channels) / float64(math.MaxInt16))
	}

	// Resample to 16kHz if needed.
	if sampleRate != 16000 {
		ratio := float64(sampleRate) / 16000.0
		outLen := int(float64(nFrames) / ratio)
		resampled := make([]float32, outLen)
		for i := 0; i < outLen; i++ {
			srcIdx := float64(i) * ratio
			idx := int(srcIdx)
			if idx >= nFrames-1 {
				idx = nFrames - 2
			}
			frac := float32(srcIdx - float64(idx))
			resampled[i] = monoFloat[idx]*(1-frac) + monoFloat[idx+1]*frac
		}
		slog.Info("[ghost-voice] resampled", "from", sampleRate, "to", 16000, "frames", nFrames, "resampled", outLen)
		return resampled, nil
	}

	return monoFloat, nil
}
