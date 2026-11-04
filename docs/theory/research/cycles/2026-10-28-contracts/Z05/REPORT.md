# R28-Z05 — exact pause caller, HY2 timer и допустимый decision seam

28 октября 2026. Авторский статус: **READY_FOR_REVIEW**; никакого product/native/live PASS. Все tests/build/device/adb/power/radio/NAT измерения — `NOT_RUN`. Exact pins/read-depth/licences — [SOURCES](SOURCES.md).

## 1. Вопрос и ограниченный ответ

**Вопрос:** достигает ли конкретный готовый Android pause/power callback HY2
timer, и какое решение допустимо без реального wake/radio/NAT outcome?

**Ответ R28-Z05-A (source-scoped negative):** в проверенном exact closure
готовый SFA callback

```text
ACTION_DEVICE_IDLE_MODE_CHANGED
→ PowerManager.isDeviceIdleMode
→ CommandServer.pause / wake
→ one Box-scoped pause.defaultManager
→ registered PauseManager subscribers only
```

**не достигает** pinned HY2/quic-go connection timer. Конкретные доказанные
subscribers — NTP ticker, URL-test ticker, WireGuard endpoint/callers; у HY2
outbound, sing-quic HY2 client и quic-go timer нет `pause.Manager`, callback или
`WaitActive` в названной source-области. `CommandServer.Pause()` сам не
останавливает Box, connections, timers или UDP socket: он лишь вызывает
`DevicePause()` общего manager; эффект существует только у подписчиков.

Это не «в HY2 pause невозможен вообще» и не whole-ecosystem proof. Это bounded
вывод для текущих pins и поиска по non-test Go callers:

- sing-box v1.13.19 / `b5ebaa1f…`;
- sing v0.8.13 / `7c349dac…`;
- sing-quic v0.6.4-…`d83826c…`;
- quic-go v0.59.0-sing-box-mod.4 / `ccd46abe…`;
- SFA callback commit `bc73504f…`.

## 2. Exact caller и shared-manager construction

### 2.1 Android caller

SFA `BoxService.kt`:

1. при start регистрирует receiver `ACTION_DEVICE_IDLE_MODE_CHANGED`;
2. `serviceUpdateIdleMode()` читает `isDeviceIdleMode`;
3. unlimited coroutine channel serializes booleans;
4. consumer вызывает `commandServer.pause()` / `wake()`.

Получение broadcast, enqueue, dequeue и JNI-call — четыре события. Source не даёт
upper bound delivery latency; deep suspend не гарантирует немедленное выполнение
consumer coroutine.

### 2.2 Manager identity

sing-box `Box.New` выполняет `pause.WithDefaultManager(ctx)` до создания network,
router/outbounds. `daemon.Instance` позже берет manager из того же context, и
libbox `CommandServer.Pause` вызывает его `DevicePause`. `defaultManager`:

- меняет `devicePaused` и заменяет/закрывает channel;
- вызывает зарегистрированные callbacks под своим mutex;
- `WaitActive` ждёт закрытия device/network channels;
- не перечисляет и не останавливает transports самостоятельно.

### 2.3 Реальные consumers

| Consumer | Pause effect | Это HY2? |
|---|---|---|
| sing NTP service | `RegisterTicker`: stop on pause; optional immediate update + reset on wake | Нет; NTP clock service |
| sing-box URL-test | `RegisterTicker`: stop/reset periodic selection probe | Нет; optional health work |
| WireGuard endpoint | callback `device.Down/Up` | Нет; другой transport |
| WireGuard client bind | `WaitActive` after connect error | Нет; retry gate |
| RemoteRuleSet | содержит поле manager, но searched file не использует его в update loop | Нет; поле не является subscriber |
| HY2/quic-go | no manager extraction/register/wait in searched exact roots | **Нет доказанного edge** |

Сильный guard против ложного вывода: quic-go создаёт отдельный `time.NewTimer`,
reset'ит его по keepalive/PTO/idle deadlines и run loop ждёт `timer.C`. Ни один
из этих callers не проверяет PauseManager. Поэтому Android receiver→manager
reachability не транзитивна до HY2 timer без явного subscriber edge.

## 3. Как HY2 действительно реагирует на path change

HY2 уже имеет готовый, но иной seam:

```text
NetworkManager interface update
→ Outbound.InterfaceUpdated()
→ Client.CloseWithError("network changed")
→ close QUIC + raw socket
→ следующий Dial/ListenPacket lazily calls offer()
→ one pending offer shared by concurrent callers
```

Это **close/recreate on proven interface/context change**, а не Doze pause.
Переиспользование допустимо, когда platform owner установил новое/утраченное
physical context и принял retirement/replacement. Вызывать его на каждом wake
без изменения path нельзя обосновать: дополнительный QUIC/TLS/auth handshake,
CPU/radio work и task latency могут быть дороже сохранения carrier.

Current Snowden source строже upstream SFA: selected physical context включает
network handle, transports, suspended/blocked flags, interface/MTU/address/DNS/
routes; callback mismatch или loss делает session fault и supervisor retires
worker. Но source не доказывает callback delivery до первого native post-resume
packet; callback observation — не atomic OS firewall/transport barrier.

## 4. State / clock / action matrix

| Состояние | Network / CPU fact | HY2 timer source | Допустимое source-only решение | Unknown без device evidence |
|---|---|---|---|---|
| Awake, same context | FGS/source eligible; CPU executes | native monotime ages; timer/PTO can fire | Keep existing carrier; no extra pause/reconnect | Actual energy/radio/NAT |
| Ordinary Doze, actual foreground UID, CPU awake in maintenance/other work | Android16 source can exempt foreground UID from ordinary Doze/AppStandby network block | If CPU runs, timer can progress; if deep suspend, callback cannot execute | Observe blocked/context facts; no blanket close merely from Doze flag | OEM proc state, residency, callback order |
| FGS active but CPU suspended | FGS is not wake lock | explicit Go/Linux reference monotime excludes suspend; timer callback not run | Do nothing solely from elapsed wall/authority gap; revalidate on return | Whether carrier/NAT/peer survived |
| UID network-exempt but no timer execution | OS may allow packets, but sleeping CPU emits none | no due/queue/write | Do not infer carrier refresh from exemption | Wake source and timer firing |
| Low Power Standby / blocked callback | Separate LPS policy may block FGS network; current path captures `blocked` when delivered | queued/native work can race delivery | Fail closed / retire on owned, current blocked/context fault | Atomicity vs first post-block write |
| Resume, same path and authority valid | peer may have discarded state while local clock did not age | old carrier `active()` is only local state | First useful operation may reuse; on failure, bounded lazy offer/reconnect. No automatic success publication | Peer/NAT survival and reconnect cost |
| Resume, changed/lost physical context | path fingerprint invalid / callback fault | old carrier no longer admitted | Reuse existing interface-update close/recreate/worker replacement owner | Callback latency; kernel in-flight packet |
| Resume after authority deadline | `elapsedRealtime` includes sleep; transport reference clock may not | native timer may not consider itself expired | Authority check wins: retire process/session; never use keepalive to extend authority | Ordering of watchdog vs already-running native write |
| Reboot/new process | `elapsedRealtime` boot-local resets; current source says deadline must not restore from cache | old native process/carrier gone | Fresh trust/profile admission only; no resumed old authority | Accepted cold-recovery policy |

## 5. Opposing cases and strongest limits

### O1 — FGS-active-but-suspended

An FGS can preserve foreground process classification while CPU deep-sleeps.
Thus «foreground UID network allowed» does not imply timer execution or packet.
Conversely `Pause` callback may not run before suspend; its existence cannot be
credited as energy saving.

### O2 — manager has callback list, therefore all services pause

False: manager is opt-in. Exact closure proves consumer edges for URL-test/NTP/WG,
not HY2. RemoteRuleSet even stores manager without using it, showing why field/
context availability alone is insufficient.

### O3 — close on every resume is safest

It is safe only in the narrow sense of discarding possibly stale transport; it
can worsen availability/energy due to needless handshakes and cannot prove NAT
state was dead. Safety authority is already separate: invalid authority/context
retires regardless of keepalive. Without outcome, universal close-on-resume is
not justified.

### O4 — leave carrier and trust `active()`

HY2 `active()` checks local QUIC context/connDone only; peer/NAT may have expired.
Therefore it is not readiness proof. Reuse is conditional; the first operation
needs typed failure/recovery accounting, not fake Ready.

### O5 — authority gates eliminate post-expiry packets

They gate Java/JNI lifecycle/platform callbacks and the 1s watchdog retires by
`elapsedRealtime`, including sleep. But already-running Go quic timers/socket
writes do not call `NativeCallbackGate` per packet. Source does not prove strict
zero-packet ordering after deadline/resume. The correct claim is: authority is
never renewed by keepalive and future admission closes/retirement is armed; a
strict packet barrier requires separate platform enforcement/measurement.

## 6. Minimal decision seam without outcome data

Recommend **observe → classify → conservative action**, not «pause saves power»:

```text
PowerPathObservation {
  boot/process generation,
  observed elapsed time,
  deviceIdle?, lowPowerStandby?,
  network blocked?, physical context generation/fingerprint,
  authority valid?, native session owner,
  outcome = unknown | carrier-failed | useful-completion
}

Decision (platform-owned):
1. authority invalid or owned context blocked/lost/changed → retire/fail closed;
2. same context + authority valid + no actual carrier failure → keep carrier;
3. first useful operation reports typed carrier failure → existing bounded
   close/recreate/lazy offer path, with one owner and recovery accounting;
4. optional health/NTP/URL-test tickers may pause through existing PauseManager;
5. energy optimization remains abstain/unknown until wake/wire/peer/recovery cost.
```

This is the smallest seam because it adds no transport fork, auto-exemption,
permanent wake lock or new periodic probe. It preserves current strict owner and
uses an existing transport recovery edge only after a reason. Exact atomic
barrier between OS block/authority expiry and a native in-flight write remains
outside this seam and belongs to platform protection acceptance.

## 7. Reuse, licence and cost

| Candidate | Exact pin / licence | Verdict | Cost / limitation |
|---|---|---|---|
| SFA idle receiver + serialized pause/wake caller | `bc73504f…`; GPL-3.0-or-later plus clause against derivative app name/association | **REFERENCE/ADAPT caller shape only** | Does not pause HY2; copying whole service conflicts with Snowden ownership/safety; no code copied |
| sing pause.defaultManager / RegisterTicker | sing v0.8.13 `7c349dac…`; GPL-3.0-or-later | **REUSE inside matching core for optional ticker consumers** | Synchronous callback under manager mutex; no timer-wide guarantee; adding HY2 subscriber is core change requiring lifecycle/concurrency review |
| HY2 `InterfaceUpdated→CloseWithError→offer` | sing-box1.13.19/sing-quic d838; GPL-3.0-or-later | **REUSE conditionally after path invalidation or typed failure** | Reconnect CPU/radio/latency; not an energy optimisation by source alone |
| quic-go timer machinery | ccd46…; MIT | **REUSE unchanged** | Suspend/wake/NAT outcome unknown; no current platform pause hook |
| Current physical context + blocked callback + authority gates | project GPL source, source-only/unverified AAR | **ADAPT as decision inputs** | Callback/order not atomic with packet write; no product proof |
| Power allowlist / permanent wake lock / blanket close-on-wake | Android platform mechanisms/policy | **DEFER/REJECT default** | User/Play/battery cost or needless reconnect; none gives peer/NAT receipt |

A direct HY2 PauseManager subscription is not recommended yet. Its semantics
would need an explicit choice: stop timer only, close connection, suppress new
offers, or defer send; each has different PTO/idle/queued-data ownership and
resume races. A generic `DevicePause` boolean is insufficient contract.

## 8. Future distinguishing observations — all NOT_RUN

1. Exact accepted APK/AAR/Go/ABI/build-tags receipt.
2. Timestamp `broadcast received → channel enqueue → JNI pause → subscriber`;
   prove whether it precedes actual suspend. No raw user traffic.
3. QUIC stages due/queued/socket-return plus OS blocked callback and named clocks.
4. Same-context resume: peer response, first useful completion, reconnect only on
   typed failure; compare with unconditional reconnect.
5. Changed-context resume: one retirement/offer owner, no old-path writes after
   accepted platform barrier, no direct fallback.
6. Authority expiry across suspend and reboot: watchdog/gate/actual process exit,
   plus kernel/native in-flight boundary; keepalive never extends grant.
7. Idle radio/NAT/peer outcome on Wi-Fi and selected cellular contexts, including
   reconnect bytes/CPU/latency and censored/missing outcomes.
8. Optional consumers (URL-test/NTP) pause before/after workload; no attribution
   to HY2 unless exact subscriber added and measured.

No percentages, mAh savings, NAT lifetime or wakeup guarantee are assigned.

## 9. Paired C04 input

C04 HANDOFF was available once at review stage and read narrowly. It corroborates
that generic pause/wake and Android unknown AAR/Go are not energy/time proof, and
adds resource/cancellation constraints. It does not supply a HY2 PauseManager
edge and does not change R28-Z05-A. Agreement between packages is not independent
runtime evidence.

## 10. Scoped verdict proposed to lead

**ACCEPT_SOURCE_SCOPED_NEGATIVE for R28-Z05-A:** the concrete SFA
Doze→CommandServer→shared PauseManager callback does not reach HY2/quic-go timer
in the reviewed exact closure; it reaches only opt-in consumers. This supports
rejecting any claim that the ready callback already suppresses HY2 keepalive.

**ACCEPT_MODEL_SCOPED for R28-Z05-B:** without wake/wire/peer/NAT/useful outcome,
use only an observation/classification seam; retire on authority/context failure,
conditionally reuse carrier when context/authority remain valid, and reconnect
through existing owner only after path invalidation or typed carrier failure.
Do not infer energy benefit, request exemption, hold a broad wake lock or close
on every wake from source alone.

Not accepted by author: product correctness, zero post-expiry packet guarantee,
battery gain, NAT survival, callback timeliness, device/OEM behavior or readiness.
