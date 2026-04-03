package gui

import "github.com/chrixbedardcad/GhostSpell/config"

// NeedsSetup delegates to config.NeedsSetup.
func NeedsSetup(cfg *config.Config) bool {
	return config.NeedsSetup(cfg)
}
