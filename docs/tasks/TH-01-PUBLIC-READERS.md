# TH-01 / OFF-06 — чтение публичных файлов Windows

2026-10-28, V2-170…187. Owner: `windows`. **Source implemented, unverified**.
Продолжение [original metadata readers / SDK workspace](TH-01-UPDATE-PAIR.md).
Здесь общий I/O-компонент и его потребители; TUF/signature policy не переносится.

## Найденная проблема

`readDeliveryFile` использовал `defer f.Close()` без обработки результата и не
сверял размер/идентичность/mtime после чтения. Через него читаются update.json,
delivery.json и установленный envelope для enrolled export. Ошибка закрытия могла
не помешать передаче bytes в дальнейшую проверку. Это source finding, не
воспроизведённый сбой. SDK seed reader имел нужный ownership pattern отдельно.

## Компонент и интеграция

`windows/backend/filetxn/bounded_read.go` — `BoundedReadSlot`:

- Долгоживущий владелец одного original reader. New создаёт только bookkeeping.
  Read не ждёт занятой блокировки: `ErrReadBusy`. Неопределённое закрытие сохраняет
  resource/`ErrReadClose` до завершения процесса; замены owner/повторного Close нет.
- Constructor ceiling и per-call limit положительные и не выше существующего
  filetxn maxFileSize=16MiB. Чтение ограничено limit+1, raw size должен совпасть
  с original-handle Stat до чтения; затем сверяются ID/size/mtime/type.
- Ошибка read/stat/close/panic не возвращает raw bytes. Cleanup uncertainty не
  выдаётся за not-exist. `Retired()` означает только отсутствие original reader,
  не trust acceptance или durability; busy/quarantined возвращают false.
- Windows adapter сохраняет `WindowsRead` до Open. Используются прежние native
  type/path/reparse/ACL/single-link/FileID проверки, deny-write sharing и bounded
  chunks32KiB. Новый `acquiring` guard сохраняет неизвестный результат panic
  внутри NtCreateFile; Close не объявляет такой owner освобождённым.
- !windows adapter сохраняет os.File, делает Lstat/open identity check и checked
  Close. Он не доказывает Windows native ownership/deny-write или hard deadline
  при path race/special file/remote filesystem. Обе платформы ещё не запускались.

| Потребитель | Собственный owner | Неизменённый предел |
|---|---|---|
| delivery SDK initial seed | metadataCacheReader | 512KiB на запись |
| App update.json | publicDeliveryReader | 64KiB |
| App delivery.json | тот же App owner | 12MiB +512B |
| App enrolled export envelope | тот же App owner | 1MiB |

App и SDK имеют независимые slots. Общий код заменяет прежнюю копию cache-reader
ownership; удалены delivery/cache_reader_windows.go и cache_reader_other.go.
workspace cleanup использует `Retired()`. Pair updater сохраняет отдельный
streaming metadata lease: он удерживает ресурс дольше одного чтения и не заменён
памятным byte reader. Native acquisition guard действует и для этого потребителя.

`readDeliveryFile` требует непустые bytes после checked Close. Update sidecar
call-site сохраняет underlying error через errors.Join, включая retirement fault.
Manifest/TUF/envelope/source/clock/role checks по-прежнему обязательны; чтение
файла не даёт права применить его. Loader/export journal и App operation ownership
не изменены, callbacks под reader mutex не добавлены.

## Evidence и границы

8 Go paths: gofmt -w exit0; два obsolete platform adapters удалены. Точные SHA/size
и removed-path receipts: `.artifacts/memory/intake-source-evidence.json`.
Ручное чтение source/call sites: все три App потребителя используют прежний helper;
старых openCacheReader/closeCacheReader/sameCacheFile ссылок в Go source не найдено.
Это не build, regression или ресурсный runtime PASS.

Не добавлялись/не менялись/не запускались tests/fixtures. Не выполнялись build,
vet/typecheck/parser/doctor/check/module/install/gen/SDK/crypto/CLI/EXE/helper/
native/admin/live. Не читались реальные trust/vault/config/profile/log values;
другие роли/чаты не затронуты. Public schema2 install остаётся unsupported.

Открыты native ABI/ACL/aliases/sharing/reparse/mapped writers/SMB, точность Stat,
Close/panic faults, concurrency/busy semantics, hard I/O deadlines, OS/DR/resources/
durability/release и весь TH-00…08. Source bounds не доказывают предел elapsed time
или working set. SDK internal file resources остаются opaque, WIN-SDK-001-T открыт.

## V2-171 — stored trust/checkpoint/envelope readers

`metadata/stored_reader.go` (в V2-172 расширен в `stored_file.go`) подключает тот же BoundedReadSlot к
`ReadTrustPolicyFile`, `LoadAcceptedProfile` и `Store.LoadEnvelope`.
Три private long-lived owners: policy6MiB, checkpoint1MiB, envelope1MiB.
Повторного bare os.Open/defer Close в этих loaders больше нет.

### Параллельные вызовы и порядок блокировок

У каждого вида файла отдельный I/O mutex. Одновременные чтения одного вида
ожидают предыдущего; обычная конкуренция не создаёт ErrReadBusy/ошибку доверия.
Под mutex выполняется только bounded read/checked Close. Он освобождается до
JSON decode, CheckDeliveryBinding, вызова другого loader и работы с journal.
Helper не берёт journal, не вызывает callbacks/Store/App/SDK. Поэтому существующий
порядок caller journal → reader не получает обратного захвата. LoadTrustPolicy
освобождает policy reader до LoadAcceptedProfile; runtime/clock/import сохраняют
свои прежние journal/source fences. Это source reasoning, не concurrency test.

Не вводится кэш raw bytes/authority или динамический pool по пользовательским
путям. Неопределённый Close удерживает исходный slot до выхода процесса.
Ожидание синхронного I/O **не имеет доказанного hard deadline**; число ожидающих
API callers и суммарные allocations после чтения этим mutex не ограничены.

### Отсутствие, повреждение и локальная ошибка

| Результат чтения | Поведение |
|---|---|
| Подтверждённый not-exist после checked cleanup | Прежний absent policy / initial checkpoint / absent envelope |
| Permission/identity/size/read/Close/panic error | Ошибка; без выдачи raw bytes и без initial-state fallback |
| Пустой или некорректный JSON | Прежний strict decoder/validation отказ |
| Старый корректный decimal version.txt | Прежний ParseUint и Version, без сброса значения |
| Envelope отсутствует при retained version/minimum | Существующие runtime/ApplyVersion gates запрещают embedded fallback |

У private read helper `present=false` означает только confirmed absence.
При error `present=true` консервативно означает, что отсутствие не доказано;
публичный caller обязан обработать error до использования present. Error-first
порядок просмотрен в runtime/import/enrollment и publisher key/update CLI source.
Ошибки journal/storage/decode Store.LoadEnvelope и journal Store.LoadTrustPolicy
оборачиваются ErrTrustRole с сохранением causes. Existing trustFailureCode получает
локальный trust fault, не наблюдение сетевого канала. Сигнатуры, enrollment,
retained history/pins/clock и CheckDeliveryBinding не заменены файловыми проверками.

### Evidence и дальнейшая работа

4 Go paths, gofmt4 + final subset2 exit0; hashes/sizes:
`.artifacts/memory/trust-read-source-evidence.json`. Только чтение исходников,
call-site review и форматирование. Tests/fixtures/build/vet/parser/SDK/native/
admin/live, реальные public state/vault/keys/config не выполнялись/не читались.
Все V2-170 runtime/OS/resource/parity/durability gates остаются открытыми;
public schema2 install unsupported. Native reader может строже отвергать старые
ACL/path/hard-link arrangements; совместимость существующих установок не доказана.

V2-172 [single-file original writes](TH-01-STORED-WRITES.md) заменяет прежний
metadata.atomicWrite и объединяет I/O gates reader/writer каждого вида.
Неопределённый writer outcome запрещает чтение через ту же пару и не становится
confirmed absence; poisoned reader запрещает обход через write. Порядок блокировок
сохраняется, кодирование/доверие остаются вне I/O. Source/gofmt only, unverified.
V2-173 [actual four-file journal I/O](TH-01-IMPORT-JOURNAL.md) подключает retained
read/write slots и process-local uncertainty barrier к fixed-set Commit/Recover.
V2-174 добавляет original native journal directory creation/no-replace move и
same-handle cleanup. V2-175 добавляет fixed target parent tree на всю транзакцию,
original rollback delete и checked absence probe. V2-176 вводит writer scope для
Store/metadata read-derive-save; vault writers/final fence ещё не покрыты,
read-only multi-file snapshots не сериализованы относительно чужого процесса.
Interprocess приёмка и directory/power-loss durability остаются открытыми gates.
V2-177 использует те же bounded slots для encrypted vault, central source CAS
и final comparison внутри Store. Read-only multi-file callers пока не получают
общий snapshot автоматически; metadata/checkpoint monotonic guard ещё требуется.
Shared trust module и чужие роли не изменены.
V2-178 добавляет whole-state runtime checkpoint guard и authenticated installed
bootstrap. Read-only multi-file snapshot/retained identity приёмка остаётся открыта.

## V2-179 — согласованный снимок публичного профиля

`metadata.ProfileSnapshot` хранит закрытые копии policy/checkpoint/envelope,
прочитанные в одном scope. `TrustPolicy()` и `AcceptedProfile()` возвращают глубокие
копии. Проекция runtime после захвата работает только с этими значениями:
повторных файловых чтений, записи anchors/history/clock или enrollment в ней нет.
Проверки подписей, срока, роли, версии и retained history остаются обязательными.

`filetxn.WithRead` использует существующий `<journal>.writer.lock` и те же
original native owners, что `WithWriter`/Store. Windows открывает только имеющиеся
parents и lock через FILE_OPEN, без mkdir/create/truncate; lock handle без
GENERIC_WRITE. Shared fail-immediate byte-range lock препятствует cooperative
exclusive writer до завершения захвата. Внутри процесса действует прежний
exclusive TryLock: повторный/конкурирующий scope получает busy, а не неявный bypass.
Callback не может делать Store.Commit/Recover/WithWriter или повторный WithRead.

Отсутствующий lock/parent возвращает `ErrReadScopeUnavailable`, **не**
os.ErrNotExist и не пустой профиль. Lock должен инициализировать обычный writer
при startup/записи. Просмотр старого хранилища до такой инициализации теперь может
завершиться явной ошибкой; совместимость установок и старых тестов не проверена.
Pending journal проверяется до/после capture. При неопределённом I/O/Unlock/Close
сохраняется исходный owner и локальный барьер; частичный снимок наружу не выходит.
После successful Unlock последующий Close fault не означает удержание OS lock.
!windows даёт только локальную сериализацию, без межпроцессной гарантии.

### Подключённые пути

| Путь | Scope и результат |
|---|---|
| Store.LoadRuntimeProfile | Private capture внутри уже удерживаемого writer; повторного native lock нет |
| Store.InspectRuntimeProfile | WithRead → owned snapshot → чистая проекция; legacy остаётся unanchored |
| GetDeliveryProfile | Один снимок для keys/history/policy/clock floor/runtime facts; нет позднего смешивания файлов |
| Future-pin wake | Проекция снимка и второй coherent capture для полного сравнения исходных значений; запуск проходит обычный fresh Start |

Общий `prepareAcceptedProfile` выделен из существующего алгоритма acceptance;
`loadTrustSource` возвращает policy и тот checkpoint, с которым проверен delivery
binding. Для anchored runtime V2-178 whole-state guard теперь применяется и к
read-only результату: пересчитанная незафиксированная история отвергается.
Legacy anchor сохраняется только writer-путём после успешной проекции; ошибка
проекции не записывает bootstrap. Four-target import authority не расширяется.

Если новая pin policy ожидает применения, снимок сохраняет policy/checkpoint
facts для состояния `policy_pending`, но envelope не читает и не допускает.
`SameSource` сравнивает хеши всех декодированных полей, включая исходную schema
policy и signature envelope, плюс policy/envelope presence и факт чтения envelope.
Это не равенство raw wire bytes: whitespace/кодирование не сохраняются; missing
checkpoint и legacy numeric zero имеют прежнее одинаковое decoded представление.
Сравнение не выдаёт полномочий commit/resume и не доказывает здоровье VPN.
Снимок описывает момент захвата; последующая запись может сделать его устаревшим.

### Evidence и следующий срез

`.artifacts/memory/profile-snapshot-source-evidence.json`: exact12 Go paths,
SHA256/size, gofmt12 exit0. Ручное чтение связей capture/projection/retirement,
clone и consumers. Tests/fixtures не менялись; build/compile/vet/typecheck/parser/
doctor/check/SDK/native/admin/live не выполнялись, реальные owner values не читались.
Это **source implemented, unverified**, не подтверждение поведения/совместимости.

Далее начальные captures import/export/update/recovery. Их final writer fences
сохраняются, но они не получают coherent snapshot автоматически. Особенно важно
не брать WithRead повторно внутри Store.BeforeCommit/WithPreparedSnapshot и не
держать reader scope через SDK/DPAPI/KDF или последующий commit. Exact raw bytes
для export требуют отдельной интеграции, decoded snapshot не заменяет raw target.
Directory/power-loss/restart ordering, SDK internal I/O, CleanupError UX и все
API/ABI/ACL/concurrency/OS/DR/resources/old-client/release/comparative/TH-00…08 gates
остаются открыты. Public schema2 install unsupported; чужие роли не изменены.

## V2-180 — coherent export source и финальный барьер

`onboarding_export_source.go` объединяет public capture legacy/enrolled export.
Owned policy/checkpoint и exact envelope bytes читаются под existing-only WithRead;
envelope использует `metadata.ReadEnvelopeBytes`, max1MiB, с той же paired
stored-envelope reader/writer uncertainty gate, что LoadEnvelope. Отдельный App
reader не обходит poisoned stored reader/writer. Это V2-180a refinement. Ошибка
чтения/закрытия/retirement не возвращает частичные значения. Отдельный authority
digest остаётся для preview; source comparison включает все decoded policy fields
(schema/comments/cache), policy presence, весь decoded checkpoint и exact raw
envelope. Raw whitespace envelope учитывается; policy/checkpoint wire formatting
и absent checkpoint vs legacy zero по-прежнему не различаются.

Enrolled verification держит read scope через final source comparison и существующий
read-only PrepareDeliveredAcceptance/pin-policy value gate. SDK verification,
prefix Commit, vault и DPAPI/KDF выполняются после освобождения reader. Initial
capture, повторные проверки и перечитывание после собственного prefix commit
используют coherent capture. Final vault.WithPreparedSnapshot уже держит writer:
там вызывается явный checkInScope без повторного WithRead. Проверки private
delivery proof, approval expiry/cancel и pending policy сохранены.

Legacy facade вынесен в `onboarding_export_legacy.go`. Убраны bare os.ReadFile,
выгрузка всего vault и удержание operation/journal через DPAPI/scrypt. Под короткими
locks готовятся public source и encrypted vault snapshot; существующие active
profile signature/version/history/clock gates проверяет PrepareAcceptance без
Save. После Seal повторно берутся operation → Serialize → vault writer, сравниваются
оба источника; ObservedClock берётся после final I/O, повторяется profile validity
и closing gate. Ciphertext выдаётся только после checked scope retirement.

Оба формата используют один renderer-driven resolver ID/kind и stable slot order;
unrelated/planned/disabled/unavailable credentials не переносятся. Это может
потребовать отдельного provisioning после будущего включения канала. Legacy
bundle остаётся прежней схемы: exact bytes захватываются/сверяются локально,
RawMessage в transport может нормализовать JSON; подпись envelope canonical.
TUF exact ProfileBytes/proof не заменяются legacy remarshal.

Новый `AcceptedProfileState.CheckLegacyExport` повторно использует существующий
advanceProfile для clean destination. Если signed envelope не сохраняет все
retained channel/certificate bans, перенос отклоняется: unsigned local history
не выдаётся за portable authority. Enrolled/global-policy/epoch state требует
своего пути. Local legacy anchor не записывается. Это не перенос clock/recovery/
private-history protocol и не доказательство unseen latest/revocation freshness.

Результат prefix retention теперь явно несёт committed outcome. Ошибка позднего
reload/retirement не скрывает уже совершённый commit; authority digest больше не
используется как индикатор изменения cache/freshness. Ошибка exports с сохранённым
prefix сообщает этот факт и не возвращает transport. Native durability этим не доказана.

Evidence: `.artifacts/memory/export-snapshot-source-evidence.json`, final exact7 Go
hashes/sizes, final gofmt7 exit0 после paired-reader refinement. Только чтение source/call sites и
форматирование. Tests/fixtures не менялись; build/compile/vet/typecheck/parser/
doctor/check/SDK/crypto/DPAPI/native/admin/live не запускались; owner values не читались.
Existing-lock/legacy stricter refusal/private helper API/two-device/expiry/race/
close/resource совместимость не проверена. Всё **source implemented, unverified**.

Далее initial import/update/pin-policy/recovery captures; они не получают native
coherence автоматически. Final writer fences, exact raw proof bytes, отдельный
SDK internal I/O request, directory/power-loss/restart/diagnostics/OS/DR/resources/
old-client/release/comparative/TH-00…08 gates сохраняются. Schema2 unsupported.

## V2-181 — trust snapshot и допуск обновлений

`metadata.TrustSnapshot` захватывает только policy и связанный checkpoint через
существующий loadTrustSource под existing-only WithRead. Private owned поля,
deep-clone accessors, policy presence. Envelope не читается, runtime permission
не выдаётся: отсутствие/повреждение envelope и pending data-plane policy сами
по себе не блокируют проверку разрешённого update. Delivery/signature/clock/roles
и antirollback остаются обязательными, missing lock по-прежнему явная ошибка.

`InspectTrustSnapshot` освобождает scope перед выдачей owned values. Вызывающий
`WithTrustSnapshot` получает scope на время callback. Callback может выполнять
короткую мутацию отдельного installation domain, но не менять coordinated
profile files, брать SDK/vault/App locks или повторный profile scope. Это
уточняет смысл WithRead; source lock сам не разрешает установку кода.

`prepareVerifiedUpdate`:

1. Под local Serialize/native shared lock захватывает coherent trust/checkpoint.
2. Освобождает оба lock до чтения пакета и SDK verification. operationMu остаётся
   у caller по прежнему lifecycle-контракту.
3. Для approved prefix отдельно берёт Serialize; существующий vault/four-target
   writer/final source fence не выполняется внутри read scope.
4. Перед возвратом private verifiedUpdate повторно захватывает trust source,
   сравнивает canonical full policy плюс explicit original schema/presence и
   весь decoded checkpoint; проверяет fresh proof/role/manifest/time/closing/cancel.
   Ошибка native retirement не возвращает подготовленный результат.

Общий final guard для install/current-code/forward-recovery теперь держит native
shared source scope через purpose/version-floor gate, fresh authority check и
supplied filesystem/registry mutation. Cooperative profile writer не может
подменить ключи/checkpoint между сравнением и мутацией. Floor I/O предшествует
последнему ObservedClock; structure/signature проверяются до time sample. Actual
SDK/workers, flush и lifecycle callbacks остаются вне этого scope. Просмотрены
App pair/root/registry callbacks: native locks fail-immediate; ReadFloor не берёт
writer повторно. Это source reasoning, не native/concurrency proof.

Policy equality сохраняет comments/cache и все поля encoder, original schema
сверяется отдельно. Порядок keys нормализуется существующим encoder; raw JSON
whitespace не identity. No-op prefix не позволяет принять произвольный late
source: final snapshot сверяется с исходными/собственными ожидаемыми значениями.
Missing successful SDK progress теперь явная ошибка, без nil dereference.

Если callback уже изменил отдельную установку, а native source retirement затем
отказал, ошибка не доказывает отсутствие mutation. Existing pair outcome/partial
owners, intent/barrier и old-image restriction сохраняются у caller. Rollback/
Close/crash/resource/old-client приёмка открыта; никакого нового install support.

Уточнение V2-180 export outcome: retainDeliveryProgress может завершиться no-op.
Committed flag теперь выставляется только после вызванного successful commit
callback; поздний source reload/Close по-прежнему его не сбрасывает. No-op не
выдаётся за новую запись. Это статическое исправление ветвления, не test result.

V2-181a: CheckUpdate после potentially long payload hashing повторяет тот же
source/role/floor/time guard перед OK=true. Ошибка очищает DeliveryVerified,
root-version/deadline facts. Preview описывает момент этого gate, actual install
по-прежнему повторяет проверку. Это не lifetime lease для UI результата.

Evidence: `.artifacts/memory/update-source-evidence.json`, final exact5 Go SHA256/sizes,
final gofmt5 exit0 после preview integration. Source review/call-site trace only; tests/fixtures,
build/compile/vet/typecheck/parser/doctor/check/SDK/crypto/DPAPI/native/admin/live
не выполнялись; реальные owner values не читались. Исторические artifacts не
объявляются актуальными для изменённых source: новый inventory фиксирует V2-181.

Далее initial profile import/pin-policy/recovery captures, затем directory/
power-loss/restart, SDK internal I/O и diagnostics UX. Full TH-00…08, existing-lock/
API/ABI/ACL/aliases/sharing/floor ordering/retirement/concurrency/two-device/OS/DR/
resources/service/old-client/release/comparative gates открыты. Schema2 unsupported.

## V2-182 — coherent profile import и финальная проверка preview

`onboarding_import_source.go` хранит owned expected policy/checkpoint из общего
TrustSnapshot: full canonical policy, explicit original schema/presence и весь
decoded checkpoint. Начальный capture согласован с cooperative writer. Только
успешный собственный authenticated prefix может обновить expected policy до
подготовки acceptance; после входа в неё source заморожен, включая recovery.
Late reads не принимаются как новый источник доверия.

SDK verification выполняется после освобождения local Serialize/native scope.
Prefix retention отдельно использует прежний vault/four-target writer и final
source fence. Затем read-only bundleAuthority.prepare со всеми role/history/
clock/epoch/pin/proof gates выполняется под shared native read после сравнения
с expected source. Никаких SDK client I/O, vault encryption или profile writes
в этом scope нет. operationMu сохраняет прежнее lifecycle-владение caller.

Обычный импорт дополнительно захватывает exact installed-envelope bytes/presence
через ReadEnvelopeBytes в том же scope, где вычисляется история. Этот источник
сверяется после ожиданий и в Store.BeforeCommit; поздняя замена envelope даже
при неизменном unanchored numeric floor не проходит. Whitespace envelope тоже
учитывается; policy key order нормализует прежний encoder, checkpoint wire
formatting/absent-vs-numeric-zero не становятся новой identity.

Explicit recovery/epoch recovery старый envelope не читают: их полномочия идут
от intact checkpoint и independently authorized permit/grant. Они не маскируют
I/O/Close/decode errors старого файла под допустимую recovery ветку. Unknown old
profile консервативно считается изменённым и требует retirement, включая replay;
это изменение поведения требует будущей совместимости. Corrupt/missing trust
checkpoint по-прежнему отказ; recovery не означает reset или обход файловых
проверок реального backup/commit. Initial epoch сохраняет strict absence gate
для version/envelope и independent root/policy/target approvals.

Перед PrepareImportSnapshot(slots) и ValidateCredentials освобождён local profile
lock; DPAPI работает вне profile/native scope. Внутренний Manager mutex при
encryption не менялся. После подготовки новый WithPreparedReadSnapshot сверяет
исходный encrypted vault и public source/time/proof/intent/closing/cancel перед
возвратом preview. Requested journal обязан совпасть с immutable Manager binding.
Existing-only shared lock не создаёт lock/parents и не сохраняет credentials,
checkpoint или clock/history. Native read/Close uncertainty не выдаёт preview.
Изменение vault или public state во время DPAPI требует нового запроса.

Final fence разделён на sourceFenceInScope и внешний WithRead wrapper. Preview
вызывает первый под vault read scope; Store.BeforeCommit — под уже удерживаемым
writer после complete staging, без recursive read acquisition. После retirement
внешний gate получает отдельный reader, затем прежний four-target commit повторяет
final public+vault comparison внутри writer. Clock sample берётся после public
source I/O; explicit deadline/proof/profile/permit и closing/cancel проверяются
снова. Neither preview nor source snapshot принимает unsigned authority.

Existing WithPreparedSnapshot сохраняет writer-семантику export; общий private
helper выделен с отдельной read веткой. Imports по-прежнему используют
WithPreparedTransaction для final vault fence. No schema/target/journal rewrite,
no direct fallback/старый worker resume. Logical committed outcome и cleanup fault
не отождествляются; их UX и native recovery приёмка остаются открытыми.

Evidence: `.artifacts/memory/import-source-evidence.json`, final exact4 Go SHA256/
sizes, gofmt4 exit0. Source/call-site/lock-order review only; tests/fixtures,
build/compile/vet/typecheck/parser/doctor/check/SDK/crypto/DPAPI/native/admin/live
не выполнялись; owner values/другие роли не затронуты. Same-journal binding,
legacy recovery replay, stale preview/expiry/cancel, resource/fault/API/native/
ACL/aliases/concurrency/two-device/OS/durability поведение не проверено.

Далее initial urgent pin-policy capsule/source captures. SDK internal I/O,
directory/power-loss/restart, postcommit diagnostics, service/OS/resources/
old-client/release/comparative и весь TH-00…08 остаются открыты; schema2 unsupported.

## V2-183 — urgent pin-policy source и общий prefix outcome

Receiver в pin_policy_import.go использует тот же profileImportSource/TrustSnapshot:
coherent policy/checkpoint, explicit presence/schema, full canonical policy/cache
и весь decoded checkpoint. Expected source замораживается перед acceptance;
только собственный отдельно сохранённый authenticated prefix обновляет policy до
этого момента. Старый envelope не захватывается и не валидируется: подтверждённые
запреты могут применяться при истёкшем/повреждённом data-plane профиле.

Public preview по-прежнему не требует vault и не читает credential values. SDK
выполняется без local/native profile locks, operationMu остаётся у lifecycle
caller. После SDK и optional prefix preparation PreparePinPolicy/Apply проходят
под совпавшим native read scope. Повторяются deadline/proof/clock/context/closing;
read/Close retirement failure не выдаёт preview. Никакой записи policy/checkpoint,
root enrollment, нового signer или смены epoch в preview нет. Старые cumulative
bans, antirollback/equivocation и exact capsule approval проверяют прежние helpers.

Commit по-прежнему retire/invalidate unsafe session до сохранения новой policy
obligation. Отдельный verified prefix может остаться после ошибки body activation;
old-session/direct fallback не разрешается. Encrypted vault snapshot готовится
вне native read scope без DPAPI; fixed4 transaction сохраняет Keep для keys,
envelope и vault, меняя только accepted public checkpoint. Внешний preflight
использует WithRead, final Store.BeforeCommit — fenceInScope под уже удерживаемым
writer. Повторного native acquisition внутри final writer нет.

Общий retainDeliveryProgress теперь выполняет initial public source comparison
под WithRead **до** no-op branch. No-op не читает vault и не пишет transaction;
изменившийся source не принимается даже в этой ветке. Для write branch reader
освобождается до encrypted vault snapshot/Store; final comparison остаётся внутри
writer. Просмотрены все четыре production consumers: profile import, urgent
policy, enrolled export и updater; они вызывают helper вне native profile scope.

retainDeliveryProgressWithOutcome выделяет один общий счётчик successful commit
callback для pin receiver и export. Cache-only persistence отмечается даже при
неизменном authority digest; no-op не объявляется новой записью. Outcome передаётся
наружу вместе с error и сохраняется при позднем read/activation failure. Export
больше не содержит свою копию wrapper. Это logical callback outcome, не native
power-loss durability или доказательство поведения injected/noncooperating writers.

Evidence: `.artifacts/memory/pin-source-evidence.json`, exact4 Go SHA256/sizes,
gofmt4 + final comment subset1 exit0. Source/lock-order/call-site review only;
tests/fixtures/build/compile/vet/typecheck/parser/doctor/check/SDK/crypto/DPAPI/
native/admin/live не выполнялись; owner values/другие роли не затронуты. Missing-lock/
helper API/cache-only/no-op/late-error/cancel/expiry/source-drift/retirement/ACL/
aliases/concurrency/OS/resource поведение требует разрешённой приёмки.

Следующий найденный ingress gap — onboarding_dialogs.go: selectPinPolicyFile и
readOnboardingTransport используют bare os.Open/defer Close без checked result
и complete original-source recheck. Это public user-selected input, поэтому
trusted-store path/ACL policy нельзя переносить вслепую или ослаблять у actual
trust state. Далее bounded existing-owner intake, затем directory durability/
power-loss/restart, SDK internal I/O и postcommit diagnostics. Все9/OS/DR/resources/
service/old-client/release/comparative gates открыты; schema2 unsupported.

## V2-184 — выбранные пользователем файлы импорта

selectPinPolicyFile и readOnboardingTransport больше не используют bare os.Open
и unchecked defer Close. Они переиспользуют BoundedReadSlot через явный
NewSelectedInputSlot: два process-lifetime input-domain owners для capsule и
encrypted transport, без новой owner на каждый path/повтор. Partial acquisition
и uncertain Close удерживаются исходным reader; busy/poisoned owner не заменяется.
Никакие bytes не выходят из slot до checked Close. Error закрытия перекрывает
предыдущий not-exist/overflow; он не маскируется под отмену/слишком большой файл.

Выбранный файл — недоверенный ввод, не часть локального хранилища доверия.
Windows mode требует data/attribute/synchronize access, FILE_SHARE_READ без
write/delete sharing; original native receipt, disk/regular/no-reparse, bounded
validated path и его final name, nonzero FileID, size и mtime до/после Read
сохраняются. Владение/DACL trusted-store и единственный hardlink не требуются;
positive link count сохраняется. Exported InspectWindowsFile и обычный constructor
по-прежнему требуют trusted ACL и single-link, их production consumers не
переведены на selected mode. Это явный режим до acquisition, не fallback при
ошибке. Portable branch сохраняет Lstat→original os.File→identity/size/mtime→
checked Close, без заявления Windows sharing или forced cancellation.

GENERIC_READ включает STANDARD_RIGHTS_READ/READ_CONTROL; selected mode использует
конкретные FILE_READ_DATA/FILE_READ_ATTRIBUTES/SYNCHRONIZE. Основание:
[native access/options](https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/wdm/nf-wdm-zwcreatefile),
[access rights](https://learn.microsoft.com/en-us/windows/win32/fileio/file-access-rights-constants),
[sharing contract](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-createfilew).
Sharing не является обещанием защиты от preexisting mappings/trusted same-owner/
admin/kernel. Same-handle bounds/checks не дают подпись, свежесть или authority.
UNC syntax существующего path validator сохранён; NTFS/FAT/removable/SMB/cloud/
alias/hardlink/provider compatibility и latency не проверены. Нет hard I/O deadline.

Capsule ceiling прежний: MaxPinPolicyCapsuleSize (13 MiB +512 B при текущих
delivery constants), transport — MaxProtectedTransferSize (2 MiB). Typed
ErrReadTooLarge возникает только при известном превышении размера; original
Close failure остаётся ErrReadClose. Пустой ввод — BadFormat. Exact raw bytes
проверяются прежним common framing/authentication до normalization; codec/KDF/
подписи/approval/enrollment здесь не дублируются. Context проверяется до/после
синхронного I/O, transport — ещё и перед возвратом; это publication gate, не
отмена native I/O. Production LoadOnboardingFile использует context variant;
старый private readOnboardingTransport signature сохранён. Dialog error теперь
возвращается caller, пустой path остаётся отменой без import.

Evidence: `.artifacts/memory/selected-input-source-evidence.json`, exact6 Go
SHA256/sizes и gofmt; manual source/call-site/ownership review only. Existing
test source прочитан для API compatibility; tests/fixtures не менялись и не
выполнялись. Build/compile/vet/typecheck/parser/check/SDK/DPAPI/native/admin/live,
реальные dialogs/owner files не запускались/не читались. Boundary/error/retirement/
cancel/concurrency/ABI/filesystem/OS/resource поведение не подтверждено.

Следующий P0: SaveProtectedProfileTransfer всё ещё пишет bare os.WriteFile после
Lstat. Нужен отдельный selected-output owner с partial-acquisition/write/Sync/
checked Close и явным outcome без ослабления trusted metadata writers. Затем
directory power-loss/restart, SDK internal I/O и postcommit diagnostics UX.
Все9/API/old-client/service/DR/release/comparative gates открыты; schema2 unsupported.

V2-185 [selected encrypted output](TH-01-STORED-WRITES.md): SaveProtectedProfileTransfer
переиспользует original bounded writer с отдельным contextual selected-output
mode, existing-only parents, unique temp/full writes/flush/publish/checked Close,
retained uncertainty и confirmed/possible publication outcome. Stored/input/output
inspector policy теперь explicit typed, strict exported/metadata/vault/journal
requirements прежние. Public Wails signature прежняя, поздняя ошибка не обещает
отсутствия файла. Source/gofmt5+3+1/exact5 hashes only; tests/build/SDK/dialog/native
не выполнялись. Target races/attributes/filesystem/API/OS/durability/resources/all9
открыты; directory durability/restart/SDK internal I/O/diagnostics UX далее.

V2-186 [postcommit process facts](TH-01-STORED-WRITES.md): CleanupError warning
отделён от original-resource quarantine через новый memory-only QuarantineError.
GetDeliveryProfile публикует versioned process-local storage facts без cleanup/
lifecycle effects; quarantine не оставляет selectable eligibility. Actual commits
и все4 verified-prefix consumers сохраняют callback outcome и не дают fresh
activation при quarantine. Lifecycle выполняется после profile scopes, benign
debris не отменяет commit. Settings/mapper/HintBox и apply-finally refresh reused;
unknown старого backend не становится healthy. Source/gofmt9+2+1+9/exact14 paths
only; no tests/build/typecheck/SDK/browser/native. SDK internal I/O/durability/
restart/API/OS/resources/all9 gates остаются открыты; schema2 unsupported.

V2-187 [SDK local storage seam](TH-01-SDK-STORAGE.md): active verifier больше не
создаёт native metadata workspace. Полный pinned updater source адаптирован только
в local I/O boundaries под required Storage, official types/crypto/workflow прежние.
Owned bounded memory cache, exact bytes/genuine miss/sticky error/target-cache deny;
same global owner и legacy retained I/O barriers не обходятся. Adapter проверяет
store fault после SDK entrypoints/до proof, context ещё раз после retirement;
поздний отказ оставляет только verified restriction prefix. Source provenance/
licenses/ADR/exact diff + gofmt5+3+1+1 only, no SDK/build/tests/native. SDK parity/
whole-I/O/CPU/RSS/deadline/durability/restart/API/OS/all9 открыты, schema2 unsupported.
