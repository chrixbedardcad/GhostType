package gui

import (
	_ "embed"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/chrixbedardcad/GhostSpell/config"
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
	speedLookup := make(map[string]int64)
	for _, sm := range statsModels {
		key := sm.Provider + "/" + sm.Model
		speedLookup[key] = sm.AvgDuration
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

		// Check if provider is configured.
		if _, ok := cfg.Providers[cm.Provider]; ok {
			entry.ProviderActive = true
		}

		// Check if model is in config.
		if match, ok := configLookup[key]; ok {
			entry.Enabled = true
			entry.ConfigLabel = match.label
			entry.IsDefault = match.label == cfg.DefaultModel
		}

		// Speed from stats.
		if spd, ok := speedLookup[key]; ok {
			entry.AvgSpeedMs = spd
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
		// Provider must be active.
		if _, ok := cfg.Providers[me.Provider]; !ok {
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
		if spd, ok := speedLookup[key]; ok {
			entry.AvgSpeedMs = spd
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
	return "ok"
}
