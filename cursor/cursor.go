package cursor

import "time"

// Notifier displays brief floating labels near the cursor position.
// Used for Ctrl+F7 (language toggle) and Ctrl+F8 (template toggle) notifications.
type Notifier interface {
	// Show displays a brief floating label near the cursor with the given text.
	// The label auto-dismisses after the given duration.
	Show(text string, duration time.Duration) error
}
