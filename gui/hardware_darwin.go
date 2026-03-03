//go:build darwin

package gui

import (
	"os/exec"
	"strconv"
	"strings"
)

// detectSystemCapacity probes RAM (via sysctl) and VRAM (via nvidia-smi) on macOS.
func detectSystemCapacity() SystemCapacity {
	var cap SystemCapacity

	// --- RAM via sysctl ---
	out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
	if err == nil {
		if bytes, err := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64); err == nil {
			cap.TotalRAMGB = float64(bytes) / (1024 * 1024 * 1024)
		}
	}

	// --- VRAM via nvidia-smi (unlikely on macOS but handle eGPU case) ---
	out, err = exec.Command(
		"nvidia-smi",
		"--query-gpu=memory.total",
		"--format=csv,noheader,nounits",
	).Output()
	if err == nil {
		line := strings.TrimSpace(string(out))
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
