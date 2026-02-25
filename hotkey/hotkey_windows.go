//go:build windows

package hotkey

import (
	"fmt"
	"log"
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

	ret, _, err := procRegisterHotKey.Call(0, uintptr(id), uintptr(mod), uintptr(vk))
	if ret == 0 {
		log.Printf("[hotkey] RegisterHotKey FAILED for %q (key=%s id=%d mod=0x%X vk=0x%X): %v", name, key, id, mod, vk, err)
		return fmt.Errorf("failed to register hotkey '%s' (id=%d)", key, id)
	}

	log.Printf("[hotkey] Registered %q: key=%s id=%d mod=0x%X vk=0x%X", name, key, id, mod, vk)
	m.hotkeys[name] = registration{id: id, handler: handler}
	return nil
}

func (m *WindowsManager) Unregister(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	reg, ok := m.hotkeys[name]
	if !ok {
		log.Printf("[hotkey] Unregister %q: not found (no-op)", name)
		return nil
	}

	log.Printf("[hotkey] Unregistering %q (id=%d)", name, reg.id)
	procUnregisterHotKey.Call(0, uintptr(reg.id))
	delete(m.hotkeys, name)
	return nil
}

func (m *WindowsManager) Listen() error {
	log.Printf("[hotkey] Entering message loop (registered hotkeys: %d)", len(m.hotkeys))
	var message msg
	for {
		select {
		case <-m.stopChan:
			log.Printf("[hotkey] stopChan signalled — exiting message loop")
			return nil
		default:
		}

		ret, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if ret == 0 {
			log.Printf("[hotkey] GetMessage returned 0 (WM_QUIT) — exiting message loop")
			break
		}

		log.Printf("[hotkey] GetMessage: msg=0x%04X wParam=0x%X lParam=0x%X", message.message, message.wParam, message.lParam)

		if message.message == wmHotkey {
			id := int(message.wParam)
			log.Printf("[hotkey] WM_HOTKEY received: id=%d", id)
			m.mu.Lock()
			matched := false
			for name, reg := range m.hotkeys {
				if reg.id == id {
					log.Printf("[hotkey] Dispatching handler for %q (id=%d)", name, id)
					go reg.handler()
					matched = true
					break
				}
			}
			if !matched {
				log.Printf("[hotkey] WARNING: no registered handler for hotkey id=%d", id)
			}
			m.mu.Unlock()
		}
	}
	return nil
}

func (m *WindowsManager) Stop() {
	close(m.stopChan)
}
