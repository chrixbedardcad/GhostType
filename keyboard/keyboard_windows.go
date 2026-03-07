//go:build windows

package keyboard

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32         = syscall.NewLazyDLL("user32.dll")
	procSendInput  = user32.NewProc("SendInput")
)

const (
	inputKeyboard = 1
	keyEventUp    = 0x0002

	vkControl = 0x11
	vkA       = 0x41
	vkC       = 0x43
	vkV       = 0x56
)

// keybdInput is the KEYBDINPUT structure for SendInput.
type keybdInput struct {
	wVk         uint16
	wScan       uint16
	dwFlags     uint32
	time        uint32
	dwExtraInfo uintptr
}

// input is the INPUT structure for SendInput.
type input struct {
	inputType uint32
	ki        keybdInput
	padding   [8]byte
}

// WindowsSimulator implements keyboard simulation on Windows using SendInput.
type WindowsSimulator struct{}

// NewWindowsSimulator creates a new Windows keyboard simulator.
func NewWindowsSimulator() *WindowsSimulator {
	return &WindowsSimulator{}
}

func sendKey(vk uint16, down bool) error {
	var flags uint32
	if !down {
		flags = keyEventUp
	}
	inp := input{
		inputType: inputKeyboard,
		ki: keybdInput{
			wVk:     vk,
			dwFlags: flags,
		},
	}
	ret, _, _ := procSendInput.Call(1, uintptr(unsafe.Pointer(&inp)), unsafe.Sizeof(inp))
	if ret == 0 {
		action := "keydown"
		if !down {
			action = "keyup"
		}
		return fmt.Errorf("SendInput failed for vk=0x%02X %s", vk, action)
	}
	return nil
}

func sendKeyCombo(modifier, key uint16) error {
	if err := sendKey(modifier, true); err != nil {
		return err
	}
	if err := sendKey(key, true); err != nil {
		return err
	}
	time.Sleep(10 * time.Millisecond)
	if err := sendKey(key, false); err != nil {
		return err
	}
	if err := sendKey(modifier, false); err != nil {
		return err
	}
	return nil
}

func (s *WindowsSimulator) WaitForModifierRelease() {}
func (s *WindowsSimulator) ReadSelectedText() string        { return "" }
func (s *WindowsSimulator) ReadAllText() string              { return "" }
func (s *WindowsSimulator) WriteSelectedText(string) bool { return false }
func (s *WindowsSimulator) WriteAllText(string) bool      { return false }
func (s *WindowsSimulator) FrontAppName() string             { return "" }
func (s *WindowsSimulator) SelectAllAX() error               { return fmt.Errorf("not supported") }
func (s *WindowsSimulator) CopyAX() error                    { return fmt.Errorf("not supported") }
func (s *WindowsSimulator) PasteAX() error                   { return fmt.Errorf("not supported") }

func (s *WindowsSimulator) SelectAll() error {
	return sendKeyCombo(vkControl, vkA)
}

func (s *WindowsSimulator) Copy() error {
	return sendKeyCombo(vkControl, vkC)
}

func (s *WindowsSimulator) Paste() error {
	return sendKeyCombo(vkControl, vkV)
}
