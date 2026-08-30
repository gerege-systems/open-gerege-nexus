# Gerege Nexus

**Integrated Digital Operations Platform**

**Gerege Nexus** is an open-source modular platform that connects services,
operations, systems, and data across public and private organizations. It is
**Mongolian-first** and integrates directly with Mongolia's national digital
infrastructure (DAN, E-ID, XYP / ХУР).

*Nexus* is the connection point: where organizations, services, workflows,
systems, users and data meet. The platform itself is not tied to one sector —
the modules running on it are what make a deployment specific.

Modules compile into a single Go binary, while a PostgreSQL-backed app
store decides which apps are active per tenant — module separation without the
network hops or operational cost of microservices.

**Language policy: Mongolian plus the six official languages of the United
Nations** — Arabic, Chinese, English, French, Russian, Spanish. Seven in total.
Mongolian is the source. This product overview exists in all seven languages;
individual technical documents do not necessarily do so. The UI has Mongolian
and English source dictionaries plus Arabic, Chinese, French, Russian, and
Spanish overlays; missing overlay strings fall back to English. See the
[translation guide](TRANSLATION.md).

<p>
  <a href="../README.md"><img src="assets/icons/flag-mn.png" width="18" height="18" alt=""> Монгол</a>
  &nbsp;·&nbsp;
  <a href="README_AR.md"><img src="assets/icons/flag-ar.png" width="18" height="18" alt=""> العربية</a>
  &nbsp;·&nbsp;
  <a href="README_ZH.md"><img src="assets/icons/flag-zh.png" width="18" height="18" alt=""> 中文</a>
  &nbsp;·&nbsp;
  <img src="assets/icons/flag-en.png" width="18" height="18" alt=""> <b>English</b>
  &nbsp;·&nbsp;
  <a href="README_FR.md"><img src="assets/icons/flag-fr.png" width="18" height="18" alt=""> Français</a>
  &nbsp;·&nbsp;
  <a href="README_RU.md"><img src="assets/icons/flag-ru.png" width="18" height="18" alt=""> Русский</a>
  &nbsp;·&nbsp;
  <a href="README_ES.md"><img src="assets/icons/flag-es.png" width="18" height="18" alt=""> Español</a>
</p>

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](../LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.26-00ADD8.svg)](https://go.dev)
[![Next.js](https://img.shields.io/badge/Next.js-16.3-black.svg)](https://nextjs.org)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](../CONTRIBUTING.md)

---

## Contents

- [Authors](#authors)
- [Core capabilities](#core-capabilities)
- [Business applications](#business-applications)
- [Repository layout](#repository-layout)
- [Getting started](#getting-started)
- [Configuration](#configuration)
- [API overview](#api-overview)
- [Testing and quality gates](#testing-and-quality-gates)
- [Security](#security)
- [Documentation index](#documentation-index)

---

## Authors

| Contributor | Role |
| --- | --- |
| **Gerege Systems Development Team** ([@gerege-systems](https://github.com/gerege-systems)) | Architecture, platform core |
| **Gemini AI** | Code generation, documentation |
| **Claude AI** | Code analysis, security audit |

---

## Core capabilities

### 1. High-performance modular monolith

- **Compile-time Go app modules** — core carries only `sso_clients`.
  Product distributions register their modules through the public `pkg/nexus`
  contract in the final binary, where they are invoked in-process.
- **Per-tenant app store** — application entitlements, menus and RBAC are driven
  from PostgreSQL (`app_installations`).
- **Dependency resolver** — recursive resolution over a directed acyclic graph
  with cycle detection and semver constraint checking.
- **Catalog sync** — production fetches a signed catalog from
  `APP_CATALOG_URL`; development/offline mode falls back to
  `catalog/apps.json`, and metadata is reconciled into `platform.apps`.

### 2. Cloud-native resilience and multiple replicas

| Module | Purpose |
| --- | --- |
| `internal/kernel/resilience/loadshedder.go` | Sheds load with `503` + `Retry-After` under pressure |
| `internal/kernel/cache/bus.go` | Redis-backed invalidation across replicas, with a local fallback |
| `internal/kernel/memo/memo.go` | Short-TTL, prefix-invalidated local cache for authorisation decisions |
| `internal/kernel/async/async.go` | Named goroutines with panic recovery and stack logging |

### 3. National digital infrastructure

- **XYP — State Information Exchange** (`internal/workspace/identity/gerege/xyp.go`): citizen civil
  registration (`WS100101`) and legal entity verification (`WS100201`).
- **National E-ID and DAN** ([`developer.gerege.mn`](https://developer.gerege.mn),
  [`eidmongolia.mn`](https://eidmongolia.mn)) — PKI digital signature, mobile
  OTP, bank SSO and biometric face verification.
- **Built-in OAuth2 / OIDC provider**
  (`/.well-known/openid-configuration`) issuing client-credentials tokens to
  third-party systems.
- **Email verification** (`internal/workspace/emailverify`) — one shared flow for proving
  an address, called in process by every app module. The mail is sent by the
  hosted service (`enigma.mn`), so the platform holds no mailbox credential and
  owns no sender address; the verification is recorded when the person comes
  back, and that return works exactly once. Visible under Settings → Email
  verification.

> **Note.** Mock mode for E-ID, DAN and XYP is a development convenience only.
> With `ENVIRONMENT=production` it is disabled automatically, so a fabricated
> registration number can never authenticate.

### 4. AI copilot and analytics

- **AI assistant** (`internal/workspace/ai/copilot.go`) — intent-classified conversation
  wired to live tenant data.
- **Stock forecast endpoint** (`internal/workspace/ai/handlers.go`) — delegates to
  an enabled distribution's `stock_forecast` capability and returns `404` when
  no module provides it.

---

## Business applications

This base repository ships exactly one app in `catalog/apps.json`. Product
distributions register their own modules and migrations through `pkg/nexus`;
their apps must not be read as features included in this repository.

| # | Application | ID | Route | Description |
| --- | --- | --- | --- | --- |
| 1 | SSO Clients | `io.gerege.nexus.sso_clients` | `/sso-clients` | OAuth2 clients for systems that sign people in through this platform |

Routes only open once the app is installed and enabled for the tenant; otherwise
the gate returns `403 Forbidden`.

---

## Repository layout

```
backend/
  cmd/api/            HTTP API server (+ demo seeder)
  cmd/migrate/        Goose migration runner
  db/migrations/      SQL migrations
  internal/
    kernel/           Plane-neutral technical primitives
    tenant/           Work performed for one organisation
    platform/         Operations for the whole deployment
    apps/             Modules carried by this distribution
  pkg/
    nexus/            Public SDK and contracts for external modules
    platform/         Composition root for both planes
frontend/             Next.js 16 (App Router) web client
catalog/              App store catalog and manifests
deploy/               Production Dockerfile, Nginx config
docs/                 Documentation and translations
```

---

## Getting started

### Prerequisites

- Go 1.26+
- Node.js 20+
- PostgreSQL 16+ (or Docker Compose)

### 1. Docker Compose

```bash
docker compose up -d
```

Migrations run as a dedicated one-shot `migrate` service before the API starts.

### 2. Manual

**Backend:**

```bash
cd backend
go mod download
DATABASE_URL="postgres://postgres:postgrespassword@localhost:5432/platform_db?sslmode=disable" \
  go run ./cmd/migrate up
go run ./cmd/api
```

**Frontend:**

```bash
cd frontend
npm ci
npm run dev
```

Open [http://localhost:3000](http://localhost:3000).

### Demo credentials

| Field | Value |
| --- | --- |
| Email | `admin@example.com` |
| Password | `Password123!` |
| Tenant | `Demo Corporation` (`slug: demo`) |

The demo account is only seeded outside production. In production it is created
only when `SEED_DEMO_DATA=true` is set explicitly.

---

## Automated deployment

Every push to `main` runs [`deploy.yml`](../.github/workflows/deploy.yml):

1. Build and push the backend and frontend images to GHCR (`:latest` and `:<sha>`).
2. Copy `docker-compose.prod.yml` to the server.
3. Write the server `.env` from GitHub secrets and pull the images.
4. Run migrations to completion, then swap the API and frontend over.
5. Probe `/health` and `/ready`, printing container logs and failing the run if
   the rollout is unhealthy.

Deploy manually from Actions → *Deploy to Production* → **Run workflow**,
optionally pinning an image tag.

Required repository secrets:

| Secret | Required | Description |
| --- | --- | --- |
| `DEPLOY_SSH_KEY` | Yes | Private key of the deploy user. Without it the rollout is skipped |
| `POSTGRES_PASSWORD` | Yes | Database password on the server |
| `SSO_DEFAULT_CLIENT_SECRET` | Yes | Mandatory for the built-in OAuth2 client in production |
| `DEPLOY_HOST` / `DEPLOY_USER` / `DEPLOY_PORT` | No | Default to `nexus.gerege.mn` / `deploy` / `22` |
| `PUBLIC_ORIGIN` | No | Defaults to `https://nexus.gerege.mn` |

> The production domain is `nexus.gerege.mn`, which replaced
> `openerp.gerege.mn` in the Gerege Nexus rename. `PUBLIC_ORIGIN` defines CORS,
> the OIDC issuer and the eID callback in one place, so moving it carries DNS,
> the TLS certificate and every client that pinned the issuer along with it.

The server needs Docker only — no source tree and no Go/Node toolchain. See
[`deploy/.env.prod.example`](../deploy/.env.prod.example) for the values.

---

## Configuration

See [`.env.example`](../.env.example) for the complete list.

| Variable | Default | Description |
| --- | --- | --- |
| `DATABASE_URL` | localhost | PostgreSQL connection string |
| `PORT` | `8080` | API listen port |
| `ENVIRONMENT` | `development` | `production` enables hardened defaults |
| `APP_CATALOG_PATH` | `catalog/apps.json` | App store catalog path |
| `ALLOWED_ORIGINS` | `http://localhost:3000` | CORS allow-list |
| `TRUST_PROXY_HEADERS` | `false` | Whether to trust `X-Forwarded-For` |
| `SEED_DEMO_DATA` | on outside production | Create the demo account |
| `SSO_DEFAULT_CLIENT_SECRET` | — | Required in production |
| `EID_MOCK_MODE` / `DAN_MOCK_MODE` / `XYP_MOCK_MODE` | on outside production | Mock national integrations |

---

## API overview

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/health`, `/ready` | Liveness and readiness probes |
| `GET` | `/metrics` | Prometheus metrics |
| `POST` | `/api/v1/auth/login` | Email and password login |
| `POST` | `/api/v1/auth/eid/login` | National E-ID login |
| `POST` | `/api/v1/auth/dan/login` | DAN gateway login |
| `POST` | `/api/v1/auth/logout` | Revoke the session |
| `GET` | `/api/v1/menus` | Menus for the tenant's enabled apps |
| `GET` | `/api/v1/store/apps` | App store listing |
| `POST` | `/api/v1/store/apps/{slug}/install` | Install an app (admin) |
| `POST` | `/api/v1/verify/send` | Ask the hosted service for an email verification link |
| `GET` | `/api/v1/verify/landed` | Receive somebody who confirmed — good exactly once |
| `GET` | `/api/platform/v1/email-verifications` | Verification ledger and service health (console) |
| `POST` | `/oauth2/token` | OAuth2 client credentials token |

Session tokens travel either in the HttpOnly cookie or as
`Authorization: Bearer <token>`.

---

## Testing and quality gates

```bash
# Backend unit tests with the race detector
cd backend && go test -race ./...

# Static analysis
cd backend && go vet ./... && golangci-lint run

# Vulnerability scan
cd backend && govulncheck ./...

# Frontend build
cd frontend && npm run build
```

CI runs lint, tests, the frontend build, the Docker image build, govulncheck and
gosec on every push and pull request.

---

## Security

- Session tokens are 256-bit random values; only their SHA-256 digest is stored.
- Passwords are hashed with bcrypt and login attempts are rate limited per IP.
- Installing, enabling or disabling apps and registering integrations require
  tenant administrator rights.
- OAuth2 client authentication uses constant-time comparison.

Report vulnerabilities as described in [`SECURITY.md`](../SECURITY.md).

---

## Documentation index

| Document | Description |
| --- | --- |
| [Documentation hub](README.md) | Index of every document and translation |
| [Architecture](ARCHITECTURE.md) | The two planes, the three schemas, how data is isolated |
| [Writing a module](MODULES.md) | The `pkg/nexus` contract, and how an app reaches a deployment |
| [Translation](TRANSLATION.md) | Language policy, and adding a language with Gemini |
| [Contributing](../CONTRIBUTING.md) | Contribution workflow |
| [Security policy](../SECURITY.md) | Reporting vulnerabilities |
| [Code of conduct](../CODE_OF_CONDUCT.md) | Community standards |
| [Changelog](../CHANGELOG.md) | Release history |

---

## Credits and inspiration

1. **[snykk/go-rest-boilerplate](https://github.com/snykk/go-rest-boilerplate)**
   by **[@snykk](https://github.com/snykk)** — Go REST API foundations.
2. **[Odoo](https://github.com/odoo/odoo)** — modular app store and dependency
   model.
3. **[go-zero](https://github.com/zeromicro/go-zero)** — cloud-native resilience
   engine.

---

## License

Copyright (c) 2026 **Gerege Systems Development Team, Gerege Nomadica Foundation**. Distributed under the Apache 2.0 License — see
[`LICENSE`](../LICENSE).

Flag icons by [Flaticon](https://www.flaticon.com/)
([attribution](assets/icons/ATTRIBUTION.md)).
