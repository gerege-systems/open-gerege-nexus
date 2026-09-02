# Төхөөрөмжийн шугамыг form factor-оор бүлэглэх

**Огноо:** 2026-09-02
**Төлөв:** батлагдсан дизайн, хэрэгжүүлэлт хүлээж байна

## Одоогийн байдал

Шугам (device line) гэдэг нь **платформ** гэсэн үг. Зургаан хост, тус бүр нэг
платформ:

| хост | платформ | form factor |
|---|---|---|
| `mac.nexus.gerege.mn` | macos | desktop |
| `win.nexus.gerege.mn` | windows | desktop |
| `ios.nexus.gerege.mn` | ios | mobile |
| `android.nexus.gerege.mn` | android | mobile |
| `kiosk.nexus.gerege.mn` | kiosk | kiosk |
| `pos.nexus.gerege.mn` | pos | pos |

Backend цор ганц: зургуулаа ижил upstream руу (3008 frontend, 8082 API) очдог.
Тусдаа хост байгаагийн шалтгаан нь webview доторх дуудлагыг same-origin
болгох, session cookie-г host-only байлгах хоёр.

`frontend/lib/deviceLine.ts` нь шугамыг **host-ын хамгийн зүүн шошгоор**
тодорхойлно (`mac` → macos). `frontend/proxy.ts` нь `/`-ыг
`/line/<platform>` руу rewrite хийж, `frontend/app/line/[line]/lines.ts`
доторх зургаан өөр нүүр дэлгэцийн нэгийг үзүүлнэ.

Хавтасны зохион байгуулалт нь мөн платформоор:
`native-apps/{macos, windows, ios, android}`.

## Юу өөрчлөгдөх вэ

**Шугам ≡ платформ** байсныг **шугам ≡ form factor** болгоно.

Энэ бол нэршлийн засвар биш, ойлголтын засвар: хаяг нь тухайн төхөөрөмжтэй
хүн хэрхэн харьцаж байгааг нэрлэнэ, ямар үйлдлийн систем ажиллаж байгааг биш.
Ширээн дээрх Mac ба ширээн дээрх Windows хоёр нэг шугам; гарт байгаа iPhone ба
гарт байгаа Android нэг шугам.

`window.GeregeShell.platform` — IPC гэрээний талбар — **хөндөгдөхгүй**. Native
бүрхүүл өөрийгөө macOS гэж хэлсээр байх бөгөөд web тал түүнийг уншиж чадсаар
байна. Зөвхөн *шугам* гэдэг ойлголт зургаагаас дөрөв болно.

### Дөрвөн шугам

```
desktop.nexus.gerege.mn   ← macOS, Windows
mobile.nexus.gerege.mn    ← iOS, Android (таблет ч мөн)
kiosk.nexus.gerege.mn     ← Windows Kiosk, Android Kiosk
pos.nexus.gerege.mn       ← Windows POS, Android POS
```

`kiosk` ба `pos` нь desktop/mobile дотор ОРОХГҮЙ. Нэг Windows машин дээр
ажлын ширээний клиент ба киоск хоёр зэрэг ажиллаж болох тул тэдгээрийн
хооронд host-only cookie-гийн тусгаарлалт хэрэгтэй хэвээр. macOS ба Windows
хоёр нэг машин дээр хэзээ ч зэрэг ажиллахгүй тул тэдний хооронд тусгаарлалт
хэрэггүй — тийм ч учраас нэгтгэж болж байна.

### Хуучин дөрвөн хаяг

`mac.` `win.` `ios.` `android.` нь `server_name`-ээс **хасагдана**. Native
клиентийн албан ёсны release хараахан гараагүй тул талбарт суулгагдсан клиент
байхгүй; `000-catch-all.conf` нь нэрлэгдээгүй хүсэлтийг 404-өөр барина.
Хуучин dev build нь чимээгүй буруу ажиллахын оронд тодорхой унана.

## Хэрэгжүүлэлт

### 1. Хавтас

```
native-apps/
├── desktop/
│   ├── macos/
│   └── windows/
├── mobile/
│   ├── ios/
│   └── android/
├── shared/
└── generated-i18n/
```

`kiosk`, `pos` нь **өөрийн кодгүй**: Windows-ийн `FormFactor` build
(`ShellProfile.cs` доторх `Kiosk`/`Pos`/`Desktop` профайл), Android-ийн flavor
(`BuildConfig.FORM_FACTOR`). Тиймээс хавтасны бүлэглэл нь дөрвөн кодын сангийн
тухай бөгөөд kiosk/pos нь variant хэвээр үлдэнэ. `desktop/windows` доторх
киоск build нь `kiosk.` шугамд үйлчилнэ — код нь Windows, шугам нь киоск.
Энэ хоёрыг хольж ойлгохоос сэргийлж `native-apps/README.md` дээр тодорхой
бичнэ.

Замыг нэрлэсэн бүх газар хамт хөдөлнө: `makefile`, `.gitignore`,
`.github/workflows/native-clients.yml`, `.github/workflows/native-release.yml`,
`frontend/scripts/export-native-auth-i18n.mjs`, `native-apps/shared/device_lines.json`,
`deploy/scripts/setup_device_lines.sh`, `native-apps/README.md`.

### 2. Frontend

| файл | өөрчлөлт |
|---|---|
| `lib/shell.ts` | `ShellLine = "desktop" \| "mobile" \| "kiosk" \| "pos"` нэмнэ. `ShellPlatform` зургаан утгаараа ҮЛДЭНЭ — IPC гэрээ. |
| `lib/deviceLine.ts` | `LINES_BY_LABEL` дөрвөн бичлэг; `DeviceLine.platform` → `DeviceLine.line`; `lineHomePath` → `/line/<line>`. |
| `proxy.ts` | `x-gerege-device-line` толгойн утга нь шугамын нэр болно. |
| `app/line/[line]/lines.ts` | Зургаан бичлэг дөрөв болно. |

`lines.ts`-ийн нэгтгэл нь агуулгын шийдвэр:

* **desktop** — `macos` ба `windows` хоёрын нэгдэл. Хоёулаа `posture: "desk"`,
  үйлдлийн жагсаалт нь бараг ижил (App store, SSO clients, төхөөрөмжийн парк).
  Ялгаатай хоёр мөр (`Холбогч` / `Хандах эрх`) хоёулаа үлдэнэ.
  Гарчиг: «Ажлын ширээ». Хайлш: одоогийн macos-ийн `#6B7A99`.
* **mobile** — `ios` ба `android` хоёрын нэгдэл, `posture: "hand"`.
  Гарчиг: «Гарын алганд». Хайлш: `#0E9AA7`.
* **kiosk**, **pos** — хэвээр, юу ч өөрчлөгдөхгүй.

Хоёр хайлшийн өнгө (`#2F6FED` win, `#2E9E5B` android) хэрэглээнээс гарна.

### 3. Native клиент

Клиент бүрд нэг тогтмол мөр:

| файл | шинэ утга |
|---|---|
| `desktop/macos/NativeSettings.swift` → `lineOrigin` | `https://desktop.nexus.gerege.mn` |
| `desktop/windows/ShellProfile.cs` → `Desktop.LineOrigin` | `https://desktop.nexus.gerege.mn` |
| `mobile/ios/Sources/GeregeShellKit/DeviceLine.swift` → `origin` | `https://mobile.nexus.gerege.mn` |
| `mobile/android/.../DeviceLine.kt` → `origin` (else салаа) | `https://mobile.nexus.gerege.mn` |

`ShellProfile.cs`-ийн `Kiosk`/`Pos`, `DeviceLine.kt`-ийн `kiosk`/`pos` салаанууд
хөдлөхгүй.

**Шилжилтийн дэгээ аль хэдийн байна.** Клиент бүр «ажиллахаа больсон хуучин
анхдагчууд» гэсэн жагсаалттай (`superseded` / `supersededOrigins`), хадгалсан
утга тэр жагсаалтад байвал шинэ анхдагч руу өөрөө шилждэг. Хуучин
платформын хаягуудыг тэдгээрт нэмнэ — тэгвэл dev машин дээрх хуучин суулгац
өөрөө зөв шугам руу зөөгдөнө. macOS дээр энэ нь `NativeSettings`-ийн decoder
доторх if-chain.

### 4. Байрлуулалт

`device_lines.json` өөрөө дарааллаа бичсэн: **DNS → nginx server_name →
certbot → `DEVICE_LINE_ORIGINS` → хамгийн сүүлд клиентийн тогтмол.** Эсрэг
дараалал нь аппыг байхгүй хост руу чиглүүлж унагаана.

1. **DNS** — `*.nexus.gerege.mn` wildcard нь `desktop.`, `mobile.`-ыг аль
   хэдийн хамарна. Шинэ бичлэг хэрэггүй.
2. **nginx** — `deploy/nginx/device-lines.nexus.gerege.mn.conf` дотор
   `server_name` дөрвөн нэр болно.
3. **certbot** — гэрчилгээг дөрвөн нэрээр дахин гаргана
   (`desktop`, `mobile` шинэ; `kiosk`, `pos` хэвээр).
4. **API** — `docker-compose.prod.yml` ба `deploy/.env.prod.example` доторх
   `DEVICE_LINE_ORIGINS` анхдагч.
5. **Клиент** — дээрх дөрвөн тогтмол.

`deploy/scripts/setup_device_lines.sh` доторх `LINES` массив дөрөв болно; тэр
скрипт DNS-ийг эхлээд шалгадаг тул дарааллыг өөрөө хамгаална.

### 5. Баримт

* `native-apps/shared/device_lines.json` — цор ганц эх сурвалж. `deviceLines`
  түлхүүрүүд нь платформ биш шугамын нэр болно; `formFactor` талбар нь
  түлхүүртэйгээ давхарлах тул хасагдана. `client` талбар нь шугам бүрд нэгээс
  олон файл нэрлэнэ.
* `docs/SHELL_CONTRACT.md` — шугамын жагсаалт ба `platform` vs `line` хоёрын
  ялгаа.
* `native-apps/README.md` — шинэ мод, kiosk/pos нь variant гэдэг тайлбар.
* `frontend/lib/deviceLine.ts`-ийн толгойн тайлбар.

## Тест

`deviceLine`-д өнөөдөр **тест байхгүй**. Нэг жижиг unit тест нэмнэ:

* `deviceLineFromHost("desktop.nexus.gerege.mn")` → `desktop` шугам
* `deviceLineFromHost("mobile.nexus.staging.gerege.mn")` → `mobile`
  (шошгоор таарах нь орчноос хамаарахгүйг батална)
* `deviceLineFromHost("mac.nexus.gerege.mn")` → `null` (хуучин нэр үхсэн)
* `deviceLineFromHost("nexus.gerege.mn")` → `null` (хөтчийн шугам)

Native талд CI-ийн `macos`, `android`, `windows` job-ууд хавтасны шинэ замыг
шалгана. Клиентийн тогтмолыг ажиллуулж шалгах тест байхгүй — тэр нь
`device_lines.json`-той гараар тааруулах зүйл хэвээр.

## Юу ЗОРИУДААР өөрчлөгдөхгүй вэ

* `window.GeregeShell.platform` ба `ShellPlatform`-ийн зургаан утга — IPC
  гэрээ. Native бүрхүүл өөрийгөө юу гэж хэлэхийг энэ ажил хөндөхгүй.
* Хөтчийн шугам `nexus.gerege.mn`.
* `kiosk`, `pos` шугамын хаяг, дэлгэц, клиентийн тогтмол.
* Backend-ийн код. Энэ бүхэн nginx, frontend, native тогтмол гурав дээр
  дуусна; API-д зөвхөн `DEVICE_LINE_ORIGINS`-ийн жагсаалт хүрнэ.
* Android-ийн `tablet` flavor — тэр нь `mobile.` шугам дээр ажиллана,
  `ShellFormFactor` дотор `tablet` утга хэвээр үлдэнэ.

## Эрсдэл

* **Хуучин хаягтай dev build.** Шууд унана. `superseded` жагсаалт нь
  хадгалсан тохиргоог зөөх боловч кодод шатсан анхдагчийг зөөхгүй — хуучин
  binary-г дахин барих шаардлагатай. Release гараагүй тул хамрах хүрээ нь
  хөгжүүлэгчийн машинууд.
* **Гэрчилгээ.** Дөрвөн нэрээр дахин гаргах хооронд шугамууд богино хугацаанд
  унана. Certbot-ыг nginx reload-ын дараа шууд ажиллуулна.
* **`mac`/`win` дэлгэцийн ялгаа алдагдана.** Санаатай: хоёр ширээний дэлгэц
  үгээрээ л ялгаатай байсан. Ялгаа дахин хэрэгтэй болбол `GeregeShell.platform`
  клиент талд байгаа тул нэг дэлгэц дотроос салаалж болно — хаягийг буцааж
  хуваах шаардлагагүй.
