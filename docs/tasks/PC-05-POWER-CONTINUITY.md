# PC-05 — сон Windows и непрерывность сессии

2026-11-05, V2-221, **source/unverified**, только ПК. Полный
[PC00…09](../planning/knowledge-application/PC-PLAN.md) и прежний backlog активны.
Продолжение [scoped lifecycle](PC-05-START-NETWORK-RECOVERY.md), без второго monitor.

## Контракт и источник событий

Сон может оставить прежний route fingerprint неизменным, поэтому обычный route
polling не доказывает сохранённую непрерывность. Использована отдельная системная
notification registration, которой владеет **прежний original network poller**:

- [PowerRegisterSuspendResumeNotification](https://learn.microsoft.com/en-us/windows/win32/api/powerbase/nf-powerbase-powerregistersuspendresumenotification)
  вызывается с DEVICE_NOTIFY_CALLBACK=2; actual DWORD result, не LastError.
- [DEVICE_NOTIFY_SUBSCRIBE_PARAMETERS](https://learn.microsoft.com/en-us/windows/win32/api/powrprof/ns-powrprof-device_notify_subscribe_parameters)
  имеет callback/context pointers; context здесь opaque cookie, не Go owner ptr.
- [Callback contract](https://learn.microsoft.com/en-us/windows/win32/api/powrprof/nc-powrprof-device_notify_callback_routine):
  Context/ULONG Type/unused Setting, Windows error return. Setting не читается.
- Обрабатываются PBT_APMSUSPEND=4 и
  [PBT_APMRESUMEAUTOMATIC=18](https://learn.microsoft.com/en-us/windows/win32/power/pbt-apmresumeautomatic).
  RESUMESUSPEND=7 — пользовательское взаимодействие после automatic resume,
  не дополнительное событие потери network continuity.

Новые Go-файлы отделяют platform binding, platform-independent retirement и
non-Windows unavailable stub. Закреплённый x/sys0.44/system-DLL/Go stdcall reuse;
новые зависимости/окно/message pump/producer task не добавлены. OS API доступны
по документации с Windows8; actual deployment/ABI/load acceptance не выполнена.

## Владение native registration

`power_notifications_windows.go` содержит capacity1 registry, включая uncertain
retirement. Cookie никогда не переиспользуется, при saturation новый register
отклоняется. Один process-lifetime NewCallback thunk; отдельные pointer-free
двухполейные params pinned через runtime.Pinner на срок registration. Нативному
коду не передаётся retained pointer на Go App/poller. Callback находит только
свой active cookie и учитывается original entered-counter; вызов на старый cookie
не получает successor. Не создаются callback thunks на каждый wake.

Close сначала закрывает callback admission, отпускает registry/owner locks,
затем вызывает [PowerUnregisterSuspendResumeNotification](https://learn.microsoft.com/en-us/windows/win32/api/powerbase/nf-powerbase-powerunregistersuspendresumenotification).
Только zero DWORD подтверждает отмену registration. Original entered callbacks
дожидаются отдельно, затем params unpin/slot release. Unregister не объявляется
join. При error/panic исходный handle/params/slot сохраняются; unknown success
без handle не превращается в receipt. Reconciliation повторяет retirement
того же original owner перед созданием одной новой пары poller/DNS.

## Callback и восстановление

1. Callback под коротким original poller lease отзывает running/observed и
   atomic local power/path continuity. После callback новый path не открывается
   обычным route bump: атомарный blocked flag и mirrored original path учитывают
   гонку создания нового source. Нет App/epoch/lifecycle lock, cancellation
   callbacks, логов, IO, Stop или новой goroutine в notification body.
2. Bounded wake channel1 будит существующий poller loop. Он отменяет original
   lifecycle context вне epoch lock и завершает свой stack. Попытка native retirement
   предшествует original done. Связанный DNS owner также отзывается/завершается;
   known power cancellation отличается от producer panic/registration/cleanup
   fault, независимые actual ошибки не снимаются с joined cause.
3. Existing exact DNS fault scope сохраняет cause и original actor/intent.
   Actual Stop/helpers остаётся у прежнего App lifecycle; неизвестный actor не
   получает cleanup authority из факта закрытия task. Никакого channel FAIL,
   physical up/down или network epoch из power event не синтезируется.
4. При desired protection и successful retirement остаётся обычный existing
   backoff/retry. Перед привязкой нового Start к отозванному path lifecycle owner
   подтверждает оба original done, **заново actual Stop/helpers**, original native
   unregister и только затем одну новую пару с подтверждённой registration/entry.
   Callback до завершения registration не открывает continuity обратно.
5. Новый original source создаётся вместо revival старого. Это unknown local
   continuity для подготовки, не hardware/Internet/native proof; known-down
   сохраняется, настоящий route event открывает новую network epoch. Далее весь
   обычный render/trust/Manager/protected probe/publication обязателен. Новый
   user Stop/Start не принимается старым id; завершённый setup сам не стартует VPN.

Если protection не запрошена, автоматический engine Start отсутствует. После
retirement отключённый App может ждать явного Start для восстановления producer
pair; always-on observation в disconnected state ещё не принято. Используется
обычный backoff30с…10м, отдельного fast power scheduler нет. Wake latency,
короткий сон/aborted suspend и влияние clocks/timers требуют приёмки; преимущество
в скорости этим source change не доказано.

## Публикация и независимые ошибки

Power-blocked phase запрещает non-safety/retained Running publication и current
channel-health epoch. GetState и последующие state-event snapshots снимают cached
Ready/ProbeRunning и поясняют отсутствие continuity через прежние DTO поля.
Native callback не вызывает UI. Ранее уже отправленные/очередные UI события,
browser/Bindings/UX ordering остаются непроверенными.

Manager/session lifetime routing теперь сохраняет original joined error.
Независимый local cleanup fault блокирует certificate-expired retry и pin-wake,
даже если последующий engine Stop вернул nil: он не подтверждает unregister
отдельной native registration. Shared trust SDK и подписанная policy не менялись.

**Atomic callback fence — process-local.** Это не доказательство немедленного
отзыва kernel/WFP forwarding permit до/во время физического resume. Actual Stop
производится позже; hard callback/Close/syscall/join/IO/latency/RSS guarantees
не заявлены. Native acceptance const=false, TUN unavailable/schema2 unsupported,
IPC9/Job8 неизменны. Native/service/kernel admission, SOCKS scope и W4c сохраняют
прежние открытые требования. Этот этап не включает TUN.

## Evidence и оставшаяся приёмка

Source13 Go/4 docs, gofmt13 exit0 receipt37a123, manual source/caller/lock/diff
и scoped byte-SHA: `.artifacts/memory/power-continuity-source-evidence.json`.
Прочитаны только public primary Microsoft docs и локальные pinned Go/xsys sources.
Tests/fixtures не добавлялись/не менялись/не запускались. Compile/build/vet/
typecheck/product parser/check/doctor/install/gen/SDK/helper/product/crypto/native/
browser/admin/live/DNS/server/owner-values/other-role/chat/subagent execution
не было; OS settings не менялись. Нет compile/runtime/native/speed/safety PASS.

Открыты Win64 ABI/32-bit/callback pinning/registration/partial handle/unregister
error/late callback/slot/cookie exhaustion/suspend/resume/Modern Standby/hibernate/
duplicate interaction/cancel/Start-Stop-Profile/clock expiry/cleanup precedence/
kernel forwarding/queued UI/missing context/API fixtures/resources/OS приёмка.
Полный PC00…09/old9/SDK/W3b/interprocess/durability/UX/release/comparative OPEN.
Далее source — generic decision-to-mutation/legacy publication atomicity и
disconnected producer recovery; безопасный быстрый resume требует отдельного
измеренного gate, не выдуманного seamless claim.

## V2-222 — восстановление наблюдателей при отключённом VPN

2026-11-05, **source/unverified**. Для clean power-retirement устранена source
зависимость V2-221 от ручного Start. Native/cleanup fault не превращается в
разрешение автоматического восстановления; runtime приёмка остаётся OPEN.

- Используется существующий watchdog30с, без нового loop/timer/task. Он стартует
  до раннего return ветки auto-connect и получает исходный live App.ctx вместо
  Background. Canceled ctx/closing прекращают loop; watchOn сохраняет прежний
  singleton guard. Это не новый task-done/join receipt и не hard shutdown bound.
- Пока power continuity отозвана, ordinary health probe пропускается. Тот же
  tick может восстановить только original observer pair, когда protectionDesired
  false, state stopped/error, нет profile/restart/image/closing transition и
  root Manager/Engine/config/metrics/startBusy. Guard повторяет захваченный id.
- Оба original done обязательны. Poller/DNS должны иметь известную power cause,
  без independent local fault. Это negative scope restriction, **не** receipt
  остановки engine из пустого runtime. Helper retirement подтверждается actual
  RetireCaptureHelpers; existing reconciliation подтверждает native unregister
  и создаёт одну новую пару с actual registration/entry. Новое observer setup
  не предоставляет engine admission; полный Start остаётся независимым.
- Existing helper TryLock не отменяет занятой capture. Busy откладывает работу
  до следующего watchdog tick, без replacement/лог-цикла/нового таймера. Любая
  ненулевая ошибка запрещает создание observer pair; mixed busy/cleanup и
  последующая reconciliation требуют отдельной приёмки. Non-busy error либо
  реальная попытка restart расходует one-attempt budget на original producer/id.
  Отдельная неизвестная native/custody fault не получает blind retry.
- Нет Manager.Stop/Start, proxy changes, policy Arm/Release, render/probe/intent reset;
  existing helper retirement может отозвать только свои capture scopes/filters.
  Нет нового HEALTHY или сетевого измерения из этого helper. App state только
  повторно проецируется. Start/Stop, зарегистрированный во время setup, может
  использовать готовую пару; старый scope не переписывает новое состояние и
  не запускает VPN. Его последующий нормальный Start проходит все прежние gates.

Source5 Go/4 existing docs, gofmt5 exit0 receipte2d48a, manual caller/lock/diff/
byte-SHA: `.artifacts/memory/disconnected-observer-source-evidence.json`.
Tests/fixtures не добавлялись/не менялись/не запускались. Compile/build/vet/
typecheck/product parser/check/doctor/install/gen/SDK/helper/product/crypto/native/
browser/admin/live/DNS/server/owner-values/other-role/chat/subagent execution
отсутствует, OS settings не менялись. API/fixtures/watchdog ctx/auto-connect/
queued Start-Stop-Profile/helper busy-mixed error/retirement/registration/late
callback/UI/native/resources/OS приёмка OPEN. Нет measured wake latency или
kernel forwarding proof. Далее generic decision-to-mutation и legacy publication
atomicity; весь PC00…09/old9/SDK/W3b/interprocess/durability/UX/release/comparative
OPEN, native=false/IPC9/Job8/TUN unavailable/schema2 unsupported прежние.
