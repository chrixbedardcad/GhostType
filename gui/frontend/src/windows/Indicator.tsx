import { useState, useEffect, useRef } from "react";
import { goCall, onEvent } from "@/bridge";

/**
 * Ghost Indicator — React content with dynamic window sizing.
 *
 * Hybrid approach (#229):
 * - Go manages window size (48x48 idle, 260x52 pill) and position
 * - React manages the CONTENT (animations, clicks, menu)
 * - Page loads ONCE — no more SetURL reloads
 * - State changes via Wails events from Go
 * - Drag uses Wails native --wails-draggable CSS property
 */

type IndicatorState = "hidden" | "idle" | "processing" | "pop";

interface StateData {
  state: IndicatorState;
  icon?: string;
  name?: string;
  model?: string;
}

interface MenuPrompt {
  name: string;
  icon: string;
  active: boolean;
}

export function IndicatorWindow() {
  const [state, setState] = useState<IndicatorState>("idle");
  const [icon, setIcon] = useState("");
  const [name, setName] = useState("");
  const [model, setModel] = useState("");
  const [elapsed, setElapsed] = useState(0);
  const [menuOpen, setMenuOpen] = useState(false);
  const [menuItems, setMenuItems] = useState<MenuPrompt[]>([]);
  const timerRef = useRef<number | null>(null);
  const [eventsReady, setEventsReady] = useState(false);

  // Set up backgrounds and fetch initial prompt.
  useEffect(() => {
    console.log("[Indicator] React mounted, wails:", typeof window.wails !== "undefined");
    const bg = "rgb(30,30,46)";
    document.documentElement.style.cssText = `background:${bg};margin:0;padding:0;overflow:hidden`;
    document.body.style.cssText = `background:${bg};margin:0;padding:0;overflow:hidden`;
    const root = document.getElementById("root");
    if (root) root.style.cssText = `background:${bg};width:100%;height:100%`;

    // Fetch current active prompt so idle indicator shows which prompt is selected.
    goCall("getActivePromptInfo").then((raw) => {
      if (raw) {
        try {
          const info = JSON.parse(raw);
          if (info.name) setName(info.name);
          if (info.icon) setIcon(info.icon);
          console.log("[Indicator] Active prompt loaded:", info.name);
        } catch { /* ignore */ }
      }
    });
  }, []);

  // Listen for state events from Go — retry subscription until it works.
  useEffect(() => {
    let unsub: (() => void) | null = null;
    let cancelled = false;

    function subscribe() {
      console.log("[Indicator] Attempting event subscription...");
      unsub = onEvent("indicatorState", (data) => {
        const d = data as StateData;
        console.log("[Indicator] Event received:", JSON.stringify(d));
        if (!eventsReady) setEventsReady(true);
        setState(d.state);
        if (d.icon !== undefined) setIcon(d.icon);
        if (d.name !== undefined) setName(d.name);
        if (d.model !== undefined) setModel(d.model);
        setMenuOpen(false);

        // Timer for processing state.
        if (timerRef.current) clearInterval(timerRef.current);
        if (d.state === "processing") {
          setElapsed(0);
          timerRef.current = window.setInterval(() => {
            setElapsed((prev) => prev + 1);
          }, 1000);
        }
      });
    }

    // Try subscribing immediately, then retry every 500ms until wails is ready.
    subscribe();
    const retryInterval = window.setInterval(() => {
      if (cancelled) return;
      if (typeof window.wails !== "undefined" && window.wails.Events) {
        console.log("[Indicator] Wails runtime confirmed ready");
        clearInterval(retryInterval);
        // Re-subscribe to ensure we're connected.
        if (unsub) unsub();
        subscribe();
        // Notify Go that we're ready for events.
        goCall("indicatorReady");
        return;
      }
      console.log("[Indicator] Wails not ready yet, retrying...");
    }, 500);

    return () => {
      cancelled = true;
      clearInterval(retryInterval);
      if (unsub) unsub();
      if (timerRef.current) clearInterval(timerRef.current);
    };
  }, []);

  // --- Click handler: single click expands to show prompt info, double-click opens settings ---
  const clickTimer = useRef<number | null>(null);

  function onClick(e: React.MouseEvent) {
    if (e.detail === 0) return;
    console.log("[Indicator] onClick: state=" + state);
    if (state !== "idle" && state !== "pop") return;
    // Use timer to distinguish single from double click.
    if (clickTimer.current) {
      clearTimeout(clickTimer.current);
      clickTimer.current = null;
      // Double click — open settings.
      console.log("[Indicator] Double-click: opening settings...");
      goCall("openSettingsFromIndicator");
      return;
    }
    clickTimer.current = window.setTimeout(() => {
      clickTimer.current = null;
      // Single click — expand to show current prompt + model.
      console.log("[Indicator] Single-click: showing current prompt...");
      goCall("showCurrentPrompt");
    }, 300);
  }

  async function onContextMenu(e: React.MouseEvent) {
    e.preventDefault();
    console.log("[Indicator] onContextMenu: state=" + state);
    if (state !== "idle" && state !== "pop") return;
    const raw = await goCall("getIndicatorMenu");
    console.log("[Indicator] onContextMenu: menu data:", raw);
    if (raw) {
      try {
        const data = JSON.parse(raw);
        setMenuItems(data.prompts || []);
        setMenuOpen(true);
        const menuH = (data.prompts?.length || 0) * 28 + 60;
        goCall("resizeIndicatorForMenu", 200, Math.max(menuH, 100));
      } catch (err) { console.error("[Indicator] onContextMenu: parse error", err); }
    }
  }

  function closeMenu() {
    setMenuOpen(false);
    if (state === "idle") {
      goCall("resizeIndicatorForMenu", 48, 48);
    } else {
      goCall("resizeIndicatorForMenu", 260, 52);
    }
  }

  async function selectPrompt(idx: number) {
    closeMenu();
    await goCall("setActivePromptFromIndicator", idx);
  }

  // --- Render ---
  const isPill = state === "processing" || state === "pop";

  if (state === "hidden") {
    return <div style={{ background: "rgb(30,30,46)" }} />;
  }

  // Idle state: 48x48 circle with ghost icon, draggable, click opens settings.
  if (!isPill && !menuOpen) {
    return (
      <div
        onClick={onClick}
        onContextMenu={onContextMenu}
        style={{
          // Wails native drag: hold and move to drag the window.
          "--wails-draggable": "drag",
          background: "rgb(30, 30, 46)",
          width: "48px", height: "48px",
          display: "flex", alignItems: "center", justifyContent: "center",
          borderRadius: "50%",
          border: "1px solid rgba(69, 71, 90, 0.5)",
          boxSizing: "border-box",
          cursor: "grab",
        } as React.CSSProperties}
        title={`${icon} ${name}`.trim() || "GhostSpell"}
      >
        <img
          src="/ghostspell-ghost.png"
          alt=""
          style={{
            width: "32px", height: "32px",
            animation: "breathe 3s ease-in-out infinite",
            pointerEvents: "none",
          }}
        />
        <style>{`@keyframes breathe { 0%,100%{transform:scale(1)} 50%{transform:scale(1.06)} }`}</style>
      </div>
    );
  }

  // Pill state (processing/pop): 260x52 pill with icon, prompt name, timer, model.
  if (isPill && !menuOpen) {
    return (
      <div
        style={{
          "--wails-draggable": "drag",
          background: "rgb(30, 30, 46)",
          width: "260px", height: "52px",
          display: "flex", alignItems: "center",
          gap: "8px",
          padding: "6px 14px 6px 6px",
          borderRadius: "16px",
          border: "1px solid rgba(69, 71, 90, 0.5)",
          boxSizing: "border-box",
          cursor: "default",
          overflow: "hidden",
        } as React.CSSProperties}
      >
        <img
          src="/ghostspell-ghost.png"
          alt=""
          style={{
            width: "28px", height: "28px", flexShrink: 0,
            animation: state === "processing" ? "bounce 1.5s ease-in-out infinite" : "none",
            pointerEvents: "none",
          }}
        />
        <div style={{ display: "flex", alignItems: "center", gap: "6px", overflow: "hidden", whiteSpace: "nowrap" }}>
          {icon && <span style={{ fontSize: "14px", flexShrink: 0 }}>{icon}</span>}
          <span style={{
            fontSize: "12px", color: "#cdd6f4", fontWeight: 500,
            maxWidth: "120px", overflow: "hidden", textOverflow: "ellipsis",
            fontFamily: "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif",
          }}>
            {name}
          </span>
          {state === "processing" && (
            <>
              <span style={{ width: "1px", height: "12px", background: "#45475a", flexShrink: 0 }} />
              <span style={{ fontSize: "11px", color: "#6c7086", fontVariantNumeric: "tabular-nums", flexShrink: 0, fontFamily: "monospace" }}>
                {elapsed}s
              </span>
              {model && (
                <span style={{ fontSize: "10px", color: "#585b70", maxWidth: "80px", overflow: "hidden", textOverflow: "ellipsis", flexShrink: 0 }}>
                  {model}
                </span>
              )}
            </>
          )}
        </div>
        <style>{`@keyframes bounce { 0%,100%{transform:translateY(0)} 50%{transform:translateY(-3px)} }`}</style>
      </div>
    );
  }

  // Context menu state.
  return (
    <div style={{
      background: "rgba(24, 24, 37, 0.95)",
      border: "1px solid rgba(69, 71, 90, 0.5)",
      borderRadius: "12px",
      overflow: "hidden",
      minWidth: "180px",
      fontFamily: "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif",
    }}>
      {menuItems.map((item, idx) => (
        <button
          key={idx}
          onClick={() => selectPrompt(idx)}
          style={{
            width: "100%", textAlign: "left", padding: "7px 12px", fontSize: "12px",
            display: "flex", alignItems: "center", gap: "8px",
            background: "none", border: "none", cursor: "pointer",
            color: item.active ? "#89b4fa" : "#a6adc8", transition: "background 150ms",
          }}
          onMouseEnter={(e) => (e.currentTarget.style.background = "rgba(49, 50, 68, 0.5)")}
          onMouseLeave={(e) => (e.currentTarget.style.background = "none")}
        >
          <span style={{ width: "18px", textAlign: "center", flexShrink: 0 }}>{item.icon || "\ud83d\udcdd"}</span>
          <span style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{item.name}</span>
          {item.active && <span style={{ marginLeft: "auto", fontSize: "8px", color: "#89b4fa" }}>\u25cf</span>}
        </button>
      ))}
      <div style={{ height: "1px", background: "rgba(69, 71, 90, 0.4)" }} />
      <button
        onClick={() => { closeMenu(); goCall("openSettingsFromIndicator"); }}
        style={{ width: "100%", textAlign: "left", padding: "7px 12px", fontSize: "12px", display: "flex", alignItems: "center", gap: "8px", background: "none", border: "none", cursor: "pointer", color: "#6c7086", transition: "background 150ms" }}
        onMouseEnter={(e) => (e.currentTarget.style.background = "rgba(49, 50, 68, 0.5)")}
        onMouseLeave={(e) => (e.currentTarget.style.background = "none")}
      >
        \u2699\ufe0f Settings
      </button>
      <button
        onClick={() => { closeMenu(); goCall("quitFromIndicator"); }}
        style={{ width: "100%", textAlign: "left", padding: "7px 12px", fontSize: "12px", display: "flex", alignItems: "center", gap: "8px", background: "none", border: "none", cursor: "pointer", color: "#6c7086", transition: "background 150ms" }}
        onMouseEnter={(e) => { e.currentTarget.style.background = "rgba(49, 50, 68, 0.5)"; e.currentTarget.style.color = "#f38ba8"; }}
        onMouseLeave={(e) => { e.currentTarget.style.background = "none"; e.currentTarget.style.color = "#6c7086"; }}
      >
        \u2715 Quit
      </button>
      <div style={{ position: "fixed", inset: 0, zIndex: -1 }} onClick={closeMenu} />
    </div>
  );
}
