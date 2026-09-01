"use client";

import { LOCALES, useI18n } from "@/lib/i18n";

/**
 * Locale toggle. Flags come from the Flaticon set stored in the repository —
 * see docs/assets/icons/ATTRIBUTION.md.
 */
export default function LanguageSwitcher({ variant = "light" }: { variant?: "light" | "dark" }) {
  const { locale, setLocale, availableLocales, t } = useI18n();
  // Only the languages this device has switched on — the full LOCALES list is
  // the catalogue, not the offer.
  const offered = LOCALES.filter((option) => availableLocales.includes(option.code));

  const base =
    variant === "dark"
      ? "border-slate-700 bg-slate-900/70"
      : "border-line bg-surface";
  const activeStyle =
    variant === "dark"
      ? "bg-indigo-500/20 text-white"
      : "bg-indigo-50 text-indigo-700";
  const idleStyle =
    variant === "dark"
      ? "text-slate-400 hover:text-slate-200"
      : "text-muted hover:text-foreground";

  return (
    <div
      className={`inline-flex items-center gap-0.5 rounded-lg border p-0.5 ${base}`}
      role="group"
      aria-label={t("base.field.language")}
    >
      {offered.map((option) => (
        <button
          key={option.code}
          type="button"
          onClick={() => setLocale(option.code)}
          aria-pressed={locale === option.code}
          title={option.label}
          className={`flex items-center gap-1.5 rounded-md px-2 py-1 text-xs font-semibold transition ${
            locale === option.code ? activeStyle : idleStyle
          }`}
        >
          <img src={option.flag} alt="" width={14} height={14} className="rounded-sm" />
          <span className="uppercase">{option.code}</span>
        </button>
      ))}
    </div>
  );
}
