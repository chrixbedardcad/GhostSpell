import { useState, useEffect, useRef } from "react";
import { goCall } from "@/bridge";

// Matches the JSON shape returned by Go's SettingsService.VoiceStatus().
interface VoiceModelStatus {
  name: string;
  file_name: string;
  size: number;
  tag: string;
  desc: string;
  installed: boolean;
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
  const [models, setModels] = useState<VoiceModelStatus[]>([]);
  const [activeModel, setActiveModel] = useState("");
  const [keepAlive, setKeepAlive] = useState(false);
  const [downloading, setDownloading] = useState("");
  const [progress, setProgress] = useState(0);
  const [testResult, setTestResult] = useState("");
  const [testing, setTesting] = useState(false);
  const [testingSample, setTestingSample] = useState(false);
  const progressTimer = useRef<ReturnType<typeof setInterval> | null>(null);

  // Live mic test state (Web Audio API — browser-side, no Go backend)
  const [micStream, setMicStream] = useState<MediaStream | null>(null);
  const [micError, setMicError] = useState("");
  const micBarRef = useRef<HTMLDivElement>(null);
  const micCtxRef = useRef<AudioContext | null>(null);
  const micAnimRef = useRef<number | null>(null);

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
    if (!raw) return;
    try {
      const parsed = JSON.parse(raw);
      // VoiceStatus returns a bare JSON array on success, or {error: "..."} on failure.
      if (Array.isArray(parsed)) {
        setModels(parsed);
      }
    } catch { /* ignore */ }
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
    if (result?.startsWith("error")) {
      stopProgressPolling();
      setDownloading("");
      setProgress(0);
      setTestResult(result);
      return;
    }
    stopProgressPolling();
    setDownloading("");
    setProgress(0);
    await loadStatus();
    // Auto-activate the freshly downloaded model. Downloading is an explicit
    // user action and the natural intent is "use this one" — matches the
    // wizard flow and avoids a dead state where a model is on disk but the
    // active config still points at a missing file.
    await selectModel(name);
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

  // Live mic test — real-time audio level using Web Audio API.
  async function startLiveMic() {
    setMicError("");
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
      setMicStream(stream);
      const ctx = new AudioContext();
      micCtxRef.current = ctx;
      const source = ctx.createMediaStreamSource(stream);
      const analyser = ctx.createAnalyser();
      analyser.fftSize = 256;
      source.connect(analyser);
      const data = new Uint8Array(analyser.frequencyBinCount);

      function update() {
        if (!micBarRef.current) return;
        analyser.getByteFrequencyData(data);
        let sum = 0;
        for (let i = 0; i < data.length; i++) sum += data[i];
        const avg = sum / data.length;
        const pct = Math.min(100, Math.round((avg / 128) * 100));
        micBarRef.current.style.width = pct + "%";
        micBarRef.current.style.background = pct > 60
          ? "linear-gradient(90deg, #a6e3a1, #f9e2af, #f38ba8)"
          : "linear-gradient(90deg, #a6e3a1, #89b4fa)";
        micAnimRef.current = requestAnimationFrame(update);
      }
      update();
    } catch {
      setMicError("No mic access");
    }
  }

  function stopLiveMic() {
    if (micStream) {
      micStream.getTracks().forEach((t) => t.stop());
      setMicStream(null);
    }
    if (micCtxRef.current) { micCtxRef.current.close(); micCtxRef.current = null; }
    if (micAnimRef.current) { cancelAnimationFrame(micAnimRef.current); micAnimRef.current = null; }
    if (micBarRef.current) micBarRef.current.style.width = "0%";
  }

  // Cleanup on unmount.
  useEffect(() => {
    return () => { stopLiveMic(); };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // A model is "downloaded" only if its file actually exists on disk.
  // The active model field in config is not a reliable signal — it can point
  // at a model that was never downloaded (e.g. a fresh install with the
  // default whisper-base set but no model file present).
  function isDownloaded(name: string): boolean {
    return models.some((m) => m.name === name && m.installed);
  }

  return (
    <div className="space-y-8">
      {/* Voice Model */}
      <section>
        <h2 className="text-[11px] font-semibold text-overlay-0 mb-4 uppercase tracking-widest">
          Voice Model
        </h2>
        <div className="bg-surface-0/20 border border-surface-0/40 rounded-xl px-5 py-3.5 space-y-3">
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

      {/* Live Mic Test */}
      <section>
        <h2 className="text-[11px] font-semibold text-overlay-0 mb-4 uppercase tracking-widest">
          Live Microphone
        </h2>
        <div className="bg-surface-0/20 border border-surface-0/40 rounded-xl px-5 py-3.5 space-y-3">
          <div className="flex items-center gap-3">
            <button
              onClick={() => {
                if (micStream) { stopLiveMic(); } else { startLiveMic(); }
              }}
              className={`px-3 py-1.5 rounded-lg text-sm font-medium transition-colors ${
                micStream
                  ? "bg-red-500/20 text-red-400 hover:bg-red-500/30"
                  : "bg-accent-blue/15 text-accent-blue hover:bg-accent-blue/25"
              }`}
            >
              {micStream ? "\u23F9 Stop" : "\uD83C\uDFA4 Test Mic"}
            </button>
            <div className="flex-1 h-4 bg-crust rounded-full overflow-hidden">
              <div
                ref={micBarRef}
                className="h-full rounded-full transition-all"
                style={{
                  width: "0%",
                  background: "linear-gradient(90deg, #a6e3a1, #89b4fa)",
                  transition: "width 50ms",
                }}
              />
            </div>
            <span className="text-[11px] text-overlay-0 min-w-[60px] text-right">
              {micStream ? "Listening..." : micError || "Ready"}
            </span>
          </div>
        </div>
      </section>

      {/* Voice Test */}
      <section>
        <h2 className="text-[11px] font-semibold text-overlay-0 mb-4 uppercase tracking-widest">
          Voice Transcription Test
        </h2>
        <div className="bg-surface-0/20 border border-surface-0/40 rounded-xl px-5 py-3.5 space-y-4">
          <div className="flex items-center gap-3">
            <button
              onClick={testMic}
              disabled={testing || !activeModel}
              className="px-3 py-1.5 rounded-lg text-sm font-medium
                         bg-accent-blue/15 text-accent-blue hover:bg-accent-blue/25
                         disabled:opacity-40 transition-colors"
            >
              {testing ? "Recording 3s..." : "Test Voice"}
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
              <span className="text-xs text-overlay-0 italic">Select a voice model first</span>
            )}
          </div>

          {/* Recording indicator */}
          {testing && (
            <div className="flex items-center gap-2">
              <span className="relative flex h-2.5 w-2.5 shrink-0">
                <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-red-400 opacity-75" />
                <span className="relative inline-flex rounded-full h-2.5 w-2.5 bg-red-500" />
              </span>
              <span className="text-xs text-red-400">Recording from microphone...</span>
            </div>
          )}

          {testResult && (
            <div className={`px-3 py-2 rounded-lg text-[12px] break-all ${
              testResult.startsWith("error")
                ? "bg-red-500/10 text-red-400"
                : testResult === "" || testResult === "No response"
                ? "bg-yellow-500/10 text-yellow-400"
                : "bg-green-500/10 text-green-400"
            }`}>
              {testResult}
            </div>
          )}
        </div>
      </section>

      {/* Settings */}
      <section>
        <h2 className="text-[11px] font-semibold text-overlay-0 mb-4 uppercase tracking-widest">
          Settings
        </h2>
        <div className="bg-surface-0/20 border border-surface-0/40 rounded-xl px-5 py-3.5">
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
              style={{ width: 44, height: 24, borderRadius: 12 }}
            >
              <span
                className="absolute bg-white rounded-full shadow-sm"
                style={{ width: 20, height: 20, top: 2, left: keepAlive ? 22 : 2, transition: "left 200ms ease" }}
              />
            </button>
          </div>
        </div>
      </section>
    </div>
  );
}
