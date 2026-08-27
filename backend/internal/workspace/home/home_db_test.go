/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * The four rules that are the whole of this feature's security.
 *
 * registry.publish_person_item is SECURITY DEFINER, so it runs as the role that
 * created it and row-level security does not apply to it at all. That is not a
 * weakness to be apologised for — it is the only way a supplier's module can
 * write into a citizen's workspace, and migration 00034 solved the identical
 * problem the identical way years of commits ago. But it does mean the function
 * is the entire attack surface, and that its narrowness is not a style choice.
 *
 * So the narrowness is tested rather than commented: it writes only into a
 * personal workspace, only into the one whose owner carries the Gerege number
 * it was given, only into one table, and only for one role. Every one of those
 * is a line somebody could delete while the feature kept working.
 */

package home_test

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/home"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

func openPool(t *testing.T) *pgxpool.Pool {
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

// seedHome is a citizen: an account with a Gerege number and the personal
// workspace 00085 gives them.
func seedHome(t *testing.T, pool *pgxpool.Pool) (geID int64, userID, homeID string) {
	t.Helper()
	ctx := context.Background()
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:12]

	// Derived from the uuid rather than a counter: the tests in this file run
	// beside every other package's against one database, and ge_id carries a
	// unique index, so a fixed number collides with whatever ran a moment ago.
	var seed [6]byte
	if _, err := rand.Read(seed[:]); err != nil {
		t.Fatal(err)
	}
	geID = int64(binary.BigEndian.Uint32(seed[:4]))<<8 | int64(seed[4]) + 1
	if err := pool.QueryRow(ctx,
		`INSERT INTO registry.users (email, password_hash, name, ge_id)
		 VALUES ($1, 'x', $2, $3) RETURNING id::text`,
		"hometest-"+suffix+"@example.mn", "Иргэн "+suffix, geID).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM registry.users WHERE id=$1`, userID) })

	if err := pool.QueryRow(ctx,
		`INSERT INTO registry.tenants (slug, name, kind, owner_user_id)
		 VALUES ($1, $2, 'personal', $3::uuid) RETURNING id::text`,
		"home-"+suffix, "Иргэн "+suffix, userID).Scan(&homeID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO workspace.memberships (tenant_id, user_id) VALUES ($1::uuid, $2::uuid)`,
		homeID, userID); err != nil {
		t.Fatal(err)
	}
	return geID, userID, homeID
}

func seedOrganisation(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO registry.tenants (slug, name) VALUES ($1, $1) RETURNING id::text`,
		"provider-"+suffix).Scan(&id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM registry.tenants WHERE id=$1`, id) })
	return id
}

func item(ref string) nexus.PersonItem {
	return nexus.PersonItem{SourceApp: "io.gerege.nexus.urtuu", SourceRef: ref, Code: "Д-101", Status: "OPEN"}
}

// A supplier publishes into the citizen's home, and the citizen reads it.
//
// The round trip in one test because the two halves are only interesting
// together: a write nobody can read is a write into a hole.
func TestAPublishedRequestArrivesInThatPersonsHome(t *testing.T) {
	pool := openPool(t)
	store := home.New(pool)
	ctx := context.Background()

	geID, _, homeID := seedHome(t, pool)
	provider := seedOrganisation(t, pool)

	published := item("REQ-1")
	published.ProviderWorkspaceID = provider
	published.Answer = "Хүсэлтийг хүлээн авлаа."
	if err := store.Publish(ctx, geID, published); err != nil {
		t.Fatalf("publish: %v", err)
	}

	var tenantID, code, status, answer string
	if err := pool.QueryRow(ctx,
		`SELECT tenant_id::text, code, status, answer FROM workspace.person_items
		  WHERE source_ref = 'REQ-1'`).Scan(&tenantID, &code, &status, &answer); err != nil {
		t.Fatalf("the published row is not there: %v", err)
	}
	if tenantID != homeID {
		t.Errorf("the row landed in %s, not in the person's home %s", tenantID, homeID)
	}
	if code != "Д-101" || status != "OPEN" || answer == "" {
		t.Errorf("the row is %q/%q/%q", code, status, answer)
	}
}

// Publishing twice is one row. A module calls this every time the request
// moves, so anything else is a feed that grows a line per state change.
func TestPublishingTheSameRequestTwiceUpdatesOneRow(t *testing.T) {
	pool := openPool(t)
	store := home.New(pool)
	ctx := context.Background()
	geID, _, homeID := seedHome(t, pool)

	if err := store.Publish(ctx, geID, item("REQ-2")); err != nil {
		t.Fatal(err)
	}
	moved := item("REQ-2")
	moved.Status = "DONE"
	moved.Answer = "Бэлэн боллоо."
	if err := store.Publish(ctx, geID, moved); err != nil {
		t.Fatal(err)
	}

	var rows int
	var status string
	if err := pool.QueryRow(ctx,
		`SELECT count(*), max(status) FROM workspace.person_items WHERE tenant_id = $1::uuid`,
		homeID).Scan(&rows, &status); err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Errorf("two publishes left %d rows", rows)
	}
	if status != "DONE" {
		t.Errorf("the row still says %q", status)
	}
}

// Rule 1, the important one: the Gerege number decides the workspace, and a
// caller who names somebody else's cannot reach it.
//
// The function takes no workspace id at all — that is the design — so this
// asserts the only thing a caller could get wrong: a number that is not this
// person's writes into that person's home and no other.
func TestTheGeregeNumberDecidesWhoseHomeItIs(t *testing.T) {
	pool := openPool(t)
	store := home.New(pool)
	ctx := context.Background()

	firstGeID, _, firstHome := seedHome(t, pool)
	secondGeID, _, secondHome := seedHome(t, pool)

	if err := store.Publish(ctx, firstGeID, item("REQ-A")); err != nil {
		t.Fatal(err)
	}
	if err := store.Publish(ctx, secondGeID, item("REQ-B")); err != nil {
		t.Fatal(err)
	}

	for _, want := range []struct{ home, ref string }{{firstHome, "REQ-A"}, {secondHome, "REQ-B"}} {
		var refs []string
		rows, err := pool.Query(ctx,
			`SELECT source_ref FROM workspace.person_items WHERE tenant_id = $1::uuid`, want.home)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var ref string
			if err := rows.Scan(&ref); err != nil {
				t.Fatal(err)
			}
			refs = append(refs, ref)
		}
		rows.Close()
		if len(refs) != 1 || refs[0] != want.ref {
			t.Errorf("home %s holds %v, want only %s", want.home, refs, want.ref)
		}
	}
}

// A Gerege number nobody here carries is an error, not a new home.
//
// The alternative — creating the workspace on demand — would let a typo open a
// space for a person who has never signed in to this deployment, and would make
// a wrong number indistinguishable from a right one.
func TestPublishingToANumberWithNoHomeFails(t *testing.T) {
	pool := openPool(t)
	store := home.New(pool)

	err := store.Publish(context.Background(), 999999999999, item("REQ-NOBODY"))
	if err == nil {
		t.Fatal("publishing to a Gerege number with no home succeeded")
	}
	if !strings.Contains(err.Error(), "no personal workspace") {
		t.Errorf("refused for an unexpected reason: %v", err)
	}
}

// Rule 1's other half: an organisation is not a home, whoever owns it.
//
// Reached by pointing the function at a workspace that exists and is the wrong
// kind — which is the state a bug would produce, rather than one an API allows.
func TestAnOrganisationIsNeverPublishedInto(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()
	geID, userID, _ := seedHome(t, pool)
	organisation := seedOrganisation(t, pool)

	// Make the citizen the owner of an organisation as well — the closest a
	// caller can get to confusing the two — and check the function still picks
	// the home. owner_user_id on a non-personal row is refused by 00085's
	// constraint, so the attempt itself is the assertion.
	_, err := pool.Exec(ctx,
		`UPDATE registry.tenants SET owner_user_id = $1::uuid WHERE id = $2::uuid`, userID, organisation)
	if err == nil {
		t.Fatal("an organisation was given an owner; 00085's constraint is not holding")
	}
	if !strings.Contains(err.Error(), "tenants_home_has_an_owner") {
		t.Fatalf("refused for an unexpected reason: %v", err)
	}

	// And the positive half: with the organisation unreachable, the number
	// still resolves to the home.
	store := home.New(pool)
	if err := store.Publish(ctx, geID, item("REQ-C")); err != nil {
		t.Fatal(err)
	}
	var kind string
	if err := pool.QueryRow(ctx,
		`SELECT t.kind FROM workspace.person_items i JOIN registry.tenants t ON t.id = i.tenant_id
		  WHERE i.source_ref = 'REQ-C'`).Scan(&kind); err != nil {
		t.Fatal(err)
	}
	if kind != "personal" {
		t.Errorf("the row landed in a %q workspace", kind)
	}
}

// Rule 4, and rule 2's consequence: the only way in is the function.
//
// The tenant role holds SELECT and nothing else, so a module that decided to
// write the row itself — with the right workspace id, having looked it up — is
// stopped by the database rather than by review. And the operator role cannot
// call the function at all: the console has no business publishing into
// somebody's home, and an operator session that could would be a way to put
// words in front of a citizen with no module and no audit row behind them.
func TestOnlyTheTenantRoleReachesTheFeedAndOnlyThroughTheFunction(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()
	_, _, homeID := seedHome(t, pool)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true)`, homeID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `SET LOCAL ROLE gerege_nexus_tenant`); err != nil {
		t.Fatal(err)
	}

	// Reading its own workspace: allowed, and the point of the feature.
	if _, err := tx.Exec(ctx, `SELECT 1 FROM workspace.person_items LIMIT 1`); err != nil {
		t.Fatalf("the tenant role cannot read its own person_items: %v", err)
	}

	// Writing directly: refused.
	_, err = tx.Exec(ctx,
		`INSERT INTO workspace.person_items (tenant_id, source_app, source_ref, code, status)
		 VALUES ($1::uuid, 'a', 'b', 'c', 'd')`, homeID)
	if err == nil {
		t.Error("the tenant role inserted into person_items directly")
	} else if !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("the direct insert was refused for an unexpected reason: %v", err)
	}
}

func TestTheOperatorRoleCannotPublish(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()
	geID, _, _ := seedHome(t, pool)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SET LOCAL ROLE gerege_nexus_operator`); err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(ctx,
		`SELECT registry.publish_person_item($1::bigint, NULL::uuid, 'app', 'ref', 'code', 'status', '')`, geID)
	if err == nil {
		t.Fatal("the operator role published into a citizen's home")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("refused for an unexpected reason: %v", err)
	}
}

// Row-level security, doing what it already did.
//
// Nothing in 00086 asked for a new policy shape, and this is the test that says
// the old one is enough: one citizen's session, bound to their own workspace,
// sees their row and not the other person's.
func TestOneCitizenDoesNotSeeAnothersRequests(t *testing.T) {
	pool := openPool(t)
	store := home.New(pool)
	ctx := context.Background()

	firstGeID, _, firstHome := seedHome(t, pool)
	secondGeID, _, _ := seedHome(t, pool)
	if err := store.Publish(ctx, firstGeID, item("REQ-MINE")); err != nil {
		t.Fatal(err)
	}
	if err := store.Publish(ctx, secondGeID, item("REQ-THEIRS")); err != nil {
		t.Fatal(err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true)`, firstHome); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `SET LOCAL ROLE gerege_nexus_tenant`); err != nil {
		t.Fatal(err)
	}

	rows, err := tx.Query(ctx, `SELECT source_ref FROM workspace.person_items`)
	if err != nil {
		t.Fatal(err)
	}
	var seen []string
	for rows.Next() {
		var ref string
		if err := rows.Scan(&ref); err != nil {
			t.Fatal(err)
		}
		seen = append(seen, ref)
	}
	rows.Close()

	if len(seen) != 1 || seen[0] != "REQ-MINE" {
		t.Errorf("a citizen's session sees %v; it should see only its own request", seen)
	}
}
