/**
 * Provider logos — who SERVES the model (API provider).
 * Single source of truth. Used in settings, wizard, indicator.
 */

const LOGOS: Record<string, (size: number) => React.ReactNode> = {
  chatgpt: (s) => (
    <svg width={s} height={s} viewBox="0 0 24 24">
      <path d="M22.2819 9.8211a5.9847 5.9847 0 0 0-.5157-4.9108 6.0462 6.0462 0 0 0-6.5098-2.9A6.0651 6.0651 0 0 0 4.9807 4.1818a5.9847 5.9847 0 0 0-3.9977 2.9 6.0462 6.0462 0 0 0 .7427 7.0966 5.98 5.98 0 0 0 .511 4.9107 6.051 6.051 0 0 0 6.5146 2.9001A5.9847 5.9847 0 0 0 13.2599 24a6.0557 6.0557 0 0 0 5.7718-4.2058 5.9894 5.9894 0 0 0 3.9977-2.9001 6.0557 6.0557 0 0 0-.7475-7.0729z" fill="#10A37F"/>
    </svg>
  ),
  openai: (s) => (
    <svg width={s} height={s} viewBox="0 0 24 24">
      <path d="M22.2819 9.8211a5.9847 5.9847 0 0 0-.5157-4.9108 6.0462 6.0462 0 0 0-6.5098-2.9A6.0651 6.0651 0 0 0 4.9807 4.1818a5.9847 5.9847 0 0 0-3.9977 2.9 6.0462 6.0462 0 0 0 .7427 7.0966 5.98 5.98 0 0 0 .511 4.9107 6.051 6.051 0 0 0 6.5146 2.9001A5.9847 5.9847 0 0 0 13.2599 24a6.0557 6.0557 0 0 0 5.7718-4.2058 5.9894 5.9894 0 0 0 3.9977-2.9001 6.0557 6.0557 0 0 0-.7475-7.0729z" fill="#cdd6f4"/>
    </svg>
  ),
  anthropic: (s) => (
    <svg width={s} height={s} viewBox="0 0 24 24">
      <path d="M17.3041 3.541h-3.6718l6.696 16.918H24Zm-10.6082 0L0 20.459h3.7442l1.3693-3.5527h7.0052l1.3693 3.5528h3.7442L10.5363 3.5409Zm-.3712 10.2232 2.2914-5.9456 2.2914 5.9456Z" fill="#D4A574"/>
    </svg>
  ),
  gemini: (s) => (
    <svg width={s} height={s} viewBox="0 0 24 24">
      <defs><linearGradient id="gmi" x1="0" y1="0" x2="24" y2="24" gradientUnits="userSpaceOnUse"><stop stopColor="#4285F4"/><stop offset=".33" stopColor="#9B72CB"/><stop offset=".66" stopColor="#D96570"/><stop offset="1" stopColor="#F9AB00"/></linearGradient></defs>
      <path d="M11.04 19.32Q12 21.51 12 24q0-2.49.93-4.68.96-2.19 2.58-3.81t3.81-2.55Q21.51 12 24 12q-2.49 0-4.68-.93a12.3 12.3 0 0 1-3.81-2.58 12.3 12.3 0 0 1-2.58-3.81Q12 2.49 12 0q0 2.49-.96 4.68-.93 2.19-2.55 3.81a12.3 12.3 0 0 1-3.81 2.58Q2.49 12 0 12q2.49 0 4.68.96 2.19.93 3.81 2.55t2.55 3.81" fill="url(#gmi)"/>
    </svg>
  ),
  xai: (s) => (
    <svg width={s} height={s} viewBox="0 0 24 24">
      <path d="M14.234 10.162 22.977 0h-2.072l-7.591 8.824L7.251 0H.258l9.168 13.343L.258 24H2.33l8.016-9.318L16.749 24h6.993zm-2.837 3.299-.929-1.329L3.076 1.56h3.182l5.965 8.532.929 1.329 7.754 11.09h-3.182z" fill="#cdd6f4"/>
    </svg>
  ),
  deepseek: (s) => (
    <svg width={s} height={s} viewBox="0 0 24 24">
      <rect width="24" height="24" rx="6" fill="#4D6BFE"/>
      <text x="12" y="9" textAnchor="middle" fontSize="5.5" fontWeight="700" fill="#fff" fontFamily="Arial,sans-serif">DEEP</text>
      <text x="12" y="16" textAnchor="middle" fontSize="5.5" fontWeight="700" fill="#fff" fontFamily="Arial,sans-serif">SEEK</text>
    </svg>
  ),
  ollama: (s) => (
    <svg width={s} height={s} viewBox="0 0 24 24">
      <path d="M16.361 10.26a.894.894 0 0 0-.558.47l-.072.148.001.207c0 .193.004.217.059.353.076.193.152.312.291.448.24.238.51.3.872.205a.86.86 0 0 0 .517-.436.752.752 0 0 0 .08-.498c-.064-.453-.33-.782-.724-.897a1.06 1.06 0 0 0-.466 0zm-9.203.005c-.305.096-.533.32-.65.639a1.187 1.187 0 0 0-.06.52c.057.309.31.59.598.667.362.095.632.033.872-.205.14-.136.215-.255.291-.448.055-.136.059-.16.059-.353l.001-.207-.072-.148a.894.894 0 0 0-.565-.472 1.02 1.02 0 0 0-.474.007z" fill="#cdd6f4"/>
    </svg>
  ),
  local: (s) => (
    <span className="relative inline-block" style={{width: s, height: s}}>
      <img src="/ghost-icon.png" width={s} height={s} alt="Local" />
      <span className="absolute -bottom-0.5 -right-1 text-[10px]" style={{textShadow: "0 0 3px #1e1e2e, 0 0 3px #1e1e2e"}}>🏠</span>
    </span>
  ),
  lmstudio: (s) => (
    <svg width={s} height={s} viewBox="0 0 24 24">
      <defs><linearGradient id="lms" x1="0" y1="0" x2="24" y2="24" gradientUnits="userSpaceOnUse"><stop stopColor="#8B7BF7"/><stop offset="1" stopColor="#6C5CE7"/></linearGradient></defs>
      <rect width="24" height="24" rx="5" fill="url(#lms)"/>
      <rect x="4" y="4" width="14" height="2.4" rx="1.2" fill="#fff" opacity=".9"/>
      <rect x="6" y="7.6" width="14" height="2.4" rx="1.2" fill="#fff" opacity=".75"/>
      <rect x="3" y="11.2" width="14" height="2.4" rx="1.2" fill="#fff" opacity=".85"/>
      <rect x="5" y="14.8" width="14" height="2.4" rx="1.2" fill="#fff" opacity=".7"/>
      <rect x="7" y="18.4" width="12" height="2.4" rx="1.2" fill="#fff" opacity=".6"/>
    </svg>
  ),
};

interface Props {
  provider: string;
  size?: number;
  className?: string;
}

export function ProviderLogo({ provider, size = 28, className }: Props) {
  const render = LOGOS[provider];
  if (!render) return null;
  return <span className={`inline-flex items-center justify-center shrink-0 ${className ?? ""}`}>{render(size)}</span>;
}
