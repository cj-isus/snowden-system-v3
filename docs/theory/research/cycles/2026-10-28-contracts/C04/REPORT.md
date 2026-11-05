# R28-C04 — один accounting/lifetime contract для HY2 UDP

28 октября 2026. **READY_FOR_REVIEW**, source/spec + ручной вывод. Научный максимум
автора — READY_FOR_REVIEW; нет product/native/device PASS. Источники и область
чтения: [SOURCES](SOURCES.md).

## 1. Ограниченный ответ и новая дельта

**R28-C04-A (conditional contract).** Ограничить attributable backing capacity
можно, если admission выполняется до существенного allocation/spawn, учитывает
waiting/in-service/retiring owners и удерживает reservation до фактического
release/transfer/reap. Дополнительно нужны самостоятельные limits metadata,
пулов, sessions, callbacks, packet-number span и generations. Очередь 32/128/8,
flow-control window и timeout сами такой контракт не замыкают.

**R28-C04-B (source-scoped cancellation seam).** У выбранной пары отдельный
`udpPacketConn.Close()` отменяет session context, но уже вошедший
`SendDatagram → datagramQueue.Add` не ждёт этот context. Его ресурсный slot нельзя
освобождать при локальном Close или истечении caller timeout. Optional port
hopping добавляет mutex, удерживаемый на протяжении raw write; отмена через
Close/SetWriteDeadline этой же обёртки ждёт тот же mutex.

Новая дельта к REPORT-010/011 — единый проверяемый ownership contract и точные
seams, на которых generic semaphore/timeout даёт ложное освобождение. Прежние
queue bounds, F+2L и sleep-оговорки здесь не объявляются новыми находками.
Глобальный численный RAM/SLO/energy bound текущего продукта **не установлен**.

## 2. Actual declarations, wrapper и выбранная область

Windows `go.mod` объявляет sing-box1.13.19; effective declared pair:
sing-quic `d83826c306d7c008cafb3fd6d8ee07cbcfd656ed` + quic-go
`ccd46abedd39c6f8384b1df8daf42008724958f5` / mod.4. Также sing0.8.13 и
x/sync0.20.0. Прямой manifest приложения важнее более старого transitive minimum
в module manifest sing-quic. Без выполнения resolver/build это declared graph,
не attestation EXE.

Исследуем source branch HY2 UDP без detour, без hopping, со scalar
Salamander, который Windows renderer явно задаёт в HY2 outbound; hopping отдельно
в §6. Реальный профиль не читался. Renderer
передаёт HY2 options; corebridge создаёт isolated worker; worker вызывает
NewEngine → box.New. Source wrapper:

```text
route ConnectionManager.NewPacketConnection
  → HY2 Outbound.ListenPacket → Client.ListenPacket / shared offer
  → route packetConnectionCopy → sing bufio.CopyPacket
  → HY2 udpPacketConn.WritePacket
  → pack / fragment aliases → quic.Conn.SendDatagram
  → datagramQueue.Add(cap32) → packet packer.Pop
  → encoded QUIC packet + distinct sent-history Frame.Data
  → sendQueue(cap8) → worker Write → Salamander staging
  → Unbind.ExtendedConn.Write → connected UDP Write / Windows WSASend
```

Unbind method set и vectorised factory не превращают вложенный UDPConn в
внешний OOBCapable/vectorised writer. Windows quic-go newConn возвращает
basicConn; рассмотренная ветка non-GSO. После Write освобождается serialized
packet buffer, но DATAGRAM payload может оставаться в sent history.

Android source-lock указывает тот же core commit
`b5ebaa1fc0f2b94256180b95468e73ef53caa27d`, но Go/AAR digest=null, required
strict-owner overlay source-only, stock AAR несовместим. Kotlin разрешает
HY2/Salamander/hopping как options; выбранный installed wrapper/ABI/runtime
**UNKNOWN**. Перенос Windows syscall/clock результата на Android не выполнен.

## 3. Holder graph: считать allocations и переходы, а не названия queues

| Holder/stage | Что реально удерживается | Transfer / release receipt |
|---|---|---|
| Caller copy / route upload | Исходный buf.Buffer, destination metadata, stack | WritePacket фактически вернулся; source.Close/onClose сам этого не доказывает |
| HY2 fragment list | Descriptors/slice; fragment `buf.As` ссылается на исходный backing | Список очищен после последовательных writePacket; исходный backing один, не F независимых U |
| Current HY2 pack | Новый buffer с HY2 header+fragment, pool rounding | `writePacket` вернулся и его defer исполнился; это может ждать Add |
| DATAGRAM pre-enqueue | `SendDatagram` уже создал отдельный `f.Data` copy | Add вернул ошибку либо передал owner очереди; caller original и pack пока могут жить |
| DATAGRAM queue | До 32 frames; ring references | Pop переносит frame в packet/history или отбрасывает; dequeue не возвращает все payload credits |
| Packer / sent history | Frame.Data прежний, serialized ciphertext — другая allocation | ACK/loss/discard clears/releases references; socket Write не terminal receipt для Frame.Data |
| sendQueue / in-service | До 8 queued ordinary buffers, один извлечённый worker, builder | Успешный Write и Release; error/Close требуют отдельного owner accounting |
| Scalar Salamander / OS | Отдельный staging payload+8, pinned write storage, kernel socket state | Actual I/O completion; cancel request и peer receipt — другие факты |
| RX queue → handler | До 128 копий; после ReceiveDatagram goroutine удерживает ту же копию | Handler return/transfer, а не dequeue. Decoded payload slice alias whole received backing |
| RX defrag / assembly / ready | Fragments, output overlap, per-session ready64, borrower | Последняя owned reference / корректный eviction / reader return; TTL10 не eager reap |
| Sampler/history metadata | Packet records, span with holes, backing high water | Present removal не обязательно shrink; отдельный metadata reservation |
| Retiring generation | Всё ещё живые queues, handlers, sockets, cleanup goroutines | Quiescent join либо подтверждённый owned-process reap; timeout не release |

Route реально запускает две последовательные copy goroutines на packet
connection (`route/conn.go:257–258`). Это существенно сильнее модели
«произвольное число writers на одной сессии», но не задаёт process cap числа
сессий. `packetConnectionCopy:375–380` вызывает onClose перед common.Close;
он не содержит join другой copy goroutine. Считать onClose proof всей пары нельзя.

Utx≤4096 — length gate WritePacket/WriteTo, а не доказательство capacity
входного buffer. `ReadWaitOptions` учитывает reader MTU/headroom/overhead и
может выделить больше. Managed allocator sing округляет backing до power of two
в своём диапазоне; buf.As не делает независимую payload copy. `Release` в
sync.Pool не доказывает уменьшение RSS.

## 4. Условная арифметика и достаточные предпосылки

### 4.1 Единицы и локальные envelopes

Обозначим ρ(x) ёмкость выбранного managed byte backing, а не его текущую Len;
Go allocator/objects/stacks учитываются отдельно. Для одной generation g:

- A_g — одновременно удерживаемые caller write operations, включая canceled,
  которые ещё не вернулись; W_g⊆A_g — pre-enqueue owners DATAGRAM copy.
- Ucap — cap исходного caller backing; Hmax — проверенный размер HY2 header;
  Fmax — проверенный fragment count; D — cap отдельной accepted DATAGRAM copy.
- H_g — число уже dequeued DATAGRAM copies, ещё принадлежащих packer/history.
- P_g — число nonterminal RX handlers; S_g — sessions; R_g — incomplete groups;
  X_g — одновременно assembling/borrowed outputs; Z_g — ready messages.
- Cmeta_g — реально выделенные capacities metadata containers; H_g не заменяет
  packet-number span, present count и Cmeta_g друг другом.

В нормальной sequential fragment ветке caller original один, fragment payload
aliases не добавляют Fmax×Ucap. Для названных TX byte backings достаточен envelope:

`TX_g ≤ A_g·[Ucap + ρ(Hmax+min(Utx,Cfrag))] + (32+W_g+H_g)·D
       + (8+1+1)·B + ρ(B+8)`.

Здесь Cfrag>0 и Fmax проверены до построения fragment list, B=1452 только для
прочитанной ordinary non-GSO ветки. Метаданные fragment list входят отдельно
в `A_g·O(Fmax)`. Формула не разрешает Cfrag≤0: baseline fragmenter не обеспечивает
эти preconditions сам. Она также не включает error oversize/transient allocations,
kernel, TLS, strings и pools, пока для них не назначен отдельный envelope.

`(32+W_g+H_g)·D` — partition одной и той же Frame.Data allocation по стадиям:
pre-enqueue / queued / dequeued-retained. Alias из packer в history учитывается
один раз. Serialized QUIC bytes и HY2 pack — действительно иные allocations.
H_g требуется ограничить отдельно; здесь он **не** подменён receive window.

**Ручной достижимый cut, не запуск.** Возьмём caller buffer cap4096,
destination `192.0.2.1:443` (13 bytes), исходную empty DATAGRAM queue и отсутствие
её drain на протяжении четырёх быстрых Adds. При udpMTU1197 header H=22,
Cfrag=1175; U4096 даёт fragments1175/1175/1175/571. Packed lengths:
1197/1197/1197/593. После создания третьей Frame.Data copy, до возврата третьего
writePacket, одновременно существуют `original4096 + current pack2048 +
retained frame copies3591 = 9735` — **9735 byte backing capacity** названных
объектов. Fragment payload aliases не добавляют ещё 3×4096. После четвёртого
Add frame copies всего 4184, но текущий pack округляется до 1024. Это наглядно
опровергает reservation «размер пользовательской датаграммы=весь peak». Здесь
не учтены descriptor objects, pools, metadata и runtime; не заявлен process
peak9735, RSS или стоимость реального пакета на wire. Queue32 не исчерпана.

На RX нужны размеры *whole copied backing*, не только tail payload slice.
При Dr как cap такой копии грубый достаточный envelope для названных holders:

`RX_g ≤ (128+P_g+R_g·Fmax_rx)·Dr
       + (X_g+Z_g)·max(Dr, ρ(Urx_assembled_max))`.

Это намеренно консервативная верхняя оценка; output/source overlap остаётся
учтённым, alias между handler и его decoded message не суммируется. Некоторые
phases не одновременны; более тесную оценку разрешает только proved state cut.
max нужен потому, что готовый single-fragment message может по-прежнему
держать весь received backing с header, даже при коротком tail payload.
Необходимы явные Urx и assembly gates: штатный TX4096 не ограничивает decoded
RX assembly. Ready64 — per-session, поэтому Z_g≤64·S_g, а не общий 64.

Wire packetID uint16 ограничивает число distinct keys в *одной* defrag map
65536, fragmentTotal uint8 даёт максимум 255 позиций. Это сильный контрпример
слову «математически безграничный cache». Эти widths не являются практической
quota, не ограничивают все sessions/handlers/borrowers и не задают release time.
Доклад не выводит remote DoS или фактическую RAM из предельного key space.

### 4.2 Sampler: count, span и high water

В meta2 indexed queue insertion заполняет holes; `NumberOfPresentEntries`,
`EntrySlotsUsed=entries.Len()` и `len(ring)` — три разные величины. Для positive
initial capacity C0 и исторического peak used span S* после штатных doubling grows:

`C_backing ≤ max(C0, 2·S*)` (slots, не bytes).

При одном grow одновременно живут старый C и новый 2C backing: transient3C
slots плюс отложенный GC старых allocations. Pop/Clear обнуляют элементы,
но не уменьшают `len(ring)`. Поэтому даже «present=0, span=0» не даёт
backing=0. Названный sampler `maxTrackedPackets` найден как поле, а не gate.

Opposing guard: engine SendMode проверяет history Len, включая holes, против
40k/50k thresholds. Нельзя утверждать «вообще нет guards». Но это другая
структура: её предел без end-to-end lifecycle/span correspondence не доказывает
предел sampler backing. ACK/loss cleanup реально существует; PTO
QueueProbePacket удаляет engine record и очищает frames отдельно. Самостоятельный
unbounded-growth theorem для actual carrier здесь **не доказан**.

### 4.3 Admission theorem — что именно он гарантирует

Для полезного верхнего bound нужны следующие **дополнительные условия**, ещё
не реализованный продуктовый контракт:

1. До payload clone, grow и goroutine spawn зарезервированы count и conservative
   capacity cost. Уже полученный ingress backing тоже принадлежит ограниченному
   owner; admission «после ReadPacket» не стирает его стоимость.
2. Ограничены *все* ожидающие операции, включая semaphore waiters и очереди
   caller/UI/Binder/native. Busy не обходит лимит новым retry goroutine.
   Например bounded nonblocking count slot берётся до byte Acquire; при его
   отказе copy/spawn не происходит. Внешние callers тоже принадлежат ingress
   envelope. Стоимость вычисляется checked arithmetic; negative/oversize cost
   отклоняется до обращения к primitive. Отказ Acquire не требует Release.
3. У reservation identity `{process, generation, operation, allocation-or-envelope}`.
   Transfer сохраняет charge; clone/grow предварительно приобретает дополнительный
   charge; alias не приобретает второй payload charge. Accounting transition
   атомарен относительно publication, cancellation и new generation admission.
4. Byte ledger включает payload и отдельные metadata/pool/stack/scratch envelopes.
   Pooled backing остаётся charged в pool owner. Нельзя объявлять физическую
   память свободной только после удаления последней logical message reference.
5. Глобальный generation slot сохраняется для active/pending/retiring/uncertain;
   unresolved cleanup запрещает дальнейшую замену, либо заранее резервируется
   конечное число retiring slots. Нужен такой же глобальный count sessions/handlers.
   Один platform worker slot не означает одну внутреннюю QUIC generation. Cache
   eviction переводит borrowed holder в retiring, а не освобождает его storage
   до окончания borrowers; один новый WithSize не обеспечивает этот lifetime.
6. Original owner остаётся custodian при cancel/timeout/partial close. Освобождение
   только по actual final return/ref cleanup или утверждённому whole-process reap.
   Для parent-held RPC/source/FD state одного child death недостаточно.

Если для каждой охваченной allocation a доказана reservation charge q(a),
`capacity(a)≤q(a)`, charge не освобождён раньше owner, а суммарный charge≤B,
то **сумма охваченных backing capacities≤B**. Это conservation proof, не
утверждение, что все перечисленные условия уже выполняет ядро. Нужны finite
explicit B и capacity ceilings; из пустого параметра не получается число.

Полезная граница честного результата:

`M_process = M_covered + M_runtime/GC + M_foreign/JNI + M_mapped/image + …`.

Неизвестный residual не заменяется нулём. RSS/PSS/working set/commit и kernel
memory — отдельные измеряемые величины. Go SetMemoryLimit — soft runtime target,
исключающий foreign/kernel/image memory; Windows JobMemoryLimit относится к
committed virtual memory. [Go contract](https://pkg.go.dev/runtime/debug@go1.26.7#SetMemoryLimit),
[Job contract](https://learn.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-jobobject_extended_limit_information).

Memory admission ограничивает конкуренцию, но само не даёт hard CPU, elapsed
или energy bound. Например indexed insertion может сделать O(PN gap) работы при
маленьком present count. Нужны отдельные work/span ceilings и scheduler/I/O
предпосылки, а на Android — device attribution.

## 5. Cancellation: самый сильный trace и существующий противовес

### 5.1 Отдельная UDP-сессия

При существующем shared carrier, заполненной DATAGRAM queue и отсутствии её
прогресса в рассматриваемом интервале:

1. WritePacket проверил session ctx и успел выделить pack/copy.
2. SendDatagram вошёл в Add; Add ждёт только queue.sent или **queue.closed**.
3. Session.Close отменил c.ctx, удалил session из map и вернул nil.
4. Ни session ctx, ни per-write deadline не входят в Add. SetWriteDeadline
   HY2-сессии возвращает os.ErrInvalid. Ожидание сохраняет caller/pack/copy owners.

ListenPacket возвращает именно net.PacketConn; его documented Close contract
предусматривает разблокирование уже ожидающих WriteTo с ошибкой. У HY2 WriteTo
имеет тот же SendDatagram path, поэтому найден **условный source-level разрыв
этого контракта**, а не только отсутствие желаемой оптимизации. Это не обещание
конкретного нормативного времени Close и не runtime reproduction.
[Go1.26.7 PacketConn](https://pkg.go.dev/net@go1.26.7#PacketConn).

Это достижимый на уровне API interleaving при заданной паузе drain, а не
наблюдённое зависание Windows. Если queue Pop происходит, Add продолжает работу;
если whole-carrier queue.closed закрывается, waiter возвращается. Сильный guard
существует, но относится к другой lifetime. Caller timeout вокруг goroutine
даёт caller-return, а не producer-return и не освобождает slot.

`Add` проверяет closed channel только на full-queue wait path. При наличии
места операция, уже прошедшая admission, может enqueue даже после закрытия.
Это ещё одна причина иметь explicit terminal admission fence и terminal owner;
не утверждение, что такой payload обязательно выйдет на сеть.

### 5.2 Whole carrier и blocked raw write

HY2 close сначала закрывает connDone и cancels sessions, затем ждёт
QUIC.CloseWithError; QUIC run ждёт sendQueue.Close, и лишь после drain закрывает
datagramQueue/context. RawConn.Close вызывается HY2 после возврата QUIC Close.
Если underlying Write не возвращается и нет независимой отмены, получается
цикл ожидания; idle detection не является release deadline.

Если после начала Close новых producers нет, q≤8 и каждый текущий/queued Write
завершается за T_w при известном scheduling overhead δ, условный send drain
занимает не более `(q+1)·T_w+δ`. **T_w не дан этим API**, поэтому из queue8
нельзя получить секунды. Error return тоже требует учёта оставшихся queued refs.

Сильный противовес: Go1.26.7 Windows source использует overlapped WSASend,
socket close/deadline и CancelIoEx/waitCanceled. Нормальные writes завершаются;
этот trace не говорит, что каждый write висит. CancelIoEx только запрашивает
отмену; completion необходим до reuse storage.
[Microsoft contract](https://learn.microsoft.com/en-us/windows/win32/api/ioapiset/nf-ioapiset-cancelioex).

## 6. Optional hopping и повторные generations

`hysteria/hop.go:178–184` удерживает c.access во время currentConn.WriteTo.
Close187–205 и SetWriteDeadline233–239 сначала берут c.access. Поэтому даже
перестановка «сначала wrapper.Close» не создаёт independent interrupt path для
уже заблокированного write: она ждёт тот же lock. Требуется отдельный inspected
socket cancellation owner либо process retirement; blind method reorder недостаточен.

Hopping также добавляет queue1024 buffers по 2048, плюс buffers recvLoop,
prev/current sockets, возможный candidate/old local owner. Эти 2MiB — только
queue backing envelope, не heap/profile total. Close не дренирует packetChan и
не join-ит recvLoop/hopLoop. Поля prev/current не доказывают cap всех retiring
goroutines. Source обрабатывает doneChan в errChan send — существующий guard
против конкретного stuck error publication надо сохранить.

Client singleflight ограничивает pending offer, но completeOffer освобождает
pending slot до discarded-result Close; connDone закрывается до QUIC drain.
Conditional internal-generation overlap возможен. **Нельзя** отсюда вывести
бесконечную whole-worker замену текущего приложения:

- Windows corebridge не публикует новый engine при failed prior Stop;
  cleanupErr сохраняет original process owner; retire требует Terminate owned
  Job, Wait root, ioDone и Job.Empty. Engine job сейчас processes8, без
  configured commitBytes. Source2s graceful+4s reap budgets не preempt synchronous
  native calls и не являются измеренным hard6s bound.
- Android NativeWorkerSession claim не сбрасывается для повторного старта;
  даже local graceful return требует process exit. Worker watchdog arm-ит
  retirement до optional cleanup. Parent repository не освобождает binding,
  пока actual RPC lanes не вернулись и prepared source не sealed+idle, даже
  если child уже умер. Не удалять эти guards ради «быстрого reconnect».

Следовательно, самый маленький полезный admission seam — scoped shared-carrier
UDP producer/handler budget с retained retiring slots, композируемый с existing
platform worker owner. Это не второй lifecycle controller и не разрешение
убивать весь carrier при каждом потерянном UDP сообщении.

## 7. Exact sender/controller pair и готовые primitives

### 7.1 Совместимость имеет несколько независимых осей

Candidate pair, исследованная заново по exact публичным исходникам:
sing-quic0.7.0 `4ab2eceaac81e073f53b22ec72ff56aaea89a3d1` + quic-go
0.61.0-sing-box-mod.6 `f7b7e9c4c6f6ec45e4ab2b4788eca63192975d80`.
Это **reference candidate**, не изменение declared graph продукта.

| Combination / property | Source result | Что ещё требуется |
|---|---|---|
| Current mod.4 + current Brutal | Старый Ex callback принимает time.Time; pinned Ex требует monotime и дополнительные lifecycle methods. Normal install выбирает base adapter с ACK/loss stubs | Исправлять exact method set/dispatch, а не только содержимое невызванного callback |
| Current mod.4 + current meta2 | OnPacketSent получает post F+L, sampler прибавляет L; normal snapshot F+2L | Это source mismatch конкретного поля; не удвоение wire/throughput. Event batch использует prior, не общий ±L fix |
| New mod.6 + old meta2 | Ex теперь требует OnPacketNeutered; old type не удовлетворяет полному new method set | Старый compile assertion к старому interface не доказывает новую сборку; base fallback может скрыть loss sampling |
| Old mod.4 + new meta2 | Типовые методы могут совпасть после caller adaptation, но sender всё ещё даёт post | Само копирование нового BBR не исправляет sender semantics |
| New matched pair | Injected sender получает prior; connection-wide congestion packet numbers и installation cutoff; Brutal Ex monotime; BBR forwards neuter/least-unacked | Нужны все packet kinds/lifecycle paths и actual caller adaptation; constructors и graph изменены |
| New QUIC / resource theorem | DATAGRAM Add по-прежнему ждёт только sent/closed; sendQueue.Close по-прежнему ждёт drain | Этот queue API не даёт per-session Add cancel. Новые HY2 session methods не прошли здесь полный аудит; whole resource fix из version number не следует |

New handler `neuterPacket` реально вызывается на отдельных drop/retry paths,
`notifyLeastUnacked` — после congestion batches. Но QueueProbePacket1191–1203
сам не вызывает ни один из них; его removeFromBytesInFlight224–232 только
меняет flight. Существование нового метода не доказывает coverage PTO cleanup.
Также PMTU individual loss callback guard не фильтрует добавляемый Ex loss list.
Это ограничение blanket remediation claim, **не** установленный sampler leak
или ущерб actual connection. Следующий normal ACK/loss event может очистить state.

Current HY2 после auth выбирает Brutal при fixed actualTx и иначе meta2;
actual authenticated peer mode не читался. Новый caller использует InitialPacketSize
и NewBbrSenderWithProfile, новый module требует sing0.9.0-beta.4,
x/sync0.22.0, новые x/* и Go≥1.25. Поэтому даже matched pair нельзя объявить
drop-in upgrade core1.13.19. [New manifest](https://github.com/SagerNet/sing-quic/blob/4ab2eceaac81e073f53b22ec72ff56aaea89a3d1/go.mod),
[new sender](https://github.com/SagerNet/quic-go/blob/f7b7e9c4c6f6ec45e4ab2b4788eca63192975d80/internal/ackhandler/sent_packet_handler.go).

### 7.2 Reuse/adapt/defer с ценой

| Primitive | Exact source/license | Решение и цена |
|---|---|---|
| Existing worker/process ownership | Прочитанные Windows Job и Android private-worker guards; Android GPL-3+ | **REUSE.** Не добавлять другой lifecycle owner. Стоимость: per-carrier reservations и typed return/reap receipts должны интегрироваться с ним |
| Weighted semaphore | x/sync v0.20.0, BSD-3-Clause; уже declared Windows | **REUSE AS PRIMITIVE.** Acquire(ctx,n) корректно обрабатывает own canceled wait, но waiters list не capped. Сначала конечный count/admission, затем byte reservation до clone. Проверять 0<n≤budget; oversize Acquire ждёт ctx вместо немедленного отказа |
| TryAcquire / TryGo | Та же версия/license | **ADAPT at ingress.** Nonblocking reject/coalesce до spawn и expensive capture. Mutex acquisition всё ещё может ждать scheduler; это не hard real-time API |
| errgroup.WithContext/SetLimit | x/sync v0.20.0, BSD-3-Clause | **REUSE FOR JOIN**, когда функции кооперативны. Go(f) при full limit блокируется на channel без select ctx; число *внешних callers*, ожидающих Go, не ограничено SetLimit. Wait ждёт actual return и не убивает заблокированный syscall |
| Fixed pool / current ring/queues | quic-go MIT; sing/sing-quic GPL-3+ | **ADAPT accounting.** Сохранить queue caps и drop guards, добавить отдельные counted owners/envelopes. sync.Pool и growable ring не становятся fixed arena от названия |
| Candidate CCA pair | sing-quic0.7.0 GPL-3+ / quic-go mod.6 MIT | **REFERENCE / ADAPT narrow semantics**, либо согласованный graph upgrade отдельной задачей. Цена — constructors, nominal package types, installation epoch, packet-space units, terminal callbacks, MTU/pacer и all consumers |
| Go soft memory limit / Job commit limit | Go1.26.7 BSD-style / Win32 API | **DEFER tuning.** Полезная дополнительная линия после real footprint/overload policy; не замена admission и не RSS bound. Current engine-specific Job commit cap не задан |
| Permanent wake lock / automatic power exemption | Android APIs | **DEFER policy**, не предлагается включать автоматически. Они не устраняют carrier queue/native-call retention и могут повысить расход |

У Weighted first large waiter блокирует продвижение smaller waiters специально,
чтобы не голодал большой запрос. Это разумная fairness policy, но не latency SLO.
Retirement/control resources должны иметь собственный заранее обеспеченный reserve;
cleanup не должен ждать тот же исчерпанный payload budget, который он освобождает.
Такой reserve не разрешает обход QUIC congestion control на wire.

Ни один generic wrapper вокруг SendDatagram не может безопасно сделать
«goroutine + select timeout + Release token»: старая goroutine и frame owners
могут продолжать жить. Минимальная интеграция должна либо дать exact cancellation
в queue admission API, либо удерживать slot/envelope до actual return/terminal
frame receipt. Если upstream не экспонирует payload terminal callback, пригоден
conservative per-generation envelope до carrier retirement, **при доказанных
внутренних count/span limits**. Без них даже этот вариант ещё не sufficient.

## 8. Time, suspend/resume, peer и энергия

Нужны разные receipts `admitted`, `producer_returned`, `queue_closed`,
`raw_write_completed`, `transport_terminal`, `worker_reaped`,
`parent_sources_quiescent`. Они не взаимозаменяемы.

Если каждая stage имеет finite admission wait, service и completion deadline,
отмена реально достигает underlying operation, scheduling delay ограничен δ,
а independent reap имеет подтверждённую upper bound T_reap, то можно сложить
наибольшие времена **последовательного critical path**. Для deadline D от
admission и последующего bounded cleanup получается условный
`T_release ≤ D + T_cleanup + δ`; overlapping stages не обязаны суммироваться.
В прочитанном коде не установлены необходимые hard syscall/scheduler/device
bounds, поэтому подставлять 2/4/3/30 секунд вместо T_release нельзя.
Сохранённый slot может ограничить memory/concurrency при бесконечно долгом
retirement, но ценой unavailable admission. Это не теорема о безотказности.

| Clock/state | Прочитанный contract | Недопустимый перенос |
|---|---|---|
| HY2 transport monotime | quic-go Now → time.Since(start); current Go1.26.7 Linux-arm64 reference nanotime1 использует CLOCK_MONOTONIC | Не аттестует ещё не принятую Android AAR/ABI; CLOCK_MONOTONIC не включает suspend |
| Kotlin authority/death | Worker сравнивает SystemClock.elapsedRealtime; он включает deep sleep | Выбранные часы не заставляют ScheduledThreadPoolExecutor выполнить watchdog во сне |
| Windows reference | nanotime1 читает interrupt-time; netpoll relative SetWaitableTimer имеет отдельные low-power semantics | Считать timestamp и wake delivery одной clock guarantee |
| Defrag age10 | cache использует time.Now().Unix, lazy access/eviction и update-on-get | Это wall age, не monotonic hard elapsed reap; backward clock/lack of access меняют ожидания |
| CPU awake / FGS | Timer может выполняться при реальном scheduling/network access | Notification/FGS label не доказывает постоянно работающий CPU/радиоканал |
| Deep suspend / resume | Локальная authority уже могла истечь; peer/NAT продолжали стареть по своим часам | Transport active flag и queued PING не продлевают signed authority и не доказывают peer alive |

[SystemClock contract](https://developer.android.com/reference/android/os/SystemClock),
[Windows relative timer](https://learn.microsoft.com/en-us/windows/win32/api/synchapi/nf-synchapi-setwaitabletimer).
Current Android source watchdog проверяет elapsed deadline каждую planned1s,
а graceful retirement использует provisional3s; никакого source доказательства
«процесс гарантированно умер за3s календарного сна» из этого нет.

Pinned HY2 source задаёт nominal idle30s/keepalive10s. Реальный keepalive due
`Dka = last_rx + max(min(10s, negotiated_idle/2), 3·PTO/2)` при допустимом mode
и отсутствии keepAlivePingSent. Idle deadline использует max(idle,3·PTO) и
собственное reanchor. Поэтому последовательность такая:

`timer_due → callback_executed → PING_queued → packet_written → peer_received`.

Для NAT ещё нужны usable selected path/mapping и определённая refresh policy;
даже successful local write не attestation NAT refresh. Контрпример: due наступил,
CPU спит либо queue/raw write задержан, peer/NAT удаляет state до первого реально
дошедшего пакета. Противовес: awake device с регулярным peer traffic может вообще
не нуждаться в standalone PING и сохранять carrier. Neither branch даёт mAh.

Energy-effect **UNKNOWN** без paired device workloads, sleep residency, actual
outer writes, CPU и radio/rail observations. Сравнивать надо idle+recovery cost
и useful task completion: уменьшение keepalive может увеличить reconnect cost.
Generic pause marker тоже не доказывает достижение HY2 timer subscriber.
Не включаются power exemptions, новые transports или ручные OS изменения.

## 9. Конечная будущая acceptance matrix — всё NOT_RUN

Это спецификация наблюдений будущему platform owner; нет executable cases,
fixtures, benchmarks или инструментирования в product code.

| ID | Различающий случай | Необходимое наблюдение / критерий |
|---|---|---|
| N1 identity | Windows declared→built; Android chosen AAR/Go/ABI/overlay→APK | Exact hashes/tags/compiler/module receipt; при отсутствии identity source transfer остаётся UNKNOWN |
| N2 allocation units | Caller copy→fragments→pack→Add→Pop→history→ACK/loss | Distinct backing IDs/capacities и owners; aliases counted once; charge не исчезает при dequeue/Write; ingress тоже учтён |
| N3 session cancel | Queue full, admitted WritePacket, закрыта только UDP session | Actual producer return отдельно от Close return; retained slot до return; declared policy bounded wake/reject либо honest pending, без phantom Release |
| N4 blocked write | Full send queue и optional Hop.WriteTo внутри lock | Independent cancel/termination действительно достигает original operation; Close/deadline не считаются successful interrupt по факту их вызова |
| N5 generations | Repeated internal offers/hops и whole-worker replacement; failed reap; child died with parent RPC/source pending | Retiring slots остаются charged; no replacement сверх ceiling; Windows Job.Empty/ioDone и Android death+parent quiescence раздельны |
| N6 bounds before cost | Invalid Cfrag, F255/256, Urx above limit, session/group/handler ceilings | Отказ до allocations/grow/spawn; overload освобождает своих owners; один UDP drop не снимает protection barrier |
| N7 controller pair | Current/candidate full pair и partial mix; F0/F>0; ACK-only, DATAGRAM, PMTU/PATH, PTO/drop/retry/install boundary | Проверить actual dispatch, before/after oracle, event units и terminal callback coverage; факт compile не достаточен |
| N8 container high water | Большой PN gap, cleanup present/span=0, последующий reuse | Capacity high water/transient grow измерены отдельно; при достижении metadata budget отказ до grow, а не надежда на ACK cleanup |
| N9 clocks | Awake, suspend/resume, wall rollback, ordinary Doze/AppStandby/LPS | Same-boot clock basis; authority не продлевается; due/execute/queue/write/peer различимы; approved AAR clock реально подтверждён |
| N10 process memory | Payload load+churn+GC+JNI+kernel/socket state | Heap/capacity/commit/RSS/PSS с корректными units; unknown residual не ноль; OS cap failure не переименован в successful graceful completion |
| N11 energy | Matched idle + actual useful tasks/reconnects на selected device/network | Total idle+recovery energy и failure/latency outcomes с missingness; отсутствие rail/device data остаётся UNKNOWN |
| N12 admission/recovery | Overflow count/bytes, expired queued request, canceled native call, cleanup reserve | Bounded waiting и metadata; recheck current generation/deadline после ожиданий; reserved retirement lane; no direct fallback, no untracked retry fanout |

Для N2/N5/N8 acceptance требует сохранённого B и ceilings **до** запуска,
приращений maxima по всем phases и terminal receipts. Если остаётся один
uncounted owner или неограниченный span/waiter, whole bound не засчитывается.
Численные SLO/ceilings устанавливает owner после permissible baseline; здесь
они не выдуманы. Native/device/live исполнение требует отдельного разрешения.

## 10. Independent opposition и итог пакета

На срез 09:14:48Z готовых парных Z04/Z05 разборов не было; независимая сверка
с их global-bound claims не заявляется. В собственном оппонировании рассмотрены:

- sequential per-session writer вместо вымышленного unlimited N на одной session;
- engine40k/50k history guards и finite16-bit cache key space;
- real socket cancellation/completion и existing original-handle process reap;
- parent RPC/source ownership, которое не заканчивается вместе с child;
- исправленная candidate CCA semantics без blanket lifecycle/resource guarantee;
- awake/regular-traffic сценарий против предположения о неизбежном suspend-failure.

Все четыре work stages и оба продолжения TASK пройдены: declared/caller identity,
accounting contract, cancellation/clocks/opponents, licensed reuse и finite native
переход N1–N12. Source evidence cut09:25:57Z. Provenance:17 selected public/cache
matches,61 local input hashes; на final comparison59 неизменны после snapshot,
2 Android metadata files изменились. Их core1.13.19/Go=null/AAR=null и required
owner/cache/closed-history flavor перечитаны отдельно; product work не изменён.
Это не atomic snapshot всей рабочей копии и не overlay/artifact attestation.

**Предлагаемое решение:** принять A как достаточный условный accounting
contract, B как узкий source-scoped cancellation defect для отдельного owner
review. Научное ACCEPTED не выставлено автором.

Последовательность применения после принятия применимых выводов:

1. Зафиксировать resource units/custodians, generation/count/byte limits и
   reaping receipts; сохранить platform protection и uncertain-owner barriers.
2. Закрывать session queue cancellation и optional hopping interrupt seam в
   существующем packet owner; один controlled scoped change, без timeout-обёртки,
   которая освобождает slot до actual return. Полный sender/controller fix
   рассматривать совместимой парой с отдельной semantic matrix.
3. После явного разрешения перейти к N1 identity, затем N2–N8/N12 ownership,
   потом N9–N11 platform/elapsed/energy. Ни одна стадия не получает PASS из build.

Открытое: numerical whole-process/RSS/kernel/native bound; accepted Android
artifact/toolchain; measured time/CPU/throughput/energy и field workload;
independent paired review и scientific verdict. Приёмочная программа полностью
**NOT_RUN**, executable fixtures не создавались.
