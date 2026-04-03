import { useState, useEffect, useCallback } from "react";
import { goCall } from "@/bridge";
import { usePlatform } from "@/hooks/usePlatform";

interface ModelStat {
  label: string;
  provider: string;
  model: string;
  count: number;
  avg_ms: number;
  success_rate: number;
}

interface PromptStat {
  name: string;
  icon: string;
  count: number;
}

/**
 * Stats tab — usage statistics.
 * Zen: clean numbers, minimal charts, calm colors.
 */
export function StatsTab() {
  const platform = usePlatform();
  const hotkey = platform === "darwin" ? "⌘G" : "Ctrl+G";
  const [total, setTotal] = useState(0);
  const [avgTime, setAvgTime] = useState(0);
  const [topPrompt, setTopPrompt] = useState("");
  const [topModel, setTopModel] = useState("");
  const [models, setModels] = useState<ModelStat[]>([]);
  const [prompts, setPrompts] = useState<PromptStat[]>([]);

  const load = useCallback(async () => {
    const raw = await goCall("getStats");
    if (!raw) return;
    try {
      const s = JSON.parse(raw);
      setTotal(s.total_requests || 0);
      setAvgTime(s.avg_duration_ms || 0);
      setTopPrompt(s.most_used_prompt || "—");
      setTopModel(s.most_used_model || "—");
      setModels(s.models || []);
      setPrompts(s.prompts || []);
    } catch { /* ignore */ }
  }, []);

  useEffect(() => { load(); }, [load]);

  const maxPromptCount = Math.max(1, ...prompts.map((p) => p.count));

  return (
    <div className="space-y-10">
      {/* Summary grid */}
      <div className="grid grid-cols-2 gap-3">
        <StatCard label="Total Requests" value={String(total)} />
        <StatCard label="Avg Response" value={avgTime > 0 ? `${(avgTime / 1000).toFixed(1)}s` : "—"} />
        <StatCard label="Top Prompt" value={topPrompt} />
        <StatCard label="Top Model" value={topModel} />
      </div>

      {/* Prompt usage */}
      {prompts.length > 0 && (
        <section>
          <h2 className="text-sm font-medium text-subtext-1 mb-4 tracking-wide uppercase">
            Prompt Usage
          </h2>
          <div className="space-y-2">
            {prompts.map((p) => (
              <div key={p.name} className="flex items-center gap-3">
                <span className="w-6 text-center shrink-0">{p.icon || "📝"}</span>
                <span className="text-xs text-subtext-0 w-24 shrink-0 truncate">{p.name}</span>
                <div className="flex-1 bg-surface-0/30 rounded-full h-1.5 overflow-hidden">
                  <div
                    className="h-full bg-accent-blue/50 rounded-full"
                    style={{ width: `${(p.count / maxPromptCount) * 100}%` }}
                  />
                </div>
                <span className="text-[11px] text-overlay-0 w-8 text-right shrink-0">{p.count}</span>
              </div>
            ))}
          </div>
        </section>
      )}

      {/* Model performance */}
      {models.length > 0 && (
        <section>
          <h2 className="text-sm font-medium text-subtext-1 mb-4 tracking-wide uppercase">
            Model Performance
          </h2>
          <div className="space-y-2">
            {models.map((m) => (
              <div key={m.label} className="bg-surface-0/30 rounded-xl p-3 flex items-center gap-4">
                <div className="flex-1 min-w-0">
                  <p className="text-xs font-medium text-text truncate">{m.label}</p>
                  <p className="text-[11px] text-overlay-0">{m.count} requests</p>
                </div>
                <div className="text-right shrink-0">
                  <p className={`text-xs font-semibold ${
                    m.avg_ms < 2000 ? "text-accent-green" : m.avg_ms < 5000 ? "text-accent-yellow" : "text-accent-red"
                  }`}>
                    {(m.avg_ms / 1000).toFixed(1)}s
                  </p>
                  <p className="text-[10px] text-overlay-0">
                    {Math.round(m.success_rate * 100)}% success
                  </p>
                </div>
              </div>
            ))}
          </div>
        </section>
      )}

      {/* Actions */}
      <div className="flex gap-2">
        <button onClick={() => goCall("openStatsFile")}
          className="px-3 py-1.5 rounded-lg text-xs bg-surface-0 text-subtext-0
                     hover:bg-surface-1 transition-colors">
          Open stats.json
        </button>
        <button onClick={async () => { await goCall("clearStats"); load(); }}
          className="px-3 py-1.5 rounded-lg text-xs bg-surface-0 text-accent-red/70
                     hover:bg-surface-1 transition-colors">
          Clear Stats
        </button>
      </div>

      {total === 0 && (
        <p className="text-xs text-overlay-0 text-center py-4">
          No usage data yet. Press {hotkey} to start using GhostSpell.
        </p>
      )}
    </div>
  );
}

function StatCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="bg-surface-0/30 rounded-xl p-4 text-center">
      <p className="text-lg font-semibold text-text">{value}</p>
      <p className="text-[11px] text-overlay-0 mt-0.5">{label}</p>
    </div>
  );
}
