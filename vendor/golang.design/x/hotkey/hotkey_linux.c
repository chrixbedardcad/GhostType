// Copyright 2021 The golang.design Initiative Authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.
//
// Written by Changkun Ou <changkun.de>

//go:build linux

#include <stdint.h>
#include <stdio.h>
#include <X11/Xlib.h>
#include <X11/Xutil.h>

extern void hotkeyDown(uintptr_t hkhandle);
extern void hotkeyUp(uintptr_t hkhandle);

// Flag set by custom X error handler when XGrabKey fails (e.g., BadAccess).
static volatile int grabFailed = 0;

static int grabErrorHandler(Display* dpy, XErrorEvent* pErr) {
	if (pErr->request_code == 33 && pErr->error_code == BadAccess) {
		// X_GrabKey (33) with BadAccess means another client already grabbed this key.
		grabFailed = 1;
		return 0;
	}
	// For other errors, just ignore — we only care about grab failures here.
	return 0;
}

int displayTest() {
	Display* d = NULL;
	for (int i = 0; i < 42; i++) {
		d = XOpenDisplay(0);
		if (d == NULL) continue;
		break;
	}
	if (d == NULL) {
		return -1;
	}
	return 0;
}

// waitHotkey blocks until the hotkey is triggered.
// Returns -1 if X display cannot be opened, -2 if the hotkey is already
// grabbed by another client, 0 on success.
int waitHotkey(uintptr_t hkhandle, unsigned int mod, int key) {
	Display* d = NULL;
	for (int i = 0; i < 42; i++) {
		d = XOpenDisplay(0);
		if (d == NULL) continue;
		break;
	}
	if (d == NULL) {
		return -1;
	}
	int keycode = XKeysymToKeycode(d, key);

	// Install custom error handler to catch BadAccess from XGrabKey
	// instead of crashing the program.
	grabFailed = 0;
	int (*oldHandler)(Display*, XErrorEvent*) = XSetErrorHandler(grabErrorHandler);

	XGrabKey(d, keycode, mod, DefaultRootWindow(d), False, GrabModeAsync, GrabModeAsync);
	XSync(d, False); // Flush to trigger any pending X errors

	// Restore original error handler.
	XSetErrorHandler(oldHandler);

	if (grabFailed) {
		XCloseDisplay(d);
		return -2; // Hotkey already grabbed by another client
	}

	XSelectInput(d, DefaultRootWindow(d), KeyPressMask);
	XEvent ev;
	while(1) {
		XNextEvent(d, &ev);
		switch(ev.type) {
		case KeyPress:
			hotkeyDown(hkhandle);
			continue;
		case KeyRelease:
			hotkeyUp(hkhandle);
			XUngrabKey(d, keycode, mod, DefaultRootWindow(d));
			XSync(d, False); // Ensure ungrab completes before closing
			XCloseDisplay(d);
			return 0;
		}
	}
}