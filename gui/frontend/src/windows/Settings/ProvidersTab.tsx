import { useState, useEffect, useCallback, useRef } from "react";
import { goCall, openURL } from "@/bridge";
import { ProviderLogo } from "@/components/logos/ProviderLogo";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface ProviderConfig {
  api_key?: string;
  api_endpoint?: string;
  refresh_token?: string;
  keep_alive?: boolean;
  disabled?: boolean;
}

interface ModelConfig {
  provider: string;
  model: string;
}

interface LocalModel {
  name: string;
  tag?: string;
  desc?: string;
  size?: number;
  installed?: boolean;
}

interface DownloadProgress {
  type?: string;
  model_name?: string;
  percent: number;
  downloaded: number;
  total: number;
}

interface OllamaModel {
  name: string;
  parameter_size?: string;
  quantization_level?: string;
  size_human?: string;
}

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const PROVIDER_NAMES: Record<string, string> = {
  local: "GhostSpell Local",
  chatgpt: "ChatGPT Login",
  openai: "OpenAI",
  anthropic: "Anthropic Claude",
  gemini: "Google Gemini",
  xai: "xAI Grok",
  deepseek: "DeepSeek",
  ollama: "Ollama",
  lmstudio: "LM Studio",
};

const CLOUD_PROVIDERS = ["openai", "chatgpt", "anthropic", "gemini", "xai", "deepseek"] as const;
const LOCAL_PROVIDERS = ["ollama", "lmstudio"] as const;

const API_KEY_LINKS: Record<string, string> = {
  openai: "https://platform.openai.com/api-keys",
  anthropic: "https://console.anthropic.com/settings/keys",
  gemini: "https://aistudio.google.com/apikey",
  xai: "https://console.x.ai/team/default/api-keys",
  deepseek: "https://platform.deepseek.com/api_keys",
};

const PROVIDER_DESCRIPTIONS: Record<string, string> = {
  openai: "GPT-4o, o1, o3 -- paste your OpenAI API key.",
  chatgpt: "Sign in with ChatGPT -- no API key needed.",
  anthropic: "Premium quality, best multilingual support.",
  gemini: "Free tier: 250 requests/day.",
  xai: "$25 free credits at signup.",
  deepseek: "Affordable, strong reasoning models.",
  ollama: "Run any open-source model locally with Ollama.",
  lmstudio: "Connect to a local LM Studio server. OpenAI-compatible API.",
};

const DEFAULT_ENDPOINTS: Record<string, string> = {
  ollama: "http://127.0.0.1:11434",
  lmstudio: "http://127.0.0.1:1234/v1",
};

// ---------------------------------------------------------------------------
// Toggle Row (reused from GeneralTab pattern)
// ---------------------------------------------------------------------------

function ToggleRow({
  label,
  description,
  checked,
  onChange,
  disabled,
}: {
  label: string;
  description?: string;
  checked: boolean;
  onChange: (v: boolean) => void;
  disabled?: boolean;
}) {
  return (
    <div className="flex items-center justify-between py-2">
      <div>
        <p className="text-sm text-text">{label}</p>
        {description && <p className="text-xs text-overlay-0 mt-0.5">{description}</p>}
      </div>
      <button
        onClick={() => !disabled && onChange(!checked)}
        disabled={disabled}
        className={`relative w-9 h-5 rounded-full transition-colors shrink-0 ${
          checked ? "bg-accent-blue" : "bg-surface-1"
        } ${disabled ? "opacity-50 cursor-not-allowed" : ""}`}
      >
        <span
          className={`absolute top-0.5 w-4 h-4 rounded-full bg-text transition-transform ${
            checked ? "translate-x-4" : "translate-x-0.5"
          }`}
        />
      </button>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Status Badge
// ---------------------------------------------------------------------------

function StatusBadge({ status }: { status: "configured" | "disabled" | "none" | "running" | "error" }) {
  const styles: Record<string, { dot: string; text: string; label: string }> = {
    configured: { dot: "bg-green-400", text: "text-green-400", label: "Configured" },
    running: { dot: "bg-green-400", text: "text-green-400", label: "Running" },
    disabled: { dot: "bg-overlay-0", text: "text-overlay-0", label: "Disabled" },
    error: { dot: "bg-red-400", text: "text-red-400", label: "Error" },
    none: { dot: "bg-overlay-0", text: "text-overlay-0", label: "Not configured" },
  };
  const s = styles[status] || styles.none;
  return (
    <span className={`flex items-center gap-1.5 text-xs ${s.text}`}>
      <span className={`w-1.5 h-1.5 rounded-full ${s.dot}`} />
      {s.label}
    </span>
  );
}

// ---------------------------------------------------------------------------
// GhostSpell Local Section
// ---------------------------------------------------------------------------

function LocalSection({
  providers,
  models,
  onRefresh,
}: {
  providers: Record<string, ProviderConfig>;
  models: Record<string, ModelConfig>;
  onRefresh: () => void;
}) {
  const [localModels, setLocalModels] = useState<LocalModel[]>([]);
  const [engineStatus, setEngineStatus] = useState("");
  const [loading, setLoading] = useState(true);
  const [downloading, setDownloading] = useState("");
  const [dlProgress, setDlProgress] = useState<DownloadProgress | null>(null);
  const [keepAlive, setKeepAlive] = useState(providers.local?.keep_alive ?? false);
  const [testResult, setTestResult] = useState("");
  const [testing, setTesting] = useState(false);
  const [deleteConfirm, setDeleteConfirm] = useState("");
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  // Currently selected local model from config
  const activeLocalModel = Object.values(models).find((m) => m.provider === "local")?.model || "";

  const loadLocalStatus = useCallback(async () => {
    setLoading(true);

    // Check for active downloads
    try {
      const pRaw = await goCall("localDownloadProgress");
      if (pRaw) {
        const p: DownloadProgress = JSON.parse(pRaw);
        if (p.type === "local" && p.model_name) setDownloading(p.model_name);
      }
    } catch { /* ignore */ }

    const raw = await goCall("localStatus");
    if (!raw) { setLoading(false); setEngineStatus("Status check failed"); return; }
    try {
      const st = JSON.parse(raw);
      setEngineStatus(st.engine_version ? `Engine ${st.engine_version}` : "Ready");

      const installed = (st.models || []) as { name: string; size?: number }[];
      const available = (st.available || []) as { name: string; tag?: string; desc?: string; size?: number }[];

      const installedNames = new Set(installed.map((m) => m.name));
      const merged: LocalModel[] = available.map((a) => ({
        name: a.name,
        tag: a.tag,
        desc: a.desc,
        size: installedNames.has(a.name) ? (installed.find((m) => m.name === a.name)?.size || a.size) : a.size,
        installed: installedNames.has(a.name),
      }));
      // Add installed models not in available
      installed.forEach((m) => {
        if (!available.find((a) => a.name === m.name)) {
          merged.push({ name: m.name, size: m.size, installed: true });
        }
      });
      setLocalModels(merged);
    } catch { setEngineStatus("Check failed"); }
    setLoading(false);
  }, []);

  useEffect(() => { loadLocalStatus(); }, [loadLocalStatus]);

  // Cleanup poll on unmount
  useEffect(() => {
    return () => { if (pollRef.current) clearInterval(pollRef.current); };
  }, []);

  // Resume download polling if active
  useEffect(() => {
    if (downloading) startPoll(downloading);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [downloading]);

  function startPoll(name: string) {
    if (pollRef.current) clearInterval(pollRef.current);
    pollRef.current = setInterval(async () => {
      try {
        const pRaw = await goCall("localDownloadProgress");
        if (!pRaw) {
          if (pollRef.current) clearInterval(pollRef.current);
          setDownloading("");
          setDlProgress(null);
          loadLocalStatus();
          onRefresh();
          return;
        }
        const p: DownloadProgress = JSON.parse(pRaw);
        setDlProgress(p);
      } catch { /* ignore */ }
    }, 500);
  }

  async function downloadModel(name: string) {
    setDownloading(name);
    setDlProgress(null);
    startPoll(name);
    const result = await goCall("localDownloadModel", name);
    if (pollRef.current) clearInterval(pollRef.current);
    setDownloading("");
    setDlProgress(null);
    if (result && result.startsWith("ok")) {
      await selectModel(name);
    }
    loadLocalStatus();
    onRefresh();
  }

  async function deleteModel(name: string) {
    if (deleteConfirm !== name) {
      setDeleteConfirm(name);
      setTimeout(() => setDeleteConfirm(""), 3000);
      return;
    }
    setDeleteConfirm("");
    await goCall("localDeleteModel", name);
    loadLocalStatus();
    onRefresh();
  }

  async function selectModel(name: string) {
    await goCall("saveProviderConfig", "local", "", "", "", keepAlive);
    await goCall("saveModel", "GhostSpell Local", "local", name, "");
    await goCall("setDefaultModel", "GhostSpell Local");
    onRefresh();
    loadLocalStatus();
  }

  async function handleTest() {
    setTesting(true);
    setTestResult("");
    const result = await goCall("testProviderConnection", "local");
    setTestResult(result || "Failed");
    setTesting(false);
  }

  async function handleKeepAlive(v: boolean) {
    setKeepAlive(v);
    await goCall("saveProviderConfig", "local", "", "", "", v);
  }

  async function cancelDownload() {
    await goCall("cancelDownload");
  }

  const prov = providers.local;
  const isConfigured = !!prov;
  const isDisabled = prov?.disabled ?? false;
  const status = isDisabled ? "disabled" : isConfigured ? "configured" : "none";

  return (
    <section>
      <h2 className="text-sm font-medium text-subtext-1 mb-4 tracking-wide uppercase">
        GhostSpell Local
      </h2>
      <div className="bg-surface-0/30 rounded-xl p-5 space-y-4">
        {/* Header */}
        <div className="flex items-center gap-3">
          <ProviderLogo provider="local" size={28} />
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2">
              <span className="text-sm font-medium text-text">GhostSpell Local</span>
              <span className="text-[10px] px-1.5 py-0.5 rounded bg-green-500/15 text-green-400 font-semibold">FREE</span>
              <StatusBadge status={status} />
            </div>
            <p className="text-xs text-overlay-0 mt-0.5">
              Runs 100% on your machine -- private, offline, free forever.
            </p>
          </div>
          <div className="flex items-center gap-2 shrink-0">
            <button
              onClick={handleTest}
              disabled={testing || !isConfigured}
              className="px-3 py-1.5 bg-accent-blue/15 text-accent-blue text-xs rounded-lg
                         hover:bg-accent-blue/25 transition-colors disabled:opacity-50"
            >
              {testing ? "Testing..." : "Test"}
            </button>
          </div>
        </div>

        {/* Test result */}
        {testResult && (
          <div className={`px-3 py-2 rounded-lg text-xs font-mono break-all ${
            testResult.startsWith("ok") ? "bg-green-500/10 text-green-400" : "bg-red-500/10 text-red-400"
          }`}>
            {testResult.startsWith("ok") ? "Connection successful" : testResult}
          </div>
        )}

        {/* Keep Alive toggle */}
        <ToggleRow
          label="Keep Model Loaded"
          description="Keep the AI model in memory for faster responses"
          checked={keepAlive}
          onChange={handleKeepAlive}
        />

        <div className="h-px bg-surface-0/50" />

        {/* Engine status */}
        <div className="flex items-center justify-between py-1">
          <span className="text-xs text-overlay-0">Engine Status</span>
          <span className="text-xs text-subtext-0">
            {loading ? "Checking..." : engineStatus}
          </span>
        </div>

        <div className="h-px bg-surface-0/50" />

        {/* Model list */}
        <div>
          <p className="text-xs text-overlay-0 mb-2">Models</p>
          {loading ? (
            <p className="text-xs text-overlay-0 py-4 text-center">Loading...</p>
          ) : localModels.length === 0 ? (
            <p className="text-xs text-overlay-0 py-4 text-center">No models available.</p>
          ) : (
            <div className="space-y-2">
              {localModels.map((m) => {
                const isActive = m.name === activeLocalModel;
                const isDownloading = downloading === m.name;
                const sizeMB = m.size ? Math.round(m.size / 1024 / 1024) : null;

                return (
                  <div
                    key={m.name}
                    className={`bg-crust border rounded-lg p-3 transition-colors ${
                      isActive ? "border-green-500/40 bg-green-500/5" : "border-surface-0 hover:border-surface-1"
                    }`}
                  >
                    <div className="flex items-center gap-3">
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2 flex-wrap">
                          <span className="text-xs font-medium text-text">{m.name}</span>
                          {m.tag && (
                            <span className={`text-[10px] px-1.5 py-0.5 rounded font-semibold ${
                              m.tag === "recommended"
                                ? "bg-blue-500/15 text-blue-400"
                                : m.tag === "fast"
                                  ? "bg-green-500/15 text-green-400"
                                  : "bg-surface-1 text-subtext-0"
                            }`}>
                              {m.tag.toUpperCase()}
                            </span>
                          )}
                          {m.installed && (
                            <span className="text-[10px] px-1.5 py-0.5 rounded bg-green-500/15 text-green-400 font-semibold">DOWNLOADED</span>
                          )}
                          {!m.installed && (
                            <span className="text-[10px] px-1.5 py-0.5 rounded bg-blue-500/15 text-blue-400 font-semibold">AVAILABLE</span>
                          )}
                          {isActive && (
                            <span className="text-[10px] px-1.5 py-0.5 rounded bg-green-500/15 text-green-400 font-semibold">ACTIVE</span>
                          )}
                        </div>
                        <div className="flex items-center gap-2 mt-0.5">
                          {sizeMB && <span className="text-[10px] text-overlay-0">{sizeMB} MB</span>}
                          {m.desc && <span className="text-[10px] text-overlay-0">{m.desc}</span>}
                        </div>
                      </div>
                      <div className="flex items-center gap-2 shrink-0">
                        {m.installed ? (
                          <>
                            {!isActive && (
                              <button
                                onClick={() => selectModel(m.name)}
                                className="px-2 py-1 rounded text-[10px] font-medium
                                           bg-accent-blue/10 text-accent-blue hover:bg-accent-blue/20 transition-colors"
                              >
                                Use
                              </button>
                            )}
                            <button
                              onClick={() => deleteModel(m.name)}
                              className={`px-2 py-1 rounded text-[10px] font-medium transition-colors ${
                                deleteConfirm === m.name
                                  ? "bg-red-500 text-crust"
                                  : "bg-red-500/10 text-red-400 hover:bg-red-500/20"
                              }`}
                            >
                              {deleteConfirm === m.name ? "Confirm?" : "Remove"}
                            </button>
                          </>
                        ) : isDownloading ? (
                          <button
                            onClick={cancelDownload}
                            className="px-2 py-1 rounded text-[10px] font-medium bg-red-500/10 text-red-400 hover:bg-red-500/20 transition-colors"
                          >
                            Cancel
                          </button>
                        ) : (
                          <button
                            onClick={() => downloadModel(m.name)}
                            disabled={!!downloading}
                            className="px-2 py-1 rounded text-[10px] font-medium
                                       bg-accent-blue/10 text-accent-blue hover:bg-accent-blue/20 transition-colors disabled:opacity-50"
                          >
                            Download
                          </button>
                        )}
                      </div>
                    </div>

                    {/* Download progress bar */}
                    {isDownloading && dlProgress && (
                      <div className="mt-2">
                        <div className="w-full bg-surface-1 rounded-full h-2 overflow-hidden">
                          <div
                            className="h-full bg-gradient-to-r from-blue-400 to-green-400 rounded-full transition-all duration-300"
                            style={{ width: `${Math.max(2, Math.min(dlProgress.percent, 100))}%` }}
                          />
                        </div>
                        <p className="text-[10px] text-overlay-0 mt-1">
                          {dlProgress.total > 0
                            ? `${Math.round(dlProgress.downloaded / 1024 / 1024)} / ${Math.round(dlProgress.total / 1024 / 1024)} MB (${dlProgress.percent.toFixed(0)}%)`
                            : "Downloading..."}
                        </p>
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          )}
        </div>
      </div>
    </section>
  );
}

// ---------------------------------------------------------------------------
// Cloud Provider Card
// ---------------------------------------------------------------------------

function CloudProviderCard({
  type,
  config,
  onRefresh,
}: {
  type: string;
  config?: ProviderConfig;
  onRefresh: () => void;
}) {
  const [apiKey, setApiKey] = useState(config?.api_key || "");
  const [showKey, setShowKey] = useState(false);
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState("");
  const [saving, setSaving] = useState(false);
  const [enabled, setEnabled] = useState(config ? !config.disabled : false);

  const isConfigured = !!config;
  const isChatGPT = type === "chatgpt";
  const name = PROVIDER_NAMES[type] || type;
  const description = PROVIDER_DESCRIPTIONS[type] || "";

  async function handleSave() {
    if (!apiKey.trim()) return;
    setSaving(true);
    const result = await goCall("saveProviderConfig", type, apiKey.trim(), "", "", false);
    setSaving(false);
    if (result && result.startsWith("ok")) {
      setTestResult("Provider saved.");
      onRefresh();
    } else {
      setTestResult(result || "Failed to save");
    }
  }

  async function handleTest() {
    setTesting(true);
    setTestResult("");
    const result = await goCall("testProviderConnection", type);
    setTestResult(result || "Failed");
    setTesting(false);
  }

  async function handleToggle(v: boolean) {
    setEnabled(v);
    const result = await goCall("toggleProvider", type, v);
    if (result && result.startsWith("error")) {
      setEnabled(!v);
    }
    onRefresh();
  }

  async function handleRemove() {
    await goCall("removeProvider", type);
    setApiKey("");
    setTestResult("");
    onRefresh();
  }

  async function handleOAuth() {
    await goCall("startChatGPTOAuth");
  }

  const status = config
    ? config.disabled ? "disabled" : "configured"
    : "none";

  return (
    <div className={`bg-surface-0/30 border rounded-xl p-5 space-y-3 transition-colors ${
      config?.disabled ? "opacity-50 border-surface-0/50" : "border-surface-0/50"
    }`}>
      {/* Header */}
      <div className="flex items-center gap-3">
        <ProviderLogo provider={type} size={24} />
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <span className="text-sm font-medium text-text">{name}</span>
            <StatusBadge status={status} />
          </div>
          <p className="text-xs text-overlay-0 mt-0.5">{description}</p>
        </div>
      </div>

      {/* ChatGPT is OAuth-only */}
      {isChatGPT ? (
        <div className="flex items-center gap-2">
          <button
            onClick={handleOAuth}
            className="px-3 py-1.5 bg-green-500/15 text-green-400 text-xs rounded-lg
                       hover:bg-green-500/25 transition-colors"
          >
            {isConfigured ? "Re-authenticate" : "Sign in with ChatGPT"}
          </button>
          {isConfigured && (
            <>
              <button
                onClick={handleTest}
                disabled={testing}
                className="px-3 py-1.5 bg-accent-blue/15 text-accent-blue text-xs rounded-lg
                           hover:bg-accent-blue/25 transition-colors disabled:opacity-50"
              >
                {testing ? "Testing..." : "Test"}
              </button>
              <button
                onClick={handleRemove}
                className="px-3 py-1.5 bg-red-500/10 text-red-400 text-xs rounded-lg
                           hover:bg-red-500/20 transition-colors"
              >
                Remove
              </button>
            </>
          )}
        </div>
      ) : (
        <>
          {/* API Key input */}
          <div className="flex items-center gap-2">
            <div className="relative flex-1">
              <input
                type={showKey ? "text" : "password"}
                value={apiKey}
                onChange={(e) => setApiKey(e.target.value)}
                placeholder="Enter API key..."
                className="w-full bg-crust border border-surface-0 rounded-lg px-3 py-1.5 text-xs text-text
                           font-mono pr-8 focus:border-accent-blue/50 focus:outline-none transition-colors"
              />
              <button
                onClick={() => setShowKey(!showKey)}
                className="absolute right-2 top-1/2 -translate-y-1/2 text-overlay-0 hover:text-text transition-colors"
                title={showKey ? "Hide" : "Show"}
              >
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  {showKey ? (
                    <>
                      <path d="M17.94 17.94A10.07 10.07 0 0112 20c-7 0-11-8-11-8a18.45 18.45 0 015.06-5.94" />
                      <path d="M9.9 4.24A9.12 9.12 0 0112 4c7 0 11 8 11 8a18.5 18.5 0 01-2.16 3.19" />
                      <line x1="1" y1="1" x2="23" y2="23" />
                    </>
                  ) : (
                    <>
                      <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
                      <circle cx="12" cy="12" r="3" />
                    </>
                  )}
                </svg>
              </button>
            </div>
            <button
              onClick={handleSave}
              disabled={saving || !apiKey.trim()}
              className="px-3 py-1.5 bg-accent-blue/15 text-accent-blue text-xs rounded-lg
                         hover:bg-accent-blue/25 transition-colors disabled:opacity-50 shrink-0"
            >
              {saving ? "Saving..." : "Save"}
            </button>
          </div>

          {/* Get API Key link */}
          {API_KEY_LINKS[type] && (
            <button
              onClick={() => openURL(API_KEY_LINKS[type])}
              className="text-[10px] text-accent-blue hover:text-accent-blue/80 transition-colors"
            >
              Get {name.split(" ")[0]} API Key &rarr;
            </button>
          )}

          {/* Actions */}
          <div className="flex items-center gap-2">
            {isConfigured && (
              <>
                <button
                  onClick={handleTest}
                  disabled={testing}
                  className="px-3 py-1.5 bg-accent-blue/15 text-accent-blue text-xs rounded-lg
                             hover:bg-accent-blue/25 transition-colors disabled:opacity-50"
                >
                  {testing ? "Testing..." : "Test Connection"}
                </button>
                <button
                  onClick={handleRemove}
                  className="px-3 py-1.5 bg-red-500/10 text-red-400 text-xs rounded-lg
                             hover:bg-red-500/20 transition-colors"
                >
                  Remove
                </button>
              </>
            )}
          </div>

          {/* Enable/disable toggle */}
          {isConfigured && (
            <ToggleRow
              label="Enabled"
              checked={enabled}
              onChange={handleToggle}
            />
          )}
        </>
      )}

      {/* ChatGPT also gets enable toggle if configured */}
      {isChatGPT && isConfigured && (
        <ToggleRow
          label="Enabled"
          checked={enabled}
          onChange={handleToggle}
        />
      )}

      {/* Test result */}
      {testResult && (
        <div className={`px-3 py-2 rounded-lg text-xs font-mono break-all ${
          testResult.startsWith("ok") || testResult === "Provider saved."
            ? "bg-green-500/10 text-green-400"
            : "bg-red-500/10 text-red-400"
        }`}>
          {testResult.startsWith("ok") ? "Connection successful" : testResult}
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Local Provider Card (Ollama / LM Studio)
// ---------------------------------------------------------------------------

function LocalProviderCard({
  type,
  config,
  onRefresh,
}: {
  type: string;
  config?: ProviderConfig;
  onRefresh: () => void;
}) {
  const [endpoint, setEndpoint] = useState(config?.api_endpoint || DEFAULT_ENDPOINTS[type] || "");
  const [checking, setChecking] = useState(false);
  const [provStatus, setProvStatus] = useState<"unknown" | "running" | "installed" | "not_installed">("unknown");
  const [statusDetail, setStatusDetail] = useState("");
  const [ollamaModels, setOllamaModels] = useState<OllamaModel[]>([]);
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState("");
  const [saving, setSaving] = useState(false);
  const [enabled, setEnabled] = useState(config ? !config.disabled : false);

  const isConfigured = !!config;
  const name = PROVIDER_NAMES[type] || type;
  const description = PROVIDER_DESCRIPTIONS[type] || "";

  const refreshStatus = useCallback(async () => {
    setChecking(true);
    const ep = endpoint.trim() || DEFAULT_ENDPOINTS[type] || "";
    const statusCall = type === "ollama" ? "ollamaStatus" : "lmStudioStatus";
    const raw = await goCall(statusCall, ep);
    if (!raw) {
      setProvStatus("not_installed");
      setStatusDetail("Could not check");
      setChecking(false);
      return;
    }
    try {
      const st = JSON.parse(raw);
      const s = st.status || "not_installed";
      setProvStatus(s);
      if (s === "running") {
        setStatusDetail(st.version ? `Running -- ${st.version}` : "Running");
        if (type === "ollama" && st.models) {
          setOllamaModels(st.models);
        }
      } else if (s === "installed") {
        setStatusDetail("Installed but not running");
      } else {
        setStatusDetail("Not installed");
      }
    } catch {
      setProvStatus("not_installed");
      setStatusDetail("Check failed");
    }
    setChecking(false);
  }, [endpoint, type]);

  useEffect(() => { refreshStatus(); }, [refreshStatus]);

  async function handleSave() {
    setSaving(true);
    const ep = endpoint.trim() || DEFAULT_ENDPOINTS[type] || "";
    const result = await goCall("saveProviderConfig", type, "", ep, "", false);
    setSaving(false);
    if (result && result.startsWith("ok")) {
      setTestResult("Saved.");
      onRefresh();
      refreshStatus();
    } else {
      setTestResult(result || "Failed to save");
    }
  }

  async function handleTest() {
    setTesting(true);
    setTestResult("");
    const result = await goCall("testProviderConnection", type);
    setTestResult(result || "Failed");
    setTesting(false);
  }

  async function handleToggle(v: boolean) {
    setEnabled(v);
    const result = await goCall("toggleProvider", type, v);
    if (result && result.startsWith("error")) {
      setEnabled(!v);
    }
    onRefresh();
  }

  async function handleRemove() {
    await goCall("removeProvider", type);
    setEndpoint(DEFAULT_ENDPOINTS[type] || "");
    setTestResult("");
    onRefresh();
  }

  async function selectOllamaModel(modelName: string) {
    const ep = endpoint.trim() || DEFAULT_ENDPOINTS[type] || "";
    await goCall("saveProviderConfig", type, "", ep, "", false);
    const label = type === "ollama" ? "Ollama" : "LM Studio";
    await goCall("saveModel", label, type, modelName, "");
    onRefresh();
  }

  const statusBadge = isConfigured
    ? config!.disabled ? "disabled" : provStatus === "running" ? "running" : "configured"
    : "none";

  return (
    <div className={`bg-surface-0/30 border rounded-xl p-5 space-y-3 transition-colors ${
      config?.disabled ? "opacity-50 border-surface-0/50" : "border-surface-0/50"
    }`}>
      {/* Header */}
      <div className="flex items-center gap-3">
        <ProviderLogo provider={type} size={24} />
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <span className="text-sm font-medium text-text">{name}</span>
            <span className="text-[10px] px-1.5 py-0.5 rounded bg-green-500/15 text-green-400 font-semibold">FREE</span>
            <StatusBadge status={statusBadge} />
          </div>
          <p className="text-xs text-overlay-0 mt-0.5">{description}</p>
        </div>
        <button
          onClick={() => {
            if (type === "ollama") openURL("https://ollama.com/download");
            else openURL("https://lmstudio.ai");
          }}
          className="text-[10px] text-accent-blue hover:text-accent-blue/80 transition-colors shrink-0"
        >
          Download {name} &rarr;
        </button>
      </div>

      {/* Endpoint input */}
      <div className="flex items-center gap-2">
        <div className="flex-1">
          <input
            type="text"
            value={endpoint}
            onChange={(e) => setEndpoint(e.target.value)}
            placeholder={DEFAULT_ENDPOINTS[type] || "Endpoint URL"}
            className="w-full bg-crust border border-surface-0 rounded-lg px-3 py-1.5 text-xs text-text
                       font-mono focus:border-accent-blue/50 focus:outline-none transition-colors"
          />
        </div>
        <button
          onClick={handleSave}
          disabled={saving}
          className="px-3 py-1.5 bg-accent-blue/15 text-accent-blue text-xs rounded-lg
                     hover:bg-accent-blue/25 transition-colors disabled:opacity-50 shrink-0"
        >
          {saving ? "Saving..." : "Save"}
        </button>
        <button
          onClick={refreshStatus}
          disabled={checking}
          className="px-3 py-1.5 bg-surface-1/50 text-subtext-0 text-xs rounded-lg
                     hover:bg-surface-1 transition-colors disabled:opacity-50 shrink-0"
        >
          {checking ? "Checking..." : "Refresh"}
        </button>
      </div>

      {/* Status */}
      <div className="flex items-center justify-between py-1">
        <span className="text-xs text-overlay-0">Status</span>
        <span className={`text-xs ${provStatus === "running" ? "text-green-400" : provStatus === "installed" ? "text-yellow-400" : "text-overlay-0"}`}>
          {statusDetail || "Unknown"}
        </span>
      </div>

      {/* Ollama installed models */}
      {type === "ollama" && provStatus === "running" && ollamaModels.length > 0 && (
        <div>
          <p className="text-xs text-overlay-0 mb-2">Installed Models</p>
          <div className="space-y-1">
            {ollamaModels.map((m) => {
              const meta = [m.parameter_size, m.quantization_level, m.size_human].filter(Boolean).join(" - ");
              return (
                <div key={m.name} className="flex items-center justify-between bg-crust border border-surface-0 rounded-lg px-3 py-2">
                  <div>
                    <span className="text-xs text-text">{m.name}</span>
                    {meta && <span className="text-[10px] text-overlay-0 ml-2">{meta}</span>}
                  </div>
                  <button
                    onClick={() => selectOllamaModel(m.name)}
                    className="px-2 py-1 rounded text-[10px] font-medium
                               bg-accent-blue/10 text-accent-blue hover:bg-accent-blue/20 transition-colors"
                  >
                    Select
                  </button>
                </div>
              );
            })}
          </div>
        </div>
      )}

      {/* Not running help text */}
      {type === "ollama" && provStatus === "installed" && (
        <p className="text-xs text-overlay-0">
          Run <code className="bg-crust px-1.5 py-0.5 rounded text-yellow-400 text-[10px]">ollama serve</code> then click Refresh.
        </p>
      )}

      {/* Actions */}
      <div className="flex items-center gap-2">
        {isConfigured && (
          <>
            <button
              onClick={handleTest}
              disabled={testing}
              className="px-3 py-1.5 bg-accent-blue/15 text-accent-blue text-xs rounded-lg
                         hover:bg-accent-blue/25 transition-colors disabled:opacity-50"
            >
              {testing ? "Testing..." : "Test Connection"}
            </button>
            <button
              onClick={handleRemove}
              className="px-3 py-1.5 bg-red-500/10 text-red-400 text-xs rounded-lg
                         hover:bg-red-500/20 transition-colors"
            >
              Remove
            </button>
          </>
        )}
      </div>

      {/* Enable/disable toggle */}
      {isConfigured && (
        <ToggleRow
          label="Enabled"
          checked={enabled}
          onChange={handleToggle}
        />
      )}

      {/* Test result */}
      {testResult && (
        <div className={`px-3 py-2 rounded-lg text-xs font-mono break-all ${
          testResult.startsWith("ok") || testResult === "Saved."
            ? "bg-green-500/10 text-green-400"
            : "bg-red-500/10 text-red-400"
        }`}>
          {testResult.startsWith("ok") ? "Connection successful" : testResult}
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// ProvidersTab (main export)
// ---------------------------------------------------------------------------

export function ProvidersTab() {
  const [providers, setProviders] = useState<Record<string, ProviderConfig>>({});
  const [models, setModels] = useState<Record<string, ModelConfig>>({});
  const [loading, setLoading] = useState(true);

  const loadConfig = useCallback(async () => {
    const raw = await goCall("getConfig");
    if (raw) {
      try {
        const cfg = JSON.parse(raw);
        setProviders(cfg.providers || {});
        setModels(cfg.models || {});
      } catch { /* ignore */ }
    }
    setLoading(false);
  }, []);

  useEffect(() => { loadConfig(); }, [loadConfig]);

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20">
        <p className="text-sm text-overlay-0">Loading providers...</p>
      </div>
    );
  }

  return (
    <div className="space-y-8">
      {/* Intro */}
      <div className="text-xs text-overlay-0 leading-relaxed">
        A <strong className="text-text">provider</strong> is the AI service that processes your text.
        Choose a <strong className="text-green-400">local</strong> provider for privacy (your text stays on your machine)
        or a <strong className="text-blue-400">cloud</strong> provider for faster, higher-quality results.
      </div>

      {/* GhostSpell Local */}
      <LocalSection providers={providers} models={models} onRefresh={loadConfig} />

      {/* Other Local Providers */}
      <section>
        <h2 className="text-sm font-medium text-subtext-1 mb-4 tracking-wide uppercase">
          Other Local Providers
        </h2>
        <p className="text-xs text-overlay-0 mb-3">Free, private -- your text never leaves your machine.</p>
        <div className="space-y-4">
          {LOCAL_PROVIDERS.map((type) => (
            <LocalProviderCard
              key={type}
              type={type}
              config={providers[type]}
              onRefresh={loadConfig}
            />
          ))}
        </div>
      </section>

      {/* Cloud Providers */}
      <section>
        <h2 className="text-sm font-medium text-subtext-1 mb-4 tracking-wide uppercase">
          Cloud Providers
        </h2>
        <p className="text-xs text-overlay-0 mb-3">
          Faster, smarter models -- requires an account or API key. Text is sent to the provider's servers.
        </p>
        <div className="space-y-4">
          {CLOUD_PROVIDERS.map((type) => (
            <CloudProviderCard
              key={type}
              type={type}
              config={providers[type]}
              onRefresh={loadConfig}
            />
          ))}
        </div>
      </section>
    </div>
  );
}
