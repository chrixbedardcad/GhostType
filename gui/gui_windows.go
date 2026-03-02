//go:build windows

package gui

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/chrixbedardcad/GhostType/config"
	"github.com/chrixbedardcad/GhostType/llm"
	webview2 "github.com/jchv/go-webview2"
)

var (
	user32                  = syscall.NewLazyDLL("user32.dll")
	procShowWindow          = user32.NewProc("ShowWindow")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
)

//go:embed frontend/index.html
var frontendFS embed.FS

// settingsGuard prevents multiple settings windows.
var (
	settingsOpen   bool
	settingsOpenMu sync.Mutex
)

// guiLog logs to both slog (log file) and fmt (stdout, if console attached).
func guiLog(msg string, args ...any) {
	formatted := fmt.Sprintf(msg, args...)
	fmt.Println(formatted)
	slog.Info(formatted)
}

// ShowSettingsBlocking opens the settings window and blocks until it closes.
// Returns the (potentially updated) config. Used by main.go on first launch.
func ShowSettingsBlocking(cfg *config.Config, configPath string) *config.Config {
	guiLog("[GUI] ShowSettingsBlocking called, configPath=%s", configPath)
	updated := showWindow(cfg, configPath)
	if updated != nil {
		return updated
	}
	return cfg
}

// ShowSettings opens the settings window. Non-blocking (for tray).
func ShowSettings(cfg *config.Config, configPath string) {
	guiLog("[GUI] ShowSettings (async) called")
	settingsOpenMu.Lock()
	if settingsOpen {
		settingsOpenMu.Unlock()
		guiLog("[GUI] ShowSettings: window already open, skipping")
		return
	}
	settingsOpen = true
	settingsOpenMu.Unlock()
	guiLog("[GUI] ShowSettings: launching goroutine")

	go func() {
		defer func() {
			if r := recover(); r != nil {
				guiLog("[GUI] PANIC in ShowSettings goroutine: %v", r)
			}
			settingsOpenMu.Lock()
			settingsOpen = false
			settingsOpenMu.Unlock()
			guiLog("[GUI] ShowSettings goroutine exited")
		}()
		showWindow(cfg, configPath)
	}()
}

// showWindow creates the WebView2 window, binds Go functions, and blocks until closed.
// Returns the updated config if any saves occurred, or nil if no changes were made.
func showWindow(cfg *config.Config, configPath string) *config.Config {
	guiLog("[GUI] showWindow entered")

	// WebView2 requires the window and message loop on the same OS thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	guiLog("[GUI] OS thread locked")

	// Work on a copy so cancelled edits don't corrupt the live config.
	cfgCopy := *cfg
	if cfg.LLMProviders != nil {
		cfgCopy.LLMProviders = make(map[string]config.LLMProviderDef, len(cfg.LLMProviders))
		for k, v := range cfg.LLMProviders {
			cfgCopy.LLMProviders[k] = v
		}
	}

	var saved bool

	guiLog("[GUI] Creating WebView2 window...")
	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     true,
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title:  "GhostType Settings",
			Width:  720,
			Height: 580,
			Center: true,
		},
	})
	if w == nil {
		guiLog("[GUI] ERROR: NewWithOptions returned nil — WebView2 runtime may not be installed")
		return nil
	}
	defer w.Destroy()
	guiLog("[GUI] WebView2 window created OK")

	// --- Bind Go functions for JS bridge ---
	guiLog("[GUI] Binding JS functions...")

	w.Bind("getConfig", func() string {
		guiLog("[GUI] JS called: getConfig")
		data, err := json.Marshal(&cfgCopy)
		if err != nil {
			return "{}"
		}
		return string(data)
	})

	w.Bind("getKnownModels", func(provider string) string {
		guiLog("[GUI] JS called: getKnownModels(%s)", provider)
		models := KnownModels(provider)
		data, _ := json.Marshal(models)
		return string(data)
	})

	w.Bind("saveProvider", func(label, provider, apiKey, model, endpoint, originalLabel string) string {
		guiLog("[GUI] JS called: saveProvider(%s, %s)", label, provider)
		if label == "" {
			return "error: label is required"
		}

		// If editing (originalLabel set) and label changed, remove old entry.
		if originalLabel != "" && originalLabel != label {
			delete(cfgCopy.LLMProviders, originalLabel)
			if cfgCopy.DefaultLLM == originalLabel {
				cfgCopy.DefaultLLM = label
			}
		}

		if cfgCopy.LLMProviders == nil {
			cfgCopy.LLMProviders = make(map[string]config.LLMProviderDef)
		}

		cfgCopy.LLMProviders[label] = config.LLMProviderDef{
			Provider:    provider,
			APIKey:      apiKey,
			Model:       model,
			APIEndpoint: endpoint,
		}

		// Set as default if first provider.
		if len(cfgCopy.LLMProviders) == 1 || cfgCopy.DefaultLLM == "" {
			cfgCopy.DefaultLLM = label
		}

		if err := config.WriteDefault(configPath, &cfgCopy); err != nil {
			return fmt.Sprintf("error: %v", err)
		}

		saved = true
		guiLog("[GUI] Provider saved: label=%s provider=%s", label, provider)
		return "ok"
	})

	w.Bind("deleteProvider", func(label string) string {
		guiLog("[GUI] JS called: deleteProvider(%s)", label)
		delete(cfgCopy.LLMProviders, label)
		if cfgCopy.DefaultLLM == label {
			cfgCopy.DefaultLLM = ""
			for k := range cfgCopy.LLMProviders {
				cfgCopy.DefaultLLM = k
				break
			}
		}

		if err := config.WriteDefault(configPath, &cfgCopy); err != nil {
			return fmt.Sprintf("error: %v", err)
		}

		saved = true
		return "ok"
	})

	w.Bind("setDefault", func(label string) string {
		guiLog("[GUI] JS called: setDefault(%s)", label)
		if _, ok := cfgCopy.LLMProviders[label]; !ok {
			return "error: provider not found"
		}
		cfgCopy.DefaultLLM = label

		if err := config.WriteDefault(configPath, &cfgCopy); err != nil {
			return fmt.Sprintf("error: %v", err)
		}

		saved = true
		return "ok"
	})

	w.Bind("testConnection", func(provider, apiKey, model, endpoint string) string {
		guiLog("[GUI] JS called: testConnection(%s)", provider)
		def := config.LLMProviderDef{
			Provider:    provider,
			APIKey:      apiKey,
			Model:       model,
			APIEndpoint: endpoint,
			MaxTokens:   32,
			TimeoutMs:   10000,
		}

		client, err := llm.NewClientFromDef(def)
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, err = client.Send(ctx, llm.Request{
			Prompt:    "Reply with OK",
			Text:      "test",
			MaxTokens: 32,
		})
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}

		return "ok"
	})

	w.Bind("testProvider", func(label string) string {
		guiLog("[GUI] JS called: testProvider(%s)", label)
		def, ok := cfgCopy.LLMProviders[label]
		if !ok {
			return "error: provider not found"
		}

		def.MaxTokens = 32
		def.TimeoutMs = 10000

		client, err := llm.NewClientFromDef(def)
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, err = client.Send(ctx, llm.Request{
			Prompt:    "Reply with OK",
			Text:      "test",
			MaxTokens: 32,
		})
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}

		return "ok"
	})

	w.Bind("openConfigFile", func() string {
		guiLog("[GUI] JS called: openConfigFile")
		cmd := exec.Command("cmd", "/c", "start", "", configPath)
		if err := cmd.Start(); err != nil {
			guiLog("[GUI] ERROR: Failed to open config file: %v", err)
		}
		return "ok"
	})

	w.Bind("closeWindow", func() string {
		guiLog("[GUI] JS called: closeWindow")
		w.Terminate()
		return "ok"
	})

	guiLog("[GUI] All JS functions bound")

	// Load the embedded HTML.
	html, err := frontendFS.ReadFile("frontend/index.html")
	if err != nil {
		guiLog("[GUI] ERROR: Failed to read embedded HTML: %v", err)
		return nil
	}
	guiLog("[GUI] Loaded HTML (%d bytes), calling SetHtml...", len(html))
	w.SetHtml(string(html))

	// Force the window to the foreground so the user can see it.
	hwnd := w.Window()
	if hwnd != nil {
		h := uintptr(unsafe.Pointer(hwnd))
		procShowWindow.Call(h, 5) // SW_SHOW = 5
		procSetForegroundWindow.Call(h)
		guiLog("[GUI] Window brought to foreground (hwnd=0x%x)", h)
	} else {
		guiLog("[GUI] WARNING: Window() returned nil, cannot bring to foreground")
	}

	guiLog("[GUI] Calling Run (blocks until window closes)...")

	// Run blocks until the window is closed.
	w.Run()

	guiLog("[GUI] Run returned, window closed")
	if saved {
		return &cfgCopy
	}
	return nil
}
