# R28-Z03 — Регистрация до раскрытия: producer custody и полнота cohort

**Статус:** READY_FOR_REVIEW — предлагаемый scoped verdict, не решение ACCEPTED. 28 октября 2026.

## 1. Ответ и новая дельта

Регистрацию раньше исхода обеспечивает **контролируемая точка исполнения**:
подтверждённый durable registration/reservation предшествует любому действию,
которое может раскрыть данные данной проверки; повторный запуск требует
того же проверяемого operation identity либо новой явно связанной регистрации.
Полнота относится только к названной популяции, если все её пути запуска
проходят этот gate и записи сохраняются при отказах/потере подтверждения.
Библиотека хранения и hash-chain эти свойства producer самостоятельно не создают.

Сверх [R03-010](../../../03-ADAPTATION/REPORT-010.md) здесь установлен
**current caller**, а не только предложен список полей:
`appProtectedProbe.Run → publishObservationLocked → chanHealth → scoredCandidates`.
На этом пути есть ранний **in-memory** номер, защищённая publication и
физический admission, но нет доказанного durable roster всех попыток.
Значительная часть результата намеренно не входит в health. Это корректные
границы operational policy, не установленный дефект защиты.

Связанные продолжения TASK также пройдены:
1. commit мог произойти при потерянном ACK/ошибке sync; повторная регистрация
   с новым ID не доказывает отмену старой и не разрешает возвращать её бюджет;
2. recovery lineage/error accountant не восстанавливаются из одного
   self-consistent head. Совместный rollback журнала и accountant неразличим
   без отдельного retained witness или более узкой гарантии.

Выводы именуются **R28-Z03-A/B/C**, не глобальными R03-номерами.
Материал — source/spec и ручные контрпримеры; tests/симуляции/crypto/native/live
не выполнялись. Публичные спецификации и исходники читались по HTTPS,
значения logs/vault/runtime config не открывались.

## 2. Baseline и область исходников

### 2.1 Что уже принято, а что нет

[Рецензия NINE-R03-010-A](../../../reviews/NINE-2026-10-27-R03-010-A.md)
принимает **только** sum-union, A1/A2 счётную композицию, post-selection
контрпример и FWER сохранение поднабора покрытой семьи. Она не принимает
bbolt/confseq, всю модель кредитов, действующие producer inputs или полноту
журнала. [INTERVALS-AND-POLICY-EVALUATION](../../../../adaptation/INTERVALS-AND-POLICY-EVALUATION.md)
§§1–4 требует joint completeness/envelope, сохраняет пропуски в denominator,
отделяет label-corruption/фильтрацию/forecast bridge.

Здесь используется принятое доказательство, **не создаётся новая CS**.

### 2.2 Current availability/pin

SOURCE_AVAILABLE: основной checkout содержит Windows source и shared/adaptive.
`windows/go.mod` — `go 1.26`, sing-box `v1.13.19`, bbolt
`v0.0.0-20231014093535-ea5cb2fe9f0a` (:102), тот же bbolt в upstream
sing-box/go.mod:29. Локальный Go source reference VERSION — `go1.26.7`.
Это version/caller evidence, не происхождение запущенного EXE/AAR.

Windows caller main-package; protected probe/core package не смешиваются с
shared/adaptive API. Прочитаны нужные исходники, а не целиком platform tree.
Нет заявления, что current Windows вызывает API1 ComposeEvidence для этого
потока: здесь health/scoring — **реальный** consumer, API1 — contract reference.

Приватные public-source captures: `.artifacts/research/r28-contracts/Z03/`.
Первичные 36 scoped inputs сохранены последовательно с SHA-256; это не
атомарный снимок всего дерева. Свежие remote/cache сравнения шести bbolt
файлов совпали, включая LICENSE. Остальной closure не объявлен remote-verified.

## 3. R28-Z03-A — actual producer chain и неполнота статистической проекции

### 3.1 Source trace

| Шаг | Точные места | Что установлено / граница |
|---|---|---|
| Перед основной probe | `windows/probe_observation.go:84–128` | ObservationID=atomic.Add до factory/Run; owner/session/epoch и times прикрепляются defer. Это allocation в памяти, не запись durable registration |
| Physical admission | `backend/core/probe.go:153–185`; `diagnostic_budget.go:44–93` | Валидация, резерв всего declared request/body workload до goroutines. При refusal нет I/O. Requests/body reservation не возвращается на fail/cancel; active slot release отдельный |
| Выполнение children | `probe.go:195–231`, `probe_dns.go:41–79` | Fixed slots, WaitGroup wait; DoH fallback хранит returned attempts в bounded array. Child resolver identity — index в одном returned report, не отдельный durable admission |
| Результат после cancel | `probe_observation.go:99–128`; `corebridge.go:633–645,723–733` | Defer делает cancelled/stale failure; watcher/fast-reprobe могут закончить без health publication |
| Publication | `observer_publication.go:41–121` | Subject/owner/intent/epoch/clock checks; ordinary older IDs отклоняются, positive unsafe не игнорируется только из-за newer ordinary report. In-memory commit, без log persistence под lock |
| Health | `observer_publication.go:117–121`; `chanhealth.go:115–192` | Только определённые phases; measurement-only failure не health FAIL; transient failures до recovery-серии не счётчик individual failures |
| Retention | `chanhealth.go:225–229`; `manager.go:79–90` | Epoch reset заменяет items/events; Manager хранит последний cloned report, не lifetime ledger |
| Consumer | `scoring.go:198–228`; `failover.go:467–486` | Epoch-local stateOf → ranking, затем separate observed-switch reservation. Score не p-value и не CS; guards не доказательство sampling model |
| Human log | `app.go:107–139,171–191,958–991` | Async queue1024 и tail500, drop-on-full, silent write/flush errors, нет sync-ACK. appendLog не conditional durable gate перед измерением |

**Scoped verdict A:** source/model тезис «ранний ObservationID и protected
publication доказывают complete pre-outcome cohort» **отклонить**. Код
подтверждает правильность части operational subject/order checks, но не этот
усиленный статистический контракт. Не предлагать убрать safety фильтры для
улучшения denominator: нужна отдельная projection всех admitted attempts.

### 3.2 Scheduled/admitted/started/returned/mature/retired — разные события

| Событие | Смысл в предлагаемом контракте | Чем не подменять |
|---|---|---|
| Eligible/scheduled | Намерение/roster единиц до assignment или очередной возможности | Запуском; watchdog skip из-за StartBusy не completed attempt |
| Registered/admitted | Сохранена точная unit/claim/config и budget reservation; разрешён controlled dispatch | Atomic ID или строкой после I/O |
| Dispatch-authorized | Сохранено право одного bounded dispatch для operation/attempt | Доказательством, что syscall реально состоялся |
| Started | Owner подтверждает фактический вход/вызов измерителя; полное физическое wire execution может быть unknown | Dispatch ticket или timestamp до вызова |
| Returned | Известен actual return owner, результат может быть missing/stale/error | Cancel request/timeout/HTTP200 headers |
| Mature | Наступил заранее заданный follow-up horizon или outcome определён по его модели | Returned: возврат не обязательно полезное завершение задачи |
| Retired | Подтверждён возврат именно учитываемых ресурсов/обязательств | Publication/CS/cancel; локальное return не proof прекращения работы удалённого сервера |

Это не линейная enum: `mature=true` может сосуществовать с owner_return_unknown,
а `returned=true` — с retirement_unknown. `cancel_requested` — отдельный flag,
не terminal outcome. `core.AdmitDiagnostic` подтверждает application reservation,
не kernel/remote-task/energy bound. В проверенном ProtectedProbe goroutines
дожидаются wg.Wait до return/release; это сильный противоположный guard против
blanket «всё отпускается сразу по deadline». Kernel/remote эффекты им не измерены.

### 3.3 Три популяции, три разных completeness требования

1. **Logical probe cohort:** одна unit = один вызов Run с исходом к horizon.
   Успех/отказ children и retry rule — внутренности объявленной unit. Roster
   должен хранить каждый admitted logical call, включая cancelled/unpublished.
2. **ITT/assigned workload:** единицы eligible/assigned **до admission**.
   Отказ storage/resource gate и skipped scheduled call должны иметь заранее
   заданный outcome/interval или явное исключение из target. Сам admission
   может зависеть от сети/нагрузки, поэтому admitted-only sample не ITT effect.
3. **Physical attempts/resources:** children, repeats/HTTP dispatches и
   actual bytes. Один logical unit может содержать несколько sends.
   На source Go1.26.7 `Transport.shouldRetryRequest` (:818–862) допускает
   отдельные retries на reused connection; RFC9110 §9.2.2 разделяет intended
   idempotent effect и повторные side effects/log entries. Наш HTTP trace хранит
   последний stage/elapsed, не полный send counter. Наличие условий retry
   **не означает**, что они фактически сработали на конкретном endpoint.

Нельзя восполнять physical denominator числом logical observations или
planned maximum requests. Если physical footprint важен, нужен separately
scoped witness/bound, а не округление counters до фактического трафика.

## 4. R28-Z03-B — минимальный custody protocol и crash/retry cuts

### 4.1 Условия доверия, не поля ради полноты

Гарантия «до раскрытия» относительна: какой controller/analyst наблюдает какие
данные. Сохранить запись до получения HTTP body недостаточно, если controller
уже видел response timing, частичный header или результат предыдущей
эквивалентной проверки, на котором выбрал current hypothesis.

Достаточный bounded protocol, **проектное предложение**:

1. Назвать cohort/family, unit, outcome horizon, data filtration и все
   controlled launch entrypoints. Если полезная задача восстанавливает outcome
   из внешнего user observation, его registration gate — иной scope.
2. Единственный cooperating registrar сериализует admission. В одной storage
   transaction: immutable operation ID/attempt ID, target/model/config,
   reference доступной history, statistical reservation, physical reservation
   и membership в конкретном cohort. Ни I/O probe, ни callback раскрытия в этой
   transaction нет. Запись не даёт права на сеть при отсутствии platform permit.
3. **После** успешного storage acknowledgment owner проверяет current intent/
   generation и physical/safety admission. Gate не разрешает launch с
   unconfirmed registration. Guard на отмену до dispatch выполняется у того
   же dispatcher; race after guard ведёт к unknown started, не к удалению записи.
4. До внешнего запуска сохранить dispatch-authorized marker. Он не обещает
   exactly-once syscall. Crash между marker и action оставляет {started,not-started}
   множество; нужен owner reconciliation или «unknown execution», не автоматический
   новый запуск под тем же marker.
5. Returned/partial/cancelled/stale/missing callbacks относятся к той же
   operation. Safety policy вправе не публиковать их в live health, но accounting
   projection сохраняет существование unit. Точный duplicate callback не новая unit;
   conflict одного ID/результатов → provenance error, а не last-write-wins success.
6. Новый **измерительный** retry — child attempt, зарегистрированный до нового
   disclosure, либо разрешённая часть заранее замороженной logical test strategy.
   Повтор доставки прежнего сообщения — не новый sample/claim. Два варианта
   требуют явно разных identity и budget semantics.
7. До query publication дать snapshot cohort membership и отдельно frozen
   health/task/resource projections. Bounded eviction pending entries допустим
   только с retained count/interval obligation; silent eviction → completeness_unknown.

При честном enforced registrar, непротиворечивом durable storage и закрытых
launch paths этот порядок гарантирует existence registration перед controlled
раскрытием. Сам журнал не доказывает эти предпосылки против compromised producer.
Это architectural induction по launch gate, а не «trust из собственного SHA».

**Точная форма инварианта.** Для каждого controlled logical start s должна
существовать единственная registration r с тем же immutable operation identity,
которая принадлежит текущей retained lineage и для которой
`durable_commit(r) ≺ acknowledged(r) ≺ authorize(s) ≺ first_relevant_disclosure(s)`.
Знак ≺ — причинный/happens-before порядок в названном dispatcher, не просто
сравнение wall timestamps. При unconfirmed commit/lineage authorize запрещён.
Для нового logical retry требуется новый child identity или заранее declared
intra-claim стратегия; повтор доставки r не создаёт новый start.

База индукции: до первого authorize нет allowed controlled starts. Переход:
единственный permitted dispatcher запускает новый s только при наличии r;
запись не удаляется после return/cancel, следовательно после каждого шага
каждый allowed start имеет retained ancestor r. Recovery сохраняет инвариант
только при проверенной lineage и неповторении старых dispatched прав.
Отдельная предпосылка — все реальные launch paths проходят именно dispatcher;
без неё индукция говорит лишь о controlled starts, не о всей системе.

Обратная импликация неверна: registration/authorization может существовать без
реального start. Поэтому registry даёт complete **admitted** cohort при этом
контракте, но не точный размер **actually-started** cohort после crash. Для
target started-only нужен execution witness либо membership uncertainty.

**Membership uncertainty не равна одному пропущенному outcome.** Пусть c>0
confirmed starts имеют true loss sum в [L,U]⊆[0,c], а ещё u registrations
могли начаться, но это неизвестно. Если все эти возможные единицы принадлежат
тому же started-only target и loss∈[0,1], его finite realized mean имеет hull
`[L/(c+u),(U+u)/(c+u)]`: минимум достигается, когда все u начались с loss0,
максимум — когда все u начались с loss1. Для возможного меньшего k≤u нижняя
граница L/(c+k) выше, верхняя (U+k)/(c+k) ниже. При c=0 возможен empty cohort,
для которого среднее не определено; нельзя возвращать fabricated success0.
Это ручная partial-identification оболочка, не CS. Если target — admitted/ITT,
membership уже задан registration/assignment, а not-started outcome должен
быть определён заранее иначе: менять denominator в started-only по итогам
запуска без этой оговорки — selection, не исправление пропуска.

### 4.2 Failure traces: проверить сильнейшие противоположные случаи

| Trace | Корректная запись/действие | Что нельзя заключить |
|---|---|---|
| Ошибка записи до dispatch, никакой action не разрешён | Storage rejection/not-started; вернуть только доказанно неиспользованные local resources | «Network failed»; пропуск assigned unit из ITT |
| Durable commit завершён, caller потерял ACK | Resolve **тот же** ID/transaction obligation; не повторять измерение вслепую | Ошибка caller ⇒ запись не сохранилась |
| У bbolt meta WriteAt succeeded, fdatasync вернул error | Ошибка commit/публикация unconfirmed; snapshot/recovery отдельно | Bytes точно отсутствуют или новый бюджет свободен |
| Crash после admitted commit до dispatch-authorized | Если protocol запрещает action без marker, record logically not-started при validated recovery | Если этот gate не enforced — тот же вывод недоказан |
| Crash после dispatch marker до started/returned ACK | Execution unknown; measurement reservation сохранён; bounded abstain | Marker ⇒ syscall/полезный outcome; повтор action безопасен по DB transaction |
| Cancel во время I/O | cancel_requested; local owner return/retirement отдельные witnesses; result interval если outcome missing | Cancellation освобождает всё или failure label всегда известен |
| Return после epoch/safety change | Accounting сохраняет original unit с unpublishable/stale tag; live publication gate остаётся | Новый epoch делает исход новой independent unit |
| Initial fail → fast-reprobe успех | Несколько logical attempts либо один заранее определённый episode с retries | Health.OK/FAIL восстановит complete attempt roster |
| Duplicate delivered callback | Same ID idempotently recognised, payload conflict visible | Два callbacks = две independent successes |
| Crash/restart сбрасывает in-memory sequence | Recovered durable family+cohort lineage с process-independent key | Номер 1 новой памяти = новый lifetime бюджет или unique old callback |
| Storage corruption/cachefile reset | Keep unconfirmed state, no inference/optional new measurement; recovery/family scope проверяется отдельно | Пустая DB = честный новый старт старой семьи |

На конкретном caller `watchCore` после cancellation возвращается до publication,
`fastReprobeAfter` возвращает nil при cancel/смене session. Поэтому вероятность
попасть в observed-health зависит от lifecycle и retries. Это не missingness-
модель для CS. Cancellation может менять сам task outcome; считать censored
без определения target столь же неверно, как всегда заменять outcome на zero.

### 4.3 Что alpha rule даёт этому disclosure process

Используем принятое [NINE-R03-010-A](../../../reviews/NINE-2026-10-27-R03-010-A.md):
для каждого допустимого P и potential admission j

\[
 E_P[\mathbf1_{E_j}\mid\mathcal H_j^-]\le A_j\alpha_j,
 \qquad\sum_j A_j\alpha_j\le\delta\text{ a.s.}
 \quad\Longrightarrow\quad P(\exists j:E_j)\le\delta.
\]

Здесь E_j — **любая** ложная декларация в разрешённой процедуре, не only
reported health success. Durable reservation поддерживает accounting A2,
но ни registration marker, ни slot release **не доказывают A1**. Filtration
включает данные реально доступного scheduler, включая timing/partial reveal,
если используются при решении.

**Ручной opposing case:** два independent true-null uniform p_1,p_2;
выдать IDs до запуска обоих, после reveal оставить меньший p и charge только
его α=δ. Полнота source roster есть, запись до reveal есть, но расход
фактически выбранной claim-процедуры не оплачен: error=1−(1−δ)^2>δ.
Вариант с registration **после** min selection нарушает ещё и chronology.
Следовательно complete pre-reveal roster не достаточен для conditional validity.

**Сильнейшая альтернатива:** если оба claims уже simultaneously covered с
общим δ, можно после раскрытия выбрать один covered claim: событие ошибки
поднабора содержится в общем. Заново начислять бюджет только за selection
не требуется. Если это один заранее defined sequential claim с retries,
нужна его anytime proof, а не new α на каждый retry. У stochastic data-dependent
продолжения корректность возможна; blanket-запрет retries неверен.

**Пропавшие исходы:** при n известном, f полученных binary losses, u missing
и k label errors использовать принятую sharp hull
[max(0,f−k)/n,min(n,f+u+k)/n]. Joint completeness/envelope с failure β
и conditional statistical guarantee α_stat дают union error≤α_stat+β
без independence. Pointwise witness каждого snapshot не даёт joint lifetime
β. Если число unrecorded attempts неограниченно, finite realized population
mean универсально может лежать в [0,1]. Журнал одинакового вида совместим
с миром «нет hidden launches» и миром «скрытый тысяче-кратный failure stream».
Предложенный gate нужен для исключения второго мира, не для вычисления SHA.

## 5. Продолжение: reset/recovery lineage и error credits

### 5.1 Три вида reset

- Health/net epoch reset: operation policy утрачивает старую freshness;
  source `chanhealth.resetLocked` это делает намеренно. Его корректность
  не разрешает reset статистического accountant.
- Statistical restart: новая procedure/version/hypothesis может получить
  **остаток прежней семьи**; старые incurred charges и declared errors не исчезают.
- Enrollment новой явно scoped семьи: отдельная guarantee допустима, но
  нельзя назвать две per-enrollment δ гарантии одной lifetime δ пользователя.

Пример independent two restarts с exact false-error δ: общая lifetime
вероятность 1−(1−δ)^2>δ. Without independence применяется sum bound;
«перенастроился network» или fresh consent не изменяют этот математический факт.

### 5.2 Identity и ambiguity

Для continuation key должен содержать retained FamilyID/CohortID/EnrollmentRef,
claim definition/procedure identity и долговечный admitted ordinal (или equally
strong unique identity). Номер process/epoch — отдельный presentation tag.
Перезапуск приложения не может самостоятельно создать независимый ID space
для уже доставляемых old callbacks. DB sequence alone при rollback теряет
монотонность; NextSequence соседствует с SetSequence и Delete.

**Ambiguous registration retry:** создать stable operation identity до записи;
после ошибки ACK повторить **lookup/reconciliation этого ID**, а не new action.
Atomic pair `registration + cumulative statistical debit` в одной DB transaction
не даст separated logical commit при стандартном storage contract. Но она не
атомарна вместе с syscall/probe; NoSync, disk failures, writer-return-unknown
и restore lineage — отдельные предпосылки.

### 5.3 Reclaim/refund

Conservative вариант — statistical charge не возвращается после любой
возможной disclosure/declaration; отсутствие significant result не refund.
Для admission, который **доказанно** никогда не мог исполниться/раскрыть
данные, возможна иная proved accounting scheme, но этот пакет её не реализует.
Physical concurrency return относится к actual owner return, bytes/rate credits
— к declared physical policy. Статистический conditional alpha и operational
resource count не взаимозаменяемы. Ранняя baseline ошибка «+1 ⇒ infinite
expectation» не используется и не повторяется.

### 5.4 Совместный rollback — неразличимость

Пусть архив содержит log/head/accountant prefix с расходом c. Мир W0:
это последний prefix, дополнительных проверок не было. Мир W1: после него
совершены дополнительные проверки с расходом d, затем все local bytes
восстановлены из того же архива. Restore-visible bytes совпадают.
Signature/hash проверки старого head могут пройти в обоих мирах. Выдача
remainder δ−c как будто continuation в W1 не оплачивает d.

Отсюда **ограниченный** вывод: из одного восстановленного локального prefix
невозможно различить эти два мира. Варианты: independent retained monotonic
checkpoint (не обязательно центральный сервер), более узкая declared
per-enrollment цель, либо statistical abstain при uncertain continuity.
Любой witness должен быть вне того же rollback domain и иметь названную
custody/freshness; бэкап второго файла рядом не независимый anchor.
Никакого нового подписного протокола/TPM/облачного сервиса здесь не предлагается.

## 6. R28-Z03-C — exact reuse: bbolt и smallest adapter, не ledger platform

### 6.1 Что именно доступно

SagerNet/bbolt pseudo-version из manifest, MIT LICENSE Ben Johnson2013
прочитан; db.go/tx.go/bucket.go/bolt_windows.go/internal/common/meta.go/LICENSE
получены по pinned raw commit ea5cb2fe9f0a и **6/6 bytes совпали** с local cache.
Проверены source API, не библиотека в runtime.

- DB.Update: db.go:825–856. Единственный writer, callback error rollback,
  Commit error возвращается. Регистрация и debit могут быть одной transaction.
- DB.Batch: :890–928,961–989. Callback повторяем при batch failure/solo retry;
  внешние launch/disclosure side effects внутри callback не допустимы contract.
- Tx.Commit: tx.go:137–223; write (:440–445) и writeMeta (:467–484).
  Dirty/meta writes+Sync проверены по source. Ошибка Sync после WriteAt
  **не даёт отрицательного доказательства сохранения**; OnCommit идёт после
  tx.close, не атомарно с registration ACK/OS effect.
- Windows fdatasync: bolt_windows.go:14–17 → File.Sync → Go Fsync →
  syscall.FlushFileBuffers. Официальная **прочитанная ru-RU** страница
  FlushFileBuffers: flush buffered data to device/channel, failure GetLastError;
  не универсальный обещанный hardware-loss SLA.
- NoSync пропускает sync: db.go:45–55/tx.go:441,478. Default sync требуется
  предлагаемому durable scope, но ни source default, ни наличие функции не
  доказательство надёжности устройства владельца.
- Windows flock: sentinel byte-range через LockFileEx (:19–61); он
  сериализует cooperating DB Open, не блокирует все writes всей файловой
  системы и не удостоверяет producer. ru-RU LockFileEx explicitly исключает
  mapped-view I/O из запрета доступа; сравнение важно для adversarial writer.
- DB.beginRWTx :741–773 берёт sync.Mutex без context; Open Options.Timeout
  применяется к flock, **не deadline каждой Update/Sync**. Timeout caller
  вокруг goroutine не доказывает остановку writer и его safe slot reuse.
- DB.View хранит byte slices только пока tx жив; long-lived readers задерживают
  page reclaim и mmap. Fixed log length не устанавливает full RSS/disk bound.
- DB.meta :1043–1063 выбирает highest **valid** txid из имеющихся meta pages,
  fallback на предыдущую при validation failure. Это crash-consistency
  mechanism, не external anti-rollback witness. Meta checksum — FNV64a
  (internal/common/meta.go:60–65), не authenticity/криптографическая цепочка.
- DB.Open (:165–198) в read/write режиме допускает создание отсутствующего
  файла через O_CREATE. Для first enrollment это нормальный примитив; для
  established family «файл пропал → Open создал пустой» не доказательство
  initial state. Adapter обязан различать first enrollment, validated recovery
  и missing established lineage до выдачи нового statistical budget.

### 6.2 Не использовать существующую cachefile wrapper как scientific ledger

Actual upstream caller sing-box@v1.13.19 `experimental/cachefile/cache.go`:
Open/unknown-bucket deletion :107–159; resetDB :205–214 удаляет файл и создаёт
новый после ошибок wrappers. Это intended cache management, но уничтожает
obligation нового complete ledger. Нет утверждения, что current Windows уже
пишет статистику в него или что этот код причинил observed потерю cohort.

### 6.3 Самый малый adapter contract

Не новый controller/service. Если platform owner выберет durable optional
measurement cohort, ему нужны лишь:

- existing diagnostic entrypoint перед dispatch → owned registrar;
- atomic record/debit, stable ID, acknowledged/unconfirmed результат;
- bounded dispatch permit + explicit not-started/unknown/returned record;
- immutable outcome/events и scoped snapshot membership;
- retained writer-operation slot для pending/ошибки, не fire-and-forget write;
- accountant continuity at restart/restore или abstain.

`windows/backend/filetxn/BoundedWriteSlot` (:18–110) уже выражает useful
pattern: оригинальный writer retained при unconfirmed publication/Close;
Check ожидает ordinary active writer и отвергает uncertainty, без hard deadline.
Это **существующий local constraint**, не готовая bbolt transaction wrapper:
он пишет public files через собственный implementation. Брать только принцип
original-owner/retirement; не подкладывать его path publisher под bbolt DB.

Storage отказ не должен остановить P0 safety обработку или запретить ручной
Disconnect. Строгий pre-registration gate нужен для **optional scientific
claims**, а mandatory protection наблюдения продолжаются как operational
facts с completeness_unknown/непригодностью для inference. Outcome logging
не является новым network permit.

Цена: sync latency, writer lock wait, finite pending/metadata/retention caps,
copy frozen values, failure reconciliation и privacy. Ни абсолютный disk/RSS
cap, ни WCET из этих API не выведены. Если существующий platform storage
может удовлетворить контракт, предпочесть его адаптацию; bbolt — условный
reuse, не разрешение создавать новую БД.

## 7. Hash-chain и реальные proof scopes

RFC9162 (Laurie, Messeri, Stradling, December2021, **Experimental**) §§2.1.3–4:
inclusion — присутствие листа в заданном tree head; consistency — совпадение
старого prefix в двух деревьях. Это не specification нашего log и не
тождество Merkle tree и linear hash-chain, а проверенный пример разделения
membership/append-only/source completeness. §11.3 требует comparing views
для misbehaving logs; §11.2 log сам не обнаруживает misissue.

Хэш-цепочка, привязанная к независимо сохранённому trusted head, может
подтверждать неизменность committed prefix при cryptographic assumptions.
Она не доказывает: все launches прошли gate; timestamps созданы до reveal;
producer labels верны; restore не откатил head вместе с log; две replicas
не публиковали различные views. Даже «no gaps» у выданных IDs — только
internal sequence; hidden launch может вообще не получить ID.

Ручной indistinguishability example §5.4 и current source filtering §3
достаточны для ответа, новую crypto implementation писать не требуется.

## 8. Opposing review и рекомендуемый verdict

**A — предложить ACCEPTED только в source/model scope:** current chain
preallocates in-memory observation, содержит реальные guards/physical reservation,
но its health projection не complete durable cohort. Исторические live safety
утверждения/новые платформенные дефекты этим не устанавливаются.

**B — предложить ACCEPTED как conditional contract/negative information result:**
pre-reveal gate + preserved membership + stable identity при recovery
достаточны лишь при closed launch paths/storage/producer assumptions;
posthoc self-hash не подтверждает их. Joint rollback остаётся indistinguishable
без separately scoped witness. Accepted alpha theorem используется условно,
producer/order/cost не следуют из него.

**C — предложить conditional REUSE/ADAPT, не product acceptance:** bbolt raw
transaction API пригоден для minimal registrar при перечисленных guards;
cachefile reuse **REJECT** для данной цели; existing original-writer custody
pattern полезен, не ready turnkey ledger. Windows source/pin matching не
attestation EXE/AAR, Android implementation не researched.

Сильнейшие контраргументы включены: early ID действительно выделен до работы;
Probe WaitGroup не отпускает local goroutines по одному cancel; returned DoH
fallback attempts сохраняются; older unsafe signal имеет exception к order;
simultaneous family permits postselection covered subset. Ни один из этих
положительных guards не подменён blanket-заявлением «все результаты потеряны».

## 9. Work stages и конечная приёмка (всё future NOT_RUN)

Пройдены TASK stages1–4, оба продолжения (§30–31 TASK) и opposing/reuse/handoff.
Paired C03 HANDOFF при собственном review-stage09:13:49Z отсутствовал: не
ждал и не опрашивал соседей в loop; independent paired review остаётся pending.

| Future case | Что должен установить platform owner |
|---|---|
| C01 | Closed launch paths + registration before every relevant disclosure |
| C02 | Registration ACK error без actual action; assigned/rejected учёт |
| C03 | Meta write succeeds/Sync fails; same ID reconciliation, no blind retry |
| C04 | Crash до/после dispatch marker; preserve unknown unit/custody |
| C05 | Cancel after start/late callback old epoch; policy drop, accounting retained |
| C06 | Initial fail/fast recovery, repeated child sends и duplicate callbacks |
| C07 | Batch callback replay/OnCommit failure; no external action inside tx |
| C08 | Writer wait exceeds observer deadline; retained original slot, no fake return |
| C09 | Restore rollback accountant/head; no new lifetime δ без scope |
| C10 | Snapshot eviction/unknown H/label corruption; joint bound/abstain |
| C11 | Known complete family vs posthoc min choice; actual conditional p model |
| C12 | ACL/noncooperating mmap, copied View values, storage/device durability |

Эти случаи описаны только словами, не fixtures. Никаких sampled p-values,
симуляций crash, root keys, device operations или package changes.
Законченный ответ относится к доказательной базе и smallest adapter contract,
а исполнение/native/field safety, numerical models и реальные storage budgets
остаются unknown.
