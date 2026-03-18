package llm

import (
	"crypto/sha256"
	"encoding/hex"
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
	// localIdleTimeout controls how long Ghost-AI keeps the model in memory
	// after the last request. Small GGUF models use 400MB-1GB of RAM —
	// keeping them loaded avoids a 500ms-2s reload penalty on each use.
	localIdleTimeout = 30 * time.Minute

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
	SHA256   string `json:"sha256,omitempty"` // hex-encoded SHA-256 for integrity verification
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
		// --- Qwen3.5 (latest, highest intelligence density) ---
		{
			Name:     "qwen3.5-2b",
			FileName: "Qwen3.5-2B-Q4_K_M.gguf",
			URL:      "https://huggingface.co/unsloth/Qwen3.5-2B-GGUF/resolve/main/Qwen3.5-2B-Q4_K_M.gguf",
			Size:     1_280_000_000,
			Tag:      "recommended",
			Desc:     "Qwen3.5 — best quality-to-size. Praised for intelligence density. 100+ languages.",
		},
		{
			Name:     "qwen3.5-0.8b",
			FileName: "Qwen3.5-0.8B-Q4_K_M.gguf",
			URL:      "https://huggingface.co/unsloth/Qwen3.5-0.8B-GGUF/resolve/main/Qwen3.5-0.8B-Q4_K_M.gguf",
			Size:     533_000_000,
			Tag:      "fast",
			Desc:     "Qwen3.5 — tiny and fast. Great for quick grammar fixes on any machine.",
		},
		{
			Name:     "qwen3.5-4b",
			FileName: "Qwen3.5-4B-Q4_K_M.gguf",
			URL:      "https://huggingface.co/unsloth/Qwen3.5-4B-GGUF/resolve/main/Qwen3.5-4B-Q4_K_M.gguf",
			Size:     2_740_000_000,
			Tag:      "best",
			Desc:     "Qwen3.5 — high quality. Understands nuance and complex grammar. Needs 4GB+ RAM.",
		},
		{
			Name:     "qwen3.5-9b",
			FileName: "Qwen3.5-9B-Q4_K_M.gguf",
			URL:      "https://huggingface.co/unsloth/Qwen3.5-9B-GGUF/resolve/main/Qwen3.5-9B-Q4_K_M.gguf",
			Size:     5_680_000_000,
			Tag:      "heavy",
			Desc:     "Qwen3.5 — excellent quality. Beats GPT-oss-120B. Needs 8GB+ RAM, slow on CPU.",
		},
		// --- Abliterated (uncensored) Qwen3.5 ---
		{
			Name:     "qwen3.5-2b-uncensored",
			FileName: "Huihui-Qwen3.5-2B-abliterated.i1-Q4_K_M.gguf",
			URL:      "https://huggingface.co/mradermacher/Huihui-Qwen3.5-2B-abliterated-i1-GGUF/resolve/main/Huihui-Qwen3.5-2B-abliterated.i1-Q4_K_M.gguf",
			Size:     1_270_000_000,
			Tag:      "uncensored",
			Desc:     "Qwen3.5 2B abliterated — no refusals. Same quality as qwen3.5-2b without content filters.",
		},
		{
			Name:     "qwen3.5-4b-uncensored",
			FileName: "Huihui-Qwen3.5-4B-abliterated.i1-Q4_K_M.gguf",
			URL:      "https://huggingface.co/mradermacher/Huihui-Qwen3.5-4B-abliterated-i1-GGUF/resolve/main/Huihui-Qwen3.5-4B-abliterated.i1-Q4_K_M.gguf",
			Size:     2_710_000_000,
			Tag:      "uncensored",
			Desc:     "Qwen3.5 4B abliterated — no refusals. High quality without content filters. Needs 4GB+ RAM.",
		},
		// --- NVIDIA Nemotron (high efficiency, trained from scratch) ---
		{
			Name:     "nemotron-nano-4b",
			FileName: "NVIDIA-Nemotron-3-Nano-4B-Q4_K_M.gguf",
			URL:      "https://huggingface.co/unsloth/NVIDIA-Nemotron-3-Nano-4B-GGUF/resolve/main/NVIDIA-Nemotron-3-Nano-4B-Q4_K_M.gguf",
			Size:     2_900_000_000,
			Desc:     "NVIDIA Nemotron 3 Nano 4B. High efficiency, tool-calling, trained from scratch. Needs 4GB+ RAM.",
		},
		// --- Other models ---
		{
			Name:     "phi-4-mini",
			FileName: "Phi-4-mini-instruct-Q4_K_M.gguf",
			URL:      "https://huggingface.co/unsloth/Phi-4-mini-instruct-GGUF/resolve/main/Phi-4-mini-instruct-Q4_K_M.gguf",
			Size:     2_490_000_000,
			Desc:     "Microsoft's Phi-4 Mini (3.8B). Excellent reasoning and grammar. Needs 4GB+ RAM.",
		},
		{
			Name:     "qwen3-1.7b",
			FileName: "Qwen3-1.7B-Q4_K_M.gguf",
			URL:      "https://huggingface.co/unsloth/Qwen3-1.7B-GGUF/resolve/main/Qwen3-1.7B-Q4_K_M.gguf",
			Size:     1_110_000_000,
			Desc:     "Previous gen Qwen3. Solid quality, 100+ languages.",
		},
		{
			Name:     "gemma-3-1b",
			FileName: "gemma-3-1b-it-Q4_K_M.gguf",
			URL:      "https://huggingface.co/unsloth/gemma-3-1b-it-GGUF/resolve/main/gemma-3-1b-it-Q4_K_M.gguf",
			Size:     806_000_000,
			Desc:     "Google's lightweight model. Fast responses, 140+ languages.",
		},
		{
			Name:     "llama-3.2-3b",
			FileName: "Llama-3.2-3B-Instruct-Q4_K_M.gguf",
			URL:      "https://huggingface.co/bartowski/Llama-3.2-3B-Instruct-GGUF/resolve/main/Llama-3.2-3B-Instruct-Q4_K_M.gguf",
			Size:     2_020_000_000,
			Desc:     "Meta's instruction-follower. English, French, Spanish + 5 more.",
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

// legacyModelNames maps model names that were removed from the curated list
// to their GGUF filenames, so existing configs keep working after upgrades.
// Add an entry here whenever a model is removed from AvailableLocalModels().
var legacyModelNames = map[string]string{
	"qwen3-8b":  "Qwen3-8B-Q4_K_M.gguf",
	"qwen3-0.6b": "Qwen3-0.6B-Q4_K_M.gguf",
}

// ResolveLocalModelPath checks if a local model is downloaded and returns its path.
func ResolveLocalModelPath(name string) (string, error) {
	return resolveLocalModel(name)
}

// resolveLocalModel maps a friendly model name to the GGUF file path.
func resolveLocalModel(name string) (string, error) {
	modelsDir, err := LocalModelsDir()
	if err != nil {
		return "", err
	}

	// Map friendly names to filenames via curated list.
	for _, m := range AvailableLocalModels() {
		if m.Name == name {
			path := filepath.Join(modelsDir, m.FileName)
			if _, err := os.Stat(path); err != nil {
				return "", fmt.Errorf("model file not found: %s (download it first in Settings)", path)
			}
			return path, nil
		}
	}

	// Check legacy model names (removed from curated list but may still be in config).
	if fileName, ok := legacyModelNames[name]; ok {
		path := filepath.Join(modelsDir, fileName)
		if _, err := os.Stat(path); err == nil {
			slog.Info("[ghost-ai] resolved legacy model name", "name", name, "file", fileName)
			return path, nil
		}
		// Legacy file not on disk — fall through to raw filename check.
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
			// Check legacy model names for a friendly name + deprecated tag.
			for legacyName, legacyFile := range legacyModelNames {
				if legacyFile == e.Name() {
					m.Name = legacyName
					m.Tag = "deprecated"
					break
				}
			}
			if m.Name == "" {
				m.Name = strings.TrimSuffix(e.Name(), ".gguf")
			}
		}
		installed = append(installed, m)
	}
	return installed, nil
}

const downloadMaxRetries = 3

// DownloadModel downloads a model GGUF file from HuggingFace with
// resume support, retry logic, and SHA-256 integrity verification.
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

	var lastErr error
	for attempt := 1; attempt <= downloadMaxRetries; attempt++ {
		lastErr = downloadWithResume(model, tmpPath, attempt, progressCb)
		if lastErr == nil {
			break
		}
		slog.Warn("[ghost-ai] download attempt failed, retrying",
			"name", name, "attempt", attempt, "max", downloadMaxRetries, "error", lastErr)
		if attempt < downloadMaxRetries {
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}
	}
	if lastErr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("download %s failed after %d attempts: %w", name, downloadMaxRetries, lastErr)
	}

	// Verify SHA-256 checksum if available, or log computed hash for future use.
	computedHash, hashErr := computeChecksum(tmpPath)
	if hashErr != nil {
		slog.Warn("[ghost-ai] could not compute checksum", "name", name, "error", hashErr)
	} else if model.SHA256 != "" {
		if computedHash != model.SHA256 {
			os.Remove(tmpPath)
			return fmt.Errorf("checksum verification failed for %s: expected %s, got %s", name, model.SHA256, computedHash)
		}
		slog.Info("[ghost-ai] checksum verified", "name", name)
	} else {
		slog.Warn("[ghost-ai] no expected checksum for model — integrity not verified",
			"name", name, "sha256", computedHash)
	}

	// Rename temp to final.
	if err := os.Rename(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename: %w", err)
	}

	info, _ := os.Stat(destPath)
	size := int64(0)
	if info != nil {
		size = info.Size()
	}
	slog.Info("[ghost-ai] model downloaded", "name", name, "size", size)
	fmt.Printf("[ghost-ai] Downloaded %s (%d MB)\n", model.FileName, size/1024/1024)
	return nil
}

// downloadWithResume performs a single download attempt with HTTP Range resume.
func downloadWithResume(model *LocalModel, tmpPath string, attempt int, progressCb func(DownloadProgress)) error {
	// Check how much we already have from a previous partial download.
	var existingSize int64
	if info, err := os.Stat(tmpPath); err == nil {
		existingSize = info.Size()
	}

	// Build request with Range header for resume.
	req, err := http.NewRequest("GET", model.URL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if existingSize > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existingSize))
		slog.Info("[ghost-ai] resuming download", "from_byte", existingSize, "attempt", attempt)
	}

	client := &http.Client{Timeout: 0} // no timeout for large downloads
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// Handle response status.
	switch resp.StatusCode {
	case http.StatusOK:
		// Server doesn't support Range — start from scratch.
		existingSize = 0
	case http.StatusPartialContent:
		// Resume successful.
	default:
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	total := resp.ContentLength + existingSize
	if resp.StatusCode == http.StatusOK && resp.ContentLength > 0 {
		total = resp.ContentLength
	}

	// Open file for append (resume) or create (fresh start).
	var f *os.File
	if existingSize > 0 && resp.StatusCode == http.StatusPartialContent {
		f, err = os.OpenFile(tmpPath, os.O_WRONLY|os.O_APPEND, 0644)
	} else {
		existingSize = 0
		f, err = os.Create(tmpPath)
	}
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}

	downloaded := existingSize
	buf := make([]byte, 256*1024) // 256KB chunks
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := f.Write(buf[:n]); writeErr != nil {
				f.Close()
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
			return fmt.Errorf("read: %w", readErr)
		}
	}

	return f.Close()
}

// computeChecksum computes and returns the SHA-256 hex string of a file.
func computeChecksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("reading file for checksum: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// verifyChecksum computes SHA-256 of a file and compares to expected hex string.
func verifyChecksum(path, expectedHex string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("reading file for checksum: %w", err)
	}

	actual := hex.EncodeToString(h.Sum(nil))
	if actual != expectedHex {
		return fmt.Errorf("expected %s, got %s", expectedHex, actual)
	}
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
