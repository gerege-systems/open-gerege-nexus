/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Document intake, listing and retrieval.
 */

package signing

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/identity/gerege"
	"github.com/go-chi/chi/v5"
)

func (m *Rails) listDocumentsHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, _, ok := m.require(w, r, PermRead)
	if !ok {
		return
	}
	limit, offset := pagination(r, logPageSize)
	status := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("status")))
	if status != "" && status != StatusPending && status != StatusSigned {
		writeDomainError(w, badRequest("INVALID_STATUS", "status must be PENDING or SIGNED"))
		return
	}

	list, total, err := m.store.listDocuments(r.Context(), tenantID, status,
		strings.TrimSpace(r.URL.Query().Get("q")), limit, offset)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	// The original endpoint answered with a bare array and the documents
	// screen still reads it that way, so paginated callers opt in explicitly
	// rather than every existing client breaking on an envelope.
	if r.URL.Query().Get("paginated") == "true" {
		nexus.JSON(w, http.StatusOK, Page[Document]{Items: list, Total: total, Limit: limit, Offset: offset})
		return
	}
	nexus.JSON(w, http.StatusOK, list)
}

func (m *Rails) getDocumentHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, _, ok := m.require(w, r, PermRead)
	if !ok {
		return
	}
	doc, err := m.store.getDocument(r.Context(), tenantID, chi.URLParam(r, "id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	nexus.JSON(w, http.StatusOK, doc)
}

// uploadDocumentHandler takes a base64 payload. It is the original intake and
// stays for API clients; browsers use the multipart route, which does not pay
// base64's 33% transfer cost.
func (m *Rails) uploadDocumentHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, actor, ok := m.require(w, r, PermManage)
	if !ok {
		return
	}

	var req struct {
		Title     string `json:"title"`
		FileName  string `json:"file_name"`
		PdfBase64 string `json:"pdf_base64"`
	}
	// The body limit has to clear a base64-inflated PDF, not just the PDF.
	if err := decodeLargeJSON(r, &req); err != nil {
		writeDomainError(w, err)
		return
	}
	if strings.TrimSpace(req.Title) == "" || req.PdfBase64 == "" {
		writeDomainError(w, badRequest("MISSING_FIELDS", "title and pdf_base64 are required"))
		return
	}

	pdf, err := base64.StdEncoding.DecodeString(req.PdfBase64)
	if err != nil {
		writeDomainError(w, badRequest("INVALID_BASE64", "pdf_base64 is not valid base64"))
		return
	}
	m.storeUpload(w, r, tenantID, actor, req.Title, req.FileName, pdf)
}

// uploadMultipartHandler takes the file directly.
func (m *Rails) uploadMultipartHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, actor, ok := m.require(w, r, PermManage)
	if !ok {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBody)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		// A body over the cap surfaces here, and it is a size problem rather
		// than a malformed request — say so, because the browser cannot tell
		// the two apart from a 400.
		writeDomainError(w, &Error{
			Code:    "PAYLOAD_TOO_LARGE",
			Message: "the upload exceeds the " + strconv.Itoa(maxPDFBytes>>20) + "MB limit",
			Status:  http.StatusRequestEntityTooLarge,
		})
		return
	}
	defer func() { _ = r.MultipartForm.RemoveAll() }()

	file, header, err := r.FormFile("file")
	if err != nil {
		writeDomainError(w, badRequest("MISSING_FILE", "a PDF file is required in the 'file' field"))
		return
	}
	defer func() { _ = file.Close() }()

	pdf, err := io.ReadAll(io.LimitReader(file, maxPDFBytes+1))
	if err != nil {
		writeDomainError(w, badRequest("UNREADABLE_FILE", "the uploaded file could not be read"))
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	fileName := header.Filename
	if title == "" {
		// Falling back to the filename without its extension keeps the list
		// readable when a caller uploads without naming the document.
		title = strings.TrimSuffix(filepath.Base(fileName), filepath.Ext(fileName))
	}
	m.storeUpload(w, r, tenantID, actor, title, fileName, pdf)
}

// storeUpload is the shared tail of both intake routes: validate, hash, store.
func (m *Rails) storeUpload(w http.ResponseWriter, r *http.Request, tenantID string, actor Actor, title, fileName string, pdf []byte) {
	_, policy, _, err := m.store.loadSettings(r.Context(), tenantID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	limit := policy.MaxUploadMB << 20
	if len(pdf) > limit {
		writeDomainError(w, &Error{
			Code:    "PAYLOAD_TOO_LARGE",
			Message: "the PDF exceeds the " + strconv.Itoa(policy.MaxUploadMB) + "MB limit",
			Status:  http.StatusRequestEntityTooLarge,
		})
		return
	}
	if err := validatePDF(pdf); err != nil {
		writeDomainError(w, err)
		return
	}

	fileName = sanitizeFileName(fileName)
	sum := sha256.Sum256(pdf)

	doc, err := m.store.createDocument(r.Context(), tenantID, actor.UserID,
		strings.TrimSpace(title), fileName, hex.EncodeToString(sum[:]),
		gerege.PDFPageCount(pdf), pdf)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	nexus.Audit(r.Context(), tenantID, actor.UserID, "esign.document_uploaded", "esign", map[string]any{
		"document_id": doc.ID,
		"title":       doc.Title,
		"bytes":       len(pdf),
	})
	nexus.JSON(w, http.StatusCreated, doc)
}

func (m *Rails) downloadDocumentHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, actor, ok := m.require(w, r, PermRead)
	if !ok {
		return
	}

	variant := "original"
	if r.URL.Query().Get("variant") == "signed" {
		variant = "signed"
	}

	pdf, fileName, err := m.store.documentPDF(r.Context(), tenantID, chi.URLParam(r, "id"), variant)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if variant == "signed" {
		fileName = signedName(fileName)
	}

	// Downloading a signed document is an auditable event: it is the moment
	// the evidence leaves the system.
	if variant == "signed" {
		m.log(r, logEntry{
			TenantID: tenantID, DocumentID: chi.URLParam(r, "id"),
			Action: ActionDownload, Outcome: OutcomeOK, ActorUserID: actor.UserID,
		})
	}
	writePDF(w, fileName, pdf)
}

func (m *Rails) deleteDocumentHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, actor, ok := m.require(w, r, PermManage)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if err := m.store.softDeleteDocument(r.Context(), tenantID, id); err != nil {
		writeDomainError(w, err)
		return
	}
	nexus.Audit(r.Context(), tenantID, actor.UserID, "esign.document_archived", "esign", map[string]any{
		"document_id": id,
	})
	w.WriteHeader(http.StatusNoContent)
}

// ─── Shared helpers ──────────────────────────────────────────────────────────

// validatePDF rejects anything that is not a PDF before it reaches the signing
// service. The magic-number check is cheap and stops the obvious case — an
// image or an archive renamed .pdf — from becoming a confusing upstream error
// minutes later during a signing ceremony.
func validatePDF(pdf []byte) error {
	if len(pdf) < 5 || string(pdf[:5]) != "%PDF-" {
		return badRequest("NOT_A_PDF", "the file is not a valid PDF document")
	}
	// A PDF without a trailer is truncated, which the signing service accepts
	// and then produces an unopenable result from.
	if !strings.Contains(string(pdf[max(0, len(pdf)-2048):]), "%%EOF") {
		return badRequest("TRUNCATED_PDF", "the PDF appears to be truncated or corrupt")
	}
	return nil
}

// unsafeFileName strips anything that could escape a directory or break a
// Content-Disposition header.
var unsafeFileName = regexp.MustCompile(`[^\p{L}\p{N}._\- ]+`)

func sanitizeFileName(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	name = unsafeFileName.ReplaceAllString(name, "")
	name = strings.TrimSpace(strings.Trim(name, "."))
	if name == "" {
		return "document.pdf"
	}
	if !strings.HasSuffix(strings.ToLower(name), ".pdf") {
		name += ".pdf"
	}
	if len([]rune(name)) > 120 {
		name = string([]rune(name)[:116]) + ".pdf"
	}
	return name
}

func signedName(fileName string) string {
	base := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	return base + "-signed.pdf"
}

// writePDF streams a document with a header that survives non-ASCII names.
// A bare filename= with Cyrillic in it is mangled by every browser, so the
// RFC 5987 filename* form carries the real name and the ASCII fallback keeps
// older clients working.
func writePDF(w http.ResponseWriter, fileName string, pdf []byte) {
	ascii := strings.Map(func(r rune) rune {
		if r < 32 || r > 126 || r == '"' || r == '\\' {
			return '_'
		}
		return r
	}, fileName)

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Length", strconv.Itoa(len(pdf)))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition",
		`attachment; filename="`+ascii+`"; filename*=`+mime.BEncoding.Encode("UTF-8", fileName))
	_, _ = w.Write(pdf)
}

// decodeLargeJSON reads an intake body. The cap allows for base64 inflating
// the PDF by a third plus the surrounding JSON.
func decodeLargeJSON(r *http.Request, out any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, maxUploadBody)
	if err := json.NewDecoder(r.Body).Decode(out); err != nil {
		return badRequest("INVALID_BODY", "the request body is not valid JSON or exceeds the size limit")
	}
	return nil
}

// decodeOptionalJSON reads a body that is allowed to be absent, and refuses one
// that is present but unreadable.
//
// The distinction matters where the two mean different things. On the export
// endpoint an absent body means "every automatic destination" and a body naming
// one connector means that connector alone — so discarding a malformed body,
// as this used to, silently turns "file this contract in the archive" into
// "file it everywhere", which for a signed document is a disclosure nobody
// asked for. An empty body is still a request; a broken one is an error.
func decodeOptionalJSON(r *http.Request, out any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, maxUploadBody)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return badRequest("INVALID_BODY", "the request body could not be read or exceeds the size limit")
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return badRequest("INVALID_BODY", "the request body is not valid JSON")
	}
	return nil
}
