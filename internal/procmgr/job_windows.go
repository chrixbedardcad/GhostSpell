//go:build windows

package procmgr

import (
	"fmt"
	"log/slog"
	"unsafe"

	"golang.org/x/sys/windows"
)

// JobObject wraps a Windows Job Object configured with KILL_ON_JOB_CLOSE.
// When the Job Object handle is closed (including parent crash), all assigned
// child processes are terminated by the OS kernel.
type JobObject struct {
	handle windows.Handle
}

// NewJobObject creates a Job Object with JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE.
func NewJobObject() (*JobObject, error) {
	handle, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("procmgr: CreateJobObject: %w", err)
	}

	// Set KILL_ON_JOB_CLOSE — when the last handle to this job is closed,
	// Windows terminates all processes assigned to it.
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE

	_, err = windows.SetInformationJobObject(
		handle,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	)
	if err != nil {
		windows.CloseHandle(handle)
		return nil, fmt.Errorf("procmgr: SetInformationJobObject: %w", err)
	}

	slog.Info("[procmgr] Job Object created", "handle", handle)
	return &JobObject{handle: handle}, nil
}

// Assign adds a process to this Job Object by its OS process handle.
func (j *JobObject) Assign(processHandle uintptr) error {
	if j == nil || j.handle == 0 {
		return nil
	}
	err := windows.AssignProcessToJobObject(j.handle, windows.Handle(processHandle))
	if err != nil {
		return fmt.Errorf("procmgr: AssignProcessToJobObject: %w", err)
	}
	return nil
}

// Close releases the Job Object handle. If KILL_ON_JOB_CLOSE is set and this
// is the last handle, all assigned processes are terminated.
func (j *JobObject) Close() {
	if j == nil || j.handle == 0 {
		return
	}
	windows.CloseHandle(j.handle)
	j.handle = 0
	slog.Info("[procmgr] Job Object closed")
}
