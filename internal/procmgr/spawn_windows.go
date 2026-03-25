//go:build windows

package procmgr

import (
	"log/slog"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// setupProcessGroup configures the child to run in a new process group on Windows.
func setupProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

// assignToJob assigns the child process to the given Job Object.
func assignToJob(job *JobObject, cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	handle, err := windows.OpenProcess(windows.PROCESS_ALL_ACCESS, false, uint32(cmd.Process.Pid))
	if err != nil {
		slog.Warn("[procmgr] OpenProcess failed", "pid", cmd.Process.Pid, "error", err)
		return
	}
	if err := job.Assign(uintptr(handle)); err != nil {
		slog.Warn("[procmgr] job assign failed", "pid", cmd.Process.Pid, "error", err)
	}
	windows.CloseHandle(handle)
}
