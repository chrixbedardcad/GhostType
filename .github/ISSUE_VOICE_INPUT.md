# Voice Input: Microphone Recording + Speech-to-Text Skill

## Summary

Add voice input support to GhostSpell via a dedicated hotkey (`Ctrl+Shift+G` or configurable). When triggered, the app records audio from the system microphone, performs speech-to-text (STT), and then either inserts the transcribed text directly or pipes it through the active skill (prompt) for processing — with output handled via the existing display modes (`replace`, `append`, `popup`).

This turns GhostSpell into a **voice-driven writing and command tool**: dictate text, have it corrected/polished by the LLM, and paste the result — all without touching the keyboard.

---

## Motivation

- **Hands-free input**: Users working away from the keyboard (standing desk, accessibility needs, brainstorming) can dictate text directly into any application.
- **Voice → LLM pipeline**: Dictated speech is often rough. Piping it through a skill like "Correct" or "Polish" before insertion produces clean, polished text automatically.
- **Natural extension of existing architecture**: GhostSpell already captures input (text/screenshots), sends it to an LLM, and outputs results. Voice is simply a new **input capture method** alongside text selection and screen capture.
- **Complements Vision mode**: Vision captures what you *see*, Voice captures what you *say*. Together they cover all non-keyboard input modalities.

---

## Architecture Overview

The voice feature slots into the existing pipeline as an alternative capture method:

```
Current pipeline:
  Hotkey → captureText() → LLM → displayResult()
  Hotkey → captureScreenshot() → LLM → displayResult()  (vision mode)

New pipeline:
  Voice Hotkey → recordAudio() → speechToText() → [optional LLM skill] → displayResult()
```

### Two Operating Modes

| Mode | Flow | Use Case |
|------|------|----------|
| **Dictation** | Record → STT → Insert | Pure voice typing — transcribed text goes straight into the active field |
| **Voice + Skill** | Record → STT → Active Skill → Insert | Dictate rough thoughts, have them corrected/polished/translated by the LLM |

The mode is determined by a per-skill `voice_mode` setting or a global toggle.

---

## Phase 1: Core Audio Recording

### New Package: `audio/`

```go
// audio/recorder.go
package audio

type Recorder struct {
    sampleRate  int           // 16000 Hz (optimal for STT)
    channels    int           // 1 (mono)
    maxDuration time.Duration // configurable, default 60s
    silenceMs   int           // silence detection threshold (ms)
}

// Start begins recording from the default input device.
// Returns a channel that receives the final audio buffer when recording stops.
func (r *Recorder) Start(ctx context.Context) (<-chan AudioResult, error)

// Stop manually stops the recording.
func (r *Recorder) Stop()

type AudioResult struct {
    PCM       []byte        // raw PCM audio data
    Duration  time.Duration // recording duration
    Format    AudioFormat   // sample rate, channels, bit depth
    Err       error
}
```

### Platform-Specific Microphone Access

| Platform | Approach | File |
|----------|----------|------|
| **macOS** | AVFoundation via cgo or `portaudio` binding | `audio/recorder_darwin.go` |
| **Windows** | WASAPI via cgo or `portaudio` binding | `audio/recorder_windows.go` |
| **Linux** | PulseAudio/ALSA via `portaudio` binding | `audio/recorder_linux.go` |

**Recommended library**: [`gordonklaus/portaudio`](https://github.com/gordonklaus/portaudio) — cross-platform, battle-tested, supports all three OS targets. Alternatively, platform-native APIs via cgo for tighter integration.

### Microphone Permissions

| Platform | Requirement |
|----------|-------------|
| **macOS** | `NSMicrophoneUsageDescription` in Info.plist + runtime permission prompt via AVFoundation |
| **Windows** | Microphone privacy setting (Settings → Privacy → Microphone) |
| **Linux** | PulseAudio/PipeWire access (usually available by default) |

Add permission check at startup (similar to existing accessibility permission check):
```go
// audio/permissions.go
func HasMicrophonePermission() bool
func RequestMicrophonePermission() error
```

---

## Phase 2: Speech-to-Text Integration

### STT Provider Interface

```go
// stt/provider.go
package stt

type Provider interface {
    Transcribe(ctx context.Context, audio []byte, format AudioFormat, opts TranscribeOpts) (*TranscribeResult, error)
    Name() string
}

type TranscribeOpts struct {
    Language string // BCP-47 code, e.g. "en", "fr", "ja" — empty = auto-detect
    Prompt   string // optional context hint for better accuracy
}

type TranscribeResult struct {
    Text       string
    Language   string        // detected language
    Confidence float64       // 0.0–1.0
    Duration   time.Duration // audio duration processed
}
```

### Supported Providers

| Provider | API | File | Notes |
|----------|-----|------|-------|
| **OpenAI Whisper** | `POST /v1/audio/transcriptions` | `stt/whisper.go` | Best accuracy, supports 57 languages |
| **Local Whisper** | `whisper.cpp` subprocess or HTTP | `stt/whisper_local.go` | Offline, privacy-preserving |
| **Deepgram** | WebSocket or REST | `stt/deepgram.go` | Fast, good for real-time |
| **Google Cloud STT** | gRPC or REST | `stt/google.go` | Enterprise option |
| **System STT** | macOS Speech Framework / Windows SAPI | `stt/system.go` | No API key, offline |

**Default**: OpenAI Whisper API (most users will already have an OpenAI API key configured for LLM use).

### Local Whisper (whisper.cpp) — Model Requirements

`whisper.cpp` is the inference engine only — **model weights are not included** and must be downloaded separately. The models are in GGML format (`.bin` files) hosted on Hugging Face.

| Model | File | Size | Quality | Speed |
|-------|------|------|---------|-------|
| `tiny` | `ggml-tiny.bin` | ~75 MB | Low — good for quick tests | Fastest |
| `base` | `ggml-base.bin` | ~142 MB | Decent — works for clear speech | Fast |
| `small` | `ggml-small.bin` | ~466 MB | Good — recommended default | Moderate |
| `medium` | `ggml-medium.bin` | ~1.5 GB | Great — handles accents well | Slower |
| `large-v3` | `ggml-large-v3.bin` | ~3 GB | Best accuracy | Slowest |

**Recommended default**: `ggml-small.bin` — best trade-off between quality, size, and speed for typical desktop use.

**Model download strategy** — do NOT bundle the model in the app binary:

```
1. User enables Voice + selects "whisper-local" provider in settings
2. App checks for model at ~/.ghostspell/models/ggml-<model>.bin
3. If missing → auto-download from Hugging Face (one-time)
   Source: https://huggingface.co/ggerganov/whisper.cpp/tree/main
4. Show download progress in Settings UI (progress bar + size)
5. Cache locally, reuse on all future launches
```

**Implementation notes**:
- whisper.cpp ships a helper script `models/download-ggml-model.sh` — replicate equivalent logic in Go (HTTP GET + progress tracking)
- Store models in `~/.ghostspell/models/` (user-configurable via `VoiceConfig.ModelPath`)
- Add a "Download Model" button in Voice Settings UI that triggers the download
- Add a "Delete Model" option to free disk space
- Validate downloaded model with checksum before use

### Configuration

Add to `config/config.go`:

```go
type VoiceConfig struct {
    Enabled         bool   `json:"enabled"`
    Hotkey          string `json:"hotkey"`             // default: "Ctrl+Shift+G"
    STTProvider     string `json:"stt_provider"`       // "whisper", "whisper-local", "deepgram", "system"
    STTModel        string `json:"stt_model"`          // e.g. "whisper-1", "base", "large-v3"
    ModelPath       string `json:"model_path"`         // local model dir, default: ~/.ghostspell/models/
    Language        string `json:"language"`            // BCP-47 or empty for auto-detect
    MaxRecordingSec int    `json:"max_recording_sec"`  // default: 60
    SilenceTimeoutMs int   `json:"silence_timeout_ms"` // auto-stop after silence, default: 2000
    Mode            string `json:"mode"`               // "dictation" or "skill"
    PushToTalk      bool   `json:"push_to_talk"`       // true = hold hotkey to record, false = toggle
}
```

Add to main `Config`:
```go
type Config struct {
    // ... existing fields ...
    Voice VoiceConfig `json:"voice,omitempty"`
}
```

---

## Phase 3: Voice Processing Pipeline

### New File: `voice.go`

```go
// voice.go — voice capture and processing pipeline

func (a *App) processVoice() {
    // 1. Show indicator in "recording" state (red dot / microphone icon)
    gui.ShowIndicator(IndicatorState{
        State: "recording",
        Icon:  "🎙️",
        Label: "Listening...",
    })

    // 2. Record audio from microphone
    result := <-a.recorder.Start(ctx)

    // 3. Show indicator in "transcribing" state
    gui.ShowIndicator(IndicatorState{
        State: "processing",
        Icon:  "🎙️",
        Label: "Transcribing...",
    })

    // 4. Speech-to-text
    transcript := a.sttProvider.Transcribe(ctx, result.PCM, result.Format, opts)

    // 5. Branch based on mode
    if voiceCfg.Mode == "dictation" {
        // Direct insertion — paste transcribed text
        pasteText(transcript.Text)
    } else {
        // Skill mode — feed transcript through active skill
        llmResult := router.Process(ctx, activeSkillIdx, transcript.Text)
        handleResult(llmResult, displayMode)
    }
}
```

### Indicator States (New)

Extend the indicator to show voice-specific states:

| State | Visual | Description |
|-------|--------|-------------|
| **recording** | 🎙️ Red pulsing dot + "Listening..." | Actively capturing audio |
| **transcribing** | 🎙️ Spinner + "Transcribing..." | Sending audio to STT provider |
| **processing** | (existing) Skill icon + "Processing..." | Feeding transcript through LLM skill |

**Frontend changes** (`Indicator.tsx`):
- New `recording` state with red pulsing animation
- Audio level meter (optional) showing input volume
- Duration counter showing recording length

### Recording Controls

| Interaction | Push-to-Talk Mode | Toggle Mode |
|-------------|-------------------|-------------|
| **Press hotkey** | Start recording | Start recording |
| **Release hotkey** | Stop recording → process | — |
| **Press hotkey again** | — | Stop recording → process |
| **Press Escape** | Cancel | Cancel |
| **Silence detected** | Auto-stop after threshold | Auto-stop after threshold |
| **Max duration reached** | Auto-stop | Auto-stop |
| **Click indicator** | Cancel | Stop → process |

---

## Phase 4: Skill Integration

### Voice-Aware Skills

Add `voice` field to `PromptEntry` / `Skill` struct:

```go
type Skill struct {
    // ... existing fields ...
    Voice       bool   `json:"voice,omitempty"`        // skill supports voice input
    VoiceMode   string `json:"voice_mode,omitempty"`   // "dictation" or "skill" (override global)
}
```

### Default Voice-Enabled Skills

| Skill | Voice Mode | Behavior |
|-------|-----------|----------|
| **Correct** | skill | Dictate → correct grammar → paste |
| **Polish** | skill | Dictate → polish prose → paste |
| **Translate** | skill | Dictate → translate → paste |
| **Dictate** (NEW) | dictation | Pure transcription → paste |
| **Voice Note** (NEW) | skill | Dictate → summarize/structure as notes → popup |

### New Default Skills

```go
{
    Name:        "Dictate",
    Prompt:      "Transcribe the following speech accurately. Preserve the speaker's words exactly, only fixing obvious speech-to-text errors. Do not rephrase or summarize.",
    Icon:        "🎙️",
    DisplayMode: "replace",
    Voice:       true,
    VoiceMode:   "dictation",
},
{
    Name:        "Voice Note",
    Prompt:      "The following is a voice transcription. Clean it up into well-structured notes with bullet points. Fix grammar and remove filler words, but preserve the meaning.",
    Icon:        "📝",
    DisplayMode: "popup",
    Voice:       true,
    VoiceMode:   "skill",
},
```

---

## Phase 5: Settings UI

### New Voice Settings Section in HotkeysTab or Dedicated VoiceTab

```
┌─────────────────────────────────────────────────────┐
│  Voice Input                                         │
├─────────────────────────────────────────────────────┤
│                                                      │
│  ● Enable voice input          [toggle: ON]          │
│                                                      │
│  Voice hotkey                   [Ctrl+Shift+G]       │
│                                                      │
│  Recording mode                                      │
│    ○ Push-to-talk (hold hotkey)                      │
│    ● Toggle (press to start/stop)                    │
│                                                      │
│  Default voice mode                                  │
│    ○ Dictation (transcribe only)                     │
│    ● Voice + Skill (transcribe → process with LLM)   │
│                                                      │
│  STT Provider           [Whisper (OpenAI)     ▼]     │
│  STT Model              [whisper-1            ▼]     │
│  Language                [Auto-detect          ▼]     │
│                                                      │
│  Max recording           [60] seconds                │
│  Silence auto-stop       [2000] ms                   │
│                                                      │
│  [Test Microphone]                                   │
│                                                      │
└─────────────────────────────────────────────────────┘
```

### Tray Menu Additions

```
┌──────────────────────────┐
│  Skills                   │
│    ✏️ Correct      ●      │
│    💎 Polish              │
│    🎙️ Dictate            │  ← new voice skills
│    📝 Voice Note          │  ← new voice skills
│  ──────────────────────── │
│  🎙️ Voice Input   [ON]   │  ← new toggle
│  ──────────────────────── │
│  Models                   │
│    ...                    │
└──────────────────────────┘
```

---

## Phase 6 (Future): Advanced Voice Features

| Feature | Description |
|---------|-------------|
| **Streaming STT** | Real-time transcription shown in indicator as user speaks |
| **Voice commands** | "Hey Ghost, correct this" — wake word + command parsing |
| **Text-to-Speech output** | Read LLM results aloud (accessibility) |
| **Continuous dictation** | Long-form dictation with automatic chunking |
| **Speaker diarization** | Multi-speaker meeting notes |
| **Audio skill input** | Send raw audio directly to multimodal LLMs (Gemini, GPT-4o) |

---

## Files to Create

| File | Purpose |
|------|---------|
| `audio/recorder.go` | Recorder interface and shared logic |
| `audio/recorder_darwin.go` | macOS microphone capture |
| `audio/recorder_windows.go` | Windows microphone capture |
| `audio/recorder_linux.go` | Linux microphone capture |
| `audio/permissions.go` | Microphone permission checks |
| `audio/permissions_darwin.go` | macOS AVFoundation permission |
| `audio/permissions_windows.go` | Windows privacy API |
| `audio/permissions_linux.go` | Linux (PulseAudio check) |
| `stt/provider.go` | STT provider interface |
| `stt/whisper.go` | OpenAI Whisper API client |
| `stt/whisper_local.go` | Local whisper.cpp integration |
| `stt/system.go` | System-native STT (macOS/Windows) |
| `voice.go` | Voice capture + processing pipeline |

## Files to Modify

| File | Changes |
|------|---------|
| `config/config.go` | Add `VoiceConfig`, `Voice` field on `Skill` struct |
| `config/config.go` | Add default voice skills (Dictate, Voice Note) |
| `config/config_test.go` | Tests for voice config, migration |
| `app.go` | Register voice hotkey, wire up `processVoice()` |
| `process.go` | Integrate voice pipeline alongside text/vision |
| `mode/router.go` | Voice-aware routing (skip text capture for voice input) |
| `gui/indicator.go` | Add "recording" and "transcribing" states |
| `gui/frontend/src/windows/Indicator.tsx` | Recording UI, pulsing red dot, level meter |
| `gui/frontend/src/windows/Settings/HotkeysTab.tsx` | Voice hotkey configuration |
| `gui/frontend/src/windows/Settings/Settings.tsx` | Voice settings section or tab |
| `tray/tray.go` | Voice toggle in tray menu |
| `sound/` | Add recording start/stop sounds |
| `go.mod` | Add portaudio dependency |

---

## Acceptance Criteria

- [ ] Microphone recording works on macOS, Windows, and Linux
- [ ] Microphone permission is requested gracefully on first use
- [ ] Audio is captured at 16kHz mono PCM (optimal for STT)
- [ ] OpenAI Whisper STT integration transcribes audio accurately
- [ ] Local whisper.cpp auto-downloads the GGML model on first use (default: `ggml-small.bin`)
- [ ] Model files are cached in `~/.ghostspell/models/` and reused across sessions
- [ ] Settings UI shows model download progress and allows model selection/deletion
- [ ] Indicator shows recording state with pulsing red dot
- [ ] Push-to-talk and toggle recording modes both work
- [ ] Silence detection auto-stops recording
- [ ] Dictation mode: transcribed text is pasted directly
- [ ] Skill mode: transcribed text is processed through active skill before output
- [ ] Voice hotkey is configurable in settings
- [ ] Voice settings UI is accessible from Settings window
- [ ] Tray menu shows voice toggle
- [ ] New default skills: "Dictate" and "Voice Note"
- [ ] All existing tests continue to pass
- [ ] Cancel (Escape or re-press hotkey) stops recording and discards audio
