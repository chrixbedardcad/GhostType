//go:build darwin

package main

import (
	"github.com/chrixbedardcad/GhostType/clipboard"
	"github.com/chrixbedardcad/GhostType/hotkey"
	"github.com/chrixbedardcad/GhostType/keyboard"
)

func newClipboard() *clipboard.Clipboard  { return clipboard.NewDarwinClipboard() }
func newKeyboard() keyboard.Simulator     { return keyboard.NewDarwinSimulator() }
func newHotkeyManager() hotkey.Manager    { return hotkey.NewXPlatManager() }

func preListen() {} // macOS event loop handled by Wails tray goroutine
