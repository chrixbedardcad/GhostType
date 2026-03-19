/**
 * Input — styled text input.
 * Zen: dark recessed field, soft blue focus glow, no shadows.
 */

import { type InputHTMLAttributes } from "react";

const SIZES = {
  sm: "px-2.5 py-1 text-xs",
  md: "px-3 py-1.5 text-sm",
} as const;

interface Props extends InputHTMLAttributes<HTMLInputElement> {
  inputSize?: keyof typeof SIZES;
}

export function Input({ inputSize = "md", className, ...rest }: Props) {
  return (
    <input
      className={[
        "w-full rounded-md",
        "bg-crust text-text placeholder:text-overlay-0",
        "border border-surface-0",
        "focus:border-accent-blue/50 focus:outline-none",
        "transition-colors duration-150",
        SIZES[inputSize],
        className,
      ]
        .filter(Boolean)
        .join(" ")}
      {...rest}
    />
  );
}
