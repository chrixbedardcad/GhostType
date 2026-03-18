//go:build !darwin

package main

// checkAccessibility is a no-op on non-macOS platforms where no special
// permission is needed for global hotkeys or keyboard simulation.
func checkAccessibility() bool {
	return true
}

// checkPostEventAccess is a no-op on non-macOS platforms.
func checkPostEventAccess() bool {
	return true
}

// checkInputMonitoring is a no-op on non-macOS platforms.
func checkInputMonitoring() bool {
	return true
}

// requestAccessibility is a no-op on non-macOS platforms.
func requestAccessibility() bool { return true }

// openAccessibilitySettings is a no-op on non-macOS platforms.
func openAccessibilitySettings() {}

// openAccessibilityPane is a no-op on non-macOS platforms.
func openAccessibilityPane() {}

// openInputMonitoringPane is a no-op on non-macOS platforms.
func openInputMonitoringPane() {}

// remindInputMonitoring is a no-op on non-macOS platforms.
func remindInputMonitoring() {}
