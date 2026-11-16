# Windows quic-go source bundle — revision 67

Module: `github.com/sagernet/quic-go`, frozen `v0.59.0-sing-box-mod.4`.
Source: existing local cache, with183 production/module/licence/README files
mirrored. Tests/testdata, synctest, mockgen, assets, dotfiles and CI were omitted.
No runtime go:embed declarations were observed. MIT licence and original source
notices are retained. No download, module-cache write, dependency version change,
Go resolver, SDK, generator, test, build or native execution was performed.
Provenance/byte receipts: `.artifacts/memory/bootstrap-v393-quic-datagrams-`.

Only the Windows parent go.mod selects this module with a local replace. It is
paired with local sing-quic revision75 (upstream d83826c306d7); that consumer
requires NewDatagramSendScope, scoped Send, Closing, public packetaddr and
HTTP/3 ClientConn.WaitForShutdown, Conn.WaitForStreamReceiveShutdown and
Conn.WaitForStreamCallsShutdown/WaitForStreamMapShutdown/WaitForStreamSendDataShutdown/WaitForPacketInputShutdown/WaitForStreamMapRetirement/WaitForCongestionControlShutdown/WaitForDatagramSendDataShutdown/WaitForPacketHistoryRetirement/WaitForCryptoQueueRetirement.
The data-only WaitForStreamReceiveDataShutdown API is retained separately.
Parent tags and sing-box1.13.19 remain.
Declared inputs and this snapshot do not authenticate an upstream artifact or
prove the compiled dependency/tag graph.
Revision2 source changes/receipts: `.artifacts/memory/bootstrap-v394-session-deadline-`.
Revision3 adds a first-close signal at the original closeErr CAS, before any
sendQueue.Close wait. HY2 observes Closing to initiate raw abort on internal QUIC
errors too. Existing Context timing and original close-call results are unchanged;
the new notification never proves task/storage/native retirement. No inline
callback, new goroutine, timer or poller is introduced by the signal.
Receipts: `.artifacts/memory/bootstrap-v396-carrier-retirement-`.
Revision4 adds immutable local address snapshots at raw wrapping and sender
creation. Receipts: `.artifacts/memory/bootstrap-v399-packet-metadata-`.
Revision5 adds an original HTTP/3 client task owner and completion API.
Receipts: `.artifacts/memory/bootstrap-v400-http3-ownership-`.
Revision6 clears the original SendStream input view on every ordinary write
return, checks reset/shutdown/deadline before buffering a short tail, and seals
abnormal write state before the existing writer gate can be reused. Receipts:
`.artifacts/memory/bootstrap-v404-stream-borrow-`.
Revision7 retires current/queued receive frames at known terminal boundaries,
preserves original callback custody on abnormal return and caps Read/Peek at a
RESET_STREAM_AT reliable prefix. Receipts:
`.artifacts/memory/bootstrap-v405-receive-retirement-`.
Revision8 owns STREAM intake through connection trace/skip/map/receive handoffs,
consumes known sorter refusal/error paths, and bounds each receive sorter's
original holders and trim-copy overlap. Receipts:
`.artifacts/memory/bootstrap-v406-stream-intake-`.
Revision9 reserves connection STREAM receive owners/backing before the first
parser allocation/copy, retains immutable original release identities through
current/queued/trim data, and exposes a sealed receive-data completion result.
Receipts: `.artifacts/memory/bootstrap-v407-connection-receive-`.
Revision10 registers original receive/control calls before their token allocation,
read gate and stream mutex waits. It preserves an independent native-loop teardown
lane and composes actual call completion with the existing data/parser result.
Receipts: `.artifacts/memory/bootstrap-v408-receive-calls-`.
Revision11 extends the same original call owner to both stream directions, with
separate receive-only and all-registered-call completion. Bounded SendStream
write/control/native calls retain original tokens through waits and notifications.
Receipts: `.artifacts/memory/bootstrap-v409-send-calls-`.
Revision12 reserves transmitted STREAM backing before the first frame/pool/copy,
uses immutable data/send-attempt owners through splits and ACK/loss/discard, and
joins residual data only after positive original-loop/all-call quiescence.
Receipts: `.artifacts/memory/bootstrap-v410-stream-send-data-`.

## Changes and ownership

- One Conn send-data owner admits32768 original frames/32MiB exposed backing
  before holder/map/pool/copy allocation. Pooled frames charge1452 bytes, empty
  FIN charges one holder. nextFrame/retransmission queues hold original loans,
  and split output reserves while its source remains charged. The generic raw
  wire split API is unchanged; the owned SendStream path uses its own loan split.
- Each transmission has a distinct immutable attempt handler. ACK/loss marks
  that original, then the history cell clears its raw frame/handler before an
  optional release hook hands data to retransmission or pool. Slice lengths stay
  intact until existing history accounting, preserving ack-eliciting semantics.
  Late successful old callbacks are no-ops, not authority over a reused frame.
- Key-space/0RTT discard has a separate owned hook, never a false delivery ACK
  or retransmission request. Shutdown/old-stream discard does not publish stale
  stream completion into reset maps. Known post-FIN shutdown frees queued/current
  loans too; outstanding attempts remain charged until their actual handoff.
- PutBack runs outside the data-owner mutex; original view clear and credit
  return follow its actual return. Abnormal originals remain retained and sealed.
  Known overload is distinct from uncertainty and uses the existing close signal.
- WaitForStreamSendDataShutdown first requires positive original loop/context
  and all registered call quiescence. A single original finalizer then retires
  residual known-unsent/outstanding loans; negative custody never sweeps/retries.
  Its bounded32768-pointer snapshot avoids repeated sparse-map teardown scans.
  HY2 joins this result alongside receive/call/HTTP3/TCP owners. Private old
  packer/history aliases have no producer left at this quiescent boundary.
- The shared streamCalls registry adds independent512 Write and64 send-control
  admissions, with per-send-stream8/2 limits. It keeps receive512/64 and8/2
  unchanged. Registered native send/receive dispatch shares the original one-call
  serialized loop lane. Both stream constructors reuse the original Conn owner;
  there is no second registry, worker, timer or route fallback.
- Write, Close, CancelWrite, SetReliableBoundary and SetWriteDeadline register
  before token allocation and gate/mutex waits. Packetizer/window/reset-enable/
  STOP/control/shutdown calls retain native tokens. Write gate/data waits observe
  owned close, preserving actual partial counts. Abnormal bodies/notifications
  poison stored state before mutex/write gate reuse and retain original tokens,
  stream references and credits; known returns clear the original input view.
- Receive-only settlement filters its original three classes. The separate
  WaitForStreamCallsShutdown joins all six registered classes after the original
  loop returns. HY2 joins both results. These are call-lifetime receipts, not
  count/byte bounds or original-release proof for transmitted STREAM frames.
  Revision12 adds original STREAM data/attempt ownership separately. RESET/control
  history metadata, raw packets, crypto, arbitrary providers and process-wide
  owners still need their own acceptance; call receipts alone remain insufficient.
- One original receive-call owner admits512 Read/Peek and64 independent control
  calls per Conn, with per-stream8/2 limits and one serialized native-dispatch/
  shutdown call. Opening counts precede token/map allocation and blocking gates.
  Public close seals admission and wakes readers waiting for data/readOnce;
  native teardown remains admitted until the original Conn loop ends. Its final
  defer seals native admission before Context completion. Abnormal/early loop
  exit publishes uncertainty instead of a successful original-loop receipt.
- Read/Peek/CancelRead/SetReadDeadline and native STREAM/reset/control/shutdown
  retain original tokens through mutex/gate release and sender notifications.
  Known pressure requests the existing close notification outside metadata locks;
  it is not abnormal retirement. Abnormal tokens/credits stay retained, while
  the call result waits for all registered actual invocations to end. No new
  worker, timer, wait-list or raw fallback is introduced.
- WaitForStreamReceiveShutdown joins original receive-call and data/parser results.
  HY2 now uses that combined result. Pure StreamID metadata, send-side/promoted
  controls outside that receive scope, arbitrary external providers and whole QUIC/native/OS retirement are
  outside this scoped join. The data-only API keeps its narrower contract.
- Inline original intake slots precede connection tracing, map lookup and native
  receive callbacks. Normal untransferred frames return once; explicit transfer
  makes the next owner responsible. Abnormal body/release keeps the original
  slot and blocks reuse. Skip-handling tracing retains the earlier packet error.
- Parser offset/range and maximum frame-size checks precede pool adoption and
  payload copy. The Conn path reserves32768 original owners/32MiB exposed backing
  before holder/frame/pool/payload allocation. Its one registered parser spans
  tracing, handoff and original handler return. A validated allocator seam keeps
  local capacity/closing errors separate from malformed peer-frame errors. The
  legacy raw parser remains available outside this connection-owned path.
- Each original frame has a distinct immutable release object. A late successful
  callback cannot return a different frame after the pool reuses its address.
  Credits return after actual PutBack and owned-view removal, never at Pop or
  handoff. Trim output reserves both stream and connection quota while the old
  source remains charged. A partial allocation/abnormal callback stays retained
  and publishes a sticky negative result through the existing close notification.
- Original close seals parser/data admission. WaitForStreamReceiveDataShutdown
  waits for Conn context termination and the sealed original data/parser owner;
  uncertainty returns an error while retaining references and credits. HY2 joins
  this through its combined receive result in the original QUIC close call. This is receive-data completion,
  not an aggregate receipt for arbitrary control callers or whole QUIC/OS work.
- Each receive sorter admits8192 original holders and8MiB exposed backing before
  holder metadata and trim-copy allocation. Pop transfers quota to the current
  reader; it does not refund it. Trim reserves output while the original remains
  held. Native callback return and owned-view clear precede quota reuse. Active
  metadata/pool high-water/global/RSS have separate scope. Crypto's original
  callback-free sorter path does not enable this receive ledger.
- Push consumes known capacity/closed/duplicate/cut/error inputs. Original
  incoming and replacement entries survive abnormal callback return; failed
  partial gap mutation seals further queue use. The fixed refusal slot never
  overwrites an earlier uncertain incoming. Whole-process/global producer
  admission, full native caller/error joins, CPU/fairness/throughput and actual
  native caller/tag compatibility remain open acceptance boundaries.

- Receive terminal cleanup releases current and queued frame callbacks under
  the original receive mutex. Each original reference remains held until its
  callback returns; abnormal release is sticky and not retried. Sorter Discard
  seals before callbacks and removes each entry only after return. No new queue,
  worker, timer, payload clone or whole-process resource bound is introduced.
- Local CancelRead, shutdown, consumed EOF and effective remote reset dispose
  buffered data. Partial reliable prefixes and nonconsuming Peek remain intact.
  Read/Peek cap output at the reliable boundary, even within a current frame;
  a duplicate smaller boundary wakes a waiting reader. Abort cleanup cannot
  manufacture EOF, and a previously consumed EOF remains stable.
- Original read/peek/control/frame-call mutex state is tracked over intentional
  waits and notifications. Abnormal operations seal stored receive state before
  mutex reuse. Known pre-sorter rejection has its own retained original release
  slot. Revisions8/9 extend the original intake, sorter and connection data
  budget in source only. Revision10 adds scoped receive-call completion;
  send-side/promoted controls, raw packet/crypto/ACK/history storage, global
  admission and full native error aggregation remain open.

- SendStream's original write wrapper tracks intentional mutex release around
  sender notifications and waits. Ordinary success/error removes dataForWriting
  under the same packetizer mutex before returning. Partial counts are computed
  first, and accepted copied STREAM frames remain independent owned storage.
- Reset/shutdown/expired pending deadline checks precede new tail-buffer copies.
  Bytes already fully accepted retain success; an unsent tail cannot become a
  new successful frame after a terminal wakeup merely because it is small.
- Abnormal write-body return retains original views/frame state with sticky local
  uncertainty. Subsequent Write and Close fail; packet/control production,
  completion/replacement and destructive known-shutdown/reset cleanup do not
  reuse that uncertain state. Existing writeOnce gates the complete original
  wrapper. Panic/Goexit is not converted to success or recovered here. No new
  payload copy, worker, timer, quota or public API is introduced.
- This is a scoped input-borrow fence, not a general QUIC stream/frame/history
  resource bound or native retirement receipt. Pending callback mutex reacquire
  has no hard duration bound. Receive/sorter cleanup, reliable reset/ACK/loss
  custody, global owners and actual peer/tag/runtime behavior remain open.

- HTTP/3 client owner exists before constructor tasks. Nonwaiting admission64
  covers registered API calls, settings/accept/uni/datagram/request-cancel/body
  workers and original stream context callbacks. Seal and registration share
  one mutex; Close does not refund a worker before actual return. Capacity closes
  the connection as known overload, independently of abnormal cleanup failure.
- Dial/NewClientConn share an owned accept loop. Explicit raw-client handlers
  register synchronously. Stream callback reservations precede allocation and
  AfterFunc registration; both TrackStream callers propagate/refuse admission
  failure. WaitForShutdown observes Closing, joins registered work and the
  existing original qlogger callback/Close result. It does not initiate Close
  or retry a negative result; external stream/body borrowers remain separate.
- Client qlogger callback waits for sealed client work before its producer WG,
  retaining the original completion/error. Server's legacy qlogger path remains
  separate. HTTP/3 DATAGRAM receive now requires local enablement AND peer
  SETTINGS, preserving raw-QUIC consumers when HTTP/3 datagrams were not enabled.
- Public packetaddr captures bounded standard address fields or opaque
  network/text under the caller's original operation. Snapshot fields are private;
  Addr returns fresh standard objects/IP or immutable opaque values. Limits precede
  our clone, not arbitrary provider allocations/blocking inside Network/String.
- wrapConn captures local metadata before native setup, rejects invalid metadata
  and nil RawConn, and decorates both basic/OOB rawConn while preserving its
  original capabilities, read/write/deadline and Close methods. Native setup on
  non-Windows OOB paths may still perform its own provider calls.
- newSendConn owns its local snapshot and returns an error before sender/OOB
  allocation if capture fails. Dial, server construction and path-switch callers
  propagate/refuse the error; path state changes follow successful capture. The
  path's new queue goroutine retains its actual queue rather than rereading a
  mutable queue field. No placeholder, remote-alias retirement or full path join
  is inferred; source changes do not prove any migrated/native path behavior.
- Keep SendDatagram's signature and connection-lifetime waiting contract; add
  SendDatagramContext for synchronous cancellation of pending enqueue. No helper
  goroutine or implicit retransmission is added. Success means transfer to QUIC,
  not peer delivery or later revocation of an already admitted frame.
- Reserve count and bytes before TX payload/entry allocation, covering waiting
  producers, queued data, packer borrowers and transferred frame/history storage.
  Pop transfers a credit rather than refunding it. Peek grants an explicit lease
  which Close cannot invalidate while the packer still reads the data.
- The original ACK/loss or known local pack-failure path retires the same storage
  once. Clear its data before credit return; history may retain only metadata and
  the immutable original length. The DATAGRAM handler never requests retransmit.
  Queue Close drops its own entries, retaining pending/borrowed/history ownership
  until those original holders return. Uncertain terminal history is not refunded.
- The current connection's RX parser borrows a raw-packet view. Count/byte queue
  admission precedes the sole copy under the receive owner. The previous owning
  parser remains available with its independent-buffer behavior. The view never
  escapes the synchronous receive/copy path; tracing receives length metadata.
- Fixed rings retain32 send/128 receive queue slots. Context methods and hasData
  callbacks execute outside queue locks. Queue Close uses send->receive lock order
  and has one immutable error/publication, so closed queues cannot accept data.
- DATAGRAM debug/qlog output uses original length, never payload contents. Clearing
  backing does not turn a lost/acked frame's recorded size into zero.
- DatagramSendScope binds original self/Conn/queue identity. A copied or zero
  object cannot grant enqueue or return another scope's quota. Queue reserve and
  enqueue hold queue->scope mutexes; Close seals the same scope mutex and wakes
  its waiters. No queue mutex is acquired while the scope mutex remains held.
- Scope-local write deadlines apply to future and pending admissions, with one
  reusable timer per blocked Add and a changed channel for reset/clear/close.
  Every timer event rechecks current state; no stale-tick timeout or helper
  goroutine. The common socket's deadline and neighboring scopes are untouched.
- Reuse the stream deadline error and monotime domain. A checked local bridge
  anchors time.Time at one current wall/monotonic reference, saturates signed
  addition and keeps every finite deadline distinct from the unlimited zero.
  Only time.Time{} clears the deadline; this supplies no trusted UTC authority.

## Engineering ceilings and scope

| Scope | Ceiling |
|---|---|
| Each connection's retained TX DATAGRAM storage | 4096 original owners and8MiB requested payload bytes |
| Visible send queue | Existing32 slots; other credits remain with producer/packer/history |
| Each connection's RX queue | Existing128 slots and2MiB copied payload bytes |
| Receive handoff | Caller receives one owned byte slice; queue credits end at that explicit transfer |
| Each connection's send scopes | 256 open/retiring scope slots; a sealed slot returns only after its active Send calls end |
| Each original scope | At most8 concurrent Send calls; one deadline/change channel, at most one timer per waiting Add |
| Each connection's STREAM receive data | 32768 original owners and32MiB exposed backing, including parser/current/queued/trim overlap |
| Each receive stream's sorter/current data | 8192 original holders and8MiB exposed backing, including trim overlap |
| Each connection's receive calls | 512 Read/Peek,64 independent controls and1 native-dispatch/shutdown call |
| Each receive stream's public calls | 8 Read/Peek and2 controls, including admitted gate/mutex waiters |
| Each connection's send calls | 512 Write and64 independent send controls; native dispatch shares the original1 slot |
| Each send stream's public calls | 8 Write and2 controls, including admitted gate/mutex waiters |
| Each connection's transmitted STREAM backing | 32768 original owners/32MiB, including queued/outstanding/known-unsent/split overlap |
| Original quiescent send-data finalizer | One snapshot of at most32768 pointers; no new worker/timer |

These limits exclude raw input/serialized packet buffers, queue/history metadata
outside the respective admitted ledgers, allocator/GC/pool high-water, caller-owned
buffers, DATAGRAM outputs after transfer, other carriers, sockets and the whole process. They do
not prove RSS, CPU, throughput, latency or native retirement. HY2 separately bounds
its adopted receive messages and staging; legacy receivers can retain their own
outputs. A closed queue's unresolved history is not a fresh global capacity grant.

## Revision13 — sent history admission and original packet output

The frozen history is a sequential pointer slice with existing SendMode scheduling
thresholds (40000 outstanding/50000 tracked), not an unbounded indexed queue.
Revision13 adds a hard admission fence before getPacket per number space and
clamps both history/probe slice growth to the existing50000 ceiling. Skipped
numbers retain sequential/Len/last-four semantics; capacity failure reaches Conn
through checked SentPacket errors or PTO. The original pending packet is attached
before congestion callbacks, with sticky failure across Retry/space/path reset.
This internal interface now returns error; all production Conn callers propagate
it before physical send. External dependency versions and public HY2 APIs stay.

One inline Conn output owner retains the original completed packet/batch before
tracing, history and queue/provider handoff. Its fixed DATAGRAM reference array
uses this packer's one-DATAGRAM-per-packet and full-segment GSO invariant, plus
one tail. Known pre-handoff errors retire all unsent DATAGRAM credits and the raw
buffer, including an accepted GSO prefix; STREAM data retains its own quiescent
owner. Ordinary queue return transfers custody without rereading pooled storage.
Alternate path Transport.WriteTo only borrows bytes: its original buffer is now
returned after actual provider return; short/error/invalid counts are explicit.
Abnormal acquisition/transfer/pool return keeps original custody and prevents a
positive loop receipt. There is no new worker, timer or direct traffic fallback.

This covers completed packer output/current batch and history pointer capacity,
not all partial packer allocations, raw receive/queue/provider ownership, ACK/
control/crypto payloads, CC/ECN samplers, aggregate connections, allocator/pool
high-water or process RSS. All source remains uncompiled and untested. Receipts:
`.artifacts/memory/bootstrap-v411-sent-history-`.

## Revision14 — original partial packer buffers

The original packer now reserves one serialized operation before callbacks or
pool acquisition. Its inline intake retains the exact partial buffer and fixed
DATAGRAM entry references before queue/history transfer, including a coalesced
payload not yet appended when another packet fails. All production packer entry
paths use wrappers with explicit ordinary-return guards; constructor and packer
interface signatures stay unchanged. No pool-size or dependency change is made.

Ordinary errors and empty output retire consumed DATAGRAM credits and owned raw
buffers only after packing returns. AppendPacket borrows the Conn/GSO original
and never releases it; the Conn handles a known unsent batch. An uncertain packer
result prevents that Conn from releasing the borrowed original and prevents a
positive loop receipt. Abnormal callback/pool returns keep originals and refuse
new intake; serializer defers no longer retire payload while such a borrow is
unresolved. Error results carry nil buffer instead of a stale pool pointer.

Successful results transfer to the existing completed-output owner. SendProbe
has a distinct synchronous implementation: WriteTo borrows its bytes, so ordinary
return now consumes the original buffer with Release, matching its caller's
transfer contract. The existing void SendProbe API still ignores its write error;
this change does not claim a full send-queue/provider retirement receipt.

The intake bounds one packer operation/owned raw buffer and its DATAGRAM pointer
array, not crypto/control producers, arbitrary hidden borrowers, queued/stopped/
fatal/abnormal send-queue paths, terminal-close cache, raw receive, global retiring
carriers, allocator/pool high-water or RSS. Private payload helpers require the
production intake and are not an independent lifetime API. Source only; no tests,
build, runtime or native acceptance. Receipts:
`.artifacts/memory/bootstrap-v412-packer-buffers-`.

## Revision15 — original send-queue custody and completion

The channel select race at stopped admission is replaced by one metadata fence
for seal/admission and a fixed8-entry ring. Nine normal slots cover8 pending plus
the original worker; one separate slot covers a synchronous probe. Each retains
the original raw buffer and at most18 DATAGRAM references for this20KiB/1200
packer invariant. Pool capacity is validated (1452 or20480 bytes). There is no
per-packet worker, dynamic pending queue, new timer or direct fallback.

Internal Send/SendProbe return accepted separately from error. Refusal leaves
the caller's original intact; acceptance copies custody into a reserved fixed
slot before return. Successful raw writes leave DATAGRAM history ownership in
place; known failed/dropped writes and fatal-drain entries retire their original
credits exactly once before actual raw pool return. Existing owned DATAGRAM
metadata uses immutable DataLen; known native ACK/loss/trace paths need no payload
after serialization. Arbitrary hidden provider/trace borrowers remain outside
this receipt. No provider, pool call or foreign error method runs under the
queue mutex. Unknown callback/pool/count outcomes retain roots and seal reuse.

Close is idempotent and joins the original worker and admitted probe. Its
retired result is distinct from the stored native write error; negative custody
does not retry cleanup. Known fatal errors seal and drain remaining originals
without more writes. EMSGSIZE/DatagramTooLarge keep the existing loss/MTU policy;
short or bytes-plus-error writes are fatal and suppress GSO prefix replay.
Permission retry requires zero bytes. Invalid raw write counts are uncertain.

Conn checks all transfer/close results, captures the initial Run queue before
handshake, and closes it on early exit. Path replacement requires a positive
error-free old queue result before mutating congestion/MTU/path state. A failed
queue cannot preserve the internal recreation exception or authorize an extra
terminal write. Actual queue retirement participates in the original-loop
receipt. Internal sender signatures changed; no public QUIC/HY2 API was added.

This is source-only: compilation/mocks/API graph, concurrency/wire/GSO/MTU/path/
early-exit/panic/Goexit/pool interleavings and speed are unverified. It does not
force a blocked raw provider to return, contain arbitrary provider panics, prove
OS/raw receive/terminal-close cache/crypto/CC/global retirement, RSS or a Stop SLO.
Native TUN/schema2/full-PC/release gates remain unchanged. Receipts:
`.artifacts/memory/bootstrap-v413-send-queue-`.

## Revision16 — terminal raw buffer and detached retry bytes

CONNECTION_CLOSE now adopts the completed original buffer into the existing Conn
output owner before ECN/tracing/native Write. Ordinary write return retains that
buffer through all closed-cache copy callbacks; only their actual ordinary
return permits the original pool return. Unknown write counts, callback/pool
failure and abnormal raw borrows retain the original and deny a positive loop
receipt. Empty/oversized terminal output is refused before native Write.

Each transport makes one independent immutable retry backing of at most1452
exposed bytes before publishing a local closed handler. Neither expiry nor a
queued retry can alias a recycled original packet buffer. The new copy is not a
whole-transport allocation budget: cache count/aggregate bytes, ID/timer admission
and actual retry/control-worker completion remain OPEN. Constructor/callback
signatures, backoff and the existing best-effort queue are retained.

Closed local/remote handlers now consume one incoming packet-buffer reference,
including shared buffers, and clear the original view after actual pool return.
Local retries capture bounded address metadata before release and use only that
snapshot/packetInfo through later logging/queueing. Expiry deletes an ID only if
it still names that exact original handler. Remote closed handlers have nonzero
identity storage, since distinct zero-sized objects may otherwise compare equal.

No new worker/timer, transport activation, direct fallback, dependency version or
licence change. This is source-only, not compiler/wire/race/provider/native/RSS/
Stop/performance proof. Registration identity, closed-cache aggregate admission,
timer/dispatch/control-worker joins, raw receive and global/crypto/CC/OS/full-PC
acceptance remain separate unfinished work. Receipts:
`.artifacts/memory/bootstrap-v414-terminal-output-`.

## Revision17 — bounded closed cache and original control worker

The closed cache now admits before holder/copy/ID/timer allocation:256 records,
384KiB exposed payload,64 IDs per record and1452 bytes per retry body. Remote,
opening, retiring and unknown records count too. Pressure discards best-effort
cache state by removing only IDs still belonging to the original connection;
it does not allocate a replacement tombstone/timer. Internal runner callbacks
carry the original Conn; authority recognizes both Conn and its wrappedConn.
Publication and expiry never overwrite/remove a different current handler.

Each immutable cache owner has a registration/ready fence, original timer state
and queued/current retry refs. Credits return only after withdrawal, actual
Stop=true or callback return, and every retry lease ends. Stop=false keeps the
original pending callback; abnormal originals retain roots/credits and yield a
negative completion. At most the admitted256 original expiry timers exist; no
new worker/timer class or module/dependency/public API change is introduced.

Transport close seals cache/retry/stateless admission, withdraws original cache
records and stops their original timers. The existing control worker retains
retry payload/OOB and stateless-reset input/output/OOB through actual provider
return; invalid counts or abnormal pool/provider return preserve originals.
Known queued originals drain after sealing. Automatic single-use read stops are
coalesced into this same worker, with its original rawConn retained through the
deadline call. Native/error/pool calls run outside transport metadata locking.
Public Close joins that worker and cache completion; init/deadline/native-close
errors are negative results. Best-effort packet-write errors keep their prior
policy; stateless-reset tokens are no longer emitted by the debug log.

Internal connRunner/closePacket/test-helper ABI changed and is uncompiled.
The limits are engineering ceilings, not measured optima, whole-process/RSS/CPU
limits or wire/performance/native acceptance. Active-ID/retiring-ID producers,
generic ID/reset-token removal authority, raw receive, direct Transport.WriteTo,
listener/root Close/tracer/caller concurrency, arbitrary hidden providers and
global carrier/crypto/CC/OS/full-PC retirement still need their own contracts.
No forced cancellation, provider-panic containment or hard Stop SLO is asserted.
Source-only receipts: `.artifacts/memory/bootstrap-v415-closed-cache-`.

## Revision18 — original Transport and server close results

Transport has one original initializer/native abort/root cleanup/public Close
and recorder-close outcome. Close seals admission first and never initializes an
unused borrowed socket. Repeated closers observe the same original result; they
do not retry a failed native operation. Known unscheduled loops are distinct from
an abnormal initializer. The original socket/recorder/server and handler calls
remain retained when completion is unknown. Handler aliases are deduplicated by
original Conn, including reset-token-only registrations; known Conn teardown
joins existing receive-data, stream-call and send-data completion APIs.

The server's original close now joins both its receive and response-send tasks.
Its metadata mutex is released before callbacks and waits. A replacement server
must wait for that original close to return normally. Refused unregistered
candidates keep their original server task owner; their nonwaiting close request
precedes their terminal loop, instead of waiting for a loop not yet started.
Recorder.Close follows the original listener and server producers, with its
actual error retained. Native callbacks never execute under lifecycle metadata
locking. A caller must not synchronously close its own callback's Transport.

Direct WriteTo reserves64 calls/4MiB exposed input spans (each at most65535 bytes)
before holder/map allocation, initialization wait or provider invocation. It
retains the exact input, original address and raw provider through actual return.
Invalid counts/abnormal return keep original roots and credits, seal admission
and yield a negative completion result. Initializer waiters are separately
limited to64 before sync.Once waiting. Public Close joins original root,
listener/control worker/cache/direct-write and recorder outcomes. Only positive
joins permit clearing a borrowed socket's temporary read deadline. Known
net.ErrClosed is accepted without repeating native cleanup.

Public signatures, frozen versions and licences are unchanged. These are
engineering admission ceilings, not RSS/backing-allocation/global caller/CPU,
hard Stop SLO, cancellation or measured performance guarantees. Existing
handler-destroy fan-out is deduplicated but not globally capped. Raw receive and
server queue view retirement, generic CID/remove authority, constructor/control
callers, crypto/CC/global retiring inventory and native acceptance remain open.
Source, diff and touched-only gofmt receipts establish no compile/race/wire/
provider/Goexit/OS/runtime result. No tests or fixtures were added/changed/run.
Receipts: `.artifacts/memory/bootstrap-v416-transport-lifecycle-`.

## Revision19 — original Windows raw read and Transport input custody

The Windows-selected basicConn reserves one original active reader before pool
acquisition and ReadFrom. It retains the exact provider, pooled buffer and
returned address through actual return/count validation. Invalid counts or
abnormal pool/provider exits keep that original slot and references, return a
negative completion result and deny another native read. Valid ordinary errors
return the original unused buffer once. No native callback runs under its
metadata mutex; concurrent reads fail before another allocation/provider call.

The listener retains its original raw source and returned packet before error,
tracer and handler callbacks. Known empty/header-parse/no-server/stateless-reset
drops consume one reference using existing shared-buffer release semantics.
Non-QUIC disabled/full/sealed drops do the same. Temporary-error handling also
observes the lifecycle seal. Production stateless resets use the original Conn's
nonwaiting close request, without a new destroy goroutine per datagram or logging
reset-token bytes. Private custom handlers stay in the guarded listener task.

The fixed32 non-QUIC queue is created by the original initializer, removing
concurrent lazy queue replacement. ReadNonQUICPacket reserves64 original calls
and4MiB exposed destination spans before token/map/init/context waits. Each token
retains its original context/output/current input through copy, standard address
snapshot and actual input release. Custom concrete addresses remain unchanged.
Closing seals publication and wakes pending readers. One original drain result
joins actual listener/read-call completion before releasing remaining queued
inputs; natural listener exit and public Close share it. Only positive composed
results permit recorder retirement and borrowed deadline clearing. Abnormal
originals remain attached; a negative signal never authorizes pool reuse/retry.

These are local engineering bounds: one current basic read buffer1452,32 queued
non-QUIC references and64 registered read callers/4MiB exposed spans. They do not
bound backing allocations, idle pool retention, all QUIC/server buffers, global
carriers, caller stacks, RSS/CPU or elapsed Stop time. Custom rawConn shortcuts
and non-Windows OOB/batch reads need separate ownership/acceptance. Conn queue
refusal/sealing, coalesced/undecryptable processing, server/0RTT/response queues
and response output custody remain the next source work. No new worker/timer,
dependency version/licence/public signature change or direct fallback.
No tests/fixtures/build/native/product execution; manual source/diff review and
touched-only gofmt establish no compile/race/wire/runtime/performance proof.
Receipts: `.artifacts/memory/bootstrap-v417-raw-receive-`.

## Revision20 — original Conn input calls and datagram references

Each Conn has64 fixed original intake cells, at most64*20KiB charged packet
backing spans before receive-mutex waiting. The existing receive queue remains
limited to256 packets. Close seals admission before terminal waits; known full,
late or refused input drops release exactly one shared reference outside queue
and input metadata mutexes. Abnormal registered calls retain their original
cells/views/credits and block a positive completion result. Capacity refusal
keeps the upstream listener/server as the rejected call's original owner.

One fixed processing cell anchors the complete original datagram before crypto
or trace callbacks. Each long/short-header handler receives its own working ref;
normal parser/version-negotiation/error exits settle untransferred refs and the
anchor only after actual callback return. Abnormal unwinding retains the anchor
and denies reuse. A value copy replaces the old per-part shallow Clone; the
coalesced counter is an int, avoiding uint8 wrap for large custom inputs.

Original popped, replay and terminal-drain views stay attached before callbacks.
The undecryptable enqueue result now says whether the packet was actually queued;
refusal no longer prevents the caller's decrement. The32-view limit spans waiting,
ready and pending replay views together. Key-event moves clear old cells, and
handshake/terminal cleanup releases original cells before clearing the arrays.
Error in a replay batch retains and drains its remaining original cells.

WaitForPacketInputShutdown composes the original loop and admitted intake calls.
The Windows Transport root and actual HY2 original QUIC close call consume it.
Connection recorder Close follows positive intake-call quiescence; its actual
error becomes a negative loop result. The prior recreation trace exception is
preserved. No per-packet registry/processing allocation, worker or timer is added;
the fixed metadata cost belongs to each Conn. This is no global/RSS/CPU/Stop or
performance proof. No existing public signature or frozen dependency/licence
changes; the new completion method requires a compatible local source pair.

Server/0RTT/response queue and response-output custody, generic CID/reset removal,
constructor/control/crypto/CC/global inventory and native acceptance remain open.
Custom/provider hidden borrows and private helper/mocks/tag/ABI/interleavings need
separate acceptance. No tests/fixtures/build/native/product execution. Receipts:
`.artifacts/memory/bootstrap-v418-conn-input-`.

## Revision21 — original server queues and response output

The server has64 fixed intake cells/64*20KiB backing-span admission before its
1024-entry receive queue. Publication and close share one seal fence. Known
full/late/refused calls return one shared input ref outside metadata locks;
unknown calls retain original cells/credits. Transport snapshots the original
server and releases its own mutex before delegation. A failure signal wakes
Accept and makes original Close return a negative result while pending producers
remain owned. A negative server outcome does not start root destroy waits on
possibly unstarted candidates; the original server and handler rows stay held.

The receive task retains original current/held/handoff/candidate/drain views
before callbacks. Ordinary Initial errors that never transferred input now
release it. 0RTT discard/retry/refusal/expiry/terminal paths release original
cells before map deletion; transfer detaches each original cell before its
handler call. The existing32 queues/31 views per queue remain. No extra packet
anchor is shared across server/Conn tasks: the packet refcounter is non-atomic.
Positive receive stop seals publication and drains queued/0RTT originals; an
abnormal original retains its views instead of speculative cleanup.

The existing response worker retains each original Retry/invalid/refused/version
input and header, pooled buffer/immutable backing or composed data, OOB and exact
raw provider through callbacks and actual WritePacket count validation. Ordinary
completion alone returns the original output/input refs. Invalid counts and
abnormal exits retain originals and stop reuse; known best-effort write errors
keep their policy. Original pooled backing survives nil/error Append results.
Response payloads are limited to1452 before pooled acquisition/append or version
composition; the version composer allowance includes its extra greased version.
Opaque token/address/provider allocation and blocking remain separate scope.

After the actual receive task ends, positive response stop drains its remaining
4/4/4/8 input queues. Original server close joins intake/receive/response outcomes;
negative completion is never a producer-stop or replacement receipt. No new
worker/timer/per-packet registry allocation or dependency/API/licence change.
The fixed metadata and local queue/output ceilings are not global/RSS/CPU/Stop,
wire compatibility or performance acceptance. Full constructor/context/trace
custody, generic CID/reset removal, crypto/CC/global and native/OS scopes remain.
No tests/fixtures/build/native/product execution. Source-only receipts:
`.artifacts/memory/bootstrap-v419-server-input-`.

## Revision22 — CID and reset-token registration authority

CID and reset-token removal now requires the original Conn identity. Registration
is idempotent for that owner and refuses foreign entries without overwriting
them. Server dual-CID registration checks both keys, aliasing and source length
before either write; client dial checks its source CID before trace/constructor
callbacks. Existing wrappedConn identity is recognized by the shared matcher.

Generated CIDs must match the initial source length and differ from active,
retiring and initial destination IDs. All runner registrations must succeed
before NEW_CONNECTION_ID is queued. A refused issuance retains its pending ID
for original teardown and makes further issuance fail with the original error.
Handshake and restored 0-RTT parameter callers now propagate issuance failures.
Refused reset-token registration requests nonwaiting original connection close.

An additional path attaches its runner before callbacks; known registration
failure withdraws only original-owned IDs before removing the runner. Unknown
callback/cleanup completion retains that prefix. Probe receives the original
enable error; no PATH_CHALLENGE is produced, the reserved peer CID is retired,
and the original Probe timer is stopped on exit. No new task or timer class.

Same-Conn registration epochs/refcounts, reset-token routing to migrated
transports, full path/control lifecycle and constructor/context/cancel/trace
prefixes remain open. No new global inventory quota or performance claim.
Public API signatures, frozen module versions/licences and input/output ceilings
remain; private callback signatures changed and are uncompiled. No tests/fixtures,
build, product/native/network execution. Source-only receipts:
`.artifacts/memory/bootstrap-v420-cid-authority-`.

## Revision23 — server construction prefix and handshake failure outcomes

One fixed opening record in the original serialized server receive task retains
the send connection, original/returned contexts and cancellation functions,
trace and partial Conn before constructor callbacks. An explicit private
constructor argument attaches the Conn before AddProducer/provider calls. Only
known promotion or original rejected-loop retirement clears the record; unknown
construction/cleanup retains it and produces a negative server outcome. Server
CID generation/length/alias validation precedes context/trace creation.

Ordinary StartHandshake / initial event errors and observed context cancellation
now use the common terminal path, including crypto, send queue, CID/reset/stream,
trace and input cleanup. The original loop timer is stopped on every exit.
The crypto Close error is preserved; the inspected local TLS Close implementation
joins its original handshake goroutine and can return that handshake error.
Terminal normalization uses a private copy of the published close record.

The public HandshakeComplete notification retains completion/failure semantics
and wakes waiters on terminal startup errors too. A separate atomic success bit
gates Dial, server enqueue and NextConnection after 0-RTT rejection. Dial rechecks
observed request cancellation and original connection closing state before
returning a ready connection; Accept skips already closing connections. Server
callback-context cancellation is observed at startup and in the original loop.
No new worker, timer class, global registry or public API signature is introduced.

This is source-only. Client constructor admission/locking/callback prefixes,
opaque provider operations, context/trace reentrancy, full recreation/path/reset
routing/global inventory, crypto/CC/OS and complete artifact acceptance remain
open. The additional private constructor signature remains uncompiled; tests and
fixtures were not changed. No tests/build/native/product/network execution.
Receipts: `.artifacts/memory/bootstrap-v421-connection-opening-`.

## Revision24 — original client construction and CID reservation

Each Transport has64 fixed client opening cells. Admission precedes initializer
waits, TLS/config clones, address/provider calls, trace and construction. The
original context/address/TLS/config/raw/send/trace/partial Conn stays attached
until known promotion or actual rejected-candidate retirement. A private
constructor argument captures the Conn before trace/address/token callbacks.
These cells limit concurrent construction only, not established connections or
opaque provider allocation, global memory, RSS, CPU or Stop duration.

Client trace/log/provider/constructor callbacks run outside Transport.mutex and
lifecycle.access. A source CID is reserved under mutex then access; there is no
fake demultiplexer handler. Generic CID and dual-CID registration refuse reserved
keys. Client publication atomically transfers that reservation to the original
map against the common seal; generic CID/reset additions share that seal fence.
The opening returns its cell only after the existing original loop is scheduled.
Known cancellation/refusal of a complete candidate runs its original terminal
loop and existing input/stream-data retirement before releasing the reservation.
Unknown construction/cleanup retains the original cell and seals failure.

An intrusive client list in the initial Transport retains promoted originals
independently of CID rows, which can disappear before registered calls retire.
No failure-time holder/map is allocated. The original client worker waits for
existing packet-input/receive-data/send-data retirement before unlinking or
offering version recreation; the existing send-data gate already joins registered
stream calls. Negative or abnormal outcomes retain/relink the original, publish
failure and request the existing native abort. No new worker or timer class.

Root close joins opening outcomes after original abort/init and before CID/client
snapshots, trace close or borrowed deadline restore. Negative completion is not
producer-stop or replacement authority. Nil TLS config has an ordinary admission
error. Existing public signatures/versions/licences and HY2/input/output quotas
remain; the private client constructor signature changed and is uncompiled.
Reentrant Close from an original callback remains unsupported (self-join).
Listener/init/provider callbacks, full recreation/trace/opaque borrows, same-Conn
epochs/reset migration/path/global/cryptoCC/nativeOS/full-PC acceptance remain
open. No tests/fixtures/build/product/native/network execution or speed claim.
Receipts: `.artifacts/memory/bootstrap-v422-client-opening-`.

## Revision25 — recreation recorder and original qlog finalization

Every connection attempt closes its own recorder after original input calls,
including version recreation. The per-attempt trace factory behavior is retained;
no successor silently inherits an unretired producer. Close errors mark the
original Conn negative, so its existing retirement gate prevents unlink/recreate.

The standard FileSeq Writer now has one original Close with a cached result.
RecordEvent admission shares its close fence; Close joins admitted sends before
decrementing the producer count, outside metadata locks. Repeated Close never
decrements twice or retries a native close. An abnormal original Close records
an uncertain result for subsequent observers; it is not recovered as success.

FileSeq.Run uses one original task through sync.Once. Its current event stays
attached through Name/Encode/writer callbacks, and normal return is recorded
only after actual queue drain/log completion. The last producer joins that task,
returns encoding and underlying Close errors, and leaves event/encoder/provider
attached without speculative raw Close after an abnormal Run. Existing queue50
and multiple-producer ownership remain; a producer does not close another owner.
Stopped-Run selection wakes blocked event senders, so their Close join can reach
the original negative result. A non-last producer also reports an observed
abnormal task exit; rejected new events were never passed to event callbacks.

No new worker/timer class, dependency/version/licence/public API change or global
event-byte/RSS/CPU/Stop bound. Optional tracing has additional synchronization;
no speed or overhead optimum is claimed. Reentrant own Close/Run/Event callbacks,
opaque event immutability/hidden borrows, full sink/count/provider contracts,
version-negotiation validation, listener/init/path/reset/global/native/full-PC
acceptance remain open. No tests/fixtures/build/product/native/network execution.
Receipts: `.artifacts/memory/bootstrap-v423-recreation-trace-`.

## Revision26 — version-negotiation request binding

The client stores the initial source CID as an immutable value for its attempt.
VN handling refuses pre-send, processed-Retry/first-packet/prior-negotiation and
closing phases. It reuses the existing owned-CID prefix parser to check response
destination against the original client source and response source against the
original Initial destination before allocating the offered-version slice.

The full decoder's destination/source return order is consumed correctly; qlog
received CID fields now reflect that order. Existing malformed-list/current-
version rejection, version preference/no-overlap handling and original packet
retirement stay. A shared drop logger does not own or release a packet reference.
There is no new tuple gate, clipping of supported versions, decoder API, worker,
timer, registry or global quota. Immutable CID metadata has a fixed local cost;
no zero-allocation or measured speed/overhead claim is made.

Read-only primary reference: RFC9000 sections6.2,17.2.1,12.1 and21.12, consulted at
https://www.rfc-editor.org/rfc/rfc9000.html. The echo/phase binding is not peer/TLS/
route authentication or downgrade protection. Source IP/port restrictions were
not added; hopping/migration interoperability and full VN/Retry/version-2 secure
negotiation acceptance remain open. This reference lookup was not a VPN/native/
live check or a system network change. Public APIs/versions/licences and earlier
HY2/QUIC quotas remain. No tests/fixtures/build/product/native/live execution.
Listener/init/provider/path/reset/global/cryptoCC/OS/full-PC acceptance stays open.
Receipts: `.artifacts/memory/bootstrap-v424-version-binding-`.

## Revision27 — Retry request and phase binding

Retry admission checks client/first-send/first-processed/duplicate/closing phases,
matching version and the immutable Initial source CID before manager/crypto work.
The server source CID must differ from the original Initial destination. Tag
calculation now uses that immutable original value instead of CID-manager Get.
Observed closing state is checked again after integrity validation before the
existing Retry transition. No source IP/port gate or new cryptographic algorithm.

The parser already rejected empty Retry tokens; ResetForRetry already preserves
the next Initial/application packet numbers with Peek. Those are not claimed as
defects. A defensive callee header/token/raw-length proof precedes tag slicing.
Existing token ownership, key/token/CID update and retransmission/number behavior
remain; received qlog destination now comes from the actual Retry header. A
shared drop logger holds no packet/pool reference or cleanup authority.

One reference is the previously read RFC9000 section17.2.5.2 at
https://www.rfc-editor.org/rfc/rfc9000.html#section-17.2.5.2. A new documentation
fetch failed; no fetched RFC9001 content or VPN/VPS result is claimed. Integrity
and CID binding are not TLS/server/path authentication. Full Retry/crypto/route/
hopping/runtime/provider contracts and listener/init/path/reset/global/OS/full-PC
acceptance remain open. Public APIs/versions/licences/earlier quotas stay.
No tests/fixtures/build/product/native/live execution or measured speed claim.
Receipts: `.artifacts/memory/bootstrap-v425-retry-binding-`.

## Remaining contract and acceptance

Legacy context-only sends can race cancellation. Scoped Close and enqueue now
share a commit fence: if sealing wins, enqueue is refused; if transfer wins,
the accepted frame remains QUIC-owned. Close does not wait for its calls or
revoke accepted frames, and scope-slot return does not refund their byte credits.
Scope calls/metadata are separate from HY2's outer producer/buffer lifetime.
Deadline is checked at admission; timer/scheduler delay is not a hard expiry SLO.
Closing a scope does not interrupt an already entered raw socket write or close,
nor authorize replacement of an unretired carrier/process. Whole-carrier abort,
global retiring inventory, actual wrapper/path budget and native gates remain.

## Revision28 — original listener construction and publication

A fixed transport-owned server opening precedes initialization, config work and
constructor/provider callbacks. The original TLS/config/raw/address and partial
baseServer remain attached. Transport metadata locks no longer enclose init or
logger/address callbacks; the constructor attaches its object before WithPrefix
and no longer starts tasks. Existing packetaddr capture supplies bounded frozen
log metadata and rejects known invalid addresses before any server worker starts.

Publication of the original server and both existing worker starts shares the
mutex/access seal fence with Close. A known refused unstarted candidate needs no
fabricated loop shutdown. Root cleanup joins the actual opening before map/server
snapshot, tracer close or borrowed-deadline restoration. An abnormal callback
seals admission and retains its original prefix with a negative outcome, without
claiming that its external work stopped. Concurrent creation refuses at one cell;
replacement still needs the previous original server's closeRetired outcome.

No new worker class, timer, protocol algorithm, public API, dependency or license
change. Private constructor/caller changed together; omitted upstream tests and
private mocks were not run or updated. Provider self-Close remains unsupported;
init/wrap/custom-provider/trace/path/global/native/OS/full-PC gates stay OPEN.
No tests/fixtures/build/product/native/live execution or speed proof. Receipts:
`.artifacts/memory/bootstrap-v426-server-opening-`.

## Revision29 — CID admission before socket initialization

The original initializer captures the configured generator before its one length
callback and checks the existing protocol 0..20 range before socket wrapping,
buffer tuning, transport queues and worker scheduling. Custom-generator precedence
and the existing default/zero-length choices are preserved. Closing is observed
after the callback and before wrapping; it is not atomic native cancellation.
Invalid lengths return the existing ErrInvalidConnectionIDLen.

The existing protocol.GenerateConnectionID also rejects negative or oversized
lengths before uint8 conversion, buffer slicing or random reads. It returns a zero
CID with the same error; valid lengths keep the original crypto/rand algorithm.
No public API, version, license, worker/timer or quota change. Full init/wrap/
provider/factory/selfClose/path/global/native/OS/full-PC acceptance stays OPEN.
No tests/fixtures/build/product/native/live execution or measured speed claim.
Receipts: `.artifacts/memory/bootstrap-v427-cid-init-`.

## Revision30 — convenience setup failure cleanup

DialAddr/Early share one original address path: TLS admission and resolution
precede owned socket acquisition, and an original Transport exists before native
bind. ListenAddr/Early likewise validate TLS before DNS/bind. A returned nonnil
socket is attached before subsequent setup; known setup failures use that exact
transport's cached Close. Dial/Early no longer discard the cleanup result, and
Listen/Early now retire their hidden transport on a known listener setup error.

When cleanup succeeds the original setup error is preserved unchanged. On a
cleanup error, a private error keeps the exact Transport, cause and cleanup result;
Unwrap preserves both identities while its message uses fixed text. Retention
lasts while that error is held: this is not a process-wide retired inventory,
automatic retry suppression or proof that an opaque provider stopped. Owned vs
borrowed socket behavior remains in the existing lifecycle Close owner; no second
native cleanup, timer, worker or timeout is added. Abnormal factory/provider exits,
init/wrap/global/OS/Stop/full-PC acceptance remain OPEN. Public API signatures,
versions/licenses/earlier quotas remain. No tests/fixtures/build/product/native/
live execution or speed proof. Receipts:
`.artifacts/memory/bootstrap-v428-setup-cleanup-`.

## Revision31 — observed Windows DF capability admission

The wrapper no longer defaults DF support to true without native setup. Opaque
PacketConns and non-UDP metadata do not prove this capability. Windows setDF
records the actual nil-error outcome of each existing socket-option attempt and
returns their OR instead of unconditional success. Both failed or missing callback
observations yield false; the original Control error still returns false/error.
One-family success and both existing native attempts/diagnostics are preserved.

The existing basic/sendConn capability and handshake-confirmed PMTU gate consume
that result; there is no second path algorithm, network fallback or new worker.
This does not prove socket-option readback, correct family/dual-stack behavior,
new hopping sockets, migrated paths, rawConn bypass providers, kernel/network
behavior or a performance gain. Full init/provider/PMTU/global/OS/full-PC gates
remain OPEN. Other platform-specific setDF implementations were not changed.
Public signatures/versions/licenses/quotas stay; no tests/fixtures/build/product/
native/live execution. Receipts: `.artifacts/memory/bootstrap-v429-df-admission-`.

## Revision32 — original initializer provider prefix

The existing lifecycle/initOnce embeds one transportConnOpening before the first
generator/provider callback. Actual initialization and raw bypass use its exact
captured socket, matching the original abort owner instead of rereading public
Conn. The prefix attaches returned LocalAddr before Capture, the original
SyscallConn before buffer/DF Control, and partial wrappers before logger calls.

Both buffer helper source and its write mirror carry the same original prefix.
The mirror was edited together with its template; no generator was executed.
Windows newConn was inspected as a direct basicConn constructor; non-Windows
constructor internals have not received a new prefix contract here. Ordinary
initializer return clears the prefix after t.conn/worker publication or known
error. Panic/Goexit retains the original prefix and uses the existing negative
initDone outcome; it is not a new cleanup retry or provider-stop receipt.

Only the original initializer writes prefix fields; metadata locks do not span
providers. No new goroutine/channel/map/admission class, native option, timeout,
public API/version/license/CC/crypto change. External providers must still honor
synchronous callbacks; opaque work/global retention/native/Stop/full-PC gates
remain OPEN. No tests/fixtures/build/product/generator/native/live execution.
Receipts: `.artifacts/memory/bootstrap-v430-init-custody-`.

## Revision33 — PMTU reset admission and original probe identity

PMTU Reset previously set a nonzero probe time unconditionally, rearming discovery
after migration even when configuration or DF admission had refused it. Its two
actual migration callers now pass the existing handshake/config/actual sendConn
capability gate explicitly; client switching uses the prospective nextConn. The
same private gate is used for initial Start and again before a due probe is packed.
Refused reset leaves the schedule disabled while preserving min/max reset logic.

Probe callbacks now bind the existing handler object's identity instead of a
wrapping uint8 generation. Reset and a known ACK/loss invalidate that exact probe;
old or duplicate callbacks have no authority over a later one. Handler allocation,
RTT delay, search bounds/loss slots, frame/output custody and payload algorithm
remain. This does not establish current-family/readback/hopping or full migration/
native correctness; provider/callback/global/OS/full-PC gates stay OPEN. Private
Reset and production callers changed together; omitted tests/mocks were untouched.
Public signatures/versions/licenses/quotas remain; no new worker/timer/crypto/CC
algorithm, tests/fixtures/build/product/native/live execution or speed proof.
Receipts: `.artifacts/memory/bootstrap-v431-pmtu-reset-`.

## Revision34 — original path-switch candidate and queue task notification

The existing client switch is isolated in connection_path.go. One embedded prefix
retains target/raw/remote, old conn/queue and returned candidate conn/queue before
provider calls and old queue Close. Known completed outcomes clear it; negative
old-queue retirement or abnormal switching retains it and marks connection-loop
custody uncertain. Loop completion also checks that no active candidate remains.

Target capture and conn/queue publication use mutex->lifecycle.access seal and
closeErr fences with providers/statistics/PMTU/old Close outside the locks. The
existing worker start shares publication. An unstarted known-refused candidate
does not close shared raw state or join an invented queue task. Initial and migrated
queues use one deferred notification wrapper: normal Run errors and abnormal
Goexit/panic request the original Conn close; panic behavior still propagates.
The queue's existing packet-custody and cached Close outcome remain authoritative.

No extra worker/timer/retry/cleanup class, public API/version/license/CC/crypto
change. Constructor internals, per-family/hopping capability, target CID removal,
multi-transport ownership/global retention and native/OS/Stop/full-PC gates remain
OPEN. The target fence is metadata admission, not atomic OS-path/Conn cancellation
proof. No tests/fixtures/build/product/native/live or measured speed claim.
Receipts: `.artifacts/memory/bootstrap-v432-path-switch-`.

## Revision35 — send constructor custody and published metadata view

All three production newSendConn callers supply their existing parent opening.
The fixed prefix attaches raw/remote/local/adjusted address/OOB and a partial sconn
before LocalAddr/Capture callbacks. Server opening now precedes this constructor;
normal parent completion clears it, abnormal parent guards retain its originals.
Shared raw state still belongs to its original owner and is not closed as a
partially built sender. Private constructor and production callers changed together.

Conn publishes an immutable sender view atomically before constructor callbacks
and at the existing target-sealed path commit. The initial view is embedded and
keeps that sender for Conn lifetime; migration uses an independent node so later
prefix clearing cannot mutate a published view. LocalAddr/RemoteAddr/SetRemoteAddr
and ConnectionState's GSO select one original sender through that view, avoiding
unsynchronized reads of the loop's conn field. SetRemoteAddr remains a point-load
operation, not a new migration-command serialization contract. gotGSOError is now
atomic across worker and metadata/connection-loop readers. wroteFirstPacket remains
unchanged after tracing serialized Write/terminal-write ordering; probes use WriteTo.

No new worker/timer/native retry/lock/public signature/version/license/crypto/CC
change. Remote-address aliasing, provider lifetime beyond callbacks, full metadata
API/target CID/multi-owner/global/OS/Stop/full-PC gates stay OPEN. Fixed prefix/view
costs and migration-node allocation are unmeasured; no race/runtime/native or speed
proof. No tests/fixtures/build/product/native/live execution. Receipts:
`.artifacts/memory/bootstrap-v433-send-metadata-`.

## Revision36 — additional-path original connection ownership

Each target Transport reserves one of 64 fixed registration cells before CID
callbacks. The original Conn, callbacks and new-binding outcome stay attached
through completion. A separately promoted Conn set survives removal of every
CID/reset row; promotion shares the target seal fence and observes the original
Conn close reason. Known refusal rolls back only a newly added runner before
clearing its cell; it does not close the old valid connection for an unactivated
target. Abnormal callback/rollback completion retains the cell and seals/aborts
its original target with an uncertain result.

Root cleanup joins these registrations before snapshotting and deduplicates the
strong path set with existing CID/reset/client owners. Rows are removed only
after actual original input/stream/call retirement, using the ended Conn loop's
immutable runner identities and idempotent locked metadata deletion. Trace close
also requires no pending registrations or retained path owners. AddPath checks
successful handshake/closing/nil target before native init and rechecks closing
after init. The private AddConnRunner reports whether it added a new binding;
its production caller and original rollback logic change together.

No extra worker/native cleanup retry/public signature/version/license/CC/crypto
change. The 64-cell admission adds metadata and one completion channel; it does
not bound all connections/runners/paths or global bytes/RSS/CPU/Stop time. Path
Close still has no general runner-detachment contract. Reset-token migration,
aliases/provider/full metadata/native/OS/full-PC gates remain OPEN. No tests,
fixtures, builds, product/native/live execution or measured speed claim. Receipts:
`.artifacts/memory/bootstrap-v434-path-ownership-`.

## Revision37 — reset tokens across registered path transports

Both actual Conn constructors route the destination-CID manager's reset-token
add/remove callbacks through the existing connRunners keys and existing runner
interface. Initial and additional transports therefore receive normal rotation,
path retirement and Conn-close removals. Same-owner idempotence and foreign-owner
collision refusal remain in packetHandlerMap. A known fanout refusal requests
the existing original Conn close; partial rows stay owned for normal removal.

A newly added runner receives the manager's existing active/probing tokens before
V36 path-owner promotion. A known token or promotion refusal removes target token
rows before original CID rollback and only for that new runner. Abnormal callback
or rollback retains the original admitted cell/Conn/manager/binding. The manager
is iterated by its original connection loop without a copied token inventory;
callbacks visit their target only. The existing failed-path-enable branch already
retires its probing CID and is unchanged.

No new callback fields, worker, lock, channel, native retry, public signature,
version, license, crypto or CC change. Token recognition is not peer identity,
path validation, protected egress or a native delivery receipt. All paths/runners,
duplicate-token policy, asynchronous custom runner behavior, global resources,
aliases/provider/OS/full-PC gates remain OPEN. No tests/fixtures/build/product/
native/live execution or measured speed claim. Receipts:
`.artifacts/memory/bootstrap-v435-path-reset-`.

## Revision38 — caller abandonment and loop-owned path CID retirement

Path.Close only marks the existing path row abandoned and closes its existing
abandon signal under the path-manager mutex. Concurrent closes cannot double
close that signal; probing cannot recreate an abandoned Path. CID/reset/frame
retirement is performed by the original connection loop before output-pressure
checks, using that same retained row rather than appending a retirement queue.
One pending flag reuses the existing switch-check lock and skips path scans when
there is no abandoned CID; no second per-iteration lock is introduced.
Active-path Close refusal is preserved. Close acknowledges abandonment, not
synchronous CID/native cleanup.

An explicit cidAssigned flag survives validation clearing pathChallenges and
precedes enable callbacks. Known failed enable clears it only after the existing
retirement returns. Unknown callbacks keep the row/original loop outcome negative.
Original Conn Closing and Context.Done are assigned before atomic manager
publication; they gate new paths/probes/switches and wake Probe on normal or
abnormal terminal completion. Terminal caller-side Close changes metadata only;
the existing original connIDManager.Close remains the token cleanup authority.

No new worker, timer, lock, channel, native retry, public signature, version,
license, crypto or CC change. Pending rows/markers are not a global path quota
or a retirement deadline. Existing duplicate/concurrent Probe, mutable validation
channels, probe queue/challenge bounds/backoff, runner detachment, aliases/provider/
native/OS/full-PC gates remain OPEN. No tests/fixtures/build/product/native/live
execution or measured speed claim. Receipts:
`.artifacts/memory/bootstrap-v436-path-close-`.

## Revision39 — admitted concurrent probes and bounded attempt state

Path.Probe reserves one of 64 original call cells before context callbacks,
attempt/channel/map/timer allocation. Each call retains its original context,
Path, row, stable attempt and reusable channel timer. Concurrent calls share one
attempt; its result channels never change while registered waiters exist. A
later re-probe gets a fresh attempt after the previous calls retire. Cancelling
one call preserves the others; the last ordinary return removes queued work.
Unknown callback/return keeps its cell and makes the original call join negative.

Internal path rows are capped at 64, the unique probe queue is a fixed 64-ID
array, and each attempt retains a ring of 32 challenge values. Shared cadence
coalesces caller timers and saturates exponential backoff at 30s, preserving the
public 200ms initial value. Each admitted caller reuses at most one timer. The
original loop rotates through at most one fixed queue pass so CID starvation for
a new path does not block a re-probe that already owns a CID. Wrapped/nonpositive
path IDs are refused. Terminal/abandon state is rechecked before returning a
successful validation. Previously validated path authority is preserved during
a new probe attempt; a new response is still required for that attempt's success.

WaitForPacketInputShutdown also joins admitted Probe calls, outside packet metadata
locks. Normal return stops the original timer outside the path mutex before cell
reuse; an uncertain call prevents positive Transport/Conn retirement. One manager
completion channel and fixed metadata are added, with no extra goroutine/native
retry/public signature/version/license/crypto/CC change. Private helpers changed
with their actual callers; tests/fixtures are untouched.

These are per-manager state bounds, not a cap on externally retained unprobed
Path/context object graphs, lifetime runner bindings, global/RSS/CPU/FD/Stop time
or native delivery. Old challenge responses beyond the 32-value window are
ignored; the choices and shared cadence need compatibility/performance acceptance.
Cross-transport path-response binding, general runner detachment, callback self-
join/provider async, aliases/native/OS/full-PC gates remain OPEN. No tests/fixtures/
build/product/native/live execution or measured speed claim. Receipts:
`.artifacts/memory/bootstrap-v437-path-probes-`.

## Revision40 — responder path reuse and correction of the proposed response filter

The existing server path manager now exits its search after finding/moving an
existing row. Its maxPaths/age/eviction branch applies only when a new row is
needed. A repeated challenge on a known path at the three-path ceiling therefore
does not get rejected as a new path or retire another original CID unnecessarily.
The existing limit, timeout, CID admission and frame construction remain.

The proposed incoming-Transport PATH_RESPONSE filter is explicitly rejected:
RFC9000 sections8.2.2-8.2.3 require the responder to use the challenge path, but
forbid the initiator from enforcing the response's arrival path. A matching
response received elsewhere still validates the original challenge path. Existing
nonce-based incoming response acceptance is preserved. This corrects the V39
next-step premise, not an observed defect in that acceptance rule. Primary source:
https://www.rfc-editor.org/rfc/rfc9000.html#section-8.2.2 .

Client responder routing through the ordinary active-path framer, origin/CID/
packetInfo-aware queued output, response padding and native path correctness
remain separate OPEN work. No new worker, retry, queue, limit, API, version,
license, crypto or CC change. No tests/fixtures/build/product/native/live or
measured speed proof. Primary RFC reading is not a VPN/server test. Receipts:
`.artifacts/memory/bootstrap-v438-path-reuse-`.

## Revision41 — retained response routes in the existing bounded framer queue

The original Transport reader stamps incoming datagrams with its own identity;
Clone and ordinary queue/coalescing copies preserve it. Both actual constructors
capture their initial Transport before trace callbacks. Client challenges and
server challenges from the active peer now enter the existing 256-response queue
with original Transport/remote/packetInfo. Admission precedes frame allocation.
Explicit legacy QueueControlFrame entries retain their ordinary active-path
meaning; generic packing never consumes a routed entry. There is no second queue.

One original Conn response prefix owns dequeue through peer-CID selection,
packing, tracing and actual write. Its independent frame array can remain with
sent history after prefix reuse. Replies use a current original or registered
path CID; a stale/unavailable route is dropped without choosing another path.
They drain before outgoing probes through SendModeAny/pacing, are padded by the
path packer, and use normal shared-Conn flight accounting. The special challenge
category is cleared because it bypasses that accounting. No separate per-path
RTT/congestion-control accuracy or optimal scheduling is claimed.

A common original packet-output helper serves probes and replies. Routed writes
reuse Transport's existing 64-call/4MiB admission with retained packetInfo/OOB;
public WriteTo keeps its signature/default. Known return releases only the
original prefix; abnormal result keeps response/output/write originals negative,
and Conn retirement checks the response prefix too. The debug message no longer
reports the active socket as the alternate packet's actual local source.

No new queue/worker/timer/native cleanup retry/public signature/version/license/
crypto/CC algorithm. Server non-active-peer responses/anti-amplification, queued
stale/no-CID reply policy, initial-versus-migrated data CID privacy, independent
path RTT/CC, opaque remote aliases and kernel local-IP selection (Windows OOB
remains unavailable) require separate acceptance/implementation. RFC's matching-
nonce acceptance from any return path remains unchanged. No tests/fixtures/build/
product/native/live execution or measured speed proof. Receipts:
`.artifacts/memory/bootstrap-v439-path-replies-`.

Revision42 gives the existing at-most-three non-active server path records their
original Transport/remote/packetInfo and saturating three-times received-byte
credit. The original authenticated input-processing anchor contributes a whole
datagram once, including coalesced input. One original Conn prefix reserves the
credit before CID/packer/trace/write callbacks. Known unused packing credit or
known pre-output refusal returns only to that original record; attempted output
and unknown results retain the charge. The original routed Transport output and
64-call/4MiB native write owner are reused. Loop retirement includes the prefix.

The private sized packer preserves the normal 1200-byte default. When credit is
smaller it emits the minimum valid packet, or refuses a packet that cannot fit.
Address reachability and MTU validation are separate: a matching small challenge
response requires a fresh expanded follow-up before the existing migration gate
can pass. The original loop drains pending follow-ups/retries; no new timer or
worker. Each record keeps a fixed32 history of successfully sent nonces and their
actual sizes. Loss renews the pending challenge instead of immediately deleting
the path/CID, so delayed matching responses can still validate the original path.
Switch selection compares the highest non-probing application-data PN, including
server 0RTT, instead of allowing newer probing packets to outrank migration intent.

This covers authenticated server migration-path probes, not initial-handshake
or DisablePathManager output, discarded-datagram attribution, or global RFC
anti-amplification compliance. The existing one-second loss signal is not a
complete validation timeout: ACK without PATH_RESPONSE, backoff/abandonment and
MTU/history expiry require follow-up. Multiple challenges/server routed-response
queue integration, CID promotion/privacy/lifetime, independent path RTT/CC and
kernel source selection remain OPEN. Special challenge accounting is retained;
no full per-path congestion claim. No public API/version/license/crypto/CC
algorithm, worker/timer, tests/build/native/live execution or speed claim.
Receipts: `.artifacts/memory/bootstrap-v440-path-credit-`.

Revision43 moves server validation retry and abandonment timing out of packet
ACK/loss history into each original bounded path record. The existing Conn
timer/loop handles the next response deadline even when packet output is blocked.
ACK alone never validates or cancels the path clock. The old one-second packet
loss callback now retires only packet history; it cannot accelerate path retries.

Each address/MTU phase permits at most three successful own challenges, with
one/two/four-PTO response windows and a seven-PTO phase budget. A new path uses
at least one second plus peer ACK delay or the larger current RTT-based PTO.
Future-time arithmetic saturates; a larger observed PTO can extend a phase but
duplicates cannot restart it indefinitely. Admission waiting for CID/credit has
an expiry; first actual successful output receives a complete response window.
Pending unsent work has no past retry alarm, avoiding a timer loop under pressure.

Small address validation starts the full-size phase once, preserving sent nonce
history and any already outstanding expanded challenge/cadence. Full validation
clears both deadlines; expired responses cannot revive a phase. Expiry retains
the original row before CID/reset callbacks, removes it only after known return,
and never changes the current working peer. Partial CID retirement remains
attached and prevents positive loop retirement. New-path admission cannot evict
an unexpired validation merely because its last input was over five seconds ago.

No new timer/worker/queue/public API/version/license/crypto/CC algorithm. These
are source scheduling/count bounds, not native wall-time, global PTO, path RTT,
packet-history retirement, RFC compliance, speed or wire evidence. Initial/bypass/
global amplification, every-challenge routed replies, selected data CID/privacy/
runner lifetime, old-path defense, per-path CC and kernel source remain OPEN.
No tests/build/product/native/live execution. Receipts:
`.artifacts/memory/bootstrap-v441-path-timeout-`.

Revision44 routes every parsed normal server non-active PATH_CHALLENGE through
the existing256 response queue. The original input owner admits/credits its
bounded3 path after authentication and duplicate rejection, before frame
callbacks. Queue entries retain the original route and scalar path ID, not a
borrowed path pointer. Production output no longer depends on the parser's last
challenge return or performs immediate server output inside packet processing.

Own validation and queued replies share the original server output reservation,
limited packing and routed write custody. Replies use ordinary shared-Conn
flight accounting under SendModeAny/pacing. A due own challenge can accompany
one response on the same path, so response pressure cannot indefinitely starve
validation. A known packing-credit refusal tries only the response before any
native write. Only an actually sent challenge changes its counter/nonce clock.

Known no-CID/credit/fit replies rotate in the same queue. A drain scans at most
the original queue length, allowing other routes/data to progress without an
immediate retry schedule or overflow queue. Stale original IDs are discarded,
never rebound to a replacement row at the same address. A known promoted path
ID permits its queued replies to retain their original path CID. Unknown output
keeps both original prefixes/debit and now always propagates an error.

The256 queue/full-requeue refusal policy and explicit legacy entries remain.
Initial/DisablePathManager amplification and bypass replies, queue terminal
metadata retirement, normal data-CID promotion/privacy/general runner lifetime,
per-path RTT/CC, kernel source and whole-PC acceptance remain OPEN. Retaining
route metadata does not prove native source selection, hard memory/CPU bounds,
fairness/latency optimality or RFC compliance. No new queue/timer/worker/public
API/version/license/crypto/CC algorithm, tests/build/native/live or speed proof.
Receipts: `.artifacts/memory/bootstrap-v442-server-replies-`.

Revision45 retires path-response metadata in the original Conn's final guard,
after known loop/drain/send-queue/close callback return and quiescent original
input/packer/output prefixes. It seals routed/deferred/legacy response admission
under the existing framer mutex, clears queued route/frame references and drops
their backing slice. Quiescent server rows and no-longer-callable CID/logger
closures are cleared too, including managers retained by sent frame handlers.
The promoted server path ID is invalidated; immutable public sender metadata,
outgoing public Path/Probe state and general stream controls stay separate.

The final parent flag becomes negative before entering metadata cleanup. A
nested publication guard still reports negative input/stream-loop completion
after abnormal cleanup; no positive input Done precedes this retirement. An
original retiring/retired marker prevents unknown cleanup replacement. The
existing WaitForPacketInputShutdown API requires the known metadata marker and
retains its separate outgoing Probe-call join. No new native cleanup, CID/reset
callback, worker, timer, queue or public API is introduced by retirement.

The canonical releaseClosedPacket now clears its original Transport stamp after
the actual pool-return callbacks return. Previously its data/address were clear
while the Transport reference remained, including in the terminal draining slot.
The added source snapshot was captured and checked against frozen original
bytes before that correction; initial snapshots/receipts are preserved.

These are source lifetime boundaries, not whole-framer/stream/ACK/crypto/runner/
raw-provider/native/global retirement, RSS, Stop wall-time or speed evidence.
Initial/bypass amplification, data-CID promotion/privacy, per-path RTT/CC/kernel/
OS/UX/release/full-PC gates remain OPEN. No tests/build/native/live execution.
Receipts: `.artifacts/memory/bootstrap-v443-path-retirement-`.

Revision46 binds the original framer to the existing stream-call owner during
Conn construction. After the original loop and all six registered stream-call
classes return, that loop or the last caller reserves one metadata finalizer.
No registry mutex is held across framer locks; stream wrappers have already
released their stream mutex/gates. No worker, cross-join or wait inside an active
loop is added. Positive allDone waits for actual framer metadata retirement.

Cleanup requires the prior path-response retirement, seals generic frame/stream
admission, clears active/control stream maps and queued frames, drops ring backing
storage and its flow-controller reference. Late queue, Append, stream registration
and 0RTT calls cannot refill closed metadata. Append/0RTT/cleanup use the same
control-then-stream mutex order. The overflow diagnostic uses atomic Load/Store
instead of an unsynchronized hot-loop read.

Unknown cleanup retains the original framer pointer and retiring marker, with
separate framerFailure. The receive-only receipt keeps its original scope;
WaitForStreamCallsShutdown adds the sealed-framer receipt without a signature
change. Automatic cleanup also works when nobody calls the waiter, once actual
producers return. Existing HY2 close already consumes that whole-call waiter.

Stream/ACK/crypto/window-update/outgoing/public handles, raw/native/runner/global
retirement remain separate. No RSS/latency/Stop deadline or speed claim, public
API/version/license/crypto/CC algorithm change, tests/build/native/live execution.
Initial/bypass amplification/dataCID/privacy/per-path CC/kernel/OS/UX/release/
full-PC gates remain OPEN. Receipts:
`.artifacts/memory/bootstrap-v444-framer-retirement-`.

Revision47 gives the original parent streamsMap fixed512 Open/Accept call slots,
reusing the existing data-call ceiling. Admission precedes parent/child locks,
context methods and wait-channel allocation. Each original invocation records
its actual context, child map, candidate and error; the registry survives 0-RTT
child replacement. Normal pressure refuses only the extra caller. Abnormal
return retains the original slot/references, seals admission and requests the
existing connection close outside the metadata lock. Actual running includes
that notification; an unknown invocation cannot publish positive completion.

Admission and successful result publication observe the original atomic first-
close record, before the later Closing notification. A late candidate is not
published or closed again by the wrapper; any still-live stream remains its
child map's native-shutdown responsibility. This does not correct old-generation
ID callbacks, partial factories or establish retirement of retained map rows.

New Conn.WaitForStreamMapShutdown joins the original loop and actual admitted
Open/Accept invocations. HY2's original external close task calls it alongside
the existing input, receive, all-stream-call and send-data joins. The six-class
WaitForStreamCallsShutdown contract is unchanged. The new waiter must not be
called from one of its own operation callbacks; no self-wait is added to run.

Outgoing Sync also caps its own waiter queue before channel allocation. Context
Err/Done execute outside child map locks, with explicit held-state unwind guards.
Accept preserves available-first behavior and reads Done only when it must wait.
Nil context is rejected; a closed Done with nil Err returns context.Canceled
instead of a zero stream with nil error. No worker, timer, merged context,
module/version/license/crypto/CC algorithm change or measured speed claim.
Factories/native stream shutdown callbacks under map mutexes, skipped incoming
IDs, retained rows, 0-RTT generation custody and global/native/OS/full-PC gates
remain OPEN. No tests/fixtures/build/product/native/live execution. Receipts:
`.artifacts/memory/bootstrap-v445-stream-map-calls-`.

Revision48 bounds each original incoming stream map to4096 charged rows and
16MiB logical metadata credit (4KiB per row). This is not RSS or payload backing;
data has its separate existing budgets. A whole skipped-ID batch is reserved
before any factory or map insertion. Existing wire stream limits stay unchanged;
a valid peer ID exceeding local resources gets an InternalError, not a false
STREAM_LIMIT_ERROR accusation. Completed unaccepted rows remain charged, and
only actual deletion or a known never-created suffix refunds row credits.

An original factory/native-close gate serializes those operations without
holding the map metadata mutex across callbacks or gate waits. Creation keeps
its exact batch/ID/factory/candidate/result and unspent reservation. Each known
row advances nextStreamToOpen when inserted; the requested original result is
retained separately from a row that Accept/Delete may concurrently consume.
Known close stops further creation, keeps a returned candidate map-owned for
native close and refunds only its uncreated suffix. Abnormal factory/publication
retains the original prefix/credits and seals/wakes the map without a retry.

Close seals Get/Accept/Delete before waiting for the original factory and sends
the existing wake notification once. The sealed map has no writers, so native
close walks its original rows without an extra snapshot. Each original retiring
stream is stored before invoking shutdown outside the metadata mutex. Unknown
callback keeps its pointer and unfinished close reservation. A known repeat is
idempotent; an unfinished/uncertain repeat is refused instead of replaying cleanup
or publishing success. Rows are retained after callback return: a void return
is not proof of native resource or all-stream-call retirement.

No worker, timer, waiter queue, module/version/license/crypto/CC algorithm change
or measured speed claim. Opaque factories/native callbacks can still block actual
close indefinitely, including their own operation gate; no deadline is asserted.
Outgoing map factories/native close, partial constructor internals, retained rows,
old-generation ID callbacks and aggregate0RTT/global/native/OS/full-PC acceptance
remain OPEN. No tests/fixtures/build/product/native/live execution. Receipts:
`.artifacts/memory/bootstrap-v446-incoming-map-lifetime-`.

Revision49 applies the same4096-row/16MiB logical metadata policy to outgoing
maps before any factory/insertion. Direct local pressure refuses one operation;
Sync waits in the existing bounded FIFO queue for local and peer capacity.
Actual deletion wakes the eligible head, as do peer credit and known factory
completion. Only the exact head can reserve a new ID; direct callers cannot
overtake it. Queue removal clears the old channel reference. Admission pressure
is an explicit result, so a close cause with a limit-like type cannot cause a
closed-channel retry loop. Context callbacks stay outside map/operation locks;
no new queue, worker, timer or pressure polling is introduced.

The original factory/native-close gate is never waited under the metadata
mutex. Factory records retain exact ID/callback/candidate/waiter/credit before
callbacks; abnormal construction/publication keeps that original prefix. A known
late candidate is inserted in the original map for its native close, without
caller-side cleanup. Seal wakes queued callers before waiting for the factory,
fences Get/Delete/SetMax/update admission, and native close walks stable original
rows without a snapshot. Its retiring stream remains held through the actual
callback. Known repeated close is idempotent; unfinished repeat refuses rather
than replaying cleanup. Rows remain retained after void callback return.

Window/reset updates acquire the same operation gate before their existing
metadata lock so an in-flight factory candidate cannot miss the update. Their
callbacks and blocked-frame queue callbacks still need a separate prefix/lock
review. Exact incoming/outgoing uncertainty sentinels seal the original public
map-call registry and request existing root close outside its mutex, while the
original invocation is still running. No foreign errors.Is dispatch is used;
normal pressure, wire limit and cancellation do not seal a working carrier.

No API/module/version/license/crypto/CC algorithm changes, tests/build/native/
live or measured speed/RSS/Stop proof. Opaque factories and native providers can
still block the original operation gate. Partial constructors, control/update
callbacks, retained rows, old-generation ID callbacks and aggregate0RTT/global/
native/OS/full-PC acceptance remain OPEN. Receipts:
`.artifacts/memory/bootstrap-v447-outgoing-map-lifetime-`.

Revision50 moves outgoing window/reset stream callbacks outside the map metadata
mutex. Their existing original operation gate still orders updates with factory
publication and native close, so an in-flight candidate is not missed. One
original update record reserves at most4096 elements and64KiB of element backing
before snapshot allocation. Actual generic element size is checked by division
before multiplication; no assumed pointer size or unbounded append is used.

Factory admission is excluded by the original gate while Delete may only shrink
the map. Original stream identities are captured under the metadata lock into
the reserved capacity, then callbacks run without that lock. Each current stream
is retained before its callback, cleared only after known actual return. Deleted
rows cannot substitute a different target. Unknown capture/allocation/callback
retains original kind/limit/snapshot/current/index/backing credit, seals the map
and cannot be replaced or retried. Close refuses an active unfinished update.

Known close can stop an uncalled tail; known finish clears the private references
before releasing the operation gate. This is metadata lifetime, not proof of
native stream/GC/RSS retirement. Existing native six-class/map/input/data joins
retain their separate scopes. No worker, queue, timer, public API, module/version/
license/crypto/CC algorithm change, tests/build/native/live or measured speed proof.
Opaque callback/self-reentry may still block the original operation gate.
STREAMS_BLOCKED/MAX_STREAMS callbacks, partial constructors, parent parameters,
old-generation IDs and retained-row/global/OS/full-PC acceptance remain OPEN.
Receipts: `.artifacts/memory/bootstrap-v448-outgoing-updates-`.

Revision51 moves outgoing STREAMS_BLOCKED and incoming MAX_STREAMS callbacks
outside map metadata and factory/update locks. Each original map retains one
actor/current frame/callback and one monotone coalesced pending scalar. Current
admission precedes frame allocation. The actor gate is reserved with nonwaiting
TryLock under metadata; gate release/running transition use that same metadata
lock, avoiding a lost pending handoff. A reentrant request records pending and
returns; its original actor drains it without another worker or queue.

Exact current frame/callback survive abnormal allocation/callback return; the
map seals and the unknown actor is not replaced or retried. A known callback
return releases only this original reference, not wire-delivery/native proof.
Outgoing deferred dispatch follows factory gate release; native SetMax and
incoming Accept/Delete also release metadata before dispatch. Control close
joins the actual original actor before waiting for the factory/update gate,
holding neither metadata nor factory lock during that join. An unfinished
negative actor refuses success; only a known uncalled pending scalar is cleared.

This control join precedes the existing old-map Close return and subsequent
0-RTT framer reset, preventing that old actor from refilling the new framer.
It does not join all old per-stream completion/ID callbacks or solve parent
parameter/generation custody. Incoming MAX_STREAMS bounds its count before ID
multiplication/addition, including exhausted-ID limits, rather than allowing
signed arithmetic to wrap a new wire limit.

Actual callbacks only queue a frame and schedule the existing Conn loop. Native
close must be external to its own control callback; arbitrary providers can
still block indefinitely. No worker/timer/event queue/public API/module/version/
license/crypto/CC algorithm changes, tests/build/native/live or speed/RSS/Stop
proof. Partial constructors/parameters/stale0RTT IDs/retained rows/global/native/
OS/full-PC acceptance remain OPEN. Receipts:
`.artifacts/memory/bootstrap-v449-stream-map-control-`.

Revision52 binds each original stream-map generation to a nonzero immutable
object token and four child-specific senders. Each sender captures its exact
original child's Delete method before factory/public exposure. All four actual
Stream/SendStream/ReceiveStream factories use those senders, including existing
bidirectional forwarding and later ACK/loss notifications. Completion no longer
looks up the current parent map by reused StreamID. One retained stream points
to its original child binding, without a generation router retaining all four
old child maps. Original child rows still need their own retirement receipt.

Framer data/control registration and active removal compare the original token
inside their existing matching metadata locks. Token swap and old map cleanup
hold both locks, preventing a stale check/use from touching replacement IDs.
Atomic token reads allow the connection-data scheduling hint without another
mutex; such a hint carries no stream/data identity and is not a flow-epoch fence.
Existing unscoped private Conn/framer seams remain, but actual factories use
the scoped path; this is not a claim about arbitrary private providers.

Conn constructs framer before maps, requires its original/error bindings, and
sets the initial token before exposure. 0-RTT reset passes the exact next token,
clears old active data and stream-control getter maps under both locks, then
publishes that token. Previously streamsWithControlFrames was not cleared.
Terminal known framer retirement clears the token too. Private constructor/reset
signatures change together with their sole production Conn caller; public APIs,
HY2 consumer, module/dependency versions and shutdown join scopes are unchanged.

No new worker/timer/queue, license/crypto/CC change, tests/build/native/live or
measured speed/RSS/Stop proof. Parent mutable parameters/reset support, old flow-
controller mutation after reset, partial constructor/context custody, retained
rows and global/native/OS/full-PC acceptance remain OPEN. Notification fencing
does not prove those old calls or their resources ended. Receipts:
`.artifacts/memory/bootstrap-v450-stream-generations-`.

Revision53 copies actual stream windows/counts/reset support into immutable
scalar snapshots. Each original generation publishes received/incoming and
applied/outgoing heads separately: client inbound 1-RTT sees validated received
parameters immediately, while outbound 0-RTT retains restored limits until apply.
All four factories load one original snapshot; the private flow factory takes
the captured initial send window instead of reading mutable Conn.peerParams.
Native reset updates require the negotiated flag. Both frozen wire 0-RTT
validation paths reject remembered reset support becoming disabled, following
the already-pinned reliable-stream-reset draft06 section3; no wire ID/version
or dependency update. Missing snapshots fail the original constructor, without
synthetic defaults. Local sing-quic revision61 is an unchanged consumer.
Source/manual review and touched-only gofmt only; no tests/build/native proof.
Other peerParams readers, old connection-flow counters after reset, partial
constructors and actual retained-row/resource retirement remain open.

Revision54 binds native stream flow controllers to a by-value original
connection-flow owner/epoch scope. A nonzero immutable token changes only on
successful 0-RTT Reset under the original connection-flow mutex. All five actual
stream-to-connection operations check identity and access counters/window/tuning
under that same mutex. Scope capture reads the atomic token without locking:
reentrant window-increase constructors must not relock the callback's mutex;
capture alone grants no counter access. Old scopes grant no new credit/window or
mutation; they do not reset local stream state or retire old resources.
Original native connection send/block/window methods now share the mutex;
existing receive mutation bodies are reused through lock-held helpers.
The stream scope adds no explicit separate adapter allocation, timer or worker.
Private binding compatibility, reentrant callbacks, extra hot-path locking cost,
partial constructors and row/native retirement remain unverified/open.
Public API, dependency/wire version and sing-quic consumer are unchanged.
Source/manual review and touched-only gofmt only; no tests/build/native proof.

Revision55 retains a fixed partial-construction prefix in each original stream
map creation record. All four native factories bind the exact original child
prefix before exposure; generic map factory signatures stay unchanged.
Known flow result, outer stream, halves and sender wrappers are retained before
later channel/context/sorter work. The original ReceiveStream stores its sorter
shell before tree/map/insert initialization; generic crypto sorter semantics
retain the same native initializer. Returned context/cancel pairs remain on the
already-retained SendStream. Unknown creation/insertion retains original prefix,
row credits and sealed map, without retry or partial native close. Successful
map insertion transfers ownership; incoming batch prefixes clear per known row.
Private stream constructor signatures change at all actual native callers.
Unknown context/provider interiors before return, cancellation/actual cleanup,
last-holder/row retirement and whole resource bounds remain separate open gates.
Public API, dependency/wire version and sing-quic consumer are unchanged.
Source/manual review and touched-only gofmt only; no tests/build/native proof.

Revision56 makes native stream shutdown return an explicit local result at all
private stream/map callers. Per-half original receipts keep started/returned/
failure state under existing stream mutexes. Refused native admission, uncertain
stream state or receive buffer retirement failures cannot become successful map
close. Bidirectional shutdown composes both ordinary results; native map errors
retain the exact original retiring stream, rows and credits without cleanup retry.
Child maps return and retain ordinary close failures; the parent composes all
four results. Failed 0-RTT cleanup cannot replace maps/framer/flow state.
Terminal Conn joins the failure into its private close result, denies a new
terminal write and keeps the original loop receipt negative. Existing actual
panics/Goexit still unwind as unknown; ordinary errors do not add a new panic.
Send shutdown explicitly cancels its original context with the shutdown cause,
including rejected 0-RTT. It signals writers first and invokes the exact cancel
function outside the send mutex; positive result follows its actual return and
state recheck. Unknown cancel/notification retains the original negative scope.
No rows or active Write views are cleared here: public/map/ACK/borrower/data/
flow/global and actual last-holder retirement remain separate open gates.
Public API, dependency/wire version and sing-quic consumer are unchanged.
Source/manual review and touched-only gofmt only; no tests/build/native proof.

Revision57 retains original and candidate stream map sets in one original 0-RTT
reset record. Parent admission is fenced before native close, then the original
four children and connection flow counters are reset outside parent metadata
locks. A flow refusal precedes candidate construction. The shared native builder
populates a retained candidate shell without overwriting current parent maps.
Framer reset validates exact original/distinct candidate bindings and swaps its
token under existing locks; sealed/retired/mismatched state returns an error.
Parent publication follows known flow/build/framer success and a close/identity
recheck. Pending/failed UseResetMaps returns an error through NextConnection;
the reset attempt is not retried. Normal refusal, completed transition and
unknown unwind remain separate. Terminal close retains unknown reset failure.
No rollback of already-reset counters is invented. Old and candidate map sets
remain strong references for subsequent actual last-holder/row retirement,
bounded to this one attempt; no immediate GC/RSS/native reclamation claim.
Public API, dependency/wire version and sing-quic consumer are unchanged.
Source/manual review and touched-only gofmt only; no tests/build/native proof.

Revision58 adds external Conn.WaitForStreamMapRetirement. One original parent
finalizer joins packet input, map calls, receive data/calls and send data (which
joins all six stream-call classes/framer), then retains fixed current/original/
candidate sets before preflight/metadata clear. Concurrent callers join its same
result; failed/unknown attempts keep original references without cleanup retry.
Only a proved empty, unexposed candidate may obtain its original empty map-close
receipt after loop end; current/old sets require prior positive child close.
All distinct sets are preflighted before row references/logical metadata credits,
factory/queue/prefix aliases and parent constructor inputs are cleared. Existing
stream objects, external contexts and immutable generation bindings stay intact.
This is metadata reference retirement, not raw/ACK/crypto/OS/GC/RSS reclamation.
The actual native retireTransportConn and paired HY2 consumer now use this
aggregate instead of duplicating its component waits. Existing receipt APIs
retain their scopes; dependency/wire versions are unchanged. Source/manual
review and touched-only gofmt only; no tests/build/native proof.

Revision59 adds immutable observed/outbound peer-parameter scalar heads. Public
ConnectionState, DATAGRAM send and AddPath no longer read the native mutable
wire pointer. Restored/received/applied/rejected0RTT phases preserve handshake
timing; rejection fences new queue admissions before dropping history/reset.
Actual queue is bound before exposure and shares sendMx with outbound publication:
reserve/enqueue/Peek/history transfer revalidate the current whole-frame limit.
Head-change broadcast wakes all pending producers; untransferred queued owners
wait through rejection until fresh apply, then are revalidated or dropped.
Exact type/LEN/payload sizing includes empty data and all varint widths. Local
receive advertisement is independent of peer receive support (RFC 9221 section3).
No public API/dependency/wire version change; paired sing-quic consumer unchanged.
Only source/manual review and touched-only gofmt; no tests/build/native proof.
Receipts: `.artifacts/memory/bootstrap-v457-peer-parameters-`.

Revision60 retains original initial Conn construction through partial child
initialization. Conn precedes server oneRTT allocation; client oneRTT lives on
Conn. A fixed original prefix holds context/TLS/address/parameter/token inputs
and partial crypto setup until known transfer. ACK/Cubic/TLS retained constructors
bind caller-owned typed slots before qlog callbacks; old wrappers share the same
initializers. Conn/map/child/sorter shells are held before native initialization,
including the initial fixed four-map set. Complete marker follows known graph
transfer and clears transient aliases; actual client/server publication requires
the original complete candidate. Unknown original opening remains retained with
no partial native Close/retry or extra worker. Internal additive constructor APIs
only; dependency/wire/public Conn APIs unchanged. Opaque helper internals before
return, fatal allocation failure and ongoing raw/ACK/crypto retirement remain
separate. Source/manual review and touched-only gofmt, no tests/build/native proof.
Receipts: `.artifacts/memory/bootstrap-v458-connection-construction-`.

Revision61 separates original Retry packet-space reset from path migration.
Fixed native prefix retains old/candidate spaces/current packet across loss,
RTT and qlog callbacks; negative/unknown keeps originals with sticky history
failure and no retry. Migration drains the current first probe row, avoiding
skips caused by compaction of a ranged slice. Initial/candidate packet-space
shells precede native init. A bounded nonwaiting congestion-change cell retains
previous/source/candidate; migration uses retained Cubic construction and the
same publication mutex as the public setter. SetRTTStatsProvider/qlog execute
outside metadata locks; nil/typed-nil/overlap/failed/closing changes are refused.
Actual initial handler binds Closing before callbacks. Public void setter keeps
its signature and requests original Conn close on refusal/abnormal setup; native
Retry/migration errors propagate through actual callers. Terminal loop receipt
is negative for active/failed packet/congestion transitions, not a join of every
custom controller call. Internal interface contracts changed; compile/private
mock/fixture compatibility and full raw/ACK/crypto/global retirement remain OPEN.
Paired consumer unchanged; public API/dependency/wire versions retained. Only
source/manual review and touched-only gofmt; no tests/build/native proof.
Receipts: `.artifacts/memory/bootstrap-v459-packet-transitions-`.

Revision62 registers the complete outer public congestion setter in one original
Conn cell before handler/controller setup. First-close atomic/Closing admission
and original seal fence stop new entries; ordinary return clears temporary call
refs, abnormal return retains them with a negative receipt and no retry. The new
external WaitForCongestionControlShutdown joins original packet input/native-loop
and public-call receipts, then inspects retained ACK congestion state. Native PacketStateError now
covers its own history/reset producer state; external public-call completion is
not confused with native-loop completion. Existing component receipt scopes are
unchanged. Known Closing refusal after returned setup drops only an unpublished
candidate's temporary aliases; unknown/other failed change retains its prefix.
Native retireTransportConn and actual HY2 close task join congestion
before stream-map/path metadata retirement. No new task/worker/timer, forced
provider cancellation or all-controller-method/RSS/OS retirement proof. External
wait must not run from a callback/call it joins; opaque callback self-close/join
and hidden provider tasks remain separate. Public additive receipt/internal
state API changes require the compatible pair; dependencies/wire unchanged.
Receipts: `.artifacts/memory/bootstrap-v460-congestion-calls-`.
Source/manual review and touched-only gofmt only; no tests/build/native proof.

Revision63 adds an original DATAGRAM send-entry ledger within the existing4096
owners/8MiB budget. Reserve precedes clone; a separate producer reference lasts
through copy/enqueue/context/scheduling/timer return. Queue/history and Peek
borrowers retain their own references, so Close cannot release an active copy.
One original producer channel settles after queue close; abnormal producer state
remains retained/negative. The additive external WaitForDatagramSendDataShutdown
joins original packet-input/native output and admitted owned-data producers.
One finalizer first preflights the complete bounded live history-only set and
exact count/byte sums, then releases owned payloads without forged ACK/loss,
retransmission callbacks, pooled STREAM access or pool operations. Negative
results retain original metadata/snapshot/current and are never retried; all
observers share one result. Actual Transport/HY2 join it after congestion and
before map/path metadata retirement. No new worker/task/dependency/wire version.
Pre-admission caller input, complete scope/public calls, receive/ACK metadata,
hidden tasks/native/OS/GC/RSS and speed/resource proof remain separate. External
wait must not run from a producer callback it joins. Source/manual/gofmt only,
no tests/build/native proof. Receipts:
`.artifacts/memory/bootstrap-v461-datagram-retirement-`.

Revision64 adds one original sent-handler metadata finalizer, constructed before
callbacks and shared by concurrent observers. External Conn.WaitForPacketHistoryRetirement
joins original congestion/native-loop, DATAGRAM data and stream-map aggregate
(STREAM data/all-call/framer) receipts before history retirement. Its fixed three
current spaces are captured before preflight: original Closing, idle reset/intake,
distinct valid spaces, bounded physical history/probe caps, logical counters and
empty ACK scratch are required before any reference-cell clear. It never reads
packet/STREAM aliases or returns pooled packets. Only owned history/probe cells
are zeroed; ACK/lost/info heads are dropped without mutating provider-held scalar
aliases. Statistics/controller/RTT/ECN and separate crypto/control/retransmission
holders remain. Negative/abnormal results retain the original prefix and do not
retry; no ACK/loss/delivery/controller callback or extra worker is invoked. Actual
Transport/HY2 use this aggregate before path metadata retirement. Source/manual/
gofmt only, no tests/build/native/performance proof. Internal additive interface
and public aggregate require a compatible pair. Receipts:
`.artifacts/memory/bootstrap-v462-packet-history-`.

Revision65 retains the original native TLS Close call/result separately from its
shutdown receipt. The installed SDK source joins the handshake before returning
handshakeErr: an ordinary API error is not unknown call lifetime. One cryptoSetup
cell/done exists before constructor callbacks; concurrent Close/WaitForShutdown
observers join it outside metadata locks. Abnormal/nonreturning work is retained
without retry/forced cancellation. Actual internal crypto interfaces change.
External WaitForCryptoQueueRetirement first joins revision64's native/public-call/
data/map/sent-history aggregate, then the native TLS completion receipt. Fixed
manager/three-stream/retransmission/framer/packer identities and all callback-free
crypto sorters are preflighted before local heads change. Existing sealed framer
retirement is required. It drops local sorter/tree/write-buffer/retransmission
heads and seals legacy retransmission admission, without bytes/key overwrites,
generic sorter callbacks, frame/pool/ACK/loss operations or fabricated delivery.
Actual Transport/HY2 consume this aggregate before path retirement. The TLS object
remains for ConnectionState; TLS events/keys/provider aliases, received ACK
history, outgoing byte budgets, OS/GC/RSS and global/native/performance acceptance
remain separate. No extra worker/dependency/wire version. Source/manual/gofmt
only; no build/tests/native proof. Inspected SDK identity is evidence for that
source, not proof of the compiled toolchain. Receipts:
`.artifacts/memory/bootstrap-v463-crypto-queues-`.

Revision66 extends the existing WaitForPacketHistoryRetirement aggregate with
received ACK-history retirement after native/public-call/data/map and sent-history
receipts. Conn retains the original ReceivedPacketHandler pointer; client/server
packers and sent ACK callbacks use it directly, and construction completion now
requires it. Its constructor binds original Closing and creates one owner-bound
result/done/fixed3-tracker finalizer. Copies cannot retire the original. All
tracker identities and bounded sorted nonoverlapping ranges/floors are preflighted
before any head changes. LargestAcked+1 is accepted as an ignore-all floor even
past the maximum valid packet. Only internal range/cached-ACK heads and scheduling
state are retired; ACK objects/backings are never inspected/reset/pooled, and
ECN/observed/connection counters remain. Serial legacy refill/timers are sealed
only after actual producer joins. Negative/abnormal state retains original
prefix/result with no retry, callbacks, fake delivery or additional worker.
The unchanged Transport/HY2 consumer reaches this stronger receipt through the
existing crypto aggregate. No public Conn API/dependency/wire version change;
optional internal constructor binding is additive. TLS internals/byte budgets/
arbitrary concurrent providers/native/OS/GC/RSS/performance remain separate.
Source/manual/gofmt only, no tests/build/native proof. Receipts:
`.artifacts/memory/bootstrap-v464-received-history-`.

Revision67 fixes queued TLS write/session-ticket aliases retained across the
SDK's next NextEvent call. QUICEvent.Data is borrowed under that contract; owned
copies now precede advancement. One serialized queue reserves count/bytes before
clone, caps128 pending events/8MiB copied payload and clamps metadata growth.
Metadata events share admission; dequeued slots are zeroed before reslicing,
empty backing heads dropped and queued credit transferred to the caller. A local
ticket copy fits remaining allowance while in-call; caller-retained copies and
TLS/STREAM/retransmission/global memory remain outside this queue budget.
Additive EventError/Err preserves NextEvent's signature; the actual Conn consumer
propagates pressure instead of fake readiness. Go1.26 QUICErrorEvent/Err is handled
explicitly, verified against installed SDK/API source. The fork Go directive is
now1.26, matching the existing Windows parent; dependency versions are unchanged,
no resolver/tidy/download/build ran. External RetireEventQueue uses the original
Close cell for one bounded metadata result after actual native/data/history/TLS
and local sorter preflight. Ordinary capacity refusal can retire once quiescent;
inconsistent credit/unknown original stays retained without retry. Only event
reference cells are cleared, never bytes/keys/provider objects. Existing Transport/
HY2 uses the stronger crypto aggregate unchanged. TLS keys/internal objects and
outgoing STREAM-crypto/retransmission lifetime budgets remain open. No extra task/
worker, public Conn API or wire change. Source/manual/gofmt only; source ownership
evidence is not runtime reproduction, compilation or performance proof. Receipts:
`.artifacts/memory/bootstrap-v465-crypto-events-`.

All source is unverified: API/legacy behavior, frame/drop/peek/ACK/loss/unsent
interleavings, scope seal/deadline/reset/identity/retiring quota, raw-view tracing,
shutdown, allocation pressure, wire compatibility,
performance and exact Windows artifact acceptance remain OPEN. No tests/fixtures
were added or run. Native TUN remains unaccepted; schema2 install unsupported.
Address snapshots preserve observed local metadata after close; they are not
route/egress/protection/trust or current-path proof. Unknown/oversized/typed-nil
standard addresses are refused; opaque addresses remain opaque, with no synthetic
UDP capability. Custom interface typed-nil/provider contracts, rawConn bypass
initializers, remote aliases and path/global/native lifetime need their own review.
HTTP/3 task reservations are not a byte/RSS/CPU or process-wide limit. Body readers,
output streams, qlog sink and custom providers can block indefinitely; no Stop SLO
or forced cancellation is asserted. WaitForShutdown must be called by an external
owner after requesting QUIC/raw abort, never from its own registered worker.
The default HTTP Transport's separate dial/retry/pool tasks, server lifecycle and
all external stream/output/trace borrowers retain their own acceptance scope.

Rollback must restore a compatible pair of source/manifest snapshots. Removing
only this replace while the consumer uses packetaddr or HY2 uses
NewDatagramSendScope/scoped Send/ClientConn.WaitForShutdown or
Conn.WaitForStreamReceiveShutdown/WaitForStreamCallsShutdown/WaitForStreamMapShutdown/WaitForStreamSendDataShutdown/WaitForPacketInputShutdown/WaitForStreamMapRetirement/WaitForCongestionControlShutdown/WaitForDatagramSendDataShutdown/WaitForPacketHistoryRetirement/WaitForCryptoQueueRetirement
is not source-compatible.
Do not edit module-cache sources. Builds/checks need separate authorization.
