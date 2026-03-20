import { useState } from "react";
import { TitleBar } from "./TitleBar";
import { AboutTab } from "./AboutTab";
import { GeneralTab } from "./GeneralTab";
import { ModelsTab } from "./ModelsTab";
import { PromptsTab } from "./PromptsTab";
import { HotkeysTab } from "./HotkeysTab";
import { StatsTab } from "./StatsTab";
import { DebugTab } from "./DebugTab";
import { HelpTab } from "./HelpTab";

/**
 * Settings window — 9-tab interface.
 * Zen: clean tab bar, generous content area, no visual noise.
 */

const TABS = [
  { id: "about", label: "About" },
  { id: "general", label: "General" },
  { id: "models", label: "Models" },
  { id: "prompts", label: "Skills" },
  { id: "hotkeys", label: "Hotkeys" },
  { id: "stats", label: "Stats" },
  { id: "debug", label: "Debug" },
  { id: "help", label: "Help" },
] as const;

type TabId = (typeof TABS)[number]["id"];

export function SettingsWindow() {
  const [activeTab, setActiveTab] = useState<TabId>("about");

  return (
    <div className="h-full flex flex-col bg-base">
      <TitleBar />

      {/* Tab bar */}
      <div className="flex border-b border-surface-0/60 px-4 shrink-0 bg-mantle/50 overflow-x-auto">
        {TABS.map((tab) => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id)}
            className={`relative px-3 py-2.5 text-[12px] font-medium transition-colors whitespace-nowrap
              ${activeTab === tab.id
                ? "text-accent-blue"
                : "text-overlay-0 hover:text-subtext-0"
              }`}
          >
            {tab.label}
            {activeTab === tab.id && (
              <span className="absolute bottom-0 left-1.5 right-1.5 h-0.5 bg-accent-blue rounded-full" />
            )}
          </button>
        ))}
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto px-6 py-6">
        {activeTab === "about" && <AboutTab />}
        {activeTab === "general" && <GeneralTab />}
        {activeTab === "models" && <ModelsTab />}
        {activeTab === "prompts" && <PromptsTab />}
        {activeTab === "hotkeys" && <HotkeysTab />}
        {activeTab === "stats" && <StatsTab />}
        {activeTab === "debug" && <DebugTab />}
        {activeTab === "help" && <HelpTab />}
      </div>
    </div>
  );
}
