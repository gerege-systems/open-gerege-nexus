/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * The organisation's half: answering somebody who asked to be let in.
 *
 * Nothing here is privileged, which is what these tests are mostly saying. The
 * queue is this workspace's own rows and the decision is an ordinary write; the
 * only thing that leaves the workspace is the asker's copy, and it goes through
 * the capability the platform publishes for everybody.
 */

package access

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/person"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/auth"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

func joinPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// asker builds the whole situation in one call: a citizen with a home, an
// organisation, and an open request between them.
func asker(t *testing.T, pool *pgxpool.Pool) (userID, orgID, requestID string) {
	t.Helper()
	ctx := context.Background()
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:12]

	if err := pool.QueryRow(ctx,
		`INSERT INTO registry.users (email, password_hash, name) VALUES ($1,'x',$2) RETURNING id::text`,
		"joiner-"+suffix+"@example.mn", "Иргэн "+suffix).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM registry.users WHERE id=$1`, userID) })

	var homeID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO registry.tenants (slug, name, kind, owner_user_id)
		 VALUES ($1, $2, 'personal', $3::uuid) RETURNING id::text`,
		"home-"+suffix, "Иргэн "+suffix, userID).Scan(&homeID); err != nil {
		t.Fatal(err)
	}

	var slug string
	if err := pool.QueryRow(ctx,
		`INSERT INTO registry.tenants (slug, name) VALUES ($1, $1) RETURNING id::text, slug`,
		"org-"+suffix).Scan(&orgID, &slug); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM registry.tenants WHERE id=$1`, orgID) })

	// Asked through the real path, so the row under test is the one the
	// function makes rather than one this file invented.
	feed := person.New(pool)
	nexus.Provide[nexus.PersonFeed](person.AsPersonFeed(feed))
	if err := feed.Ask(ctx, userID, slug, "Намайг оруулна уу"); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT id::text FROM workspace.join_requests WHERE user_id = $1::uuid`, userID).Scan(&requestID); err != nil {
		t.Fatal(err)
	}
	return userID, orgID, requestID
}

func handlersFor(pool *pgxpool.Pool) *Handlers {
	return New(pool, nil, auth.New(auth.Deps{DB: pool}))
}

// Accepting makes the membership and tells them.
func TestAcceptingAJoinRequestAddsTheMemberAndAnswersThem(t *testing.T) {
	pool := joinPool(t)
	ctx := context.Background()
	userID, orgID, requestID := asker(t, pool)

	if err := handlersFor(pool).Decide(ctx, requestID, userID, true); err != nil {
		t.Fatalf("accept: %v", err)
	}

	var member bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM workspace.memberships WHERE tenant_id=$1::uuid AND user_id=$2::uuid)`,
		orgID, userID).Scan(&member); err != nil {
		t.Fatal(err)
	}
	if !member {
		t.Error("the request was accepted and no membership was written")
	}

	var status, copyStatus string
	if err := pool.QueryRow(ctx,
		`SELECT status FROM workspace.join_requests WHERE id = $1::uuid`, requestID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT status FROM workspace.person_items WHERE source_ref = $1`, requestID).Scan(&copyStatus); err != nil {
		t.Fatalf("the asker was not told: %v", err)
	}
	if status != "ACCEPTED" || copyStatus != "ACCEPTED" {
		t.Errorf("the request says %q and the asker's copy says %q", status, copyStatus)
	}
}

// Declining tells them too, and adds nobody.
//
// The second half matters more than it looks: a decline that quietly wrote a
// membership would be the worst possible bug here, and it is one line away.
func TestDecliningAddsNobodyAndStillAnswers(t *testing.T) {
	pool := joinPool(t)
	ctx := context.Background()
	userID, orgID, requestID := asker(t, pool)

	if err := handlersFor(pool).Decide(ctx, requestID, userID, false); err != nil {
		t.Fatalf("decline: %v", err)
	}

	var member bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM workspace.memberships WHERE tenant_id=$1::uuid AND user_id=$2::uuid)`,
		orgID, userID).Scan(&member); err != nil {
		t.Fatal(err)
	}
	if member {
		t.Error("a declined request added the person anyway")
	}

	var copyStatus string
	if err := pool.QueryRow(ctx,
		`SELECT status FROM workspace.person_items WHERE source_ref = $1`, requestID).Scan(&copyStatus); err != nil {
		t.Fatalf("the asker was not told: %v", err)
	}
	if copyStatus != "DECLINED" {
		t.Errorf("the asker's copy says %q", copyStatus)
	}
}

// Answering twice is answered once.
//
// Two administrators open the screen, both press accept. The second finds the
// row no longer pending and stops — without this the membership insert would
// run twice, and the asker's copy would be rewritten by whichever landed last.
func TestARequestIsAnsweredOnlyOnce(t *testing.T) {
	pool := joinPool(t)
	ctx := context.Background()
	userID, _, requestID := asker(t, pool)
	handlers := handlersFor(pool)

	if err := handlers.Decide(ctx, requestID, userID, true); err != nil {
		t.Fatal(err)
	}
	err := handlers.Decide(ctx, requestID, userID, false)
	if err == nil {
		t.Fatal("the same request was answered twice")
	}
	if !strings.Contains(err.Error(), "no open request") {
		t.Errorf("the second answer failed for an unexpected reason: %v", err)
	}
}
