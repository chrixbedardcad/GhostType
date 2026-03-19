/**
 * Separator — horizontal divider, optionally with centered label.
 * Zen: barely-there line, quiet label.
 */

interface Props {
  label?: string;
  className?: string;
}

export function Separator({ label, className }: Props) {
  if (!label) {
    return <div className={`h-px bg-surface-0 ${className ?? ""}`} />;
  }

  return (
    <div className={`flex items-center gap-3 ${className ?? ""}`}>
      <div className="flex-1 h-px bg-surface-0" />
      <span className="text-xs text-overlay-0 shrink-0">{label}</span>
      <div className="flex-1 h-px bg-surface-0" />
    </div>
  );
}
