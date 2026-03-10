//go:build darwin

package gui

import "os/exec"

// ollamaDownloadInstallerPlatform opens the Ollama download page in the default browser.
func ollamaDownloadInstallerPlatform() error {
	return exec.Command("open", "https://ollama.com/download").Start()
}

// ollamaOpenTerminalPull opens Terminal.app running "ollama pull <model>".
func ollamaOpenTerminalPull(model string) error {
	script := `tell application "Terminal"
		activate
		do script "ollama pull ` + model + `"
	end tell`
	return exec.Command("osascript", "-e", script).Start()
}
