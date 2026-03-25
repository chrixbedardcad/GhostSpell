//go:build windows

package procmgr

import (
	"os/exec"
	"strconv"
	"strings"
)

// isProcessAlive checks if a process with the given PID exists on Windows.
func isProcessAlive(pid int) bool {
	cmd := exec.Command("tasklist", "/FI", "PID eq "+strconv.Itoa(pid), "/NH")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(out) > 0 && !strings.Contains(string(out), "No tasks")
}
