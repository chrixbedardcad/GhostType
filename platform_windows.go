//go:build windows

package main

import (
	"os"
	"runtime"

	"github.com/chrixbedardcad/GhostType/clipboard"
	"github.com/chrixbedardcad/GhostType/hotkey"
	"github.com/chrixbedardcad/GhostType/keyboard"
)

func newClipboard() *clipboard.Clipboard  { return clipboard.NewWindowsClipboard() }
func newKeyboard() keyboard.Simulator     { return keyboard.NewWindowsSimulator() }
func newHotkeyManager() hotkey.Manager    { return hotkey.NewWindowsManager() }

// startMainLoop starts the Wails event loop in a background goroutine first
// (so the wizard window can render if needed), then registers hotkeys on the
// current thread (which may block waiting for the wizard), then locks this
// thread for the Windows message loop (RegisterHotKey + GetMessageW).
func startMainLoop(trayRun func() error, registerHotkeys func() error, hk hotkey.Manager) {
	go func() {
		// LockOSThread is required because Wails' initMainLoop() and
		// runMainLoop() (both inside app.Run) must execute on the same
		// OS thread — otherwise runMainLoop panics.
		runtime.LockOSThread()
		trayRun()
	}()
	if err := registerHotkeys(); err != nil {
		os.Exit(1)
	}
	runtime.LockOSThread()
	hk.Listen()
}
