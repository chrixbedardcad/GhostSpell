// Package stats tracks usage metrics for GhostSpell invocations.
// Stores data in a simple JSON file, capped at maxEntries to limit disk usage.
// No actual text content is stored — only lengths, times, and counts.
package stats

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const maxEntries = 1000

// Entry represents one Ctrl+G invocation.
type Entry struct {
	Timestamp        time.Time `json:"ts"`
	Prompt           string    `json:"prompt"`
	PromptIcon       string    `json:"icon,omitempty"`
	Provider         string    `json:"provider"`
	Model            string    `json:"model"`
	ModelLabel       string    `json:"label"`
	InputChars       int       `json:"input_chars"`
	InputWords       int       `json:"input_words"`
	OutputChars      int       `json:"output_chars"`
	OutputWords      int       `json:"output_words"`
	DurationMs       int64     `json:"duration_ms"`
	PromptTokens     int       `json:"prompt_tokens,omitempty"`
	CompletionTokens int       `json:"completion_tokens,omitempty"`
	TokensPerSec     float64   `json:"tps,omitempty"`
	Status           string    `json:"status"` // success, error, cancelled, timeout, identical
	Error            string    `json:"error,omitempty"`
	Changed          bool      `json:"changed"`
}

// Summary provides aggregated stats for the dashboard.
type Summary struct {
	TotalRequests  int            `json:"total_requests"`
	SuccessCount   int            `json:"success_count"`
	ErrorCount     int            `json:"error_count"`
	AvgDurationMs  int64          `json:"avg_duration_ms"`
	MostUsedPrompt string         `json:"most_used_prompt"`
	MostUsedModel  string         `json:"most_used_model"`
	Models         []ModelStats   `json:"models"`
	Prompts        []PromptStats  `json:"prompts"`
	RecentEntries  []Entry        `json:"recent,omitempty"`
}

// ModelStats aggregates stats per model.
type ModelStats struct {
	Label       string  `json:"label"`
	Provider    string  `json:"provider"`
	Model       string  `json:"model"`
	Requests    int     `json:"requests"`
	AvgDuration int64   `json:"avg_duration_ms"`
	AvgInput    int     `json:"avg_input"`
	AvgOutput   int     `json:"avg_output"`
	SuccessRate float64 `json:"success_rate"`
}

// PromptStats aggregates stats per prompt.
type PromptStats struct {
	Name     string `json:"name"`
	Icon     string `json:"icon"`
	Requests int    `json:"requests"`
}

// Stats manages usage tracking.
type Stats struct {
	mu      sync.Mutex
	entries []Entry
	path    string
}

// New creates a Stats instance, loading existing data from disk.
func New(dir string) *Stats {
	s := &Stats{
		path: filepath.Join(dir, "stats.json"),
	}
	s.load()
	return s
}

// Record adds an entry and saves asynchronously.
func (s *Stats) Record(e Entry) {
	s.mu.Lock()
	s.entries = append(s.entries, e)
	// Cap at maxEntries.
	if len(s.entries) > maxEntries {
		s.entries = s.entries[len(s.entries)-maxEntries:]
	}
	entries := make([]Entry, len(s.entries))
	copy(entries, s.entries)
	s.mu.Unlock()

	// Save asynchronously — don't block the caller.
	go func() {
		data, err := json.Marshal(entries)
		if err != nil {
			slog.Error("[stats] marshal failed", "error", err)
			return
		}
		if err := os.WriteFile(s.path, data, 0644); err != nil {
			slog.Error("[stats] write failed", "error", err)
		}
	}()
}

// GetSummary returns aggregated stats as JSON.
func (s *Stats) GetSummary() string {
	s.mu.Lock()
	entries := make([]Entry, len(s.entries))
	copy(entries, s.entries)
	s.mu.Unlock()

	summary := buildSummary(entries)

	// Include last 20 recent entries.
	n := len(entries)
	if n > 20 {
		summary.RecentEntries = entries[n-20:]
	} else {
		summary.RecentEntries = entries
	}

	data, _ := json.Marshal(summary)
	return string(data)
}

// Clear removes all stats.
func (s *Stats) Clear() {
	s.mu.Lock()
	s.entries = nil
	s.mu.Unlock()
	os.Remove(s.path)
}

func (s *Stats) load() {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return // no existing stats
	}
	var entries []Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		slog.Warn("[stats] failed to parse stats file", "error", err)
		return
	}
	s.entries = entries
	slog.Info("[stats] loaded", "entries", len(entries))
}

func buildSummary(entries []Entry) Summary {
	s := Summary{}
	s.TotalRequests = len(entries)

	if len(entries) == 0 {
		return s
	}

	var totalDuration int64
	promptCounts := make(map[string]int)
	modelCounts := make(map[string]int)
	modelData := make(map[string]*modelAgg)

	for _, e := range entries {
		if e.Status == "success" || e.Status == "identical" {
			s.SuccessCount++
		} else {
			s.ErrorCount++
		}
		totalDuration += e.DurationMs
		promptCounts[e.Prompt]++
		modelCounts[e.ModelLabel]++

		key := e.ModelLabel
		if _, ok := modelData[key]; !ok {
			modelData[key] = &modelAgg{provider: e.Provider, model: e.Model}
		}
		md := modelData[key]
		md.requests++
		md.totalDuration += e.DurationMs
		md.totalInput += e.InputChars
		md.totalOutput += e.OutputChars
		if e.Status == "success" || e.Status == "identical" {
			md.successes++
		}
	}

	s.AvgDurationMs = totalDuration / int64(len(entries))

	// Most used prompt.
	maxP := 0
	for name, count := range promptCounts {
		if count > maxP {
			maxP = count
			s.MostUsedPrompt = name
		}
	}

	// Most used model.
	maxM := 0
	for label, count := range modelCounts {
		if count > maxM {
			maxM = count
			s.MostUsedModel = label
		}
	}

	// Model stats.
	for label, md := range modelData {
		rate := 0.0
		if md.requests > 0 {
			rate = float64(md.successes) / float64(md.requests) * 100
		}
		s.Models = append(s.Models, ModelStats{
			Label:       label,
			Provider:    md.provider,
			Model:       md.model,
			Requests:    md.requests,
			AvgDuration: md.totalDuration / int64(md.requests),
			AvgInput:    md.totalInput / md.requests,
			AvgOutput:   md.totalOutput / md.requests,
			SuccessRate: rate,
		})
	}

	// Prompt stats.
	for name, count := range promptCounts {
		icon := ""
		for _, e := range entries {
			if e.Prompt == name && e.PromptIcon != "" {
				icon = e.PromptIcon
				break
			}
		}
		s.Prompts = append(s.Prompts, PromptStats{Name: name, Icon: icon, Requests: count})
	}

	return s
}

type modelAgg struct {
	provider      string
	model         string
	requests      int
	successes     int
	totalDuration int64
	totalInput    int
	totalOutput   int
}
