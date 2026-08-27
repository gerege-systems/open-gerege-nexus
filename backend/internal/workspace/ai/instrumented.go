/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Timing the model calls.
 */

package ai

import (
	"context"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/gemini"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/telemetry"
)

// observed wraps a generator so every model call lands in
// external_request_duration_seconds.
//
// A decorator on the interface rather than a transport on the client. That was
// once forced — the client came from a library that kept its http.Client
// private — and it is now a choice: what is being measured is a model call,
// which is what this interface names. A RoundTripper would label the same work
// by URL, and the two clients below share one.
//
// The operation distinguishes the two clients the copilot holds. They call the
// same API with different models, and the TTS model is slower by a wide margin
// — averaged together they would describe neither.
type observed struct {
	inner     generator
	operation string
}

func observe(inner generator, operation string) generator {
	return &observed{inner: inner, operation: operation}
}

func (o *observed) GenerateContent(ctx context.Context, req gemini.Request) (gemini.Response, error) {
	return telemetry.ObserveExternalValue(ctx, telemetry.SystemGemini, o.operation,
		func(ctx context.Context) (gemini.Response, error) { return o.inner.GenerateContent(ctx, req) })
}
