package procmgr

import (
	"log/slog"
	"os"
	"time"
)

// WatchParent starts a goroutine that checks if the parent process (by PID)
// is still alive every interval. If the parent is gone, onOrphan is called.
// Used by child processes (ghostai, ghostvoice) to self-exit when the parent dies.
func WatchParent(parentPID int, onOrphan func()) {
	if parentPID <= 0 {
		return
	}
	go func() {
		slog.Info("[procmgr] watching parent", "pid", parentPID)
		for {
			time.Sleep(2 * time.Second)
			if !IsAlive(parentPID) {
				slog.Warn("[procmgr] parent process gone — shutting down", "parent_pid", parentPID)
				onOrphan()
				return
			}
		}
	}()
}

// IsAlive checks if a process with the given PID exists.
// This is a cross-platform wrapper; see isAlive_*.go for implementations.
func IsAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return isProcessAlive(pid)
}

// SelfPID returns the current process ID.
func SelfPID() int {
	return os.Getpid()
}
