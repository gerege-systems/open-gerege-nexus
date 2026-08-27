/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Asking somebody for a link, from either plane.
 *
 * The rail itself belongs to the tenant plane — internal/workspace/emailverify
 * owns the table, the provider call and the page a click lands on. What is here
 * is only what both planes have to name: the console invites the first
 * administrator of an organisation it has just created, and it cannot import a
 * plane to say what it is asking for.
 *
 * So this is the shape of the request, the shape of the answer, and the two
 * pieces of configuration that decide whether either is possible. Nothing here
 * sends anything.
 */

package mailrail

import (
	"context"
	"os"
	"strings"
	"time"
)

// Status mirrors the CHECK constraint on email_verifications.
type Status string

const (
	StatusPending  Status = "PENDING"
	StatusVerified Status = "VERIFIED"
	StatusExpired  Status = "EXPIRED"
)

// Verification is one link this platform asked for.
//
// It holds no token: the token is the provider's, and lives only in the mail.
// What is stored here is a hash of the single-use reference *we* put in the
// return address, which is how the click that comes back is matched to the
// request that caused it.
type Verification struct {
	ID       string `json:"id"`
	TenantID string `json:"tenant_id"`
	// Source names who asked: an app module id, or "portal".
	Source      string     `json:"source"`
	Purpose     string     `json:"purpose,omitempty"`
	Email       string     `json:"email"`
	RedirectURL string     `json:"redirect_url,omitempty"`
	Status      Status     `json:"status"`
	ExpiresAt   time.Time  `json:"expires_at"`
	VerifiedAt  *time.Time `json:"verified_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// Request is what a caller asks for. Every field except Email is optional.
type Request struct {
	Email string

	// RedirectURL is where the person is sent once they have come back to this
	// platform and the verification has been recorded. Empty means this
	// platform answers the click with a page of its own.
	RedirectURL string

	// Purpose is the caller's own label — "signup", "contact_invite" — carried
	// into the audit trail and back to the caller. It is not interpreted.
	Purpose string

	// Source names who asked. Empty is recorded as "platform".
	Source string

	// ClientIP is recorded for the audit trail. Empty is fine.
	ClientIP string
}

// Sender is the rail as the console needs to name it, satisfied by
// *emailverify.Service.
type Sender interface {
	Send(ctx context.Context, tenantID string, req Request) (*Verification, error)
}

// Configured reports whether a key was supplied, so that anything can be sent
// at all. Read from the environment on every call: a deployment that sets the
// key without restarting is a deployment that meant to.
func Configured() bool { return strings.TrimSpace(os.Getenv("EMAIL_VERIFY_API_KEY")) != "" }

// PublicOrigin is the address a recipient's browser can reach.
//
// It is read from PUBLIC_ORIGIN rather than from the incoming request: the
// return address is handed to somebody else's service and outlives the request,
// and taking the host from a request would let a forged Host header point every
// verification return at another server.
func PublicOrigin() string {
	origin := strings.TrimRight(strings.TrimSpace(os.Getenv("PUBLIC_ORIGIN")), "/")
	if origin == "" {
		origin = "http://localhost:8080"
	}
	return origin
}
