//go:build darwin

package keyboard

/*
#cgo LDFLAGS: -framework CoreGraphics -framework CoreFoundation -framework Carbon -framework ApplicationServices
#cgo CFLAGS: -Wno-deprecated-declarations
#include <CoreGraphics/CoreGraphics.h>
#include <Carbon/Carbon.h>
#include <ApplicationServices/ApplicationServices.h>
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

// waitForModifierRelease polls CGEventSourceKeyState for all modifier keys
// (Control, Shift, Option, Command) and waits until none are physically pressed.
// Returns the number of milliseconds waited. Max wait is maxWaitMs.
// This is critical when triggered from a hotkey: the user's hotkey modifier
// (e.g. Ctrl from Ctrl+G) may still be physically held when we need to send
// synthetic Cmd+A/C/V events. CGEventPost at kCGHIDEventTap merges with
// hardware state, so held modifiers leak into our synthetic events.
int waitForModifierRelease(int maxWaitMs) {
	// Left/Right: Control, Shift, Option, Command
	CGKeyCode modKeys[] = {0x3B, 0x3E, 0x38, 0x3C, 0x3A, 0x3D, 0x37, 0x36};
	int numKeys = sizeof(modKeys) / sizeof(modKeys[0]);
	int waitedUs = 0;
	int intervalUs = 5000; // 5ms poll interval
	int maxUs = maxWaitMs * 1000;

	while (waitedUs < maxUs) {
		bool anyPressed = false;
		for (int i = 0; i < numKeys; i++) {
			if (CGEventSourceKeyState(kCGEventSourceStateCombinedSessionState, modKeys[i])) {
				anyPressed = true;
				break;
			}
		}
		if (!anyPressed) break;
		usleep(intervalUs);
		waitedUs += intervalUs;
	}
	return waitedUs / 1000;
}

// getSelectedTextAX reads the selected text from the focused UI element using
// the macOS Accessibility API. Returns NULL if no text is selected or if the
// focused element doesn't support kAXSelectedTextAttribute.
// The caller must CFRelease the returned string.
CFStringRef getSelectedTextAX(void) {
	AXUIElementRef systemWide = AXUIElementCreateSystemWide();
	AXUIElementRef focused = NULL;
	AXError err = AXUIElementCopyAttributeValue(systemWide, kAXFocusedUIElementAttribute, (CFTypeRef *)&focused);
	CFRelease(systemWide);
	if (err != kAXErrorSuccess || !focused) return NULL;

	CFTypeRef selectedText = NULL;
	err = AXUIElementCopyAttributeValue(focused, kAXSelectedTextAttribute, &selectedText);
	CFRelease(focused);
	if (err != kAXErrorSuccess || !selectedText) return NULL;

	if (CFGetTypeID(selectedText) != CFStringGetTypeID()) {
		CFRelease(selectedText);
		return NULL;
	}
	return (CFStringRef)selectedText;
}

// getAllTextAX reads all text from the focused UI element using the
// macOS Accessibility API (kAXValueAttribute).
// The caller must CFRelease the returned string.
CFStringRef getAllTextAX(void) {
	AXUIElementRef systemWide = AXUIElementCreateSystemWide();
	AXUIElementRef focused = NULL;
	AXError err = AXUIElementCopyAttributeValue(systemWide, kAXFocusedUIElementAttribute, (CFTypeRef *)&focused);
	CFRelease(systemWide);
	if (err != kAXErrorSuccess || !focused) return NULL;

	CFTypeRef value = NULL;
	err = AXUIElementCopyAttributeValue(focused, kAXValueAttribute, &value);
	CFRelease(focused);
	if (err != kAXErrorSuccess || !value) return NULL;

	if (CFGetTypeID(value) != CFStringGetTypeID()) {
		CFRelease(value);
		return NULL;
	}
	return (CFStringRef)value;
}

// setSelectedTextAX replaces the selected text in the focused UI element.
// Returns 0 on success, -1 on failure.
int setSelectedTextAX(CFStringRef text) {
	AXUIElementRef systemWide = AXUIElementCreateSystemWide();
	AXUIElementRef focused = NULL;
	AXError err = AXUIElementCopyAttributeValue(systemWide, kAXFocusedUIElementAttribute, (CFTypeRef *)&focused);
	CFRelease(systemWide);
	if (err != kAXErrorSuccess || !focused) return -1;

	err = AXUIElementSetAttributeValue(focused, kAXSelectedTextAttribute, text);
	CFRelease(focused);
	return (err == kAXErrorSuccess) ? 0 : -1;
}

// setAllTextAX replaces all text in the focused UI element.
// Returns 0 on success, -1 on failure.
int setAllTextAX(CFStringRef text) {
	AXUIElementRef systemWide = AXUIElementCreateSystemWide();
	AXUIElementRef focused = NULL;
	AXError err = AXUIElementCopyAttributeValue(systemWide, kAXFocusedUIElementAttribute, (CFTypeRef *)&focused);
	CFRelease(systemWide);
	if (err != kAXErrorSuccess || !focused) return -1;

	err = AXUIElementSetAttributeValue(focused, kAXValueAttribute, text);
	CFRelease(focused);
	return (err == kAXErrorSuccess) ? 0 : -1;
}

// getFrontAppName returns the name of the frontmost application.
// The caller must CFRelease the returned string.
CFStringRef getFrontAppName(void) {
	AXUIElementRef systemWide = AXUIElementCreateSystemWide();
	AXUIElementRef focused = NULL;
	AXError err = AXUIElementCopyAttributeValue(systemWide, kAXFocusedUIElementAttribute, (CFTypeRef *)&focused);
	CFRelease(systemWide);
	if (err != kAXErrorSuccess || !focused) return NULL;

	pid_t pid;
	err = AXUIElementGetPid(focused, &pid);
	CFRelease(focused);
	if (err != kAXErrorSuccess) return NULL;

	AXUIElementRef app = AXUIElementCreateApplication(pid);
	CFTypeRef title = NULL;
	err = AXUIElementCopyAttributeValue(app, kAXTitleAttribute, &title);
	CFRelease(app);
	if (err != kAXErrorSuccess || !title) return NULL;

	if (CFGetTypeID(title) != CFStringGetTypeID()) {
		CFRelease(title);
		return NULL;
	}
	return (CFStringRef)title;
}

// getFrontPid returns the PID of the frontmost application.
// Tries AX focused element first, falls back to Carbon GetFrontProcess.
// The AX path fails for Chrome (focused element not accessible), but
// Carbon's GetFrontProcess always returns the frontmost app's PID.
// Returns -1 on failure.
pid_t getFrontPid(void) {
	// Try AX first — works for most apps.
	AXUIElementRef systemWide = AXUIElementCreateSystemWide();
	AXUIElementRef focused = NULL;
	AXError err = AXUIElementCopyAttributeValue(systemWide, kAXFocusedUIElementAttribute, (CFTypeRef *)&focused);
	CFRelease(systemWide);
	if (err == kAXErrorSuccess && focused) {
		pid_t pid;
		err = AXUIElementGetPid(focused, &pid);
		CFRelease(focused);
		if (err == kAXErrorSuccess) return pid;
	}

	// Fallback: Carbon GetFrontProcess — works even when AX can't find
	// the focused element (e.g. Chrome's web content area).
	ProcessSerialNumber psn;
	if (GetFrontProcess(&psn) != noErr) return -1;
	pid_t pid;
	if (GetProcessPID(&psn, &pid) != noErr) return -1;
	return pid;
}

// sendCmdKeyViaAX sends a Cmd+key combo using AXUIElementPostKeyboardEvent.
// This routes through the Accessibility framework instead of the HID event tap,
// which can reach apps (like Chrome) where CGEventPost events are ignored.
// Returns 0 on success, -1 on failure.
int sendCmdKeyViaAX(CGKeyCode key, CGCharCode ch) {
	pid_t pid = getFrontPid();
	if (pid < 0) return -1;

	AXUIElementRef app = AXUIElementCreateApplication(pid);

	// Cmd down → key down → key up → Cmd up
	AXError e1 = AXUIElementPostKeyboardEvent(app, 0, kVK_Command, true);
	usleep(2000);
	AXError e2 = AXUIElementPostKeyboardEvent(app, ch, key, true);
	usleep(2000);
	AXError e3 = AXUIElementPostKeyboardEvent(app, ch, key, false);
	usleep(2000);
	AXError e4 = AXUIElementPostKeyboardEvent(app, 0, kVK_Command, false);

	CFRelease(app);
	return (e1 == kAXErrorSuccess && e2 == kAXErrorSuccess &&
	        e3 == kAXErrorSuccess && e4 == kAXErrorSuccess) ? 0 : -1;
}

// sendCmdKeystrokeViaScript sends a Cmd+key combo using osascript / System Events.
// This is a last-resort fallback that uses yet another routing mechanism.
// Returns 0 on success, -1 on failure.
int sendCmdKeystrokeViaScript(const char *key) {
	char script[256];
	snprintf(script, sizeof(script),
		"tell application \"System Events\" to keystroke \"%s\" using command down", key);
	int ret = 0;
	FILE *fp = popen("/usr/bin/osascript -e '' 2>/dev/null", "r");
	if (fp) pclose(fp); // just test osascript exists

	char cmd[512];
	snprintf(cmd, sizeof(cmd), "/usr/bin/osascript -e '%s' 2>/dev/null", script);
	ret = system(cmd);
	return (ret == 0) ? 0 : -1;
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

	// Explicitly set ONLY the Command flag. Do NOT inherit flags from the
	// modifier event via CGEventGetFlags(modDown) — with CombinedSessionState
	// that would include hardware state (e.g. Ctrl still held from the hotkey
	// trigger), turning Cmd+C into Ctrl+Cmd+C which apps silently ignore.
	CGEventFlags cmdOnly = kCGEventFlagMaskCommand;
	CGEventSetFlags(modDown, cmdOnly);
	CGEventSetFlags(keyDown, cmdOnly);
	CGEventSetFlags(keyUp, cmdOnly);
	CGEventSetFlags(modUp, (CGEventFlags)0);

	// Explicitly set the Unicode character so ALL apps (Cocoa and non-Cocoa)
	// see the correct character regardless of key code interpretation.
	CGEventKeyboardSetUnicodeString(keyDown, 1, &ch);
	CGEventKeyboardSetUnicodeString(keyUp, 1, &ch);

	// Post to kCGHIDEventTap. WaitForModifierRelease must be called before
	// this function to ensure no physical modifier keys are held — HID-level
	// events merge with hardware state. kCGSessionEventTap was tested but
	// does not reliably deliver keyboard events on macOS Ventura 13.7.
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

// sendSingleKey posts a bare key press (no modifiers).
// Returns 0 on success, -1 if event creation failed.
int sendSingleKey(CGKeyCode key) {
	CGEventSourceRef source = CGEventSourceCreate(kCGEventSourceStateCombinedSessionState);
	if (!source) {
		source = CGEventSourceCreate(kCGEventSourceStateHIDSystemState);
	}
	CGEventRef keyDown = CGEventCreateKeyboardEvent(source, key, true);
	CGEventRef keyUp   = CGEventCreateKeyboardEvent(source, key, false);
	if (source) CFRelease(source);
	if (!keyDown || !keyUp) {
		if (keyDown) CFRelease(keyDown);
		if (keyUp)   CFRelease(keyUp);
		return -1;
	}
	// Clear all modifier flags so hardware state doesn't leak in.
	CGEventSetFlags(keyDown, (CGEventFlags)0);
	CGEventSetFlags(keyUp, (CGEventFlags)0);
	CGEventPost(kCGHIDEventTap, keyDown);
	usleep(2000);
	CGEventPost(kCGHIDEventTap, keyUp);
	CFRelease(keyDown);
	CFRelease(keyUp);
	return 0;
}
*/
import "C"

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
	"unsafe"
)

// macOS virtual key codes — ANSI/QWERTY fallbacks if layout lookup fails.
const (
	kVK_Command    = 0x37
	kVK_ANSI_A     = 0x00
	kVK_ANSI_C     = 0x08
	kVK_ANSI_V     = 0x09
	kVK_RightArrow = 0x7C
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

func (s *DarwinSimulator) WaitForModifierRelease() {
	waited := int(C.waitForModifierRelease(C.int(500))) // max 500ms
	if waited > 0 {
		slog.Debug("[keyboard] Waited for modifier release", "ms", waited)
	}
}

// cfStringToGo converts a CFStringRef to a Go string and releases it.
func cfStringToGo(cfStr C.CFStringRef) string {
	if cfStr == 0 {
		return ""
	}
	defer C.CFRelease(C.CFTypeRef(cfStr))
	length := C.CFStringGetLength(cfStr)
	if length == 0 {
		return ""
	}
	bufSize := length*4 + 1 // UTF-8 worst case + null terminator
	buf := C.malloc(C.size_t(bufSize))
	defer C.free(buf)
	if C.CFStringGetCString(cfStr, (*C.char)(buf), C.CFIndex(bufSize), C.kCFStringEncodingUTF8) == 0 {
		return ""
	}
	return C.GoString((*C.char)(buf))
}

// ReadSelectedText reads the selected text from the focused UI element
// using the macOS Accessibility API (kAXSelectedTextAttribute).
// Returns empty string if no selection or if the element doesn't support it.
func (s *DarwinSimulator) ReadSelectedText() string {
	return cfStringToGo(C.getSelectedTextAX())
}

// ReadAllText reads all text from the focused UI element using the
// macOS Accessibility API (kAXValueAttribute).
func (s *DarwinSimulator) ReadAllText() string {
	return cfStringToGo(C.getAllTextAX())
}

// WriteSelectedText replaces the selected text in the focused UI element
// using the macOS Accessibility API (kAXSelectedTextAttribute).
// Returns true on success, false if the app doesn't support it.
func (s *DarwinSimulator) WriteSelectedText(text string) bool {
	cStr := C.CString(text)
	defer C.free(unsafe.Pointer(cStr))
	cfStr := C.CFStringCreateWithCString(0, cStr, C.kCFStringEncodingUTF8)
	if cfStr == 0 {
		return false
	}
	defer C.CFRelease(C.CFTypeRef(cfStr))
	return C.setSelectedTextAX(cfStr) == 0
}

// WriteAllText replaces all text in the focused UI element
// using the macOS Accessibility API (kAXValueAttribute).
// Returns true on success, false if the app doesn't support it.
func (s *DarwinSimulator) WriteAllText(text string) bool {
	cStr := C.CString(text)
	defer C.free(unsafe.Pointer(cStr))
	cfStr := C.CFStringCreateWithCString(0, cStr, C.kCFStringEncodingUTF8)
	if cfStr == 0 {
		return false
	}
	defer C.CFRelease(C.CFTypeRef(cfStr))
	return C.setAllTextAX(cfStr) == 0
}

// FrontAppName returns the name of the frontmost application.
func (s *DarwinSimulator) FrontAppName() string {
	name := cfStringToGo(C.getFrontAppName())
	if name == "" {
		return "(unknown)"
	}
	return name
}

func (s *DarwinSimulator) SelectAllAX() error {
	// Use ANSI key code 0x00, NOT layout-resolved keyA.
	// Same reason as SelectAll(): SDL apps (Firestorm) interpret key codes as
	// QWERTY — on AZERTY keyA=0x0C maps to 'q', sending Cmd+Q (Quit!).
	slog.Debug("[keyboard] SelectAllAX (AXUIElementPostKeyboardEvent)", "keyCode", fmt.Sprintf("0x%02X", kVK_ANSI_A))
	if ret := C.sendCmdKeyViaAX(C.CGKeyCode(kVK_ANSI_A), C.CGCharCode('a')); ret != 0 {
		return fmt.Errorf("AXUIElementPostKeyboardEvent failed for SelectAll")
	}
	time.Sleep(10 * time.Millisecond)
	return nil
}

func (s *DarwinSimulator) CopyAX() error {
	resolveKeys()
	slog.Debug("[keyboard] CopyAX (AXUIElementPostKeyboardEvent)", "keyCode", fmt.Sprintf("0x%02X", keyC))
	if ret := C.sendCmdKeyViaAX(keyC, C.CGCharCode('c')); ret != 0 {
		return fmt.Errorf("AXUIElementPostKeyboardEvent failed for Copy")
	}
	time.Sleep(10 * time.Millisecond)
	return nil
}

func (s *DarwinSimulator) PasteAX() error {
	resolveKeys()
	slog.Debug("[keyboard] PasteAX (AXUIElementPostKeyboardEvent)", "keyCode", fmt.Sprintf("0x%02X", keyV))
	if ret := C.sendCmdKeyViaAX(keyV, C.CGCharCode('v')); ret != 0 {
		return fmt.Errorf("AXUIElementPostKeyboardEvent failed for Paste")
	}
	time.Sleep(10 * time.Millisecond)
	return nil
}

func (s *DarwinSimulator) SelectAllScript() error {
	slog.Debug("[keyboard] SelectAllScript (osascript)")
	cKey := C.CString("a")
	defer C.free(unsafe.Pointer(cKey))
	if ret := C.sendCmdKeystrokeViaScript(cKey); ret != 0 {
		return fmt.Errorf("osascript keystroke failed for SelectAll")
	}
	time.Sleep(50 * time.Millisecond)
	return nil
}

func (s *DarwinSimulator) CopyScript() error {
	slog.Debug("[keyboard] CopyScript (osascript)")
	cKey := C.CString("c")
	defer C.free(unsafe.Pointer(cKey))
	if ret := C.sendCmdKeystrokeViaScript(cKey); ret != 0 {
		return fmt.Errorf("osascript keystroke failed for Copy")
	}
	time.Sleep(50 * time.Millisecond)
	return nil
}

func (s *DarwinSimulator) PasteScript() error {
	slog.Debug("[keyboard] PasteScript (osascript)")
	cKey := C.CString("v")
	defer C.free(unsafe.Pointer(cKey))
	if ret := C.sendCmdKeystrokeViaScript(cKey); ret != 0 {
		return fmt.Errorf("osascript keystroke failed for Paste")
	}
	time.Sleep(50 * time.Millisecond)
	return nil
}

func (s *DarwinSimulator) SelectAll() error {
	// Use ANSI/QWERTY key code 0x00 for 'a', NOT the layout-resolved keyA.
	// This method is only called in Strategy 2 (CGEventPost fallback for non-Cocoa
	// apps like Firestorm/SDL). Non-Cocoa apps interpret key codes using QWERTY
	// mapping — on AZERTY, layout-resolved keyA=0x0C maps to 'q' on QWERTY,
	// turning Cmd+A into Cmd+Q (Quit!). The Unicode char is still set to 'a'
	// via CGEventKeyboardSetUnicodeString for Cocoa compatibility.
	slog.Debug("[keyboard] SelectAll (Cmd+A, ANSI key code)", "keyCode", fmt.Sprintf("0x%02X", kVK_ANSI_A))
	if ret := C.sendKeyComboWithChar(C.CGKeyCode(kVK_Command), C.CGKeyCode(kVK_ANSI_A), C.UniChar('a')); ret != 0 {
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

func (s *DarwinSimulator) PressRight() error {
	slog.Debug("[keyboard] PressRight (Right Arrow)", "keyCode", fmt.Sprintf("0x%02X", kVK_RightArrow))
	if ret := C.sendSingleKey(C.CGKeyCode(kVK_RightArrow)); ret != 0 {
		return fmt.Errorf("CGEventCreate failed for PressRight")
	}
	time.Sleep(10 * time.Millisecond)
	return nil
}
