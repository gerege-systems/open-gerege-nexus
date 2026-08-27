/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * eID Mongolia qualified remote signing:
 *
 *   POST /esign/sign/init            upload, push the PIN2 prompt, return a
 *                                    verification code
 *   GET  /esign/sign/{id}            poll until completed / rejected / expired
 *   GET  /esign/sign/{id}/download   stream the PAdES-signed PDF
 *
 * The ceremony itself belongs to the platform's own signing packages
 * (internal/workspace/identity/eidmongolia over internal/kernel/eidsign): it talks to eID, holds
 * the document, checks session ownership and produces the PAdES output. What
 * lives here is the part the library has no view of — which tenant, which
 * document, which batch, and the audit trail.
 */

package signing

import (
	"errors"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/eidsign"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/identity/eidmongolia"
	"github.com/go-chi/chi/v5"
)

// sessionIDPattern matches the identifier the library issues (32 lowercase
// hex). The browser validates it before polling, so a mistyped URL never
// reaches the database.
var sessionIDPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

// stateFromLibrary maps the library's vocabulary onto the platform's.
//
// The two differ in one word: the library says "running" where this app's
// stored state and its browser both say "pending". Mapping is cheaper than
// migrating a CHECK constraint on a live table and rewriting the polling view
// for a synonym.
func stateFromLibrary(state string) string {
	switch state {
	case eidmongolia.StateRunning:
		return SessionPending
	case eidmongolia.StateCompleted:
		return SessionCompleted
	case eidmongolia.StateRejected:
		return SessionRejected
	case eidmongolia.StateExpired:
		return SessionExpired
	default:
		return SessionFailed
	}
}

// signInitHandler starts a ceremony, from either a multipart upload or a JSON
// body naming a document already in the store.
func (m *Rails) signInitHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, actor, ok := m.require(w, r, PermSign)
	if !ok {
		return
	}
	if !m.eid.Enabled() {
		writeDomainError(w, &Error{
			Code:    "EID_NOT_CONFIGURED",
			Message: "eID Mongolia signing is not configured on this deployment",
			Status:  http.StatusServiceUnavailable,
		})
		return
	}

	_, policy, _, err := m.store.loadSettings(r.Context(), tenantID)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	pdf, fileName, documentID, onBehalfOf, bodySignerID, err := m.readSignInput(r, tenantID, policy)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if onBehalfOf != "" && !policy.AllowOnBehalfOf {
		writeDomainError(w, forbidden("this tenant does not allow signing on behalf of an organisation"))
		return
	}

	// Who signs. A linked eID account needs no input; anything else names the
	// citizen explicitly, and a typo there would push the PIN2 prompt at
	// somebody else's phone, so the value is validated rather than trusted.
	// r.FormValue only reaches a multipart body, so a JSON caller names the
	// signer in the payload instead. Without this the document-by-id route
	// could only ever sign as the linked account, and an unlinked one had no
	// way through at all.
	signerEtsi := actor.Etsi
	if raw := firstNonBlank(r.FormValue("signer_id"), bodySignerID); raw != "" {
		signerEtsi = eidmongolia.PersonEtsi(raw)
	}
	if signerEtsi == "" {
		writeDomainError(w, badRequest("NO_SIGNER_IDENTITY",
			"this account is not linked to eID Mongolia; sign in with eID or supply signer_id"))
		return
	}
	if err := validateEtsi(signerEtsi); err != nil {
		writeDomainError(w, err)
		return
	}
	regNo := civilIDFromEtsi(signerEtsi)

	started, err := m.eid.SignPDF(r.Context(), eidmongolia.SignRequest{
		RegNo:         regNo,
		FullName:      actor.FullName,
		FileName:      fileName,
		PDF:           pdf,
		OnBehalfOfOrg: onBehalfOf,
	})
	if err != nil {
		m.log(r, logEntry{
			TenantID: tenantID, DocumentID: documentID, Provider: ProviderEID,
			Action: ActionSignStart, Outcome: OutcomeFailed, RegNo: signerEtsi,
			ActorUserID: actor.UserID, Detail: err.Error(),
		})
		writeDomainError(w, translateEIDError(err))
		return
	}

	// The platform-side record. The document itself stays with the library for the
	// life of the ceremony, so this row is metadata only.
	session, err := m.store.createSession(r.Context(), newSession{
		ID:               started.SessionID,
		TenantID:         tenantID,
		DocumentID:       documentID,
		EIDSessionID:     started.SessionID,
		FileName:         started.Filename,
		DocumentHash:     started.DocumentHash,
		VerificationCode: started.VerificationCode,
		SignerUserID:     actor.UserID,
		SignerEtsi:       signerEtsi,
		SignerName:       actor.FullName,
		OnBehalfOfEtsi:   onBehalfOf,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}

	m.log(r, logEntry{
		TenantID: tenantID, DocumentID: documentID, SessionID: started.SessionID,
		Provider: ProviderEID, Action: ActionSignStart, Outcome: OutcomeOK,
		RegNo: signerEtsi, ActorUserID: actor.UserID,
	})
	nexus.Audit(r.Context(), tenantID, actor.UserID, "esign.sign_started", "esign", map[string]any{
		"session_id": started.SessionID, "document_id": documentID, "on_behalf_of": onBehalfOf,
	})

	nexus.JSON(w, http.StatusOK, session)
}

// readSignInput accepts either shape of request and returns the bytes to sign.
func (m *Rails) readSignInput(r *http.Request, tenantID string, policy Policy) (pdf []byte, fileName, documentID, onBehalfOf, signerID string, err error) {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		r.Body = http.MaxBytesReader(nil, r.Body, maxUploadBody)
		if parseErr := r.ParseMultipartForm(8 << 20); parseErr != nil {
			return nil, "", "", "", "", tooLarge(policy)
		}
		file, header, formErr := r.FormFile("file")
		if formErr != nil {
			return nil, "", "", "", "", badRequest("MISSING_FILE", "a PDF file is required in the 'file' field")
		}
		defer func() { _ = file.Close() }()

		pdf, err = io.ReadAll(io.LimitReader(file, int64(policy.MaxUploadMB<<20)+1))
		if err != nil {
			return nil, "", "", "", "", badRequest("UNREADABLE_FILE", "the uploaded file could not be read")
		}
		if len(pdf) > policy.MaxUploadMB<<20 {
			return nil, "", "", "", "", tooLarge(policy)
		}
		if err = validatePDF(pdf); err != nil {
			return nil, "", "", "", "", err
		}
		fileName = sanitizeFileName(header.Filename)
		onBehalfOf = normalizeOrgEtsi(r.FormValue("onBehalfOf"))

		// A multipart ceremony may still attach to a stored document, which is
		// how batch signing reuses this path.
		if id := strings.TrimSpace(r.FormValue("document_id")); id != "" {
			doc, docErr := m.store.getDocument(r.Context(), tenantID, id)
			if docErr != nil {
				return nil, "", "", "", "", docErr
			}
			documentID = doc.ID
		}
		return pdf, fileName, documentID, onBehalfOf, strings.TrimSpace(r.FormValue("signer_id")), nil
	}

	var req struct {
		DocumentID string `json:"document_id"`
		OnBehalfOf string `json:"on_behalf_of"`
		SignerID   string `json:"signer_id"`
	}
	if err = decodeJSON(r, &req); err != nil {
		return nil, "", "", "", "", err
	}
	if strings.TrimSpace(req.DocumentID) == "" {
		return nil, "", "", "", "", badRequest("MISSING_DOCUMENT", "upload a file or supply document_id")
	}

	pdf, _, title, err := m.store.documentForSigning(r.Context(), tenantID, req.DocumentID)
	if err != nil {
		return nil, "", "", "", "", err
	}
	doc, err := m.store.getDocument(r.Context(), tenantID, req.DocumentID)
	if err != nil {
		return nil, "", "", "", "", err
	}
	fileName = doc.FileName
	if fileName == "" {
		fileName = sanitizeFileName(title)
	}
	return pdf, fileName, doc.ID, normalizeOrgEtsi(req.OnBehalfOf), strings.TrimSpace(req.SignerID), nil
}

// firstNonBlank returns the first value that is not empty after trimming.
func firstNonBlank(values ...string) string {
	for _, v := range values {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func tooLarge(policy Policy) error {
	return &Error{
		Code:    "PAYLOAD_TOO_LARGE",
		Message: "the PDF exceeds the " + strconv.Itoa(policy.MaxUploadMB) + "MB limit",
		Status:  http.StatusRequestEntityTooLarge,
	}
}

// signStatusHandler is what the browser polls. The library is authoritative;
// the stored row is a cache of it kept for the log and the batch view.
func (m *Rails) signStatusHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, actor, ok := m.require(w, r, PermSign)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if !sessionIDPattern.MatchString(id) {
		writeDomainError(w, badRequest("INVALID_SESSION_ID", "the signing session id is malformed"))
		return
	}

	session, err := m.store.getSession(r.Context(), tenantID, id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if session.State != SessionPending {
		// Terminal is a settled fact; re-asking would only add latency.
		nexus.JSON(w, http.StatusOK, session)
		return
	}

	settled, err := m.settle(r, tenantID, actor, session)
	if err != nil {
		// A transient upstream failure is not a verdict — the ceremony is still
		// open on the citizen's phone, and the browser treats an unchanged
		// answer as "keep waiting", which is correct.
		nexus.JSON(w, http.StatusOK, session)
		return
	}
	nexus.JSON(w, http.StatusOK, settled)
}

// settle asks the library for the ceremony's state and records the outcome.
func (m *Rails) settle(r *http.Request, tenantID string, actor Actor, session *SignSession) (*SignSession, error) {
	state, err := m.eid.PollSign(r.Context(), civilIDFromEtsi(session.SignerEtsi), session.ID)
	if err != nil {
		return nil, err
	}

	switch mapped := stateFromLibrary(state); mapped {
	case SessionPending:
		return session, nil

	case SessionCompleted:
		signed, err := m.eid.DownloadSigned(r.Context(), civilIDFromEtsi(session.SignerEtsi), session.ID)
		if err != nil {
			return nil, err
		}
		// Pinned to 'pending' so two concurrent pollers cannot both complete
		// the same ceremony and double-write the document.
		won, err := m.store.completeSession(r.Context(), tenantID, session.ID, sessionCompletion{
			SignedPDF: signed.PDF,
		})
		if err != nil {
			return nil, err
		}
		if won && session.DocumentID != "" {
			if err := m.store.markSigned(r.Context(), tenantID, session.DocumentID, signedDocument{
				Provider:       ProviderEID,
				SignedPDF:      signed.PDF,
				SignerName:     session.SignerName,
				SignerRegNo:    civilIDFromEtsi(session.SignerEtsi),
				SignerEtsi:     session.SignerEtsi,
				OnBehalfOfEtsi: session.OnBehalfOfEtsi,
				OnBehalfOfName: session.OnBehalfOfName,
				SignedAt:       time.Now(),
			}); err != nil {
				return nil, err
			}
		}
		if won {
			m.log(r, logEntry{
				TenantID: tenantID, DocumentID: session.DocumentID, SessionID: session.ID,
				Provider: ProviderEID, Action: ActionSign, Outcome: OutcomeOK,
				RegNo: session.SignerEtsi, FirstName: session.SignerName,
				ActorUserID: actor.UserID,
			})
			nexus.Audit(r.Context(), tenantID, actor.UserID, "esign.document_signed", "esign", map[string]any{
				"session_id": session.ID, "document_id": session.DocumentID, "provider": ProviderEID,
			})
		}

	default:
		if err := m.store.failSession(r.Context(), tenantID, session.ID, mapped, state); err != nil {
			return nil, err
		}
		m.log(r, logEntry{
			TenantID: tenantID, DocumentID: session.DocumentID, SessionID: session.ID,
			Provider: ProviderEID, Action: ActionSign,
			Outcome: map[string]string{
				SessionRejected: OutcomeRejected,
				SessionExpired:  OutcomeExpired,
				SessionFailed:   OutcomeFailed,
			}[mapped],
			RegNo: session.SignerEtsi, ActorUserID: actor.UserID, Detail: state,
		})
	}
	return m.store.getSession(r.Context(), tenantID, session.ID)
}

func (m *Rails) signDownloadHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, actor, ok := m.require(w, r, PermSign)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if !sessionIDPattern.MatchString(id) {
		writeDomainError(w, badRequest("INVALID_SESSION_ID", "the signing session id is malformed"))
		return
	}

	pdf, fileName, err := m.store.sessionSignedPDF(r.Context(), tenantID, id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	m.log(r, logEntry{
		TenantID: tenantID, SessionID: id, Provider: ProviderEID,
		Action: ActionDownload, Outcome: OutcomeOK, ActorUserID: actor.UserID,
	})
	writePDF(w, signedName(fileName), pdf)
}

// signCancelHandler abandons a ceremony from this side. eID's own session is
// left to expire — the relying-party API has no cancel, and the citizen's phone
// stops mattering once the result is refused.
func (m *Rails) signCancelHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, actor, ok := m.require(w, r, PermSign)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if !sessionIDPattern.MatchString(id) {
		writeDomainError(w, badRequest("INVALID_SESSION_ID", "the signing session id is malformed"))
		return
	}
	if err := m.store.failSession(r.Context(), tenantID, id, SessionFailed, "cancelled_by_user"); err != nil {
		writeDomainError(w, err)
		return
	}
	m.log(r, logEntry{
		TenantID: tenantID, SessionID: id, Provider: ProviderEID,
		Action: ActionSign, Outcome: OutcomeCancelled, ActorUserID: actor.UserID,
	})
	session, err := m.store.getSession(r.Context(), tenantID, id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	nexus.JSON(w, http.StatusOK, session)
}

// organizationsHandler lists the organisations the signer may act for.
func (m *Rails) organizationsHandler(w http.ResponseWriter, r *http.Request) {
	_, actor, ok := m.require(w, r, PermSign)
	if !ok {
		return
	}
	if actor.Etsi == "" {
		// Not an error: an unlinked account simply has nothing to represent.
		nexus.JSON(w, http.StatusOK, []any{})
		return
	}
	orgs, err := m.eid.Representations(r.Context(), actor.Etsi)
	if err != nil {
		// The dropdown is an enhancement; failing it would block signing for a
		// permission the relying party may not even hold.
		nexus.JSON(w, http.StatusOK, []any{})
		return
	}
	nexus.JSON(w, http.StatusOK, orgs)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// translateEIDError turns an upstream failure into something a citizen can act
// on, without echoing an upstream body that may carry identifiers.
func translateEIDError(err error) error {
	// The library's own answers first, matched exactly. The text search below
	// stays for what comes from eID itself — a status line, a message this
	// platform did not write — where there is nothing to match on but words.
	switch {
	case errors.Is(err, eidsign.ErrNotRepresentative):
		return forbidden("you are not registered as a representative of this organisation")
	case errors.Is(err, eidsign.ErrSessionNotFound):
		return &Error{
			Code:    "SIGN_SESSION_NOT_FOUND",
			Message: "that signature session does not exist",
			Status:  http.StatusNotFound,
		}
	case errors.Is(err, eidsign.ErrRefused):
		return forbidden("the citizen declined the signature on their phone")
	case errors.Is(err, eidsign.ErrNotCompleted):
		return &Error{
			Code:    "SIGN_NOT_COMPLETE",
			Message: "the signature is not finished yet",
			Status:  http.StatusBadRequest,
		}
	}

	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "represent"):
		return forbidden("you are not registered as a representative of this organisation")
	case strings.Contains(msg, "not found") || strings.Contains(msg, "enroll"):
		return &Error{
			Code:    "SIGNER_NOT_ENROLLED",
			Message: "this citizen is not enrolled for signing in eID Mongolia",
			Status:  http.StatusBadRequest,
		}
	case strings.Contains(msg, "401") || strings.Contains(msg, "403") || strings.Contains(msg, "unauthor"):
		// An operator problem. "Try again" would send the citizen in circles.
		return &Error{
			Code:    "EID_RP_REJECTED",
			Message: "this deployment is not authorised to sign with eID Mongolia; contact your administrator",
			Status:  http.StatusServiceUnavailable,
		}
	}
	return upstream("EID_UNAVAILABLE", "eID Mongolia could not start the signature; please try again")
}

// validateEtsi guards the identifier before it reaches a URL path.
//
// Letters are matched as \p{L}, not [A-Za-z]. A Mongolian registration number
// is Cyrillic — УА00112233 — so the ASCII-only class this started with rejected
// every real one, including the example the signing screen itself offers. The
// guard exists to keep separators and traversal out of a URL path segment, not
// to have an opinion about alphabets.
var etsiPattern = regexp.MustCompile(`^(PNOMN|NTRMN)-[\p{L}\p{N}]{1,32}$`)

func validateEtsi(etsi string) error {
	if !etsiPattern.MatchString(etsi) {
		return badRequest("INVALID_SIGNER", "the signer identifier is not a valid registration or civil ID")
	}
	return nil
}

func normalizeOrgEtsi(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	return eidmongolia.OrgEtsi(raw)
}

// civilIDFromEtsi recovers the bare identifier, which is both what the library
// keys session ownership by and what the signature log has always recorded.
func civilIDFromEtsi(etsi string) string {
	if idx := strings.Index(etsi, "-"); idx >= 0 {
		return etsi[idx+1:]
	}
	return etsi
}
