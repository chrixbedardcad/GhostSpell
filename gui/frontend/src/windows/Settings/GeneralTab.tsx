import { useState, useEffect, useRef } from "react";
import { goCall } from "@/bridge";
import { usePlatform } from "@/hooks/usePlatform";

/**
 * Custom dropdown — zen, minimal, smooth animation.
 * Replaces native <select> that looks like 1995 on macOS.
 */
function Dropdown({
  value,
  options,
  onChange,
  placeholder = "Select...",
}: {
  value: string;
  options: { value: string; label: string }[];
  onChange: (value: string) => void;
  placeholder?: string;
}) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function onClickOutside(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener("mousedown", onClickOutside);
    return () => document.removeEventListener("mousedown", onClickOutside);
  }, []);

  const selected = options.find((o) => o.value === value);

  return (
    <div ref={ref} className="relative">
      <button
        onClick={() => setOpen(!open)}
        className="w-full flex items-center justify-between gap-2 px-3 py-2
                   bg-crust border border-surface-0 rounded-lg text-sm text-text
                   hover:border-surface-1 focus:border-accent-blue/50 focus:outline-none
                   transition-colors"
      >
        <span className={selected ? "text-text" : "text-overlay-0"}>
          {selected?.label ?? placeholder}
        </span>
        <svg width="12" height="12" viewBox="0 0 12 12" className={`text-overlay-0 transition-transform ${open ? "rotate-180" : ""}`}>
          <path d="M3 4.5L6 7.5L9 4.5" stroke="currentColor" strokeWidth="1.5" fill="none" strokeLinecap="round"/>
        </svg>
      </button>

      {open && (
        <div className="absolute z-50 mt-1 w-full bg-mantle border border-surface-0 rounded-lg
                        shadow-none overflow-hidden animate-in fade-in slide-in-from-top-1">
          {options.map((opt) => (
            <button
              key={opt.value}
              onClick={() => { onChange(opt.value); setOpen(false); }}
              className={`w-full text-left px-3 py-2 text-sm transition-colors
                ${opt.value === value
                  ? "text-accent-blue bg-accent-blue/10"
                  : "text-subtext-0 hover:text-text hover:bg-surface-0/50"
                }`}
            >
              {opt.label}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

/**
 * Toggle row — label + description + switch.
 */
function ToggleRow({
  label,
  description,
  checked,
  onChange,
}: {
  label: string;
  description?: string;
  checked: boolean;
  onChange: (v: boolean) => void;
}) {
  return (
    <div className="flex items-center justify-between">
      <div>
        <p className="text-[13px] font-semibold text-text">{label}</p>
        {description && <p className="text-[11px] text-overlay-0/80 mt-1 leading-relaxed">{description}</p>}
      </div>
      <button
        onClick={() => onChange(!checked)}
        className={`relative shrink-0 transition-colors duration-200 ${
          checked ? "bg-accent-blue" : "bg-surface-2"
        }`}
        style={{ width: 44, height: 24, borderRadius: 12 }}
      >
        <span
          className="absolute bg-white rounded-full shadow-sm"
          style={{
            width: 20,
            height: 20,
            top: 2,
            left: checked ? 22 : 2,
            transition: "left 200ms ease",
          }}
        />
      </button>
    </div>
  );
}

/**
 * General tab — app preferences.
 * Zen: clean sections, custom dropdowns, toggle switches.
 */
export function GeneralTab() {
  const platform = usePlatform();
  const [sound, setSound] = useState(true);
  const [clipboard, setClipboard] = useState(false);
  const [maxChars, setMaxChars] = useState("2000");
  const [indicatorPos, setIndicatorPos] = useState("top-right");
  const [indicatorMode, setIndicatorMode] = useState("processing");
  const [hotkey, setHotkey] = useState("Ctrl+G");

  // API server state
  const [apiEnabled, setApiEnabled] = useState(false);
  const [apiAddr, setApiAddr] = useState("127.0.0.1:7878");
  const [apiRunning, setApiRunning] = useState(false);
  const [apiListenAddr, setApiListenAddr] = useState("");
  const [apiTesting, setApiTesting] = useState(false);
  const [apiTestResult, setApiTestResult] = useState("");

  function refreshAPIStatus() {
    goCall("getAPIStatus").then((raw) => {
      if (!raw) return;
      try {
        const st = JSON.parse(raw);
        setApiRunning(st.running);
        setApiListenAddr(st.addr || "");
        setApiEnabled(st.enabled);
        if (st.configured_addr) setApiAddr(st.configured_addr || "127.0.0.1:7878");
      } catch { /* ignore */ }
    });
  }

  useEffect(() => {
    goCall("getConfig").then((raw) => {
      if (!raw) return;
      try {
        const cfg = JSON.parse(raw);
        setSound(cfg.sound_enabled ?? true);
        setClipboard(cfg.preserve_clipboard ?? false);
        setMaxChars(String(cfg.max_input_chars || 2000));
        setIndicatorPos(cfg.indicator_position || "top-right");
        setIndicatorMode(cfg.indicator_mode || "processing");
        const hk = cfg.hotkeys?.action || "Ctrl+G";
        setHotkey(platform === "darwin" ? hk.replace("Ctrl", "⌘") : hk);
        setApiEnabled(cfg.api_enabled ?? false);
        setApiAddr(cfg.api_addr || "127.0.0.1:7878");
      } catch { /* ignore */ }
    });
    refreshAPIStatus();
  }, [platform]);

  async function saveField(method: string, ...args: unknown[]) {
    await goCall(method, ...args);
  }

  return (
    <div className="space-y-8">
      {/* Hotkey display */}
      <section>
        <h2 className="text-[11px] font-semibold text-overlay-0 mb-4 uppercase tracking-widest">
          Activation
        </h2>
        <div className="bg-surface-0/20 border border-surface-0/40 rounded-xl px-5 py-3.5 flex items-center gap-4">
          <div className="px-3 py-1.5 rounded-lg bg-crust border border-surface-0 text-sm font-mono text-accent-blue">
            {hotkey}
          </div>
          <p className="text-xs text-overlay-0">Select text and press to activate</p>
        </div>
      </section>

      {/* Sound & Clipboard */}
      <section>
        <h2 className="text-[11px] font-semibold text-overlay-0 mb-4 uppercase tracking-widest">
          Behavior
        </h2>
        <div className="space-y-4">
          <div className="bg-surface-0/20 border border-surface-0/40 rounded-xl px-5 py-3.5">
            <ToggleRow
              label="Sound Effects"
              description="Play sounds during processing"
              checked={sound}
              onChange={(v) => { setSound(v); saveField("setSoundEnabled", v); }}
            />
          </div>
          <div className="bg-surface-0/20 border border-surface-0/40 rounded-xl px-5 py-3.5">
            <ToggleRow
              label="Preserve Clipboard"
              description="Restore clipboard contents after paste"
              checked={clipboard}
              onChange={(v) => { setClipboard(v); saveField("setPreserveClipboard", v); }}
            />
          </div>
        </div>
      </section>

      {/* Input limit */}
      <section>
        <h2 className="text-[11px] font-semibold text-overlay-0 mb-4 uppercase tracking-widest">
          Input
        </h2>
        <div className="bg-surface-0/20 border border-surface-0/40 rounded-xl px-5 py-3.5">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-[13px] font-semibold text-text">Max Input Characters</p>
              <p className="text-xs text-overlay-0 mt-0.5">Limit text length sent to AI</p>
            </div>
            <div className="w-36">
              <Dropdown
                value={maxChars}
                onChange={(v) => { setMaxChars(v); saveField("setMaxInputChars", parseInt(v)); }}
                options={[
                  { value: "500", label: "500 chars" },
                  { value: "1000", label: "1,000 chars" },
                  { value: "2000", label: "2,000 chars" },
                  { value: "5000", label: "5,000 chars" },
                  { value: "10000", label: "10,000 chars" },
                  { value: "0", label: "No limit" },
                ]}
              />
            </div>
          </div>
        </div>
      </section>

      {/* Ghost Indicator */}
      <section>
        <h2 className="text-[11px] font-semibold text-overlay-0 mb-4 uppercase tracking-widest">
          Ghost Indicator
        </h2>
        <div className="bg-surface-0/20 border border-surface-0/40 rounded-xl px-5 py-3.5 space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-[13px] font-semibold text-text">Position</p>
              <p className="text-xs text-overlay-0 mt-0.5">Where the ghost appears on screen</p>
            </div>
            <div className="w-40">
              <Dropdown
                value={indicatorPos}
                onChange={(v) => {
                  setIndicatorPos(v);
                  saveField("setIndicatorPosition", v);
                  goCall("previewIndicatorPosition");
                }}
                options={[
                  { value: "top-right", label: "Top Right" },
                  { value: "top-left", label: "Top Left" },
                  { value: "bottom-right", label: "Bottom Right" },
                  { value: "bottom-left", label: "Bottom Left" },
                  { value: "center", label: "Center" },
                ]}
              />
            </div>
          </div>

          <div className="h-px bg-surface-0/50" />

          <div className="flex items-center justify-between">
            <div>
              <p className="text-[13px] font-semibold text-text">Visibility</p>
              <p className="text-xs text-overlay-0 mt-0.5">When to show the ghost</p>
            </div>
            <div className="w-40">
              <Dropdown
                value={indicatorMode}
                onChange={(v) => { setIndicatorMode(v); saveField("setIndicatorMode", v); }}
                options={[
                  { value: "processing", label: "While Processing" },
                  { value: "always", label: "Always Visible" },
                  { value: "hidden", label: "Hidden" },
                ]}
              />
            </div>
          </div>
        </div>
      </section>

      {/* API Server */}
      <section>
        <h2 className="text-[11px] font-semibold text-overlay-0 mb-4 uppercase tracking-widest">
          API Server
        </h2>
        <div className="bg-surface-0/20 border border-surface-0/40 rounded-xl px-5 py-3.5 space-y-3">
          <ToggleRow
            label="Enable API Server"
            description="Expose GhostSpell over HTTP for CLI, Telegram, and integrations"
            checked={apiEnabled}
            onChange={async (v) => {
              setApiEnabled(v);
              const result = await goCall("setAPIEnabled", v);
              if (result?.startsWith("error")) {
                setApiEnabled(!v);
                setApiTestResult(result);
              }
              refreshAPIStatus();
            }}
          />

          {apiEnabled && (
            <>
              <div className="h-px bg-surface-0/50" />

              {/* Status indicator */}
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-[13px] font-semibold text-text">Status</p>
                </div>
                <div className="flex items-center gap-2">
                  <span className={`w-2 h-2 rounded-full ${apiRunning ? "bg-green-400" : "bg-overlay-0"}`} />
                  <span className="text-sm text-subtext-0">
                    {apiRunning ? `Running on ${apiListenAddr}` : "Stopped"}
                  </span>
                </div>
              </div>

              <div className="h-px bg-surface-0/50" />

              {/* Address field */}
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-[13px] font-semibold text-text">Listen Address</p>
                  <p className="text-xs text-overlay-0 mt-0.5">Restart toggle to apply changes</p>
                </div>
                <input
                  type="text"
                  value={apiAddr}
                  onChange={(e) => setApiAddr(e.target.value)}
                  onBlur={() => saveField("setAPIAddr", apiAddr)}
                  className="w-44 px-3 py-1.5 bg-crust border border-surface-0 rounded-lg text-sm text-text
                             font-mono focus:border-accent-blue/50 focus:outline-none transition-colors"
                  placeholder="127.0.0.1:7878"
                />
              </div>

              <div className="h-px bg-surface-0/50" />

              {/* Test button */}
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-[13px] font-semibold text-text">Quick Test</p>
                  <p className="text-xs text-overlay-0 mt-0.5">Send "helo wrld" through Correct skill</p>
                </div>
                <button
                  onClick={async () => {
                    setApiTesting(true);
                    setApiTestResult("");
                    const result = await goCall("testAPI");
                    setApiTestResult(result || "");
                    setApiTesting(false);
                  }}
                  disabled={apiTesting}
                  className="px-3 py-1.5 bg-accent-blue/15 text-accent-blue text-sm rounded-lg
                             hover:bg-accent-blue/25 transition-colors disabled:opacity-50"
                >
                  {apiTesting ? "Testing..." : "Test"}
                </button>
              </div>

              {/* Test result */}
              {apiTestResult && (
                <div className={`px-3 py-2 rounded-lg text-sm font-mono break-all ${
                  apiTestResult.startsWith("error")
                    ? "bg-red-500/10 text-red-400"
                    : "bg-green-500/10 text-green-400"
                }`}>
                  {apiTestResult}
                </div>
              )}
            </>
          )}
        </div>
      </section>
    </div>
  );
}
