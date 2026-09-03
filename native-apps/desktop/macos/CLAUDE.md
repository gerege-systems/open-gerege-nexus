# e-ID Mongolia macOS Desktop — CLAUDE.md

## Тойм

macOS desktop app (SwiftUI) — **first-party клиент**. iOS app шиг клиентэд RP
secret/бүртгэл огт байхгүй: бүх дуудлага өөрийн web backend-ийн нийтийн
`/api/*` route-уудаар (browser-тэй яг ижил зам) дамжина. Go RP-API (`/v3/*`)-ийн
secret-ийг web сервер (`web/src/lib/rpclient.ts`, `RP_API_SECRET` env) л барьдаг.
`../eid-mongolia-mn/desktop-app/macos-app`-аас порт хийсэн.

## Tech Stack

- Swift 5.10, Xcode 16+, macOS 14+
- SwiftUI, CoreImage (QR), Security (Keychain)
- GeregeTokenKit (`../gerege-token-kit`, local SPM) — USB FEITIAN token (локал PKCS#11/APDU)
- Sparkle 2 (auto-update), XcodeGen (`project.yml`)

## Урсгалууд (web-ийн нийтийн route-ууд — `web/src/app/api/*`)

### Нэвтрэлт (LoginView)
- **QR**: `POST /api/start` → `{sessionId, qr, vc}` — QR-д `qr` (= sessionId)
  кодлоно, `vc`-г дэлгэцэнд харуулна.
- **РД push**: `POST /api/login-notify {register}` → `{sessionId, vc}`
  (rate limit 3/60с per register).
- Poll: `GET /api/status?sessionId=&pollToken=` (`APIClient.waitForAuth`, ~0.4с интервал,
  сервер тал 1с барина). `COMPLETE`+`OK` үед **web сервер** гарын үсгийг
  криптограф баталгаажуулж (`verifyAuthIdentity`) cert subject-оос
  `name`/`idNumber`-ийг задалж өгнө — desktop cert parse хийхгүй.
- Үр дүн → `StoredIdentity` (documentNumber, fullName, civilID, certificateLevel)
  Keychain-д (`KeychainKey.identity`), restore үед Touch ID gate.

### Identity загвар
Bearer session байхгүй — `documentNumber` нь identity handle (web live demo-гийн
`x-eid-token`-той ижил). Dashboard өгөгдөл нь **локал угсралт**: identity +
локал activity log (`UserDefaults["activity.sessions"]`).

### Гарын үсэг (SignView) — web demo хуудастай яг ижил урсгал
1. PDF сонгоод SHA-256 digest-ийг **локал** тооцно (stamp хийх ижил байт)
2. `POST /api/sign-pdf-start {etsi, digestB64, fileName, callbackUrl:""}` →
   `{sessionId, vc, pollToken}` — утас руу PIN2 push (rate limit 3/60с per etsi).
   `etsi` = нэвтрэлтээс олдсон иргэний дугаар (`civilID`, fallback `nationalID`).
3. `GET /api/status?sessionId=&pollToken=` poll → COMPLETE/OK (auth-тай ижил зам)
4. `POST /api/sign-pdf-download` (multipart `file` + `sessionId` + `pollToken`) →
   тамгалагдсан (PAdES/PKCS#7 + баталгаажуулах хуудас) PDF → Downloads

## Тохиргоо (`Core/Network/AppConfig.swift`)

| Утга | Эх сурвалж (дараалал) | Default |
|---|---|---|
| Сервер URL (web app) | `UserDefaults["API_BASE_URL_OVERRIDE"]` (Settings) → env `API_BASE_URL` | DEBUG: `http://localhost:3000`, Release: `https://eidmongolia.mn` |

Secret, RP UUID, RP name — **байхгүй** (first-party).

## Бүтэц

```
App/            GeregeApp (Sparkle, MenuBarExtra), AppState (identity + локал log), ContentView
Core/Network/   AppConfig, Endpoints (web /api routes), APIClient (actor, multipart + poll)
Core/Keychain/  KeychainManager (.identity key)
Core/Token/     TokenManager, TokenProvisioner (локал PKCS#11/APDU)
Domain/         Certificates/CertificateInfo, Primitives, Tokens
Infrastructure/ Security (Pinning/Integrity), System (login item), Certificates (X.509 parse — VerifyView)
Presentation/   Localization (L10nCatalog auto-gen + LocalizationService)
Features/       Login, Main (shell), Dashboard (Page/Tabs/Home/WebParity), Sign, Verify, Settings, MenuBar
Design/         Colors, Typography, Styles, UserAvatar
```

**Хуучин app-аас устгагдсан** (v3-д backend байхгүй эсвэл хэзээ ч холбогдоогүй):
dead stack (Application/Abstractions, Infrastructure impl-ууд, AppContainer,
Presentation/ViewModels), Passkey, Org, RP Verifier, cloud storage, deep-link
web login, англи нэр засах, токены CSR enroll (CSR одоо PEM экспорт л хийнэ).

Sidebar nav: dashboard / id / organizations / children / logs / sign / tokens (+ footer
support/privacy/lock). `organizations`/`children` нь ЗӨВХӨН УНШИХ — нэвтрэх агшинд
`/api/representations` + `/api/children`-ээс татсан snapshot (бүртгэх/цуцлах нь утсаар).
`devices`/`security`/`home`/`verify` case-ууд бий ч nav-аас нуусан.

## Build & Run

```bash
cd desktop/macos-app
xcodegen generate
xcodebuild -project NexusGeregeDesktop.xcodeproj -scheme NexusGeregeDesktop \
           -configuration Debug -destination 'platform=macOS,arch=arm64' build
open NexusGeregeDesktop.xcodeproj    # ⌘R
```

⚠ `-scheme NexusGeregeDesktop` (`-target` биш) — local `GeregeTokenKit` SPM resolve-д шаардлагатай.

Локал туршилт: Go сервер (`cd server && go run ./cmd/smartid`, :8080) + web
(`cd web && npm run dev`, :3000) хоёуланг нь асаана — DEBUG default сервер нь
web (:3000). Утасны app мөн ижил Go серверт заасан байх ёстой.

## Security

- Клиентэд secret байхгүй — нийтийн route-ууд rate limit-ээр хамгаалагдсан
  (web талд). Identity verify нь web сервер дээр (H3 ACSP_V2 шалгалт).
- SEC-1 SPKI pinning: DEBUG-д bypass; Release-д **opt-in** —
  `UserDefaults["security.tlsPinning"] = true` (хуучин LE pin-үүд шинэ хостын
  CA-тай таарахгүй байж болзошгүй тул default OFF, system trust хэвээр).
- SEC-2/3 SecurityGuard (anti-debug/tamper) — Release-д л идэвхтэй.

## Дүрэм

- `async/await` — callback хориотой; `@MainActor` UI update бүрт
- Error: Монгол мессежтэй `LocalizedError`
- Commit: conventional commits, English
- `L10nCatalog.swift` auto-generated — гараар бүү зас (`scripts/gen_l10n.py`)

## Программ токен (ESIGN) — ДАН/sso.gov.mn руу USB токенгүй нэвтрэх

`Core/Esign/` — Windows гүүрийн (`EsignProtocol.cs` + `EsignHybrid.cs`) macOS порт. Апп
асахад `ws://127.0.0.1:59001` дээр сонсдог (RFC 6455-ыг `NWProtocolWebSocket` хийнэ;
listener нь `requiredLocalEndpoint`-оор ЗӨВХӨН loopback-д bind хийгдэнэ).

Хүсэлт `{ "type":"bb4702f31917793f", "data":{ "_ott", "reg-num" }, "cert":"<b64 серверийн DER>" }`
ирэхэд:

1. `P` = compact JSON `{ data, certificate, sn, keyID }` — `data` нь хүсэлтийн `data`
   объектийн **indented JSON string**; `sn` = хэрэглэгчийн cert-ийн серийн дугаар,
   `keyID` = гэрчилгээний SubjectKeyIdentifier (байхгүй бол public key-ийн SHA-1) —
   Windows клиентийн `EsignCertParser.KeyId`-тэй ЯГ ижил дүрэм. `sn` нь ASN.1 INTEGER-ийн
   утга тул тэргүүлэх `00` padding хасагдана (`X509Certificate2.SerialNumber`-тэй дүйцнэ).
2. `signature` = SHA256(P)-г **утсаар** зуруулна (`/api/esign-sign` → PIN2 push →
   `/api/status`). Хэрэглэгчид баталгаажуулах кодыг `ContentView`-ийн overlay-гээр харуулна.
3. `cipher` = AES-128-CBC/PKCS7(P), `meta` = RSA-PKCS1(IV‖K) хүсэлтэд ирсэн серверийн cert-ээр.

Гэрчилгээ нь `/api/certificates` (session-bound) тул **нэвтрэх агшинд** татагдаж
`StoredIdentity`-д (Keychain) кэшлэгдэнэ — `AppState.loadPersonExtras`.

```bash
bash scripts/esign_interop_test.sh   # крипто: аппын код → OpenSSL-ээр тайлж тулгана
```
ws давхаргыг гараар шалгах: аппыг ажиллуулаад 59001 руу RFC 6455 клиентээр дээрх JSON-ыг
илгээхэд нэвтрээгүй үед `{"status":"error","message":"Нэвтрээгүй байна…"}` буцаана.

**Дуусаагүй / хараат зүйлс**
- `wss://127.0.0.1:59005` БАЙХГҮЙ (Windows гүүрт бий). `NWProtocolTLS` нь `SecIdentity`
  шаарддаг ба Security.framework-д self-sign API байхгүй. Chrome/Edge нь https хуудаснаас
  `ws://127.0.0.1`-ыг зөвшөөрдөг тул эхний хувилбарт хангалттай; Safari/Firefox шаардвал нэмнэ.
- Апп **sandbox-гүй** тул `com.apple.security.network.server` entitlement хэрэггүй. Хэрэв
  хожим sandbox асаавал ҮҮНИЙГ нэмэхгүй бол гүүр чимээгүй асахаа болино.
- **Гарын үсгийн алгоритм — 2026-09-01-нд бодитоор хэмжив.** Бүтэн end-to-end турших үед
  (ws → утас → PIN2 → OpenSSL-ээр шалгах) гарын үсэг нь **RSA-SHA256-PKCS1, 512 байт** гарсан:
  тухайн иргэний гэрчилгээ нь **RSA-4095** (`CN=eID Mongolia Issuing CA`), өөрөөр хэлбэл
  төхөөрөмж нь **SplitKey** схемээр бүртгэгдсэн. Энэ нь физик ESIGN токены гаралттай ЯГ ИЖИЛ
  анги — тиймээс "ДАН ECDSA хүлээж авах уу?" гэсэн эрсдэл SplitKey бүртгэлтэй иргэнд
  ХАМААРАХГҮЙ. ECDSA схемээр бүртгэгдсэн иргэнд л тэр асуулт үлдэнэ (`approveAuto` схемээр
  салаалдаг).
- Gerege CA-г ДАН талд "хүлээн зөвшөөрөгдсөн тоон гарын үсэг" гэж бүртгүүлэх нь ЗАСГИЙН
  ГАЗРЫН талын ажил хэвээр — үүнгүйгээр код зөв ажиллаад ч ДАН татгалзана.
