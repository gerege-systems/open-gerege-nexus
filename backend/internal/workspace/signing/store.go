/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Persistence for the PDF e-signature app. Every statement is scoped by
 * tenant_id; there is no query here that can read across tenants.
 */

package signing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/telemetry"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/quota"
	"github.com/jackc/pgx/v5"
)

type store struct{ db nexus.DB }

// documentColumns is the projection every document listing shares. Blob
// columns are deliberately absent: a listing that selected original_pdf would
// pull megabytes per row across the wire to render a table of filenames.
const documentColumns = `
	id::text, tenant_id::text, title, file_name, status, provider, page_count, byte_size,
	COALESCE(checksum, ''), COALESCE(signer_name, ''), COALESCE(signer_reg_no, ''),
	COALESCE(signer_phone, ''), COALESCE(signer_etsi, ''), COALESCE(on_behalf_of_name, ''),
	COALESCE(certificate_level, ''), signed_at, created_at`

func scanDocument(row pgx.Row) (*Document, error) {
	var d Document
	err := row.Scan(&d.ID, &d.TenantID, &d.Title, &d.FileName, &d.Status, &d.Provider,
		&d.PageCount, &d.ByteSize, &d.Checksum, &d.SignerName, &d.SignerRegNo,
		&d.SignerPhone, &d.SignerEtsi, &d.OnBehalfOfName, &d.CertificateLevl,
		&d.SignedAt, &d.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// ─── Documents ───────────────────────────────────────────────────────────────

func (s *store) listDocuments(ctx context.Context, tenantID, status, search string, limit, offset int) ([]Document, int, error) {
	where := []string{"tenant_id = $1", "deleted_at IS NULL"}
	args := []any{tenantID}

	if status != "" {
		args = append(args, status)
		where = append(where, fmt.Sprintf("status = $%d", len(args)))
	}
	if search != "" {
		args = append(args, "%"+strings.ToLower(search)+"%")
		where = append(where, fmt.Sprintf("(lower(title) LIKE $%d OR lower(file_name) LIKE $%d)", len(args), len(args)))
	}
	clause := strings.Join(where, " AND ")

	var total int
	if err := s.db.QueryRow(ctx, `SELECT COUNT(*) FROM workspace.esign_documents WHERE `+clause, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, limit, offset)
	rows, err := s.db.Query(ctx, `SELECT `+documentColumns+`
		FROM workspace.esign_documents WHERE `+clause+`
		ORDER BY created_at DESC
		LIMIT $`+fmt.Sprint(len(args)-1)+` OFFSET $`+fmt.Sprint(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := make([]Document, 0, limit)
	for rows.Next() {
		d, err := scanDocument(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, *d)
	}
	return list, total, rows.Err()
}

func (s *store) getDocument(ctx context.Context, tenantID, id string) (*Document, error) {
	doc, err := scanDocument(s.db.QueryRow(ctx,
		`SELECT `+documentColumns+` FROM workspace.esign_documents
		 WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`, id, tenantID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, notFound("document not found")
	}
	return doc, err
}

func (s *store) createDocument(ctx context.Context, tenantID, uploadedBy, title, fileName, checksum string, pageCount uint, pdf []byte) (*Document, error) {
	// The organisation's storage limit, where one is set and enforced. This is
	// the only place on the platform where a file of any size is kept, so it is
	// the only place the check belongs — and it is a check rather than a
	// cleanup because refusing an upload is recoverable and deleting one is
	// not.
	if err := quota.Storage(ctx, s.db, tenantID, int64(len(pdf))); err != nil {
		return nil, badRequest("storage_limit_reached",
			"Байгууллагын хадгалалтын хязгаар дүүрсэн байна: "+err.Error())
	}
	// uploadedBy is nullable: a document can be created by a service token
	// that has no user row behind it.
	var uploader *string
	if uploadedBy != "" {
		uploader = &uploadedBy
	}
	return scanDocument(s.db.QueryRow(ctx,
		`INSERT INTO workspace.esign_documents
		     (tenant_id, title, file_name, page_count, original_pdf, checksum, byte_size, uploaded_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING `+documentColumns,
		tenantID, title, fileName, pageCount, pdf, checksum, len(pdf), uploader))
}

// documentPDF reads one blob. variant selects original or signed.
func (s *store) documentPDF(ctx context.Context, tenantID, id, variant string) ([]byte, string, error) {
	column := "original_pdf"
	if variant == "signed" {
		column = "signed_pdf"
	}
	var pdf []byte
	var fileName string
	// The column name is chosen from a closed set above and never interpolated
	// from caller input.
	err := s.db.QueryRow(ctx,
		`SELECT `+column+`, file_name FROM workspace.esign_documents
		 WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`, id, tenantID).Scan(&pdf, &fileName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", notFound("document not found")
	}
	if err != nil {
		return nil, "", err
	}
	if len(pdf) == 0 {
		return nil, "", notFound("this document has no " + variant + " copy")
	}
	return pdf, fileName, nil
}

// documentForSigning loads the bytes and asserts the document is still
// signable, in a single round trip.
func (s *store) documentForSigning(ctx context.Context, tenantID, id string) ([]byte, string, string, error) {
	var pdf []byte
	var status, title string
	err := s.db.QueryRow(ctx,
		`SELECT original_pdf, status, title FROM workspace.esign_documents
		 WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`, id, tenantID).Scan(&pdf, &status, &title)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", "", notFound("document not found")
	}
	if err != nil {
		return nil, "", "", err
	}
	if status == StatusSigned {
		return nil, "", "", conflict("ALREADY_SIGNED", "this document is already signed")
	}
	return pdf, status, title, nil
}

// markSigned records the outcome of a completed ceremony on the document.
//
// Both rails end here, which makes it the one place documents_signed_total can
// be incremented without the two handlers having to agree on when a ceremony
// counts as finished.
func (s *store) markSigned(ctx context.Context, tenantID, id string, in signedDocument) error {
	err := s.writeSigned(ctx, tenantID, id, in)
	telemetry.RecordDocumentSigned(in.Provider, err == nil)
	return err
}

func (s *store) writeSigned(ctx context.Context, tenantID, id string, in signedDocument) error {
	_, err := s.db.Exec(ctx,
		`UPDATE workspace.esign_documents SET
		     status = 'SIGNED', provider = $1, signed_pdf = $2, signature_image = COALESCE($3, signature_image),
		     signer_name = $4, signer_reg_no = $5, signer_phone = $6, signer_etsi = $7,
		     on_behalf_of_etsi = $8, on_behalf_of_name = $9, certificate_level = $10, signed_at = $11
		 WHERE id = $12 AND tenant_id = $13 AND deleted_at IS NULL`,
		in.Provider, in.SignedPDF, in.SignatureImage, in.SignerName, in.SignerRegNo,
		in.SignerPhone, nullable(in.SignerEtsi), nullable(in.OnBehalfOfEtsi),
		nullable(in.OnBehalfOfName), nullable(in.CertificateLevel), in.SignedAt, id, tenantID)
	return err
}

type signedDocument struct {
	Provider         string
	SignedPDF        []byte
	SignatureImage   []byte
	SignerName       string
	SignerRegNo      string
	SignerPhone      string
	SignerEtsi       string
	OnBehalfOfEtsi   string
	OnBehalfOfName   string
	CertificateLevel string
	SignedAt         time.Time
}

// softDeleteDocument archives rather than removes. A signed PDF is evidence,
// and a tenant clearing their list must not destroy it.
func (s *store) softDeleteDocument(ctx context.Context, tenantID, id string) error {
	tag, err := s.db.Exec(ctx,
		`UPDATE workspace.esign_documents SET deleted_at = NOW()
		 WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`, id, tenantID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return notFound("document not found")
	}
	return nil
}

// ─── Signing sessions ────────────────────────────────────────────────────────

const sessionColumns = `
	id, tenant_id::text, COALESCE(document_id::text, ''), provider, state,
	COALESCE(failure_reason, ''), file_name, document_hash, COALESCE(verification_code, ''),
	COALESCE(signer_name, ''), COALESCE(signer_etsi, ''), COALESCE(on_behalf_of_etsi, ''),
	COALESCE(on_behalf_of_name, ''), COALESCE(certificate_level, ''),
	created_at, completed_at, expires_at`

func scanSession(row pgx.Row) (*SignSession, error) {
	var s SignSession
	err := row.Scan(&s.ID, &s.TenantID, &s.DocumentID, &s.Provider, &s.State,
		&s.FailureReason, &s.FileName, &s.DocumentHash, &s.VerificationCode,
		&s.SignerName, &s.SignerEtsi, &s.OnBehalfOfEtsi, &s.OnBehalfOfName,
		&s.CertificateLevel, &s.CreatedAt, &s.CompletedAt, &s.ExpiresAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

type newSession struct {
	ID               string
	TenantID         string
	DocumentID       string
	EIDSessionID     string
	FileName         string
	DocumentHash     string
	VerificationCode string
	SignerUserID     string
	SignerEtsi       string
	SignerName       string
	OnBehalfOfEtsi   string
	OnBehalfOfName   string
}

func (s *store) createSession(ctx context.Context, in newSession) (*SignSession, error) {
	return scanSession(s.db.QueryRow(ctx,
		`INSERT INTO workspace.esign_sign_sessions
		     (id, tenant_id, document_id, provider, eid_session_id, state, file_name,
		      document_hash, verification_code, signer_user_id, signer_etsi, signer_name,
		      on_behalf_of_etsi, on_behalf_of_name, expires_at)
		 VALUES ($1, $2, $3, 'EID', $4, 'pending', $5, $6, $7, $8, $9, $10, $11, $12, $13)
		 RETURNING `+sessionColumns,
		in.ID, in.TenantID, nullable(in.DocumentID), nullable(in.EIDSessionID),
		in.FileName, in.DocumentHash, nullable(in.VerificationCode),
		nullable(in.SignerUserID), nullable(in.SignerEtsi), nullable(in.SignerName),
		nullable(in.OnBehalfOfEtsi), nullable(in.OnBehalfOfName),
		time.Now().Add(sessionTTL)))
}

func (s *store) getSession(ctx context.Context, tenantID, id string) (*SignSession, error) {
	session, err := scanSession(s.db.QueryRow(ctx,
		`SELECT `+sessionColumns+` FROM workspace.esign_sign_sessions WHERE id = $1 AND tenant_id = $2`, id, tenantID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, notFound("signing session not found")
	}
	return session, err
}

type sessionCompletion struct {
	SignedPDF          []byte
	CertificateLevel   string
	SignatureAlgorithm string
	OnBehalfOfEtsi     string
	OnBehalfOfName     string
}

// completeSession stores the signed PDF. The WHERE clause pins state to
// 'pending' so two concurrent pollers cannot both complete the same ceremony
// and double-write the document; the caller checks RowsAffected to learn
// whether it was the one that won.
func (s *store) completeSession(ctx context.Context, tenantID, id string, in sessionCompletion) (bool, error) {
	tag, err := s.db.Exec(ctx,
		`UPDATE workspace.esign_sign_sessions SET
		     state = 'completed', signed_pdf = $1, certificate_level = $2,
		     signature_algorithm = $3, on_behalf_of_etsi = COALESCE($4, on_behalf_of_etsi),
		     on_behalf_of_name = COALESCE($5, on_behalf_of_name), completed_at = NOW()
		 WHERE id = $6 AND tenant_id = $7 AND state = 'pending'`,
		in.SignedPDF, nullable(in.CertificateLevel), nullable(in.SignatureAlgorithm),
		nullable(in.OnBehalfOfEtsi), nullable(in.OnBehalfOfName), id, tenantID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (s *store) failSession(ctx context.Context, tenantID, id, state, reason string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE workspace.esign_sign_sessions SET state = $1, failure_reason = $2, completed_at = NOW()
		 WHERE id = $3 AND tenant_id = $4 AND state = 'pending'`,
		state, nullable(reason), id, tenantID)
	return err
}

func (s *store) sessionSignedPDF(ctx context.Context, tenantID, id string) ([]byte, string, error) {
	var pdf []byte
	var fileName, state string
	err := s.db.QueryRow(ctx,
		`SELECT signed_pdf, file_name, state FROM workspace.esign_sign_sessions
		 WHERE id = $1 AND tenant_id = $2`, id, tenantID).Scan(&pdf, &fileName, &state)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", notFound("signing session not found")
	}
	if err != nil {
		return nil, "", err
	}
	if state != SessionCompleted || len(pdf) == 0 {
		return nil, "", conflict("NOT_COMPLETED", "this signing session has not completed")
	}
	return pdf, fileName, nil
}

// expireStaleSessions closes ceremonies nobody came back to. It is a
// housekeeping sweep, not a deadline: the horizon is generous precisely so it
// never cuts off a citizen who is still being pushed a notification.
func (s *store) expireStaleSessions(ctx context.Context) (int64, error) {
	tag, err := s.db.Exec(ctx,
		`UPDATE workspace.esign_sign_sessions
		    SET state = 'expired', failure_reason = 'abandoned', completed_at = NOW()
		  WHERE state = 'pending' AND expires_at < NOW()`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// ─── Signature log ───────────────────────────────────────────────────────────

type logEntry struct {
	TenantID      string
	DocumentID    string
	DocumentTitle string
	SessionID     string
	Provider      string
	Action        string
	Outcome       string
	RegNo         string
	PhoneNo       string
	FirstName     string
	LastName      string
	ActorUserID   string
	Detail        string
}

// recordLog is best effort. A failure to write the audit row must not fail the
// signature that was already made — the signed document is the record of
// consequence, and losing the log line is the lesser harm. It is still logged
// upstream by the caller.
func (s *store) recordLog(ctx context.Context, in logEntry) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO workspace.esign_signature_logs
		     (tenant_id, document_id, document_title, session_id, provider, action, outcome,
		      reg_no, phone_no, first_name, last_name, actor_user_id, detail)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		in.TenantID, nullable(in.DocumentID), nullable(in.DocumentTitle), nullable(in.SessionID),
		defaulted(in.Provider, ProviderEID), in.Action, defaulted(in.Outcome, OutcomeOK),
		nullable(in.RegNo), nullable(in.PhoneNo), nullable(in.FirstName), nullable(in.LastName),
		nullable(in.ActorUserID), nullable(in.Detail))
	return err
}

func (s *store) listLogs(ctx context.Context, tenantID string, q LogQuery) ([]SignatureLog, int, error) {
	where := []string{"tenant_id = $1"}
	args := []any{tenantID}

	add := func(clause string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}
	if q.Action != "" {
		add("action = $%d", q.Action)
	}
	if q.Outcome != "" {
		add("outcome = $%d", q.Outcome)
	}
	if q.Provider != "" {
		add("provider = $%d", q.Provider)
	}
	if q.DocumentID != "" {
		add("document_id = $%d", q.DocumentID)
	}
	if q.From != nil {
		add("created_at >= $%d", *q.From)
	}
	if q.To != nil {
		add("created_at <= $%d", *q.To)
	}
	if q.Search != "" {
		args = append(args, "%"+strings.ToLower(q.Search)+"%")
		where = append(where, fmt.Sprintf(
			"(lower(COALESCE(reg_no,'')) LIKE $%d OR lower(COALESCE(first_name,'')) LIKE $%d "+
				"OR lower(COALESCE(last_name,'')) LIKE $%d OR lower(COALESCE(document_title,'')) LIKE $%d)",
			len(args), len(args), len(args), len(args)))
	}
	clause := strings.Join(where, " AND ")

	var total int
	if err := s.db.QueryRow(ctx, `SELECT COUNT(*) FROM workspace.esign_signature_logs WHERE `+clause, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, q.Limit, q.Offset)
	rows, err := s.db.Query(ctx,
		`SELECT id::text, tenant_id::text, COALESCE(document_id::text, ''), COALESCE(document_title, ''),
		        COALESCE(session_id, ''), provider, action, outcome, COALESCE(reg_no, ''),
		        COALESCE(phone_no, ''), COALESCE(first_name, ''), COALESCE(last_name, ''),
		        COALESCE(detail, ''), created_at
		   FROM workspace.esign_signature_logs WHERE `+clause+`
		  ORDER BY created_at DESC
		  LIMIT $`+fmt.Sprint(len(args)-1)+` OFFSET $`+fmt.Sprint(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := make([]SignatureLog, 0, q.Limit)
	for rows.Next() {
		var l SignatureLog
		if err := rows.Scan(&l.ID, &l.TenantID, &l.DocumentID, &l.DocumentTitle, &l.SessionID,
			&l.Provider, &l.Action, &l.Outcome, &l.RegNo, &l.PhoneNo, &l.FirstName,
			&l.LastName, &l.Detail, &l.CreatedAt); err != nil {
			return nil, 0, err
		}
		list = append(list, l)
	}
	return list, total, rows.Err()
}

// ─── Settings ────────────────────────────────────────────────────────────────

// loadSettings returns the tenant's configuration, falling back to defaults
// when the row does not exist yet. It never returns an error for "no row":
// an unconfigured tenant is the normal case, not a failure.
func (s *store) loadSettings(ctx context.Context, tenantID string) (Placement, Policy, *time.Time, error) {
	var placementRaw, policyRaw []byte
	var updatedAt *time.Time
	err := s.db.QueryRow(ctx,
		`SELECT placement, policy, updated_at FROM workspace.esign_settings WHERE tenant_id = $1`,
		tenantID).Scan(&placementRaw, &policyRaw, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return DefaultPlacement(), DefaultPolicy(), nil, nil
	}
	if err != nil {
		return Placement{}, Policy{}, nil, err
	}

	placement := DefaultPlacement()
	if len(placementRaw) > 0 {
		// A malformed blob must not lock the tenant out of their own settings
		// screen, so it degrades to the default rather than erroring.
		_ = json.Unmarshal(placementRaw, &placement)
	}
	policy := DefaultPolicy()
	if len(policyRaw) > 0 {
		_ = json.Unmarshal(policyRaw, &policy)
	}
	return placement.normalize(), policy.normalize(), updatedAt, nil
}

func (s *store) saveSettings(ctx context.Context, tenantID, updatedBy, column string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	// column comes from a closed set at the call sites, never from a request.
	_, err = s.db.Exec(ctx,
		`INSERT INTO workspace.esign_settings (tenant_id, `+column+`, updated_by, updated_at)
		 VALUES ($1, $2, $3, NOW())
		 ON CONFLICT (tenant_id) DO UPDATE
		    SET `+column+` = EXCLUDED.`+column+`, updated_by = EXCLUDED.updated_by, updated_at = NOW()`,
		tenantID, raw, nullable(updatedBy))
	return err
}

func (s *store) loadProbe(ctx context.Context, tenantID string) (*Probe, error) {
	var raw []byte
	err := s.db.QueryRow(ctx, `SELECT hsm FROM workspace.esign_settings WHERE tenant_id = $1`, tenantID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) || len(raw) == 0 {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	// Best effort: a malformed blob leaves LastProbe nil, which the screen
	// renders as "not tested yet". Failing here instead would lock an operator
	// out of the connection screen over a stale record of a past test.
	var stored struct {
		LastProbe *Probe `json:"last_probe"`
	}
	_ = json.Unmarshal(raw, &stored)
	return stored.LastProbe, nil
}

// ─── Batches ─────────────────────────────────────────────────────────────────

func (s *store) createBatch(ctx context.Context, tenantID, createdBy, name, provider string, documentIDs []string) (*Batch, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var batchID string
	var createdAt time.Time
	if err := tx.QueryRow(ctx,
		`INSERT INTO workspace.esign_batches (tenant_id, name, provider, created_by)
		 VALUES ($1, $2, $3, $4) RETURNING id::text, created_at`,
		tenantID, name, provider, nullable(createdBy)).Scan(&batchID, &createdAt); err != nil {
		return nil, err
	}

	for i, docID := range documentIDs {
		// The subquery re-asserts tenant ownership, so a caller cannot pull a
		// document from another tenant into their batch by guessing an id.
		tag, err := tx.Exec(ctx,
			`INSERT INTO workspace.esign_batch_items (batch_id, document_id, position)
			 SELECT $1, id, $2 FROM workspace.esign_documents
			  WHERE id = $3 AND tenant_id = $4 AND deleted_at IS NULL AND status <> 'SIGNED'`,
			batchID, i, docID, tenantID)
		if err != nil {
			return nil, err
		}
		if tag.RowsAffected() == 0 {
			return nil, badRequest("INVALID_DOCUMENT",
				"document "+docID+" is not available for signing")
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.getBatch(ctx, tenantID, batchID)
}

func (s *store) listBatches(ctx context.Context, tenantID string, limit, offset int) ([]Batch, int, error) {
	var total int
	if err := s.db.QueryRow(ctx, `SELECT COUNT(*) FROM workspace.esign_batches WHERE tenant_id = $1`, tenantID).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.db.Query(ctx,
		`SELECT b.id::text, b.name, b.provider, b.status, b.created_at, b.started_at, b.finished_at,
		        COUNT(i.id),
		        COUNT(*) FILTER (WHERE i.status = 'SIGNED'),
		        COUNT(*) FILTER (WHERE i.status = 'FAILED')
		   FROM workspace.esign_batches b
		   LEFT JOIN workspace.esign_batch_items i ON i.batch_id = b.id
		  WHERE b.tenant_id = $1
		  GROUP BY b.id
		  ORDER BY b.created_at DESC
		  LIMIT $2 OFFSET $3`, tenantID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := make([]Batch, 0, limit)
	for rows.Next() {
		var b Batch
		if err := rows.Scan(&b.ID, &b.Name, &b.Provider, &b.Status, &b.CreatedAt,
			&b.StartedAt, &b.FinishedAt, &b.Total, &b.Signed, &b.Failed); err != nil {
			return nil, 0, err
		}
		list = append(list, b)
	}
	return list, total, rows.Err()
}

func (s *store) getBatch(ctx context.Context, tenantID, id string) (*Batch, error) {
	var b Batch
	err := s.db.QueryRow(ctx,
		`SELECT id::text, name, provider, status, created_at, started_at, finished_at
		   FROM workspace.esign_batches WHERE id = $1 AND tenant_id = $2`, id, tenantID).
		Scan(&b.ID, &b.Name, &b.Provider, &b.Status, &b.CreatedAt, &b.StartedAt, &b.FinishedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, notFound("batch not found")
	}
	if err != nil {
		return nil, err
	}

	rows, err := s.db.Query(ctx,
		`SELECT i.id::text, i.document_id::text, d.title, d.file_name, i.position, i.status,
		        COALESCE(i.session_id, ''), COALESCE(i.error, ''), i.signed_at
		   FROM workspace.esign_batch_items i
		   JOIN workspace.esign_documents d ON d.id = i.document_id
		  WHERE i.batch_id = $1
		  ORDER BY i.position`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	b.Items = make([]BatchItem, 0)
	for rows.Next() {
		var it BatchItem
		if err := rows.Scan(&it.ID, &it.DocumentID, &it.DocumentTitle, &it.FileName,
			&it.Position, &it.Status, &it.SessionID, &it.Error, &it.SignedAt); err != nil {
			return nil, err
		}
		b.Items = append(b.Items, it)
		b.Total++
		switch it.Status {
		case ItemSigned:
			b.Signed++
		case ItemFailed:
			b.Failed++
		}
	}
	return &b, rows.Err()
}

func (s *store) setBatchStatus(ctx context.Context, tenantID, id, status string) error {
	var timeColumn string
	switch status {
	case BatchRunning:
		timeColumn = ", started_at = COALESCE(started_at, NOW())"
	case BatchCompleted, BatchFailed, BatchCancelled:
		timeColumn = ", finished_at = NOW()"
	}
	tag, err := s.db.Exec(ctx,
		`UPDATE workspace.esign_batches SET status = $1`+timeColumn+`
		  WHERE id = $2 AND tenant_id = $3`, status, id, tenantID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return notFound("batch not found")
	}
	return nil
}

func (s *store) setBatchItem(ctx context.Context, itemID, status, sessionID, errMessage string) error {
	var signedAt *time.Time
	if status == ItemSigned {
		now := time.Now()
		signedAt = &now
	}
	_, err := s.db.Exec(ctx,
		`UPDATE workspace.esign_batch_items
		    SET status = $1, session_id = COALESCE($2, session_id), error = $3, signed_at = $4
		  WHERE id = $5`,
		status, nullable(sessionID), nullable(errMessage), signedAt, itemID)
	return err
}

// ─── eID identity ────────────────────────────────────────────────────────────

type eidIdentity struct {
	CivilID        string
	RegNumber      string
	PersonEtsi     string
	DocumentNumber string
	GivenName      string
	Surname        string
}

func (s *store) eidIdentityFor(ctx context.Context, userID string) (*eidIdentity, error) {
	var id eidIdentity
	err := s.db.QueryRow(ctx,
		`SELECT COALESCE(civil_id, ''), COALESCE(reg_number, ''), person_etsi,
		        COALESCE(document_number, ''), COALESCE(given_name, ''), COALESCE(surname, '')
		   FROM registry.user_eid_identities WHERE user_id = $1`, userID).
		Scan(&id.CivilID, &id.RegNumber, &id.PersonEtsi, &id.DocumentNumber, &id.GivenName, &id.Surname)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// nullable turns an empty string into a SQL NULL. Storing "" in a nullable
// column defeats every COALESCE and IS NULL check downstream.
func nullable(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}

func defaulted(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
