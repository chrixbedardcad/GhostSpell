import { useState, useEffect, useCallback } from "react";
import { goCall, onEvent } from "@/bridge";
import { AboutTab } from "./AboutTab";
import { GeneralTab } from "./GeneralTab";
import { ModelsTab } from "./ModelsTab";
import { PromptsTab } from "./PromptsTab";
import { HotkeysTab } from "./HotkeysTab";
import { LanguageTab } from "./LanguageTab";
import { VoiceTab } from "./VoiceTab";
import { ProvidersTab } from "./ProvidersTab";
import { GPUTab } from "./GPUTab";
import { StatsTab } from "./StatsTab";
import { HistoryTab } from "./HistoryTab";
import { DebugTab } from "./DebugTab";
import { HelpTab } from "./HelpTab";

interface NavItem {
  id: string;
  label: string;
  icon: string;
  group?: string;
}

const NAV: NavItem[] = [
  { id: "general",   label: "General",   icon: "\u2699\uFE0F", group: "Settings" },
  { id: "providers", label: "Providers", icon: "\uD83D\uDD17", group: "Settings" },
  { id: "models",    label: "Models",    icon: "\uD83E\uDDE0", group: "Settings" },
  { id: "prompts",   label: "Skills",    icon: "\u2728",       group: "Settings" },
  { id: "hotkeys",   label: "Hotkeys",   icon: "\u2328\uFE0F", group: "Settings" },
  { id: "language",  label: "Language",  icon: "\uD83C\uDF10", group: "Settings" },
  { id: "voice",     label: "Voice",     icon: "\uD83C\uDFA4", group: "Settings" },
  { id: "gpu",       label: "GPU",       icon: "\u26A1",       group: "Performance" },
  { id: "stats",     label: "Stats",     icon: "\uD83D\uDCCA", group: "Data" },
  { id: "history",   label: "History",   icon: "\uD83D\uDCDD", group: "Data" },
  { id: "debug",     label: "Debug",     icon: "\uD83D\uDD0D", group: "Advanced" },
  { id: "help",      label: "Help",      icon: "?",            group: "Advanced" },
  { id: "about",     label: "About",     icon: "\uD83D\uDC7B", group: "Advanced" },
];

type TabId = (typeof NAV)[number]["id"];

export function SettingsWindow() {
  const [activeTab, setActiveTab] = useState<TabId>("general");
  const [version, setVersion] = useState("");
  const [llmLabel, setLlmLabel] = useState("");
  const [llmModel, setLlmModel] = useState("");
  const [voiceModel, setVoiceModel] = useState("");
  const [gpuOn, setGpuOn] = useState(false);

  const refreshStatus = useCallback(() => {
    goCall("getConfig").then((raw) => {
      if (!raw) return;
      try {
        const cfg = JSON.parse(raw);
        const label = cfg.default_model || "";
        setLlmLabel(label);
        if (label && cfg.models && cfg.models[label]) {
          setLlmModel(cfg.models[label].model || "");
        }
        setVoiceModel(cfg.voice?.model || "");
        setGpuOn(cfg.gpu_enabled !== false);
      } catch { /* ignore */ }
    });
  }, []);

  useEffect(() => {
    goCall("getVersion").then((v) => { if (v) setVersion(v); });
    refreshStatus();
    // Listen for config changes (GPU toggle, model change, etc.)
    const unsub = onEvent("configChanged", refreshStatus);
    return unsub;
  }, [refreshStatus]);

  const groups = NAV.reduce<Record<string, NavItem[]>>((acc, item) => {
    const g = item.group || "";
    if (!acc[g]) acc[g] = [];
    acc[g].push(item);
    return acc;
  }, {});

  return (
    <div className="h-full flex flex-col bg-base">
      {/* Title bar — frameless window, draggable */}
      <div
        className="flex items-center justify-between px-4 h-[36px] shrink-0 bg-crust border-b border-surface-0/30 select-none"
        style={{ ["--wails-draggable" as string]: "drag" }}
      >
        <div className="flex items-center gap-2 text-[11px] text-overlay-0">
          <img src="/dist/ghost-icon.png" alt="" className="w-4 h-4 opacity-70" />
          <span className="font-medium text-subtext-1">GhostSpell</span>
          {version && <span className="text-overlay-0/50">v{version}</span>}
        </div>
        <div
          className="flex items-center gap-1"
          style={{ ["--wails-draggable" as string]: "no-drag" }}
        >
          <button
            onClick={() => goCall("closeWindow")}
            className="w-[28px] h-[24px] flex items-center justify-center rounded
                       text-overlay-0 hover:text-white hover:bg-red-500/80 transition-colors text-[14px]"
            title="Close"
          >{"\u2715"}</button>
        </div>
      </div>

      {/* Main layout: sidebar + content */}
      <div className="flex-1 flex min-h-0">
        {/* Sidebar */}
        <div className="w-[170px] shrink-0 bg-crust border-r border-surface-0/40 flex flex-col overflow-hidden">
          <nav className="flex-1 overflow-y-auto px-2 pt-3 pb-2">
            {Object.entries(groups).map(([group, items]) => (
              <div key={group} className="mb-2">
                <p className="px-2 mb-1 text-[10px] font-medium text-overlay-0/60 uppercase tracking-widest">
                  {group}
                </p>
                {items.map((item) => (
                  <button
                    key={item.id}
                    onClick={() => setActiveTab(item.id)}
                    className={`w-full flex items-center gap-2 px-2.5 py-[5px] rounded-lg text-[12px]
                      transition-all duration-150 mb-[1px] text-left
                      ${activeTab === item.id
                        ? "bg-surface-0/60 text-text font-medium"
                        : "text-overlay-1 hover:text-subtext-0 hover:bg-surface-0/30"
                      }`}
                  >
                    <span className="w-4 text-center text-[11px] opacity-70">{item.icon}</span>
                    {item.label}
                  </button>
                ))}
              </div>
            ))}
          </nav>

          {/* Status strip */}
          <div className="px-3 py-2 border-t border-surface-0/30 space-y-1 shrink-0">
            {llmModel && (
              <div className="flex items-center gap-1.5 text-[10px] text-overlay-0 truncate" title={"LLM: " + llmModel + " (" + llmLabel + ")"}>
                <span className="w-1.5 h-1.5 rounded-full bg-green-400 shrink-0" />
                <span className="truncate">{llmModel}</span>
              </div>
            )}
            {llmLabel && !llmModel && (
              <div className="flex items-center gap-1.5 text-[10px] text-overlay-0 truncate" title={"LLM: " + llmLabel}>
                <span className="w-1.5 h-1.5 rounded-full bg-green-400 shrink-0" />
                <span className="truncate">{llmLabel}</span>
              </div>
            )}
            {voiceModel && (
              <div className="flex items-center gap-1.5 text-[10px] text-overlay-0 truncate" title={"Voice: " + voiceModel}>
                <span className="w-1.5 h-1.5 rounded-full bg-blue-400 shrink-0" />
                <span className="truncate">{voiceModel}</span>
              </div>
            )}
            {gpuOn && (
              <div className="flex items-center gap-1.5 text-[10px] text-overlay-0">
                <span className="w-1.5 h-1.5 rounded-full bg-yellow-400 shrink-0" />
                GPU
              </div>
            )}
            {!llmLabel && !voiceModel && (
              <div className="text-[10px] text-overlay-0/40 italic">No model configured</div>
            )}
          </div>
        </div>

        {/* Content area */}
        <div className="flex-1 flex flex-col min-w-0">
          <div className="px-8 pt-6 pb-4 shrink-0">
            <h1 className="text-[18px] font-semibold text-text">
              {NAV.find((n) => n.id === activeTab)?.label}
            </h1>
          </div>
          <div className="flex-1 overflow-y-auto px-8 pb-8">
            {activeTab === "about" && <AboutTab />}
            {activeTab === "general" && <GeneralTab />}
            {activeTab === "providers" && <ProvidersTab />}
            {activeTab === "models" && <ModelsTab />}
            {activeTab === "prompts" && <PromptsTab />}
            {activeTab === "hotkeys" && <HotkeysTab />}
            {activeTab === "language" && <LanguageTab />}
            {activeTab === "voice" && <VoiceTab />}
            {activeTab === "gpu" && <GPUTab />}
            {activeTab === "stats" && <StatsTab />}
            {activeTab === "history" && <HistoryTab />}
            {activeTab === "debug" && <DebugTab />}
            {activeTab === "help" && <HelpTab />}
          </div>
        </div>
      </div>
    </div>
  );
}
