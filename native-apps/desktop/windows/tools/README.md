# Packaging & code-signing — e-ID Mongolia desktop

This folder holds the build tooling for producing a signed MSIX installer
and the matching `.appinstaller` manifest for Windows-native auto-update.

## Files

| File | Purpose |
|---|---|
| `dev-cert/New-DevCert.ps1` | One-time per developer. Generates a self-signed code-signing cert (`NexusGeregeDesktop.Dev.pfx`) so MSIX builds install locally without publisher trust prompts. |
| `pack-msix.ps1` | `dotnet publish -p:EidMsix=true` → locate produced `.msix` → `signtool sign /tr <timestamp>` → copy to `artifacts/`. |
| `NexusGeregeDesktop.appinstaller.template.xml` | Auto-update manifest template; placeholders `{VERSION}`, `{APPINSTALLER_URL}`, `{MSIX_URL}`, `{PUBLISHER}`. |
| `publish-appinstaller.ps1` | Renders the template into `artifacts/NexusGeregeDesktop.appinstaller`, ready to upload alongside the `.msix`. |

## Local dev flow (sideload signed by self-signed cert)

```powershell
# Step 1 — once per machine. The script imports the public cert into
# LocalMachine\TrustedPeople (needs admin) so the MSIX installs cleanly.
.\tools\dev-cert\New-DevCert.ps1

# Step 2 — every build.
.\tools\pack-msix.ps1 -Version 0.1.0.0 -Platform x64

# Output: artifacts/NexusGeregeDesktop.{version}_x64.msix
# Install:
Add-AppPackage -Path .\artifacts\NexusGeregeDesktop*.msix
# Or via Explorer: double-click the .msix.
```

## Production flow

Production must use an **EV (Extended Validation) or OV code-signing
certificate** from a CA in the Microsoft Trusted Root Program (DigiCert,
GlobalSign, Sectigo, ...). EV provides immediate SmartScreen reputation;
OV requires reputation accrual.

The Publisher DN on the certificate **must match** `<Identity Publisher=...>`
in `Package.appxmanifest`. Plan the legal name change before ordering the
cert — DN mismatch invalidates the package.

Pipeline:

```powershell
.\tools\pack-msix.ps1 `
    -Configuration Release -Platform x64 `
    -Version 1.0.0.0 `
    -PfxPath C:\secrets\eidmongol-codesign.pfx `
    -PfxPassword (ConvertTo-SecureString $env:CODESIGN_PWD -AsPlainText -Force) `
    -TimestampUrl http://timestamp.digicert.com
```

In CI the PFX comes from an Azure Key Vault / GitHub-encrypted-secret. **Never
commit the PFX**, even encrypted — `.gitignore` already excludes
`tools/dev-cert/*.pfx` and `tools/dev-cert/*.cer`.

## Auto-update

1. Upload `NexusGeregeDesktop.X.Y.Z.msix` to your CDN.
2. Run `publish-appinstaller.ps1` with the new version + URL and upload the
   resulting `.appinstaller` file alongside.
3. Existing installations check the `.appinstaller` URL on launch
   (configurable interval; default 6h) and install the new MSIX silently
   on next exit.

The first installation is done via the `.appinstaller` URL (Windows opens it
in the App Installer UI). Subsequent updates happen automatically.

## Manifest version policy

- The version in `Package.appxmanifest` **`<Identity Version="x.y.z.0">`** is
  the source of truth.
- `-Version` on `pack-msix.ps1` overrides it for that build only.
- Use semver mapping: `MAJOR.MINOR.PATCH.0`. The 4th part stays 0 unless
  doing a hotfix where you can't bump PATCH (rare).
- Auto-update only triggers when the server-side version is **strictly
  greater** than the installed version.
