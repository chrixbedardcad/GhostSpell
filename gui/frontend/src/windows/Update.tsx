import { useState, useEffect, useRef } from "react";
import { goCall } from "@/bridge";

/**
 * Update popup — download and install progress.
 * Zen: minimal, focused on progress.
 */
export function UpdateWindow() {
  const [phase, setPhase] = useState("preparing");
  const [percent, setPercent] = useState(0);
  const [version, setVersion] = useState("");
  const pollRef = useRef<number | null>(null);

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    setVersion(params.get("version") || "");

    pollRef.current = window.setInterval(async () => {
      const raw = await goCall("getUpdateProgress");
      if (!raw) return;
      try {
        const p = JSON.parse(raw);
        setPhase(p.phase || "preparing");
        setPercent(Math.round(p.percent || 0));
        if (p.phase === "done" || p.phase === "restarting") {
          if (pollRef.current) clearInterval(pollRef.current);
        }
      } catch { /* ignore */ }
    }, 500);

    return () => { if (pollRef.current) clearInterval(pollRef.current); };
  }, []);

  const phaseText = {
    preparing: "Preparing...",
    downloading: `Downloading... ${percent}%`,
    extracting: "Extracting...",
    installing: "Installing...",
    restarting: "Restarting...",
    done: "Update complete!",
  }[phase] || phase;

  return (
    <div className="h-full flex flex-col items-center justify-center bg-base p-8">
      <img src="/ghost-icon.png" alt="" className="w-12 h-12 mb-4 opacity-70" />
      <h2 className="text-sm font-medium text-text mb-1">
        {version ? `Updating to v${version}` : "Updating GhostSpell"}
      </h2>
      <p className="text-xs text-overlay-0 mb-6">{phaseText}</p>

      {/* Progress bar */}
      <div className="w-full max-w-xs bg-surface-1 rounded-full h-1.5 overflow-hidden">
        <div
          className="h-full bg-accent-blue rounded-full transition-all duration-300"
          style={{ width: `${phase === "done" ? 100 : percent}%` }}
        />
      </div>

      {phase !== "done" && phase !== "restarting" && (
        <button
          onClick={() => window.wails.Window.Close()}
          className="mt-6 text-xs text-overlay-0 hover:text-subtext-0 transition-colors"
        >
          Cancel
        </button>
      )}
    </div>
  );
}
