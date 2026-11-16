# TH-01 / OFF-06 — I/O и барьер журнала импорта

2026-10-28, V2-173…177. Owner: `windows`. **Source implemented, unverified**.
Продолжение [одиночных stored writes](TH-01-STORED-WRITES.md). Изменён actual
filetxn.Store, используемый Windows import/recovery, retained delivery prefix,
pin-policy import и Windows publisher. Межпроцессная/OS-приёмка не выполнена.

## Проблема

Журнал использовал bare os.Open с unchecked deferred Close, отдельные stdlib
stage/target writers и recursive RemoveAll. Только существование journal path
служило барьером. Ошибка directory move не доказывала неизменность namespace;
автоматический rollback/cleanup мог продолжиться при неизвестном результате.
Это source findings; fault scenarios не воспроизводились.

## Один fixed-set протокол

Сохраняются 1…8 caller-supplied targets; Windows import использует фиксированные
keys/envelope/version/vault. Новый журнал/manifest schema не добавлен. Names и
paths повторно задаёт caller, данные manifest не выбирают назначения. BeforeCommit
повторяет прежний source/validity fence после staging полного набора и до барьера.
Восстановление проверяет **все** old backups до первого изменения target.

| Шаг | Source protocol V2-173 |
|---|---|
| Начало | Один operation gate на журнал; concurrent Commit/Recover возвращает ErrTransactionBusy; caller сохраняет внешний Serialize/vault порядок |
| Stage | Один сохранённый directory receipt до создания; bounded original reader и new-only writer на old/new/manifest |
| BeforeCommit | Все file owners уже закрыты; callback не выполняется под reader/writer mutex |
| Публикация барьера | Проверка source directory ID и отсутствия destination, move, checked destination snapshot с тем же eager ID |
| Targets | Полные Write/Flush/identity/original Close через journal target writer; Keep пропускает замену |
| Ошибка с подтверждённым retirement | При опубликованном барьере — прежний rollback полного набора |
| Неизвестный original retirement или move/remove outcome | Сохранённый process-local fault, прекращение mutation/rollback/cleanup; Check отвергает состояние даже при отсутствующем journal name |
| Commit point | Барьер убран после всех target writes; последующая ошибка очистки не превращает completed commit в uncommitted result |

При rollback old target с Existed=false восстанавливается удалением; zero-length
old/new file остаётся отдельным состоянием от отсутствия. Keep.new также может
быть пустым. Для journal slots разрешён empty output; публичный metadata writer
V2-172 по-прежнему отвергает пустой input.

## Original I/O и учёт

Прежний Serialize registry хранит journalDomain: caller serial mutex, отдельный
Commit/Recover operation gate, atomic immutable fault и долгоживущие slots для
payload reader, directory snapshots, stage-new writer и target writer. Новый
Store value и отсутствие path не выбрасывают original receipt. Поиски используют
прежний abs/clean/case-fold key; aliases и interprocess им не решаются.

Domain не берёт caller serial mutex рекурсивно. Порядок product caller остаётся
journal Serialize → vault → Store operation → I/O slot. Check читает atomic fault
без operation lock; BeforeCommit может использовать прежние metadata loaders.
Внутренний rollback вызывает private recover без повторного exported gate.

Чтение old/backup/new остаётся не больше16MiB на файл, manifest — теперь4KiB.
Генерируемый compact manifest для8 names/digests укладывается в этот предел;
произвольно раздутые локальные JSON отвергаются. Проверяются точный размер,
original ID/mtime/type и Close; raw bytes не выходят при ошибке. Close uncertainty
не становится missing. Directory snapshot использует тот же lifecycle и eager
Windows FileID; os.SameFile не получает custom Windows FileInfo.

WriteNew использует native RenameInformation.ReplaceIfExists=false. Portable
adapter использует new-only Link и проверенное удаление private имени; fallback
на overwrite не добавлен. При ошибке Link наличие private source не доказывает
отсутствие canonical target: это отдельно учтено. Native writer bounds, исходный
handle, full chunk/Flush/identity/Close и quarantine взяты из V2-172.

Данные new при cold recovery никогда не применяются: их наблюдение нужно только
для записи ограниченного набора cleanup receipts. Pure content/read error новых
байтов может оставить debris без запрета восстановления аутентифицированных old;
неопределённый resource retirement всё равно останавливает операции.

## Cleanup после commit/rollback

RemoveAll удалён. На domain хранится максимум один cleanup directory receipt и
до17 фиксированных names с size/SHA256, без удержания payload bytes. Перед
удалением проверяются directory eager ID и exact file bytes; обхода дерева нет.
Неизвестные entries не удаляются. Ошибка удаления сохраняет неопределённый receipt
без повторного guessed-path delete. Новая mutation не создаёт очередной stage,
пока предыдущий receipt не завершён. Ошибка доступна через Store.CleanupError.

Completed commit/rollback сохраняет свой результат. Если позднее I/O-очистка
оставила **resource** uncertainty, Check блокирует дальнейшее использование;
обычные retained debris сами по себе не отменяют уже принятую целую запись.
Автоматическое отображение CleanupError в App/CLI пока не подключено.

## Остаток на V2-173

**Это ещё не полное native directory custody.** Stage создаётся через
MkdirAll/MkdirTemp; directory moves, target erase и cleanup — path operations.
Snapshot/lookup до и после move не закрывает race между ними, не устанавливает
creator receipt и не гарантирует kernel no-replace. Random recovery destination
ограничен16 попытками/nonce96bit, но не является reservation. Консервативный отказ
создания/move может удержать fault до выхода процесса даже при устранимой причине.

Следующий срез: concrete original native directory/creation/move/delete owner
для fixed journal namespace и target parents. Native output сохранять до
acquisition; move/delete — через retained original objects, actual path/ID и
точные success/failure/Close receipts. Сохранить один journal/полный backup set,
BeforeCommit и caller authority; не выводить trust из manifest hash или path.
После этого — межпроцессная координация, directory/power-loss ordering, restart/
crash recovery, runtime checkpoint monotonic guard и SDK internal I/O receipts.

Registry не имеет новой global path quota/eviction: удаление потеряло бы original
owners. Waiting time, input/backup allocations, RSS и orphan stages после разных
перезапусков этим срезом не ограничены. Native ABI/ACL/aliases/sharing/rename/Close,
existing fault/API expectations, OS/DR/old-client/service/release/comparative и
весь TH-00…08 остаются непроверенными; public schema2 install unsupported.

## Evidence

11 Go paths, gofmt11 + refinement3 + final2 exit0; exact SHA256/size:
`.artifacts/memory/journal-source-evidence.json`. Просмотрены production callers,
BeforeCommit source fences, Keep/absence, full-backup restore, error/lock order и
обе platform adapters. Standalone private atomicWrite helper сохранён для прежних
fault-injection callers; production Store использует собственный domain writer.

Tests/fixtures не добавлялись/не менялись/не запускались. Build/vet/typecheck/
parser/doctor/check/SDK/CLI/native/admin/live не выполнялись. Реальные owner
env/vault/config/trust/log values и чужие роли/чаты не затронуты. Это source review
и форматирование; runtime correctness, resource bounds и durability не доказаны.

## V2-174 — исходный native каталог и очистка

Windows journal теперь удерживает `nativeJournalDirectory` внутри прежнего
cleanup receipt **до** Create/Open. Повторное открытие Store не теряет исходные
handles. Переиспользуются filetxn bounded writer: relative NtCreateFile,
InspectWindowsFile/ACL/eager ID и checked Close. Core/update packages не
импортируются и второй журнал/authority не появляется.

- Parent chain удерживается без delete sharing. Новый stage создаётся относительным
  FILE_CREATE: nonce96bit, не больше16 подтверждённых no-create collisions;
  каждый collision закрывается до замены receipt. Cold recovery использует
  FILE_OPEN и не создаёт отсутствующих родителей.
- Каталог stage/journal получает собственное DELETE access без delete sharing.
  Этот исходный handle сохраняется через stage → journal → stage/recovered.
  Destination-parent также исходный; native rename запрещает замену занятого
  назначения. Ошибка/panic оставляет sticky fault и оба предполагаемых пути;
  наблюдение исходного handle после ошибки — только evidence, не разрешение retry.
- Stage writer явно разделяет DELETE access только для точного собственного
  каталога. Сам journal owner продолжает запрещать чужие delete opens. Обычные
  metadata writers не получают этого исключения. Reader/writer slots должны
  подтвердить Close до rename; directory identity проверяется через исходный
  handle, без release/reopen каталога между фазами.
- Cleanup открывает только записанные фиксированные child names относительно
  original directory. Один retained child handle имеет read+DELETE без write/
  delete sharing. На нём проверяются exact size/SHA256, FileID/mtime/ACL/path и
  EOF; чтение порциями32KiB, не больше16MiB. FileDisposition и checked Close
  относятся к этому же файлу. Нет окна между закрытием проверенного файла и
  удалением другого объекта по пути.
- Последний каталог удаляется только через original handle и только пустым.
  STATUS_DIRECTORY_NOT_EMPTY сохраняет debris/owner и запрещает новый stage;
  неизвестные children не обходятся/не удаляются. Неопределённые delete/Close
  сохраняют исходные receipts и process-local fault. Подтверждённый собственный
  Close не доказывает немедленное физическое удаление при чужих открытых handles.

До64 ancestor receipts +1 directory +1 cleanup child; свежий stage оставляет
один ancestor slot для существующего bounded writer. Это предел собственных
receipts, не process RSS/времени syscalls/глобального числа journals/orphans.
Portable branch сохраняет прежние path operations и Windows-гарантий не получает.
Полный fixed backup/BeforeCommit/Keep/absence/rollback и отдельный committed/debris
outcome остаются прежними. Private fault callbacks сохранены как seams, но их
прежние path-based native expectations не считаются совместимыми без проверки.

**Следующий P0 после V2-174 (продолжен ниже):** fixed target parents нужно удерживать через целую транзакцию;
`Store.erase` всё ещё path-based. Нужен original target delete owner для возврата
к отсутствующему old target. Затем cross-process writer coordination, directory/
power-loss/restart ordering, runtime checkpoint monotonic guards, SDK internal
I/O receipts и CleanupError UX. Возможность открыть/удалить каталог — не право
принять профиль; authority остаётся у прежних caller fences.

Evidence V2-174: `.artifacts/memory/journal-native-source-evidence.json`, exact10
Go paths SHA256/size, gofmt10 + refinement2 exit0. Source review включил
acquisition/panic/collision, sharing с существующими readers/writers, original
rename/cleanup/Close, холодный recovery и portable factory. Проверены объявления
констант в установленном x/sys0.44.0; исполнения продукта не было.

Параметры native open/rename сверены с первичными описаниями Microsoft:
[NtCreateFile](https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/ntifs/nf-ntifs-ntcreatefile)
и [FILE_RENAME_INFORMATION](https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/ntifs/ns-ntifs-_file_rename_information).
Это источник семантики API, не подтверждение корректности нашей ABI/ОС-реализации.
Tests/fixtures/build/vet/SDK/native/admin/live не добавлялись и не запускались.
Все native compatibility/aliases/ACL/sharing/mapped-file/concurrency/OS/DR/
resources/old-client/service/release/comparative/TH-00…08 gates открыты;
public schema2 install остаётся unsupported.

## V2-175 — каталоги целей через всю транзакцию

В actual Store подключён один `journalTargets` owner. На Windows он строит
дерево исходных parent handles для caller-fixed paths; общие предки открываются
один раз, в том числе при разных каталогах metadata/vault и разных local drives.
Каждый native output записывается в retained tree до NtCreateFile. Сохранены
проверки ACL, reparse, path, FileID и запрет delete sharing. Root/path authority
не берётся из journal manifest. Portable factory возвращает nil и сохраняет
прежний путь без заявления native гарантий.

| Граница | Source behavior V2-175 |
|---|---|
| Commit | Validate fixed targets → Check journal/old debris → pin target tree → полный backup/stage → BeforeCommit → повторная проверка дерева → journal publication → target writes → проверка дерева → commit point |
| Automatic rollback | Сохраняет тот же owner и те же target bindings; не отпускает/не открывает родителей заново между неудачной записью и rollback |
| Cold Recover | Отсутствующий journal возвращается без создания target directories. Все old backups проверяются до приобретения target tree и первой restore mutation |
| Missing parents | FILE_OPEN_IF относительно исходного parent; создание только после соответствующей границы выше. Empty directories могут остаться после отказа; Keep сохраняет содержимое/отсутствие **файла**, не обещает отсутствия каталога |
| Exit | Journal rollback/cleanup → checked Close target tree в обратном порядке → release operation gate. Unknown acquisition/Close сохраняет original owner/barrier |

Список ограничен8 targets, глубина каждого пути — прежние64 parent slots,
совокупно не больше512 уникальных parent receipts. Общие пути дедуплицируются;
каждое target I/O проверяет только свою цепочку, а phase gates — целое дерево.
Это ещё не измерение handles/RSS/latency и не hard syscall deadline.

`Store.erase` в Windows теперь использует relative original read+DELETE handle.
Это возврат к отсутствующему старому файлу, поэтому permission исходит из
caller-fixed target и **полного** проверенного old набора. Новый structural guard
отвергает Existed=false с непустым old backup до любой target mutation.
Одна `nativeFileDeletion` реализация обслуживает и этот rollback, и journal
cleanup; cleanup по-прежнему дополнительно требует exact size/SHA256. Обе ветки
проверяют original ID/size/mtime/ACL, используют FileDisposition и checked Close.

После Close отдельный retained inspect-only FILE_OPEN относительно того же
parent подтверждает отсутствие имени. Delete-pending/другой появившийся файл
не считаются успехом; новый объект не удаляется. У absence probe свой native
output, он не перезаписывает исходный delete/Close receipt. Unknown probe Close
также сохраняет owner. Это observation в момент проверки, не защита от будущей
записи другим процессом: cross-process writer coordination ещё требуется.

Успешный commit/rollback и последующая ошибка освобождения родителей остаются
разными фактами. Commit point уже нельзя отменить ошибкой Close; Store.Check
получает sticky fault и блокирует новые операции/чтения, CleanupError сохраняет
причину. На неуспешном пути close fault добавляется к возвращаемой ошибке.
Отображение postcommit diagnostics в App/CLI ещё не подключено; обычный fresh
runtime loader обязан пройти прежний journal/trust gate перед Start.

**Далее:** межпроцессный original writer lock для реальных journal и standalone
metadata writers с сохранением caller/source-fence/lock order; directory/power-loss
ordering/restart recovery, runtime checkpoint monotonic guards, SDK internal I/O
и CleanupError UX. Parent custody не блокирует запись содержимого другим процессом,
не делает четыре target writes атомарным чтением и не заменяет этот lock.

Evidence: `.artifacts/memory/target-native-source-evidence.json`, exact8 Go hashes/
sizes; gofmt8 + refinement1 + comment1 exit0. Просмотрены actual import/prefix/pin-policy/
publisher callers, defer/rollback order, partial acquisition/Close, sharing,
portable factory и сохранение полного backup set. Source-only; no tests/fixtures/
build/vet/typecheck/parser/doctor/check/SDK/CLI/native/admin/live. Все прежние
ABI/ACL/aliases/sharing/mapped-file/concurrency/OS/DR/resources/old-client/service/
release/comparative/TH-00…08 gates остаются открыты; schema2 unsupported.

Проверка описания API: [FILE_DISPOSITION_INFORMATION](https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/ntddk/ns-ntddk-_file_disposition_information)
и [NtSetInformationFile](https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/ntifs/nf-ntifs-ntsetinformationfile).
После disposition на исходном handle разрешено закрытие; дополнительное наблюдение
имени выполняется отдельным open. Документация не доказывает нашу runtime-реализацию.

## V2-176 — общий writer scope для журнала и metadata

`Store.Commit/Recover` и standalone metadata writers используют один writer
scope на journal path. Отдельный process-local TryLock возвращает `ErrWriterBusy`
при конкуренции/повторном входе; implicit «уже захвачено нашим процессом» bypass
нет. Windows scope удерживает original parent chain и zero-byte файл
`<journal>.writer.lock`, открытый relative FILE_OPEN_IF без truncation/delete
sharing. Путь lock зарезервирован: Store не принимает его как target.

`LockFileEx(EXCLUSIVE|FAIL_IMMEDIATELY)` захватывает один байт с offset0;
`UnlockFileEx` снимает тот же диапазон. Output handles, lock/unlock OVERLAPPED
и partial acquisition сохранены в owner до native вызовов. Только известная
lock contention допускает обычное закрытие и новый вход. Unknown acquire/unlock/
Close остаётся sticky fault с original receipt; lock file не удаляется ни при
нормальном выходе, ни по PID/возрасту. Успешный Unlock уже отпускает OS lock,
поэтому последующая Close uncertainty означает retained receipt/local barrier,
а не обещание продолжать удерживать OS lock. !windows имеет только local gate.

| Путь | Граница writer scope |
|---|---|
| Store.Commit | После pure shape validation, до Check/cleanup/staging; через BeforeCommit, commit/rollback, cleanup и checked target-parent Close |
| Store.Recover | При наличии journal/debris — до повторного Check и restore; no-journal/no-debris остаётся без создания файла/каталогов |
| SaveKeys / SaveEnvelope / SaveAcceptedProfile | Свежие проверки исходного состояния, encode и original write под scope |
| LoadRuntimeProfile | Весь read/derive/legacy checkpoint save; private save не захватывает scope повторно; ошибка выхода не выпускает runtime view |
| RecordRuntimeClockObservation | Local Serialize → native writer → fresh profile/policy identity → derive → private save |

Порядок: caller Serialize/vault mutex → Store operation (если нужен) → writer
gate/native lock → краткие I/O gates. BeforeCommit может читать source, но не
вызывать writer entrypoints. Его ErrReadClose/ErrWriteRetirement сохраняют весь
scope и запрещают cleanup/rollback. Completed Store transaction и ошибка
освобождения после commit остаются разными outcomes через Check/CleanupError.
Native file open/inspection/Close переиспользуют existing bounded writer.
LoadRuntimeProfile теперь может создать lock/его parent directories даже без
профиля; InspectRuntimeProfile остаётся read-only и не даёт coherent multi-file
snapshot относительно чужого процесса. Lock coordination не является trust CAS.

**Покрытие неполное:** actual `secretvault.Manager` пока использует bare
load/mutate/save вне scope. `WithPreparedSnapshot` сравнивает source **до** входа
Store в native lock; final import fence пока не повторяет vault comparison.
Следующий срез — общий scope для всех vault mutations с проверкой исходного
состояния внутри него и повторным vault fence перед публикацией журнала. Один
lock только вокруг save не предотвращает потерю чужого обновления.

Full checkpoint version/pin/history/time monotonic guard, coherent read-only
snapshots, directory/power-loss/restart ordering, SDK internal I/O, postcommit UX
и приёмка old clients остаются открыты. Все TH-00…08/ABI/ACL/aliases/OS/DR/resource/
release/comparative gates непроверены; public schema2 install unsupported.

Evidence: `.artifacts/memory/journal-lock-source-evidence.json`, exact11 Go
hashes/sizes, gofmt11 + comment1 exit0. Только исходники и форматирование;
tests/fixtures/build/vet/typecheck/parser/doctor/check/SDK/native/admin/live не
добавлялись и не запускались. Семантика synchronous fail-immediate и exact range
сверена с [LockFileEx](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-lockfileex)
и [UnlockFileEx](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-unlockfileex).
Это не межпроцессная или ОС-приёмка реализации.

## V2-177 — vault CAS и final import fence

Закрыты source-пути, оставшиеся вне общего writer scope в V2-176. `Manager.save`
теперь повторно читает exact encrypted source под profile journal lock,
сравнивает presence и SHA256 исходных байтов, затем публикует bounded original
write. Stale preparation возвращает `ErrSourceChanged`, без overwrite/retry/
merge. Отсутствие и пустой файл различаются. Receipt остаётся private, не входит
в JSON/лог/UI и не даёт import authority. Exact unchanged bytes не переписываются.

Один central save покрывает Seed, Save, AddSave, Remove, Verify, Reveal и
ImportSlots. DPAPI/format preparation остаются вне native writer scope; это
optimistic CAS, а не обещание держать OS lock в течение всего шифрования.
Reveal возвращает plaintext только после успешного сохранения audit. Verify
больше не теряет ошибку сохранения failed status и не выдаёт несохранённую Meta
при таком отказе. Сами DPAPI primitives не изменены и не запускались.

`storage.go` переиспользует filetxn.BoundedReadSlot/BoundedWriteSlot с пределом
16MiB, совпадающим с journal target. Registry по canonical vault path сохраняет
original readers/writers через новые Manager values; uncertain write блокирует
read, uncertain reader — write/recovery. I/O mutex не содержит JSON/DPAPI/Manager/
journal callbacks. Bare ReadFile/WriteFile/fixed `.tmp`/rename-remove из vault
убраны. Registry cardinality, aggregate RSS и native syscall time не ограничены
этим пределом; Windows ACL compatibility/portable races остаются непроверенными.

`RecoverImport` привязывает Manager/path к одному explicit profile journal до
первой mutation. Rebind отвергается; failed recovery сохраняет binding/barrier.
Unbound writes возвращают ErrJournalRequired, отдельного fallback lock namespace
нет. App уже выполняет bind/recovery до migration/Seed. `vaulttool` теперь также
проверяет journal: default — current-user metadata directory, для другого профиля
нужен `-metadata-dir` перед command. CLI не восстанавливает journal по угаданным
targets. Между процессами совпадение profile journal — контракт конфигурации;
другой произвольно выбранный journal и старые writers не становятся безопасными.

| Путь | Изменение source V2-177 |
|---|---|
| Profile import / retained delivery prefix / pin-policy import | Новый WithPreparedTransaction сохраняет vault mutex и оборачивает Store.BeforeCommit: vault source comparison выполняется **после** staging полного набора, под native writer; прежний public/checkpoint/deadline fence сохранён |
| Custom commit seam | Success без вызова final fence отвергается; callback синхронный, Store private, прежний hook восстановлен после выхода |
| Delivered export | WithPreparedSnapshot теперь read-only final check под тем же writer scope; Store mutation из этого callback запрещена |
| Legacy migration | Bound Manager читает checked original encrypted source и пишет new-only в отсутствующий target; исходный legacy файл сохраняется, existing/corrupt target не заменяется |
| Startup | Ошибка migration/Seed передаётся в существующий storage-ready/auto-connect gate; ошибка не превращается в разрешение запуска |

Порядок locks сохраняется: Serialize (где нужен) → Manager.mu → Store operation
(если нужен) → journal writer → короткий vault I/O. Никто не захватывает writer
повторно на том же пути. Public source fence/SDK callbacks не перенесены в I/O
mutex. Four targets, Keep/absence, полный backup validation и completed Store
outcome vs retirement fault остаются прежними.

**Совместимость требует приёмки:** прежний test helper создаёт unbound Manager;
он потребует explicit journal binding по новому контракту. Tests/fixtures не
изменялись и не запускались. Это source API change, не зелёный regression result.
Unbound read-only Manager не знает чужой journal до binding; production App/CLI
bind выполнен, известный в процессе binding нельзя обойти новым Manager.

Далее — full monotonic guards одиночного checkpoint writer (version/content/
pins/revocations/history/time), coherent read-only profile snapshots, native
directory/power-loss/restart ordering, SDK internal I/O и CleanupError UX.
Cooperative CAS не является multi-file atomic snapshot, crash/power-loss proof
или защитой от старого/noncooperating writer. Все TH-00…08/ABI/ACL/aliases/OS/DR/
resources/old-client/release/comparative gates открыты; schema2 unsupported.

Evidence: `.artifacts/memory/vault-writer-source-evidence.json`, exact9 Go
hashes/sizes, gofmt9 exit0. Только source review/форматирование; no tests/fixtures/
build/compile/vet/typecheck/parser/doctor/check/SDK/CLI/DPAPI/native/admin/live.
Значения owner vault/config/env/logs не читались; другие роли не изменены.

V2-178 продолжает [runtime checkpoint guard](TH-01-STORED-WRITES.md): одиночный
writer сохраняет всю accepted identity/history, разрешает только monotonic
observed TimeFloor, а legacy bootstrap повторно выводит из установленного
подписанного envelope. Import/journal authority не менялись. Source/gofmt4 only;
coherent readonly snapshots/durability/SDK/API/native/OS/all9 gates остаются открыты.

V2-179 [profile snapshot](TH-01-PUBLIC-READERS.md) добавляет existing-only WithRead
на том же native lock: shared byte range, исходные owners, checked retirement,
без создания lock/parents. Runtime/profile facts/pin wake используют owned values.
Bare loaders и initial import/export/update/recovery captures не становятся
coherent автоматически; повторный read scope внутри writer запрещён. Source/
gofmt12 only, API/native/OS/all9 unverified, schema2 unsupported.

V2-182 [profile import source](TH-01-PUBLIC-READERS.md) now captures initial
policy/checkpoint coherently and derives history under a native read scope.
Ordinary imports bind exact original envelope bytes/absence through final
BeforeCommit; recovery deliberately needs only intact checkpoint/authority.
SDK/DPAPI work runs outside profile locks. Preview repeats public+encrypted-vault
source under explicit same-bound-journal existing-only read scope. Outer source
read wrapper and already-held writer callback are distinct, no recursive scope.
Fixed4 targets/Keep/full backups/vault fence unchanged; source/gofmt4 only.
Native/outcome/recovery/API/durability/OS/all9 acceptance remains open.
