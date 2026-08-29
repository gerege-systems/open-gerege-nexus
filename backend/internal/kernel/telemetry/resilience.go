/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * The resilience machinery, seen from outside.
 */

package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// There is no breaker metric here.
//
// The design document asked for resilience_breaker_state alongside these, but
// the adaptive breaker it referred to was deliberately removed from
// internal/kernel/resilience — see that package's header for why. A gauge
// that never leaves zero because nothing writes to it is worse than no gauge:
// it makes a dashboard panel that says "all breakers closed" on a platform that
// has no breakers at all. When one arrives attached to the call it guards, the
// gauge belongs next to it in this file.
var (
	// loadShedTotal counts requests refused because the in-flight ceiling was
	// already reached. Exported as `resilience_load_shed_total`.
	loadShedTotal = mustCounter("resilience.load_shed", "",
		"Requests refused with 503 because the in-flight ceiling was reached")

	// inFlightRequests is what the load shedder is comparing against its
	// ceiling. Without it, a shed count says something went wrong but not how
	// close to the edge the normal state is.
	//
	// An up-down counter rather than a gauge, and that is a fix as much as a
	// translation. The gauge was Set() to a value the caller had read from an
	// atomic a moment earlier, so two requests finishing at once could write
	// their two snapshots in either order and leave the exported number stale
	// — reading high while the platform was idle, which is the one moment the
	// number is looked at. Add(±1) has no such window.
	inFlightRequests = mustUpDownCounter("resilience.in_flight_requests", "",
		"Requests currently being served")

	// retryTotal counts retried attempts, by the operation doing the retrying.
	// `name` is a constant chosen at the call site, never anything from a
	// request — a subscriber URL in this attribute would be unbounded
	// cardinality driven by tenant input.
	retryTotal = mustCounter("resilience.retry", "",
		"Attempts made after a first attempt failed, by operation")
)

// RecordLoadShed counts one refused request.
func RecordLoadShed() { loadShedTotal.Add(context.Background(), 1) }

// RecordRetry counts one retried attempt of a named operation.
func RecordRetry(name string) {
	retryTotal.Add(context.Background(), 1,
		metric.WithAttributes(attribute.String("name", name)))
}

// EnterFlight and LeaveFlight bracket one request being served. They are a pair
// and the second belongs in a defer, because a handler that panics still has to
// be counted out — the recovery middleware turns the panic into a 500 and the
// process carries on, so a missed decrement would be permanent.
func EnterFlight() { inFlightRequests.Add(context.Background(), 1) }
func LeaveFlight() { inFlightRequests.Add(context.Background(), -1) }
