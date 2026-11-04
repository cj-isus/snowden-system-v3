# R28-Z04 — SOURCES

Чтение 2026-10-28; source-only. `FULL_TEXT_READ` означает весь небольшой
файл/документ; `SECTIONS_READ` — только названные symbols/sections. Captured file
не считается прочитанным автоматически. URLs GitHub commit/raw — публичный
source fetch с обычной TLS verification. Tests/build/native/network NOT_RUN.

## 1. Current source identity

| ID | Pin / источник | Read depth и используемый scope |
|---|---|---|
| C1 | Snowden `windows/go.mod`, `go.sum`, `tools/package.ps1` | SECTIONS_READ: current sing-box1.13.19, quic-go0.59.0-mod.4, sing-quic pseudo d83826c, sing0.8.13; windows/amd64 production tags |
| C2 | quic-go `ccd46abedd39c6f8384b1df8daf42008724958f5` | 5 critical raw files freshly fetched and byte-equal local cache; local selected source captured. Tag/version from manifest; source commit from published baseline and exact raw equality, not EXE attestation |
| C3 | sing-quic `d83826c306d7c008cafb3fd6d8ee07cbcfd656ed` | 6 critical raw files freshly fetched and byte-equal local cache; selected source captured. Pseudo-version embeds commit/time |
| C4 | Go1.26.7 local source reference | inherited R04-010; used only Windows scalar socket/cancellation boundaries, not artifact toolchain proof |
| C5 | Windows worker source | SECTIONS_READ: `corebridge.go`, worker_process/host/spawn/protocol, engine. Current worker owner/reap and no Job memory limit |

`fresh-source-equality.json`: 11 current files, all `remote_equals_local=true`.
`local-inputs.json`:66 selected files, source-before/source-after hash equal during
each copy; serial, not atomic tree/full-module audit.

## 2. Candidate coupled pair

### 2.1 Controller/caller side

- SagerNet sing-quic commit
  [`4ab2eceaac81e073f53b22ec72ff56aaea89a3d1`](https://github.com/SagerNet/sing-quic/commit/4ab2eceaac81e073f53b22ec72ff56aaea89a3d1),
  GitHub commit API FULL metadata read 2026-10-28 around09:47Z: exact SHA,
  signature verified=true, author/commit date2026-08-28T02:24:25Z, message
  “Fix congestion control senders and remove unused implementations”.
- Raw captures under `candidate-controller/`, captured with TLS verification:
  - `go.mod` FULL_TEXT_READ: Go1.25, quic-go0.61 mod6, sing0.9 beta4,
    x/crypto0.54/x/sys0.47, utls1.8.7 indirect.
  - `LICENSE` FULL_TEXT_READ: GPL-3.0-or-later.
  - `hysteria/congestion/brutal.go` FULL_TEXT_READ: full Ex assertion,
    monotime event, no-op neuter/lost/app-limited, actual packet-size constructor.
  - `hysteria/congestion/pacer.go` FULL_TEXT_READ: seed/burst/time arithmetic;
    negative-overflow guard is not whole arithmetic proof.
  - `congestion_meta2/bbr_sender.go` SECTIONS_READ: constructor/profile,
    OnPacketSent prior-flight expectation, batch event, neuter/lost/app-limited.
  - `bandwidth_sampler.go` SECTIONS_READ: map construction256, OnPacketSent
    stores `bytesInFlight+bytes`, ACK/loss lookup, neuter, RemoveUpTo.
  - `packet_number_indexed_queue.go`, `ringbuffer.go` FULL_TEXT_READ:
    present entries versus PN-span slots; geometric growth, Pop does not shrink
    backing capacity, two far inserts hazardous comment.
  - `hysteria2/client.go/service.go` SECTIONS_READ: paired constructors and
    per-connection/session behavior.

The requested Git tag-ref `v0.7.0` initially FETCH_FAILED on TLS timeout; exact
commit bytes, signed commit metadata and candidate go.mod are available.
Therefore report names “commit4ab2ece (baseline calls v0.7.0)” rather than
claiming this run independently re-read the tag ref.

### 2.2 Sender side

- SagerNet quic-go tag ref
  [`v0.61.0-sing-box-mod.6`](https://github.com/SagerNet/quic-go/tree/f7b7e9c4c6f6ec45e4ab2b4788eca63192975d80)
  captured from GitHub API: object exact commit
  `f7b7e9c4c6f6ec45e4ab2b4788eca63192975d80`.
- Commit API read with WebFetch 2026-10-28: signature verified=true,
  author/commit date2026-08-28T02:23:55Z, exact message “Fix reporting to
  injected congestion controllers” with prior-flight, connection-wide numbering,
  install fence, neuter, least-unacked, app-limited and migration bullets.
- `LICENSE` FULL_TEXT_READ: MIT.
- `go.mod` FULL_TEXT_READ: Go1.25, x/crypto0.54/net0.56/sys0.47,
  metacubex/utls1.8.7, tooling deps.
- `congestion/interface.go` FULL_TEXT_READ: public prior-flight documented,
  Ex terminal methods.
- `sent_packet_handler.go` SECTIONS_READ: congestionPacketNumber, prior vs legacy
  post dispatch, `reports`, DropPackets/neuter, ACK/loss batch+least-unacked,
  QueueProbePacket residual, migration behavior, SetCongestionControl.
- `cc_adapter_ex.go` FULL_TEXT_READ; `packet.go` FULL_TEXT_READ.
- `connection.go` SECTIONS_READ: sendProbePacket→QueueProbePacket.

Candidate fetch manifest records paths/hashes/status. Captures are public source,
not compile/run/native evidence.

## 3. Holder/accounting current sources

### quic-go current

- `datagram_queue.go` FULL_TEXT_READ: send32, receive128, copy before receive-cap,
  Add blocking, Pop transfer, Close only signals.
- `packet_packer.go` SECTIONS_READ: DATAGRAM Pop into payload frames; frame then
  stored in sent packet metadata.
- `sent_packet_handler.go` SECTIONS_READ: current callback semantics,
  history/terminal/limits/PTO.
- `sent_packet_history.go`, `packet.go`, `ringbuffer.go` FULL_TEXT_READ:
  pointer/frame owners, holes, backing high-water.
- `send_queue.go` FULL_TEXT_READ: queue8, in-service worker, drain Close.
- `buffer_pool.go`, `protocol/params.go/protocol.go`, `connection.go` named sections:
  sizes, tracked guards, idle/close and queue owners.
- `server.go` named sections: raw1024, accept32, callbacks/retry, handshake goroutine,
  post-handshake enqueue. A queue cap is not active-connection cap.

### sing-quic/sing current

- `hysteria2/packet.go` SECTIONS_READ: caller buffers, frag aliases, channel64,
  defragger age10/no size cap, assembly overlap, ownership transfer.
- `client.go/client_packet.go/service.go/service_packet.go` named sections:
  singleflight, active/retiring, client goroutine-per-message, server accepted
  connection/session goroutines, auth after transport Accept.
- `hysteria/hop.go` FULL_TEXT_READ for optional queue1024 scope.
- `sing/common/buf` and LruCache named sections: logical capacity, Release,
  lazy age eviction and no WithSize in defragger.

## 4. Specifications and previous baseline

- RFC9221 official TXT, March2022, Pauly/Kinnear/Schinazi: §§5.2–5.4
  SECTIONS_READ in prior R04-010 captures. DATAGRAM ack is transport receipt,
  no flow-control contribution, receiver may drop, congestion controlled.
- RFC9000 official TXT, May2021, Iyengar/Thomson: §§10.1–10.2.2 SECTIONS_READ
  in prior captures: idle floor3PTO and closing/draining state scope.
- RFC9002 fetch in this run FETCH_FAILED (TLS timeout); no new claim depends on
  unread text. Existing implementation callback trace is source evidence.
- `QUIC-ACCOUNTING-AND-LIMITS.md` SECTIONS_READ, not accepted automatically:
  source claims were independently checked where used.
- R04-007 handoff and R04-010 relevant sections read; historical claims are
  comparison baseline, not current authority.
- C04 HANDOFF was not among the source-review inputs on 2026-10-28; paired
  external validation is not claimed by this package.

## 5. Failed/limited acquisition log

`spec-identity-fetch.json` records RFC9000/9002/9221 and one sing-quic tag-ref
TLS timeouts; after failures no certificate bypass/retry loop. The RFC clauses
needed here were already available as official TXT captures from scoped prior
work and were read then. Public source web fetch is not VPN/network acceptance.

No external CCA paper was used. No claim about censorship, field performance,
fairness or deployed exploit follows from this package.
