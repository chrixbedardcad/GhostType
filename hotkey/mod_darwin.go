//go:build darwin

package hotkey

import "golang.design/x/hotkey"

// modAltXPlat is the platform-specific "Alt" modifier (Option on macOS).
var modAltXPlat = hotkey.ModOption
