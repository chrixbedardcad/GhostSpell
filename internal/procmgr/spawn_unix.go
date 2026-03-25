//go:build !windows

package procmgr

import "os/exec"

// setupProcessGroup configures the child to run in its own process group on Unix.
func setupProcessGroup(cmd *exec.Cmd) {
	SetProcessGroup(cmd)
}

// assignToJob is a no-op on Unix — process groups handle cleanup instead.
func assignToJob(job *JobObject, cmd *exec.Cmd) {}
