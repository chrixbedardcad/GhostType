//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework ApplicationServices -framework CoreGraphics
#include <ApplicationServices/ApplicationServices.h>
#include <CoreGraphics/CoreGraphics.h>

int axIsTrusted() {
    return AXIsProcessTrusted();
}

// cgPostEventAllowed uses CGPreflightPostEventAccess (macOS 10.15+) to check
// whether the process can actually post synthetic keyboard/mouse events via
// CGEventPost. This is more accurate than AXIsProcessTrusted() for detecting
// stale TCC entries after binary updates.
int cgPostEventAllowed() {
    return CGPreflightPostEventAccess();
}

// cgListenEventAllowed uses CGPreflightListenEventAccess (macOS 10.15+) to
// check whether the process has Input Monitoring permission (can listen for
// global keyboard events via CGEventTap).
int cgListenEventAllowed() {
    return CGPreflightListenEventAccess();
}
*/
import "C"

import (
	"fmt"
	"os/exec"
)

// checkAccessibility returns true if the process has Accessibility permission.
// GhostSpell needs two macOS permissions:
//   - Accessibility — for CGEventPost (keyboard simulation)
//   - Input Monitoring — for RegisterEventHotKey (global hotkeys)
// Only Accessibility can be checked programmatically (AXIsProcessTrusted).
// There is no reliable public API for Input Monitoring.
func checkAccessibility() bool {
	return C.axIsTrusted() != 0
}

// checkPostEventAccess returns true if CGEventPost will actually deliver events.
func checkPostEventAccess() bool {
	return C.cgPostEventAllowed() != 0
}

// checkInputMonitoring returns true if the process has Input Monitoring
// permission. Uses CGPreflightListenEventAccess (macOS 10.15+).
// On older macOS versions where Accessibility covers Input Monitoring,
// this correctly returns true — no separate permission needed.
func checkInputMonitoring() bool {
	return C.cgListenEventAllowed() != 0
}

// openAccessibilitySettings opens the macOS System Settings to the
// Accessibility and Input Monitoring privacy panes so the user can grant
// both permissions GhostSpell needs.
func openAccessibilitySettings() {
	exec.Command("open", "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility").Start()
	exec.Command("open", "x-apple.systempreferences:com.apple.preference.security?Privacy_ListenEvent").Start()
}

// openAccessibilityPane opens only the Accessibility privacy pane.
func openAccessibilityPane() {
	exec.Command("open", "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility").Start()
}

// openInputMonitoringPane opens only the Input Monitoring privacy pane.
func openInputMonitoringPane() {
	exec.Command("open", "x-apple.systempreferences:com.apple.preference.security?Privacy_ListenEvent").Start()
}

// remindInputMonitoring prints a reminder about Input Monitoring.
// Called on every launch because there's no API to check this permission,
// and hotkeys silently fail without it.
func remindInputMonitoring() {
	fmt.Println("")
	fmt.Println("  Ensure Input Monitoring is enabled for GhostSpell:")
	fmt.Println("  System Settings > Privacy & Security > Input Monitoring")
	fmt.Println("  (Hotkeys won't work without this permission)")
	fmt.Println("")
}
