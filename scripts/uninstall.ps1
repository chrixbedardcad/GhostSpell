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
$procs = Get-Process -Name "ghostspell*" -ErrorAction SilentlyContinue
if ($procs) {
    $procs | Stop-Process -Force -ErrorAction SilentlyContinue
    # Wait up to 10 seconds for the process to fully exit and release file handles.
    $waited = 0
    while ($waited -lt 10) {
        Start-Sleep -Seconds 1
        $waited++
        $still = Get-Process -Name "ghostspell*" -ErrorAction SilentlyContinue
        if (-not $still) { break }
    }
    if ($waited -ge 10) {
        Write-Host "Warning: GhostSpell may still be running. Trying to continue..." -ForegroundColor Yellow
    }
}

# --- Remove binaries --------------------------------------------------------

if (Test-Path $InstallDir) {
    Write-Info "Removing $InstallDir..."
    # Retry up to 3 times — file handles may take a moment to release after process exit.
    $attempt = 0
    $removed = $false
    while ($attempt -lt 3 -and -not $removed) {
        try {
            Remove-Item -Recurse -Force $InstallDir
            $removed = $true
        } catch {
            $attempt++
            if ($attempt -lt 3) {
                Write-Host "  Retrying in 2 seconds (files still locked)..." -ForegroundColor Yellow
                Start-Sleep -Seconds 2
            } else {
                Write-Host "  Could not remove $InstallDir — files may still be locked." -ForegroundColor Red
                Write-Host "  Close GhostSpell and try again, or delete the folder manually." -ForegroundColor Red
            }
        }
    }
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
