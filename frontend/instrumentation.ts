/*
 * Server-side error reporting for the Next.js process.
 *
 * This is the rendering server, not the API — the Go backend has its own
 * reporting (backend/internal/operator/observability/errortracking.go). What
 * lands here is a render that threw, which is otherwise visible only as a 500
 * in a container log.
 *
 * Off unless SENTRY_DSN is set. Note the name: this one is the server's, read
 * at runtime, and is not the NEXT_PUBLIC_ one the browser bundle carries.
 */

export async function register() {
  if (!process.env.SENTRY_DSN) {
    return;
  }

  // Imported inside the guard so a deployment without a DSN does not load the
  // SDK at all, and only for the Node runtime — the edge runtime is not used by
  // this application.
  if (process.env.NEXT_RUNTIME === 'nodejs') {
    const Sentry = await import('@sentry/nextjs');
    Sentry.init({
      dsn: process.env.SENTRY_DSN,
      environment: process.env.ENVIRONMENT ?? 'development',
      release: process.env.RELEASE_VERSION,
      tracesSampleRate: 0,
      sendDefaultPii: false,
    });
  }
}

export async function onRequestError(
  ...args: Parameters<typeof import('@sentry/nextjs').captureRequestError>
) {
  if (!process.env.SENTRY_DSN) {
    return;
  }
  const Sentry = await import('@sentry/nextjs');
  Sentry.captureRequestError(...args);
}
