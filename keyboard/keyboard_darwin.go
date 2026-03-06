//go:build darwin

package keyboard

/*
#cgo LDFLAGS: -framework CoreGraphics -framework CoreFoundation -framework Carbon
#include <CoreGraphics/CoreGraphics.h>
#include <Carbon/Carbon.h>

// keyCodeForChar finds the key code that produces the given character
// on the current keyboard layout. Returns UINT16_MAX if not found.
// This handles AZERTY, QWERTZ, Dvorak, and any other layout correctly.
CGKeyCode keyCodeForChar(UniChar c) {
	TISInputSourceRef source = TISCopyCurrentKeyboardInputSource();
	if (!source) return UINT16_MAX;

	CFDataRef layoutData = TISGetInputSourceProperty(source, kTISPropertyUnicodeKeyLayoutData);
	if (!layoutData) {
		CFRelease(source);
		return UINT16_MAX;
	}

	const UCKeyboardLayout *layout = (const UCKeyboardLayout *)CFDataGetBytePtr(layoutData);

	for (CGKeyCode keyCode = 0; keyCode < 128; keyCode++) {
		UInt32 deadKeyState = 0;
		UniCharCount actualLength = 0;
		UniChar chars[4] = {0};

		OSStatus status = UCKeyTranslate(
			layout,
			keyCode,
			kUCKeyActionDown,
			0, // no modifiers
			LMGetKbdType(),
			kUCKeyTranslateNoDeadKeysBit,
			&deadKeyState,
			sizeof(chars) / sizeof(chars[0]),
			&actualLength,
			chars
		);

		if (status == noErr && actualLength > 0 && chars[0] == c) {
			CFRelease(source);
			return keyCode;
		}
	}

	CFRelease(source);
	return UINT16_MAX;
}

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
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// macOS virtual key codes — fallbacks if layout lookup fails.
const (
	kVK_Command = 0x37
	kVK_ANSI_A  = 0x00
	kVK_ANSI_C  = 0x08
	kVK_ANSI_V  = 0x09
)

var (
	layoutOnce sync.Once
	keyA       C.CGKeyCode
	keyC       C.CGKeyCode
	keyV       C.CGKeyCode
)

// resolveKeys looks up the correct key codes for a/c/v on the current layout.
// Called once (lazy). Falls back to ANSI codes if lookup fails.
func resolveKeys() {
	layoutOnce.Do(func() {
		keyA = resolveChar('a', kVK_ANSI_A)
		keyC = resolveChar('c', kVK_ANSI_C)
		keyV = resolveChar('v', kVK_ANSI_V)
		slog.Debug("[keyboard] Resolved layout key codes",
			"a", fmt.Sprintf("0x%02X", keyA),
			"c", fmt.Sprintf("0x%02X", keyC),
			"v", fmt.Sprintf("0x%02X", keyV),
		)
	})
}

func resolveChar(ch rune, fallback int) C.CGKeyCode {
	code := C.keyCodeForChar(C.UniChar(ch))
	if code == C.CGKeyCode(0xFFFF) { // UINT16_MAX
		slog.Warn("[keyboard] Layout lookup failed, using ANSI fallback",
			"char", string(ch), "fallback", fmt.Sprintf("0x%02X", fallback))
		return C.CGKeyCode(fallback)
	}
	return code
}

// DarwinSimulator implements keyboard simulation on macOS using CGEvent.
// Requires Accessibility permission (System Preferences → Privacy → Accessibility).
// Key codes are resolved from the current keyboard layout at first use,
// so AZERTY, QWERTZ, Dvorak, etc. all work correctly.
type DarwinSimulator struct{}

// NewDarwinSimulator creates a new macOS keyboard simulator.
func NewDarwinSimulator() *DarwinSimulator {
	return &DarwinSimulator{}
}

func (s *DarwinSimulator) SelectAll() error {
	resolveKeys()
	slog.Debug("[keyboard] SelectAll", "keyCode", fmt.Sprintf("0x%02X", keyA))
	C.sendKeyCombo(C.CGKeyCode(kVK_Command), keyA)
	time.Sleep(10 * time.Millisecond)
	return nil
}

func (s *DarwinSimulator) Copy() error {
	resolveKeys()
	slog.Debug("[keyboard] Copy", "keyCode", fmt.Sprintf("0x%02X", keyC))
	C.sendKeyCombo(C.CGKeyCode(kVK_Command), keyC)
	time.Sleep(10 * time.Millisecond)
	return nil
}

func (s *DarwinSimulator) Paste() error {
	resolveKeys()
	slog.Debug("[keyboard] Paste", "keyCode", fmt.Sprintf("0x%02X", keyV))
	C.sendKeyCombo(C.CGKeyCode(kVK_Command), keyV)
	time.Sleep(10 * time.Millisecond)
	return nil
}
