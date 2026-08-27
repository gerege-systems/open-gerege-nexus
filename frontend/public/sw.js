/*
 * Gerege Nexus — service worker.
 *
 * This exists to make the platform installable and to give it something to show
 * when the network is gone. It is deliberately the smallest thing that does
 * that, because a service worker on a platform holding tenant data is a cache
 * sitting in front of somebody else's documents.
 *
 * The rule it is built around: nothing that belongs to a person is ever stored.
 *
 *   /api/*        never touched — not read from the cache, not written to it
 *   navigations   network first, and the offline page only when the network
 *                 failed outright
 *   /_next/static content-addressed by the build, so the same URL is always the
 *                 same bytes and caching it cannot go stale
 *   icons, brand  the same
 *   everything else passes straight through
 *
 * A shared machine is the case that decides this. Two people sign in to the
 * same browser an hour apart; if a single API response were cached, the second
 * could be handed the first one's data by a worker that knows nothing about
 * sessions. So the worker is not told to be careful with API responses — it is
 * told not to look at them.
 */

// Bumping this name is what retires everything cached by the previous worker.
const CACHE = "gerege-nexus-shell-v3";

// The offline page is plain HTML with no bundle behind it. It has to render
// when nothing else can, which rules out anything that needs the app to boot.
const OFFLINE_PAGE = "/offline.html";

const PRECACHE = [OFFLINE_PAGE, "/icons/app-192.png", "/icons/app-512.png"];

self.addEventListener("install", (event) => {
  event.waitUntil(
    caches
      .open(CACHE)
      .then((cache) => cache.addAll(PRECACHE))
      // Take over without waiting for every tab to close. Only immutable,
      // build-addressed assets are cached, so an older page cannot be handed
      // a newer file under the same URL.
      .then(() => self.skipWaiting()),
  );
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((names) =>
        Promise.all(
          names
            .filter((name) => name !== CACHE)
            .map((name) => caches.delete(name)),
        ),
      )
      .then(() => self.clients.claim()),
  );
});

/** Whether a URL holds bytes that are the same for everybody, forever. */
function isImmutableAsset(url) {
  return (
    url.pathname.startsWith("/_next/static/") ||
    url.pathname.startsWith("/icons/") ||
    url.pathname === "/brand.webp"
  );
}

self.addEventListener("fetch", (event) => {
  const request = event.request;

  // A cache keyed only by URL cannot tell one person's POST from another's, and
  // has nothing useful to say about one anyway.
  if (request.method !== "GET") return;

  const url = new URL(request.url);

  // Somebody else's origin is not ours to store or to reason about.
  if (url.origin !== self.location.origin) return;

  // The line this worker exists to hold. Everything under /api carries tenant
  // data, a session, or both.
  if (url.pathname.startsWith("/api/")) return;

  // Serving a stale worker or manifest is how an install gets stuck on a
  // version nobody is running any more.
  if (url.pathname === "/sw.js" || url.pathname === "/manifest.webmanifest")
    return;

  // A request that opts out of the HTTP cache is asking for the network, and a
  // service worker answering it from storage would be overruling that.
  if (request.cache === "no-store" || request.cache === "reload") return;

  if (isImmutableAsset(url)) {
    event.respondWith(
      caches.match(request).then(
        (hit) =>
          hit ||
          fetch(request).then((response) => {
            // Only a complete, successful, same-origin response is worth
            // keeping. A 404 or an opaque redirect cached here would outlive
            // whatever caused it.
            if (response.ok && response.type === "basic") {
              const copy = response.clone();
              caches.open(CACHE).then((cache) => cache.put(request, copy));
            }
            return response;
          }),
      ),
    );
    return;
  }

  // Navigations: always ask the network, because the page that comes back
  // depends on who is signed in. The cache is only ever the answer to "there
  // was no network at all".
  //
  // Two retries before that answer, because "the first fetch failed" and
  // "there is no network" are not the same sentence. A phone waking its radio,
  // a handover between cells, a browser resuming a backgrounded tab — each
  // rejects a request and then works. Without the retries the reader gets a
  // full-page "you are offline" over a link that was fine a second later, and
  // has to press a button to find that out; a real outage costs two extra
  // requests and reads the same.
  //
  // The delays are the second half of that sentence. One attempt 400ms later
  // was still inside the same dead window: a radio coming out of idle takes a
  // second or two, so both requests failed for one cause and the reader was
  // told the network was gone while it was merely waking. Waiting 1.5s for the
  // last attempt costs nothing in a real outage — nobody is watching a page
  // that has already failed twice — and covers the case that actually happens.
  if (request.mode === "navigate") {
    event.respondWith(
      retrying(request, [400, 1500]).catch(() =>
        caches.match(OFFLINE_PAGE).then((page) => page || Response.error()),
      ),
    );
  }
});

/** Fetch, and after each delay in turn try again. Rejects only if all of them do. */
function retrying(request, delays) {
  return delays.reduce(
    (attempt, delay) =>
      attempt.catch(() =>
        new Promise((resolve) => setTimeout(resolve, delay)).then(() =>
          fetch(request),
        ),
      ),
    fetch(request),
  );
}
