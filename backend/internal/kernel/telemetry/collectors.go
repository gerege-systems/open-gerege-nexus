/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Saturation collectors: the Go runtime, the process, and the database pool.
 */

package telemetry

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/metric"
)

// RuntimeCollectorsRegistered reports whether the default registry is carrying
// the Go runtime and process collectors.
//
// client_golang registers both into prometheus.DefaultRegisterer from its own
// init, so there is nothing to add here — and adding a second one would panic
// with AlreadyRegisteredError at startup. This exists so the assumption is
// checked rather than believed: the OpenTelemetry exporter shares that same
// registry (see SetupMetrics), and a future move to a private one would take
// `go_goroutines` and `process_resident_memory_bytes` off /metrics with nothing
// saying so — the saturation half of the golden signals would quietly go blank.
func RuntimeCollectorsRegistered() bool {
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		return false
	}
	var goSeen, processSeen bool
	for _, family := range families {
		switch family.GetName() {
		case "go_goroutines":
			goSeen = true
		case "process_resident_memory_bytes":
			processSeen = true
		}
	}
	return goSeen && processSeen
}

// RegisterPoolCollector exports pgxpool.Stat.
//
// Observable instruments, read at collection time. The proposal asked for a
// sampler on a 15-second ticker; this is the same numbers without a goroutine,
// without a sampling lag between the spike and the scrape that shows it, and
// without a value that keeps being served after the pool has been closed.
// pgxpool.Stat is a cheap snapshot of counters the pool already maintains, so
// there is nothing to amortise by sampling less often than Prometheus asks.
//
// A nil pool registers nothing rather than panicking: a test server may have no
// database at all. Registering twice is reported rather than fatal — the
// process has one pool and calls this once, a second call is a wiring mistake
// in a test, and taking the server down over a metric would be a worse outcome
// than a log line.
//
// The names are unchanged from the hand-written collector this replaced: the
// exporter turns `pgxpool.acquired_conns` into `pgxpool_acquired_conns`, and
// appends `_total` to the three counters exactly as before.
func RegisterPoolCollector(pool *pgxpool.Pool) {
	if pool == nil {
		return
	}

	gauge := func(name, help string) metric.Int64ObservableGauge {
		instrument, err := meter.Int64ObservableGauge(name, metric.WithDescription(help))
		if err != nil {
			panic("telemetry: invalid pool gauge " + name + ": " + err.Error())
		}
		return instrument
	}

	acquired := gauge("pgxpool.acquired_conns", "Connections currently held by a caller")
	idle := gauge("pgxpool.idle_conns", "Connections open and waiting to be handed out")
	total := gauge("pgxpool.total_conns", "Connections the pool is holding, idle and acquired together")
	maxConns := gauge("pgxpool.max_conns", "Ceiling the pool was configured with")

	empty, err := meter.Int64ObservableCounter("pgxpool.empty_acquire",
		metric.WithDescription("Acquisitions that had to wait because the pool was empty"))
	if err != nil {
		panic("telemetry: invalid pool counter: " + err.Error())
	}
	canceled, err := meter.Int64ObservableCounter("pgxpool.canceled_acquire",
		metric.WithDescription("Acquisitions abandoned because the caller's context ended first"))
	if err != nil {
		panic("telemetry: invalid pool counter: " + err.Error())
	}
	waited, err := meter.Float64ObservableCounter("pgxpool.acquire.duration",
		metric.WithUnit("s"),
		metric.WithDescription("Time callers have spent waiting for a connection"))
	if err != nil {
		panic("telemetry: invalid pool counter: " + err.Error())
	}

	_, err = meter.RegisterCallback(func(_ context.Context, observer metric.Observer) error {
		stat := pool.Stat()
		observer.ObserveInt64(acquired, int64(stat.AcquiredConns()))
		observer.ObserveInt64(idle, int64(stat.IdleConns()))
		observer.ObserveInt64(total, int64(stat.TotalConns()))
		observer.ObserveInt64(maxConns, int64(stat.MaxConns()))
		observer.ObserveInt64(empty, stat.EmptyAcquireCount())
		observer.ObserveInt64(canceled, stat.CanceledAcquireCount())
		observer.ObserveFloat64(waited, stat.AcquireDuration().Seconds())
		return nil
	}, acquired, idle, total, maxConns, empty, canceled, waited)
	if err != nil {
		slog.Warn("telemetry: could not export database pool statistics", "error", err)
	}
}
