import { useState, useEffect, useRef } from "react";
import { goCall } from "@/bridge";
import { usePlatform } from "@/hooks/usePlatform";

/**
 * Custom dropdown — zen, minimal, smooth animation.
 * Replaces native <select> that looks like 1995 on macOS.
 */
function Dropdown({
  value,
  options,
  onChange,
  placeholder = "Select...",
}: {
  value: string;
  options: { value: string; label: string }[];
  onChange: (value: string) => void;
  placeholder?: string;
}) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function onClickOutside(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener("mousedown", onClickOutside);
    return () => document.removeEventListener("mousedown", onClickOutside);
  }, []);

  const selected = options.find((o) => o.value === value);

  return (
    <div ref={ref} className="relative">
      <button
        onClick={() => setOpen(!open)}
        className="w-full flex items-center justify-between gap-2 px-3 py-2
                   bg-crust border border-surface-0 rounded-lg text-sm text-text
                   hover:border-surface-1 focus:border-accent-blue/50 focus:outline-none
                   transition-colors"
      >
        <span className={selected ? "text-text" : "text-overlay-0"}>
          {selected?.label ?? placeholder}
        </span>
        <svg width="12" height="12" viewBox="0 0 12 12" className={`text-overlay-0 transition-transform ${open ? "rotate-180" : ""}`}>
          <path d="M3 4.5L6 7.5L9 4.5" stroke="currentColor" strokeWidth="1.5" fill="none" strokeLinecap="round"/>
        </svg>
      </button>

      {open && (
        <div className="absolute z-50 mt-1 w-full bg-mantle border border-surface-0 rounded-lg
                        shadow-none overflow-hidden animate-in fade-in slide-in-from-top-1">
          {options.map((opt) => (
            <button
              key={opt.value}
              onClick={() => { onChange(opt.value); setOpen(false); }}
              className={`w-full text-left px-3 py-2 text-sm transition-colors
                ${opt.value === value
                  ? "text-accent-blue bg-accent-blue/10"
                  : "text-subtext-0 hover:text-text hover:bg-surface-0/50"
                }`}
            >
              {opt.label}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

/**
 * Toggle row — label + description + switch.
 */
function ToggleRow({
  label,
  description,
  checked,
  onChange,
}: {
  label: string;
  description?: string;
  checked: boolean;
  onChange: (v: boolean) => void;
}) {
  return (
    <div className="flex items-center justify-between py-2">
      <div>
        <p className="text-sm text-text">{label}</p>
        {description && <p className="text-xs text-overlay-0 mt-0.5">{description}</p>}
      </div>
      <button
        onClick={() => onChange(!checked)}
        className={`relative w-9 h-5 rounded-full transition-colors shrink-0 ${
          checked ? "bg-accent-blue" : "bg-surface-1"
        }`}
      >
        <span
          className={`absolute top-0.5 w-4 h-4 rounded-full bg-text transition-transform ${
            checked ? "translate-x-4" : "translate-x-0.5"
          }`}
        />
      </button>
    </div>
  );
}

/**
 * General tab — app preferences.
 * Zen: clean sections, custom dropdowns, toggle switches.
 */
export function GeneralTab() {
  const platform = usePlatform();
  const [sound, setSound] = useState(true);
  const [clipboard, setClipboard] = useState(false);
  const [maxChars, setMaxChars] = useState("2000");
  const [indicatorPos, setIndicatorPos] = useState("top-right");
  const [indicatorMode, setIndicatorMode] = useState("processing");
  const [hotkey, setHotkey] = useState("Ctrl+G");

  useEffect(() => {
    goCall("getConfig").then((raw) => {
      if (!raw) return;
      try {
        const cfg = JSON.parse(raw);
        setSound(cfg.sound_enabled ?? true);
        setClipboard(cfg.preserve_clipboard ?? false);
        setMaxChars(String(cfg.max_input_chars || 2000));
        setIndicatorPos(cfg.indicator_position || "top-right");
        setIndicatorMode(cfg.indicator_mode || "processing");
        const hk = cfg.hotkeys?.action || "Ctrl+G";
        setHotkey(platform === "darwin" ? hk.replace("Ctrl", "⌘") : hk);
      } catch { /* ignore */ }
    });
  }, [platform]);

  async function saveField(method: string, ...args: unknown[]) {
    await goCall(method, ...args);
  }

  return (
    <div className="space-y-8">
      {/* Hotkey display */}
      <section>
        <h2 className="text-sm font-medium text-subtext-1 mb-4 tracking-wide uppercase">
          Activation
        </h2>
        <div className="bg-surface-0/30 rounded-xl p-5 flex items-center gap-4">
          <div className="px-3 py-1.5 rounded-lg bg-crust border border-surface-0 text-sm font-mono text-accent-blue">
            {hotkey}
          </div>
          <p className="text-xs text-overlay-0">Select text and press to activate</p>
        </div>
      </section>

      {/* Sound & Clipboard */}
      <section>
        <h2 className="text-sm font-medium text-subtext-1 mb-4 tracking-wide uppercase">
          Behavior
        </h2>
        <div className="bg-surface-0/30 rounded-xl p-5 space-y-1">
          <ToggleRow
            label="Sound Effects"
            description="Play sounds during processing"
            checked={sound}
            onChange={(v) => { setSound(v); saveField("setSoundEnabled", v); }}
          />
          <div className="h-px bg-surface-0/50 my-2" />
          <ToggleRow
            label="Preserve Clipboard"
            description="Restore clipboard contents after paste"
            checked={clipboard}
            onChange={(v) => { setClipboard(v); saveField("setPreserveClipboard", v); }}
          />
        </div>
      </section>

      {/* Input limit */}
      <section>
        <h2 className="text-sm font-medium text-subtext-1 mb-4 tracking-wide uppercase">
          Input
        </h2>
        <div className="bg-surface-0/30 rounded-xl p-5">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm text-text">Max Input Characters</p>
              <p className="text-xs text-overlay-0 mt-0.5">Limit text length sent to AI</p>
            </div>
            <div className="w-36">
              <Dropdown
                value={maxChars}
                onChange={(v) => { setMaxChars(v); saveField("setMaxInputChars", parseInt(v)); }}
                options={[
                  { value: "500", label: "500 chars" },
                  { value: "1000", label: "1,000 chars" },
                  { value: "2000", label: "2,000 chars" },
                  { value: "5000", label: "5,000 chars" },
                  { value: "10000", label: "10,000 chars" },
                  { value: "0", label: "No limit" },
                ]}
              />
            </div>
          </div>
        </div>
      </section>

      {/* Ghost Indicator */}
      <section>
        <h2 className="text-sm font-medium text-subtext-1 mb-4 tracking-wide uppercase">
          Ghost Indicator
        </h2>
        <div className="bg-surface-0/30 rounded-xl p-5 space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm text-text">Position</p>
              <p className="text-xs text-overlay-0 mt-0.5">Where the ghost appears on screen</p>
            </div>
            <div className="w-40">
              <Dropdown
                value={indicatorPos}
                onChange={(v) => {
                  setIndicatorPos(v);
                  saveField("setIndicatorPosition", v);
                  goCall("previewIndicatorPosition");
                }}
                options={[
                  { value: "top-right", label: "Top Right" },
                  { value: "top-left", label: "Top Left" },
                  { value: "bottom-right", label: "Bottom Right" },
                  { value: "bottom-left", label: "Bottom Left" },
                  { value: "center", label: "Center" },
                ]}
              />
            </div>
          </div>

          <div className="h-px bg-surface-0/50" />

          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm text-text">Visibility</p>
              <p className="text-xs text-overlay-0 mt-0.5">When to show the ghost</p>
            </div>
            <div className="w-40">
              <Dropdown
                value={indicatorMode}
                onChange={(v) => { setIndicatorMode(v); saveField("setIndicatorMode", v); }}
                options={[
                  { value: "processing", label: "While Processing" },
                  { value: "always", label: "Always Visible" },
                  { value: "hidden", label: "Hidden" },
                ]}
              />
            </div>
          </div>
        </div>
      </section>
    </div>
  );
}
