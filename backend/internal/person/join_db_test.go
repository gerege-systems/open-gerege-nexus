/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Asking to be let in, and what stops it going wrong.
 *
 * The write crosses a workspace boundary, so it goes through a SECURITY DEFINER
 * function and the function's narrowness is the whole of the protection. These
 * are the four rules of registry.request_to_join, one test each, plus the round
 * trip that says the asker can actually see what they asked for.
 */

package person_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/person"
)

func openOrganisation(t *testing.T, pool *pgxpool.Pool) (id, slug string) {
	t.Helper()
	slug = "org-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO registry.tenants (slug, name) VALUES ($1, $1) RETURNING id::text`,
		slug).Scan(&id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM registry.tenants WHERE id=$1`, id) })
	return id, slug
}

// The round trip: one person asks, and sees that they asked.
//
// Two assertions rather than one because the row and its projection belong to
// different parties — the queue row is the organisation's, the copy is the
// person's — and the whole design is that the second exists: a request the
// asker cannot see is a request they will make again tomorrow.
func TestAskingPutsTheRequestInTheOrganisationAndTheCopyOnThePerson(t *testing.T) {
	pool := openPool(t)
	store := person.New(pool)
	ctx := context.Background()
	_, userID := seedPerson(t, pool)
	orgID, slug := openOrganisation(t, pool)

	if _, err := store.Ask(ctx, userID, slug, "Би энд ажилладаг"); err != nil {
		t.Fatalf("ask: %v", err)
	}

	var status, message, tenantID string
	if err := pool.QueryRow(ctx,
		`SELECT status, message, tenant_id::text FROM workspace.join_requests WHERE user_id = $1::uuid`,
		userID).Scan(&status, &message, &tenantID); err != nil {
		t.Fatalf("the request is not in the organisation's workspace: %v", err)
	}
	if status != "PENDING" || tenantID != orgID || message == "" {
		t.Errorf("the request is %q in %s with message %q", status, tenantID, message)
	}

	var copyStatus, copyCode, copyOwner string
	if err := pool.QueryRow(ctx,
		`SELECT status, code, user_id::text FROM registry.person_items WHERE source_app = $1`,
		person.CoreApp).Scan(&copyStatus, &copyCode, &copyOwner); err != nil {
		t.Fatalf("the asker has no copy of their own request: %v", err)
	}
	if copyOwner != userID {
		t.Errorf("the copy landed on %s, not on the asker %s", copyOwner, userID)
	}
	if copyStatus != person.StatusPending || copyCode != person.JoinRequestCode {
		t.Errorf("the copy says %q/%q", copyCode, copyStatus)
	}
}

// Pressing the button twice is one place in the queue.
func TestAskingTwiceIsOneRequest(t *testing.T) {
	pool := openPool(t)
	store := person.New(pool)
	ctx := context.Background()
	_, userID := seedPerson(t, pool)
	_, slug := openOrganisation(t, pool)

	for range 2 {
		if _, err := store.Ask(ctx, userID, slug, ""); err != nil {
			t.Fatal(err)
		}
	}
	var rows int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM workspace.join_requests WHERE user_id = $1::uuid`, userID).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Errorf("two asks left %d requests", rows)
	}
}

// An open organisation admits at the moment of asking.
//
// Three assertions, because "joined" is three facts and a screen that showed
// only the first would be lying about the other two: the membership exists, the
// row that explains it says a policy decided rather than a person, and the
// asker's own copy says ACCEPTED — a projection left on PENDING would tell
// somebody who is already inside to wait for an answer.
func TestAnOpenOrganisationAdmitsAtTheMomentOfAsking(t *testing.T) {
	pool := openPool(t)
	store := person.New(pool)
	ctx := context.Background()
	_, userID := seedPerson(t, pool)
	orgID, slug := openOrganisation(t, pool)
	if _, err := pool.Exec(ctx,
		`UPDATE registry.tenants SET join_policy = 'open' WHERE id = $1::uuid`, orgID); err != nil {
		t.Fatal(err)
	}

	outcome, err := store.Ask(ctx, userID, slug, "намайг оруулна уу")
	if err != nil {
		t.Fatalf("ask: %v", err)
	}
	if !outcome.Joined {
		t.Fatal("an open organisation answered with a request rather than a membership")
	}

	var members int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM workspace.memberships WHERE tenant_id = $1::uuid AND user_id = $2::uuid`,
		orgID, userID).Scan(&members); err != nil {
		t.Fatal(err)
	}
	if members != 1 {
		t.Errorf("memberships = %d, want 1", members)
	}

	var status string
	var decidedBy *string
	var decidedAt *time.Time
	if err := pool.QueryRow(ctx,
		`SELECT status, decided_by::text, decided_at FROM workspace.join_requests
		  WHERE tenant_id = $1::uuid AND user_id = $2::uuid`,
		orgID, userID).Scan(&status, &decidedBy, &decidedAt); err != nil {
		t.Fatalf("an open organisation left no record of who came in: %v", err)
	}
	if status != "ACCEPTED" || decidedAt == nil {
		t.Errorf("the record says %q decided at %v", status, decidedAt)
	}
	// NULL is the point: nobody decided this, the policy did.
	if decidedBy != nil {
		t.Errorf("decided_by = %q, want NULL — no person made this decision", *decidedBy)
	}

	var copyStatus string
	if err := pool.QueryRow(ctx,
		`SELECT status FROM registry.person_items WHERE user_id = $1::uuid AND source_app = $2`,
		userID, person.CoreApp).Scan(&copyStatus); err != nil {
		t.Fatalf("the person has no copy of what happened: %v", err)
	}
	if copyStatus != "ACCEPTED" {
		t.Errorf("the asker's copy says %q — they are in, and their own screen should say so", copyStatus)
	}
}

// What a new member arrives holding.
//
// Not nothing: 00008's `membership_default_role` trigger gives every new
// membership the organisation's `user` role, and that role carries every
// `%.read` permission. The same is true of an approved request — this is one
// path, not two — but it is the fact that decides what "open" means, so it is
// asserted rather than assumed. The first draft of this change claimed the
// opposite in three places; this test is why none of them shipped.
func TestAnOpenOrganisationGrantsThePlatformsDefaultRole(t *testing.T) {
	pool := openPool(t)
	store := person.New(pool)
	ctx := context.Background()
	_, userID := seedPerson(t, pool)
	orgID, slug := openOrganisation(t, pool)
	if _, err := pool.Exec(ctx,
		`UPDATE registry.tenants SET join_policy = 'open' WHERE id = $1::uuid`, orgID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Ask(ctx, userID, slug, ""); err != nil {
		t.Fatalf("ask: %v", err)
	}

	var codes []string
	rows, err := pool.Query(ctx,
		`SELECT r.code FROM workspace.membership_roles mr
		   JOIN workspace.memberships m ON m.id = mr.membership_id
		   JOIN workspace.roles r ON r.id = mr.role_id
		  WHERE m.tenant_id = $1::uuid AND m.user_id = $2::uuid`,
		orgID, userID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			t.Fatal(err)
		}
		codes = append(codes, code)
	}

	if len(codes) != 1 || codes[0] != "user" {
		t.Errorf("the new member holds %v, want exactly [user] — the door decides membership, "+
			"and the platform's own trigger decides what a membership is worth", codes)
	}
}

// The three things the function refuses, and the reason each one matters.
func TestTheDoorRefusesWhatItShould(t *testing.T) {
	pool := openPool(t)
	store := person.New(pool)
	ctx := context.Background()
	_, userID := seedPerson(t, pool)
	_, otherHomeSlug := homeSlugOf(t, pool)

	t.Run("a name nobody answers to", func(t *testing.T) {
		if _, err := store.Ask(ctx, userID, "no-such-organisation-here", ""); err == nil {
			t.Error("asking a name nobody answers to succeeded")
		}
	})

	// A home takes no members: it is one person's own space by construction,
	// and a queue outside it would be a queue nobody can answer.
	t.Run("somebody else's home", func(t *testing.T) {
		_, err := store.Ask(ctx, userID, otherHomeSlug, "")
		if err == nil {
			t.Fatal("asking to join somebody's home succeeded")
		}
		if !strings.Contains(err.Error(), "no organisation with slug") {
			t.Errorf("refused for an unexpected reason: %v", err)
		}
	})

	// A suspended organisation is not answering. Telling somebody they are in
	// a queue that will never move is worse than telling them no.
	t.Run("a suspended organisation", func(t *testing.T) {
		id, slug := openOrganisation(t, pool)
		if _, err := pool.Exec(ctx,
			`UPDATE registry.tenants SET suspended_at = NOW() WHERE id = $1::uuid`, id); err != nil {
			t.Fatal(err)
		}
		_, err := store.Ask(ctx, userID, slug, "")
		if err == nil {
			t.Fatal("asking a suspended organisation succeeded")
		}
		if !strings.Contains(err.Error(), "not accepting members") {
			t.Errorf("refused for an unexpected reason: %v", err)
		}
	})

	// Already inside. A request is a way in, not a second one.
	t.Run("an organisation this person is already in", func(t *testing.T) {
		id, slug := openOrganisation(t, pool)
		if _, err := pool.Exec(ctx,
			`INSERT INTO workspace.memberships (tenant_id, user_id) VALUES ($1::uuid, $2::uuid)`,
			id, userID); err != nil {
			t.Fatal(err)
		}
		_, err := store.Ask(ctx, userID, slug, "")
		if err == nil {
			t.Fatal("a member asked to join their own organisation")
		}
		if !strings.Contains(err.Error(), "already a member") {
			t.Errorf("refused for an unexpected reason: %v", err)
		}
	})
}

// homeSlugOf makes a second citizen with a personal workspace and returns its
// slug, which is the only way to name one from outside.
//
// A workspace is made here by hand rather than by seedPerson, because since
// 00093 a person does not need one and the helper no longer makes one — and
// this is the one test that needs a personal workspace to exist, in order to
// prove that naming it in a join request is refused.
func homeSlugOf(t *testing.T, pool *pgxpool.Pool) (id, slug string) {
	t.Helper()
	_, userID := seedPerson(t, pool)
	slug = "home-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO registry.tenants (slug, name, kind, owner_user_id)
		 VALUES ($1, $1, 'personal', $2::uuid) RETURNING id::text`, slug, userID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id, slug
}

// The queue is not writable from outside the function.
//
// The tenant role may read its own queue and answer it — that is the
// administrator's screen — and may not put somebody in it. Without this the
// crossing the function exists to control has a second door beside it.
func TestTheQueueCannotBeWrittenDirectly(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()
	_, userID := seedPerson(t, pool)
	orgID, _ := openOrganisation(t, pool)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true)`, orgID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `SET LOCAL ROLE gerege_nexus_tenant`); err != nil {
		t.Fatal(err)
	}

	if _, err := tx.Exec(ctx, `SELECT 1 FROM workspace.join_requests LIMIT 1`); err != nil {
		t.Fatalf("an organisation cannot read its own queue: %v", err)
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO workspace.join_requests (tenant_id, user_id) VALUES ($1::uuid, $2::uuid)`,
		orgID, userID)
	if err == nil {
		t.Error("the tenant role put somebody in the queue directly")
	} else if !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("the direct insert was refused for an unexpected reason: %v", err)
	}
}
