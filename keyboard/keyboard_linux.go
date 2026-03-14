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

func (s *LinuxSimulator) WaitForModifierRelease()      {}
func (s *LinuxSimulator) SaveForegroundWindow()         {}
func (s *LinuxSimulator) RestoreForegroundWindow()      {}
func (s *LinuxSimulator) ReadSelectedText() string        { return "" }
func (s *LinuxSimulator) ReadAllText() string              { return "" }
func (s *LinuxSimulator) WriteSelectedText(string) bool { return false }
func (s *LinuxSimulator) WriteAllText(string) bool      { return false }
func (s *LinuxSimulator) FrontAppName() string             { return "" }
func (s *LinuxSimulator) SelectAllAX() error               { return fmt.Errorf("not supported") }
func (s *LinuxSimulator) CopyAX() error                    { return fmt.Errorf("not supported") }
func (s *LinuxSimulator) PasteAX() error                   { return fmt.Errorf("not supported") }
func (s *LinuxSimulator) SelectAllScript() error           { return fmt.Errorf("not supported") }
func (s *LinuxSimulator) CopyScript() error                { return fmt.Errorf("not supported") }
func (s *LinuxSimulator) PasteScript() error               { return fmt.Errorf("not supported") }

func (s *LinuxSimulator) SelectAll() error {
	return xdotoolKeys("ctrl+a")
}

func (s *LinuxSimulator) Copy() error {
	return xdotoolKeys("ctrl+c")
}

func (s *LinuxSimulator) Paste() error {
	return xdotoolKeys("ctrl+v")
}

func (s *LinuxSimulator) PressRight() error {
	return xdotoolKeys("Right")
}

// xdotoolKeys sends one or more key sequences in a single xdotool invocation.
// Each key argument is a separate keystroke (e.g., "ctrl+a", "ctrl+c").
// Multiple keys are batched: xdotool key --clearmodifiers key1 key2 ...
func xdotoolKeys(keys ...string) error {
	path, err := exec.LookPath("xdotool")
	if err != nil {
		return fmt.Errorf("xdotool not found (install: apt install xdotool): %w", err)
	}
	args := append([]string{"key", "--clearmodifiers"}, keys...)
	if err := exec.Command(path, args...).Run(); err != nil {
		return fmt.Errorf("xdotool key %v: %w", keys, err)
	}
	time.Sleep(10 * time.Millisecond)
	return nil
}
