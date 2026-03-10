# GhostSpell uninstaller for Windows.
#
# Usage (from CMD, PowerShell, or Windows Terminal):
#   powershell -c "irm https://raw.githubusercontent.com/chrixbedardcad/GhostSpell/main/scripts/uninstall.ps1 | iex"
#
# What it does:
#   1. Stops GhostSpell if it's running
#   2. Removes the app binaries from %LOCALAPPDATA%\GhostSpell\
#   3. Removes config and logs from %APPDATA%\GhostSpell\
#   4. Removes the install directory from user PATH
#

$ErrorActionPreference = "Stop"
$InstallDir = Join-Path $env:LOCALAPPDATA "GhostSpell"
$DataDir = Join-Path $env:APPDATA "GhostSpell"

function Write-Info { param($Msg) Write-Host $Msg -ForegroundColor Cyan }
function Write-Ok   { param($Msg) Write-Host $Msg -ForegroundColor Green }

# --- Stop running instance --------------------------------------------------

Write-Info "Stopping GhostSpell if running..."
Get-Process -Name "ghostspell*" -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
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

# --- Remove Start Menu shortcut ---------------------------------------------

$ShortcutPath = Join-Path $env:APPDATA "Microsoft\Windows\Start Menu\Programs\GhostSpell.lnk"
if (Test-Path $ShortcutPath) {
    Write-Info "Removing Start Menu shortcut..."
    Remove-Item -Force $ShortcutPath
}

# --- Remove Startup shortcut -----------------------------------------------

$StartupShortcut = Join-Path $env:APPDATA "Microsoft\Windows\Start Menu\Programs\Startup\GhostSpell.lnk"
if (Test-Path $StartupShortcut) {
    Write-Info "Removing Startup shortcut..."
    Remove-Item -Force $StartupShortcut
}

# --- Remove from PATH -------------------------------------------------------

$UserPath = [Environment]::GetEnvironmentVariable("PATH", "User")
if ($UserPath -like "*$InstallDir*") {
    Write-Info "Removing from user PATH..."
    $NewPath = ($UserPath -split ";" | Where-Object { $_ -ne $InstallDir }) -join ";"
    [Environment]::SetEnvironmentVariable("PATH", $NewPath, "User")
}

# --- Done -------------------------------------------------------------------

Write-Host ""
Write-Host "  If old icons linger in the Start Menu, reboot to clear the icon cache." -ForegroundColor Yellow

Write-Ok ""
Write-Ok "GhostSpell has been uninstalled."
Write-Ok ""
