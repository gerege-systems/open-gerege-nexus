/**
 * A router for screens that keep state in their address.
 *
 * jsdom has no Next router, so any component calling `useRouter`,
 * `usePathname` or `useSearchParams` throws "invariant expected app router to
 * be mounted" the moment it renders. Three test files had already written
 * their own two-line stand-in; this is the same thing with a query string,
 * which the screens that moved their filters into the URL need.
 *
 * It is deliberately real enough to assert against. `replace` writes to
 * jsdom's own location — so a test can read back what a screen put in the
 * address — *and* notifies the components reading it, because in Next a
 * navigation re-renders and a mock that only writes would leave the screen
 * showing the filter it had before the one it just applied.
 */

import { useSyncExternalStore } from "react";
import { vi } from "vitest";

const listeners = new Set<() => void>();

function announce() {
  for (const listener of listeners) listener();
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => void listeners.delete(listener);
}

/** The query string, as the snapshot `useSyncExternalStore` compares. */
function snapshot() {
  return window.location.search;
}

export const replace = vi.fn((url: string) => {
  window.history.replaceState(null, "", url);
  announce();
});

export const push = vi.fn((url: string) => {
  window.history.pushState(null, "", url);
  announce();
});

export const useRouter = () => ({
  push,
  replace,
  refresh: vi.fn(),
  back: vi.fn(),
  forward: vi.fn(),
  prefetch: vi.fn(),
});

export const usePathname = () => useSyncExternalStore(subscribe, () => window.location.pathname, () => "/");
export const useSearchParams = () =>
  new URLSearchParams(useSyncExternalStore(subscribe, snapshot, () => ""));
export const useParams = () => ({});
