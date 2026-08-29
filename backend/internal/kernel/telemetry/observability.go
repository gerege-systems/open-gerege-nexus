/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * OpenTelemetry tracing: off by default, and genuinely off.
 */

package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

type ShutdownFunc func(context.Context) error

// TracerName is what every span this platform creates is attributed to.
const TracerName = "github.com/gerege-systems/open-gerege-nexus/backend"

// tracer is what the rest of the platform reaches for. It starts as a no-op and
// stays one unless SetupTracing installs a real provider, so a call site does
// not have to ask whether tracing is on before starting a span.
var tracer trace.Tracer = noop.NewTracerProvider().Tracer(TracerName)

// Tracer returns the tracer to start spans with. It is never nil.
func Tracer() trace.Tracer { return tracer }

// TracingEnabled reports whether spans are actually being exported.
func TracingEnabled() bool { return strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")) != "" }

// SetupTracing installs the OTLP exporter, or does nothing at all.
//
// Nothing at all is the default, and it is a real nothing: with
// OTEL_EXPORTER_OTLP_ENDPOINT unset there is no exporter, no batch processor,
// no background goroutine and no sampler decision — every span started through
// Tracer() is a no-op span whose methods compile down to almost nothing. A
// deployment that does not run Tempo pays no measurable cost for the tracing
// code being in the binary, which is the condition for putting it in the
// default path at all.
//
// This replaced a stub that logged "opentelemetry tracing initialized" and
// initialized nothing. That line was worse than no tracing: an operator reading
// the startup log had every reason to believe traces existed somewhere.
func SetupTracing(ctx context.Context, serviceName, env string) (ShutdownFunc, error) {
	endpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	if endpoint == "" {
		slog.Info("tracing is off; set OTEL_EXPORTER_OTLP_ENDPOINT to enable it")
		return func(context.Context) error { return nil }, nil
	}

	exporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("tracing: could not build the OTLP exporter: %w", err)
	}

	// Shared with the metric pipeline, so a span and a sample agree on which
	// service, which version and which deployment they came from — which is
	// what lets Grafana pivot from one to the other. See describeService for
	// the semconv-version trap it is written to avoid.
	res, err := describeService(serviceName, env)
	if err != nil {
		return nil, err
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		// ParentBased so a sampling decision made upstream is honoured: a trace
		// that starts at the frontend and continues here must not be half
		// recorded. The root sampler is the ratio below.
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(samplingRatio()))),
		sdktrace.WithBatcher(exporter,
			// Spans leave in batches, on a timer, from one background
			// goroutine. Exporting per span would put a network round trip on
			// the request path — which is how tracing earns the reputation for
			// costing more than it explains.
			sdktrace.WithBatchTimeout(5*time.Second),
			sdktrace.WithMaxExportBatchSize(512),
		),
	)

	otel.SetTracerProvider(provider)
	// W3C traceparent, not the older B3. It is what otelhttp sends by default
	// and what the browser SDKs emit.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))
	tracer = provider.Tracer(TracerName)

	slog.Info("tracing is on",
		"endpoint", endpoint, "service", serviceName,
		"environment", env, "sampling_ratio", samplingRatio())

	return func(shutdownCtx context.Context) error {
		// Flushes what is still in the batch. Without it, the spans describing
		// a shutdown — including the ones from whatever went wrong just before
		// it — are dropped with the process.
		if err := provider.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("tracing: shutdown: %w", err)
		}
		slog.Info("tracing shut down cleanly")
		return nil
	}, nil
}

// defaultSamplingRatio traces one request in ten.
//
// Everything is the obvious wrong answer at any volume: it is a span per
// database query per request, kept for as long as Tempo's retention. One in ten
// is enough to characterise latency and still catch a recurring slow path,
// which is what traces are read for. A specific slow request that has to be
// found is what the request id in the logs is for.
const defaultSamplingRatio = 0.1

func samplingRatio() float64 {
	raw := strings.TrimSpace(os.Getenv("OTEL_TRACES_SAMPLER_ARG"))
	if raw == "" {
		return defaultSamplingRatio
	}
	ratio, err := strconv.ParseFloat(raw, 64)
	if err != nil || ratio < 0 || ratio > 1 {
		slog.Warn("tracing: OTEL_TRACES_SAMPLER_ARG is not a ratio between 0 and 1; using the default",
			"value", raw, "default", defaultSamplingRatio)
		return defaultSamplingRatio
	}
	return ratio
}

// ExternalSpanAttributes describes one outbound call, for a span.
func ExternalSpanAttributes(system, operation string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("peer.service", ExternalSystem(system)),
		attribute.String("rpc.method", operation),
	}
}

// TracingMiddleware opens a server span per request, continuing whatever trace
// the caller arrived with.
//
// The span is named after the route pattern for the same reason the metrics
// middleware labels by it: a span per distinct URL is unbounded, and Tempo's
// service graph would show one operation per document id.
//
// /health, /ready and /metrics are excluded. They are called every ten to
// fifteen seconds by Docker and by Prometheus, they do nothing worth a trace,
// and at 10% sampling they would still be a steady majority of everything
// stored.
func TracingMiddleware(next http.Handler) http.Handler {
	instrumented := otelhttp.NewHandler(next, "http",
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			if rctx := chi.RouteContext(r.Context()); rctx != nil && rctx.RoutePattern() != "" {
				return r.Method + " " + rctx.RoutePattern()
			}
			return r.Method + " unmatched"
		}),
	)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health", "/ready", "/metrics":
			next.ServeHTTP(w, r)
		default:
			instrumented.ServeHTTP(w, r)
		}
	})
}

// InstrumentPool attaches the pgx tracer, so a slow request shows which query
// it was waiting on rather than an unexplained gap between two spans.
//
// Called on the pool configuration before the pool exists, like dbguard's own
// hook. With tracing off, otelpgx starts no-op spans.
func InstrumentPool(cfg *pgxpool.Config) {
	cfg.ConnConfig.Tracer = otelpgx.NewTracer(
		otelpgx.WithTracerProvider(otel.GetTracerProvider()),
		// The span is named after the statement, trimmed to its first line.
		// Without this every one is called "query", and a trace of eleven
		// database calls is eleven identically named rows.
		otelpgx.WithTrimSQLInSpanName(),
	)

	// Query *parameters* are deliberately left off, which is otelpgx's default
	// and must stay that way: the arguments are the row the query is about — an
	// e-mail address, a national identifier, a password hash on the way in. A
	// span is kept for as long as Tempo's retention and is readable by anyone
	// who can open Grafana, so WithIncludeQueryParameters must never be added
	// here. The SQL text itself carries no values; it is all placeholders.
}
