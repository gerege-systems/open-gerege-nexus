# e-ID Mongolia macOS — Security Hardening

Энэ баримт нь macOS desktop app-ийн аюулгүй байдлын audit болон hardening-ийн
төлөвлөгөө, хэрэгжүүлэлтийг тэмдэглэнэ. (iOS app-тай parity, defense-in-depth.)

> ⚠️ **Гол зарчим:** client-side hardening нь бинартай суусан тэвчээртэй
> халдагчийг **зогсоохгүй — зөвхөн зардлыг өсгөнө.** Эцсийн эх сурвалж нь
> **server-side** (HMAC, session/token validation, ирээдүйд device attestation).

## Baseline (audit олдвор)

| Хэсэг | Төлөв |
|---|---|
| TLS cert pinning | ✅ Нэмэгдсэн (SEC-1) — өмнө нь байхгүй байсан |
| Hardened Runtime | ✅ On — library validation идэвхтэй (DYLD injection ихэвчлэн блок) |
| Code signature / notarize | ✅ Developer ID + notarized + stapled |
| App Sandbox | ❌ Байхгүй (network.client + smartcard entitlement) |
| Secret хадгалалт | ✅ Keychain (bearer + device_secret) + HMAC-SHA256 |
| Anti-debug / runtime integrity | ✅ Нэмэгдсэн (SEC-2) |
| Root / SIP signal | ✅ Нэмэгдсэн (SEC-3) |
| String obfuscation | ✅ Pin/requirement/dylib нэр XOR (SEC-3) + class нэр (obfuscate_build.sh) |

## SEC-1 — TLS certificate pinning (P0)

- `Infrastructure/Security/Pinning/SPKIHash.swift` — cert-ийн SubjectPublicKeyInfo
  SHA-256 (base64). EC P-256/P-384 + RSA-2048/4096 ASN.1 prefix. (iOS-оос порт.)
- `Infrastructure/Security/Pinning/PinnedSessionDelegate.swift` — `URLSessionDelegate`,
  chain-walk, leaf эсвэл issuer pin тааруулна. **DEBUG-д bypass** (dev loop).
- Pin: **Let's Encrypt E7 + E8 intermediate** (leaf renew (~90 хоног)-д тэсвэртэй).
  Intermediate pin тул `api.eidmongol.mn` ба `ca.gerege.mn` хоёуланд ажиллана.
- `BackendHttpClient` + `Core/Network/APIClient` хоёрын `URLSession`-д delegate залгасан.

Pin шинэчлэх (cert ротац): `openssl s_client -connect host:443 -showcerts` → завсрын
сертификатын SPKI SHA-256-г `PinnedSessionDelegate.pinnedHashes`-д нэмнэ. iOS-тэй
**ижил pin багц** баримтлана.

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
- Pin ротац: leaf биш **intermediate** pin тул LE-ийн E-цувралын ротацид л шинэчилнэ.
