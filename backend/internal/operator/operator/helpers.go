/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * The three small things every console screen uses.
 *
 * They were beside the service that held all of them. They are here now because
 * every screen in this plane imports this package and none of them imports each
 * other: the database role a console query runs as, the hostname it answers on,
 * and the difference between "no such organisation" and a malformed id.
 */

package operator

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/dbguard"
	"github.com/jackc/pgx/v5/pgconn"
)

// scoped puts a context on the operator's database role.
//
// Every query in this package goes through it, including the ones made before
// anybody is signed in — resolving a session, checking a password. Those could
// have run on the platform path like the tenant side's do, and that is exactly
// why this exists: the platform path is the login role, which is outside the
// row-level policies and can write anywhere. A console that reads other
// people's organisations for a living should never be one forgotten WHERE
// clause away from that, so it does not have the ability at all.
func Scoped(ctx context.Context) context.Context { return dbguard.AsOperator(ctx) }

// normaliseHost lowercases a hostname and drops any port, so that a value
// written as "cp.nexus.gerege.mn:443" in an environment file compares equal to
// what a browser sends.
func normaliseHost(raw string) string {
	host := strings.ToLower(strings.TrimSpace(raw))
	if colon := strings.LastIndex(host, ":"); colon > -1 && !strings.Contains(host[colon:], "]") {
		host = host[:colon]
	}
	return strings.TrimSuffix(host, ".")
}

// IsInvalidUUID reports whether an error is PostgreSQL refusing to read a
// string as a uuid — 22P02, which is what /api/platform/v1/tenants/not-a-uuid produces.
// It is a bad address, not a broken server, and it is answered as one.
func IsInvalidUUID(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "22P02"
}

// requestHost is the hostname a request was addressed to.
//
// r.Host is what nginx forwarded with `proxy_set_header Host $host`, which is
// the header the browser sent. X-Forwarded-Host is deliberately not consulted:
// it is a header any client can set, and consulting it would let a request
// arriving at the public hostname claim to be a control-plane one. The
// TRUST_PROXY_HEADERS convention exists for the client address, where there is
// no alternative; here there is one.
func requestHost(r *http.Request) string { return normaliseHost(r.Host) }

// Timings the console is built around.
const (
	// SessionTTL bounds one operator sign-in end to end. Shorter than the
	// tenant side's twelve hours: a console session that outlives the working
	// day is one an unattended machine keeps open overnight.
	SessionTTL = 8 * time.Hour

	// SessionIdleTimeout ends a console session nobody is using. §2.1 of the
	// plan asks for 30 minutes, against the platform's 90, and the reason is
	// the difference between a screen showing an invoice and a screen that can
	// suspend an organisation.
	SessionIdleTimeout = 30 * time.Minute

	// StepUpWindow is how long a re-confirmed second factor stays good for.
	// Long enough to complete the action it was asked for and anything
	// immediately following it; short enough that walking away ends it.
	StepUpWindow = 5 * time.Minute
)

// QueryTimeout bounds one console read. A screen that hangs is a screen an
// operator reloads, and a reload is another connection held open.
const QueryTimeout = 10 * time.Second
