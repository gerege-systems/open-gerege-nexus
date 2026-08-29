# Хоёр урсгал — хэрэгжилтийн дараах шалгалт

[`TWO_PLANES_PROPOSAL.md`](TWO_PLANES_PROPOSAL.md)-ын хэрэгжилтийг одоогийн
`main` кодтой тулгасан уншилт. Огноо: 2026-08-25.

> Энэ файл өмнө нь хэрэгжүүлэх саналуудыг жагсааж байсан. Host routing, local
> хоёр origin, audit UI, шинэ platform хүснэгтийн default-deny test аль хэдийн
> кодод орсон тул тэдгээрийг “хийх” жагсаалтаас хасаж, доор нотолгоотой
> хэрэгжсэн төлөвөөр тэмдэглэв.

[Баримт бичгийн төв](README.md) · [Одоогийн архитектур](ARCHITECTURE_SPECIFICATION.md) ·
[Control-plane runbook](CONTROL_PLANE.md) ·
[ADR-0005](adr/0005-two-planes-one-origin-each.md)

---

## 1. Дүгнэлт

Хоёр урсгалын суурь refactor бүрэн орсон: нэг процесс дотор tenant/platform
package, schema, role, API, cookie, origin, test тусдаа. `controlplane` гэсэн
бүхнийг хамарсан package устаж, platform plane домэйн дэд package-уудаар
угсрагддаг болсон.

| Инвариант | Одоогийн төлөв | Нотолгоо |
| --- | --- | --- |
| Нэг бинарь, хоёр plane | ✅ | `backend/pkg/platform/server.go` |
| Plane хооронд import байхгүй | ✅ | `backend/internal/planes_test.go`; `crossPlaneExceptions` хоосон |
| Platform root зөвхөн composition | ✅ | `internal/platform/service.go` — 266 мөр |
| Tenant домэйнүүд задарсан | ✅ | `internal/tenant` дотор 18 дэд package |
| DB хоёр schema | ✅ | `ownership_test.go` — 27 platform, 40 tenant хүснэгт |
| Runtime `public` search path-гүй | ✅ | `00080_search_path_has_no_public.sql` |
| Tenant role нэр тодорхой | ✅ | `gerege_nexus_tenant`; хуучин `gerege_nexus_app` байхгүй |
| Canonical platform API | ✅ | `/api/platform/v1` — 44 route |
| Legacy API зөвхөн шилжилт | ✅ | `/cp/api/*` → HostGate-ийн ард 308 |
| Origin хоёр талдаа хаалттай | ✅ | backend HostGate, frontend proxy/layout, nginx allowlist |
| Audit унших UI | ✅ | `frontend/app/cp/audit/page.tsx` |

## 2. Schema split-ийн чухал шийдлүүд

`00079_two_schemas.sql` анхны саналын хүснэгт нүүлгэлтээс гадна production-д
зайлшгүй дөрвөн зүйлийг зөв барьсан:

1. `SECURITY DEFINER` function-уудын `search_path`-ыг тодорхой болгосон.
2. Role-оос гадна database default `search_path`-ыг тохируулсан.
3. `tenant` schema-д ирээдүйд үүсэх module хүснэгтийн default privileges-ийг
   зөв олгосон.
4. `public`-д үлдсэн module хүснэгтийг үлдэгдлээр шүүрдэж, goose ledger болон
   зориудын function-уудыг үлдээсэн.

### Бодит хил нь table grant

`gerege_nexus_tenant` role-д `platform` schema-ийн `USAGE` бий. Энэ нь алдаа
биш: доорх таван boundary хүснэгтийг нэрээр нь resolve хийхэд шаардлагатай.

```text
platform.announcements
platform.feature_flag_overrides
platform.operator_impersonations
platform.tenant_quotas
platform.usage_events
```

`USAGE` нь хүснэгтийн мөр нээхгүй. Бодит хил нь энэ тавд өгсөн нэрлэсэн
`SELECT` grant; `platform.operator_audit` зэрэг бусад хүснэгт хаалттай.
`TestTenantRoleReadsTheBoundaryButNotOperatorAudit` үүнийг бодит role-оор
шалгана.

Дараагийн migration санамсаргүйгээр platform schema-д шинэ хүснэгт үүсгэхэд
tenant role эрх өвлөхгүйг `TestNewPlatformTableIsClosedToTenantRole` түр хүснэгт
үүсгэн баталдаг. Иймээс өмнөх review-д санал болгосон regression test одоо
хэрэгжсэн.

## 3. Нэвтрэлт ба origin

| | Тенантын урсгал | Платформын урсгал |
| --- | --- | --- |
| Хост | `nexus.gerege.mn` | `cp.nexus.gerege.mn` |
| Login | `POST /api/v1/auth/login` болон identity provider-ууд | `POST /api/platform/v1/session` |
| Cookie | `session_token` | `cp_session` |
| Identity | `platform.users` + `tenant.memberships` | `platform.operator_accounts` |
| DB role | `gerege_nexus_tenant` | `gerege_nexus_operator` |
| Баталгаажуулалт | password/eID/ДАН/Google/SSO | password + TOTP заавал |

Хоёр cookie, хоёр account, хоёр origin санаатай тусдаа. Нэг хүн хоёуланд
зэрэг нэвтэрч болно. Оператор тенант руу audit-тай орох зөв зам нь 30 минутын
impersonation.

### Өмнөх review-оос хэрэгжсэн зүйл

- `CONTROL_PLANE_HOST=admin.localhost` development compose/example-д орсон;
  browser local дээр ч хоёр origin-оор явна.
- Control host-ын `/` нь 308-аар `/cp` руу орно.
- Control host дээр `/cp/*` ба `/api/platform/v1/*`-оос бусад зам `404`.
- Tenant host дээр `/cp/*` `404`.
- Эдгээрийг unit test, static boundary check, production-like host smoke test
  хамгаална.

Screenshot дээрх nginx `403 Forbidden` нь allowlist-аас гаднах client
аппликейшнд хүрээгүйг илэрхийлнэ. Энэ бол route bug биш, fail-closed network
gate. Операторын бодит `/32` эсвэл VPN CIDR-ийг GitHub-ийн
`CONTROL_PLANE_ALLOWED_CIDRS` secret-д нэмээд successful deploy ажиллуулах ёстой;
`0.0.0.0/0` ашиглаж тойрохыг renderer хориглоно.

## 4. Консолын одоогийн хамрах хүрээ

Platform API 44 canonical route-тэй. Frontend одоогоор дараах дэлгэцтэй:

| Бүлэг | Дэлгэц |
| --- | --- |
| Эрүүл мэнд | `/cp` ажиллагааны summary |
| Байгууллагууд | жагсаалт, дэлгэрэнгүй, lifecycle, apps, usage |
| Дэмжлэг/шийдвэр | people/support, impersonation, approvals |
| Тохиргоо | settings, flags, maintenance, announcements |
| Мөрдлөг | append-only audit хайлт, before/after snapshot |

Нүүр нь зориудаар investigation dashboard биш. Гүн шинжилгээг Grafana,
Alertmanager болон зориулалтын дэлгэц дээр хийнэ.

### API бий, тусдаа дэлгэцгүй үлдсэн зүйл

| API | Үлдсэн UI ажил |
| --- | --- |
| `GET /operators` | Operator account, role, төлөвийн дэлгэц |
| `GET /deletions` | Нийт pending deletion queue |
| `GET /catalog/overview`, `/catalog/status` | Client method ба нүүрийн summary бий; тусдаа каталогийн харагдацгүй |

## 5. Үлдсэн refactor/backlog

Эдгээрийг хэрэгжсэн мэт баримтжуулахгүй:

1. **Tenant composition root.** `internal/tenant/service.go` 1046 мөр хэвээр.
   Domain package-ууд бий болсон ч route/dependency assembly-г platform
   plane-ийн 266 мөрийн загварт ойртуулан үргэлжлүүлэн задлана.
2. **Operator UI.** `/operators` API-г audit-тай нэг мөрдлөгийн бүлэгт
   харагдуулна.
3. **Deletion queue UI.** 30 хоногийн эргэлт буцалтгүй ажлыг tenant detail-ээс
   гадна нийтээр нь харуулна.
4. **Boundary health.** Production DB дээр schema count, platform grants, RLS
   coverage, migration/ownership drift-ийг summary байдлаар ажиглах endpoint
   ба дөрвөн мөрийн блок одоогоор байхгүй.
5. **Session coexistence indicator.** Нэг browser-д tenant session давхар
   нээлттэй байгааг console header хараахан хэлэхгүй.
6. **Catalog detail UI.** Health summary-аас цааш status/compatibility-г
   дэлгэрүүлсэн тусдаа дэлгэцгүй.

Санал болгож буй дараалал: operator UI → deletion queue → boundary health →
session indicator → catalog detail → tenant composition root-ийн том задрал.
Backend-ийн том задралыг UI-ийн жижиг нөхөөсүүдтэй нэг PR-д холихгүй.

## 6. Regression хамгаалалт

| Эрсдэл | Test/contract |
| --- | --- |
| Plane import холилдох | `backend/internal/planes_test.go` |
| Хүснэгт буруу schema-д очих | `ownership_test.go`, `schema_split_test.go` |
| SQL unqualified болох | `qualification_test.go` |
| Grant өргөжих | `schema_split_test.go` default-deny ба boundary test |
| Route алга болох/нэмэгдэх | `backend/pkg/platform/testdata/routes.txt` golden |
| Control origin tenant UI үзүүлэх | frontend proxy unit/static/smoke tests |
| CIDR fail-open болох | `deploy/scripts/test_render_cp_allowlist.py` |

DB integration test нь `TEST_DATABASE_URL` шаардана. CI дээр migrated
PostgreSQL-тэй ажиллуулж байж schema/grant дүгнэлтийг баталгаажуулна; зөвхөн
Markdown дахь 27/40 тоонд найдахгүй.
