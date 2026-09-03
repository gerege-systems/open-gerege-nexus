/*
 * Browser-side error reporting.
 *
 * Off unless NEXT_PUBLIC_SENTRY_DSN is set, which is the default. The DSN is
 * public by design — it identifies the project to write to and grants nothing
 * else.
 *
 * The SDK is fetched only when there is one to report to. `import * as Sentry`
 * at the top of this file put the whole SDK — 246 KB, 77 KB over the wire —
 * into the first chunk of every page, and nothing has ever passed the build
 * argument that switches it on (Dockerfile `ARG NEXT_PUBLIC_SENTRY_DSN`, unset
 * in every workflow), so on nexus.gerege.mn each visitor downloaded an error
 * reporter that initialises nothing and sends nothing before the page could
 * paint. The value is inlined at build time, so a build without a DSN drops the
 * import as dead code rather than deferring it, and a build with one loads the
 * SDK beside the page instead of in front of it.
 *
 * What that costs when it is on: an error thrown in the moment between the page
 * rendering and the SDK arriving goes unreported. That window existed before —
 * `Sentry.init` ran after the bundle had parsed — this widens it by one fetch,
 * which is the price of the page appearing at all on a slow connection.
 *
 * What is deliberately NOT enabled: Session Replay. It records the DOM of what
 * the person was looking at, and on this platform that is a citizen's
 * registration number, an invoice, or the body of a document waiting for a
 * signature. No amount of masking makes shipping that to an error tracker the
 * right default.
 */

import type * as Sentry from '@sentry/nextjs';

const dsn = process.env.NEXT_PUBLIC_SENTRY_DSN;

// Filled in when the SDK lands; until then, and forever in a build with no DSN,
// the router hook below is a no-op.
let capture: typeof Sentry.captureRouterTransitionStart | undefined;

if (dsn) {
  void import('@sentry/nextjs').then((SDK) => {
    SDK.init({
      dsn,
      environment: process.env.NEXT_PUBLIC_ENVIRONMENT ?? 'development',
      release: process.env.NEXT_PUBLIC_RELEASE_VERSION,

      // Errors only. Tracing from the browser would double-report what Tempo
      // already receives from the API, from a source that cannot be trusted
      // about its own timings.
      tracesSampleRate: 0,

      // The SDK's default attaches the request body, the URL and the headers of
      // every fetch that failed. On this platform a URL can carry a single-use
      // verification reference and a body can carry a national identifier.
      sendDefaultPii: false,

      beforeSend(event) {
        if (event.request) {
          // The query string is where single-use references live.
          delete event.request.query_string;
          delete event.request.cookies;
          delete event.request.data;
          if (event.request.headers) {
            delete event.request.headers.Authorization;
            delete event.request.headers.Cookie;
          }
        }
        // Never a person. The tenant is enough to answer "how many
        // organisations does this affect", which is what the grouping is read
        // for.
        if (event.user) {
          event.user = { id: event.user.id };
        }
        return event;
      },

      // Errors the platform did not cause and cannot fix: a browser extension
      // injecting a script, a network that dropped a request mid-flight, the
      // ResizeObserver notice every Chrome build emits. Left in, they are the
      // majority of what an error tracker shows and the reason people stop
      // reading it.
      ignoreErrors: [
        'ResizeObserver loop limit exceeded',
        'ResizeObserver loop completed with undelivered notifications',
        'Non-Error promise rejection captured',
        'NetworkError when attempting to fetch resource',
        'Failed to fetch',
        'AbortError',
      ],
      denyUrls: [/extensions\//i, /^chrome:\/\//i, /^moz-extension:\/\//i],
    });
    capture = SDK.captureRouterTransitionStart;
  });
}

// Next asks for this so it can report navigation timing to the SDK. Exported
// synchronously, because Next reads the export as the module loads; it forwards
// to the SDK once that has arrived, and does nothing in a build with no DSN.
export const onRouterTransitionStart: typeof Sentry.captureRouterTransitionStart = (
  ...args
) => capture?.(...args);
