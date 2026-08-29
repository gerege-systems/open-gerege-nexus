/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package signing

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// What the signing store promises, asked of a database.
//
// The file it sits beside says it in one line — "every statement here is scoped
// by tenant_id; there is no query here that can read across tenants" — and
// until now nothing checked it. It cannot be checked anywhere else: the
// scoping is in the WHERE clauses, the batch's refusal is a subquery's row
// count, the archive is a column rather than a DELETE, and the sweep is a
// predicate over three states. Every one of those is a claim about what
// PostgreSQL does with these statements.
//
// This is also the app whose output is evidence. A signature that the wrong
// organisation can read, or that a sweep expires while somebody is still
// holding their card to the reader, is not a rendering bug.

func testStore(t *testing.T) (*store, *pgxpool.Pool) {
	t.Helper()
	dsn := os.Getenv("SIGNING_TEST_DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("DOCUMENTS_TEST_DATABASE_URL")
	}
	if dsn == "" {
		dsn = os.Getenv("TEST_DATABASE_URL")
	}
	if dsn == "" {
		t.Skip("no signing test database is configured")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return &store{db: pool}, pool
}

// tenant makes an organisation for one test and takes it away afterwards.
// Everything this file writes hangs off it, so the cleanup is one statement.
func tenant(t *testing.T, pool *pgxpool.Pool, suffix string) string {
	t.Helper()
	slug := fmt.Sprintf("signing-%s-%d", suffix, time.Now().UnixNano())
	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO registry.tenants (slug, name) VALUES ($1, $1) RETURNING id::text`, slug).Scan(&id); err != nil {
		t.Fatalf("create an organisation: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM registry.tenants WHERE id = $1::uuid`, id)
	})
	return id
}

// ceremonyID is the shape the signing library issues and sessionIDPattern
// guards: sixteen random bytes, hex. A test id of another shape would be one
// the browser refuses to poll.
func ceremonyID(t *testing.T) string {
	t.Helper()
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		t.Fatal(err)
	}
	id := hex.EncodeToString(buf)
	if !sessionIDPattern.MatchString(id) {
		t.Fatalf("the test made an id the app would refuse: %q", id)
	}
	return id
}

func document(t *testing.T, s *store, tenantID, title string) *Document {
	t.Helper()
	doc, err := s.createDocument(context.Background(), tenantID, "", title, title+".pdf",
		"sha256:"+title, 1, []byte("%PDF-1.4 "+title))
	if err != nil {
		t.Fatalf("create a document: %v", err)
	}
	return doc
}

// The one-line promise at the top of store.go, asked of every read a document
// can be reached through.
func TestADocumentIsInvisibleToAnotherOrganisation(t *testing.T) {
	s, pool := testStore(t)
	ctx := context.Background()
	mine := tenant(t, pool, "mine")
	theirs := tenant(t, pool, "theirs")
	doc := document(t, s, mine, "quarterly-report")

	if _, err := s.getDocument(ctx, mine, doc.ID); err != nil {
		t.Fatalf("the owner cannot read their own document: %v", err)
	}
	if _, err := s.getDocument(ctx, theirs, doc.ID); err == nil {
		t.Error("a sibling organisation read the document")
	}
	if _, _, err := s.documentPDF(ctx, theirs, doc.ID, "original"); err == nil {
		t.Error("a sibling organisation read the file")
	}
	if err := s.softDeleteDocument(ctx, theirs, doc.ID); err == nil {
		t.Error("a sibling organisation deleted the document")
	}

	list, total, err := s.listDocuments(ctx, theirs, "", "", 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 || len(list) != 0 {
		t.Errorf("a sibling organisation's listing has %d rows (total %d)", len(list), total)
	}
}

// The comment above the subquery in createBatch says a caller cannot pull
// another organisation's document into their batch by guessing an id. This is
// that sentence, run.
func TestABatchCannotBorrowAnotherOrganisationsDocument(t *testing.T) {
	s, pool := testStore(t)
	ctx := context.Background()
	mine := tenant(t, pool, "batch-mine")
	theirs := tenant(t, pool, "batch-theirs")
	ours := document(t, s, mine, "ours")
	stolen := document(t, s, theirs, "theirs")

	if _, err := s.createBatch(ctx, mine, "", "borrowed", "EID", []string{ours.ID, stolen.ID}); err == nil {
		t.Fatal("a batch was created across two organisations")
	}

	// And nothing was left behind: the failure happens inside the transaction
	// that made the batch row, so a rolled-back attempt must not leave an empty
	// batch in somebody's list.
	_, total, err := s.listBatches(ctx, mine, 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 {
		t.Errorf("the failed batch left %d rows behind", total)
	}
}

// A document that is already signed is not available for signing again. The
// rule is in the same subquery, and it is the one that stops a batch from
// re-stamping evidence.
func TestABatchRefusesADocumentThatIsAlreadySigned(t *testing.T) {
	s, pool := testStore(t)
	ctx := context.Background()
	mine := tenant(t, pool, "batch-signed")
	doc := document(t, s, mine, "already-signed")

	if err := s.markSigned(ctx, mine, doc.ID, signedDocument{
		Provider: "EID", SignedPDF: []byte("%PDF-1.4 signed"), SignerName: "Б. Баяр",
		SignerRegNo: "УБ00000000", SignedAt: time.Now(),
	}); err != nil {
		t.Fatalf("sign the document: %v", err)
	}
	if _, err := s.createBatch(ctx, mine, "", "again", "EID", []string{doc.ID}); err == nil {
		t.Error("a signed document was put in a batch to be signed again")
	}
}

// "A signed PDF is evidence, and a tenant clearing their list must not destroy
// it" — softDeleteDocument's own comment.
func TestDeletingADocumentArchivesTheEvidence(t *testing.T) {
	s, pool := testStore(t)
	ctx := context.Background()
	mine := tenant(t, pool, "archive")
	doc := document(t, s, mine, "receipt")
	if err := s.markSigned(ctx, mine, doc.ID, signedDocument{
		Provider: "EID", SignedPDF: []byte("%PDF-1.4 signed evidence"), SignerName: "Б. Баяр",
		SignerRegNo: "УБ00000000", SignedAt: time.Now(),
	}); err != nil {
		t.Fatalf("sign the document: %v", err)
	}
	if err := s.softDeleteDocument(ctx, mine, doc.ID); err != nil {
		t.Fatalf("delete the document: %v", err)
	}

	if _, err := s.getDocument(ctx, mine, doc.ID); err == nil {
		t.Error("a deleted document is still readable through the app")
	}
	_, total, err := s.listDocuments(ctx, mine, "", "", 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 {
		t.Errorf("a deleted document is still in the listing (%d rows)", total)
	}

	var kept []byte
	if err := pool.QueryRow(ctx,
		`SELECT signed_pdf FROM workspace.esign_documents WHERE id = $1::uuid`, doc.ID).Scan(&kept); err != nil {
		t.Fatalf("the row itself is gone: %v", err)
	}
	if len(kept) == 0 {
		t.Error("the signed file was destroyed by a delete that promised to archive it")
	}

	// Deleting it twice is not an error the second time round for the caller to
	// interpret as "somebody else's document": it is the same not-found.
	if err := s.softDeleteDocument(ctx, mine, doc.ID); err == nil {
		t.Error("deleting an already deleted document reported success")
	}
}

// The sweep must reach abandoned ceremonies and nothing else. A ceremony that
// is still within its twenty minutes belongs to somebody standing at a card
// reader.
func TestOnlyAbandonedCeremoniesExpire(t *testing.T) {
	s, pool := testStore(t)
	ctx := context.Background()
	mine := tenant(t, pool, "sweep")

	live, err := s.createSession(ctx, newSession{
		ID: ceremonyID(t), TenantID: mine, FileName: "live.pdf", DocumentHash: "sha256:live",
	})
	if err != nil {
		t.Fatalf("create the live session: %v", err)
	}
	stale, err := s.createSession(ctx, newSession{
		ID: ceremonyID(t), TenantID: mine, FileName: "stale.pdf", DocumentHash: "sha256:stale",
	})
	if err != nil {
		t.Fatalf("create the stale session: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE workspace.esign_sign_sessions SET expires_at = NOW() - INTERVAL '1 minute' WHERE id = $1`,
		stale.ID); err != nil {
		t.Fatalf("age the session: %v", err)
	}

	if _, err := s.expireStaleSessions(ctx); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	after, err := s.getSession(ctx, mine, stale.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.State != "expired" {
		t.Errorf("an abandoned ceremony is still %q", after.State)
	}
	untouched, err := s.getSession(ctx, mine, live.ID)
	if err != nil {
		t.Fatal(err)
	}
	if untouched.State != "pending" {
		t.Errorf("a live ceremony was swept: %q", untouched.State)
	}

	// And an expired ceremony can no longer be completed, which is the point of
	// expiring it: completeSession only moves a pending row.
	done, err := s.completeSession(ctx, mine, stale.ID, sessionCompletion{SignedPDF: []byte("%PDF-1.4")})
	if err != nil {
		t.Fatal(err)
	}
	if done {
		t.Error("an expired ceremony was completed anyway")
	}
}

// Signing writes the signer onto the document, and onto that document only.
func TestSigningRecordsTheSignerAndTouchesNobodyElsesRow(t *testing.T) {
	s, pool := testStore(t)
	ctx := context.Background()
	mine := tenant(t, pool, "sign-mine")
	theirs := tenant(t, pool, "sign-theirs")
	doc := document(t, s, mine, "contract")
	sibling := document(t, s, theirs, "sibling-contract")

	signedAt := time.Now().Truncate(time.Second)
	if err := s.markSigned(ctx, mine, doc.ID, signedDocument{
		Provider: "EID", SignedPDF: []byte("%PDF-1.4 signed"), SignerName: "Б. Баяр",
		SignerRegNo: "УБ00000000", SignerEtsi: "MNIDCI-УБ00000000", SignedAt: signedAt,
	}); err != nil {
		t.Fatalf("sign: %v", err)
	}

	after, err := s.getDocument(ctx, mine, doc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Status != "SIGNED" || after.SignerName != "Б. Баяр" || after.SignedAt == nil {
		t.Errorf("the signature was not recorded: status=%q signer=%q at=%v",
			after.Status, after.SignerName, after.SignedAt)
	}

	// The other organisation's document is untouched — same statement, same
	// id space, different tenant.
	other, err := s.getDocument(ctx, theirs, sibling.ID)
	if err != nil {
		t.Fatal(err)
	}
	if other.Status == "SIGNED" {
		t.Error("signing one organisation's document signed another's")
	}

	// Signing on behalf of the wrong organisation changes nothing at all.
	if err := s.markSigned(ctx, theirs, doc.ID, signedDocument{
		Provider: "EID", SignedPDF: []byte("%PDF-1.4 forged"), SignerName: "Хэн нэгэн",
		SignerRegNo: "УБ99999999", SignedAt: time.Now(),
	}); err != nil {
		t.Fatalf("the statement itself failed: %v", err)
	}
	unchanged, err := s.getDocument(ctx, mine, doc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.SignerName != "Б. Баяр" {
		t.Errorf("a sibling organisation overwrote the signer: %q", unchanged.SignerName)
	}
}

// The listing's filters are SQL, and a filter that quietly matches everything
// is a screen that shows another status than the one asked for.
func TestTheListingFiltersByStatusAndSearchesTitleAndFileName(t *testing.T) {
	s, pool := testStore(t)
	ctx := context.Background()
	mine := tenant(t, pool, "listing")
	first := document(t, s, mine, "gerege-invoice")
	document(t, s, mine, "other-paper")

	if err := s.markSigned(ctx, mine, first.ID, signedDocument{
		Provider: "EID", SignedPDF: []byte("%PDF-1.4"), SignerName: "Б. Баяр",
		SignerRegNo: "УБ00000000", SignedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	signed, total, err := s.listDocuments(ctx, mine, "SIGNED", "", 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(signed) != 1 || signed[0].ID != first.ID {
		t.Errorf("the status filter returned %d rows (total %d)", len(signed), total)
	}

	// Case-insensitive, and it reaches the file name as well as the title.
	found, total, err := s.listDocuments(ctx, mine, "", "GEREGE", 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(found) != 1 || found[0].ID != first.ID {
		t.Errorf("the search returned %d rows (total %d)", len(found), total)
	}

	none, total, err := s.listDocuments(ctx, mine, "", "nothing-like-this", 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 || len(none) != 0 {
		t.Errorf("a search for nothing returned %d rows (total %d)", len(none), total)
	}
}
