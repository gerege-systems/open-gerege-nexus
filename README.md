# Gerege Nexus

**Олон байгууллагатай, модуль нэмж бүтээгдэхүүн болгох платформын цөм**

Gerege Nexus нь Go backend, Next.js web shell, PostgreSQL өгөгдлийн сан,
операторын control plane болон native client-уудын суурь юм. Бизнес аппуудыг
compile-time Go module хэлбэрээр distribution репо нэмдэг. Энэ үндсэн репо нь
бүх салбарын бэлэн ERP багц биш.

> **Бодит хүрээ (2026-08-25):** [`catalog/apps.json`](catalog/apps.json) **хоосон**.
> Сүүлчийн апп болох `io.gerege.nexus.sso_clients` мөн өдөр
> [appstore-gerege-nexus](https://github.com/gerege-systems/appstore-gerege-nexus)
> руу гарсан бөгөөд аль ч distribution түүнийг апп стороос татаж авна. Contacts,
> products, inventory, billing, documents, government workflow, SSO клиент зэрэг
> аппын migration history эсвэл frontend translation энэ репод үлдсэн байж болох ч
> тэдгээрийн ажиллах module энэ үндсэн бинарьд байхгүй. Тийм аппын маршрут энэ
> deployment дээр нээгдэхгүй.

<p>
  <b>Монгол</b>
  &nbsp;·&nbsp;
  <a href="docs/README_AR.md">العربية</a>
  &nbsp;·&nbsp;
  <a href="docs/README_ZH.md">中文</a>
  &nbsp;·&nbsp;
  <a href="docs/README_EN.md">English</a>
  &nbsp;·&nbsp;
  <a href="docs/README_FR.md">Français</a>
  &nbsp;·&nbsp;
  <a href="docs/README_RU.md">Русский</a>
  &nbsp;·&nbsp;
  <a href="docs/README_ES.md">Español</a>
</p>

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.26-00ADD8.svg)](https://go.dev)
[![Next.js](https://img.shields.io/badge/Next.js-16.3.1-black.svg)](https://nextjs.org)
[![CI](https://github.com/gerege-systems/open-gerege-nexus/actions/workflows/ci.yml/badge.svg)](https://github.com/gerege-systems/open-gerege-nexus/actions/workflows/ci.yml)

## Энэ репо яг юу агуулдаг вэ

| Хэсэг | Одоогийн хэрэгжилт |
| --- | --- |
| Tenant plane | Нэвтрэлт, session, tenant membership, RBAC, profile, апп суулгалт/хаалт, integration, AI API, төхөөрөмж, email verification, SSO client/provider, Өртөөний суваг, reporting engine |
| Operator plane | Тенантын lifecycle, операторын TOTP нэвтрэлт, audit, support, quota/metering, feature flag, runtime settings/credentials, announcement, backup/deploy удирдлага |
| Built-in апп | Байхгүй. Апп бүр каталогоор ирнэ — 2026-08-25-нд `sso_clients` явсны дараа энэ бинарь нэг ч бизнес апп агуулахгүй |
| Web | Public landing, login/setup, profile, app store, settings, control plane; аппын дэлгэцүүд (SSO клиент, баримт, харилцагч) module-гүйгээ амьгүй байдлаар shell-д үлдсэн |
| Native | macOS, iOS/iPadOS, Windows, Android shell; Linux-д PWA |
| Observability | `/health`, `/ready`, `/metrics`, structured log, optional OTLP/Sentry; тусдаа monitoring compose |

Үндсэн платформын route, хүснэгт, service байлаа гээд хэрэглэгчид харагдах
бизнес апп заавал байна гэсэн үг биш. Compile-time module нь
[`pkg/nexus`](backend/pkg/nexus)-ийн гэрээг хэрэгжүүлж, binary-д бүртгэгдсэн,
каталогт орсон, тухайн tenant-д суусан үед л module route ажиллана.

### Энэ үндсэн бинарьд байхгүй зүйлс

- Contacts, products, inventory, billing бизнес аппууд
- Organisation & People-ийн хэлтэс/ажилтны апп (харин байгууллагын legal
  profile `/organisation` дээр платформын хэсэг хэвээр)
- Documents backend module, report UI. Contract/inbox-ийн optional frontend
  client энэ repo-д бий ч түүнд таарах contract API одоогийн base болон
  `client-gerege-nexus` distribution-д хараахан байхгүй; endpoint бодитоор
  хариулахгүй бол landing дээр холбоос нь гарахгүй
- Government services workflow UI/module
- Өртөөний task board (суваг ба peer/exchange API нь платформд үлдсэн)
- Ерөнхий adaptive circuit breaker, singleflight, generic retry engine

Эдгээрийн зарим нь product distribution репо руу салсан. Түүхэн шийдвэр ба
хуваарилалтыг [`docs/ECOSYSTEM_GIT_STRATEGY.md`](docs/ECOSYSTEM_GIT_STRATEGY.md),
одоогийн архитектурыг
[`docs/ARCHITECTURE_SPECIFICATION.md`](docs/ARCHITECTURE_SPECIFICATION.md)-ээс
үзнэ үү.

## Гол боломжууд

- Compile-time module SDK: route, permission, menu, dependency, migration,
  report, AI tool болон capability гэрээнүүд.
- Tenant бүрийн app installation ба RBAC gate. Суугаагүй/унтраасан module route
  `403` буцаана.
- Local catalog эсвэл Ed25519 гарын үсэгтэй remote catalog; cache fallback ба
  manual/background sync.
- Password, eID, ДАН, Google болон upstream OIDC нэвтрэлт; мөн OAuth2/OIDC
  provider.
- PostgreSQL-ийн `tenant`, `registry`, `operator` schema, RLS/role хамгаалалт.
- Redis тохируулсан үед replica хооронд cache invalidation ба shared rate
  limit; тохируулаагүй үед single-process fallback.
- Зэрэг хүсэлтийн load shedding. Гадаад client бүр өөрийн timeout/retry
  бодлоготой; байхгүй generic circuit breaker-ийг платформтой гэж үзэхгүй.
- Gemini түлхүүртэй үед chat, speech-to-text, text-to-speech, translation.
  Бизнес өгөгдлийн AI tool-ыг distribution module өөрөө бүртгэнэ; үндсэн
  deployment inventory эсвэл billing тоо зохиож буцаахгүй.
- E-ID, ДАН, ХУР client-ууд платформд бий. `production` орчинд mock нь
  анхдагчаар унтарна; бодит service ашиглахын тулд тус тусын credential хэрэгтэй.

## Репогийн бүтэц

```text
backend/
  cmd/                  api, migrate, bootstrap, catalog хэрэгслүүд
  db/migrations/        үндсэн PostgreSQL migration-ууд
  internal/kernel/      хоёр plane-д нийтлэг доод түвшний механизм
  internal/operator/    deployment/operator plane
  internal/tenant/      байгууллагын нэрийн өмнөөс ажиллах plane
  internal/apps/        distribution module-ийн угсрах цэг (одоо хоосон)
  pkg/nexus/            distribution module-ийн нийтийн SDK
  pkg/host/             distribution binary-г асаах нийтийн entry point
frontend/               Next.js web shell ба control plane UI
catalog/                үндсэн catalog, manifest, chronicle
native-apps/            macOS, iOS/iPadOS, Windows, Android client
deploy/                 image, compose, nginx, monitoring
docs/                   одоогийн, түүхэн, proposal баримтуудын индекс
```

## Ажиллуулах

### Урьдчилсан шаардлага

- Docker Compose, эсвэл
- Go 1.26+, Node.js 20+, PostgreSQL 16+

### Docker Compose

```bash
docker compose up -d
docker compose ps
```

Compose нь PostgreSQL, Redis, MinIO, нэг удаагийн migration, backend, frontend
асаана. Дараах хаягуудыг ашиглана:

- Tenant web: <http://nexus.localhost:3000>
- Control plane web: <http://cp.localhost:3000>
- API health: <http://localhost:8080/health>

Development demo account:

| Талбар | Утга |
| --- | --- |
| И-мэйл | `admin@example.com` |
| Нууц үг | `Password123!` |
| Tenant | `Demo Corporation` (`demo`) |

`SEED_DEMO_DATA` нь production-оос бусад орчинд анхдагчаар идэвхтэй.
Production-д explicit `true` өгөөгүй бол demo account үүсэхгүй.

### Гараар

Эхлээд PostgreSQL (шаардвал Redis)-ийг асаагаад:

```bash
cd backend
go mod download
DATABASE_URL="postgres://postgres:postgrespassword@localhost:5432/platform_db?sslmode=disable" \
  go run ./cmd/migrate up

PUBLIC_ORIGIN=http://nexus.localhost:3000 \
ALLOWED_ORIGINS=http://nexus.localhost:3000,http://cp.localhost:3000 \
CONTROL_PLANE_HOST=cp.localhost \
  go run ./cmd/api
```

Өөр terminal-д:

```bash
cd frontend
npm ci
CONTROL_PLANE_HOST=cp.localhost \
NEXT_PUBLIC_API_URL=http://nexus.localhost:8080/api/v1 \
NEXT_PUBLIC_CONTROL_PLANE_API_URL=http://cp.localhost:8080/api/platform/v1 \
  npm run dev
```

Production тохиргоог README-гээс таахгүй. Бүрэн хувьсагч ба аюулгүй default-ыг
[`.env.example`](.env.example), production жишээг
[`deploy/.env.prod.example`](deploy/.env.prod.example), анхны tenant/operator
үүсгэх алхмыг [`docs/CONTROL_PLANE.md`](docs/CONTROL_PLANE.md)-ээс дагана.

## API ба каталогийн бодит эх сурвалж

README дахь API жагсаалт бүрэн contract биш. Кодтой хамт шалгагддаг эх
сурвалжууд:

- Platform route snapshot:
  [`backend/pkg/host/testdata/routes.txt`](backend/pkg/host/testdata/routes.txt)
- Module SDK public API snapshot:
  [`backend/pkg/nexus/testdata/api.txt`](backend/pkg/nexus/testdata/api.txt)
- Base catalog: [`catalog/apps.json`](catalog/apps.json) (хоосон)
- Module SDK-ийн SSO гэрээ:
  [`backend/pkg/nexus/ssoclients.go`](backend/pkg/nexus/ssoclients.go)
- Тохиргоо: [`.env.example`](.env.example)

Үндсэн public endpoint-д `/health`, `/ready`, `/metrics`, auth, setup,
OAuth2/OIDC, app store, device enrollment, email verification болон Өртөөний
exchange орно. Tenant session шаарддаг route-ууд, control plane route-ууд,
module route-ууд тус тусдаа хамгаалалттай.

## Тест

```bash
cd backend
go test ./...
go vet ./...

cd ../frontend
npm ci
npm test
npm run lint
npm run build
```

CI нь үүн дээр migration-backed race test, API/route boundary, PWA, host,
translation-gap report, Docker build, `govulncheck`, `gosec` болон native client
build нэмнэ. Яг ажилладаг командуудыг [`.github/workflows/ci.yml`](.github/workflows/ci.yml),
[`security.yml`](.github/workflows/security.yml),
[`native-clients.yml`](.github/workflows/native-clients.yml)-ээс үзнэ үү.

## Хэл ба баримтын статус

UI-ийн эх мөр монгол, англи хэлтэй; араб, хятад, франц, орос, испани overlay
бий. Дутуу overlay англи хэл рүү fallback хийдэг бөгөөд CI цоорхойг
тайлагнадаг — бүх дэлгэц 100% орчуулагдсан гэж амлахгүй.

README долоон хэлтэй. Техникийн баримт бүр долоон орчуулгатай биш; файл бүрийн
хэл ба статусыг [`docs/README.md`](docs/README.md)-д тэмдэглэсэн. `PLAN`,
`PROPOSAL`, `PROMPT`, `WORKLOG` нэртэй файл нь одоогийн runtime contract биш.
Ажиллуулах зааварт тэдгээрийг дангаар нь эх сурвалж болгож болохгүй.

## Аюулгүй байдал ба лиценз

Эмзэг байдлыг public issue-д нийтлэхгүй; [`SECURITY.md`](SECURITY.md)-ийн
private reporting журмыг ашиглана.

Apache License 2.0 — [`LICENSE`](LICENSE).
