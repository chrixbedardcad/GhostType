//go:build darwin

package hotkey

import "golang.design/x/hotkey"

// modAltXPlat is the platform-specific "Alt" modifier (Option on macOS).
var modAltXPlat = hotkey.ModOption

// modCtrlXPlat is the platform-specific "Ctrl" modifier.
// On macOS, "Ctrl" in config means the Command key (⌘), which is the primary
// modifier users expect — matching Cmd+C, Cmd+V, etc.
var modCtrlXPlat = hotkey.ModCmd
