//go:build ghostvoice

// ghostvoice is a helper binary for speech-to-text transcription.
// It links whisper.cpp via CGo in its own process, avoiding ggml symbol
// collision with Ghost-AI (llama.cpp) in the main GhostSpell binary.
//
// Usage: ghostvoice transcribe --model <path> [--language <code>] <wav-file>
// Output: transcribed text on stdout (empty if no speech detected)
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/chrixbedardcad/GhostSpell/stt/ghostvoice"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: ghostvoice transcribe --model <path> [--language <code>] <wav-file>\n")
		os.Exit(2)
	}

	cmd := os.Args[1]
	if cmd != "transcribe" {
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		os.Exit(2)
	}

	var modelPath, language, wavPath string
	args := os.Args[2:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--model":
			if i+1 < len(args) {
				modelPath = args[i+1]
				i++
			}
		case "--language":
			if i+1 < len(args) {
				language = args[i+1]
				i++
			}
		default:
			if !strings.HasPrefix(args[i], "--") {
				wavPath = args[i]
			}
		}
	}

	if modelPath == "" || wavPath == "" {
		fmt.Fprintf(os.Stderr, "Error: --model and wav file path are required\n")
		os.Exit(2)
	}

	engine := ghostvoice.New(0)
	if err := engine.Load(modelPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading model: %v\n", err)
		os.Exit(1)
	}
	defer engine.Close()

	wavData, err := os.ReadFile(wavPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading WAV: %v\n", err)
		os.Exit(1)
	}

	text, err := engine.Transcribe(context.Background(), wavData, language)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(text)
}
