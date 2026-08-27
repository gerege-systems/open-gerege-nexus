/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package nexus

import "context"

// What a citizen sees of a request they made to somebody else's organisation.
//
// A person who asks four suppliers for four things has four rows in four
// organisations' tables, and no way to see them together: each one is behind
// that organisation's row-level policy, which is the policy working. The answer
// is not a session that reads across a hundred organisations — that is a great
// deal of access to grant for a list of four things. It is a projection: the
// supplier publishes the state of the request into the citizen's own workspace,
// and the citizen reads their own workspace, which they could always do.
//
// The direction matters and is the whole design. The module pushes; the
// platform never pulls. A platform that read a module's tables would have to
// know their shape, which is the coupling pkg/nexus exists to prevent — and it
// would be reading them without the module's transaction, so it would see
// states the module had not finished writing.
//
// # What belongs in an item
//
// State, not content. A code, a status, a time, an answer somebody can read.
// The documents, the evidence and the personal details stay in the supplier's
// workspace, where the work is done and where the audit trail is.
//
// That line is drawn in the database as well as here: workspace.person_items
// caps the answer's length, so the projection cannot grow into a document
// store one field at a time. Migration 00086 says why at length — briefly, a
// table holding every citizen's every request across every supplier is a step
// toward the central database X-Road was built to avoid, and it is only safe
// while it stays a summary.
//
// # Idempotent by construction
//
// A module calls Publish every time the request changes state, in the same
// transaction as its own write and its own audit row. The pair
// (SourceApp, SourceRef) identifies the request, so the second call updates the
// first row rather than adding one. There is no Delete: a citizen's record of
// having asked is not the supplier's to remove.
type PersonFeed interface {
	Publish(ctx context.Context, geID int64, item PersonItem) error
}

// PersonItem is one request, as the person who made it sees it.
type PersonItem struct {
	// SourceApp is the module publishing this, by module id. SourceRef is that
	// module's own identifier for the request. Together they are the key: the
	// workspace half is not here because the caller does not choose it — the
	// Gerege number does, inside the function that writes the row.
	SourceApp string
	SourceRef string

	// ProviderWorkspaceID is the organisation doing the work, so the citizen
	// can see who has it. Optional: a request nobody has picked up yet has
	// nowhere to point.
	ProviderWorkspaceID string

	// Code is the service asked for — the shared vocabulary, not free text.
	Code string
	// Status is the module's own word for where it is. The platform does not
	// interpret it; the screen shows it.
	Status string
	// Answer is what the citizen is told, when there is something to tell.
	// Long values are refused rather than truncated: see the note above.
	Answer string
}
