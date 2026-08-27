/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Who does this, and what are they called.
 *
 * 00089 let a person ask an organisation to let them in, and made them name it
 * by the slug in its address — which works for the organisation you already
 * work at and not at all for a public service. Somebody knows they need a
 * driving licence renewed; they do not know which office does it.
 *
 * So: code → the organisations that published it. The table is
 * registry.service_directory (00090), a deployment-wide record rather than a
 * copy in every home, and reading it needs no permission — it is what an
 * organisation chose to say in public.
 */

package person

import (
	"context"
	"fmt"
	"strings"
)

// Provider is one organisation's published offer.
type Provider struct {
	// Slug is what the asking screen needs, and the reason this exists.
	Slug  string `json:"slug"`
	Name  string `json:"name"`
	Code  string `json:"code"`
	Title string `json:"title"`
}

// Directory answers "who does this".
//
// Ordered by the organisation's name, alphabetically, and migration 00090 says
// at length why that boring answer was chosen over the interesting ones: it
// repeats, it can be explained to somebody who does not like their position,
// and nobody can buy a better one.
//
// A blank code lists everything published, which is the screen somebody lands
// on before they have typed anything. The limit is the same either way: this is
// a lookup, not an export.
func (s *Store) Directory(ctx context.Context, code string, limit int) ([]Provider, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	code = strings.TrimSpace(code)

	rows, err := s.db.Query(ctx, `
		SELECT t.slug, t.name, d.code, d.title
		  FROM registry.service_directory d
		  JOIN registry.tenants t ON t.id = d.tenant_id
		 WHERE ($1 = '' OR d.code ILIKE '%' || $1 || '%' OR d.title ILIKE '%' || $1 || '%')
		   -- A suspended organisation is not answering, and 00089 refuses a
		   -- request to one. Listing it would be an invitation to press a
		   -- button that cannot work.
		   AND t.suspended_at IS NULL AND t.deletion_scheduled_at IS NULL
		 ORDER BY t.name, d.code
		 LIMIT $2`, code, limit)
	if err != nil {
		return nil, fmt.Errorf("look up who provides %q: %w", code, err)
	}
	defer rows.Close()

	found := make([]Provider, 0, 16)
	for rows.Next() {
		var one Provider
		if err := rows.Scan(&one.Slug, &one.Name, &one.Code, &one.Title); err != nil {
			return nil, fmt.Errorf("read a directory entry: %w", err)
		}
		found = append(found, one)
	}
	return found, rows.Err()
}
