// Command ghost is the GhostSpell CLI — process text, manage skills, and run the API server.
//
// Usage:
//
//	ghost correct "teh quik brown fox"       # correct text
//	ghost polish "rough draft"               # polish text
//	ghost translate "bonjour"                # translate text
//	echo "fix this" | ghost correct          # pipe support
//	ghost prompts                            # list available skills
//	ghost health                             # check API status
//	ghost serve                              # start API server
//	ghost serve -addr 127.0.0.1:9090         # custom port
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/chrixbedardcad/GhostSpell/config"
	"github.com/chrixbedardcad/GhostSpell/core"
	"github.com/chrixbedardcad/GhostSpell/internal/version"
	"github.com/chrixbedardcad/GhostSpell/llm"
	"github.com/chrixbedardcad/GhostSpell/mode"
	"github.com/chrixbedardcad/GhostSpell/stats"
	"github.com/chrixbedardcad/GhostSpell/stt"
)

var serverURL = "http://127.0.0.1:7878"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := strings.ToLower(os.Args[1])

	switch cmd {
	case "serve":
		cmdServe(os.Args[2:])
	case "health":
		cmdHealth()
	case "prompts", "skills":
		cmdPrompts()
	case "transcribe":
		cmdTranscribe(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Printf("ghost %s\n", version.Version)
	case "help", "--help", "-h":
		printUsage()
	default:
		// Treat as a skill name: ghost correct "text", ghost polish "text", etc.
		cmdSkill(cmd, os.Args[2:])
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "ghost %s — GhostSpell CLI\n\n", version.Version)
	fmt.Fprintln(os.Stderr, "Usage: ghost <command> [options] [text]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Skill commands (text processing):")
	fmt.Fprintln(os.Stderr, "  correct <text>         Fix spelling and grammar")
	fmt.Fprintln(os.Stderr, "  polish <text>          Improve clarity and flow")
	fmt.Fprintln(os.Stderr, "  translate <text>       Translate to target language")
	fmt.Fprintln(os.Stderr, "  ask <text>             Answer a question")
	fmt.Fprintln(os.Stderr, "  <skill-name> <text>    Run any skill by name")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  Text can be passed as arguments or piped via stdin.")
	fmt.Fprintln(os.Stderr, "  Result goes to stdout; metadata to stderr.")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Utility commands:")
	fmt.Fprintln(os.Stderr, "  prompts                List available skills")
	fmt.Fprintln(os.Stderr, "  health                 Check if API server is running")
	fmt.Fprintln(os.Stderr, "  transcribe <file.wav>  Transcribe a WAV file")
	fmt.Fprintln(os.Stderr, "  version                Show version")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Server command:")
	fmt.Fprintln(os.Stderr, "  serve [-addr HOST:PORT]  Start the API server (default: 127.0.0.1:7878)")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Options:")
	fmt.Fprintln(os.Stderr, "  -server URL            API server URL (default: http://127.0.0.1:7878)")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Examples:")
	fmt.Fprintln(os.Stderr, "  ghost correct \"teh quik brown fox\"")
	fmt.Fprintln(os.Stderr, "  echo \"rough draft\" | ghost polish")
	fmt.Fprintln(os.Stderr, "  ghost serve")
}

// ---------------------------------------------------------------------------
// Skill command: resolve skill name → index, then call /api/process
// ---------------------------------------------------------------------------

func cmdSkill(skillName string, args []string) {
	fs := flag.NewFlagSet(skillName, flag.ExitOnError)
	server := fs.String("server", serverURL, "API server URL")
	timeout := fs.Int("timeout", 0, "timeout in seconds (0 = default)")
	fs.Parse(args)

	text := readText(fs.Args())
	if text == "" {
		fmt.Fprintf(os.Stderr, "Error: no text provided. Pass text as argument or pipe to stdin.\n")
		os.Exit(1)
	}

	// Resolve skill name to index via /api/prompts.
	skillIdx, err := resolveSkill(*server, skillName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintf(os.Stderr, "Run 'ghost prompts' to see available skills.\n")
		os.Exit(1)
	}

	// Call /api/process.
	body := map[string]any{
		"skill_index": skillIdx,
		"text":        text,
	}
	url := *server + "/api/process"
	if *timeout > 0 {
		url += fmt.Sprintf("?timeout=%d", *timeout)
	}

	resp, err := postJSON(url, body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot reach API at %s: %v\n", *server, err)
		fmt.Fprintln(os.Stderr, "Is GhostSpell running with API enabled? Or run: ghost serve")
		os.Exit(1)
	}

	var result struct {
		Text     string  `json:"text"`
		Provider string  `json:"provider"`
		Model    string  `json:"model"`
		Duration float64 `json:"duration_seconds"`
		Error    string  `json:"error"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Error (%d): %s\n", resp.StatusCode, result.Error)
		os.Exit(1)
	}

	fmt.Print(result.Text)
	if !strings.HasSuffix(result.Text, "\n") {
		fmt.Println()
	}
	fmt.Fprintf(os.Stderr, "[%s/%s in %.1fs]\n", result.Provider, result.Model, result.Duration)
}

// resolveSkill queries /api/prompts and finds the skill index matching the given name.
func resolveSkill(server, name string) (int, error) {
	resp, err := http.Get(server + "/api/prompts")
	if err != nil {
		return 0, fmt.Errorf("cannot reach API at %s: %v\nIs GhostSpell running? Or run: ghost serve", server, err)
	}
	defer resp.Body.Close()

	var result struct {
		Prompts []struct {
			Index    int    `json:"index"`
			Name     string `json:"name"`
			Disabled bool   `json:"disabled"`
		} `json:"prompts"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	// Case-insensitive match.
	lower := strings.ToLower(name)
	for _, p := range result.Prompts {
		if strings.ToLower(p.Name) == lower && !p.Disabled {
			return p.Index, nil
		}
	}

	// Partial match (prefix).
	for _, p := range result.Prompts {
		if strings.HasPrefix(strings.ToLower(p.Name), lower) && !p.Disabled {
			return p.Index, nil
		}
	}

	return 0, fmt.Errorf("unknown skill %q", name)
}

// ---------------------------------------------------------------------------
// Utility commands
// ---------------------------------------------------------------------------

func cmdHealth() {
	resp, err := http.Get(serverURL + "/api/health")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot reach API at %s: %v\n", serverURL, err)
		fmt.Fprintln(os.Stderr, "Is GhostSpell running with API enabled? Or run: ghost serve")
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	fmt.Printf("Status: %v\n", result["status"])
	if v, ok := result["has_stt"]; ok {
		fmt.Printf("STT:    %v\n", v)
	}
}

func cmdPrompts() {
	resp, err := http.Get(serverURL + "/api/prompts")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot reach API at %s: %v\n", serverURL, err)
		fmt.Fprintln(os.Stderr, "Is GhostSpell running with API enabled? Or run: ghost serve")
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result struct {
		Prompts []struct {
			Index       int    `json:"index"`
			Name        string `json:"name"`
			Icon        string `json:"icon"`
			Voice       bool   `json:"voice"`
			Vision      bool   `json:"vision"`
			DisplayMode string `json:"display_mode"`
			Disabled    bool   `json:"disabled"`
		} `json:"prompts"`
		ActiveIndex int `json:"active_index"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	for _, p := range result.Prompts {
		marker := "  "
		if p.Index == result.ActiveIndex {
			marker = "* "
		}
		flags := ""
		if p.Voice {
			flags += " [voice]"
		}
		if p.Vision {
			flags += " [vision]"
		}
		if p.Disabled {
			flags += " [disabled]"
		}
		fmt.Printf("%s%d. %s %s%s\n", marker, p.Index, p.Icon, p.Name, flags)
	}
}

func cmdTranscribe(args []string) {
	fs := flag.NewFlagSet("transcribe", flag.ExitOnError)
	language := fs.String("lang", "", "language code (e.g. 'en', 'fr')")
	server := fs.String("server", serverURL, "API server URL")
	fs.Parse(args)

	if fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "Error: WAV file path required")
		fmt.Fprintln(os.Stderr, "Usage: ghost transcribe [-lang en] <file.wav>")
		os.Exit(1)
	}

	wavData, err := os.ReadFile(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", fs.Arg(0), err)
		os.Exit(1)
	}

	body := map[string]any{
		"wav_data": base64.StdEncoding.EncodeToString(wavData),
		"language": *language,
	}

	resp, err := postJSON(*server+"/api/transcribe", body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	var result struct {
		Text  string `json:"text"`
		Error string `json:"error"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Error (%d): %s\n", resp.StatusCode, result.Error)
		os.Exit(1)
	}

	fmt.Println(result.Text)
}

// ---------------------------------------------------------------------------
// Serve command: start the API server (replaces ghostspell-server)
// ---------------------------------------------------------------------------

func cmdServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", "127.0.0.1:7878", "listen address (host:port)")
	cfgFlag := fs.String("config", "", "path to config.json (default: OS app data dir)")
	fs.Parse(args)

	// Resolve config path.
	cfgPath := *cfgFlag
	if cfgPath == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot determine config dir: %v\n", err)
			os.Exit(1)
		}
		cfgPath = filepath.Join(base, "GhostSpell", "config.json")
	}

	// Set up logging: stdout + ghost-server.log in AppData.
	setupServerLogging()

	// Load config.
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to load config from %s: %v\n", cfgPath, err)
		os.Exit(1)
	}
	slog.Info("ghost serve starting", "version", version.Version, "config", cfgPath)

	// Init LLM.
	var router *mode.Router
	if cfg.DefaultModel != "" {
		client, err := newClientFromConfig(cfg, cfg.DefaultModel)
		if err != nil {
			slog.Warn("LLM init failed", "error", err)
			fmt.Fprintf(os.Stderr, "Warning: LLM init failed: %v\n", err)
		} else {
			router = mode.NewRouter(cfg, client)
			slog.Info("LLM ready", "model", cfg.DefaultModel)
		}
	} else {
		slog.Warn("No default_model configured — /api/process will fail")
	}

	// Init STT (optional).
	var transcriber stt.Transcriber
	if cfg.Voice.Model != "" {
		modelsDir, err := llm.LocalModelsDir()
		if err == nil {
			client, err := stt.NewGhostVoiceClient(cfg.Voice.Model, modelsDir, cfg.Voice.KeepAlive)
			if err == nil {
				transcriber = client
				slog.Info("STT ready", "model", cfg.Voice.Model)
			} else {
				slog.Warn("STT init failed", "error", err)
			}
		}
	}

	// Create engine + start API server.
	configDir := filepath.Dir(cfgPath)
	st := stats.New(configDir)
	engine := core.NewEngine(cfg, router, transcriber, st)
	apiSrv := core.NewAPIServer(engine)

	listenAddr, err := apiSrv.Start(*addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	slog.Info("API server listening", "addr", listenAddr)
	fmt.Printf("ghost serve — http://%s (Ctrl+C to stop)\n", listenAddr)

	// Wait for shutdown signal.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\nShutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	apiSrv.Shutdown(ctx)
	slog.Info("ghost serve stopped")
}

// setupServerLogging configures slog to write to both stdout and ghost-server.log.
func setupServerLogging() {
	base, err := os.UserConfigDir()
	if err != nil {
		return
	}
	dir := filepath.Join(base, "GhostSpell")
	os.MkdirAll(dir, 0755)
	logPath := filepath.Join(dir, "ghost-server.log")

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}

	// Write to both stdout and log file.
	multi := io.MultiWriter(os.Stdout, logFile)
	handler := slog.NewTextHandler(multi, &slog.HandlerOptions{Level: slog.LevelInfo})
	slog.SetDefault(slog.New(handler))
}

// newClientFromConfig builds an LLM client from config.
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
		Provider:     model.Provider,
		APIKey:       prov.APIKey,
		Model:        model.Model,
		APIEndpoint:  prov.APIEndpoint,
		RefreshToken: prov.RefreshToken,
		KeepAlive:    prov.KeepAlive,
		TimeoutMs:    prov.TimeoutMs,
		MaxTokens:    model.MaxTokens,
	}
	if model.TimeoutMs > 0 {
		def.TimeoutMs = model.TimeoutMs
	}
	return llm.NewClientFromDef(def)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// readText joins args into text, or reads from stdin if no args and stdin is piped.
func readText(args []string) string {
	text := strings.TrimSpace(strings.Join(args, " "))
	if text != "" {
		return text
	}
	// Try reading from stdin if piped.
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
			os.Exit(1)
		}
		return strings.TrimSpace(string(data))
	}
	return ""
}

func postJSON(url string, body any) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 120 * time.Second}
	return client.Post(url, "application/json", bytes.NewReader(data))
}
