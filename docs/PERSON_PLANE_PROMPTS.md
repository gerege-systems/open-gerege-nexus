# Иргэний урсгал — хэрэгжүүлэлтийн prompt-ууд

**Статус: САНАЛ. P0–P2 хэрэгжээгүй; урьдчилсан нөхцөл нь 2026-08-27-нд
хэрэгжсэн.** Эдгээр нь хийгдсэн ажлын түүх биш, хийх ажлын заавар. Шийдвэрийн
үндэслэл нь [`adr/0006-a-person-owns-a-space.md`](adr/0006-a-person-owns-a-space.md),
дизайн нь [`WORKSPACE_NAMING_PROPOSAL.md`](WORKSPACE_NAMING_PROPOSAL.md) §4.9.

Үе шат бүр **нэг session, нэг branch, нэг PR**. Дараалал заавал: P1 нь P0-гийн
модноос, P2 нь P1-ийн role-оос, P3 нь P2-ийн query-гээс хамаарна.

---

## Гурван засвар — 2026-08-27

Эдгээр prompt-ыг кодтой тулгаж хэмжихэд гурван зүйл буруу байсан. Prompt-уудыг
доор нь засав; юу яагаад өөрчлөгдсөнийг энд нэг дор бичив, учир нь эхнийх нь
Үе P2-ыг бүхэлд нь өөр репо руу зөөж байна.

### 1. `urtuu_tasks` энэ репод байхгүй — P2 энд хийгдэхгүй

Үе P2 нь `workspace.urtuu_tasks`-д багана, бодлого, баганын grant нэмэхийг
`backend/db/migrations/`-д хийхээр бичсэн. Тэр хүснэгтийг **цөм эзэмшихээ
больсон**: миграц `00078_urtuu_takes_its_tasks.sql` нь `urtuu_tasks`,
`urtuu_task_events`, `urtuu_numbers` гурвыг устгаж, апп нь схемээ өөрөө үүрч
`client-gerege-nexus/modules/urtuu/migrations/00001_urtuu.sql` руу явсан.

Цөмд тэр багана нэмэх миграц бичвэл хоёр хамгаалалт зогсооно —
`TestPlatformMigrationsOwnNoAppTable` (цөмийн миграц аппын хүснэгт үүсгэхгүй)
ба `TestPlatformSQLNamesNoAppTable` (цөмийн SQL аппын хүснэгтийг нэрлэхгүй).

Тиймээс **P2 бол модулийн репогийн ажил**. Цөмд үлдэх зүйл нь тэр query
ажиллах нөхцөл: DB role, `dbguard`-ийн салаа, SDK-гийн гадаргуу — өөрөөр
хэлбэл P1. Гэвч тэр нь дараах нээлттэй асуултыг төрүүлж байна, P1 эхлэхээс
өмнө хариулах ёстой:

> **Модуль хэрхэн `gerege_nexus_person` role дээр ажиллах вэ?** Модуль
> `nexus.Module` болж бүртгэгдэж, мужийн gate-ийн ард route mount хийдэг —
> өөрөөр хэлбэл `gerege_nexus_tenant` дээр, нэг мужид хүлэгдсэн. Муж дамнасан
> уншилтыг `pkg/nexus` өнөөдөр санал болгодоггүй. Хариулт нь SDK-д шинэ
> гадаргуу нэмэх (жишээ нь `nexus.PersonScope`) байж магадгүй — тэр нь энэ
> цувралын «SDK өөрчлөх шаардлага байх ёсгүй» гэсэн дүрмийг зөрчинө, тиймээс
> зогсоод шийдэх ёстой зүйл мөн.

### 2. P3-ийн урьдчилсан нөхцөл байхгүй байсан — одоо байна

P3 нь «нэвтэрсэн, байгууллагагүй хүн» гэсэн төлөв дээр тулгуурласан. Тэр төлөв
**оршин байгаагүй**: `FirstTenantFor` нь гишүүнчлэл олохгүй бол
`ErrNoOrganisation` буцааж session нээгддэггүй байв. §5-ийн Үе C0 үүнийг
арилгах ёстой байсан ч P-цувралын хүснэгтэд ороогүй тул P3 эхлэхээсээ өмнө
хаалттай байлаа.

Миграц `00085_personal_workspace.sql` үүнийг арилгав — гэхдээ C0-гийн зам биш,
C1-ийн: гишүүнчлэлгүй хүн **өөрийн гэрт** нэвтэрнэ («хувийн муж нь муж мөр»).
`/api/v1/me` одоо `workspace_kind` буцаана, тиймээс P3-ийн «Урьдчилсан нөхцөл»
хэсгийн эхний салаа идэвхтэй: бүрхүүлийг `workspace_kind === 'personal'`-оор
шийднэ, замаар таамаглахгүй.

### 3. «Session-д муж байхгүй» нь аюултай төлөв — тиймээс сонгоогүй

Санал нь C0-г («tenant-гүй session») хямд алхам гэж дүрсэлсэн. Тэр нь хямд
биш: `dbguard`-ийн `PrepareConn` доторх switch-ийн `default` нь `"none"` буюу
**login role** — бүх бодлогын гадна, юу ч бичиж чадна. Нэвтэрсэн иргэний
хүсэлт тэр салаанд унах нь эрхийн өсөлт байсан.

Гэр бол муж мөр учир энэ асуудал огт үүсэхгүй: session-д муж байна,
`app.current_tenant` тавигдана, RLS хэвээр ажиллана. Тиймээс 00085-д шинэ
role, шинэ бодлого, `dbguard`-ийн өөрчлөлт, `sessions.tenant_id`-г NULL
болгох — эдгээрийн **нэг нь ч** байхгүй. Хоёр багана: `registry.tenants.kind`
ба `owner_user_id`.

Prompt бүрийг `### PROMPT` мөрнөөс доош бүтнээр нь хуулж өгнө.

| Үе | Юу | Репо | Өгөгдөл хөндөх үү | Эрсдэл |
| --- | --- | --- | --- | --- |
| **C1-lite** | Гэр: `tenants.kind`, `owner_user_id`, залхуу үүсгэлт | цөм | ✅ | Бага — **✅ хэрэгжсэн 00085** |
| P0 | `internal/person` араг яс, `Sessions` порт, нэг хоосон route | цөм | ❌ | Байхгүй |
| P1 | `gerege_nexus_person` role, `dbguard.AsPerson`, SDK-гийн гадаргуу | цөм | ❌ | Бага — §1-ийн нээлттэй асуулт нээлттэй |
| P2 | `applicant_user_id`, policy, эхний нэрлэсэн statement | **`client-gerege-nexus/modules/urtuu`** | ✅ | **Өндөр** |
| P3 | `/me` дэлгэц, өөрийн layout | цөм (frontend) | ❌ | Бага |

**Дарааллын утга:** P0 ба P1 нь нэг ч мөр өгөгдөл уншихгүй. Хоосон plane ба
түүний DB role-ыг **өгөгдөл урсахаас өмнө** барих нь энэ дарааллын бүх учир.
P2 бол цорын ганц эмзэг үе; P0/P1-ийг алгасаад P2 хийх нь тэр эмзэг байдлыг
хилгүй, role-гүй хийнэ гэсэн үг.

**P3 нь энэ гинжинд байхгүй.** Тэр нь C1-lite дээр тогтдог болохоос P2 дээр
биш: гэртээ буусан иргэнд харуулах хоосон дэлгэц бол бүрэн дэлгэц, дараа нь
P2 түүнийг дүүргэнэ. Одоо хийж болно.

---

## Бүх prompt-д хамаарах дүрэм

Prompt бүрд давтагдаж орсон, энд нэг удаа:

* **Репогийн хэв маяг.** Go 1.26, `pgx` + гар бичсэн SQL, goose миграц, RLS-д
  найдсан тусгаарлалт, `slog`. Frontend: Next.js 16 App Router, TS strict,
  Tailwind.
* **Шалгалт:** `cd backend && gofmt -l . && go vet ./... && go test -race ./... &&
  golangci-lint run`; `cd frontend && npx tsc --noEmit && npm run build`.
* **Commit нь монголоор, өнгөрсөн цагаар** («Иргэний урсгал өөрийн модтой болов»).
* **Тайлбар нь юу хийж байгааг биш, ЯАГААД гэдгийг хэлнэ.** Өнгө аясыг
  `backend/internal/kernel/dbguard/dbguard.go`,
  `backend/db/migrations/ownership_test.go`,
  `backend/internal/planes_test.go`-ийн толгойноос ав — эхлээд уншиж жишээ ав.
* **`pkg/nexus`-ийн API өөрчлөгдвөл** `api.txt`-ийн diff-ийг PR-ийн тайлбарт
  үгээр тайлбарла. Энэ ажилд SDK өөрчлөх шаардлага **байх ёсгүй**; өөрчлөх
  шаардлагатай болвол зогсоод асуу — тэр нь дизайн буруу байсны шинж.
* **Байгаа зан төлөвийг өөрчлөхгүй.** Алдаа олдвол засахгүй — PR-ийн тайлбарт
  бичиж, тусад нь issue болго.
* **Хоёр зүйлийг хэзээ ч бүү хий:** `internal/person`-оос `internal/workspace`
  эсвэл `internal/operator` руу импорт; person-ийн хүсэлтийг `login` role дээр
  ажиллуулах.
* **Нэрийн тэмдэглэл.** Эдгээр prompt дэх Go нэрс Үе F (`tenantID` →
  `workspaceID`)-ийн **дараах** байдлаар бичигдсэн: `nexus.WithWorkspaceID`,
  `nexus.AllowedWorkspaces`, `UserClaims.WorkspaceID`. SQL дэх GUC нэр
  (`app.current_tenant`, `app.allowed_tenants`) болон DB role-ийн нэр
  (`gerege_nexus_tenant`) өөрчлөгдөөгүй — тэднийг бүү сольж бич.

---

## Үе P0 — Хоосон урсгал

Нэг ч query байхгүй. Мод, порт, нэг route.

### PROMPT (эндээс доошхыг хуулна)

Чи Gerege Nexus репод ажиллаж байна. `git checkout -b person/p0-skeleton`.

**Заавал эхлээд унш** (эдгээрийг уншилгүйгээр бүү эхэл):

- `docs/WORKSPACE_NAMING_PROPOSAL.md` §4.9 — энэ ажлын эх сурвалж;
- `docs/adr/0006-a-person-owns-a-space.md` «Гурав дахь plane бий» хэсэг;
- `backend/internal/planes_test.go` — толгойн тайлбар ба `planes` хувьсагч.
  Дүрэм аль хэдийн бичигдсэн, `internal/person` байхгүй тул одоо ногоон байгаа;
- `backend/internal/workspace/service.go` — plane-ийн `Service`/`Deps`/`Routes`
  хэлбэр. Чи үүнийг **хуулбарлахгүй**, хэлбэрийг нь дагана;
- `backend/pkg/host/server.go`, ялангуяа `newRouter` — plane-ууд хаана
  угсрагддаг;
- `backend/internal/workspace/auth/auth.go` (`type UserClaims = nexus.UserClaims`)
  ба `session.go`-гийн `TokenFromRequest`, `SessionStore.Resolve`.

**Хийх ажил.**

1. `backend/internal/person/sessions.go` — порт:

   ```go
   // Sessions answers one question: who is making this request.
   //
   // An interface rather than an import, and the reason is the whole shape of
   // this plane: workspace/auth already resolves sessions, so the import writes
   // itself — and arrives carrying every query in that package, each one
   // written for somebody acting inside an organisation, now reachable from a
   // request that belongs to somebody who is in none of them.
   type Sessions interface {
       Resolve(ctx context.Context, token string) (nexus.UserClaims, error)
   }
   ```

   Тайлбарыг өөрөө бич, дээрхийг үг үсгээр нь бүү хуул — гэхдээ **яагаад порт
   болохыг** заавал бич.

   **Хэлбэрийн тэмдэглэл.** Энэ баримтын анхны хувилбар
   `Person(r *http.Request)` гэж бичсэн бөгөөд §4.9 нь `Resolve(ctx, token)`
   гэж бичсэн — хоёр өөр хил. `Resolve` нь зөв: `workspace/auth.SessionStore`
   түүнийг аль хэдийн яг ийм гарын үсгээр хангадаг тул адаптер үнэхээр гурван
   мөр болно, мөн порт нь HTTP-ийн тухай юу ч мэдэхгүй байх нь ADR 0001-ийн
   адаптер/домэйн дүрэмтэй нийцнэ. Token-ыг задлах нь middleware-ийн ажил
   (`auth.TokenFromRequest`), портынх биш.

2. `backend/internal/person/service.go` — `Deps{DB *pgxpool.Pool, Sessions Sessions}`,
   `New(Deps) (*Service, error)`, `Routes(r chi.Router)`. `DB`-г одоо
   ашиглахгүй ч талбар нь байг: P2-т орно.

3. `backend/internal/person/middleware.go` — `auth.TokenFromRequest`-ийн
   хэлбэрээр token-ыг задалж `Sessions.Resolve(ctx, token)` дуудна,
   алдвал `401`, амжвал `nexus.WithUser(ctx, claims)`.
   **Муж огт тавихгүй** — `nexus.WithWorkspaceID` энэ файлд гарч болохгүй.

4. Нэг route: `GET /api/v1/me/requests` → `[]` (хоосон массив), middleware-ийн
   ард. Өгөгдлийн сан хөндөхгүй.

5. `backend/pkg/host/server.go` — person plane-ыг угсарч `Routes(r)`-ыг
   mount хий. Портын адаптерыг **энд** бич (жижиг struct, `auth.TokenFromRequest`
   + `sessions.Resolve` хоёрыг холбоно): энэ файл бол хоёроос олон plane нэрлэх
   эрхтэй цорын ганц газар, өөрийн тайлбар нь ч тэгж хэлсэн.

6. `backend/internal/person/service_test.go` — нэвтрээгүй хүсэлт `401` авдаг;
   fake `Sessions`-той нэвтэрсэн хүсэлт `200` ба `[]` авдаг. Fake нь гурван мөр
   байх ёстой — байхгүй бол порт хэтэрхий том.

7. Golden route table-ыг репогийн журмаар шинэчил
   (`backend/pkg/host/routes_golden_test.go`-г эхлээд уншиж яаж шинэчлэхийг ол).

**Дуусахад биелэх ёстой.**

- `go test ./internal/` ногоон, `TestPersonDoesNotImportAnotherPlane` ба
  `TestNoPlaneImportsPerson` одоо **бодитоор хэмжиж** эхэлсэн (өмнө нь
  «does not exist yet» гэж логлодог байсан);
- `pkg/host/testdata/routes.txt` дээр яг шинэ route-ууд л нэмэгдсэн;
- `internal/person` доторх нэг ч файл `internal/workspace` эсвэл
  `internal/operator` импортлоогүй;
- `internal/person` дотор `SELECT`, `INSERT` гэсэн үг **байхгүй**.

**Бүү хий.** Миграц бичих; `dbguard` хөндөх; frontend хөндөх; `/me` дэлгэц хийх.

---

## Үе P1 — Урсгалын өөрийн role

Query байхгүй хэвээр. Зөвхөн холболтын binding ба хаагдах тал руугаа унах дүрэм.

> **Эхлэхээс өмнө:** §1-ийн нээлттэй асуултыг хариул. Энэ role-ыг ашиглах query
> нь өөр репогийн модульд байх тул role үүсгэсэн ч түүнд хүрэх зам байхгүй бол
> хийсэн ажил ажиллахгүй тавиур дээр үлдэнэ. Хариулт нь `pkg/nexus`-д гадаргуу
> нэмэх бол энэ цувралын дүрмийг зөрчиж байгаа тул тусад нь шийдвэр.

### PROMPT (эндээс доошхыг хуулна)

Чи Gerege Nexus репод ажиллаж байна. `git checkout -b person/p1-role`.

**Заавал эхлээд унш:**

- `backend/internal/kernel/dbguard/dbguard.go` — **бүтнээр нь**, ялангуяа
  package-ийн толгой, `TenantRole`, `OperatorRole`, `AsOperator`,
  `bindStatement`, `Guard.Install`/`Probe`/`Enable`;
- `backend/db/migrations/00049_control_plane.sql` — операторын role яаж
  үүсдэг, юуг grant авдаг. Чи үүнийг дуурайна;
- `docs/WORKSPACE_NAMING_PROPOSAL.md` §4.9 «Аюулгүй байдлын ганц гол дүрэм».

**Хийх ажил.**

1. Шинэ миграц `backend/db/migrations/00085_person_role.sql` (дараагийн сул
   дугаарыг өөрөө шалга):
   - `CREATE ROLE gerege_nexus_person NOLOGIN NOSUPERUSER NOCREATEDB
     NOCREATEROLE NOINHERIT NOBYPASSRLS` — 00049-ийн мөртэй яг ижил шинжүүд,
     ялангуяа `NOBYPASSRLS`;
   - `GRANT USAGE ON SCHEMA workspace, registry` — **хүснэгтийн grant өгөхгүй**;
   - `Down` нь role-ыг буцаана.

   Тайлбарт: энэ role одоо **юу ч уншиж чадахгүй**, тэр нь зориуд. Хүснэгт
   бүрийн grant P2-оос эхлээд нэг нэгээр нэмэгдэнэ.

2. `dbguard`:
   - `PersonRole = "gerege_nexus_person"` const, яагаад login role биш
     болохыг `OperatorRole`-ийн тайлбарын хэлбэрээр бич;
   - `AsPerson(ctx, userID)` + `IsPerson(ctx)`, `AsOperator`-ийн хэлбэрээр;
   - `bindStatement`-д дөрөв дэх `set_config('app.current_person', $4, false)`;
   - `Guard.Install`-ийн `PrepareConn` доторх switch-д **гурав дахь case**:
     `case IsPerson(ctx):`. Түүнийг `case IsOperator(ctx):`-ийн яг хажууд,
     мужийн case-ээс **өмнө** тавь. `personReady atomic.Bool` нь
     `operatorReady`-гийн яг тэр шалтгаанаар: 00085 хүрээгүй суулгац хэвийн
     ажиллаж, иргэний урсгал нь тэнд байхгүй байна.

   Энэ switch-ийн `default` нь `"none"` буюу **login role** гэдгийг санаж бай:
   case нэмэхгүй бол иргэний хүсэлт чимээгүйхэн бүх policy-гийн гадна гарна.
   Операторын case дээрх татгалзлын тайлбарыг уншаад ижил өнгө аясаар бич.

3. **Fail-closed.** Person plane-ийн route нь `AsPerson`-гүйгээр өгөгдлийн санд
   хүрч чадахгүй байх ёстой. `internal/person`-д DB handle-ыг ороосон жижиг
   давхарга хий: `IsPerson(ctx)` худал бол query явуулахгүй, алдаа буцаана.

   Шалтгааныг тайлбарт бич, `dbguard`-ийн өөрийнх нь өгүүлбэрийг иш тат:
   tenant байхгүй context нь `login` role руу унадаг бөгөөд тэр нь бүх
   policy-гийн гадна, юу ч бичиж чадна.

4. P0-гийн middleware-т `AsPerson(ctx, claims.UserID)` нэмэгдэнэ.

**Дуусахад биелэх ёстой.**

- `AsPerson`-гүй context-оор person plane-ийн query оролдоход **алдаа** гарна,
  `login` role дээр ажиллахгүй — үүнийг барих тест байна;
- `DATABASE_URL`-тэй үед `gerege_nexus_person` нь `workspace.urtuu_tasks`-аас
  `SELECT` хийхийг оролдоод **permission denied** авна (grant хараахан алга) —
  энэ тестийг бич, энэ нь P2-ын хамгаалалтын суурь;
- 00085-ийн up/down хоёулаа ажиллана;
- Байгаа tenant/operator зан төлөв **өөрчлөгдөөгүй**: `go test -race ./...`.

**Бүү хий.** Хүснэгтэд `GRANT` өгөх; policy бичих; `applicant_user_id` нэмэх.

---

## Үе P2 — Эхний бодит уншилт

Энэ бол цорын ганц эмзэг үе. Бусад бүх үе үүнийг аюулгүй болгохын тулд байсан.

> **⚠ Энэ үе энэ репод хийгдэхгүй.** `urtuu_tasks` нь 00078-аас хойш цөмийнх
> биш — Өртөөгийн апп схемээ өөрөө үүрч
> `client-gerege-nexus/modules/urtuu/migrations/00001_urtuu.sql` руу явсан.
> Миграц, policy, багана бүгд тэр репод, `nexus.Migrations`-аар. Доорх prompt
> түүнд хүчинтэй хэвээр — зөвхөн салбар нь өөр репод үүснэ, мөн уншиж эхлэх
> файлуудын замууд `client-gerege-nexus`-ийн харьцангуй байдлаар өөрчлөгдөнө.
> §1-ийг эхлээд унш.

### PROMPT (эндээс доошхыг хуулна)

Чи Gerege Nexus репод ажиллаж байна. `git checkout -b person/p2-own-requests`.

**Заавал эхлээд унш:**

- `backend/db/migrations/00065_urtuu_two_lines.sql` — `line='service'`,
  `applicant` JSONB, `urtuu_tasks_service_has_applicant`,
  `idx_urtuu_tasks_applicant`. Тайлбарыг бүтнээр нь унш;
- `backend/db/migrations/00073_urtuu_reads_across_organisations.sql` — policy-ийн
  хэлбэр ба `TO gerege_nexus_app`;
- `backend/db/migrations/policy_shape_test.go` ба `ownership_test.go`;
- `backend/internal/operator/tenants/tenants.go`-ийн толгой — «a handful of
  statements, each written for one screen, each selecting named columns». Чи
  яг тэр хэв маягаар бичнэ;
- `docs/WORKSPACE_NAMING_PROPOSAL.md` §3.5 ба §4.9.

**Хийх ажил.**

1. Миграц: `workspace.urtuu_tasks`-д `applicant_user_id UUID` багана
   (`REFERENCES registry.users(id) ON DELETE SET NULL`), NULL зөвшөөрнө.

   Тайлбарт заавал: `applicant` JSONB **хэвээр үлдэнэ**. Тэр бол нийлүүлэгчийн
   ажилладаг агшны хуулбар; нийлүүлэгч иргэний бүртгэл рүү query явуулах ёсгүй.
   Шинэ багана нь «миний хүсэлтүүд»-ийг мөр таарах биш query болгох цорын ганц
   зорилготой. Байгаа мөрүүд NULL хэвээр — тэдгээр нь хэнд ч харагдахгүй, тэр
   нь зөв анхдагч.

2. Хэсэгчилсэн индекс: `WHERE applicant_user_id IS NOT NULL`.

3. Policy:

   ```sql
   CREATE POLICY person_own_rows ON workspace.urtuu_tasks
       TO gerege_nexus_person
       USING (applicant_user_id = NULLIF(current_setting('app.current_person', true), '')::uuid);
   ```

   `TO <role>` тул байгаа `tenant_isolation`-д хүрэхгүй. `NULLIF(..., true)`
   хэлбэрийг 00073-аас ав — тохируулаагүй GUC нь алдаа биш, хоосон байх ёстой.

4. **Баганын түвшний grant** — энэ бол «нэрлэсэн багана» дүрмийг DB дээр
   суулгаж байгаа хэрэг, кодын сахилга биш:

   ```sql
   GRANT SELECT (id, code, line, status, created_at, updated_at, answer)
       ON workspace.urtuu_tasks TO gerege_nexus_person;
   ```

   Яг аль багана болохыг өөрөө шийд, гэхдээ нийлүүлэгчийн дотоод талбар
   (хариуцагч, дотоод тэмдэглэл) орж болохгүй. Шийдвэрээ тайлбарт бич.

5. `urtuu_task_events`-д **юу ч өгөхгүй.** Тэр хүснэгтэд `applicant_user_id`
   байхгүй; join-тэй policy бичих нь үнэтэй бөгөөд иргэнд хэрэггүй. Иргэн
   «хаана явна, хэзээ дуусах вэ» гэдгийг мэдэх ёстой, «хэн дээр хэвтэж байна»
   гэдгийг биш. Энэ шийдвэрийг миграцын тайлбарт бич.

6. `internal/person/requests.go` — **нэг** statement, нэрлэсэн баганатай,
   `applicant_user_id = $1`. P0-гийн хоосон route үүгээр дүүрнэ.

7. `ownership_test.go`, `policy_shape_test.go`-г шинэ policy ба grant-аар
   шинэчил.

**Дуусахад биелэх ёстой** (DB integration тест, `DATABASE_URL`-тэй):

- Хоёр иргэн, хоёр нийлүүлэгч. Иргэн A нь зөвхөн өөрийн мөрөө хардаг,
  нийлүүлэгч хэд ч байсан хамаагүй;
- Иргэн A нь нийлүүлэгчийн бусад мөрийг **уншиж чадахгүй** — энэ үеийн гол
  тест, бусад нь хоёрдогч;
- Grant аваагүй багана руу хандахад алдаа гарна;
- `app.current_person` тохируулаагүй үед мөр **буцахгүй** (хоосон, алдаа биш);
- Байгаа tenant-ын зан төлөв өөрчлөгдөөгүй: `urtuu_tasks` дээрх
  `tenant_isolation` тестүүд хэвээр ногоон.

**Бүү хий.** `allowed_tenants`-ыг өргөтгөх — энэ бол хамгийн том занга, §3.5-д
яагаад болохгүйг бичсэн. Frontend хөндөх.

---

## Үе P3 — `/me` дэлгэц

### PROMPT (эндээс доошхыг хуулна)

Чи Gerege Nexus репод ажиллаж байна. `git checkout -b person/p3-me`.

**Заавал эхлээд унш:**

- `docs/WORKSPACE_NAMING_PROPOSAL.md` §4 бүтнээр нь;
- `frontend/app/cp/layout.tsx` — өөр үзэгчид өөр бүрхүүл гэдгийн жишиг;
- `frontend/proxy.ts` — `CONTROL_PLANE_HOST` дээрх `/` → 308 `/cp`. Чи яг тэр
  хэв маягийг дагана;
- `frontend/components/Layout.tsx`-ийн `searchIndex` — яагаад иргэн энэ
  бүрхүүлд орж болохгүйг ойлгохын тулд;
- `frontend/app/profile/page.tsx`-ийн толгойн тайлбар.

**Урьдчилсан нөхцөл — хангагдсан.** Миграц `00085_personal_workspace.sql`
`registry.tenants.kind`-ыг нэмж, `/api/v1/me` одоо `workspace_kind`
(`"organisation"` эсвэл `"personal"`) буцаана. Бүрхүүлийг **түүгээр** шийд.
Замаар, эсвэл «идэвхтэй байгууллага байхгүй» гэдгээр таамаглаж болохгүй —
хоёр дахь нь одоо худал: гишүүнчлэлгүй хүнд ч идэвхтэй муж байдаг, тэр нь
түүний гэр.

**Хийх ажил.**

1. `frontend/app/me/layout.tsx` — өөрийн бүрхүүл. `components/Layout.tsx`-ыг
   дахин ашиглахгүй: түүний rail нь админд зориулагдсан, иргэнд бүх нүд нь
   хоосон харагдана.
2. `frontend/app/me/page.tsx` — «Миний хүсэлтүүд» (P2-ын endpoint),
   `/profile` руу холбоос, `/settings/devices` руу холбоос.
3. `frontend/proxy.ts` — нэвтэрсэн хүний идэвхтэй муж нь гэр бол `/` дээр
   ирэхэд 308 `/me`. Шалгуур нь `workspace_kind === "personal"`, «муж
   байхгүй» биш.
   `/`-ыг нэвтрэлттэй болгохгүй: нийтийн landing нь server дээр рендерлэгддэг
   бөгөөд `app/page.tsx` шалтгааныг нь өөрөө бичсэн.
4. Сэлгүүрт «Миний гэр» мөр (`components/TenantChoices.tsx`).

**Дуусахад биелэх ёстой.**

- `npx tsc --noEmit`, `npm run build`, `npm test` ногоон;
- `frontend/tests/`-д host/path шийдвэрийн тест —
  `control-plane-host.test.mjs`-ийн хэлбэрээр;
- Гишүүнчлэлтэй хүн `/apps` дээрээ буусан хэвээр — энэ тест заавал. Түүнд
  гэр байсан ч гэсэн: `FirstTenantFor` байгууллагыг түрүүлж сонгодог
  (`internal/workspace/auth/tenants.go`), тиймээс энэ тест тэр эрэмбийг ч
  барина.

**Бүү хий.** `/apps`-ыг иргэнд харуулах; `/me`-г `cp` origin дээр тавих;
`components/Layout.tsx`-д иргэний нөхцөл нэмэх.

---

## Хийхээс өмнө шийдэх нээлттэй асуулт

**Audit хэнийх вэ.** Иргэн нийлүүлэгчийн мужид мөр үүсгэхэд тэр байгууллагын
audit бичих ёстой — өөрөөр хэлбэл person plane нь байгууллагын audit хүснэгтэд
бичнэ. Энэ бол хил дамнасан бичилт бөгөөд P2 нь **уншилт** тул тэр асуултыг
хойшлуулж болно. Иргэн хүсэлт **үүсгэдэг** болох өдөр (P4) энэ хариултгүйгээр
эхэлж болохгүй.

**Модуль муж дамнан яаж уншдаг вэ.** §1-д бичсэн. Энэ нь audit-ийн асуултаас
өмнө хариулагдана: түүнгүйгээр P2 бичигдэх газаргүй.

**Гэрийн metering.** 00085 нь гишүүнчлэлгүй хүн бүрд муж үүсгэдэг болсон тул
`registry.tenants`-ийн мөрийн тоо хүн амаар өсөж болно. Хэрэглээний тооллого
(`registry.usage_events`, квот) гэрийг байгууллагаас тусад нь тоолох уу гэдэг
нь одоохондоо хариугүй — саналын Үе C1-ийн 6 дахь зүйл. Консолын жагсаалт нь
аль хэдийн `kind='organisation'`-оор шүүгддэг тул дэлгэц зөв, харин **тоо
хэмжигдэхгүй** байгаа нь энэ.
