import { useState, useEffect } from "react";
import { goCall } from "@/bridge";
import { usePlatform } from "@/hooks/usePlatform";

/**
 * Hotkeys tab — display and configure keyboard shortcuts.
 * Zen: clean keycap display, minimal controls.
 */
export function HotkeysTab() {
  const platform = usePlatform();
  const [actionKey, setActionKey] = useState("Ctrl+G");
  const [cycleKey, setCycleKey] = useState("Ctrl+Shift+T");
  const [capturing, setCapturing] = useState<"action" | "cycle" | null>(null);

  useEffect(() => {
    goCall("getConfig").then((raw) => {
      if (!raw) return;
      try {
        const cfg = JSON.parse(raw);
        setActionKey(cfg.hotkeys?.action || "Ctrl+G");
        setCycleKey(cfg.hotkeys?.cycle_prompt || "Ctrl+Shift+T");
      } catch { /* ignore */ }
    });
  }, []);

  function formatKey(key: string): string {
    if (platform === "darwin") {
      return key.replace("Ctrl", "⌘").replace("Shift", "⇧").replace("Alt", "⌥");
    }
    return key;
  }

  return (
    <div className="space-y-8">
      <section>
        <h2 className="text-sm font-medium text-subtext-1 mb-4 tracking-wide uppercase">
          Keyboard Shortcuts
        </h2>

        <div className="space-y-3">
          {/* Action hotkey */}
          <div className="bg-surface-0/30 rounded-xl p-5 flex items-center justify-between">
            <div>
              <p className="text-sm text-text">Activate GhostSpell</p>
              <p className="text-xs text-overlay-0 mt-0.5">Process selected text with the active prompt</p>
            </div>
            <Keycap keys={formatKey(actionKey)} />
          </div>

          {/* Cycle prompt hotkey */}
          <div className="bg-surface-0/30 rounded-xl p-5 flex items-center justify-between">
            <div>
              <p className="text-sm text-text">Cycle Prompt</p>
              <p className="text-xs text-overlay-0 mt-0.5">Switch to the next prompt without opening settings</p>
            </div>
            <Keycap keys={formatKey(cycleKey)} />
          </div>
        </div>
      </section>

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
            <span className="text-subtext-0">Cycle prompt</span>
          </div>
          <div className="h-px bg-surface-0/50" />
          <div className="flex justify-between text-overlay-1">
            <span>Right-click ghost</span>
            <span className="text-subtext-0">Prompt menu</span>
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
