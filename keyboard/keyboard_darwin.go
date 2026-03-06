//go:build darwin

package keyboard

/*
#cgo LDFLAGS: -framework CoreGraphics
#include <CoreGraphics/CoreGraphics.h>

void sendKeyCombo(CGKeyCode modifier, CGKeyCode key) {
	CGEventRef modDown = CGEventCreateKeyboardEvent(NULL, modifier, true);
	CGEventRef keyDown = CGEventCreateKeyboardEvent(NULL, key, true);
	CGEventRef keyUp   = CGEventCreateKeyboardEvent(NULL, key, false);
	CGEventRef modUp   = CGEventCreateKeyboardEvent(NULL, modifier, false);

	CGEventSetFlags(keyDown, CGEventGetFlags(modDown));
	CGEventSetFlags(keyUp, CGEventGetFlags(modDown));

	CGEventPost(kCGHIDEventTap, modDown);
	CGEventPost(kCGHIDEventTap, keyDown);
	CGEventPost(kCGHIDEventTap, keyUp);
	CGEventPost(kCGHIDEventTap, modUp);

	CFRelease(modDown);
	CFRelease(keyDown);
	CFRelease(keyUp);
	CFRelease(modUp);
}
*/
import "C"

import (
	"log/slog"
	"time"
)

// macOS virtual key codes (ANSI/QWERTY positions).
// Cmd+ shortcuts on macOS ALWAYS use QWERTY key positions regardless of the
// active keyboard layout (AZERTY, QWERTZ, Dvorak, etc.). This is by design:
// Cmd+C is always the same physical key whether you use French, German, or US.
// Do NOT resolve these from the current layout — that causes Cmd+A to become
// Cmd+Q on AZERTY (key code 0x0C = 'a' on AZERTY but 'q' on QWERTY).
const (
	kVK_Command = 0x37
	kVK_ANSI_A  = 0x00
	kVK_ANSI_C  = 0x08
	kVK_ANSI_V  = 0x09
)

// ResolveLayout is a no-op kept for API compatibility. Cmd+ shortcuts use
// fixed QWERTY key codes on all layouts — no resolution needed.
func ResolveLayout() {}

// DarwinSimulator implements keyboard simulation on macOS using CGEvent.
// Requires Accessibility permission (System Settings > Privacy > Accessibility).
// Uses fixed QWERTY key codes for Cmd+A/C/V — correct on all keyboard layouts.
type DarwinSimulator struct{}

func NewDarwinSimulator() *DarwinSimulator {
	return &DarwinSimulator{}
}

func (s *DarwinSimulator) SelectAll() error {
	slog.Debug("[keyboard] SelectAll (Cmd+A)", "keyCode", "0x00")
	C.sendKeyCombo(C.CGKeyCode(kVK_Command), C.CGKeyCode(kVK_ANSI_A))
	time.Sleep(10 * time.Millisecond)
	return nil
}

func (s *DarwinSimulator) Copy() error {
	slog.Debug("[keyboard] Copy (Cmd+C)", "keyCode", "0x08")
	C.sendKeyCombo(C.CGKeyCode(kVK_Command), C.CGKeyCode(kVK_ANSI_C))
	time.Sleep(10 * time.Millisecond)
	return nil
}

func (s *DarwinSimulator) Paste() error {
	slog.Debug("[keyboard] Paste (Cmd+V)", "keyCode", "0x09")
	C.sendKeyCombo(C.CGKeyCode(kVK_Command), C.CGKeyCode(kVK_ANSI_V))
	time.Sleep(10 * time.Millisecond)
	return nil
}
