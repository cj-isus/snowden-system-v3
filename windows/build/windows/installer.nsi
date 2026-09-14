; snowden.system — per-user NSIS installer.
; Правила (docs/PLAN.md):
;   - fail-closed: инсталлятор ничего не решает за продукт; он только кладёт
;     файлы и создаёт ярлыки. Никаких Run-записей автозапуска и служб.
;   - elevation: обычная установка per-user без админа; TUN поднимает права
;     точечно через UAC relaunch внутри приложения (PLAN §4.1).
;   - данные пользователя (%AppData%\snowden-system: DPAPI-vault + логи)
;     деинсталлятором не удаляются — решение за владельцем, удаление вручную.
; Сборка: wails build (собирает exe с тегами) → nsis-сборка скриптом
;   tools/package.ps1 (находит makensis и подставляет версии).

!include "MUI2.nsh"
!include "FileFunc.nsh"

; --- Версии/пути подставляются tools/package.ps1 через /D-флаги ---
!ifndef EXE_PATH
  !define EXE_PATH "..\bin\snowden-system.exe"
!endif
!ifndef WINTUN_PATH
  !define WINTUN_PATH "..\bin\wintun.dll"
!endif
!ifndef APP_VERSION
  !define APP_VERSION "2.0.0"
!endif

!define APP_NAME "snowden.system"
!define APP_PUBLISHER "snowden.system"
!define APP_EXE "snowden-system.exe"
!define APP_REGKEY "Software\snowden.system"
!define APP_UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\snowden.system"

Name "${APP_NAME} ${APP_VERSION}"
OutFile "..\bin\snowden-system-setup-${APP_VERSION}.exe"
InstallDir "$LOCALAPPDATA\Programs\snowden-system"
InstallDirRegKey HKCU "${APP_REGKEY}" "InstallDir"
RequestExecutionLevel user
Unicode true
; Installer-based NSIS defines VIProductVersion as X.X.X.X
VIProductVersion "${APP_VERSION}.0"
VIAddVersionKey "ProductName" "${APP_NAME}"
VIAddVersionKey "CompanyName" "${APP_PUBLISHER}"
VIAddVersionKey "FileDescription" "${APP_NAME} installer"
VIAddVersionKey "FileVersion" "${APP_VERSION}.0"
VIAddVersionKey "ProductVersion" "${APP_VERSION}"
VIAddVersionKey "LegalCopyright" "snowden.system"

; Пути в NSIS резолвятся от каталога этого скрипта (build/windows)
!define MUI_ICON "icon.ico"
!define MUI_UNICON "icon.ico"
!define MUI_ABORTWARNING

; Страницы: приветствие → лицензии нет (нечего принимать) → место → установка → финиш
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!define MUI_FINISHPAGE_RUN "$INSTDIR\${APP_EXE}"
!define MUI_FINISHPAGE_RUN_TEXT "Запустить ${APP_NAME}"
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

; Язык один — русский (UI тоже русский, PLAN §4.1 п.4)
!insertmacro MUI_LANGUAGE "Russian"

; --- Установка ---
Section "Install"
  SetShellVarContext current
  SetOutPath "$INSTDIR"

  ; Остановить запущенный экземпляр, если есть (иначе файл заблокирован)
  DetailPrint "Остановка запущенного ${APP_NAME}…"
  nsExec::Exec 'taskkill /IM "${APP_EXE}" /F'
  Sleep 500

  File "/oname=${APP_EXE}" "${EXE_PATH}"
  File "/oname=wintun.dll" "${WINTUN_PATH}"

  ; Ярлыки (per-user)
  CreateDirectory "$SMPROGRAMS\snowden.system"
  CreateShortCut "$SMPROGRAMS\snowden.system\${APP_NAME}.lnk" "$INSTDIR\${APP_EXE}"
  CreateShortCut "$DESKTOP\${APP_NAME}.lnk" "$INSTDIR\${APP_EXE}"
  CreateShortCut "$SMPROGRAMS\snowden.system\Удалить ${APP_NAME}.lnk" "$INSTDIR\Uninstall.exe"

  ; Uninstaller
  WriteUninstaller "$INSTDIR\Uninstall.exe"

  ; Реестр: путь + uninstall-запись (для «Установка и удаление программ»)
  WriteRegStr HKCU "${APP_REGKEY}" "InstallDir" "$INSTDIR"
  WriteRegStr HKCU "${APP_REGKEY}" "Version" "${APP_VERSION}"
  WriteRegStr HKCU "${APP_UNINST_KEY}" "DisplayName" "${APP_NAME} ${APP_VERSION}"
  WriteRegStr HKCU "${APP_UNINST_KEY}" "DisplayVersion" "${APP_VERSION}"
  WriteRegStr HKCU "${APP_UNINST_KEY}" "Publisher" "${APP_PUBLISHER}"
  WriteRegStr HKCU "${APP_UNINST_KEY}" "DisplayIcon" "$INSTDIR\${APP_EXE}"
  WriteRegStr HKCU "${APP_UNINST_KEY}" "UninstallString" '"$INSTDIR\Uninstall.exe"'
  WriteRegDWORD HKCU "${APP_UNINST_KEY}" "NoModify" 1
  WriteRegDWORD HKCU "${APP_UNINST_KEY}" "NoRepair" 1

  ; Размер для «Установка и удаление программ»
  ${GetSize} "$INSTDIR" "/S=0K" $0 $1 $2
  IntFmt $0 "0x%08X" $0
  WriteRegDWORD HKCU "${APP_UNINST_KEY}" "EstimatedSize" $0
SectionEnd

; --- Удаление ---
Section "Uninstall"
  SetShellVarContext current

  DetailPrint "Остановка запущенного ${APP_NAME}…"
  nsExec::Exec 'taskkill /IM "${APP_EXE}" /F'
  Sleep 500

  Delete "$INSTDIR\${APP_EXE}"
  Delete "$INSTDIR\wintun.dll"
  Delete "$INSTDIR\Uninstall.exe"
  RMDir "$INSTDIR"

  Delete "$SMPROGRAMS\snowden.system\${APP_NAME}.lnk"
  Delete "$SMPROGRAMS\snowden.system\Удалить ${APP_NAME}.lnk"
  RMDir "$SMPROGRAMS\snowden.system"
  Delete "$DESKTOP\${APP_NAME}.lnk"

  DeleteRegKey HKCU "${APP_UNINST_KEY}"
  DeleteRegKey HKCU "${APP_REGKEY}"

  ; %AppData%\snowden-system (DPAPI-vault + логи) НЕ удаляем — данные владельца.
  MessageBox MB_ICONINFORMATION|MB_OK \
    "Данные хранилища и логи в %AppData%\snowden-system сохранены.$\nУдалите их вручную, если они больше не нужны."
SectionEnd
