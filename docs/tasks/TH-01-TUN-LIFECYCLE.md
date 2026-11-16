# TH-01 — TUN/WFP gate в lifecycle

2026-10-21, V2-116: implementation, **unverified**. Полная цель TH-00…08
сохраняется. Tests не добавлялись/менялись/запускались; build/vet/harness/
native/live/worker/системная сеть не выполнялись. Только исходники и gofmt.

## Исправляемый путь

До среза App вызывал permit/external TUN gate только после Manager.Start.
Reload, seamless switch и verified rollback ограничивались SOCKS probe.
Имя найденного интерфейса не связывалось с отвечающим worker/config/selector,
а внешний curl использовал PATH, detached loop и неограниченный CombinedOutput.

## Реализация

- `core.LifecycleProbe`: BeforeEngineChange и RunLifecycle. Manager вызывает
  их в Start/Reload/restore/selector rollback и cleanup; все lifecycle probes,
  включая оба rollback пути, используют расширенный gate. Periodic Run не
  добавляет внешний curl. Одна engine ownership цепочка Manager → Engine.
- Полная подготовка Reload включает pinned sing-box option validation до
  изменения ownership/policy. Rejected до замены сохраняет старый runtime;
  если permits уже отозваны, прежний путь требует полного нового lifecycle gate.
  При rejection без замены Restarted=false, а не фиктивный новый процесс.
- ProtectionUnavailable — отдельная uncertainty, не подтверждённая утечка.
  Она запрещает candidate PASS. Вернуть прежний путь можно только после отзыва
  текущих permits и свежего полного старого gate. Подтверждённый leak запрещает
  rollback; неудачная/отменённая очистка оставляет protection и uncertain owner.
- `render.ClientConfig.TUNName` берётся из реально создаваемого inbound, Clone
  переносит scalar. App проверяет совпадение с process mode; probe mode следует
  за snapshot. Константы interface name/external endpoint имеют одного владельца.
- `core.Engine.CurrentRuntime`: исходный owned worker отвечает о текущем
  selector; process instance, TUN name и digest owned config сравниваются с cc.
  Digest — typed JSON value normalization ≤2MiB, без config.Parse/file validation
  под Engine.mu. Он приватный, не выводится в UI/логи; это не remote attestation.
- `tunnel_boundary.go`: lease связывает Engine pointer/instance/config/selector
  с конкретным наблюдаемым LUID. До/после external IPv4 flow и после protected
  probe повторяются worker/config/LUID/policy checks. Lease не переиспользуется
  как новое наблюдение при смене lifecycle/intent/epoch. Факт владения kernel
  TUN handle этим не доказывается; local malicious admin не исключается.
- BeforeChange снимает только owned TUN permits и проверяет сохранённый base
  lockdown. Cleanup продолжает engine/proxy retirement при ошибке guard; обе
  IP families очищаются, отсутствие permits проверяется повторно. Чужой provider
  со столкнувшимся TUN filter ID отвергается, его filter не удаляется.
- Установка permit принимает только exact ожидаемый LUID. Уже корректный набор
  не переустанавливается при seamless проверке. Неоднозначный/down интерфейс и
  изменение LUID отвергаются; ожидание прерывается caller context.
- У native boundary есть явный private injection seam. При custom probeFactory
  без explicit TUN boundary вызывается отказ до native действий. Это не bypass
  production protection. Старым TUN unit fixtures потребуется адаптация позже;
  сейчас tests не правились и не запускались.
- Periodic Observe проверяет текущий worker/config/selector/LUID/policy без
  нового external HTTP и не объявляет старое внешнее наблюдение свежим.

## Внешний IPv4 probe и ограничения

SystemDirectory/curl.exe, скрытый process, первый аргумент -q, proxy/config
выключены, только HTTPS/IPv4. Shared protected admission резервирует3 попытки
и 3×4KiB output, caller deadline и per-attempt context4s; curl max-time3s,
connect2s. V2-146 [owned helper](TH-01-NATIVE-HELPER.md) заменяет прежний
direct-child/WaitDelay на suspended Job limit1/128MiB committed memory,
bounded Max+1 pipe и original-handle complete reap; busy/uncertain owner blocked.
stdout>4KiB отвергается, stderr не публикуется; minimal system env/cwd.
Требуются успешный process exit, отдельный curl HTTP status200 trailer и ровно
один валидный ip, равный expected через net.IP.Equal. Error page, duplicate ip,
overflow/cancel/timeout не становятся egress proof.

Семантика -q/http_code/max-filesize сверена с [официальной справкой curl](https://curl.se/docs/manpage.html).
До curl8.4 max-filesize не ограничивает заранее неизвестный размер; поэтому
есть собственный writer/deadline. Установленная Windows версия curl не запускалась
и не проверялась. Новый Job source contract не является выполненной native
приёмкой или доказательством общего kernel/driver time bound.

1.1.1.1 trace остаётся фиксированной общей зависимостью Cloudflare, отмеченной
в dependency graph как conditional TUN measurement. Это не независимый канал
и не текущая проверка доступности. Endpoint outage/quorum/замена measurement
path остаются TH-02/03/04; ошибки измерителя не равны quarantine всех каналов.
IPv4 echo не доказывает native DNS/UDP/IPv6/bulk, endpoint TLS не TUN handle owner.

## Остаток

Compilation/schema fixtures, Windows nested jobs/worker/query/LUID timing,
WFP reauthorization/established flows, route/driver cleanup, policy corruption/
explicit Disconnect, crash/reboot/ACL/interprocess, late cancellation и реальный
external traffic остаются непроверенными gates. Callbacks/kernel calls не получили
общего hard timeout или отдельной службы. Исторические PASS их не покрывают.

Далее TH-03 clock/profile/cert expiry и trust recovery; TH-02/04 измерители,
bounded actions/quorum/backoff/wakeup. TH-05…08 и все9 этапов остаются активными.

<a id="pc-v228-native-reader"></a>

## V2-228 — original actor, TUN exchange и late local cause

2026-11-05, Windows **source/unverified**. Полная цель PC00…09 и прежние девять
этапов сохраняются. Tests/fixtures/build/native/product execution запрещены.

### Source findings

`exchangeTUNHelper` проверял context перед запуском I/O goroutine, затем decoder/
writer выполнялись без fresh guard; ожидание слушало только ctx.Done, а два
раздельных result/done позволяли потерять уже завершённую ошибку при cancellation.
`NativeTUNActor.Descriptor` не удерживал creation context и использовал supplied
ctx. Перед DuplicateHandle были native queries без нового source check у mutation.
`helperScope` заменял actual guard error поздним context error. Это source trace,
не воспроизведённые runtime defects.

### Изменения и ownership

- Existing TUN exchange теперь один `nativeTUNExchange` с done/message/error.
  Он прикреплён к **тому же** engineProcess/ioDone. Одна прежняя I/O goroutine
  публикует fields до close(done); потребитель читает их только после close.
  Result channel и отдельное второе ожидание I/O больше не нужны.
- Перед заменой ioDone требуется завершение original I/O. При unknown original
  помечается unusable, возвращается local cleanup error; никакой новой команды,
  процесса или забывания старого record. Один current record, без списка/карты.
- Внутри goroutine source/deadline проверяется до write codec и перед началом
  decoder/EOF read, затем после него. Exchange и ожидание original process.done
  слушают уже существующие intent/operation/dependency revoke channels.
  Отзыв не является process/Job/pipe/resource retirement receipt.
- После early cancellation record остаётся у того же original process. Existing
  reaper закрывает pipes и ждёт ioDone. Finalizer добавляет **завершённый** original
  codec/panic error к earlier failure даже после потери интереса caller. Unknown
  не становится отсутствием fault или успехом. Ошибка после forced pipe closure
  не приписывается каналу/оператору как новая causal health evidence.
- Actor хранит original bounded capture ctx, Descriptor сначала проверяет его,
  затем private adapter переносит exact bound OperationSources в supplied ctx.
  Более новый/foreign binding отрицательно отклоняется; original scope не
  подменяется. Child deadline не длиннее original until; cancel в defer.
- Confirmation и consuming worker probe наследуют ту же original operation
  binding перед Engine lock/IPC. Public observation/report/descriptor fields
  не создают её. Existing one-use native receipt/native permit gates сохраняются.
- Generic unbound original context сохраняет существующий contract; original
  own ctx проверяется, source identity из воздуха не создаётся. Это не обещание
  мгновенной передачи generic cancellation в произвольный background caller.
- Source rechecked после readonly current IPC до scope reservation, после
  original transfer и непосредственно перед DuplicateHandle после native queries.
  Remote handle сохраняется прямо в original receipt даже при partial API error;
  parent не закрывает его в своей handle table. Original target Job остаётся
  единственным владельцем неизвестного remote outcome.
- Existing negative scope/filter/Job/image cleanup использует независимый budget
  и не получает positive source precondition. helperScope/panic finalizers join
  earlier local cause с later cancellation, не затирая её.
- Descriptor и начальная/последняя CurrentRuntime проверки сохраняют actual
  engine/cancellation cause вместе с finite scope error, не теряя typed custody.

### Evidence и незакрытые gates

Source7Go/4 собственных docs; exact-file gofmt exit0 и manual source/caller/lock/
async ownership/diff/byte-SHA review. Evidence:
`.artifacts/memory/native-reader-source-evidence.json`. Original task/document
bytes и точный прежний composite task71 сохранены; журнал только дополнен.

Нет tests/fixture edits/run, compile/build/vet/typecheck/product parser/check/
doctor/install/generation/SDK/helper/product/crypto/native/browser/admin/live/
DNS/server/OS-settings/owner-values/other-role/chat/subagent execution.
Все acceptance остаются OPEN; исходники не объявлены runtime PASS.

OPEN: legacy API/fixtures; actor with generic/current/foreign/revoked contexts;
caller/original deadline, delayed timer and passive revoke races; cancellation
before goroutine/write/partial write/read/EOF/exit; malformed/trailing frames;
panic/late mixed local error publication; ioDone replacement and exact reaper
retirement; partial duplication/native grant/revoke; WFP/OS/routes/driver/real
protected path; count/bytes/RSS/CPU/latency/performance. No new monitor/source
watcher or I/O goroutine, но standard child context timer может существовать.
Checks перед codec/API не дают atomic write, syscall/driver interruption или
hard mutex/OS time bound. Native acceptance=false, IPC9/Job8/TUN unavailable/
schema2 install unsupported прежние. Далее common helper async reader и native
DNS, остальные native writers; полный PC/old9/SDK/W3b/interprocess/durability/
UX/release/comparative scope не завершён.
