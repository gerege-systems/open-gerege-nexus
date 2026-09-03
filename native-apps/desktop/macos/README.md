# e-ID Mongolia macOS Desktop

macOS (SwiftUI) desktop app — **first-party** клиент. iOS app-тай ижил зарчмаар
клиентэд ямар ч RP secret/бүртгэл байхгүй: browser-тэй яг ижил, өөрийн web
backend-ийн нийтийн `/api/*` route-уудыг дууддаг (RP-API-ийн secret зөвхөн web
серверийн орчинд байдаг).

## Урсгал

| Үйлдэл | Web route |
|---|---|
| QR нэвтрэлт | `POST /api/start` → QR = sessionId + `vc` код |
| РД push нэвтрэлт | `POST /api/login-notify {register}` |
| Auth poll | `GET /api/status?sessionId=&pollToken=` — web нь гарын үсгийг баталгаажуулж нэр + иргэний дугаар + documentNumber буцаана |
| PDF гарын үсэг | `POST /api/sign-pdf-start {etsi, digestB64, fileName}` — утас руу PIN2 push |
| Sign poll | `GET /api/status?sessionId=&pollToken=` (auth-тай ижил зам) |
| Тамгалагдсан PDF | `POST /api/sign-pdf-download` (multipart `file` + `sessionId` + `pollToken`) |

## Build

```bash
xcodegen generate
xcodebuild -project NexusGeregeDesktop.xcodeproj -scheme NexusGeregeDesktop \
           -configuration Debug -destination 'platform=macOS,arch=arm64' build
```

Шаардлага: Xcode 16+, macOS 14+, `brew install xcodegen`.
`../gerege-token-kit` (local SPM) автоматаар resolve хийгдэнэ.

## Локал туршилт

```bash
cd ../../server && SMARTID_RP_API_SECRET= go run ./cmd/smartid   # Go API :8080
cd ../../web && npm run dev                                      # web :3000
```

DEBUG build-ийн default сервер — `http://localhost:3000` (web). Өөр хаяг руу
заахдаа Settings → Сервер (эсвэл env `API_BASE_URL`).

Дэлгэрэнгүй: [CLAUDE.md](CLAUDE.md), нууцлалын хатуужилт: [SECURITY-HARDENING.md](SECURITY-HARDENING.md).
