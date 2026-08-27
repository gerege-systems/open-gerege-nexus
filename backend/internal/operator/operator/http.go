/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * The three things every console screen does with a request.
 *
 * They were in one handlers.go while every screen was in one package. Each
 * screen is its own package now and all of them read a bounded JSON body, take
 * a reason for the audit trail, and turn a refusal into an answer — so this is
 * the console's shared vocabulary rather than any one screen's.
 *
 * Fail here answers for the refusals this package owns and for everything it
 * does not recognise. A screen with sentinels of its own wraps it: its switch
 * names its own errors and falls through to this one, which is how each screen
 * states its own refusals instead of every screen appearing in one list.
 */

package operator

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
)

// Reasoned is the body every write on this console carries. A reason is not
// optional and not defaulted: Do refuses without one, so a handler that forgot
// to read it fails on the first attempt rather than filling the audit trail
// with empty strings.
type Reasoned struct {
	Reason string `json:"reason"`
}

// MaxWriteBody bounds a console write. Generous for a form, small enough that
// nothing here is a way to make the process allocate.
const MaxWriteBody = 32 << 10

// Decode reads a JSON body, bounded. Returns false having already answered.
func Decode(w http.ResponseWriter, r *http.Request, into any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, MaxWriteBody)
	if err := json.NewDecoder(r.Body).Decode(into); err != nil {
		httpx.Error(w, http.StatusBadRequest, "the request body could not be read")
		return false
	}
	return true
}

// Fail turns the console's shared sentinels into the answers they deserve.
func Fail(w http.ResponseWriter, err error, doing string) {
	switch {
	case errors.Is(err, ErrTenantNotFound):
		httpx.Error(w, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrReasonRequired), errors.Is(err, ErrNotAMember),
		errors.Is(err, ErrTenantSuspended):
		// Refusals the operator can act on, in words they can act on. These
		// are the platform's own sentinels, never a database error's text —
		// see the default.
		httpx.Error(w, http.StatusBadRequest, err.Error())
	default:
		// Anything else is logged in full and answered vaguely: an error from
		// PostgreSQL describes the schema, and the console is not the place to
		// publish it.
		slog.Error("control plane: "+doing, "error", err)
		httpx.Error(w, http.StatusInternalServerError, "that could not be completed")
	}
}
