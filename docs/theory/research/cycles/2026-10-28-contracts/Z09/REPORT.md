# R28-Z09 — fixed cohort, horizon и maturity для protected task completion

28 октября 2026.
Статус: **READY_FOR_REVIEW**. Это исправленная specification для
R06-013-A после `NEEDS_REVISION`, а не изменение старого пакета, не production
DTO и не измеренный recovery rate.

## 1. Ограниченный вопрос и verdict

**Вопрос:** как заранее задать единицу, horizon и maturity, чтобы ранние successes,
`not complete`, partial и pending не меняли знаменатель protected task completion?

**R28-Z09-A — предлагаемый verdict: ACCEPT исправленную specification A для
независимой рецензии, scoped к fixed registered cohort и endpoint-protected
outcome; не принимать её как оценку всех пользователей или всей операции.**

Основное правило:

> Outcome может стать известным раньше deadline, но episode становится
> **аналитически зрелым** только по условию, не зависящему от outcome:
> `deadline_i=t0_i+D(class_i)` и `deadline_i≤cut_time` в сравнимом clock scope.
> До закрытия enrollment финальный rate не публикуется. После закрытия roster
> interim summary использует весь фиксированный `N`, считая not-yet-due episodes
> unknown, а не исключая их из знаменателя.

Этим устраняется контрпример рецензии: ранний success не получает меньший
outcome-selected denominator. Старый R06-013-B/OSS/pilot не получает verdict
автоматически; его privacy/export части ниже пересмотрены отдельно.

## 2. Исправленный estimand: единица, roster, часы и cut

### 2.1 Единица

Одна единица — **один заранее зарегистрированный task-recovery episode**:

- один versioned `task_class` и один конечный task endpoint;
- регистрация получает `episode_id` и `t0` **до** task outcome;
- recovery stages, channel switches, waits и retries внутри него не создают новые
  единицы;
- новое событие считается новой единицей только после terminal closure прежней
  задачи и новой pre-outcome регистрации другой task attempt;
- duplicate admission отклоняется до наблюдения outcome по локальному owner/ID,
  а не удаляется post hoc.

Это ITT-подобная операционная единица: system wait, user action, admission failure,
manual protection-off и cancellation остаются её событиями, а не основаниями
выкинуть episode из roster.

### 2.2 Fixed roster

До outcomes задаются:

- enrollment window `[E0,E1)`;
- admissible task classes и mapping `D(class)`;
- registration owner/schema/clock rules;
- endpoint и правила protection scope;
- planned analytic cut `C*`, заданный до outcomes и не раньше максимального
  известного deadline принятого roster (`E1 + max D` только когда clock/calendar
  bridge действительно определён).

`N` — число **принятых registration receipts** в roster при закрытии `E1`.
С этого момента `N` не зависит от outcomes. Отсутствие доказанной completeness
registry означает, что вывод относится только к выданным receipts; он не
доказывает отсутствие скрытых/потерянных фактических попыток. Если intended
estimand требует все фактические attempts, но число неучтённых ничем не
ограничено, универсальная оболочка — `[0,1]`.

### 2.3 Часы, horizon и maturity

Для episode `i`:

```text
t0_i       = accepted registration time in named monotonic/boot scope
d_i        = t0_i + D(task_class_i), overflow checked
cut_time   = named analytic snapshot in the same comparable scope
mature_i   = comparable(t0_i, cut_time) AND d_i <= cut_time
```

- `D` фиксируется до outcome; разные классы могут иметь разные versioned `D`.
- Wall clock используется для enrollment/report labels, но не подменяет elapsed
  interval после clock reset/reboot.
- При неизвестном clock bridge episode **остаётся в roster**, но не называется
  analytically mature; timely task/protection component относится к `M_timing`
  в final fixed-roster summary. Его нельзя исключить, назвать success или считать
  доказанно достигшим D.
- `C*` не обязан физически ждать неизвестный bridge: final report может
  закрыться с `M_timing`, показывая широкую identification bound.
- Ранний absorbing completion можно сохранить сразу, но в final-cohort rate он
  входит лишь вместе со всем fixed `N` после outcome-independent cut.

### 2.4 Interim и final reports

1. **До `E1`:** только flow counts (`registered_so_far`, outcomes seen), без
   «rate fixed cohort». Любой prefix — иной estimand.
2. **После `E1`, до всех известных deadlines:** provisional bound по всему fixed
   `N`. `R` = not-yet-due/pending; они unknown mass.
3. **Final cut `C*`:** для comparable clocks `R=0`; timing/follow-up unknown
   остаются `M`, не исчезают. Поздние receipts хранятся как late descriptors и
   не переписывают timely endpoint.

Rolling-window rate допустим лишь как отдельно названный estimand, где eligibility
`d_i≤cut_time` задана независимо от outcome. Он не заменяет primary fixed cohort.

## 3. Outcomes на horizon: provisional observation ≠ terminal endpoint

### 3.1 Факторизованные оси

Плоский enum недостаточен для off→on overlap. Источник хранит три независимые
оси, а итоговая категория выводится на `D`:

1. **Task endpoint:** complete / partial-at-D / noncomplete-at-D / unknown.
2. **Protection scope:** endpoint-applied / explicit-unprotected / unknown.
3. **System/action:** action applied/failed/unknown; user stop; retries/resources.

`switch succeeded`, `tunnel up`, declaration UI и egress probe — action/path
receipts, не task endpoint. Task receipt остаётся отдельным минимальным input.

### 3.2 Категории mature episode

| Код | Определение к `D` | Terminal guard |
|---|---|---|
| `P` | task complete by D + protection producer receipt, чей **заявленный scope/interval покрывает именованный endpoint event**; нет explicit unprotected completion | Mere current snapshot/UI/tunnel/port не создаёт P; whole-operation protection всё ещё не следует |
| `U` | task complete by D после явно подтверждённого manual unprotected route | Никогда не прибавлять к protected success |
| `X` | task complete by D, protection at completion/overlap не идентифицирован | Task success известен, protected success unknown |
| `Q` | на horizon D подтверждён partial endpoint и full completion до D не установлен | Ранний partial до D — лишь observation/pending |
| `F` | на D подтверждено noncompletion либо до D получен contract-defined absorbing terminal failure | Раннее «пока нет» не F |
| `C` | user stop/cancel до D | Для system ITT — non-success; для latent any-route task — censored/unknown |
| `M` | task follow-up missing **или** timing maturity не установима | Причины `followup_missing`/`timing_unknown` публикуются отдельно |
| `R` | not yet due на interim cut | Только provisional; на final comparable cut R=0 |

`P` здесь намеренно узок: protection producer должен объявить и доказать scope,
который покрывает **именованный endpoint event/interval**, а не просто вернуть
fresh sample около ручной отметки. Current controller receipt в одной временной
точке не доказывает coverage фактического egress/business endpoint; при таком
разрыве completion относится к X. Для строгого operation-protected estimand нужны
task active interval и coverage receipt по нему; такого producer/caller сейчас
не установлено, поэтому этот показатель — `SOURCE_NOT_AVAILABLE`, а отсутствие
off-event не заменяет coverage.

### 3.3 Off→on overlap

Если пользователь выключил защиту, task отправил данные, затем защита снова
`applied` перед отметкой complete, endpoint snapshot может удовлетворить узкому
P, но операция не была доказанно protected. Поэтому:

- primary label обязан называться `endpoint-protected`;
- при известном overlap, влияющем на task, рекомендуется U; при неизвестной
  связи task interval с off/on — X;
- P нельзя повышать до operation-protected без отдельного bridge.

## 4. Формулы: fixed N, pending и unknown

Пусть после enrollment close roster фиксирован:

`N=P+U+X+Q+F+C+M+R`, `N>0`.

### 4.1 System ITT: endpoint-protected completion by D

System success = P. `U,Q,F,C` — известные non-success для вопроса «доставила ли
система endpoint-protected completion до D без abandon/manual-unprotected
resolution». `X,M,R` могут скрывать protected success.

```text
I_sys(cut) = [ P/N , (P+X+M+R)/N ]
```

На final cut с comparable clocks `R=0`:

```text
I_sys(final) = [ P/N , (P+X+M)/N ]
```

Это sharp worst/best-case hull при единственном допущении, что неизвестный
binary endpoint каждого X/M/R находится в `{0,1}` и категории/roster полны.
Это не confidence interval и не sampling inference.

### 4.2 Task resolved by any route by D

Known task completions = `P+U+X`; `Q,F` — known noncompletion к D; `C,M,R` —
unknown task completion для этого estimand.

```text
I_task(cut) = [ (P+U+X)/N , (P+U+X+C+M+R)/N ]
```

Final: убрать R. P/U/X всегда показываются отдельно, чтобы any-route summary не
скрывал manual unprotected outcomes.

### 4.3 Когда bounds тривиальны

- Если enrollment ещё открыт и future roster size ничем не ограничен — final
  cohort rate не определён; не публиковать число.
- Если registry receipts существуют, но пропущенное число units того же intended
  cohort не ограничено — для intended factual cohort `[0,1]`; можно честно
  сообщить только registered-receipt cohort.
- Если protection bridge отсутствует для всех completed episodes, system bound
  может стать `[0, completion-or-unknown mass/N]`, вплоть до `[0,1]`.
- Если `M+R` велико, wide bound — результат, а не повод imputing zero или удалять
  строки.

Jiang–Ding v6 §2.1 подтверждает общий worst-case принцип для binary missing
outcome: без assumption о missing mechanism неизвестный член допускает крайние
0/1; bounds не требуют MAR, но часто слишком широки. Их paper не доказывает
нашу registration completeness или protection bridge.

## 5. Ручные opposing traces

### T1 — early-success denominator trap

Fixed roster N=2, одинаковые `t0`, D=60. На cut=10 один P в t=5, второй R.

- неверно: mature-only `P/1=1`, выдаваемое за cohort rate;
- корректно: `I_sys=[1/2,1]` по N=2;
- если второй к D становится F: final `[1/2,1/2]`.

Outcome finality первого не даёт analytic maturity для полного roster.

### T2 — early `not complete` / partial

N=1, D=60. На t=10 пользователь отвечает «пока нет» или partial, на t=40 task
завершается с endpoint-protection receipt.

- ранний ответ хранится как observation; episode остаётся R;
- итог на D — P;
- прежний RP04 (`not complete до D → F`) терял timely success и отвергается.

F до D допустим только для contract-defined absorbing failure, а не временного
состояния.

### T3 — off→on→complete и точечный sample

N=1. На t=20 explicit protection-off; task мог отправить данные; на t=40
protection applied; фактический endpoint event может лежать в `[t=39,t=42]`, а
ручная отметка complete приходит t=45.

- sample/receipt в t=41 без coverage contract для endpoint interval не создаёт P;
  completion относится к X;
- whole-operation protected тем более неизвестен;
- если completion явно достигнут через unprotected route — U;
- только producer receipt с заявленным scope, покрывающим endpoint event/interval,
  разрешает узкий endpoint-protected P.

### T4 — clock reset/reboot

`t0` записан в старом elapsed scope; после reboot cut_time нового scope, bridge
отсутствует. Episode остаётся в fixed N как `M_timing`; не исключается и не
становится mature по wall-clock догадке. System/task upper bounds включают M.

### T5 — user cancel

User stop на t<D.

- system ITT: C — известный non-success (`C` не в upper `I_sys`);
- any-route latent task resolution: outcome после stop неизвестен (`C` входит в
  upper `I_task`);
- одна строка имеет разные роли, потому что estimands разные, не из-за ошибки.

### T6 — retries и late success

Три channel retries внутри одной task goal сохраняют один episode/N. Success
после D хранится `late_complete`, но своевременный endpoint к D не переписывается.
Создание трёх denominator units завысило бы exposure к retry-heavy failures.

## 6. Privacy-preserving record appendix

Минимальный future producer (не production DTO):

```text
registered_episode_v2 {
  schema_version, cohort_id, random_episode_id
  task_class, horizon_class
  clock_scope_id, t0_elapsed, deadline_elapsed
  registration_receipt_state
  observations[] { closed kind, elapsed, provisional|absorbing }
  task_at_D { complete|partial|noncomplete|unknown, reason }
  protection_at_completion { applied(scope)|off_manual|unknown }
  system_state { action_success|action_failed|unknown, user_stop }
  resource_at_R { retired|retained|unknown }
  maturity { due|not_due|timing_unknown }
}
```

Не записывать domains/raw URLs/IP/SNI/SSID/package/account/workplace inventory,
content, credentials, stable device identifiers или raw exceptions. `clock_scope`
должен быть локальным случайным boot/process scope, не глобальным fingerprint.

Enrollment требует локального явного consent. Withdrawal/deletion нельзя
переопределять в success/failure: если разрешён минимальный tombstone receipt,
он остаётся C/M; если consent требует удаления и даже completeness receipt нельзя
удержать, report обязан объявить roster incomplete и bound для intended cohort
может стать `[0,1]`.

Default report — локальные counts/bounds/reasons. Ручной export использует
существующий pattern: frozen immutable preview → exact displayed bytes → system
chooser. Никакой auto upload. Episode rows — отдельный opt-in, aggregate default.

## 7. Current source/caller и reuse

### 7.1 Snowden Android source

Reviewed exact working bytes (SHA-256 в artifacts) при repository anchor
`413621d…`; файлы были untracked, поэтому commit не аттестует их содержимое.
App manifest: `0.1.0-dev`, compile/target36, min29.

- `SessionJournal.kt:46–112`, SPDX GPL-3.0-or-later: process-local ring128,
  `SystemClock.elapsedRealtime`, sequence, evicted/recordingLimited; комментарий
  прямо запрещает считать его signed audit/VPN proof. **Adapt** typed bounded
  facts/clock vocabulary. **Reject as cohort registry:** no disk/background/task
  outcome; Clear/process death and semantic dedup break completeness.
- `ControllerReport.kt:25–82`: typed allowlist, frozen source fields, counts,
  unknown ages and explicit “not ... VPN proof”. **Reuse** aggregate/export format.
- `ControllerReportStore.kt:13–95`: one memory-only frozen frame, exact displayed
  share revision, 10-minute access expiry (not physical wipe). **Reuse/adapt**
  immutable manual export, not durable cohort storage.
- `MainActivity.kt:110–121`: foreground/focus guard, `textForShare(preview)`,
  `ACTION_SEND`, dispatch ≠ delivery proof. **Reuse** explicit manual boundary.

Цена нового fixed registry: durable accepted IDs until `C*`, migration/clock scope,
eviction/tombstone accounting and privacy review. Это больше текущего ring и не
следует из него бесплатно.

### 7.2 OONI Probe CLI reference

Exact tag `v3.31.0`, commit `1605e004…`, GPL-3.0, current captures/hashes saved.

- `oonirun/experiment.go:192–216`: local saver and submitter controlled separately;
- `oonirun/submitter.go:21–64`: disabled submitter is no-op stub;
- `nettests.go:160–167`: network submitter only under `Sharing.UploadResults`;
- CLI `upload.go:9–16`: named upload command is not implemented.

Decision: **REFERENCE/ADAPT only**, no code copy. Useful boundary is local save
versus explicit network side effect. It does not provide our task schema,
fixed-cohort maturity or ready manual export; GPL-3.0 and broader sensitive
measurement payload raise integration/privacy cost.

## 8. Scope verdicts и unknown

### A — cohort/maturity specification

**Recommendation to lead: ACCEPT_CORRECTED_FOR_INDEPENDENT_REVIEW** for
`R28-Z09-A`, scoped to registered fixed cohort, endpoint-protected ITT and the
explicit producer assumptions above. It resolves the exact NEEDS_REVISION:

- outcome-independent maturity;
- Q/F only at D or absorbing by contract;
- timing unknown remains denominator;
- P is endpoint-protected, not whole-operation protected;
- C differs across system/task estimands;
- formulas include pending/unknown correctly.

Not product ACCEPTED and not a measured SLO.

### B / OSS / pilot

**DEFER/ADAPT**, not accepted by analogy. Privacy schema is a bounded proposal;
current journal cannot serve as registry/task producer. OONI is reference-only.
Any user/device pilot remains NOT_RUN and requires owner/privacy/resource/consent
review plus synthetic schema checks.

### Remaining unknown

- Meaningful numerical D/R per task class;
- durable registry owner across reboot and lawful deletion/tombstone rules;
- actual task-level producer and strict operation-protection bridge;
- eligibility/participation selection beyond registered cohort;
- sampling uncertainty, comparisons/multiplicity and causal policy effect;
- user burden/effect and legal compliance.

## 9. Paired review reconciliation

At the final review stage, paired HANDOFFs became available; only strongest
relevant claims were read.

- **C03:** independently distinguishes complete roster, current gates, historical
  loss claim and future bridge; absence of prospective bridge can leave target
  `[0,1]`. This agrees with Z09's registry/bridge separation. C03's statistical
  kernel claims are outside Z09's verdict and were not adopted.
- **C06:** independently proposes fixed N and outcome-independent maturity and
  gives the stronger protection counterexample: a sample at one time does not
  cover an endpoint interval. Z09 incorporated this as a guard: point snapshot
  → X unless producer declares coverage. C06 uses a coarser `P/A/X/F/Z` accounting;
  Z09 retains `P/U/X/Q/F/C/M/R` because it separately exposes partial, cancel,
  missing and interim pending for the assigned question.

Agreement between authors is not primary evidence; manual argument/source guards
above remain the basis. No paired files were modified or messages sent.

## 10. Пройденные stages

- source/pin/caller: current project bytes + OONI exact tag/licence/callers;
- main argument: fixed unit/roster/horizon/maturity and two estimands;
- opposing review: T1–T6, including early success/failure, overlap, clock, cancel;
- reuse/licence/cost: Snowden GPL source and OONI GPL reference decisions;
- continuation: partial-identification bounds and triviality conditions;
- handoff: scoped verdict A separate from B/OSS/pilot.

Никакие tests/build/simulation/native/device/live/user experiments не запускались.
