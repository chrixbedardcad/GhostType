//go:build windows

package hotkey

import (
	"fmt"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

var (
	user32              = syscall.NewLazyDLL("user32.dll")
	procRegisterHotKey  = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey = user32.NewProc("UnregisterHotKey")
	procGetMessage      = user32.NewProc("GetMessageW")
)

// Virtual key codes for function keys and modifiers.
const (
	modAlt   = 0x0001
	modCtrl  = 0x0002
	modShift = 0x0004

	vkEscape = 0x1B
	vkF7     = 0x76
	vkF8     = 0x77
	vkF9     = 0x78
)

const wmHotkey = 0x0312

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
	"f7":     vkF7,
	"f8":     vkF8,
	"f9":     vkF9,
	"escape": vkEscape,
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

	ret, _, _ := procRegisterHotKey.Call(0, uintptr(id), uintptr(mod), uintptr(vk))
	if ret == 0 {
		return fmt.Errorf("failed to register hotkey '%s' (id=%d)", key, id)
	}

	m.hotkeys[name] = registration{id: id, handler: handler}
	return nil
}

func (m *WindowsManager) Unregister(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	reg, ok := m.hotkeys[name]
	if !ok {
		return nil
	}

	procUnregisterHotKey.Call(0, uintptr(reg.id))
	delete(m.hotkeys, name)
	return nil
}

func (m *WindowsManager) Listen() error {
	var message msg
	for {
		select {
		case <-m.stopChan:
			return nil
		default:
		}

		ret, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if ret == 0 {
			break
		}

		if message.message == wmHotkey {
			id := int(message.wParam)
			m.mu.Lock()
			for _, reg := range m.hotkeys {
				if reg.id == id {
					go reg.handler()
					break
				}
			}
			m.mu.Unlock()
		}
	}
	return nil
}

func (m *WindowsManager) Stop() {
	close(m.stopChan)
}
