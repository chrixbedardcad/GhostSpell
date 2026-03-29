//go:build !windows

package llm

import "os/exec"

func hideGhostAIConsole(_ *exec.Cmd) {}
