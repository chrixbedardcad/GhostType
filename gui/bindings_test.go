package gui

import (
	"reflect"
	"testing"

	"github.com/chrixbedardcad/GhostType/config"
)

func TestSettingsServiceHasAllMethods(t *testing.T) {
	svc := &SettingsService{
		cfgCopy:    &config.Config{LLMProviders: map[string]config.LLMProviderDef{}},
		configPath: "/tmp/test-config.json",
	}

	// All methods that must be exposed to the frontend.
	expected := []string{
		"GetVersion",
		"GetConfig",
		"GetKnownModels",
		"SaveProvider",
		"DeleteProvider",
		"SetDefault",
		"TestConnection",
		"TestProvider",
		"OpenConfigFile",
		"CloseWindow",
		"OllamaStatus",
		"OllamaListModels",
		"OllamaPullModel",
		"OllamaDownloadInstaller",
	}

	svcType := reflect.TypeOf(svc)
	for _, name := range expected {
		if _, ok := svcType.MethodByName(name); !ok {
			t.Errorf("SettingsService missing method %q", name)
		}
	}

	if got := svcType.NumMethod(); got < len(expected) {
		t.Errorf("expected at least %d public methods, got %d", len(expected), got)
	}
}
