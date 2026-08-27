# Module Authoring Guide

<p>
  <img src="assets/icons/flag-en.png" width="18" height="18" alt=""> <b>English</b>
</p>

[Back to the documentation hub](README.md)

Welcome to the **open-gerege-nexus** Module Authoring Guide! This guide explains how external developers can write, register, and distribute custom business application modules for the platform.

---

## Module architecture overview

In `open-gerege-nexus`, business modules are written in Go as compile-time packages under `backend/internal/apps/`. 

Every module MUST implement the `Module` interface defined in the public SDK,
[`backend/pkg/nexus`](../backend/pkg/nexus/module.go). It is `pkg/` and not
`internal/` for one reason: Go forbids another repository from importing a
package under `internal/`, so while the interface lived there the only way to
build a product on this platform was to fork it.

```go
type Module interface {
    ID() string
    Name() string
    Version() string
    Dependencies() []Dependency
    Permissions() []PermissionDefinition
    Menus() []MenuDefinition
    RegisterRoutes(r chi.Router, gateMiddleware func(http.Handler) http.Handler)
}
```

---

## Step by step: creating a new module

### Step 1: Define Module Struct & Register with the SDK
Create a new directory `backend/internal/apps/invoices/invoices.go`:

```go
package invoices

import (
    "net/http"
    "github.com/go-chi/chi/v5"
    "github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

type Module struct {
    db    nexus.DB
    perms nexus.PermissionStore
}

// One argument, not one per service. The platform will lend modules more over
// time, and a constructor that grew a parameter each time would be a signature
// every distribution has to chase.
func New(p nexus.Platform) *Module {
    m := &Module{db: p.DB(), perms: p.Permissions()}
    nexus.Register(m)
    return m
}

func (m *Module) ID() string      { return "io.gerege.nexus.invoices" }
func (m *Module) Name() string    { return "Invoicing & Billing" }
func (m *Module) Version() string { return "1.0.0" }

func (m *Module) Dependencies() []nexus.Dependency {
    return []nexus.Dependency{
        {ID: "io.gerege.nexus.contacts", VersionConstraint: "^1.0.0"},
        {ID: "io.gerege.nexus.products", VersionConstraint: "^1.0.0"},
    }
}
```

### Step 2: Define Permissions and Menus
```go
func (m *Module) Permissions() []nexus.PermissionDefinition {
    return []nexus.PermissionDefinition{
        {Code: "invoices.read", Name: "View Invoices"},
        {Code: "invoices.manage", Name: "Create & Edit Invoices"},
    }
}

func (m *Module) Menus() []nexus.MenuDefinition {
    return []nexus.MenuDefinition{
        {ID: "menu_invoices", Label: "Invoices", Path: "/invoices", Icon: "file-text", Order: 30},
    }
}
```

### Step 3: Register HTTP Routes with App Gate Middleware
```go
func (m *Module) RegisterRoutes(r chi.Router, gateMiddleware func(http.Handler) http.Handler) {
    r.Route("/api/v1/invoices", func(sub chi.Router) {
        sub.Use(gateMiddleware)
        sub.Get("/", m.handleListInvoices)
        sub.Post("/", m.handleCreateInvoice)
    })
}
```

#### Public routes, and the rule about them

`RegisterRoutes` receives the **root** router, not a pre-gated group. Mounting a
path outside `gateMiddleware` is one line and looks exactly like mounting one
inside it.

That is deliberate. A module may need to serve something to a caller who holds
no session — the App Store registry serves a catalogue every instance reads
before anybody signs in — and a platform that could not express that would force
such a module out into a service of its own.

The cost is that a private route can become public by accident, and nothing in
the diff would say so. So the rule is:

> A route reachable without a session must be named in `publicRoutes` in
> `backend/pkg/platform/route_policy_test.go`.

The test walks the real routing table, calls every route with no credentials,
and fails on anything that answers `200` or `201` without being on the list. It
also fails on a name in the list that nothing serves any more, so a renamed
route cannot leave an entry behind that quietly widens the next route to take
its name.

Adding a name is then a visible act in a review rather than a side effect of
where a line was put. If you find yourself adding one, say in the same comment
what authority the route relies on instead of a session — a signature, a
single-use reference in the query, a client secret — because "public" is never
the actual answer.

### Using platform services

Anything more than one app needs lives in the platform itself — `internal/tenant/`
for what acts inside one organisation, `internal/operator/` for what acts for the
deployment, `internal/kernel/` for what neither owns — and reaches a
module through its constructor, not through a package-level singleton. The
server builds one instance in `NewServer` and passes it in, the way
`gov_services` receives the integration manager.

Email verification is one of these. Do not grow your own token table:

```go
type Module struct {
    db          nexus.DB
    emailVerify *emailverify.Service
}

// The platform first, then whatever this module in particular needs. Services
// that only one or two apps use are passed like this rather than added to
// nexus.Platform — a contract every distribution compiles against should carry
// what every module needs, and nothing more.
func New(p nexus.Platform, emailVerify *emailverify.Service) *Module { /* … */ }

// Somewhere in a handler, with the tenant taken from the request context:
_, err := m.emailVerify.Send(ctx, tenantID, emailverify.Request{
    Email:       invitee.Email,
    Source:      m.ID(),          // kept on the row, so the audit trail names you
    Purpose:     "invoice_portal_invite",
    RedirectURL: "https://portal.example/invited", // optional; HTTPS only
})
```

The mail is sent by the hosted verification service — this platform holds no
mailbox credential and composes no message. `Send` asks for the link, records
the request, and enforces the local sending limits. When the recipient follows
the link they come back to `/api/v1/verify/landed`, the verification is marked
confirmed exactly once, and they are forwarded to the `RedirectURL` you named.

Map the errors rather than reporting them all as server failures:

| Error | Meaning | Answer |
| --- | --- | --- |
| `*emailverify.InvalidError` | a bad address or destination | `400` |
| `*emailverify.RateLimitedError` | carries `RetryAfter` | `429` |
| `ErrNotConfigured`, `ErrOriginNotHTTPS`, `ErrUnauthorizedKey` | this deployment's configuration, not the request | `503` |
| `ErrUpstream` | the service could not send or could not be reached; retryable | `502` |

There is no webhook yet, so a verification is recorded only when the person
returns here. Treat `PENDING` as "we have not seen them come back", not as
"they ignored it".

### Step 4: Create App Manifest JSON
Add a manifest file in `catalog/manifests/invoices.json`:

```json
{
  "id": "io.gerege.nexus.invoices",
  "name": "Invoices",
  "version": "1.0.0",
  "platform": ">=0.1.0 <2.0.0",
  "dependencies": [
    { "id": "io.gerege.nexus.contacts", "version_constraint": "^1.0.0" },
    { "id": "io.gerege.nexus.products", "version_constraint": "^1.0.0" }
  ],
  "permissions": [
    {
      "code": "invoices.read",
      "name": "Read Invoices",
      "description": "Allows viewing invoices"
    },
    {
      "code": "invoices.manage",
      "name": "Manage Invoices",
      "description": "Allows issuing and editing invoices"
    }
  ],
  "menus": [
    {
      "id": "invoices",
      "label": "Invoices",
      "path": "/invoices",
      "icon": "receipt",
      "order": 70
    }
  ]
}
```

The field names must match `appcatalog.Manifest` exactly:

| Field | Type | Notes |
| --- | --- | --- |
| `id` | string | Must equal the `id` of the matching `catalog/apps.json` entry |
| `version` | string | Valid semver |
| `platform` | string | Semver constraint checked against the platform version (`1.0.0`) |
| `dependencies` | **array** of `{id, version_constraint}` | Not an object — `{}` fails to parse |
| `permissions` | **array of objects** `{code, name, description}` | Not an array of strings |
| `menus` | array of `{id, label, path, icon, order}` | `label`/`path`/`order`, not `name`/`action`/`sequence` |

The file name must be `catalog/manifests/<slug>.json`, where `<slug>` is the
slug used in `catalog/apps.json` (lowercase letters, digits, `-` and `_`).
A manifest that fails to load or whose `id` disagrees with the catalog entry is
a **startup error** — the server refuses to boot rather than silently
installing the app with an empty dependency, permission and menu set.

And update `catalog/apps.json` to index the new app in the App Store! The
`apps` database table is synchronised from that file on every boot, so no
manual SQL is required.

---

## External apps: a platform that runs somewhere else

Not every app in the store is a Go module compiled into this binary. A third
party with a service already running — a payroll system, an HR platform, a
sector-specific SaaS — is registered as an **external** app: this platform holds
its catalogue entry, its permissions and its menu entry, and signs its users in
over OIDC. None of its code runs here.

```json
{
  "id": "mn.example.hrms",
  "type": "external",
  "name": "Example HRMS",
  "version": "2026.8.0",
  "platform": ">=1.0.0",
  "external": {
    "launch_url": "https://hrms.example.mn/sso/gerege",
    "sso_client_id": "app_hrms_x1y2",
    "scopes": ["openid", "profile", "email"],
    "embed": "new_tab",
    "health_url": "https://hrms.example.mn/healthz"
  },
  "permissions": [{ "code": "hrms.read", "name": "Open HRMS", "description": "…" }],
  "menus": [{ "id": "hrms_home", "label": "HRMS",
              "external_url": "https://hrms.example.mn/sso/gerege", "icon": "share-2" }]
}
```

A worked example ships as `catalog/manifests/example-external.json`.

What differs from a module manifest:

- `type` is `"external"`. Absent or `"module"` means a compiled Go module, which
  is what every manifest written before this existed says.
- `external.launch_url` is required and must be **absolute HTTPS**. It is put in
  front of a signed-in user as a link this platform vouches for.
- `external.embed` defaults to `new_tab`. This platform sends
  `X-Frame-Options: DENY`, so framing is a decision both sides have to make.
- A menu entry carries `external_url` **instead of** `path`. The shell renders it
  as `target="_blank" rel="noopener noreferrer"` with an external-link icon, and
  it never highlights as "the page you are on" — it is not a route here.
- No Go module is required or looked for. Permissions are granted from the
  manifest rather than from a compiled `Permissions()`.

**Installation is what authorises the sign-in.** `external.sso_client_id` names
an OAuth2 client registered in this platform. At `/oauth2/auth`, a user whose
tenant has not installed (and enabled) the app is refused with
`error=access_denied` — the app gate that keeps an uninstalled module's routes
unreachable, continued into an application that serves no routes here. Tokens
carry `tenant_id` and `tenant_slug` so the third party knows which organisation
the person is acting for.

---

## Maintainers

- **Gerege Systems Development Team** ([@gerege-systems](https://github.com/gerege-systems))
- **Gemini AI**, **Claude AI**
