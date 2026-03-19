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

  // Force transparent background — critical for Windows WebView2.
  useEffect(() => {
    console.log("[Indicator] React mounted, setting transparent background");
    console.log("[Indicator] window.wails available:", typeof window.wails !== "undefined");
    document.documentElement.style.cssText = "background:transparent!important;margin:0;padding:0;overflow:hidden";
    document.body.style.cssText = "background:transparent!important;margin:0;padding:0;overflow:hidden";
    const root = document.getElementById("root");
    if (root) root.style.cssText = "background:transparent!important;width:100%;height:100%";
    console.log("[Indicator] Background set, initial state:", state);
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

  // --- Drag support ---
  const dragRef = useRef({ active: false, startX: 0, startY: 0, offX: 0, offY: 0, moved: false });

  const onMouseDown = useCallback((e: React.MouseEvent) => {
    if (e.button !== 0 || menuOpen) return;
    dragRef.current = {
      active: true,
      startX: e.screenX,
      startY: e.screenY,
      offX: e.screenX - window.screenX,
      offY: e.screenY - window.screenY,
      moved: false,
    };
  }, [menuOpen]);

  useEffect(() => {
    const onMove = (e: MouseEvent) => {
      const d = dragRef.current;
      if (!d.active) return;
      if (!d.moved && Math.abs(e.screenX - d.startX) < 5 && Math.abs(e.screenY - d.startY) < 5) return;
      d.moved = true;
      window.moveTo(e.screenX - d.offX, e.screenY - d.offY);
    };
    const onUp = () => {
      const d = dragRef.current;
      if (!d.active) return;
      d.active = false;
      if (d.moved) {
        goCall("saveIndicatorPosition", window.screenX, window.screenY);
      }
    };
    document.addEventListener("mousemove", onMove);
    document.addEventListener("mouseup", onUp);
    return () => { document.removeEventListener("mousemove", onMove); document.removeEventListener("mouseup", onUp); };
  }, []);

  // --- Click handlers ---
  const clickTimer = useRef<number | null>(null);

  function onClick() {
    console.log("[Indicator] Click detected, state:", state, "dragged:", dragRef.current.moved);
    if (dragRef.current.moved) return;
    if (state !== "idle" && state !== "pop") return;
    if (clickTimer.current) { clearTimeout(clickTimer.current); clickTimer.current = null; return; }
    clickTimer.current = window.setTimeout(() => {
      clickTimer.current = null;
      console.log("[Indicator] Cycling prompt...");
      goCall("cyclePromptFromIndicator");
    }, 250);
  }

  function onDoubleClick() {
    if (clickTimer.current) { clearTimeout(clickTimer.current); clickTimer.current = null; }
    goCall("openSettingsFromIndicator");
  }

  async function onContextMenu(e: React.MouseEvent) {
    e.preventDefault();
    if (state !== "idle" && state !== "pop") return;
    const raw = await goCall("getIndicatorMenu");
    if (raw) {
      try {
        const data = JSON.parse(raw);
        setMenuItems(data.prompts || []);
        setMenuOpen(true);
        // Resize window to fit menu.
        const menuH = (data.prompts?.length || 0) * 28 + 60;
        goCall("resizeIndicatorForMenu", 200, Math.max(menuH, 100));
      } catch { /* ignore */ }
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
      style={{ background: "transparent", width: "100%", height: "100%", overflow: "hidden" }}
    >
      {!menuOpen && (
        <div
          onMouseDown={onMouseDown}
          onClick={onClick}
          onDoubleClick={onDoubleClick}
          onContextMenu={onContextMenu}
          style={{
            display: "flex",
            alignItems: "center",
            gap: "8px",
            background: "rgba(30, 30, 46, 0.92)",
            borderRadius: isPill ? "16px" : "50%",
            padding: isPill ? "6px 14px 6px 6px" : "4px",
            cursor: "pointer",
            width: isPill ? "auto" : "40px",
            height: isPill ? "auto" : "40px",
            justifyContent: isPill ? "flex-start" : "center",
            border: "1px solid rgba(69, 71, 90, 0.5)",
            transition: "border-radius 200ms ease, padding 200ms ease",
          }}
        >
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
        </div>
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
