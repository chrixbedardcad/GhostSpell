package gui

import (
	"reflect"
	"testing"

	"github.com/chrixbedardcad/GhostSpell/config"
)

func TestSettingsServiceHasAllMethods(t *testing.T) {
	svc := &SettingsService{
		cfgCopy: &config.Config{
			Providers: map[string]config.ProviderConfig{},
			Models:    map[string]config.ModelEntry{},
		},
		configPath: "/tmp/test-config.json",
	}

	// All methods that must be exposed to the frontend.
	expected := []string{
		"GetVersion",
		"GetConfig",
		"GetKnownModels",
		"SaveProvider",
		"SaveProviderConfig",
		"SaveModel",
		"DeleteProvider",
		"DeleteModel",
		"RemoveProvider",
		"SetDefault",
		"SetDefaultModel",
		"TestConnection",
		"TestProvider",
		"TestProviderConnection",
		"OpenConfigFile",
		"CloseWindow",
		"OllamaStatus",
		"OllamaListModels",
		"OllamaPullModel",
		"OllamaDownloadInstaller",
		"CheckForUpdate",
		"UpdateNow",
		"CheckPermissions",
		"OpenPermissions",
		"OpenAccessibilityPane",
		"OpenInputMonitoringPane",
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
