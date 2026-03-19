/**
 * GhostSpell Design Tokens — Catppuccin Mocha, Zen Edition
 *
 * Dark, minimalistic, calm. Generous whitespace, subtle borders,
 * soft glows instead of hard shadows. Nothing screams.
 */

export const colors = {
  // Base surfaces — darkest to lightest
  crust: "#11111b",
  base: "#1e1e2e",
  mantle: "#181825",
  surface0: "#313244",
  surface1: "#45475a",
  surface2: "#585b70",

  // Text hierarchy
  text: "#cdd6f4",
  subtext1: "#bac2de",
  subtext0: "#a6adc8",
  overlay2: "#9399b2",
  overlay1: "#7f849c",
  overlay0: "#6c7086",

  // Accent colors — muted, not neon
  blue: "#89b4fa",
  green: "#a6e3a1",
  red: "#f38ba8",
  yellow: "#f9e2af",
  peach: "#fab387",
  pink: "#f5c2e7",
  teal: "#94e2d5",
  sky: "#74c7ec",
  lavender: "#b4befe",
  mauve: "#cba6f7",
  flamingo: "#f2cdcd",
  rosewater: "#f5e0dc",
} as const;

export const spacing = {
  xs: "4px",
  sm: "8px",
  md: "16px",
  lg: "24px",
  xl: "32px",
  xxl: "48px",
} as const;

export const radius = {
  sm: "6px",
  md: "10px",
  lg: "14px",
  xl: "20px",
  full: "9999px",
} as const;

export const transitions = {
  fast: "150ms ease",
  normal: "200ms ease",
  slow: "300ms ease",
  spring: "400ms cubic-bezier(0.16, 1, 0.3, 1)",
} as const;
