# R28-Z04 — HANDOFF

28 октября 2026.

- status: READY_FOR_REVIEW
- report: [REPORT](REPORT.md)
- sources: [SOURCES](SOURCES.md)
- artifacts: `.artifacts/research/r28-contracts/Z04/`

## New scoped claims

### R28-Z04-A — holder composition, not a global queue cap

**Claim:** current local counts/window limits remain valid only within their
unit/owner. Global retained-memory/time accounting requires separate bounds on
pre-Add callers, post-dequeue history, fragment aliases/groups, assemblies,
handlers/sessions, sampler PN-span capacity and active/retiring generations.

**Evidence:** exact current module commits byte-checked against local cache;
manual owner/release trace. Conditional arithmetic is REPORT §2.2. It is not RSS,
CPU, kernel or field measurement.

**Strongest opposing guard:** queue8/32/64/128, history40k/50k, handshake/idle,
anti-amplification, raw1024/accept32 and Windows Job reap materially bound their
own scopes. The claim does not say “unbounded everything”. Counterexample to
blanket composition: N blocked SendDatagram callers already own copied D outside
queue32 while 32 dequeued frames can remain in sent history and 8 unrelated
serialized packet buffers occupy sendQueue.

**Recommended verdict:** `ACCEPT source/model scope` if lead confirms inputs;
not a global DoS or RSS verdict.

### R28-Z04-B — coupled sender/controller remediation

**Claim:** quic-go
`f7b7e9c4c6f6ec45e4ab2b4788eca63192975d80` + sing-quic
`4ab2eceaac81e073f53b22ec72ff56aaea89a3d1` is the ready source pair that fixes
the named injected-controller reporting mismatches: prior flight, one packet
identity across spaces, install fence, neutered/drop/migration reporting,
least-unacked cleanup, app-limited suppression and full Ex Brutal/monotime.

**Strongest opposing trace:** PTO `QueueProbePacket` in candidate still removes
engine history and clears frames without an explicit `OnPacketNeutered` or
`notifyLeastUnacked` call in that path. Therefore the pair does not by itself
prove full sampler retirement or resource bound. PMTU lost-list semantics also
remain an explicit gate.

**Reuse recommendation:** `ADAPT_AS_COUPLED_PAIR`; do not recommend global
upgrade or one-file backport. Exact candidate go.mod moves quic-go/sing/utls/x/*
and caller APIs; licences MIT + GPL-3.0-or-later with whole-binary closure cost.

**Recommended verdict:** `ACCEPT scoped semantic pair; NEEDS_REVISION for blanket
resource/memory fix`.

### R28-Z04-C — existing admission guards are stage-local

**Claim:** current transport has meaningful queue, retry, callback, handshake,
anti-amplification and tracked-packet guards, but pinned HY2 Service does not
wire pre-handshake `GetConfigForClient/ConnContext`, sets huge stream allowance,
launches per-accepted-connection and per-new-UDP-session work, and has no local
active connection/session cap. Global bound needs external admission assumptions.

**Strongest opposing case:** invalid auth is checked against userMap and receives
masquerade handling; timeouts/queue drops/Job owner reduce exposure. No remote
DoS/reachability/frequency claim is made.

**Recommended verdict:** `ACCEPT source contract gap, DEFER platform policy`.

## Semantic decision matrix

| Option | Decision | Reason |
|---|---|---|
| Current pair unchanged | reject as semantic baseline for injected BBR/Brutal correctness | Brutal batch not selected; post-flight passed; identity/terminal gaps |
| Backport only Brutal Ex/monotime | defer/partial | batch dispatch improves but sender prior/identity/terminal semantics remain old |
| Subtract L in old sampler | reject | wrong for prior callers, identity/lifecycle untouched |
| New sender + old BBR with no-op shim | reject | stateful sampler loses required terminal cleanup; method compatibility not semantic compatibility |
| New pair f7b7e9c+4ab2ece | adapt/accept scoped | matching public contracts and constructors; residual PTO/PMTU/resource/admission gates explicit |

## Open/unknown

- PTO sampler retirement intent/fix, PMTU classification and close-time sampler
  disposal need explicit semantic decision.
- Actual number of callers/sessions/connections/generations and physical allocator
  capacities/RSS are unmeasured; engine Job memory cap is not configured.
- Whole core dependency/API/build/native compatibility and performance effect
  are NOT_RUN.

No code, plans, dependencies, roles, bindings, network, devices or real profiles
were changed or used.
