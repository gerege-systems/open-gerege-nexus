"use client";

import React, { createContext, useContext, useEffect, useMemo, useState } from "react";
import { getShell } from "@/lib/shell";

export type ColorMode = "light" | "dark" | "system";
export type Accent = "neutral" | "cobalt" | "teal" | "violet" | "emerald";
export type Density = "comfortable" | "compact";
export type DesignTheme = "original" | "gerege";

export interface ThemePreferences {
  mode: ColorMode;
  accent: Accent;
  density: Density;
  design: DesignTheme;
}

const STORAGE_KEY = "gerege_theme";
const defaults: ThemePreferences = { design: "original", mode: "light", accent: "neutral", density: "comfortable" };

interface ThemeContextValue extends ThemePreferences {
  resolvedMode: "light" | "dark";
  updateTheme: (next: Partial<ThemePreferences>) => void;
  toggleMode: () => void;
  resetTheme: () => void;
}

const ThemeContext = createContext<ThemeContextValue | null>(null);

function resolveMode(mode: ColorMode): "light" | "dark" {
  if (mode !== "system") return mode;
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const [preferences, setPreferences] = useState<ThemePreferences>(defaults);
  const [resolvedMode, setResolvedMode] = useState<"light" | "dark">("light");
  /**
   * Whether the stored preferences have been read yet.
   *
   * Without it the first commit ran the effect below with `defaults` still in
   * hand — mode "light" — and painted the document light before the effect
   * above had finished reading localStorage. `public/theme-init.js` has
   * already put the right class on <html> by then, so that pass was not a
   * missing style, it was React actively undoing the pre-paint script and
   * flashing white at anyone who had chosen dark.
   */
  const [restored, setRestored] = useState(false);

  useEffect(() => {
    try {
      const saved = JSON.parse(localStorage.getItem(STORAGE_KEY) || "null");
      if (saved) {
        if (saved.design === "native") saved.design = "original";
        setPreferences({ ...defaults, ...saved });
      }
    } catch {
      localStorage.removeItem(STORAGE_KEY);
    }
    setRestored(true);
  }, []);

  useEffect(() => {
    if (!restored) return;
    const media = window.matchMedia("(prefers-color-scheme: dark)");
    const apply = () => {
      const nextMode = resolveMode(preferences.mode);
      setResolvedMode(nextMode);
      // A class, not a `data-theme` attribute: it is the signal Tailwind's
      // `dark:` variant reads and the one the pre-paint script sets, and two
      // names for one state is how they came to disagree.
      document.documentElement.classList.toggle("dark", nextMode === "dark");
      document.documentElement.dataset.accent = preferences.accent;
      document.documentElement.dataset.density = preferences.density;
      document.documentElement.dataset.design = preferences.design;
      document.documentElement.style.colorScheme = nextMode;
      // Бүрхүүлийн доторх харагдац нь хостынхоо платформоос хамаардаг. Хөтөч
      // дээр атрибут огт үлдэхгүй байх нь чухал — эсрэг тохиолдолд web горим
      // native дүрмүүдийг өвлөнө.
      const shell = getShell();
      if (shell) document.documentElement.dataset.shell = shell.platform;
      else delete document.documentElement.dataset.shell;
    };
    apply();
    media.addEventListener("change", apply);
    localStorage.setItem(STORAGE_KEY, JSON.stringify(preferences));
    return () => media.removeEventListener("change", apply);
  }, [preferences, restored]);

  const value = useMemo<ThemeContextValue>(() => ({
    ...preferences,
    resolvedMode,
    updateTheme: (next) => setPreferences((current) => ({ ...current, ...next })),
    toggleMode: () => setPreferences((current) => ({
      ...current,
      mode: resolveMode(current.mode) === "dark" ? "light" : "dark",
    })),
    resetTheme: () => setPreferences(defaults),
  }), [preferences, resolvedMode]);

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme() {
  const context = useContext(ThemeContext);
  if (!context) throw new Error("useTheme must be used inside ThemeProvider");
  return context;
}
