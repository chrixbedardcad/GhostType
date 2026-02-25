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
}
