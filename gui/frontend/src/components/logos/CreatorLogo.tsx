/**
 * Creator logos — who MADE the model (the AI company).
 * Single source of truth. 36px default for clear visibility.
 * Used in settings, wizard, indicator.
 */

const LOGOS: Record<string, (size: number) => React.ReactNode> = {
  alibaba: (s) => (
    <svg width={s} height={s} viewBox="0 0 24 24">
      <rect width="24" height="24" rx="6" fill="#6F69F7"/>
      <text x="12" y="10" textAnchor="middle" fontSize="5" fontWeight="700" fill="#fff" fontFamily="Arial,sans-serif">QWEN</text>
      <text x="12" y="17" textAnchor="middle" fontSize="4" fill="rgba(255,255,255,0.7)" fontFamily="Arial,sans-serif">Alibaba</text>
    </svg>
  ),
  nvidia: (s) => (
    <svg width={s} height={s} viewBox="0 0 24 24">
      <rect width="24" height="24" rx="6" fill="#76B900"/>
      <path d="M9.8 8.2v-.6c0-.1 0-.1.1-.1 1.7-.2 3.1.4 4.2 1.7.1.1.1.2 0 .3l-.8.9c-.1.1-.2.1-.3 0-.8-.8-1.7-1.2-2.8-1.1-.1 0-.2 0-.2-.1V8.2h-.2zm0 2v-1c0-.1 0-.1.1-.1 1.1-.1 1.9.3 2.5 1.1.1.1.1.2-.1.3l-.7.5c-.1.1-.2.1-.3 0-.4-.4-.8-.6-1.4-.6-.1 0-.2 0-.2-.1V10.2zm0 1.3v-.7s0-.1.1-.1c.5 0 .9.2 1.1.6.1.1.1.2 0 .3l-.5.4c-.1.1-.2 0-.2 0-.2-.2-.3-.3-.5-.4V11.5zm-1.5 4.4V9.5c-1.8.3-3.1 1.7-3.3 3.4 0 2 1.4 3.5 3.3 3.6v-1c-1.3-.1-2.3-1.2-2.2-2.5.1-1.1.9-2 2-2.2v4.8l1.7-1.7c.7.8 1.7 1.3 2.8 1.3 1.1 0 2.1-.5 2.8-1.3l.5.5c-.9 1.1-2.3 1.8-3.8 1.8-1.3 0-2.5-.6-3.4-1.5l-.4.5zm9-2.5c-.2-2.5-2.2-4.4-4.7-4.6v1c2 .2 3.5 1.7 3.7 3.7 0 1.2-.4 2.2-1.2 3l.5.5c1-.9 1.6-2.2 1.7-3.6z" fill="#fff"/>
    </svg>
  ),
  microsoft: (s) => (
    <svg width={s} height={s} viewBox="0 0 24 24">
      <rect x="1" y="1" width="10.5" height="10.5" fill="#F25022"/>
      <rect x="12.5" y="1" width="10.5" height="10.5" fill="#7FBA00"/>
      <rect x="1" y="12.5" width="10.5" height="10.5" fill="#00A4EF"/>
      <rect x="12.5" y="12.5" width="10.5" height="10.5" fill="#FFB900"/>
    </svg>
  ),
  google: (s) => (
    <svg width={s} height={s} viewBox="0 0 24 24">
      <defs><linearGradient id="gc" x1="0" y1="0" x2="24" y2="24" gradientUnits="userSpaceOnUse"><stop stopColor="#4285F4"/><stop offset=".33" stopColor="#9B72CB"/><stop offset=".66" stopColor="#D96570"/><stop offset="1" stopColor="#F9AB00"/></linearGradient></defs>
      <path d="M11.04 19.32Q12 21.51 12 24q0-2.49.93-4.68.96-2.19 2.58-3.81t3.81-2.55Q21.51 12 24 12q-2.49 0-4.68-.93a12.3 12.3 0 0 1-3.81-2.58 12.3 12.3 0 0 1-2.58-3.81Q12 2.49 12 0q0 2.49-.96 4.68-.93 2.19-2.55 3.81a12.3 12.3 0 0 1-3.81 2.58Q2.49 12 0 12q2.49 0 4.68.96 2.19.93 3.81 2.55t2.55 3.81" fill="url(#gc)"/>
    </svg>
  ),
  meta: (s) => (
    <svg width={s} height={s} viewBox="0 0 24 24">
      <rect width="24" height="24" rx="6" fill="#0668E1"/>
      <path d="M5.8 15.8c0 .9.2 1.5.5 1.9.4.4.9.6 1.4.6.7 0 1.3-.3 1.9-1 .7-.9 1.3-2.1 1.8-3.6l.3-.8c-1-1.7-1.7-2.9-2-3.4-.6-.9-1.2-1.3-1.8-1.3-.5 0-.9.3-1.3.8-.4.6-.6 1.4-.6 2.4l-.2 4.4zm12.4 0c0 .9-.2 1.5-.5 1.9-.4.4-.9.6-1.4.6-.7 0-1.3-.3-1.9-1-.5-.6-.9-1.3-1.4-2.3l-.5-1c-.7-1.3-1.3-2.3-1.9-3-.9-1-1.7-1.5-2.7-1.5-1 0-1.8.5-2.4 1.4-.6.9-.9 2.1-.9 3.5v1.4c0 .9.2 1.5.5 1.9s.7.6 1.1.6h.2v.9H4v-.9c.5 0 .8-.2 1.1-.6s.5-1 .5-1.9v-1.4c0-1.8.4-3.2 1.1-4.3.8-1.1 1.8-1.7 3.1-1.7 1.1 0 2.1.5 3 1.6.6.7 1.2 1.7 1.9 3l.5 1c.4.9.8 1.6 1.2 2.1.5.6 1.1.9 1.7.9.5 0 .9-.2 1.2-.6s.5-1 .5-1.8v-4.6c0-.9-.2-1.5-.5-1.9s-.7-.6-1.1-.6h-.2v-.9H20v.9c-.5 0-.8.2-1.1.6s-.5 1-.5 1.9l-.2 4.6z" fill="#fff"/>
    </svg>
  ),
  mistral: (s) => (
    <svg width={s} height={s} viewBox="0 0 24 24">
      <rect x="1" y="1" width="5" height="5" fill="#F7D046"/>
      <rect x="9.5" y="1" width="5" height="5" fill="#F7D046"/>
      <rect x="18" y="1" width="5" height="5" fill="#000"/>
      <rect x="1" y="6.5" width="5" height="5" fill="#F2A73B"/>
      <rect x="5.5" y="6.5" width="4.5" height="5" fill="#F2A73B"/>
      <rect x="9.5" y="6.5" width="5" height="5" fill="#F2A73B"/>
      <rect x="14" y="6.5" width="4.5" height="5" fill="#F2A73B"/>
      <rect x="18" y="6.5" width="5" height="5" fill="#000"/>
      <rect x="1" y="12" width="5" height="5" fill="#EE792F"/>
      <rect x="9.5" y="12" width="5" height="5" fill="#EE792F"/>
      <rect x="18" y="12" width="5" height="5" fill="#000"/>
      <rect x="1" y="18" width="5" height="5" fill="#EB5829"/>
      <rect x="5.5" y="18" width="4.5" height="5" fill="#EB5829"/>
      <rect x="9.5" y="18" width="5" height="5" fill="#EB5829"/>
      <rect x="14" y="18" width="4.5" height="5" fill="#EB5829"/>
      <rect x="18" y="18" width="5" height="5" fill="#000"/>
    </svg>
  ),
  deepseek: (s) => (
    <svg width={s} height={s} viewBox="0 0 24 24">
      <rect width="24" height="24" rx="6" fill="#4D6BFE"/>
      <text x="12" y="9" textAnchor="middle" fontSize="5.5" fontWeight="700" fill="#fff" fontFamily="Arial,sans-serif">DEEP</text>
      <text x="12" y="16" textAnchor="middle" fontSize="5.5" fontWeight="700" fill="#fff" fontFamily="Arial,sans-serif">SEEK</text>
    </svg>
  ),
  openai: (s) => (
    <svg width={s} height={s} viewBox="0 0 24 24">
      <path d="M22.2819 9.8211a5.9847 5.9847 0 0 0-.5157-4.9108 6.0462 6.0462 0 0 0-6.5098-2.9A6.0651 6.0651 0 0 0 4.9807 4.1818a5.9847 5.9847 0 0 0-3.9977 2.9 6.0462 6.0462 0 0 0 .7427 7.0966 5.98 5.98 0 0 0 .511 4.9107 6.051 6.051 0 0 0 6.5146 2.9001A5.9847 5.9847 0 0 0 13.2599 24a6.0557 6.0557 0 0 0 5.7718-4.2058 5.9894 5.9894 0 0 0 3.9977-2.9001 6.0557 6.0557 0 0 0-.7475-7.0729z" fill="#10A37F"/>
    </svg>
  ),
  anthropic: (s) => (
    <svg width={s} height={s} viewBox="0 0 24 24">
      <path d="M17.3041 3.541h-3.6718l6.696 16.918H24Zm-10.6082 0L0 20.459h3.7442l1.3693-3.5527h7.0052l1.3693 3.5528h3.7442L10.5363 3.5409Zm-.3712 10.2232 2.2914-5.9456 2.2914 5.9456Z" fill="#D4A574"/>
    </svg>
  ),
  xai: (s) => (
    <svg width={s} height={s} viewBox="0 0 24 24">
      <path d="M14.234 10.162 22.977 0h-2.072l-7.591 8.824L7.251 0H.258l9.168 13.343L.258 24H2.33l8.016-9.318L16.749 24h6.993zm-2.837 3.299-.929-1.329L3.076 1.56h3.182l5.965 8.532.929 1.329 7.754 11.09h-3.182z" fill="#cdd6f4"/>
    </svg>
  ),
};

/** Map model name prefix → creator identifier */
const CREATOR_MAP: Record<string, string> = {
  qwen: "alibaba",
  nemotron: "nvidia",
  phi: "microsoft",
  gemma: "google",
  llama: "meta",
  mistral: "mistral",
  deepseek: "deepseek",
};

/** Resolve creator from a model name (e.g. "qwen3.5-2b" → "alibaba") */
export function resolveCreator(modelName: string): string {
  const lower = modelName.toLowerCase();
  for (const [prefix, creator] of Object.entries(CREATOR_MAP)) {
    if (lower.includes(prefix)) return creator;
  }
  return "";
}

interface Props {
  creator: string;
  size?: number;
  className?: string;
}

export function CreatorLogo({ creator, size = 36, className }: Props) {
  const render = LOGOS[creator];
  if (!render) return null;
  return <span className={`inline-flex items-center justify-center shrink-0 ${className ?? ""}`}>{render(size)}</span>;
}
