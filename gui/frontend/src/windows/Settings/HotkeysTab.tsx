import { useState, useEffect } from "react";
import { goCall } from "@/bridge";
import { usePlatform } from "@/hooks/usePlatform";

interface Prompt {
  name: string;
  icon?: string;
  hotkey?: string;
  disabled?: boolean;
}

/**
 * Hotkeys tab — display and configure keyboard shortcuts.
 * Global hotkeys (action + cycle) at top, per-skill hotkeys below.
 */
export function HotkeysTab() {
  const platform = usePlatform();
  const [actionKey, setActionKey] = useState("Ctrl+G");
  const [cycleKey, setCycleKey] = useState("Ctrl+Shift+T");
  const [prompts, setPrompts] = useState<Prompt[]>([]);
  const [capturing, setCapturing] = useState<string | null>(null);

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
      return key.replace("Ctrl", "⌘").replace("Shift", "⇧").replace("Alt", "⌥");
    }
    return key;
  }

  // Capture a key combo from the user.
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

      // Build combo string.
      const parts: string[] = [];
      if (e.ctrlKey || e.metaKey) parts.push("Ctrl");
      if (e.shiftKey) parts.push("Shift");
      if (e.altKey) parts.push("Alt");

      const key = e.key.length === 1 ? e.key.toUpperCase() : e.key;
      if (!["Control", "Shift", "Alt", "Meta"].includes(e.key)) {
        parts.push(key);
      } else {
        return; // modifier-only press, keep capturing
      }

      const combo = parts.join("+");
      setCapturing(null);
      window.removeEventListener("keydown", handler);

      // Save the hotkey.
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

  return (
    <div className="space-y-8">
      {/* Global hotkeys */}
      <section>
        <h2 className="text-sm font-medium text-subtext-1 mb-4 tracking-wide uppercase">
          Global Shortcuts
        </h2>
        <div className="space-y-3">
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

      {/* Per-skill hotkeys */}
      <section>
        <h2 className="text-sm font-medium text-subtext-1 mb-4 tracking-wide uppercase">
          Skill Shortcuts
        </h2>
        <p className="text-xs text-overlay-0 mb-3">
          Assign a direct hotkey to any skill. Pressing it selects and triggers the skill instantly.
        </p>
        <div className="space-y-3">
          {prompts.map((p, i) => {
            if (p.disabled) return null;
            const id = `skill_${i}`;
            return (
              <div key={i} className="bg-surface-0/30 rounded-xl px-4 py-3.5 flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <span className="text-sm">{p.icon || "⚡"}</span>
                  <span className="text-sm text-text">{p.name}</span>
                </div>
                <div className="flex items-center gap-2">
                  {p.hotkey ? (
                    <>
                      <Keycap keys={formatKey(p.hotkey)} />
                      <button
                        onClick={() => clearSkillHotkey(i)}
                        className="text-xs text-overlay-0 hover:text-red px-1"
                        title="Clear hotkey"
                      >✕</button>
                    </>
                  ) : capturing === id ? (
                    <span className="text-xs text-mauve animate-pulse">Press a key combo...</span>
                  ) : (
                    <button
                      onClick={() => startCapture(id)}
                      className="text-xs text-overlay-0 hover:text-mauve px-2 py-1 rounded border border-surface-0/50 hover:border-mauve/30"
                    >
                      Set hotkey
                    </button>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      </section>

      {/* Quick Reference */}
      <section>
        <h2 className="text-sm font-medium text-subtext-1 mb-4 tracking-wide uppercase">
          Quick Reference
        </h2>
        <div className="bg-surface-0/30 rounded-xl p-5 space-y-3 text-xs">
          <div className="flex justify-between text-overlay-1">
            <span>Cancel active request</span>
            <span className="text-subtext-0">Press {formatKey(actionKey)} again</span>
          </div>
          <div className="h-px bg-surface-0/50" />
          <div className="flex justify-between text-overlay-1">
            <span>Undo result</span>
            <span className="text-subtext-0">{formatKey("Ctrl+Z")}</span>
          </div>
          <div className="h-px bg-surface-0/50" />
          <div className="flex justify-between text-overlay-1">
            <span>Click ghost indicator</span>
            <span className="text-subtext-0">Cycle skill</span>
          </div>
          <div className="h-px bg-surface-0/50" />
          <div className="flex justify-between text-overlay-1">
            <span>Right-click ghost</span>
            <span className="text-subtext-0">Skill menu</span>
          </div>
          <div className="h-px bg-surface-0/50" />
          <div className="flex justify-between text-overlay-1">
            <span>Double-click ghost</span>
            <span className="text-subtext-0">Open settings</span>
          </div>
        </div>
      </section>
    </div>
  );
}

/** Row for a single hotkey binding */
function HotkeyRow({ label, description, keys, capturing, onCapture }: {
  label: string;
  description: string;
  keys: string;
  capturing: boolean;
  onCapture: () => void;
}) {
  return (
    <div className="bg-surface-0/30 rounded-xl p-5 flex items-center justify-between">
      <div>
        <p className="text-sm text-text">{label}</p>
        <p className="text-xs text-overlay-0 mt-0.5">{description}</p>
      </div>
      {capturing ? (
        <span className="text-xs text-mauve animate-pulse">Press a key combo...</span>
      ) : (
        <button onClick={onCapture} className="cursor-pointer hover:opacity-80">
          <Keycap keys={keys} />
        </button>
      )}
    </div>
  );
}

/** Keycap — styled keyboard key display */
function Keycap({ keys }: { keys: string }) {
  const parts = keys.split("+").map((k) => k.trim());
  return (
    <div className="flex items-center gap-1">
      {parts.map((key, i) => (
        <span key={i}>
          {i > 0 && <span className="text-overlay-0 text-[10px] mx-0.5">+</span>}
          <span className="inline-flex items-center justify-center min-w-[28px] px-2 py-1
                         rounded-md bg-crust border border-surface-0 text-xs font-mono
                         text-subtext-1 shadow-sm">
            {key}
          </span>
        </span>
      ))}
    </div>
  );
}
