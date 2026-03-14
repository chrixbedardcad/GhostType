//go:build windows

package keyboard

import (
	"fmt"
	"log/slog"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32                    = syscall.NewLazyDLL("user32.dll")
	procSendInput             = user32.NewProc("SendInput")
	procGetAsyncKeyState      = user32.NewProc("GetAsyncKeyState")
	procGetForegroundWindow   = user32.NewProc("GetForegroundWindow")
	procSetForegroundWindow   = user32.NewProc("SetForegroundWindow")
	procGetWindowTextW        = user32.NewProc("GetWindowTextW")
)

const (
	inputKeyboard = 1
	keyEventUp    = 0x0002

	vkControl = 0x11
	vkShift   = 0x10
	vkMenu    = 0x12 // Alt
	vkLWin    = 0x5B
	vkRWin    = 0x5C
	vkA       = 0x41
	vkC       = 0x43
	vkV       = 0x56
	vkRight   = 0x27
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
type WindowsSimulator struct {
	// savedHWND is the foreground window handle saved by SaveForegroundWindow.
	// Used to restore focus after the indicator overlay steals it.
	savedHWND uintptr
}

// NewWindowsSimulator creates a new Windows keyboard simulator.
func NewWindowsSimulator() *WindowsSimulator {
	return &WindowsSimulator{}
}

func makeInput(vk uint16, down bool) input {
	var flags uint32
	if !down {
		flags = keyEventUp
	}
	return input{
		inputType: inputKeyboard,
		ki: keybdInput{
			wVk:     vk,
			dwFlags: flags,
		},
	}
}

func sendKey(vk uint16, down bool) error {
	inp := makeInput(vk, down)
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

// sendKeyComboAtomic sends modifier+key as a single atomic SendInput call.
// This prevents other processes from injecting input between our keystrokes.
func sendKeyComboAtomic(modifier, key uint16) error {
	inputs := [4]input{
		makeInput(modifier, true),
		makeInput(key, true),
		makeInput(key, false),
		makeInput(modifier, false),
	}
	ret, _, err := procSendInput.Call(
		4,
		uintptr(unsafe.Pointer(&inputs[0])),
		unsafe.Sizeof(inputs[0]),
	)
	if ret != 4 {
		return fmt.Errorf("SendInput: expected 4 events injected, got %d (err=%v)", ret, err)
	}
	slog.Info("SendInput: keystroke sent", "modifier", fmt.Sprintf("0x%02X", modifier), "key", fmt.Sprintf("0x%02X", key), "injected", ret)
	return nil
}

// WaitForModifierRelease polls GetAsyncKeyState until all modifier keys
// (Ctrl, Shift, Alt, Win) are physically released. This prevents our
// synthetic Ctrl+C/V from colliding with the user's hotkey modifiers.
// If the poll times out (e.g. Ctrl from Ctrl+G is still "stuck" in the
// Windows input queue), we inject keyup events for all modifiers to
// forcefully clear the stuck state before proceeding.
func (s *WindowsSimulator) WaitForModifierRelease() {
	modKeys := []uint16{vkControl, vkShift, vkMenu, vkLWin, vkRWin}
	const maxWait = 1000 * time.Millisecond
	const pollInterval = 5 * time.Millisecond
	start := time.Now()
	deadline := start.Add(maxWait)

	for time.Now().Before(deadline) {
		anyPressed := false
		for _, vk := range modKeys {
			ret, _, _ := procGetAsyncKeyState.Call(uintptr(vk))
			if ret&0x8000 != 0 { // high bit = currently pressed
				anyPressed = true
				break
			}
		}
		if !anyPressed {
			waited := time.Since(start)
			slog.Info("WaitForModifierRelease: modifiers released", "waited_ms", waited.Milliseconds())
			// Small settle delay — let the OS fully process the key release
			// before we inject new keystrokes.
			time.Sleep(30 * time.Millisecond)
			return
		}
		time.Sleep(pollInterval)
	}

	// Timeout — force-release all modifier keys via SendInput. On some Windows
	// machines the Ctrl key from the hotkey (Ctrl+G) stays "stuck" in the input
	// queue, which causes our subsequent Ctrl+C/A/V to be interpreted as bare
	// C/A/V (or worse, nothing at all). Injecting explicit keyup events clears
	// the stuck state so the following SendInput calls work correctly.
	slog.Warn("WaitForModifierRelease: timed out, force-releasing modifiers", "waited_ms", maxWait.Milliseconds())
	for _, vk := range modKeys {
		ret, _, _ := procGetAsyncKeyState.Call(uintptr(vk))
		if ret&0x8000 != 0 {
			slog.Info("WaitForModifierRelease: force-releasing stuck modifier", "vk", fmt.Sprintf("0x%02X", vk))
			sendKey(vk, false) // inject keyup
		}
	}
	time.Sleep(50 * time.Millisecond)
}

func (s *WindowsSimulator) ReadSelectedText() string    { return "" }
func (s *WindowsSimulator) ReadAllText() string          { return "" }
func (s *WindowsSimulator) WriteSelectedText(string) bool { return false }
func (s *WindowsSimulator) WriteAllText(string) bool      { return false }

// FrontAppName returns the title of the foreground window.
// Returns "(untitled)" if a window exists but has no title, and "" if no
// foreground window exists at all. This distinction helps diagnose capture
// issues where the hotkey fires but no target window is in focus.
func (s *WindowsSimulator) FrontAppName() string {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		slog.Info("FrontAppName: no foreground window (hwnd=0)")
		return ""
	}
	var buf [256]uint16
	ret, _, _ := procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if ret == 0 {
		slog.Info("FrontAppName: foreground window has no title", "hwnd", fmt.Sprintf("0x%X", hwnd))
		return "(untitled)"
	}
	return syscall.UTF16ToString(buf[:ret])
}

// SaveForegroundWindow records the current foreground window HWND so that
// RestoreForegroundWindow can give it focus later. Call this right after
// text capture — before ShowIndicator steals focus.
func (s *WindowsSimulator) SaveForegroundWindow() {
	hwnd, _, _ := procGetForegroundWindow.Call()
	s.savedHWND = hwnd
	slog.Info("SaveForegroundWindow", "hwnd", fmt.Sprintf("0x%X", hwnd), "title", s.FrontAppName())
}

// RestoreForegroundWindow gives focus back to the window saved by
// SaveForegroundWindow. Idempotent — skips if already focused or if
// no window was saved.
func (s *WindowsSimulator) RestoreForegroundWindow() {
	if s.savedHWND == 0 {
		return
	}
	current, _, _ := procGetForegroundWindow.Call()
	if current == s.savedHWND {
		return // already focused
	}
	slog.Info("RestoreForegroundWindow", "target", fmt.Sprintf("0x%X", s.savedHWND), "current", fmt.Sprintf("0x%X", current))
	ret, _, _ := procSetForegroundWindow.Call(s.savedHWND)
	if ret == 0 {
		slog.Warn("SetForegroundWindow failed", "hwnd", fmt.Sprintf("0x%X", s.savedHWND))
	}
	time.Sleep(20 * time.Millisecond)
}

func (s *WindowsSimulator) SelectAllAX() error     { return fmt.Errorf("not supported") }
func (s *WindowsSimulator) CopyAX() error          { return fmt.Errorf("not supported") }
func (s *WindowsSimulator) PasteAX() error          { return fmt.Errorf("not supported") }
func (s *WindowsSimulator) SelectAllScript() error  { return fmt.Errorf("not supported") }
func (s *WindowsSimulator) CopyScript() error        { return fmt.Errorf("not supported") }
func (s *WindowsSimulator) PasteScript() error       { return fmt.Errorf("not supported") }

func (s *WindowsSimulator) SelectAll() error {
	slog.Info("Windows SelectAll: sending Ctrl+A", "foreground", s.FrontAppName())
	return sendKeyComboAtomic(vkControl, vkA)
}

func (s *WindowsSimulator) Copy() error {
	slog.Info("Windows Copy: sending Ctrl+C", "foreground", s.FrontAppName())
	return sendKeyComboAtomic(vkControl, vkC)
}

func (s *WindowsSimulator) Paste() error {
	return sendKeyComboAtomic(vkControl, vkV)
}

func (s *WindowsSimulator) PressRight() error {
	if err := sendKey(vkRight, true); err != nil {
		return err
	}
	time.Sleep(10 * time.Millisecond)
	return sendKey(vkRight, false)
}
