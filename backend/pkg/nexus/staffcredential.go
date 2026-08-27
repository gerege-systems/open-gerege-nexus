/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package nexus

import (
	"context"
	"errors"
)

// StaffCredential authenticates a person on an enrolled shared device.
//
// The shape of the problem: a till or a kiosk is signed in as a *device*, and
// the people who take turns at it are not. Somebody types a short secret and
// the platform has to decide whose session to open. What the secret is — a PIN,
// a badge number, a card — is a product's decision; that a session may only be
// opened by the platform is not.
//
// So the split is: the app owns the credential and this platform owns the
// sign-in. The device route reads the device, hands the secret here, and mints
// the session from the answer. A deployment carrying no such app answers "this
// deployment authenticates nobody on a device" rather than pretending the
// secret was wrong.
//
// Implementations are asked on a request that carries no session at all. The
// tenant comes from the enrolled device, so an implementation must scope every
// lookup to it — and must decide for itself whether the organisation in
// question is entitled to the app at all.
type StaffCredential interface {
	// Verify returns who presented secret on a device belonging to workspaceID.
	//
	// ErrStaffCredentialRejected for a secret that does not match, an
	// organisation that does not have the app, or a credential that is locked
	// out. Anything else is a fault and is reported as one — the caller
	// distinguishes "not you" from "we could not tell", because answering the
	// second as the first is how a database outage becomes a wrong PIN.
	Verify(ctx context.Context, workspaceID, secret string) (StaffIdentity, error)
}

// StaffIdentity is who a verified credential belongs to.
//
// MembershipID is here because the till shows it: a person may belong to
// several organisations with one user id, and the shift is the membership's.
type StaffIdentity struct {
	UserID       string
	MembershipID string
	Name         string
	Email        string
}

// ErrStaffCredentialRejected is the answer to a secret that does not open
// anything. One error for every reason on purpose: a caller that could tell a
// wrong PIN from a locked one, or from an organisation without the app, could
// enumerate all three from outside.
var ErrStaffCredentialRejected = errors.New("nexus: the staff credential was rejected")
