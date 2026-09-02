# Gerege Nexus — pure native clients

This directory (`native-apps/`) contains the native client codebases: **iOS/iPadOS** (`GeregeShellKit` SPM + SwiftUI/WKWebView), **windows** (C#/.NET 8 WPF + WebView2), **android** (Kotlin/Compose/WebView), and **macOS** (AppKit/WKWebView).

Each client develops natively from here on. Where a screen is still web, it is
embedded as one of the app's own screens rather than as a second window.

---

## 🧱 Two rules that govern every client

**1. One frame.** Each client has exactly one window (macOS/Windows) or one
scene (iOS/Android). Login, the work area, and settings are *screens* that swap
inside that frame — never separate windows. The only thing allowed out of the
frame is a popup: `NSAlert`/`NSMenu`/`NSSavePanel`, `MessageBox` and file
dialogs, `alert`/`confirmationDialog`/share sheets, `BiometricPrompt` and
permission dialogs. `window.open` and `target="_blank"` from the work area do
not open a second webview; the shell hands the URL to the system browser.

Swapping to a native screen hides the webview, it does not remove it. Removing
it rebuilds the webview and the person loses their page, their scroll position
and anything half-typed.

**2. One backend, one domain line per device.** The backend is single. Each
client talks to its own host, and that host serves `/api/v1` too, so calls from
inside the webview are same-origin — the session cookie stays `SameSite=Strict`
and no CORS preflight is ever issued.

| Client | Line | Status |
| --- | --- | --- |
| Browser / PWA | `nexus.gerege.mn` | ✅ live |
| macOS, Windows Desktop | `desktop.nexus.gerege.mn` | ✅ live |
| iOS / iPadOS, Android mobile / tablet | `mobile.nexus.gerege.mn` | ✅ live |
| Kiosk (Windows, Android) | `kiosk.nexus.gerege.mn` | ✅ live |
| POS (Windows, Android) | `pos.nexus.gerege.mn` | ✅ live |

The line names the **form factor**, not the platform: a Mac and a Windows box
on the same desk are one line, because a person uses them the same way. Which
client is running is a separate question, answered by `window.GeregeShell`.

All four are covered by a `*.nexus.gerege.mn` wildcard A record and share one
Let's Encrypt certificate. The per-platform names `mac.` / `win.` / `ios.` /
`android.` were retired on 2026-09-02.

**When adding a NEW line, do not point a client at it before it resolves.** The
app fails with `A server with the specified hostname could not be found` and
nobody can sign in — this happened once. Do the DNS/nginx/TLS/CORS work first
and change the client's origin constant last. The order and the exact line to
edit are in [`shared/device_lines.json`](shared/device_lines.json)
under `$provisioning`.

Adding a line is a three-file change: [`shared/device_lines.json`](shared/device_lines.json),
[`../frontend/lib/deviceLine.ts`](../frontend/lib/deviceLine.ts), and the deploy
side (`DEVICE_LINE_ORIGINS` plus
[`../deploy/nginx/device-lines.nexus.gerege.mn.conf`](../deploy/nginx/device-lines.nexus.gerege.mn.conf)).

Both rules are specified in [`../docs/SHELL_CONTRACT.md`](../docs/SHELL_CONTRACT.md) §1a and §1b.

---

## 📁 Architecture Overview

```
native-apps/
├── desktop/                     # Ширээний шугам — desktop.nexus.gerege.mn
│   ├── macos/                   # macOS Native Shell (Swift 5.10 + AppKit + WKWebView)
│   │   ├── main.swift           # NSApplication Entry Point
│   │   ├── AppDelegate.swift    # App Lifecycle & Native Menu Bar
│   │   ├── MainWindowController.swift    # The single window: ribbon, rail, pane host, footer
│   │   ├── SettingsPaneViewController.swift # Settings as an in-frame NSView, not a window
│   │   ├── NativeIPC.swift      # WKScriptMessageHandler Native IPC Bridge
│   │   └── build.sh             # Swiftc Compilation Script
│   └── windows/                 # Windows Native Shell (C# .NET 8 + WPF + WebView2)
│       ├── GeregeNexusWin.csproj # .NET 8 Project File
│       ├── App.xaml / App.xaml.cs # WPF Application Lifecycle
│       ├── MainWindow.xaml / MainWindow.xaml.cs # The single window: menu, rail, pane host, footer
│       ├── SettingsPane.xaml / SettingsPane.xaml.cs # Settings as an in-frame UserControl
│       ├── ShellProfile.cs      # Desktop / Kiosk / POS profiles — each names its own line
│       └── NativeIPCBridge.cs   # CoreWebView2.WebMessageReceived IPC Bridge
│
├── mobile/                      # Гарын шугам — mobile.nexus.gerege.mn
│   ├── ios/                     # iOS/iPadOS app + shared Swift package
│   │   ├── GeregeNexusIOS.xcodeproj # Xcode app project
│   │   ├── Package.swift        # GeregeShellKit, GeregeShellUI, GeregeNexusApp
│   │   ├── Sources/             # Native login/settings and WKWebView shell
│   │   └── Tests/               # Swift auth state-machine tests
│   └── android/                 # Android mobile/tablet/kiosk/POS clients
│       ├── core/                # Shared auth/device behavior
│       └── app/                 # Four form-factor flavors
│
├── generated-i18n/              # `npm run i18n:export-native`-ийн гаралт
│
└── shared/                      # Shared Specifications & Configurations
    ├── app_config.json          # Window sizing & platform notes
    ├── device_lines.json        # Canonical line → origin map (one backend behind all)
    └── IPC_CONTRACT.md          # Bi-directional JSON IPC Message Contract Specification
```

**Хавтас нь кодын сан, шугам нь хаяг — хоёр нь нэг зүйл биш.** `kiosk` ба
`pos` шугам өөрийн кодгүй: тэдгээр нь Windows-ийн `FormFactor` build ба
Android-ийн flavor. Тиймээс `desktop/windows` доторх киоск build нь `kiosk.`
шугамд үйлчилнэ — код нь Windows, шугам нь киоск. Шугамын бүрэн бүртгэл
`shared/device_lines.json` дотор.

---

## 🚀 Building & Running

### IDE-ээр шууд нээх

- iOS/iPadOS — Xcode-д `ios/GeregeNexusIOS.xcodeproj`-ийг нээгээд `GeregeNexusIOS` scheme-ийг ажиллуулна. `project.yml` нь XcodeGen-ээр төслийг дахин үүсгэх эх файл.
- macOS — Xcode-д `macos/GeregeNexusNativeMac.xcodeproj`-ийг нээгээд `GeregeNexusNativeMac` scheme-ийг ажиллуулна.
- Android — Android Studio-д `android/` хавтсыг нээнэ. Энд `.xcodeproj` эсвэл
  `.sln` шиг тусдаа project файл БАЙХГҮЙ нь зөв: Gradle төслийн хувьд
  `settings.gradle.kts` бүхий хавтас нь өөрөө төсөл. Wrapper нь repository-д
  орсон тул Gradle тусад нь суулгах шаардлагагүй.
- Windows — Visual Studio 2022-д `windows/GeregeNexusNativeWin.sln`-ийг нээнэ. Solution дотор WPF app болон `GeregeShell.Core` хоёулаа байна.

### 1. macOS Native Shell (Swift + AppKit)

**Prerequisites**: macOS 12+, Xcode Command Line Tools (`swiftc`, `xcrun`)

```bash
# Build the native macOS executable
cd native-apps/desktop/macos
./build.sh

# Run the native macOS application
./GeregeNexusNativeMac
```

### 2. iOS/iPadOS app (SwiftUI + WKWebView)

```bash
cd native-apps/mobile/ios
xcodebuild -project GeregeNexusIOS.xcodeproj -scheme GeregeNexusIOS \
  -destination 'generic/platform=iOS Simulator' CODE_SIGNING_ALLOWED=NO build
```

### 3. Windows Native Shell (C# + WPF + WebView2)

**Prerequisites**: Windows 10/11, .NET 8 SDK

```powershell
# Build and run on Windows
cd native-apps/desktop/windows
dotnet build -p:FormFactor=Desktop
dotnet build -p:FormFactor=Kiosk
dotnet build -p:FormFactor=POS
```

### 4. Android native clients (Kotlin + Compose)

Android Studio-д `native-apps/mobile/android`-ыг нээнэ. Нэг app module дөрвөн
form-factor flavor-тай: `mobile`, `tablet`, `kiosk`, `pos`; auth state machine
нь `:core` модульд байна.

**Prerequisites**: Android SDK. Android Studio-гаар нээхэд `local.properties`-ыг
өөрөө үүсгэдэг тул IDE дотор юу ч хийх шаардлагагүй. Харин **командын мөрнөөс**
барихад тэр файл (эсвэл `ANDROID_HOME`) заавал хэрэгтэй — эс бөгөөс Gradle
`SDK location not found` гэж шууд унана. `local.properties` нь машин бүрт өөр
зам агуулдаг тул git-д ороогүй.

```bash
export ANDROID_HOME="$HOME/Library/Android/sdk"   # эсвэл local.properties бичих
./gradlew :core:test
./gradlew :app:assembleMobileDebug
./gradlew :app:assembleTabletDebug :app:assembleKioskDebug :app:assemblePosDebug
```

---

## ⚡ Native Features & Principles Preserved

1. **Native Login + Web Work Area**: Password and eID push are native controls. On success the shell copies `session_token` into the webview cookie store and opens the start route; web `/login` is never rendered in a native client, and the device lines redirect it away server-side.
2. **Native Menu Bar**:
   - macOS Top Menu Bar (`Gerege Nexus`, `Удирдах`, `Харах`) with native shortcuts (`⌘L`, `⌘,`, `⌘0`, `⌘R`, `⌘Q`).
   - Windows Native Menu Bar (`Gerege Nexus`, `Удирдах`, `Харах`) with shortcuts (`Ctrl+L`, `Ctrl+,`, `F5`).
3. **In-frame navigation**: a native rail (desktop) or tab bar (mobile) switches between the work area and the shell's own screens. It is deliberately *not* a copy of the tenant app menu — the work area draws that itself, and duplicating it would split one menu across two states.
4. **Bridge Contract v1.4**:
   - `window.GeregeShell` is injected at document start, main-frame only.
   - `auth.reLogin` returns to native login; unknown methods reject.
   - `shell.openPane` lets the work area move to a shell-owned screen without opening anything.
   - [`../docs/SHELL_CONTRACT.md`](../docs/SHELL_CONTRACT.md) defines the shared state machine.

## Deployment ба update суваг

- macOS: notarized app bundle + Sparkle feed; signing/notarization identity нь
  release environment-ийн secret байна.
- iOS/iPadOS: TestFlight → phased App Store rollout, APNs entitlement/profile.
- Windows: Desktop/Kiosk/POS тусдаа MSIX identity; Assigned Access template нь
  [`windows/deployment`](windows/deployment)-д байна.
- Android: Play managed publishing эсвэл EMM/private APK channel; kiosk нь
  Android Enterprise device-owner + Lock Task ашиглана.

Signing certificate, Apple team, Play service account, payment/vendor SDK нь
repository-д хадгалагдахгүй. CI нь бүх unsigned compile target-ийг шалгана;
release job нь deployment secret байгаа үед гарын үсэг зурна.
