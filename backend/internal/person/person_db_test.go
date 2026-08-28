/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * The rules that are the whole of this feature's security.
 *
 * registry.publish_person_item is SECURITY DEFINER, so it runs as the role that
 * created it and row-level security does not apply to it at all. That is not a
 * weakness to be apologised for — it is the only way a supplier's module can
 * write a row that belongs to somebody else, and migration 00034 solved the
 * identical problem the identical way years of commits ago. But it does mean
 * the function is the entire attack surface, and that its narrowness is not a
 * style choice.
 *
 * So the narrowness is tested rather than commented: it writes only for the
 * person it was given, only into one table, and only for one role. Every one of
 * those is a line somebody could delete while the feature kept working.
 *
 * Since 00093 the rows are keyed by the person rather than by their workspace,
 * and one rule left as a consequence: there is no longer a "wrong kind of
 * workspace" to write into, because the destination is a foreign key into
 * registry.users. What replaced it is stricter — a destination that is not a
 * person is refused by the database rather than by a lookup that could return
 * nothing quietly.
 */

package person_test

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/person"
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

// seedPerson is a citizen: an account with a Gerege number, and nothing else.
//
// Deliberately no workspace. Until 00093 one was required — the publish
// function resolved the person to their home and refused if they had none — and
// a test that keeps making one would go on passing after the requirement it was
// covering had gone.
func seedPerson(t *testing.T, pool *pgxpool.Pool) (geID int64, userID string) {
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
	return geID, userID
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

// A supplier publishes to the citizen, and the citizen reads it.
//
// The round trip in one test because the two halves are only interesting
// together: a write nobody can read is a write into a hole.
func TestAPublishedRequestArrivesOnThatPerson(t *testing.T) {
	pool := openPool(t)
	store := person.New(pool)
	ctx := context.Background()

	geID, userID := seedPerson(t, pool)
	provider := seedOrganisation(t, pool)

	published := item("REQ-1")
	published.ProviderWorkspaceID = provider
	published.Answer = "Хүсэлтийг хүлээн авлаа."
	if err := store.Publish(ctx, geID, published); err != nil {
		t.Fatalf("publish: %v", err)
	}

	var owner, code, status, answer string
	if err := pool.QueryRow(ctx,
		`SELECT user_id::text, code, status, answer FROM registry.person_items
		  WHERE source_ref = 'REQ-1'`).Scan(&owner, &code, &status, &answer); err != nil {
		t.Fatalf("the published row is not there: %v", err)
	}
	if owner != userID {
		t.Errorf("the row landed on %s, not on the person it was published to, %s", owner, userID)
	}
	if code != "Д-101" || status != "OPEN" || answer == "" {
		t.Errorf("the row is %q/%q/%q", code, status, answer)
	}
}

// Publishing twice is one row. A module calls this every time the request
// moves, so anything else is a feed that grows a line per state change.
func TestPublishingTheSameRequestTwiceUpdatesOneRow(t *testing.T) {
	pool := openPool(t)
	store := person.New(pool)
	ctx := context.Background()
	geID, userID := seedPerson(t, pool)

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
		`SELECT count(*), max(status) FROM registry.person_items WHERE user_id = $1::uuid`,
		userID).Scan(&rows, &status); err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Errorf("two publishes left %d rows", rows)
	}
	if status != "DONE" {
		t.Errorf("the row still says %q", status)
	}
}

// The important rule: the Gerege number decides whose row it is, and a caller
// who names somebody else's cannot reach past it.
//
// The function takes no workspace id at all — that is the design — so this
// asserts the only thing a caller could get wrong: a number that is not this
// person's writes onto that person and no other.
func TestTheGeregeNumberDecidesWhoseRowItIs(t *testing.T) {
	pool := openPool(t)
	store := person.New(pool)
	ctx := context.Background()

	firstGeID, firstUser := seedPerson(t, pool)
	secondGeID, secondUser := seedPerson(t, pool)

	if err := store.Publish(ctx, firstGeID, item("REQ-A")); err != nil {
		t.Fatal(err)
	}
	if err := store.Publish(ctx, secondGeID, item("REQ-B")); err != nil {
		t.Fatal(err)
	}

	for _, want := range []struct{ person, ref string }{{firstUser, "REQ-A"}, {secondUser, "REQ-B"}} {
		var refs []string
		rows, err := pool.Query(ctx,
			`SELECT source_ref FROM registry.person_items WHERE user_id = $1::uuid`, want.person)
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
			t.Errorf("person %s holds %v, want only %s", want.person, refs, want.ref)
		}
	}
}

// A Gerege number nobody here carries is an error, not a new account.
//
// The alternative — creating the person on demand — would let a typo open a
// record for somebody who has never been near this deployment, and would make a
// wrong number indistinguishable from a right one. The refusal now comes from a
// foreign key rather than from a lookup: registry.person_items.user_id
// references registry.users, so a destination that is not a person cannot be
// written even by the SECURITY DEFINER function that bypasses every policy.
func TestPublishingToSomebodyWhoDoesNotExistFails(t *testing.T) {
	pool := openPool(t)
	store := person.New(pool)
	ctx := context.Background()

	// A Gerege number nobody carries: the lookup that turns it into an account
	// finds nothing, and the write never starts.
	err := store.Publish(ctx, 999999999999, item("REQ-NOBODY"))
	if err == nil {
		t.Fatal("publishing to a Gerege number nobody carries succeeded")
	}
	if !strings.Contains(err.Error(), "find the person") {
		t.Errorf("refused for an unexpected reason: %v", err)
	}

	// And an id that is well-formed and is not a person. This is the state a
	// bug produces rather than one an API allows, which is exactly why the
	// database is asked to hold it: an organisation's id here used to be a
	// silent miss, and is now a foreign key violation.
	organisation := seedOrganisation(t, pool)
	err = store.PublishTo(ctx, organisation, item("REQ-NOT-A-PERSON"))
	if err == nil {
		t.Fatal("publishing to an organisation's id succeeded")
	}
	if !strings.Contains(err.Error(), "person_items_user_id_fkey") &&
		!strings.Contains(err.Error(), "foreign key") {
		t.Errorf("refused for an unexpected reason: %v", err)
	}
}

// Somebody with no workspace of their own still receives.
//
// This is the change 00093 exists for, stated as a test. Before it, the publish
// function resolved the person to their personal workspace and refused if they
// had none — so a citizen who had never signed in, or an employee who only ever
// worked inside a company, could not be told anything. Their record is theirs
// now, and it needs no room to be put in.
func TestAPersonWithNoWorkspaceStillReceives(t *testing.T) {
	pool := openPool(t)
	store := person.New(pool)
	ctx := context.Background()

	geID, userID := seedPerson(t, pool)

	// Said out loud, because the whole test rests on it.
	var workspaces int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM registry.tenants WHERE owner_user_id = $1::uuid`, userID).
		Scan(&workspaces); err != nil {
		t.Fatal(err)
	}
	if workspaces != 0 {
		t.Fatalf("the person under test owns %d workspace(s); the test proves nothing", workspaces)
	}

	if err := store.Publish(ctx, geID, item("REQ-NO-HOME")); err != nil {
		t.Fatalf("publishing to a person with no workspace: %v", err)
	}

	var owner string
	if err := pool.QueryRow(ctx,
		`SELECT user_id::text FROM registry.person_items WHERE source_ref = 'REQ-NO-HOME'`).
		Scan(&owner); err != nil {
		t.Fatalf("the row is not there: %v", err)
	}
	if owner != userID {
		t.Errorf("the row landed on %s, want %s", owner, userID)
	}
}

// The only way in is the function.
//
// The tenant role holds SELECT and nothing else, so a module that decided to
// write the row itself — with the right person's id, having looked it up — is
// stopped by the database rather than by review. It also stops the citizen:
// this is the grant that keeps somebody from writing "the ministry approved it"
// into their own record, which row-level security would happily allow, since a
// policy that isolates people from each other has nothing to say about a person
// writing their own row.
//
// And the operator role cannot call the function at all: the console has no
// business publishing to a citizen, and an operator session that could would be
// a way to put words in front of somebody with no module and no audit row
// behind them.
func TestOnlyTheTenantRoleReachesTheFeedAndOnlyThroughTheFunction(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()
	_, userID := seedPerson(t, pool)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_user', $1, true)`, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `SET LOCAL ROLE gerege_nexus_tenant`); err != nil {
		t.Fatal(err)
	}

	// Reading their own rows: allowed, and the point of the feature.
	if _, err := tx.Exec(ctx, `SELECT 1 FROM registry.person_items LIMIT 1`); err != nil {
		t.Fatalf("the tenant role cannot read its own person_items: %v", err)
	}

	// Writing directly: refused, even onto themselves.
	_, err = tx.Exec(ctx,
		`INSERT INTO registry.person_items (user_id, source_app, source_ref, code, status)
		 VALUES ($1::uuid, 'a', 'b', 'c', 'd')`, userID)
	if err == nil {
		t.Error("the tenant role inserted into person_items directly")
	} else if !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("the direct insert was refused for an unexpected reason: %v", err)
	}
}

func TestTheOperatorRoleCannotPublish(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()
	_, userID := seedPerson(t, pool)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SET LOCAL ROLE gerege_nexus_operator`); err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(ctx,
		`SELECT registry.publish_person_item($1::uuid, NULL::uuid, 'app', 'ref', 'code', 'status', '')`, userID)
	if err == nil {
		t.Fatal("the operator role published to a citizen")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("refused for an unexpected reason: %v", err)
	}
}

// Row-level security, now keyed by the person.
//
// Until 00093 this worked because each citizen had a workspace of their own, so
// isolating by workspace isolated by person as a side effect. The key is the
// person now, and this is the test that says the substitution is real rather
// than nominal: one citizen's session sees their row and not the other's, with
// no workspace bound at all.
func TestOneCitizenDoesNotSeeAnothersRequests(t *testing.T) {
	pool := openPool(t)
	store := person.New(pool)
	ctx := context.Background()

	firstGeID, firstUser := seedPerson(t, pool)
	secondGeID, _ := seedPerson(t, pool)
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
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_user', $1, true)`, firstUser); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `SET LOCAL ROLE gerege_nexus_tenant`); err != nil {
		t.Fatal(err)
	}

	rows, err := tx.Query(ctx, `SELECT source_ref FROM registry.person_items`)
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
