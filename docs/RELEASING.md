# Хувилбар гаргах журам

Gerege Nexus-ийн цөм нь өөр repo-нуудын **dependency** болох тул хувилбар нь
дотоод тэмдэглэл биш, гэрээ юм. Энэ баримт нь тэр гэрээ юу гэсэн үг, түүнийг
хэн, хэрхэн гаргахыг бичнэ.

[Баримт бичгийн төв рүү буцах](README.md) ·
Холбоотой: [Экосистемийн git стратеги](ECOSYSTEM_GIT_STRATEGY.md) ·
[Модуль бичих гарын авлага](MODULE_AUTHORING_GUIDE.md)

---

## 1. Амлалт

**`backend/pkg/nexus`-ийн API нь major хувилбар дотроо эвдэрдэггүй.**

Энэ бол бүх зүйлийн үндэс: distribution repo нь
`require github.com/gerege-systems/open-gerege-nexus/backend vX.Y.Z` гэж бичээд
өөрийн модулиуд нь X өөрчлөгдөх хүртэл компиллогдоно гэдгийг найдна. Зуун
distribution байх үед энэ найдвар л тэднийг fork хийхээс сэргийлж байгаа зүйл.

Амлалт нь `pkg/nexus`-д хамаарна. `internal/` доторх бүхэн нь хэзээ ч,
ямар ч хувилбарт өөрчлөгдөж болно — Go өөрөө тэднийг гаднаас импортлохыг
хориглодог тул тэр нь хэний ч найдварыг эвдэхгүй.

### Юу нь эвдэх өөрчлөлт вэ

- Экспортолсон төрөл, функц, метод, талбар **хасах** эсвэл нэрийг нь солих.
- Функцийн **гарын үсэг** өөрчлөх (параметрийн төрөл, тоо, буцах утга).
- Экспортолсон **interface-д метод нэмэх** — гаднаас түүнийг хэрэгжүүлж
  байсан бүх төрөл компиллогдохоо болино. Interface-д метод нэмэх нь
  struct-д талбар нэмэхтэй адилгүй гэдгийг санах хэрэгтэй.
- Одоо байгаа зан үйлийг чимээгүйхэн өөрчлөх (тэр нь компиляц эвдэхгүй ч
  илүү аюултай).

### Юу нь эвдэхгүй вэ

- Шинэ экспортолсон функц, төрөл нэмэх.
- Экспортолсон **struct-д талбар нэмэх** (заавал нэрээр нь эхлүүлдэг байх
  тул `nexus.MenuDefinition{...}` гэсэн эерэг литерал эвдрэхгүй).
- `internal/` доторх юу ч.
- Тайлбар, баримт бичиг.

### Эвдэх шаардлагатай бол

### Хийгдсэн эвдрэл — дараагийн tag нь **v2.0.0**

2026-08-27-нд `pkg/nexus`-ийн `tenant` гэдэг үг бүхэлдээ `workspace` болов.
`api.txt`-ийн 554 мөрөөс **65 нь дахин бичигдсэн**, нэг ч тэмдэглэгээ
нэмэгдээгүй, хасагдаагүй. Шалтгаан нь ADR 0006: `tenant` бол байршлын үг
бөгөөд хувийн муж («гэр») орж ирэх мөчид эцэслэн худал болно.

| Хуучин | Шинэ |
| --- | --- |
| `nexus.TenantID(ctx)` | `nexus.WorkspaceID(ctx)` |
| `nexus.TenantOf(ctx)` | `nexus.WorkspaceOf(ctx)` |
| `nexus.RequireTenant` | `nexus.RequireWorkspace` |
| `nexus.WithTenantID` | `nexus.WithWorkspaceID` |
| `nexus.WithoutTenant` | `nexus.WithoutWorkspace` |
| `nexus.AllowedTenants` / `WithAllowedTenants` | `nexus.AllowedWorkspaces` / `WithAllowedWorkspaces` |
| `nexus.ErrTenantMissing` | `nexus.ErrWorkspaceMissing` |
| `UserClaims.TenantID` / `.AllowedTenantIDs` | `.WorkspaceID` / `.AllowedWorkspaceIDs` |
| `DirectoryPerson.TenantID` / `.TenantName` | `.WorkspaceID` / `.WorkspaceName` |
| `SSOClient.TenantID`, `LinkMessage.TenantID` | `.WorkspaceID` |
| `ReportGrant.GrantorTenantID` / `.GranteeTenantID` | `.GrantorWorkspaceID` / `.GranteeWorkspaceID` |

**Deprecated alias үлдээгээгүй.** Доорх дүрэм нэг major цикл хүлээхийг
шаарддаг ч энд хүлээх зүйл байхгүй: `Use*` функцүүд нь **нэмэлт** зам байсан
бөгөөд хуучин нь ажилласаар байж чадна, харин нэр солих нь нэг л нэрийг хоёр
удаа зарлахыг шаардана. Тиймээс энэ нь v2.0.0-ийн ажил бөгөөд дараагийн tag
major байна. Дээрх deprecation жагсаалт мөн тэр өдрийн ажил — хоёулаа нэг
tag дээр гарна.

**Wire format өөрчлөгдөөгүй.** JSON тэмдэглэгээ `tenant_id`, `tenant_name`,
`allowed_tenant_ids`, `grantor_tenant_id`, `grantee_tenant_id` хэвээр. Go
дахь нэр домэйний үг, HTTP дахь нэр нийцтэй байдлаар хөлдсөн — frontend,
native клиент, гадны SSO клиент бүр хуучин талбарыг хардаг. Хоёр гэрээ
тусдаа, тусдаа хувилбартай.

### Одоо хүлээгдэж буй deprecation-ууд

| Юу | Оронд нь | Хэзээ устах |
| --- | --- | --- |
| `nexus.UseLink` | `nexus.Provide[nexus.Link]` | **v2.0.0** |
| `nexus.UseDocumentFiler` | `nexus.Provide[nexus.DocumentFiler]` | **v2.0.0** |
| `nexus.UseAuditSink` | `nexus.Provide[nexus.AuditSink]` | **v2.0.0** |
| `nexus.UseReportSink` | `nexus.Provide[nexus.ReportSink]` | **v2.0.0** |
| Эрхийн `.read`/`.manage` дагаврын дүрэм | `PermissionDefinition.DefaultRoles` | **v2.0.0** |

Тавуулаа 2026-08-21-нд v1.x дотор deprecated болсон. v2.0.0 гарах өдөр
тодорхойгүй бөгөөд энэ жагсаалт нь тэр өдрийн ажлын жагсаалт: v2-ын tag
гаргахын өмнө эдгээр устаж, `api.txt` дахин бичигдэнэ. v1 дотор бүгд бүрэн
ажиллана — устгах нь major-ын ажил, minor-ынх биш.

Дагаврын дүрэм нь бусад дөрвөөс өөр: тэр нь функц биш, зан төлөв. Устгах нь
`DefaultRoles` зарлаагүй модулийн эрхийг **хэн ч авахгүй** болгоно, тиймээс
v2-ын өмнө энэ репогийн модуль бүр зарлах ёстой — `internal/apps/default_roles_test.go`-ийн
хүснэгт нь тэр ажлын жагсаалт.

Deprecation → нэг major цикл хүлээх. Хуучин зүйлийг `// Deprecated:` тэмдэгтэй
болгож, юугаар солихыг нь заана — үсгийн хэлбэр нь чухал: `godoc` ба
staticcheck-ийн SA1019 нь яг энэ бичиглэлийг л таньдаг, `DEPRECATED:` бол
зүгээр л коммент бөгөөд дуудагчийн build юу ч хэлэхгүй; CHANGELOG-ийн "Deprecated" хэсэгт бичнэ;
дараагийн major дээр л хасна. Энэ репод аль хэдийн ийм жишээ бий —
`organisation.LegacyID`, `/api/v1/core/*`, каталогийн хуучин slug-ууд.

## 2. Гэрээг хамгаалдаг механизм

Хүн санаж байхад найдсан амлалт бол яаралтай өдөр эвдэрдэг амлалт. Тиймээс
дөрвөн зүйл барьдаг:

| Юу | Юуг барих | Хаана |
| --- | --- | --- |
| `TestTheExportedAPIIsTheOneOnRecord` | Экспортолсон гадаргуугийн **аливаа** өөрчлөлт | `backend/pkg/nexus/testdata/api.txt` |
| `TestTheSDKDoesNotDependOnInternal` | `pkg/nexus` `internal/` рүү хүрэх | Импортын графыг мөшгинө |
| `Downstream / Canary distribution` | Бодит бүтээгдэхүүн энэ коммит дээр компиллогдож, тестээ дааж байгаа эсэх | `business-gerege-nexus`-ыг clone хийж `replace`-ээр энэ коммит руу заана |
| `Downstream / Minimal distribution` | Хамгийн шинэ гадаргуунууд — `Provide`, `Capability`, `Migrations`, `ProvideAssistant`, `DefaultRoles` | `backend/testdata/canary` |

### Golden файл юуг барьдаггүй вэ

Энэ нь чухал бөгөөд амархан мартагддаг: **golden файл зөвхөн гарын үсгийг
барина.** Дараах бүх өөрчлөлт `api.txt`-ыг ногоон үлдээж, distribution-ыг
эвдэнэ:

* метод хэвээр, буцаах утга нь өөр;
* `Capability[T]()` байхгүй үед алдааны оронд тэг утга буцаах;
* sink өөр дараалалд, эсвэл огт дуудагдахгүй байх;
* бүртгэл хийгдэх ч хүчин төгөлдөр болохгүй байх.

Гараар туршиж баталсан хоёр жишээ:

| Гараар оруулсан эвдрэл | `api.txt` | Downstream |
| --- | --- | --- |
| `MenuPermissionOf` нь `AccessPolicy`-г уншихаа болиод `""` буцаана | 🟢 ногоон | 🔴 `menu permission: got "", want "contacts.read"` |
| `Capability[T]()` байхгүй үед `nil, nil` буцаана | 🟢 ногоон | 🔴 `TestADistributionsModuleIsWiredUpAfterConstruction` |

Downstream ажил нь тэр хоёр баганын ялгааны төлөө байгаа.

Хоёр canary байгаа шалтгаан: `business-gerege-nexus` бол бодит бүтээгдэхүүн
боловч `Provide`, `Capability`, `Migrations`, `ProvideAssistant` үүсэхээс өмнө
бичигдсэн тул тэдгээрт хүрдэггүй. Яг тэдгээр нь чимээгүй эвдрэх магадлал
хамгийн өндөртэй гадаргуунууд — репогийн гаднаас **одоохондоо хэн ч** тэдэн
рүү компайл хийдэггүй.

Эхнийх нь хөлдөөлт биш: API өөрчлөх нь зөв байх нь олонтоо. Гагцхүү
**санамсаргүй** байж болохгүй. Зориудаар өөрчилсөн бол:

```bash
cd backend && go test ./pkg/nexus -update
```

...гээд `testdata/api.txt`-ийн diff-ийг commit-д оруулна. Тэр diff нь
review-д "экосистемийн гэрээ юу авч, юу алдсан"-ыг яг харуулна.

Хоёр дахь нь илүү нарийн зүйлийг барина: `pkg/nexus`-д `internal/`-ийн импорт
орвол энэ репод асуудалгүй компиллогдоно. Зөвхөн distribution-ы build дээр,
хэдэн өдрийн дараа, тэдний хэзээ ч сонсож байгаагүй пакетын нэрээр л
илэрнэ.

## 3. Хувилбарын дугаар

Go-гийн submodule дүрмээр модуль нь `.../open-gerege-nexus/backend` тул
tag нь **`backend/vX.Y.Z`** хэлбэртэй байх ёстой. Зүгээр `v1.0.0` гэсэн tag-ийг
Go тоохгүй.

| Хэсэг | Хэзээ өснө |
| --- | --- |
| **Major** (X) | `pkg/nexus`-ийн API эвдрэх өөрчлөлт. Ховор, бэлтгэлтэй |
| **Minor** (Y) | Шинэ боломж, шинэ API, шинэ модуль. Хоёр долоо хоног тутам |
| **Patch** (Z) | Засвар, аюулгүй байдал. Хэрэгтэй үедээ |

### `PlatformVersion` бол өөр тоо

`internal/kernel/config.PlatformVersion` нь **manifest-ийн шаардлагыг** шалгах
хувилбар (`"platform": ">=1.0.0"`). Модулийн tag-тай нэг байх шаардлагагүй,
бас нэг байлгах гэж оролдох ёсгүй — тэр хоёр өөр гэрээ:

| | Хэнд хандсан | Юуг амладаг |
| --- | --- | --- |
| `backend/vX.Y.Z` tag | Distribution repo | `pkg/nexus`-ийн API |
| `PlatformVersion` | Апп манифест, апп стор | Платформын нийцтэй байдал |

Хоёрыг тэнцүү байлгах дүрэм тавих нь Go модулийн талаарх шийдвэрээр
манифестийн шаардлагыг эвдэх зам юм. Жишээ нь `organisation`, `egov`,
`reports` гурав нь `>=1.0.0` шаарддаг тул `PlatformVersion`-ийг 0.1.0 болгох
нь тэднийг суулгах боломжгүй болгож, сервер асахаа болино (каталогийн
бүрэн бүтэн байдал бол асалтын алдаа).

`PlatformVersion` өсөх нь өөрийн шалтгаантай: платформ модулиудад санал
болгодог зүйлээ нэмэгдүүлбэл minor, эвдвэл major. Түүнийг өөрчлөхдөө
манифестуудын шаардлагыг хамт харна. Build нь `-ldflags`-аар стампддаг —
эс тэгвээс образ бүр өөрийгөө 1.0.0 гэж стор бүрд хэлнэ.

## 4. Release гаргах алхмууд

1. **`main` ногоон эсэхийг шалга** — CI, Security, Docs гурвуулаа.
2. **CHANGELOG-ийн `[Unreleased]`-ийг хувилбарын гарчиг болго**, огноотой.
   Эвдэх өөрчлөлт байвал "Deprecated"-ийн хажууд тусад нь бич.
3. **Tag үүсгэж түлх**:
   ```bash
   git tag -a backend/v1.2.0 -m "backend/v1.2.0"
   git push origin backend/v1.2.0
   ```
4. **Release workflow** үлдсэнийг хийнэ: бүрэн шалгуур ажиллуулж, GitHub
   release үүсгэж, Go module proxy-г халаана.
5. **Distribution-ууд** Renovate-ын PR-аар шинэ tag-ийг авна. Тэдний CI улаан
   болбол энэ release-д асуудал байна гэсэн дохио — цөмийн багт issue очно.

Tag нь **буцаагдахгүй**. Go module proxy нэг татсан хувилбарыг үүрд хадгална,
устгасан tag ч proxy-гоос алга болохгүй. Алдаатай release-ийн ганц засвар нь
дараагийн patch гаргах явдал.

## 5. Хэн батлах вэ

`backend/pkg/nexus`-д хүрсэн PR бүр CODEOWNERS-ийн review шаардана
(`.github/CODEOWNERS`). Шалтгаан нь §1: тэр пакет бол зуун repo-гийн гэрээ
бөгөөд кодоос удаан амьдарна. API-д нөлөөлөх өөрчлөлтөд өмнө нь богино
design doc бичих журам §7-д (Экосистемийн стратеги) заасан.
