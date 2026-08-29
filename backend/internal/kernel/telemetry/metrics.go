/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * The inbound HTTP metric, by the OpenTelemetry HTTP semantic conventions.
 */

package telemetry

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
)

// metricHTTPServerDuration is the name the HTTP semantic conventions give the
// one metric a server has to have. It is exported as
// `http_server_request_duration_seconds`.
//
// There is deliberately no separate request *counter*. The histogram's `_count`
// series is the request count — same numbers, one instrument, and it is what
// the conventions say. `http_requests_total` was this platform's own invention
// and it is gone; the queries that read it now read
// `http_server_request_duration_seconds_count`.
const metricHTTPServerDuration = "http.server.request.duration"

var httpServerDuration = mustHistogram(metricHTTPServerDuration, "s",
	"Duration of inbound HTTP requests")

// knownMethods is what may become an `http.request.method` attribute.
//
// Anything else is folded into `_OTHER`, which is what the conventions ask for
// and, here, closes a hole: net/http accepts any token as a method, so before
// this a client could mint an unbounded number of time series by sending
// requests with invented verbs — the same unbounded-cardinality problem the
// route attribute below was already guarded against, from the same direction.
var knownMethods = map[string]bool{
	http.MethodGet: true, http.MethodHead: true, http.MethodPost: true,
	http.MethodPut: true, http.MethodPatch: true, http.MethodDelete: true,
	http.MethodConnect: true, http.MethodOptions: true, http.MethodTrace: true,
}

func requestMethod(method string) attribute.KeyValue {
	if knownMethods[method] {
		return semconv.HTTPRequestMethodKey.String(method)
	}
	return semconv.HTTPRequestMethodOther
}

// MetricsMiddleware records one observation per request.
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(ww, r)

		duration := time.Since(start).Seconds()
		status := ww.Status()
		if status == 0 {
			status = http.StatusOK
		}

		// Only routed patterns may become attribute values. Falling back to
		// r.URL.Path meant every 404 for a random URL minted a new time series
		// — unbounded cardinality that an unauthenticated client could drive
		// until the process ran out of memory.
		route := "unmatched"
		if rctx := chi.RouteContext(r.Context()); rctx != nil && rctx.RoutePattern() != "" {
			route = rctx.RoutePattern()
		}

		// The request's context, not context.Background(), and that is the
		// whole of what makes exemplars work: the SDK's default exemplar filter
		// keeps a sample only when the context it was recorded with is inside a
		// sampled span. That is how the slow point on a latency graph becomes a
		// link to the trace that made it slow.
		httpServerDuration.Record(r.Context(), duration, metric.WithAttributes(
			requestMethod(r.Method),
			semconv.HTTPRoute(route),
			semconv.HTTPResponseStatusCode(status),
		))
	})
}

// MetricsHandler serves the exposition, OpenTelemetry's series and
// client_golang's `go_*`/`process_*` collectors together.
//
// OpenMetrics is enabled because exemplars are only ever sent in that format.
// Without it the trace ids attached to the histogram above are collected,
// stored in memory and then dropped at the point of being written out — the
// exemplarTraceIdDestinations in the Grafana datasource would go on pointing at
// nothing, which is exactly how they behaved before. Prometheus asks for
// OpenMetrics when it is started with `--enable-feature=exemplar-storage`, and
// gets the ordinary text format otherwise; nothing breaks either way.
func MetricsHandler() http.Handler {
	return promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
}
