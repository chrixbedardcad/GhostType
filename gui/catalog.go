package gui

import (
	_ "embed"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/chrixbedardcad/GhostSpell/config"
	"github.com/chrixbedardcad/GhostSpell/llm"
	"github.com/chrixbedardcad/GhostSpell/sound"
)

//go:embed models_catalog.json
var modelsCatalogJSON []byte

// CatalogModel is one entry from models.json.
type CatalogModel struct {
	Provider      string   `json:"provider"`
	Creator       string   `json:"creator"`
	Model         string   `json:"model"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	CostTier      string   `json:"cost_tier"`
	CostNote      string   `json:"cost_note,omitempty"` // e.g. "250 req/day", "$25 free credits"
	CostPer1K     float64  `json:"cost_per_1k_tokens"`
	IFEvalScore   float64  `json:"ifeval_score"`
	Tags          []string `json:"tags"`
	ContextWindow int      `json:"context_window"`
	Thinking      bool     `json:"thinking"`
}

// CatalogFile is the top-level models.json structure.
type CatalogFile struct {
	Version int            `json:"version"`
	Updated string         `json:"updated"`
	Models  []CatalogModel `json:"models"`
}

// CatalogEntry is a model with merged config + runtime state, sent to the JS frontend.
type CatalogEntry struct {
	CatalogModel
	Enabled        bool   `json:"enabled"`         // model entry exists in config
	IsDefault      bool   `json:"is_default"`      // is the default model
	ConfigLabel    string `json:"config_label"`     // label in config, if enabled
	AvgSpeedMs     int64  `json:"avg_speed_ms"`     // from stats, 0 = no data
	SpeedPrompt    string `json:"speed_prompt"`      // which prompt was used for the speed
	SpeedIcon      string `json:"speed_icon"`        // icon of the prompt used
	SpeedTimestamp string `json:"speed_ts"`          // when the benchmark was run
	ProviderActive bool   `json:"provider_active"`  // provider is configured
}

// parseCatalog parses the embedded models.json.
func parseCatalog() []CatalogModel {
	var cat CatalogFile
	if err := json.Unmarshal(modelsCatalogJSON, &cat); err != nil {
		slog.Error("[catalog] failed to parse models_catalog.json", "error", err)
		return nil
	}
	return cat.Models
}

// GetModelCatalog returns the model catalog merged with config state as JSON.
func (s *SettingsService) GetModelCatalog() string {
	guiLog("[GUI] JS called: GetModelCatalog")

	catalog := parseCatalog()
	cfg := s.cfgCopy

	// Build a lookup: provider+model → config label + stats.
	type cfgMatch struct {
		label string
	}
	configLookup := make(map[string]cfgMatch)
	for label, me := range cfg.Models {
		key := me.Provider + "/" + me.Model
		configLookup[key] = cfgMatch{label: label}
	}

	// Get stats for speed data.
	var statsModels []struct {
		Provider    string `json:"provider"`
		Model       string `json:"model"`
		AvgDuration int64  `json:"avg_duration_ms"`
	}
	if s.GetStatsFn != nil {
		raw := s.GetStatsFn()
		var summary struct {
			Models []struct {
				Provider    string `json:"provider"`
				Model       string `json:"model"`
				AvgDuration int64  `json:"avg_duration_ms"`
			} `json:"models"`
		}
		if json.Unmarshal([]byte(raw), &summary) == nil {
			statsModels = summary.Models
		}
	}
	type speedInfo struct {
		ms        int64
		prompt    string
		icon      string
		timestamp string
	}
	speedLookup := make(map[string]speedInfo)
	for _, sm := range statsModels {
		key := sm.Provider + "/" + sm.Model
		speedLookup[key] = speedInfo{ms: sm.AvgDuration}
	}

	// Override with latest benchmark results if available (has prompt info).
	benchMu.Lock()
	br := benchResult
	benchMu.Unlock()
	if br != nil && br.Done {
		for _, bm := range br.Models {
			if bm.Status == "success" && bm.DurationMs > 0 {
				key := bm.Provider + "/" + bm.Model
				speedLookup[key] = speedInfo{ms: bm.DurationMs, prompt: br.PromptName, icon: br.PromptIcon, timestamp: br.Timestamp}
			}
		}
	}

	// Build entries from catalog.
	var entries []CatalogEntry
	seen := make(map[string]bool)
	for _, cm := range catalog {
		key := cm.Provider + "/" + cm.Model
		seen[key] = true

		entry := CatalogEntry{
			CatalogModel:   cm,
			ProviderActive: false,
		}

		// Check if provider is configured and not disabled.
		if prov, ok := cfg.Providers[cm.Provider]; ok && !prov.Disabled {
			entry.ProviderActive = true
		}

		// Check if model is in config.
		if match, ok := configLookup[key]; ok {
			entry.Enabled = true
			entry.ConfigLabel = match.label
			entry.IsDefault = match.label == cfg.DefaultModel
		}

		// Speed from stats/benchmark.
		if si, ok := speedLookup[key]; ok {
			entry.AvgSpeedMs = si.ms
			entry.SpeedPrompt = si.prompt
			entry.SpeedIcon = si.icon
			entry.SpeedTimestamp = si.timestamp
		}

		// For local provider, only show models that are actually downloaded.
		if cm.Provider == "local" && entry.ProviderActive {
			if _, resolveErr := llm.ResolveLocalModelPath(cm.Model); resolveErr != nil {
				continue // model file not on disk — skip
			}
		}

		// Only show models whose provider is active.
		if entry.ProviderActive {
			entries = append(entries, entry)
		}
	}

	// Add custom models from config that aren't in the catalog.
	for label, me := range cfg.Models {
		key := me.Provider + "/" + me.Model
		if seen[key] {
			continue
		}
		// Provider must be active and not disabled.
		prov, ok := cfg.Providers[me.Provider]
		if !ok || prov.Disabled {
			continue
		}
		// Use model name as display name; fall back to label if model is "default".
		displayName := me.Model
		if displayName == "" || displayName == "default" {
			displayName = label
		}
		costTier := ""
		if me.Provider == "local" || me.Provider == "ollama" || me.Provider == "lmstudio" {
			costTier = "free"
		}
		entry := CatalogEntry{
			CatalogModel: CatalogModel{
				Provider:    me.Provider,
				Creator:     me.Provider,
				Model:       me.Model,
				Name:        displayName,
				Description: "",
				CostTier:    costTier,
				Tags:        []string{},
			},
			Enabled:        true,
			IsDefault:      label == cfg.DefaultModel,
			ConfigLabel:    label,
			ProviderActive: true,
		}
		if si, ok := speedLookup[key]; ok {
			entry.AvgSpeedMs = si.ms
			entry.SpeedPrompt = si.prompt
			entry.SpeedIcon = si.icon
			entry.SpeedTimestamp = si.timestamp
		}
		entries = append(entries, entry)
	}

	data, _ := json.Marshal(entries)
	return string(data)
}

// SetDefaultByModel finds or creates a model entry for the given provider+model
// and sets it as the default.
func (s *SettingsService) SetDefaultByModel(provider, model string) string {
	guiLog("[GUI] JS called: SetDefaultByModel(%s, %s)", provider, model)

	// Check provider is configured.
	if _, ok := s.cfgCopy.Providers[provider]; !ok {
		return "error: provider not configured"
	}

	// Look for existing model entry.
	for label, me := range s.cfgCopy.Models {
		if me.Provider == provider && me.Model == model {
			s.cfgCopy.DefaultModel = label
			if err := s.validateAndSave(); err != nil {
				return "error: " + err.Error()
			}
			go sound.PlayToggle()
			return "ok"
		}
	}

	// Not found — create a new entry. Use catalog name or generate one.
	name := model
	for _, cm := range parseCatalog() {
		if cm.Provider == provider && cm.Model == model {
			name = cm.Name
			break
		}
	}

	// Avoid label collision.
	label := name
	if _, exists := s.cfgCopy.Models[label]; exists {
		label = strings.Title(provider) + " " + name
	}

	if s.cfgCopy.Models == nil {
		s.cfgCopy.Models = make(map[string]config.ModelEntry)
	}
	s.cfgCopy.Models[label] = config.ModelEntry{
		Provider: provider,
		Model:    model,
	}
	s.cfgCopy.DefaultModel = label

	if err := s.validateAndSave(); err != nil {
		return "error: " + err.Error()
	}
	go sound.PlayToggle()
	return "ok"
}

// ToggleModel enables or disables a model for a provider.
// When enabled, creates a config.Models entry. When disabled, removes it.
func (s *SettingsService) ToggleModel(provider, model string, enabled bool) string {
	guiLog("[GUI] JS called: ToggleModel(%s, %s, %v)", provider, model, enabled)

	if s.cfgCopy.Models == nil {
		s.cfgCopy.Models = make(map[string]config.ModelEntry)
	}

	if enabled {
		// Check if already exists.
		for _, me := range s.cfgCopy.Models {
			if me.Provider == provider && me.Model == model {
				return "ok" // already enabled
			}
		}
		// Create entry. Use catalog name for label.
		name := model
		for _, cm := range parseCatalog() {
			if cm.Provider == provider && cm.Model == model {
				name = cm.Name
				break
			}
		}
		label := name
		for _, exists := s.cfgCopy.Models[label]; exists; _, exists = s.cfgCopy.Models[label] {
			label = label + " (" + provider + ")"
		}
		s.cfgCopy.Models[label] = config.ModelEntry{
			Provider: provider,
			Model:    model,
		}
		// If no default, set this one.
		if s.cfgCopy.DefaultModel == "" {
			s.cfgCopy.DefaultModel = label
		}
	} else {
		// Find and remove.
		for label, me := range s.cfgCopy.Models {
			if me.Provider == provider && me.Model == model {
				// Can't disable the default model.
				if label == s.cfgCopy.DefaultModel {
					return "error: cannot disable the default model — pick a different default first"
				}
				delete(s.cfgCopy.Models, label)
				break
			}
		}
	}

	if err := s.validateAndSave(); err != nil {
		return "error: " + err.Error()
	}
	return "ok"
}

// GetProviderModels returns catalog models for a specific provider with enabled state, as JSON.
// For LM Studio and Ollama, dynamically queries the server for loaded models.
func (s *SettingsService) GetProviderModels(provider string) string {
	guiLog("[GUI] JS called: GetProviderModels(%s)", provider)

	cfg := s.cfgCopy

	type ProviderModelEntry struct {
		Model       string   `json:"model"`
		Name        string   `json:"name"`
		Description string   `json:"description"`
		CostTier    string   `json:"cost_tier"`
		Tags        []string `json:"tags"`
		Enabled     bool     `json:"enabled"`
		IsDefault   bool     `json:"is_default"`
	}

	var entries []ProviderModelEntry

	// For LM Studio: query server for loaded models.
	if provider == "lmstudio" {
		endpoint := ""
		if prov, ok := cfg.Providers["lmstudio"]; ok {
			endpoint = prov.APIEndpoint
		}
		if _, modelNames, err := llm.LMStudioStatus(endpoint); err == nil {
			for _, name := range modelNames {
				enabled := false
				isDefault := false
				for label, me := range cfg.Models {
					if me.Provider == "lmstudio" && me.Model == name {
						enabled = true
						isDefault = label == cfg.DefaultModel
						break
					}
				}
				entries = append(entries, ProviderModelEntry{
					Model:       name,
					Name:        name,
					Description: "Loaded in LM Studio",
					CostTier:    "free",
					Tags:        []string{},
					Enabled:     enabled,
					IsDefault:   isDefault,
				})
			}
		}
		data, _ := json.Marshal(entries)
		return string(data)
	}

	// For all other providers: use the static catalog.
	catalog := parseCatalog()
	for _, cm := range catalog {
		if cm.Provider != provider {
			continue
		}
		enabled := false
		isDefault := false
		for label, me := range cfg.Models {
			if me.Provider == cm.Provider && me.Model == cm.Model {
				enabled = true
				isDefault = label == cfg.DefaultModel
				break
			}
		}
		entries = append(entries, ProviderModelEntry{
			Model:       cm.Model,
			Name:        cm.Name,
			Description: cm.Description,
			CostTier:    cm.CostTier,
			Tags:        cm.Tags,
			Enabled:     enabled,
			IsDefault:   isDefault,
		})
	}

	data, _ := json.Marshal(entries)
	return string(data)
}
