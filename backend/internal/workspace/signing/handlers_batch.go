/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Batch signing — "Багц баталгаажуулалт". A run signs many documents under a
 * single set of PIN2 approvals.
 *
 * Each document is still its own eID ceremony: the protocol signs one digest
 * per approval, and there is no way to have a citizen approve a set of
 * documents with one PIN entry without collapsing exactly the guarantee a
 * signature is for. The batch is therefore a queue with progress, not a
 * shortcut around consent — the citizen confirms each document on their phone,
 * and the run reports how far it has got.
 */

package signing

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/identity/eidmongolia"
	"github.com/go-chi/chi/v5"
)

func (m *Rails) listBatchesHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, _, ok := m.require(w, r, PermRead)
	if !ok {
		return
	}
	limit, offset := pagination(r, 25)
	list, total, err := m.store.listBatches(r.Context(), tenantID, limit, offset)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	nexus.JSON(w, http.StatusOK, Page[Batch]{Items: list, Total: total, Limit: limit, Offset: offset})
}

func (m *Rails) getBatchHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, _, ok := m.require(w, r, PermRead)
	if !ok {
		return
	}
	batch, err := m.store.getBatch(r.Context(), tenantID, chi.URLParam(r, "id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	nexus.JSON(w, http.StatusOK, batch)
}

func (m *Rails) createBatchHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, actor, ok := m.require(w, r, PermManage)
	if !ok {
		return
	}

	var req struct {
		Name        string   `json:"name"`
		Provider    string   `json:"provider"`
		DocumentIDs []string `json:"document_ids"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeDomainError(w, err)
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeDomainError(w, badRequest("MISSING_NAME", "a batch name is required"))
		return
	}
	if len(req.DocumentIDs) == 0 {
		writeDomainError(w, badRequest("EMPTY_BATCH", "select at least one document"))
		return
	}
	// A run holds an eID ceremony open per document. An unbounded batch would
	// keep a citizen tapping PIN2 for an hour and hold as many sessions open
	// upstream.
	if len(req.DocumentIDs) > 100 {
		writeDomainError(w, badRequest("BATCH_TOO_LARGE", "a batch may hold at most 100 documents"))
		return
	}

	_, policy, _, err := m.store.loadSettings(r.Context(), tenantID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	provider := strings.ToUpper(strings.TrimSpace(req.Provider))
	if provider == "" {
		provider = policy.DefaultProvider
	}
	if provider != ProviderEID && provider != ProviderHSM {
		writeDomainError(w, badRequest("INVALID_PROVIDER", "provider must be EID or HSM"))
		return
	}
	if provider == ProviderHSM && policy.RequireEID {
		writeDomainError(w, forbidden("this tenant requires qualified eID Mongolia signatures"))
		return
	}

	batch, err := m.store.createBatch(r.Context(), tenantID, actor.UserID,
		strings.TrimSpace(req.Name), provider, dedupe(req.DocumentIDs))
	if err != nil {
		writeDomainError(w, err)
		return
	}

	nexus.Audit(r.Context(), tenantID, actor.UserID, "esign.batch_created", "esign", map[string]any{
		"batch_id": batch.ID, "documents": len(batch.Items), "provider": provider,
	})
	nexus.JSON(w, http.StatusCreated, batch)
}

// runBatchHandler starts the next pending document in the batch and returns
// the ceremony to confirm.
//
// It advances one document per call rather than looping server-side. A loop
// would have to block for as long as the citizen takes to approve every
// document — well past any HTTP deadline — and would leave the browser with no
// way to show which document it is currently asking about.
func (m *Rails) runBatchHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, actor, ok := m.require(w, r, PermSign)
	if !ok {
		return
	}
	batchID := chi.URLParam(r, "id")

	batch, err := m.store.getBatch(r.Context(), tenantID, batchID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if batch.Status == BatchCancelled {
		writeDomainError(w, conflict("BATCH_CANCELLED", "this batch was cancelled"))
		return
	}
	if batch.Provider != ProviderEID {
		writeDomainError(w, badRequest("UNSUPPORTED_PROVIDER",
			"only eID Mongolia batches can be run from here; HSM documents are signed individually"))
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
	if actor.Etsi == "" {
		writeDomainError(w, badRequest("NO_SIGNER_IDENTITY",
			"this account is not linked to eID Mongolia; sign in with eID to run a batch"))
		return
	}

	// Reconcile anything already in flight before picking the next document,
	// so a completed ceremony is recorded even if the browser stopped polling.
	m.settleRunningItems(r, tenantID, actor, batch)

	batch, err = m.store.getBatch(r.Context(), tenantID, batchID)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	next := nextPendingItem(batch)
	if next == nil {
		status := BatchCompleted
		if batch.Failed > 0 && batch.Signed == 0 {
			status = BatchFailed
		}
		if err := m.store.setBatchStatus(r.Context(), tenantID, batchID, status); err != nil {
			writeDomainError(w, err)
			return
		}
		batch.Status = status
		nexus.JSON(w, http.StatusOK, map[string]any{"batch": batch, "session": nil})
		return
	}

	if err := m.store.setBatchStatus(r.Context(), tenantID, batchID, BatchRunning); err != nil {
		writeDomainError(w, err)
		return
	}

	session, err := m.startBatchItem(r, tenantID, actor, next)
	if err != nil {
		// One document failing does not end the run: it is marked and the next
		// call moves on, which is what an operator signing fifty contracts
		// needs rather than starting over.
		_ = m.store.setBatchItem(r.Context(), next.ID, ItemFailed, "", errorMessage(err))
		batch, _ = m.store.getBatch(r.Context(), tenantID, batchID)
		nexus.JSON(w, http.StatusOK, map[string]any{
			"batch": batch, "session": nil, "error": errorMessage(err),
		})
		return
	}

	if err := m.store.setBatchItem(r.Context(), next.ID, ItemRunning, session.ID, ""); err != nil {
		writeDomainError(w, err)
		return
	}
	batch, _ = m.store.getBatch(r.Context(), tenantID, batchID)
	nexus.JSON(w, http.StatusOK, map[string]any{"batch": batch, "session": session})
}

// startBatchItem opens one ceremony for one document in the batch.
func (m *Rails) startBatchItem(r *http.Request, tenantID string, actor Actor, item *BatchItem) (*SignSession, error) {
	pdf, _, _, err := m.store.documentForSigning(r.Context(), tenantID, item.DocumentID)
	if err != nil {
		return nil, err
	}

	// The library issues the session id and holds the document for the life of
	// the ceremony, so a batch item is started exactly like a single signature.
	started, err := m.eid.SignPDF(r.Context(), eidmongolia.SignRequest{
		RegNo:    civilIDFromEtsi(actor.Etsi),
		FullName: actor.FullName,
		FileName: item.FileName,
		PDF:      pdf,
	})
	if err != nil {
		return nil, translateEIDError(err)
	}

	session, err := m.store.createSession(r.Context(), newSession{
		ID:               started.SessionID,
		TenantID:         tenantID,
		DocumentID:       item.DocumentID,
		EIDSessionID:     started.SessionID,
		FileName:         started.Filename,
		DocumentHash:     started.DocumentHash,
		VerificationCode: started.VerificationCode,
		SignerUserID:     actor.UserID,
		SignerEtsi:       actor.Etsi,
		SignerName:       actor.FullName,
	})
	if err != nil {
		return nil, err
	}

	m.log(r, logEntry{
		TenantID: tenantID, DocumentID: item.DocumentID, DocumentTitle: item.DocumentTitle,
		SessionID: started.SessionID, Provider: ProviderEID, Action: ActionBatchSign, Outcome: OutcomeOK,
		RegNo: actor.Etsi, ActorUserID: actor.UserID,
	})
	return session, nil
}

// settleRunningItems reconciles in-flight ceremonies against eID so a batch
// makes progress even when the browser was closed mid-run.
func (m *Rails) settleRunningItems(r *http.Request, tenantID string, actor Actor, batch *Batch) {
	for i := range batch.Items {
		item := &batch.Items[i]
		if item.Status != ItemRunning || item.SessionID == "" {
			continue
		}
		session, err := m.store.getSession(r.Context(), tenantID, item.SessionID)
		if err != nil {
			continue
		}
		if session.State == SessionPending {
			settled, err := m.settle(r, tenantID, actor, session)
			if err != nil {
				continue
			}
			session = settled
		}
		switch session.State {
		case SessionCompleted:
			_ = m.store.setBatchItem(r.Context(), item.ID, ItemSigned, session.ID, "")
		case SessionRejected, SessionExpired, SessionFailed:
			_ = m.store.setBatchItem(r.Context(), item.ID, ItemFailed, session.ID, session.FailureReason)
		}
	}
}

func (m *Rails) cancelBatchHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, actor, ok := m.require(w, r, PermManage)
	if !ok {
		return
	}
	batchID := chi.URLParam(r, "id")
	if err := m.store.setBatchStatus(r.Context(), tenantID, batchID, BatchCancelled); err != nil {
		writeDomainError(w, err)
		return
	}
	nexus.Audit(r.Context(), tenantID, actor.UserID, "esign.batch_cancelled", "esign", map[string]any{
		"batch_id": batchID,
	})
	batch, err := m.store.getBatch(r.Context(), tenantID, batchID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	nexus.JSON(w, http.StatusOK, batch)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func nextPendingItem(batch *Batch) *BatchItem {
	for i := range batch.Items {
		if batch.Items[i].Status == ItemPending {
			return &batch.Items[i]
		}
	}
	return nil
}

// dedupe preserves order while removing repeats, so selecting a document twice
// in the picker does not ask the citizen to sign it twice.
func dedupe(ids []string) []string {
	seen := make(map[string]bool, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// errorMessage renders a domain error for storage on a batch item without
// leaking an upstream body. errors.As rather than a type assertion, so a
// domain error wrapped on its way up still reads as one.
func errorMessage(err error) string {
	var domain *Error
	if errors.As(err, &domain) {
		return domain.Message
	}
	return "the signature could not be started"
}
