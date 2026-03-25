//go:build ghostai

// Command ghostai is the Ghost-AI inference server — a standalone HTTP server
// wrapping llama.cpp for text completion via an OpenAI-compatible API.
//
// It is spawned and managed by the main ghost/ghostspell process.
// When run standalone: ghostai --model /path/to/model.gguf --port 8391
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/chrixbedardcad/GhostSpell/internal/procmgr"
	"github.com/chrixbedardcad/GhostSpell/internal/version"
	"github.com/chrixbedardcad/GhostSpell/llm/ghostai"
)

func main() {
	port := flag.Int("port", 8391, "listen port")
	modelPath := flag.String("model", "", "path to GGUF model file")
	threads := flag.Int("threads", 0, "inference threads (0 = auto)")
	contextSize := flag.Int("context-size", 2048, "context window size")
	parentPID := flag.Int("parent-pid", 0, "parent process PID (auto-exit if parent dies)")
	flag.Parse()

	// Set up logging: stderr + ghostai.log in AppData.
	setupLogging()
	slog.Info("ghostai starting", "version", version.Version, "port", *port, "model", *modelPath)

	// Start parent PID watchdog (if spawned by ghost.exe).
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if *parentPID > 0 {
		procmgr.WatchParent(*parentPID, func() {
			slog.Info("[ghostai] parent gone, shutting down")
			cancel()
		})
	}

	// Initialize engine.
	cfg := ghostai.Config{
		ContextSize: *contextSize,
		Threads:     *threads,
	}
	engine := ghostai.New(cfg)
	defer engine.Close()

	// Load model if specified.
	if *modelPath != "" {
		slog.Info("[ghostai] loading model", "path", *modelPath)
		if err := engine.Load(*modelPath); err != nil {
			slog.Error("[ghostai] model load failed", "error", err)
			fmt.Fprintf(os.Stderr, "Error: failed to load model: %v\n", err)
			os.Exit(1)
		}
		slog.Info("[ghostai] model loaded")
	}

	// Start HTTP server.
	srv := newServer(engine)
	mux := http.NewServeMux()
	mux.HandleFunc("/health", srv.handleHealth)
	mux.HandleFunc("/v1/chat/completions", srv.handleChatCompletions)
	mux.HandleFunc("/v1/models/load", srv.handleModelLoad)
	mux.HandleFunc("/v1/models/unload", srv.handleModelUnload)
	mux.HandleFunc("/v1/models", srv.handleModelInfo)
	mux.HandleFunc("/shutdown", srv.handleShutdown)

	// Find available port.
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		// Try next port.
		listener, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			slog.Error("[ghostai] listen failed", "error", err)
			os.Exit(1)
		}
	}
	actualPort := listener.Addr().(*net.TCPAddr).Port

	httpSrv := &http.Server{Handler: mux}
	go httpSrv.Serve(listener)

	slog.Info("[ghostai] server listening", "addr", listener.Addr())

	// Signal readiness to parent (must be on stdout, one line).
	fmt.Printf("READY port=%d\n", actualPort)

	// Wait for shutdown signal, parent death, or /shutdown endpoint.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigCh:
		slog.Info("[ghostai] received signal, shutting down")
	case <-ctx.Done():
		slog.Info("[ghostai] context cancelled, shutting down")
	case <-srv.shutdownCh:
		slog.Info("[ghostai] /shutdown called, shutting down")
	}

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutCancel()
	httpSrv.Shutdown(shutCtx)
	slog.Info("[ghostai] stopped")
}

// setupLogging configures slog to write to both stderr and ghostai.log.
func setupLogging() {
	base, err := os.UserConfigDir()
	if err != nil {
		return
	}
	dir := filepath.Join(base, "GhostSpell")
	os.MkdirAll(dir, 0755)
	logPath := filepath.Join(dir, "ghostai.log")

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}

	multi := io.MultiWriter(os.Stderr, logFile)
	handler := slog.NewTextHandler(multi, &slog.HandlerOptions{Level: slog.LevelInfo})
	slog.SetDefault(slog.New(handler))

	fmt.Fprintf(logFile, "\n=== ghostai %s started at %s ===\n", version.Version, time.Now().Format(time.RFC3339))
}
