/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * The OpenTelemetry metric pipeline, served through the same /metrics endpoint.
 */

package telemetry

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/config"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/otlptranslator"
	"go.opentelemetry.io/otel"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
)

// meter is what every instrument in this package is built from.
//
// It goes through the *global* provider, which delegates: an instrument created
// here at package initialisation keeps working when SetupMetrics installs the
// real provider later, and records nothing at all if it never does. That is
// what lets the instruments in metrics.go, business.go, external.go and
// resilience.go stay package variables — no call site has to ask whether
// metrics are on, and no start-up ordering constraint exists to get wrong.
var meter = otel.Meter(TracerName)

// SetupMetrics installs the OpenTelemetry meter provider behind /metrics.
//
// Unlike tracing, this is unconditional. There is no endpoint to configure and
// nothing leaves the process: the reader *is* the Prometheus exporter, which
// registers itself into client_golang's default registry, which is what
// MetricsHandler already serves. Prometheus scrapes the same URL it always did
// and finds the same series plus `target_info`.
//
// Sharing the default registry rather than taking a private one is deliberate.
// client_golang's own `go_*` and `process_*` collectors register themselves
// there from their init, every dashboard reads them, and a second registry
// would have meant either losing them or serving two endpoints.
func SetupMetrics(serviceName, env string) (ShutdownFunc, error) {
	exporter, err := otelprom.New(
		otelprom.WithRegisterer(prometheus.DefaultRegisterer),
		// Named explicitly rather than left to the default, which the exporter
		// derives from a package-level variable in prometheus/common and which
		// its own documentation says is going to change. The strategy decides
		// whether `http.server.request.duration` in seconds is exported as
		// `http_server_request_duration_seconds` or as something else, so a
		// silent change of default here would rename every series on this
		// platform during a routine dependency bump.
		otelprom.WithTranslationStrategy(otlptranslator.UnderscoreEscapingWithSuffixes),
		// One process, one instrumentation scope. The three otel_scope_* labels
		// the exporter adds by default would then be the same 60-character
		// import path and two empty strings on every series of every scrape,
		// carrying no information and making the label browser in Grafana
		// harder to read. The spec allows this to be turned off; the names and
		// attributes, which are what "OpenTelemetry" means for a consumer, are
		// untouched by it. If a module ever brings a meter of its own, this is
		// the line to reconsider.
		otelprom.WithoutScopeInfo(),
	)
	if err != nil {
		return nil, fmt.Errorf("metrics: could not build the Prometheus exporter: %w", err)
	}

	res, err := describeService(serviceName, env)
	if err != nil {
		return nil, err
	}

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(exporter),
		// The SDK's default histogram boundaries stop at 10s and start at 0,
		// which is the wrong shape for both of the histograms this platform
		// keeps. Views are how OpenTelemetry says "these buckets", and they
		// live here rather than at the instrument so the instrument stays a
		// declaration of what is measured.
		sdkmetric.WithView(
			// The boundaries the HTTP semantic conventions recommend for
			// http.server.request.duration.
			bucketView(metricHTTPServerDuration,
				0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10),
			// Calls to somebody else's system run much longer: eID waits on a
			// citizen reaching for their phone and the HSM client allows
			// ninety seconds, so the default top bucket of 10s would put every
			// interesting call in +Inf.
			bucketView(metricExternalDuration,
				0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 20, 30, 60, 120),
		),
	)

	otel.SetMeterProvider(provider)
	slog.Info("metrics are on", "service", serviceName, "environment", env)

	return func(ctx context.Context) error {
		if err := provider.Shutdown(ctx); err != nil {
			return fmt.Errorf("metrics: shutdown: %w", err)
		}
		return nil
	}, nil
}

// describeService is the OpenTelemetry resource — what these signals are about.
// Shared by tracing and metrics so a span and a sample agree on which service,
// which version and which deployment they came from.
//
// The semconv version has to be the one resource.Default() was built against or
// Merge refuses with "conflicting Schema URL", and the failure is a logged
// error that leaves the process running. When the otel SDK is upgraded, the
// import at the top of this file moves with it.
func describeService(serviceName, env string) (*resource.Resource, error) {
	res, err := resource.Merge(resource.Default(), resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(serviceName),
		semconv.ServiceVersion(config.PlatformVersion),
		semconv.DeploymentEnvironmentNameKey.String(env),
	))
	if err != nil {
		return nil, fmt.Errorf("telemetry: could not describe this service: %w", err)
	}
	return res, nil
}

func bucketView(instrument string, boundaries ...float64) sdkmetric.View {
	return sdkmetric.NewView(
		sdkmetric.Instrument{Name: instrument},
		sdkmetric.Stream{
			Aggregation: sdkmetric.AggregationExplicitBucketHistogram{
				Boundaries: boundaries,
				NoMinMax:   false,
			},
		},
	)
}

// Instrument constructors that refuse to fail quietly.
//
// The only way these return an error is an invalid instrument name, which is a
// typo in this package and not a condition a deployment can be in. They are
// called from package initialisation, so a panic here is a compile-adjacent
// failure: the first test that imports telemetry catches it, and it can never
// reach a server that is serving requests.

func mustCounter(name, unit, help string) metric.Int64Counter {
	instrument, err := meter.Int64Counter(name,
		metric.WithUnit(unit), metric.WithDescription(help))
	if err != nil {
		panic("telemetry: invalid counter " + name + ": " + err.Error())
	}
	return instrument
}

func mustUpDownCounter(name, unit, help string) metric.Int64UpDownCounter {
	instrument, err := meter.Int64UpDownCounter(name,
		metric.WithUnit(unit), metric.WithDescription(help))
	if err != nil {
		panic("telemetry: invalid up-down counter " + name + ": " + err.Error())
	}
	return instrument
}

func mustHistogram(name, unit, help string) metric.Float64Histogram {
	instrument, err := meter.Float64Histogram(name,
		metric.WithUnit(unit), metric.WithDescription(help))
	if err != nil {
		panic("telemetry: invalid histogram " + name + ": " + err.Error())
	}
	return instrument
}
