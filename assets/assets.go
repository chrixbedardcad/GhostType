package assets

import _ "embed"

//go:embed GhostType_icon_512.png
var AppIcon512 []byte

//go:embed GhostType_icon_16.png
var AppIcon16 []byte

//go:embed GhostType_icon_32.png
var AppIcon32 []byte

//go:embed GhostType_tray_64.png
var TrayIcon64 []byte

//go:embed GhostType_tray_64_macOS.png
var TrayIconMacOS []byte
