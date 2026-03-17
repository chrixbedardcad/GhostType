package gui

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/chrixbedardcad/GhostSpell/config"
	"github.com/chrixbedardcad/GhostSpell/llm"
	"github.com/chrixbedardcad/GhostSpell/sound"
)

// BenchmarkResult holds the result of benchmarking all models.
type BenchmarkResult struct {
	Running    bool                `json:"running"`
	Done       bool                `json:"done"`
	PromptName string              `json:"prompt_name"`
	PromptIcon string              `json:"prompt_icon"`
	Models     []BenchmarkModelRes `json:"models"`
	Error      string              `json:"error,omitempty"`
}

// BenchmarkModelRes holds the benchmark result for one model.
type BenchmarkModelRes struct {
	Label      string `json:"label"`
	Provider   string `json:"provider"`
	Model      string `json:"model"`
	DurationMs int64  `json:"duration_ms"`
	Output     string `json:"output"`
	Status     string `json:"status"` // "pending", "running", "success", "error", "timeout"
	Error      string `json:"error,omitempty"`
}

var (
	benchMu     sync.Mutex
	benchResult *BenchmarkResult
	benchCancel context.CancelFunc
)

const benchmarkTestText = "fix this: helo wrld, i'm typng fast and makng erors evrywhere"

// RunBenchmark starts a background benchmark of all visible catalog models.
// Tests every model whose provider is configured — not just models in config.
func (s *SettingsService) RunBenchmark() string {
	guiLog("[GUI] JS called: RunBenchmark")

	benchMu.Lock()
	if benchResult != nil && benchResult.Running {
		benchMu.Unlock()
		return "error: benchmark already running"
	}

	cfg := s.cfgCopy
	if len(cfg.Providers) == 0 {
		benchMu.Unlock()
		return "error: no providers configured"
	}

	// Get the active prompt.
	promptText := config.DefaultCorrectPrompt
	promptName := "Correct"
	promptIcon := "\u270F\uFE0F"
	if cfg.ActivePrompt >= 0 && cfg.ActivePrompt < len(cfg.Prompts) {
		promptText = cfg.Prompts[cfg.ActivePrompt].Prompt
		promptName = cfg.Prompts[cfg.ActivePrompt].Name
		promptIcon = cfg.Prompts[cfg.ActivePrompt].Icon
	}

	// Build model list from catalog + config (all models with active providers).
	type benchTarget struct {
		label    string
		provider string
		model    string
	}
	var targets []benchTarget
	seen := make(map[string]bool)

	// From catalog: all models whose provider is active.
	for _, cm := range parseCatalog() {
		if _, ok := cfg.Providers[cm.Provider]; !ok {
			continue
		}
		key := cm.Provider + "/" + cm.Model
		if seen[key] {
			continue
		}
		seen[key] = true
		targets = append(targets, benchTarget{label: cm.Name, provider: cm.Provider, model: cm.Model})
	}

	// From config: any custom models not in catalog.
	for label, me := range cfg.Models {
		key := me.Provider + "/" + me.Model
		if seen[key] {
			continue
		}
		if _, ok := cfg.Providers[me.Provider]; !ok {
			continue
		}
		seen[key] = true
		targets = append(targets, benchTarget{label: label, provider: me.Provider, model: me.Model})
	}

	if len(targets) == 0 {
		benchMu.Unlock()
		return "error: no models to benchmark"
	}

	// Sort: cloud providers first (fast, 1-5s), local providers last (slow, 10-60s).
	sort.SliceStable(targets, func(i, j int) bool {
		iLocal := targets[i].provider == "local" || targets[i].provider == "ollama" || targets[i].provider == "lmstudio"
		jLocal := targets[j].provider == "local" || targets[j].provider == "ollama" || targets[j].provider == "lmstudio"
		if iLocal != jLocal {
			return !iLocal // cloud first
		}
		return targets[i].label < targets[j].label // alphabetical within group
	})

	// Initialize results.
	result := &BenchmarkResult{Running: true, PromptName: promptName, PromptIcon: promptIcon}
	for _, t := range targets {
		result.Models = append(result.Models, BenchmarkModelRes{
			Label:    t.label,
			Provider: t.provider,
			Model:    t.model,
			Status:   "pending",
		})
	}
	benchResult = result
	benchMu.Unlock()

	go sound.PlayWorking()

	// Run benchmark in background.
	go func() {
		for i := range result.Models {
			benchMu.Lock()
			result.Models[i].Status = "running"
			benchMu.Unlock()

			provider := result.Models[i].Provider
			model := result.Models[i].Model

			prov, ok := cfg.Providers[provider]
			if !ok {
				benchMu.Lock()
				result.Models[i].Status = "error"
				result.Models[i].Error = "provider not configured"
				benchMu.Unlock()
				continue
			}

			def := config.LLMProviderDef{
				Provider:     provider,
				APIKey:       prov.APIKey,
				Model:        model,
				APIEndpoint:  prov.APIEndpoint,
				RefreshToken: prov.RefreshToken,
				KeepAlive:    prov.KeepAlive,
			}

			client, err := llm.NewClientFromDef(def)
			if err != nil {
				slog.Error("[benchmark] client creation failed", "provider", provider, "model", model, "error", err)
				benchMu.Lock()
				result.Models[i].Status = "error"
				result.Models[i].Error = err.Error()
				benchMu.Unlock()
				continue
			}

			// Use provider timeout or default 30s.
			timeout := 30 * time.Second
			if prov.TimeoutMs > 0 {
				timeout = time.Duration(prov.TimeoutMs) * time.Millisecond
			}
			if provider == "local" || provider == "ollama" || provider == "lmstudio" {
				timeout = 120 * time.Second
			}

			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			start := time.Now()
			resp, err := client.Send(ctx, llm.Request{
				Prompt: promptText,
				Text:   benchmarkTestText,
			})
			elapsed := time.Since(start)
			cancel()
			client.Close()

			benchMu.Lock()
			result.Models[i].DurationMs = elapsed.Milliseconds()
			var status, errMsg, output string
			if err != nil {
				if ctx.Err() == context.DeadlineExceeded {
					result.Models[i].Status = "timeout"
					result.Models[i].Error = fmt.Sprintf("timed out after %ds", int(timeout.Seconds()))
					status = "timeout"
					errMsg = result.Models[i].Error
				} else {
					result.Models[i].Status = "error"
					result.Models[i].Error = err.Error()
					status = "error"
					errMsg = err.Error()
				}
			} else {
				result.Models[i].Status = "success"
				result.Models[i].Output = resp.Text
				status = "success"
				output = resp.Text
			}
			benchMu.Unlock()

			// Record benchmark result into usage stats.
			if s.RecordStatFn != nil {
				s.RecordStatFn(
					promptName, promptIcon,
					provider, model, result.Models[i].Label,
					status, errMsg, output,
					len(benchmarkTestText), int(elapsed.Milliseconds()),
				)
			}
		}

		benchMu.Lock()
		result.Running = false
		result.Done = true
		benchMu.Unlock()
		go sound.PlaySuccess()
		slog.Info("[benchmark] complete")
	}()

	return "ok"
}

// RunBenchmarkFiltered benchmarks only the specified models in the given order.
// modelsJSON is a JSON array of {provider, model, name} objects.
func (s *SettingsService) RunBenchmarkFiltered(modelsJSON string) string {
	guiLog("[GUI] JS called: RunBenchmarkFiltered")

	benchMu.Lock()
	if benchResult != nil && benchResult.Running {
		benchMu.Unlock()
		return "error: benchmark already running"
	}

	cfg := s.cfgCopy

	type modelSpec struct {
		Provider string `json:"provider"`
		Model    string `json:"model"`
		Name     string `json:"name"`
	}
	var specs []modelSpec
	if err := json.Unmarshal([]byte(modelsJSON), &specs); err != nil {
		benchMu.Unlock()
		return "error: invalid model list"
	}
	if len(specs) == 0 {
		benchMu.Unlock()
		return "error: no models to benchmark"
	}

	promptText := config.DefaultCorrectPrompt
	promptName := "Correct"
	promptIcon := "\u270F\uFE0F"
	if cfg.ActivePrompt >= 0 && cfg.ActivePrompt < len(cfg.Prompts) {
		promptText = cfg.Prompts[cfg.ActivePrompt].Prompt
		promptName = cfg.Prompts[cfg.ActivePrompt].Name
		promptIcon = cfg.Prompts[cfg.ActivePrompt].Icon
	}

	result := &BenchmarkResult{Running: true, PromptName: promptName, PromptIcon: promptIcon}
	for _, sp := range specs {
		result.Models = append(result.Models, BenchmarkModelRes{
			Label:    sp.Name,
			Provider: sp.Provider,
			Model:    sp.Model,
			Status:   "pending",
		})
	}
	benchCtx, cancelFn := context.WithCancel(context.Background())
	benchCancel = cancelFn
	benchResult = result
	benchMu.Unlock()

	go sound.PlayWorking()

	go func() {
		defer func() {
			benchMu.Lock()
			benchCancel = nil
			benchMu.Unlock()
		}()

		for i := range result.Models {
			// Check if cancelled.
			if benchCtx.Err() != nil {
				benchMu.Lock()
				for j := i; j < len(result.Models); j++ {
					if result.Models[j].Status == "pending" {
						result.Models[j].Status = "error"
						result.Models[j].Error = "cancelled"
					}
				}
				benchMu.Unlock()
				break
			}

			benchMu.Lock()
			result.Models[i].Status = "running"
			benchMu.Unlock()

			provider := result.Models[i].Provider
			model := result.Models[i].Model

			prov, ok := cfg.Providers[provider]
			if !ok {
				benchMu.Lock()
				result.Models[i].Status = "error"
				result.Models[i].Error = "provider not configured"
				benchMu.Unlock()
				continue
			}

			def := config.LLMProviderDef{
				Provider:     provider,
				APIKey:       prov.APIKey,
				Model:        model,
				APIEndpoint:  prov.APIEndpoint,
				RefreshToken: prov.RefreshToken,
				KeepAlive:    prov.KeepAlive,
			}

			client, err := llm.NewClientFromDef(def)
			if err != nil {
				slog.Error("[benchmark] client creation failed", "provider", provider, "model", model, "error", err)
				benchMu.Lock()
				result.Models[i].Status = "error"
				result.Models[i].Error = err.Error()
				benchMu.Unlock()
				continue
			}

			timeout := 30 * time.Second
			if prov.TimeoutMs > 0 {
				timeout = time.Duration(prov.TimeoutMs) * time.Millisecond
			}
			if provider == "local" || provider == "ollama" || provider == "lmstudio" {
				timeout = 120 * time.Second
			}

			ctx, cancel := context.WithTimeout(benchCtx, timeout)
			start := time.Now()
			resp, err := client.Send(ctx, llm.Request{
				Prompt: promptText,
				Text:   benchmarkTestText,
			})
			elapsed := time.Since(start)
			cancel()
			client.Close()

			benchMu.Lock()
			result.Models[i].DurationMs = elapsed.Milliseconds()
			var status, errMsg, output string
			if err != nil {
				if ctx.Err() == context.DeadlineExceeded {
					result.Models[i].Status = "timeout"
					result.Models[i].Error = fmt.Sprintf("timed out after %ds", int(timeout.Seconds()))
					status = "timeout"
					errMsg = result.Models[i].Error
				} else {
					result.Models[i].Status = "error"
					result.Models[i].Error = err.Error()
					status = "error"
					errMsg = err.Error()
				}
			} else {
				result.Models[i].Status = "success"
				result.Models[i].Output = resp.Text
				status = "success"
				output = resp.Text
			}
			benchMu.Unlock()

			if s.RecordStatFn != nil {
				s.RecordStatFn(
					promptName, promptIcon,
					provider, model, result.Models[i].Label,
					status, errMsg, output,
					len(benchmarkTestText), int(elapsed.Milliseconds()),
				)
			}
		}

		benchMu.Lock()
		result.Running = false
		result.Done = true
		benchMu.Unlock()
		go sound.PlaySuccess()
		slog.Info("[benchmark] filtered complete")
	}()

	return "ok"
}

// StopBenchmark cancels a running benchmark.
func (s *SettingsService) StopBenchmark() string {
	guiLog("[GUI] JS called: StopBenchmark")
	benchMu.Lock()
	if benchCancel != nil {
		benchCancel()
	}
	benchMu.Unlock()
	return "ok"
}

// GetBenchmarkResult returns the current benchmark result as JSON.
func (s *SettingsService) GetBenchmarkResult() string {
	benchMu.Lock()
	defer benchMu.Unlock()
	if benchResult == nil {
		return "{}"
	}
	data, _ := json.Marshal(benchResult)
	return string(data)
}
