import { useState, useEffect, useCallback } from "react";
import { goCall } from "@/bridge";

interface HistoryEntry {
  ts: string;
  prompt: string;
  icon: string;
  provider: string;
  model: string;
  label: string;
  input: string;
  output: string;
  input_len: number;
  output_len: number;
  duration_ms: number;
  status: string;
  error?: string;
}

function statusBadge(status: string) {
  switch (status) {
    case "success":
      return <span className="px-1.5 py-0.5 rounded text-[10px] font-medium bg-green-500/15 text-green-400">OK</span>;
    case "identical":
      return <span className="px-1.5 py-0.5 rounded text-[10px] font-medium bg-blue-500/15 text-blue-400">No change</span>;
    case "error":
      return <span className="px-1.5 py-0.5 rounded text-[10px] font-medium bg-red-500/15 text-red-400">Error</span>;
    case "timeout":
      return <span className="px-1.5 py-0.5 rounded text-[10px] font-medium bg-yellow-500/15 text-yellow-400">Timeout</span>;
    case "cancelled":
      return <span className="px-1.5 py-0.5 rounded text-[10px] font-medium bg-surface-1 text-overlay-0">Cancelled</span>;
    default:
      return <span className="text-overlay-0 text-[10px]">{status}</span>;
  }
}

function formatTime(ts: string) {
  try {
    const d = new Date(ts);
    return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" });
  } catch { return ts; }
}

function formatDate(ts: string) {
  try {
    const d = new Date(ts);
    return d.toLocaleDateString([], { month: "short", day: "numeric" });
  } catch { return ""; }
}

function formatDuration(ms: number) {
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
}

function truncate(text: string, max: number) {
  if (!text) return "";
  if (text.length <= max) return text;
  return text.slice(0, max) + "...";
}

export function HistoryTab() {
  const [entries, setEntries] = useState<HistoryEntry[]>([]);
  const [expanded, setExpanded] = useState<number | null>(null);

  const load = useCallback(async () => {
    const raw = await goCall("getHistory");
    if (!raw) return;
    try { setEntries(JSON.parse(raw) || []); } catch { /* ignore */ }
  }, []);

  useEffect(() => { load(); }, [load]);

  function toggle(i: number) {
    setExpanded(expanded === i ? null : i);
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <p className="text-[12px] text-overlay-0">
          {entries.length > 0
            ? `${entries.length} actions recorded. Click to expand.`
            : "No actions recorded yet. Use GhostSpell to see history here."}
        </p>
        <button
          onClick={load}
          className="text-[11px] text-overlay-0 hover:text-subtext-0 transition-colors px-2 py-1 rounded hover:bg-surface-0/30"
        >
          Refresh
        </button>
      </div>

      <div className="space-y-3">
        {entries.map((e, i) => (
          <div key={i} className="bg-surface-0/30 rounded-xl overflow-hidden border border-surface-0/20">
            {/* Summary row — click to expand */}
            <button
              onClick={() => toggle(i)}
              className="w-full flex items-center gap-3 px-4 py-3 text-left hover:bg-surface-0/50 transition-colors"
            >
              <span className="text-[16px] w-6 text-center shrink-0">{e.icon || "\u26A1"}</span>
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2 flex-wrap">
                  <span className="text-[12px] font-medium text-text">{e.prompt}</span>
                  {statusBadge(e.status)}
                  <span className="text-[10px] text-overlay-0">{e.label || e.model}</span>
                </div>
                {e.input && (
                  <p className="text-[11px] text-overlay-0 mt-0.5 truncate">
                    {truncate(e.input, 80)}
                  </p>
                )}
              </div>
              <div className="text-right shrink-0">
                <div className="text-[11px] text-subtext-0 font-mono">{formatDuration(e.duration_ms)}</div>
                <div className="text-[10px] text-overlay-0">{formatDate(e.ts)} {formatTime(e.ts)}</div>
              </div>
              <span className="text-overlay-0 text-[10px] shrink-0">{expanded === i ? "\u25B2" : "\u25BC"}</span>
            </button>

            {/* Expanded detail */}
            {expanded === i && (
              <div className="px-4 pb-4 space-y-3 border-t border-surface-0/30">
                {e.error && (
                  <div className="mt-3 px-3 py-2 rounded-lg bg-red-500/10 text-red-400 text-[11px]">
                    {e.error}
                  </div>
                )}

                {e.input && (
                  <div className="mt-3">
                    <p className="text-[10px] font-medium text-overlay-0 uppercase tracking-wider mb-1">Input ({e.input_len} chars)</p>
                    <div className="bg-crust rounded-lg px-3 py-2 text-[11px] text-text whitespace-pre-wrap break-words max-h-[200px] overflow-y-auto select-text">
                      {e.input}
                    </div>
                  </div>
                )}

                {e.output && (
                  <div>
                    <p className="text-[10px] font-medium text-overlay-0 uppercase tracking-wider mb-1">Output ({e.output_len} chars)</p>
                    <div className="bg-crust rounded-lg px-3 py-2 text-[11px] text-text whitespace-pre-wrap break-words max-h-[200px] overflow-y-auto select-text">
                      {e.output}
                    </div>
                  </div>
                )}

                <div className="flex items-center gap-4 text-[10px] text-overlay-0 pt-1">
                  <span>Provider: {e.provider}</span>
                  <span>Model: {e.model}</span>
                  <span>Duration: {formatDuration(e.duration_ms)}</span>
                </div>
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}
