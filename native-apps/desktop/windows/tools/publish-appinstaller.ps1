<#
.SYNOPSIS
  Render the .appinstaller template with concrete URLs + version, ready to upload alongside the .msix.

.EXAMPLE
  .\publish-appinstaller.ps1 `
    -Version 0.1.0.0 `
    -AppInstallerUrl https://eidmongolia.mn/download/NexusGeregeDesktop.appinstaller `
    -MsixUrl https://eidmongolia.mn/download/eid-mongolia-setup.msix `
    -Publisher "CN=Gerege Systems LLC, O=Gerege Systems LLC, C=MN"
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory)] [string]$Version,
    [Parameter(Mandatory)] [string]$AppInstallerUrl,
    [Parameter(Mandatory)] [string]$MsixUrl,
    [Parameter(Mandatory)] [string]$Publisher,
    [string]$OutPath
)

$ErrorActionPreference = "Stop"
if (-not $OutPath) {
    $OutPath = Join-Path (Resolve-Path (Join-Path $PSScriptRoot "..")) "artifacts\NexusGeregeDesktop.appinstaller"
}
$template = Get-Content -Raw -Path (Join-Path $PSScriptRoot "NexusGeregeDesktop.appinstaller.template.xml")
$rendered = $template `
    -replace '\{VERSION\}', $Version `
    -replace '\{APPINSTALLER_URL\}', $AppInstallerUrl `
    -replace '\{MSIX_URL\}', $MsixUrl `
    -replace '\{PUBLISHER\}', $Publisher

New-Item -ItemType Directory -Path (Split-Path $OutPath) -Force | Out-Null
Set-Content -Path $OutPath -Value $rendered -Encoding UTF8
Write-Host "Wrote $OutPath"
