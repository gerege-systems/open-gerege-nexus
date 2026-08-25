# Архитектурын тодорхойлолт

**Gerege Nexus**-ын одоогийн кодын бүтэц, хоёр урсгалын хил ба өгөгдлийн
эзэмшил. Шинэчлэгдсэн: 2026-08-25.

<p>
  <img src="assets/icons/flag-mn.png" width="18" height="18" alt=""> <b>Монгол</b>
  &nbsp;·&nbsp;
  <a href="ARCHITECTURE_SPECIFICATION_EN.md"><img src="assets/icons/flag-en.png" width="18" height="18" alt=""> English</a>
</p>

[Баримт бичгийн төв](README.md) · [Control plane](CONTROL_PLANE.md) ·
[ADR-0005](adr/0005-two-planes-one-origin-each.md)

---

## 1. Нэг процесс, хоёр урсгал

Gerege Nexus нь Go + Next.js + PostgreSQL дээрх **модульт монолит**. Нэг
`cmd/api` бинарь, нэг image, нэг deploy дотор хоёр бие даасан хүсэлтийн урсгал
ажиллана:

| | Тенантын урсгал | Платформын урсгал |
| --- | --- | --- |
| Хариуцах зүйл | Нэг байгууллага доторх хэрэглэгчийн ажил | Бүх deployment-ийг оператор удирдах |
| Origin | `nexus.gerege.mn` | `cp.nexus.gerege.mn` |
| API | `/api/v1/*` | `/api/platform/v1/*` |
| Session cookie | `session_token` | `cp_session` |
| Бүртгэл | `platform.users` + `tenant.memberships` | `platform.operator_accounts` |
| DB role | `gerege_nexus_tenant` | `gerege_nexus_operator` |
| Go package | `internal/tenant/*` | `internal/platform/*` |

Операторын бүртгэл нь хэрэглэгчийн бүртгэл биш. Нэг хүн хоёр урсгалд зэрэг
нэвтэрч болох ч тусдаа identity, cookie, эрх, audit ашиглана. Оператор тенантын
нүдээр харах шаардлагатай бол шалтгаантай, 30 минутын impersonation урсгалыг
ашиглана.

```text
tenant origin ─┐                         ┌─ internal/tenant/* ─ tenant schema
               ├─ pkg/platform/server.go ┤
control origin ┘   shared middleware     └─ internal/platform/* ─ platform schema
                          │
                    internal/kernel/*
                          │
                      PostgreSQL
```

`backend/pkg/platform/server.go` нь хоёр урсгалын composition root: дундын
store, middleware, router-ийг босгож хоёр route table-ийг зэрэг mount хийнэ.
Хоёр урсгал хоорондоо import хийхгүй; `internal/planes_test.go` үүнийг
компиляцын граф дээр шалгана.

## 2. Кодын хил ба хариуцлага

| Байршил | Хариуцлага |
| --- | --- |
| `backend/internal/kernel` | Аль ч урсгалыг import хийдэггүй cache, config, security, telemetry, settings, flags зэрэг суурь primitive |
| `backend/internal/tenant` | Auth, access, directory, devices, identity, integrations, profile, SSO, app install зэрэг нэг тенантын ажиллагаа |
| `backend/internal/platform` | Operator session, tenants, approvals, settings, flags, audit, support, metering, backup, catalog, observability |
| `backend/internal/apps` | Distribution модулийн угсрах цэг. 2026-08-25-нд SSO Clients App Store руу явсны дараа **хоосон** — апп бүр `pkg/nexus`-ээр бүртгэгдэж каталогоор ирнэ |
| `backend/pkg/platform` | Хоёр урсгалыг нэг HTTP процесст угсрах public host package |
| `backend/pkg/nexus` | Гадаад module/distribution-д зориулсан тогтвортой SDK contract |

Plane-ийн үндсэн package нь зөвхөн дэд package-уудаа угсарна. Handler, store,
бизнес логик шинэчлэгдэхдээ зохих домэйн дэд package-д орно. Одоогийн
`internal/tenant/service.go` нь дараагийн задралын ажил хэвээр; энэ нь хоёр
урсгалын import/schema хилийг сулруулах зөвшөөрөл биш.

## 3. Хүсэлтийн урсгал

### 3.1 Дундын давхарга

Хоёр урсгал `pkg/platform/server.go` дээр request ID, tracing, structured log,
panic recovery, load shedding, metrics, security headers, CORS, CSRF
middleware-ийг хуваалцана. `/health`, `/ready`, `/metrics` нь аль нэг plane-д
харьяалагдахгүй process endpoint.

### 3.2 Тенантын хүсэлт

1. `session_token` эсвэл зөвшөөрөгдсөн bearer token-оос хэрэглэгчийг танина.
2. Идэвхтэй тенантыг context-д тогтооно.
3. `dbguard` query-г `gerege_nexus_tenant` role-оор ажиллуулна.
4. PostgreSQL RLS ба `tenant_id` тухайн байгууллагын мөрөөр хязгаарлана.
5. Module route бол `tenant.app_installations` ба kill switch-ийг шалгана.

### 3.3 Платформын хүсэлт

1. `HostGate` зөвхөн `CONTROL_PLANE_HOST` origin-ыг нэвтрүүлнэ.
2. `cp_session`-ийг шалгаж, нууц үг + TOTP, богино idle timeout болон
   шаардлагатай үед step-up хэрэглэнэ.
3. Query бүр `gerege_nexus_operator` role-оор ажиллана.
4. Бичих үйлдэл ба `platform.operator_audit` мөр нэг transaction-д commit
   хийнэ; audit бичигдээгүй write амжилт болохгүй.

Production-д nginx-ийн CIDR allowlist нь HostGate-ээс өмнө ажиллана. Тиймээс
origin, session, DB role, audit нь тус тусдаа хамгаалалтын давхарга юм.

## 4. Өгөгдлийн сангийн эзэмшил

Миграц `00079_two_schemas.sql` хүснэгтүүдийг `platform` ба `tenant` schema-д
салгаж, `00080_search_path_has_no_public.sql` runtime замаас `public`-ийг
хассан.

| Schema | Эзэмшдэг өгөгдөл |
| --- | --- |
| `platform` | tenants, users, apps, operator account/session/audit, approvals, settings, flags, announcements, quota, usage, backup metadata |
| `tenant` | memberships, roles, sessions, app installations, profile/directory/device/integration/SSO/audit өгөгдөл |
| `public` | goose migration ledger болон зориуд үлдээсэн `SECURITY DEFINER` function |

Одоогийн migration inventory нь 27 platform, 40 tenant хүснэгттэй. Энэ тоо
дангаараа contract биш; `backend/db/migrations/ownership_test.go`-д нэр бүрийн
эзэмшлийг зарласан бөгөөд `schema_split_test.go` бодит DB-тэй тулгана.

Тенант role-д `platform` schema-ийн `USAGE` хэрэгтэй: announcements, feature
flag overrides, operator impersonations, tenant quotas, usage events гэсэн
таван boundary хүснэгтийг нэрээр нь уншина. Иймээс бодит хил нь schema USAGE
биш, **хүснэгтийн түвшний grant**. `platform` schema-д шинээр үүсэх хүснэгт
тенант role-д анхдагчаар хаалттайг DB integration test батална.

Бүх DDL `backend/db/migrations/` дахь goose migration-аар орно. Runtime DDL
хоригтой; distribution module өөрийн migration-ийг `pkg/nexus` contract-аар
нийлүүлнэ.

## 5. Модуль ба каталог

Core нь business app-ийн хүснэгт, handler-ийг эзэмшихгүй. Distribution нь
module code, manifest, migration-аа нийлүүлж, Nexus SDK contract-аар бүртгэнэ.
Платформ каталогийг татаж `platform.apps` metadata-г синк хийнэ; тенантын
суулгалт, хувилбар, төлөв `tenant.app_installations`-д хадгалагдана.

AI stock forecast endpoint ч built-in inventory table ашиглахгүй. Идэвхтэй
distribution `stock_forecast` capability нийлүүлсэн үед delegation хийж,
байхгүй бол `404` буцаана.

## 6. Олон replica ба тэсвэрлэлт

- `kernel/resilience/loadshedder.go` зэрэг ажиллах хүсэлтийг хязгаарлана.
- `kernel/cache.Bus` нь Redis pub/sub байгаа үед replica хооронд cache
  invalidation түгээнэ; Redis байхгүй бол local ажиллагаа хэвээр.
- `kernel/memo` эрх ба суулгалтын шийдвэрт зориулсан богино TTL-тэй,
  prefix-ээр хүчингүй болдог process-local cache өгнө.
- `kernel/async` нэртэй goroutine-ыг panic recovery, stack log-той ажиллуулна.
- Settings ба feature flags event invalidation-аас гадна хугацаат refresh-тэй.

## 7. Архитектурын автомат хамгаалалт

| Инвариант | Баталгаа |
| --- | --- |
| Plane хооронд шууд import байхгүй | `backend/internal/planes_test.go` |
| Хүснэгт зөв schema-д байна | `backend/db/migrations/ownership_test.go`, `schema_split_test.go` |
| SQL хүснэгтээ schema-аар тодорхой заана | `backend/db/migrations/qualification_test.go` |
| Тенант role зөвхөн таван boundary хүснэгт уншина | `schema_split_test.go` |
| Шинэ platform хүснэгт анхдагчаар хаалттай | `TestNewPlatformTableIsClosedToTenantRole` |
| API route санамсаргүй өөрчлөгдөхгүй | `backend/pkg/platform/testdata/routes.txt` |
| Origin ба `/cp` host routing | `frontend/tests/control-plane-host.test.mjs`, `frontend/scripts/check-control-plane-host.mjs`, `frontend/scripts/smoke-control-plane-host.mjs` |

Шийдвэрийн үндэслэлийг [хоёр урсгалын санал](TWO_PLANES_PROPOSAL.md),
[хэрэгжилтийн шалгалт](TWO_PLANES_REVIEW.md),
[ADR-0005](adr/0005-two-planes-one-origin-each.md)-аас үзнэ үү. Санал, prompt,
plan баримтууд нь түүхэн design record; энэ файл ба ажиллаж буй код нь
одоогийн эх сурвалж.
