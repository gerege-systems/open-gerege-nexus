/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package nexus

import (
	"context"
	"time"
)

// ------------------------------------------------------------------ the ring

// PeerDirectory is what a module may ask about this installation's links.
//
// The channel — the links, the signatures, the outbox, the retries — is the
// platform's, and Link is how a module *uses* it. This is how a module *reads*
// it: the names to put beside a task, whether a code has been announced on a
// link, what a code means. Three questions that a screen about work exchanged
// with another installation cannot avoid asking and that no module should
// answer by reading the channel's tables.
//
// Which is what happened until 2026-08-23. The task board joined urtuu_peers in
// nine queries and read urtuu_request_codes and urtuu_peer_codes in two more,
// on the argument — written down beside the code — that "the two packages are
// one product split by layer, sharing one schema". That argument holds exactly
// as long as both halves live in one repository, and ADR 0004 named it as the
// obstacle to the app leaving. This is the contract that removes it.
//
// The shape follows nexus.Directory: one read per page, and the caller maps ids
// to names in memory. A per-row accessor would turn a join into N queries,
// which is the wrong way to pay for a boundary.
type PeerDirectory interface {
	// Peers returns this organisation's links, with the state a screen shows.
	//
	// Revoked links are not returned: a revoked link carries nothing and a
	// screen that listed it would be offering somebody a peer to send to.
	Peers(ctx context.Context, workspaceID string) ([]Peer, error)

	// RequestCode returns what a code means to this installation, and false if
	// this installation has never been told.
	RequestCode(ctx context.Context, workspaceID, code string) (RequestCode, bool, error)

	// CodeOpenOn reports whether a code has been announced on one link.
	//
	// A parent that has not opened a code on a link must not send work under
	// it: the other end would receive a task naming a code nobody told it
	// about, and announcing the vocabulary is what stops it having to guess.
	CodeOpenOn(ctx context.Context, workspaceID, peerID, code string) (bool, error)

	// DeliveryLoad is what actually went over each link in a period.
	//
	// Here rather than left as a join because the alternative is a module
	// reading urtuu_deliveries, and a count of envelopes is the one question
	// about the channel that a report can ask and Peers cannot answer: Peers
	// says what is stuck now, this says what moved between two dates.
	DeliveryLoad(ctx context.Context, workspaceID string, from, to time.Time) ([]PeerLoad, error)
}

// PeerLoad is one link's traffic over a period.
type PeerLoad struct {
	PeerID    string `json:"peer_id"`
	Envelopes int64  `json:"envelopes"`
	Delivered int64  `json:"delivered"`
	Pending   int64  `json:"pending"`
	// Retries counts attempts beyond the first. An envelope that went first
	// time has one attempt and no retry — counting it as one would make a
	// healthy channel look like a struggling one.
	Retries int64 `json:"retries"`
}

// Peer is one link, as a screen about work sees it.
//
// Narrower than the channel's own record on purpose. The rail's peer carries
// the base URL, the public key, the invitation's expiry and the clock skew —
// which are an administrator's business on the links screen, and none of a task
// board's. A contract that carried all fourteen fields would make every one of
// them a promise to keep.
type Peer struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Role is what this installation is on the link, from here: the direction
	// rather than a rank.
	Role   string `json:"role"`
	Status string `json:"status"`
	// LastSeenAt and LastError are the two halves of "is this link working".
	// Nil and empty on a link that has never been used, which is not an error.
	LastSeenAt *time.Time `json:"last_seen_at,omitempty"`
	LastError  string     `json:"last_error,omitempty"`
	// Undelivered is how much is waiting in the outbox for this peer. It is the
	// number that turns "the link is fine" into "the link has been fine for an
	// hour and nothing has moved".
	Undelivered int `json:"undelivered"`
}

// RequestCode is an entry in the vocabulary two installations exchange work
// under: what the code is called, what it promises, and whether it is in use.
type RequestCode struct {
	Code string `json:"code"`
	// Names is the code's label per language. A code is read by people on both
	// sides of a link and they do not have to share a language.
	Names map[string]string `json:"names"`
	// SLA is the promise in seconds, and nil where none was made. A pointer
	// rather than a zero: "answer within no time at all" and "nobody promised
	// anything" are different facts and a screen says different things about
	// them.
	SLA *int64 `json:"sla_seconds,omitempty"`
	// Line is which of the two kinds of work this is — a service a citizen
	// asked for, or an instruction from above.
	Line   string `json:"line"`
	Active bool   `json:"active"`
	// Source is the installation the code came from: this one, or the peer
	// that announced it.
	Source string `json:"source"`
}

// Peers returns this organisation's links from whatever the platform provides.
func Peers(ctx context.Context, workspaceID string) ([]Peer, error) {
	directory, err := Capability[PeerDirectory]()
	if err != nil {
		return nil, err
	}
	return directory.Peers(ctx, workspaceID)
}
