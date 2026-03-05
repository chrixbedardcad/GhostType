# GhostType uninstaller for Windows.
#
# Usage (from CMD, PowerShell, or Windows Terminal):
#   powershell -c "irm https://raw.githubusercontent.com/chrixbedardcad/GhostType/main/scripts/uninstall.ps1 | iex"
#
# What it does:
#   1. Stops GhostType if it's running
#   2. Removes the app binaries from %LOCALAPPDATA%\GhostType\
#   3. Removes config and logs from %APPDATA%\GhostType\
#   4. Removes the install directory from user PATH
#

$ErrorActionPreference = "Stop"
$InstallDir = Join-Path $env:LOCALAPPDATA "GhostType"
$DataDir = Join-Path $env:APPDATA "GhostType"

function Write-Info { param($Msg) Write-Host $Msg -ForegroundColor Cyan }
function Write-Ok   { param($Msg) Write-Host $Msg -ForegroundColor Green }

# --- Stop running instance --------------------------------------------------

Write-Info "Stopping GhostType if running..."
Get-Process -Name "ghosttype*" -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
Start-Sleep -Seconds 1

# --- Remove binaries --------------------------------------------------------

if (Test-Path $InstallDir) {
    Write-Info "Removing $InstallDir..."
    Remove-Item -Recurse -Force $InstallDir
}

# --- Remove app data --------------------------------------------------------

if (Test-Path $DataDir) {
    Write-Info "Removing $DataDir..."
    Remove-Item -Recurse -Force $DataDir
}

# --- Remove from PATH -------------------------------------------------------

$UserPath = [Environment]::GetEnvironmentVariable("PATH", "User")
if ($UserPath -like "*$InstallDir*") {
    Write-Info "Removing from user PATH..."
    $NewPath = ($UserPath -split ";" | Where-Object { $_ -ne $InstallDir }) -join ";"
    [Environment]::SetEnvironmentVariable("PATH", $NewPath, "User")
}

# --- Done -------------------------------------------------------------------

Write-Ok ""
Write-Ok "GhostType has been uninstalled."
Write-Ok ""
