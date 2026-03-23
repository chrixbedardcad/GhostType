<p align="center">
  <img src="assets/GhostSpell_icon_1024.png" alt="GhostSpell" width="160">
</p>

<h1 align="center">GhostSpell</h1>

<p align="center">
  <strong>AI-powered text correction, translation, and rewriting — anywhere you type.</strong>
</p>

<p align="center">
  <a href="https://github.com/chrixbedardcad/GhostSpell/actions/workflows/ci.yml"><img src="https://github.com/chrixbedardcad/GhostSpell/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/chrixbedardcad/GhostSpell/releases/latest"><img src="https://github.com/chrixbedardcad/GhostSpell/actions/workflows/release.yml/badge.svg" alt="Release"></a>
  <a href="https://github.com/chrixbedardcad/GhostSpell/releases/latest"><img src="https://img.shields.io/github/v/release/chrixbedardcad/GhostSpell?color=blue&label=version" alt="Version"></a>
  <img src="https://img.shields.io/badge/platform-Windows%20%7C%20macOS%20%7C%20Linux-lightgrey" alt="Platform">
</p>

---

Unlike traditional auto-correction tools that constantly interrupt your flow with inline suggestions and red underlines, GhostSpell stays invisible until you need it. No distractions, no popups — just press **F7** when you're ready, and it does the work. Your workflow, your timing.

GhostSpell runs in the background as a system tray app. Select text (or let it select all), press **F7**, and your text is corrected, translated, or rewritten — powered by your choice of LLM. Works in any application: browsers, chat clients, editors, email.

**Check latest version:**

macOS / Linux:
```bash
curl -s https://api.github.com/repos/chrixbedardcad/GhostSpell/releases/latest | grep tag_name
```

Windows:
```cmd
curl -s https://api.github.com/repos/chrixbedardcad/GhostSpell/releases/latest | findstr tag_name
```

## Install

**macOS / Linux:**

```bash
curl -fsSL https://raw.githubusercontent.com/chrixbedardcad/GhostSpell/main/scripts/install.sh | bash
```

**Windows** (CMD, PowerShell, or Windows Terminal):

```cmd
powershell -c "irm https://raw.githubusercontent.com/chrixbedardcad/GhostSpell/main/scripts/install.ps1 | iex"
```

That's it. The installer downloads the latest release, installs it, and launches GhostSpell. On first run, a setup wizard helps you configure your LLM provider.

Updates are built in — go to **Settings > About > Check for Updates** and click **Update Now**.

<details>
<summary>Manual download</summary>

| Platform | Download |
|----------|----------|
| **Windows** | [ghostspell-windows-amd64.exe](https://github.com/chrixbedardcad/GhostSpell/releases/latest/download/ghostspell-windows-amd64.exe) |
| **Linux** | [ghostspell-linux-amd64](https://github.com/chrixbedardcad/GhostSpell/releases/latest/download/ghostspell-linux-amd64) |
| **macOS** (Intel) | [GhostSpell-darwin-amd64.dmg](https://github.com/chrixbedardcad/GhostSpell/releases/latest/download/GhostSpell-darwin-amd64.dmg) |
| **macOS** (Apple Silicon) | [GhostSpell-darwin-arm64.dmg](https://github.com/chrixbedardcad/GhostSpell/releases/latest/download/GhostSpell-darwin-arm64.dmg) |

</details>

<details>
<summary>What does the install script do?</summary>

**macOS:** Downloads the `.dmg` for your chip (Intel or Apple Silicon), mounts it, copies `GhostSpell.app` to `/Applications`, removes the Gatekeeper quarantine flag, and opens macOS permission settings.

**Linux:** Downloads the binary to `/usr/local/bin/ghostspell` and checks for required dependencies (`xclip`, `xdotool`, `libwebkit2gtk-4.1`, `libgtk-3`).

**Windows:** Downloads `ghostspell.exe` to `%LOCALAPPDATA%\GhostSpell\`, adds to PATH, and creates a Start Menu shortcut.

Scripts only download from official GitHub releases — inspect them at [`scripts/install.sh`](scripts/install.sh) and [`scripts/install.ps1`](scripts/install.ps1).
</details>

<details>
<summary>Check latest version</summary>

macOS / Linux:
```bash
curl -s https://api.github.com/repos/chrixbedardcad/GhostSpell/releases/latest | grep tag_name
```

Windows:
```cmd
curl -s https://api.github.com/repos/chrixbedardcad/GhostSpell/releases/latest | findstr tag_name
```

</details>

<details>
<summary>Uninstall</summary>

> **⚠️ Warning:** This will remove GhostSpell and all its data. Back up `config.json` first if you want to keep your settings.

**macOS / Linux:**
```bash
curl -fsSL https://raw.githubusercontent.com/chrixbedardcad/GhostSpell/main/scripts/uninstall.sh | bash
```

**Windows:**
```cmd
powershell -c "irm https://raw.githubusercontent.com/chrixbedardcad/GhostSpell/main/scripts/uninstall.ps1 | iex"
```

</details>

## Build from Source

### macOS — one-liner (installs everything)

```bash
curl -fsSL https://raw.githubusercontent.com/chrixbedardcad/GhostSpell/main/scripts/setup-mac-dev.sh | bash
```

Installs Go, Node, CMake, Claude Code via Homebrew. Builds everything. Don't use `sudo` — run as your normal user.

### macOS — manual steps (if Homebrew is already installed)

```bash
# Install tools
brew install go node cmake
npm install -g @anthropic-ai/claude-code

# Clone and build
git clone https://github.com/chrixbedardcad/GhostSpell.git
cd GhostSpell
./scripts/build-ghostai.sh
./scripts/build-ghostvoice.sh
cd gui/frontend && npm ci && npm run build && cd ../..
CGO_ENABLED=1 go build -tags "production ghostai" -o ghostspell .

# Run
./ghostspell
```

### Windows

Requires: [Go](https://go.dev/dl/), [Node.js](https://nodejs.org/), [MSYS2](https://www.msys2.org/) (MinGW64 toolchain).

```cmd
_build.bat
```

Full clean rebuild: `_buildall.bat`

---

## Quick Start

1. **Install** GhostSpell using the one-liner above
2. **Choose an AI engine** — the setup wizard opens on first launch. Pick **Ghost-AI** (free, local, private), **Log in with ChatGPT** (one click, no API key), or connect any cloud provider with an API key.
3. **Use it** — type something in any app, press **F7**

That's the whole workflow. Set your writing language once in Settings, and all skills adapt automatically.

### Skills

GhostSpell ships with 12 built-in skills. Switch between them from the tray menu, the ghost indicator, or press **Shift+F7** to cycle.

| Skill | Input | Output | What it does |
|-------|-------|--------|-------------|
| ✏️ **Correct** | Text | Replace | Fix spelling, grammar, and syntax |
| 💎 **Polish** | Text | Replace | Improve clarity and flow |
| 😄 **Funny** | Text | Replace | Rewrite with humor |
| 📝 **Elaborate** | Text | Replace | Expand with more detail |
| ✂️ **Shorten** | Text | Replace | Make it more concise |
| 🌐 **Translate** | Text | Replace | Translate to target language |
| ❓ **Ask** | Text | Replace | Answer a question |
| 📖 **Define** | Text | Popup | Define a word or phrase |
| 📸 **Describe Screenshot** | Screenshot | Popup | Describe what's on screen |
| 🖥️ **Screenshot OCR** | Screenshot | Popup | Extract text from screen |
| 💬 **Voice to Text** | Voice → Direct | Paste | Record speech, paste transcription |
| 📝 **Voice Note** | Voice → LLM | Paste | Record speech, clean up with AI |

Each skill can be configured with:
- **Input mode** — Text, Voice → LLM, Voice → Direct, Screenshot
- **Output mode** — Replace, Append, Popup
- **LLM override** — use a different model per skill
- **Enable/disable** — hide from tray and cycling without deleting

Create custom skills in **Settings > Skills**. Use `{{language}}` in any skill prompt to reference your writing language.

### Architecture

GhostSpell uses separate processes for AI engines — no conflicts, clean isolation:

| Binary | Engine | What it does |
|--------|--------|-------------|
| **ghostspell** | — | Main app: hotkeys, clipboard, UI, orchestration |
| **ghostai** | llama.cpp | LLM text processing (Correct, Polish, Ask, etc.) |
| **ghostvoice** | whisper.cpp | Speech-to-text transcription (Voice skills) |

Each binary links its own AI libraries. If one crashes, the main app stays alive. Each can be tested independently from the command line.

### Local AI

**Ghost-AI** — Built-in local LLM engine powered by llama.cpp. Pick a model in the wizard, download it (~1-3 GB), and you're running local AI with zero setup. Recommended: Qwen3.5-4B.

**Ghost Voice** — Built-in local speech-to-text powered by whisper.cpp. Download a whisper model (75MB-3GB) in Settings > Voice. Recommended: whisper-base (142MB).

**Ollama** — If you already use Ollama, select it in the setup wizard. GhostSpell detects your install, lets you pick a model, and connects automatically.

---

## Features

- **One hotkey** — F7 does everything. Shift+F7 cycles skills.
- **12 built-in skills** — Correct, Polish, Funny, Elaborate, Shorten, Translate, Ask, Define, Screenshot, OCR, Voice to Text, Voice Note.
- **Voice input** — Record speech with push-to-talk (F7 start, F7 stop). Voice-reactive ghost indicator with red recording dot.
- **Screenshot skills** — Capture the active window and send to a vision model (GPT-4o, Claude, Gemini).
- **Ghost-AI** — Built-in local LLM engine (llama.cpp). No account, no API key, works offline.
- **Ghost Voice** — Built-in local speech-to-text (whisper.cpp). Runs as a separate process.
- **Multi-provider** — Anthropic, OpenAI, Gemini, xAI, DeepSeek, Ollama, LM Studio. Use different models per skill.
- **ChatGPT OAuth** — Log in with your ChatGPT account, no API key needed.
- **Global writing language** — Set once, all skills adapt. Use `{{language}}` in custom skill prompts.
- **Native language accent correction** — Helps the LLM fix accent-related transcription errors in voice skills.
- **Ghost indicator** — Floating overlay shows active skill, processing status, recording state. Draggable.
- **Settings GUI** — Tabbed settings panel: About, General, Models, Skills, Hotkeys, Stats, Debug, Help.
- **Enable/disable skills** — Hide skills from tray and cycling without deleting them.
- **One-click updates** — Check for updates and install from Settings > About.
- **Sound effects** — Audio feedback for every action. Toggleable.
- **System tray** — Switch skills, view models (LLM + Voice), report bugs.
- **Cross-platform** — Windows, macOS, Linux.

---

## Hotkeys

| Hotkey | Action |
|--------|--------|
| **F7** | Perform active skill (correct, translate, record voice, etc.) |
| **Shift+F7** | Cycle to next skill |
| **F7** (during recording) | Stop voice recording |
| **F7** (during processing) | Cancel active request |
| **Ctrl+Z** | Undo replacement (native) |

Hotkeys are configurable in **Settings > Hotkeys**.

---

## Supported Providers

| Provider | Notes |
|----------|-------|
| **OpenAI GPT** | GPT-5.4, GPT-5-mini, GPT-5.3-Codex, and more. ChatGPT OAuth login available. |
| **Anthropic Claude** | Claude Opus 4.6, Sonnet 4.6, Haiku 4.5. Excellent multilingual support. |
| **Google Gemini** | Gemini 2.5 Flash/Pro, 3.1 Pro preview. Good for multilingual tasks. |
| **xAI Grok** | Grok 4.1 Fast, Grok 4, Grok 3. Fast inference. |
| **DeepSeek** | DeepSeek Chat (V3.2) and Reasoner. Affordable and capable. |
| **Ollama** | Free, private, local. No API key needed. |
| **LM Studio** | Local inference server. Auto-detects loaded models. |

---

## Configuration

Most users only need the **Settings GUI**. For power users, everything is stored in `config.json`:

| Platform | Config path |
|----------|------------|
| **macOS** | `~/Library/Application Support/GhostSpell/config.json` |
| **Windows** | `%APPDATA%\GhostSpell\config.json` |
| **Linux** | `~/.config/GhostSpell/config.json` |

<details>
<summary>Full config example</summary>

```json
{
  "llm_providers": {
    "claude": {
      "provider": "anthropic",
      "api_key": "sk-ant-xxxxx",
      "model": "claude-sonnet-4-6"
    },
    "chatgpt": {
      "provider": "openai",
      "api_key": "sk-xxxxx",
      "model": "gpt-5-mini",
      "refresh_token": "v1-xxxxx"
    }
  },
  "default_llm": "claude",
  "active_prompt": 0,
  "prompts": [
    { "name": "Correct", "icon": "✏️", "prompt": "Detect the language. Fix spelling and grammar. Return ONLY the corrected text." },
    { "name": "Polish", "icon": "💎", "prompt": "Improve clarity and flow. Return ONLY the improved text." },
    { "name": "Translate", "icon": "🌐", "prompt": "Translate to {target_language}. Return ONLY the translation.", "llm": "chatgpt" }
  ],
  "hotkeys": {
    "action": "Ctrl+G",
    "cycle_prompt": ""
  },
  "preserve_clipboard": true,
  "sound_enabled": true,
  "max_input_chars": 2000,
  "log_level": ""
}
```

</details>

---

## Platform Notes

### Windows

No dependencies. The installer creates a Start Menu shortcut and adds GhostSpell to Windows startup. Debug logs are available in **Settings > Debug**.

### macOS

No dependencies to install. Runs as a menu bar app (no Dock icon). The curl installer handles everything.

GhostSpell is **signed and notarized** by Apple — no Gatekeeper warnings. Just install and run.

**Grant permissions**

GhostSpell needs two macOS permissions — these cannot be bypassed, even by signed apps:

- **Accessibility** (System Settings > Privacy & Security > Accessibility) — required for keyboard simulation (Ctrl+A, Ctrl+C, Ctrl+V). [Apple's guide](https://support.apple.com/guide/mac-help/allow-accessibility-apps-to-access-your-mac-mh43185/mac)
- **Input Monitoring** (System Settings > Privacy & Security > Input Monitoring) — required for global hotkeys (Ctrl+G). [Apple's guide](https://support.apple.com/guide/mac-help/control-access-to-input-monitoring-on-mac-mchl4cedafb6/mac)

The app will prompt you to grant these on first launch. Toggle GhostSpell **ON** in each.

### Linux

Requires: `sudo apt install libwebkit2gtk-4.1-0 libgtk-3-0 xclip xdotool`

Sound requires PulseAudio (`paplay`) or ALSA (`aplay`).

---

## How It Works

1. GhostSpell runs in the system tray, watching for hotkey presses.
2. Press **Ctrl+G** — it detects selected text (or selects all if nothing selected).
3. Text is sent to your LLM provider with the active prompt.
4. The result replaces the original text. **Ctrl+Z** to undo.
5. Your clipboard is preserved and restored.

---

## Building from Source

### Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| **Go** | 1.25+ | https://go.dev/dl/ |
| **Node.js** | 22+ | https://nodejs.org (LTS) — includes npm |
| **Git** | any | https://git-scm.com |
| **MinGW** (Windows only) | latest | `winget install MSYS2.MSYS2` then see below |

#### Windows: Install MinGW (required for CGO)

GhostSpell uses CGO for the Ghost-AI engine. On Windows, you need MinGW:

```powershell
# Install MSYS2 (if not installed)
winget install MSYS2.MSYS2

# Open MSYS2 UCRT64 terminal and install GCC:
pacman -S --noconfirm mingw-w64-x86_64-gcc mingw-w64-x86_64-ninja

# Add to PATH (run in PowerShell or add to system PATH permanently):
$env:PATH = "C:\msys64\mingw64\bin;" + $env:PATH
```

Verify: `gcc --version` should show the MinGW GCC version.

### Build Steps (Windows)

Open a terminal (PowerShell, CMD, or Git Bash) in the project directory:

```bash
# 1. Clone the repo
git clone https://github.com/chrixbedardcad/GhostSpell.git
cd GhostSpell

# 2. Build the React frontend (required — without this, windows are blank)
cd gui/frontend
npm install
npm run build
cd ../..

# 3. Build Ghost-AI static libraries (optional — for local AI)
# In MSYS2 UCRT64 terminal or Git Bash:
bash scripts/build-ghostai.sh

# 4. Build the binary
# With Ghost-AI (local AI):
go build -tags "production ghostai" -ldflags "-H=windowsgui -extldflags '-static'" -o ghostspell.exe .

# Without Ghost-AI (cloud providers only):
go build -tags "production" -ldflags "-H=windowsgui" -o ghostspell.exe .

# 5. Run (with console output for debugging):
.\ghostspell.exe
```

> **Note:** The `-H=windowsgui` flag hides the console window. For debugging, remove it to see log output:
> ```bash
> go build -tags "production ghostai" -extldflags '-static'" -o ghostspell.exe .
> .\ghostspell.exe
> ```

### Build Steps (macOS)

```bash
git clone https://github.com/chrixbedardcad/GhostSpell.git
cd GhostSpell
cd gui/frontend && npm install && npm run build && cd ../..
bash scripts/build-ghostai.sh
go build -tags "production ghostai" -o ghostspell .
./ghostspell
```

### Build Steps (Linux)

```bash
sudo apt install libwebkit2gtk-4.1-dev libgtk-3-dev
git clone https://github.com/chrixbedardcad/GhostSpell.git
cd GhostSpell
cd gui/frontend && npm install && npm run build && cd ../..
bash scripts/build-ghostai.sh
go build -tags "webkit2_41 production ghostai" -o ghostspell .
./ghostspell
```

### Development

```bash
# Frontend hot reload (Vite dev server):
cd gui/frontend && npm run dev

# Type-check without building:
cd gui/frontend && npm run typecheck

# Go tests:
go test -tags webkit2_41 ./...
```

---

## Troubleshooting

| Problem | Solution |
|---------|----------|
| Hotkeys don't work | Check that GhostSpell is running (look for tray icon). On macOS, grant Input Monitoring permission. |
| **Windows**: Ctrl+G captures empty text | Update to v0.10.2+. Older versions had a stuck modifier key issue where Ctrl from the hotkey wasn't released before simulating Ctrl+C. |
| API errors | Verify API key in Settings > Models. Check that your provider account has credits. |
| Slow responses | Try a faster model, or switch to local Ollama. |
| **Linux**: Missing dependencies | `sudo apt install libwebkit2gtk-4.1-0 libgtk-3-0 xclip xdotool` |
| **macOS**: Keyboard simulation fails | Grant Accessibility permission in System Settings > Privacy & Security. |

### Debug Logs

GhostSpell writes logs to `ghostspell.log` in your config directory. To view logs in real time:

**macOS:**
```bash
tail -f ~/Library/Application\ Support/GhostSpell/ghostspell.log
```

**Windows** (PowerShell):
```powershell
Get-Content "$env:APPDATA\GhostSpell\ghostspell.log" -Wait
```

**Linux:**
```bash
tail -f ~/.config/GhostSpell/ghostspell.log
```

Crash logs are written to `ghostspell_crash.log` in the same directory.

You can also enable debug logging from **Settings > Debug** (auto-disables after 30 minutes).

<details>
<summary>Run from terminal for live output</summary>

For the most detailed output, run GhostSpell directly from a terminal:

**macOS:**
```bash
/Applications/GhostSpell.app/Contents/MacOS/GhostSpell
```

**Windows** (CMD):
```cmd
"%LOCALAPPDATA%\GhostSpell\ghostspell.exe"
```

**Linux:**
```bash
ghostspell
```

</details>

### Reporting a Bug

To file a bug report with logs attached:

**macOS / Linux:**
```bash
# Copy the log path for your platform:
#   macOS:  ~/Library/Application Support/GhostSpell/ghostspell.log
#   Linux:  ~/.config/GhostSpell/ghostspell.log

gh issue create --repo chrixbedardcad/GhostSpell \
  --title "Bug: <describe the issue>" \
  --body "$(cat <<'EOF'
## Description
<What happened and what you expected>

## Steps to Reproduce
1. ...

## Log file attached below
EOF
)" \
  --label bug
# Then attach the log file as a comment:
gh issue comment --repo chrixbedardcad/GhostSpell ISSUE_NUMBER \
  --body "$(cat ~/Library/Application\ Support/GhostSpell/ghostspell.log)"
```

**Windows** (PowerShell):
```powershell
# Create the issue
gh issue create --repo chrixbedardcad/GhostSpell `
  --title "Bug: <describe the issue>" `
  --body "## Description`n<What happened>`n`n## Steps to Reproduce`n1. ...`n`n## Log file attached below" `
  --label bug
# Then attach the log file as a comment:
gh issue comment --repo chrixbedardcad/GhostSpell ISSUE_NUMBER `
  --body (Get-Content "$env:APPDATA\GhostSpell\ghostspell.log" -Raw)
```

> **Note:** Replace `ISSUE_NUMBER` with the number returned by `gh issue create`. Requires the [GitHub CLI](https://cli.github.com/).

---

## License

MIT

## Acknowledgments

Inspired by [Grammarly](https://www.grammarly.com/), [LanguageTool](https://languagetool.org/), [Espanso](https://espanso.org/), [Raycast AI](https://www.raycast.com/), and macOS inline autocorrect.
