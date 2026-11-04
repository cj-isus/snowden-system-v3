# R28-Z05 — sources and provenance

Read/capture date: **2026-10-28**. Public captures and selected hashes: `.artifacts/research/r28-contracts/Z05/`. No execution/build/device/network probe.

Statuses: `FULL_TEXT_READ`, `SECTIONS_READ`, `SOURCE_READ`, `LINK_ONLY`,
`FETCH_FAILED`. A source file read is not shipped artifact proof.

## 1. Project and baseline inputs

| ID | Input | Read depth / role |
|---|---|---|
| L01 | `android/app/build.gradle.kts` | `SOURCE_READ`: compile/target36, min29, dev version; no build |
| L02 | `android/vpn/src/main/AndroidManifest.xml`, `SnowdenVpnService.kt`, `WorkerProcess.kt` | `SOURCE_READ`: systemExempted/private process/always-on false, watchdog/authority/retirement callers |
| L03 | `android/profile/ProfileClock.kt` | `SOURCE_READ`: elapsedRealtime projection, process/boot-local deadline, must not restore old cache |
| L04 | corebridge `NativeCallbackGate`, `NativeWorkerSession`, `LibboxPlatform`, `PhysicalNetworkBinding`, `PhysicalPathContext`, `ProtectedSocketControl`, `NativeEvents` | `SOURCE_READ`: JNI/platform admission, selected path/blocked context, callback mailbox, transport socket protection and retirement boundary |
| L05 | `android/native/source-lock.json` | `FULL_TEXT_READ`: sing-box v1.13.19/b5ebaa…, current sourceRevision3; AAR hash and Go remain null; source adaptation unbuilt/unverified. Initial snapshot saw revision2; final drift review recorded the narrow 2→3 change while relevant pin/artifact fields remained stable |
| B01 | `research/04-PERFORMANCE/REPORT-011.md`, `SOURCES-011.md` | `SECTIONS_READ`: pause boundary, power matrix, exact HY2 timer and open unknowns |
| B02 | `performance/QUIC-ACCOUNTING-AND-LIMITS.md` | `FULL_TEXT_READ`: peer timeout, clock/wake separation, resource/lifetime distinctions |
| PAIRED | `cycles/.../C04/HANDOFF.md` | `FULL_TEXT_READ` narrowly at review stage; corroborating resource/cancellation boundary only |

Hashes: `selected-inputs.json` (30 selected inputs). Sequential hash cut, not
atomic repository snapshot.

## 2. Exact upstream source pins and callers

### U01 — Android caller reference

- Repository: `SagerNet/sing-box-for-android`
- Commit: `bc73504fcb858400a939140ad785ed92f4c9f429`
- File: `app/src/main/java/io/nekohasekai/sfa/bg/BoxService.kt`
- Symbols/sections read: `idleModeUpdates`, receiver
  `ACTION_DEVICE_IDLE_MODE_CHANGED`, `serviceUpdateIdleMode`, registration and
  destruction lifecycle.
- Capture URL:
  `https://raw.githubusercontent.com/SagerNet/sing-box-for-android/bc73504fcb858400a939140ad785ed92f4c9f429/app/src/main/java/io/nekohasekai/sfa/bg/BoxService.kt`
- Status: `SOURCE_READ`, captured and hashed.
- License file at same commit: GPL v3-or-later text plus clause forbidding a
  derivative work from using the app name or implying association without consent.
  Status: `FULL_TEXT_READ`. Decision: reference/adapt; no copied product code.

### U02 — shared manager construction and libbox call

- Repository: `SagerNet/sing-box`, v1.13.19,
  commit `b5ebaa1fc0f2b94256180b95468e73ef53caa27d`.
- `box.go`: `Box.New`, line-scope around `pause.WithDefaultManager(ctx)` before
  network/router/outbound creation.
- `daemon/instance.go`: context manager retained as `Instance.pauseManager`.
- `experimental/libbox/command_server.go`: `Pause`, `Wake`, `ResetNetwork`.
- Public captures of `box.go` and `command_server.go` match local module bytes
  exactly (SHA-256 pairs recorded).
- License: GPL-3.0-or-later; `SOURCE_READ` + `FULL_TEXT_READ` licence.

### U03 — pause manager and concrete consumers

- Repository: `SagerNet/sing`, v0.8.13,
  commit resolved from tag as `7c349dacf402256d3a7029746073b05d2ead584a`.
- `service/pause/default.go`: `WithDefaultManager`, `DevicePause/Wake`,
  `WaitActive`, callback list/emit.
- `service/pause/timer.go`: `RegisterTicker` stops on pause and resets on wake.
- `common/ntp/service.go`: concrete NTP `RegisterTicker` subscriber.
- sing-box consumers read:
  - `protocol/group/urltest.go` — ticker subscriber;
  - `transport/wireguard/endpoint.go` — Down/Up callback;
  - `transport/wireguard/client_bind.go` — WaitActive after errors;
  - `route/rule/rule_set_remote.go` — stores manager but searched file has no
    `s.pauseManager` use (negative fact, not subscriber).
- Licence: GPL-3.0-or-later.
- Status: `SOURCE_READ`; two public pause files captured and hashed.

### U04 — HY2/quic boundary and recovery edge

- sing-box v1.13.19 `protocol/hysteria2/outbound.go`:
  `NewOutbound`, `InterfaceUpdated`, `Close`.
- sing-quic v0.6.4-…d83826c306d7 `hysteria2/client.go`:
  `offer`, `completeOffer`, `offerNew`, `CloseWithError`, `active`.
- quic-go v0.59.0-sing-box-mod.4 / commit
  `ccd46abedd39c6f8384b1df8daf42008724958f5`:
  `connection.go` run loop, `nextKeepAliveTime`, `maybeResetTimer`;
  `send_queue.go` queue/write/close.
- Negative search scope:
  - roots: sing-box `protocol/hysteria2`, whole sing-quic pinned module, whole
    quic-go pinned module;
  - non-test `*.go` tokens: `PauseManager`, `pause.Manager`, `DevicePause`,
    `DeviceWake`, `WaitActive`, `RegisterTicker`, `RegisterCallback`;
  - result: no match.
- This proves absence of an explicit PauseManager edge in named/current roots,
  not impossibility of all future pause mechanisms.
- Licences: sing-box/sing-quic GPL-3.0-or-later; quic-go MIT.

## 3. Official Android specifications/source

Public docs were refreshed 2026-10-28 (HTTPS/TLS validation retained):

1. Google, **Optimize for Doze and App Standby**, sections Doze restrictions,
   App Standby foreground-service criterion, partial power exemption.
   `SECTIONS_READ`:
   https://developer.android.com/training/monitoring-device-state/doze-standby?hl=en
2. Google, **Foreground service types**, `systemExempted` VPN eligibility.
   `SECTIONS_READ`:
   https://developer.android.com/develop/background-work/services/fgs/service-types?hl=en
3. Google, **PowerManager API**, `isLowPowerStandbyEnabled`,
   `isExemptFromLowPowerStandby`, `isIgnoringBatteryOptimizations`.
   `SECTIONS_READ`:
   https://developer.android.com/reference/android/os/PowerManager?hl=en
4. Google, **SystemClock API**, `uptimeMillis/System.nanoTime` vs
   `elapsedRealtime` deep-sleep semantics. `SECTIONS_READ`:
   https://developer.android.com/reference/android/os/SystemClock?hl=en

Pinned AOSP Android16 reference, frameworks/base tag-resolved commit inherited
from checked baseline and freshly captured selected files:
`99b01a65cc4c104933788b3143285ab6bae65827`.

- `core/java/android/os/SystemClock.java` — decoded/read relevant clocks;
- `services/core/java/com/android/server/net/NetworkPolicyManagerService.java` —
  selected foreground/Doze/AppStandby/LPS blocked/allowed reasons.

Status: `SOURCE_READ`; selected files carry Apache-2.0 headers in baseline cut.
No device/OEM conformance claimed.

## 4. Strongest source/model boundaries

- SFA receiver delivery is not bounded before CPU suspend.
- `DevicePause` invokes registered callbacks synchronously under manager mutex;
  a slow/reentrant callback is a potential cost/risk, not measured here.
- HY2 local `active()` checks local context/connDone, not peer/NAT liveness.
- Existing interface update closes HY2 carrier and the next operation lazily
  obtains one shared pending offer; it is a recovery seam, not energy proof.
- Kotlin authority uses boot-local `elapsedRealtime`; native transport reference
  monotime may exclude suspend. Source does not establish strict ordering between
  watchdog retirement and an already-running Go/socket write after resume.
- Current Android AAR identity/runtime remain unknown; all transport conclusions
  are source/model scope.

## 5. Public captures / failures / execution

Artifacts include:

- `receipt.txt`, `selected-inputs.json`, `input-drift.json`, `artifact-sha256.txt`;
- initial selected-input snapshot is preserved; final review found only
  `android/native/source-lock.json` sourceRevision2→3, while core/SFA pins,
  requiredApi, unknown AAR/Go and unverified status stayed unchanged;
- SFA caller/LICENSE;
- sing pause manager/timer;
- sing-box Box and command server;
- quic-go connection and licence;
- refreshed Android docs and pinned AOSP captures.

A GitHub API request for SFA commit metadata timed out once during exploratory
lookup; the immutable commit path itself fetched successfully and was used.
No repeated bypass/rate-limit workaround; commit identity came from TASK/baseline
and raw path. Status: `FETCH_FAILED` for that one metadata call, no claim depends
on it.

`NOT_RUN`: tests, fixtures, build, compiler, SDK/Gradle/Go execution, device/adb,
forced power modes, battery/radio/NAT measurements, VPN/network endpoints,
benchmarks/simulation, product change, permissions/exemption request.
