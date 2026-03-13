package ghostai

import (
	"log/slog"
	"sync/atomic"
	"time"
)

// Tracer provides structured debug output for Ghost-AI engine operations.
// All operations are logged via slog; when verbose mode is on, extra per-token
// detail is emitted.
type Tracer struct {
	verbose atomic.Bool
}

// NewTracer creates a tracer. Verbose mode can be toggled at runtime.
func NewTracer(verbose bool) *Tracer {
	t := &Tracer{}
	t.verbose.Store(verbose)
	return t
}

// SetVerbose enables/disables per-token trace output.
func (t *Tracer) SetVerbose(v bool) { t.verbose.Store(v) }

// Verbose returns whether per-token tracing is on.
func (t *Tracer) Verbose() bool { return t.verbose.Load() }

// TraceLoad logs model loading.
func (t *Tracer) TraceLoad(path string) {
	slog.Info("[ghost-ai] loading model", "path", path)
}

// TraceLoadDone logs successful model load with timing and model info.
func (t *Tracer) TraceLoadDone(info ModelInfo, elapsed time.Duration) {
	sizeMB := info.SizeBytes / (1024 * 1024)
	paramsM := info.NumParams / 1_000_000
	slog.Info("[ghost-ai] load: complete",
		"elapsed", elapsed.Round(time.Millisecond),
		"size_mb", sizeMB,
		"params_m", paramsM,
		"vocab", info.VocabSize,
		"ctx_train", info.ContextTrain,
		"desc", info.Description)
}

// TraceLoadFail logs model load failure.
func (t *Tracer) TraceLoadFail(err error, elapsed time.Duration) {
	slog.Error("[ghost-ai] load: FAILED",
		"elapsed", elapsed.Round(time.Millisecond),
		"error", err)
}

// TraceComplete logs the start of a completion.
func (t *Tracer) TraceComplete(promptLen int, maxTokens int) {
	slog.Info("[ghost-ai] complete: start",
		"prompt_len", promptLen,
		"max_tokens", maxTokens)
}

// TraceCompleteDone logs completion results.
func (t *Tracer) TraceCompleteDone(stats Stats, textLen int) {
	slog.Info("[ghost-ai] complete: done",
		"prompt_tok", stats.PromptTokens,
		"gen_tok", stats.CompletionTokens,
		"prompt_ms", stats.PromptTimeMs,
		"gen_ms", stats.CompletionTimeMs,
		"tps", stats.TokensPerSecond,
		"text_len", textLen)
}

// TraceCompleteFail logs completion failure.
func (t *Tracer) TraceCompleteFail(err error) {
	slog.Error("[ghost-ai] complete: FAILED", "error", err)
}

// TraceAbort logs an abort event.
func (t *Tracer) TraceAbort() {
	slog.Warn("[ghost-ai] abort: user cancelled inference")
}

// TraceUnload logs model unload.
func (t *Tracer) TraceUnload() {
	slog.Info("[ghost-ai] unload: freeing model memory")
}

// TraceCircuitTrip logs circuit breaker activation.
func (t *Tracer) TraceCircuitTrip(failures int) {
	slog.Error("[ghost-ai] circuit: TRIPPED — engine disabled",
		"consecutive_failures", failures)
}

// TraceCircuitReset logs circuit breaker recovery.
func (t *Tracer) TraceCircuitReset() {
	slog.Info("[ghost-ai] circuit: reset — engine re-enabled")
}
