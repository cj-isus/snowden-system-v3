# Ресурсы QUIC/HY2: учёт, владение и отмена

**Срез 28 октября 2026, 09:25:57Z. Статус автора — READY_FOR_REVIEW.**
Пакет: [REPORT](REPORT.md), [SOURCES](SOURCES.md).

## Что нового и какой verdict предлагается

### R28-C04-A — достаточный accounting/lifetime contract

**Claim:** global charge до clone/grow/spawn, bounded count waiting/active/retiring,
ownership transfers без phantom Release, metadata/pool/span ceilings и exact
return/reap receipts позволяют ограничить *охваченные backing capacities*.
Это не текущий численный process RSS/CPU/time/energy cap.

Новая полезная дельта: единицы allocations/aliases и схема custodian между
caller→pack→DATAGRAM queue→history→serialized write→retirement; отдельный
metadata high water. Фрагменты ссылаются на original, serialized output —
другая allocation. Ручной valid cut U4096/H22/F4 удерживает не менее 9735 bytes
названных backings до окончания операции; ни simulation, ни wire/RSS measurement.
Sampler present/span/backing различаются; doubling/Clear не даёт shrink.

**Strongest opposition:** actual route имеет sequential writer per session,
engine history40k/50k guards и finite cache key space. Existing Windows/Android
worker owners запрещают забыть uncertain process. Всё это сохранено; A требует
internal QUIC-generation/session/handler admission отдельно от worker slot.

**Recommendation:** ACCEPT_SCOPED_CONTRACT как sufficient conditional design;
не присваивать product PASS и не задавать RAM из длины queues.

### R28-C04-B — отмена UDP session не доходит до уже ожидающего Add

**Exact source:** public/current HY2 `packet.go:180–319`,
`client.go:309–408`; quic-go `connection.go:3040–3057` и datagram_queue.Add.
Session Close отменяет свой ctx, но Add ждёт queue.sent/closed; WriteTo/WritePacket
не получают session cancellation внутри wait. Returned type — net.PacketConn,
документированный Close должен разблокировать pending WriteTo с ошибкой.

**Дополнительный continuation trace:** optional `hysteria/hop.go:178–205,233–239`
держит тот же mutex на raw WriteTo, Close и SetWriteDeadline. Независимая отмена
через эту же обёртку не доказана; простая перестановка вызовов Close недостаточна.

**Assumptions/opposition:** queue/full и отсутствие drain/write completion в
рассматриваемом интервале; это conditional source trace, не reproduced hang.
Pop/whole-carrier queue close и kernel I/O completion реально могут завершить
ожидание. Windows owned Job/reap и Android death+parent-source barriers сильнее
library Close; их нельзя вырезать ради быстрого reconnect. Нет доказанного
бесконечного числа whole-worker generations или deployed remote DoS.

**Recommendation:** ACCEPT_SOURCE_SCOPED для отдельного platform/transport owner
review. Минимальный future slice: bounded shared-carrier producer/handler
admission и retained slots; actual queue/session cancel и interrupt owner без
held data-plane mutex. Не «goroutine+timeout+Release».

## Что можно переиспользовать

- Existing platform retained owners/reap — REUSE.
- x/sync0.20.0, BSD-3-Clause — REUSE semaphore/errgroup primitives, не полную
  guarantee. Waiter list/внешние callers SetLimit не capped, Wait не preempt I/O;
  нужен count до byte Acquire, checked positive cost, exact join/release.
- Matched sing-quic0.7.0 `4ab2ece…` GPL-3+ / quic-go mod.6 `f7b7e9c…` MIT —
  REFERENCE/ADAPT semantics. Prior-flight/method set/PN installation/lifecycle
  меняются вместе; это не blind upgrade core1.13.19. PTO callback coverage не
  доказано существованием OnPacketNeutered; new QUIC Add остаётся contextless.
- Go soft target / Job commit cap / generic pause/wake — не RSS/time/energy
  proof. Android actual AAR/Go unknown; authority elapsed и transport timer
  clocks/event delivery разведены. Никаких battery-saving процентов.

<a id="сдача-по-work-stages"></a>
## Область выводов и незавершённые проверки

1. Actual declarations/wrapper/caller availability — пройдено; Windows source
   доступен, Android artifact не аттестован.
2. Holder graph/условные capacities/count/metadata/generations — пройдено.
3. Cancellation/Close/drain/reap/clocks/strong opponents — пройдено; оба
   continuation TASK (повторные generations/raw write и clock portability) пройдены.
4. Licensed reusable primitives/CCA compatibility и future matrix N1–N12 — пройдено.

Z04/Z05 TASK прочитаны; готовых paired HANDOFF в review09:14:48Z не было.
Никакой global-bound claim им не приписан; independent input остаётся ожидаемым,
без polling/messages.34 public provenance records,17 current public/cache matches,
61 local input hashes.59 input hashes stable после snapshot;2 Android metadata
files moved, declared pin/unknown artifact/required cache+history flavor перечитаны.

Открыты научное решение в названной области, численные пределы всего
процесса/native/ресурсов, идентичность артефакта, фактическое ограниченное
время I/O/reap и измерения памяти/энергии/скорости. N1–N12 — NOT_RUN.

<a id="r28-c04--finite-handoff"></a>
