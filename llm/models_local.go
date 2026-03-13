package llm

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chrixbedardcad/GhostSpell/llm/ghostai"
)

const (
	localIdleTimeout = 5 * time.Minute

	// BundledLlamaCppVersion is the llama.cpp release tag compiled into GhostSpell.
	// Single source of truth for CI builds and version display.
	BundledLlamaCppVersion = "b8281"
)

// LLMProviderDefCompat mirrors config.LLMProviderDef fields needed by local
// clients, avoiding a circular import.
type LLMProviderDefCompat struct {
	Model     string
	MaxTokens int
	TimeoutMs int
	KeepAlive bool
}

// GhostAIAvailable reports whether the embedded Ghost-AI engine is compiled in.
// Recovers from panics in case CGo initialization fails (missing DLL, etc.).
func GhostAIAvailable() (available bool) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("[ghost-ai] availability check panicked", "panic", r)
			available = false
		}
	}()
	return ghostai.Available()
}

// LocalModel describes a downloadable local model.
type LocalModel struct {
	Name     string `json:"name"`
	FileName string `json:"file_name"`
	URL      string `json:"url"`
	Size     int64  `json:"size"`
	Tag      string `json:"tag,omitempty"`
	Desc     string `json:"desc,omitempty"`
}

// DownloadProgress reports download progress.
type DownloadProgress struct {
	FileName   string  `json:"file_name"`
	Downloaded int64   `json:"downloaded"`
	Total      int64   `json:"total"`
	Percent    float64 `json:"percent"`
}

// AvailableLocalModels returns the list of models that can be downloaded.
func AvailableLocalModels() []LocalModel {
	return []LocalModel{
		{
			Name:     "qwen3-0.6b",
			FileName: "Qwen3-0.6B-Q4_K_M.gguf",
			URL:      "https://huggingface.co/unsloth/Qwen3-0.6B-GGUF/resolve/main/Qwen3-0.6B-Q4_K_M.gguf",
			Size:     397_000_000,
			Tag:      "recommended",
			Desc:     "Tiny and fast. Great for quick spelling/grammar fixes. 100+ languages.",
		},
		{
			Name:     "qwen3-1.7b",
			FileName: "Qwen3-1.7B-Q4_K_M.gguf",
			URL:      "https://huggingface.co/unsloth/Qwen3-1.7B-GGUF/resolve/main/Qwen3-1.7B-Q4_K_M.gguf",
			Size:     1_110_000_000,
			Tag:      "best",
			Desc:     "Best quality-to-size ratio. Accurate corrections with context awareness. 100+ languages.",
		},
		{
			Name:     "qwen3-4b",
			FileName: "Qwen3-4B-Q4_K_M.gguf",
			URL:      "https://huggingface.co/unsloth/Qwen3-4B-GGUF/resolve/main/Qwen3-4B-Q4_K_M.gguf",
			Size:     2_600_000_000,
			Desc:     "High quality. Understands nuance and complex grammar. Needs 4GB+ RAM.",
		},
		{
			Name:     "gemma-3-1b",
			FileName: "gemma-3-1b-it-Q4_K_M.gguf",
			URL:      "https://huggingface.co/unsloth/gemma-3-1b-it-GGUF/resolve/main/gemma-3-1b-it-Q4_K_M.gguf",
			Size:     806_000_000,
			Tag:      "fast",
			Desc:     "Google's lightweight model. Fast responses, 140+ languages including French.",
		},
		{
			Name:     "llama-3.2-3b",
			FileName: "Llama-3.2-3B-Instruct-Q4_K_M.gguf",
			URL:      "https://huggingface.co/bartowski/Llama-3.2-3B-Instruct-GGUF/resolve/main/Llama-3.2-3B-Instruct-Q4_K_M.gguf",
			Size:     2_020_000_000,
			Desc:     "Meta's strong instruction-follower. Officially supports English, French, Spanish + 5 more.",
		},
	}
}

// LocalModelsDir returns the path to the local models directory.
func LocalModelsDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "GhostSpell", "models")
	os.MkdirAll(dir, 0755)
	return dir, nil
}

// resolveLocalModel maps a friendly model name to the GGUF file path.
func resolveLocalModel(name string) (string, error) {
	modelsDir, err := LocalModelsDir()
	if err != nil {
		return "", err
	}

	// Map friendly names to filenames.
	for _, m := range AvailableLocalModels() {
		if m.Name == name {
			path := filepath.Join(modelsDir, m.FileName)
			if _, err := os.Stat(path); err != nil {
				return "", fmt.Errorf("model file not found: %s (download it first in Settings)", path)
			}
			return path, nil
		}
	}

	// Assume it's already a filename.
	path := filepath.Join(modelsDir, name)
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("model file not found: %s", path)
	}
	return path, nil
}

// InstalledLocalModels returns a list of GGUF files in the models directory.
func InstalledLocalModels() ([]LocalModel, error) {
	dir, err := LocalModelsDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	available := AvailableLocalModels()
	var installed []LocalModel
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".gguf") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		m := LocalModel{
			FileName: e.Name(),
			Size:     info.Size(),
		}
		// Match to known models.
		for _, a := range available {
			if a.FileName == e.Name() {
				m.Name = a.Name
				m.Tag = a.Tag
				break
			}
		}
		if m.Name == "" {
			m.Name = strings.TrimSuffix(e.Name(), ".gguf")
		}
		installed = append(installed, m)
	}
	return installed, nil
}

// DownloadModel downloads a model GGUF file from HuggingFace.
func DownloadModel(name string, progressCb func(DownloadProgress)) error {
	var model *LocalModel
	for _, m := range AvailableLocalModels() {
		if m.Name == name {
			model = &m
			break
		}
	}
	if model == nil {
		return fmt.Errorf("unknown model: %s", name)
	}

	dir, err := LocalModelsDir()
	if err != nil {
		return err
	}

	destPath := filepath.Join(dir, model.FileName)
	tmpPath := destPath + ".tmp"

	slog.Info("[ghost-ai] downloading model", "name", name, "url", model.URL, "dest", destPath)
	fmt.Printf("[ghost-ai] Downloading %s...\n", model.FileName)

	resp, err := http.Get(model.URL)
	if err != nil {
		return fmt.Errorf("download %s: %w", name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d", name, resp.StatusCode)
	}

	total := resp.ContentLength

	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	var downloaded int64
	buf := make([]byte, 256*1024) // 256KB chunks
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := f.Write(buf[:n]); writeErr != nil {
				f.Close()
				os.Remove(tmpPath)
				return fmt.Errorf("write: %w", writeErr)
			}
			downloaded += int64(n)
			if progressCb != nil {
				pct := 0.0
				if total > 0 {
					pct = float64(downloaded) / float64(total) * 100
				}
				progressCb(DownloadProgress{
					FileName:   model.FileName,
					Downloaded: downloaded,
					Total:      total,
					Percent:    pct,
				})
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			f.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("read: %w", readErr)
		}
	}

	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close: %w", err)
	}

	// Rename temp to final.
	if err := os.Rename(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename: %w", err)
	}

	slog.Info("[ghost-ai] model downloaded", "name", name, "size", downloaded)
	fmt.Printf("[ghost-ai] Downloaded %s (%d MB)\n", model.FileName, downloaded/1024/1024)
	return nil
}

// DeleteModel removes a downloaded model file.
func DeleteModel(name string) error {
	dir, err := LocalModelsDir()
	if err != nil {
		return err
	}

	// Map name to filename.
	fileName := name
	for _, m := range AvailableLocalModels() {
		if m.Name == name {
			fileName = m.FileName
			break
		}
	}

	path := filepath.Join(dir, fileName)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	slog.Info("[ghost-ai] model deleted", "name", name, "path", path)
	return nil
}
