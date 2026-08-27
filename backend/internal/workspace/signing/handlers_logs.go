/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * The signature log — every certificate check, ceremony and download, with
 * their outcomes. Backs the "Гарын үсгийн лог" screen.
 */

package signing

import (
	"encoding/csv"
	"net/http"
	"strings"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

func (m *Rails) listLogsHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, _, ok := m.require(w, r, PermRead)
	if !ok {
		return
	}
	query := logQueryFrom(r)
	list, total, err := m.store.listLogs(r.Context(), tenantID, query)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	// As with documents, the original endpoint answered with a bare array.
	if r.URL.Query().Get("paginated") == "true" {
		nexus.JSON(w, http.StatusOK, Page[SignatureLog]{
			Items: list, Total: total, Limit: query.Limit, Offset: query.Offset,
		})
		return
	}
	nexus.JSON(w, http.StatusOK, list)
}

// exportLogsHandler streams the filtered log as CSV. An auditor needs the
// evidence outside the browser, and asking them to paginate through a table is
// not an answer.
func (m *Rails) exportLogsHandler(w http.ResponseWriter, r *http.Request) {
	tenantID, _, ok := m.require(w, r, PermRead)
	if !ok {
		return
	}
	query := logQueryFrom(r)
	// An export is not a page. It is still bounded, so a runaway tenant cannot
	// stream an unbounded result set out of the database.
	query.Limit, query.Offset = maxPageSize*10, 0

	list, _, err := m.store.listLogs(r.Context(), tenantID, query)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="esign-signature-log.csv"`)
	w.Header().Set("X-Content-Type-Options", "nosniff")

	// Excel reads a UTF-8 CSV as Latin-1 without a byte-order mark, which
	// turns every Cyrillic name in the log into mojibake.
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})

	out := csv.NewWriter(w)
	defer out.Flush()
	_ = out.Write([]string{
		"timestamp", "action", "outcome", "provider", "document", "document_id",
		"session_id", "reg_no", "phone_no", "first_name", "last_name", "detail",
	})
	for _, entry := range list {
		_ = out.Write([]string{
			entry.CreatedAt.Format(time.RFC3339), entry.Action, entry.Outcome, entry.Provider,
			entry.DocumentTitle, entry.DocumentID, entry.SessionID, entry.RegNo,
			entry.PhoneNo, entry.FirstName, entry.LastName, entry.Detail,
		})
	}
}

func logQueryFrom(r *http.Request) LogQuery {
	limit, offset := pagination(r, logPageSize)
	params := r.URL.Query()
	return LogQuery{
		Action:     strings.ToUpper(strings.TrimSpace(params.Get("action"))),
		Outcome:    strings.ToUpper(strings.TrimSpace(params.Get("outcome"))),
		Provider:   strings.ToUpper(strings.TrimSpace(params.Get("provider"))),
		DocumentID: strings.TrimSpace(params.Get("document_id")),
		Search:     strings.TrimSpace(params.Get("q")),
		From:       parseTime(params.Get("from")),
		To:         parseTime(params.Get("to")),
		Limit:      limit,
		Offset:     offset,
	}
}
