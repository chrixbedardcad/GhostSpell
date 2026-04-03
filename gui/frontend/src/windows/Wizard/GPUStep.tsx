import { useState, useEffect } from "react";
import { goCall } from "@/bridge";

interface GPUInfo {
  type: string;
  name: string;
  available: boolean;
  reason: string;
}

/**
 * GPU step — detect GPU and show acceleration status.
 * Informational only. The GPU toggle is in Settings > GPU.
 * Fully optional, auto-advances for users without GPU.
 */
export function GPUStep({
  onNext,
  hasLocalModel,
}: {
  onNext: () => void;
  hasLocalModel: boolean;
}) {
  const [gpu, setGPU] = useState<GPUInfo | null>(null);

  useEffect(() => {
    goCall("detectGPU").then((raw) => {
      if (raw) {
        try { setGPU(JSON.parse(raw)); } catch { /* ignore */ }
      }
    });
  }, []);

  const noGPU = !gpu || !gpu.available;
  const isMetalBuiltin = gpu?.type === "metal";

  return (
    <div className="max-w-lg mx-auto py-4 space-y-6">
      <div className="text-center mb-4">
        <h2 className="text-lg font-semibold text-text">GPU Acceleration</h2>
        <p className="text-xs text-overlay-0 mt-1">
          {noGPU
            ? "GhostSpell works great on CPU. No GPU required."
            : isMetalBuiltin
            ? "Your Mac has Metal acceleration built in — it's already active."
            : `Your ${gpu?.name} will accelerate AI inference 5-10x.`}
        </p>
      </div>

      {/* GPU Detection Card */}
      <div className="bg-surface-0/30 border border-surface-0/50 rounded-xl p-5">
        <div className="flex items-center gap-3">
          <div className={`w-10 h-10 rounded-full flex items-center justify-center text-lg ${
            noGPU ? "bg-surface-1 text-overlay-0" :
            "bg-accent-green/15 text-accent-green"
          }`}>
            {noGPU ? "💻" : "⚡"}
          </div>
          <div className="flex-1">
            <p className="text-sm font-medium text-text">{gpu?.name || "CPU Mode"}</p>
            <p className="text-xs text-overlay-0 mt-0.5">
              {noGPU
                ? gpu?.reason || "All models run on CPU — reliable and always works"
                : isMetalBuiltin
                ? "Metal acceleration active — no setup needed"
                : "CUDA detected — GPU layers auto-calculated based on model size and available memory"}
            </p>
          </div>
        </div>

        {!noGPU && !isMetalBuiltin && !hasLocalModel && (
          <p className="text-xs text-overlay-0 mt-3 italic">
            Download a local AI model first, then GPU acceleration kicks in automatically.
          </p>
        )}
      </div>

      {/* Continue */}
      <div className="text-center">
        <button
          onClick={onNext}
          className="px-8 py-2.5 rounded-xl text-sm font-medium
                     bg-accent-blue text-crust hover:bg-accent-blue/90 transition-colors"
        >
          Continue
        </button>
        {!noGPU && (
          <p className="text-[11px] text-overlay-0 mt-2">
            Fine-tune GPU settings later in Settings &gt; GPU.
          </p>
        )}
      </div>
    </div>
  );
}
