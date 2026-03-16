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

// cgCanCreateEventTap tries to create a real CGEventTap and immediately
// destroys it. This is the only 100% reliable way to check Input Monitoring.
// Returns 1 if the tap was created successfully, 0 if it failed (permission denied).
int cgCanCreateEventTap() {
    CFMachPortRef tap = CGEventTapCreate(
        kCGSessionEventTap,
        kCGHeadInsertEventTap,
        kCGEventTapOptionListenOnly,
        CGEventMaskBit(kCGEventKeyDown),
        NULL,  // no callback needed — just testing if creation succeeds
        NULL
    );
    if (tap == NULL) {
        return 0; // Input Monitoring not granted
    }
    CFRelease(tap);
    return 1;
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

// checkInputMonitoring returns true if the process has Input Monitoring permission.
// Uses a real CGEventTap creation test — the only 100% reliable method.
// CGPreflightListenEventAccess is unreliable on some macOS versions (returns
// true even when the permission is not granted).
func checkInputMonitoring() bool {
	return C.cgCanCreateEventTap() != 0
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
