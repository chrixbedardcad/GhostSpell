package llm

import (
	"os/exec"
	"syscall"
)

func hideGhostAIConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
