/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package nexus

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// The identity rails a module signs with.
//
// eID and ДАН are the state's two ways of proving who somebody is, and both are
// the platform's to hold: credentials, a mock mode, a session store, a token it
// refreshes. A module that asks a citizen to sign needs none of that. It needs
// to start a ceremony, poll it, and read the five fields that say who signed
// and with which certificate.
//
// Five fields, where the platform's own record has fourteen — the same trade
// MeetingConnector makes, and for the same reason: a contract that hands over a
// storage record makes every field of that record part of the contract.
type SignatureCeremony struct {
	SessionID string `json:"session_id"`
	// DeviceLinkURL is the deep link a phone opens. Empty when the ceremony is
	// confirmed some other way.
	DeviceLinkURL    string `json:"device_link_url,omitempty"`
	VerificationCode string `json:"verification_code"`
	ExpiresAt        string `json:"expires_at"`
}

// SignerIdentity is who signed, and with what.
//
// CertificateSerial and CertificateIssuer are the two that make a signature
// checkable afterwards by somebody who was not there: they say which
// certificate the citizen approved with, which is the difference between a
// record of an act and evidence of one.
type SignerIdentity struct {
	RegNumber         string `json:"reg_number"`
	FirstName         string `json:"first_name"`
	LastName          string `json:"last_name"`
	CertificateSerial string `json:"certificate_serial,omitempty"`
	CertificateIssuer string `json:"certificate_issuer,omitempty"`
}

// CeremonyState is how a signing ceremony is going.
type CeremonyState struct {
	// State is the rail's own word — "pending", "completed", "expired". A
	// module shows it and does not branch on it beyond "is Identity set".
	State    string          `json:"state"`
	Identity *SignerIdentity `json:"identity,omitempty"`
}

// EIDSigner is the eID rail: start a signature, poll it.
type EIDSigner interface {
	// Mode is "live" or "mock". A deployment with no eID registration answers
	// mock, which a module should say out loud rather than hide.
	Mode() string
	StartSignature(ctx context.Context, nationalID, displayText, callbackURL string) (*SignatureCeremony, error)
	Poll(ctx context.Context, sessionID string) (*CeremonyState, error)
}

// ErrIdentityRailUnavailable reports that the rail could not be used at all —
// no credentials, or the service refused to answer. It is not "this person
// could not be verified", which is an answer.
var ErrIdentityRailUnavailable = errors.New("nexus: the identity rail is unavailable")

// DANCitizen is a person as ДАН verified them.
type DANCitizen struct {
	SessionID    string    `json:"dan_session_id"`
	RegNumber    string    `json:"reg_number"`
	CivilID      string    `json:"civil_id"`
	LastName     string    `json:"last_name"`
	FirstName    string    `json:"first_name"`
	MobileNumber string    `json:"mobile_number"`
	Email        string    `json:"email"`
	VerifiedAt   time.Time `json:"verified_at"`
}

// DANAuthenticator is the ДАН rail.
//
// AuthenticateCitizen returns ErrIdentityRailUnavailable when the rail itself
// is off, which a caller must tell apart from a citizen who failed to verify:
// the first is the deployment's problem and the second is an answer.
type DANAuthenticator interface {
	Mode() string
	AuthenticateCitizen(ctx context.Context, regNumber, otpCode string) (*DANCitizen, error)
}

// EID and DAN return the identity rails this deployment provides.
func EID() (EIDSigner, error)        { return Capability[EIDSigner]() }
func DAN() (DANAuthenticator, error) { return Capability[DANAuthenticator]() }

// SigningRails is the PDF signing surface the platform mounts inside an app's
// route tree.
//
// The rail stays the platform's — one signing path, PAdES and the HSM among
// them, and ADR 0002 is about why there must be exactly one. What the app owns
// is where those routes appear: they are reached under the documents app
// because that is where a person doing the signing already is.
//
// One method, and it takes the app's own gate: the platform mounts the
// handlers, the app decides who may reach them.
type SigningRails interface {
	Mount(r chi.Router, tenantAuthMiddleware func(http.Handler) http.Handler)
}

// SigningRailsOf returns the PDF signing rails this deployment provides.
//
// Not `Signing` — internal/operator already has a function by that name for the
// Signer capability, and two accessors one letter apart is how the wrong one
// gets called.
func SigningRailsOf() (SigningRails, error) { return Capability[SigningRails]() }
