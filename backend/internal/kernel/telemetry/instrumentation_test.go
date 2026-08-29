package telemetry_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/telemetry"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// TestMain installs the metric pipeline once for the whole package.
//
// Nothing in this package exports anything until it runs: the instruments are
// created at package initialisation through OpenTelemetry's *global* meter
// provider, which is a no-op that delegates, and SetupMetrics is what installs
// the real provider and puts its Prometheus exporter on the default registry.
// Every assertion below gathers from that registry, so without this they would
// all read zero — and would go on passing, because zero is what a counter that
// has never moved also reads.
func TestMain(m *testing.M) {
	shutdown, err := telemetry.SetupMetrics("gerege-nexus-test", "test")
	if err != nil {
		panic("could not set up metrics: " + err.Error())
	}
	code := m.Run()
	_ = shutdown(context.Background())
	os.Exit(code)
}

// The saturation half of the golden signals depends on collectors this package
// does not register itself. If client_golang ever stops registering them by
// default, /metrics goes quiet about the runtime with nothing else saying so.
func TestRuntimeCollectorsAreRegistered(t *testing.T) {
	if !telemetry.RuntimeCollectorsRegistered() {
		t.Fatal("go_goroutines and process_resident_memory_bytes are not on the default registry")
	}
}

// A pool collector with no pool must not panic and must export nothing. A test
// server has no database and still scrapes /metrics.
func TestPoolCollectorWithoutPool(t *testing.T) {
	telemetry.RegisterPoolCollector(nil)
	if findMetric(t, "pgxpool_max_conns", nil) != nil {
		t.Fatal("a nil pool exported pgxpool_max_conns")
	}
}

// The rename. `http_requests_total` and `http_request_duration_seconds` were
// this platform's own names; the HTTP semantic conventions call for one
// histogram, and its `_count` series is the request count. Every alert rule,
// dashboard panel and console query was moved onto these names at the same
// time, so a silent revert here — an exporter default changing the translation
// strategy, say — would blank all three at once with nothing failing.
func TestHTTPMetricFollowsSemanticConventions(t *testing.T) {
	labels := map[string]string{
		"http_request_method":       http.MethodGet,
		"http_route":                "/api/v1/semconv/{id}",
		"http_response_status_code": "204",
	}
	before := histogramCount(t, "http_server_request_duration_seconds", labels)

	router := chi.NewRouter()
	router.Use(telemetry.MetricsMiddleware)
	router.Get("/api/v1/semconv/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	router.ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequest(http.MethodGet, "/api/v1/semconv/abc", nil))

	if after := histogramCount(t, "http_server_request_duration_seconds", labels); after != before+1 {
		t.Fatalf("http_server_request_duration_seconds did not move: %d → %d", before, after)
	}
	if findMetric(t, "http_requests_total", nil) != nil {
		t.Error("the old http_requests_total is still being exported")
	}
}

// One request, one observation — even though two pieces of middleware in the
// chain both know how to record an HTTP server metric.
//
// otelhttp starts recording `http.server.request.duration` of its own accord
// the moment a real meter provider exists, under the same name this package
// uses and with a different label set. Both were live in production for one
// deploy: every request counted twice, `sum(rate(...))` read double, and the
// extra series carried `server.address` — a label taken from the request.
func TestTheHTTPMetricIsRecordedOnce(t *testing.T) {
	labels := map[string]string{
		"http_request_method": http.MethodGet,
		"http_route":          "/api/v1/once/{id}",
	}
	before := histogramCount(t, "http_server_request_duration_seconds", labels)

	// The real chain, in the order pkg/host/server.go installs it: the tracing
	// wrapper outside, the metrics middleware inside.
	router := chi.NewRouter()
	router.Use(telemetry.TracingMiddleware)
	router.Use(telemetry.MetricsMiddleware)
	router.Get("/api/v1/once/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	router.ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequest(http.MethodGet, "/api/v1/once/abc", nil))

	if after := histogramCount(t, "http_server_request_duration_seconds", labels); after != before+1 {
		t.Errorf("one request produced %d observations, want 1", after-before)
	}
	// The label otelhttp adds and this package does not, matched by name with
	// any value. Its presence means the second recorder is back, whatever the
	// counts happen to say.
	if labelExists(t, "http_server_request_duration_seconds", "server_address") {
		t.Error("a series carrying server_address is being exported: otelhttp is recording metrics again")
	}
}

// A method is a token net/http will accept in any shape, so an unrecognised one
// must not become an attribute value: that is an unbounded series count driven
// by whoever is sending the requests.
func TestInventedMethodsDoNotMintSeries(t *testing.T) {
	// Straight at the middleware rather than through chi, because chi refuses
	// to *register* a route for an invented verb — but it still runs the
	// middleware chain for a request that arrives with one, on its way to the
	// 405. That is the path this guards.
	handler := telemetry.MetricsMiddleware(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusMethodNotAllowed) }))
	handler.ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequest("XYZZY", "/api/v1/invented", nil))

	if findMetric(t, "http_server_request_duration_seconds",
		map[string]string{"http_request_method": "XYZZY"}) != nil {
		t.Fatal("an invented HTTP method reached the exposition as its own series")
	}
	if findMetric(t, "http_server_request_duration_seconds",
		map[string]string{"http_request_method": "_OTHER"}) == nil {
		t.Fatal("the invented method was not folded into _OTHER")
	}
}

func TestObserveExternalRecordsOutcome(t *testing.T) {
	before := histogramCount(t, "external_request_duration_seconds", map[string]string{
		"system": telemetry.SystemEID, "operation": "unit_test", "status": "error",
	})

	err := telemetry.ObserveExternal(context.Background(), telemetry.SystemEID, "unit_test",
		func(context.Context) error { return errors.New("upstream refused") })
	if err == nil {
		t.Fatal("expected the call's error to be returned unchanged")
	}

	after := histogramCount(t, "external_request_duration_seconds", map[string]string{
		"system": telemetry.SystemEID, "operation": "unit_test", "status": "error",
	})
	if after != before+1 {
		t.Fatalf("expected one more observation, went from %d to %d", before, after)
	}
}

// An unrecognised system name must not mint a series of its own: `system` is a
// label, and a call site added without a constant would otherwise widen it.
func TestUnknownExternalSystemIsFoldedIntoOther(t *testing.T) {
	if got := telemetry.ExternalSystem("some-new-gateway.example.mn"); got != "other" {
		t.Fatalf("expected \"other\", got %q", got)
	}
	if got := telemetry.ExternalSystem(telemetry.SystemXYP); got != telemetry.SystemXYP {
		t.Fatalf("a known system must pass through, got %q", got)
	}
}

func TestBusinessCountersIncrement(t *testing.T) {
	before := counterValue(t, "logins_total", map[string]string{
		"method": telemetry.LoginPassword, "result": telemetry.ResultFailure,
	})
	telemetry.RecordLogin(telemetry.LoginPassword, false)
	after := counterValue(t, "logins_total", map[string]string{
		"method": telemetry.LoginPassword, "result": telemetry.ResultFailure,
	})
	if after != before+1 {
		t.Fatalf("logins_total did not move: %v → %v", before, after)
	}

	shedBefore := counterValue(t, "resilience_load_shed_total", nil)
	telemetry.RecordLoadShed()
	if got := counterValue(t, "resilience_load_shed_total", nil); got != shedBefore+1 {
		t.Fatalf("resilience_load_shed_total did not move: %v → %v", shedBefore, got)
	}
}

// The whole point of the handler: a log line written while serving a request
// carries the request and the organisation it was for.
func TestContextHandlerAddsRequestAndTenant(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(telemetry.NewContextHandler(
		slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})))

	ctx := context.WithValue(context.Background(), chimiddleware.RequestIDKey, "req-42")
	ctx = nexus.WithWorkspaceID(ctx, "11111111-1111-1111-1111-111111111111")

	logger.InfoContext(ctx, "something happened")

	line := buf.String()
	if !strings.Contains(line, `"request_id":"req-42"`) {
		t.Errorf("request_id missing from %s", line)
	}
	if !strings.Contains(line, `"tenant_id":"11111111-1111-1111-1111-111111111111"`) {
		t.Errorf("tenant_id missing from %s", line)
	}
}

// slog.With must not lose the context attributes. Promoting the embedded
// handler's WithAttrs would have returned the inner handler and silently
// dropped them for every derived logger.
func TestContextHandlerSurvivesWith(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(telemetry.NewContextHandler(
		slog.NewJSONHandler(&buf, nil))).With("component", "test")

	ctx := context.WithValue(context.Background(), chimiddleware.RequestIDKey, "req-7")
	logger.InfoContext(ctx, "derived")

	if !strings.Contains(buf.String(), `"request_id":"req-7"`) {
		t.Errorf("request_id lost by slog.With: %s", buf.String())
	}
}

// Outside a request there is neither, and neither must be invented.
func TestContextHandlerOmitsWhatIsAbsent(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(telemetry.NewContextHandler(slog.NewJSONHandler(&buf, nil)))
	logger.Info("startup")

	if strings.Contains(buf.String(), "request_id") || strings.Contains(buf.String(), "tenant_id") {
		t.Errorf("attributes invented outside a request: %s", buf.String())
	}
}

// The access log names the route pattern, never the URL: a path can carry a
// single-use token, and a log line is a place a credential must not land.
func TestRequestLoggerLogsRoutePatternNotURL(t *testing.T) {
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(telemetry.NewContextHandler(slog.NewJSONHandler(&buf, nil))))
	defer slog.SetDefault(previous)

	router := chi.NewRouter()
	router.Use(chimiddleware.RequestID)
	router.Use(telemetry.RequestLogger)
	router.Get("/api/v1/verify/{ref}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/verify/secret-token-value", nil))

	line := buf.String()
	if !strings.Contains(line, `"route":"/api/v1/verify/{ref}"`) {
		t.Errorf("route pattern missing from %s", line)
	}
	if strings.Contains(line, "secret-token-value") {
		t.Errorf("the raw path leaked into the access log: %s", line)
	}
	if !strings.Contains(line, `"status":204`) {
		t.Errorf("status missing from %s", line)
	}
}

// helpers

func findMetric(t *testing.T, name string, labels map[string]string) *dto.Metric {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.GetMetric() {
			if matchesLabels(metric, labels) {
				return metric
			}
		}
	}
	return nil
}

// labelExists reports whether any series of a family carries a label with this
// name, whatever its value.
func labelExists(t *testing.T, name, label string) bool {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.GetMetric() {
			for _, pair := range metric.GetLabel() {
				if pair.GetName() == label {
					return true
				}
			}
		}
	}
	return false
}

func matchesLabels(metric *dto.Metric, labels map[string]string) bool {
	for key, want := range labels {
		found := false
		for _, pair := range metric.GetLabel() {
			if pair.GetName() == key && pair.GetValue() == want {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func histogramCount(t *testing.T, name string, labels map[string]string) uint64 {
	t.Helper()
	metric := findMetric(t, name, labels)
	if metric == nil {
		return 0
	}
	return metric.GetHistogram().GetSampleCount()
}

func counterValue(t *testing.T, name string, labels map[string]string) float64 {
	t.Helper()
	metric := findMetric(t, name, labels)
	if metric == nil {
		return 0
	}
	return metric.GetCounter().GetValue()
}
