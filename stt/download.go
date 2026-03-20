package stt

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/chrixbedardcad/GhostSpell/llm"
)

// VoiceModelsDir returns the directory for voice models (same as LLM models).
func VoiceModelsDir() (string, error) {
	return llm.LocalModelsDir()
}

// DownloadVoiceModel downloads a whisper model by name.
func DownloadVoiceModel(name string, progressCb func(llm.DownloadProgress)) error {
	model := findVoiceModel(name)
	if model == nil {
		return fmt.Errorf("unknown voice model: %s", name)
	}

	modelsDir, err := VoiceModelsDir()
	if err != nil {
		return fmt.Errorf("models dir: %w", err)
	}

	destPath := filepath.Join(modelsDir, model.FileName)

	// Check if already downloaded.
	if info, err := os.Stat(destPath); err == nil && info.Size() > 0 {
		slog.Info("[ghost-voice] model already downloaded", "name", name, "size", info.Size())
		return nil
	}

	slog.Info("[ghost-voice] downloading model", "name", name, "url", model.URL, "dest", destPath)

	client := &http.Client{Timeout: 30 * time.Minute}
	resp, err := client.Get(model.URL)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download HTTP %d", resp.StatusCode)
	}

	tmpPath := destPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}

	total := resp.ContentLength
	var downloaded int64
	buf := make([]byte, 256*1024)

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := f.Write(buf[:n]); err != nil {
				f.Close()
				os.Remove(tmpPath)
				return fmt.Errorf("write: %w", err)
			}
			downloaded += int64(n)
			if progressCb != nil && total > 0 {
				progressCb(llm.DownloadProgress{
					Downloaded: downloaded,
					Total:      total,
					Percent:    float64(downloaded) / float64(total) * 100,
				})
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			f.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("read: %w", readErr)
		}
	}
	f.Close()

	if err := os.Rename(tmpPath, destPath); err != nil {
		return fmt.Errorf("rename: %w", err)
	}

	slog.Info("[ghost-voice] model downloaded", "name", name, "size", downloaded)
	return nil
}

// DeleteVoiceModel deletes a downloaded voice model.
func DeleteVoiceModel(name string) error {
	model := findVoiceModel(name)
	if model == nil {
		return fmt.Errorf("unknown voice model: %s", name)
	}

	modelsDir, err := VoiceModelsDir()
	if err != nil {
		return err
	}

	path := filepath.Join(modelsDir, model.FileName)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	slog.Info("[ghost-voice] model deleted", "name", name)
	return nil
}
