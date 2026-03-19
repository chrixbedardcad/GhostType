/**
 * Badge — minimal pill-shaped label.
 * Zen: soft colors, no hard borders, slight roundness.
 */

const VARIANTS = {
  recommended: "bg-accent-blue/20 text-accent-blue",
  best: "bg-accent-yellow/20 text-accent-yellow",
  fast: "bg-accent-pink/20 text-accent-pink",
  cheap: "bg-accent-green/20 text-accent-green",
  free: "bg-accent-green/20 text-accent-green",
  heavy: "bg-accent-lavender/20 text-accent-lavender",
  deprecated: "bg-surface-1 text-overlay-0",
  uncensored: "bg-accent-peach/20 text-accent-peach",
  vision: "bg-accent-sky/20 text-accent-sky",
  active: "bg-accent-green/20 text-accent-green",
  downloaded: "bg-accent-green/20 text-accent-green",
  available: "bg-accent-blue/20 text-accent-blue",
  default: "bg-surface-1 text-subtext-0",
} as const;

const LABELS: Record<string, string> = {
  recommended: "RECOMMENDED",
  best: "BEST",
  fast: "FAST",
  cheap: "CHEAP",
  free: "FREE",
  heavy: "HEAVY",
  deprecated: "DEPRECATED",
  uncensored: "UNCENSORED",
  vision: "👁 VISION",
  active: "ACTIVE",
  downloaded: "DOWNLOADED",
  available: "AVAILABLE",
};

interface Props {
  variant: keyof typeof VARIANTS;
  label?: string;
  className?: string;
}

export function Badge({ variant, label, className }: Props) {
  const colors = VARIANTS[variant] ?? VARIANTS.default;
  const text = label ?? LABELS[variant] ?? variant.toUpperCase();
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-[0.65rem] font-semibold tracking-wide ${colors} ${className ?? ""}`}>
      {text}
    </span>
  );
}
