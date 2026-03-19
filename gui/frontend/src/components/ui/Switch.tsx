/**
 * Switch — toggle switch with smooth sliding thumb.
 * Zen: soft color transition, no harsh edges.
 */

interface Props {
  checked: boolean;
  onChange: (checked: boolean) => void;
  disabled?: boolean;
  label?: string;
}

export function Switch({ checked, onChange, disabled, label }: Props) {
  return (
    <label
      className={[
        "inline-flex items-center gap-2 cursor-pointer select-none",
        disabled && "opacity-40 pointer-events-none",
      ]
        .filter(Boolean)
        .join(" ")}
    >
      <button
        role="switch"
        type="button"
        aria-checked={checked}
        onClick={() => onChange(!checked)}
        disabled={disabled}
        className={[
          "relative inline-flex items-center shrink-0",
          "w-9 h-5 rounded-full",
          "transition-colors duration-150 ease-out",
          "cursor-pointer",
          checked ? "bg-accent-blue/60" : "bg-surface-1",
        ].join(" ")}
      >
        <span
          className={[
            "absolute top-0.5 w-4 h-4 rounded-full",
            "transition-all duration-150 ease-out",
            checked
              ? "left-[calc(100%-1.125rem)] bg-accent-blue"
              : "left-0.5 bg-overlay-0",
          ].join(" ")}
        />
      </button>
      {label && <span className="text-sm text-subtext-0">{label}</span>}
    </label>
  );
}
