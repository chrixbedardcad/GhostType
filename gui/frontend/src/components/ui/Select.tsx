/**
 * Select — custom dropdown replacing native <select>.
 * Zen: smooth open/close, subtle borders, keyboard accessible.
 */

import { useState, useRef, useEffect, useCallback, type KeyboardEvent } from "react";

interface Option {
  value: string;
  label: string;
}

interface Props {
  value: string;
  onChange: (value: string) => void;
  options: Option[];
  placeholder?: string;
  className?: string;
}

export function Select({ value, onChange, options, placeholder, className }: Props) {
  const [open, setOpen] = useState(false);
  const [focusIdx, setFocusIdx] = useState(-1);
  const containerRef = useRef<HTMLDivElement>(null);
  const listRef = useRef<HTMLUListElement>(null);

  const selected = options.find((o) => o.value === value);

  // Close on outside click
  useEffect(() => {
    function handler(e: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, []);

  // Scroll focused option into view
  useEffect(() => {
    if (open && focusIdx >= 0 && listRef.current) {
      const item = listRef.current.children[focusIdx] as HTMLElement | undefined;
      item?.scrollIntoView({ block: "nearest" });
    }
  }, [focusIdx, open]);

  const toggle = useCallback(() => {
    setOpen((prev) => {
      if (!prev) {
        const idx = options.findIndex((o) => o.value === value);
        setFocusIdx(idx >= 0 ? idx : 0);
      }
      return !prev;
    });
  }, [options, value]);

  const pick = useCallback(
    (v: string) => {
      onChange(v);
      setOpen(false);
    },
    [onChange],
  );

  const handleKey = useCallback(
    (e: KeyboardEvent) => {
      if (!open) {
        if (e.key === "ArrowDown" || e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          toggle();
        }
        return;
      }

      switch (e.key) {
        case "ArrowDown":
          e.preventDefault();
          setFocusIdx((i) => Math.min(i + 1, options.length - 1));
          break;
        case "ArrowUp":
          e.preventDefault();
          setFocusIdx((i) => Math.max(i - 1, 0));
          break;
        case "Enter":
        case " ":
          e.preventDefault();
          if (focusIdx >= 0 && focusIdx < options.length) {
            pick(options[focusIdx].value);
          }
          break;
        case "Escape":
          e.preventDefault();
          setOpen(false);
          break;
      }
    },
    [open, focusIdx, options, toggle, pick],
  );

  return (
    <div ref={containerRef} className={`relative ${className ?? ""}`} onKeyDown={handleKey}>
      {/* Trigger */}
      <button
        type="button"
        onClick={toggle}
        aria-haspopup="listbox"
        aria-expanded={open}
        className={[
          "w-full flex items-center justify-between",
          "px-3 py-1.5 text-sm rounded-md",
          "bg-crust border border-surface-0 text-text",
          "hover:border-surface-1 transition-colors duration-150",
          "cursor-pointer",
          open && "border-accent-blue/50",
        ]
          .filter(Boolean)
          .join(" ")}
      >
        <span className={selected ? "text-text" : "text-overlay-0"}>
          {selected?.label ?? placeholder ?? "Select..."}
        </span>
        <svg
          className={`w-3.5 h-3.5 text-overlay-0 transition-transform duration-150 ${open ? "rotate-180" : ""}`}
          viewBox="0 0 20 20"
          fill="currentColor"
        >
          <path
            fillRule="evenodd"
            d="M5.23 7.21a.75.75 0 011.06.02L10 11.168l3.71-3.938a.75.75 0 111.08 1.04l-4.25 4.5a.75.75 0 01-1.08 0l-4.25-4.5a.75.75 0 01.02-1.06z"
            clipRule="evenodd"
          />
        </svg>
      </button>

      {/* Dropdown */}
      <ul
        ref={listRef}
        role="listbox"
        className={[
          "absolute z-50 left-0 right-0 mt-1",
          "bg-crust border border-surface-0 rounded-md",
          "max-h-48 overflow-y-auto py-1",
          "transition-all duration-150 origin-top",
          open
            ? "opacity-100 scale-y-100 pointer-events-auto"
            : "opacity-0 scale-y-95 pointer-events-none",
        ].join(" ")}
      >
        {options.map((opt, i) => (
          <li
            key={opt.value}
            role="option"
            aria-selected={opt.value === value}
            onMouseDown={(e) => e.preventDefault()}
            onClick={() => pick(opt.value)}
            onMouseEnter={() => setFocusIdx(i)}
            className={[
              "px-3 py-1.5 text-sm cursor-pointer transition-colors duration-100",
              opt.value === value
                ? "text-accent-blue bg-accent-blue/10"
                : i === focusIdx
                  ? "text-text bg-surface-0/60"
                  : "text-subtext-0 hover:text-text",
            ].join(" ")}
          >
            {opt.label}
          </li>
        ))}
      </ul>
    </div>
  );
}
