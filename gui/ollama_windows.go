//go:build windows

package gui

import (
	"os/exec"
	"syscall"
)

// ollamaDownloadInstallerPlatform opens the Ollama download page in the default browser.
func ollamaDownloadInstallerPlatform() error {
	cmd := exec.Command("cmd", "/c", "start", "", "https://ollama.com/download")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Start()
}

// ollamaOpenTerminalPull opens a cmd window running "ollama pull <model>".
func ollamaOpenTerminalPull(model string) error {
	// /k keeps the window open after the command finishes so the user can see the result.
	return exec.Command("cmd", "/c", "start", "cmd", "/k", "ollama pull "+model).Start()
}
