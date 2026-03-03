//go:build windows

package gui

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// ollamaDownloadInstallerPlatform downloads OllamaSetup.exe to %TEMP% and launches it.
func ollamaDownloadInstallerPlatform() error {
	const url = "https://ollama.com/download/OllamaSetup.exe"

	tmpDir := os.TempDir()
	dest := filepath.Join(tmpDir, "OllamaSetup.exe")

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned HTTP %d", resp.StatusCode)
	}

	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return fmt.Errorf("write file: %w", err)
	}
	f.Close()

	// Launch the installer via cmd /c start so the user sees the standard setup wizard.
	cmd := exec.Command("cmd", "/c", "start", "", dest)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch installer: %w", err)
	}
	return nil
}

// ollamaStartServePlatform starts "ollama serve" as a detached background process
// and polls GET / for up to 10 seconds until the server is reachable.
func ollamaStartServePlatform() error {
	cmd := exec.Command("ollama", "serve")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x00000008 | 0x00000010, // DETACHED_PROCESS | CREATE_NEW_CONSOLE
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start ollama serve: %w", err)
	}

	// Poll until the server responds.
	client := &http.Client{Timeout: 1 * time.Second}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		resp, err := client.Get("http://localhost:11434/")
		if err == nil {
			resp.Body.Close()
			return nil
		}
	}
	return fmt.Errorf("ollama serve started but server not reachable after 10s")
}
