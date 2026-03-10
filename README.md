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
| **Windows** | [ghostspell-windows-amd64.exe](https://github.com/chrixbedardcad/GhostSpell/releases/latest/download/ghostspell-windows-amd64.exe) — tray only, no console |
| **Windows** (debug) | [ghostspell-windows-amd64-window.exe](https://github.com/chrixbedardcad/GhostSpell/releases/latest/download/ghostspell-windows-amd64-window.exe) — with console window |
| **Linux** | [ghostspell-linux-amd64](https://github.com/chrixbedardcad/GhostSpell/releases/latest/download/ghostspell-linux-amd64) |
| **macOS** (Intel) | [GhostSpell-darwin-amd64.dmg](https://github.com/chrixbedardcad/GhostSpell/releases/latest/download/GhostSpell-darwin-amd64.dmg) |
| **macOS** (Apple Silicon) | [GhostSpell-darwin-arm64.dmg](https://github.com/chrixbedardcad/GhostSpell/releases/latest/download/GhostSpell-darwin-arm64.dmg) |

</details>

<details>
<summary>What does the install script do?</summary>

**macOS:** Downloads the `.dmg` for your chip (Intel or Apple Silicon), mounts it, copies `GhostSpell.app` to `/Applications`, removes the Gatekeeper quarantine flag, and opens macOS permission settings.

**Linux:** Downloads the binary to `/usr/local/bin/ghostspell` and checks for required dependencies (`xclip`, `xdotool`, `libwebkit2gtk-4.1`, `libgtk-3`).

**Windows:** Downloads `ghostspell.exe` and `ghostspell-window.exe` to `%LOCALAPPDATA%\GhostSpell\`, adds to PATH, and creates a Start Menu shortcut.

Scripts only download from official GitHub releases — inspect them at [`scripts/install.sh`](scripts/install.sh) and [`scripts/install.ps1`](scripts/install.ps1).
</details>

---

## Quick Start

1. **Install** GhostSpell using the one-liner above
2. **Add a provider** — the setup wizard opens on first launch. You can **Log in with ChatGPT** (one click, no API key needed) or pick any provider and paste an API key.
3. **Use it** — type something in any app, press **Ctrl+G**

That's the whole workflow. GhostSpell auto-detects the language and fixes spelling, grammar, and syntax.

### Prompts

GhostSpell ships with 6 built-in prompts. Switch between them from the tray menu or press your **Cycle Prompt** hotkey.

| Prompt | What it does |
|--------|-------------|
| **Correct** | Fix spelling, grammar, and syntax (default) |
| **Polish** | Improve clarity and flow without changing meaning |
| **Funny** | Rewrite with humor |
| **Elaborate** | Expand on the text with more detail |
| **Shorten** | Make it more concise |
| **Translate** | Translate between configured language pairs |

You can create custom prompts and assign per-prompt LLM overrides in **Settings > Templates**.

### Local AI (Ollama)

No API key needed. Select **Ollama** in the setup wizard — GhostSpell detects your Ollama install, lets you pick a model, and handles everything locally.

---

## Features

- **One hotkey** — Ctrl+G does everything. Optional Cycle Prompt hotkey for power users.
- **Multi-provider** — Anthropic, OpenAI, Gemini, xAI, DeepSeek, Ollama. Use different models per prompt.
- **ChatGPT OAuth** — Log in with your ChatGPT account, no API key needed.
- **Settings GUI** — 5-tab settings panel: Models, Templates, General, About, Debug. No JSON editing required.
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
    { "name": "Correct", "prompt": "Detect the language. Fix spelling and grammar. Return ONLY the corrected text." },
    { "name": "Polish", "prompt": "Improve clarity and flow. Return ONLY the improved text." },
    { "name": "Translate", "prompt": "Translate to {target_language}. Return ONLY the translation.", "llm": "chatgpt" }
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

No dependencies. The installer creates a Start Menu shortcut. Two exe variants: `ghostspell.exe` (tray only, recommended) and `ghostspell-window.exe` (with console for debugging).

### macOS

No dependencies to install. Runs as a menu bar app (no Dock icon). The curl installer handles everything, but macOS requires two manual permission grants:

**1. Allow the app to run (Gatekeeper)**

GhostSpell is not signed with an Apple Developer certificate, so macOS blocks it by default. The curl installer removes the quarantine flag automatically. If you install manually:

Right-click `GhostSpell.app` > **Open** > click **Open** in the security dialog. Or from Terminal:
```bash
xattr -d com.apple.quarantine /Applications/GhostSpell.app
```

**2. Grant permissions**

GhostSpell needs two macOS permissions — these cannot be bypassed, even by signed apps:

- **Accessibility** (System Settings > Privacy & Security > Accessibility) — required for keyboard simulation (Ctrl+A, Ctrl+C, Ctrl+V). [Apple's guide](https://support.apple.com/guide/mac-help/allow-accessibility-apps-to-access-your-mac-mh43185/mac)
- **Input Monitoring** (System Settings > Privacy & Security > Input Monitoring) — required for global hotkeys (Ctrl+G). [Apple's guide](https://support.apple.com/guide/mac-help/control-access-to-input-monitoring-on-mac-mchl4cedafb6/mac)

The installer opens both settings panes automatically. Toggle GhostSpell **ON** in each, then press Enter in the terminal to relaunch.

> **Note:** An Apple Developer account ($99/year) would eliminate the Gatekeeper warning and allow notarization, but Accessibility and Input Monitoring prompts are always required by macOS for any app that simulates keystrokes or listens to global hotkeys.

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

Requires **Go 1.25+**.

```bash
git clone https://github.com/chrixbedardcad/GhostSpell.git
cd GhostSpell
```

**Windows** (CGO_ENABLED=0):
```bash
go build -ldflags "-H=windowsgui" -o ghostspell.exe .     # tray only
go build -o ghostspell-window.exe .                         # with console
```

**Linux** (CGO_ENABLED=1):
```bash
sudo apt install libwebkit2gtk-4.1-dev libgtk-3-dev
CGO_ENABLED=1 go build -tags webkit2_41 -o ghostspell .
```

**macOS** (CGO_ENABLED=1):
```bash
CGO_ENABLED=1 go build -o ghostspell .
```

**Tests:**
```bash
go test ./...
```

---

## Uninstall

**macOS / Linux:**
```bash
curl -fsSL https://raw.githubusercontent.com/chrixbedardcad/GhostSpell/main/scripts/uninstall.sh | bash
```

**Windows:**
```cmd
powershell -c "irm https://raw.githubusercontent.com/chrixbedardcad/GhostSpell/main/scripts/uninstall.ps1 | iex"
```

> Back up `config.json` first if you want to keep your settings.

### Migrating from GhostType (pre-v0.2.0)

GhostSpell was previously named GhostType. If you have the old version installed, uninstall it first, then install GhostSpell.

**Migrate your config (optional — do this before uninstalling):**

macOS:
```bash
mkdir -p ~/Library/Application\ Support/GhostSpell
cp ~/Library/Application\ Support/GhostType/config.json ~/Library/Application\ Support/GhostSpell/config.json
```

Windows (PowerShell):
```powershell
New-Item -ItemType Directory -Force "$env:APPDATA\GhostSpell"
Copy-Item "$env:APPDATA\GhostType\config.json" "$env:APPDATA\GhostSpell\config.json"
```

Linux:
```bash
mkdir -p ~/.config/GhostSpell
cp ~/.config/GhostType/config.json ~/.config/GhostSpell/config.json
```

<blockquote>
<h3>:rotating_light: <span style="color:red">Uninstall old GhostType (SUPERCLEANER)</span> :rotating_light:</h3>
</blockquote>

> **Warning** — These scripts **permanently delete** all GhostType files, caches, and references. Back up `config.json` first if needed.

<details>
<summary><b style="color:red">macOS — Nuclear Supercleaner</b></summary>

```bash
#!/bin/bash
# GhostType SUPERCLEANER for macOS — removes every trace
set -e

echo "=== GhostType Supercleaner ==="
echo ""

# 1. Kill all GhostType processes
echo "[1/8] Killing GhostType processes..."
pkill -9 -f GhostType 2>/dev/null || true
pkill -9 -f ghosttype 2>/dev/null || true
sleep 1

# 2. Find and remove ALL GhostType .app bundles (everywhere)
echo "[2/8] Removing GhostType.app from all locations..."
sudo rm -rf /Applications/GhostType.app
rm -rf "$HOME/Applications/GhostType.app"
# Find any other copies via Spotlight index
mdfind "kMDItemDisplayName == 'GhostType'" 2>/dev/null | while read -r f; do
    echo "  Found: $f"
    sudo rm -rf "$f"
done

# 3. Unregister from Launch Services
echo "[3/8] Unregistering from Launch Services..."
LSREGISTER="/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister"
$LSREGISTER -u /Applications/GhostType.app 2>/dev/null || true
$LSREGISTER -u "$HOME/Applications/GhostType.app" 2>/dev/null || true

# 4. Remove all app data, config, preferences, caches
echo "[4/8] Removing app data, preferences, caches..."
rm -rf "$HOME/Library/Application Support/GhostType"
rm -rf "$HOME/Library/Preferences/com.ghosttype.app.plist"
rm -rf "$HOME/Library/Preferences/com.ghosttype.plist"
rm -rf "$HOME/Library/Saved Application State/com.ghosttype.app.savedState"
rm -rf "$HOME/Library/Caches/com.ghosttype.app"
rm -rf "$HOME/Library/Caches/com.ghosttype"
rm -rf "$HOME/Library/HTTPStorages/com.ghosttype.app"
rm -rf "$HOME/Library/HTTPStorages/com.ghosttype"
rm -rf "$HOME/Library/WebKit/com.ghosttype.app"
rm -rf "$HOME/Library/Logs/GhostType"

# 5. Remove lock files
echo "[5/8] Removing lock files..."
rm -f "$HOME/.ghosttype.lock"
find /tmp -name "*ghosttype*" -exec rm -rf {} + 2>/dev/null || true

# 6. Clean Launchpad database
echo "[6/8] Cleaning Launchpad database..."
LPDB=$(find /private/var/folders -name "com.apple.dock.launchpad" -type d 2>/dev/null | head -1)
if [ -n "$LPDB" ] && [ -f "$LPDB/db/db" ]; then
    sqlite3 "$LPDB/db/db" "DELETE FROM apps WHERE title LIKE '%GhostType%';" 2>/dev/null || true
    sqlite3 "$LPDB/db/db" "DELETE FROM apps WHERE title LIKE '%ghosttype%';" 2>/dev/null || true
    echo "  Launchpad entries removed."
fi

# 7. Reset Launch Services database and refresh Dock
echo "[7/8] Resetting Launch Services and Dock..."
$LSREGISTER -kill -r -domain local -domain system -domain user 2>/dev/null || true
killall Dock 2>/dev/null || true
killall Finder 2>/dev/null || true
sleep 2

# 8. Verify removal
echo "[8/8] Verifying..."
REMAINING=$(mdfind "GhostType" -name 2>/dev/null | grep -i ghosttype || true)
if [ -z "$REMAINING" ]; then
    echo ""
    echo "GhostType has been COMPLETELY removed."
else
    echo ""
    echo "Some references may still exist (Spotlight may take a moment to update):"
    echo "$REMAINING"
fi

echo ""
echo "MANUAL STEP: Open System Settings > Privacy & Security and remove GhostType from:"
echo "  - Accessibility"
echo "  - Input Monitoring"
echo ""
echo "Done."
```

</details>

<details>
<summary><b style="color:red">Windows — Nuclear Supercleaner</b> (run as Administrator)</summary>

```powershell
# GhostType SUPERCLEANER for Windows — removes every trace
# Run this in PowerShell as Administrator

Write-Host ""
Write-Host "=== GhostType Supercleaner ===" -ForegroundColor Red
Write-Host ""

# 1. Kill all processes
Write-Host "[1/7] Killing GhostType processes..." -ForegroundColor Yellow
Get-Process -Name "ghosttype*" -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Sleep 1

# 2. Remove binaries
Write-Host "[2/7] Removing binaries..." -ForegroundColor Yellow
Remove-Item -Recurse -Force "$env:LOCALAPPDATA\GhostType" -ErrorAction SilentlyContinue
# Check Program Files too
Remove-Item -Recurse -Force "$env:ProgramFiles\GhostType" -ErrorAction SilentlyContinue
Remove-Item -Recurse -Force "${env:ProgramFiles(x86)}\GhostType" -ErrorAction SilentlyContinue

# 3. Remove app data and config
Write-Host "[3/7] Removing app data and config..." -ForegroundColor Yellow
Remove-Item -Recurse -Force "$env:APPDATA\GhostType" -ErrorAction SilentlyContinue

# 4. Remove all shortcuts
Write-Host "[4/7] Removing shortcuts..." -ForegroundColor Yellow
Remove-Item -Force "$env:APPDATA\Microsoft\Windows\Start Menu\Programs\GhostType.lnk" -ErrorAction SilentlyContinue
Remove-Item -Force "$env:APPDATA\Microsoft\Windows\Start Menu\Programs\Startup\GhostType.lnk" -ErrorAction SilentlyContinue
Remove-Item -Force "$env:USERPROFILE\Desktop\GhostType.lnk" -ErrorAction SilentlyContinue
Remove-Item -Force "$env:PUBLIC\Desktop\GhostType.lnk" -ErrorAction SilentlyContinue
# Search Start Menu folders for any ghosttype shortcuts
Get-ChildItem "$env:APPDATA\Microsoft\Windows\Start Menu" -Recurse -Filter "*ghosttype*" -ErrorAction SilentlyContinue | Remove-Item -Force

# 5. Remove from PATH
Write-Host "[5/7] Cleaning PATH..." -ForegroundColor Yellow
$p = [Environment]::GetEnvironmentVariable("PATH", "User")
if ($p -like "*GhostType*") {
    $newPath = ($p -split ";" | Where-Object { $_ -notlike "*GhostType*" -and $_ -ne "" }) -join ";"
    [Environment]::SetEnvironmentVariable("PATH", $newPath, "User")
    Write-Host "  Removed GhostType from user PATH." -ForegroundColor Cyan
}
$sp = [Environment]::GetEnvironmentVariable("PATH", "Machine")
if ($sp -like "*GhostType*") {
    $newSPath = ($sp -split ";" | Where-Object { $_ -notlike "*GhostType*" -and $_ -ne "" }) -join ";"
    [Environment]::SetEnvironmentVariable("PATH", $newSPath, "Machine")
    Write-Host "  Removed GhostType from system PATH." -ForegroundColor Cyan
}

# 6. Clean registry
Write-Host "[6/7] Cleaning registry..." -ForegroundColor Yellow
Remove-Item -Path "HKCU:\Software\GhostType" -Recurse -ErrorAction SilentlyContinue
$runKeys = @(
    "HKCU:\Software\Microsoft\Windows\CurrentVersion\Run",
    "HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Run",
    "HKCU:\Software\Microsoft\Windows\CurrentVersion\RunOnce",
    "HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\RunOnce"
)
foreach ($rk in $runKeys) {
    if (Test-Path $rk) {
        $props = Get-ItemProperty $rk -ErrorAction SilentlyContinue
        foreach ($name in ($props.PSObject.Properties | Where-Object { $_.Name -like "*GhostType*" -or $_.Name -like "*ghosttype*" }).Name) {
            Remove-ItemProperty -Path $rk -Name $name -ErrorAction SilentlyContinue
            Write-Host "  Removed $name from $rk" -ForegroundColor Cyan
        }
    }
}
# App Paths
Remove-Item -Path "HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\ghosttype.exe" -Recurse -ErrorAction SilentlyContinue
Remove-Item -Path "HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\ghosttype-window.exe" -Recurse -ErrorAction SilentlyContinue
# Uninstall entries
Get-ChildItem "HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall" -ErrorAction SilentlyContinue |
    Where-Object { $_.GetValue("DisplayName") -like "*GhostType*" } |
    ForEach-Object { Remove-Item $_.PSPath -Recurse -Force; Write-Host "  Removed uninstall entry: $($_.GetValue('DisplayName'))" -ForegroundColor Cyan }

# 7. Clean temp and lock files
Write-Host "[7/7] Cleaning temp and lock files..." -ForegroundColor Yellow
Remove-Item -Recurse -Force "$env:TEMP\ghosttype*" -ErrorAction SilentlyContinue
Remove-Item -Recurse -Force "$env:TEMP\GhostType*" -ErrorAction SilentlyContinue
Remove-Item -Force "$env:LOCALAPPDATA\.ghosttype.lock" -ErrorAction SilentlyContinue
Remove-Item -Force "$env:APPDATA\.ghosttype.lock" -ErrorAction SilentlyContinue

Write-Host ""
Write-Host "GhostType has been COMPLETELY removed." -ForegroundColor Green
Write-Host ""
```

</details>

<details>
<summary><b style="color:red">Linux — Nuclear Supercleaner</b></summary>

```bash
#!/bin/bash
# GhostType SUPERCLEANER for Linux — removes every trace

echo "=== GhostType Supercleaner ==="
echo ""

# Kill all processes
pkill -9 -f ghosttype 2>/dev/null || true
sleep 1

# Remove binary
sudo rm -f /usr/local/bin/ghosttype
sudo rm -f /usr/bin/ghosttype

# Remove config, data, lock files
rm -rf "$HOME/.config/GhostType"
rm -f "$HOME/.ghosttype.lock"
find /tmp -name "*ghosttype*" -exec rm -rf {} + 2>/dev/null || true

# Remove desktop entries and icons
rm -f "$HOME/.local/share/applications/ghosttype.desktop"
rm -f "$HOME/.local/share/applications/GhostType.desktop"
find "$HOME/.local/share/icons" -name "*ghosttype*" -delete 2>/dev/null || true

# Update desktop database
update-desktop-database "$HOME/.local/share/applications" 2>/dev/null || true

echo ""
echo "GhostType has been COMPLETELY removed."
```

</details>

Then install GhostSpell using the [install instructions](#install) above.

---

## Troubleshooting

| Problem | Solution |
|---------|----------|
| Hotkeys don't work | Check that GhostSpell is running (look for tray icon). On macOS, grant Input Monitoring permission. |
| API errors | Verify API key in Settings > Models. Check that your provider account has credits. |
| Slow responses | Try a faster model, or switch to local Ollama. |
| **Linux**: Missing dependencies | `sudo apt install libwebkit2gtk-4.1-0 libgtk-3-0 xclip xdotool` |
| **macOS**: Keyboard simulation fails | Grant Accessibility permission in System Settings > Privacy & Security. |

---

## License

MIT

## Acknowledgments

Inspired by [Grammarly](https://www.grammarly.com/), [LanguageTool](https://languagetool.org/), [Espanso](https://espanso.org/), [Raycast AI](https://www.raycast.com/), and macOS inline autocorrect.
