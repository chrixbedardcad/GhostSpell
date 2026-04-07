import { useState, useEffect, useRef } from "react";
import { goCall, openURL } from "@/bridge";

/**
 * About tab — app info, update check, bug report, licenses.
 * Zen: centered, calm, just the essentials.
 */
export function AboutTab() {
  const [version, setVersion] = useState("");
  const [updateStatus, setUpdateStatus] = useState("");
  const [updateURL, setUpdateURL] = useState("");
  const [checking, setChecking] = useState(false);
  const [updating, setUpdating] = useState(false);
  const [updatePhase, setUpdatePhase] = useState("");
  const [updatePercent, setUpdatePercent] = useState(0);
  const [updateError, setUpdateError] = useState("");
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  useEffect(() => {
    goCall("getVersion").then((v) => v && setVersion(v));
    return () => { if (pollRef.current) clearInterval(pollRef.current); };
  }, []);

  async function checkUpdate() {
    setChecking(true);
    setUpdateStatus("");
    setUpdateURL("");
    setUpdateError("");
    const raw = await goCall("checkForUpdate");
    if (!raw) {
      setUpdateStatus("Check failed.");
      setChecking(false);
      return;
    }
    try {
      const data = JSON.parse(raw);
      if (data.error) {
        setUpdateStatus(`Check failed: ${data.error}`);
      } else if (data.has_update) {
        setUpdateStatus(`Update available: v${data.latest}`);
        setUpdateURL(data.url || "");
      } else {
        setUpdateStatus("You're up to date.");
      }
    } catch {
      setUpdateStatus("Check failed.");
    }
    setChecking(false);
  }

  async function doUpdate() {
    setUpdating(true);
    setUpdateError("");
    setUpdatePhase("starting");
    setUpdatePercent(0);

    const result = await goCall("updateNow");
    if (result && result.startsWith("error")) {
      setUpdateError(result);
      setUpdating(false);
      return;
    }

    // Poll progress.
    pollRef.current = setInterval(async () => {
      const raw = await goCall("getUpdateProgress");
      if (!raw) return;
      try {
        const p = JSON.parse(raw);
        setUpdatePhase(p.phase || "");
        setUpdatePercent(p.percent || 0);
        if (p.error) {
          setUpdateError(p.error);
          setUpdating(false);
          if (pollRef.current) clearInterval(pollRef.current);
        }
        if (p.phase === "restarting") {
          if (pollRef.current) clearInterval(pollRef.current);
        }
      } catch { /* ignore */ }
    }, 500);
  }

  return (
    <div className="space-y-8">
      {/* App identity */}
      <div className="text-center py-4">
        <img src="/ghost-icon.png" alt="" className="w-16 h-16 mx-auto mb-4 opacity-80" />
        <h2 className="text-xl font-semibold text-text tracking-tight">GhostSpell</h2>
        {version && <p className="text-sm text-overlay-0 mt-1">v{version}</p>}
        <p className="text-xs text-overlay-0 mt-3">AI-powered text correction and rewriting.</p>

        <div className="flex items-center justify-center gap-4 mt-4 text-xs">
          <button
            onClick={() => openURL("https://github.com/chrixbedardcad/GhostSpell")}
            className="text-accent-blue hover:text-accent-blue/80 transition-colors"
          >
            GitHub
          </button>
          <span className="text-surface-1">·</span>
          <span className="text-accent-green">AGPL-3.0</span>
        </div>
      </div>

      {/* Update check */}
      <section className="bg-surface-0/20 border border-surface-0/40 rounded-xl px-5 py-3.5">
        <div className="flex items-center justify-between">
          <div>
            <h3 className="text-sm font-medium text-text">Updates</h3>
            {updateStatus && (
              <p className={`text-xs mt-1 ${updateURL ? "text-accent-green" : "text-overlay-0"}`}>
                {updateStatus}
              </p>
            )}
            {updateError && (
              <p className="text-xs mt-1 text-red-400">{updateError}</p>
            )}
          </div>
          <div className="flex items-center gap-2">
            {updateURL && !updating && (
              <button
                onClick={doUpdate}
                className="px-4 py-1.5 rounded-lg text-xs font-medium
                           bg-accent-green/15 text-accent-green hover:bg-accent-green/25
                           transition-colors"
              >
                Update Now
              </button>
            )}
            <button
              onClick={checkUpdate}
              disabled={checking || updating}
              className="px-4 py-1.5 rounded-lg text-xs font-medium
                         bg-accent-blue/15 text-accent-blue hover:bg-accent-blue/25
                         disabled:opacity-50 transition-colors"
            >
              {checking ? "Checking..." : "Check for Updates"}
            </button>
          </div>
        </div>

        {/* Update progress */}
        {updating && (
          <div className="mt-3">
            <div className="w-full bg-surface-1 rounded-full h-2 overflow-hidden">
              <div
                className="h-full bg-gradient-to-r from-blue-400 to-green-400 rounded-full transition-all duration-300"
                style={{ width: `${Math.max(2, updatePercent)}%` }}
              />
            </div>
            <p className="text-[10px] text-overlay-0 mt-1 capitalize">
              {updatePhase === "downloading" ? `Downloading... ${updatePercent.toFixed(0)}%` :
               updatePhase === "installing" ? "Installing..." :
               updatePhase === "restarting" ? "Restarting..." :
               updatePhase || "Starting..."}
            </p>
          </div>
        )}
      </section>

      {/* Open source licenses */}
      <details className="group">
        <summary className="text-xs font-medium text-overlay-0 cursor-pointer
                          flex items-center gap-2 hover:text-subtext-0 transition-colors">
          <span className="text-[10px] transition-transform group-open:rotate-90">&#9654;</span>
          Open Source Licenses
        </summary>
        <div className="mt-3 space-y-3 text-xs text-overlay-1 leading-relaxed">
          <div>
            <strong className="text-subtext-0">llama.cpp</strong> — MIT License
            <br />Local AI inference engine used by GhostSpell Local.
          </div>
          <div>
            <strong className="text-subtext-0">Wails</strong> — MIT License
            <br />Desktop application framework.
          </div>
          <div>
            <strong className="text-subtext-0">Qwen3 / Qwen3.5 Models</strong> — Apache 2.0
            <br />Language models by Alibaba Cloud.
          </div>
          <div>
            <strong className="text-subtext-0">Phi-4 Mini</strong> — MIT License
            <br />Language model by Microsoft.
          </div>
        </div>
      </details>
    </div>
  );
}
