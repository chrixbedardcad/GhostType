package clipboard

import (
	"fmt"
	"sync"
)

// Clipboard provides thread-safe clipboard operations with content preservation.
type Clipboard struct {
	mu       sync.Mutex
	saved    string
	hasSaved bool
	// read and write functions are injectable for testing and platform abstraction
	readFn  func() (string, error)
	writeFn func(text string) error
}

// New creates a new Clipboard with the given platform-specific read/write functions.
func New(readFn func() (string, error), writeFn func(text string) error) *Clipboard {
	return &Clipboard{
		readFn:  readFn,
		writeFn: writeFn,
	}
}

// Read returns the current clipboard text content.
func (c *Clipboard) Read() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.readFn == nil {
		return "", fmt.Errorf("clipboard read not available on this platform")
	}
	return c.readFn()
}

// Write sets the clipboard text content.
func (c *Clipboard) Write(text string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.writeFn == nil {
		return fmt.Errorf("clipboard write not available on this platform")
	}
	return c.writeFn(text)
}

// Save saves the current clipboard content so it can be restored later.
func (c *Clipboard) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.readFn == nil {
		return fmt.Errorf("clipboard read not available on this platform")
	}
	text, err := c.readFn()
	if err != nil {
		return fmt.Errorf("failed to save clipboard: %w", err)
	}
	c.saved = text
	c.hasSaved = true
	return nil
}

// Restore restores the previously saved clipboard content.
func (c *Clipboard) Restore() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.hasSaved {
		return nil // nothing to restore
	}
	if c.writeFn == nil {
		return fmt.Errorf("clipboard write not available on this platform")
	}
	err := c.writeFn(c.saved)
	if err != nil {
		return fmt.Errorf("failed to restore clipboard: %w", err)
	}
	c.hasSaved = false
	return nil
}
