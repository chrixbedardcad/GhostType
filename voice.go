package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chrixbedardcad/GhostSpell/audio"
	"github.com/chrixbedardcad/GhostSpell/clipboard"
	"github.com/chrixbedardcad/GhostSpell/config"
	"github.com/chrixbedardcad/GhostSpell/gui"
	"github.com/chrixbedardcad/GhostSpell/keyboard"
	"github.com/chrixbedardcad/GhostSpell/mode"
	"github.com/chrixbedardcad/GhostSpell/sound"
	"github.com/chrixbedardcad/GhostSpell/stt"
)

// voiceRecording tracks whether we're currently recording.
var voiceRecording atomic.Bool
var voiceStopCh chan struct{}
var voiceStopMu sync.Mutex

// processVoice handles the voice prompt path:
// record audio → transcribe → process with LLM → paste result.
// Called from processMode when the active prompt has Voice: true.
func processVoice(
	promptName string,
	promptIdx int,
	cfg *config.Config,
	router *mode.Router,
	cb *clipboard.Clipboard,
	kb keyboard.Simulator,
	mu *sync.Mutex,
	cancelCtx context.Context,
	startAnim func(),
	stopAnim func(),
	transcriber stt.Transcriber,
) {
	// Check if already recording — second press stops recording.
	if voiceRecording.Load() {
		slog.Info("[voice] Second press — stopping recording")
		voiceStopMu.Lock()
		if voiceStopCh != nil {
			close(voiceStopCh)
			voiceStopCh = nil
		}
		voiceStopMu.Unlock()
		return
	}

	// Start recording.
	voiceRecording.Store(true)
	defer voiceRecording.Store(false)

	voiceStopMu.Lock()
	voiceStopCh = make(chan struct{})
	stopCh := voiceStopCh
	voiceStopMu.Unlock()

	slog.Info("[voice] Recording started", "prompt", promptName)
	fmt.Printf("[%s] Voice recording started...\n", promptName)
	sound.PlayStart()

	// Show recording indicator.
	gui.ShowIndicator("🎙️", "Recording...", "")

	// Record audio.
	recorder := audio.NewRecorder()
	if !recorder.Available() {
		slog.Error("[voice] No audio recording tool available")
		gui.HideIndicator()
		gui.PopIndicator("🎙️❌", "No mic tool")
		sound.PlayError()
		return
	}

	wavData, duration, err := recorder.Record(cancelCtx, stopCh)
	if err != nil {
		slog.Error("[voice] Recording failed", "error", err)
		gui.HideIndicator()
		gui.PopIndicator("🎙️❌", "Recording failed")
		sound.PlayError()
		return
	}

	// Check if cancelled.
	if cancelCtx.Err() != nil {
		slog.Info("[voice] Cancelled during recording")
		gui.HideIndicator()
		return
	}

	slog.Info("[voice] Recording complete", "duration", duration, "wav_size", len(wavData))
	sound.PlayToggle() // "stop" sound

	// Transcribe.
	gui.ShowIndicator("🎙️", "Transcribing...", "")

	if transcriber == nil {
		slog.Error("[voice] No STT provider configured")
		gui.HideIndicator()
		gui.PopIndicator("🎙️❌", "No voice model")
		sound.PlayError()
		return
	}

	// Get language preference.
	language := ""
	if cfg.Voice.Language != "" {
		language = cfg.Voice.Language
	}

	transcript, err := transcriber.Transcribe(cancelCtx, wavData, language)
	if err != nil {
		slog.Error("[voice] Transcription failed", "error", err)
		gui.HideIndicator()
		gui.PopIndicator("🎙️❌", "Transcription failed")
		sound.PlayError()
		return
	}

	transcript = strings.TrimSpace(transcript)
	if transcript == "" {
		slog.Warn("[voice] Empty transcription")
		gui.HideIndicator()
		gui.PopIndicator("🎙️", "No speech detected")
		sound.PlayError()
		return
	}

	slog.Info("[voice] Transcription complete", "text_len", len(transcript), "text", transcript)
	fmt.Printf("[%s] Transcribed: %s\n", promptName, transcript)

	// Check voice mode — dictation (paste directly) or skill (process with LLM).
	voiceMode := "skill"
	if promptIdx >= 0 && promptIdx < len(cfg.Prompts) && cfg.Prompts[promptIdx].VoiceMode != "" {
		voiceMode = cfg.Prompts[promptIdx].VoiceMode
	}

	if voiceMode == "dictation" {
		// Direct paste — no LLM processing.
		slog.Info("[voice] Dictation mode — pasting transcript directly")
		if err := cb.Write(transcript); err != nil {
			slog.Error("[voice] Clipboard write failed", "error", err)
			gui.HideIndicator()
			sound.PlayError()
			return
		}
		time.Sleep(50 * time.Millisecond)
		kb.Paste()
		time.Sleep(150 * time.Millisecond)
		gui.HideIndicator()
		sound.PlaySuccess()
		fmt.Printf("[%s] Dictation complete (%d chars)\n", promptName, len(transcript))
		return
	}

	// Skill mode — process transcript with active prompt.
	promptIcon := ""
	if promptIdx >= 0 && promptIdx < len(cfg.Prompts) {
		promptIcon = cfg.Prompts[promptIdx].Icon
	}
	modelLabel := cfg.DefaultModel
	if promptIdx >= 0 && promptIdx < len(cfg.Prompts) && cfg.Prompts[promptIdx].LLM != "" {
		modelLabel = cfg.Prompts[promptIdx].LLM
	}
	indicatorModel := modelLabel
	if me, ok := cfg.Models[modelLabel]; ok && me.Model != "" {
		indicatorModel = me.Model
	}
	gui.ShowIndicator(promptIcon, promptName, indicatorModel)

	timeout := time.Duration(router.TimeoutForPrompt(promptIdx)) * time.Millisecond
	ctx, cancel := context.WithTimeout(cancelCtx, timeout)
	defer cancel()

	resp, err := router.Process(ctx, promptIdx, transcript)
	gui.HideIndicator()

	if err != nil {
		slog.Error("[voice] LLM processing failed", "error", err)
		sound.StopWorkingLoop()

		if ctx.Err() == context.Canceled && !strings.Contains(err.Error(), "deadline exceeded") {
			gui.PopIndicator("🛑", "Cancelled")
			return
		}
		gui.PopIndicator("🎙️❌", "Processing failed")
		sound.PlayError()
		return
	}

	result := strings.TrimSpace(resp.Text)
	if result == "" {
		slog.Warn("[voice] LLM returned empty result")
		gui.HideIndicator()
		sound.PlayError()
		return
	}

	// Check display mode.
	displayMode := ""
	if promptIdx >= 0 && promptIdx < len(cfg.Prompts) {
		displayMode = cfg.Prompts[promptIdx].DisplayMode
	}

	if displayMode == "popup" {
		gui.ShowResult(result, promptName, promptIcon, modelLabel)
		sound.PlaySuccess()
		return
	}

	// Default: paste result.
	if err := cb.Write(result); err != nil {
		slog.Error("[voice] Clipboard write failed", "error", err)
		sound.PlayError()
		return
	}
	time.Sleep(50 * time.Millisecond)
	kb.Paste()
	time.Sleep(150 * time.Millisecond)
	sound.PlaySuccess()

	slog.Info("[voice] Complete", "prompt", promptName, "transcript_len", len(transcript), "result_len", len(result))
	fmt.Printf("[%s] Voice complete (%d chars → %d chars)\n", promptName, len(transcript), len(result))
}
