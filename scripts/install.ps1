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

# Kill any running GhostType before overwriting the binaries.
Get-Process -Name "ghosttype*" -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
Start-Sleep -Seconds 1

if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

$Assets = @(
    @{ Name = "ghosttype.exe";        File = "ghosttype-windows-amd64.exe" },
    @{ Name = "ghosttype-window.exe"; File = "ghosttype-windows-amd64-window.exe" }
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
}
# Always refresh current session PATH so the command works immediately.
if ($env:PATH -notlike "*$InstallDir*") {
    $env:PATH = "$env:PATH;$InstallDir"
}

# --- Refresh icon cache -----------------------------------------------------

try { Start-Process -FilePath "ie4uinit.exe" -ArgumentList "-show" -NoNewWindow -Wait -ErrorAction SilentlyContinue } catch { }

# --- Start Menu shortcut ----------------------------------------------------

$StartMenu = Join-Path $env:APPDATA "Microsoft\Windows\Start Menu\Programs"
$ShortcutPath = Join-Path $StartMenu "GhostType.lnk"
$ExePath = Join-Path $InstallDir "ghosttype.exe"

Write-Info "Creating Start Menu shortcut..."
$WshShell = New-Object -ComObject WScript.Shell
$Shortcut = $WshShell.CreateShortcut($ShortcutPath)
$Shortcut.TargetPath = $ExePath
$Shortcut.WorkingDirectory = $InstallDir
$Shortcut.IconLocation = "$ExePath,0"
$Shortcut.Description = "GhostType - AI-powered text correction"
$Shortcut.Save()

# --- Done -------------------------------------------------------------------

Write-Ok ""
Write-Ok "GhostType $Version installed to $InstallDir"
Write-Ok ""
Write-Info "Two variants installed:"
Write-Host "  ghosttype.exe        - tray only, no console (recommended)"
Write-Host "  ghosttype-window.exe - with console window (useful for debugging)"
Write-Host ""
Write-Info "Config is stored in: $env:APPDATA\GhostType\"
Write-Host ""

# --- Auto-launch ------------------------------------------------------------

Write-Info "Launching GhostType..."
Start-Process -FilePath $ExePath
Write-Ok "GhostType is running in your system tray (bottom-right, near the clock)."
Write-Host "  Look for the GhostType icon — click the ^ arrow if it's hidden."
Write-Host ""
Write-Info "To launch manually later:"
Write-Host "  Search 'GhostType' in the Start menu, or type 'ghosttype' in a terminal."
