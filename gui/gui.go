package gui

import "github.com/chrixbedardcad/GhostSpell/config"

// NeedsSetup returns true if no usable provider is configured.
func NeedsSetup(cfg *config.Config) bool {
	// If there are any providers configured at all, the user already went
	// through setup. Don't re-show the wizard even if an API key is empty
	// (e.g. Ollama, custom endpoints, or keys stored elsewhere).
	if len(cfg.Providers) > 0 {
		return false
	}
	return true
}
