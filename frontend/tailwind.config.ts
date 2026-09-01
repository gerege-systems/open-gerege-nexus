import type { Config } from "tailwindcss";

/**
 * The utilities that read the theme.
 *
 * Before this file listed anything but `background`/`foreground` — two entries
 * nothing used — every screen reached for Tailwind's own palette directly:
 * `bg-white`, `text-slate-900`, `border-slate-200`, `bg-red-50`. Those are
 * fixed colours, so dark mode did not follow, and `globals.css` ended up
 * carrying fifty-odd `!important` rules that repainted each of them after the
 * fact. The names below are the same colours with the theme put back in front
 * of them: one definition in `globals.css`, both modes, no override layer.
 *
 * Values are `var(...)` rather than channel triplets, so the opacity modifier
 * (`bg-surface/50`) does not apply to them. That is deliberate — a translucent
 * surface is a decision, and the two places that genuinely need one say so with
 * `--gerege-overlay`.
 */
const config: Config = {
  // The class, not a `data-theme` attribute: `dark:` variants and the theme
  // switch then agree on one signal, and the pre-paint script in
  // `public/theme-init.js` can set it before the first frame is drawn.
  darkMode: "class",
  content: [
    "./app/**/*.{js,ts,jsx,tsx,mdx}",
    "./components/**/*.{js,ts,jsx,tsx,mdx}",
    "./features/**/*.{js,ts,jsx,tsx,mdx}",
  ],
  theme: {
    extend: {
      colors: {
        // Surfaces, page outwards.
        background: "var(--gerege-bg)",
        surface: "var(--gerege-surface)",
        "surface-2": "var(--gerege-surface-2)",
        "surface-hover": "var(--gerege-surface-hover)",
        chrome: "var(--gerege-chrome)",

        // Text, three steps.
        foreground: "var(--gerege-fg)",
        muted: "var(--gerege-muted)",
        subtle: "var(--gerege-fg-subtle)",

        // The one accent, plus what may be written on it.
        accent: "var(--gerege-blue)",
        "accent-soft": "var(--gerege-blue-soft)",
        "on-accent": "var(--gerege-on-blue)",

        // Borders. `line` is decoration; `input` is the interactive-control
        // border WCAG 1.4.11 holds to 3:1 on every surface it sits on.
        line: "var(--gerege-border)",
        "line-strong": "var(--gerege-border-strong)",
        input: "var(--gerege-border-input)",
        ring: "var(--gerege-ring)",
        overlay: "var(--gerege-overlay)",

        // Status: soft fill, its border, and the text that goes on the fill.
        danger: "var(--gerege-danger-fg)",
        "danger-soft": "var(--gerege-danger-soft)",
        "danger-border": "var(--gerege-danger-border)",
        "danger-solid": "var(--gerege-danger)",
        success: "var(--gerege-success-fg)",
        "success-soft": "var(--gerege-success-soft)",
        "success-border": "var(--gerege-success-border)",
        "success-solid": "var(--gerege-success)",
        warning: "var(--gerege-warning-fg)",
        "warning-soft": "var(--gerege-warning-soft)",
        "warning-border": "var(--gerege-warning-border)",
        info: "var(--gerege-info-fg)",
        "info-soft": "var(--gerege-info-soft)",
        "info-border": "var(--gerege-info-border)",
        neutral: "var(--gerege-neutral-fg)",
        "neutral-soft": "var(--gerege-neutral-soft)",
        "neutral-border": "var(--gerege-neutral-border)",
      },
      borderRadius: {
        sm: "var(--gerege-radius-sm)",
        DEFAULT: "var(--gerege-radius-control)",
        md: "var(--gerege-radius-control)",
        lg: "var(--gerege-radius-card)",
        xl: "var(--gerege-radius-lg)",
      },
      boxShadow: {
        sm: "var(--gerege-shadow-sm)",
        DEFAULT: "var(--gerege-shadow-sm)",
        md: "var(--gerege-shadow-md)",
        lg: "var(--gerege-shadow-lg)",
        menu: "var(--gerege-shadow-menu)",
      },
      zIndex: {
        dropdown: "var(--z-dropdown)",
        sticky: "var(--z-sticky)",
        overlay: "var(--z-overlay)",
        modal: "var(--z-modal)",
        popover: "var(--z-popover)",
        toast: "var(--z-toast)",
        tooltip: "var(--z-tooltip)",
      },
    },
  },
  plugins: [],
};

export default config;
