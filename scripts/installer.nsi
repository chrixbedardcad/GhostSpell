; GhostSpell NSIS Installer
; Creates a proper Windows installer with Start Menu, Add/Remove Programs,
; optional auto-start, and a clean uninstaller.
;
; Build:
;   makensis scripts\installer.nsi
;
; Requires NSIS 3.x — https://nsis.sourceforge.io/

;---------------------------------------------------------------------------
; Includes
;---------------------------------------------------------------------------
!include "MUI2.nsh"
!include "FileFunc.nsh"
!include "LogicLib.nsh"

;---------------------------------------------------------------------------
; Version — read from release\version.txt (written by _release.bat)
;---------------------------------------------------------------------------
!ifndef VERSION
  !define VERSION "0.0.0"
!endif

!define PRODUCT_NAME      "GhostSpell"
!define PRODUCT_PUBLISHER  "GhostSpell"
!define PRODUCT_WEB_SITE   "https://www.ghostspell.com"
!define PRODUCT_EXE        "ghostspell.exe"
!define UNINSTALLER         "uninstall.exe"

; Registry keys for Add/Remove Programs.
!define UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT_NAME}"
!define UNINST_ROOT_KEY "HKCU"

;---------------------------------------------------------------------------
; General
;---------------------------------------------------------------------------
Name "${PRODUCT_NAME} ${VERSION}"
OutFile "..\release\GhostSpell-Setup.exe"

; Per-user install — no admin required.
InstallDir "$LOCALAPPDATA\${PRODUCT_NAME}"
InstallDirRegKey ${UNINST_ROOT_KEY} "${UNINST_KEY}" "InstallLocation"

RequestExecutionLevel user
SetCompressor /SOLID lzma
SetCompressorDictSize 32

; Version info embedded in the .exe (shows in Properties > Details).
VIProductVersion "${VERSION}.0"
VIAddVersionKey "ProductName" "${PRODUCT_NAME}"
VIAddVersionKey "CompanyName" "${PRODUCT_PUBLISHER}"
VIAddVersionKey "FileDescription" "${PRODUCT_NAME} Installer"
VIAddVersionKey "FileVersion" "${VERSION}"
VIAddVersionKey "ProductVersion" "${VERSION}"
VIAddVersionKey "LegalCopyright" "Copyright (c) GhostSpell contributors"

;---------------------------------------------------------------------------
; MUI Configuration
;---------------------------------------------------------------------------
!define MUI_ICON "..\assets\ghostspell.ico"
!define MUI_UNICON "..\assets\ghostspell.ico"
!define MUI_ABORTWARNING

; Welcome page — branding text.
!define MUI_WELCOMEPAGE_TITLE "Welcome to ${PRODUCT_NAME}"
!define MUI_WELCOMEPAGE_TEXT "This will install ${PRODUCT_NAME} ${VERSION} on your computer.$\r$\n$\r$\n\
${PRODUCT_NAME} is an AI-powered text assistant that runs in your system tray. \
Select text anywhere, press Ctrl+G, and GhostSpell corrects, translates, or rewrites it instantly.$\r$\n$\r$\n\
Click Next to continue."

; Finish page — offer to launch + auto-start option.
!define MUI_FINISHPAGE_RUN "$INSTDIR\${PRODUCT_EXE}"
!define MUI_FINISHPAGE_RUN_TEXT "Launch ${PRODUCT_NAME}"
!define MUI_FINISHPAGE_SHOWREADME ""
!define MUI_FINISHPAGE_SHOWREADME_TEXT "Start ${PRODUCT_NAME} with Windows"
!define MUI_FINISHPAGE_SHOWREADME_FUNCTION CreateAutoStart

;---------------------------------------------------------------------------
; Pages
;---------------------------------------------------------------------------
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_LICENSE "..\LICENSE"
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

; Uninstaller pages.
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

;---------------------------------------------------------------------------
; Language
;---------------------------------------------------------------------------
!insertmacro MUI_LANGUAGE "English"

;---------------------------------------------------------------------------
; Install Section
;---------------------------------------------------------------------------
Section "Install" SecInstall
  SetOutPath "$INSTDIR"

  ; Kill running instances before overwriting.
  nsExec::ExecToLog 'taskkill /F /IM ghostspell.exe'
  nsExec::ExecToLog 'taskkill /F /IM ghostai.exe'
  nsExec::ExecToLog 'taskkill /F /IM ghostvoice.exe'
  nsExec::ExecToLog 'taskkill /F /IM ghost.exe'
  Sleep 1000

  ; Core binaries.
  File "..\release\ghostspell-windows-amd64.exe"
  Rename "$INSTDIR\ghostspell-windows-amd64.exe" "$INSTDIR\ghostspell.exe"

  ; ghostai — LLM inference server (optional, may not be in release).
  IfFileExists "..\release\ghostai-windows-amd64.exe" 0 +3
    File "..\release\ghostai-windows-amd64.exe"
    Rename "$INSTDIR\ghostai-windows-amd64.exe" "$INSTDIR\ghostai.exe"

  ; ghost CLI (optional).
  IfFileExists "..\release\ghost-windows-amd64.exe" 0 +3
    File "..\release\ghost-windows-amd64.exe"
    Rename "$INSTDIR\ghost-windows-amd64.exe" "$INSTDIR\ghost.exe"

  ; Create uninstaller.
  WriteUninstaller "$INSTDIR\${UNINSTALLER}"

  ; Start Menu shortcut.
  CreateDirectory "$SMPROGRAMS\${PRODUCT_NAME}"
  CreateShortcut "$SMPROGRAMS\${PRODUCT_NAME}\${PRODUCT_NAME}.lnk" \
    "$INSTDIR\${PRODUCT_EXE}" "" "$INSTDIR\${PRODUCT_EXE}" 0
  CreateShortcut "$SMPROGRAMS\${PRODUCT_NAME}\Uninstall.lnk" \
    "$INSTDIR\${UNINSTALLER}" "" "$INSTDIR\${UNINSTALLER}" 0

  ; Register in Add/Remove Programs.
  WriteRegStr ${UNINST_ROOT_KEY} "${UNINST_KEY}" "DisplayName" "${PRODUCT_NAME}"
  WriteRegStr ${UNINST_ROOT_KEY} "${UNINST_KEY}" "DisplayVersion" "${VERSION}"
  WriteRegStr ${UNINST_ROOT_KEY} "${UNINST_KEY}" "Publisher" "${PRODUCT_PUBLISHER}"
  WriteRegStr ${UNINST_ROOT_KEY} "${UNINST_KEY}" "URLInfoAbout" "${PRODUCT_WEB_SITE}"
  WriteRegStr ${UNINST_ROOT_KEY} "${UNINST_KEY}" "UninstallString" '"$INSTDIR\${UNINSTALLER}"'
  WriteRegStr ${UNINST_ROOT_KEY} "${UNINST_KEY}" "QuietUninstallString" '"$INSTDIR\${UNINSTALLER}" /S'
  WriteRegStr ${UNINST_ROOT_KEY} "${UNINST_KEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr ${UNINST_ROOT_KEY} "${UNINST_KEY}" "DisplayIcon" "$INSTDIR\${PRODUCT_EXE},0"
  WriteRegDWORD ${UNINST_ROOT_KEY} "${UNINST_KEY}" "NoModify" 1
  WriteRegDWORD ${UNINST_ROOT_KEY} "${UNINST_KEY}" "NoRepair" 1

  ; Calculate and write installed size.
  ${GetSize} "$INSTDIR" "/S=0K" $0 $1 $2
  IntFmt $0 "0x%08X" $0
  WriteRegDWORD ${UNINST_ROOT_KEY} "${UNINST_KEY}" "EstimatedSize" $0

  ; Add to user PATH.
  nsExec::ExecToLog 'powershell -NoProfile -Command "$$p = [Environment]::GetEnvironmentVariable(\"PATH\",\"User\"); if ($$p -notlike \"*$INSTDIR*\") { [Environment]::SetEnvironmentVariable(\"PATH\",\"$$p;$INSTDIR\",\"User\") }"'

SectionEnd

;---------------------------------------------------------------------------
; Auto-start helper (called from MUI finish page checkbox)
;---------------------------------------------------------------------------
Function CreateAutoStart
  CreateShortcut "$SMSTARTUP\${PRODUCT_NAME}.lnk" "$INSTDIR\${PRODUCT_EXE}" \
    "" "$INSTDIR\${PRODUCT_EXE}" 0
FunctionEnd

;---------------------------------------------------------------------------
; Uninstall Section
;---------------------------------------------------------------------------
Section "Uninstall"

  ; Kill running instances.
  nsExec::ExecToLog 'taskkill /F /IM ghostspell.exe'
  nsExec::ExecToLog 'taskkill /F /IM ghostai.exe'
  nsExec::ExecToLog 'taskkill /F /IM ghostvoice.exe'
  nsExec::ExecToLog 'taskkill /F /IM ghost.exe'
  Sleep 1000

  ; Remove binaries.
  Delete "$INSTDIR\ghostspell.exe"
  Delete "$INSTDIR\ghostai.exe"
  Delete "$INSTDIR\ghostvoice.exe"
  Delete "$INSTDIR\ghost.exe"
  Delete "$INSTDIR\${UNINSTALLER}"
  RMDir "$INSTDIR"

  ; Remove Start Menu shortcuts.
  Delete "$SMPROGRAMS\${PRODUCT_NAME}\${PRODUCT_NAME}.lnk"
  Delete "$SMPROGRAMS\${PRODUCT_NAME}\Uninstall.lnk"
  RMDir "$SMPROGRAMS\${PRODUCT_NAME}"

  ; Remove Startup shortcut (auto-start).
  Delete "$SMSTARTUP\${PRODUCT_NAME}.lnk"

  ; Remove registry entries.
  DeleteRegKey ${UNINST_ROOT_KEY} "${UNINST_KEY}"

  ; Remove from user PATH.
  nsExec::ExecToLog 'powershell -NoProfile -Command "$$p = [Environment]::GetEnvironmentVariable(\"PATH\",\"User\"); if ($$p -like \"*$INSTDIR*\") { $$new = ($$p -split \";\" | Where-Object { $$_ -ne \"$INSTDIR\" }) -join \";\"; [Environment]::SetEnvironmentVariable(\"PATH\",$$new,\"User\") }"'

  ; Note: config + models in %APPDATA%\GhostSpell are intentionally preserved.
  ; The user can delete them manually if they want a full wipe.
  MessageBox MB_ICONINFORMATION|MB_OK \
    "${PRODUCT_NAME} has been uninstalled.$\r$\n$\r$\n\
Your configuration and downloaded models are preserved in:$\r$\n\
$APPDATA\${PRODUCT_NAME}$\r$\n$\r$\n\
Delete that folder to remove all data."

SectionEnd
