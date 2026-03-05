//go:build darwin

package main

import (
	"os"

	"github.com/chrixbedardcad/GhostType/clipboard"
	"github.com/chrixbedardcad/GhostType/hotkey"
	"github.com/chrixbedardcad/GhostType/keyboard"
)

func newClipboard() *clipboard.Clipboard  { return clipboard.NewDarwinClipboard() }
func newKeyboard() keyboard.Simulator     { return keyboard.NewDarwinSimulator() }
func newHotkeyManager() hotkey.Manager    { return hotkey.NewXPlatManager() }

// startMainLoop runs the Cocoa event loop on the main thread (required by
// macOS). Hotkey registration is deferred to a background goroutine — the
// registerHotkeys function waits on the appReady channel (closed when
// ApplicationStarted fires) so the Carbon API's dispatch_sync to the main
// queue doesn't deadlock.
func startMainLoop(trayRun func() error, registerHotkeys func() error, hk hotkey.Manager) {
	go func() {
		if err := registerHotkeys(); err != nil {
			os.Exit(1)
		}
		hk.Listen()
	}()
	// Cocoa event loop on main thread — blocks until app quits.
	trayRun()
	hk.Stop()
}
