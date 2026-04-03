import { useState, useEffect, useCallback } from "react";
import { goCall } from "@/bridge";
import { ProviderLogo } from "@/components/logos/ProviderLogo";
import { CreatorLogo, resolveCreator } from "@/components/logos/CreatorLogo";
import { Badge } from "@/components/ui/Badge";

interface CatalogModel {
  provider: string;
  creator: string;
  model: string;
  name: string;
  description: string;
  cost_tier: string;
  tags: string[];
  enabled: boolean;
  is_default: boolean;
  config_label: string;
  provider_active: boolean;
  avg_speed_ms: number;
  vision?: boolean;
}

/**
 * Models tab — model catalog with cards, toggles, search.
 * Zen: clean cards, subtle hover, no visual clutter.
 */
export function ModelsTab() {
  const [models, setModels] = useState<CatalogModel[]>([]);
  const [search, setSearch] = useState("");
  const [provFilter, setProvFilter] = useState("");
  const [loading, setLoading] = useState(true);

  const loadModels = useCallback(async () => {
    const raw = await goCall("getModelCatalog");
    if (raw) {
      try {
        setModels(JSON.parse(raw));
      } catch { /* ignore */ }
    }
    setLoading(false);
  }, []);

  useEffect(() => { loadModels(); }, [loadModels]);

  async function setDefault(provider: string, model: string) {
    await goCall("setDefaultByModel", provider, model);
    loadModels();
  }

  async function toggleModel(provider: string, model: string, enabled: boolean) {
    await goCall("toggleModel", provider, model, enabled);
    loadModels();
  }

  // Filter models
  const filtered = models.filter((m) => {
    if (search) {
      const q = search.toLowerCase();
      if (!m.name.toLowerCase().includes(q) && !m.model.toLowerCase().includes(q) && !m.provider.toLowerCase().includes(q)) return false;
    }
    if (provFilter && m.provider !== provFilter) return false;
    return true;
  });

  // Active model (default)
  const active = models.find((m) => m.is_default);

  // Unique providers for filter
  const providers = [...new Set(models.map((m) => m.provider))];

  if (loading) {
    return <div className="flex items-center justify-center py-20"><p className="text-sm text-overlay-0">Loading models...</p></div>;
  }

  return (
    <div className="space-y-6">
      {/* Active model summary */}
      {active && (
        <div className="bg-accent-green/5 border border-accent-green/20 rounded-xl p-4 flex items-center gap-4">
          <div className="flex items-center gap-1 shrink-0">
            <ProviderLogo provider={active.provider} size={40} />
            {active.creator && active.creator !== active.provider && (
              <>
                <span className="text-accent-blue text-xs mx-1">→</span>
                <CreatorLogo creator={active.creator} size={40} />
              </>
            )}
          </div>
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2 flex-wrap">
              <span className="text-sm font-semibold text-text">{active.name}</span>
              <Badge variant="active" />
              {active.tags?.includes("vision") && <Badge variant="vision" />}
            </div>
            {active.description && (
              <p className="text-xs text-overlay-0 mt-0.5 line-clamp-1">{active.description}</p>
            )}
          </div>
          <CostBadge tier={active.cost_tier} provider={active.provider} />
        </div>
      )}

      {/* Search + filter */}
      <div className="flex gap-2">
        <input
          type="text"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Search models..."
          className="flex-1 bg-crust border border-surface-0 rounded-lg px-3 py-2
                     text-sm text-text placeholder:text-overlay-0
                     focus:border-accent-blue/50 focus:outline-none transition-colors"
        />
        <select
          value={provFilter}
          onChange={(e) => setProvFilter(e.target.value)}
          className="bg-crust border border-surface-0 rounded-lg px-3 py-2
                     text-sm text-subtext-0 focus:outline-none"
        >
          <option value="">All Providers</option>
          {providers.map((p) => (
            <option key={p} value={p}>{p}</option>
          ))}
        </select>
      </div>

      {/* Model list */}
      <div className="space-y-3">
        {filtered.map((m) => (
          <ModelCard
            key={`${m.provider}-${m.model}`}
            model={m}
            onSetDefault={() => setDefault(m.provider, m.model)}
            onToggle={(enabled) => toggleModel(m.provider, m.model, enabled)}
          />
        ))}
        {filtered.length === 0 && (
          <p className="text-sm text-overlay-0 text-center py-8">No models found.</p>
        )}
      </div>
    </div>
  );
}

function ModelCard({
  model: m,
  onSetDefault,
  onToggle,
}: {
  model: CatalogModel;
  onSetDefault: () => void;
  onToggle: (enabled: boolean) => void;
}) {
  const creator = m.creator || resolveCreator(m.model);

  return (
    <div
      className={`bg-surface-0/30 border rounded-xl p-4 flex items-center gap-4
                  transition-colors hover:border-accent-blue/30
                  ${m.is_default ? "border-accent-green/30" : "border-surface-0/50"}
                  ${!m.enabled && !m.is_default ? "opacity-50" : ""}`}
    >
      {/* Logos */}
      <div className="flex items-center gap-1 shrink-0">
        <ProviderLogo provider={m.provider} size={36} />
        {creator && creator !== m.provider && (
          <>
            <span className="text-accent-blue/50 text-[10px] mx-0.5">→</span>
            <CreatorLogo creator={creator} size={36} />
          </>
        )}
      </div>

      {/* Info */}
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 flex-wrap">
          <span className="text-sm font-medium text-text">{m.name}</span>
          {m.tags?.filter((t) => t !== "free" && t !== "offline").map((t) => (
            <Badge key={t} variant={t as any} />
          ))}
          {m.is_default && <Badge variant="active" label="DEFAULT" />}
        </div>
        {m.description && (
          <p className="text-xs text-overlay-0 mt-0.5 line-clamp-2">{m.description}</p>
        )}
      </div>

      {/* Cost */}
      <CostBadge tier={m.cost_tier} provider={m.provider} />

      {/* Speed */}
      {m.avg_speed_ms > 0 && (
        <div className="text-right shrink-0">
          <p className="text-[10px] text-overlay-0 uppercase tracking-wider">Speed</p>
          <p className={`text-sm font-semibold ${
            m.avg_speed_ms < 2000 ? "text-accent-green" : m.avg_speed_ms < 5000 ? "text-accent-yellow" : "text-accent-red"
          }`}>
            {(m.avg_speed_ms / 1000).toFixed(1)}s
          </p>
        </div>
      )}

      {/* Actions */}
      <div className="flex items-center gap-2 shrink-0">
        {!m.is_default && m.enabled && (
          <button
            onClick={onSetDefault}
            className="px-2.5 py-1 rounded-md text-[11px] font-medium
                       bg-accent-blue/10 text-accent-blue hover:bg-accent-blue/20 transition-colors"
          >
            Use
          </button>
        )}
        <button
          onClick={() => onToggle(!m.enabled)}
          className={`relative shrink-0 transition-colors duration-200 ${
            m.enabled ? "bg-accent-blue" : "bg-surface-2"
          }`}
          style={{ width: 32, height: 18, borderRadius: 9 }}
          title={m.is_default ? "Cannot disable the default model" : m.enabled ? "Disable" : "Enable"}
        >
          <span className="absolute bg-white rounded-full shadow-sm" style={{ width: 14, height: 14, top: 2, left: m.enabled ? 16 : 2, transition: "left 200ms ease" }} />
        </button>
      </div>
    </div>
  );
}

function CostBadge({ tier, provider }: { tier: string; provider: string }) {
  if (!tier && (provider === "local" || provider === "ollama" || provider === "lmstudio")) {
    return <span className="text-xs font-semibold text-accent-green shrink-0">FREE</span>;
  }
  const display = tier === "free" ? "FREE" : tier === "$" ? "$" : tier === "$$" ? "$$" : tier === "$$$" ? "$$$" : "PAID";
  const color = tier === "free" ? "text-accent-green" : tier === "$" ? "text-accent-blue" : tier === "$$" ? "text-accent-yellow" : "text-accent-red";
  return <span className={`text-xs font-semibold ${color} shrink-0`}>{display}</span>;
}
