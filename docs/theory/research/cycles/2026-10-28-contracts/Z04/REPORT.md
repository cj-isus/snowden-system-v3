# R28-Z04 — QUIC/HY2: все holders и semantic matrix готового исправления

- author_status: READY_FOR_REVIEW
- evidence: [SOURCES](SOURCES.md), [HANDOFF](HANDOFF.md)
- captures: `.artifacts/research/r28-contracts/Z04/`

## 1. Ограниченный ответ

### R28-Z04-A — queue/callback bounds теряют силу при смене owner и unit

Локальные пределы корректны только для своего owner: `sendQueue=8` считает
serialized `packetBuffer`; `datagramQueue=32/128` — pointers/copies QUIC
DATAGRAM; HY2 `data=64` — application `udpMessage`; defragger — fragment groups;
sent history — packet metadata+frame references; BBR sampler — slots от первого
до последнего congestion packet number. `Pop`, socket-write и ACK/loss имеют
разные release receipts. Поэтому ни один локальный count/window/singleflight
не является global RAM/time bound без лимитов на callers, sessions, fragments,
handlers, active/retiring generations и allocator capacities.

### R28-Z04-B — готовое исправление действительно парное, но scoped

Совместимая source-пара:

- **sender:** SagerNet quic-go commit
  `f7b7e9c4c6f6ec45e4ab2b4788eca63192975d80`
  (tag ref `v0.61.0-sing-box-mod.6`, MIT);
- **controllers/callers:** SagerNet sing-quic commit
  `4ab2eceaac81e073f53b22ec72ff56aaea89a3d1`
  (go.mod требует quic-go mod.6; GPL-3.0-or-later).

Пара исправляет named mismatches: injected `OnPacketSent` получает **prior**
flight; packet identity становится per-connection across number spaces;
pre-install packets фильтруются; drop/migration могут вызвать
`OnPacketNeutered`; после ACK/loss event передаётся exact `leastUnacked`;
Brutal реализует полный Ex и monotime, pacer seed — actual initial packet size.

**Scoped verdict:** `ADAPT_AS_COUPLED_PAIR`, не drop-in и не whole-resource fix.
Brutal Ex исправляет потерянный batch dispatch. BBR получает правильный prior
flight и terminal callbacks. Но PTO `QueueProbePacket` всё ещё удаляет packet
из engine history и очищает frames без `OnPacketNeutered`/`notifyLeastUnacked`.
Следовательно, sampler record может пережить этот specific retirement до
следующего congestion event/connection disposal; готовая пара не доказывает
полный sampler/resource bound.

## 2. Actual path и holder passport

Current manifest: sing-box1.13.19; quic-go0.59.0-mod.4 commit
`ccd46abedd39c6f8384b1df8daf42008724958f5`; sing-quic pseudo-version commit
`d83826c306d7c008cafb3fd6d8ee07cbcfd656ed`; sing0.8.13. Fresh raw fetch:
11 current critical files byte-equal локальному module cache. Windows packaging:
windows/amd64 + with_quic/utls/gvisor/clash_api.

Actual non-hopping Windows/HY2 path из R04-010 подтверждён адресно:
DefaultDialer connected UDP → trackedConn (если manager) → UnbindPacketConn →
scalar Salamander → quic-go Windows basicConn → Go UDP WSARecv/WSASend.
Unbind не экспортирует SyscallConn/ReadMsgUDP/WriteMsgUDP; Windows quic-go
`newConn` возвращает basicConn. Это сохраняет границы одной UDP datagram и
исключает перенос Linux UDP_SEGMENT fast path по имени GSO.

### 2.1 Паспорт holders

Обозначения: U — original application payload bytes; D — packed HY2 DATAGRAM
payload; L≤1452 — обычный encoded QUIC packet текущего Windows пути; N_* —
максимальное одновременное количество соответствующих owners. Все формулы —
условные source-модели, не RSS measurements.

| Stage / variable | Unit | Owner | Локальный guard | Release receipt / перенос |
|---|---|---|---|---|
| Caller original `U_caller` | bytes/backing | caller приложения | MaxUDPSize4096 только на HY2 API | Возврат Write/WritePacket не доказывает немедленный GC caller buffer |
| HY2 fragments `F_frag` | `*udpMessage`, alias slices | один Write caller | U≤4096; `fragmentTotal` wire uint8; gate на C=B-H и F≤255 отсутствует | `releaseMessages`; alias не владеет original backing; caller lifetime критичен |
| HY2 packed staging `B_pack` | managed `buf.Buffer` per fragment | `writePacket` | packet size/allocator class | deferred Release после `SendDatagram` return |
| QUIC pre-Add copies `N_wait·D` | copied `wire.DatagramFrame.Data` | каждый concurrent SendDatagram caller | copy создаётся **до** Add; Add blocks when queue32 full | Add error/queue acceptance; blocked copies не входят в 32 |
| QUIC DATAGRAM send queue | frame pointers+Data | `datagramQueue.sendQueue` | 32 queued | Pop только переносит frame в packer/history; не release Data |
| Packet payload/history | frame refs+stream refs | `sentPacketHistory` | current tracked guards 40k/50k slots; distinct PN spaces | ACK/loss/remove/neuter paths; DATAGRAM `Frame.Data` удерживается history после Pop/write |
| QUIC serialized send queue | encoded buffer bytes | `sendQueue` | 8 entries; Windows non-GSO ⇒ Σencoded≤8·1452=11616 | worker Write return releases serialized buffer; **не** DATAGRAM frame Data |
| In-service + builder + Salamander | encoded/staging backing | send worker / packer / Salamander | один serialized write in service и current builder в normal scalar trace; allocator round-up | write/return/defer; kernel completion не peer/app receipt |
| Kernel UDP buffers | kernel bytes | Windows Winsock | desired sizes — requests; no engine Job memory cap | socket close/kernel completion; не Go queue receipt |
| QUIC RX datagram queue | `[]byte` copies | datagramQueue | 128 queued; copy occurs before cap check | Receive dequeues; dropped new copy becomes GC candidate, not immediate RSS proof |
| Client post-dequeue handler | goroutine+`[]byte`+message | `loopMessages` child | goroutine-per-message, no local numeric cap | handler return / connection close; upstream128 no longer applies after dequeue |
| Server post-dequeue path | message/session goroutine | serverSession | serial loop until new session, then goroutine per new UDP session | udpTimeout/canceler and close; simultaneous sessions need external bound |
| HY2 ready queue | `udpMessage`+data | per `udpPacketConn` | channel64 | Read/WaitRead; WaitRead may transfer `p.data` ownership when no headroom copy |
| Defrag groups `G_defrag` | map entry + up to255 message slots+buffers | LruCache packetID | age10s; **no WithSize** | eviction runs lazily on cache operations, callback releases messages; time alone not active timer receipt |
| RX assembly overlap | fragments+new output | `udpDefragger.feed` | no repeat total≤4096 gate | fragments released after output copy; full ready queue drops output after allocation |
| BBR sampler | RingBuffer slots of connectionState | injected controller | initial256, grows geometrically; no capacity ceiling | ACK sample does not immediately remove; event-end RemoveUpTo / neuter. `Clear/Pop` zeros slots but backing high-water remains allocated |
| Pending offer | one registered `clientOffer` | HY2 Client | one pending under mutex | done closes before discarded conn cleanup; singleflight≠retiring generation bound |
| Active/retiring generation | QUIC state+raw socket+sessions | clientQUICConnection / Windows worker | connDone marks inactive before full QUIC/raw close; platform forbids replacement after failed Stop | graceful close then raw close; Windows Job Terminate/Wait/Empty is stronger process receipt, but commitBytes=0 |

### 2.2 Условная композиционная арифметика

Для выбранного carrier, без kernel/TLS/stream/pool overhead, логические живые
payload/backing bytes удовлетворяют только параметрической форме:

`B_live ≤ B_caller + N_wait·D + 32D + B_history + 8L + B_writer + B_builder
        + B_salamander + 128D_rx + N_handler·D_rx
        + Σ_sessions(64·U_ready + B_defrag + B_assembly) + B_sampler + B_retiring`

где

- `B_defrag ≤ Σ_groups(Σparts U_part + slots·metadata)` только при заданных
  `G_defrag`, parts/group и capacities; age10s не задаёт количество groups;
- `B_sampler` зависит от PN span/high-water, а не только present records;
- `B_retiring` требует cap generations и completion receipts;
- `B_caller`, N_wait, N_handler, sessions, groups, retiring generations не
  ограничены рассмотренными queue counts.

Формула не обещает finite global number без admission assumptions. Сильнейший
контрпример: queue32 полна, N callers уже скопировали D и ждут Add; одновременно
Pop переносит 32 frame Data в history, socket queue8 удерживает другие encoded
buffers. Count32 и count8 оба истинны, но bytes находятся у разных owners.

## 3. Sender/controller semantic matrix

| Seam | Current mod.4 + d838 | Pair f7b7e9c + 4ab2ece | Verdict |
|---|---|---|---|
| Brutal batch dispatch | Brutal asserts base interface; Ex method не выбран; ackRate не получает normal batch | Brutal asserts CongestionControlEx; monotime event batch | **ACCEPT pair** для named dispatch mismatch |
| `OnPacketSent` flight | Injected callback получает post-flight; sampler добавляет L ⇒ snapshot F+2L | Injected controller получает prior F; sampler stores F+L | **ACCEPT pair**; не применять `-L` глобально к legacy controllers |
| Packet identity | PN scoped per Initial/Handshake/App spaces; controller map assumes one ordered domain | отдельный congestionPacketNumber across spaces | **ACCEPT pair** для injected controller identity |
| Install boundary | Старые packets могут пересечь SetCongestionControl без explicit reports predicate | firstPacketNumber фильтрует pre-install packets | **ACCEPT pair**; whole connection lifecycle still external |
| ACK/loss cleanup | Ex event + approximate/current lowest app PN; current BBR самостоятельно estimates leastUnacked | sender computes exact least across reported spaces after events | **ACCEPT pair**, better sampler retirement |
| Drop/migration | Drop packet spaces/path migration может убрать history без controller terminal record | `OnPacketNeutered`, controller retained across migration | **ACCEPT pair** для listed paths |
| PTO probe retirement | QueueProbePacket DeclareLost + clears frames; no loss batch/neuter/least callback in that function | то же: no terminal sampler callback visible | **UNKNOWN/NEEDS_ADAPTATION**; named residual seam |
| PMTU loss | individual controller callback skipped; current Ex list may include loss (baseline mismatch) | candidate Ex lost list append remains under reports but outside individual PMTU guard | **UNKNOWN**, pair not blanket PMTU fix |
| App-limited | current packer emptiness can overmark between writes | candidate suppresses within one RTT since network-limited | **ACCEPT source behavior**, field effect unknown |
| Pacer seed | current Brutal fixed1252; BBR old constructor clock/size seams | actual `InitialPacketSize`, monotime, profile constructor | **ACCEPT compatibility improvement**; rates/overflow still separate |
| Method set | Current BBR implements old Ex; Brutal no full Ex | both implement new Ex including neuter/lost/app-limited | **PAIR ONLY**; new controller with old sender does not get new callbacks |
| Licence/deps | quic-go MIT; sing-quic GPL-3+; existing closure sing0.8.13 | same root licences; go.mod moves sing0.9 beta4, quic0.61 mod6, x/*/utls versions | **ADAPT, no drop-in**; whole dependency/build/API closure required |

### 3.1 Partial-fix decisions

- **Backport only Brutal full Ex/monotime onto current sender:** restores dispatch
  only if current public Ex method set is implemented exactly. It does **not**
  fix prior/post flight, cross-space identity, install filtering, drop/migration
  retirement or app-limited semantics. `DEFER` as transitional experiment, not
  accepted remediation.
- **Subtract packet length inside old BBR sampler:** may repair one post→before
  snapshot, but corrupts legacy/prior callers and does not solve identity or
  retirement. `REJECT`.
- **Backport only candidate sender semantics while retaining old BBR:** old BBR
  lacks new `OnPacketNeutered` public method and its own estimated cleanup differs;
  it is type/semantic incompatible without an adapter. A no-op neuter loses
  stateful cleanup. `REJECT` as complete fix.
- **Candidate pair but ignore PTO terminal seam:** acceptable only for scoped
  dispatch/prior-flight verdict; not full resource-bound or lifecycle acceptance.

## 4. Opposing cases and admission guards

### 4.1 Strong existing guards

- Transport raw receive queue1024, per-connection unprocessed256, accept queue32,
  retry/reject queues and anti-amplification3× limit specific queue/work classes.
- `GetConfigForClient`/ConnContext can reject before connection creation in
  quic-go API, handshake idle timeout bounds inactive attempts, tracked packets
  gate sending at40k/50k, DATAGRAM queues32/128, HY2 ready64.
- Windows application uses one isolated worker owner: failed previous Stop blocks
  replacement; Job process count8 and Terminate/Wait/Empty receipts guard
  retirement. Job memory commit limit is **not set** (`commitBytes=0`).

### 4.2 Why these do not close admission

Pinned HY2 Service's `quic.Config` sets `MaxIncomingStreams=1<<60`, but does not
set `GetConfigForClient`; qtls Listen does not pass a ConnContext. The listener
starts one `handleConnection` goroutine per accepted QUIC connection. Auth is
application HTTP/3 after transport handshake; invalid password gets masquerade
response, not pre-handshake resource rejection. After auth, first new sessionID
creates udpPacketConn/map entry and handler goroutine; map size has no local cap.

Thus accept-queue32 limits **completed not-yet-Accepted** connections, not active
accepted connections or handshakes already running. Anti-amplification limits
outbound bytes before validation, not local CPU/maps/TLS state. Handshake timeout
bounds duration conditional on scheduler/timer delivery, not concurrency.
A global bound needs independent caps/rate budgets at pre-handshake, authenticated
connection and UDP-session stages plus worker/process memory monitoring.

Strong opposing case against a blanket DoS claim: address validation, handshake
idle timeout, queue drops, auth map, tracked-packet limits and Windows Job reap
all reduce exposure. No deployed remote DoS, reachable malicious peer profile,
frequency or RSS effect is established here.

## 5. Recommendation / reuse / cost

**Recommendation:** platform/dependency owner should treat f7b7e9c+4ab2ece as
one compatibility unit and adapt it only after API/build/licence closure. First
acceptance target is semantic: prior-flight, cross-space identity, install fence,
ACK/loss exact least-unacked, drop/migration neuter and Brutal Ex dispatch. Keep
PTO retirement and PMTU inclusion as explicit open gates; don't market a memory
fix or speed gain.

Cost surface:

- sing-quic go.mod moves quic-go mod.4→mod.6, sing0.8.13→0.9.0-beta.4,
  utls1.8.4→1.8.7 and x/* packages; constructors/wrappers/caller APIs change;
- sing-quic GPL-3.0-or-later, quic-go MIT; entire resulting binary retains wider
  dependency/licence obligations, not only these two roots;
- BBR sampler high-water capacity and global owner admission remain outside
  callback correctness; platform instrumentation must report owner/unit/release,
  not one global scalar;
- current Windows scalar/Salamander path remains a compatibility constraint;
  sender/controller fix does not enable GSO or alter on-wire framing by itself.

## 6. Future finite acceptance — NOT_RUN

1. **Interface/build:** exact coupled versions with full production tags; no shim
   silently no-ops neuter/lost/app-limited; whole dependency/license inventory.
2. **Semantic oracle:** F=0/F>0, STREAM/DATAGRAM, ACK-only, ACK+PING, PMTU/PATH,
   Initial/Handshake/App PN overlaps, install with preexisting packets.
3. **Terminal ownership:** ACK, threshold/time loss, key drop, Retry, PTO probe,
   path migration, controller replacement and close; compare engine history,
   frame backing, sampler present/slot high-water.
4. **Holder accounting:** queue full with N waiting SendDatagram callers;
   Add→Pop→Write→ACK/loss; RX128→handlers→session64; defrag groups255/age10 and
   assembly overlap; active/retiring generations.
5. **Admission:** bounded concurrent handshakes, accepted connections, sessions,
   per-user attempts and CPU/bytes; invalid auth/slow client/timeout variants.
6. **Platform retirement:** graceful close blocked/write error, worker terminate/
   Wait/Empty and memory accounting; no direct fallback on failure.
7. **Comparative behavior:** useful goodput, latency tails, loss classes, RSS/CPU
   and fairness under a finite registered workload; benchmark/device/live only
   under separate permission.

No tests, build, simulation, benchmark, native/network/endpoint actions or code
patches were run. All empirical and product acceptance remains UNKNOWN.
