package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chrixbedardcad/GhostType/config"
	"github.com/chrixbedardcad/GhostType/gui"
	"github.com/chrixbedardcad/GhostType/llm"
	"github.com/chrixbedardcad/GhostType/mode"
	"github.com/chrixbedardcad/GhostType/sound"
)

// appDataDir returns the OS-standard directory for GhostType's config, logs,
// and other persistent data.
//
//	macOS:   ~/Library/Application Support/GhostType/
//	Windows: %APPDATA%\GhostType\
//	Linux:   ~/.config/GhostType/  (or $XDG_CONFIG_HOME/GhostType/)
func appDataDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("os.UserConfigDir: %w", err)
	}
	return filepath.Join(base, "GhostType"), nil
}

// migrateConfigFromExeDir checks whether a config.json exists next to the
// executable (the old storage location). If it does and the target path does
// not yet exist, it copies the old config to the new app data directory and
// renames the original to config.json.bak so it isn't loaded again.
func migrateConfigFromExeDir(newConfigPath string) {
	// Already have a config in the new location — nothing to migrate.
	if _, err := os.Stat(newConfigPath); err == nil {
		return
	}

	execPath, err := os.Executable()
	if err != nil {
		return
	}
	oldPath := filepath.Join(filepath.Dir(execPath), "config.json")
	if _, err := os.Stat(oldPath); err != nil {
		return // no old config
	}

	data, err := os.ReadFile(oldPath)
	if err != nil {
		return
	}
	if err := os.WriteFile(newConfigPath, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to migrate config to %s: %v\n", newConfigPath, err)
		return
	}

	// Rename old file so it won't be picked up again.
	os.Rename(oldPath, oldPath+".bak")
	fmt.Printf("Migrated config from %s to %s\n", oldPath, newConfigPath)
}

// logStartupError writes a fatal startup error to a crash log file next to the
// config so that errors are visible even in windowless builds.
func logStartupError(dir, msg string, err error) {
	crashPath := filepath.Join(dir, "ghosttype_crash.log")
	f, ferr := os.OpenFile(crashPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if ferr != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "time=%s level=ERROR version=%s msg=%q error=%q\n",
		time.Now().Format(time.RFC3339), Version, msg, err)
	// Also log via slog in case the logger is already set up.
	slog.Error(msg, "error", err)
}

// setupLogging initialises the slog default logger from the loaded config.
// It creates the log file immediately so a version stamp is recorded as early
// as possible. Safe to call more than once (e.g. after settings change).
func setupLogging(cfg *config.Config, configDir string) {
	// Normalize to lowercase so "Debug", "DEBUG", etc. all work.
	cfg.LogLevel = strings.ToLower(strings.TrimSpace(cfg.LogLevel))

	if cfg.LogLevel != "" {
		logLevel := slog.LevelInfo
		switch cfg.LogLevel {
		case "debug":
			logLevel = slog.LevelDebug
		case "info":
			logLevel = slog.LevelInfo
		case "warn":
			logLevel = slog.LevelWarn
		case "error":
			logLevel = slog.LevelError
		}

		// Resolve log file path relative to the config file directory.
		logPath := cfg.LogFile
		if !filepath.IsAbs(logPath) {
			logPath = filepath.Join(configDir, logPath)
		}

		// Ensure the parent directory exists.
		if dir := filepath.Dir(logPath); dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not create log directory %s: %v\n", dir, err)
			}
		}

		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: could not open log file %s: %v\n", logPath, err)
			fmt.Fprintf(os.Stderr, "Logs will be written to stderr instead.\n")
			logFile = os.Stderr
		}

		logger := slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{Level: logLevel}))
		slog.SetDefault(logger)
		fmt.Printf("Logging enabled: level=%s file=%s\n", cfg.LogLevel, logPath)
	} else {
		// Disabled: set a no-op logger that discards everything.
		slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError + 1})))
		fmt.Println("Logging disabled (set log_level in config.json to enable)")
	}
}

func main() {
	fmt.Printf("GhostType v%s - AI-powered multilingual auto-correction\n", Version)
	fmt.Println("====================================================")

	// Determine the app data directory using the OS-standard location:
	//   macOS:   ~/Library/Application Support/GhostType/
	//   Windows: %APPDATA%\GhostType\
	//   Linux:   ~/.config/GhostType/  (XDG_CONFIG_HOME)
	appDir, err := appDataDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not determine app data directory: %v\n", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(appDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not create app directory %s: %v\n", appDir, err)
		os.Exit(1)
	}

	configPath := filepath.Join(appDir, "config.json")

	// Migration: if a config exists next to the executable (old behavior) but
	// not in the new app directory, move it over so existing users don't lose
	// their settings.
	migrateConfigFromExeDir(configPath)

	// Load configuration (without validation so the wizard can run first).
	fmt.Printf("App data: %s\n", appDir)
	cfg, err := config.LoadRaw(configPath)
	if err != nil {
		logStartupError(filepath.Dir(configPath), "Failed to load config", err)
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		fmt.Println("A default config.json has been created. Please add your API key and restart.")
		os.Exit(1)
	}

	// Derive the base directory from the config file for resolving relative paths.
	configDir := filepath.Dir(configPath)

	// Set up logging early so the log file is created and the version is
	// stamped before anything else (including the settings GUI).
	setupLogging(cfg, configDir)

	slog.Info("GhostType starting",
		"version", Version,
		"default_llm", cfg.DefaultLLM,
		"llm_providers", len(cfg.LLMProviders),
	)

	// First-launch check: if no provider is configured, the wizard will
	// run on the tray Wails app (no separate app to avoid goroutine leaks).
	needsSetup := gui.NeedsSetup(cfg)
	slog.Info("First-launch check", "needs_setup", needsSetup, "providers", len(cfg.LLMProviders), "default_llm", cfg.DefaultLLM)
	fmt.Printf("First-launch check: needs_setup=%v providers=%d default_llm=%q\n", needsSetup, len(cfg.LLMProviders), cfg.DefaultLLM)

	var router *mode.Router
	if !needsSetup {
		// Validate the config — if invalid, fall back to the wizard instead of crashing.
		if err := config.Validate(cfg); err != nil {
			slog.Warn("Config invalid, will show setup wizard", "error", err)
			fmt.Fprintf(os.Stderr, "Config invalid: %v — opening setup wizard\n", err)
			needsSetup = true
		}
	}
	if !needsSetup {
		// Initialize LLM client — if it fails, fall back to the wizard.
		var client llm.Client
		if cfg.DefaultLLM != "" {
			def := cfg.LLMProviders[cfg.DefaultLLM]
			client, err = llm.NewClientFromDef(def)
		} else {
			client, err = llm.NewClient(cfg)
		}
		if err != nil {
			slog.Warn("LLM init failed, will show setup wizard", "error", err)
			fmt.Fprintf(os.Stderr, "LLM init failed: %v — opening setup wizard\n", err)
			needsSetup = true
		} else {
			router = mode.NewRouter(cfg, client)
			sound.Init(*cfg.SoundEnabled)
			sound.PlayStart()
			printStatus(cfg, client, router)
		}
	}
	if needsSetup {
		fmt.Println("Setup needed — wizard will open...")
	}

	slog.Info("GhostType launching",
		"version", Version,
		"needs_setup", needsSetup,
	)

	runApp(cfg, router, configPath, needsSetup)
}

// printStatus prints provider, mode, and hotkey info to stdout.
func printStatus(cfg *config.Config, client llm.Client, router *mode.Router) {
	if len(cfg.LLMProviders) > 0 {
		fmt.Println("")
		fmt.Println("LLM Providers:")
		for label, def := range cfg.LLMProviders {
			suffix := ""
			if label == cfg.DefaultLLM {
				suffix = " (default)"
			}
			fmt.Printf("  %s: %s / %s%s\n", label, def.Provider, def.Model, suffix)
		}
		if cfg.CorrectLLM != "" {
			fmt.Printf("  correct  → %s\n", cfg.CorrectLLM)
		}
		if cfg.TranslateLLM != "" {
			fmt.Printf("  translate → %s\n", cfg.TranslateLLM)
		}
		for _, tmpl := range cfg.Prompts.RewriteTemplates {
			if tmpl.LLM != "" {
				fmt.Printf("  rewrite/%s → %s\n", tmpl.Name, tmpl.LLM)
			}
		}
	} else {
		fmt.Printf("Provider: %s\n", client.Provider())
		fmt.Printf("Model: %s\n", cfg.Model)
	}

	fmt.Println("")
	fmt.Printf("Active mode: %s\n", cfg.ActiveMode)
	targetLabels := cfg.TranslateTargetLabels()
	if idx := router.CurrentTranslateIdx(); idx < len(targetLabels) {
		fmt.Printf("Translate target: %s\n", targetLabels[idx])
	}
	fmt.Printf("Rewrite template: %s\n", router.CurrentTemplateName())
	fmt.Println("")
	fmt.Println("Hotkeys:")
	fmt.Printf("  %s - Action (%s)\n", cfg.Hotkeys.Correct, cfg.ActiveMode)
	if cfg.Hotkeys.Translate != "" {
		fmt.Printf("  %s - Translate\n", cfg.Hotkeys.Translate)
	}
	if cfg.Hotkeys.ToggleLanguage != "" {
		fmt.Printf("  %s - Toggle translation language\n", cfg.Hotkeys.ToggleLanguage)
	}
	if cfg.Hotkeys.Rewrite != "" {
		fmt.Printf("  %s - Rewrite\n", cfg.Hotkeys.Rewrite)
	}
	if cfg.Hotkeys.CycleTemplate != "" {
		fmt.Printf("  %s - Cycle rewrite template\n", cfg.Hotkeys.CycleTemplate)
	}
	fmt.Println("")
}
