/**
 * Button — versatile button with variant and size support.
 * Zen: flat, muted accents, soft hover transitions, no shadows.
 */

import { type ButtonHTMLAttributes } from "react";

const VARIANTS = {
  primary:
    "bg-accent-blue/20 text-accent-blue hover:bg-accent-blue/30 active:bg-accent-blue/35",
  secondary:
    "bg-surface-0 text-subtext-0 hover:bg-surface-1 active:bg-surface-2",
  danger:
    "bg-accent-red/20 text-accent-red hover:bg-accent-red/30 active:bg-accent-red/35",
  ghost:
    "bg-transparent text-subtext-0 hover:bg-surface-0 active:bg-surface-1",
} as const;

const SIZES = {
  sm: "px-3 py-1 text-xs rounded-md",
  md: "px-4 py-1.5 text-sm rounded-md",
  lg: "px-5 py-2 text-sm rounded-lg",
} as const;

interface Props extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: keyof typeof VARIANTS;
  size?: keyof typeof SIZES;
}

export function Button({
  variant = "primary",
  size = "md",
  className,
  disabled,
  children,
  ...rest
}: Props) {
  return (
    <button
      className={[
        "inline-flex items-center justify-center font-medium",
        "transition-all duration-150 ease-out",
        "cursor-pointer",
        VARIANTS[variant],
        SIZES[size],
        disabled && "opacity-40 pointer-events-none",
        className,
      ]
        .filter(Boolean)
        .join(" ")}
      disabled={disabled}
      {...rest}
    >
      {children}
    </button>
  );
}
