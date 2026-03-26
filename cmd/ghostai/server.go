//go:build ghostai

package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/chrixbedardcad/GhostSpell/llm/ghostai"
)

type server struct {
	engine     *ghostai.Engine
	modelName  string
	mu         sync.RWMutex
	shutdownCh chan struct{}
}

func newServer(engine *ghostai.Engine) *server {
	return &server{
		engine:     engine,
		shutdownCh: make(chan struct{}),
	}
}

// --- Request/Response types (OpenAI-compatible subset) ---

type chatRequest struct {
	Messages  []chatMessage `json:"messages"`
	MaxTokens int           `json:"max_tokens"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []chatChoice `json:"choices"`
	Usage   chatUsage    `json:"usage"`
	Model   string       `json:"model"`
}

type chatChoice struct {
	Index   int         `json:"index"`
	Message chatMessage `json:"message"`
}

type chatUsage struct {
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TokensPerSecond  float64 `json:"tokens_per_second"`
}

type modelLoadRequest struct {
	ModelPath string `json:"model_path"`
}

// --- Handlers ---

func (s *server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	info := map[string]any{
		"status": "ok",
		"model":  s.modelName,
		"loaded": s.engine != nil && s.engine.IsLoaded(),
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}

	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	if len(req.Messages) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "messages array is empty"})
		return
	}

	// Extract system + user messages.
	var systemMsg, userMsg string
	for _, msg := range req.Messages {
		switch msg.Role {
		case "system":
			systemMsg = msg.Content
		case "user":
			userMsg = msg.Content
		}
	}

	if userMsg == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no user message"})
		return
	}

	s.mu.RLock()
	engine := s.engine
	modelName := s.modelName
	s.mu.RUnlock()

	if engine == nil || !engine.IsLoaded() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "no model loaded"})
		return
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 256
	}

	// Apply thinking model logic.
	thinking := isThinkingModel(modelName)
	if thinking && !isQwen35(modelName) {
		systemMsg = "/no_think\n" + systemMsg
	}

	// Dynamic token cap.
	// Thinking models (Qwen3/3.5, DeepSeek) use most tokens on <think> blocks,
	// so they need a much larger budget to produce the actual answer.
	inputWords := len(strings.Fields(userMsg))
	if thinking {
		// Thinking models: generous budget. The <think> block alone can be 500+ tokens.
		dynamicMax := inputWords*5 + 512
		if dynamicMax < 2048 {
			dynamicMax = 2048
		}
		if dynamicMax > maxTokens {
			maxTokens = dynamicMax
		}
	} else {
		// Non-thinking models: tighter budget.
		dynamicMax := inputWords*3 + 128
		if dynamicMax < 512 {
			dynamicMax = 512
		}
		if dynamicMax < maxTokens {
			maxTokens = dynamicMax
		}
	}
	if maxTokens < 64 {
		maxTokens = 64
	}

	// Format using model's chat template.
	prompt, err := engine.ApplyChat(systemMsg, userMsg)
	if err != nil {
		slog.Warn("[ghostai] chat template failed, using raw", "error", err)
		prompt = systemMsg + "\n\nUser: " + userMsg
	}

	if isQwen35(modelName) {
		prompt += "<think>\n\n</think>\n\n"
	}

	// Use request context — cancelled if client disconnects (abort).
	ctx := r.Context()

	start := time.Now()
	text, stats, err := engine.Complete(ctx, prompt, maxTokens)
	duration := time.Since(start)

	if err != nil {
		if ctx.Err() != nil {
			// Client disconnected (abort).
			writeJSON(w, 499, map[string]string{"error": "request cancelled"})
			return
		}
		slog.Error("[ghostai] completion failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Log raw output for debugging.
	slog.Info("[ghostai] raw output", "len", len(text), "preview", truncate(text, 500))

	// Clean model output.
	raw := text
	text = cleanLocalModelResponse(text)
	slog.Info("[ghostai] cleaned output", "len", len(text), "preview", truncate(text, 200))
	if strings.TrimSpace(text) == "" {
		slog.Warn("[ghostai] cleaned output is empty, trying extractFromThinking")
		text = extractFromThinking(raw)
		if strings.TrimSpace(text) == "" {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "model returned empty content"})
			return
		}
	}

	slog.Info("[ghostai] complete",
		"prompt_tok", stats.PromptTokens,
		"gen_tok", stats.CompletionTokens,
		"tps", fmt.Sprintf("%.1f", stats.TokensPerSecond),
		"elapsed", duration.Round(time.Millisecond),
		"text_len", len(text),
	)

	resp := chatResponse{
		Choices: []chatChoice{{
			Index: 0,
			Message: chatMessage{
				Role:    "assistant",
				Content: text,
			},
		}},
		Usage: chatUsage{
			PromptTokens:     stats.PromptTokens,
			CompletionTokens: stats.CompletionTokens,
			TokensPerSecond:  stats.TokensPerSecond,
		},
		Model: modelName,
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *server) handleModelLoad(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}

	var req modelLoadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Unload current model if one is loaded.
	if s.engine.IsLoaded() {
		s.engine.Unload()
	}

	slog.Info("[ghostai] loading model", "path", req.ModelPath)
	if err := s.engine.Load(req.ModelPath); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Derive model name from filename.
	parts := strings.Split(req.ModelPath, "/")
	if len(parts) == 0 {
		parts = strings.Split(req.ModelPath, "\\")
	}
	name := parts[len(parts)-1]
	name = strings.TrimSuffix(name, ".gguf")
	s.modelName = name

	slog.Info("[ghostai] model loaded", "name", s.modelName)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "model": s.modelName})
}

func (s *server) handleModelUnload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.engine.Unload()
	slog.Info("[ghostai] model unloaded")
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *server) handleModelInfo(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.engine.IsLoaded() {
		writeJSON(w, http.StatusOK, map[string]any{"loaded": false})
		return
	}

	info, err := s.engine.ModelInfo()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"loaded":        true,
		"name":          s.modelName,
		"description":   info.Description,
		"size_bytes":    info.SizeBytes,
		"num_params":    info.NumParams,
		"context_train": info.ContextTrain,
		"vocab_size":    info.VocabSize,
	})
}

func (s *server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "shutting down"})
	// Signal main goroutine to shut down.
	select {
	case s.shutdownCh <- struct{}{}:
	default:
	}
}

// --- Helpers ---

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// --- Output cleaning (moved from ghostai_client.go) ---

func isThinkingModel(name string) bool {
	n := strings.ToLower(name)
	return strings.Contains(n, "qwen3") || strings.Contains(n, "deepseek")
}

func isQwen35(name string) bool {
	return strings.Contains(strings.ToLower(name), "qwen3.5")
}

func cleanLocalModelResponse(s string) string {
	s = stripThinkingTags(s)

	for _, tok := range []string{"<|im_end|>", "<|im_start|>", "</s>", "<|endoftext|>", "/no_think", "no_think"} {
		s = strings.ReplaceAll(s, tok, "")
	}

	trimmed := strings.TrimSpace(s)
	firstLine := trimmed
	if nl := strings.IndexByte(trimmed, '\n'); nl != -1 {
		firstLine = trimmed[:nl]
	}
	for _, marker := range []string{"Answer:", "Answer :", "Corrected:", "Corrected text:"} {
		if strings.HasPrefix(firstLine, marker) {
			after := strings.TrimSpace(trimmed[len(marker):])
			if after != "" {
				s = after
				break
			}
		}
	}

	for _, stop := range []string{"\nUser:", "\nuser:", "\nAssistant:", "\nassistant:", "\nSystem:", "\n---"} {
		if idx := strings.Index(s, stop); idx != -1 {
			s = s[:idx]
		}
	}

	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > 1 {
		first := strings.ToLower(strings.TrimSpace(lines[0]))
		reasoningStarts := []string{"okay,", "ok,", "let me", "let's", "i need to", "first,",
			"looking at", "the user", "checking", "i'll", "now,", "so,", "here"}
		for _, prefix := range reasoningStarts {
			if strings.HasPrefix(first, prefix) {
				for i := len(lines) - 1; i >= 1; i-- {
					candidate := strings.TrimSpace(lines[i])
					if candidate != "" && !strings.HasPrefix(strings.ToLower(candidate), "answer") {
						s = candidate
						break
					}
				}
				break
			}
		}
	}

	return strings.TrimSpace(s)
}

func stripThinkingTags(s string) string {
	for {
		start := strings.Index(s, "<think>")
		if start == -1 {
			break
		}
		end := strings.Index(s[start:], "</think>")
		if end == -1 {
			s = s[:start]
			break
		}
		s = s[:start] + s[start+end+8:]
	}
	return s
}

func extractFromThinking(raw string) string {
	start := strings.Index(raw, "<think>")
	if start == -1 {
		return ""
	}
	content := raw[start+7:]
	if end := strings.Index(content, "</think>"); end != -1 {
		content = content[:end]
	}

	lines := strings.Split(content, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		for _, marker := range []string{
			"Corrected text:", "Corrected:", "Corrected Text:",
			"Answer:", "Result:", "Output:", "Fixed:",
			"Final answer:", "Final:", "Response:",
		} {
			if strings.HasPrefix(line, marker) {
				answer := strings.TrimSpace(line[len(marker):])
				for j := i + 1; j < len(lines); j++ {
					next := strings.TrimSpace(lines[j])
					if next == "" || strings.HasPrefix(next, "**") || strings.HasPrefix(next, "---") {
						break
					}
					answer += "\n" + next
				}
				if answer != "" {
					return cleanLocalModelResponse(answer)
				}
			}
		}
	}
	return ""
}
