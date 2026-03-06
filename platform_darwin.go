//go:build darwin

package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/chrixbedardcad/GhostType/clipboard"
	"github.com/chrixbedardcad/GhostType/hotkey"
	"github.com/chrixbedardcad/GhostType/keyboard"
)

func newClipboard() *clipboard.Clipboard  { return clipboard.NewDarwinClipboard() }
func newKeyboard() keyboard.Simulator     { return keyboard.NewDarwinSimulator() }
func newHotkeyManager() hotkey.Manager    { return hotkey.NewXPlatManager() }

// initKeyboard resolves keyboard layout key codes on the main thread.
// TIS (Text Input Source) API is not thread-safe and crashes/hangs if called
// from a background goroutine. This MUST run before startMainLoop().
func initKeyboard() { keyboard.ResolveLayout() }

// startMainLoop runs the Cocoa event loop on the main thread (required by
// macOS). Hotkey registration is deferred to a background goroutine — the
// registerHotkeys function waits on the appReady channel (closed when
// ApplicationStarted fires) so the Carbon API's dispatch_sync to the main
// queue doesn't deadlock.
func startMainLoop(trayRun func() error, registerHotkeys func() error, hk hotkey.Manager) {
	go func() {
		if err := registerHotkeys(); err != nil {
			// Log the error but keep the app running so the user can
			// access Settings to diagnose the issue. Previously this
			// called os.Exit(1) which silently killed the .app bundle.
			fmt.Fprintf(os.Stderr, "Hotkey registration failed: %v\n", err)
			slog.Error("Hotkey registration failed — app remains running for diagnostics", "error", err)
			return
		}
		hk.Listen()
	}()
	// Cocoa event loop on main thread — blocks until app quits.
	trayRun()
	hk.Stop()
}
