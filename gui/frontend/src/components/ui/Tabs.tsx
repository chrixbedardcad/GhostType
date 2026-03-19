/**
 * Tabs — horizontal tab bar with active indicator.
 * Zen: minimal underline, soft color transitions, generous spacing.
 */

import { createContext, useContext, type ReactNode } from "react";

/* ── Context ── */

interface TabsContext {
  activeTab: string;
  onChange: (value: string) => void;
}

const ctx = createContext<TabsContext>({ activeTab: "", onChange: () => {} });

/* ── Tabs (root) ── */

interface TabsProps {
  activeTab: string;
  onChange: (value: string) => void;
  children: ReactNode;
  className?: string;
}

export function Tabs({ activeTab, onChange, children, className }: TabsProps) {
  return (
    <ctx.Provider value={{ activeTab, onChange }}>
      <div className={className}>{children}</div>
    </ctx.Provider>
  );
}

/* ── TabList (bar) ── */

interface TabListProps {
  children: ReactNode;
  className?: string;
}

export function TabList({ children, className }: TabListProps) {
  return (
    <div
      role="tablist"
      className={`flex gap-1 border-b border-surface-0 ${className ?? ""}`}
    >
      {children}
    </div>
  );
}

/* ── Tab (button) ── */

interface TabProps {
  value: string;
  label: string;
  className?: string;
}

export function Tab({ value, label, className }: TabProps) {
  const { activeTab, onChange } = useContext(ctx);
  const active = activeTab === value;

  return (
    <button
      role="tab"
      type="button"
      aria-selected={active}
      onClick={() => onChange(value)}
      className={[
        "relative px-3 py-2 text-sm font-medium",
        "transition-colors duration-150 cursor-pointer",
        "-mb-px", // overlap parent border
        active
          ? "text-accent-blue"
          : "text-overlay-0 hover:text-subtext-0",
        className,
      ]
        .filter(Boolean)
        .join(" ")}
    >
      {label}
      {/* Active indicator line */}
      <span
        className={[
          "absolute bottom-0 left-0 right-0 h-px",
          "transition-all duration-150",
          active ? "bg-accent-blue" : "bg-transparent",
        ].join(" ")}
      />
    </button>
  );
}

/* ── TabPanel (content) ── */

interface TabPanelProps {
  value: string;
  children: ReactNode;
  className?: string;
}

export function TabPanel({ value, children, className }: TabPanelProps) {
  const { activeTab } = useContext(ctx);
  if (activeTab !== value) return null;

  return (
    <div role="tabpanel" className={className}>
      {children}
    </div>
  );
}
