package gui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/chrixbedardcad/GhostSpell/llm"
	"github.com/chrixbedardcad/GhostSpell/sound"
	"github.com/chrixbedardcad/GhostSpell/stt"
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

// --- Ghost Voice (speech-to-text) model management ---

// VoiceDownloadModel downloads a voice model by name (blocking).
func (s *SettingsService) VoiceDownloadModel(name string) string {
	guiLog("[GUI] JS called: VoiceDownloadModel(%s)", name)
	if !downloadActive.CompareAndSwap(false, true) {
		return "error: a download is already in progress"
	}
	defer downloadActive.Store(false)
	s.downloadProgress.Store(&llm.DownloadProgress{})
	if err := stt.DownloadVoiceModel(name, func(p llm.DownloadProgress) {
		s.downloadProgress.Store(&p)
	}); err != nil {
		s.downloadProgress.Store((*llm.DownloadProgress)(nil))
		return fmt.Sprintf("error: %v", err)
	}
	s.downloadProgress.Store((*llm.DownloadProgress)(nil))
	return "ok"
}

// VoiceDeleteModel deletes a downloaded voice model.
func (s *SettingsService) VoiceDeleteModel(name string) string {
	guiLog("[GUI] JS called: VoiceDeleteModel(%s)", name)
	if err := stt.DeleteVoiceModel(name); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return "ok"
}

// VoiceAvailableModels returns the list of available voice models as JSON.
func (s *SettingsService) VoiceAvailableModels() string {
	models := stt.AvailableVoiceModels()
	data, _ := json.Marshal(models)
	return string(data)
}

// VoiceStatus returns installed + available voice models as JSON.
func (s *SettingsService) VoiceStatus() string {
	modelsDir, err := stt.VoiceModelsDir()
	if err != nil {
		return fmt.Sprintf(`{"error":"%s"}`, err.Error())
	}
	type modelStatus struct {
		Name      string `json:"name"`
		FileName  string `json:"file_name"`
		Size      int64  `json:"size"`
		Tag       string `json:"tag"`
		Desc      string `json:"desc"`
		Installed bool   `json:"installed"`
	}
	var result []modelStatus
	for _, m := range stt.AvailableVoiceModels() {
		ms := modelStatus{
			Name:     m.Name,
			FileName: m.FileName,
			Size:     m.Size,
			Tag:      m.Tag,
			Desc:     m.Desc,
		}
		path := filepath.Join(modelsDir, m.FileName)
		if info, err := os.Stat(path); err == nil && info.Size() > 0 {
			ms.Installed = true
			ms.Size = info.Size()
		}
		result = append(result, ms)
	}
	data, _ := json.Marshal(result)
	return string(data)
}

// SetVoiceModel sets the active voice model in config and reloads the STT engine.
func (s *SettingsService) SetVoiceModel(model string) string {
	guiLog("[GUI] JS called: SetVoiceModel(%s)", model)
	s.cfgCopy.Voice.Model = model
	s.cfgCopy.Voice.Enabled = true
	if err := s.validateAndSave(); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	// Reload the STT engine with the new model.
	if s.ReloadSTTFn != nil {
		s.ReloadSTTFn()
	}
	return "ok"
}

// SetVoiceLanguage sets the voice language preference (speaking language).
func (s *SettingsService) SetVoiceLanguage(lang string) string {
	guiLog("[GUI] JS called: SetVoiceLanguage(%s)", lang)
	s.cfgCopy.Voice.Language = lang
	if err := s.validateAndSave(); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return "ok"
}

// TestVoice records 3 seconds from the microphone and runs ghostvoice transcription.
// Returns the transcribed text, empty string if no speech, or "error: ..." on failure.
func (s *SettingsService) TestVoice() string {
	guiLog("[GUI] JS called: TestVoice")

	// Record 3 seconds.
	recorder := sound.NewRecorder()
	if !recorder.MicAvailable() {
		return "error: no microphone found"
	}

	recCtx, recCancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer recCancel()

	stopCh := make(chan struct{})
	go func() {
		time.Sleep(3 * time.Second)
		close(stopCh)
	}()

	wavData, duration, err := recorder.Record(recCtx, stopCh)
	if err != nil {
		return fmt.Sprintf("error: recording failed — %v", err)
	}

	guiLog("[GUI] TestVoice: recorded %.1fs (%d bytes)", duration.Seconds(), len(wavData))

	// Transcribe using the configured STT provider.
	if s.TestVoiceFn == nil {
		return "error: no voice transcriber configured"
	}

	sttCtx, sttCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer sttCancel()

	text, err := s.TestVoiceFn(sttCtx, wavData)
	if err != nil {
		return fmt.Sprintf("error: transcription failed — %v", err)
	}

	return text
}

// TestVoiceSample runs the built-in "GhostVoice 1 2 3" WAV through the transcriber.
// No microphone needed — pure engine test.
func (s *SettingsService) TestVoiceSample() string {
	guiLog("[GUI] JS called: TestVoiceSample")

	if s.TestVoiceFn == nil {
		return "error: no voice transcriber configured"
	}

	wavData := sound.HumanVoiceTestWAV
	if len(wavData) == 0 {
		return "error: built-in test WAV not found"
	}

	guiLog("[GUI] TestVoiceSample: running %d bytes through ghostvoice", len(wavData))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	text, err := s.TestVoiceFn(ctx, wavData)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}

	return text
}

// SetVoiceNativeLanguage sets the speaker's native language for accent correction.
func (s *SettingsService) SetVoiceNativeLanguage(lang string) string {
	guiLog("[GUI] JS called: SetVoiceNativeLanguage(%s)", lang)
	s.cfgCopy.Voice.NativeLanguage = lang
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
