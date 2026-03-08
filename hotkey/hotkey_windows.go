//go:build windows

package hotkey

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32                = syscall.NewLazyDLL("user32.dll")
	kernel32              = syscall.NewLazyDLL("kernel32.dll")
	procRegisterHotKey    = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey  = user32.NewProc("UnregisterHotKey")
	procGetMessage        = user32.NewProc("GetMessageW")
	procPostThreadMessage = user32.NewProc("PostThreadMessageW")
	procGetCurrentThreadID = kernel32.NewProc("GetCurrentThreadId")
)

// Virtual key codes for function keys and modifiers.
const (
	modAlt   = 0x0001
	modCtrl  = 0x0002
	modShift = 0x0004

	vkReturn = 0x0D
	vkTab    = 0x09
	vkEscape = 0x1B
	vkSpace  = 0x20
	vkDelete = 0x2E
)

const (
	wmHotkey = 0x0312
	wmQuit   = 0x0012
)

// msg represents a Windows MSG structure.
type msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ x, y int32 }
}

type registration struct {
	id      int
	handler Handler
}

// WindowsManager implements the Manager interface for Windows.
type WindowsManager struct {
	mu       sync.Mutex
	hotkeys  map[string]registration
	nextID   int
	stopChan chan struct{}
	stopOnce sync.Once
	threadID uint32
}

// NewWindowsManager creates a new Windows hotkey manager.
func NewWindowsManager() *WindowsManager {
	return &WindowsManager{
		hotkeys:  make(map[string]registration),
		nextID:   1,
		stopChan: make(chan struct{}),
	}
}

// parseKey converts a key string like "F7" or "Ctrl+F8" into modifier and vk code.
func parseKey(key string) (uint32, uint32, error) {
	var mod uint32
	parts := strings.Split(key, "+")
	keyName := parts[len(parts)-1]

	for _, p := range parts[:len(parts)-1] {
		switch strings.ToLower(strings.TrimSpace(p)) {
		case "ctrl":
			mod |= modCtrl
		case "alt":
			mod |= modAlt
		case "shift":
			mod |= modShift
		}
	}

	vk, ok := keyMap[strings.ToLower(strings.TrimSpace(keyName))]
	if !ok {
		return 0, 0, fmt.Errorf("unknown key: %s", keyName)
	}
	return mod, vk, nil
}

var keyMap = map[string]uint32{
	// Letters (VK_A=0x41 .. VK_Z=0x5A)
	"a": 0x41, "b": 0x42, "c": 0x43, "d": 0x44, "e": 0x45,
	"f": 0x46, "g": 0x47, "h": 0x48, "i": 0x49, "j": 0x4A,
	"k": 0x4B, "l": 0x4C, "m": 0x4D, "n": 0x4E, "o": 0x4F,
	"p": 0x50, "q": 0x51, "r": 0x52, "s": 0x53, "t": 0x54,
	"u": 0x55, "v": 0x56, "w": 0x57, "x": 0x58, "y": 0x59, "z": 0x5A,
	// Digits (VK_0=0x30 .. VK_9=0x39)
	"0": 0x30, "1": 0x31, "2": 0x32, "3": 0x33, "4": 0x34,
	"5": 0x35, "6": 0x36, "7": 0x37, "8": 0x38, "9": 0x39,
	// Function keys (VK_F1=0x70 .. VK_F12=0x7B)
	"f1": 0x70, "f2": 0x71, "f3": 0x72, "f4": 0x73, "f5": 0x74, "f6": 0x75,
	"f7": 0x76, "f8": 0x77, "f9": 0x78, "f10": 0x79, "f11": 0x7A, "f12": 0x7B,
	// Special keys
	"space": vkSpace, "escape": vkEscape, "tab": vkTab,
	"return": vkReturn, "enter": vkReturn, "delete": vkDelete,
}

func (m *WindowsManager) Register(name string, key string, handler Handler) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mod, vk, err := parseKey(key)
	if err != nil {
		return fmt.Errorf("failed to parse key '%s': %w", key, err)
	}

	id := m.nextID
	m.nextID++

	ret, _, err := procRegisterHotKey.Call(0, uintptr(id), uintptr(mod), uintptr(vk))
	if ret == 0 {
		slog.Error("RegisterHotKey failed", "name", name, "key", key, "id", id, "mod", fmt.Sprintf("0x%X", mod), "vk", fmt.Sprintf("0x%X", vk), "error", err)
		return fmt.Errorf("failed to register hotkey '%s' (id=%d)", key, id)
	}

	slog.Debug("Hotkey registered", "name", name, "key", key, "id", id, "mod", fmt.Sprintf("0x%X", mod), "vk", fmt.Sprintf("0x%X", vk))
	m.hotkeys[name] = registration{id: id, handler: handler}
	return nil
}

func (m *WindowsManager) Unregister(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	reg, ok := m.hotkeys[name]
	if !ok {
		slog.Debug("Hotkey unregister: not found (no-op)", "name", name)
		return nil
	}

	slog.Debug("Hotkey unregistering", "name", name, "id", reg.id)
	procUnregisterHotKey.Call(0, uintptr(reg.id))
	delete(m.hotkeys, name)
	return nil
}

func (m *WindowsManager) Listen() error {
	tid, _, _ := procGetCurrentThreadID.Call()
	m.threadID = uint32(tid)
	slog.Debug("Entering message loop", "registered_hotkeys", len(m.hotkeys), "threadID", m.threadID)

	// Unregister all hotkeys when Listen exits, regardless of exit path.
	// UnregisterHotKey must be called from the same OS thread that called
	// RegisterHotKey (hwnd=0). The caller locks this goroutine to its thread.
	defer func() {
		slog.Debug("Listen() exiting — unregistering hotkeys")
		m.mu.Lock()
		for name, reg := range m.hotkeys {
			procUnregisterHotKey.Call(0, uintptr(reg.id))
			slog.Debug("Hotkey unregistered on listen exit", "name", name, "id", reg.id)
		}
		m.mu.Unlock()
	}()

	// Heartbeat goroutine — logs listener state every 30s to confirm the
	// message loop thread is alive and hotkeys are still registered.
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-m.stopChan:
				return
			case <-ticker.C:
				m.mu.Lock()
				names := make([]string, 0, len(m.hotkeys))
				for name := range m.hotkeys {
					names = append(names, name)
				}
				m.mu.Unlock()
				slog.Debug("Hotkey listener heartbeat",
					"threadID", m.threadID,
					"registered", len(names),
					"names", fmt.Sprintf("%v", names),
				)
			}
		}
	}()

	var message msg
	for {
		select {
		case <-m.stopChan:
			slog.Debug("stopChan signalled — exiting message loop")
			return nil
		default:
		}

		ret, _, err := procGetMessage.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)

		// GetMessage returns: >0 = message, 0 = WM_QUIT, -1 = error.
		if int32(ret) == -1 {
			slog.Error("GetMessage returned -1 (error)", "error", err, "threadID", m.threadID)
			return fmt.Errorf("GetMessage failed: %w", err)
		}
		if ret == 0 {
			slog.Debug("GetMessage returned 0 (WM_QUIT) — exiting message loop")
			return nil
		}

		slog.Debug("GetMessage", "msg", fmt.Sprintf("0x%04X", message.message), "wParam", fmt.Sprintf("0x%X", message.wParam), "lParam", fmt.Sprintf("0x%X", message.lParam))

		if message.message == wmHotkey {
			id := int(message.wParam)
			slog.Debug("WM_HOTKEY received", "id", id)
			m.mu.Lock()
			matched := false
			for name, reg := range m.hotkeys {
				if reg.id == id {
					slog.Debug("Dispatching hotkey handler", "name", name, "id", id)
					go reg.handler()
					matched = true
					break
				}
			}
			if !matched {
				slog.Warn("No registered handler for hotkey", "id", id)
			}
			m.mu.Unlock()
		}
	}
}

// Reregister is a no-op on Windows. The Windows hotkey API doesn't suffer from
// the NSMenu modal event loop issue that affects macOS Carbon hotkeys.
func (m *WindowsManager) Reregister() error { return nil }

func (m *WindowsManager) Stop() {
	m.stopOnce.Do(func() {
		close(m.stopChan)
		if m.threadID != 0 {
			procPostThreadMessage.Call(uintptr(m.threadID), uintptr(wmQuit), 0, 0)
		}
	})
}
