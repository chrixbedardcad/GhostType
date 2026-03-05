# GhostType installer for Windows.
#
# Usage (PowerShell):
#   irm https://raw.githubusercontent.com/chrixbedardcad/GhostType/main/scripts/install.ps1 | iex
#
# What it does:
#   1. Downloads the latest GhostType release from GitHub
#   2. Installs both variants (console + windowless) to %LOCALAPPDATA%\GhostType\
#   3. Adds the install directory to your user PATH
#

$ErrorActionPreference = "Stop"
$Repo = "chrixbedardcad/GhostType"
$InstallDir = Join-Path $env:LOCALAPPDATA "GhostType"

function Write-Info  { param($Msg) Write-Host $Msg -ForegroundColor Cyan }
function Write-Ok    { param($Msg) Write-Host $Msg -ForegroundColor Green }
function Write-Warn  { param($Msg) Write-Host $Msg -ForegroundColor Yellow }

# --- Resolve latest release -------------------------------------------------

Write-Info "Fetching latest GhostType version..."
$Release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
$Version = $Release.tag_name
Write-Info "Latest version: $Version"

# --- Download binaries ------------------------------------------------------

if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

$Assets = @(
    @{ Name = "ghosttype.exe";            File = "ghosttype-windows-amd64.exe" },
    @{ Name = "ghosttype-windowless.exe"; File = "ghosttype-windows-amd64-windowless.exe" }
)

foreach ($Asset in $Assets) {
    $Url = "https://github.com/$Repo/releases/download/$Version/$($Asset.File)"
    $Dest = Join-Path $InstallDir $Asset.Name
    Write-Info "Downloading $($Asset.File)..."
    Invoke-WebRequest -Uri $Url -OutFile $Dest -UseBasicParsing
}

# --- Add to PATH ------------------------------------------------------------

$UserPath = [Environment]::GetEnvironmentVariable("PATH", "User")
if ($UserPath -notlike "*$InstallDir*") {
    Write-Info "Adding $InstallDir to user PATH..."
    [Environment]::SetEnvironmentVariable("PATH", "$UserPath;$InstallDir", "User")
    $env:PATH = "$env:PATH;$InstallDir"
    Write-Warn "Restart your terminal for PATH changes to take effect."
}

# --- Done -------------------------------------------------------------------

Write-Ok ""
Write-Ok "GhostType $Version installed to $InstallDir"
Write-Ok ""
Write-Info "Two variants installed:"
Write-Host "  ghosttype.exe             - with console window (useful for debugging)"
Write-Host "  ghosttype-windowless.exe  - tray only, no console (recommended)"
Write-Host ""
Write-Info "To launch:"
Write-Host "  ghosttype-windowless"
Write-Host ""
Write-Info "Config is stored in: $env:APPDATA\GhostType\"
