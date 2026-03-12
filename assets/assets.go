package assets

import _ "embed"

//go:embed GhostSpell_icon_512.png
var AppIcon512 []byte

//go:embed GhostSpell_icon_16.png
var AppIcon16 []byte

//go:embed GhostSpell_icon_32.png
var AppIcon32 []byte

//go:embed GhostSpell_tray_64.png
var TrayIcon64 []byte

//go:embed GhostSpell_tray_64_macOS.png
var TrayIconMacOS []byte

//go:embed ghostspell.ico
var AppIconICO []byte

// Tray animation frames for working indicator (floating/breathing ghost).
// 4 frames: up-faded, center-bright, down-faded, center-dim.
//
//go:embed GhostSpell_tray_working_1.png
var TrayWorking1 []byte

//go:embed GhostSpell_tray_working_2.png
var TrayWorking2 []byte

//go:embed GhostSpell_tray_working_3.png
var TrayWorking3 []byte

//go:embed GhostSpell_tray_working_4.png
var TrayWorking4 []byte

//go:embed GhostSpell_tray_working_1_macOS.png
var TrayWorkingMacOS1 []byte

//go:embed GhostSpell_tray_working_2_macOS.png
var TrayWorkingMacOS2 []byte

//go:embed GhostSpell_tray_working_3_macOS.png
var TrayWorkingMacOS3 []byte

//go:embed GhostSpell_tray_working_4_macOS.png
var TrayWorkingMacOS4 []byte
