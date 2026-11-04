# R28-Z05 — finite handoff

**READY_FOR_REVIEW**, 2026-10-28. Пакет: [REPORT](REPORT.md), [SOURCES](SOURCES.md).

## R28-Z05-A — конкретный ready callback не достигает HY2 timer

**Claim (source-scoped negative):** SFA `ACTION_DEVICE_IDLE_MODE_CHANGED` →
`CommandServer.pause/wake` достигает общего `pause.defaultManager`, но в exact
pins HY2/quic-go не регистрирует callback, не ждёт `WaitActive` и не читает
PauseManager. Manager оповещает только opt-in consumers; доказаны NTP/URL-test
Ticker и WireGuard paths. Therefore ready callback already suppresses neither
HY2 PING/PTO/idle timer nor UDP write.

Evidence: SFA `bc73504f…`, sing-box1.13.19/b5ebaa…, sing0.8.13/7c349d…,
sing-quic d838…, quic-go mod.4/ccd46…; negative search restricted to named
non-test roots/tokens. No whole-ecosystem/future impossibility claim.

**Strongest opposition:** one manager is constructed before outbounds and
available through context. Rejected as sufficient: availability is not a
consumer edge; RemoteRuleSet even stores manager without using it. quic-go owns
an independent `time.NewTimer` run loop.

**Proposed verdict:** `ACCEPT_SOURCE_SCOPED_NEGATIVE` — subject to independent
lead review and unchanged pins. Not product/native proof.

## R28-Z05-B — допустимое решение без wake/radio/NAT outcome

**Conditional model claim:** use observation/classification, not an assumed
energy optimization:

1. authority invalid or owned physical context blocked/lost/changed → fail-closed
   retirement/replacement;
2. same context + authority valid + no actual carrier failure → keep carrier;
3. typed carrier failure or accepted path invalidation → reuse existing HY2
   `InterfaceUpdated/CloseWithError → lazy one-pending offer` recovery;
4. pause only exact optional subscribers already known (URL-test/NTP); do not
   generalize PauseManager to HY2;
5. energy/NAT value remains `unknown/abstain` until wake→due→queue→socket→peer and
   useful recovery costs are measured.

No auto battery exemption, broad WakeLock, extra ping, blanket close-on-wake or
new core timer seam follows from source alone.

**Strongest opposing trace:** FGS may remain foreground/network-allowed while CPU
is suspended, so no PING executes. On resume `elapsedRealtime` authority may be
expired while native transport clock/timer did not age. Java watchdog/gates retire,
but native quic socket writes do not cross those gates per packet; strict zero
post-expiry write ordering is not source-proven. Keepalive never renews authority.
Reboot creates new process/boot domain; old authority deadline must not restore.

**Proposed verdict:** `ACCEPT_MODEL_SCOPED` for a minimal owner-controlled seam;
strict packet barrier and energy benefit remain future platform evidence.

## Reuse / licence / cost

- SFA caller `bc73504f…`: **REFERENCE/ADAPT** only; GPL-3.0-or-later plus clause
  against derivative app name/association; whole service incompatible with
  Snowden owner/safety. No code copied.
- sing pause manager/ticker v0.8.13: **REUSE** for matching optional subscribers;
  GPL-3.0-or-later. Synchronous callbacks under manager mutex add reentrancy/
  latency cost. Adding HY2 subscriber is a reviewed core lifecycle change.
- HY2 interface-update recovery: **REUSE CONDITIONALLY** after path invalidation
  or typed carrier failure; reconnect handshake/CPU/radio/latency cost unknown.
- quic-go timer ccd46…: **REUSE unchanged**, MIT. No pause hook proven.
- Current physical-context/authority guards: **ADAPT as decision inputs**, but
  callback delivery is not atomic with in-flight native packet writes.

## Work stages completed

1. Source/pin/artifact/caller availability — complete; shipped AAR still unknown.
2. Android receiver→JNI→shared manager trace — complete.
3. Exact subscribers + bounded HY2 negative search — complete.
4. Awake/Doze/AppStandby/LPS/blocked/suspend/resume/reboot table — complete.
5. Opposing FGS-suspended, network-exempt/no-timer, authority ordering — complete.
6. Reuse/licence/cost + paired C04 scoped read — complete.
7. Future distinguishing observations — finite and all NOT_RUN.

## Open / future evidence (NOT_RUN)

Exact accepted artifact; callback latency before suspend; transport due/queue/
socket stages; peer/NAT state; same-path conditional reuse vs blanket reconnect;
changed-path no-old-write barrier; authority expiry/reboot and in-flight kernel
boundary; device/OEM/RAT/energy/recovery outcomes. No numeric savings or timeout
is inferred.

Lead should compare C04's lifetime/cancellation contract with this packet's
pause-edge negative; agreement is not independent runtime evidence.
