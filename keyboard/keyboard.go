package keyboard

// Simulator provides platform-specific keyboard simulation.
// Used to send Ctrl+A, Ctrl+C, Ctrl+V key sequences to the active window.
type Simulator interface {
	// SelectAll simulates Ctrl+A (or Cmd+A on macOS).
	SelectAll() error

	// Copy simulates Ctrl+C (or Cmd+C on macOS).
	Copy() error

	// Paste simulates Ctrl+V (or Cmd+V on macOS).
	Paste() error

	// WaitForModifierRelease waits for all physical modifier keys to be released.
	// On macOS, this prevents hotkey modifiers (e.g. Ctrl from Ctrl+G) from
	// leaking into subsequent synthetic Cmd+A/C/V events via CGEventPost's
	// HID-level hardware state merging.
	WaitForModifierRelease()

	// ReadSelectedText reads the selected text from the focused UI element
	// using the platform's accessibility API. Returns empty if unavailable.
	ReadSelectedText() string

	// ReadAllText reads all text from the focused UI element.
	ReadAllText() string

	// WriteSelectedText replaces the selected text in the focused UI element.
	// Returns true on success. Falls back to false for apps that don't support it.
	WriteSelectedText(text string) bool

	// WriteAllText replaces all text in the focused UI element.
	// Returns true on success.
	WriteAllText(text string) bool

	// FrontAppName returns the name of the frontmost application.
	FrontAppName() string

	// SelectAllAX sends Cmd+A via AXUIElementPostKeyboardEvent (Accessibility
	// framework routing). Fallback for apps like Chrome where CGEventPost
	// keystrokes don't reach the content area. No-op on non-macOS.
	SelectAllAX() error

	// CopyAX sends Cmd+C via AXUIElementPostKeyboardEvent.
	CopyAX() error

	// PasteAX sends Cmd+V via AXUIElementPostKeyboardEvent.
	PasteAX() error
}
