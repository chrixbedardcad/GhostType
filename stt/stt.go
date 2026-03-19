// Package stt provides speech-to-text transcription.
// Supports local (Ghost Voice via whisper.cpp) and cloud (OpenAI Whisper API).
package stt

import "context"

// Transcriber converts audio to text.
type Transcriber interface {
	// Transcribe converts WAV audio data to text.
	// language is a BCP-47 code (e.g. "en", "fr") or empty for auto-detect.
	Transcribe(ctx context.Context, wavData []byte, language string) (string, error)

	// Name returns the provider name.
	Name() string
}
