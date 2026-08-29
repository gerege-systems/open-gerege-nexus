"use client";

/**
 * The dictionary, as the tests see it: every key answers with itself.
 *
 * `vi.mock("@/lib/i18n", () => import("../helpers/i18n"))` at the top of a test
 * file, and an assertion reads `cp.action.add_operator` instead of "Оператор
 * нэмэх". That is deliberate. A test written against the Mongolian wording
 * fails when somebody improves the wording, which teaches the next person that
 * the suite is noise; a test written against the key fails only when the screen
 * shows something else, which is what it is for.
 *
 * That whether a key exists at all is not checked here is not an omission: it
 * is `npm run i18n:check`'s job, and it checks all seven languages rather than
 * the one a test happened to render in.
 */

import React from "react";

export type Locale = "mn" | "en";

export const LOCALES = [
  { code: "mn", label: "Монгол", flag: "/icons/flag-mn.png" },
  { code: "en", label: "English", flag: "/icons/flag-en.png" },
];

export const DEFAULT_LOCALES: Locale[] = ["mn", "en"];

/**
 * Interpolation is kept visible rather than dropped.
 *
 * `cp.message.schedules_trouble` says how many schedules are failing and how
 * many have never run — a sentence whose whole content is its two numbers. A
 * double that returned the bare key would let a screen pass the test while
 * passing nothing into it.
 */
export function translate(key: string, vars?: Record<string, string | number>): string {
  if (!vars || Object.keys(vars).length === 0) return key;
  const named = Object.entries(vars)
    .map(([name, value]) => `${name}=${value}`)
    .join(" ");
  return `${key} {${named}}`;
}

export function useI18n() {
  return {
    locale: "mn" as Locale,
    setLocale: () => {},
    availableLocales: DEFAULT_LOCALES,
    setLocaleEnabled: () => {},
    t: translate,
  };
}

export function I18nProvider({ children }: { children: React.ReactNode }) {
  return <>{children}</>;
}
