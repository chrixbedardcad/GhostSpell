package gui

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/chrixbedardcad/GhostSpell/config"
	"github.com/chrixbedardcad/GhostSpell/llm"
)

// BenchmarkResult holds the result of benchmarking all models.
type BenchmarkResult struct {
	Running bool                `json:"running"`
	Done    bool                `json:"done"`
	Models  []BenchmarkModelRes `json:"models"`
	Error   string              `json:"error,omitempty"`
}

// BenchmarkModelRes holds the benchmark result for one model.
type BenchmarkModelRes struct {
	Label      string `json:"label"`
	Provider   string `json:"provider"`
	Model      string `json:"model"`
	DurationMs int64  `json:"duration_ms"`
	Output     string `json:"output"`
	Status     string `json:"status"` // "pending", "running", "success", "error", "timeout"
	Error      string `json:"error,omitempty"`
}

var (
	benchMu     sync.Mutex
	benchResult *BenchmarkResult
)

const benchmarkTestText = "fix this: helo wrld, i'm typng fast and makng erors evrywhere"

// RunBenchmark starts a background benchmark of all configured models.
func (s *SettingsService) RunBenchmark() string {
	guiLog("[GUI] JS called: RunBenchmark")

	benchMu.Lock()
	if benchResult != nil && benchResult.Running {
		benchMu.Unlock()
		return "error: benchmark already running"
	}

	cfg := s.cfgCopy
	if len(cfg.Models) == 0 {
		benchMu.Unlock()
		return "error: no models configured"
	}

	// Get the active prompt text.
	promptText := config.DefaultCorrectPrompt
	if cfg.ActivePrompt >= 0 && cfg.ActivePrompt < len(cfg.Prompts) {
		promptText = cfg.Prompts[cfg.ActivePrompt].Prompt
	}

	// Initialize results.
	result := &BenchmarkResult{Running: true}
	for label, me := range cfg.Models {
		result.Models = append(result.Models, BenchmarkModelRes{
			Label:    label,
			Provider: me.Provider,
			Model:    me.Model,
			Status:   "pending",
		})
	}
	benchResult = result
	benchMu.Unlock()

	// Run benchmark in background.
	go func() {
		for i := range result.Models {
			benchMu.Lock()
			result.Models[i].Status = "running"
			benchMu.Unlock()

			label := result.Models[i].Label
			me, ok := cfg.Models[label]
			if !ok {
				benchMu.Lock()
				result.Models[i].Status = "error"
				result.Models[i].Error = "model not found"
				benchMu.Unlock()
				continue
			}

			prov, ok := cfg.Providers[me.Provider]
			if !ok {
				benchMu.Lock()
				result.Models[i].Status = "error"
				result.Models[i].Error = "provider not configured"
				benchMu.Unlock()
				continue
			}

			def := config.LLMProviderDef{
				Provider:     me.Provider,
				APIKey:       prov.APIKey,
				Model:        me.Model,
				APIEndpoint:  prov.APIEndpoint,
				RefreshToken: prov.RefreshToken,
				KeepAlive:    prov.KeepAlive,
			}
			if me.MaxTokens > 0 {
				def.MaxTokens = me.MaxTokens
			} else {
				def.MaxTokens = cfg.MaxTokens
			}

			client, err := llm.NewClientFromDef(def)
			if err != nil {
				slog.Error("[benchmark] client creation failed", "label", label, "error", err)
				benchMu.Lock()
				result.Models[i].Status = "error"
				result.Models[i].Error = err.Error()
				benchMu.Unlock()
				continue
			}

			timeout := 30 * time.Second
			if me.TimeoutMs > 0 {
				timeout = time.Duration(me.TimeoutMs) * time.Millisecond
			}

			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			start := time.Now()
			resp, err := client.Send(ctx, llm.Request{
				Prompt: promptText,
				Text:   benchmarkTestText,
			})
			elapsed := time.Since(start)
			cancel()
			client.Close()

			benchMu.Lock()
			result.Models[i].DurationMs = elapsed.Milliseconds()
			var status, errMsg, output string
			if err != nil {
				if ctx.Err() == context.DeadlineExceeded {
					result.Models[i].Status = "timeout"
					result.Models[i].Error = fmt.Sprintf("timed out after %ds", int(timeout.Seconds()))
					status = "timeout"
					errMsg = result.Models[i].Error
				} else {
					result.Models[i].Status = "error"
					result.Models[i].Error = err.Error()
					status = "error"
					errMsg = err.Error()
				}
			} else {
				result.Models[i].Status = "success"
				result.Models[i].Output = resp.Text
				status = "success"
				output = resp.Text
			}
			benchMu.Unlock()

			// Record benchmark result into usage stats.
			if s.RecordStatFn != nil {
				s.RecordStatFn(
					"Benchmark", "\U0001F3AF", // 🎯
					me.Provider, me.Model, label,
					status, errMsg, output,
					len(benchmarkTestText), int(elapsed.Milliseconds()),
				)
			}
		}

		benchMu.Lock()
		result.Running = false
		result.Done = true
		benchMu.Unlock()
		slog.Info("[benchmark] complete")
	}()

	return "ok"
}

// GetBenchmarkResult returns the current benchmark result as JSON.
func (s *SettingsService) GetBenchmarkResult() string {
	benchMu.Lock()
	defer benchMu.Unlock()
	if benchResult == nil {
		return "{}"
	}
	data, _ := json.Marshal(benchResult)
	return string(data)
}
