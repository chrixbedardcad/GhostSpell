//go:build !windows

package procmgr

import (
	"os"
	"syscall"
)

// isProcessAlive checks if a process with the given PID exists on Unix.
// FindProcess always succeeds on Unix — signal 0 probes without killing.
func isProcessAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}
