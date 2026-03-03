//go:build darwin

package clipboard

import (
	"fmt"
	"os/exec"
	"strings"
)

// NewDarwinClipboard creates a Clipboard using macOS pbcopy/pbpaste.
func NewDarwinClipboard() *Clipboard {
	return New(darwinRead, darwinWrite).WithClear(darwinClear)
}

func darwinRead() (string, error) {
	out, err := exec.Command("pbpaste").Output()
	if err != nil {
		return "", fmt.Errorf("pbpaste: %w", err)
	}
	return strings.TrimRight(string(out), "\n"), nil
}

func darwinWrite(text string) error {
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(text)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pbcopy: %w", err)
	}
	return nil
}

func darwinClear() error {
	return darwinWrite("")
}
