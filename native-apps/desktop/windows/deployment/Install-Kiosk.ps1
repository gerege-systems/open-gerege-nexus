param([string]$ConfigurationPath = "$PSScriptRoot\AssignedAccess.xml")
$ErrorActionPreference = 'Stop'
if (-not ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) { throw 'Run as Administrator' }
$xml = Get-Content -Raw -LiteralPath $ConfigurationPath
$namespace = 'root\cimv2\mdm\dmmap'
$instance = Get-CimInstance -Namespace $namespace -ClassName MDM_AssignedAccess
$instance.Configuration = [System.Net.WebUtility]::HtmlEncode($xml)
Set-CimInstance -CimInstance $instance | Out-Null
Write-Host 'Gerege Nexus Assigned Access policy applied. Restart Windows to enter kiosk mode.'
