/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Whose tables these are.
 *
 * db/migrations is the platform's schema and the only place a table could be
 * created, which is why 28 of its 108 tables belonged to apps that are not in
 * this repository: commerce, the government services workflow, point of sale
 * and the registry. Their tables could not follow them out, so every deployment
 * carried the schema of every app anybody ever wrote here.
 *
 * Migration 00075 dropped all twenty-eight, and 00077 dropped eleven more:
 * the nine document_* tables, `departments` and `organisation_people`, which
 * went with their apps to client-gerege-nexus. Those eleven are a different
 * case and a harder one. The twenty-eight were unreachable from here — this
 * repository served none of those apps' routes — while these were being read
 * by code in this tree until the commit that dropped them. What made that possible was
 * nexus.Migrations (Үе 3): a module brings its own schema, so
 * business-gerege-nexus declares commerce's five itself. What made it safe was
 * counting the routes first — the core served none of those apps' endpoints,
 * so nothing here could read the tables anyway.
 *
 * The list below is what remains, and this test is what keeps that true. A
 * count kept in a comment is only true until the next migration — this one has
 * said sixty-nine and then sixty-six, and both were right on the day they were
 * written — so the number lives in schema_split_test.go, where a migration that
 * changes it has to change it. Migration 00078 took urtuu_tasks,
 * urtuu_task_events and urtuu_numbers to the Өртөө module; 00087 took the
 * channel's remaining six the same way, and with them the last row in this file
 * that named Өртөө.
 *
 * Which plane a table belongs to.
 *
 * Ownership answers "is this the platform's table or an app's". It does not
 * answer the question the two planes ask, which is whose behalf a row is held
 * on: a deployment holds one platform_settings, and it holds one sessions per
 * organisation. So every entry carries a plane as well, decided by one
 * sentence (docs/TWO_PLANES_PROPOSAL.md §2.1):
 *
 *	A row that exists once per deployment is the operator plane. A row that
 *	exists separately for each tenant is the workspace plane.
 *
 * There is no third value. Code both planes need is not a third plane, it is
 * the floor underneath them, and it owns no table.
 *
 * The operator plane's tables sit in two schemas, and that is a different cut.
 * Migration 00083 asked a narrower question of the same twenty-seven tables —
 * which of them may the workspace plane reach at all — and moved the seven it may
 * not into `operator`, leaving twenty in `registry`. operatorOnlyTables below
 * is that answer. A plane is who a row is held for; a schema is who may name
 * it. The five boundary tables show why both are needed: they are the operator
 * plane's and they are in registry, because a tenant reads its own rows.
 *
 * A tenant_id column is the usual sign of the workspace plane but not the rule.
 * Six workspace tables have none — esign_batch_items, membership_roles,
 * role_permissions, installation_events, oauth2_access_tokens, report_grants —
 * because they hang off a parent row that does. Five go the other way:
 * announcements, feature_flag_overrides, operator_impersonations,
 * tenant_quotas and usage_events carry a tenant_id and are still the
 * operator plane's, because the operator writes them and a tenant only reads its
 * own. Those five are the boundary between the planes, and they were not
 * chosen here: policy_shape_test.go had already found the same five and
 * written "console, FOR SELECT" beside them, before anyone was looking for a
 * boundary. TestBoundaryTablesAreThePlatforms below keeps the two lists from
 * drifting, because everything that comes after — the schemas, the grants, the
 * import rules — is drawn to that line.
 */

package migrations_test

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// A table, and the two things a review needs to know about it: whose behalf
// its rows are held on, and what it is for.
type table struct {
	plane   string // "workspace" or "operator" — the §2.1 rule, nothing else
	purpose string
}

// Every table the platform's migrations create, who it is for, and which plane
// it belongs to.
//
// A new name here is not refused — the platform does grow tables. It is made
// visible: adding one means adding a line to this list, and that line is a
// sentence in a review saying "this belongs to the platform, on this plane",
// which somebody can disagree with. Before, a CREATE TABLE in a 300-line
// migration was indistinguishable from every other line in it.
var platformTables = map[string]table{
	// ---------------------------------------------------- the operator plane
	// One row per deployment, whoever is signed in.
	"announcements":             {"operator", "control plane"},
	"app_dependencies":          {"operator", "app store"},
	"app_versions":              {"operator", "app store"},
	"apps":                      {"operator", "app store"},
	"credential_grants":         {"operator", "access recovery"},
	"eid_sign_state":            {"operator", "eID"},
	"feature_flag_overrides":    {"operator", "control plane"},
	"feature_flags":             {"operator", "control plane"},
	"identity_binding_sessions": {"operator", "identity"},
	"oauth2_signing_keys":       {"operator", "OAuth2 provider"},
	"operator_accounts":         {"operator", "control plane"},
	"operator_audit":            {"operator", "control plane"},
	"operator_impersonations":   {"operator", "control plane"},
	"operator_sessions":         {"operator", "control plane"},
	"pending_approvals":         {"operator", "control plane"},
	// The names of the rights, not who holds them: role_permissions is the
	// tenant's half of this pair.
	"permissions":      {"operator", "access control"},
	"platform_backups": {"operator", "control plane"},
	// The keys a deployment reaches other systems with, sealed. Its values
	// never leave the process, which is why there is no history table beside
	// it: a history of a credential is a list of the ones it used to have.
	"platform_credentials":      {"operator", "control plane"},
	"platform_settings":         {"operator", "control plane"},
	"platform_settings_history": {"operator", "control plane"},
	"store_app_versions":        {"operator", "app store"},
	// Who does what, deployment-wide. In registry because it is neither one
	// workspace's nor one person's: an organisation publishes into it and
	// everybody reads it. Migration 00090.
	"service_directory": {"operator", "directory"},
	"tenant_quotas":     {"operator", "tenants"},
	"tenants":           {"operator", "tenants"},
	"usage_events":      {"operator", "usage"},
	// A person is one person across the organisations they belong to; the
	// membership is what is per-tenant, and that is on the other side.
	"user_eid_identities": {"operator", "identity"},
	"user_sso_identities": {"operator", "identity"},
	"users":               {"operator", "users"},

	// --------------------------------------------------- the workspace plane
	// One row per organisation, and no organisation reads another's.
	"access_change_events":       {"workspace", "access control"},
	"ai_knowledge":               {"workspace", "assistant"},
	"ai_prompts":                 {"workspace", "assistant"},
	"app_installations":          {"workspace", "app store"},
	"audit_events":               {"workspace", "audit"},
	"device_enrollment_codes":    {"workspace", "devices"},
	"device_telemetry":           {"workspace", "devices"},
	"devices":                    {"workspace", "devices"},
	"email_verifications":        {"workspace", "email verification"},
	"esign_batch_items":          {"workspace", "signing rail"},
	"esign_batches":              {"workspace", "signing rail"},
	"esign_documents":            {"workspace", "signing rail"},
	"esign_settings":             {"workspace", "signing rail"},
	"esign_sign_sessions":        {"workspace", "signing rail"},
	"esign_signature_logs":       {"workspace", "signing rail"},
	"installation_events":        {"workspace", "app store"},
	"membership_roles":           {"workspace", "access control"},
	"memberships":                {"workspace", "access control"},
	"oauth2_access_tokens":       {"workspace", "OAuth2 provider"},
	"oauth2_authorization_codes": {"workspace", "OAuth2 provider"},
	"oauth2_clients":             {"workspace", "OAuth2 provider"},
	"oauth2_consents":            {"workspace", "OAuth2 provider"},
	"oauth2_tokens":              {"workspace", "OAuth2 provider"},
	// A projection of somebody else's row, written into this workspace by
	// registry.publish_person_item. The workspace plane by §2.1 and not a
	// borderline case: the row exists for one home and no other, which is
	// exactly what the rule asks. Migration 00086.
	"person_items": {"workspace", "a person's own requests"},
	// Somebody asking to be let in. The organisation's row, not the asker's:
	// they are not a member yet, and the decision, the audit and the work are
	// the organisation's. What the asker sees is the person_items projection
	// above. Migration 00089.
	"join_requests":         {"workspace", "access control"},
	"push_tokens":           {"workspace", "devices"},
	"report_grants":         {"workspace", "report sharing"},
	"report_schedules":      {"workspace", "reports"},
	"role_permissions":      {"workspace", "access control"},
	"roles":                 {"workspace", "access control"},
	"sessions":              {"workspace", "auth"},
	"staff_pin_credentials": {"workspace", "devices"},
	"tenant_profiles":       {"workspace", "tenants"},
}

// The seven the workspace plane may not reach at all.
//
// Migration 00083 split the operator plane's twenty-seven tables into two
// schemas. The line was not drawn by taste: four of these seven — the operator
// accounts, sessions, audit and the sealed credentials — already had no grant
// of any kind for gerege_nexus_tenant, so the database was refusing them
// already. The other three are named by no query outside internal/operator;
// their tenant grants were left over from before 00079, when all sixty-six
// tables shared `public` and one line granted the lot.
//
// What the move buys is a second lock. Before it, the boundary was table
// grants alone: the tenant role held USAGE on `platform` because five tables
// in it are the boundary, so operator_audit was shut by its own grant and
// nothing else. Now the tenant role has no USAGE on `operator`, and the name
// does not resolve for it.
//
// Everything not listed here is `registry` — deliberately, so that a table
// added to the operator plane later lands on the open side and a review has to
// argue it onto the closed one. schema_split_test.go checks the result against
// the database.
var operatorOnlyTables = map[string]bool{
	"operator_accounts":         true,
	"operator_audit":            true,
	"operator_sessions":         true,
	"pending_approvals":         true,
	"platform_backups":          true,
	"platform_credentials":      true,
	"platform_settings_history": true,
}

// schemaOf is the schema a table must be in once 00083 has run.
func schemaOf(name string) string {
	if platformTables[name].plane == "workspace" {
		return "workspace"
	}
	if operatorOnlyTables[name] {
		return "operator"
	}
	return "registry"
}

var (
	createTable = regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(?:(?:public|platform|registry|operator|tenant|workspace)\.)?([a-z0-9_]+)`)
	dropTable   = regexp.MustCompile(`(?i)DROP\s+TABLE\s+(?:IF\s+EXISTS\s+)?(?:(?:public|platform|registry|operator|tenant|workspace)\.)?([a-z0-9_]+)`)
	// Comments are stripped first. These files explain themselves at length,
	// and a comment quoting `CREATE TABLE IF NOT EXISTS` was read as a table
	// called "if".
	sqlComment = regexp.MustCompile(`(?m)--.*$`)
)

// upSection is the half of a migration that runs going forward.
//
// Reading the whole file would count a Down section's DROP as a drop and its
// CREATE as a creation, which is the opposite of what either does on `up`.
func upSection(source string) string {
	code := sqlComment.ReplaceAllString(source, "")
	if idx := strings.Index(source, "-- +goose Down"); idx >= 0 {
		code = sqlComment.ReplaceAllString(source[:idx], "")
	}
	return code
}

func TestPlatformMigrationsOwnNoAppTable(t *testing.T) {
	files, err := filepath.Glob("*.sql")
	if err != nil || len(files) == 0 {
		t.Fatalf("read the migration directory: %v", err)
	}

	// What the schema ends up with, not what its history mentions. An applied
	// migration cannot be rewritten, so the twenty-eight tables migration 00075
	// dropped are still created by 00003, 00006, 00007, 00038 and 00040 — and
	// are not in any deployment's schema. Replaying creations and drops in
	// order is how a text scan answers the question the database would.
	sort.Strings(files)
	found := map[string]string{}
	for _, file := range files {
		source, err := os.ReadFile(file) // #nosec G304 -- a glob of this directory
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		code := upSection(string(source))
		for _, match := range createTable.FindAllStringSubmatch(code, -1) {
			found[strings.ToLower(match[1])] = file
		}
		for _, match := range dropTable.FindAllStringSubmatch(code, -1) {
			delete(found, strings.ToLower(match[1]))
		}
	}

	var unlisted []string
	for table, file := range found {
		if _, ok := platformTables[table]; !ok {
			unlisted = append(unlisted, table+" ("+file+")")
		}
	}
	sort.Strings(unlisted)
	if len(unlisted) > 0 {
		t.Errorf(`db/migrations creates %d table(s) the platform has not claimed:

	%s

An app's table belongs in the app's own migrations — a module registers them
with nexus.Migrations and they run under public.goose_db_version_<slug>. A table
created here is a table every deployment carries whether or not it has the app,
and it can never leave with the app: 28 of the 108 tables here are already in
that position.

If this really is a platform table, add it to platformTables in
db/migrations/ownership_test.go with a word for what it is for and the plane it
belongs to:

	a row that exists once per deployment is the operator plane; a row that
	exists separately for each tenant is the workspace plane

That line is the decision, and a review can disagree with it.`,
			len(unlisted), strings.Join(unlisted, "\n\t"))
	}

	// The list must not outlive the tables either: a name left here after its
	// migration was rewritten is a name the next table can quietly inherit.
	var stale []string
	for table := range platformTables {
		if _, ok := found[table]; !ok {
			stale = append(stale, table)
		}
	}
	sort.Strings(stale)
	for _, table := range stale {
		t.Errorf("platformTables claims %q but no migration creates it", table)
	}
}

// A plane is one of two things, and the absence of one is not the third.
//
// The value is a string because the list reads better with it spelled out, and
// a string is a thing somebody can leave empty or invent a value for. "shared",
// "both" or "core" would each be a reasonable-sounding way to avoid deciding,
// and deciding is the whole of what this list is for: a table serving both
// planes is a table on the wrong side of a line that has not been drawn yet.
func TestEveryTableDeclaresAPlane(t *testing.T) {
	var undecided []string
	for name, entry := range platformTables {
		if entry.plane != "workspace" && entry.plane != "operator" {
			undecided = append(undecided, name+" ("+entry.plane+")")
		}
	}
	sort.Strings(undecided)
	for _, name := range undecided {
		t.Errorf(`%s is on neither plane.

A row that exists once per deployment is the operator plane; a row that exists
separately for each tenant is the workspace plane. There is no third value: shared
code is the floor underneath both planes and owns no table, so a table that
seems to want one is a table whose owner has not been decided.

The operator plane's tables live in two schemas — registry and operator — but
that is a second, narrower cut inside one plane, and operatorOnlyTables makes
it. It is not a third plane.`, name)
	}
}

// The five tables the planes meet on are the platform's.
//
// The platform writes them and a tenant reads its own rows, which is why they
// carry a tenant_id and are still not the tenant's — the one place §2.1's rule
// and the tenant_id column disagree. Everything after this is drawn to that
// line: which schema the table lands in, which role is granted SELECT on it,
// which side of the import boundary the code reading it lives on.
//
// So it is checked against the other list rather than restated here.
// policy_shape_test.go marked these five "console, FOR SELECT" long before
// there was a plane to put them on, and two lists of five names in two files
// drift the moment one of them is edited alone.
func TestBoundaryTablesAreThePlatforms(t *testing.T) {
	const consoleRead = "console, FOR SELECT"

	var boundary []string
	for name, reason := range narrowPolicies {
		if reason != consoleRead {
			continue
		}
		boundary = append(boundary, name)
		if plane := platformTables[name].plane; plane != "operator" {
			t.Errorf(`%s is the boundary between the planes but ownership_test.go puts it on the %q plane.

policy_shape_test.go calls it %q: the platform writes it, the console shows it
and a tenant reads only its own rows through a FOR SELECT policy. That is the
operator plane by §2.1 — the row is the deployment's statement about a tenant,
not the tenant's own row — and a schema move or a grant written from the other
answer would hand the workspace plane something it may only read.`,
				name, plane, consoleRead)
		}
	}

	sort.Strings(boundary)
	if len(boundary) != 5 {
		t.Errorf(`the boundary is %d table(s), not five:

	%s

The five are announcements, feature_flag_overrides, operator_impersonations,
tenant_quotas and usage_events. Widening or narrowing that set is a decision
about where the planes touch, which is worth arguing about in a review; it is
not something to arrive at by editing a reason string in the other file.`,
			len(boundary), strings.Join(boundary, "\n\t"))
	}
}

// The platform's Go code must not name a table it does not own.
//
// internal/apps/boundaries_test.go already stops the platform importing an app
// package; the compiler enforces that half. This is the other half, and it is
// the half nothing enforces: a table name inside a SQL string is exactly the
// same dependency, and it survives the app leaving. The query keeps compiling
// and keeps returning rows — right up until the deployment that never had the
// app runs it against a table that is not there.
//
// The names are the twenty-eight that migration 00075 dropped. They no longer
// exist in any deployment's schema, so a query naming one now fails outright
// rather than quietly answering zero — which is the better failure, and still
// not one to ship.
func TestPlatformSQLNamesNoAppTable(t *testing.T) {
	departed := []string{
		"billing_invoices", "contacts", "products", "stock_levels", "stock_movements", "warehouses",
		"gov_application_events", "gov_applications", "gov_appointments", "gov_delivery_outbox",
		"gov_org_units", "gov_routing_rules", "gov_services", "gov_tasks", "gov_unit_members",
		"gov_upstream_connectors", "gov_workflow_steps", "gov_workflow_transitions",
		"gov_workflow_versions", "gov_workflows",
		"pos_shifts",
		"store_app_texts", "store_apps", "store_catalog_snapshots", "store_external_registrations",
		"store_publishers", "store_registry_state", "store_review_events",
	}
	sort.Strings(departed)

	// No allowlist. There were four entries when this test was written — the
	// assistant's commerce queries, the till shift handlers and three test
	// fixtures — and each was a debt with an owner. All four are paid: Үе 4a
	// took the assistant's, and this change took the rest with the tables.
	// A new entry here would be a new debt, which is a thing to argue about in
	// a review rather than to add quietly.

	// A table name where SQL would put one. Comments are stripped first, the
	// same as in the migration scan above: these files explain themselves, and
	// a sentence saying which tables a query *used* to read is not a query.
	comments := regexp.MustCompile(`(?s)/\*.*?\*/|(?m)//.*$`)
	pattern := regexp.MustCompile(`(?i)\b(?:FROM|JOIN|INTO|UPDATE|DELETE\s+FROM)\s+(?:public\.)?(` +
		strings.Join(departed, "|") + `)\b`)

	root := filepath.Join("..", "..")
	var offences []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return err //nolint:wrapcheck // walk errors are reported as they are
		}
		source, err := os.ReadFile(path) // #nosec G304 -- a walk of this repository
		if err != nil {
			return err //nolint:wrapcheck
		}
		rel := filepath.ToSlash(strings.TrimPrefix(path, root+string(filepath.Separator)))
		code := comments.ReplaceAllString(string(source), "")
		seen := map[string]bool{}
		for _, match := range pattern.FindAllStringSubmatch(code, -1) {
			table := strings.ToLower(match[1])
			if seen[table] {
				continue
			}
			seen[table] = true
			offences = append(offences, rel+" queries "+table)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan the backend: %v", err)
	}

	sort.Strings(offences)
	for _, offence := range offences {
		t.Errorf(`%s

That table belonged to a module in another repository and migration 00075
dropped it. A SQL string naming it is the same dependency a Go import would be,
and the only difference is that no compiler reports it — which is why these
outlived their modules. There is no table behind this query on any deployment.`, offence)
	}
}
