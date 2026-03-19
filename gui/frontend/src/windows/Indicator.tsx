import { useState, useEffect, useRef, useCallback } from "react";
import { goCall, onEvent } from "@/bridge";

/**
 * Ghost Indicator — fixed-size transparent overlay.
 *
 * Architecture:
 * - 320x400 transparent window, never resizes
 * - Ghost circle at top-left, pill expands right, menu drops down
 * - All state changes via Wails events from Go
 * - Drag to reposition, position saved to config
 * - CSS transitions for smooth state changes
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
  const [state, setState] = useState<IndicatorState>("hidden");
  const [icon, setIcon] = useState("");
  const [name, setName] = useState("");
  const [model, setModel] = useState("");
  const [elapsed, setElapsed] = useState(0);
  const [menuOpen, setMenuOpen] = useState(false);
  const [menuItems, setMenuItems] = useState<MenuPrompt[]>([]);
  const timerRef = useRef<number | null>(null);
  const dragRef = useRef({
    dragging: false,
    startX: 0,
    startY: 0,
    offsetX: 0,
    offsetY: 0,
    moved: false,
  });

  // Listen for state events from Go
  useEffect(() => {
    const unsub = onEvent("indicatorState", (data) => {
      const d = data as StateData;
      setState(d.state);
      if (d.icon !== undefined) setIcon(d.icon);
      if (d.name !== undefined) setName(d.name);
      if (d.model !== undefined) setModel(d.model);
      setMenuOpen(false);

      // Reset timer
      if (timerRef.current) clearInterval(timerRef.current);
      if (d.state === "processing") {
        setElapsed(0);
        timerRef.current = window.setInterval(() => {
          setElapsed((prev) => prev + 1);
        }, 1000);
      }
    });

    // Check URL params for initial state (fallback for old Go code)
    const params = new URLSearchParams(window.location.search);
    if (params.get("state") === "idle") setState("idle");

    return () => { unsub(); if (timerRef.current) clearInterval(timerRef.current); };
  }, []);

  // --- Drag support ---
  const onMouseDown = useCallback((e: React.MouseEvent) => {
    if (e.button !== 0) return; // left click only
    dragRef.current = {
      dragging: true,
      startX: e.screenX,
      startY: e.screenY,
      offsetX: e.screenX - window.screenX,
      offsetY: e.screenY - window.screenY,
      moved: false,
    };
    e.preventDefault();
  }, []);

  useEffect(() => {
    function onMouseMove(e: MouseEvent) {
      const d = dragRef.current;
      if (!d.dragging) return;
      const dx = e.screenX - d.startX;
      const dy = e.screenY - d.startY;
      // 5px threshold to distinguish click from drag
      if (!d.moved && Math.abs(dx) < 5 && Math.abs(dy) < 5) return;
      d.moved = true;
      window.moveTo(e.screenX - d.offsetX, e.screenY - d.offsetY);
    }

    function onMouseUp(e: MouseEvent) {
      const d = dragRef.current;
      if (!d.dragging) return;
      d.dragging = false;
      if (d.moved) {
        // Save position
        goCall("saveIndicatorPosition", window.screenX, window.screenY);
      }
    }

    document.addEventListener("mousemove", onMouseMove);
    document.addEventListener("mouseup", onMouseUp);
    return () => {
      document.removeEventListener("mousemove", onMouseMove);
      document.removeEventListener("mouseup", onMouseUp);
    };
  }, []);

  // --- Click handlers ---
  const clickTimerRef = useRef<number | null>(null);

  function onClick(e: React.MouseEvent) {
    if (dragRef.current.moved) return; // was a drag, not a click
    if (state !== "idle" && state !== "pop") return;

    // Debounce for double-click detection
    if (clickTimerRef.current) {
      clearTimeout(clickTimerRef.current);
      clickTimerRef.current = null;
      return; // double-click handled below
    }

    clickTimerRef.current = window.setTimeout(async () => {
      clickTimerRef.current = null;
      // Single click → cycle prompt
      await goCall("cyclePromptFromIndicator");
    }, 250);
  }

  function onDoubleClick() {
    if (clickTimerRef.current) {
      clearTimeout(clickTimerRef.current);
      clickTimerRef.current = null;
    }
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
      } catch { /* ignore */ }
    }
  }

  async function selectPrompt(idx: number) {
    setMenuOpen(false);
    await goCall("setActivePromptFromIndicator", idx);
  }

  // --- Render ---
  const isVisible = state !== "hidden";
  const isPill = state === "processing" || state === "pop";

  return (
    <div className="w-full h-full relative" style={{ background: "transparent" }}>
      {/* Ghost indicator */}
      <div
        className={`absolute top-1 left-1 transition-all duration-200 ease-out cursor-pointer
          ${isVisible ? "opacity-100 scale-100" : "opacity-0 scale-75 pointer-events-none"}
        `}
        onMouseDown={onMouseDown}
        onClick={onClick}
        onDoubleClick={onDoubleClick}
        onContextMenu={onContextMenu}
      >
        {/* The pill container — morphs between circle and pill */}
        <div
          className={`flex items-center gap-2 transition-all duration-200 ease-out
            bg-[#1e1e2e]/90 backdrop-blur-sm border border-[#313244]/60
            ${isPill
              ? "rounded-2xl px-3 py-1.5 min-w-[200px]"
              : "rounded-full w-12 h-12 justify-center"
            }
            ${state === "idle" ? "opacity-60 hover:opacity-95" : "opacity-100"}
          `}
          style={{ boxShadow: "0 4px 24px rgba(0,0,0,0.3)" }}
        >
          {/* Ghost icon */}
          <img
            src="/ghostspell-ghost.png"
            alt=""
            className={`shrink-0 transition-all duration-200
              ${isPill ? "w-7 h-7" : "w-8 h-8"}
              ${state === "processing" ? "animate-bounce-slow" : ""}
              ${state === "idle" ? "animate-breathe" : ""}
            `}
          />

          {/* Pill content — only visible when expanded */}
          {isPill && (
            <div className="flex items-center gap-2 overflow-hidden animate-fade-in">
              {icon && <span className="text-sm shrink-0">{icon}</span>}
              <span className="text-xs text-[#cdd6f4] font-medium truncate max-w-[120px]">
                {name}
              </span>
              {state === "processing" && (
                <>
                  <span className="w-px h-3 bg-[#45475a] shrink-0" />
                  <span className="text-[11px] text-[#6c7086] tabular-nums shrink-0">
                    {elapsed}s
                  </span>
                  {model && (
                    <span className="text-[10px] text-[#585b70] truncate max-w-[80px] shrink-0">
                      {model}
                    </span>
                  )}
                </>
              )}
            </div>
          )}
        </div>
      </div>

      {/* Context menu */}
      {menuOpen && (
        <div
          className="absolute top-14 left-1 bg-[#181825]/95 backdrop-blur-md
                     border border-[#313244]/60 rounded-xl overflow-hidden
                     animate-fade-in min-w-[180px]"
          style={{ boxShadow: "0 8px 32px rgba(0,0,0,0.4)" }}
        >
          {menuItems.map((item, idx) => (
            <button
              key={idx}
              onClick={() => selectPrompt(idx)}
              className={`w-full text-left px-3 py-2 text-xs flex items-center gap-2
                transition-colors hover:bg-[#313244]/50
                ${item.active ? "text-[#89b4fa]" : "text-[#a6adc8]"}
              `}
            >
              <span className="w-5 text-center shrink-0">{item.icon || "📝"}</span>
              <span className="truncate">{item.name}</span>
              {item.active && <span className="ml-auto text-[10px] text-[#89b4fa]">●</span>}
            </button>
          ))}
          <div className="h-px bg-[#313244]/50" />
          <button
            onClick={() => { setMenuOpen(false); goCall("openSettingsFromIndicator"); }}
            className="w-full text-left px-3 py-2 text-xs text-[#6c7086] hover:text-[#a6adc8]
                       hover:bg-[#313244]/50 transition-colors"
          >
            ⚙️ Settings
          </button>
          <button
            onClick={() => { setMenuOpen(false); goCall("quitFromIndicator"); }}
            className="w-full text-left px-3 py-2 text-xs text-[#6c7086] hover:text-[#f38ba8]
                       hover:bg-[#313244]/50 transition-colors"
          >
            ✕ Quit
          </button>
        </div>
      )}

      {/* Click-outside to close menu */}
      {menuOpen && (
        <div className="fixed inset-0 z-[-1]" onClick={() => setMenuOpen(false)} />
      )}

      {/* Inline styles for animations */}
      <style>{`
        @keyframes breathe {
          0%, 100% { transform: scale(1); }
          50% { transform: scale(1.05); }
        }
        @keyframes bounce-slow {
          0%, 100% { transform: translateY(0); }
          50% { transform: translateY(-3px); }
        }
        @keyframes fade-in {
          from { opacity: 0; transform: translateX(-8px); }
          to { opacity: 1; transform: translateX(0); }
        }
        .animate-breathe { animation: breathe 3s ease-in-out infinite; }
        .animate-bounce-slow { animation: bounce-slow 1.5s ease-in-out infinite; }
        .animate-fade-in { animation: fade-in 200ms ease-out; }
      `}</style>
    </div>
  );
}
