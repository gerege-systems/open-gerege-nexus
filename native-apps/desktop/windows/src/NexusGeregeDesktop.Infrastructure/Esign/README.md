# ESIGN локал гүүр — тоон гарын үсгээр төрийн сайтад нэвтрэх

Төрийн ДАН (sso.gov.mn) зэрэг вэбсайтын **"Тоон гарын үсгээр нэвтрэх"** сонголт
`ws://127.0.0.1:59001` (эсвэл `wss://…:59005`) руу холбогдож, локал клиентээр токеноор
challenge-д гарын үсэг зуруулдаг. Энэ модуль тэрхүү клиентийг (ESIGNClient-ийн эквивалент)
eID платформын байгаа токен дэд бүтэц дээр хэрэгжүүлнэ.

## Урсгал
```
Browser (төрийн сайт) → ws://127.0.0.1:59001 → EsignBridgeHostedService
   → гэрчилгээ сонгох диалог (олон бол) → PIN → токеноор гарын үсэг
   → {signature, cipher, meta} → browser → sso.gov.mn → нэвтэрнэ
```

## Бүрэлдэхүүн
| Файл | Үүрэг |
|---|---|
| `EsignBridgeHostedService` | Апп эхлэхэд ws:59001 + wss:59005 серверийг ажиллуулна |
| `EsignWebSocketServer` | Түүхий RFC6455 ws/wss сервер |
| `EsignBridge` | Хүсэлт → токен/гэрчилгээ (ITokenRegistry) → сонголт → PIN → гарын үсэг |
| `EsignCrypto` | ENCRYPTED_DATA_SIGN схем (P/AES-128/RSA-wrap) |
| `IEsignInteraction` (Application) | Гэрчилгээ сонгох + PIN асуух UI callback (Client хэрэгжүүлнэ) |

## Криптограф (ESIGNClient.exe эх кодоос батлагдсан)
```
P = JSON { "data": <хүсэлтийн data объектийн JSON string>,
           "certificate": <b64 userCert DER>, "sn": <токен серийн>, "keyID": <b64 CKA_ID> }
signature = RSA-SHA256-PKCS1( P )        (P-г бүхэлд нь sign, ott биш!)
cipher    = AES-128-CBC/PKCS7( P )        (AES-128 — CKA_VALUE_LEN=16)
meta      = RSA-PKCS1( IV[16] || K[16] )  серверийн cert-ээр (32 байт)
```

## Бүртгэл (DI)
`AppHost`-д:
```csharp
builder.Services.AddSingleton<IEsignInteraction, EsignInteraction>();  // Client (WinUI)
builder.Services.AddEsignBridge();                                     // Infrastructure
```
Порт солих: `AddEsignBridge(o => o.WsPort = 59001)`.

## Тэмдэглэл
- Токены гарын үсэг байгаа `ITokenSession.SignAsync(certId, P, Sha256WithRsa)`-аар (CKM_SHA256_RSA_PKCS).
- Олон applet/гэрчилгээг `ListObjectsAsync` + `ReadCertificateAsync`-аар уншиж сонгуулна.
- Хувь хүний РД нь гэрчилгээний `SERIALNUMBER`-аас.
- 59001 портыг ESIGNClient эзэлдэг тул хамт ажиллуулбал портыг өөрчилнө.
