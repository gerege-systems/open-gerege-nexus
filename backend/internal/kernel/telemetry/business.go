/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * What the platform is being used for, as opposed to how it is holding up.
 */

package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// No tenant on any of these.
//
// A tenant id or slug in an attribute multiplies every series by the number of
// organisations on the deployment, and it never shrinks — an organisation that
// leaves keeps its series until the retention window expires. The per-tenant
// breakdown is a reporting question, and it is answered by the reports module
// against the database, where a row can be deleted.
//
// The instrument names carry no `_total`: the Prometheus exporter appends it to
// every counter, so `logins` is exported as `logins_total` — the same series
// name these metrics have always had.
var (
	// loginsTotal counts sign-in attempts by how they were made and how they
	// ended. method: password|eid|dan|google|sso. result: success|failure.
	loginsTotal = mustCounter("logins", "",
		"Sign-in attempts by method and outcome")

	// invoices_created_total is not here. It counts a billing event, and
	// billing is a distribution module — a platform that ships a counter named
	// after somebody else's domain has to be edited every time that domain
	// moves. The metric keeps its name and is registered by the module that
	// knows when an invoice happens; a deployment without billing simply does
	// not export it, which is the truth rather than a permanent zero.

	// documentsSignedTotal counts completed signature ceremonies.
	// rail: EID|DAN|HSM. result: success|failure.
	documentsSignedTotal = mustCounter("documents_signed", "",
		"Signature ceremonies by rail and outcome")

	// cpLoginAttemptsTotal counts attempts to sign in to the operator console,
	// by how they ended: success, unknown, bad_password, bad_code, locked,
	// disabled, no_second_factor, step_up, bad_step_up.
	//
	// Kept apart from loginsTotal rather than given a method attribute, because
	// the two answer different questions and one of them needs an alert.
	// Sign-ins to the platform fail all day — people mistype passwords.
	// Sign-ins to the control plane are a handful of people a week, so a dozen
	// failures in an hour is not noise, it is somebody trying.
	//
	// The result is a closed set and there is no operator identity in it. Both
	// matter: an address as an attribute value would be an unbounded series
	// driven by whoever is guessing.
	cpLoginAttemptsTotal = mustCounter("cp_login_attempts", "",
		"Operator console sign-in attempts by outcome")

	// aiRequestsTotal counts calls into the copilot, by what was asked of it.
	// kind: copilot|chat|stt|tts|translate|forecast.
	aiRequestsTotal = mustCounter("ai_requests", "",
		"Requests handled by the AI endpoints, by kind")
)

// Login methods and outcomes, as attribute values rather than strings typed at
// each call site.
const (
	LoginPassword = "password"
	LoginEID      = "eid"
	LoginDAN      = "dan"
	LoginGoogle   = "google"
	LoginSSO      = "sso"

	ResultSuccess = "success"
	ResultFailure = "failure"
)

// RecordLogin counts one sign-in attempt.
func RecordLogin(method string, ok bool) {
	loginsTotal.Add(context.Background(), 1, metric.WithAttributes(
		attribute.String("method", method),
		attribute.String("result", resultLabel(ok)),
	))
}

// RecordControlPlaneLogin counts one attempt to reach the operator console.
func RecordControlPlaneLogin(result string) {
	cpLoginAttemptsTotal.Add(context.Background(), 1,
		metric.WithAttributes(attribute.String("result", result)))
}

// RecordDocumentSigned counts one signature ceremony reaching its end.
func RecordDocumentSigned(rail string, ok bool) {
	documentsSignedTotal.Add(context.Background(), 1, metric.WithAttributes(
		attribute.String("rail", rail),
		attribute.String("result", resultLabel(ok)),
	))
}

// RecordAIRequest counts one call into the AI endpoints.
func RecordAIRequest(kind string) {
	aiRequestsTotal.Add(context.Background(), 1,
		metric.WithAttributes(attribute.String("kind", kind)))
}

func resultLabel(ok bool) string {
	if ok {
		return ResultSuccess
	}
	return ResultFailure
}
