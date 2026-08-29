# Architecture Specification

The current code structure, two-plane boundary, and data ownership model of
**Gerege Nexus**. Updated 2026-08-25.

<p>
  <a href="ARCHITECTURE_SPECIFICATION.md"><img src="assets/icons/flag-mn.png" width="18" height="18" alt=""> Монгол</a>
  &nbsp;·&nbsp;
  <img src="assets/icons/flag-en.png" width="18" height="18" alt=""> <b>English</b>
</p>

[Documentation hub](README.md) · [Control plane](CONTROL_PLANE.md) ·
[ADR-0005](adr/0005-two-planes-one-origin-each.md)

---

## 1. One process, two planes

Gerege Nexus is a **modular monolith** built with Go, Next.js, and PostgreSQL.
One `cmd/api` binary, image, and deployment serve two independent request
planes:

| | Tenant plane | Operator plane |
| --- | --- | --- |
| Responsibility | A user's work inside one organisation | Operating the entire deployment |
| Origin | `nexus.gerege.mn` | `admin.nexus.gerege.mn` |
| API | `/api/v1/*` | `/api/platform/v1/*` |
| Session cookie | `session_token` | `cp_session` |
| Account | `registry.users` + `tenant.memberships` | `operator.operator_accounts` |
| Database role | `gerege_nexus_tenant` | `gerege_nexus_operator` |
| Go package | `internal/workspace/*` | `internal/operator/*` |

An operator account is not a user account. A person may be signed into both
planes, but each has a distinct identity, cookie, privilege set, and audit
trail. When an operator needs a tenant's view, they use the reason-bound,
30-minute impersonation flow.

```text
tenant origin ─┐                         ┌─ internal/workspace/* ─ workspace schema
               ├─ pkg/host/server.go ┤
control origin ┘   shared middleware     └─ internal/operator/* ─ operator + registry
                          │
                    internal/kernel/*
                          │
                      PostgreSQL
```

`backend/pkg/host/server.go` is the composition root. It constructs shared
stores, middleware, and the router, then mounts both route tables. The planes do
not import one another; `internal/planes_test.go` enforces that rule over the Go
import graph.

## 2. Code boundaries

| Location | Responsibility |
| --- | --- |
| `backend/internal/kernel` | Plane-neutral cache, config, security, telemetry, settings, flags, and other primitives |
| `backend/internal/workspace` | Authentication, access, directory, devices, identity, integrations, profile, SSO, and app installation for one tenant |
| `backend/internal/operator` | Operator sessions, tenants, approvals, settings, flags, audit, support, metering, backup, catalog, and observability |
| `backend/internal/apps` | Where a distribution's modules are assembled. Empty since 2026-08-25, when SSO Clients left for the App Store — every app now arrives through `pkg/nexus` and a catalogue |
| `backend/pkg/host` | Public host package that assembles both planes into one HTTP process |
| `backend/pkg/nexus` | Stable SDK contract for external modules and distributions |

A plane's root package only composes its subpackages. Handlers, stores, and
business logic belong in a domain subpackage. The current
`internal/workspace/service.go` remains a future decomposition task; it does not
relax the import or schema boundary.

## 3. Request paths

### 3.1 Shared layer

Both planes share request IDs, tracing, structured logging, panic recovery,
load shedding, metrics, security headers, CORS, and CSRF middleware in
`pkg/host/server.go`. `/health`, `/ready`, and `/metrics` are process-level
endpoints owned by neither plane.

### 3.2 Tenant request

1. Resolve the user from `session_token` or an allowed bearer token.
2. Place the active tenant in the request context.
3. Run database work as `gerege_nexus_tenant` through `dbguard`.
4. Let PostgreSQL RLS and `tenant_id` constrain rows to that organisation.
5. For module routes, check `tenant.app_installations` and the kill switch.

### 3.3 Operator request

1. `HostGate` admits only the `CONTROL_PLANE_HOST` origin.
2. Resolve `cp_session`; password plus TOTP, short idle timeout, and step-up
   apply.
3. Run every query as `gerege_nexus_operator`.
4. Commit each write with its `operator.operator_audit` row in the same
   transaction. A write without audit cannot report success.

In production, nginx's CIDR allowlist runs before HostGate. Origin, session,
database role, and audit are independent layers.

## 4. Database ownership

Migration `00079_two_schemas.sql` split tables into `platform` and `tenant`;
`00080_search_path_has_no_public.sql` removed `public` from runtime search
paths. `00083_registry_and_operator.sql` split `platform` in two, and
`00084_workspace_schema.sql` renamed `tenant` to `workspace`.

| Schema | Owned data |
| --- | --- |
| `registry` | Tenants, users, identity, apps, permissions, quotas, flags, announcements, usage, current setting values |
| `operator` | Operator accounts/sessions/audit, approvals, backup metadata, setting change history, sealed credentials |
| `workspace` | Memberships, roles, sessions, app installations, profile, directory, device, integration, SSO, and workspace audit data |
| `public` | Goose migration ledgers and deliberately retained `SECURITY DEFINER` functions |

The current migration inventory contains 20 registry, 7 operator and 40
workspace tables. Counts are not the contract: `backend/db/migrations/ownership_test.go`
declares ownership by name, and `schema_split_test.go` compares that
declaration with a real database.

Two planes have three schemas because of the boundary tables. The tenant role
must resolve five of them by name — announcements, feature flag overrides,
operator impersonations, tenant quotas, usage events — so `USAGE` on whichever
schema holds them cannot be revoked from it. Until 00083 that schema held all
twenty-seven tables, and the boundary rested on the **table-level grant**
alone.

Now the five boundary tables are in `registry` and the seven the tenant plane
never reaches are in `operator`. The tenant role holds no `USAGE` on
`operator`, so `operator.operator_audit` is not even a name to it. The boundary
is two locks: the schema hides the name, the table grant opens the row. A
database integration test proves that a newly created registry table is closed
to the tenant role by default.

All DDL enters through goose migrations in `backend/db/migrations/`. Runtime
DDL is forbidden. Distribution modules provide their own migrations through
the `pkg/nexus` contract.

## 5. Modules and catalog

Core does not own a business app's tables or handlers. A distribution supplies
module code, a manifest, and migrations, then registers them through the Nexus
SDK contract. The operator plane fetches the catalog and reconciles metadata in
`registry.apps`; tenant installation, version, and state live in
`tenant.app_installations`.

The AI stock forecast endpoint does not depend on a built-in inventory table.
It delegates to an enabled distribution's `stock_forecast` capability and
returns `404` when no provider exists.

## 6. Multi-replica behaviour and resilience

- `kernel/resilience/loadshedder.go` bounds concurrent requests.
- `kernel/cache.Bus` distributes invalidations over Redis pub/sub when Redis is
  configured and remains locally functional when it is not.
- `kernel/memo` provides a short-TTL, prefix-invalidated process-local cache
  for authorisation and installation decisions.
- `kernel/async` runs named goroutines with panic recovery and stack logging.
- Settings and feature flags use periodic refresh in addition to event-driven
  invalidation.

## 7. Automated architecture guards

| Invariant | Enforcement |
| --- | --- |
| No direct imports between planes | `backend/internal/planes_test.go` |
| Every table lands in its declared schema | `backend/db/migrations/ownership_test.go`, `schema_split_test.go` |
| SQL qualifies owned tables by schema | `backend/db/migrations/qualification_test.go` |
| Tenant role reads only five platform boundary tables | `schema_split_test.go` |
| New platform tables are closed by default | `TestNewPlatformTableIsClosedToTenantRole` |
| The HTTP surface does not drift silently | `backend/pkg/host/testdata/routes.txt` |
| Origin and `/cp` host routing remain separated | `frontend/tests/control-plane-host.test.mjs`, `frontend/scripts/check-control-plane-host.mjs`, `frontend/scripts/smoke-control-plane-host.mjs` |

For rationale, see the [two-plane proposal](TWO_PLANES_PROPOSAL.md),
[implementation review](TWO_PLANES_REVIEW.md), and
[ADR-0005](adr/0005-two-planes-one-origin-each.md). Proposal, prompt, and plan
documents are historical design records; this document and executable code are
the current sources of truth.
