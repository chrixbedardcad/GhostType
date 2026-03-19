/**
 * Textarea — styled multiline text input.
 * Zen: matches Input styling, vertical resize only.
 */

import { type TextareaHTMLAttributes } from "react";

type Props = TextareaHTMLAttributes<HTMLTextAreaElement>;

export function Textarea({ className, ...rest }: Props) {
  return (
    <textarea
      className={[
        "w-full rounded-md resize-y min-h-[80px]",
        "bg-crust text-text placeholder:text-overlay-0",
        "border border-surface-0",
        "focus:border-accent-blue/50 focus:outline-none",
        "transition-colors duration-150",
        "px-3 py-1.5 text-sm leading-relaxed",
        className,
      ]
        .filter(Boolean)
        .join(" ")}
      {...rest}
    />
  );
}
