//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework ApplicationServices -framework CoreGraphics -framework Foundation
#include <ApplicationServices/ApplicationServices.h>
#include <CoreGraphics/CoreGraphics.h>
#import <Foundation/Foundation.h>

int axIsTrusted() {
    return AXIsProcessTrusted();
}

// axRequestAccess calls AXIsProcessTrustedWithOptions with kAXTrustedCheckOptionPrompt.
// This triggers the native macOS Accessibility prompt dialog on first call, which
// pre-lists the app in System Settings > Accessibility. Much better UX than manually
// navigating to the settings pane. The prompt only fires once per process launch.
int axRequestAccess() {
    NSDictionary *options = @{(__bridge id)kAXTrustedCheckOptionPrompt: @YES};
    return AXIsProcessTrustedWithOptions((__bridge CFDictionaryRef)options);
}

// cgPostEventAllowed uses CGPreflightPostEventAccess (macOS 10.15+) to check
// whether the process can actually post synthetic keyboard/mouse events via
// CGEventPost. This is more accurate than AXIsProcessTrusted() for detecting
// stale TCC entries after binary updates.
int cgPostEventAllowed() {
    return CGPreflightPostEventAccess();
}

// cgCanCreateKeyEvent tests Accessibility by actually creating a keyboard event
// and immediately releasing it. This is the strongest validation — it catches
// stale TCC entries where AXIsProcessTrusted()=true but the binary hash changed
// after an update, making CGEventCreateKeyboardEvent return NULL.
int cgCanCreateKeyEvent() {
    CGEventSourceRef src = CGEventSourceCreate(kCGEventSourceStateCombinedSessionState);
    if (!src) {
        src = CGEventSourceCreate(kCGEventSourceStateHIDSystemState);
    }
    CGEventRef ev = CGEventCreateKeyboardEvent(src, 0, true);
    if (src) CFRelease(src);
    if (ev == NULL) {
        return 0; // Accessibility NOT truly granted
    }
    CFRelease(ev);
    return 1; // Accessibility working
}

// cgCanCreateEventTap tests Input Monitoring by actually creating a CGEventTap
// and immediately destroying it. CGPreflightListenEventAccess is unreliable
// (returns true on macOS 13+ even when permission is NOT granted).
// Creating a real tap is the ONLY 100% reliable method (#172, v0.23.1).
int cgCanCreateEventTap() {
    CFMachPortRef tap = CGEventTapCreate(
        kCGSessionEventTap,
        kCGHeadInsertEventTap,
        kCGEventTapOptionListenOnly,
        CGEventMaskBit(kCGEventKeyDown),
        NULL, NULL);
    if (tap == NULL) {
        return 0; // Input Monitoring NOT granted
    }
    CFRelease(tap);
    return 1; // Input Monitoring granted
}
*/
import "C"

import (
	"fmt"
	"os/exec"
)

// requestAccessibility triggers the native macOS Accessibility prompt dialog.
// This pre-lists GhostSpell in System Settings > Accessibility, making it easy
// for users to toggle it ON. The prompt only fires once per process launch.
// Returns true if already trusted.
func requestAccessibility() bool {
	return C.axRequestAccess() != 0
}

// checkAccessibility returns true if the process has working Accessibility permission.
// Uses a real CGEventCreateKeyboardEvent test — the strongest validation.
// AXIsProcessTrusted() alone is unreliable after binary updates (returns true
// even when the TCC entry is stale and events won't actually post).
func checkAccessibility() bool {
	// Fast check first — if AX isn't trusted at all, no point testing further.
	if C.axIsTrusted() == 0 {
		return false
	}
	// Strong validation: actually create a keyboard event to verify the
	// binary hash matches the TCC entry.
	return C.cgCanCreateKeyEvent() != 0
}

// checkPostEventAccess returns true if CGEventPost will actually deliver events.
func checkPostEventAccess() bool {
	if C.cgPostEventAllowed() == 0 {
		return false
	}
	// Double-check with real event creation.
	return C.cgCanCreateKeyEvent() != 0
}

// checkInputMonitoring returns true if the process has Input Monitoring
// permission. Creates a real CGEventTap to test — the only reliable method.
// CGPreflightListenEventAccess is unreliable on macOS 13+ (#172).
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
