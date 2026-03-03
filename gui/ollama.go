package gui

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// ollamaBaseURL normalises an endpoint string into a base URL.
// Defaults to http://localhost:11434 when empty.
func ollamaBaseURL(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "http://localhost:11434"
	}
	return strings.TrimRight(endpoint, "/")
}

// ollamaProbeRunning checks if the Ollama server is reachable at the given base URL.
// Returns (running, versionString).
func ollamaProbeRunning(base string) (bool, string) {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(base + "/")
	if err != nil {
		return false, ""
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
	return true, strings.TrimSpace(string(body))
}

// ollamaCheckInstalled returns true if the "ollama" binary is on PATH.
func ollamaCheckInstalled() bool {
	_, err := exec.LookPath("ollama")
	return err == nil
}

// ollamaGetStatus combines probe + install check.
// Returns a map with "status" => "running" | "installed" | "not_installed".
func ollamaGetStatus(endpoint string) map[string]string {
	base := ollamaBaseURL(endpoint)
	running, version := ollamaProbeRunning(base)
	if running {
		return map[string]string{"status": "running", "version": version}
	}
	if ollamaCheckInstalled() {
		return map[string]string{"status": "installed"}
	}
	return map[string]string{"status": "not_installed"}
}

// ollamaModelInfo holds metadata about a locally-installed model.
type ollamaModelInfo struct {
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	ParamSize  string `json:"parameter_size"`
	Quant      string `json:"quantization_level"`
	SizeHuman  string `json:"size_human"`
}

// ollamaFetchModels retrieves the list of locally-installed models.
func ollamaFetchModels(base string) ([]ollamaModelInfo, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(base + "/api/tags")
	if err != nil {
		return nil, fmt.Errorf("cannot reach Ollama: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama /api/tags returned %d", resp.StatusCode)
	}

	var payload struct {
		Models []struct {
			Name    string `json:"name"`
			Size    int64  `json:"size"`
			Details struct {
				ParameterSize      string `json:"parameter_size"`
				QuantizationLevel  string `json:"quantization_level"`
			} `json:"details"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode error: %w", err)
	}

	out := make([]ollamaModelInfo, len(payload.Models))
	for i, m := range payload.Models {
		out[i] = ollamaModelInfo{
			Name:      m.Name,
			Size:      m.Size,
			ParamSize: m.Details.ParameterSize,
			Quant:     m.Details.QuantizationLevel,
			SizeHuman: formatBytes(m.Size),
		}
	}
	return out, nil
}

// pullProgress tracks the state of an active model pull.
type pullProgress struct {
	mu        sync.Mutex
	Active    bool              `json:"active"`
	cancel    context.CancelFunc `json:"-"`
	Status    string            `json:"status"`
	Pct       float64           `json:"pct"`
	Completed int64             `json:"completed"`
	Total     int64             `json:"total"`
	Done      bool              `json:"done"`
	Error     string            `json:"error,omitempty"`
}

// activePull is the singleton tracking the current pull operation.
var activePull pullProgress

// ollamaStartPull kicks off an async model pull. Only one pull at a time.
func ollamaStartPull(base, model string) {
	activePull.mu.Lock()
	if activePull.Active {
		activePull.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	activePull = pullProgress{Active: true, cancel: cancel, Status: "starting"}
	activePull.mu.Unlock()

	go func() {
		defer func() {
			activePull.mu.Lock()
			activePull.Active = false
			activePull.Done = true
			activePull.cancel = nil
			activePull.mu.Unlock()
		}()

		body, _ := json.Marshal(map[string]interface{}{
			"name":   model,
			"stream": true,
		})
		req, err := http.NewRequestWithContext(ctx, "POST", base+"/api/pull", strings.NewReader(string(body)))
		if err != nil {
			activePull.mu.Lock()
			activePull.Error = err.Error()
			activePull.mu.Unlock()
			return
		}
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			activePull.mu.Lock()
			activePull.Error = err.Error()
			activePull.mu.Unlock()
			return
		}
		defer resp.Body.Close()

		decoder := json.NewDecoder(resp.Body)
		for {
			var line struct {
				Status    string `json:"status"`
				Completed int64  `json:"completed"`
				Total     int64  `json:"total"`
				Error     string `json:"error"`
			}
			if err := decoder.Decode(&line); err != nil {
				if err == io.EOF || ctx.Err() != nil {
					break
				}
				activePull.mu.Lock()
				activePull.Error = err.Error()
				activePull.mu.Unlock()
				return
			}

			activePull.mu.Lock()
			activePull.Status = line.Status
			activePull.Completed = line.Completed
			activePull.Total = line.Total
			if line.Total > 0 {
				activePull.Pct = float64(line.Completed) / float64(line.Total) * 100
			}
			if line.Error != "" {
				activePull.Error = line.Error
			}
			activePull.mu.Unlock()
		}
	}()
}

// ollamaCancelPull cancels the active pull if any.
func ollamaCancelPull() {
	activePull.mu.Lock()
	defer activePull.mu.Unlock()
	if activePull.cancel != nil {
		activePull.cancel()
	}
}

// ollamaGetPullProgress returns a JSON snapshot of current pull state.
func ollamaGetPullProgress() string {
	activePull.mu.Lock()
	defer activePull.mu.Unlock()
	data, _ := json.Marshal(map[string]interface{}{
		"active":    activePull.Active,
		"status":    activePull.Status,
		"pct":       activePull.Pct,
		"completed": activePull.Completed,
		"total":     activePull.Total,
		"done":      activePull.Done,
		"error":     activePull.Error,
	})
	return string(data)
}

// ollamaDeleteModelAPI deletes a model via the Ollama API.
func ollamaDeleteModelAPI(base, model string) error {
	body, _ := json.Marshal(map[string]string{"name": model})
	req, err := http.NewRequest("DELETE", base+"/api/delete", strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("delete returned %d", resp.StatusCode)
	}
	return nil
}

// ollamaRecommendModel picks the best Ollama model for the detected hardware.
// Returns (model, reason).
func ollamaRecommendModel(cap SystemCapacity) (string, string) {
	vram := cap.NVIDIAVRAMGB
	ram := cap.TotalRAMGB

	switch {
	case vram >= 24:
		return "qwen2.5:14b", "High quality — fits in your " + formatGB(vram) + " VRAM"
	case vram >= 8 || ram >= 16:
		return "mistral:7b", "Best quality/speed balance for your system"
	case vram >= 4 || ram >= 8:
		return "llama3.2:3b", "Fast and compact — good for 8 GB systems"
	default:
		return "phi3", "Smallest usable model — works on CPU"
	}
}

// formatGB formats a float GB value for display.
func formatGB(gb float64) string {
	if gb == float64(int(gb)) {
		return fmt.Sprintf("%d GB", int(gb))
	}
	return fmt.Sprintf("%.1f GB", gb)
}

// formatBytes converts bytes to a human-readable string.
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
