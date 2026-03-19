/**
 * Indicator window — placeholder for Phase 3 migration.
 * Will be the fixed-size transparent overlay with CSS transitions.
 */
export function IndicatorWindow() {
  return (
    <div className="h-full flex items-center justify-center">
      <div className="w-12 h-12 rounded-full bg-surface-0/50 flex items-center justify-center">
        <img src="/ghostspell-ghost.png" alt="" className="w-8 h-8 opacity-50" />
      </div>
    </div>
  );
}
