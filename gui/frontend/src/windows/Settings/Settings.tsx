import { useState } from "react";
import { TitleBar } from "./TitleBar";
import { AboutTab } from "./AboutTab";
import { GeneralTab } from "./GeneralTab";
import { DebugTab } from "./DebugTab";
import { HelpTab } from "./HelpTab";

/**
 * Settings window — tabbed interface.
 * Zen: clean tab bar, generous content area, no visual noise.
 */

const TABS = [
  { id: "about", label: "About" },
  { id: "general", label: "General" },
  { id: "models", label: "Models" },
  { id: "prompts", label: "Prompts" },
  { id: "hotkeys", label: "Hotkeys" },
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
      <div className="flex border-b border-surface-0/60 px-6 shrink-0 bg-mantle/50">
        {TABS.map((tab) => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id)}
            className={`relative px-4 py-2.5 text-[13px] font-medium transition-colors
              ${activeTab === tab.id
                ? "text-accent-blue"
                : "text-overlay-0 hover:text-subtext-0"
              }`}
          >
            {tab.label}
            {/* Active indicator line */}
            {activeTab === tab.id && (
              <span className="absolute bottom-0 left-2 right-2 h-0.5 bg-accent-blue rounded-full" />
            )}
          </button>
        ))}
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto px-6 py-6">
        {activeTab === "about" && <AboutTab />}
        {activeTab === "general" && <GeneralTab />}
        {activeTab === "models" && <ModelsPlaceholder />}
        {activeTab === "prompts" && <PromptsPlaceholder />}
        {activeTab === "hotkeys" && <HotkeysPlaceholder />}
        {activeTab === "debug" && <DebugTab />}
        {activeTab === "help" && <HelpTab />}
      </div>
    </div>
  );
}

/* Placeholders for complex tabs — will be built next */
function ModelsPlaceholder() {
  return (
    <div className="flex items-center justify-center py-20">
      <p className="text-sm text-overlay-0">Models tab — building...</p>
    </div>
  );
}
function PromptsPlaceholder() {
  return (
    <div className="flex items-center justify-center py-20">
      <p className="text-sm text-overlay-0">Prompts tab — building...</p>
    </div>
  );
}
function HotkeysPlaceholder() {
  return (
    <div className="flex items-center justify-center py-20">
      <p className="text-sm text-overlay-0">Hotkeys tab — building...</p>
    </div>
  );
}
