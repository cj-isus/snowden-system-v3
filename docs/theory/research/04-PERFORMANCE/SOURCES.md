# Источники R04-001

Дата доступа: **2026-10-25**. Статус пакета: READY_FOR_REVIEW.
Применимость выводов и контрпримеры — [REPORT](REPORT.md).

## 1. Закреплённые зависимости

Origin получен из публичного Go module proxy, не из предположения по имени tag.
Это provenance source, не подпись сборки и не проверенный Android dependency graph.

| Module | Версия Windows graph / кандидат Android | Commit | Лицензия прочитанного source |
|---|---|---|---|
| SagerNet/sing-box | v1.13.19 | `b5ebaa1fc0f2b94256180b95468e73ef53caa27d` | GPL-3.0-or-later |
| SagerNet/sing-quic | v0.6.4-0.20260803041914-d83826c306d7 | `d83826c306d7c008cafb3fd6d8ee07cbcfd656ed` | GPL-3.0-or-later |
| SagerNet/quic-go | v0.59.0-sing-box-mod.4 | `ccd46abedd39c6f8384b1df8daf42008724958f5` | MIT |
| SagerNet/sing | v0.8.13 | `7c349dacf402256d3a7029746073b05d2ead584a` | GPL-3.0-or-later |
| SagerNet/sing-tun | v0.8.12-0.20260810140523-7c73233bd0fb | `7c73233bd0fb0dae02e76505af888ef729319be9` | GPL-3.0-or-later |
| golang.org/x/sync | v0.20.0 | `ec11c4a93de22cde2abe2bf74d70791033c2464c` | BSD-3-Clause |

Origin URLs: [sing-box](https://proxy.golang.org/github.com/sagernet/sing-box/@v/v1.13.19.info),
[sing-quic](https://proxy.golang.org/github.com/sagernet/sing-quic/@v/v0.6.4-0.20260803041914-d83826c306d7.info),
[quic-go](https://proxy.golang.org/github.com/sagernet/quic-go/@v/v0.59.0-sing-box-mod.4.info),
[sing](https://proxy.golang.org/github.com/sagernet/sing/@v/v0.8.13.info),
[sing-tun](https://proxy.golang.org/github.com/sagernet/sing-tun/@v/v0.8.12-0.20260810140523-7c73233bd0fb.info),
[x/sync](https://proxy.golang.org/golang.org/x/sync/@v/v0.20.0.info).

Все шесть LICENSE захвачены из module cache и сопоставлены с raw по commit.
Полный corresponding-source/SBOM/дистрибуционный review будущего whole artifact
этим не выполнен. GPL source adaptation не представляется независимым permissive SDK.

## 2. Первичный код: утверждение → точка проверки

| ID | Первичный source / символ | Что устанавливает |
|---|---|---|
| U01 | [sing-tun TCPForwarder](https://github.com/SagerNet/sing-tun/blob/7c73233bd0fb0dae02e76505af888ef729319be9/stack_gvisor_tcp.go), [gLazyConn](https://github.com/SagerNet/sing-tun/blob/7c73233bd0fb0dae02e76505af888ef729319be9/stack_gvisor_lazy.go) | TCP flow forwarding и lazy endpoint; не raw inner TCP-over-stream |
| U02 | [sing-box route/conn.go](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/route/conn.go), NewConnection/kickWriteHandshake; [sing handshake](https://github.com/SagerNet/sing/blob/7c349dacf402256d3a7029746073b05d2ead584a/common/network/handshake.go) | Порядок outbound/local acceptance/первой записи; уже есть Write(nil) |
| U03 | [HY2 outbound](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/protocol/hysteria2/outbound.go), [HY2 client](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/hysteria2/client.go), [qtls](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/quic.go) | offer/auth перед stream, lazy target request/response, carrier reuse и Close при InterfaceUpdated |
| U04 | [HY2 service](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/hysteria2/service.go), handleStream/serverConn | Модель target handoff/handshake; не версия действующего VPS |
| U05 | [HY2 packet.go](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/hysteria2/packet.go), headerSize/fragUDPMessage/newUDPPacketConn/WriteTo/writePacket/newUDPDefragger/feed | 1197, повторяемый address header, fragmentation/reassembly/channel64/lazy expiry |
| U06 | [HY2 protocol/proxy.go](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/hysteria2/internal/protocol/proxy.go) | Исходящий MaxUDPSize4096; не глобальный UDP стандарт |
| U07 | [quic-go connection.go](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/connection.go), SendDatagram/handleAckFrame/estimateMaxPayloadSize | Queue admission/copy и разные единицы текущей MTU estimate |
| U08 | [packet_packer.go](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/packet_packer.go), maybeGetShortHeaderPacket/composeNextPacket | Реальный packet payload budget и снятие не поместившегося DATAGRAM |
| U09 | [mtu_discoverer.go](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/mtu_discoverer.go), CurrentSize/GetPing/OnAcked; [wire frame](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/internal/wire/datagram_frame.go) | Probe packet size против frame payload size |
| U10 | [QUIC config](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/config.go), [protocol params](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/internal/protocol/params.go), [queue](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/datagram_queue.go) | Default InitialPacketSize1280; queue32/128 |
| U11 | [HY2 client_packet.go](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/hysteria2/client_packet.go), loopMessages/handleUDPMessage | Отдельная goroutine на receive datagram до per-session channel |
| U12 | [sing LRU](https://github.com/SagerNet/sing/blob/7c349dacf402256d3a7029746073b05d2ead584a/common/cache/lrucache.go), LoadOrStore/maybeDeleteOldest/WithSize/WithEvict | Ленивый sliding expiry; нет автоматического janitor; строительные блоки caps |
| U13 | [x/sync semaphore](https://github.com/golang/sync/blob/ec11c4a93de22cde2abe2bf74d70791033c2464c/semaphore/semaphore.go), [versioned API](https://pkg.go.dev/golang.org/x/sync@v0.20.0/semaphore) | Nonblocking TryAcquire, фактический Release; вариант bounded admission без новой зависимости |

U01…13 являются первоисточниками кода, не сравнительным performance evidence.
Не каждый файл повторно загружался отдельно: полный список 18 byte comparisons
и остальных module-cache captures хранится в manifest, а не подразумевается таблицей.

## 3. Проектные входы

Пути относительно корня `D:/SNOWDEN-SYSTEM-V3`; точные SHA256 — source-manifest.

| ID | Файлы | Роль в выводе |
|---|---|---|
| P01 | windows/go.mod | Реальные версии Windows; никакого автоматического переноса на AAR |
| P02 | windows/backend/render/render.go | TUN/gvisor opt-in, mixed inbound, параметры carrier |
| P03 | windows/backend/core/engine.go, worker_child.go, worker_protocol.go | Изолированный box и control IPC; caps не измеренные SLO |
| P04 | windows/backend/core/manager.go, probe.go | Start order, fanout/gate и teardown HTTP idle pool |
| P05 | android/native/README.md, source-lock.json | Source candidate/strict-owner adaptation, отсутствие approved binary |
| P06 | android/corebridge/.../NativeRuntime.kt, NativeWorkerSession.kt, AndroidTunFactory.kt, LibboxPlatform.kt, NativeCallbackGate.kt | JNI/OS/TUN boundary; callback32; setup cost не тождествен packet cost |
| P07 | android/corebridge/.../configuration/NativeConfigurationComposer.kt | Explicit stack/MTU, native fragment candidate без signed authority |
| P08 | docs/CENSORSHIP-THEORY.md, docs/STATE.md, android/STATE.md | Baseline/novelty и status snapshot, не эксперимент |

`...` в таблице P06/P07 означает `src/main/kotlin/org/snowden/android/corebridge/`
внутри соответствующего module; manifest содержит полные пути.
Для Android применимость core-выводов условна до построения/приёмки exact whole
artifact. Работающий сервер и его config не читались/не проверялись.

## 4. Нормативные первоисточники

- **N01:** [RFC9000 §14/14.1](https://www.rfc-editor.org/rfc/rfc9000.html#section-14):
  различение UDP datagram/PMTU, minimum Initial UDP payload1200, path scope.
  Не обосновывает inner MTU ниже IPv6 minimum и не даёт HY2-specific overhead.
- **N02:** [RFC9221 §5](https://www.rfc-editor.org/rfc/rfc9221.html#section-5):
  QUIC DATAGRAM message semantics, предел размера, shared congestion и отсутствие
  гарантии application processing; expiration API — возможность, не текущий HY2 API.
- **N03:** [HY2 protocol](https://v2.hysteria.network/docs/developers/Protocol/):
  TCP/UDP relay и assembly fragments. Это current документ: Gecko/другие новые
  возможности не перенесены на pin d83826c. Arithmetic берётся из закреплённого кода.
- **N04:** [Android VPN guide](https://developer.android.com/develop/connectivity/vpn):
  TUN fd и protect boundary. Не доказывает gVisor/JNI/offload возможности нашей сборки.

Дополнительно просмотрены RFC9002 и RFC9312; новых самостоятельных выводов
из них в этом цикле нет. Поиск issues по точным currentMTUEstimate/udpMTU строкам
не дал применимого подтверждения готового upstream fix. Это ограниченный negative
search, не доказательство отсутствия fix во всех ветках/релизах.

## 5. Reuse и отклонённые сокращения

| Вариант | Решение | Основание / что ещё требуется |
|---|---|---|
| Existing sing-box route/HY2 pooling/fragmenter | Reuse, узкие адаптации только после gates | Уже в dependency graph; сохранять wire/ownership и GPL provenance |
| x/sync Weighted / Go bounded channel | Adapt для pre-spawn admission | Уже доступно, BSD-3-Clause для x/sync; TryAcquire до fanout, release после actual completion, bytes и count отдельно |
| sing cache WithSize/WithEvict | Building block, не готовый fix | Same pin GPL; lifetime, lock-order, concurrent feed/eviction и глобальные bytes требуют отдельного решения |
| Blind cap increase / SendDatagram nil как receipt | Reject | Queue admission и packet fit имеют разные границы; потом нужна delivery проверка |
| GOMEMLIMIT как full RSS cap, timer10s как reclamation SLA | Reject | Разные уровни ресурсов/времени; исходники cache и baseline §13.10 |
| Новый transport/fragmenter/QUIC engine, массовый upgrade | Не требуется этим исследованием | Нет доказанного преимущества, добавит compatibility/maintenance объём |

## 6. Ключевые byte fingerprints

Для пяти файлов, непосредственно определяющих новые ограничения:

| Файл | SHA256 совпавшего local/raw source |
|---|---|
| HY2 packet.go | `891f9a732dd0b16f8feb429efdbde39e4cab5f17ef27b0b836280a2fa33495e3` |
| HY2 client_packet.go | `3fc781ad2afca7b87696a0127db73c3dc22b3aea8f14c757da2eb4b9303c7c2f` |
| quic-go connection.go | `64af8a0617c8d519818c2371b7a68b476a547b89b4fc82ed6b21417b4fd177e5` |
| quic-go packet_packer.go | `8f3e0ad60d8ed724447adc7c049b11c45c56e46667341650b20ee6ff9dd4d50f` |
| sing common/cache/lrucache.go | `a4ca5793a961fd9b7cdc356c66a4c54f2b75cf4df7ecded39035a4d26ed2b77a` |

47 captures, 18 raw comparisons и 6 Origin metadata сохраняют область ручного
review. Они не заменяют native fault/resource matrix из REPORT §8.


<a id="r04-002-sources"></a>

## 7. R04-002 — источники обслуживания и lifetime

2026-10-25. Отдельный новый срез по DEPTH-CYCLE-002; прежние §1–6 сохранены.
Ссылки U01…U13 первого цикла остаются историческими входами R04-001.

### 7.1. Exact pins и границы provenance

| Package / версия | Commit | Лицензия captured source |
|---|---|---|
| SagerNet/quic-go v0.59.0-sing-box-mod.4 | `ccd46abedd39c6f8384b1df8daf42008724958f5` | MIT |
| SagerNet/sing-quic v0.6.4-0.20260803041914-d83826c306d7 | `d83826c306d7c008cafb3fd6d8ee07cbcfd656ed` | GPL-3.0-or-later |
| SagerNet/sing v0.8.13 | `7c349dacf402256d3a7029746073b05d2ead584a` | GPL-3.0-or-later |
| golang.org/x/sync v0.20.0 | `ec11c4a93de22cde2abe2bf74d70791033c2464c` | BSD-3-Clause |

Четыре Go proxy Origin records повторно получены по version URLs из §1.
Windows go.mod сохраняет эти pins и sing-box1.13.19. От этого не следует такой же
transitive graph ещё не поставленного Android whole AAR. Shared crypto/transport
реализация не менялась, dependencies не импортировались/не обновлялись.

Capture root: `.artifacts/research/theory-04/2026-10-25-hy2-scheduler-v2/`.
`source-manifest.json`:45 записей =31 module source/license +11 project inputs
+3 preserved prior documents. `upstream-comparison.json`:17 совпадений raw/cache
bytes, включая все четыре LICENSE. Не все31 module files отдельно fetched:
список raw comparisons точный, без подразумеваемой проверки всего module.

### 7.2. Actual service/ownership pointers

| ID | Точный primary path / caller → consumer | Проверяемый факт |
|---|---|---|
| D2-U01 | [connection.go](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/connection.go), triggerSending→sendPackets→appendOneShortHeaderPacket; [sent_packet_handler.go](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/internal/ackhandler/sent_packet_handler.go), SendMode | Shared CC/pacing/history, отдельные PTO/ACK modes; отсутствие per-HY2-session service reservation |
| D2-U02 | [packet_packer.go](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/packet_packer.go), maybeGetShortHeaderPacket→composeNextPacket | ACK/DATAGRAM/control retransmissions/framer порядок и residual bytes |
| D2-U03 | [framer.go](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/framer.go), Append→getNextStreamFrame; [params.go](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/internal/protocol/params.go) | STREAM gate128, RR active IDs, control API guard и direct Append paths вне него; path-response cap |
| D2-U04 | [send_stream.go](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/send_stream.go), Write/popStreamFrame/OnAcked/OnLost; [stream_flow_controller.go](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/internal/flowcontrol/stream_flow_controller.go) | Состояния reliable bytes, per-stream retransmissions, min(stream,conn) credit |
| D2-U05 | [datagram_queue.go](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/datagram_queue.go), Add/Pop/HandleDatagramFrame/Receive | Count32/128, внешние blocked producers, dequeue notification, allocation-before-cap |
| D2-U06 | [send_queue.go](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/send_queue.go), Send/Run/SendProbe/Close; [buffer_pool.go](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/buffer_pool.go); [protocol.go](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/internal/protocol/protocol.go) |8 queued entries, buffer capacities1452/20KiB, actual Write/Release; GSO entry не один packet |
| D2-U07 | [HY2 client_packet.go](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/hysteria2/client_packet.go), loopMessages→handleMessage→handleUDPMessage→inputPacket | Реальная concurrent client ветвь и fatal handle error для всего carrier |
| D2-U08 | [HY2 service_packet.go](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/hysteria2/service_packet.go), loopMessages→handleMessage, new session handler; [service.go](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/hysteria2/service.go) | Serial per-connection server receive, session fanout, declared stream ceiling2^60 |
| D2-U09 | [HY2 packet.go](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/hysteria2/packet.go), headerSize/fragUDPMessage/writePacket/inputPacket/feed/newUDPDefragger/releaseMessage/Close | Вся source цепь от legal1175 payload к1197 message и от async feed к eviction/ready64 |
| D2-U10 | [LRU](https://github.com/SagerNet/sing/blob/7c349dacf402256d3a7029746073b05d2ead584a/common/cache/lrucache.go), LoadOrStore→maybeDeleteOldest→deleteElement→onEvict | Cache mutex и synchronous eviction callback; item.access не приобретается библиотекой |
| D2-U11 | [buf.Buffer](https://github.com/SagerNet/sing/blob/7c349dacf402256d3a7029746073b05d2ead584a/common/buf/buffer.go), Release/Len | Unmanaged Release может ничего не делать; logical message reset устанавливается отдельно HY2 release |
| D2-U12 | [Weighted semaphore](https://github.com/golang/sync/blob/ec11c4a93de22cde2abe2bf74d70791033c2464c/semaphore/semaphore.go), TryAcquire/Release | Ready nonblocking admission building block; отсутствие автоматической передачи credit в retained cache |

Дополнительные captured files: receive_stream, streams_map, retransmission_queue,
base/connection flow controllers, mtu_discoverer, HY2 client/protocol/packet_wait.
Они использованы для проверки альтернативных branches и границ, не для заявления
о полном review всех транспортов/реализаций QUIC.

### 7.3. Первичные исследования и стандарты

| ID / дата документа | Прямой источник | Что применено / предел |
|---|---|---|
| D2-N01 / март2022 | [RFC9221 §5](https://www.rfc-editor.org/rfc/rfc9221.html#section-5) | DATAGRAM shared CC, отсутствие stream FC и application receipt из transport ACK |
| D2-N02 / июнь1996 | [Shreedhar/Varghese, IEEE/ACM TON4(3),375–385](https://web.stanford.edu/class/ee384x/EE384X/papers/DRR.pdf) | DRR byte deficit/throughput fairness; базовый алгоритм не обещает latency bound. Не путать journal paper1996 с technical report1994 |
| D2-N03 / январь2018 | [RFC8290](https://www.rfc-editor.org/rfc/rfc8290.html), особенно §6.2 | Experimental FQ-CoDel, modified DRR/sparse queues; opaque encapsulation collapse внутренние flows |
| D2-N04 / январь2018 | [RFC8289 §4.3](https://www.rfc-editor.org/rfc/rfc8289.html#section-4.3) | Experimental CoDel persistent-sojourn control; TARGET не hard worst-case bound и не универсальная настройка нашей QUIC queue |
| D2-N05 / сентябрь1997 | [RFC2212](https://www.rfc-editor.org/rfc/rfc2212.html) | Для delay guarantee нужны полисируемый arrival/service contract; его IntServ model не принят как существующая поддержка VPS/оператора |
| D2-N06 / original LNCS2050; current author edition accessed2026-10-25 | [Le Boudec/Thiran author page](https://leboudec.github.io/netcal/), [author source repository](https://github.com/leboudec/netcal/) | Arrival/service curves как метод; формулы REPORT10.6 выведены там вручную под явными упрощёнными условиями |

У DRR статья и алгоритм не являются готовой лицензией на заимствование любой
найденной реализации. RFC code components имеют свои IETF Trust условия;
конкретный Linux qdisc/Go scheduler code в этом цикле не импортирован и его pin
не выбран. Author book README указывает Attribution-NonCommercial-Sharealike
для TeX/PDF: в продукт не переносились ни текст, ни изображения, ни TeX assets;
используется математическая ссылка и собственный derivation. Сравнение разрешений
конкретной будущей реализации остаётся отдельным reuse gate.

Авторская страница и current web docs — dated sources, а не immutable code pins.
При review математический derivation проверяется непосредственно по указанным
предпосылкам; отсутствие service lower bound не устраняется программой расчёта.

### 7.4. Актуальные платформенные входы и неизменность старого пакета

Windows: go.mod, render.go, core/probe.go, docs/STATE snapshot V2-162.
Android: STATE snapshot AND-11-S2, native/source-lock.json, NativeCallbackGate,
NativeWorkerSession и WorkerRpcLane. В двух фиксированных parent RPC lanes
CONTROL/RETIREMENT по одному executor; NativeCallbackGate32 — отдельный callback
учёт. Это source candidates без whole AAR/compile/device proof.

Старые REPORT/SOURCES/handoff R04-001 сохранены до append в prior-packet/.
Финальная сверка `source-drift.json` отдельно различает изменившийся source,
неизменные captured bytes и сохранность original prefixes старых документов.
Смена STATE/платформенного source других owners — dated drift, не разрешение
переписать их файлы и не скрытый native PASS.

Финальная сверка: из42 текущих выбранных inputs изменился только Android STATE
S2→S3; все45 captures и3 original document prefixes сохранены. Relevant transport
code и перечисленные Android bridge/RPC files совпадают с captures. Свежий S3
снимок прочитан как context, его новые OS-observer файлы полностью не reviewed.
Windows фактически captured V2-162, хотя начальное знакомство ещё видело V2-161.

Ключевые новые fingerprints:

| Файл | SHA256 raw/cache |
|---|---|
| quic-go framer.go | `d062ad677db999f50d09783648aaa85c5d8cde23d28f7bdbf08148de17d18bcf` |
| quic-go datagram_queue.go | `f2e504fe6a12373037d39880f0803d2dc5a9531b90460127453305585fe54e62` |
| HY2 service_packet.go | `c03e0a575af90adad3c597a6cbfee7a92ed8046ec7cf30341642e84c6cfe574a` |

Остальные fingerprints перечислены в новом manifest; совпадение с первым
пакетом по тем же файлам не переносит его complete на новые fairness/lifetime
гипотезы. Новые runtime/native проверки не выполнялись.

<a id="r04-003-sources"></a>

## 8. R04-003 — clocks, local keepalive, energy attribution

Просмотр2026-10-25/26. Пакет [REPORT §11](REPORT.md#r04-003),
captures `.artifacts/research/theory-04/2026-10-25-clock-energy-v3/`.

### 8.1. Версии и лицензии

| Source | Exact commit / версия | Лицензия / применение |
|---|---|---|
| quic-go | `ccd46abedd39c6f8384b1df8daf42008724958f5`, v0.59.0-sing-box-mod.4 | MIT; process monotime и connection timers |
| sing-quic | `d83826c306d7c008cafb3fd6d8ee07cbcfd656ed`, v0.6.4-0.20260803041914-d83826c306d7 | GPL-3.0-or-later; HY2 config/UDP caller |
| sing-tun | `7c73233bd0fb0dae02e76505af888ef729319be9`, v0.8.12-0.20260810140523-7c73233bd0fb | GPL-3.0-or-later; gVisor local endpoint/TUN link |
| gvisor | `20b7e9b2d7e2c107118614f0516c2e92b9f1c3f1`, v0.0.0-20250811.0-sing-box-mod.1 | Apache-2.0; clock/timer/sendRaw; Origin record сохранён |
| sing-box | `b5ebaa1fc0f2b94256180b95468e73ef53caa27d`, v1.13.19 | GPL-3.0-or-later; TCP socket config/dialer |
| Go reference | `d90b98e65320778f3b1f99a6951ab20f04d218b3`, go1.26.0 | BSD-3-Clause; explicit reference, не verified EXE/APK toolchain |

Лицензии захвачены; новая зависимость/код не импортировались. Перенос алгоритма
не снимает obligations соответствующего upstream. Exact source-candidate Android
lock и отсутствующие approved AAR/Go digest сохранены, Windows directive не
использован как доказательство компилятора Android.

### 8.2. Claim → pinned caller

| ID | Файл и символ | Значение / предел |
|---|---|---|
| D3-U01 | [quic-go monotime](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/internal/monotime/time.go), start/Now/Until/FromTime | process relative ns; time.Since, не boot elapsed |
| D3-U02 | [connection.go](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/connection.go), nextKeepAliveTime/maybeResetTimer/run/handleTransportParameters | Receive/PTO/blocked semantics, PING queued vs wire |
| D3-U03 | [sys_conn.go](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/sys_conn.go), basicConn.ReadPacket | User-space rcvTime; не автоматически kernel-wire timestamp. Optimized ветви отдельно |
| D3-U04 | [gLazyConn](https://github.com/SagerNet/sing-tun/blob/7c73233bd0fb0dae02e76505af888ef729319be9/stack_gvisor_lazy.go), HandshakeContext | Local endpoint, keepalive15/15, gonet wrapper |
| D3-U05 | [stack_gvisor.go](https://github.com/SagerNet/sing-tun/blob/7c73233bd0fb0dae02e76505af888ef729319be9/stack_gvisor.go), newGVisorStack; [gVisor stack](https://github.com/SagerNet/gvisor/blob/20b7e9b2d7e2c107118614f0516c2e92b9f1c3f1/pkg/tcpip/stack/stack.go), New; [StdClock](https://github.com/SagerNet/gvisor/blob/20b7e9b2d7e2c107118614f0516c2e92b9f1c3f1/pkg/tcpip/stdclock.go) | No custom Clock → StdClock → time.Since(baseTime), AfterFunc |
| D3-U06 | [TCP connect.go](https://github.com/SagerNet/gvisor/blob/20b7e9b2d7e2c107118614f0516c2e92b9f1c3f1/pkg/tcpip/transport/tcp/connect.go), keepaliveTimerExpired/sendEmptyRaw/sendRaw; [snd.go](https://github.com/SagerNet/gvisor/blob/20b7e9b2d7e2c107118614f0516c2e92b9f1c3f1/pkg/tcpip/transport/tcp/snd.go), sendEmptySegment | Dataless local TCP ACK, route.WritePacket; не HY2 stream application Write |
| D3-U07 | [Windows TUN endpoint](https://github.com/SagerNet/sing-tun/blob/7c73233bd0fb0dae02e76505af888ef729319be9/tun_windows_gvisor.go), WritePackets; [Linux TUN](https://github.com/SagerNet/sing-tun/blob/7c73233bd0fb0dae02e76505af888ef729319be9/tun_linux_gvisor.go), WritePacket/NewEndpoint | Tun write / FD, включая fdbased branch. Не production device capture |
| D3-U08 | [HY2 client](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/hysteria2/client.go), NewClient/offerNew; [defaults](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/hysteria/protocol.go) | QUIC10/30, UDP DialContext, public ClientOptions без period option |
| D3-U09 | [default dialer](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/common/dialer/default.go), NewDefault/DialContext | net.KeepAliveConfig для TCP socket; не QUIC heartbeat |
| D3-G01 | [Go time.go](https://github.com/golang/go/blob/d90b98e65320778f3b1f99a6951ab20f04d218b3/src/time/time.go), Since/Now/monotonic docs; [runtime time](https://github.com/golang/go/blob/d90b98e65320778f3b1f99a6951ab20f04d218b3/src/runtime/time.go), time_runtimeNano; [timestub](https://github.com/golang/go/blob/d90b98e65320778f3b1f99a6951ab20f04d218b3/src/runtime/timestub.go) | Exact normal-runtime path, не synctest/faketime; monotonic не serializable authority |
| D3-G02 | [Linux arm64 nanotime](https://github.com/golang/go/blob/d90b98e65320778f3b1f99a6951ab20f04d218b3/src/runtime/sys_linux_arm64.s), nanotime1/CLOCK_MONOTONIC | Clock id1; только разобранная архитектура/reference |
| D3-G03 | [Windows time.now](https://github.com/golang/go/blob/d90b98e65320778f3b1f99a6951ab20f04d218b3/src/runtime/time_windows_amd64.s), [nanotime1](https://github.com/golang/go/blob/d90b98e65320778f3b1f99a6951ab20f04d218b3/src/runtime/sys_windows_amd64.s), [definitions](https://github.com/golang/go/blob/d90b98e65320778f3b1f99a6951ab20f04d218b3/src/runtime/time_windows.h) | `_INTERRUPT_TIME`, ns conversion; не QPC/unbiased clock |

### 8.3. Внешние первоисточники

| ID | Источник | Использование / не переносить |
|---|---|---|
| D3-P01 | [Linux man-pages clock_gettime](https://man7.org/linux/man-pages/man2/clock_gettime.2.html), CLOCK_MONOTONIC/BOOTTIME | Suspend semantics; не фактический sleep residency нашего устройства |
| D3-P02 | [Android SystemClock](https://developer.android.com/reference/android/os/SystemClock) | elapsedRealtime включает deep sleep; uptime исключает; не trusted UTC |
| D3-P03 | [Microsoft Interrupt Time](https://learn.microsoft.com/en-us/windows/win32/sysinfo/interrupt-time) | Biased/unbiased interrupt time; не подменять QueryPerformanceCounter |
| D3-P04 | [Go build constraints](https://pkg.go.dev/cmd/go#hdr-Build_constraints) | Android дополнительно совпадает с Linux tags/files; tool не запускался |
| D3-P05 | [RFC4787 §4.3](https://www.rfc-editor.org/rfc/rfc4787.html#section-4.3) | NAT outbound/inbound refresh requirements; не measured Russian CGNAT lifetime |
| D3-P06 | [RFC8085 §3.5](https://www.rfc-editor.org/rfc/rfc8085.html#section-3.5) | No arbitrary middlebox timeout, recovery, middlebox keepalive pacing; не автоматическое нарушение QUIC10s |
| D3-P07 | [Optimize network access](https://developer.android.com/develop/connectivity/network-ops/network-access-optimization) | Radio/tail/batching context. Пример3G, не LTE/5G benchmark и не mAh estimate |
| D3-P08 | [Doze and App Standby](https://developer.android.com/training/monitoring-device-state/doze-standby) | Deferred resources/exemptions, не тождество deep sleep и Doze; adb examples не выполнялись |
| D3-P09 | [ConnectivityManager createSocketKeepalive](https://developer.android.com/reference/android/net/ConnectivityManager), [SocketKeepalive](https://developer.android.com/reference/android/net/SocketKeepalive) | Public IPsec NAT-T API/async lifecycle; не произвольный encrypted QUIC PING offload |

### 8.4. Provenance и сбои получения

`source-manifest.json`:30 files. `go-reference-manifest.json`:8 nonempty files,
Go tag response и gVisor Origin сохранены отдельно. `reused-upstream-provenance.json`:
7 current hashes совпадают с ранее проверенными upstream bytes; это reuse доказательств
при identical hash, не повторный network fetch. `upstream-comparison.json`:3 свежих
raw/cache совпадения (gLazyConn, gVisor connect.go, Apache LICENSE);2 fetch timeouts
не выданы за source equality. Не все19 module files независимо сравнивались с raw.

Неудачная догадка `runtime/time_linux_arm64.s` дала timeout/NOT_FOUND; до включения
terminating errors возник пустой staging file. Он **не source**, excluded из
authoritative manifest, запись `sourcing-failures.json`. Затем фактический
`timestub.go` и runtime/time.go получены с nonempty validation и SHA256.
Повторный GitHub directory API timeout не был обойдён изменением network policy.
Ни ошибка скачивания, ни SHA256 source provenance не являются test/build proof.

Финальный scoped drift/prefix review и artifact inventory — `source-drift.json`
и `packet-output-manifest.json`. Current source snapshots могут двигаться в других
чатах; выводы относятся к captured inputs, не ко всем будущим revisions дерева.

<a id="r04-004-sources"></a>

## 9. R04-004 — HY2/BBR clocks и extended feedback

Проверка2026-10-26. Canonical path после migration — docs/theory/research/04-PERFORMANCE/;
новые отчёты в старом THEORY-LAB не записывались. [REPORT §12](REPORT.md#r04-004).
Capture root `.artifacts/research/theory-04/2026-10-26-bbr-clock-v4/`.

### 9.1. Pins, лицензии и provenance

| Module | Exact commit / версия | License |
|---|---|---|
| SagerNet/quic-go | `ccd46abedd39c6f8384b1df8daf42008724958f5`, v0.59.0-sing-box-mod.4 | MIT |
| SagerNet/sing-quic | `d83826c306d7c008cafb3fd6d8ee07cbcfd656ed`, v0.6.4-0.20260803041914-d83826c306d7 | GPL-3.0-or-later |
| SagerNet/sing | `7c349dacf402256d3a7029746073b05d2ead584a`, v0.8.13 | GPL-3.0-or-later |
| SagerNet/sing-box | `b5ebaa1fc0f2b94256180b95468e73ef53caa27d`, v1.13.19 | GPL-3.0-or-later |
| Go reference | `d90b98e65320778f3b1f99a6951ab20f04d218b3`, go1.26.0 | BSD-3-Clause |

45 source-manifest captures:33 module sources/licenses +8 project inputs +4 prior
own docs.6 свежих raw/cache comparisons совпали: interface, ccAdapterEx,
sent_packet_handler, Brutal, BBR sender, meta2 pacer. Их SHA256 и URLs сохранены
в upstream-comparison.json; остальные captured files не объявляются независимо
remote verified.4 Origin records повторно **не** запрашивались — reused prior
module-metadata с теми же pins, с hashes в отдельном inventory.

Go spec взят по exact1.26 commit, не с текущей страницы go.dev (она уже1.27).
time.go/LICENSE reuse из R04-003 совпадают по прежним hashes; reference manifest
не утверждает real compiler EXE/APK. Никакой go list/mod download/resolve не было.

### 9.2. Claim → caller и branch

| ID | Exact primary URL / symbol | Что проверяется |
|---|---|---|
| D4-U01 | [HY2 client](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/hysteria2/client.go), offerNew; [service](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/hysteria2/service.go), ServeHTTP | Auth/RxAuto/rates, different direction selection, BBR Clock injection |
| D4-U02 | [sing-quic go.mod](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/go.mod), [sing-box go.mod](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/go.mod) | mod.2 lower requirement против declared root/core mod.4; не artifact proof |
| D4-U03 | [outbound.go](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/protocol/hysteria2/outbound.go), NewOutbound; [HY constants](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/hysteria/protocol.go) | UpMbps/DownMbps×125000 bytes/s |
| D4-U04 | [Box](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/box.go), New; [time_wrapper.go](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/common/tls/time_wrapper.go); [NTP context](https://github.com/SagerNet/sing/blob/7c349dacf402256d3a7029746073b05d2ead584a/common/ntp/context.go), [service](https://github.com/SagerNet/sing/blob/7c349dacf402256d3a7029746073b05d2ead584a/common/ntp/service.go) | Conditional registration, fallback и time.Now.Add(offset); не audit TLS/trust/NTP security |
| D4-U05 | [Clock](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/congestion_meta2/clock.go), DefaultClock.Now; [BBR sender](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/congestion_meta2/bbr_sender.go), newBbrSender/enterStartupMode/OnPacketSent/OnCongestionEventEx/maybeUpdateMinRtt/maybeEnterOrExitProbeRtt | Единственный injected clock result dead; actual mono event state; full extended interface присутствует у BBR |
| D4-U06 | [public interface](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/congestion/interface.go), CongestionControlEx; [handler](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/internal/ackhandler/sent_packet_handler.go), SetCongestionControl/ReceivedAck/OnLossDetectionTimeout | Method-set selection и only-Ex batch dispatch |
| D4-U07 | [cc_adapter.go](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/internal/ackhandler/cc_adapter.go), [cc_adapter_ex.go](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/internal/ackhandler/cc_adapter_ex.go) | Нет extended callback у base; forward type conversions без wall-clock conversion |
| D4-U08 | [Brutal](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/hysteria/congestion/brutal.go), NewBrutalSender/OnPacketAcked/OnCongestionEvent/OnCongestionEventEx/updateAckRate | Old time.Time signature, missing lifecycle methods, base stubs, ackRate writers/caller |
| D4-U09 | [public monotime alias](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/monotime/time.go), [internal](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/internal/monotime/time.go) | Same type/domain; FromTime relative subtraction, Add/Sub actual arithmetic |
| D4-U10 | [connection](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/connection.go), registerPackedShortHeaderPacket; [basic receive](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/sys_conn.go), [Windows](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/sys_conn_windows.go), [OOB](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/sys_conn_oob.go) | Send/event mono path; receive user-space timestamp, не wire receipt |
| D4-U11 | [meta2 pacer](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/congestion_meta2/pacer.go), [Brutal pacer](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/hysteria/congestion/pacer.go), [bandwidth](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/congestion_meta2/bandwidth.go), [sampler](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/congestion_meta2/bandwidth_sampler.go) | Units, positivity checks, multiply-before-divide, unused helper boundary |
| D4-U12 | [existing internal pacer](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/internal/congestion/pacer.go), Budget/timeScaledBandwidth | MIT reuse guard pattern уже в closure; rate×5/4/burst policy не переносить автоматически |
| D4-P01 | [Go1.26 spec](https://github.com/golang/go/blob/d90b98e65320778f3b1f99a6951ab20f04d218b3/doc/go_spec.html), Method_sets/Type_identity/Integer_overflow; [time.go](https://github.com/golang/go/blob/d90b98e65320778f3b1f99a6951ab20f04d218b3/src/time/time.go), Add/Sub | Статический вывод совместимости/overflow; spec reading не compilation |

### 9.3. Проектные входы и manual-only evidence

Windows go.mod/go.sum, renderer/config validator и STATE V2-167; Android STATE
AND-03-S11 и native/source-lock.json. Реальные profile/rates/runtime-config не
читались. Собственные REPORT/SOURCES/STATE/handoff003 сохранены перед append.
Срез ограничен HY2 meta2/Brutal, не полным аудитом всех CC/TLS/NTP consumers.

Hand traces: rejected NTP→BBR deadline dependency; method-set/base-vs-Ex dispatch;
80ACK/20loss illustration; B=2^30,Δ=2^34ns overflow refill. Это не tests/fixtures,
не simulation/benchmark и не assertion, выполненный compiler. Будущие Q1–Q7
в REPORT12.7 имеют NOT_RUN. Итог drift/prefix review — source-drift.json.

<a id="r04-005-sources"></a>

## 10. R04-005 — timestamp placement, queue/GSO и PMTU loss

2026-10-26. [REPORT13](REPORT.md#r04-005), capture root
`.artifacts/research/theory-04/2026-10-26-queue-pmtu-v5/`.
Те же exact pins: quic-go `ccd46abedd39c6f8384b1df8daf42008724958f5`
(v0.59.0-sing-box-mod.4, MIT), sing-quic
`d83826c306d7c008cafb3fd6d8ee07cbcfd656ed`
(v0.6.4-0.20260803041914-d83826c306d7, GPL-3.0-or-later).

### 10.1. Новые caller/evidence pointers

| ID | Exact primary source / символ | Вывод / граница |
|---|---|---|
| D5-U01 | [connection.go](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/connection.go), run/handlePackets/triggerSending/sendPacketsWithoutGSO/sendPacketsWithGSO/appendOneShortHeaderPacket/registerPackedShortHeaderPacket | Один run timestamp, registration до enqueue,32 receive entries, partial flush и mode checks |
| D5-U02 | [send_queue.go](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/send_queue.go), Send/Run/Available; [send_conn.go](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/send_conn.go), Write/writePacket | Async Write/release, GSO fallback, queue credit не wire receipt |
| D5-U03 | [sys_conn.go](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/sys_conn.go), wrapConn/basicConn.ReadPacket/capabilities; [Windows](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/sys_conn_windows.go), newConn; [OOB](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/sys_conn_oob.go), ReadPacket | User-space stamp, interface capability gate; не hardware/kernel arrival time |
| D5-U04 | [HY2 client](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/hysteria2/client.go), offerNew; [Salamander](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/hysteria2/salamander.go), both wrappers; [hop](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/hysteria/hop.go), SyscallConn; [qtls](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/quic.go), CreateTransport; [Transport](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/transport.go), init | Standard branch no GSO through these wrappers. Custom TLS implementation не attested; obfs per-datagram framing нельзя обходить |
| D5-U05 | [mtu_discoverer.go](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/mtu_discoverer.go), GetPing/OnAcked/OnLost; [packet_packer.go](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/packet_packer.go), PackMTUProbePacket/appendShortHeaderPacket/PackPathProbePacket | Distinct PMTU/path probe flags, padded PING, own PMTU handlers |
| D5-U06 | [packet.go](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/internal/ackhandler/packet.go), Outstanding/IsAckEliciting; [history](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/internal/ackhandler/sent_packet_history.go), SentPacket/Packets/FirstOutstanding | PMTU excluded from outstanding count, retained in normal loss iteration — ключевой контраргумент проверен |
| D5-U07 | [handler](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/internal/ackhandler/sent_packet_handler.go), SentPacket/detectLostPackets/ReceivedAck/OnLossDetectionTimeout/queueFramesForRetransmission; [Ex adapter](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/internal/ackhandler/cc_adapter_ex.go); [interface](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/congestion/interface.go) | Legacy PMTU guard не охватывает Ex lost list; reason не переносится |
| D5-U08 | [BBR](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/congestion_meta2/bbr_sender.go), OnCongestionEventEx/updateRecoveryState/calculateRecoveryWindow/GetCongestionWindow; [sampler](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/congestion_meta2/bandwidth_sampler.go), OnPacketSent/OnPacketLost/onPacketAcknowledged | Probe loss counted; conditional full-bandwidth recovery trace. min(sendRate,ackRate) ограничивает обобщения ACK compression |
| D5-U09 | [buffer_pool.go](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/buffer_pool.go), [protocol.go](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/internal/protocol/protocol.go), [params.go](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/internal/protocol/params.go) | ordinary1452/large20KiB/initial1280 units; не actual path MTU или total RSS |

### 10.2. Первичная теория и недоступный источник

| ID | URL / версия / просмотр | Применение |
|---|---|---|
| D5-P01 | [RFC9000 §14.4](https://www.rfc-editor.org/rfc/rfc9000.html#section-14.4), May2021,2026-10-26 | PMTU probe ack-eliciting/window usage; loss SHOULD NOT cause congestion reaction. Не blanket conformance assertion |
| D5-P02 | [RFC9002 §5.1/7.7](https://www.rfc-editor.org/rfc/rfc9002.html), May2021,2026-10-26 | Sample/ACK-delay и pacing architecture; local timestamp model выведена отдельно по source |
| D5-P03 | [Linux segmentation offloads](https://docs.kernel.org/networking/segmentation-offloads.html), docs просмотр2026-10-26 | Segmentation/GSO concepts, не proof real socket/Android capability |
| D5-P04 | [BBR authors publication](https://research.google/pubs/bbr-congestion-based-congestion-control/), Cardwell et al., CACM60/2017 | Primary model context, не подтверждение характеристик нашего port. ACM Queue direct link вернул403; его full text не считается прочитанным |

### 10.3. Provenance и область manual review

source-manifest35=26 module source/licenses +5 project +4 prior docs.
3 fresh raw/cache matches: MTU finder, packet flags, sent_packet_history.
packet_packer raw request timeout явно записан unavailable; source capture из
module cache не выдаётся за свежую remote equality.5 reuse из R04-004 совпали
по exact hashes (handler/Ex/interface/BBR/pacer). Полный inventory в manifests.

Windows V2-168 renderer/go.mod/STATE и Android AND-12-S9 STATE/source-lock
captured; whole AAR/EXE/runtime/profiles не прочитаны/не verified. После capture
платформенные STATE могут меняться — source-drift.json определяет bounded review.
Ранее сохранённые own reports/handoff004 сохраняются полными prefixes.

Manual evidence: R_raw decomposition, byte-service bound, batch CPU model,
PMTU-only Ex event recovery arithmetic и9 opposing claims. Все future cases
REPORT13.8 NOT_RUN. Нет tests/fixtures/parser/compiler/simulation/benchmark/native/
live/OS-network execution. Использование source SHA256 — только provenance.

<a id="r04-006-sources"></a>

## 11. R04-006 — semantic flight count contract

2026-10-26, [REPORT14](REPORT.md#r04-006). Capture root
`.artifacts/research/theory-04/2026-10-26-flight-contract-v6/`:
13 files=7 module source/license +2 project manifests +4 prior own docs.
Pins/licenses — те же quic-go mod.4 MIT / sing-quic d83826c GPL-3.0-or-later.

| Primary exact source | Symbol / проверка |
|---|---|
| [sent_packet_handler.go](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/internal/ackhandler/sent_packet_handler.go) | SentPacket: after-flight increment до callback, PathProbe separate branch |
| [cc_adapter_ex.go](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/internal/ackhandler/cc_adapter_ex.go), [public interface](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/congestion/interface.go) | OnPacketSent numeric cast без semantic conversion; ByteCount contract |
| [BBR sender](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/congestion_meta2/bbr_sender.go) | OnPacketSent/current state/idle flag; maybeEnterOrExitProbeRtt; shouldExitStartupDueToLoss |
| [bandwidth sampler](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/congestion_meta2/bandwidth_sampler.go) | OnPacketSent zero check/+bytes; newConnectionStateOnSentPacket/NewSendTimeState stored after-flight; ACK/loss consumers |

Source scope — selected meta2/HY2 declared closure, не все CC consumers.
Windows go.mod/Android source-lock captured; artifact compiler/whole AAR не proven.
Raw sampler acquisition/result записан upstream-comparison.json; absence equality
не заменять предположением. Остальные reused source hashes привязаны к earlier
v4 verified comparison. Final preservation — source-drift.json.
Manual F=0,L=1200→post1200/snapshot2400 и four future cases NOT_RUN.

<a id="r04-007-sources"></a>

## 12. R04-007 — upstream remediation и semantic compatibility

Просмотр2026-10-26,10:58–11:29Z. [Handoff007](handoffs/R04-007.md),
[REPORT15](REPORT.md#r04-007). Capture root
`.artifacts/research/theory-04/2026-10-26-callback-remediation-v7/`.

### 12.1. Версии, primary URLs и реальные callers

| Источник / pin | Точка проверки | Что взято / предел |
|---|---|---|
| [sing-quic v0.7.0,4ab2ece](https://github.com/SagerNet/sing-quic/commit/4ab2eceaac81e073f53b22ec72ff56aaea89a3d1),28авг2026 | [Brutal](https://github.com/SagerNet/sing-quic/blob/4ab2eceaac81e073f53b22ec72ff56aaea89a3d1/hysteria/congestion/brutal.go): assertion/OnCongestionEventEx/lifecycle/constructor; [pacer](https://github.com/SagerNet/sing-quic/blob/4ab2eceaac81e073f53b22ec72ff56aaea89a3d1/hysteria/congestion/pacer.go) | Full Ex, monotime buckets, initial MTU seed; GPL-3.0-or-later captured. Не whole-core fix |
| Same sing-quic pin | [go.mod](https://github.com/SagerNet/sing-quic/blob/4ab2eceaac81e073f53b22ec72ff56aaea89a3d1/go.mod), [HY2 client](https://github.com/SagerNet/sing-quic/blob/4ab2eceaac81e073f53b22ec72ff56aaea89a3d1/hysteria2/client.go): constructor selection | Go1.25/sing0.9.0-beta.4/quic0.61-mod.6; changed transport/constructor seams. Declared, not resolved/built |
| Same sing-quic pin | [BBR sender](https://github.com/SagerNet/sing-quic/blob/4ab2eceaac81e073f53b22ec72ff56aaea89a3d1/congestion_meta2/bbr_sender.go): NewBbrSenderWithProfile/OnPacketSent/OnPacketNeutered/OnPacketsLost; [sampler](https://github.com/SagerNet/sing-quic/blob/4ab2eceaac81e073f53b22ec72ff56aaea89a3d1/congestion_meta2/bandwidth_sampler.go); [pacer](https://github.com/SagerNet/sing-quic/blob/4ab2eceaac81e073f53b22ec72ff56aaea89a3d1/congestion_meta2/pacer.go) | Clock dependency removed, retirement callback forwarding. Not proof all discard paths bounded; sampler still adds packet bytes to expected prior-flight |
| quic-go v0.61.0-sing-box-mod.6, `f7b7e9c4c6f6ec45e4ab2b4788eca63192975d80` | [Interface](https://github.com/SagerNet/quic-go/blob/f7b7e9c4c6f6ec45e4ab2b4788eca63192975d80/congestion/interface.go): documented prior-flight/global congestion PN/OnPacketNeutered; [handler](https://github.com/SagerNet/quic-go/blob/f7b7e9c4c6f6ec45e4ab2b4788eca63192975d80/internal/ackhandler/sent_packet_handler.go): SentPacket/SetCongestionControl/reports/neuterPacket/notifyLeastUnacked/detectLostPackets | Paired engine semantics. MIT captured; Go proxy Origin archived. reports(p) checks install boundary, not PMTU flag; Ex loss append remains outside PMTU guard |
| Same quic pin | [ccAdapterEx](https://github.com/SagerNet/quic-go/blob/f7b7e9c4c6f6ec45e4ab2b4788eca63192975d80/internal/ackhandler/cc_adapter_ex.go), [go.mod](https://github.com/SagerNet/quic-go/blob/f7b7e9c4c6f6ec45e4ab2b4788eca63192975d80/go.mod) | Forwarding new terminal callback, dependency cost. No binary compatibility assertion |
| Hysteria core/v2.13.0, `ca8fbd874fd6ca82413948de6dc5924405e103e1`,5окт2026 | [Brutal](https://github.com/apernet/hysteria/blob/ca8fbd874fd6ca82413948de6dc5924405e103e1/core/internal/congestion/brutal/brutal.go): disableLossCompensation/min cwnd/monotime; [core go.mod](https://github.com/apernet/hysteria/blob/ca8fbd874fd6ca82413948de6dc5924405e103e1/core/go.mod), [MIT license](https://github.com/apernet/hysteria/blob/ca8fbd874fd6ca82413948de6dc5924405e103e1/LICENSE.md) | Reference/adapt algorithm/option; not a drop-in SagerNet API or Snowden config field. App release tag and core tag separated in metadata |
| Apernet/quic-go v0.63.1-0.20261004180939-a10df75c260c, `a10df75c260cee8161f3261d63a37bd125a6bb2a` | [Interface](https://github.com/apernet/quic-go/blob/a10df75c260cee8161f3261d63a37bd125a6bb2a/congestion/interface.go), full Origin archived | Ex requires batch only above base, unlike SagerNet lifecycle set; package types differ. Interface source archived, no whole runtime/license closure review |

### 12.2. Current inputs и provenance

Current pinned source: sing-quic d83826c306d7c008cafb3fd6d8ee07cbcfd656ed
(GPL-3.0-or-later), quic-go ccd46abedd39c6f8384b1df8daf42008724958f5(MIT).
source-manifest21=10 pinned source/license+7 project inputs+4 prior docs.
Current upstream equality переиспользована только для5 identical source hashes;
остальные current source captures не объявлены freshly remote compared.

Remote source sets:8 sing-quic v0.7.0 +5 paired quic +3 Hysteria reference +1
Apernet interface. Их exact URLs/hashes — отдельные *-manifest.json. Это source
acquisition, не module installation, resolver/build или compiler execution.
Origin/tag/commit/release JSON metadata сохранены как lookup provenance.
Current HEAD получен только для discovery; выводы используют immutable commits.
API request full fix commit и HEAD Hysteria timed out; raw source и versioned
Go proxy metadata использованы вместо недоступного full diff/HEAD. Они не стали
доказательством чтения всех37 changed files или всех возможностей новых версий.

Windows V2-171 / Android AND-11-S5 — captured context, Android whole AAR не
approved. Current root go.mod/source-lock, review2H-HB02-R04-10, CLAIM-FORMAT и
window instructions captured. Secrets/env/real profiles/runtime logs не читались.
Никаких tests/fixtures/build/crypto/native/live и изменений dependencies нет.

## 13. R04-008 — retention sources

Срез26октября2026. Чтения — local module cache exact go.mod pins;
хэши процитированных файлов:
`.artifacts/research/theory-04/2026-10-26-retention-v8/local-inputs-sha256.txt`.

| Source pin | Symbols и boundary |
|---|---|
| quic-go v0.59.0-sing-box-mod.4 | internal/ackhandler/sent_packet_history.go (no cap, Remove/DeclareLost/cleanupStart/reslice); sent_packet_handler.go :31 maxPTODuration=60s, :638 Remove on ACK, :665-671 getScaledPTO клэмп shift-overflow, :795-815 detectLostPathProbes, :902-985 OnLossDetectionTimeout PTO без удаления |
| sing-quic v0.6.4-0.20260803… | hysteria/protocol.go:18-21 окна 8/20MB, idle30s, keepalive10s; hysteria2/client.go:112-154 single-flight offer/pending |
| RFC 9002 §6.1/§6.2 | time-threshold loss detection требует ACK большего PN; PTO не объявляет потерю — соответствие fork проверено по source |

Отдельно захэшированы sent_packet_history.go и sent_packet_handler.go
(см. artifacts). Remote byte equality не заявлена; никакого build/test/run.
Зависимости/код продукта не менялись. Все future execution cases NOT_RUN.

## 14. R04-009 — fairness/admission sources

| Источник | Что фиксирует |
|---|---|
| [HyNetworks/tcp-brutal README](https://github.com/HyNetworks/tcp-brutal) | Brutal: фиксированная rate, без bandwidth-probing, ретрансмиссии той же rate при потерях |
| [sing-box: Hysteria2](https://sing-box.sagernet.org/manual/proxy-protocol/hysteria2) | Brutal = «достичь заданной полосы несмотря на потери» (официальная документация) |
| [Wang et al. 2025 «Is Custom Congestion Control a Bad Idea…»](https://censoredplanet.org/papers/congestion.pdf) | Кастомный CCA — различимый сигнал; конфликт с принципами circumvention |
| [apernet/hysteria#1447](https://github.com/apernet/hysteria/issues/1447) | Полевое подозрение CCA→IP-блокировка (2025-09-24) |
| [Briscoe 2007, ACM CCR 37(2)](https://dl.acm.org/doi/10.1145/1232919.1232922) | Критика frame «справедливости потоков»; цели распределения первичны |
| Hock, Bless, Zitterbart — Experimental Evaluation of BBR (ICNP 2017, [IEEE 8116707](https://ieeexplore.ieee.org/document/8116707)) | RTT-нечестность/задержко-нечувствительность BBRv1 |
| sing-quic pin (module cache) | congestion_meta2/bbr2_sender.go (auto-ветка); hysteria2/service.go — per-user bandwidth/obfs/udp, отсутствие conn-cap |

Срез26октября2026: web-страницы прочитаны; PDF Wang et al. —
по аннотации/цитатам каталога (full-text NOT_RUN). Никаких симуляций/замеров.
Все future cases NOT_RUN.
