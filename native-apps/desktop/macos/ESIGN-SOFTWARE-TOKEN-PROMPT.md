# macOS ESIGN "Программ токен" — Claude session-д өгөх PROMPT

> Доорх prompt-ыг macOS дээрх Claude Code session-д бүтнээр хуулж өг. Windows
> хувилбар `main` дээр аль хэдийн хийгдсэн бөгөөд лавлагаа болно.

---

```
macOS апп (desktop/macos-app, Swift/SwiftUI) дээр "Программ токен"-оор
төрийн сайтад тоон гарын үсгээр нэвтрэх феатур хийх. Физик USB токенгүйгээр,
апп-д нэвтэрсэн иргэний identity-ээр (гар утасны threshold ECDSA) ДАН/ESIGN
нэвтрэлт хийнэ.

## Лавлагаа (main дээр аль хэдийн байгаа — Windows хувилбар)
Windows-app дээр энэ бүхэн хийгдсэн. ЯГ ижил логикийг Swift рүү буулга:
- desktop/windows/src/NexusGeregeDesktop.Infrastructure/Esign/  ← гол лавлагаа:
    • EsignWebSocketServer.cs — ws://127.0.0.1:59001 + wss://127.0.0.1:59005 (RFC6455)
    • EsignCrypto.cs          — ENCRYPTED_DATA_SIGN схемийн ЯГ хэрэгжүүлэлт
    • EsignBridge.cs          — токен/программ токен салаалалт (fallback)
    • EsignSoftwareToken.cs   — backend руу cert/sign дуудлага (программ токен)
    • EsignCertParser.cs, EsignBridgeHostedService.cs
    • README.md
- desktop/windows-app/ESIGN-SOFTWARE-TOKEN-BACKEND.md — backend контрактын баримт (ЗААВАЛ УНШ)
- desktop/windows/src/NexusGeregeDesktop.Infrastructure/Auth/RpAuthService.cs
    — sign-pdf start→poll→/api/status{signatureValueB64} загвар

## ESIGN протокол (type "bb4702f31917793f")
Төрийн сайтын ДАН нэвтрэлт локал ws руу ENCRYPTED_DATA_SIGN хүсэлт илгээнэ.
Хариу (токен эсвэл программ токен өгнө):
  P = JSON { "data": <data объектын indented JSON string>,
             "certificate": <b64 userCert DER>, "sn": <serial>, "keyID": <b64 CKA_ID/SKI> }
  signature = P дээрх гарын үсэг (физик=RSA-SHA256; программ=ECDSA-SHA256, hash-then-sign)
  K=random 16B (AES-128!), IV=random 16B
  cipher = AES-128-CBC/PKCS7(P)
  meta   = RSA-PKCS1(IV‖K)  серверийн cert-ээр (32 байт)
Хариу: { status:"success", signature:b64, cipher:b64, meta:b64 }.
⚠️ AES-128 (256 БИШ); P.data нь data объектын JSON STRING; signature нь P-г бүхэлд нь.

## Хийх зүйл (macOS)
1. Локал ESIGN ws/wss сервер (127.0.0.1:59001/59005) — DAN хуудас холбогдоно.
2. ENCRYPTED_DATA_SIGN боловсруулах: P угсрах → гарын үсэг → cipher/meta → буцаах.
3. Салаалалт: физик USB токен байвал (gerege-token-kit ашиглаж) түүгээр;
   БАЙХГҮЙ бол ПРОГРАММ ТОКЕН.
4. Программ токен: нэвтэрсэн иргэний
     • гэрчилгээ:  POST /api/certificates {personId} → { signing, auth } (b64 DER) — signing-ийг эхэлж
     • гарын үсэг: POST /api/esign-sign {personId, digestB64: SHA256(P), key:"signing", displayText}
                   → { sessionId, pollToken, vc } → VC-г UI-д харуул →
                   GET /api/status?sessionId=&pollToken= → { state, endResult, signatureValueB64 }
   (first-party загвар: bearer биш, personId биетэд; identity нь session-ээс.)
5. Backend эдгээр /api/* endpoint хараахан БАЙХГҮЙ (404) — Windows шиг
   "esign_backend_not_available" маягаар цэвэрхэн уналт өг; UI бэлэн байг.

## macOS-т анхаарах
- Swift/SwiftUI; USB токен нь desktop/gerege-token-kit (SPM).
- ws server-т Network.framework (NWListener) ашиглаж болно; TLS-т localhost self-signed.
- Endpoints нь desktop/macos-app/Core/Network/Endpoints.swift дотор.
- Крипто: AES-CBC-д CommonCrypto/Security (CryptoKit-д CBC байхгүй);
  RSA encrypt-д SecKeyCreateEncryptedData(.rsaEncryptionPKCS1).

## Тест
mitmproxy ProxyOverride=<-loopback> ашиглаж isf.mn (ДАН) урсгал хянаж болно.
Backend endpoint ирээгүй тул одоохондоо программ токен зам "not available"-аар
уналт өгнө — гол нь ws server + физик токен зам + программ токен клиент бэлэн байх.
```
