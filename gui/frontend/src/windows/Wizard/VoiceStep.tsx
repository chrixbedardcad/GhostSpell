import { useState } from "react";
import { goCall } from "@/bridge";

/**
 * Voice step — optional whisper model download for speech-to-text.
 * Fully skippable.
 */
export function VoiceStep({ onNext }: { onNext: () => void }) {
  const [downloading, setDownloading] = useState(false);
  const [progress, setProgress] = useState(0);
  const [status, setStatus] = useState("");
  const [error, setError] = useState("");

  async function downloadVoice() {
    setDownloading(true);
    setProgress(0);
    setError("");
    setStatus("Downloading voice model...");

    const pollId = window.setInterval(async () => {
      const raw = await goCall("localDownloadProgress");
      if (raw) {
        try {
          const p = JSON.parse(raw);
          setProgress(Math.round(p.percent || 0));
        } catch { /* ignore */ }
      }
    }, 500);

    const result = await goCall("voiceDownloadModel", "whisper-base");
    clearInterval(pollId);

    if (result && (result.startsWith("ok") || result === "")) {
      setProgress(100);
      setStatus("Voice model installed!");

      // Enable voice in config.
      await goCall("setVoiceModel", "whisper-base");

      setTimeout(() => onNext(), 800);
    } else {
      setError(result || "Download failed");
      setStatus("");
      setDownloading(false);
    }
  }

  return (
    <div className="max-w-lg mx-auto py-4 space-y-6">
      <div className="text-center mb-4">
        <h2 className="text-lg font-semibold text-text">Voice-to-Text</h2>
        <p className="text-xs text-overlay-0 mt-1">
          Add speech-to-text so you can dictate text and have it corrected by AI.
        </p>
      </div>

      {/* Voice Card */}
      <div className="bg-surface-0/30 border border-surface-0/50 rounded-xl p-5">
        <div className="flex items-center gap-3 mb-3">
          <div className="w-10 h-10 rounded-full flex items-center justify-center text-lg
                          bg-accent-blue/15 text-accent-blue">
            🎤
          </div>
          <div className="flex-1">
            <p className="text-sm font-medium text-text">Whisper Base</p>
            <p className="text-xs text-overlay-0 mt-0.5">
              Local speech-to-text (~142MB). Good balance of speed and accuracy.
            </p>
          </div>
          <span className="text-xs font-semibold text-accent-green">FREE</span>
        </div>

        {downloading ? (
          <div className="space-y-2">
            <div className="w-full bg-surface-1 rounded-full h-2 overflow-hidden">
              <div
                className="h-full bg-gradient-to-r from-accent-blue to-accent-green rounded-full transition-all"
                style={{ width: `${progress}%` }}
              />
            </div>
            <p className="text-xs text-overlay-0">{status || `${progress}%`}</p>
          </div>
        ) : (
          <button
            onClick={downloadVoice}
            className="w-full py-2 rounded-lg text-sm font-medium
                       bg-accent-blue text-crust hover:bg-accent-blue/90 transition-colors"
          >
            Download Voice Model (~142 MB)
          </button>
        )}
        {error && <p className="text-xs text-accent-red mt-2">{error}</p>}
      </div>

      {/* How it works */}
      <div className="bg-surface-0/20 rounded-xl p-4 space-y-2 text-xs text-overlay-1">
        <p>With voice enabled, you can:</p>
        <ul className="list-disc list-inside space-y-1 ml-1">
          <li>Hold a hotkey and speak — GhostSpell transcribes and processes your speech</li>
          <li>Use voice skills like "Dictate" to convert speech to text</li>
          <li>All voice processing runs locally — nothing sent to the cloud</li>
        </ul>
      </div>

      {/* Skip */}
      {!downloading && (
        <div className="text-center">
          <button
            onClick={onNext}
            className="text-xs text-overlay-0 hover:text-subtext-0 transition-colors"
          >
            Skip — I don't need voice right now
          </button>
          <p className="text-[11px] text-overlay-0 mt-2">
            You can download the voice model later in Settings.
          </p>
        </div>
      )}
    </div>
  );
}
