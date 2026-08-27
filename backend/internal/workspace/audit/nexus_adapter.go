/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

// maxAuditPage is how many lines a reader may take at once. A history screen
// shows a page; a caller asking for the whole trail is asking for an export,
// which is a different feature with a different permission.
const maxAuditPage = 500

// AsReader presents the audit table as the SDK's nexus.AuditReader.
//
// Reading was the half a module never had. nexus.Audit has written to this
// trail since the SDK existed, so an app that shows "what has this organisation
// looked up" had to reach into workspace.audit_events with its own SQL — a dependency on
// a platform table that no compiler sees and that survives the app moving to
// another repository.
//
// The query is the tenant's own rows only. That is not a courtesy: the pool is
// bound to the caller's organisation by dbguard and the row-level policy on
// audit_events refuses the rest, so a reader that asked for somebody else's
// would get nothing rather than an error.
func AsReader(db *pgxpool.Pool) nexus.AuditReader { return reader{db} }

type reader struct{ db *pgxpool.Pool }

func (r reader) RecentByPrefix(ctx context.Context, tenantID string, prefixes []string, limit int) ([]nexus.AuditEntry, error) {
	if len(prefixes) == 0 {
		return nil, fmt.Errorf("audit: a reader must name at least one action prefix")
	}
	if limit <= 0 || limit > maxAuditPage {
		limit = maxAuditPage
	}

	// LIKE patterns, built from the prefixes rather than interpolated: a
	// prefix arrives from a module and a module can come from anywhere.
	patterns := make([]string, 0, len(prefixes))
	for _, prefix := range prefixes {
		patterns = append(patterns, strings.TrimSuffix(prefix, ".")+".%")
	}

	rows, err := r.db.Query(ctx,
		`SELECT action, COALESCE(user_id, ''), details, created_at
		   FROM workspace.audit_events
		  WHERE tenant_id = $1 AND action LIKE ANY($2)
		  ORDER BY created_at DESC
		  LIMIT $3`, tenantID, patterns, limit)
	if err != nil {
		return nil, fmt.Errorf("read the audit trail: %w", err)
	}
	defer rows.Close()

	entries := make([]nexus.AuditEntry, 0, 32)
	for rows.Next() {
		var entry nexus.AuditEntry
		var raw []byte
		// A row that will not scan is skipped rather than failing the screen:
		// this is a history, and one unreadable entry in it is not worth
		// refusing the other hundred and ninety-nine.
		if err := rows.Scan(&entry.Action, &entry.UserID, &raw, &entry.At); err != nil {
			continue
		}
		_ = json.Unmarshal(raw, &entry.Details)
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}
