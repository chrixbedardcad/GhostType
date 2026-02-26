//go:build windows

package clipboard

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	user32             = syscall.NewLazyDLL("user32.dll")
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	procOpenClipboard  = user32.NewProc("OpenClipboard")
	procCloseClipboard = user32.NewProc("CloseClipboard")
	procGetClipboardData = user32.NewProc("GetClipboardData")
	procSetClipboardData = user32.NewProc("SetClipboardData")
	procEmptyClipboard = user32.NewProc("EmptyClipboard")
	procGlobalAlloc    = kernel32.NewProc("GlobalAlloc")
	procGlobalLock     = kernel32.NewProc("GlobalLock")
	procGlobalUnlock   = kernel32.NewProc("GlobalUnlock")
)

const (
	cfUnicodeText = 13
	gmemMoveable  = 0x0002
)

// NewWindowsClipboard creates a Clipboard using native Windows clipboard API.
func NewWindowsClipboard() *Clipboard {
	return New(windowsRead, windowsWrite).WithClear(windowsClear)
}

func windowsClear() error {
	ret, _, _ := procOpenClipboard.Call(0)
	if ret == 0 {
		return fmt.Errorf("failed to open clipboard")
	}
	defer procCloseClipboard.Call()

	ret, _, _ = procEmptyClipboard.Call()
	if ret == 0 {
		return fmt.Errorf("failed to empty clipboard")
	}
	return nil
}

func windowsRead() (string, error) {
	ret, _, _ := procOpenClipboard.Call(0)
	if ret == 0 {
		return "", fmt.Errorf("failed to open clipboard")
	}
	defer procCloseClipboard.Call()

	h, _, _ := procGetClipboardData.Call(cfUnicodeText)
	if h == 0 {
		return "", nil // empty clipboard
	}

	p, _, _ := procGlobalLock.Call(h)
	if p == 0 {
		return "", fmt.Errorf("failed to lock clipboard data")
	}
	defer procGlobalUnlock.Call(h)

	text := syscall.UTF16ToString((*[1 << 20]uint16)(unsafe.Pointer(p))[:])
	return text, nil
}

func windowsWrite(text string) error {
	ret, _, _ := procOpenClipboard.Call(0)
	if ret == 0 {
		return fmt.Errorf("failed to open clipboard")
	}
	defer procCloseClipboard.Call()

	procEmptyClipboard.Call()

	utf16, err := syscall.UTF16FromString(text)
	if err != nil {
		return err
	}

	size := len(utf16) * 2
	h, _, _ := procGlobalAlloc.Call(gmemMoveable, uintptr(size))
	if h == 0 {
		return fmt.Errorf("failed to allocate global memory")
	}

	p, _, _ := procGlobalLock.Call(h)
	if p == 0 {
		return fmt.Errorf("failed to lock global memory")
	}

	copy((*[1 << 20]uint16)(unsafe.Pointer(p))[:len(utf16)], utf16)
	procGlobalUnlock.Call(h)

	procSetClipboardData.Call(cfUnicodeText, h)
	return nil
}
