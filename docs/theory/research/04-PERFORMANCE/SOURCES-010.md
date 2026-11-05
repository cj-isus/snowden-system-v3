# SOURCES R04-010 — Windows UDP, resource holders и retirement

Пакет [REPORT-010](REPORT-010.md),
[краткий разбор](handoffs/R04-010.md). Чтение: 27 октября 2026, с 13:36:03Z; первичный
source capture около 14:20Z, дополнительные sampler/timer captures около 14:30Z.
Точные времена и hashes — собственные manifests в
`.artifacts/research/theory-04/2026-10-27-win-udp-gso-res4/`.

**Статусы:** `SOURCE_REVIEW` — прочитаны названные функции/разделы локального
исходника; это не полный audit модуля. `FULL_TEXT_READ` относится к названным
разделам официального документа, не к непрочитанным разделам целого RFC.
`LINK_ONLY` не переносит содержимое работы. Наличие capture не означает, что
все строки файла были проанализированы. Ни один источник не является product,
native или benchmark evidence.

## 1. Версии, provenance и лицензии

| ID | Организация / документ / версия | Реальное основание и предел |
|---|---|---|
| P1 | Snowden, `windows/go.mod`, go.sum, packaging source; чтение 27.10 | go.mod:8 sing-box1.13.19; :139 quic-go0.59.0-sing-box-mod.4; :140 sing0.8.13; :142 sing-quic0.6.4-0.20260803041914-d83826c306d7; :17 x/sys0.44.0. Packaging: windows/amd64 + with_quic/utls/gvisor/clash_api. Version selection source, не execution |
| P2 | SagerNet sing-quic, pseudo-version `v0.6.4-0.20260803041914-d83826c306d7` | Prefix commit d83826c306d7 дан самим manifest; upstream full commit из прежнего baseline не используется как новая независимая equality. Local module bytes сняты/hash; root LICENSE1–14 GPL-3.0-or-later |
| P3 | SagerNet quic-go, tag `v0.59.0-sing-box-mod.4`, local .info Time2026-02-16T03:31:27Z | Tag/version доступны; local .info **не содержит Origin Hash**. Не придуман exact commit. LICENSE FULL_TEXT_READ: MIT, copyright quic-go authors/Google2016 |
| P4 | SagerNet sing-box, tag1.13.19 | Module-cache version + manifest; LICENSE FULL_TEXT_READ GPL-3.0-or-later, дополнительное требование naming/association consent. Не whole dependency license closure |
| P5 | SagerNet sing, tag0.8.13 | Module-cache version + manifest; LICENSE FULL_TEXT_READ GPL-3.0-or-later |
| P6 | Go Authors, локальный `C:/Program Files/Go/VERSION`: go1.26.7; time2026-08-18T21:44:21Z | VERSION FULL_TEXT_READ; net/internal/poll — source reference. LICENSE сохранён; Go BSD-style header. Локальная версия не compiler attestation EXE |

Строки go.sum читаются как expected sums выбранных модулей, не объявляются
результатом заново вычисленного Go module dirhash. Remote module byte equality
не проверялась. 76 selected source/doc files captured serially; source-before
и source-after совпали во время каждой копии. Это не атомарный checkout и не
полная неизменность во время всего исследования. Итоговый drift seal — отдельно.

## 2. Адресная карта SOURCE_REVIEW (27.10.2026)

Для каждого module-файла ниже URL строится как конкретная version/tag reference;
текст фактически прочитан **локально**, открытие всех GitHub URL не заявляется.
Сохранённые bytes и hashes дают проверяемые адресные входы вне готовых пакетов.

### Q — quic-go v0.59.0-sing-box-mod.4

Base: [SagerNet/quic-go tree/tag](https://github.com/SagerNet/quic-go/tree/v0.59.0-sing-box-mod.4).

| Файл / прочитанные символы | Какое утверждение поддерживает |
|---|---|
| `sys_conn.go:38–80,94–122` — OOBCapablePacketConn/wrapConn/basicConn | Методный scope внешнего PacketConn, scalar ReadFrom/WriteTo, GSO/ECN unsupported |
| `sys_conn_windows.go:1–14,16–42` — newConn, buffer inspect | На windows newConn тоже basicConn; packetInfo.OOB=nil |
| `sys_conn_oob.go:1,80–156` — build tag/newConn | OOB-код относится к darwin/linux/freebsd, не Windows |
| `sys_conn_helper_linux.go:64–107`, `helper_nonlinux.go` | Linux UDP_SEGMENT probe/cmsg/GSO failure gates, не Windows-USO |
| `sys_conn_buffers.go:14–68`, generated write counterpart | Desired buffer request ≠ проверенный hard cap; setters зависят от method set |
| `send_conn.go:73–117` — Write/writePacket/WriteTo/capabilities | actual rawConn delegation; write return не peer receipt |
| `send_queue.go:35–114` — newSendQueue/Send/Run/Close | channel8, worker1, drain-before-runStopped, failure branches |
| `buffer_pool.go:9–91`; `internal/protocol/protocol.go:110–117` | Ordinary buffer1452 и large20KiB, reference release/pool ≠ RSS reclamation |
| `packet_packer.go` — ordinary coalesced/PTO/ACK/MTU builders, AppendPacket:476–503, appendShortHeaderPacket:921–1018 | Valid encoded packet size и отличие ordinary vs GSO allocation |
| `connection.go:2459–2605,2682–2745,2881–2896` | SendMode→WithoutGSO, PTO QueueProbePacket, ordinary sizes/caller |
| `connection.go:3040–3065`; `datagram_queue.go:12–137` | Send payload copied before Add32 wait; receive copies before queue128 check |
| `connection.go:853–918,1716–1717,2428–2433,2778–2779` | Exact idle: max(negotiated idle,3PTO), receive/first ack-eliciting send origin |
| `connection.go:571–598,751–762,2185–2198,2265–2268` | ctxCancel after Run queue drain; CloseWithError waits connection ctx |
| `internal/utils/rtt_stats.go:18,63–77`; `ackhandler/sent_packet_handler.go:665–711` | BasePTO includes RTT/variance/ackdelay;60s upper clamp not minimum interval |
| `ackhandler/sent_packet_handler.go:1022–1095`; `protocol/params.go:65–73` | 40000/50000 tracked guards; PTO QueueProbePacket performs DeclareLost |
| `ackhandler/sent_packet_history.go:13–253`, packet.go | History pointer/frames/holes distinct from payload and flow-control; reslice and pool scopes |
| `internal/flowcontrol/connection_flow_controller.go:19–78` | ReceiveWindow vs sendWindow distinction; data consumption/windows not ACK equivalence |
| `server.go:266–274,743–802,856–903` | Positive guards: queues32/1024, retry/callback rejection; queue32 after handshake not cap all active connections |
| `internal/protocol/params.go:45–60`; `connection.go:1964–1989,1010–1054` | queue scopes and count256; no whole RAM bound |

### H — sing-quic v0.6.4-0.20260803041914-d83826c306d7

Base: [SagerNet/sing-quic d83826c306d7](https://github.com/SagerNet/sing-quic/tree/d83826c306d7).

| Файл / символы | Что поддерживает |
|---|---|
| `hysteria2/client.go:72–85,112–291,341–410` | QuicConfig receive windows, offerNew wrapping, shared offer and discarded completion, CloseWithError/rawClose ordering |
| `quic.go:65–85` | CreateTransport custom vs STD branch; read source, no crypto execution |
| `hysteria2/salamander.go:15–109` | Scalar salt8 transform and separate vectorised factory; exact envelope boundary |
| `hysteria2/client_packet.go:5–47` | ReceiveDatagram→goroutine per message; cap upstream queue != all handlers |
| `hysteria2/packet.go:133–144,180–302,330–398` | Session channel64, pre-close checks, SendDatagram, context cancellation not queue/defragger drain |
| `hysteria/hop.go:17–206` | Optional hopping queue1024, buffer2048, current/prev sockets; caller-specific scope |
| `hysteria/protocol.go:14–21` | Exact8MiB/20MiB receive windows;30s idle config/10s keepalive config |
| `hysteria2/service.go` — config/auth portions | Used as baseline scope; absence-of-admission proof not inferred from grep |
| `congestion_meta2/packet_number_indexed_queue.go` — Emplace/Remove/RemoveUpTo/EntrySlotsUsed | No generic hard slot cap; slots grow with PN span, two far keys hypothetical container misuse ≠ actual peer exploit |
| `congestion_meta2/bandwidth_sampler.go:677–703`, `bbr_sender.go:515–527`; Q `cc_adapter_ex.go:47–73` | ACK/loss/sampler retirement separate; estimate leastUnacked in sender, no exact numeric sampler-memory theorem in this packet |

### S — sing0.8.13 и B — sing-box1.13.19

[SagerNet/sing tag](https://github.com/SagerNet/sing/tree/v0.8.13),
[sing-box tag](https://github.com/SagerNet/sing-box/tree/v1.13.19).

- S `common/bufio/bind.go:58–112`, `conn.go:188–239`,
  `network/conn.go:68–72`: точный набор методов UnbindPacketConn/ExtendedConn.
- S `bufio/vectorised.go:65–77`, `vectorised_windows.go:45–98`:
  factory не рекурсивна; Windows WSABUF scatter/gather использует
  WSASendTo с NULL-overlapped.
- S `buf/buffer.go:15–62`, `alloc.go:18–86`, `pool.go`:
  запрошенный логический размер и фактическое power-of-two выделение
  различаются (1452+8 → backing2048); важно владение sync.Pool.
- B `protocol/hysteria2/outbound.go:42–120`, `common/dialer/dialer.go:39–56`,
  `default.go:246–318,388–399`, `route/conn.go:72–92,383–427`:
  фактические UDP dialer/trackedConn, InterfaceUpdated/Close и отсутствие
  дополнительного продвижения методов обёрткой.

### W — исходники Windows приложения (не изменены)

- `windows/backend/render/render.go:775–817`: outbound shape и optional hopping;
  источник, не real profile.
- `windows/corebridge.go:465–504`, `backend/core/engine.go:72–135,410`:
  NewIsolatedEngine, previous.Stop before replacement, worker box composition.
- `worker_child.go:61–102`: cooperative stop executes Engine.Stop.
- `worker_process.go:138–214,269–290`: terminate/wait/ioDone/Empty and retained
  errors;2s/4s constants from `worker_protocol.go:13–25`.
- `worker_host_windows.go:62–129,142–149`, `worker_spawn_windows.go:54–78`:
  native Job/Wait wrappers; processes8; engine commitBytes0; conditional memory
  limit branch. Эти syscalls только прочитаны, не вызваны.

### G — Go1.26.7 local source reference

- `src/net/net.go:192–224`: унаследованные connected Read/Write/Close.
- `src/net/udpsock.go:154–160,232–244`, `udpsock_posix.go:5,46–69,113–176`:
  Windows включён в файл с posix в названии; обычные альтернативы ReadFrom/WriteTo.
- `src/internal/poll/fd_windows.go:239–346,558–612,830–879,983–1108`:
  WSARecv/WSASend, WSARecvFrom/WSASendto, OVERLAPPED execIO,
  CancelIoEx+waitCanceled; выделение живёт до завершения/отмены.
- `src/net/rawconn.go:52–61`: делегат RawWrite альтернативного
  SyscallVectorisedPacketWriter; это не рассмотренный скалярный путь.

## 3. Внешние официальные документы, реально прочитанные разделы

| ID | Организация, название, дата | URL / section / чтение и статус |
|---|---|---|
| M1 | Microsoft, «Функция WSASendTo (winsock2.h)»; ms.date2018-12-05, updated2025-03-01 | [ru-RU](https://learn.microsoft.com/ru-ru/windows/win32/api/winsock2/nf-winsock2-wsasendto); Parameters/Remarks/Overlapped I/O/FUNCTION requirements; FULL_TEXT_READ27.10 около 13:48–13:51Z |
| M2 | Microsoft, «IPPROTO_UDP socket options»; ms.date2019-10-02, updated2025-03-11 | [en-US](https://learn.microsoft.com/en-us/windows/win32/winsock/ipproto-udp-socket-options); вся таблица UDP_SEND_MSG_SIZE/UDP_RECV_MAX_COALESCED_SIZE, defaults0 и UDP_COALESCED_INFO; FULL_TEXT_READ27.10 около 13:48–13:51Z |
| M3 | Microsoft, «Функция WSASend (winsock2.h)»; ms.date2018-12-05, updated2025-03-01 | [ru-RU](https://learn.microsoft.com/ru-ru/windows/win32/api/winsock2/nf-winsock2-wsasend); Parameters/Remarks/Overlapped I/O; FULL_TEXT_READ27.10 около 14:28Z |
| M4 | Microsoft Windows SDK, «WSASetUdpSendMessageSize»; ms.date2019-10-01, updated2026-10-04 | [ru-RU](https://learn.microsoft.com/ru-ru/windows/win32/api/ws2tcpip/nf-ws2tcpip-wsasetudpsendmessagesize); весь текст, Requirements build19041; FULL_TEXT_READ27.10 около 14:30Z |
| M5 | Microsoft Windows SDK, «WSASetUdpRecvMaxCoalescedSize»; ms.date2019-10-01, updated2025-03-01 | [ru-RU](https://learn.microsoft.com/ru-ru/windows/win32/api/ws2tcpip/nf-ws2tcpip-wsasetudprecvmaxcoalescedsize); весь текст, Requirements build19041; FULL_TEXT_READ27.10 около 14:30Z |
| R1 | IETF; T. Pauly, E. Kinnear, D. Schinazi, «An Unreliable Datagram Extension to QUIC», RFC9221, March2022, Standards Track | [official text](https://www.rfc-editor.org/rfc/rfc9221.txt); §5 behavior, §5.2 acknowledgements, §5.3 flow control, §5.4 congestion control, §6 security; FULL_TEXT_READ этих разделов 27.10 около 14:22Z; bytes сохранены |
| R2 | IETF; J. Iyengar, M. Thomson (eds.), «QUIC: A UDP-Based Multiplexed and Secure Transport», RFC9000, May2021, Standards Track | [official text](https://www.rfc-editor.org/rfc/rfc9000.txt); §10.1/§10.1.2/§10.2–10.2.2; FULL_TEXT_READ этих разделов 27.10 около 14:23Z; bytes сохранены |

ru-RU pages Microsoft реально вернули русский текст с locale/contentlocale
ru-ru, но translationtype MT: машинный перевод, не самостоятельное российское
исследование. Содержание option-table проверено также по canonical en-US.
Русская локаль в URL не используется как доказательство до получения текста.

### Ошибки получения и ограничения

WebFetch RFC Editor HTML/text и datatracker несколько раз завершился timeout;
IETF txt перенаправил на RFC Editor. Эти запросы — `FETCH_FAILED`/redirect, не
прочитанные источники. Гипотеза получения изменена: curl official TXT с
TLS-validation, --fail, --max-time20, без `-k`, успешно сохранил непустые файлы.
Файлы реально прочитаны через Read, а не оценены по ссылке/аннотации.

[R04-009 paper link](https://censoredplanet.org/papers/congestion.pdf) —
**LINK_ONLY** в этом пакете; baseline не даёт корректного impossibility theorem.
Мы не переносим ни его результаты, ни attribution цензуры, ни «CCA-трилемму».
Full-text paper/venue/эксперименты требуют отдельного просмотра перед цитированием.
R04-007 upstream v0.7.0 coupling — baseline pointer, не новая remote verification.

## 4. Поиск, воспроизводимость и пределы

Scoped negative search setters: Windows backend, sing-box common/dialer,
sing common/control, local Go net: `UDP_SEND_MSG_SIZE`,
`UDP_RECV_MAX_COALESCED_SIZE`, WSASetUdp* не найдены. В отличие от отрицательного
поиска, Windows basicConn-branch доказана положительным caller/source чтением.
Поиск не охватывает все внешние ОС/драйвер/провайдер настройки.

Статическая сверка own Markdown paths/hashes допустима и не называется
product-test. Все native execution, fixtures, race/build, benchmarks,
UDP/QUIC probes, load-test, packet capture, config/OS changes — **NOT_RUN**.
Не читаются secret refs values, .env, vault, real saved profiles и raw user logs.
