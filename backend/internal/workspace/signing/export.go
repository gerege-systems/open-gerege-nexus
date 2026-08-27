package signing

import (
	"context"
	"log/slog"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/async"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/workspace/integration"
	"github.com/go-chi/chi/v5"
)

// exportFilename builds the name a signed document is filed under.
//
// It is the document's own title rather than its UUID, because the person who
// opens the folder afterwards is looking for a contract, not for
// 7f3a1c8e-…-pdf. The title comes from a user, so it is stripped of anything
// that would make it a path rather than a name.
func exportFilename(title, fallbackID string) string {
	name := strings.TrimSpace(title)
	if name == "" {
		name = fallbackID
	}
	// Path separators, control characters and the characters Windows and
	// Dropbox both refuse. A title arriving as "../../etc/passwd" has to end up
	// as a filename, not a traversal.
	name = filepath.Base(name)
	name = regexp.MustCompile(`[\x00-\x1f/\\:*?"<>|]+`).ReplaceAllString(name, " ")
	name = strings.Join(strings.Fields(name), " ")
	if name == "" || name == "." || name == ".." {
		name = fallbackID
	}
	// Leave room for the extension inside the 255-byte limit every one of these
	// filesystems enforces. The cut is on a character boundary: a Mongolian
	// title is two bytes per letter, so name[:200] would routinely end halfway
	// through one and leave invalid UTF-8 in a filename that then travels
	// through JSON to Drive and through a header to Dropbox.
	if len(name) > 200 {
		cut := 200
		for cut > 0 && !utf8.RuneStart(name[cut]) {
			cut--
		}
		name = strings.TrimSpace(name[:cut])
	}
	if !strings.EqualFold(filepath.Ext(name), ".pdf") {
		name += ".pdf"
	}
	return name
}

// exportSignedDocument files a freshly signed PDF with every connector the
// tenant has marked for automatic export.
//
// It is deliberately best-effort and off the request's critical path: the
// signature is already made and stored by the time this runs, and failing the
// response because somebody's Dropbox was full would tell the citizen their
// signature did not happen when it did. Failures are recorded against the
// connector and in the delivery log, which is where an administrator looks.
//
// The context is detached for the same reason webhook delivery is: this runs
// after the handler returns, and the request context is cancelled by then.
func (m *Rails) exportSignedDocument(ctx context.Context, tenantID, documentID, title string, pdf []byte) {
	if m.exports == nil || len(pdf) == 0 {
		return
	}
	filename := exportFilename(title, documentID)
	sendCtx := context.WithoutCancel(ctx)

	async.Go("esign-export", func() {
		results := m.exports.ExportFileToAll(sendCtx, tenantID, filename, pdf, documentID)
		for _, res := range results {
			slog.Info("esign: signed document exported",
				"tenant", tenantID, "document", documentID,
				"connector", res.IntegrationName, "provider", res.Provider)
		}
	})
}

// exportDocumentHandler files an already-signed document on demand.
//
// Automatic export covers the documents signed after a connector was set up.
// This covers the ones signed before it, and the retry after a destination was
// unreachable — without it the only way to get an existing document into a
// newly connected Drive would be to sign it again.
func (m *Rails) exportDocumentHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, actor, ok := m.require(w, r, PermManage)
	if !ok {
		return
	}
	if m.exports == nil {
		writeDomainError(w, upstream("EXPORT_UNAVAILABLE", "no export destinations are configured"))
		return
	}

	id := chi.URLParam(r, "id")
	var req struct {
		IntegrationID string `json:"integration_id"`
	}
	// A body is optional: without one the document goes to every automatic
	// destination, which is what the retry case wants. A body that is present
	// but unreadable is not treated as absent — that would widen a request for
	// one destination into a copy to all of them.
	if err := decodeOptionalJSON(r, &req); err != nil {
		writeDomainError(w, err)
		return
	}

	pdf, filename, err := m.store.documentPDF(r.Context(), tenantID, id, "signed")
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if len(pdf) == 0 {
		writeDomainError(w, badRequest("NOT_SIGNED", "this document has not been signed yet"))
		return
	}

	doc, err := m.store.getDocument(r.Context(), tenantID, id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	name := exportFilename(doc.Title, id)
	if name == "" {
		name = filename
	}

	var results []integration.ExportResult
	if strings.TrimSpace(req.IntegrationID) != "" {
		res, exportErr := m.exports.ExportFile(r.Context(), tenantID, req.IntegrationID, name, pdf, id)
		if exportErr != nil {
			writeDomainError(w, upstream("EXPORT_FAILED", exportErr.Error()))
			return
		}
		results = []integration.ExportResult{*res}
	} else {
		results = m.exports.ExportFileToAll(r.Context(), tenantID, name, pdf, id)
		if len(results) == 0 {
			writeDomainError(w, badRequest("NO_DESTINATION",
				"no connected destination is set to receive documents automatically"))
			return
		}
	}

	nexus.Audit(r.Context(), tenantID, actor.UserID, "esign.document_exported", "esign", map[string]any{
		"document_id": id, "destinations": len(results),
	})
	nexus.JSON(w, http.StatusOK, map[string]any{"exported": results})
}
