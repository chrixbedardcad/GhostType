package gui

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"runtime"
	"time"

	"github.com/chrixbedardcad/GhostType/config"
	"github.com/chrixbedardcad/GhostType/internal/version"
	"github.com/chrixbedardcad/GhostType/llm"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// guiLog logs to both slog (log file) and fmt (stdout, if console attached).
func guiLog(msg string, args ...any) {
	formatted := fmt.Sprintf(msg, args...)
	fmt.Println(formatted)
	slog.Info(formatted)
}

// SettingsService exposes all Settings GUI bindings as public methods.
// Wails v3 auto-binds all public methods for JS access via wails.Call().
type SettingsService struct {
	cfgCopy    *config.Config
	configPath string
	onSaved    func()
	app        *application.App
	saved      bool
}

// clearLegacyAndSave writes the config to disk, clearing legacy flat fields.
func (s *SettingsService) clearLegacyAndSave() error {
	s.cfgCopy.LLMProvider = ""
	s.cfgCopy.APIKey = ""
	s.cfgCopy.Model = ""
	s.cfgCopy.APIEndpoint = ""
	if err := config.WriteDefault(s.configPath, s.cfgCopy); err != nil {
		return err
	}
	s.saved = true
	if s.onSaved != nil {
		s.onSaved()
	}
	return nil
}

// GetVersion returns the app version string.
func (s *SettingsService) GetVersion() string {
	return version.Version
}

// GetConfig returns the current config as JSON.
func (s *SettingsService) GetConfig() string {
	guiLog("[GUI] JS called: GetConfig")
	data, err := json.Marshal(s.cfgCopy)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// GetKnownModels returns a curated model list for the given provider.
func (s *SettingsService) GetKnownModels(provider string) string {
	guiLog("[GUI] JS called: GetKnownModels(%s)", provider)
	models := KnownModels(provider)
	data, _ := json.Marshal(models)
	return string(data)
}

// SaveProvider saves or updates a named LLM provider.
func (s *SettingsService) SaveProvider(label, provider, apiKey, model, endpoint, originalLabel string) string {
	guiLog("[GUI] JS called: SaveProvider(%s, %s)", label, provider)
	if label == "" {
		return "error: label is required"
	}

	if originalLabel != "" && originalLabel != label {
		delete(s.cfgCopy.LLMProviders, originalLabel)
		if s.cfgCopy.DefaultLLM == originalLabel {
			s.cfgCopy.DefaultLLM = label
		}
	}

	if s.cfgCopy.LLMProviders == nil {
		s.cfgCopy.LLMProviders = make(map[string]config.LLMProviderDef)
	}

	s.cfgCopy.LLMProviders[label] = config.LLMProviderDef{
		Provider:    provider,
		APIKey:      apiKey,
		Model:       model,
		APIEndpoint: endpoint,
	}

	if len(s.cfgCopy.LLMProviders) == 1 || s.cfgCopy.DefaultLLM == "" {
		s.cfgCopy.DefaultLLM = label
	}

	if err := s.clearLegacyAndSave(); err != nil {
		return fmt.Sprintf("error: %v", err)
	}

	guiLog("[GUI] Provider saved: label=%s provider=%s", label, provider)
	return "ok"
}

// DeleteProvider removes a provider by label.
func (s *SettingsService) DeleteProvider(label string) string {
	guiLog("[GUI] JS called: DeleteProvider(%s)", label)
	delete(s.cfgCopy.LLMProviders, label)
	if s.cfgCopy.DefaultLLM == label {
		s.cfgCopy.DefaultLLM = ""
		for k := range s.cfgCopy.LLMProviders {
			s.cfgCopy.DefaultLLM = k
			break
		}
	}

	if err := s.clearLegacyAndSave(); err != nil {
		return fmt.Sprintf("error: %v", err)
	}

	return "ok"
}

// SetDefault sets the default provider.
func (s *SettingsService) SetDefault(label string) string {
	guiLog("[GUI] JS called: SetDefault(%s)", label)
	if _, ok := s.cfgCopy.LLMProviders[label]; !ok {
		return "error: provider not found"
	}
	s.cfgCopy.DefaultLLM = label

	if err := s.clearLegacyAndSave(); err != nil {
		return fmt.Sprintf("error: %v", err)
	}

	return "ok"
}

// TestConnection tests provider credentials.
func (s *SettingsService) TestConnection(provider, apiKey, model, endpoint string) string {
	guiLog("[GUI] JS called: TestConnection(%s)", provider)

	// Ollama needs much longer timeout — first request loads model into memory.
	timeout := 10 * time.Second
	timeoutMs := 10000
	if provider == "ollama" {
		timeout = 120 * time.Second
		timeoutMs = 120000
	}

	def := config.LLMProviderDef{
		Provider:    provider,
		APIKey:      apiKey,
		Model:       model,
		APIEndpoint: endpoint,
		MaxTokens:   32,
		TimeoutMs:   timeoutMs,
	}

	client, err := llm.NewClientFromDef(def)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}

	tctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	_, err = client.Send(tctx, llm.Request{
		Prompt:    "Reply with OK",
		Text:      "test",
		MaxTokens: 32,
	})
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}

	return "ok"
}

// TestProvider tests a saved provider by label.
func (s *SettingsService) TestProvider(label string) string {
	guiLog("[GUI] JS called: TestProvider(%s)", label)
	def, ok := s.cfgCopy.LLMProviders[label]
	if !ok {
		return "error: provider not found"
	}

	// Ollama needs much longer timeout — first request loads model into memory.
	timeout := 10 * time.Second
	timeoutMs := 10000
	if def.Provider == "ollama" {
		timeout = 120 * time.Second
		timeoutMs = 120000
	}

	def.MaxTokens = 32
	def.TimeoutMs = timeoutMs

	client, err := llm.NewClientFromDef(def)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}

	tctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	_, err = client.Send(tctx, llm.Request{
		Prompt:    "Reply with OK",
		Text:      "test",
		MaxTokens: 32,
	})
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}

	return "ok"
}

// OpenConfigFile opens the config file in the OS default editor.
func (s *SettingsService) OpenConfigFile() string {
	guiLog("[GUI] JS called: OpenConfigFile")
	openFileInOS(s.configPath)
	return "ok"
}

// CloseWindow terminates the settings window.
func (s *SettingsService) CloseWindow() string {
	guiLog("[GUI] JS called: CloseWindow")
	if s.app != nil {
		s.app.Quit()
	}
	return "ok"
}

// OllamaStatus checks if Ollama is running and returns status + models in one call.
func (s *SettingsService) OllamaStatus(endpoint string) string {
	guiLog("[GUI] JS called: OllamaStatus(%s)", endpoint)
	st := ollamaGetStatus(endpoint)

	result := map[string]interface{}{
		"status": st["status"],
	}
	if v, ok := st["version"]; ok {
		result["version"] = v
	}

	// If running, include models in the same response.
	if st["status"] == "running" {
		base := ollamaBaseURL(endpoint)
		models, err := ollamaFetchModels(base)
		if err != nil {
			guiLog("[GUI] OllamaStatus model fetch error: %v", err)
			result["models"] = []ollamaModelInfo{}
		} else {
			result["models"] = models
		}
	}

	data, _ := json.Marshal(result)
	return string(data)
}

// OllamaListModels returns the list of locally-installed models.
func (s *SettingsService) OllamaListModels(endpoint string) string {
	guiLog("[GUI] JS called: OllamaListModels(%s)", endpoint)
	base := ollamaBaseURL(endpoint)
	models, err := ollamaFetchModels(base)
	if err != nil {
		guiLog("[GUI] OllamaListModels error: %v", err)
		return "[]"
	}
	data, _ := json.Marshal(models)
	return string(data)
}

// OllamaPullModel runs "ollama pull" synchronously (up to 10 minutes).
func (s *SettingsService) OllamaPullModel(model string) string {
	guiLog("[GUI] JS called: OllamaPullModel(%s)", model)
	if err := ollamaPullModelSync(model); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return "ok"
}

// OllamaDownloadInstaller downloads the Ollama installer (platform-specific).
func (s *SettingsService) OllamaDownloadInstaller() string {
	guiLog("[GUI] JS called: OllamaDownloadInstaller")
	if err := ollamaDownloadInstallerPlatform(); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return "ok"
}

// openFileInOS opens a file using the platform's default handler.
func openFileInOS(path string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	if err := cmd.Start(); err != nil {
		guiLog("[GUI] ERROR: Failed to open file: %v", err)
	}
}
