/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Error tracking through the Sentry protocol, which GlitchTip speaks.
 */

package telemetry

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
	"github.com/getsentry/sentry-go"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel/trace"
)

// errorFlushTimeout bounds how long shutdown waits for queued events. Events are
// sent from a background goroutine; without a flush, the panics that happened in
// the last few seconds of a process's life — which are the interesting ones —
// leave with it.
const errorFlushTimeout = 2 * time.Second

// errorTrackingOn is set once, at startup. Read on the panic path, where asking
// the environment again would be a syscall inside a recover.
var errorTrackingOn bool

// SetupErrorTracking initialises the Sentry SDK against SENTRY_DSN.
//
// Empty DSN is off, and off is the default. GlitchTip is what this platform is
// documented against — it speaks the same protocol as Sentry with a fraction of
// the moving parts — but nothing here is specific to it; a hosted Sentry DSN
// works unchanged.
func SetupErrorTracking(serviceName, env string) (ShutdownFunc, error) {
	dsn := strings.TrimSpace(os.Getenv("SENTRY_DSN"))
	if dsn == "" {
		slog.Info("error tracking is off; set SENTRY_DSN to enable it")
		return func(context.Context) error { return nil }, nil
	}

	err := sentry.Init(sentry.ClientOptions{
		Dsn:         dsn,
		Environment: env,
		Release:     strings.TrimSpace(os.Getenv("RELEASE_VERSION")),
		// Traces go to Tempo, not here. Sending them twice would double the
		// outbound volume to describe the same requests, and GlitchTip's trace
		// view is not what anybody would open with Grafana beside it.
		EnableTracing: false,
		// The SDK's own default attaches the request body and every header to
		// an event. On this platform that is a session cookie, a bearer token
		// and, on a signing endpoint, a PDF.
		SendDefaultPII: false,
		BeforeSend:     scrubEvent,
	})
	if err != nil {
		return nil, err
	}
	errorTrackingOn = true
	slog.Info("error tracking is on", "service", serviceName, "environment", env)

	return func(context.Context) error {
		sentry.Flush(errorFlushTimeout)
		return nil
	}, nil
}

// scrubEvent removes what must not leave this deployment.
//
// An error tracker is a third-party service that receives a copy of whatever
// was in flight when something broke, and this platform's requests carry
// national identifiers, e-mail addresses, session cookies and signed documents.
// The default SDK behaviour with SendDefaultPII off already omits most of it;
// this closes the rest, and it fails closed — a header not on the allow-list is
// dropped rather than kept.
func scrubEvent(event *sentry.Event, _ *sentry.EventHint) *sentry.Event {
	if event.Request != nil {
		// The query string is where single-use references live —
		// /api/v1/verify/landed?ref=… is a credential in a URL.
		event.Request.QueryString = ""
		event.Request.Cookies = ""
		event.Request.Data = ""
		event.Request.Headers = allowedHeaders(event.Request.Headers)
	}
	// The user is identified by their tenant and nothing else. Sentry groups by
	// user to say "this affected 40 people", and a tenant id answers that
	// without naming anybody.
	if event.User.Email != "" || event.User.Username != "" || event.User.IPAddress != "" {
		event.User = sentry.User{ID: event.User.ID}
	}
	return event
}

// allowedHeaders keeps the three that help and drops everything else —
// including Authorization, Cookie, and any header a proxy adds later.
func allowedHeaders(headers map[string]string) map[string]string {
	const (
		requestID = "X-Request-Id"
		agent     = "User-Agent"
		accept    = "Accept-Language"
	)
	kept := make(map[string]string, 3)
	for name, value := range headers {
		switch http.CanonicalHeaderKey(name) {
		case requestID, agent, accept:
			kept[http.CanonicalHeaderKey(name)] = value
		}
	}
	return kept
}

// RecoveryMiddleware answers 500 and reports the panic.
//
// It replaces chi's Recoverer, which prints a stack trace to stdout and nothing
// else: on a deployment that ships logs to Loki, a panic became a wall of
// unstructured text with no request id attached, and there was no way to ask how
// many people hit it or whether it was new.
//
// The response is deliberately bare. A panic is a bug in this platform, and the
// caller learns nothing from its details that is not also useful to somebody
// probing for them.
func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}
			// http.ErrAbortHandler is the documented way for a handler to give
			// up on a connection. Reporting it as a crash would fill the error
			// tracker with clients that hung up.
			if recovered == http.ErrAbortHandler { //nolint:errorlint // sentinel compared by identity, as net/http does
				panic(recovered)
			}

			ctx := r.Context()
			// The log line first, and unconditionally: it is the record that
			// exists whether or not error tracking is configured.
			slog.ErrorContext(ctx, "panic recovered while serving a request",
				"panic", recovered,
				"method", r.Method,
				"route", routePattern(r),
			)

			if errorTrackingOn {
				reportPanic(ctx, r, recovered)
			}

			// The status may already be written; writing again would only log a
			// superfluous-WriteHeader warning.
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"internal server error"}`))
		}()

		next.ServeHTTP(w, r)
	})
}

// reportPanic sends one event, on a hub of its own.
//
// A cloned hub rather than the global one: scope is per-hub, and two requests
// panicking at once on the shared hub would each see the other's tags.
func reportPanic(ctx context.Context, r *http.Request, recovered any) {
	hub := sentry.CurrentHub().Clone()
	hub.Scope().SetRequest(r)
	hub.Scope().SetTag("route", routePattern(r))

	if id := chimiddleware.GetReqID(ctx); id != "" {
		hub.Scope().SetTag("request_id", id)
	}
	// The tenant, as the "user". It is what turns a list of events into "this
	// affected four organisations" without naming a person.
	if tenantID, err := nexus.WorkspaceID(ctx); err == nil {
		hub.Scope().SetUser(sentry.User{ID: tenantID})
		hub.Scope().SetTag("tenant_id", tenantID)
	}
	// And the trace, so an event opens the request that produced it.
	if span := trace.SpanContextFromContext(ctx); span.IsValid() {
		hub.Scope().SetTag("trace_id", span.TraceID().String())
	}

	hub.RecoverWithContext(ctx, recovered)
}

func routePattern(r *http.Request) string {
	if rctx := chi.RouteContext(r.Context()); rctx != nil && rctx.RoutePattern() != "" {
		return rctx.RoutePattern()
	}
	return "unmatched"
}
