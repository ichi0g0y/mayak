; MAYAK's Windows installer (NSIS 3). tools/nsis builds it from build/bin
; with `task installer`, passing the paths and the version as defines.
;
; It installs for the current user only (%LOCALAPPDATA%\Programs\MAYAK), so
; it needs no administrator rights and the app's own updater can replace
; the files later. A running MAYAK is asked to quit first (its single
; instance mutex tells). The settings under %APPDATA%\Mayak are kept on
; uninstall unless the user says otherwise.

Unicode true

!include "MUI2.nsh"
!include "x64.nsh"
!include "WinVer.nsh"
!include "FileFunc.nsh"
!include "LogicLib.nsh"

!ifndef VERSION
  !define VERSION "0.0.0"
!endif
!ifndef VERSION4
  !define VERSION4 "${VERSION}.0"
!endif
!ifndef DISPLAYVERSION
  !define DISPLAYVERSION "${VERSION}"
!endif
!ifndef BIN
  !define BIN "..\..\bin"
!endif
!ifndef OUTFILE
  !define OUTFILE "..\..\dist\Mayak-Setup-windows-amd64.exe"
!endif
!ifndef ICON
  !define ICON "..\icon.ico"
!endif
!ifndef LICENSE
  !define LICENSE "..\..\..\LICENSE"
!endif

!define PRODUCT "MAYAK"
!define EXE "Mayak.exe"
; The app's single instance mutex (internal/app/run.go: UniqueID + "-sim").
!define MUTEX "com.ichi0g0y.mayak-sim"
!define UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\MAYAK"
!define RUN_KEY "Software\Microsoft\Windows\CurrentVersion\Run"
!define SITE "https://mayak.ichi0g0y.workers.dev"
!define REPO "https://github.com/ichi0g0y/mayak"

Name "${PRODUCT}"
OutFile "${OUTFILE}"
InstallDir "$LOCALAPPDATA\Programs\MAYAK"
InstallDirRegKey HKCU "${UNINST_KEY}" "InstallLocation"
RequestExecutionLevel user
SetCompressor /SOLID lzma
ManifestDPIAware true
ShowInstDetails show
ShowUninstDetails show

VIProductVersion "${VERSION4}"
VIFileVersion "${VERSION4}"
VIAddVersionKey /LANG=0 "ProductName" "${PRODUCT}"
VIAddVersionKey /LANG=0 "ProductVersion" "${DISPLAYVERSION}"
VIAddVersionKey /LANG=0 "FileVersion" "${VERSION4}"
VIAddVersionKey /LANG=0 "FileDescription" "${PRODUCT} Setup"
VIAddVersionKey /LANG=0 "CompanyName" "MAYAK contributors"
VIAddVersionKey /LANG=0 "LegalCopyright" "Copyright MAYAK contributors. GPL-3.0."

!define MUI_ICON "${ICON}"
!define MUI_UNICON "${ICON}"
!define MUI_ABORTWARNING
!define MUI_FINISHPAGE_RUN "$INSTDIR\${EXE}"
!define MUI_FINISHPAGE_RUN_TEXT "$(RunText)"
!define MUI_FINISHPAGE_SHOWREADME ""
!define MUI_FINISHPAGE_SHOWREADME_TEXT "$(DesktopShortcutText)"
!define MUI_FINISHPAGE_SHOWREADME_FUNCTION CreateDesktopShortcut

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

; The installer picks the language of the user's Windows.
!insertmacro MUI_LANGUAGE "English"
!insertmacro MUI_LANGUAGE "Japanese"

LangString RunText ${LANG_ENGLISH} "Start MAYAK"
LangString RunText ${LANG_JAPANESE} "MAYAK を起動する"
LangString DesktopShortcutText ${LANG_ENGLISH} "Create a desktop shortcut"
LangString DesktopShortcutText ${LANG_JAPANESE} "デスクトップにショートカットを作成する"
LangString Running ${LANG_ENGLISH} "MAYAK is running. Quit it from the tray icon, then click Retry."
LangString Running ${LANG_JAPANESE} "MAYAK が起動中です。トレイアイコンから終了してから「再試行」を押してください。"
LangString Win10Required ${LANG_ENGLISH} "MAYAK needs 64-bit Windows 10 or later."
LangString Win10Required ${LANG_JAPANESE} "MAYAK には 64 ビット版の Windows 10 以降が必要です。"
LangString WebView2Missing ${LANG_ENGLISH} "The WebView2 Runtime was not found. MAYAK needs it: install it from https://developer.microsoft.com/microsoft-edge/webview2/ (it is part of Windows 11 and most Windows 10 installations)."
LangString WebView2Missing ${LANG_JAPANESE} "WebView2 ランタイムが見つかりません。MAYAK の実行に必要です。https://developer.microsoft.com/microsoft-edge/webview2/ からインストールしてください（Windows 11 と多くの Windows 10 には最初から入っています）。"
LangString RemoveData ${LANG_ENGLISH} "Also delete MAYAK's settings and data ($APPDATA\Mayak)?"
LangString RemoveData ${LANG_JAPANESE} "MAYAK の設定とデータ（$APPDATA\Mayak）も削除しますか？"

; Waits for a running MAYAK to quit, or aborts.
!macro CheckRunning
  retry:
  System::Call 'kernel32::OpenMutexW(i 0x00100000, i 0, w "${MUTEX}") p .r0'
  ${If} $0 P<> 0
    System::Call 'kernel32::CloseHandle(p r0)'
    IfSilent 0 +2
      Abort
    MessageBox MB_RETRYCANCEL|MB_ICONEXCLAMATION "$(Running)" /SD IDCANCEL IDRETRY retry
    Abort
  ${EndIf}
!macroend

Function .onInit
  ${IfNot} ${AtLeastWin10}
  ${OrIfNot} ${IsNativeAMD64}
    MessageBox MB_OK|MB_ICONSTOP "$(Win10Required)" /SD IDOK
    Abort
  ${EndIf}
  !insertmacro CheckRunning
FunctionEnd

Function un.onInit
  !insertmacro CheckRunning
FunctionEnd

Function CreateDesktopShortcut
  CreateShortcut "$DESKTOP\${PRODUCT}.lnk" "$INSTDIR\${EXE}"
FunctionEnd

Section "MAYAK" SecMain
  SetShellVarContext current
  SetOutPath "$INSTDIR"

  ; A copy that is somehow still running keeps its file when it is renamed
  ; rather than overwritten; the app deletes *.mayak-old when it starts.
  IfFileExists "$INSTDIR\${EXE}" 0 +3
    Delete "$INSTDIR\${EXE}.mayak-old"
    Rename "$INSTDIR\${EXE}" "$INSTDIR\${EXE}.mayak-old"
  File "${BIN}\${EXE}"
  File "${BIN}\THIRD_PARTY_NOTICES.txt"
  File "/oname=LICENSE.txt" "${LICENSE}"

  ; The OCR runtime is replaced whole, so files of an older build do not linger.
  RMDir /r "$INSTDIR\tesseract"
  SetOutPath "$INSTDIR\tesseract"
  File /r "${BIN}\tesseract\*.*"
  SetOutPath "$INSTDIR"

  CreateShortcut "$SMPROGRAMS\${PRODUCT}.lnk" "$INSTDIR\${EXE}"

  WriteUninstaller "$INSTDIR\uninstall.exe"
  WriteRegStr HKCU "${UNINST_KEY}" "DisplayName" "${PRODUCT}"
  WriteRegStr HKCU "${UNINST_KEY}" "DisplayVersion" "${DISPLAYVERSION}"
  WriteRegStr HKCU "${UNINST_KEY}" "Publisher" "MAYAK contributors"
  WriteRegStr HKCU "${UNINST_KEY}" "DisplayIcon" "$INSTDIR\${EXE}"
  WriteRegStr HKCU "${UNINST_KEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "${UNINST_KEY}" "UninstallString" "$\"$INSTDIR\uninstall.exe$\""
  WriteRegStr HKCU "${UNINST_KEY}" "QuietUninstallString" "$\"$INSTDIR\uninstall.exe$\" /S"
  WriteRegStr HKCU "${UNINST_KEY}" "URLInfoAbout" "${SITE}"
  WriteRegStr HKCU "${UNINST_KEY}" "HelpLink" "${REPO}/issues"
  WriteRegDWORD HKCU "${UNINST_KEY}" "NoModify" 1
  WriteRegDWORD HKCU "${UNINST_KEY}" "NoRepair" 1
  ${GetSize} "$INSTDIR" "/S=0K" $0 $1 $2
  IntFmt $0 "0x%08X" $0
  WriteRegDWORD HKCU "${UNINST_KEY}" "EstimatedSize" "$0"

  ; WebView2 is checked, not bundled: it ships with Windows 11 and nearly
  ; every Windows 10, and the download is a few clicks away otherwise.
  SetRegView 64
  ReadRegStr $0 HKLM "SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}" "pv"
  ${If} $0 == ""
    ReadRegStr $0 HKCU "Software\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}" "pv"
  ${EndIf}
  ${If} $0 == ""
    MessageBox MB_OK|MB_ICONINFORMATION "$(WebView2Missing)" /SD IDOK
  ${EndIf}
SectionEnd

Section "Uninstall"
  SetShellVarContext current
  Delete "$INSTDIR\${EXE}"
  Delete "$INSTDIR\${EXE}.mayak-old"
  Delete "$INSTDIR\THIRD_PARTY_NOTICES.txt"
  Delete "$INSTDIR\LICENSE.txt"
  Delete "$INSTDIR\uninstall.exe"
  RMDir /r "$INSTDIR\tesseract"
  RMDir "$INSTDIR"
  Delete "$SMPROGRAMS\${PRODUCT}.lnk"
  Delete "$DESKTOP\${PRODUCT}.lnk"
  ; The app's own "launch at startup" entry (internal/autostart).
  DeleteRegValue HKCU "${RUN_KEY}" "Mayak"
  DeleteRegKey HKCU "${UNINST_KEY}"
  MessageBox MB_YESNO|MB_ICONQUESTION "$(RemoveData)" /SD IDNO IDNO keep
    RMDir /r "$APPDATA\Mayak"
  keep:
SectionEnd
