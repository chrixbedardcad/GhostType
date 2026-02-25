<p align="center">
  <img src="GhostType_logo.png" alt="GhostType Logo" width="300">
</p>

# GhostType

**AI-powered multilingual auto-correction, translation, and creative rewriting for virtual world chat.**

GhostType is a lightweight background service that hooks into your chat application (primarily Firestorm Second Life viewer) and provides real-time spelling correction, language translation, and creative text rewriting — powered by your choice of LLM provider.

Type in French, hit **F6**, get it corrected. Switch to English, hit **F6**, corrected too. Need to translate? **F7**. Want to change the target language? **Ctrl+F7** — a tiny floating label near your cursor shows "To French" or "To English". Want a funny reply? **F8**. Want to switch rewrite style? **Ctrl+F8** — a floating label shows "Funny", "Professional", etc. Undo with **Ctrl+Z** or cancel with **Escape**. That simple.

---

## Table of Contents

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
- **Hotkey Driven** — F6 to correct, F7 to translate, F8 to rewrite, Ctrl+F7 to toggle translation language, Ctrl+F8 to cycle rewrite templates, Escape to cancel
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

GhostType starts minimized in your system tray. Open Firestorm, type something in chat, and press **F6**.

---

## Hotkeys

| Hotkey | Action |
|--------|--------|
| **F6** | Correct spelling, grammar, and syntax |
| **Ctrl+F7** | Toggle translation target language (shows cursor notification) |
| **F7** | Translate to selected target language |
| **Ctrl+F8** | Toggle rewrite template (shows cursor notification) |
| **F8** | Rewrite using selected template |
| **Escape** | Cancel in-progress operation |
| **Ctrl+Z** | Undo replacement (native) |

All hotkeys are configurable in `config.json`.

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
  "default_translate_target": "en",
  "hotkeys": {
    "correct": "F6",
    "translate": "F7",
    "toggle_language": "Ctrl+F7",
    "rewrite": "F8",
    "cycle_template": "Ctrl+F8",
    "cancel": "Escape"
  },
  "target_window": "Firestorm",
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

Cycle through templates in real-time with **Ctrl+F8**. A brief floating label appears near your cursor showing the newly selected template name (e.g., "Funny", "Professional"). Similarly, toggle the translation target language with **Ctrl+F7** — a label appears showing "To French", "To English", etc.

---

## Building from Source

### Requirements

- Go 1.22 or later
- Windows 10/11 (MVP target)

### Build

```bash
git clone https://github.com/chrixbedardcad/GhostType.git
cd GhostType
go mod download
go build -o ghosttype.exe
```

### Run Tests

```bash
go test ./...
```

---

## How It Works

1. GhostType runs in the background and watches for hotkey presses.
2. It only activates when the configured target window (default: Firestorm) is focused.
3. Before translating, you can press **Ctrl+F7** to toggle the translation target language. A brief floating label appears near your cursor showing the new target (e.g., "To French"). Before rewriting, you can press **Ctrl+F8** to toggle the rewrite template. A floating label appears showing the template name (e.g., "Funny", "Professional").
4. When you press an action hotkey (**F6**, **F7**, or **F8**), GhostType selects all text in the active chat input, copies it to clipboard, and reads it.
5. The text is sent to your configured LLM provider with the appropriate prompt.
6. The corrected/translated/rewritten result appears in an overlay near the chat input.
7. The result auto-replaces your text. Press **Escape** to cancel, or **Ctrl+Z** to undo.
8. Your original clipboard content is preserved and restored.

---

## Roadmap

| Version | Focus | Highlights |
|---------|-------|------------|
| **v0.1** | MVP (current) | Windows desktop app. Correction mode. Anthropic and OpenAI support. |
| **v0.2** | Translation & Overlay | Translation mode. Ctrl+F7 toggle language with cursor notification. Transparent overlay. Ollama support. Linux. |
| **v0.3** | Rewrite Mode | Creative rewrite templates. Ctrl+F8 toggle template with cursor notification. Config hot-reload. macOS. |
| **v0.4** | More Providers | Gemini and xAI support. GUI config panel. Additional languages. |
| **v0.5** | Power Features | Real-time Grammarly-style correction. Usage stats. Custom plugins. |

---

## Troubleshooting

| Problem | Solution |
|---------|----------|
| GhostType doesn't respond to hotkeys | Make sure the target window (Firestorm) is focused and the window title matches `target_window` in `config.json`. |
| API errors | Check your API key in `config.json`. Check `ghosttype.log` for details. Verify your provider account has credits. |
| Slow corrections | Response time depends on provider and network. Try a faster model or switch to a local Ollama instance. |
| Hotkey conflicts | If F6/F7/F8 conflict with other apps, change the hotkeys in `config.json`. |

---

## License

MIT

## Author

Chris

## Acknowledgments

Inspired by the UX patterns of [Grammarly](https://www.grammarly.com/), [LanguageTool](https://languagetool.org/), [Espanso](https://espanso.org/), [Raycast AI](https://www.raycast.com/), and macOS inline autocorrect. Built for the Second Life community.
