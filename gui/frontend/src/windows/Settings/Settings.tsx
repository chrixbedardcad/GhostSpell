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

/*
 * Design system — consistent across sidebar + content:
 *
 * Font sizes:
 *   Page title:      16px  semibold
 *   Section header:  11px  semibold  uppercase  tracking-wider  text-overlay-0
 *   Card label:      13px  medium    text-text
 *   Card desc:       12px  normal    text-overlay-0
 *   Sidebar nav:     13px  medium    (active: text-text, inactive: text-overlay-1)
 *   Sidebar group:   10px  medium    uppercase  tracking-widest  text-overlay-0/50
 *   Status strip:    11px  normal    text-overlay-0
 *
 * Spacing:
 *   Between sections:    space-y-8  (32px)
 *   Between cards:       space-y-3  (12px)
 *   Card padding:        px-5 py-4
 *   Content padding:     px-8 pb-8
 *   Sidebar width:       200px
 */

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
      {/* Title bar */}
      <div
        className="flex items-center justify-between px-5 h-[38px] shrink-0 bg-crust border-b border-surface-0/30 select-none"
        style={{ ["--wails-draggable" as string]: "drag" }}
      >
        <div className="flex items-center gap-2">
          <img src="/dist/ghost-icon.png" alt="" className="w-4 h-4 opacity-70" />
          <span className="text-[12px] font-medium text-subtext-0">GhostSpell</span>
          {version && <span className="text-[11px] text-overlay-0/50">v{version}</span>}
        </div>
        <div style={{ ["--wails-draggable" as string]: "no-drag" }}>
          <button
            onClick={() => goCall("closeWindow")}
            className="w-[30px] h-[24px] flex items-center justify-center rounded
                       text-overlay-0 hover:text-white hover:bg-red-500/80 transition-colors text-[14px]"
          >{"\u2715"}</button>
        </div>
      </div>

      {/* Main: sidebar + content */}
      <div className="flex-1 flex min-h-0">
        {/* Sidebar */}
        <div className="w-[200px] shrink-0 bg-crust border-r border-surface-0/30 flex flex-col">
          <nav className="flex-1 overflow-y-auto px-3 pt-4 pb-3">
            {Object.entries(groups).map(([group, items]) => (
              <div key={group} className="mb-4">
                <p className="px-3 mb-2 text-[10px] font-semibold text-overlay-0/50 uppercase tracking-widest">
                  {group}
                </p>
                {items.map((item) => (
                  <button
                    key={item.id}
                    onClick={() => setActiveTab(item.id)}
                    className={`w-full flex items-center gap-3 px-3 py-[7px] rounded-lg text-[13px]
                      transition-all duration-150 mb-[2px] text-left
                      ${activeTab === item.id
                        ? "bg-surface-0/50 text-text font-medium"
                        : "text-overlay-1 hover:text-subtext-0 hover:bg-surface-0/25"
                      }`}
                  >
                    <span className="w-5 text-center text-[13px] opacity-60">{item.icon}</span>
                    <span>{item.label}</span>
                  </button>
                ))}
              </div>
            ))}
          </nav>

          {/* Status strip */}
          <div className="px-4 py-3 border-t border-surface-0/20 space-y-1.5 shrink-0">
            {llmModel && (
              <div className="flex items-center gap-2 text-[11px] text-overlay-0 truncate" title={"LLM: " + llmModel}>
                <span className="w-2 h-2 rounded-full bg-green-400 shrink-0" />
                <span className="truncate">{llmModel}</span>
              </div>
            )}
            {llmLabel && !llmModel && (
              <div className="flex items-center gap-2 text-[11px] text-overlay-0 truncate" title={"LLM: " + llmLabel}>
                <span className="w-2 h-2 rounded-full bg-green-400 shrink-0" />
                <span className="truncate">{llmLabel}</span>
              </div>
            )}
            {voiceModel && (
              <div className="flex items-center gap-2 text-[11px] text-overlay-0 truncate" title={"Voice: " + voiceModel}>
                <span className="w-2 h-2 rounded-full bg-blue-400 shrink-0" />
                <span className="truncate">{voiceModel}</span>
              </div>
            )}
            {gpuOn && (
              <div className="flex items-center gap-2 text-[11px] text-overlay-0">
                <span className="w-2 h-2 rounded-full bg-yellow-400 shrink-0" />
                GPU
              </div>
            )}
            {!llmLabel && !voiceModel && (
              <div className="text-[11px] text-overlay-0/30 italic">No model configured</div>
            )}
          </div>
        </div>

        {/* Content */}
        <div className="flex-1 flex flex-col min-w-0 bg-base">
          <div className="px-8 pt-7 pb-2 shrink-0">
            <h1 className="text-[16px] font-semibold text-text">
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
