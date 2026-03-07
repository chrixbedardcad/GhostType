//go:build darwin

package keyboard

/*
#cgo LDFLAGS: -framework CoreGraphics -framework CoreFoundation -framework Carbon
#include <CoreGraphics/CoreGraphics.h>
#include <Carbon/Carbon.h>
#include <unistd.h>

// keyCodeForChar finds the key code that produces the given character
// on the current keyboard layout. Returns UINT16_MAX if not found.
// This is required because CGEventPost uses the CURRENT layout to map
// key codes to characters — not QWERTY. On AZERTY, key code 0x00
// produces 'q' (not 'a'), so Cmd+0x00 would send Cmd+Q (Quit) instead
// of Cmd+A (Select All). Layout resolution ensures correct behaviour.
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

// sendKeyComboWithChar posts a Cmd+key event with an explicit Unicode character.
// The key code is layout-resolved (correct for Cocoa apps), and the Unicode
// string is set explicitly (correct for non-Cocoa apps like Firestorm that
// interpret key codes using QWERTY mapping regardless of layout).
// Returns 0 on success, -1 if event creation failed (permission denied).
int sendKeyComboWithChar(CGKeyCode modifier, CGKeyCode key, UniChar ch) {
	// Use an explicit event source with CombinedSessionState. Passing NULL
	// uses a default source that may be silently blocked by macOS TCC even
	// when AXIsProcessTrusted() and CGPreflightPostEventAccess() return true.
	CGEventSourceRef source = CGEventSourceCreate(kCGEventSourceStateCombinedSessionState);
	if (!source) {
		// Fallback to HID system state if combined fails.
		source = CGEventSourceCreate(kCGEventSourceStateHIDSystemState);
	}
	// source may still be NULL — CGEventCreateKeyboardEvent accepts NULL.

	CGEventRef modDown = CGEventCreateKeyboardEvent(source, modifier, true);
	CGEventRef keyDown = CGEventCreateKeyboardEvent(source, key, true);
	CGEventRef keyUp   = CGEventCreateKeyboardEvent(source, key, false);
	CGEventRef modUp   = CGEventCreateKeyboardEvent(source, modifier, false);

	if (source) CFRelease(source);

	// CGEventCreateKeyboardEvent returns NULL when the process lacks
	// Accessibility permission (macOS 10.15+). Detect and report this
	// instead of crashing on CFRelease(NULL).
	if (!modDown || !keyDown || !keyUp || !modUp) {
		if (modDown) CFRelease(modDown);
		if (keyDown) CFRelease(keyDown);
		if (keyUp)   CFRelease(keyUp);
		if (modUp)   CFRelease(modUp);
		return -1;
	}

	CGEventSetFlags(keyDown, CGEventGetFlags(modDown));
	CGEventSetFlags(keyUp, CGEventGetFlags(modDown));

	// Explicitly set the Unicode character so ALL apps (Cocoa and non-Cocoa)
	// see the correct character regardless of key code interpretation.
	CGEventKeyboardSetUnicodeString(keyDown, 1, &ch);
	CGEventKeyboardSetUnicodeString(keyUp, 1, &ch);

	// Post with small inter-event delays so the target app's run loop
	// has time to process each event before the next arrives.
	CGEventPost(kCGHIDEventTap, modDown);
	usleep(2000); // 2ms
	CGEventPost(kCGHIDEventTap, keyDown);
	usleep(2000);
	CGEventPost(kCGHIDEventTap, keyUp);
	usleep(2000);
	CGEventPost(kCGHIDEventTap, modUp);

	CFRelease(modDown);
	CFRelease(keyDown);
	CFRelease(keyUp);
	CFRelease(modUp);
	return 0;
}
*/
import "C"

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// macOS virtual key codes — ANSI/QWERTY fallbacks if layout lookup fails.
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

// ResolveLayout resolves key codes for a/c/v from the current keyboard layout.
// MUST be called from the main thread before any hotkey callbacks fire — the
// TIS (Text Input Source) API is not thread-safe and can hang or crash if
// called from a background goroutine. Falls back to ANSI key codes on failure.
//
// This is critical for non-QWERTY layouts: CGEventPost maps key codes to
// characters using the CURRENT layout, so we must find the key code that
// produces 'a'/'c'/'v' on the active layout (e.g., on AZERTY, 'a' is at
// key code 0x0C, not 0x00).
func ResolveLayout() {
	resolveKeys()
}

func resolveKeys() {
	layoutOnce.Do(func() {
		slog.Debug("[keyboard] Resolving layout key codes...")
		done := make(chan struct{})
		go func() {
			keyA = resolveChar('a', kVK_ANSI_A)
			keyC = resolveChar('c', kVK_ANSI_C)
			keyV = resolveChar('v', kVK_ANSI_V)
			close(done)
		}()
		select {
		case <-done:
			slog.Debug("[keyboard] Resolved layout key codes",
				"a", fmt.Sprintf("0x%02X", keyA),
				"c", fmt.Sprintf("0x%02X", keyC),
				"v", fmt.Sprintf("0x%02X", keyV),
			)
		case <-time.After(3 * time.Second):
			slog.Warn("[keyboard] TIS layout resolution timed out, using ANSI fallbacks")
			keyA = C.CGKeyCode(kVK_ANSI_A)
			keyC = C.CGKeyCode(kVK_ANSI_C)
			keyV = C.CGKeyCode(kVK_ANSI_V)
		}
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
// Requires Accessibility permission (System Settings > Privacy > Accessibility).
// Key codes are resolved from the current keyboard layout so that Cmd+A/C/V
// work correctly on AZERTY, QWERTZ, Dvorak, and other non-QWERTY layouts.
type DarwinSimulator struct{}

func NewDarwinSimulator() *DarwinSimulator {
	return &DarwinSimulator{}
}

func (s *DarwinSimulator) SelectAll() error {
	resolveKeys()
	slog.Debug("[keyboard] SelectAll (Cmd+A)", "keyCode", fmt.Sprintf("0x%02X", keyA))
	if ret := C.sendKeyComboWithChar(C.CGKeyCode(kVK_Command), keyA, C.UniChar('a')); ret != 0 {
		slog.Error("[keyboard] CGEventCreate returned NULL — Accessibility permission revoked or stale",
			"action", "SelectAll")
		return fmt.Errorf("CGEventCreate failed for SelectAll — Accessibility permission likely revoked (toggle OFF/ON in System Settings)")
	}
	time.Sleep(10 * time.Millisecond)
	return nil
}

func (s *DarwinSimulator) Copy() error {
	resolveKeys()
	slog.Debug("[keyboard] Copy (Cmd+C)", "keyCode", fmt.Sprintf("0x%02X", keyC))
	if ret := C.sendKeyComboWithChar(C.CGKeyCode(kVK_Command), keyC, C.UniChar('c')); ret != 0 {
		slog.Error("[keyboard] CGEventCreate returned NULL — Accessibility permission revoked or stale",
			"action", "Copy")
		return fmt.Errorf("CGEventCreate failed for Copy — Accessibility permission likely revoked (toggle OFF/ON in System Settings)")
	}
	time.Sleep(10 * time.Millisecond)
	return nil
}

func (s *DarwinSimulator) Paste() error {
	resolveKeys()
	slog.Debug("[keyboard] Paste (Cmd+V)", "keyCode", fmt.Sprintf("0x%02X", keyV))
	if ret := C.sendKeyComboWithChar(C.CGKeyCode(kVK_Command), keyV, C.UniChar('v')); ret != 0 {
		slog.Error("[keyboard] CGEventCreate returned NULL — Accessibility permission revoked or stale",
			"action", "Paste")
		return fmt.Errorf("CGEventCreate failed for Paste — Accessibility permission likely revoked (toggle OFF/ON in System Settings)")
	}
	time.Sleep(10 * time.Millisecond)
	return nil
}
