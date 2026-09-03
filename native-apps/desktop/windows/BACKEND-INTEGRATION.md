# Windows app — backend интеграцийн төлөв (eID Mongolia)

Энэ апп нь `gerege-systems/eid-gerege-mn-v1/desktop-app/windows-app`-аас
**eID Mongolia** болгон rebrand хийгдэж (`EidGerege*` → `NexusGeregeDesktop*`,
`mn.gerege-nexus.desktop` → `mn.eidmongol.desktop`, `e-ID Gerege` → `e-ID Mongolia`,
`gerege-nexus.mn` → `eidmongolia.mn`) энэ repo-д орж ирсэн.

## Trust model — юу өөрчлөгдсөн

Эх (gerege) апп нь backend-руу **шууд** `/web2app/v1/*` + `/rp/v1/*` замаар,
**HMAC device secret** (`X-Device-Id`/`X-Signature`) болон **Bearer token**-той
холбогддог RP-эрхтэй клиент байсан.

Энэ platform (`eid-platform-mn`)-ийн desktop trust model нь **эсрэг** (CLAUDE.md §desktop):
desktop нь iOS апп шиг **first-party клиент** — RP secret, device HMAC secret,
удаан насладаг bearer token **аль нь ч байхгүй**. Бүх дуудлага **web** backend-ийн
нийтийн `/api/*` route-уудаар (browser-тэй яг ижил) явна; RP secret зөвхөн web
серверт үлдэнэ.

Иймд дараах бүтцийн өөрчлөлтийг **аль хэдийн хийсэн**:

| Хийсэн | Файл |
|---|---|
| HMAC + Bearer handler-уудыг HTTP pipeline-аас хассан | `NexusGeregeDesktop.Infrastructure/DependencyInjection.cs` |
| `BaseUrl` → web origin (`https://eidmongolia.mn`, dev `http://localhost:3000`) | `appsettings.json`, `appsettings.Development.json` |
| SPKI pin, X-Road client утгуудыг цэвэрлэсэн | `appsettings*.json` |

`HmacSigningHandler`, `BearerAuthHandler` класс-ууд tree-д хэвээр (pipeline-д
холбогдоогүй) — direct-backend build дахин хэрэгтэй бол лавлагаа болгож үлдээв.

## Endpoint map — үлдсэн rewire

`web/src/app/api/*` (Next.js) нь desktop-д зориулсан нийтийн гэрээ. Гэрээ:

| Route | Method | Body / Query | Хариу |
|---|---|---|---|
| `/api/start` | POST | — | `{sessionId, qr, deviceLinkBase, vc, pollToken}` |
| `/api/login-notify` | POST | `{register, callbackUrl?}` | `{sessionId, vc, pollToken}` |
| `/api/status` | GET | `?sessionId=&pollToken=` | `{status, name?, idNumber?, …}` |
| `/api/sign-pdf-start` | POST | `{etsi, digestB64, fileName, callbackUrl}` | `{sessionId, vc, pollToken}` |
| `/api/sign-pdf-download` | GET | `?…` | PDF |
| `/api/health` | GET | — | `{status}` |
| `/api/dashboard` | POST | `{sessionId, pollToken}` | person summary (cert/activity/device/rep тоолол + нэр) |
| `/api/certificates` | POST | `{sessionId, pollToken}` | `{personEtsi, counts, certificates[]}` |
| `/api/devices` | POST | `{sessionId, pollToken}` | `{devices[], activeCount, total}` |
| `/api/activity` | POST | `{sessionId, pollToken, limit?}` | `{counts, sessions[], total}` (RP-scoped) |
| `/api/representations` | POST | `{personId}` | `{representations[]}` (төлөөлж чадах ACTIVE org) |

> ⚠️ **BREAKING (session-bound хамгаалалт, 2026-07):** `dashboard`·`certificates`·
> `devices`·`activity` route-ууд `{personId}`-г БАТАЛГААГҮЙ хүлээж авдаггүй
> (аудит: broken access control — civil_id нь нууц биш тул хэн ч дурын хүний
> төхөөрөмж/гэрчилгээг татаж авах эрсдэлтэй байсан). Эдгээр нь login flow-оос гарсан
> **`sessionId` + `pollToken`**-ыг шаардаж, буцаах identity-г дуусаж баталгаажсан
> session-ий гэрчилгээнээс гаргана. Desktop тал үүнийг `WebSession.Token`
> (=sessionId) + `WebSession.PollToken`-оор дамжуулдаг — `DashboardService`,
> `CitizenCertificateService`, `OrgService`, `GuardianService`, `EsignSoftwareToken`
> бүгд ижил `SessionAuthBody`-той. `/api/representations` нь **өөрчлөгдөөгүй** (RP-ийн нээлттэй lookup,
> `loadSignOrgs` дахь оператор-бичсэн civil_id урсгалыг дэмжсээр).

`sessionId`+`pollToken` = login-notify/start хариунд ирсэн session credential.
`personId` (representations) = иргэний civil_id/etsi. Эдгээр нь Go RP-API-ийн
`/v3/person/summary`·`/certificates`·`/devices`·`/rp/activity`·
`/organization/representations`-руу first-party RP Bearer-ээр proxy хийнэ
(first-party RP оператор тул PKI_READ чөлөөтэй). Rate limit 60с-д 10.

**Гол ялгаа (impedance mismatch):**
- `sessionId` нь **string** (gerege нь `Guid`). Windows апп бүх interface,
  `WebSessionStore`, ViewModel даяар `Guid`-аар дамжуулдаг тул rewire нь зөвхөн
  URL солих биш — **төрлийн** өөрчлөлт (Guid → string).
- `/api/status` нь **`session_token`/`user_id` буцаадаггүй** — амжилттай нэвтрэхэд
  cert subject-оос **`name` + `idNumber`** шууд ирдэг. Иймд login-ийн дараах
  identity model (`WebSession{ Token, UserId, FullName }`, `/me`, `/dashboard`)
  нь bearer-д тулгуурлахаа болих ёстой.

### Service бүрийн rewire — төлөв

Статус: ✅ хийгдсэн · 🔁 stub (эх нь local-д алга) · ⏳ хараахан хийгээгүй (404 болж
graceful degrade хийнэ).

| Service (`NexusGeregeDesktop.Infrastructure/…`) | Төлөв | `/api/*` буулгалт |
|---|---|---|
| `Http/BackendApiClient.GetHealthAsync` | ✅ | `GET /api/health` (live=ready=200) |
| `Auth/CitizenAuthService.InitiateAsync` | ✅ | `POST /api/login-notify {register}` |
| `Auth/CitizenAuthService.InitiateQrAsync` | ✅ | `POST /api/start` (`qr`→`DeviceLinkUrl`, `vc`→`ControlCode`) |
| `Auth/CitizenAuthService.PollAsync` | ✅ | `GET /api/status?sessionId=&pollToken=` (`state`/`endResult`; complete→`name`/`idNumber`) |
| `Auth/CitizenAuthService.LogoutAsync` | ✅ | Сервер logout байхгүй → зөвхөн local session + identity cache цэвэрлэнэ |
| `Auth/UserProfileService.GetMeAsync` | ✅ | `/me` байхгүй → **`ISessionIdentityCache`**-аас (login-complete дэх `name`/`idNumber`) |
| `Auth/CitizenAuthService.InitiateTokenChallenge/VerifyToken` | 🔁 | USB-token web-login local-д алга → `ApiError.Internal("token_login_not_available_on_first_party_backend")` |
| `Auth/RpAuthService.*` (PDF sign) | ✅ | `POST /api/sign-pdf-start` + `GET /api/status` (digest-ийг клиент SHA-256-аар тооцно) |
| `Dashboard/DashboardService.LoadAsync` | ✅ | `/api/dashboard` + `/api/devices` + `/api/activity` — session-bound (`sessionId`+`pollToken`); харуулах identity нь `ISessionIdentityCache`-ээс. Devices/activity нь best-effort (уналт нь самбарыг хоосруулахгүй) |
| `Org/OrgService.ListAsync` | ✅ | `/api/representations` (нэвтэрсэн иргэний ACTIVE төлөөлөл) |
| `Org/OrgService.*` (lookup/register/members/name-en/xroad) | 🔁 | org-WRITE — `/api/*` эквивалент алга → stub |
| `Certificates/CitizenCertificateService.ListAsync` | ✅ | `POST /api/certificates {sessionId, pollToken}` → "Миний гэрчилгээ" хуудас (`CertificatesPage` + `CertificatesViewModel`). Мөр бүрийн `certValue` (b64 DER)-ийг `CertDetailDialog` задалж бүрэн X.509 харуулж, файлаар экспортлоно |
| `Auth/CertEnrollmentService.*` (CSR enroll) | 🔁 | cert enroll — `/api/*` эквивалент алга → stub |
| `Verify` flow | ⏳ | Client-side cert parse-аар (macOS-той адил) — сонголтоор, backend шаардлагагүй |

**Identity bridge (impedance mismatch-ийн шийдэл):** `/api/status` нь bearer/`user_id`
буцаадаггүй, complete үед `name`+`idNumber`-ийг **шууд** өгдөг. Иймд
`CitizenAuthService.PollAsync` нь энэ identity-г `ISessionIdentityCache`-д хадгалж,
session бүрд синтетик `SessionToken`(=sessionId)+`UserId`(GUID) үүсгэнэ;
`UserProfileService.GetMeAsync` нь cache-аас уншина. Ингэснээр login ViewModel-ийн
`Confirmed → GetMeAsync` урсгал **өөрчлөлтгүй** ажиллана.

🔁 stub-ууд нь local backend-д эквивалент endpoint байхгүй тул `ApiError.Internal(
"not_available_on_first_party_backend")` буцаана (crash биш) — харгалзах UI цэвэрхэн
унана. Эдгээрийг жинхэнэ функц болгоход **backend талын шинэ route** (org/dashboard/
cert-enroll) шаардана — энэ нь first-party `/api/*` хамрах хүрээнээс гадуур.
Дэд ажил: `HmacSigningHandler`/`BearerAuthHandler` нь pipeline-д холбогдоогүй үлдсэн
(direct-backend build-д лавлагаа).

## Build / verify

`/api/*` rewire + config/pipeline адаптаци **дууссан**. Windows CI
(`.github/workflows/windows-app.yml` — 2026-08-06-нд устгасан, GitLab-д Windows runner алга) нь solution-ыг бүрэн build
хийж (**ногоон**), MSIX багцалдаг. macOS дээр WinUI/MSIX build хийх боломжгүй тул
локал шалгалтыг Windows дээр:

```powershell
dotnet restore
dotnet build NexusGeregeDesktop.sln -c Debug
dotnet test tests/NexusGeregeDesktop.UnitTests
# packaging: tools/pack-msix.ps1
```

## ESIGN "программ токен" — backend контракт (хэрэгжсэн)

Иргэн физик USB токенгүйгээр ДАН/`isf.mn`-д тоон гарын үсгээр нэвтрэхэд desktop нь ESIGN
токены дүрд ордог: ws `127.0.0.1:59001` дээрх `ENCRYPTED_DATA_SIGN`-д хариулахын тулд (а)
иргэний **гэрчилгээ DER**, (б) угсарсан **P payload-ийн SHA256 дээрх түүхий гарын үсэг**
хэрэгтэй. Хоёуланг нь одоо first-party `/api/*` өгнө.

Аль эх сурвалжаар зурахыг `EsignBridge` шийднэ: анхдагчаар **залгаастай физик токен
давуу**, токен олдоогүй үед программ токен. Тохиргоо хуудсын "Программ токен ашиглах"
(`IEsignPreferences`, Windows LocalSettings-д хадгална) сэлгүүрийг асаавал токен
залгаастай ч физик токеныг огт хайлгүй шууд программ токеноор зурна. Мөн тэндээс локал
гүүрийн төлөв (ws:59001 / wss:59005) харагдаж, гараар асааж/зогсоох боломжтой.

### (A) Гэрчилгээ — `POST /api/certificates`

```jsonc
// хүсэлт — session-bound (personId БИШ, доорх тайлбарыг үз)
{ "sessionId": "…", "pollToken": "…" }
// хариу — өмнөх жагсаалт ХЭВЭЭР, дээр нь:
{ "signing": "<b64 DER>", "auth": "<b64 DER>", "certificateLevel": "QUALIFIED",
  "personEtsi": "PNOMN-…", "counts": {…}, "certificates": [ { …, "certValue": "<b64 DER>" } ] }
```

`signing` = PIN2 (contentCommitment), `auth` = PIN1 (clientAuth); тус бүрд ХҮЧИНТЭЙ
(`VALID`) гэрчилгээнүүдээс хамгийн шинийг сонгоно, олдоогүй бол `""`.

> **`personId`-г ЗОРИУД хүлээж авахгүй.** Эдгээр person-PKI route нь өмнө нь клиентээс
> ирсэн `personId`(civil_id)-д баталгаагүй итгэдэг байсныг **broken access control** гэж
> аудитаар илрүүлж зассан (`web/src/lib/sessionIdentity.ts`): civil_id нь нууц биш
> үндэсний дугаар тул хэн ч дурын хүний PKI-г татах боломжтой байв. Иймд desktop нь
> нэвтрэлтийн `sessionId`+`pollToken` хосыг хадгалж энд үзүүлнэ.
> Нэмэлт: `GET /api/status` одоо `certificateDerB64`-ыг ч буцаадаг тул нэвтрэлтийн
> **auth** гэрчилгээ нэмэлт дуудлагагүйгээр гарт орно (signing олдоогүй үеийн fallback).

### (B) Дурын digest зуруулах — `POST /api/esign-sign`

```jsonc
// хүсэлт
{ "personId": "<РД>", "digestB64": "<SHA256(P), 32 байт>",
  "key": "signing", "displayText": "Цахим гарын үсгээр нэвтрэх" }
// хариу
{ "sessionId": "…", "pollToken": "…", "vc": "1234" }

// дараа нь — одоо байгаа poll:
GET /api/status?sessionId=…&pollToken=…
→ { "state": "COMPLETE", "endResult": "OK",
    "signatureValueB64": "<түүхий гарын үсэг>", "signatureAlgorithm": "…",
    "certificateDerB64": "<b64 DER>" }
```

`/api/sign-pdf-start`-тай ижил posture: `personId` биетэд (push урсгал тул), утас руу PIN2
push, 60 секундэд 3 удаа (rate limit). PDF stamp ХИЙХГҮЙ — гарын үсгийн түүхий утга шууд.

`key` нь одоогоор **зөвхөн `signing`**. `auth` (PIN1) нь ACSP_V2 бүтэц дээр зурдаг тул
ДАН-ий P payload дээр тохирохгүй — `400 unsupported_key` буцаана (чимээгүй буруу гарын
үсэг өгөхгүйн тулд).

### Хэрэгжээгүй үлдсэн (backend-ийн гадна)

1. **ДАН тал:** Gerege/eID гэрчилгээг "хүлээн зөвшөөрөгдсөн тоон гарын үсэг" гэж
   бүртгүүлэх. Энэгүйгээр апп бүхнийг зөв хийсэн ч ДАН татгалзана.

   *Алгоритмын эрсдэл багассан (2026-09-01, macOS клиент дээр хэмжив):* бодит end-to-end
   туршилтад гарын үсэг **RSA-SHA256-PKCS1, 512 байт** гарсан — иргэний гэрчилгээ RSA-4095,
   төхөөрөмж нь **SplitKey** схемээр бүртгэгдсэн. Энэ нь физик ESIGN токены гаралттай ижил
   анги тул "ДАН ECDSA хүлээж авах уу?" гэсэн асуулт SplitKey бүртгэлтэй иргэнд хамаарахгүй.
   ECDSA схемээр бүртгэгдсэн иргэнд л үлдэнэ.
2. **Аль гэрчилгээгээр нэвтрэхийг ДАН хүлээн авах вэ** (signing vs auth) — эхний туршилтыг
   `signing`-оор хийхээр контракт бэлдсэн; auth хэрэгтэй бол ACSP_V2 асуудлыг эхлээд шийднэ.
