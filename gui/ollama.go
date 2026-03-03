package gui

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// ollamaBaseURL normalises an endpoint string into a base URL.
// Defaults to http://localhost:11434 when empty.
func ollamaBaseURL(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "http://localhost:11434"
	}
	return strings.TrimRight(endpoint, "/")
}

// ollamaProbeRunning checks if the Ollama server is reachable at the given base URL.
// Returns (running, versionString).
func ollamaProbeRunning(base string) (bool, string) {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(base + "/")
	if err != nil {
		return false, ""
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
	return true, strings.TrimSpace(string(body))
}

// ollamaCheckInstalled returns true if the "ollama" binary is on PATH.
func ollamaCheckInstalled() bool {
	_, err := exec.LookPath("ollama")
	return err == nil
}

// ollamaGetStatus combines probe + install check.
// Returns a map with "status" => "running" | "installed" | "not_installed".
func ollamaGetStatus(endpoint string) map[string]string {
	base := ollamaBaseURL(endpoint)
	running, version := ollamaProbeRunning(base)
	if running {
		return map[string]string{"status": "running", "version": version}
	}
	if ollamaCheckInstalled() {
		return map[string]string{"status": "installed"}
	}
	return map[string]string{"status": "not_installed"}
}

// ollamaModelInfo holds metadata about a locally-installed model.
type ollamaModelInfo struct {
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	ParamSize string `json:"parameter_size"`
	Quant     string `json:"quantization_level"`
	SizeHuman string `json:"size_human"`
}

// ollamaFetchModels retrieves the list of locally-installed models.
func ollamaFetchModels(base string) ([]ollamaModelInfo, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(base + "/api/tags")
	if err != nil {
		return nil, fmt.Errorf("cannot reach Ollama: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama /api/tags returned %d", resp.StatusCode)
	}

	var payload struct {
		Models []struct {
			Name    string `json:"name"`
			Size    int64  `json:"size"`
			Details struct {
				ParameterSize     string `json:"parameter_size"`
				QuantizationLevel string `json:"quantization_level"`
			} `json:"details"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode error: %w", err)
	}

	out := make([]ollamaModelInfo, len(payload.Models))
	for i, m := range payload.Models {
		out[i] = ollamaModelInfo{
			Name:      m.Name,
			Size:      m.Size,
			ParamSize: m.Details.ParameterSize,
			Quant:     m.Details.QuantizationLevel,
			SizeHuman: formatBytes(m.Size),
		}
	}
	return out, nil
}

// ollamaPullModelSync runs "ollama pull <model>" synchronously with a 10-minute timeout.
func ollamaPullModelSync(model string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ollama", "pull", model)
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("pull timed out after 10 minutes")
	}
	if err != nil {
		return fmt.Errorf("ollama pull failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// formatBytes converts bytes to a human-readable string.
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
