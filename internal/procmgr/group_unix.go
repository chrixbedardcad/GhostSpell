//go:build !windows

package procmgr

import (
	"log/slog"
	"os/exec"
	"syscall"
)

// JobObject is a no-op on Unix. Process group cleanup is handled via
// SetProcessGroup + KillProcessGroup instead.
type JobObject struct{}

// NewJobObject returns a stub on Unix (process groups are set per-child).
func NewJobObject() (*JobObject, error) {
	return &JobObject{}, nil
}

// Assign is a no-op on Unix — process group is set at spawn time.
func (j *JobObject) Assign(processHandle uintptr) error {
	return nil
}

// Close is a no-op on Unix.
func (j *JobObject) Close() {}

// SetProcessGroup configures cmd to run in its own process group.
// This allows KillProcessGroup to terminate it and all its children.
func SetProcessGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

// KillProcessGroup sends SIGTERM to the entire process group led by pid.
func KillProcessGroup(pid int) error {
	err := syscall.Kill(-pid, syscall.SIGTERM)
	if err != nil {
		slog.Warn("[procmgr] kill process group failed", "pgid", pid, "error", err)
	}
	return err
}
