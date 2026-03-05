package gui

import "github.com/chrixbedardcad/GhostType/config"

// NeedsSetup returns true if no usable LLM provider is configured.
func NeedsSetup(cfg *config.Config) bool {
	// If there are any providers configured at all, the user already went
	// through setup. Don't re-show the wizard even if an API key is empty
	// (e.g. Ollama, custom endpoints, or keys stored elsewhere).
	if len(cfg.LLMProviders) > 0 {
		return false
	}
	// Check legacy flat fields.
	if cfg.LLMProvider == "ollama" || cfg.APIKey != "" {
		return false
	}
	return true
}
