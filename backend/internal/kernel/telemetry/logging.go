/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Putting the request back into the log line.
 */

package telemetry

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel/trace"
)

// ContextHandler adds request_id and tenant_id to every record written while a
// request is being served.
//
// Without it, a log line in Loki can be searched by its text and by nothing
// else: the four lines a failing request produced are scattered among the
// thousand lines the other requests produced at the same moment, and nothing
// says which organisation any of them belonged to. Passing the two through by
// hand would have meant editing several hundred slog calls and would have been
// missed by the next one written.
//
// The attributes are read from the context slog already carries. Neither is put
// on the record when it is absent, which is every log line outside a request —
// startup, the housekeeping sweeps, the shutdown.
type ContextHandler struct {
	slog.Handler
}

// NewContextHandler wraps an existing handler.
func NewContextHandler(inner slog.Handler) *ContextHandler {
	return &ContextHandler{Handler: inner}
}

func (h *ContextHandler) Handle(ctx context.Context, record slog.Record) error {
	if id := chimiddleware.GetReqID(ctx); id != "" {
		record.AddAttrs(slog.String("request_id", id))
	}
	if tenantID, err := nexus.WorkspaceID(ctx); err == nil {
		record.AddAttrs(slog.String("tenant_id", tenantID))
	}
	// The trace, when there is one. This is the join between the two halves of
	// an investigation: Grafana's Loki datasource is configured with a derived
	// field on `trace_id`, so the log line grows a button that opens the trace
	// it belongs to, and Tempo's view links back to the logs.
	//
	// Deliberately not the otelslog bridge. That is a handler which *ships logs
	// over OTLP* — a second delivery path for something Alloy already carries
	// to Loki, and one that would put log delivery on the tracing endpoint's
	// availability. Two attributes on a record is the part that was wanted.
	if span := trace.SpanContextFromContext(ctx); span.IsValid() {
		record.AddAttrs(
			slog.String("trace_id", span.TraceID().String()),
			slog.String("span_id", span.SpanID().String()),
		)
	}
	return h.Handler.Handle(ctx, record)
}

// WithAttrs and WithGroup have to return a *ContextHandler too. Promoting the
// embedded handler's versions would hand back the inner handler and drop the
// context attributes for every logger derived with slog.With — which is most of
// them.
func (h *ContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ContextHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *ContextHandler) WithGroup(name string) slog.Handler {
	return &ContextHandler{Handler: h.Handler.WithGroup(name)}
}

// SetupLogging installs the process-wide logger and returns it.
//
// JSON in every environment, which is what this process has always emitted and
// what Alloy ships to Loki without a parsing stage. A text handler for local
// work was considered and rejected: the one thing worse than JSON in a terminal
// is a production incident investigated against a log format nobody has seen
// since development.
func SetupLogging(level slog.Level) *slog.Logger {
	handler := NewContextHandler(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}

// RequestLogger writes one structured line per request.
//
// It replaces chi's middleware.Logger, which wrote a coloured human line
// straight to stdout: a second, unparseable format in the middle of a JSON
// stream, carrying no request id, no tenant and no route pattern, and printing
// the raw path — which for a request that includes a token in the query is a
// credential written to the log.
//
// The route pattern is logged rather than the URL, for the same reason the
// metrics middleware labels by it.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(ww, r)

		status := ww.Status()
		if status == 0 {
			status = http.StatusOK
		}
		pattern := "unmatched"
		if rctx := chi.RouteContext(r.Context()); rctx != nil && rctx.RoutePattern() != "" {
			pattern = rctx.RoutePattern()
		}

		level := slog.LevelInfo
		if status >= http.StatusInternalServerError {
			level = slog.LevelError
		}
		// r.Context() rather than context.Background(): that is what carries
		// the request id and the tenant for ContextHandler to pick up.
		slog.Log(r.Context(), level, "http_request",
			"method", r.Method,
			"route", pattern,
			"status", status,
			"duration_ms", time.Since(start).Milliseconds(),
			"bytes", ww.BytesWritten(),
		)
	})
}
