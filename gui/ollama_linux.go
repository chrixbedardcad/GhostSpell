//go:build linux

package gui

import "os/exec"

// ollamaDownloadInstallerPlatform opens the Ollama download page in the default browser.
func ollamaDownloadInstallerPlatform() error {
	return exec.Command("xdg-open", "https://ollama.com/download").Start()
}

// ollamaOpenTerminalPull opens a terminal window running "ollama pull <model>".
func ollamaOpenTerminalPull(model string) error {
	// Try common terminal emulators in order of preference.
	script := "ollama pull " + model + "; echo; echo 'Done! You can close this window.'; read"
	terminals := []struct{ bin string; args []string }{
		{"gnome-terminal", []string{"--", "bash", "-c", script}},
		{"konsole", []string{"-e", "bash", "-c", script}},
		{"xfce4-terminal", []string{"-e", "bash -c '" + script + "'"}},
		{"x-terminal-emulator", []string{"-e", "bash", "-c", script}},
		{"xterm", []string{"-e", "bash", "-c", script}},
	}
	for _, t := range terminals {
		if _, err := exec.LookPath(t.bin); err == nil {
			return exec.Command(t.bin, t.args...).Start()
		}
	}
	// Fallback: just run in background (no visible terminal).
	return exec.Command("bash", "-c", "ollama pull "+model).Start()
}
