//go:build linux

package main

import (
	"fmt"
	"log/slog"
	"os"
	"runtime"

	"github.com/chrixbedardcad/GhostType/clipboard"
	"github.com/chrixbedardcad/GhostType/hotkey"
	"github.com/chrixbedardcad/GhostType/keyboard"
)

func newClipboard() *clipboard.Clipboard  { return clipboard.NewLinuxClipboard() }
func newKeyboard() keyboard.Simulator     { return keyboard.NewLinuxSimulator() }
func newHotkeyManager() hotkey.Manager    { return hotkey.NewXPlatManager() }
func initKeyboard()                       {} // no-op on Linux

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
		fmt.Fprintf(os.Stderr, "Hotkey registration failed: %v\n", err)
		slog.Error("Hotkey registration failed — app remains running for diagnostics", "error", err)
		select {} // block forever; tray stays alive for Settings access
	}
	hk.Listen()
}

// restartHotkeyListener starts a new hotkey listener goroutine.
func restartHotkeyListener(hk hotkey.Manager, register func() error) {
	go func() {
		if err := register(); err != nil {
			slog.Error("Failed to re-register hotkeys", "error", err)
			fmt.Fprintf(os.Stderr, "Hotkey re-registration failed: %v\n", err)
			return
		}
		hk.Listen()
	}()
}
