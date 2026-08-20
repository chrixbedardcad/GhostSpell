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
# No administrator rights are required: everything is written under your own
# user profile. If you hit "access denied", it is a local policy or antivirus
# block, not a permissions problem with the install location - the script
# prints the exact step and path that was refused.

$ErrorActionPreference = "Stop"
$Repo = "chrixbedardcad/GhostSpell"
$InstallDir = Join-Path $env:LOCALAPPDATA "GhostSpell"

function Write-Info  { param($Msg) Write-Host $Msg -ForegroundColor Cyan }
function Write-Ok    { param($Msg) Write-Host $Msg -ForegroundColor Green }
function Write-Warn  { param($Msg) Write-Host $Msg -ForegroundColor Yellow }
function Write-Err   { param($Msg) Write-Host $Msg -ForegroundColor Red }

# Windows PowerShell 5.1 on older builds still negotiates TLS 1.0, which
# github.com refuses outright. Opt in to TLS 1.2 before any web call.
try {
    [Net.ServicePointManager]::SecurityProtocol =
        [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
} catch { }

# Invoke-WebRequest in PS 5.1 renders a progress bar per byte block, which makes
# a 160 MB download roughly ten times slower. Suppress it, restore it at the end.
$OrigProgress = $ProgressPreference
$ProgressPreference = "SilentlyContinue"

function Test-AccessDenied {
    param($ErrorRecord)
    if ($ErrorRecord.Exception -is [System.UnauthorizedAccessException]) { return $true }
    $msg = "$($ErrorRecord.Exception.Message)"
    return ($msg -match "denied|UnauthorizedAccess|0x80070005")
}

function Fail {
    param($Step, $ErrorRecord, [string[]]$Hints)
    $ProgressPreference = $OrigProgress
    Write-Host ""
    Write-Err "GhostSpell install failed."
    Write-Err "  Step:  $Step"
    Write-Err "  Error: $($ErrorRecord.Exception.Message)"
    if (Test-AccessDenied $ErrorRecord) {
        Write-Host ""
        Write-Warn "This is an 'access denied' from Windows, not from GitHub."
        Write-Warn "GhostSpell installs into your own user profile, so this is almost"
        Write-Warn "always antivirus, Controlled Folder Access, or a workplace policy:"
        Write-Host ""
        Write-Host "  1. Antivirus / Microsoft Defender flagged the unsigned binary."
        Write-Host "     In an ADMIN PowerShell, allow the install folder, then re-run:"
        Write-Host "       Add-MpPreference -ExclusionPath `"$InstallDir`""
        Write-Host ""
        Write-Host "  2. Controlled Folder Access (ransomware protection) is on."
        Write-Host "     Windows Security > Virus & threat protection > Ransomware"
        Write-Host "     protection > Manage > allow PowerShell, or turn it off briefly."
        Write-Host ""
        Write-Host "  3. GhostSpell is already running and holding the file open."
        Write-Host "     Close it from the system tray, then re-run this installer."
        Write-Host ""
        Write-Host "  4. A managed/work computer blocks scripted installs. Download"
        Write-Host "     the .exe manually instead:"
        Write-Host "       https://github.com/$Repo/releases/latest"
    }
    if ($Hints) {
        Write-Host ""
        foreach ($h in $Hints) { Write-Host "  $h" }
    }
    Write-Host ""
    exit 1
}

# --- Resolve latest release -------------------------------------------------

# Unauthenticated api.github.com is rate limited per source IP, so a shared
# office or VPN address can get a 403 here. Fall back to the /latest/download/
# redirect, which needs no API call.
$Version = $null
Write-Info "Fetching latest GhostSpell version..."
try {
    $Release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -UseBasicParsing
    $Version = $Release.tag_name
    Write-Info "Latest version: $Version"
} catch {
    Write-Warn "Could not query the GitHub API ($($_.Exception.Message))."
    Write-Warn "Falling back to the 'latest release' redirect."
}

function Get-AssetUrl {
    param($Name)
    if ($Version) { return "https://github.com/$Repo/releases/download/$Version/$Name" }
    return "https://github.com/$Repo/releases/latest/download/$Name"
}

# --- Prepare install directory ----------------------------------------------

# Kill any running GhostSpell before overwriting the binaries.
$procs = Get-Process -Name "ghostspell*" -ErrorAction SilentlyContinue
if ($procs) {
    Write-Info "Stopping running GhostSpell..."
    $procs | Stop-Process -Force -ErrorAction SilentlyContinue
    $waited = 0
    while ($waited -lt 10) {
        Start-Sleep -Seconds 1
        $waited++
        if (-not (Get-Process -Name "ghostspell*" -ErrorAction SilentlyContinue)) { break }
    }
}

if (-not (Test-Path $InstallDir)) {
    try {
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    } catch {
        Fail "creating the install folder $InstallDir" $_
    }
}

# Probe writability now rather than after a 160 MB download.
try {
    $Probe = Join-Path $InstallDir ".write-test"
    Set-Content -Path $Probe -Value "ok" -Force
    Remove-Item -Path $Probe -Force
} catch {
    Fail "writing to the install folder $InstallDir" $_
}

# Clean up old console variant from previous versions.
$OldWindow = Join-Path $InstallDir "ghostspell-window.exe"
if (Test-Path $OldWindow) {
    Remove-Item -Force $OldWindow -ErrorAction SilentlyContinue
    Write-Info "Removed old ghostspell-window.exe"
}

# --- Download binaries ------------------------------------------------------

# Download beside the target, then rename into place. A partial or AV-quarantined
# download never replaces a working install, and the rename stays on one volume.
function Install-Binary {
    param($AssetName, $ExeName, [switch]$Required)

    $Url  = Get-AssetUrl $AssetName
    $Dest = Join-Path $InstallDir $ExeName
    $Tmp  = "$Dest.download"

    if (Test-Path $Tmp) { Remove-Item -Force $Tmp -ErrorAction SilentlyContinue }

    try {
        Invoke-WebRequest -Uri $Url -OutFile $Tmp -UseBasicParsing -ErrorAction Stop
        if (-not (Test-Path $Tmp)) {
            throw "the download completed but $Tmp is missing (antivirus may have quarantined it)"
        }
        $size = (Get-Item $Tmp).Length
        if ($size -lt 1MB) {
            throw "downloaded file is only $size bytes, which is not a valid binary"
        }
        Move-Item -Path $Tmp -Destination $Dest -Force -ErrorAction Stop
    } catch {
        Remove-Item -Force $Tmp -ErrorAction SilentlyContinue
        if ($Required) {
            Fail "downloading $AssetName to $Dest" $_ @(
                "Direct download link: $Url"
            )
        }
        Write-Warn "Optional component $AssetName was not installed: $($_.Exception.Message)"
        return $false
    }
    return $true
}

Write-Info "Downloading ghostspell-windows-amd64.exe (about 160 MB)..."
Install-Binary "ghostspell-windows-amd64.exe" "ghostspell.exe" -Required | Out-Null

$ExePath = Join-Path $InstallDir "ghostspell.exe"

# Download ghostai LLM server (optional).
Write-Info "Downloading ghostai (local LLM server)..."
if (Install-Binary "ghostai-windows-amd64.exe" "ghostai.exe") {
    Write-Info "ghostai installed to $(Join-Path $InstallDir 'ghostai.exe')"
}

# Download ghost CLI (optional).
if (Install-Binary "ghost-windows-amd64.exe" "ghost.exe") {
    Write-Info "ghost CLI installed to $(Join-Path $InstallDir 'ghost.exe')"
}

# --- Add to PATH ------------------------------------------------------------

# From here on nothing is essential: the binary is installed and runnable, so a
# blocked registry write or shortcut folder warns instead of failing the install.
try {
    $UserPath = [Environment]::GetEnvironmentVariable("PATH", "User")
    if ($UserPath -notlike "*$InstallDir*") {
        Write-Info "Adding $InstallDir to user PATH..."
        [Environment]::SetEnvironmentVariable("PATH", "$UserPath;$InstallDir", "User")
    }
} catch {
    Write-Warn "Could not update your PATH: $($_.Exception.Message)"
    Write-Warn "GhostSpell still works; run it from $ExePath or the Start Menu."
}
# Always refresh current session PATH so the command works immediately.
if ($env:PATH -notlike "*$InstallDir*") {
    $env:PATH = "$env:PATH;$InstallDir"
}

# --- Refresh icon cache -----------------------------------------------------

try { Start-Process -FilePath "ie4uinit.exe" -ArgumentList "-show" -NoNewWindow -Wait -ErrorAction SilentlyContinue } catch { }

# --- Start Menu shortcut ----------------------------------------------------

try {
    $StartMenu = Join-Path $env:APPDATA "Microsoft\Windows\Start Menu\Programs"
    $ShortcutPath = Join-Path $StartMenu "GhostSpell.lnk"

    Write-Info "Creating Start Menu shortcut..."
    $WshShell = New-Object -ComObject WScript.Shell
    $Shortcut = $WshShell.CreateShortcut($ShortcutPath)
    $Shortcut.TargetPath = $ExePath
    $Shortcut.WorkingDirectory = $InstallDir
    $Shortcut.IconLocation = "$ExePath,0"
    $Shortcut.Description = "GhostSpell - AI-powered text correction"
    $Shortcut.Save()
} catch {
    Write-Warn "Could not create the Start Menu shortcut: $($_.Exception.Message)"
}

# --- Startup shortcut (auto-start on login) ---------------------------------

# Controlled Folder Access and most workplace policies block the Startup folder.
# Losing auto-start is not a reason to fail an otherwise complete install.
try {
    $StartupDir = Join-Path $env:APPDATA "Microsoft\Windows\Start Menu\Programs\Startup"
    $StartupShortcut = Join-Path $StartupDir "GhostSpell.lnk"

    Write-Info "Adding GhostSpell to Windows startup..."
    $WshStartup = New-Object -ComObject WScript.Shell
    $Startup = $WshStartup.CreateShortcut($StartupShortcut)
    $Startup.TargetPath = $ExePath
    $Startup.WorkingDirectory = $InstallDir
    $Startup.Description = "GhostSpell - AI-powered text correction"
    $Startup.Save()
} catch {
    Write-Warn "Could not add GhostSpell to startup: $($_.Exception.Message)"
    Write-Warn "Launch it manually from the Start Menu instead."
}

# --- Done -------------------------------------------------------------------

$ProgressPreference = $OrigProgress

Write-Ok ""
if ($Version) {
    Write-Ok "GhostSpell $Version installed to $InstallDir"
} else {
    Write-Ok "GhostSpell installed to $InstallDir"
}
Write-Ok ""
Write-Info "Config is stored in: $env:APPDATA\GhostSpell\"
Write-Host ""

# --- Auto-launch ------------------------------------------------------------

Write-Info "Launching GhostSpell..."
try {
    Start-Process -FilePath $ExePath -ErrorAction Stop
    Write-Ok "GhostSpell is running in your system tray (bottom-right, near the clock)."
    Write-Host "  Look for the GhostSpell icon - click the ^ arrow if it's hidden."
} catch {
    Write-Warn "Could not launch GhostSpell automatically: $($_.Exception.Message)"
    Write-Warn "Start it yourself from: $ExePath"
}
Write-Host ""
Write-Info "To launch manually later:"
Write-Host "  Search 'GhostSpell' in the Start menu, or type 'ghostspell' in a terminal."
