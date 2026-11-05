# R04-010 — Windows/HY2 UDP: фактический путь и границы retained ресурсов

Срез источников: 27 октября 2026.
Статус: **READY_FOR_REVIEW**, не ACCEPTED. Источники: [SOURCES-010](SOURCES-010.md);
handoff: [R04-010](handoffs/R04-010.md); evidence:
`.artifacts/research/theory-04/2026-10-27-win-udp-gso-res4/`.

## 1. Вопрос, baseline и область

Вопрос: проследить quic-go → PacketConn → Salamander → Windows/Go UDP,
установить условия fast path и перепроверить прежние RAM/idle/singleflight
утверждения. Baseline — R04-005/007 и непринятые продолжения R04-008/009. Последние не
используются как аксиомы: ниже разобраны конкретные ошибочные предпосылки.

Рабочее дерево содержит исходники Windows и публичный локальный module cache;
это **не SOURCE_NOT_AVAILABLE**. Однако cache-version, локальные SHA-256 и
строки go.sum не являются независимой аттестацией установленного EXE.
Manifest закрепляет sing-box 1.13.19, quic-go 0.59.0-sing-box-mod.4,
sing-quic 0.6.4-0.20260803041914-d83826c306d7, sing 0.8.13, x/sys 0.44.0.
Windows packaging source выбирает windows/amd64 и with_quic/with_utls/
with_gvisor/with_clash_api. Локальная Go 1.26.7 — **source reference**, не
доказательство компилятора поставленного приложения.

Область главного trace: HY2 outbound, прямой разрешённый dialer, Salamander,
без port hopping; отдельная строка ниже показывает отличие hopping. Мы не
читаем реальный runtime config, vault, пользовательские логи или VPS. Наличие
optional descriptor-поля не означает, что оно активно у владельца.

## 2. R04-010-A — фактический scalar Windows путь, не GSO

### 2.1 Источник конфигурации и процесс исполнения

`windows/backend/render/render.go:775–817` формирует hysteria2 outbound с
Salamander и TLS; ServerPorts/HopInterval и Up/DownMbps копируются, если заданы
в дескрипторе. В этом renderer нет явного Windows-USO/URO параметра.

`windows/corebridge.go:465–504` останавливает прежний manager до нового
`NewIsolatedEngine`; ошибка старого cleanup запрещает старт замены.
`worker_child.go:61–68` создаёт `NewEngine`, а `engine.go:410` — `box.New`.
Таким образом, пакетный путь sing-box находится в worker-процессе, не в GUI.
Это source-композиция, не доказанное native исполнение.

`sing-box/protocol/hysteria2/outbound.go:63–82` → `dialer.New` и
`hysteria2.NewClient`. Без Detour создаётся DefaultDialer; имя сервера может
добавить resolve-wrapper до адресного dial. Для UDP DefaultDialer вызывает
`net.Dialer.DialContext`, а connection manager может вернуть `trackedConn`.
Обе ветви сохраняют поведение Read/Write underlying connection; внешние
интерфейсы у wrapper не наследуют все методы concrete UDPConn.

### 2.2 Матрица участка и точного API

| Участок | Фактический caller/source | Что сохраняется и чего нет |
|---|---|---|
| Сокет → net.PacketConn | HY2 `client.go:202–210`: connected UDP → `NewUnbindPacketConn` → `NewSalamanderConn` | UnbindPacketConn embeds `N.ExtendedConn` и преобразует WriteTo/ReadFrom в Write/Read; SyscallConn, ReadMsgUDP и WriteMsgUDP в его method set не появились |
| Выбор vectorised Salamander | `salamander.go:24–39`; `sing/common/bufio/vectorised.go:65–77` | Factory проверяет внешний N.VectorisedPacketWriter/syscall.Conn/RawConn, не вызывает Upstream рекурсивно. UnbindPacketConn не удовлетворяет этим interfaces → scalar Salamander |
| Выбор quic-go rawConn | `sys_conn.go:51–80` | Type assertions выполняются по внешнему PacketConn. Обёртка не становится OOBCapable только потому, что внутри лежит UDPConn |
| Windows newConn даже без обёртки | `sys_conn_windows.go:12–14` | Возвращает basicConn даже для OOBCapablePacketConn. `basicConn.capabilities()` задаёт только DF: GSO=false, ECN=false |
| QUIC TX | `connection.go:2564–2585` → sendPacketsWithoutGSO → sendQueue → sconn.Write → basicConn.WritePacket | gsoSize=0, ordinary buffer; ReadMsgUDP/WriteMsgUDP и GSO aggregation этой веткой не выбираются |
| Salamander TX | `salamander.go:57–69` | Отдельный managed buffer длиной payload+8, salt и transform; underlying WriteTo делегирует connected UDP Write |
| Go Windows TX | `net/net.go:204–212`; `internal/poll/fd_windows.go:830–879` | Connected UDP Write → WSASend с одним WSABUF, OVERLAPPED и execIO; не WSASendTo и не WSASendMsg в этом scalar trace |
| Go Windows RX | Unbind.ReadFrom → UDPConn.Read → `net/net.go:192–201`, `fd_windows.go:558–612` | WSARecv с одним WSABUF, затем Salamander ReadFrom transform; quic-go basicConn выставляет receive time после ReadFrom |
| Сырой UDPConn без Unbind (альтернативный caller) | Go `udpsock_posix.go:46–69,113–136` | ReadFrom/WriteTo → WSARecvFrom/WSASendto; это отдельный API-путь, а не имя syscall текущего connected trace |

Результат ограничен указанной композицией. Custom PacketConn с иными методами
или другая версия библиотеки может изменить factory-ветку; фактический
Windows quic-go newConn этого pin всё равно возвращает basicConn.

### 2.3 Scatter/gather, segmentation, coalescing — разные свойства

В sing 0.8.13 существует Windows `SyscallVectorisedPacketWriter`:
`vectorised_windows.go:45–98` строит WSABUF-массив и вызывает WSASendTo с
NULL overlapped/completion. Это **scatter/gather частей одной датаграммы**,
а не batch разных датаграмм и не UDP segmentation. В рассмотренном HY2 client
данная ветка не достигается из-за Unbind method set. Это не утверждение об
отсутствии vectorised-кода вообще или о его применимости ко всем серверам.

Официальные Microsoft документы действительно прочитаны, включая ru-RU:

- WSASendTo/WSASend: один или несколько буферов одного сообщения; completion
  не означает доставку peer или обработку приложения.
- UDP_SEND_MSG_SIZE > 0 включает разбиение outgoing buffer; default 0 — без
  segmentation. UDP_RECV_MAX_COALESCED_SIZE > 0 включает объединение сообщений;
  default 0 — без coalescing. Для восстановления границ нужен receive API с
  control information, например WSARecvMsg и UDP_COALESCED_INFO.
- WSASetUdpSendMessageSize и WSASetUdpRecvMaxCoalescedSize — типобезопасные
  wrappers; их pages требуют Windows 10/Server version 2004, build 19041.
  Доступность API не доказывает успешную настройку, NIC-offload или fast path.

Поиск setters в reviewed Windows backend, sing-box dialer, sing control и Go
net не нашёл настройки этих Windows UDP options. Этот отрицательный поиск
имеет только названный scope; основание отсутствия **quic-go GSO** сильнее —
положительная Windows branch/basicConn source-проверка. Linux путь отдельно
ограничен build tags: OOB-файл `darwin || linux || freebsd`; Linux helper
проверяет kernel major≥5 и UDP_SEGMENT, формирует cmsg; runtime env может
отключить GSO. Эти условия не переносим на Windows.

Возможность Windows-USO/URO — кандидат дальнейшей совместимости, не готовый
adapter и не основание патча: Salamander требует отдельный salt/transform на
каждую on-wire UDP-датаграмму; разделение агрегата после одного transform
изменило бы framing. Receive-coalescing без split по control metadata также
не соответствует текущему single-datagram ReadFrom контракту. Ускорение неизвестно.

## 3. R04-010-B — одна корректная граница очереди, не глобальная RAM

### 3.1 Определения и доказуемая область

Пусть Q — число entries непосредственно в `sendQueue.queue`; L_i — encoded
payload length i-го entry; B=1452 байта — MaxPacketBufferSize этого pin.
Для reviewed Windows non-GSO пути, корректных успешно сериализованных packets,
без custom packer и без replacement queue внутри выбранного среза:

**Q≤8; Σ L_i≤8B=11 616 байт.**

Обоснование:

1. `send_queue.go:35–44` создаёт channel capacity 8; Send либо enqueue, либо
   stopped branch, либо panic. Он не увеличивает capacity.
2. `connection.go:2570–2605` использует ordinary `getPacketBuffer`; GSO=false.
3. `packet_packer.go:951–957` проверяет размер successful short-header packet
   против maxPacketSize; config/MTU builders ограничены protocol B. ACK/PTO/
   coalesced builders также используют ordinary buffer в просмотренной ветке.
4. Factory `buffer_pool.go:85–91` создаёт ordinary backing capacity B, large
   20KiB используется в другой, GSO, ветке.

Последний пункт даёт также **8B backing-capacity** для queue только при
инварианте «валидный builder не реаллоцировал oversized backing». Общий
allocator округляет свои allocations, error path может временно выделять иное;
поэтому unconditional theorem о heap/RSS из byte-length здесь не выводится.

### 3.2 Что за пределом очереди

| Держатель | Count/размер в выбранной области | Почему не входит в 11 616 байт |
|---|---|---|
| Выполняющийся send worker | До 1 dequeued ordinary buffer B | Run извлёк entry и может ждать Write |
| Последовательный producer/builder | До 1 ordinary buffer B в рассмотренной ветке | Builder ещё не enqueue; error/panic path требует отдельного учёта |
| Scalar Salamander staging | RoundUpPow2(L+8), max 2048 при L≤1452 | `buf.NewSize` → reviewed default allocator; логическая capacity и физический backing различны |
| Kernel/socket buffer | Не измерен и не установлен как hard cap этим доказательством | DesiredSend/ReceiveBufferSize — пожелания quic-go; на scalar Unbind setters отсутствуют, errors wrapConn игнорирует |
| Pools/GC/history/TLS/frames | Отдельные структуры/allocations | Возврат в sync.Pool не означает возврат RSS ОС |
| Direct probes, error branches и другие connections | Не включены | Есть send-path обходы queue и другие владельцы |

Условная сумма названных живых byte backings для одной нормальной scalar
pipeline: `(8+1+1)*1452+2048=16 568 байт`. Это **не** полный heap одной QUIC
connection, не максимум transient allocations при ошибке, не global budget
процесса и не latency/RAM benchmark. Самый надёжный утверждаемый предел —
11 616 encoded байт именно внутри queue.

### 3.3 Сильнейший контрпример к переносу queue bound на всю систему

`Conn.SendDatagram:3040–3057` сначала создаёт копию payload, **затем** вызывает
`datagramQueue.Add`. Add ограничивает queued frames 32, но при заполнении
отпускает mutex и ждёт. N concurrent callers могут уже удерживать N copies
вне queue. Для фиксированного максимального D и известных N:

`B_copied_send ≤ (32+N)*D`

— лишь логическая сумма DATAGRAM copies этих владельцев, без caller original
buffers, frame/stack overhead и других stages. Без заданной upper bound N
из count32 нельзя получить численную глобальную границу. Это условный trace
API, не утверждение о числе concurrent callers фактического приложения.

RX: `datagram_queue.go:93–109` копирует до проверки cap128;
`hysteria2/client_packet.go:5–18` ReceiveDatagram вычитывается циклом, который
запускает goroutine на message. Cap128 не ограничивает все уже извлечённые
messages/handlers. У HY2 также per-session data channel64, udpConnMap,
defragger cache и application buffers. Их scopes различны. Нужны simultaneous
handler/session/producer caps и release accounting, а не ещё один общий score.

Port hopping создаёт отдельную packetChan capacity1024 и recvLoop buffers2048
(`hysteria/hop.go`); значит, доказательство non-hopping не охватывает весь
optional профиль. Close не является drain всего packetChan по одному имени API.

## 4. Адресный пересмотр прежнего R04-008

| Прежняя формулировка | Перепроверенное основание | Корректный предел |
|---|---|---|
| `peer_conn_window=20MB` — cap retained stream/history/RAM | HY2 local receive windows 8MiB/20MiB; sender credit получает от peer; ackhandler хранит frames/metadata отдельно | Это receive credit/window, не process RAM, не общий sender retransmission cap; число «20MB» — в source ровно 20MiB |
| ACK-пауза сама исчерпывает flow-control | Credit растёт по MAX_DATA/stream data consumption; ACK и flow-control — разные сигналы | Отсутствие ACK не тождественно отсутствию MAX_DATA. DATAGRAM вообще не расходует этот credit |
| PTO ничего не удаляет и DeclareLost только при позднем ACK | `connection.go:2709–2745` → `QueueProbePacket`; `ackhandler:1083–1095` → DeclareLost/removeFromBytesInFlight/queueFramesForRetransmission | Сам timeout handler лишь задаёт пробы, но end-to-end PTO caller удаляет исходный history packet. Sampler cleanup — отдельный контракт |
| History без hard cap: нужны только window/min-frame | `SendMode:1022–1065` checks 40 000/50 000 tracked slots по params | Guards существуют, хотя capacity, overshoot между checks, PN holes и sampler — отдельно. Новый exact RSS-cap не заявлен |
| Idle lifetime ≤30s | `nextIdleTimeoutTime:853–856` = start + max(negotiated idle,3*PTO); start учитывает first ack-eliciting send после RX | Нет безусловной 30s-bound от момента последнего RX; успешный receive рестартует idle, так что active carrier lifetime вообще не ≤30s |
| 60s PTO cap исключает любые слишком частые события | `getScaledPTO:665–671` задаёт upper clamp; base PTO = RTT+variance+ackdelay | 60s не minimum interval; логарифмическая формула probe count требует фиксированного base, отсутствия resets/ACK и знания schedules. Общий no-spin theorem не выведен |
| singleflight означает ≤1 живое поколение и release | `offer` coalesces pending; `completeOffer` signals done до discarded-result cleanup; `CloseWithError` не ждёт pending goroutine | В одном Client ≤1 зарегистрированный pending offer, не все retiring holders; ниже точный conditional trace |

RFC 9221 §5.3 прямо исключает DATAGRAM из per-flow/connection-wide data limits;
§5.4 сохраняет congestion control. ACK DATAGRAM (§5.2) не означает полезную
обработку application. RFC 9000 §10.1 подтверждает timer restart и floor3*PTO.
Нормативный close/discard state не становится доказательством Go GC или
своевременного возврата памяти конкретного implementation.

## 5. R04-010-C — singleflight, graceful drain и настоящий Windows owner

### 5.1 Два разных release barrier

При завершении соединения HY2:

1. `clientQUICConnection.closeWithError` closes connDone и cancels UDP sessions.
2. Затем вызывает `quicConn.CloseWithError`, который ждёт `c.ctx.Done()`.
3. QUIC run при выходе **сначала** вызывает `sendQueue.Close()`; Close ждёт
   runStopped, а send worker дренирует очередь через underlying Write.
4. После drain выполняется `handleCloseError`, возврат run и deferred ctxCancel.
5. Лишь после возврата QUIC Close HY2 выполняет `rawConn.Close()`.

**Conditional counterexample:** underlying Write не возвращается в
рассматриваемый промежуток и нет внешнего deadline/close/termination. Тогда
run ждёт send queue, CloseWithError ждёт context, а raw socket closure ещё не
достигнута. Idle обнаружение не задаёт upper bound release-time. Это source
interleaving, не воспроизведённое зависание Windows или доказанная частота дефекта.

Сильный противоположный случай: Go execIO умеет отмену по socket close/deadline,
CancelIoEx и waitCanceled; успешные writes быстро drain, QUIC Close возвращается.
Предпосылка «write может ждать» не означает «каждый write зависает». Но указанный
Close caller сам не обеспечивает early raw Close для прерывания своей drain.

### 5.2 Singleflight не покрывает retiring generations

`offer:112–154` публикует один pending под mutex. Timeout foreground caller
прекращает ожидание, не shared offer. `CloseWithError:341–359` помечает pending
как discarded, отменяет и возвращается без ожидания completeOffer.
`completeOffer:156–184` clears c.pending, closes pending.done и unlocks **до**
close discarded conn. Возможен точный порядок: successful old offer result;
concurrent close mark discarded; completeOffer освобождает pending slot;
следующий offer начинает replacement, пока old-result close ещё выполняется.

Для active conn `closeWithError` closes connDone раньше QUIC drain; новый offer
может пройти `active()==false` до полного освобождения old conn. Следовательно,
нет source-доказательства «singleflight ⇒ всё старое освободилось». Bound на
число одновременно retiring holders требует дополнительных generation/return
receipts или platform process boundary. Пакет не заявляет бесконечное число
поколений при actual finite workload.

### 5.3 Windows composition содержит более сильный процессный owner

В реальном Windows приложении нельзя игнорировать existing worker barriers:
`corebridge:465–474` не заменяет manager при failed Stop;
`worker_process.go:269–290` не забывает uncertain owner и запрещает повторный
Start после cleanupErr. После graceful stop request (budget2s) retire использует
отдельный budget4s, Terminate owned Job, Wait root, wait ioDone и Job.Empty.
Ошибки query/Wait/handles означают unresolved owner, не синтетическое освобождение.

`worker_host_windows.go:142–149` для engine задаёт processes8, но commitBytes=0;
`worker_spawn_windows.go:73–76` включает JobMemoryLimit только при ненулевой
policy. **Engine-specific hard Job commit-memory cap здесь не настроен**.
Это не отсутствие любых Windows/OS/provider limits. Reap success — сильнее
кооперативного QUIC Close, однако source native calls не означают measured
4s syscall bound или глобальный process RSS-budget.

## 6. R04-009: что не переносится в новую теорию

Отрицательный поиск cap в service.go не доказывает отсутствие admission во всём
closure: quic-go server queues 32 accept/1024 unprocessed; `handleNewConn` отказ
при full accept queue; `GetConfigForClient`/ConnContext могут отказать до newConn;
address-validation/anti-amplification/0RTT queues имеют свои guards.

Противоположная граница: accept-queue32 — не cap всех handshake или уже принятых
active sessions. Reviewed server может начать handshaking goroutine до queue;
нельзя заменить старое «вообще нет guards» новым «все ресурсы bounded32».
Конкретные deploy admission/host settings вне задачи и не изучены.

«Трилемма CCA» в R04-009 не имеет сформулированной и доказанной impossibility
теоремы в том baseline; ни full-text paper, ни field attribution не прочитаны
этим пакетом. Она **не принимается и не используется** для bounds/рекомендации
CCA. Paper остаётся LINK_ONLY; перенос её выводов требует отдельного full-text
review. Нет claim о том, что выявленный Windows путь улучшит обход блокировок.

## 7. Reuse, цена и адресаты

- Сохранить actual scalar Windows path как baseline, не добавлять GSO-патч по
  Linux-инструкции. quic-go pin MIT, sing/sing-quic GPL-3.0-or-later; sing-box
  root GPL-3.0-or-later + ограничение naming/association, не вся license closure.
- Reuse существующих queue/size guards и Windows worker ownership/reap, а не
  новый packet engine. Graceful drain, retained owners и process termination —
  отдельные observation kinds; failure не даёт direct fallback.
- TH-06/FR lifecycle адресаты: выводить admission из максимальных **всех**
  holders (queued+executing+waiting+retiring), не из receive window. Нужны N
  callers, число sessions/handlers, allocation capacities, interval work и
  completion receipts. Числа без actual inputs не подставлять нулём.
- Возможный Windows-USO/URO adapter — отдельный совместимый transport контракт,
  учитывающий salt на каждую датаграмму, receive-boundaries, ошибок и ownership;
  no drop-in readiness, неизвестный benefit и regression cost.
- Android clocks/Doze не рассматриваются этим пакетом; syscall-путь Windows
  нельзя переносить на Android по одинаковому Go API.

## 8. Конечная будущая программа — все NOT_RUN

| Case | Что должно быть проверено при отдельном разрешении |
|---|---|
| V1 | Artifact/toolchain attestation и actual concrete socket/wrapper type для scalar/hopping/custom профиля |
| V2 | Non-GSO successful builder sizes и queued≤8B; отдельно in-service, builder, staging physical capacities |
| V3 | Queue full + N concurrent SendDatagram: copies вне cap32; observer показывает blocked callers и release |
| V4 | RX DATAGRAM burst: queue128 против post-dequeue handlers/session64/defragger; count и bytes без conflation |
| V5 | Idle с RTT/PTO>10s и first ack-eliciting send: отсутствие обещания ≤30s от last RX; no ACK против no packets |
| V6 | PTO end-to-end: QueueProbePacket DeclareLost и отдельный BBR sampler retirement |
| V7 | Write задержан/ошибся/canceled: queue drain, CloseWithError, raw socket close; grace/reap результат отдельно |
| V8 | Pending completed discarded conn + новый offer до старого release: source trace и actual generation receipts |
| V9 | Windows manager Stop failure: replacement blocked; Job terminate/wait/empty fail retains owner |
| V10 | Windows USO/URO только в изолированном compatibility-профиле: framing/salt/segment sizes, API failure, no bypass |

Не выполнялись тесты, fixtures, benchmark, build, crypto, Windows native
вызовы, сетевые пробы, server load или изменения policy. Источники читались
через public HTTPS, это не live проверка VPN. Корректность whole RSS/CPU-budget,
severity/frequency conditional close trace и выигрыш fast path **unknown**.

## 9. Предлагаемый фрагмент для главы (не интегрирован автором)

> В рассмотренной Windows/HY2-композиции QUIC работает через scalar connected
> UDP/Sаламander и basicConn без GSO/ECN fast path. Cap8 даёт 11 616 encoded
> байт именно в ordinary send queue; executing/builder/staging/kernel/pools и
> DATAGRAM handlers считаются отдельно. Receive-window20MiB не ограничивает
> global RAM; idle30s и singleflight не доказывают release retiring owners.
> End-to-end PTO caller может удалить history packet; Windows app использует
> дополнительный worker termination/reap barrier, не hard engine memory cap.
> Область source/doc, native и performance проверки NOT_RUN — R04-010.

