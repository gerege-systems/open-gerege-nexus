# Gerege Nexus — native клиентүүд

Гурван клиент, нэг бүтээгдэхүүн: **macOS** (`desktop/macos`), **iOS/iPadOS**
(`mobile/ios`), **Android** (`mobile/android`). Гурвуулаа Gerege Nexus
байрлуулалтын иргэн рүү харсан клиент — вэб бүрхүүл БИШ, native апп.

## Нэр нь шугамаа дагана

Аппын нэр нь ПЛАТФОРМЫГ биш, ТӨХӨӨРӨМЖИЙН ШУГАМЫГ нэрлэнэ — хаягуудтайгаа
яг ижил дүрэм (`shared/device_lines.json`):

| Шугам | Хаяг | Аппын нэр | Юу нь энэ нэрийг барих вэ |
|---|---|---|---|
| desktop | `desktop.nexus.gerege.mn` | **NexusGeregeDesktop** | Xcode target/scheme, .NET solution ба namespace |
| mobile | `mobile.nexus.gerege.mn` | **NexusGeregeMobile** | Xcode target/scheme, Gradle `rootProject.name` |
| kiosk | `kiosk.nexus.gerege.mn` | **NexusGeregeKiosk** | ЗАХИАЛГАТАЙ — клиент хараахан байхгүй |
| pos | `pos.nexus.gerege.mn` | **NexusGeregePos** | ЗАХИАЛГАТАЙ — клиент хараахан байхгүй |

Ширээний macOS ба Windows хоёр НЭГ нэртэй байгаа нь алдаа биш: хүн тэр
хоёртой ижил байдлаар харьцдаг тул тэд нэг шугам, нэг апп. Иргэний харах нэр
нь эдгээрийн аль нь ч биш — **Gerege Nexus** (`PRODUCT_NAME`, `DisplayName`).

## Гурван зарчим (гурвуулан дээр ижил)

**1. Клиентэд secret байхгүй.** Бүх дуудлага өөрийн web backend-ийн нийтийн
`/api/…` route-уудаар явна — хөтөчтэй яг ижил зам. RP-ийн нууцыг зөвхөн web
сервер барина. Тиймээс аппыг задалж үзсэн хүнд авах юм алга.

**2. Identity нь snapshot, session биш.** Bearer token байхгүй: нэвтрэлтийн
үр дүн (`documentNumber`, нэр, иргэний дугаар) нь дараагийн үйлдлийн бариул
бөгөөд Keychain (Apple) / Android Keystore-оор шифрлэгдэж хадгалагдана. Апп
сүлжээгүй ч нээгдэнэ — зөвхөн шинэ өгөгдөл татагдахгүй.

**3. Нэр МОНГОЛООР.** Гэрчилгээний subject дэх нэр нь латин галиг
(«ERDENEBAT TSENDDORJ») тул дэлгэцэнд харагдах нэрийг `POST /api/dashboard`
(XYP-ийн хураангуй) -аас авна. Гурван клиент энэ дүрмийг нэвтрэлтийн урсгал
дотроо хэрэгжүүлдэг — эхний кадраасаа зөв нэр.

## Нэвтрэлт — платформ бүрт өөр, session нь ижил

| Клиент | Хэрхэн | Яагаад |
|---|---|---|
| macOS | QR + РД push | Зөвшөөрөгч нь ХӨРШ утас |
| iOS / Android | app-to-app (`geregesmartid://approve?sessionId=…`), fallback РД push | Зөвшөөрөгч нь ӨӨРӨӨ тэр утас — QR-аа өөрөө скан хийж чадахгүй |

Гурвуулан ижил `POST /api/start` → `GET /api/status` poll дээр суудаг: session
нь хэн зөвшөөрснөөс үл хамааран ижил тул app-to-app-д НЭГ Ч шинэ backend
endpoint нэмээгүй. eID Mongolia апп суугаагүй бол РД push зам үлдэнэ (утас
дээрх схемийг асуухын тулд iOS `LSApplicationQueriesSchemes`, Android
`<queries>` блокт бүртгэсэн байх ЁСТОЙ — эс бөгөөс OS «суугаагүй» гэж худал
хэлнэ).

## Код хуваалцах — хуулбар биш

```
desktop/macos/            macOS апп + ХУВААЛЦСАН давхаргууд
  Core/Network            AppConfig, Endpoints, APIClient
  Core/Keychain           identity snapshot
  App/AppState.swift      төлөв, локал лог
  Design/                 ШИРЭЭНИЙ өнгө, фонт, дүрсүүд (Windows-той нийцтэй)
  Presentation/           локализаци (7 хэл)
  Core/Token, Core/Esign   ← ЗӨВХӨН ширээний (PKCS#11, ws гүүр)
mobile/ios/               iOS апп — дээрхийг `project.yml`-ээр ШУУД эх файлаар нь оруулна
  Design/                 ← УТАСНЫ дизайны токенууд + дүрсүүд (доор үзнэ үү)
mobile/android/           Android апп — өнгө, орчуулгыг ҮҮСГЭНЭ (scripts/gen_from_swift.py)
  ui/theme/Gw.kt          ← УТАСНЫ дизайны токенууд (үүсгэгддэггүй)
```

iOS нь ширээний файлуудыг хуулдаггүй, ШУУД эх файлаар нь компайл хийдэг тул
endpoint, орчуулга хоёр дээр салбарлах боломжгүй. Android өөр хэл дээр
тул тэр замыг явж чадахгүй — оронд нь `scripts/gen_from_swift.py` нь
`Design/Colors.swift` → `EidColors.kt`, локализацийн каталог →
`res/values*/eid_strings.xml` болгож үүсгэнэ. CI нь скриптийг дахин ажиллуулж
ялгаа гарвал улаан болно.

## Харагдац — ширээ, утас хоёр ӨӨР

Ширээний `Design/` нь Windows аппын `Colors.xaml`/`Typography.xaml`-ийн порт:
тэр хоёр клиент нэг л зүйл харагдах ёстой. Утас нь **Gerege Wallet-ийн
дизайны системээс** гаралтай — иргэн Gerege-гийн хоёр аппыг зэрэг барьдаг тул
гар дээрх хоёр нь нэг гэр бүл байх ёстой.

| | Токен | Дүрсүүд | Фонт |
|---|---|---|---|
| macOS, Windows | `desktop/macos/Design/Colors.swift` | `Design/Styles.swift` | системийн |
| iOS | `mobile/ios/Design/Theme.swift` | `mobile/ios/Design/BrandComponents.swift` | Montserrat |
| Android | `mobile/android/.../ui/theme/Gw.kt` | `.../ui/components/Components.kt` | Montserrat |

Утасны хоёр файл нь ХАРИЛЦАН ПОРТ: токены нэр (`bg`, `surface1..3`, `fg1..4`,
`brand`/`brandSoft`/`brandLine`, `credit`/`debit`/`accent`/`gold`), геометр
(52 оролтын мөр, 56 CTA, 14 радиус), фонтын хэмжээс гурвуулан 1:1. Нэг талд
утга солиход нөгөөд нь механик — орчуулга биш, хуулбар.

Ширээний `Design/` нь iOS target-д ОРСООР байна (`AppCard`, `StatusPill`,
`VerificationCodeRow` … нь macOS-ынх); гар дээрх дэлгэцүүд түүнийг уншихаа
больсон. Android талд `EidColors.kt` ба `gen_from_swift.py` гинж ХЭВЭЭР —
ширээ↔Android нийцлийн CI шалгалт утасны харагдацаас хамаарахгүй.

Montserrat нь `mobile/ios/Resources/Fonts/` (Info.plist `UIAppFonts`) ба
`mobile/android/app/src/main/res/font/`-д — ЯГ ижил дөрвөн .ttf. Тоо (регистр,
баримтын дугаар, баталгаажуулах код) нь monospace хэвээр: Montserrat-д tabular
figure байхгүй тул баганаар эгнэхгүй.

## Барих

```bash
# macOS (Xcode 16+, xcodegen)
cd desktop/macos && ./build.sh

# iOS/iPadOS
cd mobile/ios && ./build.sh
DESTINATION='generic/platform=iOS' ./build.sh     # төхөөрөмжид

# Android (ANDROID_HOME эсвэл local.properties шаардлагатай)
cd mobile/android && ./gradlew assembleDebug
python3 scripts/gen_from_swift.py                 # өнгө/орчуулгыг дахин үүсгэх
```

`.xcodeproj` нь артефакт (`.gitignore`) — `project.yml`-ийг засаж `build.sh`
ажиллуулна. CI: `.github/workflows/native-clients.yml` гурвууланг компайл хийнэ.

## Төхөөрөмжийн шугам

| Клиент | Шугам | Төлөв |
|---|---|---|
| macOS | `desktop.nexus.gerege.mn` | ⏳ nginx/TLS шалгах |
| iOS, Android | `mobile.nexus.gerege.mn` | ⏳ nginx/TLS шалгах |

Бүртгэл ба асаах дараалал: [`shared/device_lines.json`](shared/device_lines.json)
→ `$provisioning`. Клиентийн доторх хаягийг ХАМГИЙН СҮҮЛД солино — эсрэгээр
явбал апп байхгүй host руу чиглэж унана.

## Хараахан хийгээгүй (утсан дээр)

Гарын үсэг (PDF), гэрчилгээ шалгах, платформын webview таб, биометр түгжээ,
push мэдэгдэл. Ширээн дээр эдгээр бий; гар дээр суурь урсгал (нэвтрэлт,
самбар, ID, лог, тохиргоо) эхэлж ирлээ.
