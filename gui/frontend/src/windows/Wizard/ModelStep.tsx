import { useState } from "react";
import { goCall } from "@/bridge";
import { ProviderLogo } from "@/components/logos/ProviderLogo";

/**
 * Model selection step — choose AI provider.
 * Zen: clean cards, focused choices, no overwhelm.
 */
export function ModelStep({
  onNext,
  returnToSettings,
}: {
  onNext: () => void;
  returnToSettings: boolean;
}) {
  const [downloading, setDownloading] = useState(false);
  const [progress, setProgress] = useState(0);
  const [status, setStatus] = useState("");
  const [error, setError] = useState("");

  async function oneclickDownload() {
    setDownloading(true);
    setProgress(0);
    setError("");
    setStatus("Downloading...");

    // Poll progress
    const pollId = window.setInterval(async () => {
      const raw = await goCall("localDownloadProgress");
      if (raw) {
        try {
          const p = JSON.parse(raw);
          setProgress(Math.round(p.percent || 0));
        } catch { /* ignore */ }
      }
    }, 500);

    const result = await goCall("localDownloadModel", "qwen3.5-2b");
    clearInterval(pollId);

    if (result && result.startsWith("ok")) {
      setProgress(100);
      setStatus("Download complete!");

      // Configure provider + model + default
      await goCall("saveProviderConfig", "local", "", "", "", true);
      await goCall("saveModel", "GhostSpell Local", "local", "qwen3.5-2b", "");
      await goCall("setDefaultModel", "GhostSpell Local");

      setTimeout(() => onNext(), 800);
    } else {
      setError(result || "Download failed");
      setStatus("");
      setDownloading(false);
    }
  }

  async function startOAuth() {
    await goCall("startChatGPTOAuth");
    // OAuth redirects back — poll for result
    const pollId = window.setInterval(async () => {
      const result = await goCall("pollOAuthResult");
      if (result && result !== "pending") {
        clearInterval(pollId);
        try {
          const data = JSON.parse(result);
          if (data.api_key) {
            await goCall("setRefreshToken", "chatgpt", data.refresh_token);
            await goCall("saveModel", "chatgpt", "chatgpt", "gpt-5-mini", "");
            await goCall("setDefaultModel", "chatgpt");
            onNext();
          }
        } catch { /* ignore */ }
      }
    }, 2000);
  }

  return (
    <div className="max-w-lg mx-auto py-4 space-y-4">
      <div className="text-center mb-4">
        <h2 className="text-lg font-semibold text-text">Choose Your AI</h2>
        <p className="text-xs text-overlay-0 mt-1">Pick a model to get started. You can add more later.</p>
      </div>

      {/* Ghost-AI one-click */}
      <div className="bg-surface-0/30 border border-surface-0/50 rounded-xl p-5">
        <div className="flex items-center gap-3 mb-3">
          <ProviderLogo provider="local" size={28} />
          <div>
            <p className="text-sm font-medium text-text">GhostSpell Local</p>
            <p className="text-xs text-overlay-0">Free, private, runs on your device</p>
          </div>
          <span className="ml-auto text-xs font-semibold text-accent-green">FREE</span>
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
            onClick={oneclickDownload}
            className="w-full py-2 rounded-lg text-sm font-medium
                       bg-accent-blue text-crust hover:bg-accent-blue/90 transition-colors"
          >
            Download Qwen 3.5 2B (~1.3 GB)
          </button>
        )}
        {error && <p className="text-xs text-accent-red mt-2">{error}</p>}
      </div>

      {/* Divider */}
      <div className="flex items-center gap-3">
        <div className="flex-1 h-px bg-surface-0/50" />
        <span className="text-xs text-overlay-0">or use a cloud provider</span>
        <div className="flex-1 h-px bg-surface-0/50" />
      </div>

      {/* ChatGPT OAuth */}
      <ProviderCard
        provider="chatgpt"
        name="ChatGPT"
        description="One-click login with your OpenAI account"
        cost="$$"
        onClick={startOAuth}
        buttonLabel="Sign in with ChatGPT"
      />

      {/* Cloud providers */}
      <div className="grid grid-cols-2 gap-2">
        <MiniProviderCard provider="anthropic" name="Anthropic" />
        <MiniProviderCard provider="gemini" name="Google Gemini" />
        <MiniProviderCard provider="xai" name="xAI (Grok)" />
        <MiniProviderCard provider="deepseek" name="DeepSeek" />
      </div>

      <p className="text-[11px] text-overlay-0 text-center">
        Cloud providers require an API key. You can configure them in Settings after setup.
      </p>
    </div>
  );
}

function ProviderCard({
  provider,
  name,
  description,
  cost,
  onClick,
  buttonLabel,
}: {
  provider: string;
  name: string;
  description: string;
  cost: string;
  onClick: () => void;
  buttonLabel: string;
}) {
  return (
    <div className="bg-surface-0/30 border border-surface-0/50 rounded-xl p-4">
      <div className="flex items-center gap-3 mb-3">
        <ProviderLogo provider={provider} size={24} />
        <div className="flex-1">
          <p className="text-sm font-medium text-text">{name}</p>
          <p className="text-xs text-overlay-0">{description}</p>
        </div>
        <span className="text-xs font-semibold text-accent-yellow">{cost}</span>
      </div>
      <button
        onClick={onClick}
        className="w-full py-2 rounded-lg text-xs font-medium
                   bg-surface-0 text-subtext-0 hover:bg-surface-1 transition-colors"
      >
        {buttonLabel}
      </button>
    </div>
  );
}

function MiniProviderCard({ provider, name }: { provider: string; name: string }) {
  return (
    <div className="bg-surface-0/20 border border-surface-0/30 rounded-lg p-3 flex items-center gap-2
                    opacity-60 cursor-default" title="Configure in Settings after setup">
      <ProviderLogo provider={provider} size={20} />
      <span className="text-xs text-overlay-0">{name}</span>
    </div>
  );
}
