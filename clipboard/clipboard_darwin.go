//go:build darwin

package clipboard

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit

#import <AppKit/AppKit.h>

// readClipboardNS reads the clipboard using NSPasteboard.
// Returns a malloc'd UTF-8 C string (caller must free), or NULL if empty.
const char* readClipboardNS(void) {
	NSPasteboard *pb = [NSPasteboard generalPasteboard];
	NSString *str = [pb stringForType:NSPasteboardTypeString];
	if (!str || [str length] == 0) return NULL;
	const char *utf8 = [str UTF8String];
	if (!utf8) return NULL;
	return strdup(utf8);
}

// writeClipboardNS writes UTF-8 text to the clipboard using NSPasteboard.
// NSPasteboard handles multi-flavor conversion automatically (UTF-8, UTF-16, etc.).
// Returns 0 on success, -1 on failure.
int writeClipboardNS(const char *text) {
	if (!text) return -1;
	NSPasteboard *pb = [NSPasteboard generalPasteboard];
	[pb clearContents];
	NSString *str = [NSString stringWithUTF8String:text];
	if (!str) return -1;
	BOOL ok = [pb setString:str forType:NSPasteboardTypeString];
	return ok ? 0 : -1;
}

// clearClipboardNS clears all clipboard content.
// Returns 0 on success, -1 on failure.
int clearClipboardNS(void) {
	NSPasteboard *pb = [NSPasteboard generalPasteboard];
	[pb clearContents];
	return 0;
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// NewDarwinClipboard creates a Clipboard using the native macOS NSPasteboard API.
// NSPasteboard handles multi-flavor conversion automatically, so all apps
// can read the clipboard regardless of whether they expect UTF-8 or UTF-16.
func NewDarwinClipboard() *Clipboard {
	return New(darwinRead, darwinWrite).WithClear(darwinClear)
}

func darwinRead() (string, error) {
	cStr := C.readClipboardNS()
	if cStr == nil {
		return "", nil
	}
	defer C.free(unsafe.Pointer(cStr))
	return C.GoString(cStr), nil
}

func darwinWrite(text string) error {
	if len(text) == 0 {
		return darwinClear()
	}
	cStr := C.CString(text)
	defer C.free(unsafe.Pointer(cStr))
	if ret := C.writeClipboardNS(cStr); ret != 0 {
		return fmt.Errorf("native clipboard write failed")
	}
	return nil
}

func darwinClear() error {
	if ret := C.clearClipboardNS(); ret != 0 {
		return fmt.Errorf("native clipboard clear failed")
	}
	return nil
}
