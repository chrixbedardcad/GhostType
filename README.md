<p align="center">
  <img src="GhostType_logo.png" alt="GhostType Logo" width="300">
</p>

# GhostType

**AI-powered multilingual auto-correction, translation, and creative rewriting — for any text input.**

GhostType is a lightweight background service that provides real-time spelling correction, language translation, and creative text rewriting — powered by your choice of LLM provider. It works globally with any application: chat clients, text editors, browsers, email — anywhere you type.

Type in French, hit **Ctrl+G**, get it corrected. Switch to English, hit **Ctrl+G**, corrected too. Want to translate or rewrite instead? Change the active mode in the system tray (or `config.json`) — **Ctrl+G** always does whatever mode is active. Undo with **Ctrl+Z** or cancel from the tray menu. That simple.

---

## Install

[![CI](https://github.com/chrixbedardcad/GhostType/actions/workflows/ci.yml/badge.svg)](https://github.com/chrixbedardcad/GhostType/actions/workflows/ci.yml)
[![Release](https://github.com/chrixbedardcad/GhostType/actions/workflows/release.yml/badge.svg)](https://github.com/chrixbedardcad/GhostType/releases/latest)
[![Version](https://img.shields.io/badge/version-v0.1.99-blue)](https://github.com/chrixbedardcad/GhostType/releases/latest)

### Quick Install (recommended)

Open a terminal and paste the command for your platform. It downloads the latest release, installs GhostType, and you're ready to go.

**macOS / Linux:**

```bash
curl -fsSL https://raw.githubusercontent.com/chrixbedardcad/GhostType/main/scripts/install.sh | bash
```

**Windows** (paste in CMD, PowerShell, or Windows Terminal):

```cmd
powershell -c "irm https://raw.githubusercontent.com/chrixbedardcad/GhostType/main/scripts/install.ps1 | iex"
```

<details>
<summary>What does the install script do?</summary>

**macOS:** Downloads the `.dmg` for your chip (Intel or Apple Silicon), mounts it, copies `GhostType.app` to `/Applications`, and removes the Gatekeeper quarantine flag so it launches without the "unidentified developer" warning.

**Linux:** Downloads the binary to `/usr/local/bin/ghosttype` and checks for required system dependencies (`xclip`, `xdotool`, `libwebkit2gtk-4.1`, `libgtk-3`). Prints install commands for any missing packages.

**Windows:** Downloads both `ghosttype.exe` (with console) and `ghosttype-windowless.exe` (tray only) to `%LOCALAPPDATA%\GhostType\` and adds that directory to your user PATH.

The scripts only download from official GitHub releases — you can inspect them at [`scripts/install.sh`](scripts/install.sh) and [`scripts/install.ps1`](scripts/install.ps1).
</details>

### Manual Download

| Platform | Download |
|----------|----------|
| **Windows** (recommended) | [ghosttype-windows-amd64.exe](https://github.com/chrixbedardcad/GhostType/releases/latest/download/ghosttype-windows-amd64.exe) |
| **Windows** (no console) | [ghosttype-windows-amd64-windowless.exe](https://github.com/chrixbedardcad/GhostType/releases/latest/download/ghosttype-windows-amd64-windowless.exe) |
| **Linux** | [ghosttype-linux-amd64](https://github.com/chrixbedardcad/GhostType/releases/latest/download/ghosttype-linux-amd64) |
| **macOS** (Intel) | [GhostType-darwin-amd64.dmg](https://github.com/chrixbedardcad/GhostType/releases/latest/download/GhostType-darwin-amd64.dmg) |
| **macOS** (Apple Silicon) | [GhostType-darwin-arm64.dmg](https://github.com/chrixbedardcad/GhostType/releases/latest/download/GhostType-darwin-arm64.dmg) |

---

## Table of Contents

- [Install](#install)
- [Platform Requirements](#platform-requirements)
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
- [Uninstall](#uninstall)
- [License](#license)

---

## Platform Requirements

### Windows

No additional dependencies. Download and run.

### Linux

Install the following packages before running GhostType:

```bash
sudo apt install libwebkit2gtk-4.1-0 libgtk-3-0 xclip xdotool
```

| Package | Purpose |
|---------|---------|
| `libwebkit2gtk-4.1-0` | Settings GUI and system tray |
| `libgtk-3-0` | GTK window toolkit |
| `xclip` | Clipboard read/write |
| `xdotool` | Keyboard simulation (Ctrl+A/C/V) |

Sound requires PulseAudio (`paplay`) or ALSA (`aplay`) — usually pre-installed on desktop Linux.

### macOS

No additional packages to install. GhostType ships as a `.dmg` disk image.

#### 1. Install

1. **Download** the `.dmg` for your Mac ([Apple Silicon](https://github.com/chrixbedardcad/GhostType/releases/latest/download/GhostType-darwin-arm64.dmg) or [Intel](https://github.com/chrixbedardcad/GhostType/releases/latest/download/GhostType-darwin-amd64.dmg)).
2. **Open** the `.dmg` — double-click to mount it.
3. **Drag** `GhostType.app` to the **Applications** folder shortcut inside the disk image.

#### 2. Allow the app to run (Gatekeeper)

GhostType is not signed with an Apple Developer certificate, so macOS will block it by default.

**Right-click** `GhostType.app` → **Open** → click **Open** in the security dialog.

Or from Terminal:

```bash
xattr -d com.apple.quarantine /Applications/GhostType.app
```

#### 3. Grant permissions

On first launch, macOS will prompt you to grant:

- **Accessibility** permission (System Settings → Privacy & Security → Accessibility) — required for keyboard simulation
- **Input Monitoring** permission — required for global hotkeys

If the prompts don't appear automatically, add GhostType manually in System Settings → Privacy & Security.

#### 4. Run

Double-click `GhostType.app` in Applications, or:

```bash
open /Applications/GhostType.app
```

GhostType runs as a tray-only app (no Dock icon) — look for the icon in the menu bar.

---

## Features

- **Correct** — Auto-detects language and fixes spelling, grammar, and syntax errors
- **Translate** — Instantly translates between any configured language pair
- **Rewrite** — Rewrites your text using customizable prompt templates (funny, formal, sarcastic, flirty, poetic, and more)
- **Settings GUI** — Built-in settings panel for managing providers, testing connections, and configuring options — no hand-editing JSON required
- **Multi-Provider** — Named providers with per-mode LLM selection (e.g., use Claude for corrections, GPT for translations, Ollama for rewrites)
- **Ollama One-Click Setup** — Detect, install, pull models, and activate Ollama directly from the Settings GUI — no terminal needed
- **Hotkey Driven** — One hotkey to learn: Ctrl+G performs the active mode (correct, translate, or rewrite). Optional dedicated hotkeys for power users
- **System Tray** — Switch modes, languages, templates, providers, and toggle sound from the tray icon
- **Sound Effects** — Audio feedback with random WAV variants for each event (startup, working, success, error, toggle)
- **Configurable** — JSON config file for providers, hotkeys, prompts, and custom rewrite templates — or just use the GUI
- **Lightweight** — Single binary, runs in the background, under 50 MB memory, near-zero CPU at idle
- **Cross-Platform** — Windows, macOS, and Linux

---

## Quick Start

### Cloud Provider (Anthropic, OpenAI, etc.)

1. **Install** GhostType using the [one-liner](#quick-install-recommended) or [manual download](#manual-download).
2. **Run** GhostType — the Settings GUI opens automatically on first launch.
4. **Add a provider** — pick a provider from the dropdown (e.g., Anthropic), paste your API key, choose a model, click **Test**, then **Save**.
5. **Use it** — open any application, type something, press **Ctrl+G**.

### Local AI (Ollama)

1. **Run** GhostType and open the Settings GUI.
2. **Click Ollama** in the provider dropdown — the Ollama panel appears.
3. If Ollama isn't installed, click **Install Ollama** — your browser opens the download page. Install it, then click **Refresh**.
4. Start Ollama (`ollama serve`), then pick a recommended model (mistral, llama3, or gemma2) — it downloads automatically.
5. **Save** and press **Ctrl+G** in any application — corrections run locally, no API key needed.

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

> **macOS note**: Hotkeys use the same key names (e.g., `Ctrl+G`). On macOS, `Ctrl` automatically maps to the Command key (⌘), so `Ctrl+G` in the config registers as **⌘G**. The `Alt` modifier maps to Option (⌥).

### Why These Keys?

- **Ctrl+G** — G for Grammar
- **Ctrl+J** — J is adjacent to G on the keyboard, easy to reach
- **Ctrl+Y** — Y for Yes/Yeet/rewrite, mostly unused in apps
- **Ctrl+Shift+J** — Shift modifier on translate key for toggle
- **Ctrl+Shift+R** — R for Rewrite template cycling

---

## Configuration

Most users won't need to hand-edit config — the **Settings GUI** handles provider management, testing, and defaults. For power users, GhostType stores everything in `config.json`.

### Data Location

GhostType stores its config, logs, and crash logs in the OS-standard app data directory:

| Platform | Path |
|----------|------|
| **macOS** | `~/Library/Application Support/GhostType/` |
| **Windows** | `%APPDATA%\GhostType\` |
| **Linux** | `~/.config/GhostType/` |

Files in this directory:

| File | Purpose |
|------|---------|
| `config.json` | All settings (providers, hotkeys, prompts, etc.) |
| `ghosttype.log` | Application log (when `log_level` is set) |
| `ghosttype_crash.log` | Startup error log for windowless builds |

> **Upgrading from v0.1.94 or earlier?** If a `config.json` exists next to the executable, GhostType automatically migrates it to the new location on first launch.

### Provider Map (`llm_providers`)

Providers are stored as a named map. Each label is a user-defined name:

```json
{
  "llm_providers": {
    "claude": {
      "provider": "anthropic",
      "api_key": "sk-ant-xxxxx",
      "model": "claude-sonnet-4-5-20250929"
    },
    "gpt": {
      "provider": "openai",
      "api_key": "sk-xxxxx",
      "model": "gpt-4o"
    },
    "local": {
      "provider": "ollama",
      "model": "mistral"
    }
  },
  "default_llm": "claude",
  "correct_llm": "",
  "translate_llm": ""
}
```

- `default_llm` — the provider used for all modes unless overridden
- `correct_llm` — optional override for corrections (falls back to `default_llm`)
- `translate_llm` — optional override for translations (falls back to `default_llm`)
- Rewrite templates can also specify a per-template `"llm"` label

### Full Example

```json
{
  "llm_providers": {
    "claude": {
      "provider": "anthropic",
      "api_key": "sk-ant-xxxxx",
      "model": "claude-sonnet-4-5-20250929"
    }
  },
  "default_llm": "claude",
  "correct_llm": "",
  "translate_llm": "",
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
| Ollama (local) | `ollama` | Free, private, no API key needed. One-click install and model pull from the Settings GUI. |

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

- **Go 1.25** or later
- Platform-specific build dependencies (see below)

### Windows

No additional dependencies. Pure Go build with `CGO_ENABLED=0`:

```bash
git clone https://github.com/chrixbedardcad/GhostType.git
cd GhostType
go build -o ghosttype.exe .
```

Windowless build (tray only, no console window):

```bash
go build -ldflags "-H=windowsgui" -o ghosttype.exe .
```

### Linux

Install build dependencies:

```bash
sudo apt install libwebkit2gtk-4.1-dev libgtk-3-dev
```

Build with CGO and the webkit2_41 tag:

```bash
git clone https://github.com/chrixbedardcad/GhostType.git
cd GhostType
CGO_ENABLED=1 go build -tags webkit2_41 -o ghosttype .
```

### macOS

No additional build dependencies (uses system frameworks via CGO):

```bash
git clone https://github.com/chrixbedardcad/GhostType.git
cd GhostType
CGO_ENABLED=1 go build -o ghosttype .
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

| Version | Focus | Status |
|---------|-------|--------|
| **v0.1.0** | MVP | Done — Correct, translate, rewrite. Anthropic + OpenAI. System tray. Sound effects. |
| **v0.1.10–30** | Providers & Polish | Done — Ollama, Gemini, xAI providers. Config hot-reload. Multi-provider named map. |
| **v0.1.30–48** | GUI & Lifecycle | Done — Settings GUI. Ollama one-click install/pull/activate. Per-mode LLM selection. Tray provider switching. |
| **v0.1.49–56** | Cross-Platform | Done — Wails v3 migration. macOS and Linux support. |
| **v0.2** | Overlay & UX | Planned — Transparent overlay with diff view. Cursor animation during processing. |
| **v0.3** | Power Features | Planned — Real-time Grammarly-style correction. Usage stats. Custom plugins. |

---

## Troubleshooting

| Problem | Solution |
|---------|----------|
| GhostType doesn't respond to hotkeys | Verify GhostType is running. Check `ghosttype.log` for registration errors. If hotkeys conflict with other apps, change them in `config.json`. |
| API errors | Check your API key in `config.json`. Check `ghosttype.log` for details. Verify your provider account has credits. |
| Slow corrections | Response time depends on provider and network. Try a faster model or switch to a local Ollama instance. |
| Hotkey conflicts | If Ctrl+G conflicts with another app, change it via `hotkeys.correct` in `config.json`. |
| **Linux**: `xclip not found` | Install xclip: `sudo apt install xclip` |
| **Linux**: `xdotool not found` | Install xdotool: `sudo apt install xdotool` |
| **Linux**: No sound | Install PulseAudio (`paplay`) or ALSA (`aplay`). Check `pactl list sinks` for audio output. |
| **Linux**: Settings GUI won't open | Install webkit2gtk: `sudo apt install libwebkit2gtk-4.1-0 libgtk-3-0` |
| **macOS**: Hotkeys don't work | Grant Input Monitoring permission in System Settings → Privacy & Security. |
| **macOS**: Keyboard simulation fails | Grant Accessibility permission in System Settings → Privacy & Security → Accessibility. |

---

## Uninstall

### Quick Uninstall

Stops GhostType, removes the binary, and deletes all config/log files.

**macOS / Linux:**

```bash
curl -fsSL https://raw.githubusercontent.com/chrixbedardcad/GhostType/main/scripts/uninstall.sh | bash
```

**Windows:**

```cmd
powershell -c "irm https://raw.githubusercontent.com/chrixbedardcad/GhostType/main/scripts/uninstall.ps1 | iex"
```

### Manual Uninstall

If you prefer to do it yourself:

**macOS:**
```bash
rm -rf /Applications/GhostType.app
rm -rf ~/Library/Application\ Support/GhostType
```

**Linux:**
```bash
sudo rm /usr/local/bin/ghosttype
rm -rf ~/.config/GhostType
```

**Windows (PowerShell):**
```powershell
Remove-Item -Recurse -Force "$env:LOCALAPPDATA\GhostType"
Remove-Item -Recurse -Force "$env:APPDATA\GhostType"
```

> Back up `config.json` first if you want to keep your settings.

---

## License

MIT

## Author

Chris

## Acknowledgments

Inspired by the UX patterns of [Grammarly](https://www.grammarly.com/), [LanguageTool](https://languagetool.org/), [Espanso](https://espanso.org/), [Raycast AI](https://www.raycast.com/), and macOS inline autocorrect.
