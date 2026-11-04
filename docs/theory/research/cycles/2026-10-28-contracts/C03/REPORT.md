# R28-C03 — От producer evidence к допустимой рекомендации

28 октября 2026. **READY_FOR_REVIEW**; source/design/manual analysis. Не реализация и не field reliability result.

## 1. Ограниченный ответ и новая дельта

**R28-C03-A, PROPOSED:** для optional advisor нужны четыре раздельные проекции:
current coverage/authority, complete registered cohort, statistical guarantee и
historical→target bridge. API1 подходит для первой и части фактов второй, но не
содержит достаточных inputs для всего решения. Ни `AllPassed`, ни current receipt,
ни persisted ordinal не заменяют отсутствующие части. Дополнительный controller
или DB не являются математической предпосылкой; trusted producer boundary — да.

Сверх baseline010 здесь прослежен реальный Windows consumer и найдено конкретное
расхождение **назначений**: latest-value publication отбрасывает устаревший обычный
результат ради текущего состояния; statistical cohort обязан сохранить его как
исход прежней unit. Сравнение A/B сохраняет разные identities. Это не finding
уязвимости/измеренного дефекта existing deterministic preference.

Формулы envelope, A1/A2/FWER и поправки R03-17 берутся из [baseline/review](SOURCES.md#1-baseline-и-научный-статус).
Их не доказываем заново и не выдаём наличие полей за выполненность предпосылок.

## 2. Один actual seam: network-change optional preference

Source snapshot28октября:23project inputs before/copy/after согласованы; pin/SHA
и read depth — [SOURCES](SOURCES.md). Код ничего не запускал.

| Этап / actual caller | Что source действительно даёт | Что ещё не получено |
|---|---|---|
| `onNetPathChanged` | Branch running/startBusy; вызов netWatchProbeStart либо retry wake/return | Не roster всех OS events и не статистический admission |
| `netChangeReprobe` | Сохранённый applied cc, sessionCurrent, outer context15s, вызов protectedProbe.Run | Сам deadline не full useful-task outcome |
| `appProtectedProbe.Run` | ObservationID выделен до underlying probe; intent/owner/generation/session/epoch; original start/finish; late-context/trust/stale guards | Нет FamilyID/GoalRef/purpose roster; planned/admitted times не измерены этой выдачей ID |
| `ProtectedProbe.Run→AdmitDiagnostic` | Pre-I/O reservation полного bounded request/body workload; rejected не измеряет путь; defer release | Request count — resource units, не число независимых samples или α |
| HTTP/DoH steps | Bounded response/status/body/elapsed и fixed result slots; WaitGroup до aggregation | Несколько endpoints могут иметь общий bottleneck; HTTP task не user work task |
| outer finalization | Normalize, cancel/trust/current checks, timestamps; original identity сохранён | UTC timestamp не clock-continuity certificate, AllPassed не future-risk bound |
| `verifySessionProtection→publishObservation` | Current Manager/intent/session/epoch/clock gates; older ordinary observation не регрессирует state, unsafe имеет отдельный приоритет | Это current state projection, не complete cohort/negative evidence всех trials |
| `tryPreferredChannelOnNetworkChange→preferredChannelForNetworkChange` | Другой enabled/live-verified descriptor по class hint; no-op при unknown/no match | Не scoring вероятности успеха/оптимальности/стоимости |
| `switchChannelCoreFor` | operationMu, Expected/current probe check, Manager selector либо structural Reload, new probe/rollback и current publication | Прошлый probe не делает результат candidate заранее известным |

`Manager.LastProbeReport` возвращает clone **одного** retained исторического
report. Он не является журналом. У underlying HTTP observation нет full producer
clock-domain/admission/cohort identity. Результаты проверки fresh current owner
и статистическая полнота — независимые обязанности.

В bounded non-test search Windows/shared SDK `ComposeEvidence/Prefer` production
consumer не обнаружен; actual consumer использует Windows helper. Это не
доказательство отсутствия любого адаптивного кода на других платформах/в EXE.

### 2.1. Две оси identity, которые нельзя склеивать

`ComposeEvidence` требует exact `receipt.Identity==input.Identity`, выбирает
latest decisive observation для exact capability и пересекает его окна с
`HardNotAfter`. Поэтому это подходящая проверка **current coverage A**.
Для historical evidence B, иной конфигурации/owner/cutover требуется original
identity и явная mapping/bridge к decision context. Переписать B.PathRef в A.PathRef,
обновить clock/window или присвоить старой записи текущую generation нельзя.

Historical aggregation может связывать несколько исходных identities по одному
заранее заданному experiment/goal. Такая связь не является новым hard permit;
single-identity coverage composer её за владельца не доказывает.

## 3. Схема `EvidenceForDecision` — proposal, не public API

`Known(value,witness)` отличается от `Unknown(reason)`. Witness может быть owned
source fact или явно объявленной model assumption; opaque ref сам по себе не
делает предпосылку истинной. Все identifiers/коллекции/counters имеют owner caps;
их отсутствие/overflow → unknown/validation error, не infinite retention.

| Часть | Minimum fields / semantics | Existing API1 reuse и недостающее |
|---|---|---|
| Decision scope | Expected current EvidenceIdentity, immutable config/policy/cutover refs, actual Now/ClockDomain, candidate policy ref и GoalRef | Scope/EvidenceIdentity есть; whole-policy target и freeze refs нужно дать owner |
| Current gate | Current owned protection/authority result, exact required capabilities, original hard expiry; CurrentProofRef | ComposeEvidence сохраняет RequiresPlatformGate; это coverage, не разрешение применить рекомендацию |
| Goal | Task, eligibility/unit definition, loss/support/normalization, follow-up origin/D/H, all branch outcomes, full-policy versus probe-only, attribution boundary | Capability.Workload/Observation.TaskDeadline несут часть фактов, не определяют loss/model сами |
| Roster | Frozen CohortRef/PopulationRef, source entrypoint coverage, admitted/start/record/reveal orders, per-unit original identity; rejected/not-started/pending/cancelled/missing retained; completeness status | Observation.ID/Sequence/state помогают. Нет complete roster/purpose/family witness в snapshot API1 |
| Label envelope | MaturePrefixRef, `[a_i,b_i]`, prefix/snapshot-valid aggregate Γ in loss units, maturity/follow-up witness, original receipt refs | Censoring/Followup/CompletionLatency есть. `completed` не доказывает нужный outcome; Γ/true loss producer отсутствует |
| Statistical claim | BoundKind/EstimandRef/FiltrationRef/ProcedurePin, KernelRef/EnvelopeKind/TransformRef, frozen prediction/tuning/history cut, interval и его exact family/time/support scope | API1 статистического сертификата не выдаёт. Health OK/FAIL не его замена |
| Error accounting | CoverageMode, FamilyRef/ClaimRef; либо membership в уже jointly covered семье, либо admission α_j и A1/A2 witness. Joint label/bridge budgets отдельно | AttemptReceipt.Scope/Generation коррелируют действие, но не содержат family/α/conditional validity |
| Bridge | Historical→target policy mapping, `q` definition, validity horizon, age/selection/workload/family/path/probe-load conditions, bound B/guarantee scope и η | В просмотренном API1 нет prospective bridge. Не брать B из score/no alarm/TTL |
| Physical feasibility | Planned/reserved/actual amounts/units и owner recheck, pending/retention/operation caps; outstanding custody | ObserverFacts/OperationCustody/settlement помогают. Own retirement не означает, что общий pool свободен |
| Output lineage | Exact snapshot/roster prefix/claim/model/clock refs, current Expected, bounded WhyUnknown list, RequiresPlatformGate=true | Existing trace/Expected pattern переиспользуется; новый controller/DB не требуется автоматически |

Группа CurrentScope относится к состоянию **сейчас**. Группа Roster содержит
original identities каждой unit. Связь между ними — Bridge, а не неявное равенство.
Семья/процедура/target замораживаются в момент, который требует доказательство;
completion-order или UI refresh не сдвигают этот момент назад.

### 3.1. Что advisor действительно обязан получать на каждом вызове

Таблица выше задаёт **producer obligations**, а не требование пересылать всю
историю в каждую pure function. Минимальное computational ядро сравнения:
Expected/current gate + fixed target/units/H + два совместимо обоснованных target
intervals/GuaranteeRefs + действующий margin + relevant physical admission status.

Roster/order/Γ/family/model binding могут быть одним frozen bounded producer
summary с independently inspectable provenance. Advisor не доказывает scientific
валидность по непрозрачной строке `witness=true`. Owner обязан обеспечить свойства
summary, а scientific review — проверить применимость модели. Raw rows/payloads
в DTO не обязательны при уже доказанном sufficient summary. Snapshot limit64 в
API1 не становится lifetime cohort limit и не требует новой БД.

### 3.2. Что значит mature label

Предлагается сначала выбрать **одну** минимальную task definition. Например,
«parent принял result указанной операции до D» может наблюдаться parent owner,
включая локальный reject. Это не равно «пользователь завершил работу через VPN».

- Доказанное completion нужной task в declared scope даёт point loss согласно goal.
- Доказанное отсутствие parent acceptance до D может дать failure **только для
  этого parent-acceptance goal**, если follow-up/clock/record coverage обеспечены.
- Нет ответа к D не всегда означает отсутствие remote/user completion. Для
  latency total или полезности приложения это может остаться `[0,1]`/right censoring.
- Local rejection/cancel либо имеют заранее определённый policy loss, либо не
  принадлежат этой целевой величине. После результата менять denominator нельзя.
- Mature statistical label и actual OwnerReturned/ResourcesRetired раздельны.
  Label может быть известен, когда ресурсы ещё удерживаются; вернуть лимит нельзя.

`TaskDeadline.Known`, `Followup.Present`, `Custody.ReturnKnown` не заполняются
StartedAt/timeout/false по умолчанию. Existing validator проверяет форму и ordering,
но не подтверждает, что producer действительно наблюдал нужную task всё время.

### 3.3. Варианты типов и недопустимые преобразования

Ниже нотация sum types, не исполняемый API/fixture:

- `Fact<T> = Known<T>{value, originalOrigin, witnessRef} | Unknown{why}`.
- `CurrentGate = Passed{proofRef, exactScope, clock, expiry} |
  Unsafe{signalRef} | Unavailable{why}`. Known unsafe не теряется внутри Unknown.
- `CoverageMode = CoveredFamilySubset{jointFamilyRef, membership} |
  NewAdmissionClaim{admissionRef, alpha, A1Ref, A2Ref}`.
- `LossClaim = FiniteRegistered{cohortRef, prefix, lossUnits, interval} |
  JointHistorical{estimandRef, filtrationRef, guaranteeRef, interval} |
  JointTarget{policyRef, goalRef, horizon, bridgeRef, guaranteeRef, interval}`.
- `Advice = PlatformReviewRequired{facts, why} | Abstain{facts, why} |
  SuggestCandidate{candidateRef, expected, comparisonRef, requiresPlatformGate} |
  CandidateDoesNotClearMargin{pairRef, horizon, comparisonRef}`.

Переход FiniteRegistered→JointHistorical требует statistical proof binding;
JointHistorical→JointTarget требует bridge. Равенство numeric endpoints не даёт
этих преобразований. Для варианта SuggestCandidate `requiresPlatformGate` —
константа true, а не выключаемая настройка. Full family/roster inputs могут
оставаться у producer по §3.1; существование такого типа не удостоверяет witness.

## 4. Finite decision rule

Это только правило допустимости **optional** recommendation в выбранном seam.
Start/manual choice/hard recovery/safety stop остаются у platform owners.

1. Проверить version/caps/shape/original refs. При malformed input — unusable result;
   при missing assumption — partial evidence и `WhyUnknown`, не success.
2. Сначала current hard gate. Safety signal, changed Expected, unknown/expired
   authority/clock или unsupported required capability → `PlatformReviewRequired`.
   Это не `Keep`, не новая permission и не очередная statistical attempt.
3. Проверить declared unit/cohort/completeness/maturity и label envelope. Получить
   допустимый finite/historical bound только по соответствующему baseline contract.
   Нет A1/допустимой совместной процедуры — сохранить descriptive/finite evidence,
   **не** выдавать confidence label.
4. Различить CoverageMode из§6. Нужны exact family/target/procedure membership и
   continuity; расходы физических requests не используются как α.
5. Применить **отдельный** bridge к тому же target каждой сравниваемой policy.
   Unknown bridge оставляет `TargetPolicyBound=Unknown` либо тривиальный `[0,1]`
   при известном bounded support. Ни prior, ни drift0 не подставляются.
6. Проверить physical feasibility/remaining reservation у owner. Unknown amount,
   custody или отсутствие reserved capacity не дают права начать measurement/switch.
7. Для full-horizon expected losses A и B в **одних единицах**, где B включает
   transition/probe/rollback/failure costs по declared goal, пусть valid target
   intervals `[L_A,U_A]`, `[L_B,U_B]` и действующий owner margin m≥0 (см.§7.1).
   Если `L_A−U_B>m`, вернуть `SuggestCandidate(B)` с Expected/refs.
   Если `U_A−L_B≤m`, вернуть `CandidateDoesNotClearMargin` для этой пары/горизонта.
   Иначе — `Abstain(InsufficientSeparation)`; произвольное дополнительное probing
   этим output не разрешено. Компоненты gain006 можно использовать только при
   отдельно обеспеченных inputs; новый score не вводится.
8. Перед effect platform повторно проверяет Expected/current authority/capacity и
   одноразовое владение attempt. Late/stale advice не переименовывается в current.
   Existing switch/new probe/rollback/current publication gates сохраняются.

На совместном событии покрытия intervals сравнение корректно для объявленного
**conditional expected target**, не гарантирует каждый индивидуальный outcome.
Ошибка этого сравнения ограничена тем joint guarantee, который реально дан
producer/model; универсальная численная вероятность из схемы не возникает.
`Abstain` не снимает защиту и не разрешает direct fallback.

Ручной positive check D1: при обоснованных target intervals A[.4,.5], B[.1,.2]
и margin.05 нижняя gain.2 превышает margin, поэтому SuggestB допустим **при тех
же current/physical gates**. При margin.25 separation недостаточна; при.5 даже
optimistic gain.4 её не достигает. Если у B исчез bridge и interval становится
[0,1], при margin.05 снова abstain. Это арифметический sanity argument, не
статистический запуск, не установленные вероятности/стоимость VPN.

### 4.1. Минимальный `WhyUnknown`

`scope_changed`, `clock_binding_unknown`, `authority_unavailable`,
`current_capability_missing`, `population_not_closed`, `registration_not_predata`,
`maturity_unobserved`, `label_envelope_unknown`, `conditional_model_unproved`,
`family_coverage_unproved`, `continuity_lost`, `bridge_unknown`,
`target_or_units_mismatch`, `resource_custody_unknown`, `separation_insufficient`.
Это proposed enum vocabulary, не реализованные reason codes в API1. Сохраняются
конкретные partial facts; unknown в statistical части не превращает известное
unsafe evidence в рядовой informational message.

## 5. Producer obligations и самый дешёвый scope

Current scalar counters не доказывают historical completeness. Для минимальной
реализации допустим **closed finite cohort** только новых optional experiments,
перечисленный до evidence access в уже существующем owner state. Каждая unit
имеет fixed goal/policy/purpose; невыполненная/неполная сохраняется. Нужна одна
контролируемая точка исполнения этих units, unique consumption и bounded retention.

Не нужно автоматически регистрировать каждый пакет всего приложения, если
claim относится лишь к указанной когорте. Но это не позволяет перенести её успех
на все network-change events, на unseen workloads или на lifetime reliability.
Current netChangeReprobe отсекает часть событий **до** appProtectedProbe.Run;
выданные Run IDs поэтому не roster всех network events. Purpose annotation для
выбранной когорты owner должен дать до использования данных.

Для one-owner/process-local scope DB необязательна. При restart/restore/потере
continuity — unknown старой семьи/receipt lineage; это не новое бесплатное δ.
Если заявляется device-lifetime family, требуется дополнительно обоснованное
сохранение spent budget/identity/rollback boundary. Такой storage contract не
возникает от `NextSequence` или hash-chain. Его не реализуем здесь.

Clock producer сохраняет original sample/follow-up/expiry и exact continuity
domain. Go `UTC()` убирает monotonic component [P3]; raw UTC nanoseconds нельзя
объявлять готовым elapsed/trust-clock binding. Existing hard clock gates остаются
своими gates, а не источником статистического drift bound.

### 5.1. Ресурсный и statistical budgets

Existing `AdmitDiagnostic` резервирует requests/body и active lane до I/O, не
refunding error/cancel в rolling history. Это полезный reusable guard; его нельзя
заменить «успехами» CS. Existing AttemptReceipt/settlement различают returned/
retired/discard. Они не знают family α и не доказывают truthful label.

Обратная сторона: уже покрытый fixed claim не обязан тратить новый α на каждый
UI query. Physical operation может требовать новый admission даже без нового
statistical claim. Два accountant отвечают на разные вопросы. Не вводится запрет
на любые допустимые recycled-budget методы; без конкретного доказательства
completion/cancel не являются основанием возврата α.

## 6. Связанные углубления

### 6.1. Самый слабый bridge

Даже при полном honest roster, корректной clock/custody и narrow historical CS
будущий useful-task риск может остаться неидентифицированным. Existing successful
egress/DoH workload не наблюдает все рабочие приложения и реакции будущего DPI.
Два мира с тем же историческим probe outcome и opposite future task outcome
дают одну input history. При unknown future model нельзя универсально сузить
target `[0,1]`. Это bounded impossibility без дополнительного input, не утверждение
о конкретной атаке или недоступности всех forecasting моделей.

Что действительно меняет задачу:

- Новый owner-observed **нужный task outcome**, а не ещё один status200 того же
  measurement workload. Это улучшает historical target alignment, не доказывает
  stationarity на следующий час.
- Заранее определённый full-policy experiment, eligibility/assignment/positive
  support/common-dependency unit/workload/H и complete outcome. Внутренний DPI-state
  восстанавливать не обязательно; baseline§5 даёт ограниченный ITT target.
  Interference/admission failures/probe load входят в исход, а не отфильтровываются.
- Отдельно обоснованная **prospective** scope/horizon guarantee связывает historical
  target с future q. Finite past alone не удостоверяет unknown drift bound.
- При отсутствии такой гарантии можно честно выдавать historical diagnostics
  или сохранять existing deterministic policy. Scientific forecast не объявляется
  необходимым условием каждой обычной platform recovery операции.

Даже perfect past A/B assignment не создаёт unseen joint rollout support и не
позволяет автоматически изменить schedule/probe/termination policy в target.
Поля `ProtocolRef` и `InterferenceDomainRef` — refs обязанностей, не доказательства.

**Aggregation bridge, O11.** Даже если уже известен **future** mean failure loss
µ для каждого из N задач в среднем, риск хотя бы одного срыва не обязан равняться
µ. При фиксированном известном N и arbitrary dependence из `1_any≤K≤N·1_any`
получаем `µ≤P(any)≤min(1,Nµ)`, где K — число срывов. Для N2 и marginal risks.1
совпадающие failure events дают.1, несовместимые —.2 при том же mean. Это ручной
пример определения цели, не новая dependency theorem и не model VPN.
Known N может сузить **aggregation** uncertainty; откуда взялась future µ —
по-прежнему отдельный temporal bridge. Если goal сразу whole-episode binary loss
«хотя бы один срыв», его complete observer может обойти это промежуточное mean
преобразование. Число HTTP subrequests нельзя выдать за N пользовательских задач.

### 6.2. Поздние claims: subset или новая проверка

**CoveredFamilySubset:** цели/contexts/procedures уже входят в одно simultaneously
covered family event. Поздняя доставка оригинальных outcomes и data-dependent
выбор поднабора не расширяют error event. Можно производить deterministic interval
arithmetic для сопоставимых targets; каждый новый экран/сравнение не новый α.

**NewAdmissionClaim:** новое условие/workload/срез/процедура/target после просмотра
данных не становится covered по смене ClaimID. Нужны A1 при реальной pre-admission
history и A2 семьи либо другое доказанное selective-inference решение. Соседние
pending results, уже раскрытые scheduler, являются частью этой истории.

Registration order, reveal order и record/publication order сохраняются отдельно.
Фактически поздний label можно добавить в original unit/итоговый outcome, но не
в predictor/history, которыми якобы пользовалось прежнее решение. Для already
valid CS допускается declared valid prefix/interval refinement; finish-order mean
не исправляется одним отсортированным экспортом после факта.

Если label envelope совместно валидна с error budget β и bridge с η, accepted
baseline использует additive union bound с α_stat, без предположенной независимости.
Pointwise сертификаты каждого late snapshot не создают joint coverage. Одинаковое
имя model ref не даёт права считать их одним уже оплаченным joint certificate.

### 6.3. R28-C03-B — aggregate envelope и weighted caller несовместимы без условия

**PROPOSED, новая дельта.** В baseline Γ ограничивает обычные prefix sums:
`Σa_i−Γ_n≤ΣY_i≤Σb_i+Γ_n`. Выбранный caller confseq имеет weighted center
`Σw_i X_i/Σw_i`. Это разные функционалы. Одного current Γ_n недостаточно,
чтобы просто объявить weighted corruption равной Γ_n или `max(w_i)Γ_n`.
Второе было бы достаточно при **отдельном L1-error bound**, которого aggregate
envelope сама по себе не даёт.

**Ручной контрпример K1.** В двух позициях с весами u>v>0 истинные losses(1,0),
reported point labels(0,1). Обычные totals совпадают: final Γ2=0 допустима;
для первого prefix Γ1=1. Weighted totals равны u и v, ошибка u−v>0.
Если это не первые позиции, все предыдущие true/reported losses равны0.
Реальный default λ source после участка truncation имеет строго убывающие пары;
мы не утверждаем, что его первые два веса при alpha.05 равны1и.5.
Γ1=1,Γ2=0 — допустимый **aggregate** профиль, не nondecreasing count corruptions.

**Достаточный вариант A: prefix envelope + nonincreasing kernel.** Пусть
`w1≥…≥wn≥0` и joint envelope дана для каждого prefix k. По summation by parts
для lower prefix deviations `D_k=Σ_(i≤k)(Y_i−a_i)≥−Γ_k`:

`Σw_i(Y_i−a_i)=w_n D_n+Σ_(k<n)(w_k−w_(k+1))D_k≥−κ_n`,

где `κ_n=w_n Γ_n+Σ_(k<n)(w_k−w_(k+1))Γ_k`.
Все коэффициенты неотрицательны. Для upper deviations относительно b_i
получаем симметричную верхнюю границу+κ_n. Следовательно weighted finite total
лежит между `Σw_i a_i−κ_n` и `Σw_i b_i+κ_n`.
Это собственный elementary algebra вывод из baseline envelope, не новый CS.
В K1 κ=u−v и bound достигается. Statistical kernel дополнительно должен быть
предсказуемым относительно **нужной** filtration; post-hoc fitting весов не
разрешается одной pathwise алгеброй. Полный current API1 этих Γ_k не предоставляет.

**Достаточный вариант B: frozen constant kernel, меньший input contract.**
Exact `predmix_hoeffding_lower_cs` при положительном `fixed_n=Nopt`, выбранном
**до** анализируемых данных, использует постоянный w>0. Тогда κ_n=wΓ_n, weighted
mean становится обычным mean и отдельная история prefix Γ для этого перехода
не нужна. Source `fixed_n` — tuning horizon, не обязанность закончить ровно
на Nopt и не новый α на каждый query. Нельзя повторно ставить Nopt=len(x) после
каждого просмотра и называть это одной заранее выбранной CS.

Для валидной constant-conditional-mean модели **истинных Y_i** относительно
объявленной filtration, `N=None`, `running_intersection=False` и two-sided level α,
source expression даёт `w=sqrt(8 log(2/α)/Nopt)` и
`r_n=log(2/α)/(n w)+w/8`. Corrupted/interval labels сами этой модели не обязаны
удовлетворять: envelope переносит bound с истинных Y, не доказывает модель по X.
После добавления aggregate Γ: `[Σa/n−Γ_n/n−r_n, Σb/n+Γ_n/n+r_n]`, clipped[0,1].
Это алгебраическая адаптация существующего kernel, **не реализованный wrapper**.
При ином historical conditional-mean target требуется соответствующее proof
binding, как в baseline010; не переименовываем target в future risk.

**Цена B:** `r_n→w/8`, поэтому fixed kernel не даёт shrinking-to-zero interval
на бесконечном горизонте. Для bounded incident вокруг заранее выбранного Nopt
это допустимый компромисс, но optimality/численная выгодность не проверены.
Цена A — хранение/обоснование prefix Γ и exact kernel/order mapping. Известный
pathwise L1/corrupt-count bound может дать иной достаточный transform; TV bound
его не заменяет. При unknown envelope/kernel binding → `WhyUnknown`.

Следствие для schema§3: добавить `KernelRef/EnvelopeKind/TransformRef` в guarantee
binding. Existing Observation и библиотечное имя «CS» эти семантики не кодируют.
Upstream принимает обычный числовой x; отсутствие регистрации/censoring/Γ там
не объявляется багом. Подавать midpoint unknown labels не является адаптером.

**Порядок postprocessing также часть контракта, K2.** Поправку каждого prefix
нужно применить **до** optional running intersection. Если сначала взять maximum
неисправленных lower bounds, затем вычесть только current Γ_n/n, ранняя ошибка
может остаться даже при постоянном kernel. Ручной пример: α=2e^(-2), Nopt=64,
w=1/2; все истинные Y_i=0, первые64 reported labels равны1, следующие64 равны0.
Совместная envelope Γ_k=min(k,64) верна детерминированно. При k64 raw radius=1/8,
raw lower=7/8. На n128 raw running lower≥7/8; вычитание current Γ128/128=1/2
оставляет lower≥3/8, ошибочно исключая µ=0. Это counterexample неправильного
wrapper, не upstream miscoverage при его исходных assumptions.

Исправленный порядок: построить robust `[l*_k,u*_k]` каждого prefix с его Γ_k,
потом `max_(k≤n) l*_k` / `min_(k≤n) u*_k` при совместной валидности для **одного
и того же µ**. Если исторические targets θ_k меняются с k, простое пересечение
разных θ_k не оправдывается совместным покрытием: нужна отдельная mapping.
Либо оставить `running_intersection=False` и использовать только current robust
interval: именно этот вариант B не требует историю Γ для отдельного query.
Storage/expiry semantics относятся к полученному joint claim; пересечение не
превращает historical bound в current authority или future forecast.

## 7. Ручной opposing review

Все traces ниже — конечные логические/source модели. Их не запускали и не
выдаём за наблюдённый race, leak, miscoverage или реальную сеть владельца.

| Trace | Самый сильный отрицательный случай | Existing guard / точная граница вывода |
|---|---|---|
| O1 outcome-dependent delay | Unit1 slow failure стартует первой; unit2 fast success приходит раньше. Completed-only mean0 заменяет mean1/2 двух assigned binary losses | start-order ObservationID полезен, но publishObservation намеренно не даёт старому ordinary report регрессировать состояние. Cohort сохраняет обе original units отдельно от latest coverage |
| O2 hidden admissions | У retained успешной записи непрерывный ID, но scheduled событие могло уйти по busy/no-session branch до Run. Claim «все network changes успешны» не имеет такого denominator | `onNetPathChanged/netChangeReprobe` source branches реально есть. Можно сузить population до controlled admitted experiments; нельзя скрытно расширить на events |
| O3 stale proof | Advice подготовлено при owner/epoch/configA; к применению поменялась сеть/intent/authority | Current checks и Manager publication в source сохраняются. Они не заменяются frozen statistical certificate; новая рекомендация обязана пройти тот же owner gate |
| O4 concurrent stale budget | Два planners читают remaining.05 и каждый резервирует.04; Update callbacks пишут вычисленный снаружи остаток.01 | Single writer сериализует transactions, но не исправляет stale arithmetic. Reread/check/debit+unique ClaimID должны быть внутри одного owned admission commit; это proposed ledger counterexample, не current code bug |
| O5 Batch side effect | CallbackA запускает probe и пишет roster; callbackB возвращает error. Batch rollback затем повторяет A: side effect дважды, committed запись одна | exact bbolt batch.run повторяет оставшиеся callbacks после failIdx. Actual cache caller делает DB cleanup, не наш probe. Новый probe нельзя помещать в Batch/Update callback |
| O6 reset/restore | После возврата старой DB/current owner sequence снова выдаёт прежний ID, spent α неизвестен, controller считает budget свежим | Durable ordinal без continuity/rollback contract не lifetime accountant. CacheFile.resetDB вообще удаляет storage. Новый family label не возвращает δ |
| O7 future risk | В двух мирах все historical probes/accepted receipts одинаковы; следующий useful task loss0 либо1 | Protected probe подтверждает свой workload/путь; actual code не обещает все user tasks. Without bridge current target остаётся[0,1]/unknown |
| O8 label versus custody | Parent-acceptance-by-D failure уже определён, но underlying owner ещё не вернулся/ресурсы не retired | API1 separately tracks custody. Такой label не освобождает reservation и не открывает новый physical slot |
| O9 duplicate/late callback | Завершение attempt17 пришло дважды либо после attempt18 | SettleFailover проверяет Attempting/generation/scope, mismatch не меняет successor. Proposed cohort тоже не увеличивает n за повтор delivery и не переписывает origin |
| O10 kernel mismatch | Aggregate Γ_n=0 при взаимно компенсированных label errors, но weighted total имеет ненулевую ошибку; raw running intersection удерживает раннюю ошибку после current correction | K1/κ и K2§6.3; fixed kernel либо stronger envelope, correction before intersection для одного target. Это adapter mismatch, не runtime finding confseq |

**Важный positive opposing case.** Если shared network factor делает ошибки
streams зависимыми, это само по себе **не** опровергает уже установленный A1/A2
или jointly covered family. Нельзя отвергать всё зависимое data только из-за
зависимости; нужно проверить конкретный target/filtration/support. Равно и honest
complete journal не доказывает эти предпосылки автоматически.

### 7.1. Минимальность: не всё надо замораживать до данных

Предзаданный margin — простой достаточный вариант, но
**pre-data margin не математически необходим для interval guarantee.** На event одновременного
покрытия fixed targets, из `L_A−U_B>m` следует `q_A−q_B>m` pathwise даже для
data-dependent выбора A/B/времени/m, если это selection **внутри уже покрытой семьи**
и все величины имеют прежний смысл/единицы. Нового α такое threshold reading
не требует. Margin должен быть записан как действующее owner policy choice.

Менять после данных **сам loss/normalization, population, прогнозный horizon,
модель, kernel/predictor или процедуру** — другой случай. Нельзя ссылкой на
threshold selection перенести guarantee на новый target. Сохраняем только те
pre-data freezes, которые реально нужны выбранному proof binding; не заставляем
создавать новую DB transaction для каждого интерфейсного просмотра.

### 7.2. Validity после нового snapshot

Новая порция данных сама по себе не делает старый jointly valid claim ложным.
Он остаётся claim своего original prefix. Нельзя молча переименовать его в mean
нового prefix/текущий risk или продлить исходный bridge сроком нового callback.
При изменении task/model assumptions, clock continuity или relevant live scope
binding старый target certificate использовать нельзя.

Для эффекта берётся minimum **сопоставимых** current authority/coverage/bridge/
reservation bounds; окна разных clock domains нельзя минимизировать простым
сравнением integer nanoseconds. Structural future bound B и hard expiry имеют
разные свойства. Current metadata не обязаны иметь «самую новую» numerical CS,
но все использованные original гарантийные области должны ещё применяться.

## 8. Reuse и осуществимость projection

### 8.1. API1: ADAPT существующие facts, DEFER probabilistic producer

Можно сохранить нынешние components: existing App/Manager owner, diagnostic pool,
Observation/EvidenceReceipt, exact coverage composer, Expected и typed settlement.
`EvidenceForDecision` — proposal для bounded wrapper/read-model владельца.
Existing API1 **в одиночку** не выражает FamilyGuarantee/Cohort completeness/
loss envelope/Kernel binding/Bridge. Строки `Workload/DependencyRef` не стоит
перегружать невалидируемым JSON ради видимости полной поддержки.

Минимальный closed process-local roster и frozen guarantee refs могут жить в
existing owner state. Если появляется новый bounded getter/projection, это
platform-owned source work и отдельная приёмка. Не нужны второй VPN controller,
новый service, ML score или новая DB только ради передачи immutable input.

Дополнительный контракт нужен для **relevant** custody/capacity конкретного effect,
а не blanket запрет из-за любой незавершённой исторической операции. Независимый
зарезервированный lane может быть допустим owner; неизвестность старого ресурса
нельзя объявить его освобождением. Library ни один admission сама не выдаёт.

**Actual scope:** netWatcher.run в current source пропускает polling/dispatch при
`tunRequested()`. Поэтому выбранный automatic network-change consumer не является
доказанным TUN/Android measurement path. Watchdog/lifecycle/Android требуют своего
caller review. Сам proposed input contract переносим только при owner binding.

### 8.2. confseq0.0.11 — OFFLINE_REFERENCE / scoped mathematical ADAPT

Exact MIT commit **5ffe733ca2447a2e28c2c91f3b00086173f2ab2c**, fresh source и
LICENSE/setup прочитаны [C2]. Actual path: two-sided Hoeffding→две lower calls
с alpha/2→weighted sum/margin; CI-wrapper устанавливает fixed_n=len(x).

Для future offline wrapper выбирать frozen CS path, bounded honest/interval
support, правильный outcome order/filtration и KernelRef. Constant-mean theorem
не становится future q theorem. §6.3B — один конкретный bounded candidate
упрощения adapter: fixed заранее Nopt/kernel и aggregate envelope, без приёма
current source за готовый producer. §6.3A — другой вариант с prefix Γ.

Цена: validation/label transform/finite precision/alpha underflow/overflow и parity;
package NumPy/SciPy/multiprocess/pybind11 dependencies не ставятся на клиента.
Raw formula требует простых sums, но это не измеренный CPU/RAM/WCET: bounded
roster/pending IDs и late updates всё равно имеют retention cost. Численная
устойчивость/reference C++/ABI и execution coverage не проверены.

### 8.3. Уже pinned bbolt — CONDITIONAL_REUSE primitive, REJECT cachefile как ledger

Version `v0.0.0-20231014093535-ea5cb2fe9f0a`; full commit
**ea5cb2fe9f0afaec4827c0591954db0e11ba548c**, MIT2013BenJohnson. Fresh primary
Go proxy metadata и db.go/bucket.go/LICENSE совпали с exact cache [C1].
Это закрывает только прежний narrow provenance gap этих files, не package audit.

`DB.Update` позволяет reserve+record внутри одной writer transaction при recheck
свежего owned state. `NextSequence` — primitive, не append-only enforcement;
Put/SetSequence/Delete остаются доступны. `Batch` callbacks повторяются;
`OnCommit` вызывается после disk/meta step/закрытия transaction, но не атомизирует
старт/завершение внешнего worker. Ни одна DB transaction не подтверждает network
outcome или отсутствие скрытых входов.

Если lifetime persistence действительно выбрана platform owner, порядок такой:
validate/freeze → atomic **DB-only** reserve+record → успешный commit → current
owner recheck/one-use claim → effect через existing executor → original outcome/
retirement update. Crash между commit и effect оставляет registered not-started/
unknown, не основание наугад повторить действие или выдать synthetic success.

Этот протокол не добавляет DB I/O внутрь nested publication critical section:
`publishObservationLocked` требует callback-free in-memory commit и запрещает
I/O/Manager mutation при runtime/epoch locks. Один operationMu сам по себе
не объявляется здесь запретом любого I/O — current switch держит его и при
работе Manager. Новый lock/storage latency design нужно согласовать с owner.

Actual sing-boxv1.13.19 CacheFile.Start удаляет top-level buckets вне whitelist
и resetDB может удалить весь файл. Не утверждаем, что каждая nested cleanup
ветка удаляет всё; достаточно реально существующего whole-file reset. Это
cache semantics, не дефект cache по назначению. Использовать этот файл для
lifetime statistical spent ledger нельзя. Wrapper LICENSE GPL-3.0-or-later,
low-level bbolt MIT; новый copy не получает MIT автоматически от зависимости.

Цена отдельного owned storage: lifecycle/ACL/quota/retention/backup/restore,
single-writer latency, copied export bytes, long-reader/mmap/resource effects и
anti-rollback/continuity assumptions. Ни fsync, ни GUID сами не доказывают весь
контракт. Windows-only source candidate не переносит бbolt в Android автоматически.
При process-local closed cohort предпочтительнее сначала existing owner state.

## 9. Открытая приёмка и статус системы

Следующие prose cases конечны и **все NOT_RUN**; исполняемые tests/fixtures не
создавались:

| Case | Будущая приёмка | Что нельзя объявить по plan-only результату |
|---|---|---|
| F1 | Full versus filtered roster, delayed failure, duplicate callback | Current coverage trace не полная статистическая когорта |
| F2 | Local reject/cancel/timeout по двум разным task definitions | Один status не universal loss или physical return |
| F3 | Family membership/derived threshold versus new post-hoc target | Не каждый query новый α, не каждый новый ID already covered |
| F4 | Same scope but expired bridge, changed epoch/owner, cross-clock values | Latest receipt не продлевает original guarantee |
| F5 | Conditional A1 с actual pending-result history, missing Γ/B/model | No invented probabilities/priors/zero drift |
| F6 | Aggregate Γ/K1, prefix κ, frozen versus repeatedly retuned Nopt, K2 correction/intersection order | Kernel/units/proof/target и порядок postprocessing должны совпасть |
| F7 | Concurrent stale DB arithmetic, Batch retry, commit-before-effect crash | Storage primitive не exactly-once executor/complete cohort |
| F8 | Source/DB reset and family budget continuity, pending resource | Не fresh lifetime δ и не automatic refund |
| F9 | Positive full-policy intervals with current owner gate | Only conditional expected comparison; field outcome не гарантирован |
| F10 | Windows non-TUN source scope versus TUN/Android producer | Перенос contract не implementation/runtime parity |

**Implementation:** существующие guards/API описаны по source. Proposed wrapper,
cohort/family/bridge producer и weighted-envelope adapter **NOT_IMPLEMENTED** этим
пакетом. **Field reliability:** NOT_EVALUATED; VPS/device/live/скорость/энергия не
измерялись. Независимая научная рецензия ещё требуется.

Z03/Z09 HANDOFF отсутствовали на срезах 08:56:41Z и 09:23:06–07Z;
`paired-inputs.json` сохраняет это ограничение. Их claims —
**NOT_AVAILABLE_AT_CHECK**; независимая producer/cohort сверка с ними
остаётся открытой.

Финальная source/provenance сверка09:29:35Z: все23repo inputs и8module inputs
совпали с исходными captures; `final-input-drift.json`. Это не атомарный snapshot
всего дерева и не runtime proof. Claim A/B — **PROPOSED / READY_FOR_REVIEW**;
решение о научном принятии и интеграции остаётся руководителю.
