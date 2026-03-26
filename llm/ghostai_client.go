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

	// GPU layers: 99 = offload all layers (GPU backend decides if available).
	// When built without CUDA/Metal/Vulkan, this is ignored by llama.cpp.
	// When GPUEnabled=false, force CPU-only (0 layers).
	gpuLayers := 99
	if !def.GPUEnabled {
		gpuLayers = 0
	}

	args := []string{
		"--model", modelPath,
		"--context-size", fmt.Sprintf("%d", contextSize),
		"--threads", fmt.Sprintf("%d", threads),
		"--gpu-layers", fmt.Sprintf("%d", gpuLayers),
	}

	slog.Info("[ghost-ai] spawning ghostai process", "bin", binPath, "model", def.Model)
	agent, err := procmgr.SpawnHTTPAgent("ghostai", binPath, args, nil)
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
		"--gpu-layers", "99",
	}

	agent, err := procmgr.SpawnHTTPAgent("ghostai", binPath, args, nil)
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
