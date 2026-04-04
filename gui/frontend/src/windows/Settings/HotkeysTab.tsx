import { useState, useEffect } from "react";
import { goCall } from "@/bridge";
import { usePlatform } from "@/hooks/usePlatform";

interface Prompt {
  name: string;
  icon?: string;
  hotkey?: string;
  disabled?: boolean;
}

export function HotkeysTab() {
  const platform = usePlatform();
  const [actionKey, setActionKey] = useState("Ctrl+G");
  const [cycleKey, setCycleKey] = useState("Ctrl+Shift+T");
  const [prompts, setPrompts] = useState<Prompt[]>([]);
  const [capturing, setCapturing] = useState<string | null>(null);
  const [showAddPicker, setShowAddPicker] = useState(false);

  function loadConfig() {
    goCall("getConfig").then((raw) => {
      if (!raw) return;
      try {
        const cfg = JSON.parse(raw);
        setActionKey(cfg.hotkeys?.action || "Ctrl+G");
        setCycleKey(cfg.hotkeys?.cycle_prompt || "Ctrl+Shift+T");
        setPrompts(cfg.prompts || []);
      } catch { /* ignore */ }
    });
  }

  useEffect(() => { loadConfig(); }, []);

  function formatKey(key: string): string {
    if (!key) return "";
    if (platform === "darwin") {
      return key.replace("Ctrl", "\u2318").replace("Shift", "\u21E7").replace("Alt", "\u2325");
    }
    return key;
  }

  function startCapture(id: string) {
    setCapturing(id);
    const handler = (e: KeyboardEvent) => {
      e.preventDefault();
      e.stopPropagation();
      if (e.key === "Escape") {
        setCapturing(null);
        window.removeEventListener("keydown", handler);
        return;
      }
      const parts: string[] = [];
      if (e.ctrlKey || e.metaKey) parts.push("Ctrl");
      if (e.shiftKey) parts.push("Shift");
      if (e.altKey) parts.push("Alt");
      const key = e.key.length === 1 ? e.key.toUpperCase() : e.key;
      if (!["Control", "Shift", "Alt", "Meta"].includes(e.key)) {
        parts.push(key);
      } else {
        return;
      }
      const combo = parts.join("+");
      setCapturing(null);
      window.removeEventListener("keydown", handler);
      if (id === "action") {
        goCall("setHotkey", "action", combo);
        setActionKey(combo);
      } else if (id === "cycle") {
        goCall("setHotkey", "cycle_prompt", combo);
        setCycleKey(combo);
      } else if (id.startsWith("skill_")) {
        const idx = parseInt(id.replace("skill_", ""));
        goCall("setPromptHotkey", idx, combo).then(() => loadConfig());
      }
    };
    window.addEventListener("keydown", handler);
  }

  function clearSkillHotkey(idx: number) {
    goCall("setPromptHotkey", idx, "").then(() => loadConfig());
  }

  // Skills with hotkeys assigned.
  const assignedSkills = prompts
    .map((p, i) => ({ ...p, idx: i }))
    .filter((p) => !p.disabled && p.hotkey);

  // Skills without hotkeys (for the Add picker).
  const unassignedSkills = prompts
    .map((p, i) => ({ ...p, idx: i }))
    .filter((p) => !p.disabled && !p.hotkey);

  return (
    <div className="space-y-8">
      {/* Global hotkeys */}
      <section>
        <h2 className="text-[11px] font-semibold text-overlay-0 mb-4 uppercase tracking-widest">
          Global Shortcuts
        </h2>
        <div className="space-y-4">
          <HotkeyRow
            label="Activate GhostSpell"
            description="Process selected text with the active skill"
            keys={formatKey(actionKey)}
            capturing={capturing === "action"}
            onCapture={() => startCapture("action")}
          />
          <HotkeyRow
            label="Cycle Skill"
            description="Switch to the next skill"
            keys={formatKey(cycleKey)}
            capturing={capturing === "cycle"}
            onCapture={() => startCapture("cycle")}
          />
        </div>
      </section>

      {/* Skill shortcuts — only show assigned + add button */}
      <section>
        <h2 className="text-[11px] font-semibold text-overlay-0 mb-4 uppercase tracking-widest">
          Skill Shortcuts
        </h2>
        <p className="text-[12px] text-overlay-0 mb-5">
          Assign a direct hotkey to trigger a skill instantly — no cycling needed.
        </p>

        {/* Assigned skills */}
        {assignedSkills.length > 0 && (
          <div className="space-y-3 mb-5">
            {assignedSkills.map((p) => {
              const id = `skill_${p.idx}`;
              return (
                <div key={p.idx} className="bg-surface-0/30 rounded-xl px-5 py-4 flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <span className="text-[18px]">{p.icon || "\u26A1"}</span>
                    <span className="text-[14px] font-medium text-text">{p.name}</span>
                  </div>
                  <div className="flex items-center gap-3">
                    {capturing === id ? (
                      <span className="text-[12px] text-accent-mauve animate-pulse">Press a key combo...</span>
                    ) : (
                      <button onClick={() => startCapture(id)} className="cursor-pointer hover:opacity-80">
                        <Keycap keys={formatKey(p.hotkey!)} />
                      </button>
                    )}
                    <button
                      onClick={() => clearSkillHotkey(p.idx)}
                      className="text-[12px] text-overlay-0 hover:text-accent-red px-2 py-1 rounded
                                 hover:bg-red-500/10 transition-colors"
                      title="Remove hotkey"
                    >{"\u2715"}</button>
                  </div>
                </div>
              );
            })}
          </div>
        )}

        {/* Add button */}
        {unassignedSkills.length > 0 && (
          <div className="relative">
            <button
              onClick={() => setShowAddPicker(!showAddPicker)}
              className="px-4 py-2.5 rounded-xl text-[13px] font-medium
                         bg-surface-0/40 text-subtext-0 hover:text-text hover:bg-surface-0/60
                         transition-colors"
            >
              + Add Skill Shortcut
            </button>

            {showAddPicker && (
              <div className="mt-2 bg-mantle border border-surface-0 rounded-xl overflow-hidden shadow-lg">
                {unassignedSkills.map((p) => {
                  const id = `skill_${p.idx}`;
                  return (
                    <button
                      key={p.idx}
                      onClick={() => {
                        setShowAddPicker(false);
                        startCapture(id);
                      }}
                      className="w-full flex items-center gap-3 px-5 py-3.5 text-left
                                 hover:bg-surface-0/40 transition-colors border-b border-surface-0/20 last:border-0"
                    >
                      <span className="text-[16px]">{p.icon || "\u26A1"}</span>
                      <span className="text-[13px] text-text">{p.name}</span>
                      <span className="ml-auto text-[11px] text-overlay-0">Click to assign hotkey</span>
                    </button>
                  );
                })}
              </div>
            )}
          </div>
        )}

        {assignedSkills.length === 0 && unassignedSkills.length === 0 && (
          <p className="text-[12px] text-overlay-0 italic">No skills available. Add skills in the Skills tab.</p>
        )}
      </section>

      {/* Quick Reference */}
      <section>
        <h2 className="text-[11px] font-semibold text-overlay-0 mb-4 uppercase tracking-widest">
          Quick Reference
        </h2>
        <div className="bg-surface-0/30 rounded-xl px-6 py-5">
          <div className="space-y-0">
            <RefRow left="Cancel active request" right={"Press " + formatKey(actionKey) + " again"} />
            <RefRow left="Undo result" right={formatKey("Ctrl+Z")} />
            <RefRow left="Click ghost indicator" right="Cycle skill" />
            <RefRow left="Right-click ghost" right="Skill menu" />
            <RefRow left="Double-click ghost" right="Open settings" />
          </div>
        </div>
      </section>
    </div>
  );
}

function HotkeyRow({ label, description, keys, capturing, onCapture }: {
  label: string;
  description: string;
  keys: string;
  capturing: boolean;
  onCapture: () => void;
}) {
  return (
    <div className="bg-surface-0/30 rounded-xl px-6 py-5 flex items-center justify-between">
      <div>
        <p className="text-[14px] font-medium text-text">{label}</p>
        <p className="text-[12px] text-overlay-0 mt-1">{description}</p>
      </div>
      {capturing ? (
        <span className="text-[12px] text-accent-mauve animate-pulse">Press a key combo...</span>
      ) : (
        <button onClick={onCapture} className="cursor-pointer hover:opacity-80">
          <Keycap keys={keys} />
        </button>
      )}
    </div>
  );
}

function RefRow({ left, right }: { left: string; right: string }) {
  return (
    <div className="flex justify-between py-3 border-b border-surface-0/30 last:border-0">
      <span className="text-[13px] text-overlay-1">{left}</span>
      <span className="text-[13px] text-subtext-0">{right}</span>
    </div>
  );
}

function Keycap({ keys }: { keys: string }) {
  const parts = keys.split("+").map((k) => k.trim());
  return (
    <div className="flex items-center gap-1.5">
      {parts.map((key, i) => (
        <span key={i}>
          {i > 0 && <span className="text-overlay-0 text-[10px] mx-0.5">+</span>}
          <span className="inline-flex items-center justify-center min-w-[32px] px-2.5 py-1.5
                         rounded-lg bg-crust border border-surface-0 text-[12px] font-mono
                         text-subtext-1 shadow-sm">
            {key}
          </span>
        </span>
      ))}
    </div>
  );
}
