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
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/chrixbedardcad/GhostSpell/internal/version"
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

	// Open ghostvoice.log in the app data directory (same location as ghostspell.log).
	appDir := ""
	if d, err := os.UserConfigDir(); err == nil {
		appDir = filepath.Join(d, "GhostSpell")
	} else {
		exePath, _ := os.Executable()
		appDir = filepath.Dir(exePath)
	}
	logPath := filepath.Join(appDir, "ghostvoice.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err == nil {
		defer logFile.Close()
		// Write session header with version info.
		fmt.Fprintf(logFile, "\n=== Ghost Voice Session ===\n")
		fmt.Fprintf(logFile, "time=%s version=%s os=%s/%s\n",
			time.Now().Format("2006-01-02 15:04:05"),
			version.Version, runtime.GOOS, runtime.GOARCH)
		fmt.Fprintf(logFile, "model=%s language=%s wav=%s\n", modelPath, language, wavPath)
	}

	log := func(msg string) {
		line := fmt.Sprintf("[ghostvoice] %s", msg)
		fmt.Fprintln(os.Stderr, line)
		if logFile != nil {
			fmt.Fprintf(logFile, "%s %s\n", time.Now().Format("15:04:05.000"), line)
		}
	}

	engine := ghostvoice.New(0)
	log(fmt.Sprintf("loading model: %s", modelPath))
	if err := engine.Load(modelPath); err != nil {
		log(fmt.Sprintf("ERROR loading model: %v", err))
		os.Exit(1)
	}
	defer engine.Close()
	log("model loaded")

	wavData, err := os.ReadFile(wavPath)
	if err != nil {
		log(fmt.Sprintf("ERROR reading WAV: %v", err))
		os.Exit(1)
	}
	log(fmt.Sprintf("WAV: %d bytes", len(wavData)))

	text, err := engine.Transcribe(context.Background(), wavData, language)
	if err != nil {
		log(fmt.Sprintf("ERROR transcribe: %v", err))
		os.Exit(1)
	}

	log(fmt.Sprintf("result: %q (%d chars)", text, len(text)))
	fmt.Print(text)
}
