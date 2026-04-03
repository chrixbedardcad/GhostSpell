// Package history stores a full log of user actions with input/output text.
// Separate from stats (which only tracks metadata for aggregation).
// Stored in history.json in the app config directory.
package history

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const maxEntries = 500

// Entry represents one user action with full text.
type Entry struct {
	Timestamp  time.Time `json:"ts"`
	Prompt     string    `json:"prompt"`
	PromptIcon string    `json:"icon,omitempty"`
	Provider   string    `json:"provider"`
	Model      string    `json:"model"`
	ModelLabel string    `json:"label"`
	Input      string    `json:"input"`
	Output     string    `json:"output"`
	InputLen   int       `json:"input_len"`
	OutputLen  int       `json:"output_len"`
	DurationMs int64     `json:"duration_ms"`
	Status     string    `json:"status"` // success, error, cancelled, timeout, identical
	Error      string    `json:"error,omitempty"`
}

// History manages the action log.
type History struct {
	mu      sync.Mutex
	entries []Entry
	path    string
}

// New creates a History instance, loading existing data from disk.
func New(dir string) *History {
	h := &History{
		path: filepath.Join(dir, "history.json"),
	}
	h.load()
	return h
}

// Record adds an entry and saves asynchronously.
func (h *History) Record(e Entry) {
	h.mu.Lock()
	h.entries = append(h.entries, e)
	if len(h.entries) > maxEntries {
		h.entries = h.entries[len(h.entries)-maxEntries:]
	}
	entries := make([]Entry, len(h.entries))
	copy(entries, h.entries)
	h.mu.Unlock()

	go func() {
		data, err := json.Marshal(entries)
		if err != nil {
			slog.Error("[history] marshal failed", "error", err)
			return
		}
		if err := os.WriteFile(h.path, data, 0600); err != nil {
			slog.Error("[history] write failed", "error", err)
		}
	}()
}

// GetRecent returns the last n entries as JSON (most recent first).
func (h *History) GetRecent(n int) string {
	h.mu.Lock()
	defer h.mu.Unlock()

	count := len(h.entries)
	if n > count {
		n = count
	}

	// Return most recent first.
	recent := make([]Entry, n)
	for i := 0; i < n; i++ {
		recent[i] = h.entries[count-1-i]
	}

	data, err := json.Marshal(recent)
	if err != nil {
		return "[]"
	}
	return string(data)
}

// Clear removes all history entries.
func (h *History) Clear() {
	h.mu.Lock()
	h.entries = nil
	h.mu.Unlock()
	os.Remove(h.path)
}

func (h *History) load() {
	data, err := os.ReadFile(h.path)
	if err != nil {
		return
	}
	var entries []Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		slog.Warn("[history] failed to parse history.json", "error", err)
		return
	}
	h.entries = entries
}
