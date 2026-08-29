/**
 * What every test in this suite can assume.
 *
 * jsdom is a document, not a browser: it has no layout, so the handful of
 * browser APIs the shell asks about are absent rather than wrong. They are
 * filled in here, once, because a component that calls one of them crashes
 * with a message about `undefined` that says nothing about the test.
 *
 * A file that declares `@vitest-environment node` — the ones that read source
 * text rather than render — runs this too, so everything below is conditional
 * on there being a document at all.
 */
import { afterEach, vi } from "vitest";
import { cleanup } from "@testing-library/react";

const hasDocument = typeof window !== "undefined";

if (hasDocument) {
  // Node 24 grew a `localStorage` global of its own, and it answers `undefined`
  // unless the process was started with --localstorage-file. In vitest's jsdom
  // environment `window` *is* the global object, so that getter shadows jsdom's
  // storage on every Node new enough to have it: the console's frame, which
  // reads its folded menu groups straight from `localStorage`, crashes on a
  // developer's machine and passes on CI's Node 20. jsdom's `sessionStorage` is
  // untouched and still the real thing; only this one name is taken.
  //
  // A Map behind the Storage interface makes both Nodes behave like the
  // browser. It is not jsdom's implementation, but the only things asked of it
  // here are get, set, remove and clear.
  if (typeof localStorage === "undefined") {
    const entries = new Map<string, string>();
    const storage: Storage = {
      get length() {
        return entries.size;
      },
      key: (index) => [...entries.keys()][index] ?? null,
      getItem: (key) => (entries.has(key) ? entries.get(key)! : null),
      setItem: (key, value) => void entries.set(String(key), String(value)),
      removeItem: (key) => void entries.delete(key),
      clear: () => entries.clear(),
    };
    Object.defineProperty(globalThis, "localStorage", { value: storage, configurable: true });
  }

  // The console asks this to decide whether its menu button folds a column or
  // slides a drawer. jsdom has no viewport, so the answer is "desktop" unless a
  // test says otherwise.
  if (!window.matchMedia) {
    window.matchMedia = (query: string) =>
      ({
        matches: true,
        media: query,
        onchange: null,
        addListener: () => {},
        removeListener: () => {},
        addEventListener: () => {},
        removeEventListener: () => {},
        dispatchEvent: () => false,
      }) as MediaQueryList;
  }

  // jsdom has no scrolling, so this is absent rather than a no-op. Anything
  // that keeps a list pinned to its newest row — the assistant's transcript —
  // calls it during an effect and takes the render down with it.
  if (!Element.prototype.scrollIntoView) {
    Element.prototype.scrollIntoView = () => {};
  }

  // Recharts measures its container before it draws. jsdom reports every
  // element as 0×0 and never observes a resize, so charts render empty — which
  // is what a test asserting on the numbers beside them wants.
  if (!globalThis.ResizeObserver) {
    globalThis.ResizeObserver = class {
      observe() {}
      unobserve() {}
      disconnect() {}
    };
  }
}

// Each test renders into a fresh document. Without this a query that should
// find one button finds the previous test's as well.
afterEach(() => {
  if (!hasDocument) return;
  cleanup();
  localStorage.clear();
});

// Nothing in this suite talks to a network. A component that reaches one is a
// bug in the test, and this makes it say so rather than hang; a test that means
// to exercise the client stubs `fetch` itself with vi.stubGlobal.
globalThis.fetch = vi.fn(() => {
  throw new Error("a test reached the network: stub fetch, or mock the module that calls it");
}) as unknown as typeof fetch;
