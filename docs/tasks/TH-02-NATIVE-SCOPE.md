# TH-02/04 — ручное native DNS-наблюдение Windows-сеанса

Статус: V2-149, 2026-10-25, **исходники, unverified**, только PC/Windows.
Продолжение [image owner](TH-02-NATIVE-IMAGE.md), а не завершение TH-02 или всех
TH-00…08. Пользователь запретил тестирование: тесты не добавлялись/не менялись/
не запускались; compile/build/vet/check/module/gen/SDK/helper/native/network/live
не выполнялись. `gofmt -w` шести Go-файлов exit0 — только форматирование.

## Действие и место в существующей архитектуре

Добавлены `App.RunNativeProbe()` и отдельный `App.GetNativeProbeReport()`.
Это ручное optional действие, без аргументов endpoint/packet/path/timeout.
Оно не запускается из startup, `appProtectedProbe`, watchdog или getter.
Применяются существующие `operationMu.TryLock`, `beginOperation/endOperation`,
`netEpoch.bindOperation`, Manager owner и общий `ObservedClock`. Не создаются
очередь действий, таймер, новый owner engine или отдельная модель здоровья.

Доступны только fixed A/example.com DNS-попытки UDP4/TCP4/UDP6/TCP6 к двум
источникам из существующего протокола, максимум8 попыток. Общий App context12s
охватывает подготовку и capture; core wrapper сохраняет свой optional admission,
bounded IPC и исходный Job/image owner. Cleanup/Win32/системные расходы этим не
объявлены hard12s SLO. Занятый lifecycle не ставит ручное действие в очередь.

Custom `probeFactory`, `tunnelBoundary` или `policy` отключают нативный путь до
runtime/interface/WFP обращений. Пустой/неcanonical packaging pin, SOCKS-only,
неизвестная физическая epoch0, нет активного owner/clock/trust — unavailable.
Getter capability теперь сообщает `execution_state=manual_action`, сохраняя
`image_state=not_checked`: факт существования action не подтверждает helper.

## Настоящая область действия

`native_probe_scope.go` хранит точный applied config pointer и его owned Clone,
Manager/Engine pointers, session/owner/generation/intent, профиль/source/pin policy,
validity и selector, nonzero наблюдаемую сетевую эпоху, process observed clock,
compiled pin и приватный worker/config/selector/LUID lease.

Перед запросом и после cleanup/decode callback core wrapper проверяет:

1. Тот же applied snapshot/Manager/Engine/owner/generation/intent, protection desired,
   нет profile transition/closing/trust block/custom компонентов.
2. `currentTunnelRuntime`: acknowledged worker instance/config digest/TUN/default
   outbound соответствуют owned конфигурации и исходному worker.
3. Повторно наблюдаемый nonzero LUID прежний; существующий `verifyWFPPolicyContext`
   подтверждает ожидаемую форму текущей policy. Новый permit не устанавливается,
   `Establish`, внешний curl и proxy/DNS/TUN/WFP mutations action не вызывает.
4. Та же известная epoch, непрерывное наблюдаемое время и неистёкшие текущие
   profile/certificate ограничения. Clock не объявлен аутентифицированным UTC.

Manual Stop меняет intent и отменяет context до ожидания operationMu; up/change
отменяет epoch-bound работу. Проверки получают этот context. Scope mutation,
incomplete decode, image/cleanup/admission failure не публикуют partial result.
Ошибка очищает только свой draft pointer, не обновляет чужое наблюдение.

После полного core guard финальная публикация использует короткий memory gate:
`runtime -> Manager.GuardRunningOwner -> App -> netEpoch -> ObservedClock`.
В нём нет IPC, interface/WFP/file вызовов, событий или lifecycle callbacks.
Getter повторяет memory gate и срок актуальности; он не запускает OS-наблюдение.
Applied config считается immutable после публикации, как и в существующем App.
Это source-contract; возможные OS/worker/route гонки требуют отдельной приёмки.

## Отдельный отчёт, ограниченные факты

`NativeProbeReportView` — значение с schema/state/reason, началом/завершением/
истечением, четырьмя fixed family slots и explicit count. В каждой семье до2
fixed attempt slots: публичный фиксированный endpoint/dependency, answered или
unavailable, stage/reason из конечного словаря, duration и размер ответа.
В report отсутствуют raw packets, nonce, DNS query IDs, RuntimeIdentity/LUID,
source/config hashes, пути, сертификаты и credentials. Ошибки action редактируются
до конечных публичных сообщений; сырые OS/worker/helper ошибки не возвращаются.

`observed` означает только завершённый batch. Все четыре семьи могут иметь
unavailable, и action завершится без aggregate VPN failure. Срок фактов <=60s
после завершения и не позже известных profile/identity deadlines. Scope change
возвращает `stale`, TTL — `expired`, без старых attempt slots; polling не освежает
время. Копирование value arrays не позволяет менять private cache через getter.

Обязательный `ProbeReport`, `state.Probe`, его timestamps, `lastObservation`,
`lastHealthObservation`, channel health, failover, SM и retry не изменяются.
DNS answer не доказывает отсутствие утечки, IPv6 egress, независимость путей,
DNSSEC, DPI-причину отказа, end-to-end VPN availability или скорость. Проверка
policy shape до/после не является атомарной kernel route attestation пакета.

## Текущие границы и продолжение

- Wails bindings и frontend consumer этого нового API ещё не подключены; generated
  файлы вручную не менялись. Потребитель должен явно различать no_data/running/
  observed/stale/expired и family failures без зелёного aggregate VPN статуса.
- Cache один, приватный и без истории/пакетов. Getter проверяет memory subject;
  он не доказывает неизменность kernel policy/LUID между OS-наблюдениями.
- Pair updater/rollback/owned installer, полный bounded controller/backoff,
  независимый egress quorum, client migration/custody/DR и остальные этапы открыты.
- Compile/codec/protocol/Wails/locking/Stop/epoch/clock/worker/LUID/policy/race/
  cancellation/Job/image/cleanup/resources/Win32/IPv4/IPv6/EXE/NSIS/live acceptance
  отложены до явного изменения запрета тестирования. Production readiness и
  comparative superiority не подтверждены; VPS/Android/harness не затронуты.
