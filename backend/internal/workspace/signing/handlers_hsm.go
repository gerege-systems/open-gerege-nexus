/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * The Gerege eSign HSM rail: certificate proof plus a drawn signature stamped
 * server-side. It is synchronous and predates the eID rail.
 *
 * This rail does not produce a qualified electronic signature — the key lives
 * in the operator's HSM rather than with the citizen — so a tenant can refuse
 * it outright from the signing policy, and every response says which rail
 * produced the document.
 */

package signing

import (
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/identity/gerege"
	"github.com/go-chi/chi/v5"
)

func (m *Rails) checkCertHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, actor, ok := m.require(w, r, PermSign)
	if !ok {
		return
	}
	if err := m.assertHSMAllowed(r, tenantID); err != nil {
		writeDomainError(w, err)
		return
	}

	var req struct {
		PhoneNo string `json:"phone_no"`
		CivilID string `json:"civil_id"`
		Data    string `json:"data"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeDomainError(w, err)
		return
	}
	if strings.TrimSpace(req.PhoneNo) == "" || strings.TrimSpace(req.CivilID) == "" {
		writeDomainError(w, badRequest("MISSING_FIELDS", "phone_no and civil_id are required"))
		return
	}

	cert, err := m.hsm.CheckCertificate(r.Context(), gerege.EsignCertRequest{
		PhoneNo: req.PhoneNo,
		CivilID: req.CivilID,
		Data:    req.Data,
	})
	if err != nil {
		// A failed certificate check is exactly the event an auditor looks
		// for — somebody tried to sign as a citizen they could not prove.
		m.log(r, logEntry{
			TenantID: tenantID, Provider: ProviderHSM, Action: ActionCertCheck,
			Outcome: OutcomeFailed, RegNo: req.CivilID, PhoneNo: req.PhoneNo,
			ActorUserID: actor.UserID, Detail: err.Error(),
		})
		writeDomainError(w, badRequest("CERTIFICATE_INVALID", err.Error()))
		return
	}

	m.log(r, logEntry{
		TenantID: tenantID, Provider: ProviderHSM, Action: ActionCertCheck, Outcome: OutcomeOK,
		RegNo: req.CivilID, PhoneNo: req.PhoneNo,
		FirstName: cert.SubjectDN.GivenName, LastName: cert.SubjectDN.Surname,
		ActorUserID: actor.UserID,
	})
	nexus.Audit(r.Context(), tenantID, actor.UserID, "esign.cert_checked", "esign", map[string]any{
		"civil_id": req.CivilID,
	})

	nexus.JSON(w, http.StatusOK, map[string]any{
		"is_valid":    cert.IsValid,
		"given_name":  cert.SubjectDN.GivenName,
		"surname":     cert.SubjectDN.Surname,
		"common_name": cert.SubjectDN.CommonName,
		"uid":         cert.SubjectDN.UID,
	})
}

func (m *Rails) signDocumentHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, actor, ok := m.require(w, r, PermSign)
	if !ok {
		return
	}
	if err := m.assertHSMAllowed(r, tenantID); err != nil {
		writeDomainError(w, err)
		return
	}

	id := chi.URLParam(r, "id")
	var req struct {
		PhoneNo          string `json:"phone_no"`
		SignerName       string `json:"signer_name"`
		SignerRegNo      string `json:"signer_reg_no"`
		SignatureImage64 string `json:"signature_image64"`
		SignatureText    string `json:"signature_text"`
		X                uint   `json:"x"`
		Y                uint   `json:"y"`
		Width            uint   `json:"width"`
		Height           uint   `json:"height"`
		PageNumber       uint   `json:"page_number"` // 0 = last page
	}
	if err := decodeLargeJSON(r, &req); err != nil {
		writeDomainError(w, err)
		return
	}
	if strings.TrimSpace(req.PhoneNo) == "" || req.SignatureImage64 == "" {
		writeDomainError(w, badRequest("MISSING_FIELDS", "phone_no and signature_image64 are required"))
		return
	}

	sigImage, err := base64.StdEncoding.DecodeString(req.SignatureImage64)
	if err != nil {
		writeDomainError(w, badRequest("INVALID_BASE64", "signature_image64 is not valid base64"))
		return
	}

	pdf, _, title, err := m.store.documentForSigning(r.Context(), tenantID, id)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	// The request may override placement; anything it leaves unset falls back
	// to the tenant's configured stamp position rather than a hard-coded one.
	placement, _, _, err := m.store.loadSettings(r.Context(), tenantID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	placement = Placement{
		X: req.X, Y: req.Y, Width: req.Width, Height: req.Height,
		PageNumber: req.PageNumber, Text: req.SignatureText,
	}.mergeOver(placement)
	if err := placement.validate(); err != nil {
		writeDomainError(w, err)
		return
	}

	pageCount := gerege.PDFPageCount(pdf)
	page := placement.PageNumber
	if page == 0 || page > pageCount {
		page = pageCount
	}

	signed64, err := m.hsm.SignPDF(r.Context(), gerege.EsignDocSignRequest{
		MSISDN:           "976" + strings.TrimSpace(req.PhoneNo),
		X:                placement.X,
		Y:                placement.Y,
		Width:            placement.Width,
		Height:           placement.Height,
		PageNumber:       page,
		SignatureText:    placement.Text,
		Pdf64:            base64.StdEncoding.EncodeToString(pdf),
		SignatureImage64: req.SignatureImage64,
		ExtraImages:      make([]gerege.EsignExtraImage, 0),
	})
	if err != nil {
		m.log(r, logEntry{
			TenantID: tenantID, DocumentID: id, DocumentTitle: title,
			Provider: ProviderHSM, Action: ActionSign, Outcome: OutcomeFailed,
			RegNo: req.SignerRegNo, PhoneNo: req.PhoneNo, FirstName: req.SignerName,
			ActorUserID: actor.UserID, Detail: err.Error(),
		})
		writeDomainError(w, upstream("HSM_UNAVAILABLE", err.Error()))
		return
	}

	signed, err := base64.StdEncoding.DecodeString(signed64)
	if err != nil {
		writeDomainError(w, upstream("HSM_BAD_RESPONSE", "the eSign service returned invalid PDF data"))
		return
	}
	// The HSM answers with base64 whatever went wrong upstream, so the result
	// is checked before it is stored as a signed document.
	if err := validatePDF(signed); err != nil {
		writeDomainError(w, upstream("HSM_BAD_RESPONSE", "the eSign service did not return a valid PDF"))
		return
	}

	now := time.Now()
	if err := m.store.markSigned(r.Context(), tenantID, id, signedDocument{
		Provider:       ProviderHSM,
		SignedPDF:      signed,
		SignatureImage: sigImage,
		SignerName:     req.SignerName,
		SignerRegNo:    req.SignerRegNo,
		SignerPhone:    req.PhoneNo,
		SignedAt:       now,
	}); err != nil {
		writeDomainError(w, err)
		return
	}

	m.log(r, logEntry{
		TenantID: tenantID, DocumentID: id, DocumentTitle: title,
		Provider: ProviderHSM, Action: ActionSign, Outcome: OutcomeOK,
		RegNo: req.SignerRegNo, PhoneNo: req.PhoneNo, FirstName: req.SignerName,
		ActorUserID: actor.UserID,
	})
	nexus.Audit(r.Context(), tenantID, actor.UserID, "esign.document_signed", "esign", map[string]any{
		"document_id": id, "signer": req.SignerName,
		"page_number": page, "provider": ProviderHSM,
	})

	nexus.JSON(w, http.StatusOK, map[string]any{
		"status":      StatusSigned,
		"document_id": id,
		"signed_at":   now,
		"page_number": page,
		"provider":    ProviderHSM,
	})
}

// assertHSMAllowed refuses the older rail when the tenant has moved to
// qualified signatures. Without this the policy toggle would be decorative:
// the endpoint would still sign for anyone who called it directly.
func (m *Rails) assertHSMAllowed(r *http.Request, tenantID string) error {
	_, policy, _, err := m.store.loadSettings(r.Context(), tenantID)
	if err != nil {
		return err
	}
	if policy.RequireEID {
		return forbidden("this tenant requires qualified eID Mongolia signatures; the HSM rail is disabled")
	}
	return nil
}

// mergeOver fills this placement's unset fields from a base, so a request can
// override one coordinate without restating all of them.
func (p Placement) mergeOver(base Placement) Placement {
	if p.X == 0 {
		p.X = base.X
	}
	if p.Y == 0 {
		p.Y = base.Y
	}
	if p.Width == 0 {
		p.Width = base.Width
	}
	if p.Height == 0 {
		p.Height = base.Height
	}
	if p.PageNumber == 0 {
		p.PageNumber = base.PageNumber
	}
	if strings.TrimSpace(p.Text) == "" {
		p.Text = base.Text
	}
	return p.normalize()
}
