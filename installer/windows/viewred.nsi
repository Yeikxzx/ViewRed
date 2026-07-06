!define PRODUCT_NAME "ViewRed"
!define PRODUCT_PUBLISHER "ViewRed"
!define PRODUCT_WEB_SITE "https://github.com/viewred/viewred"
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

; --- Pages ---
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_LICENSE "..\..\LICENSE"
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "English"

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

    ; Open port 8080 in Windows Firewall for local access
    ; (optional — uncomment if needed)
    ; nsExec::ExecToStack 'netsh advfirewall firewall add rule name="ViewRed" dir=in action=allow protocol=TCP localport=8080'
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
    RMDir /r "$INSTDIR\web"
    RMDir /r "$INSTDIR\internal"

    ; Start menu
    Delete "$SMPROGRAMS\${PRODUCT_NAME}\${PRODUCT_NAME}.lnk"
    Delete "$SMPROGRAMS\${PRODUCT_NAME}\Uninstall.lnk"
    RMDir "$SMPROGRAMS\${PRODUCT_NAME}"

    ; Desktop
    Delete "$DESKTOP\${PRODUCT_NAME}.lnk"

    RMDir "$INSTDIR"
    DeleteRegKey HKLM "${PRODUCT_UNINST_KEY}"

    ; Remove firewall rule if added
    ; nsExec::ExecToStack 'netsh advfirewall firewall delete rule name="ViewRed"'
SectionEnd
