package tenants

import (
	"context"
	"fmt"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"

	"github.com/jackc/pgx/v5"
)

// Handing an organisation its data back.
//
// This is the one place the console reads what an organisation keeps, and it
// is worth being explicit about why that is not a hole in everything CP-1
// built. The console's ordinary powers stop at metadata: its database role can
// SELECT ten named tables and no others, so no handler here can wander into a
// customer's invoices. The export does not use that role. It runs on the
// platform path, deliberately, for one purpose the plan names — an
// organisation being deleted must be able to leave with its data — and it
// pays for that with every control there is:
//
//	a capability only two roles hold, a second factor within five minutes,
//	an audit row naming the organisation, and a log line.
//
// The alternative designs are worse. A console that could never produce the
// data would make deletion a data loss event; a console whose *ordinary* role
// could read every table would make the export cheap and the isolation
// theatre. One audited exception is the honest shape.

// exportRowCap bounds each table in a bundle.
//
// An export is a file somebody downloads through a browser, and an
// organisation with two million stock movements would produce one nobody can
// open and a request nobody can serve. Truncation is reported in the bundle
// rather than silently applied — a partial export presented as complete is
// how somebody discovers, after the deletion, that it was not.
const exportRowCap = 50_000

// ExportBundle is an organisation's data, table by table.
type ExportBundle struct {
	Tenant     operator.TenantState `json:"tenant"`
	ExportedAt time.Time            `json:"exported_at"`
	ExportedBy string               `json:"exported_by"`
	// Tables maps a table name to its rows. Keys are whatever the schema had
	// at the time; a module added next year appears here without this file
	// changing, which is the property that keeps an export from quietly
	// falling behind the platform.
	Tables map[string][]map[string]any `json:"tables"`
	// Truncated names the tables that hit the cap, so nobody has to count.
	Truncated []string `json:"truncated"`
}

// ExportTenant assembles the bundle.
func (s *Service) ExportTenant(ctx context.Context, sess operator.Session, tenantID string) (ExportBundle, error) {
	state, err := s.op.StateOf(ctx, tenantID)
	if err != nil {
		return ExportBundle{}, err
	}

	// Audited first. An export that failed half way through has still been an
	// attempt to read an organisation's data, and the trail should say so.
	if err := s.op.Record(ctx, sess, operator.Change{
		Action:     "tenant.export",
		TargetType: "tenant",
		TargetID:   tenantID,
		Reason:     "export before deletion or at the organisation's request",
		After:      map[string]any{"slug": state.Slug},
	}, nil); err != nil {
		return ExportBundle{}, err
	}

	bundle := ExportBundle{
		Tenant:     state,
		ExportedAt: time.Now(),
		ExportedBy: sess.Email,
		Tables:     map[string][]map[string]any{},
		Truncated:  []string{},
	}

	// The platform path, not the console's role. See the note above.
	exportCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	tables, err := s.exportableTables(exportCtx)
	if err != nil {
		return ExportBundle{}, err
	}
	for _, table := range tables {
		rows, truncated, err := s.exportTable(exportCtx, table, tenantID)
		if err != nil {
			return ExportBundle{}, err
		}
		if len(rows) == 0 {
			continue
		}
		bundle.Tables[table] = rows
		if truncated {
			bundle.Truncated = append(bundle.Truncated, table)
		}
	}
	return bundle, nil
}

// exportableTables is every table that carries a tenant_id.
//
// Asked of the schema rather than listed here, for the same reason migration
// 00029 discovers the tables it protects: a list written by hand is a list
// that is wrong the first time somebody adds a table and does not think of
// this file. The difference in risk is small — the answer is the set of tables
// that belong to organisations, which is precisely what an export is.
func (s *Service) exportableTables(ctx context.Context) ([]string, error) {
	rows, err := s.db.Query(ctx,
		`SELECT c.table_name
		   FROM information_schema.columns c
		   JOIN information_schema.tables t
		     ON t.table_schema = c.table_schema AND t.table_name = c.table_name
		  WHERE c.table_schema = 'workspace'
		    AND c.column_name = 'tenant_id'
		    AND t.table_type = 'BASE TABLE'
		  ORDER BY c.table_name`)
	if err != nil {
		return nil, fmt.Errorf("control plane: list the exportable tables: %w", err)
	}
	defer rows.Close()

	tables := make([]string, 0, 32)
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return nil, fmt.Errorf("control plane: read a table name: %w", err)
		}
		tables = append(tables, table)
	}
	return tables, rows.Err()
}

// exportTable reads one table's rows for one organisation.
//
// The table name is interpolated, which is only safe because it came from
// information_schema a moment ago and can therefore only be a table that
// exists — never from a request. Quoted with %q so a table whose name needs
// quoting still parses. The tenant id stays a parameter.
func (s *Service) exportTable(ctx context.Context, table, tenantID string) ([]map[string]any, bool, error) {
	query := fmt.Sprintf(`SELECT * FROM tenant.%q WHERE tenant_id = $1::uuid LIMIT $2`, table)
	rows, err := s.db.Query(ctx, query, tenantID, exportRowCap+1)
	if err != nil {
		return nil, false, fmt.Errorf("control plane: export %s: %w", table, err)
	}
	collected, err := pgx.CollectRows(rows, pgx.RowToMap)
	if err != nil {
		return nil, false, fmt.Errorf("control plane: read %s: %w", table, err)
	}
	if len(collected) > exportRowCap {
		return collected[:exportRowCap], true, nil
	}
	return collected, false, nil
}
