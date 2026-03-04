//go:build linux

package main

import (
	"github.com/chrixbedardcad/GhostType/clipboard"
	"github.com/chrixbedardcad/GhostType/hotkey"
	"github.com/chrixbedardcad/GhostType/keyboard"
)

func newClipboard() *clipboard.Clipboard  { return clipboard.NewLinuxClipboard() }
func newKeyboard() keyboard.Simulator     { return keyboard.NewLinuxSimulator() }
func newHotkeyManager() hotkey.Manager    { return hotkey.NewXPlatManager() }

// startMainLoop preserves Linux behavior: register hotkeys, run Wails in a
// background goroutine, then block on the hotkey listener.
func startMainLoop(trayRun func() error, registerHotkeys func() error, hk hotkey.Manager) {
	registerHotkeys()
	go func() { trayRun() }()
	hk.Listen()
}
