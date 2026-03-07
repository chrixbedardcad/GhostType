//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework ApplicationServices
#include <ApplicationServices/ApplicationServices.h>

int axIsTrusted() {
    return AXIsProcessTrusted();
}
*/
import "C"

import (
	"fmt"
	"os/exec"
)

// checkAccessibility returns true if the process has Accessibility permission.
// GhostType needs two macOS permissions:
//   - Accessibility — for CGEventPost (keyboard simulation)
//   - Input Monitoring — for RegisterEventHotKey (global hotkeys)
// Only Accessibility can be checked programmatically (AXIsProcessTrusted).
// There is no reliable public API for Input Monitoring.
func checkAccessibility() bool {
	return C.axIsTrusted() != 0
}

// openAccessibilitySettings opens the macOS System Settings to the
// Accessibility and Input Monitoring privacy panes so the user can grant
// both permissions GhostType needs.
func openAccessibilitySettings() {
	exec.Command("open", "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility").Start()
	exec.Command("open", "x-apple.systempreferences:com.apple.preference.security?Privacy_ListenEvent").Start()
}

// remindInputMonitoring prints a reminder about Input Monitoring.
// Called on every launch because there's no API to check this permission,
// and hotkeys silently fail without it.
func remindInputMonitoring() {
	fmt.Println("")
	fmt.Println("  Ensure Input Monitoring is enabled for GhostType:")
	fmt.Println("  System Settings > Privacy & Security > Input Monitoring")
	fmt.Println("  (Hotkeys won't work without this permission)")
	fmt.Println("")
}
