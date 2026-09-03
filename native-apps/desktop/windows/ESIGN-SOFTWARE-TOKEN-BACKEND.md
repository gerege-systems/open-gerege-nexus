# NexusGeregeDesktop — "Программ токен" (ESIGN) — Backend талд хэрэгтэй ажил

**Зорилго:** Иргэн физик USB токенгүйгээр, зөвхөн гар утсандаа нэвтэрсэн eID Mongolia / Gerege identity-ээ ашиглаж, төрийн вэбсайтуудад (ДАН / sso.gov.mn дамжуулан, жишээ: isf.mn) **тоон гарын үсгээр нэвтрэх**. Өөрөөр хэлбэл desktop апп нь ESIGN токены дүрд орж, гарын үсгийг гар утасны threshold түлхүүрээр зуруулна.

**Уншигч:** Backend (Go `server/` + web `/api/*`) хариуцсан инженер.

**Товч дүгнэлт:** Desktop апп дангаараа энэ feature-ийг барьж ЧАДАХГҮЙ. Гэхдээ backend-д хэрэгтэй бүх криптографийн primitive `/v3` RP-API дээр аль хэдийн бэлэн байна — үлдсэн ажил нь эдгээрийг desktop-руу гаргах (proxy/wire) + ДАН тал дээрх итгэлцлийн асуудлыг шийдэх.

---

## 1. ESIGN протокол — payload юу шаарддаг вэ

Төрийн сайтын ДАН нэвтрэлт нь дотоод `ws://127.0.0.1:59001` (болон `wss://…:59005`) руу холбогдож, `ENCRYPTED_DATA_SIGN` (type `bb4702f31917793f`) хүсэлт илгээдэг. Токен дараах хариуг буцаах ёстой:

| Талбар | Утга | Хэн үүсгэдэг |
|---|---|---|
| `certificate` | b64(хэрэглэгчийн X.509 cert **DER**) | Токен → **backend-аас авах хэрэгтэй** |
| `signature` | Payload **P** дээрх гарын үсэг (токен нь RSA-SHA256, eID нь ECDSA-SHA256) | Токен → **утасны түлхүүрээр зуруулах** |
| `sn` | Токен сериал дугаар | Синтетикээр үүсгэж болно (cert serial) |
| `keyID` | b64(CKA_ID) | Синтетикээр үүсгэж болно |
| `cipher` | AES-128-CBC-PKCS7(P), key=16 байт | Desktop **локалд** хийнэ |
| `meta` | RSA-PKCS1(IV‖K), серверийн cert-ээр | Desktop **локалд** хийнэ |

**P** = `{ data: "<indented JSON of {_ott, reg-num}>", certificate: "<b64 DER>", sn: "…", keyID: "…" }`.
`signature` нь P-ийн SHA256 дээрх гарын үсэг (hash-then-sign) — тиймээс **SHA256(P)-ийг зуруулж, түүхий гарын үсэг авах** primitive л хэрэгтэй.

---

## 2. Desktop талд ЮУ БАЙГАА / ДУТУУ байгаа

### Байгаа ✅
- **AES / RSA meta / payload угсрах** код бэлэн (`EsignCrypto.cs`, `EsignBridge.cs`, `EsignWebSocketServer.cs`).
- **ws/wss гүүр** ажиллаж байна (`EsignBridgeService`, порт 59001/59005).
- Токентой үед бүх урсгал **батлагдсан, ажилладаг** (physical токеноор isf.mn дээр амжилттай нэвтэрсэн).

### Дутуу ❌ (backend шаардлагатай)
1. **Хэрэглэгчийн гэрчилгээ (DER) авах арга байхгүй.**
   - Login → зөвхөн bearer + нэр/РД (cert буцдаггүй).
   - `/api/status` → "no cert PEM" (зөвхөн гарын үсэг).
   - Dashboard → зөвхөн cert-ийн **тоо**.
2. **Дурын digest-ийг зуруулж түүхий гарын үсэг авах first-party arга хязгаарлагдмал.**
   - Одоо зөвхөн `/api/sign-pdf-start` (PDF digest, PIN2) байгаа бөгөөд эцсийн үр дүн нь **stamped PDF** (`/api/sign-pdf-download`). `/api/status` нь `signatureValueB64`-г буцаадаг ч энэ урсгал PDF-д зориулагдсан, cert буцаадаггүй.
3. **Токен-веб-нэвтрэлт зориудаар устгагдсан** — `VerifyTokenAsync`/`InitiateTokenChallengeAsync` = `"token_login_not_available_on_first_party_backend"`.

---

## 3. Backend-д АЛЬ ХЭДИЙН байгаа primitive (`server/internal/httpapi/server.go`)

Дараах `/v3` RP-API endpoint-ууд яг хэрэгтэй зүйлийг хийж чадна:

| Endpoint | Юу хийдэг | ESIGN-д хэрэглэх нь |
|---|---|---|
| `GET /v3/certificates/etsi/{personEtsi}` (`d.personCertificates`) | Хэрэглэгчийн гэрчилгээ(нүүд), **base64 DER** | `certificate` талбарт шууд ашиглана |
| `POST /v3/signature/notification/etsi/{personEtsi}` (`signSessionBody(digest, …)`) | **Дурын digest** зурж, гар утас руу push → гарын үсэг буцаана | `signature` = SHA256(P) дээрх гарын үсэг |
| `POST /v3/authentication/notification/etsi/{personEtsi}` | Auth challenge (ACSP_V2) зурна | ⚠️ ACSP_V2 бүтэц дээр зурдаг тул P дээр шууд тохирохгүй (доор үзнэ үү) |

DTO-д аль хэдийн бий (`internal/dto/enrollment_dtos.go`):
```go
Certificate     string `json:"certificate"`     // base64 DER — signing cert
AuthCertificate string `json:"authCertificate,omitempty"`
SignatureAlgorithm string `json:"signatureAlgorithm"`
```

> **Санамж:** Эдгээр нь RP-API — RP secret (`rp_sk_…`) / mTLS шаарддаг. Desktop бол **first-party** (`/api/*`, secret агуулдаггүй). Тиймээс ажил нь эдгээрийг first-party урсгалд аюулгүйгээр гаргах явдал.

---

## 4. Backend-аас ХҮСЭХ ажил (тодорхой)

> **Desktop тал бэлэн.** Доорх контрактыг desktop client аль хэдийн хэрэгжүүлж
> дуусгасан (`EsignSoftwareToken.cs`). Backend эдгээр endpoint-ийг first-party
> `/api/*`-д (одоогийн Dashboard/sign-pdf-тэй ижил конвенц: **bearer биш, `personId`
> биетэд**) нэмэхэд шууд ажиллана. Endpoint байхгүй үед desktop `esign_backend_not_available`
> алдаа өгч цэвэрхэн уналт хийдэг.

### (A) Нэвтэрсэн хэрэглэгчийн гэрчилгээ буцаах
```
POST /api/certificates
body:  { "personId": "<РД>" }
→ 200 { "signing": "<b64 DER>", "auth": "<b64 DER>", "certificateLevel": "…" }
```
Backend дотор `/v3/certificates/etsi/{etsi}`-г дуудаад буцаана. (Desktop нэвтрэлтэд
эхлээд `signing` cert-ийг, байхгүй бол `auth`-г ашиглана.)

### (B) Дурын 32-байт digest-ийг зуруулж, түүхий гарын үсэг буцаах
```
POST /api/esign-sign
body:  { "personId": "<РД>", "digestB64": "<SHA256(P)>", "key": "signing", "displayText": "…" }
→ 200 { "sessionId": "…", "pollToken": "…", "vc": "<баталгаажих код>" }

дараа нь (одоогийн /api/status-тай ЯГ ижил):
GET /api/status?sessionId=…&pollToken=…
→ 200 { "state": "RUNNING|COMPLETE", "endResult": "OK|…",
        "signatureValueB64": "<raw signature>", "signatureAlgorithm": "…", "error": "…" }
```
Backend дотор `/v3/signature/notification/etsi/{etsi}` урсгалыг дуудаж, гарын үсгийн
түүхий утгыг **PDF stamp хийхгүйгээр** `signatureValueB64`-д буцаана
(`/api/sign-pdf-start` + `/api/status` загварыг дуурайв).

Энэ 2 endpoint байхад desktop өөрөө P угсрах → SHA256(P) зуруулах → cert хавсаргах →
cipher/meta хийх → ws-д буцаах бүхнийг гүйцэтгэнэ (аль хэдийн хэрэгжсэн).

---

## 5. Шийдэх шаардлагатай нарийн асуудлууд

1. **Аль түлхүүр / cert вэ? (signing PIN2 vs auth PIN1)**
   - `/v3/signature` нь **PIN2 (contentCommitment / signing)** түлхүүрээр **түүхий digest** зурдаг → P дээр шууд баталгаажих гарын үсэг өгнө. **Техникээр хамгийн тохиромжтой.**
   - `/v3/authentication` нь **PIN1 (clientAuth)**-ээр гэхдээ **ACSP_V2 бүтэц** дээр зурдаг тул P дээр шууд тохирохгүй.
   - **Нээлттэй асуулт:** ДАН нэвтрэлтэд аль cert-ийг хүлээн авах вэ? (Нэвтрэлт → ихэвчлэн clientAuth; гэхдээ ESIGN нь баримт-загвар гарын үсэг тул contentCommitment ч болж магадгүй.) Эхлэхдээ **signing cert + /v3/signature** аргаар туршихыг санал болгож байна.

2. **Гарын үсгийн алгоритм (ECDSA vs RSA).** Физик ESIGN токен нь **RSA-SHA256**; eID Mongolia нь **ECDSA-SHA256** (threshold). ДАН-ий ESIGN сервер ECDSA хүлээн авдаг эсэхийг **баталгаажуулах хэрэгтэй**.

3. **Гэрчилгээний итгэлцэл (ХАМГИЙН ЧУХАЛ, ДАН тал).** ДАН/isf.mn нь Gerege / eID Mongolia гэрчилгээг (Gerege L1 → үндэсний root) "хүлээн зөвшөөрөгдсөн тоон гарын үсэг" гэж **бүртгэсэн** байх ёстой. Үгүй бол апп бүх зүйлийг зөв хийсэн ч ДАН татгалзана. Энэ нь Gerege ↔ засгийн газрын **сервер талын интеграц/бүртгэл**.

---

## 6. Санал болгож буй дараалал

1. **ДАН-тай тодруулах (блок #2, #3):** ДАН ESIGN нэвтрэлтэд (а) Gerege CA-гийн cert, (б) ECDSA гарын үсэг хүлээн авах уу? — Энэ хариунаас бусад ажил хамаарна.
2. **Backend endpoint (A)+(B) нэмэх** — `/v3` primitive-ууд бэлэн тул wrapping ажил.
3. **Desktop ESIGN гүүрийг өргөтгөх** — токен байхгүй үед (A)+(B)-ийг ашиглаж payload угсрах. (Апп талын өөрчлөлт харьцангуй бага; угсрах код бэлэн.)
4. **isf.mn дээр end-to-end турших.**

---

## 7. Холбогдох файлууд (лавлагаа)

**Desktop:**
- `src/NexusGeregeDesktop.Infrastructure/Esign/` — `EsignBridge.cs`, `EsignCrypto.cs`, `EsignWebSocketServer.cs`, `EsignBridgeHostedService.cs`
- `src/NexusGeregeDesktop.Infrastructure/Auth/RpAuthService.cs` — `/api/sign-pdf-start` + `/api/status` (raw signature буцаах жишээ загвар)
- `src/NexusGeregeDesktop.Infrastructure/Auth/CitizenAuthService.cs` — first-party login; M9.B токен-веб-нэвтрэлт байхгүй болохыг тэмдэглэсэн

**Backend (Go):**
- `server/internal/httpapi/server.go` — `/v3/certificates/etsi/{personEtsi}`, `/v3/signature/notification/etsi/{personEtsi}`, `/v3/authentication/notification/etsi/{personEtsi}`
- `server/internal/dto/enrollment_dtos.go`, `dtos.go` — cert / signature DTO-ууд
- `server/internal/crypto/acsp.go` — ACSP_V2 payload (auth гарын үсэг)
- `sdk/README.md`, `sdk/typescript/` — RP-API-г ашиглах ResponseValidator загвар

**Web:**
- `web/src/lib/rpclient*.ts` + `web/src/app/api/*` — first-party `/api/*` нь `/v3`-руу proxy хийдэг загвар (шинэ endpoint нэмэх байрлал)
