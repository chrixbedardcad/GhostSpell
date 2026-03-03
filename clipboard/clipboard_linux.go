//go:build linux

package clipboard

import (
	"fmt"
	"os/exec"
	"strings"
)

// NewLinuxClipboard creates a Clipboard using xclip (or xsel as fallback).
func NewLinuxClipboard() *Clipboard {
	return New(linuxRead, linuxWrite).WithClear(linuxClear)
}

func linuxRead() (string, error) {
	// Try xclip first, fall back to xsel.
	if path, err := exec.LookPath("xclip"); err == nil {
		out, err := exec.Command(path, "-selection", "clipboard", "-o").Output()
		if err != nil {
			return "", fmt.Errorf("xclip read: %w", err)
		}
		return string(out), nil
	}
	if path, err := exec.LookPath("xsel"); err == nil {
		out, err := exec.Command(path, "--clipboard", "--output").Output()
		if err != nil {
			return "", fmt.Errorf("xsel read: %w", err)
		}
		return string(out), nil
	}
	return "", fmt.Errorf("clipboard read: xclip or xsel not found (install: apt install xclip)")
}

func linuxWrite(text string) error {
	if path, err := exec.LookPath("xclip"); err == nil {
		cmd := exec.Command(path, "-selection", "clipboard")
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("xclip write: %w", err)
		}
		return nil
	}
	if path, err := exec.LookPath("xsel"); err == nil {
		cmd := exec.Command(path, "--clipboard", "--input")
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("xsel write: %w", err)
		}
		return nil
	}
	return fmt.Errorf("clipboard write: xclip or xsel not found (install: apt install xclip)")
}

func linuxClear() error {
	return linuxWrite("")
}
