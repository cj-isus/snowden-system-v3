# TH-02 — владение проверенным образом native helper

Статус: V2-148, 2026-10-25, **исходники, unverified**, только PC/Windows.
Тесты не добавлялись/не менялись/не запускались; build/compile/vet/check/module/
generation/SDK/product CLI/helper/native/network/live не выполнялись.
`gofmt -w` восьми Go-файлов завершился exit0; это не type/runtime/Win32 evidence.
Реальные EXE/ACL/owner certificates/keys/vault/runtime config не читались.

## Место в архитектуре

После [V2-147](TH-02-NATIVE-PROTOCOL.md) существовали codec/helper и отдельная lane,
но не было проверки образа между pin и Resume. Теперь `CaptureNativeDNS` —
bounded core primitive: fixed DNS operation, optional diagnostic reservation,
compiled pin и обязательный caller scope guard. На V2-148 App caller отсутствовал.
V2-149 [manual App scope/action](TH-02-NATIVE-SCOPE.md) подключён в исходниках;
capabilities теперь `not_checked/manual_action`, без утверждения работоспособности.

Переиспользованы original Win32 process/Job/reaper, declared `x/sys/windows` и
`tailscale/wf`, standard hash/binary/PE types и DiagnosticEndpoint budget.
`filetxn` даёт file-batch rollback, publisher lock — writer serialization; они
не удерживают executable/ancestors с нужным sharing и не заменяют image lease.
Новые зависимости/версии/generated/tests/Android/harness не менялись.

## Проверка файла

Private constructor выбирает только sibling `snowden-native-probe.exe` возле
`os.Executable`, без caller path/argv/PATH/environment selector. Empty/неcanonical
64-character lowercase SHA256 отклоняется до файловой работы.

Поддерживается только local fixed NTFS/amd64. Absolute canonical DOS path≤2048,
без UNC/device/ADS/8.3 alias/reparse; components проходят стандартный Windows
`filepath.IsLocal` и дополнительные length/separator/trailing-dot-space bounds.
Путь открывается от root к directory, parent EXE и helper, не более64 directory handles.
Каждый ancestor удерживается до завершения batch; следующий компонент открывается
после проверки предыдущего. Оба EXE — disk, non-directory, single hardlink,
имеют разные file IDs; parent bytes не хешируются/не декодируются. Helper имеет
nonzero file ID, size1…32MiB. Это initial ceiling, не измеренный startup/RSS bound.

Все handles non-inheritable. `CreateFileW` использует OPEN_EXISTING,
OPEN_REPARSE_POINT, для directory BACKUP_SEMANTICS, **только FILE_SHARE_READ**.
Write/delete sharing не разрешается. Attributes, canonical final handle path,
volume/file IDs и security descriptor читаются по handle. Symlink/junction/mount
reparse, remote/non-NTFS/unsupported spelling/unknown identity → unavailable.
Правила sharing/reparse/directory —
[Microsoft CreateFileW](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-createfilew).

SHA256 читает exact declared file size через retained synchronous handle,
32KiB chunks/context checks, без mmap/whole-file allocation/передачи владения
`os.File` finalizer. ReaderAt сериализует seek/read mutex. Fixed PE header
projection использует standard `pe.FileHeader`/`OptionalHeader64` и binary decoder:
DOS/PE signatures, amd64 executable/non-DLL/CUI, header/section-table bounds и
entry-point range. Размер optional header240, число sections1…96.
Это capability projection, **не полный PE validator**; OS loader отдельно
принимает образ при CreateProcess. Полный `pe.NewFile` здесь не нужен: он также
читает file-declared symbol/string tables, которых эта проверка не использует.

## Консервативный ACL-контракт

Owner и active allow ACE, способные менять file/path, допустимы только для
current process user, SYSTEM, Administrators и точного TrustedInstaller service
SID. SID последнего — documented stable identifier, не SAM/domain lookup:
[Microsoft WindowsAppSDK](https://github.com/microsoft/WindowsAppSDK/blob/main/specs/applicationdata/ApplicationData.md).
NULL/absent/invalid DACL, >128 ACE или неизвестный active ACE format отклоняются.
SID body bounds проверяются до SID API; descriptor удерживается в Go lifetime.
Deny ACE не даёт дополнительных прав и не используется для оправдания опасного
allow. Inherit-only ACE не применяется к уже открытому объекту.

Для directory отдельный append/add-subdirectory bit не считается правом менять
существующий pinned component; file append запрещается для недоверенного principal.
Write data/EA/attributes, DELETE/DELETE_CHILD, WRITE_DAC/OWNER и generic write/all
учитываются. Это conservative allow inspection, не полный AuthZ/effective-access
движок. Правила bit meanings —
[Microsoft file rights](https://learn.microsoft.com/en-us/windows/win32/fileio/file-access-rights-constants).
ACL не исправляются и privileges не включаются; неподходящая установка просто
не получает capability. Не заявляется совместимость со всеми обычными ACL.

## Удержание через запуск и retirement

Optional lane теперь владеет и процессом, и private image lease. Даже partial
open/panic возвращает exact resource owner; failure не теряет ранее открытые
handles. Сперва original process/IO/empty Job/host handles, затем image/ancestor
handles закрываются в обратном порядке. Uncertain process cleanup не закрывает
image lease. Uncertain image close сохраняет оставшиеся handles и блокирует
replacement. Нет PID/name/taskkill/bare-exec/image-lock workaround.

Generic Win32 host получил optional beforeResume callback после assignment
приостановленного процесса в Job. Он повторяет handle IDs/path/ACL/AppID, сверяет
`QueryFullProcessImageName` именно original created process handle, затем caller
scope guard. Ошибка не Resume, partial owner уходит прежнему reaper.
[API contract](https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-queryfullprocessimagenamew).
Engine использует nil callback, сохраняет8-process и прежний env/memory contract.
Protected curl lane сохраняется; optional работа её не занимает.

## Guard и результаты

Нужен nonnil guard до admission/file work, после прежнего retirement, непосредственно
до Resume, после полного cleanup и после bounded reply decode. Context/guard panic,
смена scope, missing image, invalid/incomplete reply не выдаёт partial success.
Полный request/response+Max+1/IPC payload резервируется DiagnosticEndpoint до работы;
one deadline500ms…12s, one optional process и lane без очереди. Reservations не
возвращаются при ошибке. Generic public arbitrary optional helper API удалён;
единственный native operation wrapper не принимает произвольные endpoints/argv.

**Nonnil callback не доказывает корректный session lease.** V2-149 source caller
связывает actual intent/config/selector/worker/LUID/network epoch/observed clock,
cancellation/manual Stop и отдельные typed facts; [границы](TH-02-NATIVE-SCOPE.md).
Это реализация контракта в исходниках, не подтверждённая native/session приёмка.
Job exit/file identity не даёт маршрута, health/quorum/IPv6 egress или DNSSEC.
Raw packets не должны попадать в UI/log; отдельный resolver failure не channel FAIL.

## Пределы и остаток

Текущий user/Admins/SYSTEM/TrustedInstaller считаются доверенными. Sharing/ACL
не объявляются изоляцией от hostile same-owner/admin/kernel, ранее созданных
writable mappings или changes DOS device namespace доверенным владельцем.
Это ограничения модели, не проверенная устойчивость. Syscalls/loader/fs filters/
cleanup не получают hard total-time/CPU/RSS bound от cooperative context.

Нужны compile/API/PE/ACL/path/sharing/AV/nested Job/handle inheritance/short-path/
mapped-view/installer-updater/crash/reboot/native WFP/network/resource acceptance
после разрешения владельца. Pair update/rollback/owned installer и реальные
сроки/расходы не доказаны. Проверки ОС/сети не выполнялись; файлы установленного
приложения не открывались. Все TH-00…08/P0/quorum/controller/custody/DR/client/bindings/
rollout/release/comparative gates сохраняются.

Исходники: core `native_capture.go`, `native_image_{windows,acl_windows,
reader_windows,other}.go`, `helper_process.go`, `worker_process.go`,
`worker_host_windows.go`.
