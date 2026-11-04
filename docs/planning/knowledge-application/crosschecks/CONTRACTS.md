# Общие контракты: применение знаний без взаимной блокировки платформ

## 1. Состояние существующих компонентов

**Срез 28 октября 2026.** Переносимый adaptive API существует в исходниках,
но не подключён к Windows/Android. Переносимый trust API для Android не
опубликован в `shared/`: имеются Windows delivery/metadata и обязательные
Android интерфейсы без production driver/provider. Новая общая DTO-платформа
не нужна. Минимальное направление — закрепить семантику существующих границ,
реализовать адаптеры у владельцев платформ и отдельно пройти приёмку.

- `shared/adaptive/go.mod` — standalone Go1.26/std-only; `Eligible`, `Rank`,
  `Prefer`, budget functions, `ComposeEvidence` и background-suppression prototype.
  API1 **draft/unverified**, не frozen release и не разрешение выполнить action.
- Windows использует `scoring.go`, `failover.go`, `observer_publication.go`,
  `autoretry.go`, `backend/delivery`/`metadata`. В `windows/go.mod` нет adaptive
  dependency; текущие прочитанные callers не импортируют его. Существующая
  policy остаётся единственной production-цепочкой до cutover.
- Android `ProfileTrustDriver.inspect/verifyForApply` и `RuntimeSourceProvider.create`
  уже задают нужную границу; production implementations не обнаружены в
  проверенных profile/vpn/corebridge source. Структурный composer явно содержит
  `signedAuthorityVerified=false`; его нельзя подключить как accepted provider.
- `WorkerRpcLane`, `RetainedPreparedSource`, `ProfileImportCoordinator`,
  `ProfileClock`, `PreparedRuntime` уже выражают retention, CAS и раздельные
  deadlines. Их наличие не доказывает native/ABI/OS работу, но исключает
  необходимость заново проектировать эти сущности ради общего названия.

Основные существующие адресаты: **WIN-SDK-001-T/A, ADP-002…008,
TH-02/03/04/06/08, AND-04-S4b T01…14, AND-07 M01…06, AND-01/02/05/06/11/16**.
Новые task IDs, роли и продуктовые статусы не вводятся.

## 2. Знание → фактический caller → минимальная дельта

| Семейство знания / источник | Current code и план | Разница и disposition |
|---|---|---|
| Applied context и полная цепочка сортировок: [AD-01/02](../../../theory/adaptation/POLICY-AND-EVIDENCE.md#source-17-1), API1 — `docs/adaptive/API.md` | Windows `scoring.go:128–228`, `failover.go:427–486`; shared `rank.go:13–90`; ADP-002/006/007, WIN-SDK-001-A | **ADAPT:** один owner snapshot descriptor/health/D2/recent/dependency перед pure call; сохранить score→class→recent stages и current gate. Не объявлять один scalar score всем controller |
| Unknown dependence — не независимость: [trust/interval contracts](../../../theory/architecture/TRUST-CONTINUITY-AND-AGREEMENT.md), [AD-02](../../../theory/adaptation/POLICY-AND-EVIDENCE.md#source-17-2) | `render/dependency_graph.go:186–281`, `fdmodel.go:97–104`; `adaptive/types.go:109–125`; TH-03-DEPENDENCIES | **ALREADY SOURCE-IMPLEMENTED + ADAPT:** configured RankHint/Coverage существуют; проекция Available/AgainstID должна сохранять known0/unknown. Не превращать label diversity в измеренную вероятность |
| Registration/cohort/late outcomes: [R28-Z03 handoff](../../../theory/research/cycles/2026-10-28-contracts/Z03/HANDOFF.md), [intervals §1–3](../../../theory/adaptation/INTERVALS-AND-POLICY-EVALUATION.md) | `probe_observation.go:84–128`, `observer_publication.go:41–121`; `evidence_types.go:162–205`; TH-02/06, ADP-004/008, AND-07/16 | **CHANGE REQUIRED только для scientific consumer:** sequence в памяти и latest health projection не полный cohort; нужен controlled registration/summary. Не менять safety publication, не внедрять БД по одному research proposal |
| Capability freshness и statistical loss — разные intervals: [C03 B](../../../theory/research/cycles/2026-10-28-contracts/C03/HANDOFF.md#r28-c03-b--envelope-должна-соответствовать-kernel-и-postprocessing) | `adaptive/evidence.go`, `EvidenceWindow`, `ComposeEvidence`; ADP-004/008 | **DEFER numerical advisor / KEEP coverage API:** CommonWindow не confidence interval; для future comparison нужны loss/model/kernel/envelope/bridge semantics. Расширять текущий API только после выбранного consumer, без новой универсальной DTO |
| Attempt outcome/current publication/retirement отдельно: [AD-06](../../../theory/adaptation/POLICY-AND-EVIDENCE.md#source-17-6) | `observer_failover.go:35–113`, `adaptive/failover_budget.go:100–166`, Android `WorkerRpcLane:53–105`; ADP-003/006, TH-04, AND-02/11 | **ADAPT:** preserve origin receipt и result session отдельно; settle после actual return, resource obligation до retirement. Logical discard не health success; два pure proposals не две reservations |
| Три режима failure limit и разные platform retries: BUDGETS — `docs/adaptive/BUDGETS.md` | Windows `failover.go`, `autoretry.go:85–186`; Android repository:426–470; ADP-003/007 | **KEEP PLATFORM DIFFERENCES:** default2, legacy candidates и observed candidates не единый cap; Android actual3/60s не Windows30…600s. Freeze semantic receipt, не общий timer algorithm |
| Persistable TUF progress ≠ SDK mutation: [TUF chapter](../../../theory/architecture/SAFETY-AND-TRUST.md#tuf-progress-and-reset-2026-10-27), [C05 C](../../../theory/research/cycles/2026-10-28-contracts/C05/HANDOFF.md) | `delivery/client.go:150–172`, `retained_state.go:16–79`, `update_delivery.go:86–106`, `delivery_state.go:41–119`; TH-03-TUF-INTEGRATION/FR-008/011, T05…11 | **NEEDS REVIEW + узкая eligibility дельта:** original bytes/prefix CAS уже есть, но pointer matching не completed-step eligibility. Новый rejected expired record не accepted floor; prior accepted expired predecessor сохраняется. Не запрещать весь progress при общем error |
| ResetAllowed/quorum exclusion/compromise recovery разные predicates: [Z07](../../../theory/research/cycles/2026-10-28-contracts/Z07/HANDOFF.md), [trust continuity §1–3](../../../theory/architecture/TRUST-CONTINUITY-AND-AGREEMENT.md) | `retained_inputs.go:61–98`, `retained_state.go:25–65`, `timestampContinuity`; TH-03, T05…11 | **ALREADY SOURCE GUARDS + NEEDS POLICY REVIEW:** intermediate root path и atomic root/cache image существуют; нельзя объявлять обязательным новое basis field. Partial rotations/overlap/reuse требуют выбранного supported contract, не любого key-set diff |
| Portable trust source и accepted renderer отсутствуют: T01…14 — `docs/android/CHANGE-REQUEST-TRUST-001.md` | `ProfileTrustDriver.kt:121–166`, `ConnectionState.kt:38–65`; `delivery/workspace.go`, `workspace_other.go`; AND-04-S4b, WIN-SDK-001-T | **CONTRACT-FIRST:** Windows delivery — reusable semantics, но window module/platform workspace не Android diskless SDK. Нужен принятый portable entrypoint с bounded bytes/storage-clock-progress, не второй Go runtime/Кotlin verifier |
| Authority deadline, preparation TTL и clock observation отдельно: ADAPTER-MATRIX — `docs/adaptive/ADAPTER-MATRIX.md`, [release evidence](../../../theory/architecture/RELEASE-EVIDENCE-BOUNDARIES.md) | `SessionValidity.Remaining`, `ProfileClock:44–104`, `PreparedRuntime:14–38`, `RetainedPreparedSource:60–97`; T09/10/13, M04, ADP-006 | **ALREADY SOURCE-IMPLEMENTED + ADAPT:** exclusive minimum authority и local freshness раздельны; Remaining≤1s — observation ceiling, не expiry. ms→ns helper не превращает elapsed в UTC |
| UID capture/exemption/mapper/current app lease отдельно: [UID chapter](../../../theory/android/UID-ROUTING-AND-OWNERSHIP.md), [Z08](../../../theory/research/cycles/2026-10-28-contracts/Z08/HANDOFF.md) | `LibboxPlatform.kt:94–127`, `NativeCallbackGate`, structural composer app bindings; AND-01/02/06/07, ADP-004 ApplicationContextRef | **SOURCE GUARD EXISTS + DEFER capability expansion:** rawUID mapped owner не full identity proof; current strict-owner bypass stock tuple cache уже candidate, не новый defect. API35 sandbox mapping/new fields требуют agreed schema и corresponding whole AAR, не offset fallback |
| Mapping/first-execution/OS receipt ≠ artifact approval: [release §2–4](../../../theory/architecture/RELEASE-EVIDENCE-BOUNDARIES.md) | Windows private delivery.Prepared/proof.Check; Android artifact gate/current STATE; TH-08/COM-14, AND-01/14 | **SEPARATE PLATFORM GATES:** TUF-proof, OS signing, staged file, installed set и running incarnation — разные scopes. Не блокировать весь adaptive freeze ожиданием общего release attestor |

Read-only crosscheck не меняет решение scientific acceptance. R28 author recommendations
остаются **READY_FOR_REVIEW**, пока руководитель не принял конкретную область.
Накопленные accepted математические contracts применяются только при их assumptions.

## 3. Несовместимые definitions и frozen handoff minima

Ниже «freeze» означает согласовать существующие names/types/semantics по
versioned source revision, а не объявить API1 уже frozen или создать новый DTO.

### 3.1 Контракт ranking (ADP-002/006; Windows adapter)

**Вход:** `Descriptor/Facts/AttemptHint`, applied `Scope`, explicit `Now`.
Projection — один owned snapshot; pointer/config/profile/dependency source
сняты с той же applied сессии. `Dependency.Rank=RankHint`, `Coverage` и
`AgainstID=FailedID`; Available относится к known comparison, не достижимости.
Если сравнение unknown: Available=false, rank/coverage0; известное совпадение
может иметь rank0 и положительную coverage. Основание presence сохраняется.

**Выход:** `RankResult.AfterScore/AfterClass/Order/Trace`, fixed reason,
RequiresPlatformGate. Только `Order` — final ordered list, AfterScore не winner.
Startup использует Eligible.IDs, observed failover — ReachableIDs. Platform
повторяет current intent/session/protection gate и сам reserve/applies.

**Compatibility:** shared identifier `[A-Za-z0-9_.-]` шире
`metadata.ValidChannelID` lowercase/dash/underscore/non-device-name. Модуль
не заменяет metadata ingress validator; не нормализовать signed IDs/aliases.
Bounds64 совпадают с current profile channel cap, но oversize/duplicates теперь
boundary errors; legacy malformed input не гарантированно имеет прежний fallback.
Valid-input ordering parity и malformed disposition согласовываются отдельно.
Сохраняются dormant legacy class tiers; их удаление не migration prerequisite.

**Наименьший deliverable:** Windows-owned `applied values → existing API1`
adapter и removal/rollback plan одного caller. Не добавлять `shared/DTO2`.

### 3.2 Контракт evidence coverage (ADP-004/006; обе платформы)

Использовать `EvidenceIdentity/Capability/Observation/OperationCustody`.
`Kind,Family,Workload,Method,Route,EndpointRef` — exact keys, не wildcard.
Known zero отличается от absent и от source sentinel0. ApplicationContextRequired
задаёт current policy owner, не UI; app immutable policy digest не заменяет
actual original lease/current observed app context.

`EvidenceWindow` и `CommonWindow` — интервалы **пригодности во времени**.
`AllRequirementsCovered` — coverage, не authenticity/protected Ready/statistical
confidence. `PlatformAccepted` — trusted owner projection, не bool из public JSON.
Unknown fields остаются unknown; не копировать StartedAt в missing AdmittedAt,
не обновлять old receipt completion временем getter.

Windows обычный HTTP/DoH trace не сообщает actual IP family; не заполнять
FamilyIPv4 по A-question/URL. Android capture IPv4/IPv6 — не receipt external
egress/business workload. Narrow native measurement дополняет только own key.
Модель scope должна учитывать UID/capture/exemption/profile для relevant Android
proof; сейчас её нельзя спрятать под one bool `VpnService established`.

**Наименьший deliverable:** короткий field mapping+reason matrix к существующему
API1 и explicit unavailable mappings. Full protected producer отдельный stage;
согласование coverage types не требует уже завершённого all-family измерителя.

### 3.3 Контракт attempt/settlement (ADP-003/006; platform-specific)

Сохранить `Expected`, monotonic origin generation, `DecisionCurrent`,
`PublishedCurrent`, `RetainedRunning`, `OwnerReturned`, `ResourcesRetired`.
Matching origin attempt отличается от нового applied session. Resource return
должен относиться к тому owner, чьё обязательство удерживается.

**Нельзя переносить Windows normalization на Android:** Android release требует
оба RPC returns, producer return, Stub reply marshalling и exact old-worker death
по своему scope. WorkerRpcLane.takeCompleted удерживает delivery до acknowledge;
возврат transport message не resource retirement. Эти существующие guards
предшествуют любому portable bookkeeping effect.

Windows legacy obsolete-while-desired prebranch может оставить attempt без reset;
shared normalized SettleFailover default discard сбрасывает matching count.
Сохранить wrapper prebranches до normalized call. Candidate failed-work hint
при legacy verified rollback и observed rollback различается. Нельзя заменить
это общим outcome=success или всегда записывать health.FAIL.

`RetrySnapshot.ScheduleGeneration` требует monotonic owner-issued token;
current Windows timer передаёт `attempt` и сбрасывает индекс в0. Новый field
**не заполняется** attempt index. Старый late callback после reset должен быть
отдельным future negative case, не утверждением already-runtime race.

**Наименьший deliverable:** отдельные Windows adapter changes для generation/
wrapper gates; Android сохраняет actor budget3/60s и не обязан принимать
Windows schedule function. Common семантика return/cancel/retirement не требует
одинаковой реализации таймеров и не передаёт lifecycle ownership SDK.

### 3.4 Контракт trust progress / accepted snapshot / first enrollment

Повторно использовать existing Windows `delivery.State/Progress/Prepared`
semantics и Android `ProfileTrustWork/Review/AcceptedCheckpoint` границы;
не публиковать ещё одно промежуточное «trusted DTO» с forgeable flags.

**Минимум для ответа на T05–11 и WIN-SDK-001-T:** exact original bytes/source
identity; один workflow reference time; допустимость каждого predecessor step;
role/reset context; error/cancel progress; cleanup/unknown source outcome;
полный artifact proof отдельно от restriction-only progress. Initial failed
root enrollment не persistable prefix. Established cached predecessor и новый
incoming expired object не одно и то же, даже если SDK pointer ненулевой.

Точный current разрыв: `PrepareArtifacts` после Refresh вызывает retainedState;
`boundMetadata` выбирает unique original bytes по matching parsed object, но
проверка не сообщает completed timestamp/snapshot workflow acceptance.
`update_delivery.go` при retain и Progress!=nil вызывает actual prefix transaction
до возврата deliveryErr. Source/CAS/vault/anchor/clock guards уже имеются;
они не являются provenance-step eligibility. Узкую правку требуется рассмотреть
на этом seam, не переписывать весь updater и не suppress весь progress при error.

**Opposing guard:** `CheckDeliveryProof` допускает exact original source либо
точный verified next-source digest, поэтому сохранение именно этого prefix
не обязательно инвалидирует его proof. Нельзя заменить проверку равенством
только старому source и сломать correct retain→apply путь. Возврат cleanup error
оставляет Progress без proofs; это уже useful separation, не «всё accepted».

ResetAllowed supported transitions согласовать до claiming automatic recovery:
current intermediate-root OR видит S→S′→S в одном вызове; threshold/alias alone
не erase floors. Old envelope acceptance новым quorum и compromised successor
keys — другие predicates. Atomic root+empty T/S image может исключить ambiguity
без new basis field; arbitrary rotations не становятся supported по слову reset.
TUF cache reset не reset application profile epochs/cumulative revocations.

### 3.5 Контракт portable trust и accepted measurement plan

`windows/backend/delivery` — часть Windows module с workspace/path/OS adapters;
`workspace_other.go` использует MkdirTemp/files, не diskless Android SDK.
Отсутствие Windows build tag само не означает pure portable bytes operation.
Reuse полного go-tuf workflow сохраняется; portable boundary должен явно решить
original-byte progress/storage/clock/cancel без ambient capabilities.

**Freeze минимум, не вся реализация:** версия/capability, named callable path,
borrowed input lifetime и owned outputs; T01…14 supported/unsupported matrix;
full uint64 representation; exclusive minimum deadline; lower-workflow coverage;
один shared module, включаемый Android в существующий whole libbox.
`ProfileTrustDriver` и JNI adapter остаются Android-owned; не импортировать
Windows module в Android и не строить второй conflicting AAR/Go runtime.

M01…06 добавляет accepted measurement extraction из **того же** accepted head.
Если accepted T-output уже даёт endpoint/policy/budget/capability, достаточно
точного маршрута; новый `MeasurementDTO`/public method не prerequisite.
User/network/app/native ownership поля даёт платформа, не portable verifier.
Не подключать stock helper с direct fallback ради заполнения production gap.

### 3.6 Контракт времени и чисел (общие semantics, не общий clock)

- Current `Rank/Failover/Retry` принимают Go `time.Time`; `ComposeEvidence`
  Known/Nanos/ClockDomain. Baseline structs нельзя serialize через epoch-ms и
  объявить одинаковыми с elapsedRealtime. Android сначала использует evidence
  mapping, а native ranking wire ABI выбирается только при actual consumer.
- `TickTimeFromMilliseconds` — checked units conversion с signed-native ceiling;
  boot/process/domain binding отдельно. Source0 sentinel остаётся absent,
  helper Known0 не разрешает фиктивную observation.
- Android ProfileClock actual projection уже умеет переносить **verified**
  exclusive epoch deadline в elapsed по original sample; template preparation
  TTL не заменяет signed authority. PreparedRuntime/WorkerProtocol6 требуют
  оба поля; older worker не adopt, неизвестное не default authority infinity.
- Windows `SessionValidity.Remaining` потолок1s — частота clock observation,
  не фактический signed expiry для evidence HardNotAfter.
- Profile epoch/counter UInt64 нельзя truncate в Kotlin Long/Double. Existing
  ProfileTrustReview использует полный bounded identity string; новый DTO с
  numeric Long не cheapest sufficient replacement.
- Raw bytes hash TUF target, canonical application content hash и sealed local
  checkpoint hash — разные named identities. `Envelope.ContentSHA256` исключает
  signature; не подменяет raw delivery target SHA. Version/CAS/semantic identity
  не interchangeable только потому, что все выглядят как digest строки.

### 3.7 Scientific summary / prospective bridge — отдельный необязательный этап

Current API1 **не содержит** estimator/complete roster/loss envelope/kernel/
alpha-family/prospective bridge. Это ожидаемая граница, а не missing field defect
у coverage composer. Достаточный owner summary может храниться вне pure call;
closed process-local cohort не требует durable БД. Если выбрана restart-continuous
scientific семья, нужен separate retained registration/accountant lineage.

Для каждого будущего loss comparison сначала назвать task/policy/horizon/unit,
allowed outcome/censoring/membership, same-unit transition costs, completeness,
joint label envelope и target. Accepted lifetime FWER доказательство применимо
только при actual conditional validity; hash-chain/AES/CAS её не создают.

C03 `EvidenceForDecision`, KernelRef/EnvelopeKind/TransformRef — **proposal** для
семантического handoff, не основание создать новый общий DTO сегодня. Его
weighted-kernel counterexample полезен как future contract negative case:
current Γ_n для ordinary sum не обязательно ограничивает weighted error;
порядок correction/intersection и changing target должен совпадать с моделью.
Не внедрять confseq, memberlist, Envoy или β/B/Γ defaults ради заполнения полей.
Cheapest sufficient solution до модели: deterministic baseline+typed unknown,
finite cohort bounds при действительно известном denominator.

## 4. Последовательность изменений и независимый DAG

**Легенда:** `→` — входной контракт/артефакт конкретного deliverable, не ожидание
всего плана чужой платформы. Контракт согласуется по saved specification;
реализация и разрешённая приёмка — следующие gates. Обозначения S0…S5 ниже —
локальные стадии этой карты, не новые ID задач и не правка реестров владельцев.
Ни один этап не выполняется этим документом.

```text
                  согласованные существующие semantics/version/caps
                   (context, authority, outcome, return, clocks)
                      /                 |                 \
     Windows applied adapter      portable trust seam     Android local owner/
     + API1 valid-input plan      + progress/reset scope   clock/app/native guards
              |                    /              \                |
       Windows one-caller    Windows delivery     Android ProfileTrustDriver
       migration gates       eligibility adapter + accepted renderer/whole AAR
              |                     |                        |
       existing Manager      own fixed4/CAS             accepted M01…06 plan
       and live publication  + release gates                 |
              |                                          Android scoped producer
              +--------------------+-------------------------+
                                   |
                     separate approved producer/cohort model
                     + future comparison/bridge (ADP-008)
```

Нижний узел — **не AND-барьер двух платформ**. Оценка Windows может начинаться
после готовности её собственного producer и согласованного протокола измерений,
не ожидая Android; для Android действует симметричное правило. Общие definitions
и сценарный протокол нужны для сопоставимости результатов, но завершение
реализации другой платформы не является входным условием локальной приёмки.

| Stage / priority | Independent deliverable и owner | Входная зависимость / критерий завершения |
|---|---|---|
| S0 / P0-contract | Совместимость и разделение existing scopes — руководитель + существующие владельцы компонентов | Freeze minimum §§3.1–3.6, поддержанные T/M поля, unknown и version behavior. Не требует all-native/live или завершения чужого приложения |
| S1-W / P0 | Windows узкая progress eligibility/change review | Existing TUF step/source/cached-vs-incoming contract; source/CAS/fixed4 guards остаются. Windows не ждёт Android; closed supported rotation set лучше universal reset обещания |
| S1-A / P0 | Android callback/source/clock/retirement corrections и artifact seam по AND-01/02/11 | Local guard proof и текущая adapted whole-core source revision. Не ждёт shared adaptive integration; runtime Connect не включается без AND-04 |
| S2-D / P1 | Adaptive owner закрепляет API1 функции/границы и version/capabilities | Caller mapping ADP-006; не full probabilistic advisor. Windows ranking migration может идти отдельно от evidence/failover/retry modules |
| S2-T / P0 | Portable trust owner публикует concrete callable path и T01…14/M01…06 capabilities | Existing WIN-SDK-001-T + Android requests; wrapper можно менять минимально. Не просто новый DTO/code copy под shared/trust, не dependency на Windows updater завершение |
| S3-W / P1 | Windows single consumer values→API1 adapter + expected removal/rollback | Accepted S2-D contract и independently allowed parity gates. Сохранить local P0/current gate; no second autonomous policy |
| S3-A / P0 | Android driver/JNI/accepted source binding | Accepted S2-T path/version + local S1-A; own whole-AAR/ABI gate. Согласование входов возможно заранее, live execution только после platform acceptance |
| S4-A / P1 | Android native scoped measurement producer | M-output от same accepted source + existing native owner/resource lane; AND-07. Не требует Windows full producer/parity или API1 ranking; если T-output покрывает M, no new method |
| S4-W / P1 | Windows evidence mapping/optional pressure read | ADP-004 exact capability/time field mapping и own source availability. Не заменяет mandatory proof или quorum, не ждёт Android loss measurements |
| S5 / P2 | Full-policy/cohort evaluation и один justified optional change | Own meaningful task outcomes, conserved cohort/error budgets и separate bridge; ADP-008/TH-06/AND-16. Независимые platform results, общий сценарный протокол; field позже explicit permission |

**Что не является hard dependency:** Windows Release/Installer completion на
Android driver; Android all-app UID mapping на Windows ranking; comparative
success или новый statistical estimator на P0 Stop/revocation; все девять TH на
freeze минимальных semantic handoffs. Конкретный executable cutover требует
его local gates — общий contract agreement не подтверждает их выполнение.

## 5. Opposing guards, старые клиенты и лицензия/цена

1. **Missing field не автоматически new layer.** Android already has two
   deadlines/custody, Windows already has source/next proof binding, byte clones
   и root path inspection. Сначала mapping существующих fields; duplicate DTO
   увеличит translation risk и не даст authority.
2. **Ключевой no-bridge противовес:** current recovery остаётся deterministic,
   поэтому missing statistical model не блокирует обычный safety путь. Не
   изобретать quantified95% readiness или force wait на roster completion.
3. **Old consumer compatibility:** API1 draft strict validation меняет обработку
   duplicate/nonfinite/oversize/zero-context. Нужен documented error mapping:
   no usable suggestion, not synthetic success/direct; preserve old baseline
   при условно valid independent local flow, не fallback на malformed input.
4. **Старая wire version:** WorkerProtocol6 и native marker/sourceRevision
   не взаимозаменяемы. Freeze версия функций/ABI в том же whole libbox;
   unknown version отказ, не игнорирование authority lifetime поля. Profile
   public schema1/2 и uint64→string mapping сохраняются отдельно от API1 version.
5. **Expiry boundaries:** existing app guards now>=deadline отвергают;
   verifier fixed-reference acceptance и fresh commit/current checks раздельны.
   Отложенный return не получает deadline from now. Cleanup обязан иметь свой
   permit, чтобы expired authority не запретила обязательное освобождение.
6. **Trust agreement:** map quorum/length/hash agreement не выдаёт policy
   authority/custom purpose или независимые custodians. Multirepo deployment
   и mandatory TPM/biometric/cached status verifier здесь не необходимы;
   standard existing TUF/native store reuse дешевле нового protocol.
7. **Licences:** existing go-tuf2.4.2 — Apache-2.0 по сохранённому ADR/LICENSE;
   bbolt pin — MIT, confseq0.0.11 MIT как offline reference. Android исходники
   и whole sing-box adaptation GPL-3.0-or-later. Эти лицензии не переходят на
   весь общий SDK автоматически; shared/adaptive отдельного LICENSE в
   проверенном дереве не имеет — release license declaration отдельный gate,
   а не invented MIT/std-only license. Transitive closure/NOTICE/binary SBOM
   остаются собственными platform deliverables.
8. **Стоимость:** сериализация/копии/history caps, JNI/Gomobile ABI, context
   projection, actual owner retention и parity. Нет измеренных часов внедрения,
   RAM/CPU/энергии/денежной экономии. Freeze контракт не требует новых daemons,
   shared scheduler, второй health database или telemetry stack.

## 6. Приёмка и открытые вопросы

Все критерии **NOT_RUN**; документ не добавляет fixtures и не разрешает execution.

| Gate / existing ID | Проверяемая будущая область |
|---|---|
| ADP-002/006/007, WIN-SDK-001-A | Full ordering stages/ties/missing descriptors, strict ID ingress, known0/unknown dependencies, old epoch and one coherent snapshot; single-caller migration/rollback |
| ADP-003/007, TH-04 | Legacy/observed failure limits, actual return versus retirement, late retry callback after reset, monotonic schedule token, Expected/CAS race and preserved wrapper prebranches |
| ADP-004/006, TH-02, AND-07 | Exact family/method/route/workload/endpoint, clock unit/domain/sentinel, partial scope, stale narrow success and expired original hard bound, app-context requirement |
| TH-03-TUF-INTEGRATION, FR-008/011, T05…11 | New rejected expired timestamp versus accepted expired predecessor; reset path/continuation; prefix+error ownership; cleanup proof suppression; source→exact verified-next binding, no artifact on incomplete batch |
| AND-04-S4b T01…14 | Owned bytes/no ambient capabilities, synchronous actual return, delivery-only progress and generation+1/+2, independent approval, uint64 preservation, typed errors, accepted-only renderer |
| AND-01/02/11/14, TH-08 | Whole module/ABI/AAR/EXE identity, dependency/NOTICE, first-execution boundary, native callback+marshalling cleanup, install/signing/candidate identity independently scoped |
| M01…06, AND-05/06/07 | Same accepted head measurement plan, bound endpoint permission, original authority deadline, rawUID/capture/exemption/family mapping, no direct helper fallback |
| ADP-008, TH-06, AND-16 | Registered complete cohort, cancelled/missing/retained outcomes, joint β+α/bridge bounds, kernel/postprocessing agreement, no physical cap from α, no model from hash-chain |

**Нерешённые вопросы контрактов:**

- Какая concrete portable callable path удовлетворяет T01…14 без Windows module?
  Existing driver contract сам не answer. Нужна версия и supported matrix,
  не статус «shared/trust pending» как причина остановить локальное развитие.
- Какая completed-step eligibility определяет persistable new T/S при SDK
  error? Existing guards source-level проверены, execution ещё отсутствует.
  Не выдавать весь R28-C05-C/Z07-A…C за accepted requirement.
- Какие rotations гарантированы publisher workflow? Universal ResetAllowed
  overlap/threshold/reuse сейчас не freeze без этого выбора; atomic image может
  заменить дополнительное field, его all-writers guarantee — отдельный gate.
- Имеет ли API1 release license/version и семантически согласованный Windows
  adapter? Отсутствие подключения фактически подтверждено, не remediation
  request массового dependency upgrade.
- Какой первый meaningful task/unit/outcome нужен comparative stage? Полный
  borrowed native result/HTTP200 не user task. Если current T API уже покрывает
  M02/M05, договориться о named extraction, а не втором API.

## 7. Источники и предел crosscheck

Использованы **93 выбранные frozen knowledge units** из initial inventory,
включая ранние AD-01…06/R05-001 и accepted NINE-R03-010-A, актуальные тематические
главы и C03/C05/Z03/Z07/Z08 handoffs. Они read-only сопоставлены с current source;
весь корпус2698 units не объявлен прочитанным. Exact per-unit IDs/source hashes,
read/disposition/code/plan refs и remaining unreviewed перечислены в private
coverage Z03.json; principal ownership знаний не переassign'ено.

Late inventory10:21:14Z: **16 units** Z01 STATUS, Z08 SOURCES/STATUS прочитаны
из frozen late copies. Они уточняют evidence/read-depth/caller drift, не новая
research acquisition. Z01 current renderer real ECH не включает — не dependency
общего SDK scope этого документа. Z08 strict-owner candidate уже существует:
stock-cache counterexample не current unconditional bug, raw/attributed UID
mapping и соответствующий AAR остаются отдельными согласуемыми boundaries.

**Граница актуальности:** frozen корпус проверен по hashes; current code
закреплён именованными snapshots, а не Git HEAD всего untracked дерева.
В private source cut **2026-10-28T11:44:00Z** перечислены 74 исходных входа
и отдельно изменения после их содержательного просмотра.

В просмотренные изменения вошли Windows V2-182/183: coherent initial read
fence в `retainDeliveryProgress`, включая no-op, и отдельный
`retainDeliveryProgressWithOutcome` с commit-callback outcome. Они уже есть
в source и не предлагаются повторно; TUF completed-step eligibility и portable
trust driver ими не установлены. Для retention использовать captured V2-183
`delivery_state.go:43–121,127–140`; прочие line refs относятся к указанным
symbols и сохранённому cut. V2-184 selected-file intake учтён по изменениям
плана/SDK request, не как независимая проверка всего нового reader.

Android `LibboxPlatform` уже оборачивает Go-backed metadata в callbackStep;
дополнительно прочитаны `AndroidTunFactory.open` и
`ProtectedSocketControl.protectBorrowedSocket`: тот же physical binding
проверяется вокруг capability/establish, а dup/protect/bind используют original
lease и безусловное освобождение owned duplicate. Это source-guards, не atomic
OS admission, native proof или закрытие AND-02/05/06. Предложения об общем
context/retirement contract не должны повторно создавать эти локальные guards.

**Поздние входы для сверки перед применением:** после reviewed snapshots
продолжили меняться `docs/PLAN.md`, `docs/ANDROID-PLAN.md`, `docs/STATE.md`,
`android/STATE.md`, `LibboxPlatform.kt` и `CHANGE-REQUEST-WINDOWS-SDK-001.md`.
На последнем проверенном заголовке уже V2-185 и AND-02-S6; полное содержание
этих последних revisions **не проверено** данным crosscheck. Their latest
bytes сохранены как `LATE_NOT_REVIEWED`, а не подставлены вместо reviewed
hashes. Руководителю перед внедрением согласовать дельту с актуальными
владельцами; сам факт позднего изменения не доказывает новый дефект и не
отменяет зафиксированные разрывы API/producer. Дальнейшая freshness за cut
не гарантируется.

При проверке поздних входов старые administrative Z01/Z08 STATUS уже
архивированы из canonical paths, но frozen late snapshots доступны и прочитаны.
Это не missing knowledge и не новая версия научных выводов. Ссылки на их
STATUS не используются как current runtime status или обязательный продуктовый
вход; semantic units сохраняют sourceSha256 из late inventory.

**Не делать:** внедрять все research proposals; унифицировать platform retry
числа; менять score weights ради новизны; открывать Android Connect до accepted
provider; переносить Windows package целиком в второй AAR; ждать завершения
всех чужих P0 ради минимального freeze; брать full cohort из latest health;
равнять coverage/freshness и statistical loss; забывать floors/ошибочный budget
при reset; создавать новый DTO/store/layer вместо имеющегося ownership seam.
