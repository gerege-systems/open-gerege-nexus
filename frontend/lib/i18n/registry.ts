/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

import type { Locale } from "../locale";

/**
 * Where an app's translations arrive from.
 *
 * The dictionary used to be one `as const` object built in index.tsx out of
 * twenty-five imports and twenty-five spreads, with five overlay files of about
 * 1381 lines each beside it. Adding an app meant editing all of them, which is
 * why they were among the files app work dragged along with it: the locale
 * files were changed by 21 of the last app commits, and the file that assembles
 * them by nearly as many. Not because anybody wanted a shared dictionary — because
 * there was nowhere else for an app's words to go.
 *
 * Now there is. An app hands its own strings over at import time:
 *
 *     registerDictionary("urtuu", { ...source(urtuu), ar, zh, fr, ru, es });
 *
 * and a distribution's app does exactly the same, for keys this repository has
 * never heard of. The core dictionary stays compile-time — see core.ts, where
 * TranslationKey comes from — so a typo in a platform key is still a build
 * error. An app key is a string, and t() takes either.
 */
const registry = new Map<string, Partial<Record<Locale, Record<string, string>>>>();

/** key → the appId that last provided it, per locale. Only used for the warning. */
const claimed = new Map<string, string>();

/**
 * Adds an app's translations to the dictionary.
 *
 * Order does not matter: t() reads the registry per lookup rather than
 * capturing a merged object, so an app registered after the provider mounted is
 * translated from that moment. In practice every app in this repository
 * registers at import time, before React renders anything — see apps/index.ts
 * for why that is deliberate rather than incidental.
 *
 * Registering the same appId twice replaces what it had. Two different apps
 * claiming one key is the case worth hearing about, so that is the one warned
 * about — in development only, because a warning nobody can act on in a
 * production console is noise.
 */
export function registerDictionary(
  appId: string,
  dictionaries: Partial<Record<Locale, Record<string, string>>>,
): void {
  if (process.env.NODE_ENV !== "production") {
    for (const [locale, entries] of Object.entries(dictionaries)) {
      for (const key of Object.keys(entries ?? {})) {
        const id = `${locale}:${key}`;
        const previous = claimed.get(id);
        if (previous && previous !== appId) {
          console.warn(
            `i18n: ${appId} overrides ${key} (${locale}), which ${previous} also provides`,
          );
        }
        claimed.set(id, appId);
      }
    }
  }
  registry.set(appId, dictionaries);
}

/**
 * What the registered apps say for this key in this language, if any.
 *
 * Last registration wins on a collision, matching how the spread that this
 * replaced behaved: the addon listed later in index.tsx overwrote the earlier.
 */
export function lookup(locale: Locale, key: string): string | undefined {
  for (const dictionaries of registry.values()) {
    const text = dictionaries[locale]?.[key];
    if (text !== undefined) return text;
  }
  return undefined;
}

/**
 * Turns an addon — authored key-major, `{ "a.b": { mn, en } }` — into the
 * locale-major shape the registry holds.
 *
 * The two shapes exist for two different readers. An addon is written and
 * reviewed by a person, who wants a term's languages side by side; an overlay
 * is written by the generator, one language at a time. Neither should have to
 * change to suit the other.
 */
export function source(
  addon: Record<string, Partial<Record<Locale, string>>>,
): Partial<Record<Locale, Record<string, string>>> {
  const out: Partial<Record<Locale, Record<string, string>>> = {};
  for (const [key, translations] of Object.entries(addon)) {
    for (const [locale, text] of Object.entries(translations)) {
      if (text === undefined) continue;
      (out[locale as Locale] ??= {})[key] = text;
    }
  }
  return out;
}
