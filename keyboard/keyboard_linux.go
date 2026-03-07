//go:build linux

package keyboard

import (
	"fmt"
	"os/exec"
	"time"
)

// LinuxSimulator implements keyboard simulation on Linux using xdotool.
// Requires xdotool: apt install xdotool
type LinuxSimulator struct{}

// NewLinuxSimulator creates a new Linux keyboard simulator.
func NewLinuxSimulator() *LinuxSimulator {
	return &LinuxSimulator{}
}

func (s *LinuxSimulator) WaitForModifierRelease() {}
func (s *LinuxSimulator) ReadSelectedText() string        { return "" }
func (s *LinuxSimulator) ReadAllText() string              { return "" }
func (s *LinuxSimulator) WriteSelectedText(string) bool { return false }
func (s *LinuxSimulator) WriteAllText(string) bool      { return false }
func (s *LinuxSimulator) FrontAppName() string             { return "" }
func (s *LinuxSimulator) SelectAllAX() error               { return fmt.Errorf("not supported") }
func (s *LinuxSimulator) CopyAX() error                    { return fmt.Errorf("not supported") }
func (s *LinuxSimulator) PasteAX() error                   { return fmt.Errorf("not supported") }

func (s *LinuxSimulator) SelectAll() error {
	return xdotoolKey("ctrl+a")
}

func (s *LinuxSimulator) Copy() error {
	return xdotoolKey("ctrl+c")
}

func (s *LinuxSimulator) Paste() error {
	return xdotoolKey("ctrl+v")
}

func xdotoolKey(keys string) error {
	path, err := exec.LookPath("xdotool")
	if err != nil {
		return fmt.Errorf("xdotool not found (install: apt install xdotool): %w", err)
	}
	if err := exec.Command(path, "key", "--clearmodifiers", keys).Run(); err != nil {
		return fmt.Errorf("xdotool key %s: %w", keys, err)
	}
	time.Sleep(10 * time.Millisecond)
	return nil
}
