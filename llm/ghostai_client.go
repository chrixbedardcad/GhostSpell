package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chrixbedardcad/GhostSpell/internal/procmgr"
)

// GhostAIClient implements the Client interface by communicating with a
// ghostai.exe HTTP server process (llama.cpp inference via OpenAI-compatible API).
type GhostAIClient struct {
	mu        sync.Mutex
	agent     *procmgr.AgentProcess
	modelPath string
	modelName string
	maxTokens int
	keepAlive bool
	idleTimer *time.Timer
	httpClient *http.Client
}

// newGhostAIFromDef creates a GhostAIClient that spawns and manages ghostai.exe.
func newGhostAIFromDef(def LLMProviderDefCompat) (*GhostAIClient, error) {
	maxTokens := def.MaxTokens
	if maxTokens == 0 {
		maxTokens = 256
	}
	// Thinking models need a much larger budget — their <think> blocks
	// consume most tokens before the answer. Force minimum 2048 regardless
	// of what the config says.
	if isThinkingModel(def.Model) && maxTokens < 2048 {
		maxTokens = 2048
	}

	modelPath, err := resolveLocalModel(def.Model)
	if err != nil {
		return nil, fmt.Errorf("local model %q: %w", def.Model, err)
	}

	// Find ghostai binary.
	binPath, err := findGhostAI()
	if err != nil {
		return nil, err
	}

	// Spawn ghostai.exe with the model.
	contextSize := 512
	if isThinkingModel(def.Model) {
		contextSize = 2048
	}

	// Auto-detect thread count: use NumCPU, cap at 12.
	threads := runtime.NumCPU()
	if threads > 12 {
		threads = 12
	}
	if threads < 1 {
		threads = 4
	}

	// GPU layers: offload model layers to GPU (Metal/CUDA/Vulkan).
	// On Apple Silicon (unified memory), large models need headroom for
	// inference buffers (KV cache, compute scratch). Using 99 (all layers)
	// can trigger GGML_ASSERT(buf_dst) in the Metal backend when memory is
	// tight. Scale GPU layers based on model file size vs available RAM.
	gpuLayers := 0
	if def.GPUEnabled {
		gpuLayers = autoGPULayers(modelPath)
	}

	args := []string{
		"--model", modelPath,
		"--context-size", fmt.Sprintf("%d", contextSize),
		"--threads", fmt.Sprintf("%d", threads),
		"--gpu-layers", fmt.Sprintf("%d", gpuLayers),
	}

	slog.Info("[ghost-ai] spawning ghostai process", "bin", binPath, "model", def.Model)
	agent, err := procmgr.SpawnHTTPAgent("ghostai", binPath, args, nil, )
	if err != nil {
		return nil, fmt.Errorf("ghost-ai: failed to start: %w", err)
	}
	slog.Info("[ghost-ai] process started", "pid", agent.PID(), "port", agent.Port)

	c := &GhostAIClient{
		agent:     agent,
		modelPath: modelPath,
		modelName: def.Model,
		maxTokens: maxTokens,
		keepAlive: def.KeepAlive,
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}

	// Start idle timer (unless keep-alive).
	if !def.KeepAlive {
		c.idleTimer = time.AfterFunc(localIdleTimeout, func() {
			slog.Info("[ghost-ai] idle timeout — unloading model")
			c.mu.Lock()
			defer c.mu.Unlock()
			c.postJSON("/v1/models/unload", nil)
		})
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

	// Check if process is alive, restart if needed.
	if err := c.ensureRunning(); err != nil {
		c.mu.Unlock()
		return nil, err
	}

	// Reset idle timer.
	if c.idleTimer != nil {
		c.idleTimer.Reset(localIdleTimeout)
	}

	baseURL := c.agent.BaseURL()
	c.mu.Unlock()

	// Build chat messages.
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
		return nil, fmt.Errorf("ghost-ai: marshal: %w", err)
	}

	// Use request context so cancellation aborts the HTTP call,
	// which causes ghostai.exe to detect the closed connection and abort inference.
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		baseURL+"/v1/chat/completions", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("ghost-ai: request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("ghost-ai: aborted")
		}
		return nil, fmt.Errorf("ghost-ai: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ghost-ai: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		json.Unmarshal(respBody, &errResp)
		return nil, fmt.Errorf("ghost-ai: %s", errResp.Error)
	}

	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int     `json:"prompt_tokens"`
			CompletionTokens int     `json:"completion_tokens"`
			TokensPerSecond  float64 `json:"tokens_per_second"`
		} `json:"usage"`
		Model string `json:"model"`
	}

	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("ghost-ai: parse response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("ghost-ai: empty response")
	}

	text := chatResp.Choices[0].Message.Content

	slog.Info("[ghost-ai] complete",
		"prompt_tok", chatResp.Usage.PromptTokens,
		"gen_tok", chatResp.Usage.CompletionTokens,
		"tps", fmt.Sprintf("%.1f", chatResp.Usage.TokensPerSecond),
		"text_len", len(text))

	return &Response{
		Text:     text,
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

	if c.agent != nil {
		c.agent.Stop(3 * time.Second)
		c.agent = nil
	}
}

// ensureRunning checks if the ghostai process is alive and restarts it if needed.
// Must be called with c.mu held.
func (c *GhostAIClient) ensureRunning() error {
	if c.agent != nil {
		if err := c.agent.Health(); err == nil {
			return nil
		}
		slog.Warn("[ghost-ai] process unhealthy, restarting")
		c.agent.Stop(2 * time.Second)
	}

	// Re-spawn.
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

	args := []string{
		"--model", c.modelPath,
		"--context-size", fmt.Sprintf("%d", contextSize),
		"--threads", fmt.Sprintf("%d", threads),
		"--gpu-layers", fmt.Sprintf("%d", autoGPULayers(c.modelPath)),
	}

	agent, err := procmgr.SpawnHTTPAgent("ghostai", binPath, args, nil, )
	if err != nil {
		return fmt.Errorf("ghost-ai: restart failed: %w", err)
	}

	c.agent = agent
	slog.Info("[ghost-ai] process restarted", "pid", agent.PID(), "port", agent.Port)
	return nil
}

// postJSON sends a POST request to the ghostai server. Used for model management.
func (c *GhostAIClient) postJSON(path string, body any) error {
	if c.agent == nil {
		return fmt.Errorf("ghost-ai not running")
	}

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reqBody = bytes.NewReader(data)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.agent.BaseURL()+path, reqBody)
	if err != nil {
		return err
	}
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
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

	return "", fmt.Errorf("ghost-ai: ghostai%s not found — build it with: go build -tags ghostai -o ghostai%s ./cmd/ghostai", ext, ext)
}

// isThinkingModel returns true for models that can generate <think> blocks.
func isThinkingModel(name string) bool {
	n := strings.ToLower(name)
	return strings.Contains(n, "qwen3") || strings.Contains(n, "deepseek")
}

// isQwen35 is kept for reference but no longer used in the client —
// thinking model handling is now done server-side in ghostai.exe.
func isQwen35(name string) bool {
	return strings.Contains(strings.ToLower(name), "qwen3.5")
}

// isMacOS13 returns true if running on macOS 13 (Ventura).
// The Metal backend in llama.cpp has page-alignment issues on macOS 13 that
// cause GGML_ASSERT crashes. GPU must be disabled on this version.
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

// autoGPULayers determines how many layers to offload to GPU based on model
// file size and available system RAM. On Apple Silicon (unified memory), the
// Metal backend needs headroom for KV cache and compute buffers beyond the
// model weights. Overcommitting causes GGML_ASSERT(buf_dst) crashes in the
// Metal tensor copy path (llama.cpp b8281).
func autoGPULayers(modelPath string) int {
	fi, err := os.Stat(modelPath)
	if err != nil {
		return 0
	}
	modelGB := float64(fi.Size()) / (1024 * 1024 * 1024)

	ramGB := systemRAMGB()
	if ramGB == 0 {
		ramGB = 8 // safe default
	}

	// Reserve memory for OS + GhostSpell + Metal inference buffers (KV cache,
	// compute scratch). The Metal backend in llama.cpp b8281 needs ~2x the
	// model weight size for inference buffers; underestimating causes
	// GGML_ASSERT(buf_dst) in ggml_metal_cpy_tensor_async.
	overheadGB := 4.0 + modelGB*0.5 // OS/app + ~50% of model for Metal buffers
	availableGB := float64(ramGB) - overheadGB
	if availableGB < 1 {
		slog.Info("[ghost-ai] not enough RAM for GPU offload, using CPU", "ram_gb", ramGB, "model_gb", fmt.Sprintf("%.1f", modelGB))
		return 0
	}

	if modelGB <= availableGB {
		// Model + buffers fit comfortably — offload everything.
		slog.Info("[ghost-ai] GPU: full offload", "ram_gb", ramGB, "model_gb", fmt.Sprintf("%.1f", modelGB))
		return 99
	}

	// Model too large for full offload — use partial (proportional).
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
