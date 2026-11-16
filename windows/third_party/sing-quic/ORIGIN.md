# Windows sing-quic source bundle — revision 75

Upstream module: `github.com/sagernet/sing-quic`.
Frozen declared version: `v0.6.4-0.20260803041914-d83826c306d7`.
Source: the existing local module cache; no network fetch or cache edits.
The Windows parent module alone selects this directory through `replace`.
All dependency requirements retain their original versions. Windows still
selects quic-go `v0.59.0-sing-box-mod.4` and sing `v0.8.13` through its existing graph.

The 78 mirrored production/module/license/README files were copied byte-for-byte.
Tests, CI/workflows, dotfiles and Makefile were omitted. No embedded source assets
were declared in the inspected source. `LICENSE` and upstream copyright notices
are retained, including notices in congestion code. The source baseline hashes
and local changes are recorded under `.artifacts/memory/bootstrap-v391-hy2-receive-`.
This receipt is not upstream authentication or a compiled artifact attestation.
Revision2 source changes and receipts are under
`.artifacts/memory/bootstrap-v392-hy2-transmit-`.
Revision3 pairs this consumer with local quic-go mod4/revision1; HY2 uses its
SendDatagramContext API to cancel a pending queue wait with the original session
context. The legacy SendDatagram API remains on the QUIC side. Pair source and
receipts: `.artifacts/memory/bootstrap-v393-quic-datagrams-`.
Revision4 pairs with quic-go revision2 and uses an original DatagramSendScope
per UDP session. Scope creation can fail before publication; actual client/server
constructors propagate/refuse that failure and retain their existing ownership.
Receipts: `.artifacts/memory/bootstrap-v394-session-deadline-`.
Revision5 retains that pair and changes only the Windows Hysteria hopping
wrapper, used by both Hysteria clients. Original socket close tasks and wrapper
operations are retained through actual return; no module cache or version change.
Receipts: `.artifacts/memory/bootstrap-v395-hop-retirement-`.
Revision6 changes the Windows HY2 client offer/carrier lifetime, preserving the
same frozen quic-go version with local revision3. The owner precedes initialization, explicit
QUIC dial replaces HTTP/3's hidden asynchronous dial/retry pointer, and raw/QUIC/
standard Transport close calls start independently. Receipts:
`.artifacts/memory/bootstrap-v396-carrier-retirement-`.
Revision7 retains quic-go revision3 and adds early original UDP socket custody
plus bounded Salamander output ownership. Receipts:
`.artifacts/memory/bootstrap-v397-socket-salamander-`.
Revision8 retains quic-go revision3 and gives the Hysteria hopping wrapper its
own resettable read deadline/queue admission. Receipts:
`.artifacts/memory/bootstrap-v398-hop-read-deadline-`.
Revision9 pairs with quic-go revision4 and reuses its public packetaddr snapshot
for original-socket metadata. Hop native callback operations retain the original
RawConn object and join the existing close owner. Receipts:
`.artifacts/memory/bootstrap-v399-packet-metadata-`.
Revision10 pairs with quic-go revision5 and joins the original HTTP/3 client
task/qlogger completion through WaitForShutdown. QUIC Closing is observed before
HTTP/3 authentication begins. Receipts:
`.artifacts/memory/bootstrap-v400-http3-ownership-`.
Revision11 retains quic-go revision5 and adds bounded HY2 TCP core-call owners,
serialized/resumable request and response framing, and carrier cleanup joins.
Receipts: `.artifacts/memory/bootstrap-v401-tcp-ownership-`.
Revision12 retains quic-go revision5 and supplies tracked HY2 TCP half-close and
modern early-handshake capabilities for the pinned sing-box ConnectionManager
and sing CopyConn consumers.
TLS capability preparation precedes native dial, with the original owner still
attached before actual QUIC dial. Receipts:
`.artifacts/memory/bootstrap-v402-tcp-halfclose-`.
Revision13 retains quic-go revision5 and implements the pinned consuming
N.ExtendedWriter/N.VectorisedWriter contracts directly on HY2 TCP. It avoids
generic whole-vector payload merging, tracks bounded original input custody and preserves scalar
framing under one writer lock. Receipts:
`.artifacts/memory/bootstrap-v403-tcp-vectorised-`.
Revision14 pairs this same consuming HY2 implementation with quic-go revision6.
The lower writer removes ordinary input views before returning and retains
sticky abnormal originals instead of reusing the writer/packetizer. No consumer
API, dependency version or licence change. Receipts:
`.artifacts/memory/bootstrap-v404-stream-borrow-`.
Revision15 pairs the same HY2 consumer with quic-go revision7's receive terminal
callback custody and reliable-prefix read/peek handling. Versions/licences and
consumer admission quotas remain unchanged. Receipts:
`.artifacts/memory/bootstrap-v405-receive-retirement-`.
Revision16 pairs the same HY2 consumer with quic-go revision8's original STREAM
intake and bounded per-stream receive-sorter custody. Versions/licences and HY2
quotas remain unchanged. Receipts:
`.artifacts/memory/bootstrap-v406-stream-intake-`.
Revision17 pairs with quic-go revision9. The original HY2 QUIC close call joins
Conn.WaitForStreamReceiveDataShutdown after CloseWithError, preserving either
error with errors.Join. The result covers sealed STREAM receive data and its
original parser/dispatch producer, alongside existing independent TCP/HTTP3 joins.
It does not prove arbitrary control callers or whole native/OS retirement.
Per-connection receive pressure ceilings are32768 owners/32MiB exposed backing;
HY2 quotas and frozen versions/licences are unchanged. Receipts:
`.artifacts/memory/bootstrap-v407-connection-receive-`.
Revision18 pairs with quic-go revision10 and switches the original HY2 QUIC close
call to Conn.WaitForStreamReceiveShutdown. That result composes original receive
data/parser completion with bounded Read/Peek/CancelRead/deadline and native
receive/control/shutdown calls. Existing independent raw abort and TCP/HTTP3 joins
remain. It does not prove send-side/promoted controls, raw/ACK/crypto/history,
whole QUIC/native/OS or process-wide retirement. Versions/licences are unchanged.
Receipts: `.artifacts/memory/bootstrap-v408-receive-calls-`.
Revision19 pairs with quic-go revision11. Original HY2 QUIC close also joins
WaitForStreamCallsShutdown, covering registered send calls alongside the separate
receive-call/data receipt. Existing TCP half-close/framing, raw abort and HTTP3
joins remain. Transmitted STREAM/ACK/loss/history storage and callback last holders
are not completed by this call registry. Frozen versions/licences are unchanged.
Receipts: `.artifacts/memory/bootstrap-v409-send-calls-`.
Revision20 pairs with quic-go revision12 and also joins original transmitted
STREAM data through WaitForStreamSendDataShutdown. Normal ACK/loss/discard use
immutable attempts and explicit history-cell handoff; residual data retirement
requires positive original loop/all-call quiescence. Unknown originals stay
retained. Raw/crypto/ACK-control metadata/global/native/OS retirement remains
separate; frozen versions/licences and previous HY2 quotas are unchanged.
Receipts: `.artifacts/memory/bootstrap-v410-stream-send-data-`.

## Local changes

- HY2 client processes received datagrams in its existing carrier loop, without
  a goroutine per packet. The server's existing serial loop is preserved.
- Wire sizes and fragment indices are checked before destination/message/buffer
  allocation. HY2-owned storage has nonblocking per-carrier count/byte admission.
- A session-owned fragment map replaces callback-based LRU eviction. Feed,
  expiration, group replacement and close share one session mutex. No group
  borrower escapes the owner; user reads never block feed or eviction.
- Session creation is bounded before allocation, IDs cannot overwrite a live
  client session, and destruction removes only the original map entry.
- Session close seals input, frees owned partial groups and drains ready messages.
  A reader that already dequeued a message retains its credit until actual release
  or explicit buffer handoff. Assembly reserves output while parts remain held.
- Capacity rejection drops the affected UDP packet or rejects a new session;
  malformed framing retains a separate protocol error. Overload does not close
  the entire carrier.
- HY2 TX lives in packet_write.go. Before frame/view creation, one carrier-wide
  staging permit covers bounded address materialization, packed backing and a
  fragment pointer table. There is no semaphore wait-list or timeout goroutine.
- Both write APIs check header/budget arithmetic and fragment count before
  allocation. The retry uses a checked, strictly smaller size and a fresh packet
  ID, so fragments admitted by the first attempt cannot join a new layout.
  Initial1197, conservative retry-3 and at most one retry remain.
- TX messages/views retain independent pool ownership and no RX accounting
  pointers. WritePacket releases its original input buffer once, before returning
  the staging credit; WriteTo borrows its caller's slice until the actual call ends.
- Destinations use bounded concrete Socksaddr (value/pointer), UDPAddr or TCPAddr
  representations. Unknown/custom net.Addr implementations are explicitly
  rejected without calling their String method. This is a compatibility boundary
  requiring future validation, not a claim that every historical adapter works.
- Close seals the original send scope before session cancellation and RX cleanup.
  SetWriteDeadline updates that scope, and SetDeadline combines it with the
  existing read deadline. Neither changes the shared QUIC/raw socket deadline.
  Send uses the same scope at queue transfer; an early Err observation is not a
  forwarding permit. The outer TX producer/buffer still lives through actual return.

- HopPacketConn no longer holds its lifetime mutex over WriteTo, LocalAddr,
  deadline/buffer setters or SyscallConn callbacks. It borrows the original Go
  socket object through a bounded public call, without caching a numeric FD.
- Each original hopping socket has one independent Close task and one result.
  All currently retained sockets start closing before any is awaited. Repeated
  wrapper Close waits for the same retirement and returns its result. An abnormal
  callback return or close error is negative, never a nil receipt.
- Close joins the original hop task (including late/partial dial cleanup), all
  receive tasks and admitted public calls before draining the bounded packet
  queue. A separate control mutex serializes setters and candidate publication;
  raw abort does not wait for it. Candidate deadlines/buffers are applied before
  publication. Existing current-socket control result semantics are retained.
- I/O call admission is nonblocking at16, with separate control2 and metadata2
  lanes; receive generations are capped at3 before
  dial/spawn. Invalid ports/interval are rejected; the bounded port list is copied.

## Versioned engineering ceilings

Revision16 adds no HY2 worker/queue or provider change. Quic-go revision8 accounts
each receive sorter's8192 original holders/8MiB exposed backing through actual
last-holder return, including current reads and trim-copy overlap. First parser
copy/connection/global bounds and aggregate native uncertainty remain open;
this is not a complete process RSS/Stop/performance or whole-PC acceptance.

Revision15 preserves all prior bounds and the send/half-close/consuming fences.
Known native receive terminal cleanup now joins current/queued callback return;
uncertain original frames are not retried or replaced. It does not establish
complete parser/map/sorter-error intake ownership, receive admission ceilings,
ACK/history/global/native retirement or measured runtime/performance acceptance.

Revision14 retains all revision13 quotas and the io.Writer contract boundary.
Quic-go revision6 adds terminal/deadline-before-tail checks, ordinary input-view
retirement under the packetizer mutex and a sticky abnormal write fence.
This does not close the lower receive/sorter/history/reliable-reset/global,
advanced caller/provider, native/runtime/performance or whole-PC acceptance.

Revision13 adds256 fixed consuming-write owners per carrier, at most256 original
input pointers per owner,4096 aggregate pointers and16MiB aggregate exposed
backing. Count and checked backing arithmetic precede allocating/copying the
input pointer table, ref acquisition and native writes. Pointer-table payload is
at most4096 pointers, separate from fixed slot metadata/inline single pointers,
payload charges and the existing8*64KiB prefix staging. Queued consuming writers
reserve/copy/ref-pin before waiting for the writer mutex, so their original
inputs remain charged. One core-call registration and writer mutex span
the entire batch; existing scalar framing/partial-header behavior is reused.
An empty batch retains Write(nil)'s bounded-header flush. Ordinary refusal/error
consumes the inputs, while abnormal native return retains original ref-pinned
buffers/credits and poisons before writer unlock. Cleanup must return before
slot reuse. WriteBuffer uses this same owner with one input, replacing the
generic scalar adapter's unconditional deferred input release; the one-input
pointer table uses existing inline slot storage. No worker/timer,
full payload merge or new raw Upstream is added. Default pinned copy batch is8;
actual Windows wrapper/copy/tag reachability, packetization, throughput,
interleavings and ref/pool compatibility remain
unverified. Inputs created before admission, invisible backing prefixes/aliases,
custom allocator/global pool/RSS and lower QUIC borrows remain outside this bound.

Revision12 adds CloseWrite/CloseRead on the existing original core/control lanes,
with no new worker/timer/quota. Without CloseWrite both inspected consumers chose
full Close at upload EOF and could cancel a pending response. CloseWrite now
serializes with core writers, flushes a bounded header for empty upload, issues
the original FIN once and retains its outcome. Read remains open. CloseRead
cancels read through the control owner without taking the blocked reader mutex;
write remains open. Both local directions closed request the existing full owner
after a recorded write-half outcome; close intent alone is no success receipt.
Header-flush failure requests logical full close to wake a waiting copy reader;
ordinary write reset/close error does not itself become native uncertainty.
Empty request flushes borrow their original bounded header through actual Write
return, without allocating a payload coalescing buffer or consuming its8 permits.
Partial offsets survive retry; abnormal return retains that original header.
When payload coalescing is full, a normal writer uses that same bounded header
and borrowed payload on the same QUIC stream. Only the known staging-capacity
error permits this encoding choice; closed/uncertain admission still refuses.
The shared coalescing memory ceiling is unchanged; no direct network route or
automatic channel/authority change is introduced.

Atomic read-framing state supports modern EarlyReader/EarlyWriter while retaining
the legacy NeedHandshake/Write(nil) contract. No Upstream/raw writer escape or
implicit read-triggered request was added. These predicates attest framing facts,
not peer acceptance or protected egress. Raw promoted QUIC controls remain open.

TLS clone/ALPN/STD capability errors are refused before creating the native socket.
Custom qtls.Config retains its own path; no silent STD fallback or fingerprint
downgrade. Transport/raw ownership still precedes actual DialEarly. The current
Windows HY2 renderer uses STD TLS; tagged uTLS support alone is not QUIC support.
Provider hidden transports/native joins, wire/caller ABI and native gates remain
unverified; this change does not activate unsupported uTLS/Reality QUIC providers.

Revision11 reserves at most256 opening/visible/retiring TCP stream owners before
destination clone or OpenStream. Core I/O admission512 permits one Read+Write
pair on every visible stream; each stream has at most8 admitted I/O calls.
Deadline controls use a separate carrier lane8/per-stream2. First-write staging
has8 permits charged64KiB each (512KiB); its copied payload prefix is at most16KiB.
The checked header writer bounds address/padding before allocation. Original
header offsets and the payload-only partial byte count survive write deadlines.
Large caller payload tails are borrowed through the actual QUIC Write return.

Read/Peek share one persistent bounded TCPResponseReader. Status, partial varints,
message and skipped padding survive timeout without treating later bytes as a new
response. Message<=2048/padding<=4096, no padding-length allocation. Per-stream
Close seals core calls, cancels read and wakes writes with a past deadline, joins
admitted operations, then calls Stream.Close without concurrent core Write.
Deadline setters cannot overwrite that wake after seal. Repeated Close waits for
the same task/result; uncertain buffers/owners retain their original capacity.
Carrier raw/QUIC/Transport aborts start before any TCP join. Known stream reset/
protocol Close errors are distinct from a native cleanup failure; the original
stream Close result is still returned to its caller. LocalAddr is an observed
owned QUIC socket fact; RemoteAddr is the owned logical destination, neither an
egress/protection proof. Promoted advanced QUIC controls remain a separate gate.

Fixed per-stream framing/header storage and borrowed caller input are separate
from staging charges. The charge assumes the inspected default sing allocator;
allocator/pool high-water, lower QUIC frames/ACK/history, arbitrary caller aliases,
closed wrappers retained by callers, advanced controls and global/native retirement
are not covered. No runtime/compatibility/latency/Stop/RSS or speedup is claimed.

Revision10 retains one original HTTP/3 completion task alongside the independent
raw/QUIC/standard Transport close tasks. Late HTTP/3 attachment starts its wait
after seal; initializer completion fences the final inventory. The paired client
task owner bounds64 call/worker/callback reservations before spawn, joins actual
returns and distinguishes known overload from uncertain cleanup. Original qlogger
callback/Close return is included when present. There is no timer-based refund.
HY2 authentication caps response header bytes at64KiB and disables automatic
compression. Closing watch starts immediately on QUIC attachment, including
authentication failure/SETTINGS paths. HTTP/3 DATAGRAM receive requires both
local opt-in and peer SETTINGS; HY2's raw QUIC DATAGRAM reader is not an HTTP/3
datagram consumer. Wire/provider compatibility and payload/allocator/global bounds
still require acceptance; no measured speed/RSS or complete native join is claimed.

Revision9 caches local address facts on the exact hopping socket. Known standard
address fields are copied; bounded opaque network/text is captured under the
original metadata call. Reads of an observed cache remain provider-free after
close and return fresh standard address objects, without mutable IP aliases.
An unobserved/invalid closed address stays unknown, not a placeholder. A cache is
an observed socket fact, not continuous path/route/protection evidence.

Metadata admission2 is independent of I/O16/control2 and joins the original call
WaitGroup. SyscallConn returns an original-generation wrapper; Control/Read/Write
register before invocation, check current generation before callback delivery,
and reject stale/closed operations. No numeric FD is retained by this wrapper.
An already admitted callback may finish after sealing; standard synchronous
RawConn provider/caller lifetime rules still apply. This does not constrain a
hostile asynchronous provider, arbitrary allocations inside Network/String,
foreign retained descriptors or caller-created goroutines. No hard Stop SLO.

Revision8 moves the wrapper read deadline to hop_read.go. Native receiver loops
are not given that deadline and remain available after extend/clear. Deadline
check and queue dequeue share access; change notifications wake all readers, and
one reusable timer per admitted reader only wakes a current-state recheck.
No AfterFunc, timeout goroutine or receiver replacement is created on expiry.
SetDeadline applies this logical read gate plus the existing native write setter;
a write-control error can leave the read update applied and is returned honestly.

Read readiness has one buffered signal and no error mailbox. Current native I/O
failure is retained with its original socket; faults from previous generations
cannot retire the current one. Fault choice and close seal share the hop publish
mutex. Known I/O failure remains separate from actual negative Close uncertainty.
Existing I/O calls16 now have a separate control lane2; inline read-deadline
metadata cannot be exhausted by waiting readers. The queue/buffer/receiver bounds
remain unchanged, with at most16 pending read timers and no timer per deadline set.

Revision7 reserves one opening slot before each actual HY2 dial and publishes
its once-only original net.Conn close owner before RemoteAddr/Unbind/Salamander
construction. Close aborts a late socket immediately; partial-error/abnormal
cleanup retains the original slot/result. A successful handoff remains joined
through the original initializer or Hop task and its existing raw owner.

Salamander WriteTo borrows input unchanged; the consuming vector API owns its
input release and sends only an unmanaged view of owned ciphertext. Continuous
XOR offset replaces incorrect per-buffer indexing. Both variants use the same
bounded output ledger:8 staging slots, each at most65535 logical bytes (64KiB
default allocator class), vectors at most256 before staging. Original input
release and actual writer return precede successful slot/pool reuse. Abnormal
return retains output, poisons future writes and starts the original socket abort;
Close joins that original call and reports negative uncertainty rather than
inventing retirement. Salt-short frames are skipped, salt-only frames decode to
an empty datagram; invalid read sizes/short output writes are refused.

These limits exclude caller/input backing, password/hash scratch, allocator/pool
high-water, external RawConn borrowers and kernel storage. The vector fast path
now coalesces encrypted output, and WriteTo uses original PacketConn.WriteTo;
neither a speed gain nor universal adapter/wire compatibility is proven.

HY2 revision6 adds client_lifetime.go for original close-call results, late resource
attachment, initializer/RX/watch task joins and retained failed/discarded offers.
Close/reset seals client admission before external callbacks. A successor requires
the original recorded cleanup result; a negative result retains the old slot.
Carrier cancellation owns later hops and UDP sessions. One15s shared initializer
timeout/watch task may request abort; its decision and initializer completion
share one mutex so canceling a finished offer cannot close the accepted carrier.

The standard TLS branch retains an explicit quic.Transport before DialEarly;
the existing optional qtls.Config provider uses its DialEarly method. HTTP/3's
existing NewClientConn is built over the actual returned connection, without
an implicit retry or asynchronously written **quic.Conn result pointer.
TLS is cloned and ALPN forced to h3. Wire/provider behavior needs acceptance.

RX/TX budget seal and reserve share their existing mutex. Cleanup waits for
original HY2 credits, including dequeued readers and admitted writers. A poisoned
ledger settles negatively and never authorizes replacement or refunds live data.
The paired QUIC Closing signal precedes send-queue drain, so internal QUIC close
can request raw abort before Context cancellation. It is observation only, not
an original close-result or native completion receipt.

| Scope | Ceiling / unit |
|---|---|
| Each carrier | 256 visible UDP sessions; 4096 HY2 RX storage owners; 8 MiB charged storage |
| Each UDP session | 64 partial fragment groups; 256 KiB charged partial storage; existing ready queue 64 |
| One UDP payload | Existing protocol maximum 4096 bytes; fragment count at most wire uint8 = 255 |
| Destination | Existing protocol maximum 2048 bytes |
| Fragment expiry | Existing 10-second age, reclaimed lazily on feed or explicitly on close |
| Assembly | Reserve a 4096-byte output backing plus destination while all input parts remain charged |
| Each carrier TX | At most8 active producers and128KiB of charged HY2 staging |
| Each TX permit | 14456 charged bytes: default packed class8192, bounded address materialization4224, fragment pointer table2040 |

Hopping wrapper ceilings:16 active I/O calls plus2 control and2 metadata calls,
3 receive generations,1 hop task;
existing1024 queue buffers of2048 bytes, at most3 active receive buffers and at
most16 dequeued public-reader buffers under the shared public-call ceiling.
Each original socket has one Close task, with at most3 simultaneously retained
live/pending/retiring sockets. The copied port list is limited to65536 uint16 values;
empty lists, zero ports and negative intervals are rejected, not silently clamped.

Charged storage counts original input `cap`, retained destination string lengths,
fragment pointer slots (8 bytes each for Windows/amd64), and assembly backing.
Count bounds separately cover message/group metadata; these are not Go heap/RSS,
pool high-water, allocator, QUIC queue, kernel socket or whole-process limits.
The current serial raw input and caller-owned buffers after an explicit reader
handoff are outside this ledger. No retained HY2 queue/group loses credit merely
because it is dequeued or hidden from a session map.

TX charges assume the inspected default sing buffer allocator. Caller input
backing/aliases, caller goroutines, QUIC's pre-Add copies, queued/history data,
custom allocators and allocator/pool high-water are outside the staging ledger.
Fragment metadata count is bounded by8 producers×255 views. A local write return
does not retire copies already transferred to QUIC or prove peer delivery. Session
cancel is checked before further allocation/send. Revision3 passes the original
session context into QUIC to wake a pending Add. Revision4 adds a scope-local
seal/enqueue fence and deadline. Already admitted frames remain QUIC-owned;
this is not whole-carrier/raw-socket retirement or a hard wall-time expiry barrier.
Sealed-scope quota remains occupied through active scope Send calls; HY2 outer
writer/packed-buffer and transferred QUIC frame owners have independent returns.

## Remaining scope and rollback

This is source-only PC-06 partial D0/D3/D4 work. No tests, fixtures, build, race, benchmark,
SDK, native or network execution was performed. API/tags, wire interoperability,
expiry/close interleavings, overhead, throughput, RSS/CPU and resource fairness
remain unverified. There is no measured speedup, total memory bound or Stop SLO.
Lower QUIC now has source-only pre-copy admission, context-aware pending Add and
the original scoped seal/deadline protocol. Whole-carrier independent abort,
validated path/wrapper/PMTU evidence, packet-ID wrap and downstream buffer lifetime
remain separate work. Source size checks do not establish path MTU or interoperability.
Revision5 is a hopping-wrapper foundation, not the whole carrier retirement fix.
The legacy dial callback has no cancellation argument; Close may wait for its
actual return. A foreign socket Close/read/write/control may also remain blocked;
there is no hard Stop SLO or forced native retirement. Public-call/receiver counts
exclude external RawConn borrowers, socket/kernel storage, caller input, pools and
other carriers. Each dial must transfer a fresh exclusively owned socket, not an
alias. Constructor partial-close errors, whole-client offer replacement, QUIC
close-before-raw ordering, global retirement quotas and vectorised Salamander TX
need their own carrier/caller acceptance. Existing receive-loop termination on
read timeout and deadline-reset behavior remain a separate compatibility gap.
Revision6 adds the HY2 carrier protocol described above, without claiming total
native retirement. Wrapper construction before raw attachment and initial Hop
partial-error cleanup still need earlier socket custody. Custom QUIC providers'
hidden transports, HTTP/3 internal tasks, TCP stream borrowers, lower QUIC last
holders and process-wide quotas remain outside this join. Hysteria1 carrier offer
logic is not migrated by this HY2-only slice. No hard Stop SLO, resource/speedup
measurement, provider compatibility or actual protected-path proof is asserted.
The current Windows channel validator accepts VLESS/HY2 only and rejects Hysteria1;
the renderer likewise has VLESS/HY2 branches. Hysteria1 remains compiled/registered
upstream and its legacy library lifecycle is still an open reachability/acceptance
item; revision9 neither activates it nor removes that existing scope. QUIC raw and
sender metadata are paired revision5; remote-address aliases, path-manager and
lower/native/global retirement remain separate work. The historical read-reset
and early raw-custody gaps above have source changes in revisions8/7 respectively,
without runtime acceptance.
Revision10 adds client-internal HTTP/3 task and original qlogger completion,
not arbitrary external RequestStream/response body borrowers, custom TLS hidden
transports, the default HTTP Transport's dial/retry/pool owner or server lifecycle.
Those scopes, H1 library custody and global last holders remain OPEN.
Revision11 adds the HY2 TCP core-call boundary above; it does not close advanced
promoted-control borrowers, lower QUIC/TCP frame ownership or native/global scope.
Native TUN acceptance and update schema2 support remain disabled/unsupported.
Revision7 supersedes the earlier HY2 wrapper/initial-Hop socket custody and
Salamander TX gaps in source only. Generic other clients, custom hidden transports,
HTTP3/TCP/lower-QUIC/global last holders, hopping read-timeout reset, native/OS and
release acceptance remain open. The optional Salamander server caller shares this
Windows bundle implementation, but no server was deployed or executed.
Revision8 supersedes the wrapper-generated read-timeout/reset gap in source.
RawConn/native external deadlines, custom providers, Hysteria1 carrier ownership,
HTTP3/TCP/lower QUIC/global last holders and native/OS/runtime acceptance remain
separate work. No hard expiry/Stop SLO, clock/suspend proof or measured speedup.

Revision21 pairs the unchanged HY2 consumer with quic-go revision13. That local
module hard-fences sent history before packet allocation at its existing50000
per-number-space ceiling and propagates internal registration errors before
physical send. It retains original completed output batches through tracing/
history/queue handoff, retires known-unsent DATAGRAM/raw buffers and returns the
alternate path-probe borrowed buffer after actual WriteTo return. Existing
SendMode soft guards remain; no unbounded indexed-history claim is made. Public
HY2 APIs, quotas, dependency versions and licences are unchanged. Partial packer,
crypto/CC/raw/global/native and full acceptance remain OPEN. Source only; no
build/test/native execution. Receipts:
`.artifacts/memory/bootstrap-v411-sent-history-`.

Revision22 pairs the unchanged HY2 consumer with quic-go revision14. The original
packer retains partial raw buffers/consumed DATAGRAM entries through actual
packing return; known errors/empty output release owned resources, AppendPacket
borrows the Conn/GSO buffer, and uncertain intake prevents premature pool return
or a positive original-loop receipt. The synchronous SendProbe implementation now
returns its borrowed buffer after actual WriteTo return; existing void write-error
policy is unchanged. API/version/licence/consumer quotas remain unchanged. Whole
queue/provider/crypto/raw/global/native and full-PC acceptance remain OPEN. No
tests/build/native execution. Receipts:
`.artifacts/memory/bootstrap-v412-packer-buffers-`.

Revision23 pairs the unchanged HY2 consumer with quic-go revision15. Fixed
send-queue custody covers8 pending/worker1/probe1, accepted-vs-error transfer,
known-unsent raw/DATAGRAM retirement and an idempotent original Close receipt.
Conn checks original queue retirement before path replacement and rejects
negative loop evidence; short/partial/invalid raw counts no longer silently
succeed or replay through GSO fallback. Internal sender ABI changes are source
only; public HY2 APIs, dependency versions, licences and consumer quotas stay.
Raw/provider/crypto/CC/global/native/full-PC acceptance and measured performance
remain OPEN; no tests/build/native execution. Receipts:
`.artifacts/memory/bootstrap-v413-send-queue-`.

Revision24 pairs the unchanged HY2 consumer with quic-go revision16. Terminal
output keeps its original pooled buffer through actual Write/cache-copy return;
each transport retries from a separate bounded immutable backing. Closed handlers
consume incoming views, and expiry checks exact original identity (also for
remote closed handlers). This does not join the whole transport/control worker
or bound aggregate cache/timer/global retention. Public APIs/quotas/versions/
licences remain; no tests/build/native execution. Full-PC acceptance stays OPEN.
Receipts: `.artifacts/memory/bootstrap-v414-terminal-output-`.

Revision25 pairs the unchanged HY2 consumer with quic-go revision17. Its closed
cache reserves256 records/384KiB/64 IDs before allocation, fences original Conn
registration (including wrappedConn), and retains timer/queued/current retry
custody until actual return. Existing Transport.Close now joins original control
output and cache completion, including stateless-reset views and coalesced read
stop operations. Public signatures/quotas/versions/licences stay unchanged.
Internal runner/helper ABI, wire/concurrency/overload/runtime/native/resource/
performance acceptance remain unverified; raw/listener/root/global/crypto/CC/
OS/full-PC scopes stay OPEN. No tests/build/native execution. Receipts:
`.artifacts/memory/bootstrap-v415-closed-cache-`.

Revision26 pairs this unchanged HY2 consumer with quic-go revision18. Transport
keeps one original init/native abort/root/public Close/recorder result, joins
original listener/server response tasks and retains uncertain originals. Known
Conn teardown reuses receive-data/stream-call/send-data completion APIs. Direct
WriteTo reserves64 calls/4MiB exposed spans (each<=65535 bytes) before provider
entry; initializer waiters have a separate64-call ceiling. Repeated Close does
not retry native cleanup, and a borrowed deadline is cleared only after positive
joins. These are uncompiled source changes, not global/RSS/Stop/wire/performance
or OS acceptance. Raw receive/queue custody, CID/constructor/control/crypto/CC/
global/native/full-PC gates remain open. Public signatures/versions/licences and
HY2 quotas are unchanged; no tests/build/native execution. Receipts:
`.artifacts/memory/bootstrap-v416-transport-lifecycle-`.

Revision27 pairs this unchanged HY2 consumer with quic-go revision19. The Windows
basic reader retains its original provider/buffer through count validation and
abnormal exit. Transport fixes known local input drops and registers64 original
non-QUIC readers/4MiB exposed destination spans before waits; its fixed32 queue
is initialized once and drained once after original listener/caller completion.
Production stateless resets use nonwaiting Conn close, with no per-packet destroy
goroutine or reset-token log. Public signatures/versions/licences/HY2 quotas stay.
Conn/coalesced/undecryptable and server/0RTT/response queue/output, custom raw/OOB,
global/RSS/wire/native/OS/full-PC acceptance remain open; no measured speed claim.
No tests/build/native/product execution. Source-only receipts:
`.artifacts/memory/bootstrap-v417-raw-receive-`.

Revision28 pairs with quic-go revision20 and adds WaitForPacketInputShutdown to
the original HY2 QUIC close operation before its existing stream joins. Conn
intake uses64 fixed cells/64*20KiB backing-span admission, queue256 and a separate
anchored datagram processing cell. Unknown originals remain retained; known
queued/coalesced/undecryptable references release after actual returns. Pending
undecryptable views share one32-view cap across waiting/ready/replay stages.
The connection recorder joins intake calls and preserves Close errors. Existing
HY2 quotas, public APIs, versions and licences stay; the added quic-go completion
method requires this source pair. No tests/build/native/product execution or
measured speed claim. Server/response/global/native/full-PC scopes remain OPEN.
Receipts: `.artifacts/memory/bootstrap-v418-conn-input-`.

Revision29 pairs the unchanged HY2 consumer with quic-go revision21. Server
intake64/backing-span admission and queue1024 share a close fence; original
receive/0RTT/handoff/candidate views and response input/output/backing/OOB/provider
remain attached through actual returns. Known stop drains original queues; a
negative server result retains producers/rows and blocks a replacement. Reply
payloads are bounded1452 before output allocation/composition. No new worker or
timer, public API/version/licence/HY2 quota change. Constructor/context/trace,
generic CID/reset/global/native/full-PC acceptance remains OPEN; no measured
speed/RSS claim and no tests/build/native/product execution. Receipts:
`.artifacts/memory/bootstrap-v419-server-input-`.

Revision30 pairs the unchanged HY2 consumer with quic-go revision22. Expected
original Conn identity protects CID/reset removal and foreign registration;
generated CIDs are published only after all runner registrations succeed.
Handshake/0-RTT callers propagate known issuance failure. An additional path's
known CID conflict is withdrawn and returned to Probe without publishing its
PATH_CHALLENGE; it does not itself close the healthy existing connection.
Reset-token refusal requests nonwaiting connection close. Same-Conn generations,
full reset routing on migration, path/control/constructor/global/native/full-PC
acceptance remain open. Public APIs, versions/licences and HY2 quotas stay.
No tests/build/native/product execution or measured speed claim. Receipts:
`.artifacts/memory/bootstrap-v420-cid-authority-`.

Revision31 pairs the unchanged HY2 consumer with quic-go revision23. The server
retains its original context/cancel/trace/partial-Conn construction prefix.
Early handshake errors use common terminal cleanup; public completion also wakes
failed waiters, while separate success gates Dial/enqueue/NextConnection.
Dial/Accept recheck observed closing state. The original timer
is stopped on every exit; terminal normalization no longer mutates the published
close record. Client construction admission/locking, opaque providers, full
recreation/path/reset/global/native/full-PC acceptance remain open. Public APIs,
versions/licences and HY2 quotas stay; no tests/build/native/product execution or
measured speed claim. Receipts:
`.artifacts/memory/bootstrap-v421-connection-opening-`.

Revision32 pairs the unchanged HY2 consumer with quic-go revision24. Fixed64
opening cells retain original client construction; trace/provider/constructor
callbacks run outside transport metadata locks. Reserved CID publication and
generic additions share the close seal. The initial Transport keeps a promoted
client independently of CID rows until existing packet/stream retirement; unknown
outcomes retain it and publish failure. Root Close joins openings before original
client/map/trace/deadline cleanup; version recreation requires that retirement.
Construction slots do not limit established connections or global/opaque/RSS/
CPU/Stop resources. Reentrant callback Close, listener/init/custom providers,
full recreation/path/reset/global/native/full-PC acceptance remain open. Public
APIs/versions/licences/HY2 quotas stay; no tests/build/native/product execution or
measured speed claim. Receipts:
`.artifacts/memory/bootstrap-v422-client-opening-`.

Revision33 pairs the unchanged HY2 consumer with quic-go revision25. Each attempt
retires its own recorder before version recreation; recorder failure blocks the
existing original retirement gate. Standard qlog Writer Close is single-owner,
joins accepted event sends and preserves the original ordinary/abnormal result.
FileSeq joins its original event task before last-producer native close, preserves
encoding/close errors and retains an abnormal event/provider without retry.
Stopped-task selection wakes blocked event senders before their Close join.
Queue50/public APIs/versions/licences/HY2 quotas stay; no new worker/timer class.
Full trace/provider/count contracts, reentrant own callbacks, version validation,
listener/init/path/reset/global/native/full-PC acceptance remain open. No tests/
build/native/product execution or measured speed/overhead claim. Receipts:
`.artifacts/memory/bootstrap-v423-recreation-trace-`.

Revision34 pairs the unchanged HY2 consumer with quic-go revision26. VN checks
both immutable request CIDs before version-list allocation, rejects post-Retry/
processed/closing phases and fixes the decoder's destination/source field order.
No peer-IP/port restriction or new decoder/API/worker/timer/quota was introduced.
Read-only RFC9000 lookup supports echo/phase rules, not live interoperability,
peer/TLS identity or downgrade protection. Public APIs/versions/licences and HY2
quotas stay; no tests/build/native/product/live execution. Full VN/Retry/hopping/
listener/init/provider/path/reset/global/OS/full-PC acceptance remains open.
Receipts: `.artifacts/memory/bootstrap-v424-version-binding-`.

Revision35 pairs the unchanged HY2 consumer with quic-go revision27. Retry checks
the original request CID, version and phase before integrity/state work; received
qlog destination reflects its actual header. Existing parser empty-token rejection
and ResetForRetry next packet numbers are preserved, not reported as defects.
The callee adds a defensive length proof and uses immutable original destination
for the existing integrity algorithm. No new peer-IP/port gate or crypto/API/
worker/timer/quota change. Full Retry/TLS/route/hopping/listener/init/provider/path/
global/native/full-PC acceptance remains open; no tests/build/native/product/live
execution or measured speed claim. Receipts:
`.artifacts/memory/bootstrap-v425-retry-binding-`.

Revision36 pairs the unchanged HY2 consumer with quic-go revision28. One original
server-opening reservation precedes init/provider work. Constructor callbacks run
without transport metadata locks and retain their partial baseServer; server
publication and both existing worker starts share Close's seal fence. Root joins
the opening before snapshot/trace/deadline cleanup; abnormal outcomes retain the
original prefix. Existing packetaddr capture is reused for log metadata. No new
worker class/timer/API/protocol/dependency/license change; full init/provider/
selfClose/path/global/native/OS/full-PC acceptance remains OPEN. No tests/build/
native/product/live execution or measured speed claim. Receipts:
`.artifacts/memory/bootstrap-v426-server-opening-`.

Revision37 pairs the unchanged HY2 consumer with quic-go revision29. CID length
admission precedes socket wrapping/tuning/worker scheduling; original generator
is retained before its single length callback. Invalid lengths return the existing
protocol error, also in GenerateConnectionID before slicing/random input. Valid
zero/default/custom and crypto algorithms remain. No public API/version/license/
worker/timer/quota change; init/wrap/provider/factory/selfClose/path/global/native/
OS/full-PC acceptance remains OPEN. No tests/build/native/product/live execution
or measured speed claim. Receipts: `.artifacts/memory/bootstrap-v427-cid-init-`.

Revision38 pairs the unchanged consumer with quic-go revision30. Public Addr
factories admit TLS/resolve before owned socket acquisition; known convenience
Dial/Listen setup errors use original cached Transport.Close and preserve cleanup
errors. A private error retains the exact transport only while the error is held;
WrapError's existing Unwrap preserves that chain. This is not global retention,
retry suppression or abnormal-provider retirement proof. No public signature,
version/license/worker/timer/quota change. Full init/wrap/factory/provider/global/
native/OS/full-PC acceptance stays OPEN; no tests/build/product/native/live or
measured speed claim. Receipts: `.artifacts/memory/bootstrap-v428-setup-cleanup-`.

Revision39 pairs the unchanged consumer with quic-go revision31. Wrapper DF
defaults to unconfirmed false; Windows enables the existing PMTU capability only
when at least one original socket-option attempt reports success. Both failed or
missing observations do not grant capability. Control errors propagate, one-family
behavior and two attempts remain. This is not family/dual-stack/readback/hopping/
raw-provider/native/full-PMTU proof or measured speed. No public signature/version/
license/worker/timer/quota change; no tests/build/product/native/live execution.
Full init/provider/global/OS/full-PC gates stay OPEN. Receipts:
`.artifacts/memory/bootstrap-v429-df-admission-`.

Revision40 pairs the unchanged consumer with quic-go revision32. The original
initOnce/lifecycle retains one initializer prefix for the captured socket, returned
address/control provider and partial wrapper. Actual wrapping and abort share the
same socket. Known return clears the prefix; abnormal callbacks retain it with
the existing negative init outcome. Buffer template/write mirror changed together
without generation. No new worker/timer/channel/API/version/license/quota change;
provider async/global/native/Stop/full-PC gates remain OPEN. No tests/build/product/
generation/native/live or speed proof. Receipts:
`.artifacts/memory/bootstrap-v430-init-custody-`.

Revision41 pairs the unchanged consumer with quic-go revision33. PMTU reset now
receives handshake/config/actual target sendConn capability admission and leaves
refused probing disabled. Initial Start and due-probe output share that private
gate. Probe ACK/loss ownership uses the existing handler object's identity instead
of wrapping uint8 generation; stale/duplicate callbacks cannot update a later
probe. Existing allocations/search/RTT/payload algorithm remain. Full family/
readback/hopping/provider/migration/native/global/OS/full-PC gates stay OPEN.
No public signature/version/license/worker/timer/quota/crypto/CC change, no tests/
build/product/native/live or measured speed claim. Receipts:
`.artifacts/memory/bootstrap-v431-pmtu-reset-`.

Revision42 pairs the unchanged consumer with quic-go revision34. A fixed original
client path-switch prefix retains old/next queue/conn and target inputs; target
publication and existing worker start share its seal fence. Negative/abnormal
switching retains the candidate. Both initial/migrated queue tasks notify original
Conn close on Run error or abnormal exit; existing queue custody and panic semantics
remain. No extra worker/timer/retry/public API/version/license/quota/crypto/CC change.
Full constructor/family/hopping/target CID/multiowner/global/native/OS/full-PC gates
stay OPEN. No tests/build/product/native/live or measured speed claim. Receipts:
`.artifacts/memory/bootstrap-v432-path-switch-`.

Revision43 pairs the unchanged consumer with quic-go revision35. All sender
constructors retain metadata/partial sender in original parent openings. Conn
public metadata loads one immutable atomically published sender view; migration
nodes survive prefix clearing. GSO fallback flag uses atomic access; source audit
keeps the serialized first-permission-retry behavior unchanged. No new lock/worker/
timer/native retry/public signature/version/license/quota/crypto/CC change. Remote
aliases/point-loaded setters/provider/metadata API/target CID/multiowner/global/
native/OS/full-PC gates stay OPEN. No tests/build/product/native/live or speed proof.
Receipts: `.artifacts/memory/bootstrap-v433-send-metadata-`.

Revision44 pairs the unchanged consumer with quic-go revision36. Additional-path
registration has 64 original fixed cells and a separately promoted per-target
Conn set independent of CID/reset rows. Known unactivated-target refusal rolls
back only a new runner; unknown callback/rollback retains the original prefix.
Root joins registrations before snapshot and releases path rows only after actual
Conn input/stream/call retirement. AddPath gates handshake/closing/nil target
before init and rechecks closing afterward. No extra worker/native retry/public
signature/version/license/crypto/CC change; one registration completion channel
and fixed metadata are added. All-runner/path/global/reset migration/aliases/
provider/native/OS/full-PC gates remain OPEN. No tests/build/product/native/live
execution or speed proof. Receipts:
`.artifacts/memory/bootstrap-v434-path-ownership-`.

Revision45 pairs the unchanged consumer with quic-go revision37. Original CID
manager callbacks add/remove reset tokens on all existing transport runners.
A new path runner receives existing active/probing tokens before owner promotion;
known refusal removes its tokens before CID rollback, abnormal callbacks retain
the original admitted prefix. Existing runner interface/owner checks and failed
path CID retirement are reused. No new callback field/worker/lock/channel/native
retry/public signature/version/license/crypto/CC change. Delivery, all-path/global,
duplicate-token policy/custom provider/native/OS/full-PC gates remain OPEN.
No tests/build/product/native/live execution or speed proof. Receipts:
`.artifacts/memory/bootstrap-v435-path-reset-`.

Revision46 pairs the unchanged consumer with quic-go revision38. Path.Close
marks its original row and closes abandon under the existing mutex; the original
connection loop alone retires its CID/reset/frame state. Explicit cidAssigned
survives validation; pending retirement reuses its row without a new queue.
Original Conn Closing/Context.Done gate path admission and wake Probe. Active
path refusal remains; Close acknowledges abandonment, not native retirement.
No new worker/timer/lock/channel/native retry/public signature/version/license/
crypto/CC change. Concurrent Probe/validation channels/probe bounds/global/
aliases/provider/native/OS/full-PC gates stay OPEN. No tests/build/product/native/
live execution or speed proof. Receipts:
`.artifacts/memory/bootstrap-v436-path-close-`.

Revision47 pairs the unchanged consumer with quic-go revision39. Concurrent
Probe callers share stable attempt channels under fixed64 original call
admission, with bounded64 rows/unique queue and32 challenge ring. Shared backoff
saturates30s, each admitted caller reuses one timer, cancellation remains local.
Known return stops its original timer; abnormal call keeps a negative prefix.
Packet-input shutdown joins these calls before positive retirement. One manager
done channel/fixed metadata, no new goroutine/native retry/public signature/
version/license/crypto/CC change. Window/cadence compatibility, cross-transport
response binding, external Path/context graphs/lifetime runners/global resources/
provider/native/OS/full-PC gates stay OPEN. No tests/build/product/native/live
execution or speed proof. Receipts:
`.artifacts/memory/bootstrap-v437-path-probes-`.

Revision48 pairs the unchanged consumer with quic-go revision40. The server path
manager reuses an existing row without applying new-path pressure/eviction. The
proposed incoming-Transport response filter was rejected after reading primary
RFC9000 sections8.2.2-8.2.3: existing matching-nonce acceptance from any return path
is preserved. Actual client same-path response routing/padding/origin/CID/native
output remains OPEN. No new worker/retry/queue/limit/API/version/license/crypto/CC
change, tests/build/product/native/live execution or speed proof. Receipts:
`.artifacts/memory/bootstrap-v438-path-reuse-`.

Revision49 pairs the unchanged consumer with quic-go revision41. Client and server
active-peer PATH_RESPONSE entries retain input Transport/remote/packetInfo in
the existing256 framer queue. An original Conn prefix owns CID/pack/trace/write;
padded replies drain under SendModeAny/pacing with ordinary shared-Conn flight
accounting. The original Transport write admission retains packetInfo/OOB; public
WriteTo/default and matching-nonce response acceptance stay. Known stale/no-CID
routes are dropped without another-path fallback. No new queue/worker/timer/native
cleanup retry/public signature/version/license/crypto/CC algorithm. Server non-
active/antiamp/data-CID privacy/per-path RTT-CC/kernel source/aliases/global/native/
OS/full-PC gates remain OPEN. No tests/build/product/native/live or speed proof.
Receipts: `.artifacts/memory/bootstrap-v439-path-replies-`.

Revision50 pairs the unchanged consumer with quic-go revision42. Its existing
bounded3 non-active server path records retain original route metadata, a
saturating3x input allowance and fixed32 sent-nonce/size history. Original Conn
output reserves credit before callbacks; known unused bytes return only to the
same record, attempted/unknown output keeps the debit. Sized packing preserves
the default1200 and permits minimal short packets only under limited credit.
Address and MTU validation are separate, with a fresh full-size follow-up in the
original loop. Loss renews the pending nonce without immediately deleting the
path, and switching uses the highest non-probing app-data PN including0RTT.
Matching responses may still arrive on any path. No new worker/timer/public API/
version/license/crypto/CC algorithm or native cleanup retry. Initial/bypass/global
antiamp, ACK-without-response timeout/backoff, multiple challenge response queue,
CID promotion/privacy/lifetime, per-path RTT-CC/kernel source/OS/full-PC acceptance
remain OPEN. No tests/build/product/native/live execution or speed proof.
Receipts: `.artifacts/memory/bootstrap-v440-path-credit-`.

Revision51 pairs the unchanged consumer with quic-go revision43. Original bounded
server path records now own a response clock independent of packet ACK/loss.
The existing Conn timer/loop schedules at most3 own challenges per address/MTU
phase using1/2/4 PTO windows, a7PTO budget, conservative initial floor and larger
current RTT PTO. Pending CID/credit refusal keeps expiry without a past retry
alarm. ACK never validates or cancels the clock; loss cannot accelerate it.
Small address response starts the full-size phase once or preserves an already
outstanding expanded probe; matching sent history still accepts any return path.
Expired responses cannot revive the phase. Expiry retains original CID retirement
before callbacks, removes only that candidate and keeps the working connection.
No new timer/worker/queue/public API/version/license/crypto/CC algorithm. Initial/
bypass/global amp/every-challenge replies/dataCID/privacy/CC/kernel/native/OS/
full-PC gates remain OPEN. No tests/build/product/native/live or speed proof.
Receipts: `.artifacts/memory/bootstrap-v441-path-timeout-`.

Revision52 pairs the unchanged consumer with quic-go revision44. Every normal
server non-active challenge enters the existing256 response queue with original
route/scalar pathID, after authenticated once-only input credit. Immediate
last-frame server output is removed. Own validation/replies share original
credit/pack/write custody; replies use normal shared-Conn flight accounting.
A due own challenge can accompany the response, with known no-fit response-only
fallback before native output. Deferred CID/credit/fit rotates in the same queue
with a bounded original-length scan; stale IDs never borrow a replacement row.
Known promoted path replies preserve the original path CID. Unknown original
prefixes/debit remain negative with an explicit error. Initial/bypass amp/replies,
terminal queue metadata/dataCID/privacy/CC/kernel/global/OS/full-PC gates remain
OPEN. No new queue/timer/worker/public API/version/license/crypto/CC algorithm,
tests/build/product/native/live or measured speed proof.
Receipts: `.artifacts/memory/bootstrap-v442-server-replies-`.

Revision53 pairs the unchanged consumer with quic-go revision45. Its original
Conn final guard retires path-response queue/server-row metadata only after
actual loop/input/drain/send-queue/output/packer return receipts. Routed,
deferred and legacy response admission is sealed under the existing mutex,
queued route/frame/backing references cleared, server callback closures released.
Nested negative publication and original retirement markers prevent a positive
input shutdown receipt before this cleanup. The public input-shutdown method
keeps its outgoing Probe join and now requires this metadata receipt. Canonical
closed-packet release clears the retained Transport stamp after actual pool
callbacks. No new native cleanup/callback/worker/timer/queue/public API/version/
license/crypto/CC algorithm. Whole-framer/stream/ACK/outgoing/runner/native/global,
bypass/dataCID/privacy/CC/kernel/OS/full-PC gates remain OPEN. No tests/build/
product/native/live execution or measured speed proof. Receipts:
`.artifacts/memory/bootstrap-v443-path-retirement-`.

Revision54 pairs the unchanged consumer with quic-go revision46. The existing
stream-call owner automatically retires original framer metadata after the loop
and all6 registered call classes actually return. One finalizer runs on that
loop or last caller, outside registry/stream locks; positive allDone follows
known cleanup. Generic queue/maps/ring/flow references are cleared behind sealed
admission, with uniform control-then-stream lock order and atomic overflow flag.
Unknown cleanup retains the original framer/marker and separate failure. The
receive-only receipt keeps its scope; WaitForStreamCallsShutdown now includes
framer metadata, already consumed by existing HY2 close. No new worker/timer/
queue/wait/public API/version/license/crypto/CC algorithm, tests/build/native/live
or speed proof. Stream/ACK/window/outgoing/raw/runner/global/bypass/dataCID/CC/
kernel/OS/full-PC acceptance remain OPEN. Receipts:
`.artifacts/memory/bootstrap-v444-framer-retirement-`.

Revision55 pairs with quic-go revision47. HY2's original QUIC close task now
consumes the separate WaitForStreamMapShutdown API after packet-input shutdown,
alongside the existing receive/all-stream-call/send-data joins. It joins actual
bounded Open/Accept invocations and the original loop; six-class stream-call
scope stays unchanged. Fixed512 admission precedes map locks/context callbacks/
wait-channel allocation, retains original child/context/candidate on abnormal
return, and survives 0-RTT child replacement. Entry/result gates read original
atomic first-close state; extra pressure refuses one operation. Outgoing waiter
allocation is bounded, context methods run outside child locks with held-state
unwind guards, nil context and closed-Done/nil-Err are controlled failures.
No new close task, worker, timer or module/version/license/crypto/CC algorithm
change, tests/build/native/live or performance proof. Partial factories/native
map callbacks, retained/skipped rows, stale-generation IDs and global/native/
OS/full-PC acceptance remain OPEN. Receipts:
`.artifacts/memory/bootstrap-v445-stream-map-calls-`.

Revision56 pairs the unchanged consumer with quic-go revision48. Each original
incoming map reserves a whole skipped-ID factory batch before allocations,
with4096 charged rows and16MiB logical metadata credit, not RSS/data proof.
Advertised wire limits remain unchanged; local pressure returns InternalError.
Factories/native close execute behind one original operation gate outside the
map metadata mutex. Original batch/candidate/result/unspent credits and retiring
stream survive abnormal return; close seals/wakes once, freezes map writers and
walks original rows without another snapshot. Known repeat is idempotent;
unfinished repeat refuses instead of cleanup replay. Completed unaccepted rows
remain charged, only known unused suffix/deletion refunds; no naive row clear.
Actual HY2 consumes the same input/map-call/receive/all-call/send-data joins.
Outgoing factories/close, partial constructor internals,0RTT generation custody,
retained rows/global/native/OS/full-PC gates remain OPEN. No new worker/timer/
queue/API/module/version/license/crypto/CC algorithm, tests/build/native/live or
speed proof. Receipts:
`.artifacts/memory/bootstrap-v446-incoming-map-lifetime-`.

Revision57 pairs the unchanged consumer with quic-go revision49. Outgoing maps
reuse the4096-row/16MiB logical metadata policy before factories. Direct local
pressure rejects one call; Sync waits its existing bounded FIFO on actual local/
peer credit, with deletion/head-identity wake and explicit pressure outcomes.
Original factory/native-close gate runs those callbacks outside metadata mutex,
retains unknown ID/factory/candidate/waiter/credit/retiring stream, seals/wakes
once before gate wait and keeps rows after void return. Window/reset updates
serialize with factory publication so no candidate misses them; their callbacks
remain a separate lock/prefix scope. Exact map uncertainty escalates through the
existing public operation registry/root close, outside lock and inside actual
call lifetime. HY2 uses its same shutdown joins; no API or new worker/timer/queue/
module/version/license/crypto/CC algorithm, tests/build/native/live or speed proof.
Partial constructors/control callbacks/0RTT/retained rows/global/native/OS/full-PC
gates remain OPEN. Receipts:
`.artifacts/memory/bootstrap-v447-outgoing-map-lifetime-`.

Revision58 pairs the unchanged consumer with quic-go revision50. Outgoing window/
reset stream callbacks run outside map metadata locks, ordered by the existing
factory/update/close gate. One original snapshot owner reserves4096 elements/
64KiB element backing before allocation using actual generic element size and
overflow-safe arithmetic. Original targets are captured under lock, callbacks
hold their exact current stream through actual return, and unknown allocation/
capture/callback retains snapshot/kind/limit/index/credits without retry. Known
close skips an uncalled tail; known metadata cleanup precedes gate release.
Existing shutdown joins are unchanged; metadata clear is not native/GC/RSS proof.
No new worker/queue/timer/public API/module/version/license/crypto/CC algorithm,
tests/build/native/live or speed proof. Scalar control callbacks/partial objects/
0RTT/retained rows/global/native/OS/full-PC gates remain OPEN. Receipts:
`.artifacts/memory/bootstrap-v448-outgoing-updates-`.

Revision59 pairs the unchanged consumer with quic-go revision51. Outgoing
STREAMS_BLOCKED/incoming MAX_STREAMS callbacks run outside metadata/factory
locks with one original actor/current frame/callback and one monotone pending
scalar. Nonwaiting gate reservation and atomic running/gate release avoid lost
handoff; reentrant requests record pending without self-wait. Unknown original
frame/callback remains sealed without retry. Native close joins that actual
actor before the factory gate and old-map return/0RTT framer reset, without
claiming all old stream/ID callbacks joined. MAX_STREAMS count arithmetic is
bounded before ID multiplication/addition. Existing HY2 shutdown APIs unchanged;
no worker/timer/event queue/module/version/license/crypto/CC algorithm change,
tests/build/native/live or speed proof. Partial constructors/parameters/old0RTT
IDs/retained rows/global/native/OS/full-PC gates remain OPEN. Receipts:
`.artifacts/memory/bootstrap-v449-stream-map-control-`.

Revision60 pairs the unchanged consumer with quic-go revision52. All four actual
stream factories use original child-specific senders and a nonzero immutable
generation token. Completion targets the exact original child; data/control/
removal compare token under existing framer locks. Initial framer precedes maps,
original bindings are required before exposure, 0RTT swaps token and clears old
data/control getters under both locks, terminal retirement clears token. Atomic
connection-data hint is not a flow-epoch proof. Private constructor/reset call
signatures change with the sole actual Conn caller; public APIs and HY2 joins
remain unchanged. No worker/timer/queue/module/version/license/crypto/CC change,
tests/build/native/live or speed proof. Parent parameters/reset support/old flow
mutation/partial objects/retained rows/global/native/OS/full-PC gates remain OPEN.
Receipts: `.artifacts/memory/bootstrap-v450-stream-generations-`.

Revision61 pairs this unchanged consumer with quic-go revision53. Stream
constructors now capture immutable original-generation scalar transport
parameters, separating received/incoming from applied/outgoing publication.
The private flow factory no longer borrows live peerParams; native reset updates
are conditional and accepted 0-RTT cannot disable remembered reset support.
Public consumer/shutdown contracts, dependencies and wire version are unchanged.
Source/manual review and touched-only gofmt only; no tests/build/native proof.
Old flow/global counters, other peerParams readers and resource retirement
remain separate open gates.

Revision62 pairs this unchanged consumer with quic-go revision54. Native stream
flows retain a by-value original connection-flow owner/epoch scope. Successful
0-RTT Reset and all stream-to-connection counter/window/tuning operations use
the same original mutex, fencing late old scopes without rebinding them.
Native connection send methods share that mutex. Public/shutdown contracts and
dependency/wire versions are unchanged. Extra locking cost and resource/native
retirement remain unmeasured/unverified; no tests/build/native execution.

Revision63 pairs this unchanged consumer with quic-go revision55. Fixed original
map creation prefixes now retain known partial stream/flow/halves/context results
before later constructor work. Exact child-bound factories and original gates
remain; unknown construction retains its original owner/credits without retry
or unsafe partial cleanup. Sorter shells precede native initialization.
Public/shutdown contracts and dependency/wire versions are unchanged. Hidden
context/provider internals, actual cleanup/row retirement and whole resource
bounds remain open; no tests/build/native execution.

Revision64 pairs this unchanged consumer with quic-go revision56. Native stream
shutdown has explicit per-half local results and original receipt state; child
maps retain originals on error. Send contexts are explicitly canceled on native
shutdown/0-RTT rejection outside the stream mutex, after waking writers.
Ordinary close errors propagate through all four child maps and native Conn;
failed cleanup prevents 0-RTT replacement and a positive terminal-loop receipt.
Public/shutdown consumer APIs and dependency/wire versions are unchanged.
Local cleanup result is not public-call/ACK/borrower/row/native quiescence;
those gates remain open. No tests/build/native execution.

Revision65 pairs this unchanged consumer with quic-go revision57. Original0RTT
reset keeps old/candidate map sets and stages; native close/flow reset precede
candidate publication outside parent metadata locks. Framer old/next identities
are checked, pending/failed reset access returns an error via NextConnection.
No retry or invented flow rollback; actual row/last-holder retirement remains
separate. Public APIs and dependency/wire versions are unchanged. No tests/build/
native execution or speed/resource proof.

Revision66 pairs this consumer with quic-go revision58. The actual HY2 QUIC close
task calls WaitForStreamMapRetirement after requesting connection close. The new
aggregate composes the prior packet/map/receive/all-call/send receipts, then
retires map metadata through one original finalizer. Native Transport uses the
same aggregate. No extra HY2 task/worker, dependency or wire version change.
Existing stream/context APIs and immutable generation bindings are retained;
raw/ACK/crypto/native/OS/GC/RSS retirement remains separate. No tests/build/native
execution or speed/resource proof. A compatible quic-go bundle with the new
method is required; removing only its local replace is not a source rollback.

Revision67 pairs the unchanged consumer with quic-go revision59. Public peer
parameter reads use immutable observed/outbound scalar snapshots, with distinct
restore/receive/apply/rejected0RTT timing. Actual DATAGRAM queue admission and
history transfer share the outbound publication lock and revalidate exact peer
frame limits; all waiting producers observe a head-change broadcast. Queued
unsent owners retain their prior custody until fresh apply/drop/Close. No new
HY2 API/task/worker, dependency or wire version change. Source/manual review and
touched-only gofmt do not prove runtime interoperability, speed or resources.
Receipts: `.artifacts/memory/bootstrap-v457-peer-parameters-`.

Revision68 pairs the unchanged consumer with quic-go revision60. Initial Conn
constructors retain original partial stream/map/ACK/congestion/TLS graphs before
later callbacks; actual client/server promotion requires completed construction.
Unknown openings remain original-owned without a partial native Close/retry.
Old constructor wrappers use the same initializers, with no new HY2 API/task,
dependency or wire version change. Ongoing raw/ACK/crypto/global/native retirement
and compilation/interoperability/performance acceptance remain open. Source/manual
review and touched-only gofmt only. Receipts:
`.artifacts/memory/bootstrap-v458-connection-construction-`.

Revision69 pairs the unchanged consumer with quic-go revision61. Native Retry/
path-migration transitions retain original packet/congestion candidates through
callbacks; shared bounded congestion publication removes foreign setter callbacks
from metadata locks. Public SetCongestionControl keeps its void signature and
requests connection close on refused/abnormal setup, including overlapping calls.
Actual Closing and terminal negative-state gate prevent a false positive loop
receipt; all custom-controller call joins/aliasing/nonreturn and whole raw/ACK/
crypto/global/native retirement remain separate. No new HY2 task/API/dependency/
wire version; source/manual/gofmt only, no tests/build/native proof. Receipts:
`.artifacts/memory/bootstrap-v459-packet-transitions-`.

Revision70 pairs with quic-go revision62 and changes the actual HY2 QUIC close
task: after original CloseWithError, join WaitForCongestionControlShutdown before
stream-map metadata retirement. The native Transport uses the same new receipt.
Independent raw/QUIC/Transport aborts still start before waits; no new HY2 task,
worker, dependency or wire version. Original public setter input/handler and
outer close-request path stay registered until known return; unknown stays
retained/negative. Native-loop and external setter receipts remain distinct.
The new additive quic-go method is required by this consumer. Opaque callback
self-join/nonreturn, hidden tasks/all-controller/native/OS/resource acceptance
remain open. Source/manual review and touched-only gofmt, no tests/build/native
proof. Receipts: `.artifacts/memory/bootstrap-v460-congestion-calls-`.

Revision71 pairs with quic-go revision63 and changes the actual HY2 QUIC close
task to join WaitForDatagramSendDataShutdown after congestion and before stream-
map retirement. Its owned-entry ledger and separate producer reference preserve
DATAGRAM payloads through original copy/enqueue/scheduling return. Only positive
native-loop and producer receipts permit bounded history-data preflight/release;
unknown state stays retained/negative, without retry or fabricated delivery.
Independent raw/QUIC/Transport aborts still precede waits; no new HY2 task/worker,
dependency or wire version. The new additive method requires a compatible pair.
Scope/public-call completion, receive/ACK metadata, hidden tasks/native/OS/GC/RSS
and performance proof remain separate. Source/manual/gofmt only; no tests/build/
native execution. Receipts: `.artifacts/memory/bootstrap-v461-datagram-retirement-`.

Revision72 pairs with quic-go revision64. The existing HY2 QUIC close task now
joins WaitForPacketHistoryRetirement, which preserves the previous native-loop/
congestion/DATAGRAM/stream-map data and call joins before sent-history metadata
retirement. Native Transport consumes the same aggregate before path retirement.
Original close errors remain joined; independent aborts still precede waits.
No additional task/worker, dependency/wire version or fabricated ACK/loss/delivery.
The history finalizer clears owned reference cells without reading recycled
packets/frames or invoking handlers/pools. Unknown state stays retained/negative;
crypto/control/retransmission, provider/OS/resource acceptance remain separate.
Source/manual/gofmt only, no tests/build/native proof. Receipts:
`.artifacts/memory/bootstrap-v462-packet-history-`.

Revision73 pairs with quic-go revision65. The existing HY2 QUIC close task now
uses WaitForCryptoQueueRetirement, preserving the native/public-call/data/map/
sent-history joins and adding original native TLS Close completion before local
crypto/retransmission heads are dropped. A returned TLS handshake error remains
the API error while original-call quiescence has its own receipt; abnormal work
retains its prefix without retry. Framer sealing and original identity/sorter
preflight precede cleanup. No data/key wipe, fabricated ACK/loss/pool operations
or added task/worker/dependency/wire version. TLS state/provider aliases,
received-history, byte budgets/native/OS/resource acceptance remain separate.
Source/manual/gofmt only, no build/tests/native proof. Receipts:
`.artifacts/memory/bootstrap-v463-crypto-queues-`.

Revision74 pairs the unchanged consumer with quic-go revision66. Existing
WaitForCryptoQueueRetirement now reaches sent AND received ACK-history retirement
through its original history aggregate. The original receiver is pointer-bound
to actual client/server packers and ACK callbacks; all tracker identities/ranges
are preflighted before internal heads and ACK scheduling are retired. Cached ACK
objects/backings and historical ECN/observed/connection counters are preserved.
No new HY2 API/task/worker/dependency/wire version; negative original state stays
retained without retry. TLS internals/byte budgets/provider/native/OS/performance
remain separate. Source/manual/gofmt only, no tests/build/native proof. Receipts:
`.artifacts/memory/bootstrap-v464-received-history-`.

Revision75 pairs the unchanged consumer with quic-go revision67. Native TLS
write/session-ticket data is copied before the SDK borrow expires;128 pending
events/8MiB queue-owned clone credit is reserved before allocation. Explicit local
EventError and Go1.26 QUICErrorEvent failures reach the actual Conn instead of
readiness. Existing crypto aggregate retires native pending events only after
original loop/data/history/TLS and local sorter preflight. No new HY2 task/worker/
API or dependency/wire version; the fork minimum now matches Windows Go1.26.
No data/key wipe, arbitrary-provider/global memory or speed claim. Source/manual/
gofmt only, no tests/build/native proof. Receipts:
`.artifacts/memory/bootstrap-v465-crypto-events-`.

Source rollback: restore a compatible source/manifest pair; removing only the
quic-go replace while this consumer requires NewDatagramSendScope, Closing and
WaitForStreamReceiveShutdown/WaitForStreamCallsShutdown/WaitForStreamMapShutdown/WaitForStreamSendDataShutdown/WaitForPacketInputShutdown/WaitForStreamMapRetirement/WaitForCongestionControlShutdown/WaitForDatagramSendDataShutdown/WaitForPacketHistoryRetirement/WaitForCryptoQueueRetirement cannot compile
against stock mod4. Rebuild only after explicit authorization for builds/checks.
Retain source/evidence for review; do not overwrite module-cache sources.
