package core

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"
)

// APIServer serves GhostSpell's core Engine over HTTP.
// All endpoints bind to localhost only — not exposed to the network.
type APIServer struct {
	engine *Engine
	server *http.Server
	addr   string // actual listen address after Start
}

// NewAPIServer creates a new API server backed by the given Engine.
// It does not start listening — call Start() for that.
func NewAPIServer(engine *Engine) *APIServer {
	return &APIServer{engine: engine}
}

// Start begins listening on the given address (e.g. "127.0.0.1:7878").
// Returns the actual address (useful when port is 0 for auto-assign).
// Non-blocking — the server runs in a background goroutine.
func (s *APIServer) Start(addr string) (string, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/prompts", s.handlePrompts)
	mux.HandleFunc("POST /api/process", s.handleProcess)
	mux.HandleFunc("POST /api/transcribe", s.handleTranscribe)

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return "", fmt.Errorf("api: listen %s: %w", addr, err)
	}
	s.addr = ln.Addr().String()

	s.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("[api] Server started", "addr", s.addr)
		if err := s.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Error("[api] Server error", "error", err)
		}
	}()

	return s.addr, nil
}

// Shutdown gracefully stops the API server.
func (s *APIServer) Shutdown(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	slog.Info("[api] Server shutting down")
	return s.server.Shutdown(ctx)
}

// Addr returns the address the server is listening on, or empty if not started.
func (s *APIServer) Addr() string {
	return s.addr
}

// --- Request/Response types ---

// ProcessRequest is the JSON body for POST /api/process.
type ProcessRequest struct {
	SkillIndex int    `json:"skill_index"`
	Text       string `json:"text"`
}

// ProcessResponse is the JSON response from POST /api/process.
type ProcessResponse struct {
	Text     string  `json:"text"`
	Provider string  `json:"provider,omitempty"`
	Model    string  `json:"model,omitempty"`
	Duration float64 `json:"duration_seconds"`
}

// TranscribeRequest is the JSON body for POST /api/transcribe.
type TranscribeRequest struct {
	// WAVData is base64-encoded WAV audio.
	WAVData  string `json:"wav_data"`
	Language string `json:"language,omitempty"`
}

// TranscribeResponse is the JSON response from POST /api/transcribe.
type TranscribeResponse struct {
	Text string `json:"text"`
}

// PromptInfo describes a prompt for the GET /api/prompts response.
type PromptInfo struct {
	Index       int    `json:"index"`
	Name        string `json:"name"`
	Icon        string `json:"icon,omitempty"`
	Voice       bool   `json:"voice,omitempty"`
	Vision      bool   `json:"vision,omitempty"`
	DisplayMode string `json:"display_mode,omitempty"`
	Disabled    bool   `json:"disabled,omitempty"`
}

// ErrorResponse is the JSON error envelope.
type ErrorResponse struct {
	Error string `json:"error"`
}

// --- Handlers ---

func (s *APIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":  "ok",
		"has_stt": s.engine.HasSTT(),
	})
}

func (s *APIServer) handlePrompts(w http.ResponseWriter, r *http.Request) {
	cfg := s.engine.Config()
	if cfg == nil {
		writeError(w, http.StatusServiceUnavailable, "no config loaded")
		return
	}

	prompts := make([]PromptInfo, len(cfg.Prompts))
	for i, p := range cfg.Prompts {
		prompts[i] = PromptInfo{
			Index:       i,
			Name:        p.Name,
			Icon:        p.Icon,
			Voice:       p.Voice,
			Vision:      p.Vision,
			DisplayMode: p.DisplayMode,
			Disabled:    p.Disabled,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"prompts":      prompts,
		"active_index": cfg.ActivePrompt,
	})
}

func (s *APIServer) handleProcess(w http.ResponseWriter, r *http.Request) {
	var req ProcessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Text == "" {
		writeError(w, http.StatusBadRequest, "text is required")
		return
	}

	// Use timeout from query param or skill config.
	timeout := s.engine.TimeoutForSkill(req.SkillIndex)
	if tStr := r.URL.Query().Get("timeout"); tStr != "" {
		if tSec, err := strconv.Atoi(tStr); err == nil && tSec > 0 {
			timeout = time.Duration(tSec) * time.Second
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	result, err := s.engine.Process(ctx, req.SkillIndex, req.Text)
	if err != nil {
		status := http.StatusInternalServerError
		if ctx.Err() == context.DeadlineExceeded {
			status = http.StatusGatewayTimeout
		}
		writeError(w, status, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ProcessResponse{
		Text:     result.Text,
		Provider: result.Provider,
		Model:    result.Model,
		Duration: result.Duration.Seconds(),
	})
}

func (s *APIServer) handleTranscribe(w http.ResponseWriter, r *http.Request) {
	var req TranscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.WAVData == "" {
		writeError(w, http.StatusBadRequest, "wav_data is required")
		return
	}

	wavBytes, err := base64.StdEncoding.DecodeString(req.WAVData)
	if err != nil {
		writeError(w, http.StatusBadRequest, "wav_data must be valid base64: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	text, err := s.engine.Transcribe(ctx, wavBytes, req.Language)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(TranscribeResponse{Text: text})
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{Error: msg})
}
