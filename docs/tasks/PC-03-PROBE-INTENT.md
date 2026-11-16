# PC-03 — исходная операция защищённого probe

V2-223, 2026-11-05, Windows, **source/unverified**. Частичный срез PC-03/05;
полный [PC-00…09](../planning/knowledge-application/PC-PLAN.md) и прежние
FR/TH/OFF/COM/RQ/TR остаются открытыми. Тестирование запрещено владельцем.

## Дефект в исходниках

`appProtectedProbe.Run` получал `a.operationID` и глобальный Manager при начале
выполнения. Между созданием проверки и её выполнением мог зарегистрироваться
новый Start. Проверка ещё применённого старого engine тогда получала новый
intent; совпадение этого номера при завершении не обнаруживало подмену.

Обратная крайность — навсегда привязать retained lifecycle-probe к исходному
сетевому пути или поколению Manager — мешает свежему rollback прежней конфигурации.
Эти два режима требуют разных источников идентичности.

## Реализация

- `windows/protection.go`: `beginOperation(id)` добавляет private App/intent value
  в исходный network-bound контекст. Потомки сохраняют его; чтения текущего
  номера для подстановки отсутствующего scope нет.
- `windows/probe_subject.go`: отдельный value-only subject и общий короткий guard
  с порядком `runtime → Manager → App → epoch`. Factory, probe, native/IPC,
  события, отмена и очистка выполняются после освобождения этих locks.
- `protectedOperationProbe(ctx, cc, manager)` связывает неизменяемые config/factory
  с исходным intent и конкретным Manager. На каждом lifecycle Run захватываются
  активное поколение этого Manager, его engine и **путь из исходного caller ctx**.
  `Start`, selector candidate/rollback и Reload используют этот конструктор.
  Start без original operation ctx отклоняется до Stop прежнего Manager и
  подготовки преемника; утрата private scope не повод останавливать живую сессию.
- Retained previous probe на Reload rollback допускает новую полную проверку
  прежнего config под новым caller ctx того же App intent/Manager. Старые
  report/path/lease не становятся результатом новой проверки. Чужой/отменённый
  ctx, новый App intent или другой Manager отклоняются.
- Обычный `protectedProbe(cc)` до Run сохраняет applied config pointer,
  Manager/engine, действительное поколение, path/epoch и исходный intent из
  `Manager.LastProbeReport`. Старый report используется только для identity
  той же сессии/Manager: его время, PASS и network epoch не обновляются.
  После rejection без замены engine поколение может быть новее report;
  новая проверка заранее захватывает фактическое поколение Manager.
- `Run` проверяет subject перед factory, после её возврата и при завершении.
  Report получает только захваченные номера. Отозванный subject даёт stale и
  не публикует успех в следующую операцию. Periodic binding нельзя использовать
  как lifecycle; lifecycle binding нельзя незаметно превратить в periodic.
- `BeforeEngineChange` сначала очищает прежний preparation slot, затем требует
  текущий исходный lifecycle subject. Native commit потребляет slot до любого
  исхода и повторяет guard перед положительными действиями.
- `native_tun_lifecycle.go` получает intent и path из исходного ctx. Проверяет
  тот же path pointer, down/power fence и epoch, включая случай power gap без
  увеличения счётчика. Обычный probe может измерять новую сеть заново; его
  старый результат не переносится на неё.

`LifetimeRemaining` по-прежнему проверяет доверие отдельно от завершившегося
App operation ctx: истечение срока обязано остановить оригинальный engine.
Независимые negative cleanup/Stop/restrict deadlines не получили положительный
subject gate. Новые горутины, таймеры, SDK, wire/API поля и admission grants
не добавлялись. Native acceptance=false, IPC9/Job8 прежние, TUN unavailable,
schema2 installation unsupported.

## Границы и оставшаяся приёмка

Ручной просмотр охватил отложенный Run до/после нового Start, смену applied
pointer/Manager/generation, manual/watchdog/net-change/fast reprobe callers,
fresh rollback с retained previous probe, rejection без новой проверки,
отзыв исходного path и независимость lifetime/negative cleanup. Это анализ
исходников; он не доказывает ни воспроизведение дефекта, ни его runtime-исправление.

- Не выполнены compile/API/fixtures/race/queue/Stop/Close/profile/power/network/
  native commit/OS/resource проверки. Прежние private helper signatures
  сохранены, но `Background` без настоящего operation scope больше не
  разрешает lifecycle; старые fixtures не изменялись и совместимость не принята.
- Guard не держит locks во время factory/engine/native I/O. Не заявляется
  атомарность всех decision→mutation участков или мгновенная отмена I/O.
  Проверка исходного subject и отбрасывание stale report не заменяют
  окончательный writer/publication gate.
- У selector `SwitchDefaultContext` исходный Manager всё ещё делает mutation
  до candidate probe; всю цепочку admission/terminal/legacy publication надо
  продолжить отдельно. W4c и общая forwarding acceptance не закрыты.
- Скорость восстановления, CPU/RSS и превосходство над другими VPN не измерялись.

Evidence: `.artifacts/memory/probe-intent-source-evidence.json`, exact originals
`probe-intent-before.json`, scoped identities `probe-intent-final-identities.json`.
Только source/gofmt/manual diff/byte-SHA и собственный checkpoint; без запуска
продукта, SDK, helper, тестов, сборок, native API или сети.

<a id="pc-v224-lifecycle-source"></a>

## V2-224 — разделение изменения и очистки

2026-11-05, PC-01/03, **source/unverified**.

**Исправление вывода V223.** Абзацы выше о независимости cleanup были неполны:
`backend/core/cleanup.go` фактически вызывал
`prepareEngineChange(context.Background(), probe)`. Положительный guard V223
не принимал такой ctx и мешал бы restriction TUN перед Stop. Полная цепочка
этого caller была пропущена. Прежнее утверждение не является доказательством
работающей очистки; V224 исправляет исходники, runtime-подтверждения по-прежнему нет.

### Три разные границы

| Граница | Проверка/эффект | Где выполняется |
|---|---|---|
| `ValidateLifecycle` | Исходный App/intent/path, тот же active Manager/engine; не создаёт permit | До положительных действий, вне Manager/Engine locks |
| `GuardLifecycleCompletion` | Только downstream App/epoch value lease; Manager уже подтверждает original active operation/engine | Под Manager.mu; без runtime/Manager reentry, native/IPC/events/cancel |
| `BeforeEngineStop` | Сброс preparation, restriction сохранённого boundary; никакого forwarding или Release | Независимый cleanup ctx, без positive App/intent/current-manager gate |

`LifecycleProbe` сохранён. Generic non-platform probes имеют прежний context
контракт и legacy cleanup fallback. Source-aware probe без отдельного cleanup
или completion API получает явный отказ; positive guard не подставляется
вместо negative cleanup, отсутствие terminal lease не превращается в успех.
Nil source и panic не дают положительный результат; payload panic не публикуется.

### Изменённая цепочка

1. Manager проверяет platform source перед proxy Snapshot, перед Start после
   boundary, перед SetProxy, перед первым selector mutation, перед rollback
   selector/Reload после boundary и перед candidate Reload. Эти проверки не
   выполняют boundary restriction: retained/rejected операция не теряет permits
   только из-за чтения source. Полная TUN selector/WFP admission всё ещё pending.
2. Обычный и native forwarding commit повторяют source до/после управляющего
   RPC. Trust lifetime проверки сохранены. Отозванный subject возвращает ошибку
   в существующий путь actual cleanup, а не новый report/health/permit.
3. Финализация читает trust/source вне Manager lock. Затем original Manager.mu
   подтверждает active operation, Stop request и тот же engine. Root completion
   guard проверяет explicit bound Manager и original ctx intent/path, берёт
   только `App.mu → epoch.mu`, без обратного захода в runtime/Manager.
4. Running фиксируется при удержанном source lease. Release предшествует
   lifetime-watch arming, unregister active owner, `done` и operation cancel.
   Scope acquisition освобождает partial locks даже при source-check panic.
   Если release вернул panic, промежуточное state не наблюдаемо через Manager.mu;
   прежнее transitional state возвращается, active/done сохраняются до actual
   cleanup. Release fault typed EngineCleanupFailed; stale/cleanup flags остаются.
5. `Manager.cleanup` вызывает отдельный negative API для каждого retained probe,
   затем пытается actual Engine.Stop и proxy restoration даже после restriction
   failure/panic. Любой его ненулевой результат содержит EngineCleanupFailed:
   это локальная неопределённость очистки, не диагноз удалённого канала.

`LifetimeRemaining` и retirement по истечению доверия остаются независимыми от
завершённого App operation. Не добавлены goroutine/timer/controller, worker wire,
SDK/API поля UI или новые permits. Legacy direct helper signatures не менялись.

### Пределы source-среза

- Terminal lease связывает App intent и наблюдаемый epoch с решением Manager.
  Проверки перед/после I/O **не делают системные записи атомарными** с новым
  intent. Native writer/private worker и legacy App publication требуют
  дальнейшего разбора; общий lifecycle handoff не объявлен завершённым.
- Native power callback и closing используют независимые atomics, не App/epoch
  locks. Синхронная kernel/WFP revocation, callback atomicity и I/O preemption
  этим lease не доказываются. Power snapshot masking и actual cleanup нужны
  отдельно. Не заявлены hard deadline/CPU/RSS или измеренный выигрыш скорости.
- Не выполнялись compile/tests/fixtures/API/race/queue/Stop/Close/profile/
  rejected retention/rollback/partial acquisition-release/native/OS/resource
  проверки. До них status только source/unverified. Generic fixture behavior,
  обязательные paired interfaces и release-error ordering остаются open.
- Native acceptance=false, private IPC9, engine Job8, TUN unavailable и schema2
  installation unsupported сохранены. Полный PC-00…09/old9 и все прежние gates
  остаются открытыми.

Evidence: `.artifacts/memory/operation-writer-source-evidence.json`; exact
originals `operation-writer-before.json`, scoped identities
`operation-writer-final-identities.json`. Только source/gofmt/manual caller/
lock/diff/byte-SHA и собственный checkpoint, без продуктового исполнения.

<a id="pc-v225-operation-sources"></a>

## V2-225 — отзыв операции перед отложенной записью

2026-11-05, PC-01/03/05, **source/unverified**.

V224 проверяет App intent вне Manager/Engine locks. Между этим guard и actual
command остаются ожидание Engine.mu и планирование существующей pipe-I/O
goroutine. `worker_process.exchange` раньше без повторной проверки вызывал
`writeWorkerFrame`; новая регистрация intent могла ещё не доставить ctx cancel.
Это найденный путь в исходниках, не воспроизведённая runtime-гонка.

### Источники отзыва

- Новый core `OperationSource` разрешает только `Revoke`: после отзыва тот же
  объект не становится current снова. Он не доказывает health/trust/custody и
  не выдаёт forwarding authority.
  На source приходится один passive done channel; atomic Swap закрывает его
  не более одного раза. Source не копируется, передаётся только pointer.
- `OperationSources` в private context key содержит original intent, optional
  operation, original path atomic и closing atomic. Проверка не берёт App/
  Manager locks, не вызывает их callbacks и не запускает IO. Generic unbound
  contexts сохраняют прежний контракт. Nil/malformed bound sources дают отказ.
- Повторное связывание сохраняет original operation source. Чужой intent/path/
  closing/operation создаёт только запрещённый context; нельзя заменить старый
  отозванный source новым. `contextError` также сверяет elapsed deadline, не
  полагаясь только на доставку timer cancellation.
- Все четыре фактические App регистрации — explicit Start, automatic retry,
  profile cutover и Stop/restart/quit — используют `advanceOperationLocked`.
  Старый intent source отзывается под App.mu **до** ID change. Zero ID не
  разрешает beginOperation. Новый source не переиспользует прежний объект.
- `beginOperation` захватывает exact intent source и отдельный operation source.
  Его cancel сперва отзывает operation, затем вызывает старый ctx/epoch cancel.
  Нормальное завершение Start не отзывает applied intent. Новый intent отзывает
  все его оставшиеся positive contexts независимо от задержки стандартной отмены.

### Фоновые проверки и отчёт

В `ProbeReport` добавлен только private unexported intent-source pointer. Root
Run связывает owned report с исходным source; попытка изменить существующую
привязку делает report stale. Clone намеренно делит отзыв того же объекта;
mutable result slices по-прежнему копируются. Source не несёт старый operation
ctx и не продлевает его жизнь. JSON/worker/UI поля не добавлялись, протокол9 прежний.
Это source-contract; сериализация и bindings фактически не запускались.

Periodic constructor берёт private original source из same-session historical
report, затем фактические Manager/generation/path по правилам V223. Текущий App
source не подставляется вместо отсутствующего или отозванного. Свежий periodic
Run после успешного Start получает живой applied intent и заново захваченный
path, без завершённого operation source. Manual/lifecycle ctx сохраняет свой
operation source; чужая повторная привязка закрывается. Root subject/terminal
guard сравнивают exact source pointer с App, помимо scalar ID и Manager scope.

### Запись и завершение

1. Уже существующие `contextError` после Engine.mu теперь видят negative sources,
   включая ожидание selector/admission lock, без обратного вызова root code.
2. `request` повторяет source/deadline до sequence reservation. При отказе ID
   не расходуется. Если отзыв произошёл уже после reservation, `exchange`
   сохраняет cancellation cause и помечает оригинальный process unusable.
3. Existing I/O goroutine проверяет ctx sources непосредственно перед frame
   write/read. После I/O проверка повторяется до принятия reply. Cancelled
   sequence не используется повторно; existing caller/original Job owner
   выполняет actual retirement, uncertainty не заменяется новым владельцем.
   Existing exchange select также ждёт original intent/operation revoke channels
   до reply и до ioDone: новый Start/retry может прервать ожидание без задержки
   доставки ctx cancellation. Ни один новый watcher/goroutine/timer не создан.
   Source.Done означает только revoke notification; p.done/ioDone/Job.Empty
   по-прежнему подтверждаются отдельно при actual retirement.
4. Source не сериализуется в worker command и не принимается как receipt.
   Успешный старый ACK не меняет App intent/source. Native acceptance=false,
   IPC9/Job8/TUN unavailable/schema2 unsupported сохранены.

Вручную прослежены `Manager.cleanup → prepareEngineStop(Background)`, actual
worker Stop, original native capture revoke и process reap с собственными
Background budgets. Новые positive sources им не подставляются. Lifetime expiry
проверяет доверие независимо от завершившегося operation source.

### Открыто

Гонка между последней atomic-проверкой и уже начавшейся записью/системным вызовом
остаётся: флаг не прерывает in-flight I/O и не отзывает worker/kernel допуск
синхронно. Actual cancellation/Job retirement и closed native gate необходимы.
Service-DNS observer handoff и legacy synthetic App reports имеют свои scopes;
они не объявлены полностью переведёнными на этот контракт. Следующий source
срез — legacy publication producers и оставшиеся private-worker/native writes.

Не выполнялись tests/fixtures/build/typecheck/parser/SDK/helper/native/network
или сериализация продукта. Core API/JSON/private Clone/zero-ID/old fixtures/
foreign rebind/queue/deadline/sequence/I-O/Stop/Profile/epoch-power/lifetime/
runtime/race/resources/OS приёмка открыта. Новых goroutine/timer/map/controller/
dependencies нет; hard deadline/CPU/RSS/скорость/превосходство не измерены.
Полные PC-00…09/old9 и все прежние gates сохранены.

Evidence: `.artifacts/memory/worker-intent-source-evidence.json`, exact originals
`worker-intent-before.json`, scoped identities `worker-intent-final-identities.json`.
Только source/gofmt/manual caller-lock-diff/byte-SHA и собственный checkpoint.

<a id="pc-v226-report-publication"></a>

## V2-226 — публикация пути и остановка исходного владельца

2026-11-05, PC-01/03/05, **source/unverified**.

Аудит actual root callers: live `ProbeReport` создаётся через protected Run;
`trustFailureReport` вне этого пути — локальная UI-проекция, не live authority.
Общий publisher до V226 сравнивал scalar ID/epoch, не original path pointer.
Одновременно blanket stale check отбрасывал и actual resource/protection faults
при потере сети. Policy Observe мог потерять independent error из-за последней
проверки freshness. Это исходники, а не воспроизведённые runtime-сбои.

### Два разных решения

| Решение | Обязательная область | Что разрешено |
|---|---|---|
| Health/Running/retained/current measurement/selection | Exact App intent source, config/session, Manager generation и current original path/epoch, trust/clock/observer gates | Только исходный свежий результат; старый путь не получает новую эпоху |
| Restrictive Stop | Private actual-work marker + unsafe fact, тот же current intent source/config/session/Manager/engine/owner/generation | Остановить только этого inactive владельца после path/time loss; без health/ProbeLastAt refresh |

`ProbeReport` получил только private path pointer/restriction marker. Owned
Clone сохраняет эти значения и общий отзыв original source. Публичные JSON/UI/
worker поля прежние, serialize/generation не выполнялись. Повторная привязка
к иному path делает stale; private marker не даёт положительного допуска.

Protected Run ставит marker только после перехода к actual probe/boundary work
при наличии unsafe факта. Early unavailable из-за original source gap не
означает ошибку защиты и не резервирует Stop. Отсутствующий binding/public
ProtectionUnavailable без private marker не принимаются как safety scope.

### Публикация и actor guard

- `windows/report_scope.go` разделяет intent/path проверки и typed protection
  faults. Общие predicates выполняют только value work под известными locks.
- Ordinary `currentProbeReport` исключает unsafe/stale/retirement scope: им
  нельзя обосновать выбор канала или положительный переход. Отдельный
  `currentProbeSubject` допускает actual safety scope для retirement callers.
- `GuardRetirementOwner` пинит exact inactive Manager/engine/owner/generation
  в Running/Error. Failed original process не скрывает cleanup uncertainty.
  Он не заменяет GuardRunningOwner и не разрешает forwarding. Новый active
  lifecycle, другой config/actor/source или ручной Disconnect не принимаются.
- `publishObservation` требует private intent source/path. Retained rejection
  может иметь старое report generation по прежним правилам, но не получает
  новый path, время или health. Положительная публикация блокируется также
  trust/restart/image transition. Restrictive reports принимаются только для
  Stopping/SafetyStopping; failed clock/network не создают channel judgment.
- Stop reservation сразу сбрасывает Ready/ProbeRunning/native cache. После
  cleanup старый path отображается historical даже при том же scalar epoch.

Watchdog/fast/net-change обрабатывают local faults до обычного freshness filter.
Manual probe использует уже удержанный operationMu, без recursive Stop lock.
Observed CleanupUnconfirmed имеет приоритет перед leak/protection presentation
и остаётся typed EngineCleanupFailed даже после later Stop nil; он не становится
автоматическим поводом перебирать каналы. Report-free Stop по actual Manager
error дополнительно сверяет original intent source, не переиспользуя PASS.

### Проверка текущей политики

`verifySessionProtection` сохраняет private signature, но читает boundary под
original report intent/path через existing beginOperation/8s child budget,
без Background revival. Новый path при bind mismatch закрывается до Observe.
Root/native work выполняется вне App/Manager/epoch publication locks.

Actual Observe error возвращает owned typed fault report с original bindings,
AllPassed=false и ProtectionUnavailable, сохраняя independent custody flags.
Потеря path не стирает этот факт; actor guard повторяется перед Stop. Pure
cancellation/source supersession без local custody fault не считается ошибкой
политики. Panic после начала наблюдения сохраняет только тип и custody
uncertainty, budget/operation освобождаются до выхода из operationMu. Все четыре
actual policy callers используют typed fault, не приписывают старому PASS
ошибку/текущий actor по полям UI.

### Открытая приёмка

Не выполнялись compile/tests/fixtures/API/JSON/private Clone/marker/old-source/
failed-engine/partial Stop/queue/clock/network-power/late callback/typed error/
context-cancel/panic/runtime/race/resources/OS проверки. Source predicates не
доказывают syscall preemption, kernel/WFP atomicity, forwarding safety, hard
deadline или производительность. Source-aware fixtures с голыми public reports
могут потребовать корректного original scope; они не изменялись.

Service-DNS handoff, active/retired asynchronous actors и остальные worker/native
writes требуют своих original-owner scopes; этот срез не объявляет их закрытыми.
Independent cleanup по V224, lifetime expiry, native=false/IPC9/Job8/TUN
unavailable/schema2 unsupported и полный PC-00…09/old9 сохраняются.

Evidence: `.artifacts/memory/report-source-source-evidence.json`; exact originals
`report-source-before.json`, scoped identities `report-source-final-identities.json`.
Только source/gofmt/manual caller-lock-diff/byte-SHA и собственный checkpoint.

<a id="pc-v233-late-probe"></a>

## V2-233 — finite original-worker faults через отмену probe

2026-11-05, PC-01/03 source/unverified. Весь PC00…09/old9 сохраняется.

### Потеря причины в прежних исходниках

После фактического `settings.Run` ошибки ctx/actor/epoch/native preparation
обнуляли captured epoch или сам report. Child при ошибке RPC очищал report,
даже если тот содержал независимую cleanup/trust restriction. Родительский
`workerProbeFailure` восстанавливал только custody flags, теряя typed trust.
Прежняя проверка subject/tag/report shape выполнялась после ожидания caller,
поэтому cancellation обходила и этот разбор. V232 сохранил original completion
header; сам смысл доставленного late probe reply тогда оставался открытым.

### Source implementation

- После уже выполненной local probe возвращаются report и original epochBefore
  при последующей ошибке. Это private negative provenance, не свежий epoch или
  выдача permission. До начала работы actor/context rejection остаётся без epoch.
- Child в существующем `probe_failed` переносит только bounded
  `workerProbeFailure(err, reportLifecycleError(report))` при nonzero original
  epoch и finite local restriction. Subject/tag — исходного validated request;
  ordinary/native tokens очищаются. AllPassed/admitted/health/session metadata
  не выдаются. Такой отрицательный reply не проходит последующий current-state
  query, который мог стереть факт или приписать ему новую актуальность.
- Parent в original I/O goroutine после wire version/ID/native-token checks
  вызывает pure `workerProbeReplyFailure` до done. Shared report shape проверяет
  исходный probe request/subject/tag, nonzero epoch, 1…16 results, пустой scope.
  Typed cleanup/trust извлекается только из этой correlated finite формы;
  AllPassed/token и admitted в failure-envelope противоречия отвергаются.
  Ordinary empty probe_failed — worker unavailable, без invented local flags.
- V232 original-process wrapper после bounded reaper сохраняет completed typed
  cause. `workerProbeFailure` восстанавливает cleanup и TrustFaultError, используя
  общий прежний finite trust reason allowlist; unknown → trust_unavailable.
  В отрицательном envelope нет исходных error/path/result/private bytes.
- Parent current ctx/process/instance gates, native permit/probePassed/final
  Manager gates и App original work/source/path marker остаются отдельными.
  Публичный Report не становится actor/health/admission/retirement authority.
  Raw Stop и exact TUN scope cleanup receipts не менялись.

Использованы существующие fields/code IPC9, никаких новых wire полей или версии;
обычный failure по-прежнему fail-closed. Совместимость требует отдельной приёмки.
Новых goroutines/watchers/timers/budgets нет. Уже entered pipe/native syscall не
прерывается source predicate; forced original retirement может прекратить child
до доставки отчёта. Недоставленный/unfinished факт остаётся unknown, не synthetic
cleanup/trust/success. Не обещается гарантированная доставка late diagnostics.

### Evidence / OPEN

Source5Go/4docs, exact-file gofmt5 exit0, manual caller/import/type/Engine-lock/
completion/error/diff review и named byte/SHA identities. Exact originals9,
prior task71 rev464 и цепочка прежних обязательств — `.artifacts/memory/worker-probe-*`;
source evidence — `worker-probe-source-evidence.json`. PLAN и этот task дописаны
с сохранением прежнего byte prefix; goal/constraints/tasks0…70/acceptance/
decisions/старые checks не сокращены.

Tests/fixtures/build/vet/typecheck/product parser/check/SDK/helper/native/browser/
admin/live/network/other-role/chat/subagent execution не выполнялись. Compile,
existing fixtures, IPC9 success/ordinary/typed/foreign/contradictory report shapes,
unknown trust reason, combined custody+trust, deadline/source/epoch/actor loss,
partial/late frame, forced reap, race/Job/handle/resources/OS остаются OPEN.
Other command late semantics, native Release/launch atomicity, scalar async
provenance, полный PC00…09/old9/SDK/W3b/interprocess/durability/UX/release/comparative
также OPEN; native=false/IPC9/Job8/TUN unavailable/schema2 unsupported сохранены.
