import { useState, useEffect } from "react";
import { goCall } from "@/bridge";

/**
 * Result popup — shows LLM output for vision/define prompts.
 * Zen: clean, readable, focused on content.
 */
export function ResultWindow() {
  const [text, setText] = useState("");
  const [promptName, setPromptName] = useState("");
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    goCall("getResultText").then((raw) => {
      if (raw) setText(raw);
    });
    // Read prompt name from URL params
    const params = new URLSearchParams(window.location.search);
    setPromptName(params.get("prompt") || "Result");
  }, []);

  function copyToClipboard() {
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }

  return (
    <div className="h-full flex flex-col bg-base">
      {/* Header */}
      <div className="flex items-center justify-between px-5 py-3 border-b border-surface-0/40 shrink-0"
        style={{ ["--wails-draggable" as string]: "drag" }}>
        <div className="flex items-center gap-2">
          <img src="/ghost-icon.png" alt="" className="w-5 h-5 opacity-70" />
          <span className="text-xs font-medium text-subtext-1">{promptName}</span>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={copyToClipboard}
            className="px-2.5 py-1 rounded-md text-[11px] text-overlay-0
                       hover:text-subtext-0 hover:bg-surface-0/50 transition-colors"
          >
            {copied ? "✓ Copied" : "Copy"}
          </button>
          <button
            onClick={() => window.wails.Window.Close()}
            className="text-overlay-0 hover:text-accent-red px-1.5 py-0.5 rounded
                       hover:bg-surface-0/50 transition-colors text-sm"
            style={{ ["--wails-draggable" as string]: "no-drag" }}
          >
            ✕
          </button>
        </div>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto p-5">
        <div className="text-sm text-text leading-relaxed whitespace-pre-wrap font-sans">
          {text || <span className="text-overlay-0 italic">No result.</span>}
        </div>
      </div>
    </div>
  );
}
