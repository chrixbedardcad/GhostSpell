//go:build !windows

package gui

import "fmt"

// ollamaDownloadInstallerWindows is a stub on non-Windows platforms.
func ollamaDownloadInstallerWindows() error {
	return fmt.Errorf("Ollama installer download is only available on Windows")
}

// ollamaStartServe is a stub on non-Windows platforms.
func ollamaStartServe() error {
	return fmt.Errorf("ollama serve management is only available on Windows")
}
