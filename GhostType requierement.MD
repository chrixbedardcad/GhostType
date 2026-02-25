LLM AutoCorrect Service — Full Project Specification v2.0

Project Name

LLM AutoCorrect (working title: GhostType)

Overview

A cross-platform background service written in Go that intercepts keyboard input in any target application (primarily Firestorm Second Life viewer), detects the language being typed, and provides three AI-powered modes: grammar/spelling correction, automatic translation, and creative rewriting via configurable LLM prompts. The service displays results through a lightweight overlay window and allows the user to accept or dismiss corrections via hotkeys. It supports multiple LLM providers configured through a JSON file.



Market Context — Existing Tools and Why This Is Different

What Exists Today

OS-Level Autocorrect: Windows built-in autocorrect works only in UWP/Office apps, limited to predefined word replacements, no AI. macOS built-in autocorrect is system-wide with a clean inline bubble UX and multi-language support — the gold standard for unobtrusive correction. Linux has IBus/Fcitx input method frameworks primarily for CJK, inconsistent across desktop environments.

AI Correction Tools: Grammarly is the market leader with a floating widget, color-coded underlines, and correction cards — works on Windows and Mac but not Linux at OS level. LanguageTool is open source, supports 30+ languages with auto-detection, has a local server mode and API. ProWritingAid and Ginger Software are desktop apps focused on long-form writing. WhiteSmoke includes grammar, style, and translation.

Text Expansion Tools: Espanso is open source, cross-platform, written in Rust, runs as a background service with trigger-based text replacement — the closest architecture to this project. AutoHotkey is Windows-only but extremely powerful for hotkeys and automation. TextExpander, PhraseExpress, and aText are commercial text expansion tools.

AI Writing Assistants: Raycast AI (Mac only) offers hotkey-triggered rewrite, translate, and fix grammar from any app — very close to this concept. WritingTools by Jesai is an open source project replicating Apple Intelligence writing tools on Windows using Gemini or local LLM.

What Makes GhostType Different



Multi-mode: correct, translate, and creative rewrite in one tool

Multi-provider: swap between Claude, GPT, Gemini, Grok, or local Ollama via config

Multi-language: automatic detection and correction for French and English (expandable)

Runs as a background service with overlay — no browser extension required

Cross-platform from a single codebase (Windows first, then Mac and Linux)

Open config with custom prompt templates — users define their own rewrite styles

Designed specifically for virtual world chat (Firestorm) but works anywhere





Three Operating Modes

Mode 1 — Correct (F6)

Detects the language of the typed text (French or English). Fixes all spelling, grammar, and syntax errors. Preserves the original meaning and language. Returns only the corrected text. This is the core use case.

Mode 2 — Translate (F7)

Translates the typed text to the currently selected target language. Auto-detects source language. User configures default target language in config.json and can toggle through the languages list at any time with Ctrl+F7 — a brief floating label near the cursor confirms the new target (e.g., "To French"). Useful for switching between French and English chat in Firestorm.

Mode 3 — Rewrite (F8)

Sends the typed text to the LLM with a creative prompt template. User toggles through available templates with Ctrl+F8 — a brief floating label near the cursor shows the newly selected template name (e.g., "Funny", "Professional"). Templates are fully configurable in config.json. Examples: funny reply, formal tone, sarcastic, flirty, poetic, ELI5, pirate speak, or any custom prompt the user defines.



Hotkey Workflow

Full Hotkey Map



F6: Capture text and send for correction (Mode 1)

Ctrl+F7: Toggle translation target language. Cycles through the languages list in config.json. Shows a brief cursor-near notification displaying the new target language name (e.g., "To French", "To English"). Does not perform any text action, just switches the target.

F7: Capture text and send for translation to the currently selected target language (Mode 2)

Ctrl+F8: Toggle rewrite template. Cycles through the rewrite_templates list in config.json. Shows a brief cursor-near notification displaying the template name (e.g., "Funny", "Professional", "Casual"). Does not perform any text action, just switches the active template.

F8: Capture text and send for rewrite using active template (Mode 3)

Escape: Cancel in-progress operation

Ctrl+Z: Undo replacement (native, handled by target app)



Detailed Flow



User types text in target application chat input field.

Optionally, user presses Ctrl+F7 to toggle the translation target language or Ctrl+F8 to toggle the rewrite template. A brief floating label appears near the cursor confirming the selection (e.g., "To French" or "Funny").

User presses the hotkey for the desired mode (F6, F7, or F8).

Service selects all text in chat input (Ctrl+A), copies to clipboard (Ctrl+C), reads clipboard.

Service sends text to configured LLM API with the appropriate prompt for the selected mode.

Service receives the result from the LLM.

Overlay window appears near the chat input showing the result. For correction mode, changed words are highlighted. For translation mode, the full translation is shown. For rewrite mode, the rewritten text is shown with the template name.

Result auto-replaces the original text. User presses Ctrl+Z (native undo) to revert if needed.

User presses Escape to cancel: clears stored result, hides overlay, no changes made.





Default Hotkey Rationale

F6, F7, and F8 were chosen as the default hotkeys because they are three consecutive function keys that are virtually unused across Windows, Linux, and macOS applications, making them safe defaults with near-zero conflict risk.

Why other function keys were avoided:

F1 — Help in nearly every application
F2 — Rename (Explorer, IDEs, spreadsheets)
F3 — Search/Find in browsers, Explorer, and many editors
F4 — Mostly safe alone, but Alt+F4 closes windows, creating accidental-close risk
F5 — Refresh in browsers and Explorer
F10 — Activates the menu bar in most Windows applications
F11 — Fullscreen toggle in browsers and many apps
F12 — Developer tools in browsers, Save As in Office

F6, F7, and F8 have no standard system-wide or application-wide bindings, making them ideal for a background service that must coexist with other software.

All hotkeys are fully configurable in config.json, so users can remap them to any key combination if needed.



Architecture

High-Level Flow



Service starts on boot or manually, reads config.json.

Service runs in background, monitoring for hotkey presses when target window is active.

User can press Ctrl+F7 to toggle the translation target language or Ctrl+F8 to toggle the rewrite template. A brief floating label appears near the cursor confirming the selection.

On action hotkey press (F6, F7, F8), service captures text, determines mode, sends to LLM API.

LLM returns result, service displays in overlay.

User accepts or dismisses via hotkey.



Components

Input Hook Module — Platform-specific keyboard hooking. Windows: Win32 SetWindowsHookEx low-level keyboard hook. Linux: X11/XCB input event capture (Wayland as stretch goal). macOS: Quartz Event Services (CGEventTap).

Window Detection Module — Detects if the target application is the active/focused window. Windows: GetForegroundWindow + window title matching. Linux: X11 \_NET\_ACTIVE\_WINDOW. macOS: NSWorkspace active application detection. Target window name is configurable (default: "Firestorm").

Text Capture Module — Grabs current chat input text via clipboard. Selects all text in the active input field (Ctrl+A / Cmd+A), copies to clipboard, reads clipboard content. Saves and restores original clipboard content to avoid disrupting user workflow.

Cursor Notification Module — Displays a tiny, semi-transparent floating label near the mouse cursor position when the user presses Ctrl+F7 or Ctrl+F8. The label shows the name of the newly selected language (e.g., "To French") or template (e.g., "Funny"). It appears for 2 seconds and fades out. On Windows this is implemented as a small borderless, always-on-top, transparent window positioned near the cursor using GetCursorPos. This is NOT a full overlay, just a minimal floating label.

Mode Router — Determines which prompt to use based on the hotkey pressed. Routes to correction, translation, or rewrite prompt. Manages the active rewrite template selection and active translation target language.

LLM API Client Module — Sends requests to configured provider. Supports: OpenAI, Anthropic (Claude), Google Gemini, xAI (Grok), Ollama (local). Handles authentication, request formatting, response parsing per provider. Implements timeout and retry logic. Provider-agnostic interface so adding new providers is straightforward.

Overlay Module — Displays results in a transparent, always-on-top, borderless overlay window. Positioned near the target application's chat input area or near the cursor. Shows original vs corrected text with highlighted differences (correction mode). Shows translated text with source/target language labels (translation mode). Shows rewritten text with active template name (rewrite mode). Shows a small mode indicator icon. Auto-dismisses after configurable timeout. Built with Fyne toolkit for cross-platform rendering.

Text Replacement Module — Inserts result back into target application chat input. Selects all text, pastes result from memory. Restores original clipboard content after replacement.

Configuration Manager — Reads and validates config.json on startup. Watches config file for changes and hot-reloads without restart. Generates default config.json if none exists.

System Tray Module — System tray icon showing service status. Menu with: current mode display, enable/disable toggle, open config file, view log, quit. Status indicator: green (idle/ready), yellow (processing), red (error).



Configuration File (config.json)

{

&nbsp; "llm\_provider": "anthropic",

&nbsp; "api\_key": "sk-ant-xxxxx",

&nbsp; "model": "claude-sonnet-4-5-20250929",

&nbsp; "api\_endpoint": "",

&nbsp; "languages": \["en", "fr"],

&nbsp; "language\_names": {

&nbsp;   "en": "English",

&nbsp;   "fr": "French"

&nbsp; },

&nbsp; "default\_translate\_target": "en",

&nbsp; "hotkeys": {

&nbsp;   "correct": "F6",

&nbsp;   "translate": "F7",

&nbsp;   "toggle\_language": "Ctrl+F7",

&nbsp;   "rewrite": "F8",

&nbsp;   "cycle\_template": "Ctrl+F8",

&nbsp;   "cancel": "Escape"

&nbsp; },

&nbsp; "target\_window": "Firestorm",

&nbsp; "prompts": {

&nbsp;   "correct": "Detect the language of the following text (French or English). Fix all spelling and grammar errors while preserving the original meaning and language. Return ONLY the corrected text with no explanation.",

&nbsp;   "translate": "Translate the following text to {target\_language}. Preserve the tone and intent. Return ONLY the translation with no explanation.",

&nbsp;   "rewrite\_templates": \[

&nbsp;     {

&nbsp;       "name": "funny",

&nbsp;       "prompt": "Rewrite this as a funny, witty reply. Keep it short and punchy. Return ONLY the rewritten text."

&nbsp;     },

&nbsp;     {

&nbsp;       "name": "formal",

&nbsp;       "prompt": "Rewrite this in a formal, professional tone. Return ONLY the rewritten text."

&nbsp;     },

&nbsp;     {

&nbsp;       "name": "sarcastic",

&nbsp;       "prompt": "Rewrite this with heavy sarcasm. Return ONLY the rewritten text."

&nbsp;     },

&nbsp;     {

&nbsp;       "name": "flirty",

&nbsp;       "prompt": "Rewrite this in a playful, flirty tone. Return ONLY the rewritten text."

&nbsp;     },

&nbsp;     {

&nbsp;       "name": "poetic",

&nbsp;       "prompt": "Rewrite this as if you were a romantic poet. Return ONLY the rewritten text."

&nbsp;     },

&nbsp;     {

&nbsp;       "name": "eli5",

&nbsp;       "prompt": "Rewrite this so a 5-year-old could understand it. Return ONLY the rewritten text."

&nbsp;     },

&nbsp;     {

&nbsp;       "name": "pirate",

&nbsp;       "prompt": "Rewrite this as a pirate would say it. Return ONLY the rewritten text."

&nbsp;     }

&nbsp;   ]

&nbsp; },

&nbsp; "overlay": {

&nbsp;   "enabled": true,

&nbsp;   "position": "above\_chat",

&nbsp;   "opacity": 0.85,

&nbsp;   "auto\_dismiss\_seconds": 10,

&nbsp;   "show\_mode\_indicator": true,

&nbsp;   "highlight\_changes": true,

&nbsp;   "font\_size": 14

&nbsp; },

&nbsp; "max\_tokens": 256,

&nbsp; "timeout\_ms": 5000,

&nbsp; "preserve\_clipboard": true,

&nbsp; "log\_level": "info",

&nbsp; "log\_file": "autocorrect.log"

}

Supported LLM Providers

OpenAI — llm\_provider: "openai" — default endpoint: https://api.openai.com/v1/chat/completions

Anthropic — llm\_provider: "anthropic" — default endpoint: https://api.anthropic.com/v1/messages

Google Gemini — llm\_provider: "gemini" — default endpoint: https://generativelanguage.googleapis.com/v1/models

xAI (Grok) — llm\_provider: "xai" — default endpoint: https://api.x.ai/v1/chat/completions

Ollama (local) — llm\_provider: "ollama" — default endpoint: http://localhost:11434/api/generate

If api\_endpoint is provided in config, it overrides the default endpoint for the selected provider. This allows pointing to custom proxies, local instances, or alternative API-compatible services.



Release Plan

v0.1 — MVP (Windows Only, Simple App)

This is a simple Windows desktop application, NOT a service. It runs as a visible window with a system tray icon. No overlay — results are shown in the app window itself.

Scope:



Windows desktop application with a minimal GUI window

System tray icon for quick access

JSON config file for LLM provider, API key, model, hotkeys

F6 hotkey for correction mode only

Escape to cancel

Support for 2 LLM providers: Anthropic (Claude) and OpenAI

Automatic French/English language detection via LLM prompt

Target window detection (only active when Firestorm is focused)

Results displayed in the app window (no overlay yet)

Logging to file

Basic error handling (API timeout, invalid key, no network)



v0.2 — Translation and Overlay



Add translation mode (F7)

Ctrl+F7 toggle translation target language with cursor notification

Transparent overlay window replaces the app window for showing results

Overlay positioned near target application chat input

Diff highlighting for correction mode

Add Ollama local LLM support

Linux support (X11)



v0.3 — Rewrite Mode and Polish



Add rewrite mode (F8) with configurable templates

Ctrl+F8 toggle rewrite template with cursor notification

Template cycling (Ctrl+F8)

Config hot-reload without restart

Clipboard preservation (save/restore original clipboard)

Correction history and undo stack

macOS support



v0.4 — Advanced Features



Add Gemini and xAI provider support

GUI configuration panel (edit config without touching JSON)

Support for additional languages beyond French/English

Wayland support on Linux

Auto-update mechanism



v0.5 — Power User Features



Real-time auto-correction mode (no hotkey, Grammarly-style)

Inline underline overlay if target app supports it

Custom provider plugin system

Usage statistics and cost tracking per provider





Technical Requirements

Build and Deployment



Language: Go 1.22+

Build target: Single static binary for Windows (MVP)

GUI toolkit: Fyne (pure Go, cross-platform) for the app window and future overlay

No CGO dependencies if possible

Build commands: go build -o ghosttype.exe (Windows), go build -o ghosttype (Linux/Mac)

Installer: simple zip distribution for MVP, MSI/NSIS installer for v0.3+



Dependencies (Go modules)



fyne.io/fyne/v2 — Cross-platform GUI and overlay rendering

github.com/moutend/go-hook or golang.org/x/sys/windows — Windows keyboard hooks

golang.design/x/clipboard — Cross-platform clipboard access

net/http (stdlib) — HTTP client for LLM API calls

encoding/json (stdlib) — JSON config parsing

github.com/getlantern/systray — System tray icon

log/slog (stdlib) — Structured logging

github.com/fsnotify/fsnotify — Config file watching for hot-reload (v0.3)



Performance Targets



Hotkey response time: < 50ms from keypress to API call initiated

Total correction cycle (F6 to result ready): < 3 seconds (network dependent)

Overlay render time: < 100ms

Memory usage: < 50MB RSS

CPU usage at idle: < 1%





Error Handling



Invalid API key: Log error, show notification in app window/tray, continue running

API timeout: Retry once after 2 seconds, then show "Correction timed out" message

No network: Show notification, allow retry when network returns

Target app not running: Service idles with minimal CPU, resumes when target app starts

Config file missing: Generate default config.json template and show setup instructions

Config file invalid: Log specific validation error, show message, use last valid config

Empty text captured: Show "Nothing to correct" message, no API call made

LLM returns empty or error: Show error in app window, log details





Logging



Log file: configurable path in config.json, default ghosttype.log

Log levels: debug, info, warn, error

Log format: \[TIMESTAMP] \[LEVEL] \[MODULE] message

Log rotation: not required for MVP

Debug mode logs full API requests and responses (with API key redacted)





Testing Plan



Unit tests for JSON config parsing and validation

Unit tests for LLM API request/response formatting per provider (Anthropic, OpenAI)

Unit tests for mode routing logic

Integration test: mock LLM API, verify full F6 correction cycle

Integration test: mock LLM API, verify F7 translation cycle

Manual testing in Firestorm on Windows

Manual testing with each supported LLM provider

Performance test: measure hotkey-to-result latency





File Structure

ghosttype/

&nbsp; main.go                  — Entry point, app initialization

&nbsp; config/

&nbsp;   config.go              — JSON config loading, validation, defaults

&nbsp;   config\_test.go         — Config unit tests

&nbsp; hooks/

&nbsp;   hooks.go               — Interface for input hooks

&nbsp;   hooks\_windows.go       — Windows keyboard hooks (MVP)

&nbsp;   hooks\_linux.go         — Linux X11 hooks (v0.2)

&nbsp;   hooks\_darwin.go        — macOS hooks (v0.3)

&nbsp; window/

&nbsp;   window.go              — Interface for window detection

&nbsp;   window\_windows.go      — Windows implementation (MVP)

&nbsp;   window\_linux.go        — Linux implementation (v0.2)

&nbsp;   window\_darwin.go       — macOS implementation (v0.3)

&nbsp; clipboard/

&nbsp;   clipboard.go           — Clipboard read/write with preservation

&nbsp; cursor/

&nbsp;   cursor.go              — Interface for cursor position and floating label

&nbsp;   cursor\_windows.go      — Windows implementation using GetCursorPos and borderless window

&nbsp; mode/

&nbsp;   router.go              — Mode selection based on hotkey

&nbsp;   correct.go             — Correction mode logic

&nbsp;   translate.go           — Translation mode logic

&nbsp;   rewrite.go             — Rewrite mode logic and template cycling

&nbsp; llm/

&nbsp;   client.go              — LLM client interface

&nbsp;   anthropic.go           — Anthropic Claude API implementation

&nbsp;   anthropic\_test.go      — Anthropic unit tests

&nbsp;   openai.go              — OpenAI API implementation

&nbsp;   openai\_test.go         — OpenAI unit tests

&nbsp;   gemini.go              — Google Gemini implementation (v0.4)

&nbsp;   xai.go                 — xAI Grok implementation (v0.4)

&nbsp;   ollama.go              — Ollama local implementation (v0.2)

&nbsp; overlay/

&nbsp;   overlay.go             — Overlay window rendering (v0.2)

&nbsp;   diff.go                — Text diff highlighting logic (v0.2)

&nbsp; ui/

&nbsp;   app.go                 — Main app window (MVP)

&nbsp;   tray.go                — System tray icon and menu

&nbsp; assets/

&nbsp;   icon.png               — App icon

&nbsp;   icon.ico               — Windows icon

&nbsp; config.json              — Default configuration file

&nbsp; go.mod

&nbsp; go.sum

&nbsp; README.md

&nbsp; LICENSE



How to Use This Spec with Claude Code

Open Claude Code in your project directory and paste this prompt:

"Read the specification document GhostType\_Spec.md and build the MVP (v0.1). This is a Windows desktop application in Go. Start with the project structure and go.mod, then implement each module in this order: (1) config loading and validation with tests, (2) LLM client interface with Anthropic and OpenAI implementations and tests, (3) clipboard read/write, (4) Windows window detection for Firestorm, (5) Windows keyboard hooks for F6 and Escape, (6) main app window UI showing results, (7) system tray icon, (8) main.go wiring everything together. Only implement correction mode (F6) for the MVP. Target Windows only. Use Fyne for the GUI."



License

MIT

Author

Chris

