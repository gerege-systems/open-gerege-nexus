/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

/**
 * Per-language overlays for the platform's own dictionary, fetched when asked
 * for rather than shipped to everybody.
 *
 * Lookup order in `t()` is overlay → entry[locale] → entry.en, so a term is
 * translated the moment it appears here and falls back to English until then.
 * That is what lets a locale be switched on while it is still only partly
 * translated, instead of waiting for every key.
 *
 * These five languages are the ones a device has to switch on (Settings →
 * Appearance); mn and en are authored in the dictionary itself and are always
 * present. So the overlays are, on almost every visit, 262 KB nobody reads.
 * They used to be five static imports, which meant every page — the signed-out
 * landing page included — waited for Arabic, Chinese, French, Russian and
 * Spanish before it could render. Now each language is a chunk, and the
 * provider asks for one only after the reader has chosen it.
 *
 * The load is therefore asynchronous, and the screen is briefly the language it
 * falls back to. That was already true: the stored preference is applied in an
 * effect, so a French reader's first paint was Mongolian regardless. This moves
 * the moment the words arrive, not whether they replace something.
 */
import type { Locale } from "../../locale";

/** One language's words: the platform's overlay, plus each app's. */
export interface LocaleBundle {
  core: Record<string, string>;
  apps: Record<string, Record<string, string>>;
}

// A map of thunks rather than a switch, so that adding a language is one line
// and a language with no overlay is simply absent — `load` says so by
// returning undefined rather than resolving to an empty bundle nobody can
// distinguish from a failed fetch.
const loaders: Partial<Record<Locale, () => Promise<{ default: LocaleBundle }>>> = {
  ar: () => import("../bundles/ar"),
  zh: () => import("../bundles/zh"),
  fr: () => import("../bundles/fr"),
  ru: () => import("../bundles/ru"),
  es: () => import("../bundles/es"),
};

/** The chunk for this language, or undefined for one authored in the dictionary. */
export function loadLocaleBundle(locale: Locale): Promise<LocaleBundle> | undefined {
  const loader = loaders[locale];
  return loader?.().then((module) => module.default);
}
