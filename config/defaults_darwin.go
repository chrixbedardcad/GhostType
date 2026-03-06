//go:build darwin

package config

// defaultCorrectHotkey is the default hotkey for the "correct" action.
// On macOS, Ctrl maps to Cmd (⌘), so "Ctrl+Shift+G" becomes ⌘⇧G.
// Plain "Ctrl+G" (⌘G) conflicts with the system-wide "Find Next" shortcut.
const defaultCorrectHotkey = "Ctrl+Shift+G"
