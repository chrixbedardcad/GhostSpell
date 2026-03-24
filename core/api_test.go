package core

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chrixbedardcad/GhostSpell/config"
	"github.com/chrixbedardcad/GhostSpell/llm"
	"github.com/chrixbedardcad/GhostSpell/mode"
)

// testConfig returns a minimal config with prompts for API testing.
func testConfig() *config.Config {
	cfg := config.DefaultConfig()
	return &cfg
}

// mockRouter wraps a mode.Router for testing. We can't easily mock
// mode.Router, so we test error paths and use Engine's sentinel errors.

func TestAPI_Health(t *testing.T) {
	e := NewEngine(testConfig(), nil, &mockTranscriber{text: "hi"}, nil)
	srv := NewAPIServer(e)

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	srv.handleHealth(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "ok" {
		t.Fatalf("expected status=ok, got %v", resp["status"])
	}
	if resp["has_stt"] != true {
		t.Fatalf("expected has_stt=true, got %v", resp["has_stt"])
	}
}

func TestAPI_Health_NoSTT(t *testing.T) {
	e := NewEngine(testConfig(), nil, nil, nil)
	srv := NewAPIServer(e)

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	srv.handleHealth(w, req)

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["has_stt"] != false {
		t.Fatalf("expected has_stt=false, got %v", resp["has_stt"])
	}
}

func TestAPI_Prompts(t *testing.T) {
	cfg := testConfig()
	e := NewEngine(cfg, nil, nil, nil)
	srv := NewAPIServer(e)

	req := httptest.NewRequest("GET", "/api/prompts", nil)
	w := httptest.NewRecorder()
	srv.handlePrompts(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Prompts     []PromptInfo `json:"prompts"`
		ActiveIndex int          `json:"active_index"`
	}
	json.NewDecoder(w.Body).Decode(&resp)

	if len(resp.Prompts) == 0 {
		t.Fatal("expected prompts list to be non-empty")
	}
	if resp.Prompts[0].Name != "Correct" {
		t.Fatalf("expected first prompt to be Correct, got %q", resp.Prompts[0].Name)
	}
}

func TestAPI_Prompts_NoConfig(t *testing.T) {
	e := NewEngine(nil, nil, nil, nil)
	srv := NewAPIServer(e)

	req := httptest.NewRequest("GET", "/api/prompts", nil)
	w := httptest.NewRecorder()
	srv.handlePrompts(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestAPI_Process_NoRouter(t *testing.T) {
	e := NewEngine(testConfig(), nil, nil, nil)
	srv := NewAPIServer(e)

	body := `{"skill_index": 0, "text": "hello"}`
	req := httptest.NewRequest("POST", "/api/process", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.handleProcess(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	var resp ErrorResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if !strings.Contains(resp.Error, "no LLM router") {
		t.Fatalf("expected router error, got %q", resp.Error)
	}
}

func TestAPI_Process_EmptyText(t *testing.T) {
	e := NewEngine(testConfig(), nil, nil, nil)
	srv := NewAPIServer(e)

	body := `{"skill_index": 0, "text": ""}`
	req := httptest.NewRequest("POST", "/api/process", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.handleProcess(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAPI_Process_InvalidJSON(t *testing.T) {
	e := NewEngine(testConfig(), nil, nil, nil)
	srv := NewAPIServer(e)

	req := httptest.NewRequest("POST", "/api/process", strings.NewReader("{bad json"))
	w := httptest.NewRecorder()
	srv.handleProcess(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAPI_Transcribe_NoSTT(t *testing.T) {
	e := NewEngine(testConfig(), nil, nil, nil)
	srv := NewAPIServer(e)

	wavB64 := base64.StdEncoding.EncodeToString([]byte("fake-wav"))
	body := `{"wav_data": "` + wavB64 + `"}`
	req := httptest.NewRequest("POST", "/api/transcribe", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.handleTranscribe(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestAPI_Transcribe_Success(t *testing.T) {
	e := NewEngine(testConfig(), nil, &mockTranscriber{text: "hello world"}, nil)
	srv := NewAPIServer(e)

	wavB64 := base64.StdEncoding.EncodeToString([]byte("fake-wav"))
	body := `{"wav_data": "` + wavB64 + `", "language": "en"}`
	req := httptest.NewRequest("POST", "/api/transcribe", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.handleTranscribe(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp TranscribeResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Text != "hello world" {
		t.Fatalf("expected 'hello world', got %q", resp.Text)
	}
}

func TestAPI_Transcribe_EmptyWAV(t *testing.T) {
	e := NewEngine(testConfig(), nil, &mockTranscriber{text: "hi"}, nil)
	srv := NewAPIServer(e)

	body := `{"wav_data": ""}`
	req := httptest.NewRequest("POST", "/api/transcribe", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.handleTranscribe(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAPI_Transcribe_InvalidBase64(t *testing.T) {
	e := NewEngine(testConfig(), nil, &mockTranscriber{text: "hi"}, nil)
	srv := NewAPIServer(e)

	body := `{"wav_data": "not-valid-base64!!!"}`
	req := httptest.NewRequest("POST", "/api/transcribe", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.handleTranscribe(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAPI_StartAndShutdown(t *testing.T) {
	e := NewEngine(testConfig(), nil, nil, nil)
	srv := NewAPIServer(e)

	addr, err := srv.Start("127.0.0.1:0")
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if addr == "" {
		t.Fatal("expected non-empty address")
	}
	if srv.Addr() != addr {
		t.Fatalf("Addr() mismatch: %q vs %q", srv.Addr(), addr)
	}

	// Hit the health endpoint on the real server.
	resp, err := http.Get("http://" + addr + "/api/health")
	if err != nil {
		t.Fatalf("GET /api/health failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}
}

// Verify that Process with a working router returns results.
// We need a real Router for this — use a minimal mock LLM client.
type mockLLMClient struct {
	text string
}

func (m *mockLLMClient) Send(_ context.Context, req llm.Request) (*llm.Response, error) {
	return &llm.Response{Text: m.text, Provider: "mock", Model: "mock-model"}, nil
}

func (m *mockLLMClient) Provider() string { return "mock" }
func (m *mockLLMClient) Close()           {}

func TestAPI_Process_Success(t *testing.T) {
	cfg := testConfig()
	cfg.DefaultModel = "test"
	cfg.Models["test"] = config.ModelEntry{Provider: "mock", Model: "mock-model"}
	cfg.Providers["mock"] = config.ProviderConfig{APIKey: "fake"}
	router := mode.NewRouter(cfg, &mockLLMClient{text: "corrected text"})
	e := NewEngine(cfg, router, nil, nil)
	srv := NewAPIServer(e)

	body := `{"skill_index": 0, "text": "teh quik brown fox"}`
	req := httptest.NewRequest("POST", "/api/process", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.handleProcess(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp ProcessResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Text != "corrected text" {
		t.Fatalf("expected 'corrected text', got %q", resp.Text)
	}
	if resp.Provider != "mock" {
		t.Fatalf("expected provider=mock, got %q", resp.Provider)
	}
}
