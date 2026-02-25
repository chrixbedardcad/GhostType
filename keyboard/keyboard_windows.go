//go:build windows

package keyboard

import (
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

func sendKey(vk uint16, down bool) {
	var flags uint32
	if !down {
		flags = keyEventUp
	}
	inp := input{
		inputType: inputKeyboard,
		ki: keybdInput{
			wVk:   vk,
			dwFlags: flags,
		},
	}
	procSendInput.Call(1, uintptr(unsafe.Pointer(&inp)), unsafe.Sizeof(inp))
}

func sendKeyCombo(modifier, key uint16) {
	sendKey(modifier, true)
	sendKey(key, true)
	time.Sleep(10 * time.Millisecond)
	sendKey(key, false)
	sendKey(modifier, false)
}

func (s *WindowsSimulator) SelectAll() error {
	sendKeyCombo(vkControl, vkA)
	return nil
}

func (s *WindowsSimulator) Copy() error {
	sendKeyCombo(vkControl, vkC)
	return nil
}

func (s *WindowsSimulator) Paste() error {
	sendKeyCombo(vkControl, vkV)
	return nil
}
