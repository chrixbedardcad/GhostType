//go:build !darwin

package main

// checkAccessibility is a no-op on non-macOS platforms where no special
// permission is needed for global hotkeys or keyboard simulation.
func checkAccessibility() bool {
	return true
}

// openAccessibilitySettings is a no-op on non-macOS platforms.
func openAccessibilitySettings() {}
