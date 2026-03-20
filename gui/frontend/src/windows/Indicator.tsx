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

type IndicatorState = "hidden" | "idle" | "processing" | "pop" | "done";

interface StateData {
  state: IndicatorState;
  icon?: string;
  name?: string;
  model?: string;
  elapsed?: number;
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
  const [doneElapsed, setDoneElapsed] = useState(0);
  const [menuOpen, setMenuOpen] = useState(false);
  const [menuItems, setMenuItems] = useState<MenuPrompt[]>([]);
  const [menuVersion, setMenuVersion] = useState("");
  const [menuModel, setMenuModel] = useState("");
  const [menuMode, setMenuMode] = useState("processing");
  const [isVoice, setIsVoice] = useState(false);
  const [isVision, setIsVision] = useState(false);
  const timerRef = useRef<number | null>(null);
  const [eventsReady, setEventsReady] = useState(false);

  // Fetch active prompt info (name, icon, voice, vision flags).
  function fetchPromptInfo() {
    goCall("getActivePromptInfo").then((raw) => {
      if (raw) {
        try {
          const info = JSON.parse(raw);
          if (info.name) setName(info.name);
          if (info.icon) setIcon(info.icon);
          setIsVoice(!!info.voice);
          setIsVision(!!info.vision);
        } catch { /* ignore */ }
      }
    });
  }

  // Set up backgrounds and fetch initial prompt.
  useEffect(() => {
    console.log("[Indicator] React mounted, wails:", typeof window.wails !== "undefined");
    document.documentElement.style.cssText = "background:transparent;margin:0;padding:0;overflow:hidden";
    document.body.style.cssText = "background:transparent;margin:0;padding:0;overflow:hidden";
    const root = document.getElementById("root");
    if (root) root.style.cssText = "background:transparent;width:100%;height:100%";
    fetchPromptInfo();
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
        // Capture final elapsed for done state.
        if (d.state === "done" && d.elapsed !== undefined) {
          setDoneElapsed(d.elapsed);
        }
        // Re-fetch prompt info on idle to update voice/vision badges.
        if (d.state === "idle") {
          fetchPromptInfo();
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

  // --- Click handler ---
  // idle: single click = show prompt info, double click = open settings
  // pop/pill: single click = cycle to next prompt, double click = open settings
  const clickTimer = useRef<number | null>(null);

  function onClick(e: React.MouseEvent) {
    if (e.detail === 0) return;
    console.log("[Indicator] onClick: state=" + state);
    if (state !== "idle" && state !== "pop") return;
    if (clickTimer.current) {
      clearTimeout(clickTimer.current);
      clickTimer.current = null;
      console.log("[Indicator] Double-click: opening settings...");
      goCall("openSettingsFromIndicator");
      return;
    }
    clickTimer.current = window.setTimeout(() => {
      clickTimer.current = null;
      if (state === "pop") {
        // Already expanded — cycle to next prompt.
        console.log("[Indicator] Click on pill: cycling prompt...");
        goCall("cyclePromptFromIndicator");
      } else {
        // Idle — expand to show current prompt + model.
        console.log("[Indicator] Click on idle: showing current prompt...");
        goCall("showCurrentPrompt");
      }
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
        if (data.version) setMenuVersion(data.version);
        if (data.activeModel) setMenuModel(data.activeModel);
        if (data.indicatorMode) setMenuMode(data.indicatorMode);
        setMenuOpen(true);
        // Height: version(26) + prompts(34 each) + divider(5) + "Display" label(22) + 3 modes(28 each) + divider(5) + settings(34) + quit(34) + padding(20)
        const menuH = 26 + (data.prompts?.length || 0) * 34 + 5 + 22 + 3 * 28 + 5 + 34 + 34 + 20;
        goCall("resizeIndicatorForMenu", 220, menuH);
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

  async function setDisplayMode(mode: string) {
    setMenuMode(mode);
    closeMenu();
    await goCall("setIndicatorModeFromIndicator", mode);
  }

  // --- Badge style for mic/camera on idle circle ---
  const badgeStyle = (position: "bottom-right" | "bottom-left"): React.CSSProperties => ({
    position: "absolute",
    ...(position === "bottom-right" ? { bottom: "3px", right: "2px" } : { bottom: "3px", left: "2px" }),
    fontSize: "10px",
    lineHeight: 1,
    pointerEvents: "none",
  });

  // --- Render ---
  const isPill = state === "processing" || state === "pop" || state === "done";

  if (state === "hidden") {
    return <div style={{ background: "rgb(30,30,46)" }} />;
  }

  // Idle state: 48x48 circle with ghost icon + active prompt icon overlay + badges.
  if (!isPill && !menuOpen) {
    return (
      <div
        onClick={onClick}
        onContextMenu={onContextMenu}
        style={{
          "--wails-draggable": "drag",
          background: "rgb(30, 30, 46)",
          width: "48px", height: "48px",
          display: "flex", alignItems: "center", justifyContent: "center",
          borderRadius: "50%",
          border: "1px solid rgba(69, 71, 90, 0.5)",
          boxSizing: "border-box",
          cursor: "default",
          opacity: 0.5,
          transition: "opacity 200ms",
          position: "relative",
          overflow: "visible",
        } as React.CSSProperties}
        title={`${icon} ${name}`.trim() || "GhostSpell"}
        onMouseEnter={(e) => (e.currentTarget.style.opacity = "1")}
        onMouseLeave={(e) => (e.currentTarget.style.opacity = "0.5")}
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
        {icon && (
          <span style={{
            position: "absolute", top: "2px", left: "2px",
            fontSize: "12px", lineHeight: 1, pointerEvents: "none",
          }}>{icon}</span>
        )}
        {isVoice && <span style={badgeStyle("bottom-right")}>{"\uD83C\uDF99\uFE0F"}</span>}
        {isVision && <span style={badgeStyle("bottom-left")}>{"\uD83D\uDCF7"}</span>}
        <style>{`@keyframes breathe { 0%,100%{transform:scale(1)} 50%{transform:scale(1.06)} }`}</style>
      </div>
    );
  }

  // Pill state (processing/pop): pill with icon, prompt name, timer, model.
  if (isPill && !menuOpen) {
    return (
      <div
        onClick={onClick}
        onContextMenu={onContextMenu}
        style={{
          "--wails-draggable": "drag",
          background: "rgb(30, 30, 46)",
          width: "fit-content", maxWidth: "300px", minWidth: "120px", height: "52px",
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
        <div style={{ display: "flex", flexDirection: "column", overflow: "hidden", whiteSpace: "nowrap", gap: "1px" }}>
          <div style={{ display: "flex", alignItems: "center", gap: "6px" }}>
            {icon && <span style={{ fontSize: "14px", flexShrink: 0 }}>{icon}</span>}
            <span style={{
              fontSize: "12px", color: "#cdd6f4", fontWeight: 500,
              maxWidth: "150px", overflow: "hidden", textOverflow: "ellipsis",
              fontFamily: "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif",
            }}>
              {name}
            </span>
            {state === "processing" && (
              <>
                <span style={{ width: "1px", height: "16px", background: "#45475a", flexShrink: 0 }} />
                <span style={{
                  fontSize: "14px", color: "#f9e2af", fontWeight: 600,
                  fontVariantNumeric: "tabular-nums", flexShrink: 0, fontFamily: "monospace",
                  animation: "pulse 1.5s ease-in-out infinite",
                }}>
                  {elapsed}s
                </span>
              </>
            )}
            {state === "done" && (
              <>
                <span style={{ width: "1px", height: "16px", background: "#45475a", flexShrink: 0 }} />
                <span style={{
                  fontSize: "14px", color: "#a6e3a1", fontWeight: 600,
                  fontVariantNumeric: "tabular-nums", flexShrink: 0, fontFamily: "monospace",
                }}>
                  {doneElapsed.toFixed(1)}s
                </span>
              </>
            )}
          </div>
          {model && (
            <span style={{ fontSize: "9px", color: "#585b70", paddingLeft: icon ? "22px" : "0",
              fontFamily: "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif" }}>
              {model}
            </span>
          )}
        </div>
        <style>{`
          @keyframes bounce { 0%,100%{transform:translateY(0)} 50%{transform:translateY(-3px)} }
          @keyframes pulse { 0%,100%{opacity:1} 50%{opacity:0.5} }
        `}</style>
      </div>
    );
  }

  // Context menu state.
  const mBtn: React.CSSProperties = {
    width: "100%", textAlign: "left", padding: "8px 12px", fontSize: "12px",
    display: "flex", alignItems: "center", gap: "8px",
    background: "none", border: "none", cursor: "pointer",
    color: "#a6adc8", transition: "background 150ms",
  };
  const modeOptions = [
    { key: "always", label: "Always visible" },
    { key: "processing", label: "Process only" },
    { key: "hidden", label: "Hidden" },
  ];
  return (
    <div style={{
      background: "rgba(24, 24, 37, 0.98)",
      border: "1px solid rgba(69, 71, 90, 0.5)",
      borderRadius: "12px",
      overflow: "hidden",
      minWidth: "200px",
      fontFamily: "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif",
    }}>
      {(menuVersion || menuModel) && (
        <div style={{ padding: "6px 12px 4px", fontSize: "10px", color: "#585b70", letterSpacing: "0.5px", display: "flex", justifyContent: "space-between" }}>
          <span>GhostSpell v{menuVersion}</span>
          {menuModel && <span style={{ color: "#6c7086" }}>{menuModel}</span>}
        </div>
      )}
      {menuItems.map((item, idx) => (
        <button
          key={idx}
          onClick={() => selectPrompt(idx)}
          style={{ ...mBtn, color: item.active ? "#89b4fa" : "#a6adc8" }}
          onMouseEnter={(e) => (e.currentTarget.style.background = "rgba(49, 50, 68, 0.5)")}
          onMouseLeave={(e) => (e.currentTarget.style.background = "none")}
        >
          <span style={{ width: "18px", textAlign: "center", flexShrink: 0 }}>{item.icon || "\ud83d\udcdd"}</span>
          <span style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{item.name}</span>
          {item.active && <span style={{ marginLeft: "auto", fontSize: "8px", color: "#89b4fa" }}>{"\u25cf"}</span>}
        </button>
      ))}
      <div style={{ height: "1px", background: "rgba(69, 71, 90, 0.4)", margin: "2px 0" }} />
      {/* Display mode section */}
      <div style={{ padding: "4px 12px 2px", fontSize: "10px", color: "#585b70", letterSpacing: "0.5px" }}>
        Display
      </div>
      {modeOptions.map((opt) => (
        <button
          key={opt.key}
          onClick={() => setDisplayMode(opt.key)}
          style={{ ...mBtn, padding: "6px 12px", fontSize: "11px", color: menuMode === opt.key ? "#89b4fa" : "#a6adc8" }}
          onMouseEnter={(e) => (e.currentTarget.style.background = "rgba(49, 50, 68, 0.5)")}
          onMouseLeave={(e) => (e.currentTarget.style.background = "none")}
        >
          <span style={{ width: "18px", textAlign: "center", flexShrink: 0, fontSize: "8px" }}>
            {menuMode === opt.key ? "\u25cf" : ""}
          </span>
          <span>{opt.label}</span>
        </button>
      ))}
      <div style={{ height: "1px", background: "rgba(69, 71, 90, 0.4)", margin: "2px 0" }} />
      <button onClick={() => { closeMenu(); goCall("openSettingsFromIndicator"); }}
        style={mBtn}
        onMouseEnter={(e) => (e.currentTarget.style.background = "rgba(49, 50, 68, 0.5)")}
        onMouseLeave={(e) => (e.currentTarget.style.background = "none")}
      >{"\u2699\ufe0f"} Settings</button>
      <button onClick={() => { closeMenu(); goCall("quitFromIndicator"); }}
        style={mBtn}
        onMouseEnter={(e) => { e.currentTarget.style.background = "rgba(49, 50, 68, 0.5)"; e.currentTarget.style.color = "#f38ba8"; }}
        onMouseLeave={(e) => { e.currentTarget.style.background = "none"; e.currentTarget.style.color = "#a6adc8"; }}
      >{"\u2715"} Quit</button>
      <div style={{ position: "fixed", inset: 0, zIndex: -1 }} onClick={closeMenu} />
    </div>
  );
}
