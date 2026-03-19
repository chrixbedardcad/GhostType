/**
 * Card — simple container surface.
 * Zen: semi-transparent, subtle border, flat (no shadow).
 */

import { type ReactNode } from "react";

interface Props {
  children: ReactNode;
  className?: string;
  hover?: boolean;
}

export function Card({ children, className, hover }: Props) {
  return (
    <div
      className={[
        "bg-surface-0/50 rounded-xl p-4",
        "border border-surface-0",
        "transition-colors duration-150",
        hover && "hover:border-accent-blue/40",
        className,
      ]
        .filter(Boolean)
        .join(" ")}
    >
      {children}
    </div>
  );
}
