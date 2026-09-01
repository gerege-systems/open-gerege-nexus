"use client";

/**
 * The bits of a screen's state that belong in its address.
 *
 * A filter, a sort, a page number and a search term are not private to one
 * browser tab — they are what the reader is currently looking at. Held in
 * `useState` they survive nothing: refreshing loses them, the Back button
 * leaves the screen instead of undoing the filter, and a link pasted to a
 * colleague opens on a different list than the one being discussed. Every
 * console screen here held them that way.
 *
 * The rule this follows (11-dashboard-patterns): what the *data* looks like
 * goes in the URL; what the *device* looks like (density, sidebar, theme) goes
 * in localStorage; what the *account* prefers goes to the server. And it goes
 * in exactly one of those — a value duplicated across two of them is a value
 * that will disagree with itself.
 */

import { useCallback, useMemo, useRef } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";

export type UrlState = Record<string, string>;

/**
 * Reads the named query parameters, and returns a setter that writes them back.
 *
 * `defaults` names every key this screen owns and what it means for one to be
 * absent. A value equal to its default is removed from the address rather than
 * written out, so an untouched screen has a clean URL and a filtered one says
 * exactly what is filtered.
 *
 * The defaults are captured once. A screen's set of keys is a fact about the
 * screen, not about a render, and freezing it is what lets the returned setter
 * keep a stable identity — otherwise every caller that puts it in an effect's
 * dependencies gets a loop.
 */
export function useUrlState<T extends UrlState>(defaults: T) {
  const router = useRouter();
  const pathname = usePathname();
  const params = useSearchParams();
  const fixed = useRef(defaults);

  // Keyed on the query *string*, not on the params object. Next returns a
  // stable instance per navigation, but that is an implementation detail to
  // depend on: anything that hands back a fresh object per render — a test
  // double, a future version — would make `state` a new object every render,
  // and a caller with `state` in an effect's dependencies would then loop.
  const query = params.toString();
  const state = useMemo(() => {
    const read = {} as T;
    const current = new URLSearchParams(query);
    for (const key of Object.keys(fixed.current) as (keyof T & string)[]) {
      read[key] = (current.get(key) ?? fixed.current[key]) as T[typeof key];
    }
    return read;
  }, [query]);

  /**
   * Stable across renders on purpose: it reads the current query from the
   * address bar rather than from a captured `params`, so it never goes stale
   * and never changes identity.
   *
   * `replace` rather than `push`, because typing six letters into a search box
   * should leave one history entry and not six — otherwise Back becomes a way
   * to retype what you just deleted.
   */
  const setState = useCallback(
    (next: Partial<T>) => {
      const search = new URLSearchParams(
        typeof window === "undefined" ? "" : window.location.search,
      );
      for (const [key, value] of Object.entries(next)) {
        if (value === undefined) continue;
        if (value === fixed.current[key as keyof T]) search.delete(key);
        else search.set(key, String(value));
      }
      const query = search.toString();
      const target = query ? `${pathname}?${query}` : pathname;
      if (typeof window !== "undefined" && window.location.pathname + window.location.search === target) {
        return; // nothing to say; writing it again would re-render for nothing
      }
      router.replace(target, { scroll: false });
    },
    [pathname, router],
  );

  return [state, setState] as const;
}
