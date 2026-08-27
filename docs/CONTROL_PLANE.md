# Control plane — операторын консол

`cp.nexus.gerege.mn` дээрх операторын урсгалыг босгох, хамгаалах, ажиллуулах
одоогийн заавар. Шинэчлэгдсэн: 2026-08-24.

[Баримт бичгийн төв](README.md) · [Архитектур](ARCHITECTURE_SPECIFICATION.md) ·
[ADR-0005](adr/0005-two-planes-one-origin-each.md) ·
[Мониторинг](MONITORING.md) · [Runbook](RUNBOOKS.md)

---

## 1. Одоогийн contract

| Талбар | Утга |
| --- | --- |
| Origin | `https://cp.nexus.gerege.mn` |
| Frontend | `/` нь 308-аар `/cp` руу; UI нь `/cp/*` |
| Canonical API | `/api/platform/v1/*` — одоогоор 44 route |
| Legacy API | `/cp/api/*` нь HostGate-ийн ард 308; vNEXT-д устгана |
| Нэвтрэлт | Нууц үг + баталгаажсан TOTP |
| Cookie | `cp_session`, тенантын `session_token`-оос тусдаа |
| Session | Дээд тал нь 8 цаг, 30 минут idle, мэдрэмтгий үйлдэлд step-up |
| Account | `platform.operator_accounts`; `platform.users` биш |
| DB role | `gerege_nexus_operator` |
| Audit | `platform.operator_audit`, append-only; write бүртэй нэг transaction |

Нэг Go бинарь tenant ба platform plane-ийг зэрэг үйлчилдэг ч origin, identity,
cookie, DB role, audit нь тусдаа. Энэ хилийг нэгтгэж болохгүй. Оператор
тенантын нүдээр харахдаа хоёр дахь энгийн login биш, audit-тай impersonation
ашиглана.

## 2. Хамгаалалтын давхаргууд

Консол руу хүрэхийн тулд дараах бүх хаалгыг дарааллаар өнгөрнө:

1. **Хаягийн шалгалт — платформ ХААЛТТАЙ (private) горимд байхад.**
   `CONTROL_PLANE_ALLOWED_CIDRS`-д жагсаасан хаягнаас ирээгүй хүсэлт **404**
   авна (403 биш — доорх 2-ын шалтгаанаар).

   Платформ **нээлттэй (public)** горимд байхад энэ шалгалт огт хийгдэхгүй:
   хэн ч бүртгүүлж болдог суулгац операторуудаа хаанаас нэвтрэхийг нь
   зааж чадахгүй. Шийдвэрийг платформ өөрөө, хүсэлт бүр дээр гаргана
   (`backend/internal/operator/operator/address.go`) — тохиргоог нь харж
   чаддаггүй nginx биш.

   nginx-ийн `cp-allowlist.conf` нь хэвээр байгаа бөгөөд хүсвэл ирмэг дээр
   давхар хаалт болно. `CONTROL_PLANE_ALLOWED_CIDRS`-д `open` гэж бичвэл тэр
   хаалтыг өргөнө (`0.0.0.0/0` гэж бичих боломжгүй — буруу бичсэн prefix
   консолыг чимээгүй нээхээс сэргийлж deploy татгалздаг; `open` гэдэг үгийг
   санамсаргүй бичих боломжгүй).
2. **`CONTROL_PLANE_HOST`** — API ба frontend хоёулаа энэ нэрээр ирээгүй
   хүсэлтэд **404** хариулна (403 биш: 403 нь тэнд ямар нэг зүйл байгааг
   баталгаажуулна). Production дээр хоосон орхивол консол огт байхгүй.
3. **Нэвтрэлт** — нууц үг + TOTP. Хоёр дахь хүчин зүйлгүй бүртгэл нэвтэрч
   чадахгүй.
4. **Хязгаарлагдсан DB role.** Query бүр `dbguard.AsOperator` context-оор
   `gerege_nexus_operator` role-д орно; login role-оор чимээгүй үргэлжлэхгүй.
5. **Заавал audit.** `operator.RequireAudit` ба `operator.Do` нь write болон
   audit мөрийг хамт commit хийнэ.

Эдгээр нь бие биеэ орлохгүй. Private deployment-ийн CIDR буруу болсон ч
HostGate/session/role үлдэнэ; nginx allowlist идэвхтэй бол аппын алдааны урд
нэмэлт хаалт болно.

## 3. Production-д босгох

### 3.1 DNS, TLS, nginx

`cp.nexus.gerege.mn` DNS нь deployment server-ийг заана. Репогийн
`deploy/nginx/cp.nexus.gerege.mn.conf` болон
`deploy/nginx/snippets/cp-allowlist.conf`-ыг идэвхжүүлээд TLS сертификат
олгоно:

```bash
sudo cp deploy/nginx/cp.nexus.gerege.mn.conf /etc/nginx/sites-available/
sudo cp deploy/nginx/snippets/cp-allowlist.conf /etc/nginx/snippets/
sudo ln -s /etc/nginx/sites-available/cp.nexus.gerege.mn.conf \
  /etc/nginx/sites-enabled/cp.nexus.gerege.mn.conf
sudo nginx -t
sudo systemctl reload nginx
sudo certbot --nginx -d cp.nexus.gerege.mn
```

GitHub Actions production deploy дараах тохиргоог хэрэглэнэ:

| GitHub тохиргоо | Жишээ | Үүрэг |
| --- | --- | --- |
| Repository variable `CONTROL_PLANE_HOST` | `cp.nexus.gerege.mn` | Backend/frontend-ийн origin gate |
| Secret `CONTROL_PLANE_ALLOWED_CIDRS` | `203.0.113.10/32, 2001:db8:1::/64` | nginx allowlist үүсгэх |

Тогтмол office/VPN хаягийг аль болох CIDR-аар оруул. Түр зуурын нэг IPv4
хаяг бол `203.0.113.10/32` хэлбэртэй байна; хаяг солигдоход secret-ийг
шинэчилж deploy-ийг дахин ажиллуулна. Хаягаар хязгаарлах шаардлагагүй бол
`open` гэж бичнэ — тэр үед платформ хаалттай горимд ч хаяг шалгахгүй.

CIDR secret зай, шинэ мөр, таслалаар тусгаарласан утга авч болно. Renderer нь
invalid CIDR, host bit зөрсөн network, бүх интернетийг нээх `0.0.0.0/0` ба
`::/0`-ийг татгалзана. Secret хоосон бол сервер дээрх одоогийн allowlist-ыг
дарж бичихгүй. Тогтмол хаяг байхгүй бол CIDR-ийг улам өргөсгөхийн оронд VPN
эсвэл identity-aware proxy хэрэглэнэ.

### 3.2 Env ба контейнер

Production env-д дор хаяж:

```dotenv
CONTROL_PLANE_HOST=cp.nexus.gerege.mn
```

Энэ утга backend болон frontend хоёрт ижил очих ёстой. Compose startup goose
migration-уудыг ажиллуулна. `00049_control_plane.sql`-аас эхэлсэн control-plane
schema/role, `00079_two_schemas.sql`, `00080_search_path_has_no_public.sql`
хүртэл бүрэн орсны дараа API ажиллана.

Startup log-д operator role bind амжилттайг шалга. Role эсвэл grant дутуу үед
console query login role-оор үргэлжлэхгүй, хаалттай унах нь санаатай.

### 3.3 Анхны оператор

Вэб бүртгэл байхгүй. DB эрхтэй хүн контейнер дотор bootstrap command ажиллуулна:

```bash
docker exec -it gerege_nexus_backend /app/operator-bootstrap \
  -email you@gerege.mn -name "Таны нэр" -role superadmin
```

Command нууц үгийг TTY-ээс хоёр удаа асууж, TOTP secret ба `otpauth://` URI-г
нэг удаа харуулна. Authenticator-т нэмээд кодоор баталгаажуул. Баталгаажаагүй
эсвэл дундаа тасарсан account нэвтэрч чадахгүй. Нууц үгийг flag/env-д бүү
дамжуул: shell history, process list, container inspect-д үлдэнэ.

## 4. Хөгжүүлэлтийн хоёр origin

`.env.example` болон `docker-compose.yml` development-д
`CONTROL_PLANE_HOST=cp.localhost` ашиглана:

```text
Тенант:   http://nexus.localhost:3000
Консол:   http://cp.localhost:3000     → 308 → /cp
CP API:   http://cp.localhost:8080/api/platform/v1
```

Орчин үеийн browser `*.localhost`-ыг loopback руу шийддэг. Ингэснээр HostGate,
cookie separation, CSRF Origin шалгалт production-д анх удаа биш, local test
дээр ажиллана.

Frontend-ийн host contract:

```text
Host == CONTROL_PLANE_HOST
  /                       → 308 /cp
  /cp, /cp/*              → allow
  /api/platform/v1/*      → allow
  бусад зам               → 404

Host != CONTROL_PLANE_HOST
  /cp, /cp/*              → 404
```

## 5. Үүрэг ба эрх

| Үүрэг | Үндсэн боломж |
| --- | --- |
| `superadmin` | Бүх capability; устгал хүсэх/өөр хүний хүсэлтийг зөвшөөрөх, export, impersonation |
| `operator` | Тенант үүсгэх, lifecycle, quota, дэмжлэг; устгал ба impersonation үгүй |
| `support` | Унших, дэмжлэг, impersonation; platform administration үгүй |
| `auditor` | Зөвхөн унших |

Үүргүүд шаталсан hierarchy биш. Capability mapping нь
`backend/internal/operator/operator` package-д төвлөрнө; handler бүр role name
дахин тайлбарлах ёсгүй. Устгалын хүсэлтийг үүсгэсэн superadmin өөрөө
зөвшөөрөхгүй.

## 6. Үйл ажиллагааны contract

### Тенантын lifecycle

```text
active → suspended → active
active → pending deletion → cancelled
                          → deleted (approval + 30 хоногийн grace дараа job)
```

- Suspend нь тухайн тенантын session-уудыг transaction дотор revoke хийнэ.
- Deletion нь хоёр өөр superadmin шаардсан approval; grace хугацаанд цуцалж
  болно.
- Console role-д шууд bulk `DELETE` эрх байхгүй. Хугацаа дууссан цэвэрлэгээг
  background job platform замаар гүйцэтгэнэ.
- Export нь тусдаа capability, step-up, audit шаардсан JSON багц.

### Impersonation

Оператор шалтгаан бичиж TOTP step-up хийсний дараа 60 секунд хүчинтэй нэг
удаагийн hand-over token авна. Тенантын `/impersonate` хуудас түүнийг 30
минутын `session_token` болгоно. UI banner харуулж, operator ба tenant audit
хоёуланд actor/reason үлдэнэ. Түдгэлзсэн тенант руу нэвтрэхгүй.

### Settings, flags, maintenance

- Platform setting key кодын registry-д бүртгэлтэй байх ёстой; secret төрлийн
  setting байхгүй. Нууц env/GitHub secret-д үлдэнэ.
- Утгын precedence: DB → env → default. Өөрчлөлт ба rollback бүр түүхтэй.
- Feature flag нь release, kill switch, experiment төрөл, tenant override,
  тогтвортой percentage rollout, expiry дэмжинэ.
- `module.<app-id>.disabled` kill switch module route-д `503` өгнө.
- Maintenance mode уншихыг нээлттэй үлдээж write-д `503` + `Retry-After`
  өгнө; logout үргэлж нээлттэй.

### Metering, backup, catalog

Usage event-үүд tenant quota, хэрэглээний график, CSV export-ийг тэжээнэ.
Backup restore test болон deploy action нь operator capability, step-up,
audit-тай. Catalog API status/overview/sync өгч, running platform version-той
manifest compatibility-г шалгана.

## 7. Консолын дэлгэц ба API

Одоогийн frontend дэлгэцүүд:

- `/cp` — ажиллагааны товч хураангуй; investigation dashboard биш;
- `/cp/tenants` ба `/cp/tenants/[id]` — байгууллага, lifecycle, apps, usage;
- `/cp/support` — хүмүүс, session/credential тусламж, impersonation;
- `/cp/approvals` — хоёр хүний шийдвэр;
- `/cp/config` — settings, flags, maintenance;
- `/cp/announcements` — tenant banner;
- `/cp/audit` — append-only operator audit хайлт ба өөрчлөлтийн snapshot.

API-д байгаа боловч тусдаа UI дэлгэцгүй contract:

- `GET /operators` — operator account/role жагсаалт;
- `GET /deletions` — deletion queue;
- `GET /catalog/overview`, `GET /catalog/status` — client method бий, нүүрийн
  health/catalog summary-аас цааш тусдаа харагдацгүй.

Эдгээрийг “хийсэн UI” гэж runbook-д тэмдэглэхгүй. Boundary health block болон
хоёр session зэрэг нээлттэйг хэлэх indicator мөн хэрэгжээгүй backlog хэвээр.

## 8. Шалгах

Кодын бүрэн CI нь `.github/workflows/ci.yml`-д canonical. Орон нутагт
control-plane-ийн өөрчлөлтөд хамгийн багадаа:

```bash
cd backend
go test ./internal/operator/... ./internal/kernel/security/... ./pkg/host/...

cd ../frontend
npm run test
npx tsc --noEmit
npm run lint
npm run build
npm run host:smoke

cd ../deploy/scripts
python3 -m unittest test_render_cp_allowlist.py
```

`TEST_DATABASE_URL` бүхий migrated PostgreSQL дээр schema/grant/RLS
integration test-ийг давхар ажиллуул:

```bash
cd backend
TEST_DATABASE_URL="$TEST_DATABASE_URL" go test ./db/migrations/...
```

Production deploy нь successful `main` CI-ийн `workflow_run`-аас эхэлж,
health/ready, OIDC, control-plane root redirect, canonical API host boundary,
legacy redirect-ийг smoke test хийнэ. Allowlist-аас гаднах хаяг nginx-ээс
`403` авах нь зөв; allowlist доторх зөв хост auth-гүй API нь аппын түвшний
`401` авах ёстой.

## 9. Түүхэн баримт

[CONTROL_PLANE_PLAN.md](CONTROL_PLANE_PLAN.md) нь хэрэгжүүлэх үеийн ажлын
төлөвлөгөө бөгөөд хуучин `/cp/api` нэр, CP үе шатны тайлбар агуулж болно.
Одоогийн operational contract-д энэ файл, route golden file, migration test,
frontend host test-ийг эх сурвалж болгоно.
