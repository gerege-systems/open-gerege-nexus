"use client";

import { Button } from "@gerege-systems/ui";
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
  //
  // The dark variant sits on the landing header's navy (`--gerege-navy`),
  // which is the same in both colour modes and under every accent. The theme's
  // text tokens flip with the mode, so on that fixed surface they cannot be
  // relied on; `on-navy` (white) at an alpha is what reads.
  const base =
    variant === "dark"
      ? "border-on-navy/20 bg-overlay"
      : "border-line bg-surface-2";
  const activeStyle =
    variant === "dark"
      ? "bg-on-navy/15 text-on-navy hover:bg-on-navy/15 hover:text-on-navy"
      : "bg-surface text-accent shadow-sm hover:bg-surface hover:text-accent";
  const idleStyle =
    variant === "dark"
      ? "text-on-navy/80 hover:bg-transparent hover:text-on-navy"
      : "text-muted hover:bg-transparent hover:text-foreground";

  return (
    <div
      className={`inline-flex items-center gap-0.5 rounded-md border p-0.5 ${base}`}
      role="group"
      aria-label={t("base.field.language")}
    >
      {offered.map((option) => (
        <Button
          key={option.code}
          variant="ghost"
          size="sm"
          onClick={() => setLocale(option.code)}
          aria-pressed={locale === option.code}
          className={`h-7 rounded px-2 text-xs font-semibold ${locale === option.code ? activeStyle : idleStyle}`}
        >
          <span className="uppercase">{option.code}</span>
          <span className="sr-only"> {option.label}</span>
        </Button>
      ))}
    </div>
  );
}
