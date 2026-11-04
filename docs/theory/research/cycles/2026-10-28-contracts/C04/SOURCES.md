# R28-C04 — sources and provenance

Чтение 2026-10-28 UTC. Корень actual source: `D:/SNOWDEN-SYSTEM-V3`.
Source/spec/manual inference не являются artifact/native/runtime доказательствами.
Статус чтения относится к названным sections, а не всему repository.

## 1. Actual declarations и caller — SECTIONS_READ

| ID | Source / прочитанная область | Значение и предел |
|---|---|---|
| L01 | `windows/go.mod`: go directive, core/transport/sing/x/sync declarations | Go directive1.26; sing-box1.13.19, sing-quic d83826c306d7, quic-go0.59.0-sing-box-mod.4, sing0.8.13, x/sync0.20.0. Не binary attestation |
| L02 | `windows/tools/package.ps1` tags; `backend/render/render.go` HY2; `corebridge.go:456–505`; `backend/core/worker_child.go` and `engine.go` creation seam | Source composition: actual isolated worker; optional Salamander/hopping; box.New. Реальный профиль не читался |
| L03 | `windows/backend/core/worker_process.go:59–128,138–214,269–290`; `worker_protocol.go` budgets; `worker_host_windows.go:62–109,142–149`; `worker_spawn_windows.go` job flags | Existing original-handle Job/Wait/I/O/Empty barrier; failed cleanup retains owner. Engine policy processes8, commitBytes omitted. Таймер2s+4s не задаёт hard OS syscall deadline |
| L04 | `android/native/source-lock.json` (manifest); `android/native/README.md`; app build declarations | core commit b5ebaa1, AAR hash=null, Go=null, owner/cache/closed-history adaptation source-only, stock AAR incompatible. Exact adopted runtime UNKNOWN |
| L05 | `android/corebridge/.../NativeWorkerSession.kt`: initialize/start/retirement; `NativeRuntime.kt`; `configuration/NativeGraphSecurity.kt` HY2 keys | Per-process claim, retained TUN/scratch/native owner; successful local close still requires exit. Hopping/Salamander accepted as optional fields; actual use UNKNOWN |
| L06 | `android/vpn/.../WorkerProcess.kt:30–87,263–337,365–399,519–527`; `VpnSessionRepository.kt:426–443,659–677` | elapsed authority/death deadlines; independent watchdog; parent waits actual normal+retirement RPC returns and prepared-source idle. Source watchdog is not elapsed-time death guarantee |

`...` in this table expands to the matching `src/main/kotlin/org/snowden/android/<module>/` directory; full paths and hashes will be recorded in own provenance manifest. No product configuration/secrets/logs were read.

## 2. Exact public implementation sources

Public raw captures and acquisition UTC/SHA256:
`.artifacts/research/r28-contracts/C04/public-source-manifest.jsonl`.
Byte equality against local cache was confirmed for the initial four HY2/QUIC files and both x/sync files; later confirmations recorded separately.

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

Candidate LICENSEs and new HY2 client were copied from the immutable Oct26 public
captures into this C04 with **REUSED_PUBLIC_CAPTURE_2026-10-26** provenance; their
capture time is not reported as a new remote fetch. All other own raw fetches
are dated individually. Selected candidate metadata/API files were fetched anew.

### Fresh tag→commit identity

Public GitHub tag refs, FULL_TEXT_READ of returned ref/object metadata, 09:17Z:

- sing-box1.13.19 → b5ebaa1fc0f2b94256180b95468e73ef53caa27d.
- quic-go0.59.0-sing-box-mod.4 → ccd46abedd39c6f8384b1df8daf42008724958f5.
- sing0.8.13 → 7c349dacf402256d3a7029746073b05d2ead584a.
- x/sync0.20.0 → ec11c4a93de22cde2abe2bf74d70791033c2464c.

URLs, object bytes and SHA256 are in `public-source-manifest.jsonl` and
`tag-*.json`. These verify source identity, not a compiled artifact.

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

No ABSTRACT_ONLY paper used. Public API/docs pages above were read through web;
they are not claimed as local HTML captures. Primary selected source files have
local SHA256 provenance. No external endpoint/VPN probe was performed.

## 4. Baseline, recovery and limits

- QUIC-ACCOUNTING-AND-LIMITS: §§1–6 relevant accounting/clock/reuse sections, §7 future matrix — SECTIONS_READ.
- REPORT-010: §§2–5 caller/local bounds/retiring/process owner — SECTIONS_READ.
- REPORT-011: §§2–5 and §§6–8 relevant clock/power/reuse/measurement — SECTIONS_READ. Its claims are baseline to check, not accepted by this author automatically.
- 2026-10-28-DEEP-REVIEW: result table, scope limits and next source gap — SECTIONS_READ.
- R04-008/009 are not axioms and are not transferred as product facts.
- Missing guessed source filenames (`route/connection.go`, `common/bufio/copy_packet.go`, `congestion_meta2/ring_buffer.go`, `common/buf/allocator.go`, nonexistent Gradle version catalog) were corrected through targeted file discovery. They are not SOURCE_NOT_AVAILABLE for the actual trace.

## 5. Local snapshot and moving-input policy

`.artifacts/research/r28-contracts/C04/local-input-manifest.json`
records61 named files with full path/SHA256/length/mtime at09:18:05Z, after their
source reading. This is provenance; no execution, module resolver or runtime check.
`.artifacts/research/r28-contracts/C04/public-cache-equality.json`
records11 further exact matches, in addition to the initial six. Native source
lock/README/NativeWorkerSession were being edited independently during the read
window; relevant lifecycle/API regions were reread after the snapshot. Additional
source-only cache/history capability checks appeared; no conclusion about their
complete overlay/artifact implementation is made here. Child retirement and parent
quiescence claims use the named reread regions. Final input drift is recorded
separately, with moving regions/UNKNOWN retained rather than overriding owners.

Final comparison09:24:59Z:59/61 hashes unchanged after the09:18:05Z snapshot;
changed: native source-lock and README, reflecting in-progress cache/history work.
`.artifacts/research/r28-contracts/C04/final-input-drift.json`
records both hashes. Reread09:25:57Z:
`.artifacts/research/r28-contracts/C04/android-final-declaration.json`
confirms b5ebaa1/core1.13.19, Go/AAR=null, source-only flavor
`snowden-android-owner-r1-session-cache-r1-closed-history-r1`. No later moving
platform work is claimed reviewed. Z04/Z05 HANDOFF были недоступны
на срез 09:14:48Z; независимая парная сверка с ними не заявляется.

Public manifest has34 records:31 fresh successful captures (including4 tag refs)
and3 reused public captures. There were no failed public fetches in this C04.
Failed guessed local filenames are scope/discovery notes above, not HTTP failures.
