Unicode true
RequestExecutionLevel user
ManifestDPIAware true
SetCompressor /SOLID zlib
!include "MUI2.nsh"
!include "x64.nsh"
!include "WinVer.nsh"
!include "LogicLib.nsh"

!define APP_NAME "游栖 · GameNest"
!define UNINSTALL_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\GameNest"
Name "${APP_NAME}"
OutFile "${OUTPUT_FILE}"
InstallDir "$LOCALAPPDATA\Programs\GameNest"
InstallDirRegKey HKCU "${UNINSTALL_KEY}" "InstallLocation"
VIProductVersion "${APP_VERSION}.0"
VIAddVersionKey "ProductName" "${APP_NAME}"
VIAddVersionKey "FileDescription" "GameNest Windows Installer"
VIAddVersionKey "FileVersion" "${APP_VERSION}"
VIAddVersionKey "LegalCopyright" "Copyright (c) 2026 Sagiri_sense"
!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
!define MUI_ABORTWARNING
!define MUI_WELCOMEPAGE_TEXT "欢迎安装游栖，让收藏的游戏有个家。$\r$\n$\r$\n本安装包不包含任何个人游戏数据或密钥。升级时保留已有 data；卸载也会保留个人资料。$\r$\n$\r$\n安装前请退出正在运行的游栖。缺少 WebView2 时将联网安装微软运行时。"
!define MUI_FINISHPAGE_RUN "$INSTDIR\游栖.exe"
!define MUI_FINISHPAGE_RUN_TEXT "启动游栖"
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!define MUI_UNCONFIRMPAGE_TEXT_TOP "将移除游栖程序及快捷方式。您的 data 文件夹和其他个人文件会保留在安装目录，之后可备份或重新安装继续使用。"
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "SimpChinese"

Function .onInit
    SetShellVarContext current
    SetRegView 64
    ${IfNot} ${AtLeastWin10}
        SetErrorLevel 64
        MessageBox MB_OK|MB_ICONSTOP "游栖需要 Windows 10 或更新版本。" /SD IDOK
        Abort
    ${EndIf}
    ${IfNot} ${IsNativeAMD64}
        SetErrorLevel 65
        MessageBox MB_OK|MB_ICONSTOP "此安装包适用于 Windows x64。" /SD IDOK
        Abort
    ${EndIf}
FunctionEnd

Function EnsureWebView2
    ReadRegStr $0 HKLM "SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}" "pv"
    ${If} $0 != ""
    ${AndIf} $0 != "0.0.0.0"
        Return
    ${EndIf}
    ReadRegStr $0 HKCU "Software\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}" "pv"
    ${If} $0 != ""
    ${AndIf} $0 != "0.0.0.0"
        Return
    ${EndIf}
    InitPluginsDir
    SetOutPath "$PLUGINSDIR"
    File /oname=MicrosoftEdgeWebview2Setup.exe "${BOOTSTRAPPER}"
    DetailPrint "正在安装 Microsoft Edge WebView2 Runtime…"
    ClearErrors
    ExecWait '"$PLUGINSDIR\MicrosoftEdgeWebview2Setup.exe" /silent /install' $1
    IfErrors webview_failed
    ReadRegStr $0 HKLM "SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}" "pv"
    ${If} $0 != ""
    ${AndIf} $0 != "0.0.0.0"
        Return
    ${EndIf}
    ReadRegStr $0 HKCU "Software\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}" "pv"
    ${If} $0 != ""
    ${AndIf} $0 != "0.0.0.0"
        Return
    ${EndIf}
webview_failed:
    SetErrorLevel 66
    MessageBox MB_OK|MB_ICONSTOP "WebView2 安装未完成，请检查网络，或从微软官网下载运行时后重试。" /SD IDOK
    Abort
FunctionEnd

Section "安装游栖"
    Call EnsureWebView2
    SetOutPath "$INSTDIR"
    File "${PAYLOAD}\游栖.exe"
    File "${PAYLOAD}\LICENSE"
    File "${PAYLOAD}\THIRD_PARTY_NOTICES.md"
    File "${PAYLOAD}\使用指南.md"
    File /r "${PAYLOAD}\licenses"
    SetOutPath "$INSTDIR\data"
    SetOverwrite off
    File "${PAYLOAD}\data\games.json"
    SetOverwrite on
    SetOutPath "$INSTDIR"
    CreateShortcut "$DESKTOP\游栖.lnk" "$INSTDIR\游栖.exe"
    CreateShortcut "$SMPROGRAMS\游栖.lnk" "$INSTDIR\游栖.exe"
    WriteUninstaller "$INSTDIR\uninstall.exe"
    WriteRegStr HKCU "${UNINSTALL_KEY}" "DisplayName" "${APP_NAME}"
    WriteRegStr HKCU "${UNINSTALL_KEY}" "DisplayVersion" "${APP_VERSION}"
    WriteRegStr HKCU "${UNINSTALL_KEY}" "Publisher" "Sagiri_sense"
    WriteRegStr HKCU "${UNINSTALL_KEY}" "InstallLocation" "$INSTDIR"
    WriteRegStr HKCU "${UNINSTALL_KEY}" "DisplayIcon" "$INSTDIR\游栖.exe"
    WriteRegStr HKCU "${UNINSTALL_KEY}" "UninstallString" '$\"$INSTDIR\uninstall.exe$\"'
    WriteRegStr HKCU "${UNINSTALL_KEY}" "QuietUninstallString" '$\"$INSTDIR\uninstall.exe$\" /S'
    WriteRegDWORD HKCU "${UNINSTALL_KEY}" "NoModify" 1
    WriteRegDWORD HKCU "${UNINSTALL_KEY}" "NoRepair" 1
SectionEnd

Function un.onInit
    SetShellVarContext current
    SetRegView 64
FunctionEnd

Section "Uninstall"
    !include "${UNINSTALL_FILES}"
    Delete "$DESKTOP\游栖.lnk"
    Delete "$SMPROGRAMS\游栖.lnk"
    DeleteRegKey HKCU "${UNINSTALL_KEY}"
    Delete "$INSTDIR\uninstall.exe"
    ; Only remove the directory if empty. Never remove data or unknown files.
    RMDir "$INSTDIR"
SectionEnd
