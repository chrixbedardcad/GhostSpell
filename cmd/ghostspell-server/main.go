// Command ghostspell-server runs GhostSpell as a headless HTTP API server.
// No GUI, no tray, no hotkeys — pure core Engine exposed over HTTP.
//
// Usage:
//
//	ghostspell-server                       # default: 127.0.0.1:7878
//	ghostspell-server -addr 127.0.0.1:9090  # custom port
//	ghostspell-server -config /path/to/config.json
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/chrixbedardcad/GhostSpell/config"
	"github.com/chrixbedardcad/GhostSpell/core"
	"github.com/chrixbedardcad/GhostSpell/llm"
	"github.com/chrixbedardcad/GhostSpell/mode"
	"github.com/chrixbedardcad/GhostSpell/stt"
	"github.com/chrixbedardcad/GhostSpell/stats"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:7878", "listen address (host:port)")
	configPath := flag.String("config", "", "path to config.json (default: OS app data dir)")
	flag.Parse()

	// Resolve config path.
	cfgPath := *configPath
	if cfgPath == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot determine config dir: %v\n", err)
			os.Exit(1)
		}
		cfgPath = filepath.Join(base, "GhostSpell", "config.json")
	}

	// Load config.
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to load config from %s: %v\n", cfgPath, err)
		os.Exit(1)
	}
	fmt.Printf("GhostSpell Server — config: %s\n", cfgPath)

	// Init LLM client + router.
	var router *mode.Router
	if cfg.DefaultModel != "" {
		client, err := newClientFromConfig(cfg, cfg.DefaultModel)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: LLM init failed: %v\n", err)
			slog.Warn("LLM init failed", "error", err)
		} else {
			router = mode.NewRouter(cfg, client)
			fmt.Printf("LLM ready: %s\n", cfg.DefaultModel)
		}
	} else {
		fmt.Fprintln(os.Stderr, "Warning: no default_model configured — /api/process will fail")
	}

	// Init STT (optional).
	var transcriber stt.Transcriber
	if cfg.Voice.Model != "" {
		modelsDir, err := llm.LocalModelsDir()
		if err == nil {
			client, err := stt.NewGhostVoiceClient(cfg.Voice.Model, modelsDir, cfg.Voice.KeepAlive)
			if err == nil {
				transcriber = client
				fmt.Printf("STT ready: %s\n", cfg.Voice.Model)
			} else {
				fmt.Fprintf(os.Stderr, "Warning: STT init failed: %v\n", err)
			}
		}
	}

	// Init stats.
	configDir := filepath.Dir(cfgPath)
	st := stats.New(configDir)

	// Create engine + API server.
	engine := core.NewEngine(cfg, router, transcriber, st)
	apiSrv := core.NewAPIServer(engine)

	listenAddr, err := apiSrv.Start(*addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("API server listening on http://%s\n", listenAddr)
	fmt.Println("Press Ctrl+C to stop.")

	// Wait for shutdown signal.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\nShutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	apiSrv.Shutdown(ctx)
	fmt.Println("Bye.")
}

// newClientFromConfig builds an LLM client from config (same logic as main app).
func newClientFromConfig(cfg *config.Config, label string) (llm.Client, error) {
	model, ok := cfg.Models[label]
	if !ok {
		return nil, fmt.Errorf("model %q not found", label)
	}
	prov, ok := cfg.Providers[model.Provider]
	if !ok {
		return nil, fmt.Errorf("provider %q not configured", model.Provider)
	}
	def := config.LLMProviderDef{
		Provider:    model.Provider,
		APIKey:      prov.APIKey,
		Model:       model.Model,
		APIEndpoint: prov.APIEndpoint,
		RefreshToken: prov.RefreshToken,
		KeepAlive:   prov.KeepAlive,
		TimeoutMs:   prov.TimeoutMs,
		MaxTokens:   model.MaxTokens,
	}
	if model.TimeoutMs > 0 {
		def.TimeoutMs = model.TimeoutMs
	}
	return llm.NewClientFromDef(def)
}
