"use client";

import React, { createContext, useContext, useEffect, useMemo, useState } from "react";

import { DEFAULT_BRAND } from "../brand";
import type { BrandCopy } from "../brandCopy";
import { coreDictionary, type TranslationKey } from "./core";
import { lookup } from "./registry";
import { overlays } from "./locales";

// Imported for its side effects: every app in this build registers its own
// translations at import time. See apps/index.ts for why that happens here
// rather than from each app's route.
import "./apps";

// The type and the two constants live in lib/locale.ts, which carries no
// "use client": the server reads them too, and a value imported from a client
// module reaches a server component as a stub rather than as itself.
export type { Locale } from "../locale";
import { DEFAULT_LOCALE as DEFAULT_LOCALE_VALUE, LOCALE_KEY, type Locale } from "../locale";

/**
 * Mongolian plus the six official languages of the United Nations, in the UN's
 * own alphabetical order. The list is deliberately a policy rather than a
 * wishlist: every entry here is one more column every future translation has to
 * fill, so growing it is a decision, not a convenience.
 */
export const LOCALES: { code: Locale; label: string; flag: string; rtl?: boolean }[] = [
  { code: "mn", label: "Монгол", flag: "/icons/flag-mn.png" },
  { code: "ar", label: "العربية", flag: "/icons/flag-ar.png", rtl: true },
  { code: "zh", label: "中文", flag: "/icons/flag-zh.png" },
  { code: "en", label: "English", flag: "/icons/flag-en.png" },
  { code: "fr", label: "Français", flag: "/icons/flag-fr.png" },
  { code: "ru", label: "Русский", flag: "/icons/flag-ru.png" },
  { code: "es", label: "Español", flag: "/icons/flag-es.png" },
];

/**
 * What a fresh install offers: Mongolian, the source language, and English, the
 * one every translation falls back to. These two cannot be switched off — that
 * is what keeps "turn everything off" from being a reachable state, so no guard
 * against an empty language list is needed anywhere else.
 *
 * The remaining five are opt-in per device from Settings → Appearance. They are
 * shipped but not offered, because the dictionary is not fully translated into
 * them yet: an untranslated term falls back to English (see `t` below), and
 * that is a reasonable screen to hand someone who asked for French, but not one
 * to hand everybody by default.
 */
export const DEFAULT_LOCALES: Locale[] = ["mn", "en"];
const OPTIONAL_LOCALES: Locale[] = LOCALES.map((l) => l.code).filter(
  (code) => !DEFAULT_LOCALES.includes(code),
);

const STORAGE_KEY = LOCALE_KEY;
const ENABLED_STORAGE_KEY = "locales.enabled";
const DEFAULT_LOCALE = DEFAULT_LOCALE_VALUE;

export type { TranslationKey };

/**
 * A key t() will resolve.
 *
 * A platform key is checked at compile time; an app key is a string, because
 * the app that defines it may not be in this repository. `(string & {})`
 * rather than `string` so the union still offers the platform keys to an
 * editor completing one.
 *
 * Named rather than written out at each use because screens declare keys in
 * data as well as passing them to t() — a resource registry that types its
 * labels `TranslationKey` compiles only for apps that live here, which is the
 * one thing a shell shared by every distribution must not require.
 */
export type DictionaryKey = TranslationKey | (string & {});

interface I18nValue {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  /** Locales offered in the switchers: the two defaults plus whatever is on. */
  availableLocales: Locale[];
  /** Turn one of the optional locales on or off. The defaults ignore this. */
  setLocaleEnabled: (locale: Locale, enabled: boolean) => void;
  /**
   * A platform key is checked at compile time; an app key is a string, because
   * the app that defines it may not be in this repository. `(string & {})`
   * rather than `string` so the union still offers the platform keys.
   */
  t: (key: DictionaryKey, vars?: Record<string, string | number>) => string;
}

const I18nContext = createContext<I18nValue | null>(null);

const ALL_CODES = LOCALES.map((entry) => entry.code);

function isLocale(value: unknown): value is Locale {
  return typeof value === "string" && (ALL_CODES as string[]).includes(value);
}

/**
 * `brand` is the deployment's product name, and every translation may use it as
 * `{brand}`.
 *
 * The name used to be written into the strings themselves — nineteen entries in
 * the dictionary and their overlays in five more languages all said "Gerege
 * Nexus" — which made a rebrand a translation job in seven languages. It is a
 * variable now, and one nobody has to pass: a sentence that names the product
 * is ordinary prose, and asking every caller to hand `t()` the same value would
 * mean the one that forgot renders "{brand}" at somebody.
 *
 * It has a default so that a provider mounted without one — a test, a harness —
 * still reads as this product rather than as plumbing.
 *
 * `copy` is the deployment's own wording, from `BRAND_COPY_FILE`. A name is not
 * always the whole difference between two deployments: one that positions
 * itself differently says so in prose, and prose is not a variable. Overrides
 * are matched per key *and* per locale, so supplying English does not put
 * English in front of a Mongolian reader — it leaves that language reading as
 * it did, which is the honest state for a translation nobody has written yet.
 */
export function I18nProvider({
  brand = DEFAULT_BRAND.name,
  copy = {},
  children,
}: {
  brand?: string;
  copy?: BrandCopy;
  children: React.ReactNode;
}) {
  // Server and first client render must agree, so the stored preference is
  // applied in an effect rather than during initial state.
  const [locale, setLocaleState] = useState<Locale>(DEFAULT_LOCALE);
  const [extraLocales, setExtraLocales] = useState<Locale[]>([]);

  useEffect(() => {
    const stored = window.localStorage.getItem(STORAGE_KEY);
    let enabled: Locale[] = [];
    try {
      const raw = window.localStorage.getItem(ENABLED_STORAGE_KEY);
      const parsed: unknown = raw ? JSON.parse(raw) : [];
      if (Array.isArray(parsed)) {
        enabled = parsed.filter(isLocale).filter((code) => OPTIONAL_LOCALES.includes(code));
      }
    } catch {
      // A hand-edited or half-written value should cost the user their language
      // list, not the whole shell — fall back to shipping defaults.
      enabled = [];
    }
    setExtraLocales(enabled);
    // Only restore a stored language that is still on offer. Someone who picked
    // French and later switched it off would otherwise be stuck in a language
    // the switcher no longer shows.
    if (isLocale(stored) && (DEFAULT_LOCALES.includes(stored) || enabled.includes(stored))) {
      setLocaleState(stored);
    }
  }, []);

  useEffect(() => {
    document.documentElement.lang = locale;
    // Arabic reverses the whole layout, so the document direction follows the
    // language rather than being set per component.
    document.documentElement.dir = LOCALES.find((l) => l.code === locale)?.rtl ? "rtl" : "ltr";
  }, [locale]);

  const value = useMemo<I18nValue>(() => {
    const availableLocales = ALL_CODES.filter(
      (code) => DEFAULT_LOCALES.includes(code) || extraLocales.includes(code),
    );

    const setLocale = (next: Locale) => {
      window.localStorage.setItem(STORAGE_KEY, next);
      // Also a cookie, because the server has to know: the tab title, the
      // description and the launcher name are metadata, produced before any of
      // this runs, and localStorage is not readable there. Without it a reader
      // in English had an English page inside a Mongolian-titled tab.
      document.cookie = `${STORAGE_KEY}=${next}; path=/; max-age=31536000; samesite=lax`;
      setLocaleState(next);
    };

    const setLocaleEnabled = (target: Locale, enabled: boolean) => {
      if (DEFAULT_LOCALES.includes(target)) return; // mn/en are not switchable
      const next = enabled
        ? ALL_CODES.filter((code) => code === target || extraLocales.includes(code))
        : extraLocales.filter((code) => code !== target);
      window.localStorage.setItem(ENABLED_STORAGE_KEY, JSON.stringify(next));
      setExtraLocales(next);
      // Switching off the language currently being read would leave the user
      // looking at a locale with no way back to it.
      if (!enabled && locale === target) setLocale(DEFAULT_LOCALE);
    };

    // The deployment's name in the language being read, when it wrote one:
    // `{brand}` inside a translated sentence has to agree with the header
    // beside it, and the header follows the same override (lib/brand.ts).
    const brandName = copy["brand.name"]?.[locale]?.trim() || brand;

    const t = (key: DictionaryKey, vars?: Record<string, string | number>) => {
      // Entries are authored with mn and en; the other five locales are filled
      // in progressively, so a lookup is widened to "this locale, maybe".
      const entry = (coreDictionary as Record<string, Partial<Record<Locale, string>> | undefined>)[key];
      // The deployment's own wording first, when it wrote this key in this
      // language: it is the most specific statement anybody has made about what
      // this string should say, and the only one made about this deployment.
      //
      // Then the overlay: that is where the generated and reviewed translations
      // for the optional languages live. Then the entry's own locale, then the
      // English source term rather than the key, as gettext does — an
      // untranslated screen reads as English, not as plumbing.
      //
      // An app's words come last of the three sources and by the same rules:
      // registry.lookup already prefers the language asked for, and falls back
      // to the app's English before this does to the key. A key belonging to no
      // app and no core entry renders as itself, which is what it always did.
      let text: string =
        copy[key]?.[locale] ||
        overlays[locale]?.[key] ||
        (entry ? entry[locale] || entry.en : undefined) ||
        lookup(locale, key) ||
        lookup("en", key) ||
        key;
      // The brand is offered to every key, so a string that names the product
      // needs nothing at the call site. A caller that passes its own `brand`
      // still wins — the spread order says so — which is what a screen naming
      // some *other* application would want.
      //
      // Guarded on a brace rather than run unconditionally: almost no entry
      // has a placeholder, `t()` is called for every menu item on every render,
      // and this used to build no regular expression at all when the caller
      // passed no variables.
      if (text.includes("{")) {
        for (const [name, replacement] of Object.entries({ brand: brandName, ...(vars ?? {}) })) {
          text = text.replace(new RegExp(`\\{${name}\\}`, "g"), String(replacement));
        }
      }
      return text;
    };

    return { locale, setLocale, availableLocales, setLocaleEnabled, t };
  }, [locale, extraLocales, brand, copy]);

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n(): I18nValue {
  const context = useContext(I18nContext);
  if (!context) {
    throw new Error("useI18n must be used inside I18nProvider");
  }
  return context;
}
