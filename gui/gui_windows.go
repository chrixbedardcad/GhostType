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
	"time"

	"github.com/chrixbedardcad/GhostType/config"
	"github.com/chrixbedardcad/GhostType/llm"
	webview2 "github.com/jchv/go-webview2"
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

// ShowWizard opens the setup wizard and blocks until the user saves or cancels.
// Returns the (potentially updated) config.
func ShowWizard(cfg *config.Config, configPath string) *config.Config {
	guiLog("[GUI] ShowWizard called, configPath=%s", configPath)
	result := showWindow(cfg, configPath, "wizard")
	guiLog("[GUI] ShowWizard returned: saved=%v", result.Saved)
	if result.Saved && result.Config != nil {
		return result.Config
	}
	return cfg
}

// showAsync opens a GUI window in a goroutine (non-blocking).
// Only one window can be open at a time (shared guard).
func showAsync(cfg *config.Config, configPath string, view string) {
	guiLog("[GUI] showAsync called: view=%s", view)
	settingsOpenMu.Lock()
	if settingsOpen {
		settingsOpenMu.Unlock()
		guiLog("[GUI] showAsync: window already open, skipping")
		return
	}
	settingsOpen = true
	settingsOpenMu.Unlock()
	guiLog("[GUI] showAsync: launching goroutine")

	go func() {
		defer func() {
			if r := recover(); r != nil {
				guiLog("[GUI] PANIC in showAsync goroutine: %v", r)
			}
			settingsOpenMu.Lock()
			settingsOpen = false
			settingsOpenMu.Unlock()
			guiLog("[GUI] showAsync goroutine exited")
		}()
		showWindow(cfg, configPath, view)
	}()
}

// ShowSettings opens the settings (provider list) window. Non-blocking.
func ShowSettings(cfg *config.Config, configPath string) {
	showAsync(cfg, configPath, "settings")
}

// ShowWizardAsync opens the setup wizard from the tray. Non-blocking.
func ShowWizardAsync(cfg *config.Config, configPath string) {
	showAsync(cfg, configPath, "wizard")
}

func showWindow(cfg *config.Config, configPath string, initialView string) Result {
	guiLog("[GUI] showWindow entered: view=%s", initialView)

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

	result := Result{Config: &cfgCopy}

	guiLog("[GUI] Creating WebView2 window...")
	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     true,
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title:  "GhostType Setup",
			Width:  720,
			Height: 580,
			Center: true,
		},
	})
	if w == nil {
		guiLog("[GUI] ERROR: NewWithOptions returned nil — WebView2 runtime may not be installed")
		return result
	}
	defer w.Destroy()
	guiLog("[GUI] WebView2 window created OK")

	// --- Bind Go functions for JS bridge ---
	guiLog("[GUI] Binding JS functions...")

	w.Bind("getInitialView", func() string {
		guiLog("[GUI] JS called: getInitialView")
		return initialView
	})

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

		result.Saved = true
		result.Config = &cfgCopy
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

		result.Saved = true
		result.Config = &cfgCopy
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

		result.Saved = true
		result.Config = &cfgCopy
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
		return result
	}
	guiLog("[GUI] Loaded HTML (%d bytes), calling SetHtml...", len(html))
	w.SetHtml(string(html))
	guiLog("[GUI] SetHtml done, calling Run (blocks until window closes)...")

	// Run blocks until the window is closed.
	w.Run()

	guiLog("[GUI] Run returned, window closed")
	return result
}
