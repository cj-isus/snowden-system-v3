# TH-01 / OFF-02/06 — intent от Stop до restart/apply

2026-10-25, V2-151. Только PC/Windows. **Исходники, unverified**. Tests/fixtures
не добавлялись/не менялись/не запускались; compile/build/vet/check/doctor/module/
SDK/gen/helper/child/native/network/live не выполнялись. `gofmt -w` семи
Go-файлов exit0 — форматирование, не Wails/Windows/UAC/locking proof.

## Исправляемое окно

[V2-150](TH-01-STOP-OWNERSHIP.md) связал queued Stop с ID регистрации, однако
успешный `stopCoreFor(stopRestart)` отпускал operationMu до `restartWithArgs`
или update rename/launch. Новый Start/Disconnect/quit мог изменить намерение
между этими действиями. Update source gate отдельно брал operationMu только
на authority/checkpoint и swap, а процесс запускался уже после освобождения.
Давний nil от Stop не является разрешением на любое последующее продолжение.

Раньше mode `wasRunning` и updater `resumeRequested` читались отдельно от Stop
registration. Теперь они захватываются с тем же intent под App.mu. Apply также
сохраняет initiating ID до ожидания updateMu, подготовки и staging: новое
намерение не превращается в свежую authority старого запроса.

## Реализованная транзакция App

`restart_handoff.go` — private transaction token вокруг существующего serializer,
не второй Manager/engine/worker owner и не публичный Wails API.

1. Caller получает initiating operationID. `beginRestartHandoff` проверяет его,
   использует общий `registerStopLocked(stopRestart)` и generation-specific Stop.
   Все V2-150 original Manager/helper Job/image cleanup barriers сохраняются.
2. После успешного Stop token удерживает operationMu до конца apply/launch.
   Defer release обязателен. До commit новое принятое намерение или closing
   отклоняет продолжение; scoped failure не перезаписывает successor.
3. Commit — короткий value-only gate `runtime -> App`: исходный intent,
   closing=false, те же desired/profile-transition, stopped и пустые runtime
   Manager/engine/cc. После него App.Start/Stop/beginOperation/retry не создают
   новый lifecycle intent: операция уже применяется и возвращает busy. Существующий
   UI state `stopping` публикуется после освобождения runtime lock, без новых bindings.
4. После commit проверки источника/rename/UAC/launch проходят без App.mu.
   OnBeforeClose отмечает pending close под тем же mutex, а не уничтожает App
   посередине launch. При отказе pending close атомарно переводится в closing
   до открытия входа для нового Start, затем идёт обычный closeAfterLifecycle.
5. Положительный результат launch помечает App closing до освобождения token.
   Имеющийся requestQuit завершает старое приложение; это launch request,
   не Ready/успешное соединение нового процесса.

Это явная точка упорядочивания: **до commit** Start/Disconnect/quit могут
сделать старое продолжение stale; **после commit** Start/Stop получают busy,
а закрытие ожидает результата применения/запуска. UI не удерживается App.mu
на время UAC; прерываемость самого ShellExecuteW этим не доказана.

Mode использует прежний `wasRunning` contract для auto-connect. Update использует
captured protectionDesired; ordinary fresh startup/trust/protected-probe gates
нового процесса сохраняются. Смена intent при preparation/staging отменяет
Apply перед stop/swap, временный staging-файл по-прежнему exclusively owned.

## Финальный update source gate

`withVerifiedUpdateSourceLocked` принимает уже удерживаемый operationMu; старый
wrapper сохраняется. Тот же private proof/raw bytes, policy schema/roles,
authority/checkpoint identity, observed time, manifest signature и version floor
проверяются непосредственно перед swap. В journal callback остаются только
filesystem/verified-payload операции, без App/lifecycle callback/повторного lock.
In-process profile/source mutation сериализована тем же operationMu; внешние
writers/ACL/reparse/crash/durability этим не объявлены закрытыми.

## Ошибки и нативный launch

Scoped failure устанавливает restart_failed/BLOCKED/Ready=false и отменяет
сетевой retry; обычные network failures сохраняют прежний backoff. Если до
commit выиграл successor, его состояние не меняется. Pending window close
сохраняется при declined UAC/source/apply/launch error.
Interrupted committed path снимает busy и оставляет scoped restart_failed;
panic продолжает распространяться, а operationMu до передачи token имеет defer release.

Исправлен ShellExecuteW false success: код<=32 возвращает ошибку и при нулевом
last-error. [Microsoft Return value](https://learn.microsoft.com/en-us/windows/win32/api/shellapi/nf-shellapi-shellexecutew#return-value)
определяет успех как значение >32; last-error — дополнительная информация.
Для обычного GUI launch после Cmd.Start освобождается parent process handle;
ошибка release не превращает уже принятый launch в разрешение повторного запуска.
GUI handoff не переносится в engine Job, который должен умереть со старым App.

## Изменённые источники

`windows/app.go`, `protection.go`, `restart_handoff.go`, `autoretry.go`,
`updateapp.go`, `update_delivery.go`, `elevation_windows.go`. Новых зависимостей,
bindings/manifests/tests нет. Shared trust/adaptive, Android, configs/harness
и общие LESSONS не менялись; все9 этапов сохраняются.

## Открытый объём и следующий P0

- Текущий updater по-прежнему single EXE. **Pair EXE/helper staging/apply/rollback
  не реализован**; этот handoff не делает его готовым к native helper поставке.
  V2-152 [schema2/issuer + process-local containment](TH-01-UPDATE-PAIR.md)
  добавлены source-only; pair installer остаётся явно pending.
- При launch failure после успешного swap либо неуспешном restore текущий процесс
  остаётся со старым mapped image и изменённым installed path. Нужны own-image
  containment и согласованный pair journal/recovery, без старого worker Start
  из смешанной установки. Это следующий P0 source-срез вместе с manifest/issuer.
- Native launch owner/ack/readiness, bounded UAC/parent death/SingleInstance wait,
  crash/power-loss, image handles/ACL/sharing, version-floor/rollback coordination,
  child compatibility и EXE/NSIS приёмка остаются открытыми.
- Compile/locking/Stop/Close/epoch/clock/protocol/OS/resource/live проверки отложены
  по запросу владельца. Нет доказанного VPN-пути, rollout или comparative win.

## Узкое предложение для общего LESSONS

Сериализация самого Stop не сериализует последующее restart/apply. Захватывать
caller intent и resume facts в одном registration, держать один lifecycle lock,
выделять commit point и сохранять pending Close без App mutex поверх UAC.
Launch acceptance не означает child readiness. V2-151 source reasoning only;
предложение остаётся в Windows-owned task, общий LESSONS не редактируется.

<a id="pc-v231-intent-source"></a>

## V2-231 — original registration от Stop до updater IPC

2026-11-05, Windows **source/unverified**. Полный PC00…09/old9 и прежние
обязательства сохраняются; tests/fixtures/build/product/native execution запрещены.

### Найдено в исходниках

Stop/restart/update хранили scalar operationID. После ожидания mutex/IO этот
ID не отличал original source от successor с тем же значением. Update contexts
не были bound к core OperationSources, а при отсутствии App.ctx возрождались
на Background. Apply delivery preflight сам выбирал App.ctx, не контекст original
request. Legacy staging context после Stop уже относился к прошлой регистрации;
его поздний cleanup оставался после lifecycle unlock.
Это source finding, не runtime reproduction или оценка частоты uint64 wrap.

### Изменения и границы

- Новый private intentRegistration хранит id+original OperationSource. Pointer
  живой original source нельзя восстановить из public ID/JSON или клона DTO.
  sameLocked означает ту же регистрацию для negative cleanup; currentLocked
  дополнительно требует live source для positive maintenance/restart decisions.
- NewApp создаёт initial source при id0. Это maintenance identity до первого
  запроса; beginOperation/probe/native/forwarding по-прежнему запрещают intent0.
  Advance отзывает predecessor; wrap0 source остаётся revoked, не новый допуск.
- Registered Stop captures full identity до operationMu. Negative cleanup не
  требует live operation ctx/source и использует прежние Background reapers.
  Public Disconnect Release/terminal publication сверяет original registration
  после cleanup; scalar internal teardown не разрешает manual Release. Internal
  failure helper по scalar всё ещё capture-at-entry: full async provenance всех
  таких caller этой правкой не объявляется реализованным.
- Restart/update preflight, handoff commit/launched/error, source/code-acceptance
  guards, activation и forward recovery используют original source вместе с ID.
  Новая регистрация не становится старым request из-за совпадения scalar.
  Token release/actual negative cleanup не блокируются source cancellation.
- Existing update operation slot и netEpoch.bindOperation переиспользованы.
  Context carries original intent, отдельный operation source, **реальный уже
  captured** negative path и closing flag. Retire отзывает operation до std ctx
  cancel. Background fallback удалён. No appOperationScope/lifecycle probe grant,
  synthetic path/epoch/health/network availability или новый controller.
- Existing Stop/epoch/Close cancellation сохранена; локальная работа может идти
  с VPN stopped, но это не отменяет прежнюю negative continuity gate. Generic
  cold headless recovery остаётся terminal serial SDK/native operation со своим
  ctx; новый App intent для него не выдуман.
- Apply delivery preflight передаёт bound context в actual SDK helper, final
  source/journal fence проверяет pure original sources. Preview сохраняет App
  context, не получает install authority; missing/canceled lifetime отвергается.
  Уже проверенный delivery prefix и actual storage quarantine сохраняют свою
  negative/security семантику, не стираются из-за потери интереса caller.
- Pair/legacy install после registered Stop берёт **fresh committed** context
  именно нового handoff; old staging source не ретегируется. Legacy source до
  Stop snapshot-bound, external launch проверяет committed ctx/source. Actual
  temporary-name cleanup и root close перенесены до lifecycle token unlock;
  early path очищает те же originals без двойного позднего defer.
- Changed installed-image fact всегда sticky, даже если old request уже stale;
  UI projection этого факта не переписывает successor. Actual uncertain file/
  helper/Job/image owners сохраняются; Release/cleanup не равны VPN Ready.

### Evidence и незакрытое

Source11Go/4 собственных docs, exact-file gofmt11+2+1 exit0; manual source/caller/
registration type/lock/defer/negative-vs-positive/diff/byte-SHA review. Evidence:
`.artifacts/memory/restart-source-source-evidence.json`. Original bytes и точный
previous composite task71 сохранены; журнал только дополнен. Это **не compile/
runtime PASS**, все acceptance остаются OPEN.

Нет tests/fixture edits/run, compile/build/vet/typecheck/product parser/check/
doctor/install/generation/SDK/helper/product/crypto/native/browser/admin/live/
DNS/server/OS-settings/owner-values/other-role/chat/subagent execution.

OPEN: legacy private API/fixtures; initial0/wrap/foreign source/stale queued Stop;
explicit Disconnect vs failed lifecycle/restart; callback/operationMu/source/epoch/
ctx races; same negative identity with revoked source; preflight prefix/clock/
quarantine; pair/legacy/forward/headless recovery; nested defer/temp/root cleanup;
helper/Job/image/policy ownership; actual release/launch/SDK/child/OS/resources.
Source checks before Release/external OS launch не atomic с concurrent registration:
существующий native action gap и protection-controller/service gates остаются.
Legacy single-EXE launcher не становится owned GUI protocol; migration gate прежний.
Не заявлены syscall preemption, GUI readiness, protected path или performance.
Native=false/IPC9/Job8/TUN unavailable/schema2 unsupported не изменены.

Следующее source: normal worker late-result cause propagation, оставшиеся native
Release/launch authorization gaps и scalar async failure callers; весь PC00…09/
old9/SDK/W3b/interprocess/durability/UX/release/comparative scope остаётся активен.

<a id="pc-v232-worker-result"></a>

## V2-232 — original worker I/O failure через отмену и retirement

2026-11-05, PC-01 source/unverified; полный PC00…09 и old9 сохраняются.

### Найдено и изменено

Обычный `engineProcess.exchange` передавал result через buffered channel, а
I/O completion — отдельно. Cancellation могла выбрать другую ветку и скрыть
уже полученный protocol/codec/panic fault. Проверка version/ID/prepared token
происходила только после успешного ожидания caller. Сам reaper знал ioDone,
но не имел original ordinary-worker completion cause.

Теперь существующий `ownedPipeIO` публикует finite err и reply одним done.
Незавершённый original ioDone не заменяется; version/ID/prepared-token validation
идёт в том же I/O goroutine до done. Arbitrary pipe/codec paths не публикуются.
После отмены reply не выдаётся как успешный: текущие context/source gates прежние.
Process хранит отдельно allocated completion header; большой reply/report не
остаётся удержан этим указателем. Новых goroutines/watchers/budgets не добавлено.

`retireIsolatedFailureLocked` сохраняет original process до прежнего independent
bounded reaper, затем объединяет operation error, actual retirement error и уже
completed original I/O cause. Это применено к Start/current/switch/probe/admit,
selector snapshot sync, service-DNS pause/resume и native confirm/commit errors.
TUN scope defers отдельно присоединяют original cause, сохраняя `scope.retired`
только из cleanup receipt. Обычный Stop по-прежнему решает по original Job/exit/
I/O/handles; успешный actual cleanup не отравляется старой ошибкой команды.
При незавершённом I/O нет выдуманной причины или receipt: owner остаётся retained.

### Evidence и остаток

Source7Go/4docs; exact gofmt7+2+1 exit0, manual caller/type/import/lock/completion/
error/diff review и byte/SHA identity artifacts — `worker-result-*` в
`.artifacts/memory/`. Original11 files и exact prior task71 rev459 сохранены.
PLAN/task старые bytes — prefix; ledger goal/constraints/tasks0…70/acceptance/
decisions/old checks сохраняются. Всё это source evidence, не compile/runtime PASS.
Tests/fixtures/build/vet/typecheck/product-parser/SDK/helper/native/browser/admin/
live/network/other-role/chat/subagent execution не выполнялись.

OPEN: compilation и старые fixtures; completed-vs-cancel/deadline/source timing;
panic/Goexit/short/zero/partial frames, request ID exhaustion; original owner retry/
unfinished reap/Job/handles/resource bounds; caller cleanup classification; late
probe lifecycle flags и action-specific semantic validation после cancellation.
Этот срез сохраняет finite transport/correlation causes, но не превращает raw
public Report flags в retirement authority или актуальный health report.
Native Release/external launch atomicity, scalar async failure provenance,
SDK/W3b/interprocess/durability/UX/release/comparative и весь PC00…09 остаются OPEN.
Native=false/IPC9/Job8/TUN unavailable/schema2 unsupported неизменны.

<a id="pc-v237-owned-gui"></a>

## V2-237 — App authorization и owned GUI transfer receipt

2026-11-06, PC-01/08 source/unverified; весь PC00…09/old9 сохранён.

### Причина пересмотра

StartGUI считала OWN5 завершённой передачей, а App позднее вызывала void
markLaunched. Контекст или App phase могли измениться между этими точками.
Просто добавить local one-use callback недостаточно: после OWN5 child уже
ожидал exit parent и мог войти в GUI при parent crash до local acceptance.
Этот промежуточный вариант отвергнут при source review.

### Source protocol

Новый fixed argument `--internal-update-gui-v6` и stages
SE06 → ARM6 → GO06 → OWN6 → USE6 → ACK6 используют прежние52/36-byte buffers.
Initial seed передаёт только original target-process Job/query и parent/sync
handles; эти numeric target values не закрываются в parent handle table.

1. StartGUI запускает original suspended owned process, проверяет image, original
   Job/handles и OWN6. Pipes остаются открыты: это preparation, не App acceptance.
2. CommitGUI проверяет exact owner/process/I/O completion/image/source и выдаёт
   opaque UpdateGUICommit только finite callback. Root под Runtime→App locks
   повторяет original registration/renderer/lifetime/runtime-empty phase и
   consumes grant через atomic open→authorized. Native I/O под App.mu отсутствует;
   launched/closing ещё не публикуются, original context остаётся пригодным для
   финального bounded exchange. Zero/replayed/closed grant не разрешает USE6.
3. После повторного actual authority/source guard core отправляет USE6. Child
   только после exact digest/stage, image и Job checks отвечает ACK6. До USE6
   EOF/timeout/parent crash ведёт в terminal dispatcher error, без GUI fallback.
4. Parent проверяет ACK6, original image/source/process и завершение pipes,
   переводит decision в transferred и возвращает opaque UpdateGUIReceipt.
   Receipt consumes только matching original grant, transferred→finalized.
   Это completed ownership fact: late ctx loss/error не разрешает возродить
   old App или убить уже переданного owner. Core сохраняет receipt рядом с
   поздней ошибкой; root всё равно завершает parent после valid receipt.
5. Child ожидает exit исходного parent handle, заново проверяет actual SDK/current
   code/image, затем Wails startup и DOM acceptance. Это прежние независимые gates;
   никакой переданный report/receipt не равен GUI Ready, health или VPN admission.

Normal pair apply, forward recovery и headless cold repair используют этот
порядок. Headless сохраняет stopped/proxy и собственный original maintenance
lifetime, не принимает resume из unauthenticated args. h.launchCommit связывает
App permission с exact wire receipt; финальное closing — после native exchange,
без повторного positive admission по уже изменившемуся контексту.

### Cleanup / compatibility

Перед transferred failure/panic/Goexit закрывает grant и вызывает существующий
independent original Job/I/O reaper. Uncertain owner остаётся сохранён; чужой
replacement record не становится abort authority. После transferred Close
возвращает ErrUpdateGUICommitted и сохраняет original process/Job/image до
parent OS exit, не закрывая handle под outstanding Wait. GUITransferred теперь
означает final wire transfer; OWN6/authorization отдельно этого не доказывают.

GUI5/GUI6 не смешиваются: exact argument/stage mismatch terminal/fail-closed.
Engine worker IPC9, Job ceiling8 и code-acceptance worker purpose/version
независимы и не менялись. Не добавлены parallel monitor/resource budget; один
дополнительный fixed36+36-byte auth/ACK exchange идёт последовательно по original
pipe lane, используя существующий per-exchange I/O goroutine/completion header.

После отправленной авторизации crash/late-cancel исходы требуют отдельной OS
приёмки; distributed exactly-once, kernel atomicity, hard latency/total memory,
physical path continuity и GUI survival не доказываются source predicates.
Legacy exec.Command/ShellExecuteW single-EXE/mode restart ещё unowned: миграция
к отдельному authenticated image/owner purpose остаётся в полном плане. Нельзя
подставлять self EXE hash вместо signed manifest/SDK/pair authority.

### Evidence / OPEN

Source8Go/4docs; exact gofmt7+1+7+2+4 exit0, manual original/current source
comparisons/types/callers/lock graph/frame shapes/one-use phases/receipt/error/
child-before-after-parent-exit review и named byte/SHA. Exact12 originals
(initial11 manifest отдельно), prior task71 rev477 и predecessor chain —
`.artifacts/memory/launch-owner-*`; evidence `launch-owner-source-evidence.json`.
PLAN/task old byte prefixes retained; full goal/constraints/tasks0…70/acceptance/
decisions/old checks preserved, all acceptance OPEN.

No tests/fixtures/build/compile/vet/typecheck/product parser/check/doctor/install/
generation/SDK/helper/product/crypto/native/browser/admin/live/network/owner-values/
other-role/chat/subagents. OPEN: API/private fixtures/compilation; GUI5→6 packaging;
zero/foreign/replayed/late grants and receipts; partial/EOF/wrong frames/crash at
each stage; callback nil/panic/Goexit/late error; source/renderer/lifetime loss;
original process/I/O/image/Job custody, parent wait/SDK/DOM readiness, race/resource/
native/OS/elevation/interprocess acceptance. Legacy owned launch/Source gaps и
full PC00…09/old9/SDK/W3b/durability/UX/release/comparative OPEN. Native=false/
IPC9/Job8/GUI6/TUN unavailable/schema2 unsupported; no runtime/safety/performance PASS.

<a id="pc-v238-mode-startup"></a>

## V2-238 — Предусловия перезапуска режима и остановка неподтверждённого старта

2026-11-06. PC-01 source/unverified; полный PC00…09 и прежние обязательства сохранены.

### Найденные дефекты

1. switchMode мог сначала остановить VPN и вызвать UAC для TUN при закрытом
   compiled native forwarding gate. Запрошенный режим всё равно не мог получить
   допуск трафика, но действующее соединение уже было прервано.
2. Режим использовал restartWithArgs без исходного operation/path context.
   Handoff блокировал конкурирующие регистрации, но не покрывал потерю path,
   lifetime и deadline перед фактическим запросом OS.
3. После таймаута ожидания старого экземпляра main продолжал housekeeping и
   startup. CreateMutex с неизвестным результатом или неуспешный Close не давали
   подтверждения освобождения namespace, хотя Close раньше игнорировался.
4. Ручной quoteArg не реализовывал полные Windows правила backslash/quote.

### Изменения в исходниках

- TUN capability проверяется до регистрации Stop, смены состояния и UAC. Пока
  nativeTUNForwardingAccepted=false, EnableTUN возвращает объяснимый unavailable.
  Отключение TUN сохраняет прежний путь: gate не мешает уйти из этого режима.
- После успешного handoff commit переключение использует прежний
  beginCommittedUpdate/bindUpdateOperationLocked и operationCancel slot.
  Исходные registration/path/lifetime сохраняются, deadline30s; defer order
  остаётся timeout → endOperation → release handoff. Нового operation owner,
  engine/helper/trust/policy не создаётся.
- restartRequestCurrent повторяет lifecyclePublicationCurrent до и после
  callback guard; сам callback повторяет original handoff/runtime-empty gates.
  Guard проверяется при входе helper и непосредственно перед exec/ShellExecuteW.
  Никакой App/Runtime mutex не удерживается поверх native запроса.
- Принимаются только точные modeSwitchArgs(requestElevation, false|true).
  Empty, duplicate и foreign args не нормализуются в другую команду. Это частный
  внутренний purpose, не generic executor и не публичный Wails API.
- quoteArg использует syscall.EscapeArg. Просмотрен локальный исходник
  `C:/Program Files/Go/src/syscall/exec_windows.go:30`; код не исполнялся.
  ComposeCommandLine для одного аргумента не использован: его первый элемент
  имеет специальные правила имени executable. Стандартный codec корректнее
  выражает empty/embedded quote/trailing backslash, но это source reasoning.
- awaitSingleInstanceReleased сохраняет прежний bool API. Один TryLock guard
  сериализует original probe, nonpositive timeout отвергается. Valid original
  handle сохраняется до подтверждённого Close; неизвестный Close не заменяется
  новым probe. Успех требует нового mutex, подтверждённого Close и ещё живого
  deadline. Sleep ограничен min(150ms, remaining); native call не прерывается.
- При false main пишет диагностическое сообщение и завершает процесс кодом3
  до обычных housekeeping/App/storage/Wails/network services. Partial original
  probe сохраняется в process-local owner до OS exit, без PID/reopen/retry.

### Что этим не закрыто

Проверка перед OS request не атомарна с syscall/source revocation.30s context
не прерывает UAC. Native request success после долгого UAC не превращается в
«не запускалось» из-за поздней отмены, но его исходный process handle/Job/image
ещё не удерживаются как в GUI6. Cooperative named mutex — только namespace
Wails: новый probe не доказывает exit конкретного parent и не защищает окно
между probe и Wails. Legacy markLaunched и child admission требуют отдельной
миграции, включая поздний источник/ошибку после принятого native request.

Обычный exec из elevated parent сам по себе не выполняет de-elevation. Режим
SOCKS и низкие привилегии процесса — разные факты. Source change не обещает
снижения прав, полной защиты дерева, GUI readiness или непрерывности VPN.

Полная миграция остаётся отдельным пунктом: exact original process/Job/image,
отдельный private restart purpose, child authorization и completed transfer fact.
Для same-application restart уже есть openVerifierApplication с on-disk custody,
который не аттестует loaded sections и не является update signing authority.
Для single-EXE update нужны actual signed manifest/SDK/image и свежая child
проверка; нельзя заменить их собственным EXE hash или объявить app-only receipt
приёмкой подписанной пары. UAC требует собственного bridge/службы и оригинальных
native handles; это не готовый FR-009 ProtectionController.

### Evidence и открытая приёмка

4 Go-файла и4 собственных документа; exact gofmt4+1 exit0. Manual4 исходных
сравнения, API/callers/types/lock order/defer/error/startup sequence и named
byte/SHA identities;8 точных оригиналов и task71 rev481 сохранены в
`.artifacts/memory/legacy-launch-*`. PLAN и этот task сохраняют старый byte prefix.
Полные goal/constraints/tasks0…70/acceptance/decisions/old checks сохранены;
task71 архивирован, а не отменён. Все acceptance OPEN.

Никаких тестов/fixtures/build/compile/vet/typecheck/product parser/check/doctor/
install/gen/SDK/helper/product/crypto/native/browser/admin/live/network/owner
values/other-role/chat/subagent execution. Старое exact-string quoting ожидание
в fixtures может требовать обновления после разрешения; fixtures не менялись.
Compile/API/canceled-nil-stale/guard failure/private args/quoting/native errors/
Close uncertainty/timeout/parent wait/race/resources/OS/elevation/de-elevation
и full PC00…09/old9/SDK/W3b/interprocess/durability/UX/release/comparative OPEN.
Native=false/IPC9/Job8/GUI6/TUN unavailable/schema2 unsupported. Нет runtime,
safety, performance, hard-deadline или OS acceptance PASS.

<a id="pc-v239-restart-custody"></a>

## V2-239 — Обычный перезапуск с исходным process/Job/image owner

2026-11-06. PC-01 source/unverified. Весь PC00…09 и прежние обязательства сохранены.

### Назначение и переиспользование

Обычный non-UAC mode restart раньше использовал exec.Command/Process.Release,
а child ожидал только cooperative Wails mutex. Новый путь сохраняет исходные
process/Job/pipe/image handles и использует уже существующие GUI6 suspended spawn,
target handle duplication, finite I/O, original reaper, App grant и final receipt.
Не добавлен второй engine owner, новый trust/policy или generic executor.

Это same-application restart, отдельный от signed update purpose. Родитель
проверяет и удерживает текущий on-disk image через прежний verifier image lease:
NTFS/ACL/nonreparse/single-link/FileID/ancestor/deny-write-delete/current-process
path и suspended-child path. Собственный EXE hash не выдаётся за подпись/SDK proof
или аттестацию mapped sections. UAC и single-EXE update не мигрированы этим срезом.

### Протокол R1 и разделение authority

- Fixed argument `--internal-restart-gui-v1`; fixed frames
  SR01 → AR01 → GR01 → OR01 → UR01 → KR01. Binding — private32-byte random nonce,
  не manifest SHA, credential, ключ или install authority. Разные argument и
  stages не дают смешать R1 с GUI6. Buffers прежние52/36 bytes, схема installation
  и engine IPC не меняются.
- RestartGUIOwner оборачивает private custody часть UpdateCodeAcceptance;
  signed constructor всё ещё требует actual schema2/SDK/image. Restart owner
  не имеет signed manifest/worker argument и отвергается Verify. Публичный
  UpdateGUICommit/Receipt означает только custody: он не публикует SDK verdict.
- Выбор purpose происходит под original owner.mu, без public boolean/report
  переключения signing authority. Win32 spawner принимает только два фиксированных
  GUI invocations через существующий argument predicate; каждый dispatcher
  дополнительно требует собственный exact argument. Malformed reserved args
  завершают процесс, без normal GUI fallback.
- Child bootstrap вынесен в один private helper с actual purpose-specific
  prepare/validate/verify callbacks. GUI6 сохраняет SE06/ARM6/GO06/OWN6/USE6/ACK6
  и повторную actual SDK/current-code/image проверку после original parent exit.
  R1 повторяет on-disk image/Job guards и затем обычные startup gates.

### Владение и порядок

1. App под operationMu сохраняет новый pure RestartGUIOwner до первого native
   действия. Тот сохраняет pure image allocation перед in-place Prepare.
   Старый openVerifierApplication использует ту же подготовку; его delivery
   purpose не получает новых signing прав. Partial panic/Goexit не выбрасывает
   новый R1 image/ancestor owner.
2. Original source guard и closed burn проверяются вокруг подготовки и обменов.
   Процесс запускается suspended, получает original Job, проверку image и
   target-process duplicates query Job/original-parent synchronize handles,
   затем Resume. Parent не закрывает numeric remote handle в собственной table.
3. OR01 означает preparation. Под original Runtime→App value locks одноразовый
   grant разрешает финальный UR01; native I/O под App.mu отсутствует. До UR01
   child не может перейти к parent wait/Wails; EOF/crash ведёт в terminal error.
4. KR01 и original I/O/image/source/process/pipe gates дают отдельный matching
   final receipt. Parent финализирует launched/closing и обязательно выходит
   даже рядом с late error. При local finalization panic/Goexit/uncertainty после
   actual wire custody отдельный terminal exit запрещает возобновить old App;
   это отрицательное обязательство, не positive GUI Ready/SDK admission.
5. Child ждёт actual original parent handle, повторяет image/Job/context gates,
   подтверждённо закрывает extra image до housekeeping и хранит non-inheritable
   lifetime Job reference до собственного OS exit. Normal main затем повторяет
   установку/floor/profile gates; initial-source auto-connect остаётся отдельным
   допуском. Mode/resume берутся из private session, не unrelated CLI flag.

До transferred ошибка закрывает grant и запускает original Job/I/O reaper.
Uncertain process/image Close сохраняет exact App owner; replacement запрещён.
Перед new engine, positive protection Release и disconnected power recovery
один root adapter повторяет cleanup original owner. Confirmed Close очищает
App pointer; accepted GUI сохраняется до parent exit и не уничтожается Stop.
Новая root custody ошибка проецируется в cleanup_unconfirmed с прежним blocked
retry outcome. Фоновых очередей и мониторов не добавлено; в App допускается
один R1 owner. Parent/child image leases живут одновременно до parent exit;
общий process memory/RSS/latency потолок этим не доказан.

Обычный bare-exec fallback из mode helper удалён. Остался отдельно названный
requestElevatedModeRestart для UAC; TUN compiled gate=false продолжает запрещать
его вызов до Stop. Обычный spawn наследует token родителя: SOCKS и de-elevation
не являются одним фактом.

### Открытые границы

On-disk custody не аттестует kernel loaded sections и не защищает от hostile
trusted-owner/admin/kernel/preexisting writable map. Nonce — correlation,
не independent code identity. Job/handles/source predicates не доказывают
kernel atomicity, distributed exactly-once, physical continuity или hard latency.
Другой legitimate instance может занять Wails namespace после parent exit;
GUI Ready и новый VPN требуют собственных фактических gates.

Далее: UAC original handle/bridge/служба и child authorization; single-EXE actual
signed image/manifest/SDK/fresh child acceptance; корректный de-elevation;
scalar async provenance и прежний полный PC00…09/old9/SDK/W3b/interprocess/
durability/UX/release/comparative. Не включать schema2 install по этому коду.

### Evidence и приёмка

Source14Go/4docs. Exact gofmt12+4+1+4 exit0. Manual review трёх новых компонентов
и11 original/current diffs: типы/потребители/locks/source/lifetime/stages/frames/
defer/receipt/error/cleanup/startup.18 exact originals, initial16 manifest и
task71 rev484/predecessor chain в `.artifacts/memory/legacy-custody-*`.
PLAN и этот task сохраняют старый byte prefix; full goal/constraints/tasks0…70/
acceptance/decisions/old checks не заменены. Все acceptance OPEN.

Не добавлялись/не менялись/не запускались tests/fixtures; нет build/compile/vet/
typecheck/product parser/check/doctor/install/gen/SDK/helper/product/crypto/native/
browser/admin/live/network/owner values/other-role/chat/subagent execution.
Compile/API/fixtures/GUI6 refactor/R1↔GUI6/version/nonce/replay/zero-foreign-late
grants/EOF/partial/wrong frames/crash at each stage/parent death/image-close/
local-finalization panic-Goexit/SDK/DOM/initial auto-connect/reap/race/resources/
OS/packaging/elevation/de-elevation OPEN. Native=false/IPC9/Job8/GUI6/R1 source/
TUN unavailable/schema2 unsupported. Runtime/safety/performance PASS отсутствует.

<a id="pc-v240-uac-owner"></a>

## V2-240 — Родительский UAC request owner; child bridge ещё unavailable

2026-11-06. PC-01 source/unverified; весь PC00…09 и прежние обязательства сохранены.

### Что меняется

Старый ShellExecuteW возвращал только код принятия запроса. Его helper удалён:
App сохраняет pure ElevatedGUIRequest до открытия image, COM и process/Job/token
handles. Новый ShellExecuteExW пишет hProcess непосредственно в сохранённую
структуру owner, включая partial failure. Ни PID reopen, ни Process.Release,
ни generic executable/argument API для UAC не добавлено.

Путь — текущий on-disk verifier image с прежними NTFS/ACL/FileID/ancestor/PE/
deny-write-delete/current-process path guards. Это не loaded-section attestation,
не подпись, не schema2/update authority и не новая доверенная служба.

### Native contract

- SHELLEXECUTEINFOW/flags сопоставлены с установленным Windows SDK26100
  shellapi.h:395–503. Runtime ABI predicate требует размер112/offset hProcess104;
  W вызов, exact file/cwd, verb runas, fixed --tun/optional --auto-connect/
  --await-previous. Association/IDLIST/class/async inputs остаются zero.
- Используются NOCLOSEPROCESS/NOASYNC/UNICODE. NOASYNC не доказывает wall-time
  bound, не прерывает UAC и не означает готовность child. Никакой timeout
  goroutine не объявляет зависший native вызов завершённым.
- COM STA+DISABLE_OLE1DDE выполняется на locked OS thread. По pinned x/sys0.44
  CoInitializeEx возвращает S_FALSE как Errno1: этот результат владеет COM ref
  и требует Uninitialize. Changed apartment/прочая ошибка не получает fake
  ref и не вызывает чужой Uninitialize. Незавершённый Shell/COM cleanup не
  возвращает thread в pool и сохраняет original uncertainty.
- Info/UTF16 pointers закреплены через retained runtime.Pinner. Если native
  return не подтверждён, pins/COM/owner не выбрасываются. После подтверждённого
  NOASYNC return и COM cleanup pins снимаются, прежде чем дать request receipt.
- Проверяется original handle object, nonzero/nonself PID только сравнением
  (без сохранения/acquisition по PID), actual original process image path.
  Handle inheritance выключается. Неизвестный enclosing Job отвергается:
  ShellExecute не даёт CREATE_BREAKAWAY_FROM_JOB, и lifetime старого Job нельзя
  выдать за независимое владение child.
- Новый unnamed lifetime Job использует прежние configure/accounting primitives
  с KILL_ON_JOB_CLOSE/BREAKAWAY_OK. После actual assignment query-only handle
  дублируется в exact target process, non-inheritable. Parent не закрывает
  numeric target value в своей handle table. Это kernel lifetime reference,
  но не подтверждение child/adoption/readiness.
- Original process должен оставаться live, принадлежать original Job с exact
  flags и иметь elevated token того же пользователя, что actual self image
  trust. Foreign credential/profile substitution отвергается. Token handle
  также сохраняется в owner; actual query values агентом не читались.

### Receipt и cleanup

Отдельный ElevatedGUIReceipt означает только проверенный original process
request. Его нельзя трактовать как GUI6/R1 ACK, native forwarding permit,
GUI Ready или SDK verdict. Native validation/source guard предшествуют receipt;
Consume под Runtime→App value locks повторяет original handoff/source/runtime
gates и exact-owner CAS. Close конкурирует за тот же atomic phase: либо новая
передача, либо retirement, без native I/O под App.mu.

После Consume App sets launched/closing, original process/Job/image/token
сохраняются до parent OS exit. Late local error/panic/Goexit после этого факта
вызывает terminal parent exit, не разрешает повторный запуск/возобновление old App.
До Consume ошибки сохраняют exact partial owner. Общий Start/Stop/power helper
adapter учитывает его до engine/protection Release/observer replacement.

Assigned Job ретирируется по original handle, root wait и actual empty accounting;
нет pending Wait goroutine, под которой закрывается handle. Неподтверждённые
Terminate/Wait/Close/COM/pins не заменяются. Unassigned running process может
получить один negative abort по exact original matching-image handle; его
root exit не доказывает retirement потомков. Unknown/live mismatched image
не становится произвольной kill authority. UAC decline с ERROR_CANCELLED и
нулевым hProcess отделён от generic missing-handle uncertainty.

ShellExecute запускает child RUNNING до Job attachment. Ни поздний empty Job,
ни успешный root reap не доказывают отсутствия pre-attachment descendants.
Этот witness намеренно остаётся cleanup_unconfirmed даже после закрытия known
assigned handles; image/owner сохраняется до terminal parent exit. Не разрешать
Release/new engine на основании только leaf exit или subset accounting.

### Почему UAC пока нельзя включить

Private child-auth bridge ещё отсутствует. Старый --await-previous namespace
не предотвращает child startup при crash parent до App authorization. Новый
request receipt и target Job duplicate этого не исправляют.

Поэтому elevatedGUIChildBridgeAvailable=false — отдельный compiled predicate.
App проверяет его до Stop, core — до native acquisition; успешное будущее W4c
не активирует этот незаконченный путь. Нет env/linker/public bool override.
TUN native acceptance также остаётся false. Это явный incomplete stage,
не завершённый UAC/handoff, native service или FR-009 ProtectionController.

Далее требуются actual private child bridge/permission contract, устранение
running-before-Job и parent-crash-before-authorization gap, deterministic Job
lifetime/rights/handle accounting. Остаются signed single-EXE actual manifest/
SDK/fresh child authority, de-elevation, scalar async и полный PC00…09/old9/
SDK/W3b/interprocess/durability/UX/release/comparative.

### Evidence и открытая приёмка

Source8Go/4docs. Exact gofmt8+1+1+1+2+1 exit0. Manual4 новых компонентов и4
original/current diffs; types/callers/locks/CAS/defer/native-layout/COM/token/
Job/cleanup source review. Локальные SDK shellapi.h/objbase.h и pinned x/sys
syscall/security source прочитаны, не исполнены.12 original snapshots (новые
source — empty), task71 rev488 и predecessor chain сохранены в
`.artifacts/memory/uac-owner-*`. PLAN/task old byte prefixes сохранены;
goal/constraints/tasks0…70/acceptance/decisions/old checks не заменены.

Ни tests/fixtures/build/compile/vet/typecheck/product parser/check/doctor/install/
gen/SDK/helper/product/crypto/native/browser/admin/live/network/owner values/
other-role/chat/subagent execution. Compile/ABI/COM/S_FALSE/apartment/pinning/
UAC decline/partial/missing-original-handle/rights/SID/foreign Job/assignment/
target duplicate/Close/each-stage crash/Goexit/pre-attachment descendants/private
child bridge/OS/packaging остаются OPEN. Никакой runtime/safety/performance,
whole-tree/GUI readiness/atomicity/latency/RSS acceptance не получен.
Native=false/UACbridge=false/IPC9/Job8/GUI6/R1/TUN unavailable/schema2 unsupported.

<a id="pc-v241-uac-bridge"></a>

## V2-241 — Private U1 parent/child bridge, source/unaccepted

2026-11-06. PC-01; полный PC00…09 и прежние обязательства сохранены.

### Transport и reuse

Вместо старых --tun/--await-previous child получает только fixed reserved
`--internal-elevated-gui-v1 <nonce> tun resume|stopped`. Случайный32-byte nonce
связывает единственный original owner с local named pipe и fixed frames
SEU1 → ARU1 → GOU1 → OWU1 → USU1 → AKU1. Это correlation, не signing authority.
Размеры52/36 bytes прежние; подпись/SDK/installation schema/engine IPC не меняются.

Существующий go-winio (pinned tailscale fork из go.mod) изучен. Его file Close
игнорирует CancelIoEx/windows.Close и возвращает nil; listener Close также не
предоставляет original handle retirement receipt. Для PC-01 это не доказательство
cleanup. Новая зависимость/модуль/lockfile не добавлены: использованы закреплённые
xsys bindings и existing GUI bootstrap/frames/Job/image/source primitives.

Parent хранит pipe owner до CreateNamedPipe: one instance, FIRST_PIPE_INSTANCE,
byte/NOWAIT, REJECT_REMOTE_CLIENTS, user+SYSTEM protected DACL. Child открывает
ровно derived local name; SQOS IDENTIFICATION не даёт server выполнять операции
с правами elevated client; child
устанавливает/проверяет NOWAIT. Nonzero/unknown original handle не заменяется;
повторное listen/dial на том же owner запрещено. Retry при pipe busy/not-found
не заменяет существующий handle. Параметры quota256/256 — request к kernel,
не доказанный total-memory/RSS потолок.

Обмен синхронными NOWAIT Win32 calls использует fixed local buffers и не создаёт
I/O goroutine/overlapped request/очередь. Scope/peer проверяются вокруг операций;
частичные Write/Read учитываются, NO_DATA poll имеет deadline и10ms паузу.
Native syscall/driver может не быть прерываемым; hard latency этим не доказана.
Read bytes публикуются только после полного frame и original scope/peer checks.

### Peer identity и порядок

- Parent сравнивает GetNamedPipeClientProcessId с GetProcessId original UAC
  hProcess, который остаётся удержан. PID служит сравнением, не acquisition.
- Seed передаёт query Job + original-parent query/synchronize target-process
  duplicates. Parent сохраняет оба remote numeric values, не закрывая их в
  своей table. Child принимает только valid non-inheritable handles, actual
  membership/exact Job flags и nonself parent handle.
- Child GetNamedPipeServerProcessId сверяется с original duplicated parent
  handle. Query parent image path должен совпасть с actual self-image path;
  parent token SID — с actual self-image trust user, сам child должен быть
  elevated. Token query/Close имеет original owner; неизвестный Close terminal.
- Core child owner публикуется до native dial/prepare и удерживается даже при
  partial panic/Goexit. Dispatcher вызывается перед остальными main modes,
  housekeeping/storage/Wails/services; malformed reserved args не становятся GUI.

Проверки соединения и image/Job не заменяют App authorization. Execute доводит
только до OWU1 preparation. Commit получает opaque one-use grant под original
Runtime→App value locks; launched/closing ещё не публикуются. Только после USU1
child может подтвердить AKU1, закончить transport и ждать actual parent handle.
Без USU1 EOF/parent crash после входа в bootstrap ведёт в terminal failure.

Parent после exact ACK повторяет original source/image/token/Job/process checks,
закрывает original pipe, сохраняет actual final receipt. Root сопоставляет его
с original owner/grant и публикует closing как completed ownership fact.
Поздняя отмена/ошибка не отменяет этот факт и не разрешает повторный запуск;
panic/Goexit/local finalization uncertainty сохраняет terminal parent exit.
Execute/Commit scope проверяет и исходную operation: новый ctx не продлевает
старый deadline/intent/path. Close конкурирует за тот же atomic phase.

### ACK/EOF race

Child final ACK остаётся connected до parent server EOF. Parent сначала читает
ACK и проверяет actual source, затем закрывает endpoint. Child send-only ACK
не делает post-write peer query, которая могла бы гоняться с нормальным server
Close. EOF wait использует только что проверенный unchanged NOWAIT mode без
повторной metadata query, которая могла бы ошибочно объявить normal EOF отказом.
Любой лишний byte в EOF phase — ошибка; EOF сам по себе не разрешает сеть.
Затем сохраняются original-parent wait, self-image/Job recheck, checked image
Close и обычные install/floor/profile/initial-source auto-connect gates.

GUI6/R1 продолжают использовать прежний stdin/stdout exchange и stream Close
через thin transport adapter. Их argument/stages/SDK/DOM semantics не заменены
U1 nonce или частным pipe report. Подписной update owner не выбирает U1 purpose.

### Что не закрыто

Source bridge существует, но elevatedGUIChildBridgeAccepted=false и native=false.
Ни App до Stop, ни core/child entry не включают UAC до отдельной приёмки.
Gate переименован с missing availability в unaccepted, без runtime override.

Private main dispatcher не аттестует весь pre-main/import/init/loader surface.
UAC запускается running до Job assignment. Handshake/Job/leaf exit не доказывают
отсутствие pre-attachment descendants: прежний witness не сброшен. Требуется
следующий минимальный bootstrap/init-side-effect contract и проверка оригинальных
rights/Job/close/parent-crash/packaging на Windows, когда это будет разрешено.
До этого нет whole-tree/OS atomicity/GUI Ready/forwarding/performance PASS.

Остаются signed single-EXE actual manifest/SDK/fresh child authority, de-elevation,
scalar async и весь PC00…09/old9/SDK/W3b/interprocess/durability/UX/release/comparative.

### Evidence

Source12Go/4docs; exact gofmt12+4+1+1 exit0. Manual3 новых компонентов и9
original/current diffs/types/callers/locks/CAS/frames/peer/partial/EOF/defer/
ownership review.16 exact originals и task71 rev492/predecessor chain в
`.artifacts/memory/uac-bridge-*`. PLAN/task старые byte prefixes сохранены;
full goal/constraints/tasks0…70/acceptance/decisions/old checks не заменены.

Ни tests/fixtures/build/compile/vet/typecheck/product parser/check/doctor/install/
gen/SDK/helper/product/crypto/native/browser/admin/live/network/owner values/
other-role/chat/subagent execution. Compile/API/private fixtures/GUI6-R1-U1/
mixed versions/NOWAIT/ACL/SQOS/peer PID/SID/nonce/replay/partial/wrong frame/
EOF/final ACK/server-close race/each-stage crash/Goexit/pre-main/pre-Job effects/
rights/target handles/Close/resource/OS/packaging acceptance OPEN.
BridgeAccepted=false/native=false/GUI6/R1/U1 source/IPC9/Job8/TUN unavailable/
schema2 unsupported; all acceptance OPEN.

<a id="pc-v242-uac-bootstrap"></a>

## V2-242 — Pre-main/bootstrap admission, source/unverified

2026-11-06. PC-01; полный PC00…09 и прежние обязательства сохранены.

### Найденная граница

U1 dispatcher находится первым в `main`, но Go выполняет package initialization
раньше входа в него. В закреплённом Wails v2.13.0 прослежена source-цепочка:
`windows/main.go` → `wails.go` → `pkg/application/application.go` →
`internal/app/app_production.go` (`production`) →
`internal/frontend/desktop/desktop_windows.go` (`windows`) →
`internal/frontend/desktop/windows/frontend.go` → `winc`/`w32`.
Это конкретная production/windows source-ветка, не результат сборки/import parser.

| Локальный pinned source | Действие до main по исходнику |
| --- | --- |
| `winc/init.go:14` | GdiplusStartup; return не проверяется вызывающим init |
| `winc/app.go:22` | LockOSThread, GetModuleHandle, InitCommonControlsEx |
| `winc/font.go:23` | NewFont → GetDC/GetDeviceCaps/ReleaseDC и создание font |
| `winc/icon.go:34` | явные user32.Load/gdi32.Load, panic при error |
| `winc/wndproc.go:18` | RegisterWindowMessage |
| `winc/w32/user32.go:159` | LockOSThread и GetCurrentThreadId |

Наблюдение — наличие native calls в локальном коде зависимостей. Их runtime
эффекты, threads/descendants, DLL resolution/loaded sections и whole import
closure не измерялись. Здесь не установлена уязвимость Wails/DLL и не доказана
чистота прочих init. Изменять module cache, зависимости или Wails fork не нужно.

`ShellExecuteExW` запускает UAC child running; original hProcess и последующее
AssignProcessToJobObject не покрывают предшествующее выполнение. Existing
`ranBeforeJob` остаётся sticky даже после root exit/empty Job/Close. U1 USE/ACK
разрешает main-services после App grant, но не может отменить этот факт.

### Исправление admission

`elevatedGUIBootstrapAccepted=false` — новый отдельный compiled gate в existing
`ElevatedGUIAvailable`, вместе с `elevatedGUIChildBridgeAccepted=false` и native
TUN admission. Вызовы App до Stop и повторно под operation owner, parent Execute
до native acquisitions и child dispatcher используют прежний общий API.
Не появляется runtime override, config-флаг или новое публичное Wails поле.
Комментарий main явно ограничивает ранний dispatcher последующими main-services.

Три факта должны приниматься отдельно: native forwarding/protection, custody
creation/bootstrap и private U1 authorization. Принятый wire protocol сам по себе
не снимает bootstrap gate. Source guard не является реализацией containment.

### Следующий executable seam и критерии решения

1. Переиспользовать существующий `snowden-native-probe.exe` как первый кандидат.
   Его main импортирует netprobe/stdlib; изученные entry/transport не импортируют
   core/Wails. Полный transitive graph/stdlib/native loader ещё не аттестован.
   Текущий `pipeJobReady` проверяет существующий DNS/TUN role; нельзя считать его
   готовой UAC authority или просто добавить публичный arbitrary-executable mode.
2. Не импортировать core в минимальный bootstrap ради пары методов: core связан
   с sing-box и продуктовым engine. Сначала найти минимальный переносимый seam
   existing image/handle/Job/frame primitives; не копировать весь native owner.
   Выделение пакета требует конкретных consumers и сохранения original cleanup.
3. Изменение bootstrap purpose фиксируется в protocol/role и signed pinned pair.
   Existing package.ps1 строит helper и встраивает его SHA в App; менять helper
   отдельно от App/recovery/mixed-version contract нельзя. Третий artifact не
   добавляется без явного installation/rollback решения; public schema2 всё ещё
   unsupported. Packaging source прочитан, сам script не выполнялся.
4. Если elevated helper создаёт GUI, original UAC hProcess относится к helper,
   а не к GUI. Нужны отдельные original process/image owners, сверка каждой
   роли/пути/SID и authentic передача original Job/parent handles. Existing U1
   child same-image-parent predicate нельзя молча ослабить или подменить PID.
   GUI создаётся suspended и допускается после actual Job+image checks; helper
   lifetime, ACK/EOF, original-parent wait и cleanup не теряются на втором hop.
5. Сам elevated helper также начнёт выполняться до late Job assignment. Меньший
   бинарник снижает исследуемую поверхность, но не закрывает этот интервал.
   Для bootstrapAccepted требуется обоснованный creation-time containment
   mechanism либо отдельный принятый контракт всего bootstrap/init/loader
   выполнения с custody каждого ресурса и исключением/учётом descendants.
   Native syscalls, Go runtime и загрузка DLL входят в эту границу. Возможный
   privileged broker/service требует отдельного install/lifecycle решения.
6. До выбора конкретного механизма сохранять gate=false и original witness.
   Main reordering, nonce/ACK, source grep, self hash, root-only termination,
   late empty Job, helper rename или новый context не являются этим механизмом.
7. Будущая приёмка отдельно охватывает source/ABI/package closure и Windows
   creation/initialization/rights, each-stage failure/parent-crash/Goexit,
   original descendants/Job/remote handles/Close/resource bounds, mixed versions
   и negative non-elevated/same-user/unknown bootstrap. Сейчас она NOT_RUN.

### Evidence и остаток

Source2Go/4docs; exact gofmt2 exit0, manual original/current diffs, pinned import
chain/native calls, shared-admission callers и retained witness review.
6 exact originals, task71 rev496/predecessor chain и named primary-source SHA
в `.artifacts/memory/uac-init-*`. PLAN/task прежние byte prefixes сохраняются;
full goal/constraints/tasks0…70/acceptance/decisions/старые checks не заменяются.

Абсолютный plan-only: без tests/fixtures/build/compile/vet/typecheck/product
parser/check/doctor/install/gen/SDK/helper/product/crypto/native/browser/admin/
live/network/owner values/other-role/chat/subagents. Source effects не являются
runtime наблюдением. Полная import/loader closure, actual bootstrap mechanism,
rights/descendants/Close/OS/packaging остаются OPEN; all acceptance OPEN.
Signed single-EXE actual SDK/fresh child authority/de-elevation/scalar async и
полный PC00…09/old9/SDK/W3b/interprocess/durability/UX/release/comparative сохранены.
bootstrap=false/bridge=false/native=false; GUI6/R1/U1 source/IPC9/Job8,
TUN unavailable/schema2 unsupported, нет runtime safety/performance PASS.

<a id="pc-v243-child-bootstrap"></a>

## V2-243 — Minimal childbootstrap wire and sequence, source/unverified

2026-11-06. PC-01; полный PC00…09/old9 и все прежние gates сохранены.

### Существующий код и выделенная граница

В V2-242 установлены pre-main native effects и отдельный closed bootstrap gate.
Для concrete helper необходимы минимальные reuse seams, которые не подтянут
core/engine/Wails в его executable. Перед созданием просмотрены existing
application_gui_protocol/update_gui/update_gui_exchange/update_gui_windows,
restart/U1 child dispatchers, original contextError, helper pipeJobReady и
spawn/image owners. Native lease и image теснее связаны с core; фиксированные
wire bytes и child orchestration можно отделить без переноса ресурсов.

`backend/childbootstrap/protocol.go` содержит один stage table GUI6/R1/U1,
factory закрытого value Protocol,36-byte formatter,52-byte seed и четыре
допустимые exchange shapes. Zero/unknown purpose отклоняется; Protocol не даёт
изменять purpose/binding снаружи. `Stages`/`Frame` — данные, не полномочия.
Старые core `updateGUIFrame`/`validUpdateGUIExchange`/size constants сохранены
как delegates/aliases, так что stream и U1 transport используют тот же формат.

`bootstrap.go` содержит последовательность child bootstrap. Imports нового
пакета: errors, bytes, context, encoding/binary; нет core/Wails/sing-box/xsys,
init/native loader calls, executable entry, файлового или сетевого I/O.
Callbacks могут выполнять native действия; их реализация входит в TCB caller.
Sequencer не запускает goroutine/timer и не владеет OS handles. Это граница
компонента по исходнику, не аттестация всего transitive runtime/loader.

### Порядок и ownership

| Шаг | Сохраняемый инвариант |
| --- | --- |
| Seed | fixed purpose/binding до извлечения target-process handle values |
| Adopt → Prepare | original native lease и image owner остаются в session |
| ARM → GO | purpose-specific preparation раньше разрешения GO |
| Validate → OWN → USE | OWN не становится окончательной авторизацией |
| Validate + Job → ACK | child не подтверждает handoff до USE |
| Transport Close → AwaitParent | exact original parent handle, не PID/EOF authority |
| Verify → Validate + Job | ordinary startup/SDK и original native checks остаются |

Каждый callback/exchange получает тот же original ctx. Проверки std ctx и
обязательный caller Scope окружают шаг; core передаёт existing contextError с
deadline/OperationSourcesCurrent. Новый ctx не создаётся. Nil ctx/zero protocol/
nil required callbacks terminal до первого I/O. Child context по-прежнему не
получает выдуманные parent App sources: фактические peer/image/native checks
остаются в callbacks. Callback error возвращается прежним error; private
protocol errors отображаются в existing ErrUpdateGUIBootstrap. Core Close
adapter сохраняет прежний terminal bootstrap error при transport-close failure.

Final U1 ACK не получает post-write peer query из нового Scope: здесь выполняется
только value context check. Existing pipe ACK/server-EOF rule не меняется.
При cancellation после частичного native эффекта partial owner остаётся у
session; package не закрывает, не заменяет и не повторяет resource acquisition.
Panic/Goexit не превращаются в локальный success: прежний dispatcher/owner
сохраняет terminal handling. One-use/reentrancy и final custody receipt остаются
ответственностью original owner, а не stateless wire formatter/sequencer.

### Подключение и остаток

Три existing consumers — update GUI6, ordinary restart R1, elevated U1 — входят
через прежний core bootstrap API и thin Operations adapter. Parent stage names
берутся из того же Protocol factory; argument grammar, nonce/manifest parser,
image/SDK authority, Job flags, native remote handles, process reaper, GUI Ready
и install schema не расширены. Новый helper role или arbitrary executable mode
не добавлены. Bootstrap/bridge/native gates=false; ranBeforeJob не сброшен.

Далее — минимальный native parent/Job lease seam, затем конкретный helper
bootstrap с original helper+GUI owners, role/image/SID/parent/remote-handle/pair
pin compatibility. Native image/pipe сейчас всё ещё core-bound; helper остаётся
непринятым кандидатом. Full import/loader closure, creation-time/whole-tree
custody, rights/Close/crash/OS/package и comparative gates остаются OPEN.
Signed single-EXE actual SDK/fresh child authority, scalar async, de-elevation,
full PC00…09/old9/SDK/W3b/interprocess/durability/UX/release также сохранены.

### Evidence

Source5Go/4docs; gofmt5+2+1 exit0. Manual2 новых компонента против empty originals,
3 original/current diffs, type/caller/stage/size/byte ordering/error mapping/
nil/operation scope/ACK-EOF/original owner review.9 exact originals и task71
rev500/predecessor chain — `.artifacts/memory/bootstrap-seam-*`; named current
source SHA и unchanged prior-source inventory. PLAN/task старые byte prefixes
сохранены, goal/constraints/tasks0…70/acceptance/decisions/old checks unchanged.

Абсолютный запрет tests/fixtures/build/compile/vet/typecheck/product parser/
check/doctor/install/gen/SDK/helper/product/crypto/native/browser/admin/live/
network/owner values/other-role/chat/subagents. Source review не доказывает
компиляцию/API parity, mixed-version/frame/partial/stale/replay/nil/cancel/
Goexit/EOF/parent-crash/Close/resources/OS/packaging. Все acceptance OPEN.
GUI6/R1/U1 source/IPC9/Job8/TUN unavailable/schema2 unsupported; нет runtime
safety/performance/whole-tree/GUI readiness PASS.

<a id="pc-v244-native-lease"></a>

## V2-244 — Native original-parent/Job lease, source/unverified

2026-11-06. PC-01; полный PC00…09/old9 и прежние gates сохранены.

### Reuse и зависимости

Из existing update_gui_windows извлечён child query-Job/original-parent lease
в `backend/childbootstrap/nativelease`: common error/scope, Windows native owner,
non-Windows fail-closed stub. Базовый childbootstrap wire/sequencer из V2-243
остаётся отдельным stdlib-only Go package. Native subpackage импортирует stdlib
и existing pinned xsys0.44; core/Wails/sing-box/wf/delivery SDK не импортирует.
Module/lockfile/packaging/executable arguments и GUI6/R1/U1 frames не менялись.

Existing core callback signatures adopt/validateJob/awaitParent сохранены через
thin common adapter. Original GUI session содержит zero native lease до Adopt.
Native error mapping сохраняет ErrUpdateGUIBootstrap, cancellation/source cause
и identity purpose-specific callback errors. Non-Windows возвращает отказ.
TUN actor и elevated-parent используют прежние core procedure globals: они не
удалены вместе с перенесённым child lease. Shared representation predicate
ValidHandleValue переиспользован core spawner без изменения его выражения.

### Original ownership и состояния

| Состояние/операция | Правило |
| --- | --- |
| Before Adopt | owner уже в original session; Lease после использования не копируется |
| Claimed seed | exact2 raw values retained до native validation, включая rejected input |
| Adopt | одна попытка, distinct/representable real/noninherit handles, nonself process |
| Job validation | actual self membership, exact KILL_ON_JOB_CLOSE + BREAKAWAY_OK flags |
| Failure | native/callback/partial/panic/Goexit сохраняет original values и failed witness |
| WithParent | callback заимствует handle под тем же TryLock, без публичного raw getter |
| AwaitParent | одна попытка, original handle;15s derived ctx,100ms synchronous waits |
| Parent Close | только после signaled original Wait; error сохраняет value, retry запрещён |
| Success | parent cleared только после confirmed Close; actual Job проверен снова |
| Lifetime | child Job reference остаётся до terminal process exit |

Проверка representation отсекает0/псевдохэндлы, не заменяет object type/identity.
PID используется только для nonself comparison; OpenProcess/reopen по PID нет.
При невалидном input никакой arbitrary local handle не закрывается. Claimed
numeric values — witness input, не принятая authority. Actual granted-access
masks отдельно не аттестуются: query/synchronize rights задаёт existing parent
DuplicateHandle path; rights/ABI/OS acceptance остаётся OPEN.

TryLock сериализует native queries, borrower callback и wait/Close. Callback не
должен сохранять/закрывать borrowed handle или reenter Lease. U1 query peer PID,
same-image path, parent SID/elevation и token owner остались в core и выполняются
в WithParent; lease не заменяет image/auth proof. Повторное/конкурирующее API
действие не получает нового handle, процесс или background waiter. Native и
callback uncertainty блокирует дальнейшие positive operations того же owner.

Scope — existing value-only contextError, с исходными OperationSourcesCurrent
и deadline. Выделенный native lease проверяет ctx/time до и после scope hook,
между finite waits, перед Close и после actual Job recheck. Новый15s ctx только
сужает original deadline и наследует values; новый вызов не продлевает one-use
wait. Mutex удерживается до завершения синхронного Wait/Close; outstanding I/O
goroutine не создаётся. Native syscall/driver может задержаться: hard latency,
process memory/resource cap и cleanup всей ОС этим не доказаны.

### Следующее и открытые границы

Original child native lease теперь доступен без engine import. Native image,
ACL/path leases и U1 pipe ещё core-bound; concrete helper bootstrap не добавлен.
Далее минимальный image/pipe seam и конкретный fixed pinned-helper role, с
отдельными original helper/GUI owners, parent-image/SID/remote-handle/role/pair
compatibility. Нельзя превратить WithParent, Job query, nonce/ACK или минимальный
package в signing/startup/whole-tree authority. ranBeforeJob witness остаётся,
bootstrapAccepted=false/bridgeAccepted=false/native=false.

Source/API/ABI/rights/unknown Job/mixed-version/partial/one-use/reentrant/concurrent/
cancel/deadline/Goexit/parent-crash/Close/OS/resources/packaging остаются OPEN.
Signed single-EXE actual manifest/SDK/fresh child authority/scalar async/
de-elevation и full PC00…09/old9/SDK/W3b/interprocess/durability/UX/release/
comparative сохранены. TUN unavailable/schema2 unsupported.

### Evidence

Source8Go/4docs; exact gofmt8+1 exit0. Manual4 новых источника против empty
originals и4 original/current diffs; type/caller/platform/import/lock/defer/
one-use/failure-witness/wait/Close/parent-token/context/error-boundary review.
12 exact originals и task71 rev504/predecessor chain —
`.artifacts/memory/bootstrap-lease-*`, named source SHA/unchanged prior inventory.
PLAN/task старые byte prefixes и full goal/constraints/tasks0…70/acceptance/
decisions/old checks сохранены. Все acceptance OPEN.

Абсолютный запрет tests/fixtures/build/compile/vet/typecheck/product parser/check/
doctor/install/gen/SDK/helper/product/crypto/native/browser/admin/live/network/
owner values/other-role/chat/subagents. Нет compile/native/runtime safety/
performance/whole-tree/GUI readiness PASS. GUI6/R1/U1 source/IPC9/Job8 сохранены.

<a id="pc-v245-control-pipe"></a>

## V2-245 — Isolated U1 controlpipe and native allocations, source/unverified

2026-11-06. PC-01; полный PC00…09/old9 и все прежние gates сохранены.

### Выделенная граница

Existing uac_control_pipe transport перенесён в childbootstrap/controlpipe;
stdlib + existing childbootstrap/nativelease + pinned xsys, без core/Wails/
sing-box/wf/delivery SDK. Common Name/Scope/polling и non-Windows fail-closed
stub отдельно от Windows owner. Core сохраняет original request/session before
acquisition, thin callback/error adapters, image/parent SID/elevation и App grant.
Нет нового executable mode, arbitrary pipe path, helper role или signing proof.
Existing elevatedPipeName delegates canonical Name с прежним private error.
Wire/argument grammar/GUI6/R1/U1 versions/module/lockfile не менялись.

CheckScope вынесен в базовый stdlib-only childbootstrap и переиспользован его
sequencer, nativelease и controlpipe: original ctx/check, explicit deadline
до и после value hook, без нового ctx/timer. Native owners по-прежнему обязаны
удерживать partial effects при callback panic/Goexit. Новая dependency cycle
не создаётся: base не импортирует native subpackages или core.

### Сохранённый transport contract

| Свойство | Граница |
| --- | --- |
| Name | fixed local prefix + canonical64 lower-hex nonce, не caller path |
| Listen | FIRST_PIPE_INSTANCE, max1, BYTE/NOWAIT, REJECT_REMOTE_CLIENTS |
| ACL | protected DACL current primary-token user + SYSTEM, owner=current user |
| Dial | exact name, SQOS IDENTIFICATION, BYTE/NOWAIT проверяется |
| Peer | kernel PID comparison только к original held process/parent handle |
| Frames | existing52/36 bytes, owned fixed buffers, bounded partial counts |
| Poll |10ms local timer под original deadline, без detached I/O goroutine |
| Close | original handle, one attempted native Close, unknown result retained |
| EOF | после successful client send-only frame; no metadata/peer query |

Client final ACK не делает post-write peer query; parent может уже прочесть ACK
и закрыть server endpoint. awaited EOF сохраняет verified unchanged NOWAIT;
любой extra byte — error, EOF не разрешает GUI/network/startup. Image/Job/
original-parent wait и normal startup/SDK остаются отдельными gates.

### Обнаруженные cleanup gaps и правки

Pinned xsys0.44 SID.String и SecurityDescriptorFromString используют native
allocations и defer LocalFree, не возвращая его outcome. На этом pipe пути они
заменены существующим ConvertSidToStringSid binding и raw fixed W-SDDL call с
output непосредственно в retained owner. SID получается из validated current
primary token; не принимается SDDL/SID от caller. Bounded256 UTF16 conversion
сохранена по existing pinned String source. Text/descriptor pointers удерживаются
до cleanup; освобождение в обратном порядке после pipe handle, LocalFree return
проверяется. Partial/error/unreturned acquisition не заменяется новым pointer.
После неопределённого Free/Close повтор на том же numeric value запрещён;
pointer/handle обнуляется только при confirmed success. Native quota256/256 и
две retained allocations не являются доказанным total-memory/RSS cap.

Raw SDDL W ABI сверена с pinned zsyscall wrapper: four args, revision1, retained
SECURITY_DESCRIPTOR** output, optional size=nil. Local SDK sddl.h по ожидаемому
пути отсутствует; compile/header/ABI/native gates OPEN. Module cache не менялся.
Это не repository-wide исправление всех LocalFree helpers и не runtime leak proof.

TryLock сериализует endpoint state/native calls/Close. Pure atomic Retire intent
устанавливается даже когда Close busy, и проверяется вокруг positive scope:
ошибка/unknown cleanup не оставляет endpoint пригодным к новому обмену.
Acquiring/failed/close/free flags сохраняют конкретную original uncertainty;
Retire не утверждает syscall interruption или cleanup. Negative EOF wait после
ACK использует plain value scope, иначе нормальное завершение было бы запрещено
собственным Retire. Callback не должен reenter owner; busy не создаёт второго.

Core tokenMu защищает parent token acquisition/query/Close и EOF cleanup от
конкурирующего Close. Parent borrowed через nativelease.WithParent; same-image,
SID/child elevation и actual pipe-server identity проверяются в прежнем owner.
OpenProcessToken пишет original slot, unreturned acquisition retained; token
Close one-shot, unknown result blocks further cleanup and retires transport.
Частичный token owner не выбрасывается и не становится доказательством absence.

### Остаток и evidence

Далее minimal image seam и concrete pinned-helper bootstrap с separate exact
helper/GUI original owners, role/parent-image/SID/remote-handle/pair compatibility.
Native image/ACL/loaded-section/full loader/creation-time tree custody ещё OPEN;
ranBeforeJob и bootstrap/bridge/native=false сохранены. Signed single-EXE actual
SDK/fresh child authority/scalar async/de-elevation и full PC00…09/old9/SDK/W3b/
interprocess/durability/UX/release/comparative остаются. TUN unavailable/schema2
unsupported, IPC9/Job8/GUI6/R1/U1 source unchanged.

Source10Go/4docs; exact gofmt10+3+2 exit0. Manual5 новых компонентов против empty
originals и5 original/current diffs; type/caller/platform/dependency/Scope/
TryLock/Retire/token/partial I/O/ACK-EOF/SDDL-SID/Free-Close/error review.
14 exact originals и task71 rev508/predecessor chain —
`.artifacts/memory/bootstrap-pipe-*`, named current-source SHA/unchanged inventory.
PLAN/task старые byte prefixes, goal/constraints/tasks0…70/acceptance/decisions/
old checks сохранены. Все acceptance OPEN.

Абсолютный запрет tests/fixtures/build/compile/vet/typecheck/product parser/check/
doctor/install/gen/SDK/helper/product/crypto/native/browser/admin/live/network/
owner values/other-role/chat/subagents. API/ABI/platform/mixed versions/ACL/SQOS/
peer/SID/alloc-return/Free/Close/Retire races/partial/replay/cancel/Goexit/EOF/
parent-crash/resources/OS/packaging OPEN. Нет runtime safety/performance/whole-tree/
GUI readiness/atomicity/latency/RSS PASS.

<a id="pc-v246-image-slots"></a>

## V2-246 — Original image file slots and minimal imagecheck, source/unverified

2026-11-06. PC-01; full PC00…09/old9 и все прежние gates сохранены.

### Сначала фактические consumers

Existing windowsNativeImage/nativeImageFile используется не только helper/self/
recovery/installed-pair pinning, но и installer root/lock/directory/payload/staging
и relative rename/rebind. Прямой перенос только image type с новым private
handle owner изменил бы эти consumers. Сохранён прежний original record; введён
общий acquisition/Close contract и выделен самостоятельный read/format seam.
WFP AppID/path/ACL/authority не перемещены в новый wire/helper package.

В image.openFile и installer.openDirectorySlot обнаружен CreateFile в local
variable с сохранением только после err==nil. Теперь bounded record находится
в исходном owner до вызова, result пишется в его handle до проверки ошибки.
Native APIs не выполнялись; это вывод из старого/нового source ordering.

### Acquisition и cleanup

| Этап | Contract |
| --- | --- |
| beginAcquire | original slot уже опубликован; нет текущего handle или unreturned acquire |
| Native call | acquiring=true до CreateFile/NtCreateFile, output в original field |
| finishAcquire | только после фактического return; InvalidHandle нормализован0 |
| API error | partial handle сохранён, acquireFailed запрещает positive validation |
| Unknown return | acquiring остаётся true, cleanup не выдаёт отсутствие ресурса |
| close | одна native попытка, original value до confirmed success |
| Unknown Close | nonzero value/closeAttempted сохранены, retry/validation запрещены |
| Known close0 | existing rebind может начать новый acquisition в этом slot |

Shared begin/finish покрывает5 существующих sites: image/directory CreateFile,
payload CreateFile, root-lock NtCreateFile и createInstallationObject NtCreateFile.
Staging/created directory проходят последний путь. Shared Close используется
image reverse close, ReleaseDirectory, MoveDirectory descendants, payload и
staging. Outer staging closeErr/ready/closed semantics сохранены. No-create
collision сбрасывает только known returned/no-handle slot, как прежде.

Directory rebind остаётся явным: исходные private FileID/volume identity и
known-close0, existing bounded cleanup и held root lock. Global image retire bit
не добавлен, чтобы не запрещать legitimate close/rebind. Unknown Close сохраняет
nonzero handle и не может быть обнулён новым beginAcquire; неизвестный acquire
сохраняет acquiring. root lock не освобождается поверх failed child cleanup.
Это custody файловых handles, не доказательство rollback всех filesystem effects.
Caller actor/lock serialization остаётся обязательной, новых mutex/потоков нет.

### Minimal imagecheck

`childbootstrap/imagecheck` содержит existing fixed DOS64 + PE264 projection и
bounded synchronous borrowed-handle ReadAt. PE использует standard header types,
не загружает file-declared symbol/string tables. AMD64/PE32+/executable/non-DLL/
subsystem/sections/offset/header/entrypoint checks сохранены; nil reader/size
outside existing application ceiling отклоняются. Это format capability only.

Windows ReadAt сохраняет exact Seek/count/partial/EOF semantics и32KiB native
chunks; caller supplies existing ceiling и original contextError.32MiB helper и
1GiB application policy сохранены, MaxBytes совпадает с current update.MaxPayloadSize.
Source fence добавлен перед финальным positive return. Core wrapper сохраняет
свой reader mutex и private ErrNativeImageUnavailable mapping; source/cancel/EOF
causes проходят обратно. Non-Windows native-read stub fail closed.

Borrowed ReadAt не приобретает и не закрывает handle. Caller обязан удерживать
original owner, сериализовать shared file pointer/Close и проверить identity/ACL.
Его mutex не является новой глобальной inter-reader или read-vs-Close защитой.
Library не подтверждает actual loaded sections, signature, hash authority,
native loader acceptance или custody всего дерева процессов. Величины32KiB/
1GiB — operation/payload bounds, не доказанный RSS/hard latency cap.

### Остаток и evidence

Path/NTFS/ACL/WFP/native image owner пока core-bound. Existing filetxn ACL helper
использует GetSecurityInfo, который скрывает LocalFree outcome; отдельный raw
allocation owner ещё нужен. Полный reusable image lease и concrete pinned-helper
bootstrap с exact helper/GUI owners/role/image/SID/parent/remote-handle/pair
compatibility остаются следующим P0. ranBeforeJob и bootstrap/bridge/native=false
сохранены. Signed single-EXE actual SDK/fresh child authority/scalar async/
de-elevation/full PC00…09/old9/SDK/W3b/interprocess/durability/UX/release/comparative
OPEN; GUI6/R1/U1/IPC9/Job8/TUN unavailable/schema2 unsupported.

Source10Go/4docs; exact gofmt10 exit0, manual3 новых источника против empty
originals и7 original/current diffs; type/caller/acquire-error/Close/rebind/
ID/root-lock/staging error/Scope/PE bounds/EOF/borrowed handle review.
14 exact originals и task71 rev512/predecessor chain —
`.artifacts/memory/bootstrap-image-*`, named source SHA/unchanged prior inventory.
PLAN/task прежние byte prefixes, goal/constraints/tasks0…70/acceptance/decisions/
old checks сохранены. Все acceptance OPEN.

Абсолютный запрет tests/fixtures/build/compile/vet/typecheck/product parser/check/
doctor/install/gen/SDK/helper/product/crypto/native/browser/admin/live/network/
owner values/other-role/chat/subagents. API/ABI/parity/partial/failure/Goexit/
collision/rebind/read-Close concurrency/ACL/NTFS/filesystem/OS/resources/packaging
OPEN. Нет runtime safety/performance/whole-tree/GUI readiness/atomicity/latency PASS.

<a id="pc-v247-owned-acl"></a>

## V2-247 — Retained raw ACL descriptor owners, source/unverified

2026-11-06. PC-01; full PC00…09/old9 и все прежние gates сохранены.

### Raw native owner

Existing filetxn.ValidateWindowsFileACL вызывал pinned windows.GetSecurityInfo,
который копирует descriptor в Go heap и скрывает результат defer LocalFree.
Minimal childbootstrap/fileacl.Slot теперь удерживает raw native output pointer
с первой инструкции query до confirmed Free. GetSecurityInfo W-neutral ABI
сверена с pinned zsyscall:8 args, DWORD status return, OWNER|DACL only, output
SECURITY_DESCRIPTOR** в original field. GetLastError не подменяет DWORD status.

Slot содержит TryLock, pure atomic retire intent, acquiring/freeAttempted/fault
witness. Normal query/policy error допускает новую проверку после confirmed
Free. Returned error также проходит checked release original output. Unreturned
query и unknown Free сохраняют pointer/witness; повтор Free на неопределённом
value запрещён. Callback panic/Goexit не делает
owner новым или successful. Descriptor освобождается в defer до positive return;
ошибка release заменяет потенциальный успех typed cleanup failure. Slot.Close
retire до TryLock, busy возвращает отказ без нового owner/очереди/native call.
Файловый handle Slot не приобретает и не закрывает. Native API Windows-only.

### По одному slot на исходный файл

| Consumer | Где удерживается и как очищается |
| --- | --- |
| Image/installer | nativeImageFile pointer; ACL Close перед original file Close |
| Reader | WindowsRead pointer; original reader остаётся при ошибке ACL retirement |
| Writer | boundedNativeWriteFile pointer, вместе с existing closeErr/parent pins |
| Journal | тот же original writer-file slot, включая observational rename query |
| SDK workspace | workspaceFile pointer, cleanup до освобождения native file |
| Update floor | floorNativeFile pointer, cleanup failure сохраняет original pins |

Record copies переносят pointer к тому же slot, не mutex value. Slot публикуется
до GetSecurityInfo. При cleanup uncertainty нельзя закрыть файл/корень и начать
новый независимый slot. Explicit directory rebind из V2-246 освобождает старый
ACL owner только при confirmed cleanup и known handle0; root lock/private FileID
и relative rename не заменены новой ACL authority.

Public legacy ValidateWindowsFileACL/InspectWindowsFile signatures сохранены;
для совместимости один retained standalone slot с TryLock и sticky fault, без
path/handle map или unbounded queue. Все найденные production consumers перешли
на owned API. Legacy concurrent call может получить busy, не очередь. Selected
untrusted input/output по прежнему private policy не получают trusted-store ACL
requirements и не приобретают local trust. Core image trust теперь импортирует
minimal fileacl напрямую, без filetxn. Typed cleanup/domain causes
сохранены в image/read/floor errors; новый scope/context не создаётся.

### Policy и границы памяти

Сохранён original conservative owner/allow-ACE predicate и trusted current-user/
Admins/SYSTEM/TrustedInstaller set. Deny и inherit-only rules прежние; unknown
active ACE forms отвергаются. Это не full effective-access/privilege evaluator.

До SID/ACE reads проверяются self-relative SD, accepted returned length<=128KiB,
owner SID span, ACL header/declared size, count<=128, each ACE header/body и SID
revision/subauthority span внутри original allocation/ACL/ACE. Pinned xsys ACL
size private, поэтому проверяется Windows8-byte header (size bytes2…3). Native
descriptor validity и native APIs остаются TCB; pointer bounds не sandbox для
malicious kernel или callback. trusted — pure transient SID predicate, не может
сохранять/менять pointers или reenter slot.128KiB — accepted descriptor limit,
не предел уже сделанной OS allocation, total-memory/RSS или native-call latency.

### Что ещё не закрыто

Full native image/path/NTFS/WFP owner ещё core-bound. Identity SID factories,
другие security/native wrappers и весь import/loader graph требуют отдельной
ownership приёмки; это не repository-wide leak fix. Далее reusable image seam
и concrete pinned-helper bootstrap с exact helper/GUI owners, role/image/SID/
parent/remote-handle/pair compatibility. GetSecurityInfo policy outcome не
attestation loaded sections/signature/protection/whole-tree. ranBeforeJob и
bootstrap/bridge/native=false сохранены; TUN unavailable/schema2 unsupported.
Signed single-EXE actual SDK/fresh child authority/scalar async/de-elevation/
full PC00…09/old9/SDK/W3b/interprocess/durability/UX/release/comparative OPEN.

### Evidence

Source12Go/4docs; exact gofmt12+3 exit0, manual3 новых источника против empty
originals и9 original/current diffs; raw ABI, descriptor/ACL/SID bounds, legacy
policy comparison, caller/type/ownership/retire/TryLock/defer/Free/Close/rebind
review.16 originals/task71 rev516/predecessor chain —
`.artifacts/memory/bootstrap-acl-*`, named SHA/unchanged prior source inventory.
PLAN/task прежние byte prefixes, goal/constraints/tasks0…70/acceptance/decisions/
old checks сохранены. Все acceptance OPEN.

Абсолютный запрет tests/fixtures/build/compile/vet/typecheck/product parser/check/
doctor/install/gen/SDK/helper/product/crypto/native/browser/admin/live/network/
owner values/other-role/chat/subagents. API/ABI/self-relative descriptor/ACL
corpus/partial error/Free/retire races/callback panic-Goexit/caller serialization/
legacy contention/rebind/OS/resources/packaging OPEN. Нет runtime safety/
performance/whole-tree/GUI readiness/atomicity/latency/RSS PASS.

<a id="pc-v248-image-inspection"></a>

## V2-248 — Shared original-handle image inspection, source/unverified

2026-11-06. PC-01; full PC00…09/old9 и прежние gates сохранены.

### Reuse и границы

Existing imagecheck расширен двумя Windows-only source files; новая параллельная
image policy не создана. Из core перенесены fixed-local component/ancestor plan,
NTFS/persistent-ACL check, original final path/FileID/nonreparse/single-link/ACL
inspection и reported process-path comparison. Прямые зависимости — stdlib,
existing childbootstrap scope/nativelease/fileacl и pinned xsys0.44; core/Wails/
sing-box/wf/SDK в этих источниках отсутствуют. Это source dependency inspection,
не доказательство полного executable/import/loader/init closure.

Core wrappers оставляют original nativeImageFile/windowsNativeImage records,
CreateFile/NtCreate output custody, acquire/failed/closeAttempted flags, reverse
Close, installer known-close0 rebind/private FileID/root lock и WFP AppID checks.
Installer records по прежнему изменяемые; форматная проверка не подменяет их
владельца. Verifier/recovery/UAC используют тот же process-path adapter; recovery
больше не содержит второго native query. Existing trust.Validate signature и
current-user/Admins/SYSTEM/TrustedInstaller predicate сохранены.

### Контракты заимствования

| Операция | Правило |
| --- | --- |
| Component/DirectoryPaths | Canonical fixed-local path<=2048 bytes, at most64 ancestor names; actual drive type checked, handles not opened |
| NTFS/FilePath | Только original held handle; persistent ACLs и bounded2049-unit final-path buffer |
| FileIdentity | Metadata пишет прямо в original snapshot, не стирает его при отказе до GetFileInformation |
| ACL | Caller retains original per-file slot before query and retires before file Close |
| ProcessPathMatches | Original process or pseudo-self, bounded query, existing source fence before/after; no PID reopen |
| Ошибки | Ordinary leaf ErrImage maps to exact ErrNativeImageUnavailable; ACL cleanup/busy keeps ErrHelperCleanup and cause |

Directory shape теперь проверяется до current-SID factory, сохраняя те же имена
и глубину; это намеренное изменение порядка, не утверждение полной parity.
Process wrapper не создаёт новый ctx/таймер и сохраняет original cancellation/
deadline/source cause. Nil/shape/size отказ не разрешает native use. Metadata с
ошибкой не становится authority. Handle representation не attestation object
type, granted rights, parent identity или actual Job membership.

FileIdentity не открывает/не закрывает файл, но использует retained fileacl.Slot
для native descriptor query/checked Free. Остальные операции не приобретают
native resource. Caller serialization и lifetime остаются обязательными; новый
borrowed API не делает shared handle/Close race безопасным автоматически.
Path/FileID/ACL/NTFS не заменяют signature/loaded-section/protected-egress proof,
не изолируют trusted owner/admin/kernel/preexisting writable mappings.

### Остаток и evidence

Full reusable original file/identity owner и SID factories ещё core-bound;
concrete pinned-helper bootstrap/exact helper+GUI owners/role/image/SID/parent/
remote-handle/pair compatibility остаются следующим шагом. Новый executable
mode/CLI role/pin/protocol не добавлен. Full import/loader/creation-time custody,
signed single-EXE actual SDK/fresh child authority/scalar async/de-elevation/
full PC00…09/old9/SDK/W3b/interprocess/durability/UX/release/comparative OPEN.
ranBeforeJob и bootstrap/bridge/native=false сохранены; GUI6/R1/U1/IPC9/Job8,
TUN unavailable/schema2 unsupported.

Source5Go/4docs; gofmt5+3exit0, manual2 новых sources против empty originals и
3 original/current diffs. Original component/path/NTFS/FileID/ACL policy и native
signatures сверены по исходникам; caller/type/snapshot/error/scope/pseudo-self/
acquire-Close-rebind-WFP boundaries просмотрены вручную.9 exact originals и
task71 rev520/predecessor chain — `.artifacts/memory/bootstrap-image-owner-*`.
Named SHA, untouched prior source inventory, PLAN/task byte prefixes и full
goal/constraints/tasks0…70/acceptance/decisions/old checks сохраняются.

Абсолютный запрет tests/fixtures/build/compile/vet/typecheck/product parser/check/
doctor/install/gen/SDK/helper/product/crypto/native/browser/admin/live/network/
owner values/other-role/chat/subagents. ABI/parity/path and ACL corpus/pseudo-self/
scope/nil/partial-error/retire races/panic-Goexit/caller serialization/OS/resources/
packaging OPEN. Все acceptance OPEN; runtime safety/performance/whole-tree/
GUI readiness/atomicity/latency/RSS PASS отсутствуют.

<a id="pc-v249-file-owner"></a>

## V2-249 — Original file and ACL lease, source/unverified

2026-11-06. PC-01; full PC00…09/old9 и прежние gates сохранены.

### Original ownership

Existing nativeImageFile acquisition/Close/ACL state выделен в minimal
childbootstrap/filelease.Lease. Private fields — original native handle,
acquiring/acquireFailed/closeAttempted, retained fileacl.Slot и original native
IO_STATUS_BLOCK. Core record хранит один pointer до первой acquisition; path,
directory и private FileID остаются в original record. Copies и slice relocation
переносят тот же pointer, а не независимую копию native ownership state.

Lease Open сохраняет OPEN_EXISTING Win32 path; OpenRelative сохраняет actual
parent/name/access/attributes/share/disposition/options и OBJ_CASE_INSENSITIVE.
NtCreateFile пишет handle и IO_STATUS прямо в original owner. Information — raw
creation witness, не NTSTATUS и не разрешение use/delete. Existing FILE_CREATE
collision и FILE_OPEN_IF root-lock decisions остаются у installer.

| Acquisition | Consumer policy сохранена |
| --- | --- |
| Image parent/helper/ancestors | Core path/ACL/pin/PE/WFP checks, original tree lifetime |
| Root lock | Original parent, FILE_OPEN_IF/no truncate, zero size, fail-fast LockFileEx |
| New directory/staging | FILE_CREATE, original created/no-create collision classification |
| Directory rebind | Only known handle0/checked ACL retirement, same private FileID/root lock |
| Payload | Existing deny-write/delete-sharing and allowed payload/hash/size checks |

Acquiring остаётся true при unreturned call/panic/Goexit. Native error сохраняет
partial handle/IO_STATUS и acquireFailed. Unknown Close не обнуляет handle и не
повторяется. ACL Close идёт перед file Close; unknown Free сохраняет original
file/root pins. Только confirmed file Close очищает handle. Explicit rebind может
повторно открыть тот же lease после known handle0 и confirmed ACL retirement;
глобальный irreversible retire bit не ломает directory rename reconciliation.

### Borrowing и API boundary

Все image/verifier/recovery/pair/installer read/write/flush/rename/lock consumers
перешли на original lease. HandleValue возвращает прежнее raw значение, включая
uncertain результат, для существующего native I/O и reconciliation. Это trusted
internal API: scalar не передаёт ownership и не даёт отдельную authority закрывать
handle или игнорировать original source/identity/lifetime checks. Нового Wails/
CLI entrypoint не добавлено. Ready сообщает лишь acquisition state.

Прежний caller/actor сериализует Open/Inspect/borrowed I/O/Close/rebind; Lease не
добавляет mutex вокруг внешних Win32 операций. ReaderAt mutex по прежнему отвечает
за его file pointer. Никакого нового concurrent I/O-Close или attacker-in-process
enforcement proof нет. In-process trusted owner/admin/kernel/mapped-view границы
не расширены. Path/FileID/trust/tree/root-lock/WFP policy остаётся у core.

Inspect переиспользует imagecheck и original per-file ACL slot. Common typed
ErrCleanup сохраняется через nativeImageInspectionError как ErrHelperCleanup;
native status causes сохраняются для collision/sharing classification. Public
application API, GUI6/R1/U1/IPC9, helper pin/name/role/packaging не менялись.
Private nativeImageFile representation изменена; compile/private consumer parity
остаётся отдельным непроверенным gate. Native Lease API Windows-only.

### Evidence и остаток

Source12Go/4docs; gofmt12+1exit0, manual2 new sources против empty originals и
10 original/current diffs. Пять acquisition sites, all found original type/handle
consumers, exact native arguments/NT status-vs-Information, original output,
partial/Close/ACL/rebind/collision/root-lock/staging flags и error mapping
просмотрены вручную;16 originals/task71 rev524/predecessor chain в
`.artifacts/memory/bootstrap-file-owner-*`. Named SHA/untouched prior sources,
PLAN/task byte prefixes и goal/constraints/tasks0…70/acceptance/decisions/old
checks сохраняются. Это source evidence, не native execution.

Далее SID/trust factory и concrete minimal image/helper bootstrap с exact helper/
GUI owners/role/image/SID/parent/remote-handle/pair compatibility. Whole tree,
signature/loaded-section/creation-time/loader/rights/cleanup/performance/OS/
packaging acceptance OPEN. Signed single-EXE actual SDK/fresh child authority/
scalar async/de-elevation/full PC00…09/old9/SDK/W3b/interprocess/durability/UX/
release/comparative OPEN. ranBeforeJob и bootstrap/bridge/native=false прежние;
GUI6/R1/U1/IPC9/Job8/TUN unavailable/schema2 unsupported.

Абсолютный запрет tests/fixtures/build/compile/vet/typecheck/product parser/check/
doctor/install/gen/SDK/helper/product/crypto/native/browser/admin/live/network/
owner values/other-role/chat/subagents. ABI/parity/private-type/collision/partial/
panic-Goexit/borrowed I-O-Close serialization/rebind/ACL/OS/resources/packaging
OPEN. Все acceptance OPEN; runtime safety/whole-tree/GUI readiness PASS отсутствуют.

<a id="pc-v250-file-identity"></a>

## V2-250 — Shared bounded local ACL identity, source/unverified

2026-11-06. PC-01; full PC00…09/old9 и прежние gates сохранены.

### Query и Go-owned snapshots

Existing fileacl расширен CurrentUser/CurrentTrust/Trust без зависимости от core.
Pinned GetTokenUser wrapper использует grow/retry loop, StringToSid возвращает
Go-copy через native allocation и defer LocalFree, чей результат скрыт. Эти два
пути заменены в local ACL factories; repository-wide wrapper audit не заявлен.

CurrentUser сохраняет прежний GetCurrentProcessToken selector, не effective/
impersonated-thread token. Псевдохэндл не требует Close. Один GetTokenInformation
TokenUser пишет в fixed128-byte Go buffer с typed windows.Tokenuser header,
alignment/KeepAlive и private trailing SID bytes. Returned length проверяется,
SID должен лежать после header внутри returned span; reused sidWithin проверяет
revision1/count<=15/полный SID span до чтения и native IsValid. Только затем SID
копируется в отдельный aligned Go68-byte buffer. Недостаточный/неожиданный buffer
отвергается, дополнительный query/allocation resize не запускается.

Fixed installer SID использует прежние S-1-5 authority и6 subauthorities:
80,956008885,3418522649,1831038044,1853292631,2271478464. Кодирование header и
little-endian DWORDs происходит в Go storage; native IsValid проверяет результат.
Это private fixed encoder, не общий parser профилей/ввода и не account lookup.
StringToSid/ConvertStringSidToSid/LocalFree в этом factory не вызываются. SID
storage — Go-owned, нет нового native allocation owner или cleanup uncertainty.
Константа TrustedInstallerSID сохранена как readable identity reference.

Trust держит private user/installer pointers, безопасно копируется как value и
не публикует mutable pointers этих snapshots. CurrentUser возвращает отдельную
свою Go-owned копию. Global cache/registry/native finalizer не добавлены. Zero
Trust отклоняет любой SID, включая builtin principals; подготовленный Trust
сохраняет прежний current-user/Admins/SYSTEM/TrustedInstaller allow predicate.
Это local ACL trust set, не effective-access, TUF/profile/signing authority.

### Integration

| Consumer | Source change |
| --- | --- |
| Core image/installer | Existing nativeImageTrust methods delegate to fileacl.Trust |
| Reader/writer | Original owners store Trust value, selected untrusted I/O skips local ACL policy as before |
| SDK workspace/update floor | Shared factory/predicate before native file acquisition, original domain errors |
| Control pipe | Shared bounded primary user SID; original ConvertSidToStringSid/SDDL allocation owners and scopes retained |

Five duplicated factories/predicates сокращены. Original file/ACL/Job/parent/
SDDL/SID-text owners, close semantics, path/private-ID/root lock/WFP и caller
serialization не изменены. Private owner representation меняется; API/private
consumer parity остаётся unverified. New native identity APIs Windows-only.
Новых helper roles/arguments/pins/SDK versions/protocols/Ready не добавлено.

### Evidence и остаток

Source9Go/4docs; gofmt9+1exit0, manual2 новых sources против empty originals и
7 original/current diffs. Pinned Tokenuser/SIDAndAttributes/GetTokenInformation/
pseudo-token/getInfo/StringToSid source, existing SID bounds/predicate, typed
header/Go lifetime/SID prefix/6 numeric constants/length/alignment/caller/domain
errors/selected-policy/pipe-owner references просмотрены вручную.13 originals и
task71 rev528/predecessor chain — `.artifacts/memory/bootstrap-identity-*`;
named SHA/untouched prior sources/PLAN-task byte prefixes и full goal/constraints/
tasks0…70/acceptance/decisions/old checks сохранены.

Fixed128/68-byte buffers — bounds этого source path, не whole-process allocation/
RSS/latency guarantee. Native TokenUser/IsValid/Equals/IsWellKnown остаются TCB.
ABI/SID encoding/GC-lifetime/private-type/primary-vs-effective-token/parity/
partial/OS/resources/packaging OPEN. Далее concrete minimal image/helper
orchestration с exact helper+GUI/role/image/SID/parent/remote-handle/pair
compatibility; full import/init/loader/creation-time/whole-tree custody OPEN.
Full PC00…09/old9/signed single-EXE actual SDK/fresh child authority/scalar async/
de-elevation/SDK/W3b/interprocess/durability/UX/release/comparative OPEN.
ranBeforeJob сохранён; bootstrap/bridge/native=false, GUI6/R1/U1/IPC9/Job8/TUN
unavailable/schema2 unsupported. Native identity source не attestation GUI Ready.

Абсолютный запрет tests/fixtures/build/compile/vet/typecheck/product parser/check/
doctor/install/gen/SDK/helper/product/crypto/native/browser/admin/live/network/
owner values/other-role/chat/subagents. Все acceptance OPEN; runtime safety/
performance/whole-tree/GUI readiness/atomicity/latency/RSS PASS отсутствуют.

<a id="pc-v251-image-pair"></a>

## V2-251 — Concrete minimal App/helper image pair, source/unverified

2026-11-06. PC-01; full PC00…09/old9 и прежние gates сохранены.

### Реальное подключение

Native helper image и installed-pair preparation теперь используют minimal
childbootstrap/imagelease.Pair через core.windowsNativePair. Старые helper-only
open/hash/WFP/process methods удалены из mutable windowsNativeImage; installer,
verifier и recovery сохраняют свои исходные records/rebind/private-ID/root-lock.
Общая SameSnapshot/SameFileID policy находится в imagecheck; zero ID не совпадает.
Это actual current call graph, не только неиспользуемый интерфейс.

Pair владеет fixed66 original records: <=64 ancestors, exact App и helper.
Per-file Lease публикуется до Open, pointer/ACL/identity/trust сохраняются при
partial failure/panic/Goexit. Подготовка one-attempt; reverse Close останавливается
при первой неопределённой file/ACL retirement, сохраняя предков. Handle getters
не экспортируются. Paths возвращает только immutable имена без authority.

### Границы и сериализация

| Работа | Source contract |
| --- | --- |
| Prepare | Exact canonical sibling paths, fixed local NTFS/ACL/nonreparse/single-link/IDs, distinct App/helper objects |
| Helper bytes | Original handle SHA match, fixed PE projection, <=32MiB, synchronous32KiB payloads |
| App pin | Caller-authorized digest/actual App+helper sizes, App<=1GiB; no implicit SDK/signature authority |
| Validate | Original snapshots/ACLs and same file size/time/link policy; directory child changes allowed |
| WithPaths | Trusted synchronous nonreentrant inspector holds original files through WFP/process checks |
| Retirement | Atomic intent before TryLock, scope/retire checked between read chunks and before/after callbacks |
| Close | Caller must first settle original Job/process, Pair then closes held files in reverse; unknown stays owned |

Pair mutex удерживается до фактического возврата synchronous reads/inspectors.
Private ReaderAt не выходит наружу; нового I/O goroutine/timer/overlapped waiter
нет. Положительные операции при contention/retire/fault отвергаются; partial
owner нельзя заменить. Callback panic/Goexit помечает retained logical fault;
native file Close uncertainty остаётся в original filelease. Immutable Pair
не rebindится; mutable installer использует прежний отдельный known-close0 path.

WFP AppID и process-role checks остаются в core, но выполняются в scoped
WithPaths/PinApplication callbacks, пока Pair держит file custody. PinApplication
сохраняет pre/post snapshot+WFP checks вокруг одного hash; core не выполняет
дополнительные дублирующиеся обходы. Caller/Job ownership и assigned-host checks
по прежнему обязательны. Core fixed helper name и32MiB/1GiB caps сравниваются с
minimal contract; drift отвергается. Public nativeImageLease signatures прежние,
private concrete type меняется. Неправильный размер теперь отвергается до query.

Прямые dependencies minimal owner — stdlib, existing childbootstrap/scope,
fileacl/filelease/imagecheck и pinned xsys. Core/Wails/wf/SDK/sing-box внутри
нового owner не импортируются; WFP callback принадлежит core. Это source imports,
не доказательство полного executable/init/loader closure.

### Evidence и остаток

Source8Go/4docs; gofmt8+4+4+2exit0, manual4 новых sources против empty originals и
4 original/current diffs. Original helper/installed-pair/type/size/name/ACL/NTFS/
snapshot/hash/PE/callback/error/scope/lock/retire/partial-Close paths просмотрены
вручную.12 originals/task71 rev532/predecessor chain —
`.artifacts/memory/bootstrap-tree-*`; named SHA/untouched prior sources/PLAN-task
byte prefixes и goal/constraints/tasks0…70/acceptance/decisions/old checks kept.

Limits:66 records/32KiB reads/32MiB helper/1GiB App не RSS или hard latency cap.
Native I/O/ACL/WFP могут задержаться; callback lifetime и process/Job-before-file
cleanup требуют native приёмки. Hash/path/ACL не loaded-section/signature,
whole-tree/protected-egress proof и не изоляция trusted owner/admin/kernel/
preexisting writable mappings. Сокращение source обходов не измерение скорости.

Конкретный новый fixed-role helper executable bootstrap, remote handles/purpose/
exact helper+GUI role/pair compatibility и full init-loader/creation-time custody
ещё не добавлены. GUI6/R1/U1/IPC9/Job8 unchanged; ranBeforeJob сохранён и все
bootstrap/bridge/native gates=false, TUN unavailable/schema2 unsupported.
Full PC00…09/old9/signed single-EXE actual SDK/fresh child authority/scalar async/
de-elevation/SDK/W3b/interprocess/durability/UX/release/comparative OPEN.

Абсолютный запрет tests/fixtures/build/compile/vet/typecheck/product parser/check/
doctor/install/gen/SDK/helper/product/crypto/native/browser/admin/live/network/
owner values/other-role/chat/subagents. API/private-type/ABI/pin/PE/partial/retire/
callback/panic-Goexit/scope/caller-Job/OS/resources/packaging OPEN. Все acceptance
OPEN; runtime safety/performance/whole-tree/GUI readiness PASS отсутствуют.

<a id="pc-v252-published-custody"></a>

## V2-252 — Published original images and exact UAC user, source/unverified

2026-11-06. PC-01; full PC00…09/old9 и прежние gates сохранены.

### Конкретные source defects

V250 заменил nativeImageTrust.user/installer на private fileacl.Trust, но два
потребителя остались в parent/child UAC validation. Их обращения к отсутствующему
полю исправлены через MatchesUser: строго actual primary-user SID, без допуска
всего trusted Admins/SYSTEM/installer union. Zero/unprepared Trust и nil SID не
совпадают. UserSID заимствует existing TOKEN_QUERY token в original owner и
использует тот же typed128-byte/Go68-byte bounded query/copy. Новый token не
открывается и не закрывается; existing elevation/path/pipe/Job checks и token
acquire/checked-Close state сохранены. Это source correction без compile proof.

В helper/TUN/delivery factories native acquisition начиналась до присваивания
результата lane.image/ownsNew. Pure allocate теперь выполняется отдельно: слот
и cleanup flag заполняются до prepareCurrent/prepare, в том числе при unreturned
call/panic/Goexit. Arbitrary openImage callback в private helper options заменён
validated pin value; fixed system helper path и keepImage policy прежние.
Пустой/невалидный native pin не создаёт bare-exec fallback.

### Retained slots

RetainInstalledPair/RetainRecoveryImage принимают адрес original caller slot,
сразу записывают pure receipt и только потом готовят native image. Nested image
pointer также сохраняется до in-place Prepare. Nonnil slot не заменяется и не
закрывается этой API. Readiness выставляется только после успешной подготовки/
pin; ошибка сохраняет exact receipt для checked cleanup. Existing public Open
return APIs сохранены; их локальный named return не доказывает reachability
owner у внешнего вызывающего при unreturned factory.

| Consumer | Где original image сохраняется до Prepare |
| --- | --- |
| Native helper capture | helper lane.image + ownsNew |
| TUN observation | protectedHelper.image + ownsNew |
| Delivery verifier | delivery lane.image + ownsNew, existing pure verifier constructor |
| App candidate/recovery | App.pairImage через RetainInstalledPair |
| Accepted startup | original global acceptedStartupImage |
| Code acceptance image | original acceptance image.pair (outer acceptance factory remains separate) |
| Recovery companion | App.recoveryEntrypointImage через RetainRecoveryImage |
| Update GUI native session | one-use original global before native adoption |

Actual SDK/candidate/version/hash/purpose/root-lock/prospective-image guards не
перенесены в generic image API и не ослаблены. Original process/Job-before-file
retirement и source contexts прежние. Non-Windows constructors дают nil/отказ;
native image API не выдаёт ложный success. Public GUI frames/arguments/helper
pin/name/resource caps/SDK/packaging не изменены; startup session не переиспользуется.

### Source review и границы

Source21Go/4docs; gofmt21+1exit0, manual21 original/current diffs. Whole-Windows
production reference search (без test sources) подтвердил отсутствие stale
trust.user/installer и openImage consumers; legacy Open image factories остались
definitions/compatibility wrappers. Interface method sets/platform stubs, slot
preconditions/assignment-before-call, cleanup flags/readiness/SID exactness/source
contexts/callback handoff просмотрены вручную.25 originals/task71 rev536 и chain
в `.artifacts/memory/bootstrap-custody-*`; named SHA/untouched prior sources,
PLAN-task byte prefixes/full goal/constraints/tasks0…70/acceptance/decisions kept.

56 прежних NOT_RUN checks сохранены точно. Для новой source-only итерации нет
новой исполненной проверки и нет waiver: unverified scope/evidence записаны
здесь и в next/task71. Сам record NOT_RUN не является runtime evidence.

Урок source миграции: consumers private fields нужно искать по всей платформе,
включая обе UAC sides; equality одного user нельзя заменять trusted-principal
predicate. Проверка только provider/factory directories пропустила эти два места.

Outer UpdateCodeAcceptance/process/root factories и полная end-to-end publication
ещё требуют отдельного аудита. Immediate image slot исправлен, но это не whole
process-tree/Goexit/cleanup acceptance. Actual fixed-role executable helper
bootstrap/exact helper+GUI role/pair/remote handles/init-loader/creation-time
custody остаются следующим шагом. Full PC00…09/old9/signed single-EXE actual SDK/
fresh child authority/scalar async/de-elevation/SDK/W3b/interprocess/durability/
UX/release/comparative OPEN; ranBeforeJob сохранён и bootstrap/bridge/native=false,
GUI6/R1/U1/IPC9/Job8/TUN unavailable/schema2 unsupported.

Абсолютный запрет tests/fixtures/build/compile/vet/typecheck/product parser/check/
doctor/install/gen/SDK/helper/product/crypto/native/browser/admin/live/network/
owner values/other-role/chat/subagents. API/private-type/ABI/SID/nil/partial/
Goexit/Close/GC/source/OS/resources/packaging OPEN; все acceptance OPEN.
Компиляция/работоспособность/скорость/native protection этим не подтверждены.

<a id="pc-v253-process-publication"></a>

## V2-253 — Process and outer-owner publication before acquisition, source/unverified

2026-11-06. PC-01; full PC00…09/old9 и все прежние gates сохранены.

### Исправленный порядок владения

Предыдущий spawner создавал original host перед CreateProcess, но внешний
Engine/lane/GUI получал process только после возврата factory. При невозврате
внешний cleanup не имел этого owner. Теперь pure newOwnedProcess создаёт только
Go host/I-O wrappers/done; вызывающий сохраняет его до in-place spawn.
spawnAttempted устанавливается перед подготовкой и никогда не сбрасывается.
Nil, retired/unusable или уже использованный process не допускается к spawn.

| Потребитель | Публикация до native preparation |
| --- | --- |
| Engine | Engine.process; instance резервируется до spawnEngine |
| Helper capture | lane.owner и ownsNew |
| TUN observation | protectedHelper.owner и ownsNew |
| Delivery verifier | delivery lane.owner и ownsNew |
| GUI transfer | acceptance lane.owner и gui.process |

Engine отклоняет замену existing process. startIsolatedLocked передаёт исходный
operation ctx вместо Background; cancellation теперь может остановить подготовку
до создания/Resume. Это изменение поведения отдельно остаётся unverified.
Lifetime original Job после запуска не привязан к отмене operation ctx; Stop/
retirement используют прежний original reaper. Instance0 при переполнении
отклоняется до spawn; это не новый proof глобальной неизменности sequence.

### Внешние acceptance/root owners

RetainUpdateCodeAcceptance публикует original App.pairAcceptance до guard и
image preparation. RetainInstallationRoot публикует App.pairRoot до native
работы; pure nested native root сохраняется перед open. Nonnil caller slot
отклоняется, не заменяется и не закрывается этим API. Ошибка сохраняет original
частичное состояние для прежнего checked cleanup. App pair-update, recovered
launch и recovery entrypoint используют новые Retain APIs. Native root constructor
сохраняет прежние bounded directory/file records и lockIndex=-1; non-Windows
constructor возвращает nil/отказ. Native lock/path/ACL/SDK authority не менялись.

Public OpenUpdateCodeAcceptance/OpenInstallationRoot и private return spawners
остались compatibility wrappers/stubs. Production reference search нашёл только
их определения/внутренние wrappers; актуальные потребители публикуют slots до
подготовки. Сам return API не гарантирует внешнюю reachability при невозврате.

Original suspended CreateProcess/ProcessInformation, inherited-handle allowlist,
Job assignment before Resume, beforeResume image/source/remote-handle checks,
resource budgets/minimal environment и checked preparation/retirement сохранены.
Engine Job8 и independent lifetime не заменены helper policy. Это изменение
owner publication, не разрешение native traffic или новая bootstrap authority.

### Review и незакрытая приёмка

Source15Go/4docs; gofmt15exit0. Вручную просмотрены15 original/current diffs,
actual production callers, method sets обоих platform branches, scope/error
variables, slot/cleanup flags и original retirement.19 точных originals и task71
rev540 archived в .artifacts/memory/bootstrap-process-owner-*; named SHA,
PLAN/task original byte prefixes и полный ledger сохраняются отдельно.
56 прежних NOT_RUN checks неизменны; новая source-only итерация не добавляет
фиктивного run/waiver. Все100 acceptance остаются OPEN.

Публикация этих owners не доказывает end-to-end Goexit/ABI/Close/acquisition
semantics. Нижние os.Pipe/native wrappers и полная process-tree custody требуют
отдельной проверки; checked Close не подтверждён native исполнением. Actual
fixed-role executable bootstrap/exact helper-GUI-role-pair/remote handles/full
init-loader/creation-time custody остаются следующим implementation slice.
Full PC00…09/old9/signed single-EXE/actual SDK/fresh child authority/scalar async/
de-elevation/W3b/interprocess/durability/UX/release/comparative OPEN.
bootstrap/bridge/native=false, ranBeforeJob retained; GUI6/R1/U1/IPC9/Job8,
TUN unavailable/schema2 unsupported. Компиляция, защита и скорость не подтверждены.

Абсолютный запрет tests/fixtures/build/compile/vet/typecheck/product parser/check/
doctor/install/gen/SDK/helper/product/crypto/native/browser/admin/live/network/
owner values/other-role/chat/subagents сохраняется. Только source/docs/собственная
память, разрешённый gofmt и ручной просмотр исходников/байтов.

<a id="pc-v254-native-retirement"></a>

## V2-254 — Original spawner one-attempt native retirement, source/unverified

2026-11-06. PC-01; полный PC00…09/old9 и прежние gates сохранены.

### Почему следующий helper пока не получил новый spawner

Existing snowden-native-probe imports netprobe/stdlib/platform wrappers, а
core-spawner связан с product engine. Перед его reusable boundary обнаружены
два конкретных source defects: CloseHandle повторялся с retained numeric handle
после неизвестного результата, а attribute cleanup повторял LocalFree. Поздний
успех такого retry нельзя использовать как original-resource cleanup proof.
Новая helper role, CLI и упаковка этим срезом не вводятся.

### Изменённый порядок

- Thread/process/Job и3 inherited duplicates имеют отдельные original attempted
  flags. Они выставляются до CloseHandle и не сбрасываются. Confirmed zero slot
  пропускается; ненулевой slot после попытки больше не передаётся native Close.
- При неизвестном close обход останавливается: оставшийся original набор
  сохраняется. Host не закрывает root/Job при неизвестном preparation cleanup.
- LocalFree и Pinner.Unpin получили отдельные one-attempt witnesses. Native
  allocation/list обнуляются только после фактического successful LocalFree.
  Interrupted Delete уже имел собственный witness; он сохранён.
- Attribute initialize теперь имеет один attempt и preparing witness. Только
  реальный возврат preparation снимает preparing. Panic/Goexit оставляет его
  установленным, и Close удерживает весь original набор. Pinned flag установлен
  до Pin, чтобы partial attempt не потерялся. Сериализация остаётся у caller.
- Engine retirement comment исправлен: повторный обход пропускает confirmed
  closed slots, но не повторяет unknown native closes. Drained не даёт cleanup
  authority и не снимает защёлкнутое неопределённое состояние.

Original CreateProcess/suspended/Job assignment/Resume, image/source guards,
resource limits, private frames/roles/SDK и original-file retirement прежние.
Нет нового grace period, PID reopen, bare-exec fallback или разрешения трафика.
Удержание неизвестного owner может потребовать terminal OS exit; доступность
не восстанавливается ложным successful retry. Это source policy, не OS proof.

### Минимальный reusable seam

Actual attribute owner/native-close implementation перенесён из core в
childbootstrap/spawnprep; duplicate implementation не остаётся. Direct imports
нового пакета — stdlib, existing nativelease и pinned x/sys; core/Wails/engine
не импортируются. Это ручной source dependency review, не full loader proof.
Original core host содержит spawnprep.Attributes до Initialize. WithStartup
заимствует list только для одного synchronous callback; public list getter нет.
Actual CreateProcess пишет original ProcessInformation в host; прежние standard
handles/SHOWWINDOW/HIDE, flags/environment и Job-before-Resume order сохранены.
Невозврат callback оставляет borrowing witness, который блокирует Delete/Unpin/
Free. Close снимает ready и запрещает новую Initialize/use. Caller serialization
остаётся обязательной, callback не должен сохранять pointer или reenter owner.
CloseOnce переиспользует original caller slots и existing valid-handle predicate.
Это native preparation seam; полный process/Job/reaper и helper orchestration
не объявляются выделенными или готовыми.

### Evidence и остаток

Source7Go/4docs, gofmt3+7exit0. Вручную просмотрены3 новых файла и diff4
существующих, actual constructors/callers/cleanup ordering и byte identities.8 точных originals и3 записи отсутствующих new files, task71 rev544 и
predecessor chain в .artifacts/memory/bootstrap-retirement-*. PLAN/task original
byte prefixes и весь прежний ledger сохраняются;56 NOT_RUN checks без изменений,
100 acceptance OPEN, новой выполненной проверки или waiver нет.

Не закрыты lower-level os.Pipe/LocalAlloc acquisition, whole process-tree custody,
ABI/partial/panic/Goexit/Close/GC/native resource semantics и OS/packaging gates.
Следующая реализация — concrete minimal reusable process seam, затем fixed-role
helper executable/exact helper-GUI-role-pair/remote handles/init-loader/
creation-time. Bootstrap/bridge/native=false и ranBeforeJob сохранены;
GUI6/R1/U1/IPC9/Job8/TUN unavailable/schema2 unsupported. Full PC00…09/old9/
signed single-EXE/SDK/fresh child authority/scalar async/de-elevation/W3b/
interprocess/durability/UX/release/comparative OPEN.

Абсолютный запрет tests/fixtures/build/compile/vet/typecheck/product parser/check/
doctor/install/gen/SDK/helper/product/crypto/native/browser/admin/live/network/
owner values/other-role/chat/subagents сохранён. Компиляция, работоспособность,
защита и производительность не подтверждены.

<a id="pc-v255-native-process"></a>

## V2-255 — Minimal original Process/Job custody, source/unverified

2026-11-06. PC-01; полный PC00…09/old9 и все прежние gates сохранены.

### Подключён существующий production spawner

Native Process/Job lifetime и accounting перенесены из windowsProcessHost в
existing childbootstrap/spawnprep. Core host содержит original zero Process
до acquisition; нет второго процесса, копии owner или PID-based reopen.
CreateProcess пишет непосредственно в private original ProcessInformation.
Отдельный adopt/copy/zero шаг удалён. Невозврат create сохраняет creating и
считается неизвестным созданием, а не доказательством отсутствующего child.

| Операция | Source contract |
| --- | --- |
| OpenJob | Один unnamed/non-inheritable native attempt; success witness до снятия opening |
| CreateSuspended | Один trusted synchronous adapter, direct original output; actual core сохраняет CREATE_SUSPENDED |
| Assign/Resume | Отдельные one-attempt/pending witnesses, valid original process/thread; Assign до Resume |
| WithJob/WithAssigned | Synchronous nonreentrant borrowed handles, без raw getters; unreturned callback остаётся restrictive |
| ReserveWait/Wait | Резервирование до goroutine, один CAS-entry; finished только после actual native return |
| Close | Pending operations/borrow/Wait/unknown Close блокируют release; root exit и empty Job проверяются перед one-attempt handle release |

Native Close recheck — дополнительное изменение поведения: retained root должен
быть signaled, Job пуст, reserved Wait физически returned. Новый drained flag
относится только к этим native facts; I/O, image, attributes и SDK acceptance
он не подтверждает. Actual core reaper сохраняет прежние completion channels,
самостоятельный cleanup deadline, process-before-image retirement и запрет
replacement при неизвестном исходе. Новый owner не задаёт native hard deadline.

### Миграция потребителей

Core spawn/host/resource checks используют actual original native owner.
Pair/recovery/verifier image checks и GUI remote-handle preparation работают
в WithAssigned. TUN transfer заимствует source и target original owners, проверяя
прежние process stamp, fixed helper Job budget/accounting, distinct PID и
post-duplicate source fence. Remote value остаётся в target process; parent не
пытается закрыть его в своей handle table.

Resource policy остаётся у платформы: прежние Job8/helper/SDK/GUI limits и точные
query/accounting predicates сохранены. Unchanged policy helper используется
также отдельно удерживаемым running UAC request. Последний не преобразован в
suspended owner: original ShellExecute hProcess, cleanup и ranBeforeJob остаются.
UAC ABI/layout и разрешение bootstrap этим не аттестуются.

После V254 остались2 production references на удалённый spawnKernel32 в GUI
native bridge. Они заменены local updateKernel32 с прежними kernel32 procedures.
Source migration lesson: при переносе private global проверять consumers по всей
платформе, включая соседние GUI/TUN/UAC adapters, до фиксации source status.

Direct imports нового native owner/accounting — stdlib, existing nativelease и
pinned x/sys. Core/Wails/engine dependency в minimal package не добавлена.
Trusted create callback обязан действительно выполнить один CREATE_SUSPENDED
вызов; callback success, Job membership и numeric handles сами по себе не
аутентифицируют role/image/SDK. Caller serialization остаётся обязательной;
только один reserved Wait допускается параллельно, original handle неизменен.

### Review и оставшийся объём

Source12Go/4docs, gofmt12+1exit0; manual2 new/10 original diffs, actual production
callers/method sets/raw-field removal/phase order/borrow scopes/Wait-Close barriers.
16 snapshots:14 exact originals/2 absent-new; task71 rev549 archived в
.artifacts/memory/bootstrap-native-process-*. Named SHA/PLAN-task byte prefixes
и full goal/constraints/tasks0…70/acceptance/decisions/56 checks сохраняются.
Все100 acceptance OPEN,56 checks NOT_RUN; нового run/waiver нет.

Actual fixed-role helper executable ещё не реализован. Следующий implementation
seam — reusable original pipe-I/O/preparation и shared fixed-role Job policy,
затем helper/GUI-role-pair orchestration/remote handles/init-loader/creation-time.
OS, ABI, callback contracts, partial/Goexit, Wait/races/Close, resources/packaging
и lower acquisition остаются OPEN. Full PC00…09/old9/signed single-EXE/SDK/fresh
child authority/scalar async/de-elevation/W3b/interprocess/durability/UX/release/
comparative OPEN. Bootstrap/bridge/native=false, ranBeforeJob retained;
GUI6/R1/U1/IPC9/Job8/TUN unavailable/schema2 unsupported.

Абсолютный запрет tests/fixtures/build/compile/vet/typecheck/product parser/check/
doctor/install/gen/SDK/helper/product/crypto/native/browser/admin/live/network/
owner values/other-role/chat/subagents сохранён. Source review и gofmt не
подтверждают компиляцию, работоспособность, native safety или скорость.

<a id="pc-v256-original-pipes"></a>

## V2-256 — Original pipes and shared native Job limits, source/unverified

2026-11-06. PC-01; полный PC00…09/old9 и прежние gates сохранены.

### Actual pipe ownership

Existing core File/pipe/duplicate/attribute preparation перенесена в минимальный
spawnprep; второй implementation не остаётся. Core хранит zero Pipes в original
host и публикует его input/output interfaces ещё в pure process constructor.
Owns сверяет точные original parent slots, не просто совместимость типов.
Пять File slots,3 inherited duplicates и attributes остаются в одном owner.
Native duplicate outputs пишутся в исходный private массив. WithStartup даёт
только synchronous borrow stdio/allowlist; file/handle getters не добавлены.
Прежние directions stdin/stdout, DevNull stderr, SHOWWINDOW/HIDE, CREATE flags,
environment, Job-before-Resume и image/source guards сохранены.

### Acquisition и retirement

File резервируется до os.Pipe/OpenFile. Acquire/Read/Write не держат state mutex
над blocking вызовом; фактический возврат публикует original output под mutex.
Snapshot и Close проверяют acquiring до доступа к file, который acquisition
записывает вне mutex. Невозврат оставляет acquiring и original owner удержанными.
Close во время acquire помечает retiring и возвращает ошибку без native Close.
После late return только повторная очистка того же original slot может выполнить
свой единственный physical File.Close; новый Read/Write/acquire запрещён.
После уже начатого File.Close error/panic/Goexit sticky closeErr не сбрасывается.
Confirmed-empty Close теперь также запечатывает slot, предотвращая reopen.

Pipes имеет one-attempt Prepare и pending witness до actual return. Незавершённая
preparation блокирует release. Attribute borrow/Delete/Free и inherited Close
сохраняют прежние one-attempt barriers. ClosePreparation закрывает только child
preparation; parent endpoints живут для протокола после Resume и закрываются
прежним core reaper, который ждёт original I/O completion до release процесса/
image. Отложенное завершение API не подменяется timeout/canceled/ErrClosed.
Это не доказательство отсутствия hidden allocations внутри os.Pipe/OpenFile
или native hard deadline. Whole-Pipes операции требуют caller serialization;
параллельны только защищённые File I/O/Close. Проверка race/ABI ещё не выполнена.

### Общие Job limits

Прежняя native projection/configuration/verifier-query/accounting policy
перенесена в spawnprep.JobLimits. Core выбирает compiled Engine/helper/SDK/GUI
values и сохраняет role/image/source/SDK preconditions; module не выдаёт authority.
Shared100ns UserTimeTick исключает расхождение единиц CPU времени. Engine Job8,
helper memory, SDK CPU/memory/process ceilings, GUI kill+breakaway и verifier
exact-flag/no-breakaway/termination/peak/user-time predicates сохранены.
Configure дополнительно отвергает некорректные shapes до SetInformation; actual
compiled values не менялись. Expected original slot заполняется до native Set,
повторная конфигурация running Job не разрешается контрактом. Commit memory
не RSS; aggregate user CPU, kernel CPU и wall time не взаимозаменяемы.

### Evidence, восстановление и остаток

Source11Go/4docs, gofmt11+1exit0; manual4 new/7 original diffs, production callers/
removed fields/method sets/lock-publish order/stdio directions/limits predicates.
15 snapshots:11 exact originals/4 absent-new; task71 rev553 archived в
.artifacts/memory/bootstrap-v256-streams-*. Named SHA, PLAN/task byte prefixes
и full goal/constraints/tasks0…70/acceptance/decisions/56 checks сохраняются.
Все100 acceptance OPEN,56 checks NOT_RUN; нового run/waiver нет.

Перед реализацией обнаружена коллизия со старым own artifact prefix V245.
Исторический status восстановлен из SHA-matching predecessor, helper source и
receipt — из exact own rollout patch/stdout;14 original old snapshots неизменны.
Ошибочный stale checkpoint отклонён revision-conflict; product code до нового
successful V256 checkpoint не менялся. Recovery receipt сохранён отдельно.
Для следующих срезов использовать уникальный versioned artifact prefix.

Actual fixed-role helper executable ещё не реализован. Далее concrete dispatch/
fixed command-env/parent-helper-GUI role-pair/remote handles/init-loader/
creation-time orchestration; не general arbitrary-executable launch mode.
API/private types/ABI/acquire/Goexit/Close/I-O/concurrency/GC/OS/resources/
packaging и full PC00…09/old9/signed single-EXE/SDK/fresh child authority/scalar
async/de-elevation/W3b/interprocess/durability/UX/release/comparative OPEN.
Bootstrap/bridge/native=false, ranBeforeJob retained; GUI6/R1/U1/IPC9/Job8,
TUN unavailable/schema2 unsupported.

Абсолютный запрет tests/fixtures/build/compile/vet/typecheck/product parser/check/
doctor/install/gen/SDK/helper/product/crypto/native/browser/admin/live/network/
owner values/other-role/chat/subagents сохранён. Source/gofmt/byte receipts не
подтверждают компиляцию, работоспособность, native safety или скорость.

<a id="pc-v257-fixed-invocation"></a>

## V2-257 — Fixed argv/env and current helper dispatch, source/unverified

2026-11-06. PC-01; full PC00…09/old9 и прежние gates сохранены.

### Concrete production consumers

Childbootstrap теперь задаёт единственные existing GUI6/R1/U1 argument constants,
closed purpose factory, four-word argv builder и exact parser. Binding — ровно
32 bytes/canonical lowercase64 hex; modes только tun/proxy и resume/stopped,
elevation только tun. Zero/unknown purpose, лишние/неверные слова отвергаются.
Это value/correlation contract; image/parent/SID/SDK/App grants не переносятся
в parser. Core fixedApplicationGUIProtocol, GUI Start/validation, UAC shell argv
и child dispatcher используют этот код; duplicate unused protocol.argument field
удалён. Shell arguments формируются ComposeCommandLine, existing words/version
и private wire stages сохранены. Никакого argv-selected executable path нет.

Netprobe.HelperInvocation — closed header только current DNS/TUN roles, с bounded
DNS token и caller-owned Arguments. Core capture constructors и actual helper
main migrated. Header shape/size проверяются до pipeJobReady, strict DNS decode
по-прежнему после actual pipe/Job guard и через существующий DecodeNativeRequest.
Нового DNS decoder/packet mode нет. TUN subject/nonce остаются bounded stdin,
unknown/mixed/extra flags не переходят в measurement или GUI fallback. Header,
pipe/Job presence и role enum сами по себе не аутентифицируют родителя.

### Minimal environment

Actual clean environment construction перенесена из core в spawnprep builder.
Три compiled scopes: SystemRoot/WINDIR; public store дополнительно APPDATA и
TEMP/TMP; GUI дополнительно LOCALAPPDATA. Names/order/Unicode entry terminators/
double-NUL сохранены, generic env map/inherited extras не добавлены. Directories
чистятся и требуют fixed-local absolute drive paths <=2048 bytes; each encoded
entry <=4096 UTF16 words, fixed count <=6. Теперь system/Windows locations тоже
проходят эту explicit validation, поэтому invalid/noncanonical cases не считаются
байтово прежними. Total buffer/RSS/native latency и loader containment не доказаны.

Core по-прежнему получает platform directory values и выбирает scope по прежним
compiled role flags. Engine inheritance при cleanSystemEnvironment=false прежнее.
Environment и working-directory buffers удерживаются отдельными KeepAlive до
actual CreateProcess return. Builder не читает OS/store, не проверяет file IDs/
ACL и не даёт authority SDK. ShellExecuteExW UAC environment этим не изменён;
его init/loader interval и ranBeforeJob по-прежнему отдельные gates.

### Почему broker не подменяет existing U1

В current uacControlPipe.validateParent проверяется original parent handle path
== current App image и exact prepared-user SID. Если поставить helper вместо
App, это уже иная цепочка owners/образов. Predicate оставлен прежним.
Новая broker role здесь НЕ реализована, новая ABI/flag не объявлена supported.

Следующий concrete source slice обязан задать отдельный versioned broker purpose,
fixed App/helper targets и original parent/helper/GUI owners. До создания GUI
нужно расписать каждого владельца ancestor/child Job и query/control handles
на фазах до Resume, partial duplication, ACK и parent exit; отдельно показать
cleanup при crash/Goexit без PID reopen/root-only fallback. Полный запуск затем
использует уже имеющиеся Process/Pipes/JobLimits/imagelease/controlpipe primitives.
Original UAC helper execution до позднего Job attachment нельзя стереть nonce,
self hash или ACK. Creation-time/init/loader contract остаётся OPEN; bootstrap
gate не получает success от наличия меньшего executable или этого argv builder.

### Review и остаток

Source11Go/4docs, gofmt11+1+1exit0; manual3 new/8 original diffs, actual global
callers/removed fields/imports/method sets/mode-binding/argv bounds/decoder order/
env scopes, buffer lifetime и source guards.15 snapshots:12 exact originals/
3 absent-new; task71 rev557 archived в .artifacts/memory/bootstrap-v257-role-*.
Named SHA/PLAN-task byte prefixes/full ledger сохраняются.100 acceptance OPEN,
56 checks NOT_RUN exact; нового run/waiver нет.

Helper binary source changed: будущая поставка требует matched App/helper pin
обычным packaging contract; сборка/пара/compatibility не проверены. GUI6/R1/U1,
IPC9/Job8/native budgets/SDK и schema2 unsupported прежние. App-helper-GUI broker,
new role/pair/remote handles/init-loader/creation-time/API/private types/ABI/argv/
env/partial/Goexit/Close/GC/OS/resources/packaging и full PC00…09/old9/signed
single-EXE/SDK/fresh child authority/scalar async/de-elevation/W3b/interprocess/
durability/UX/release/comparative OPEN. All gates=false/ranBeforeJob retained;
TUN unavailable. Абсолютный запрет tests/fixtures/build/compile/vet/typecheck/
product parser/check/doctor/install/gen/SDK/helper/product/crypto/native/browser/
admin/live/network/owner values/other-role/chat/subagents сохранён.
Source/gofmt/byte receipts не подтверждают компиляцию, VPN safety или скорость.

<a id="pc-v258-creation-job"></a>

## V2-258 — Creation-time Job and post-Resume GUI handles, source/unverified

2026-11-06. PC-01 prerequisite для будущего App→helper→GUI; full PC00…09/old9
сохраняется. Новый broker purpose/dispatch здесь ещё не реализован.

### Production creation sequence

Раньше actual spawner сначала CreateProcess(CREATE_SUSPENDED), затем Assign.
Теперь Prepare получает original Job через native.WithJob. Attributes хранит
отдельный one-element Job vector без нового handle owner; HANDLE_LIST и JOB_LIST
устанавливаются в list capacity2. Оба vectors pinned/immutable и original Job
удерживаются до actual CreateProcess return; checked preparation cleanup прежний.
Partial Update/Pin/initialization остаётся в original owner, no late-Assign fallback.

Константа JOB_LIST взята из установленного Windows SDK10.0.26100.0 um/winbase.h:
ProcThreadAttributeJobList=13 (line3531), INPUT=0x00020000 (3509), macro uses
thread=false/input=true/additive=false (3963…3964). В pinned x/sys v0.44.0
exported constant отсутствует. Это локальная source provenance, не запуск SDK,
не подтверждение ABI или поддержки OS. Unsupported attribute/creation fails closed.

После Create и reservation исходного Wait native.ConfirmJob делает ровно одну
IsProcessInJob(original process, original Job) проверку Win32 BOOL. Pending
confirmation sticky при nonreturn; positive query — необходимое условие Resume,
не доказательство image/trust/SDK/Ready. Late Assign API удалён из этого owner.
После checked preparation close и existing image/source/resource guards Resume
теперь требует previous suspend count==1. Zero/>1/native error не позволяют spawn
считать переход успешным; caller сохраняет original process для cleanup.

### Uncertain membership and GUI handles

Failed Create/confirmation может уже означать принадлежность original Job.
При assigned=false Terminate пытается и original Job, и original root process;
root termination error снимается только при actual zero-wait signaled root.
Job termination error не скрывается этой нормализацией. Empty требует root exit
AND zero ActiveProcesses исходного Job. Существующий reaper дополнительно ждёт
original Wait/I/O и checked File/attributes/image/native handle release; timeout
и root exit отдельно cleanup authority не дают. No PID reopen/taskkill/replacement.

StartGUI теперь оставляет beforeResume только image/source guards. После actual
spawn return/Resume повторно проверяет source и original process image, затем
duplicate query Job/parent handles в original child, ещё раз source перед seed.
Partial remote values остаются target-owned, родитель не CloseHandle чужие числа.
Это убирает выдачу self-retaining query Job handle ещё не resumed child.
GUI ожидает fixed seed по прежнему protocol; EOF/deadline/physical scheduling/
loader/crash/GC/OS guarantees здесь не измерены. Success count1 не исключает
последующую внешнюю suspension и не доказывает исполнение child entry point.

### Review, limits and next step

Source6 existing Go/4docs; gofmt6+1exit0. Вручную просмотрены6 diffs, production
callers/removed Assign and witnesses, Win32 BOOL/result/count, same original Job
through Prepare/Create/Confirm/Resume, pinned vectors/partial release, original
reaper and GUI seed ordering. x/sys ResumeThread return signature и local SDK
macros прочитаны как исходники.10 exact originals/task71 rev561 archived в
.artifacts/memory/bootstrap-v258-creation-*. Named SHA/old PLAN-task prefixes/
full ledger сохраняются;100 acceptance OPEN/56 checks NOT_RUN exact/no waiver.

No tests/fixtures/build/compile/vet/typecheck/product parser/check/doctor/install/
gen/SDK/helper/product/crypto/native/browser/admin/live/network/owner values/
other roles/chats/subagents. Gofmt/source bytes не подтверждают компиляцию,
creation-time OS containment, совместимость nested Job/breakaway, crash cleanup,
latency/RSS или VPN safety. Все platform/ABI/Close/partial/Goexit/GC/resource/
packaging gates OPEN. Engine limits8/GUI breakaway flags и протоколы прежние.
UAC ShellExecuteExW всё ещё запускает child до позднего Job attachment:
ranBeforeJob сохраняется; all bootstrap/bridge/native gates=false.

Далее — distinct versioned broker purpose/fixed App/helper targets, original
parent/helper/GUI и ancestor-Job query/control owners по фазам до Create/Resume,
partial duplicate/ACK/parent exit, затем executable dispatch. Existing U1
same-App parent/SID predicate нельзя молча переиспользовать для helper.
Full PC00…09/old9/signed single-EXE/SDK/fresh child authority/scalar async/
de-elevation/W3b/interprocess/durability/UX/release/comparative OPEN.
GUI6/R1/U1/IPC9/Job8/TUN unavailable/schema2 unsupported retained.

<a id="pc-v259-broker-wire"></a>

## V2-259 — Distinct broker wire and coordinator, source/unverified

2026-11-06. PC-01; full PC00…09/old9 сохраняется. Это source protocol layer,
а не реализованный native broker или разрешение ElevatedGUIAvailable.

### Closed purposes and reuse

BrokerProtocol отделён от existing Protocol/Purpose: helper control SB01/AB01/
GB01/OB01/UB01/KB01, successor GUI SG01/AG01/GG01/OG01/UG01/KG01. Нет reuse
U1's same-App parent predicate. Exact flags --internal-bootstrap-broker-v1 и
--internal-broker-gui-v1, ровно4 слова, canonical lowercase64hex binding, tun,
resume|stopped. Это только elevation; произвольные paths/extra argv не передаются.
Binding, role enum и callbacks не заменяют file/peer/token/Job authentication.

Actual App main и helper run распознают оба reserved prefix в любом месте argv
до legacy GUI/worker/DNS/TUN services. Пока native integration отсутствует,
оба режима (включая valid argv) терминально отвергаются с прежними exit3/2.
Новый parser/sequencer ещё НЕ имеет native consumer. Current helper DNS/TUN и
GUI6/R1/U1 wires/versions/availability flags этим не объявлены migrated/supported.

### Handle contract and resource hierarchy

| Hop | Seed size | Target-process values in exact order |
|---|---|---|
| App→helper control | 60 | ancestor Job query, helper Job query, original App query/sync |
| helper→GUI successor | 68 | GUI Job query, original helper query/sync, ancestor Job query, original App query/sync |

Control uses3 entries of fixed4 array, last must be0. GUI uses all4. Exact length/
stage/binding, nonzero/nonpseudo/distinct numeric values are checked before native
Adopt. Invalid raw handles never authorize Close. Object identity, pointer width,
noninheritance, access rights, BOTH memberships, fixed pair/SID/peer remain native
owner obligations; distinct numbers do not prove distinct kernel objects.

Initial pre-checkpoint proposed2 control handles. Source review of current
one-process helper policy versus uncapped GUI descendants required a third:
helper's resource Job must be separate from ancestor lifetime Job. Otherwise a
helper active-process1/CPU/memory ceiling also constrains WebView/engine children.
Native job hierarchy/limits/breakaway ordering are NOT selected or proven by the
wire. Future implementation must fail closed if it cannot retain ancestry while
escaping only helper limits; it must never clear breakaway/resource flags to pass.

Intended custody for next native slice (not current implementation):

| Phase | Original App | Original helper | GUI |
|---|---|---|---|
| Before helper GO | owns helper hProcess and ancestor/helper control Jobs | validates own two query Jobs and original App handle | absent |
| GUI Create→before Resume | retains ancestor control | owns original GUI process/thread/Job; confirms containment before Resume | no lifetime query handles yet |
| After Resume→USE | keeps same original tree owner | duplicates4 query/sync handles only after Resume; retains partial remote values | adopts all4, no normal services |
| GUI ACK→broker ACK | retains commit/ACK ambiguity and original tree | records actual commit/ACK/close facts, no replacement | awaits BOTH original App and helper |
| Accepted helper/App exit | original receipts/authority remain platform decisions | actual physical exit, not timeout receipt | query GUI/ancestor Job survives; then own fresh startup checks |
| Failure/unknown | retain/retire original tree, never root-only success | keep exact partial native owners | terminal, no GUI fallback |

Shell UAC execution before Job attachment still sets ranBeforeJob. This source
layer cannot remove loader/init/creation-time gaps. Helper termination after ACK
is expected and must not be confused with a pre-ACK crash; parent native receipt
and GUI await must handle that boundary explicitly.

### Coordinator and common child sequence

BrokerCoordinator is published before Run and cannot run twice. It records
Attempted/Returned/Succeeded for adoption, successor preparation, commit, broker
ACK and transport close. Returned errors, panic/Goexit, post-callback scope error
and lost ACK cannot erase an earlier actual committed callback. Progress is data,
not transferable grant or whole-tree cleanup receipt; caller serializes it.
No timer/goroutine/native acquire/implicit cleanup or reset exists in this layer.

App GO precedes PrepareSuccessor. App USE precedes CommitSuccessor; the callback
must observe exact GUI ACK before broker ACK. Scope and ValidateChain surround
boundaries. Failed broker ACK/close after child commit remains terminal/ambiguous;
future App/native owners may not silently restore an untransferred state.

RunBrokerGUI reuses the existing child sequence implementation through private
runHandoff with role-specific seed/adopt/exchange bounds. Four-handle native
adoption and ValidateTree occur before Prepare/ARM; tree validation repeats before
ACK and after awaiting BOTH origins. Prepare/Validate/Verify/source/transport
callbacks stay platform-owned. Legacy Run still uses seed52, original2 handles,
same stages and ValidExchange; old U1 parent/SID checks are untouched. Internal
seed storage grows52→68bytes, no RSS/performance/ABI equivalence is asserted.

### Evidence and remaining work

Source6Go/4docs:3 new protocol/coordinator/GUI adapters +3 existing child sequence/
App/helper entries. gofmt6+2exit0; manual3 new files/3 original diffs, callers,
closed grammar/sizes/order, partial progress/nonreturn, reused legacy sequence and
entry-before-services traced.10 snapshots7 originals/3 absent-new/task71 rev565
archived in .artifacts/memory/bootstrap-v259-broker-*. Named SHA/old PLAN-task
prefixes/full ledger retained;100 acceptance OPEN/56 checks NOT_RUN/no waiver.

Next actual native App/helper/GUI owners and separate ancestor/helper Jobs,
fixed sibling pair/loaded-image limits/SID/peer predicates, distinct pipe namespace
and bounded60/68-byte exchanges, original suspended Create/Resume, partial
duplicate/Close/ACK ambiguity/expected helper exit, then native handler integration.
No repeated extraction of already available Process/Pipes/image/control primitives.

Absolute no tests/fixtures/build/compile/vet/typecheck/product parser/check/doctor/
install/gen/SDK/helper/product/crypto/native/browser/admin/live/network/owner values/
other-role/chat/subagents. Source/gofmt does not prove compilation or runtime.
All protocol/API/ABI/GC/Close/OS/nested-job/breakaway/resource/packaging/full PC/
old9/SDK/signed single-EXE/fresh child authority/scalar async/de-elevation/W3b/
interprocess/durability/UX/release/comparative OPEN. All gates=false/ranBeforeJob;
GUI6/R1/U1/IPC9/Job8/TUN unavailable/schema2 unsupported retained.

<a id="pc-v260-broker-transport"></a>

## V2-260 — Typed broker transport on original Windows Pipe, source/unverified

2026-11-06. PC-01; full PC00…09/old9 remains active. Prior V259 was source
wire/coordinator only. This slice implements the native transport adapter;
App/helper chain/Jobs/spawn/entry handlers are still NOT integrated.

### Existing native owner extended

Existing controlpipe.Pipe retains Win32 handle, LocalAlloc SID/descriptor results,
NOWAIT/local/first-instance flags, original peer ID comparisons, mutex/retirement
intent, pending acquire/physical I-O and sticky one-attempt Close/LocalFree.
No second pipe implementation, raw native handle getter or PID-based acquisition.
Private listen/dial/exchange now take only a closed compiled wire kind; kind is
stored in original owner before native acquisition and never reset/reassigned.

| Connection | Fixed namespace | Exact seed grammar |
|---|---|---|
| Existing U1 | snowden-uac-v1- + canonical nonce | 52 bytes |
| App→helper control | snowden-broker-v1- + protocol binding | 60 bytes |
| helper→GUI successor | snowden-broker-gui-v1- + protocol binding | 68 bytes |

Each namespace is under the existing local Windows pipe prefix. Protocol.Binding
returns32 value bytes only. Names use canonical lowercase64 hex; no arbitrary
pipe name/prefix supplied by a caller. Existing Name/Listen/Dial/Exchange APIs
remain U1-only. Broker lengths delegate to the V259 protocol grammar; legacy
ValidExchange remains unchanged. Internal input/output storage52→68 bytes, with
length checks before copy/native I-O. This is no total RSS/latency/ABI proof.

### Original typed binding and sequencing

NewBrokerPipe is a pure constructor for one valid broker purpose and binding.
Caller publishes its original pointer before Listen/Dial; zero/invalid purpose
rejects. Native Pipe is private; no escape for switching it to U1 or another hop.
Accept/CheckServer use only comparison PIDs from original held child/parent
process handles supplied by the platform. The transport's PID/name fact alone
does not authenticate that original image, primary-user SID or Job membership.

RunCoordinator and RunGUI bind Exchange and CloseAfterServer to this exact
original client transport; caller-provided transport callbacks are rejected.
Scope is captured separately rather than retaining a self-referential operations
closure. The caller supplies existing original native chain/spawn callbacks and
retains all partial image/process/Job owners after any error/nonreturn.

Atomic runStarted permits one sequencer attempt only. readyClient under the
native mutex verifies fixed broker kind, connected client/peer and no prior
sequence/EOF/failure, then latches sequencing before any sequenced frame. Native
Exchange compares this latch under the SAME mutex with manual/sequenced mode.
Thus a manual call that races the outer atomic check cannot emit a frame after
sequencing has latched. An already-running manual native call makes readyClient
busy and burns the original run attempt; it is not a completed handoff.
Failure/Close does not reset either latch; no replacement/reconnect happens.

Final client ACK still enters awaitingEOF without a post-write peer query;
CloseAfterServer waits expected EOF using the same original handle and checked
Close/LocalFree. ACK may already have reached its peer before a later scope or
Close error: prior coordinator attempted/returned/succeeded facts remain intact.
EOF/deadline/failed interest is not transfer, installation or cleanup authority.
Retire/Close retain original cancellation/unknown-owner semantics. Non-Windows
methods return unavailable and do not implement a fallback transport.

### Review and next work

Source5Go/4docs:1 new typed broker adapter,4 existing protocol/controlpipe files.
gofmt5+3+3exit0; manually reviewed1 new/4 original diffs, production U1 consumers,
method signatures/build variants, fixed role/name/length mapping, latch/lock order,
partial I-O/no-copy-before-full-response and ACK/EOF/Close paths.9 snapshots:
8 exact originals/1 absent-new; task71 rev569 exact archived in
.artifacts/memory/bootstrap-v260-transport-*. Named SHA and old PLAN/task byte
prefixes preserved;100 acceptance OPEN/56 checks NOT_RUN exact/no waiver.

BrokerPipe native methods are connected to sequencer adapters as source code;
there is still no production native broker handler invoking them. App/helper
entrypoints remain terminal for BOTH valid/malformed new roles. The next slice
must implement actual original App/helper/GUI leases, separate ancestor/helper
Jobs and fixed pair/SID/peer predicates, then suspended spawn, partial remote
duplicates, ACK ambiguity/expected helper physical exit and entry integration.
Do not re-extract ready Process/Pipes/image/control primitives.

Absolute no tests/fixtures/build/compile/vet/typecheck/product parser/check/doctor/
install/gen/SDK/helper/product/crypto/native/browser/admin/live/network/owner
values/other-role/chat/subagents. Source/gofmt is not compiler/Windows evidence.
Native ABI/I-O/lock-race/Close/GC/OS/nested Job/breakaway/resources/packaging/full
PC00…09/old9/SDK/signed single-EXE/fresh child authority/scalar async/de-elevation/
W3b/interprocess/durability/UX/release/comparative remain OPEN. All gates=false/
ranBeforeJob; GUI6/R1/U1/IPC9/Job8/TUN unavailable/schema2 unsupported retained.

<a id="pc-v261-broker-custody"></a>

## V2-261 — Native child custody and gated App dispatcher, source/unverified

2026-11-06. PC-01; full PC00…09/old9 preserved. Native child source is now wired
to the App entrypoint, but availability/acceptance remains false. No helper-side
controller/spawn or App-parent broker request has been implemented here.

### Closed Job policy, original ownership

Existing spawnprep extended; no second ACL/SID/image reader/native Wait framework.
BrokerJobInformation/ConfigureBrokerJob/QueryBrokerJobLimits share a closed role
projection. Configure publishes the caller's original expected slot BEFORE native
query/Set, rejects a nonfresh Job/accounting and never retries/reset CPU allowance.
Caller retains the original Job/control slot; writer is prepared for the pending
parent/helper handler, not an automatically invoked installer/service.

| Role | Required flags/limits |
|---|---|
| Ancestor | KILL_ON_JOB_CLOSE only, no breakaway/helper resource limits |
| Helper | KILL/explicit BREAKAWAY/active1/job committed128MiB/user CPU20sec, terminate at Job time |
| GUI | KILL/explicit BREAKAWAY, no helper active/memory/CPU ceiling |

Helper caps are initial/unmeasured policy, NOT throughput/RSS/wall-time proof.
Query checks exact role flags/limits and helper time action0; child helper lease
also checks count1/no terminated member, observed user CPU and memory peaks.
Nested-job/breakaway feasibility and exact granted access masks remain native
acceptance items. Supporting operations/flags do not attest hierarchy or rights.

BrokerLease is published before Adopt, retains claimed values and original native
slots. Existing wire shape validates3/4 values via new ValidHandles without an
encode allocation. Native checks width/nonpseudo/handle flags0; role Jobs contain
this child. Different role flags reject a same-object alias under different
numeric handles. Original predecessors cannot belong to child's role Job; live
GUI helper also belongs to this same ancestor Job. No process is opened by PID.

The lease snapshots current primary SID through existing bounded fileacl codec,
requires elevated current process, opens original predecessor TOKEN_QUERY outputs
directly into retained slots, and compares exact users (not the broader trusted
Admin/SYSTEM set). GUI helper token must also be elevated. Original process IDs
are comparison facts from held handles, never acquisition/ownership authority.
Alive/status and SID checks repeat before handoff; after origins close current
primary-user/elevation still rechecked. Reported path/token facts do not prove
loaded sections, App grant, SDK trust or original pre-Job containment.

### Both-origin Wait and permanent Job references

WithChain borrows handles under original mutex for synchronous image/peer checks;
WithAncestor can borrow the retained lifetime query after origins have exited.
Neither callback may retain/close/reenter. Error/panic/Goexit burns positive use;
there is no re-adoption, arbitrary raw Close or default replacement.

GUI AwaitOrigins uses one derived15sec context and repeated100ms WaitAll over
the TWO original process handles. waitPending is published before the native
call and cleared only on actual return; original-signaled fact precedes later
scope checks. Helper may already have exited after ACK/EOF: wait admission permits
signaled origins without trying to reclassify them as live Job members.
Only both-signaled actual Wait permits original tokens and process slots to close
once via existing CloseOnce. Uncertain acquisition/Close/nonreturn is retained;
deadline is not cleanup. OriginsRetired describes only these slots, not a drained
descendant tree. Both Job query references remain until this GUI's OS exit.

### Actual App child dispatcher

App main calls DispatchBrokerGUI before legacy dispatch/normal services, passing
the existing compiled helper pin. Both reserved namespaces/malformed/mixed argv
remain terminal; only successor role belongs in the App. The original session,
native lease, typed pipe and image owner are stored before acquisition, including
partial/nonreturn. Legacy U1 retains its separate lease/predicate; the two
bootstrap globals prevent a second conflicting legacy/broker session.

Windows session uses current windowsNativePair (same held files/ACL/hash/helper
PE/WFP AppIDs policy), typed BrokerPipe and original lease callbacks. Before ARM/
GO/USE it checks self/current App path, original helper path, original App path,
exact primary users, Jobs and named-pipe server PID derived from held helper.
Lock order: native lease→held Pair→original Pipe; preliminary checks release each
mutex before that chain. After both origins/transport retire it rechecks held
image/self/jobs without reopening parent PID/token/peer. Image closes checked only
after the child sequence succeeds; remaining query Job slots stay session-owned.
Non-Windows adapter returns unavailable; no platform fallback.

Existing bootstrap/bridge/native gates AND new brokerGUIChainAccepted=false are
required before native run. The handler is source-connected, NOT enabled/proven.
No source change to the helper main's reserved-role rejection or to original UAC
ShellExecute ranBeforeJob; no new parent authority/SDK/installation receipt.

### Newly identified relaunch dependency

Current prepareUpdateGUIHandles/updateGUITransfer passes only2 values, while
successor spawner requests BREAKAWAY. A GUI started under the new nonbreakaway
ancestor must retain that ancestor query across ALL later restart/update handoffs.
The old R1/GUI6 child would lack that reference; last original ancestor-handle
closure can kill the intended successor under the declared KILL policy. This is
a source/model finding for the unavailable new chain, NOT observed live failure.

The new gate explicitly cannot be accepted before a distinct versioned relaunch
wire carries that retained reference. WithAncestor provides the guarded borrow;
it does NOT implement a relaunch by itself. Preserve legacy wire/version and add
explicit child leases, fixed args/frames, membership before Resume and query
duplication after Resume. Keep one original ancestor lifetime, without accumulating
a fresh ancestor layer on every restart. Then complete actual original parent/
helper controllers/spawn/remote handles/ACK ambiguity/expected helper exit.

### Review/evidence and limits

Source9Go/4docs:6 new native policy/lease/identity and core GUI dispatcher/platform
adapters +3 existing protocol/session/main. gofmt9+2+1+2exit0; manual6 new files/
3 original diffs and real consumers/imports/signatures, layout/membership/role
caps, primary tokens/Close witnesses, lock order, expected helper exit/WaitAll and
legacy2-handle relaunch source reviewed. x/sys WaitForMultipleObjects signature/
body read as source only.13 snapshots7 exact originals/6 absent-new/task71 rev573
archived in .artifacts/memory/bootstrap-v261-custody-*. Named SHA/old PLAN/task
prefixes/full ledger retained;100 acceptance OPEN/56 checks NOT_RUN/no waiver.

Absolute no tests/fixtures/build/compile/vet/typecheck/product parser/check/doctor/
install/gen/SDK/helper/product/crypto/native/browser/admin/live/network/owner
values/other-role/chat/subagents. Gofmt/manual source does not prove compilation,
ABI/Win32 rights/Job hierarchy/breakaway/physical cleanup/speed or production.
Parent/helper writer/spawn and ancestor-preserving relaunch remain pending;
native/API/I-O/GC/Close/resources/packaging/full PC/old9/signed single-EXE/SDK/
fresh child authority/scalar async/de-elevation/W3b/interprocess/durability/UX/
release/comparative OPEN. All gates=false/ranBeforeJob; GUI6/R1/U1/IPC9/Job8/
TUN unavailable/schema2 unsupported retained.

<a id="pc-v262-ancestor-relaunch"></a>

## V2-262 — Ancestor-preserving GUI7/R2 relaunch, source/unverified

2026-11-06. PC-01/full PC00…09/old9. Implements the V261 source/model dependency
for an extra ancestor reference across restart/update. No native acceptance is
claimed; ancestorRelaunchAccepted=false and all prior gates remain false.

### Separate versioned wire and actual consumers

| Purpose | Fixed argument | Stages | Seed values |
|---|---|---|---|
| Existing update | --internal-update-gui-v6 | unchanged SE06…ACK6 |52bytes/2 handles |
| Existing restart | --internal-restart-gui-v1 | unchanged SR01…KR01 |52bytes/2 handles |
| Ancestor update | --internal-update-gui-v7 | SE07/ARM7/GO07/OWN7/USE7/ACK7 |60bytes/GUI Job,parent,ancestor |
| Ancestor restart | --internal-restart-gui-v2 | SR02/AR02/GR02/OR02/UR02/KR02 |60bytes/GUI Job,parent,ancestor |

Existing U1 and broker hops keep their distinct tags/namespace/seed52 or60/68.
New application argv still exactly4 canonical words; no arbitrary executable/
extra mode/handle argv. Protocol owns fixed seed size/encoding and exchange bounds.
Shared numeric handle validator now supports closed2/3/4-value shapes; each role
still supplies its own exact size. Raw numeric shape/binding grants no authority.
Legacy receive/adoption still uses2 values; new roots require a separate3-value
callback before any Prepare. Single private child sequence reused, max buffer68
unchanged there; stdio exchange copies52/60 bytes into bounded buffers60.
General transport capacity does not widen legacy child or U1 pipe grammar.

Parent UpdateCodeAcceptance captures processGUIAncestor only from the original
bootstrap. Its private factory picks GUI7/R2 only when that reference exists,
retains it for commit and never retries with a legacy wire after failure. The
default process without a reference continues selecting GUI6/R1. Existing private
restart nonce/update manifest SHA bindings, SDK/acceptance and App receipts remain.
Producer uses Protocol.EncodeSeed; partial target values are stored even on error.
Dispatchers recognize actual version, fail unsupported root modes before native
pipe inspection, and select the matching native lease. A named one-attempt mode
is stored before adoption; failed mode cannot switch to the other lease branch.

### Same root, effective breakaway and original native ownership

The provider is published once before native adoption/dial and remains present
on partial failure, not a config/PID/source-selected replacement. Broker GUI and
GUI7/R2 children publish their exact retained native lease. Current process creates
only the ordinary successor GUI Job; it never creates another ancestor layer.

Before Resume, existing original Process.WithAssigned borrows the actual candidate
and then current GUI lease.WithJobs borrows original ancestor and old role Job.
IsProcessInJob must show candidate IN ancestor and OUTSIDE old role Job. This
checks effective breakaway instead of inferring it from flags. Source/own-image
checks remain before Resume. After successful Resume, the same predicates repeat
and DuplicateHandle lends only JOB_OBJECT_QUERY into that exact child, with
inherit=false. Numeric target values never CloseHandle in this parent. Existing
original child/Job/image/I-O reaper retains every uncertain/partial outcome.

Lock/borrow order is candidate Process→current GUI lease. Pair image validation
is separate, so no reverse Pair/root lock nesting was added. No PID reopen, named
Job discovery, generic spawn endpoint or reinterpretation of handles as grants.

### One-origin child lease and subsequent lifetime

BrokerLease.AdoptAncestor maps the new tuple to its existing GUI+ancestor policy,
with explicit singleOrigin branch. Query flags/nonpseudo/width/object shape,
membership in both Jobs, original parent outside child role Job/inside same root,
same primary SID/elevation and original query-token outputs remain checked.
Current implementation requires elevated current GUI/parent for this branch;
a proxy mode word is NOT de-elevation. COM/de-elevation obligations remain OPEN.

The child authenticates its current/held application path and original parent
path during image Prepare (before ARM) and later stages. It waits ONE original
parent via the same retained native Wait state and one15sec context, closes only
that original process/token after actual signaled return, and retains GUI/ancestor
queries until OS exit. The normal four-handle broker GUI still waits BOTH origins.
After parent retirement no PID/token/peer is reopened. A later GUI7/R2 handoff
borrows this same ancestor; the source-level scheme retains two permanent Job
references per current GUI without adding an ancestor at each restart. This is
not measured RSS, kernel depth, crash or actual repeated-relaunch proof.

Restart dispatcher now publishes its original session globally before the first
context/image/native acquisition and rejects a second attempt. Update/broker
already retained original sessions; partial/nonreturn restart image custody no
longer depends on a local-only returned pointer. Existing App/GUI acceptance and
final readiness remain distinct from OWN/ACK or these native owner facts.

### Permanent source cancellation versus uncertainty

New Root provider must survive a canceled later operation. WithJobs now treats
the confirmed post-origin GUI lease as a reusable query-only source: a returned
candidate error, or ordinary cancellation/deadline during such a borrow, may
return failure without burning the permanent reference. Every next borrow still
performs fresh source/job/token validation. It does not accept the failed child,
clear its original owner or permit a new candidate before retirement.

Before origin retirement bootstrap error semantics remain terminal. Nonreturn/
panic/Goexit and structural native source validation errors still mark failed;
unknown acquisition/Close cannot be made retryable. The callback is compiled,
synchronous and forbidden to mutate/close/retain source handles. This is not a
general exception for uncertain native I-O or a timeout cleanup receipt.

### Review and pending work

Source18Go/4docs:4 new seed/root-provider/platform files +14 existing producers,
decoders/lease/identity/dispatch/stdio/session adapters. gofmt18+6exit0;
manual4 new files/14 original diffs, actual production signatures/callers, mode
sealing/registration, exact legacy/new byte bounds, effective breakaway/duplicate
ordering, single/two-origin mappings, scope/partial/Wait/Close and lock order.
22 snapshots18 exact originals/4 absent-new/task71 rev577 archived in
.artifacts/memory/bootstrap-v262-ancestor-*. Named SHA/old PLAN/task byte prefixes/
full ledger preserved;100 acceptance OPEN/56 checks NOT_RUN exact/no waiver.

New parent and child paths are connected as source but gated unavailable. This
does not complete native relaunch acceptance. Parent path reporting across an
installation rename/old mapped image, exact rights, nested-job breakaway, GC,
native I-O/Close/physical cleanup/loop compatibility require Windows evidence.
Helper128MiB/20CPU sec policy unchanged and unmeasured. Schema2 remains unsupported.

NEXT actual App-parent broker request and helper controller: original Shell result,
separate original ancestor/helper Jobs, fixed paired target/SID/peer validation,
suspended GUI creation, original query/control transfer before Resume, partial
remote handles/ACK ambiguity/expected helper physical exit and source grant.
Use the existing Process/Pipes/Job policy/BrokerLease/typed transport; do not
extract these primitives again. Source-only relaunch integration is available
to compose, but all availability/acceptance gates must remain false.

Absolute no tests/fixtures/build/compile/vet/typecheck/product parser/check/doctor/
install/gen/SDK/helper/product/crypto/native/browser/admin/live/network/owner
values/other-role/chat/subagents. Source/gofmt/manual receipts do not prove
compilation, ABI or Windows behavior. Full PC00…09/old9/signed single-EXE/SDK/
fresh child authority/scalar async/de-elevation/W3b/interprocess/durability/UX/
release/comparative OPEN. All gates=false/ranBeforeJob; legacy GUI6/R1/U1,
IPC9/Job8/TUN unavailable/schema2 unsupported retained.

<a id="pc-v263-broker-parent"></a>

## V2-263 — App-parent broker request, source/unverified

2026-11-06. PC-01/full PC00…09/old9. Actual App parent source is connected;
brokerParentAccepted=false independently of bootstrap/bridge/native/chain/relaunch.
ElevatedGUIAvailable includes these gates so the UI cannot advertise acceptance
from legacy flags alone. No helper process was executed and no gate is accepted.

### Original App owner and fixed control-v2

App.launchElevatedModeGUI now stores NewBrokerGUIRequest(compiled helper pin)
before any native acquisition. The same ElevatedGUIRequest owns phase CAS,
original operation context, private App commit and exact-owner receipt. A private
legacy/broker adapter selects image preparation, native operations, seed and pipe;
no arbitrary executable/command callback or source-selected authority was added.
Legacy constructor/U1 dispatcher remain; no failure switches from broker to U1.

| Wire | Arguments | Stages | Seed / namespace |
|---|---|---|---|
| Existing broker control-v1 |4 exact words | SB01…KB01 unchanged |60 bytes / snowden-broker-v1- |
| Pinned broker control-v2 |5 exact words, last canonical helper SHA256 | SB02/AB02/GB02/OB02/UB02/KB02 |60 bytes / snowden-broker-v2- |
| Broker successor |4 words unchanged | SG01…KG01 unchanged |68 bytes / snowden-broker-gui-v1- |

New pin is compiled App data. The receiving helper must first authenticate the
original App/peer/primary user; an argv digest or helper self-hash is not trust.
The existing typed BrokerPipe fixes its version/namespace before acquisition.
Legacy U1/GUI6/R1 and ancestor GUI7/R2 grammar and byte bounds are not widened.
The actual helper entry still rejects every broker namespace before DNS/TUN.

### Shell result and two separate original Jobs

Broker parent embeds the existing Shell owner rather than copying its original
SHELLEXECUTEINFOW/hProcess into another request. Shared private launchShell keeps
the exact W ABI, fixed runas flags, thread-affine COM and original pins/output.
ranBeforeJob is now stored immediately after returned Shell reports execution
or a partial hProcess, before classify/SetHandleInformation can fail. This source
correction does not contain or remove that initial interval.

Ancestor Job and helper Job have original pre-published acquisition/configure/
close slots. Existing policy is reused: ancestor KILL only; helper active1,
128MiB committed memory/20 CPU-user seconds plus explicit breakaway. These are
initial unmeasured limits, not RSS/time measurements. Original helper must have
no unknown Job before assigning ancestor then helper. Independent attempted/
pending/returned-result facts retain uncertain assignment and duplicate calls.

Parent pins the current App/helper pair using the compiled helper pin; requires
the App's fixed packaged sibling name, original helper reported path, same exact
primary SID and elevated token, both Job memberships, limits/accounting/peaks and
live original process. Pair.MatchesUser reuses the original Trust user, not the
wider trusted ACL set of Admins/SYSTEM/installer. Pipe peer PID derives only from
the original held process and is comparison data, never PID acquisition.

The helper receives QUERY-only noninherited ancestor/helper Job references and
SYNC|QUERY_LIMITED original App reference in its own handle table. DuplicateHandle
writes into retained original output slots; partial target values stay recorded,
never CloseHandle'd in the parent. No root layer is recreated for later relaunch.

### ACK, expected helper exit and failure retirement

Existing App scope/private grant is checked before USE. Exact broker ACK follows
the helper's eventual GUI ACK; parent validates while helper remains live, closes
the original server, then waits for the actual original helper to exit zero under
one bounded15-second source scope. Each actual Wait return/signaled fact precedes
later cancellation. A live-helper check after server close would reject normal
helper exit; this branch instead requires empty helper Job and ancestor accounting
of exactly two total/one active waiting GUI. Original native refs remain retained
until App exit on successful transfer. This is wire custody, never VPN readiness.

Failure after GUI commit/ACK remains uncertain. Retirement operates on both
original Jobs and the known original helper root, recording each attempted/
returned termination before the next native call. Actual root exit AND both Job
active counts zero precede reference close. An unreturned acquire/assignment/
duplicate/Wait/COM/termination blocks cleanup. Original tokens/process/Jobs now
use one-attempt CloseOnce: an uncertain Close is never retried on a possibly
recycled numeric value. Failed token acquisition is latched and cannot become a
later successful validation just because a partial handle was returned.

Even if all known references close and both Jobs become empty, ranBeforeJob
continues returning cleanup_unconfirmed. It is not erased by a smaller helper,
root-only kill, zero exit, timeout or empty accounting. Original owner remains in
App on uncertainty; no replacement request or success receipt is minted.

### Evidence and next implementation

Source10 Go files/4 docs. Exact touched gofmt10+5+3 exit0; manual all3 new files,
7 original diffs and original production phase/grant/receipt/callers, transport
tags/names/bounds, native pair/user/Job/partial/Wait/Close ownership inspected.
14 snapshots11 exact originals/3 absent-new; task71 rev581 archived under
.artifacts/memory/bootstrap-v263-parent-*. Named source SHA and old PLAN/task
byte prefixes preserved; full goal/tasks0…70/100 OPEN acceptance/56 NOT_RUN checks
and decisions/constraints unchanged. No check run, no waiver, no compile claim.

NEXT actual minimal helper controller, terminal dispatcher and fixed sibling GUI
spawn. Compose existing Pair/BrokerLease/Process/Pipes/Job policy/coordinator:
authenticate original App before adopting pin; create suspended GUI in its own
Job while staying in the same ancestor/outside helper resource Job; original
query/control transfer, Resume, exact GUI ARM/OWN/ACK, helper ACK/EOF/exit and
original Job/process/I-O cleanup on every error/nonreturn. Keep all gates false.

Native UAC pre-Job gap, rights, nested-job breakaway, original environment/GC,
loaded sections/install-rename compatibility, physical I-O/cleanup/resources/
de-elevation, SDK/ABI/packaging/full PC00…09/old9/signed single-EXE/UX/release/
comparative remain OPEN. Absolute no tests/fixtures/build/compile/vet/typecheck/
product parsers/check/doctor/install/gen/SDK/helper/product/crypto/native/browser/
admin/live/network/owner values/other-role/chat/subagents. Source and gofmt are
not runtime evidence. Legacy GUI6/R1/U1/IPC9/Job8/TUN unavailable/schema2 unsupported.

<a id="pc-v264-helper-controller"></a>

## V2-264 — Minimal helper controller and suspended GUI, source/unverified

2026-11-06. PC-01/full PC00…09/old9. The actual helper entry/controller/GUI
successor are now composed as source. brokerHelperAccepted=false independently
of App parent/bootstrap/bridge/native/chain/relaunch=false. No executable ran.

### Terminal helper entry and original App authentication

Reserved broker namespaces in helper main now route to runBrokerInvocation,
never DNS/TUN on any failure. Only pinned control-v2 is supported by the actual
controller; malformed/mixed/v1/successor arguments are terminal. Global original
session is published before Dial/adoption/image/child/native acquisition. One
coordinator/one child/one attempt, original pending and returned state retained.

Existing BrokerPipe.RunCoordinator binds the original transport. BrokerLease
first checks delivered Jobs, current elevated primary user and held original
App. Helper then checks its fixed compiled sibling names/current and App reported
paths, parent==origin and original App pipe peer derived from the held handle.
Only after this authentication does it use the argv pin in original Pair.Prepare.
No self-hash or argv scalar alone becomes trusted authority. Subsequent steps
repeat source/Pair/path/SID/Job/peer checks; no PID lookup or replacement owner.

ApplicationFile moved to the existing small childbootstrap contract and the
updater keeps a const alias with identical value. The helper controller does not
import update/core/Wails or signers; native PE/ACL/file/Job/pipe owners are reused.
The existing netprobe role package and normal DNS/TUN paths remain separate.

### One fixed suspended GUI and effective ancestry

After original App GO, helper prepares only the fixed sibling GUI with a fresh
purpose-specific binding/pipe. Its original Process/Pipes/Job/attributes/token/
target-handle/image-vector slots already exist in the original session. Existing
GUI Job policy and creation-time JOB_LIST are used; no late Assign fallback.
Explicit breakaway is requested; source must then observe candidate IN the same
ancestor and OUTSIDE helper resource Job before Resume. The original GUI Job
must have exactly one live process and no terminated process/helper caps.

Fixed GUI args and UTF16 command/environment/cwd vectors remain reachable
through synchronous CreateProcess into Process's original output structure.
After native creation return: ConfirmJob, checked preparation Close, original
GUI image/primary SID/elevation/resource/ancestry validation. Four noninherited
target values are duplicated BEFORE Resume: GUI Job QUERY, helper SYNC|QUERY,
same ancestor QUERY, original App SYNC|QUERY. Partial values/attempts/native
pending and actual return facts are retained, never closed in the helper table.
Fresh source/identity/ancestry checks surround Resume and actual held-process
pipe Accept, then exact seed/ARM/GO/OWN. No arbitrary image/command/handle argv.

### Detached stdio and bounded GUI environment preparation

Existing Pipes now has a one-attempt PrepareDetached branch: read-only NUL stdin,
write-only NUL stdout/stderr. The same original three-handle allowlist/Job-list
attributes and checked native ownership are reused. Anonymous input/output parent
endpoints are never opened; no reader/writer goroutines or buffering are added.
Ordinary Prepare still opens the existing anonymous pipes with unchanged API.

GUI environment reuses BuildEnvironment: only native RoamingAppData/LocalAppData,
fixed LocalAppData/Temp, system Windows directory fields and system cwd. No
arbitrary inherited environment map is forwarded. Known-folder raw native output
slots are retained before SHGetKnownFolderPath, actual success is distinguished
from partial failure, copy stops at NUL with a2048 UTF16-unit cap. Each proven
CoTaskMem allocation has a one-attempt free; uncertain failure/nonreturn is kept.
Current installed x/sys0.44 source was read for this ABI/CoTaskMem behavior;
no API was executed. Native profile redirection/custom APPDATA/temp continuity
and loaded DLL/WebView/bootstrap resource compatibility remain acceptance work.

### Commit, actual ACK/EOF facts and local retirement limits

Only original App USE invokes the GUI commit. The helper validates origin/GUI,
sends exact GUI USE, records matched GUI ACK before a later scope cancellation,
then revalidates and closes the original GUI server. GUI can now close its client
and await BOTH original helper and App. Coordinator sends helper ACK only after
that child result, then observes original App server EOF. Exact native EOF/Close
progress is retained even if a later scope fails. No peer query follows expected
App EOF. After final origin/GUI checks the helper returns zero and physically
exits; App V263 still requires its original actual zero-exit/Job facts before its
private transfer receipt. No wire stage is VPN/protection readiness.

Every failed/partial sequence remains terminal. Original child retirement first
terminates its known Process/Job, then closes original pipe/preparation/folder
resources, observes empty local Job, and uses Process.Close's actual root-exit
check/one-attempt handles plus original token CloseOnce. Pending prepare/commit/
duplicate/token/termination calls prevent a fabricated completed cleanup. No
goroutine timeout/PID reaper/new Job/alternate child is introduced.

This is a LOCAL child-resource result, not whole-ancestor authority: helper has
only an ancestor query reference; original App retains ancestor control for its
terminal error. Initial runas ranBeforeJob cannot be removed by this composition.
QUERY reference lifetime versus last-handle closure, explicit root termination,
unexpected origin death and descendants outside a role Job need a separate
whole-chain safety argument and Windows evidence. Do not treat closing one
control handle as an observed empty root or open any acceptance gate from it.

### Evidence, unchanged obligations and next work

Source9 Go files/4 docs; exact touched gofmt9+1+1 exit0. Manual all4 new files,
5 original diffs and actual main/parent/GUI/coordinator/creation/transport callers,
original partial/native pending/locks/ACK/EOF/Close reviewed.13 snapshots9 exact
originals/4 absent-new; task71 rev585 archived under bootstrap-v264-helper-*.
Named source identity and old PLAN/task byte prefixes preserved. Full goal,
tasks0…70,100 OPEN acceptance,56 NOT_RUN checks, decisions and constraints remain
exact; no test/check/build/native/SDK run, no waiver or compile proof.

NEXT source integration review of complete App/helper/GUI/relaunch and PC-01
admission/retirement: original root authority/query-reference lifetime, post-ACK
failure/cancellation, partial native ownership, physical teardown and traffic
cutover through existing Manager/protection claims. Compose existing consumers,
not another generic spawner or parallel lifecycle. Keep every acceptance flag
false until its actual independently authorized evidence exists.

Full PC00…09/old9/OS rights/nested Job breakaway/UAC pre-Job gap/environment/
profile continuity/loaded sections/GC/resources/de-elevation/SDK/ABI/packaging/
signed single-EXE/UX/release/comparative OPEN. Absolute no tests/fixtures/build/
compile/vet/typecheck/product parser/check/doctor/install/gen/SDK/helper/product/
crypto/native/browser/admin/live/network/owner values/other-role/chat/subagents.
Legacy GUI6/R1/U1/IPC9/Job8/TUN unavailable/schema2 unsupported retained.

<a id="pc-v265-final-io-facts"></a>

## V2-265 — Retain final native pipe effects across late failure, source/unverified

2026-11-06. PC-01/full PC00…09/old9. All acceptance flags remain false.

### Concrete source defect and original receipt

Original controlpipe exchange used a local raw input buffer and copied it to the
caller only AFTER final peer/source validation. A full native ACK read followed
by cancellation/peer failure was returned as an error without preserving those
bytes in the original parent/helper request. V264 helper's ACK assignment after
Exchange could therefore miss the actual observation. Raw effect and accepted
handoff need separate facts; a failed exchange is still a failed exchange.

New ExchangeReceipt is a one-attempt original caller-owned record, published
before any native Write/Read. Private send/reply buffers are each bounded68 bytes;
actual partial counts, pending and native-return facts are stored before later
error handling. Oversized native count faults the record. ReplyMatches compares
exact original send/reply bytes and actual returned counts; MayHaveSent includes
an unreturned native write as negative uncertainty. Neither method supplies
native handles, source/peer/SID/Job authority, receipt redemption or readiness.
Caller serialization is required; the record is never reset/reused/replaced.

Existing Pipe.Exchange stays available with its original signature. Explicit
U1/Broker ExchangeObserved adapters reuse the same native pipe/byte limits and
fixed wire/namespace/sequencing checks. No injected transport, retries or fallback
were added. Observational bytes may remain available even when caller receive
stays untouched due to the existing late peer/source error.

### Actual parent/helper consumers and native nonreturn

Original ElevatedGUIRequest now owns its final receipt/expected USE+ACK/actual
exchange-return and ACK-observed fields. The original private App grant and all
positive source/peer/native/helper exit/Job checks still precede Transferred.
If final native USE may have taken effect and transfer fails, error includes
ErrElevatedGUIUncertain AND ErrHelperCleanup. Existing App local retirement
classification therefore preserves blocked cleanup semantics, not channel churn
or an ordinary retry. A raw matched ACK alone cannot mint a transfer receipt.
Effect recording in recovery cannot invent a native returned/completed result.

The helper's GUI USE/ACK consumes the same observed transport path. Matched raw
GUI ACK persists after later cancellation, error or callback panic/Goexit; the
effect-only defer compares retained data, while commitPending/actual returned
fields retain their original meanings. Coordinator sends no positive helper ACK
after a failed child commit; original local/root cleanup remains necessary.

Original native Pipe now stores ioPending before each exchange Write/Read and
expected EOF Read, clears only after the native call actually returns. Close
refuses an original handle with pending native I/O, even if a failure unwound
the pipe mutex. No timeout/cancel/defer is converted into physical completion.
This does not establish physical syscall latency or whole-tree retirement.

### Existing terminal admission and unresolved scope

Manual source trace through Manager.commitEngineForwarding/finishLifecycle and
cleanup found existing final scalar owner/source/trust/Stop guards and protection
before engine teardown. Those paths are reused; this slice does not install a
second Manager/protection transaction or enable forwarding. No runtime claim.

Original stdio GUI6/7/R1/R2 exchange has a related independent buffer/late-error
boundary; it is not fixed by the named-pipe receipt and stays the next source
consumer. Root QUERY-reference lifetime/last-close versus explicit root control,
post-ACK native retirement and PC-01 traffic cutover remain independent work.
App keeps its original ancestor control; local helper resource cleanup cannot
prove whole-ancestor cleanup. ranBeforeJob is still retained, never cleared.

### Evidence and continuation

Source8 Go files/4 docs; exact touched gofmt8+3 exit0. Manual1 new file/7 original
diffs, actual U1/Broker/App-parent/helper consumers and non-Windows signatures,
source/peer sequencing/partial counts/no-copy-on-error/locks/EOF/Close reviewed.
12 snapshots11 exact originals/1 absent-new; task71 rev589 archived under
bootstrap-v265-cutover-*. Source identities/old PLAN+task byte prefixes and full
goal/tasks0…70/100 OPEN acceptance/56 NOT_RUN checks/decisions/constraints preserved.
No check run/no waiver; gofmt/source/diffs do not prove compilation or behavior.

NEXT extend original stdio final-effect retention without bypassing actual I-O
completion, same private App decision/receipt and retained original process.
Then remaining complete-chain root authority/lifetime/physical retirement and
PC-01 admission integration through existing Manager/protection consumers.
No repeated generic spawner/lifecycle extraction or positive gate from raw data.

Full PC00…09/old9/OS rights/nested breakaway/UAC pre-Job gap/profile continuity/
loaded sections/GC/resources/de-elevation/SDK/ABI/packaging/signed single-EXE/UX/
release/comparative OPEN. Absolute no tests/fixtures/build/compile/vet/typecheck/
product parser/check/doctor/install/gen/SDK/helper/product/crypto/native/browser/
admin/live/network/owner values/other-role/chat/subagents. All gatesfalse;
legacy GUI6/R1/U1/IPC9/Job8/TUN unavailable/schema2 unsupported.

<a id="pc-v266-stdio-effects"></a>

## V2-266 — Stdio final effects and actual I/O retirement, source/unverified

2026-11-06. PC-01/full PC00…09/old9. No acceptance flag opened.

### Existing owner, actual I/O and synchronization

The stdio GUI exchange had the same late-error boundary as V265 named pipes:
goroutine buffers survive cancellation, but final App decision had no original
pointer/counts to distinguish a raw ACK from successful exchange. Existing
ownedPipeReader/Writer now optionally record original raw returned counts and
pending/returned facts BEFORE later source checks. Only writeEntered is atomic
and read while I/O may run; plain counters/flags/buffers require original done.
Invalid underlying read counts fail before io.ReadFull consumes them.

Existing updateGUIExchange embeds that original effect record and attaches it to
its existing ownedPipeIO header before launching the same single I/O goroutine.
No new generic transport, queue or goroutine. Original copied send/receive buffers
stay bounded60 bytes; actual legacy52/ancestor60/frame36 grammar unchanged.
Local Go io source was read: ReadFull can discard a reader error after the full
requested count, so raw facts and final source validation must stay independent.
No Go/product/native execution was used to establish this source observation.

The original parent commit decision stores the final exchange pointer/expected
USE+ACK before launch. Atomic write-entry supplies negative may-have-sent data;
exact raw ACK requires original done plus actual return/count/buffer match.
It is observed after caller return and again AFTER actual original retirement,
so a late I/O result is not silently discarded. Neither datum accepts transfer.
Positive original App grant/source/image/private phase and final receipt checks
stay unchanged. Failed possible USE carries ErrUpdateGUIUncertain; separately,
unconfirmed retirement still joins ErrHelperCleanup. A confirmed local retirement
is not mislabeled as currently unknown just because USE was previously attempted.

### Physical retirement and child-side reachability

ownedPipeIO.retirementReady requires original done and no raw pending call.
Closing done in a panic/Goexit defer records goroutine end only. Process retirement
checks this in normal, already-drained and no-created-process branches before
closing process handles/releasing image. Existing managed pipe Close still runs
to interrupt I/O BEFORE waiting; original physical Wait/empty Job/checked Close
and independent cleanup deadline remain. No timeout becomes a return receipt.

Child stdio also needed original reachability after an interrupted exchange.
Its already-published updateGUINativeLease now retains four fixed original
exchange slots: seed, ARM/GO, OWN/USE, ACK. Slot reservation precedes goroutine
publication; a failed/skipped/unreturned preceding slot prevents another stage.
No native-session replacement/reset, extra stage or fallback to another wire.
Original GUI6/7/R1/R2 dispatch/session/root custody remains; U1/Broker stays in
the distinct named-pipe path. The two child-consumer files were snapshotted before
their first edit; the initial8-snapshot manifest is archived with extension reason.

This barrier is wired to stdio GUI effects now. Other existing worker/helper/TUN
headers still have nil effects and retain their old behavior; they are the next
consumers, not claimed complete. Local role-Job/resource retirement is not whole
ancestor-tree authority. Root QUERY-reference lifetime/last-close/explicit root
control, actual PC-01 traffic cutover and all native acceptance remain separate.

### Evidence and continuation

Source6 Go files/4 docs, no new source file; gofmt4+2+1 exit0. Manual6 original
diffs/production constructors/callers, keyed reader/writer/header literals,
atomic/done publication, late completion/partial counts/retirement and four child
protocol exchanges reviewed.10 exact originals/task71 rev593 archived under
bootstrap-v266-stdio-*. Initial manifest and2-source extension archived too.
Source identities/old PLAN+task byte prefixes/full goal/tasks0…70/100 OPEN
acceptance/56 NOT_RUN checks/decisions/constraints preserved; no check run/waiver.

NEXT attach these same original raw-call/retirement facts to existing worker,
helper-output and native TUN I/O owners, without copying a live record or reading
plain fields before done. Then remaining root authority/lifetime/whole-chain
physical retirement/PC-01 admission integration through existing consumers.
No second lifecycle/spawner or authority from a matched raw frame.

Full PC00…09/old9/OS rights/nested breakaway/UAC pre-Job gap/profile continuity/
loaded sections/GC/resources/de-elevation/SDK/ABI/packaging/signed single-EXE/UX/
release/comparative OPEN. Absolute no tests/fixtures/build/compile/vet/typecheck/
product parser/check/doctor/install/gen/SDK/helper/product/crypto/native/browser/
admin/live/network/owner values/other-role/chat/subagents. Source/gofmt is not
compile/runtime proof. All gatesfalse/ranBeforeJob; legacy GUI6/R1/U1/IPC9/Job8/
TUN unavailable/schema2 unsupported.

<a id="pc-v267-process-io-owners"></a>

## V2-267 — Core process I/O owners share actual-return facts, source/unverified

2026-11-06. PC-01/full PC00…09/old9. All acceptance flags remain false.

### Existing producers and a shared negative boundary

V266 effects are now attached to original workerExchange/helperOutputRead/
nativeTUNExchange headers BEFORE publication/launch. Existing owned reader/writer
and done-fenced header are reused; no live record copy, queue, transport or extra
goroutine. Worker retains a separate finite header/effect cell, not an interior
pointer from that cell into its potentially large reply object. Effects include
only counts/pending/actual returns; source/peer/codec/private grant stay separate.

New engineProcess.ioRetirementReady verifies exact captureIO.done==ioDone, rejects
missing/partial/mismatched registration and requires original raw-call retirement
in addition to goroutine end. Both empty slots mean no registered I/O. Applied
before replacement in worker/helper/TUN/parent-GUI and before native handle/image
release in normal, drained and no-created-process retirement branches. Existing
managed pipe Close can still interrupt I/O, followed by actual original Wait and
Job/file cleanup. No completed channel, deadline or source loss grants cleanup.
Helper-output and TUN completion readers now include original effect/cleanup
failure before releasing bytes. Worker already used that same completedFailure.
Finite frame errors remain distinct from an unconfirmed physical return.

### Necessary delivery-verifier adapter migration

Searching every ioDone assignment found the existing Windows delivery worker
published only ioDone plus a separate buffered error channel. That producer had
to migrate with the shared guard; allowing a header-less exception would weaken
the new invariant. Its original core adapter now publishes ownedPipeIO/effects
before the SAME goroutine, with existing source-fenced reader/writer. The redundant
error channel is removed; completedFailure reads only after original done.

WorkerCall.Exchange requires input io.WriteCloser and calls Close for EOF. The
small private deliveryWorkerInput borrows the exact original process input,
preserves WriteCloser/EOF semantics and records one attempted Close with pending/
actual-return facts. A deferred done cannot manufacture returned Close. Checked
original file-close errors remain in normal process/file retirement too. Header
readiness covers read/write/EOF-Close pending state. No child is replaced on error.

Only windows/backend/core/delivery_worker.go changed at this seam. Delivery/SDK
implementation, API, fixed invocation/digest/HMAC/wire, request/reply caps, Job
policy, original image/zero exit/resource/retirement checks remain. Existing late
cancellation/restriction-prefix decision stays with delivery after confirmed
transport retirement. No nested SDK worker or in-process fallback was added.

The SDK adapter was the6th necessary source file; it was snapshotted before edit.
Initial9-snapshot manifest and extension reason are archived. All five found
production core ownedPipeIO families now attach effects before registration:
worker, helper output, native TUN, GUI stdio, delivery verifier. This is source
coverage of that type, not all operating-system I/O or Windows acceptance.

### Evidence and next source integration

Source6 Go files/4 docs, no new source file. Gofmt5+2+1 exit0; manual6 original
diffs/constructors/callers/all captureIO+ioDone assignments, raw pending/return/
done fences, WriteCloser method set/original file-close and SDK API inspected.
10 exact originals/task71 rev597 archived under bootstrap-v267-ioowners-*.
Own memory recall/checkpoint briefly returned incomplete/conflict data; refreshed
full own status proved unchanged597 and retry accepted598 before any source edit.
Rejected/successful receipts and initial snapshot manifest are preserved.
Source identities/old PLAN+task byte prefixes/full goal/tasks0…70/100 OPEN
acceptance/56 NOT_RUN checks/decisions/constraints preserved; no check run/waiver.

NEXT trace/connect existing original processGUIAncestor custody before positive
Engine/helper/TUN lifecycle and native admission, with fresh original queries
outside App/Manager value locks. Missing/faulted retained root must not select a
legacy fallback or be erased; Stop/negative cleanup must remain reachable. This
query validation is not root control: explicit root authority/lifetime/physical
whole-chain retirement/admission and OS acceptance remain independent work.

Full PC00…09/old9/OS rights/nested breakaway/UAC pre-Job gap/profile continuity/
loaded sections/GC/resources/de-elevation/SDK/ABI/packaging/signed single-EXE/UX/
release/comparative OPEN. Absolute no tests/fixtures/build/compile/vet/typecheck/
product parser/check/doctor/install/gen/SDK/helper/product/crypto/native/browser/
admin/live/network/owner values/other-role/chat/subagents. Source/gofmt is not
compile/runtime proof. All gatesfalse/ranBeforeJob; legacy GUI6/R1/U1/IPC9/Job8/
TUN unavailable/schema2 unsupported.

<a id="pc-v268-root-custody"></a>

## V2-268 — Исходный GUI/root при запуске и положительных worker-командах

2026-11-06. PC-01, source/unverified. Полный PC00…09 и прежние обязательства активны.

- newOwnedProcess один раз сохраняет processGUIAncestor до native acquisition.
  Ошибочный retained lease не стирается и не заменяется legacy-веткой. Режим,
  изначально не имевший inherited ancestor, сохраняет прежнюю отдельную семантику.
- ancestorScope переиспользует существующий QUERY lease. До создания проверяет
  исходную GUI/root custody, перед Resume и после него — тот же original child.
  Обычные worker/helper/delivery/TUN потомки обязаны быть в root И текущем GUI Job.
  Для successor сохранена обратная роль: в root, вне старого GUI Job. Spawn role
  записывается в original owner при единственной попытке, затем не пересчитывается.
- Общий withChild переиспользован для обеих membership-полярностей; raw handles
  не публикуются. Порядок заимствования: original process host → retained lease.
  Вызовы идут под lifecycle/process owner, вне App/Manager value locks. Callback
  panic/nonreturn не превращается в успешную source query или cleanup receipt.
- Worker hello и положительные команды проверяют источник до I/O и после original
  done/completedFailure/контекста, до выдачи reply. Это включает start/switch/probe/
  admit/admit_native/dns_resume/tun_scope_open/tun_native_confirm/current. Ошибка
  помечает process unusable; исходный owner передаётся прежнему retirement пути.
- stop/dns_pause/tun_scope_close не зависят от новой ancestor-проверки. Если owner
  уже unusable, физический Stop/reap остаётся доступен без положительного IPC.
  Pipe interruption, actual Wait, original I/O return, empty Job/image/handle
  retirement по-прежнему отдельные условия. Root QUERY не даёт control authority.

Изменены4 существующих Go-файла и4 docs. Gofmt4+2 exit0; вручную просмотрены
original diffs, все реализации/потребители guiAncestor, capture-before-spawn,
before/after Resume, worker command polarity, original done и negative cleanup,
Manager→Engine call sites и host borrowing.8 exact originals/task71rev601 archive;
source-byte identities и прежние PLAN/task prefixes сохраняются.100 OPEN и
56 NOT_RUN, цель/tasks0…70/constraints/decisions точны; без waiver. Тесты, fixtures,
build/compile/vet/typecheck, продуктовые parsers/checks, SDK/native/browser/admin/
live/network не выполнялись. All gatesfalse/ranBeforeJob, TUN unavailable/schema2
unsupported. Gofmt/source review не доказывают компиляцию или Windows-поведение.

Следующий срез: fresh original ancestor на положительных helper/TUN обменах,
NativeTUNActor.Descriptor и публикации helper/delivery bytes. Release/EOF/revoke/
физическое retirement не должны требовать положительной проверки. Explicit root
control/lifetime/whole-chain physical retirement/admission остаются отдельной
незакрытой работой. OS rights/nested breakaway/UAC pre-Job gap/profile continuity/
loaded sections/GC/resources/de-elevation/SDK/ABI/packaging/signed single-EXE/UX/
release/comparative/full PC00…09/old9 OPEN.

<a id="pc-v269-helper-custody"></a>

## V2-269 — Original ancestor перед helper/TUN/delivery действиями и выдачей результата

2026-11-06. PC-01, source/unverified. Полный PC00…09 и прежние обязательства активны.

- Helper-output и delivery adapter проверяют original child/root custody перед
  регистрацией исходного pipe I/O. NativeTUNActor.Descriptor повторяет эту проверку
  после original image validation и перед возвратом дескриптора для WFP lease.
- TUN initial/go и остальные положительные формы проверяют original ancestor до
  I/O и после original done/completedFailure/context, до выдачи сообщения. Только
  exact release (нет initial/EOF) и EOF (нет initial/command) имеют negative
  exemption. Смешанная форма не может использовать исключение release/EOF.
- Финальные helper bytes и TUN observation проверяют retained root source после
  original child/Job/I/O retirement. Закрытый child не опрашивается. Existing final
  intent/operation guard сохраняется; ошибка обнуляет выдаваемые bytes/observation.
- Delivery сохраняет exact original process за пределами lane.owner, который
  очищает reaper; поздняя captureFailure присоединяется к исходной ошибке. После
  успешного полного retirement root source проверяется в отдельном bounded
  custody context. Это attestation исходного источника, не новый запуск/допуск.
  Оригинальный SDK затем сам обрабатывает late cancellation: verified restriction
  prefix может сохраниться, artifact proofs — нет. SDK/API/wire/HMAC не менялись.
- NativeTUNWFPLease Acquire/Validate/Close callers просмотрены: положительные
  Descriptor queries выполняются под resource owner, вне App/Manager value locks;
  negative Close не запрашивает Descriptor. Revoke/release/EOF и native reaper не
  требуют новой positive custody query и не получают root control authority.

Source3 существующих Go-файла/4 docs; gofmt3 exit0; ручной разбор3 original diffs,
original process binding/defer order, final result clearing, TUN polarity/done,
SDK completion/late-cancel policy, WFP resource locks и negative Close.7 exact
original snapshots/task71rev605 archived. Полная цель/tasks0…70/100 OPEN/56 NOT_RUN/
constraints/decisions и старые PLAN/task byte prefixes сохраняются; no waiver.
Тесты/fixtures/build/compile/vet/typecheck/product parsers/checks/SDK/native/browser/
admin/live/network не выполнялись. Source/gofmt не доказывают Windows-поведение.

Следующий подтверждённый seam: ancestorScope возвращает ErrEngineWorkerUnavailable,
который не входит в App.localRetirementFailure. При успешной очистке worker такой
локальный root fault может попасть в candidate failover/auto-retry. Нужно провести
typed source-custody failure через существующий local barrier, сохранив pure
cancellation отдельно от независимого native source fault/late mixed cancellation.
Explicit root authority/lifetime/whole-chain physical retirement/admission/full
PC00…09/old9/native/SDK/ABI/packaging/UX/release/comparative OPEN. Все acceptance
gatesfalse/ranBeforeJob; legacyGUI6R1U1/IPC9/Job8/TUN unavailable/schema2 unsupported.

<a id="pc-v270-root-fault"></a>

## V2-270 — GUI/root source fault не превращается в отказ канала после cleanup

2026-11-06. PC-01, source/unverified. Полный PC00…09/old9 активен.

- ErrGUIAncestorUnavailable обозначает недоступную original GUI/Job custody.
  ancestorScope сохраняет прежний ErrEngineWorkerUnavailable и добавляет новый
  local marker при independent native/source error, отсутствующем original owner
  или interrupted native wrapper. Успешная очистка child не снимает root fault.
- Pure returned context.Canceled/DeadlineExceeded не получают marker. Простой
  errors.Is(canceled) недостаточен: source failure, joined с поздней отменой,
  остаётся независимым ограничением. Private ancestorCancellationOnly использует
  fixed32 pending slots/64 visits; opaque, empty, cyclic или слишком большое
  дерево не считается чистой отменой. Это bound обхода private error tree, не
  доказанный bound произвольного пользовательского Error/Unwrap метода или ОС.
- Existing App.localRetirementFailure принимает новый marker. Существующие
  Start/candidate/failover/retry/canceled-lifecycle/restart paths используют тот
  же local barrier, без нового policy owner/таймера/словаря channel diagnoses.
- Report-only seams также сохраняют restriction: workerProbeFailure,
  tunnelProtectionFailure, native forwarding commit и terminal outcome. Existing
  CleanupUnconfirmed означает неподтверждённое владение ИЛИ завершение; report
  не является original-resource retirement receipt. reportLifecycleError и
  Manager rollback/switch/reload paths сохраняют локальный запрет преемника.
- No-ancestor legacy semantics, source refs, original I/O facts, negative
  Stop/revoke/reaper и все availability flags не меняются. QUERY остаётся
  проверкой источника, без root control authority или whole-chain cleanup proof.

Изменены6 существующих Go-файлов/4 docs. Gofmt4+2 exit0, вручную просмотрены6
original diffs, все marker producers/consumers, native WithJobs/CheckScope error
shapes, pure/mixed errors/wrappers/bounds и Manager/App report/error/retry paths.
Native commit/terminal projections обнаружены адресным поиском и добавлены как
необходимое расширение: их originals сохранены до первой правки. Initial8 manifest
и pre610 receipt сохранены; extension checkpoint611 принят; final10 originals/
task71rev609 archive. Полная цель/tasks0…70/100 OPEN/56 NOT_RUN/constraints/decisions
и прежние PLAN/task prefixes сохраняются; no waiver. Тесты/fixtures/build/compile/
vet/typecheck/product parsers/checks/SDK/native/browser/admin/live/network не
выполнялись. Source/gofmt не доказывают компиляцию или работу Windows/VPN.

Следующий source-срез: retained root перед GUI stdio и final USE→ACK/private
transfer. exchangeUpdateGUIObserved сейчас проверяет original I/O header, но не
ancestor после spawn. Нужно проследить исходный effect/ACK/decision путь: late
source failure не стирает MAY_HAVE_SENT/физически прочитанный ACK/irreversible
accepted transfer и не даёт прежнему владельцу новую abort authority. Negative
cleanup остаётся отдельным. Root explicit control/lifetime/whole-chain physical
retirement/admission/OS/SDK/ABI/packaging/UX/release/comparative/full PC00…09/old9
OPEN. All gatesfalse/ranBeforeJob/legacyGUI6R1U1/IPC9/Job8/TUN unavailable/schema2 unsupported.

<a id="pc-v271-gui-custody"></a>

## V2-271 — Original GUI/root в stdio и финальной передаче USE→ACK

2026-11-06. PC-01, source/unverified. Полный PC00…09/old9 активен.

- Parent exchangeUpdateGUIObserved проверяет original child/ancestor перед
  публикацией I/O и после original done/completedFailure/context до receive copy.
  Original exchange публикуется до единственной goroutine; late source failure
  не стирает raw write-entry/count/returned/ACK facts на этом же original record.
- CommitGUI проверяет exact captureIO.done==ioDone/actual raw return до App grant.
  Native ancestor query выполняется до value-only accept и непосредственно перед
  Authorized→Transferred CAS, вне App/Manager value locks. Grant.Accept и
  Receipt.Consume остаются value-only. Existing observeFinal/uncertainty/actual
  retirement и accepted branch не получают новых прав abort после transfer.
- Child exchangeStdio проверяет свой original adopted lease до обмена и после
  успешных receive-bearing ARM→GO и OWN→USE. Initial seed остаётся read-only
  pre-adoption. Send-only ACK имеет pre-query; после ACK parent вправе выйти,
  поэтому используется прежний Close→AwaitParent→Verify→final ValidateJob, без
  добавления parent-alive post-ACK query. Mode/stage reservations не сбрасываются.
- Fixed wire/stages/seed sizes, original owner references, no-ancestor legacy,
  child dispatch gates и negative Close/reaper сохраняются. Новых goroutines,
  transport/Job/control authority нет. Root QUERY не является cleanup receipt.

Source3 существующих Go-файла/4 docs; gofmt3 exit0; manual3 original diffs,
все stdio callers, childbootstrap.runHandoff order, native lease repeat validation,
App receipt consumers, original I/O/effect/phase/negative-cleanup paths.7 exact
originals/task71rev614 archived; fullgoal/tasks0…70/100 OPEN/56 NOT_RUN/constraints/
decisions/старые PLAN+task byte prefixes сохраняются; no waiver. Тесты/fixtures/
build/compile/vet/typecheck/product parsers/checks/SDK/native/browser/admin/live/
network не выполнялись. Source/gofmt не доказывают компиляцию или Windows/VPN.

Следующий подтверждённый seam: core RetireCaptureHelpers возвращает nil после
очистки известных lanes, не проверяя retained processGUIAncestor. Повторный Stop
после прежнего source fault может успешно очистить уже пустой worker и допустить
positive protection Release. Нужно после всех actual negative reaps проверить
тот же original root source, сохранив typed local marker. Source query не должна
предшествовать/заменять physical cleanup, создавать fake process owner или
требовать empty root (GUI ещё работает). Existing App retirement/release gate
использовать повторно. Explicit root control/lifetime/whole-chain retirement/
admission/OS/SDK/ABI/packaging/UX/release/comparative/full PC00…09/old9 OPEN.
All gatesfalse/ranBeforeJob/legacyGUI6R1U1/IPC9/Job8/TUN unavailable/schema2 unsupported.

<a id="pc-v272-root-barrier"></a>

## V2-272 — Повторный Stop не забывает недоступный original GUI/root

2026-11-06. PC-01, source/unverified. Полный PC00…09/old9 активен.

- RetireCaptureHelpers сначала пытается negative retirement во всех3 прежних core
  lanes и освобождает их mutex. При busy/retirement failure возвращает прежнюю
  ошибку. Только при успешной очистке проверяет original processGUIAncestor,
  captured один раз для этого borrow. Root fault не препятствует actual cleanup.
- Вынесена существующая typed/cancellation/recovery boundary в private
  guiAncestorQuery. Её используют real engineProcess.ancestorScope и сам original
  retained source. Fake engineProcess/native owner/новый controller не создаются;
  source ref/Job/handles не заменяются, не переоткрываются и не re-adopt.
- После lane cleanup используется прежний workerReapTimeout в independent context.
  Отсутствующая успешная source attestation, включая свой deadline, возвращает
  ErrGUIAncestorUnavailable. Это local barrier uncertainty; pure caller-operation
  cancellation в process query по-прежнему не выдаётся за structural root failure.
  Deadline не доказывает native cancellation/return/whole-chain retirement.
- Legacy без inherited root не получает дополнительных queries/timer. Root не
  требуется быть empty, пока GUI жив; query не подменяет original Wait/pipe/Job/
  image proof. Existing App.retireCaptureHelpers продолжает собственный negative
  GUI/helper cleanup и сохраняет error через Stop/new Start/power/restart/Release.
- Успех повторного Manager.Stop пустого worker не стирает retained failed root и
  не даёт positive Release/new engine. Existing terminal Quit при error и accepted
  GUI transfer branches сохраняются; root query не даёт новую abort authority.

Source2 существующих Go-файла/4 docs, gofmt2 exit0. Manual2 original diffs,
equivalence original process query/error walk, all production barrier consumers,
core lane mutex release order, source-only borrow, Stop→Release error gate,
new-engine/power/restart consumers, App terminal Quit и accepted GUI skip paths.
6 exact originals/task71rev618 archived. Fullgoal/tasks0…70/100 OPEN/56 NOT_RUN/
constraints/decisions/старые PLAN+task prefixes сохраняются; no waiver.
Тесты/fixtures/build/compile/vet/typecheck/product parsers/checks/SDK/native/browser/
admin/live/network не выполнялись. Source/gofmt не доказывают Windows/VPN behavior.

Следующий source audit/integration: найти actual root creator/control owner и
проследить его связь с retained QUERY refs, original GUI/worker/helper tree,
accepted successor и terminal Quit. BrokerLease оставляет Job refs до OS exit;
App.closeAfterLifecycle вызывает Quit даже после Stop error. Нельзя приравнивать
query validation/last-close ожидание к whole-chain physical cleanup или выдавать
QUERY handle за control authority. Сначала переиспользовать существующий native
owner/retirement API; explicit root control/lifetime/whole-chain retirement/
admission/OS/SDK/ABI/packaging/UX/release/comparative/full PC00…09/old9 OPEN.
All gatesfalse/ranBeforeJob/legacyGUI6R1U1/IPC9/Job8/TUN unavailable/schema2 unsupported.

<a id="pc-v273-root-control"></a>

## V2-273 — Root control, QUERY lifetime и независимая negative GUI cleanup

2026-11-06. PC-01, source/unverified. Полный PC00…09/old9 активен.
После возвращения D: сопоставлены 210 ранее названных source/doc/policy identities:
до правки drift отсутствовал. Это сверка файлов, не запуск продукта.

### Владение, установленное по исходникам

| Original owner | Что он действительно держит | Cleanup / граница передачи |
|---|---|---|
| Initial App: brokerParentNative.launch.host | Созданный root Job с control, отдельно helper Job и original Shell process/token | До принятой передачи retire вызывает Terminate для обоих Jobs и, при необходимости, leaf; actual Wait/accounting/Close и ranBeforeJob учитываются раздельно. После elevatedTransferred/finalized Close возвращает ErrElevatedGUITransferred, App не abort. Original refs удерживаются до OS exit. |
| Minimal helper: brokerHelperSession | BrokerLease с QUERY root/helper-role и sync/query App; original child.process контролирует новый GUI-role Job | Ошибка retire пытается вернуть свой child/pipe/pair. Root termination authority отсутствует. Successful helper return0 ведёт к OS exit; это не подтверждение возврата всего root tree. |
| Broker GUI: brokerGUINative.lease | QUERY root/свой GUI role, original helper/App sync/query и identity tokens при bootstrap | AwaitOrigins ожидает оба исходных процесса, закрывает их process/token slots. Root/role QUERY refs остаются до OS exit. Services запускаются после bootstrap; собственный GUI состоит в обоих Jobs. |
| Relaunched GUI: updateGUIAncestorLease | Тот же root QUERY, новый GUI-role QUERY и original predecessor sync/query | Original root передаётся через дополнительный target-owned duplicate; старый role не должен включать successor. Accepted predecessor не восстанавливает abort authority из поздней ошибки. |
| Ordinary worker/helper | Собственные original Process/Job/I-O/image owners под GUI/root ancestry | Их confirmed retirement обязателен отдельно. Проверка root source не заменяет эти receipts и не добавляет прав завершить root. |

Source pointers: windows/backend/core/broker_parent_windows.go (execute/duplicate/
retire), elevated_gui.go (Transferred/Commit/Close), broker_gui_windows.go,
gui_ancestor_windows.go, update_gui.go/update_gui_commit.go; spawnprep/
broker_lease_windows.go (jobHandles/WithJobs/AwaitOrigins) и broker_jobs_windows.go;
cmd/snowden-native-probe/broker_windows.go/broker_spawn_windows.go/broker.go.
Ни один из этих source observations не подтверждает фактические Windows rights,
effective nested Job hierarchy, loaded code, inherited handles или shutdown ОС.

### Подтверждённая source-правка

Ранее App.retireCaptureHelpers возвращал ошибку restartGUI.Close сразу, поэтому
независимый elevatedGUI owner ниже по коду не получал даже попытки cleanup.
Теперь сохраняется core result и выполняются обе известные negative попытки:

- Только untransferred original owners получают Close; accepted GUI не abort.
- Каждая ошибка добавляется к прежним через errors.Join; failed original pointer
  остаётся в App. Успешный Close очищает только свой pointer.
- Core root source failure не препятствует обеим App negative попыткам. Ошибка
  первой GUI cleanup не прячет ни ошибку, ни successful cleanup второй.
- operationMu остаётся владельцем lifecycle; новые locks/goroutines/timers/native
  APIs не добавлены. Existing core barrier, App Stop→positive Release/new-engine/
  power consumers и local cleanup_unconfirmed projection не изменены.
- Terminal closeAfterLifecycle по-прежнему Quits после Stop error. Accepted
  handoff по-прежнему требует parent exit. Это терминальные обязанности, не
  positive cleanup receipt и не доказанная whole-chain physical retirement.

### Отвергнутая гипотеза и следующий gate

Пробная source-правка добавляла root/current-role ActiveProcesses==1 после cleanup.
Manual source review выявил отсутствие необходимого основания: actual main.go
запускает Wails, go.mod закрепляет go-webview2, GUI Job policy допускает descendants
и специально не применяет helper active-process cap. Состав/Jobs renderer children
не установлен. Единственный допустимый process нельзя выводить из факта живого
GUI; правильность или неправильность фактического наследования без native приёмки
не установлена. Singleton predicate способен запретить легитимный GUI lifecycle.

Обе пробные query правки полностью сняты, исходные gui_ancestor.go и
gui_ancestor_windows.go восстановлены byte-exact. Нового API/interface/query/Job
predicate в итоговом source нет; промежуточный код никогда не выполнялся.
Own source-plan-revision.json сохраняет отвергнутое решение и уточнение checkpoint.

Далее: по реальным Wails/go-webview2 и original platform owners описать допустимые
GUI/renderer descendants, их ресурсные/сетевые/lifetime границы и terminal teardown.
После этого определить, какие accounting observations действительно допустимы
перед Release и кто с original rights может вернуть остаток без self-termination
и без убийства accepted successor. Не добавлять PID/job reopen, fabricated owner,
инferred singleton, role BREAKAWAY weakening или ожидание last-handle close как proof.

Итог source1 существующий Go-файл/4 docs; gofmt initial3+final1 exit0, две source
реставрации точны. Manual final1diff, обе negative branches/error composition/
pointer retention, accepted guards, all existing barrier consumers/operationMu,
root-control/QUERY/source native owners/terminal exit.7 originals/task71rev622 archive.
Fullgoal/tasks0…70/100 OPEN/56 NOT_RUN/constraints/decisions/старые PLAN+task prefixes
сохраняются; no waiver. Тесты/fixtures/build/compile/vet/typecheck/product parsers/
checks/SDK/helper/native/browser/admin/live/network не выполнялись.
Root physical cleanup/control transfer/whole-chain admission/full PC/old9/native/
OS/SDK/ABI/packaging/UX/release/comparative OPEN. All bootstrap/bridge/native/chain/
relaunch/parent/helper gatesfalse; ranBeforeJob/GUI6R1U1/IPC9/Job8/TUN unavailable/
schema2 unsupported сохранены. Gofmt/source review не доказывают Windows/VPN behavior.

<a id="pc-v274-renderer"></a>

## V2-274 — Повторный Close не обходит cleanup; WebView2 не выдаёт process owner

2026-11-06. PC-01, source/unverified. Полный PC00…09/old9 активен.

### Разбор закреплённой локальной зависимости

Исследованы доступные в локальном module cache исходники Wails v2.13.0 и
go-webview2 v1.0.22, версии совпадают с windows/go.mod. Сохранены byte identities
10 конкретных файлов: собственные SHA receipts фиксируют прочитанные bytes,
не подтверждают module download/checksum provenance, сборку или загруженный runtime.
Cache/dependencies не менялись, module/SDK/loader/browser не запускались.

| Source path / symbol в закреплённом cache | Установленная source-граница |
|---|---|
| Wails internal/frontend/desktop/windows/frontend.go: Run/setupChromium | Создаёт edge.Chromium и вызывает Embed. OnStartup запускается отдельной goroutine; App не получает original browser process/Job output. |
| go-webview2 pkg/edge/chromium.go: Embed; create_env_go.go; webviewloader/env_create.go | Передаёт environment options и callbacks loader entrypoint, затем создаёт controller. В этой цепочке нет App-owned CreateSuspended/Job/Wait/retirement receipt для browser tree. |
| Wails frontend.go: Quit; internal/app/app_production.go: Run | OnBeforeClose=true отменяет Quit; иначе mainWindow.Invoke(winc.Exit). После main loop вызывается WindowClose и optional OnShutdown. Возврат Quit/цикла/GUI callback не подтверждает physical browser-tree exit. |
| Wails frontend.go: ProcessFailedCallback | Browser-process failure вызывает error dialog и os.Exit(-1), минуя обычный App terminal cleanup. Данный путь не исправлен фиктивным OnShutdown receipt. |
| Wails frontend.go: processRequest/setupChromium | Same scheme/host обслуживает assetserver; прочие origins передаются default WebView2 handler. Global permission выставляется Allow. Это источник browser activity, не утверждение о фактической утечке или protected route. |
| go-webview2 Chromium.Embed / App main.go Windows options | При пустых BrowserPath/DataPath используется runtime/default UDF выбор зависимости. App не задаёт own renderer process scope. Evergreen/runtime version, shared browser/UDF, effective Job hierarchy/rights/inheritance и background networking остаются неизвестными. |

Точная source inventory: .artifacts/memory/bootstrap-v274-renderer-dependency-source-identities.json.
Нет основания считать root ActiveProcesses==1 допустимым invariant. Query-only
root и original parent exit не превращаются в browser child retirement. Не добавлены
PID/name reopen/kill, ослабление sandbox/code integrity/Job flags, глобальные
environment overrides, новый browser fork или сторонняя lifecycle реализация.

### Исправленный обход повторным закрытием

Ранее OnBeforeClose после первого closing.Swap(true) возвращал false для второго
нажатия. Wails трактует false как разрешение main-loop exit: в исходниках повторный
window Close мог обойти ещё работающую closeAfterLifecycle task.

Теперь closing остаётся restriction/intent, а private quitRequested под App.mu
означает только явно запрошенную терминальную фазу:

1. Existing pending restart/release обрабатываются первыми. Непринятый handoff
   сохраняет closeRequested; accepted launched successor допускает parent exit.
2. Без explicit quitRequested повторный ordinary Close возвращает true. Новая
   cleanup task не создаётся; первый owner продолжает исходный Stop/cleanup.
3. requestQuit после существующей задержки берёт App.mu, один раз выставляет
   quitRequested, освобождает mutex, затем вызывает runtime.Quit. Повторный
   request сохраняет первый native request и его existing fallback.
4. Терминальный callback теперь разрешает выход. Явный request работает и если
   closing ещё false; он не должен запускать ещё один Stop вместо выхода.
5. Close-after-Stop-error, accepted-custody late-error exits, nil-context fallback,
   прежние 300/700ms caller delays и 3s fallback сохранены. Ни mutex wait, ни
   runtime.Quit/native return не объявлены жёстко ограниченным временем или
   successful process/Job/browser cleanup. Quit flag не positive Release/Ready.

Все7 production requestQuit callsites просмотрены: ordinary close, mode switch,
restart/elevated partial accepted receipt, legacy update, pair update и recovery.
Все3 closeAfterLifecycle producers сохраняют original registered close:
OnBeforeClose, restartHandoff.release, protectionReleaseCommit.finish.
Native/UI calls не выполняются под новым App value lock. Fields/flag не exported;
новых Wails bindings/native APIs/dependencies не требуется.

### Оставшиеся source и acceptance gates

Следующий P0: Wails вызывает OnStartup асинхронно, а App.startup безусловно пишет
closing=false и создаёт log/tailer/storage/network services. Проследить original
startup task и registration terminal Close; нельзя объявлять новый terminal flag
общим устранением startup/close resource race. Нужен один владелец фактического
startup completion/negative cleanup, без преждевременного снятия closing intent.
Actual browser supervision/retirement, crash path, environment permissions,
external/background network, CSP/navigation, root-control transfer и OS cutover
остаются отдельными открытыми gates. Снятие защиты и принятый successor не получают
новой authority из Quit flag, environment callback, controller или source query.

Source1Go4docs, gofmt1 exit0, manual1 original diff/all callers/locks/phase order/
duplicate/window/accepted/pending/delay/no-context/fallback paths;5 exact originals,
task71rev627 archive,10 local dependency source identities. Fullgoal/tasks0…70/
100 OPEN/56 NOT_RUN/constraints/decisions/старые PLAN+task prefixes сохранены;
no waiver. Тесты/fixtures/build/compile/vet/typecheck/product parsers/checks/SDK/
helper/native/browser/admin/live/network не выполнялись. All bootstrap/bridge/
native/chain/relaunch/parent/helper gatesfalse; ranBeforeJob/GUI6R1U1/IPC9/Job8/
TUN unavailable/schema2 unsupported. Full PC/old9/native/OS/SDK/ABI/packaging/UX/
release/comparative OPEN. Source/gofmt не доказывают Windows/VPN behavior.

<a id="pc-v275-startup"></a>

## V2-275 — Startup/Close разделяют serializer; original log writer не заменяется

2026-11-06. PC-01, source/unverified. Полный PC00…09/old9 активен.

### Исправляемые source-дефекты

Wails asynchronous OnStartup установлен source trace V2-274. Прежний startup
безусловно сбрасывал closing=false и создавал writer/tailer/vault/tray/producers
без lifecycle serializer. Close мог закончить свою очистку раньше этих writers.
Поздняя присваиваемая целиком App.state также не учитывала зарегистрированный
параллельно Start/Stop. В log writer send под App.mu не был согласован с close
канала без mutex; goroutine читала mutable App.logFileCh вместо исходного ch.

### Использованный existing owner

- startup вызывает private prepareStartup, который берёт existing operationMu
  до первой synchronous acquisition. startupAttempted допускает один original
  scope; повтор не заменяет context/resources и не закрывает optional done снова.
- Original updateGUIStartupDone захватывается один раз. Его close обозначает
  retirement preparation stack, включая error/panic/Goexit unwind, не happy result,
  весь Wails callback, child/browser/native readiness или успешный storage commit.
  DOM activation по-прежнему требует отдельный storage-ready и прежние gates.
- closing не снимается. Если terminal Close уже зарегистрирован до scope, setup
  пропускается; actual App context публикуется под App.mu для существующего Quit.
  Если Close приходит во время setup, его Stop ждёт тот же operationMu, затем
  выполняет existing negative cleanup уже после завершения synchronous creation.
- ctx/vault/state публикуются под App.mu. Disk/OS recovery/migration/Seed/retained
  marker observation остаются вне value lock. Recovery→migration→Seed order прежний.
- Initial state baseline применим только к original startup registration. Новый
  Start/Stop может зарегистрироваться, пока ждёт operationMu; startup не заменяет
  его state/protection intent. Sticky installed-image restriction не снимается.
- Late closing/context loss не разрешает startup state publication/services;
  storage-ready при closing/terminal/canceled остаётся false. Explicit Start без
  live App context возвращает cancellation до нового forwarding registration.
- prepareNetworkServices использует тот же operationMu и свежий ctx/image/
  closing/terminal guard, затем существующие netWatch/bgprobe/watchdog producers.
  Initial startup и DOM-ready consumer проходят один seam. Auto-connect выполняется
  после unlock, с прежним original registration; recursive operationMu не введён.

### Original log writer и actual drain

- Под App.mu один раз публикуются original channel/done; goroutine захватывает
  эти local values. Writer после shutdown не создаётся заново: original done
  остаётся в App, closing/terminal также отвергают новое создание.
- appendLogLocked и close+detach используют один App.mu: send не пересекает close.
  stopLogWriter ждёт original done вне mutex. Повторный Stop не закрывает канал
  снова и ждёт тот же original task, а не созданный replacement.
- После detach лог не запускает synchronous disk fallback рядом с ещё текущим
  drain. Late records остаются в memory/UI. Синхронный fallback сохранён только
  до первого опубликованного writer; channel capacity/batch/ticker прежние.
- done означает task-return fence. Existing persistLogLines игнорирует write/
  Flush/Close errors, durable receipt/Sync не добавлены. Эти ограничения и время
  flush/mutex/native calls остаются открытыми; timeout не имитирует completion.

### Manual source review, без исполнения продукта

| Сценарий | Рассмотренная source-граница |
|---|---|
| Close до startup lock | closing остаётся true, поздняя preparation не создаёт ресурсы. |
| Close во время setup | Original operationMu serializes setup с Stop; negative owners не заменяются. |
| Close между setup и producers | Новый producer scope повторяет current terminal/lifetime/image guard. |
| Close во время producer scope | Stop ждёт serializer; existing terminal path повторяет stopNetWatch и stopBgProbes после Stop. Physical task return не выведен из stop signal. |
| Start/Disconnect при startup | Queued registration/state сохраняется; automatic resume не присваивает чужой initial intent. |
| Restricted DOM callback | Original optional done/storage-ready/activation gates сохранены; producer mutex не удерживается при auto-connect. |
| Log send/close/duplicate stop | Один mutex, detached original ch, same captured done; wait вне value lock. |
| Late logging during drain | Memory/UI only после original writer; competing disk fallback не создаётся. |

Просмотрены callers prepareStartup/startup, оба startNetworkServices consumers,
Start/startForStartupIntent/beginOperation, stopCoreFor/registerStopLocked,
closeAfterLifecycle/accepted mode restart, log writer producers/stops/appendLog,
DOM wait/activation, netWatch/bgprobe/watchdog setup, recovery/retained-image paths.
Первый большой patch был отклонён по несовпавшему comment context; bytes оставались
исходными, затем применены адресные hunks. Product code при этом не выполнялся.

Source1 существующий Go-файл/4 docs; gofmt1 file в2 batches exit0, manual final1diff;
5 exact originals/task71rev631 archive. Fullgoal/tasks0…70/100 OPEN/56 NOT_RUN/
constraints/decisions/старые PLAN+task prefixes сохраняются; no waiver.
Тесты/fixtures/build/compile/vet/typecheck/product parsers/checks/SDK/helper/native/
browser/admin/live/network/owner values не читались и не выполнялись.

Далее: original task return/negative stop ownership у engine log tailer, background
probe loop и tray. Сейчас stopEngineLogTailer/stopBgProbes только сигналят; tray
Exit вызывает systray.Quit при пустом onTrayExit. Нельзя называть stop signal
actual producer completion или UI Exit — завершением всего приложения. Browser
supervision/crash/permissions/CSP/egress/root physical retirement/admission/full PC/
old9/native/OS/SDK/ABI/packaging/UX/release/comparative OPEN. All bootstrap/bridge/
native/chain/relaunch/parent/helper gatesfalse; ranBeforeJob/GUI6R1U1/IPC9/Job8/
TUN unavailable/schema2 unsupported. Source/gofmt не доказывают Windows/VPN behavior.

<a id="pc-v276-producers"></a>

## V2-276 — Original producer joins и «Выход» через App lifecycle

2026-11-06. PC-01, source/unverified. Полный PC00…09/old9 активен.

### Tailer: отмена отдельно от возврата

- До goroutine App под своим mutex публикует original engineLogRetired. Original
  cancel channel из NewApp не заменяется; worker захватывает его и свой done.
- Повторный/late start, включая start после уже закрытого cancel, отклоняется.
  Partial scheduling сохраняет original owner/done, а не выдаёт completed.
- stopEngineLogTailer закрывает original cancel под App.mu и ждёт same done вне
  value lock. Нельзя вывести task return из закрытого cancel или возвращённого
  time.After. Engine file read/Close success не подменяются goroutine done.

### Background probes: original task и App cancellation

- Один backgroundProbeTask хранится в App до terminal exit: ctx от live App,
  cancel, original stop/done и one-use stop. Он опубликован до scheduling.
- Global registry по-прежнему хранит budgets/старые process bookkeeping slots;
  очистка этих value maps не заменяет retained App task и не передаёт abort
  authority другому App. Existing test helper bytes не менялись.
- Loop waits и rounds учитывают original ctx; диагностический admission и dial
  используют его вместо context.Background. Stop отменяет тот же admission/dial,
  закрывает тот же stop, затем наблюдает actual original loop done.
- После caller/task cancellation или closing результат dial не превращается
  в новую FAIL/OK health observation. Собственный diagnostic timeout остаётся
  unknown по прежней policy, а не доказательством блокировки/порта/server death.
- QUIC skip, budget/epoch/weak-ordering policy сохранены. На естественном возврате
  task cancel освобождает его original context перед done. Прежний conn.Close
  error не стал native cleanup proof; per-socket custody/error receipt ещё открыт.
- Прямой private round без опубликованного task всё равно требует live App ctx;
  nil lifetime не заменяется Background. Public API/fixture не добавлены.

### Один existing join и terminal consumers

joinAppProducer — узкая обвязка существующего joinTerminalNetworkTask, не второй
controller/task runner. Используется2s бюджет ожидания, как у observer cleanup.
При error/deadline повторно читается тот же original done; только его фактическое
закрытие даёт task-return. Иначе остаётся errAppProducerPending вместе с context
error. Owner/каналы/ctx не обнуляются и не заменяются после timeout.

Оба production consumers tailer и оба consumers bgprobe — ordinary terminal
closeAfterLifecycle и accepted mode restart — читают ошибки join и пишут warning.
После pending join сохраняется обязанность terminal/accepted parent exit;
confirmed task completion не выдаёт Ready/Release, late error не возвращает abort
authority над successor. Wait не идёт под App value lock; native/file/scheduler/
context-cancel duration не объявлены жёстко ограниченными этим timer.

### Tray Exit исправлен, native tray retirement пока открыт

Ранее пункт «Выход» вызывал только systray.Quit, при пустом onTrayExit: это
закрывало tray subsystem, но не запускало App Stop/cleanup/exit.
Теперь action снимает App context под mutex, вызывает OnBeforeClose вне него:

- Если обычная cleanup уже запущена/зарегистрирована, она остаётся owner выхода.
- Pending restart/release получает тот же closeRequested, как window callback.
- Если accepted handoff/явная terminal phase уже разрешают exit, action вызывает
  requestQuit(0). Action goroutine после передачи управления возвращается.
- Прямой systray.Quit больше не выдаётся за завершение приложения. Нового WFP/
  proxy/system setting action, backend binding или native protocol нет.

Прочитаны pinned local systray v1.2.2 sources, совпадающие с go.mod; сохранены2 SHA
receipts (systray.go/systray_windows.go), dependency/cache не менялись/не запускались.
Register вызывает onReady на отдельной goroutine, onExit идёт в native event loop;
Run return и ready/actions/relay return не эквивалентны. Quit использует global
once и PostMessage, игнорируя native result. Это source constraints, не runtime
proof и не supply-chain checksum verification. Exact inventory в own artifacts.

### Evidence и следующий P0

Manual4 original diffs, все production start/stop/join consumers, App/bg registry
lock order, initial/late/duplicate Stop, actual done versus cancel/deadline,
diagnostic cancellation versus failure classification, accepted/ordinary terminal
paths, tray pending/authorized close routing, existing generic join semantics.
Source4Go4docs/gofmt4 затем2 touched exit0;8 exact originals/task71rev635 archive.
Fullgoal/tasks0…70/100 OPEN/56 NOT_RUN/constraints/decisions/старые PLAN+task prefixes
сохраняются; no waiver. Тесты/fixtures/test helpers не добавлялись/не менялись/
не запускались; build/compile/vet/typecheck/product parsers/checks/SDK/helper/
native/browser/admin/live/network не выполнялись; owner values не читались.

Следующий срез: original tray session, native Run entry/return versus onReady,
actions/relays/poll loop, безопасная работа с Quit-once/readiness/OS-thread и
negative stop/join. Отдельно remaining update/recovery и partial-accepted
requestQuit callers пока не используют новые producer joins: их terminal
обязанности нельзя скрыть под общим claimed cleanup. Socket/file close errors,
log flush durability, Wails/browser/root tree/control/actual cutover/native/OS/
SDK/ABI/packaging/UX/full PC/old9/release/comparative OPEN. All bootstrap/bridge/
native/chain/relaunch/parent/helper gatesfalse; ranBeforeJob/GUI6R1U1/IPC9/Job8/
TUN unavailable/schema2 unsupported. Source/gofmt не доказывают Windows/VPN behavior.

<a id="pc-v277-tray"></a>

## V2-277 — Одна original tray session, native Run отдельно от callback/workers

2026-11-06. PC-01, source/unverified. Полный PC00…09/old9 активен.

### Реальное владение до scheduling

- startTray под processTray mutex публикует один exact App-bound traySession до
  первого go. Global systray callbacks/Quit-once не позволяют безопасно заменить
  сессию после ошибки/timeout/Run return. Другой App не получает cleanup authority.
- Original stop/readyDone/runDone/aggregate done и worker group не переоткрываются
  и не подменяются. Native entry, pre-native skip, actual Run return, callback
  entry/onExit observation и Quit attempted/returned хранятся отдельно.
- New goroutine закрепляется LockOSThread до systray.Run: package init закрепляет
  startup thread, а не эту goroutine. Registration и pump идут на одном dedicated
  thread; он остаётся locked при выходе goroutine, чтобы не возвращать неизвестный
  native thread в pool. Local runtime/proc.go описывает termination такого thread;
  actual Windows destruction/termination приёмкой не подтверждены.

### Порядок negative retirement

1. Original stop или App context cancellation будят coordinator. Native onExit
   только фиксирует observation и закрывает own stop; не вызывает App/native Quit
   под value locks и не выдаёт DestroyWindow receipt.
2. Coordinator ожидает original onReady stack return. Все WaitGroup.Add происходят
   только внутри этой callback, поэтому readyDone закрывает фазу добавления до
   Wait. Callback return не означает успешную инициализацию UI/нативных ресурсов.
3. Relays имеют cancel/context cases и при чтении ClickedCh, и при потенциально
   заблокированном chSelected send. Closed menu channel завершает relay. Actions
   повторяют stop guard перед существующими App/registry actions; poll использует
   interruptible ticker wait вместо unconditional Sleep. Все workers отмечают
   фактический stack return, dispatcher/poll не остаются бесконечными циклами.
4. После callback и workers coordinator делает не более одной попытки opaque
   systray.Quit, только если Run ещё не вернулся и onExit не observed. Так наша
   остановка не уничтожает window одновременно с нашими menu update callbacks.
   Guards не являются atomic HWND identity/native object lifetime proof.
5. Затем ожидается original runDone и actual runReturned. Aggregate закрывается
   лишь после всей наблюдённой последовательности. Deferred stack retirement
   без actual Run return не превращается в успешный aggregate.
6. Если cancellation выиграла ДО native registration, runSkipped позволяет
   закончить no-acquisition branch. readyDone при этом искусственно не закрывается:
   callback/workers никогда не создавались. Module init/loader/ranBeforeJob не erased.
7. Missing readiness, blocked worker/native call или nonreturn оставляют exact
   session pending. joinAppProducer использует прежний2s wait/final same-done read;
   timer не закрывает aggregate, не сбрасывает owner и не запускает replacement.

### Все terminal consumers через один seam

Все8 production requestQuit callsites (ordinary close, mode, legacy/pair update,
recovery, two partial-accepted exits и tray) теперь выполняют stopTray перед
публикацией quitRequested и Wails Quit. App/global value locks не удерживаются
вокруг wait/native calls. Повтор после уже запрошенного Quit — value no-op;
concurrent pending requests наблюдают тот же owner и тот же coordinator, не новый.

Pending error логируется как warning; explicit terminal/accepted parent exit
остаётся обязательным. Нет positive Release/Ready, новой abort authority над
successor, повторного native Quit, глобального reset или PID/window reopen.
Чужой App не может закрыть retained session. Existing caller delays/fallback и
pending restart/release closeRequested guards сохранены.

### Прочитанные источники и границы доказательства

Сохранены SHA identities local systray v1.2.2 systray.go/systray_windows.go и
установленного Go runtime/proc.go; source inventory в own artifacts. Dependency/
runtime cache не менялись/не запускались. SHA/read path не checksum authenticity,
не доказательство выбранного runtime/binary/native ABI или фактического OS поведения.
Opaque Quit возвращает void и может не раскрывать PostMessage result/global-once
effects; Run может вернуться по GetMessage error. Никакой Go aggregate не доказывает
успех DestroyWindow, HWND identity/remaining library resources, actual thread exit,
Wails/WebView2/root cleanup. Эти native gates остаются открытыми.

Manual2 original diffs, all session/run/coordinator/ready/action/relay/poll paths,
pre-native skip/late-ready/missing-ready/early native return, duplicate stop/foreign
App, Add-before-Wait seal, value locks outside native/join, actual/deferred return
distinction, all8 terminal callsites и accepted/pending Close precedence.
Source2Go4docs/gofmt2 exit0;6 exact originals/task71rev639 archive +3 source SHA.
Fullgoal/tasks0…70/100 OPEN/56 NOT_RUN/constraints/decisions/старые PLAN+task prefixes
сохраняются; no waiver. Tests/fixtures/test helpers не добавлялись/не менялись/
не запускались; build/compile/vet/typecheck/product parsers/checks/SDK/helper/
native/browser/admin/live/network не выполнялись; owner values не читались.

Следующий P0: все terminal update/recovery/partial-accepted callers должны также
иметь общий tailer/bg/logger retirement path. Сейчас только ordinary/mode consumers
явно используют V2-276 joins; stopLogWriter ждёт flush без bounded error receipt,
что нельзя молча добавить перед обязательным accepted parent exit. Нужны original
logger task/file outcomes и pending/terminal различие, без fake durability. Socket/
file Close, native opaque tray limits, browser/root/control/actual cutover/full PC/
old9/native/OS/SDK/ABI/packaging/UX/release/comparative OPEN. All bootstrap/bridge/
native/chain/relaunch/parent/helper gatesfalse; ranBeforeJob/GUI6R1U1/IPC9/Job8/
TUN unavailable/schema2 unsupported. Source/gofmt не доказывают Windows/VPN behavior.

<a id="pc-v278-terminal"></a>

## V2-278 — Общая отмена terminal producers и исходный писатель журнала

2026-11-06. PC-01, source/unverified. Полный PC00…09/старые9 этапов активен.

### Причина и переиспользование

До этого V277 централизовал tray, но tailer/bg/logger retirement выполнялся лишь
в части callers; stopLogWriter ждал бесконечно, а rotation/Write/Flush/Close errors
не учитывались. Последовательный fallback после Wails Quit зависел от его return.
Ранние appendLog всё ещё делали файловый I/O под App.mu. Existing generic original-
task join, same-done re-read и producer owners переиспользованы; cancel выделен из
join без второй actor/registry. BoundedWriteSlot публикует/заменяет файл, не имеет
append API: app_log_writer.go содержит прежний App logger с отдельным outcome.

### Исходная реализация

1. requestQuit резервирует quitTaskStarted под App.mu до scheduling; duplicate
   callers не создают второй worker/backstop. Closing установлен до capture
   producers; tray rechecks closing под processTray publication lock. Timer
   создан до retirement worker и не зависит от producer/file/UI/нативных вызовов:
   caller delay +2s budget +3s grace. Он не доказывает фактический срок OS exit,
   cleanup или progress scheduler; ожидание caller/App.mu до него не покрыто.
2. Все8 callers: ordinary Close, mode restart, legacy update, pair update,
   recovery, partial accepted ordinary/elevated GUI и tray accepted exit проходят
   одну процедуру. Exact net/DNS/tailer/bg/tray отменяются все до первого join.
   Shared2s context и final original-done re-read сохраняют pending owner; logger
   drain использует тот же deadline. Foreign App tray/net не отменяются. Early
   network cancel перед обычным Stop сохранён. Manager.Stop/Release/positive Ready
   здесь не вызываются, уже принятая передача не получает abort из cleanup error.
3. App сохраняет original appLogWriter/ch/done. Early history (последние500 строк)
   переигрывается в input1024 до scheduling; batch64/250ms сохранён. Send/close
   сериализованы App.mu; disk I/O под ним удалён. Full queue/known failed writer
   увеличивают dropped, memory/UI сохраняются. After stop late logs только memory/
   UI, не обещание их durability. Никакого replacement/restart writer после error.
4. Process append slot публикует original batch до path/Mkdir/rotation/Open.
   Open entry/return, actual Write/Flush/final Sync, one-attempt original Close и
   ошибки сохраняются отдельно от worker done. Partial file/original batch при
   uncertainty удерживаются, retry запрещён. Rotation error прекращает append;
   64MiB threshold до batch не является hard disk quota или interprocess lock.
5. Ordinary batches Flush/Close; Sync только final drain. Empty final после prior
   data открывает прежний path без CREATE. Это не directory durability, identity/
   ACL custody, atomic rotation, protection от стороннего writer или power-loss
   proof. Row/queue limits не ограничивают длину LogLine.Text или memory bytes.
6. stopLogWriterContext ожидает original done с final re-read, только потом читает
   result и dropped count. Timeout сохраняет исходные writer/file; quitRequested
   и terminalRetirementErr — terminal permission/diagnostic, не cleanup success.

### Допустимое подтверждение и ограничения

Только source/manual: просмотр6 diffs, всех8 callers, startup publication/late
start, lock order, shared deadline/cancel-before-join, original done, duplicate/
foreign owner, empty/failing/partial logging и accepted-error paths. gofmt6 exit0.
10 before snapshots:9 существовавших файлов +1 ранее отсутствовавший путь; это
не10 прежних исходников. Предыдущий task71rev643 сохранён отдельно. Source-byte
SHA и ledger preservation — учёт, не проверка поведения приложения.

Тесты/fixtures/build/compile/vet/typecheck/product parser/check/doctor/install/
gen/SDK/helper/crypto/native/browser/admin/live/network не добавлялись/не запускались.
100 acceptance OPEN и56 checks NOT_RUN без waiver; all gatesfalse/ranBeforeJob,
GUI6/R1/U1/IPC9/Job8, TUN unavailable/schema2 unsupported сохраняются. Нативная
retirement tray/browser/root, original socket/file closure, hard latency/resource
bounds, directory/power-loss/interprocess durability и PC00…09/старые9 открыты.

Практический урок: cancel request, Go task return и successful I/O/Close — разные
факты. Единый context ограничивает join observation, но не прерывает зависший OS
вызов; независимый terminal backstop не следует ставить после такого вызова.

Далее: original tailer file I/O ownership и ordinary Close/Stop до reservation;
затем root/browser/admission cutover в полном PC-01. UX/SDK/release/сравнительная
приёмка остаются частью полной цели; работа с исходниками не закрывает эти gates.

<a id="pc-v279-tailer"></a>

## V2-279 — Ограниченное чтение tailer и общий original log file owner

2026-11-06. PC-01 source/unverified; полный PC00…09/старые9 активен.

### Причина и компоненты

Прежний ReadString мог накапливать строку без границы, проход читал до EOF без
byte/row budget, а deferred File.Close игнорировался. Original task done сам по
себе не показывал successful file retirement. V278 append writer уже содержал
нужную Open/Close логику: она выделена в log_file_owner.go и переиспользуется обеими
операциями. Это частный diagnostic file owner, не native filetxn/ACL custody.
Existing filetxn.WindowsRead не даёт Seek для сохранённого cursor; replacement
BoundedWriteSlot не является append API. Общие backend packages не изменялись.

### Исходная реализация

1. Existing engineFileLogPath/classifier/readNewEngineLines/start/cancel/stop/
   sleepTill перенесены в engine_log_tailer.go; signature read helper и names
   сохранены, tests/helpers не менялись. App.engineLogState вместе с exact done
   публикуются до goroutine. Ошибка/panic/unfinished stack сохраняет failure,
   original state/file не заменяется. Result читается только после original done.
2. originalLogFile сохраняет original os.File до actual one-attempt Close и
   различает Open entry/return, pending I/O, Close attempt/return. Unknown Open/
   I/O не разрешает Close по догадке; Close error не разрешает retry. Append
   writer использует тот же owner, включая pending Write/Flush/Sync. Его прежний
   sticky process append slot, final-only Sync и terminal budget сохранены.
3. Tailer публикует read operation до Open. Missing file до создания — expected
   no acquisition; другие Open/Stat/Seek/Read/Close errors завершают diagnostic
   task с сохранённым outcome. Non-regular file отклоняется после actual Open/
   Stat; это не защита от блокирующего Open named pipe и не hard OS-call timeout.
4. LimitReader допускает256KiB file reads за pass; ReadSlice использует64KiB
   line buffer; loop завершает не более128 rows/pass. Oversized row пропускается
   bounded chunks с одним счётчиком на строку и warning (warning не completed
   file row). Ordinary partial row сохраняет начальный offset для следующего
   pass. Existing blank/debug/trace/consecutive-duplicate behavior сохранено.
5. Cursor/last/discard reset при observed shortening или replacement metadata
   hint/SameFile. Metadata не native identity/ACL lease; same-file truncate+regrow
   между polls может быть пропущен. Нет гарантии полной доставки журнала при
   быстрых rotation/backlog/UI queue drops/cancel; whole-App memory/disk quota,
   physical Close/directory/interprocess/power-loss properties не доказаны.
6. All-terminal coordinator и stop wrapper join exact original done, затем
   учитывают read/Close failure и oversized omissions. Диагностическая ошибка
   не меняет channel health/intent/Ready/protection и не создаёт нового reader.
7. Ordinary Close отменяет tailer/bg до network join и Stop. Existing register/
   operationMu/Manager.Stop/helper/WFP ordering сохранён: forced exit до actual
   Stop мог бы обойти resource/protection obligations. Pre-request mutex/Stop
   waiting остаётся отдельным открытым gate, V278 backstop его не покрывает.

### Что действительно подтверждено

Только manual source5 diffs/callers/lock/done/error/cancel/pending I/O/partial-row/
oversized/byte-limit/rotation paths и gofmt5 exit0.9 before snapshots:7 existing
+2 absent paths; previous task71rev647 archive. Source-byte/ledger accounting не
compile или execution proof.100 acceptance OPEN/56 checks NOT_RUN exact/no waiver.
Тесты/fixtures/build/compile/vet/typecheck/product parser/check/doctor/install/gen/
SDK/helper/crypto/native/browser/admin/live/network не добавлялись/не запускались.

All gatesfalse/ranBeforeJob/GUI6R1U1/IPC9/Job8/TUN unavailable/schema2 unsupported.
Native handle/Close/path/ACL/same-file races, hard OS timing/quotas, original
ordinary Stop wait, whole-chain root/browser/control и main original-helper/WFP
admission остаются открытыми; supporting diagnostics не заменяет PC-01 acceptance.
Практический урок: ограничение bufio buffer не ограничивает ReadString; нужны
ограниченные fragments и общий byte budget, а task done не заменяет actual Close.
Далее вернуть фокус на main PC01 admission/control и сохранить весь PC00…09/old9/
SDK/ABI/OS/UX/packaging/release/comparative объём, без переноса старого PASS.
