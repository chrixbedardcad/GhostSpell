//go:build linux

package gui

import (
	"fmt"
	"net/http"
	"os/exec"
	"time"
)

// ollamaDownloadInstallerPlatform installs Ollama on Linux via the official install script.
func ollamaDownloadInstallerPlatform() error {
	cmd := exec.Command("bash", "-c", "curl -fsSL https://ollama.com/install.sh | sh")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("install failed: %s", string(out))
	}
	return nil
}

// ollamaStartServePlatform starts "ollama serve" as a background process on Linux
// and polls until the server is reachable.
func ollamaStartServePlatform() error {
	cmd := exec.Command("ollama", "serve")
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
