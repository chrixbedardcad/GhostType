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

import "os/exec"

// checkAccessibility returns true if the process has Accessibility permission.
// On macOS, without this permission hotkey registration crashes (SIGTRAP) and
// keyboard simulation silently fails.
func checkAccessibility() bool {
	return C.axIsTrusted() != 0
}

// openAccessibilitySettings opens the macOS System Settings to the
// Accessibility privacy pane so the user can grant permission.
func openAccessibilitySettings() {
	exec.Command("open", "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility").Start()
}
