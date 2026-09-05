/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * The two response helpers moved to `backend/pkg/nexus`, where an app module in
 * another repository can reach them. These forward, so the platform's own
 * handlers did not all have to change on the same day — and so there is one
 * implementation rather than two that could answer differently.
 */

package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

// JSON writes value as the whole response body.
func JSON(w http.ResponseWriter, status int, value any) { nexus.JSON(w, status, value) }

// Error answers with {"error": message}.
func Error(w http.ResponseWriter, status int, message string) { nexus.Error(w, status, message) }

// DecodeLimited reads a JSON request body no larger than max bytes.
//
// A handler that decodes an unbounded body is a handler that can be handed a
// gigabyte, and the cost is paid before any check the handler makes. Every
// caller names its own ceiling because the honest size differs by two orders of
// magnitude across them — a PIN, a pasted document, a base64 recording.
//
// It lived in ai_handlers.go as an unexported helper until 2026-08-23, which is
// where it was first needed rather than where it belonged. When the assistant
// left for internal/apps/ai it turned out nine platform handlers used it, and
// when the connectors left too there would have been three copies. A helper
// this many callers share is part of the request vocabulary, which is what this
// package is.
func DecodeLimited(r *http.Request, dst any, max int64) error {
	if r.Body == nil {
		return io.EOF
	}
	decoder := json.NewDecoder(http.MaxBytesReader(nil, r.Body, max))
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	// Require the whole body to fit and contain exactly one JSON value.
	// LimitReader alone makes the size limit look like a successful EOF.
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		if err != nil {
			return err
		}
		return errors.New("request body must contain a single JSON value")
	}
	return nil
}
