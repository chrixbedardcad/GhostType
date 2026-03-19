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

Unlike traditional auto-correction tools that constantly interrupt your flow with inline suggestions and red underlines, GhostSpell stays invisible until you need it. No distractions, no popups — just press **Ctrl+G** when you're ready, and it does the work. Your workflow, your timing.

GhostSpell runs in the background as a system tray app. Select text (or let it select all), press **Ctrl+G**, and your text is corrected, translated, or rewritten — powered by your choice of LLM. Works in any application: browsers, chat clients, editors, email.

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

```bash
gh release view --repo chrixbedardcad/GhostSpell --json tagName -q .tagName
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

---

## Quick Start

1. **Install** GhostSpell using the one-liner above
2. **Choose an AI engine** — the setup wizard opens on first launch. Pick **Ghost-AI** (free, local, private), **Log in with ChatGPT** (one click, no API key), or connect any cloud provider with an API key.
3. **Use it** — type something in any app, press **Ctrl+G**

That's the whole workflow. GhostSpell auto-detects the language and fixes spelling, grammar, and syntax.

### Prompts

GhostSpell ships with 6 built-in prompts. Switch between them from the tray menu or press your **Cycle Prompt** hotkey.

| Prompt | What it does |
|--------|-------------|
| ✏️ **Correct** | Fix spelling, grammar, and syntax (default) |
| 💎 **Polish** | Improve clarity and flow without changing meaning |
| 😄 **Funny** | Rewrite with humor |
| 📝 **Elaborate** | Expand on the text with more detail |
| ✂️ **Shorten** | Make it more concise |
| 🌐 **Translate** | Translate between configured language pairs |

Emoji icons show in the tray menu for quick recognition. You can create custom prompts and assign per-prompt LLM overrides in **Settings > Prompts**.

### Local AI

**Ghost-AI (built-in)** — No install needed. GhostSpell ships with an embedded llama.cpp engine. Pick a model in the wizard, download it (~1 GB), and you're running local AI with zero setup. Recommended model: Qwen3.5-2B.

**Ollama** — If you already use Ollama, select it in the setup wizard. GhostSpell detects your install, lets you pick a model, and connects automatically.

---

## Features

- **One hotkey** — Ctrl+G does everything. Optional Cycle Prompt hotkey for power users.
- **Ghost-AI** — Built-in local AI engine (embedded llama.cpp). No account, no API key, works offline.
- **Multi-provider** — Anthropic, OpenAI, Gemini, xAI, DeepSeek, Ollama. Use different models per prompt.
- **ChatGPT OAuth** — Log in with your ChatGPT account, no API key needed.
- **Setup wizard** — 4-step guided setup on first launch: Welcome → Permissions (macOS) → Model Selector → Ready.
- **Prompt icons** — Emoji icons in the tray menu for each prompt (✏️ Correct, 💎 Polish, etc.).
- **Settings GUI** — 8-tab settings panel built with React: About, General, Models, Prompts, Hotkeys, Stats, Debug, Help. Dark zen theme with custom-styled dropdowns.
- **One-click updates** — Check for updates and install from Settings > About.
- **Ollama integration** — Detect, install, pull models from the GUI.
- **Custom prompts** — Add your own prompt templates with optional per-prompt LLM overrides.
- **Sound effects** — Audio feedback for every action. Toggleable.
- **System tray** — Switch prompts, models, languages from the tray icon.
- **Single instance** — Prevents duplicate processes across all platforms.
- **Lightweight** — Single binary, under 50 MB memory, near-zero CPU at idle.
- **Cross-platform** — Windows, macOS, Linux.

---

## Hotkeys

| Hotkey | Action |
|--------|--------|
| **Ctrl+G** | Perform active prompt (correct, translate, etc.) |
| **Ctrl+Z** | Undo replacement (native) |

Configure an additional hotkey in **Settings > General**:

| Hotkey | Action |
|--------|--------|
| Cycle Prompt | Cycle through prompt templates |

> **macOS:** `Ctrl` maps to Command, `Alt` maps to Option.

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

Requires **Go 1.25+** and **Node.js 22+**.

```bash
git clone https://github.com/chrixbedardcad/GhostSpell.git
cd GhostSpell
```

### 1. Build the React frontend

The UI is built with React + Vite + Tailwind. This step must run before `go build`.

```bash
cd gui/frontend
npm install
npm run build
cd ../..
```

This creates `gui/frontend/dist/` with the compiled assets. If you skip this step, the app will show blank windows.

### 2. Build Ghost-AI (optional)

To include the built-in local AI engine (embedded llama.cpp), build the static libraries first:

```bash
./scripts/build-ghostai.sh   # downloads llama.cpp b8281, builds static libs
```

Skip this step if you only want cloud providers (OpenAI, Anthropic, etc.).

### 3. Build the binary

All platforms require `CGO_ENABLED=1`.

**Windows** (requires MinGW):
```bash
go build -tags "production ghostai" -ldflags "-H=windowsgui -extldflags '-static'" -o ghostspell.exe .
```

**Linux**:
```bash
sudo apt install libwebkit2gtk-4.1-dev libgtk-3-dev
go build -tags "webkit2_41 production ghostai" -o ghostspell .
```

**macOS**:
```bash
go build -tags "production ghostai" -o ghostspell .
```

Drop the `ghostai` tag if you skipped step 2:
```bash
go build -tags "production" -o ghostspell.exe .
```

### 4. Run

```bash
./ghostspell          # macOS / Linux
.\ghostspell.exe      # Windows
```

### Development

For frontend development with hot reload:

```bash
cd gui/frontend
npm run dev           # starts Vite dev server with HMR
```

Type-check without building:
```bash
npm run typecheck
```

**Tests:**
```bash
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
