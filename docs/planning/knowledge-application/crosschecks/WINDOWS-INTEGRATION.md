# Windows: применение теории защиты, восстановления и рабочей сети

## Состояние кода на source cut 2026-10-28

Windows-клиент уже имеет существенную основу, которую не следует заменять новым
controller или вторым daemon:

- `App.operationMu` сериализует lifecycle; `operationID`, session/owner generation,
  `ObservationID` и network epoch отсекают устаревшие результаты.
- `Manager` разделяет prepare и lifecycle ownership, удерживает engine process,
  возвращает typed cleanup uncertainty и допускает rollback только после свежего
  old-path probe.
- `StopReason` снимает strict WFP только при явном Disconnect. Автоматическая
  ошибка, quit и restart сохраняют барьер; неочищенный engine/helper переводит
  UI в `cleanup_unconfirmed` и исключается из auto-retry.
- TUN boundary связывает worker/config/selector, LUID/WFP и внешний probe при
  Start; periodic observation повторяет worker/config/LUID, но не внешний HTTP.
- Probe report внутри backend уже содержит session, owner generation, intent,
  observation ID, network epoch и времена. Публичный `ProbeReportView` теряет
  эти поля и оставляет только boolean `OK` и шаги.
- Renderer уже имеет DNS hijack, RFC1918/`.local` direct и TUN exclusion
  `172.16/12`; это не signed корпоративная split-DNS/LAN policy и не proof её
  эффективного socket-hop.
- Frontend polling и backendState имеют single-flight/generation guards; основной
  пробел — сила отображаемых обещаний и недостаточная типизация producer facts,
  а не отсутствие очередного store/router.

Все эти пути после V2-110 помечены в STATE как source-only/unverified; сохранённые
старые live PASS относятся к другим входам. Ниже предлагается применение знаний,
не изменение статусов PLAN и не product implementation.

## Карта знаний → текущий код/план → минимальное решение

| Семейство знания и frozen sources | Current code и plan | Разница / opposing guard | Решение |
|---|---|---|---|
| Fail-closed: intent отдельно от engine; timeout не завершает owner; cleanup uncertainty удерживает барьер. `SAFETY-AND-TRUST` K-b45…, K-ec96…; recovery K-fddf…, R06 K-c667… | `protection.go`, `Manager.Stop`, `Engine.retireIsolatedLocked`, `RetireCaptureHelpers`, `cleanup_unconfirmed`; FR-009/010, OFF-01/02/03 | Сильные guards уже есть: Stop intent, Job/reaper, WFP retention и retry suppression. Не доказаны OS process/WFP/reboot и старые TCP/UDP flows. Нельзя предлагать rewrite lifecycle или считать Stop acceptance cleanup completion | **REUSE existing lifecycle.** Добавлять только frozen receipts для proxy/settings и projections; native/OS приёмка остаётся gate |
| Proxy settings принадлежат actor/epoch; snapshot rollback может затереть внешний S2. R06 K-b87…, K-cee…, K-273…; C06 K-ad9… | `ProxyManager.Snapshot→SetProxy→Restore`; startup/manual `HealStaleSystemProxy`; NetGuard UserMode повторяет эвристику; COM-06/07, OFF-03/05 | Existing guard узок: только 127.0.0.1:1080, внешний/комбинированный proxy/PAC и живой listener не трогаются. Но адрес/мертвый listener не доказывают owner; heal и Restore имеют read/write TOCTOU, игнорируется PAC delete/readback, отсутствует receipt effective values | **CHANGE REQUIRED P0:** общий owner-aware proxy transaction до любых writes; unknown/conflict не мутирует foreign settings. UI manual heal — request/outcome, не гарантированный repair |
| Proxy P и endpoint E — разные hops/resolvers; browser SSO/metadata не transport permission; CONNECT/auth scope отдельны. PROXY-DNS K-0ade…, K-e5cd…, K-746… | Нет upstream corporate proxy adapter в admitted profile/render/guard/probe. HKCU manager — downstream proxy для apps; HTTP connector upstream существует лишь как GPL building block. COM-06/07 | Сильный guard: текущий profile admission не разрешает raw HTTP node, поэтому unsupported path не становится bypass. Добавление одного outbound не закрывает PAC, NTLM/Negotiate, DNS realm или WFP bootstrap | **DEFER transport adapter** до отдельного profile/schema/security design. Сейчас реализовать compatibility preflight/typed unsupported only; не брать browser/default credentials и не менять PAC |
| Route/DNS/private/foreign VPN нельзя выводить из отсутствия одного WFP permit; effective hop важнее названия policy. R06 K-084…, K-5f62…, K-2aa…, K-ef7… | Renderer RFC1918/.local direct, DNS hijack, route.final proxy, 172.16/12 TUN exclusion; static WFP AppID/LUID permits/catch-all. COM-04/05/06, OFF-05 | Engine AppID может повторно открыть LAN socket, physical exclusion — иной case; external WFP layers/veto неизвестны. Неподтверждён комментарий «HardAction=false делает static block soft». Нет per-suffix resolver contract/receipt | **NEEDS REVIEW then ADAPT:** сначала native WFP arbitration gate. Затем signed route/DNS policy compiler с explicit scope; no blanket private ranges/direct. Не строить UI обещание из renderer config |
| Proof должен разделять protection, task/path и recovery. Probe DNS/HTTPS/egress не business task и не вся OS. R06 K-dc53…, C06 K-dcc…, Z09 K-848… | Core report богат owner/intent/epoch/time; `toView` сворачивает в OK+steps; Dashboard пишет «Соединение защищено», «DNS через туннель», «Доступ к ресурсам открыт», «утечек нет». COM-07/11, OFF-05/08 | Existing normalization правильно не даёт PASS при unavailable/leak; UI labels расширяют scope: DoH/SOCKS step≠native DNS, две HTTPS цели≠resources, leak step≠весь device interval | **ADAPT projection P0/P1:** передать scope/freshness/evidence kind, заменить absolute copy на scoped facts/unknown; task completion отдельно и только opt-in future producer |
| Recovery retry имеет один owner/budgets; network transition должен отзывать old proof. Historical K-fddf…, R06 K-4a99…, performance K-cfe… | `autoretry` exactly-once/backoff; diagnostic lanes separate protected/optional; netwatch epochs up/change, но TUN mode polling skipped; watchdog periodic. FR-010, OFF-03/05, COM-03/09/12 | Strong guards: no retry for trust/cleanup/restart custody; protected lane reserved. Gap: physical transition without event in TUN means old epoch may persist until watchdog failure; no business-operation retry is present and should not be added | **REUSE budgets/owners; CHANGE observation contract:** introduce dated physical-path observation/coverage state, not another retry loop. Unknown/no-event TUN remains unknown; mandatory revoke outranks quiet mode |
| NetGuard task/listener facts are not protection/repair receipts. C06 K-ad9…, K-406… | `NetGuardStatus`, task COM status, local port connect, untrusted log, Vue hero; OFF-08, COM-07/12 | ready/running/no-access and TCP connect have narrower semantics. Current hero says protection active, tasks work, repair within 2 min, SYSTEM continues. Opposing guard: status parser preserves no-access/not-found/unknown and poller generations | **CHANGE REQUIRED P0 presentation:** scheduler available/running, status unavailable, local port responds; no protection/automatic repair promise. Date producer facts and split task/proxy/DNS sections |
| Resource/custody: bounded visible queue not total memory; timeout not owner completion. TRANSPORT K-2c1…, K-791…, K-f829… | Engine worker Job/process owner; diagnostic requests/bytes/minute and active≤2; helper lanes retained. COM-12, OFF-02/05 | Existing limits useful; NetGuard external scheduled task and PowerShell DoH probes are outside Go diagnostic budget. DNS script can rewrite adapter servers and DoH HKLM globally, and has no owner snapshot/restore contract | **DEFER/ISOLATE legacy NetGuard DNS heal:** inventory as separate privileged legacy mechanism, do not treat as client recovery. Any future migration needs policy owner, exact adapter generation/snapshot/conflict rollback, budgets and admin consent |
| Important session: optional switching/update can wait, safety cannot; ordinary selector switch does not promise old-flow continuity. R06 K-c667…, K-b8d…, PLAN COM-09/17 | Selector/Reload branches, validity constraints, fresh probe; no complete quiet intent contract in Windows source. COM-09 P1, COM-17 P3 | Existing code is safer than blanket claim but comments/UI can imply seamless channel change. Deferred bonding/migration should not block basic truthful scope | **ADAPT later P1:** bounded quiet intent at existing controller; copy says new connections/default only. Keep COM-17 deferred |

## Приоритетная последовательность компонентных изменений

### W0. Зафиксировать frozen producer contracts до UI и новых network features

**Owner:** Windows client core. **Plan:** FR-009/010, OFF-01/02/03/05.

Reuse current `operationID`, manager identity/generation, session/network epoch,
process owners and diagnostic reservations. Define compact internal receipts:

- `LifecycleReceipt`: intent, owner/generation, applied config/session, phase,
  cleanup disposition (`confirmed | retained-uncertain | not-attempted`), time scope.
- `ProtectionReceipt`: mode, WFP policy generation, observed LUID/config/selector,
  evidence kind (`lifecycle-external | periodic-local | unavailable`), observation age.
- `SettingsReceipt`: actor/owner token, exact before fingerprint, exact owned value,
  mutation generation, write/readback result, conflict/unknown, cleanup result.

These are frozen semantic contracts first; final DTO names are developer-owned.
No second lifecycle owner, no database and no generic event bus.

### W1. Proxy settings owner and heal precondition — first code slice

**Owner:** `windows/backend/core`, then App facade. **Plan:** OFF-03/05,
COM-06/07; protection implications FR-009/010.

1. Replace loose `ProxySnapshot{values,originalSet}` with an owner-scoped transaction:
   exact presence/value for ProxyEnable/ProxyServer/AutoConfigURL, operation/session
   generation and canonical owned target. Do not infer owner from `127.0.0.1:1080`.
2. Before SetProxy, check authority/compatibility: managed/policy/per-connection
   states that cannot be represented remain `unsupported/unknown`; no mutation.
3. Apply under one retained owner. PAC deletion errors and all value writes require
   readback. A partial Set is a cleanup obligation, not a successful configured state.
4. Restore only if current values still equal this owner's expected written state.
   External S2 yields `conflict_external_change`; preserve S2 and surface action.
   Compare/readback is not atomic with MDM, so retain uncertainty if final read/write
   cannot be established; never report “restored” on unknown.
5. Startup/manual heal accepts only a durable owner receipt from a prior Snowden
   session. Legacy heuristic remains **read-only diagnosis** or explicit user-confirmed
   compatibility path; no automatic `ProxyEnable=0` from address+dead listener.
6. Unify Go startup/manual/PowerShell UserMode around the contract or retire the
   duplicate writer. Do not leave two uncoordinated registry writers.

**Cheapest sufficient slice:** owner token + exact expected-current compare/readback
for current `ProxyManager`, disable auto-heal mutation when token missing. This
preserves foreign proxy/PAC immediately without implementing enterprise proxy auth.

### W2. Lifecycle/cleanup projection, not another controller

**Owner:** App/Manager/Engine. **Plan:** FR-009/010, OFF-01/02/03.

- Thread `LifecycleReceipt` through Start/Stop/Reload/Switch result. Keep existing
  manager/engine/helper owner objects; do not clear runtime or release WFP when any
  retained owner is uncertain.
- Distinguish `stop_requested`, `engine_retired`, `helpers_retired`,
  `settings_restored/conflict`, `protection_released/retained`, and final outcome.
- An accepted Stop or cleanup timeout is not terminal success. Automatic retry remains
  disabled for cleanup uncertainty; explicit action may retry cleanup using original
  owners. Parent late return cannot authorize successor publication.
- Preserve source-implemented safeguards: old generation cannot publish; Stop intent
  captured before waiting; probe reports bound to owner/session/epoch; WFP released
  only after Manager/helper retirement succeeds.

No need to implement Android SDK/accepted provider first; this is isolated Windows
lifecycle work over existing contracts.

### W3. Effective route/DNS and working-network compatibility preflight

**Owner:** render/config + Windows platform adapter; policy approval remains owner.
**Plan:** COM-04/05/06/07, OFF-04/05/06, FR-001/008/011.

Build a **read-only preflight** before any settings mutation:

- intended task/profile scope; route and DNS suffix ownership; actual carrier
  requirement (TCP proxy versus UDP QUIC); proxy P and endpoint E resolution realm;
  WFP/app/LUID prerequisites; known corporate/foreign owner conflict; unknown fields.
- Explain renderer candidate separately from OS-observed/applied outcome. RFC1918,
  `.local` and `172.16/12` are not one LAN grant. mDNS/LLMNR/SMB and IPv6/link-local
  remain unsupported until explicit policy/caller evidence.
- For split DNS, use a signed/versioned namespace→resolver policy with terminal
  default and network/profile scope. Do not rewrite NRPT/PAC/adapter DNS automatically.
  Unknown or managed full-tunnel returns action barrier, not another transport retry.
- Future enterprise HTTP proxy path is conditional TCP carrier support only; reject
  silent default credentials, unlimited 407 retries and claims that CONNECT success
  proves inner trust/protected egress/task success.

### W4. UI weak-to-strong ladder and NetGuard copy correction

**Owner:** Go projection + Vue existing views/components. **Plan:** OFF-08,
COM-02/07/11/12, TH-04/08.

1. Extend projection from boolean `probe.ok` to scoped evidence already present in
   backend: evidence kind, observation time/age, mode/scope, owner/session/epoch match,
   protection unavailable/leak/unknown, and cleanup/settings conflict. Raw endpoints,
   PAC URLs, credentials and organization identifiers stay out.
2. Copy ladder:
   - **Request:** “Проверка/остановка/ремонт запрошены”.
   - **Observation:** “Планировщик: ready”, “Локальный порт отвечает”,
     “WFP/LUID обнаружены”, with timestamp/scope.
   - **Applied:** only from component receipt.
   - **Protected:** only current scoped protection receipt + applicable path evidence.
   - **Task complete:** never inferred from VPN probe.
3. Replace NetGuard hero promises:
   - `ready` → “задача готова к запуску”; `running` → “экземпляр выполняется”;
   - `no-access` → “состояние задачи недоступно”, not “SYSTEM continues”;
   - stale heuristic → “совпала конфигурация 127.0.0.1:1080, listener не ответил;
     владелец не подтверждён”; disable mutation button until owner receipt or explicit
     legacy-confirmed flow;
   - no “исправится за 2 минуты” without scheduled/fresh successful receipt.
4. Dashboard scoped labels: “Проверенные цели прошли через заявленный путь”,
   “DoH-проверка через туннель”, “ожидаемый egress совпал”, “в этом probe прямое
   расхождение не обнаружено”. Remove “resources open/no leaks” absolutes.
5. Reuse existing backendState/poller generation and components; no router/store/UI
   framework. A11y/native WebView2 remains NOT_RUN.

### W5. Resource and legacy NetGuard separation

**Owner:** Windows/platform operations, not UI. **Plan:** COM-12, OFF-05/06.

- Keep `DiagnosticProtected` and optional lanes; classify budget rejection as local
  admission, not channel failure. One controller owns retries; PTO/HTTP/UI are not
  additional independent recovery loops.
- Inventory scheduled NetGuard DNS heal as a separate privileged legacy controller:
  adapter selection, DoH endpoint probes, HKLM flags and DNS replacement are not
  represented by App lifecycle/epoch/budget. Until an ownership/conflict/restore
  contract exists, do not advertise it as Snowden protection or auto-recovery.
- Prefer retiring or narrowing that script over importing it into App. If retained,
  give it explicit admin policy scope, adapter identity/generation, previous DNS
  state, conflict-safe restore, bounded network calls and separate UI status.

## Frozen contracts and dependencies

| Contract | Minimal inputs | Outputs / consumers | Dependency and gate |
|---|---|---|---|
| Settings ownership | exact value presence/bytes, actor+intent/session, owner token, expected current, policy/managed availability | applied/conflict/unknown receipt; cleanup obligation | W1 before any automatic proxy mutation; real HKCU/MDM fault acceptance NOT_RUN |
| Lifecycle result | intent, owner/generation, engine/helper owners, proxy transaction, protection reason | phase-specific receipt, retained uncertainty; App/UI | Existing operationMu/Manager/Jobs/WFP; no new owner |
| Route/DNS decision | signed profile/policy generation, network/profile scope, app/service/destination, namespace/resolver, carrier/proxy capabilities | eligible/unsupported/unknown + explain; renderer/platform adapter | COM-04 first; OS route/DNS/WFP evidence before protected claim |
| Protection evidence | config/session, WFP generation+LUID, evidence kind, network epoch, start/completion time, path targets | scoped protection projection; recovery controller | Lifecycle external evidence stronger than periodic local observation; no all-device promise |
| UI action | typed reason, evidence scope/age, current intent, allowed actor action | request receipt and later effect receipt; views/toasts | UI never owns authority; generation fences retained |
| Recovery episode (future) | fixed goal/scope/time basis and task producer | pending/success/failure/unknown; local opt-in | Z09/C06 contract; not prerequisite for W0–W4 safety changes |

Independent Windows deliverables W1, W2 and NetGuard projection corrections do not
depend on full Android/SDK completion. Split-DNS/proxy transport implementation
needs profile/security and platform compatibility, but read-only preflight and honest
unsupported states can ship first.

## Strong opposing cases and guards to preserve

- Another VPN happens to use the same loopback address/port: address predicate is
  not ownership. Existing nonmatching-port/external/PAC/live-listener guards are
  retained but insufficient for mutation.
- MDM/user changes proxy after Snowden writes it: expected-current conflict must
  preserve new settings. A mutex inside process cannot serialize external actor.
- Listener accepts TCP but is not usable SOCKS/Snowden: listener success is only a
  port observation; no positive repair/protection conclusion.
- Stop accepted while worker/helper close hangs: no successor/release; existing Job
  owner and `cleanup_unconfirmed` guard win over UX convenience.
- Network transition during probe: existing epoch/owner/intent publication rejects
  observed stale report. TUN no-event remains unknown; do not “refresh” timestamp.
- Private route through engine can work while direct app path is blocked; reverse
  also possible with other WFP layers. Config intent is not effective outcome.
- Proxy 200/browser works but connector auth/carrier/inner trust fails: report exact
  stage, no password spray or automatic transport downgrade.
- Scheduler ready/no-access and stale log event do not show current execution/effect.
- User Stop ends intent but does not prove task completion or foreign policy removal.

## Future acceptance — all NOT_RUN

1. **Proxy ownership:** unchanged owned values; PAC present; partial Set; foreign S2
   before restore/heal; competing writer during compare/readback; same address foreign
   listener/dead owner; crash/reboot and missing receipt. Verify no foreign overwrite.
2. **Lifecycle:** Stop/Disconnect/failure/restart/quit at every Start/Reload phase;
   engine/helper uncertain close; both helper lanes; stale result; WFP retained/released
   only by reason; successor forbidden until actual owners retire.
3. **WFP/TUN:** static filter arbitration, reauthorization, pre-existing TCP and
   one-way UDP, external provider/veto, BFE restart, IPv4/IPv6/DNS/reboot. Source
   enum/transaction/route removal is not acceptance.
4. **Route/DNS:** 192.168 versus excluded 172.16, IPv6/link-local/overlap, `.local`
   unicast versus mDNS/LLMNR/SMB, split suffix/resolver loss, corporate full-tunnel,
   proxy-named versus numeric endpoint, TCP versus HY2 UDP carrier.
5. **Observer:** physical network changes under TUN/no event, sleep/resume, callback
   loss, late report, network epoch ABA, periodic local observation versus lifecycle
   external evidence.
6. **Resources:** protected versus optional lanes; blocked network/IO callbacks;
   queued writers/buffers/helpers/process memory and scheduler PowerShell work. Timeouts
   do not refund work still running.
7. **UI:** factual states, stale/unknown, cleanup/settings conflict, task statuses,
   native WebView2 scaling/keyboard/screen-reader; wording comprehension. Browser/mock
   is not OS acceptance.

## Existing plan routing and disposition

- **P0 reuse/change:** FR-009, OFF-01/02 for barrier/process/cleanup ownership;
  W1 proxy owner is a missing prerequisite before current auto-heal writes.
- **P1:** FR-010, OFF-03/05 for operation/evidence/observer atomicity; COM-04/05/06/07
  for route/DNS/work; COM-12 resources; OFF-08/UI projections.
- **Later/deferred:** COM-09 quiet intent after safety receipts; COM-11 task readiness
  after producer contract; COM-17 seamless/bonding remains deferred.
- **Reject now:** broad LAN/direct fallback, automatic corporate policy changes,
  default credential reuse, second controller/daemon/store, generic “protection active”
  from Scheduler/port/readback, new database/telemetry framework.

## Sources, coverage and limits

Frozen knowledge unit coverage and dispositions are machine-readable in
`.artifacts/planning/knowledge-application/2026-10-28/coverage/Z08.json`.
Source cut and selected frozen excerpts are under the assigned private captures.
Current code/plan snapshots were copied sequentially, not atomically. No executable,
registry, WFP, proxy, DNS, NetGuard task or UI behavior was invoked. No tests/build/
web/new research occurred. Estimates of implementation time, runtime overhead or
field success are unknown; they are not filled with zero.
