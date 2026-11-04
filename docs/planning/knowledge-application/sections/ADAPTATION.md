# Адаптация: применение накопленной теории к существующей системе

## 1. Основание плана

Срез исходников и планов: 28 октября 2026. Приоритет — сохранить защищённое
восстановление, сделать решения объяснимыми и получить честные результаты
сравнения. Затем улучшать один измеримый аспект. Вероятностный оптимизатор не
является условием обычного Start, Stop, revoke или recovery.

**Уже написано, но не проверено исполнением:**

- Windows: applied-config eligibility, C2/D2/recent ordering, typed failure
  observations, общий diagnostic admission, current publication, switch/rollback.
- `shared/adaptive`: API1 Eligible/Rank/Prefer, четыре budget operations,
  ComposeEvidence, checked clock-unit helper и неподключённый ADP-005 prototype.
  Source API не заморожен; platform migration/parity отсутствуют.
- Windows manual native DNS: фиксированные udp4/tcp4/udp6/tcp6 observations,
  отдельный scope/expiry и memory-only getter. Это не periodic protected gate,
  не DNSSEC/quorum и не удостоверение маршрута всего TUN.
- Android: отдельные control/retirement lanes, original identity/elapsed facts,
  actual-admission recovery budget, physical/app fences. SessionJournal уже хранит
  typed worker observations с original producer/receipt times, а не только команды.
  Ring128/dedup/eviction не составляют complete attempt cohort.
- Android callback value steps уже проверяют original lease/phase/deadline
  до/после отдельных JNI/OS entries; late PFD/fact остаются у прежнего lease.
  Это не прерывание вошедшей foreign operation, не measurement permit и не
  доказательство latency/native quiescence. Эти guards сохраняются в адаптере.
- Android source уже содержит metered-aware TUN builder без allowBypass/allowFamily,
  checked protect, CE/no-backup encrypted checkpoint и import coordinator.
  ProfileTrustDriver в прочитанной границе — ещё контракт; это не принятый
  verifier/measurement permit. Split/package exclusions initial TUN bridge
  отвергает: UI/правила не должны объявлять их работающими до AND-06 gates.

**Открыто:** реальные platform adapters API1, Android AND-07 measurement producer,
accepted measurement authority, task/whole-trial outcomes, полный roster,
conditional statistical model и historical→future bridge. Наличие DTO, подписи,
HTTP200, HEALTHY или раннего sequence ID не закрывает эти пробелы.

Основные code/plan seams:

| Обозначение | Существующие компоненты / реестр |
|---|---|
| C1 — Windows выбор | `windows/scoring.go`, `chanhealth.go`, `failover.go`, `failover_attempts.go`, `transportpref.go`, `netwatch.go`; TH-02/04, COM-07 |
| C2 — Windows facts/effect | `probe_observation.go`, `observer_publication.go`, `observer_failover.go`, `backend/core/{probe,probe_http_observation,diagnostic_budget}.go`; TH-01/02/04, OFF-02/03/05 |
| C3 — portable policy | `shared/adaptive/{rank,eligibility,preference,evidence,evidence_types,evidence_time,failover_budget,retry_budget,observer}.go`; ADP-002…008 |
| C4 — Android owner | `android/vpn/.../VpnSessionRepository.kt`, `WorkerRpcLane.kt`, `WorkerProcess.kt`, `SessionJournal.kt`; AND-02/03/05/07/11/12 |
| C5 — Android native boundary | `android/corebridge/.../NativeRuntime.kt`, `NativeWorkerSession.kt`, `PhysicalPathContext.kt`, `NativePolicyLowering.kt`; AND-04/05/06/07, M01…06 |
| C6 — optional work | `windows/bgprobe.go`, `adaptive.go`, `backend/core/diagnostic_budget.go`, `shared/adaptive/observer.go`; ADP-005, TH-02/06, COM-12 |
| C7 — continuity/storage | Existing platform owner/retirement, Windows `backend/filetxn/bounded_write.go`; ADP-004/006, TH-01/04. Это не готовый statistical ledger |

Полные Android prefixes: `android/vpn/src/main/kotlin/org/snowden/android/vpn/`
и `android/corebridge/src/main/kotlin/org/snowden/android/corebridge/`.

## 2. Решения по знаниям и разница с текущим кодом

`source implemented` ниже означает наличие исходников, а не compile/unit/native/
device/live PASS. `already planned` сохраняет незакрытые критерии соответствующего
владельца. Сводка объединяет повторяющиеся положения старых и новых пакетов;
разные предпосылки повторов сохраняются.

### 2.1. Измерения, выбор и повседневное поведение

| Family | Знание и источник | Actual code + existing plan | Решение / полезная разница |
|---|---|---|---|
| <a id="F01"></a>F01 | Наблюдение не устанавливает причину: POLICY §8.1/17.3, R03 §12.5, archive §12.4 | C1/C2: `classifyObservedFailure` уже использует typed stages; measurement-only failures не quarantine. TH-02/OFF-05, AND-07 | **Already source implemented + adapt producer:** сохранять unknown/origin; добавить только реально наблюдаемые local admission/queue facts. Не возвращать inferred DPI labels для оживления legacy class helper |
| <a id="F02"></a>F02 | Exact identity/capability/clock, age отдельного факта: AD-01, R03-04, C03 §§2–3/7.2 | C2/C3: exact join, Known/Nanos/ClockDomain и windows clipping есть. Windows ordinary reports используют UTC и aggregate completion; native DNS — отдельный typed source. C4/C5 имеют elapsed/context. ADP-004/006, TH-02/04, AND-05/07 | **Reuse + change producer binding:** original generation interval, receipt age и wall display раздельно. Unknown planned/admitted/family/window не синтезировать. Coverage текущего пути не historical sampler |
| <a id="F03"></a>F03 | Полная baseline: recent→class→bucket→score→D2→stable; score-collapse: AD-02, R03-01 | C1 `scoredCandidates` объясняет C2 до двух поздних passes; C3 Rank уже возвращает AfterScore/AfterClass/Order/Trace. ADP-002/006, TH-04/COM-07 | **Reuse source + change final explanation:** выводить конечный target и причины всех passes. Сначала сохранить порядок. H/F/E производны verdict; новая настройка их весов не новая информация |
| <a id="F04"></a>F04 | Hard authority/safety отдельно от optional benefit: AD-06, R03-02, archive §12.1/2/5 | C2 current owner/intent/epoch, unsafe precedence, verified rollback; C4 pending/delivery/retirement fences. FR-009/010, TH-01/03/04, AND-02…06 | **Already source implemented / already planned:** сохранять и принимать по платформенным gates. Не строить второй controller и не заставлять safety ждать scientific storage, confidence, quiet или cooldown |
| <a id="F08"></a>F08 | Work/physical credits, α и authority раздельны; timeout не возврат: POLICY §13.6, R03-010 §4, Z03 §5.3 | C2 protected/optional rolling lanes, no refund admitted errors; C3 typed settlement; C4 retained lanes и actual recovery3/60s. ADP-003/006, TH-02/04/06, AND-11 | **Reuse + preserve compatibility:** разные budgets/threshold modes не объединять. Request/body caps не on-wire/CPU/RAM/energy/WCET bound. Error sentinel включает invalid/cancelled input; он не сам по себе измерение pressure |
| <a id="F12"></a>F12 | Диагностировать измеритель и сокращать optional work: AD-03, R03-06, C03 physical projection | C6 private pool counters, отдельные lanes и unconnected PlanAutomaticBackground. ADP-005, COM-12, AND-11 | **Already planned + adapt:** сначала coherent owner snapshot и только automatic background suppression. Не общий запрет manual/Endpoint/native/protected work, не выдуманный CPU score |
| <a id="F13"></a>F13 | Task readiness, quiet lease, scoped service/portal/split UX: archive §12, COMPETITOR-FEATURES §11 | C4/C5/SessionJournal — presentation/lifecycle facts; C2 manual diagnostics. COM-07/09/10/11/12, AND-08…12, A-FR-06/10 | **Already planned:** учитывать quiet/metered/roaming в optional policy; показывать точный scope/freshness/следующее действие. Не называть OS VALIDATED/handshake рабочим сервисом; не расширять direct grant из диагностики |
| <a id="F15"></a>F15 | Freshness feasibility: q/P — necessity, не EDF certificate; timer/context не SLO: R03-04/05/06 | C2 limiter/watchdog, C4 fixed lanes/elapsed; ADP-003/004/006, TH-02/04/06, AND-07/11 | **Adapt measured contract; defer optimizer:** сначала bounded waits/costs/owners, затем best-effort deadline ordering/coalescing по одинаковому scope. Unknown upper bound не физическая невозможность; lower bound за exclusive deadline — иной reason |

### 2.2. Учёт, статистика и сравнение политик

| Family | Знание и источник | Actual code + existing plan | Решение / полезная разница |
|---|---|---|---|
| <a id="F05"></a>F05 | Roster/interval labels/Γ/completeness: R03-02/03/15/16, R03-010 §5, C03-A, Z03-A/B | C2 latest-value health и один retained Manager report; C3 snapshot≤64; C4 ring128. **R28-REV-02** принял ограничение двух функций. ADP-004/006/008, TH-02/04/06, AND-07/16 | **Change producer, reuse owner:** отдельная bounded accounting projection original units; latest health не ломать. Сначала closed process-local cohort/summary. Durable/lifetime ledger — только при требуемом scope и отдельном storage/recovery review |
| <a id="F06"></a>F06 | Conditional A1/A2, delayed disclosure, family/subset, moving target: R03-010 §§2–3, scoped NINE review, DR27-08 | C3 пока не statistical accountant/CS; `Sequence`, custody и coverage не proof модели. ADP-004/006/008, TH-06/FR-012, AND-16 | **Adapt accepted conditions; defer probabilistic consumer:** original admission/reveal order и frozen predictor; не finish-only stream. Уже jointly covered subset/threshold не новый α. Новый target/model/reset — отдельная validity/composition |
| <a id="F07"></a>F07 | Past→future и full-policy causal estimand: R03-08/09/16, INTERVALS §§4–6, DR27-09, C03 §6.1 | C1 deterministic selection не даёт outcomes невыбранных policies; C4 RPC не user task sample. ADP-008, TH-06/07/FR-012, AND-15/16 | **Already planned + refine design:** assigned/admitted/executed/observed различны. Whole-policy ITT на common-dependency unit предпочтительнее сложного OPE. Support/interference/carryover/external validity отдельно. Unknown bridge → historical/descriptive result, без future superiority |
| <a id="F14"></a>F14 | VoI/minimax/EC²/DIRECT/ECED/Track-and-Stop: R03-06/07 | C6 suppression — не VoI; нет accepted joint likelihood/cost model. ADP-005 ограничен, ADP-008/TH-06 — future evaluation | **Defer runtime optimizer; reference principles:** purpose, stop, bounded search, joint dominance. Entropy не utility, zero one-step value не бесполезность связки. Не добавлять solver/ML stack/новый actor |
| <a id="F16"></a>F16 | Optional preference с horizon/drift/transition/full-trial cost: R03-12, C03 §4 | C1 `netwatch→tryPreferredChannelOnNetworkChange→switchChannelCoreFor`; автоматический netWatcher пропускает TUN. Prefer — class hint без measured benefit. TH-04/06, COM-09; separate future item после ADP baseline | **Defer rate/risk advisor; narrow future seam:** две сопоставимые full-policy target envelopes дешевле полной параметрической модели. Не переносить этот consumer на Start/manual/hard recovery/TUN/Android без своего caller review |
| <a id="F18"></a>F18 | A(D) parent acceptance, U(D) useful task, R(D) retirement; conditional chain и tails: R03-13/14, Z03 §3.2 | C2 Applied/RolledBack/PublishedCurrent и C3 OwnerReturned/ResourcesRetired; C4 pending→retained delivery→ack. ADP-003/004/008, TH-04/06, AND-07/11/16 | **Reuse typed outcomes + adapt metric:** rollback A не успех B. При полном final outcome не нужен event-tree estimator. Высокая deadline-success вероятность не mean/WCET cleanup bound; custody за H остаётся |
| <a id="F19"></a>F19 | Aggregate Γ versus weighted kernel, κ и correction-before-intersection: C03-B; **R28-REV-01** | Statistical wrapper в C3 не написан. ADP-004/006/008, TH-06/FR-012 | **Adapt accepted algebra as future gate:** exact Kernel/Envelope/Transform binding; numeric/model/filtration/bridge отдельно. Confseq runtime не требуется; не считать принятой всю CS/decision policy |
| <a id="F17"></a>F17 | Ошибочные §18/R03-17 и пересказы STATE/handoff009; erratum и R03-010 | В C3 нет такого statistical ledger. Старое требование ledger в shared противоречит pure ownership. Accepted review исправил union/credit/selection | **Reject superseded parts:** independence product как universal bound, mandatory fixed W, blanket infinite drift, unconditional selection penalty, shared durable store. Сохранить bounded registration/completeness; A1/A2 — достаточное, не универсально необходимое решение |

### 2.3. Готовое reuse и методические ограничения

<a id="F09"></a>**F09 — reuse:** источники AD-03/05, R03 SOURCES §§3–10,
SOURCES-010 C1/C2, C03 SOURCES и Z03 SOURCES. Сверка относится к сохранённым pins,
не к новым releases и не к полной transitive licence/security приёмке.

| Готовое | Решение / совместимость / цена |
|---|---|
| Existing API1 / Go sort/time | Reuse baseline/trace/typed facts; std-only pure module, уже используемый Go. Не новый clock/store/sorter. Go time.Time baseline не готовый Android elapsed FFI |
| memberlist `5cd2c4b0fb09caea5de03aae1adb9c36374ef3e0`, MPL-2.0 | Adapt local detector-awareness/soft suspicion principle; не gossip engine. Sender dedup не independence; hard expiry не растягивать |
| Envoy1.38.2 `f387231af8dd7274e37c5ae2cc797cb09a948818`, Apache-2.0 | Reference origin/sample-adequacy/detection versus enforcement. Не C++ sidecar, panic bypass или HTTP defaults для VPN |
| confseq0.0.11 `5ffe733ca2447a2e28c2c91f3b00086173f2ab2c`, MIT | Offline numerical/reference candidate. No NumPy/SciPy/pybind11 runtime. Weighted target/two-sided split/frozen tuning/postprocessing — binding, не готовый missingness producer |
| catoni-confseq `381d7fb17d2af5f4cc78d422c92900f2b1d5d1c4`, GPL-3.0 | Reference only; reported two-sided string/numeric split mismatch и growing-history/root work. Не nominal-α drop-in; repair/model/numeric/licence review прежде адаптации |
| bbolt `v0.0.0-20231014093535-ea5cb2fe9f0a`, full `ea5cb2fe9f0afaec4827c0591954db0e11ba548c`, MIT | Conditional low-level reuse при выбранной persistence. Не append-only/chronology/antirollback guarantee. Batch повторяем; Update/Sync могут ждать, View bytes копируются. CacheFile sing-box1.13.19 cleanup/reset — **reject для ledger** |
| OBP0.5.5 `8cbd5fa4558b7ad2ba4781546d6604e4cc3e07c4`, Apache-2.0 | Future offline IPS/DR reference, не VPN runtime/dataset. Python<3.10 и numerical/ML stack — существенная цена. Bootstrap API не validity для нашей выборки |
| Track-and-Stop `9161abd16be74ae82b47b4653ae5b1a6e6a037df`, MIT | Julia0.4 simulator reference; policy allocation/stopping только после подходящей модели. Не готовый network adapter/deadline guarantee |
| River0.26.1 `b674b758518341bc5c343ca113f34903ba55020a`, BSD-3-Clause | Offline change-hint reference. Python/Rust/PyO3 не добавлять в клиенты; sample clock не seconds, no-alarm не zero drift, buckets не hard constant memory |
| Android bridge/UI/import references из archive §12.7 | SFA/libbox whole-core, Mullvad/Proton/Amnezia — adapt только в AND owner scope с provenance. Не второй engine/Windows module. Hiddify Extended GPLv3 — только reference до licence decision; ZXing/NIA Apache-2.0 — existing Android plan, не adaptive dependency |

<a id="F11"></a>**F11 — качество evidence:** MEASUREMENT-CORPUS-AND-COVERAGE §§1–4,
RESEARCH-METHOD §10.3, archival source registers. Credential/signature/age флаг
не удостоверяет truthful DNS/TLS outcome или известный Γ. Test-path, upload-path,
collector, storage/API и dashboard — разные scopes. IP→DNS historical join должен
иметь дату/coverage; «не измерено» не «работает». Sparse RF reports/старые Android
policy dates не nationwide ranking/текущий release gate. Применить к TH-02/06/07,
AND-15/16 и privacy-bounded export; OONI credentials/backend/массовый scanner не нужны.
Administrative anchors, старые окна/status/failed fetch records не создают новый
product backlog; полезные mixed summaries сопоставлены содержательным families.

<a id="F10"></a>
## 3. Улучшенная последовательность по существующим компонентам

### Этап 0 — сохранить baseline и подготовить контракт миграции

**Приоритет P1; Windows + adaptive.** ADP-001/002/006, WIN-SDK-001-A, TH-02/04.
Независимая поставка: exact candidate-order trace и agreement полей/ошибок.

1. Снять один owned applied snapshot descriptors/selector/representation,
   dependency provenance, health/D2/recent и explicit time. Не перечитывать registry
   внутри comparator и не использовать UI-selected IDs.
2. Сохранить current bucket/score/class/recent/missing/tie branches. Финальное
   объяснение формировать после всех passes, с bounded exclusion/reason enum.
   Использовать имеющийся Rank trace либо расширить существующую presentation
   проекцию без второго алгоритма. Не менять веса под видом extraction.
3. Согласовать API1 caps/version/Go/module, stricter malformed handling и owner
   fallback/error policy. Никакой silent truncation, identity retagging или direct.
4. После отдельной parity приёмки — один consumer cutover и removal/thin-wrapper
   план прежнего кода. До этого единственный production baseline остаётся прежним.
   Остальные public API не требуют одновременной миграции.

Это не снимает открытые TH-01 safety/SDK/OS gates и не задерживает параллельное
P0 исправление. Design/read-only projection можно готовить независимо; effect
wiring и promotion допускаются только при нужных платформенных предусловиях.

### Этап 1 — реальные facts и согласование platform boundary

**P1; Windows/Android producers + adaptive contract.** ADP-004/006, TH-02,
AND-04…07, M01…06. Отдельные platform deliverables, общая семантика.

- Windows ordinary ProbeReport и manual native DNS не объединять по одному
  CompletedAt/health TTL. Добавить обоснованные generation intervals/clock-domain
  mappings и только наблюдённые route/family/capability facts. UI wall time отдельно.
- Сохранить current publication и unsafe exception. Не принимать новый narrow
  success как обновление всех required capabilities. Authority expiry не TTL observer.
- Android сначала получает accepted measurement plan от того же immutable
  accepted source: exact endpoints/operations/expected result/scope/original expiry,
  M01…06. Существующего trust API может хватить; новый DTO не самоцель.
- Далее one-operation native-outbound seam в **том же whole libbox**, explicit
  outbound/DialContext + standard HTTP/TLS, bounded optional lane/receipt. Не
  stock daemon URLTest batch; не TrySocks5 direct fallback/default public URL.
- Native-outbound HTTP, remote egress и user TUN/UID/DNS/family/work-task proof
  остаются разными capability kinds. No measurement API не превращать в fake Ready.
- Android recovery3/60s и original RPC delivery/ack не заменять Windows backoff.
  Rule lowering не считать ranking consumer; Wire/API/ABI/FFI проверяются отдельно.

### Этап 2 — один дешёвый deterministic improvement

**P1; Windows producer/consumer + adaptive.** Только существующий ADP-005,
TH-02/06, COM-12; Android AND-11 — самостоятельный будущий consumer.

1. Owner предоставляет coherent locked snapshot actual protected reservations
   со scope/domain/original observation time; старый draining work тоже может
   потреблять pool. Библиотека не читает private counters напрямую.
2. В automatic background branch до `bgProbeChannel`/его `bgMarkDone` выполнить
   typed suggestion и current owner recheck. Suppression не measurement success,
   failure или refund; не отмечать skipped канал как проверенный.
3. Unknown/future/stale/mismatched snapshot сохраняет ordinary admission, а не
   выдаёт новый grant. Manual site/native/required diagnostics не подавляются по
   одному имени optional lane. TUN skip нынешнего bg loop сохраняется.
4. Зафиксировать fairness/resume semantics в текущем bounded loop: без catch-up
   storm, нового timer/pool и потери Stop/revoke. Strict exclusion потребует
   атомарного owner admission; один прочитанный count не исключает гонку.
5. Сравнить только этот treatment с baseline: меньше optional work при protected
   нагрузке без худших recovery/missed faults/starvation. Численная польза unknown.

### Этап 3 — честный учёт без вероятностной оптимизации

**P1; platform producers.** TH-02/04/06, ADP-004/006/008, AND-07/11/16.
Independent output: bounded process-local accounting summary, пригодный для
descriptive/finite-cohort оценки даже без prospective model.

- Выбрать один наблюдаемый goal/unit/deadline: parent accepted-by-D **или** полезная
  задача, не оба под одним success. Outcome и cancel/reject/not-started semantics
  задаются до использования результатов. Отдельно action, service и resources.
- Controlled cohort регистрируется до relevant disclosure, original IDs/история
  сохраняются. Planned/admitted/started/revealed/recorded orders различаются.
  Дубликат доставки не новая unit; retry — child или declared intra-claim strategy.
- Health latest-value остаётся прежним. Старый unpublishable outcome может
  относиться к своей historical unit; его не переписывают новым owner/epoch.
- Closed bounded cohort/aggregates в existing owner state — первый вариант.
  Не требовать packet history, все URL или новые DB/event bus. Retention overflow,
  unknown membership/observer loss видны в counts/intervals; N=0 не success.
- Complete final goal обычно дешевле staged probability tree. Unknown label
  остаётся [0,1]; corrupted labels требуют обоснованной Γ, не estimated confidence
  по числу одинаковых replies. Physical return/retirement не следует из maturity.
- Если выбран durable/lifetime scope: register/debit → acknowledged commit →
  platform gate → owned dispatch → outcome/retirement. Unconfirmed commit/ACK или
  dispatch crash сохраняют original ID/unknown; blind resend запрещён. DB-only
  callbacks, bounded retained writer, no I/O under publication locks. Restore
  lineage вне rollback domain либо narrower guarantee/abstain. Это отдельный
  platform design; mandatory safety и Disconnect journal не ждут.

### Этап 4 — условные bounds и gate статистического адаптера

**P2, после этапа3; adaptive + platform/model owners.** ADP-004/006/008,
TH-06/FR-012, AND-16. Принятые формулы не переизобретать.

1. Сначала finite registered loss interval: denominator/completeness/support/Γ
   должны быть обоснованы. Более сильный statistical mode требует названных
   filtration/model/error family, а target mode — отдельного bridge.
2. Для минимум достаточного варианта использовать уже принятый unweighted
   historical construction DR27-08. Confseq — optional offline reference, не
   client runtime dependency и не необходимый способ получить finite arithmetic.
3. **R28-REV-01 — обязательный future adapter gate:** exact kernel/envelope/target
   binding. Для nonincreasing weights нужна joint prefix envelope и κ;
   aggregate Γ_n сама не weighted error. Более простой frozen constant kernel
   допускает current Γ для raw prefix bound, но имеет nonzero limiting radius.
   Варианты не объявлены оптимальными/реализованными.
4. Corruption correction применяется к каждому prefix **до** running intersection
   и только для одного target. Для changing θ_n пересечение разных targets не
   разрешено. Frozen tuning, N=None assumptions, α/2 sides, finite precision/
   overflow/underflow и численная parity — отдельные критерии. Tiny unsupported α
   не превращается в нулевой риск.
5. A1/A2/accountant относятся к actual admission history, включая использованные
   pending/partial outcomes. Physical credit не α. Joint label error β и bridge η
   учитываются вместе с α_stat суммарно; independence не выдумывается.
6. Output явно FiniteRegistered / JointHistorical / JointTarget + WhyUnknown.
   Непрозрачный GuaranteeRef не доказательство. Unknown bridge не заполнять L=0,
   pmax=0 или удобным H; ordinary protected recovery продолжает свою работу.

Rate/drift/horizon advisor R03-12 остаётся **deferred**. Если уже будут обоснованы
два full-policy target intervals в общих loss units, достаточно их конечного
сравнения с margin, без всей параметрической модели. B включает transition,
probe, rollback и failure costs. Любой совет проходит fresh owner gate.

### Этап 5 — доказать добавочную пользу и решить о выпуске

**P2; TH-06/07/08, FR-012, ADP-007/008, AND-15/16.** Все execution gates сейчас
**NOT_RUN**; план не разрешает запуск стенда или VPS.

Сначала narrow contract/parity/negative приёмка, затем platform native/device
scope; replay проверяет semantics, не counterfactual benefit. Comparative design
назначает **целую policy** на заранее заданную eligible/common-dependency unit,
offered workload и H. ITT сохраняет rejected/waits/cancel/rollback/missing и
полный cost; assignment, applied transport и useful outcome не смешиваются.

Primary metric — полезные завершения≤D либо declared outage/recovery loss;
допуски churn/disruption/bytes/CPU/RSS/energy и missed faults задаются заранее.
ISP/day/device/family/load/cold-warm blocks и held-out условия отдельно. Shared
VPS/interference, unknown carryover, unsupported rollout exposure и outcome loss
не исправляются bootstrap/DR/seed/epoch reset. Field после оплаченной infra и
отдельного разрешения; build/отсутствие ошибок в модели не VPN PASS.

## 4. Минимальные контракты и совместимость

| Граница | Минимум / владелец | Чего не добавлять |
|---|---|---|
| Platform→baseline | Owned applied set, original Scope/Now, health/D2/recent/dependency projection, eligibility reasons | Callback registry в comparator; raw config/secrets/UI authority |
| Producer→coverage | Original identity/path/image/cutover/app context, exact Capability keys, known timestamps/domain/window/hard bound, typed state/origin/censor/custody | Новый label вместо отсутствующего family/expiry; re-tag старого receipt |
| Owner→ADP-005 | Coherent count/presence + original time/domain/scope, Expected | Count=0 при неизвестном pool; skip как permission или success |
| Owner→accounting | Goal/population/policy refs, bounded original roster либо sufficient summary, all branch dispositions, completeness/unknown membership | Ring/state log как полный dataset; lifetime storage без заявленной цели |
| Bound→optional advice | BoundKind, exact target/units/H, family/model/kernel/envelope/transform/bridge refs и два совместных intervals; current Expected/relevant capacity | Score→probability; false certainty по одной строке witness=true |
| Advice→effect | Fresh current trust/protection/owner gate и actual atomic claim/reservation, original settlement/retirement | Второй lifecycle owner, direct fallback, release по timeout |

API1≤64 descriptors/receipts и≤16 capability keys — существующие structural caps,
не измеренные SLO и не lifetime limits. Strict rejection duplicates/nonfinite/
oversize/version mismatch — совместимость, требующая owner решения. Не обрезать
неподдержанные старые профили молча. Metadata ID validation строже SDK token
grammar и остаётся платформенной. Baseline Go time.Time и evidence Known/Nanos
не один Android wire format: checked ms→ns меняет единицы, не clock domain.
Original UUID/owner refs нельзя превращать lossy hash в token; source0 sentinel
не known-zero coordinate. Старый IPC worker version не adopted новым клиентом.

Не включать все десять SDK operations одним cutover. В budget migration сохранить
default-two / legacy-N / observed-N, retained-running exception, wrapper prebranches
и rollback hint различия. Retry ScheduleGeneration — новое усиление относительно
старого attempt-index timer, требующее owner source/parity, а не уже готовая
гарантия. OwnerEligible включает current TH-01 trust/image/cleanup/restart fences.

## 5. Приёмка и сильные контраргументы к плану

Все строки — **будущая приёмка NOT_RUN**, без исполняемых fixtures.

| Сценарий | Критерий / сохраняемый сильный guard |
|---|---|
| C2 winner отличается от final target; missing descriptors; recent TTL edge | Baseline order/parity и полный bounded rationale; не скрытое изменение приоритета |
| Actual timeout вместо legacy causal class; measurement outage | Нет inferred DPI/массовой quarantine. Existing typed classifier/measurementOnlyFailure сохранены; обязательная safety policy продолжает действовать |
| Early sub-fact, late aggregate; wall step; sleep/boot change | Original generation/elapsed domain и earliest required validity; delivery не renewal; no health-clock→security-bypass claim |
| Новый narrow success/поздний old success/новый unsafe | Exact key/context/latest decisive rules; unsafe-order exception сохранён; no implied family upgrade |
| Busy/no-session scheduled event или slow failure после fast success | Correct population/roster, а не reports-only denominator. Early ID и health filters сами completeness не доказывают |
| B failed, rollback A verified, user task неизвестна | B не success; A/service/task/resource outcomes раздельны. Existing verified-rollback исключение не превратить в BLOCKED живого пути |
| Observer timeout, RPC returned delivery ещё не acknowledged | Retain original owner; pending==null не free lane. Existing WaitGroup/RPC retirement guards сильнее blanket «cancel всё освободил» |
| Protected pressure/unknown snapshot/manual request/long suppression | Только exact automatic purpose, coherent current recheck, no fake bgMarkDone/FAIL, bounded fairness. Separate lanes уже ограничивают admission; дополнительный benefit нужно измерить |
| Commit/Sync/ACK uncertain; crash после dispatch marker | Reconcile same ID, unknown started membership, no blind action/refund/new family. No Batch side effects; storage не мешает P0 safety |
| K1 weighted Γ, K2 correction order, changing target | Gate R28-REV-01 + отдельные model/numeric acceptance; ни одно library name не заменяет proof binding |
| Two post-hoc inspected claims versus covered subset | A1/A2 valid family или no inference; covered subset/threshold не новый α; старое R03-17 не используется |
| High deadline-success, неограниченный cleanup tail | Fixed-H cost и outstanding custody; no finite mean/WCET из малой tail probability |
| Whole-policy comparison с admission filtering/shared VPS/carryover | Complete ITT denominator и actual exposure support; replay/one-step IPS не общий rollout effect |
| Old client/oversize/unknown field/FFI overflow/stale timer | Explicit compatibility refusal/owner policy, no truncation/direct/new owner; migration по одному consumer |

Source review подтверждает только прочитанные branches/контракты. Compile/unit,
numerical, native/OS/device, field и release — разные уровни; прежние PASS не
переносятся на изменённые inputs. Успешное выполнение одного уровня не закрывает
остальные и не устанавливает превосходство над существующими VPN.

## 6. Что не включать в ближайшую разработку

- Новую настройку весов C2, learned probabilities из health/independence hints,
  nationwide ranking из sparse reports или фиксированные «16КБ»/jitter presets.
- VoI/bandit/RL/ADWIN runtime, второй scheduler/actor/engine, универсальную БД,
  полный event tree либо новый DTO при уже достаточном owner summary.
- Cloud/TPM/hash-chain как автоматическое доказательство complete roster;
  device-lifetime error guarantee без continuity и restore scope.
- Новые транспорты ради статистики; Python/Rust/Julia evaluation stacks в клиент;
  OTel exporter/OONI measurement upload/сбор user browsing history по умолчанию.
- Требование полного вероятностного forecast перед recovery или задержку
  Stop/revoke ради model/storage/quiet. Unknown — явный предел optional inference.

## 7. Источники решений

- [POLICY-AND-EVIDENCE](../../../theory/adaptation/POLICY-AND-EVIDENCE.md): §8/13.6/17; исходные архивные §8/12/17 сохранены отдельно.
- [R03 REPORT](../../../theory/research/03-ADAPTATION/REPORT.md): §§4–17; соответствующие handoffs001…008 и SOURCES содержат pins/ограничения. §18/009 применяется только вместе с исправлением.
- [R03-010](../../../theory/research/03-ADAPTATION/REPORT-010.md), [scoped acceptance](../../../theory/research/reviews/NINE-2026-10-27-R03-010-A.md), [erratum R03-17](../../../theory/research/reviews/NINE-2026-10-27-R03-17-ERRATUM.md).
- [INTERVALS-AND-POLICY-EVALUATION](../../../theory/adaptation/INTERVALS-AND-POLICY-EVALUATION.md), [DR27-08/09](../../../theory/research/reviews/2026-10-28-DEEP-REVIEW.md): conditional model scope.
- [C03](../../../theory/research/cycles/2026-10-28-contracts/C03/REPORT.md), [Z03](../../../theory/research/cycles/2026-10-28-contracts/Z03/REPORT.md), [R28-REV-01/02](../../../theory/research/cycles/2026-10-28-contracts/REVIEW-01.md): algebra/source приняты; full producer/storage/model не приняты автоматически.
- [Corpus quality](../../../theory/methodology/MEASUREMENT-CORPUS-AND-COVERAGE.md), [research method](../../../theory/methodology/RESEARCH-METHOD.md), [competitor mechanisms](../../../theory/ux/COMPETITOR-FEATURES.md).
- Existing adaptive PLAN — `docs/adaptive/PLAN.md`, API — `docs/adaptive/API.md`, adapter matrix — `docs/adaptive/ADAPTER-MATRIX.md`, consumer request — `docs/adaptive/CHANGE-REQUEST.md`, [Windows PLAN](../../../PLAN.md), Android PLAN — `docs/ANDROID-PLAN.md`, measurement boundary — `docs/android/MEASUREMENT-NATIVE.md`.

Номера строк относятся к captured source cut; исходники развиваются параллельно.
Адресная provenance, все unit dispositions и late-input reconciliation находятся
в private coverage/results C03. Непрочитанные части сторонних implementations и
непроверенные условия не повышаются до product requirements или PASS.
