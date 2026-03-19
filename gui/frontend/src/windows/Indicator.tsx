import { useState, useEffect, useRef, useCallback } from "react";
import { goCall, onEvent } from "@/bridge";

/**
 * Ghost Indicator — React content with dynamic window sizing.
 *
 * Hybrid approach (#229):
 * - Go manages window size (48x48 idle, 260x52 pill) and position
 * - React manages the CONTENT (animations, clicks, menu)
 * - Page loads ONCE — no more SetURL reloads
 * - State changes via Wails events from Go
 * - Window background is transparent — only visible content shows
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
  // Start as "idle" so the ghost is visible immediately when the page loads.
  // Go will update the state via events. If events never arrive, the ghost
  // is still visible (helps debug React loading issues).
  const [state, setState] = useState<IndicatorState>("idle");
  const [icon, setIcon] = useState("");
  const [name, setName] = useState("");
  const [model, setModel] = useState("");
  const [elapsed, setElapsed] = useState(0);
  const [menuOpen, setMenuOpen] = useState(false);
  const [menuItems, setMenuItems] = useState<MenuPrompt[]>([]);
  const timerRef = useRef<number | null>(null);

  // Set up backgrounds and log mount status.
  useEffect(() => {
    console.log("[Indicator] React mounted");
    console.log("[Indicator] window.wails available:", typeof window.wails !== "undefined");
    // Match the solid dark background from Go-side BackgroundTypeSolid.
    const bg = "rgb(30,30,46)";
    document.documentElement.style.cssText = `background:${bg};margin:0;padding:0;overflow:hidden`;
    document.body.style.cssText = `background:${bg};margin:0;padding:0;overflow:hidden`;
    const root = document.getElementById("root");
    if (root) root.style.cssText = `background:${bg};width:100%;height:100%`;
    console.log("[Indicator] Background set, initial state:", state);

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

  // Listen for state events from Go.
  useEffect(() => {
    console.log("[Indicator] Registering event listener for indicatorState");
    const unsub = onEvent("indicatorState", (data) => {
      const d = data as StateData;
      console.log("[Indicator] Event received:", JSON.stringify(d));
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

    return () => { unsub(); if (timerRef.current) clearInterval(timerRef.current); };
  }, []);

  // --- Drag support (uses Go-side window positioning since window.moveTo doesn't work in WebView2) ---
  const dragRef = useRef({ active: false, startX: 0, startY: 0, winStartX: 0, winStartY: 0, moved: false });

  const onMouseDown = useCallback(async (e: React.MouseEvent) => {
    if (e.button !== 0 || menuOpen) return;
    console.log("[Indicator] mousedown", "screenX:", e.screenX, "screenY:", e.screenY);
    // Get current window position from Go side.
    let winX = 0, winY = 0;
    try {
      const raw = await goCall("getIndicatorWindowPosition");
      if (raw) {
        const pos = JSON.parse(raw);
        winX = pos.x || 0;
        winY = pos.y || 0;
      }
    } catch { /* ignore */ }
    dragRef.current = {
      active: true,
      startX: e.screenX,
      startY: e.screenY,
      winStartX: winX,
      winStartY: winY,
      moved: false,
    };
  }, [menuOpen]);

  useEffect(() => {
    const onMove = (e: MouseEvent) => {
      const d = dragRef.current;
      if (!d.active) return;
      const dx = e.screenX - d.startX;
      const dy = e.screenY - d.startY;
      if (!d.moved && Math.abs(dx) < 5 && Math.abs(dy) < 5) return;
      d.moved = true;
      goCall("moveIndicatorWindow", d.winStartX + dx, d.winStartY + dy);
    };
    const onUp = async () => {
      const d = dragRef.current;
      if (!d.active) return;
      d.active = false;
      if (d.moved) {
        console.log("[Indicator] drag ended, saving position");
        try {
          const raw = await goCall("getIndicatorWindowPosition");
          if (raw) {
            const pos = JSON.parse(raw);
            goCall("saveIndicatorPosition", pos.x, pos.y);
          }
        } catch { /* ignore */ }
      }
    };
    document.addEventListener("mousemove", onMove);
    document.addEventListener("mouseup", onUp);
    return () => { document.removeEventListener("mousemove", onMove); document.removeEventListener("mouseup", onUp); };
  }, []);

  // --- Click handlers ---
  const clickTimer = useRef<number | null>(null);

  function onClick() {
    console.log("[Indicator] onClick: state=" + state + " dragged=" + dragRef.current.moved);
    if (dragRef.current.moved) { console.log("[Indicator] onClick: ignored (was drag)"); return; }
    if (state !== "idle" && state !== "pop") { console.log("[Indicator] onClick: ignored (state=" + state + ")"); return; }
    if (clickTimer.current) {
      console.log("[Indicator] onClick: double-click detected, clearing single-click timer");
      clearTimeout(clickTimer.current);
      clickTimer.current = null;
      return;
    }
    clickTimer.current = window.setTimeout(() => {
      clickTimer.current = null;
      console.log("[Indicator] onClick: single-click confirmed, cycling prompt...");
      goCall("cyclePromptFromIndicator").then(r => console.log("[Indicator] cyclePrompt result:", r));
    }, 250);
  }

  function onDoubleClick() {
    console.log("[Indicator] onDoubleClick: opening settings");
    if (clickTimer.current) { clearTimeout(clickTimer.current); clickTimer.current = null; }
    goCall("openSettingsFromIndicator").then(r => console.log("[Indicator] openSettings result:", r));
  }

  async function onContextMenu(e: React.MouseEvent) {
    e.preventDefault();
    console.log("[Indicator] onContextMenu: state=" + state);
    if (state !== "idle" && state !== "pop") { console.log("[Indicator] onContextMenu: ignored (state=" + state + ")"); return; }
    const raw = await goCall("getIndicatorMenu");
    console.log("[Indicator] onContextMenu: menu data:", raw);
    if (raw) {
      try {
        const data = JSON.parse(raw);
        setMenuItems(data.prompts || []);
        setMenuOpen(true);
        const menuH = (data.prompts?.length || 0) * 28 + 60;
        console.log("[Indicator] onContextMenu: opening menu, items=" + (data.prompts?.length || 0) + " height=" + menuH);
        goCall("resizeIndicatorForMenu", 200, Math.max(menuH, 100));
      } catch (err) { console.error("[Indicator] onContextMenu: parse error", err); }
    }
  }

  function closeMenu() {
    setMenuOpen(false);
    // Restore window size based on current state.
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
    return <div style={{ background: "transparent" }} />;
  }

  return (
    <div
      onMouseDown={onMouseDown}
      onClick={onClick}
      onDoubleClick={onDoubleClick}
      onContextMenu={onContextMenu}
      title={state === "idle" ? `${icon} ${name}`.trim() || "GhostSpell" : undefined}
      style={{
        background: "rgb(30, 30, 46)",
        width: "100%", height: "100%", overflow: "hidden",
        display: "flex",
        alignItems: "center",
        justifyContent: isPill ? "flex-start" : "center",
        gap: "8px",
        padding: isPill ? "6px 14px 6px 6px" : "0",
        boxSizing: "border-box",
        cursor: "pointer",
        border: "1px solid rgba(69, 71, 90, 0.5)",
        borderRadius: isPill ? "16px" : "50%",
      }}
    >
      {!menuOpen && (
        <>
          {/* Ghost icon */}
          <img
            src="/ghostspell-ghost.png"
            alt=""
            style={{
              width: isPill ? "28px" : "32px",
              height: isPill ? "28px" : "32px",
              flexShrink: 0,
              animation: state === "processing" ? "bounce 1.5s ease-in-out infinite"
                : state === "idle" ? "breathe 3s ease-in-out infinite"
                : "none",
            }}
          />

          {/* Pill content */}
          {isPill && (
            <div style={{ display: "flex", alignItems: "center", gap: "6px", overflow: "hidden", whiteSpace: "nowrap" }}>
              {icon && <span style={{ fontSize: "14px", flexShrink: 0 }}>{icon}</span>}
              <span style={{ fontSize: "12px", color: "#cdd6f4", fontWeight: 500, maxWidth: "120px", overflow: "hidden", textOverflow: "ellipsis", fontFamily: "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif" }}>
                {name}
              </span>
              {state === "processing" && (
                <>
                  <span style={{ width: "1px", height: "12px", background: "#45475a", flexShrink: 0 }} />
                  <span style={{ fontSize: "11px", color: "#6c7086", fontVariantNumeric: "tabular-nums", flexShrink: 0, fontFamily: "monospace" }}>
                    {elapsed}s
                  </span>
                  {model && (
                    <span style={{ fontSize: "10px", color: "#585b70", maxWidth: "80px", overflow: "hidden", textOverflow: "ellipsis", flexShrink: 0, fontFamily: "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif" }}>
                      {model}
                    </span>
                  )}
                </>
              )}
            </div>
          )}
        </>
      )}

      {/* Context menu */}
      {menuOpen && (
        <div
          style={{
            background: "rgba(24, 24, 37, 0.95)",
            border: "1px solid rgba(69, 71, 90, 0.5)",
            borderRadius: "12px",
            overflow: "hidden",
            minWidth: "180px",
            fontFamily: "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif",
          }}
        >
          {menuItems.map((item, idx) => (
            <button
              key={idx}
              onClick={() => selectPrompt(idx)}
              style={{
                width: "100%",
                textAlign: "left",
                padding: "7px 12px",
                fontSize: "12px",
                display: "flex",
                alignItems: "center",
                gap: "8px",
                background: "none",
                border: "none",
                cursor: "pointer",
                color: item.active ? "#89b4fa" : "#a6adc8",
                transition: "background 150ms",
              }}
              onMouseEnter={(e) => (e.currentTarget.style.background = "rgba(49, 50, 68, 0.5)")}
              onMouseLeave={(e) => (e.currentTarget.style.background = "none")}
            >
              <span style={{ width: "18px", textAlign: "center", flexShrink: 0 }}>{item.icon || "📝"}</span>
              <span style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{item.name}</span>
              {item.active && <span style={{ marginLeft: "auto", fontSize: "8px", color: "#89b4fa" }}>●</span>}
            </button>
          ))}
          <div style={{ height: "1px", background: "rgba(69, 71, 90, 0.4)" }} />
          <button
            onClick={() => { closeMenu(); goCall("openSettingsFromIndicator"); }}
            style={{ width: "100%", textAlign: "left", padding: "7px 12px", fontSize: "12px", display: "flex", alignItems: "center", gap: "8px", background: "none", border: "none", cursor: "pointer", color: "#6c7086", transition: "background 150ms" }}
            onMouseEnter={(e) => (e.currentTarget.style.background = "rgba(49, 50, 68, 0.5)")}
            onMouseLeave={(e) => (e.currentTarget.style.background = "none")}
          >
            ⚙️ Settings
          </button>
          <button
            onClick={() => { closeMenu(); goCall("quitFromIndicator"); }}
            style={{ width: "100%", textAlign: "left", padding: "7px 12px", fontSize: "12px", display: "flex", alignItems: "center", gap: "8px", background: "none", border: "none", cursor: "pointer", color: "#6c7086", transition: "background 150ms" }}
            onMouseEnter={(e) => { e.currentTarget.style.background = "rgba(49, 50, 68, 0.5)"; e.currentTarget.style.color = "#f38ba8"; }}
            onMouseLeave={(e) => { e.currentTarget.style.background = "none"; e.currentTarget.style.color = "#6c7086"; }}
          >
            ✕ Quit
          </button>
        </div>
      )}

      {/* Click-outside to close menu */}
      {menuOpen && (
        <div style={{ position: "fixed", inset: 0, zIndex: -1 }} onClick={closeMenu} />
      )}

      <style>{`
        @keyframes breathe { 0%,100%{transform:scale(1)} 50%{transform:scale(1.06)} }
        @keyframes bounce { 0%,100%{transform:translateY(0)} 50%{transform:translateY(-3px)} }
      `}</style>
    </div>
  );
}
