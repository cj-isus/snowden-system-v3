<a id="quic-accounting-limits-2026-10-28"></a>
# QUIC/HY2: семантика счётчиков, пределы памяти и сроки peer

[Оглавление](../README.md) · [Основная глава](TRANSPORT-AND-RESOURCES.md) · [Решения review](../research/reviews/2026-10-28-DEEP-REVIEW.md)

**2026-10-28, DR27-05/06/07.** Публичная связка: sing-quic
`d83826c306d7c008cafb3fd6d8ee07cbcfd656ed` и quic-go
`ccd46abedd39c6f8384b1df8daf42008724958f5`. Версии/стороны приложения и реальный
peer не проверены. Арифметика ниже — ручная модель source, не замер ресурса
или воспроизведение дефекта.

## 1. Один ByteCount может означать разные моменты времени

Пусть F — bytes-in-flight до регистрации очередного учитываемого QUIC packet,
L — его сериализованный размер. В normal ack-eliciting path:

| Точка | Требуемый/наблюдаемый смысл |
|---|---|
| Engine после регистрации | F+L |
| `sentPacketHandler.SentPacket` callback в mod.4 | Уже F+L |
| `ccAdapterEx` → meta2 `bbrSender.OnPacketSent` | Значение передаётся без коррекции |
| `bandwidthSampler.OnPacketSent` | Сам добавляет L перед созданием post-send snapshot |
| Полученный snapshot | F+2L вместо F+L |

При F=0 и L=1200 snapshot равен2400, хотя engine flight1200 и sampler
`totalBytesSent` увеличен только на1200. Проверка sampler `bytesInFlight==0`
тоже не видит начало пустого flight. **Не следует**, что удвоены отправленные
bytes, bandwidth или задержка: это разные поля и guarded consumers.

Цепочка: [handler](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/internal/ackhandler/sent_packet_handler.go),
[adapter](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/internal/ackhandler/cc_adapter_ex.go),
[BBR](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/congestion_meta2/bbr_sender.go),
[sampler](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/congestion_meta2/bandwidth_sampler.go).

L включает QUIC framing/AEAD overhead, но не UDP/IP/внешнюю обёртку. DATAGRAM
учитывается этим normal path; имя callback-аргумента `isRetransmittable` не
означает retransmission его payload. ACK-only, ACK+PING, PMTU и PATH probe
имеют различные inclusion predicates. Регистрация предшествует socket write:
before/after здесь — позиции accounting, не timestamps выхода на провод.

`OnCongestionEventEx` использует **pre-event flight** до обработки ACK/loss,
хотя вызывается после обработки. Поэтому нельзя механически исправить все
callbacks одним `±L`. [RFC9002](https://www.rfc-editor.org/rfc/rfc9002.html)
определяет QUIC accounting, но не полный контракт этого Go BBR adapter.

## 2. Готовый upstream fix требует совместимой пары

Исследованный sing-quic v0.7.0
`4ab2eceaac81e073f53b22ec72ff56aaea89a3d1` использует quic-go mod.6
`f7b7e9c4c6f6ec45e4ab2b4788eca63192975d80`.
В новой паре injected controller получает сохранённый **prior** flight;
Brutal реализует Ex/monotime contract. Это source-fix, не измеренный выигрыш.

| Перенос | Граница совместимости |
|---|---|
| Новый Brutal к старому Ex | Дополнительные методы допустимы, но constructor/pacer/callers изменены |
| Старый BBR к новому Ex | Не хватает `OnPacketNeutered`; прежний compile-time assertion не соответствует новому method set |
| Новый BBR к старому sender | Совместимый тип не устраняет post→before semantic mismatch |
| Новая пара целиком | Меняются dependency graph, packet lifecycle и installation boundary; не drop-in upgrade |

Primary: [fix](https://github.com/SagerNet/sing-quic/commit/4ab2eceaac81e073f53b22ec72ff56aaea89a3d1),
[new manifest](https://github.com/SagerNet/sing-quic/blob/4ab2eceaac81e073f53b22ec72ff56aaea89a3d1/go.mod),
[new handler](https://github.com/SagerNet/quic-go/blob/f7b7e9c4c6f6ec45e4ab2b4788eca63192975d80/internal/ackhandler/sent_packet_handler.go),
[new Brutal](https://github.com/SagerNet/sing-quic/blob/4ab2eceaac81e073f53b22ec72ff56aaea89a3d1/hysteria/congestion/brutal.go).
Новый controller может временно хранить prior после send и пересчитывать
текущее значение при batch. Oracle «каждое внутреннее поле всегда равно
engine post» поэтому неверен. R04-14 требует такой уточнённой матрицы.

## 3. Payload cap не проверяет арифметику fragmentation

В HY2 `WritePacket/WriteTo` отвергают U>4096. Однако `fragUDPMessage` вычисляет:

`H = 8 + varint_length(address_length) + address_length`, `C = B − H`.

Для U>0 перед созданием списка нужны как минимум C>0 и
`F=ceil(U/C)≤255`, поскольку wire `fragmentTotal`/`fragmentID` — uint8.
Pinned функция этих gates не имеет: сначала выделяет весь список, затем
сужает counts/IDs. При положительном C список конечен, F≤4096, но может
превысить255; при C=0 цикл не продвигается, при C<0 возникает недопустимый slice.
Это условные следствия выражений, не отправленные тестовые пакеты.

`DatagramTooLargeError` возвращает sender в fragmenter с новым B.
Minimum1200 для `max_udp_payload_size` нельзя применять к отдельному
`max_datagram_frame_size`, которому RFC9221 не задаёт такого minimum.
[HY2 function/callers](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/hysteria2/packet.go),
[QUIC parameter parser](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/internal/wire/transport_parameters.go),
[RFC9221 §3](https://www.rfc-editor.org/rfc/rfc9221.html#section-3).

Противовес: обычные pinned HY2 peers объявляют большой DATAGRAM limit;
при B=1197, H=22, U=4096 нужны лишь4 fragments. Достижимость необычного
малого B, actual peer policy и последствия на поставленном artifact требуют
отдельной проверки. Установленный remote DoS этим не объявляется.

## 4. Receive, ready queue и sent history имеют разных владельцев bytes

**Receiver не симметричен sender.** В `udpDefragger.feed` lengths складываются
и `buf.NewSize(finalLength)` вызывается без повторного gate4096. Пять допустимых
частей по1000 дают5000 на assembly path. Count64 ready queue проверяется уже
после allocation/copy. При сборке одновременно удерживаются fragments и output.
`ReaderMTU()==4096` сам не является enforcement, а позднее усечение copy не
отменяет уже понесённую стоимость. [HY2 receiver](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/hysteria2/packet.go),
[allocator](https://github.com/SagerNet/sing/blob/7c349dacf402256d3a7029746073b05d2ead584a/common/buf/buffer.go).

Сильное ограничение: штатный sender сам не создаёт исходный U>4096.
Нужны отдельно условия на отправителя, groups/IDs и lifetime; общий RSS-bound
нельзя вывести из одной возможной группы. Предыдущий R04-06 о borrower lifetime
сохраняется и не считается новой находкой.

**TX byte-credit не возвращается весь при dequeue.** `SendDatagram` создаёт
копию frame.Data. Packet packing удаляет её из DATAGRAM queue, но frame остаётся
в `p.Frames` sent history. Socket write освобождает serialized packet buffer,
а не эту ссылку; нормальные ACK/loss paths освобождают её отдельно. Поэтому
`queue empty` и `Write returned` не означают отсутствие TX-retained bytes.
[Queue](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/datagram_queue.go),
[packing](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/packet_packer.go),
[history/release](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/internal/ackhandler/sent_packet_handler.go).

Практический contract: отдельно Utx/Urx/Fmax, число sessions/groups/writers,
assembly overlap, ready/borrowed/retiring и sent-history bytes. Shared slices
считать по backing allocation. Bounds живых payload не становятся измеренными
RSS/CPU/kernel/timer bounds. Обычный overload drop должен освобождать именно
своих owners; ошибка одного UDP сообщения не обязана разрушать весь carrier.

## 5. Объявленный peer timeout и локальный effective timeout различаются

Pinned parser делает `MaxIdleTimeout=max(5s, received_ms)`, в том числе для
явного0; отсутствие параметра оставляет0 в свежей структуре.
При HY2 local idle30s/keepalive10s и достаточно малом PTO:

| Peer wire value | Parsed value | Локальный idle / keepalive до PTO-floor |
|---|---|---|
| Отсутствует | 0 | 30s / 10s |
| Явный0 | 5s | 5s / 2.5s |
| 1000ms | 5s | 5s / 2.5s |
| 30000ms | 30s | 30s / 10s |

[Parser](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/internal/wire/transport_parameters.go),
[constant](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/internal/protocol/params.go),
[effective timers](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/connection.go).
[RFC9000 §18.2](https://www.rfc-editor.org/rfc/rfc9000.html#section-18.2)
задаёт default0, а [§10.1](https://www.rfc-editor.org/rfc/rfc9000.html#section-10.1)
— выбор ненулевого idle timeout и PTO floor.

Контрпример: peer действительно использует1s, PTO0.2s, keepalive выключен,
промежуток idle начинается после последних служебных пакетов. Его допустимое
удаление state может опередить локальный PING2.5s даже без сна и batching.
Это source/interoperability вопрос, не замер нынешнего сервера.

## 6. Часы, пробуждение и энергия — отдельные контракты

Время может включать suspend, а механизм ожидания не обязан будить систему.
Windows Go-reference transport clock и relative netpoll wait имеют разные
low-power semantics. Android `elapsedRealtime` включает deep sleep, Handler
использует uptime; non-wakeup alarm не делает `_WAKEUP` только из-за выбранных
часов. После resume peer мог уже удалить state, независимо от локального
`active()` или возраста таймера.
[Windows SetWaitableTimer](https://learn.microsoft.com/en-us/windows/win32/api/synchapi/nf-synchapi-setwaitabletimer),
[Go reference netpoll](https://github.com/golang/go/blob/d90b98e65320778f3b1f99a6951ab20f04d218b3/src/runtime/netpoll_windows.go),
[Android SystemClock](https://developer.android.com/reference/android/os/SystemClock).

Android NAT-T keepalive offload не generic QUIC service. RFC3948 прямо запрещает
использовать получение NAT keepalive как доказательство liveness; поддержание
mapping совместимо с потерей QUIC state.
[RFC3948 §4](https://www.rfc-editor.org/rfc/rfc3948.html#section-4),
[SocketKeepalive API](https://developer.android.com/reference/android/net/SocketKeepalive).
Quiet сокращает необязательную работу; ACK/PTO/expiry/revoke не становятся
необязательными. Бюджет scheduling оценивает feasibility, а энергию измеряют
вместе с useful work и recovery cost, не по числу настроенных таймеров.

## 7. Конечная будущая проверка — NOT_RUN

1. Exact sender/controller pair: F=0/F>0, STREAM/DATAGRAM, ACK-only/ACK+PING,
   PMTU/PATH, batch ACK+loss; разные before/after oracle.
2. Fragmentation: U4095/4096/4097, C positive/zero/negative, F255/256;
   reject до списка allocations. Receive4096/4097 и full-ready queue отдельно.
3. TX: Add→Pop→Write→ACK/loss, credits каждого владельца, Close и partial errors.
4. Peer timeout absent/0/1/5/30s, разные PTO; raw→parsed→effective→peer receipt.
5. Suspend/awake-pause, clock jump, network change, authority expiry; native
   callback delivery отдельно от локального времени.

Это адресаты TH-02/04/06, не готовые тесты и не изменение ядра. Reuse transport
scheduler/готового fix рассматривается после source compatibility и owned
resource contract; фактический performance effect остаётся неизвестным.
