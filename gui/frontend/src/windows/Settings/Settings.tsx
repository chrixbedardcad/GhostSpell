import { useState, useEffect } from "react";
import { goCall } from "@/bridge";
import { TitleBar } from "./TitleBar";
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

/**
 * Settings window — sidebar navigation, zen dark theme.
 * Inspired by Claude Desktop / modern desktop apps.
 */

interface NavItem {
  id: string;
  label: string;
  icon: string;
  group?: string;
}

const NAV: NavItem[] = [
  { id: "general",   label: "General",   icon: "⚙", group: "Settings" },
  { id: "providers", label: "Providers", icon: "🔗", group: "Settings" },
  { id: "models",    label: "Models",    icon: "🧠", group: "Settings" },
  { id: "prompts",   label: "Skills",    icon: "✨", group: "Settings" },
  { id: "hotkeys",   label: "Hotkeys",   icon: "⌨", group: "Settings" },
  { id: "language",  label: "Language",  icon: "🌐", group: "Settings" },
  { id: "voice",     label: "Voice",     icon: "🎤", group: "Settings" },
  { id: "gpu",       label: "GPU",       icon: "⚡", group: "Performance" },
  { id: "stats",     label: "Stats",     icon: "📊", group: "Data" },
  { id: "history",   label: "History",   icon: "📝", group: "Data" },
  { id: "debug",     label: "Debug",     icon: "🔍", group: "Advanced" },
  { id: "help",      label: "Help",      icon: "?",  group: "Advanced" },
  { id: "about",     label: "About",     icon: "👻", group: "Advanced" },
];

type TabId = (typeof NAV)[number]["id"];

export function SettingsWindow() {
  const [activeTab, setActiveTab] = useState<TabId>("general");
  const [version, setVersion] = useState("");
  const [activeModelName, setActiveModelName] = useState("");

  useEffect(() => {
    goCall("getVersion").then((v) => {
      if (v) setVersion(v);
    });
    goCall("getConfig").then((raw) => {
      if (!raw) return;
      try {
        const cfg = JSON.parse(raw);
        setActiveModelName(cfg.active_model_name || cfg.model || "");
      } catch { /* ignore */ }
    });
  }, []);

  // Group nav items
  const groups = NAV.reduce<Record<string, NavItem[]>>((acc, item) => {
    const g = item.group || "";
    if (!acc[g]) acc[g] = [];
    acc[g].push(item);
    return acc;
  }, {});

  return (
    <div className="h-full flex bg-base">
      {/* Sidebar */}
      <div
        className="w-[180px] shrink-0 bg-crust flex flex-col border-r border-surface-0/40"
        style={{ ["--wails-draggable" as string]: "drag" }}
      >
        {/* Logo area */}
        <div className="px-4 pt-5 pb-4 flex items-center gap-2.5">
          <img src="/dist/ghost-icon.png" alt="" className="w-6 h-6 opacity-80" />
          <span className="text-[13px] font-semibold text-subtext-1 tracking-tight">GhostSpell</span>
        </div>

        {/* Navigation */}
        <nav className="flex-1 overflow-y-auto px-2 pb-4" style={{ ["--wails-draggable" as string]: "no-drag" }}>
          {Object.entries(groups).map(([group, items]) => (
            <div key={group} className="mb-3">
              <p className="px-2 mb-1 text-[10px] font-medium text-overlay-0/60 uppercase tracking-widest">
                {group}
              </p>
              {items.map((item) => (
                <button
                  key={item.id}
                  onClick={() => setActiveTab(item.id)}
                  className={`w-full flex items-center gap-2.5 px-2.5 py-[6px] rounded-lg text-[12.5px]
                    transition-all duration-150 mb-[1px]
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

        {/* Version + active model */}
        {(version || activeModelName) && (
          <div className="px-3 pb-2" style={{ ["--wails-draggable" as string]: "no-drag" }}>
            <div className="px-2 py-2 space-y-0.5">
              {version && (
                <p className="text-[10px] text-overlay-0/60 truncate">v{version}</p>
              )}
              {activeModelName && (
                <p className="text-[10px] text-overlay-0/60 truncate" title={activeModelName}>
                  {activeModelName}
                </p>
              )}
            </div>
          </div>
        )}

        {/* Close button (Windows frameless) */}
        <div className="px-3 pb-3" style={{ ["--wails-draggable" as string]: "no-drag" }}>
          <button
            onClick={() => window.wails?.Window?.Close()}
            className="w-full py-1.5 rounded-lg text-[11px] text-overlay-0
                       hover:text-accent-red hover:bg-surface-0/30 transition-colors"
          >
            Close
          </button>
        </div>
      </div>

      {/* Content area */}
      <div className="flex-1 flex flex-col min-w-0">
        {/* Content header */}
        <div
          className="px-6 pt-5 pb-3 shrink-0"
          style={{ ["--wails-draggable" as string]: "drag" }}
        >
          <h1 className="text-[15px] font-semibold text-text">
            {NAV.find((n) => n.id === activeTab)?.label}
          </h1>
        </div>

        {/* Scrollable content */}
        <div className="flex-1 overflow-y-auto px-6 pb-6">
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
  );
}
