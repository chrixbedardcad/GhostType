//go:build linux

package main

import (
	"os"
	"runtime"

	"github.com/chrixbedardcad/GhostType/clipboard"
	"github.com/chrixbedardcad/GhostType/hotkey"
	"github.com/chrixbedardcad/GhostType/keyboard"
)

func newClipboard() *clipboard.Clipboard  { return clipboard.NewLinuxClipboard() }
func newKeyboard() keyboard.Simulator     { return keyboard.NewLinuxSimulator() }
func newHotkeyManager() hotkey.Manager    { return hotkey.NewXPlatManager() }

// startMainLoop starts the GTK event loop in a background goroutine first (so
// the wizard window can render if needed), then registers hotkeys (which may
// block waiting for the wizard to complete), then blocks on the hotkey listener.
func startMainLoop(trayRun func() error, registerHotkeys func() error, hk hotkey.Manager) {
	go func() {
		// LockOSThread keeps GTK's g_application_run on a single OS thread
		// for the lifetime of the event loop (matches old tray.Start behaviour).
		runtime.LockOSThread()
		trayRun()
	}()
	if err := registerHotkeys(); err != nil {
		os.Exit(1)
	}
	hk.Listen()
}
