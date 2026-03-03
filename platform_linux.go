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

func preListen() {} // no OS thread lock needed on Linux
