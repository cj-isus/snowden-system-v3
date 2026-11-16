# PC-06 — HY2 receive ownership and bounded reassembly

<a id="pc-v391-hy2-receive"></a>

## V2-391: source implementation, not runtime acceptance

2026-11-10. Implements part of PERFORMANCE D0/D3 and F04/F05, §4.2.
Full PC00…09, FR/TH/OFF/COM/RQ/TR and all nine TH stages remain active.

### Source findings and reuse

The pinned client created a goroutine per received DATAGRAM. LRU expiration
released fragment messages under the cache lock while feed used an item after
lookup under a different mutex; its group/byte retention was not bounded. The
existing ready queue64 and nonblocking input were useful and remain. Reuse
existing packet/read APIs, buffers, carrier/session contexts and process owner;
no second transport, native engine, dispatcher/executor or new dependency version.

Windows parent go.mod selects third_party/sing-quic through a local replace.
Baseline: sing-quic d83826c306d7, parent quic-go mod.4/sing0.8.13, existing
windows/amd64 tags with_utls,with_gvisor,with_quic,with_clash_api. The child's
go.mod/go.sum and78 production/module/licence/README source bytes were mirrored;
73 mirrors remain exact. Upstream tests/CI/Makefile/dotfiles were not copied.
The module cache and Android manifests/artifacts remain unchanged. Provenance,
versioned ceilings and source rollback: windows/third_party/sing-quic/ORIGIN.md.
No module resolution, SDK invocation or artifact authentication was performed.

### Ownership and admission

- The original carrier loop handles client DATAGRAMs synchronously. Its ready
  enqueue stays nonblocking; no goroutine/task is created per packet.
- A bounded wire view checks fixed fields, QUIC-varint/address length, fragment
  indices and payload before string/message/buffer allocation. Nonblocking
  carrier admission covers owner count and charged bytes; oversized/resource
  rejection drops a UDP packet, while malformed framing is a separate error.
- One session mutex serializes feed, bounded lazy expiry, replacement, assembly
  and close. Fragment groups never escape the owner; generic cache eviction
  callbacks and the item/cache lock-order graph are removed.
- Each carrier caps visible sessions256, RX owners4096 and charged storage8MiB.
  A session caps groups64/partial storage256KiB and retains ready64. Payload4096
  and destination2048 preserve existing protocol limits; wire fragments≤255.
- Group slot storage is reserved before allocation. Sum≤4096 is checked before
  output creation. Assembly reserves output while every part is still charged,
  then releases parts only after copying. Duplicates, pressure, total changes,
  expiration and close use the same original-message release path.
- Close cancels input, takes the session owner, frees partial groups and drains
  ready messages. A reader that already dequeued a message retains it until
  release or the existing WaitReadPacket buffer handoff. Its credit cannot be
  reclaimed by queue removal/eviction alone. External caller output lifetime is
  outside this internal ledger; zero-copy handoff was not replaced by a copy.
- Client IDs cannot overwrite a live entry at uint32 wrap; immutable sessionID
  is assigned before publication. Admission precedes session construction, and
  destruction removes only that original object. The server's existing serial
  receive path uses the same private helpers and explicit pressure handling;
  this changes no server deployment and does not bound retiring server handlers.

### Evidence and remaining implementation

6 changed/new Go files, parent manifest, ORIGIN and4 task/state/plan documents;
73 unchanged mirrors.85 snapshots (4existing81new). Manual6 original diffs plus
new receive source/ORIGIN;7 named inputs and78 cached source files hashed.
Touched-only gofmt6+1 exit0. Immutable receipts: .artifacts/memory/bootstrap-v391-hy2-receive-.

Next: checked TX fragment/retry/PMTU sizes (current header>=budget path remains
unsafe), lower QUIC pre-copy admission and session cancellation, whole-carrier
abort, downstream buffer/retiring session metadata and versioned resource
inventory. After authorization: V1/V4/V5 wire/legacy API, buffer handoff, duplicate/
out-of-order/total-change, exact pressure/release, close/read/feed and ID wrap
interleavings, CPU/RSS/fairness/throughput and original Job/Wait/Empty acceptance.
No tests/fixtures/build/vet/typecheck/parser/SDK/browser/native/network/generation. Manual source/diff/byte review and gofmt do not prove compilation, interoperability, race freedom, throughput, RSS/CPU, whole-carrier retirement or protected egress. HY2 internal RX credits exclude lower QUIC copies/queues, the current raw receive input, pool high-water and output buffers after explicit caller handoff. Visible-session caps do not attest all retiring handler metadata. PC06 and fullPC00..09/old9/rootNETGUARD001/native/OS/wholeUX/release remain OPEN;100OPEN56NOT_RUN/gates=false/TUN unavailable/schema2 unsupported.

<a id="pc-v392-hy2-transmit"></a>

## V2-392 — TX admission, fragment arithmetic and retry identity

2026-11-10, PC-06 D4 **partial source/unverified**; PERFORMANCE F03/F04/§4.2–4.3.

### Findings and compatible extraction

Old fragUDPMessage subtracted an unchecked header from budget, then used the
result in a decrement/slice loop; C≤0 could fail to progress or panic. Fragment
count was truncated to uint8 after allocation. Retry converted signed size after
subtracting3 without a bound and reused the original packetID after partial
send. Whole-message assignment also overwrote the pool referenced witness and
cloned ownership fields; fit-path fragment lists could alias the original owner.

Move existing public write implementations and pack/fragment helpers into
packet_write.go. WritePacket/WriteTo signatures, protocol fields, payload4096,
initial1197, conservative retry-3 and at most one retry remain. Existing client
and server constructors receive one original carrier send-budget pointer; public
client/service APIs, pinned module versions and RX owner/credits are unchanged.

### Source implementation

- A shared allocation-free plan checks bounded payload/address, header and
  capacity, checked ceil division and count≤255 before message/view allocation.
  Fragmentation requires C>0; a zero-byte unfragmented packet whose header fits
  can still use the direct path. No grow-until-success loop or uint8 truncation.
- One carrier-wide non-waiting staging permit precedes address/frame work.
  Ceilings8 producers/128KiB, fixed14456 charged bytes per active producer are
  versioned in ORIGIN revision2. No semaphore wait-list, timer or goroutine.
  Default packed class8192 plus bounded address materialization4224 and pointer
  table2040 are conservative local staging charges, not total memory/RSS.
- Bound known Socksaddr value/pointer, UDPAddr and TCPAddr fields before their
  known formatter; reject unsupported/custom Addr without invoking its String.
  Serialized address still respects2048. This deliberate compatibility boundary
  and nil/IPv4/IPv6/zone/domain cases need future acceptance; no universal legacy
  adapter claim. Do not apply deprecated strict domain-name heuristics.
- Retry size must be signed-positive and strictly smaller before int conversion.
  Recheck the full plan before another packet-ID reservation or fragment array.
  A short carrier metadata mutex excludes the original packetID even across
  concurrent reservations/wrap, with at most2 atomic increments and no spin loop.
  This does not solve collisions with every unrelated old16-bit packet on wire.
- Message/view fields are set individually, retaining each pool witness and no
  RX credit alias. Fragment views borrow the original payload and have distinct
  wrappers; the original message never belongs to a fragment-release list.
  WritePacket consumes/releases the original caller buffer once after views/
  packed buffers return, then refunds staging credit. WriteTo borrows input until
  actual synchronous return. No refund on timeout or a new I/O goroutine.
- Session cancellation is checked before further message/fragment allocation and
  before/after pack. This does not wake or revoke an already-entered QUIC Add.

### Evidence and next work

5Go1origin4docs,10 original snapshots (9existing1new); manual5 existing source
diffs plus new TX source.9 named inputs/6 cached sources; gofmt5+2+1 exit0.
Immutable receipts: .artifacts/memory/bootstrap-v392-hy2-transmit-. Source fork baseline/provenance and73 unchanged
mirrors remain; no module-cache edits, dependency update or artifact promotion.

Next implementation: lower QUIC pre-copy count/bytes, session-local cancellation/
write deadline and atomic admission, whole-carrier independent abort/retirement,
actual path/wrapper budget and downstream/retiring resource inventory. After
authorization, cover C≤0/F255/F256/zero payload, signed retry/nonprogress/overflow,
partial send/retry ID/16-bit wrap, pool/alias/input consumption, pressure across
sessions, custom/typed destinations and actual cancel/close/reap/interoperability.
No tests/fixtures/build/vet/typecheck/parser/SDK/browser/native/network/generation. Manual source/diff review and touched-only gofmt do not prove compile, custom/legacy address compatibility, wire/PMTU behavior, race freedom, CPU/RSS/throughput, atomic session cutoff or actual cleanup. HY2 staging excludes caller backing/aliases, lower QUIC copies/history, retiring carriers and pool high-water; entered SendDatagram/Add remains uncancelled. General 16-bit packet-ID wrap, path/wrapper evidence and full resource inventory remain OPEN. FullPC00..09/old9/rootNETGUARD001/native/OS/wholeUX/release OPEN;100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported.

<a id="pc-v393-quic-datagrams"></a>

## V2-393 — lower QUIC pre-copy admission and original storage return

2026-11-10, PC-06 F04/F06/D0/D4, **partial source/unverified**.

### Frozen pair and actual source gaps

Old SendDatagram copied before Add could wait on queue32. Context of the HY2 UDP
session did not reach that wait. RX parsed/copy-allocated before queue pressure,
then copied again. Pop handed the same frame to payload/history, so a queue-only
credit refund would be premature. Generic debug frame formatting could include
DATAGRAM contents. These are source findings, not reproduced runtime failures.

Windows parent now selects a local quic-go mod4 fork, with183 production/module/
MIT-licence/README files mirrored from the fixed cache;176 remain exact. No tests,
fixtures, synctest, mockgen, assets or CI copied. Original go.mod/go.sum/pins kept.
The existing sing-quic fork is revision3 and calls new SendDatagramContext; it and
the quic-go fork must be kept together; source/API acceptance is still OPEN.
No resolver/SDK/build/module-cache edits or artifact promotion. Provenance, units,
limitations and pair rollback are in both third_party ORIGIN documents.

### Ownership and integration

- TX reserves retained count4096/bytes8MiB before payload/entry allocation.
  Waiting producer, queue, packer borrower and history are explicit holders of
  one original copy. The existing send queue32 is a fixed ring, not the total
  retained bound. Resource pressure returns ErrDatagramSendCapacity without copy.
- Peek acquires a packer lease. Pop transfers the queue owner to a non-retransmit
  DATAGRAM frame handler or drops only that queue reference. Close cannot clear
  data while a packer borrower or original history holder remains. Close retains
  unresolved history instead of claiming that a closed queue retired everything.
- ACK/loss and known unsent local pack failure converge on the exact history
  owner once. On final reference return, clear data before refunding bytes/count;
  preserve immutable original length for metadata after lost/spurious ACK paths.
  Known local long/short-header pack failure has a deferred unsent return. An
  unknown later terminal path is not promoted into a successful retirement fact.
- RX has a fixed128-slot/2MiB queue. Only the synchronous Conn receive path uses
  a borrowed parse view; queue admission checks close/count/bytes before its
  sole copy. The previous independent-buffer parser remains. Receive explicitly
  transfers a slice to the caller; this is not proof of that caller's release.
- Preserve public SendDatagram and add SendDatagramContext. HY2 passes its
  original session context to Add wait; cancellation retires a pending original
  producer synchronously, including its copy, without a helper goroutine/timer.
  An already admitted frame remains QUIC-owned. Independent late error is retained
  with net.ErrClosed rather than erased by a concurrent session cancellation.
- Captured channels, not custom Context methods, are checked under queue locks.
  Context methods and hasData execute outside them. Close locks send then receive,
  publishes one immutable error and cannot admit a new queue entry afterward.
- Explicit DATAGRAM debug/qlog formatting emits size only. The original payload
  is not formatted, and released backing does not turn historical length into0.

### Evidence, limits and next implementation

9Go1mod2origin4docs+176 unchanged mirrors;192 snapshots(7existing185new).
Manual10 original source diffs plus owned-storage source/new ORIGIN;8 named inputs
and183 frozen cache inputs; touched-only gofmt8+2 exit0. Own copy process completed
with its183-file receipt and was observed absent before source mutation; no restart.
Receipts: .artifacts/memory/bootstrap-v393-quic-datagrams-. No product code was executed by metadata scripts.

Next: atomic session seal/enqueue fence and session-local write deadlines, then
whole-carrier independent abort/actual retirement and global retiring/resource
inventory. After authorization, cover allocation pressure before copy, pending
context/close/enqueue races, Peek/Pop/Close lease lifetime, ACK/loss/spurious ACK,
unsent/header failure, zero DATAGRAM, raw-view lifetime/tracing, legacy API graphs,
queued/retiring carrier resources and exact Windows artifact/wire acceptance.
No tests/fixtures/build/vet/typecheck/parser/SDK/browser/native/network/generation. Source review and gofmt do not prove compilation/API graph, race freedom, buffer/ACK/loss/unsent/raw-view interleavings, interoperability, CPU/RSS/throughput, whole-carrier/native retirement or protected egress. Context cancellation can race enqueue; atomic session sealing/write deadlines remain unimplemented. Queued-close does not refund live producer/packer/history; terminal history uncertainty remains retained. RX Receive is caller handoff, not actual caller release. Serialized buffers, other carriers, foreign outputs/aliases, metadata/allocator/pool high-water and global retiring inventory remain outside these per-owner byte ceilings. FullPC00..09/old9/rootNETGUARD001/native/OS/wholeUX/release OPEN;100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported.

<a id="pc-v394-session-deadline"></a>

## V2-394 — original session seal and local write deadline

2026-11-10, PC-06 PERFORMANCE §4.1A/D4 **partial source/unverified**.

### Source gap and reuse

V393 context checks woke pending Add but could race the actual enqueue. HY2
SetWriteDeadline/SetDeadline still returned os.ErrInvalid. Existing QUIC stream
deadline error, monotime representation and timer-wait patterns are reused,
without a second transport/controller or a shared raw-socket deadline.

### Original owner and enqueue boundary

- DatagramSendScope records its immutable original self/Conn/queue. Copied/zero
  values cannot send, change deadlines or refund another owner's quota. Err is
  only an observation; it never substitutes for the final queue admission check.
- Successful queue reserve/transfer holds queue.sendMx -> scope.mu. Scope Close
  seals that same mutex and wakes its changed channel. If sealing wins, transfer
  fails; if enqueue wins, the accepted frame remains QUIC-owned. No reverse lock
  acquisition: call/quota return releases scope.mu before taking queue.sendMx.
- Bound open/retiring scope slots256 and active scope Send calls8. A sealed scope
  returns its slot once only after those actual calls end. This quota does not
  replace HY2's outer producer/buffer owner or transferred queue/history credits.
  Invalid call accounting seals/wakes its scope; uncertain quota cannot grant a
  fresh scope slot.
- HY2 actual client/server constructors handle scope-creation failure before
  publication/handler spawn; partial construction closes its original scope.
  Close seals before cancel/RX retirement, and every actual packet send uses that
  scope. Existing SendDatagram and SendDatagramContext retain their legacy paths.

### Deadline and clock behavior

- SetWriteDeadline changes only the scope; SetDeadline combines it with the
  existing read deadline. Set/reset/clear/close wakes all own waiting admissions.
  Other scopes and shared QUIC/raw-socket deadlines are not modified.
- Before copy and final enqueue, validate current closure/deadline under the
  original fence. A blocked Add owns one reusable timer. Queue progress, scope
  change, context cancellation, carrier close and timer wake all recheck state.
  A stale tick after extension/clear cannot itself decide timeout. No new helper
  goroutine, deadline context per fragment or background deadline dispatcher.
- Reuse errDeadline (net.Error, wrapping os.ErrDeadlineExceeded) and monotime.
  A scope-local bridge anchors standard time.Time at one current wall/monotonic
  reference, checks signed addition/saturation and maps finite zero collision to
  an expired finite instant. Only time.Time{} means unlimited. It does not create
  trusted UTC authority, persist elapsed time across boots or change other APIs.
- Deadline is sampled at the admission decision. Timer/scheduler lateness,
  already accepted frames and raw socket I/O are not an exact wall-time expiry
  barrier. Close seals under its owner mutex but does not await active calls or
  prove native/process retirement; original owners remain until actual return.

### Evidence and remaining work

7Go2origin4docs,13 snapshots(12existing1new); manual8 existing source diffs plus
new scope source.11 named inputs/4 cached references; touched-only gofmt7+1exit0.
261 cached originals and249 verbatim mirrors preserved. Paired quic-go revision2/
sing-quic revision4, exact upstream versions/licences and module manifests remain.
Receipts: .artifacts/memory/bootstrap-v394-session-deadline-. No product modules were loaded by bookkeeping scripts.

Next: whole-carrier independent abort and original socket/writer retirement,
hopping/close lock graph and global retiring/output/path resource inventory.
After authorization, cover both seal/enqueue orders, queue-full deadline set/
extend/clear, stale timer tick, copied/zero/foreign owner, constructor failure,
closed scope with active Send, quota reuse/pressure, finite/extreme/zero clock
conversion, neighboring sessions and legacy API/wire/exact Windows acceptance.
No tests/fixtures/build/vet/typecheck/parser/SDK/browser/native/network/generation. Source review/gofmt do not prove compile, scope/close/enqueue/deadline/constructor interleavings, clock/suspend/OS behavior, wire/legacy compatibility, resource fairness, throughput/RSS/CPU, native retirement or protected egress. The fence governs scoped admission only; legacy context-only APIs remain separate. Accepted frames are not revoked, and Close is not an actual-call/whole-carrier join. Scope quota covers its own Send calls, independently of HY2 outer writers, receiver outputs and transferred frame storage; no total/global memory or hard expiry SLO. FullPC00..09/old9/rootNETGUARD001/native/OS/wholeUX/release OPEN;100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported.

<a id="pc-v395-hop-retirement"></a>

## V2-395 — original hopping socket and close ownership

Status: PC-06 source/unverified. Windows sing-quic revision5 retains the exact
quic-go revision2 pair and module versions. The shared Hysteria hop wrapper has
two actual Windows callers, Hysteria1 and HY2; neither caller/server was executed.

### Defect and implementation

- Existing WriteTo held access over the foreign socket write; Close/deadlines
  needed the same mutex. Repeated Close returned nil while an earlier close could
  remain pending. Receive buffers, hop/late dial and detached old Close had no
  common actual-return join. Queue contents were not drained on shutdown.
- Existing public API remains. New private hop_lifetime.go owns original socket
  Close results and bounded call registration; no second QUIC owner or timer
  wrapper is introduced. Every external Write/Read/control/LocalAddr/SyscallConn
  callback runs outside access, retaining its exact Go socket object.
- First Close seals admission/queue publication under access, starts every
  retained current/previous/pending/retiring original Close independently, then
  one coordinator waits for all observed close results. One sync.Once task per
  original socket; repeated Close waits the same result, with no retry/ErrClosed
  substitution and no goroutine per waiter/timeout.
- The coordinator also joins the original hop task, late/partial dial cleanup,
  all registered receive tasks and admitted public calls. Only then may it drain
  the receive queue. A read already dequeued its buffer keeps its registered call
  through copy/release. A send still retains its caller buffer until actual return.
- Receive publication and shutdown share access. No WaitGroup Add after closure;
  receiver registration precedes publication/spawn. Abnormal callback return
  (including Goexit) sets a negative sticky result; internal task panic is contained
  without exposing payload/credentials or claiming successful native cleanup.
- The one hop task retains pending/retiring sockets while preparing or closing
  them. It rejects publication after shutdown and closes a late socket before
  hopDone. A negative old/discard close seals further activity and preserves the
  original socket/result. No replacement is granted by a timeout.
- Separate control serializes cached deadlines/buffers and new socket publish.
  Order: control -> access; call admission releases access before control wait.
  Deferred control unlock also covers abnormal prepare return. Socket abort takes
  access alone, never control. Candidate settings finish before publication;
  failed prepare discards its original candidate without replacing current.
- Public setters retain upstream current-socket outcome semantics. Previous and
  pending setters do not assert cleanup; already-retiring sockets are not queried
  for live control results. A returned RawConn remains the Go socket's API object;
  its external borrowers are outside wrapper-call accounting, no numeric FD cached.

### Engineering ceilings and exclusions

| Scope | Source ceiling |
|---|---|
| Public calls |16 before admission, including bounded control waiters; no semaphore wait-list |
| Receive generations |3 checked before next dial; register before spawn, retain until actual read-loop return |
| Hop task |1 including candidate prepare and late/partial dial cleanup |
| Original closes |1 task per socket, up to3 simultaneously live/pending/retiring sockets |
| Packet storage |Existing1024 queued buffers of2048 bytes, up to3 in-progress receive buffers and up to16 dequeued public-reader buffers within the shared call16 ceiling |
| Ports |At most65536 uint16 values, copied once; nonempty/nonzero, nonnegative interval;0 uses30s default |

These bounds are wrapper-local. They do not count external RawConn borrowers,
caller backing, pools/allocator, socket/kernel queues, other carriers, accumulated
global retiring clients or process RSS. Fresh exclusive original sockets are an
actual dialer contract, not inferred from net.PacketConn interface identity.
The legacy dial callback has no cancellation argument and may not return; foreign
Close/I/O/control can also remain blocked. Close then waits rather than reporting
native retirement or granting a successor. No hard Stop wall-time claim is made.

### Evidence and remaining work

2Go1origin4docs;7 snapshots(6existing1new), manual2 existing source diffs plus new
lifetime helper, touched-only gofmt2+2exit0.9 named inputs/2 cached references;
261 cached originals unchanged,248 still-verbatim mirrors unchanged, original
hop baseline matches the archived upstream hash and is explicitly recorded as a
new local modification. Receipts: .artifacts/memory/bootstrap-v395-hop-retirement-. No product module invoked by bookkeeping.

Next whole carrier: retain original failed/discarded offer and initializer cleanup,
start independent QUIC/raw abort before sendQueue join, keep successor denied until
actual retirement, and bind legacy later-hop dial cancellation to that owner.
Also fix existing read-timeout loop exit/deadline-reset semantics and vectorised
Salamander input mutation/indexing/ownership against exact writer consumers.
After authorization, accept close vs blocked Write/control/dial, late candidate,
partial error, abnormal return, concurrent Close, queue drain/read borrower,
receiver-generation pressure and both actual clients on exact Windows inputs.
No tests/fixtures/build/vet/types/parser/SDK/browser/native/network/generation. Source review/gofmt do not prove compile, close/hop/control/public-I/O interleavings, PacketConn/RawConn compatibility, wire/runtime behavior, fairness, speed, memory or native retirement. Legacy dial callback cannot be canceled by this wrapper; actual foreign calls may remain blocked, so no hard Stop SLO. Fresh exclusive socket transfer is a dialer contract. Call/receiver/queue ceilings exclude external RawConn borrowers, input/pool/kernel backing, other carriers and global retiring owners. Initial partial-dial cleanup custody, whole-client offer/discard retention and independent QUIC/raw abort integration remain open. Existing recv timeout termination/reset and vectorised Salamander TX remain separate defects. FullPC00..09/old9/rootNETGUARD001/native/OS/UX/release OPEN;100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported.

<a id="pc-v396-carrier-retirement"></a>

## V2-396 — original HY2 carrier, independent abort and retained offer

Status: PC-06 source/unverified. Windows sing-quic revision6 and quic-go revision3
retain frozen upstream versions/licences and both local replaces. HY2 requires
the paired Closing API as well as NewDatagramSendScope. Hysteria1 carrier logic
is unchanged. No server, platform SDK, module cache, generated code or other role
was edited or executed.

### Source defects and actual ownership

- Old CloseWithError cleared Client.conn before cleanup, ignored close errors,
  and called QUIC Close before raw Close. completeOffer cleared pending/published
  done before discarded cleanup. A new carrier could replace an uncertain owner.
- Existing HTTP3 Transport.RoundTrip hides async Dial and can return request
  cancellation before its pointer-writing dial returns. That pointer could be
  sampled during cleanup without the original initializer's completion.
- New client_lifetime.go creates the original carrier before the shared offer.
  One initializer attaches each raw/QUIC/standard Transport close task before
  publication. Attach after seal starts that same original task immediately.
  initDone prevents the coordinator from missing late resources/task registration.
- HY2 now performs one explicit QUIC dial and builds the existing HTTP3 ClientConn
  over that exact connection. No automatic HTTP3 auth redial/retry or mutable
  async result pointer. Standard TLS retains explicit quic.Transport before dial;
  optional qtls.Config retains its existing DialEarly capability. TLS is cloned
  and ALPN set to h3. Provider-specific CreateTransport behavior, connection-ID
  differences and wire compatibility require separate future acceptance.
- The first owner close records cause/seals connDone, starts every known raw,
  QUIC and standard Transport close task independently, then cancels the carrier
  and joins. No carrier/client mutex is held over those foreign Close calls.
  One original task/result each; panic/Goexit is negative, not an ErrClosed retry.
- The original initializer finishes before cleanup snapshot. Own initializer-
  timeout/RX/connection-watch tasks are registered before initDone and joined;
  they request close without waiting for their own join. UDP map capture occurs
  after connDone, under the existing constructor mutex, then session close/drain
  occurs outside that mutex. Known original close results are aggregated.
- Failed/discarded pending result is published only after recorded cleanup.
  Negative cleanup retains Client.pending even after its result channel closes.
  Active reset sets retiring under connAccess before foreign work; Client.conn
  remains until known cleanup. An inactive/retiring carrier cannot be replaced
  while cleanup waits, and a negative outcome cannot be retried into a new slot.
  Public CloseWithError waits the original result instead of returning nil early.

### Internal QUIC errors and handshake timeout

- QUIC Context cancellation follows sendQueue.Close in run. Merely watching
  Context therefore cannot break a blocked raw write during an internal failure.
  New Closing closes at the first original closeErr CAS, before send-queue drain.
  It introduces no timer/goroutine/callback and preserves Context semantics.
- HY2 active/watch observes Closing and starts the independent raw abort. It is
  an early failure observation, not packet/native retirement or permission to
  replace a carrier. The original close tasks/results still govern cleanup.
- Shared15s initial deadline covers UDP dial, QUIC and auth rather than only
  the final request. One registered observer requests abort of already attached
  resources even while the initializer has not returned. Caller timeout only
  stops that caller's wait; it does not cancel the shared attempt.
- requestClose(initializingOnly) and finishInitialization share lifeAccess.
  If cancellation wins, final success cannot publish. If initialization wins,
  canceling the completed offer cannot later revoke the accepted carrier.
  Later hops and UDP use the carrier's own context, canceled on retirement.
  Uncooperative calls still retain the owner; no timeout manufactures completion.

### HY2 credits and source ceilings

- Existing RX/TX reserve and new seal use their original budget mutex. New work
  is refused after seal; no queued/borrowed writer/reader credit is refunded.
  Cleanup waits for actual HY2 staging/message credits, including dequeued
  readers, or a negative poisoned accounting result. Poison never grants a
  successor even if outstanding storage cannot be counted safely.
- Per Client there is one pending or active/retiring original carrier, one
  initializer, at most3 original close tasks and3 own observers/RX tasks, plus
  one coordinator during closure. Existing HY2/Hop/QUIC limits remain separate.
  This is not a global client cap, TCP/HTTP3 task bound, allocator/RSS limit or
  a guarantee that all native/QUIC storage has returned.

### Evidence and remaining work

6Go2origin4docs;12 original snapshots(11existing1new). The two QUIC snapshots and
early-close scope were appended before editing them at own checkpoint984;
initial snapshot/pre-checkpoint982 and progress983 are retained. Manual7 existing
source diffs plus the complete new lifetime helper; gofmt5+2+4+2exit0.15 named
references include the original connection.go subsequently promoted to owned
source with an exact matching before hash;14 remain unchanged.2 cached references,
261 cached originals and248 still-verbatim mirrors remain unchanged, including
the prior locally changed hopping baseline. Receipts: .artifacts/memory/bootstrap-v396-carrier-retirement-.

Next: publish original UDP socket custody before wrapper construction and retain
initial Hop partial-close results; fix Salamander vectorised input mutation/
indexing/consumption and existing read-timeout reset. Then close the remaining
custom transport/HTTP3 internal task/TCP/lower QUIC last-holder and global quotas.
After authorization accept close during each init phase, internal close with a
blocked raw write, both timeout/init fence orders, late attachments, canceled/
failed offers, concurrent reset/offer/Close, poisoned vs actually drained budgets,
read/write borrowers and real provider/ALPN/wire/native Windows behavior.
No tests/fixtures/build/vet/types/parser/SDK/browser/native/network/generation. Manual source/gofmt do not prove compile, close/attach/initializer/offer/timeout/credit interleavings, native retirement, provider/ALPN/connection-ID/HTTP3/wire compatibility, CPU/RSS/speedup/fairness or Stop SLO. Raw wrapper construction before attachment and initial Hop partial-error cleanup still need earlier original-socket custody. Optional custom qtls transports, HTTP3 internal tasks, TCP borrowers, lower QUIC last holders and process-wide/carrier quotas are outside this join. Existing read-timeout/reset and vectorised Salamander TX defects remain open. A negative ledger settlement does not imply zero live credits; it retains the old owner and denies replacement. FullPC00..09/old9/rootNETGUARD001/native/OS/UX/release OPEN;100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported.

<a id="pc-v397-socket-salamander"></a>

## V2-397 — original UDP custody before wrappers and Salamander ownership

Status: PC-06 source/unverified. Windows sing-quic revision7 retains quic-go
revision3 and both frozen versions/manifests/licences. This slice extends HY2's
V396 owner; no server/native/SDK/browser/network activity was performed.

### Original socket acquisition and transfer

- Cached NewUnbindPacketConn calls RemoteAddr in its constructor. The old
  factory also built the Salamander/vector writer before attaching raw ownership;
  those callbacks could block/panic while Close had no socket to abort.
- New client_socket.go reserves one opening slot under lifeAccess before actual
  DialContext. No wait-list or overwrite. A returned original net.Conn receives
  its once-only clientCloseCall before RemoteAddr/Unbind/vector constructors.
- Carrier close snapshots this known opening abort alongside raw/QUIC/Transport
  calls. A late socket observes closing at attach and starts its original Close
  immediately. No lifetime mutex is held over dial, metadata or native callbacks.
- A nonnil socket plus dial error is closed by its original guard before returning
  an error-only result; it is not handed to Hop for anonymous partial cleanup.
  Known successful Close may clear the opening slot; negative/abnormal cleanup
  retains the slot/socket/result and poisons the carrier, denying replacement.
- Successful transfer and carrier sealing share lifeAccess. Transfer retains the
  same guard inside the wrapper and existing initializer/Hop/raw ownership chain.
  The initializer/Hop original task join covers the interval before outer adoption.
  Repeated wrapper closes reuse the original native call/result, never ErrClosed
  as a substitute. Retirement joins the opening callback and actual original close.
- The guard exposes syscall.Conn only if the original net.Conn explicitly does;
  it does not synthesize a raw capability by unwrapping a proxy adapter. Existing
  outer Upstream/RawConn contracts still need platform/provider compatibility.

### Salamander writes, input and original close

- Existing vector WriteTo XORed caller input in place. Existing multi-buffer XOR
  indexed each buffer using the aggregate offset and used a mis-parenthesized key
  index; second/later buffers could be corrupted or panic. Header/input release
  was delegated inconsistently to an external consuming vector writer.
- New salamander_write.go creates one owned ciphertext packet, preserving the
  original password+8-byte salt/XOR wire construction. Password is copied once.
  The plaintext source is read without mutation; cross-buffer offset advances
  continuously modulo32, with local output indices for each source segment.
- WriteTo now shares the safe base implementation for both wrapper variants:
  one packed buffer via the original PacketConn.WriteTo, no Addr.String conversion
  introduced by this wrapper. A nil-error short underlying write is an error.
- The consuming WriteVectorisedPacket API releases original inputs itself,
  including known validation/admission errors. Its output is one unmanaged buf.As
  view of owned ciphertext. The cached Windows writer can return a Getsockname
  error before its ReleaseMulti defer; neither that path nor normal downstream
  ReleaseMulti can free the managed packet or release original input twice.
- Original input release and downstream actual return precede successful output
  pool/slot reuse. Abnormal callback/cleanup return retains the ciphertext slot,
  poisons further writes and starts original raw abort. It never frees a buffer
  that a foreign writer could still hold or grants fresh capacity by timeout.
- Salamander Close seals new writes, starts one original underlying close before
  waiting for writers, and returns the same original result on repeat. Poison
  settles negatively even if storage/native custody cannot be proved; the upper
  V396 owner retains that negative wrapper and denies a new carrier.
- ReadFrom skips packets shorter than the salt rather than returning salt bytes
  as application data. Salt-only packets decode to an empty datagram. Small read
  buffers and impossible underlying n are explicit errors; no authentication or
  path-health proof is inferred from obfuscation decode.

### Engineering ceilings and exclusions

| Scope | Source ceiling |
|---|---|
| Opening UDP work |1 reservation per carrier before external dial, retained through construction/negative cleanup |
| Original native Close |Once per returned net.Conn guard, shared through wrapper/Hop/retirement |
| Salamander staging |8 non-waiting slots per wrapper, each at most65535 logical packet bytes |
| Default staging classes |8x64KiB =512KiB ceiling under inspected default allocator; not pool high-water/RSS |
| Vector inputs |At most256 accepted pointers; checked aggregate payload at most65527 before clone |
| Wire overhead |8 random salt bytes; no initial size is asserted to be validated path MTU |

The input list/backing, caller work, password/hash scratch, pool/allocator metadata,
RawConn borrowers and kernel queues are outside this staging ledger. Refusing an
oversized vector list still releases its transferred inputs; that cleanup work is
outside the accepted-vector ceiling. Coalescing/changed WriteTo dispatch requires
throughput and adapter acceptance; no speed gain or universal API compatibility
is claimed. Arbitrary downstream retention after a normal return violates the
synchronous borrowing contract and is not resolved by a pool/reference counter.

### Evidence and next work

5Go1origin4docs;10 original snapshots(8existing2new). Manual4 existing source diffs
plus both complete helpers; touched-only gofmt5+1+1exit0.10 named inputs/7 cached
contracts plus2 allocator source observations.261 cached upstream originals and
247 still-verbatim mirrors preserved; original Salamander baseline matches its
archived upstream SHA and is explicitly promoted to a local modification. Prior
hop modification remains unchanged. Receipts: .artifacts/memory/bootstrap-v397-socket-salamander-.

Next PC-06: fix hopping read-timeout/deadline reset on the original receive loops;
then close custom transport/HTTP3 internal task/TCP/lower QUIC last-holder/global
ownership gaps. Generic Hysteria1 acquisition is not migrated by this HY2 slice.
After authorization accept blocked RemoteAddr/Syscall constructor, late dial,
partial/error/panic Close, both transfer/seal orders, repeated Close, random salt,
zero/oversize packets, XOR across32-byte and vector boundaries, nil/large/aliased
input cases, downstream early error/normal/abnormal return and writer pressure.
No tests/fixtures/build/vet/types/parser/SDK/browser/native/network/generation. Source review/gofmt do not prove compile, socket/cancel/transfer/partial-close/wrapper interleavings, syscall/provider/adapter compatibility, UDP/Salt/XOR/wire/entropy behavior, fairness, native retirement, CPU/RSS/throughput or Stop SLO. Downstream vector writer must honor synchronous borrowing; arbitrary hidden native work, invalid caller ownership/aliasing and custom allocator contracts are not proved. Staging bounds exclude input/password/hash backing, input-list rejection cleanup work, pool high-water, RawConn borrowers and kernel storage; poison is a negative retained result, not zero live storage. Generic Hysteria1/custom/HTTP3/TCP/lower QUIC/global owners and hopping read-timeout reset remain open. FullPC00..09/old9/rootNETGUARD001/native/OS/UX/release OPEN;100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported.

<a id="pc-v398-hop-read-deadline"></a>

## V2-398 — resettable Hop read deadline on the original receive owners

Status: PC-06 source/unverified. Windows sing-quic revision8 keeps quic-go revision3
and the same frozen versions/licences/manifests. Shared Hop has actual Hysteria1
and HY2 callers; this does not migrate Hysteria1's carrier lifecycle or prove it.

### Defect and owned queue deadline

- Previous SetReadDeadline/SetDeadline reached native receive loops, which
  returned permanently after timeout. Clearing/extending the deadline could not
  resume them. A shared error mailbox could also carry a stale timeout from a
  previous socket into a later read. Sixteen blocked reads exhausted control
  admission used by the deadline setters themselves.
- New hop_read.go owns the public read gate at the existing bounded queue.
  SetReadDeadline updates local monotonic deadline under access and broadcasts a
  changed channel. It never sets a native read deadline, replaces a receiver or
  creates a cancellation/timeout goroutine. New sockets inherit write settings;
  the common wrapper read policy already covers every receive generation.
- SetDeadline applies local read first, then the native write control. A write
  admission/callback error is returned; read may already be applied, with no false
  no-effect/atomic-pair claim. Standalone write setters keep existing original
  current-socket result semantics.
- Public read checks closure/deadline and dequeues under the same access mutex.
  Expired reads leave queued packets available for a later clear/extension.
  A data readiness channel(cap1) is a hint; it is reissued while data remains.
  A dequeued reader retains its buffer/call through copy and actual release.
- One reusable Timer per admitted waiting reader, at most16; Stop/drain/reset
  follows the existing scoped DATAGRAM timer pattern. Data/change/close/timer
  wakes all recheck current state. An old tick after extend/clear cannot itself
  decide timeout. Unlimited reads do not create a timer. No AfterFunc per update.
- Cached pipe.Deadline was inspected first. Its separate mutex/AfterFunc channel
  cannot supply the shared queue-dequeue fence and no-callback timer ownership
  used here; it remains unchanged. Existing monotime/public API and the local
  DATAGRAM bridge pattern are reused rather than adding another clock domain.
- One current wall/monotonic reference bridges standard time.Time; signed add
  saturates and finite zero maps to an expired finite value. Only Time{} means
  unlimited. Remaining duration checks ordering and unsigned signed-domain distance
  before clamping, without relying on overflowing monotime.Sub/Until arithmetic.
  No trusted UTC, persisted boot clock or exact scheduler/expiry SLO is implied.
- Timeout implements net.Error and unwraps os.ErrDeadlineExceeded. Caller loops
  still must handle an expired deadline. Inspected QUIC Transport.Close marks its
  transport closed before using a past read deadline to wake its listener; no
  native receiver retirement is inferred from that logical wake.

### Native fault, control and actual close ownership

- Returned native I/O failure is retained with its exact current hopSocket in a
  single slot. Previous/retiring errors are ignored; a fault that became old after
  publication cannot retire the new current socket. Queued packets are consumed
  before delivering a still-current native fault.
- Fault selection and close seal now occur under the same access fence as hop
  publication. If fault sealing wins, candidate publication refuses; if publish
  wins, old fault is discarded. Native close tasks still launch outside the lock,
  all independent before joins. No new fault/timeout worker or per-packet task.
- A returned I/O error is delivered to the reader and initiates existing close
  without setting the cleanup-uncertain failure. Original negative Close/abnormal
  results remain negative. A known network failure cannot permanently poison an
  otherwise known successful cleanup merely by sharing one error field.
- Foreign control has a separate non-waiting quota2; IO retains16. Both register
  in the existing original call WaitGroup before closure. Control->access order,
  no lock over external callbacks, original hop/receiver/call join and stable queue
  drain remain. Pure read metadata is bounded inline under access and needs no
  foreign slot, so16 waiting reads cannot stop its reset/expiry/clear update.

### Engineering ceilings and exclusions

| Scope | Source ceiling |
|---|---|
| IO/public readers |16 original calls, each retaining dequeued backing until actual release |
| Foreign control |2 calls/waiters in their own lane, joined by the same original owner |
| Read timers |At most1 per admitted read, at most16; no callback/timer per setter |
| Readiness/error |1 buffered readiness hint;1 original-socket I/O fault; no error mailbox |
| Native receive |Existing3 generations and original Hop task; no respawn on read timeout |
| Packet queue |Existing1024x2048-byte buffers plus3 native and16 dequeued reader buffers |

These are wrapper-local structural ceilings, not global/RSS/kernel limits. A
deadline decision is sampled under dequeue lock; bytes/copy may return later.
External RawConn/native deadlines are not silently cleared/adopted. Native Close,
custom adapters and caller-created goroutine pressure still have their own scope.

### Evidence and next work

3Go1origin4docs;8 original snapshots(7existing1new). Manual3 existing source diffs
plus complete hop_read.go; gofmt3+2exit0.13 named inputs/1 cached pipe contract;
261 cache originals and247 still-verbatim mirrors preserved. Prior owned Hop
baseline is archived before this change; Salamander's prior owned bytes remain.
Receipts: .artifacts/memory/bootstrap-v398-hop-read-deadline-. No product module was executed by bookkeeping.

Next: inspect metadata/LocalAddr after close (current closed admission returns
nil), retain safe address/RawConn custody without introducing a foreign callback
before socket adoption; migrate Hysteria1's original carrier owner. Then complete
custom transport/HTTP3 internal task/TCP/lower QUIC/global last-holder boundaries.
After authorization accept expire/extend/clear with queued/empty data, stale tick,
all16 readers plus controls, both native-fault/publish orders, current/previous
socket errors, concurrent Close, partial SetDeadline write error, zero/extreme/
wall-only deadline bridge, actual QUIC listener shutdown and provider behavior.
No tests/fixtures/build/vet/types/parser/SDK/browser/native/network/generation. Source/gofmt do not prove compile, queue/deadline/reset/fault/hop/close/control interleavings, native/clock/suspend behavior, adapter/provider/wire compatibility, fairness, CPU/RSS/latency or Stop/expiry SLO. Deadline is sampled at dequeue decision, not an exact wall-time barrier. External native/RawConn deadlines are not cleared or adopted. A caller repeatedly reading an expired deadline must handle its timeout; no claim about arbitrary caller loops. Inline metadata excludes caller-created goroutine pressure; buffers/pools/RawConn/kernel and global owners retain separate bounds. Hop LocalAddr after close currently returns nil and needs caller compatibility/metadata custody work. Hysteria1 carrier/custom/HTTP3/TCP/lower QUIC/global/native/OS/UX/release remain open. FullPC00..09/old9/rootNETGUARD001/100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported.

<a id="pc-v399-packet-metadata"></a>

## V2-399 — собственные метаданные и исходные RawConn callbacks

Статус: PC-06 source/unverified. Windows sing-quic revision9 и quic-go revision4;
замороженные версии, лицензии, manifests и родительский dependency graph сохранены.
Новый std-only packetaddr переиспользуется обоими локальными модулями.

### Реализованные изменения

- Hop.LocalAddr раньше возвращал provider-owned net.Addr и nil после закрытия.
  Теперь у каждого исходного hopSocket отдельный immutable Snapshot. Первый
  getter/capture учитывается как операция этого сокета; наблюдённый кэш можно
  читать после Close без provider. Адрес старого поколения не подставляется
  вместо неизвестного нового. Два параллельных capture сохраняют первый снимок.
- Для стандартных UDP/TCP/IP/Unix копируются поля; IP хранится в массиве16,
  порт0…65535, IP длиной0/4/16. Zone/text≤2048, network≤64, Unix name≤4096,
  control characters отклоняются. Строки клонируются после ограничения длины.
  Addr выдаёт свежий стандартный объект/IP либо immutable opaque value.
  Opaque не становится UDP и не получает native capability по строке адреса.
- Custom Network/String вызываются только в исходной metadata operation;
  ограничение нашей копии не ограничивает аллокации, blocking/panic внутри
  provider. Неизвестный/invalid/typed-nil стандартный адрес не выдумывается.
- Metadata admission2 не расходует IO16/control2 и регистрируется в прежнем
  call WaitGroup под access до closure. Чтение готового кэша не вызывает чужой
  код и не требует foreign-call slot. Close сохраняет прежние actual joins.
- SyscallConn возвращает wrapper над исходным Go RawConn. Control/Read/Write
  регистрируют операцию у exact hopSocket; до callback delivery проверяются
  current generation/closure. Stale poll завершается без вызова пользовательского
  callback. Пустой callback/RawConn отвергается; nil-result без callback считается
  отрицательной неопределённостью. Abnormal return инициирует прежний close.
  Ни один внешний callback не выполняется под access; numeric FD не сохраняется.
  Уже допущенный callback может закончиться после seal. Синхронный контракт
  syscall.RawConn и запрет caller на удержание FD остаются необходимыми.
- QUIC wrapConn захватывает local metadata до native setup, отказывает invalid
  адресу и nil RawConn. Basic/OOB rawConn декорируются без изменения интерфейса
  capabilities/read/write/deadline/Close. Windows newConn сохраняет basic path;
  другие OS OOB constructors могут выполнять собственные provider getter.
- newSendConn возвращает (*sconn,error), хранит собственный local Snapshot,
  сохраняет packetInfo IP substitution только для непустого UDPAddr и отказывает
  invalid адресу до OOB/sender allocation. LocalAddr отдаёт новую копию.
  Все3 production caller просмотрены: dial, server constructor, path switch.
  Server проверяет до создания context/tracer; path — до изменения MTU/history.
  Старая очередь закрывается до замены sender; новая goroutine держит nextQueue,
  не перечитывает изменяемое поле c.sendQueue. Полный path-manager join не заявлен.

### Граница применимости Hysteria1

windows/backend/metadata/channel.go принимает только Protocol vless/hysteria2,
render.go имеет соответствующие ветки. Cached sing-box include/quic.go при этом
регистрирует Hysteria1 и HY2; библиотечные H1 consumers Hop остаются в fork.
Это доказательство границы текущего валидируемого channel/renderer пути,
а не невозможности любого generic/internal вызова H1. Новый legacy transport
не активировался. Предыдущая задача H1 carrier lifecycle/reachability остаётся
OPEN; её нельзя закрыть по одному факту validator rejection.

### Область ограничений и остаток

Hop сохраняет IO16/control2/metadata2: максимум20 зарегистрированных операций
этих трёх lanes. Кэш ограничен исходными socket owners; внешние RawConn wrapper
объекты, caller goroutines, allocator/pools и kernel не входят в глобальный лимит.
Ненаблюдённый адрес закрытого Hop остаётся nil и отвергается новым QUIC capture.
Snapshot описывает наблюдённый исходный сокет, не свежий маршрут/egress/trust.
RemoteAddr aliases и синхронизация всего path manager не менялись. RawConn bypass
в Transport.init, нестандартные провайдеры, HTTP3 internal/TCP/lower QUIC/global
last holders и native/OS границы требуют следующего адресного разбора.

15 исходных snapshots(13existing2new),9Go2origin4docs. Вручную просмотрены9 existing
Windows diffs плюс оба новых helper; touched-only gofmt9 exit0. Сохранены261
cache originals и243 verbatim mirrors;4 прежних QUIC mirrors переведены в owned
changes с исходными SHA, prior owned Hop baseline архивирован, Salamander сохранён.
14 named inputs и2 cached contracts без изменений. Receipts: .artifacts/memory/bootstrap-v399-packet-metadata-.
Source/unverified. No tests/fixtures/build/vet/types/parser/SDK/browser/native/network/generation. Manual source/diff/byte review and touched-only gofmt do not prove compilation/API graph, callback/hop/close/path interleavings, native/provider/wire/legacy compatibility, Stop SLO, CPU/RSS/throughput, current protected path or global retirement. Opaque Network/String may allocate/block/panic before our clone; standard synchronous RawConn FD rules still required, no hostile async-provider or retained external descriptor guarantee. Unobserved closed Hop local metadata stays nil; snapshot is one observed original socket fact, not continuously observed current path. Remote aliases, rawConn-bypass initialization, non-Windows OOB provider calls, path-manager/global/custom/HTTP3/TCP/lower QUIC/OS/UX/release remain OPEN. H1 current validator rejection is not universal unreachability proof; legacy compiled library lifecycle retained. FullPC00..09/old9/rootNETGUARD001/100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported.

<a id="pc-v400-http3-ownership"></a>

## V2-400 — исходные HTTP/3 задачи, callback и HY2 abort

Статус: PC-06 source/unverified. Sing-quic revision10/quic-go revision5,
замороженные версии/licences/manifests сохранены. Новый helper находится в
существующем http3 package; методы старого QUIC Close не заменены новым receipt.

### Задачи и владение

- Client task owner создаётся до constructor workers. Nonwaiting admission64
  предшествует worker spawn и stream context callback registration. Счётчик
  включает registered RoundTrip/OpenRequestStream/raw handlers, settings,
  accept/uni/datagram/cancel/body workers и обещанные stream callbacks.
  Общий mutex ограждает admission/seal; нет очереди ожидания, нового timeout
  worker, fake completion или возврата capacity по истечению срока.
- Dial/NewClientConn используют общий owned accept loop; raw external handlers
  регистрируются синхронно. Два TrackStream caller обрабатывают ошибку; перед
  newStateTrackingStream/AfterFunc зарезервирован исходный callback permit.
  Он удерживается до возврата callback, а не до удаления stream из map.
- Group completion ожидает Closing, seal и фактический возврат всех своих
  операций. Повторный wait читает тот же итог. Abnormal callback/task return
  инициирует QUIC close и сохраняет отрицательный результат. Известный overload
  возвращает отдельную admission error/закрывает перегруженный HTTP3 connection,
  но не отравляет успешный cleanup: capacity после actual retirement допустима.
- Request cancellation watcher также наблюдает Closing. Отказ запуска body/
  cancel worker отменяет обе стороны stream; непринятый body закрывается в
  исходном синхронном API call. Actual body writer/Close остаётся зарегистрирован
  через возврат worker. Произвольные body providers всё ещё могут зависнуть.
- Existing client qlogger Context.AfterFunc имеет собственный original result.
  Он ждёт sealed task group до producer WG/Close; WaitForShutdown включает его
  completion и возвращённую ошибку/abnormal result. Нет повторного qlogger Close.
  Это не доказывает успех скрытого sink.Close: inspected upstream Recorder может
  скрывать нижележащую ошибку. Server legacy trace lifecycle остаётся отдельно.

### DATAGRAM и аутентификация HY2

- Раньше raw HTTP3 создавал DATAGRAM receive-loop при одном peer SETTINGS=true,
  независимо от local EnableDatagrams=false. При таком peer он мог забирать
  datagrams у отдельного HY2 raw QUIC reader. Условие теперь требует обе стороны.
  Если HTTP3 datagrams не включены локально, скрытый consumer не создаётся.
  Наш текущий service.go HTTP3 datagrams не включает; live failure не наблюдался.
- HY2 attachQUIC регистрирует ранний Closing/context watcher до создания HTTP3
  и auth RoundTrip. SETTINGS/auth error теперь может инициировать independent
  raw abort через этот уже существующий carrier owner. Дублирующий post-auth
  watcher удалён; initialDone по-прежнему ограждает регистрацию tasks/resources.
- HTTP3 ClientConn присоединяется к carrier как один once-only completion task.
  Late attachment после seal сразу начинает wait; initialDone не позволяет
  финальному inventory пропустить его. Raw/QUIC/Transport/opening abort стартуют
  независимо до joins. Negative HTTP3 completion сохраняет старый carrier owner.
- Auth использует response header limit64KiB и DisableCompression=true.
  Это protocol/parser limit, не обещание64KiB heap или total transfer bound.
  Wire/large-header/custom TLS compatibility требует отдельной будущей приёмки.

### Остаток и доказательства

WaitForShutdown вызывается внешним owner после запроса QUIC/raw abort, никогда
из своей task.64 permits — не полный счёт goroutines/bytes carrier: qlogger
callback, HY2 wait, initializer и lower QUIC имеют собственные владельцы.
Выданные RequestStream/response bodies, trace sinks, default HTTP Transport
dial/retry/pool, custom qtls hidden native transport, TCP/path/global last holders
и server/H1 scope остаются открытыми. Нет измеренного ускорения, Stop SLO или RSS.

8Go2origin4docs,14 snapshots(13existing1new), manual9 existing Windows diffs плюс
новый client_lifetime.go, touched-only gofmt8 exit0. Дополнительный stream callback
сначала архивирован scope-extension receipt.13 reviewed inputs/0 cached contracts.
261 cache originals и238 verbatim mirrors сохранены;5 HTTP3 mirrors стали owned
changes с исходными SHA. Все предыдущие owned sources сохранены. Receipts: .artifacts/memory/bootstrap-v400-http3-ownership-.
Source/unverified; no tests/fixtures/build/vet/types/parser/SDK/browser/native/network/generation. Manual source/diff/byte review and touched-only gofmt do not prove compilation/API graph, admission/close/callback/body/qlogger interleavings, runtime race freedom, provider/wire/legacy compatibility, CPU/RSS/throughput or native/global retirement.64 is registered client call/worker/future-callback reservations, not whole-carrier goroutines or bytes; one existing qlogger context callback and HY2 original wait task have separate owners. External body/output-stream/trace borrowers and arbitrary provider/sink blocking or hidden Close errors remain separate; no Stop SLO/forced cancellation/physical quota. Configured64KiB header limit does not cap total transfer/QPACK/allocator/native memory. Current local HY2 server does not enable HTTP3 datagrams; competing-reader defect is conditional on a peer advertising SETTINGS, not an observed live failure. Custom qtls hidden transport, default HTTP Transport dial/retry/pool tasks, server/H1 lifecycle, TCP/lowerQUIC/path/rawConn-bypass/global/OS/wholeUX/release OPEN. FullPC00..09/old9/rootNETGUARD001/100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported.

<a id="pc-v401-tcp-ownership"></a>

## V2-401 — исходный TCP owner и сохраняемый handshake

Статус: PC-06 source/unverified. Sing-quic revision11 сохраняет quic-go revision5,
те же frozen module versions/manifests/licences. clientConn выделен из client.go
в TCP operations/lifetime; protocol response decoder переиспользуется one-shot API.

### Исправленные исходные дефекты

- Старые requestWritten/responseRead не синхронизировались: пара первых Write
  могла повторить заголовок, пара Read — одновременно читать ответ. Теперь writer
  и reader/Peek имеют отдельную сериализацию; NeedHandshake использует atomic flag.
  Read/Peek не отдают framing как пользовательский payload. Zero Read не запускает
  I/O, а Write(nil) сохраняет прежний способ явно отправить request header.
- Раньше WriteTCPRequest копировал весь первый payload и возвращал len(p),
  игнорируя фактическую partial wire count. Теперь перед staging зарезервирован
  permit; checked header writer проверяет address/padding до allocation. Первый
  prefix≤16KiB остаётся coalesced с заголовком; большой остаток borrowed исходным
  QUIC Write. Header offset и payload-only n учитываются раздельно. Partial error
  не сбрасывает/не повторяет уже принятые bytes; short nil становится ErrShortWrite.
- Response parser теперь принадлежит stream. Частичные status/1…8-byte varint/
  message/padding сохраняются между Read deadline и retry. Message≤2048, padding
  ≤4096 пропускается fixed chunk256, status только0/1. Нет padding-length allocation,
  бесконечного zero-progress retry или повторного чтения завершённого response.
  One-shot ReadTCPResponse использует тот же decoder без второй реализации.

### Владение и закрытие

- Carrier lifeAccess ограждает reserve/open-result publication/seal. Резервация
  исходного объекта предшествует address clone/local metadata/OpenStream. Late/
  partial stream остаётся в exact owner; ctx cancel/Stop не публикуют его заново.
  Destination fields bounded до String/Clone; IP+FQDN ambiguity/port0/control chars/
  oversize отклоняются. IPv4-mapped адрес нормализуется после проверки ambiguity.
- Opening/visible/retiring slots≤256; core IO512 позволяет одну обычную Read+Write
  пару всех256 streams, per-stream≤8. Первоначально намеченный64 source limit
  увеличен до512 после анализа ожидающих readers, до финальной фиксации.
  Deadline controls имеют отдельные carrier8/per-stream2, не расходуют IO lane.
- Staging≤8, fixed charge64KiB, ledger≤512KiB до allocation. Checked header≤6168
  logical bytes, first prefix≤16384; default allocator classes header≤8192/stage
  ≤32768, padding scratch≤8192. Retained partial headers дополнительно ограничены
  stream slots; caller input, pool high-water и lower QUIC copies не включены.
- Одна Close task/result на исходный stream. Seal запрещает новые core calls;
  read cancel и past-write deadline будят текущие операции. Admitted setters
  сериализованы control→seal recheck: future deadline не снимает close wake.
  После реального core-call drain освобождается owned framing и вызывается
  original Stream.Close без concurrent core Write. Repeated Close ждёт тот же итог.
- Если write-half уже closed/reset, invalid Close-after-CancelWrite не вызывается.
  Возможная гонка remote reset с Close сохраняет исходную protocol error caller;
  известный stream cancellation не объявляется native cleanup uncertainty.
  Abnormal operation/close сохраняет negative carrier result, original objects,
  uncertain staging и занятые slots, блокируя замену через прежний carrier owner.
  Poison публикуется до освобождения direction mutex, чтобы уже admitted waiter
  не переиспользовал uncertain state; carrier failure и seal имеют общий fence.
- Carrier raw/QUIC/Transport/opening abort стартуют до TCP joins. TCP runtime
  admission не использует initializer task WG после initialDone: отдельный
  lifeAccess-fenced map/settlement не допускает Add-after-Wait race.
- LocalAddr теперь читает owned original QUIC snapshot, RemoteAddr возвращает
  bounded owned logical destination. Ни один getter не выдаёт пустую заглушку;
  факт адреса не доказывает actual protected path или native TCP transport.

### Остаток и приёмка

Совместимость promoted advanced QUIC controls и внешних stream holders остаётся
отдельным C1 gate. Legacy NeedHandshake/Write(nil) contract не заменён скрытой
read-triggered записью: first-read/server-first consumers требуют своей прежней
handshake orchestration. Lower QUIC send/ACK/history, custom qtls hidden transport,
default HTTP pool/server/H1/trace sinks/path/global/native/OS/wholeUX/release OPEN.
Будущая приёмка должна охватить concurrent first writes/reads, partial header и
payload n, clear/extend deadline на каждой framing phase, large/zero writes,
all256 Read+Write pairs, capacity pressure, opening vs Stop, Close vs setters/
readers/writers, repeated Close/remote reset, abnormal buffers и actual caller ABI.

6Go1origin4docs,11 snapshots(8existing3new), manual4 existing Windows diffs плюс
3 полных новых helper, touched-only gofmt6 exit0.12 reviewed inputs/6 cached
contracts;261 cache originals и237 verbatim mirrors сохранены, protocol proxy
переведён в owned changes с original SHA; все prior owned sources сохранены.
Сбой progress checkpoint git-common-directory-unavailable терминально зафиксирован;
после окончания byte review повтор CAS с той же revision998 успешен→999.
Receipts: .artifacts/memory/bootstrap-v401-tcp-ownership-.
Source/unverified. No tests/fixtures/build/vet/types/parser/SDK/browser/native/network/generation. Manual source/diff/byte review and touched-only gofmt do not prove compilation/API graph, open/seal/Close/deadline/partial-frame/borrowed-buffer interleavings, race freedom, wire/legacy/provider compatibility, fairness, CPU/RSS/throughput or native/global retirement. Bounds govern admitted core operations and original staging/stream owners, not lower QUIC frames/ACK/history, custom allocators, caller input aliases, retained closed wrappers or process-wide/physical memory. Advanced promoted QUIC controls, original native stream final holders and complete generic net.Conn/early-handshake consumer compatibility remain OPEN. Legacy NeedHandshake/zero-write contract retained: Read does not silently inject a request, first-read/server-first callers need their existing handshake path. LocalAddr is an observed QUIC socket fact and RemoteAddr the logical destination, not protected-egress/native-TCP proof. No hard Stop/deadline SLO or measured superiority. Custom qtls/defaultHTTPTransport/server/H1/trace sink/lowerQUIC/path/rawConn-bypass/global/OS/wholeUX/release and fullPC00..09/old9/rootNETGUARD001/100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported remain OPEN.

<a id="pc-v402-tcp-halfclose"></a>

## V2-402 — half-close, early API и TLS preflight

Статус: PC-06 source/unverified. Sing-quic revision12 сохраняет quic-go revision5,
frozen versions/licences/manifests. Новая half-close часть расширяет существующий
TCP component и переиспользует pinned N.ReadCloser/WriteCloser/EarlyReader/Writer.

### Фактические потребители и дефект

- Cached sing-box1.13.19 route/conn.go ConnectionManager.connectionCopy при
  успешном upload EOF вызывает destination.(N.WriteCloser).CloseWrite(), если
  capability есть; иначе вызывает destination.Close(). Наш старый Close отменял
  read и мог прервать pending download. Sing0.8.13 bufio.CopyConn имеет аналогичный
  capability branch, а затем общий cleanup после обеих сторон.
- ConnectionManager.kickWriteHandshake проверяет N.NeedHandshakeForWrite и
  отправляет cached payload или Write(nil), для server-first тоже Write(nil),
  с временным write deadline/reset. Legacy predicate сохранён. Modern atomic
  read/write predicates описывают только framing; успешный response публикуется
  через atomic read flag. Нет скрытой записи из Read и нового Upstream/raw escape.
- NewCopySession/HandshakeState наблюдают переход framing flags, rewrap не
  удаляет исходный TCP owner. Это source wiring, не runtime доказательство
  конкретной Windows connection/wrapper/tag graph.

### Реализация half-close и bounded encoding

- CloseWrite регистрируется в прежнем IO lane, сериализуется с Write и не вызывает
  Stream.Close одновременно с core writer. Empty upload сначала завершает bounded
  request header. Затем original FIN attempt/result хранится под write mutex;
  повтор при активном owner читает тот же результат, write payload больше не
  принимается. Receive half остаётся доступной для response после upload EOF.
- Уже closed/reset write-half не вызывает invalid Close-after-CancelWrite;
  сохраняется original context cause. Нормальная protocol error не становится
  native uncertainty. Caller решает обработку: ConnectionManager закрывает пару
  при CloseWrite error, CopyConn может её игнорировать. Полный runtime effect
  обоих consumers не объявляется проверенным.
- Header-flush error при CloseWrite запрашивает прежний logical full Close,
  чтобы проигнорированная caller ошибка не оставляла reader на incomplete request.
  CloseRead использует control lane/CancelRead, не ждёт заблокированный read mutex
  и оставляет write открытым. Обе local halves закрыты — после recorded write
  outcome запрашивается прежний full owner; active calls по-прежнему joined.
- Atomic intent и outcome разделены. Abnormal return использует прежний poison
  до direction unlock, original refs/credits не переиспользуются. Нет новой
  goroutine/timer/quota; coreIO512/control8/stream256/staging8×64KiB сохранены.
- Write(nil)/CloseWrite(empty) напрямую borrow bounded original header до actual
  native Write return. Они не создают payload coalescing buffer и не расходуют
  его8 permits. Header offset сохраняется через partial timeout; abnormal header
  остаётся в исходном stream owner, освобождение не опережает actual call.
- Для payload known errTCPStaging выбирает тот же header-only prefix, затем
  исходный payload borrow на том же QUIC. Ошибки closed/uncertain этим путём не
  обходятся. Shared coalescing ceiling не увеличен; per-stream framing storage
  остаётся под slot bound. Есть packetization/performance/pressure acceptance,
  измеренного ускорения, total RSS или peer-delivery proof нет.

### TLS/provider граница

- NewClient отвергает nil TLS; offer создаёт собственный clone/h3 ALPN и проверяет
  STDConfig до первого socket dial. Unsupported/nil STD result не создаёт наш raw
  socket. После успешной подготовки по-прежнему attachRaw/attachTransport до
  actual DialEarly; offer initializer/late-attachment owner не заменён.
- Custom qtls.Config сохраняет отдельный DialEarly path. Ошибка не вызывает
  fingerprint/provider downgrade. Current Windows HY2 render.go создаёт STD TLS;
  cached TLS factory выбирает STD, uTLS или Reality, последние STDConfig отвергают.
  Tag with_utls сам по себе не означает поддержанный QUIC uTLS. Новые custom
  providers не активировались; hidden native resource joins остаются OPEN.

### Остаток, будущая приёмка и evidence

Advanced promoted CancelRead/CancelWrite/SetReliableBoundary, Context/StreamID
contracts, vectorised caller merge/release, custom providers, lower QUIC/path/
global/server/H1/trace/OS/wholeUX/release остаются открытыми. Будущая приёмка:
upload EOF с поздним response, empty/server-first request, read-close при write,
две local halves/repeated half-close/full Close, peer reset/FIN race, partial
header/expired deadline, coalescing pressure+seal и actual caller/tag ABI.
CloseWrite может ждать existing writer: нет hard Stop/deadline SLO.

3Go1origin4docs;8 snapshots(7existing1new), manual3 existing Windows diffs плюс
полный новый half-close helper, gofmt3 exit0.12 reviewed inputs/11 cached contracts;
261 cache originals/237 verbatim mirrors/prior owned bytes сохранены. Initial pre
checkpoint revision-conflict сохранён; original own ledger1000 не изменился,
fallback ledger отсутствовал. После read-only Git identity revalidation повтор
CAS с той же revision1000 успешен→1001; начальный route failure не доказан.
Receipts: .artifacts/memory/bootstrap-v402-tcp-halfclose-.
Source/unverified; no tests/fixtures/build/vet/types/parser/SDK/browser/native/network/generation. Manual source/diff/byte review and touched-only gofmt do not prove compilation/interface/tag graph, EOF/FIN/read-drain/half-close/Close/deadline/header/capacity interleavings, race freedom, peer/wrapper/provider compatibility, CPU/RSS/throughput, native/global retirement or protected egress. Source routing to CloseWrite is not live ConnectionManager evidence. Half-close completion is local outcome, not peer ACK/delivery. CloseWrite may wait for an existing writer; no Stop/deadline SLO. Current renderer/factory TLS analysis is scoped to inspected frozen inputs, not universal custom-provider absence or runtime fingerprint proof. Custom hidden resources/config callbacks, advanced promoted QUIC controls/Context/StreamID, generic vectorised writer merge/release, lowerQUIC/path/global/server/H1/trace sink/OS/wholeUX/release remain OPEN. Direct Read still needs the caller early-write path; no hidden read-triggered request was added. Per-stream headers/caller buffers/custom allocator/pool high-water have separate scope from shared coalescing charges. FullPC00..09/old9/rootNETGUARD001/100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported.

<a id="pc-v403-tcp-vectorised"></a>

## V2-403 — consuming WriteBuffer/WriteVectorised без whole-payload merge

Статус: PC-06 source/unverified. Sing-quic revision13 сохраняет frozen versions,
quic-go revision5, manifests/licences. Новый helper расширяет существующий HY2
TCP component, используя точные pinned N.ExtendedWriter/VectorisedWriter APIs.

### Сверка потребителей

- Sing0.8.13 NewVectorisedWriter выбирает capability либо BufferedVectorisedWriter;
  последний объединяет весь пакет и defer-освобождает исходные buffers. Explicit
  cached caller — uot.LazyClientConn. Его текущая Windows активация не доказана.
- Обычный CopyWait до этого мог продолжать scalar path при отсутствии capability,
  а не обязательно вызывать generic merge. Default batch8 и переход после512000
  bytes — факты pinned source. Windows read waiter и ExtendedWriter wrapper
  просмотрены; это не runtime/tag acceptance.
- Оба обычных CopyExtended/CopyWait вызывают WriteBuffer. Общий wrapper освобождает
  input в defer даже при abnormal original Write. HY2 теперь предоставляет
  собственный WriteBuffer через тот же consuming owner. NewExtendedWriter может
  выбрать его напрямую; foreign/force-stdio wrappers остаются отдельной границей.

### Владение, давление и framing

- Carrier содержит256 фиксированных slots, суммарно4096 input pointers и16MiB
  exposed backing; один вызов принимает не более256 buffers. Проверяются nil,
  start/length/effective capacity/raw backing, reachable slice suffix и bounded
  arithmetic. Prefix/suffix capacity учитывается до копирования ссылок.
- Core-call admission и count/byte reservation предшествуют metadata make/copy,
  IncRef и ожиданию write mutex. Ожидающие writers также charged; ограничение
  не описывает ещё не принятые caller inputs. Точная отдельная pointer table не
  удерживает избыточный caller slice; single input использует inline slot.
- Один write mutex охватывает весь batch/header. Scalar Write переиспользует
  тот же writePayload; нет чередования с другим payload/CloseWrite внутри batch.
  Existing bounded header/prefix/partial offset сохранены. Empty batch flushes
  Write(nil); пустые элементы mixed batch пропускаются без лишней записи.
- Известная ошибка прерывает batch без повторения уже переданных bytes; error-only
  API не сообщает partial count. Обычный отказ/ошибка consumes inputs. Pressure
  возвращает явную ошибку и не подменяет QUIC stream, маршрут или trust.
- IncRef удерживает original managed inputs от pool return; unmanaged views
  сохраняются original references. После actual write return каждый pin снимается
  и input освобождается до refund. Cleanup panic оставляет slot/credits и даёт
  negative original owner, а не новый cleanup attempt.
- Abnormal native return сохраняет originals/pins/credits, poison/seal до write
  unlock и прежний carrier retirement. Existing core-call join учитывает всю
  consuming operation, включая cleanup. Новых goroutine/timer/Upstream нет.
- Metadata slots фиксированы; динамическая pointer table ограничена4096 pointers
  плюс fixed inline single storage. Existing prefix staging8×64KiB/stream256/
  coreIO512/control8 не увеличены. Это structural ceilings, не total RSS/SLO.

### Остаток и evidence

Draft vector-only receipts сохранены отдельно. При source review добавлен
реальный scalar consuming consumer и исправлен учёт writers до ожидания mutex;
обязательства и scope не сокращены. Проверка actual lowerQUIC normal/abnormal
Write/CancelWrite/shutdown last-holder return — следующий связанный участок.
Advanced promoted controls/custom hidden/path/global/server/H1/OS/wholeUX/release
и полная цель PC-00…09/старые девять этапов остаются OPEN.

3Go1origin4docs;8 snapshots(7existing1new), manual3 existing Windows diffs плюс
полный consuming helper. Touched-only gofmt exit0,12 reviewed/11 cached contracts;
261 cache originals/237 verbatim mirrors/prior owned bytes сохранены.
Receipts: .artifacts/memory/bootstrap-v403-tcp-vectorised-.
Source/unverified. No tests/fixtures/build/vet/types/parser/SDK/browser/native/network/generation. Manual source/diff/byte review and touched-only gofmt do not prove compilation/API/tag graph, actual caller/Windows copy reachability, ref/pool or alias safety, concurrency/Goexit/panic/Close/deadline/pressure interleavings, throughput/CPU/RSS, fairness, hard wall-time/native retirement or protected egress. The bound covers admitted consuming inputs, including queued writers, by exposed backing/capacity; caller generation before the API, raw scalar Write slices, invisible backing prefixes/aliases, custom allocators/global pools, fixed metadata, lower QUIC frame/history/ACK/RX/TX and hidden provider resources have separate scope. Normal writer return is the current io.Writer contract boundary, not a proof of complete lower-QUIC borrow retirement or remote delivery. Default copy may previously remain scalar; generic fallback full merge is not proven universal Windows runtime behavior. force-stdio/foreign wrappers remain separate. Advanced promoted controls/custom hidden/lowerQUIC/path/global/server/H1/trace/OS/wholeUX/release OPEN. FullPC00..09/old9/rootNETGUARD001/100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported.

<a id="pc-v404-stream-borrow"></a>

## V2-404 — normal/abnormal SendStream input retirement

Статус: PC-06 source/unverified. Quic-go revision6/sing-quic revision14 сохраняют
frozen versions, manifests/licences и прежние HY2 scalar/vector/half-close APIs.

### Найденные source branches

- SendStream.write после пробуждения сначала мог coalesce short tail и лишь в
  другой ветке проверял reset/shutdown/deadline. Маленький остаток при terminal
  wakeup мог попасть в новый STREAM frame и получить полный successful count.
  Это подтверждённый порядок исходников, не воспроизведённая runtime история.
- Обычный reset/shutdown error не всегда очищал dataForWriting. Ссылка на caller
  backing сохранялась в stream после возврата, тогда как верхний consuming API
  мог законно вернуть буфер в pool. Последующее чтение этой ссылки и corruption
  не объявляются доказанными: packetizer/reliable boundary изучены отдельно.
- Старый defer Unlock не учитывал intentional unlock во время sender callback.
  Abnormal callback/Goexit в этой точке мог попытаться unlock незахваченный mutex.

### Исполнение исходного владельца

- Один wrapper write сохраняет mutexHeld/normal-return state вокруг прежнего
  writeLocked body. Обычный результат очищает dataForWriting под тем же mutex,
  который использует packetizer. Count вычислен до очистки. Packer получает
  собственные copied STREAM frames; их ACK/loss lifetime этим не закрыт.
- Каждый loop сначала фиксирует accepted count, затем terminal state и pending
  deadline, и только потом может копировать новый tail. Уже полностью accepted
  bytes сохраняют success; unsent остаток возвращает исходную ошибку/partial n.
  Existing resettable timer, monotime и FIN/frame-copy strategy сохранены.
- Abnormal body reacquires mutex, когда тот intentionally released, и сохраняет
  sticky writeUncertain с original view/stored frame state. Это предшествует
  освобождению прежнего writeOnce. Panic/Goexit не swallowed и не превращён в nil.
- Повторный Write/Close получает явную uncertainty; packet/control production и
  normal stream-completion не переиспользуют состояние. CancelWrite/STOP_SENDING/
  known shutdown не выполняют destructive cleanup поверх uncertain originals.
  Native Conn Context/Close не выдаются за отдельную retirement receipt.
- Верхний HY2 consuming owner по-прежнему удерживает original ref-pinned inputs
  при abnormal native return и seals carrier до direction unlock. Новых worker,
  timer, payload clone, quota или публичного QUIC API нет. Mutex reacquire может
  ожидать существующий borrower: hard Stop/wall-time bound не заявляется.

### Остаток и evidence

ReceiveStream CancelRead/shutdown/EOF и frameSorter original done callbacks —
следующий связанный участок. Reliable reset, frame/history/ACK/loss custody,
post-FIN shutdown cleanup, promoted advanced controls, custom hidden resources,
whole/global/native/OS/UX/release и весь PC-00…09/старые девять этапов OPEN.
Future acceptance после снятия запрета: short-tail reset/STOP_SENDING/shutdown/
expired/reset/cleared deadline, zero/partial/full input, normal retained view,
abnormal locked/unlocked callback/Goexit, repeated Write/Close и actual HY2 caller.

1Go2origin4docs;7existing snapshots, manual3 Windows diffs, touched-only gofmt1
exit0,17 reviewed/3 cached source inputs.261 cache originals/236 verbatim mirrors
и прежние owned changes сохранены; send_stream.go имеет собственный before SHA.
Receipts: .artifacts/memory/bootstrap-v404-stream-borrow-.
Source/unverified. No tests/fixtures/build/vet/types/parser/SDK/browser/native/network/generation. Manual source/diff/byte review and touched-only gofmt do not prove compilation/API/tag graph, actual Windows consumer/peer behavior, reset/deadline/shutdown/tail/Close/Goexit/panic interleavings, error/partial count compatibility, race freedom, resource/native last-holder retirement, RSS/throughput or protected egress. The new fence covers stored SendStream input view and registered HY2 consuming custody; it is not a complete lower QUIC frame/history/reliable-reset/ACK/loss or receive/sorter/global resource ledger. Abnormal write-body return keeps the original view/frame state, and the existing HY2 owner retains the original ref-pinned inputs; no claim that arbitrary raw callers honor consuming lifetime. Completion notifications outside the body, foreign callbacks and their hidden resources remain separate. Mutex reacquire can wait for an existing borrower; no hard wall-time/Stop SLO or automatic whole-Conn close is provided by the local fence. Existing known shutdown after FIN/reliable frame cleanup and Context/advanced control contracts remain OPEN. FullPC00..09/old9/rootNETGUARD001/100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported/wholeUX/release.

<a id="pc-v405-receive-retirement"></a>

## V2-405 — receive terminal release и RESET_STREAM_AT prefix

Статус: PC-06 source/unverified. Quic-go revision7/sing-quic revision15 сохраняют
frozen versions/licences/manifests и прежние HY2 admission/send/half-close fences.
Новый receive lifetime helper расширяет существующий component, без второго
native receiver/queue и без новой provider/config authority.

### Сверка исходных веток

- CancelRead и shutdown отмечали terminal state, но не освобождали сохранённые
  current/queued frame callbacks. EOF вызывал currentFrameDone, оставляя stale
  callback field; новая cleanup не должна вызывать его второй раз.
- Read мог скопировать из current frame больше reliableSize-readPos. Peek проверял
  readable current frame раньше reliable cutoff и мог вернуть unreliable tail.
  Это source branch audit, не воспроизведённый сетевой дефект.
- Проверены read/peek mutex release, sorter Pop/Peek/Push, native frame pool,
  parser/connection/map source ownership и actual HY2 core/control Close join.
  Parser/map/skipHandling/Push-error handoff обнаружен как отдельный open участок.

### Реализация

- releaseCurrentFrame хранит original callback/view до actual return и очищает
  оба поля после него. frameSorter.Discard seals перед обходом, удаляет entry
  только после callback return, затем снимает queue/gap tree. Abnormal callback
  удерживает original entry/flag и не повторяется новым Discard/Pop/Peek/Push.
- Local CancelRead, shutdown, consumed EOF и effective remote reset используют
  этот путь под одним receive mutex. Native readers копируют, а не выдают view;
  cleanup не опережает текущий mutex borrower. Частично потреблённый reliable
  prefix сохраняется, пока remote reset ещё не effective.
- finishReceiveCall учитывает intentional unlock/relock во время Read/Peek waits
  и sender notifications. Abnormal native operation seals stored receive state
  до mutex reuse; Read/Peek/deadline/control/completion не выдают healthy reuse.
  Panic/Goexit не recovered-as-success; новые workers/timers/copies не добавлены.
- Read ограничивает source slice reliable remainder и возвращает partial count
  с original cancel error на границе, освобождая остаток. Peek проверяет reliable
  cutoff до current-frame fast path, копирует только contiguous reliable prefix
  и не consumes его. Повторный меньший RESET_STREAM_AT wakes pending reader.
- После abort unread last frame не превращается в manufactured EOF: terminal
  last-frame marker сбрасывается, retired empty queue не создаёт новый FIN.
  Already-consumed EOF остаётся стабильным. Repeated local cancel/cleanup не
  переисполняет callback; cleanup uncertainty не снимается сменой deadline.
- Known validation/terminal reject до Push имеет отдельный fixed original frame
  release slot. Нельзя обобщать его на parser/map rejects или ambiguous Push
  error/cut/replacement branches: их intake owner ещё не завершён.

### Остаток и evidence

Следующее: exact source intake owner от ParseStreamFrame через connection skip/
streamsMap до sorter Push, включая known errors, duplicate/cut/replacement и
abnormal callbacks; count/bytes до production copy/admission. RX global bounds,
flow-control acceptance, ACK/history/reliable-reset/advanced/native/OS/wholeUX/
release и весь PC-00…09/старые девять этапов не закрыты этим срезом.

3Go2origin4docs;9 snapshots(8existing1new), manual4 existing Windows diffs плюс
полный receive lifetime helper; touched-only gofmt3 exit0.16 reviewed/2 cached
source inputs,261 cache originals/234 verbatim mirrors и prior owned changes
сохранены. Source/byte review не является compile/runtime/native PASS.
Receipts: .artifacts/memory/bootstrap-v405-receive-retirement-.
Source/unverified. No tests/fixtures/build/vet/types/parser/SDK/browser/native/network/generation. Manual source/diff/byte review and touched-only gofmt do not prove compilation/API/tag graph, actual Windows/peer behavior, partial read/peek/reliable reset/duplicate boundary/cancel/EOF/shutdown/deadline/callback/Goexit interleavings, race freedom, correct flow-control accounting, full native negative-result aggregation, receive resource admission, RSS/throughput or protected egress. Current/queued terminal callbacks are covered; parser/map/skipHandling and sorter Push-error/duplicate/cut/replacement intake still have separate unresolved original handoff. There is no complete lower QUIC receive/global/frame/history/ACK/loss resource ledger. Arbitrary foreign callbacks/reentrancy and hidden resources, promoted raw controls, post-FIN send shutdown, whole-Conn cleanup and caller ABI remain OPEN. Mutex reacquire/callback execution has no hard duration bound. FullPC00..09/old9/rootNETGUARD001/100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported/wholeUX/release.

<a id="pc-v406-stream-intake"></a>

## V2-406 — original STREAM intake и per-stream receive quota

Статус: PC-06 source/unverified. Quic-go revision8/sing-quic revision16 сохраняют
frozen versions/manifests/licences и прежние HY2 send/half-close/reliable fences.
Два новых helper развивают существующие intake/sorter components; новой схемы
транспорта, raw fallback, worker или публичного QUIC API не добавлено.

### Original handoff и исправленные ветки

- ParseStreamFrame проверяет offset+length через subtraction и1452-byte ceiling
  до metadata/pool/payload allocation. Ошибка range больше не теряет уже созданный
  frame после копирования. Full producer admission до первой parser copy ещё OPEN.
- Inline streamFrameIntake хранит оригинал до Conn trace/skip, streamsMap lookup
  и ReceiveStream validation/callbacks. Known untransferred success/error releases
  один оригинал; transfer очищает прежний slot перед входом следующего владельца.
  Abnormal body/release сохраняет original/negative slot без retry/replacement.
- SkipHandling освобождает decoded STREAM frame, сохраняя earlier packet error.
  Invalid/deleted map stream теперь consumes frame. Qlog source преобразует STREAM
  в независимые length/offset/id/FIN facts, а не сохраняет raw payload view.
- Receive intake release выполняется под original receive mutex до completion/
  sender publication. Его abnormal guard использует прежний mutex/state fence;
  raw slot не очищается из deferred callback вне mutex. Conn/maps сохраняют
  существующий serialized native frame-dispatch contract, не второй producer.

### Sorter ownership и count/bytes

- Push consumes normal capacity/closed/duplicate/cut/error inputs. Fixed incoming
  и refused slots не перезаписывают прежний uncertain original. Replacement и
  deleteConsecutive вызывают callback до удаления исходной queue entry; unknown
  callback оставляет root. Failed partial gap mutation seals дальнейший queue use.
- ReceiveStream использует отдельный newReceiveFrameSorter. Принимаются не более
  8192 holders и8MiB exposed backing. Count/bytes резервируются до holder metadata,
  method-value callback и новой trim copy. Original wire backing charged через
  cap(data), а не только длину обрезанного view.
- Pop переносит holder callback к current reader без refund. Holder map удерживает
  original bytes/callback через actual last-reader release; view/callback clear
  предшествуют count/byte refund. Successful repeated release не возвращает квоту
  дважды; retry uncertain original запрещён и не превращается в healthy result.
- Small trim сохраняет исходную copy optimization: output admission предшествует
  make/copy, пока original ещё charged. Original pool return затем освобождает
  только его credit; output остаётся charged до своего actual return. При known
  отказе новая output allocation не выполняется.
- Generic crypto sorter остаётся с nil callbacks/старым Pop: receive holders не
  навязываются потребителю, игнорирующему release callback. Gap20000 и logical
  flow-control windows сами по себе не являются physical receive/global bounds.

### Остаток и evidence

Следующий связанный участок — per-connection original receive reservation до
первой parser allocation/copy с переносимой release identity, затем sealed
native aggregate join. Global producers/credits, pool/custom allocator/RSS,
flow-control/admission compatibility, ACK/history/advanced/OS/wholeUX/release и
весь PC-00…09/старые девять этапов остаются OPEN. Pressure limits не заявляются
оптимальными для всех custom windows/fragment layouts без будущих измерений.

8Go2origin4docs;14 snapshots(12existing2new), manual8 existing Windows diffs плюс
два полных helper, touched-only gofmt8 exit0.17 reviewed/2 cached inputs;
261 cache originals/232 verbatim mirrors и prior owned changes сохранены.
Первый prepare завершился ENOSPC/exit1 с пустым start-status до source edits.
Пустой artifact сохранён отдельно; read-only NTFS observation показало2.9GB free,
тот же prepare затем успешен без удаления файлов. Первичная причина не доказана.
Receipts: .artifacts/memory/bootstrap-v406-stream-intake-.
Source/unverified. No tests/fixtures/build/vet/types/parser/SDK/browser/native/network/generation. Manual source/diff/byte review and touched-only gofmt do not prove compilation/API/tag graph, serialized dispatch/reentrancy assumptions, actual Windows/native/peer behavior, ref/pool/cut/overlap/error/callback/Goexit interleavings, race freedom, flow control, fairness, throughput or protected egress. The receive ledger covers admitted per-stream sorter holders, current reads and trim-copy overlap. First parser allocation/copy and per-connection/global budgets, refused pre-admission inputs/fixed intake metadata, pool high-water/custom allocator/RSS, native error aggregation, caller cancellation/Stop SLO and ACK/history/global ownership remain separate OPEN scope.8192/8MiB are versioned pressure ceilings, not measured optimal settings or a universal compatibility promise; larger/custom receive windows and tiny-frame layouts need later acceptance. Conn/maps inline ownership assumes their existing serialized native frame dispatcher; ReceiveStream/holds use its original mutex. Crypto callbacks remain unbounded by the receive ledger. No complete native resource-release claim or new public QUIC API. FullPC00..09/old9/rootNETGUARD001/100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported/wholeUX/release.

<a id="pc-v407-connection-receive"></a>

## V2-407 — connection STREAM receive budget и исходное освобождение

Статус: PC-06 source/unverified. Quic-go revision9/sing-quic revision17 сохраняют
frozen upstream versions, manifests, licences и прежние HY2 quotas. Два новых
helper развивают существующие Conn/parser/intake/sorter components.

### Приём до первой аллокации

- Conn создаёт один streamReceiveBudget до producers. Не более32768 original
  data owners и32MiB exposed backing на соединение. Count/bytes резервируются
  до holder/map/frame/pool/payload allocation; opening reservation уже учитывается
  при seal, даже если original object ещё создаётся. Счётчик не зависит от числа
  отдельных receive streams и сохраняется при ResetFor0RTT.
- Один original parser registration охватывает проверку header/range/1452,
  первую allocation/copy, trace, все handoffs и исходный handler return. Partial
  allocator result хранится в pending до known release или sticky uncertainty.
  Seal и producer admission используют тот же budget mutex.
- Wire allocator seam вызывается только после полной проверки входного диапазона
  и до первого frame/backing copy. Pool frames резервируют1452 bytes, small
  frames — requested size; count ограничивает также zero-length frames. Local
  capacity/closing/InternalError сохраняется TransportError и не превращается
  в обвинение peer в malformed framing. Старый raw parser API остаётся доступен
  его отдельным callers; Conn не использует raw fallback при отказе budget.

### Владение и возврат квоты

- receivedStreamData имеет отдельную original identity: callback хранит этот
  object, а не method value изменяемого pooled frame. После успешного release
  повторный callback — noop; промежуточный/unknown release не повторяется.
  Переиспользование адреса frame пулом не даёт старому callback новый original.
- Conn/maps/ReceiveStream intake проверяют принадлежность тому же budget и явно
  передают original. Native receive mutex и прежние known/abnormal handoffs
  сохранены. Per-stream8192/8MiB ledger продолжает владеть current/queued views.
- Small trim резервирует local holder, затем Conn data holder до output make/
  copy, пока source ещё удерживает обе квоты. При known Conn refusal пустой
  local reservation освобождается; новый output не создаётся. На успехе оба
  output credits сохраняются до actual last-reader release. Generic crypto
  sorter сохраняет callback-free контракт и отдельную область ответственности.
- Pool/provider callback выполняется вне budget mutex. Original frame/data view
  очищается, count/bytes возвращаются только после actual callback return.
  На abnormal allocation/body/release исходные references/credits удерживаются;
  failure sticky и через прежний closeLocal запрашивает закрытие. Этот callback
  только публикует исходный close error/notification, без sendQueue wait под
  receive/budget mutex. Новый worker, timer или recovery fallback не добавлен.

### Закрытие и точная область результата

- Первый original close CAS сначала публикует Closing, затем seals STREAM data
  admission. Начавшийся parser учитывается до своего original return, даже если
  его data holder уже передан или освобождён. Positive zero требует sealed,
  count=0 и отсутствия active parser/pending original.
- Публичный Conn.WaitForStreamReceiveDataShutdown ждёт исходный Conn context,
  sealed data/parser result и возвращает sticky failure. Original HY2 QUIC
  close call делает CloseWithError, затем errors.Join с этим результатом.
  Существующий независимый raw abort и TCP/HTTP3 joins сохраняются. Negative
  settlement не означает очистку: original references/credits остаются retained.
- Название API намеренно ограничено receive data. Это не join всех arbitrary
  promoted/control callers, late public callbacks, native resources или всего
  QUIC. Existing receive guards передают выявленную uncertainty в Conn budget;
  полный original-call registry/aggregate остаётся дальнейшим P0.

### Evidence и остаток

13Go2origin4docs;19 snapshots(17existing2new), manual13 existing Windows diffs
и два полных helper; touched-only gofmt13 exit0.17 reviewed/2 cached inputs;
261 cache originals/231 verbatim mirrors и previous owned bytes сохранены.
Initial source receipts сохранены отдельно перед уточнением ORIGIN о границе
DATAGRAM output/STREAM current и source-compatible rollback пары.
Предыдущий ответ только с оценкой процента не продвигал разработку; текущий
проход сверил revision1016 и возобновил незавершённый V407. Оценка55% не является
измеренным прогрессом требований или доказательством работоспособности.

Следующий участок — полный bounded original-call registry receive/control
операций с join исходных returns и отдельным результатом uncertainty. Далее
сохраняются raw packet/crypto/ACK/send history/global producers, настоящий C1/
native TUN/OS, все страницы UX, выпуск и весь PC-00…09/старые девять этапов.
Receipts: .artifacts/memory/bootstrap-v407-connection-receive-.
Source/unverified. No tests/fixtures/build/vet/types/product parser/SDK/browser/native/network/generation. Manual source/diff/byte review and touched-only gofmt do not prove compilation, API/tag graph, concurrency, wire/peer/flow-control compatibility, callback/pool/Goexit interleavings, shutdown duration, native protection or speed. Conn/maps use the existing serialized dispatcher; receive/sorter use the original receive mutex. The connection ledger covers admitted STREAM parser/current/queued/trim backing; one original parser registration spans dispatch.32768/32MiB and per-stream8192/8MiB are versioned pressure ceilings, not measured optimal settings. Exposed requested/pool capacity excludes allocator/GC/pool high-water, raw input packets, crypto, ACK/send history, arbitrary caller/control/task lifetime, empty-stream metadata, other connections and whole-process RSS. WaitForStreamReceiveDataShutdown proves only sealed data/parser scope; a negative result retains original references/credits and is not cleanup. Existing HY2 TCP/HTTP3 joins remain separate. No whole QUIC/native/global retirement or hard Stop/expiry/performance claim. FullPC00..09/old9/rootNETGUARD001/100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported/wholeUX/release remain OPEN.

<a id="pc-v408-receive-calls"></a>

## V2-408 — original receive-call registry и завершение исходных вызовов

Статус: PC-06 source/unverified. Quic-go revision10/sing-quic revision18,
frozen versions/manifests/licences без изменений. Один новый helper развивает
существующие ReceiveStream/Conn owners и HY2 close; новый транспорт не создаётся.
V407 классифицирован progress: исходники, provenance, docs и checkpoint изменены.

### Admission и независимость управления

- streamReceiveCalls создаётся вместе с Conn receive budget до публикации
  streams и запуска Conn loop. Он допускает512 Read/Peek, отдельные64 controls
  и один native dispatch/shutdown call. Per-stream limits8/2. Counters, running
  и per-stream slots резервируются до original token/map allocation, readOnce
  и receive-mutex wait; opening reservation уже мешает positive settlement.
- Native здесь означает внутренний QUIC dispatcher, а не вызов Win32. Existing
  frame/framer/0RTT-reset/shutdown callers сериализованы Conn loop; их lane
  независим от public read/control pressure. Close не запрещает исходному loop
  закончить teardown, но после его return новые native calls не принимаются.
- Public admission и seal разделяют owner mutex. Дополнительно проверяется
  original Conn.Closing channel: опубликованный close не даёт принять новую
  public operation, пока закрывающая goroutine ещё не выполнила metadata seal.
  Причина закрытия возвращается при её наличии; до её публикации — net.ErrClosed.
- Known capacity seals public calls и через существующий closeLocal запрашивает
  Conn close вне owner/stream locks. Это известный отказ, не uncertain cleanup.
  Void CancelRead не превращает перегрузку управляющего lane в молчаливое
  продолжение приёма; отдельный close signal будит текущих readers.

### Время жизни original calls

- Read/Peek регистрируются до ожидания readOnce. Их gate wait и data waits
  слушают owned closing channel наряду с прежним read/deadline signal. После
  wakeup состояние проверяется под прежним receive mutex; новый worker/timer/
  wait-list или polling не добавлен. Partial byte counts сохраняются.
- CancelRead/SetReadDeadline используют отдельные control slots, поэтому
  занятые read slots сами по себе не запрещают отмену/смену deadline. Native
  STREAM/reset/getControlFrame/closeForShutdown получают original native token.
  Known refusal до STREAM adoption consumed через исходный data.release.
- Original token остаётся зарегистрированным через receive mutex unlock,
  readOnce release и sender/flow-control notifications. getControlFrame и
  SetReadDeadline теперь используют прежний abnormal receive mutex guard.
  Per-stream counters защищает call-owner mutex; этот mutex никогда не ждёт
  receive mutex/read gate или foreign callback. Обратного lock order не введено.
- Normal completion удаляет original token и возвращает credit только после
  исходного тела/cleanup/notifications. Abnormal call/opening сохраняет token/
  stream/credits, seals public reuse и передаёт sticky failure в data budget/
  existing Conn close. Running count заканчивается после failure notification;
  negative result не опережает ещё выполняющиеся зарегистрированные calls.
  Новый original не заменяет uncertain previous owner.

### Loop и составной результат

- Original Conn loop owner существует до run. Его final defer выполняется
  после прежнего queued-packet drain и перед ctxCancel. Отдельные body/drain
  markers требуют normal завершения обоих. Лишь путь через исходный
  close/stream shutdown/final body получает normal loop marker. Early/abnormal
  body/cleanup return даёт sticky negative, без recover-as-success.
- После loop return public/native admission sealed. Call settlement ждёт
  running=0; normal результат дополнительно требует пустых token/count ledgers.
  Uncertain credits/references остаются retained в negative settlement.
- Новый Conn.WaitForStreamReceiveShutdown ждёт original context, затем actual
  call result и прежний data/parser result с errors.Join. HY2 original QUIC close
  использует именно этот составной API. Existing independent raw abort, TCP
  calls/half-close и HTTP3 task joins сохранены. Старый data-only API остаётся
  доступен с прежним узким контрактом; это не whole QUIC/native/OS join.

### Evidence и дальнейший объём

5Go2origin4docs;11 snapshots(10existing1new), manual6 existing Windows diffs и
полный helper. Touched-only gofmt5 exit0, затем5 exit0 после исправления
original Closing/admission gap; затем connection.go/gofmt1 exit0 для отдельного
deferred-drain marker. Initial source receipts сохранены отдельно;
latest source diffs включают это исправление.15 reviewed/0 additional cached
inputs,261 cache originals/231 verbatim mirrors и previous owned bytes сохранены.
Ни parser/SDK, ни продукт/тесты/сборка/native/сеть не запускались.

Следующий связанный участок — bounded original SendStream operations и last
holders передача/ACK/loss/history, затем global/raw/crypto/resource inventory.
Настоящий C1/native TUN/OS, все страницы UX, выпуск и весь PC-00…09/старые
девять этапов сохраняются. Pressure ceilings не являются measured optimum.
Receipts: .artifacts/memory/bootstrap-v408-receive-calls-.
Source/unverified. No tests/fixtures/build/vet/types/product parser/SDK/browser/native/network/generation. Manual source/diff/byte review and touched-only gofmt do not prove compilation, source/tag/API graph, race freedom, actual syscall/peer/flow-control/0RTT compatibility, callback/Goexit interleavings, shutdown duration or speed.512 Read/Peek,64 controls, per-stream8/2 and native1 are versioned pressure ceilings, not measured optimal settings. Registry owns original receive operations from pre-allocation/pre-wait admission through cleanup/notifications. Independent native lane assumes the existing serialized Conn frame/framer/reset/shutdown dispatcher; it is not OS-native execution or a resource receipt for arbitrary providers. Early/abnormal original-loop completion is negative; unknown call tokens/credits remain retained after invocation ends. No forceful interruption, hard Stop SLO, whole-goroutine/RSS/CPU bound or global quota. Pure StreamID metadata and send-side/promoted controls, raw input/crypto/ACK/send history, empty stream metadata, allocation/GC/pool high-water, other connections/processes and whole QUIC/OS retirement remain outside this join. Existing data-only API retains its narrow scope. FullPC00..09/old9/rootNETGUARD001/100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported/wholeUX/release OPEN.

<a id="pc-v409-send-calls"></a>

## V2-409 — original SendStream calls в существующем владельце

Статус: PC-06 source/unverified. Quic-go revision11/sing-quic revision19;
frozen versions/manifests/licences прежние. V408 — progress: original receive
registry/close fences, source/docs и ledger обновлены. Полная цель сохранена.

### Reuse и границы общего реестра

- Существующий receive owner обобщён до streamCalls, без второго параллельного
  ledger. Шесть classes: read/read-control/native-read и write/write-control/
  native-write. Приём сохраняет512 data/64 controls; отправка имеет отдельные
  512/64. Per stream direction —8 data/2 controls. Native classes вместе занимают
  original serialized dispatcher lane1; production constructor/window/reset/
  framer/frame/shutdown paths используют прежнюю исходную сериализацию.
- Original token хранит явный *ReceiveStream/*SendStream и его счётчики. Это
  прямая удерживаемая ссылка, без зависимости от неявного GC-custody через адрес
  поля. Reservation предшествует token/map allocation и обеим blocking gates;
  неизвестный original/credit не заменяется новым объектом.
- Narrow receive settlement учитывает прежние первые три classes. Отдельный
  allDone требует конца всех шести. Original body+deferred-drain loop markers,
  Context ordering, Closing publication fence и independent receive data budget
  сохранены. Расширение не выдаёт receive API за общий frame/native join.

### SendStream calls и ожидания

- Bidi и outgoing-uni constructors передают существующий Conn budget. Write,
  Close, CancelWrite, SetReliableBoundary и SetWriteDeadline регистрируются
  до token/writeOnce/mutex wait. Занятые data slots сами по себе не расходуют
  control slots; known pressure использует прежний close notification.
- Write gate и data waits слушают owned closing channel. Stop не требует нового
  helper/timer/poller; initial/current state повторно проверяется под send mutex.
  Actual partial counts, already accepted bytes и known input-view clear остаются
  в прежнем write wrapper. Непринятый остаток не копируется после known close.
- Original token живёт через body, send-mutex/write-gate release и sender
  notifications. finishSendCall знает intentional unlocks; abnormal body/callback
  делает writeUncertain и publishes original Conn failure перед mutex/gate reuse.
  Inner write guard также связывает abnormal borrowed input с Conn uncertainty.
  Normal input view снимается до возврата вызывающему consuming wrapper.
- Registered native send calls: popStreamFrame, updateSendWindow,
  enableResetStreamAt, handleStopSendingFrame, getControlFrame, closeForShutdown.
  Flow-controller calls/notifications входят в исходный invocation lifetime.
  Guard охватывает known early returns, mutex release и sticky abnormal state.
  Pure StreamID/Context metadata не регистрируется как mutable producer.
- Новый WaitForStreamCallsShutdown ждёт original context и all-call result.
  HY2 original QUIC close сохраняет через errors.Join и этот результат, и
  прежний receive call/data result. Existing raw abort/TCP framing/half-close/
  HTTP3 joins остаются; CloseWrite/CancelRead не превращаются в новый транспорт.

### Transmitted frame custody остаётся отдельной работой

- Call tokens ограничивают операции, а не все queued/outstanding bytes/frames.
  ACK/loss/reset-history handlers пока не имеют собственного registration/
  original release identity. MaybeSplitOffFrame, nextFrame, retransmission queue,
  packetizer/ACK history и loss/reset/close paths требуют единого data owner до
  первой pool/copy allocation, с credit до настоящего последнего borrower.
- В текущем OnAcked есть PutBack до последующей stream bookkeeping. Факт виден
  в исходнике, но сам по себе не доказывает use-after-free: нужно проследить
  оставшиеся history/packer borrowers и callback returns. Этот риск не закрыт
  объявлением all-call receipt и не выдаётся за runtime-reproduced дефект.
- Shutdown после finishedWriting, partial pool return/queue compaction и
  immutable callback identity требуют того же последующего data-owner среза.
  Нет утверждения full transmitted-buffer/global/native cleanup или выигрыша
  скорости; resource/performance acceptance остаётся отдельным будущим gate.

### Evidence и следующий шаг

7Go2origin4docs;13 snapshots(12existing1new), manual8 existing Windows diffs
и полный send helper. Touched-only gofmt7 exit0, затем2 exit0 для явного original
stream reference.14 reviewed/0 additional cached inputs;261 cache originals/
231 verbatim mirrors и previous owned bytes сохранены. Source edits through
generalize.cjs — только точные private symbol substitutions, не product execution.
No tests/fixtures/build/vet/types/parser/SDK/browser/native/network/generation.

Далее — original transmitted STREAM data owner через reserve/copy/split/queue/
packer/ACK/loss/reset/shutdown и actual last release, без pool-address ABA и
раннего refund. Затем global/raw/crypto inventory. Native C1/TUN/OS, все страницы
UX, выпуск и весь PC-00…09/старые девять этапов остаются в полной цели.
Receipts: .artifacts/memory/bootstrap-v409-send-calls-.
Source/unverified. No tests/fixtures/build/vet/types/product parser/SDK/browser/native/network/generation. Manual source/diff/byte review and touched-only gofmt do not prove compilation, API/tag graph, race freedom, source/native caller compatibility, flow-control/reliable/half-close/0RTT behavior, callback/Goexit interleavings, shutdown duration, speed or protected egress. The single shared registry covers original registered calls: receive512/send512 data calls and separate controls64 each, per-direction8/2; native receive/send share one serialized constructor/Conn dispatcher lane. These are versioned pressure ceilings, not measured optimum or hard whole-process bounds. Original normal calls end after gate/mutex release and notifications; abnormal stream tokens/credits stay retained. Receive-only settlement keeps its original classes, while all-call settlement waits for six classes. ACK/loss/reset-history callbacks are not independently registered here; transmitted STREAM frame quota/immutable identity, split/retransmission/packer/raw history borrowers and true last release remain next linked OPEN work. Current OnAcked calls PutBack before its later stream bookkeeping; this is a source ownership risk requiring full borrower tracing, not a runtime-proven use-after-free. New call receipts do not prove transmitted data cleanup. Raw packets, crypto/ACK/send history, empty stream metadata, allocator/GC/pool high-water, arbitrary providers/context consumers, other connections/processes and whole QUIC/native/OS retirement remain outside scope. No worker/timer/direct fallback, forced cancellation, Stop/RSS/performance SLO or measured advantage. FullPC00..09/old9/rootNETGUARD001/100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported/wholeUX/release OPEN.

<a id="pc-v410-stream-send-data"></a>

## V2-410 — original transmitted STREAM data и попытки отправки

Статус: PC-06 source/unverified. Quic-go revision12/sing-quic revision20;
frozen versions/manifests/licences сохранены. V409 — progress: общий call owner,
send guards, source/docs/checkpoint изменены. Полная цель остаётся активной.

### Владелец до первой копии

- В existing Conn budget добавлен streamSendBudget:32768 original holders и
  32MiB exposed backing. Reservation до holder/map/frame/pool/payload allocation;
  pooled frame charges1452 bytes, empty FIN — count1/bytes0. Known pressure
  вызывает прежний close notification и отличается от uncertain retirement.
  First Closing signal проверяется при admission, без raw fallback.
- nextFrame и retransmissionQueue хранят sentStreamData, а не reusable raw
  frame pointers. Map держит original frame/stream и partial acquisition через
  actual return; newAttempt имеет отдельную immutable identity на каждую передачу.
  Старый успешный attempt callback после pool reuse остаётся noop.
- Small tail, new packetizer copy, next-frame remainder и retransmission split
  используют один allocator. Output резервируется до copy, пока source charged.
  Existing pooled split swaps private backings и копирует tail; оба1452-byte
  originals остаются charged. Generic raw wire split API не изменён и не служит
  fallback для owned SendStream. FIN/reliable offsets и partial counts сохранены.

### История, terminal handoff и освобождение

- STREAM FrameHandler больше не освобождает reusable frame до завершения
  history handoff. OnAcked/OnLost только отмечают original attempt. Existing
  ACK/loss history loops работают по индексам: callback returned → raw frame/
  handler cell cleared → optional OnStreamFrameReleased. Containing slice length
  сохраняется до прежнего Remove/DeclareLost/packet accounting; иначе меняется
  IsAckEliciting/Outstanding и теряется правильный counter decrement.
- Original release hook регистрирует native send call и использует прежний
  send mutex/abnormal guard. После explicit history release он применяет ACK/
  loss/reliable/reset bookkeeping, detach original attempt и либо передаёт loan
  в retransmission queue, либо завершает sender notifications и возвращает pool.
  Root frame/ref/credit очищаются только после actual PutBack return. Callback
  failure удерживает original и запрещает повторное использование/замену.
- Key-space/0RTT invalidation использует отдельный Discarded hook. Это не ACK
  доставки и не запрос retransmission. Legacy handlers без owned discard API
  сохраняют свой прежний drop contract. Shutdown/old0RTT stream не публикует
  позднее completion в новые reset maps. Generic RESET/control frame metadata
  остаётся отдельной областью и не объявляется принятой этим STREAM owner.
- Known post-FIN shutdown теперь освобождает current/queued loans, даже если
  finishedWriting уже true. Queue entries очищаются после actual release;
  retransmission pop зануляет удалённый slot. Unknown state не очищается как
  якобы успешное завершение. Caller input view остаётся под прежним write fence.

### Остатки после packer/error/close

- При packer/serialization error loan не исчезает из original map и не получает
  преждевременный refund: active attempt удерживает known-unsent data до конца
  исходного producer. Packer/framer/debug/qlog и normal history обработаны в
  исходном serialized Conn loop; STREAM qlog хранит metadata, не payload view.
- Новый WaitForStreamSendDataShutdown сначала ждёт original Context и положительный
  WaitForStreamCallsShutdown. Только затем один исходный finalizer sealed data
  owner освобождает остаточные queued/outstanding/known-unsent originals. Его
  snapshot≤32768 pointers ограничен owner count и избегает repeated sparse-map
  scans; новых worker/timer нет. При отрицательном loop/call/data результате
  sweep не выполняется, references/credits сохранены. Concurrent waits получают
  тот же original result; abnormal finalizer не становится новым healthy owner.
- Эта quiescent граница опирается на фактические private loop/call producers,
  а не на произвольную задержку или один Context label. Старые private aliases
  packer/history не имеют продолжающегося producer; late old attempt закрыт до
  pool return. Это не whole QUIC/raw/native/OS retirement.
- Original HY2 QUIC close joins новый data result вместе с receive/call joins;
  independent raw abort, TCP/HTTP3 owners и отказ direct fallback сохранены.

### Evidence и следующий объём

9Go2origin4docs;15 snapshots(11existing4new), manual7 existing Windows diffs
и4 полных helper. Touched-only gofmt9 exit0, затем2 exit0 для named terminals/
bounded finalizer snapshot.15 reviewed/0 additional cached inputs;261 cache
originals/230 verbatim mirrors и previous owned bytes сохранены. Obsolete private
raw data ACK handler удалён точными source bounds, RESET metadata handler сохранён.
No tests/fixtures/build/vet/types/parser/SDK/browser/native/network/generation.

Далее — bounds/ownership raw packet и ACK/control history metadata (включая
packet-number gaps), crypto producers и global carrier inventory. Native C1/
TUN/OS, все страницы UX, выпуск, весь PC-00…09 и старые девять этапов сохранены.
Source implementation не выдаётся за peer/native/resource/performance приёмку.
Receipts: .artifacts/memory/bootstrap-v410-stream-send-data-.
Source/unverified. No tests/fixtures/build/vet/types/product parser/SDK/browser/native/network/generation. Manual source/diff/byte review and touched-only gofmt do not prove compilation, API/tag compatibility, race freedom, wire/flow-control/ACK/loss/retry/0RTT/reliable/half-close behavior, callback/Goexit/pool interleavings, actual quiescence/native protection or speed.32768 owners/32MiB per-Conn transmitted backing and32768 finalizer pointers are versioned engineering ceilings, not measured optimum or whole-process bounds. Pooled frames charge1452 bytes, empty FIN counts as one zero-byte holder. Current/queued/outstanding/known-unsent/split payload stays charged until actual pool return; original call/loop source serialization and metadata-only tracing are required. The explicit history-cell handoff retains containing slice length, not a generic ACK metadata retirement receipt. Residual data sweep requires positive Context/original-loop/all-call receipts; uncertainty retains originals and refuses sweep/retry/replacement. Arbitrary custom providers/hidden borrowers, raw packet/serialized buffers, ACK/control/reset history metadata, crypto frames, empty-stream metadata, allocator/GC/pool high-water, caller input, other connections/processes, global custody and whole QUIC/native/OS retirement remain OPEN. No new worker/timer/raw fallback, forced cancellation, hard Stop/RSS/latency SLO or measured advantage. Existing TUN unavailable/schema2 unsupported/rootNETGUARD001/100OPEN56NOT_RUN/fullPC00..09/old9/wholeUX/release gates unchanged.

<a id="pc-v411-sent-history"></a>

## V2-411 — sent history admission и original completed output

Статус: PC-06 source/unverified. Quic-go revision13/sing-quic revision21;
versions/manifests/licences сохранены. V410 — progress, а не закрытие полного
ПК-плана. Все PC-00…09 и прежние девять этапов остаются в текущей цели.

### Фактическая история и admission

- Текущая frozen реализация хранит sequential []*packet, включая nil holes и
  path stubs. Это не packetNumberIndexedQueue. SendMode уже ограничивает общий
  tracked count50000 и new-data threshold40000; отсутствия защиты от роста не
  утверждаем. Sequential PN, Len/ACK/Difference и last-four skips сохранены.
- beginTracking до getPacket проверяет sticky failure и per-space cap50000,
  включая path-probe vector. Explicit grow clamps physical capacity50000,
  не полагаясь на Go append growth. Исходный pending packet прикрепляется до
  внешнего congestion callback; reentrant/invalid original не затирается.
  Capacity refusal отличается от uncertain intake. Handler failure sticky
  через Retry/key-space/path replacement; существующие thresholds не повышены.
- Internal SentPacket теперь возвращает error. Все production Conn callers
  проверяют его до physical send; path branch и long/short coalesced callers
  также проверены вручную. Explicit PTO skip возвращает ошибку через существующий
  timeout result; automatic skip сохраняет sticky failure для регистрации.

### Completed output: raw buffer и DATAGRAM prefix

- Один inline Conn slot принимает original completed packet/batch до tracing,
  history и queue/provider handoff. Non-GSO/GSO ownership начинается сразу
  после getBuffer; ACK/coalesced/MTU/path outputs принимаются после successful
  packer return. Partial packer allocations отдельно остаются открытыми.
- Fixed DATAGRAM pointer array ограничен MaxLargePacketBufferSize/
  MinInitialPacketSize+1: этот packer выдаёт максимум один DATAGRAM на packet,
  а GSO добавляет следующий только после full-size segment; один tail разрешён.
  Known pre-handoff errors возвращают все unsent history refs текущего batch,
  включая уже зарегистрированный GSO prefix, затем actual raw Release. History
  повторного ACK/loss не выдаёт второй credit. STREAM loan остаётся под своим
  original data/attempt/quiescent owner, а не освобождается по raw-buffer label.
- Send/SendProbe transfer flag ставится до actual call. Только ordinary return
  снимает slot, не перечитывая raw buffer, который queue могла вернуть в pool.
  Abnormal transfer/pool return удерживает original и запрещает positive loop
  receipt. New output не заменяет неразрешённого original.
- Alternate Transport.WriteTo принимает borrowed []byte, не packetBuffer owner.
  Ранее после успешного path-probe send исходный buffer не возвращался в pool.
  Теперь actual original сохраняется через logger/register/WriteTo, short/error
  return обрабатывается явно, обычный return возвращает buffer. Invalid counts
  и abnormal provider return не превращаются в known-unsent cleanup.

### Evidence и ограничения

6Go2origin4docs;12 snapshots(10existing2new), manual6 existing Windows diffs и2
полных helper. Touched-only gofmt6 exit0, затем1 exit0 после уточнения comment.
12 reviewed/0 additional cached inputs;261 cache originals/228 verbatim mirrors
и previous owned bytes сохранены. Source/diff review не является compile/API/
runtime доказательством. No tests/fixtures/build/vet/types/parser/SDK/browser/
native/network/generation. Native TUN gate остаётся false, schema2 unsupported.

Далее — original partial packer allocations, crypto/raw receive/queue providers,
ACK/control/CC sampler custody и global carrier inventory; затем сохранённые
C1/nativeOS/wholeUX/release/full-PC gates. Не заявляется whole RSS/Stop SLO,
безотказность или измеренное превосходство. Receipts: .artifacts/memory/bootstrap-v411-sent-history-.
Source/unverified. No tests/fixtures/build/vet/types/product parser/SDK/browser/native/network/generation. Manual source/diff/byte review and touched-only gofmt do not prove compilation, internal interface/tag compatibility, races, wire/ACK/loss/retry/0RTT/PTO/GSO behavior, callback/Goexit/pool/provider interleavings, native protection or speed. The frozen history is a sequential pointer slice with existing40000/50000 scheduler thresholds, not evidence of an unbounded indexed queue. Hard capacity50000 applies per number space to history/probe vectors, not whole connection/process memory. Output DATAGRAM references rely on this packer emitting at most one per packet and GSO appending only after a full segment, with one tail. Completed packer/current-batch output is covered, not partial packer errors, raw receive/queue/provider owners, full ACK/control/crypto payloads, CC/ECN sampler retention, allocator/GC/pool high-water, aggregate retiring carriers, OS/global/RSS or performance. STREAM data keeps its separate original loan/attempt/quiescent owner. Unknown original/transfer/pool return is retained; no forced cancellation, new worker/timer/direct fallback or hard Stop/latency SLO. TUN unavailable/schema2 unsupported/rootNETGUARD001/100OPEN56NOT_RUN/fullPC00..09/old9/wholeUX/release remain OPEN.

<a id="pc-v412-packer-buffers"></a>

## V2-412 — original partial packet packing

Статус: PC-06 source/unverified, quic-go revision14/sing-quic revision22.
V411 — progress, полный PC-00…09/old9 остаётся активным; версии/лицензии сохранены.

- packetPackerIntake резервирует одну serialized operation до callbacks/pool.
  Seven wrappers покрывают coalesced, close(App/transport), ACK, Append, PTO,
  MTU и path entry paths без изменения constructor/packer signatures. Original
  partial buffer и bounded DATAGRAM entry references сохраняются до return;
  повторный вход/abnormal original не заменяет прежнего владельца.
- DATAGRAM entry удерживается до queue Pop, а не лишь после serialization.
  Поэтому ошибка раннего coalesced packet возвращает credit уже подготовленного,
  ещё не appended payload. Existing queue credits остаются единственными;
  отдельного quota/refund счётчика нет. Array bound переиспользует original
  completed-output limit; новых dynamic queues/workers/timers нет.
- Ordinary error/empty output после actual packing return возвращает consumed
  history refs и owned raw buffer. ACK/MTU/path errors больше не теряют buffer
  и не возвращают stale pooled pointer. Local serializer defers больше не
  освобождают payload во время unresolved abnormal borrow.
- AppendPacket только borrows original Conn/GSO buffer; его known-error cleanup
  не делает raw Release. Conn возвращает known-unsent batch/prefix по прежнему
  owner. errPacketOutputUncertain удерживает обе original references, блокирует
  pool return и positive original-loop receipt; successful return передаёт
  результат existing completed-output owner. STREAM data/quiescent owner сохранён.
- Фактический SendProbe вызывает synchronous WriteTo с borrowed bytes и ранее
  не делал Release. Теперь actual ordinary return consumes original buffer;
  abnormal WriteTo/Release остаётся unresolved в Conn transfer owner. Прежний
  void SendProbe по-прежнему не передаёт write error, это отдельный open contract.

5Go2origin4docs;11 snapshots(10existing1new). Intake snapshot расширен до правки
после обнаружения cross-owner GSO error propagation; original bytes сверены с
V411. Manual6 existing Windows diffs+1 полный helper, touched-only gofmt5 exit0.
12 reviewed/0 additional cached inputs;261 cache originals/227 verbatim mirrors
и prior owned bytes сохранены. No tests/fixtures/build/vet/types/parser/SDK/
browser/native/network/generation; source review не является compile/runtime PASS.

Следующий объём — sendQueue original buffers при stopped/fatal/abnormal write,
seal/admission/drain ownership и actual Close result; затем crypto/raw receive/
CC/global inventory. C1/nativeOS/TUN/schema2/wholeUX/release/full-PC не закрыты.
Receipts: .artifacts/memory/bootstrap-v412-packer-buffers-.
Source/unverified. No tests/fixtures/build/vet/types/product parser/SDK/browser/native/network/generation. Manual source/diff/byte review and touched-only gofmt do not prove compilation, private helper/API/tag compatibility, races, wire/ACK/loss/retry/0RTT/PTO/GSO semantics, callback/Goexit/queue-close/pool interleavings or native protection/performance. The intake bounds one serialized packer operation/owned pool buffer plus fixed DATAGRAM reference array; it reuses existing credits and pool, not a new whole-process byte budget. AppendPacket borrows a separately owned Conn/GSO original. Public production entry paths use guards; private payload helpers are not an independent lifetime API. Unknown original/borrow/transfer/pool result retains roots and denies a positive original-loop receipt. Successful results move to existing completed-output custody; terminal-close raw/cache/provider lifetime remains separate. SendProbe now consumes its buffer after ordinary WriteTo return but still has the preexisting void/write-error policy. Queued/stopped/fatal/abnormal send-queue paths, raw receive/transport providers, crypto/control/CC/ECN samplers, global retiring carriers, hidden borrowers, allocator/GC/pool high-water/RSS and whole QUIC/nativeOS retirement remain OPEN. No worker/timer/direct fallback, forced cancellation, hard Stop/latency SLO or measured advantage. Full PC00..09/old9/wholeUX/release/100OPEN56NOT_RUN/TUN unavailable/schema2 unsupported/rootNETGUARD001 remain OPEN.

<a id="pc-v413-send-queue"></a>

## V2-413 — original sendQueue custody и close outcome

Статус: PC-06 source/unverified, quic-go revision15/sing-quic revision23.
V412 — progress; весь PC-00…09/old9 сохранён, versions/manifests/licences прежние.

- Channel select больше не может принять пакет после остановки worker: admission
  и seal используют один metadata fence. Fixed ring8 и nine normal slots дают
  pending8+worker1; separate probe slot ограничивает synchronous probe1. Inline
  batch сохраняет raw original и≤18 DATAGRAM refs по20KiB/1200 invariant. Capacity
  buffer1452/20480 проверяется; copy после slot reservation, без dynamic pending
  queue/per-packet worker. Provider/pool/error methods не выполняются под mutex.
- Internal sender Send/SendProbe теперь возвращают accepted и error отдельно:
  refusal оставляет original у caller, acceptance передаёт reserved queue slot,
  даже если synchronous write позже вернул ошибку. Все production Conn outputs
  проверяют результат; unknown transfer сохраняет root, known refusal освобождает
  caller's batch. Публичные QUIC/HY2 API не изменены; internal mocks/ABI unverified.
- Ordinary failed write и fatal-drain entries возвращают original DATAGRAM
  credits один раз и raw buffer после actual provider/pool return. Successful
  writes сохраняют прежний history owner. Existing owned DataLen immutable;
  ACK/loss/debug/qlog не читают retired payload после serialization. Unknown
  write/pool/count outcome удерживает roots, seals reuse и не запускает cleanup
  заново. Fatal error сначала seals, затем known queued originals drain без
  новых writes. EMSGSIZE/DatagramTooLarge сохраняют прежний loss/MTU contract.
- Close idempotent, joins actual original Run+admitted probe, возвращает retired
  отдельно от stored write error. Negative completion не становится permission
  заменить carrier. Initial Run queue захватывается до handshake, early exit
  имеет one-attempt close guard. Actual retirement участвует в loop receipt.
  Path replacement ждёт retired/error-free old queue до congestion/MTU/state
  mutation; queue failure не сохраняет internal recreation exception и не
  разрешает новый terminal CONNECTION_CLOSE write.
- sconn проверяет actual WritePacket counts: short/bytes-plus-error — fatal,
  original error сохранён, GSO prefix replay запрещён. Permission retry только
  при zero bytes. Invalid counts дают uncertainty и retention, не normal cleanup.

5Go2origin4docs;11 snapshots(10existing1new), manual6 existing Windows diffs+1
полный helper; touched-only gofmt5 exit0.11 reviewed/0 additional cached inputs;
261 cache originals/227 verbatim mirrors и prior owned bytes сохранены. No
tests/fixtures/build/vet/types/parser/SDK/browser/native/network/generation.

Далее — terminal CONNECTION_CLOSE/cache/retry и transport output originals;
затем crypto/raw receive/CC/global inventory. C1/nativeOS/TUN/schema2/wholeUX/
release/full-PC gates остаются открытыми. Receipts: .artifacts/memory/bootstrap-v413-send-queue-.
Source/unverified. No tests/fixtures/build/vet/types/product parser/SDK/browser/native/network/generation. Manual source/diff/byte review and touched-only gofmt do not prove compilation, changed internal sender/mocks/API/tag compatibility, race freedom, wire/GSO/MTU/ACK/loss/path/early-close/recreate behavior, callback/Goexit/pool/queue interleavings or speed. Fixed8 pending plus worker1/probe1 and18 DATAGRAM references per slot are per-queue engineering bounds, not measured optimum, whole API caller limits, allocator/GC/pool high-water or RSS. Input raw pool capacities1452/20480 are validated; successful queue transfer does not prove packet delivery or protected egress. Original provider/pool calls and foreign error inspection run outside metadata lock. Known ACK/loss/frame logging uses immutable owned DATAGRAM length after serialization; arbitrary hidden async payload borrowers/nonconforming providers remain outside this receipt. Unknown original write/pool/count custody retains roots and denies healthy completion/replacement; no retry cleanup, new worker/timer/direct fallback, forced cancellation, provider-panic containment or hard Stop/latency SLO. Close requires the scheduled original Run and admitted probe to actually finish. Terminal CONNECTION_CLOSE/cache/retry producers, transport/raw receive, crypto/control/CC/ECN samplers, global retiring carriers and whole QUIC/nativeOS remain OPEN. FullPC00..09/old9/wholeUX/release/100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported/rootNETGUARD001 unchanged.

<a id="pc-v414-terminal-output"></a>

## V2-414 — terminal packet pool custody и detached cache

Статус: PC-06 source/unverified, quic-go revision16/sing-quic revision24.
V413 — progress; весь PC-00…09/old9 сохранён, versions/manifests/licences прежние.

- Existing Conn packetOutput принимает original terminal buffer до ECN/trace/
  native Write. Actual ordinary Write return завершает raw borrow, но buffer
  остаётся у Conn до ordinary return всех ReplaceWithClosed callbacks. Только
  затем original Release. Unknown count/callback/pool/borrow state сохраняет
  root и не даёт positive loop receipt; empty/oversized output не отправляется.
- Раньше cache/queued retry удерживали срез исходного pooled packet, поэтому
  безопасного pool return не было. Теперь каждый Transport перед публикацией
  closed handler делает independent immutable backing≤1452 bytes. Original
  pool buffer не заимствуется timer/queue retry. Empty/oversized cache input не
  создаёт payload copy. Callback/constructor signatures и backoff не изменены.
- Local и remote closed handlers consume один incoming buffer ref через
  Decrement/MaybeRelease, сохраняя shared-buffer semantics. Original data/buffer/
  address/info view очищается после actual pool operation и до logger/retry.
  Local address snapshot захватывается до input release через existing bounded
  packetaddr API; после него используются только immutable address facts/info.
- Expiry удаляет ID лишь при совпадении exact original closed handler. Remote
  handler больше не zero-sized: отдельные пустые Go objects могут иметь равные
  pointers, поэтому private identity byte нужен для этой проверки. Registration
  authority/late callback admission этим не доказывается.

4Go2origin4docs;10 snapshots(9existing1new), manual5 existing Windows diffs+1
полный helper; final touched-only gofmt4 exit0.12 reviewed/0 additional cached
inputs;261 cache originals/226 verbatim mirrors и prior owned bytes сохранены.
No tests/fixtures/build/vet/types/parser/SDK/browser/native/network/generation.

Далее — aggregate per-Transport closed-cache count/byte/timer/ID admission,
registration identity и actual retry/control-worker retirement, включая
stateless reset producers. Crypto/raw receive/CC/global/nativeOS/wholeUX/release/
full-PC gates не закрыты. Receipts: .artifacts/memory/bootstrap-v414-terminal-output-.
Source/unverified. No tests/fixtures/build/vet/types/product parser/SDK/browser/native/network/generation. Manual source/diff/byte review and touched-only gofmt do not prove compilation/API/tag compatibility, race freedom, wire/close/retry/expiry/multi-runner/shared-buffer behavior, callback/Goexit/pool/provider interleavings or speed. Each retry copy has at most1452 exposed bytes and independent backing; this is not aggregate cache count/byte/timer/ID admission, RSS/GC/allocator high-water or a measured optimum. Original Conn buffer remains owned through actual Write and all production ReplaceWithClosed callbacks; abnormal/uncertain originals remain retained. Known closed-handler input return consumes one ref and clears local view; arbitrary opaque address callback/hidden borrower lifetime is not a full receive/transport completion receipt. Existing constructor/callback signatures and best-effort queue/backoff remain. Expiry identity checks do not prove registration/admission authority, bounded CID history or timer-task quiescence. Transport still lacks a joined retry/control-worker owner and global retiring budget; queued copies/active provider views are not covered by Conn loop retirement. No new worker/timer/direct fallback, forced cancellation, provider-panic containment, hard Stop/latency SLO or measured superiority. Aggregate cache/timers/control output, stateless reset, raw receive/crypto/CC/global/nativeOS/fullPC00..09/old9/wholeUX/release/100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported/rootNETGUARD001 stay OPEN.

<a id="pc-v415-closed-cache"></a>

## V2-415 — bounded closed cache и original control output

Статус: PC-06 source/unverified, quic-go revision17/sing-quic revision25.
V414 — progress, весь PC-00…09/old9 сохранён; dependency versions/licences прежние.

- До owner/map/copy/ID/timer allocation резервируются256 records и384KiB exposed
  payload; record≤64 IDs, body≤1452. Remote/opening/retiring/unknown учитываются.
  Capacity refusal удаляет только expected-owned IDs и не создаёт substitute
  tombstone/timer. Это engineering best-effort cache policy, wire/overload OPEN.
- Три production runner callbacks несут original Conn. Authority учитывает
  current Conn и wrappedConn.Conn, потому что начальные handlers представлены
  wrapper. Publication повторяет check после подготовки; expiry/cache withdrawal
  не удаляют новый чужой handler. Generic CID/reset-token Remove — другой open
  контракт. Internal runner/closePacket/helper ABI изменён, compile/mocks OPEN.
- Immutable owner/ready fence защищает registration против early expiry/close.
  Timer pending регистрируется до AfterFunc; actual Stop=true отменяет original
  callback, Stop=false оставляет его pending до actual callback return. Cache
  withdrawal и все queued/current retry refs нужны до refund. Unknown owner/
  timer/borrow сохраняет roots/credits; failure completion не даёт replacement.
- Cache/retry/stateless admission seals с Transport.close под metadata mutex.
  Existing control worker хранит original retry/OOB и stateless input/output/OOB
  до actual provider/pool return; queued known refs drain после seal. Automatic
  single-use read-stop coalesced в тот же worker, original rawConn удерживается
  через deadline return. Дополнительных workers/timer classes нет.
- Public Close joins worker и cache completion после listener return. Init/
  native-close/deadline errors negative; invalid counts/abnormal completion не
  превращаются в healthy result. Best-effort packet write-error policy сохранён;
  reset token больше не печатается debug logger. Provider/error/pool callbacks
  выполняются вне transport metadata lock. Этот join не равен whole Transport
  rootClose/listener/tracer/caller/native retirement.

5Go2origin4docs;11 snapshots(9existing2new), manual5 existing Windows diffs+2
полных helper; final touched-only gofmt5 exit0.13 reviewed/0 additional cached
inputs;261 cache originals/226 verbatim mirrors и prior owned bytes сохранены.
No tests/fixtures/build/vet/types/parser/SDK/browser/native/network/generation.

Далее — original root Close/init/listener и direct Transport.WriteTo admission/
completion; raw receive/CID retiring producers, crypto/CC/global inventory и
nativeOS/wholeUX/release/full-PC gates остаются открытыми. Receipts: .artifacts/memory/bootstrap-v415-closed-cache-.
Source/unverified. No tests/fixtures/build/vet/types/product parser/SDK/browser/native/network/generation. Manual source/diff/byte review and touched-only gofmt do not prove compilation, changed internal connRunner/closePacket/private mocks/helper ABI or tag compatibility, race freedom, wire/RFC overload/closing/draining behavior, timer.Stop/expiry/registration/queue/provider/pool/Goexit interleavings or speed.256 records/384KiB exposed payload/64 IDs/1452-byte copy are per-Transport engineering ceilings, not measured optima, global connection/caller counts, physical goroutine/heap/GC/RSS/CPU or Stop/latency guarantees. Capacity refusal drops best-effort closed cache only for expected-owned IDs; this needs later wire/overload acceptance. Existing expiry timers and one control worker are retained, with one coalesced wake channel; no extra worker/timer class/direct fallback. Credits follow original withdrawal, actual Stop=true/callback return and all retry refs; unknown roots/credits remain and completion is negative, never permission to replace. Providers/error methods/pool calls execute outside transport metadata locking. Public Close joins this control worker/cache, not every listener/root-close/tracer/initialization/caller/direct-WriteTo producer; native abort/deadline errors remain negative. Noncooperative/hidden provider borrows and panics are not forcibly contained. Active/retiring CID producers, generic CID/reset-token Remove authority, raw receive/crypto/CC/global retiring/nativeOS/wholeUX/release/fullPC00..09/old9/100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported/rootNETGUARD001 remain OPEN.

<a id="pc-v416-transport-lifecycle"></a>

## V2-416 — один исходный результат Transport.Close

Статус: PC-06 source/unverified; quic-go revision18/sing-quic revision26.
Полная цель PC-00…09 и прежние девять этапов сохранены. Ответ о55% не был
разработкой (no progress); этот срез меняет исходники (progress). V415 сохранён.

### Реализовано в исходниках

- Отдельный lifecycle mutex защищает только metadata: Transport.mutex может
  предшествовать ему, обратного порядка нет. Native I/O, provider callbacks,
  error methods, handler destroy и Recorder.Close выполняются вне этого mutex.
  Исходные socket/recorder фиксируются до первого callback; root хранит original
  server и отдельные destroy rows до фактического нормального возврата.
- Init callers≤64 регистрируются до sync.Once wait. Body повторно проверяет seal
  перед native preparation; abnormal init не превращается в nil через Once.
  Начатая и ещё не начатая инициализация различаются. Close не запускает новую
  инициализацию; known unscheduled loops не выдаются за существующие workers.
  Закрытие first seals admission, затем вызывает один original socket abort.
- Owned socket Close либо borrowed read-deadline interruption имеют один исходный
  результат; повторный Close не повторяет native action. Known net.ErrClosed
  принимается как уже закрытый socket. Ошибка/abnormal return сохраняет originals.
  Обычная ошибка init завершает original call, но не доказывает готовность VPN.
- Internal root cleanup имеет один owner/done/result. Второй internal caller
  не ждёт сам себя из listener/control worker; public Close ожидает original root.
  closeErr от закрытого single-use server не пропускает cleanup. Handler rows
  deduplicate original Conn/wrappedConn across CID/reset maps; destroy goroutines
  always account actual exit, including Goexit. Known Conn retirement использует
  существующие receive-data и stream-call/send-data joins; negative row сохраняет
  original handler. Карты очищаются только после positive handler results.
- Server close хранит оригинальный результат, отпускает metadata mutex перед
  callback/waits и ожидает receive + response-send loops. Каждый loop отдельно
  отмечает ordinary return. Новый server не заменяет ещё closing/failed original.
  В AddWithConnID refusal найдено ожидание ctx.Done до запуска candidate loop:
  теперь nonwaiting close request предшествует terminal loop в исходной server
  task. Candidate остаётся attached до positive existing Conn receipts; unknown
  прекращает ingress и не разрешает перезаписать original следующим Initial.
- Listener имеет normal marker. Recorder.Close выполняется один раз после
  исходных listener/server producers и положительного root result; его ошибка
  не игнорируется. Concurrent/public Close наблюдает тот же actual trace outcome.
  Automatic read-stop coalesced в прежний control worker и использует тот же
  original native abort; нет ещё одного прямого deadline retry под map mutex.
- Direct WriteTo резервирует≤64 calls/4MiB exposed input spans/≤65535 на вход до
  token/map allocation, init wait или provider. Original input, concrete net.Addr
  и rawConn сохраняются через actual WritePacket и count/error inspection.
  Ordinary known return очищает views и credits; invalid count/abnormal exit
  сохраняет original token/views/credits и даёт negative completion. Из этой
  connection-loop операции нет синхронного Close, который ждал бы её саму.
- Public Close имеет original outer done/result. Positive path joins root,
  listener/control/cache/direct writes/recorder до clear временного borrowed read
  deadline. Negative early return сохраняет оставшиеся original roots; deadline
  не очищается и cleanup не перезапускается. CID/reset/cache admission учитывает
  lifecycle seal, cache повторяет его при publication. Public signatures,
  frozen module versions/licences/quotas HY2 не изменены.

### Область просмотра и ограничения

6 Go +2 ORIGIN +4 docs;12 snapshots(10existing2new). Просмотрены6 existing Windows
diffs и2 полных новых helper; touched-only gofmt6 exit0.15 reviewed inputs,
0 additional cache reads;261 frozen cache originals/226 verbatim mirrors и
предшествующие owned bytes сохранены. Новая серверная часть добавлена в pre-write
snapshot после обнаружения незавершённого response-send worker.

Не добавлялись/не менялись/не запускались tests/fixtures; build/vet/typecheck/
product parser/SDK/browser/native/admin/network/generation не выполнялись.
Далее: original raw receive/drop/server queue views и terminal drain; затем
generic CID/reset-token removal, constructor/control producers и crypto/CC/global.
Это не whole-QUIC/OS или full-PC приёмка; TUN unavailable, schema2 unsupported.
Receipts: .artifacts/memory/bootstrap-v416-transport-lifecycle-.
Source/unverified only. No tests/fixtures/build/vet/types/product parser/SDK/browser/native/admin/network/generation/dependencies/other roles/chats/subagents. Manual source/diff/byte review and touched-only gofmt do not prove compile, changed internal structs/private helper/mocks/tag compatibility, race freedom, Goexit/panic/provider/Once/server/handler/waiter/write/deadline/recorder interleavings, wire/RFC/collision behavior or speed.64 direct calls/4MiB exposed spans/65535 per input and separate64 initializer waiters are per-Transport engineering ceilings, not backing-allocation/RSS/global-caller/goroutine/CPU/Stop SLO or measured optima. Original handler-destroy fan-out is deduplicated, not globally capped. Abnormal completion retains original references and credits and returns a negative result; it never authorizes replacement or native retry. Public Close joins these original tasks on the positive path; a negative early result retains remaining original producers and does not clear the borrowed deadline. Noncooperative callbacks may block indefinitely, native/provider panics are not forcibly contained, and callbacks must not synchronously close their own Transport/server (self-join). Clearing the temporary borrowed read deadline does not reconstruct an unknown earlier deadline. Raw receive/drop/server queue buffer custody, constructor/control call inventory, generic CID/reset-token removal authority, crypto/CC/global retirement/nativeOS/wholeUX/release/fullPC00..09/old9 remain OPEN. All100 acceptance OPEN/56 checks NOT_RUN/72tasks active/gatesfalse/TUN unavailable/schema2 unsupported/rootNETGUARD001 untouched.55% remains a rough unmeasured estimate, not accepted readiness.

<a id="pc-v417-raw-receive"></a>

## V2-417 — original Windows raw read и входные ref Transport

Статус: PC-06 source/unverified, quic-go revision19/sing-quic revision27.
V416 — progress; этот срез даёт progress, весь PC-00…09/old9 сохранён.

- Windows newConn выбирает basicConn. Его один active reader регистрируется
  до pool acquisition/provider; original PacketConn, current buffer и returned
  address сохраняются через actual ReadFrom/count validation. Concurrent read
  отказывает до новой allocation/native call. n<0/n>input length или abnormal
  pool/provider return оставляют original slot/refs и запрещают reuse. Обычная
  error return возвращает не переданный потребителю buffer ровно один раз.
- Transport хранит original raw source и returned packet перед error methods,
  logger/tracer и handler callback. Unknown input/source остаётся attached.
  Known empty/header-parse/no-server/matched-reset drops используют прежний
  Decrement+MaybeRelease helper; одного MaybeRelease при refCount1 недостаточно.
  Error/temporary/truncation paths освобождают возвращённый custom packet тоже;
  temporary-close check учитывает lifecycle seal. Nonempty nil-buffer input
  получает negative result вместо недоказанной передачи/успеха.
- Production matched stateless reset использует nonwaiting destroyImpl original
  Conn/wrappedConn; новая goroutine на каждый duplicate packet больше не нужна.
  Private custom handler выполняется в исходной listener task под её failure
  guard. Reset-token bytes больше не попадают в debug log этого receive path.
- Fixed32 nonQUIC channel создаётся в original initializer до goroutines;
  конкурентная lazy initialization/replacement удалена. Publication и seal
  используют lifecycle mutex. Disabled/full/sealed input returns one shared ref;
  прежнее отсутствие drop tracing при отключённом nonQUIC reader сохраняется.
- ReadNonQUICPacket reserves64 calls/4MiB exposed destination spans до token/map,
  init и context waits. Original context, destination и queued input attached
  до actual copy, standard address snapshot и pool-return. Opaque concrete
  addresses сохраняются для custom PacketConn, без скрытой замены их типа.
  Ordinary outcomes очищают views/credits; abnormal exit сохраняет оригиналы,
  seals admission и публикует negative completion. Close wake не создаёт timer.
- После original listener + read-call positive joins один исходный drain берёт
  каждый queued input в retained slot и освобождает ровно1 ref. Public Close и
  natural listener exit наблюдают тот же actual drain result, не повторяют pool
  cleanup. Unknown pending input/read/context не очищается speculative retry.
  Recorder retirement и clear borrowed deadline остаются за positive composition.
- Сначала закончена граница native read→Transport/drop/nonQUIC consumer. Она
  нужна перед следующим Conn/server ingress/drain срезом. Per-Transport aggregate
  raw budget пока не введён: блокировать read при занятом всеми0RTT/undecryptable
  буферами лимите без отдельной ingress policy было бы ошибкой liveness.

6 Go +2 ORIGIN +4 docs;12 snapshots(10existing2new), manual6 existing Windows
diffs +2 полных новых helper. Touched-only gofmt6 exit0;13 reviewed inputs,
0 additional cache reads;261 frozen cache originals/226 verbatim mirrors и
предшествующие owned bytes сохранены. Public signatures/versions/licences/HY2
quotas прежние; no new worker/timer/direct fallback. Tests/fixtures не добавлялись,
не менялись и не запускались; build/vet/typecheck/product parser/SDK/browser/
native/admin/network/generation не выполнялись.

Далее: Conn queue refusal/after-close fence, original coalesced/version-negotiation/
undecryptable buffers; server receive/0RTT/response queue terminal drain и output
custody. Generic CID/reset removal/constructor/control/crypto/CC/global/OS/UX/
release/fullPC не закрыты. Receipts: .artifacts/memory/bootstrap-v417-raw-receive-.
Source/unverified. No tests/fixtures/build/vet/typecheck/product parser/SDK/browser/native/admin/network/generation/dependency changes/other roles/chats/subagents. Manual source/diff/byte review and touched-only gofmt do not prove compile, changed internal basicConn/Transport private-helper/mocks/tag compatibility, race freedom, error/provider/pool/handler/context/count/Goexit/deadline/drain/Close interleavings, wire behavior or speed.1 current basic read buffer1452,32 queued nonQUIC references,64 registered reads/4MiB exposed destination spans are local engineering bounds; not aggregate QUIC/server raw backing, physical stacks/goroutines/heap/GC/RSS/CPU or measured Stop/latency limits. Read spans are charged by length, not full backing allocation. Custom opaque address type is preserved; arbitrary provider/address/context allocations/borrows are not bounded or forcibly interrupted. Hidden providers and non-Windows OOB/batch readers have separate open scope. Existing shared refcount release is reused for known input drops only; no blanket Release on abnormal callback exit. Unknown original input/context/provider/pool state retains roots/credits and gives a negative result, never native/pool retry or replacement authority. Native/error/context/pool callbacks execute outside lifecycle/basic-reader metadata locks; existing server delegation under Transport.mutex remains its own next source task. Callbacks must not synchronously Close their own owner (self-join). Conn queue-full/after-close admission, raw coalesced/version-negotiation/undecryptable processing, server receive/0RTT/response queue/drain/output custody, generic CID/reset removal, constructors/control/crypto/CC/global/nativeOS/wholeUX/release/fullPC00..09/old9 remain OPEN. All100 acceptance OPEN/56 checks NOT_RUN/72tasks active/gatesfalse/TUN unavailable/schema2 unsupported/rootNETGUARD001 unchanged;55% remains rough/unmeasured.

<a id="pc-v418-conn-input"></a>

## V2-418 — original Conn packet-input completion

Статус: PC-06 source/unverified; quic-go revision20/sing-quic revision28.
V417 — progress; текущий срез меняет исходники (progress), весь PC-00…09/old9 сохранён.

- 64 fixed original intake cells/64*20KiB backing-span credits резервируются
  до receive-mutex wait. Slot и processing cell не требуют отдельной allocation
  на каждый пакет. Unknown cell остаётся attached/charged и не переиспользуется;
  normal return освобождает его после actual trace/pool/queue handoff. При refusal
  исходный upstream listener/server держит rejected input до фактического release.
- Existing receive queue≤256 сохраняется. Enqueue проверяет original close
  decision/seal под receivedPacketMx→input.access; обратного lock order нет.
  Winner close и early-loop exit seals до terminal waits. Known full/late input
  освобождает1 ref, trace/pool callbacks выполняются вне обоих metadata mutexes.
  Это закрывает missing drop release и enqueue-after-terminal-drain гонку.
- До crypto/trace callback фиксируется original datagram + дополнительный anchor
  ref. Каждый long/short handler получает свой working ref; normal error/parser/
  version-negotiation/empty exits освобождают не переданный ref и anchor после
  actual return. При abnormal unwind anchor удерживает backing, даже когда callee
  уже выполнил свой deferred Decrement. Unknown cleanup не повторяет pool return.
- Original popped/delivery, processing и drain views прикреплены к Conn перед
  callbacks. Shallow per-part Clone заменён value copy; coalesced counter теперь
  int, без uint8 wrap для большого custom datagram. Nil-qlogger parse-drop path
  больше не вызывает RecordEvent на nil. Byte/view limits не доказывают wire ABI.
- tryQueueingUndecryptablePacket возвращает реальный bool: capacity refusal больше
  не выглядит как queued и не пропускает caller decrement. Limit32 включает
  waiting/ready/remaining-replay views вместе. Key events переносят references с
  очисткой старых cells; handshake-complete делает actual release до nil slice.
  Replay batch хранится в original Conn; ошибка не теряет остаток очереди.
- Terminal drain seals intake, pop берёт original drain slot под queue mutex и
  освобождает его после unlock. Waiting/ready/replay cells возвращают1 ref каждая,
  очищаясь только после actual release. Unknown current processing/delivery не
  очищается optimistic cleanup и не даёт positive loop receipt.
- Intake call quiescence отдельно ожидается до connection Recorder.Close.
  Unknown calls сохраняют recorder; actual recorder Close error даёт negative
  loop result. Existing recreation trace-transfer exception не изменён.
- Новый WaitForPacketInputShutdown ожидает original ctx/loop и admitted intake
  result. Actual Transport root и HY2 original QUIC close подключают его до
  прежних stream joins. Negative completion сохраняет owner/credits; existing
  public signatures, frozen versions/licences/HY2 quotas не изменены. Для нового
  API нужен совместимый local source pair; stock mod4 rollback по одному модулю
  не source-compatible.

4 Go +2 ORIGIN +4 docs;10 snapshots(9existing1new), manual5 existing Windows
diffs +1 полный helper. Touched-only gofmt4 exit0;13 reviewed inputs,
0 additional cache reads;261 cache originals/226 verbatim mirrors и prior owned
bytes сохранены. No tests/fixtures/build/vet/typecheck/product parser/SDK/browser/
native/admin/network/generation. Limits — engineering ceilings, не CPU/RAM/Stop
или сравнительный выигрыш. Per-Transport aggregate upstream pool budget и global
retiring inventory по-прежнему отсутствуют как принятые full-PC контракты.

Далее: server receive/0RTT/response queue terminal custody/drain и response output
through actual provider return. Generic CID/reset removal/constructor/control/
crypto/CC/global/nativeOS/wholeUX/release/fullPC остаются OPEN. Receipts: .artifacts/memory/bootstrap-v418-conn-input-.
Source/unverified. No tests/fixtures/build/vet/typecheck/product parser/SDK/browser/native/admin/network/generation/dependencies/other roles/chats/subagents. Manual source/diff/byte inspection and touched-only gofmt do not prove compile, new WaitForPacketInputShutdown or changed internal helpers/mocks/tag/ABI compatibility, race freedom, fixed-slot reuse/queue/close/trace/counter/anchor/coalesced/decryption/replay/error/pool/Goexit interleavings, RFC/overload behavior or performance.64 fixed Conn intake cells/64*20KiB charged backing spans, existing queue256 and combined32 pending undecryptable views are local engineering bounds, not global upstream pre-pool admission, caller/goroutine/heap/GC/RSS/CPU/Stop limits or measured optima. Incoming backing already belongs to its upstream listener/server; rejected calls keep that original upstream owner through actual release. The fixed cells add per-Conn metadata cost, not a measured memory/speed improvement. Ordinary completion alone frees a fixed cell; unknown retains original slot/views/credits and gives a negative result. Negative done does not prove remaining callbacks stopped and never authorizes replacement. Pool/callback/error methods execute outside Conn input/receive queue metadata locks; callback self-join and noncooperative/hidden provider borrows are not forcibly contained. One original Conn loop owns processing/replay/drain; arbitrary private helper reentry/custom raw providers are not an accepted public contract. Existing recreation trace exception stays. Server receive/0RTT/response queue/output custody, generic CID/reset removal, constructor/control/crypto/CC/global/nativeOS/wholeUX/release/fullPC00..09/old9 remain OPEN.100 acceptance OPEN/56 checks NOT_RUN/72tasks active/gatesfalse/TUN unavailable/schema2 unsupported/rootNETGUARD001 unchanged;55% remains rough/unmeasured.

<a id="pc-v419-server-input"></a>

## V2-419 — original server input/response queues

Статус: PC-06 source/unverified, 2026-11-11; quic-go revision21/sing-quic revision29.
Version pair сверена по текущим ORIGIN; прежние module versions/licences сохранены.
V418 — progress; текущий source-срез — progress, весь PC-00…09/old9 сохранён.

- Server64 fixed original intake cells/64*20KiB backing-span credits до queue
  publication. Existing receive channel1024 shares seal с Close. Known full/late/
  refused input возвращает1 ref вне mutex; unknown call/cell/credits остаются.
  Transport берёт original server snapshot под mutex, снимает его и лишь затем
  вызывает handler. Callback и фактический pool-return больше не идут под ним.
- Receive task держит original current/held/handoff/candidate/drain до callbacks.
  Обычная Initial error, не передавшая input, теперь освобождает его. Ownership
  transfer к Conn/response/0RTT помечается явно. Cross-task extra anchor не добавлен:
  packetBuffer.refCount non-atomic, второй такой holder между задачами был бы опасен.
- 0RTT queues32*31 сохраняют прежние count limits. Retry/refusal/discard/expiry
  освобождают original cells до map delete. Handoff сначала переносит cell в
  retained original slot, затем вызывает consumer; normal return clears views.
  Candidate хранится до фактического принятия map + scheduled task либо positive
  original rejected-loop retirement. Unknown candidate не перезаписывается.
- Known receive stop seals publication и освобождает queued receive/0RTT refs.
  Actual done channel публикуется после этого finalizer. Abnormal original не
  делает blanket release неизвестного current/handoff или повтор native cleanup.
- Existing response worker держит original Retry/invalid/refused/version input
  и header, output buffer/data/OOB и точный raw provider через callbacks/Write/
  count validation. Удалены unconditional deferred releases входа/выхода из helpers.
  Known return освобождает1 input ref после output; invalid count/abnormal state
  сохраняет originals и останавливает reuse. Best-effort known write errors
  сохраняют прежнюю политику, не становятся direct fallback или retry prefix.
- Original pooled backing сохраняется отдельно от mutable buffer.Data. Ошибка
  Header.Append может вернуть nil; known cleanup возвращает в pool исходный span,
  не nil/grown view. Unknown phase не повторяет этот pool-return. Reply1452 check
  предшествует pooled get/append либо version composition; Retry учитывает16-byte
  tag, version composer — дополнительную greased version. Это не bound произвольной
  allocation внутри token/address/provider callback.
- Response worker после actual receive done освобождает очереди4/4/4/8 через
  retained drain slot. Positive server Close joins intake/receive/response. Failure
  signal даёт отрицательный Accept/Close outcome и сохраняет ещё живые producer
  roots; такой return не выдаётся за их остановку. Повторный Close наблюдает
  исходный результат. Root после failed server сохраняет rows/candidate и не
  зависает на ожидании не запущенного connection loop.

5 Go +2 ORIGIN +4 docs;11 snapshots(9existing2new), manual5 existing Windows
diffs +2 полных helper. Touched-only gofmt5 exit0;16 reviewed inputs,
0 additional cache reads;261 cache originals/226 verbatim mirrors и prior owned
bytes сохранены. No tests/fixtures/build/vet/typecheck/product parser/SDK/browser/
native/admin/network/generation. Новых workers/timers/per-packet registry
allocations нет; существующие public signatures и HY2 quotas сохранены.

Далее: original constructor/context/cancel/trace prefix ownership, generic CID/
reset-token producer/removal authority и global retiring inventory; crypto/CC/
nativeOS/wholeUX/release/fullPC по-прежнему OPEN. Server handshake/control и
произвольные provider contracts не считаются принятыми этим срезом. Receipts: .artifacts/memory/bootstrap-v419-server-input-.
Source/unverified. No tests/fixtures/build/vet/typecheck/product parser/SDK/browser/native/admin/network/generation/dependencies/other roles/chats/subagents. Manual source/diff/byte review and touched-only gofmt do not prove compilation, new server structs/private helpers/mocks/tags/ABI, race freedom, seal/delegation/queue/0RTT/transfer/candidate/provider/count/Append/pool/Goexit/error/failure/Close/Accept interleavings, wire/RFC/amplification/overload compatibility or speed.64 server intake cells/64*20KiB charged backing spans, existing queue1024/0RTT32*31/response4+4+4+8 and output1452 are local engineering ceilings; not global admission before upstream raw pool use, retained-carrier/task count, actual heap/backing/GC/RSS/CPU/Stop or measured optima. Fixed per-server metadata has a cost; no comparative memory/performance claim. Native input ref ownership is transferred without a second cross-task anchor because packetBuffer.refCount is non-atomic. Arbitrary callbacks, custom raw/addr/token/context/trace provider allocations or hidden borrows cannot be forced to stop. Negative failure signal/Close can precede actual producer termination; original server/cells/current/rows remain retained and never authorize replacement/pool/native retry/deadline clearing. Normal best-effort response-write errors retain their prior policy; invalid counts/abnormal cleanup are negative. The changed server intake/delegation/drop/drain/write callbacks and pool/native/error operations execute outside their metadata mutexes; constructor callbacks remain open scope; callback self-join remains unsupported. Full constructor/context/cancel/trace prefix lifetime, handshaking/control caller inventory, generic CID/reset removal, crypto/CC/global/nativeOS/wholeUX/release/fullPC00..09/old9 remain OPEN.100 acceptance OPEN/56 checks NOT_RUN/72tasks active/gatesfalse/TUN unavailable/schema2 unsupported/rootNETGUARD001 unchanged;55% remains rough/unmeasured.

<a id="pc-v420-cid-authority"></a>

## V2-420 — original connection authority при регистрации CID/reset

Статус: PC-06 source/unverified, 2026-11-11; quic-go revision22/sing-quic revision30.
V419 — progress; текущий срез — progress. Полный PC-00…09 и прежние девять этапов
сохранены. Запрет tests/build/native/live остаётся в силе.

- packetHandlerMap.Remove/RemoveResetToken принимают original Conn и сверяют
  текущую запись через существующий closedSourceMatches, включая wrappedConn.
  Чужой владелец не удаляется. Add/AddResetToken возвращают принятие; повтор той
  же original регистрации идемпотентен, чужой CID/reset не перезаписывается.
  Existing close/seal и local CID length проверяются до записи.
- Server AddWithConnID под одним существующим mutex проверяет оба ключа, alias
  и source length прежде любой записи. Initial client Dial отказывает при занятом
  source CID/неверной длине до Tracer/newClientConnection. Полный constructor
  prefix/context/cancel/trace lifecycle этим не закрыт.
- connRunnerCallbacks.AddConnectionID возвращает bool. Generator проверяет
  nonzero initial source length и uniqueness среди active/retiring/initial IDs;
  NEW_CONNECTION_ID ставится в очередь после принятия всеми runners. Original
  pending active row прикреплён до callbacks: при partial refusal его знает
  teardown. Sticky registrationFailure не позволяет следующему вызову затереть
  незавершённый ID или объявить успех. Highest published sequence не продвигается.
- Два ранее игнорировавшихся SetMaxActiveConnIDs errors теперь передаются через
  restore/apply transport parameters и все реальные handshake/0RTT callers.
  Ошибка не закрывает early-ready channel как успешное применение. Refused reset
  registration вызывает nonwaiting destroyImpl исходного Conn; apply/path enable
  замечают original close error. Нового close-waiter/task нет.
- AddConnRunner возвращает error и прикрепляет original runner до callbacks.
  Known failure снимает только original-owned active/initial entries через
  callbacks с захваченным Conn, затем удаляет runner из map. Foreign collision
  остаётся на месте. Unknown callback/cleanup не отсоединяет prefix и не выдаёт
  положительный outcome; закрытие соединения сохраняет original custody.
- Дополнительный путь получает error-returning enable callback. Probe возвращает
  исходную ошибку; failed paths не получают новый PATH_CHALLENGE и повторную
  регистрацию. Reserved peer CID retired, challenge views очищаются, failed path
  больше не проходит switch gate и снимается из pending switch. Исходный Probe
  timer останавливается на любом возврате. Известный конфликт source CID нового
  runner сам по себе не закрывает действующее здоровое соединение.

4 Go +2 ORIGIN +4 docs;10 existing snapshots/0 new product files.
Manual6 existing Windows diffs; touched-only gofmt4 exit0.12 reviewed inputs,
0 additional cache reads;261 frozen cache originals/224 verbatim mirrors/prior
owned bytes сохранены. Existing public signatures, module versions/licences,
HY2/QUIC fixed input/output ceilings сохранены. Никаких tests/fixtures/build/
vet/typecheck/product parser/SDK/browser/native/admin/network/generation.

Следующий срез: original constructor/context/cancel/trace prefixes и handshaking
caller outcomes. Same-Conn registration epochs/refcounts, полный reset-token
routing при migration и path/control/retiring/global inventory требуют отдельных
изменений; crypto/CC/OS/wholeUX/release остаются OPEN. Receipts: .artifacts/memory/bootstrap-v420-cid-authority-.
Source/unverified. No tests/fixtures/build/vet/typecheck/product parser/SDK/browser/native/admin/network/generation/dependencies/other roles/chats/subagents. Manual source/diff/byte inspection and touched-only gofmt do not prove compilation/private callback ABI/mocks/tags, race freedom, CID reuse/collision/rollback/retire/error/Close/path/handshake/0RTT/provider interleavings, wire/RFC compatibility, global/RSS/CPU/Stop limits or performance. Expected original Conn identity distinguishes connections; it is not a same-Conn registration epoch/refcount. Shared reset tokens and repeated/delayed same-owner removal need their own protocol/caller contract. Reset-token manager still registers on the original runner: complete migrated-transport reset routing is not implemented here. Known additional-runner failure withdraws expected-owned active/initial IDs only after callbacks return; unknown callback/cleanup keeps the original runner/pending CID prefix for negative teardown, without forced release or a replacement retry. No arbitrary retiring-ID/runner/global quota was introduced. Existing path callbacks still run under the path mutex; constructor callbacks remain under their existing transport lock. Whole constructor/context/cancel/trace prefixes, path/control/retiring/global/crypto/CC/nativeOS/wholeUX/release/fullPC00..09/old9 remain OPEN. Public API signatures/versions/licences and earlier fixed input/output quotas stay; private callbacks changed and remain uncompiled.100 acceptance OPEN/56 checks NOT_RUN/72tasks active/gatesfalse/TUN unavailable/schema2 unsupported/rootNETGUARD001 unchanged;55% remains rough/unmeasured, not newly calculated.

<a id="pc-v421-connection-opening"></a>

## V2-421 — original server opening и действительный outcome handshake

Статус: PC-06 source/unverified, 2026-11-11; quic-go revision23/sing-quic revision31.
V420 — progress; текущий срез — progress. Весь PC-00…09/old9 остаётся active.

- В serverPacketInputState добавлен один fixed serverConnectionOpening. Original
  receive task прикрепляет sendConn/context/cancel до ConnContext; затем сохраняет
  returned context/cancel/trace. Private newConnection принимает этот prefix и
  прикрепляет partial Conn до первого AddProducer/provider callback. Нового map,
  worker или timer class нет. Prefix очищается только после known promotion к
  original map/scheduled loop либо actual rejected-loop retirement. Unknown
  construction/cleanup сохраняет prefix и даёт negative server outcome.
- GenerateConnectionID и source length/alias validation перенесены до создания
  contexts/trace. Ordinary generator error больше не оставляет созданные ранее
  контексты/trace. Это не общий client constructor admission или глобальная квота.
- Ранние StartHandshake / initial-event errors теперь публикуют nonwaiting close
  request и проходят существующий общий terminal path: crypto/send queue, streams,
  CID/reset/trace и input cleanup. Уже запрошенный close/canceled callback-context
  не запускает новый handshake. Original loop наблюдает context cancellation и
  перед ожиданием, и в select. Timer Stop выполняется deferred на каждом выходе.
- Crypto Close error сохранён в terminal result. В прочитанном локальном Go
  source QUICConn.Close ждёт actual handshake goroutine и возвращает handshakeErr:
  известная ошибка не объявлена автоматически неизвестным retirement. Эта ссылка
  не доказывает фактический compiled toolchain/graph.
- Terminal normalization работает с private copy atomic close record, включая
  обычный nil→ApplicationError outcome. Публичный original close record больше не
  переписывается terminal helper, пока Dial/path readers наблюдают его.
- Первоначальная идея закрывать readiness channel только на успехе уточнена по
  существующему публичному контракту и реальным consumers. HandshakeComplete
  сохраняет completion/failure notification; sync.Once будит и ранние terminal
  failures без double-close. Отдельный atomic handshakeSucceeded выставляется
  только после полного успешного handleHandshakeComplete. Это отдельный grant.
- Dial до init/recreate замечает уже отменённый запрос; перед ready return
  повторно проверяет observed cancellation/closing и actual handshake success.
  При отказе ждёт original loop error/recreation, а не возвращает Conn,nil.
  Server enqueue проверяет тот же success для обычного Accept; Accept пропускает
  closing queued peers. Early-connection semantics сохранены отдельно.
- NextConnection после 0-RTT rejection больше не возвращает failed/closed Conn
  как success и не вызывает UseResetMaps при неуспешном handshake. При failure
  notification ждёт original terminal cause или отмену вызывающего контекста.

4 Go +2 ORIGIN +4 docs;10 existing snapshots/0 new product files. Manual6
existing Windows diffs, touched-only gofmt4+4 exit0.11 reviewed project inputs +1
локальный TLS source reference;0 additional cache reads;261 cache originals/
224 verbatim mirrors/prior owned bytes сохранены. Tests/fixtures/build/vet/types/
product parser/SDK/browser/native/admin/network/generation не выполнялись.

Далее: client constructor admission/Transport lock/callback prefixes, original
retirement и полные recreation/provider contracts. SameConn epochs/reset migration/
path/control/retiring/global/cryptoCC/nativeOS/wholeUX/release остаются OPEN.
Receipts: .artifacts/memory/bootstrap-v421-connection-opening-.
Source/unverified. No tests/fixtures/build/vet/typecheck/product parser/SDK/browser/native/admin/network/generation/dependencies/other roles/chats/subagents. Manual source/diff/byte inspection and touched-only gofmt do not prove compilation/private constructor ABI/mocks/tags, race freedom, callback/cancel/Close/readiness/promotion/rejected-loop/recreation/0RTT/queue/provider interleavings, wire/RFC compatibility, global/RSS/CPU/Stop limits or speed. One fixed server prefix retains only its serialized opening; it is not a process/global task or byte budget. Unknown callback/cleanup completion retains context/cancel/trace/partial Conn and fails the original server; it does not force providers to stop or prove native retirement. Cancellation checks observe known state at those points; a returned healthy connection may close later. Public HandshakeComplete still notifies completion/failure and therefore alone is not a success grant. Separate internal atomic success gates are added for actual Dial/server/NextConnection consumers; full HTTP3/custom-consumer/recreation semantics remain separate scope. Local installed TLS QUICConn.Close source was read and hashed as a reference, not as proof of a compiled/selected toolchain or dependency graph. Known TLS Close errors can describe the already joined handshake failure; they are preserved without equating every ordinary error to uncertain task retirement. Client constructor admission/Transport lock/callback custody and reentrant Close, opaque hidden allocation/borrows, full context/trace/provider cleanup, sameConn epochs/reset migration/path/control/retiring/global/crypto/CC/nativeOS/wholeUX/release/fullPC00..09/old9 remain OPEN. Public signatures/versions/licences and previous quotas stay; private constructor signature changed and is uncompiled.100 acceptance OPEN/56 checks NOT_RUN/72tasks active/gatesfalse/TUN unavailable/schema2 unsupported/rootNETGUARD001 unchanged;55% remains rough/unmeasured, not recalculated.

<a id="pc-v422-client-opening"></a>

## V2-422 — original client opening, reserved CID и promoted client lifetime

Статус: PC-06 source/unverified, 2026-11-11; quic-go revision24/sing-quic revision32.
V421 — progress; текущий срез — progress. Полный PC-00…09 и прежние этапы сохранены.

- transport_client_opening.go использует64 fixed cells в original Transport.
  Reserve происходит до init wait, config/TLS clones и context/address/provider/
  trace/constructor callbacks. Original ctx/addr/config/TLS/raw/send/trace/partial
  Conn остаются attached. Nil TLS получает ordinary admission error. Preparation
  передаёт один original следующему constructor stage; его finalizer заканчивается
  до ожидания handshake. doDial копирует нужные inputs до возврата fixed cell,
  поэтому поздний waiter не читает переиспользованную запись.
- Source CID резервируется под Transport.mutex → lifecycle.access. Placeholder
  handler в demux не создаётся. Generic Add/AddWithConnID отвергают reserved keys;
  они и AddResetToken теперь используют тот же atomic seal fence при записи.
  Publish original Conn atomically переносит CID в map и снимает reservation.
  Slot возвращается после scheduling существующего original client loop.
- Client Tracer/logger/address/token/provider/constructor callbacks выполняются
  вне metadata locks. Private newClientConnection получает original opening и
  прикрепляет partial Conn до первого callback. Это не разрешает callback self-
  Close: ожидание собственного original Close остаётся unsupported self-join.
- Полностью созданный, но отменённый/refused candidate запускает исходный
  terminal Conn.run в существующем constructor call, затем joins existing
  packet/receive-data/send-data retirement. CID/cell возвращаются только после
  known positive outcome. Unknown constructor/cleanup сохраняет original cell/
  CID/partial Conn, seals failure и использует original native abort.
- Promoted clients записаны в intrusive list initial Transport независимо от
  CID rows: RemoveAll может удалить CID до retirement остальных borrowers.
  Embedded node не требует allocation нового failure holder/map. Root snapshot
  дедуплицирует этот original с CID/reset aliases. Active list не получает новый
  произвольный limit established connections; это отдельный global scope.
- Existing original client worker после Conn.run joins прежний retirement helper
  до unlink либо version recreation. Send-data gate уже joins registered stream
  calls; дополнительный одинаковый Wait не добавлен. Nil/negative/abnormal
  outcome сохраняет/relinks original и публикует negative error до original abort.
  Успешный SingleUse transport Close не оборачивает прежнюю typed error; ненулевой
  close error сохраняется. Нового task/timer class нет.
- Root Close после original native abort/init joins original openings до CID/
  client snapshot, recorder close и borrowed read-deadline restore. Negative
  opening outcome не выдаётся за завершение producers и не разрешает replacement.
  Constructor credits64 — count текущих construction calls, не global bytes/
  live-connection/RSS/CPU/Stop bound. Full provider/worker retirement не доказан.

5 Go +2 ORIGIN +4 docs;11 snapshots(10existing1new). Manual6 existing Windows
diffs +1 полный новый helper; touched-only gofmt5 exit0.12 reviewed inputs,
0 additional cache reads;261 cache originals/224 verbatim mirrors/prior owned
bytes сохранены. No tests/fixtures/build/vet/types/product parser/SDK/browser/
native/admin/network/generation. Private client constructor/doDial ABI uncompiled;
public APIs/module versions/licences/HY2 and earlier QUIC quotas сохранены.

Далее: actual version recreation/trace producer custody, listener/init provider
callbacks и full original retirement. SameConn epochs/reset migration/path/
control/global/cryptoCC/nativeOS/wholeUX/release/fullPC остаются OPEN.
Receipts: .artifacts/memory/bootstrap-v422-client-opening-.
Source/unverified. No tests/fixtures/build/vet/typecheck/product parser/SDK/browser/native/admin/network/generation/dependencies/other roles/chats/subagents. Manual source/diff/byte inspection and touched-only gofmt do not prove compilation/private constructor/doDial ABI/mocks/tags, race freedom, fixed-cell handoff/CID seal/promotion/Close/rejected-loop/worker/retirement/recreation/provider interleavings, wire compatibility, global/RSS/CPU/Stop limits or speed.64 fixed opening cells cap concurrent construction only; the intrusive promoted-client list mirrors existing live originals and adds no arbitrary established-connection limit or global retiring bound. TLS/config/opaque-provider allocation and hidden borrows remain outside those count credits. Failed/abnormal original keeps its cell/CID or promoted client node and yields a negative outcome; this neither forces callbacks to stop nor authorizes a replacement. Root may return negative before actual producers terminate, retaining original pointers. Existing packet/receive-data/send-data retirement gates are composed; send-data already joins registered stream calls, and no duplicate new call gate was added. These gates do not prove whole crypto/CC/raw/custom/native/OS shutdown or actual wrapper-task completion. Reentrant Close from an original callback remains unsupported self-join even though client metadata mutexes are released. Listener/createServer/init/raw/native callbacks and custom context/trace/error providers, full version recreation/recorder lifetime, sameConn epochs/reset migration/path/control/global/cryptoCC/nativeOS/wholeUX/release/fullPC00..09/old9 remain OPEN. Earlier public APIs/versions/licences/HY2/QUIC quotas stay; private client constructor/doDial signatures changed and remain uncompiled.100 acceptance OPEN/56 checks NOT_RUN/72tasks active/gatesfalse/TUN unavailable/schema2 unsupported/rootNETGUARD001 unchanged;55% remains rough/unmeasured, not recalculated.

<a id="pc-v423-recreation-trace"></a>

## V2-423 — per-attempt qlog producer и version recreation

Статус: PC-06 source/unverified, 2026-11-11; quic-go revision25/sing-quic revision33.
V422 — progress; текущий срез — progress. Весь PC-00…09 и прежний объём сохранены.

- Сверены actual client opening, Conn.run и default trace factory: каждая попытка
  вызывает фабрику trace и AddProducer. Прежний skip qlogger.Close при recreate
  не передавал old producer следующему владельцу. Теперь original recorder
  закрывается после input joins и для recreate. Factory behavior сохранён;
  новый synthetic/shared producer не вводится. Ненулевой recorder close error
  marks Conn negative, поэтому V422 retirement gate не разрешает unlink/recreate.
- Standard FileSeq Writer имеет один original Close через Once и cached result.
  Неизвестный исходный Close сохраняет negative error для subsequent callers;
  повторный Close не уменьшает count дважды и не повторяет native cleanup.
- RecordEvent admission и closing делят Writer.access; Close закрывает admission
  и joins уже принятые sends перед producer decrement. Wait/provider Close идут
  вне Writer.access/FileSeq.mx. Multi-producer trace сохраняет остальных owners:
  закрытие одного recorder не закрывает их общий sink преждевременно.
- FileSeq.Run имеет один original через Once. Один current event прикреплён до
  Event.Name/Encode/provider callbacks; view очищается после actual return,
  включая known encoding error. runReturned публикуется только после known
  drain/log completion. Это не подтверждение immutable backing любого Event.
- Last producer joins original Run до raw Close, возвращает encoding и native
  close errors. Unknown Run сохраняет original event/encoder/sink и даёт negative
  outcome без speculative native Close/retry. Nonlast producer также сообщает
  observed abnormal Run; обычный nonlast success относится только к его producer.
- При manual review выявлен secondary hang: blocked send в полной queue50 мог
  не закончиться после abnormal Run, мешая Writer.Close достичь negative outcome.
  RecordEvent теперь замечает runStopped до enqueue и при blocked send. Новое
  rejected событие не попадает в callbacks; original current/queued inputs остаются
  trace-owned. Никакой новый poller/worker/timer для этого не создан.

2 Go +2 ORIGIN +4 docs;8 existing snapshots/0 new product source. Manual4
existing Windows diffs; touched-only gofmt2+2 exit0.9 reviewed inputs,
0 additional cache reads;261 cache originals/223 verbatim mirrors/prior owned
bytes сохранены. No tests/fixtures/build/vet/types/product parser/SDK/browser/
native/admin/network/generation. Public APIs/versions/licences/queue50/HY2 quotas
сохранены. Optional tracing получил дополнительную синхронизацию; скорость и
оптимальность overhead не измерены.

Далее: actual VN/recreation source/destination/peer binding, listener/init provider
callbacks и full original retirement. Event bytes/sink/header/count/opaque provider
contracts, selfClose, sameConn/reset migration/path/global/cryptoCC/OS/UX/release
остаются OPEN. Receipts: .artifacts/memory/bootstrap-v423-recreation-trace-.
Source/unverified. No tests/fixtures/build/vet/typecheck/product parser/SDK/browser/native/admin/network/generation/dependencies/other roles/chats/subagents. Manual source/diff/byte inspection and touched-only gofmt do not prove compilation/private qlog helper ABI/mocks/tags, race freedom, producer/Run/Once/event-send/close/wakeup/Goexit/error/recreate interleavings, trace/wire compatibility, global/RSS/CPU/Stop limits or speed. Existing queue50 bounds queued event count only; Event payloads, simultaneous producers/callers, encoder/provider allocation and hidden borrows remain outside a byte/global/retiring quota. RecordEvent admission and Close join apply to the standard FileSeq Writer, not arbitrary custom Recorders. Rejected stopped-task events were not passed to event callbacks; the uncertain original current/queued events and sink remain trace-owned. Retaining an event interface does not prove immutable payload/backing ownership. sync.Once preserves one original Run/Close and cached abnormal outcome; it does not recover a panic, force a blocked callback to stop or make reentrant own Close/Run/Event.Encode safe. Run must actually be started by the trace owner; missing/blocked Run or a noncooperative sink can still prevent Close completion. Last-producer Close returns encoding/native Close errors; nonlast success retires only that producer, not the shared trace. Per-attempt trace factories keep their existing behavior, with no synthetic inherited producer; shared trace lifetime requires its existing other producer ownership. Full constructor/factory/header/sink/write-count/provider/event contracts, VN source/destination/peer validation, listener/init callbacks, sameConn epochs/reset migration/path/control/global/cryptoCC/nativeOS/wholeUX/release/fullPC00..09/old9 remain OPEN. Optional tracing adds synchronization; no measured speed, overhead or optimal-resource claim. Public APIs/versions/licences/HY2/QUIC quotas stay; all source remains uncompiled.100 acceptance OPEN/56 checks NOT_RUN/72tasks active/gatesfalse/TUN unavailable/schema2 unsupported/rootNETGUARD001 unchanged;55% rough/unmeasured, not recalculated.

<a id="pc-v424-version-binding"></a>

## V2-424 — привязка VN к исходной попытке

Статус: PC-06 source/unverified, 2026-11-11; quic-go revision26/sing-quic revision34.
V423 — progress; текущий срез — progress. Весь PC-00…09 и прежний объём сохранены.

- Прослежены actual VN decoder, Transport demux, client constructor, Retry state,
  first-send/first-processed flags и server composer. Decoder возвращает dest,src;
  клиент называл их src,dest, поэтому received qlog поля были перепутаны. Названия
  и actual echo comparison теперь соответствуют wire order.
- Client хранит immutable initial source CID по value. До version-list allocation
  existing prefix parser даёт owned CID copies; VN destination сверяется с source
  CID этой попытки, VN source — с её Initial destination. При mismatch packet
  игнорируется с unknown_connection_id, без выбора версии/recreation.
- Phase gate отбрасывает VN до first-send и после processed Retry/first packet/
  prior negotiation/closing. Existing current-version rejection, malformed list,
  preferred overlap и no-overlap outcome сохранены. Общий drop logger не делает
  packet ref/pool retirement; это остаётся у original input owner.
- Нового decoder API, raw payload view, source-IP/port gate, version-list clipping,
  worker/timer/registry/global quota нет. Prefix parser и full decoder сохранены;
  additional prefix check не заявлен как zero-allocation или measured speed gain.
- Сверены primary [RFC9000 §6.2](https://www.rfc-editor.org/rfc/rfc9000.html#section-6.2)
  и [§17.2.1](https://www.rfc-editor.org/rfc/rfc9000.html#section-17.2.1):
  processed-packet phase и отражённые request CIDs. Это read-only research,
  не тест VPN, endpoint или изменение системной сети. [§12.1/21.12](https://www.rfc-editor.org/rfc/rfc9000.html#section-21.12)
  отделяют CID request binding от cryptographic peer/downgrade protection.

1 Go +2 ORIGIN +4 docs;7 existing snapshots/0 new product source. Manual3
existing Windows diffs; touched-only gofmt1 exit0.14 reviewed project inputs,
0 additional cache reads;261 cache originals/223 verbatim mirrors/prior owned
bytes сохранены. Read-only standard notes: .artifacts/memory/bootstrap-v424-version-binding-spec-notes.json.
No tests/fixtures/build/vet/types/product parser/SDK/browser/native/admin/VPS/
live/system network mutations/generation. Public APIs/versions/licences/HY2/
QUIC quotas сохранены. Private source remains uncompiled.

Далее: actual Retry request/token/phase validation, затем listener/init provider
callbacks и full original retirement. Полный VN/Retry/peer/downgrade/route/hopping
runtime contract не принят; selfClose/sameConn/reset/path/global/cryptoCC/nativeOS/
wholeUX/release/fullPC остаются OPEN. Receipts: .artifacts/memory/bootstrap-v424-version-binding-.
Source/unverified. No tests/fixtures/build/vet/typecheck/product parser/SDK/browser/native/admin/VPS/live/system network changes/generation/dependencies/other roles/chats/subagents. Read-only RFC9000 primary-source browsing was documentation research, not network/VPN testing or endpoint verification. Manual source/diff/byte inspection and touched-only gofmt do not prove compilation/private helpers/mocks/tags, race freedom, VN/Retry/CID/phase/close/recreation/log/parser/provider interleavings, RFC/version-2/hopping/migration interoperability, global/RSS/CPU/Stop limits or speed. Request CID echo binding is not cryptographic peer/TLS/path/route authentication or downgrade protection; RFC9000 sections12.1/21.12 explicitly leave that gap. No blanket source-IP/port restriction or new versions/count/global quota was added. Existing prefix parser still allocates bounded owned CID copies; the full decoder is reused after identity checks, so this is not zero-allocation parsing. New immutable source-CID metadata has a fixed per-Conn cost, not measured overhead or a proof that all protocol state is bounded. Original packet-input ownership and earlier close/recorder/worker gates remain separate. Full Retry/token/peer/version-negotiation, secure future-version negotiation, listener/init/provider/event/sink/selfClose/sameConn/reset migration/path/control/global/cryptoCC/nativeOS/wholeUX/release/fullPC00..09/old9 remain OPEN. Public APIs/versions/licences/HY2/QUIC quotas stay; all source remains uncompiled.100 acceptance OPEN/56 checks NOT_RUN/72tasks active/gatesfalse/TUN unavailable/schema2 unsupported/rootNETGUARD001 unchanged;55% rough/unmeasured, not recalculated.

<a id="pc-v425-retry-binding"></a>

## V2-425 — Retry request/phase binding и честные audit findings

Статус: PC-06 source/unverified, 2026-11-11; quic-go revision27/sing-quic revision35.
V424 — progress; текущий срез — progress. Весь PC-00…09 и прежний объём сохранены.

- Прослежены wire.Header Retry parser, actual handleLongHeaderPacket caller,
  existing integrity helper, packet packer и ResetForRetry. Два предположения
  отклонены по source: parser уже rejects tokenLen<=0 после вычета16-byte tag;
  ResetForRetry создаёт Initial/application spaces с прежними next PN из Peek.
  Тесты не запускались; эти source findings не являются runtime PASS.
- Retry handler теперь до manager/crypto work отвергает server/pre-send/
  first-processed/duplicate/closing phase, несовпадающую version и destination,
  отличный от immutable initial client source CID. Server source должен отличаться
  от original Initial destination. Нового IP/port gate нет.
- Callee отдельно проверяет parsed token/header/raw length перед tag slicing:
  это defensive proof, не исправление уже существующего parser empty-token guard.
  Existing GetRetryIntegrityTag использует immutable origDestConnID, без Get у
  CID manager. Не заявлено, что прежний Get действительно менял CID в легитимной
  Retry phase. Closing наблюдается ещё раз после tag validation до state transition.
- Accepted Retry сохраняет прежний ResetForRetry/key/token/CID transition,
  owned token и packet-input anchor/ref retirement. Packet numbers и crypto
  algorithm не переписаны. Received qlog destination теперь из actual Retry
  header, а не old outgoing destination. Common drop logger не освобождает packet.
  Invalid-tag diagnostic не приписывает всякую ошибку подтверждённой атаке.
- [Ранее прочитанный RFC9000 Retry handling](https://www.rfc-editor.org/rfc/rfc9000.html#section-17.2.5.2)
  остаётся reference. Текущая попытка загрузить RFC9000/RFC9001 завершилась network
  error; RFC9001 content не получен/не используется. Это ошибка документационного
  запроса, не доказательство отказа VPN/VPS. Integrity не является TLS/server/
  route authentication. Полная Retry/crypto/runtime interoperability не принята.

1 Go +2 ORIGIN +4 docs;7 existing snapshots/0 new product source. Manual3
existing Windows diffs; touched-only gofmt1 exit0.12 reviewed project inputs,
0 additional cache reads;261 cache originals/223 verbatim mirrors/prior owned
bytes сохранены. Source/spec notes: .artifacts/memory/bootstrap-v425-retry-binding-spec-notes.json.
No tests/fixtures/build/vet/types/product parser/SDK/browser/native/admin/VPS/
live/system network mutations/generation. Public APIs/versions/licences/earlier
HY2/QUIC quotas сохранены; all source remains uncompiled.

Далее: listener/createServer/init provider callbacks и original opening/retirement.
FullRetry/peer/TLS/downgrade/route/hopping, selfClose/sameConn/reset migration/path/
global/cryptoCC/nativeOS/wholeUX/release/fullPC остаются OPEN. Receipts: .artifacts/memory/bootstrap-v425-retry-binding-.
Source/unverified. No tests/fixtures/build/vet/typecheck/product parser/SDK/browser/native/admin/VPS/live/system network changes/generation/dependencies/other roles/chats/subagents. Current primary-source fetch failed with network error; no fetched RFC9001 text or VPN/VPS reachability result is claimed. Previously read RFC9000 Retry text and current source were used. Manual source/diff/byte inspection and touched-only gofmt do not prove compilation/private helpers/mocks/tags, race freedom, Retry/tag/phase/CID/token/close/state-transition/retransmission/provider interleavings, cryptographic conformance, v1/v2/route/hopping/native interoperability, global/RSS/CPU/Stop limits or speed. Production parser already rejects empty Retry tokens and ResetForRetry already keeps next Initial/application packet numbers with Peek; neither is an identified production defect or modified algorithm. Explicit callee length/token checks are defensive. Original destination replaces a mutable manager view without claiming that the former Get actually rotated a CID in the legitimate Retry phase. Closing checks observe their points; they are not a new atomic callback/cancellation transaction. Retry integrity/request binding is not TLS/server/path authentication; no peer-IP/port pinning, new algorithm, quota, worker or timer class was introduced. Existing token copies, frame/reset/retransmission/number/key/packer transitions and packet-input anchors remain separate contracts. Full Retry/crypto/peer/downgrade/listener/init/provider/event/sink/selfClose/sameConn/reset migration/path/control/global/cryptoCC/nativeOS/wholeUX/release/fullPC00..09/old9 remain OPEN. Public APIs/versions/licences/earlier quotas stay; source remains uncompiled.100 acceptance OPEN/56 checks NOT_RUN/72tasks active/gatesfalse/TUN unavailable/schema2 unsupported/rootNETGUARD001 unchanged;55% rough/unmeasured, not recalculated.

<a id="pc-v426-server-opening"></a>

## V2-426 — listener construction и единый publication/seal fence

Статус: PC-06 source/unverified, 2026-11-11; quic-go revision28/sing-quic revision36.
Полная цель PC-00…09 и прежние обязательства сохранены; текущий срез — progress.

- В исходном createServer Transport.mutex охватывал init, previous.closeRetired
  и newServer с WithPrefix/LocalAddr/logger callbacks. Constructor запускал оба
  worker до публикации server в Transport/lifecycle. Отдельной opening root не было.
- Новый transport_server_opening.go — одна fixed cell, а не per-call goroutine или
  вторая серверная policy. Reservation до init/config/provider work сохраняет
  original TLS/config/raw/address/previous/partial baseServer. Concurrent Listen
  получает errListenerAlreadySet; replacement всё ещё требует closeRetired
  прежнего original server. Init/close observation/providers вне metadata locks.
- newServer и sole production caller изменены совместно: partial baseServer
  прикрепляется до WithPrefix; constructor не спавнит. Existing packetaddr.Capture
  даёт frozen diagnostic address; known invalid address returns error before
  worker/packet borrowers. Public Addr/path/native contract этим не переписан.
- Publication original server и оба прежних worker starts выполнены под одним
  mutex→access fence. Close либо seals до публикации, либо видит original server
  и scheduled workers. Known refused unstarted candidate не закрывает shared
  tracer/raw и не ждёт running/sendQueueDone несуществующих tasks.
- Root после original init/client-opening join ждёт actual server opening до
  map/server snapshot, trace Close и borrowed-deadline restoration. Abnormal
  constructor/provider/Goexit retains original prefix, seals and invokes original
  abort; completion notification negative и не доказывает producer retirement.
  Trace gate отдельно запрещает закрытие при active opening.

5 Go +2 ORIGIN +4 docs;10 existing snapshots/1 new product source. Manual6
existing Windows diffs +1 new source; touched-only gofmt5 exit0.13 reviewed
project inputs/0 additional cache reads;261 cache originals/223 verbatim mirrors
и prior owned bytes сохранены. No tests/fixtures/build/vet/types/product/SDK/
browser/native/admin/VPS/live/system network mutations/generation.

Далее: original init/wrap/provider prefix и known convenience-factory failure
cleanup, сохраняя one original socket/trace result. Hidden ListenAddr/Listen
transports и self-Close не объявлены исправленными этим срезом. Receipts: .artifacts/memory/bootstrap-v426-server-opening-.
Source/unverified. No tests/fixtures/build/vet/typecheck/product/SDK/browser/native/admin/VPS/live/system network changes/generation/dependencies/other roles/chats/subagents. Manual source/diff/byte inspection and touched-only gofmt do not prove compilation/private helpers/mocks/tags, race freedom, callbacks/Close/constructor/initialization/worker scheduling interleavings, global/RSS/CPU/Stop bounds, TLS/QUIC interoperability or speed. One fixed opening bounds concurrent listener construction per Transport, not established connections, caller-held config/TLS backing, opaque provider allocations, process-wide resources or elapsed time. Unknown provider outcomes retain their original prefix and are negative, not proof external tasks stopped. Synchronous callback self-Close remains unsupported (self-join). Known unstarted candidates have no scheduled server workers or packet borrowers; only that branch is released without server loop shutdown. Existing packetaddr capture freezes diagnostic metadata, not listener Addr/public path, peer identity, route or native lifetime. Original init/wrap/raw bypass/provider/factory error cleanup, hidden convenience transports, full Retry/VN/crypto/event/sink/sameConn/reset/path/global/nativeOS/wholeUX/installer/release/fullPC00..09/old9 remain OPEN. Private newServer signature and sole production caller changed together; omitted upstream tests/private mocks were not modified or run. Public APIs/versions/licenses/earlier quotas remain.100 acceptance OPEN/56 checks NOT_RUN/72tasks active/gatesfalse/TUN unavailable/schema2 unsupported/rootNETGUARD001 unchanged;55% rough/unmeasured, not recalculated.

<a id="pc-v427-cid-init"></a>

## V2-427 — некорректный CID не допускается к socket initialization

Статус: PC-06 source/unverified, 2026-11-11; quic-go revision29/sing-quic revision37.
V426 — progress; текущий source-срез — progress. Весь PC-00…09 и прежний объём сохранены.

- Source audit: initializeTransport получал ConnectionIDLen после raw wrapping,
  настройки native buffers и allocation очередей. Range gate отсутствовал.
  GenerateConnectionID задавал uint8 length и срезал fixed20-byte buffer без
  проверки входного int. Это подтверждённые source paths, не воспроизведение.
- Exact configured generator сохраняется до одного ConnectionIDLen callback.
  Existing protocol.MaxConnIDLen/ErrInvalidConnectionIDLen переиспользованы до
  raw wrapping/socket tuning/queues/workers. Existing zero/default/custom rules
  и custom-generator precedence сохранены. Seal наблюдается после callback
  до wrapper work; отдельный final loop-scheduling seal gate остаётся.
- GenerateConnectionID отдельно returns zero CID +existing error для negative/
  oversized int до uint8 conversion, slice и random reads. Valid length0…20
  использует прежний crypto/rand algorithm. Новый формат/протокол/API не введён.
- Вручную прослежён early-error cleanup source: unscheduled init completion,
  nil-channel guards в output/cache retirement и input drain. Это не execution
  proof и не исправление hidden convenience-factory ownership этим срезом.

2 Go +2 ORIGIN +4 docs;8 existing snapshots/0 new product source. Manual4
existing Windows diffs; touched-only gofmt2 exit0.14 captured reviewed project
inputs/0 additional cache reads;261 cache originals/222 verbatim mirrors/prior
owned bytes сохранены. Source remains uncompiled. No tests/fixtures/build/vet/
types/product/SDK/browser/native/admin/VPS/live/system network mutations/generation.

Далее: original init/wrap/provider prefix и known convenience-factory error
cleanup; полная native/callback/borrowed-tuning/Stop приёмка OPEN. Receipts: .artifacts/memory/bootstrap-v427-cid-init-.
Source/unverified. No tests/fixtures/build/vet/typecheck/product/SDK/browser/native/admin/VPS/live/system network changes/generation/dependencies/other roles/chats/subagents. Manual source/diff/byte inspection and touched-only gofmt do not prove compilation/private helpers/mocks/tags, race freedom, invalid/zero/default/custom CID/runtime/parser/provider/random-source/Close interleavings, crypto conformance, interoperability, native resources or speed. Zero length and custom precedence are preserved, not universally suitable for every endpoint/migration. Existing constant bounds apply to CID length, not caller config backing, provider-internal allocations, global/RSS/CPU/Stop or wall time. Seal is observed at one point, not a new atomic native cancellation transaction. Custom generator lifetime/stability remains its contract; actual generated-ID length/collision admission remains a separate earlier gate. Full init/wrap/raw-provider/borrowed tuning/convenience-factory error cleanup/selfClose/Retry/VN/crypto/event/sameConn/reset/path/global/nativeOS/wholeUX/installer/release/fullPC00..09/old9 remain OPEN. Public APIs/versions/licenses/earlier quotas stay.100 acceptance OPEN/56 checks NOT_RUN/72tasks active/gatesfalse/TUN unavailable/schema2 unsupported/rootNETGUARD001 unchanged;55% rough/unmeasured, not recalculated.

<a id="pc-v428-setup-cleanup"></a>

## V2-428 — known factory failures закрывают исходный скрытый Transport

Статус: PC-06 source/unverified, 2026-11-11; quic-go revision30/sing-quic revision38.
V427 — progress; текущий срез — progress. Полная цель PC-00…09 сохранена.

- Source findings: DialAddr/Early открывали UDP socket до DNS/TLS admission;
  эти ранние errors не закрывали socket. ListenAddr/Early открывали socket до
  nil-TLS rejection и возвращали createServer error без hidden Transport cleanup.
  Borrowed Listen/Early также теряли hidden transport при known setup error.
  Dial/Early вызывали Close, но wrapper игнорировал его результат.
- Shared dialAddr сохраняет full/early mode, original IPv4 binding и host input.
  TLS/resolve происходят до socket acquisition; original Transport уже создан
  до native bind, returned nonnil socket прикреплён до subsequent setup. Shared
  listenAddr делает TLS admission до existing DNS/bind и сохраняет early/full.
- Known Dial/Listen setup errors используют closeAfterSetupError: один exact
  cached Transport.Close, а не новый cleanup/native retry. Existing lifecycle
  отличает owned socket close от borrowed interrupt/restore. Cause остаётся
  прежним object, если cleanup успешен. При cleanup error private typed wrapper
  сохраняет exact Transport/cause/result, fixed Error text и оба Unwrap inputs.
- Source consumers sing-quic/quic.go и WrapError прослежены: error chain сохраняется
  через existing Unwrap. Не объявлено, что consumers запрещают всякий restart.
  Wrapper держит Transport только пока держат error; global retained inventory
  и abnormal native/provider panic/Goexit этим срезом не реализованы.

3 Go +2 ORIGIN +4 docs;8 existing snapshots/1 new product source. Manual4
existing Windows diffs +1 new source; touched-only gofmt3 exit0.14 captured
reviewed project inputs/0 additional cache reads;261 cache originals/221 verbatim
mirrors и prior owned bytes сохранены. No tests/fixtures/build/vet/types/product/
SDK/browser/native/admin/VPS/live/system network mutations/generation.

Далее: exact original init/wrap/provider prefix и source review abnormal cleanup
ownership. Full native/global/Stop/UX/release приёмка OPEN. Receipts: .artifacts/memory/bootstrap-v428-setup-cleanup-.
Source/unverified. No tests/fixtures/build/vet/typecheck/product/SDK/browser/native/admin/VPS/live/system network changes/generation/dependencies/other roles/chats/subagents. Manual source/diff/byte inspection and touched-only gofmt do not prove compilation/private mocks/tags, race freedom, native DNS/bind/socket/close/deadline/constructor/provider interleavings, resource cleanup, interoperability or speed. Known ordinary setup errors are covered, not abnormal panic/Goexit in native/provider factories. Resolver remains original net.ResolveUDPAddr without a new cancellation/timeout contract; original IPv4 bind choice remains. Cleanup uses cached original Transport.Close and does not retry native cleanup. Failed-cleanup wrapper retains original transport only while error is retained; no process-wide inventory, retry suppression, forced opaque-provider cancellation, hard RSS/CPU/FD/Stop/wall-time bound is claimed. Successful cleanup returns original cause unchanged; cleanup failure adds private wrapper, so callers should use existing errors.Is/As chain rather than rely on direct concrete type assertions. sing-quic WrapError preserves Unwrap by source, not runtime proof. Init/wrap/provider/selfClose/abnormal factory/global/Retry/VN/crypto/event/sameConn/reset/path/nativeOS/wholeUX/installer/release/fullPC00..09/old9 remain OPEN. Public signatures/versions/licenses/earlier quotas stay.100 acceptance OPEN/56 checks NOT_RUN/72tasks active/gatesfalse/TUN unavailable/schema2 unsupported/rootNETGUARD001 unchanged;55% rough/unmeasured, not recalculated.

<a id="pc-v429-df-admission"></a>

## V2-429 — assumed DF не запускает PMTU probing

Статус: PC-06 source/unverified, 2026-11-11; quic-go revision31/sing-quic revision39.
V428 — progress; текущий срез — progress. Полная цель PC-00…09 сохранена.

- Прослежен exact init/wrap→Windows setDF→basicConn.capabilities→sconn→
  handshake-confirmed PMTU Start. Source findings: supportsDF default true даже
  без SyscallConn/UDP setup; Windows setDF unconditional true после двух errors.
  Эти пути найдены в source, не воспроизведены native экспериментом.
- Wrapper default false. Native DF отсутствует/не наблюдался — capability не
  назначается из одного наличия PacketConn или адреса. Windows сохраняет две
  существующие setsockopt attempts, их nil-error outcomes и возвращает OR. Control
  error остаётся false/error; missing callback observations остаются false.
- Existing basic/sendConn/handshake PMTU gate используется без новой policy,
  worker, timer, transport fallback или crypto/protocol changes. One-family
  success behavior сохранено. Readback/family/dualstack/new hopping socket/
  migrated path/rawConn bypass не объявлены доказанными этим bool.
- Hop SyscallConn и HY2 socket capability wrappers прочитаны как consumers:
  capability initial setup не является доказательством для каждого future socket.
  Другие OS-specific helpers и Android не изменялись.

2 Go +2 ORIGIN +4 docs;8 existing snapshots/0 new product source. Manual4 existing
Windows diffs; touched-only gofmt2 exit0.13 captured reviewed project inputs,
0 additional cache reads;261 cache originals/220 verbatim mirrors/prior owned
bytes сохранены. No tests/fixtures/build/vet/types/product/SDK/browser/native/
admin/VPS/live/system network mutations/generation. Source uncompiled.

Далее: original init/wrap/provider custody, actual family/path/hopping capability
и abnormal cleanup. Full PMTU/OS/Stop/UX/release приёмка OPEN. Receipts: .artifacts/memory/bootstrap-v429-df-admission-.
Source/unverified. No tests/fixtures/build/vet/typecheck/product/SDK/browser/native/admin/VPS/live/system network changes/generation/dependencies/other roles/chats/subagents. Manual source/diff/byte inspection and touched-only gofmt do not prove compilation/private mocks/tags, race freedom, Winsock option/control/provider/handshake/migration/hopping interleavings, native capability, packet fragmentation, measured PMTU or speed. The flag records successful option-call observations, not getsockopt readback, valid family/dual-stack routing, every new hopping socket or a current path. One-family OR behavior is preserved, not newly proved sufficient for every family. Opaque preconfigured providers without explicit native setup may remain capable, but this wrapper has no evidence to enable probing; no capability recovery API is invented. Existing rawConn bypass providers and other OS-specific setDF implementations retain separate contracts; only Windows platform helper and shared Windows-source wrapper changed. Missing callback observations stay false but do not prove an arbitrary provider honored synchronous lifetime. Full init/wrap/abnormal factory/provider/global/selfClose/fullPMTU/Retry/VN/crypto/event/sameConn/reset/path/nativeOS/wholeUX/installer/release/fullPC00..09/old9 remain OPEN. Public signatures/versions/licenses/earlier quotas stay.100 acceptance OPEN/56 checks NOT_RUN/72tasks active/gatesfalse/TUN unavailable/schema2 unsupported/rootNETGUARD001 unchanged;55% rough/unmeasured, not recalculated.

<a id="pc-v430-init-custody"></a>

## V2-430 — исходный prefix initializer сохраняется через providers

Статус: PC-06 source/unverified, 2026-11-11; quic-go revision32/sing-quic revision40.
V429 — progress; текущий срез — progress. Вся цель PC-00…09 сохранена.

- Existing lifecycle/initOnce создаёт один embedded transportConnOpening до
  first generator/provider callback. Его socket — exact original из captureLocked,
  которым владеет abort. Actual initializeTransport/raw bypass/wrap используют
  этот socket вместо повторного чтения public Conn после callbacks.
- Original initializer alone writes prefix fields. LocalAddr value сохраняется
  до packetaddr.Capture, returned SyscallConn до buffer/DF Control. В basic branch
  partial wrapper сохранён до logger.Infof. Windows newConn inspected as direct
  basicConn constructor; его returned wrapper attached до дальнейшей публикации.
- setReceiveBuffer и setSendBuffer принимают тот же original prefix. Каждый
  returned control view сохраняется до последующих inspect/set/Control callbacks.
  Source и generated write mirror изменены вручную совместно; go:generate не
  запускался, header/licence/трансформация template сохранены.
- Known initializer return clears prefix under metadata lock after t.conn/worker
  publication либо known error. Panic/Goexit сохраняет prefix с existing negative
  initDone outcome. Root по-прежнему joins original initializer. Error return сам
  по себе не доказывает native cleanup: это обязанность original Close owner.
- Provider calls остаются вне metadata locks. Нет новых workers/channels/maps/
  native options/timeouts/cleanup retries. Captured socket identity не защищает
  от произвольных изменений внутри external provider; async/native/global scope OPEN.

5 Go +2 ORIGIN +4 docs;11 existing snapshots/0 new product source. Manual7 existing
Windows diffs; touched-only gofmt5 exit0.11 captured reviewed project inputs,
0 additional cache reads;261 cache originals/218 verbatim mirrors/prior owned
bytes сохранены. No tests/fixtures/build/vet/types/product/SDK/browser/native/
admin/VPS/live/system network mutations/generation. Source uncompiled.

Далее: provider/native/global retirement и family/path/hopping capability gates.
Полная init/PMTU/OS/Stop/UX/release приёмка OPEN. Receipts: .artifacts/memory/bootstrap-v430-init-custody-.
Source/unverified. No tests/fixtures/build/vet/typecheck/product/SDK/browser/native/admin/VPS/live/system network changes/generation/dependencies/other roles/chats/subagents. Manual source/diff/byte inspection and touched-only gofmt do not prove compilation/private mocks/tags, race freedom, initializer/provider/Control/Close/Goexit/queue/trace interleavings, native resources or speed. Prefix retains returned objects after they become observable; internals of a provider before return or deliberately asynchronous callbacks remain outside this source contract. Buffer helpers retain their current original control view until their ordinary return allows reuse; external retained aliases/FD duplication/underlying mutations are separate. Snapshot socket identity is the exact captured interface, not immutable OS-object or trusted-provider proof. Known initializer error ends its callbacks but does not alone prove native cleanup; existing original Close owns cleanup. Windows newConn is a direct basicConn constructor by source; non-Windows constructor internals were not given a new capture API. The write mirror was manually edited with its template, no generation executed; generator output and omitted upstream tests/private helpers remain unverified. One embedded prefix adds fixed metadata per Transport; no measured overhead, hard RSS/CPU/FD/Stop/wall-time or global inventory guarantee. Full init/provider/async/abnormal factory/global/selfClose/fullPMTU/family/hopping/raw-provider/Retry/VN/crypto/event/sameConn/reset/path/nativeOS/wholeUX/installer/release/fullPC00..09/old9 remain OPEN. Public signatures/versions/licenses/earlier quotas stay.100 acceptance OPEN/56 checks NOT_RUN/72tasks active/gatesfalse/TUN unavailable/schema2 unsupported/rootNETGUARD001 unchanged;55% rough/unmeasured, not recalculated.

<a id="pc-v431-pmtu-reset"></a>

## V2-431 — PMTU не включается повторно через безусловный Reset

Статус: PC-06 source/unverified, 2026-11-11; quic-go revision33/sing-quic revision41.
V430 — progress; текущий срез — progress. Полная цель PC-00…09 сохранена.

- Source findings: Reset безусловно устанавливал lastProbeTime=now; оба migration
  callers обходили initial config/DF Start gate, а due-probe send gate не повторял
  config/DF. Старые callbacks фильтровались через generation uint8; повторение
  значения после256 resets структурно возможно. Native reproduction не делалась.
- Один private pathMTUDiscoveryEnabled использует handshakeConfirmed, config и
  actual sendConn DF. Initial Start, оба Reset callers и due-probe send используют
  этот gate. Client switching передаёт prospective nextConn, server сохраняет
  original raw sendConn. Private Reset получает explicit allowProbing; refusal
  оставляет lastProbeTime zero, search min/max при этом reset как раньше.
- Finder хранит identity существующего nonzero-sized handler object вместо
  generation counter. Reset invalidates current identity; known ACK/loss после
  проверки invalidates её перед дальнейшими updates. Stale/duplicate callback
  не может приобрести authority над later probe через counter wrap.
- Source per-probe allocation остаётся прежней: handler и PingFrame. RTT delay,
  search bounds/loss slots/payload algorithm и frame/output custody сохранены.
  Дополнительный current-handler pointer имеет fixed metadata cost, не measured
  overhead. Доступность старого callback после256 resets в runtime не доказана.

2 Go +2 ORIGIN +4 docs;8 existing snapshots/0 new product source. Manual4 existing
Windows diffs; touched-only gofmt2 exit0.13 captured reviewed project inputs,
0 additional cache reads;261 cache originals/217 verbatim mirrors/prior owned
bytes сохранены. Preparation metadata ENOENT исправлен до product edits; исходные
snapshots сохранены, resume записал только отсутствующие metadata files.
No tests/fixtures/build/vet/types/product/SDK/browser/native/admin/VPS/live/system
network mutations/generation. Source uncompiled; private mocks/tests untouched.

Далее: original path-switch/sendQueue startup и actual family/hopping capabilities.
Full PMTU/provider/native/global/OS/Stop/UX/release приёмка OPEN. Receipts: .artifacts/memory/bootstrap-v431-pmtu-reset-.
Source/unverified. No tests/fixtures/build/vet/typecheck/product/SDK/browser/native/admin/VPS/live/system network changes/generation/dependencies/other roles/chats/subagents. Manual source/diff/byte inspection and touched-only gofmt do not prove compilation/private mocks/tags, race freedom, config/DF/handshake/reset/ACK/loss/output/migration/provider interleavings, native PMTU or speed. Prior uint8 alias risk is established structurally; no practical delayed-callback reachability, timing exploit or runtime reproduction is claimed. Current handler identity is a source ownership gate; native/data-frame/opaque callback retirement retains its own contracts. Finder adds one current-handler pointer, reusing the existing explicit per-probe handler allocation; compiled allocation/escape cost, global/RSS/CPU/Stop limits and performance remain unmeasured. The gate consumes existing sendConn DF metadata, not getsockopt readback, family/dual-stack/new hopping socket/current path or server new-destination proof. Client reset uses prospective nextConn; server reset retains its original raw send connection. Due-send check applies to new probes, not a new whole-carrier packet-size or blackhole-recovery algorithm. Private Reset and both production callers changed together; omitted tests/mocks were not modified or run. Full path switch/sendQueue/provider/global/selfClose/family/hopping/raw-provider/Retry/VN/crypto/event/sameConn/reset/nativeOS/wholeUX/installer/release/fullPC00..09/old9 remain OPEN. Public signatures/versions/licenses/earlier quotas stay.100 acceptance OPEN/56 checks NOT_RUN/72tasks active/gatesfalse/TUN unavailable/schema2 unsupported/rootNETGUARD001 unchanged;55% rough/unmeasured, not recalculated.

<a id="pc-v432-path-switch"></a>

## V2-432 — switching candidate не теряется при отказе sendQueue

Статус: PC-06 source/unverified, 2026-11-11; quic-go revision34/sing-quic revision42.
V431 — progress; текущий срез — progress. Полная цель PC-00…09 сохранена.

- Source findings: initial/migrated queue tasks сообщали Conn только о normal
  Run error; Goexit обходил notification. Existing sendQueue.finishWorker при
  этом уже сохранял negative custody. Switch не имел собственного original
  candidate и final target-close publication fence. Native reproduction не делалась.
- Existing switch вынесен в connection_path.go после поиска аналогов. Один
  embedded prefix до providers/old Close хранит exact target/raw/remote/previous
  conn+queue/returned next conn+queue. Known outcomes clear; negative original
  retirement и abnormal switch сохраняют prefix/mark uncertainty. Loop completion
  дополнительно требует отсутствия active candidate.
- Target snapshot и final publication используют mutex→lifecycle.access и
  sealed/failed/closeErr. Actual conn+queue assignment и существующий worker start
  выполняются в одном target seal fence. Providers, old Close, migration stats
  и PMTU gate — вне locks. No interface-to-interface equality invokes custom
  comparability under metadata locks. Conn close проверяется atomic observation.
- Common runSendQueue wrapper сохраняет exact original queue аргументом task;
  deferred cause изначально uncertain и заменяется только actual Run return.
  Error/Goexit/panic request original Conn close. Original panic propagation,
  packet custody и cached queue Close сохраняются, повторного Run/Close нет.
- Known refused unstarted candidate не ждёт несуществующий worker и не закрывает
  shared raw socket. PMTU использует тот же prospective nextConn gate. Final
  target CID removal/multi-owner lifetime и constructor internals остаются OPEN.

2 Go +2 ORIGIN +4 docs;7 existing snapshots/1 new product source. Manual3 existing
Windows diffs +1 new source; touched-only gofmt2 exit0.13 captured reviewed project
inputs/0 additional cache reads;261 cache originals/217 verbatim mirrors/prior
owned bytes сохранены. No tests/fixtures/build/vet/types/product/SDK/browser/native/
admin/VPS/live/system network mutations/generation. Source uncompiled.

Далее: newSendConn metadata/provider prefix и additional-path original ownership.
Full migration/provider/native/global/OS/Stop/UX/release приёмка OPEN. Receipts: .artifacts/memory/bootstrap-v432-path-switch-.
Source/unverified. No tests/fixtures/build/vet/typecheck/product/SDK/browser/native/admin/VPS/live/system network changes/generation/dependencies/other roles/chats/subagents. Manual source/diff/byte inspection and touched-only gofmt do not prove compilation/private mocks/tags, race freedom, sender/provider/newSendConn/Close/publication/Goexit/migration/target-root interleavings, native retirement or speed. Target mutex->access fence orders its metadata seal/closeErr and publication, not atomic OS-path/Conn cancellation, CID authority or full multi-transport ownership. Conn-close observation is a point gate; already admitted work retains original owners. Additional-path target CID removal, original-client list membership, callbacks/aliases/cross-path roots and global retention remain separate. Prefix stores constructor inputs and returned candidate; LocalAddr/Capture internals inside newSendConn were not given a new prefix API here. Worker guard requests original close on abnormal Run, while the queue own guard supplies packet custody; panic still propagates and no opaque provider is forcibly canceled. Known unstarted refusal has no worker/native write and never closes borrowed/shared raw state. Fixed per-Conn candidate metadata is not a global/RSS/CPU/FD/Stop bound or measured overhead. Full constructor/provider/async/abnormal factory/global/selfClose/family/hopping/fullPMTU/Retry/VN/crypto/event/sameConn/reset/path/nativeOS/wholeUX/installer/release/fullPC00..09/old9 remain OPEN. Public signatures/versions/licenses/earlier quotas stay.100 acceptance OPEN/56 checks NOT_RUN/72tasks active/gatesfalse/TUN unavailable/schema2 unsupported/rootNETGUARD001 unchanged;55% rough/unmeasured, not recalculated.

<a id="pc-v433-send-metadata"></a>

## V2-433 — sender construction и чтение опубликованного пути

Статус: PC-06 source/unverified, 2026-11-11; quic-go revision35/sing-quic revision43.
V432 — progress; текущий срез — progress. Полная цель PC-00…09 сохранена.

- Source findings: c.conn переписывался при migration, public LocalAddr/RemoteAddr/
  SetRemoteAddr и ConnectionState GSO читали его без этого publication fence.
  gotGSOError — plain bool, writer queue и metadata/connection-loop readers разные.
  Эти concurrency paths прослежены в source; race detector не запускался.
- Parent-owned sendConnOpening во всех3 actual callers хранит raw/remote/local/
  adjusted address/OOB/partial sconn до LocalAddr/Capture. Server original opening
  активен до newSendConn. Known parent return clears; abnormal parent guards
  сохраняют оригиналы. Shared raw не закрывается как частичный sender.
- Immutable connectionSendState связывает один sender. Both real Conn constructors
  публикуют embedded initial view до trace callbacks; migration создаёт independent
  node под existing active candidate и Store в target seal fence. Prefix reuse не
  меняет опубликованный node. Initial view держит initial sender до Conn release.
- Public address methods/setter и ConnectionState GSO используют atomic Load
  view. gotGSOError Store/Load atomic. Нативные address values/aliases и setter
  migration-intent serialization сохраняют свой отдельный scope; setter point-loaded.
- wroteFirstPacket audit не подтвердил такой же concurrency defect: current
  Write идёт original queue worker, terminal write следует cached queue Close,
  probes используют WriteTo без этого flag. Алгоритм первого permission retry
  и native call count оставлены прежними. Custom private concurrent callers OPEN.

6 Go +2 ORIGIN +4 docs;12 existing snapshots/0 new product source. Manual8 existing
Windows diffs; touched-only gofmt6 exit0.12 captured reviewed project inputs,
0 additional cache reads;261 cache originals/217 verbatim mirrors/prior owned
bytes сохранены. No tests/fixtures/build/vet/types/product/SDK/browser/native/
admin/VPS/live/system network mutations/generation. Source uncompiled.

Далее: additional-path original ownership/target CID removal, metadata aliases
и native-call lifetime. Full migration/provider/global/OS/Stop/UX/release OPEN.
Receipts: .artifacts/memory/bootstrap-v433-send-metadata-.
Source/unverified. No tests/fixtures/build/vet/typecheck/product/SDK/browser/native/admin/VPS/live/system network changes/generation/dependencies/other roles/chats/subagents. Manual source/diff/byte inspection and touched-only gofmt do not prove compilation/private mocks/tags, race freedom, metadata/constructor/provider/Close/atomic-view/GSO/setter/migration/target-root interleavings, native resource retirement or speed. Published binding is immutable; underlying sender, remote address values and provider state retain separate mutability/lifetime contracts. SetRemoteAddr selects a sender at atomic-load point; no linearizable migration-intent, address-cloning/alias prevention or cross-call coherent snapshot is promised. Initial embedded view retains the original sender for Conn lifetime; later independent nodes stay alive with readers and are not a global/RSS/CPU/Stop bound. Parent reservations precede fixed constructor prefixes and migration-node allocation; costs and escape/GC behavior remain unmeasured. Source inspection supports keeping wroteFirstPacket unchanged for existing worker/terminal Write and direct WriteTo probe paths, not arbitrary private custom concurrent callers. Private constructor callers changed together, and both real Conn constructors publish initial view before trace callbacks; old direct private fixtures/mocks need their own later acceptance. Full metadata API/peer state/provider async/target CID removal/additional-path/multiowner/global/selfClose/family/hopping/PMTU/Retry/VN/crypto/event/sameConn/reset/nativeOS/wholeUX/installer/release/fullPC00..09/old9 remain OPEN. Public signatures/versions/licenses/earlier quotas stay.100 acceptance OPEN/56 checks NOT_RUN/72tasks active/gatesfalse/TUN unavailable/schema2 unsupported/rootNETGUARD001 unchanged;55% rough/unmeasured, not recalculated.

<a id="pc-v434-path-ownership"></a>

## V2-434 — original Conn вне CID/reset-таблиц дополнительного Transport

Статус: PC-06 source/unverified, 2026-11-11; quic-go revision36/sing-quic revision44.
V433 и текущий срез — progress; полная цель PC-00…09 сохраняется.

- Source finding: initial client Transport уже имел отдельный original client
  list. На дополнительном target Conn находился только через CID/reset rows,
  которые RemoveAll/ReplaceWithClosed удаляют до actual original-loop retirement.
  Close target мог пропустить ещё завершающееся соединение.
- Original loop резервирует одну из64 fixed pathConnectionOpening cells до
  CID callbacks/map promotion. Cell хранит Conn, callbacks и факт новой binding;
  abnormal callback/rollback не очищает original prefix, а seals/aborts target.
- AddConnRunner возвращает added bool вместе с прежней ошибкой. Known Add
  refusal уже завершил original rollback; refusal при promotion откатывает только
  newly added runner. Existing binding остаётся. Это позволяет закрываемому
  unactivated target завершить попытку без разрушения старого рабочего Conn.
- Promotion в mutex→lifecycle.access fence проверяет target seal и original
  Conn close reason, затем сохраняет отдельную per-target/Conn row. CID/reset
  removal её не удаляет. Root ждёт фактические регистрации до handler snapshot,
  dedups path set/CID/reset/client list; trace gate учитывает обе новые сущности.
- retireTransportConn удаляет path rows только после positive original packet
  input + stream receive + stream send/call retirement. Ended-loop runner map
  immutable; concurrent observers повторяют лишь locked metadata deletion,
  без нового native Close. Обычный original client worker вызывает тот же join.
- AddPath теперь отказывает до native init при closing, незавершённом handshake
  или nil target; после init повторяет readiness. Это не готовность handshake-
  confirmed PMTU или реального OS-path. Existing 200ms initial RTT сохранён.

5 Go +2 ORIGIN +4 docs;10 existing snapshots/1 new product source. Manual6 existing
Windows diffs +1 new source; touched-only gofmt5 exit0.11 captured reviewed
project inputs/0 additional cache reads;261 cache originals/217 verbatim mirrors/
prior owned bytes сохранены. No tests/fixtures/build/vet/types/product/SDK/browser/
native/admin/VPS/live/system network mutations/generation. Source uncompiled.

Далее: reset-token migration/path retirement, metadata aliases и native-call
lifetime. Path.Close не получает general runner-detachment; active/retained
connection/runner/path counts и global bounds остаются отдельной работой.
Receipts: .artifacts/memory/bootstrap-v434-path-ownership-.
Source/unverified. No tests/fixtures/build/vet/typecheck/product/SDK/browser/native/admin/VPS/live/system network changes/generation/dependencies/other roles/chats/subagents. Manual source/diff/byte inspection and touched-only gofmt do not prove compilation/private mocks/tags, races, callback/rollback/target seal/root/Conn retirement interleavings, wire compatibility, OS resource retirement or speed. The fixed64 registration class adds metadata and one done channel, no new worker/native cleanup retry. Its admission does not bound all active connections, retained runners, path-manager entries, global bytes/RSS/CPU/FD or Stop wall-time. Strong rows are one per target/Conn, independent of CID/reset aliases; removal uses runner-map immutability after actual original Conn-loop/input/stream/call retirement, not early cancellation. Concurrent locked deletion is repeatable metadata retirement, not repeated socket/native Close. Known rejected unactivated target is rolled back before root snapshot; abnormal rollback/callback preserves the original prefix and negative target result. Existing valid binding is never rolled back just because another registration attempt is refused. Path.Close does not generally detach runner state. Reset-token migration, metadata aliases/providers/native calls, global/selfClose/family/hopping/PMTU/Retry/VN/crypto/event/sameConn/reset/OS/wholeUX/installer/release/fullPC00..09/old9 remain OPEN. Public signatures/versions/licenses/earlier quotas stay.100 acceptance OPEN/56 checks NOT_RUN/72tasks active/gatesfalse/TUN unavailable/schema2 unsupported/rootNETGUARD001 unchanged;55% rough/unmeasured, not recalculated.

<a id="pc-v435-path-reset"></a>

## V2-435 — доставка reset-токенов всем зарегистрированным транспортам

Статус: PC-06 source/unverified, 2026-11-11; quic-go revision37/sing-quic revision45.
V434 и текущий срез — progress; полная цель PC-00…09 сохраняется.

- Source finding: оба destination-CID manager callbacks замыкали initial runner.
  Дополнительный target получал source CIDs, но не reset tokens. Его unknown-CID
  lookup в Transport не мог передать такой reset в Conn; fallback внутри Conn
  помогает только пакету, уже доставленному через известный CID.
- Существующий connRunners теперь использует уже объявленные runner AddResetToken/
  RemoveResetToken, без новых callback fields. Оба real constructors направляют
  manager callbacks через этот набор. Known fanout failure просит existing Conn
  close, частичные rows остаются original-owned до обычного удаления.
- Новый runner после source CID registration получает текущие active/probing
  tokens существующего manager до V434 path-owner promotion. Итерация без
  копирования inventory проходит original loop под existing path-manager mutex;
  visitor меняет лишь target map. Same-owner повторный Add/foreign-owner refusal
  и expected-original Remove переиспользуются без изменения identity policy.
- Known token или promotion refusal удаляет target reset rows до original CID
  rollback, только если runner действительно новый. Existing binding не удаляется.
  Abnormal callback/rollback сохраняет cell/Conn/manager/binding с negative result.
  Future rotation, path retirement и Conn Close используют общий removal fanout.
- Source audit подтвердил уже имеющийся retireConnID в failed-enable branch;
  дополнительного исправления/повторного retirement там не добавлено.
  Публичный Path.Close отдельно вызывает manager retirement из caller task:
  его races/shutdown/повторный close не объявляются исправленными этим срезом.

4 Go +2 ORIGIN +4 docs;10 existing snapshots/0 new product source. Manual6 existing
Windows diffs. Touched-only gofmt4 exit0, после уточнения comment — gofmt1 exit0.
10 captured reviewed project inputs/0 additional cache reads;261 cache originals/
216 verbatim mirrors/prior owned bytes сохранены. conn_id_manager mirror теперь
owned source с исходным frozen hash. No tests/fixtures/build/vet/types/product/
SDK/browser/native/admin/VPS/live/system network mutations/generation. Uncompiled.

Далее: public Path.Close original-loop CID retirement и end-of-Conn behavior,
metadata aliases/native-call lifetime. Global runner/path/resource gates OPEN.
Receipts: .artifacts/memory/bootstrap-v435-path-reset-.
Source/unverified. No tests/fixtures/build/vet/typecheck/product/SDK/browser/native/admin/VPS/live/system network changes/generation/dependencies/Git mutations/other roles/chats/subagents. Manual source/diff/byte inspection and touched-only gofmt do not prove compilation/private mocks/tags, races, packet/reset/path/target seal/rollback/root/Conn retirement interleavings, native token delivery/OS cleanup or speed. Existing connRunner reset methods are reused instead of expanding planned callback fields. Hydration and rollback iterate existing active/probing state on original connection loop under path-manager mutex; callbacks only visit target. No copied inventory, worker, lock, channel, native retry or new token identity authority. Manager callbacks may still run through existing public Path.Close caller-side retirement; its concurrency/shutdown/panic contract remains OPEN and is next, rather than being called fully loop-owned or race-safe. Same-owner repeat Add and owner-checked removal remain; existing duplicate-token policy/custom asynchronous runners/prefix lifetime need separate acceptance. Failed enable already retires its probing CID, unchanged. Global connection/runner/path/count/bytes/RSS/CPU/FD/Stop bounds, metadata aliases/native provider calls, selfClose/family/hopping/PMTU/Retry/VN/crypto/event/sameConn/reset/OS/wholeUX/installer/release/fullPC00..09/old9 remain OPEN. Public signatures/versions/licenses/earlier quotas stay.100 acceptance OPEN/56 checks NOT_RUN/72tasks active/gatesfalse/TUN unavailable/schema2 unsupported/rootNETGUARD001 unchanged;55% rough/unmeasured, not recalculated.

<a id="pc-v436-path-close"></a>

## V2-436 — Path.Close без caller-side CID/reset/frame mutation

Статус: PC-06 source/unverified, 2026-11-11; quic-go revision38/sing-quic revision46.
V435 и текущий срез — progress; полная цель PC-00…09 сохраняется.

- Source findings: Path.Close вызывал connIDManager.RetireConnIDForPath из caller,
  тогда как Get/Add/Close manager меняет original loop. abandon закрывался после
  mutex — concurrent Close мог закрыть канал дважды. Проверка len(pathChallenges)
  теряла retirement уже validated path, поскольку validation очищает challenges.
- Caller теперь помечает abandoned на existing row и closes abandon под тем же
  pm mutex, что admission и другие Close. Active-path refusal сохранён. addPath
  не воссоздаёт закрытый Path; enqueue/probe selection/response/switch игнорируют
  abandoned state. No extra channel/lock/worker/retirement queue.
- cidAssigned устанавливается до enable callbacks и живёт независимо от
  challenges. Failed enable очищает его после прежнего retire callback. Original
  loop в ShouldSwitchPath под existing mutex обрабатывает abandoned entries до
  output-pressure check; pending flag позволяет не сканировать map в обычном
  цикле. Row/flag удаляются только после известного return; abnormal result
  остаётся original-loop negative. Закрытие Path — запрос abandonment, не sync
  receipt CID/native retirement. Terminal manager.Close сохраняет свою роль.
- Оба original Conn signals, Closing и Context.Done, присоединяются до atomic
  публикации lazy path manager. Admission/NewPath/Switch/Probe учитывают terminal;
  ожидающий Probe просыпается также после abnormal original-loop completion.
  Manager, созданный после terminal race, видит уже закрытые signals. Это не
  positive lifetime/OS proof. Private signatures/actual callers изменены вместе.

2 Go +2 ORIGIN +4 docs;8 existing snapshots/0 new product source. Manual4 existing
Windows diffs; touched-only gofmt2 exit0, после pending-scan оптимизации ещё
gofmt2 exit0.11 captured reviewed project inputs/0 additional cache reads;
261 cache originals/216 verbatim mirrors/prior owned bytes сохранены.
No tests/fixtures/build/vet/types/product/SDK/browser/native/admin/VPS/live/
system network mutations/generation. Source uncompiled.

Далее: concurrent Probe сейчас заменяет shared validated channel, что оставляет
race/waiter lifetime; queue/challenges/backoff и global path/runner bounds также
OPEN. Native/aliases/provider/OS/Stop/UX/release не объявлены принятыми.
Receipts: .artifacts/memory/bootstrap-v436-path-close-.
Source/unverified. No tests/fixtures/build/vet/typecheck/product/SDK/browser/native/admin/VPS/live/system network changes/generation/dependencies/Git mutations/other roles/chats/subagents. Manual source/diff/byte inspection and touched-only gofmt do not prove compilation/private fixtures/tags, race freedom, Close/Probe/Switch/enable/loop/target/terminal interleavings, native cleanup or speed. Public Path.Close is abandonment acknowledgment: CID/reset/frame retirement may occur later on original loop or normal terminal manager.Close; no synchronous retirement/wall-time/native guarantee is promised. Original loop serializes actual manager retirement, existing pm mutex owns metadata and channel-close. cidAssigned is separate from challenge list and retains unknown callback prefix; original loop negative outcome remains negative. Closing/Context.Done are notification/admission gates, not positive task retirement; a lazily published manager after terminal observes already closed originals. No new worker/timer/lock/channel/retirement queue; pending flag avoids normal per-loop path scans, performance unmeasured. Private addPath/NewPath/remove signatures and actual production callers changed together, tests/fixtures untouched. Duplicate/concurrent Probe still replaces shared validation channel; readers/attempt lifetime/queue/challenge bounds/backoff/path-ID wrap/general runner-detachment/global count/bytes/RSS/CPU/FD/Stop remain OPEN and are next. Metadata aliases/provider/native calls/selfClose/family/hopping/PMTU/Retry/VN/crypto/event/sameConn/reset/OS/wholeUX/installer/release/fullPC00..09/old9 remain OPEN. Public signatures/versions/licenses/earlier quotas stay.100 acceptance OPEN/56 checks NOT_RUN/72tasks active/gatesfalse/TUN unavailable/schema2 unsupported/rootNETGUARD001 unchanged;55% rough/unmeasured, not recalculated.

<a id="pc-v437-path-probes"></a>

## V2-437 — стабильная shared attempt и bounds для Path.Probe

Статус: PC-06 source/unverified, 2026-11-11; quic-go revision39/sing-quic revision47.
V436 и текущий срез — progress; полная цель PC-00…09 сохраняется.

- Source findings: addPath заменял validated channel, тогда как Probe читал его
  вне mutex; concurrent waiters могли ждать разные сигналы. isValidated мешал
  закрытию нового канала при re-probe. Queue и challenges росли через append,
  backoff без saturation мог переполнить duration. Timer создавался снова после
  каждого события. Packet-input retirement не учитывал живые public Probe calls.
- Fixed64 original call cells резервируются до ctx.Done/Cause/schedule callbacks,
  map/attempt/channels/timer allocation. Prefix держит exact ctx/Path/row/attempt/
  timer. Known return stops channel timer вне pm mutex, затем clears cell.
  Abnormal callback/return сохраняет cell/count negative, запрещая positive join.
- Concurrent callers разделяют один attempt с неизменными done/sent channels.
  Только после ухода всех прежних зарегистрированных calls возможен fresh
  attempt. Отмена одного не отменяет остальных. Last known return removes queued
  work; отправленные challenges могут ещё завершиться. Re-probe ждёт новый nonce,
  сохраняя прежнюю usable path validation для непрерывности рабочего соединения.
- Внутренние map rows64, fixed array queue64, unique queued row per path, ring32
  nonce values per attempt. Shared nextDue/nextDelay не умножают wire requests
  при одновременном срабатывании caller timers. Backoff capped30s, public200ms
  сохранён; private initial normalized1ms…30s. Каждый admitted call reuses timer.
- Original loop обходит максимум текущие64 queue rows; отсутствие нового peer
  CID у головы не блокирует path, у которого CID уже есть. Wrapped/nonpositive
  Path ID даёт controlled refusal. Actual wire/enabler/CID/reset/frame ownership
  остаётся в original loop. Terminal/abandon перепроверяются до success result.
- WaitForPacketInputShutdown после actual original-loop/input receipt joins
  registered Probe calls, отпустив packet metadata lock. Manager, опубликованный
  позже, не допускает calls против закрытых originals. Unknown не даёт снять
  V434 strong Conn row как positive retirement. Новых goroutines/native retries
  нет; добавлены fixed metadata и одна manager completion channel.

3 Go +2 ORIGIN +4 docs;8 existing snapshots/1 new product source. Manual4 existing
Windows diffs +1 new source; touched-only gofmt3 exit0.13 captured reviewed
project inputs/0 additional cache reads;261 cache originals/216 verbatim mirrors/
prior owned bytes сохранены. No tests/fixtures/build/vet/types/product/SDK/browser/
native/admin/VPS/live/system network mutations/generation. Source uncompiled.

Далее: response binding к actual входному Transport и lifetime runner bindings.
Пределы ring/backoff требуют отдельной совместимости и измерений; external
unprobed handles/ctx graphs/global resources/aliases/provider/OS/Stop/UX/release
не объявлены ограниченными или принятыми этим срезом.
Receipts: .artifacts/memory/bootstrap-v437-path-probes-.
Source/unverified. No tests/fixtures/build/vet/typecheck/product/SDK/browser/native/admin/VPS/live/system network changes/generation/dependencies/Git mutations/other roles/chats/subagents. Manual source/diff/byte inspection and touched-only gofmt do not prove compilation/private fixtures/tags, races, timer/context/cancel/re-probe/response/queue/CID/Close/rollback/loop/terminal interleavings, wire/window/cadence compatibility, native cleanup or speed. New bounds are per manager:64 admitted calls including uncertain retained cells,64 internal map rows, fixed64 unique queue,32 nonce values per retained attempt; retired rows/context graphs in cells remain distinct holders. External unprobed Path objects/opaque ctx+error graphs, repeated lifetime runner bindings, global bytes/RSS/CPU/FD/Stop are not bounded by this cut. Ring eviction ignores responses beyond32 retained values;30s backoff ceiling/min1ms private normalization/default200ms and bounded CID queue rotation require later acceptance, no optimality claim. Shared attempt channels are immutable while waiters exist; one cancellation does not cancel remaining waiters. Last known return removes queued work; already emitted challenges may still validate, and re-probe preserves prior valid path authority. Packet-input shutdown joins admitted calls after actual loop/input outcome and outside packet locks; Closing/Context.Done notification alone is never retirement. Unknown ctx/schedule callback retains original cell negative, no cleanup retry; callback self-join/asynchronous providers remain OPEN. One manager completion channel/fixed metadata added, no new goroutine/native/public signature/version/license/crypto/CC change. Cross-transport response binding/general runner detachment/metadata aliases/native providers/family/hopping/PMTU/Retry/VN/crypto/event/sameConn/reset/OS/wholeUX/installer/release/fullPC00..09/old9 remain OPEN.100 acceptance OPEN/56 checks NOT_RUN/72tasks active/gatesfalse/TUN unavailable/schema2 unsupported/rootNETGUARD001 unchanged;55% rough/unmeasured, not recalculated.

<a id="pc-v438-path-reuse"></a>

## V2-438 — повторный challenge существующего responder path при full capacity

Статус: PC-06 source/unverified, 2026-11-11; quic-go revision40/sing-quic revision48.
V437 и текущий срез — progress; полная цель PC-00…09 сохраняется.

- Source finding: path_manager.HandlePacket находил existing row и переносил её
  в конец, затем всё равно выполнял full maxPaths/age/eviction branch. При трёх
  свежих rows repeated challenge мог остаться без frames; при expired oldest
  мог преждевременно retire другой CID, хотя новый row вообще не создавался.
- Поиск теперь заканчивается после match/move. Admission/eviction применяется
  лишь к p==nil. Max3, pathTimeout, getConnID, known-no-CID refusal, frame types,
  random challenge и nonce-based response acceptance сохранены. Это source
  correction; runtime воспроизведение/проверка не выполнялись.
- **Поправка к V437 next:** restrictive incoming-Transport filter не нужен и
  не реализован. Проверен primary [RFC9000 §8.2.2–8.2.3](https://www.rfc-editor.org/rfc/rfc9000.html#section-8.2.2):
  responder output и initiator acceptance имеют разные правила. Условие приёма
  matching nonce не заменяется локальным фильтром arrival path. Полный набор
  PC/FR требований этой поправкой не снимается; исправлено основание предложения.
- Actual remaining source: client handlePathChallengeFrame queues response в
  active-path framer; server handleShortHeaderPacket active-remote early return
  пропускает path-manager response. Incoming origin/CID/packetInfo, bounded256
  queue и оригинальный write owner должны пройти до правильного output path,
  сохранив CC/pacing и padding. Immediate per-frame native send и speculative
  receive-path filter не добавлены; full responder/native compliance OPEN.

1 Go +2 ORIGIN +4 docs;7 existing snapshots/0 new product source. Manual3 existing
Windows diffs; touched-only gofmt1 exit0.13 captured reviewed project inputs,
0 additional cache reads;261 cache originals/215 verbatim mirrors/prior owned
bytes сохранены. path_manager mirror promoted в owned с frozen original identity.
Primary RFC read receipt: .artifacts/memory/bootstrap-v438-path-reuse-rfc-source.json.
No tests/fixtures/build/vet/types/product/SDK/browser/native/admin/VPS/live/system
network mutations/generation. Source uncompiled; scientific source read не тест.

Далее: actual responder output routing/queue/CID/packetInfo/padding/CC, затем
general runner/reset/CID lifetime и global retention. Whole-PC acceptance OPEN.
Receipts: .artifacts/memory/bootstrap-v438-path-reuse-.
Source/unverified. No tests/fixtures/build/vet/typecheck/product/SDK/browser/native/admin/VPS/live/system network changes/generation/dependencies/Git mutations/other roles/chats/subagents. Primary RFC web read is protocol research only, not a product/HTTP/VPN/server test. Source finding and manual diff/gofmt do not prove compilation/tags/private fixtures, known-row reorder/full-young/full-expired/new-row/no-CID/frame/ACK/Close interleavings, wire/native compatibility or performance. Existing max3/pathTimeout/new-only eviction/CID/nonce/frame behavior retained. RFC correction withdraws only the proposed same-arrival-Transport initiator filter from V437 next, not original PC/FR requirements or actual responder path correctness. Incoming matching nonce remains valid from any path; no whole QUIC compliance claim. Existing client queues responses through active framer; server active-remote early return skips its path-manager response. Proper same-path responder requires retained incoming origin, peer-CID choice, packetInfo/local address, congestion/pacing, padding, bounded existing256 queue and original output/provider retirement; these remain OPEN. Research rejected speculative transport stamping/filter and unbounded/per-frame immediate sends, no such source changes. General runner/reset/CID detachment/lifetime maps, external unprobed Path/context graphs/global count/bytes/RSS/CPU/FD/Stop, metadata aliases/providers/family/hopping/PMTU/Retry/VN/crypto/event/sameConn/reset/OS/wholeUX/installer/release/fullPC00..09/old9 remain OPEN. No new worker/retry/queue/limit/public API/version/license/crypto/CC change.100 acceptance OPEN/56 checks NOT_RUN/72tasks active/gatesfalse/TUN unavailable/schema2 unsupported/rootNETGUARD001 unchanged;55% rough/unmeasured, not recalculated.

<a id="pc-v439-path-replies"></a>

## V2-439 — routed ответы клиента и server active-peer branch

Статус: PC-06 source/unverified, 2026-11-11; quic-go revision41/sing-quic revision49.
V438 и текущий срез — progress; полная цель PC-00…09 сохраняется.

- Original Transport reader присваивает receivedPacket.transport после actual
  ReadPacket, не доверяя provider hint; Clone/value queue/coalescing сохраняют
  stamp. Both actual Conn constructors удерживают initial Transport из runner
  до trace callbacks. Missing input origin даёт controlled refusal.
- Client и server active-peer handlers сохраняют каждый challenge nonce в
  existing max256 response queue вместе с target/remote/packetInfo. Admission
  предшествует wire frame allocation; второй очереди нет. Explicit legacy
  QueueControlFrame entries остаются обычными, generic Append не берёт routed.
- Исходный loop присоединяет popped route к response prefix до CID/packer/
  trace/native callbacks. Initial или registered path CID выбирается при том же
  current peer; known stale/unregistered/abandoned/no-CID reply отбрасывается
  без другого выходного пути. Independent one-frame array переживает clearing
  prefix в sent history, incoming packet backing не заимствуется очередью.
- Drain в sendPackets выполняется до outgoing probes после SendModeAny/pacing.
  PackPathProbePacket даёт padding1200; затем IsPathProbePacket сбрасывается,
  чтобы обычный SentPacket увеличил bytesInFlight и вызвал current CC. Его
  special challenge branch этого не делал. Это shared-Conn accounting, не
  независимый per-path RTT/CC. Full route-mixed measurement acceptance OPEN.
- Existing packet-output helper общий для probes/replies; новый private
  Transport.writePacketTo сохраняет info/OOB в actual original64/4MiB write
  admission. Public WriteTo/default unchanged. Run retirement учитывает active
  response prefix. Known return освобождает только original; abnormal CID/pack/
  trace/write callback сохраняет negative prefix. Debug log больше не выдаёт
  active LocalAddr за actual source альтернативного socket.

7 Go +2 ORIGIN +4 docs;12 existing snapshots/1 new product source. Manual8 existing
Windows diffs +1 new source; touched-only gofmt7 exit0.19 captured reviewed
project inputs/0 additional cache reads;261 cache originals/214 verbatim mirrors/
prior owned bytes сохранены. framer mirror promoted с frozen original identity.
No tests/fixtures/build/vet/types/product/SDK/browser/native/admin/VPS/live/
system network mutations/generation. Source uncompiled. Pre checkpoint был
отклонён по max2000 text: исходный payload сохранён, shortened own retry1 принят
до source edits; installed harness/snapshots/goal/constraints не менялись.

Далее: server non-active replies/anti-amplification/credit-aware padding;
data-CID path privacy, per-path RTT/CC, stale/pressure reply policy, general runner
lifetime/global retention. Windows OOB nil — kernel local-source selection и
same-native-path delivery не доказаны. Matching-nonce acceptance любого return
path, уточнённый V438/RFC, сохраняется без incoming-Transport filter.
Receipts: .artifacts/memory/bootstrap-v439-path-replies-.
Source/unverified. No tests/fixtures/build/vet/typecheck/product/SDK/browser/native/admin/VPS/live/system network changes/generation/dependencies/Git mutations/other roles/chats/subagents. Manual source/diff/byte inspection and touched-only gofmt do not prove compilation/tags/private fixtures, input-stamp/Clone/coalescing/CID/queue/legacy/Close/callback/write/history interleavings, wire/native correctness, throughput or latency. This cut implements client and server active-peer responses; existing server non-active-peer branch stays separate. Known stale peer/unregistered/abandoned/no-CID reply is discarded, no other-path fallback; full retry/fairness/pressure policy remains OPEN. Response allocation count256 plus one original in-flight prefix is bounded structurally; opaque address/provider/error graphs, global bytes/RSS/CPU/FD/Stop are not. Independent frame-array storage survives prefix clearing; incoming packet bytes are not retained by response queue. Source ordinary SendModeAny/pacing and normal SentPacket accounting apply, but shared-Conn controller/RTT may mix routes; no independent path CC/RTT accuracy or optimality claim. Server active pre-address-validation/anti-amplification and variable-padding credit, server non-active replies/multiple challenges, initial-versus-migrated normal data CID privacy, lifecycle runner detachment and queue-terminal retirement need separate work/acceptance. Windows/non-OOB packetInfo.OOB remains nil: retaining metadata does not prove kernel-selected source IP/interface, multi-homing or actual same-native-path delivery. Existing legacy private QueueControlFrame route meaning is preserved; actual routed producer rejects missing origin, generic Append never consumes routed records. Public WriteTo signature/default stays; private reservation/handler/caller changes uncompiled. Unknown originals prevent positive loop/output/write retirement; no native cleanup retry. RFC matching-nonce acceptance from any return path unchanged. No new queue/goroutine/timer/public API/version/license/crypto/CC algorithm. FullPC00..09/old9/100 acceptance OPEN/56 checks NOT_RUN/72tasks active/TUN unavailable/schema2 unsupported/rootNETGUARD001/OS/wholeUX/release remain OPEN;55% rough/unmeasured, not recalculated.

<a id="pc-v440-path-credit"></a>

## V2-440 — credit3x, address/MTU и server migration loss state

Статус: PC-06 source/unverified, 2026-11-11; quic-go revision42/sing-quic revision50.
Полная цель PC-00…09 и прежние обязательства остаются active.

- Existing max3 path records привязаны к Transport/remote/packetInfo и сохраняют
  saturating3x credit. Original processing anchor считает datagram один раз,
  включая coalesced input; input должен пройти authenticated packet handling.
- Один loop-owned serverProbe prefix удерживает original row/route/frames/buffer/
  outcome и участвует в actual run retirement. Allowance резервируется до CID/
  pack/trace/write callbacks. Исправлена immediate ветка: CID больше не выдаётся
  раньше общего reservation. Known unused packing и known no-attempt отказ
  возвращают credit только исходному row. После входа в common output helper
  debit сохраняется консервативно даже при отказе до native send. Unknown
  original retained negative; native cleanup не повторяется.
- Private limited packer сохраняет default1200. При меньшем credit он формирует
  минимальный valid packet с header-protection sample и AEAD overhead; если
  frames не помещаются, native output не вызывается. Original buffer guard и
  packetInfo-aware Transport write64/4MiB из V439 переиспользованы.
- Адрес и MTU проверяются раздельно. Matching short response валидирует адрес,
  затем fresh nonce/full1200 follow-up идёт через existing send tick. Миграция
  требует обоих признаков. Fixed32 history содержит только successfully sent
  nonces и actual packet sizes; delayed full response принимается после retry.
  Duplicate short response не заменяет уже отправленную full-size проверку.
- Existing loss callback renews challenge вместо удаления path/CID. Pending
  no-CID/no-credit row не мешает попыткам остальных bounded rows. Ни нового
  worker, ни нового timer/queue нет. Highest non-probing app-data PN учитывает
  server0RTT/1RTT; probing-only пакет не перебивает migration intent. Matching
  response по-прежнему принимается с любого return path.

Primary basis: [RFC9000 §§8/8.2/9.3](https://www.rfc-editor.org/rfc/rfc9000.html).
Это source design, не доказательство полной RFC/native реализации.6 Go +2 ORIGIN
+4 docs;11 existing snapshots/1 new source; manual7 existing Windows diffs +1 new
source. Touched-only gofmt6 exit0 (после CID correction повторено для тех же6);
14 captured reviewed inputs/0 cached reads,261 cache originals/214 verbatim
mirrors/prior owned bytes сохранены. Первые receipts сохранены отдельно перед
refresh; исходные snapshots не менялись. No tests/build/native/live/network changes.

Следующий дефект: ACK на challenge не равен PATH_RESPONSE, но existing ACK
retirement снимает loss alarm. Нужен original-loop response deadline с PTO-aware
backoff/abandonment, включая short→full phase; fixed1s loss renewal не закрывает
этот сценарий. Отдельно остаются initial/bypass/global antiamp, attribution
discarded datagrams, every-challenge response queue, selected data CID promotion/
privacy/lifetime, RTT/CC/old-path defense и kernel local-source selection.
Receipts: .artifacts/memory/bootstrap-v440-path-credit-.
Source/unverified. No tests/fixtures/build/vet/typecheck/product/SDK/browser/native/admin/VPS/live/system network mutations/generation/dependency/Git changes/otherroles/chats/subagents. Manual source/diff/byte inspection and touched-only gofmt do not establish compilation/tags/private fixtures, crypto/credit/CID/coalescing/queue/callback/write/history interleavings, wire/native behavior, throughput or latency. Scope: authenticated non-active server migration paths only. Initial handshake, DisablePathManager/hopping, discarded datagrams/global antiamp and every-challenge routed response queue are separate. Existing special challenge accounting and one-second loss signal are retained; ACK without PATH_RESPONSE removes loss history without an independent response deadline, so complete retry/backoff/abandonment/expiry remains OPEN. Fixed32 sent history can evict delayed nonces; bounded3 records keep existing new-only5s eviction and now retain no-CID pending work. Source stores packetInfo but Windows OOB remains nil; no kernel-source/native-route proof. Data-CID promotion/privacy/retirement, runner lifetime, per-path RTT/CC and old-path migration defense remain OPEN. Credit refunds are conservative: after entering common output helper, even a known pre-native rejection keeps charge. Unknown output retains original prefix/debit and blocks positive retirement; no native retry. No new worker/timer/queue/public API/version/license/crypto/CC algorithm. FullPC00..09/old9/100 acceptance OPEN/56 checks NOT_RUN/72tasks active/TUN unavailable/schema2 unsupported/rootNETGUARD001/OS/wholeUX/release remain OPEN.55% is a rough unmeasured estimate, not recalculated.

<a id="pc-v441-path-timeout"></a>

## V2-441 — независимый таймаут PATH_RESPONSE

Статус: PC-06 source/unverified, 2026-11-11; quic-go revision43/sing-quic revision51.
Полная цель PC-00…09 и старые обязательства сохраняются.

- Original bounded3 server path records получили отдельное value-only состояние
  validation: start/expiry/last send/next retry/PTO/count. ACK и packet-loss
  callback больше не управляют validation/retry. ACK без PATH_RESPONSE не
  выключает этот clock. Existing1s loss alarm удаляет только packet history.
- Существующий Conn timer включает ближайший retry/expiry до hard/congestion
  early return; existing loop обрабатывает его до queue pressure. Новый timer,
  worker или queue не создаётся. Pending unsent retry очищает alarm; no-CID/
  no-credit/no-fit ждёт input/CID/pressure progress либо конечный phase expiry.
- Не больше3 успешно отправленных own challenges за address/MTU phase. Окна
  ответа1/2/4 PTO, phase budget7PTO. New-path floor1s+peerACK или больший current
  RTT PTO; глобальный RTTStats default100ms не менялся. Умножение/сложение времени
  saturating; рост PTO может удлинить original phase, повторные ответы — нет.
  Admission имеет конечный срок; first actual send получает полное response
  window. Native callback delay не превращается в обещание wall-time.
- Первый matching short response начинает full-MTU phase либо сохраняет уже
  outstanding full-size challenge и cadence. История32 sent nonces/sizes остаётся.
  Duplicate short response не перезапускает phase. Full response обнуляет оба
  deadlines; expired phase не принимает late response. Any-return-path matching
  rule сохранён. Source output prefix/credit3x/route/buffer ownership из V440 stay.
- Expiry сначала удерживает exact retiring row перед CID/reset/control-frame
  callbacks, затем known return удаляет именно candidate. Abnormal retirement
  сохраняется в original manager и запрещает positive run retirement; retry нет.
  Active peer/connection не меняются. Старый5s new-path eviction не обрывает
  unexpired validation; bounded3 admission сохранён.

Primary design: [RFC9000 §§8.2.3/8.2.4/9.4](https://www.rfc-editor.org/rfc/rfc9000.html)
и [RFC9002 §§6.2.1/6.2.2](https://www.rfc-editor.org/rfc/rfc9002.html).
4 Go +2 ORIGIN +4 docs;9 existing snapshots/1 new source. Manual5 existing Windows
diffs +1 new source; touched-only gofmt4 exit0.14 captured reviewed inputs/0 cache
reads;261 cache originals/214 verbatim mirrors/prior owned bytes сохранены.
No tests/build/native/VPN/live/system network changes. Source uncompiled.

Далее: server non-active challenges сохраняются parser-ом только последним
frame и обходят routed queue/SendModeAny. Нужно интегрировать каждый challenge
в existing bounded response queue с original route/CID/credit и безопасным
разделением own validation от peer responses. Initial/bypass/global amp, dataCID/
privacy, old-path defense, per-path RTT/CC/kernel/global/OS/UX/release OPEN.
Receipts: .artifacts/memory/bootstrap-v441-path-timeout-.
Source/unverified. No tests/fixtures/build/vet/typecheck/product/SDK/browser/native/admin/VPS/live/system network changes/generation/dependency/Git mutations/otherroles/chats/subagents. Manual source/diff/byte review and touched-only gofmt do not prove compilation/private fixture compatibility, timers/callbacks/queue/ACK/loss/credit/CID/phase interleavings, native output, wire compatibility, performance or runtime behavior. The3-probe limit concerns successful own challenges per address/MTU phase, not peer-triggered response packets or global work. Expiry remains subject to original loop progress and callback/native return; no OS wall-time/RSS/FD/CPU bound. Admission starts a7PTO window and first actual send can rebase it once; larger current RTT PTO may lengthen the original phase, saturating at representable future time. These are conservative source scheduling/count bounds, not optimal latency or complete RFC/global PTO compliance. New-path floor1s+peerACK is local to this policy; global RTTStats default100ms and existing1s packet history loss stay unchanged. New address validation starts MTU budget once, preserving already outstanding full-size probe/cadence; duplicate short response cannot reset it. Sent history32 remains separate from packet ACK/loss history, which has its own retirement scope. Initial/bypass/global amp, discarded attribution, every-challenge routed replies, CID promotion/privacy/lifetime, old-path defense, runner/global custody, per-path RTT/CC, kernel source/OS/UX/release remain OPEN. FullPC00..09/old9/100 acceptance OPEN/56 checks NOT_RUN/72tasks active/TUN unavailable/schema2 unsupported/rootNETGUARD001;55% remains rough/unmeasured. Goal active.

<a id="pc-v442-server-replies"></a>

## V2-442 — normal server replies: original route, очередь и credit

Статус: PC-06 source/unverified, 2026-11-11; quic-go revision44/sing-quic revision52.
Полная цель PC-00…09 и старые обязательства остаются active.

- Original input anchor после authentication/duplicate rejection, до frame
  callbacks, удерживает admitted bounded3 server row и считает datagram credit
  один раз. Known input retirement очищает row pointer. Все parsed challenges
  идут в existing256 response queue после admission; queue хранит nonce copy,
  Transport/remote/packetInfo и scalar immutable pathID, не borrowed row pointer.
  Last-frame private return сохранён, production output его больше не использует.
- Post-frame migration gate обновляет только non-probing PN/flags. Immediate
  server packet send удалён: own validation и ответы идут через existing
  SendModeAny/pacing tick. Common original server prefix резервирует3x до CID/
  pack/trace/write, сохраняет known refund/unknown debit и routed native owner.
- Один ответ может нести due own challenge того же row. Known no-fit при двух
  кадрах позволяет единственный response-only repack до native output; исходный
  packer guard уже вернул свой буфер. Reply использует normal shared-Conn flight
  accounting, own counter/history/clock меняется только при фактически отправленном
  challenge. Это сохраняет продвижение validation под потоком ответов.
- Known no-CID/credit/fit reply возвращается в ту же queue. Scan ограничен её
  исходной длиной; deferred head не блокирует другие routes, immediate retry
  не планируется. Full requeue сохраняет прежний refusal/drop policy; второй
  очереди нет. Native attempted/error/unknown ответ не ставится повторно.
  Unknown inner/outer prefixes сохраняются, nil error заменяется на uncertainty.
- Entry ищет exact original ID+route; expired/retired row не заменяется новым по
  тому же адресу. Known promoted server path ID позволяет queued reply взять
  именно его path CID. Полная normal data-CID promotion/privacy ещё OPEN.
  Explicit legacy queue entries сохраняют прежний ordinary-packet путь.

Интеграционная граница подтверждена исходниками: local
sing-quic/hysteria2/service.go83, hysteria/service.go79 и tuic/service.go75
задают DisablePathManager:true. Обычная server policy здесь не доказывает их
bypass/hopping correctness, actual Windows profile activation или VPS readiness.
Этот flag не менялся. Client shared queue drain затронут, его runtime acceptance
тоже не выполнен. Primary design: [RFC9000 §§8.2.2/8.2.3](https://www.rfc-editor.org/rfc/rfc9000.html).

5 Go +2 ORIGIN +4 docs;11 existing snapshots/0 new source. Manual7 Windows diffs;
touched-only gofmt5 exit0.14 captured reviewed inputs +3 consumer source identities,
0 dependency-cache reads;261 cache originals/214 verbatim mirrors/prior owned
bytes сохранены. No tests/build/native/VPN/live/system network changes.

Далее: terminal response queue/input-path metadata retirement после actual
input/loop/output owners; не освобождать по timeout/cancel/GUI interest. Рядом
остаются actual DisablePathManager consumers/initial-global amp/dataCID/privacy/
per-path CC/kernel/global/OS/UX/release gates. Receipts: .artifacts/memory/bootstrap-v442-server-replies-.
Source/unverified. No tests/fixtures/build/vet/typecheck/product/SDK/browser/native/admin/VPS/live/system network changes/generation/dependencies/Git mutations/otherroles/chats/subagents. Manual source/diff/byte inspection and touched-only gofmt do not prove compilation/private fixtures, frame/queue/credit/CID/phase/packer/callback/native interleavings, wire behavior, speed or memory/CPU/FD/Stop wall-time. Scope is normal server path-manager branch plus shared response drain; local HY2/Hysteria/TUIC service constructors actually set DisablePathManager:true. Neither their bypass behavior nor any real Windows profile/server/VPS is accepted by this cut. Missing origin is refused; bounded3 path admission or full256 response/requeue can refuse an entry under the existing pressure policy. The queue retains scalar path ID, not an old path pointer; stale IDs are dropped, never rebound. Only known no-send is deferred; native attempts and unknown outcomes never requeue. Response-only no-CID policy for legacy/client branches remains its prior scope. A drain scans at most its original queue length; this is a count bound, not elapsed-time fairness or optimal scheduling. Same-path piggyback carries at most one own challenge with one reply; only known minimum-size refusal allows response-only repack before native output, under original reservation. Only actually sent own challenge increments its phase counter; response alone cannot validate the path or spend that count. Shared flight accounting is not per-path RTT/CC accuracy. Initial/bypass/global amp, discarded attribution, queue terminal metadata retirement, normal dataCID promotion/privacy, general runner/global retention, old-path defense/kernel source/OS/UX/release remain OPEN. FullPC00..09/old9/100 acceptance OPEN/56 checks NOT_RUN/72tasks active/TUN unavailable/schema2 unsupported/rootNETGUARD001;55% rough/unmeasured. Goal active.

<a id="pc-v443-path-retirement"></a>

## V2-443 — завершение владельца response/path metadata

Статус: PC-06 source/unverified, 2026-11-11; quic-go revision45/sing-quic revision53.
Полная цель PC-00…09 и прежние обязательства остаются active.

- Original Conn final defer сохраняет actual loop/drain/sendQueue/close callback
  receipts и quiescent packet-output/packer/path prefixes. Sealed input проверяет
  zero count/bytes и отсутствие references в processing/delivery/drain/replay и
  fixed64 call cells. Input lock освобождается до framer mutex. Timeout/cancel
  или закрытый notification не заменяют actual return.
- Под existing controlFrameMutex закрывается routed/deferred/legacy response
  admission, очищаются route/frame entries и backing slice. Late enqueue/requeue
  отклоняется. Quiescent server rows и no-longer-callable CID/logger closures
  очищаются даже у manager, удержанного sent frame handler-ом; его ACK/loss
  callbacks уже no-op. Promoted ID инвалидируется без новых CID/native callbacks.
- Parent returned становится false перед cleanup. Nested final guard даже при
  abnormal unwind публикует отрицательный input/stream-loop result и uncertainty
  error. Original retiring marker удерживается; replacement/retry нет.
  Positive input Done публикуется после metadata retirement. Public
  WaitForPacketInputShutdown требует marker и по-прежнему отдельно ждёт admitted
  outgoing Probe calls. Его signature не меняется.
- Найден дополнительный canonical defect: releaseClosedPacket очищал data/
  remote/info после actual pool calls, оставляя Transport stamp. Это удерживало
  original Transport, в том числе в terminal draining slot. Теперь stamp
  очищается в том же known-return месте. Адрес closed-cache retry копируется
  до release; callers после release возвращаются/очищают original state.
  Extra snapshot closed_conn.go добавлен до правки, сверён с frozen source;
  initial-before.json и snapshot-extension receipt сохранены. Scope стал5 Go.

5 Go +2 ORIGIN +4 docs;11 existing snapshots/0 new source. Manual7 Windows diffs;
touched-only gofmt5 exit0,14 captured reviewed inputs/0 dependency-cache reads,
261 cache originals/214 verbatim mirrors/prior owned bytes сохранены. No tests/
build/native/VPN/live/system network changes. Source uncompiled. Positive
retirement receipt не равен successful I/O/protected VPN: обычная I/O ошибка
может иметь подтверждённое завершение своего владельца.

Далее: оставшиеся generic framer control frames/stream maps и их actual producers.
Нужен отдельный barrier после зарегистрированных stream/control calls и loop;
не добавлять waitAll внутрь ещё active loop, иначе join сам себя заблокирует.
Outgoing Path/Probe, ACK/crypto/native/runner/global retention остаются своими
owners. Bypass/initial amp/dataCID/privacy/CC/kernel/OS/UX/release OPEN.
Receipts: .artifacts/memory/bootstrap-v443-path-retirement-.
Source/unverified. No tests/fixtures/build/vet/typecheck/product/SDK/browser/native/admin/VPS/live/system network changes/generation/dependency/Git mutations/otherroles/chats/subagents. Manual source/diff/byte review and touched-only gofmt do not prove compilation/private fixtures, close/defer/panic/Goexit/queue/mutex/frame/CID/packet callback interleavings, native retirement, wire behavior, RSS/CPU/FD/Stop deadlines or speed. Scope is response queue/server-path metadata after original loop receipts; whole framer controls/stream maps, public outgoing Path/Probe state, sent/ACK/crypto history, immutable sender/initial Transport and general runner/native/global owners remain separate. A positive input shutdown means known retirement of that declared scope, not functional VPN or successful I/O. Ordinary I/O error can coexist with known task retirement. Packet metadata preflight requires sealed zero-count/byte intake, no active processing/delivery/drain/replay references and cleared fixed call slots; it is not a timeout receipt. Input lock is released before framer mutex; cleanup calls no CID/reset/trace/provider/native function. Unknown output/prefix/retirement skips cleanup or keeps original retiring marker and publishes negative result; no replacement/retry. Late routed/deferred/legacy responses are rejected after retirement, not claimed immediately at first Close notification. Canonical Transport clear follows actual pool-return callbacks; no earlier release on failed unwind. Actual local HY2/Hysteria/TUIC service constructors still disable path manager; bypass/initial/global amp, dataCID promotion/privacy, per-path RTT/CC/kernel/OS/UX/release remain OPEN. FullPC00..09/old9/100 acceptance OPEN/56 checks NOT_RUN/72tasks active/TUN unavailable/schema2 unsupported/rootNETGUARD001;55% remains rough/unmeasured. Goal active.

<a id="pc-v444-framer-retirement"></a>

## V2-444 — original framer и all-stream-call retirement

Статус: PC-06 source/unverified, 2026-11-11; quic-go revision46/sing-quic revision54.
Полная цель PC-00…09 и старые обязательства сохраняются.

- Actual framer связывается с existing streamCalls owner в preSetup до public
  Conn/Stream exposure. Original loop либо последний закончившийся caller
  резервирует единственный metadata finalizer после sealed loop=false и actual
  running/count/live zero по всем6 registered classes. Stream wrappers отпускают
  stream mutex/gates до call.finish. Owner lock не удерживается на framer locks.
  Worker, channel wait и self-join внутри active loop не добавляются.
- Positive allDone публикуется после known framer cleanup. Unknown сохраняет
  original pointer/retiring marker, без retry. Separate framerFailure расширяет
  whole-call receipt, сохраняя receive-only scope. Inconsistent retained metadata
  без active producer даёт negative result вместо вечного ожидания. Normal
  active calls проверяют closing condition под уже взятым lock, без лишнего
  registry mutex roundtrip. Скоростной выигрыш не измерен.
- После prior path-response retirement cleanup берёт controlFrameMutex→mutex,
  seals generic queue/stream admission, очищает active/control maps, frame
  entries, ring backing и flow-controller reference. Не вызывает callbacks.
  Late Queue/Append/AddStream/0RTT не восстанавливает очищенные структуры.
  0RTT получил тот же порядок locks, что Append и cleanup. Overflow diagnostic
  изменён на atomic Load/Store вместо несинхронизированного чтения loop-ом.
- WaitForStreamCallsShutdown сохраняет public signature и требует whole-call/
  framer receipt. Existing HY2 client_lifetime.attachQUIC уже вызывает его между
  input/receive и send-data joins. Cleanup автоматический после actual finish,
  даже без вызова waiter. Это source integration, не runtime приёмка HY2.

4 Go +2 ORIGIN +4 docs;9 existing snapshots/1 new lifecycle source. Manual5 existing
Windows diffs +1 new source; touched-only gofmt4 exit0.14 captured reviewed inputs/
0 dependency-cache reads;261 cache originals/214 verbatim mirrors/prior owned
bytes сохранены. Первые receipts сохранены перед refinement активного hot path.
No tests/build/native/VPN/live/system network changes. Source uncompiled.

Следующий реальный scope — streams_map Open/Accept/factory и retained entries:
CloseWithError оставляет stream entries и вызывает их shutdown под map mutex;
Open/Accept не входят в6 registered per-stream classes. Нельзя объявлять их
joined из framer receipt. Current module не содержит отдельную windowUpdateQueue:
onHasConnectionData только schedules original loop. ACK/crypto/outgoing/raw/
runner/global/bypass/initial amp/dataCID/privacy/CC/kernel/OS/UX/release OPEN.
Receipts: .artifacts/memory/bootstrap-v444-framer-retirement-.
Source/unverified. No tests/fixtures/build/vet/typecheck/product/SDK/browser/native/admin/VPS/live/system network changes/generation/dependency/Git mutations/otherroles/chats/subagents. Manual source/diff/byte review and touched-only gofmt do not prove compilation/private fixture compatibility, finalizer/stream/queue/mutex/0RTT/panic/Goexit/callback interleavings, race freedom, native retirement, throughput/RSS/CPU/FD/Stop bounds or speedup. Scope is framer-owned metadata after its original loop and all6 registered stream operation classes, not all Conn Open/Accept/factory/native/ACK/crypto/outgoing public handles. Late unjoined queue producers are fenced by sealed admission; they are not declared joined. Receive-only wait retains its original error/scope; whole stream-call wait includes a separate framerFailure and known framer marker. Normal I/O/capacity errors can coexist with known metadata retirement; positive receipt is not working VPN/protected path proof. Cleanup runs on an original finisher outside registry and stream mutex/gates, with only framer control-then-stream metadata locks; it invokes no stream/flow/provider/native callbacks and has no new worker/wait. Mutex delay is not an OS deadline. Unknown original pointer/retiring marker retained without retry; inconsistent count/live metadata with no actual producer becomes negative instead of an endless wait. Stream-map Open/Accept/factory operations and retained stream entries, ACK/transmitted history, crypto/data scopes, immutable sender/raw/runner/global retirement remain separate. No standalone windowUpdateQueue exists in current inspected module; onHasConnectionData schedules the original loop. Actual local HY2/Hysteria/TUIC service constructors still disable path manager; bypass/initial amp/dataCID/privacy/per-path RTT-CC/kernel/OS/UX/release remain OPEN. FullPC00..09/old9/100 acceptance OPEN/56 checks NOT_RUN/72tasks active/TUN unavailable/schema2 unsupported/rootNETGUARD001;55% rough/unmeasured. Goal active.

<a id="pc-v445-stream-map-calls"></a>

## V2-445 — original Open/Accept calls и ограничение ожиданий

Статус: PC-06 source/unverified, 2026-11-11; quic-go revision47/sing-quic revision55.
Полная цель PC-00…09 и старые обязательства сохраняются.

- Original streamsMap создаёт fixed512 operation slots, используя existing
  maxStreamDataCalls. Все6 parent wrappers резервируют slot до parent/child
  mutex, любого context callback и waiting-channel allocation. Fixed owner
  captures actual ctx/child map/candidate/error. Ordinary return очищает свой
  slot; pressure отклоняет только лишний вызов, без закрытия рабочего carrier.
- Unknown panic/Goexit сохраняет original slot/references, seals admission и
  вызывает existing destroyImpl вне metadata lock. Running остаётся занятым
  через actual failure notification, включая abnormal unwind. Negative held
  slot не становится positive receipt после обычного прекращения выполнения.
- Actual original atomic closeErr привязан до public exposure. Entry и successful
  publication читают этот record, без gap между CAS и более поздним Closing.
  Late candidate не публикуется и не закрывается wrapper-ом вторично: ещё живой
  stream остаётся обязанностью native close своей child map; completed incoming
  row мог уже быть удалён. Весь map/factory/native lifetime этим не закрыт.
- Новый WaitForStreamMapShutdown ждёт original loop и actual registered calls;
  Conn.run публикует только metadata finish, без self-wait. Actual HY2 close task
  вызывает API вместе с прежними joins. Six-class WaitForStreamCallsShutdown
  не расширен скрытно. Registry не заменяется вместе с0RTT child maps; slot
  держит исходный child, но stale-ID generation callbacks ещё не исправлены.
- Outgoing OpenStreamSync проверяет собственный queue cap до make; popped
  channel reference очищается. Err/Done исполняются вне child mutex. Explicit
  held-state guards отпускают только реально удерживаемый mutex при abnormal
  unwind. Incoming Accept сохраняет available-first behavior и lazy Done вне
  lock. Nil context rejected; closed Done с nil Err даёт context.Canceled,
  не zero stream/nil error. Forced cancellation/deadline не обещаны.

6 Go +2 ORIGIN +4 docs;11 existing snapshots/1 new source. Manual7 existing
Windows diffs +1 new source; touched-only gofmt6 exit0.14 captured reviewed inputs/
0 dependency-cache reads;261 cache originals и212 verbatim mirrors сохранены.
Incoming/outgoing maps впервые стали owned changes из2 previous verbatim mirrors;
их original bytes сохранены до правки. Версии dependencies/manifests/tags/licences
не менялись, source pair/rollback API declarations обновлены. No tests/build/
native/VPN/live/system network changes. Source uncompiled.

Открыты actual factory/native-close/control callbacks под map locks, partial
factories и retained rows. Incoming GetOrOpenStream создаёт skipped IDs циклом;
actual service constructors HY2/Hysteria/TUIC задают MaxIncomingStreams1<<60,
TUIC также MaxIncomingUniStreams1<<60.512 public calls не дают stream-row bound.
Нужен count+bytes admission до factory и original close/generation custody;
простое удаление map entries после void close не доказывает last-holder return.
Повторное incoming Close/notification и0RTT stale IDs остаются отдельными.
ACK/crypto/outgoing/raw/runner/global/bypass/initial amp/dataCID/privacy/CC/kernel/
OS/UX/release OPEN. Receipts: .artifacts/memory/bootstrap-v445-stream-map-calls-.
Source/unverified. No tests/fixtures/build/vet/typecheck/product/SDK/browser/native/admin/VPS/live/system network changes/generation/dependency/Git mutations/otherroles/chats/subagents. Manual source/diff/byte review and touched-only gofmt do not prove compilation, fixture/API compatibility, map/slot/context/queue/close/0RTT/panic/Goexit/callback interleavings, race freedom, native retirement or throughput/RSS/CPU/FD/Stop bounds.512 is admitted original Open/Accept operations, not open stream count/bytes or whole-Conn/process resource proof. An opaque context may still block indefinitely outside map locks; no forced cancellation or Stop deadline is asserted. Publication rechecks the original close CAS; it does not make the later method return atomic with close. A refused still-live candidate remains its original child-map shutdown responsibility; already-completed incoming rows may have been deleted. No extra candidate cleanup. Registry persists across child replacement, but stale-ID callbacks/generation custody and factory/native-close prefixes remain unresolved. Incoming skipped-ID allocation can still create many rows, including service declarations1<<60; no row ceiling is inferred from call admission. Existing map factory/shutdown/control callbacks under mutexes and retained stream entries, repeated incoming close notification, ACK/crypto/outgoing/raw/runner/global retirement remain separate. New waiter is an external-owner join of loop and actual admitted calls, not full native/map-row cleanup; existing six-class waiter scope remains unchanged. No new worker/timer/wait inside run. Actual local services still disable path manager; bypass/initial amp/dataCID/privacy/per-path RTT-CC/kernel/OS/UX/release remain OPEN. FullPC00..09/old9/100 acceptance OPEN/56 checks NOT_RUN/72tasks active/TUN unavailable/schema2 unsupported/rootNETGUARD001;55% rough/unmeasured. Goal active.

<a id="pc-v446-incoming-map-lifetime"></a>

## V2-446 — incoming skipped-ID admission и original factory/native close

Статус: PC-06 source/unverified, 2026-11-11; quic-go revision48/sing-quic revision56.
Полная цель PC-00…09 и старые обязательства сохраняются.

- GetOrOpenStream ранее создавал все skipped stream IDs под metadata mutex,
  без local batch bound. Actual HY2/Hysteria/TUIC services advertise1<<60.
  Original incoming map теперь считает всю batch до первого factory/channel/
  sorter/flow-controller/map insertion:4096 rows и16MiB logical metadata credit
  по4KiB на row. Это policy accounting, не physical/RSS/data proof. Valid peer
  ID, превышающий local resources, даёт InternalError; advertised wire limits
  не переписаны и ложного peer STREAM_LIMIT_ERROR нет. Gap arithmetic проверена
  вручную до narrowing; negative/inconsistent original metadata становится
  uncertain, invalid new IDs rejected. MaxStream error snapshot взят под lock.
- Factory/native-close используют один original operation gate; ожидание gate
  никогда не держит map metadata mutex. Factory вызывается вне metadata lock.
  Original batch содержит ID/end/callback/candidate/requested result/remaining
  credits. Candidate retained до insertion, а requested result отдельно от row,
  которую concurrent Accept/Delete может уже consume. nextStreamToOpen растёт
  по known inserted rows, сохраняя отношение next/row/deletion/MAX_STREAMS.
- Known close прерывает batch при очередной проверке; уже returned candidate
  остаётся в исходной map для native shutdown. Refund получает только known
  never-created suffix. Completed unaccepted rows остаются charged до actual
  deletion. Unknown panic/Goexit/publication сохраняет original prefix/credits,
  seals/wakes map; no retry/new factory. Partial objects внутри ещё не вернувшего
  результат opaque constructor требуют отдельного owner, не объявлены solved.
- Close сначала seals Get/Accept/Delete и один раз closes existing notification,
  затем ждёт original factory gate вне metadata mutex. Factory publisher не
  отправляет wake в закрытый канал. После factory return map не имеет writers;
  native close проходит original rows без дополнительного snapshot. Retiring
  stream записан до callback; shutdown callback вне metadata mutex. Unknown
  сохраняет pointer и unfinished close reservation. Known repeat idempotent;
  unfinished/uncertain repeat refuses, не делает cleanup replay/false success.
- Stream rows не удалены после void close callback: return не доказывает
  resource/last-holder retirement. Actual Send/Receive native callbacks могут
  отдельно отмечать uncertainty в существующих budgets. HY2 сохраняет прежние
  input/map-call/receive/all-call/send-data joins с разными error domains.

2 Go +2 ORIGIN +4 docs;7 existing snapshots/1 new source. Manual3 existing Windows
diffs +1 new source; touched-only gofmt2 exit0.21 captured reviewed inputs/
0 dependency-cache reads;261 cache originals/212 verbatim mirrors/prior owned
bytes сохранены. Consumer code/API, dependencies/manifests/tags/licences не
менялись; pair provenance обновлён. No tests/build/native/VPN/live/system network
changes. Source uncompiled; скорость и memory/Stop SLO не измерены.

Следующий scope — outgoing map factory/native close, затем original partial
constructor и incoming MAX_STREAMS callback under metadata mutex. Retained rows
и0RTT old-generation ID callbacks требуют собственных actual retirement/custody
receipts; per-map row limit не является whole-Conn/aggregate0RTT/process bound.
ACK/crypto/raw/runner/global/bypass/initial amp/dataCID/privacy/CC/kernel/OS/UX/
release OPEN. Receipts: .artifacts/memory/bootstrap-v446-incoming-map-lifetime-.
Source/unverified. No tests/fixtures/build/vet/typecheck/product/SDK/browser/native/admin/VPS/live/system network changes/generation/dependency/Git mutations/otherroles/chats/subagents. Manual source/diff/byte review and touched-only gofmt do not prove compilation, private API/fixture compatibility, factory/map/close/context/queue/0RTT/panic/Goexit/callback interleavings, race freedom, native retirement or throughput/RSS/CPU/FD/Stop bounds.4096 rows and16MiB logical metadata credit apply to one original incoming map, including reserved unused suffix/completed unaccepted rows; they are not physical backing, process RSS, aggregate0RTT or arbitrary retained external-stream limits. Fixed4KiB charge is a policy accounting unit, not a measured object size. Wire limits and1<<60 service declarations are unchanged. Valid peer IDs exceeding local resources produce local InternalError, not peer stream-limit blame. Opaque factory/native callbacks can still block actual close indefinitely behind the original operation gate, including forbidden self-reentry; no forced deadline/new task exists. Unknown factory/publication retains original batch/callback/candidate/result/credits; inner constructor resources not yet returned by an opaque factory remain a separate custody gap. Unknown native callback keeps original retiring pointer/unfinished close reservation. Known repeated close is idempotent; unfinished repeat refuses rather than claiming completion or replaying cleanup. Rows are retained after native void return, which does not prove last-holder/data/call/native retirement. Existing global HY2 joins retain their separate failure domains. Outgoing factories/native close, incoming MAX_STREAMS callback under metadata mutex, retained rows, old-generation ID callbacks and whole-Conn/raw/ACK/crypto/runner/global/bypass/dataCID/privacy/CC/kernel/OS/UX/release remain OPEN. FullPC00..09/old9/100 acceptance OPEN/56 checks NOT_RUN/72tasks active/TUN unavailable/schema2 unsupported/rootNETGUARD001;55% rough/unmeasured. Goal active.

<a id="pc-v447-outgoing-map-lifetime"></a>

## V2-447 — outgoing factory/native close и ожидание локальной ёмкости

Статус: PC-06 source/unverified, 2026-11-11; quic-go revision49/sing-quic revision57.
Полная цель PC-00…09 и старые обязательства сохраняются.

- Outgoing maps переиспользуют existing incoming4096-row/16MiB logical policy
  через явные aliases. Count/4KiB logical credits резервируются до factory и
  map insertion. Original ID/factory/candidate/waiter/inserted state сохраняются
  через actual constructor/publication; abnormal return удерживает prefix и
  credit, seals existing queue, без retry. Fixed credit не является RSS bound.
- Shared tryOpenStream обслуживает Open и Sync. Exact existing head waiter
  резервирует ID; direct callers не обходят FIFO. Direct local pressure даёт
  ошибку только лишнему caller. Sync ждёт peer/local capacity в прежней bounded512
  queue; Delete/peer credit/known factory completion будят eligible head. Нет
  нового worker/timer/polling. Immediate Sync не выделяет waiter channel. Перед
  enqueue закрыт lost-wake gap, popped channel reference cleared. Pressure —
  отдельный outcome, не тип close cause: limit-like close не даёт busy retry
  по closed channel. Err/Done остаются вне metadata и operation gate locks.
- Factory/native close выполняются под original operation gate, вне metadata
  mutex; gate никогда не ждёт под metadata lock. Late returned candidate остаётся
  в исходной map для native close, caller получает error без second cleanup.
  Close seals/wakes queued operations до gate wait, fences Get/Delete/SetMax/
  update admission, затем идёт по stable original map без snapshot. Retiring
  stream сохранён до callback. Unknown retains pointer/reservation; known repeat
  idempotent, unfinished repeat negative. Rows после void return не очищаются.
- Window/reset updates берут тот же gate до прежнего metadata mutex: current
  in-flight factory candidate не пропускает update. Сами update callbacks и
  STREAMS_BLOCKED/MAX_STREAMS callbacks требуют следующего prefix/lock среза.
  Actual SendStream native callbacks и current parent parameter ownership
  просмотрены по source; race/native correctness не объявлены.
- Exact incoming/outgoing uncertainty result seals original public map-call
  registry и вызывает existing root close вне lock. Actual invocation running
  сохраняется через notification; foreign errors.Is/format callbacks не нужны.
  Normal local pressure, peer limit и cancel carrier не закрывают. HY2 consumer
  code и input/map-call/receive/all-call/send-data join contracts не менялись.

3 Go +2 ORIGIN +4 docs;8 existing snapshots/1 new source. Manual4 existing Windows
diffs +1 new source; touched-only gofmt3 exit0.17 captured reviewed inputs/
0 dependency-cache reads;261 cache originals/212 verbatim mirrors/prior owned
bytes сохранены. Dependencies/manifests/tags/licences/API не менялись. No tests/
build/native/VPN/live/system network changes. Source uncompiled; superiority,
speed/RSS/Stop guarantees не измерены.

Следующий scope — control/update callback prefixes вне metadata mutex, partial
constructors и original0RTT generation custody/retained-row retirement. Native
void result не заменяет actual last-holder/call/data receipt; whole-Conn/OS/UX/
release/fullPC остаются OPEN. Receipts: .artifacts/memory/bootstrap-v447-outgoing-map-lifetime-.
Source/unverified. No tests/fixtures/build/vet/typecheck/product/SDK/browser/native/admin/VPS/live/system network changes/generation/dependency/Git mutations/otherroles/chats/subagents. Manual source/diff/byte review and touched-only gofmt do not prove compilation, fixture/API compatibility, factory/map/FIFO/local-pressure/close/context/0RTT/panic/Goexit/callback interleavings, race freedom, native retirement or throughput/RSS/CPU/FD/Stop bounds.4096 rows/16MiB logical4KiB metadata credits apply per original map, not measured object size, physical backing/RSS/process/aggregate0RTT or arbitrary externally retained streams. Direct local pressure refuses one operation; Sync waits original bounded queue, does not create fallback carrier/new tasks or poll. Operation gate can still block indefinitely on opaque factory/native providers; context methods stay outside gate/mutex but do not force cancellation of a started factory. Unknown prefix retains original ID/factory/candidate/waiter/row credit; unknown native callback retains original retiring stream/close reservation, without retry. Known repeated close is idempotent; unfinished repeat is negative. Native void return is not last-holder/resource proof; rows remain retained. Exact local map-uncertainty sentinels notify root through existing registry, outside metadata lock while the actual call remains running; normal pressure/peer limit/cancel do not. Window/reset updates take same operation gate before existing metadata lock, preventing missed in-flight candidates; their foreign callbacks, incoming MAX_STREAMS and outgoing STREAMS_BLOCKED callbacks still need prefix/lock work. Partial constructor internals, retained rows, old-generation IDs, parameter ownership and whole-Conn/raw/ACK/crypto/runner/global/bypass/dataCID/privacy/CC/kernel/OS/UX/release remain OPEN. FullPC00..09/old9/100 acceptance OPEN/56 checks NOT_RUN/72tasks active/TUN unavailable/schema2 unsupported/rootNETGUARD001;55% rough/unmeasured. Goal active.

<a id="pc-v448-outgoing-updates"></a>

## V2-448 — original bounded window/reset update callbacks

Статус: PC-06 source/unverified, 2026-11-11; quic-go revision50/sing-quic revision58.
Полная цель PC-00…09 и старые обязательства сохраняются.

- UpdateSendWindow/EnableResetStreamAt больше не вызывают stream methods под
  map metadata mutex. Existing original operation gate сохраняет порядок с
  factory publication/native close, не пропуская in-flight candidate.
- Original update prefix резервирует count<=4096 и element backing<=64KiB до
  allocation. Actual unsafe.Sizeof(T) проверяется делением до умножения, без
  assumed pointer size/overflow. Fixed preallocated capacity не расширяется.
  Factory gate исключает additions; Delete может только уменьшить map между
  reservation и capture. Original stream identities capture под metadata lock,
  callbacks выполняются после Unlock и не переопределяют target по новому ID.
- Kind/limit/count/charge/snapshot/index/current принадлежат original operation.
  Current stream сохраняется до callback; reference/index меняются только после
  его known return. Unknown allocation/capture/panic/Goexit/callback сохраняет
  оригинал и credits, seals map admission/queue, без replacement/retry.
- Known close может остановить ещё не вызванный tail. Known finish очищает
  private references до gate release. Close отдельно проверяет active update.
  Это metadata lifetime, не native/GC/RSS cleanup proof; legacy void callbacks
  и native stream/data/call receipts сохраняют отдельный scope. Worker, timer,
  queue, public API и actual HY2 consumer не изменялись.

3 Go +2 ORIGIN +4 docs;8 existing snapshots/1 new source. Manual4 existing Windows
diffs +1 new source; touched-only gofmt3 exit0.17 captured reviewed inputs/
0 dependency-cache reads;261 cache originals/212 verbatim mirrors/prior owned
bytes сохранены. Dependencies/manifests/tags/licences/API не менялись. No tests/
build/native/VPN/live/system network changes. Source uncompiled; speed/RSS/Stop
guarantees не измерены.

Следующий scope — original outgoing STREAMS_BLOCKED/incoming MAX_STREAMS callbacks
вне metadata mutex с bounded scalar state/custody, затем partial constructor/
parent params/0RTT generation и retained-row retirement. Не объявлять whole-Conn/
native/OS/UX/release/fullPC закрытыми из update snapshot receipt. Receipts: .artifacts/memory/bootstrap-v448-outgoing-updates-.
Source/unverified. No tests/fixtures/build/vet/typecheck/product/SDK/browser/native/admin/VPS/live/system network changes/generation/dependency/Git mutations/otherroles/chats/subagents. Manual source/diff/byte review and touched-only gofmt do not prove compilation, fixture/API compatibility, map/snapshot/update/factory/delete/close/context/0RTT/panic/Goexit/callback interleavings, race freedom, native retirement or speed/RSS/CPU/FD/Stop bounds.4096 elements/64KiB bounds only original snapshot element storage, charged before allocation using actual T size; allocator headers/GC/whole-process memory are separate. Original gate prevents additions during capture but Delete may shrink map; no arbitrary append/replacement identity. Unknown snapshot/current/index/kind/limit/charge retained without retry, even after an operation unwinds. Known finish clears references after actual callback return or a never-called tail at known close; void native methods, GC reclamation and last-holder retirement remain separate. Opaque callbacks/self-reentry can still block original gate, so no forced deadline is asserted. Scalar outgoing STREAMS_BLOCKED/incoming MAX_STREAMS callbacks are still under metadata locks and next in scope; parent parameter/0RTT ownership, partial constructors, retained rows and raw/ACK/crypto/runner/global/bypass/dataCID/privacy/CC/kernel/OS/UX/release remain OPEN. Existing input/map-call/six-class/send-data joins/API/consumer unchanged. FullPC00..09/old9/100 acceptance OPEN/56 checks NOT_RUN/72tasks active/TUN unavailable/schema2 unsupported/rootNETGUARD001;55% rough/unmeasured. Goal active.

<a id="pc-v449-stream-map-control"></a>

## V2-449 — original scalar control dispatcher и close/0RTT fence

Статус: PC-06 source/unverified, 2026-11-11; quic-go revision51/sing-quic revision59.
Полная цель PC-00…09 и старые обязательства сохраняются.

- STREAMS_BLOCKED/MAX_STREAMS callbacks вынесены из metadata и factory/update
  gates. Parent metadata содержит one actual actor/current frame/callback и
  one coalesced monotone pending scalar, без event queue/worker. Current owner
  резервируется до frame allocation. Running actor принимает reentrant pending
  request без повторного gate wait. Nonwaiting TryLock под metadata и atomic
  running/gate release под тем же mutex закрывают lost-handoff gap.
- Exact original frame/callback/current/pending retained на abnormal allocation/
  callback return. Map seals admission, actor gate освобождается только после
  actual unwind, no replacement/retry. Known callback return очищает только
  исходную reference; downstream framer хранит собственный handed-off frame.
  Это не wire delivery/native/GC/last-holder proof.
- Outgoing tryOpen dispatch происходит после factory unlock; SetMax и incoming
  Accept/Delete отпускают metadata до dispatch. Native close сначала seals map,
  затем joins original control без metadata/factory lock, и лишь потом берёт
  factory gate. Не вводится inversion control→factory против close. Own callback
  не может вызывать joining Close; actual c.queueControlFrame только queues frame
  и schedules Conn. Arbitrary private provider deadlines не обещаны.
- Join происходит до old-map Close return. Actual Conn 0RTT path выполняет
  streamsMap.ResetFor0RTT перед framer.Handle0RTTRejection: old scalar actor больше
  не может поздно refill новый framer. Остальные old-stream completion/ID callbacks
  и parent parameter custody этим не закрыты. Known uncalled pending очищается
  после actual actor join; negative original сохраняет frame/callback/scalars.
- MAX_STREAMS считает remaining/base и проверяет maximum count до multiplication/
  addition StreamID, включая exhaustion boundary. Advertised limit не wrap-ится
  signed overflow и не уменьшается; только новый больший limit становится pending.

5 Go +2 ORIGIN +4 docs;10 existing snapshots/1 new source. Manual6 existing Windows
diffs +1 new source; touched-only gofmt5 exit0.17 captured reviewed inputs/
0 dependency-cache reads;261 cache originals/212 verbatim mirrors/prior owned
bytes сохранены. Dependencies/manifests/tags/licences/API/actual HY2 consumer не
менялись. No tests/build/native/VPN/live/system network changes. Source uncompiled;
speed/RSS/Stop guarantees не измерены.

Следующий scope — original per-stream/child generation: current Conn completion
маршрутизируется только по ID в текущую map, а0RTT меняет child maps/повторяет IDs.
Нужен exact original child/framer identity до callback, одновременно с partial
constructor/flow/context/parent parameter custody и retained-row retirement.
Input/map-call/six-class/send-data receipts сохраняются. Whole-Conn/native/OS/UX/
release/fullPC OPEN. Receipts: .artifacts/memory/bootstrap-v449-stream-map-control-.
Source/unverified. No tests/fixtures/build/vet/typecheck/product/SDK/browser/native/admin/VPS/live/system network changes/generation/dependency/Git mutations/otherroles/chats/subagents. Manual source/diff/byte review and touched-only gofmt do not prove compilation, fixture/API compatibility, scalar handoff/frame/callback/reentry/close/factory/update/0RTT/panic/Goexit interleavings, race freedom, native retirement or speed/RSS/CPU/FD/Stop bounds. One actor/current fixed-size control frame/callback plus one scalar bounds only this local dispatcher, not downstream framer/native/provider memory. A continuously supplied original actor may keep draining until close/quiet; no callback wall-time or CPU budget is asserted. Exact original frame/callback/current/pending retained on abnormal return with no retry. Metadata/gate locks release before foreign callbacks, but the actor gate remains owned; joining close must be external to its own callback. Actual original callback only queues into framer and schedules Conn; arbitrary private providers/self-close can still block. Control join before factory gate avoids that gate inversion and precedes old-map return/framer0RTT reset, but does not join all old completion/stream IDs or establish parent parameter ownership. Known callback return is queue handoff, not delivery/native/last-holder proof. Known pending can be discarded only after actual actor return at close; unknown pending remains. Input/map-call/six-class/send-data joins/API/consumer unchanged. Partial constructor/flow/context custody, stale generation IDs, retained rows, ACK/crypto/raw/runner/global/bypass/dataCID/privacy/CC/kernel/OS/UX/release remain OPEN. FullPC00..09/old9/100 acceptance OPEN/56 checks NOT_RUN/72tasks active/TUN unavailable/schema2 unsupported/rootNETGUARD001;55% rough/unmeasured. Goal active.

<a id="pc-v450-stream-generations"></a>

## V2-450 — exact original child completion и framer generation fence

Статус: PC-06 source/unverified, 2026-11-11; quic-go revision52/sing-quic revision60.
Полная цель PC-00…09 и старые обязательства сохраняются.

- До изменения completion маршрутизировался через Conn в текущую parent map по
  StreamID.0RTT меняет children и повторно использует IDs. Каждый initMaps теперь
  создаёт nonzero immutable generation object и4 child-specific senders. Exact
  Delete method value связан с исходным child до первого factory/public exposure,
  без последующего lookup parent.current map. All4 production factories используют
  этот sender, включая bidirectional uniStreamSender и ACK/loss уведомления.
- Data/control Add и active Remove сравнивают token под соответствующим existing
  framer mutex.0RTT swap/clear держит оба mutex, поэтому старый callback либо
  закончился до clear, либо отказан после token change. Нет предварительной
  check/use проверки с последующим unscoped Add. Atomic connection-data hint
  не добавляет mutex и только schedules existing loop; flow authority не даёт.
- Framer создаётся до streamsMap. Original framer/error bindings обязательны,
  initial token установлен до exposure. Actual server/client preSetup call sites
  просмотрены; private constructor/reset signatures изменены вместе с sole
  production Conn caller. Legacy private unscoped helpers оставлены, actual
  factory paths scoped. Public API/HY2 consumer/shutdown contracts не менялись.
- Handle0RTTRejection требует exact next token, очищает active data и
  streamsWithControlFrames под обоими locks, затем публикует token. Control map
  раньше не очищалась. Terminal framer retirement тоже очищает token. Эти
  metadata boundaries не являются join/retirement всех old calls/flow/data.
- Child-specific method binding не требует общего generation router на все4
  old maps, но original child остаётся strong reference. Его rows/resources
  нельзя наивно считать retired из этого notification fence.

5 Go +2 ORIGIN +4 docs;10 existing snapshots/1 new source. Manual6 existing Windows
diffs +1 new source; touched-only gofmt5 exit0.17 captured reviewed inputs/
0 dependency-cache reads;261 cache originals/212 verbatim mirrors/prior owned
bytes сохранены. Dependencies/manifests/tags/licences/public API не менялись.
No tests/build/native/VPN/live/system network changes. Source uncompiled;
private fixture compatibility, speed/RSS/Stop guarantees не проверялись.

Следующий scope — current parent m.supportsResetStreamAt/c.peerParams reads in
factories против native writes и unconditional EnableResetStreamAt calls, затем
old flow-controller counters/context после ConnFlow.Reset. Нужны original config/
generation/flow ownership и partial-constructor/retained-row retirement, без
скрытого fallback или объявления native/OS/fullPC acceptance. Receipts: .artifacts/memory/bootstrap-v450-stream-generations-.
Source/unverified. No tests/fixtures/build/vet/typecheck/product/SDK/browser/native/admin/VPS/live/system network changes/generation/dependency/Git mutations/otherroles/chats/subagents. Manual source/diff/byte review and touched-only gofmt do not prove compilation/private fixture compatibility, generation/factory/notification/framer/reset/close/ACK/loss/0RTT/panic/Goexit interleavings, race freedom, wire compatibility, native retirement or speed/RSS/CPU/FD/Stop bounds. Private newStreamsMap/Handle0RTTRejection signatures change with actual sole Conn caller; public API/consumer/dependency versions unchanged. Nonzero original object identity prevents numeric ID reuse ambiguity; original child method values remain strong references and need actual retired-row/holder proof. No whole-generation memory/RSS limit inferred. Legacy unscoped Conn/framer helpers remain for private seams, while every actual stream factory is scoped; arbitrary private bypass providers are not claimed fenced. Connection-data hint can race a token change after its atomic read but only schedules the existing loop, with no stream/data identity; it is not a flow epoch barrier. Parent m.supportsResetStreamAt and c.peerParams factory reads/writes, unconditional native reset enable, old flow-controller counters after Conn.Reset, partial flow/context/constructor objects and retained rows remain unresolved. Notification fencing does not prove original callbacks/calls/data/native resources ended. Control actor/row/FIFO/update/input/map-call/six-class/send-data receipts retain their separate scopes. ACK/crypto/raw/runner/global/bypass/dataCID/privacy/CC/kernel/OS/UX/release remain OPEN. FullPC00..09/old9/100 acceptance OPEN/56 checks NOT_RUN/72tasks active/TUN unavailable/schema2 unsupported/rootNETGUARD001;55% rough/unmeasured. Goal active.

<a id="pc-v451-stream-parameters"></a>

## V2-451 — immutable stream parameters и согласованный partial reset

Статус: source/unverified, 2026-11-13; quic-go revision53/sing-quic revision61.
Полная цель PC-00…09 и прежние обязательства сохраняются.

- До изменения factories читали mutable m.supportsResetStreamAt, а private
  Conn.newFlowController — live c.peerParams. Теперь immutable snapshot содержит
  только scalar stream windows/counts/reset flag, без адресов/ключей/wire alias.
  Generation atomic heads разделяют received/incoming и applied/outgoing.
- Client получает validated received snapshot для inbound1RTT немедленно:
  crypto и входящий STREAM могут обрабатываться до handshake-complete apply.
  Outbound0RTT сохраняет actual restored limits до1RTT apply. Cached restoration
  инициализирует оба heads до early-ready; новое0RTT generation наследует heads,
  но native outgoing stream limit остаётся закрытым до actual application.
- Каждый из4 constructors читает один original-generation snapshot для reset и
  initial send window. Private flow factory принимает этот scalar явно. Missing
  snapshot даёт original constructor failure, без synthetic defaults. Existing
  original factory/update gates сохраняют последовательность поздних обновлений.
- Native EnableResetStreamAt вызывается только при actual peer flag. Оба
  ValidFor0RTT/ValidForUpdate запрещают saved true → new false. Это правило
  закреплённой [draft06 §3](https://datatracker.ietf.org/doc/html/draft-ietf-quic-reliable-stream-reset-06#section-3),
  прочитанной 2026-11-11; обновления wire version/dependencies не было. Принятый
  0RTT не может утратить remembered support; rejected0RTT заменяет child maps.

5 Go +2 ORIGIN +4 docs;10 existing snapshots/1 new source. Manual6 existing
Windows diffs +1 new source; touched-only gofmt5 exit0.16 captured reviewed inputs,
0 dependency-cache reads;261 frozen cache originals/211 verbatim mirrors и prior
owned source сохранены. Public API/consumer/manifests/licences не менялись.
Tests/build/native/VPN/live не выполнялись. Receipts: .artifacts/memory/bootstrap-v451-stream-parameters-.

Остаются: old stream mutations shared connection-flow counters после Reset;
other public peerParams readers; partial flow/context/constructor custody;
actual retired rows/holders/global resources. Snapshot не является flow epoch
barrier или доказательством native/OS/fullPC приёмки.
Source/unverified: manual source/diff/byte review and touched-only gofmt do not prove compilation, legacy private fixtures, races, constructor/reset/close/0RTT/ACK/loss/panic/Goexit interleavings, wire/runtime/native correctness or speed/RSS/CPU/FD/Stop bounds. Public API, dependency versions, wire IDs and consumer unchanged; private flow-factory signature changed at its sole actual Conn binding and four constructor calls. Other peerParams readers and old stream mutations of the shared connection flow controller after Reset remain open, as do partial flow/context/constructor custody and actual row/resource retirement. All prior scalar/FIFO/update/control/input/map-call/six-class/send-data receipts retain separate scopes. No tests/fixtures/build/vet/typecheck/product/SDK/browser/native/admin/VPS/live/system network changes/generation/dependency/otherroles/chats/subagents. Owner 2026-11-13 explicitly authorizes a local scoped commit; no publication. Full PC00..09/old9/100 acceptance OPEN/56 checks NOT_RUN/72 tasks active; TUN unavailable/schema2 unsupported/rootNETGUARD001 remains OPEN;55% only rough/unmeasured.

<a id="pc-v452-stream-flow-epochs"></a>

## V2-452 — connection-flow epoch без переноса старых counters

Статус: source/unverified, 2026-11-13; quic-go revision54/sing-quic revision62.
Полная цель PC-00…09 и прежние обязательства сохраняются.

- Native NewConnectionFlowController публикует nonzero immutable epoch token
  до exposure. Stream flow хранит value scope с exact owner/token вместо общего
  изменяемого connection interface. Новый явный adapter allocation не добавлен;
  это не доказательство отсутствия heap allocations или RSS bound.
- AddBytesSent, SendWindowSize, AddBytesRead (включая Abandon),
  IncrementHighestReceived и EnsureMinimumWindowSize сравнивают token и читают/
  изменяют connection state в одной critical section под original base mutex.
  Successful0RTT Reset меняет token и send counters под этим же mutex. Нет
  предварительного check/use с последующим вызовом unscoped mutator.
- Stale scope возвращает zero send window/false connection-window credit;
  прочие connection mutations игнорируются. Late old highest-received не
  инициирует ошибку нового соединения. Stream-local state этим не очищается.
  Failed Reset после received/read data сохраняет прежний token/counters.
- Native promoted send counter/window/blocking methods теперь явно используют
  original mutex. Base IsNewlyBlocked вызывает base SendWindowSize напрямую,
  без recursive lock. Receive update bodies переиспользуются через locked helpers.
- Scope capture намеренно atomic/nonwaiting и не даёт counter access. Первичный
  план capture-under-mutex уточнён: existing AllowConnectionWindowIncrease
  вызывается под connection receive mutex и может открыть stream. Новый захват
  mutex из constructor добавил бы reentrant deadlock. Initial review receipts
  сохранены; final source использует atomic capture и locked mutation checks.
- Просмотрен actual порядок child CloseWithError/factory join → new maps/framer
  fence → connection Reset. Old factory заканчивается до Reset; uncertain close
  не даёт успешный переход. New outgoing wire limit закрыт до actual apply;
  incoming factories вызываются исходным Conn loop. Scope capture не заменяет
  этот lifecycle/admission порядок. Arbitrary private providers не приняты.

4 Go +2 ORIGIN +4 docs;9 existing snapshots/1 new source. Manual5 existing Windows
diffs +1 new source; touched-only gofmt4 exit0.13 captured reviewed inputs/
0 dependency-cache reads;261 cache originals/208 verbatim mirrors/prior owned
bytes сохранены. Public API/consumer/manifests/wire versions не менялись.
Source-only local commit3cd1417 сохранён по запросу владельца; планы с real server
data исключены, no publication. Receipts: .artifacts/memory/bootstrap-v452-stream-flow-epochs-; local commit receipt:
.artifacts/memory/pc-v451-local-commit-receipt.json.

Следующий P0 — partial flow/context/constructor custody и actual last-holder/
retained-row retirement; other peerParams readers тоже открыты. Extra send-path
locking cost требует будущего разрешённого измерения, максимум скорости не доказан.
Source/unverified: manual source/diff/byte review and touched-only gofmt do not prove compilation/private fixture compatibility, race/deadlock freedom, constructor/reset/close/0RTT/read/write/Abandon/auto-tuning/panic/Goexit interleavings, native or wire/runtime correctness, speed/RSS/CPU/FD/Stop bounds. Existing AllowConnectionWindowIncrease/logger callbacks still run under the original connection receive mutex; nonwaiting capture avoids adding a new constructor mutex edge but does not prove arbitrary callback reentrancy. Extra send-path locking and token/scope lifetime are unmeasured; no explicit separate adapter allocation is not an allocation/RSS guarantee. Stream-local flow state, original partial flow/context/constructor resources, actual last-holder/row retirement and other peerParams readers remain open. This fence does not join old operations or retire OS/native/global resources. Existing generation sender/framer/scalar/FIFO/update/control/input/map-call/six-class/send-data scopes stay separate. No tests/fixtures/build/vet/typecheck/product/SDK/browser/native/admin/VPS/live/system network changes/generation/dependency/otherroles/chats/subagents. Owner explicitly authorizes local scoped commits only; no publication. PC00..09/old9/100 acceptance OPEN/56 checks NOT_RUN/72tasks active; TUN unavailable/schema2 unsupported/rootNETGUARD001 OPEN;55% only rough/unmeasured.

<a id="pc-v453-stream-construction"></a>

## V2-453 — original partial-construction prefix

Статус: source/unverified, 2026-11-13; quic-go revision55/sing-quic revision63.
Полная цель PC-00…09 и прежние обязательства сохраняются.

- Раньше creation record сохранял только returned candidate. Flow controller,
  outer Stream, половины и sender wrappers могли существовать раньше возврата
  factory. Теперь исходный creation record содержит fixed construction prefix:
  parent/context inputs, immutable parameters, exact flow factory/result,
  Stream/SendStream/ReceiveStream и оба uniStreamSender с фазой создания.
- Все4 actual factories захватывают exact original child в local binding до
  exposure; completion по-прежнему связан с его Delete method. Нет lookup
  parent.current map после0RTT. Generic map factory signature не меняется;
  private stream constructors принимают original prefix во всех native callers.
- Flow result сохраняется до следующего constructor call. Outer Stream и sender
  wrappers удерживаются до создания половин. SendStream/ReceiveStream shell
  записывается до channel/context/sorter initialization. Возвращённые context/
  CancelCauseFunc сразу находятся на уже удержанном SendStream. Missing/reused
  prefix или nil flow дают original uncertain failure, без synthetic fallback.
- ReceiveStream сохраняет frameSorter shell до tree/map/insert initialization.
  Native init переиспользуется generic crypto/newFrameSorter и receive helper;
  payload/holder ledgers и nil-budget retirement semantics не заменены.
- Original factory gate защищает prefix; callbacks/allocations не переносятся
  под map metadata mutex. Abnormal factory или insertion сохраняют original
  prefix/row credits/sealed owner. Не вызывается full native Close у объекта с
  недостроенными полями, нет duplicate cleanup/constructor retry. Known insertion
  передаёт graph original map-owned stream; incoming batch prefix очищается
  только после успешной вставки, перед следующим ID. Outgoing whole prefix
  очищается прежним known-insert finishCreation.
- Ограничение: вызывающий код не может сохранить private allocation/cancel
  внутри opaque provider или context.WithCancelCause, который не вернулся.
  Известные parent/partial objects/phase удержаны; отмена/cleanup не объявляются
  подтверждёнными. Actual Conn creates native cancel context before preSetup;
  local Go context source inspected only, не доказательство shipped binary ABI.

9 Go +2 ORIGIN +4 docs;14 existing snapshots/1 new source. Manual10 existing
Windows diffs +1 new source; touched-only gofmt9 exit0.11 captured reviewed inputs,
0 cache source reads;261 cache originals/208 verbatim mirrors/prior owned bytes
сохранены. Public API/consumer/manifests/wire versions не менялись. No tests/
build/native/VPN/live/system network changes. Receipts: .artifacts/memory/bootstrap-v453-stream-construction-.

Следующие P0: actual last-holder/native/retained-row retirement, other peerParams
readers и hidden provider/context internals. Strong reference custody не равна
cleanup/join или физическому resource bound; весь PC-план остаётся активным.
Source/unverified: manual source/diff/byte review and touched-only gofmt do not prove compilation/private fixture compatibility, races, constructor/context/factory/insert/close/0RTT/panic/Goexit interleavings, native/wire/runtime correctness or speed/RSS/CPU/FD/Stop bounds. Private stream constructors require original prefix at all actual native callers; generic map factory and public/shutdown/consumer contracts remain unchanged. Only known returned objects/results are retained: arbitrary opaque provider allocations and context.WithCancelCause internal child/registration before return are not recovered by the caller. Existing parent context and phase remain held on unknown return; no substitute cancellation implementation or fabricated cleanup success. Actual Conn native context origin was inspected, without treating local stdlib source as shipped executable provenance. No extra worker/timer/global construction registry; this does not prove aggregate generations/whole API/resource bounds. Known map ownership is not actual last-holder/native/row retirement; all prior flow-epoch/framer/scalar/FIFO/update/control/input/map-call/six-class/send-data scopes stay separate. No tests/fixtures/build/vet/typecheck/product/SDK/browser/native/admin/VPS/live/system network changes/generation/dependency/otherroles/chats/subagents. Earlier local source commits3cd1417/a04b665 remain, no publication; historical server-data plans stay local. FullPC00..09/old9/100 acceptance OPEN/56 checks NOT_RUN/72tasks active; TUN unavailable/schema2 unsupported/rootNETGUARD001 OPEN;55% rough/unmeasured.

<a id="pc-v454-stream-shutdown"></a>

## V2-454 — native shutdown result перед освобождением строк

Статус: source/unverified, 2026-11-13; quic-go revision56/sing-quic revision64.
Полная цель PC-00…09 и прежние обязательства сохраняются.

- Выявлено препятствие к row retirement: void closeForShutdown скрывал отказ
  native admission и receive buffer cleanup failure. Send native close также не
  отменял original stream context явно при rejected0RTT. Close methods/private
  interfaces теперь возвращают error; actual providers/callers обновлены.
- Fixed per-half receipt started/returned/failure защищён original stream mutex.
  Known negative/unknown остаётся отрицательным, без cleanup retry. Original
  notification должна вернуться до установки returned; guard сбрасывает свой
  positive marker перед callback, чтобы panic/Goexit не стал успешным результатом.
- Send публикует shutdown cause, возвращает queued/next original frames, будит
  writer и вызывает exact original ctxCancel вне send mutex. После возврата
  повторно проверяется write uncertainty. Причина сохраняется и для0RTT reject;
  nil private cause нормализуется в net.ErrClosed. Active dataForWriting view
  остаётся у реального Write до его фактического возврата, без eager clearing.
- Receive передаёт retirement/discard error и помечает original data budget
  uncertain. Bidi после ordinary send error всё ещё пытается закрыть receive и
  возвращает combined result. Child map хранит exact closeFailure и original
  retiring stream/rows/credits; повторный close не повторяет unknown cleanup.
- Первичный узкий путь преобразовывал typed error в map panic. Scope расширен
  до parent/Conn: все4 ordinary child results соединяются; ResetFor0RTT при
  ошибке не создаёт новые maps и не допускает framer/flow reset. Terminal Conn
  сохраняет joined error в private close record, блокирует terminal write и
  оставляет original loop receipt отрицательным. Existing actual panic/Goexit
  по-прежнему unwind; fake recovery/cleanup worker не добавлены.
- В исходном map gate остаются native factories/control/close. Ни одна row
  здесь не удаляется и metadata credit не возвращается как native retirement.
  Typed successful close доказывает только завершённые synchronous steps по
  исходникам; public/map/ACK/history/borrower/flow/last-holder gates отдельные.

10 Go +2 ORIGIN +4 docs;15 existing snapshots/1 new source. Первичный scope8Go
уточнён до10Go после error-path review;2 ранее просмотренных source inputs
сохранены до правок, initial receipts не перезаписаны. Manual11 existing Windows
diffs +1 new source; touched-only gofmt10 exit0.15 captured reviewed inputs/
0 cache source reads;261 cache originals/208 verbatim mirrors/prior owned bytes
сохранены. Public API/consumer/manifests/wire versions не менялись. No tests/
build/native/VPN/live/system network changes. Receipts: .artifacts/memory/bootstrap-v454-stream-shutdown-.

Next P0: actual holder/retained-row retirement и native0RTT flow-reset failure
после успешного map-close этапа; other peerParams readers также открыты. Не
считать local typed receipt полным join или доказательством native/OS release.
Source/unverified: manual source/diff/byte review and touched-only gofmt do not prove compilation/private fixture compatibility, races/deadlocks, shutdown/cancel/notification/close/0RTT/panic/Goexit interleavings, native/wire/runtime correctness or speed/RSS/CPU/FD/Stop bounds. Private close interfaces/map/ResetFor0RTT/handleCloseError signatures changed at actual callers; public API/consumer/dependencies/wire version unchanged. Local synchronous cleanup receipt does not join active public/map/ACK/history/raw/flow/data holders or release rows/resources. Active Write view is not cleared; actual writer finish owns it. Native cancel is the original returned function, outside send mutex; arbitrary private cancellation/provider callbacks are not certified, factory gate remains original and no hard cancellation deadline is introduced. Original budget notification and receive pool release paths retain their existing callback/locking assumptions. Actual panics/Goexit still unwind as unknown; only ordinary error propagation becomes graceful. Failed0RTT cleanup leaves original maps fenced and no framer/flow reset. Later flow-reset failure after otherwise successful map replacement remains a separate transition gate. No row clearing/refund, worker/timer/new global registry or claimed GC/native resource retirement. Existing construction/epoch/framer/scalar/FIFO/update/control/input/map-call/six-class/send-data scopes stay separate. No tests/fixtures/build/vet/typecheck/product/SDK/browser/native/admin/VPS/live/system network changes/generation/dependency/Git mutations/otherroles/chats/subagents. Earlier local source commits remain/no publication; historical server-data plans stay local. FullPC00..09/old9/100 acceptance OPEN/56 checks NOT_RUN/72tasks active; TUN unavailable/schema2 unsupported/rootNETGUARD001 OPEN;55% rough/unmeasured.

<a id="pc-v455-stream-reset"></a>

## V2-455 — original0RTT reset prefix и отложенная публикация maps

Статус: source/unverified, 2026-11-14; quic-go revision57/sing-quic revision65.
Полная цель PC-00…09 и прежние обязательства сохраняются.

- До изменения parent заменял карты/framer до connFlow.Reset, который мог
  отказать; parent mutex также удерживался через native child shutdown/cancel.
  Теперь admission закрывается короткой metadata section, а original4 child
  close и exact bound native flow Reset выполняются вне parent metadata lock.
- Один fixed original reset record хранит old/candidate sets, actual flow Reset,
  stage/ended/completed/uncertain/failure. Flow refusal возвращается до создания
  новых maps. Повторный вызов использует исходный result, не повторяет операции.
  Known ordinary refusal и actual unknown unwind не объявляются одним receipt.
- Shared native builder заполняет original candidate shell до публикации,
  сохраняя returned child pointers перед следующими constructors. Initial setup
  использует тот же builder. All4 factory closures продолжают ссылаться на exact
  original child prefix/Delete, immutable generation parameter/flow contracts.
  Current parent child refs не перезаписываются частично при failed build.
- Framer reset требует original/distinct next generations, привязанные к этому
  framer. Under existing control→data locks проверяется current original token,
  sealed/retired state; затем очищаются прежние data/control queues и меняется
  token. Silent sealed return заменён explicit error. Это не full native join.
- Parent map publication выполняется после known close/flow/build/framer phases
  и close/identity recheck. Pending/failed UseResetMaps не снимает admission gate;
  actual NextConnection передаёт ошибку. Обычный закрывающий CAS всё ещё может
  прийти между фазами: retained prefix сохраняет обе стороны, native counters
  не откатываются, новая попытка поверх неизвестного результата не запускается.
- Terminal Close берёт current set под parent mutex, затем закрывает children
  вне него и включает pending/unknown reset failure. Нормальный отказ перехода
  отделён от unknown native cleanup; прежние typed child/Conn receipts сохранены.
- Old/candidate pointers остаются strong references до отдельного actual terminal
  metadata/last-holder retirement. Это один reset attempt, без растущего списка
  generations. Ни одна row/credit здесь не очищается как native/GC/RSS release.

4 Go +2 ORIGIN +4 docs;9 existing snapshots/1 new source. Manual5 existing Windows
diffs +1 new source; touched-only gofmt4 exit0.14 captured reviewed inputs/
0 cache source reads;261 cache originals/208 verbatim mirrors/prior owned bytes
сохранены. Public API/consumer/manifests/wire versions не менялись. No tests/
build/native/VPN/live/system network changes. Receipts: .artifacts/memory/bootstrap-v455-stream-reset-.

Next P0: actual terminal retirement current/original/candidate sets после
original map/public/data/ACK/borrower receipts; other peerParams readers также
открыты. Initial Conn/preSetup partial construction — отдельный незакрытый scope.
Source/unverified: manual source/diff/byte review and touched-only gofmt do not prove compilation/private fixture compatibility, race/deadlock freedom, close/reset/flow/build/framer/commit/handshake/CAS/panic/Goexit interleavings, native/wire/runtime correctness or speed/RSS/CPU/FD/Stop bounds. Private ResetFor0RTT/UseResetMaps/framer reset signatures and actual callers changed; public API/consumer/dependency/wire version unchanged. Actual native flow Reset is bound once and executes outside parent/framer locks; arbitrary opaque private providers are not certified. No new callback, worker/timer/global owner or hard cancellation deadline. Parent publication is gated, but native flow/token mutation is not an all-or-nothing rollback transaction. A later close or candidate/framer failure retains old/candidate/stage and closes the connection; no native counters are restored or same attempt retried. Original/candidate refs remain strong, bounded to one reset attempt rather than an unlimited generations list; actual metadata/last-holder/native/GC/RSS retirement is still separate. Initial Conn/preSetup partial construction is not solved by a retained reset candidate shell. Normal refused transition stays distinct from unknown cleanup: terminal Close includes pending/unknown reset failure, and local synchronous results retain earlier scopes. All prior construction/shutdown/epoch/scalar/FIFO/update/control/input/map-call/six-class/send-data contracts remain. No tests/fixtures/build/vet/typecheck/product/SDK/browser/native/admin/VPS/live/system network changes/generation/dependency/Git mutations/otherroles/chats/subagents. FullPC00..09/old9/100 acceptance OPEN/56 checks NOT_RUN/72tasks active; TUN unavailable/schema2 unsupported/rootNETGUARD001 OPEN;55% rough/unmeasured.

<a id="pc-v456-map-retirement"></a>

## V2-456 — external aggregate и metadata retirement current/old/candidate

Статус: source/unverified, 2026-11-14; quic-go revision58/sing-quic revision66.
Полная цель PC-00…09 и прежние обязательства сохраняются.

- Новый additive Conn.WaitForStreamMapRetirement вызывается внешним owner после
  закрытия. Он соединяет original packet input, map Open/Accept, receive calls/
  data и send data; последний уже соединяет все6 stream-call classes/framer.
  Старые receipt APIs сохраняют свою область, их реализации не дублируются.
- Одна original parent-finalizer record содержит done/result/stage/index и fixed3
  pointer sets. Второй caller ждёт этот же результат вне metadata locks.
  Negative/unknown не очищает original owner и не запускает новую попытку.
- После positive prerequisites снимаются current/original/candidate pointers и
  проверяются known reset end/identity. Repeated groups пропускаются. Current/old
  children требуют positive native close, отсутствия factory/update/control
  actors/uncertainty и корректного row accounting. Все groups preflighted до clear.
- Fully built, но не опубликованный candidate отдельно проверяется на zero rows,
  reservations/waiters/control state. Только он может выполнить original empty
  map close после окончания loop: stream native callback здесь не вызывается.
  Partial/unknown candidate не sweep-ится, никакой positive receipt не выдуман.
- Child metadata clear под его mutex не вызывает native stream/pool callbacks:
  drops rows и logical metadata credits, factory/queue/construction/update/control
  aliases. Original flags/receipt guards отличают known completion от unknown;
  incomplete clear остаётся retained/negative без retry. Admission уже sealed.
- Parent после всех known child clears освобождает current/reset set pointers,
  context/factory/parameter inputs. Fixed finalizer prefix очищается только при
  normal positive завершении. Immutable generation bindings и сами Stream/Context
  objects не мутируются; public StreamID/Context держат прежние objects.
- Actual HY2 attachQUIC close task и native retireTransportConn используют новый
  aggregate вместо повторённой композиции waits. Source sites просмотрены:
  transport external close task и пути после actual conn.run return, включая
  failed opening/rejected server candidate. HY2 starts independent aborts перед
  joins. Новый worker/timer/self-join из registered/native operation не добавлен.

7 Go +2 ORIGIN +4 docs;11 existing snapshots/2 new sources. Manual7 existing
Windows diffs +2 new sources; touched-only gofmt7 exit0.18 captured reviewed
inputs/0 cache source reads;261 cache originals/208 verbatim mirrors/prior owned
bytes сохранены. Consumer изменён вместе с additive quic-go API; зависимости/
manifest/wire versions не менялись. No tests/build/native/VPN/live/system network
changes. Receipts: .artifacts/memory/bootstrap-v456-map-retirement-.

Это metadata-reference retirement, не physical/native/OS/GC/RSS release и не
whole QUIC/ACK/crypto proof. Next P0 — other public peerParams readers и
initial Conn/preSetup partial ownership, затем raw/control-ACK/crypto metadata
producer/holder scopes; full PC-план активен.
Source/unverified: manual source/diff/byte review and touched-only gofmt do not prove compilation/private fixture compatibility, races/deadlocks, concurrent-finalizer/close/reset/input/call/data/preflight/clear/panic/Goexit interleavings, native/wire/runtime correctness or speed/RSS/CPU/FD/Stop bounds. New additive external WaitForStreamMapRetirement API is consumed by the paired HY2 source and native Transport; original receipt API scopes/dependencies/wire versions remain. The aggregate must not be called from its own registered/native callback; actual production sites are external owners or after original conn.run return. No new worker/timer, forced cancellation or global native bound. Unpublished candidate close is allowed only after known reset end and zero-row/zero-admission/control state; it performs no stream native callback after loop end. Preflight/mutation drops metadata refs/logical row credits only; raw/control-ACK/history/crypto and externally held Stream/Context objects remain independent, immutable generation bindings are not changed and GC/OS/native physical release is not inferred. A partial metadata clear may stay negative without retry; its original fixed pointer prefix remains retained. Active Write view and stream-local fields are never cleared by this finalizer. Initial Conn/preSetup partial construction, other public peerParams readers, raw/ACK/crypto metadata, global/kernel/OS/UX/release/full-PC gates remain open. No tests/fixtures/build/vet/typecheck/product/SDK/browser/native/admin/VPS/live/system network changes/generation/dependency/Git mutations/otherroles/chats/subagents. FullPC00..09/old9/100 acceptance OPEN/56 checks NOT_RUN/72tasks active; TUN unavailable/schema2 unsupported/rootNETGUARD001 OPEN;55% rough/unmeasured.

<a id="pc-v457-peer-parameters"></a>

## V2-457 — безопасное чтение peer-параметров и повторная проверка DATAGRAM

Статус: source/unverified, 2026-11-14; quic-go revision59/sing-quic revision67.
Полная цель PC-00…09 и прежние обязательства сохраняются.

- Устранены прямые public-чтения native peerParams из ConnectionState,
  sendDatagram и AddPath. Два atomic immutable scalar snapshots содержат только
  phase, remote DATAGRAM limit/derived payload, partial-reset и migration flags;
  aliases wire objects, адресов, токенов и ключей не передаются.
- Observed head хранит успешно восстановленные либо validated received факты.
  Outbound head хранит успешно восстановленные параметры до early readiness,
  затем только actual applied fresh параметры. Client не использует новые
  outbound limits раньше окончания handshake; server применяет их прежним путём.
  Nil inputs отказывают до обращения к параметрам; malformed received не публикуется.
- Rejected0RTT публикуется до history DropPackets и прежнего stream-map reset.
  Новые DATAGRAM admissions/history transfers возвращают Err0RTTRejected или
  отказывают в передаче; свежий received сам по себе этот запрет не снимает.
  Fresh successful apply заменяет outbound head. Existing native 0RTT accepted
  limit/reset downgrade validation сохранена без изменений.
- Actual preSetup однократно связывает original queue с outbound atomic head.
  Publisher и reserve/enqueue/Peek/Pop-transfer используют один sendMx: изменение
  между ранней проверкой размера и финальной передачей не проходит незамеченным.
  Lock order queue→scope сохранён. Очередь читает immutable entry.size; mutable
  frame flags/data, custom Context и callbacks под этим lock не читаются/не вызываются.
- Head-change close-and-replace channel будит всех waiting producers, включая
  заблокированных полной очередью. После wake повторяются original close/scope/
  deadline/peer gates. Scheduling выполняется после unlock; новых tasks/timers нет.
- Peek сохраняет unsent queue owners при отсутствующем/rejected head. Fresh apply
  будит packet loop; запрещённые/слишком большие entries теряют только original
  queue reference через прежний releaseEntryLocked, а подходящие могут перейти
  в 1RTT. Pop повторно проверяет peer head перед history transfer; отказ сохраняет
  queue owner и original Peek borrower. Уже переданный history не переотправляется.
- Exact remote limit включает type+LEN+payload нашего queued encoding; проверены
  по исходникам empty payload и ширины varint1/2/4/8, без арифметического overflow.
  Это не path MTU proof: прежняя conservative estimate и packetizer fit остаются.
  Local receive enablement не добавлен к send gate: peer support unidirectional.
  ReceiveDatagram/parser/local advertisement и ACK/loss/Close custody не изменены.
- Оставшиеся plain peerParams uses — native restore/receive/apply/validation и
  path MTU внутри Conn-loop. Existing TLS/provider/config/path APIs сохраняют
  собственные контракты; snapshots не дают trust/protection/whole-connection authority.

Нормативный источник: [RFC 9221 §§3–5](https://www.rfc-editor.org/rfc/rfc9221.html#section-3),
прочитан 2026-11-14. Он задаёт односторонний receive limit и размер полного frame;
это основание реализации, не подтверждение protocol conformance.

3 Go +2 ORIGIN +4 docs;8 existing snapshots/1 new source. Manual4 existing
Windows diffs +1 new source; touched-only gofmt3 exit0.13 captured reviewed
inputs/0 cache source reads;261 cache originals/208 verbatim mirrors/prior owned
bytes сохранены. Paired consumer unchanged; public APIs/dependency/wire versions
сохранены. No tests/build/native/VPN/live/system network changes. Receipts: .artifacts/memory/bootstrap-v457-peer-parameters-.

Next P0 — initial Conn/preSetup partial ownership, затем raw/control-ACK/crypto
producer/holder retirement. Приёмка текущего среза и всего PC-плана открыта.
Source/unverified: manual source/diff/byte review and touched-only gofmt do not prove compilation/private fixture compatibility, races/deadlocks, restore/receive/apply/rejection/admission/Peek/Pop/Close interleavings, wire interoperability or speed/RSS/CPU/FD/Stop bounds. Two small scalar heads report observed versus actually applied peer facts, not TLS/path/trust/protection authority. Mutable native peerParams remains Conn-loop-only for validation/application/path MTU. Actual preSetup binds queue to original outbound atomic head; privately unbound generic queue retains old AddCopy seam. No local receive flag is imposed on send: peer receive support is unidirectional per RFC9221 section3. Exact limit includes our type+LEN+payload encoding, even empty payload; MTU/packet fit remain separate existing conservative and packetizer decisions, not a global path-size guarantee. Rejected0RTT prevents new admissions/transfers; already queued unsent data may be considered under fresh applied parameters, with no retransmission/replay of prior history added. Queue owns only its original credit; Peek borrower/history/packet custody are not revoked or inferred retired. Waiting producers wake on a broadcast head change then recheck original admission/deadline/close gates; no new worker/timer/foreign context callback under metadata locks. Initial Conn/preSetup partial ownership, raw/control-ACK/history/crypto producers/holders, whole SDK/global/native/OS/UX/release/comparative gates remain open. No tests/fixtures/build/vet/typecheck/product/SDK/browser/native/admin/VPS/live/system network changes/generation/dependency/Git mutations/otherroles/chats/subagents. FullPC00..09/old9/100 acceptance OPEN/56 checks NOT_RUN/72tasks active; TUN unavailable/schema2 unsupported/rootNETGUARD001 OPEN;55% rough/unmeasured.

<a id="pc-v458-connection-construction"></a>

## V2-458 — исходный владелец конструктора Conn и частичных объектов

Статус: source/unverified, 2026-11-14; quic-go revision60/sing-quic revision68.
Полная цель PC-00…09 и прежние обязательства сохраняются.

- Server Conn сохраняется в original opening до создания oneRTT crypto-stream:
  allocation удалена из field initializer. Client использует тот же original
  oneRTT stream в cryptoStreamManager и в поле Conn вместо отдельного local-only
  pointer. Initial/handshake/oneRTT shells удерживаются до sorter/tree init.
  Client session-ticket sending не добавлено; server branch остаётся прежним.
- Новый fixed connectionConstruction хранит stage, parent context/TLS/runner/
  generator, returned addresses/transport parameters/token и partial crypto setup.
  Его меняет только исходный constructor до exposure. External opening guards
  уже удерживают Conn и исходный вызов; новый второй owner/worker не создаётся.
  Возвращённый TokenStore результат сохранён до SetToken/RTT, параметры — до qlog.
- Actual initial ACK constructor получает &Conn.sentPacketHandler. Original
  handler shell хранится в этом typed interface slot до создания congestion и
  histories. Cubic shell сначала сохраняется в original handler.congestion,
  затем создаёт cubic/pacer и вызывает прежний qlog event. Порядок событий,
  алгоритм Reno/Cubic, windows/packet numbering/ECN параметры сохраняются.
- Actual TLS client/server используют retained CryptoSetup slot в construction
  prefix. Native shell и returned initial AEAD pair сохранены до qlog callbacks;
  TLS clone/QUICConn результаты — в том же shell до последующей работы. Native
  setup/keys не закрываются как fully initialized при abnormal constructor return.
  Старые NewCryptoSetup/NewSentPacketHandler/NewCubicSender wrappers сохранены,
  делегируют одному initializer; новые API находятся только в internal packages.
- Parent streamsMap shell сохраняется на Conn до init. Fixed initialMaps set
  удерживает generation и четыре child shells до их map/channel initialization.
  Каждый factory сохраняет exact original child binding. После полного build
  pointers передаются в current fields и temporary set очищается. Existing reset
  candidate использует этот же buildMaps и теперь также держит partial child shells.
  Ни stream/native shutdown, ни allocation-failure cleanup из partial map не вызван.
- finishConstruction проверяет наличие returned graph/current complete maps и
  отсутствие pending initialMaps, затем очищает transient prefix и ставит complete.
  Actual client constructor result проверяется до run/cleanup и повторно под
  publication fence; server проверяет exact opening identity/completion до первой
  передачи пакета и map admission. Partial/mismatched result остаётся у исходного
  negative opening guard; не запускает normal Conn.run/Close/retirement.
- Constructor inputs и retained slots — custody, не runtime-ready/protection
  authority. Оpaque provider internals до return, fatal allocation failure,
  legacy private seams/mocks и active Retry/congestion replacement остаются
  отдельными scope. Existing peer snapshots/0RTT phases/map retirement unchanged.

11 Go +2 ORIGIN +4 docs;16 existing snapshots/1 new source. Manual12 existing
Windows diffs +1 new source; touched-only gofmt11 exit0. После ручного просмотра
уточнён stale server-only comment поля oneRTT и повторно отформатирован только
этот файл; initial receipts сохранены отдельно.19 captured reviewed inputs/
0 cache source reads;261 cache originals/205 verbatim mirrors/prior owned bytes
сохранены. Paired consumer unchanged; dependency/wire/public Conn APIs сохранены.
No tests/build/native/VPN/live/system network changes. Receipts: .artifacts/memory/bootstrap-v458-connection-construction-.

Next P0 — active ACK Retry/congestion replacement и raw/control-ACK/history/crypto
producer/holder retirement. Полная приёмка текущего среза и PC-плана открыта.
Source/unverified: manual source/diff/byte review and touched-only gofmt do not prove compilation/private fixture compatibility, races/deadlocks, custom qlog/context/address/token/TLS/provider panic/Goexit/nonreturn interleavings, wire compatibility or speed/RSS/CPU/FD/Stop bounds. Original construction prefix and typed slots preserve known shell/input/returned child references, not successful initialization, cleanup or physical/native resource release. Incomplete Conn must not enter its normal run/Close/retirement APIs; actual client/server promotion checks complete and exact opening identity before native use. Unknown original opening remains sealed/negative with retained source graph, no retry/partial native Close/sweep. Complete marker is original constructor metadata written before exposure, not public concurrent protection state; successful transfer clears transient constructor aliases while actual Conn/map/crypto fields retain original objects. Old constructor wrappers remain compatibility seams over one initializer but their stack-local slots do not claim caller custody after abnormal return; actual initial production Conn uses retained slots. Opaque helper/provider internals before return, fatal allocation failure, legacy private mocks/initializers and active Retry/congestion replacement need separate review. Existing crypto/scrambling/config/stream-generation/0RTT/1RTT/wire/dependency/public Conn contracts are not replaced; setting client oneRTT field only retains its original manager stream and does not add client session-ticket output. Ongoing raw/control-ACK/history/crypto producers/holders and aggregate metadata/native/GC/RSS/global/OS/UX/release/comparative gates remain open. No tests/fixtures/build/vet/typecheck/product/SDK/browser/native/admin/VPS/live/system network changes/generation/dependency/Git mutations/otherroles/chats/subagents. FullPC00..09/old9/100 acceptance OPEN/56 checks NOT_RUN/72tasks active; TUN unavailable/schema2 unsupported/rootNETGUARD001 OPEN;55% rough/unmeasured.

<a id="pc-v459-packet-transitions"></a>

## V2-459 — явные native packet transitions и публикация congestion

Статус: source/unverified, 2026-11-14; quic-go revision61/sing-quic revision69.
Полная цель PC-00…09 и прежние обязательства сохраняются.

- Исправлено предположение следующего шага: ResetForRetry пересоздаёт Initial и
  application packet-number spaces; новый congestion object создаёт MigratedPath.
  Эти native transitions получили отдельные kind/stage в одном original fixed
  prefix. До изменения history сохраняются старые spaces, текущий packet, прежние
  bytes/PTO/alarm. Новые space shells сохраняются до init и publication.
- Preflight отвергает sticky history failure, pending/admitting/pendingProbe,
  некорректный outstanding count и противоречивые Retry probes. Original packet
  сохраняется до DeclareLost/loss callback/RemovePathProbe; unknown callback не
  теряет original frames. Negative/abnormal transition держит source prefix и
  sticky historyFailure, не запускает повторную попытку или invented rollback.
- При migration прежний range по PathProbes удалял строки из compacted slice и
  мог пропускать следующую. Теперь берётся текущая первая строка, original probe
  сохраняется до удаления, nil/mismatch дают typed refusal. Новые протокольные
  retransmissions для probe не добавлены. Full frame/history retirement отдельно.
- Одна congestionChange cell допускает native migration или public setter без
  blocking wait. Она удерживает previous/source/candidate; rejected overlap не
  запускает foreign setup. Missing/typed-nil controller, failed/closing state
  отвергаются. Candidate adapter хранится до SetRTTStatsProvider, Cubic native
  candidate — через retained constructor до qlog. Callbacks идут вне metadata lock.
- Publication shared с getter/setter mutex и разрешена только после normal setup
  return. Actual Closing channel связан до initial ACK callbacks через compatible
  optional argument; admission и publication повторно проверяют его. Complete
  transition очищает temporary aliases; failure/unknown сохраняет original cell.
- Internal ResetForRetry/MigratedPath/SetCongestionControl теперь возвращают error.
  Actual Retry останавливает дальнейшее CID/token применение; server migration
  передаёт error; client path-switch сохраняет original prefix и negative receipt.
  Public Conn.SetCongestionControl сохраняет void signature: отказ, включая
  overlap, запрашивает original connection close; abnormal callback делает то же
  через defer, сохраняя прежнее panic/Goexit поведение. Silent applied не выдуман.
- Native-loop-only PacketStateError читает original history/reset и mutex-guarded
  active/failed congestion state. Conn.run не публикует positive loop receipt при
  таком состоянии. Это deny-only status, не join/cancel каждого public provider;
  original Closing предотвращает новые production admissions после close.

6 Go +2 ORIGIN +4 docs;10 existing snapshots/2 new sources. Manual6 existing
Windows diffs +2 new sources; touched-only gofmt6 exit0. Ручной review уточнил
pendingProbe/nil-probe guards; initial receipts сохранены, повторно formatted
только2 изменённых файла.14 captured reviewed inputs/0 cache source reads;
261 cache originals/205 verbatim mirrors/prior owned bytes сохранены. Paired
consumer unchanged; public Conn API/dependency/wire versions сохранены. Private
interface/mock compatibility не подтверждена. No tests/build/native/VPN/live/
system network changes. Receipts: .artifacts/memory/bootstrap-v459-packet-transitions-.

Next P0 — custom congestion public-call custody/join и raw/control-ACK/history/
crypto producer/holder metadata retirement. Приёмка PC-плана остаётся открытой.
Source/unverified: manual source/diff/byte review and touched-only gofmt do not prove compilation/private mock or fixture compatibility, races/deadlocks, loss/RTT/qlog/public setter/Close/panic/Goexit/nonreturn interleavings, protocol conformance or speed/RSS/CPU/FD/Stop bounds. Retry resets packet-number histories; native path migration resets congestion. Existing callbacks/RTT floor/Reno/windows and packet retransmission policy remain; probe iteration now drains current first rows rather than skipping compacted rows. Fixed prefixes retain known old/candidate/current references and sticky negative state, not rollback/native cleanup or physical resource release. Public void SetCongestionControl retains its signature but refused nil/typed-nil/overlapping/failed/closing and abnormal setup requests original connection close; overlapping calls are not silently queued or claimed applied. Foreign setup/qlog callbacks run outside metadata locks, but this does not serialize every custom congestion method, prove same-object/provider alias safety, cancel nonreturning callbacks or join all external calls. Actual Closing is bound before constructor callbacks and checked at change admission/publication; PacketStateError is native-loop/terminal-only, includes active/failed changes, and can only deny a positive receipt. It is not a public-call completion/join receipt, global shutdown proof or resource bound. Internal SentPacketHandler signatures gained typed errors/new native status; original public Conn signature and old constructor wrappers remain. Private unbound constructor/mocks require separate compatibility acceptance. Opaque factory internals, whole ACK/control/history/raw/crypto producer/holder metadata retirement, custom controller call custody, global/native/OS/UX/release/comparative gates remain open. No tests/fixtures/build/vet/typecheck/product/SDK/browser/native/admin/VPS/live/system network changes/generation/dependency/Git mutations/otherroles/chats/subagents. FullPC00..09/old9/100 acceptance OPEN/56 checks NOT_RUN/72tasks active; TUN unavailable/schema2 unsupported/rootNETGUARD001 OPEN;55% rough/unmeasured.

<a id="pc-v460-congestion-calls"></a>

## V2-460 — original setter call и external congestion shutdown receipt

Статус: source/unverified, 2026-11-14; quic-go revision62/sing-quic revision70.
Полная цель PC-00…09 и прежние обязательства сохраняются.

- Новый connectionCongestionCalls — одна fixed original Conn cell, создаваемая
  до exposure. В ней controller, immutable handler, owned stage/result и guard
  complete-return. Begin проверяет original first-close atomic/Closing, seal,
  failed/active и handler до входа в native setter. Первый close CAS seal-ит
  registry; duplicate/reentrant call не создаёт worker/очередь и не выдаёт applied.
- Последний deferred finish выполняется после normal/abnormal outer close request.
  Registry mutex не удерживается при provider/close/error methods/waits. Known
  ordinary API return удаляет temporary call refs; unknown return сохраняет
  original input/handler/stage/result, seal и negative receipt без retry. Все
  external observers ждут один original done channel вне metadata locks.
- Additive Conn.WaitForCongestionControlShutdown ждёт Context, затем original
  packet-input/native-loop receipt, registered public setter и ACK congestion
  state. Ручной review уточнил: parent Context cancellation не доказывает actual
  loop return, поэтому простое ожидание Context.Done не используется как proof.
- PacketStateError теперь сообщает только native history/reset producer state.
  CongestionStateError отдельно читает retained active/failed ACK change под его
  mutex. Native loop не join-ит внешний setter и не смешивает его completion со
  своим producer receipt. Existing stream/map/input APIs сохраняют прежний scope.
- Exact net.ErrClosed после normal setup return при original Closing и ещё не
  опубликованном candidate очищает только temporary change aliases: previous
  остаётся current, applied не выдаётся. Unknown/other failed change остаётся
  retained/negative. Это различие API failure и unknown lifetime, не rollback и
  не закрытие произвольных ресурсов custom controller.
- Actual retireTransportConn и HY2 QUIC close task после original close ждут
  новый receipt до stream-map/path metadata retirement. Negative не удаляет owner
  и не запускает повторное cleanup. HY2 по-прежнему запускает independent raw/
  QUIC/Transport/opening/HTTP3 aborts до waits. Native call sites после original
  conn.run или external handler.destroy просмотрены; новый worker не добавлен.
- Receipt external-only: нельзя вызывать из controller callback/операции, которую
  он join-ит, включая opaque callback с synchronous Transport.Close. Generic
  self-join/nonreturn/hidden tasks не объявлены решёнными. Actual HY2 Brutal/meta2
  SetRTTStatsProvider source только присваивает provider; это не runtime proof
  и не гарантия поведения всех пользовательских congestion implementations.

6 Go +2 ORIGIN +4 docs;11 existing snapshots/1 new source. Manual7 existing
Windows diffs +1 new source; touched-only gofmt6 exit0. Source review добавил
native packet receipt и known Closing-refusal distinction; initial source/
format/reviewed receipts сохранены, повторно formatted только changed files.
15 captured reviewed inputs/0 cache source reads;261 cache originals/205 verbatim
mirrors/prior owned bytes сохранены. Actual paired consumer изменён вместе с
additive method; dependencies/wire unchanged. Private interface/mock compatibility
не подтверждена. No tests/build/native/VPN/live/system network changes.
Receipts: .artifacts/memory/bootstrap-v460-congestion-calls-.

Next P0 — terminal ACK/control-frame/history/crypto producer/holder metadata
retirement после actual receipts. Полная приёмка PC-плана остаётся открытой.
Source/unverified: manual source/diff/byte review and touched-only gofmt do not prove compilation/private mock/fixture compatibility, races/deadlocks, callback reentry/Context-parent cancellation/Close/ordinary refusal/panic/Goexit/nonreturn interleavings, interoperability or speed/RSS/CPU/FD/Stop bounds. One original public setter cell bounds registered call metadata per Conn, not all API calls/process memory/native work. It retains input/handler/stage/result before actual setup and finishes after the outer close-request/defer path. Known ordinary API error can be quiescent without being applied; only exact net.ErrClosed after returned setup, unpublished candidate and original Closing clears temporary ACK change aliases. Unknown/other failed state remains retained/negative, no retry/rollback/native Close inferred. New additive external WaitForCongestionControlShutdown first joins original packet-input/native-loop receipt because Context cancellation alone is insufficient, then registered public caller and retained ACK congestion status. Native PacketStateError now covers native history/reset only; external setter completion is not folded into that producer receipt. Existing component receipt scopes remain, and actual Transport/HY2 join stronger receipt before map/path metadata release. The receipt must never be called from its own controller callback or a call it joins, including custom callback synchronously joining Transport.Close; generic self-join/nonreturn is not solved or forcibly cancelled. Inspected actual HY2 Brutal/meta2 provider setters assign the provider only; this is source evidence, not proof of all controller behaviors or native execution. Hidden tasks/provider aliasing/all custom methods/whole ACK/control/history/raw/crypto retirement and global/native/GC/RSS/OS/UX/release/comparative acceptance remain open. No tests/fixtures/build/vet/typecheck/product/SDK/browser/native/admin/VPS/live/system network changes/generation/dependency/Git mutations/otherroles/chats/subagents. FullPC00..09/old9/100 acceptance OPEN/56 checks NOT_RUN/72tasks active; TUN unavailable/schema2 unsupported/rootNETGUARD001 OPEN;55% rough/unmeasured.

<a id="pc-v461-datagram-retirement"></a>

## V2-461 — original DATAGRAM data ledger и shutdown receipt

Статус: source/unverified, 2026-11-16; quic-go revision63/sing-quic revision71.
Полная цель PC-00…09 и прежние обязательства сохраняются.

### Причина и граница среза

Terminal ACK metadata нельзя безопасно использовать как реестр владельцев данных:
STREAM budget уже возвращает свои original loans после quiescence, и frame aliases
в ACK могут указывать на повторно использованный pooled frame. У DATAGRAM до этого
были count/byte credit и queue/history refs, но не было отдельного live-entry
ledger и producer completion receipt. Сначала добавлен этот prerequisite; ACK
metadata и его stale physical slots не очищаются в данном срезе.

### Реализация

- Existing send budget4096 owners/8MiB дополнен bounded live map и original
  producer count/channel. Entry shell и credit резервируются под sendMx до clone.
  Lock order sendMx→scope.mu сохранён; copy/custom Context/scheduling/timer work
  и ожидание producer выполняются без metadata locks. Используется уже захваченный
  done channel: helper не вызывает custom Context.Done повторно.
- Producer держит собственную reference до последнего defer после возврата
  helper и его timer cleanup. Enqueue выдаёт отдельную queue reference; Pop
  переносит её в history, Peek имеет independent borrower. Close/ACK/loss/unsent
  могут вернуть только свою reference и не освобождают active producer backing.
  Ordinary returned failure возвращает producer credit; abnormal return сохраняет
  exact original entry/ref/data/count и negative state без retry.
- Queue Close закрывает admission, освобождает queued/receive owners и будит
  original producer receipt при zero producers или poisoned negative result.
  Scope и полное завершение публичного Send этим не подтверждаются.
- Additive external Conn.WaitForDatagramSendDataShutdown сначала ждёт actual
  WaitForPacketInputShutdown, затем admitted owned-data producers. Context.Done
  сам по себе не считается loop completion. Один original finalizer/result/done
  используется всеми concurrent observers; ожидание вне sendMx.
- До sweep проверяются closed empty queue, непротиворечивый live count, точная
  сумма bytes и каждый original entry: queue identity, producer finished,
  history=true, refs=1, immutable size и фактический payload. Bounded snapshot
  не копирует данные. Только после полного preflight возвращаются history credit
  и owned payload. Нет fake ACK/loss/retransmission, user callback, pool call
  или dereference pooled STREAM/ACK scratch. Unknown/failed result сохраняет
  оригинальные владельцы и snapshot/current; повторного finalizer нет.
- Actual retireTransportConn и existing HY2 QUIC close task ждут новый receipt
  после congestion и до stream-map/path metadata retirement. Independent raw/
  QUIC/Transport aborts всё так же начинаются до waits. Новый task/worker/timer,
  wire/dependency version и системные сетевые изменения не добавлены.

### Доказательство и остаток

Ручной просмотр6 existing Windows diffs и1 new source, включая обе ORIGIN;
touched-only gofmt5 exit0.17 captured reviewed inputs/0 cache source reads;
261 cache originals/205 verbatim mirrors и prior owned bytes сохранены.
Это source evidence; compilation/native/runtime acceptance не выполнялась.
Receipts: .artifacts/memory/bootstrap-v461-datagram-retirement-.

Next P0 — terminal ACK/control/history/crypto metadata after actual data/call
receipts, без доступа к recycled frame aliases и без fake delivery callbacks.
Source/unverified: manual source/diff/byte inspection and touched-only gofmt do not prove compilation, private fixture/mock compatibility, races/deadlocks, panic/Goexit/nonreturn/cancellation/close/ACK/loss interleavings, interoperability, speed, resource usage or native cleanup. The existing4096-owner/8MiB limit bounds logical admitted send entries/payload bytes, not all callers, CPU, GC/RSS or process memory. Original producer reference covers owned clone/enqueue/context/scheduling/timer work only; pre-admission Context methods/caller input, complete Send/scope/public-call completion and hidden tasks remain separate. A nonreturning producer is retained and not forcibly cancelled. External WaitForDatagramSendDataShutdown must not be invoked by a producer callback it joins. Positive packet-input/native-loop and admitted-producer receipts are required before complete bounded history-data preflight. Context cancellation alone is insufficient. Negative native or producer result, unexpected borrower/history/reference/count/byte state and abnormal finalizer unwind retain originals and one negative result, without speculative sweep or retry. Retirement invokes no ACK/loss/retransmission/user/pool callbacks and never dereferences pooled STREAM aliases or ACK scratch. ACK/control/history/crypto metadata, raw providers and full native/OS/global retirement remain open. Actual Transport/HY2 integration changes only the existing close task ordering; independent aborts still precede waits. No tests/fixtures/build/vet/typecheck/product/SDK/harness/browser/native/admin/live/VPS/network/dependency/generation/Git/other-role/chat/subagent actions. Existing commits retained. Full PC/old nine stages active;100 acceptance OPEN,56 checks NOT_RUN,72 tasks; TUN unavailable, schema2 install unsupported, root NETGUARD-001 OPEN.

<a id="pc-v462-packet-history"></a>

## V2-462 — завершение владения sent-history metadata

Статус: source/unverified, 2026-11-16; quic-go64/sing-quic72.
Полная цель PC-00…09 и прежние обязательства сохраняются.

- В original sent-handler до внешних constructor callbacks создаётся один
  finalizer/result/done. Он хранит fixed3 current spaces, stage/index и negative
  prefix при ошибке. Concurrent observers ждут один результат вне mutex;
  ожиданий, provider/error callbacks и pool operations под metadata mutex нет.
- Новый external Conn.WaitForPacketHistoryRetirement сначала соединяет original
  congestion/native-loop, DATAGRAM data и stream-map aggregate receipts. Последний
  включает STREAM data/all-call/framer. Только positive results допускают
  internal RetireHistoryMetadata; original Closing отдельно проверяется, но не
  заменяет эти receipts. Self-join callback/producer запрещён контрактом.
- Whole-set preflight: app space существует, dropped Initial/Handshake допустимы,
  непустые identities различны; reset/history/intake idle; pending packet/probe
  отсутствуют; count непротиворечив и physical history/probe caps bounded прежним
  producer limit. Nonempty partial ACK scratch даёт отказ без догадки о том,
  какие packet aliases уже вернулись в pool. До полного preflight изменений нет.
- После него clear обнуляет только собственные history/probe pointer cells,
  включая physical tail после compaction. Packet/STREAM objects не читаются и
  не меняются; putPacket/ACK/loss/retransmission callbacks не вызываются. Heads
  ACK/lost scratch и scalar info отпускаются; provider aliases не обнуляются.
  Logical history count очищается, scalar network statistics/RTT/controller/ECN
  сохраняются. Это reference retirement, не proof GC/RSS/native reclamation.
- Actual Transport и existing HY2 QUIC close task используют aggregate до path
  retirement. Прежние independent aborts идут до waits, close errors joined;
  нового worker/task/timer, wire/dependency change или системной сети нет.

Ручной просмотр6 existing Windows diffs/2 new source; touched-only gofmt6 exit0.
16 captured reviewed inputs/0 cache reads;261 cache originals/205 verbatim mirrors
и прочие prior owned bytes сохранены. Receipts: .artifacts/memory/bootstrap-v462-packet-history-.
Следующий P0: crypto/control/retransmission и received-history holders после
фактических producer receipts. Полная приёмка остаётся открытой.
Source/unverified: manual source/diff/byte review and touched-only gofmt do not prove compilation, private mocks/fixtures, concurrency, cancellation/Close/ordinary-error/panic/Goexit/nonreturn behavior, interop, latency, GC/RSS or native/OS retirement. Internal RetireHistoryMetadata requires caller-side positive native-loop/public setter/STREAM/DATAGRAM receipts; Closing alone is insufficient. Its fixed prefix/result is retained on failure, and no finalizer retry or forced provider cancellation occurs. Nonempty partial ACK scratch refuses cleanup rather than guessing pool progress. Only owned reference cells are zeroed; packet/frame objects and provider-held scalar info backings are never mutated. Logical history count is retired; network progress/connection statistics are not fabricated. Received ACK history, crypto/control/retransmission queues, RTT/controller/ECN/custom provider and raw/OS/global lifetimes remain separate. The external aggregate must not run inside an operation/callback it joins. Full PC-00..09 and all prior obligations remain active;100 acceptance OPEN/56 checks NOT_RUN/72 tasks, TUN unavailable/schema2 install unsupported/root NETGUARD-001 OPEN. No tests/fixtures/build/vet/typecheck/product/SDK/harness/browser/native/admin/live/VPS/network/dependency/generation/Git/other-role/chat/subagent actions; existing commits retained.

<a id="pc-v463-crypto-queues"></a>

## V2-463 — original TLS Close receipt и local crypto queue retirement

Статус: source/unverified, 2026-11-16; quic-go65/sing-quic73.
Полная цель PC-00…09 и прежние обязательства сохраняются.

- Installed Go source crypto/tls/quic.go: QUICConn.Close отменяет и join-ит
  handshake, затем возвращает handshakeErr. Поэтому normal API failure и unknown
  original lifetime разделены. Это source evidence с captured SDK identity,
  без подтверждения того, какой toolchain войдёт в реальную сборку.
- Native cryptoSetup до constructor callbacks создаёт original close cell/done.
  Она удерживает exact TLS conn/result до нормального завершения; concurrent
  Close observers ждут тот же результат вне mutex. WaitForShutdown не вызывает
  Close повторно и сообщает completion отдельно от ordinary TLS error. Partial
  setup/panic/Goexit дают negative, original остаётся, retry нет. Nonreturning TLS
  join не прерывается искусственно. Internal CryptoSetup/root handler API расширен;
  private mocks/fixtures/compilation не проверены.
- External Conn.WaitForCryptoQueueRetirement сначала соединяет V462 native/public
  setter/data/map/sent-history aggregate и original TLS completion. Один finalizer
  хранит fixed3 streams/manager/retransmission/framer/packer prefix, stage/index/
  result/done. Negative/abnormal originals сохранены, concurrent observers ждут
  один результат; callbacks/waits не выполняются под его metadata mutex.
- До любых изменений проверяются identities всех трёх original stream wrappers,
  manager и native quiescent packer, distinct retransmission spaces и already-
  retired/sealed control framer. Crypto sorters должны оставаться callback-free,
  без STREAM ownership/budget, active/refused/incoming owners. Все queued entries
  проверяются по offset/positive length/nil callback и logical16KiB limit. Этот
  preflight не является лимитом retained backing/outgoing bytes/RSS.
- После whole-set preflight закрывается legacy retransmission admission,
  отпускаются local retransmission и crypto sorter/tree/write-buffer heads.
  Packet/frame объекты и plaintext/keys не перезаписываются; generic Discard,
  pool/ACK/loss/retransmission callbacks не вызываются. TLS handler сохранён для
  ConnectionState; TLS internal events/keys/provider aliases — отдельный scope.
- Native Transport и existing HY2 QUIC close task используют aggregate до path
  retirement. Independent aborts и сохранение close errors остаются; workers/
  timers/wire/dependency versions не добавлены. Received ACK history и outgoing
  crypto/retransmission byte budgets остаются дальнейшей работой.

Ручной просмотр8 existing Windows diffs/2 new source; touched-only gofmt8 exit0.
18 captured reviewed inputs и1 installed SDK source.261 cache originals сохранены;
203 verbatim mirrors;2 ранее неизменённых interface/retransmission files переведены
в owned changes с original byte snapshots. Receipts: .artifacts/memory/bootstrap-v463-crypto-queues-.
Следующий P0 — received ACK history и native TLS events/keys/other holders,
с отдельными границами ownership, diagnostics и callback compatibility.
Source/unverified: source/diff/byte review and touched-only gofmt do not prove compilation/private mocks, races/deadlocks, TLS callbacks/self-join/cancellation/ordinary failure/panic/Goexit/nonreturn interleavings, native resource retirement, interop or speed/CPU/GC/RSS/Stop bounds. Installed SDK source Close joins handshake before returning handshakeErr; its identity was captured, but no compiled-toolchain/version proof exists. Native Close API error is preserved separately from its normal-return receipt. Unknown originals retain state with no retry/forced cancellation; nonreturn blocks its original join. CryptoQueue retirement requires V462 native/public setter/STREAM/DATAGRAM/map/sent-history receipts, native TLS completion and existing framer retirement. Custom handler/provider hidden tasks and all custom API calls are not proved. Only local reference heads are dropped; no generic sorter callbacks, pool/ACK/loss operations, packet/frame dereference, data/key wipe or delivery fabrication. TLS handler/keys/events/provider-held aliases remain for independent lifetime/ConnectionState. Receive-sorter logical bytes/entries preflight does not bound exposed backing, outgoing crypto/retransmission bytes, process memory or rate. Retransmission retired flag assumes the native producer has already joined; it is not arbitrary concurrent-provider synchronization. Received ACK history, crypto internal holders, global/raw/native/OS/UX/release/comparative acceptance remain OPEN. No tests/fixtures/build/vet/typecheck/product/SDK/harness/browser/native/admin/live/VPS/network/dependency/generation/Git/other-role/chat/subagent actions; existing commits retained. Full PC/old9 active;100OPEN56NOT_RUN72tasks/TUN unavailable/schema2 unsupported/root NETGUARD-001 OPEN.

<a id="pc-v464-received-history"></a>

## V2-464 — original received ACK-history retirement

Статус: source/unverified, 2026-11-16; quic-go66/sing-quic74 unchanged consumer.
Полная цель PC-00…09 и прежние обязательства сохраняются.

- Conn.receivedPacketHandler теперь original pointer вместо скопированного value.
  Client/server packer получают его напрямую; sent ACK ignore callback привязан
  к тому же receiver. FinishConstruction требует nonnil receiver до публикации.
  Дополнительный original snapshot construction guard снят до его изменения.
- Constructor сохраняет immutable original Closing и owner identity, создаёт
  один result/done/finalizer. Копия receiver не может retire-ить original owner.
  Fixed3 current trackers и index сохраняются при negative/abnormal result;
  concurrent observers ждут один done вне metadata mutex, retry нет.
- Existing external WaitForPacketHistoryRetirement сохраняет native packet-loop/
  public setter/data/all-call/framer/map joins, затем sent-history receipt, и
  только после него вызывает received retirement. Closing — defensive check,
  не замена producer receipts. V463 crypto aggregate и actual Transport/HY2
  наследуют более сильный scope без нового public Conn method/task/worker.
- Whole-set preflight: optional dropped Initial/Handshake, distinct original
  tracker identities, logical MaxNumAckRanges, valid ordered nonoverlapping
  intervals и ignore floor. LargestAcked+1 при maximum packet корректно означает
  ignore-all, поэтому floor max+1 не выдаёт ложную ошибку. Все проверки идут до
  любых изменений, cached ACK frame вообще не разыменовывается.
- После preflight отпускаются только range/cached-ACK heads, hasNewAck/ackQueued/
  alarm/since-last-ACK scheduling state. Serial legacy receive/refill/get-ACK/
  timer paths отказывают в новом действии после retire. Cached ACK object и его
  backing не Reset/clear/pool-ятся, callbacks не вызываются. ECN/observed/floor/
  connection counters сохраняются; не создаётся fake ACK delivery. Это не
  physical capacity/GC/RSS/native reclaim или arbitrary-provider concurrency proof.

Ручной просмотр7 existing Windows diffs/1 new source; touched-only gofmt6 exit0.
16 reviewed inputs/0 SDK reads;261 cache originals/202 verbatim mirrors сохранены.
1 ранее неизменённый received-handler mirror переведён в owned source с original
byte snapshot. Receipts: .artifacts/memory/bootstrap-v464-received-history-.
Следующий P0: native TLS events/keys/other retained holders и outgoing crypto/
retransmission byte budgets, с сохранением diagnostics/provider lifetimes.
Source/unverified: original source/diff/byte review and touched-only gofmt do not prove compilation/private mocks/fixtures, concurrency/races/deadlocks, copied handler/custom provider/Close/cancellation/panic/Goexit/nonreturn interleavings, wire behavior, speed/GC/RSS or native/OS cleanup. Native packet input/output, public setter/data/call/map and sent-history receipts must precede internal receiver retirement; Closing alone is insufficient. One original owner/result/done is retained on negative/abnormal work, no retry. Actual Conn retains the original pointer; copied shells cannot retire it. Range preflight covers logical MaxNumAckRanges, packet bounds/order/floor, including legitimate maximum+1 ignore-all floor. Dropping heads does not zero physical capacity or prove pool/GC/memory reclamation. Cached ACK objects/backings are never inspected/reset/pooled; provider/trace aliases remain independent. ECN/observed/floor/connection counters remain; ACK scheduling is sealed without fabricated delivery. Serial legacy guards assume legitimate native producers already joined; arbitrary concurrent calls/providers remain unsupported/unproved. Existing public Conn signatures/actual HY2 source retained; history aggregate scope is stronger. TLS internal events/keys/provider aliases, outgoing crypto/retransmission byte budgets, raw/global/native/OS/UX/release/comparative acceptance remain OPEN. No tests/fixtures/build/vet/typecheck/product/SDK/harness/browser/native/admin/live/VPS/network/dependency/generation/Git/other-role/chat/subagent actions; existing commits retained. Full PC/old9 active;100OPEN56NOT_RUN72tasks/TUN unavailable/schema2 unsupported/rootNETGUARD001 OPEN.

<a id="pc-v465-crypto-events"></a>

## V2-465 — owned TLS events, pending queue budget и Go1.26 errors

Статус: source/unverified, 2026-11-16; quic-go67/sing-quic75 unchanged consumer.
Полная цель PC-00…09 и прежние обязательства сохраняются.

- Captured installed Go1.26.7 source crypto/tls/quic.go явно ограничивает жизнь
  QUICEvent.Data следующим NextEvent. В original writeRecord и ticket loop данные
  сохранялись без copy до следующего SDK call. Исправлено копирование в owned
  storage до продвижения. Это source-contract defect, без runtime reproduction.
- Все native metadata/write enqueue идут через один checked admission:128 pending
  event slots и8MiB queued copied bytes. Original slot/count/bytes резервируются
  до clone; metadata capacity растёт с hard clamp. Нет waiting task/worker.
  Local in-call ticket copy проверяется по оставшемуся allowance; после return
  она caller-owned. Budget не покрывает TLS allocations, transport-parameter
  objects, caller-retained data, stream/retransmission buffers или process RSS.
- Additive EventError/Err сохраняет signature NextEvent. Actual Conn consumer
  возвращает ошибку вместо fake readiness; native Start/HandleMessage также
  сразу получают capacity failure. Dequeue очищает original event cell до
  reslice, возвращает queued byte credit и отпускает empty backing head.
- SDK/API go1.26.txt подтверждают QUICErrorEvent и QUICEvent.Err. Эти ошибки
  обрабатываются в handshake и ticket loops, больше не попадают в unknown-ignore.
  Fork go.mod1.24→1.26 приведён к уже существующему Windows parent1.26.
  Dependencies/go.sum/version graph не обновлялись, resolver/tidy/download не
  выполнялись. Standalone build старым SDK больше не поддерживается этим fork.
- External-only RetireEventQueue вызывается existing crypto aggregate после
  actual native/public-call/data/history/TLS и local sorter preflight. Original
  Close mutex/result хранит один bounded retirement outcome без callbacks/waits.
  Все event kinds/ownership/count/bytes проходят preflight до очистки reference
  cells. Ordinary capacity refusal может быть quiescent; unknown credit/state
  retained/negative без retry. TLS object/keys/diagnostics/provider aliases и
  payload bytes не перезаписываются. Existing Transport/HY2 source unchanged.

Ручной просмотр8 existing Windows diffs/1 new source; touched-only gofmt6 exit0.
13 reviewed inputs/3 captured SDK inputs;261 cache originals/201 verbatim mirrors
сохранены.1 original fork go.mod mirror переведён в owned change со snapshot.
Receipts: .artifacts/memory/bootstrap-v465-crypto-events-.
Следующий P0: TLS key/AEAD/parameter holders и lifetime byte budgets crypto stream/
retransmission, сохраняя диагностику и provider aliases.
Source/unverified: captured SDK1.26.7/API ownership contract and source review identify borrowed-slice lifetime misuse; no runtime corruption reproduction or compilation proof exists. Touched-only gofmt/manual diff/byte review do not prove private fixture/mock compatibility, races/deadlocks, pressure/Close/cancellation/panic/Goexit/nonreturn/wire behavior, speed or GC/RSS/native cleanup. Fork now explicitly requires Go1.26 for QUICErrorEvent/Err, matching the existing Windows parent; older standalone SDK compatibility is not retained. No dependencies were resolved/downloaded/version-updated.128 slots/8MiB bounds queued owned write clones; a local session-ticket copy fits remaining allowance while in-call, then transfers to caller. TLS provider allocations/transport-parameter objects, data already transferred, stream write/retransmission buffers, aggregate process memory/CPU/rate are separate. Copying adds unmeasured handshake cost. Ordinary capacity refusal propagates as error, not readiness/unknown lifetime; inconsistent accounting retains originals and negative result/no retry. RetireEventQueue requires actual native/public-call/data/history/TLS and local sorter receipts; Close/Context alone are insufficient. Only metadata reference cells are cleared, never payload/key bytes or provider objects. TLS keys/native internals/ConnectionState/provider aliases and global/raw/OS/UX/release/comparative acceptance remain OPEN. No tests/fixtures/build/vet/typecheck/product/SDK/harness/browser/native/admin/live/VPS/network/dependency/generation/Git/other-role/chat/subagent execution; existing commits retained. FullPC/old9 active;100OPEN56NOT_RUN72tasks/TUN unavailable/schema2 unsupported/rootNETGUARD001 OPEN.
