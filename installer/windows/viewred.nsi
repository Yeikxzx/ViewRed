!define PRODUCT_NAME "ViewRed"
!define PRODUCT_PUBLISHER "ViewRed"
!define PRODUCT_WEB_SITE "https://github.com/Yeikxzx/ViewRed"
!define PRODUCT_UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT_NAME}"

!ifndef VERSION
  !define VERSION "0.1.0"
!endif

Name "${PRODUCT_NAME} ${VERSION}"
OutFile "..\..\dist\viewred-setup-${VERSION}.exe"
InstallDir "$PROGRAMFILES64\${PRODUCT_NAME}"
RequestExecutionLevel admin
SetCompressor /SOLID lzma

!include "MUI2.nsh"
!include "FileFunc.nsh"
!include "LogicLib.nsh"
!include "nsDialogs.nsh"
!include "WinMessages.nsh"

; --- Dependency URLs ---
!define URL_POSTGRESQL "https://www.postgresql.org/download/windows/"
!define URL_NPCAP     "https://npcap.com/#download"
!define URL_OLLAMA    "https://ollama.com/download/windows"
!define URL_NMAP      "https://nmap.org/download.html"

; --- Global vars for dependency status ---
Var DepPostgreSQL
Var DepNpcap
Var DepOllama
Var DepNmap
Var MissingCount
Var Dialog
Var LabelStatus
Var LinkPostgreSQL
Var LinkNpcap
Var LinkOllama
Var LinkNmap
Var ImgOkPostgreSQL
Var ImgFailPostgreSQL
Var ImgOkNpcap
Var ImgFailNpcap
Var ImgOkOllama
Var ImgFailOllama
Var ImgOkNmap
Var ImgFailNmap

; --- Pages ---
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_LICENSE "..\..\LICENSE"
Page custom DepsPageShow DepsPageLeave
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "English"

; --- Detect if a command is available in PATH ---
!macro CheckCommand result cmd
  ClearErrors
  nsExec::ExecToStack 'where ${cmd} 2>nul'
  Pop $0
  Pop $1
  ${If} $0 == 0
    StrCpy ${result} 1
  ${Else}
    StrCpy ${result} 0
  ${EndIf}
!macroend

; --- Detect PostgreSQL via registry ---
Function CheckPostgreSQL
  StrCpy $DepPostgreSQL 0
  ClearErrors
  EnumRegKey $0 HKLM "SOFTWARE\PostgreSQL\Installations" 0
  ${If} ${Errors}
    ; Try 32-bit registry on 64-bit
    SetRegView 32
    EnumRegKey $0 HKLM "SOFTWARE\PostgreSQL\Installations" 0
    SetRegView 64
    ${IfNot} ${Errors}
      StrCpy $DepPostgreSQL 1
    ${EndIf}
  ${Else}
    StrCpy $DepPostgreSQL 1
  ${EndIf}
FunctionEnd

; --- Detect Npcap via driver file ---
Function CheckNpcap
  StrCpy $DepNpcap 0
  ${If} ${FileExists} "$SYSDIR\Npcap\Packet.dll"
    StrCpy $DepNpcap 1
  ${ElseIf} ${FileExists} "$SYSDIR\drivers\npcap.sys"
    StrCpy $DepNpcap 1
  ${EndIf}
FunctionEnd

; --- Detect Ollama via command ---
Function CheckOllama
  !insertmacro CheckCommand $DepOllama "ollama"
FunctionEnd

; --- Detect nmap via command ---
Function CheckNmap
  !insertmacro CheckCommand $DepNmap "nmap"
FunctionEnd

; --- Run all checks ---
Function RunAllChecks
  Call CheckPostgreSQL
  Call CheckNpcap
  Call CheckOllama
  Call CheckNmap
  StrCpy $MissingCount 0
  ${If} $DepPostgreSQL == 0
    IntOp $MissingCount $MissingCount + 1
  ${EndIf}
  ${If} $DepNpcap == 0
    IntOp $MissingCount $MissingCount + 1
  ${EndIf}
  ${If} $DepOllama == 0
    IntOp $MissingCount $MissingCount + 1
  ${EndIf}
  ${If} $DepNmap == 0
    IntOp $MissingCount $MissingCount + 1
  ${EndIf}
FunctionEnd

; --- Custom depdendency check page ---
Function DepsPageShow
  Call RunAllChecks
  nsDialogs::Create 1018
  Pop $Dialog
  ${If} $Dialog == error
    Abort
  ${EndIf}

  ; Title
  ${NSD_CreateLabel} 0 0 100% 20u "System Dependencies Check"
  Pop $0
  CreateFont $1 "$(^Font)" "$(^FontSize)" "700"
  SendMessage $0 ${WM_SETFONT} $1 0

  ; Subtitle
  ${If} $MissingCount == 0
    ${NSD_CreateLabel} 0 25u 100% 15u "All required dependencies detected. You're ready to install."
  ${Else}
    ${NSD_CreateLabel} 0 25u 100% 15u "$MissingCount required dependenc${{If}} $MissingCount == 1{{Then}}y is{{Else}}ies are{{EndIf}} missing. Install them before running ViewRed."
  ${EndIf}
  Pop $0

  ; --- PostgreSQL ---
  ${If} $DepPostgreSQL == 1
    ${NSD_CreateLabel} 10 50u 80% 13u "[OK]  PostgreSQL 15+ with TimescaleDB"
  ${Else}
    ${NSD_CreateLabel} 10 50u 80% 13u "[MISSING]  PostgreSQL 15+ with TimescaleDB"
    ${NSD_CreateLink} 10 65u 80% 13u "Download PostgreSQL"
    Pop $LinkPostgreSQL
    ${NSD_OnClick} $LinkPostgreSQL OpenPostgreSQLURL
  ${EndIf}
  Pop $0

  ; --- Npcap ---
  ${If} $DepNpcap == 1
    ${NSD_CreateLabel} 10 82u 80% 13u "[OK]  Npcap (packet capture driver)"
  ${Else}
    ${NSD_CreateLabel} 10 82u 80% 13u "[MISSING]  Npcap (packet capture driver)"
    ${NSD_CreateLink} 10 97u 80% 13u "Download Npcap"
    Pop $LinkNpcap
    ${NSD_OnClick} $LinkNpcap OpenNpcapURL
  ${EndIf}
  Pop $0

  ; --- Ollama ---
  ${If} $DepOllama == 1
    ${NSD_CreateLabel} 10 114u 80% 13u "[OK]  Ollama (local AI assistant)"
  ${Else}
    ${NSD_CreateLabel} 10 114u 80% 13u "[MISSING]  Ollama (local AI assistant)"
    ${NSD_CreateLink} 10 129u 80% 13u "Download Ollama"
    Pop $LinkOllama
    ${NSD_OnClick} $LinkOllama OpenOllamaURL
  ${EndIf}
  Pop $0

  ; --- nmap ---
  ${If} $DepNmap == 1
    ${NSD_CreateLabel} 10 146u 80% 13u "[OK]  nmap (vulnerability scanner)"
  ${Else}
    ${NSD_CreateLabel} 10 146u 80% 13u "[MISSING]  nmap (vulnerability scanner)"
    ${NSD_CreateLink} 10 161u 80% 13u "Download nmap"
    Pop $LinkNmap
    ${NSD_OnClick} $LinkNmap OpenNmapURL
  ${EndIf}
  Pop $0

  nsDialogs::Show
FunctionEnd

Function DepsPageLeave
FunctionEnd

; --- URL openers ---
Function OpenPostgreSQLURL
  ExecShell "open" "${URL_POSTGRESQL}"
FunctionEnd

Function OpenNpcapURL
  ExecShell "open" "${URL_NPCAP}"
FunctionEnd

Function OpenOllamaURL
  ExecShell "open" "${URL_OLLAMA}"
FunctionEnd

Function OpenNmapURL
  ExecShell "open" "${URL_NMAP}"
FunctionEnd

Section "Install"
    SetOutPath "$INSTDIR"

    ; Main binary
    File "..\..\dist\viewred-windows-amd64.exe"

    ; ONNX Runtime DLL (optional — user may install separately)
    File /nonfatal "..\..\onnxruntime.dll"

    ; Config template
    File "..\..\config.example.yaml"

    ; Create data directories
    CreateDirectory "$INSTDIR\data\pcaps"
    CreateDirectory "$INSTDIR\data\reports"

    ; Start menu shortcuts
    CreateDirectory "$SMPROGRAMS\${PRODUCT_NAME}"
    CreateShortCut "$SMPROGRAMS\${PRODUCT_NAME}\${PRODUCT_NAME}.lnk" "$INSTDIR\viewred-windows-amd64.exe"
    CreateShortCut "$SMPROGRAMS\${PRODUCT_NAME}\Uninstall.lnk" "$INSTDIR\uninstall.exe"

    ; Desktop shortcut
    CreateShortCut "$DESKTOP\${PRODUCT_NAME}.lnk" "$INSTDIR\viewred-windows-amd64.exe"

    ; Write uninstaller
    WriteUninstaller "$INSTDIR\uninstall.exe"

    ; Registry for Add/Remove Programs
    WriteRegStr HKLM "${PRODUCT_UNINST_KEY}" "DisplayName" "${PRODUCT_NAME}"
    WriteRegStr HKLM "${PRODUCT_UNINST_KEY}" "DisplayVersion" "${VERSION}"
    WriteRegStr HKLM "${PRODUCT_UNINST_KEY}" "Publisher" "${PRODUCT_PUBLISHER}"
    WriteRegStr HKLM "${PRODUCT_UNINST_KEY}" "URLInfoAbout" "${PRODUCT_WEB_SITE}"
    WriteRegStr HKLM "${PRODUCT_UNINST_KEY}" "UninstallString" "$INSTDIR\uninstall.exe"
    WriteRegStr HKLM "${PRODUCT_UNINST_KEY}" "InstallLocation" "$INSTDIR"
    WriteRegDWORD HKLM "${PRODUCT_UNINST_KEY}" "NoModify" 1
    WriteRegDWORD HKLM "${PRODUCT_UNINST_KEY}" "NoRepair" 1

    ; Estimate size for Add/Remove Programs
    ${GetSize} "$INSTDIR" "/S=0K" $0 $1 $2
    IntFmt $0 "0x%08X" $0
    WriteRegDWORD HKLM "${PRODUCT_UNINST_KEY}" "EstimatedSize" "$0"
SectionEnd

Section "Uninstall"
    ; Stop any running instance
    nsExec::ExecToStack 'taskkill /f /im viewred-windows-amd64.exe'

    Delete "$INSTDIR\viewred-windows-amd64.exe"
    Delete "$INSTDIR\config.example.yaml"
    Delete "$INSTDIR\config.yaml"
    Delete "$INSTDIR\onnxruntime.dll"
    Delete "$INSTDIR\uninstall.exe"
    RMDir /r "$INSTDIR\data"

    ; Start menu
    Delete "$SMPROGRAMS\${PRODUCT_NAME}\${PRODUCT_NAME}.lnk"
    Delete "$SMPROGRAMS\${PRODUCT_NAME}\Uninstall.lnk"
    RMDir "$SMPROGRAMS\${PRODUCT_NAME}"

    ; Desktop
    Delete "$DESKTOP\${PRODUCT_NAME}.lnk"

    RMDir "$INSTDIR"
    DeleteRegKey HKLM "${PRODUCT_UNINST_KEY}"
SectionEnd
