/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package nexus

import "context"

// Who belongs to an organisation, as a module sees it.
//
// This is the platform's own model — users, memberships, the roles a membership
// holds — and no module may query those tables. They are where a mistake is
// least recoverable: a query that forgets a tenant clause on `memberships`
// hands one organisation the names of another's staff.
//
// It exists because an app was doing exactly that. internal/apps/organisation
// read six platform tables directly, which made it unmovable in the way no
// contract had yet described: not a Go import that a compiler could catch, but
// SQL naming somebody else's schema. Migration 00076 is the other half of the
// same change — `memberships` carried `job_title` and `department_id`, two
// columns belonging to that app, one of them a foreign key pointing *into* it.
//
// What is here is what a screen needs to draw a staff list. What is not here is
// anything that changes who somebody is: adding a member, granting a role and
// removing an account are acts of the platform, done in Access control, and a
// module that could do them from a contract would be a module that could grant
// itself a permission.
type DirectoryPerson struct {
	MembershipID string `json:"membership_id"`
	UserID       string `json:"user_id"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	Phone        string `json:"phone"`
	// Active is the membership's state, not the account's. A person deactivated
	// in one organisation still signs in and still belongs to the others.
	Active bool `json:"active"`
	// IsAdmin is the account's, and is the one field here that is not scoped to
	// the organisation: a platform administrator is one everywhere.
	IsAdmin  bool     `json:"is_admin"`
	Roles    []string `json:"roles"`
	JoinedAt string   `json:"joined_at"`
	// WorkspaceID and WorkspaceName say which organisation this membership is in.
	// Only meaningful when the session reads across more than one, which is
	// when a screen has to show it.
	WorkspaceID   string `json:"tenant_id"`
	WorkspaceName string `json:"tenant_name"`
}

// DirectoryMembership is the smallest answer about one membership: who it is
// and whether they administer the platform.
type DirectoryMembership struct {
	UserID  string
	IsAdmin bool
}

// Directory reads and — in one narrow respect — writes the platform's record of
// who belongs where.
type Directory interface {
	// People lists the memberships of the named organisations, with the roles
	// each holds. Ordered by organisation, then active first, then by name:
	// a staff list is read by a person looking for somebody.
	People(ctx context.Context, workspaceIDs []string) ([]DirectoryPerson, error)

	// Membership is one membership, or an error if this organisation has no
	// such row. Used before an act on somebody, to establish that they are
	// somebody this organisation may act on at all.
	Membership(ctx context.Context, workspaceID, membershipID string) (DirectoryMembership, error)

	// CountAdmins is how many active platform administrators this organisation
	// has, not counting one membership.
	//
	// The exception is the point: it answers "if I deactivate this person, is
	// anybody left who can undo it", which is the check that stops an
	// organisation locking itself out. A count without the exception would be
	// the wrong number by exactly one at the only moment it matters.
	CountAdmins(ctx context.Context, workspaceID, exceptMembershipID string) (int, error)

	// SetActive turns a membership on or off, reporting whether a row moved.
	//
	// The one write here, and the narrowest one that is still useful: a staff
	// screen has to be able to say somebody has left. It cannot add a member,
	// grant a role or delete an account — those are Access control's, and a
	// module that could do them could grant itself a permission.
	SetActive(ctx context.Context, workspaceID, membershipID string, active bool) (bool, error)
}

// People returns the directory this deployment provides.
func People() (Directory, error) { return Capability[Directory]() }
