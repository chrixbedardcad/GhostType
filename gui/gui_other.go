//go:build !windows

package gui

import "github.com/chrixbedardcad/GhostType/config"

// ShowWizard is a no-op on non-Windows platforms.
// Returns the original config unchanged.
func ShowWizard(cfg *config.Config, configPath string) *config.Config {
	return cfg
}

// ShowSettings is a no-op on non-Windows platforms.
func ShowSettings(cfg *config.Config, configPath string) {}

// ShowWizardAsync is a no-op on non-Windows platforms.
func ShowWizardAsync(cfg *config.Config, configPath string) {}
