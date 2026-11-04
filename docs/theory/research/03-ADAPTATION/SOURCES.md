# R03 — источники и границы первого среза

Дата чтения: **2026-10-25**. Первичные материалы; review руками, без выполнения
продукта/чужого кода. Exact bytes локального source сохранены в
`.artifacts/research/theory-03/local/`, SHA256/время в `local-inputs.json`.
Дерево развивается параллельно; вывод относится к captured bytes, а не ко всем
будущим версиям. В отсутствие committed baseline hash файла важнее номера строки.

## 1. Локальная база

Пути в таблице — от корня `D:/SNOWDEN-SYSTEM-V3`.

| ID | Файл / символ | Что прочитано и что это доказывает |
|---|---|---|
| L01 | `docs/CENSORSHIP-THEORY.md` §8, §10, §13.6–7, §14.5, §16.4, §17 | Existing context/epoch/freshness, budgets, missing outcomes, partial order, local detector, confidence и safety-first. Это baseline, не новые результаты R03 |
| L02 | `windows/scoring.go`: scoreCandidate, healthBucket, rankCandidates, scoredCandidates | C2: healthBucket → score desc → D2 → stable input; AD-02 algebra сохраняется |
| L03 | `windows/failover.go`: failoverCandidates, tryFailoverWithObservation, switchChannelCoreFor, finishChannelSwitchError, finishFailoverAttempt, orderCandidatesForFailure, countermeasureTier | От applied descriptor eligibility через три прохода до admission, switch/reload, fresh publication и verified rollback; разные ветки caller |
| L04 | `windows/failover_attempts.go`: expireAttemptsLocked, noteAttemptFailure, orderByRecentAttempts | Stable partition recent vs nonrecent; hint TTL=2×reloadCooldown; hint не health evidence и не исключение из eligible set |
| L05 | `windows/observer_failover.go`: observedSwitch, reserveObservedSwitch, finishObservedSwitch, endScopedAttempt | Отдельные decision/attempt/current runtime identities; verified rollback не switched=true; отмена/поздний callback не обязаны стать channel failure |
| L06 | `windows/observer_publication.go`: publishObservationLocked | Intent/session/owner/epoch/observation fences, required source/trust checks, protected phase publication, scoped health write; unsafe positive evidence отдельно от обычного order |
| L07 | `windows/failclass.go`: ClassifyFailure; `windows/failure_observation.go`: classifyObservedFailure, measurementOnlyFailure | Live classification typed; четыре класса countermeasureTier не производятся обычным current typed classifier. Это ограничивает практический смысл class counterexample |
| L08 | `windows/corebridge.go`: watchCore; `windows/netwatch.go`: netChangeReprobe / tryPreferredChannelOnNetworkChange; `windows/probe_observation.go`: currentProbeReport / observation stamping | Watchdog confirmation→typed classifier→observed failover; network preference — отдельная ветка, не тот же ranking. Existing identity reuse, не новый owner |
| L09 | `windows/backend/core/probe.go`: ProbeReport, ProtectedProbe.Run; `windows/backend/core/diagnostic_budget.go`: AdmitDiagnostic | Report IDs/stages, reserved lanes, admission cap, no error refund. Успех узкого probe не пользовательская задача и не все families |
| L10 | `windows/controller.go`, `windows/chanhealth.go`, `windows/bgprobe.go`, `windows/quarantine.go` | Presentation/recovery state, health vs work hints, weak background failures, source-specific scope. Negative health не получает автоматический TTL в verdict; не предлагаем обнуление отказов как trust recovery |
| L11 | `android/vpn/.../ConnectionState.kt`, `PhysicalNetworkState.kt` | OS/connection intent, issue, physical owner/epoch и отсутствие invented protected Ready; source hints не speed/egress proof |
| L12 | `android/vpn/.../VpnSessionRepository.kt`: completed, pending timeout, release; `WorkerRpcLane.kt`: Call, submit, takeCompleted | Один potentially-blocking lane, provisional10s observation deadline не cancellation/return; typed result, admission не завершённый переход |
| L13 | `android/vpn/.../SessionJournal.kt`: SessionEvent, stateChanged, append, clear | Ring128, state-pair dedup, evictions/limited; не complete episode/action/outcome trace. Уже известно OP-04; новое R03 — accounting contract |
| L14 | `docs/PLAN.md`: TH-02/04/06, COM-07/09/12; `docs/ANDROID-PLAN.md`: AND-07/11/16 | Только маршрутизация предложений. Research packet не меняет текущие product statuses |
| L15 | `windows/go.mod`: go1.26 | Совместимое семейство stdlib reference. Фактическая установленная toolchain здесь не запускалась/не утверждается |

Имена Android в L11–13 сокращены только в таблице: полный префикс
`android/vpn/src/main/kotlin/org/snowden/android/vpn/`. Полные exact paths/hash
каждого из 27 inputs находятся в manifest. Чтение scoped source не является
полным аудитом платформ или доказательством достижимости каждого возможного caller.

## 2. Новые внешние первоисточники

| ID | Первичный источник / версия | Использование и ограничение |
|---|---|---|
| P01 | [Go sort docs](https://go.dev/pkg/sort/?m=old), [go1.26.0 slice.go](https://github.com/golang/go/blob/d90b98e65320778f3b1f99a6951ab20f04d218b3/src/sort/slice.go) | SliceStable сохраняет порядок равных элементов. Точный reference commit `d90b98e65320778f3b1f99a6951ab20f04d218b3`; tag разрешён через GitHub commit API. Производный lexicographic вывод R03-01 сделан вручную |
| P02 | [Go LICENSE при том же commit](https://github.com/golang/go/blob/d90b98e65320778f3b1f99a6951ab20f04d218b3/LICENSE) | BSD-3-Clause; уже используемый stdlib sort, новую dependency/копию сортировщика не вводить. License notice не доказывает лицензионную чистоту всего приложения |
| P03 | [gRPC cancellation](https://grpc.io/docs/guides/cancellation/), official HTML capture 2026-10-25 | Отмена требует участия обработчика; состояние ожидающего не равняется возврату начатой работы. Только reference principle, gRPC в проект не добавляется |
| P04 | [gRPC core concepts: RPC termination / cancellation](https://grpc.io/docs/what-is-grpc/core-concepts/) | Наблюдатели могут получить разные результаты; уже сделанные изменения отменой не откатываются. Аналогия не специфицирует Binder/Go ownership проекта |
| P05 | [Imbens & Manski, CWP09/03](https://www.cemmap.ac.uk/wp-content/uploads/2020/08/CWP0903.pdf), draft **2003-05-29**, §2–3, PDF pp4–6 | Различие identification region и sampling confidence; missing bounded outcomes. Проверены постановка и формула §3, не весь statistical proof. Конечный расчёт R03-03 — отдельное ручное применение, асимптотические CI не внедряются |

В `.artifacts/research/theory-03/external-inputs.json` пять сохранённых файлов,
полные URLs/SHA256/время. `external/go1.26.0-reference.json` — tag→commit metadata.
Mutable HTML закреплён capture hash, это не обещание неизменности online page.
PDF — research copy, не код с разрешением на перепубликацию. gRPC code не копируется.
Go reference — не утверждение установленной patch version.

### Ограничения доступа

Publisher abstract Imbens–Manski 2004 DOI `10.1111/j.1468-0262.2004.00555.x`
найден, но вывод не приписан проверке полного journal text. Harvard PDF retrieval
в web reader завершился timeout, ScienceDirect Manski2005 full page —403.
Использован доступный первичный CWP09/03, не пересказ/AI summary этих текстов.
Дата upload в URL/поиске не дата научной публикации.

## 3. Уже закреплённый reuse: повторно прочитанные scoped captures

Исходный manifest: `.artifacts/research/2026-10-25-adaptive-evidence/upstream-sources.json`.
Эти pin относятся к baseline §17, не выдаются за новые releases на сегодня.

| Компонент | Pin / license | Прочитанная часть / решение R03 |
|---|---|---|
| [memberlist](https://github.com/hashicorp/memberlist/tree/5cd2c4b0fb09caea5de03aae1adb9c36374ef3e0) | `5cd2c4b0fb09caea5de03aae1adb9c36374ef3e0`, MPL-2.0 | awareness.go и suspicion.go: local saturation/soft timeout, sender dedup. Сохранить AD-03 principle; quorum sender count не independence, cluster не импортировать |
| [Envoy](https://github.com/envoyproxy/envoy/tree/f387231af8dd7274e37c5ae2cc797cb09a948818) | v1.38.2 `f387231af8dd7274e37c5ae2cc797cb09a948818`, Apache-2.0 | outlier_detection.proto: origin split, detected/enforced, request volume. Не импортировать C++ proxy/panic escape; разграничение detection/action полезно как reference |
| [confseq](https://github.com/gostevehoward/confseq/tree/5ffe733ca2447a2e28c2c91f3b00086173f2ab2c) | `5ffe733ca2447a2e28c2c91f3b00086173f2ab2c`, MIT | README и сохранённая license metadata; numerical core не просмотрен. Остаётся future offline reference; не исправляет отсутствующие outcomes, не нужен для конечной арифметики |

README confseq содержит install/demo инструкции. Это source data, команды из него
не выполнялись. Lifeguard model уже закреплена §17; нового независимого VPN
результата из повторного чтения memberlist не заявляем.

## 4. R03-002 — свежесть и расписание

2026-10-25, новый независимый cut. Предыдущие L/P записи относятся к R03-001;
прошлое отсутствие drift не переносится на новый срез. Captures/manifest:
`.artifacts/research/theory-03/cycle-002/`. Прочитана только относящаяся к age,
admission и publication цепочка, не whole-platform audit.

### 4.1. Scoped source

| ID | Exact path / symbols | Значение и ограничение |
|---|---|---|
| T01 | `windows/probe_observation.go`: appProtectedProbe.Run, currentProbeSubject, newAppProbe | Aggregate UTC start/completion, owner/epoch/order guards, default10s timeout; lifecycle boundary отдельно от periodic Run |
| T02 | `windows/backend/core/probe.go`: ProtectedProbe.Run | Parallel steps, wg.Wait, aggregate timestamps, q/body reservations, outer context; не доказанный WCET |
| T03 | `windows/backend/core/probe_http_observation.go`: ProbeObservation, fetchProbeHTTPRequest | Sub-step StartedAt.UTC и elapsed time.Since; exact generation remote state не аттестован |
| T04 | `windows/backend/core/probe_dns.go`: checkDNS | Sequential fallback под общим admitted budget/remaining deadline; cached DNS freshness уже OP-01 |
| T05 | `windows/chanhealth.go`: evidenceFreshMax, healthState.verdict, GetChannelHealth; `windows/observer_publication.go`: publishObservationLocked | LastOKAt из aggregate CompletedAt, positive TTL2min, wall-time comparisons; projection/ranking не полный security gate |
| T06 | `windows/backend/core/diagnostic_budget.go`: AdmitDiagnostic | Protected120requests/1MiBbody/rolling60s/2whole active ops; optional lane отдельно; reservation без refund, release по actual defer. Это не cap всего OS traffic |
| T07 | `windows/corebridge.go`: watchdogInterval, fastReprobeAfter, watchCoreTicks | Period30s, gaps5s×2, outer contexts15s, sequential confirmation и tick skips; без универсального recovery bound |
| T08 | `windows/trust_lifetime.go`: sessionLifetime/checkpointSessionClock; `windows/backend/metadata/clock.go`: ObservedClock | Independent wall/steady logical projection, floor/trust checks. Clock counterexample health не доказывает обход этих gates; сами gates не runtime-tested |
| T09 | `android/vpn/src/main/kotlin/org/snowden/android/vpn/OsProtection.kt`: read, isFreshOsObservation | Start-dated elapsed observation,5s provisional TTL, не protected egress proof; применимый local reuse |
| T10 | Тот же Android префикс: `WorkerProcessLink.kt`: Observation/inspect; `WorkerRpcLane.kt`: Call/takeCompleted; `VpnSessionRepository.kt`: pending/completed | ReceivedAt отличается от generation, pending owner переживает observation deadline; нет подтверждённого upper bound Binder actual return |
| T11 | `docs/CENSORSHIP-THEORY.md` §8/13.6–7/14.5/16.1/17, scoped START/README; `docs/PLAN.md`, `docs/ANDROID-PLAN.md`; R03-001 handoff | Baseline и адреса требований; чужие source/statuses не переписаны |
| T12 | `shared/adaptive/{types,health,rank,validation}.go`, README/go.mod, `docs/adaptive/API.md`, PLAN | Поздно найденный API1 baseline: caller supplied Now/time.Time и platform gate, full rank trace; ADP-003 budget effects active/source plan, ADP-004 joins planned. Сам module не UTC-converts inputs и не даёт authority. Consumer parity/FFI не проверялись |

Полные30 captured paths/SHA256/UTC и before/copy/after — `local-inputs.json`.
Один ранний guessed path `windows/lifecycle_probe.go` не найден и честно записан
not_found; корректный `windows/backend/core/lifecycle_probe.go` найден по rg и
сохранён. Это не отсутствующая capability продукта. Внешние manifests содержат
6 captures; код не выполнялся. Номера строк в живом дереве могут сместиться.

### 4.2. Первичные материалы и reuse

| ID | Источник / pin | Прочитанный scope / решение |
|---|---|---|
| P06 | [Sun et al., Update or Wait, arXiv1601.02284v9](https://arxiv.org/html/1601.02284v9),2017-04-21; §I/III/IV setup | Age metric и model assumptions; «zero wait» оптимальность зависит от модели. **Adapt objective/reference**, не numerical/SMDP runtime. Earlier v4 page найден, пакет закрепляет v9 |
| P07 | [Liu & Layland, JACM20(1),46–61,1973](https://bears.ece.ucsb.edu/class/ece253/papers/laylandliu77.pdf); DOI10.1145/321738.321743, assumptions§3/theorem7 | Preemptive single-processor periodic/independent/constant-runtime model; **reference** deadline ordering, не proof сетевого EDF. Имя URL77 не дата публикации |
| P08 | [Go1.26.0 time/tick.go](https://github.com/golang/go/blob/d90b98e65320778f3b1f99a6951ab20f04d218b3/src/time/tick.go) | NewTicker drop/adjust behavior; existing primitive reuse. Не гарантия доставки tick по wall deadline |
| P09 | [Go1.26.0 time/time.go](https://github.com/golang/go/blob/d90b98e65320778f3b1f99a6951ab20f04d218b3/src/time/time.go) | Clock docs, UTC/setLoc/stripMono, Sub/Since; wall vs monotonic/process/sleep semantics. **Reuse clock contract**, фактическая Windows suspend behavior не проверена |
| P10 | [Android SystemClock API](https://developer.android.com/reference/android/os/SystemClock), HTML2026-10-25 | elapsedRealtime vs uptime/sleep; source API contract, не device benchmark или callback timing SLA |
| P02 reused | [Go LICENSE при том же commit](https://github.com/golang/go/blob/d90b98e65320778f3b1f99a6951ab20f04d218b3/LICENSE) | BSD-3-Clause; captured повторно, hash тот же. Новых libraries/scheduler code не добавлено |

Exact Go reference commit **d90b98e65320778f3b1f99a6951ab20f04d218b3**, ранее
разрешённый taggo1.26.0. Это reference, не утверждение installed toolchain.
Имена/URLs/SHA256/UTC всех внешних файлов — `external-inputs.json`.
AoI/arXiv non-exclusive publication license и paper copyright **не** дают готовую
software license для Matlab appendix/чужого SDK. Code из papers не копируется,
команды/симуляции не выполняются; выбран перенос идей/контрактов с атрибуцией.
Android API prose используется как reference, examples не копируются.

### 4.3. Собственные выводы и границы

- Интервал `t−e≤age≤t−s`, q/P demand necessity, packing example3×3s/2slots/D5s,
  latest-start formula и условное P+3C+2g — ручное применение/вывод в REPORT§11.
  Это не численный experiment и не новые literature theorems.
- Средняя AoI, максимальный age, допустимость trust и useful task outcomes
  различны. Stationarity/ergodicity/known conditional cost у VPN не установлены.
- Неисправный endpoint не обязан дать successful fresh proof при любом schedule.
  WCET отсутствует, consumer timeout не заменяет measured actual return.
- На source freshness cut `2026-10-25T14:17:39Z`22 captured inputs unchanged;
  полная запись в `drift-at-review.json`. Это адресное read/hash evidence,
  не атомарный snapshot и не product PASS. Поздний drift требует пересмотра.
- На следующем cut14:22:08Z изменились docs/PLAN.md (V2-158 resource custody и
  adaptive API source reference) и docs/ANDROID-PLAN.md (AND-12-S6 report UI).
  Прочитаны relevant diffs; это не закрытие TH-02/04/06 или AND-07/11/16.
  Параллельные изменения сохранены. После обнаружения adaptive source добавлены
  ещё8 scoped captures T12. Итог — `final-cut.json`, а не утверждение no drift.

## 5. R03-003 — decision-focused измерения

2026-10-25. Primary literature/public code прочитаны без выполнения. Срез
дополняет §1–4; прежние captures/handoffs не заменены. Общий THEORY17 уже содержит
AD-01…06; новый вклад — bounded выбор измерений/продолжений, dominance conditions,
идентифицируемость доступными experiments и distinction possible/guaranteed time.

### 5.1. Точные первичные версии

| ID | Источник / дата / scope | Что использовано |
|---|---|---|
| P11 | Golovin, Krause, Ray, [Near-Optimal Bayesian Active Learning with Noisy Observations, arXiv1010.3091v2](https://arxiv.org/html/1010.3091v2), **2013-12-16**; исходная конференция2010 отдельно; §3 Theorem3, §4 | EC² cross-decision edges, cost-sensitive surrogate, expanded correlated-noise realizations, decision classes/tie. Known joint prior не равен калиброванному VPN prior; гарантия определения класса не arbitrary deadline optimization |
| P12 | Chen, Javdani, Karbasi, Bagnell, Srinivasa, Krause, [Submodular Surrogates for Value of Information, extended author PDF](https://las.inf.ethz.ch/files/chen15submsrgtvoi-long.pdf), AAAI2015; [publication metadata](https://ojs.aaai.org/index.php/AAAI/article/view/9694), DOI10.1609/aaai.v29i1.9694, metadata published2015-03-04; pp2–5/Theorem2 | Decision utility/regret, overlapping decision regions, специальный Noisy-OR из ECD graphs. Не general submodularity raw VoI. Finite known joint model/efficient inference. Noisy-OR название не означает physical independence endpoints |
| P13 | Chen, Hassani, Krause, [Near-optimal Bayesian Active Learning with Correlated and Noisy Tests, arXiv1605.07334v2](https://arxiv.org/html/1605.07334v2), **2016-07-11**; §2/3/4, Algorithm1/Theorem1 | ECED discount/offset, conditional independence при Θ, known prior/likelihood/unit costs; основная гарантия binary/root-cause-independent noise. AISTATS2017/journal extension не названы прочитанной v2 |
| P14 | Garivier, Kaufmann, [Optimal Best Arm Identification with Fixed Confidence](https://proceedings.mlr.press/v49/garivier16a.html), COLT **2016-06-23…26**, PMLR49:998–1027; [PDF](https://proceedings.mlr.press/v49/garivier16a.pdf) §1/2/4/5/Theorem14 | Independent fresh fixed-distribution draws, exponential family/unique best; tracking и stopping, fixed-confidence/asymptotic complexity. Не wall-clock/dynamic-network гарантия |
| C03 | [Авторская страница Garivier](https://perso.ens-lyon.fr/aurelien.garivier/www.math.univ-toulouse.fr/_agarivie/index065e.html?q=node/160) → [jsfunc/best-arm-identification](https://github.com/jsfunc/best-arm-identification/tree/9161abd16be74ae82b47b4653ae5b1a6e6a037df), exact **9161abd16be74ae82b47b4653ae5b1a6e6a037df**, committer2016-07-31T07:47:17Z | `mainBAI.jl:MCexp/policies → BAIalgos.jl:TrackAndStop/OptimalWeights/sample → recommendation,N → simulation summaries`; `KLfunctions.jl`; README, LICENSE.md. MIT2016 Garivier/Kaufmann; Julia0.4.0 и README dependencies, не текущая Go/Android API |

P11/P13 имеют arXiv non-exclusive publication license; P12 печатает AAAI2015
copyright. Это не разрешение копировать найденную позже чужую software library.
C03 MIT applies к собственному code репозитория при сохранении notice; licenses
Distributions/HDF5/PyPlot этим capture не проверены. Paper code/pseudocode в
product не перенесены. Ограниченный поиск paper/author links не установил
pin/license/API code P11–13; глобальное отсутствие code не заявлено.

### 5.2. Локальный source scope и current status

33 первоначальных пути и exact hashes находятся в `cycle-003/local-inputs.json`.
Relevant groups:

- `windows/bgprobe.go`: loop/round/due/channel → Background admission → TCP
  → bgReg/chanHealth; refusal не channel failure. QUIC no-probe сохранён.
- `windows/adaptive.go`: ручной RunAdaptiveCheck/Endpoint/classifySite → wg.Wait
  → session-current gate → adaptiveLast/persistAdaptive. No secret/runtime data.
- `windows/probe_observation.go`, `backend/core/{probe,probe_http_observation,
  diagnostic_budget}.go`, `observer_publication.go`: required measurement,
  rolling reservation, actual release, publication scope. Это не native/live proof.
- `shared/adaptive/{evidence,evidence_types,evidence_validation,evidence_time,
  budget_types,failover_budget,retry_budget,types,rank}.go`, README/go.mod;
  `docs/adaptive/{API,EVIDENCE,BUDGETS,PLAN,ADP-005-DESIGN}.md`:
  ADP-003/004 **source implemented/unverified**; ADP-005 initially design-only,
  late cut — unconnected source prototype. Exact keys,
  opaque DependencyRef, ClockDomain/TickTime и retained custody уже reuse points;
  no-new-owner. Scoped non-test source search: pure API definitions, platform
  callers ещё не найдены; это согласовано с BUDGETS, не build/link verification.
- Android `OsProtection.kt`, `WorkerRpcLane.kt`: elapsed generation/TTL и pending
  actual return; время не permission. Separate AND-07/11/16 applicability.
- DEPTH-CYCLE-002/own START/README, THEORY17, platform plans и handoffs001/002
  задают baseline/ownership/requirement links. Их статусы не переписаны этим чатом.

`external-inputs.json` содержит10 captures:4 primary articles, author page,
5 repo files. `external/bai-commit.json` и `bai-tree.json` дополнительно фиксируют
GitHub metadata/tree. HEAD был **разрешён один раз**, дальнейшие raw reads только
по exact commit. Files прочитаны, Julia/Go/Android не запускались.

**Source drift:** `drift-at-review.json` cut2026-10-25T19:02:21Z содержит7 изменений
из33; прочитаны addressed line differences всех7. `review-local-inputs.json`
сохраняет7 обновлённых файлов и3 новых: Android VpnSessionRepository/WorkerProcess/
ADR-039. Итого36 unique local paths,43 captures с сохранённым baseline.
Latest-decisive join не подменён прежним terminal описанием; административные
outcomes не отменяют действующий proof и не скрывают failure. Android plan
перешёл AND-11-S2→S3 source/unverified; actual optional OS caller и guards
просмотрены, bounded OS runtime не заявлен. Итоговые hashes — `final-cut.json`.

Late cut19:07…19:08Z: ещё4 docs обновлены под `PlanAutomaticBackground`;
прочитан actual `shared/adaptive/observer.go`: scalar DTO/constant branches,
current scope/clock/count/MaxAge, suppression suggestion без admission/wiring.
`late-local-inputs.json` содержит эти4 docs + source, всего48 local captures/
37 unique paths. Гипотетический split `observer_types.go` не найден — types
реально в observer.go; это не отсутствие capability. Перед интеграцией учитывать
последующий drift, а не превращать addressed hash seal в global PASS.

### 5.3. Что выведено автором R03, а не «найдено в paper»

EVSI в loss convention — явное определение; D1 entropy example, D2 XOR bundle,
D3 deterministic joint dominance proof, D4 lower/upper deadline distinction,
D5 identical-experiment-history proof, D6 common-cause arithmetic, D7 minimax
perfect-information bound, D8 minimax/Bayes objective counterexample, D9 overlap
nontransitivity — собственные конечные ручные выводы REPORT§12. Они условны,
не simulation/experiment и не новые опубликованные теоремы. Не требуют iid для
finite support, но требуют корректной **совместной** модели и действительных
cost/time/safety предпосылок. Все physical costs/likelihood остаются unknown.

Противоположные научные основания сохранены: P11 действительно допускает сложный
joint noise, P12 работает с overlapping regions, P13 различает conditional
dependence при target и independence при root cause. Поэтому вывод не отвергает
готовые active methods вообще. Ограничение — перенос их гарантий в текущие
непроверенные VPN models; отдельного независимого review R03 пока нет.

## 6. R03-004 — causal policy evaluation

2026-10-25/26, окно до2026-10-26T06:30:26Z. Новый scope — не общий warning о
winner-only logs, а условия определения эффекта полной policy, assignment versus
admission и fit/propensity/observation contract.

| ID | Primary / exact version / прочитанный scope | Решение |
|---|---|---|
| P15 | Dudík, Langford, Li, [Doubly Robust Policy Evaluation and Learning](https://icml.cc/2011/papers/554_icmlpaper.pdf), ICML2011,8pages, §2–3 | IPS/DR conditional scores/bias; reference и собственный loss/expectation разбор, не rollout authority |
| P16 | Hadad, Hirshberg, Zhan, Wager, Athey, [arXiv1911.02768v4](https://arxiv.org/html/1911.02768v4),2021-02-12; §2/Theorem2 | Positive probabilities, IID potential-outcome vectors, predictable weighting, infinite sampling/variance/moment conditions; adaptive asymptotic CI не arbitrary drift/anytime guarantee |
| P17 | Jiang, Li, [Doubly Robust Off-policy Value Evaluation for RL](https://proceedings.mlr.press/v48/jiang16.pdf), ICML2016,PMLR48:652–661, §3.3/4 | Horizon1 versus sequential DR, trajectory ratios и model/policy/eval split. Наличие estimator не доказывает VPN MDP/support |
| P18 | Bojinov, Simchi-Levi, Zhao, [Design and Analysis of Switchback Experiments, arXiv2009.00148v4](https://arxiv.org/html/2009.00148v4),2025-09-17; §2/assumptions | Non-anticipating potential outcomes и given finite m-carryover; design reference, не универсальный washout/wait/schedule |
| P19 | Saito, Aihara, Matsutani, Narita, [OBD/Pipeline, arXiv2008.07146v5](https://arxiv.org/html/2008.07146v5),2021-10-26; §2 | Independent product triples/IPS/DR setup; fashion dataset не исследование VPN и не нужная нам загрузка |
| C04 | [st-tech/zr-obp exact8cbd5fa4558b7ad2ba4781546d6604e4cc3e07c4](https://github.com/st-tech/zr-obp/tree/8cbd5fa4558b7ad2ba4781546d6604e4cc3e07c4); pinned estimators.py/meta.py, README, pyproject | `OffPolicyEvaluation.estimate_policy_values→_create_estimator_inputs→estimate_policy_value→_estimate_round_rewards→mean`; IPW ratio/clipping и DR factual residual прочитаны; intervals зовут bootstrap helper. **Reference now/future offline adapt**, не runtime import |
| C04-L | [LICENSE при том же commit](https://github.com/st-tech/zr-obp/blob/8cbd5fa4558b7ad2ba4781546d6604e4cc3e07c4/LICENSE) | Apache-2.0 copyright2020 Saito/Narita/ZOZO. Project manifest0.5.5, Python>=3.7.1,<3.10 и numerical/ML dependencies; транзитивные licenses/compatibility не проверены |
| P20 route only | [Official VW OPE tutorial](https://vowpalwabbit.org/docs/vowpal_wabbit/python/latest/tutorials/off_policy_evaluation.html), read2026-10-25 | Alternative interface reference; exact C++ pin не исследован, comparative superiority/ready drop-in не заявлены |

**Capture и negative retrieval:**21 local inputs в `cycle-004/local-inputs.json`.
External manifest содержит successes **и** errors, а не фиктивные скачанные files.
Из11 первых external requests6 successful; additional3 дали2successes. Public
GitHub page отдельно captured: `currentOid` дал точный C04 commit после timeout
GitHub API. Четыре pinned repo files и public GitHub page captured; Hadad/VW/Jiang/switchback —
прочитаны/captured. ICML2011/OBP paper и pinned LICENSE доступны и прочитаны через
web primary renderer, но прямые filesystem fetch истекли. `external-inputs.json`
и `external-additional-inputs.json` сохраняют ошибки; partial files не proof.
Null API responses/ограничения tags/commits access не интерпретированы как
archived/unlicensed/отсутствующий code. Пакеты/код не выполнялись/не ставились.

**Собственные выводы:** E1 one-step/system occupancy counterexample, E2 sequence
weight arithmetic, E3 same-row DR leakage, E4 support identification bound,
E5 clipping-bias expression, E6 admission denominator reversal — ручные finite
модели REPORT13. Они не эксперимент и не product distributions/числа.
P15–19 assumptions сохранены; границы модели/uncertainty интервалов не спутаны.
Science label R03-08/09 остаётся предложением. Все16 future Q cases NOT RUN.

Local review06:03:30Z:18/21 unchanged,3 Android changes. Relevant differences,
current WorkerRpcLane delivery/ack и AND03-S9 STATE прочитаны;3 refreshed captures
в review-local-inputs.json (24 captures/21 unique). Это не verified lifecycle и
не новая causal dataset. Windows/shared examined source/previous handoffs совпали.

## 7. R03-005 — delayed disclosure / reset-family validity

2026-10-26; начало до06:30:26Z, scientific plan-only.

| ID | Exact primary/source | Прочитанный scope / reuse |
|---|---|---|
| P21 | Howard, Ramdas, McAuliffe, Sekhon, [arXiv1810.08240v9](https://arxiv.org/html/1810.08240v9), **2022-08-06**; journal2021 отдельно | Definition(1), Lemma3, Theorem4/§4.1, §6 running intersection. Uniform event/random index, average conditional mean, predictable inputs, sides accounting; reference, не arbitrary-disclosure validity |
| C01 deeper | [confseq exact5ffe733ca2447a2e28c2c91f3b00086173f2ab2c demo/ate_demo.py](https://raw.githubusercontent.com/gostevehoward/confseq/5ffe733ca2447a2e28c2c91f3b00086173f2ab2c/demo/ate_demo.py) | `estimate_average_treatment_effect→lagged predictions→Xt/St/Vt→gamma_exponential_mixture_bound(coverage_alpha/2)→DataFrame`. Caller/source прочитан web, не выполнен; numerical core/ABI не audited |
| C01-L reused | LICENSE/README при том же pin, captured earlier, copied own cycle005 | MIT2021 confseq developers; exact README C++14/Boost/Python/partialR/early-stage. **Reference/future offline adapt**, без нового runtime dependency |

Новый v9 не назван v5, ранее указанной THEORY17, или датой journal2021.
Собственные F1 fast-success selection, F2 valid-CS prefix postprocessing,
F3 historical-average/current counterexample, F4 family error composition и
F5 sides accounting вручную разобраны в REPORT14. Это не численная validation.
Family allocation — достаточная конструкция, не утверждение new theorem/оптимума.

9 initial local captures: THEORY/window/handoff004, shared evidence fields/join,
Windows HTTP/probe/failover, Android RPC. Exact hashes/time — local-inputs.json.
Два earlier pinned confseq files copied с сохранённым baseline. HTTP fetch paper,
demo и source archive timeout; все3 ошибки в external-inputs.json. P21/demo
прочитаны через primary web renderer; архив/core не выданы за captured/audited.
README install и demo __main__/test_ate/CSV **не выполнялись**; dataset не читался.
Все12 S-cases — future NOT RUN, не tests/fixtures. No private/raw owner data.

## 8. R03-006 — drift/delay/switch-cost decision candidate

2026-10-26. R03-12 — собственный conditional finite-horizon advisor,
не paper theorem.

| ID | Primary / exact date/version / scope | Применение и граница |
|---|---|---|
| P22 | Bifet–Gavaldà, [ADWIN author draft](https://www.cs.upc.edu/~gavalda/papers/adwin06.pdf), **2006-10-17**,17pp, §3.1–3.4/Theorem3.1; отдельная SDM2007 DOI10.1137/1.9781611972771.42 | Independent [0,1] sequence, adaptive windows, per-step false-positive bound. Practical Bernstein/approximation discussion не объявлена точной River guarantee; no-alarm не future drift bound |
| P23 | Garivier–Moulines, [arXiv0805.3415v1](https://arxiv.org/html/0805.3415v1), **2008-05-22**, §1–3/Theorem7/AppendixA | D-/SW-UCB, piecewise-stationarity/breakpoints/window/gap/independent bounded rewards. Прочитанная v1 не переименована ALT2011 proceedings version; не delay/cost-aware VPN guarantee |
| P24 | Dekel, Ding, Koren, Peres, [arXiv1310.2997v2](https://arxiv.org/html/1310.2997v2), **2013-11-19**; [author STOC2014 PDF](https://oferdekel.github.io/pdf/2014DekelDiKoPe.pdf), §1/Theorem1 | Oblivious bounded losses, unit switch cost, best-fixed-action benchmark, minimax lower bound. Не unrestricted dynamic-oracle regret и не РФ field prediction |
| P25 | Wu, Shariff, Lattimore, Szepesvári, [Conservative Bandits](https://proceedings.mlr.press/v48/wu16.pdf), ICML2016/PMLR48:1254–1262, §2–4/Theorem2 | Baseline-relative reward budget; stochastic и adversarial variants. Performance constraint не hard protection/trust/instant delivery guarantee |
| C05 | [River b674b758518341bc5c343ca113f34903ba55020a](https://github.com/online-ml/river/tree/b674b758518341bc5c343ca113f34903ba55020a), manifest0.26.1 | adwin.py→PyO3 binding→adwin.rs insert/detect/evaluate_cut/drop; actual Rust core и wrapper/registration прочитаны, source only |
| C05-L | [LICENSE при том же pin](https://github.com/online-ml/river/blob/b674b758518341bc5c343ca113f34903ba55020a/LICENSE), BSD-3-Clause/copyright2020 river developers | Future adapt/reference с notices; Python>=3.11/NumPy/SciPy/Narwhals/Rust-PyO3-maturin — integration cost, не direct Go/Android import |

River resolved один раз из currentOid public HTML; далее raw files только exact
commit. Python version0.26.1 не смешана с internal Cargo crate0.1.0. Source
ADWIN default clock32 — samples, не seconds; window size/buckets не hard fixed
lifetime memory cap. Constructor/stream inputs нельзя считать validated по одному
наличию API. Copy/port/FFI/parity/runtime/energy measurement не выполнялись.

Cycle006:23local captures с before/copy/after/UTC;12 external manifest captures
(3initial +6literature/tree +3Rust) и отдельный public repo page. Строгое назначение
файлов см. local-inputs/external-initial/external-literature/external-rust.json.
Paper/date/caller scopes в REPORT15; no private owner data. Ошибочная ранняя
поисковая гипотеза COLT для switching-cost/conservative papers исправлена по
самим primary: STOC2014 и ICML2016. Не заимствованы AI/secondary summaries.

Собственные выводы: weighted-mean localization через L*d, drift-age advantage
envelope, quadratic exposure integral, finite endpoint/stationary extrema,
law-of-total-expectation complete-trial bound, B1…B5 и bounded contingent regret
comparison. Это conditional model, не experiment/новые published guarantees.
Физические L/b/H/C/p values неизвестны; все14 future P cases NOT_RUN.

## 9. R03-007 — conditional staged risk / cost tail

2026-10-26; source/web reads09:18…09:23Z, новый срез начат до10:03:26Z cutoff.

| ID | Exact primary / scope | Применение и reuse decision |
|---|---|---|
| P26 | [NIST/SEMATECH Series model §8.1.8.2](https://www.itl.nist.gov/div898/handbook/apr/section1/apr182.htm), official HTML read/capture2026-10-26 | Marginal component product требует independence/first-failure lifetime assumptions. Reference; не перенос independent component model на VPN stages |
| P27 | [NIST exact binomial CI §7.2.4.1](https://www.itl.nist.gov/div898/handbook/prc/section2/prc241.htm), official HTML read/capture2026-10-26; [Dataplot exact-binomial docs](https://itl.nist.gov/div898/software/dataplot/refman2/auxillar/exacbici.htm), doc date2010-10-05 | Binomial inversion/one versus two-sided limits. Reference/manual zero-failure derivation; fixed-sampling assumptions, software/examples не выполнялись/не audited. Dataplot date не новая публикация2026 |
| P28 | [OpenTelemetry Tracing API SetStatus/End](https://opentelemetry.io/docs/specs/otel/trace/api/), official web read2026-10-26 | Unset/Ok/Error и End без завершения children. Vocabulary reference, **не** owner/custody/task outcome protocol. SDK/version/license/import/export не выбраны |
| L-reuse | Existing API1 budget_types/failover_budget/evidence_types, Windows failover/observer_failover, Android WorkerRpcLane | Exact source branches/typed receipts/actual return/delivery ack reused, source unverified; не новый ledger/controller/telemetry system |

Current source trace: `switchChannelCoreFor` selector-only SwitchChannel либо
ErrSelectorSwitchUnavailable→Reload, затем error/rollback/current publication;
`SettleFailover` AwaitReturn/discard/Applied/RolledBack отдельно от ResourcesRetired.
WorkerRpcLane pending→retained delivery→exact Call acknowledge; submit проверяет
оба состояния. Ни source facts, ни существующий control counter не dataset.

Cycle007 local-inputs.json:13 source/doc captures с before/copy/after/UTC.
external-inputs.json:NIST2 successful HTML; OTel filesystem timeout сохранён,
official text доступен web. No code/SDK copied, installer/network export не
запускались. Own manual derivations C1..C5 — conditional chain/telescoping,
event-type union bounds, retry conditioning, finite missing fraction, binomial
zero-failure inequality и heavy-tail counterexample. Не literature theorem
нового алгоритма/не product probabilities.12future R cases NOT_RUN.

Strong opposing result сохранён: direct full-trial outcome часто проще/лучше
staged product; больше stage schemas не создаёт observations/statistical power.
Past confidence не predictive/stability guarantee; actual mean tail неизвестен.

## 10. R03-008 — producer intervals / sequential validity / unreliable feedback

2026-10-26. REPORT §17, R03-008; R03-15/16 — **READY_FOR_REVIEW**.
Новое: interval-valued/corruption envelope поверх registered cohort и
conservative anytime bound с exact target; это не переименование finite
binary identification из R03-007 или delayed-prefix результата R03-005.

| ID | Primary source / exact version | Прочитанная область и предел |
|---|---|---|
| P29 | [Waudby-Smith/Ramdas2010.09686v7](https://arxiv.org/html/2010.09686v7),25августа2022; journal[DOI10.1093/jrsssb/qkad009](https://doi.org/10.1093/jrsssb/qkad009),2024 issue | §2,Prop1,Thm2/3,constant conditional mean/predictable bets. v5 прочитан сначала, final assumptions сверены по v7; не считать2024paper новым2026result. Journal page fetch redirect blocked, arXiv primary HTML доступен/captured |
| P30 | [Wang/Ramdas AISTATS2023/PMLR206:9662–9679](https://proceedings.mlr.press/v206/wang23p.html),[PDF](https://proceedings.mlr.press/v206/wang23p/wang23p.pdf) | Lemma3/Thm1, after-Lemma conditional-Q relaxation, §4.1 TV versus replacement. Supplied clean moment/TV ball не arbitrary false-label budget и не unknown drift.18-page primary PDF captured; no examples executed |
| P20-reuse | [Howard etal1810.08240v9](https://arxiv.org/html/1810.08240v9),6августа2022 | Thm4 avg conditional expectation/bounded support/predictable prediction. Previously005; new17.4 is own elementary conditional-MGF+union-bound envelope, не borrowed optimality claim |
| C29 | [confseq pin5ffe733ca2447a2e28c2c91f3b00086173f2ab2c](https://github.com/gostevehoward/confseq/tree/5ffe733ca2447a2e28c2c91f3b00086173f2ab2c),setup0.0.11,MIT2021confseq developers | New deeper read predmix.py/betting_strategies.py/misc.py/setup.py/LICENSE. Caller twosided→lower→generic weighted accumulation, α/2; lagged EmpBernλ. Not runtime/C++core/ABI/parity audit, direct adapter absent |
| C30 | [HJ-Wg/catoni-confseq pin381d7fb17d2af5f4cc78d422c92900f2b1d5d1c4](https://github.com/HJ-Wg/catoni-confseq/tree/381d7fb17d2af5f4cc78d422c92900f2b1d5d1c4),untagged, GPL-3.0 | repo currentOid resolved11:17:32Z; demo→RCS_Generator→observe→find_zero_decreasing→get_CS. Constructor side normalization95/effective_alpha115 and per-tail roots165…175 reviewed. Source split mismatch only, no empirical miscoverage/security claim. Growing arrays/history/root work, no client runtime import |

Cycle008 artifacts:17local captures before/copy/after consistent;5confseq files,
5catoni files, root HTML/currentOid,2primary paper captures. `local-inputs.json`,
`external-inputs.json`, `catoni-pin.json`, `catoni-inputs.json`, `paper-inputs.json`
carry paths/UTC/SHA256. READMEs/docstrings were evidence, no command execution.

Manual U1…U7: interval/corruption width; selection; common latent factor;
indistinguishable probe/target worlds; measurement interference; invalid moving-
target intersection; exact string/numeric alpha branch.18future M cases NOT_RUN.
Known source wrapper licensing/model/computation costs retained; external code
не копировался в product, dependencies/manifests не менялись. No field inputs,
owner logs/secrets/physical probabilities/real Γ/ε/L/cost acquired.

## 11. R03-009 — регистрация попыток: источники

Дата среза: **26 октября 2026**. Primary русскоязычная база: просмотр
web-страниц каталогов и материалов; raw byte equality не заявлена,
тексты работ не копировались.

- Ширяев А.Н. **«Статистический последовательный анализ. Оптимальные правила
  остановки»**. М.: Наука, 1976 (переработанное 2-е изд.; 1-е — 1969).
  Библиографическая запись: [istina.msu.ru](https://istina.ficp.ac.ru/publications/book/2978524).
  Байесовский/минимаксный framework оптимальной остановки; гарантии определены
  относительно априорно фиксированного класса решающих правил.
- Вальд А. **«Последовательный анализ»**, пер. с англ. М.: Физматгиз, 1960.
  SPRT: границы A/B и ошибки I/II рода задаются до испытаний.
- Воронцов К.В. комбинаторная теория переобучения: [«Комбинаторный подход к
  оценке качества обучаемых алгоритмов» (2004)](http://www.ccas.ru/frc/papers/voron04mpc.pdf);
  докторская диссертация [«Комбинаторная теория надёжности обучения по
  прецедентам» (2010)](https://www.dissercat.com/content/kombinatornaya-teoriya-nadezhnosti-obucheniya-po-pretsedentam);
  лекции [«Комбинаторная теория переобучения»](https://machinelearning.ru/wiki/images/b/bd/Voron-ML-overfitting.pdf).
  Вероятность переобучения растёт с числом рассмотренных гипотез; связность и
  расслоение семейства уточняют границу — структурный аналог зависимости
  кандидатов в union-границе.
- Dwork, Feldman, Hardt, Pitassi, Reingold, Roth. **"The reusable holdout:
  Preserving validity in adaptive data analysis"**, Science 349(6248), 2015 —
  адаптивный выбор запросов требует бюджета/шума; аннотация просмотрена.

Reuse из прежних срезов: anytime/confseq pins (§10, MIT 5ffe733…),
catoni (GPL-3.0 381d7fb…), River b674b758…. Контрпример §18.2 — ручная
модель без исполнения. Все future cases **NOT_RUN**.
На §18.2 распространяется [поправка](../reviews/NINE-2026-10-27-R03-17-ERRATUM.md).
