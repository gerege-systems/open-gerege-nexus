# e-ID Mongolia — Windows Desktop Client

C# / WinUI 3 desktop client for the Mongolian e-ID platform — feature-parity with the web frontend (`web/`).
Internal assembly / project IDs use the `NexusGeregeDesktop.*` prefix; display name and MSIX identity use **e-ID Mongolia**.

> **⚠️ Port note (read `BACKEND-INTEGRATION.md` first).** This app was ported from
> the `gerege` Windows client, which was an **RP-holding, direct-backend** client
> (HMAC device secret + Bearer + `/web2app/v1` / `/rp/v1`). On this platform the
> desktop trust model is the **opposite** — a **first-party client** with no RP/HMAC
> secret and no bearer, talking to the web backend's public `/api/*` routes like a
> browser (CLAUDE.md §desktop). The config + HTTP-pipeline have been adapted
> (first-party, `BaseUrl` → web origin); the per-service `/api/*` rewire is fully
> specified in [`BACKEND-INTEGRATION.md`](BACKEND-INTEGRATION.md) and must be
> finished + compiled on a **Windows** toolchain (macOS cannot build WinUI/MSIX).
> Several sections below still describe the original `/rp/v1` + HMAC model — treat
> them as reference for the legacy contract, not the current wiring.

## Stack

- **.NET 8 (LTS)** + **C# 12**
- **WinUI 3** via Windows App SDK 2.0
- **MVVM** with CommunityToolkit.Mvvm (source-generated properties + commands)
- **Generic Host** (`Microsoft.Extensions.Hosting`) with DI, Configuration, Options
- **Logging:** Serilog → Console + Debug + rolling file (`%LOCALAPPDATA%/NexusGeregeDesktop/logs/`)
- **HTTP:** `HttpClient` + `Microsoft.Extensions.Http.Resilience` + custom HMAC + cert-pinning handlers (M1)
- **Validation:** FluentValidation + DataAnnotations
- **Tests:** xUnit + FluentAssertions + NSubstitute
- **Packaging:** Single-project MSIX, EV code-signing, `.appinstaller` auto-update (M8)

## Solution layout (Clean Architecture)

```
desktop-app/windows-app/
├── src/
│   ├── NexusGeregeDesktop.Domain/          # Entities, value objects, errors. No external deps.
│   ├── NexusGeregeDesktop.Application/     # Use-case services, options, contracts. References Domain.
│   ├── NexusGeregeDesktop.Infrastructure/  # HttpClient, cert pinning, DPAPI vault, FCM, etc. Windows TFM.
│   ├── NexusGeregeDesktop.Presentation/    # ViewModels, navigation, validation. UI-agnostic.
│   └── NexusGeregeDesktop.Client/          # WinUI 3 entry point: App.xaml, MainWindow, pages.
└── tests/
    └── NexusGeregeDesktop.UnitTests/       # xUnit, references all src projects.
```

**Reference graph:** `Client → Presentation, Infrastructure, Application, Domain`
- `Presentation → Application, Domain`
- `Infrastructure → Application, Domain`
- `Application → Domain`

## Build

```powershell
cd desktop-app/windows-app
dotnet restore
dotnet build NexusGeregeDesktop.sln -c Debug /p:Platform=x64
dotnet test
```

Open `NexusGeregeDesktop.sln` in **Visual Studio 2022 17.10+** with the *Windows application development* workload installed. Set `NexusGeregeDesktop.Client` as startup project. Press F5.

## Configuration

`src/NexusGeregeDesktop.Client/appsettings.json` is the canonical config. Environment overlay via `appsettings.{ENV}.json` (e.g. `Development`). Environment-variable overrides via the prefix `EIDMNG__` (double-underscore as section separator):

```powershell
$env:EIDMNG__NexusGeregeDesktop__Backend__BaseUrl = "https://staging.eidmongolia.mn"
```

All settings are bound to typed `NexusGeregeDesktopOptions` with DataAnnotations validation at startup.

### Backend connection (RP subsystem-ID header)

The `/rp/v1/*` routes on `api.eidmongolia.mn` require an `X-RP-Client` header (subsystem-ID naming convention; we are a standalone backend, not a federation participant — the legacy `X-Road-Client` header name is still accepted during migration). In production, RP clients set this header themselves. For local desktop development, set the test RP identifier directly:

```jsonc
// appsettings.Development.json
"NexusGeregeDesktop": {
  "Backend": {
    "BaseUrl": "https://api.eidmongolia.mn",
    "RpClient": "MN/COM/5544334/eid-test"
  }
}
```

`eid-test` is the seeded development RP (`backend/migrations/0004_seeds.sql`) with allowed scopes `auth` + `sign` and `0.0.0.0/0` IP allowlist. Real deployments register their own subsystem.

## Security defaults

| Concern | Mechanism |
|---|---|
| TLS | TLS 1.3 only when pinning is on (M1) |
| Cert pinning | SPKI-SHA256 over the whole chain (M1) — see below |
| Auth | HMAC-SHA256 (mobile parity) (M1) |
| Secrets | Windows `PasswordVault` (DPAPI) (M1) |
| Biometric | Windows Hello (`UserConsentVerifier`) for sensitive ops (M4) |
| Packaging | MSIX + EV code-signing, no `runFullTrust` (M8) |
| Logging | Serilog with PII redaction enricher (M7) |

### TLS certificate pinning

`appsettings.json` ships with `RequireCertificatePinning: true` and pins the two
**ISRG (Let's Encrypt) roots** that `eidmongolia.mn` chains to:

| Pin | Certificate |
|---|---|
| `diGVwiVYbubAI3RW4hB9xU8e/CH2GnkuvVFZE8zmgzI=` | ISRG Root X2 — current ECDSA chain (`eidmongolia.mn` → `YE2` → `Root YE` → X2) |
| `C5+lpZ7tcVwmwQIMcRtPbsQtWLABXhQzejna0wHFr8M=` | ISRG Root X1 — backup, used if the RSA chain is served |

`SpkiPinValidator` matches **any** element of the presented chain, so roots are
pinned rather than the leaf: Let's Encrypt leaves rotate every 60–90 days and a
leaf pin would break the app on every renewal. `appsettings.Development.json`
turns pinning off so local builds can talk to `http://localhost:3000`.

Recompute a pin after a CA change:

```bash
openssl s_client -connect eidmongolia.mn:443 -servername eidmongolia.mn -showcerts </dev/null \
  | openssl x509 -pubkey -noout | openssl pkey -pubin -outform der \
  | openssl dgst -sha256 -binary | openssl base64
```

A wrong pin blocks **every** backend call, so change these only together with a
verified handshake against the live host.

## Packaging (MSIX)

See [`tools/README.md`](tools/README.md) for the full pipeline. Short version:

```powershell
# Generate a dev signing cert (once per machine, run as admin)
.\tools\dev-cert\New-DevCert.ps1

# Build + sign an MSIX into artifacts/
.\tools\pack-msix.ps1 -Version 0.1.0.0 -Platform x64

# Install locally
Add-AppPackage -Path .\artifacts\NexusGeregeDesktop*.msix
```

Production builds use an EV / OV code-signing cert with the matching `Publisher` DN; the `.appinstaller` template wires up Windows-native auto-update.
