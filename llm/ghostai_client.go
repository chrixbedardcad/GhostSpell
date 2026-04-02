package llm

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// GhostAIClient implements the Client interface by communicating with a
// ghostai daemon process via stdin/stdout JSON (same pattern as ghostvoice).
type GhostAIClient struct {
	mu        sync.Mutex
	proc      *exec.Cmd
	stdin     io.WriteCloser
	stdout    *bufio.Scanner
	modelPath string
	modelName string
	maxTokens int
	keepAlive bool
	running   bool
	idleTimer *time.Timer
	logFile   *os.File
}

// newGhostAIFromDef creates a GhostAIClient that spawns and manages ghostai.
func newGhostAIFromDef(def LLMProviderDefCompat) (*GhostAIClient, error) {
	maxTokens := def.MaxTokens
	if maxTokens == 0 {
		maxTokens = 256
	}
	// Thinking models need a much larger budget — their <think> blocks
	// consume most tokens before the answer.
	if isThinkingModel(def.Model) && maxTokens < 2048 {
		maxTokens = 2048
	}

	modelPath, err := resolveLocalModel(def.Model)
	if err != nil {
		return nil, fmt.Errorf("local model %q: %w", def.Model, err)
	}

	c := &GhostAIClient{
		modelPath: modelPath,
		modelName: def.Model,
		maxTokens: maxTokens,
		keepAlive: def.KeepAlive,
	}

	if err := c.startDaemon(def.GPUEnabled); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *GhostAIClient) Provider() string { return "local" }

func (c *GhostAIClient) Send(ctx context.Context, req Request) (*Response, error) {
	// Vision not supported with local models.
	if len(req.Images) > 0 {
		return nil, fmt.Errorf("vision is not supported with Ghost-AI local models — use a cloud provider or Ollama with a vision model")
	}

	c.mu.Lock()

	if !c.running {
		if err := c.startDaemon(GPUEnabled); err != nil {
			c.mu.Unlock()
			return nil, err
		}
	}

	// Reset idle timer.
	if c.idleTimer != nil {
		c.idleTimer.Reset(localIdleTimeout)
	}

	// Build request JSON.
	messages := []map[string]string{
		{"role": "system", "content": req.Prompt},
		{"role": "user", "content": req.Text},
	}
	body := map[string]any{
		"messages":   messages,
		"max_tokens": c.maxTokens,
	}
	data, err := json.Marshal(body)
	if err != nil {
		c.mu.Unlock()
		return nil, fmt.Errorf("ghost-ai: marshal: %w", err)
	}
	data = append(data, '\n')

	// Write to daemon stdin.
	_, err = c.stdin.Write(data)
	if err != nil {
		// Daemon crashed — try to restart and retry once.
		slog.Warn("[ghost-ai] daemon write failed, restarting", "error", err)
		c.stopDaemonLocked()
		if err := c.startDaemon(GPUEnabled); err != nil {
			c.mu.Unlock()
			return nil, err
		}
		_, err = c.stdin.Write(data)
		if err != nil {
			c.mu.Unlock()
			return nil, fmt.Errorf("ghost-ai: write after restart: %w", err)
		}
	}

	// Read response from daemon stdout (with context cancellation).
	type readResult struct {
		line string
		err  error
	}
	ch := make(chan readResult, 1)
	go func() {
		if c.stdout.Scan() {
			ch <- readResult{line: c.stdout.Text()}
		} else {
			ch <- readResult{err: fmt.Errorf("ghost-ai: daemon closed")}
		}
	}()

	c.mu.Unlock()

	// Wait for response or context cancellation.
	select {
	case <-ctx.Done():
		// Context cancelled — daemon will detect broken pipe or we restart later.
		return nil, fmt.Errorf("ghost-ai: aborted")
	case res := <-ch:
		if res.err != nil {
			return nil, res.err
		}
		return c.parseResponse(res.line)
	}
}

func (c *GhostAIClient) parseResponse(line string) (*Response, error) {
	var resp struct {
		Text             string  `json:"text"`
		Error            string  `json:"error"`
		Model            string  `json:"model"`
		PromptTokens     int     `json:"prompt_tokens"`
		CompletionTokens int     `json:"completion_tokens"`
		TokensPerSecond  float64 `json:"tokens_per_second"`
	}
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		return nil, fmt.Errorf("ghost-ai: parse response: %w", err)
	}
	if resp.Error != "" {
		if resp.Error == "aborted" {
			return nil, fmt.Errorf("ghost-ai: aborted")
		}
		return nil, fmt.Errorf("ghost-ai: %s", resp.Error)
	}

	slog.Info("[ghost-ai] complete",
		"prompt_tok", resp.PromptTokens,
		"gen_tok", resp.CompletionTokens,
		"tps", fmt.Sprintf("%.1f", resp.TokensPerSecond),
		"text_len", len(resp.Text))

	return &Response{
		Text:     resp.Text,
		Provider: "local",
		Model:    c.modelName,
	}, nil
}

func (c *GhostAIClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.idleTimer != nil {
		c.idleTimer.Stop()
		c.idleTimer = nil
	}

	c.stopDaemonLocked()
}

// startDaemon spawns the ghostai daemon process.
// Must NOT be called with c.mu held (it acquires it internally if needed),
// OR must be called when c.mu is already held (the initial spawn from newGhostAIFromDef).
func (c *GhostAIClient) startDaemon(gpuEnabled bool) error {
	binPath, err := findGhostAI()
	if err != nil {
		return err
	}

	contextSize := 512
	if isThinkingModel(c.modelName) {
		contextSize = 2048
	}

	threads := runtime.NumCPU()
	if threads > 12 {
		threads = 12
	}
	if threads < 1 {
		threads = 4
	}

	gpuLayers := 0
	if gpuEnabled {
		gpuLayers = autoGPULayers(c.modelPath)
	}

	slog.Info("[ghost-ai] starting daemon", "model", c.modelName, "threads", threads, "gpu_layers", gpuLayers)
	cmd := exec.Command(binPath, "--daemon",
		"-m", c.modelPath,
		"-t", fmt.Sprintf("%d", threads),
		"--gpu-layers", fmt.Sprintf("%d", gpuLayers),
		"--context-size", fmt.Sprintf("%d", contextSize),
	)
	hideConsoleWindow(cmd) // no-op on non-Windows

	// Add parent PID so daemon auto-exits if we crash.
	cmd.Args = append(cmd.Args, "--parent-pid", fmt.Sprintf("%d", os.Getpid()))

	// Redirect stderr to ghostai.log.
	if logFile, err := openGhostAILog(); err == nil {
		cmd.Stderr = logFile
		c.logFile = logFile
	} else {
		cmd.Stderr = os.Stderr
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("ghost-ai: stdin pipe: %w", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return fmt.Errorf("ghost-ai: stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		stdin.Close()
		return fmt.Errorf("ghost-ai: start: %w", err)
	}

	// Read the ready message.
	scanner := bufio.NewScanner(stdoutPipe)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)

	if !scanner.Scan() {
		cmd.Process.Kill()
		cmd.Wait()
		return fmt.Errorf("ghost-ai: daemon did not send ready message")
	}

	readyLine := scanner.Text()
	var ready struct {
		Ready bool   `json:"ready"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(readyLine), &ready); err != nil || !ready.Ready {
		cmd.Process.Kill()
		cmd.Wait()
		errMsg := ready.Error
		if errMsg == "" {
			errMsg = "daemon failed to load model"
		}
		return fmt.Errorf("ghost-ai: %s", errMsg)
	}

	c.proc = cmd
	c.stdin = stdin
	c.stdout = scanner
	c.running = true

	slog.Info("[ghost-ai] daemon started (model loaded)", "model", c.modelName, "pid", cmd.Process.Pid)

	// Start idle timer.
	if !c.keepAlive {
		c.idleTimer = time.AfterFunc(localIdleTimeout, func() {
			slog.Info("[ghost-ai] idle timeout — stopping daemon")
			c.mu.Lock()
			defer c.mu.Unlock()
			c.stopDaemonLocked()
		})
	}

	return nil
}

// stopDaemonLocked stops the daemon process. Must be called with c.mu held.
func (c *GhostAIClient) stopDaemonLocked() {
	if !c.running {
		return
	}

	// Send quit command.
	if c.stdin != nil {
		fmt.Fprintf(c.stdin, "{\"quit\":true}\n")
		c.stdin.Close()
		c.stdin = nil
	}

	// Wait for clean exit (3 second timeout).
	done := make(chan struct{})
	go func() {
		if c.proc != nil {
			c.proc.Wait()
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		if c.proc != nil && c.proc.Process != nil {
			c.proc.Process.Kill()
		}
	}

	if c.logFile != nil {
		c.logFile.Close()
		c.logFile = nil
	}

	c.proc = nil
	c.stdout = nil
	c.running = false
	slog.Info("[ghost-ai] daemon stopped")
}

// hideConsoleWindow is defined in platform-specific files (stt/ package has it,
// but we need it here too for the llm package).
func hideConsoleWindow(cmd *exec.Cmd) {
	hideGhostAIConsole(cmd)
}

// openGhostAILog opens the ghostai log file for stderr redirection.
func openGhostAILog() (*os.File, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	logDir := filepath.Join(dir, "GhostSpell")
	os.MkdirAll(logDir, 0755)

	logPath := filepath.Join(logDir, "ghostai.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(f, "\n=== ghostai %s started at %s ===\n", "daemon", time.Now().Format(time.RFC3339))
	return f, nil
}

// findGhostAI locates the ghostai binary.
func findGhostAI() (string, error) {
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}

	names := []string{
		fmt.Sprintf("ghostai-%s-%s%s", runtime.GOOS, runtime.GOARCH, ext),
		"ghostai" + ext,
	}

	// Check next to the main executable.
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		for _, name := range names {
			path := filepath.Join(dir, name)
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				return path, nil
			}
		}
	}

	// Check PATH.
	for _, name := range names {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("ghost-ai: ghostai%s not found — run _build.bat to build it", ext)
}

// isThinkingModel returns true for models that can generate <think> blocks.
func isThinkingModel(name string) bool {
	n := strings.ToLower(name)
	return strings.Contains(n, "qwen3") || strings.Contains(n, "deepseek")
}

// isQwen35 checks for Qwen3.5 specifically.
func isQwen35(name string) bool {
	return strings.Contains(strings.ToLower(name), "qwen3.5")
}

// isMacOS13 returns true if running on macOS 13 (Ventura).
func isMacOS13() bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	out, err := exec.Command("sw_vers", "-productVersion").Output()
	if err != nil {
		return false
	}
	return strings.HasPrefix(strings.TrimSpace(string(out)), "13.")
}

// isIntelMac returns true if running on an Intel (x86_64) Mac.
// The Metal backend in llama.cpp can stall on Intel Macs with
// AMD/Intel discrete GPUs — inference hangs indefinitely.
func isIntelMac() bool {
	return runtime.GOOS == "darwin" && runtime.GOARCH == "amd64"
}

// autoGPULayers determines how many layers to offload to GPU.
func autoGPULayers(modelPath string) int {
	// Disable GPU on Intel Macs — Metal backend hangs on discrete AMD/Intel GPUs.
	if isIntelMac() {
		slog.Info("[ghost-ai] Intel Mac detected — using CPU (Metal unreliable on discrete GPU)")
		return 0
	}
	// Disable GPU on macOS 13 — Metal has page-alignment bugs.
	if isMacOS13() {
		slog.Info("[ghost-ai] macOS 13 detected — using CPU (Metal bug)")
		return 0
	}

	fi, err := os.Stat(modelPath)
	if err != nil {
		return 0
	}
	modelGB := float64(fi.Size()) / (1024 * 1024 * 1024)

	ramGB := systemRAMGB()
	if ramGB == 0 {
		ramGB = 8
	}

	overheadGB := 4.0 + modelGB*0.5
	availableGB := float64(ramGB) - overheadGB
	if availableGB < 1 {
		slog.Info("[ghost-ai] not enough RAM for GPU offload, using CPU", "ram_gb", ramGB, "model_gb", fmt.Sprintf("%.1f", modelGB))
		return 0
	}

	if modelGB <= availableGB {
		slog.Info("[ghost-ai] GPU: full offload", "ram_gb", ramGB, "model_gb", fmt.Sprintf("%.1f", modelGB))
		return 99
	}

	ratio := availableGB / modelGB
	layers := int(ratio * 99)
	if layers < 1 {
		layers = 0
	}
	slog.Info("[ghost-ai] GPU: partial offload", "ram_gb", ramGB, "model_gb", fmt.Sprintf("%.1f", modelGB), "layers", layers)
	return layers
}

// systemRAMGB returns total system RAM in GB.
func systemRAMGB() int {
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
		if err != nil {
			return 0
		}
		bytes, _ := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
		return int(bytes / (1024 * 1024 * 1024))
	default:
		return 0
	}
}
