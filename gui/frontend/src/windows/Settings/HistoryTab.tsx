import { useState, useEffect, useCallback } from "react";
import { goCall } from "@/bridge";

interface HistoryEntry {
  ts: string;
  prompt: string;
  icon: string;
  provider: string;
  model: string;
  label: string;
  input_chars: number;
  output_chars: number;
  duration_ms: number;
  status: string;
  error?: string;
}

function statusBadge(status: string) {
  switch (status) {
    case "success":
      return <span className="text-accent-green text-[10px] font-medium">OK</span>;
    case "identical":
      return <span className="text-accent-blue text-[10px] font-medium">No change</span>;
    case "error":
      return <span className="text-accent-red text-[10px] font-medium">Error</span>;
    case "timeout":
      return <span className="text-accent-yellow text-[10px] font-medium">Timeout</span>;
    case "cancelled":
      return <span className="text-overlay-0 text-[10px] font-medium">Cancelled</span>;
    default:
      return <span className="text-overlay-0 text-[10px]">{status}</span>;
  }
}

function formatTime(ts: string) {
  try {
    const d = new Date(ts);
    return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" });
  } catch {
    return ts;
  }
}

function formatDate(ts: string) {
  try {
    const d = new Date(ts);
    return d.toLocaleDateString([], { month: "short", day: "numeric" });
  } catch {
    return "";
  }
}

function formatDuration(ms: number) {
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
}

export function HistoryTab() {
  const [entries, setEntries] = useState<HistoryEntry[]>([]);

  const load = useCallback(async () => {
    const raw = await goCall("getHistory");
    if (!raw) return;
    try {
      setEntries(JSON.parse(raw) || []);
    } catch { /* ignore */ }
  }, []);

  useEffect(() => { load(); }, [load]);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-[13px] font-semibold text-text">Action History</h3>
        <button
          onClick={load}
          className="text-[11px] text-overlay-0 hover:text-subtext-0 transition-colors"
        >
          Refresh
        </button>
      </div>

      {entries.length === 0 ? (
        <p className="text-[12px] text-overlay-0">No actions recorded yet. Press F7 to get started.</p>
      ) : (
        <div className="space-y-1">
          {entries.map((e, i) => (
            <div
              key={i}
              className="flex items-center gap-3 px-3 py-2 rounded-lg bg-surface-0/40 hover:bg-surface-0/70 transition-colors"
            >
              {/* Icon + prompt */}
              <span className="text-[14px] w-5 text-center shrink-0">{e.icon || "⚡"}</span>
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2">
                  <span className="text-[12px] font-medium text-text truncate">{e.prompt}</span>
                  {statusBadge(e.status)}
                </div>
                {e.error && (
                  <p className="text-[10px] text-accent-red truncate mt-0.5">{e.error}</p>
                )}
                <div className="flex items-center gap-3 mt-0.5">
                  <span className="text-[10px] text-overlay-0">{e.label || e.model}</span>
                  {e.input_chars > 0 && (
                    <span className="text-[10px] text-overlay-0">{e.input_chars} chars in</span>
                  )}
                  {e.output_chars > 0 && (
                    <span className="text-[10px] text-overlay-0">{e.output_chars} chars out</span>
                  )}
                </div>
              </div>
              {/* Duration + time */}
              <div className="text-right shrink-0">
                <div className="text-[11px] text-subtext-0 font-mono">{formatDuration(e.duration_ms)}</div>
                <div className="text-[10px] text-overlay-0">{formatDate(e.ts)} {formatTime(e.ts)}</div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
