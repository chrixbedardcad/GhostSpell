import { useState, useEffect } from "react";
import { goCall } from "@/bridge";

interface GPUInfo {
  type: string;
  name: string;
  available: boolean;
  reason: string;
  installed: boolean;
  binary_mb: number;
}

/**
 * GPU tab — GPU detection and acceleration toggle.
 * Shows what GPU is available and whether acceleration is active.
 */
export function GPUTab() {
  const [gpu, setGPU] = useState<GPUInfo | null>(null);
  const [gpuEnabled, setGpuEnabled] = useState(true);

  useEffect(() => {
    goCall("detectGPU").then((raw) => {
      if (raw) {
        try { setGPU(JSON.parse(raw)); } catch { /* ignore */ }
      }
    });
    goCall("getConfig").then((raw) => {
      if (!raw) return;
      try {
        const cfg = JSON.parse(raw);
        setGpuEnabled(cfg.gpu_enabled !== false);
      } catch { /* ignore */ }
    });
  }, []);

  async function toggleGPU(enabled: boolean) {
    setGpuEnabled(enabled);
    await goCall("setGPUEnabled", enabled);
  }

  if (!gpu) {
    return <div className="text-sm text-overlay-0">Detecting GPU...</div>;
  }

  return (
    <div className="space-y-10">
      {/* Detection Result */}
      <section>
        <h2 className="text-[13px] font-semibold text-subtext-0 mb-5 uppercase tracking-wider">
          GPU Detection
        </h2>
        <div className="bg-surface-0/30 rounded-xl p-7 space-y-3">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm text-text">{gpu.name || "No GPU detected"}</p>
              <p className="text-xs text-overlay-0 mt-0.5">
                {gpu.available
                  ? `${gpu.type.toUpperCase()} acceleration available`
                  : gpu.reason || "GPU acceleration not available"}
              </p>
            </div>
            <span className={`px-2.5 py-1 rounded-full text-xs font-medium ${
              gpu.available
                ? "bg-green-500/15 text-green-400"
                : "bg-surface-1 text-overlay-0"
            }`}>
              {gpu.available ? "Available" : "Unavailable"}
            </span>
          </div>
        </div>
      </section>

      {/* GPU Toggle */}
      {gpu.available && (
        <section>
          <h2 className="text-[13px] font-semibold text-subtext-0 mb-5 uppercase tracking-wider">
            Acceleration
          </h2>
          <div className="bg-surface-0/30 rounded-xl p-7">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm text-text">Enable GPU Acceleration</p>
                <p className="text-xs text-overlay-0 mt-0.5">
                  Offload AI inference to GPU for 5-10x faster processing.
                  Disable to force CPU-only mode.
                </p>
              </div>
              <button
                onClick={() => toggleGPU(!gpuEnabled)}
                className={`relative shrink-0 transition-colors duration-200 ${
                  gpuEnabled ? "bg-accent-blue" : "bg-surface-2"
                }`}
                style={{ width: 44, height: 24, borderRadius: 12 }}
              >
                <span className="absolute bg-white rounded-full shadow-sm" style={{ width: 20, height: 20, top: 2, left: gpuEnabled ? 22 : 2, transition: "left 200ms ease" }} />
              </button>
            </div>
          </div>
        </section>
      )}

      {/* How It Works */}
      <section>
        <h2 className="text-[13px] font-semibold text-subtext-0 mb-5 uppercase tracking-wider">
          How It Works
        </h2>
        <div className="bg-surface-0/30 rounded-xl p-7 space-y-3 text-xs text-overlay-1">
          <p>
            GPU acceleration offloads AI model layers to your graphics card,
            making local inference 5-10x faster than CPU-only mode.
          </p>
          {gpu.type === "metal" ? (
            <p>
              Metal acceleration is built into the macOS binary. If enabled,
              GhostSpell automatically offloads model layers to your Apple Silicon GPU.
            </p>
          ) : gpu.type === "cuda" ? (
            <>
              <p>
                CUDA acceleration requires an NVIDIA GPU with up-to-date drivers.
                GhostSpell automatically detects your GPU and calculates optimal layer offloading
                based on your available VRAM and model size.
              </p>
              <p>
                For CUDA acceleration, build GhostSpell locally with <code className="text-subtext-0">_build.bat</code> on a machine
                with CUDA toolkit installed. CI releases are CPU-only to keep downloads small.
              </p>
            </>
          ) : (
            <p>
              GPU acceleration requires an NVIDIA GPU (CUDA) on Windows/Linux
              or Apple Silicon (Metal) on macOS.
            </p>
          )}
        </div>
      </section>
    </div>
  );
}
