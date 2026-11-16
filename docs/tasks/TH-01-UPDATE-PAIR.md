# TH-01 / OFF-06 — подписанная пара EXE/helper и custody старого App

2026-10-26, V2-152…169. PC/Windows source, **unverified**. Тесты/fixtures не
добавлялись/не менялись/не запускались; compile/build/vet/check/module/gen/SDK/
crypto/codec/product CLI/helper/EXE/installer/native/network/live не выполнялись.
V2-152 `gofmt -w`16, V2-153 `gofmt -w`8, V2-154 `gofmt -w`20 Go-файлов exit0 — только форматирование. Реальные owner EXE/key/
certificate/env/vault/runtime config/logs не читались.

V2-155: native original root/ancestor + cross-process writer owner, shared image
ACL/path/identity primitives и App integration для pair/legacy/activation source;
`gofmt`9 unique Go-файлов exit0, не compile/native/OS proof.

V2-156: actual App root-bound directory custodian, bounded8 native stage/pending/
archive objects + original-handle move/private-ID rebind, partial cleanup/archive
receipt; `gofmt`14 unique Go-файлов exit0 only. Public install остаётся unsupported.

V2-157: prospective signed image + original isolated code-acceptance worker,
startup actual SDK gate reuse, activation protocol2, explicit unfinished GUI
ownership-transfer gate; `gofmt`15 unique Go-файлов exit0 only. Worker/SDK/native
не запускались. Проверка кода — часть будущей runtime-транзакции, не выполненный
development test; GUI readiness и public schema2 install остаются незавершёнными.

V2-158: pre-Create original resource owner в прежнем общем spawner; partial
Job/pipe/duplicate/attribute allocation сохраняются. Preparation close до Resume,
never-created cleanup без fake exit, sticky Go File.Close uncertainty; `gofmt`5
unique Go-файлов exit0 only. GUI coordinator/lifetime/readiness ещё не реализованы.

V2-159: same-EXE GUI bootstrap, original Job/parent handle lifetime handoff и
post-parent SDK; mounted Vue nonce receipt + startup/navigation/intent barriers,
actual activation gate до network producers. Protocol3. `gofmt`16 Go exit0 +2
frontend source edits; no compile/typecheck/renderer/native acceptance. Public
schema2 install unsupported до native durability/crash/recovery/OS/release gates.

V2-160: bounded original regular-payload owner и native handle-based apply/
rollback вместо os.Rename; retained writers/readers и checked Sync/Close reuse.
`gofmt`13 unique Go exit0 only; source не доказывает Windows rename/flush/loader/
power-loss behavior. Durable control publication/late recovery остаются открыты.

Продолжение [V2-151 handoff](TH-01-RESTART-HANDOFF.md), исходная полная цель
TH-00…08 сохраняется. V2-152 — intake/issuer и process-local containment;
V2-153 — actual streaming filesystem stage/apply/rollback + persistent blocking.
V2-154 связывает App/private-SDK и current-code startup activation; **публичная
установка schema2 остаётся недоступной** до native installer/child ownership.

## Фактические основания и переиспользование

- `backend/update` — один Manifest/Canonical/Verify/DecodeStrict и bounded regular
  file verifier. Schema1 подписывает один EXE, `updateapp.go` меняет только его.
- `tools/updatetool` — реальный issuer, `deliverypack.CheckApplicationArtifact`
  и repository используют тот же update decoder и роль update. Новый формат
  проходит через прежний raw-artifact TUF target, не через второй trust verifier.
- `tools/package.ps1` уже строит helper до main, передаёт его SHA через ldflags
  и проверяет staged/destination sibling. Это build source contract; packaging
  не выполнялся, pin/architecture не объявлены подтверждёнными.
- `publisherfiles.PublishNewStreamChecked` — готовый streaming/sync/close/new-only
  hard-link publisher. Новый issuer переиспользует его вместо overwrite helper.
- `filetxn` держит metadata Data в памяти и ограничивает файл16MiB. App payload
  допускается до1GiB. Увеличение metadata limit не заменит binary transaction;
  нужен отдельный fixed-two-target streaming journal, без journal-chosen путей.

## Schema2 и общий decoder

Schema1 сохранена для старых артефактов/потребителей. Новые `target` и
`native_helper` omitempty при её обычном нулевом состоянии, не меняют прежний
порядок canonical fields. Это source compatibility intent; crypto/parity не
исполнялись. Schema1 с pair fields отвергается, schema2 не понижается до single EXE.

Schema2 фиксирует:

- `target=windows/amd64`, основной `file=snowden-system.exe`;
- `native_helper`: exact `file=snowden-native-probe.exe`, `sha256`, `size`,
  `protocol=1`, без выбранного caller другого executable/протокола;
- оба SHA256 canonical lowercase64 и различаются; helper <=32MiB;
  App+helper суммарно <=существующего1GiB ceiling;
- прежние version/released_at/notes/update-role key/signature. Подпись включает
  оба payload descriptors и target, TUF proof связывает полные raw manifest bytes.

Shared `netprobe.MaxNativeImageBytes` теперь также источник image limit core
wrapper и update payload. Packet/IPC/DNS/admission пределы не расширены.
Payload hashing/copy используют один regular Lstat/open/SameFile/size/Max+1
streaming primitive. Он не делает PE/build-pin/ABI/native ownership attestation.

Nested helper decoder требует все4 exact fields и отвергает unknown/duplicate/
aliased/null/missing/trailing/object-shape ошибки до typed decode. Общий manifest
limit64KiB сохраняется. PayloadSet проверяет оба authenticated hashes; partial
pair не выдаётся за полный успешный payload set.

## Windows offline issuer source

Новые команды существующего updatetool:

```text
sign-pair <app.exe> <helper.exe> <version> <private.key> <new-outdir> [notes]
sign-pair-copy <app.exe> <helper.exe> <version> <private.key> <new-outdir> [notes]
verify-pair <update.json> <payload-dir> <trusted_keys.json>
```

Это описание интерфейса, команды не запускались. Valid version/notes bounds —
до hashing. Issuer берёт bounded input snapshots, назначает fixed output names,
подписывает update key и требует свежий output directory. Copy streaming/new-only,
sync/close до link; итоговый manifest публикуется последним после проверки
подписанных файлов. При ошибке остаётся незавершённый каталог без manifest,
без force overwrite/recursive deletion/adoption ранее принятых release файлов.
`sign-pair` без copy повторно проверяет исходники перед manifest publication.

Legacy `verify` отказывается от schema2 и направляет в verify-pair. Последний
проверяет подпись по update-role keys переданного policy и оба файла; не проверяет
enrollment/version floor/установку/readiness/физическую сеть. Существующий delivery
publisher по-прежнему аутентифицирует manifest metadata; файлы выпускаются явно.

## Честная install capability текущего клиента

CheckUpdate/ApplyUpdate используют общий payload-set verifier, однако сначала
проверяют install support. Schema2 возвращает явный pair-installer-pending;
в сборке с compiled native pin schema1 тоже отвергается. Неподдерживаемый набор
не вызывает pointless binary hashing/staging/Stop/rename и не получает OK=true.
Legacy unpaired build сохраняет single-EXE path со всеми прежними authority gates.
Эта временная capability не является окончанием реализации updater или плана.

## Process-local installed-image containment

Перед первой попыткой изменить installed/backup names callback отмечает owned
`mutationAttempted`. Defer, расположенный после handoff.release defer, переводит
App в sticky installedImageChanged **до** открытия lifecycle gate. Он срабатывает
также при ошибке/панике/неподтверждённом restore/launch failure; отрицательный
Win32 result не принимается за доказательство неизменности всех имён.

Старый mapped App после этого не может:

- принять новый Start или выполнить lower-level startWithProbeResult;
- начать lifecycle/probe operation, сетевой retry или future-pin wake;
- возобновить worker после успешного нового profile import;
- снова применять mode/update из изменённого installed path.

Flag не сбрасывают Disconnect, импорт профиля, clock/epoch/intent change или
network recovery. Explicit Disconnect/quit сохраняются; поля Ready=false и
installed_image_changed/BLOCKED не представляют local custody как channel fail.
Успешно запрошенный новый GUI отмечен closing; sticky flag старого процесса не
превращает launch acceptance в ошибку или readiness. Это process-local защита,
**не persistent recovery/startup gate и не доказательство согласованного pair**.

## Следующая обязательная реализация

1. V2-153 filesystem stage/apply/rollback/startup block и V2-154 actual Windows
   App/private-SDK bridge/current-code activation source добавлены ниже.
   V2-155 original root/ancestor writer owner source добавлен ниже. Следующий
   V2-156 transaction directory custody/move source добавлен ниже. Следующий срез —
   V2-157 prospective image/code-acceptance worker source добавлен ниже. Следующий
   срез — GUI original process/Job ownership transfer и post-parent readiness.
2. Полный recovery с source/time/intent после sync, OS crash/durability и child
   ACK/readiness ещё открыты. V2-154 снимает pending только после fresh private
   authority/full installed set/compiled helper pin/version/held on-disk image;
   это не доказанная kernel loaded-section attestation и не доверие phase.
3. Role/TUF/revocation/history сохранение при offline resume, native child owner/
   ack/readiness и version-floor/rollback coordination. Старый EXE-only клиент
   не получает schema2 как якобы совместимый rollout.
4. После интеграции реального installer снять pending capability адресно,
   подключить full payload-set staging и обновить package/installer consumers.

SDK/compile/codec/crypto/CLI/Go tags/locking/panic/Win32/sharing/ACL/reparse/crash/
power-loss/interprocess/resources/EXE/NSIS/native/live/parity/comparative приёмка
отложена. Android/shared/configs/harness/generated/tests/общий LESSONS сохранены.
Все9/P0/custody/DR/quorum/client/bindings/rollout/release требования открыты.

## Узкое предложение для общего LESSONS

Подписать только главный EXE недостаточно для pinned sibling helper. Новый
wire schema должен связывать полный fixed artifact set, а intake capability —
отличаться от install support. После попытки изменить executable namespace
старый mapped App не должен создавать новый worker, даже если rollback вернул
nil; process-local containment не заменяет persistent recovery. Source V2-152,
runtime доказательств нет; общий LESSONS этим чатом не меняется.

## V2-153 — реальная файловая транзакция, source-only

### Владение и подготовка

`backend/update/install_stage.go` реализует StagePair: фиксированные App/helper,
owned raw manifest+optional delivery sidecar (64KiB/12MiB+512), streamed new/old
snapshots с исходным общим hash/copy primitive. Каждый set <=1GiB, helper <=32MiB.
Preinstall helper может отсутствовать для явной первой миграции, App обязателен.
Копии и immutable journal sync/close до prepared. Header<=4KiB, exact required
root/nested fields, без duplicate/unknown/aliased/null/trailing values или путей.

Возвращаемый private PairTransaction хранит directory/file creation identities,
owned old fingerprints и parsed new payload values. Ненулевой объект при ошибке
сохраняет незавершённые ресурсы. Discard разрешён только до pending publication:
удаляет известные созданные файлы при SameFile, не трогает unknown replacements/
extra entries/installed names, не делает recursive delete. Неопределённость
сохраняет объект и данные. Source constructor/methods имеют panic error boundary.
Directory snapshots получены через Open→File.Stat, не оставлены как lazy Lstat:
локальный Go `os/types_windows.go` откладывает path FileID lookup до SameFile,
а `os/stat_windows.go` File.Stat читает handle. Это предотвращает потерю исходного
ID при последующем stage rename; открытые directory handles здесь не удерживаются.

### Pending и apply

Apply требует nonnil **actual** caller authorization guard; сам callback не
является доказательством authority. Он повторяется после staging, metadata/phase
sync и долгого hashing, перед изменениями и после проверки installed pair.
Backend не реализует вторую роль/TUF/clock policy и не публикует Ready.

Перед первым installed rename атомарно резервируется новый фиксированный
`.snowden-update/`, существующая recovery-директория не заменяется. Подготовленный
stage переносится в `data`; при uncertain move отслеживается исходная identity.
Внутри только fixed `update.json`, optional `delivery.json`, `journal.json`,
`new/`, `old/`, `displaced/`, `rejected/`, `restore/` и fixed phase markers.
Root/control body/sidecar identities проверяются повторно; journal не выбирает
filesystem destinations. Фазы — подсказки порядка, не trusted acceptance.

Helper размещается первым, App вторым. Перед rename проверяются exact old/new
hashes и отсутствие конфликтующего destination; оригинал сохраняется в displaced,
в установленное имя переносится staged new. Затем проверяется весь payload set.
Ошибка не удаляет неизвестные installed bytes. Context проверяется между этапами
и writes; обычный OS file read/rename/sync этим не становится прерываемым SLO.

Apply nil означает **files placed, pending остаётся**. Нет floor promotion,
child launch, journal removal или права запускать VPN. Caller всё ещё должен
владеть lifecycle и заранее retire original Manager/helper Jobs/images.

### Rollback и recovery material

Rollback доступен только исходному private PairTransaction, не изготовляется из
public journal. Сначала сверяется полный owned backup set; известные новые bytes
переносятся в rejected, неизвестные не удаляются. Точные исходные displaced файлы
либо проверенные streamed backups возвращаются в фиксированные имена. Final old
set проверяется, marker rolled-back sync, **pending остаётся**. Файловое исправление
не объявляет старый code свежей authority и не обходит version floor/revocation.

LoadPairCandidate возвращает только bounded raw manifest/delivery и parsed facts
из вычисленного fixed data path. Проверяет structure/digests, не выдаёт Prepared
proof и не возвращает trusted rollback owner. Следующий App adapter обязан снова
пройти existing private prepareVerifiedUpdate/role/TUF/epoch/history/time gates.

### Persistent startup restriction

Любой pending entry (включая empty/corrupt/unknown) блокирует worker dispatcher
до serveEngineWorker. Main не вызывает update housekeeping; сама housekeeping
повторяет gate до RaiseFloor/удаления .old. Новый GUI получает image restriction,
update_recovery_required/Ready=false; proxy self-heal, network watch/background
probe producers/auto-connect/watchdog не запускаются. Close/Disconnect остаются.
GUI/vault/profile initialization не является принятием бинарного candidate.

Absence/phase/public SHA не подтверждают установленный код. Это ограничивающий
startup gate, а не автоматическое recovery. В текущем App schema2 install support
по-прежнему pending, StagePair/Apply/Rollback ещё не связаны с UI private guard.

### Остаток V2-153

Следующий actual App срез: сохранить owned verified sidecar, связать source/time/
intent guard и partial transaction owner, передать pending новому процессу,
freshly authenticate candidate через существующий SDK и проверить installed
App+helper/compiled pin/running version/self image перед finalization. Затем
снимать барьер/поднимать floor/очищать backup/возобновлять normal Start.
Same-process restore и later-process recovery не взаимозаменяемы.

Native owned installer/root leases/interprocess lock, path/ACL/reparse/sharing,
WriteThrough/directory metadata durability, power-loss ordering, original child
handle/ack/readiness и old-client migration остаются открытыми. File.Sync calls
не являются подтверждённой Windows crash/power-loss гарантией. Ничего из нового
backend, codec, SDK, файловых переходов или startup behaviour не исполнялось.

## V2-154 — actual App bridge, current-code acceptance и strict floor

### Изменённые пути

| Слой | Исходники |
|---|---|
| App/source/lifecycle | `windows/app.go`, `main.go`, `updateapp.go`, `update_image.go`, `update_delivery.go`, новые `update_pair.go`, `update_activation.go` |
| Native installed image | `backend/core/worker_child.go`, `native_image_reader_windows.go`, новые `installed_pair.go`, `installed_pair_windows.go`, `installed_pair_other.go` |
| Filesystem/manifest/floor | `backend/update/update.go`, `pair.go`, `install_apply.go`, новые `install_finalize.go`, `floor.go`, `floor_windows.go`, `floor_other.go` |
| Windows issuer | `tools/updatetool/pair.go` |

### Реальная связь с прежними владельцами

- `verifiedUpdate` владеет точными sidecar bytes. App/updateMu хранит partial
  `PairTransaction`; unpublished Discard только через original owner, uncertainty
  не перезаписывается новым staging. Работа отменяется прежним operation slot по
  Stop/epoch/Close, допустима при остановленном VPN без protectionDesired=true.
- Pair branch использует existing generation-fenced Stop/helper retirement и
  restartHandoff. `PairTransaction.Apply` теперь передаёт конкретный короткий
  filesystem commit actual private source/clock guard: reserve/rename и финальный
  applied marker выполняются внутри прежней journal serialization. Большие hash/
  copy идут снаружи. No-op authorize без вызова commit не считается успехом;
  ошибки mutation не теряются, даже если callback их проглотил.
- Pending/names touched/нечитаемый persistent barrier устанавливают sticky image
  restriction до handoff release. Original-owner rollback после ошибки не снимает
  pending и не разрешает старый код. Legacy/pair переиспользуют один launch helper.
- **Ветка pair apply пока недостижима через публичный ApplyUpdate**:
  `checkUpdateInstallSupport` отклоняет schema2 до staging/Stop. Source bridge
  подготовлен, но нет завершённого native root/installer/child owner. Это отдельная
  incomplete capability, а не «готово, просто не протестировано».

### Принятие полного нового комплекта при startup

1. Restricted startup после vault/import recovery, до proxy self-heal/network
   watch/background/auto-connect/watchdog, читает только fixed candidate facts.
2. Повторяет **actual** `prepareVerifiedUpdate(..., retain=true)` на exact raw
   manifest+delivery: действующие role/TUF/revocation/epoch/history/checkpoint/
   observed clock. Prefix сохраняется прежним владельцем, ошибки не откатывают его.
   Journal phase/old hashes не предоставляют code authority.
3. Требует schema2/activation_protocol1, ту же numeric running version и exact
   compiled helper pin. Protocol1 подписывается существующим Windows issuer;
   schema1 omitempty/canonical byte order сохранены на уровне исходника. Протокол
   заявляет совместимость, не доказывает ABI/сборку или child readiness.
4. `OpenInstalledPair` переиспользует прежний native image owner: retained original
   root/ancestor/parent/helper handles, canonical local NTFS/ACL/path/IDs, helper
   hash/PE/32MiB. Новый bounded reader хеширует original parent handle до1GiB и
   проверяет signed App/helper sizes. Partial/uncertain Close сохраняет pointer.
5. `FinalizePair` повторяет exact candidate/full installed payload set, создаёт
   fresh owned archive container. Actual private source/time check охватывает
   filesystem-only floor+archive commit. `RaiseFloor` выполняется **до** переноса
   pending в archive/transaction; ни backups, ни old bytes не удаляются.
6. Только после повторной native validation, confirmed original-handle Close,
   floor и прежнего intent/nonclosing снимается process restriction. Ready не
   выставляется: нормальный Start всё ещё требует fresh protected-path admission.
   Любая ошибка/паника/cancel/uncertain close оставляет текущий процесс blocked.

### Floor и пределы результата

`ReadFloor` отличает initial absence от malformed/oversized/nonregular/unreadable
state. Все production acceptance/startup/worker consumers используют error-aware
API; CurrentFloor остаётся deprecated display/compatibility wrapper без authority.
Housekeeping failure больше не игнорируется и не удаляет .old. Для release worker
main проверяет floor **до** internal serve; dev не является update target.

`RaiseFloor`: mutex + native fail-fast LockFileEx на retained regular single-link
nonreparse zero-size lock handle, re-read под lock, synced unique same-dir temp,
rename. Unlock/Close error сохраняет original handle для следующего точного retry;
новый handle не заменяет uncertain owner. Другие ОС не получают фиктивный lock.
Install требует version>running/floor; activation допускает floor==current, чтобы
повторить acceptance после floor success + archive failure. Floor ниже или storage
error не разрешают старт numeric release.

Это **per-user startup floor**, не защита от клиентов, которые этот код не
содержат, и не служба/машинный antirollback. Held on-disk pair не доказывает kernel
loaded-section identity и не изолирует hostile trusted owner/admin/kernel или
preexisting mapped view. Root/directory native interprocess ownership, Windows
write-through/durability/power-loss ordering, original child handle+ACK/readiness,
поздний repair смешанного комплекта/old-code restore/old-client migration и все
прежние OS/resource/SDK/DR/quorum/custody/rollout/release/comparative/all9 gates
остаются открытыми. Context30s не является hard native/filesystem deadline.

### Evidence и следующий срез

Разобраны текущие call sites, original image owner и pinned x/sys LockFileEx/
UnlockFileEx declarations. `gofmt -w` по20 изменённым Go-файлам exit0; после малых
source corrections повторно форматировались только затронутые файлы, exit0.
Это только formatting/parse, **не compile/runtime/crypto/filesystem/native PASS**.
Тесты/fixtures не добавлялись/не менялись/не запускались. Build/vet/check/doctor/
module/gen/SDK/product CLI/EXE/helper/Win32/network/native/live не исполнялись.
Owner secrets/configs/logs и чужие области не читались/не менялись.

Далее: original native root/directory installer owner + process handle/ACK/readiness
и crash/recovery protocol; затем пересмотр public install admission. Сохранить
actual private source commits, partial owners, persistent restriction и все9
обязательства. Узкий proposed LESSON: commit должен исполняться под actual
source lease; успешный callback без commit не является успешной транзакцией.

## V2-155 — original installation root writer owner, source-only

### Код и переиспользование

- Новые `windows/backend/core/installation_root{,_windows,_other}.go`: concrete
  `InstallationRootLease` с partial native owner при любом error/panic. App/updateMu
  сериализует use/Validate/Close; новый owner не заменяет прежний uncertain pointer.
- `native_image_windows.go`: прежние directory open, NTFS/ACL/path/FileID checks и
  file identity validation выделены в методы того же owner. Native image сохраняет
  свои parent/helper digest/PE/AppID/process gates и reverse original-handle close.
  Отдельной ACL/trusted-principal реализации для installer не появилось.
- `windows/app.go`, `update_pair.go`, `update_activation.go`, `updateapp.go`,
  `update_image.go`: actual App acquisition/validation/release на pair и legacy
  путях, honest public schema2 unsupported capability сохраняется. Generated,
  shared, manifests/dependencies и чужие платформы не менялись.

### Native lifetime

`OpenInstallationRoot` допускает только каталог фактического `os.Executable`,
canonical absolute DOS path<=2048, local fixed NTFS/persistent ACLs. До64 original
directory handles открываются root→ancestors→installation directory без share
write/delete; каждый проверяется прежним conservative owner+DACL/path/FileID
inspector. Это pin самих директорий, не произвольных child entries.

Fixed `.snowden-update.lock` открывается OPEN_ALWAYS, share0, OPEN_REPARSE_POINT,
read/write/READ_CONTROL. Требуются disk/regular/nonreparse/single-link/FileID и
прежняя trusted ACL, размер строго0. Exclusive fail-immediate LockFileEx закреплён
за original handle. Sharing/lock conflict возвращает typed busy; нет очереди,
PID/liveness эвристики, stale deletion, truncate, автоматического force/unlock
чужого owner. Имя остаётся после Close; kernel handle определяет текущее владение.

Directory+lock vector предварительно ограничен65 slots до первого native open.
Validate повторяет identity/path/ACL и неизменность regular lock file через
original handles; произвольный другой root не принимается. Close сначала снимает
свой range lock, затем использует прежний reverse-handle reaper. При Unlock/Close
error исходный owner/handles остаются в App, ready сбрасывается; разрешён только
точный cleanup retry. Нет replacement после неизвестного результата. На !windows
capability unavailable, без фиктивного mutex-only interprocess обещания.

### Actual App integration

1. Pair staging получает root scope до чтения/copy installed snapshots. Post-copy
   intent/context/root/source fences сохраняются. Unpublished discard закрывает
   root только после confirmed owned cleanup; published/unknown state не стирает
   owner. Неопределённый image owner также не позволяет бросить root scope.
2. Pair apply/rollback держат тот же original root; actual native validation идёт
   перед source callback **вне** metadata journal. Short filesystem commits остаются
   внутри actual private role/TUF/source/clock lease V2-154. Rollback не запускается
   при утрате root validation и не снимает persistent pending.
3. Новый current-code startup получает свой root scope до candidate/SDK intake и
   сохраняет его через native pair proof и floor/archive. Только confirmed image
   Close, root validation/Close, floor и intent позволяют снять process restriction.
   Ошибки сохраняют restriction и original owner; Ready не публикуется.
4. Legacy single-EXE Apply участвует в том же root lock, чтобы current-code legacy
   путь не обходил новую сериализацию. После final payload hash повторяются native
   root и actual source gates; большой hash вынесен из journal callback. Ошибка
   cleanup возвращается вызывающему. Перенесённое temp name больше не удаляется
   deferred cleanup после release writer scope.
5. Перед запросом launch и pair, и legacy отмечают old mapped App restricted,
   затем требуют confirmed root Close. Неопределённое закрытие не допускает launch.
   Эта передача не является original child owner или ACK/readiness: прежний bare
   launch helper остаётся незавершённым, schema2 admission не включён.

### Остаток и evidence

Это **cooperative root writer custody**, не полная native directory transaction.
Staged/pending/archive child directory handles/ACL/custody через owned moves,
rename/write-through/directory sync/power-loss ordering, original child process
handle/ACK/readiness, bounded native cancellation/resource/cleanup/AV/filesystem/
OS acceptance, privileged install/old-client migration/late repair остаются.
Исторические клиенты и hostile trusted owner/Admins/SYSTEM/TrustedInstaller/kernel/
preexisting writable mapping не становятся изолированными этим lock. Per-user floor
и held on-disk image не получили machine-wide/loaded-section гарантий.

Читались текущие native owner/call sites и pinned x/sys declarations/errors;
`gofmt -w` по9 unique Go-файлам exit0 (после точечных правок только затронутые
файлы). Это formatting/parser only. Source IDs/размеры и пределы evidence:
`.artifacts/memory/native-installer-source-evidence.json`. Нет compile/build/vet/
check/doctor/module/gen/SDK/codec/product CLI/EXE/helper/installer/Win32/network/
native/live execution; tests/fixtures не добавлены/не изменены/не запускались.
Сам `.snowden-update.lock` этим ходом не создавался: product native API не вызывался.

Следующий P0: staged/pending directory ownership through moves и original child
process/ACK/readiness с отдельным crash/recovery protocol. Не включать schema2
admission по факту наличия root lease. Все9 и прежние deferred gates сохранены.

## V2-156 — transaction directory custody и native move, source-only

### Код и actual consumers

| Слой | Изменённые пути |
|---|---|
| Core/native | `native_image_windows.go`, `installation_root.go`, `installation_root_windows.go`, новый `installation_directories_windows.go` |
| Backend update | новый `install_custody.go`, `install_stage.go`, `install_files.go`, `install_apply.go`, `install_record.go`, `install_finalize.go` |
| App | `app.go`, `update_pair.go`, `update_activation.go`, `update_image.go` |

`DirectoryCustodian` — filesystem capability, не SDK/update-role proof. StagePair,
LoadPairCandidate и FinalizePair теперь **требуют** actual owner; App передаёт тот
же concrete `InstallationRootLease`. Validate привязан к точному root до I/O.
Portable/no-op/default fallback не добавлен. Actual private source/clock/intent/
floor checks, prefix retention и protected-path admission остаются прежними.

### Bounded directory objects

- Native root owner хранит до8 transaction directory slots, allocated до native
  opens. Namespace ограничен fresh `.snowden-update-stage-*` + fixed5 children,
  `.snowden-update/data` + children, `.snowden-update-accepted-*/transaction/data`
  + children. Parent directory уже должен быть original root или pinned object.
- Pin/Validate повторно используют прежние path/ACL/FileID primitives. Добавлено
  отклонение zero FileID также для директорий. Partial native handle сохраняется
  в slot до проверок и не перезаписывается при ошибке. Повторный pin проверяет
  original object, не открывает ещё один handle и не расширяет ресурсный ceiling.
- Root/ancestor handles теперь запрашивают metadata/traverse/read-control/sync,
  share read/write без share delete: pin самого ancestor не запрещает legitimate
  child entry mutation. Regular App/helper deny-write/delete sharing сохранено.
  Transaction directories имеют DELETE и share read/write/delete для controlled
  move; это не изоляция hostile trusted owner, который игнорирует writer lock.

### Два directory moves

Stage→pending/data и pending→fresh archive/transaction используют original source
handle и original destination-parent handle через NtSetInformationFile /
FileRenameInformation. ReplaceIfExists=false; нет Ex/POSIX flags, overwrite,
path-based retry или подмены нового owner после ошибки. ABI сверена по установленным
Go stdlib/x/sys definitions и [Microsoft FILE_RENAME_INFORMATION](https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/ntifs/ns-ntifs-_file_rename_information).
Relative target/parent и отказ при существующей цели также описаны в
[Win32 FILE_RENAME_INFO](https://learn.microsoft.com/en-us/windows/win32/api/winbase/ns-winbase-file_rename_info).

Документация Microsoft ограничивает rename при открытых файлах в subtree. Поэтому
переход явный: под retained root lock validate всех originals → confirmed Close
только descendants → rename через всё ещё удерживаемый source handle → actual
source-handle path query → rebind descendants лишь при совпадении **private**
volume/FileID/path/ACL. IDs не читаются из journal и не заменяются при rebind.
Parent/source handles не бросаются ради попытки rename. Cleanup rebind имеет
soft2s budget/до7 known objects; это не hard native deadline.

Результат ОС сам по себе не выбирает путь: original source handle должен сообщить
один из двух ожидаемых путей, полный directory set должен снова пройти validation.
Native error после фактического move возвращает moved=true вместе с error;
backend сохраняет actual location и остаётся restrictive. Неизвестное положение,
Close/rebind/ACL/FileID error сохраняют transition/remaining handles; новая
транзакция не заменяет этот owner. Root lock освобождается только после confirmed
Close всех child handles и собственного lock/ancestor набора.

### App/filesystem transitions и cleanup

- Stage pins fresh stage +5 directories до записи artifacts. Apply pins exclusive
  pending, затем передаёт original stage object native move; immutable backend
  FileInfo IDs и native handles должны согласоваться после перехода.
- Candidate reader pins pending/data/fixed5 directories до чтения public journal/
  manifest/sidecar. Эти факты не принимаются как authority: current-code activation
  повторяет actual private SDK и installed image/pin/version gates.
- Finalization pins fresh archive container и переносит pending через custodian
  внутри actual private-source filesystem commit. SDK/floor authority не передана
  интерфейсу native custody. Native metadata rechecks внутри filesystem commit
  не вызывают App/Manager/process callback и не меняют прежний lock order.
- Unpublished Discard сначала навсегда снимает prepared, confirmed-close child
  pins, удерживая root lock/private creation IDs; затем прежний identity-limited
  cleanup удаляет только known files и empty owned directories. Partial cleanup
  нельзя позже превратить в Apply. Unknown entries/IDs остаются; recursive delete
  или public-journal rollback не добавлены.
- Archive cleanup начинается с release собственного directory pin и требует
  original identity + empty directory. Ошибки не проглатываются. FinalizePair
  возвращает retained/uncertain archive receipt даже при неудачном cleanup;
  `App.pairArchive` сохраняет его без повышения authority/Ready и без новой UI API.

### Evidence и следующий P0

`gofmt -w`14 unique Go-файлов exit0; после source corrections форматировались
только затронутые. Текущие signatures/call sites прочитаны, старых production
call sites не осталось. Source IDs:
`.artifacts/memory/directory-owner-source-evidence.json`. Это source/formatting,
**не** compile/ABI/filesystem/native/locking/resource/crash acceptance.

Tests/fixtures не добавлялись/не менялись/не запускались. Build/vet/check/doctor/
module/gen/SDK/crypto/codec/product CLI/EXE/helper/installer/Win32/network/native/
live не исполнялись; реальные owner values не читались. Чужие платформы/shared/
configs/harness/generated/common LESSONS сохранены.

Далее: prospective signed App/helper image lease перед loader, original child
process handle/ACK/readiness и explicit ownership transfer вместо bare launch /
Process.Release; native durability/crash/late-recovery/privilege/old-client/OS/
resources/SDK/DR/all9 acceptance остаётся. Public schema2 install **unsupported**:
наличие native directory source не завершает native installer или его приёмку.

## V2-157 — prospective image и current-code acceptance worker

### Реализованный срез

- Новый `backend/core/update_code_acceptance.go`: dedicated owner вне reserved
  network helper lanes. App передаёт actual SDK/source/clock/intent guard и raw
  target SHA. До release writer root открывается прежний `OpenInstalledPair` на
  **новые подписанные** App/helper bytes; partial image остаётся в owner.
- `native_image_windows.go`, `installed_pair_windows.go`/`_other.go`: общий original
  suspended-process path gate теперь применим и к App path, и к helper path.
  Hash/ACL/path/ID/PE/compiled-pin gates не заменены строкой process path. Source
  предполагает прежние cooperative sharing/trusted-owner пределы, не kernel
  loaded-section attestation или защиту от preexisting writable mapping.
- `helper_process.go`, `native_capture.go`, `worker_process.go`,
  `worker_host_windows.go`: переиспользован прежний spawn/capture/reaper. Updater
  может подтверждённо retire **process** и сохранить original image; обычные
  network helper lanes сохраняют прежний close-image и DNS128MiB contract.
- `update_acceptance.go`, `update_activation.go`: один extracted actual candidate
  gate для startup и internal worker, один cleanup original image/root owners.
  `main.go` dispatch до Wails/Singleton/housekeeping. `app.go`, `update_pair.go`,
  `update_image.go`: retained owner/source fence и honest GUI-transfer pending.
  `backend/update/pair.go`: versioned capability2. Всего15 Go-файлов.

### Внутренний режим и authority

Fixed `--internal-update-code-acceptance-v2 <raw-manifest-sha256>` допускает только
канонический32-byte hash, exact args и inherited pipe stdin/stdout. Любая похожая
malformed flag тоже обрабатывается как internal error, не превращается в GUI.
Нет произвольных command/path/credentials в протоколе.

Worker создаётся suspended в original one-process Job, c explicit handle allowlist
и before-Resume actual App-image/source gate. Environment содержит только
APPDATA/SystemRoot/TEMP/TMP/WINDIR, cwd — system directory; public SDK store и temp
пути explicit same-user absolute DOS locations. Application mode не наследует
proxy/PATH/HOME/keylog/startup environment и разрешён spawner только для exact
internal command. Вне этого режима прежние engine/helper environments не изменены.
SDK memory policy сохранена без применения произвольного DNS128MiB лимита.

Current-code callback использует `NewApp` как value constructor и прежние
DirectoryCustodian/prepareVerifiedUpdate/actual purpose/source checks. Он требует
exact candidate raw SHA+sidecar, текущую private role/TUF/epoch/revocation/history/
clock authority, numeric **running** version и compiled helper pin, held full App/
helper identity. `retain=false`: accepted prefix, floor, pending/archive не меняет;
не выполняет startup/vault recovery/seed/logs/proxy/WFP/TUN/GUI/network producers.

**SDK verification не является полностью read-only filesystem:** существующий
go-tuf adapter использует temporary public-metadata workspace. TEMP/TMP переданы
явно, чтобы clean environment не отправлял этот workspace в недоступный Windows
directory. Prefix persistence, workspace ACL/reparse/cleanup-on-kill и resource
гарантии различаются; последние остаются отдельными native/SDK gates.

Ответ — ровно33 bytes: protocol2 + expected raw target SHA. Он формируется только
после actual acceptance и confirmed own image/root cleanup. Parent принимает bytes
только после zero exit, original process wait, IO completion, empty Job и confirmed
original process handle cleanup, затем свежего **parent** SDK/source/clock fence.
Prospective image остаётся в dedicated owner; uncertainty запрещает replacement.
Это не новый signer/proof primitive: hash/reply не заменяют исходную private SDK
authority и не публикуют VPN Ready, GUI readiness, floor или install success.

### Capability и оставшаяся передача

`PairActivationProtocol=2` означает наличие code-acceptance mode; Windows issuer
использует эту существующую константу. Protocol1 schema2 теперь возвращает explicit
unsupported error; подписанные bytes не переписываются/не переинтерпретируются.
DNS helper protocol остаётся1. Schema1 omission/canonical order source сохранены.
Ни CLI, ни keys, ни manifests/EXE владельца не запускались/не менялись.

Pair apply после native placement pins prospective pair до writer release, затем
выполняет fixed worker phase. После неё возвращает explicit GUI ownership-transfer
pending; прежний bare launch/Process.Release для pair удалён. Неопределённый
process/image owner хранится в App до exact cleanup/exit. Code acceptance worker
завершается: он не объявлен persistent installer или владельцем будущего GUI.
Legacy unpaired launch остаётся прежним отдельным незавершённым источником.

Wails Singleton требует выхода old parent до GUI startup. Следующий P0 — native
installer coordinator/GUI original process+Job lifetime transfer и post-parent
acceptance/readiness без deadlock и без убийства successor при закрытии old Job.
Учесть existing pre-Create auxiliary handle cleanup uncertainty в reused spawner,
не терять prospective image, закрепить crash/durability/late-recovery/privilege/
old-client/resource/OS/SDK/DR/all9 acceptance. Public schema2 install **unsupported**.

### Evidence

Прочитаны фактические call sites, process/Job/image ownership и upstream SDK temp
workspace path. `gofmt -w`15 unique Go-файлов exit0; после corrections — только
затронутые. Source SHA/size:
`.artifacts/memory/successor-source-evidence.json`. Это formatting/source evidence,
не compile/worker/SDK/crypto/ABI/native/runtime acceptance. Tests/fixtures не
добавлялись/не менялись/не запускались. Build/vet/check/doctor/module/gen/product
CLI/EXE/helper/worker/installer/Win32/network/native/live не исполнялись. Owner
values и чужие платформы/shared/configs/harness/generated/common LESSONS сохранены.

## V2-158 — ресурсный owner до CreateProcess

### Найденный пробел и решение

Прежний factory возвращал process owner после CreateProcess, а ранние Job/pipes/
duplicates/attribute-list закрывал best-effort defer. Ошибка Close или failed
attribute initialization могла потерять receipt ресурса. Это вывод по source,
не воспроизведённый runtime-инцидент. Переиспользован тот же factory для engine,
native helpers и code-acceptance; новый launcher или parallel policy не введены.

Пять changed Go paths:

- `backend/core/worker_spawn_windows.go`: factory вынесен из Windows host. Host,
  parent I/O и preparation owner создаются до первого Job; после acquisitions
  ошибки возвращают partial process. CreateProcess пишет в retained
  ProcessInformation; preparation закрывается до Resume suspended child.
- `backend/core/worker_host_windows.go`: actual created state, adoption partial
  native outputs, один waiter original process; reserved negative pseudo-handle
  values отвергаются перед Wait/Terminate/Close. Это value guard, не object-type
  attestation. Engine ceiling8 и прежняя environment policy сохранены.
- `backend/core/worker_process_io.go`: wrapper сохраняет `os.Pipe` и его I/O,
  не удерживает свой mutex при blocking Read/Write. Close latch ставится до Go
  Close; success idempotent, ошибка/прерывание sticky. Повторный ErrClosed не
  выдаётся за native closure; такой owner может потребовать выхода приложения.
- `backend/core/worker_spawn_resources_windows.go`: fixed3 child Go files,
  fixed3 inherited duplicates, caller-owned native attribute allocation с pinned
  vector. Allocation сохраняется даже при failed Initialize; void Delete и
  checked LocalFree разделены. После failed free повторяется только original
  allocation free; interrupted Delete не повторяется. Unknown deletion удерживает
  references/vector, native close failures сохраняют исходный handle.
- `backend/core/worker_process.go`: optional native preparation interface без
  изменения base processHost. Never-created cleanup не вызывает Wait/Terminate
  и не закрывает искусственно done. Actual root exit/I/O/empty Job отдельно от
  preparation/pipe closure; retry после drained всё ещё требует их успеха.

### Основания и ограничения

Проверены pinned x/sys v0.44.0 declarations: LocalAlloc возвращает uintptr,
LocalFree — Handle; convenience constructor не возвращает allocation при failed
second Initialize, Delete не возвращает free result. Pinned Go File.Close consumes
close state; это основание sticky uncertainty, не повод подменить Go pipe raw
синхронным CreatePipe. Скрытые внутри stdlib acquisitions/cleanup не получают
receipt через её API и остаются ограничением этого среза.

Microsoft определяет [двухшаговый Initialize](https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-initializeprocthreadattributelist),
[Delete без результата](https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-deleteprocthreadattributelist)
и [нулевой результат LocalFree при успехе](https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-localfree).
API docs прочитаны; native API не выполнялись. Возврат из void Delete — отсутствие
наблюдаемого прерывания, не независимое доказательство native correctness.

`gofmt -w`5 unique changed Go files exit0, source/call-site review и SHA/size:
`.artifacts/memory/spawn-owner-source-evidence.json`. Compile/ABI/panic/cancel/
resource/race/native/OS поведение не проверено. Tests/fixtures не добавлялись,
не менялись, не запускались; build/vet/check/doctor/module/gen/SDK/crypto/codec/
product CLI/EXE/helper/worker/installer/native/network/live не исполнялись.

Следующий P0 — persistent installer coordinator и original GUI Process/Job/image
ownership transfer с post-parent acceptance/readiness, Singleton ordering и
crash/durability recovery. Публичная schema2 installation остаётся unsupported.
Native hard bounds, SDK temp workspace, trusted owner/admin/loaded sections,
old-client/service/OS/DR/resource/release/comparative и все9 этапов остаются открыты.

Узкое предложение для общего LESSONS: process-drained и all-resource-closed —
разные receipts; acquire receipt до первого ресурса и хранить uncertain original
owner. В общий LESSONS запись не вносилась по границам пяти направлений.

## V2-159 — original-handle GUI bootstrap и mounted renderer receipt

### Передача вместо bare launch

Новый подписанный App сначала работает как coordinator, затем **тот же процесс**
входит в Wails. Отдельный постоянно живущий installer-процесс не добавлен.
`UpdateCodeAcceptance.StartGUI` использует существующий suspended factory и
prospective image после успешной fixed code-acceptance фазы. Один owner mutex
сериализует Verify/StartGUI/Close; Verify не может отправить transferred GUI в
прежний helper reaper. Parent side source/clock/intent gate повторяется.

До Resume оригинальный parent Job duplicate и original parent process duplicate
помещаются в suspended target process через DuplicateHandle. Это target-table
values: parent не закрывает их как свои handles. Права — JOB_OBJECT_QUERY и
SYNCHRONIZE|PROCESS_QUERY_LIMITED_INFORMATION, non-inheritable; PID используется
только для запрета self, без reopen/lookup/taskkill. Child проверяет non-inherit,
membership в конкретном Job и exact limits через native queries. Значения из
pipe не создают trust authority; неподтверждённые произвольные handles не закрываются.

Fixed bounded protocol: seed52 bytes (`SE03`+raw target SHA+2 handles), затем36-byte
`ARM3`/`GO03`/`OWN3` frames. Состояния не называют GUI Ready. Child перед ARM получает
свой SDK-verified installed image owner; partial root/image остаются достижимыми
до terminal os.Exit. После GO/OWN closes standard slots/pipes, ждёт **original
parent handle**, с 15s soft budget/100ms native waits, затем повторяет actual
current-code SDK/candidate gate и image validation. Только потом Wails/Singleton.
Reserved malformed mode, missing pipes, failed source/handle/SDK/timeouts не
переходят в обычный GUI. Child dispatch budget60s, parent existing operation
budget30s; это engineering caps, не измеренное время и не native hard deadline.

Parent сохраняет original process/Job/image owner до своего OS exit: не вызывает
Process.Release, не закрывает process handle под ещё активным Wait и не запускает
retire после committed transfer. Ошибка до committed transfer использует прежний
original Job reaper; uncertainty сохраняет owner. Сообщение пользователю — лишь
«передача подготовлена»; готовность после выхода parent ему не приписывается.

### Job и окружение

GUI Job имеет KILL_ON_JOB_CLOSE и **explicit** BREAKAWAY_OK, без helper active
process/memory ceiling: WebView2 создаёт process tree. Следующий fixed successor
использует CREATE_BREAKAWAY_FROM_JOB и получает новый Job, поэтому не умирает с
предыдущим GUI Job. Внешний enclosing Job с запрещённым breakaway даёт error,
без retry внутри старого Job/без direct fallback. Engine8 и helper1/128MiB
nested Jobs не получают breakaway или иной новой политики. GUI lifetime query
handle не наследуется descendants и удерживается самим GUI до OS exit.

Clean environment сохраняет явные APPDATA/TEMP/TMP/SystemRoot/WINDIR; GUI получает
ещё проверенный LOCALAPPDATA для cache, без наследования PATH/proxy/HOME/keylog.
Режим tun/proxy и resume/stopped — fixed enums; normal Start всё ещё проходит
защиту. Не объявлены поддержка всех WebView2/launcher environments или performance
win; SDK temporary workspace, GUI memory и native resource gates открыты.

Основания: [Job lifecycle/breakaway](https://learn.microsoft.com/en-us/windows/win32/procthread/job-objects),
[duplicate rights/target handle table](https://learn.microsoft.com/en-us/windows/win32/api/handleapi/nf-handleapi-duplicatehandle),
[IsProcessInJob](https://learn.microsoft.com/en-us/windows/win32/api/jobapi/nf-jobapi-isprocessinjob),
[JOB_OBJECT_QUERY](https://learn.microsoft.com/en-us/windows/win32/procthread/job-object-security-and-access-rights),
[pending Wait handle нельзя закрывать](https://learn.microsoft.com/en-us/windows/win32/api/synchapi/nf-synchapi-waitforsingleobject).
Это прочитанные API contracts; native вызовы не выполнялись.

### Actual GUI acceptance, включая ручной recovery

Pinned Wails2.13.0 запускает OnStartup и OnDomReady в разных goroutines;
navigationCompleted вызывает OnDomReady без success check. Поэтому один callback
не даёт readiness. Main создаёт startup-done barrier для restricted image;
startup восстанавливает storage, но не активирует pending и не запускает network
producers. OnDomReady ждёт barrier и fresh mounted-root receipt.

Backend посылает 32-byte fresh nonce как64 lowercase hex, ждёт exact response
10s с bounded250ms retransmit, очищает timer/listener. Корневой Vue onMounted
регистрирует ACK через существующий `api/backend.ts`, onUnmounted отписывает;
mock/browser без Wails не ACK. Нет новых generated bindings/dependencies.
Это liveness корневого renderer, **не proof всех страниц/данных/визуальной
корректности**, не signer/SDK/floor/GUI security/protected VPN authority.

Повторная navigation инвалидирует renderer epoch; stale receipt не снимает
restriction. Actual installed activation снова проверяет exact bootstrap target
SHA (если передан), private SDK/source/clock/version/pin/whole image; extra bootstrap
image закрывается под fresh App image lease до floor/archive. После native/root
cleanup и session receipt final value gate сверяет intent/renderer epoch/closing.
Только затем снимается restriction и запускаются прежние network services.
Startup resume использует тот же Start registration с expected intent: поздний
Disconnect/Close/другой intent не переопределяется автоматическим подключением.

Pair activation protocol теперь3; old1/2 отклоняются, signed bytes не мигрируются.
Internal acceptance name тоже v3; DNS protocol1/schema1 omission unchanged source.
Public schema2 installation **по-прежнему unsupported**: наличие source path не
проход native/OS/durability/release acceptance.

### Paths, evidence и следующий срез

16 Go sources: core `worker_process`, `worker_spawn_windows`,
`update_code_acceptance`, `update_gui`/`_windows`/`_other`; update `pair`;
App `app`, `main`, `elevation_windows`, `update_acceptance`, `update_activation`,
`update_pair`, `update_image`, `update_gui`, `update_gui_renderer`.
Frontend: `src/App.vue`, `src/api/backend.ts`. Changed source hashes/sizes:
`.artifacts/memory/gui-owner-source-evidence.json`.

`gofmt -w`16 Go exit0, source/call-site/pinned-declaration review only. Skills
modern-web-guidance/frontend-postgres прочитаны; локальный guide search не дал
подходящего guide, использованы фактические Vue/Wails lifecycle/API patterns.
Compile/typecheck/build/tests/fixtures/vet/check/doctor/module/gen/SDK/product CLI/
EXE/helper/worker/WebView2/native/admin/network/live не выполнялись. Тесты/fixtures
не добавлялись/не менялись. Common/adaptive/trust/Android/configs/harness и чужие
снимки/чаты не изменялись; dependency/publication/release действий не было.

Следующий P0: `backend/update/install_apply.go`, `install_files.go`,
`install_record.go`, `install_finalize.go` + native custody. Payload replacements
пока используют os.Rename; root/directory leases не заменяют original regular-file
custody, crash/disk-full/flush ordering и durable late recovery. Оставить private
transaction authority и pending restriction, устранить ложный успех на partial
file/close/flush/rename outcome до public admission. OS/Job/renderer callback/race/
privilege/hard-bound/temp-workspace/old-client/DR/resource/release/comparative и
все9 этапов остаются unverified/open. Общий LESSONS не изменялся.

## V2-160 — original regular-file custody и receipts записи/чтения

### Actual apply/rollback

`install_custody.go`: обязательная platform capability `PayloadCustodian` поверх
DirectoryCustodian; StagePair сохраняет прежнюю signature, но до записи требует
конкретную capability. `PayloadFile` хранит Path/Payload facts и original-object
Validate/CopyTo/Move/Close. Эти facts — filesystem receipt, не SDK authority.

InstallationRootLease реализует capability через тот же Windows root writer.
До8 regular-file owners: old installed2/new2/backup2/restore2; переименование не
создаёт новый owner. Нативный owner сохраняется **до** CreateFile; documented
InvalidHandle failure не принимается за acquired handle. Fixed root/pending/data
namespace, только два известных имени EXE/helper, single-link/non-reparse/NTFS/
ACL/path/FileID validation через прежние native primitives. At most2 разрешённых
private snapshot описывают допустимые size/SHA; размер должен совпасть до чтения,
hash через original synchronous handle, payload чтения <=32KiB. Unknown bytes
остаются owned/unready, не перемещаются/не удаляются и запрещают продолжение.

Original file открывается read+DELETE, share-read без write/delete sharing. После
pin применяются оригинальные FileID/size/LastWriteTime, не новый path-based hash
в journal. `CopyTo` повторяет bounded hash из original handle перед возвратом;
App serializer не допускает concurrent Copy/Move/Close. Сам helper не исполняет
network/trust/clock/lifecycle решения.

`install_apply.go` больше не вызывает os.Rename: native FileRenameInformation
получает source handle + retained destination-parent handle, replace=false;
без overwrite/Ex/POSIX/path fallback. Разрешены root→displaced/rejected и
new/displaced/restore→root; backup/rejected не превращаются в новый installed set.
Actual path original handle проверяется даже при error; known location обновляет
receipt, unknown оставляет root moving/restrictive. Cleanup soft budget2s не
является гарантией прерывания Win32 I/O. Rollback выбирает old/new из одного
owned snapshot, не теряет owner при неверной догадке о hash; unknown target
сохраняется. Restore из backup копируется по original handle в private output.

Pins приобретаются **после** stage→pending directory publication; directory move
с payload handles запрещён. Actual App после Apply подтверждённо закрывает payload
owners под retained root lock, затем pins prospective signed image до release
root. Это controlled scope handoff с новой проверкой bytes/ID, не kernel loaded-
section attestation. Failed Close сохраняет root/remaining originals и запрещает
loader/GUI continuation. Root.Close закрывает payloads раньше directory handles.

### Staging writes и metadata readers

`install_payload.go`: до24 retained Go writer receipts, private local path,
O_EXCL/new regular zero-size file, context write, checked Sync и Close. Никакого
ignored Close defer; Close uncertainty sticky, повторный ErrClosed не proof.
Apply/rollback/discard сначала retire exact writers; incomplete close не даёт
unlink/замену owner. Marker/restore reuse требует **этого** writer.complete receipt:
Sync+Close+noncancelled completion. Просто совпавшие bytes после Sync error не
становятся новым успехом. Published failure сохраняет pending и старые backups.

Pinned Go syscall.Open не включает FILE_SHARE_DELETE; ordinary os.Open конфликтует
с custodian DELETE access. Lstat FileInfo также может лениво открывать path за
FileID при SameFile. `install_metadata*.go` даёт explicit compatible Windows
share-delete read, reparse-point opens и eager handle Stat snapshots; pairRead
сравнивает два таких IDs/size/mtime. Share modes разделены для dir attributes и
regular content; snapshot не заменяет native custodian или SDK.

Один process-level metadata reader owner, fail-fast TryLock, без очереди/cache
authority. Original raw handle сохранён до NewFile conversion, после conversion
Go owns тот же handle — двойной raw Close исключён. Confirmed close освобождает
slot; Go close error/panic держит pointer и блокирует следующие update reads до
OS exit. Это bounded quarantine, не доказанное восстановление ошибочного fd.
Не добавлены общий scheduler, второй installer или новые зависимости.

Основания: [CreateFile access/share](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-createfilew),
[original source/relative parent rename](https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/ntifs/ns-ntifs-_file_rename_information),
[FlushFileBuffers требует write access](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-flushfilebuffers),
actual pinned Go os/types_windows.go + syscall/syscall_windows.go и x/sys declarations.
Read-only image handle не выдаётся за flushed file/volume. **Native rename и
успешный Sync файла не доказывают power-loss ordering directory metadata.**

### Evidence, ограничения, следующий P0

13 changed Go paths: core `installation_root.go`, `_windows.go`,
`installation_directories_windows.go`, `installation_payload_windows.go`;
update `install_custody.go`, `install_stage.go`, `install_files.go`,
`install_apply.go`, `install_payload.go`, `install_metadata.go`, `_windows.go`,
`_other.go`; App `update_pair.go`. `gofmt -w`13 exit0, later touched subsets exit0.
Hashes/sizes: `.artifacts/memory/payload-owner-source-evidence.json`. Source/call-
site/API review only; no tests/fixtures added/changed/run, compile/build/typecheck/
vet/check/doctor/module/gen/SDK/product CLI/EXE/helper/worker/native/admin/network/
live. Owner secrets и Android/shared/configs/harness/чужие STATE/чаты не тронуты.

Public schema2 remains unsupported. Native sharing/rename/loaded EXE/ACL/partial
status/close/cancel/resource/race/OS/crash/DR gates не проверены. Model не изолирует
trusted same-owner/admin/kernel или preexisting writable mapped views.

Следующий срез: durable intent/control publication и late recovery ordering в
install_record/install_finalize/install_stage/install_files; custody создания
barrier/directories, сохранение private transaction authority, никакого исполнения
старого кода по journal/marker. Также пройти remaining path verifier readers
`update.go:payloadDigestTo`: текущий V2-160 owner покрывает pair metadata reads,
не всю библиотеку/stdlib/SDK. Не выдавать directory rename/absence pending за
power-loss proof. Full9/SDK/temp/old-client/privilege/OS/DR/resource/release/
comparative обязательства сохраняются; shared LESSONS не менялся.

## V2-161 — durable user intent, archive recovery и accepted-image restrictions

### Запись до замены файлов

`install_intent*.go`: один фиксированный HKCU slot
`Software\SnowdenSystem\UpdateIntentV1`, REG_BINARY `State`, максимум2048 bytes.
Strict exact13-field JSON: duplicate/unknown/null/type/oversize/canonical version/
hash/combined payload bounds отклоняются. Нет строки полного installation path:
root сопоставляется по SHA256 lower-case canonical path; stage/archive — только
фиксированный basename family с bounded suffix. Это correlation/restriction,
не secret, подпись, доверенный root или право отката.

Под тем же fail-fast `floorMu` + actual interprocess floor file lock сохраняются
nonce intent, exact manifest/delivery hashes, app/helper size/hash и версия.
Apply делает это после полной подготовки **до** pending Mkdir и installed names.
В actual source/clock guard выполняется только bounded compare-and-Set; ключ
и writer lock остаются у первоначального owner. RegFlushKey/readback/checked
RegCloseKey выполняются после выхода из metadata journal. Затем Apply повторяет
actual source/intent/time/root gate перед filesystem mutation. Flush не считается
прерываемым по Go context и не доказывает NTFS directory metadata ordering.

Нативный reader использует фиксированный buffer, не unbounded GetBinaryValue.
Сохраняется original output HKEY до следующего native шага; predefined HKEY не
закрываются. Один process slot/checked close, close uncertainty sticky и запрещает
новое открытие. На Windows нет fallback на обычный JSON-файл. Non-Windows writer
явно unsupported; pure publisher может читать initial state.

Registry slot свободен во время SDK/floor recheck; иначе ReadFloor внутри actual
guard рекурсивно блокировал бы writer. Startup/worker restriction reads отдельно
сериализуются с writer через interprocess floor lock: видимый Set при ещё идущем
Flush не допускает worker. ReadFloor только повышает ограничение и может читать
согласованное bounded value внутри writer guard, без повторного захвата floorMu.

Любая попытка публикации делает private transaction logical Pending, даже при
ошибке до pending Mkdir. Discard больше не удаляет такое staging. Actual App
удерживает installed-image restriction; неизвестный Set/flush/close outcome не
выдаётся за отсутствие изменений. Active intent ограничивает **все roots этого
пользователя**, поэтому перенос папки не обходит незавершённую транзакцию.

### Архив и приёмка

Перед pending→archive/transaction сохраняется bounded archive basename, затем
вновь проверяется source/clock/image/intent. Контейнер сохраняется при uncertain
locator publication. Reader выбирает ровно pending/data либо указанный
archive/transaction/data, pins original container/transaction/data/5children
через существующий максимум8. Одновременное существование двух кандидатов —
ошибка, не выбор по journal phase. Manifest/delivery/payload/version должны
совпасть с intent. Архив — только locator: current-code actual SDK/private local
proof/compiled helper pin/held image остаются обязательны.

Если процесс умер после archive move до приёмки, FinalizePair повторно использует
этот архив с actual authorizer, без второго move и без права запуска старого кода.
CompletePairIntent после этой проверки сохраняет **одним value** cleared intent,
принятую версию/floor и последние payload facts/archive locator. Original root и
image leases остаются до завершения Flush, затем actual App подтверждает их
закрытие и только после этого снимает свой флаг. Post-Set error может означать уже
видимую/сохранённую приёмку: rollback по error не делается, текущий App остаётся
restrictive до корректного нового старта.

ReadFloor берёт максимум strict legacy file floor и registry floor; corrupt
legacy file не игнорируется. Accepted value не удаляется: потеря старого file
rename после power loss не превращается в нулевой floor. Обычный release startup
перед housekeeping и engine worker сверяет same-root accepted version и compiled
helper pin, повторно pins/hashes EXE/helper через original native image owner.
Partial/failed close сохраняет global original owner до OS exit. Fresh expired
TUF envelope не требуется на каждом обычном запуске; accepted facts только
добавляют ограничения и не заменяют private SDK при update/recovery.

PairActivationProtocol теперь4; fixed internal acceptance/GUI аргументы v4 и
ARM4/OWN4 frames. DNS helper protocol1/schema1 остаются прежними. Исторические
protocol1…3 manifests/аргументы не объявлены совместимыми; fixtures не обновлялись
из-за запрета тестирования. Issuer берёт существующую общую protocol constant.

### Remaining verifier reader

`update.go:payloadDigestTo` использует V2-160 retained metadata reader вместо
обычного os.Open/ignored Close. Eager original File.Stat до/после чтения, exact
size/mtime/FileID, bounded32KiB streaming и checked Close; ошибка обнуляет digest.
Никакого lazy Lstat SameFile reopen для этой проверки. Это не полный аудит
stdlib/SDK/temp-resource ownership и не loaded-section attestation.

Основания: [RegSetValueExW](https://learn.microsoft.com/en-us/windows/win32/api/winreg/nf-winreg-regsetvalueexw),
[RegQueryValueExW](https://learn.microsoft.com/en-us/windows/win32/api/winreg/nf-winreg-regqueryvalueexw),
[RegFlushKey](https://learn.microsoft.com/en-us/windows/win32/api/winreg/nf-winreg-regflushkey)
и pinned x/sys declarations. Это API/source review, **не выполненные native calls**.
HKCU — user scope с inherited DACL, не служба/TPM/machine-wide rollback seal.
Same-user trusted owner/admin/kernel, old clients и другие Windows users не
покрыты. Accepted receipt добавляет image restriction только для прежнего root;
перенос принятой установки/несколько installations требуют отдельной политики.

### Evidence и оставшийся P0

16 Go paths отформатированы `gofmt -w` exit0; touched subsets также exit0.
Hashes/sizes — `.artifacts/memory/intent-owner-source-evidence.json`.
Не выполнялись tests/fixtures/build/compile/vet/typecheck/doctor/harness checks/
module/gen/SDK/crypto/product CLI/EXE/helper/native/admin/network/live; реальный
реестр/vault/config/owner values не читались/не менялись. Android/shared/configs/
harness/чужие snapshots/чаты не тронуты.

**Public schema2 install остаётся unsupported.** Следующий P0: forward recovery
после durable intent, но до полной установки пары (включая stage-only и partial
helper/app placement), и accepted receipt + повторно появившийся pending после
power loss. Сейчас эти случаи restrictive, автоматической починки нет. Нужны
original-owner/control creation и crash/disk-full/rename/flush/native sharing/
loader/ACL/partial close/renderer/OS/DR/resource проверки после снятия запрета.
Generic floor temp writer и SDK temporary readers остаются отдельным аудитом.
Никакого old-code запуска/rollback authority по registry/journal; all9,
old-client/service/privilege/release/comparative gates сохранены открытыми.

Урок для этого Windows-среза: видимость RegSetValue не означает завершённый
durable handoff. Flush следует вынести из metadata journal, а startup reads
сериализовать с оставшимся writer; исходный ключ и logical restriction удерживать
до известного результата. Общий LESSONS не менялся.

## V2-162 — forward-only partial pair recovery через прежний App owner

### Backend и границы authority

`install_recovery.go`: отдельный `PairRecovery` facade использует существующий
PairTransaction custodian, private snapshots, checked writers, source commit и
native payload handles. Нет публичных Rollback/Discard; внутренний forwardOnly
также запрещает rollback. Старые hashes из прочитанного journal нужны только
для точного control comparison, **не** дают право восстановить/запустить старый
код. Constructor сохраняет частичный owner на ошибке, Apply требует actual App
authorizer; Retire только подтверждённо закрывает writers, ничего не удаляет.

Candidate loader теперь различает stage-only после durable intent, stage +
незавершённую pending reservation, pending/data и exact archive. Никакого walk/
PID/stale-file удаления или произвольного пути из journal. Два незавершённых
data sets не выбираются эвристикой. Stage публикуется прежним original-handle
directory move в pending/data **до** приобретения regular payload handles.
Max8 directory и max8 payload owners сохраняются.

Helper затем App: уже совпадающий signed NEW payload не открывается с DELETE
(это также не конфликтует с удерживаемым bootstrap image). Полную image identity
и running-code acceptance после placement всё равно проверяет core/App.
Для отсутствующего/не совпадающего target поиск ограничен fixed5 new/rejected/
restore/displaced/old. Size admission до hash; нужны точные NEW size/SHA из
freshly authorized manifest. Нет разрешения старого hash/версии по месту файла.
Из old/rejected подписанные NEW bytes копируются в свободный new/restore с
checked Sync/Close; сами old/rejected не перемещаются обратно как code.
Context проверяется между32KiB hash/copy chunks и inventory entries; это soft
cancellation, не hard bound stalled native/filesystem I/O.

Если требуется заменить regular current target, сначала берётся **новый private
snapshot реально прочитанных bytes**, затем original native payload owner
повторно подтверждает его. Signed forward purpose разрешает сохранить этот
объект в свободный displaced/rejected и поставить проверенный NEW payload.
Даже повреждённые current bytes сохраняются, не исполняются и не становятся
old-code rollback authority. Занятые quarantine/output slots, unknown nonregular/
oversized/changed objects, failed close или неизвестный rename outcome остаются
restrictive. Никакого overwrite чужих/старых файлов или cleanup по одному hash.
Normal original-owner rollback V2-160 этой новой authority не получает.

### Accepted receipt и повторная filesystem barrier

При accepted receipt + reappeared pending/stage loader требует прежние exact
manifest/delivery/version/payload facts, а App — fresh actual local SDK/role/
history/clock proof. Старый архив сохраняется. Перед исправлением одна bounded
registry mutation+Flush заново устанавливает nonce intent, сохраняя floor.
Для pending/stage сбрасывается только locator нового recovery archive; прежний
archive не удаляется/не перезаписывается. Archive-only продолжение сохраняет
свой locator и требует уже полного installed NEW set. После исправления работают
прежние current-code FinalizePair/CompletePairIntent/image cleanup gates.

### Actual App flow и передача процесса

`prepareUpdatePairCandidate` выделяет прежнее точное private SDK preparation;
fixed code-acceptance worker/bootstrap сохраняют строгий current-version/
compiled helper pin/held-image gate. Новый private forward purpose разрешает
target>=running и target>=strict floor, а equal running также требует compiled
helper pin. Старый restricted GUI может только поставить будущую подписанную
пару; принять себя должен её **новый** код. Dev не становится release target.
Версии/purposes выбирает фиксированный enum, не произвольная trust callback.

DOM-ready activation после storage и mounted nonce использует PairRecovery и
повторяет source/time/root/intent/renderer guard после длительных reads/Flush и
перед каждой mutation. Partial writer держится в App, confirmed retirement и
ReleasePayloads предшествуют prospective native image. Failed cleanup не теряет
root. Если names менялись или running version отличается, старый mapped GUI
остаётся restricted и передаёт работу существующему UpdateCodeAcceptance:
original image→new-code Verify→GUI Job handoff→parent exit→fresh SDK/renderer.
Не запускаются network producers старого App и не публикуется VPN Ready.

Для этого применяется **тот же** restartHandoff token: actual operationMu
передаётся его release без повторного Lock, renderer epoch включён в source
gate, Close deferral/markLaunched/terminal closing сохранены. Release owner
устанавливается до pushState, включая panic paths. Полная пара без изменения
names в уже текущем коде идёт в прежний native acceptance без лишнего рестарта.
Startup image preparation теперь делает одну checked cleanup попытку при
известной ошибке; unconfirmed Close удерживает original global owner до exit,
повторный Close/reopen не выдаётся за подтверждение. Такой owner блокирует repair.

Protocol4 GUI frames дополнены SE04/GO04 на обеих сторонах, вместе с ARM4/OWN4;
v4 argument/33-byte acceptance contract прежние. Historical fixtures не менялись.

### Evidence, следующий P0 и остановка promotion

15 Go sources: update `install_record`, `install_stage`, `install_apply`,
`install_intent`, новый `install_recovery`; App `update_acceptance`,
`update_delivery`, `app`, `update_pair`, новый `update_recovery`,
`restart_handoff`, `update_activation`, `update_gui`, `update_accepted_image`;
core `update_gui`. `gofmt -w`15 exit0, later recovery subset exit0 — форматирование.
Hashes/sizes: `.artifacts/memory/forward-recovery-source-evidence.json`.
Source/call-site reasoning only; no tests/fixtures added/changed/run, compile/
build/vet/typecheck/doctor/check/module/gen/SDK/product CLI/EXE/helper/native/
admin/network/live/real registry/owner values. Android/shared/configs/harness/
чужие snapshots/чаты не менялись. Нет runtime/crash/durability/DR PASS.

**Public schema2 по-прежнему unsupported.** Forward placement теперь есть в
source, но обычный запуск recovery GUI возможен только при работоспособном
entrypoint EXE. Если процесс уже умер и App EXE отсутствует/не загружается после
two-step rename, он не сможет сам запустить этот код. Следующий P0: устранить
окно отсутствующего App name либо предоставить независимо аутентифицируемый
owned recovery entrypoint; journal/старый backup не дают право запуска.
Archive-only повреждение без доступных signed NEW bytes также не чинится
выдумыванием payload или старым откатом. Missing/corrupt control, ambiguous
directories, exhausted fixed slots и expired fresh recovery authority блокируют
продолжение; новая доставка нужна отдельно. Control/directory creation custody,
legacy floor temp writer/SDK readers, native sharing/loaded EXE/ACL/rename/close/
panic/cancel/renderer/Close/crash/power-loss/OS/DR/resources/old clients/cross-user/
service/release/comparative/all9 gates остаются открытыми.

Windows-решение: восстановление может лишь вновь подтвердить и поставить NEW
signed target; переживший crash public journal не наследует private original-
owner rollback. Поддерживать этот контракт и при будущем внешнем entrypoint.

## V2-163 — fixed NEW-code cold entrypoint и retained recovery bytes

### Почему не выдаём ReplaceFile за crash-safe swap

Microsoft документирует частичные rename outcomes у ReplaceFileW и отсутствие
поддержки REPLACEFILE_WRITE_THROUGH. Native обычный replace также имеет ограничения
для исполняемого target. Поэтому нет замены прежнего owner на path-based
ReplaceFile/POSIX fallback или заявления, что один API call доказывает power-loss
atomicity. Основания: [ReplaceFileW return outcomes](https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-replacefilew),
[FILE_RENAME_INFORMATION rules](https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/ntifs/ns-ntifs-_file_rename_information).
[Hard links](https://learn.microsoft.com/en-us/windows/win32/fileio/hard-links-and-junctions)
разделяют содержимое, поэтому recovery — отдельная byte copy, не hard link к Main.
Это API/source analysis, никаких native calls выполнено не было.

### Fixed companion, actual authority и ранняя маршрутизация

Protocol5 определяет `snowden-recovery.exe` — точные bytes signed NEW application
из того же manifest, не отдельно собранный старый helper/backup. Schema2 logical
pair остаётся Main/helper; alias и его права фиксированы activation protocol,
произвольный executable filename в manifest не разрешён. Требуется **RoleUpdate**;
profile/recovery metadata roles не разрешают EXE, keys/root/epoch не переназначаются.
Это проверка полномочий до repair, **не OS-enforced pre-load signature seal**.
Trusted owner/admin/kernel/preexisting writable mappings остаются вне покрытия.

`DispatchUpdateRecovery` вызывается в main раньше любых GUI/worker/updater modes.
Companion basename + zero args/один fixed cold flag либо exact read-only check
flag+manifest SHA; неизвестные/старые modes, normal Main с recovery flags и
лишние аргументы terminal, без GUI fallback. Нет caller root/source/command path.
Check требует actual pipes и выдаёт fixed33 bytes protocol5+exact target SHA
только после callback и подтверждённого native cleanup. Read-only callback также
проверяет retained NEW helper bytes и повторяет SDK/time после чтения. Cold work soft60s;
это не bound stalled Win32 I/O. Ни Wails startup, ни config/network services,
enrollment, vault seed, proxy/DNS/WFP/TUN этим entrypoint не запускаются.

Actual `runRecoveryEntrypoint`: прежний root writer/candidate + local private SDK
preparation retain=false, точный target build/running version/compiled helper pin/
strict floor и held native self-image. Запуск по одному public journal/receipt
не разрешён. Read-only mode не меняет installed payload/trust/floor; root lock
bookkeeping и SDK temporary workspace остаются прежними, это не zero-I/O claim.
Cold repair вызывает тот же forward-only PairRecovery со свежими source/time
gates, original payload retirement, prospective Main image→new Main code Verify
→GUI Job transfer→terminal parent exit. Intent/floor не принимаются companion:
прежний новый Main GUI обязан пройти storage/SDK/native/renderer acceptance.
Cold repair передаёт stopped/proxy args, не восстанавливает VPN intent из argv.

### Native custody и правильный executable target

`RecoveryImageLease` реиспользует прежние ACL/NTFS/nonreparse/single-link/FileID/
close primitives, но не требует ещё исправного Main/helper. Original owner до
первого native acquisition; bounded exact-size SHA + fixed amd64 PE header,
<=1GiB/32KiB chunks. GUI/CUI subsystem разрешены только application image;
native DNS helper wrapper сохраняет CUI-only. Runtime/native compatibility этим
не доказана. Process validation uses original assigned Job/process handle и
fixed companion path; нет запроса нового process по PID.

Отдельная ready flag и original native resources сохраняются на ошибках;
checked Close, никакой замены partial owner. `OpenUpdateRecoveryCheck` использует
**тот же** UpdateCodeAcceptance process/Job lane с fixed check argument и actual
App source guard. Wrapper хранится в lane до native acquisition. Этот owner
не может StartGUI; обычная acceptance/GUI argument и frames теперь v5/
SE05/ARM5/GO05/OWN5, 33-byte verification contract прежний. Historical protocols
1…4 не объявлены совместимыми; fixtures не менялись.

OpenInstalledPair теперь pins **fixed Main sibling**, даже если caller — recovery
companion. Проверить собственную recovery copy вместо реально установленного
Main и затем сообщить его готовность нельзя. Native helper image open сохраняет
прежний caller path; изменение касается installed application pair scope.

### Retained bytes и packaging source

StagePair сохраняет ещё одну exact NEW pair copy рядом с journal/control:
`snowden-recovery.exe` и non-dispatched `native-helper.recovery`. Потоки проходят
прежний CopyVerified/checked Sync/Close; дополнительные staged bytes ограничены
тем же combined1GiB. Старые/private backups не заменяются. Recovery сначала
ищет прежние fixed5 sources, затем только эти fixed aliases (для App также
fixed root companion), каждый раз требует exact NEW size/SHA и пишет в свободный
new/restore output. Unknown/occupied files не удаляются/не перезаписываются.
Таким образом accepted archive может дать подписанные NEW bytes для missing
Main/helper, а не разрешение на старый rollback. Native regular-file namespace
расширен только на уже поддержанный exact archive/transaction/data/5child family;
max8 directory/max8 payload owners сохранены, directory moves под payload pins
по-прежнему запрещены.

`package.ps1` source копирует именно built Main в fixed companion и сравнивает
оба SHA до NSIS, без второго build. NSIS source кладёт/removes exact sibling;
нового shortcut/autorun/service нет. UTF-8 BOM installer source сохранён.
Package hash equality — byte identity, не release signature/runtime PASS.
**Скрипты, сборка и installer не запускались; никакой новый EXE не создан.**

### Evidence и незавершённая связь с Apply

18 Go source paths `gofmt -w` exit0; ещё2 packaging source paths. SHA/size receipts:
`.artifacts/memory/entrypoint-source-evidence.json`. Source/call-site/API analysis
only. No tests/fixtures added/changed/run, compile/build/vet/typecheck/doctor/check/
module/gen/product SDK/crypto/CLI/EXE/helper/native/admin/network/live/real registry/
owner values. Web использован только для Microsoft API docs, не VPN/live probes.
Android/shared/configs/harness/чужие snapshots/чаты не тронуты.

**Public schema2 остаётся unsupported.** Cold entrypoint/bytes/packaging source
добавлены, но обязательная публикация NEW companion **до первой installed-name
mutation** и actual process/SDK readiness **ещё не подключены** к Apply/forward
repair. OpenUpdateRecoveryCheck пока seam, не выполненный consumer pipeline.
Следующий P0: original-handle companion publication без overwrite unknown;
retire DELETE pins→hold read-only companion image под прежним root writer;
подтверждённо отпустить root для child SDK verifier, затем re-acquire/pin прежние
private root/directory IDs и исходный transaction owner, не mint rollback из
journal. Повторить source/intent/time/image fence после child и до helper/Main
mutation, удерживать companion image через prospective Main image acquisition.
Только затем вместе пересматривать public support; сам protocol5 не proof.
Native publication/loader/sharing/close/interprocess/disk-full/crash/power-loss/
control creation/GUI/OS/DR/resources, floor temp writer/SDK readers/old-client/
cross-user/service/release/comparative и all9 gates открыты. Damaged/missing
companion/control/retained bytes либо expired fresh authority не чинятся догадкой.

Windows-решение: отдельно хранить NEW byte-identical entrypoint и повторять actual
SDK/self-image authority, а не делать отсутствующий Main запускаемым через
old backup hash или недоказанную atomicity ReplaceFile. Общий LESSONS не менялся.

## V2-164 — mandatory publication/readiness перед изменением пары

### Backend sequencing

`PairReadiness` — synchronous platform callback с ровно одной scoped binder
операцией, не новая trust authority. Original Apply и forward-only Apply теперь
требуют явный typed третий аргумент; nil возвращает ошибку до intent/filesystem
mutation. Нельзя забыть callback в новом product call site и получить компилируемый
two-argument вызов. Существующие tests/fixtures не менялись и не запускались;
их API migration остаётся отложенной до разрешения тестирования.

После durable intent и публикации data, **до первого Main/helper mutation**, тот
же PairTransaction публикует fixed recovery name. Уже совпадающий NEW companion
не открывается с DELETE; actual native image/SDK проверка всё равно обязательна.
Для замены старого auxiliary target — fresh private bounded capture, original
native pin и свободный displaced/rejected slot. NEW bytes берутся только из
fixed7 известных мест по exact signed App size/SHA, поток пишется в свободный
new/restore recovery output с checked Sync/Close, затем moves через original
source+parent handles. Нет overwrite неизвестных файлов, old-code запуска или
восстановления старого companion по public journal. Main/helper-флаг изменения
не поднимается одним auxiliary move; persistent Pending всё равно restrictive.

Regular-file namespace расширен лишь на fixed RecoveryApplicationFile в прежних
root/pending/archive child families. Max8 payload и max8 directory owners прежние.
Временный auxiliary output добавляет не более App.Size дополнительных записанных
bytes; max24 checked writer receipts сохраняется. Это per-transaction bound,
не глобальная quota накопленных архивов и не замер скорости/disk cost.

Перед read-only image/child проверкой закрываются ВСЕ точные DELETE payload
owners и writers. Failed close или incomplete Sync receipt запрещает дальнейший
handoff. Matching bytes не превращают ошибку writer этого owner в успех.
Moved собственный auxiliary output удаляется только из прежнего location map;
physical original остаётся в native custody, затем его root FileID сохраняется.

### Private snapshots и root handoff

Под первоначальным root writer фиксируются original root ID, bounded7/8 data/
parent/child directory IDs, собственные created-file IDs с завершёнными size/
mtime и exact companion ID. Это private live capture, не PID/journal recreation.
Создатель writer сохранял Stat **до записи**: size/mtime нельзя брать из того
creation Stat; для handoff они снимались заново при сохранённом creation ID.

Backend держит исходный transaction mutex и private old fingerprints через
callback. Binder guarded mutex + active/called state допускает ровно один bind;
после callback/panic новые вызовы не выполняют I/O, допущенный bind не остаётся
после release transaction mutex. Callback contract остаётся synchronous.

Actual App открывает `OpenUpdateRecoveryCheck` под первоначальным root и current
source/clock/intent guard. Затем подтверждённо закрывает именно original root,
не рекурсивный `closePairRoot/PairRecovery.Retire` под active transaction mutex.
Native companion image удерживает root/path/bytes; child получает writer lock,
делает собственную actual current-code SDK/self-image/retained-helper проверку,
завершает original owned Job/cleanup и возвращает exact receipt.

После Verify App reacquires actual root, binder pins прежние directory paths и
сравнивает **все сохранённые IDs** и own file metadata, immutable control и
companion. Только затем тот же transaction получает новый custodian; private
old hashes не читаются заново из journal. Original rollback дополнительно
проверяет прежние private backup bytes, forwardOnly этот grant не получает.
Source/time/intent/renderer и readiness повторяются после child/hashes и до
helper/Main move. Partial new root/check owner на ошибке остаётся в App.

### Actual consumers и image lifetime

Подключены все3 product Apply call sites: normal signed pair bridge, restricted
GUI forward recovery и cold companion repair. `OpenUpdateRecoveryCheck` теперь
фактический consumer pipeline, не только seam. Его private verified flag ставится
только после existing owned process/empty Job/exact reply flow; новый
ValidateRecoveryReady требует этот flag, fixed check purpose и held image.
Он вызывается **после** Verify, не рекурсивно из его source guard.

App держит отдельный original readiness Job/image owner через pair placement.
До каждого последующего update commit есть его validation вне metadata journal.
Confirmed retirement — лишь после prospective Main/helper image acquisition под
reacquired root; uncertainty сохраняет owners и запрещает root release/GUI
continuation. Cleanup использует тот же original process/image, не PID/reopen.
Cold companion вместо recursive child использует своё уже фактически проверенное
self-image + actual current SDK и binder с тем же root. Missing callback,
unknown slots, failed IDs/source/clock/cleanup не открывают Ready/VPN/direct path.

### Evidence и следующий P0

14 Go paths: update `install_custody/stage/apply/recovery`, новый
`install_readiness`; core `installation_payload_windows/update_recovery_dispatch`;
App `app`, новый `update_readiness`, `update_pair/recovery/acceptance/
recovery_entrypoint/image`. `gofmt -w`14 exit0; source/call-site reasoning only.
SHA/size: `.artifacts/memory/publication-source-evidence.json`. No tests/fixtures
added/changed/run, compile/build/vet/typecheck/doctor/check/module/gen/product
SDK/crypto/CLI/EXE/helper/native/admin/network/live/real registry/owner values.
Android/shared/configs/harness/чужие snapshots/чаты не менялись. Никакого runtime,
crash/power-loss, native sharing/loader/close/Job/renderer/OS/DR/resource PASS.

**Public schema2 остаётся unsupported.** Publication/readiness/root rebind теперь
source-wired, но не compile/native/OS accepted. Protocol5 сам по себе ничего не
разрешает. Следующий P0 — original receipts для создания control directories и
staging/temp files: текущие os.Mkdir/MkdirTemp/OpenFile дают path/API результаты
раньше pin/custody; нужно native creation ownership до validation/conversion и
checked partial cleanup. Не ослаблять immutable control/private transaction или
делать directory absence доказательством acceptance. Legacy floor temp writer,
SDK temporary readers, hard I/O/process containment, disk-full/cancel/Close/
interprocess/crash/reboot/ACL/sharing/loaded image/GUI/DR/resources/old-client/
cross-user/service/release/comparative и все9 этапов остаются открытыми.

Windows-урок: root handoff не переносит private rollback authority в journal.
Переносить исходный transaction и live IDs, а после writer Close создавать новый
native custodian только при проверке прежних объектов. Общий LESSONS не менялся.

## V2-165 — original native creation receipts

2026-10-26. Windows source only, unverified.

### Создание внутри существующего root owner

Прежние `os.Mkdir/MkdirTemp/OpenFile` возвращали результат раньше native pin.
Теперь actual `InstallationRootLease` реализует `CreationCustodian`: создание
каталога, ограниченный выбор уникального имени, staging writer и его retirement.
StagePair, Apply, forward recovery и Finalize требуют эту capability; path-only
подстановка не допускается. Это владение файловыми объектами, не SDK authority.

Native root резервирует original slot **до** вызова `NtCreateFile`; указатель
выходного handle ведёт прямо в этот slot. Имя — один разрешённый компонент
относительно уже удерживаемого parent handle. `FILE_CREATE` не открывает и не
перезаписывает существующий объект; no reparse traversal/path retry/overwrite
fallback. Directory/regular type, NTFS identity/path и inherited ACL проходят
прежние проверки по тому же handle. Частичный результат остаётся в original
owner, даже если последующая проверка/отмена завершилась ошибкой. Свойства API:
[Microsoft NtCreateFile](https://learn.microsoft.com/en-us/windows/win32/api/winternl/nf-winternl-ntcreatefile).

Stage/archive suffix — 12 random bytes в hex, не больше16 попыток. Повтор
допускается только после exact collision с отсутствием native creation receipt;
только тогда удаляется пустой slot. Остальные ошибки не маскируются выбором
следующего имени. Max8 directory owners сохранён: stage+5 children+pending+
archive. Новый объект с `created=false, err=nil` не принимается как созданный.

Persistent `.snowden-update.lock` использует отдельный `FILE_OPEN_IF`: pre-bound
original output, actual root parent, sharing0, проверка opened/created result,
zero-byte regular object, прежний `LockFileEx`. Нет truncate/delete stale lock
или доверия PID. NT sharing/lock conflicts распознаются как busy. Ни native ABI,
ни interprocess/sharing behavior этой реализации не запускались.

### Ограниченная запись и точная identity

Staging writer использует исходный raw handle и `WriteFile/FlushFileBuffers/
CloseHandle`, без передачи владения в `os.File`. Shared `StagingFileLimit`
ограничивает точные control/marker/retained-pair/new/old/restore имена и размер
каждого файла; displaced/rejected и installed names через writer недоступны.
Auxiliary recovery output разрешён только в new/restore. Native creator повторяет
ту же проверку. Max24 writer receipts сохраняется; один native write ≤32KiB,
context проверяется между блоками. Это bound объёма/числа вызовов, не hard deadline
зависшего kernel I/O и не общая quota старых архивов.

Оригинальный writer сохраняется в root до первого acquisition. Partial/short
write, превышение cap, ошибка/неопределённость flush и Close остаются sticky;
повторный вызов не превращает их в completion. Перед native write/flush/close
заранее установлен restrictive receipt на случай panic. Backend считает запись
завершённой только после Sync, подтверждённого Close и проверки cancellation.
Matching bytes не отменяют failed completion этого transaction owner.

После создания из original native handle сохраняется portable `CreatedFileID`
(volume/index). При известном ID частичный writer может дать cleanup receipt;
неизвестный ID не даёт права удалить path. Marker/discard/readiness используют
существующий checked metadata reader и сравнение **его native handle** с этим
ID; затем снимаются обычные OS FileInfo. Custom FileInfo/SameFile fallback нет.
Completed size/mtime для handoff по-прежнему фиксируются после записи отдельно.
Non-Windows не получает ослабленную имитацию native creation identity.

Staging handle допускает read sharing; directory rename/delete не совместимы
с ним. Directory moves явно отклоняются при live/uncertain writer. Root Close
сначала подтверждает retirement всех original writers, затем payloads/directories
и root lock. Sharing semantics до Close описаны в
[Microsoft CreateFile](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-createfilew);
это не runtime подтверждение данной последовательности.

### Частичные результаты и сохранённые ограничения

Apply и forward recovery записывают `published` сразу при observed pending
creation, **до** возврата native validation error. Исходный Apply также фиксирует
attempted. Pending intent остаётся restrictive; нельзя повторить операцию как
будто reservation не было. Stage сохраняет partial stage path; Finalize возвращает
partial container receipt. Unknown identity/path/extra entries не удаляются.
Actual App root retains original handles независимо от backend return value.

V2-164 mandatory companion readiness, private snapshots/SAME transaction rebind,
private original old fingerprints, forward-only recovery, fresh SDK/clock/intent
и held image lifetime сохраняются. Creation capability не даёт code authority,
не принимает старые backup hashes и не делает schema2 публично installable.

### Evidence и следующий P0

16 Go source paths `gofmt -w` exit0; final whitelist subset2 также exit0. Точные
SHA/size — `.artifacts/memory/creation-source-evidence.json`. Source/API/call-site
reasoning only; это не compile/runtime acceptance. Tests/fixtures не добавлялись,
не менялись, не запускались. Build/vet/typecheck/doctor/check/module/gen/product
SDK/crypto/CLI/EXE/helper/native/admin/network/live и owner values не выполнялись/
не читались. Внешние обращения — только official API docs; новый native pipeline
не запускался.
Android/shared/configs/harness/чужие snapshots/чаты не менялись.

**Public schema2 остаётся unsupported.** Нет directory-flush/atomic durability
или power-loss claim от одного successful file flush. Следующий P0 — legacy
floor/registry AppData directory/lock/temp writer и SDK/metadata reader ownership:
там ещё нужно адресно устранить acquisition/conversion/partial cleanup gaps.
Дальше native ABI/ACL/sharing/loader/close/Job/GUI/disk-full/cancel/crash/reboot/
power-loss/OS/DR/resources/hard bounds/old-client/cross-user/service/release/
comparative и все9 этапов. Исторические tests/API fixtures требуют отдельной
миграции только после разрешения тестирования.

Windows-урок: native output должен попадать в уже удерживаемый original owner;
успешное последующее открытие path не восстанавливает потерянный creation receipt.
Общий LESSONS не менялся.

## V2-166 — original floor owner для legacy/intent/startup

2026-10-26. Windows source only, unverified.

### Реальные потребители и владение

`raiseFloor`, registry `changePairState` и `lockPairStartup` больше не создают
AppData-каталоги через bare MkdirAll. Все три используют прежний `lockFloor`
seam с одним actual native floor owner под `floorMu`. Native output направлен
в уже удерживаемые ancestor/app-dir/lock/temp records, а не в transient handle
до регистрации владельца. Даже partial acquisition остаётся достижимым.

Rooted traversal начинается с actual drive root; существующие ancestors OPEN,
только fixed snowden-system/update directories и persistent zero-byte lock
OPEN_IF. Relative children используют удерживаемый parent handle. NTFS/type/
no-reparse/path/FileID/ACL проверяются по original objects; regular lock/temp
требуют один link. Ancestors/lock запрещают delete sharing, сохраняя read/write
sharing для прежнего byte-range lock protocol. Max64 directory records+lock1+
temporary1; native blocking I/O не получает hard deadline от этого bound.
Semantics относительного имени, open/create и inherited ACL:
[Microsoft NtCreateFile](https://learn.microsoft.com/en-us/windows/win32/api/winternl/nf-winternl-ntcreatefile).

**Scope/compatibility:** этот native owner поддерживает local drive/NTFS,
bounded2048-char clean paths, не UNC/device/reparse/non-NTFS profile. Неподдержанный
путь возвращает unavailable; обход через path-only writer не добавлен.
Это source contract, не проверенная совместимость roaming/enterprise profiles.
ReadFloor сохраняет прежний canonical-file reader; его NewFile conversion,
parent/file ACL и retained-read lifetime требуют отдельной приёмки ниже.

### Переиспользование ACL

Бывший `core/native_image_acl_windows.go:Validate` алгоритм вынесен в существующий
Windows-only `filetxn/acl_windows.go`. Conservative owner/DACL allow-ACE walk,
max128 ACE, complete SID bounds, NULL/unknown active ACE rejection и прежний
mutation mask сохранены. Pure SID predicate предоставляется concrete caller;
никаких App/SDK callbacks или account/network lookup под journal. Core оставляет
свои current-user/Admin/SYSTEM/TrustedInstaller predicate, private struct/factory
и ErrNativeImageUnavailable wrapper. Floor применяет тот же helper со своим
текущим process token. Это platform primitive, не второй shared trust SDK.
Algorithm migration не прошла compile/native/ACL regression acceptance.

### Ограниченная запись, публикация и cleanup

Temporary name — fixed .update-floor- prefix + random96-bit hex, max16 попыток,
FILE_CREATE. Retry только exact no-create collision; partial output не заменяется
новым record. Payload canonical version ≤64 bytes, один native write с exact
count, FlushFileBuffers, original identity/size recheck. Нет os.File conversion.

Перед публикацией повторяется прежний ReadFloor=max(valid legacy file, valid
HKCU state) под actual writer lock; corruption остаётся ошибкой, missing legacy
file остаётся initial floor. Rename выполняется по original temp handle и actual
destination-parent, обычный FileRenameInformation. Replace разрешён **только**
для fixed version control file; Ex/POSIX/path fallback не используется.
Свойства rename ABI/DELETE requirement:
[Microsoft FILE_RENAME_INFORMATION](https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/ntifs/ns-ntifs-_file_rename_information).

Даже после native error запрашивается actual path original handle. Confirmed
private temp остаётся доступным для точного cleanup; canonical location никогда
не удаляется как temp. Unknown location сохраняет все original handles/lock и
ошибку. WRITE handle подтверждённо закрывается **до** checked ReadFloor readback,
чтобы старый metadata reader с deny-write sharing мог открыть canonical file.
Return version не меньше requested, cleanup error не теряется в defer.

Неопубликованный known temp удаляется через FileDispositionInformation на
original handle после повторной private ID/path/ACL проверки, не os.Remove по
имени. Lock/ancestors освобождаются только после writer retirement. Native
close/unlock/delete uncertainty sticky, original owner не заменяется/reopened.
Release callback привязан к конкретному owner и сохраняет один result: stale
callback не может закрыть следующий global owner. Startup wrapper дополнительно
использует sync.Once для единственного release/unlock `floorMu`.

### Evidence, остаток и границы

9 Go paths `gofmt -w` exit0; final original-release subset3 exit0. Source SHA/size:
`.artifacts/memory/floor-source-evidence.json`. Source/call-site/pinned x/sys/API
reasoning only. Tests/fixtures не добавлялись, не менялись, не запускались;
build/vet/typecheck/doctor/check/module/gen/product SDK/crypto/CLI/EXE/helper/
native/admin/network/live/реальные registry/config/vault/owner values не тронуты.
Web использован только для official API docs, не для product network probes.
Android/shared/configs/harness/чужие snapshots/чаты не менялись.

**Public schema2 остаётся unsupported.** Next P0: SDK temporary workspace,
readCache original acquisition/checked Close, update pair metadata NewFile
conversion/reader retirement, safe partial workspace cleanup. Текущий SDK всё
ещё пишет temporary metadata и игнорирует часть cleanup outcomes — не считать
этот source-срез решением SDK resource custody. Fresh SDK/source/clock/intent,
private original rollback fingerprints/forward-only recovery/readiness/held
image sequencing сохранены. No directory-flush/atomic power-loss claim; native
ABI/ACL/sharing/lock/close/loader/GUI/disk-full/cancel/crash/reboot/OS/DR/resources/
hard bounds/old-client/cross-user/service/release/comparative/full9 gates открыты.
Исторические private test helpers/API fixtures требуют миграции после разрешения
тестирования; текущий запрет не обойдён запуском парсера/компилятора/validator.

Windows-урок: release должен держать original owner, а не читать следующий global
resource. Повторная ошибка закрытия и повторное callback-освобождение не должны
переходить на чужой handle/новую регистрацию. Общий LESSONS не менялся.

## V2-167 — original metadata readers / 2026-10-26

### Владение и идентичность

Новый Windows-only reader размещён в существующем backend/filetxn: delivery
не может импортировать update, поскольку update уже использует delivery.
Constructor только выделяет receipt; concrete pair/cache owner сохраняет его
до Open. NtCreateFile пишет handle прямо в этот record, включая partial error.
Нет os.NewFile, двойного raw/Go owner или finalizer. Open одноразовый; checked
Close сохраняет один result, uncertain original handle остаётся достижимым.

Read использует original handle, ≤32KiB за вызов и caller-supplied total cap
с overflow byte. Проверяются disk/type/no-reparse, regular single link, size,
actual final path, nonzero FileID и прежний conservative ACL helper/current
user/Admin/SYSTEM/TrustedInstaller predicate. Directory reads запрещены.
Update adapter сохраняет ErrPairRecoveryRequired поверх native cause, чтобы
существующие consumers продолжали распознавать свой recovery contract.
Ordinary drive/UNC names bounded; device/ADS/malformed components отвергаются.
Это не поддержка всех aliases/roaming/SMB/filesystems: final-path equality и ACL
могут отвергнуть иной путь, совместимость пока не проверена. Ancestor custody
установщика остаётся у прежнего root owner; reader не заменяет её.

Stat создаёт immutable FileInfo с eager volume/index и value Sys; comparison
принимает только этот private type. os.SameFile для custom FileInfo не вызывается,
path-reopen fallback отсутствует. PairTransaction root/dirs/private old snapshots,
completion/readiness и finalization используют samePairFile; native creator
CreatedFileID сопоставляется с тем же original handle snapshot. На !windows
сохранены os.File/stdlib comparison, typed-nil не записывается в interface.

### Cache bytes, отсутствие и закрытие

Delivery readCache использует один retained reader под fail-fast mutex.
До возврата exact upstream-persisted bytes требуются declared bound, exact length,
before/after ID/size/mtime и confirmed Close. Panic/error не возвращают bytes.
Unknown Close сохраняет reader и ошибку, запрещая replacement до выхода процесса.
Если Open сообщил missing, а Close не подтверждён, наружу не возвращается
os.ErrNotExist: retainedState не должен принять это за основание удалить cache.

Native missing определяется exact NT status и отсутствием handle при FILE_OPEN.
Information на failure не обязан быть нулём; учтён документированный вариант
FILE_DOES_NOT_EXIST. FILE_OPEN не создаёт отсутствующий объект:
[Microsoft NtCreateFile](https://learn.microsoft.com/en-us/windows/win32/api/winternl/nf-winternl-ntcreatefile).

### SDK остаётся отдельным открытым пунктом

В pinned go-tuf v2.4.2 DisableLocalCache отключает persistMetadata/создание
каталогов, но loadLocalMetadata продолжает os.ReadFile. Пустой directory
не устраняет I/O: возникает относительное чтение. Поэтому флаг не переключён;
private SDK state, UnsafeLocalMode и новая собственная TUF state machine не
использованы. Exact raw bytes и verified-prefix/final artifact batch gates
сохранены. Источник: [официальный updater v2.4.2](https://raw.githubusercontent.com/theupdateframework/go-tuf/v2.4.2/metadata/updater/updater.go).

Наш reader не контролирует SDK's internal read/write/temp/rename/cleanup,
MkdirTemp/initial cache writes и removeWorkspace; это следующий source-срез.
[WIN-SDK-001](CHANGE-REQUEST-WINDOWS-SDK-001.md) содержит локальный запрос bounded
storage seam с checked cleanup и exact verified-prefix bytes. Другим чатам
ничего не отправлялось; shared SDK/dependencies не менялись.

### Evidence и остаток

13 unique Go sources: gofmt -w exit0; missing-status subset1 и последующее
уточнение update error wrapper1 также gofmt exit0.
SHA256/size — .artifacts/memory/sdk-source-evidence.json. Source/call-site/API
reading only: tests/fixtures не добавлялись/не менялись/не запускались;
build/compile/vet/typecheck/parser/doctor/check/module/gen/product SDK/crypto/
CLI/EXE/helper/native/admin/network/live и owner values не выполнялись/не читались.
No hard native I/O deadline, cleanup/durability/OS success или trusted-owner/
kernel/loaded-section guarantee. Public schema2 unsupported. All9, SDK workspace,
ABI/ACL/path/sharing/close/loader/GUI/disk-full/cancel/crash/power-loss/OS/DR/
resources/old-client/cross-user/service/release/comparative gates открыты.

## V2-168 — owned SDK workspace / 2026-10-26

### Admission и результат проверки

PrepareArtifacts резервирует единственный process-level workspace owner до
первого platform acquisition. Concurrent operation получает ошибку сразу; это
не очередь и не измеренная throughput оптимизация. Partial Prepare возвращает
тот же owner для checked Close. Original owner/release result одноразовые;
unknown cleanup сохраняет platform receipts и запрещает новый workspace.
Перед cleanup требуется retirement прежнего metadataCacheReader, чтобы не
удалять каталог под неподтверждённым исходным reader.

Cleanup выполняется до возврата из PrepareArtifacts. Если он не подтверждён,
private Progress сохраняет source/next verified prefix, а proof/proofs отсутствуют;
ошибка возвращается даже после успешной проверки всего batch. Это не сброс
rollback floors и не разрешение частично применить artifact set. Panic сохраняет
прежний restrictive outcome. SDK configuration/verification workflow неизменны.

### Windows acquisition и начальные bytes

Новый delivery platform adapter использует существующие x/sys и filetxn ACL/
path/type/FileID checks. Из V2-167 выделен InspectWindowsFile для уже owned handle;
он не приобретает и не закрывает file handle. WindowsRead использует тот же
алгоритм. Отдельная shared trust реализация/dependency не создана.

Ancestors temp directory удерживаются от drive root; relative NtCreateFile
пишет каждый handle прямо в reserved record. До вызова выставляется unknown
acquisition guard: panic между native output и классификацией не теряет handle.
Только exact known missing/no-create collision снимают guard без валидации.
Create-new work name содержит random96-bit suffix, ≤16 attempts; metadata/targets
создаются new-only. Directory delete sharing запрещён, traversal разрешён.
Local drive/NTFS, clean bounded2048 paths, no reparse, actual path/ACL/ID gates;
aliases/redirected profiles/UNC/non-NTFS и native ABI compatibility не приняты.

≤64 ancestors, 3 own directories и 2 reserved file slots. Initial cache ≤5
records по ≤512KiB, native WriteFile chunks32KiB/FlushFileBuffers/size check и
confirmed Close перед SDK. Context проверяется между подготовительными шагами
и writes; cleanup продолжает retirement после отмены. Это bounds на объём/число
операций, **не hard deadline** для одного synchronous native call.

### Ограниченная очистка

Enumeration использует original directory handle и aligned32KiB buffer,
FileIdBothDirectoryRestartInfo/Info. Проверяются record offsets/alignment,
UTF16 length, duplicates, FileID/type; ≤64 entries (+dot entries), bounded calls.
Основания: [Microsoft FILE_ID_BOTH_DIR_INFO](https://learn.microsoft.com/en-us/windows/win32/api/winbase/ns-winbase-file_id_both_dir_info)
и [GetFileInformationByHandleEx](https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-getfileinformationbyhandleex).

Allowed cleanup names — root.json, пять cache roles и tuf_tmp + 1…10 digits.
Это фактический prefix pinned SDK и uint32 decimal suffix текущего Go tempfile
source; другое имя требует отдельного source change. Targets должен быть пуст.
Unknown entries/reparse/directory/oversize/multi-link regular objects не обходятся
рекурсивно. Per-file native open связан с original parent и перечисленным FileID;
disposition относится к этому handle, затем checked Close и parent-relative
подтверждение отсутствия. Delete-pending/close failure не заменяются missing.
Directories удаляются по original handles после детей, ancestors закрываются
в обратном порядке. Нет RemoveAll/повторного close/path deletion fallback.
Удаление по disposition окончательно зависит от закрытия других handles:
[Microsoft FILE_DISPOSITION_INFORMATION](https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/ntddk/ns-ntddk-_file_disposition_information).

На !windows оставлен portable publisher adapter: stdlib new-only seed writes,
retained File, checked Sync/Close, bounded Readdir и identity-limited known-name
removal. Windows original acquisition/ancestor custody guarantees ему не
приписываются; Unix path race/durability acceptance остаются отдельными.

### Evidence, следующий шаг и ограничения

8 unique Go paths: gofmt -w exit0, context/path refinement5 и acquisition-guard1
также exit0. Exact SHA/size: .artifacts/memory/workspace-source-evidence.json.
Source/call-site/pinned API reading only. Tests/fixtures не добавлялись/не менялись/
не запускались; build/compile/vet/typecheck/parser/doctor/check/module/install/gen/
product SDK/crypto/CLI/EXE/helper/native/admin/network/live/owner values не запускались
и не читались. Другие роли/чаты не затронуты. Public schema2 unsupported.

SDK внутренние CreateTemp/ReadFile/Write/Close/Rename всё ещё opaque: cleanup
нашей namespace не доказывает retirement всех внутренних/unlinked SDK resources.
Source inventory: [pinned go-tuf updater v2.4.2](https://raw.githubusercontent.com/theupdateframework/go-tuf/v2.4.2/metadata/updater/updater.go).
Его local-load branches трактуют read error как отсутствие cache; persist может
отказать после обновления trusted set. Следующий P0 — source review сохранения
retained prefix/antirollback на этих ветках и публичный bounded storage seam,
без private SDK hacks/UnsafeLocalMode/второй TUF state machine. Не объявлять
эту source гипотезу воспроизведённой уязвимостью. Existing native/OS/DR/resources/
cleanup/ABI/ACL/sharing/loader/GUI/crash/power-loss/old-client/cross-user/service/
release/comparative/full9 gates остаются открытыми.

## V2-169 — сохранение принятого SDK prefix / 2026-10-26

### Причина и граница

Source review pinned [updater.go v2.4.2](https://raw.githubusercontent.com/theupdateframework/go-tuf/v2.4.2/metadata/updater/updater.go):
ошибка local metadata read может трактоваться как отсутствие cache, а persist
вызывается после изменения trusted set. Прежний retainedState читал output files,
удалял отсутствующий cache entry и мог потерять принятый prefix при поздней ошибке.
Это выявленный source gap, не воспроизведённая эксплуатация или runtime PASS.

### Изменённый поток

- `client.go`: fetcher отмечает только успешные передачи оригинальных records;
  после Refresh/каждого lookup сохраняется безопасный результат retainedState
  даже с ошибкой следующей роли. Никакого subset artifact proof.
- `retained_inputs.go`: public accepted object сопоставляется с защищённым
  исходным cache и фактически переданными SDK owned bytes. Сравнение включает
  signatures/unknown fields. Разные raw bytes с одинаковым parsed object дают
  ошибку неоднозначности. Нет повторной сериализации, private fields, unsafe
  clock/local mode, собственного signature verifier или второй TUF state machine.
- `retained_state.go`: root → timestamp → snapshot → посещённые targets.
  Не посещённый/не прочитанный cache не удаляется. Успешно принятый ранний prefix
  сохраняется без разрешения применить artifact при отказе следующего шага.
- `retained_policy.go`: проверяются timestamp version/snapshot reference floors,
  сохранение snapshot reference names/versions, неизменность signed content при
  равной версии. Проверки дополняют SDK при пропущенном local read. У промежуточного
  snapshot own version не объявляется самостоятельным монотонным счётчиком:
  [trustedmetadata v2.4.2](https://raw.githubusercontent.com/theupdateframework/go-tuf/v2.4.2/metadata/trustedmetadata/trustedmetadata.go)
  допускает его сохранение до окончательного совпадения с timestamp.
- `workspace.go`: existing checked readCache подтверждает точные initial seed bytes
  перед SDK. Неопределённый Close сохраняет прежний original reader/workspace.
- `policy.go`: typed timestamp/snapshot shape guards, положительные версии,
  nonzero expiry, обязательные ссылки и bounded nonnil references; unsigned
  publisher drafts разрешены прежним отдельным флагом. Crypto остаётся у SDK.
- `model.go`/`cache_reader.go`: комментарии отражают новую роль cache/reader.

### Ротация и полноценный proof

Сброс timestamp/snapshot разрешает только SDK-accepted последовательная цепочка
roots с изменением фактических online public keys. Проверяются и промежуточные
roots: возврат к прежнему конечному набору не скрывает ротацию. Изменение только
key-ID aliases/threshold не сбрасывает floors. Основание —
[TUF 1.0.36 §5.3.11](https://theupdateframework.github.io/specification/v1.0.36/#update-root).
Root и сброс сохраняются вместе. Если SDK удержал старый seed при пересечении
ключей, возвращается continuation без artifact authority; следующий ограниченный
проход начинается с новым root и очищенным online cache. Остальные retained
profile/pin/revocation/clock/product gates не меняются.

Перед выдачей полного batch proof все текущие SDK objects должны соответствовать
retained raw bytes. Timestamp→snapshot и snapshot→каждая загруженная target role
повторно связаны версиями и SDK VerifyLengthHashes. Это охватывает cached snapshot,
для которого upstream isTrusted path не повторяет hash check. Expiry/signatures/
delegations по-прежнему проходят обычный SDK workflow и существующий finalPolicy.

Сравнение объектов зависит от pinned SDK. В [types.go](https://raw.githubusercontent.com/theupdateframework/go-tuf/v2.4.2/metadata/types.go)
есть внутренний Key.id cache; проверенный source VerifyDelegate его не заполняет.
[marshal.go](https://raw.githubusercontent.com/theupdateframework/go-tuf/v2.4.2/metadata/marshal.go)
заполняет TargetFiles.Path при обоих FromBytes. Обновление SDK требует повторного
review/parity; это не универсальный API raw receipts. Неоднозначные сериализации
отклоняются консервативно; полный public storage/raw-receipt контракт всё ещё нужен.

### Evidence и остаток

Изменены 8 Go-файлов; gofmt -w exit0. Source reading/call-site review и SHA/size
receipt `.artifacts/memory/prefix-source-evidence.json` — единственные evidence.
Тесты/fixtures не добавлены/изменены/запущены; build/vet/typecheck/parser/doctor/
check/module/install/gen/product SDK/crypto/CLI/native/admin/live не выполнялись.
Нет проверенного сценария swallowed read, write/close fault, ambiguous raw,
intermediate rotation, rollback/equivocation, полного batch или platform cleanup.
SDK internal/unlinked file retirement не доказан. Windows native/OS/durability/
GUI/crash/power-loss/DR/resources/old-client/cross-user/service/release/comparative/
all9 gates остаются открыты; public schema2 install unsupported.

Следующий source P0: Windows public sidecar intake (`readDeliveryFile` и потребители),
его unchecked Close/identity continuity; переиспользовать original filetxn reader
с сохранением существующих лимитов. WIN-SDK-001-T содержит отдельный запрос public
bounded storage/raw-prefix receipts; shared модули и чужие чаты не изменять.

## V2-170 — общий reader и App intake

[TH-01-PUBLIC-READERS](TH-01-PUBLIC-READERS.md) выделяет общий platform I/O contract
из прежнего SDK cache pattern. App sidecars/export и SDK seed имеют отдельные
original slots. Два delivery platform adapter файла удалены; WindowsRead и pair
streaming lease сохранены, добавлен native acquisition panic guard. Source/gofmt8
only; tests/build/SDK/native не запускались. V2-169 trust/prefix semantics и все
непроверенные native/OS/resource/public install gates сохраняются.

<a id="pc-v230-gui-source"></a>

## V2-230 — GUI fixed-frame I/O и original transfer lifetime

2026-11-05, Windows **source/unverified**. Полная цель PC00…09 и old9 сохраняется.
Tests/fixtures/build/product/native execution по-прежнему запрещены владельцем.

### Source findings и переиспользование

Old exchangeUpdateGUI захватывал send/receive slices вызывающего кода в I/O
goroutine, проверял scope только до её запуска и ждал только ctx.Done. После
cancellation caller мог вернуть/изменить frame, пока read/write ещё выполнялся.
Result/done были раздельными, source не проверялся между physical pipe reads.
Native duplication не принимала ctx; post-wait image/Job/acceptance операции
имели неполные late-source gates. Это source trace, не runtime reproduction.

- `update_gui_exchange.go` выделяет прежний exchange из lifecycle. Разрешены
  только actual transcript shapes: 52→36, 36→36, 0→52 и36→0. Две fixed arrays
  по52 bytes принадлежат exchange. Send копируется до goroutine, receive — из
  private storage в caller только после actual done и current context.
- Existing ownedPipeIO/process.captureIO/ioDone переиспользованы; нет второго
  process owner. Unfinished predecessor не заменяется. Parent retains original
  record/process на failure; error source не доказывает kernel/task retirement.
  Child имеет собственный record до terminal exit; reserved dispatch ошибки
  main превращает в os.Exit(3), не обычный GUI fallback.
- Intent/operation/dependency passive channels учитываются, если исходный ctx
  действительно bound. Generic unbound/bootstrap ctx сохраняет свой cancel/
  deadline contract; App/network identity для него не создаётся. Original root
  restart token/SDK guards не заменены protocol receipt или новым controller.
- Shared ownedPipeReader/Writer расширяют existing helper reader. Они ставят
  context/source gates вокруг каждого actual Read/Write, включая protocol
  header/body/EOF у TUN/worker. Finite protocol marker сохраняет mixed scope
  error, partial write не становится full command. No App callback внутри I/O.
- Existing worker pipe goroutine получает panic finalizer: finite protocol
  failure и original I/O close вместо unhandled goroutine panic. Sequence/size/
  reply/negative retirement contract и protocol9 сохраняются. Normal worker
  buffered result/late-cause propagation остаётся отдельным незакрытым review.
- StartGUI удерживает local pointer original child даже при partial spawn;
  precommit error идёт в прежний original reaper, затем joins completed late
  I/O error. Unusable protocol не переиспользуется. После committed transfer
  original Job/process/image удерживаются до parent OS exit, не уничтожаются
  helper cleanup. OWN5 не подтверждает DOM-ready/SDK/floor/forwarding.
- Context проверяется перед каждым DuplicateHandle и после transfer. Remote
  values записаны в original partial receipt; parent не закрывает их в своей
  handle table и не открывает процесс по сохранённому PID. Non-Windows signature
  обновлена совместно; unsupported behavior не получает success fallback.
- Parent перепроверяет scope после closePipes и original child exit перед
  custody commit. Child parent-wait/session prepare/ready/final dispatch проверяют
  ctx после native/callback waits. Actual image-close receipt сохраняется при
  late cancellation; prepared — negative cleanup fact, не новая authority.

### Evidence и реальные пределы

Source7Go/4 собственных docs, exact-file gofmt7+2+1 exit0, manual caller/lock/
original buffer-owner/source/transfer/diff/byte-SHA review. Evidence:
`.artifacts/memory/gui-source-source-evidence.json`. Original document bytes и
точный прежний composite task71 сохранены; журнал только дополнен. Свежий
EOL helper сохраняет delimiter после original no-newline строки.

Нет tests/fixture edits/run, compile/build/vet/typecheck/product parser/check/
doctor/install/generation/SDK/helper/product/crypto/native/browser/admin/live/
DNS/server/OS-settings/owner-values/other-role/chat/subagent execution.
Все acceptance остаются OPEN; исходники не объявлены compile/runtime PASS.

OPEN: legacy API/fixtures/fixed shapes; caller-buffer aliases and source cancel
before queued goroutine/header/body/partial write/receive copy; mixed read/write/
context/panic errors; transcript/digest/stage mismatches; partial native handles;
source loss around scope/commit/parent-exit; prepared/ready restart; actual
Job/image/pipe/native cleanup/child lifetime/DOM-ready/floor/SDK/OS/resources.
Fixed104 bytes frame storage не total heap/RSS/kernel/CPU/time bound. Context
checks не atomic syscall cancellation и не transactional Win32/GUI commit;
уже entered native/pipe operation требует прежнего owner/reaper. No new source
watcher/monitor/I/O goroutine или network proof; existing timeout contexts и
one original exchange goroutine сохраняются. Private GUI argument v5/stages/
frame sizes/manifest digest, worker IPC9/Job8 и native=false/TUN unavailable/
schema2 install unsupported прежние.

Следующее source: root stopRequest/restartHandoff/update registration до core
GUI сохраняет scalar intent; проверить original intent-source identity и generic
control context без fake network path. Затем normal worker late-result cause и
remaining writers/consumers. Полный PC00…09/old9/SDK/W3b/interprocess/durability/
UX/release/comparative scope не завершён.
