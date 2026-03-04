//go:build windows

package main

import (
	"runtime"

	"github.com/chrixbedardcad/GhostType/clipboard"
	"github.com/chrixbedardcad/GhostType/hotkey"
	"github.com/chrixbedardcad/GhostType/keyboard"
)

func newClipboard() *clipboard.Clipboard  { return clipboard.NewWindowsClipboard() }
func newKeyboard() keyboard.Simulator     { return keyboard.NewWindowsSimulator() }
func newHotkeyManager() hotkey.Manager    { return hotkey.NewWindowsManager() }

// startMainLoop preserves Windows behavior: register hotkeys on the current
// thread, run Wails in a background goroutine, then lock this thread for the
// Windows message loop (RegisterHotKey + GetMessageW).
func startMainLoop(trayRun func() error, registerHotkeys func() error, hk hotkey.Manager) {
	registerHotkeys()
	go func() { trayRun() }()
	runtime.LockOSThread()
	hk.Listen()
}
