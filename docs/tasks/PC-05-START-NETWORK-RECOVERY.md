# PC-05 — отмена Start при смене сети и ограниченное восстановление

2026-11-05, V2-218, Windows **source/unverified**. Продолжение
[route facts](PC-05-SOCKETLESS-NETWORK-FACTS.md); полный
[PC00…09](../planning/knowledge-application/PC-PLAN.md) активен.

## Дефект и компоненты

Первое доставленное событие poller отзывает original path и отменяет Start.
Передача только ctx.Err могла скрыть исходную ошибку подготовки/очистки;
backoff не учитывал уже появившийся новый path. Callback с повторно используемым
attempt не отличал старую регистрацию после ручного действия. Trust-ветке также
нужны original intent и запрет retry при неподтверждённой остановке.

Расширены существующие App lifecycle/retry API в пяти Go-файлах. Новый
controller/producer/timer owner/SDK API/worker IPC/public report/forwarding grant
не вводятся. Тестовые файлы не менялись.

## Отмена Start

1. `corebridge.go` сохраняет фактическую render/start/candidate ошибку в local
   cancellationCause, проверяет ctx перед обработкой candidate failure и
   очищает cause после успешного кандидата. Все Start failure/trust paths
   передают зарегистрированный id, не новый пользовательский intent.
2. `protection.go` объединяет ctx.Err и cause через errors.Join. Finalizer
   действует только для current desired starting/reloading intent, без profile
   transition/closing. Уже опубликованная running session не отменяется из-за
   штатного завершения operation context.
3. `failCoreForIntent` повторяет exact-intent fence. При cancellation/deadline
   actual stopCoreGenerationLocked вызывается даже без Manager: проверяются
   прежний engine owner и capture helpers. Ошибка Stop получает typed
   ErrEngineCleanupFailed, отменяет retry и даёт cleanup_unconfirmed.
4. Только nil из actual Stop даёт private lifecycleRetirementReceipt этого
   intent. Это source control-flow result, не native/WFP acceptance token,
   inference из пустого runtime, таймер или положительный probe.
5. `start_network_recovery.go` разрешает ускорение лишь после этого receipt для
   context.Canceled, отозванного original path, нового current path и того же
   live App/intent. Fail-closed и existing retry owner должны быть действующими.
   Новый запуск целиком проходит render/trust/Manager/probe/publication;
   старые ctx/report/config не переносятся.

Deadline, ручной Stop/Close, profile/image transition, dead App lifetime,
trust/cleanup uncertainty не дают этого ускорения. Generic Reload/Select
finalizer сохранён; полная cause/intent миграция вне Start ещё не заявляется.

## Timer и бюджет

- Регистрация timer имеет monotonic serial и exact operation intent. Stop
  отзывает оба; serial никогда не сбрасывается, насыщение не допускает reuse.
  Callback проверяет attempt/serial/intent до мутации расписания. Timer.Stop
  **не** является join callback; actual asynchronous AfterFunc захватывает
  original registration. Прежний private fireFailClosedRetry(attempt) сохранён
  для direct callers, читает только current schedule; timer его не вызывает.
- Finalizer и обычный network wake используют общий лимит: **два выданных
  внеочередных запроса за fail-closed эпизод**, не больше одного на original
  current path source. Это запросы, не успешные подключения и не глобальная
  граница goroutines. Source повторно не активируется; бюджет переносится через
  automatic operation IDs, обычный backoff его не пополняет.
- После лимита остаётся прежний backoff 30с…10м. Отозванный до callback captured
  fast path возвращается к ordinary delay без запуска и сброса бюджета.
  Wake/timer сериализованы existing done guard и registration invalidation.
- Existing full cancellation сбрасывает episode/budget: ручные Start/Stop/Close,
  успешная Started publication и blocking profile/trust/image/local-custody
  transitions. Dead App.ctx не допускает fail-closed retry.

## Trust и очистка

`trust_lifetime.go` проверяет original intent до cancel/actual Stop и до
публикации. При cleanup error BlockedReason=cleanup_unconfirmed, trustBlocked=true:
даже certificate_expired не retry поверх неубранного владельца. При successful
cleanup прежние signed pin-wake/certificate-retry rules сохранены. Изменён
Windows adapter, не shared SDK. Generic failCore/failTrustCoreLocked сохраняют
API и фиксируют current intent на входе; provenance остальных asynchronous
callers требует отдельного разбора.

## Evidence и открытая приёмка

Source review, gofmt5 exit0 receipt40a13e, manual diff и byte/SHA inventory:
`.artifacts/memory/network-recovery-source-evidence.json`. Локально прочитан
Go src/time/sleep.go: Timer.Stop не ждёт уже запущенный AfterFunc callback.
Таймеры и продукт не исполнялись.

Tests/fixtures не добавлялись/не менялись/не запускались. Compile/build/vet/
typecheck/parser/check/doctor/install/generation/SDK/helper/product/crypto/native/
browser/admin/live/DNS/server execution отсутствует, OS settings не менялись.
Форматирование/чтение исходников не compile/runtime/safety/speed PASS.

First-event/Start/down→up/change, timer ABA/wake/cancel/Stop/Close, cleanup до
Manager/в trust/candidate, exact-intent publication, actual retirement/probe,
API/fixture compatibility и resource/OS acceptance OPEN. Нет hard shutdown/
callback join/CPU/RSS/latency bound. Далее source — cause/intent вне Start и
desktop suspend/resume. IPC9/Job8 прежние, native acceptance const=false,
TUN unavailable/schema2 unsupported. Полный PC00…09, FR/TH/OFF/COM/RQ/TR,
W3b/SDK/interprocess/durability/UX/release/comparative критерии OPEN.

## V2-219 — переключение, Reload и остановка по наблюдению

2026-11-05, **source/unverified**; расширены существующие `failover.go`,
`protection.go`, `observer_failover.go`, без нового lifecycle owner.

- SelectChannel wrapper фиксирует App intent **до** ожидания operationMu.
  Observed auto-switch передаёт intent принятой исходной попытки. Внутренний
  switch entry проверяет этот id через existing beginOperation, не принимает
  новый Stop/Start, зарегистрированный во время ожидания. Public/Wails API и
  прежние private direct-call signatures сохранены.
- Named return передаёт фактическую составную ошибку selector/Reload/rollback
  в finishCanceledLifecycleCause. Реальный ctx.Err сохраняется, deadline не
  подменяется cancel-only. Finalizer действует для current desired reloading.
  Root render/clock prepare до этого состояния сохраняет старого владельца;
  отмена уже внутри Manager prepare может потребовать Stop даже при rejection
  до mutation. Уже опубликованный running outcome finalizer не останавливает.
- Все switch failure/publication paths используют failCoreForIntent(id).
  Independent local custody failure отзывает joined positive rollback receipt
  через existing InvalidateLifecycleOutcome, даже при ErrReloadRejected или
  Manager running. Trust/custody причины сохраняются; старый отчёт не получает
  права на новый путь. Fast recovery V2-218 остаётся только Start-finalizer.
- Automatic expected-session apply/verified rollback публикует Applied, явный
  UI selection — Started. Automatic outcome больше не hardReset своей ещё
  незавершённой policy attempt как ручное действие; scoped completion владеет
  её завершением. Для network preference это тоже automatic Applied, не новый
  ручной incident reset. Actual fresh probe/publication guards обязательны.
- Policy регистрирует и возвращает original attempt generation одним lock.
  Generic auto caller несёт её до completion, вместе с original path source и
  epoch. Stale success и failure отбрасываются, в том числе epoch0→down.
  End/reset завершает только исходную попытку; old direct wrapper получает
  текущее значение лишь для синхронных callers, production work его не вызывает.
- При том же intent/epoch local cleanup/trust/image error идёт раньше
  cancel-only discard: deferScopedAttempt снимает только own attempting flag,
  оставляя switches/lastError/lastAttempt и cooldown. Нет расхода или обнуления
  бюджета, candidate hint, HEALTHY/DEGRADED публикации или нового rollback proof.
  Устаревший токен не снимает successor. Фактический новый positive observation
  по-прежнему может штатно завершить incident. Exhaustion сам по себе не
  останавливает retained running owner; этот negative fence не новый proof.
- stopObservedSession передаёт intent принятого report и в trust adapter, и в
  actual Stop. Любая ненулевая cleanup error получает ErrEngineCleanupFailed,
  cleanup_unconfirmed и cancel retry до markBlocked. После смены user intent
  старый handler не переписывает состояние. Stop failure не диагноз канала.

Source3 Go/4 existing docs, gofmt3 exit0 receipt09b320, manual caller/diff/byte-SHA
inventory: `.artifacts/memory/lifecycle-cause-source-evidence.json`. Tests/fixtures
не добавлялись/не менялись/не запускались; compile/build/vet/typecheck/parser/
check/doctor/install/gen/SDK/helper/product/crypto/native/browser/admin/live/
DNS/server/OS-setting execution отсутствует. API/fixture compatibility не
скомпилирована; source/gofmt не runtime/native/safety/performance PASS.

Открыты queue/Stop/Start/prepare/mutation/cancel/late publication/retained owner,
joined-error precedence, both completion paths/reset/token exhaustion, actual
cleanup/probe/resources/OS и API приёмка. Полная atomicity всех legacy
публикаций, decision-to-mutation path binding, hard cancellation/latency/RSS
этим не доказаны. Далее source — оставшиеся async negative/trust callbacks и
desktop suspend/resume. Native gate=false/IPC9/Job8/TUN unavailable/schema2
unsupported и весь PC00…09/old9/SDK/W3b/UX/release/comparative OPEN сохранены.

## V2-220 — исходный scope фоновых ошибок

2026-11-05, **source/unverified**, четыре Go-файла, прежние lifecycle owners.

- Первый negative fence failed DNS task сохраняет private immutable tuple:
  current App intent, actual Manager/Engine/config pointers. Это регистрация
  локального отказа на фактически привязанном runtime, не исходный Start intent,
  не probe/native receipt и не новый forwarding grant. Один tuple на original
  task; повторные poller/DNS finalizers его не заменяют новым пользователем или
  конфигурацией. Scope хранит ссылки, значения конфигурации не публикуются.
- Lock order `runtime → App → DNS owner`; poller/run отпускают свои locks перед
  entry. До value-only Ready/native-cache/retry fence повторяются tuple/id;
  captured cancellation вызывается после всех Unlock. Manager/engine stop,
  ожидание operationMu и I/O под этими locks не выполняются.
- После ожидания operationMu retirement повторяет exact DNS/runtime tuple и
  App intent, вызывает failCoreForIntent. Смена intent/runtime не разрешает
  ни повторный negative state write, ни остановку нового actor. Без actual
  Manager receipt отсутствует: неизвестный actor не получает cleanup authority;
  прежний Start/Stop reconciliation и orphan gates сохраняются. Original done
  по-прежнему означает task completion, не доказательство остановки engine.
- DNS control-error defer передаёт сохранённый intent в failCoreForIntent.
  Lifetime callback передаёт IntentGeneration своего fault.Report, сохраняя
  прежние session/Manager owner/retired-generation проверки. Report даёт только
  identity, не новое доверие, здоровье, epoch или измерение пути.
- Watchdog clock checkpoint фиксирует id до operationMu; сверяет actual applied
  config/Manager running/owner generation и report session/intent перед I/O.
  Applied cc клонируется. I/O идёт без runtime/Manager/App/DNS locks; поздняя
  storage/trust error маршрутизируется старым intent, успешный возврат повторяет
  current intent/session. Hard cancellation/durability/latency I/O не заявлены.
- Ручной probe фиксирует id до operationMu, все unsafe outcomes вызывают
  scoped failure. CleanupUnconfirmed дополнительно сохраняется как typed
  ErrEngineCleanupFailed, поэтому не превращается в ordinary retry даже если
  следующий Stop вернул nil. Остальные measurement/publication guards прежние.
- В текущих non-test callers не осталось вызовов generic failCore и
  failTrustCoreLocked; private direct entry signatures сохранены. Новые задачи,
  controller/SDK/wire/generated API не добавлены. Scope bounded1 на original
  DNS task, predecessors прежние; это не hard RSS/whole-process task bound.

Source4 Go/4 existing docs, gofmt4 exit0 receiptd3b5d2/manual caller/lock/diff/
byte-SHA: `.artifacts/memory/async-fault-source-evidence.json`. Tests/fixtures
не добавлялись/не менялись/не запускались. Compile/build/vet/typecheck/parser/
check/doctor/install/gen/SDK/helper/product/crypto/native/browser/admin/live/
DNS/server/OS-setting execution отсутствует. Нет API/runtime/native/safety/
speed PASS. Original/repeated fault/queued Start/Stop/Profile/Reload/missing
scope/partial cleanup/panic/late I/O/expiry/probe/reset/lock/race/resources/OS
приёмка OPEN. Далее source — desktop suspend/resume и generic decision-to-
mutation/legacy publication atomicity. Полный PC00…09/old9/SDK/W3b/interprocess/
durability/UX/release/comparative OPEN, native gate=false/IPC9/Job8/TUN unavailable/
schema2 unsupported сохранены.
