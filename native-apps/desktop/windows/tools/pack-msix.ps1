<#
.SYNOPSIS
  Build and sign a self-contained MSIX of e-ID Mongolia desktop client.

.DESCRIPTION
  Usage:
    .\pack-msix.ps1 -Configuration Release -Platform x64 -Version 0.1.0.0

  The script:
    1. Runs `dotnet publish` with EidMsix=true so the WinAppSDK single-project
       MSIX tooling emits a packaged build.
    2. Locates the produced .msix under bin/<arch>/<config>/.../AppPackages/.
    3. Signs it with tools/dev-cert/NexusGeregeDesktop.Dev.pfx (or a custom -PfxPath).
    4. Copies the signed file to artifacts/eid-mongolia-setup.msix
       (stable name — referenced by the web download link) AND
       artifacts/NexusGeregeDesktop.{version}.msix (versioned archive copy).

  For PRODUCTION signing use:
    -PfxPath C:\secrets\eidmongol-codesign.pfx -TimestampUrl http://timestamp.digicert.com
  with a Microsoft-trusted EV / OV code-signing certificate.

.PARAMETER Configuration
  Release (default) or Debug. Release strongly recommended.

.PARAMETER Platform
  x64 (default), x86, or ARM64.

.PARAMETER Version
  4-part MSIX version (1.2.3.4). Must be >= current Package.appxmanifest version
  for users on auto-update to receive it.

.PARAMETER PfxPath
  Path to the .pfx code-signing certificate. Default: tools/dev-cert/NexusGeregeDesktop.Dev.pfx

.PARAMETER PfxPassword
  Cert password. Prompted if not supplied.

.PARAMETER TimestampUrl
  RFC 3161 timestamp authority. Recommended for production so the signature
  stays verifiable after the cert expires.
#>
[CmdletBinding()]
param(
    [ValidateSet("Debug", "Release")]
    [string]$Configuration = "Release",
    [ValidateSet("x64", "x86", "ARM64")]
    [string]$Platform = "x64",
    [string]$Version,
    [string]$PfxPath,
    [SecureString]$PfxPassword,
    [string]$TimestampUrl = "http://timestamp.digicert.com"
)

$ErrorActionPreference = "Stop"

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
$clientCsproj = Join-Path $repoRoot "src\NexusGeregeDesktop.Client\NexusGeregeDesktop.Client.csproj"
$artifacts = Join-Path $repoRoot "artifacts"
if (-not $PfxPath) {
    $PfxPath = Join-Path $PSScriptRoot "dev-cert\NexusGeregeDesktop.Dev.pfx"
}

if (-not (Test-Path $clientCsproj)) {
    throw "Client csproj not found at $clientCsproj"
}
if (-not (Test-Path $PfxPath)) {
    throw "Code-signing PFX not found at $PfxPath. Run tools\dev-cert\New-DevCert.ps1 first, or pass -PfxPath."
}

New-Item -ItemType Directory -Path $artifacts -Force | Out-Null

# --- 1. Build packaged MSIX ---------------------------------------------------
$buildArgs = @(
    "publish", $clientCsproj,
    "-c", $Configuration,
    "-p:Platform=$Platform",
    "-p:RuntimeIdentifier=win-$($Platform.ToLowerInvariant())",
    "-p:EidMsix=true",
    "-p:GenerateAppxPackageOnBuild=true",
    "-p:AppxBundle=Never",
    "-p:AppxPackageSigningEnabled=false",  # we sign ourselves below to control cert + timestamp
    "--nologo"
)
if ($Version) {
    $buildArgs += "-p:Version=$Version"
    $buildArgs += "-p:AppxPackageVersion=$Version"
}

Write-Host "==> dotnet $($buildArgs -join ' ')"
& dotnet @buildArgs
if ($LASTEXITCODE -ne 0) { throw "dotnet publish failed (exit $LASTEXITCODE)" }

# --- 2. Locate produced MSIX -------------------------------------------------
# WinAppSDK single-project MSIX writes to src\NexusGeregeDesktop.Client\AppPackages\
# <Name>_<Version>_<Platform>_Test\<Name>_<Version>_<Platform>.msix
$searchRoot = Join-Path $repoRoot "src\NexusGeregeDesktop.Client\AppPackages"
$candidates = Get-ChildItem -Path $searchRoot -Filter "*.msix" -Recurse -ErrorAction SilentlyContinue |
    Sort-Object LastWriteTime -Descending

if (-not $candidates -or $candidates.Count -eq 0) {
    throw "No .msix found under $searchRoot. Check the publish output."
}
$msix = $candidates[0]
Write-Host "==> Found: $($msix.FullName)"

# --- 3. Sign -----------------------------------------------------------------
$signtoolPath = $null
$signtool = Get-Command "signtool.exe" -ErrorAction SilentlyContinue
if ($signtool) {
    $signtoolPath = $signtool.Source
} else {
    # Try the latest Windows SDK install location.
    $sdk = Get-ChildItem "${env:ProgramFiles(x86)}\Windows Kits\10\bin\10.0.*\$Platform\signtool.exe" -ErrorAction SilentlyContinue | Sort-Object FullName -Descending | Select-Object -First 1
    if ($sdk) { $signtoolPath = $sdk.FullName }
}
if (-not $signtoolPath) {
    # Fallback: signtool from the Microsoft.Windows.SDK.BuildTools NuGet package
    # (restored transitively by the WinAppSDK build) — no system Windows SDK needed.
    $nugetRoot = if ($env:NUGET_PACKAGES) { $env:NUGET_PACKAGES } else { Join-Path $env:USERPROFILE ".nuget\packages" }
    $nugetSt = Get-ChildItem (Join-Path $nugetRoot "microsoft.windows.sdk.buildtools\*\bin\*\$Platform\signtool.exe") -ErrorAction SilentlyContinue | Sort-Object FullName -Descending | Select-Object -First 1
    if ($nugetSt) { $signtoolPath = $nugetSt.FullName }
}

if (-not $signtoolPath) {
    Write-Warning "signtool.exe not found. Skipping signing - the MSIX will be UNSIGNED."
    Write-Warning "Install the Windows SDK to enable signing: https://developer.microsoft.com/windows/downloads/windows-sdk/"
    $signed = $false
} else {
    if (-not $PfxPassword) {
        $PfxPassword = Read-Host -Prompt "PFX password" -AsSecureString
    }
    $plain = [System.Runtime.InteropServices.Marshal]::PtrToStringAuto(
        [System.Runtime.InteropServices.Marshal]::SecureStringToBSTR($PfxPassword))

    Write-Host "==> Signing $($msix.Name) with $PfxPath"
    & $signtoolPath sign /fd SHA256 /f $PfxPath /p $plain /tr $TimestampUrl /td SHA256 $msix.FullName
    if ($LASTEXITCODE -ne 0) { throw "signtool sign failed (exit $LASTEXITCODE)" }
    $signed = $true
}

# --- 4. Copy to artifacts ----------------------------------------------------
# We emit two filenames:
#   * eid-mongolia-setup.msix — stable, what the web download link / GitHub
#     Release "latest" URL points to.
#   * <Name>_<Version>_<Platform>.msix — versioned archive copy (audit trail).
$stableDest = Join-Path $artifacts "eid-mongolia-setup.msix"
$versionedDest = Join-Path $artifacts $msix.Name
Copy-Item $msix.FullName $stableDest -Force
Copy-Item $msix.FullName $versionedDest -Force
Write-Host ""
if ($signed) {
    Write-Host "Signed MSIX -> $stableDest"
    Write-Host "             -> $versionedDest"
} else {
    Write-Host "UNSIGNED MSIX -> $stableDest"
    Write-Host "              -> $versionedDest"
    Write-Host "To install an unsigned MSIX, enable Developer Mode in Windows Settings, then:"
}
Write-Host ("Install with: Add-AppPackage -Path '" + $stableDest + "'")
