/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package nexus

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Sending something to another installation, and being handed what arrives.
//
// The Өртөө ring is a platform capability for the same reason document filing
// is: it is one signing key, one outbox, one retry schedule and one set of
// peers per *installation*, not per app. Two apps that both needed to reach
// another deployment would otherwise each grow their own transport, and the day
// they disagreed about retries, freshness or signature format the ring would
// stop being one ring.
//
// So the transport stays in the platform and is published here. What travels on
// it is the caller's business: a task assignment, a report, a request for a
// service — the ring does not know what a task is, and that is deliberate. It
// carries a signed envelope of a named kind between two installations that have
// agreed to talk, and refuses everything else.
//
// # What is deliberately not here
//
// Peering. Who this installation is linked to, in which direction, with which
// key, is an operator's decision made in the console — not something a module
// can arrange for itself. A module that could add a peer could arrange its own
// audience, and the whole value of the ring is that both ends agreed first.
//
// Nor is delivery order, retry or acknowledgement. A module hands over a
// message and is told the id it will be known by; when it arrives, and how
// often the platform tried, is the transport's problem. A module that could
// influence that would be a module that could starve another one.
type Link interface {
	// Enabled reports whether this installation can send at all.
	//
	// False on every deployment that has not been given a signing key, which is
	// most of them: the ring is opt-in and an installation with no key is not
	// half-joined, it is not joined. A module should ask before offering a
	// screen whose buttons would all fail.
	Enabled() bool

	// InstallationID is what this deployment is called on the ring — the name
	// the other end sees as the sender.
	InstallationID() string

	// Enqueue hands a message to the outbox and returns the id it will be
	// known by at both ends.
	//
	// It does not send. The platform signs, delivers and retries on its own
	// schedule, so a caller that returns successfully has been promised the
	// message is *recorded*, not that it has arrived — the difference matters
	// on a link that is down, which is the case this exists for.
	//
	// With no peer ids the message goes to every peer this tenant is linked to
	// in the sending direction. Naming peers narrows it to those.
	Enqueue(ctx context.Context, workspaceID, kind string, payload any, peerIDs ...string) (string, error)

	// EnqueueTx is Enqueue inside the caller's transaction.
	//
	// This is the one that is usually right. A task written to a module's own
	// table and a message announcing it are one fact: enqueuing outside the
	// transaction that writes the row produces an announcement of something
	// that never existed when the transaction rolls back, and there is nothing
	// downstream that can tell.
	EnqueueTx(ctx context.Context, tx pgx.Tx, workspaceID, kind string, payload any, peerIDs ...string) (string, error)

	// Deliver registers the reader for one kind of arriving message.
	//
	// Called during construction, once per kind. One reader per kind by design:
	// two would make "was this processed" a question with two answers.
	//
	// Returning an error leaves the envelope unprocessed and it is offered
	// again on the next round, so a reader that fails because the database was
	// briefly away does not lose the work. A reader that fails because the
	// message is nonsense will be offered it for ever, which is the right
	// trade: the platform cannot tell those apart, and losing work quietly is
	// worse than retrying it loudly.
	Deliver(kind string, reader LinkReader)
}

// LinkReader consumes one arriving message.
type LinkReader func(ctx context.Context, message LinkMessage) error

// LinkMessage is one envelope as the receiving module sees it.
type LinkMessage struct {
	WorkspaceID string
	PeerID      string
	PeerName    string
	MessageID   string
	Kind        string
	// CreatedAt is the *sender's* clock. Deadlines are measured from it by
	// design: the receiving installation's clock is not the one the work was
	// promised against.
	CreatedAt time.Time
	Payload   []byte
}

// ErrNoLink is returned by Ring when this binary has no transport compiled in.
//
// A distribution that builds the platform without it — or a test — gets a clear
// answer rather than a nil interface that panics on first use.
var ErrNoLink = errors.New("nexus: this deployment has no installation link")

// UseLink installs the capability.
//
// Deprecated: use Provide[Link] instead. This is a wrapper over it and behaves
// identically, including UseLink(nil) to withdraw. It stays for one major
// version so a distribution pinned to v1 keeps compiling, and goes in v2 — see
// docs/RELEASING.md.
func UseLink(l Link) { Provide[Link](l) }

// Ring returns the link capability, or ErrNoLink.
//
// Asked for per use rather than captured at construction, because a module may
// be built before the platform has installed it — the same reason Documents()
// is a function. That property is now Capability's, which this calls.
//
// The sentinel is kept rather than passing Capability's error through: callers
// written against v1 may compare against ErrNoLink, and "this deployment has no
// installation link" says more than the generic form can.
func Ring() (Link, error) {
	link, err := Capability[Link]()
	if err != nil {
		return nil, ErrNoLink
	}
	return link, nil
}
