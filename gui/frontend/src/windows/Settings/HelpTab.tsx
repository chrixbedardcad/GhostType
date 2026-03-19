import { usePlatform } from "@/hooks/usePlatform";

/**
 * Help tab — how to use GhostSpell.
 * Zen: clean numbered steps, calm typography, lots of breathing room.
 */
export function HelpTab() {
  const platform = usePlatform();
  const hotkey = platform === "darwin" ? "⌘G" : "Ctrl+G";
  const undo = platform === "darwin" ? "⌘Z" : "Ctrl+Z";

  const steps = [
    { num: 1, text: "Select text in any application, or place your cursor in a text field." },
    { num: 2, text: `Press ${hotkey} to activate GhostSpell.` },
    { num: 3, text: "GhostSpell captures the text, sends it to your AI model, and replaces it with the result." },
    { num: 4, text: `Press ${undo} to undo and restore the original text.` },
  ];

  const tips = [
    `Press ${hotkey} twice to cancel an active request.`,
    "Click the ghost indicator to cycle between prompts.",
    "Right-click the ghost indicator for a prompt menu.",
    "Double-click the ghost indicator to open settings.",
    "Vision prompts (📸) capture a screenshot instead of text.",
    "Each prompt can use a different AI model — set it in the Prompts tab.",
  ];

  return (
    <div className="space-y-8">
      <section>
        <h2 className="text-sm font-medium text-subtext-1 mb-4 tracking-wide uppercase">
          How It Works
        </h2>
        <div className="space-y-3">
          {steps.map((s) => (
            <div key={s.num} className="flex gap-4 items-start">
              <span className="shrink-0 w-7 h-7 rounded-full bg-accent-blue/15 text-accent-blue
                             text-xs font-semibold flex items-center justify-center mt-0.5">
                {s.num}
              </span>
              <p className="text-[13px] text-subtext-0 leading-relaxed pt-1">{s.text}</p>
            </div>
          ))}
        </div>
      </section>

      <section>
        <h2 className="text-sm font-medium text-subtext-1 mb-4 tracking-wide uppercase">
          Tips
        </h2>
        <ul className="space-y-2">
          {tips.map((tip, i) => (
            <li key={i} className="flex gap-3 items-start">
              <span className="shrink-0 text-overlay-0 text-[11px] mt-1">•</span>
              <p className="text-[13px] text-overlay-1 leading-relaxed">{tip}</p>
            </li>
          ))}
        </ul>
      </section>
    </div>
  );
}
