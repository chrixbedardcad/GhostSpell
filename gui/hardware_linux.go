//go:build linux

package gui

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// detectSystemCapacity probes RAM (via /proc/meminfo) and VRAM (via nvidia-smi) on Linux.
func detectSystemCapacity() SystemCapacity {
	var cap SystemCapacity

	// --- RAM via /proc/meminfo ---
	data, err := os.ReadFile("/proc/meminfo")
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "MemTotal:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					if kb, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
						cap.TotalRAMGB = float64(kb) / (1024 * 1024)
					}
				}
				break
			}
		}
	}

	// --- VRAM via nvidia-smi ---
	out, err := exec.Command(
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
