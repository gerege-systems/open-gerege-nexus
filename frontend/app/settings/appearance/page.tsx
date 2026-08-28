"use client";

import { Check, Moon, Monitor, Palette, RotateCcw, Sun } from "lucide-react";
import { Accent, ColorMode, Density, DesignTheme, useTheme } from "@/lib/theme";
import { DEFAULT_LOCALES, LOCALES, TranslationKey, useI18n } from "@/lib/i18n";

// Every label resolves through the dictionary, so this screen switches with the
// rest of the app instead of carrying its own inline translations.
const modes: { value: ColorMode; label: TranslationKey; icon: typeof Sun }[] = [
  { value: "light", label: "appearance.mode.light", icon: Sun },
  { value: "dark", label: "appearance.mode.dark", icon: Moon },
  { value: "system", label: "appearance.mode.system", icon: Monitor },
];

const accents: { value: Accent; label: TranslationKey; color: string }[] = [
  { value: "neutral", label: "appearance.accent.neutral", color: "#64748b" },
  { value: "cobalt", label: "appearance.accent.cobalt", color: "#0064e1" },
  { value: "teal", label: "appearance.accent.teal", color: "#008b99" },
  { value: "violet", label: "appearance.accent.violet", color: "#7656d6" },
  { value: "emerald", label: "appearance.accent.emerald", color: "#16845b" },
];

const styles: { value: DesignTheme; name: TranslationKey; hint: TranslationKey; top: string; accent: string }[] = [
  { value: "original", name: "appearance.style.original", hint: "appearance.view.original_hint", top: "#0064e1", accent: "#6366f1" },
  { value: "gerege", name: "appearance.style.gerege", hint: "appearance.view.gerege_hint", top: "#ffffff", accent: "#0064e1" },
];

const densities: { value: Density; label: TranslationKey }[] = [
  { value: "comfortable", label: "appearance.density.comfortable" },
  { value: "compact", label: "appearance.density.compact" },
];

export default function AppearanceSettingsPage() {
  const { t, availableLocales, setLocaleEnabled } = useI18n();
  const theme = useTheme();

  return (
    <div className="w-full space-y-6">
      <div className="border-b border-slate-200 pb-4 flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold flex items-center gap-2">
            <Palette className="w-6 h-6 text-[var(--gerege-blue)]" />
            {t("appearance.view.title")}
          </h1>
          <p className="text-sm text-slate-500 mt-1">{t("appearance.view.subtitle")}</p>
        </div>
        <button onClick={theme.resetTheme} className="flex items-center gap-2 px-3 py-2 text-sm border border-slate-200 rounded-lg bg-white hover:bg-slate-50 text-slate-600">
          <RotateCcw className="w-4 h-4" />
          {t("appearance.action.reset")}
        </button>
      </div>

      <section className="bg-white border border-slate-200 rounded-[var(--gerege-radius-card)] p-5 shadow-sm">
        <h2 className="font-semibold text-base">{t("appearance.field.languages")}</h2>
        <p className="text-sm text-slate-500 mt-1">{t("appearance.view.languages_hint")}</p>
        <p className="text-xs text-slate-400 mt-1 mb-4">{t("appearance.view.languages_partial")}</p>
        <ul className="divide-y divide-slate-100 border border-slate-200 rounded-xl overflow-hidden">
          {LOCALES.map((option) => {
            const isDefault = DEFAULT_LOCALES.includes(option.code);
            const isOn = availableLocales.includes(option.code);
            return (
              <li key={option.code} className="flex items-center gap-3 px-4 py-3">
                <img src={option.flag} alt="" width={20} height={20} className="rounded-sm shrink-0" />
                <span className="text-sm font-medium text-slate-800 min-w-0 truncate">{option.label}</span>
                <span className="text-[11px] uppercase tracking-wider text-slate-400">{option.code}</span>
                <span className="ml-auto">
                  {isDefault ? (
                    // Not a disabled control: there is nothing to press, so the
                    // state is stated rather than shown as a dead switch.
                    <span className="text-xs text-slate-400">{t("appearance.state.language_always")}</span>
                  ) : (
                    <button
                      type="button"
                      role="switch"
                      aria-checked={isOn}
                      aria-label={option.label}
                      onClick={() => setLocaleEnabled(option.code, !isOn)}
                      className={`relative w-11 h-6 rounded-full transition ${isOn ? "bg-[var(--gerege-blue)]" : "bg-slate-200"}`}
                    >
                      <span className={`absolute top-0.5 w-5 h-5 rounded-full bg-white shadow transition-all ${isOn ? "left-[22px]" : "left-0.5"}`} />
                    </button>
                  )}
                </span>
              </li>
            );
          })}
        </ul>
      </section>

      <section className="bg-white border border-slate-200 rounded-[var(--gerege-radius-card)] p-5 shadow-sm">
        <h2 className="font-semibold text-base">{t("appearance.field.theme_style")}</h2>
        <p className="text-sm text-slate-500 mt-1 mb-4">{t("appearance.view.theme_style_hint")}</p>
        <div className="grid sm:grid-cols-2 gap-4">
          {styles.map((item) => (
            <button key={item.value} onClick={() => theme.updateTheme({ design: item.value })} className={`overflow-hidden rounded-xl border text-left transition ${theme.design === item.value ? "border-[var(--gerege-blue)] ring-2 ring-[color-mix(in_srgb,var(--gerege-blue)_15%,transparent)]" : "border-slate-200 hover:border-slate-300"}`}>
              <span className="block h-12 border-b border-slate-200" style={{ background: item.top }}>
                <span className="block w-1/3 h-full border-r border-black/10" style={{ background: item.value === "original" ? "#ffffff" : "#f7f9fc" }} />
              </span>
              <span className="flex items-center gap-3 p-4 bg-white">
                <span className="w-3 h-8 rounded-full" style={{ background: item.accent }} />
                <span className="flex-1">
                  <strong className="block text-sm">{t(item.name)}</strong>
                  <span className="block text-xs text-slate-500 mt-0.5">{t(item.hint)}</span>
                </span>
                {theme.design === item.value && <Check className="w-4 h-4 text-[var(--gerege-blue)]" />}
              </span>
            </button>
          ))}
        </div>
      </section>

      <section className="bg-white border border-slate-200 rounded-[var(--gerege-radius-card)] p-5 shadow-sm">
        <h2 className="font-semibold text-base">{t("appearance.field.color_mode")}</h2>
        <p className="text-sm text-slate-500 mt-1 mb-4">{t("appearance.view.color_mode_hint")}</p>
        <div className="grid sm:grid-cols-3 gap-3">
          {modes.map(({ value, label, icon: Icon }) => (
            <button key={value} onClick={() => theme.updateTheme({ mode: value })} className={`relative flex items-center gap-3 p-4 rounded-lg border text-left transition ${theme.mode === value ? "border-[var(--gerege-blue)] bg-[var(--gerege-blue-soft)]" : "border-slate-200 hover:border-slate-300"}`}>
              <Icon className="w-5 h-5 text-[var(--gerege-blue)]" />
              <span className="font-medium text-sm">{t(label)}</span>
              {theme.mode === value && <Check className="absolute right-3 w-4 h-4 text-[var(--gerege-blue)]" />}
            </button>
          ))}
        </div>
      </section>

      <section className="bg-white border border-slate-200 rounded-[var(--gerege-radius-card)] p-5 shadow-sm">
        <h2 className="font-semibold text-base">{t("appearance.field.accent")}</h2>
        <p className="text-sm text-slate-500 mt-1 mb-4">{t("appearance.view.accent_hint")}</p>
        <div className="grid sm:grid-cols-2 gap-3">
          {accents.map((accent) => (
            <button key={accent.value} onClick={() => theme.updateTheme({ accent: accent.value })} className={`flex items-center gap-3 p-3 rounded-lg border text-left ${theme.accent === accent.value ? "border-[var(--gerege-blue)] bg-[var(--gerege-blue-soft)]" : "border-slate-200 hover:border-slate-300"}`}>
              <span className="w-8 h-8 rounded-lg shadow-inner" style={{ background: accent.color }} />
              <span className="font-medium text-sm flex-1">{t(accent.label)}</span>
              {theme.accent === accent.value && <Check className="w-4 h-4 text-[var(--gerege-blue)]" />}
            </button>
          ))}
        </div>
      </section>

      <section className="bg-white border border-slate-200 rounded-[var(--gerege-radius-card)] p-5 shadow-sm">
        <h2 className="font-semibold text-base">{t("appearance.field.density")}</h2>
        <div className="mt-4 inline-flex rounded-lg border border-slate-200 p-1 bg-slate-50">
          {densities.map(({ value, label }) => (
            <button key={value} onClick={() => theme.updateTheme({ density: value })} className={`px-4 py-2 rounded-md text-sm font-medium ${theme.density === value ? "bg-white text-[var(--gerege-blue)] shadow-sm" : "text-slate-500"}`}>
              {t(label)}
            </button>
          ))}
        </div>
      </section>
    </div>
  );
}
