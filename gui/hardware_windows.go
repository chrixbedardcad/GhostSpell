//go:build windows

package gui

import (
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

// memoryStatusEx maps to the Windows MEMORYSTATUSEX structure.
type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

var (
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	procGlobalMemoryStatus = kernel32.NewProc("GlobalMemoryStatusEx")
)

// detectSystemCapacity probes RAM (via Win32) and VRAM (via nvidia-smi).
func detectSystemCapacity() SystemCapacity {
	var cap SystemCapacity

	// --- RAM via GlobalMemoryStatusEx ---
	var ms memoryStatusEx
	ms.Length = uint32(unsafe.Sizeof(ms))
	ret, _, _ := procGlobalMemoryStatus.Call(uintptr(unsafe.Pointer(&ms)))
	if ret != 0 {
		cap.TotalRAMGB = float64(ms.TotalPhys) / (1024 * 1024 * 1024)
	}

	// --- VRAM via nvidia-smi ---
	out, err := exec.Command(
		"nvidia-smi",
		"--query-gpu=memory.total",
		"--format=csv,noheader,nounits",
	).Output()
	if err == nil {
		line := strings.TrimSpace(string(out))
		// nvidia-smi may list multiple GPUs; take the first line.
		if idx := strings.Index(line, "\n"); idx > 0 {
			line = line[:idx]
		}
		line = strings.TrimSpace(line)
		if mib, err := strconv.ParseFloat(line, 64); err == nil && mib > 0 {
			cap.HasNVIDIA = true
			cap.NVIDIAVRAMGB = mib / 1024
		}
	}

	return cap
}
