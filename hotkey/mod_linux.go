//go:build linux

package hotkey

import "golang.design/x/hotkey"

// modAltXPlat is the platform-specific "Alt" modifier.
var modAltXPlat = hotkey.Mod1

// modCtrlXPlat is the platform-specific "Ctrl" modifier (physical Ctrl on Linux).
var modCtrlXPlat = hotkey.ModCtrl
