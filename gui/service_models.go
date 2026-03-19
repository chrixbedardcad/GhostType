package gui

import (
	"encoding/json"
	"fmt"
	"sync/atomic"

	"github.com/chrixbedardcad/GhostSpell/llm"
)

// downloadActive prevents concurrent model downloads.
var downloadActive atomic.Bool

// --- Local AI management ---------------------------------------------------

// GhostAIStatus returns JSON with Ghost-AI engine availability and status.
func (s *SettingsService) GhostAIStatus() string {
	guiLog("[GUI] JS called: GhostAIStatus")
	result := map[string]interface{}{
		"available": llm.GhostAIAvailable(),
	}
	data, _ := json.Marshal(result)
	return string(data)
}

// LocalStatus returns JSON with Ghost-AI engine status and installed models.
func (s *SettingsService) LocalStatus() string {
	guiLog("[GUI] JS called: LocalStatus")
	installed, err := llm.InstalledLocalModels()
	if err != nil {
		guiLog("[GUI] LocalStatus: error listing models: %v", err)
		installed = nil
	}

	result := map[string]interface{}{
		"engine_available": llm.GhostAIAvailable(),
		"engine_version":   llm.BundledLlamaCppVersion,
		"models":           installed,
		"available":        llm.AvailableLocalModels(),
	}
	data, _ := json.Marshal(result)
	return string(data)
}

// LocalDownloadModel downloads a local model by name (blocking).
func (s *SettingsService) LocalDownloadModel(name string) string {
	guiLog("[GUI] JS called: LocalDownloadModel(%s)", name)
	if !downloadActive.CompareAndSwap(false, true) {
		return "error: a download is already in progress"
	}
	defer downloadActive.Store(false)
	s.downloadProgress.Store(&llm.DownloadProgress{})
	if err := llm.DownloadModel(name, func(p llm.DownloadProgress) {
		s.downloadProgress.Store(&p)
	}); err != nil {
		s.downloadProgress.Store((*llm.DownloadProgress)(nil))
		guiLog("[GUI] LocalDownloadModel error: %v", err)
		return fmt.Sprintf("error: %v", err)
	}
	s.downloadProgress.Store((*llm.DownloadProgress)(nil))
	return "ok"
}

// LocalDownloadProgress returns the current download progress as JSON.
func (s *SettingsService) LocalDownloadProgress() string {
	v := s.downloadProgress.Load()
	if v == nil {
		return ""
	}
	p, ok := v.(*llm.DownloadProgress)
	if !ok || p == nil {
		return ""
	}
	data, _ := json.Marshal(p)
	return string(data)
}

// LocalDeleteModel deletes a downloaded local model.
// On Windows the Ghost-AI engine may hold the GGUF file open, so we
// reset (close) all cached LLM clients first to release the file lock.
func (s *SettingsService) LocalDeleteModel(name string) string {
	guiLog("[GUI] JS called: LocalDeleteModel(%s)", name)
	if s.ResetClientsFn != nil {
		guiLog("[GUI] LocalDeleteModel: resetting LLM clients to release file lock")
		s.ResetClientsFn()
	}
	if err := llm.DeleteModel(name); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return "ok"
}

// SetLocalKeepAlive toggles the keep-alive setting for a local provider.
func (s *SettingsService) SetLocalKeepAlive(providerType string, enabled bool) string {
	guiLog("[GUI] JS called: SetLocalKeepAlive(%s, %v)", providerType, enabled)
	prov, ok := s.cfgCopy.Providers[providerType]
	if !ok {
		return "error: provider not found"
	}
	prov.KeepAlive = enabled
	s.cfgCopy.Providers[providerType] = prov
	if err := s.validateAndSave(); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return "ok"
}

// LocalAvailableModels returns the list of downloadable models as JSON.
func (s *SettingsService) LocalAvailableModels() string {
	models := llm.AvailableLocalModels()
	data, _ := json.Marshal(models)
	return string(data)
}

// LocalInstalledModels returns the list of installed models as JSON.
func (s *SettingsService) LocalInstalledModels() string {
	models, err := llm.InstalledLocalModels()
	if err != nil {
		guiLog("[GUI] LocalInstalledModels error: %v", err)
		return "[]"
	}
	data, _ := json.Marshal(models)
	return string(data)
}

// --- Ollama management -----------------------------------------------------

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

// OllamaOpenPull opens a terminal window running "ollama pull <model>".
func (s *SettingsService) OllamaOpenPull(model string) string {
	guiLog("[GUI] JS called: OllamaOpenPull(%s)", model)
	if err := ollamaOpenTerminalPull(model); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return "ok"
}

// --- LM Studio management ----------------------------------------------------

// LMStudioStatus checks if an LM Studio server is reachable and returns available models.
func (s *SettingsService) LMStudioStatus(endpoint string) string {
	guiLog("[GUI] JS called: LMStudioStatus endpoint=%s", endpoint)
	running, models, err := llm.LMStudioStatus(endpoint)
	result := map[string]interface{}{
		"running": running,
		"models":  models,
	}
	if err != nil {
		result["error"] = err.Error()
	}
	data, _ := json.Marshal(result)
	return string(data)
}

// OllamaDownloadInstaller downloads the Ollama installer (platform-specific).
func (s *SettingsService) OllamaDownloadInstaller() string {
	guiLog("[GUI] JS called: OllamaDownloadInstaller")
	if err := ollamaDownloadInstallerPlatform(); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return "ok"
}
