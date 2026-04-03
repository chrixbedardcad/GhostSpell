import { useState, useEffect, useCallback, useRef } from "react";
import { createPortal } from "react-dom";
import { goCall } from "@/bridge";
import { Badge } from "@/components/ui/Badge";
import EmojiPicker, { Theme, EmojiClickData } from "emoji-picker-react";

interface Prompt {
  name: string;
  prompt: string;
  icon: string;
  llm: string;
  timeout_ms: number;
  display_mode: string;
  vision: boolean;
  voice: boolean;
  voice_mode: string;
  disabled: boolean;
}

/**
 * Skills tab — list of skills (prompts) with inline editing.
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
    await goCall("savePrompt", idx, p.name, p.prompt, p.llm, p.icon, timeoutSec, p.display_mode, p.vision, p.voice, p.voice_mode || "", p.disabled);
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
        <div key={idx} className={`bg-surface-0/30 border border-surface-0/50 rounded-xl overflow-hidden ${p.disabled ? "opacity-50" : ""}`}>
          {/* Header row — always visible */}
          <button
            onClick={() => setExpandedIdx(expandedIdx === idx ? null : idx)}
            className="w-full flex items-center gap-3 px-4 py-3 text-left hover:bg-surface-0/20 transition-colors"
          >
            <span className="text-lg shrink-0">{p.icon || "📝"}</span>
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2">
                <span className="text-sm font-medium text-text">{p.name}</span>
                {idx === activeIdx && !p.disabled && <Badge variant="active" />}
                {p.disabled && <Badge variant="disabled" />}
                {p.vision && <Badge variant="vision" />}
                {p.voice && <Badge variant="voice" />}
                {p.display_mode === "popup" && (
                  <span className="text-[10px] text-overlay-0 bg-surface-0 px-1.5 py-0.5 rounded">popup</span>
                )}
                {p.display_mode === "append" && (
                  <span className="text-[10px] text-overlay-0 bg-surface-0 px-1.5 py-0.5 rounded">append</span>
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
          const newPrompt: Prompt = {
            name: "New Skill",
            prompt: "Enter your skill instructions...",
            icon: "📝",
            llm: "",
            timeout_ms: 30000,
            display_mode: "",
            vision: false,
            voice: false,
            voice_mode: "",
            disabled: false,
          };
          await savePrompt(-1, newPrompt);
          setExpandedIdx(prompts.length);
        }}
        className="w-full py-3 rounded-xl border border-dashed border-surface-1 text-sm text-overlay-0
                   hover:text-subtext-0 hover:border-surface-2 transition-colors"
      >
        + Add Skill
      </button>
    </div>
  );
}

// emoji-picker-react is dynamically imported only when user clicks the icon button.

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
  const [showEmojiPicker, setShowEmojiPicker] = useState(false);
  const emojiRef = useRef<HTMLDivElement>(null);

  function update(field: Partial<Prompt>) {
    setP((prev) => ({ ...prev, ...field }));
  }

  // Close emoji picker on outside click.
  useEffect(() => {
    if (!showEmojiPicker) return;
    function onClickOutside(e: MouseEvent) {
      if (emojiRef.current && !emojiRef.current.contains(e.target as Node)) {
        setShowEmojiPicker(false);
      }
    }
    document.addEventListener("mousedown", onClickOutside);
    return () => document.removeEventListener("mousedown", onClickOutside);
  }, [showEmojiPicker]);

  function onEmojiClick(data: EmojiClickData) {
    update({ icon: data.emoji });
    setShowEmojiPicker(false);
  }

  return (
    <div className="px-4 pb-4 space-y-3 border-t border-surface-0/30">
      {/* Enable/Disable toggle */}
      <div className="flex items-center justify-between pt-3 pb-2 border-b border-surface-0/30">
        <div className="flex items-center gap-2">
          <span className={`text-[13px] font-medium ${p.disabled ? "text-overlay-0" : "text-accent-green"}`}>
            {p.disabled ? "Disabled" : "Enabled"}
          </span>
        </div>
        <button
          onClick={() => update({ disabled: !p.disabled })}
          className={`relative shrink-0 transition-colors duration-200 ${p.disabled ? "bg-surface-2" : "bg-accent-green/60"}`}
          style={{ width: 44, height: 24, borderRadius: 12 }}
        >
          <span className="absolute bg-white rounded-full shadow-sm" style={{ width: 20, height: 20, top: 2, left: p.disabled ? 2 : 22, transition: "left 200ms ease" }} />
        </button>
      </div>

      {/* Name + Icon */}
      <div className="flex gap-3">
        <div ref={emojiRef}>
          <button
            onClick={() => setShowEmojiPicker(!showEmojiPicker)}
            className="w-12 h-[38px] bg-crust border border-surface-0 rounded-lg
                       text-center text-xl hover:border-accent-blue/50 transition-colors
                       focus:outline-none focus:border-accent-blue/50 cursor-pointer"
            title="Pick an emoji"
          >
            {p.icon || "\uD83D\uDCDD"}
          </button>
          {showEmojiPicker && createPortal(
            <div
              className="fixed inset-0 z-[9999] flex items-center justify-center"
              style={{ background: "rgba(0,0,0,0.5)" }}
              onClick={(e) => { if (e.target === e.currentTarget) setShowEmojiPicker(false); }}
            >
              <div onClick={(e) => e.stopPropagation()}>
                <EmojiPicker
                  onEmojiClick={onEmojiClick}
                  theme={Theme.DARK}
                  width={350}
                  height={420}
                  searchPlaceHolder="Search emoji..."
                  previewConfig={{ showPreview: false }}
                  skinTonesDisabled
                  lazyLoadEmojis
                />
              </div>
            </div>,
            document.body
          )}
        </div>
        <input
          value={p.name}
          onChange={(e) => update({ name: e.target.value })}
          className="flex-1 bg-crust border border-surface-0 rounded-lg px-3 py-1.5
                     text-sm text-text focus:outline-none focus:border-accent-blue/50"
          placeholder="Skill name"
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

        {/* Output mode */}
        <div className="flex items-center gap-2">
          <label className="text-xs text-overlay-0">Output</label>
          <select
            value={p.display_mode || "replace"}
            onChange={(e) => update({ display_mode: e.target.value === "replace" ? "" : e.target.value })}
            className="bg-crust border border-surface-0 rounded-lg px-2 py-1 text-xs text-subtext-0
                       focus:outline-none"
          >
            <option value="replace">Replace</option>
            <option value="append">Append</option>
            <option value="popup">Popup</option>
          </select>
        </div>

        {/* Input mode */}
        <div className="flex items-center gap-2">
          <label className="text-xs text-overlay-0">Input</label>
          <select
            value={
              p.voice && p.voice_mode === "dictation" ? "voice-dictation"
              : p.voice ? "voice-skill"
              : p.vision ? "screenshot"
              : "text"
            }
            onChange={(e) => {
              const v = e.target.value;
              update({
                voice: v === "voice-skill" || v === "voice-dictation",
                voice_mode: v === "voice-dictation" ? "dictation" : v === "voice-skill" ? "skill" : "",
                vision: v === "screenshot",
              });
            }}
            className="bg-crust border border-surface-0 rounded-lg px-2 py-1 text-xs text-subtext-0
                       focus:outline-none"
          >
            <option value="text">Text</option>
            <option value="voice-skill">Voice → LLM</option>
            <option value="voice-dictation">Voice → Direct</option>
            <option value="screenshot">Screenshot</option>
          </select>
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
