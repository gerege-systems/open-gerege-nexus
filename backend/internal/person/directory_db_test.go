/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * What the directory shows, and the three things it must never show.
 */

package person_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/person"
)

func publish(t *testing.T, pool *pgxpool.Pool, tenantID, code, title string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO registry.service_directory (tenant_id, code, title) VALUES ($1::uuid, $2, $3)`,
		tenantID, code, title); err != nil {
		t.Fatal(err)
	}
}

// Published is listed; unpublished is not.
//
// The second half is the whole opt-in rule: an organisation that handles a kind
// of request has said something about its own queue, and appearing in a
// deployment-wide list is a promise to strangers. The table is the promise, so
// a row that is not there is an organisation that has not made it.
func TestOnlyWhatWasPublishedIsListed(t *testing.T) {
	pool := openPool(t)
	store := person.New(pool)
	ctx := context.Background()

	loud, loudSlug := openOrganisation(t, pool)
	_, quietSlug := openOrganisation(t, pool)
	code := "D-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:8]
	publish(t, pool, loud, code, "Жолооны үнэмлэх сунгах")

	found, err := store.Directory(ctx, code, 0)
	if err != nil {
		t.Fatal(err)
	}
	var slugs []string
	for _, one := range found {
		slugs = append(slugs, one.Slug)
	}
	if len(slugs) != 1 || slugs[0] != loudSlug {
		t.Fatalf("the directory returned %v, want only %s", slugs, loudSlug)
	}
	for _, one := range found {
		if one.Slug == quietSlug {
			t.Error("an organisation that published nothing is in the directory")
		}
	}
	if found[0].Title == "" {
		t.Error("the organisation's own words for the code did not come back")
	}
}

// A local. code cannot be published, and the refusal is the schema's.
//
// Go checks it too — the administrator gets a sentence rather than a constraint
// name — but this asserts the constraint, because the Go check is the one that
// will be forgotten. 00062 wrote that down and 00090 followed it.
func TestALocalCodeCannotBePublished(t *testing.T) {
	pool := openPool(t)
	orgID, _ := openOrganisation(t, pool)

	_, err := pool.Exec(context.Background(),
		`INSERT INTO registry.service_directory (tenant_id, code) VALUES ($1::uuid, 'local.secret')`,
		orgID)
	if err == nil {
		t.Fatal("a local. code was published")
	}
	if !strings.Contains(err.Error(), "service_directory_no_local_namespace") {
		t.Errorf("refused for an unexpected reason: %v", err)
	}
}

// A suspended organisation is not answering, so it is not offered.
//
// 00089 refuses a request to one, and a directory that listed it would be
// inviting somebody to press a button that cannot work.
func TestASuspendedOrganisationIsNotOffered(t *testing.T) {
	pool := openPool(t)
	store := person.New(pool)
	ctx := context.Background()

	orgID, _ := openOrganisation(t, pool)
	code := "S-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:8]
	publish(t, pool, orgID, code, "")

	if found, err := store.Directory(ctx, code, 0); err != nil || len(found) != 1 {
		t.Fatalf("before suspension the directory returned %d entries (%v)", len(found), err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE registry.tenants SET suspended_at = NOW() WHERE id = $1::uuid`, orgID); err != nil {
		t.Fatal(err)
	}
	found, err := store.Directory(ctx, code, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 0 {
		t.Errorf("a suspended organisation is still offered: %v", found)
	}
}

// One organisation cannot publish in another's name.
//
// registry has no row-level security on most of its tables, so this one says
// what would otherwise be missing: the policy reads wide and writes narrow, and
// the narrow half is what stops a session claiming somebody else's services.
func TestAnOrganisationCannotPublishForAnother(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()
	mine, _ := openOrganisation(t, pool)
	theirs, _ := openOrganisation(t, pool)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true)`, mine); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `SET LOCAL ROLE gerege_nexus_tenant`); err != nil {
		t.Fatal(err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO registry.service_directory (tenant_id, code) VALUES ($1::uuid, 'D-OWN')`,
		mine); err != nil {
		t.Fatalf("an organisation cannot publish its own service: %v", err)
	}
	// The read half stays wide: finding somebody else is the point. Asked
	// before the refusal below, because a failed statement aborts the
	// transaction and every later one in it — which would report this as a
	// second failure rather than the one it is.
	var visible int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM registry.service_directory`).Scan(&visible); err != nil {
		t.Fatalf("a session cannot read the directory: %v", err)
	}
	if visible == 0 {
		t.Error("the directory reads as empty from inside an organisation")
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO registry.service_directory (tenant_id, code) VALUES ($1::uuid, 'D-THEIRS')`,
		theirs)
	if err == nil {
		t.Error("an organisation published a service in another's name")
	} else if !strings.Contains(err.Error(), "row-level security") {
		t.Errorf("the cross-organisation write was refused for an unexpected reason: %v", err)
	}
}
