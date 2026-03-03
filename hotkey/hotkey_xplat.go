//go:build !windows

package hotkey

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"golang.design/x/hotkey"
)

// XPlatManager implements the Manager interface using golang.design/x/hotkey.
// Works on macOS and Linux. On macOS, the caller must ensure the main thread
// event loop is running (handled by Wails app.Run() in the unified lifecycle).
type XPlatManager struct {
	mu       sync.Mutex
	hotkeys  map[string]*hkEntry
	stopChan chan struct{}
	stopOnce sync.Once
}

type hkEntry struct {
	hk      *hotkey.Hotkey
	handler Handler
}

// NewXPlatManager creates a new cross-platform hotkey manager.
func NewXPlatManager() *XPlatManager {
	return &XPlatManager{
		hotkeys:  make(map[string]*hkEntry),
		stopChan: make(chan struct{}),
	}
}

func (m *XPlatManager) Register(name string, key string, handler Handler) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mods, k, err := parseKeyXPlat(key)
	if err != nil {
		return fmt.Errorf("failed to parse key '%s': %w", key, err)
	}

	hk := hotkey.New(mods, k)
	if err := hk.Register(); err != nil {
		slog.Error("Failed to register hotkey", "name", name, "key", key, "error", err)
		return fmt.Errorf("failed to register hotkey '%s': %w", key, err)
	}

	slog.Debug("Hotkey registered", "name", name, "key", key)
	m.hotkeys[name] = &hkEntry{hk: hk, handler: handler}
	return nil
}

func (m *XPlatManager) Unregister(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.hotkeys[name]
	if !ok {
		return nil
	}

	entry.hk.Unregister()
	delete(m.hotkeys, name)
	slog.Debug("Hotkey unregistered", "name", name)
	return nil
}

func (m *XPlatManager) Listen() error {
	slog.Debug("Starting hotkey listener", "registered", len(m.hotkeys))

	// Collect all registered hotkeys for multiplexed listening.
	m.mu.Lock()
	entries := make([]*hkEntry, 0, len(m.hotkeys))
	for _, entry := range m.hotkeys {
		entries = append(entries, entry)
	}
	m.mu.Unlock()

	// Start a goroutine for each hotkey to listen for keydown events.
	var wg sync.WaitGroup
	for _, entry := range entries {
		wg.Add(1)
		go func(e *hkEntry) {
			defer wg.Done()
			for {
				select {
				case <-m.stopChan:
					return
				case <-e.hk.Keydown():
					go e.handler()
				}
			}
		}(entry)
	}

	// Block until stop is called.
	<-m.stopChan
	wg.Wait()
	return nil
}

func (m *XPlatManager) Stop() {
	m.stopOnce.Do(func() {
		close(m.stopChan)
		m.mu.Lock()
		defer m.mu.Unlock()
		for name, entry := range m.hotkeys {
			entry.hk.Unregister()
			slog.Debug("Hotkey unregistered on stop", "name", name)
		}
	})
}

// parseKeyXPlat converts a key string like "Ctrl+G" or "F7" to hotkey modifiers and key.
func parseKeyXPlat(key string) ([]hotkey.Modifier, hotkey.Key, error) {
	var mods []hotkey.Modifier
	parts := strings.Split(key, "+")
	keyName := strings.ToLower(strings.TrimSpace(parts[len(parts)-1]))

	for _, p := range parts[:len(parts)-1] {
		switch strings.ToLower(strings.TrimSpace(p)) {
		case "ctrl", "cmd":
			mods = append(mods, hotkey.ModCtrl)
		case "alt", "option":
			mods = append(mods, hotkey.Mod1) // Mod1 = Alt on X11/Windows, Option on macOS
		case "shift":
			mods = append(mods, hotkey.ModShift)
		default:
			return nil, 0, fmt.Errorf("unknown modifier: %s", p)
		}
	}

	k, ok := xplatKeyMap[keyName]
	if !ok {
		return nil, 0, fmt.Errorf("unknown key: %s", keyName)
	}
	return mods, k, nil
}

var xplatKeyMap = map[string]hotkey.Key{
	"a": hotkey.KeyA, "b": hotkey.KeyB, "c": hotkey.KeyC,
	"d": hotkey.KeyD, "e": hotkey.KeyE, "f": hotkey.KeyF,
	"g": hotkey.KeyG, "h": hotkey.KeyH, "i": hotkey.KeyI,
	"j": hotkey.KeyJ, "k": hotkey.KeyK, "l": hotkey.KeyL,
	"m": hotkey.KeyM, "n": hotkey.KeyN, "o": hotkey.KeyO,
	"p": hotkey.KeyP, "q": hotkey.KeyQ, "r": hotkey.KeyR,
	"s": hotkey.KeyS, "t": hotkey.KeyT, "u": hotkey.KeyU,
	"v": hotkey.KeyV, "w": hotkey.KeyW, "x": hotkey.KeyX,
	"y": hotkey.KeyY, "z": hotkey.KeyZ,
	"0": hotkey.Key0, "1": hotkey.Key1, "2": hotkey.Key2,
	"3": hotkey.Key3, "4": hotkey.Key4, "5": hotkey.Key5,
	"6": hotkey.Key6, "7": hotkey.Key7, "8": hotkey.Key8,
	"9": hotkey.Key9,
	"f1": hotkey.KeyF1, "f2": hotkey.KeyF2, "f3": hotkey.KeyF3,
	"f4": hotkey.KeyF4, "f5": hotkey.KeyF5, "f6": hotkey.KeyF6,
	"f7": hotkey.KeyF7, "f8": hotkey.KeyF8, "f9": hotkey.KeyF9,
	"f10": hotkey.KeyF10, "f11": hotkey.KeyF11, "f12": hotkey.KeyF12,
	"space":  hotkey.KeySpace,
	"escape": hotkey.KeyEscape,
	"return": hotkey.KeyReturn, "enter": hotkey.KeyReturn,
	"tab":    hotkey.KeyTab,
	"delete": hotkey.KeyDelete,
}
