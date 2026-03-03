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

// preListen locks the current goroutine to its OS thread.
// Windows hotkeys require RegisterHotKey + GetMessageW on the same thread.
func preListen() { runtime.LockOSThread() }
