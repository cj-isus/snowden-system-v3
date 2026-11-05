# R28-C04 — sources and provenance

Чтение 2026-10-28 UTC. Корень actual source: `D:/SNOWDEN-SYSTEM-V3`.
Source/spec/manual inference не являются artifact/native/runtime доказательствами.
Статус чтения относится к названным sections, а не всему repository.

## 1. Actual declarations и caller — SECTIONS_READ

| ID | Source / прочитанная область | Значение и предел |
|---|---|---|
| L01 | `windows/go.mod`: go directive, core/transport/sing/x/sync declarations | Go directive1.26; sing-box1.13.19, sing-quic d83826c306d7, quic-go0.59.0-sing-box-mod.4, sing0.8.13, x/sync0.20.0. Не binary attestation |
| L02 | `windows/tools/package.ps1` tags; `backend/render/render.go` HY2; `corebridge.go:456–505`; `backend/core/worker_child.go` and `engine.go` creation seam | Source composition: actual isolated worker; optional Salamander/hopping; box.New. Реальный профиль не читался |
| L03 | `windows/backend/core/worker_process.go:59–128,138–214,269–290`; `worker_protocol.go` budgets; `worker_host_windows.go:62–109,142–149`; `worker_spawn_windows.go` job flags | Existing original-handle Job/Wait/I/O/Empty barrier; failed cleanup retains owner. Engine policy processes8, commitBytes omitted. Таймер 2s+4s не задаёт hard OS syscall deadline |
| L04 | `android/native/source-lock.json` (manifest); `android/native/README.md`; app build declarations | core commit b5ebaa1, AAR hash=null, Go=null, owner/cache/closed-history adaptation source-only, stock AAR incompatible. Exact adopted runtime UNKNOWN |
| L05 | `android/corebridge/.../NativeWorkerSession.kt`: initialize/start/retirement; `NativeRuntime.kt`; `configuration/NativeGraphSecurity.kt` HY2 keys | Per-process claim, retained TUN/scratch/native owner; successful local close still requires exit. Hopping/Salamander accepted as optional fields; actual use UNKNOWN |
| L06 | `android/vpn/.../WorkerProcess.kt:30–87,263–337,365–399,519–527`; `VpnSessionRepository.kt:426–443,659–677` | elapsed authority/death deadlines; independent watchdog; parent waits actual normal+retirement RPC returns and prepared-source idle. Source watchdog is not elapsed-time death guarantee |

Сокращение `...` в таблице означает соответствующий каталог
`src/main/kotlin/org/snowden/android/<module>/`. Полные пути и хеши находятся
в названном реестре копий; значения конфигурации продукта, secrets и логи
не использовались.

## 2. Exact public implementation sources

Публичные raw-копии с UTC/SHA256:
`.artifacts/research/r28-contracts/C04/public-source-manifest.jsonl`.
Совпадение с module cache подтверждено для начальных четырёх HY2/QUIC-файлов
и двух x/sync; последующие сопоставления записаны отдельно.

| ID | Pin / source | Read depth |
|---|---|---|
| P01 | sing-box1.13.19 / `b5ebaa1fc0f2b94256180b95468e73ef53caa27d`: [HY2 outbound](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/protocol/hysteria2/outbound.go), [route/conn.go](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/route/conn.go) | SECTIONS_READ: outbound34–120; route144–259,356–418 and tracking/CloseAll |
| P02 | sing-quic `d83826c306d7c008cafb3fd6d8ee07cbcfd656ed`: [packet.go](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/hysteria2/packet.go), [client.go](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/hysteria2/client.go), client_packet.go, packet_wait.go; [hop.go](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/hysteria/hop.go); hysteria2/salamander.go | packet/client/Salamander SECTIONS_READ; client_packet/packet_wait/hop FULL_TEXT_READ. Caller buffers, fragments, shared carrier, locks/close and all hop recvLoop callers |
| P03 | quic-go `ccd46abedd39c6f8384b1df8daf42008724958f5`: [datagram_queue.go](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/datagram_queue.go), [send_queue.go](https://github.com/SagerNet/quic-go/blob/ccd46abedd39c6f8384b1df8daf42008724958f5/send_queue.go), connection.go, packet_packer.go, internal/ackhandler/sent_packet_handler.go | Queue files FULL_TEXT_READ; rest SECTIONS_READ SendDatagram, pack DATAGRAM, non-GSO send, run close, ACK/loss/PTO/SendMode |
| P04 | same sing-quic pin: [packet_number_indexed_queue.go](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/congestion_meta2/packet_number_indexed_queue.go), [ringbuffer.go](https://github.com/SagerNet/sing-quic/blob/d83826c306d7c008cafb3fd6d8ee07cbcfd656ed/congestion_meta2/ringbuffer.go), bandwidth_sampler.go, bbr_sender.go | Queue/ring FULL_TEXT_READ; sampler/BBR SECTIONS_READ on insertion/retirement and exact field search. Present/span/backing high water are separate |
| P05 | sing0.8.13 / `7c349dacf402256d3a7029746073b05d2ead584a`: common/buf/{buffer,pool,alloc}.go; common/bufio/{copy,bind,vectorised}.go; common/network/{direct,thread}.go; common/canceler/{packet,instance}.go; common/cache/lrucache.go | SECTIONS_READ; allocator Get path and canceler files read. Alias, rounded backing, wrapper method set, caller MTU/headroom, ownership and lazy wall-clock TTL |
| P06 | [x/sync v0.20.0 semaphore](https://github.com/golang/sync/blob/v0.20.0/semaphore/semaphore.go), [errgroup](https://github.com/golang/sync/blob/v0.20.0/errgroup/errgroup.go), [LICENSE](https://github.com/golang/sync/blob/v0.20.0/LICENSE) | FULL_TEXT_READ of both source files and BSD-3-Clause license. Already declared in Windows; no dependency change |

### Candidate pair, independently read in this package

| ID | Pin / exact scope | Read depth and limit |
|---|---|---|
| P07 | sing-quic0.7.0 `4ab2eceaac81e073f53b22ec72ff56aaea89a3d1`: go.mod; congestion_meta2/bbr_sender.go constructor/OnPacketSent/lifecycle; hysteria/congestion/brutal.go constructor/method set; hysteria2/client.go CCA selection | SECTIONS_READ; manifest FULL_TEXT_READ; GPL-3.0-or-later LICENSE FULL_TEXT_READ from exact prior public capture. Constructor/graph adaptation required, not current product |
| P08 | quic-go0.61.0-sing-box-mod.6 `f7b7e9c4c6f6ec45e4ab2b4788eca63192975d80`: congestion/interface.go, internal/ackhandler/sent_packet_handler.go:190–232,344–379,947–979,1191–1221,1304–1338; datagram_queue.go, send_queue.go | Interface/sendQueue FULL_TEXT_READ; DATAGRAM Add/receive/close and handler SECTIONS_READ, call-site search for neuter/least-unacked. Matched prior-flight/interface improvement; no asserted whole resource fix. MIT LICENSE header/scope read from exact prior public capture |

LICENSE кандидата и новый HY2 client перенесены из immutable публичных
копий 26 октября с пометкой REUSED_PUBLIC_CAPTURE_2026-10-26. Их прежнее время
не объявляется новым remote fetch. Остальные raw-получения датированы
индивидуально; выбранные candidate metadata/API-файлы получены заново
в исходном цикле.

### Fresh tag→commit identity

Public GitHub tag refs, FULL_TEXT_READ of returned ref/object metadata, 09:17Z:

- sing-box1.13.19 → b5ebaa1fc0f2b94256180b95468e73ef53caa27d.
- quic-go0.59.0-sing-box-mod.4 → ccd46abedd39c6f8384b1df8daf42008724958f5.
- sing0.8.13 → 7c349dacf402256d3a7029746073b05d2ead584a.
- x/sync0.20.0 → ec11c4a93de22cde2abe2bf74d70791033c2464c.

URL, object bytes и SHA256 записаны в `public-source-manifest.jsonl`
и `tag-*.json`. Это идентичность исходника, не собранного артефакта.

## 3. Runtime-reference/spec sources — SECTIONS_READ

| ID | Source / date | Exact relevant scope |
|---|---|---|
| P09 | Local `C:/Program Files/Go/VERSION`=go1.26.7, source time2026-08-18T21:44:21Z; public tag go1.26.7 selected files fetched and byte-compared | internal/poll/fd_windows.go:215–263,830–879 (WSASend/pinning/CancelIoEx/completion); runtime/sys_linux_arm64.s:318–363 (CLOCK_MONOTONIC), sys_windows_amd64.s:200–205 (interrupt-time), runtime/netpoll_windows.go:242–271 (relative SetWaitableTimer); time/time.go monotonic Since; runtime/debug/garbage.go:181–235. Go source reference, not compiler/AAR attestation |
| P10 | [Go1.26.7 runtime/debug](https://pkg.go.dev/runtime/debug@go1.26.7#SetMemoryLimit) | Soft memory target, included/excluded memory categories. Unversioned default Go1.27.3 pages were initially opened, then rejected as selected-runtime evidence in favour of exact1.26.7 |
| P11 | [Go1.26.7 net.PacketConn](https://pkg.go.dev/net@go1.26.7#PacketConn), local net/net.go:322–382 | Concurrent calls, Close unblock semantics, deadline scope. Used only for conditional source contract comparison |
| P12 | [Microsoft CancelIoEx](https://learn.microsoft.com/en-us/windows/win32/api/ioapiset/nf-ioapiset-cancelioex), read2026-10-28 | Request vs completion, no immediate storage reuse, normal/aborted/error completion outcomes |
| P13 | [Job extended limit information](https://learn.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-jobobject_extended_limit_information), read2026-10-28 | JobMemoryLimit limits committed virtual memory when relevant flag enabled; distinct from RSS/kernel |
| P14 | [SetWaitableTimer](https://learn.microsoft.com/en-us/windows/win32/api/synchapi/nf-synchapi-setwaitabletimer), read2026-10-28 | Negative relative due; Windows8+ low-power intervals excluded; fResume semantics. Actual Go source call supplied0 |
| P15 | [Android SystemClock](https://developer.android.com/reference/android/os/SystemClock), read2026-10-28 | elapsedRealtime includes deep sleep; uptime/nanoTime and relative waiting exclude it; clock vs event delivery distinct |
| P16 | [RFC9221 §§5.1–5.4](https://www.rfc-editor.org/rfc/rfc9221.html#section-5.1), March2022 | Connection-wide DATAGRAM, no stream flow-control credit, congestion/drop, ACK is transport receipt rather than app result |
| P17 | [RFC9000 §10.1](https://www.rfc-editor.org/rfc/rfc9000.html#section-10.1), May2021 | Effective idle, RX/first-ack-eliciting reanchor, floor3PTO and liveness testing. No implementation GC/reap theorem follows |

Статьи, прочитанные только по аннотации, не использовались. Указанные
public API/docs прочитаны через web; локальные HTML-копии не заявлены там,
где их нет. Выбранные первичные source-файлы имеют локальные SHA256.
Проб внешнего endpoint/VPN не было.

## 4. Baseline, recovery and limits

- QUIC-ACCOUNTING-AND-LIMITS: §§1–6 об учёте/часах/переносе, §7 о будущих
  случаях — SECTIONS_READ.
- REPORT-010: §§2–5 о caller, локальных пределах, завершении и process owner
  — SECTIONS_READ.
- REPORT-011: §§2–5,6–8 о часах, питании, переносе и измерениях —
  SECTIONS_READ; прежние утверждения проверяются, не принимаются автоматически.
- 2026-10-28-DEEP-REVIEW: таблица решений, границы и открытый source-вопрос
  — SECTIONS_READ.
- R04-008/009 не являются аксиомами или готовыми продуктовыми фактами.
- Неверно угаданное имя локального файла не означает SOURCE_NOT_AVAILABLE
  для реализации; доступность оценивается по фактически найденному пути.

## 5. Local snapshot and moving-input policy

`.artifacts/research/r28-contracts/C04/local-input-manifest.json`
содержит 61 адресный файл: полный путь, SHA256, размер и mtime на 09:18:05Z
после чтения. Это происхождение входов, не выполнение программы,
module resolver или runtime-проверка.
`.artifacts/research/r28-contracts/C04/public-cache-equality.json`
фиксирует ещё 11 точных совпадений сверх первых шести. Native source-lock,
README и NativeWorkerSession менялись параллельно; нужные lifecycle/API области
перечитаны после снимка. Появились дополнительные source-only проверки
cache/history capabilities; полнота overlay/артефакта этим не установлена.
Выводы о child retirement и parent quiescence относятся к названным перечитанным
областям. Финальный drift указан отдельно с UNKNOWN, без перезаписи владельцев.

Финальный исходный срез 09:24:59Z:59/61 хешей не изменились после 09:18:05Z.
Изменились native source-lock и README при работе над cache/history.
Совпадение относится к тому времени, не к сегодняшнему дереву.
`.artifacts/research/r28-contracts/C04/final-input-drift.json`
records both hashes. Reread09:25:57Z:
`.artifacts/research/r28-contracts/C04/android-final-declaration.json`
confirms b5ebaa1/core1.13.19, Go/AAR=null, source-only flavor
`snowden-android-owner-r1-session-cache-r1-closed-history-r1`. No later moving
platform work is claimed reviewed. Z04/Z05 HANDOFF были недоступны
на срез 09:14:48Z; независимая парная сверка с ними не заявляется.

Публичный реестр содержит 34 записи:31 успешное новое получение
(включая четыре tag refs) и три использованные прежние публичные копии.
Неудачного публичного получения в этом пакете не было. Ошибка локального
имени файла не является HTTP-ошибкой.
