# PC-03 — DNS поколения original worker при смене сети

2026-11-05, V2-213, Windows **source/unverified**. Полный
[PC00…09](../planning/knowledge-application/PC-PLAN.md), продолжение
[root path revocation](PC-03-NETWORK-HANDOFF.md) и
[service refresh](PC-03-ENDPOINT-REFRESH.md). Tests/build/worker/network/native
execution запрещены владельцем; новые процедуры в этой задаче не запускались.

## Источник пробела

Root отзывает старую операцию, но committed box имеет независимый lifetime.
Его ordinary service DNS flight мог пересечь delivered network event, включая
временную подсказку и late local fault. Нужен собственный original DNS-generation
retirement, сохраняя независимость уже established protected transports.
Прежний netwatch запускал go-реакцию на каждый up/change; это не bounded owner
с одним ожидающим событием и собственным подтверждением завершения.

## Изменения по компонентам

1. `bootstrap_network.go` владеет original per-box path: ctx/cancel/live,
   borrower count/drained. Pause отзывает live под worker.mu, затем отменяет
   снаружи и ждёт original flight/cancellation callback/borrowers, проверяет
   **тот же** work domain. Successor создаётся после подтверждённого retirement.
   При uncertainty retained fault/domain закрывают admission; domain не заменяется.
   Новое поколение наследует box lifetime, а не временный control context.
2. `bootstrap_worker.go` связывает каждую операцию/flight/caller с original path,
   проверяет его при выдаче результата и перед borrower release. Existing six
   strategies/clock/TTL/current-source contract сохраняются. Local error отмечается
   до release и только для того же current path; delayed old error не загрязняет
   stamp новой диагностики. Общий cap16 callers включает завершённых followers.
3. `endpoint_hint.go` удерживает original seed path. После pause/resume seed не
   переиспользуется, даже если его UTC window ещё свежий. Initial private probe
   key/one-use/TLS identity gates сохранены; ordinary data не получает seed.
4. `engine_dns_handoff.go` — parent/child мост к существующему engine, без нового
   контроллера или resolver. Parent проверяет actual process instance/full runtime
   digest/selector/TUN name/admission epoch/admitted phase; child сверяет actual
   box/selector/admission под engine mutex. Pending ordinary/native receipts
   отзываются. Existing open/epoch сохраняются; control не выдаёт новый допуск.
5. Worker protocol теперь **9**, argument `--internal-engine-worker-v9`.
   Два exact action `dns_pause/dns_resume` используют прежние subject/tag/epoch
   поля, без новых payload полей/credentials/бюджетов/Report/token. Reply обязан
   подтвердить тот же actor/config/tag/epoch/admitted state. Несоответствие/error
   идёт через original process retirement; нет embedded/legacy fallback.
   Old8 rejected по прежним exact argument/hello/version правилам. Число8 в
   существующем engine Job process ceiling — другая величина, она не менялась.
6. `dns_network_handoff.go` даёт существующему netWatcher один original task,
   cancel/done/fault и один pending value. Active control заканчивается до
   обработки latest pending event; нового go на каждый flap нет. Poller делает
   только enqueue. Root захватывает original observed path, current intent/
   Manager owner/generation/runtime/config и проверяет их без value locks через
   I/O. OperationMu сериализует с platform lifecycle; six-second I/O budget
   начинается после mutex и сокращается оставшимся session lifetime.
7. Down запрашивает pause, up/change — retirement/resume и только затем existing
   reprobe/preference/retry reaction. Stale/no-ACK/changed-owner response не
   становится Ready/health. No-session/busy case передаёт existing reaction для
   Start/fail-closed logic, не утверждая worker ACK. Reset failure current actor
   идёт через existing failCore; channel-health failure из неё не создаётся.
8. Net-change reprobe/fast-reprobe наследуют original observer context.
   `closeAfterLifecycle` отменяет observer до StopCore; прежний поздний stop
   повторно ждёт **тот же** owner после engine retirement. Wait2s не доказывает
   произвольный mutex/syscall/cleanup bound; незавершённый owner не стирается и
   не заменяется. Terminal task fault retained, current captured actor использует
   existing failure cleanup. Полная reconciliation/manual reconnect остаётся OPEN.

Успешный переход не вызывает SDK DNSRouter.ResetNetwork, outbound transport
Reset/Close, смену selector/profile/pins, WFP/TUN/proxy/DNS settings или новый
worker. Его ACK — DNS-work receipt, не native path measurement, profile proof,
снятие strict-барьера или обновление старого положительного отчёта.

## Реальные пределы source-контракта

- Отзыв внутри worker начинается при обработке команды. Debounce, polling,
  очередь, active RPC/probe и mutex могут задерживать доставку. Root invalidation
  и private command не образуют atomic transaction с OS route/kernel admission.
- Уже логически выданный SDK адрес или начатый physical dial находится за этой
  Lookup boundary; отмена/close всего SDK и фактическое переподключение отдельно
  не доказаны. Stock resolver pool не resetится этим срезом. Healthy established
  flow сохраняется при успешном control, local uncertainty может потребовать
  original engine retirement по прежней safety policy.
- One pending slot объединяет flaps до latest observed source. Old command может
  завершиться после нового event; его ACK не публикуется как новый маршрут,
  следующий pending command ещё должен быть обработан. Нет обещания нулевой
  задержки, hard deadline/RSS/stdlib task ceiling или исключённого package-level race.
- Generation ctx — процессная custody/freshness, не verified physical network
  identity. Profile/operation/clock/role/actual TLS и native/OS gates независимы.
  Parent/child bind к существующему admitted actor не обновляет его authority.

## Проверенное и открытое

Source9 Go/4 docs, gofmt/manual source/caller/diff/byte-SHA inventory:
`.artifacts/memory/dns-handoff-source-evidence.json`. Ни один worker/private
command/SDK/parser/HTTP/DNS/native/OS call не выполнялся. Tests/fixtures не
добавлялись/не изменялись/не запускались, включая legacy protocol8 expectations.
Нет compile/build/vet/typecheck/doctor/install/generation/browser/admin/live/
server/owner-values/other-role/chat/subagent действий. Это **не runtime PASS**.

Открыты: IPC9 codec/hello/replay/wrong-actor/old8 fixtures, startup/worker/API и
packaging/image compatibility; paused/no-flight/leader/followers/stock-fallback/
seed/local-stamp/generation/cancel/late reply/partial Close/poison races; concurrent
Stop/profile expiry/new Manager/down-up flaps/empty/unknown/TUN-disabled observer;
shutdown and terminal observer reconciliation; actual SDK dial/pool/new family
fallback and healthy-flow continuity; Windows routes/WFP/IPv4/IPv6/UDP/TCP/
actual client-server pair; resources/RSS/CPU/latency/hard I/O. Current binary
ещё не пересобран. Prior protocol8 source evidence остаётся историческим.

Native acceptance const=false, TUN unavailable, schema2 install unsupported.
Весь PC00…09 и прежние FR/TH/OFF/COM/RQ/TR/TH00…08, W3b/SDK/interprocess/
durability/UX/release/comparative остаются OPEN. Следующее source — control-fault/
reconnect ownership и actual route/observer observability без запуска измерений.

<a id="pc-v227-dns-event-scope"></a>

## V2-227 — исходный запрос, событие и сессия DNS-контроля

2026-11-05, Windows **source/unverified**. Дополняет V213; runtime-приёмка
не проводилась. Полная цель PC00…09 и прежние девять этапов сохраняются.

### Найдено в исходниках

`applyServiceDNSChange` брал текущий `probeIntent` и runtime после ожидания
`operationMu`; success вызывал unscoped `onNetPathChanged`. Поэтому событие могло
получить более новый Start/actor или поздний retry. Даже захват непосредственно
в `submit` оставлял epoch/log/cancel callbacks перед original registration.
Это source finding; воспроизведение и tests запрещены владельцем.

### Изменено

- `dns_change_scope.go` фиксирует request/source и applied config/Manager/engine/
  owner/generation до callbacks. Переиспользуется `appliedProbeSubject`; historical
  report даёт только original identity. Нет текущего App-source fallback.
- `bumpEvent` возвращает точный path/control source из того же epoch commit;
  callback не может подменить событие более поздним global current. Каждое
  delivered down/up/change отзывает предшественника. Power cancellation отзывает
  control source на existing poller stack; native callback остаётся atomic+wake.
- Request/actor и event связываются без recapture. Single active/single pending
  owner сохраняется; после operationMu читаются только original record и checks
  его актуальности. Actor-less/busy событие не может получить новый actor.
- Отдельный DNS-control binding допускает down pause с revoked physical path,
  но не probe/health/native/forwarding authority. Попытка привязать этот context
  к обычному protected probe path получает отрицательный mismatch.
- Optional core Dependency/Blocked сохраняют original task и power denial.
  Generic/independent cleanup contexts прежние; existing pipe exchange слушает
  третий passive revoke channel без новой goroutine/timer/monitor. Rebind не
  стирает original optional dependency/operation и не меняет power pointer.
- Original DNS task source отзывается под publication lease до stop callbacks,
  при terminal task/dependency failure и перед ожиданием native cleanup poller.
  Source notification не доказывает task.done, Job/reap/handle retirement или
  право заменить producer; прежние actual retirement gates остаются.
- Production reaction использует исходные ctx/config, не unscoped callback.
  Actor-less retry проверяет тот же registered intent/source/event/path и
  отсутствие нового applied actor/pending Start; existing retry owns episode,
  exactly-once reservation и прежние per-path/burst limits.
- Pure canceled pre-I/O с всё ещё running original engine не становится policy
  fault. Независимая custody failure, действительно failed/retired worker и
  panic error сохраняют negative cleanup исходного actor после event/task loss.
  Новый request/actor ими не усыновляется. First terminal task fault удерживает
  registered source, original runtime tuple и first cause через оба finalizer.
- Source review дополнительно обнаружил retirement даже при отказе `request`
  до reserve. `sourceCanceledBeforeRequest` используется в readonly current/
  selector и DNS control: original sequence snapshot под Engine.mu обязан
  совпадать, source отозван, error canceled, worker не unusable и не exited.
  Только этот случай сохраняет healthy predecessor. Reserved ID/started exchange/
  malformed response/uncertainty не получают исключение и идут в прежний reap.

### Доказательства и открытые границы

Десять Go source-файлов и четыре собственных документа; только exact-file gofmt,
manual source/caller/lock/diff review и byte/SHA identities. Артефакт:
`.artifacts/memory/dns-intent-source-evidence.json`; original bytes и прежний
composite task сохранены отдельно. Все acceptance остаются OPEN.

Не добавлялись/не менялись/не запускались tests/fixtures; нет compile/build/vet/
typecheck/product parser/check/doctor/install/generation/SDK/helper/product/
crypto/native/browser/admin/live/DNS/server/OS-settings/owner-values/other-role/
chat/subagent execution. Legacy fixture/API compatibility не подтверждена.

OPEN: delayed callbacks/new Start/Stop/retained generation; no actor/busy/retry;
flaps/identical scalar epoch/down-up/power; source/ctx/deadline/mixed local error;
panic/partial command/first fault; original task/process/Job/I/O cleanup; real
SDK/pool/established-flow continuity, Windows network/protection path and resource/
latency/performance acceptance. Mutex acquisition не ограничен six-second I/O
budget; atomics/passive channels не доказывают kernel revocation или прерывание
уже начатого syscall/SDK lookup. Задержка existing fast-reprobe wait возможна.
Native=false/IPC9/Job8/TUN unavailable/schema2 unsupported не изменены.

Следующий source-срез: remaining queued worker/native mutation и negative fault
ownership. Полный PC00…09/old9/SDK/W3b/interprocess/durability/UX/release/
comparative scope не завершён.

<a id="pc-v234-command-replies"></a>

## V2-234 — common reply predicates и finite DNS faults

2026-11-05, PC-01/03 source/unverified; полный PC00…09/old9 сохраняется.

### Исходная потеря и реализация

После V233 original pipe owner проверял смысл только probe reply. Проверки
admit/admit_native/TUN open-close-confirm/DNS pause-resume оставались в parent
после ожидания. Cancellation могла обойти разбор доставленного ответа.
`localServiceDNS` возвращал typed bootstrap cleanup error, но child оставлял
только общий dns_handoff_failed; смешанная отмена теряла независимую причину.

Новый `worker_reply.go` — pure component внутри прежнего core package.
Подходящего общего аналога не было: V233 покрывал probe, остальные predicates
повторялись у отдельных parent callers. Dispatcher теперь композирует probe и
control checks после wire version/ID/native-token validation до original done.
Common scoped-state predicate проверяет original subject/tag/epoch, ожидаемые
admitted/nonce phases и отсутствие лишних ordinary/native tokens/report.
Parent admit/DNS/native confirm/commit и прежний netprobe scope adapter используют
те же predicates. Native permit/context/process/Manager/source/actual cleanup
проверяются их прежними владельцами. ACK не создаёт новую authority.

Control failure codes заданы по действию; state_error и expected failure
допускают только пустую отрицательную форму. Она даёт unavailable без local
cleanup/trust flag. Probe/control используют общий empty-failure predicate.
Противоречивый/foreign payload сохраняется как protocol error до done, в том
числе при concurrent cancellation; positive reply всё ещё требует current ctx.

Только после фактического pause/resume исходного bootstrap, прошедшего locked
box/subject/tag/admission/epoch checks, finite local error может создать private
`workerDNSFault`. Он хранит original request ID/action/subject/tag/epoch и cause.
До начала bootstrap работы такого marker нет. Child повторно сопоставляет
request с private fault и строит прежний bounded workerProbeFailure report.
В существующем dns_handoff_failed идут только finite cleanup/trust flags/reason,
original tuple и фиксированная диагностика. Нет raw error/path/result/health,
tokens или claimed admitted state; current-state query не обновляет старый факт.

Parent принимает такую отрицательную форму только для exact DNS action/tuple,
без tokens/scope/admitted/AllPassed, с bounded results и finite lifecycle cause.
Original ownedPipeIO публикует cause до done, V232 wrapper сохраняет completed
cause после independent bounded reap. App DNS finalizer уже распознаёт typed
custody через localRetirementFailure; отрицательная очистка остаётся у исходного
actor. Raw Stop/TUN scope.retired receipts используют actual Job/I/O/handles.
Pure cancellation/no reservation/source gap не получают invented custody flag.

IPC9 fields/version и Job8 прежние; новых goroutines/watchers/timers/budgets нет.
Forced retirement может не дать child доставить отчёт: undelivered/unfinished
facts остаются unknown. Нет гарантии доставки diagnostics, syscall preemption,
реального protected route, admission/retirement, safety или performance proof.

### Evidence и остаток

Source9Go/4docs; exact gofmt9 exit0, manual original/current diffs и source
caller/import/type/lock/phase/fault/cancellation/reap review, named byte/SHA.
`worker-command-source-evidence.json` и exact original13 snapshots (новый source
с пустым before), prior task71 rev467 — `.artifacts/memory/worker-command-*`.
PLAN/task дописаны с сохранением старого byte prefix; весь goal/constraints/
tasks0…70/acceptance/decisions/прежние checks сохраняются, acceptance OPEN.

Tests/fixtures/build/compile/vet/typecheck/product parser/check/doctor/install/
generation/SDK/helper/product/crypto/native/browser/admin/live/network/owner
values/other-role/chat/subagent execution не выполнялись. OPEN: compilation/
legacy fixtures/API/IPC9, ACK phases/nonce/epoch/foreign/empty/contradictory/
negative payloads, private DNS marker/no work/combined custody+trust, cancel/
deadline/source/late frames/reap/race/Job/handle/resources/OS.
Start/current/switch/stop late semantics, native Release/launch atomicity,
scalar async provenance и full PC00…09/old9/SDK/W3b/interprocess/durability/UX/
release/comparative также OPEN; native=false/TUN unavailable/schema2 unsupported.

<a id="pc-v314-dns-transport"></a>

## V2-314 — исходный DNS-запрос и его socket при смене сети

**Source/unverified, 2026-11-08.** Полный PC00…09/old9 active,
100 OPEN/56 NOT_RUN, без waiver.8 Go/4 docs; tests/fixtures/build/typecheck/
SDK/options/codec/check/worker/native/browser/admin/network не выполнялись.
Предыдущий goal turn V312/313 дал реальные source/doc/evidence изменения;
это progress, не blocker streak. Публичные live-отчёты не являются нашей приёмкой.

### Причина по закреплённым исходникам

UDPTransport.Exchange в sing-box1.13.19 берёт ConnPoolSingle.AcquireShared и
запускает recvLoop. Возврат Lookup/отмена вопроса оставляют shared connection.
ConnPool.Reset заменяет state, closeState не возвращает результаты conn.Close,
Close самого pool возвращает nil. Отдельного SDK join receive task нет.
Поэтому прежние path/flight/caller receipts не подтверждали retirement этого
socket, а простой Reset не давал недостающего проверяемого результата.

DNS client Lookup делегирует A/AAAA task.Group; cached sing0.8.13 Group.Run
ждёт taskContext.Done после cancellation/cleanup. Это source-связь вызывающих
стеков, не доказательство native retirement/runtime scheduling.

### Компоненты и reuse

Новые production bootstrap_transport.go и bootstrap_exchange.go отвечают за
два разных контракта: SDK adapter/ресурсное завершение и bounded service exchange.
Существующий pool не даёт требуемого Close/join API; его копия или второй DNS
controller не создавались. Переиспользуются original SDK transport registry/
option decoders, DNS client/router/TransportAdapter/NewRemoteDialer, miekg codec,
preflightNetwork/preflightConn sticky Close и прежний строгий netprobe wire gate.

Engine регистрирует wrapper только в original per-box service context. Единственный
bootstrap-dns owner резервируется до SDK dialer construction. Защищённый final
TLS/proxy transport строится прежним SDK registry. Constructor принимает ровно
ожидаемые typed remote options и literal resolver из normalized cfg; legacy/local/
другие dial options не присваивают эту роль. V313 plain-direct lowering вынесен
в общий helper без изменения значения; custom direct сохраняет detour/SDK Start
InitializeDetour semantics. SDK option registry остаётся прежним decoder.

Endpoint router проверяет именно original private transport этого worker, затем
передаёт worker/path/host context только своему service Lookup fallback. Ordinary
user Exchange/serialized request не создают этот use. Original policy, bootstrap
hint/TTL/one-use, exact domain resolver/no-cache и outer flight/path checks сохранены.
Lookup по имени endpoint пользователя по-прежнему следует protected final.

### Запрос, корреляция и пределы

- Под worker.mu сначала проверяются original context/transport/path и резервируется
  query. Cap2 включает завершение/неопределённые записи; затем copy/entropy/buffers/
  dial. Это два A/AAAA exchanges текущего service flight, не глобальная rate/RSS
  гарантия всего SDK/процесса. Borrower cap16 и resolver work domain сохранены.
- Поддерживается один IN A/AAAA вопрос точного host и optional empty OPT≤4096,
  без ECS/options/authority/answer sections в request. Copy bounded shape перед
  PackBuffer; crypto nonce16bit задаётся transport, исходный request ID возвращается
  SDK client. Это корреляция, не аутентификация DNS provider или physical path.
- Один fresh connected UDP exchange; только согласованный TC/header/question
  разрешает TCP к тому же literal resolver. Общий исходный deadline не продлевается.
  TCP framing/4096-byte response bound, question/ID/opcode/QR и existing exact
  frame/count/RDLENGTH/trailing gate предшествуют codec Unpack. Correlated TC может
  иметь обрезанный RR body; TCP ответ обязан быть полным. NXDOMAIN/NODATA сохраняют
  SDK negative-answer обработку. Ни прямой user-DNS fallback, ни system DNS нет.
- 4096 bytes/прежний frame RR limit — явная поддерживаемая граница. Большие ответы
  недоступны, а не усечённый успех. Это не весь DNS-протокол/DNSSEC/любые типы;
  scope соответствует service Lookup A/AAAA. Actual provider/family/large-answer
  compatibility и доступность остаются OPEN.
- SDK remote socket dialer сохраняет platform controls/кастомный direct detour;
  его скрытые dial tasks/parallel interfaces и native socket defaults этим не
  аттестованы. Transport не хранит socket pool или отдельный receive goroutine.

### Отмена, завершение и uncertainty

PreflightNetwork.dialWith расширяет прежний reserve-before-acquire/sticky-Close
owner делегированным SDK dialer. Старый dial продолжает вызывать net.Dialer тем же
путём. Late acquired object/conn+error/неверный nil,nil удерживаются и проверяются;
deadline задаётся на исходный Conn. Close по-прежнему выполняется один раз и
ошибка не превращается в успешный повторный net.ErrClosed.

Каждый query имеет original ctx/cancel/owner/done. Context cancellation callback
закрывает original connections; finalizer ждёт его actual return и итоговый Close.
Обычный result также проверяется по original path/parent/deadline после cleanup.
Положительный ответ не переживает pause/Reset/Close. Unknown Close/panic/Goexit
оставляет query в map и его owner/error, перед release worker.mu ставится negative
fault/admission fence, затем existing worker.poison выполняет полный отзыв.
Done закрывается с отдельным cleanupErr: завершившаяся ошибка не равна clean receipt.

Retirement имеет свой retained owner/done/error; concurrent Reset/Close joins
original task, затем повторно читает queries под уже closed/paused gate, не берёт
чужой ACK за новый scope. Все captured calls сначала canceled, затем inspected
Close и joins. Native I/O/join выполняются вне worker/admission value locks.
Panic/Goexit/timeout сохраняют failed retirement/queries, запрещая replacement.
Reset имеет void SDK API: failure сохраняется как negative worker fault.

Pause/resume до нового bootstrapPath явно завершают эти queries и прежний
flight/borrower/work domain. Worker Close использует общий timeout context для
query/flight/borrower joins; native blocking Close не объявлен прерываемым по
этому deadline. Original process/Job containment и его неопределённость отдельны.
User established flows не resetятся этим адаптером; source-контроль не создаёт
новую route/forwarding/native/trust authority и не измеряет их непрерывность.

### Evidence и остаток

Manual caller/diff/cached source/lock/defer/error/correlation review и touched
gofmt8+3+1exit0.12 snapshots(10 existing/2 new), original task71rev731,
11 immutable reviewed+14 cached primary source SHA. Evidence:
.artifacts/memory/bootstrap-v314-dns-transport-source-evidence.json.
Source guards/форматирование/SHA не являются compile/SDK/native/network PASS.

OPEN: compiler/API/fixtures/registry/context propagation/constructor/Start,
private-source A/AAAA/EDNS/TC/negative/malformed/large/IPv6/custom direct cases;
Reset/pause/resume/Close ordering, partial acquisition/panic/Goexit/hanging Close,
SDK inner tasks, OS sockets/resource/time/global-rate/RSS/perf; protected final
native route/WFP/TUN/egress/release и все PC00…09/old9 acceptance.
Следующее source — logical service address→actual transport dial handoff:
уже отданный SDK адрес не становится новым по одному path receipt.
Все bootstrap/bridge/native/chain/relaunch/parent/helper gates=false,
ranBeforeJob sticky, GUI6/R1/U1/IPC9/Job8, TUN unavailable/schema2 unsupported.
Android/shared/configs/harness/dependencies/Git/ОС/сервер/чужие чаты не менялись.


<a id="pc-v315-service-dial"></a>

## V2-315 — исходный путь до DNS и передача результата dial

**Source/unverified, 2026-11-08.** Полный PC00…09/old9 active,
100 OPEN/56 NOT_RUN, без waiver.10 Go/4 docs; tests/fixtures/build/typecheck/
SDK/options/parser/worker/native/browser/admin/network не выполнялись.
V314 был progress; новая source-работа не сужает общую цель до одного P0.

### Почему внешний ctx недостаточен

В cached sing-quic HY2 Client.offer shared pending принадлежит client lifetime;
foreground timeout прекращает ожидание, но не сам offer. OfferNew передаёт свой
ctx socket dialer, а поздние port hops — lifetime ctx. В VLESS TLS/transport/mux
также расположены внутри protocol object. Обёртка над внешним DialContext не
покрывает эти вызовы и может скрыть optional SDK lifecycle/interface-update API.

Новые service_dial_config.go, service_dial_outbound.go, service_dial_task.go
разделяют deterministic SDK projection, существующий registry/dialer API и
original pending acquisition/transfer. Protocol objects не оборачиваются.
Это исправление dial-call boundary; shared protocol-session publication ещё OPEN.

### Перевод SDK view и binding

Для normalized VLESS/HY2 создаётся deterministic private tag с полным SHA256
исходного tag. Reserved prefix в original config отклоняется, коллизия также.
Новый private direct projection содержит только original detour/resolver/TCP
socket options. Credentials, TLS, pins, server/mux/hopping остаются в original
protocol outbound. У protocol socket options заменены на private detour;
user selector/route.final/rules не получают этих private tags. Explicit final
сохранён даже при создании private objects перед protocol constructors.

Обе SDK точки используют прежний sdkRuntimeConfiguration; copy-on-write Outbounds
и DNS не меняют original cfg/raw bytes/BootstrapHints/subject/hash/IPC/rollback.
SDK JSON получает отдельный maxWorkerConfigBytes bound и не становится новым
normalized format. Неподдержанный namespace/options даёт refusal до замены owner.

Original per-box registry распознаёт только собственный spec. Unknown reserved
tag не делегируется stock direct. Private direct options повторно decoded тем же
official option decoder, exact value comparison; никакой runtime-доверие этим
не создаётся. Original constructor pointer резервируется до SDK dialer creation
и остаётся в worker даже после Close, без внутрибоксового replacement.

Endpoint DNS view перемещён на этот original private socket dialer и использует
прежний source channel tag/endpoint digest/hints/current path. Original protocol
constructor проверяет exact source server/port и private-only dial options.
HY2 UDPFragmentDefault переносится на actual socket options; custom detour и
TCP keepalive/TFO сохраняют их original declarations. SDK optional behavior,
ordering/KTLS/transport compatibility этим ещё не приняты.

### Владелец вызова и передача

- До delegated DNS/dial захватывается original worker/path/parent, проверяется
  exact original outbound pointer, source endpoint/network и declared hop port.
  AllowsServicePort переиспользует тот же parsePortRange, что config validation.
  Cap16 — прежняя численная worker budget, но отдельные pending dial-call slots,
  не предел всех SDK/native tasks или измеренный optimum throughput.
- Path cancellation отзывается до source handoff; callbacks cancel и Close
  имеют original done и join. SDK remote/resolve/default dialer сохраняет свою
  функцию; нет embedded/serial fallback. PreflightNetwork.dialWith держит actual
  returned Conn/conn+error/late acquisition и sticky Close.
- После delegated return временный deadline снимается, callback links stopped/
  joined. Только current original source под worker.mu может metadata-only
  handoff одного completed native object. Preflight owner releases ровно этот
  original interface; Conn/PacketConn optional interfaces не заменяются thin
  stream wrapper. Packet acquisition использует SDK bind view лишь для Close/
  deadline контроля до передачи original PacketConn.
- Own cancel не отменяет parent и не назначает lifetime timer готовому stream.
  Dial-result commit перед pause считается уже переданным объектом; это не
  подтверждение TLS/QUIC handshake, active server identity, physical route,
  complete protocol-session publication или continuity healthy flows.
- Error/changed source не передаёт объект. Unknown native Close/panic/Goexit
  удерживают original task/owner/error, ставят negative fault/admission fence
  перед unlock, затем existing poison. Timeout/cancel не освобождает pending slot
  до original delegated call/callback/Close return. Нативный hard bound не доказан.

Pause/resume сначала retire pending service calls, затем DNS queries/flight/
borrowers/work domain и только потом создаёт successor path. Worker Close делит
один existing timeout context между этими joins. Private outbound Close закрывает
свой admission и ждёт только свои pending calls; весь worker — при явном global
pause/close. Checked handoff не закрывает уже переданный SDK object этим доменом.

### Следующий выявленный P0: inner parallel racers

В cached sing/common/network/multi.go DialParallel возвращает первый success,
закрывает returned channel и отменяет racer ctx. Поздний racer сам закрывает
свой Conn, Close error не возвращается; отдельный join всех losers отсутствует.
Следовательно V315 slot/done описывает внешний delegated dial-call, а не всю
внутреннюю SDK resource custody. Это известный конкретный пробел, не PASS.

Для следующего среза есть existing exported SDK NewResolveDialer: можно передать
ему owned inner N.Dialer и учитывать actual native attempts/late losing Close,
сохранив Happy Eyeballs. Не заменять parallel на serial и не снимать pending
budget по первому winner. Winner handoff, retained loser owner и actual completion
нужно разделить, без нового detached wrapper/reaper, выдающего timeout за retirement.
Далее остаётся independent shared QUIC/mux/TLS session publication/teardown gate.

### Операционный вход и evidence

External report о SFA RU-direct и запросе Windows launch сохранён как attributed
input; не чтение/управление чужим чатом, не authorization на наши live/build/secret
действия и не Windows product proof. Private SFA JSON не открывался/не копировался.
Требование regional direct policy/subdomains/CDN требует signed routing contract;
DNS bypass ручного профиля не переносится без отдельного DNS scope.

Публичные EXE только прочитаны для SHA/size/file timestamps. Correspondence к
current source/helper/build/native gates не установлено. Supported source intake
— staged signed PublicProfile source preview/apply, credentials отдельным
protected transfer/vault; raw SFA JSON не является этим пакетом. Краткая no-secret
справка: .artifacts/memory/bootstrap-v315-service-dial-operational-source-note.md.
Существующий system-proxy Connect seam не объявлен нейтральным запуском GUI или
безопасным совместным режимом с другим VPN. Приложение/Happ/ОС не запускались и
не менялись; внешний отчёт не превращён в собственный acceptance.

Manual source/caller/lock/defer/error/SDK projection/diff review и touched gofmt
10+4+1+2exit0.14 snapshots(11 existing/3 new), original task71rev734,
11 immutable reviewed+14 cached primary source SHA, separate4 source/2 EXE
file identities. Evidence: .artifacts/memory/bootstrap-v315-service-dial-source-evidence.json.
Ни parser/SDK/runtime, ни codec/crypto/native/network проверок не было.

OPEN: compile/API/fixtures/private graph ordering/option decoding/namespace/default
route/DNS roles, vless TCP/UDP/mux/ws/TLS/KTLS/ShadowTLS, HY2 shared offer/hopping/
IPv4/IPv6/PacketConn, native interface/deadline and actual source races;
SDK inner task/loser cleanup/full resource/time/global rate/RSS/latency, protocol
session handoff/identity/PKI/egress/OS/release/comparative, весь PC00…09/old9.
Все bootstrap/bridge/native/chain/relaunch/parent/helper gates=false,
ranBeforeJob sticky, GUI6/R1/U1/IPC9/Job8, TUN unavailable/schema2 unsupported.
Android/shared/configs/harness/dependencies/Git/сервер/системная сеть не менялись.


<a id="pc-v316-native-attempts"></a>

## V2-316 — inner dial attempts и поздний проигравший Happy Eyeballs

**Source/unverified, 2026-11-08.** Полный PC00…09/old9 active,
100 OPEN/56 NOT_RUN, без waiver.6 Go/4 docs; tests/fixtures/build/typecheck/
SDK/options/parser/worker/native/browser/admin/network не выполнялись.
V315 был progress, не blocker streak. Полная цель не заменена этим срезом.

### Переиспользованная точка расширения

SDK ResolveDialer вызывает N.DialParallel/N.DialSerial/ListenSerial. В V315 owner
охватывал только внешний result; late racer закрывал свой объект вне этого scope.
Теперь private constructor создаёт original base dialer без outer resolver,
передаёт owned inner N.Dialer в existing exported NewResolveDialer и сохраняет
original resolver options, fallback delay и parallel predicate: empty detour и
TCPFastOpen=false. Алгоритм Happy Eyeballs не копируется/не заменяется serial.
Original base detour отдельно initialized через SDK InitializeDetour.

Новые service_native_dialer.go и service_dial_retirement.go разделяют actual
base API acquisition/Close hook и passive original retirement. Existing
preflightNetwork/preflightConn reserve/sticky Close/handoff сохранены; добавлен
metadata-only TryLock inspection. Protocol clients/selector/signed normalized
subject/IPC/rollback/trust и protected final DNS этим не меняются.

### Source binding от Lookup до inner IP

Original task по-прежнему резервируется до DNS. Его private context key проходит
через SDK resolver/racer ctx. Endpoint router записывает один bounded address
set того же task/path/endpoint после original Lookup/hint gate. Invalid/duplicate/
foreign task, path, host или адрес/zone/size отказываются до native use.
Новая ошибка после worker.lookup отдельно notes local fault; не теряется в HTTP
строковой ошибке и не становится свидетельством блокировки канала.

Inner entry принимает только original outbound/task/mode, current source,
declared network/port и literal либо адрес из этого lookup set. Address membership
— корреляция внутри операции, не DNS authentication, TTL/physical path receipt
или trust/forwarding authority. Query/hint clock/TTL gates остаются прежними;
никакого synthetic expiry или кеширования адресов между tasks не добавлено.

### Владельцы ресурсов и latency

- До каждого actual base DialContext/ListenPacket резервируется свой original
  attempt с preflight owner. Cap2 соответствует pinned A/AAAA racers; serial
  retries могут переиспользовать только реально retired clean slots. До16 outer
  tasks удерживаются, включая late attempts после возврата winner.
- Native ctx/cancel/Close callback принадлежат одному attempt. Body/defers и
  cancellation callback возвращаются до API publication. Late conn+error,
  stale/canceled acquisition и unexpected nil,nil сохраняют исходный Close/
  error; timeout сам по себе не освобождает ownership.
- TCP/UDP DialContext возвращает SDK внутренний Close-observing wrapper. Даже
  когда race игнорирует возвращённую Close error, wrapper записывает sticky
  negative fault и закрывает admission. В native mutex никто не ждёт под
  worker.mu: retirement inspection использует TryLock и retains unknown.
- Successful winner переносит original native object с очищенным dial deadline.
  Для ListenSerial сохраняется original PacketConn и SDK NAT view; resource
  bind view используется только до handoff. Это не новая stream wrapper policy.
- Winner handoff не ждёт late losing attempts. Его callback/native result
  completion не стирает outer task budget; seal запрещает новые native calls из
  позднего SDK serial/race loop. Existing loser return/Close уведомляют original
  owner; bounded passive progress также выполняется перед new task/control.
  Нет detached reaper, объявляющего дедлайн native retirement.
- Task.done закрывается после tracked calls/callbacks/actual Close attempts и
  outer publication. CleanupErr отдельный: finished negative resource receipt
  удерживает original task/attempt/connection и не освобождает namespace.
  Done не утверждает, что вообще все SDK goroutines или kernel activity исчезли.

Failure/control cancel original tasks и закрывают captured attempts вне value
lock, затем ждут original done. Pause/Close gate закрыт до snapshot, поэтому
late waiter не присваивает новый scope. Success path не делает synchronous
loser Close, задерживающий winning connection. Committed object остаётся у SDK;
late losing fault после commit отзывает admission/worker DNS source и требует
original actor retirement, не ретроспективной новой authority.

Обычная race-loser cancellation не notes новый local failure/не отнимает healthy
winner evidence. Resource/source budget/membership failures и outer canceled/
stale result остаются local observations; actual losing Close uncertainty —
negative worker fault. Это source-классификация, runtime ordering не проверялся.

### Точный остаток scope

Cap2/16 относится к tracked base-dialer API calls/returned resources. Base SDK
DefaultDialer может условно выполнять interface races при networkStrategy!=nil;
custom detour также может иметь собственные внутренние tasks/соединения. Этот
срез не считает их kernel sockets/goroutines и не выдаёт global RSS/rate/time bound.
В Windows config wrapper нет network_strategy override; фактическая достижимость
ветки через original SDK context пока отдельно не установлена. Следующий source
проходит actual context/callers до добавления нового owner/adapter.

Shared HY2/mux/TLS/QUIC handshake/session publication после raw socket handoff
также остаётся independent OPEN. Callback отмены bootstrap worker не доказывает
полную остановку уже переданных protocol objects. Native routing/WFP, protected
payload/peer authentication, healthy-flow continuity и OS teardown требуют своих
gates; ни Node/SDK parser, ни EXE/source hashes не заменяют эту приёмку.

### Evidence и завершение среза

Manual source/caller/lock/defer/error/membership/Happy Eyeballs/packet/slot review
и touched gofmt6+5exit0.10 snapshots(8 existing/2 new), original task71rev737,
12 immutable reviewed+9 cached primary source SHA. Draft body artifact — только
input history, authoritative source — текущий service_dial_task.go. Evidence:
.artifacts/memory/bootstrap-v316-native-attempts-source-evidence.json.
Own ledger progress initial CAS failed revision-conflict after terminal process;
authoritative original ledger remained738, scoped retry1 succeeded739. No harness
files/routing were changed and no application/check command was run.

OPEN: compiler/fixtures/API/constructor/context propagation/current source,
DNS-set/literal/IPv4/IPv6/parallel/serial/TFO/packet/custom-detour cases;
early winner/late acquisition/Close error/hang/panic/Goexit/seal/cleanup races,
SDK internal interfaces/tasks and native hard cancel/memory/latency/global rate;
protocol publication/trust/PKI/egress/OS/GUI/release/comparative/fullPC00…09/old9.
Все bootstrap/bridge/native/chain/relaunch/parent/helper gates=false,
ranBeforeJob sticky, GUI6/R1/U1/IPC9/Job8, TUN unavailable/schema2 unsupported.
Android/shared/configs/harness/dependencies/Git/сервер/сеть/чужие чаты не менялись.


<a id="pc-v317-runtime-graph"></a>

## V2-317 — strict runtime route graph и достижимость base fanout

**Source/unverified, 2026-11-08.** Полный PC00…09/old9 active,
100 OPEN/56 NOT_RUN, без waiver.5 Go/4 docs; tests/fixtures/build/typecheck/
SDK/options/parser/worker/native/browser/admin/network не выполнялись.
V316 был progress, не blocker streak; цель не заменена этим срезом.

### Исходная граница и реальный пробел

Config.Parse запрещал unknown fields и валидировал каждый candidate proxy,
но route.final принимал любой existing tag кроме literal name direct.
Extra aliases/selector/orphan protocol могли находиться вне candidate validation.
Любое nonempty action пропускало проверку outbound reference; например,
поддерживаемый SDK action=route не становился validated route только по названию.
Условие для direct не проверялось. Renderer уже создавал final=proxy,
canonical direct/block, two DNS capture rules и условные process/LAN/.local
split rules. Это source gap strict raw-parser boundary, не наблюдение live leak,
не доказанный внешний exploit и не утверждение о текущем GUI config.

### Изменения по существующим границам

- Новый backend/config/runtime_graph.go выделяет graph/rule gate из Config.Validate.
  Existing protected TLS/Reality/transport/dial/candidate checks сохранены;
  новый gate вызывается после них и до DNS/bootstrap hints.
- Весь graph содержит ровно proxy selector, его already-validated candidates
  и optional canonical plain direct/block. Дополнительные selector/urltest
  aliases, direct aliases, unselected orphan services и reserved-role mutation
  отвергаются. Selector не получает protocol/dial options.
- Final обязательно proxy. Rules могут ссылаться только proxy/direct/block,
  не на отдельный candidate с обходом original selected-channel admission.
  Implicit action или explicit route требуют существующий target; hijack-dns
  не несёт outbound и имеет supported exact port53 либо protocol=dns shape.
  Остальные actions, включая direct/bypass/resolve/route-options, не admitted.
- Direct требует непустой explicit condition; /0 и mapped CIDR отвергаются.
  CIDR syntax, canonical nonempty ASCII suffix (optional leading dot), explicit
  process basename без whitespace/control/path/wildcard, nonzero ports и
  find_process при process rules проверяются до SDK construction. Root/empty
  suffix не становится catch-all. Некорректный raw config нужно regenerate из
  accepted profile; валидатор не чинит его молча и не создаёт fallback.
- IsPlainDirect переиспользует прежнюю exact DeepEqual форму original no-options
  direct из core. Её используют runtime graph, DNS bootstrap role и private
  SDK lowering. Private service helpers по-прежнему создаются после Parse,
  не становятся accepted normalized graph/candidates/IPC subject.
- newSingBox отвергает foreign adapter.PlatformInterface до SDK construction.
  Обычный Engine создаёт fresh Background lifetime с private custody value;
  worker start тоже не переносит caller platform services. include.Context
  добавляет registries, не PlatformInterface. Pinned DefaultDialer устанавливает
  networkStrategy только в platform+auto-detect branch; normalized RouteConfig
  не содержит interface/network-strategy overrides. Переключение SDK branch
  без новой scoped migration/acceptance не разрешается.

Explicit split condition означает declaration, не independently authenticated
routing permission. Проверка не доказывает разумность/полноту произвольного
набора explicit exceptions и не утверждает, что объединение отдельных правил
не может покрыть большой объём трафика. Existing signed profile/render/control
authority, DNS-first capture и forwarding admission остаются отдельными gates.
Новые lists российских сайтов/CDN, raw SFA import или direct final не добавлялись.

### Сверка исходников без исполнения

Вручную просмотрены caller chain Config.Parse → Validate → protected selector →
runtime graph → DNS/bootstrap; Engine protectedRuntimeConfig и повторный Parse
в sdkRuntimeConfiguration до lowering. Сопоставлены renderer's nonempty process
split/FindProcess, RFC1918/.local, literal/domain bootstrap и pure direct/block/
selector declarations. Reviewed исходники показывают отказ для non-proxy final,
orphan/alias, direct without condition, direct /0, root suffix, zero port,
unsupported actions и named candidate rule. Эти expected source outcomes не
запускались через parser/SDK и не являются выполненными test cases.

Touched gofmt5+1 exit0 и manual no-index diff (exit1 = показаны изменения),
9 snapshots8existing1new, task71rev740 archived;15 reviewed +8 cached SHA.
Snapshots/byte identity не доказывают compile/API/SDK/runtime/native/performance.
Own checkpoint pre: два terminal revision-conflict, original740 сохранён.
Read-only Git root timing: installed executable270ms, bundled2524ms при installed
metadata lookup timeout2000ms. Scoped PATH precedence и retry2 записали741;
точная причина прежних конфликтов не доказана. Harness/routing files не менялись.

### Независимый shared-session gate

Pinned sing-quic HY2 offer shares pending and derives offerCtx from client ctx,
не foreground waiter. Raw PacketConn handoff предшествует QUIC/authentication.
completeOffer публикует c.conn под своим connAccess; CloseWithError marks pending
discarded/cancels, но возвращает без join. pending.done закрывается до discard
conn.closeWithError. Поэтому neither socket owner nor cancellation/Close return
доказывает complete old-offer/Close retirement или запрещает late publication
сам по себе. Cap2/16 — base API attempts/returned resources, не all SDK/OS tasks,
kernel sockets, global RSS/rate или hard timeout. Обычная unreachable interface
ветка не требует ещё одной dialer обёртки. ShadowTLS rejected across all nodes
раньше и не включается этим срезом; orphan custom graph теперь тоже rejected.

Следующее source: formal supported before-publication/source-fence и original
offer+Close retirement receipt seam для shared protocol client; сверить existing
Windows SDK request, сохранить early winning latency и healthy established
streams. Не подменять это foreground wrapper, dynamic Context.Done, packet/IP
inference, blind InterfaceUpdated или false timeout retirement. При отсутствии
API — узкий Windows-owned change request и другой actionable PC priority,
а не бесконечное повторение той же недоступной реализации.

OPEN: compile/API/codec/negative compatibility/SDK branch/context propagation,
route/split semantic execution, selector retirement and shared HY2/mux/TLS/QUIC
publication, native cancellation/concurrency/resources/PKI/egress/OS/GUI/release/
comparative/fullPC00…09/old9. Все bootstrap/bridge/native/chain/relaunch/parent/
helper gates=false, ranBeforeJob sticky, GUI6/R1/U1/IPC9/Job8, TUN unavailable,
schema2 unsupported. Android/shared/configs/harness/dependencies/Git/server/
system network/secrets/другие чаты не менялись; tests/fixtures не добавлялись.

