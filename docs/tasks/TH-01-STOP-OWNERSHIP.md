# TH-01 / OFF-01…02 — Stop intent и полнота process ownership

2026-10-25, V2-150. Только PC/Windows. **Исходники, unverified**: тесты/fixtures
не добавлялись/не менялись/не запускались; compile/build/vet/check/doctor/module/
SDK/gen/helper/native/admin/network/live не выполнялись. Форматирование четырёх
Go-файлов `gofmt -w` exit0 — не доказательство работы Stop/WFP/Job.

## Основание source review

Продолжен текущий P0 по [WINDOWS](FIVE-STREAMS/WINDOWS.md). Прослежены реальные
App Start/Stop/switchMode/OnBeforeClose, update stop entry, profile retirement,
Manager.Stop/cleanup, original process reaper и обе helper capture lanes.
Не читались owner EXE/ACL/certificates/keys/env/vault/runtime config/user logs.

### 1. Зарегистрированный Stop терял identity в очереди

`stopCoreFor` увеличивал operationID и ожидал operationMu, а `stopCoreLocked`
после ожидания заново читал **текущий** ID. Поэтому старый stopRestart от режима/
updater мог принять более поздний Disconnect за свой intent и вернуть nil.
Реальные mode/update callers после nil продолжают restart/update; позднее ручное
Disconnect не должно давать старому restart новое разрешение.

Это вывод о допустимом source interleaving, не воспроизведённый runtime-инцидент.
Проверка зарегистрированного intent также нужна старому queued Stop перед более
новым Start; порядок mutex acquisition не принимается за контракт пользовательских
намерений. Окно **после успешного Stop** до restart/apply отдельно остаётся открытым.

### 2. Engine retirement не охватывал отдельные helper owners

`captureOwnedHelper` сохраняет original process/image в своей lane, если reap/
close не подтверждены. Manager.Stop очищает engine/proxy/lifecycle boundary, но
не эти глобальные protected/optional lanes. App мог получить nil от Manager.Stop,
снять WFP на Disconnect и очистить runtime pointers без итоговой проверки helper
ownership. Наличие сохранённого owner предотвращает потерю handle, но само по
себе не делает App Stop полным cleanup всех процессов.

### 3. Orphan engine нельзя забывать

`retireProfileSession` уже отказывался от commit при engine без Manager. Start
и общий Stop не имели такого же guard: могли соответственно перезаписать или
очистить engine pointer без подтверждённой retirement. Добавлен защитный инвариант;
нормальное создание Manager/engine идёт парой, фактический orphan-инцидент не заявлен.

## Реализовано в source

- `stopCoreFor` сохраняет ID регистрации и передаёт его через ожидание в
  `stopCoreGenerationLocked`. Superseded entry возвращает context.Canceled до
  teardown; mode/update callers уже прекращают продолжение на ошибке.
- При изменении intent во время teardown итог также context.Canceled, не nil.
  Cleanup старых ресурсов может закончиться, но старый caller не получает
  разрешение на restart/update. Internal stop wrapper использует тот же gate.
- После closing не регистрируются новые restart/disconnect Stop; terminal Quit
  сохраняет свой путь. При Stop registration/entry немедленно сбрасываются Ready,
  ProbeRunning и optional native cache. Поздний native result всё ещё fenced.
- `core.RetireCaptureHelpers` пытается очистить обе имеющиеся lanes без очереди,
  через их исходные `retireHelperLane`/Job/reaper/image handles. Busy или uncertain
  cleanup возвращает ошибку; другой lane всё равно получает cleanup attempt.
  Функция не создаёт процессы, не выбирает PID/имя и не запускает capture.
- App вызывает этот барьер после Manager.Stop, до WFP Release/runtime clear;
  перед публикацией нового engine — после previous Manager.Stop. Ошибка не
  разрешает release/replacement, exact remaining owner/image остаются для retry.
- Engine без Manager вызывает errRuntimeOwnerUnavailable; Start его не заменяет,
  Stop не обнуляет и не снимает защиту. Отдельный fallback cleanup owner не создан.
- Типизированные helper/engine cleanup и missing-owner failures останавливают
  обе startup candidate series. Это local custody error, не channel failure.
  `cleanup_unconfirmed`/BLOCKED сбрасывает retry; общий failClosedState исключает
  такую ошибку и для timers/network wakeup. Явный новый Start/Stop может повторить
  существующую cleanup original owners; automatic channel churn не запускается.
- Обычная network recovery сохраняет прежний backoff. Только explicit Disconnect
  может снять WFP; automatic error/restart/quit этого права не получают.

## Входы и границы

Изменены `windows/protection.go`, `corebridge.go`, `autoretry.go`,
`backend/core/helper_process.go`. Новые зависимости/manifests/generated/tests
не менялись. Manager.Stop и original engineProcess.retire не заменены.
operationMu по-прежнему единственный App lifecycle serializer; partial helper
cleanup не преобразуется в новый ресурс/процесс или разрешение повторного capture.

TryLock исключает очередь на lane, **не** устанавливает суммарный hard deadline:
original reaper имеет свой отдельный cleanup budget, Win32/handle close и две
последовательные lanes требуют OS/resource приёмки. Policy shape/route/Job exit
не исполнялись; source branch не является доказательством отсутствия утечки.

## Остаток и точный следующий P0

1. Mode/update handoff **после** успешного Stop: привязать дальнейшее restart/
   rename/apply к тому же intent/closing/source, не принимать старый nil за
   вечное разрешение после нового Start/Disconnect/quit. Использовать текущие
   Windows entrypoints и update verified-source gate, не второй lifecycle owner.
   V2-151 [transaction token/commit](TH-01-RESTART-HANDOFF.md) добавлен в source;
   native child readiness, own-image containment/pair rollback/OS gates открыты.
2. Pair EXE/helper updater/rollback/owned installer остаётся открытым; этот Stop
   barrier не делает существующий EXE-only updater совместимым с helper pair.
3. Runtime/module/locking/panic/Stop phase/owner retention/proxy/WFP/TUN/Job/image/
   clock/native/OS/cleanup/resources/EXE acceptance отложена по запрету владельца.
   Все TH-00…08, SDK integration/custody/DR/client/rollout/release/comparative
   требования сохраняются. P0 не объявлен закрытым.

## Узкое предложение для общего LESSONS

ID отменяемого действия захватывается при регистрации, не после ожидания mutex.
Подтверждение закрытия engine не заменяет retirement отдельных helper Jobs/images;
все владельцы должны участвовать в release/replacement gate. Source reasoning
V2-150; behavioral proof отсутствует. Общий LESSONS не редактируется этим чатом.

<a id="pc-v236-native-intent"></a>

## V2-236 — manual Release commit и original startup resume

2026-11-06, PC-01/08 source/unverified; весь PC00…09/old9 сохранён.

### Найдено

Release проверялся под App.mu и вызывался уже после unlock. State=stopping
обычно отклонял Start, но не был общим запретом новых Stop/retry/profile
registrations. Повторный state callback мог открыть другой producer между
проверкой и native call. Restart token имел свои entry guards; общий advance
не запрещал обход commit через менее прямой private producer.

DOM bootstrap сохранял поздний numeric operationID. Если Disconnect успевал
зарегистрироваться до DOM callback, bootstrap мог принять его ID за исходное
разрешение auto-connect. Retained-policy snapshot также мог поздно изменить
protectionDesired или restriction уже другой операции.

### Изменения

- Новый protectionReleaseCommit создаётся только после successful original
  Manager.Stop и capture-helper retirement в stopCoreRegisteredLocked при
  manual Disconnect. Требуются exact nonzero/current source registration,
  live App lifetime, desired=false/no transition/no committed restart.
  Negative teardown продолжает работать со своим revoked source; opening
  direct traffic требует current positive exception.
- Короткий App.mu commit фиксирует original policy и блокирует новый advance.
  Policy.Release вызывается once без App.mu, при прежнем caller-owned
  operationMu. Token удерживается до terminal Stop publication; он не владеет
  WFP/Job/handle и не снимает operationMu самостоятельно.
- advanceOperationLocked теперь возвращает busy при release/restart commit.
  Все4 producers проверяют error до регистрации/новых state facts. Start
  меняет desired только после успешного advance. Retry/pin eligibility не
  принимает commit; все3 consumers nullable retry launch проверены.
- OnBeforeClose сохраняет pending Close в exact phase. Завершение ставит closing
  перед снятием commit, затем запускает обычный closeAfterLifecycle: его task
  ждёт caller-owned operationMu. Нового окна между Close и successor registration
  не добавлено. Native call не выполняется под App lock и не требует UI callback.
- Policy error/panic/unfinished call остаётся finite cleanup uncertainty; panic
  payload не публикуется. Denied positive Release даёт protection_retained,
  никогда successful disconnect receipt. Defer не оставляет permanent stopping
  при Goexit/panic позже Stop; function return не означает independent native
  enforcement. Существующие backend recovery marker/owners не заменены token.
- Immutable startupIntent создаётся вместе с initial source0 в NewApp. Normal
  и updated-GUI startup используют его вместо nil/current numeric fallback;
  startForStartupIntent проверяет полный current registration, затем обычный
  Start создаёт nonzero forwarding intent. Initial0 не разрешает Release.
- DOM desired/restriction publication дополнительно требует original source,
  renderer epoch, App/caller lifetime и отсутствие commit. Observers используют
  App lifetime; их запуск после accepted bootstrap не выдаёт auto-connect.
  Отдельный auto-connect gate использует initial registration, поэтому late DOM
  не переусыновляет Disconnect. Restart staging также отклоняет active Release.

### Границы

Barrier задаёт порядок registrations внутри этого App. Он не делает транзакции
WFP/legacy registry/kernel syscall атомарными, не прерывает зависший native call,
не доказывает enforcement/reap/readiness или освобождение OS ресурсов.
App.ctx/physical path/renderer/native source могут теряться отдельно; process
launch и owned successor protocol требуют своего source/receipt. Legacy launch,
cross-process policy/service, scalar async producers остаются OPEN. Это не
реализация FR009 ProtectionController/OFF06 service и не TUN enablement.

### Evidence / OPEN

Source9Go/4docs, exact gofmt7+2+2+2 exit0; manual source/type/caller/lock/phase/
native-error/defer/initial-vs-current registration/diff review и named byte/SHA.
Exact13 originals (initial11 manifest сохранён отдельно до scope expansion),
prior task71 rev473 — `.artifacts/memory/native-intent-*`; evidence:
`native-intent-source-evidence.json`. PLAN/task old byte prefixes retained;
goal/constraints/tasks0…70/acceptance/decisions/old checks unchanged, OPEN.

No tests/fixtures/build/compile/vet/typecheck/product parser/check/doctor/install/
generation/SDK/helper/product/crypto/native/browser/admin/live/network/owner-values/
other-role/chat/subagent execution. OPEN: private API/fixtures/compilation;
current-vs-revoked/nonzero/wrap/busy registration; repeated/pending Close; nil/
native error/panic/Goexit/later Stop interruption; policy marker/partial removal;
DOM-before-after-Disconnect/renderer/lifetime/observer/initial auto-connect;
retry/pin/profile/deferred-launch ordering, native/OS/reap/race/resources/UX.
Full PC00…09/old9/SDK/W3b/interprocess/durability/UX/release/comparative OPEN;
native=false/IPC9/Job8/TUN unavailable/schema2 unsupported. No runtime/safety/
performance proof or measured token savings.

<a id="pc-v309-secret-control"></a>

## V2-309 — TestSecret вне общего mutex и один исходный владелец

**Source/unverified, 2026-11-08.** Продолжается весь PC-00…09/TH-00…08:
100 OPEN/56 NOT_RUN, без waiver. Это исходники production кнопки проверки
секрета, не Go tests/fixtures. Тесты/сборки/SDK/native/browser/network не исполнялись.

### Найденная причина и граница исправления

До среза App.TestSecret захватывал App.mu до runSecretTest и освобождал после
SSH/Cloudflare/refetchMeta. stopCoreFor требует этот же mutex до registerStopLocked.
Это прямая source-зависимость регистрации Stop от сетевого ответа, без измерения
runtime задержки. Простой unlock оставил бы чтение разных версий vault, фоновые
команды и публикацию после отзыва без владельца.

Теперь beginSecretTest под коротким App.mu резервирует один secretTestTask с
точным vault pointer, исходным App ctx/intentRegistration и closing fence. Нет
Background fallback, очереди новых тестов, appOperationScope, замены общего
operationCancel, engine/measurement/native/forwarding authority. Busy отклоняется.
Vault/decrypt, parsing, descriptors, network, Close/Wait и watcher join выполняются
без App.mu/operationMu. Final publication проверяет тот же task/vault/intent/ctx
и closing; lock освобождается до finishSecretTest (LIFO defers).

Смена intent через прежний advanceOperationLocked отзывает исходный source до
замены. Единственный watcher ждёт source.Done либо ctx.Done, отменяет context
и закрывает исходный net.Conn вне App/task locks. Socket.SetDeadline покрывает
и handshake/session phases; DialContext использует тот же ctx. Reservation и
публикация socket проверяют отзыв по обе стороны dialing, позднее соединение
закрывается. Task не заменяет исходный conn. Close, cancel и deadline сами по
себе не считаются retirement. Слот занят до возврата синхронной команды, её defers
и watcher; только затем original done закрывается и admission освобождается.
Неожиданная ошибка original socket Close (кроме already-closed) sticky:
cleanupFailed не сбрасывается повторным close, original pointer/done остаются
pending до отдельного разрешения неопределённости. Named deferred finalizer
возвращает unavailable вместо positive report при таком cleanup; логирование
идёт после finalizer и сообщает получение отчёта, не полный успех очистки.

retireTerminalProducers отменяет этот original task вместе с другими producers
до ожидания и присоединяет его original done к существующему общему budget.
Таймаут даёт прежний pending error, не стирает pointer и не разрешает replacement.
Это учёт собственного GUI запроса, не доказательство завершения всех goroutines
SSH/HTTP/OS или успешного protected Stop/Release/relaunch. Обычный Stop отзывает
intent без ожидания этого диагностического запроса под App.mu; terminal имеет join.

### Один источник значения и результата

Переиспользованы secretvault.PrepareImportSnapshot(nil), ExportReference/Get и
WithPreparedSnapshot; добавлен MetaByID для того же immutable encrypted file.
Один exact ID даёт metadata/kind/value, fingerprint считается из извлечённого
значения. До/после сети источник сравнивается штатным vault/journal fence;
callback только проверяет original ctx/intent/closing, не вызывает App/lifecycle.
Report возвращается с исходными metadata вместо отдельного refetch текущего слота.
Смена encrypted source (включая audit) отвергает результат, может требовать повтор.
Проверка не держит vault/journal через сеть и не отменяет уже отправленный запрос.
Финальная сверка — наблюдение в конкретной точке, не atomic transaction с сетью.

SSH address теперь извлекается один раз из доступных enabled VLESS descriptors;
missing/error/different hosts отказывают вместо старого IP fallback. Тот же addr
используется для TCP и ssh.NewClientConn. Channel hash command получает unique
vps-ssh-key того же vault snapshot; missing/ambiguous/invalid key даёт finite fail,
не чтение внешнего ~/.ssh/vps_key. Явная проверка самого SSH-key использует свой
exact slot. Старые pinnedServerHostKeys и mismatch refusal остаются, optional
local expectation mismatch тоже не ослаблен. SSH root/22 и legacy descriptor
selection пока не signed purpose/channel/credential binding.

### Объём ответов и честные исходы

Вместо отдельной CombinedOutput goroutine + time.After используется синхронный
Session.Run, original socket deadline/cancel; SSH Client.Close/Wait и Session.Close
предшествуют release слота. Один synchronized stdout/stderr sink заранее выделяет
4096 payload bytes, overflow отменяет исходный task. SHA префиксы принимаются
только как16 lowercase hex; raw server stdout/transport errors не попадают в report.
Совпадение hash prefix не объявляется работой всей конфигурации/VPN-пути.

Cloudflare использует original ctx, fixed HTTPS URL и собственный transport с
прежним ProxyFromEnvironment; redirect запрещён. Headers16KiB, body читается
до16KiB+1 и превышение отклоняется; compression/keepalive выключены. Полный
JSON разбирается без trailing document, HTTP non200/invalid/nil Result/inactive
не получают success. Исправлен прежний nil dereference при success=true/result=null.
Raw response error/status string/token ID не отражаются в UI; finite сообщения
не выводят причин «блокировка/токен отозван/сеть умерла» из одного отказа.
Credentials используются по назначению, это не zeroization либо глобальная
гарантия отсутствия секретов в библиотечных/IPC/native producer paths.

### Доказательства и незакрытое

4 Go (включая новый secret_operation.go),4 docs;7 existing+1 new before snapshot,
prior task71 revision717,17 immutable reviewed source SHA. Manual source/caller/
lock/defer/error/late-result review; gofmt4+4+2+3+2 exit0. Собственный source-text
editor сначала имел ошибку экранирования JS template; исправлен до замены body,
продукт не исполнялся. Byte evidence в .artifacts/memory/bootstrap-v309-secret-control-*.
gofmt/SHA не compile/typecheck/runtime/native/race/security/performance evidence.

OPEN: native/SDK cancellation and resource retirement; all terminal/Stop races,
noncooperative OS/read/close/decrypt, bounded whole process and admission rate;
other vault facade App.mu-through-local-I/O; unsigned legacy SSH host-key rotation/
purpose/target/credential delivery, local expectations file unbounded read and
optional malformed/missing semantics, legacy UDP old-IP fallback. Existing
descriptor selection is not an authenticated exact SecretID→server binding.
UI/Wails multi-return contract and generated bindings не регенерировались/не приняты.
Нет measured latency/throughput/token savings или доказанного superiority.

Source lesson: не держать App.mu через сетевой producer; отменять точный original
source/resource, сохранять busy до фактического возврата, связывать report с теми
же encrypted inputs. Это source-derived правило, не наблюдение живого deadlock.

Полный PC04 all8flows/PC00…09/old9/PC01W3bW4c/PC03 signed/native остаётся OPEN.
Bootstrap/bridge/native/chain/relaunch/parent/helper gates=false, ranBeforeJob sticky,
GUI6/R1/U1/IPC9/Job8, TUN unavailable/schema2 unsupported сохранены.


<a id="pc-v310-diagnostic-source"></a>

## V2-310 — общий владелец ручной диагностики и scope результата

**Source/unverified, 2026-11-08.** Полный PC-00…09/TH-00…08 активен,
100 OPEN/56 NOT_RUN, без waiver. Ни production GUI диагностика, ни tests/fixtures/
build/typecheck/SDK/native/browser/network не исполнялись. Новые файлы — обычные
production Go components manual_diagnostics.go и diagnostic_udp.go, не *_test.go.

### Причины и существующие API

В App.runTest UDP payload из случайных байт назывался QUIC Initial; любой ответ
давал «UDP работает/HY2 можно», тишина — «UDP отфильтрован». HY2 мог использовать
старый запасной IP или последний дескриптор, skip возвращал true. Freeze wrapper
создавал Background ctx и начинал allTunnelOK=true: отсутствие tunnel steps могло
вернуть успех. Глобальный UI aggregate дополнительно считал optional OS-route
fail обязательным отказом tunnel workload. Это source findings, не измеренный сбой.

V309 original task расширен для всех6 известных ручных GUI diagnostic IDs.
Переиспользованы core.AdmitDiagnostic (существующая optional lane), negative
intent/path sources, joinAppProducerContext, appliedProbeSubject.current,
render.LoadDescriptorSnapshot/Selectable/ValidateCredentials и vault snapshot.
Auth roles текущего Channel не описывают signed SSH management-purpose/endpoint;
другая trust chain или разрешение arbitrary host keys не добавлялись.

### Владение, бюджет и кэш

secret_operation.go сохраняет исходного владельца, теперь manualDiagnosticTask:
один shared slot для RunTest и TestSecret; без очереди, второго engine, замены
operationCancel либо appOperationScope/probe/native/forwarding authority. Под
коротким App.mu — reserve/final publish/retire; vault/network/close/joins вне него.
Network requests наследуют исходный path, local facts от него не зависят.
Stop/новый intent отменяет original ctx/watcher/socket; path revocation проверяется
на phase/publication boundaries, а не объявляется hard immediate I/O cancellation.

До обращения к сети резервируется Endpoint workload: secret1/(16KiB+1), UDP2/
2*(1200+4096+1) application bytes. Volume сначала снимет и проверит свой полный
план (1…8 размеров, каждый≤64KiB), резервирует его штатным AdmitDiagnostic и
передаёт original release владельцу. Protected lane/limits не изменены. Release
только после возврата original call/defers/watcher; unexpected socket Close error
sticky, pointer/done/budget остаются pending. Cancel/deadline не retirement/refund.
Это ceiling логических application requests/body, не wire/handshake/kernel/RSS/
library task или hard scheduling bound. Пустой/неприменимый секрет может потратить
консервативную reservation без сетевого запроса; мгновенные отказы не refund.

RunTest отвергает неизвестные IDs до cache/log allocation; cache максимум6 IDs.
DTO использует прежний nullable OK. ListTests копирует steps и сохраняет original
checkedAt; original done должен быть закрыт, intent/path current, cleanup без
неопределённости. Иначе observation становится unavailable, не retagged PASS.
Save/Add/Delete/Verify перед попыткой записи сбрасывают credential cache и ставят
sticky flag текущему task. Pending TestSecret/credential outcome проверяет этот
flag перед публикацией и в deferred finalizer. Это не внешний file watcher, TTL
или atomic network+vault commit. Clock/epoch факт не обновляется чтением списка.
После возврата watcher/close finalizer повторно проверяет исходный parent lifetime,
intent/path/deadline под App.mu. Собственная отмена не маскирует отмену parent.
Result rejection фиксируется до original done; cache не покажет positive в окне
между завершением очистки и возвратом Wails. Подтверждённое завершение ресурсов
при stale scope освобождает owner/budget, но отвергает observation; uncertain Close
по-прежнему не освобождает ни одно из них. Последующий IPC/runtime не атомарен.

### UDP и границы протокольного ответа

Старые testServerHost/testEndpoints fallback удалены. Для HY2 нужен один enabled
descriptor из одного snapshot; при reported running используется его active ID,
иной protocol/missing/ambiguous не запускают запасной сервер. Это объявленное
назначение, не подтверждённый фактический selector/transport/защищённый маршрут.
Общий UDP endpoint8.8.8.8:443 остаётся явно названным встроенным назначением,
не fallback VPS и не доказательством всех UDP destinations/IPv4/IPv6 клеток.

Формируется1200-byte long-header invariant request с reserved version0x0a0a0a0a,
двумя случайными8-byte CID; максимум2 send/read на одном original connected UDP
socket, per-attempt3s/earlier original deadline. Read buffer4096+1, превышение
отклоняется. Проверяются version0, high bit, exact swapped CID, nonempty complete
4-byte version list без zero/offered version. Код сопоставлен с layout установленного
официального quic-go v0.59.0-sing-box-mod.4/internal/wire/version_negotiation.go;
dependency source SHA сохранён отдельно, library не исполнялась/не обновлялась.
Вызвать internal parser из main нельзя из-за Go internal boundary; локальный
минимальный validator не является QUIC stack или приёмкой interoperability.

VN остаётся unauthenticated. Positive означает подходящий ответ этого endpoint
через текущий OS route с challenge correlation, не HY2 auth/obfs/TLS, доступность
всего UDP или protected egress. Тишина/неподходящий ответ/skip остаются null с
finite detail. Другая VPN/WARP может находиться на маршруте; причина не выводится.

### Credentials и объём

Credential check использует реальные selectable profile channels и штатный
ValidateCredentials на одном PreparedImport, затем encrypted-source fence.
Точные SecretID/role resolution не заменены первым значением по kind. Empty set
не PASS; формат проверяется фактически, старый audit не подменяет его. Сетевая
доступность не заявлена. Проверка admin token сообщает только elevation, TUN gate
остаётся закрытым; CoreReady — reported факт, не самостоятельный path check.

Freeze наследует App ctx и проверяет существующий applied subject до/после
workload. Mixed listener адрес/порт из render constants, не duplicated1080.
Production URL только HTTPS с host, без userinfo/fragment; отсутствующий target,
budget или original scope — unavailable. Captured sizes считаются expected и
observed отдельно; пустой/неполный план не возвращает true. TUN comparison не
выполняется и имеет skipped/false placeholder, не fake success. EOF/reset около
deadline больше не превращается по elapsed heuristic в silence. Finite messages
не выводят raw URL/errors, no-data не диагноз DPI, summary только observed counts.

В существующем api/backend.ts только freeze-check получает scoped aggregation:
четыре текущие tunnel names8/16/32/64KiB должны присутствовать по одному с terminal
measured status; missing/duplicate/unknown plan → null. Exact OS comparison names
не участвуют в tunnel outcome, все observations остаются видимыми в steps.
Protected probe mapper/informational allowlist/роль обязательных проверок не
ослаблены; generated bindings/DTO schema не менялись. При смене производственного
плана размеров этот adapter требует согласованного обновления, пока нет typed plan DTO.

### Доказательства и остаток

6Go1TS4docs;11 originals snapshots (9existing2new), prior task71 rev719,
21 immutable reviewed SHA +1 cached dependency source. Manual source/caller/
null/empty/malformed/late result/lock/defer/budget/mutation review и gofmt6+4+1+2+2
exit0. UI scope promotion имеет отдельный before snapshot и progress checkpoint;
сам source editor/metadata tools не запускали продукт. Byte evidence —
.artifacts/memory/bootstrap-v310-diagnostic-source-*; никаких новых PASS.

OPEN: compile/ABI/generated binding/fixture migration/typecheck/browser/native/
QUIC interoperability/actual endpoint response; cancellation/resource/OS/library
goroutine retirement, network down during blocked call, early-close/body read
semantics, interprocess external vault/profile writes, measured full-resource/
latency/throughput/clock/freshness/route-independence. Cached source guards —
process-local negative observations, не durable receipts или network authority.
Legacy SSH root22/static host pins/purpose-bound delivery и unbounded optional
local expectation file остаются separate debt. Любые trust/SDK replacements только
через опубликованный поддерживаемый API и отдельный scoped acceptance.

FullPC00…09/old9/PC04all8flows/PC01W3bW4c/PC03signed-native OPEN;
bootstrap/bridge/native/chain/relaunch/parent/helper gates=false, ranBeforeJob sticky,
GUI6/R1/U1/IPC9/Job8, TUN unavailable/schema2 unsupported. Нет measured superiority
или token savings; Android/shared/configs/harness/other chats/subagents не затронуты.


<a id="pc-v311-ssh-expectation"></a>

## V2-311 — bounded SSH expectation и доставка отчёта через Wails

**Source/unverified, 2026-11-08.** Продолжается весь PC-00…09/TH-00…08,
100 OPEN/56 NOT_RUN, без waiver.3 Go/4 docs: secrettests.go, новый production
ssh_expectation.go и узкая поправка facade app.go. Это не Go tests/fixtures;
product/SDK/parser/check/build/typecheck/native/browser/network не исполнялись.

### Локальное ожидание и ресурс

Старый loadKeyExpectation читал os.ReadFile без размера, пробовал следующий путь
после любой ошибки и принимал первый JSON с непустым fingerprint. Отсутствие,
denied/размер/схема смешивались; два противоречащих файла не обнаруживались.

Переиспользован filetxn.NewBoundedReadSlot(4096), обычный stored-file policy,
без SelectedInput downgrade/нового I/O engine/timeout goroutine. Один domain и
один reader живут весь процесс, не создаются на retry/path/request. Domain.TryLock
отклоняет contention без очереди; owner фиксируется до path/read работы.
App.mu/operationMu не захватываются. Domain guard держит только свой read/parse,
не SSH/HTTP или App callback. После read требуется Retired; byte buffer выходит
из BoundedReadSlot только после проверенного закрытия исходного файла.

Неопределённое открытие/Close сохраняет reader и original domain owner; тот же
manualDiagnosticTask получает cleanupFailed, поэтому прежний finalizer не
освобождает task/done/optional budget. Новый caller не присваивает себе старой
resource custody. При блокирующем read остаётся исходный stack/task, не wrapper
goroutine, объявленная завершённой по deadline. Cancellation проверяется до/между/
после read, но штатный reader не принимает ctx: hard native cancellation/read/
Close/ACL/path deadline и реальный Windows cleanup этим не доказаны.

Кандидаты прежние: .tmp/expectations.v1.json рядом с EXE и в рабочем каталоге.
Пути захватываются как absolute один раз; exact duplicate схлопывается. Максимум2
последовательных source reads по4KiB, без path map неограниченного размера.
Ошибка разрешения пути не превращается в разрешение иной source priority.
All absent с retired reader возвращает nil,nil и явный skip. Любая иная read
ошибка, busy/poisoned domain, oversized input или malformed/conflicting present
input возвращает только finite sentinels и останавливает SSH до dialing.
Один валидный файл при отсутствующем другом допустим; два должны согласоваться
по fingerprint. Raw bytes/errors/absolute paths/expected value не отражаются UI.

### Формат и границы доверия

Closed object содержит ровно private_key_fingerprint со строкой16lowercase hex.
Использован существующий secretHashValid; case aliases/duplicate members/null/
unknown fields/нестроковые значения/пустой объект/trailing document отклоняются.
Private metadata object decoder не экспортируется из metadata и привязан к иным
типам; новая локальная функция разбирает только эту ограниченную one-key форму.
Дополнительные/старые неканоничные поля теперь дают unavailable, не silent skip;
автоматической миграции/перезаписи operator файла или ACL не выполняется.

Это prefix fingerprint локального vault private-value, не ssh-keygen host-key
fingerprint, не trusted pin и не remote identity. Старый mismatch refusal и
pinnedHostKeyCallback сохранены без accept-any/TOFU/keyscan fallback. Отсутствие
файла оставляет существующую optional local check непройденной/skipped, а не
производит local expectation PASS. Совпадение описывается на момент чтения;
изменение файла после read не становится atomic revocation сетевого сеанса.
Signed SSH management-purpose/endpoint/root22/pin rotation остаются отдельным
непринятым контрактом. Никакой второй trust chain не создан.

### Обнаруженный контракт Wails

Прочитаны cached Wails v2.13.0 internal/binding/{reflect.go,generate.go,
boundMethod.go}, закреплённой в go.mod версии. Reflect регистрирует все outputs;
generator для3 outputs объявляет Promise первого типа. BoundMethod.Call содержит
обработку только OutputCount1 и2: при3 результаты вызова не присваиваются и
возвращаются nil,nil. Это static source conclusion о потере report/error после
выполнения метода, не установленная ошибка запуска приложения или runtime trace.
Ранее App.TestSecret возвращал report,metadata,error, а generated App.d.ts и
api.testSecret уже ожидали единственный SecretTestReport. Store.perform отдельно
перечитывает metadata после action, поэтому третье Go значение не было нужным
JS контрактом. Других production Go callers TestSecret точечный поиск не нашёл;
это не исчерпывающий AST-аудит всех Wails методов.

Facade теперь возвращает (report SecretTestReport,resultErr error), сохраняет
original task/source/vault/credential/cleanup guards, deferred downgrade и finite
errors/logging. Внутренний runSecretTest по-прежнему использует same-source meta;
лишняя metadata не отправляется отдельным RPC return. Intended JS shape прежний;
generated bindings/TS/manual adapters не редактировались и SDK не запускалась.
Cached3 source SHA зафиксированы, но ABI/native/compiled EXE/browser приёмка,
regeneration и возможная Go fixture migration остаются OPEN.

### Доказательства и остаток

Manual source/caller/error/null/size/duplicate/conflict/owner/defer/return-shape
review; gofmt2+1 exit0.7 before snapshots (6existing1new), task71 rev722,
18 immutable reviewed source SHA +3 cached Wails source. Scope App.go расширен
только после caller/source evidence, отдельного snapshot и progress checkpoint.
Byte evidence — .artifacts/memory/bootstrap-v311-ssh-expectation-*.
Ни реальный expectations file/vault/keys, ни пользовательские логи не читались.

OPEN: compile/typecheck/generated API/fixtures/native/browser, denied/missing/
reparse/hardlink/ACL/interprocess/fs race/hang/panic/Goexit/uncertain Close;
actual Wails rejection/data delivery/cancel-after-body/teardown, SDK/native tasks,
full-process resource/time/secret erasure/route/throughput/comparative proof.
Incoming external mobile-SFA report не использовался как Windows evidence и не
изменяет запрет PC execution или роли; чужие чаты/планы/runtime не читались.
FullPC00…09/old9/PC04all8/PC01W3bW4c/PC03signed-native остаются OPEN;
bootstrap/bridge/native/chain/relaunch/parent/helper gates=false, ranBeforeJob sticky,
GUI6/R1/U1/IPC9/Job8, TUN unavailable/schema2 unsupported. No measured superiority
или token savings. Android/shared/configs/harness/dependencies/Git не менялись.


<a id="pc-v318-session-retirement"></a>

## V2-318 — исходный operation sampler и отдельный protocol API gap

**Source/unverified, 2026-11-08.** PC-01/06, полный PC00…09/old9 active,
100 OPEN/56 NOT_RUN, без waiver.4 Go/4 docs; tests/fixtures/build/typecheck/
SDK/options/parser/worker/native/browser/admin/network не выполнялись.
V317 был progress: изменён strict runtime graph; нет blocker streak.

### Самостоятельно проверенный protocol source

Закреплены sing-box1.13.19, sing-quic0.6.4-d83826c306d7, sing-mux0.3.5;
точные source SHA в dependency-source-identities собственного evidence prefix.
Module cache/manifests/SDK/transport code не изменялись и не исполнялись.

| Слой | Существующая семантика исходников | Недостающее подтверждение |
|---|---|---|
| HY2 | sing-box передаёт constructor ctx в client. Shared offer живёт от client lifetime; completeOffer пишет c.conn под connAccess | Atomic original source fence до shared-session publication |
| HY2 Close | CloseWithError marks pending discarded/cancels и возвращает без join; pending.done закрыт до discard conn.closeWithError | Original pending/auth/Close completion, не факт cancel/return |
| VLESS mux | offer держит access через offerNew; connections.PushBack после carrier setup. Reset под тем же access закрывает sessions и очищает list; Close возвращает nil | Source-bound publish permit и проверенные Close outcomes; mutex serialization не является platform path retirement |
| TLS | raw native handoff предшествует ClientHandshake; error path закрывает conn, возможен bounded ECH retry | Original protocol attempt/late Close receipt после raw socket transfer |
| WS/httpupgrade | Handshake идёт после underlying dial; stock Close возвращает nil. WS early-data может вернуть lazy connection до actual dial | Не считать adapter.Close или wrapper return original handshake/session retirement |

InterfaceUpdated не решает этот контракт: HY2 вызывает тот же CloseWithError,
VLESS закрывает transport/reset mux без scoped pending receipt. Blind reset также
может разрывать healthy established streams. Foreground DialContext wrapper,
dynamic Context.Done, packet/IP observation и срок ожидания не дают private
shared-session publication authority. C1 в WIN-SDK-001 относится к trust SDK;
смешивать его с sing-box/sing-quic protocol lifecycle нельзя.

### Local request WIN-TRANSPORT-001 — контракт, не реализованный API

Нужен явно версионированный protocol seam в согласованном frozen stack:

1. Original unpublished offer/carrier owner до DNS/native/clone/spawn, bounded
   count+bytes и точные worker/source/path/endpoint/role/trust-revision bindings.
   Native socket, authenticated shared carrier и consumer stream — разные owners.
2. Private before-publication gate у реального c.conn/session-list publication.
   Source revocation и публикация имеют одну документированную linearization
   boundary и ацикличный lock order. Callback под SDK mutex не может произвольно
   входить в Manager/App или вызывать platform I/O/retirement.
3. Отзыв именно unpublished work старого source/path без необоснованного reset
   уже established healthy carriers. Новые flows всё равно требуют актуальных
   platform/trust/forwarding gates; API не продлевает pin/profile или network proof.
4. Join original pending/handshake/native callbacks/descendants/checked Close с
   outcome joined/failed/uncertain и original identity. Отмена ожидания не равна
   завершению. Receipt нельзя изготовить из public bool/digest/JSON.
5. HY2 pending.done должен различать protocol result и завершение discarded Close;
   mux/transport cleanup errors не скрываются nil Close. Нужны bounded passive
   observation/retained owner и запрет successor при uncertainty, без reaper fanout.
6. Exact API/tags/source/licence/SBOM/rollback и платформенная migration/acceptance
   остаются отдельными. Здесь нет SDK fork, dependency upgrade, actor reset,
   изменения shared, сообщения другому чату или обещания уже доступного API.

Это конкретизирует незакрытый PC03/06 контракт. Он не блокирует остальные
Windows P0 source задачи и не закрывает shared-session acceptance.

### Найденный Windows-owned дефект и реализация

observeOperationLifetime запускал goroutine, но возвращал только CancelFunc.
Manager Start/Switch/Reload вызывали его через defer уже после finishLifecycle:
active мог быть снят, operation.done закрыт и новый running watch запущен, пока
старый sampler ещё находился в callback. Cancel не был original task receipt.
Это допустимое исходниками interleaving, не воспроизведённый runtime случай.

Теперь operation_lifetime.go владеет одним original sampler: separate stop/done,
one stop via sync.Once, original probe/context и abnormal-return guard. Запись
lifecycleOperation.lifetime выполняет только original caller; Stop не читает
этот pointer и по-прежнему отменяет operation и ждёт original operation.done.
Sampler не вызывает engine/Manager callbacks. Его panic/Goexit сначала отменяет
trust context finite trust_unavailable, затем закрывает done. Sampling stop не
отменяет успешный operationCtx; parent cancel/actual sampled trust error сохраняются.

finishLifecycle до fresh final trust/source/proxy checks запрашивает stop и join
original sampler вне Manager locks. Если context уже отозван или отозван во время
join, lifecycle owner фиксирует Stopping своего active op, пытается cleanup original engine/protection/probes,
потом ждёт actual sampler return. Failed cleanup остаётся joined cause; повторный
terminal cleanup для этой ветки не нужен. Active op удерживается; новый Start/
Switch/Reload не может обойти pending original sampler. completeOperationLocked
дополнительно требует original lifetimeReturned, иначе Error и active retained.

После sampler join текущий terminal gate ещё раз проверяет restored/current probe,
source, deadline, proxy и Stop intent перед Running. Rollback marker отзывается
при terminal failure прежним общим путём. Нового timer worker/reaper или scope
для отключения защиты не создано. Existing running lifetimeWatch сохранён отдельно.

### Границы и сверка без исполнения

Вручную просмотрены три production observe callers, terminal complete callers,
Stop active/done wait, cleanup outside Manager locks, appProtectedProbe →
sessionLifetime (local clock/validity), old/new no-index diff и new source.
Gofmt4+1 exit0;8 snapshots7existing1new; task71rev743 archived;
15 reviewed +7 cached source SHA. No parser/SDK/worker/native command executed.
Ни тесты/fixtures, ни generated bindings не добавлялись/менялись/запускались.

Join не делает произвольный LifetimeRemaining прерываемым. При нарушении его
local/no-I/O/no-Manager-callback contract операция может удерживаться до actual
return; Stop сохраняет свой wait budget, но не выдаёт ложный completed/restart.
Cleanup также остаётся реальной потенциально blocking платформенной операцией;
hard wall-time/OS preemption/latency/RSS/global Go-task bound здесь не доказаны.
Existing running lifetimeWatch cancel/clear без join может пересечь следующую
операцию; исходный running-owner return/rearm — следующий самостоятельный срез.
Также остаются synchronous callback/abnormal main lifecycle exit, full Manager
watcher ownership, pinned transport pending/session/private hook и API/ABI gates.

OPEN: compile/caller/test compatibility, stop-before-first-sample/active callback/
normal success/fault/panic/Goexit/cancel/late return/rollback/cleanup interleavings,
running-watch retirement/rearm, shared HY2/mux/TLS/WS/upgrade publication, native
resources/PKI/egress/OS/GUI/installer/release/comparative/fullPC00…09/old9.
Все bootstrap/bridge/native/chain/relaunch/parent/helper gates=false,
ranBeforeJob sticky, GUI6/R1/U1/IPC9/Job8, TUN unavailable/schema2 unsupported.
Android/shared/configs/harness/dependencies/Git/server/system network/secrets/
other-role state/chats/subagents не менялись/не выполнялись.


<a id="pc-v319-running-lifetime"></a>

## V2-319 — возврат running-watch и уведомление без App callback

**Source/unverified, 2026-11-08.** PC-01/06, полный PC00…09/old9 active,
100 OPEN/56 NOT_RUN, без waiver.7 Go/4 docs; tests/fixtures/build/typecheck/
SDK/options/parser/worker/native/browser/admin/network не выполнялись.
V318 изменил authoritative source/owner receipt: progress, не blocker streak.

### Исходный пробел

cancelLifetimeLocked отменял context и сразу обнулял pointer. Следующий sampler
мог появиться, пока previous watch ещё находился в LifetimeRemaining/cleanup/
fault handler. Кроме того, handler брал App.operationMu: прямой join из Start/Stop,
который уже держит этот mutex, создал бы цикл ожидания. Это source interleaving,
не воспроизведённый инцидент. Background timer/cancel не означает actual return.

### Владение и переходы

| Граница | Реализация в исходниках |
|---|---|
| Capture | lifecycleOperation.runningLifetime получает exact original watch под Manager.mu; cancel не очищает Manager slot |
| Start/Switch/Reload | Join predecessor вне Manager locks до initial lifetime sample/engine mutation/operation sampler. Revoked wait сначала пытается original cleanup, затем держит active до actual done |
| Late sampled fault | cause читается после done. Actual old trust fault не теряется при смене generation; positive successor abort/cleanup, local trust reason retained |
| Stop | Original engine/protection cleanup предшествует join. StateStopped/done не объявляются до полного original return; old disposed trust failure сама не выдаётся за cleanup failure ручного Disconnect |
| Terminal Running | Требуются V318 operation sampler и captured running-watch receipts. prepareLifetimeLocked резервирует fresh owner до go; launch идёт после active=nil под тем же Manager lock |
| Immediate fresh fault | Watch не может потерять первый error из-за ещё active predecessor: active уже снят к spawn, final Manager lock сериализует expiry claim |
| Self-expiry | Сам watch резервирует cleanup lifecycle op; этот op не ждёт собственного full watch. Его done означает cleanup completion; отдельный Manager watch slot ещё retained до notification/final done |
| Abnormal exit | Watch panic/Goexit ограничивает trust; abnormal expiry cleanup сохраняет ErrEngineCleanupFailed и завершает negative lifecycle. No false successful resource receipt |

Новый running_lifetime.go отделяет original running task от local trust predicates
в lifetime.go и original operation sampler в operation_lifetime.go. Main methods
Manager сохраняют единственного lifecycle owner; второго scheduler/reaper нет.
cause пишет только original watch, другие callers читают после done. Старый
watch slot освобождается только после этого receipt; uncertainty native cleanup
по-прежнему сохраняется в original engine/protection owners и joined error.

### Доставка ошибки через existing watchdog

Производственный Go API SetLifetimeFaultHandler/LifetimeFault заменён на
SetLifetimeFaultSignal. Единственный production caller в corebridge передаёт
immutable App.lifetimeWake, созданный NewApp с capacity1. Signal never closed
приложением; send nonblocking и coalesced, без report/error/credential payload.
Old App.handleLifetimeFault, ожидавший operationMu внутри watch task, удалён.

Existing watchCoreTicks выбирает этот wake вместе с обычным ticker. На wake
считывает **текущий** runtime Manager/config, active-start state и StateError,
после чего использует прежний failSession/stopObservedSession: current config,
private report intent source/session, LifetimeError и protection intent ещё раз
проверяются под App operation owner. Stale signal не переносит fault на новый
здоровый Manager. Это local restriction, не network-health/quarantine sample.
Wake-only path не запускает remote probe; обычные power/tick guards сохраняются.
Manager сохраняет полный trust+cleanup cause до wake; historical report получает
negative/stale mark, private intent source Clone остаётся identity-only.

После expiry Manager cleanup завершается до best-effort signal. Даже если App
watchdog уже держит operationMu и вызывает Manager.Stop, original watch не ждёт
App callback: nonblocking send возвращается, final done закрывается, Stop может
join. Текущая startup ensureWatchdog переиспользуется; нового background worker
или per-fault goroutine нет. Если watchdog занят/Start идёт, retained Manager fault
и periodic tick остаются; это не обещание мгновенной UI delivery/hard latency.

### Сверка и ограничения

Вручную просмотрены four original cancellation captures, three positive joins,
Stop cleanup/join/complete, original self-expiry claim/finalizer, fresh reserve/go
order, current report intent source и existing watchdog failed-owner path.
Source-only expected interleavings: early/late cancel, active callback, late trust
fault, expiry vs Stop, immediate fresh fault, self-cleanup before watch done,
coalesced/stale wake, App operationMu held during Stop. Они не запускались.
Reviewed old/new no-index diffs, new source и touched gofmt7 exit0;
11 snapshots10existing1new, task71rev746 archived,7 unchanged reviewed SHA.

Bound в inspected production graph — один tracked background lifetime task на
Manager; V318 operation sampler запускается только после previous running done,
а fresh running task — после operation sampler done. Main synchronous callback,
другие Managers/App/SDK tasks и всё kernel/RSS/global rate этим не ограничены.
Непрерываемый callback/cleanup может задержать actual join; retained owner не
освобождается по timeout. Stop wait budget для active lifecycle не является
hard deadline его собственной native cleanup или final watch join.

Go API/source/fixture compatibility остаётся unverified; tests/generated bindings
не менялись. Synchronous main Start/Switch/Reload abnormal return/Goexit и cleanup
перед terminal completion требуют следующего P0 source review; полный App
watchdog lifetime/retirement, private protocol publication/join WIN-TRANSPORT-001,
CPU/RAM/FD/latency/concurrency/native/OS/old-EXE/ABI/packaging/release/comparative
и все PC00…09/TH00…08 остаются OPEN. Source/format/diff/SHA не доказывают runtime.
Все bootstrap/bridge/native/chain/relaunch/parent/helper gates=false,
ranBeforeJob sticky, GUI6/R1/U1/IPC9/Job8, TUN unavailable/schema2 unsupported.
Android/shared/configs/harness/dependencies/Git/server/system network/secrets/
other-role state/chats/subagents не менялись/не выполнялись.


<a id="pc-v320-lifecycle-return"></a>

## V2-320 — ordinary lifecycle return и порядок отрицательного unwind

**Source/unverified, 2026-11-08.** PC-01, полный PC00…09/old9 active,
100 OPEN/56 NOT_RUN, без waiver.5 Go/4 docs; tests/fixtures/build/typecheck/
SDK/options/parser/worker/native/browser/admin/network не выполнялись.
V319 был progress с original running-watch owner и cap1 notification.

### Источник дефекта

Manager defers проверяли recover()!=nil. Switch/Reload по умолчанию сохраняли
finalState=Running для ordinary rejection/verified rollback; Goexit в callback
не даёт nonnil panic payload и мог пройти к этому terminal decision без normal
body return. Start также не гарантировал полный cleanup для такого выхода,
Stop не имел собственного body/finalizer boundary после захвата active owner.
Sequential cleanupCall конвертировал panic, но Goexit мог пропустить следующие
protection/engine/proxy steps. Это source finding, не runtime reproduction.

### Реализация по компонентам

- lifecycle_return.go: operation.runBody вызывает тело в **исходной goroutine**,
  private bodyReturned выставляется только после обычного возврата. Start,
  SwitchChannel, Reload и stopOwnedOperation используют этот общий witness.
  Три тела сохранены в scoped closures; ordinary errors, rejection и rollback
  branches сопоставлены whitespace-insensitive original diff. В частности,
  verifyPrevious продолжает сохранять cause в original named return до rollback.
- finalizeLifecycle откладывается напрямую, поэтому recover видит original
  panic. Отсутствие ordinary return, включая nil payload/Goexit, создаёт typed
  ErrEngineCleanupFailed, снимает positive rollback outcome и отменяет original
  operation. Payload values не копируются в сообщения — только fixed stage/type.
- Negative owner заранее регистрирует final StateError/done и original watch
  joins, затем выполняет cleanup. Если cleanup сам прекращает goroutine, уже
  registered defers продолжают этот порядок; rescue worker не создаётся.
  Foreign active operation не очищается. Pending original callbacks удерживают
  active/done; успешный timeout/cancel не изготавливает retirement receipt.
- cleanup.go: original lifecycleCleanup receipt опубликован до callback. All
  cleanup stages зарегистрированы до первого вызова; reverse defer registration
  сохраняет прежний execution order: protection probes в исходном порядке,
  затем original engine, затем owned proxy restore. Каждый stage пишет outcome
  при ordinary return либо abnormal unwind; итоговый result/drained хранится в
  original operation даже когда cleanup не вернулся вызывающему из-за Goexit.
- Drained — завершение frames последовательности, **не доказательство** успешного
  Close/Wait/Release native resources. Error остаётся ErrEngineCleanupFailed;
  original Engine/proxy/protection owners и platform cleanup gates сохраняются.
  Abnormal fallback не повторяет native sequence, если exact original receipt
  уже drained; ordinary lifecycle cleanup error flow остаётся прежним.
- completeOperationLocked требует bodyReturned для positive Running/Stopped,
  а также V318 operation/V319 previous-running task receipts. Self-expiry имеет
  отрицательный cleanup op и separate full watch owner, не получает fake witness.
- finishLifecycle имеет scoped Manager mutex release на любом unwind. Если
  callback прекращает terminal path с удержанным lock, provisional Running
  сначала заменяется Error, только потом mutex освобождается для negative owner.
  Own appProtectedProbe.GuardLifecycleCompletion source имеет defer для partial
  downstream App/epoch release; supplied adapters обязаны соблюдать этот contract.
- signalManagerFault переиспользует V319 nonblocking existing watchdog wake.
  Manager expiry включает cleanup sequence error в original cause. Нового API
  для trust/transport, goroutine, timer, direct fallback или native permit нет.

### Порядок и ordinary behavior

| Сценарий исходников | Terminal behavior |
|---|---|
| Ordinary successful Start/Switch/Reload | Original body witness → original sampler join → fresh trust/source/proxy/Stop gate → Running |
| Ordinary rejection/verified rollback | Прежние state/rollback checks сохранены; без terminal error error identity не оборачивается дополнительно |
| Ordinary owned Stop | Original cleanup → full running-watch join → Stopped при отсутствии cleanup error; caller ctx error возвращается отдельно как раньше |
| Body panic/Goexit | Cancel original → negative cleanup sequence → actual watch joins → Error; no positive rollback/Running/Stopped |
| Cleanup callback Goexit | Remaining registered stages/unwind still run; original result retained; no claim of successful native closure |
| Terminal callback panic/Goexit | Manager mutex releases with Error; common finalizer enters exact original negative owner |
| Callback/cleanup never returns | Original owner/active remain pending; no timeout-as-receipt, successor or detached rescue task |

Goexit не превращается в обычный Go return: исходная goroutine по-прежнему
прекращается. Этот срез отвечает за Manager cleanup/negative state и retained
custody; подтверждение доставки RPC/App outcome остаётся отдельным App contract.
Stop's active wait budget не стал hard deadline native cleanup/join.

### Сверка без исполнения

Manual source/caller review: four body boundaries/direct defers, all positive
completion callers, original Stop manual/error distinction, V318/319 watch joins,
deferred cleanup registration/execution, original error/rollback branches and
platform terminal partial lease release. Original manager body diff reviewed with
ignored whitespace; cleanup/terminal/watch diffs и new component просмотрены.
Gofmt5+2 exit0;9 snapshots8existing1new/task71rev749 archived/11 reviewed SHA.
Source edit script работает только с original text/bytes, не Go AST/product parser.
Tests/fixtures/generated bindings/dependencies не менялись/не запускались.

Current private Manager cleanup callers supply at most two explicit probes or
one default probe; это source caller bound, не cap произвольного future adapter
или доказанный whole-process memory/CPU/deadline/Go-task limit. Opaque callback,
нарушающий partial-release contract, и native syscall/Close failure не получают
synthetic ownership proof. Panic/Goexit/OOM/fatal process exit не взаимозаменяемы.

OPEN: compile/API/source/fixture compatibility, ordinary/error/rollback/Stop/
body/terminal/cleanup panic-Goexit interleavings и lock/cancellation checks;
internal Box/forwarding cleanup sub-stages, App pre-acquisition/RPC abnormal
scopes and full task ownership; WIN-TRANSPORT-001/shared-session publication,
actual native resources/PKI/egress/OS/GUI/installer/release/comparative/allPC/old9.
Все bootstrap/bridge/native/chain/relaunch/parent/helper gates=false,
ranBeforeJob sticky, GUI6/R1/U1/IPC9/Job8, TUN unavailable/schema2 unsupported.
Android/shared/configs/harness/dependencies/Git/server/system network/secrets/
other-role state/chats/subagents не менялись/не выполнялись.


<a id="pc-v321-box-retirement"></a>

## V2-321 — единственный исходный Box.Close и post-SDK drain

**Source/unverified, 2026-11-09.** PC-01, полный PC00…09/old9 active,
100 OPEN/56 NOT_RUN, без waiver.5 Go/4 docs. Это продолжение V320,
не runtime reproduction и не подтверждение работоспособности текущего EXE.

### Дефект и граница доказательства

Pinned sing-box v1.13.19, box.go:496: SDK закрывает свой done **до** прохода
девяти lifecycle managers, internal services и logger. Повторный Close даёт
os.ErrClosed, не результат исходной очистки. Прежний admissionBox.Close
вызывал bootstrap/gate/SDK/drain последовательно через cleanupCall: Goexit
мог пропустить последующие этапы. Аналогичный пропуск был внутри finishClose.
Engine не сохранял cleanupErr, если original box.Close не вернулся из-за Goexit.
Вывод получен чтением исходников; воспроизведение не выполнялось.

### Компоненты и порядок

- box_close.go: admissionBox перенесён из forwarding_admission.go; embedding
  original *box.Box и production constructor сохранены. Private closeOwner
  резервируется под коротким closeMu до первого callback; mutex не удерживается
  над SDK/native Close или drain. Слот содержит исходные done/result.
- Повторный Close после done читает исходный результат; pending/reentrant
  возвращает ErrEngineCleanupFailed без self-wait и повторного SDK Close.
  Полный порядок регистрируется заранее через V320 lifecycleCleanupStep:
  bootstrap.beginClose → admission.beginClose → bootstrap.close → Box.Close →
  forwarding finishClose. Panic одного этапа фиксируется; Goexit продолжает
  уже зарегистрированные defers, сохраняя ошибку в исходном владельце.
- forwarding_close.go: original closeScope со routes/initial mixed inputs/scope
  опубликован под admission.mu **до** deactivation/cancellation. Повторный
  beginClose использует тот же cohort; новая выборка не заменяет исходный owner.
  Existing route/input cancels остаются обычной отменой, не Close receipts.
- Единственный drainOwner резервируется до post-SDK snapshots и callbacks.
  finishClose использует фактический stored closeScope. Rejection intake и
  mixed inputs запечатываются **после** попытки SDK Close; новые untrackable
  callbacks сохраняют существующий worker fail-stop. Worker custody не заменена.
- Deferred drain order: original TUN diagnostic scope → routes → rejected
  inputs → mixed inputs. Все группы зарегистрированы до первой cleanup callback.
  Scope slot очищается только после обычного успешного original scope.close
  и exact-pointer check; ошибка или Goexit оставляет original scope владельцу.
  Existing group internals/timeout policies сохранены. Общий drain timeout
  создаётся на post-SDK этапе, не до bootstrap/SDK Close.
- Result/drained/done публикуются после unwind зарегистрированных этапов.
  Drain finalizer под admission.mu учитывает текущий routeFault; Box finalizer
  и repeated-result readers также учитывают новый negative routeFault. Поздняя
  ошибка не стирается старым nil. Это не отсутствие будущих SDK callbacks:
  их фактический join по-прежнему требует separate SDK/actor/Job contract.
- Engine.closeBoxLocked сохраняет original Box/runCtx и sticky cleanupErr,
  если вызов Close не вернулся нормально. Named result исключает nil при
  recovered panic до ordinary Close return. Ordinary error также sticky;
  isolated parent Job/reap path не изменялся.

### Разбор исходных веток без исполнения

| Случай | Ожидаемый исходный контроль |
|---|---|
| Ordinary successful Close | Исходные revoke/cleanup stages, затем done/result; existing engine release после nil |
| Callback возвращает error | Остальные deferred stages выполняются; original result остаётся ошибкой |
| Callback panic | Stage фиксирует только fixed label/type payload; остальные stages продолжаются |
| Callback Goexit | Original goroutine заканчивается после defers; sequence result отрицательный; Engine owner сохраняется |
| Callback/Close блокируется | Original done остаётся pending; нет rescue goroutine или synthetic success |
| Повторный/reentrant Close | Completed original result либо restrictive pending error; SDK/native не повторяются |
| Scope close error/abnormal return | Original scope slot сохраняется, drain result не становится успешным |
| Fault после старого completed result | Current negative routeFault присоединяется при повторном чтении |

Source/caller review: constructor/interface embedding, actual finishClose callers,
same stored cohort, mutex boundaries, reverse defer execution, scope retention,
post-SDK sealing, current routeFault and Engine original-owner/error paths.
Original no-index diffs трёх existing Go просмотрены; два new component прочитаны
целиком. Exit1 git diff означает различие source bytes, не test failure.
Final touched gofmt5 exit0; initial gofmt5 exit0 также наблюдался. Результат
промежуточного gofmt2 потерян при compaction и не выдан за наблюдённый.
9 snapshots7existing2new/task71rev752 archived/9 reviewed+1 cached SDK SHA.
Tests/fixtures/build/typecheck/options/parser/SDK/worker/browser/native/admin/
network не выполнялись; dependencies/cache/secret inputs не менялись.

### Остаток

Sequence drained означает unwind наших frames, **не** successful native Close,
Wait/Release, завершение внутренних SDK tasks или полное освобождение памяти.
SDK Close остаётся opaque sequential loop: panic/Goexit внутри него может
пропустить его внутренние managers; этот срез не повторяет их и не патчит cache.
WIN-TRANSPORT-001/shared HY2/mux/TLS/QUIC publication/join остаётся OPEN.
Goexit не превращён в Go return или подтверждённый RPC outcome; App
pre-acquisition/RPC abnormal cleanup ownership — следующий P0 source-срез.
Compile/API/ABI/fixtures/races/interleavings/OS/native/performance/egress/UI/
release/comparative/fullPC/old9 acceptance OPEN. Bootstrap/bridge/native/chain/
relaunch/parent/helper gates=false; ranBeforeJob sticky; GUI6/R1/U1/IPC9/Job8;
TUN unavailable/schema2 unsupported. Android/shared/configs/harness/other-role
state/chats/subagents/Git/server/system network не затрагивались.


<a id="pc-v322-app-stop-outcome"></a>

## V2-322 — исходный итог App Stop и очистка всех известных групп

**Source/unverified, 2026-11-09.** PC-01, fullPC00…09/old9 active,
100 OPEN/56 NOT_RUN, без waiver.6 Go/4 docs; V321 был progress.

### Найдено в исходниках

После захвата operationMu App Stop выполнял publish/protection/Manager/helpers
последовательно. Goexit мог пропустить более поздние группы и оставить UI в
stopping. Panic/Goexit до создания protectionReleaseCommit не имел общего
отрицательного App outcome. RetireCaptureHelpers имел такие же последовательные
группы; core capture loop удерживал lane mutex без deferred unlock над retirement.
appendLog/pushState wrappers также освобождали App.mu только обычным возвратом.
Это source findings; воспроизведение и native диагностика не выполнялись.

### Изменения

- stopCoreRegisteredLocked сохраняет original registration и local cleanup
  sequence, напрямую откладывает finishStopOutcome; returned выставляется только
  после обычного возврата всего extracted body, включая releaseCommit.finish.
  Обычные error/cancel/StopReason branches сохранены.
- stop_outcome.go при missing return/panic/Goexit присоединяет original cleanup
  error и ErrEngineCleanupFailed, отзывая **исходный** OperationSource. Error,
  cleanup_unconfirmed, CoreReady=false/ProbeRunning=false, снятие auto-retry и
  native observation применяются только при exact same registration. Protection
  intent не превращается в implicit Disconnect; successor source/state не меняется.
  Сообщения о panic содержат fixed label/type, не значение payload.
- stop_cleanup.go регистрирует все negative stages до первого callback. Порядок:
  publish stopping → determine protection/BlockDirect для automatic Stop →
  original Manager.Stop → original App helpers. Исходный orphaned engine даёт
  restrictive error. Mutex App.mu не удерживается над этими native callbacks.
  Stage panic записывает ошибку; Goexit продолжает оставшиеся registered defers.
- Body допускает positive Release только при original drained/error-free cleanup
  и прежних manual StopReason/intent/currentness gates. Release **не** является
  deferred cleanup и не выполняется при ошибке предыдущего этапа. Runtime owner
  slots очищаются только прежним обычным successful path; failure их сохраняет.
- App retireCaptureHelpers заранее снимает original restart/elevation pointers,
  затем deferred order сохраняет capture lanes → restart → elevation. Close
  error/abnormal unwind не очищает pointer. Перед и после успешного Close есть
  exact-pointer check. Transferred GUI не закрывается и не забывается.
- Core retireCaptureLanes использует уже имеющиеся V320 cleanup steps. Все три
  original lanes registered прежде первого callback: protected → optional →
  delivery. Existing TryLock не стал ожиданием; claimed lane unlock теперь defer.
  Original native handles остаются у тех же owners. Ancestor query по-прежнему
  разрешён только после обычного error-free завершения всех lane attempts.
- appendLog/pushState wrappers освобождают App.mu через defer. Stop stopping/error
  events перенесены за ручной App.mu scope и проходят wrapper; failed publication
  не оставляет этот mutex занятым. Общая схема доставки Wails events не менялась.

### Ручной разбор веток

| Ветка | Исходный контроль после изменения |
|---|---|
| Ordinary Stop success | Полная negative sequence; manual Release лишь после её обычного успеха |
| Ordinary cleanup error | Остальные группы attempted, original owners retained, no Release |
| Event/policy/Manager/helper panic | Stage error, remaining registered stages, no false cleanup success |
| Goexit в cleanup | Later defers unwind; original goroutine заканчивается; App negative outcome |
| Cleanup блокируется | Original operationMu/task остаются заняты; timeout не становится retirement |
| Stop body/release finalizer interrupted | Direct outer outcome rejects stopping/stopped success for original registration |
| Новый intent во время Stop | Existing registration gates retained; finalizer не публикует ошибку преемнику |
| Transferred GUI / busy lane | Existing custody retained / restrictive error, no replacement or new helper |

Manual source/caller review: all production serialized Stop callers, direct defer
placement, extracted-body original diff, reverse deferred order, TryLock/unlock,
same-source revocation, original pointer checks, ordinary manual/automatic Release
distinction и error publication locks. New components прочитаны целиком; четыре
existing Go diffs просмотрены. Gofmt6+2 exit0.10 snapshots8existing2new,
task71rev755 archived,12 reviewed source SHA; cache/dependencies не менялись.
Tests/fixtures/build/typecheck/options/parser/SDK/worker/browser/native/admin/
network не запускались. Source review и formatting не являются их доказательством.

### Остаток и пределы

Этот срез начинается в serialized Stop; регистрация Stop **до** operationMu,
Start/Switch/pre-acquisition/factory scopes и общий App task ownership ещё OPEN.
Прямые Locked-event callers вне изменённых Stop paths требуют отдельного review.
Goexit не превращён в Go/RPC return; доставка error event не доказывает native
состояние. Blocked callbacks не получили hard deadline или rescue goroutine.
Sequence drained — unwind наших frames, не successful native Close/Wait/Release,
не полный SDK internal/protocol join и не whole-process resource bound.
WIN-TRANSPORT-001/shared-session/SDK opaque cleanup, compile/API/ABI/fixtures/
races/native/OS/egress/UI/release/comparative/allPC/old9 acceptance OPEN.
Bootstrap/bridge/native/chain/relaunch/parent/helper gates=false; ranBeforeJob
sticky; GUI6/R1/U1/IPC9/Job8; TUN unavailable/schema2 unsupported. Android/shared/
configs/harness/other-role state/chats/subagents/Git/server/system network/secrets
не затрагивались. Следующий P0 — original App Start/Switch abnormal outcome и
pre-serializer Stop registration; без replay уже наблюдённой cleanup sequence.


<a id="pc-v323-app-lifecycle"></a>

## V2-323 — исходный App Start/Switch и отмена Stop до ожидания

**Source/unverified, 2026-11-09.** PC-01; fullPC00…09/old9 active,
100 OPEN/56 NOT_RUN, без waiver.6 Go/4 docs; V322 был progress.

### Дефект и выбранная граница

Start/Switch завершали canceled context, но не имели общего ordinary-return
witness для original App body и его defers. Panic/Goexit при render/preparation,
фабрике/engine/event мог оставить starting/reloading, уже применённый runtime
или ошибочный positive rollback outcome. registerStopLocked отправлял Wails
event **до** old operation cancel; сбой/блокирование event задерживал отмену.
Это source findings, не runtime reproduction.

### Реализация

- app_lifecycle_outcome.go: private original-frame receipt, существующий
  operationMu и один App.lifecycleOutcome slot под этим же serializer. Capture
  exact intent/source происходит до observer/pre-acquisition waits. Start/Switch
  bodies вынесены в private methods; ordinary-return bit выставляется только
  после их actual normal return, включая original deferred cancellation/startBusy.
- Direct finish defer сохраняет ordinary error/verified rollback без новой
  обёртки. Missing return/panic/Goexit отзывают только captured source; nonnil
  cleanup cause invalidates positive rollback через существующий core helper.
  Тот же registration получает Error/cleanup_unconfirmed и остановку auto-retry.
  После negative Stop этот outcome применяется ещё раз до освобождения serializer:
  нормальный Stop не превращает interrupted Start/Switch в successful result.
- Отрицательная очистка при pre-acquisition unwind также берёт operationMu.
  Original pending frame не заменяется; conflict restrictive. Release active slot
  и mutex отложен после negative cleanup/outcome/event defers. Blocked callback
  остаётся исходным pending work; rescue task/новый timeout не создаётся.
- Positive body проверяет original source после serializer wait. beginOperation
  получил private optional exact-source fence: Start/Switch передают captured
  pointer, проверяемый **под тем же App.mu**, где создаётся operation context.
  Source substitution с тем же scalar ID не принимается. Остальные private
  callers сохраняют прежний вызов; публичные Wails request/response не менялись.
- app_stop_attempt.go: при serialized Stop original sequence и Manager/Engine/
  restart/elevation pointers сохраняются в активном исходном App frame **до**
  cleanup callback. Fallback не повторяет эту sequence, пока current slots —
  те же originals либо nil после удаления. Удаление одного успешно закрытого
  slot не даёт основание повторить остальные failed callbacks. Новый substituted
  owner требует другого cleanup; отсутствие owner не выдаётся за native success.
  Retained sequence error присоединяется к отрицательному outcome.
- registerStopLocked больше не отправляет event. Captured old operation cancel
  зарегистрирован defer после exact Stop registration и срабатывает при выходе,
  включая abnormal bookkeeping unwind. Serialized Stop публикует stopping
  внутри V322 заранее зарегистрированной cleanup sequence. Снимок stopping
  доступен через GetState уже после регистрации; ранний duplicate event удалён.
- Start/Switch starting/reloading events вызывают existing pushState wrapper
  вне ручного App.mu scope. В failCoreForIntent финальный App lock теперь defer
  освобождается и при error rendering/log/state callback failure. Его final
  publication/logging остаются locked variants, без self-lock.

### Ручной разбор исходных веток

| Ветка | Исходный контроль |
|---|---|
| Ordinary Start/Switch return | Original body defers, witness, frame release; прежний outcome сохраняется |
| Ordinary rejected candidate / verified rollback | Прежние error/currentness/rollback branches, без дополнительного abort |
| Body/defer panic | Negative original source, original Stop или retained sequence, final Error |
| Body/defer Goexit | Registered negative cleanup/outcome/release defers; Goexit продолжает завершать goroutine |
| Pre-acquisition unwind | Captured source отзывается, negative cleanup сериализуется, successor не усыновляется |
| Nested Stop уже выполнялся | Original/cleared cohort использует recorded error, без replay |
| Новый intent/source | Same-registration gate не меняет successor; exact source fence не выдаёт новый operation ctx |
| Stop event failure | В регистрации event отсутствует; cancel не зависит от его доставки |

Manual review: четыре existing source diffs, оба new component целиком,
production Start/Switch/Stop callers, exact-source binding, body named error
и original defers, negative stage ordering/locks, removed-slot cohort rule,
manual/automatic Stop distinction и retained uncertainty. Gofmt6+4+5 exit0.
10 snapshots8existing2new/task71rev758 archived/12 reviewed source SHA.
Tests/fixtures/build/typecheck/options/parser/SDK/worker/native/browser/admin/
network не выполнялись; dependency/cache/real owner inputs не менялись.

### Остаток

Goexit не стал ordinary Go/RPC return; event delivery не является native proof.
Opaque callbacks, удерживающие чужие locks или native resources, не получили
hard deadline/full-task/RSS guarantee. Этот receipt покрывает Start/Switch frame,
а не весь App process: startup/Close/other action/watchdog scopes требуют review.
SDK internal cleanup, WIN-TRANSPORT-001/shared-session joins, compile/API/ABI/
fixture/race/native/OS/egress/UI/release/comparative/fullPC/old9 acceptance OPEN.
Bootstrap/bridge/native/chain/relaunch/parent/helper gates=false; ranBeforeJob
sticky; GUI6/R1/U1/IPC9/Job8; TUN unavailable/schema2 unsupported. Android/shared/
configs/harness/other-role state/chats/subagents/Git/server/system network/secrets
не затрагивались. Следующий P0 — startup/Close и оставшиеся App action/task scopes,
не выдавая sequence unwind за actual native/OS retirement.


<a id="pc-v324-startup-close"></a>

## V2-324 — исходные startup/Close/Quit и последовательность завершения

**Source/unverified, 2026-11-09.** PC-01; fullPC00…09/old9 active,
100 OPEN/56 NOT_RUN, без waiver.7 Go/4 docs; V323 был progress.

### Источники дефектов

prepareStartup мог закончиться panic/Goexit после частичного запуска producers
без общего отрицательного App outcome. Startup done закрывался в body до outer
negative finalization; storage-ready publication не требовал ordinary-return witness.
Close выполнял cancel/Stop/requestQuit последовательно: abnormal callback мог
пропустить следующий этап. Terminal producer cleanup мог пропустить последующие
cancel/join/log-drain группы; Quit worker не имел общего normal-return boundary
и перезаписывал terminalRetirementErr, теряя более раннюю ошибку.
Это source findings; воспроизведение и native/live диагностика не выполнялись.

### Компоненты

- startup_outcome.go: private original preparation frame используется prepareStartup
  и prepareNetworkServices под existing operationMu. Registration снимается до
  callbacks. Actual normal body return включает его defers; только после него
  обновляется storage readiness. Единственный original startup done закрывается
  после finalization, не до возможного negative outcome. Повторный prepareStartup
  не закрывает тот же channel. Ordinary profile/migration error остаётся обычной
  profile recovery веткой, не превращается в automatic fatal latch.
- Panic/Goexit подготовки дают sticky startupErr, storageReady=false, negative
  Stop→producer retirement→Error с original registration/source. Initial startup
  source и captured preparation source отзываются; successor не усыновляется
  для positive work. Закрывающийся App сохраняет registered terminal phase.
  New Start/beginOperation, operationCurrent/sessionCurrent и disconnected power
  recovery отказывают при startupErr. Так watchdog не создаёт producers заново
  поверх неудачной подготовки. Partial task/file/native owners не заменяются.
- app_close_task.go: original Close task публикуется в App до goroutine и хранится
  до exit. Повторный Close не создаёт новую cleanup goroutine. All original stages
  registered до callbacks: cancel tailer → background → network → stopQuit → log.
  Network tasks joined later by terminal coordinator, после попытки Manager Stop;
  pre-Stop waiting, которое могло ждать lifecycle finalizer, удалено.
  Whole close-frame normal-return witness обрабатывает panic/Goexit, сохраняет
  sequence result и вызывает existing once-reserved requestQuit. Его done значит
  unwind close frame, не successful native retirement или завершение Quit.
- OnBeforeClose освобождает App.mu через общий defer; после регистрации closing
  запуск original Close откладывается до освобождения mutex, включая abnormal
  retry-bookkeeping unwind. Existing pending restart/Release handoff branches
  и explicit terminal Quit permission сохранены.
- terminal_retirement.go: cancellation network/tailer/background/diagnostic/tray
  registered до callbacks и предшествует **всем** joins. Original returned done
  fences используются existing join methods; tailer и пять producer joins тоже
  registered до первого wait. Shared caller budget и final done re-read сохранены.
  Ошибка/Goexit cancellation не превращает отсутствующий returned fence в успех:
  aggregate sequence остаётся negative. Warning перед detached disk queue и final
  stopLogWriterContext имеют свои registered stages, так log failure не пропускает
  disk drain. Original owners remain retained on timeout/error.
- terminal_quit.go: existing Quit worker получил ordinary-return finalizer. Даже
  при producer Goexit/error он фиксирует terminal intent и attempts log→Wails Quit
  через registered stages. Earlier terminalRetirementErr сохраняется через join;
  producer errors записываются также при Goexit, до caller publication. Это
  diagnostic result, не permission на Release/native readiness или успешный exit.
  Existing independent backstop и его delay/budget unchanged. Заблокированный
  **pre-requestQuit** Stop/native call всё ещё находится вне backstop; не заявлен
  whole-App hard deadline. Accepted successor GUI custody не изменялась.

### Ручной разбор веток и границ

| Ветка | Исходный контроль |
|---|---|
| Ordinary preparation success | Body defers → readiness → original done; ordinary start-services decision |
| Ordinary profile error | Существующее profile recovery/error поведение, без fatal latch |
| Preparation panic/Goexit | Sticky startup failure → negative original Stop/producers → Error; no new Start/recovery |
| Повторный Close | Тот же original task, без duplicate cancellation/cleanup goroutine |
| Close stage error/Goexit | Remaining registered stages; recorded error; existing terminal Quit requested |
| Producer cancel/join/log abnormal | Later registered cancellation/joins/disk stage continue; no fake retirement |
| Quit worker/log/UI callback fault | Negative diagnostic retained; later registered Quit/backstop остаются separate terminal intent |
| Blocked native/opaque callback | Original owner остаётся pending; no timeout-as-success or global time bound |

Manual source/caller review: original startup/Close/Quit bodies, readiness/done
ordering, storage versus producer preparation, exact source capture, all cancel/
join/logger stages, once publication, transferred Close branches and source
guards. Four new components прочитаны целиком; три existing source diffs reviewed.
Gofmt6+1+3 exit0.11 snapshots7existing4new/task71rev761 archived/12 reviewed SHA.
Power recovery guard добавлен как зависимая поправка того же среза: original
pre-edit snapshot и scope-extension receipt сохранены отдельно; исходные десять
snapshots не изменялись. Tests/fixtures/build/typecheck/options/parser/SDK/worker/
native/browser/admin/network не выполнялись, dependencies/cache не менялись.

### Остаток

Ordinary preparation return не доказывает readiness каждого producer/native
component. Frame done/sequence drained не являются Close/Wait/Release proof.
Goexit по-прежнему прекращает original goroutine; ordinary RPC delivery не
доказана. Retained vault/file/native uncertainty не лечится повторным startup.
Watchdog/other action task lifetime и full App completion inventory требуют
следующего source review; opaque callback-held locks/native blocking сохраняют
свои OPEN gates. SDK internal/shared-session/WIN-TRANSPORT-001, compile/API/ABI/
fixture/race/OS/egress/UI/release/comparative/fullPC/old9 acceptance OPEN.
Bootstrap/bridge/native/chain/relaunch/parent/helper gates=false; ranBeforeJob
sticky; GUI6/R1/U1/IPC9/Job8; TUN unavailable/schema2 unsupported. Android/shared/
configs/harness/other-role state/chats/subagents/Git/server/system network/secrets
не затрагивались. Следующий P0 — original App watchdog и оставшиеся action scopes:
actual task/error fences без replacement на основании только cancel/timeout.


<a id="pc-v325-watchdog-owner"></a>

## V2-325 — исходная задача watchdog и честная проекция контроля

**Source/unverified, 2026-11-09.** PC-01; fullPC00…09/old9 active,
100 OPEN/56 NOT_RUN, без waiver.10 Go/4 docs; V324 был progress.

### Source finding

watchOn выставлялся до goroutine и не подтверждал ни actual entry, ни текущую
liveness, ни завершение задачи. GetFailoverStatus.Enabled читал этот flag.
Watchdog не входил в terminal producer cancellation/join. Panic/Goexit либо
неожиданный normal return могли убрать контроль без original error/done owner.
Это source findings, не runtime reproduction или measurement частоты отказа.

### Реализация

- watchdog_owner.go резервирует единственный appWatchdogTask в actual coreRuntime
  до scheduling. Own ctx/cancel/started/done/running/fault/cleanup сохраняются
  в original slot и не заменяются после deadline/cancel/done. Foreign App не
  принимает эту задачу. Task entry публикуется после создания original ticker,
  перед existing watchCoreTicks; ticker cleanup входит в body-return witness.
- ensureWatchdog использует существующий awaitNetworkTaskEntry/двухсекундный
  entry-wait budget и короткий runtime→task metadata lease. Timeout означает
  pending original task; повторная попытка ждёт тот же owner. Actual entry не
  означает first successful tick/health/native readiness. Original 30s interval,
  probe/failover body и physical measurements сохраняются. Probe child timeout
  теперь отменяется через defer при abnormal callback.
- Start проверяет entry до engine/probe setup. Startup producer preparation при
  ordinary unavailable entry возвращает false без запуска auto-connect; original
  task остаётся retained и может быть rechecked после actual entry. Фактический
  task fault/stopping ставит one-way App.watchdogBlocked до App/operationMu waits;
  после него новый positive Start/operation/session не допускается в этом App.
- watchdog_fault.go выполняет fault cleanup **в исходной goroutine**. Captured
  exact registration/source и original operation cancel не усыновляют successor
  после ожидания existing operationMu. Registered cancel→negative stopFailure→
  same-registration Error stages сохраняют local cause и cleanup uncertainty,
  включая Goexit. Own done закрывается после loop и fault finalizers; fault path
  не вызывает terminal producer join и не ждёт собственную задачу.
- Explicit Disconnect (protectionDesired=false), profile transition и Close
  сохраняют свою фазу: producer fault не отзывает manual release registration
  и не делает implicit Release. Local task error входит в existing retirement
  classifier, не в channel-health/quarantine/remote censorship evidence.
- Negative atomic fence связан с existing core OperationSources.Blocked для
  original operation, protected observation и policy verification. Positive
  observation/retained rollback publication также отказывает после revocation;
  restrictive negative observations остаются allowed. GetState projection сразу
  исключает stale Ready/running, не ожидая fault cleanup за lifecycle mutex.
- terminal_retirement.go отменяет original watchdog вместе с producers **до**
  любого join и затем ждёт точный original done в том же общем budget. Actual
  fault читается после done. Timeout сохраняет original slot/fault; он не считает
  task/native ресурсы завершёнными. Startup cleanup под operationMu может получить
  pending fault finalizer; budget не разрешает заменить его или выдавать PASS.
- GetFailoverStatus.Enabled отражает entered/current/not-stopping task. При
  unavailable task LastError описывает local control failure, NextCheckIn=0;
  исторический transport policy state не переписывается диагнозом канала.
  DiagnosticContext Ready/Lifecycle согласованы с watchdog/power negative fence
  до вычисления immutable Revision; public schema не менялась.
- Legacy watchOn сохранён как **reservation mirror only**: существующий fixture
  ссылается на поле. Ни Enabled, ни task ownership, ни forwarding/publication
  permission его не читают. Fixture не изменён/не запускался; это сохранение
  source surface, не утверждение, что его старые runtime expectations проходят.

### Ручной разбор

| Ветка | Исходный контроль |
|---|---|
| Concurrent ensure calls | Один published original slot, общий entry/done, no duplicate ticker |
| Entry wait deadline | Same original task pending; no replacement/health assumption |
| Ordinary live task | Original ticker→existing loop; Enabled is task state, not probe freshness |
| Normal terminal cancellation | Negative fence→original cancel→actual done join; manual Stop не отменяет watchdog |
| Panic/Goexit/unexpected return | Fast negative fence, local fault, original scope cancel/Stop/Error; no rescue worker |
| New Stop/Close intent during fault | Exact-source gates preserve successor/manual phase, no implicit Release |
| Fault finalizer blocked on operationMu | Original done stays pending; terminal deadline retains owner |
| Late positive result after fault | Existing source/positive-publication/diagnostic gates reject stale readiness |

Manual source/caller review: original ensure/loop/consumer/terminal callsites,
entry before positive work, runtime/App/task lock ordering, callbacks outside
metadata locks, no self-join, same-source negative Stop/manual distinction,
Blocked binding/positive publication/Enabled/DiagnosticContext consistency.
Seven existing-scope diffs plus diagnostic extension and two new components
reviewed. Gofmt9+3+2 exit0.14 snapshots12existing2new/task71rev764 archived,
13 reviewed SHA, including unchanged existing fixture source. Diagnostic extension
has its own original pre-edit snapshot/scope receipt; first13 snapshots preserved.
Tests/fixtures/build/typecheck/options/parser/SDK/worker/native/browser/admin/
network не выполнялись; dependencies/cache/real owner inputs не менялись.

### Остаток

Task entry не доказывает healthy tick, task done не доказывает successful native
Close/Wait/Release или все дочерние OS/SDK tasks. Cancel/deadline не дают hard
clock/OS/RSS bound. Blocking opaque callbacks сохраняют original pending owner;
Goexit не становится ordinary RPC completion. Native/egress acceptance и current
EXE соответствие этим исходникам не подтверждены.
Other RunProbe/native-action/task scopes и full App completion inventory требуют
следующего source review. WIN-TRANSPORT-001/shared-session/SDK opaque cleanup,
compile/API/ABI/fixture/race/OS/UI/release/comparative/fullPC/old9 acceptance OPEN.
Bootstrap/bridge/native/chain/relaunch/parent/helper gates=false; ranBeforeJob
sticky; GUI6/R1/U1/IPC9/Job8; TUN unavailable/schema2 unsupported. Android/shared/
configs/harness/other-role state/chats/subagents/Git/server/system network/secrets
не затрагивались. Следующий P0 — original RunProbe/optional native-action scopes
и их exact completion/cancellation/error ownership без timer-as-retirement.


<a id="pc-v326-probe-actions"></a>

## V2-326 — исходный RunProbe и отдельный outcome необязательной native-проверки

**Source/unverified, 2026-11-09.** PC-01; fullPC00…09/old9 active,
100 OPEN/56 NOT_RUN, без waiver.4 Go/4 docs; V325 был progress.

### Найдено в исходниках

RunProbe выставлял ProbeRunning и вызывал pushStateLocked **до** регистрации
clear defer. UI callback panic/Goexit мог оставить busy flag и App.mu занятыми.
Не было общего body-return outcome для этой mandatory action. Native optional
action публиковала draft и committed flag без outer outcome всего RPC body;
cached native scope проверял scalar intent, но не exact original source или
watchdog/startup failure. Value lease имел только explicit error releases.
Это source findings; ни одна проверка VPN/credentials/native action не запускалась.

### Реализация

- RunProbe captures exact registration под App.mu до ожидания operationMu и
  использует существующий appLifecycleOutcome. Body/error/publication branches
  вынесены в private method; beginOperation получает original source. Missing
  normal return/panic/Goexit проходят existing negative Stop/same-registration
  Error boundary; ordinary failed/stale observation сохраняет прежнее поведение.
- Clear ProbeRunning зарегистрирован до mutation/event. Local published witness
  не позволяет очистить флаг, который эта frame не выставляла. Original config/
  Manager и same registration проверяются перед clear; callback выполняется под
  deferred App unlock. Initial event использует pushState вне ручного lock scope.
  Body defers завершаются до outer ordinary-return witness.
- appProtectedProbe.Run имеет outer normal-return boundary вокруг существующих
  body и report finalizer. Original attempted subject/observation/work-started
  сохраняются до callbacks. Abnormal fallback report всегда Stale/AllPassed=false/
  CleanupUnconfirmed; он не запрашивает newer global subject, clock acceptance
  или новый positive probe. Private restrictive path marker выдаётся только
  после actual original scoped work; original actor/source checks сохранены.
  Existing caught callback panic также помечается local cleanup uncertainty,
  не remote channel evidence. Panic payload values в report не копируются.
- native_probe_outcome.go — отдельный original optional action receipt. Его
  registration captures до TryLock и binds exact source при beginOperation.
  Ordinary body return включает draft/lease/context defers. Missing return даёт
  fixed ErrNativeScopeUnavailable и удаляет **только** свой original draft,
  включая value, committed до последующего abnormal defer. Successor observation
  не очищается. Эта ветка не останавливает engine, не меняет mandatory health,
  channel scores, protection intent или retry/failover.
- Native draft publication находится в scoped function с deferred release.
  nativeProbeScope.lockCurrent после acquired runtime/Manager/App/epoch lease
  имеет deferred failure release до handoff; error/panic/Goexit в последующих
  value/clock checks не удерживают этот lease. Successful handoff сохраняет
  прежний returned release callback. Native/interface/WFP/helper work остаётся
  за пределами value gate.
- Native scope хранит pointer source из original private operation context;
  scalar-ID match больше не заменяет source equality/currentness. Startup error
  или watchdog revocation запрещают scope и cached read. Поэтому polling с
  Background не выдаёт старое native observation за current после local fault.
- GetNativeProbeReport получил separate safe read outcome: unexpected read fault
  возвращает fixed unavailable view, без payload/OS/IPC details. Это read-only
  cache path, не новый native request, не refresh timestamp/expiry и не proof
  VPN success. Target, helper SHA, timeout12s и public schema не менялись.

### Ручной разбор веток

| Ветка | Исходный контроль |
|---|---|
| Ordinary RunProbe | Existing scoped measurement/result branches, exact original source |
| RunProbe event/body/defer interrupted | Original busy clear, operation cancel, outer negative lifecycle outcome |
| Original session/intent replaced | Old frame не очищает successor flag/result и не принимает его source |
| Protected report finalizer fault | Only original attempted negative/stale custody, never fabricated fresh health |
| Native optional action error/panic | Sanitized error, original draft removed; mandatory VPN state не меняется |
| Native body committed then abnormal defer | Outer original witness rejects cached value of that same action |
| Native lease check fails/aborts | Acquired value locks released; no native admission or replacement owner |
| Cached native read after dependency/source loss | Stale/unavailable, no refresh/native execution |

Manual review: original source/callers/diffs, exact registration bindings,
existing lifecycle normal/error paths, busy/defer/event lock order, original
attempted-subject fallback/private restrictive marker, separate native optional
semantics, draft pointer checks, lease failure release and cached report gates.
Three existing Go diffs/new component fully read. First gofmt4 exited0 via
original running session25062/poll; final gofmt4 exited0 directly. Intermediate
gofmt1 belonged to a group ending in diff exit1; его отдельный exit не заявлен.
8 snapshots7existing1new/task71rev767 archived/12 reviewed source SHA.
Tests/fixtures/build/typecheck/options/parser/SDK/worker/native/browser/admin/
network не выполнялись. Production diagnostics code редактировался как source;
диагностика, реальные vault/credentials/runtime profiles не выполнялись/не читались.

### Остаток

Goexit не стал ordinary RPC return; body/lease unwind не является successful
native Close/Wait/Release, actual helper/OS retirement или whole-App task proof.
Metadata/report fallback не даёт health/trust/native authority и не чинит
подтверждённый родительский OS resource failure. Opaque blocking остаётся OPEN.
Original manualDiagnosticTask socket Close/watcher/budget/error completion
требует следующего среза; найденные source gaps ещё не исправлены здесь.
WIN-TRANSPORT-001/shared-session/SDK internal cleanup, compile/API/ABI/fixture/
race/native/OS/egress/UI/release/comparative/fullPC/old9 acceptance OPEN.
Bootstrap/bridge/native/chain/relaunch/parent/helper gates=false; ranBeforeJob
sticky; GUI6/R1/U1/IPC9/Job8; TUN unavailable/schema2 unsupported. Android/shared/
configs/harness/other-role state/chats/subagents/Git/server/system network/secrets
не затрагивались. Следующий P0 — manual diagnostic original Close/watchDone/
budget/done distinction, no cancel/deadline-as-retirement и no result on Goexit.


<a id="pc-v327-manual-retirement"></a>

## V2-327 — исходные Dial/Close и завершение ручной диагностики

**Source/unverified, 2026-11-09.** PC-01; fullPC00…09/old9 active,
100 OPEN/56 NOT_RUN, без waiver.8 Go/4 docs; V326 был progress.

### Найдено в исходниках

У manualDiagnosticTask был один socket pointer; HTTP token/volume Dial вообще
не попадали в этот учёт. Watcher.done не отличал ordinary return от Goexit.
Native Close мог вызываться повторно из SDK/body/watcher. finishManualDiagnostic
освобождал budget и done до итогового log callback, а cached scope изначально
не был rejected до полного окончания RPC. Это source findings; production
диагностика и реальные credentials/network не выполнялись.

### Реализация

- manual_diagnostic_connections.go хранит исходный ограниченный registry Dial.
  Slot создаётся до native/library callback; каждая запись имеет собственные
  cancellable context, done, ordinary-return witness и original connection.
  Cancel watcher seals intake и отменяет все captured Dial **до** первого Close.
  Поздний raw socket сохраняется даже после watcher return, затем отвергается и
  закрывается исходным owner. Finisher ждёт именно его Dial.done/Close.done.
  Нет нового cleanup goroutine, replacement owner или retry-as-retirement.
- Потолок32 ограничивает total registered callbacks задачи: max8 sizes ×2 routes
  и общее дополнительное headroom. Это не две попытки для каждого запроса,
  не число DNS/HappyEyeballs внутренних задач и не граница wire/RSS. Локальный
  atomic dialLimitReached отклоняет observation как unavailable, не выдавая
  истощение admission за remote fault. При известном cleanup budget освобождается.
- Connection wrapper выдаётся каждому протоколу. Первый Close резервирует
  исходную попытку до callback; остальные completed читают её saved result,
  pending/reentrant возвращают unavailable без ожидания себя и без native replay.
  Сам pending Close не создаёт sticky cleanup error. Actual native error, panic
  или missing ordinary return помечает cleanup uncertainty; finisher проверяет
  original closeDone/closeReturned. Native I/O выполняется вне metadata mutexes.
- SSH/UDP используют этот же wrapper. SSH client.Close и client.Wait зарегистрированы
  отдельными defer stages, поэтому abnormal Close не пропускает registered Wait
  при stack unwind. Ordinary SSH shutdown errors не объявляются OS-retirement
  proof; missing stage return сохраняет cleanupFailed. Opaque blocked frame
  может задержать последующие stages: cancellation или timeout не снимают slot.
- HTTP token и обе volume transports получили owned DialContext. Generic
  owner=nil volume path сохраняет прежний lexical release. Default direct
  DialContext fallback соответствует zero net.Dialer; ForceAttemptHTTP2=true
  сохраняет его прежнюю HTTP/2 eligibility после custom dial hook. Targets,
  Proxy semantics, body/step/time/header/redirect/compression limits и TUN skip
  direct-comparison не менялись. Ordinary dial errors сохраняют shape для
  существующего HTTP observation classifier; panic payload не публикуется.
- manual_diagnostic_outcome.go — общий original synchronous RPC finalizer для
  TestSecret/RunTest. Body вынесен в private method; ordinary-return bit ставится
  после его defers. Cache task.resultRejected=true до final completion. Resource
  retirement, fixed safe log и complete зарегистрированы до callbacks. Missing
  body/watch/log return и uncertain Close/Dial не выдают current report и не
  снимают original pending task. Лог не содержит payload/credentials/remote error.
- finishManualDiagnostic теперь проверяет известные ресурсы и currentness,
  отдельно от освобождения admission/detach. Body/watch normal witnesses и
  все original Dial/Close frames нужны для retirementConfirmed. Stale source/
  path/parent/deadline/context отвергает result, но не отменяет уже известное
  завершение ресурсов. Common completion требует own App pointer, вызывает
  original budget release один раз после resource/log stages и лишь затем
  повторно проверяет publication context и снимает pointer/закрывает done.
  Interrupted release не повторяется. Final context callback failure после
  успешного release не возвращает budget назад и не публикует completion;
  original App task сохраняется pending. Goexit не стал ordinary RPC return.

### Ручной разбор веток

| Ветка | Исходный контроль |
|---|---|
| Ordinary RPC/body/watch/log | Known resources joined, original release, final context gate, detach/done |
| Cancel до Dial intake | Sealed registry/currentness reject; no new callback |
| Cancel во время callback, late socket | Original Dial slot retained; late socket closes before same done |
| SDK/watcher/body concurrent Close | One native attempt; pending wrapper does not wait/retry/erase outcome |
| Native Close returns error/panic/Goexit | Cleanup uncertainty; no accepted observation or replacement task |
| Body/watch/log frame does not return | Safe empty report/error; original slot/done retained |
| Local Dial ceiling | Unavailable result, no remote verdict; known cleanup can free admission |
| Resources known, original subject stale | Reject old result, original budget can release, no successor retag |
| Original release interrupted | releaseStarted prevents replay; no detach/done |
| Opaque callback never returns | Original task/admission stays pending; no timeout-as-retirement |

Manual review: callers, six original Go diffs/two new components, registry seal/
late publish, metadata-lock order, exact task/source/path/context, native once-close,
SSH registered stages, optional HTTP protocol preservation, cached-result rejection
and separation of resource retirement from publication. gofmt8+8+2+2 exited0.
12 snapshots10existing2new/task71rev770 archived;12 reviewed source SHA and2
installed Go-source SHA. transport.go source confirms nil-DialContext fallback
and custom-dial HTTP2 condition; VERSION says go1.26.7. This is source provenance,
not the compiler/runtime identity. No tests/fixtures/build/typecheck/parser/options/
SDK/native/browser/admin/network/product checks executed; no real vault data read.

### Остаток

Known socket and callback frames are **not** all DNS/HappyEyeballs/HTTP2/TLS/SSH
library tasks, OS resource retirement, actual RPC delivery or whole-process proof.
No hard wall-time/RSS/wire bound is claimed. Unknown original work remains
pending rather than fabricated completion. Pre-admission factory/budget unwind,
adaptive optional admission and remaining whole-App producers require source
inventory next. Compile/API/ABI/fixture/race/SDK/native/OS/egress/UI/release/
comparative/fullPC/old9 acceptance remain OPEN. All bootstrap/bridge/native/
chain/relaunch/parent/helper gates=false; ranBeforeJob sticky; GUI6/R1/U1/IPC9/
Job8; TUN unavailable/schema2 unsupported. Android/shared/configs/harness/other-role
state/chats/subagents/Git/server/system network/secrets untouched.


<a id="pc-v328-diagnostic-admission"></a>

## V2-328 — получение бюджета до запуска ручной диагностики

**Source/unverified, 2026-11-09.** PC-01; fullPC00…09/old9 active,
100 OPEN/56 NOT_RUN, без waiver.2 Go/4 docs; V327 был progress.

### Исправленные source gaps

Shared AdmitDiagnostic увеличивал lane.active **до** context.WithTimeout.
Unreturned parent/context factory могла оставить reservation без выданного
release callback. Manual factory получала budget перед presentation context,
использовала только explicit error cleanup и не имела normal-return boundary
всей preparation. Production diagnostics и parent callbacks не запускались.

- Context/release в core создаются до pool reservation. Пока factory не вернулась,
  она не удерживает active slot. Pool guard registered сразу после Lock; unused
  context cancel выполняется после unlock. После ожидания mutex проверяется
  ctx.Err. Rolling workload append предшествует active increment; после commit
  нет parent/context callbacks, handoff передаёт тот же release вызывающему.
- Reserved protected lane, optional lane, active ceiling2, requests/bytes/minute
  и duration ceilings сохранены. Existing original sync.Once release отменяет
  context до decrement; interrupted cancellation не выдаёт free slot и не
  повторяется как доказательство завершения. Rolling costs не возвращаются.
- Manual presentation создаётся до timeout/admission. Unused cancel и unused
  release зарегистрированы отдельными defers; abnormal cancel при unwind не
  пропускает registered budget stage. Ошибки подготовки возвращают fixed safe
  unavailable без payload. App.mu unlock входит в private preparation body;
  outer witness ставится только после возврата body и его defers.
- Перед task publication повторно проверяются bounded ctx, closing, exact
  original registration и path pointer/currentness. После publish handoff
  передаёт cancel/release только original manualDiagnosticTask и его существующему
  watcher/finisher. Ни background retry, ни новый task для ошибки не создаётся.
  Функции public Wails/Diagnostic API сохранили прежние results/contracts.

### Ручной разбор

| Ветка | Source ownership |
|---|---|
| Invalid request | No context/reservation; прежний fixed error |
| Context factory abort | No active reservation committed; internals of unreturned factory remain unknown |
| Budget busy/cost rejected | Pool unlock, unused context canceled; no accepted workload refund |
| Context expires during preparation/lock wait | Refusal before reservation/task handoff at next check |
| Manual presentation unavailable | Refusal before timeout/admission |
| Admission returned, original source/path revoked | Independent unused cancel/release stages, no task published |
| Successful manual handoff | Same task owns subsequent cancellation/Dial/Close/budget/done |
| Cleanup callback abnormal | Safe error; no retry/refund or fabricated native retirement |

Manual review covered two original diffs, preparation/caller shapes and generic
budget consumers in protected probe/worker/helper/background/volume. gofmt2+1
exited0.6 snapshots6existing/task71rev773 archived;12 reviewed source SHA and2
installed source SHA. context.go WithTimeout/WithDeadlineCause produce local
cancelCtx/timerCtx; parent callbacks belong to construction, local Err to returned
context. VERSION go1.26.7 is source provenance only, not runtime/compiler identity.
Tests/fixtures/build/typecheck/parser/options/SDK/native/browser/admin/network/
product checks not executed; actual vault/profiles/secrets untouched.

### Границы и следующий P0

Pre-admission can create temporary context-library work before a later rejection.
No total API-concurrency/RSS/timer/goroutine/wall-time cap or performance gain is
claimed. Context construction that never returns has no active slot but its
library internals remain unknown. Post-check cancellation races remain subject
to original caller/task negative fences. Existing generic callers still own
their returned reservation; this patch is not a whole-App/SDK retirement proof.

Next concrete source gap: windows/adaptive.go RunAdaptiveCheck still uses
Background, launches parallel HTTP classifiers outside manualDiagnosticTask,
and WaitGroup.Done alone does not witness normal classifier return. Reuse
existing manual owner/Dial retirement and original applied subject; preserve
history versus current observation and optional versus mandatory health.
Compile/API/ABI/fixture/race/SDK/native/OS/egress/UI/release/comparative/fullPC/
old9 acceptance OPEN. All gates=false; ranBeforeJob sticky; GUI6/R1/U1/IPC9/Job8;
TUN unavailable/schema2 unsupported. Android/shared/configs/harness/other-role
state/chats/subagents/Git/server/system network/secrets untouched.


<a id="pc-v329-adaptive-owner"></a>

## V2-329 — параллельная проверка сайтов под исходным владельцем

**Source/unverified, 2026-11-09.** PC-01/03/04; fullPC00…09/old9 active,
100 OPEN/56 NOT_RUN, без waiver.4 Go/1 Vue/4 docs; V328 был progress.

### Исправленные source gaps

RunAdaptiveCheck использовал Background и отдельную lexical admission, а HTTP
соединения не учитывались manualDiagnosticTask. WaitGroup.Done не отличал
ordinary classifier return от Goexit. Mutable targets перечитывались после
вычисления cost. Memory/disk cache писался до network retirement, fixed .tmp
не имел original-owner receipts, read был без byte/record ceiling. Vue игнорировал
reply команды и перечитывал историю даже после unavailable. Ошибочные comments
приписывали challenge однозначную причину и WARP фиксированную server route.

### Реализация и reuse

- adaptive.go остаётся public facade/classifier. adaptive_task.go отвечает за
  исходную batch/worker lifetime; adaptive_history.go — bounded historical I/O.
  Перед admission копируется и проверяется max8 уникальных canonical DNS hosts;
  использован existing netprobe.CanonicalDNSHost. Logical endpoint workload
  reserve остаётся len(targets) ×16KiB/30s; headers/TLS/OS bytes не входят в него.
- Использован existing beginManualDiagnostic/manualDiagnosticOutcome. Exact
  original registration/path должны совпадать с captured appProbeSubject;
  Manager/engine/applied session/owner generation checks переиспользуют existing
  subject.current. No Background, independent retry, direct fallback, forwarding
  grant или изменение mandatory health/protection/failover policy.
- Каждый original classifier worker резервируется до go и имеет done, attempted
  и ordinary-return witness. Parent join зарегистрирован до первого launch и
  выполняется также при body unwind. Worker panic/Goexit очищает partial result
  и сохраняет cleanup uncertainty; done само по себе не normal return. Если хотя
  бы один classifier не начат, batch unavailable; это не отрицательный сайт-факт.
  Body normal witness включает classifier/transport defers и registered joins.
- classifySite принимает original manual task; HTTP DialContext использует
  existing task.ownedDialer/one original native Close. Mixed address берётся из
  render constants. Explicit SOCKS5h/h1-only, response header16KiB/body prefix16KiB,
  client12s/no redirect/no compression сохранены; added per-host connection1.
  Raw dial error strings/panic payload не передаются UI. HTTP code/challenge
  остаются формой ответа, не причиной DPI/egress failure; unused WARP map удалён.
- Shared outcome получил optional beforeRelease stage, nil для прежних Secret/
  RunTest callers. Он выполняется вне App.mu после body/watch/Dial/Close/log,
  пока original task/admission ещё retained. Missing stage return проходит
  original complete outcome; explicit filetxn retirement errors ставят cleanupFailed.
  Original budget release и App detach/done не происходят при unknown cleanup.
  Обычная ошибка сохранения с известным retirement не стирает HTTP-выборку;
  UI получает «сохранение не подтверждено», без заявления об отсутствии файла.
- History publisher проверяет original subject до/после storage. После own cancel
  используется WithoutCancel **только для value validation**, с отдельными
  original parent.Err/deadline/source/path checks; ни один I/O не использует эту
  view. Common completion затем ещё раз проверяет original presentation/source/
  parent/deadline перед reply. History может быть записана до последующего
  final-context/release rejection; она явно не доказывает completion/currentness.
- Existing filetxn.BoundedReadSlot/BoundedWriteSlot retained для одного history
  domain.32KiB byte ceiling/max8 records before next decode, canonical unique
  domains/bounded strings/RFC3339/code/challenge consistency, unknown-field and
  trailing-document refusal. Same adaptive-sites.json JSON-array shape/path.
  Poisoned original reader нельзя обходить записью, writer — чтением; fixed .tmp
  и unbounded ReadFile удалены. Existing memory copies доступны как история без
  повторного I/O. Legacy permissions/format compatibility ещё не проверялись.
- Vue NetworkView использует empty/unavailable result.note как action feedback,
  не выдаёт старую историю за новую выборку; success note отдельный status.
  Existing component, API mapping, polling.invalidate, owner/unmount guard,
  busy button и role=status переиспользованы. New bindings/contracts/components
  не создавались. modern-web-guidance local search/list не нашёл guide именно
  для async command feedback; existing project status pattern сохранён.
  frontend-postgres flow rules применены без SQL/DB изменений.

### Ручной разбор и границы

Manual source/caller review: two existing Go diffs/two new components/Vue diff;
registered children before launch, canceled/late owned Dial and once-Close reuse,
parent join/body witnesses, original subject checks/validation-only view,
pre-release callback lock order, storage uncertainty versus ordinary failure,
historical labels and empty/error/received Vue branches. gofmt4+1 exited0.
9 snapshots7existing2new/task71rev776 archived;17 initial plus3 additional reviewed
source SHA (20 total),2 installed context/VERSION SHA. VERSION go1.26.7 is source
identity only, not runtime/compiler. Tests/fixtures/build/typecheck/parser/options/
SDK/native/browser/admin/network/product checks not executed; actual history,
vault, runtime profiles and secrets not read. No new tests/fixtures were changed.

Known callback/socket frames are not all HTTP/TLS/DNS/HappyEyeballs/filetxn/OS
tasks or actual Wails delivery. Opaque blocking stays pending; no hard wall-time/
RSS/wire cap or measured speed/reliability/visual improvement is claimed. History
is neither authenticated measurement custody nor a command-completion receipt.
Decoder does not claim rejection of repeated known JSON object keys. Filetxn
reuse is not directory durability/interprocess/legacy-ACL/native acceptance.

Next inspected P0: bgProbeChannel still directly Dials/Close, ignores Close error
before markOK, and lexically releases budget; generic parent task completion is
not an original Dial/Close normal-return receipt. Preserve separate background
ranking evidence and protected probe authority while fixing that owner next.
Compile/API/ABI/fixture/race/SDK/native/OS/egress/UI/release/comparative/fullPC/
old9 acceptance OPEN. All gates=false; ranBeforeJob sticky; GUI6/R1/U1/IPC9/Job8;
TUN unavailable/schema2 unsupported. Android/shared/configs/harness/other-role
state/chats/subagents/Git/server/system network/secrets untouched.


<a id="pc-v330-background-owner"></a>

## V2-330 — фоновая TCP-проверка: исходные ресурсы и время наблюдения

**Source/unverified, 2026-11-09.** PC-01; fullPC00…09/old9 active,
100 OPEN/56 NOT_RUN, без waiver.5 Go/4 docs; V329 был progress.

### Найдено и изменено

bgProbeChannel игнорировал Close error перед markOK, lexical release не отличал
body/Close normal return от Goexit, а producer.done не сообщал terminal join о
retained resource fault. Round мог загрузить targets и затем заново взять иной
App/path context. Metadata использовала время публикации после cleanup. В общем
channelHealth более старый background result мог переписать newer LastOutcome.

- background_probe_owner.go выделяет existing backgroundProbeTask lifetime,
  original app/context/stop/done, stopping/normal-return/fixed local fault и
  единственный retained attempt. Constructor context создаётся до bgReg mutex,
  unused cancel зарегистрирован до handoff; task публикуется до go. No replacement
  после остановки/сбоя. done закрывается после loop/cancel finalizers; result-aware
  join проверяет exact App/task/done, normal return, fault и retained attempt.
  Foreign App не получает cancel и его fence не принимается за собственный.
- background_probe_attempt.go — synchronous original TCP frame, reserved **до**
  admission и Dial. Raw conn сохраняется сразу после callback return, включая
  поздний результат. Body, Dial, native Close, observer и release имеют отдельные
  witnesses. Close attempt запускается один раз; error/panic/Goexit не стирается
  retry. Registered cancel → native Close → observer join → complete не позволяет
  abnormal stage пропустить следующие cleanup stages при unwind. Blocking stage
  остаётся pending; resource slots не объявляются свободными по deadline.
- One original observer на serial admitted attempt следит за captured passive
  intent/control Done, owner.stop и own deadline. Он отменяет только original
  child context; не освобождает admission, не закрывает socket и не ищет successor.
  Parent ждёт original watchDone и normal return. Это один planned observer,
  не per-fault rescue worker. Нет добавленного callback в native power fast path.
- Round capture выполняется до descriptor load: original App registration,
  activeID, path pointer, positive observed epoch и original network control source.
  Due filtering использует captured epoch. Изменение pointer/source state/active
  target/path/control/epoch или startup/trust/profile/terminal denial отклоняет
  исходный round. Удалены unused getters с App-context fallback; private прямой
  probe без published producer не получает ephemeral network owner. Enabled/
  live-verified/inactive target filter, serial schedule, QUIC skip и TUN skip сохранены.
- Background remains independent weak ranking data after stopped/blocked intent.
  Fresh round может захватить **latest already-revoked** intent с тем же pointer/
  scalar и captured false state. Source не становится current, positive authority
  не выдаётся. Уже закрытый Done не умеет сигналить второй раз: такой round имеет
  identity checks до/после work и original parent/control/deadline cancellation;
  instant notification о следующей замене intent не заявлена. Round, захвативший
  current source до его revocation, отменяется original Done и не принимается позже.
- Publication происходит только после known body/Dial/Close/observer outcome.
  Local cleanup failure не классифицируется как remote channel fault. Отказ
  admission не даёт health/cooldown; QUIC skip отмечает только scheduling cooldown.
  Known retired stale work может release без публикации. Timely Dial bit записан
  до own finalizer cancel; positive TCP sample дополнительно ограничен original
  deadline. Existing legacy refused/timeout classes и anti-flicker policy сохранены;
  comments больше не приписывают active refusal однозначному actor/причине.
- Snapshot expansion сохранил before chanhealth.go до пятой source-правки.
  observedAt фиксирует ordinary Dial callback return; markOK/confirmed failure/
  background health получают этот original time. Scheduler cooldown использует
  completion time отдельно. channelHealth под existing epoch/health gate отвергает
  zero/older/equal background timestamp относительно LastOKAt/LastFailAt до mutation.
  Protected probe branches не менялись. API-return time не является wire-event
  timestamp; weak registry и health не образуют новую atomic cross-domain transaction.
- Existing terminalProducerSet зарегистрировал dedicated background result-aware
  join в том же shared cleanup budget. Все cancel stages по-прежнему раньше всех
  joins. Manual Disconnect/automatic failure/protection intent/mandatory health/
  native forwarding admission не менялись. Optional TCP uses OS route, external
  VPN/physical independence не аттестуются; никакого protected-path PASS он не даёт.

### Ручной разбор

Reviewed three original diffs/two new components/callers: task reserve-before-go,
one attempt before admission, cancel/watch normal return, late Dial/one Close,
known cleanup versus stale publication, release-before-detach and failure retention,
exact result-aware terminal join, fresh revoked-intent round semantics and original
Dial timestamp/weak ordering guard. gofmt batches4+2+2+2+2+1 exited0. Snapshot
expansion8→9 retained original8 and original15 review list; chanhealth.go moved
to changed-source snapshot, remaining14 plus4 newly reviewed sources =18 SHA.
9 snapshots7existing2new/task71rev779 archived; no cached SDK inputs this cut.
Progress checkpoint recorded4Go plus planned dependent fifth source before that
health edit; final checkpoint records completed5Go source scope. All full-goal
criteria/checks remain OPEN/NOT_RUN. No tests/fixtures/build/typecheck/parser/options/
SDK/native/browser/admin/network/product checks; real logs/vault/profiles/secrets
not read. Existing test-only reset/helper bodies and fixtures were not edited.

### Остаток

Normal API/callback/Close/watcher frames are not all DNS/dialer/OS tasks, actual
kernel retirement, RPC delivery or a hard wall-time/RSS/wire bound. Native blocking
stays under original owner. A release callback's unknown side effects are not
replayed/refunded. Pointwise source checks/epoch gates do not prove a single
atomic App/registry/health/log transaction; older historical weak registry data
and cross-consumer policy/freshness still need their own acceptance. No measured
performance/reliability gain or native cleanup success is claimed. Direct private
fixtures that previously omitted a published producer/real epoch need review once
testing is authorized; compatibility or PASS is not claimed.

Next: audit remaining generic tray/poller/DNS terminal joins against actual
original producer normal-return/native/result receipts before further edits.
Compile/API/ABI/fixture/race/SDK/native/OS/egress/UI/release/comparative/fullPC/old9
OPEN. All gates=false; ranBeforeJob sticky; GUI6/R1/U1/IPC9/Job8; TUN unavailable/
schema2 unsupported. Android/shared/configs/harness/other-role state/chats/subagents/
Git/server/system network/secrets untouched.


<a id="pc-v331-producer-retirement"></a>

## V2-331 — результаты завершения tray, poller и DNS

**Source/unverified, 2026-11-09.** PC-01; fullPC00…09/old9 active,
100 OPEN/56 NOT_RUN, без waiver.9 Go/4 docs; V330 был progress.

### Найдено и изменено

Generic terminal joins принимали закрытый канал без original producer outcome.
Сетевые finalizers выполняли очистку одним телом: abnormal callback мог пропустить
последующие stages, а explicit rearm проверял done без cleanup result. Tray workers
использовали WaitGroup без normal-return результата; закрытый command channel мог
снова выбрать действие. Native power acquisition публиковал original entry после
Pin, а ошибочная очистка могла повторить unregister.

- tray_lifecycle_windows.go выделяет существующую process-global сессию без
  смены публичного API. Original owner резервируется до go и не заменяется.
  Run frame, library Run, ready, exit, command/relay workers и retire имеют
  отдельные признаки normal return. Panic/Goexit остаются отрицательным исходом.
  Работник записан до go; Add и ready sealing используют общий mutex. После
  readyDone исходный WaitGroup и каждый original worker.done/returned проверяются
  до Quit. Закрытые menu/relay channels вызывают fail, не новое действие.
  Queued/missing ready и не вернувшийся native Run остаются pending. Quit один
  раз; API result не объявляется результатом PostMessage/DestroyWindow.
- network_producer_retirement.go содержит отдельный body/cleanup receipt.
  finishPoller/finishDNS непосредственно deferred у original run; recover и
  completed различают ordinary return и abnormal unwind. Cleanup steps
  зарегистрированы до callbacks и продолжают unwind после panic/Goexit:
  negative running/source fence → linked DNS revocation → original power
  retirement → cancel → failed original scope. Blocking callback остаётся
  pending; ошибки не теряются перед original done. Функциональный fault,
  включая ожидаемую потерю continuity после сна, отделён от cleanup uncertainty.
- Terminal dedicated tray/poller/DNS joins проверяют exact App/owner/done и
  original результат после ожидания в прежнем общем бюджете. Все cancellation
  stages по-прежнему раньше всех joins. cancelNetWatch фиксирует original fences
  до callbacks и регистрирует revoke/stop/cancel stages через существующий helper.
  Foreign App не получает cancel; timeout не разрешает замену владельца.
- Explicit DNS/poller rearm после original done дополнительно требует
  cleanupKnown. Abnormal body допускает последующее явное восстановление только
  при известной original cleanup и прежних Manager.Stop/helper/runtime/native
  условиях. retireFailedServiceDNS — локальная fault/revocation процедура, её
  return не заменяет отдельную проверку фактического Engine/Manager retirement.
- power_notifications_windows.go добавлен в scope до редактирования: original
  owner.power и process callback entry опубликованы до Pin/native Register.
  Guard сохраняет известный handle или unknown native acquisition; nil factory
  result не перетирает original partial entry. Retire фиксирует attempted/error
  до native call и возвращает сохранённый результат, не повторяет unregister
  после error/panic/Goexit. Admission закрывается до original callback Wait;
  abnormal callback удерживает pinned parameters/process slot. DEVICE_NOTIFY_
  CALLBACK=2, DWORD status и ULONG event ABI не менялись. Native fast callback
  не получил новых workers/native I/O/logging, только local outcome flag.

### Ручной разбор и границы доказательств

Seven original diffs/two new components/callers reviewed вручную: original identity,
LIFO cleanup, separate functional/resource outcomes, WaitGroup admission sealing,
callback/registration partial acquisition, one unregister и explicit rearm gates.
gofmt batches9+3+1 завершились exit0.13 snapshots11existing2new; original12 archived
до dependent native expansion. task71rev782 archived;13 reviewed source SHA и
2 cached systray v1.2.2 source SHA сохраняются. Early progress checkpoint отражал
план расширения до9Go, final checkpoint фиксирует реализованный source scope.

Cached SDK source: Register запускает ready отдельно; Windows nativeLoop может
вернуться на GetMessage error/WM_QUIT без error result; Quit скрывает PostMessage
result, WM_CLOSE скрывает результаты DestroyWindow/unregister. Поэтому even
normal Run/exit/Quit frames не дают native window/class/thread/kernel cleanup
proof. Pin/Register/Unregister/ABI/FFI/callback ordering остаются unverified.
Registered App cleanup не доказывает completion всех SDK/OS работ, delivery RPC,
hard wall-time/RSS/wire bound или улучшение скорости/безотказности.

Tests/fixtures/build/vet/typecheck/parser/options/SDK/browser/native/admin/network
и product checks не выполнялись; реальные logs/vault/profiles/secrets не читались.
Existing tests/fixtures/dependencies, Android/shared/configs/harness/чужие snapshots,
чаты и Git не изменялись. No subagents. Full100 acceptance OPEN/56 checks NOT_RUN.

### Остаток и следующий срез

Autostart registry helpers в tray_windows.go ещё игнорируют registry.Key.Close
errors; legacy checkbox сводит read failure к false. Следующий PC source-срез —
исследовать existing checked API/потребителей и original key/read/write/Close/
readback ownership, затем исправить подтверждённые пробелы без реального registry
I/O. Меню не должно показывать неподтверждённый успех. Native systray cleanup
остаётся отдельным unresolved SDK boundary, не закрытым новым API-frame receipt.
Compile/API/ABI/fixture/race/SDK/native/OS/egress/UX/release/comparative/fullPC/old9
OPEN. All gates=false; ranBeforeJob sticky; GUI6/R1/U1/IPC9/Job8; TUN unavailable/
schema2 unsupported. Никакого live/server изменения или выпуска EXE.


<a id="pc-v332-autostart-owner"></a>

## V2-332 — автозапуск: исходный ключ, точная команда, неизвестное состояние

**Source/unverified, 2026-11-09.** PC-01/04; fullPC00…09/old9 active,
100 OPEN/56 NOT_RUN, без waiver.5 Go/4 docs; V331 был progress.

### Дефекты и изменения

Старые read/set helpers игнорировали registry.Key.Close errors, не имели общего
владельца с terminal cleanup, а tray checkbox сводил read error к false. Cached
x/sys registry.GetStringValue использует realloc loop по native size и обрезает
UTF-16 на первом NUL: строка с чужим suffix могла сравниваться как own command.

- autostart_windows.go сохраняет existing GetAutostart/SetAutostart Wails API,
  quoted absolute executable command и прежний HKCU Run/value. Путь ограничен,
  проверяется UTF-8, absolute form и отсутствие quotes/NUL/CR/LF. Один GetValue
  в buffer64KiB не делает realloc/retry по returned size. REG_SZ и exact UTF-16
  units вместе с последним terminator должны совпасть целиком. Odd/unterminated/
  zero-length/unsupported/oversize/foreign/embedded-NUL suffix дают ошибку.
  Canonical empty REG_SZ и отсутствие value/key при read/disable дают false.
  Включение не создаёт отсутствующий Run key. Приватные read/set helpers
  сохранены, используют тот же owner; production tray/UI идут через App API.
- autostart_registry_windows.go резервирует единственную process operation
  до command/Open, без worker goroutine. Tray/UI не перекрываются; busy — error.
  windows.RegOpenKeyEx пишет прямо в original task.key; wrapper OpenKey,
  обнуляющий output при ошибке, больше не используется. Native start/return, body normal return,
  one Close attempt и cleanup имеют original witnesses. Invalid/pseudo handle,
  interrupted acquisition/Close или abnormal body сохраняют task; последующий
  вызов не заменяет owner и не повторяет Close. Ordinary functional error при
  known body/cleanup не считается неизвестным resource retirement.
- Existing appCleanupStep/sequence переиспользованы: body guard → registered
  original Close → sequence result → immutable task outcome/done. Panic payload
  не выводится; Goexit не превращается в успех. Blocking Reg* остаётся pending.
  After terminal seal новые операции запрещены; all cancellations остаются до
  joins. Shared terminal budget ждёт exact captured task и body/cleanup outcome.
  Foreign App не меняет owner/seal и не получает его successful receipt.
- tray_autostart_windows.go — отдельный menu component, initial ready затем
  original command worker. Unknown label прямо предлагает проверить состояние;
  первый click только read. Если fresh read отличается от показанного known
  state, click обновляет его без записи. Совпадающее known состояние допускает
  явный toggle; display успеха только после checked Set и отдельного fresh Get.
  Read/write/Close error возвращает unknown. Legacy autostartEnabled fallback
  удалён, dummy filepath import убран. Existing logs не заявляют факта будущего
  login, actual launch или actual write, если запрос уже совпадал с чтением.

### Ручной разбор / ограничения

Two original diffs/three new components/callers reviewed: source reserve-before-
native, partial output custody, pseudo-handle exclusion, one checked Close,
normal/error/panic/Goexit/block, exact bounded codec, absent/empty/foreign value,
no-op write, shutdown seal/admission, exact original join and unknown tray click.
Reused existing proxy registry ownership/bounded value and update native-handle
patterns; private APIs этих packages не импортировались как новый public API.
Native handle representation helper nativelease.ValidHandleValue переиспользован.
gofmt5+1 observed exit0.9snap6existing3new/task71rev785 archived;15 reviewed source
SHA и4 cached SDK source SHA. No product imports in metadata scripts.

RegOpenKeyEx/RegCloseKey x/sys wrappers и systray methods исследованы как source,
но ABI/status/callback/window/actual kernel retirement не проверены. Serialized
own operations не дают atomic CAS против внешнего writer между read/write;
наблюдённая чужая команда не перезаписывается, новая race не аттестована. Fixed
64KiB query buffer не является общим RSS/wall-time bound. os.Executable path не
доказывает pinned/current image. Matching registry command не доказывает Windows
StartupApproved/MDM policy/login launch/auto-connect/VPN forwarding.

SettingsView уже использует nullable state/error/readback, source оставлен
неизменным. Замечено для следующего среза: refresh, начатый во время busy toggle,
может завершиться после toggle с прежним autostartRevision и опубликовать старое
значение; successful refresh не очищает прежний autostartOutcome error. Эти
frontend races здесь не закрыты. Приватные fixtures/reset/ABI/компиляция остаются
непроверенными; tests/fixtures не редактировались и не исполнялись.

No tests/build/vet/typecheck/parser/options/SDK/browser/native/admin/network или
product checks, actual registry reads/writes, owner logs/vault/profiles/secrets.
Android/shared/configs/harness/other-role states/chats/subagents/Git не затронуты.
FullPC/old9/compile/runtime/SDK/native/OS/UX/egress/release/comparative OPEN;
100OPEN56NOT_RUN/all gates=false/ranBeforeJob sticky/GUI6/R1/U1/IPC9/Job8;
TUN unavailable/schema2 unsupported. Next: PC04 Settings concurrent refresh/
write/readback publication and old outcome clearing through existing component.


<a id="pc-v384-process-cleanup"></a>

## V2-384 — original child preparation and parent pipe unwind

2026-11-10, **partial source/unverified**, PC-01.

Source review followed App/core positive native commit through Manager failure
paths. Existing Start/Switch/Reload/rollback and terminal rejection already call
protection-before-Engine.Stop and own-proxy cleanup; no second revocation API was
introduced. Compiled native forwarding remains unavailable.

The concrete cleanup gap is in engineProcess.retire: sequential evaluation of
ClosePreparation and closePipes skips parent endpoints if the first callback
exits its goroutine. Sequential input.Close/output.Close has the same problem.

- Reuse existing lifecycleCleanupStep and lifecycleCleanup. Register the original
  child preparation and parent-pipe steps before invoking either callback, in
  the same preparation -> parent endpoint order. All three existing retirement
  branches use this helper; process/Job/image owners and barriers are retained.
- Register input and output Close separately, preserving input -> output order.
  The common finalizer sets pipesClosed only when both ordinary endpoint results
  are successful. Panic(nil)/Goexit keeps the negative result from the existing
  cleanup witness and still executes the other registered step during unwind.
- Native spawnprep.File already records original one-attempt physical Close and
  sticky uncertainty. Reuse it: no replacement file/handle, new goroutine, timeout
  receipt or permanent process-wide failure latch. Known transient preparation
  refusal is not upgraded into a new irrecoverable latch by this wrapper.
- Original Wait, actual I/O completion, empty Job and host Close still determine
  retirement. Successful parent pipe cleanup alone cannot release process/image
  custody or authorize GUI transfer/new forwarding.

1Go5docs/6 existing snapshots/manual1 original source diff/12named0cached;
touched-only gofmt1exit0. Receipts: .artifacts/memory/bootstrap-v384-process-cleanup-.
No tests/fixtures/build/vet/types/parser/SDK/native/browser/network/generation. Deferred cleanup does not interrupt a stalled syscall or prove physical pipe/Job/process/image retirement, deadline, responsiveness or native resource bounds. Root NETGUARD001/legacy/migration/fullPC00..09/old9 remain OPEN;100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported/release OPEN.

<a id="pc-v390-process-retirement"></a>

## V2-390 — early process failure and original waiter outcome

2026-11-10, PC-01 **partial source/unverified**.

Source findings: retire returned on failed Terminate/Empty before closing its
independent preparation and parent endpoints. ProcessCreated/EnsureWait abnormal
return could skip them too. The original awaitExit goroutine had no abnormal
return finalizer, so panic could escape and Goexit left its done unpublished.

- Register original resource cleanup before host callbacks. Reuse lifecycleCleanupStep
  and the existing preparation/endpoint unwind. Local attempt/result prevents a
  duplicate callback during the same retire invocation; no new permanent latch.
- Preserve normal terminate→pipes→Wait→I/O→empty Job→host Close order. Early failure
  attempts endpoints without forgetting process/Job/image or permitting replacement.
  Retain cleanup errors alongside the primary error, without joining the same result twice.
- awaitExit publishes a finite negative waitErr on panic(nil)/Goexit before closing
  the original done. Ordinary Wait returns/errors remain unchanged. done means waiter
  completion; helper/delivery/TUN consumers already reject waitErr, and original
  native Wait/Job/I/O/Close barriers still govern retirement. No new waiter/retry.

Manual1 source diff;13 named read inputs preserved; touched-only gofmt1exit0.
Receipts: .artifacts/memory/bootstrap-v390-process-retirement-. After authorization, cover early termination/query/wait setup
failures, abnormal Wait, ordinary errors, original owner retention and late cleanup.
No tests/fixtures/build/vet/typecheck/parser/SDK/browser/native/network/generation. Source review and touched-only gofmt do not prove compilation, syscall interruption, pipe/Job retirement, cleanup latency, protected egress or release readiness. All PC00..09/old9 acceptance remains OPEN;100OPEN56NOT_RUN, native gates=false, TUN unavailable, schema2 unsupported. Root NETGUARD001 unchanged/review-required.
