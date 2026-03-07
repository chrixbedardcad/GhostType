//go:build windows

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

func newClipboard() *clipboard.Clipboard  { return clipboard.NewWindowsClipboard() }
func newKeyboard() keyboard.Simulator     { return keyboard.NewWindowsSimulator() }
func newHotkeyManager() hotkey.Manager    { return hotkey.NewWindowsManager() }
func initKeyboard()                       {} // no-op on Windows

// startMainLoop runs the Wails event loop on the main thread (required because
// COM/CoInitializeEx was called on the main thread during init — WebView2 needs
// it). Hotkeys are registered in a background goroutine (which blocks on
// wizardDone so the wizard can render first), then that goroutine runs the
// Windows message loop for RegisterHotKey + GetMessageW.
func startMainLoop(trayRun func() error, registerHotkeys func() error, hk hotkey.Manager) {
	go func() {
		runtime.LockOSThread()
		if err := registerHotkeys(); err != nil {
			fmt.Fprintf(os.Stderr, "Hotkey registration failed: %v\n", err)
			slog.Error("Hotkey registration failed — app remains running for diagnostics", "error", err)
			select {} // block forever; tray stays alive for Settings access
		}
		hk.Listen()
	}()
	// Wails event loop on main thread — blocks until app quits.
	// COM was initialized here by go-webview2's init().
	trayRun()
	hk.Stop()
}

// restartHotkeyListener starts a new hotkey listener goroutine.
// On Windows, RegisterHotKey and GetMessage must be on the same OS thread.
func restartHotkeyListener(hk hotkey.Manager, register func() error) {
	go func() {
		runtime.LockOSThread()
		if err := register(); err != nil {
			slog.Error("Failed to re-register hotkeys", "error", err)
			fmt.Fprintf(os.Stderr, "Hotkey re-registration failed: %v\n", err)
			return
		}
		hk.Listen()
	}()
}
