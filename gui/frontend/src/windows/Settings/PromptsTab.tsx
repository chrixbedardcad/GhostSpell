import { useState, useEffect, useCallback } from "react";
import { goCall } from "@/bridge";
import { Badge } from "@/components/ui/Badge";

interface Prompt {
  name: string;
  prompt: string;
  icon: string;
  llm: string;
  timeout_ms: number;
  display_mode: string;
  vision: boolean;
}

/**
 * Prompts tab — list of prompts with inline editing.
 * Zen: clean cards, collapsible editors, no clutter.
 */
export function PromptsTab() {
  const [prompts, setPrompts] = useState<Prompt[]>([]);
  const [activeIdx, setActiveIdx] = useState(0);
  const [expandedIdx, setExpandedIdx] = useState<number | null>(null);
  const [modelLabels, setModelLabels] = useState<string[]>([]);
  const [status, setStatus] = useState("");

  const loadPrompts = useCallback(async () => {
    const raw = await goCall("getConfig");
    if (!raw) return;
    try {
      const cfg = JSON.parse(raw);
      setPrompts(cfg.prompts || []);
      setActiveIdx(cfg.active_prompt || 0);
      // Extract model labels from config
      const labels = Object.keys(cfg.models || {});
      setModelLabels(labels);
    } catch { /* ignore */ }
  }, []);

  useEffect(() => { loadPrompts(); }, [loadPrompts]);

  async function savePrompt(idx: number, p: Prompt) {
    const timeoutSec = Math.round((p.timeout_ms || 30000) / 1000);
    await goCall("savePrompt", idx, p.name, p.prompt, p.llm, p.icon, timeoutSec, p.display_mode, p.vision);
    setStatus("Saved");
    setTimeout(() => setStatus(""), 2000);
    loadPrompts();
  }

  async function deletePrompt(idx: number) {
    await goCall("deletePrompt", idx);
    setExpandedIdx(null);
    loadPrompts();
  }

  async function movePrompt(idx: number, direction: number) {
    await goCall("movePrompt", idx, idx + direction);
    loadPrompts();
  }

  return (
    <div className="space-y-4">
      {/* Status */}
      {status && (
        <div className="text-xs text-accent-green text-right">{status}</div>
      )}

      {/* Prompt list */}
      {prompts.map((p, idx) => (
        <div key={idx} className="bg-surface-0/30 border border-surface-0/50 rounded-xl overflow-hidden">
          {/* Header row — always visible */}
          <button
            onClick={() => setExpandedIdx(expandedIdx === idx ? null : idx)}
            className="w-full flex items-center gap-3 px-4 py-3 text-left hover:bg-surface-0/20 transition-colors"
          >
            <span className="text-lg shrink-0">{p.icon || "📝"}</span>
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2">
                <span className="text-sm font-medium text-text">{p.name}</span>
                {idx === activeIdx && <Badge variant="active" />}
                {p.vision && <Badge variant="vision" />}
                {p.display_mode === "popup" && (
                  <span className="text-[10px] text-overlay-0 bg-surface-0 px-1.5 py-0.5 rounded">popup</span>
                )}
              </div>
              {p.llm && (
                <p className="text-[11px] text-overlay-0 mt-0.5">LLM: {p.llm}</p>
              )}
            </div>

            {/* Reorder buttons */}
            <div className="flex gap-1 shrink-0" onClick={(e) => e.stopPropagation()}>
              {idx > 0 && (
                <button onClick={() => movePrompt(idx, -1)}
                  className="text-overlay-0 hover:text-subtext-0 text-xs px-1">↑</button>
              )}
              {idx < prompts.length - 1 && (
                <button onClick={() => movePrompt(idx, 1)}
                  className="text-overlay-0 hover:text-subtext-0 text-xs px-1">↓</button>
              )}
            </div>

            <svg width="12" height="12" viewBox="0 0 12 12"
              className={`text-overlay-0 transition-transform shrink-0 ${expandedIdx === idx ? "rotate-180" : ""}`}>
              <path d="M3 4.5L6 7.5L9 4.5" stroke="currentColor" strokeWidth="1.5" fill="none" strokeLinecap="round"/>
            </svg>
          </button>

          {/* Editor — expanded */}
          {expandedIdx === idx && (
            <PromptEditor
              prompt={p}
              modelLabels={modelLabels}
              onSave={(updated) => savePrompt(idx, updated)}
              onDelete={() => deletePrompt(idx)}
            />
          )}
        </div>
      ))}

      {/* Add prompt */}
      <button
        onClick={async () => {
          await goCall("addPrompt");
          loadPrompts();
          setExpandedIdx(prompts.length);
        }}
        className="w-full py-3 rounded-xl border border-dashed border-surface-1 text-sm text-overlay-0
                   hover:text-subtext-0 hover:border-surface-2 transition-colors"
      >
        + Add Prompt
      </button>
    </div>
  );
}

function PromptEditor({
  prompt: initial,
  modelLabels,
  onSave,
  onDelete,
}: {
  prompt: Prompt;
  modelLabels: string[];
  onSave: (p: Prompt) => void;
  onDelete: () => void;
}) {
  const [p, setP] = useState({ ...initial });

  function update(field: Partial<Prompt>) {
    setP((prev) => ({ ...prev, ...field }));
  }

  return (
    <div className="px-4 pb-4 space-y-3 border-t border-surface-0/30">
      {/* Name + Icon */}
      <div className="flex gap-3 pt-3">
        <input
          value={p.icon}
          onChange={(e) => update({ icon: e.target.value })}
          className="w-12 bg-crust border border-surface-0 rounded-lg px-2 py-1.5
                     text-center text-lg focus:outline-none focus:border-accent-blue/50"
          title="Emoji icon"
        />
        <input
          value={p.name}
          onChange={(e) => update({ name: e.target.value })}
          className="flex-1 bg-crust border border-surface-0 rounded-lg px-3 py-1.5
                     text-sm text-text focus:outline-none focus:border-accent-blue/50"
          placeholder="Prompt name"
        />
      </div>

      {/* Prompt text */}
      <textarea
        value={p.prompt}
        onChange={(e) => update({ prompt: e.target.value })}
        className="w-full min-h-[100px] bg-crust border border-surface-0 rounded-lg p-3
                   text-sm text-text placeholder:text-overlay-0 resize-y
                   focus:outline-none focus:border-accent-blue/50 font-mono"
        placeholder="Enter your prompt instructions..."
      />

      {/* Settings row */}
      <div className="flex flex-wrap gap-3 items-center">
        {/* LLM override */}
        <div className="flex items-center gap-2">
          <label className="text-xs text-overlay-0">LLM</label>
          <select
            value={p.llm}
            onChange={(e) => update({ llm: e.target.value })}
            className="bg-crust border border-surface-0 rounded-lg px-2 py-1 text-xs text-subtext-0
                       focus:outline-none"
          >
            <option value="">Default</option>
            {modelLabels.map((l) => (
              <option key={l} value={l}>{l}</option>
            ))}
          </select>
        </div>

        {/* Display mode */}
        <div className="flex items-center gap-2">
          <label className="text-xs text-overlay-0">Output</label>
          <select
            value={p.display_mode || "replace"}
            onChange={(e) => update({ display_mode: e.target.value })}
            className="bg-crust border border-surface-0 rounded-lg px-2 py-1 text-xs text-subtext-0
                       focus:outline-none"
          >
            <option value="">Replace text</option>
            <option value="popup">Popup window</option>
          </select>
        </div>

        {/* Vision toggle */}
        <div className="flex items-center gap-2">
          <label className="text-xs text-overlay-0">Vision</label>
          <button
            onClick={() => update({ vision: !p.vision })}
            className={`relative w-8 h-[18px] rounded-full transition-colors ${
              p.vision ? "bg-accent-sky" : "bg-surface-1"
            }`}
          >
            <span className={`absolute top-[2px] w-[14px] h-[14px] rounded-full bg-text transition-transform ${
              p.vision ? "translate-x-[14px]" : "translate-x-[2px]"
            }`} />
          </button>
        </div>
      </div>

      {/* Actions */}
      <div className="flex gap-2 pt-1">
        <button
          onClick={() => onSave(p)}
          className="px-4 py-1.5 rounded-lg text-xs font-medium
                     bg-accent-blue/15 text-accent-blue hover:bg-accent-blue/25 transition-colors"
        >
          Save
        </button>
        <button
          onClick={onDelete}
          className="px-4 py-1.5 rounded-lg text-xs font-medium
                     bg-accent-red/10 text-accent-red hover:bg-accent-red/20 transition-colors"
        >
          Delete
        </button>
      </div>
    </div>
  );
}
