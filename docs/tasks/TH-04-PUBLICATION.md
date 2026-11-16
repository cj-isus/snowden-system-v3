# TH-01/04 — публикация наблюдений и решений

2026-10-21, V2-115: implementation, **unverified**. Полная цель TH-00…08
сохраняется; tests/build/vet/harness/native/live по указанию владельца отложены.
Новые tests не добавлялись/менялись, worker и системная сеть не запускались.

## Проблема и граница решения

Проверка current report перед отдельными записями App/SM/failover оставляла
окно для нового Disconnect/сессии/эпохи. Генерация Manager не заменяет намерение
пользователя, а поздний успешный probe не обязательно новее уже опубликованного.
V2-115 добавляет общий короткий commit наблюдения; Manager остаётся единственным
путём применения engine action. Runtime/concurrency корректность не проверена.

| Идентичность | Источник | Для чего нужна |
|---|---|---|
| config SessionID + pointer | owned applied cc | результат относится к этому конфигу |
| Manager ID + generation | lifecycle owner | результат не относится к прошлому запуску/переключению |
| IntentGeneration | App.operationID до Run | Stop/новый запрос отзывают старую работу |
| NetworkEpoch | наблюдатель физической сети | факт не переносится на другой маршрут |
| ObservationID | process-wide monotonic sequence до Run | старый поздний результат не перезаписывает более новый |
| attempt generation | failover reservation | старый completion не завершает следующую попытку |

## Исходники и правила

- `appProtectedProbe.Run` присваивает intent/observation IDs, проверяет intent
  после работы наряду с owner/epoch/clock. Core report хранит только scalar
  новые поля; Clone переносит их как значения. Generated Wails bindings не менялись.
- `observer_publication.go`: operationMu → runtime → App → epoch → health/SM/
  failover. Повторная проверка всех identities и in-memory commit происходят
  вместе. EventsEmit — после освобождения runtime/epoch, под App.mu для порядка
  событий. Probe/Manager mutation/WFP/network/log persistence внутри commit нет.
- Start, applied switch/verified rollback, manual probe, watchdog и net-change
  используют этот вход. Healthy/recovered/degraded состояния, ProbeLastAt,
  strong health и failover reset больше не записываются раздельными observers.
  ProbeLastAt берётся из completed observation, не из времени чтения UI.
- Process-wide watermark отвергает старые ordinary observations. Повтор одного
  report в нескольких фазах не добавляет health counters повторно. Epoch0
  допускает session-local состояние, но не создаёт strong physical health.
  ISP для новых strong events остаётся unknown: неподписанный cache не является
  ISP provenance конкретного probe. Старые прямые background helpers сохранены.
- Положительная unsafe evidence не поглощается более новым ordinary probe,
  но обязана принадлежать тем же intent/session/epoch. Отдельная safety-stop
  фаза разрешает остановку при невозможности подтвердить WFP; это не утверждение
  о доказанной утечке и не synthetic channel FAIL. Superseded verification
  отдельно от actual policy error: его нельзя превращать в safety failure.
- `netEpoch.bindOperation`: одна serialized lifecycle operation, без goroutine/
  subscription map. Up/change отзывает старый context после освобождения epoch
  mutex; release старого binding не снимает новый. Engine lifetime отделён от
  operation context. Новое engine действие повторно проверяет исходный report.
- Отмена не всегда означает «Stop уже идёт». Start/Reload cancellation finalizer
  при текущем intent очищает partially applied работу и планирует fail-closed
  recovery; отмена до первого Arm не снимает запрос на protection. Завершённая
  published сессия отделена от этого context. Новое намерение владеет своим UI.
- `observer_failover.go`: reserve под тем же commit, отдельный attempt token,
  повторный report gate после получения operationMu. Success/rollback используют
  новый applied report; late generic failure не сбивает новый healthy verdict.
  Retained live path не объявляется BLOCKED по ошибке кандидата. Исчерпанная
  попытка сохраняет cooldown; generic error остаётся work hint, не health FAIL.
- Preference берёт applied descriptors и передаёт исходный report к action gate.
  Unscoped private failover helpers оставлены для existing local fixtures;
  production watchdog/network callers используют observed path.
- Автоматический Stop сначала резервирует решение в epoch/intent commit, затем
  выполняет внешнюю очистку. Изменение эпохи после принятого Stop помечает его
  report как historical/stale. Report-free worker exit проверяет actual failed
  owner и intent применённого gate; last report используется только как identity.
  Stop/failCore final publication не перезаписывает последующее намерение.
- Отказ до замены ядра не освежает старое probe evidence. UI steps больше не
  пересобираются в выдуманный текущий causal report при failCore.

## Открытые gates и следующий срез

Compilation, source compatibility существующих fixtures, runtime lock order,
race/concurrency/late completion/cancellation и реальные Windows flows **не
проверены**. Только чтение исходников/gofmt; никаких новых PASS. Это не вся
ProtectionController/service архитектура и не доказательство OS atomicity.

P0 далее: source-аудит WFP permit/external TUN gate после полного Reload/rollback
и совпадения фактического интерфейса/worker с applied state; существующий Start
имеет этот gate, switch lifecycle требует отдельного среза. Затем clock/expiry/
trust recovery и bounded action/quorum/backoff policy. Старт из epoch0 может быть
отменён первым наблюдением сети; восстановление и задержки нужно разбирать без
обещаний скорости. Replay/shadow/canary/OS/native/release/comparative gates открыты.
