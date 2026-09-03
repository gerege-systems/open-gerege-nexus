<#
.SYNOPSIS
  Generate a self-signed code-signing certificate for local MSIX builds.

.DESCRIPTION
  The certificate is used ONLY to sign development MSIX packages so they can
  be sideloaded on the developer's own machine. It is NOT trusted by anyone
  else. Production builds MUST use an EV / OV code-signing certificate from a
  CA in the Microsoft Trusted Root Program (e.g. DigiCert, Sectigo).

  Output: tools/dev-cert/NexusGeregeDesktop.Dev.pfx, password-protected.

  The Subject MUST match the <Identity Publisher="..."> in Package.appxmanifest
  exactly. Currently: CN=Gerege Systems LLC, O=Gerege Systems LLC, C=MN.

.NOTES
  Run once per developer. The .pfx + .cer are gitignored - never commit them.
  Re-run if you delete tools/dev-cert.

.PARAMETER Password
  Password to encrypt the .pfx file. Use the same value in pack-msix.ps1.
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $false)]
    [SecureString]$Password = (Read-Host -Prompt "PFX password" -AsSecureString),
    [string]$Subject = "CN=Gerege Systems LLC, O=Gerege Systems LLC, C=MN",
    [string]$OutDir = $PSScriptRoot,
    [int]$ValidYears = 5
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path $OutDir)) { New-Item -ItemType Directory -Path $OutDir | Out-Null }

$pfxPath = Join-Path $OutDir "NexusGeregeDesktop.Dev.pfx"
$cerPath = Join-Path $OutDir "NexusGeregeDesktop.Dev.cer"

if (Test-Path $pfxPath) {
    Write-Warning "$pfxPath already exists. Delete it first if you want to regenerate."
    exit 1
}

Write-Host "Generating self-signed code-signing cert (Subject=$Subject, $ValidYears years)..."

$cert = New-SelfSignedCertificate `
    -Subject $Subject `
    -KeyUsage DigitalSignature `
    -FriendlyName "NexusGeregeDesktop Dev" `
    -CertStoreLocation "Cert:\CurrentUser\My" `
    -KeyExportPolicy Exportable `
    -KeyAlgorithm RSA `
    -KeyLength 3072 `
    -HashAlgorithm SHA256 `
    -TextExtension @("2.5.29.37={text}1.3.6.1.5.5.7.3.3", "2.5.29.19={text}") `
    -NotAfter (Get-Date).AddYears($ValidYears)

Export-PfxCertificate -Cert $cert -FilePath $pfxPath -Password $Password | Out-Null
Export-Certificate -Cert $cert -FilePath $cerPath | Out-Null

# Trust the certificate locally so the sideloaded MSIX installs without UAC prompts.
$trustedRoot = "Cert:\LocalMachine\TrustedPeople"
try {
    Import-Certificate -FilePath $cerPath -CertStoreLocation $trustedRoot -ErrorAction Stop | Out-Null
    Write-Host "Imported public cert into LocalMachine\TrustedPeople (needs admin)."
} catch {
    Write-Warning "Could not import to TrustedPeople ($($_.Exception.Message)). Run this script as admin OR manually run: certutil -addstore -f TrustedPeople $cerPath"
}

Remove-Item "Cert:\CurrentUser\My\$($cert.Thumbprint)" -Force -ErrorAction SilentlyContinue

Write-Host ""
Write-Host "Done. Files:"
Write-Host "  $pfxPath"
Write-Host "  $cerPath"
Write-Host ""
Write-Host "Add the password to your local secret store; pack-msix.ps1 will prompt for it."
