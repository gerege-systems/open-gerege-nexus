"use client";

import { LOCALES, useI18n } from "@/lib/i18n";

/**
 * Locale toggle: the language's code, in words.
 *
 * It used to carry a circle flag beside each code. A flag is a country and not
 * a language — the attribution file said as much about its own set, where
 * English is the flag of the United States and Arabic the flag of Saudi
 * Arabia. Neither is true of the people reading in those languages, and there
 * is no flag that is: Spanish and French each belong to dozens of countries,
 * and Mongolian is read on both sides of a border. The code alone says the
 * thing without claiming the other.
 *
 * The visible label stays the short code, so the control is the same size it
 * was. The full name is added out of sight for a screen reader, after the
 * code, so the accessible name still contains what is on screen (WCAG 2.5.3).
 */
export default function LanguageSwitcher({ variant = "light" }: { variant?: "light" | "dark" }) {
  const { locale, setLocale, availableLocales, t } = useI18n();
  // Only the languages this device has switched on — the full LOCALES list is
  // the catalogue, not the offer.
  const offered = LOCALES.filter((option) => availableLocales.includes(option.code));

  // A segmented control, the way UserMenu and the sign-in screen already draw
  // one: a recessed track with the chosen segment raised out of it. The colour
  // it used to mark the selection with was `indigo-50`/`indigo-700`, a second
  // brand hue in the header of every screen; the raised surface says the same
  // thing without spending an accent on it, and keeps working when the
  // deployment picks a different one.
  const base =
    variant === "dark"
      ? "border-slate-700 bg-slate-900/70"
      : "border-line bg-surface-2";
  const activeStyle =
    variant === "dark"
      ? "bg-slate-800 text-white"
      : "bg-surface text-accent shadow-sm";
  const idleStyle =
    variant === "dark"
      ? "text-slate-400 hover:text-slate-200"
      : "text-muted hover:text-foreground";

  return (
    <div
      className={`inline-flex items-center gap-0.5 rounded-md border p-0.5 ${base}`}
      role="group"
      aria-label={t("base.field.language")}
    >
      {offered.map((option) => (
        <button
          key={option.code}
          type="button"
          onClick={() => setLocale(option.code)}
          aria-pressed={locale === option.code}
          className={`flex items-center rounded px-2 py-1 text-xs font-semibold transition ${
            locale === option.code ? activeStyle : idleStyle
          }`}
        >
          <span className="uppercase">{option.code}</span>
          <span className="sr-only"> {option.label}</span>
        </button>
      ))}
    </div>
  );
}
