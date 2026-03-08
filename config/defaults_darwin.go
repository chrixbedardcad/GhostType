//go:build darwin

package config

// defaultActionHotkey is the default hotkey for the main action.
// On macOS, Ctrl maps to Cmd (⌘), so "Ctrl+G" becomes ⌘G.
const defaultActionHotkey = "Ctrl+G"
