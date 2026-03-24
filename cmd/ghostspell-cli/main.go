// Command ghostspell-cli is a CLI client for the GhostSpell API server.
//
// Usage:
//
//	ghostspell-cli process "teh quik brown fox"          # use default skill (Correct)
//	ghostspell-cli process -skill 1 "make this polished" # use skill index 1 (Polish)
//	ghostspell-cli transcribe recording.wav              # transcribe a WAV file
//	ghostspell-cli prompts                               # list available prompts
//	ghostspell-cli health                                # check server status
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

var serverURL string

func main() {
	flag.StringVar(&serverURL, "server", "http://127.0.0.1:7878", "GhostSpell API server URL")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	switch args[0] {
	case "process":
		cmdProcess(args[1:])
	case "transcribe":
		cmdTranscribe(args[1:])
	case "prompts":
		cmdPrompts()
	case "health":
		cmdHealth()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", args[0])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: ghostspell-cli [-server URL] <command> [args]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  process [-skill N] <text>   Process text through a skill")
	fmt.Fprintln(os.Stderr, "  transcribe <file.wav>       Transcribe a WAV file")
	fmt.Fprintln(os.Stderr, "  prompts                     List available prompts/skills")
	fmt.Fprintln(os.Stderr, "  health                      Check server status")
}

func cmdProcess(args []string) {
	fs := flag.NewFlagSet("process", flag.ExitOnError)
	skill := fs.Int("skill", 0, "skill/prompt index (0 = Correct)")
	timeout := fs.Int("timeout", 0, "timeout in seconds (0 = server default)")
	fs.Parse(args)

	text := strings.Join(fs.Args(), " ")
	if text == "" {
		// Try reading from stdin if no text argument.
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
				os.Exit(1)
			}
			text = string(data)
		}
	}
	if text == "" {
		fmt.Fprintln(os.Stderr, "Error: no text provided. Pass text as argument or pipe to stdin.")
		os.Exit(1)
	}

	body := map[string]any{
		"skill_index": *skill,
		"text":        text,
	}
	url := serverURL + "/api/process"
	if *timeout > 0 {
		url += fmt.Sprintf("?timeout=%d", *timeout)
	}

	resp, err := postJSON(url, body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
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

func cmdTranscribe(args []string) {
	fs := flag.NewFlagSet("transcribe", flag.ExitOnError)
	language := fs.String("lang", "", "language code (e.g. 'en', 'fr') or empty for auto-detect")
	fs.Parse(args)

	if fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "Error: WAV file path required")
		os.Exit(1)
	}

	wavPath := fs.Arg(0)
	wavData, err := os.ReadFile(wavPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", wavPath, err)
		os.Exit(1)
	}

	body := map[string]any{
		"wav_data": base64.StdEncoding.EncodeToString(wavData),
		"language": *language,
	}

	resp, err := postJSON(serverURL+"/api/transcribe", body)
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

func cmdPrompts() {
	resp, err := http.Get(serverURL + "/api/prompts")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
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
		ActiveIndex int    `json:"active_index"`
		Error       string `json:"error"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Error (%d): %s\n", resp.StatusCode, result.Error)
		os.Exit(1)
	}

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

func cmdHealth() {
	resp, err := http.Get(serverURL + "/api/health")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot reach server at %s: %v\n", serverURL, err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	fmt.Printf("Status: %v\n", result["status"])
	fmt.Printf("STT:    %v\n", result["has_stt"])
}

func postJSON(url string, body any) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 120 * time.Second}
	return client.Post(url, "application/json", bytes.NewReader(data))
}
