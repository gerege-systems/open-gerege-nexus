/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package ai

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
)

// requestError is a refusal the caller can act on: the audio was not audio, the
// prompt was empty. Its words are meant to be read by whoever sent the request.
//
// Everything else — the model refusing, a key that is not set, the network — is
// ours, and the assistant talks to somebody else's service for a living: the
// text of those failures is theirs, arrives unedited, and has already once
// reached a citizen's screen through a different sign-in path
// (auth/handlers.go:467, the bcrypt sentence in the eID card). So it is logged
// with the act it belongs to and never rendered.
type requestError struct{ msg string }

func (e requestError) Error() string { return e.msg }

// BadRequest marks a refusal as the caller's to read.
func BadRequest(msg string) error { return requestError{msg: msg} }

// fail answers an assistant failure.
//
// `doing` names the act for the log line — "answer a question", "transcribe
// audio" — so a 502 in somebody's browser can be found in Loki without
// guessing which of the five routes it came from.
func fail(w http.ResponseWriter, err error, doing string) {
	var visible requestError
	if errors.As(err, &visible) {
		httpx.Error(w, http.StatusBadRequest, visible.Error())
		return
	}
	slog.Error("the assistant could not "+doing, "error", err)
	httpx.Error(w, http.StatusBadGateway, "the assistant is unavailable")
}
