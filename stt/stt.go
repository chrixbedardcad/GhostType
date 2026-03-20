// Package stt provides speech-to-text transcription.
// Supports local (Ghost Voice via whisper.cpp) and cloud (OpenAI Whisper API).
package stt

import (
	"context"
	"log/slog"
	"strings"
	"time"
)

// Transcriber converts audio to text.
type Transcriber interface {
	// Transcribe converts WAV audio data to text.
	// language is a BCP-47 code (e.g. "en", "fr") or empty for auto-detect.
	Transcribe(ctx context.Context, wavData []byte, language string) (string, error)

	// Name returns the provider name.
	Name() string
}

// StreamingTranscriber extends Transcriber with streaming capability.
type StreamingTranscriber interface {
	Transcriber
	SupportsStreaming() bool
}

// TranscribeStreaming runs periodic transcriptions on growing audio data.
// snapshotFn returns the current WAV snapshot. onPartial is called with each
// partial result. Returns the last successful transcription text.
func TranscribeStreaming(
	ctx context.Context,
	transcriber Transcriber,
	snapshotFn func() []byte,
	language string,
	onPartial func(text string),
	interval time.Duration,
) string {
	lastText := ""
	lastSize := 0
	minNewBytes := 16000 // ~0.5s of 16kHz mono 16-bit audio

	for {
		select {
		case <-ctx.Done():
			return lastText
		case <-time.After(interval):
		}

		if ctx.Err() != nil {
			return lastText
		}

		wav := snapshotFn()
		if len(wav) < 44 { // WAV header is 44 bytes
			continue
		}
		// Skip if insufficient new audio since last transcription.
		if len(wav)-lastSize < minNewBytes {
			continue
		}
		lastSize = len(wav)

		text, err := transcriber.Transcribe(ctx, wav, language)
		if err != nil {
			if ctx.Err() != nil {
				return lastText
			}
			slog.Debug("[stt] streaming transcription error", "error", err)
			continue
		}
		text = strings.TrimSpace(text)
		if text != "" && text != lastText {
			lastText = text
			onPartial(text)
		}
	}
}
