package hotkey

// Handler is a callback function invoked when a hotkey is pressed.
type Handler func()

// Manager is the interface for registering and listening for global hotkeys.
// Platform-specific implementations are in hotkey_windows.go, hotkey_linux.go, etc.
type Manager interface {
	// Register registers a hotkey with a callback handler.
	Register(name string, key string, handler Handler) error

	// Unregister removes a registered hotkey.
	Unregister(name string) error

	// Listen starts listening for hotkey events. Blocks until Stop is called.
	Listen() error

	// Stop stops the hotkey listener.
	Stop()
}
