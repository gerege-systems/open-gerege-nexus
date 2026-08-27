#!/usr/bin/env node
/**
 * Holds the one rule the service worker exists to keep.
 *
 *   npm run pwa:check
 *
 * A service worker sits in front of every request the app makes, and this
 * platform's requests carry tenant data. The worker is written so that nothing
 * belonging to a person is ever stored: /api is not read from the cache and not
 * written to it, and only build-addressed assets are kept.
 *
 * That rule lives in a handful of early returns, which is exactly the kind of
 * code a later change walks through without noticing. The failure would be
 * silent and would look like a performance improvement: two people sign in to
 * the same browser an hour apart, and the second is handed the first one's
 * documents by a worker that knows nothing about sessions.
 *
 * So the worker's decisions are asserted rather than trusted. The real file is
 * loaded with stubbed service-worker globals and asked, for each shape of
 * request, whether it takes the request over or lets it through.
 *
 * The second half asks a different question of the same file: how hard it tries
 * before it gives up. A navigation that fails once is usually a radio waking,
 * not an outage, and answering it with a full-page "you are offline" is a lie
 * the reader has to press a button to disprove.
 *
 * Exits non-zero on any wrong answer.
 */
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";
import vm from "node:vm";

const here = dirname(fileURLToPath(import.meta.url));
const workerPath = join(here, "..", "public", "sw.js");
const ORIGIN = "https://nexus.gerege.mn";

const source = readFileSync(workerPath, "utf8");
const OFFLINE = { offlinePage: true };

/**
 * Loads the real worker with stubbed globals and hands back what it registered.
 *
 * fetchImpl decides what the network does; waits collects every delay the
 * worker asked for and grants it immediately, so a check that is about how many
 * times it tries does not also take as long as the waiting.
 */
function loadWorker(fetchImpl = async () => ({ ok: true, type: "basic", clone: () => ({}) })) {
  const listeners = {};
  const waits = [];
  const sandbox = {
    self: {
      addEventListener: (name, fn) => (listeners[name] = fn),
      skipWaiting: async () => {},
      clients: { claim: async () => {} },
      location: { origin: ORIGIN },
    },
    // Enough of the Cache API to load; the routing assertions are about which
    // requests are taken over, not about what ends up stored.
    caches: {
      open: async () => ({
        addAll: async () => {},
        put: async () => {},
        match: async () => undefined,
      }),
      keys: async () => [],
      delete: async () => true,
      match: async (key) => (key === "/offline.html" ? OFFLINE : undefined),
    },
    fetch: fetchImpl,
    setTimeout: (fn, ms) => {
      waits.push(ms);
      fn();
    },
    Response: { error: () => ({}) },
    URL,
    Promise,
    console,
  };
  vm.createContext(sandbox);
  vm.runInContext(source, sandbox);

  if (typeof listeners.fetch !== "function") {
    console.error("sw.js registered no fetch handler — nothing to check");
    process.exit(1);
  }
  return { listeners, waits };
}

const { listeners } = loadWorker();

/** Whether the worker took this request over. */
function handles(url, { mode = "no-cors", method = "GET", cache = "default" } = {}) {
  let taken = false;
  listeners.fetch({
    request: { url, method, mode, cache },
    respondWith: () => {
      taken = true;
    },
  });
  return taken;
}

const cases = [
  // The rule. Every shape an API call can arrive in has to pass through.
  ["an API read", `${ORIGIN}/api/v1/contacts/`, {}, false],
  ["an API read with a query", `${ORIGIN}/api/v1/documents?tenant=x`, {}, false],
  ["an API call shaped like a navigation", `${ORIGIN}/api/v1/auth/me`, { mode: "navigate" }, false],
  ["a write of any kind", `${ORIGIN}/contacts`, { method: "POST" }, false],
  ["somebody else's origin", "https://sso.gerege.mn/x", {}, false],
  // Serving either of these from storage is how an install gets stuck on a
  // version nobody is running.
  ["the worker itself", `${ORIGIN}/sw.js`, {}, false],
  ["the manifest", `${ORIGIN}/manifest.webmanifest`, {}, false],
  // A caller that opted out of the HTTP cache is asking for the network.
  ["a no-store request", `${ORIGIN}/_next/static/chunks/a.js`, { cache: "no-store" }, false],

  // What the worker is for.
  ["a build asset", `${ORIGIN}/_next/static/chunks/a.js`, {}, true],
  ["an app icon", `${ORIGIN}/icons/app-192.png`, {}, true],
  ["the brand mark", `${ORIGIN}/brand.webp`, {}, true],
  ["a page navigation", `${ORIGIN}/apps`, { mode: "navigate" }, true],
];

let wrong = 0;
for (const [name, url, options, expected] of cases) {
  const actual = handles(url, options);
  if (actual !== expected) {
    wrong += 1;
    console.error(
      `  ${name}: the worker ${actual ? "took it over" : "let it through"}, expected the opposite`,
    );
  }
}

if (wrong > 0) {
  console.error(`\n${wrong} service-worker routing decision(s) wrong — see public/sw.js`);
  process.exit(1);
}

// How a navigation ends, by how many times the network refuses it.
//
// The failure this catches is a change that drops back to one attempt: every
// assertion above still passes, and the only symptom is somebody on a phone
// being told the platform is unreachable while it is answering everybody else.
/** Drives one navigation and reports the response and the delays it waited. */
async function navigate(failures) {
  let attempts = 0;
  const { listeners: worker, waits } = loadWorker(async () => {
    attempts += 1;
    if (attempts <= failures) throw new Error("network refused");
    return { ok: true, type: "basic", served: true, clone: () => ({}) };
  });
  let answer;
  worker.fetch({
    request: { url: `${ORIGIN}/apps`, method: "GET", mode: "navigate", cache: "default" },
    respondWith: (promise) => (answer = promise),
  });
  return { response: await answer, attempts, waits };
}

const twoFailures = await navigate(2);
const alwaysFails = await navigate(Infinity);

const retries = [
  ["a navigation that works", (await navigate(0)).attempts === 1],
  ["a navigation that fails twice is still served", twoFailures.response.served === true],
  ["it takes three attempts to give up", alwaysFails.attempts === 3],
  ["giving up shows the offline page", alwaysFails.response === OFFLINE],
  // Both attempts inside the same second is one attempt as far as a radio
  // coming out of idle is concerned.
  ["the last wait outlasts a waking radio", twoFailures.waits.at(-1) >= 1000],
];

const failed = retries.filter(([, ok]) => !ok);
for (const [name] of failed) console.error(`  ${name}: no`);
if (failed.length > 0) {
  console.error(`\n${failed.length} navigation retry rule(s) broken — see public/sw.js`);
  process.exit(1);
}

console.log(
  `service worker: ${cases.length} routing decisions correct, ${retries.length} retry rules held`,
);
