import { usePlatform } from "@/hooks/usePlatform";
import { goCall } from "@/bridge";
import { useEffect, useState } from "react";

/**
 * Settings title bar — draggable, with close button on Windows.
 * Zen: minimal, no clutter. Ghost icon + name + version.
 */
export function TitleBar() {
  const platform = usePlatform();
  const [version, setVersion] = useState("");

  useEffect(() => {
    goCall("getVersion").then((v) => v && setVersion(v));
  }, []);

  return (
    <div
      className="flex items-center gap-3 px-6 py-4 border-b border-surface-0/60 shrink-0"
      style={{ ["--wails-draggable" as string]: "drag" }}
    >
      <img src="/ghost-icon.png" alt="" className="w-8 h-8 opacity-80" />
      <div className="min-w-0">
        <h1 className="text-[15px] font-medium text-text tracking-tight">Settings</h1>
        {version && (
          <span className="text-[11px] text-overlay-0">v{version}</span>
        )}
      </div>

      {/* Spacer */}
      <div className="flex-1" />

      {/* Close button — Windows only (frameless) */}
      {platform === "windows" && (
        <button
          onClick={() => window.wails.Window.Close()}
          className="text-overlay-0 hover:text-accent-red rounded-md px-2 py-1
                     hover:bg-surface-0/50 transition-colors text-sm leading-none"
          style={{ ["--wails-draggable" as string]: "no-drag" }}
        >
          ✕
        </button>
      )}
    </div>
  );
}
