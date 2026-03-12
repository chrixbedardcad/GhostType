# GhostSpell installer for Windows.
#
# Usage (PowerShell):
#   irm https://raw.githubusercontent.com/chrixbedardcad/GhostSpell/main/scripts/install.ps1 | iex
#
# What it does:
#   1. Downloads the latest GhostSpell release from GitHub
#   2. Installs ghostspell.exe to %LOCALAPPDATA%\GhostSpell\
#   3. Adds the install directory to your user PATH
#

$ErrorActionPreference = "Stop"
$Repo = "chrixbedardcad/GhostSpell"
$InstallDir = Join-Path $env:LOCALAPPDATA "GhostSpell"

function Write-Info  { param($Msg) Write-Host $Msg -ForegroundColor Cyan }
function Write-Ok    { param($Msg) Write-Host $Msg -ForegroundColor Green }
function Write-Warn  { param($Msg) Write-Host $Msg -ForegroundColor Yellow }

# --- Resolve latest release -------------------------------------------------

Write-Info "Fetching latest GhostSpell version..."
$Release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
$Version = $Release.tag_name
Write-Info "Latest version: $Version"

# --- Download binaries ------------------------------------------------------

# Kill any running GhostSpell before overwriting the binaries.
Get-Process -Name "ghostspell*" -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
Start-Sleep -Seconds 1

if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

# Clean up old console variant from previous versions.
$OldWindow = Join-Path $InstallDir "ghostspell-window.exe"
if (Test-Path $OldWindow) {
    Remove-Item -Force $OldWindow -ErrorAction SilentlyContinue
    Write-Info "Removed old ghostspell-window.exe"
}

$Url = "https://github.com/$Repo/releases/download/$Version/ghostspell-windows-amd64.exe"
$Dest = Join-Path $InstallDir "ghostspell.exe"
Write-Info "Downloading ghostspell-windows-amd64.exe..."
Invoke-WebRequest -Uri $Url -OutFile $Dest -UseBasicParsing

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
$ShortcutPath = Join-Path $StartMenu "GhostSpell.lnk"
$ExePath = Join-Path $InstallDir "ghostspell.exe"

Write-Info "Creating Start Menu shortcut..."
$WshShell = New-Object -ComObject WScript.Shell
$Shortcut = $WshShell.CreateShortcut($ShortcutPath)
$Shortcut.TargetPath = $ExePath
$Shortcut.WorkingDirectory = $InstallDir
$Shortcut.IconLocation = "$ExePath,0"
$Shortcut.Description = "GhostSpell - AI-powered text correction"
$Shortcut.Save()

# --- Startup shortcut (auto-start on login) ---------------------------------

$StartupDir = Join-Path $env:APPDATA "Microsoft\Windows\Start Menu\Programs\Startup"
$StartupShortcut = Join-Path $StartupDir "GhostSpell.lnk"

Write-Info "Adding GhostSpell to Windows startup..."
$WshStartup = New-Object -ComObject WScript.Shell
$Startup = $WshStartup.CreateShortcut($StartupShortcut)
$Startup.TargetPath = $ExePath
$Startup.WorkingDirectory = $InstallDir
$Startup.Description = "GhostSpell - AI-powered text correction"
$Startup.Save()

# --- Done -------------------------------------------------------------------

Write-Ok ""
Write-Ok "GhostSpell $Version installed to $InstallDir"
Write-Ok ""
Write-Info "Config is stored in: $env:APPDATA\GhostSpell\"
Write-Host ""

# --- Auto-launch ------------------------------------------------------------

Write-Info "Launching GhostSpell..."
Start-Process -FilePath $ExePath
Write-Ok "GhostSpell is running in your system tray (bottom-right, near the clock)."
Write-Host "  Look for the GhostSpell icon — click the ^ arrow if it's hidden."
Write-Host ""
Write-Info "To launch manually later:"
Write-Host "  Search 'GhostSpell' in the Start menu, or type 'ghostspell' in a terminal."
