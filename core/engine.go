// Package core provides GhostSpell's business logic separated from UI concerns.
//
// The Engine struct owns all shared state (config, LLM router, STT, stats)
// and exposes pure methods that return data — no indicators, no sounds,
// no clipboard, no paste. Desktop, Telegram, and CLI callers handle
// presentation and platform glue.
//
// Thread safety: all Engine methods are safe for concurrent use. The internal
// RWMutex protects hot-swappable dependencies (config, router, STT) that
// change when the user modifies settings. Read-heavy workloads (Process,
// Transcribe) take a read lock; writes (SetConfig, SetRouter, SetSTT)
// are rare and brief.
package core

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/chrixbedardcad/GhostSpell/config"
	"github.com/chrixbedardcad/GhostSpell/mode"
	"github.com/chrixbedardcad/GhostSpell/stats"
	"github.com/chrixbedardcad/GhostSpell/stt"
)

// Sentinel errors returned by Engine methods.
// Callers can use errors.Is() to distinguish recoverable conditions.
var (
	ErrNoRouter      = errors.New("no LLM router configured")
	ErrNoSTT         = errors.New("no speech-to-text provider configured")
	ErrNoSpeech      = errors.New("no speech detected")
	ErrEmptyInput    = errors.New("empty input")
)

// Engine is the core of GhostSpell — owns config, LLM, STT, and stats.
// All methods are safe for concurrent use.
// Engine has zero UI dependencies: no Wails, no tray, no indicator, no sound.
type Engine struct {
	mu     sync.RWMutex
	cfg    *config.Config
	router *mode.Router
	stt    stt.Transcriber
	stats  *stats.Stats
}

// NewEngine creates a new Engine with the given dependencies.
// Any dependency can be nil (e.g., STT may be nil if no voice model is configured).
func NewEngine(cfg *config.Config, router *mode.Router, transcriber stt.Transcriber, st *stats.Stats) *Engine {
	return &Engine{
		cfg:    cfg,
		router: router,
		stt:    transcriber,
		stats:  st,
	}
}

// --- Dependency accessors ---

// Config returns the current config pointer. The config should not be
// modified by callers — use SetConfig to swap it atomically.
func (e *Engine) Config() *config.Config {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.cfg
}

// SetConfig atomically replaces the config. Called after settings are saved.
func (e *Engine) SetConfig(cfg *config.Config) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cfg = cfg
}

// SetRouter atomically replaces the LLM router.
// Called after model/provider changes or config reload.
func (e *Engine) SetRouter(r *mode.Router) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.router = r
}

// SetSTT atomically replaces the speech-to-text transcriber.
// Called after voice model download or config change.
func (e *Engine) SetSTT(t stt.Transcriber) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stt = t
}

// Stats returns the stats tracker (may be nil).
func (e *Engine) Stats() *stats.Stats {
	return e.stats
}

// HasSTT reports whether a speech-to-text transcriber is configured.
func (e *Engine) HasSTT() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.stt != nil
}

// STTName returns the name of the active STT provider, or empty string.
func (e *Engine) STTName() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.stt == nil {
		return ""
	}
	return e.stt.Name()
}

// --- Core operations ---

// ProcessResult contains the result of a skill execution.
type ProcessResult struct {
	Text     string        // LLM output text
	Provider string        // provider that handled the request (e.g. "local", "openai")
	Model    string        // model name (e.g. "qwen3.5-0.8b")
	Duration time.Duration // wall-clock time for the LLM call
}

// Process runs the given text through the skill at skillIdx.
// Returns the LLM result. Pure logic — no UI, no paste, no sound.
func (e *Engine) Process(ctx context.Context, skillIdx int, text string) (*ProcessResult, error) {
	e.mu.RLock()
	r := e.router
	e.mu.RUnlock()

	if r == nil {
		return nil, ErrNoRouter
	}

	start := time.Now()
	resp, err := r.Process(ctx, skillIdx, text)
	if err != nil {
		return nil, err
	}

	result := strings.TrimSpace(resp.Text)
	elapsed := time.Since(start)

	slog.Info("[core] Process complete",
		"skill_idx", skillIdx,
		"input_len", len(text),
		"result_len", len(result),
		"provider", resp.Provider,
		"model", resp.Model,
		"elapsed", elapsed,
	)

	return &ProcessResult{
		Text:     result,
		Provider: resp.Provider,
		Model:    resp.Model,
		Duration: elapsed,
	}, nil
}

// ProcessWithImages runs text + images through the skill at skillIdx.
// Used by vision skills (screenshot describe, OCR).
func (e *Engine) ProcessWithImages(ctx context.Context, skillIdx int, text string, images [][]byte) (*ProcessResult, error) {
	e.mu.RLock()
	r := e.router
	e.mu.RUnlock()

	if r == nil {
		return nil, ErrNoRouter
	}

	start := time.Now()
	resp, err := r.ProcessWithImages(ctx, skillIdx, text, images)
	if err != nil {
		return nil, err
	}

	result := strings.TrimSpace(resp.Text)
	elapsed := time.Since(start)

	return &ProcessResult{
		Text:     result,
		Provider: resp.Provider,
		Model:    resp.Model,
		Duration: elapsed,
	}, nil
}

// Transcribe converts WAV audio data to text using the configured STT provider.
// If language is empty, the provider auto-detects the language.
// Pure logic — no recording, no indicator, no paste.
func (e *Engine) Transcribe(ctx context.Context, wavData []byte, language string) (string, error) {
	e.mu.RLock()
	t := e.stt
	e.mu.RUnlock()

	if t == nil {
		return "", ErrNoSTT
	}
	if len(wavData) == 0 {
		return "", ErrEmptyInput
	}

	start := time.Now()
	text, err := t.Transcribe(ctx, wavData, language)
	if err != nil {
		return "", err
	}

	text = strings.TrimSpace(text)
	slog.Info("[core] Transcribe complete",
		"wav_bytes", len(wavData),
		"text_len", len(text),
		"elapsed", time.Since(start),
	)

	return text, nil
}

// VoiceLanguage returns the configured voice language from the config.
// Returns empty string for auto-detect.
func (e *Engine) VoiceLanguage() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.cfg == nil {
		return ""
	}
	return e.cfg.Voice.Language
}

// VoiceNativeLanguage returns the speaker's native language for accent correction.
// Returns empty string if not configured.
func (e *Engine) VoiceNativeLanguage() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.cfg == nil {
		return ""
	}
	return e.cfg.Voice.NativeLanguage
}

// VoiceProcess transcribes audio and then processes the transcript through a skill.
// Combines Transcribe + Process for the common voice skill pipeline.
// Returns (result, transcript, error). On transcription failure, transcript is empty.
// On LLM failure, transcript is returned so the caller can still use the raw text.
//
// If the speaker has a configured native language, it's prepended to the transcript
// so the LLM can correct accent-related transcription errors.
func (e *Engine) VoiceProcess(ctx context.Context, skillIdx int, wavData []byte) (*ProcessResult, string, error) {
	language := e.VoiceLanguage()

	transcript, err := e.Transcribe(ctx, wavData, language)
	if err != nil {
		return nil, "", err
	}
	if transcript == "" {
		return nil, "", ErrNoSpeech
	}

	// Prepend native language context for accent correction.
	textToSend := transcript
	if native := e.VoiceNativeLanguage(); native != "" {
		textToSend = "[Speaker's native language: " + native +
			". The transcription may contain errors due to accent. Correct accordingly.]\n\n" + transcript
	}

	result, err := e.Process(ctx, skillIdx, textToSend)
	if err != nil {
		return nil, transcript, err
	}

	return result, transcript, nil
}

// TimeoutForSkill returns the configured timeout for the given skill index.
// Returns 30s as a default if no router is configured.
func (e *Engine) TimeoutForSkill(skillIdx int) time.Duration {
	e.mu.RLock()
	r := e.router
	e.mu.RUnlock()

	if r == nil {
		return 30 * time.Second
	}
	ms := r.TimeoutForPrompt(skillIdx)
	return time.Duration(ms) * time.Millisecond
}
