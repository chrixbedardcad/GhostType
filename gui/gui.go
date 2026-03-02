package gui

import "github.com/chrixbedardcad/GhostType/config"

// NeedsSetup returns true if no usable LLM provider is configured.
func NeedsSetup(cfg *config.Config) bool {
	// Check llm_providers first.
	for _, def := range cfg.LLMProviders {
		if def.Provider == "ollama" || def.APIKey != "" {
			return false
		}
	}
	// Check legacy flat fields.
	if cfg.LLMProvider == "ollama" || cfg.APIKey != "" {
		return false
	}
	return true
}
