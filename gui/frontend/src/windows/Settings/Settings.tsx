import { useConfig } from "@/hooks/useConfig";
import { usePlatform } from "@/hooks/usePlatform";

/**
 * Settings window — placeholder for Phase 1 migration.
 * Will contain 7 tabs: About, General, Models, Prompts, Hotkeys, Debug, Help.
 */
export function SettingsWindow() {
  const { config, loading } = useConfig();
  const platform = usePlatform();

  return (
    <div className="h-full flex flex-col bg-base">
      {/* Title bar */}
      <div
        className="flex items-center gap-3 px-6 py-4 border-b border-surface-0"
        style={{ ["--wails-draggable" as string]: "drag" }}
      >
        <img src="/ghost-icon.png" alt="" className="w-9 h-9" />
        <div>
          <h1 className="text-lg font-semibold text-text">GhostSpell Settings</h1>
          <span className="text-xs text-overlay-0">
            {loading ? "Loading..." : `v${(config as Record<string, unknown>)?.version ?? ""}`}
          </span>
        </div>
        <div className="ml-auto">
          {platform === "windows" && (
            <button
              onClick={() => window.wails.Window.Close()}
              className="text-overlay-0 hover:text-accent-red px-2 py-1 rounded text-lg leading-none"
            >
              ✕
            </button>
          )}
        </div>
      </div>

      {/* Content */}
      <div className="flex-1 flex items-center justify-center">
        <div className="text-center space-y-4">
          <div className="text-6xl opacity-20">👻</div>
          <p className="text-subtext-0 text-sm">Settings — React migration in progress</p>
          <p className="text-overlay-0 text-xs">Phase 1 will replace this with the full settings UI</p>
        </div>
      </div>
    </div>
  );
}
