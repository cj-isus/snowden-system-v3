# TH-01 / OFF-06 — одиночная запись состояния доверия Windows

2026-10-28, V2-172/178/185/186. Owner: `windows`. **Source implemented, unverified**.
Продолжение [stored readers](TH-01-PUBLIC-READERS.md). Это платформенный I/O;
криптография, source binding и правила принятия профиля остаются прежними.

## Проблема и потребители

`metadata.atomicWrite` писал через фиксированный `path + ".tmp"`, os.WriteFile и
os.Rename без явного Sync. Ошибка Rename вызывала unchecked path cleanup.
SaveAcceptedProfile сначала кодировал/проверял checkpoint, затем сериализовал
объект повторно в helper. Это source finding; сбой не воспроизводился.

| Путь | Новая запись | Лимит после кодирования |
|---|---|---|
| SaveKeys / trusted_keys.json | точный EncodeKeys output | 6MiB |
| SaveAcceptedProfile / version.txt | точный EncodeAcceptedProfile output | 1MiB |
| SaveEnvelope | прежний MarshalIndent output | 1MiB |

SaveKeys по-прежнему не может сплющивать schema2. SaveAcceptedProfile сохраняет
свои проверки delivery/pin/epoch authority и пропускает неизменённый checkpoint.
Legacy decimal reading, runtime bootstrap/clock caller ownership, import journal
и source gates не заменены I/O-компонентом. Чтение/запись файла не подтверждает
его подпись, актуальность, принятие или разрешение использовать канал.

## Компонент и порядок блокировок

`filetxn.BoundedWriteSlot` удерживает один исходный writer до подтверждённого
завершения. Constructor создаёт только bookkeeping; оригинал сохраняется до
любого native acquisition. Empty/oversized data отвергаются; ceiling не выше
существующего filetxn maxFileSize=16MiB. Input остаётся у caller и не кэшируется.

`metadata/stored_reader.go` расширен и переименован в `stored_file.go`: у каждого
из трёх fixed storage domains одна пара reader/writer и общий I/O mutex. Порядок:
caller journal → domain I/O → private slot. Domain не захватывает journal,
не вызывает Store/App/SDK/callbacks; освобождается до decode/другого loader.
Обычная конкуренция ждёт текущий I/O. Неопределённый reader Close запрещает write;
неопределённый writer outcome/Close запрещает и read, и write через эту пару.
Read при такой ошибке не выдаёт raw bytes или confirmed absence. Ошибка write
классифицируется как ErrTrustRole с сохранением причины.

Это process-local координация. Caller продолжает владеть прежним journal/source
serialization; публичный Save не получает рекурсивный journal lock. Другие
процессы и прямые filetxn transaction writers не входят в эти I/O mutex.
Штатного сброса/quarantine bypass, замены original slot или повторного Close нет.

## Windows original owner

Переиспользованы path/component helpers, InspectWindowsFile, ACL/SID policy,
eager ID comparison и существующие NtCreateFile/FileRenameInformation patterns.
Новых криптографических verifier/trust policy или recovery journal не добавлено.

- Только local-drive path, до 2048 символов и 64 directory objects, включая
  корень. UNC/device paths отвергаются. Все directory handles удерживаются без
  delete sharing; дочерние компоненты открываются относительно original parent.
  Missing directories создаются FILE_OPEN_IF; reparse/ACL/type/path/ID проверяются
  на original handles. Созданные directories не удаляются при ошибке.
- Один private temp: FILE_CREATE, nonce96bit, максимум16 подтверждённых
  no-create collisions. До следующей попытки предыдущий пустой receipt закрыт.
  Native output пишется непосредственно в заранее сохранённый record. Panic
  acquisition и неизвестный returned handle не объявляются освобождёнными.
- Полная запись chunks≤32KiB, точное число bytes, FlushFileBuffers, проверка
  size/ID/path/ACL и pinned ancestors до замены. Temp запрещает parallel writers.
- Замена через original file handle и retained destination-parent handle;
  фиксированный target basename. Нет path Rename, Ex/POSIX fallback или удаления
  прежнего target. После native success проверяется original object по новому
  имени, повторяются Flush/size/ID/ancestors; затем требуется checked Close temp
  и всех directories. Успех до подтверждённого Close не возвращается.
- Rename error с подтверждённым original object всё ещё по private имени может
  безопасно завершить cleanup. Неизвестный результат, canonical mutation с
  последующей ошибкой, native I/O panic, cleanup/Close fault удерживают original
  operation в slot. Error не считается доказательством неизменности target.
- До публикации удаляется только подтверждённый original FILE_CREATE object
  через FileDispositionInformation, затем checked Close. Путь не удаляется.
  После возможной canonical publication cleanup не угадывает местоположение.

Directory handles/identity и file Flush не доказывают durability directory entry
при потере питания. Это source protocol, не подтверждённый Windows crash contract.
Совместимость ACL, aliases/8.3/Unicode, hardlinks, sharing/AV/mapped files и native
ABI не проверена; консервативные отказы возможны. Политика доверенных SID прежняя:
current user/Admin/SYSTEM/installer. Защита от этих доверенных писателей этим не
доказывается. Нет hard I/O deadline, bound waiting callers или aggregate RSS.

## Portable compatibility

!windows сохраняет original os.File для directory/temp, проверяет identity,
полную запись, file/directory Sync и Close. Create/rename/remove остаются path
operations с race window; это не Windows relative-handle/ACL proof. Неизвестный
результат удерживает original operation. Portable поведение также не запускалось.

## Evidence и остаток

7 Go paths: gofmt exit0, final refinement5 exit0. SHA256/size и moved-path receipt:
`.artifacts/memory/stored-write-source-evidence.json`. Просмотрены текущие callers,
runtime clock/legacy bootstrap, прежний journal и cached x/sys0.44.0 signatures.
В metadata production source больше нет старого atomicWrite/os.WriteFile/
os.Rename/MkdirAll. Это source inspection и форматирование, не build/acceptance.

Tests/fixtures не добавлялись/не менялись/не запускались. Build/vet/typecheck/
parser/doctor/check/SDK/CLI/native/admin/live не выполнялись; owner env/vault/
public trust/config/log values и чужие роли не затронуты.

V2-173 продолжает [actual filetxn journal](TH-01-IMPORT-JOURNAL.md): bounded
checked readers, stage-new/target original writers, retained per-journal fault,
full-set rollback и bounded known-entry cleanup. Metadata domains сохраняют
nonempty input; journal slots отдельно поддерживают legitimate empty backups.
Это source/unverified; native directory creation/move/delete custody, межпроцессная
сериализация и directory/power-loss guarantees ещё не реализованы/не доказаны.
V2-174 добавляет original journal directory creation/relative no-replace moves
и same-handle known-child cleanup; source only. V2-175 удерживает fixed target
parent tree через whole Commit/Recover, заменяет Store.erase на original native
deletion и подтверждает отсутствие имени отдельным original open.
V2-176 добавляет общий writer scope Store/metadata, включая runtime read/derive/
save. Actual vault mutations и prepared vault final fence ещё вне scope; дальше
закрыть это покрытие, runtime checkpoint monotonic guards и coherent readers.
Native/interprocess/durability/OS/DR/resources/SDK/release acceptance открыта.
V2-177 подключает vault CAS/final Store fence, CLI и migration к writer scope;
самостоятельные writes требуют explicit profile journal binding. Следующий
source-срез — monotonic checkpoint guards, затем coherent readers/durability.
Все TH-00…08 gates остаются открытыми; public schema2 install unsupported.

## V2-178 — полномочия одиночной записи checkpoint

2026-10-28, Windows source implemented/unverified. Прежний SaveAcceptedProfile
сравнивал только delivery/epoch/global-policy authority. Version/content,
per-channel pin/revocation/retired history, issued/recovery fields и clock floor
не были полностью защищены этим writer. Это source finding, не воспроизведённый
runtime defect. Runtime writer выделен в `metadata/runtime_checkpoint.go`;
общий encoder/import acceptance остаются в существующих файлах.

| Исходное состояние | Разрешённая операция V2-178 |
|---|---|
| Есть accepted checkpoint | Только сохранение прежнего состояния либо продвижение TimeFloor. Schema, tuple/content SHA, IssuedAtFloor, LastRecoverySHA256, delivery, epoch/grant/origin/history, pins/revocations/retired и global pin policy неизменны |
| Missing/legacy numeric checkpoint | Повторное вычисление ожидаемого checkpoint из текущего installed envelope и active RoleProfile keys через PrepareAcceptance; сохранённые numeric floor, historical signature, active signer, validity и pin continuity проверяются прежним кодом |
| Нет установленного envelope / есть enrolled delivery без checkpoint | Bootstrap запрещён; переданный caller state не становится полномочием для записи |
| Ошибка/неопределённое I/O | Прежние native writer/paired slot barriers сохраняются; ошибка не даёт bootstrap/reset fallback |

Guard сравнивает весь encoded state с owned expected state, в котором изменено
только разрешённое TimeFloor. Новое сериализуемое поле также неизменно по
умолчанию. Перед сравнением нормализуются только unordered local Pins/Revoked/
Retired lists; input не мутирует, duplicate/invalid state по-прежнему отвергается.
EpochHistory не сортируется. Глобальная policy уже требует canonical sorted lists.
Exact semantic replay этих retention sets не вызывает повторную запись.

TimeFloor нельзя уменьшить; увеличение не может превышать переданное локальное
наблюдение. Public SaveAcceptedProfile использует wall-time sample внутри writer
scope; runtime bootstrap и clock observation передают свой explicit now/observed.
Это local continuity bound, не authenticated UTC и не защита от доверенного
caller, который сфабриковал observation. Trust/clock errors сохраняют категории
ErrTrustRole/ErrClockUncertain. В диск уходят те же owned encoded bytes, которые
прошли сравнение; дополнительных journal/lock namespaces не создано.

Authenticated profile/epoch/recovery/pin-policy imports продолжают менять
checkpoint через прежнюю four-target transaction. Новая граница касается
одиночного runtime writer; она не предоставляет импортных полномочий и не
заменяет final source fence. Публичная сигнатура SaveAcceptedProfile сохранена,
но произвольный bootstrap state без installed signed envelope теперь отвергается.

Evidence: `.artifacts/memory/checkpoint-guard-source-evidence.json`, exact4 Go
SHA256/sizes; gofmt4 exit0. Просмотрены обе production private call sites,
PrepareAcceptance/advanceProfile, full state schema, epoch/global-policy helpers
и writer scope. No tests/fixtures/build/compile/vet/typecheck/parser/doctor/check/
SDK/DPAPI/native/admin/live. Missing/legacy/equal/rollback/mutation/clock/order/API
сценарии требуют будущей приёмки; source guard не равен PASS.

Далее coherent read-only profile snapshots/retained identity, directory/power-loss/
restart ordering, SDK internal I/O и CleanupError UX. Прежняя vault helper/API
адаптация и все ABI/ACL/aliases/concurrency/OS/DR/resources/old-client/release/
comparative/TH-00…08 gates остаются открыты; schema2 unsupported.

V2-179 [profile snapshot](TH-01-PUBLIC-READERS.md) переиспользует whole-state guard
для read-only runtime projection и выделяет acceptance в общий value-only helper.
Runtime захватывает source внутри существующего writer, просмотр — existing-only
shared read scope; bootstrap сохраняется только после успешной проекции. Gofmt12/
source only, unverified. Initial import/export/update/recovery reader integration,
directory durability/SDK/native/API/OS/all9 приёмка остаются открыты.

## V2-185 — выбранный файл зашифрованного переноса

SaveProtectedProfileTransfer больше не вызывает os.WriteFile после диалога.
Используется один process-lifetime NewSelectedOutputSlot с прежним 2 MiB ceiling.
Existing BoundedWriteSlot/native/portable writer расширен явным WriteSelected(ctx):
selected slots не принимают обычные Write/WriteNew или journal-directory exception;
stored-state slots не принимают selected mode. New mode не используется metadata,
vault, checkpoint или journal consumers. Busy selected writer возвращает отдельную
ошибку через TryLock; uncertain acquisition/I/O/publication/Close сохраняет original
owner, а не создаёт replacement на новый path.

Writer создаёт отдельный unique temp в существующем выбранном каталоге; missing
parents не создаются. Полные chunks32KiB, original file/type/size/ID/path и parent
identity, Sync/flush до публикации; затем прежняя same-parent name replacement,
повторные flush/size/ID/parent checks и checked Close original file/ancestors.
До публикации existing target не открывается на запись и не truncates. Старый
overwrite сценарий сохранён; отказ не запускает guessed path cleanup/fallback.
Windows cleanup удаляет только original созданный temp через его handle.

Windows private inspector теперь имеет три явных policy: stored/input/output.
Exported InspectWindowsFile всегда выбирает stored и требует прежние ACL/trusted
SID/single-link; selected input policy V2-184 прежняя. Selected output удерживает
existing ancestry с specific traverse/attributes/synchronize access, не требует
trusted-store ownership/DACL и сохраняет no-reparse/final-path/nonzero FileID.
Его созданный regular temp остаётся single-link и использует GENERIC_WRITE для
FlushFileBuffers, DELETE для original-handle rename/cleanup и FILE_SHARE_READ
без write/delete sharing. Strict storage writers сохраняют прежние access/sharing,
local-drive restriction и parent creation semantics. Selected output допускает
валидированный drive/UNC root через тот же path validator, без обходного path I/O.
Основания: [FlushFileBuffers](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-flushfilebuffers),
[FILE_RENAME_INFORMATION](https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/ntifs/ns-ntifs-_file_rename_information).

Context gates выполняются до acquisition, между write chunks, до initial flush и
name publication. После публикации cancellation не прерывает обязательное
завершение flush/retirement. Final context проверяется только после Close, вместе
с error возвращаются WritePublication.Published/MayHavePublished. Это факт исхода
name operation, не доверие или успешная durability/retirement. App различает
confirmed publication и неопределённую публикацию в error text и возвращает path
вместе с error; Wails может доставить только error, поэтому сообщение само
предупреждает о сохранённом/возможно сохранённом файле. Пустой dialog path остаётся
cancel, dialog/context failures не выдаются за успех. Public string/error binding
не изменён. После успешного writer outcome нет App log callback, способного
превратить уже завершённое сохранение в позднюю ошибку.

Выбранный output — framed encrypted transfer, не новое local trust state:
ValidateProtectedTransport не аутентифицирует ciphertext/pin policy; common
import/approval/KDF/signature gates на получателе прежние. Замена создаёт новый
файловый объект с inherited directory permissions; preservation ACL/attributes/
alternate streams старого target не заявляется. Это не target-FileID compare-and-
swap: preflight Lstat/type check не устраняет race с другим writer целевого имени.
Шифротекст не следует сохранять в координированные служебные файлы. Selected-
folder adversaries/preexisting mappings/trusted owner/admin/kernel containment
не доказаны. Directory entry power-loss durability, remote/filesystem latency и
forced cancellation отсутствуют в доказательствах. Portable branch сохраняет
path create/rename/remove races и собственный file/directory Sync, без Win32 proof.

Evidence: `.artifacts/memory/selected-output-source-evidence.json`, exact5 Go
SHA256/sizes; gofmt5+refinement3+final guard1 exit0, manual current call-site/API/
ownership/outcome review. No tests/fixtures added/changed/run; no build/compile/
vet/typecheck/parser/check/SDK/DPAPI/native/admin/live/dialog/export/owner data.
API/native ABI/UNC/FAT/removable/provider/ACL/aliases/sharing/mapped-view/target
race/old attributes/cancel/concurrency/crash/OS/DR/resource/compatibility и все9
gates остаются открыты; schema2 install unsupported. Далее SDK internal I/O
receipt seam, directory/power-loss/restart ordering и postcommit diagnostics UX.

## V2-186 — завершённая запись, очистка и process quarantine

Store.Commit намеренно сохраняет logical success при последующей ошибке cleanup/
retirement. До этого CleanupError нигде не наблюдался App consumers, поэтому
последующий старт мог сначала выглядеть разрешённым, хотя оригинальные ресурсы
уже удерживались карантином. Это source finding; воспроизведения не выполнялись.

Store.QuarantineError читает только existing process-local atomic fault. Метод
не открывает файлы, не берёт native scope, не чистит journal и не подтверждает
исправность диска при nil. Он отделён от CleanupError: debris с подтверждённым
владением остаётся warning; fault original I/O/identity/retirement требует recovery.
Не всякий quarantine — конкретно Close failure, поэтому UI не угадывает причину.
Commit/recovery semantics и фиксированные on-disk targets/manifest не изменены.

Windows-private profile_storage.go — общий классификатор/смысл сообщений и reuse
существующего lifecycle. После возврата WithPreparedTransaction, вне callback,
проверяется quarantine. Benign cleanup не превращает успешную запись в отказ;
quarantine подавляет дальнейшую activation. Проверка добавлена в общий verified
prefix и actual profile/pin commits; startup recovery также замечает поздний fault.
Все четыре prefix consumers (profile import, urgent policy, export, updater)
используют существующий callback outcome, включая cache-only и no-op distinction.
Normal profile и urgent policy сохраняют факт successful commit callback до
поздних vault/quarantine/restart errors. Это контракт production callback, который
синхронно вызывает actual Store.Commit, не native durability или доверие к
произвольной injected реализации; helper/API compatibility acceptance открыта.

App lifecycle вызывается только после release journal/vault/native profile
scopes, с operationMu. При cache-only write, для которого ещё не было transition,
retireProfileStorageFailure использует actual owner через retireProfileSession.
Уже открытый transition не заменяется. finishProfileTransition сохраняет barrier,
trustBlocked и disabled retries, не публикует CoreReady и не рекомендует повторить
импорт при quarantine. Stop failure удерживает прежнего owner/protection; automatic
direct fallback не добавлен. У updater отдельный installation root/updateMu может
ещё удерживаться: просмотренный Stop path не захватывает updateMu. Полная
lock-order/native/Stop/restart приёмка не выполнялась.

Тот же GetDeliveryProfile добавляет storageFactsSchema=1, storageState и
storageWarning. Состояния not_observed / cleanup_pending / recovery_required
описывают только этот процесс, без latest cross-process или disk-health claims.
Getter не меняет lifecycle и не запускает cleanup/recovery; snapshot quarantine
снимает сведения о selectable eligibility и возвращает unavailable. Отсутствие
fault не даёт Ready/healthy или доказательство завершения конкретного импорта.
Benign warning не переписывает accepted profile identity и не отменяет commit.

Frontend contract/mapper различает старую/неизвестную схему (unknown), строгие
version/state/nonempty message/inconsistent profile случаи. Settings переиспользует
existing row/HintBox и wrapping; остаются loading/generation/unmount fences,
warning читается polite без дублирования error и без новой карточки/скрытого poller.
Существующие apply-finally events профиля/политики/экспорта обновляют facts; updater
теперь тоже emits changed. Update preview consumed при каждой apply attempt,
mounted/generation fences подавляют stale result и запрос check после unmount.
Success оставляет busy до restart. Удалены безусловные UI обещания атомарности/
защиты старых клиентов и автоматический retry advice из policy failure. Public
Wails method signatures прежние, generated bindings/fixtures не менялись.

Evidence: `.artifacts/memory/storage-outcome-source-evidence.json`, exact14 paths
(9 Go +5 frontend), SHA256/sizes; gofmt9, refinements2+1 и final9 captured exit0.
Manual source/call-site/lock/outcome/API review only. Modern Web Guidance local
accessibility §8 Live Regions использован для polite notices; нет browser proof.
Tests/fixtures/build/compile/vet/typecheck/parser/check/SDK/DPAPI/native/admin/live/
actual UI/App/owner values не запускались/не читались. Go/Wails/native ABI, race/
fault injection/process retirement/WFP/protected path/startup/recovery/crash/
directory power-loss/OS/resources/old-client/release/comparative и весь TH-00…08
остаются unverified. SDK internal I/O original receipt seam и directory durability/
restart — следующий P0; public schema2 install по-прежнему unsupported.
