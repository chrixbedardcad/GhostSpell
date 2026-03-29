@echo off
setlocal enabledelayedexpansion

:: GhostSpell Release — Windows
::
:: Builds everything, packages artifacts with CI-compatible names,
:: and uploads to a GitHub Release. The in-app update checker picks
:: up the new version automatically.
::
:: Prerequisites: gh (GitHub CLI) authenticated, git clean working tree.
::
:: Usage:
::   _release.bat              # build + upload
::   _release.bat --skip-build # upload existing binaries (already built)

cd /d "%~dp0"

echo ============================================
echo        GhostSpell Release — Windows
echo ============================================
echo.

:: --- Pre-flight checks ---
where gh >nul 2>&1
if !errorlevel! neq 0 (
    echo ERROR: gh ^(GitHub CLI^) not found. Install from https://cli.github.com
    pause
    exit /b 1
)

:: Read version from source of truth.
for /f "tokens=4 delims= " %%v in ('findstr /C:"const Version" internal\version\version.go') do set "VER=%%~v"
if "!VER!"=="" (
    echo ERROR: Could not read version from internal\version\version.go
    pause
    exit /b 1
)
set TAG=v!VER!
echo Version: !VER!
echo Tag:     !TAG!
echo.

:: Check for uncommitted changes.
git diff --quiet 2>nul
if !errorlevel! neq 0 (
    echo WARNING: You have uncommitted changes. Commit or stash them first.
    echo.
    git status --short
    echo.
    set /p CONT="Continue anyway? (y/N) "
    if /i not "!CONT!"=="y" exit /b 1
)

:: --- Build (unless --skip-build) ---
if "%~1"=="--skip-build" (
    echo Skipping build ^(--skip-build^)
    echo.
) else (
    echo [1/3] Building...
    echo.
    call _build.bat
    if !errorlevel! neq 0 (
        echo ERROR: Build failed
        pause
        exit /b 1
    )
    echo.
)

:: --- Verify artifacts exist ---
echo [2/3] Checking artifacts...
set MISSING=0
if not exist ghostspell.exe (echo   MISSING: ghostspell.exe & set MISSING=1)
if not exist ghostai.exe (echo   MISSING: ghostai.exe & set MISSING=1)
if not exist ghost.exe (echo   MISSING: ghost.exe & set MISSING=1)
if !MISSING!==1 (
    echo ERROR: Required artifacts missing. Run _build.bat first.
    pause
    exit /b 1
)

:: --- Package with CI-compatible names ---
echo   Packaging artifacts...
if not exist release mkdir release
copy /y ghostspell.exe "release\ghostspell-windows-amd64.exe" >nul
copy /y ghostai.exe "release\ghostai-windows-amd64.exe" >nul
copy /y ghost.exe "release\ghost-windows-amd64.exe" >nul
:: Include CUDA DLLs if present (needed at runtime for GPU acceleration).
for %%f in (ggml*.dll llama.dll) do (
    if exist "%%f" copy /y "%%f" "release\" >nul
)
echo   Done.
echo.

:: --- Create tag + upload ---
echo [3/3] Uploading to GitHub Release !TAG!...
echo.

:: Create tag if it doesn't exist.
git tag "!TAG!" 2>nul
git push origin "!TAG!" 2>nul

:: Create or update the release.
gh release create "!TAG!" release\* --title "!TAG!" --generate-release-notes
if !errorlevel! neq 0 (
    echo.
    echo Release may already exist. Uploading assets to existing release...
    for %%f in (release\*) do (
        gh release upload "!TAG!" "%%f" --clobber
    )
)

echo.
echo ============================================
echo   RELEASE COMPLETE: !TAG!
echo   https://github.com/chrixbedardcad/GhostSpell/releases/tag/!TAG!
echo ============================================
echo.
pause
