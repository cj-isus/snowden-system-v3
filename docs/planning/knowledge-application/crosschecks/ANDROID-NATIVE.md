# Android native: от source-кандидата к одному рабочему клиенту

Анализ Android-каркаса на 28 октября. Позднейшие exact AAR/APK/UI/JNI
результаты имеют собственные границы; см. [актуальный план Android](../ANDROID-PLAN.md).
Старое отсутствие артефакта не следует переносить на новый source-срез.

## Краткое состояние кода

Android-дерево уже содержит правильный каркас владения, но **не содержит рабочего
нативного клиента**:

- `:app` владеет одним `VpnSessionRepository`; UI/tile/widget не создают ядро.
- `SnowdenVpnService` работает в private `:vpn`, продвигается в FGS до JNI и
  передаёт lifecycle одному `WorkerProcess`.
- `WorkerProcess` допускает один start attempt на worker, удерживает unknown owner,
  разделяет control/retirement RPC lanes и требует фактическую смерть процесса перед
  заменой.
- `NativeWorkerSession` удерживает TUN/physical/app/authority owners, проверяет JNI
  steps и требует три capability revision1 до `CommandServer`:
  strict-owner, session-cache disable и closed-history disable.
- `PhysicalNetworkBinding`, `ProtectedSocketControl`, `AndroidTunFactory` и
  application-owner guard уже задают deny/retire semantics без `allowBypass`.
- `corebridge` уже имеет правильный artifact gate: независимый SHA-256 адресует одну
  verified-копию whole AAR; из неё извлекается `classes.jar` для compile, а приложение
  пакует **тот же whole AAR** через producer-owned `approvedLibbox`.

Критический разрыв находится до runtime:

1. `android/native/strict-owner` — sourceRevision3, `wiring.patch` + 6 overlays,
   но patch не применён, не собран и не проверен;
2. `android/native/source-lock.json`: `aarSha256=null`, Android Go compiler `null`;
3. stock libbox AAR несовместим с Kotlin consumers;
4. generated Java/JNI inventory отсутствует;
5. `ProfileTrustDriver` не имеет production implementation, а T01…T14 имеют статус
   **requested, SDK/API не опубликованы и не приняты**;
6. `SnowdenApplication` намеренно не создаёт fake driver/provider;
7. `NativeConfigurationCandidate.signedAuthorityVerified=false`; структурный graph
   composer не является authority;
8. APK/native/device/protected-path gates не выполнялись.

Следовательно, очередная source-фича или power callback не является кратчайшим путём.
Кратчайший путь — зафиксировать один complete source closure, получить один whole AAR
с проверяемым JNI/API contract и только затем соединить принятый профильный источник
с уже существующим lifecycle owner.

## Применение накопленных знаний

| Знание / источник | Существующий code + plan | Разница | Решение |
|---|---|---|---|
| Один владелец VpnService/native/TUN; coroutine timeout/Close не death (`PLATFORM-AND-UX` §12.2, C04 lifetime units) | `VpnSessionRepository` → `WorkerProcess` → `NativeWorkerSession`; retained Binder/RPC/source/death fences. AND-02/03, A-FR-01/02 | Runtime/JNI/process-death behaviour не выполнено; неизвестный owner может сделать систему недоступной, но не разрешает второй core | **Reuse, не переписывать.** Один Android platform owner реализует последовательность; не дробить lifecycle и native core между независимыми исполнителями |
| Whole libbox1.13.19 + узкий bridge, не SFA HEAD (`PLATFORM-AND-UX` §12.7/§15.5) | `source-lock.json`, strict-owner revision3, `NativeWorkerSession` version/capability checks. AND-01 | Current patch/API exists only as source; stock AAR lacks markers/wiring | **Adapt current closure only.** Применить exact patch/overlays к b5ebaa1…, не обновлять core и не копировать SFA service wholesale |
| Hash/marker не доказывает artifact (`R04-011`, Z05, Z08 late source identity) | `verifyLibboxArtifact`, approved-SHA-qualified output, same AAR/JAR source. AND-01-S3/S4, ADR-023/061 | Нет approved digest, generated API inventory, ABI/16KiB/toolchain/SBOM receipt | **Complete existing gate**, не создавать второй pipeline |
| Signed profile authority отдельно от structural graph (`PLATFORM-AND-UX` §12.5, trust units) | `ProfileTrustDriver` mandatory; `ProfileImportCoordinator`; `NativeConfigurationComposer`; `RuntimeSourceProvider`. AND-04/06, A-FR-03/05 | Production driver и accepted-only provider отсутствуют; T01…14 только requested | **Dependency blocker.** Trust owner сначала публикует accepted API/source revision. Android интегрирует его в **тот же whole AAR**, затем реализует driver/provider; до этого Connect остаётся disabled |
| Authority clock включает suspend; transport clock/peer state — другой контракт (`R04-011`, Z05/C04, QUIC accounting) | `ProfileClock`, `authorityValidUntilElapsedMs`, `NativeCallbackGate`, worker watchdog. AND-02/04/11 | Java/JNI gates не дают per-packet atomic cutoff; accepted Go/AAR clock неизвестен | **Reuse current expiry/retirement contract.** Не вводить transport-derived renewal. Device acceptance отдельно проверяет in-flight boundary и resume/reboot |
| SFA Doze→PauseManager не достигает HY2 timer (R28-Z05) | Current Snowden вообще не экспортирует Pause/Wake; physical blocked/context fault ведёт к retirement. AND-05/11 | Нет wake/radio/NAT outcome и нет HY2 subscriber edge | **Reject direct pause feature.** Optional NTP/URL-test pause допустим только внутри exact core consumers; HY2 оставлять как есть. Path invalidation использует existing retire/recreate, same-path resume — conditional reuse |
| HY2 due/queue/write/peer, peer timeout и retention — разные receipts (`R04-011`, `QUIC-ACCOUNTING`, C04) | Pinned sing-quic/quic-go source, existing queue caps; Android private worker owner. AND-02/07/11 | Нет complete internal admission/resource ledger; session cancel может не прервать waiting `Add`; blocked write/reap bounds неизвестны | **Defer transport changes до принятого artifact.** Сначала preserve worker owner and retirement reserve; затем один scoped upstream/adapter task, без timeout-goroutine с ранним Release |
| Cache и closed-history retention не нужны Android app (`ADR-057/058`) | Native-only `SnowdenDisablePersistentCache` и `SnowdenDisableClosedConnectionHistory`; Kotlin всегда требует/задаёт оба. AND-11 | Marker/wiring не artifact; active trackers/events/other native IO остаются | **Reuse current revision3.** Приёмка проверяет propagation, wrong scope, no cache.db, active-event parity; не обещает RSS/secure wipe |
| Observer admission и startup error должны пережить cleanup (`ADR-059/060`) | sourceRevision3 закрывает Observer owners; `Box.Start` сохраняет start error при cleanup panic | Patch/JNI/runtime paths не проверены; Close return не worker death | **Include in same AAR receipt.** Нельзя одобрять older revision2 AAR только потому, что JNI markers прежние |
| APK install/signing/adoption различаются (`R05-004`) | AND-14 planned; current app versionCode1/dev, release minify false | Whole APK/splits/signer/lineage/update/adoption отсутствуют | **После native+trust closure.** PackageInstaller/signing/readiness — отдельный release gate, не часть source SHA approval |
| Native backup/Keystore не authority (`R05-008`) | CE/no-backup encrypted store, `ProfileClock`, trust floors. AND-04/13 | Restore/re-enrollment/server floors и production trust driver отсутствуют | **Keep recovery out of first native activation.** No directBoot/always-on until accepted offline recovery; no trust reset on reinstall |
| Battery/speed benefit нельзя вывести из keepalive/pause/source (`R04-011/Z05/C04`) | AND-11/15/16 gates; no wake lock/exemption; source only | Нет actual AAR, device/OEM/RAT/NAT/useful-task measurements | **No preset/claim.** Instrument only after protected client works; thresholds preregistered, missing outcomes retained |

## Минимальная последовательность одного согласованного native artifact

### Stage 0 — заморозить входной contract, не собирать два ядра

**Owner:** Android platform owner + trust owner как поставщик принятого portable API.
**Plan:** AND-01/04, A-FR-03/13.

Зафиксировать один source manifest:

- sing-box v1.13.19 commit `b5ebaa1…`;
- strict-owner flavor
  `owner-r1-session-cache-r1-closed-history-r1`, sourceRevision3;
- все base hashes, `wiring.patch`, 6 overlays;
- sing dependency v0.8.13 и selected transport pins;
- **принятую** trust API/version/source revision T01…T14;
- Go/gomobile/NDK/AGP/Kotlin/JDK, build tags, ABI set;
- corresponding source, notices, SBOM input graph.

Stop condition: пока T01…14 не опубликованы/приняты, допустима подготовка native
build recipe, но нельзя фиксировать final artifact digest или включать Connect.
Trust module добавляется в существующий gomobile bind/package list и один Go runtime;
отдельный trust AAR/второй Go runtime запрещён.

### Stage 1 — применить revision3 и получить source-parity receipt

**Owner:** один Android native/platform owner.
**Plan:** AND-01, AND-11; ADR-057…060.

В чистом checkout exact base:

1. сверить каждый base SHA до patch;
2. применить **весь** `wiring.patch` и все overlays;
3. проверить destination inventory, отсутствие лишних/пропущенных generated inputs;
4. зафиксировать post-apply source hashes и sourceRevision3;
5. убедиться, что revision3 включает cache/history/observer/start-error изменения как
   один closure, а не набор marker-only файлов;
6. включить принятый trust package и narrow JNI exports в тот же manifest.

**Future acceptance, NOT_RUN:** patch applicability, formatting, compile; diff против
manifest; deliberate missing/modified hunk rejects; older revision2/stock source rejects.

### Stage 2 — собрать один whole AAR и доказать generated API parity

**Owner:** тот же Android native owner; security/release reviewer независимо выдаёт
approved SHA.
**Plan:** AND-01, AND-14; ADR-023/061.

Build receipt обязан включать:

- exact Go/gomobile fork/NDK/JDK/flags/tags/module graph;
- whole AAR SHA-256 и corresponding source manifest SHA;
- generated Java classes/JNI symbol inventory;
- exactly expected `Libbox.version()==1.13.19`;
- three revision1 marker methods and three OverrideOptions fields;
- accepted trust API revision/methods/DTO ownership semantics;
- ABI inventory (`arm64-v8a`, optional x86_64 lab), ELF/ZIP alignment and 16KiB;
- no second Go runtime/native library with conflicting symbols;
- licence/SBOM/notices/source-offer outputs.

Existing Gradle gate remains authoritative: externally approved SHA is an input;
`corebridge` compiles against extracted `classes.jar` from the approved stage; `:app`
packs the same whole AAR. Hash approval does not derive from the candidate itself.

**Strongest guard:** marker/API parity is necessary but not sufficient. A malicious or
incomplete AAR could return correct revision numbers. Receipt must bind built source
closure + generated inventory + native libraries + independently approved whole bytes.

### Stage 3 — connect accepted profile to runtime without bypass

**Owner:** Android profile/integration owner, after accepted trust dependency.
**Plan:** AND-04/06, A-FR-03/05.

Implement exactly two Android adapters, not a new architecture:

1. `ProfileTrustDriver` JNI adapter to accepted T01…T14 API:
   - synchronous owned bytes;
   - full TUF + application authority, progress/floors/revocation/clock;
   - typed redacted outcomes;
   - no ambient FS/network/global state;
   - accepted deadline remains separate from preparation TTL.
2. `RuntimeSourceProvider` accepted-only renderer:
   - reads only accepted checkpoint/current policy;
   - compiles `NativePolicyPlan` and `NativeConfigurationComposer` candidate;
   - rejects every `admissionBlocker` and `signedAuthorityVerified=false` shortcut;
   - binds exact config/policy digest, native required revisions, TUN tag,
     network/app owner+epoch+fingerprint, worker/request and original authority deadline;
   - returns one parent-local `IPreparedRuntimeSource` consumed once.

No direct construction from preview/JSON/UI/Intent; no default/mock success.
`SnowdenApplication` may expose import/connect only after production driver/provider
is installed and source identity matches the approved AAR.

### Stage 4 — retain existing lifecycle/error/privacy/resource controls

**Owner:** Android service/native owner; do not split into a second controller team.
**Plan:** AND-02/03/05/06/11; A-FR-01/02/04/05/10.

Keep these frozen contracts:

- one app repository, one private worker, one native attempt/process;
- FGS promotion before profile/JNI; typed foreground failure;
- one original TUN owner, borrowed socket dup never closes caller fd;
- physical/app/authority gates around every JNI/OS step; terminal faults close gate
  before delayed cleanup;
- no command listener/hot reload; Start/TUN never publishes Ready;
- Stop/Close/coroutine timeout not death; successor waits for Binder death plus both
  RPC returns, source body+Stub idle and parent monitor cleanup;
- uncertain FD/native/observer owners retained to process exit;
- raw core/config/domain/package/error text never crosses state/log/support seams;
- persistent cache and closed-history override required for every worker;
- no direct/allowBypass/plaintext fallback.

Resource additions should be minimal: first validate revision3 retention controls and
existing fixed lanes. The C04 shared-carrier/session cancellation and resource ledger is
a later scoped transport-owner task. Do not couple first AAR activation to a quic-go
upgrade, CCA change or new pause subscriber.

### Stage 5 — platform integration gates before “working VPN”

**Owner:** Android platform owner.
**Plan:** AND-02/03/05/06/07/11.

Ordered future gates, all `NOT_RUN`:

1. generated Kotlin/Java/JNI calls return expected version/revisions and typed errors;
2. FGS consent/eligibility/revoke/destroy on API29/API34+/target36;
3. TUN establish/protect/bind with original PFD retention and failure paths;
4. strict-owner path over TCP/UDP/PreMatch/DNS/fakeIP/ICMP/fragments; unknown UID denies;
5. exact physical context before TUN/socket, blocked/lost/change causes controlled
   retirement, no old-path direct fallback;
6. profile expiry/revocation during blocking JNI, delayed callbacks and late PFD;
7. startup normal/error/cleanup panic/hang retains nonnil typed cause, no raw diagnostics;
8. cache/history flags: wrong scope/options reject, no cache DB, active tracker/event parity;
9. observer late Subscribe/Close/backpressure and actual worker death;
10. parent/child shutdown with blocked JNI/Binder/raw write and uncertain owner;
11. accepted protected observation before `ready`; STARTED/handshake/key icon are not Ready.

### Stage 6 — artifact/package/device/release evidence

**Owner:** Android release/QA; dependencies AND-01…14.
**Plan:** AND-14/15/16/17, A-FR-13…16.

- Produce a signed complete APK/split set from the approved AAR, with signer lineage,
  versionCode, SBOM/notices/corresponding source and channel identity.
- Verify installation outcome separately from executing/adopted VPN readiness.
- Device matrix: API29, API34+, API36, selected OEM/patch, 4/16KiB, arm64 and x86_64 lab.
- Fault matrix: consent denial, notification denial, always-on/lockdown, another VPN,
  process/UI/native death, reboot/force-stop/pre-unlock, network/Doze/LPS/OEM policy.
- Protected-path and privacy checks precede performance claims.
- Field/energy/Russia comparison only after AND-15 and explicit live authorization.
- RC→canary→release only after all P0 and rollback/support gates.

## Frozen cross-component contracts

### Artifact identity

```text
NativeArtifactIdentity {
  baseCommit,
  sourceRevision = 3,
  overlay/wiring manifest hash,
  trustApiRevision,
  toolchain + tags + ABI,
  generatedApiInventoryHash,
  wholeAarSha256,
  correspondingSource/SBOM hash
}
```

A marker return value or whole-AAR hash alone cannot satisfy this contract.

### Accepted runtime source

```text
AcceptedRuntimeSource {
  acceptedCheckpoint identity + authority fence,
  config/policy digests,
  native required revisions,
  network owner/epoch/context,
  app owner/epoch/pins,
  worker/request identity,
  preparation deadline,
  independent authority deadline
}
```

It is parent-local, one-shot, redacted, retained through producer return and Binder
reply marshalling. Reconnect/new worker cannot renew authority.

### Native ownership

```text
NativeOwnerReceipt {
  process/session incarnation,
  original TUN/PFD owner,
  gate phase + in-flight JNI/callback counts,
  control and retirement RPC completion,
  prepared source sealed+idle,
  parent monitor cleanup,
  actual Binder/process death
}
```

Graceful `Close` and local zero callbacks are intermediate facts, not release.

### Path/power/transport observation

```text
PathObservation {
  boot/process/network generation,
  physical fingerprint + blocked/suspended facts,
  authority validity,
  native session owner,
  outcome = unknown | carrier_failed | protected_useful_completion
}
```

SFA PauseManager does not reach HY2 timer in the reviewed pin. Do not add a generic
pause edge to the first artifact. Same-context resume conditionally reuses carrier;
context invalidation or typed carrier failure uses existing retire/recreate owner.
Battery/radio/NAT gain remains unknown until AND-15/16 evidence.

## Strongest opposing guards and dependencies

- **“Source patch already implements it.”** No: sourceRevision3 and markers have
  `artifactProof=false`; no AAR/Go/compiler/JNI inventory exists.
- **“Independent SHA is enough.”** No: it authenticates approved bytes but not source
  parity, generated API, ABI/16KiB or correct trust implementation.
- **“Stock AAR plus Kotlin checks.”** Rejected: stock release lacks required APIs/wiring.
- **“Build native now, add trust later.”** A technical intermediate build may validate
  patch/JNI, but must not become final approved runtime artifact or enable Connect.
  Final AAR source closure must include accepted trust module/API.
- **“Second trust AAR is cleaner.”** Rejected: creates second Go runtime/dependency/
  ABI/lifecycle boundary contrary to T14 and current one-owner architecture.
- **“SFA pause/wake saves HY2 battery.”** Rejected by exact caller trace; no HY2
  subscriber and no device outcome.
- **“Close carrier on every resume.”** Rejected without path/failure evidence; may add
  handshake/latency/energy. Existing context invalidation path is sufficient.
- **“Timeout then release resources.”** Rejected: blocked JNI/Binder/UDP/raw write may
  retain owners. Keep slot/process/source until actual return/death.
- **“One developer per layer in parallel.”** Avoid until contracts freeze. Native patch,
  generated JNI, Kotlin bridge, trust adapter, provider and service share one artifact
  and lifecycle boundary. Independent work is safe only for immutable docs/fixtures/
  release review after source/API freeze; integration has one principal Android owner.

## Future acceptance and remaining unknowns

All acceptance below is `NOT_RUN`:

- exact patch/build/API/ABI/toolchain/artifact reproducibility;
- production trust API acceptance and semantic parity T01…14;
- JNI ownership/error/cancellation/privacy parity;
- lifecycle/process death/FD/IPC races and hard cutoff limits;
- actual transport writers, queue/cancel/resource ceilings and retirement latency;
- API/OEM/patch/Doze/LPS/reboot/force-stop/lockdown behavior;
- protected egress and task completion;
- device energy/speed/RSS/FD/wakeup and NAT outcomes;
- signed package install/update/adoption and release rollback/support.

Unknown inputs remain gates, not inferred failures or zeros. No battery/speed gains,
runtime readiness or release date follows from the current source.

## Что не стоит делать

- Не писать второй VPN controller, второй TUN/native owner или второй Go AAR.
- Не переносить SFA HEAD/whole service или добавлять another transport during parity.
- Не включать pause/wakelock/power exemption/keepalive preset без device outcome.
- Не создавать Kotlin signature/TUF canonicalizer или принимать parsed JSON as trust.
- Не подменять accepted AAR marker/hash иконкой VPN, STARTED или handshake.
- Не добавлять broad package inventory, direct fallback, `allowBypass`, raw native logs.
- Не блокировать первый coherent artifact UI-параллелизмом, store submission или
  optional performance work; сначала source→artifact→provider→lifecycle→device gate.

## Источники и трассируемость

Основные frozen knowledge families:

- Android ownership/platform: `PLATFORM-AND-UX` §§12.2/12.3/12.5–12.7/13.8/15.5;
- power/HY2: R04-011 REPORT/SOURCES/HANDOFF;
- QUIC ownership/time: `QUIC-ACCOUNTING-AND-LIMITS` §§4–7;
- pause callback: R28-Z05 REPORT/SOURCES/HANDOFF;
- resource/cancellation: R28-C04 REPORT/HANDOFF;
- artifact/adoption/recovery: R05-004 §§4/6–8, R05-008 §§2–4;
- late strict-owner crosscheck: Z08 SOURCES N01/N02/drift sections.

Actual source/plan anchors:

- `android/native/{README.md,source-lock.json,strict-owner/}`;
- `android/{corebridge,profile,vpn,app}`;
- `android/corebridge/build.gradle.kts`, `android/app/build.gradle.kts`;
- `docs/android/CHANGE-REQUEST-TRUST-001.md`;
- `android/STATE.md`, `docs/ANDROID-PLAN.md` (read-only current status).

Unit-level dispositions are in private `coverage/Z05.json`; current source hashes are
in `captures/Z05/current-source-cut.json`. This crosscheck covers 69 selected initial
units and 3 relevant late units, not the whole 2714-unit corpus.
