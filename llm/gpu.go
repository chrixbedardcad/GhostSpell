package llm

import (
	"os/exec"
	"runtime"
	"strings"
)

// GPUType identifies the GPU acceleration backend.
type GPUType string

const (
	GPUNone   GPUType = "none"
	GPUCUDA   GPUType = "cuda"
	GPUMetal  GPUType = "metal"
	GPUVulkan GPUType = "vulkan"
)

// GPUInfo describes the detected GPU and available acceleration.
type GPUInfo struct {
	Type      GPUType `json:"type"`      // cuda, metal, vulkan, none
	Name      string  `json:"name"`      // human-readable GPU name (e.g. "NVIDIA RTX 3060")
	Available bool    `json:"available"` // true if the GPU can be used for acceleration
	Reason    string  `json:"reason"`    // why acceleration is unavailable (if !Available)
	Installed bool    `json:"installed"` // true if GPU acceleration is active in current binary
}

// DetectGPU detects the system GPU and returns acceleration info.
// GPU acceleration is a build-time feature: _build.bat with CUDA produces a
// CUDA-enabled ghostai.exe. CI builds are CPU-only. The toggle in Settings
// controls whether GPU layers are requested at runtime.
func DetectGPU() GPUInfo {
	switch runtime.GOOS {
	case "darwin":
		return detectGPUDarwin()
	case "windows":
		return detectGPUWindows()
	case "linux":
		return detectGPULinux()
	default:
		return GPUInfo{Type: GPUNone, Reason: "unsupported platform"}
	}
}

func detectGPUDarwin() GPUInfo {
	if isIntelMac() {
		return GPUInfo{
			Type:   GPUMetal,
			Name:   "Intel Mac (Metal unsupported)",
			Reason: "Metal acceleration is unreliable on Intel Macs with discrete GPUs",
		}
	}
	if isMacOS13() {
		return GPUInfo{
			Type:   GPUMetal,
			Name:   "Apple Silicon (macOS 13)",
			Reason: "Metal has page-alignment bugs on macOS 13 — update to macOS 14+",
		}
	}

	// Apple Silicon with macOS 14+ — Metal works great.
	return GPUInfo{
		Type:      GPUMetal,
		Name:      "Apple Silicon",
		Available: true,
		Installed: true,
	}
}

func detectGPUWindows() GPUInfo {
	if name := detectNVIDIA(); name != "" {
		return GPUInfo{
			Type:      GPUCUDA,
			Name:      name,
			Available: true,
			Installed: true, // If GPU detected, ghostai auto-uses it (gpu_layers > 0)
		}
	}
	return GPUInfo{
		Type:   GPUNone,
		Name:   "No NVIDIA GPU detected",
		Reason: "GPU acceleration requires an NVIDIA GPU with CUDA support",
	}
}

func detectGPULinux() GPUInfo {
	if name := detectNVIDIA(); name != "" {
		return GPUInfo{
			Type:      GPUCUDA,
			Name:      name,
			Available: true,
			Installed: true,
		}
	}
	return GPUInfo{
		Type:   GPUNone,
		Name:   "No NVIDIA GPU detected",
		Reason: "GPU acceleration requires an NVIDIA GPU with CUDA support",
	}
}

// detectNVIDIA checks if an NVIDIA GPU is present and returns its name.
func detectNVIDIA() string {
	out, err := exec.Command("nvidia-smi", "--query-gpu=name", "--format=csv,noheader,nounits").Output()
	if err == nil {
		name := strings.TrimSpace(string(out))
		if name != "" {
			// Take first line (first GPU) if multiple.
			if idx := strings.IndexByte(name, '\n'); idx != -1 {
				name = strings.TrimSpace(name[:idx])
			}
			return "NVIDIA " + name
		}
	}
	return ""
}
