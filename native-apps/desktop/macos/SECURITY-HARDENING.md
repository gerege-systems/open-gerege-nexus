# NexusGeregeDesktop (macOS) — Security Hardening

Энэ баримт нь macOS desktop клиентийн аюулгүй байдлын audit болон hardening-ийн
төлөвлөгөө, хэрэгжүүлэлтийг тэмдэглэнэ. (iOS клиенттэй parity, defense-in-depth.)

> **Гарал үүсэл.** Энэ файл ба доорх SEC-1/2/3 хэрэгжилтүүд нь e-ID Mongolia-гийн
> клиентээс порт хийгдсэн. 2026-09-05-нд Nexus-ын бодит байдалд тааруулж
> залруулсан: хост нэрс, pinning-ийн opt-in гарц, гарын үсгийн мөр гурав нь
> порт хийгдсэн хэвээрээ үлдэж, байхгүй зүйлийг «✅» гэж бичсэн байв.

> ⚠️ **Гол зарчим:** client-side hardening нь бинартай суусан тэвчээртэй
> халдагчийг **зогсоохгүй — зөвхөн зардлыг өсгөнө.** Эцсийн эх сурвалж нь
> **server-side** (HMAC, session/token validation, ирээдүйд device attestation).

## Baseline (audit олдвор)

| Хэсэг | Төлөв |
|---|---|
| TLS cert pinning | ⚠️ Код бэлэн (SEC-1), гэхдээ Release-д **opt-in** — `UserDefaults["security.tlsPinning"]`, анхдагч OFF. Унтраалттай үед system trust л ажиллана |
| Hardened Runtime | ✅ On — library validation идэвхтэй (DYLD injection ихэвчлэн блок) |
| Code signature / notarize | ❌ **Байхгүй.** `build.sh` нь `CODE_SIGNING_ALLOWED=NO`, `native-release.yml`-ийн артефакт нь `macos-arm64-unsigned`. Gatekeeper-ийн зам хараахан шийдэгдээгүй |
| App Sandbox | ❌ Байхгүй (network.client + smartcard entitlement) |
| Secret хадгалалт | ✅ Keychain (bearer + device_secret) + HMAC-SHA256 |
| Anti-debug / runtime integrity | ✅ Нэмэгдсэн (SEC-2) |
| Root / SIP signal | ✅ Нэмэгдсэн (SEC-3) |
| String obfuscation | ✅ Pin/requirement/dylib нэр XOR (SEC-3) + class нэр (obfuscate_build.sh) |

## SEC-1 — TLS certificate pinning (P0)

- `Infrastructure/Security/Pinning/SPKIHash.swift` — cert-ийн SubjectPublicKeyInfo
  SHA-256 (base64). EC P-256/P-384 + RSA-2048/4096 ASN.1 prefix. (iOS-оос порт.)
- `Infrastructure/Security/Pinning/PinnedSessionDelegate.swift` — `URLSessionDelegate`,
  chain-walk, гинжний **дурын** pin (leaf эсвэл issuer) таарвал зөвшөөрнө.
  **DEBUG-д bypass** (dev loop).
- **Release-д ч opt-in.** `UserDefaults["security.tlsPinning"]` асаагүй бол system
  trust-аар л явна. Шалтгаан нь порт хийх үед бодит байсан: баригдсан pin-үүд шинэ
  хостын CA гинжтэй таарахгүй байж болзошгүй байв. Байрлуулалт бүр үүнийг асаах
  эсэхээ мэдэж шийдэх ёстой — асаагаагүй бол «pinning бий» гэж хэлэх үндэсгүй.
- Pin багц (`Obfuscated.pins`): **Let's Encrypt E7 + E8 завсрын** БА
  **ISRG Root X1 + X2**. Эхэндээ зөвхөн завсрынх байсан бөгөөд `nexus.gerege.mn`
  YE2-оор гарын үсэг зурагдахад таарахаа больж, Release build нэвтрэх дэлгэц дээр
  «A TLS error caused the secure connection to fail» өгч байв — үндсийг нэмсэн
  шалтгаан нь тэр. Windows клиент (`appsettings.json` → `CertificateSpkiPins`)
  ижил багцтай.
- `Core/Network/APIClient`-ийн `URLSession`-д delegate залгасан.

**Хамрах хүрээ.** Pinning нь `URLSession`-ий дуудлагад л үйлчилнэ. Ажлын мужийг
үзүүлдэг `WKWebView` нь өөрийн сүлжээний стектэй бөгөөд системийн trust store-оор
явна — өөрөөр хэлбэл дэлгэцийн ихэнх агуулга pin-ий гадна байна.

Pin шинэчлэх (CA ротац): `openssl s_client -connect <host>:443 -showcerts` → шинэ
гэрчилгээний SPKI SHA-256-г `Obfuscated.pins`-д нэмнэ. iOS ба Windows-тэй **ижил
pin багц** баримтлана.

## SEC-2 — Anti-debug + runtime integrity (P1)

`Infrastructure/Security/Integrity/SecurityGuard.swift`:
- **Debugger detect** — `sysctl(KERN_PROC, P_TRACED)`.
- **Debugger attach хориглох** — `ptrace(PT_DENY_ATTACH)` эрт `main`-д.
- **Өөрийн гарын үсэг batalgaa** — `SecCodeCopySelf` + `SecStaticCodeCheckValidity`,
  requirement: Apple anchor + team OU `CQTHTD6YJQ`.
- **DYLD injection** — `DYLD_INSERT_LIBRARIES` env + loaded image сэжигтэй нэр.
- **Enforce:** зөвхөн Release (`#if !DEBUG`). Илрэхэд → Keychain session устгах + exit.

## SEC-3 — Env integrity + obfuscation (P2)

- **Root** — `getuid()==0` → татгалзах signal.
- **SIP** — best-effort (`csr_get_active_config` SPI) → degrade signal.
- **String obfuscation** — pin hash, requirement string, dylib нэрсийг XOR-оор
  далдалж runtime-д угсарна (`strings`-д plaintext харагдахгүй).
- **Scattered checks** — `SecurityGuard.enforce()`-ийг олон цэгээс дуудна (launch,
  dashboard, token sign/login-ийн өмнө) — нэг patch бүгдийг унтраахгүй.

## Хязгаарлалт ба анхаарах зүйл

- Бүх шалгалт Release-д хүчтэй, **DEBUG-д унтраалттай** (Xcode debug, dev loop эвдэхгүй).
- Sparkle auto-update, notarization, crash reporter-т саад болохгүй (ptrace/SecCode зөвшөөрөгдсөн).
- Anti-tamper нь **deterrent** — server-side enforcement үргэлж нэн тэргүүн.
- Pin ротац: үндэс + завсрын хосолсон багц тул LE-ийн E-цувралын ротац дангаараа
  клиентийг унагахгүй; ISRG үндэс солигдох үед л заавал шинэчилнэ.
- **SEC-2-ийн гарын үсгийн шалгалт өнөөдөр ажиллах боломжгүй**: `SecCodeCopySelf`
  requirement нь team OU `CQTHTD6YJQ`-г шаарддаг ч бүтээгдсэн бинарь гарын үсэггүй.
  Nexus өөрийн Developer ID авах хүртэл энэ шалгалт Release-д бүтэлгүйтэх эсвэл
  утгагүй байна — гарын үсгийн ажилтай хамт дахин үзэх зүйл.
