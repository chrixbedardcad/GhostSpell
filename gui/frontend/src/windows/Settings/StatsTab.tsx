import { useState, useEffect, useCallback, useRef } from "react";
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

interface BenchmarkModel {
  label: string;
  provider: string;
  model: string;
  status: string;
  duration_ms: number;
  error?: string;
}

interface BenchmarkResult {
  running: boolean;
  prompt_name: string;
  prompt_icon: string;
  models: BenchmarkModel[];
}

export function StatsTab() {
  const platform = usePlatform();
  const hotkey = platform === "darwin" ? "\u2318G" : "Ctrl+G";
  const [total, setTotal] = useState(0);
  const [avgTime, setAvgTime] = useState(0);
  const [topPrompt, setTopPrompt] = useState("");
  const [topModel, setTopModel] = useState("");
  const [models, setModels] = useState<ModelStat[]>([]);
  const [prompts, setPrompts] = useState<PromptStat[]>([]);
  const [benchRunning, setBenchRunning] = useState(false);
  const [benchResult, setBenchResult] = useState<BenchmarkResult | null>(null);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const load = useCallback(async () => {
    const raw = await goCall("getStats");
    if (!raw) return;
    try {
      const s = JSON.parse(raw);
      setTotal(s.total_requests || 0);
      setAvgTime(s.avg_duration_ms || 0);
      setTopPrompt(s.most_used_prompt || "\u2014");
      setTopModel(s.most_used_model || "\u2014");
      setModels(s.models || []);
      setPrompts(s.prompts || []);
    } catch { /* ignore */ }
  }, []);

  useEffect(() => {
    load();
    return () => { if (pollRef.current) clearInterval(pollRef.current); };
  }, [load]);

  async function startBenchmark() {
    setBenchRunning(true);
    setBenchResult(null);
    await goCall("runBenchmark");
    // Poll for results
    pollRef.current = setInterval(async () => {
      const raw = await goCall("getBenchmarkResult");
      if (!raw) return;
      try {
        const r: BenchmarkResult = JSON.parse(raw);
        setBenchResult(r);
        if (!r.running) {
          if (pollRef.current) clearInterval(pollRef.current);
          setBenchRunning(false);
          load(); // Refresh stats after benchmark
        }
      } catch { /* ignore */ }
    }, 1000);
  }

  async function stopBenchmark() {
    await goCall("stopBenchmark");
    if (pollRef.current) clearInterval(pollRef.current);
    setBenchRunning(false);
  }

  const maxPromptCount = Math.max(1, ...prompts.map((p) => p.count));

  return (
    <div className="space-y-8">
      {/* Summary */}
      <div className="grid grid-cols-2 gap-3">
        <StatCard label="Total Requests" value={String(total)} />
        <StatCard label="Avg Response" value={avgTime > 0 ? `${(avgTime / 1000).toFixed(1)}s` : "\u2014"} />
        <StatCard label="Top Prompt" value={topPrompt} />
        <StatCard label="Top Model" value={topModel} />
      </div>

      {/* Prompt usage */}
      {prompts.length > 0 && (
        <section>
          <h2 className="text-[11px] font-semibold text-overlay-0 mb-4 uppercase tracking-widest">
            Prompt Usage
          </h2>
          <div className="space-y-2">
            {prompts.map((p) => (
              <div key={p.name} className="flex items-center gap-3">
                <span className="w-6 text-center shrink-0">{p.icon || "\uD83D\uDCDD"}</span>
                <span className="text-[12px] text-subtext-0 w-24 shrink-0 truncate">{p.name}</span>
                <div className="flex-1 bg-surface-0/20 border border-surface-0/30 rounded-full h-2 overflow-hidden">
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
          <h2 className="text-[11px] font-semibold text-overlay-0 mb-4 uppercase tracking-widest">
            Model Performance
          </h2>
          <div className="space-y-3">
            {models.map((m) => (
              <div key={m.label} className="bg-surface-0/20 border border-surface-0/40 rounded-xl px-5 py-3.5 flex items-center gap-4">
                <div className="flex-1 min-w-0">
                  <p className="text-[13px] font-semibold text-text truncate">{m.label}</p>
                  <p className="text-[11px] text-overlay-0 mt-0.5">{m.count} requests</p>
                </div>
                <div className="text-right shrink-0">
                  <p className={`text-[13px] font-semibold ${
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

      {/* Benchmark */}
      <section>
        <h2 className="text-[11px] font-semibold text-overlay-0 mb-4 uppercase tracking-widest">
          Benchmark
        </h2>
        <div className="bg-surface-0/20 border border-surface-0/40 rounded-xl px-5 py-4">
          <p className="text-[12px] text-overlay-0 mb-3">
            Run a speed test across all enabled models using the active skill.
          </p>
          <div className="flex gap-3">
            {benchRunning ? (
              <button
                onClick={stopBenchmark}
                className="px-4 py-2 rounded-lg text-[12px] font-medium bg-red-500/15 text-red-400 hover:bg-red-500/25 transition-colors"
              >
                Stop Benchmark
              </button>
            ) : (
              <button
                onClick={startBenchmark}
                className="px-4 py-2 rounded-lg text-[12px] font-medium bg-accent-blue/15 text-accent-blue hover:bg-accent-blue/25 transition-colors"
              >
                Run Benchmark
              </button>
            )}
          </div>

          {/* Benchmark results */}
          {benchResult && benchResult.models && benchResult.models.length > 0 && (
            <div className="mt-4 space-y-2">
              <p className="text-[11px] text-overlay-0">
                {benchResult.prompt_icon} {benchResult.prompt_name} {benchRunning ? "— running..." : "— complete"}
              </p>
              {benchResult.models.map((m) => (
                <div key={m.label} className="flex items-center gap-3 py-1.5">
                  <span className="text-[12px] text-text w-36 truncate">{m.label}</span>
                  {m.status === "done" ? (
                    <span className={`text-[12px] font-semibold ${
                      m.duration_ms < 2000 ? "text-accent-green" : m.duration_ms < 5000 ? "text-accent-yellow" : "text-accent-red"
                    }`}>
                      {(m.duration_ms / 1000).toFixed(1)}s
                    </span>
                  ) : m.status === "running" ? (
                    <span className="text-[11px] text-accent-blue animate-pulse">Running...</span>
                  ) : m.status === "error" ? (
                    <span className="text-[11px] text-accent-red">{m.error || "Error"}</span>
                  ) : (
                    <span className="text-[11px] text-overlay-0">Pending</span>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      </section>

      {/* Actions */}
      <div className="flex gap-3">
        <button onClick={() => goCall("openStatsFile")}
          className="px-4 py-2 rounded-lg text-[12px] bg-surface-0/20 border border-surface-0/40 text-subtext-0
                     hover:bg-surface-0/40 transition-colors">
          Open stats.json
        </button>
        <button onClick={async () => { await goCall("clearStats"); load(); }}
          className="px-4 py-2 rounded-lg text-[12px] bg-surface-0/20 border border-surface-0/40 text-accent-red/70
                     hover:bg-red-500/10 transition-colors">
          Clear Stats
        </button>
      </div>

      {total === 0 && !benchResult && (
        <p className="text-[12px] text-overlay-0 text-center py-4">
          No usage data yet. Press {hotkey} to start using GhostSpell.
        </p>
      )}
    </div>
  );
}

function StatCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="bg-surface-0/20 border border-surface-0/40 rounded-xl px-5 py-3.5 text-center">
      <p className="text-[16px] font-semibold text-text">{value}</p>
      <p className="text-[11px] text-overlay-0 mt-1">{label}</p>
    </div>
  );
}
