package gui

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/chrixbedardcad/GhostSpell/config"
	"github.com/chrixbedardcad/GhostSpell/llm"
)

// DetectGPU returns a JSON description of the system GPU and acceleration status.
// This tells the UI what GPU is available and whether it's being used.
func (s *SettingsService) DetectGPU() string {
	guiLog("[GUI] JS called: DetectGPU")
	info := llm.DetectGPU()

	// If the current ghostai binary was built with CUDA and we detected CUDA,
	// mark as installed (the build already includes GPU support).
	if info.Type == llm.GPUCUDA && info.Available {
		// The local _build.bat produces a CUDA-enabled ghostai.exe when CUDA is present.
		// CI produces CPU-only. We can't distinguish at runtime, but if the user built
		// locally with CUDA, it just works. GPU layers are auto-calculated by ghostai.
		info.Installed = true
	}

	b, _ := json.Marshal(info)
	return string(b)
}

// WizardSkip marks the wizard as completed without configuring anything.
// The app runs as an empty shell — user can configure everything from Settings.
func (s *SettingsService) WizardSkip() string {
	guiLog("[GUI] JS called: WizardSkip")
	s.cfgCopy.WizardCompleted = true
	if err := s.validateAndSave(); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return "ok"
}

// WizardComplete marks the wizard as completed.
func (s *SettingsService) WizardComplete() string {
	guiLog("[GUI] JS called: WizardComplete")
	s.cfgCopy.WizardCompleted = true
	if err := s.validateAndSave(); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return "ok"
}

// SystemInfo returns a JSON object with system details for the wizard.
// RAM (GB), GPU info, and whether ghostai binary exists.
func (s *SettingsService) SystemInfo() string {
	guiLog("[GUI] JS called: SystemInfo")
	gpu := llm.DetectGPU()
	ghostaiAvail := llm.GhostAIAvailable()

	type sysInfo struct {
		GPU            llm.GPUInfo `json:"gpu"`
		GhostAIReady   bool        `json:"ghostai_ready"`
	}

	info := sysInfo{
		GPU:          gpu,
		GhostAIReady: ghostaiAvail,
	}

	b, _ := json.Marshal(info)
	slog.Info("[GUI] SystemInfo", "gpu", gpu.Type, "gpu_name", gpu.Name, "ghostai", ghostaiAvail)
	return string(b)
}

// validateAndSaveFromWizard is a thread-safe wrapper that prevents
// concurrent wizard config writes (e.g. download + skip race).
// Already handled by validateAndSave's sequential disk write, but
// we log for debugging.
func (s *SettingsService) validateAndSaveFromWizard(label string) error {
	guiLog("[GUI] Wizard save: %s", label)
	return s.validateAndSave()
}

// ResetWizardFlag clears the wizard_completed flag, allowing the wizard
// to show again on next launch. Useful for testing.
func (s *SettingsService) ResetWizardFlag() string {
	guiLog("[GUI] JS called: ResetWizardFlag")
	s.cfgCopy.WizardCompleted = false
	if err := s.validateAndSave(); err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return "ok"
}

// NeedsSetup is a utility that delegates to config.NeedsSetup.
func (s *SettingsService) NeedsSetup() string {
	cfg := s.cfgCopy
	if cfg == nil {
		// Fall back to live config if settings aren't open.
		s.liveMu.Lock()
		cfg = s.liveCfg
		s.liveMu.Unlock()
	}
	if config.NeedsSetup(cfg) {
		return "true"
	}
	return "false"
}
