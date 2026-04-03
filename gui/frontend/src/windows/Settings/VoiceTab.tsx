import { useState, useEffect, useRef } from "react";
import { goCall } from "@/bridge";

interface VoiceModel {
  name: string;
  downloaded: boolean;
  active: boolean;
  size_mb: number;
}

interface DownloadProgress {
  percent: number;
  type: string;
  model?: string;
}

const WHISPER_MODELS: { name: string; label: string; desc: string }[] = [
  { name: "whisper-tiny",            label: "Tiny",             desc: "39 MB  --  Fastest, lowest accuracy" },
  { name: "whisper-base",            label: "Base",             desc: "74 MB  --  Fast, basic accuracy" },
  { name: "whisper-small",           label: "Small",            desc: "244 MB  --  Balanced speed/accuracy" },
  { name: "whisper-medium",          label: "Medium",           desc: "769 MB  --  Good accuracy, slower" },
  { name: "whisper-large-v3-turbo",  label: "Large v3 Turbo",  desc: "809 MB  --  Fast + high accuracy" },
  { name: "whisper-large-v3",        label: "Large v3",         desc: "1550 MB  --  Best accuracy, slowest" },
];

/**
 * Voice tab -- voice/STT model management, mic test, keep-alive toggle.
 */
export function VoiceTab() {
  const [models, setModels] = useState<VoiceModel[]>([]);
  const [activeModel, setActiveModel] = useState("");
  const [keepAlive, setKeepAlive] = useState(false);
  const [downloading, setDownloading] = useState("");
  const [progress, setProgress] = useState(0);
  const [testResult, setTestResult] = useState("");
  const [testing, setTesting] = useState(false);
  const [testingSample, setTestingSample] = useState(false);
  const progressTimer = useRef<ReturnType<typeof setInterval> | null>(null);

  // Load voice status and config on mount
  useEffect(() => {
    loadStatus();
    loadConfig();
    return () => {
      if (progressTimer.current) clearInterval(progressTimer.current);
    };
  }, []);

  async function loadStatus() {
    const raw = await goCall("voiceStatus");
    if (raw) {
      try {
        const st = JSON.parse(raw);
        if (st.models) setModels(st.models);
        if (st.active_model) setActiveModel(st.active_model);
      } catch { /* ignore */ }
    }
    // Also try available models list
    const avail = await goCall("voiceAvailableModels");
    if (avail) {
      try {
        const list = JSON.parse(avail);
        if (Array.isArray(list)) {
          setModels(list);
        }
      } catch { /* ignore */ }
    }
  }

  async function loadConfig() {
    const raw = await goCall("getConfig");
    if (!raw) return;
    try {
      const cfg = JSON.parse(raw);
      setActiveModel(cfg.voice?.model || "");
      setKeepAlive(cfg.voice?.keep_alive ?? false);
    } catch { /* ignore */ }
  }

  function startProgressPolling() {
    if (progressTimer.current) clearInterval(progressTimer.current);
    progressTimer.current = setInterval(async () => {
      const raw = await goCall("localDownloadProgress");
      if (!raw) return;
      try {
        const p: DownloadProgress = JSON.parse(raw);
        if (p.type === "voice") {
          setProgress(p.percent || 0);
          if (p.percent >= 100) {
            stopProgressPolling();
            setDownloading("");
            setProgress(0);
            loadStatus();
          }
        }
      } catch { /* ignore */ }
    }, 500);
  }

  function stopProgressPolling() {
    if (progressTimer.current) {
      clearInterval(progressTimer.current);
      progressTimer.current = null;
    }
  }

  async function downloadModel(name: string) {
    setDownloading(name);
    setProgress(0);
    startProgressPolling();
    const result = await goCall("voiceDownloadModel", name);
    // Download might complete synchronously for small models
    if (result && !result.startsWith("error")) {
      stopProgressPolling();
      setDownloading("");
      setProgress(0);
      loadStatus();
    } else if (result?.startsWith("error")) {
      stopProgressPolling();
      setDownloading("");
      setProgress(0);
      setTestResult(result);
    }
  }

  async function cancelDownload() {
    await goCall("cancelDownload");
    stopProgressPolling();
    setDownloading("");
    setProgress(0);
  }

  async function deleteModel(name: string) {
    await goCall("voiceDeleteModel", name);
    loadStatus();
  }

  async function selectModel(name: string) {
    setActiveModel(name);
    await goCall("setVoiceModel", name);
  }

  async function toggleKeepAlive(v: boolean) {
    setKeepAlive(v);
    await goCall("setVoiceKeepAlive", v);
  }

  async function testMic() {
    setTesting(true);
    setTestResult("Recording 3 seconds...");
    const result = await goCall("testVoice");
    setTestResult(result || "No response");
    setTesting(false);
  }

  async function testSample() {
    setTestingSample(true);
    setTestResult("Testing with sample audio...");
    const result = await goCall("testVoiceSample");
    setTestResult(result || "No response");
    setTestingSample(false);
  }

  // Build display list from WHISPER_MODELS + downloaded state
  function isDownloaded(name: string): boolean {
    return models.some((m) => m.name === name && m.downloaded);
  }

  return (
    <div className="space-y-8">
      {/* Voice Model */}
      <section>
        <h2 className="text-sm font-medium text-subtext-1 mb-4 tracking-wide uppercase">
          Voice Model
        </h2>
        <div className="bg-surface-0/30 rounded-xl p-5 space-y-2">
          {WHISPER_MODELS.map((wm) => {
            const downloaded = isDownloaded(wm.name);
            const isActive = activeModel === wm.name;
            const isDownloading = downloading === wm.name;

            return (
              <div
                key={wm.name}
                className={`flex items-center gap-4 p-3 rounded-lg transition-colors
                  ${isActive ? "bg-accent-blue/10 border border-accent-blue/20" : "border border-transparent hover:bg-surface-0/40"}`}
              >
                {/* Model info */}
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium text-text">{wm.label}</span>
                    {isActive && (
                      <span className="px-1.5 py-0.5 rounded text-[10px] font-medium bg-accent-blue/15 text-accent-blue uppercase">
                        Active
                      </span>
                    )}
                    {downloaded && !isActive && (
                      <span className="px-1.5 py-0.5 rounded text-[10px] font-medium bg-accent-green/15 text-accent-green uppercase">
                        Ready
                      </span>
                    )}
                  </div>
                  <p className="text-xs text-overlay-0 mt-0.5">{wm.desc}</p>

                  {/* Download progress bar */}
                  {isDownloading && (
                    <div className="mt-2 flex items-center gap-3">
                      <div className="flex-1 h-1.5 bg-crust rounded-full overflow-hidden">
                        <div
                          className="h-full bg-accent-blue rounded-full transition-all duration-300"
                          style={{ width: `${progress}%` }}
                        />
                      </div>
                      <span className="text-[11px] text-overlay-0 shrink-0 w-10 text-right">
                        {progress.toFixed(0)}%
                      </span>
                    </div>
                  )}
                </div>

                {/* Actions */}
                <div className="flex items-center gap-2 shrink-0">
                  {isDownloading ? (
                    <button
                      onClick={cancelDownload}
                      className="px-2.5 py-1 rounded-md text-[11px] font-medium
                                 bg-red-500/10 text-red-400 hover:bg-red-500/20 transition-colors"
                    >
                      Cancel
                    </button>
                  ) : downloaded ? (
                    <>
                      {!isActive && (
                        <button
                          onClick={() => selectModel(wm.name)}
                          className="px-2.5 py-1 rounded-md text-[11px] font-medium
                                     bg-accent-blue/10 text-accent-blue hover:bg-accent-blue/20 transition-colors"
                        >
                          Use
                        </button>
                      )}
                      {!isActive && (
                        <button
                          onClick={() => deleteModel(wm.name)}
                          className="px-2.5 py-1 rounded-md text-[11px] font-medium
                                     bg-red-500/10 text-red-400 hover:bg-red-500/20 transition-colors"
                        >
                          Delete
                        </button>
                      )}
                    </>
                  ) : (
                    <button
                      onClick={() => downloadModel(wm.name)}
                      disabled={downloading !== ""}
                      className="px-2.5 py-1 rounded-md text-[11px] font-medium
                                 bg-accent-blue/10 text-accent-blue hover:bg-accent-blue/20
                                 disabled:opacity-40 transition-colors"
                    >
                      Download
                    </button>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      </section>

      {/* Test Microphone */}
      <section>
        <h2 className="text-sm font-medium text-subtext-1 mb-4 tracking-wide uppercase">
          Test Microphone
        </h2>
        <div className="bg-surface-0/30 rounded-xl p-5 space-y-4">
          <div className="flex items-center gap-3">
            <button
              onClick={testMic}
              disabled={testing || !activeModel}
              className="px-3 py-1.5 rounded-lg text-sm font-medium
                         bg-accent-blue/15 text-accent-blue hover:bg-accent-blue/25
                         disabled:opacity-40 transition-colors"
            >
              {testing ? "Recording..." : "Test Mic"}
            </button>
            <button
              onClick={testSample}
              disabled={testingSample || !activeModel}
              className="px-3 py-1.5 rounded-lg text-sm font-medium
                         bg-surface-1 text-subtext-0 hover:text-text hover:bg-surface-1/80
                         disabled:opacity-40 transition-colors"
            >
              {testingSample ? "Testing..." : "Test Sample"}
            </button>
            {!activeModel && (
              <span className="text-xs text-overlay-0">Select a voice model first</span>
            )}
          </div>

          {/* Recording level gauge */}
          {testing && (
            <div className="space-y-2">
              <div className="flex items-center gap-3">
                <span className="relative flex h-3 w-3 shrink-0">
                  <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-red-400 opacity-75" />
                  <span className="relative inline-flex rounded-full h-3 w-3 bg-red-500" />
                </span>
                <span className="text-xs font-medium text-red-400">Recording...</span>
              </div>
              <div className="h-2 bg-crust rounded-full overflow-hidden">
                <div
                  className="h-full rounded-full bg-gradient-to-r from-accent-blue via-accent-green to-accent-blue"
                  style={{
                    animation: "mic-pulse 1.2s ease-in-out infinite",
                    width: "100%",
                  }}
                />
              </div>
              <style>{`
                @keyframes mic-pulse {
                  0%, 100% { transform: scaleX(0.2); transform-origin: left; opacity: 0.6; }
                  25% { transform: scaleX(0.8); transform-origin: left; opacity: 1; }
                  50% { transform: scaleX(0.4); transform-origin: left; opacity: 0.8; }
                  75% { transform: scaleX(0.95); transform-origin: left; opacity: 1; }
                }
              `}</style>
            </div>
          )}

          {testResult && (
            <div className={`px-3 py-2 rounded-lg text-sm font-mono break-all ${
              testResult.startsWith("error")
                ? "bg-red-500/10 text-red-400"
                : "bg-green-500/10 text-green-400"
            }`}>
              {testResult}
            </div>
          )}
        </div>
      </section>

      {/* Settings */}
      <section>
        <h2 className="text-sm font-medium text-subtext-1 mb-4 tracking-wide uppercase">
          Settings
        </h2>
        <div className="bg-surface-0/30 rounded-xl p-5">
          <div className="flex items-center justify-between py-2">
            <div>
              <p className="text-sm text-text">Keep Model Loaded</p>
              <p className="text-xs text-overlay-0 mt-0.5">
                Keep the voice model in memory for faster response. Uses more RAM.
              </p>
            </div>
            <button
              onClick={() => toggleKeepAlive(!keepAlive)}
              className={`relative shrink-0 transition-colors duration-200 ${
                keepAlive ? "bg-accent-blue" : "bg-surface-2"
              }`}
              style={{ width: 36, height: 20, borderRadius: 10 }}
            >
              <span
                className="absolute bg-white rounded-full shadow-sm"
                style={{ width: 16, height: 16, top: 2, left: keepAlive ? 18 : 2, transition: "left 200ms ease" }}
              />
            </button>
          </div>
        </div>
      </section>
    </div>
  );
}
