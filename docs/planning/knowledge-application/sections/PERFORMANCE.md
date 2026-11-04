# Производительность, ресурсы и время жизни операций

## 1. Состояние и граница плана

План применения накопленного корпуса R04: **378 разделов из 41 документа**,
включая архивы, R04-001…011, независимые рецензии и R28-C04/Z04/Z05.
Это уточнение существующих **TH-02/04/06, COM-12, RQ/TR, AND-07/11**,
а не ещё один транспортный roadmap. Статусы требований владельцев не изменены.
Все перечисленные ниже будущие проверки имеют статус **NOT_RUN**.

Текущая доказательная граница — чтение исходников и ручное сопоставление.
Ни ускорение, ни жёсткое время остановки, ни общий RSS, ни экономия батареи
не установлены. Архивный live HY2 от сентября не характеризует нынешний VPS.

<a id="F00"></a>

### F00 — точные входы и уже существующий код

| Слой | Текущее состояние | Следствие |
|---|---|---|
| Windows | `windows/go.mod`: sing-box **1.13.19**, sing-quic **d83826c306d7**, quic-go **0.59.0-sing-box-mod.4 / ccd46ab…**, sing **0.8.13**. `tools/package.ps1`: windows/amd64, `with_utls,with_gvisor,with_quic,with_clash_api` | Это declared/source closure; resolver, production binary и теги реального EXE здесь не аттестованы |
| Windows owner | `corebridge.go → NewIsolatedEngine`; `worker_process.go`, `worker_host_windows.go`: оригинальные process/Job handles, Terminate/Wait/Empty и удержание при неопределённом reap | Сохранить процессный барьер; не заменять его очередным timeout вокруг goroutine. Engine Job: processes8, **commitBytes=0**. DNS helper128MiB — иной scope |
| Windows file I/O | `filetxn.BoundedReadSlot/BoundedWriteSlot`, fixed journal/target owners уже source implemented; V2-172…181a | Один original I/O owner и sticky uncertainty уже существуют. Не предлагать их заново; числовые file/depth/chunk bounds не дают hard elapsed/global RSS/durability |
| Android | Один private `:vpn`, `NativeWorkerSession`, callback gate32, watchdog, две CONTROL/RETIREMENT RPC lanes, обе actual returns + old death + source/Stub idle перед заменой | Сохранить владельцев и резерв остановки. Это source guards, не измеренный Stop SLO и не per-packet firewall |
| Android native | core **b5ebaa1… / 1.13.19**, whole-libbox overlay **sourceRevision3**; strict-owner/session-cache/closed-history JNI r1. `aarSha256=null`, `go=null` | Stock AAR не совместимая замена. AND-11-S12…15 уже исключают persistent cache/closed-list retention, закрывают Observer admission и сохраняют startup error в source; patch/AAR/device gates открыты |
| Android storage | `NativeScratchLease`, `NativeScratchCleanup/Deletion`: lease до смерти worker; один cleanup owner; FIFO8 known + один cold request; bounded traversal и retained uncertainty | Это не глобальная disk quota и не гарантированный rescan. Не добавлять периодический purge и не удалять живые/неопознанные каталоги |
| Adaptive | API1, `ComposeEvidence`, checked clocks, failover/retry actual-return fields; `PlanAutomaticBackground` — unconnected prototype | Повторный policy engine не нужен. Фактические permits, наблюдения, sockets и исполнение остаются платформам; Android FFI/time mapping не принят |

Source cut и точные hashes: [current source](../../../../.artifacts/planning/knowledge-application/2026-10-28/captures/C04/source-cut.json),
[module source](../../../../.artifacts/planning/knowledge-application/2026-10-28/captures/C04/module-cut.json).
Это последовательные снимки выбранных файлов, не atomic tree или full-module audit.
Read depth и поздние изменения перечислены в §8.

## 2. Знания → существующие компоненты → решение

Идентификаторы F01…F15 ниже используются также в полном [coverage](../../../../.artifacts/planning/knowledge-application/2026-10-28/coverage/C04.json).
Одна family может объединять несколько исторических формулировок; научное
ACCEPTED сохраняет исходный scope и не становится приёмкой реализации.

<a id="F01"></a>

### F01 — полезная операция и наблюдения

**Основание:** [TRANSPORT-AND-RESOURCES §9/13](../../../theory/performance/TRANSPORT-AND-RESOURCES.md),
[REPORT §3/13](../../../theory/research/04-PERFORMANCE/REPORT.md).

Реальный путь — граф стадий, а не фиксированное число RTT. В TUN локальный TCP
завершается в stack; его ACK и gVisor keepalive не подтверждают VPS/target.
Worker IPC управляет lifecycle; payload не проходит через этот IPC на каждом
пакете. Cold core, warm carrier и warm origin различаются. Существующий lazy
`clientConn.Write(nil)` уже запускает HY2 header для server-first протокола.

**Есть:** Windows `ProbeObservation`/`ProbeReport` с route/stage/body/owner/epoch,
`AdmitDiagnostic` с отдельным protected lane. Android AND-07-S1 уже описывает
[MEASUREMENT-NATIVE](../../../android/MEASUREMENT-NATIVE.md) и requested M01…06.

**Дельта / ADAPT, ALREADY_PLANNED:** расширять эти producers минимальными
stage receipts; не создавать второй probe/RTT engine. Для каждого результата
нужны scope, clock domain, owner/generation, config identity, cold/warm state,
not-started/cancelled/overdue/terminal и actual-return отдельно. Registration до
send queue, user-space receive stamp и поздняя доставка callback имеют разные
ошибки. Post-stamp pause увеличивает возраст реакции и не обязана увеличивать
сам RTT sample. Transport ACK, socket return и useful response — разные receipts.

FR-012/TH-02/06/AND-07/11: основной outcome — полезное завершение к заданному
deadline с failures/missing outcomes; затем tails/recovery/churn/CPU/bytes/energy.
Нельзя оценивать только успешные быстрые запросы. При фиксированной работе и
неизменной остальной стоимости `T'_total ≥ (1-f)T_total` и предел ускорения
`1/(1-f)` полезны лишь при известной доле устраняемой работы f.
Числовой выигрыш и f сейчас неизвестны. Для пересекающихся стадий использовать
critical path/max-plus, не складывать независимые параллельные задержки дважды.
CPU считать как раздельные затраты на bytes, packets, новые flows и control/
background work; коэффициенты неизвестны. Меньше copies/syscalls не гарантирует
лучший completion при дополнительном buffering, handshake или большем burst.

<a id="F02"></a>

### F02 — wrapper capability и offload

**Основание:** [REPORT-010 §2](../../../theory/research/04-PERFORMANCE/REPORT-010.md),
TR-01/03, [REPORT §13.4](../../../theory/research/04-PERFORMANCE/REPORT.md).

Windows non-hopping HY2: connected UDP → Unbind → **scalar Salamander** →
quic-go **basicConn** → ordinary Go/Windows UDP. `sys_conn_windows.newConn`
возвращает basicConn даже при OOB API нижнего UDPConn. Scatter/gather частей
одной датаграммы не означает GSO/USO/GRO/URO. M10 counterexample в собственном
vector method Salamander **не достигается этой обычной цепью**.

**ALREADY_PLANNED / DEFER fast path:** RQ-01/02 и TR-01 уже содержат gate.
Перед vector/offload нужно сохранить protect/bind/ownership, один salt8 на каждую
датаграмму, независимые envelopes, receive boundaries, partial/error release,
in-place mutation и fallback semantics. Не раскрывать raw WriteMsgUDP в обход
обёртки. Declared DF=true также не actual setDF/wire receipt. Сначала исправления
lifetime, затем capability matrix конкретного EXE/AAR; offload включать только
при подтверждённой пользе CPU/packet и tails.

<a id="F03"></a>

### F03 — размеры, PMTU и fragmentation

**Основание:** [QUIC-ACCOUNTING §3](../../../theory/performance/QUIC-ACCOUNTING-AND-LIMITS.md),
[REPORT §4](../../../theory/research/04-PERFORMANCE/REPORT.md), RQ-03/TR-03.

**CHANGE_REQUIRED, после scoped review:** текущий payload cap4096 не проверяет
всю арифметику. Перед `fragUDPMessage` нужны **C=B−headerSize >0**, checked
ceil division, **F≤255**, bound destination/header и reservation на peak copies.
Проверять первый и retry-size пути WriteTo/WritePacket. Assembly RX отдельно
проверяет суммарную длину, fragments/groups/bytes до новой allocation; один
Tx cap не защищает RX. Консервативный initial budget1197 нельзя просто увеличить:
size admission и packer оставляют разный space. Успешный enqueue допускает
поздний drop при уменьшении PMTU и не доказывает доставку.

Outer PMTU и inner TUN MTU — разные величины. При outer IPv6 MTU1280 и salt8
верхняя граница QUIC UDP payload1224 получается **до** QUIC/HY2 headers.
Это пример overhead accounting, не новый preset. Не снижать inner IPv6 MTU ниже
1280 ради обхода: поддерживать корректную fragmentation/PTB/stack semantics.
IPv4/IPv6/NAT64, actual DF, ICMP loss/spoof, PMTU decrease и handover — будущая
native матрица. Менять packet size только в исходном threat/useful-work контексте:
wire statistics и объём тоже меняются.

<a id="F04"></a>

### F04 — admission и состав памяти

**Основание:** [R28-C04 §3/4](../../../theory/research/cycles/2026-10-28-contracts/C04/REPORT.md),
[Z04 §2/4](../../../theory/research/cycles/2026-10-28-contracts/Z04/REPORT.md),
[REPORT-010 §3](../../../theory/research/04-PERFORMANCE/REPORT-010.md).

| Holder | Существующий guard | Чего он не ограничивает |
|---|---|---|
| serialized sendQueue | 8; ordinary Windows encoded sum≤11 616B при L≤1452 | In-service/builder/Salamander/kernel/pool, другие generations |
| DATAGRAM queue | send32 / receive128 | N concurrent copies **до Add**; RX handlers после dequeue |
| sent history | tracked guards40k/50k; PTO может удалить engine owner | Frame backing, sampler span/capacity и глобальную сумму |
| HY2 ready | 64 на UDP session | Количество sessions, partial fragments, assembly overlap, consumer-held data |
| defragger | uint16 IDs, fragment slots≤255, lazy age10s | Число live bytes, borrowers, detached groups; age не active reclaim timer |
| sampler ring | initial256, geometric growth, zeroing Pop/Clear | Высокую backing capacity после очистки; present count не PN span |
| framer control queue | QueueControlFrame проверяет maxControlFrames; PATH_RESPONSE имеет свой cap | Append добавляет некоторые blocked frames напрямую; один API guard16384 не global cap всех control holders |
| client offer | один зарегистрированный pending | Discarded-result cleanup и ещё retiring старые carriers |

**CHANGE_REQUIRED:** перед clone/spawn — bounded **count AND bytes** admission;
после dequeue — transfer, а не refund. Два limits32 и8 могут одновременно
выполняться, пока N ожидающих callers держат дополнительные copies и sent history
удерживает уже вынутые frames. Aliases одного backing считать один раз; fragment
views удерживают original allocation. Assembly требует временно и parts, и output.
Return в pool не подтверждает RSS release.

Для положительной начальной ring capacity C0 и максимального span S геометрическая
capacity≤max(C0,2S); при grow одновременно существуют old C и new2C.
Это параметрический storage bound, не число живых records и не RSS cap. Сам S
требует отдельного contract; произвольный cap не должен терять state retransmission.

Платформенные reservations уже частично реализованы, но они не ограничивают
native HY2 packet producers. Минимальный deliverable описан в §4.2. RQ-12/19,
TR-04/06, COM-12/AND-11 уже дают место для этого изменения.

<a id="F05"></a>

### F05 — cache eviction и borrower lifetime

**Основание:** [HB01-R04-06](../../../theory/research/reviews/HB01-R04-06.md),
[REPORT §10.5](../../../theory/research/04-PERFORMANCE/REPORT.md).

**ACCEPTED source/conditional; CHANGE_REQUIRED:** у клиента goroutine-per-message;
cache eviction может release/reset messages под cache.mu без item.access,
пока другой handler использует их после lookup. Это условный interleaving,
не воспроизведённый race/crash и не утверждение о таком же server dispatcher:
у него feed синхронный, но новый UDP session создаёт отдельную работу.

Первый выбор по цене — **один bounded receive owner на carrier** с прямой
обработкой вместо нового actor/executor на каждый packet, если весь feed/eviction/
close действительно сериализуется этим owner. Reassembly не должна блокироваться
на пользовательском read/ready queue. Если другой concurrent borrower остаётся,
нужны explicit acquire/detach/reclaim и credits до actual last borrower return.
Выбрать один вариант по полной caller матрице, не внедрять оба.

Один `WithSize`, janitor либо mutex в eviction callback недостаточен: callback
исполняется под cache.mu, обратный cache.Delete под item.access создаст lock-order
риск. Expiry закрывает admission/видимость, но не разрешает recycle live borrower.

<a id="F06"></a>

### F06 — отмена сессии, carrier close и generations

**Основание:** [R28-C04 §5/6](../../../theory/research/cycles/2026-10-28-contracts/C04/REPORT.md),
[REPORT-010 §5](../../../theory/research/04-PERFORMANCE/REPORT-010.md), TR-04/06.

**CHANGE_REQUIRED, source-scoped:** `udpPacketConn.Write*` проверяет ctx только
на входе; `SendDatagram` делает copy и ждёт `datagramQueue.Add`, где нет session
ctx/deadline. Session.Close отменяет session/delete, но не будит этот Add;
SetWriteDeadline возвращает ErrInvalid. Отмена caller, request timeout и фактический
write return различаются. Нельзя лечить это goroutine на каждый timeout.

У whole carrier другая цепочка: QUIC graceful close ждёт sendQueue drain;
raw socket закрывается в HY2 после QUIC close. При задержанном raw Write это
условное незавершённое ожидание, **не установленный runtime hang**. В hopping
WriteTo держит access mutex через raw Write; Close/deadline требуют тот же mutex.
Простая перестановка Close выше не доказывает independent abort.

**REUSE:** Windows Terminate/Wait/Empty и Android actual private-process death
дают более сильную границу, чем локальный connDone. Не снимать reservation и не
пускать successor по pending.done/active=false/close reply/elapsed timeout.
Минимальные два независимых deliverables — §4.1; полное исправление core не
отменяет платформенную death/reap приёмку.

<a id="F07"></a>

### F07 — congestion callback semantics

**Основание:** [2H-HB02-R04-10](../../../theory/research/reviews/2H-HB02-R04-10.md),
[REPORT §12…15](../../../theory/research/04-PERFORMANCE/REPORT.md),
[Z04 §3](../../../theory/research/cycles/2026-10-28-contracts/Z04/REPORT.md).

**CHANGE_REQUIRED / COMPATIBILITY_REVIEW:** current Brutal не реализует полный
public Ex method set (time.Time вместо monotime и отсутствующие methods), поэтому
normal dispatch выбирает base stubs; ackRate остаётся1. Достижимость — negotiated
positive rate (`!RxAuto && actualTx>0`) в соответствующем направлении. Это не
поломка всей QUIC recovery/RTT/payload и не доказательство, что данный режим включён.

Текущий engine передаёт post-flight F+L; meta2 sampler прибавляет L и сохраняет
F+2L. Это semantic mismatch, не двойная отправка wire bytes. PMTU probe loss
пропускается individual congestion guard, но попадает в Ex lost list; cleanup
flight/history и реакция на congestion должны быть раздельны. Удалить probe event
целиком нельзя: можно потерять lifecycle cleanup. NTP TimeFunc в исследованной
ветви достигает unused constructor argument; менять NTP/TLS/trust из-за этой
опровергнутой гипотезы не требуется.

**Готовое ADAPT:** quic-go **f7b7e9c… / mod.6** + sing-quic **4ab2ece…** как
согласованная semantic пара: prior-flight для injected CC, connection-wide PN,
install fence, exact least-unacked, neuter/drop/migration, full Ex/monotime Brutal.
Это не drop-in upgrade core1.13.19: меняются sing0.9 beta4, quic/utls/x/*,
constructors/callers/native closure. Пара **не закрывает автоматически** PTO
QueueProbePacket terminal notification, PMTU Ex classification, pacer arithmetic,
sampler high-water или общий resource cap. Candidate DATAGRAM Add также contextless.

Выбор реализации: одна согласованная patch series с таблицей equivalent changes
на поддерживаемом whole-core bundle; готовую пару использовать как reference.
Полную смену dependencies рассматривать только если её closure дешевле и проходит
те же gates. Не редактировать module cache и не совмещать это с новым transport.
Узкий Brutal backport допустим как отдельный **частичный** результат по dispatch,
если rate branch нужен; он не закрывает весь F07. Глобальное `-L`, один rename
signature или no-op neuter у stateful BBR — REJECT.

<a id="F08"></a>

### F08 — арифметика pacer

**Основание:** REPORT §12.5, R04-007; existing quic-go MIT internal pacer.

**ADAPT:** B×Δ_ns вычисляется до деления; проверка только negative результата
не ловит положительный/нулевой wrap. Сначала positive domain, pre-multiply bound,
checked addition, division/round-up и burst saturation. Переиспользовать guard
pattern `internal/congestion/pacer.go`, сохраняя policy конкретного controller;
не копировать его bandwidth×5/4 как «исправление BBR». Нулевые/краевые rate,
late/reordered events, MTU/burst multiplication — отдельные cases. Частота и
влияние на скорость не установлены; новый math framework не нужен.

<a id="F09"></a>

### F09 — обслуживание общего carrier

**Основание:** REPORT §10, §13.3; RQ-04/06/12.

DATAGRAM-first в packetPacker и остаточный STREAM gate128 допускают нулевые
ordinary STREAM visits при устойчивых полных DATAGRAM1200/M1280. Это не
универсальный freeze: tails, паузы, retransmission modes и иной PMTU меняют trace.
RR внутри STREAM не доказывает положительную долю, если до него не дошли.

**ALREADY_PLANNED / ADAPT после F04–06:** адресный bounded opportunity contract
в существующем packer/framer; при необходимости локальный RR/DRR deficit по
wire-byte cost, с верхней границей state. Не гарантировать миллисекунды без
положительной service curve/arrival envelope. Формулы delay≤T+σ/R и backlog≤σ+ρT
требуют доказанных предпосылок, которых для текущего Internet/Go/Android нет.
Linux FQ-CoDel/qdisc на opaque UDP carrier не планирует его inner streams;
гостевая ОС не управляет очередью hypervisor. Несколько carriers/mux/pools имеют
цену handshake, памяти, fairness и failure fanout; это поздний one-factor выбор.

<a id="F10"></a>

### F10 — clocks, keepalive и Android power

**Основание:** [REPORT §11](../../../theory/research/04-PERFORMANCE/REPORT.md),
[REPORT-011](../../../theory/research/04-PERFORMANCE/REPORT-011.md),
[Z05](../../../theory/research/cycles/2026-10-28-contracts/Z05/REPORT.md).

**REUSE / ADAPT наблюдений:** nominal HY2 K10s — вход receive/PTO timer;
source due использует lastRx и max(min(K,idle/2),1.5PTO), плюс ping state.
Due → executed → queued → socket return → wire → peer/NAT refresh → useful work
не взаимозаменяемы. Effective idle=max(negotiated idle,3PTO), поэтому30s не
безусловный lifetime. gVisor local TCP15/15 и наружный TCP60/20 имеют иных peers.

Android elapsedRealtime authority включает suspend и boot-local; выбранный
Go/Linux reference monotime исключает suspend. Это explicit reference, не
компилятор shipped AAR. FGS/systemExempted даёт определённую eligibility/network
классификацию, но не создаёт wake lock и не гарантирует CPU execution/LPS exemption.
SFA broadcast→pause/wake→shared PauseManager достигает opt-in URL-test/NTP/WG
consumers; edge к pinned HY2 timer не найден. Наличие manager в context не
подписывает все transports.

Минимальное действие платформы: invalid authority/blocked/lost/changed owned path
→ existing fail-closed retirement; same context + valid authority + no typed failure
→ условно сохранить carrier; actual carrier failure → existing bounded close/re-offer.
`active()` не peer/NAT receipt и не Ready. Не reconnect на каждом wake.
При resume invalid authority доминирует над native timer; строгий zero post-expiry
packet barrier не доказан Java gate/watchdog, поскольку native writes не проходят
его на каждом пакете. Reboot не восстанавливает прежний elapsed grant.

Optional jobs можно объединять только внутри совместимых release/deadline,
authority/owner/clock windows и budgets. Это не energy optimum. Постоянный wake
lock, extra ping, auto battery exemption, WorkManager heartbeat10s и IPsec NAT-T
SocketKeepalive для QUIC не принимаются как готовая оптимизация. Device/OEM/RAT,
radio/NAT/reconnect energy неизвестны. COM-12/RQ-16/AND-11 уже содержат эту приёмку.

<a id="F11"></a>

### F11 — миграция пути

**Основание:** TRANSPORT §14.2, archive §14.2, TR-02.

**ALREADY_PLANNED / DEFER интеграцию:** библиотечные AddPath→Probe→Switch есть,
owned public HY2/native seam отсутствует. Switch return означает запрос;
actual switch применяется позже. Current MigratedPath сбрасывает RTT/MTU/loss
и заменяет injected controller на NewCubicSender(useReno=true). Server
DisablePathManager/direct remote-address branch — другой путь.

Сначала новый принадлежащий session socket, Android protect+bind **до connect**,
тот же admitted gateway/handshake/peer capability, validation и applied receipt,
loser cleanup, новая PMTU/CC policy и preserved retirement. Не rebind live fd.
Пока gate не выполнен — использовать контролируемый restart существующего owner;
не обещать seamless handover и не создавать параллельный engine.

<a id="F12"></a>

### F12 — платформенная обвязка и storage

**REUSE / ALREADY_SOURCE_IMPLEMENTED:** Windows bounded file slots сохраняют
original owner при acquisition/publication/Close uncertainty. Check может ждать
I/O без hard deadline. Android отдельные lanes/cleanup owners, trim initialized
report store, source cache/history flags, Observer.Close и startup-error retention
уже реализованы в source. Не размножать executors/reapers после timeout и не
отменять эти guards ради low latency.

**Дельта в COM-12/RQ-19/AND-11:** one process-wide inventory GUI+worker /
JVM+native+Go+kernel+fds+files, producer concurrency/bytes/retiring counts и bounded
diagnostic history. Измерители не читают пользовательский traffic/config/raw logs.
File size bound не ограничивает все ожидающие caller buffers; GOMEMLIMIT/GCPercent
не hard RSS cap, logMaxLines64 не byte/disk quota. iOS45MiB branch libbox не
Android limit. OS Job memory limit — optional last-resort guard после budget/
failure-mode проверки, не произвольное128MiB для всего VPN. Kill должен сохранять
защиту и перейти в owner-controlled failure, а не бесконечный restart.
AND-01-S4 уже добавил lexical cleanup AAR/JAR stages; это не pair transaction.
ZIP entry-count проверяется после открытия ZipFile и не доказывает heap admission
самого parser. Не превращать artifact bounds в whole-build memory guarantee.

<a id="F13"></a>

### F13 — маленький VPS и серверный admission

**ALREADY_PLANNED / DEFER deployment:** COM-12/15, TH-07, RQ-12/19 и AND-13/15.
Текущий VPS не оплачен; его CPU/RAM/kernel/quota/core version/запущенный сервис
не известны. Один VPS остаётся одним failure domain. Предлагать distributed
control plane или обещать достаточность RAM по старой константе нельзя.

У quic-go есть raw1024/accept32, unprocessed/handshake/retry/anti-amplification
и tracked guards. Но accept32 — очередь ещё не принятых соединений, не cap active
connections; 3× anti-amplification — wire limit, не CPU/TLS/map budget. В pinned
HY2 Service MaxIncomingStreams=1<<60, нет wiring GetConfigForClient/ConnContext;
создаются goroutines per accepted carrier и new UDP session. Это local source gap,
не доказательство deployed DoS. Нужны отдельные pre-handshake/connection/session/
per-user attempt limits, auth/masquerade compatibility и recovery reserve.
Архивный «admission нет нигде» неверен. Начать с configured limits и OS accounting
на будущем сервере; не менять client path и не включать лимиты без версии/rollback.

<a id="F14"></a>

### F14 — что заимствовать позже

**REUSE сейчас:** existing routes/dialers/Happy Eyeballs, HTTP/TLS, quic-go queues/
timers, bounded worker owners и имеющиеся x/sync/channel primitives; second mux,
DNS cache, TCP/QUIC/crypto engine и shared policy layer не нужны. Nonblocking
TryAcquire до spawn предпочтительнее semaphore waiters, число которых не ограничено.
TUF2.4.2 уже declared/source integrated в Windows trust/update направлении;
не внедрять второй updater и не объявлять TUF compliance по сходству design.

**DEFER:** NaiveProxy/XHTTP/Tor/UPGen/FEC/L4S/new packet engine — нет нужного
сценария/совместимости/измеренной выгоды для текущего resource slice. Sing-box1.13.19
не получает XHTTP от смены названия поля. DRR/FQ-CoDel — применимы в своих
scheduler/bottleneck слоях, не готовая универсальная latency гарантия. Toxiproxy
TCP не моделирует UDP censor; iperf3 не доказывает bypass и Windows не его
официальная target platform. Новый survey для этих решений не требуется.
Старые NaiveProxy release references — источник принципов preamble/carrier
lifetime, не approved binary. Новые HY2 modes из mutable docs не приписывать
нашему pin; Salamander и обычное HTTP/3 cover имеют разные wire/compatibility свойства.

<a id="F15"></a>

### F15 — исправленные и непринятые выводы

| Старая формулировка | Решение для разработки |
|---|---|
| Архив до V2-109: UDP не прошёл, TCP должен всегда быть baseline | **REJECT устаревший вывод**: поздний архив сохраняет HY2 live09-09; ни он, ни старый timeout не устанавливают текущую RF policy/состояние VPS |
| R04-008: receive20MB ограничивает весь sender RAM | **REJECT**: локальное receive window20MiB не peer send credit, DATAGRAM и все holders |
| PTO ничего не удаляет, lifetime≤30s,60s clamp доказывает no-spin | **REJECT**: QueueProbePacket→DeclareLost; idle floor3PTO; clamp не minimum service/время жизни. Sampler cleanup отдельный |
| Singleflight⇒≤1 live/retiring generation | **REJECT**: pending.done/connDone раньше полного cleanup. Сохранить реальный Windows/Android barrier |
| Нет history/admission guards | **REJECT**: guards40k/50k и нижние очереди существуют; global composition всё ещё не доказана |
| R04-009 «CCA-трилемма choose2of3» и auto=bbr2 как доказанное решение | **NEEDS_REVIEW / не использовать**: cited paper был link/abstract-only; auto path читает meta2 bbr_sender.go, не основание назвать BBRv2 или вывести default/fairness/detectability theorem |
| Новый sender/controller pair полностью исправляет memory, PTO, PMTU и скорость | **REJECT blanket claim**; принять только перечисленные source semantics после review, residual gates F04/F06–08 сохраняются |

Старые источники/отчёты остаются provenance. Их errors не стирают полезный вопрос
о resource accounting, но ошибочные формулы не переносятся в требования.

## 3. Порядок изменений по существующим компонентам

Обозначения P0/P1 ниже — предложения приоритета, не изменение нормативного PLAN.
P0 lifecycle/trust/OS gates владельцев сохраняются впереди speed tuning.

| Шаг / готовый deliverable | Приоритет и причина | Компоненты / существующие IDs / owner | Зависимости и граница завершения |
|---|---|---|---|
| **D0. Зафиксировать поддерживаемый source bundle и contract matrix** | P0 допуска native change: не потерять уже реализованные guards | Windows/core/build: go.mod, package/worker; Android/native: source-lock, existing overlay/artifact tasks; TH-01, AND-01/02 | Input pins/tags/API/licences, callback/close/cancel matrix; source-only deliverable независим от VPS. Bundle не promoted до S/N gates §6 |
| **D1. Отмена UDP session → Add actual return** | P0 корректности bounded work; отдельная сессия не должна удерживать бесконечное множество ожиданий | sing-quic packet Write*, quic-go SendDatagram/datagramQueue; TR-04, RQ-12/19, AND-02/11; core owner + Android bundle consumer | Frozen contract §4.1-A; legacy API preserved, race/close/backing receipts. Не зависит от CC tuning/migration/paid VPS |
| **D2. Carrier abort и retiring accounting** | P0 lifetime: graceful drain не completion proof | HY2 Client/completeOffer/close, sendQueue, optional Hop; существующий platform worker/reap; TR-06, TH-01, AND-02/03/11 | §4.1-B; independent abort без mutex за blocked write; no successor до actual required receipts. Не вырезать process barriers |
| **D3. Bounded RX owner + безопасная reassembly** | P0 source ownership и P1 ресурсов | client_packet, packet/defragger, server session admission; RQ-03/12/19, COM-12, AND-11 | F05 + §4.2; сначала единый serial owner либо explicit borrowers, затем count/bytes/group caps. Serial processing без blocking consumer и nonfatal overload |
| **D4. Checked sizes + total producer admission** | P1; cap4096/queue32 не ограничивают pre-copy и assembly peak | HY2 fragment/retry/encode, QUIC pre-Add/RX copy, platform budget adapter; RQ-03/19/TR-04 | §4.2/4.3; resource outcome отдельно от protocol parse fatal; caps before allocation/spawn и through history/retiring. Одна HY2-правка не закрывает lower QUIC copy |
| **D5. Semantic CC patch series + pacer guards** | P1 корректности выбранных branches; P2 speed policy | sender/adapter/meta2/Brutal/pacer, source bundle build owners; RQ-05/TH-06/AND-07/11 | D0; matching full matrix, residual PTO/PMTU explicit. Пара как source reuse, narrow backport не whole fix; rate defaults не меняются по одной type assertion |
| **D6. Минимальные наблюдения и integration budgets** | P1 измеримости; часть D0–D5 можно сделать source-only до producers | existing ProbeObservation/diagnostic pool; Android measurement request/native adapter; adaptive API1; TH-02/04/06, AND-07/11, ADP-005 | §4.4; accepted measurement authority/endpoint/owner. Budget rejection не FAIL канала. Не дублировать M01…06 и policy |
| **D7. Bounded STREAM opportunity и one-factor tuning** | P2 после ownership; устранить доказанный conditional starvation case без роста работы | packetPacker/framer; RQ-04/06, COM-09/12 | D3–D6; §6 V8, useful call+bulk и burst loss; затем один mux/window/CC/offload factor. Нет числового latency promise |
| **D8. Android power/handover и VPS эксплуатация** | P1 reliability, P2 energy optimization | existing physical/authority/watchdog paths; TR-02/03/05, RQ-16, AND-05/11/15, TH-07 | D0/D2/D6, accepted artifacts/devices/paid infra. Observe/classify сначала; migration/keepalive changes только по соответствующим gates |

**Можно вести независимо:** D1 source contract, D3 ownership и D5 compatibility
matrix при неизменных source pins; Android platform resource inventory и Windows
file/worker owner inventory не требуют общего portable resource manager. Контракты
§4 должны совпасть до объединения patches. Не запускать одновременно несколько
изменений scheduler/CC/MTU/pool и затем приписывать результат одному.

## 4. Минимальные контракты до реализации

### 4.1. Session cancellation и carrier retirement — два deliverables

**A. Session write admission/cancellation**

Вход: original session generation/context; current carrier owner; payload/header
bound; count+byte permit; queue admission deadline в явно названном clock domain.

1. Существующий public `SendDatagram([]byte)` сохранить. Внутренний context-aware
   seam должен проводить session cancellation до ожидания Add; legacy callers
   получают прежний connection-lifetime режим, без background wait goroutine.
2. До copy/spawn — nonblocking count/bytes reservation. При full queue wait
   выбирает queue progress, **session cancel**, carrier close и write deadline.
   Перед enqueue повторно проверяются authority/closed/cancel под owner protocol.
3. Enqueue — точная граница transfer. До неё charge остаётся у caller; rejected/
   cancelled outcome освобождает его copies. После неё charge принадлежит queue/
   history; это не обещание peer delivery или отзыва уже переданного в kernel.
   Конкурирующие enqueue/cancel получают один outcome по явному admission protocol.
   Owned session Close seal и приём новой записи должны иметь общий fence;
   context check сам по себе не atomic cutoff относительно syscall/чужих часов.
4. Session.Close будит ожидающие session writes и запрещает новые. Deadline
   поддерживается session-local либо явно отклоняется; нельзя менять deadline
   shared raw socket ради одной UDP session. Соседние sessions сохраняются.
5. Terminal result и reservation release публикуются после actual write-call return
   и передачи/освобождения каждого owned holder. Timeout result не подменяет join.

Выходы: admitted/transferred, not-started-budget, cancelled-before-admission,
deadline-before-admission, carrier-closed, protocol-error; каждый с owner/generation
и actual return. Доставка DATAGRAM может остаться unknown после admitted.

**B. Whole-carrier abort/retirement**

Root owner сначала seal new offers/writes; отдельно запросить graceful close и
обеспечить независимый abort оригинального socket/write. Abort не должен ждать
sendQueue drain либо Hop access mutex, который удерживает блокирующий writer.
Для hopping pin original socket generation, запретить новые borrowers, закрывать
оригинальные sockets вне metadata mutex и удерживать writer/backing receipts до
возврата; не переиспользовать FD/name/буфер по одному `done`.

Resource token active→retiring остаётся занятым через discarded offer cleanup,
QUIC drain, raw write и raw close. `pending.done` означает только offer result.
Windows successor дополнительно требует retained original Job/Wait/Empty и I/O
completion; Android — original Binder death **и обе actual RPC returns**, source/
Stub idle. Ошибка abort/reap оставляет BLOCKED/uncertain без direct fallback.
Source contract не обещает wall-clock upper bound работы ОС.

### 4.2. Producer count ДО allocation/spawn

Минимальная схема — **один permit на реально удерживаемую работу**, не новый
background dispatcher. Nonblocking `TryReserve(count, chargedBytes, scope)`
проверяет positive engineering ceilings и checked arithmetic. Не создавать
goroutine/task/queue node либо payload clone до решения. Reject не ставит работу
в неограниченный semaphore wait-list; attempts/bytes rate budget не refund при
ошибке, retained-capacity credit возвращается только после реального release.
Destination должен иметь bounded typed representation до materialization headers.
Произвольный `net.Addr.String()` — внешний вызов; проверка длины после его возврата
не ограничивает его собственные allocations/время. Legacy type compatibility и
время удержания caller-owned input указываются отдельно, без hard elapsed обещания.

| Точка | Что обязано произойти до новой работы | Куда переходит charge |
|---|---|---|
| HY2 Write*/fragment | Bound header/C/F и peak backing, один producer count | packed staging → QUIC copy → queue/history; original alias lifetime отдельно |
| QUIC SendDatagram | Admission **до make/copy**, включая уже waiting producers | Add→Pop→history; socket serialized buffer имеет отдельный owner |
| QUIC RX | Copy reserve до make при cap, либо без-allocation drop | RX queue→HY2 receive owner; lower-layer charge не исчезает при dequeue |
| HY2 client receive | Count+len reserve до handler spawn; предпочтительно fixed serial owner | decode→defrag/assembly→ready→consumer/retiring |
| Новый carrier/session | Global и per-owner count до map/handler allocation | Active→retiring до full completion; pending/discarded также считаются |
| Defrag assembly | Group/parts/checked total и свободный output+ready budget | Старые fragments и новый output одновременно до завершения copy |

Консервативно reserve peak на операцию и удерживать до terminal — дешевле сложной
динамической передачи credits; допустимо лишь если покрыты downstream retained
holders. Когда downstream переживает write return, charge передаётся его owner,
а не освобождается. Count и bytes ограничиваются одновременно, плюс metadata/
group/session/generation и pool high-water caps. Нельзя замкнуть формулу только
на payload bytes и объявить RSS cap. Значения ceilings должны попасть в versioned
bundle/config manifest с units/scope; workload SLO задаётся отдельно.

Overload-drop — штатный nonfatal outcome с bounded counters; не возвращать его
в текущую ветвь `handleMessage error → close entire carrier`. Malformed protocol,
authority failure и resource rejection имеют разные решения. Защита/Stop/revoke/
recovery reserve не расходуется на optional work; чужой budget не даёт permission.

### 4.3. Размер и путь

`PacketBudget = {outer family, validated path generation, actual wrapper overhead,
QUIC frame budget, header bound, maxFragments, maxPayload, source of size}`.
Каждое изменение MTU/path инвалидирует старый budget. C≤0/F>255/overflow/unsupported
family → typed rejection без mutation/allocation loop. Не повышать1197 до
совпадения admission/packer semantics; не снижать security или crypto envelope.

### 4.4. Measurement и adaptive consumer

Расширять existing immutable observations: operation ID, owner/session/network/
config/endpoint/family/clock identity, started/ended/received отдельно, useful
outcome и bytes scope, local budget outcome, retained reservations и unknown fields.
Пользовательский payload/raw URI/profile не передаётся. Native measurement через
тот же admitted outbound без SOCKS→direct fallback и ambient proxy/redirect.
Current libbox TrySocks5/URLTest/HEAD не готовый protected measurement producer.

Для `PlanAutomaticBackground` подавать actual coherent protected-reservation
snapshot, не UI Running и не guessed count0. False/unknown proposal не разрешает
admission; финальный atomic recheck у platform owner. Не переводить Windows
time.Time в Android elapsed произвольным offset и не renew authority/result TTL.

## 5. Готовый код, совместимость и стоимость

| Reuse | Решение и практическая цена |
|---|---|
| Existing quic-go queues, timers, PMTU reason/flight accounting, MIT pacer guards | **REUSE/ADAPT** narrow seams. Сохранить frame/ACK/loss ownership и normal recovery; source context cancellation потребует согласованного API patch |
| SagerNet f7b7e9c +4ab2ece | **ADAPT coupled semantics**, MIT + GPL-3.0-or-later. Go≥1.25/sing0.9 beta4/utls/x/* и constructors шире текущего closure. Build/API/SBOM/corresponding-source/rollback whole EXE/AAR — существенная цена; не «обновить два файла» |
| Original Hysteria core2.13.0 reference | MIT; opt-out loss compensation/cwnd floor — reference policy. Apernet Ex API отличается; это не существующий JSON option Snowden и не второй embedded core |
| sing cache/pause, sing-quic/box | GPL-3.0-or-later; existing boundary reuse. Lock order, synchronous pause callback под mutex и borrower completion важнее наличия helper |
| x/sync semaphore/errgroup v0.20.0 либо channel | Existing primitive, без dependency install; сохранённый Go Authors LICENSE — BSD-3-Clause. Не считать SetLimit/context cancellation автоматическим cap всех callers и native resources |
| SFA idle receiver | **REFERENCE/ADAPT** caller shape; GPL-3.0-or-later плюс ограничение имени/ассоциации. Не переносить whole service и unlimited channel; не даёт HY2 pause edge |
| Existing platform owners/filetxn/Android cleanup | **REUSE** без нового portable executor или resource DB. Их timeout/depth/count ceilings не являются latency, disk/global memory или durability proof |

Наиболее дешёвый достаточный старт — D1 и D3: небольшой cancellation seam и
сериализованный bounded RX owner на существующих ядрах. D2 необходим независимо
для shutdown. D5 оправдан как correctness compatibility series; обещание скорости
появляется только после comparative gate. D7/D8 не блокируют первые source fixes.

## 6. Конечная будущая приёмка — всё NOT_RUN

Source review устанавливает caller/contract; unit/controlled test — выбранный
кодовый trace; native — конкретный артефакт/ABI/OS boundary; device/live — реальный
path/power/VPS; comparative — превосходство в зарегистрированной нагрузке.
Ни один нижний уровень сам по себе не доказывает следующий.

| Case | Слой и обязательный результат | Не подменять |
|---|---|---|
| **V1 pins/compatibility** | Whole EXE/AAR digest/compiler/tags, approved overlay/API/16KiB JNI/ABI и licence closure; current unchanged baseline | go.mod, `version()==1.13.19`, stock AAR или compile assertion не whole-artifact proof |
| **V2 cancel** | Full queue32 + N callers; cancel before copy/while Add/before enqueue/after transfer; session Close/deadline/carrier Close; exact actual returns и neighbour sessions | Нельзя PASS по timeout wrapper или одной пустой очереди |
| **V3 abort/replacement** | Blocked raw write; no-hop/hop; raw Close/deadline/error; graceful drain; discarded offer; new generation запрещена до required release/Job/Binder+RPC/source receipts | done/active=false, смерть только child или close reply не полный retirement |
| **V4 ownership** | Pause feed borrower + expiry/close/size eviction; duplicate/out-of-order/total change; no double reset/recycle; lock order; client/server отдельно | Serial hypothesis без full caller proof; visible map count не total retained bytes |
| **V5 budgets/sizes** | C=0/negative, F255/256, long destination, retry-size, max valid payload; simultaneous RX assembly/output; reject до clone/spawn; count+byte totals и nonfatal drops | Tx4096, queue counts и released pool не RSS/global cap |
| **V6 CC terminal matrix** | Engine→adapter→controller, positive/auto up/down; F0/F>0, ACK-only/STREAM/DATAGRAM/PMTU; PN spaces/install/drop/Retry/PTO/migration/replacement/close; sampler present/span/capacity | Direct batch call, one signature fix, loss filtering без retirement |
| **V7 arithmetic/timestamps** | Zero/max rate, negative/late/reordered deltas, positive/zero wrap, saturation/round-up/MTU burst; registration/queue/socket/user receive/reaction separated | NTP hypothesis, monotonic type или negative-only overflow guard |
| **V8 mixed workload** | Registered interactive+bulk, DATAGRAM1200/M1280 и tails/другие PMTU, STREAM opportunity bound, same useful goodput, failures/tails/fairness/CPU/RSS | Queue bound/DRR имя не ms SLO; рост Mbps не оправдывает ухудшение звонков |
| **V9 path/PMTU** | IPv4/v6/NAT64, actual DF, salt8, ICMP drop/spoof, MTU decrease; protect/bind-before-connect; Probe/queued Switch/applied switch/CC policy/loser cleanup | Flag, library AddPath и new socket не seamless migration |
| **V10 Android power** | Accepted AAR/OEM/API, awake/Doze/AppStandby/LPS/exemption/suspend/resume/reboot; authority expiry vs native writes, same-path keep vs typed failure recovery | FGS/network allowance, nominal10s, PauseManager или local TCP ACK не wire/NAT/battery receipt |
| **V11 platform/storage pressure** | GUI+worker/JVM+native+Go/kernel/fds/disk, uncertain file Close, retained cleanup/cold scan, Stop/revoke under CPU/I/O/memory pressure | Job process cap/128MiB helper, GC target, log64 или FIFO8 не system bound |
| **V12 future VPS/field** | Оплаченный versioned server, pre/post-auth/session admission, CPU steal/traffic/quota/restore/recovery, paired ISP/time/workload и failures | Локальная source model, ToxiproxyTCP, iperf или старый September live не RF bypass/health proof |

До запуска сравнений определить primary metric, minimum meaningful effect,
допуски safety/tails/fairness/resources, workload/condition matrix, failed/missing
outcome rules и stop criteria. Сейчас эти числа неизвестны. Вред safety,
неподтверждённый cleanup, рост нагрузки/потерь и перегрузка стенда блокируют promotion.
Текущий plan-only не разрешает запуск даже unit/build cases этой таблицы.

## 7. Сохранённые источники и разрешение противоречий

| Группа | Точный source / corpus | Использование |
|---|---|---|
| Путь/размер/overhead | TRANSPORT-AND-RESOURCES; R04 REPORT §3…10; REPORT-010; current module captures | F01–06/F09/F11; ordinary Windows caller проверен отдельно от vector expression |
| Holder/lifetime | R28-C04 REPORT §3…6; Z04 REPORT §2/4; current datagram_queue/send_queue/packet/client/hop/cache | F04–06; positive local guards сохраняются; global bound только при явных admissions/actual receipts |
| Independent reviews | HB01-R04-06; 2H-HB02-R04-10 | Scoped acceptance lifetime interleaving/normal method dispatch, не whole package/runtime |
| CC correction | R04 REPORT §12…15; R04-007; Z04 source pair exact f7b7e9c/4ab2ece; QUIC-ACCOUNTING | F07–08; old standalone prior/post fix не complete remediation; PTO/PMTU остатки обязательны |
| Clock/power | R04 REPORT §11; REPORT-011/SOURCES-011; R28-Z05; saved Android/AOSP16 and Go references | F10; source/spec/reference разделены с shipped/device proof |
| Архивы | before-split и pre-V2-109, R04-008/009/report/source/status | F15; исторические условия сохранены, ошибочные implications отвергнуты |
| Existing plans | PLAN FR-012/TH-02/04/06/COM-12/RQ-01…06/12/16/19/TR-01…06; ANDROID-PLAN AND-01/02/05/07/11/15/16; adaptive ADP-004/005/008 | Новые deliverables уточняют backlog, не дублируют requirement IDs и не закрывают чужие статусы |

Разногласие «narrow Brutal fix уже достаточен» разрешается по scope: достаточен
лишь для source dispatch, недостаточен для full callback/lifecycle/resource
semantics. Разногласие «новая пара всё исправила» разрешается residual PTO/PMTU/
contextless Add gates. Из позднего Z05 берётся доказанный opt-in consumer graph;
его модель не даёт atomic packet barrier/energy outcome.

## 8. Остаток, поздние входы и завершение среза

Прочитан весь назначенный frozen corpus; unit-level решения и provenance — в
coverage. Неинформативная администрация отмечена отдельно с причиной; mixed
sections сохраняют substantive family. Исторические exact-body duplicate
кандидаты не считались автоматически одинаковыми доказательствами.

Current source review: bounded read/write slots и diagnostic admission целиком;
worker retire/Stop/Job/caller sections; Android RPC/callback/native-session/cache/
history/cleanup/physical/socket seams и platform retirement sections; shared
adaptive budget/observer types; current и saved candidate transport symbols.
Остальные capture files — inventory, не автоматически полный code review.

Поздняя сверка: [final-drift](../../../../.artifacts/planning/knowledge-application/2026-10-28/captures/C04/final-drift.json),
[read scopes](../../../../.artifacts/planning/knowledge-application/2026-10-28/captures/C04/late-review.json).
В первом closing cut изменились девять selected project/doc files; новых файлов
в отслеживаемых code seams не было, selected module sources не изменились.
Пять Android callback/TUN/value-construction diffs прочитаны: generic callbackStep,
Go-backed constructors/getters/setters теперь проходят existing pre/post gate,
late successful TUN сохраняет owner до postcheck. Это не новый cap/packet barrier
или проверенный JNI runtime. AND-01-S4 artifact cleanup также прочитан по task/ADR.

Windows STATE/PLAN дошли до **V2-182**: coherent import/recovery/preview source
guards. Прочитано документационное уточнение; новая import/trust implementation
вне performance seams **не аудирована этим срезом**, руководителю/PC owner нужно
использовать её актуальные receipts. RQ/TR/TH-06 и transport module inputs в
прочитанной области не изменены. Активные владельцы не останавливались; closing
cut последовательный, последующие правки не покрываются автоматически.

**Следующий конкретный срез разработки:** D0 contract/bundle gate, затем D1
session cancellation до Add actual return; независимо D3 safe bounded RX owner.
Не начинать speed presets/offload/migration/новый transport до этих границ.
Новые web-исследования, product patches, тесты, сборка, native/device/live и
доступ к VPS в этом application-срезе не выполнялись.

