//go:build !windows

package gui

import "github.com/chrixbedardcad/GhostType/config"

// ShowSettingsBlocking is a no-op on non-Windows platforms.
// Returns the original config unchanged.
func ShowSettingsBlocking(cfg *config.Config, configPath string) *config.Config {
	return cfg
}

// ShowSettings is a no-op on non-Windows platforms.
func ShowSettings(cfg *config.Config, configPath string) {}
