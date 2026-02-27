<p align="center">
  <img src="GhostType_logo.png" alt="GhostType Logo" width="300">
</p>

# GhostType

**AI-powered multilingual auto-correction, translation, and creative rewriting for virtual world chat.**

GhostType is a lightweight background service that hooks into your chat application (primarily Firestorm Second Life viewer) and provides real-time spelling correction, language translation, and creative text rewriting — powered by your choice of LLM provider.

Type in French, hit **Ctrl+G**, get it corrected. Switch to English, hit **Ctrl+G**, corrected too. Want to translate or rewrite instead? Change the active mode in the system tray (or `config.json`) — **Ctrl+G** always does whatever mode is active. Undo with **Ctrl+Z** or cancel from the tray menu. That simple.

---

## Download

[![GitHub Release](https://img.shields.io/github/v/release/chrixbedardcad/GhostType?style=for-the-badge&logo=github)](https://github.com/chrixbedardcad/GhostType/releases/latest)
[![CI](https://img.shields.io/github/actions/workflow/status/chrixbedardcad/GhostType/ci.yml?branch=main&style=for-the-badge&label=CI)](https://github.com/chrixbedardcad/GhostType/actions/workflows/ci.yml)

| Platform | Download |
|----------|----------|
| **Windows** (recommended) | [ghosttype-windows-amd64.exe](https://github.com/chrixbedardcad/GhostType/releases/latest/download/ghosttype-windows-amd64.exe) |
| **Windows** (no console) | [ghosttype-windows-amd64-windowless.exe](https://github.com/chrixbedardcad/GhostType/releases/latest/download/ghosttype-windows-amd64-windowless.exe) |
| **Linux** | [ghosttype-linux-amd64](https://github.com/chrixbedardcad/GhostType/releases/latest/download/ghosttype-linux-amd64) |
| **macOS** (Intel) | [ghosttype-darwin-amd64](https://github.com/chrixbedardcad/GhostType/releases/latest/download/ghosttype-darwin-amd64) |
| **macOS** (Apple Silicon) | [ghosttype-darwin-arm64](https://github.com/chrixbedardcad/GhostType/releases/latest/download/ghosttype-darwin-arm64) |

> **Note**: GhostType currently only runs on Windows. Linux and macOS binaries are provided for future compatibility — they will print "Windows only" until platform support is added.

---

## Table of Contents

- [Download](#download)
- [Features](#features)
- [Quick Start](#quick-start)
- [Hotkeys](#hotkeys)
- [Configuration](#configuration)
- [Supported Providers](#supported-providers)
- [Custom Rewrite Templates](#custom-rewrite-templates)
- [Building from Source](#building-from-source)
- [How It Works](#how-it-works)
- [Roadmap](#roadmap)
- [Troubleshooting](#troubleshooting)
- [License](#license)

---

## Features

- **Correct** — Auto-detects French or English and fixes spelling, grammar, and syntax errors
- **Translate** — Instantly translates between French and English (or any configured language pair)
- **Rewrite** — Rewrites your text using customizable prompt templates (funny, formal, sarcastic, flirty, poetic, and more)
- **Multi-Provider** — Works with Anthropic Claude, OpenAI GPT, Google Gemini, xAI Grok, or local Ollama models
- **Hotkey Driven** — One hotkey to learn: Ctrl+G performs the active mode (correct, translate, or rewrite). Cancel from the tray menu. Optional dedicated hotkeys for power users
- **System Tray** — Switch modes, languages, templates, and toggle sound from the tray icon
- **Sound Effects** — Audio feedback with random WAV variants for each event (start, working, success, error, toggle). Disable via tray or config
- **Configurable** — JSON config file for API keys, providers, hotkeys, prompts, overlay settings, and custom rewrite templates
- **Lightweight** — Single binary, runs in the background, under 50 MB memory, near-zero CPU at idle
- **Cross-Platform** — Windows first, Linux and macOS coming in future releases

---

## Quick Start

### 1. Download

Download the latest release for your platform from the [Releases](https://github.com/chrixbedardcad/GhostType/releases) page.

### 2. Configure

On first run, GhostType creates a default `config.json` in the same directory. Open it and add your API key:

```json
{
  "llm_provider": "anthropic",
  "api_key": "YOUR_API_KEY_HERE",
  "model": "claude-sonnet-4-5-20250929"
}
```

### 3. Run

```bash
ghosttype.exe
```

GhostType starts minimized in your system tray. Open Firestorm, type something in chat, and press **Ctrl+G**.

---

## Hotkeys

### Default

| Hotkey | Action |
|--------|--------|
| **Ctrl+G** | Perform active mode (correct, translate, or rewrite) |
| **Ctrl+Z** | Undo replacement (native) |

Cancel an in-progress LLM call from the **Cancel LLM** item in the system tray menu.

### Optional (add in `config.json`)

Power users can add dedicated hotkeys for specific modes:

| Config Key | Recommended | Action |
|------------|-------------|--------|
| `hotkeys.translate` | `"Ctrl+J"` | Translate directly |
| `hotkeys.toggle_language` | `"Ctrl+Shift+J"` | Cycle translation target language |
| `hotkeys.rewrite` | `"Ctrl+Y"` | Rewrite directly |
| `hotkeys.cycle_template` | `"Ctrl+Shift+R"` | Cycle rewrite template |

All hotkeys are configurable in `config.json`. Set `active_mode` to `"correct"`, `"translate"`, or `"rewrite"` to choose what **Ctrl+G** does.

### Why These Keys?

- **Ctrl+G** — G for Grammar
- **Ctrl+J** — J is adjacent to G on the keyboard, easy to reach
- **Ctrl+Y** — Y for Yes/Yeet/rewrite, mostly unused in apps
- **Ctrl+Shift+J** — Shift modifier on translate key for toggle
- **Ctrl+Shift+R** — R for Rewrite template cycling

---

## Configuration

GhostType is configured entirely through `config.json`. Here is a full example:

```json
{
  "llm_provider": "anthropic",
  "api_key": "sk-ant-xxxxx",
  "model": "claude-sonnet-4-5-20250929",
  "api_endpoint": "",
  "languages": ["en", "fr"],
  "language_names": {
    "en": "English",
    "fr": "French"
  },
  "translate_targets": ["en|fr"],
  "active_mode": "correct",
  "hotkeys": {
    "correct": "Ctrl+G",
    "translate": "",
    "toggle_language": "",
    "rewrite": "",
    "cycle_template": ""
  },
  "prompts": {
    "correct": "Detect the language. Fix spelling and grammar. Return ONLY corrected text.",
    "translate": "Translate to {target_language}. Return ONLY the translation.",
    "rewrite_templates": [
      { "name": "funny", "prompt": "Rewrite as a funny, witty reply. Return ONLY the text." },
      { "name": "formal", "prompt": "Rewrite in a formal tone. Return ONLY the text." },
      { "name": "sarcastic", "prompt": "Rewrite with heavy sarcasm. Return ONLY the text." },
      { "name": "flirty", "prompt": "Rewrite in a playful, flirty tone. Return ONLY the text." },
      { "name": "poetic", "prompt": "Rewrite as a romantic poet. Return ONLY the text." }
    ]
  },
  "overlay": {
    "enabled": true,
    "opacity": 0.85,
    "auto_dismiss_seconds": 10,
    "highlight_changes": true,
    "font_size": 14
  },
  "max_tokens": 256,
  "timeout_ms": 5000,
  "preserve_clipboard": true,
  "sound_enabled": true,
  "log_level": "info",
  "log_file": "ghosttype.log"
}
```

---

## Supported Providers

| Provider | Config Value | Notes |
|----------|-------------|-------|
| Anthropic Claude | `anthropic` | Recommended. Excellent multilingual support. |
| OpenAI GPT | `openai` | GPT-4o or GPT-4 Turbo recommended. |
| Google Gemini | `gemini` | Good for multilingual tasks. |
| xAI Grok | `xai` | Fast inference. |
| Ollama (local) | `ollama` | Free, private, no API key needed. Requires Ollama running locally. |

Set `api_endpoint` to override the default endpoint for any provider — useful for proxies or custom deployments.

---

## Custom Rewrite Templates

You can add your own rewrite styles by editing the `rewrite_templates` array in `config.json`:

```json
{
  "name": "pirate",
  "prompt": "Rewrite this as a pirate would say it. Return ONLY the rewritten text."
}
```

Switch between templates from the system tray menu, or assign a dedicated hotkey (e.g., `"cycle_template": "Ctrl+Shift+R"`) to cycle through them in real-time. Similarly, switch translation targets from the tray or via a toggle hotkey.

---

## Building from Source

### Requirements

- Go 1.22 or later
- Windows 10/11 (MVP target)

### Build (console + tray)

Standard build — opens a console window alongside the tray icon (useful for development and debugging):

```bash
git clone https://github.com/chrixbedardcad/GhostType.git
cd GhostType
go mod download
go build -o ghosttype.exe
```

### Build (tray only, no console)

Production build — runs as a tray icon only with no console window:

```bash
go build -ldflags -H=windowsgui -o ghosttype.exe
```

### Run Tests

```bash
go test ./...
```

---

## How It Works

1. GhostType runs in the background and watches for hotkey presses.
2. It works globally — hotkeys fire regardless of which window is focused.
3. Choose your active mode: **correct**, **translate**, or **rewrite** from the system tray icon (or via `active_mode` in config).
4. When you press **Ctrl+G**, GhostType detects any selected text. If you have a selection, only that text is processed. If nothing is selected, it selects all text in the active input.
5. The text is sent to your configured LLM provider with the appropriate prompt for the active mode.
6. The result replaces the original text. Cancel from the tray menu, or **Ctrl+Z** to undo.
7. Your original clipboard content is preserved and restored.

---

## Roadmap

| Version | Focus | Highlights |
|---------|-------|------------|
| **v0.1** | MVP (current) | Windows desktop app. Correct, translate, and rewrite modes. Anthropic and OpenAI support. System tray with mode/language/template switching. Sound effects. |
| **v0.2** | Platform & Providers | Ollama local LLM support. Linux support. Gemini and xAI providers. |
| **v0.3** | Polish | Cursor animation during processing. macOS support. Config hot-reload. |
| **v0.4** | GUI & Overlay | GUI config panel. Transparent overlay with diff view. |
| **v0.5** | Power Features | Real-time Grammarly-style correction. Usage stats. Custom plugins. |

---

## Troubleshooting

| Problem | Solution |
|---------|----------|
| GhostType doesn't respond to hotkeys | Verify GhostType is running. Check `ghosttype.log` for registration errors. If hotkeys conflict with other apps, change them in `config.json`. |
| API errors | Check your API key in `config.json`. Check `ghosttype.log` for details. Verify your provider account has credits. |
| Slow corrections | Response time depends on provider and network. Try a faster model or switch to a local Ollama instance. |
| Hotkey conflicts | If Ctrl+G conflicts with another app, change it via `hotkeys.correct` in `config.json`. |

---

## License

MIT

## Author

Chris

## Acknowledgments

Inspired by the UX patterns of [Grammarly](https://www.grammarly.com/), [LanguageTool](https://languagetool.org/), [Espanso](https://espanso.org/), [Raycast AI](https://www.raycast.com/), and macOS inline autocorrect. Built for the Second Life community.
