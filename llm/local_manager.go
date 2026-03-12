package llm

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// LocalModel describes a downloadable local model.
type LocalModel struct {
	Name     string `json:"name"`
	FileName string `json:"file_name"`
	URL      string `json:"url"`
	Size     int64  `json:"size"` // approximate bytes
	Tag      string `json:"tag,omitempty"`
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
		},
		{
			Name:     "qwen3-1.7b",
			FileName: "Qwen3-1.7B-Q4_K_M.gguf",
			URL:      "https://huggingface.co/unsloth/Qwen3-1.7B-GGUF/resolve/main/Qwen3-1.7B-Q4_K_M.gguf",
			Size:     1_110_000_000,
			Tag:      "best",
		},
	}
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

	slog.Info("[local] downloading model", "name", name, "url", model.URL, "dest", destPath)
	fmt.Printf("[local] Downloading %s...\n", model.FileName)

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

	slog.Info("[local] model downloaded", "name", name, "size", downloaded)
	fmt.Printf("[local] Downloaded %s (%d MB)\n", model.FileName, downloaded/1024/1024)
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
	slog.Info("[local] model deleted", "name", name, "path", path)
	return nil
}

// LlamaServerInstalled checks if the llama-server binary is present.
func LlamaServerInstalled() bool {
	_, err := LlamaServerPath()
	return err == nil
}

// DownloadLlamaServer downloads the llama-server binary for the current platform
// from the ggml-org/llama.cpp GitHub releases (CPU-only build).
func DownloadLlamaServer(progressCb func(DownloadProgress)) error {
	// Determine the right asset name based on OS/arch.
	assetPattern := ""
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "darwin/arm64":
		assetPattern = "macos-arm64"
	case "darwin/amd64":
		assetPattern = "macos-x64"
	case "windows/amd64":
		assetPattern = "win-x64"
	case "linux/amd64":
		assetPattern = "ubuntu-x64"
	default:
		return fmt.Errorf("unsupported platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	// Get latest release info from GitHub API.
	type ghAsset struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int64  `json:"size"`
	}
	type ghRelease struct {
		TagName string    `json:"tag_name"`
		Assets  []ghAsset `json:"assets"`
	}

	apiURL := "https://api.github.com/repos/ggml-org/llama.cpp/releases/latest"
	apiResp, err := http.Get(apiURL)
	if err != nil {
		return fmt.Errorf("fetch latest release: %w", err)
	}
	defer apiResp.Body.Close()

	if apiResp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub API returned %d", apiResp.StatusCode)
	}

	var release ghRelease
	if err := json.NewDecoder(apiResp.Body).Decode(&release); err != nil {
		return fmt.Errorf("parse release: %w", err)
	}

	// Find the matching asset (CPU-only, no CUDA/Vulkan).
	var downloadAsset *ghAsset
	for _, a := range release.Assets {
		if strings.Contains(a.Name, assetPattern) &&
			!strings.Contains(a.Name, "cuda") &&
			!strings.Contains(a.Name, "vulkan") &&
			strings.HasSuffix(a.Name, ".zip") {
			asset := a
			downloadAsset = &asset
			break
		}
	}
	if downloadAsset == nil {
		return fmt.Errorf("no matching llama-server asset for %s in release %s", assetPattern, release.TagName)
	}

	slog.Info("[local] downloading llama-server", "asset", downloadAsset.Name, "url", downloadAsset.BrowserDownloadURL)
	fmt.Printf("[local] Downloading %s...\n", downloadAsset.Name)

	// Download the zip file.
	dlResp, err := http.Get(downloadAsset.BrowserDownloadURL)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer dlResp.Body.Close()

	if dlResp.StatusCode != http.StatusOK {
		return fmt.Errorf("download: HTTP %d", dlResp.StatusCode)
	}

	dir, err := localBinDir()
	if err != nil {
		return err
	}

	tmpZip := filepath.Join(dir, downloadAsset.Name+".tmp")
	f, err := os.Create(tmpZip)
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}

	total := dlResp.ContentLength
	var downloaded int64
	buf := make([]byte, 256*1024)
	for {
		n, readErr := dlResp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := f.Write(buf[:n]); writeErr != nil {
				f.Close()
				os.Remove(tmpZip)
				return fmt.Errorf("write: %w", writeErr)
			}
			downloaded += int64(n)
			if progressCb != nil {
				pct := 0.0
				if total > 0 {
					pct = float64(downloaded) / float64(total) * 100
				}
				progressCb(DownloadProgress{
					FileName:   downloadAsset.Name,
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
			os.Remove(tmpZip)
			return fmt.Errorf("read: %w", readErr)
		}
	}
	f.Close()

	// Extract llama-server from the zip.
	if err := extractLlamaServerFromZip(tmpZip, dir); err != nil {
		os.Remove(tmpZip)
		return fmt.Errorf("extract: %w", err)
	}

	os.Remove(tmpZip)
	slog.Info("[local] llama-server installed", "dir", dir)
	fmt.Println("[local] llama-server installed successfully")
	return nil
}

// extractLlamaServerFromZip extracts the llama-server binary from a zip archive.
func extractLlamaServerFromZip(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	targetName := "llama-server"
	if runtime.GOOS == "windows" {
		targetName = "llama-server.exe"
	}

	for _, f := range r.File {
		// Look for llama-server (might be in a subdirectory within the zip).
		baseName := filepath.Base(f.Name)
		if baseName != targetName {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		destPath := filepath.Join(destDir, targetName)
		out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			rc.Close()
			return err
		}

		_, err = io.Copy(out, rc)
		rc.Close()
		out.Close()
		if err != nil {
			return err
		}

		slog.Info("[local] extracted llama-server", "path", destPath)
		return nil
	}

	return fmt.Errorf("llama-server binary not found in zip archive")
}
