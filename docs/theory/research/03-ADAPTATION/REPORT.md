# R03-001 — прослеживаемость адаптивного решения и пределы его оценки

**2026-10-25 · READY_FOR_REVIEW.**
Метод: source /
literature / ручной математический анализ, не реализация policy, эксперимент,
результат тестов или доказательство работы VPN. Источники: [SOURCES](SOURCES.md).
Снимки и SHA256: `.artifacts/research/theory-03/`. Дальнейший source drift требует адресной перепроверки.

## 1. Вопрос и отличие от уже известного

**Как связать наблюдение с конкретной рекомендацией, фактическим применением и
полезным исходом, сохранив честный знаменатель и цену неизвестности?**

| Уже закреплено | Что добавляет этот пакет |
|---|---|
| AD-01: context/scope/freshness join, trial ≠ ready | Описывает producer/consumer обязанности на всех границах, без второй authority |
| AD-02: C2 score collapse; поздние passes надо учитывать | R03-01: выводит точный lexicographic порядок, контрпример и достижимость class-aware ветки |
| AD-03: measurement origin/local pressure | Не добавляет синтетический CPU score; сохраняет unknown и отдельный resource outcome |
| AD-04: все missing outcomes/estimand; winner-only история недостаточна | R03-02: раздельный учёт action/service/resource; R03-03: конкретные конечные границы вместо удаления missing |
| AD-05: time-uniform uncertainty при названных assumptions | Разделяет uncertainty из-за неполноты и sampling error; интервалы не взаимозаменяемы |
| AD-06: safety first, переход дороже нулевого, один owner | Привязывает cost к эпизоду и факту исполнения, а не к boolean успеха callback |
| OP-04: journal не полный audit, StateFlow/ring могут пропускать события | Не повторяет этот диагноз; задаёт минимальный проверяемый accounting contract без нового logging SDK |

Это **новизна относительно текущего сборника**, не заявление новых мировых
алгоритмов. Дополнительно просмотрены named baseline memberlist/Envoy/confseq.
Ни один не даёт готового полного VPN observation→action→user-outcome contract.
Bandit/RL/change detector не добавлены: сначала надо определить наблюдаемый результат.

## 2. Фактическая цепочка в captured source

### Windows

1. `watchCore` / `netChangeReprobe`: current protected report, unsafe gates,
   bounded confirmation. Один report не устанавливает причину отказа.
2. `publishObservationLocked`: normalize, trust/source checks, текущие intent /
   applied config / session / running owner / epoch / observation-order guards.
   Небезопасный positive факт имеет специальный порядок; это не обычный score.
3. `tryFailoverWithObservation`: кандидаты **applied** descriptors, enabled /
   legacy live-verified / not current / present `ChannelIDs`; затем C2 → failure
   class stable sort → recent-attempt partition. Legacy label не fresh proof.
4. `reserveObservedSwitch`: current failure publication + scoped attempt token /
   cooldown. Выбранный target до этого момента не означает admitted action.
5. `switchChannelCoreFor`: повторная currentness, render/trust/lifetime, один
   Manager, selector-only change либо Reload. Fresh protected checks и publication;
   verified rollback — отдельная ветка возврата к прежнему пути.
6. `finishObservedSwitch`: связывает receipt и результат с текущим runtime;
   scoped completion не завершает преемника. `switched=false, blocked=false`
   возможно при rollback, obsolete work и отсутствии пригодного publication.
   Эти пары сами по себе не являются таблицей причин/статистикой качества.

Важные исключения: `tryPreferredChannelOnNetworkChange` выбирает transport
preference другим правилом; ручной выбор/Start и legacy no-report branch имеют
свои последствия. Единой формулой из пункта 3 весь controller не описывается.
Текущие guards уже существуют — предлагать второй owner вместо них нельзя.

### Android

`ConnectionState` и `PhysicalNetworkState` содержат intent/issue/OS context,
не protected user readiness. `VpnSessionRepository.completed` различает typed
RPC result; `WorkerRpcLane` удерживает pending Call до actual result. Его 10s
порог — предел ожидания наблюдателя, не подтверждение возврата Binder/producer.
`SessionJournal` хранит commands/state pairs в ring128 с dedup/eviction;
по нему нельзя восстановить все trials, причины неприменения или task completion.

Полный AND-07 adaptive measurement/ranking pipeline этими исходниками не доказан.
Нет основания переносить Windows ordering как уже существующий Android алгоритм.
Здесь только предложения для будущего подключения после настоящих platform gates.

## 3. Целевой контракт границ

Ниже **проектное предложение**, не новый DTO/SDK и не реализованная authority.
Использовать имеющиеся IDs/receipts; внешний research ID не становится runtime ID.

| Граница | Минимальное значение | Обязанность потребителя / запрет |
|---|---|---|
| Observation | Existing producer identity, owner/session/intent, applied profile+policy identity, physical context/epoch, observation ID, method/scope/family/workload, dependency, start/finish/expiry, typed origin/outcome | Unknown/not-started/stale не remote failure; arbitrary HTTP success не полный readiness |
| Evidence admission | Какие requirements факт покрывает, interval/context joins, hard gate status; отдельная допустимость trial/user traffic | Не заменять trust, OS protection или истечение полномочий score/CI/cooldown |
| Decision input | Bounded captured candidate set с exclusion reasons, evidence refs, policy revision, required workload, remaining budget/deadline, current path facts | Missing exclusion reason остаётся unknown; не подключать arbitrary candidate из журнала |
| Recommendation | stay/reprobe/diagnose/switch/recover/blocked; target если есть, полный priority trace, основания abstain, время/контекст пригодности | Recommendation не apply receipt; после ожидания платформа заново проверяет gates |
| Admission/execution | Existing attempt/operation token, proposed/applied identity, not-admitted reason, reserved work; platform-owned transition | Admission не completed; no second lifecycle owner; отмена потребителя не освобождает retained work |
| Decision disposition | Pending либо один известный конечный результат: not-admitted/retained/applied-verified/rollback-verified/failed-closed/superseded; неизвестность фиксируется отдельно | Нельзя получить candidate success из service recovery на прежнем пути |
| Outcome of service episode | Заранее заданный start/end/deadline, useful completions, failed/unknown outcomes, retained window/coverage, disruption/cost vector | Stage success/OS VALIDATED/controller HEALTHY не заменяют измерение пользовательской задачи |
| Resource disposition | reserved/admitted/started, фактические return/cleanup receipts и consumed work; unresolved отдельно | Просроченный resource owner удерживается платформой; learning record его не завершает |

Identity/epoch/scope обязательны **по смыслу**, но policy не должна получать
секреты, raw endpoints, package inventory или пользовательский payload. Платформа
передаёт bounded opaque identifiers/capability facts. Наличие одинакового ID не
проверяет подписанные полномочия; пригодность фактов подтверждает их владелец.
Отсутствие измерений не даёт права ослабить hard constraints.

## 4. R03-01 — поздние stable passes становятся старшими приоритетами

**Тип основания:** source fact + ручной вывод из stable sorting, P01/L02–08.

Для normal case, когда known failed descriptor и все candidate descriptors
представлены, значения comparator конечны, а stage keys зафиксированы для
рассматриваемого вызова:

```
K_C2(c) = (healthBucket(c), -score(c), d2Rank(c), originalPosition(c))
K_final(c) = (recentAttempt(c), countermeasureTier(c), K_C2(c))
```

В `recentAttempt` nonrecent=0/recent=1. Class tier меньше — выше.
Первый stable sort задаёт C2. Второй пересортировывает **весь** eligible list
по tier, сохраняя C2 лишь при равном tier. Последующая stable partition делает
recent старшим ключом, оставляя предыдущий порядок внутри каждой группы.
Это вывод о кодовой композиции, а не запуск сортировки или benchmark.

### Контрпримеры и границы достижимости

Описательные значения, не добавленные fixtures и не runtime reproduction:

| Состояние двух eligible кандидатов | C2 | После поздних passes |
|---|---|---|
| A healthy, recent; B failing, nonrecent; одинаковый tier | A перед B | B перед A из-за recent partition |
| A healthy TCP; B no_data QUIC; оба nonrecent; передан silent_freeze | A перед B | B перед A из-за tier0 против tier2 |
| Одинаковые recent/tier/health; отличаются independence | Выше score/I | Поздние passes сохраняют C2 |

**Важное ограничение:** current `ClassifyFailure` делегирует typed
`classifyObservedFailure`. Четыре special класса (`silent_freeze`,
`tls_policing_443`, `port_blackholed`, `reset_injected`) обычная прочитанная
typed цепочка не производит; для остальных `countermeasureTier` возвращает 0.
Поэтому второй контрпример описывает callable ordering/helper и риск будущего
producer, **не доказанную текущую авто-реакцию на timeout**. В ordinary typed
branch effective class key вырожден: остаётся recent → C2. Иные callers требуют
отдельного trace. Нет основания возвращать legacy inferred DPI labels ради
«активации» этого алгоритма.

Приведённые health/recent сочетания доступны интерфейсу сортировки; их частота
в production не измерена. Failing candidate не исключён из trials, и поздний
protected gate остаётся обязательным. Это расхождение между локальным C2
приоритетом и общим порядком, не доказательство unsafe bypass или ущерба.

`scoredCandidates` готовит объяснение **до** двух последних passes. Оно объясняет
C2, но недостаточно как доказательство «почему выбрали окончательный target».

### Предложение и цена

Сначала зафиксировать existing final baseline и ветки выбора, не менять веса.
Для будущей policy объявить: health — soft priority или барьер для trial?
Может ли work hint/heuristic tier его перекрыть? Из hard admissibility он никогда
не выводит. В зависимости от ответа **один** explicit tuple comparator либо
ограниченные passes внутри объявленных equivalence classes. Не менять порядок
молча под видом refactoring; сохранить old behavior как отдельный baseline.

Decision trace должен содержать stage winner/reason и конечный target, а не
склеенный первый score log. Reuse: existing Go sort/guards, P01–02; не свой sorter.
Цена: bounded per-candidate explanation и versioned priority contract;
развёрнутые списки не unbounded metrics labels. Выигрыш/новый порядок не измерены.

**Платформы:** TH-02/04/06 + COM-07 — Windows trace и legacy reachability;
AND-07/16 — специфицировать свой future comparator, без копирования Windows
эвристик; shared adaptive получает только recommendation contract. Серверной
реализации для этого вывода не требуется; dependency diversity поставляет R02/R04.

## 5. R03-02 — восстановленный сервис и удачный выбор канала имеют разные исходы

**Тип основания:** source-supported design + ручной accounting argument,
L03/L05/L06/L12/L13; P03–04 — reference об асинхронной отмене, не VPN SDK.

Current source уже отличает verified rollback от switched=true и сохраняет
currentness/attempt tokens. Новое требование — **не терять это различие при
передаче в policy, метрики и оценку**. Boolean reset circuit breaker (включая
discard/reset после obsolete observation) не является success label для обучения.

### Контрпример

Рекомендован B, fresh trial B не прошёл, A после rollback повторно подтверждён.
Обязательный выход:

- decision target=B; disposition=rollback-verified; applied outcome=A;
- кандидат B не получил positive evidence от успешного probe A;
- сервис может восстановиться на A; успех пользовательской задачи до D ещё
  требует собственного scoped результата;
- ресурсы B считаются затраченными, cleanup — только по actual receipt.

Если считать «после рекомендации снова HEALTHY» успехом B, будущая policy
получает положительное подкрепление за неудачный переход. Если удалить этот trial
из знаменателя, стоимость неудачных рекомендаций исчезает. Обратная ошибка —
считать failed B доказательством failure у восстановленного A.

Это логическое смешение единиц анализа, **не найденный уже работающий ML дефект**:
такой learner в просмотренном срезе не заявлен. Успех восстановления не доказывает,
что switch был необходим: A мог восстановиться сам во время probe/ожидания.
Нужен matched causal comparison, как уже требует AD-04.

### Закон сохранения учёта на фиксированном cut

Для process-scoped набора `N` зарегистрированных decisions каждому существующему
token сопоставляется ровно одна текущая accounting категория:

`N = pending + not_admitted + retained + applied_verified + rollback_verified
     + failed_closed + superseded + outcome_unavailable`.

Категории взаимоисключающие **по определению контракта**. Pending может стать
конечной категорией позже; нельзя увеличивать N повторным callback. Superseded
является disposition решения для attribution, а не обещанием завершения ресурса.
У resource inventory собственные `in_flight/returned/cleanup_unconfirmed`, связанные
с этим token: оно не выводится из суммы terminal decision categories.

Это не доказательство exactly-once доставки или crash-safe хранения. В пределах
retained process window существующий owner делает idempotent accounting update.
После eviction/recording exhaustion/restart нельзя восстанавливать отсутствующие
dispositions из последнего состояния: учитывать gap/unknown, не synthetic zero.
Bounded aggregates допустимы с явной saturation/coverage; durable full audit —
другая задача OP-04/R05, здесь не предлагается.

### Цена ошибки и единицы

Service episode начинается до admission, если цель — end-to-end recovery. Child
attempt остаётся вложенным; один retry episode не превращается в N независимых
outages, и успех одного child не закрывает чужие children/resources.

Хранить раздельный cost vector: недоступность в ms, failed useful tasks/count,
transition count, measurement bytes, actual occupied time, wakeups/energy при
наличии измерения. Складывать ms/bytes/energy без объявленной utility нельзя.
Для missed switch измеряется продолжение потерь на текущем пути; для false switch
— disruption и потраченная работа. Stay/abstain не обязательно бесплатны.
Safety/authority violations не становятся ещё одним конечным штрафом utility.

**Reuse/границы:** расширять существующие scoped receipts/typed bounded ring и
операционный owner, а не создавать второй actor/event bus. gRPC dependency не
нужна. Privacy: opaque process tokens, bounded reason enums; без URLs/IP/UID/keys
в metric labels. Цена — ограниченные counters/records и versioned outcome schema.

**Платформы:** TH-02/04 и COM-07/12 — existing observedSwitch/publication;
AND-07/11/16 — future action assessment поверх существующих WorkerRpcLane/repository.
Admission/OS state не Android protected proof. Backend/server может дать собственный
scoped stage result; он не определяет client task success. R06 задаёт useful task,
R05 — custody/cleanup invariants, этот пакет их не переопределяет.

## 6. R03-03 — отсутствие исходов даёт вычислимые границы, а не удобный процент

**Литература:** Imbens–Manski CWP09/03 §2–3 различает неполную идентификацию
bounded outcome и статистическую uncertainty. Эта работа не является VPN
испытанием. Ни её асимптотические интервалы, ни iid assumptions сети здесь не
переносятся. [Первичный текст](https://www.cemmap.ac.uk/wp-content/uploads/2020/08/CWP0903.pdf).

**Отдельное ручное конечное применение:** до просмотра результатов задан набор
`N>0` сопоставимых episodes, binary outcome `Y=1` означает полезное завершение
до фиксированного D. Есть `s` известных успехов, `f` известных неуспехов, `u`
неизвестных исходов, `N=s+f+u`. Для среднего именно этого конечного набора:

`s/N ≤ mean(Y) ≤ (s+u)/N`, ширина envelope `u/N`.

Доказательство: сумма неизвестных binary исходов лежит от 0 доu; прибавить s и
разделить на N. Обе крайние точки достижимы заполнением всех unknown нулями или
единицами без дополнительных ограничений. Возможные конечные средние дискретны;
интервал — tight envelope, не утверждение о непрерывном множестве всех значений.
Не нужны iid/случайное missing для этого accounting вывода о конечном наборе.

### Сравнение без выдуманных labels

При одном заранее заданном target наборе и совместимой интерпретации outcomes
для A/B, envelopes `[L_A,U_A]`, `[L_B,U_B]` дают
`mean(Y_B)-mean(Y_A) ∈ [L_B-U_A, U_B-L_A]`.

Пример **арифметики, не данные продукта**: из 100 эпизодов у A50 успехов и 50
unknown, у B80 успехов/20 неуспехов. Complete-case A=100%, B=80% ложно выглядит
однозначным преимуществом A. Envelopes A=[.5,1], B=[.8,.8], разность B−A
лежит в[−.2,.3]: знак не установлен. Больше повторов при неизменной доле missing
само по себе не сужает эту часть неопределённости; уменьшать нужно её долю или
вводить обоснованные ограничения, а не больше знаков после запятой.

Если B вообще не выбирался, его potential outcomes остаются unknown [0,1].
Никакой reweight/CI не создаёт их из победителей. Для causal сравнения нужны
определённые potential outcomes, consistency/coverage и разрешённый дизайн;
общий endpoint/сеть, время, probing effect и перенос состояния между trials
могут сделать простую A/B интерпретацию неприменимой. Не считать future benefit
доказанным даже при раздельных конечных envelopes.

### Что является unknown, а что уже определено целью

Timeout времени чтения — не известный полный RTT. Но если известно, что полезная
задача не завершилась к её D, её binary deadline outcome уже 0, не unknown.
Budget rejection реальной принятой задачи может означать 0 в end-to-end policy
оценке, но **не** отрицательный факт о remote channel. Неисполненный optional
probe не является принятой пользовательской задачей. User cancel/supersession
требуют заранее объявленного estimand/stratum; нельзя постфактум исключить только
неудобные cancellations. Evicted record без знания outcome — unknown, не failure.

Для bounded cost `Z∈[a,b]`, известной суммы `S` по остальным N−u строкам:
`(S+u·a)/N ≤ mean(Z) ≤ (S+u·b)/N`. Единицы a,b,S одинаковы.
Если cost не ограничен сверху, такой конечной верхней границы нет. Ограниченный
`min(recovery_time,D)` — отдельная метрика; она не доказывает, что recovery либо
cleanup действительно завершились за D. Нельзя выдать resource tail за 0 из-за
observer deadline. Safety нельзя нормировать в этот bounded performance cost.

**Решение:** сначала accounting R03-02, затем finite coverage/envelopes в offline
оценке; overlapping знак → inconclusive для optional optimization, не запрет
обязательного safety recovery. Для будущего population/future inference отдельно
нужен валидный sampling/time-uniform метод AD-05 и проверка drift. Эти конечные
границы **не confidence interval, не UCB скорости и не прогноз uptime**.

**Reuse/цена:** готовый принцип partial identification адаптируется арифметикой
без library; code из paper/confseq не копируется. Confseq пока reference, numerical
core не audited. O(1) counters на заранее ограниченную stratum достаточны для
envelope, но корректная классификация/coverage дороже самой формулы. Не взрывать
cardinality по URLs/user IDs. TH-06/FR-012/RQ-20 и AND-16 получают предложение
методики; AND-07/11 и TH-02/04 поставляют реальные исходы. Ничего не запущено.

## 7. Конечные будущие cases

Это **план приёмки**, не fixtures/тест-код/выполненные проверки. Уровень разрешения
и стенд выбирают владельцы. Ни один case не закрывает native/OS/live gate.

| Case | Вход / событие | Ожидаемая проверяемая граница |
|---|---|---|
| C01 | Eligible A healthy/recent; B failing/nonrecent, same tier | Exact old baseline B→A; новая policy не скрывает смену приоритета; fresh trial gate остаётся |
| C02 | Typed timeout вместо legacy silent_freeze | Нет invented class0 preference из причины DPI; known class helper case отдельно от producer reachability |
| C03 | Равные recent/tier/C2/D2, разный initial order | Объявленный stable tie-break и bounded explanation; deterministic при одинаковом captured input |
| C04 | C2 winner A, final target B | Trace показывает каждую перестановку; score-only rationale не считается полной причиной выбора |
| C05 | Target выбран, admission rejected из-за cooldown/budget | not_admitted, без remote failure/положительного результата, ресурсы учитываются по фактической стадии |
| C06 | После recommendation меняются epoch/profile/intent | Platform revalidation отвергает применение; old success не переносится в новый context |
| C07 | B failed, rollback A свежо подтверждён | B не success; service может recovered; task и resource outcomes отдельны |
| C08 | B applied, новый probe успешен, пользовательская задача до D провалилась | applied-verified не равен useful task success; полное N сохранено |
| C09 | Повторный callback/старый attempt после successor | Не удваивает N/disposition; не завершает новый owner; ресурс старой работы ещё удерживается при необходимости |
| C10 | Observer deadline, child/provider ещё выполняется | Resource pending/cleanup unknown, no replacement authority; не synthetic completed/failed channel |
| C11 | Ring eviction/recording limit/process restart/slow collector | Coverage gap видим; невозможно «восстановить» полный trace из state pair; no fake zero counts |
| C12 | N=0; u=0; u=N | undefined mean; exact finite average; envelope[0,1] соответственно, без division-by-zero/fake precision |
| C13 | A:50/0/50; B:80/20/0 на общем N=100 | Envelopes[.5,1] и[.8,.8], delta[−.2,.3], inconclusive; это ещё не future CI |
| C14 | D deadline известен, завершения к D нет; raw duration отсутствует | Binary0, duration censored; не unknown binary и не RTT=D |
| C15 | B ни разу не выбран; dependent/no coverage trials | Неидентифицированные counterfactuals не получают synthetic propensity/CI benefit |
| C16 | Quiet + hard revoke; optional benefit inconclusive | Hard policy выполняется; optional optimization может stay; abstain не маскирует safety failure |

## 8. Последовательность адресных предложений

Новые product IDs не создаются, статусы существующих работ не меняются.

1. **После P0 platform safety/trust gates**, TH-02/04 и AND-07: согласовать
   минимальный observation/action/outcome vocabulary и повторные gates на границах.
   R05 supplies hard invariants, R06 task/deadline, R04 measured cost; неизвестные
   поля остаются unknown. Общий adaptive код выдаёт советы, платформы исполняют.
2. **TH-04/COM-07, AND-07:** сохранить exact current baselines и decision trace;
   explicitly decide priority tuple/reachability, затем accounting R03-02.
   Android не обязан иметь Windows legacy states/классы.
3. **TH-06/FR-012/RQ-20, AND-16:** заранее выбрать episode/denominator/strata,
   missing/cancellation policy, cost units и gain margin; envelope/coverage до
   confidence, без переноса старых PASS на новые source.
4. Только при разрешённой оценке: deterministic cases → scoped platform gates →
   matched comparative study и один выбранный improvement. Shadow observation
   не знает исход неисполненного действия; canary не исправляет отсутствие coverage.

R02: вопрос о provenance/failure-domain различии кандидатов, без нашей новой
теории цензора. R04: стоимость холодного перехода/нагрузки и полезный workload.
R05: какие receipts разрешают apply/release, с которыми связывается outcome.
R06: единица service episode/deadline и цена ошибочного/запаздывающего решения.
Эти вопросы зафиксированы здесь; сообщений соседним чатам нет.

## 9. Предлагаемая вставка в общую теорию

**Предлагаемое дополнение к §17.8: прослеживаемость решения.**

> Финальное ranking правило следует выводить из всей композиции фильтров и
> stable passes. В рассмотренном Windows failover late recent-attempt partition
> старше class tier, который старше C2 health/score/D2. В обычной текущей typed
> classification class tier одинаков: legacy censorship classes не производятся
> этим classifier. Это source algebra, не полевой диагноз или доказательство
> ущерба. Будущая policy должна явно определить допустимость и старшинство
> признаков; rationale фиксируется после всех проходов.
>
> Рекомендация, её admission, applied/rollback outcome, полезное завершение
> задачи и возврат ресурсов — разные факты. Verified rollback на A после неудачи
> B может восстановить сервис, но не является успехом выбранного B. Existing
> owner/intent/session/epoch/attempt receipts связывают эти факты; obsolete work
> не получает credit нового поколения. Для retained process window все decisions
> имеют accounting disposition либо pending/unknown с явным coverage gap.
>
> Для заранее определённого конечного набора N=s+f+u binary deadline outcomes
> tight lower/upper envelope равен[s/N,(s+u)/N]. Это адаптация partial identification
> к учёту эпизодов; формула не требует случайного missing, но не является CI,
> прогнозом или causal преимуществом policy. Окончившийся deadline известной
> задачи без завершения означает binary0; невыбранный канал остаётся counterfactual
> unknown. Неопределённость данных отделяется от sampling/drift и hard safety gates.

Ссылки для этой вставки: source pointers L02–08/L12–13, P01 и P05 из
[SOURCES](SOURCES.md). Точные входы handoff сохраняются отдельно.

## 10. Что установлено и чего пакет не доказывает

Установлены scoped source composition и аналитические контрпримеры; предложен
проверяемый accounting contract и выведены конечные bounds. Не измерены частота
нежелательных switches, latency/throughput/energy/RSS, coverage реальной сети,
причины российских блокировок или superiority новой policy. Нет внедрённого
shared SDK, Android adaptive pipeline либо доказанной exactly-once/durable истории.
Current source snapshots без runtime проверки не закрывают P0/platform gates.
Самостоятельная реализация/смена dependency, CI/security scans, численные опыты,
VPS/live и управление другими чатами не выполнялись.

---

## 11. R03-002 — свежесть доказательств и выполнимость расписания измерений

**2026-10-25 · READY_FOR_REVIEW.** Второй исследовательский пакет. Первый пакет
R03-001/§1–10 и его handoff сохранены; научное принятие ни одному не присвоено.
Входы: `.artifacts/research/theory-03/cycle-002/`,30 captured local files,
6 external captures; provenance — [SOURCES §4](SOURCES.md#4-r03-002--свежесть-и-расписание).
Ниже два новых уточнения, R03-04/05; численные примеры вычислены руками.

### 11.1. Пробел, baseline и фактический source

AD-01 уже требует context/epoch/scope/validity joins, AD-03 различает detector
health, AD-06 сохраняет owner до actual return. R03-001 добавил disposition /
coverage. **Не разобраны** происхождение возраста отдельного факта при долгом
составном probe и совместная выполнимость freshness promises с admission caps.
Нельзя закрыть этот пробел одним «проверять чаще» или большим TTL.

Source facts нового cut, не результаты запуска:

- Windows `appProtectedProbe.Run` датирует итог через `started.UTC()` /
  `time.Now().UTC()`. HTTP sub-observation уже имеет свой StartedAt и elapsed
  `time.Since(started)`. `ProtectedProbe.Run` ждёт parallel egress/control/DNS,
  а DNS fallback делит оставшийся срок между sources.
- `publishObservationLocked` пишет health timestamp из report.CompletedAt;
  `healthState.verdict(now)` считает positive freshness от LastOKAt, максимум 2min.
  **Это age завершённого агрегата**, не автоматически возраст каждого sub-fact.
  `GetChannelHealth` и C2 используют этот health verdict.
- Watchdog period30s; два fast reprobes с gap5s; outer context15s, default
  `newAppProbe.Timeout=10s`. Admission protected lane:2 active whole operations,
  120 reserved requests и 1MiB reserved body bytes на rolling60s. Внутри одного
  operation есть fanout;2 — не предел всех sockets/HTTP requests приложения.
- Запрос резервирует `q = len(Targets)+len(resolverTargets)+control`, control=1
  при обязательном direct-control step. Все потенциальные DNS attempts charged
  заранее; early success/error/cancel уже admitted работы не возвращает reservation.
  Body cap исключает TLS/HTTP overhead, retransmissions и kernel buffers.
- Android `OsProtectionReader` уже датирует **начало** чтения через
  elapsedRealtime; max age5s provisional. WorkerProcessLink хранит отдельно
  receivedAt для IPC; WorkerRpcLane10s — observation deadline, не actual return.
- Windows trust использует отдельный ObservedClock/sessionLifetime и retained
  floor. Health age не заменяет этот clock, подписи, native/OS или expiry gates.
- При итоговом чтении параллельно обновлённого PLAN обнаружен **shared/adaptive
  API1 source**. Просмотрены Health/RankInput/healthVerdict/Rank/validation и API:
  время поступает от caller, RequiresPlatformGate сохраняется, complete rank trace
  уже описан/написан. Это развитие baseline, не доказанная consumer parity.
  Temporal/outcome join и budgets адресуются существующим ADP-004/003; API1 не
  выдаётся за их реализацию. Сам модуль `.UTC()` не вызывает: возраст зависит от
  clock provenance входов/адаптера, Windows counterexample не дефект любого caller.

### 11.2. R03-04 — возраст факта, задержка доставки и возраст отчёта различны

**Primary reference:** [Sun et al., Update or Wait, arXiv1601.02284v9](https://arxiv.org/html/1601.02284v9),
§III. Age of Information отсчитывается от generation доставленного обновления;
полученный update не имеет нулевого возраста из-за одного факта доставки.
Их optimization model предполагает известный stochastic service process и
stationarity/ergodicity; оптимальность этой модели к VPN не переносится.

**Ручное применение к составному измерению.** Пусть относящийся к required
scope факт получен в неизвестный момент `g_k∈[s_k,e_k]` по одному локальному
сопоставимому clock. Отчёт опубликован в p≥e_k, текущее время t≥p. Тогда:

`t−e_k ≤ age_k(t) ≤ t−s_k`.

Доказательство — вычитание двух границ g_k из t. Если вместо этого присвоить
факту age=t−p, недоучтённая часть равна p−g_k. Она может быть почти всей
длительностью report и его ожидания публикации. Генерация request не обязательно
момент наблюдения состояния, поэтому начало — консервативная граница, а не
вымышленный точный remote timestamp. DNS cached data freshness — отдельная OP-01.

**Контрпример, не измерение:** egress fact в[0,.1]s, DNS заканчивается в 10s,
весь report публикуется в 10s. Report age0s, возраст egress от 9.9 до 10s.
TTL5s для этого egress уже истёк на публикации, хотя агрегат только завершён.
Если семантика поля именно «когда завершён report», timestamp корректен;
ошибка возникает при его повышении до freshness всех более ранних фактов.

Для age threshold τ_k консервативное решение требует `t−s_k≤τ_k`, если нет более
точного обоснованного observation point. Для набора обязательных scope использовать
наиболее раннюю expiry обязательного факта, а не report max timestamp. Начало
общего probe допустимо как более грубая граница; может чаще давать unknown.
Три исхода age gate: `t−s≤τ` — возраст точно допустим в этой модели;
`t−e>τ` — точно старый; пересечение границы — uncertain, а не доказанный отказ
канала. Currentness/authority/protection проверяются независимо во всех случаях.
Любой TTL остаётся правилом пригодности информации: сеть может измениться через
микросекунду после успешного ответа. Малый age не доказывает неизменность пути.

#### Часы — конкретное source уточнение

Pinned Go1.26.0 `Time.UTC` снимает monotonic component; сравнение/Sub с таким
значением использует wall clock. Следовательно, текущий health age от UTC
report не является steady elapsed age. [Go source](https://github.com/golang/go/blob/d90b98e65320778f3b1f99a6951ab20f04d218b3/src/time/time.go).

Ручной counterexample функции verdict: последний успешный wall timestamp1000s;
steady прошло 130s, wall скорректирован назад на 60s. nowWall=1070s: positive age70s
≤120s, хотя elapsed age130s. При wall step за последнюю отметку existing guard
now.Before(LastOKAt) возвращает no_data; приведённый пример эту ветку не включает.
Это ограничение age для ranking/UI, **не воспроизведённый обход trust protection**.
Clock continuity/authority gates рассматриваются отдельно и могут остановить
действие. Нельзя объявить найденную утечку по одному этому вычислению.

**Proposed contract:** внутри producer хранить дату/интервал наблюдения на
подходящем elapsed clock, отдельные elapsed durations и wall time для отображения.
Объявить process/boot scope, suspend semantics и поведение при clock uncertainty.
Не сериализовать Go monotonic tick как portable timestamp и не сравнивать его
с Android elapsedRealtime. На Android elapsedRealtime включает deep sleep;
сам clock не гарантирует своевременного исполнения callback. [Android API](https://developer.android.com/reference/android/os/SystemClock).

**Reuse:** уже существующие HTTP StartedAt/ElapsedMicros, scoped identities,
Android start-dated OS observation; один clock adapter платформы. Не вводить NTP
service или внешнее время как новую authority. Code не копируется; Go stdlib
reference BSD-3-Clause, новый dependency не нужен. Для Windows suspend надо
отдельно подтвердить выбранный clock на целевой OS; generic Go caveat не является
проверкой Windows hardware. После reboot/owner loss прежние elapsed ages непереносимы.

**Цена и проверка:** чуть больше bounded timestamp metadata и версионирование
producer/consumer schema; conservative start age может давать лишний unknown.
Уменьшать это следует лучшими наблюдениями, не relabeling старого факта. TH-02/04,
COM-07 и AND-07/11 получают предложение; R05 определяет authority deadlines,
R04 измеряет overhead/clock/suspend behavior. Ни ready, ни trust TTL не продлены.

### 11.3. R03-05 — freshness target сначала требует проверки выполнимости

**Вывод:** период/timeout/admission cap по отдельности не дают upper bound возраста
измерения или времени обнаружения отказа. Нужны доступность producer, затраты,
успешные scoped completions и ограничения всех мест ожидания. Становление unknown
при просрочке — иной результат, чем сохранение свежего positive proof.

#### A. Необходимые ограничения нагрузки

Для разных непереиспользуемых типов измерительных jobs i заданы period P_i>0,
request reservation q_i и body reservation b_i. Если обещано регулярно **допускать**
все эти jobs, из rolling cap B_q/B_b на окно W следует long-run необходимость:

`Σ(q_i/P_i) ≤ B_q/W`, `Σ(b_i/P_i) ≤ B_b/W`.

Единицы — reserved requests/s и body bytes/s. Сумма charges за долгий интервал
растёт как T·Σ(q_i/P_i), а лимит пропускает не более T·B_q/W + краевой burst;
деление на T и предел дают неравенство. Это наш ручной necessary condition,
не sufficiency theorem. Одно измерение, покрывающее несколько requirements,
считается один раз; неизвестный reuse не выдумывается.

Пример арифметики: q=6, P30s, cap120/60s допускает по среднему не более 10 таких
потоков. Это не утверждение о текущем числе каналов и не рекомендация опрашивать
их все; byte limit,2 slots, competing protected work и неуспехи ещё не учтены.
Частые повторы одного failure также расходуют эти charges без нового success.

Средний load может укладываться, но одновременные deadlines и retained owners
сделают задачу невыполнимой. **Контрпример:**2 non-preemptible slots,3 jobs,
каждый занимает ровно 3s, общий release0/deadline5s. Total work9slot·s≤10slot·s,
но каждый slot успевает только один job: два подряд требуют 6s. Хотя агрегатная
capacity inequality выполнена, один deadline обязательно пропущен. Никакого
алгоритма/стенда для этого counting proof не запускалось.

[Liu–Layland1973](https://bears.ece.ucsb.edu/class/ece253/papers/laylandliu77.pdf)
доказывает deadline-driven результаты в объявленной preemptive single-processor
periodic independent constant-runtime модели. В нашем сетевом/Binder workload
эти assumptions не установлены; классическое EDF utilization condition нельзя
выдать за сертификат реального VPN. Это reference принципа deadline ordering,
не перенос scheduler/kernel code и не новое доказательство оптимальности.

#### B. Deadline потребителя не upper bound освобождения producer

Текущий protected pool не занимает новый slot при отказе в admission и не
возвращает charges после admitted error. Это полезные защитные свойства.
Но 2 occupied operations могут оставаться occupied, пока underlying calls реально
не вернулись; consumer timeout не даёт права обнулить active. Android lane явно
удерживает один pending Call. Значит, обещание нового наблюдения к E требует
либо доказанной доступности slot/возврата, либо честного «гарантия отсутствует».

В модели с bounded costs определить latest start:

`L_j = E_j − C_upper,j − J_upper,j − M_j`.

E_j — expiry/дедлайн результата по тому же elapsed domain, C — upper bound всех
нужных этапов после start, J — bound задержки доставки/публикации, M — явный margin;
всё в одинаковых единицах времени. Начало после L_j не имеет этой worst-case
гарантии. Неизвестные C/J не равны context timeout. Начало до L_j тоже недостаточно
без resource availability, совместимых фактов и оговорённых environment assumptions.
Required positive proof может не обновиться даже при идеально доступном scheduler:
неисправный endpoint не обязан дать success. Будущий contract обязан сказать,
что гарантируется: попытка, known outcome или fresh successful capability.

#### C. Реальный watchdog — условия, а не сумма констант

Pinned Go ticker может корректировать интервал/пропускать ticks при медленном
receiver. [Go1.26.0 tick.go](https://github.com/golang/go/blob/d90b98e65320778f3b1f99a6951ab20f04d218b3/src/time/tick.go).
Наш loop также пропускает ticks при startBusy/noncurrent/nonrunning и выполняет
confirmation последовательно. Поэтому 30s — configured opportunity cadence.

В идеализированной модели fault сразу после завершённой последней проверки,
свободный observer, следующий tick≤P, каждая из 3 проверок возвращается за C,
2 gaps≤g, без дополнительного ожидания: `T_confirm≤P+3C+2g`.
Подстановка 30/15/5 даёт 85s **только если C действительно ограничен 15s** и gaps/tick
укладываются. Код задаёт 15s context, default HTTP timeout10s, но не доказывает
верхнюю границу всей publication/lock/producer chain. Поэтому 85s не SLO и не
измеренный recovery. Длительный sleep/занятый owner разрушает предпосылки модели.

Собственный предел: если хотя бы один обязательный этап допускает произвольно
долгое ожидание, для любого заявленного конечного D выбирается исполнение с
ожиданием>D; из одних timer constants гарантированный T≤D не следует. Это
элементарный аргумент о модели, не утверждение, что зависание уже наблюдалось.

### 11.4. Предложение для bounded policy и reuse

После platform safety/trust gates, в рамках existing measurement owner:

1. Required proof set и age/deadline contract от R03-04; positive/negative/unknown
   остаются разными. Signed expiry, retire/revoke и защита не scheduling штрафы.
2. Получить current reservations/actual owners и remaining rolling budget.
   Не создавать work/goroutine на rejected admission. Optional work уступает
   required работе; existing protected/optional lanes сохраняются. Внутри protected
   lane справедливость/criticality ещё нужно специфицировать:2 slots её не доказывают.
3. Для допустимых queued requests рассмотреть earliest expiry/latest-start как
   **детерминированную эвристику**, не оптимальный multi-resource scheduler.
   Deadlines не заменяют remaining-byte/attempt caps и retirement precedence.
4. Coalesce только ещё не начатые эквивалентные информационные запросы с теми же
   subject/scope/epoch/semantics. Не стирать unsafe/revoke события, pending native
   operations или разные capability требования ради «последнего свежего».
5. Unknown feasibility/expired proof → typed age/coverage/admission result и
   platform-owned обработка. Никакого synthetic refresh/READY/direct fallback.
   После sleep/новой epoch повторные gates; не запускать storm накопленных ticks.

Готовое: shared/adaptive API1 как baseline contract и future ADP-003/004 seams,
без повторной реализации Rank/trace; Go time primitives и existing limiter; Android elapsedRealtime /
start-dated observation / retained RPC lane. AoI reference полезен для цели
freshness, EDF — для идеи deadline ordering. **Defer** SMDP/Markov optimizer:
нет оценённых conditional service distributions/stationarity/cost map; численная
реализация не проверялась. Paper license не является license готового SDK; код
из papers не копировался, приложения/библиотеки не устанавливались.

Стоимость предложения: bounded queue/descriptor/counter contract, измерение
stage costs и load, schema compatibility; частые probes сами стоят bytes/energy
и меняют путь. Числа C/J/battery предоставляет R04, полезную цель/quiet — R06,
hard invariants R05. Это не исследование нового transport или их failure domains.

### 11.5. Конечные будущие cases и адресная очередь

Все случаи ниже **не выполнялись**, это ручные спецификации для владельцев.

| ID | Вход / конкурирующий сценарий | Проверяемый результат |
|---|---|---|
| F01 | Один sub-fact рано, другой завершает report поздно | Отдельный fact age/граница, publication не renew прежний scope |
| F02 | Только общий start известен | Консервативная age bound либо unknown; не invented precise generation |
| F03 | Steady130s, wall−60s, TTL120s | Age не становится 70s из-за wall correction; UI projection не readiness authority |
| F04 | Serialize/parse `.UTC()` или process/boot replacement | Monotonic identity не выдумана; старый elapsed context непереносим |
| F05 | Sleep дольше age deadline, callbacks доставлены после resume | Freshness пересмотрена до применения; late callback не продлевает proof |
| F06 | Average request budget подходит, simultaneous3×3s jobs/2slots/D5s | Deadline feasibility не объявлена по 9≤10; admission/unknown сохраняется |
| F07 | Σq/P превышает cap либо исчерпан byte reservation | Обещание частоты отвергается; rejection не remote failure |
| F08 | Оба protected slots удержаны после consumer timeout | No replacement/phantom release; no guaranteed fresh outcome без реального возврата |
| F09 | Optional flood и обязательное обновление proof | Existing lane isolation плюс явная политика criticality, без бесконечного fanout |
| F10 | Частые admitted failures/ранний DNS success | Charges не возвращаются ради новой попытки; все admitted costs учтены |
| F11 | Coalescing разных scope / unsafe event среди positive facts | Отвергнуть unsafe coalescing; hard revoke/positive unsafe evidence не теряются |
| F12 | Подтверждение из 3 probes, пропущенные ticks/slow producer | Показаны actual start/return и duration,30s/15s не обещают bound85s |
| F13 | Scheduler успевает, но endpoint действительно не отвечает | Known negative/unknown не заменён fresh positive capability |
| F14 | Deadline и trust expiry истекают во время очереди | Повторная platform проверка, no authority extension от планировщика |

**Порядок предложений:** TH-02/AND-07 — temporal provenance/schema;
TH-04/AND-07/11 — deadline/admission/owner/criticality contract;
TH-06/FR-012/RQ-19/20 и AND-16 — actual costs/coverage/detection tails и comparative
оценка только на разрешённом стенде. Общий adaptive слой получает ограниченные
facts/advice; ADP-003 — budget effects, ADP-004 — temporal evidence contract.
Платформы исполняют и публикуют.

<a id="116-готовая-вставка-руководителю-и-границы-вывода"></a>
### 11.6. Предлагаемый вывод для общей теории и границы вывода

Предлагается после R03-001 либо как уточнение §17.1/17.3/17.8, без перезаписи
авторского материала:

> Возраст составного отчёта от CompletedAt не равен возрасту каждого измеренного
> факта. Для observation time g∈[s,e] на сопоставимом elapsed clock его age в t
> лежит между t−e и t−s; доставка не обновляет generation. Wall timestamps для
> отображения отделяются от bounded age/expiry с явными process/boot/suspend
> semantics. TTL обозначает пригодность информации, не неизменность сети.
>
> Freshness contract должен быть выполнимым при rolling reservations, занятых
> owners, publication delay и hard authority deadlines. Средняя Σq/P≤B/W — лишь
> necessary condition; deadlines/non-preemptible blocking могут нарушиться при
> допустимой средней нагрузке. Период watchdog/context timeout не доказывают
> фактический срок возврата/обнаружения. При неизвестной выполнимости алгоритм
> возвращает ограниченный outcome/coverage и сохраняет platform safety policy.
>
> Existing limiter/clock/owner gates следует расширять измеримыми temporal facts.
> Earliest expiry — candidate heuristic после safety, не перенос optimal EDF/AoI
> теорем в произвольную сеть. Никакое расписание не гарантирует успешный ответ
> неисправного endpoint и не продлевает полномочия протухшего proof.

**Установлено:** source clock/age/cadence/cap composition, ручные temporal и packing
контрпримеры, условия необходимой нагрузки. **Не установлено:** частота проявления,
вред реальным пользователям, оптимальный scheduler, замеры CPU/RAM/energy или
bounded Windows/Android native completion. Не обнаружен/не доказан security
bypass по clock counterexample.

**Input drift при завершении:** cut14:22:08Z выявил изменения только двух из
первоначальных 22 captured файлов: общий PLAN (Windows V2-158/source SDK request)
и ANDROID-PLAN (AND-12-S6 controller report UI). Их source-only updates просмотрены;
адресные TH-02/04/06 и AND-07/11/16 критерии не закрыты. Поздно найденные 8
adaptive source/docs закреплены отдельным capture временем в том же manifest.
Исследованные probe/clock/limiter/owner source и R03-001 handoff на этом cut не
изменились. Точный следующий seal — final-cut.json; это не атомарность дерева.

## 12. R03-003 — полезность измерения для решения и границы идентификации

2026-10-25 · **READY_FOR_REVIEW**. Постановка: DEPTH-CYCLE-002, направление R03.
Предыдущие пакеты сохранены; READY не означает ACCEPTED. Это научный contract
и ручной разбор, без изменения policy, выполнения моделей или продуктовых проверок.

### 12.1. Дополнительный пробел и актуальная цепочка

THEORY17 уже разделяет hard admissibility/optional gain, описывает correlated
измерители, missing outcomes и цену перехода. R03-001/002 уточнили порядок ranking,
учёт, bounds пропусков и время. Оставался вопрос **выбора самого измерения**:
как связать его возможные ответы с конкретным действием и ограничить весь
последовательный план, когда нет проверенных prior/likelihood/cost models.

| Текущий source seam → consumer | Что действительно есть | Граница предложения |
|---|---|---|
| `windows/bgprobe.go`: `bgProbeLoop → runBgRound → bgDueTargets → bgProbeChannel` | Последовательный проход due targets; QUIC пропускается; TCP dial после `AdmitDiagnostic(Background)`; admission refusal не negative observation; повторный fail → `chanHealth.observeBg`, success не protected proof | Отсюда можно обсуждать optional background состав. TCP dial не подтверждает HTTP/DNS/egress или российский blocking mechanism |
| `windows/adaptive.go`: UI-call `RunAdaptiveCheck → AdmitDiagnostic(Endpoint) → classifySite → wg.Wait → current-session check → adaptiveLast/persistAdaptive` | Ручная site-диагностика с отказом при занятом budget; late/stale результат не публикуется | Не подавлять все Endpoint/manual операции правилом для automatic background |
| `windows/probe_observation.go`: `appProtectedProbe.Run → ProtectedProbe.Run`; `windows/backend/core/probe.go → AdmitDiagnostic(Protected)` | Required protected measurements, bounded reservation до работы; unavailable при отказе admission | Малая optional VoI не позволяет убрать required proof, считать unavailable успехом или включить direct fallback |
| `windows/backend/core/diagnostic_budget.go`: `AdmitDiagnostic → processDiagnostics.{protected,optional} → caller defer release` | Rolling60s: protected120requests/1MiBbody/2ops, optional64/2MiB/2ops; no refund, no queue при rejection | Это caps declared request/body work и active operations, не wire bytes/energy/WCET. Новый planner не создаёт свой owner/limiter |
| `shared/adaptive/evidence.go`: `ComposeEvidence`, API1 ADP-004 | Exact identity/capability/window join, latest **decisive measurement/status** sequence, typed incomplete/unsafe; `RequiresPlatformGate=true`. Rejection/cancel/unknown сохранены в trace, не заменяют/не продлевают prior measurement. `DependencyRef` не создаёт independence | Не Bayesian inference, не authenticity/protected Ready. Scoped non-test `.go/.kt` search в windows/android/shared нашёл только definitions новых pure APIs, не platform consumers |
| `shared/adaptive/{failover_budget,retry_budget}.go`, ADP-003 | Pure plan/settlement arithmetic и typed outcome effects уже source implemented, unverified; Expected/live owner recheck остаётся platform obligation | Не объявлять этот код отсутствующим, проверенным, reservation или готовым portable JNI DTO |
| `android/.../OsProtection.kt`: `read → observedAt elapsed → isFresh`; `WorkerRpcLane.kt`: `submit/pending/takeCompleted` | Начальная elapsed дата OS observation; late call удерживается до actual return | Нельзя сделать повторный Binder call/освободить slot по истечению observation deadline; OS observation не egress measurement |
| `docs/adaptive/ADP-005-DESIGN.md`, late `shared/adaptive/observer.go` | На initial cut дизайн; к 19:08Z есть unconnected source prototype `PlanAutomaticBackground`, scoped count/clock/MaxAge advice. Producer/consumer отсутствуют, fairness/mark-done требуют owner решения | Не внедрённая VoI-policy и не повод расширять объём ADP-005 без owner review |

Зафиксированы 33 scoped local inputs, включая оба прежних handoff, и 10 primary/
author-code external captures. Exact UTC/SHA256/before-copy-after — manifest в
`.artifacts/research/theory-03/cycle-003/`. Это адресный capture, не atomic tree.
Неиспользованный UI/OS runtime, raw logs и secrets не читались. Отсутствие
проверенной likelihood matrix относится к изученным contracts, не ко всему миру.

На review cut19:02:21Z обнаружен drift7 из 33 inputs: ANDROID-PLAN, adaptive
API/EVIDENCE/README и evidence_types/validation/join. Relevant differences
прочитаны: latest-terminal заменён latest-decisive; administrative outcomes
не заменяют/не обновляют старое measurement, trace включает state/origin/censoring.
Новый code не представлен как наш результат или verified поведение. AND-11-S3
source admission/dispatch isolation прочитан по плану/ADR-039 и WorkerProcess:
`requestOsObservation → platformObserver.execute → observeOs → OsProtectionReader`
с local phase/original authority guards; uncertain dispatch удерживает admission.
`VpnSessionRepository` loop принимает `rpc/retirementRpc.takeCompleted → completed`,
не освобождает pending по одному observation deadline. Это source review,
не OS-cancellation/latency/device proof. Captured ещё 10 review inputs:
7 обновлённых и 3 новых paths, всего 36 уникальных. Прежний capture сохранён.

Late review19:07…19:08Z выявил ещё 4 doc changes: ADP-005 теперь **unconnected
source prototype**, не только design. Прочитаны actual `observer.go` DTO/branch:
known current protected count>0 → suppression suggestion; context/clock mismatch,
unknown/future/stale → no admission grant; MaxAge0…1min structural bound, не SLO.
`Expected` требует owner recheck; no resource mutations. Новых platform callers
в рассмотренной цепочке нет. Ещё 5 late captures (4 docs + observer.go), итого
37 unique local paths/48 captures; все ранние версии сохранены. Это source-only
изменение другого владельца, не наш выполненный эффект и не acceptance.

### 12.2. R03-06 — единица выбора: ограниченный план для объявленного решения

**Предложение:** оценивать optional измерение через то, что оно может изменить
в конечном действии. Разделить: (a) required proof перед platform action,
(b) optional diagnosis/optimization для текущего вопроса, (c) отдельно разрешённое
обучение/обслуживание для будущего горизонта. Одно измерение может иметь несколько
целей; отказ по одной цели не отменяет остальные обязанности.

Формальное различие при известной совместной вероятностной модели: пусть W —
скрытое состояние, A — конечные допустимые действия, L(a,W) — loss, p — prior.
Тогда Bayes risk `R(p)=min_a E_p[L(a,W)]`, а gross expected value of sample
information `EVSI(q)=R(p)−E_y[min_a E(L(a,W)|Yq=y)]`. Это определение,
согласованное с decision-theoretic постановкой [DIRECT, background](https://las.inf.ethz.ch/files/chen15submsrgtvoi-long.pdf).
Стоимость/ожидание/нарушение задачи ещё не вычтены. Неотрицательность gross
свободной информации требует возможности игнорировать ответ, неизменных действий
и состояния, отсутствия вредного вмешательства/задержки. Для реальной проверки
эти условия нужно обосновать. Неизвестный p или likelihood не заменять 0.5.

**Ручной пример D1 — entropy не равна пользе.** Только в иллюстративной модели:
W — fair bit, Z — четыре независимых nuisance bits, L ошибки действия 10 units.
qN раскрывает Z (4bits), qD раскрывает W (1bit). Без проверок risk5; после qN
он 5, после qD —0. При одинаковой условной цене 1 net gain соответственно−1/+4.
Числа не относятся к VPN и не задают preset. Обновление деталей fingerprint,
которые не меняют ни hard gate, ни выбор действия, может быть бесполезным для
текущего решения даже при большом information gain.

**Ручной пример D2 — zero one-step value не доказывает бесполезность.** W есть
X1 XOR X2; X1/X2 независимые fair bits; доступны отдельные измерения каждого.
Ошибка действия стоит 10, каждое измерение 1. Одно измерение оставляет risk5,
gross EVSI0. Два вместе дают risk0, total cost2, net gain3 относительно stop.
Условная ценность второго возрастает после первого: у этой raw VoI нет общего
adaptive diminishing-returns свойства. Правило «не брать ничего с EVSI≤0 на
первом шаге» ошибается. Нужен допустимый bounded continuation либо специально
обоснованный surrogate, а не обещание universal greedy optimum.

**Достаточное доминирование D3, собственный конечный вывод.** Если при одном
контексте и совместной модели `Y2=f(Y1)`, проверка q1 не дороже/позже q2 по
всем принятым constraints, не меняет workload/state/observer иначе и допустима,
то любое решение после q2 можно воспроизвести после q1 вычислением f. Поэтому
q2 dominated относительно q1 для этой задачи. Для замены внутри многошагового
плана нужны те же свойства **совместно со всеми дальнейшими наблюдениями**,
costs и доступностью: одной marginal связи недостаточно. Proof: подставить
f(Y1) вместо Y2 в том же contingent rule; loss одинаков, costs не выше.
Разные endpoints/URL/порты не устанавливают такого отношения. Даже настоящий
duplicate может быть полезен позже при изменившемся состоянии или новом шуме.

### 12.3. Предлагаемый bounded contract — не новый API implementation

Переиспользовать existing API1 scope/observation/coverage/custody и платформенные
owners. Следующая таблица задаёт смысл полей для будущего design review; имена
не объявлены уже существующими DTO и не требуют отдельной universal library.

| Вход / выход | Обязательное содержание | Неизвестное / запрет |
|---|---|---|
| Decision question | Purpose, workload/task, конечные actions, current safe baseline/closed state, горизонт и deadline | Без вопроса нельзя считать «информационную пользу» вообще; у диагностики и оптимизации могут быть разные actions |
| Hard platform envelope | Exact owner/intent/path/image/cutover/clock domain/application context; current Required capability keys и hard expiry | Никакой model/prior/argmin не выпускает permit, не стирает unsafe signal и не продлевает authority |
| Candidate measurements | Конечный список известных методов/targets с exact coverage, observer/dependency provenance, совместимостью с current platform permission | Unknown route/family/endpoint не выдаёт coverage; GET через SOCKS не становится native UDP |
| Outcome alphabet | Success с exact facts; local admission rejection, not attempted/supported, timeout/cancel/supersede, failed и origin/censoring/unknown; pending actual owner отдельно | Не бинаризовать всё в «канал плохой». Известен terminal observation outcome, но physical resource может ещё удерживаться |
| Joint model | Версия, finite scenarios либо известный joint prior/likelihood, allowed outcome support, noise/common causes, validity horizon и явно отсутствующие причины | DependencyRef — provenance, не correlation coefficient; unknown independence остаётся unknown |
| Costs | Ranges/known flags отдельно для requests/body/slot occupancy/energy/bytes/privacy/длительности/disruption; reference к producer R04/platform | Не суммировать ms, bytes и battery без принятой функции loss; no refund/counter reset из utility расчёта |
| Time feasibility | Generation interval, clock domain, deadline для получения и публикации, bounds исполнения/очереди/gate и actual pending occupancy | Consumer timeout — не WCET и не доказанное освобождение; получение свежего endpoint ответа не гарантируется |
| Search limits | Явные caps числа scenarios, tests, outcomes, глубины, ветвей, рассматриваемых plans, CPU/work и памяти самого выбора | MaxReceipts64/MaxCapabilities16 API1 ограничивают receipts, **не** число всех планов. Полное M^K дерево без cap запрещено |
| Proposed output | Stop / suggest first measurement of bounded plan / inconclusive / unsupported model; reason и model/input refs, remaining constraints, validity/latest-start assumptions | Только рекомендация `RequiresPlatformGate`; reservation/action/authority принимает existing owner после атомарного recheck |

Finite plan family Π включает stop и лишь заранее ограниченные contingent plans.
Один вызов возвращает первый шаг; после ответа owner заново проверяет scope/time,
required evidence, budget и остановку. Это не создание actor, таймера, глобального
clock или второго retry owner. Параллельные measurements можно рекомендовать
только если явно смоделированы их interference и общая reservation/occupancy;
сумма отдельных полезностей этого не доказывает. Search exhaustion возвращает
bounded best-known suggestion/inconclusive с указанной полнотой, не «оптимум».

Contract последовательности:

```
hard event/current platform gate
  → required proof missing? ordinary bounded platform measurement/recovery
  → при отказе admission/measurement сохранить ordinary closed/unknown policy
  → optional question + declared model/constraints
  → отбросить недопустимое; проверить время; доказанное dominance в scope
  → bounded contingent alternatives вместе со stop, без fake likelihood
  → advice + reasons/expiry; owner atomic admission и единственный execution
  → typed observation + retained custody + фактическая публикация/возврат
  → update only under valid model; hard gate и stop проверяются снова
```

**Обязательное измерение не обязательно выполнить любой ценой:** permission,
admission, unsafe current state могут запретить его. Тогда required capability
остаётся отсутствующей и действие недопустимым; её не удаляют ради complete.

### 12.4. Время: «не успеет» и «не гарантировано успеет» различаются

Для текущего решения пусть E — earliest exclusive hard deadline/window end,
t — время сейчас в одном принятом clock domain. Для q обозначим нижние bounds
execution/queue/publication как C−, J−, G−, верхние C+, J+, G+; margin M≥0.

- Если `t+C−+J−+G− ≥ E`, ответ **не может** участвовать в этом решении при
  заявленной модели. Это основание исключить optional q именно для данного
  горизонта. Требуется реальный lower bound, а не среднее/p99.
- Если известны честные upper bounds, `t + C_plus + J_plus + G_plus + M < E` — достаточная temporal
  feasibility для заданной последовательности. Это не гарантия success endpoint.
- Если условие upper bounds нарушено или сами bounds unknown, своевременность
  **не доказана**, но физическая невозможность из этого не следует. R03-002
  latest-start `E−C+−J+−G+−M` — граница гарантированного окна при этих условиях,
  не universal cutoff возможного результата. Platform policy решает, допустим
  ли bounded best-effort optional trial; authority expiry остаётся hard.

**D4:** E=10,t=8,C∈[1,4],J=G=M=0. Finish возможен в 9, worst case12.
Отказ гарантировать не означает «заведомо опоздает». При t=9 и C−=1 каждый
finish≥10 уже не входит в exclusive window. Это ручные условные units, не SLO.
Если остаётся занятый owner с неизвестным actual-return bound, свободный slot
и конечное J+ не изобретаются. Возврат позже E может иметь отдельно разрешённую
future-learning/maintenance ценность; для current permission он непригоден.

### 12.5. R03-07 — joint uncertainty и предел различающих измерений

**Предложение:** для непроверенных probabilities сначала рассматривать finite
support/сценарии и показать, существует ли различающее наблюдение. Число
свидетелей само по себе не число независимых failure domains. Это не новый
блокировочный classifier: одинаковый fail может оставлять несколько причин.

**D5 — common cause.** q1/q2 обращаются к разным URL одного measurement origin E.
В мире wG не работает нужный путь G, E исправен; в wE путь G исправен, отказал E.
Все разрешённые q1/q2 в обоих мирах дают fail. Если смена пути нужна в wG,
а сохранение текущего — в wE, измерения не устанавливают причину/выбор.
Формально, когда для двух статических миров любая доступная проверка имеет
одинаковый ответ, по индукции любой конечный causal contingent plan получает
одинаковую историю и не может гарантировать разные правильные действия.
Рандомизация даёт одинаковое распределение действий, также без гарантии в обоих.
Это **текущая экспериментальная** неидентифицируемость, не предыдущий аргумент
THEORY17 о произвольном неизвестном будущем. Отличающий разрешённый контроль
может разрушить эквивалентность; score/ML не создаёт отсутствующий контроль.

**D6 — арифметика зависимости.** В условной модели только общий outage с p=0.1,
оба endpoint-fails совпадают: P(F1∩F2)=0.1, не 0.01. Два разных ref/host/порт
не дают произведение вероятностей. Но correlation не запрещает все готовые
active methods: при известной совместной latent модели зависимость учитывается.
EC² допускает correlated noise в расширенном joint space; ECED предполагает
conditional independence при полной Θ, хотя при target Y тесты могут быть
зависимы. Главный пробел проекта — не название «корреляция», а неустановленные
joint causes/likelihood/stability и сопоставимость observations.

**Missing support:** rejection/not-attempted не наблюдает сеть; timeout может
иметь положительную вероятность во многих мирах. Удалять мир допустимо только
если принятая модель действительно исключает этот ответ в нём. При model mismatch
или пустом support → inconclusive/model-invalid, не автоматический «истинный»
оставшийся класс. Исторические bounds `[s/N,(s+u)/N]` R03-03 ограничивают конечный
набор исходов; они не поставляют point prior, iid, likelihood или causal effect.
Смена scope/сети/нагрузки обрывает применимость модели, не сбрасывает resource
или statistical error budget. Coverage join не доказывает причинную модель.

### 12.6. Ограниченное сравнение без вероятностной модели: что оно доказывает

Один пригодный **design/reference**, а не production recommendation: конечные
joint scenarios Ω, конечные допустимые contingent plans Π, явный static loss
L(a,w) и сопоставимая total cost Cπ(w)≥0. Каждый w задаёт совместно все ответы,
common causes и costs; допускается расширить w noise realizations. Всем branches
нужны ordinary platform gates, одинаковый safety envelope и feasibility.

`R0 = min_a max_w L(a,w)` — worst-case baseline в объявленном action set;
`Rπ = max_w [Cπ(w)+L(aπ(history(w)),w)]` — worst-case total loss плана;
`π* ∈ argmin_{π∈Π∪{stop}} Rπ` — только минимум по **объявленному** bounded набору.

Это minimax criterion, не expected probability/confidence. Включение stop
гарантирует `min Rπ≤R0` **в этой точной модели**, если stop реализует minimax
baseline; текущий безопасный пользовательский baseline может быть другим и
должен сравниваться отдельно. При многокритериальных costs без функции обмена
остаётся feasibility/Pareto/non-dominance, а не фиктивное scalar значение VoI.

**D7 — верхняя граница возможной пользы, собственный вывод.** При static W,
неизменном action set и чисто информационном тесте:
`RPI=max_w min_a L(a,w)`, `VPI=R0−RPI≥0`.
Для любого плана с **равномерной по всем w** нижней ценой Cπ(w)≥c− имеем
`Rπ≥c−+RPI`. Если c−>VPI, такой план не улучшает R0. Доказательство: выбранный
loss в каждом w не меньше min_a L, затем взять max. Если в некоторых branches
есть бесплатный stop, нельзя брать положительный c− от только дорогих branches.
Неравенство неприменимо к тесту, который чинит путь/даёт новое полномочие,
меняет workload или нужен hard gate; не распространяется на future horizon.

**Сильнейший контраргумент D8 — minimax может скрыть полезный средний выигрыш.**
Два мира, loss A=[0,10], B=[6,6]. R0=6 (B), RPI=6, VPI=0: улучшения worst case
нет даже при free perfect information. При **иллюстративном** prior1/2 baseline
Bayes A имеет expected loss5, perfect-information action —3, gain2. Поэтому
консервативный критерий не «теоретически лучший» для всех целей. Выбор риска,
ожидаемой пользы, regret или constraints — явное продуктовое/научное решение.

Сравнивать планы надо на одном **joint Ω в корне**, включая наблюдаемые costs и
поддержку историй. Независимо брать worst case каждой проверки или строить
несовместимые posterior sets создаёт несуществующие миры. Представленные формулы
не доказывают Bellman/time-consistency для произвольных ambiguity sets. Без
такого доказательства допустима оценка полных bounded contingent plans на общем
Ω; myopic reoptimization обозначается эвристикой и может изменить цель.

### 12.7. Готовые методы: граница переноса и лицензия

Точные первичные версии и code pin — SOURCES§5. Короткие выводы по papers ниже
не являются измерением VPN или переносом их experimental results.

| Метод / механизм | Необходимая предпосылка и действительная цель | Решение для проекта |
|---|---|---|
| [EC², Golovin–Krause–Ray, v2 2013-12-16, §3–4](https://arxiv.org/html/1010.3091v2): weighted cross-class edge cutting / cost | Known finite joint prior; expanded deterministic realizations; disjoint decision classes/tie rule; задача минимизировать цену определения класса. Correlated noise допустим; это не свободная independence assumption | **Adapt/reference** различать пары, требующие разных действий. Не объявлять direct EVSI greedily submodular; не переносить class guarantee на неизвестную РФ-сеть. Expanded model может быть слишком большим |
| [DIRECT, Chen et al., AAAI2015, extended pp2–5/Theorem2](https://las.inf.ethz.ch/files/chen15submsrgtvoi-long.pdf): несколько ECD graphs + специальный Noisy-OR surrogate | Known joint distribution/efficient inference; overlapping decision regions; near-minimal expected testing cost для заданного regret относительно all-tests decision. Не generic optimization всех deadline/cost/intervention objectives | **Adapt/reference** ε-регионы и stop-критерий. Не всякое объединение submodular функций наследует adaptive submodularity. Нет blanket near-optimality нашего planner |
| [ECED, Chen–Hassani–Krause, arXiv v2 2016-07-11, §2–4](https://arxiv.org/html/1605.07334v2): edge discounting/offset при шуме | Known Θ prior/conditional likelihood; независимые тесты при Θ; unit costs. Main bound дополнительно binary outcomes/root-cause-independent test noise; target error и худшее число тестов | **Reference**, prospective model review. Не подставлять произвольные dependent/censored VPN samples. Не ссылаться на AISTATS2017 как дату прочитанной v2 |
| [Track-and-Stop, Garivier–Kaufmann, COLT2016 §2/4/5](https://proceedings.mlr.press/v49/garivier16a.pdf): empirical allocation tracking + Chernoff stopping | Fresh independent draws fixed arm distributions, one-parameter families/unique best; fixed-confidence correctness и asymptotic sample efficiency. Это не finite wall-clock availability bound | **Reference/adapt only after model validation** для ограниченного future comparison. Нестационарность, зависимость, неслучайные пропуски, повторные resets ломают перенос гарантий |
| Finite joint-scenario/contingent-plan comparison §12.6 | Явное полное finite support, static comparable losses/costs, bounded search и hard gates. Ни prior, ни guarantees вне Ω | **Design candidate**, прозрачная граница unknown; выбирать objective отдельно. Не новая готовая библиотека/новая оптимальность |
| Existing API1 ComposeEvidence / ADP-003 + platform limiter/owners | Exact scope/coverage/windows/custody и source-only состояние | **Reuse first**. Нельзя заменить эту механику Bayesian posterior и нельзя ввести второй budget owner |

**D9 — overlapping decisions не equivalence relation.** Opt(w1)={A},
Opt(w2)={A,B}, Opt(w3)={B}. w1/w2 имеют общий оптимум, w2/w3 тоже, w1/w3 нет.
Нельзя склеить все три в один класс транзитивным объединением. Это причина
рассматривать decision-region постановку DIRECT либо явный tie/decision rule,
а не механически переносить disjoint EC² classes.

**Реальный готовый code:** авторская ссылка ведёт в
[`jsfunc/best-arm-identification`, commit9161abd16be74ae82b47b4653ae5b1a6e6a037df](https://github.com/jsfunc/best-arm-identification/tree/9161abd16be74ae82b47b4653ae5b1a6e6a037df),
committer date2016-07-31. `LICENSE.md` — MIT, copyright Garivier/Kaufmann2016;
сохранение notices обязательно для копируемого существенного code. Scope: этот
repository, не автоматически его dependencies. README фиксирует Julia0.4.0,
Distributions0.8.7/HDF50.5.6/PyPlot2.1.1, не утверждает поддержку нынешней Julia.

**Actual caller:** `mainBAI.jl: MCexp → policy(mu,delta,beta)`, policies содержит
`TrackAndStop`; `BAIalgos.jl: TrackAndStop → OptimalWeights/d/sample`, импортирует
KLfunctions/Distributions/PyPlot. `mu` поступает от simulator, не сетевого
measurement adapter. Consumer — recommendation/N и simulation aggregation;
guard10,000,000 draws — не network deadline/resource contract. **Reference сейчас**;
при будущем доказанном stationary эксперименте возможно адаптировать allocation/
stopping math с MIT attribution, отдельно портировав dependencies и unknown/
censoring/stop semantics. Прямой import Julia в Go1.26 policy/Android JNI
не обоснован. Ничего не запускалось/не установлено/не перенесено в product.

Для EC²/DIRECT/ECED в прочитанных primary paper/author links не установлен
точный готовый code/license/API. Это ограниченный результат поиска, не «кода
не существует». ArXiv publication grant и AAAI copyright не software license;
paper pseudocode не скопирован в продукт. Перенос концепции требует атрибуции,
а копирование чужой реализации — отдельной лицензии и version review.

### 12.8. Когда остановиться, отказаться или оставить unknown

| Условие | Допустимый результат | Нельзя выводить |
|---|---|---|
| Required capability не доказана / current unsafe | Ordinary platform blocked/unknown/recovery; искать разрешённый mandatory measurement | «Проверка невыгодна, значит required можно убрать» |
| Для optional вопроса один action оптимален во всех принятых мирах; либо все допустимые планы доказанно не лучше stop | Stop в этом model/horizon; можно указать отдельную diagnostic/future цель | Универсальная бесполезность всех будущих проверок |
| Distinguishing experiment отсутствует; identical-history worlds требуют разных действий | Inconclusive, сохранить baseline/hard policy; запросить **какой** независимый control нужен | Уверенный causal label из количества повторов |
| Upper-time bound unknown/не помещается, lower-time bound ещё допускает ответ | Feasibility unproved; optional best-effort только по explicit platform policy | Заведомая физическая невозможность |
| Lower-time bound уже за E | Exclude для текущего deadline; retained owner завершается обычным порядком | Release/refund/error-budget reset по одному clock expiry |
| Нет calibrated p/likelihood или comparable loss | Support/Pareto/finite model proposal, unsupported numeric EVSI | Удобные вероятности/числа costs без producer |
| Нет свободной reservation / actual return pending / state superseded | Admission reject/stop, typed local outcome | Network failure или новый work сверх cap |
| Search cap достигнут / model mismatch | Bounded incomplete recommendation или inconclusive с reason | Global optimum / доказанная неинформативность нерассмотренных связок |

Общее отсутствие достаточного основания для trial не требует менять работающий
protected путь. При реальном hard failure baseline может быть closed barrier;
«keep current» не превращается в разрешение продолжать опасную сессию.

### 12.9. Самокритика, опровержимость и недостающие данные

Это opposing self-review автора, **не независимое внешнее принятие**.

1. **Модель Ω неполна.** Неучтённый общий DNS/CDN/ISP/control cause делает
   «различение» ложным; finite proof сохраняет только условную корректность.
   Falsifier: разрешённый outcome вне support или изменение post-measure action
   risk вопреки модели. Нужен explicit model-invalid, не выкидывание неудобных данных.
2. **Robust choice слишком осторожен.** D8 показывает потерю expected gain.
   Нет основания навязывать minimax всем workloads; objective обсуждается с R06.
3. **Слишком агрессивное pruning.** D2 опровергает zero-one-step cutoff. D3
   требует joint продолжения; невозможна замена по одной marginal «точности».
4. **Критика готовых методов может быть чрезмерной.** EC² реально допускает
   joint correlated noise, DIRECT решает overlapping regions. Их не отвергать
   словом «корреляция»; показать, какие model assumptions ещё нельзя подтвердить.
5. **Измерение само вмешивается.** Одновременный throughput test меняет очереди,
   battery/radio state и дальнейшие outcomes. Pure-information proof тогда не
   действует; нужен R04 producer/interference model, а не простая цена bytes.
6. **Текущий decision не вся польза.** Background maintenance, rare drift alarm,
   будущая calibration и UX explanation могут оправдать проверку отдельно.
   Нужны purpose/budget/horizon; запрет на срез не становится запретом навсегда.
7. **Высокая цена planner.** Полный Ω/plan tree может стоить дороже экономии
   probes; declared caps/search accounting обязательны, эффект пока неизвестен.
8. **Проверки пути не все safety.** Удаление «лишнего» observer может потерять
   quiet+revocation detection. Hard surveillance obligation задаёт R05/platform,
   не optional optimizer; это главный review вопрос к минимальному составу.

Недостающие producer inputs: от R02 — joint observer/censorship mechanisms и
границы причин; R04 — bounds/unknown на execution, queue/publication/actual return,
interference и energy; R05 — минимально обязательные current proof/authority gates;
R06 — конкретное действие пользователя, горизонт, loss/disruption/stop semantics.
Ни одно поле не заполнено выдуманным product числом. Числа D1…D9 — ручные модели.

### 12.10. Конечная будущая приёмка — не запуск и не разрешение

| Case | Будущая постановка | Наблюдаемый критерий |
|---|---|---|
| V01 | Required proof имеет EVSI0 | Не удалён Required; action остаётся gated; отказ измерять не превращён в success |
| V02 | D1 nuisance entropy против decision bit | Не выбирать nuisance только по entropy; purpose/model/units отражены |
| V03 | D2 XOR, depth2 допустим | Zero one-step не объявлен proof useless; bundle сравнивается либо depth limit честно указан |
| V04 | D3 deterministic duplicate с теми же joint continuation/costs | Dominance reason воспроизводим; нет лишнего branch/inference |
| V05 | «Duplicate» после смены time/scope/observer/cost | Старое dominance не переносится; model revalidated/inconclusive |
| V06 | D5 одинаковые measurements при разных best actions | Inconclusive, no false cause; добавление отличающего control меняет идентифицируемость |
| V07 | D6 common cause против независимой модели | Не перемножать failures без модели; unknown зависимости не confidence gain |
| V08 | ECED likelihood condition нарушена / out-of-support result | Model-invalid/inconclusive, без прежнего theorem claim |
| V09 | Timeout/reject/not selected + pending actual call | Типы не смешаны; no negative network fact, no fake completion/refund/new slot |
| V10 | D4 possible-but-not-guaranteed и impossible deadline | Разные reasons; exclusive expiry соблюдён; no hard impossibility из upper bound |
| V11 | Deadline сменился между advice/admission/publication | Recheck current scope/expiry; late evidence не разрешает действие |
| V12 | D7 cost bound держится лишь в части branches | Не применять uniform c− theorem к условному branch cost |
| V13 | D8 minimax/Bayes разные предпочтения | Objective/model явно указаны; нет подмены expected/worst-case benefits |
| V14 | D9 пересекающиеся ε-action regions | Не использовать транзитивную equivalence склейку без корректного mapping |
| V15 | Unknown energy/duration, нет exchange function | Vector/unknown; без фальшивых scalar scores/нулей |
| V16 | Перебор scenarios/plans превосходит cap | Bounded stop/incomplete reason; отсутствие global-optimal claim; resource owner не обходится |
| V17 | Protected work active, automatic background и manual site check | ADP-005 suppression если отдельно реализована касается exact automatic purpose; required/manual действия не потеряны |
| V18 | Current task против отдельно разрешённого future learning | Раздельные цели/учёт; past cutoff не глобальная бесполезность |
| V19 | Ω mismatch, network epoch/cutover/clock discontinuity | Старый model invalidated; counters/actual custody/error budget не reset ради результата |
| V20 | На одинаковом corpus baseline vs bounded planner | Useful completion≤D, cost vectors, false switches/missed hard events, missing fraction и planner work; no improvement claim без разрешённого paired результата |

Первый разрешённый будущий этап — static contract/adapter cases, затем bounded
local model evaluation, отдельно device и field comparison. Сейчас все execution
criteria **NOT RUN**. Нет simulation-derived optimum, throughput или energy цифр.

<a id="1211-адресная-передача-и-предлагаемая-вставка"></a>

### 12.11. Следствия для наблюдений и решений

- **ADP-004/006:** reuse exact clock/identity/capability/outcome/custody; model/ref
  может быть научной надстройкой, не новый Required bypass. Existing source APIs
  остаются unverified до platform adapter/parity приёмки.
- **ADP-005:** текущий suppression design не расширять до универсального VoI
  planner. Сначала review purpose/manual distinction и fairness; новое bounded
  measurement planning — отдельный согласуемый design item после baseline.
- **TH-02:** измерения/controls нужные для различения D5 и correct missing/origin
  publication; TCP background/site/native/protected не взаимозаменяемы.
- **TH-04:** существующий owner/budget/admission/atomic gates; stop/expiry не
  скрытый reset/refund/retry. Advice-to-execution scope/time recheck обязателен.
- **TH-06:** producer реальных cost/interference/time ranges и cost самого
  planner, затем paired useful-task comparison; не выбирать веса вслепую.
- **AND-07:** applicability gate до заимствования best-arm/active-learning;
  текущие OS/Binder факты и future protected observations имеют разный coverage.
- **AND-11:** radio/battery/JNI/actual pending return должны входить в constraints;
  partial results не разрешают второй concurrent inspection сверх owner bound.
- **AND-16:** российский field/model/correlation/held-out calibration и сравнение
  только после оплаченной infra/отдельного live-разрешения; paper не заменяет его.

**Предлагаемое уточнение THEORY17:**

> Выбор измерения привязывается к конкретному безопасному действию и горизонту.
> Required proof нельзя убрать из-за малой expected utility. Для optional работы
> сравнивают ограниченные последовательные планы вместе со stop, с явными joint
> assumptions/cost ranges/временем и ресурсами самого выбора. Большая entropy не
> означает полезность; zero one-step gain не исключает полезную связку проверок.
> Correlated endpoint failures не дают независимый quorum. Если доступные
> measurements не различают миры с разными нужными действиями, causal diagnosis
> остаётся unknown; нужен различающий разрешённый control, а не новый score.
>
> Нарушение upper-time bound означает отсутствие гарантии успеть; доказанная
> невозможность требует lower bound за deadline. Ни поздний ответ, ни posterior
> confidence не продлевают authority/coverage и не освобождают actual owner.
> EC²/DIRECT/ECED/Track-and-Stop полезны как готовые научные механизмы при точных
> model assumptions; без проверенных priors/likelihoods/stability/costs проект
> не наследует их optimality/error guarantees. Minimax и expected utility — разные
> objectives; выбор и conservative unknown должны быть явными.

R03-06/07 требуют отдельного scoped решения; авторский статус не является ACCEPTED.

## 13. R03-004 — как идентифицировать эффект новой policy

2026-10-25/26 · **READY_FOR_REVIEW**. Это отдельный срез после 003,
не принятие прежних результатов.
Реальные пользовательские логи/трафик не читались, эксперименты не выполнялись.

### 13.1. Новый вопрос и source-to-outcome

THEORY17.4 уже предупреждает о winner-only samples и отсутствии propensities.
R03-03 дал finite missing bounds, R03-06/07 — выбор измерений при joint uncertainty.
Дополнение здесь: **какой именно эффект можно оценить** при последовательных
действиях, admission отказах, неполном исполнении и переносе состояния между ними.
Correct estimator не исправляет неправильную единицу сравнения/цель.

Текущая Windows цепочка в `failover.go:tryFailoverWithObservation`:

```
applied cc → same descriptor/selector provenance → failoverCandidates
  → reachable IDs → scoredCandidates → orderCandidatesForFailure
  → orderByRecentAttempts → target=scored[0]
  → reserveObservedSwitch / beginAttemptIfDue
  → switchChannelCoreFor → finishObservedSwitch / finishFailoverAttempt
```

`observer_failover.go` receipt содержит intent/observation/session/attempt/epoch.
Завершение отдельно проверяет token/current scope, cancellation, accepted
publication; verified rollback и discard тоже могут вернуть `(false,false)`.
Это корректно не равно единому «network failure». `appendLog` пишет сообщения,
а scoped source не предоставляет полный randomized assignment/outcome dataset.
Сам raw журнал не читался. No-propensity finding относится к captured selection/
receipt DTO и SOURCE-MAP, не объявляет отсутствие любых данных во всём продукте.

В `shared/adaptive/budget_types.go` уже раздельные AttemptOutcome/OwnerReturned/
ResourcesRetired/DecisionCurrent/PublishedCurrent; `evidence_types.go` имеет
Origin/Censoring/Sequence и opaque load/warmth. Это полезные reused identities,
но не propensity, user task completion или случайный experimental assignment.
В Android `VpnSessionRepository` RPC completion/retained pending и recovery budgets
также не создают такие данные. AND-07 measurement producer остаётся отдельным
незавершённым platform contract; этот срез не добавляет его и не меняет lifecycle.

21 initial scoped source/docs с hashes/before-copy-after сохранены в cycle-004.
Ни recommendation trace, ни deterministic replay не названы causal comparison.

Review cut2026-10-26T06:03:30Z:18/21 совпали, изменились только Android
VpnSessionRepository/WorkerRpcLane/STATE. Relevant current source и STATE
прочитаны: AND-03-S9 sticky failure/retained delivery до exact Call acknowledge,
no replacement owner/authority renewal. Добавлены 3 review captures; это source
не runtime proof и не assignment/measurement producer. Основная Windows/shared
selection-to-outcome цепочка и прежние handoffs на cut не изменились.

### 13.2. R03-08 — one-step value и эффект полной адаптивной системы различны

Для одного решения обозначим pre-action history/context как H, поведение как b,
целевую policy как π, action как A, наблюдаемый bounded reward Y∈[0,1]. При
нужных assumptions one-step OPE оценивает:

`V_one(π;d_b)=E_{H~d_b}[Σ_a π(a|H) μ(H,a)]`.

Полная новая система обычно интересует по другому estimand:
`V_system(π)=E_{trajectory~P_π}[useful-task reward/cost]`.
Новая policy меняет будущие history, warmed streams/cache, cooldown, budget,
queue и stop/recovery transitions. Поэтому `d_b` не обязано совпадать с `d_π`.
Выше — определения, не доказательство stationarity/MDP для VPN.

**E1, собственный контрпример:** history имеет состояние ready/depleted.
В готовом состоянии A даёт 1 и оставляет ready; B даёт 1 и делает depleted.
В depleted никакое действие не даёт reward и состояние сохраняется.
Поведение всегда A, поэтому на каждом наблюдаемом history ready. Даже если
one-step reward B в ready **известен oracle**, оценка B на d_b равна 1, тогда как
два последовательных B дают total1/2=0.5 против A=1. Это не только отсутствие
support: знание one-step responses само по себе не восстанавливает occupancy
целевой policy. В VPN это допустимая аналогия истощения ресурса, не найденный
реальный отказ или предсказание масштаба эффекта.

Для полного horizon нужен либо экспериментальный unit, которому назначают целую
policy на объявленный горизонт, либо корректный sequential model/trajectory
evaluation. [Jiang–Li2016, §3–4](https://proceedings.mlr.press/v48/jiang16.pdf)
отделяет contextual bandit horizon1 от sequential DR. Ратио trajectory содержит
произведение соответствующих action propensities; недостаточно одного текущего
score. Product state не признан Markov только из наличия epoch.

**E2:** если на каждом из L шагов target выбирает действие, имеющее у behavior
вероятность 1/2, совпадающая полная sequence имеет вероятность 2^(−L), вес 2^L.
Это ручная conditional model, не measured VPN distribution. Более сложный
sequential estimator может уменьшить variance, но не создаёт support для
невозможной sequence. Need per-decision full history/state/transition assumptions;
нет разрешения на небезопасные exploratory transitions ради overlap.

### 13.3. Минимальные условия one-step IPS/DR и чему они не равны

Полезно сохранить простой conditional derivation вместо обещания «библиотека
исправит bias». Требуются:

1. **Consistency/action definition:** наблюдаемый Y есть outcome определённого
   A. Если A значит assignment policy, не переименовывать его в фактически
   применённый transport. Версия policy/config/measurement/горизонта фиксирована.
2. **Assignment exchangeability:** при объявленном pre-action H назначение имеет
   известный механизм `e(a|H)` и не зависит от ещё скрытого potential outcome.
   Рандомизация — один способ, не автоматическое свойство deterministic logs.
3. **Positivity/support:** для каждой target mass π(a|H)>0 нужно e(a|H)>0.
   Tiny e не zero, но может делать оценку практически бесполезной по variance.
4. **Outcome observation:** Y валиден/наблюдаем в объявленном horizon; missing/
   censoring/admission не выбрасываются неявно. При correction observation
   probability нужны дополнительные MAR/exchangeability/positivity assumptions.
5. **History/model discipline:** π и регрессия m до текущего исхода фиксированы,
   либо fit/evaluate split действительно соответствует зависимости данных.
6. **Target population/interference:** one-step distribution и treatment версии
   определены. Shared VPS/queues, carryover и endogenous future eligibility не
   устранены случайным перемешиванием строк или opaque LoadRef.

При exact e, predictable m и этих one-step условиях:

`Z_IPS = π(A|H)Y/e(A|H)`;
`Z_DR = Σ_a π(a|H)m(H,a) + π(A|H)[Y−m(H,A)]/e(A|H)`.

Ручная проверка: условное expectation correction равно
`Σ_a π(a|H)[μ(H,a)−m(H,a)]`; оно сокращает model term. Отсюда unbiased
conditional score для declared one-step value, а **не** confidence interval,
независимость последовательных scores или protected-path authority.
[Dudík et al.2011 §2–3](https://icml.cc/2011/papers/554_icmlpaper.pdf) — primary
основа IPS/DR; точность либо модели rewards, либо propensities не снимает
support/consistency/observation assumptions. При estimated e обещание exact
finite unbiasedness нельзя переносить без анализа ошибок/fit procedure.

**E3, same-row leakage:** два actions всегда дают reward1, e=π=(1/2,1/2).
После наблюдения выбрать m(выбранный)=1,m(невыбранный)=0. Residual равен 0,
DR score равен 1/2 вместо true1, хотя e известен точно. Ошибка — m зависит от
current assignment/outcome; previous conditional cancellation недействительна.
Это собственный отрицательный пример, не обвинение конкретной library.
Стратифицированный temporal/block fit plan нужен до переноса DR в проект.

**E4, lack of support:** behavior всегда A, outcomes A=1. Два мира B=0 и B=1
имеют одинаковые logs; value target always B различно. Подстановка propensity1
для B неверна, а learned m(B) — model assumption/extrapolation, не наблюдение.
При известных population means поддержанных actions можно дать identification
bound. Пусть веса контекстов v_x заданы, Σv=1, `m0=Σ_x v_x Σ_{a:e=0}π(a|x)`;
`Vs=Σ_x v_x Σ_{a:e>0}π(a|x)μ(x,a)`. Тогда bounded reward даёт
`V(π)∈[Vs,Vs+m0]`, tight без дополнительных ограничений на unseen rewards.
Proof: неподдержанная contribution находится между 0 и m0, extremes достижимы.
μ/v в практике ещё оцениваются; interval — **identification**, не confidence.
Например supported mass0.7 с μ=0.6 даёт [0.42,0.72], не point0.6.

**E5, clipping меняет bias:** при μ=1,target mass1,behavior e=.01,
unclipped weight100. После cap10 expected clipped estimate=.1, true value1.
В общем для nonnegative rewards bias capped IPS равен
`−Σ_a [π(a)−λe(a)]_+ μ(a)` при fixed H. Это собственный finite вывод из
`e·min(π/e,λ)=min(π,λe)`. DR clipping имеет model-dependent residual bias, знак
не универсален. Self-normalization тоже не exact finite unbiasedness. Уменьшение
variance не позволяет скрыть смену estimator/assumptions и показать неверный CI.

### 13.4. R03-09 — assignment, admission и observed outcome: три разные воронки

Предлагаемый future comparison сначала фиксирует **unit**: например eligible
incident/session/task на ограниченном горизонте, не только успешный probe.
Возможные назначения — **целые версии policy внутри одного hard envelope**.
После assignment ordinary owner может отклонить work, отменить, rollback или
оставить closed state. Это часть end-to-end эффекта assignment, а не повод
удалить unit. Эксперимент не даёт исключения из platform safety gates.

**E6, admission denominator:** из 50 assigned A допустила 40, завершила 32;
из 50 assigned B допустила 20, завершила 18. Среди admitted B=90% против A=80%;
среди assigned useful completion B=36% против A=64%. Это конечный пример
разворота ranking, не causal significance или ожидаемые проценты продукта.
Нельзя считать observed/accepted success rate целью общей безотказности.

Рандомизация **после общего, независимого от назначения admission** может быть
корректна для более узкой population already-admitted units. Она не оценивает
разницу policies в admission при offered load. Когда admission зависит от
назначенной policy/cost, условие admission — post-treatment selection; новая
propensity исполненного action не обязана равняться assignment probability.

| Слой | Что записать в будущем bounded research trace | Что запрещено подменять |
|---|---|---|
| Population / precheck | Unit ID, policy/contract version, заранее заданный task/горизонт, pre-assignment eligibility и причины exclusion | Начинать denominator с successful receipts; выбирать «удобные» incidents после outcomes |
| Assignment | Policy arm/version Z, actual e(Z|prehistory) и allowed assignment set до draw; frozen target/model/split reference | Считать deterministic score вероятностью; восстанавливать e из доли observed winners; писать seed/секреты |
| Advice / admission / execution | Existing decision/attempt refs; timestamps, refusal stage/origin, applied vs rollback/current/superseded, actual resource custody | Приравнять Z, proposed candidate и реально применённый transport; drop failed admission |
| Task outcome | Y definition, D, observation status/follow-up, competing user cancel/epoch change и typed reason | Protected health success автоматически Y=1; missing автоматически 0 или completed RTT |
| Resources / measurement | Declared reservation и observed physical quantities раздельно; observer workload, retained pending и exposure/load/warmth strata | Expiry=resource return; неизвестные bytes/energy=0; concurrency interference отсутствует по умолчанию |
| Data lineage / privacy | Bounded opaque refs, model/estimator/fit/split IDs, counts lost records, retention/export decision | URLs/секреты/traffic/app history ради богатого H без отдельной необходимости/согласования |

Это **proposed data contract**, не новый implemented DTO/logger. RNG implementation
не проектируется здесь; exact assignment distribution review — отдельная будущая
работа, не разрешение randomized VPN behavior. Полный RNG seed не нужен для
оценки assigned effect; per-unit Z/e и проверенный assignment mechanism нужны.

Known non-completion by D и observation missing различаются. При валидном task
observer «не завершилось к D» может определить Y=0, даже если native owner ещё
pending; **его resource cost неизвестен/продолжается**. При потере самого observer
или лога Y неизвестен, а пользовательская отмена может быть competing event по
заранее принятому contract. Не превращать эти случаи в удобный общий failure.

### 13.5. Carryover, interference и допустимый дизайн сравнения

Обычный randomized order A/B на одном устройстве не стирает предыдущие state:
stream lifetime, DNS/cache, cooldown, pending owner, radio, offered load, shared VPS.
Epoch — provenance token, не experimental washout и не IID certificate.

В [Bojinov–Simchi-Levi–Zhao, switchback v4 2025-09-17 §2](https://arxiv.org/html/2009.00148v4)
потенциальные outcomes зависят от assignment path; formal design опирается на
non-anticipation и заданный finite carryover order m. Их optimal schedule не
становится нашим расписанием без обоснованного m и нужного estimand. Arbitrary
«подождать 30s» не доказывает исчезновение carryover у pending native call.

Практическая последовательность future design, **не выполнение**:

1. Зафиксировать outcome/единицу/горизонт/loss и common hard envelope вместе
   с R05/R06; честно выбрать effect whole policy или one-step action на old d_b.
2. На известных traces/replay проверить только semantics/parity/order; не causal
   gain. Их outcomes не контрфактическая истина нового исполнения.
3. Сначала разрешённая controlled setting с одинаковым offered workload,
   объявленной dependency/carryover scope и actual-return barriers. Незавершённый
   owner не reset между группами ради экспериментальной независимости.
4. Если будущая randomization целых независимых sessions/clusters возможна,
   определить unit, precheck, assignment probabilities, ITT denominator и lost
   data bounds заранее. All-policy effect внутри этой experimental population.
5. Когда весь device/VPS меняется во времени, рассмотреть block/switchback
   лишь с явной carryover моделью; нельзя одновременно именовать shared server
   независимыми individual units. При неизвестном m оставить ограничение.
6. Отделить nuisance fitting/policy selection от evaluation; заранее family
   hypotheses/looks и error budget. Прогрессивное обучение не оправдывает reuse
   одного outcome для выбора победителя и naive fixed CI без correction.
7. Затем отдельно external validity: ISP/day/device/family и distribution drift.
   Даже правильное значение на d_b не доказательство будущей РФ-сети.

Это не покупка новых серверов/не сообщение владельцам. Необходимую нагрузку/
time/cost/interference bounds передать R04, scope и контроль безопасности — R05,
workload/recovery goal — R06. Privacy и стоимость дополнительного observer входят
в сам сравниваемый system effect; их нельзя вычесть без смены вопроса.

### 13.6. Готовое и exact reuse

| Источник/механизм | Что пригодно | Предпосылки и решение |
|---|---|---|
| Dudík–Langford–Li, ICML2011 §2–3 | IPS/DR conditional correction; честная model-vs-propensity decomposition | **Reference/adapt** evaluator после logging contract; не production ML и не лечение unsupported actions |
| Hadad et al., arXiv1911.02768v4,2021-02-12 §2/Theorem2 | Adaptive AIPW weighting и условия inference при изменяющихся probabilities | **Reference**: IID potential-outcome vectors, positive propensities, infinite sampling/variance/moment условия; asymptotic normality не anytime validity, arbitrary drift или tiny finite-n guarantee |
| Jiang–Li, ICML2016 §3–4 | Sequential DR/trajectory structure и split policy/model/evaluation | **Reference** вместо неверного bandit-to-system transfer. Не доказан VPN MDP и нет automatic safe rollout |
| Switchback v4,2025-09-17 §2 | Assignment path/carryover unit и строгие условия design | **Design reference**; unknown carryover не заменить fixed wait; optimal schedule paper не копируется |
| Open Bandit Pipeline source ниже | Готовые IPS/DM/DR implementations, explicit action_dist/pscore и evaluator API | **Reference now; future offline adapt** после проверки данных/зависимостей; не ставить в VPN runtime |

**Pinned code:** [st-tech/zr-obp](https://github.com/st-tech/zr-obp/tree/8cbd5fa4558b7ad2ba4781546d6604e4cc3e07c4),
commit **8cbd5fa4558b7ad2ba4781546d6604e4cc3e07c4**, resolved из public GitHub
`currentOid` страницы, не guessed tag. `pyproject.toml` version0.5.5,
Python>=3.7.1,<3.10; NumPy/SciPy/sklearn/PyTorch и plotting dependencies.
Exact [LICENSE](https://github.com/st-tech/zr-obp/blob/8cbd5fa4558b7ad2ba4781546d6604e4cc3e07c4/LICENSE)
— Apache-2.0, copyright2020 Saito/Narita/ZOZO. Это лицензия library, не право
загружать fashion/user dataset и не проверка транзитивных licenses/совместимости.
Ничего не импортировано/не установлено/не выполнено.

Actual example caller: README `ope=OffPolicyEvaluation(bandit_feedback,...IPW/DM/DR)`
→ `obp/ope/meta.py:estimate_policy_values → _create_estimator_inputs`
→ `estimator.estimate_policy_value(**inputs)` → per-round numpy score mean.
В `estimators.py:InverseProbabilityWeighting._estimate_round_rewards` вес
`action_dist[round,action,position]/pscore`, optional clipping lambda; в DR —
model expectation плюс weighted factual residual. Это именно массивы observed
rewards/pscore и target action distribution, не producer VPN assignments.
Метод `estimate_interval` идёт в bootstrap helper. Наличие API/вычисленного CI
не устанавливает для correlated/adaptive VPN данных IID/bootstrap validity.
Не использовать ready defaults как разрешение метрик или гарантию inference.

[OBP primary paper v5,2021-10-26 §2](https://arxiv.org/html/2008.07146v5)
явно задаёт independent product triples. Его fashion benchmark не VPN evidence.
Official VW OPE tutorial найден как альтернатива интерфейса, но exact C++ pin/API
в этом срезе не проверен; direct runtime dependency не предлагается. Повторять
две тяжёлые evaluation системы ради одного estimator незачем.

### 13.7. Оппонент и finite future acceptance

Self-review, не независимое scientific ACCEPTED:

- **Полные randomized session units тоже могут взаимодействовать** через VPS/
  measurement origin/маршрут. Cluster definition и estimand нельзя объявить
  выполненными одним session ID. При неизмеренном interference — ограничить claim.
- **Off-policy иногда достаточно:** если точный вопрос действительно one-step
  на old context distribution и условия выполнены, не требовать full RL. R03-08
  запрещает подмену estimand, а не любой IPS/DR.
- **DR не обязательно требует идеального reward model:** при exact design e и
  корректном predictable fit оно может быть неверным, derivation это сохраняет.
  Нельзя чрезмерной осторожностью отвергнуть доказанный conditional score.
- **Рандомизация после common admission допустима** для узкой admitted population;
  ошибка — перенос результата на весь offered workload, а не сам такой дизайн.
- **Консервативные support bounds могут быть широкими.** Это честная нехватка
  данных, не основание опасного exploration/ложного model confidence.
- **Персональный VPN мал и costly experiments дороги:** сложный evaluator может
  не окупиться. Сначала outcome/trace contract и baseline, затем только реально
  нужный offline method. Улучшение продукта/эффект на ресурсы не измерены.

| Future case | Критерий, которого сейчас не проверяли выполнением |
|---|---|
| Q01 | E1 one-step oracle value не выдан за whole-policy two-step effect |
| Q02 | E2 missing trajectory support/огромные weights честно отражены; no safe-explore bypass |
| Q03 | E3 current-row fitting отклонён или корректно изолирован fit/eval contract |
| Q04 | E4 unsupported target mass даёт bounded identification/unknown, не propensity1 |
| Q05 | Positive tiny propensity и structural zero различаются; finite effective information показана |
| Q06 | E5 clipping меняет estimator/bias claim; hyperparameter/estimator frozen до evaluation |
| Q07 | E6 assigned/admitted/executed/observed denominators сохраняются отдельно |
| Q08 | Assignment Z не переименован в applied channel после cancellation/rollback |
| Q09 | Known task-not-complete≤D отличается от observer missing и user cancel |
| Q10 | Outcome deadline не снимает resource reservation; late cost остаётся unknown/continuing |
| Q11 | Whole-policy arms не меняют hard envelope/required proof/direct fallback |
| Q12 | Epoch/пауза не объявлены washout; shared VPS/carryover scope определены либо unknown |
| Q13 | Adaptive weights/naive bootstrap не получают anytime/drift validity без условий |
| Q14 | Policy selection/model tuning/evaluation separation и family/looks budget объявлены |
| Q15 | Lost trace/export data меняет completeness bound, не фильтруется невидимо |
| Q16 | Privacy/scoped refs/retention и observer cost входят в comparison; no secret/raw user history |

<a id="138-адресная-передача-и-theory-insertion-proposal"></a>

### 13.8. Предлагаемое уточнение теории

**ADP-004/006/008:** reused trace/typed outcomes полезны, но не randomized ledger.
Для будущего design требуется explicit estimand/unit/assignment version/e/follow-up;
не расширять source API молча и не называть ADP-008 закрытым по replay/формуле.
**TH-02/04:** полная admission/attempt/rollback/cancel воронка и owned IDs; existing
retained-running success и `(false,false)` не бинарный task failure/success.
**TH-06:** cost/interference/offered workload/carryover producer и сравнительный
design; не запускать две policies параллельно через shared bottleneck и считать
их независимыми без обоснования. **AND-07/11/16:** same estimand discipline,
добавочно radio/metered/foreground/pending Binder/resource costs; device и field
acceptance отдельны. Ни один platform PLAN пункт этим пакетом не закрыт.

> Предлагаемое уточнение THEORY17: сравнительное доказательство новой
> policy начинается с unit/estimand, не с estimator. One-step value на историях
> baseline не равен эффекту новой адаптивной системы, меняющей будущие истории.
> Assignment, admission, execution, task outcome и actual resource return —
> разные этапы. Post-treatment filtering допуска/успехов может развернуть вывод.
> IPS/DR требуют support и корректного conditional assignment/observation/fit
> contract; structural zero нельзя исправить выдуманной propensity. При unknown
> support/исходах показывать bounds и unknown, не synthetic counterfactuals.
> Clipping, self-normalization, bootstrap и адаптивные CI имеют разные bias и
> validity assumptions. Никакая statistical confidence не разрешает опасный
> action, не продлевает authority и не освобождает pending owner.

Продолжение о дрейфе, повторных проверках и отложенных исходах находится
в §14–15. Авторская готовность не является научным принятием.

## 14. R03-005 — порядок раскрытия исходов и статистика после resets

2026-10-26 · **READY_FOR_REVIEW**. Новое к THEORY17.5: delayed disclosure /
contiguous-prefix contract, average-versus-current estimand и composition между
эпохами/вариантами. Статистического runtime evaluator в срезе нет.

### 14.1. Что уже есть в source и чего это не доказывает

`shared/adaptive/evidence_types.go:Observation.Sequence` — owner-assigned
start/order, не finish time. Есть StartedAt/FinishedAt, Origin/Censoring,
optional CompletionLatency/FollowUp. `ComposeEvidence` выбирает latest decisive
measurement для exact capability; это **не** statistical sample selector/CS.
У Windows `probe_http_observation.go:ProbeObservation` хранит bounded HTTP
BodyComplete/StartedAt/elapsed, у `ProtectedProbe.Run` возможны parallel steps;
у Android WorkerRpcLane наблюдение late return/custody отделено от deadline.
Эти reused seams нужны для честного roster, но сами не дают IID/predictability.

Lifecycle `failoverPolicy.hardReset` — существующий control-state reset;
не утверждается дефект статистического error budget: такой budget не объявлен
реализованным. Предложение относится к **будущему** evaluator, который иначе
может неверно переносить sequential guarantees на lifecycle epochs.

9 local inputs сохранены в cycle-005. Prior004 handoff зафиксирован как input. Raw user logs/native runtime не использовались.

### 14.2. R03-10 — disclosure process входит в measurement model

**F1, finite counterexample:** до запуска назначены N задач с независимыми
Y_i∼Bernoulli(1/2). При Y=1 ответ приходит через 1 unit, при Y=0 — через 100.
После 1 unit все завершившиеся исходы равны 1. При наличии хотя бы одного ответа
completed-only среднее 1, хотя mean назначенных задач 1/2. Event непустого completed
набора имеет probability `1−2^(−N)`. Сортировка этих **только completed** по
Sequence не возвращает пропущенные failures. Anytime method не исправляет
нарушенный conditional-mean/selection model. Это собственная модель, не измерение
проектных HTTP durations. Сам timeout ещё не определяет task outcome.

**Достаточный postprocessing F2:** пусть для **исходного объявленного порядка**
есть действительно валидная sequence C_n:
`P(∀n≥1: θ_n∈C_n)≥1−δ`. Обозначим K(t) длину наибольшего полностью раскрытого
**непрерывного префикса** roster к wall/elapsed моменту t. Тогда тот же event
обеспечивает θ_K(t)∈C_K(t) для каждого finite K(t)≥1. Proof: event пересекает
все n, следовательно включает выбранный n независимо от способа выбора.
K=0 → interval пока отсутствует; pending не заменяется нулём. Arbitrary-random
time property явно поддержана [Howard et al. v9, Lemma3/§1.1](https://arxiv.org/html/1810.08240v9).

Это **условное сохранение уже существующей CS**, не новый способ сделать
невалидные samples валидными. И не proof unbiasedness среднего при случайном K.
Nuisance predictions/weights должны удовлетворять изначальной filtration:
нельзя после получения более позднего Y_j обучить predictor для раннего X_i и
выдать его за predictable. Контекст/assignment/support требования R03-004
сохраняются. θ_K не превращается в θ_N всех запущенных или current quality.

Roster индекс — индекс единиц **заранее объявленного estimator population**.
Owner Sequence может включать другие methods/capabilities/отказы до запуска;
числовой gap сам по себе не доказывает missing task. Нужны bounded roster,
unit/assignment/observation refs и известные planned/started/revealed states.
Raw `latest decisive` join не заменяет statistical roster. Один observation
не counted повторно после нового getter/epoch publication.

Цена F2: ранний никогда не вернувшийся outcome может навсегда остановить K,
хотя многие следующие ответы есть. Это честная потеря мощности/доступности
**optional inference**, а не причина остановить safety controller. Collector
имеет declared cap bytes/units/pending/retention. При потере обязательного roster
entry — bounded incomplete/unknown; не бесконечная очередь и не выдуманный prefix.
Existing MaxReceipts64 не обещает хранить весь lifetime statistical stream.

Другие пригодные варианты: заранее фиксированный follow-up D с валидно
наблюдаемым Y=completion≤D; explicit non-informative observation model; separate
missing-data identification bounds. Каждый меняет условия/estimand. Если сам
task observer пропал или cancel меняет задачу, D не даёт автоматическое Y=0.
IPCW/другую correction нельзя включить без модели observation probabilities.
Ничего из перечисленного не освобождает underlying owner по clock deadline.

### 14.3. Даже правильная nonstationary CS может оценивать прошлое

Сильный контраргумент blanket IID требованию: **не все** CS требуют common mean.
В [Howard v9 §4.1/Theorem4](https://arxiv.org/html/1810.08240v9) bounded adapted
X_t и predictable predictions дают interval для
`μ_t = (1/t)Σ_{i≤t} E[X_i|F_(i−1)]`, average conditional expectation.
One-sided crossing α даёт в указанной two-sided формуле 1−2α; support scale и
prediction условия обязательны. Это не current/future conditional mean.

**F3:** первые 99 conditional means равны 1, сотый и все следующие 0. На t=100
историческая μ_t=.99, current mean0. Достоверный interval для μ_t вправе
оставаться высоким; это не нарушение theorem и не доказательство текущего
хорошего пути. Time-average reward и current-path proof должны иметь разные
названия, scope и consumer. Невозможно получить future guarantee лишь сужением
интервала прошлого при произвольном drift.

Running intersection C1∩…∩Cn сохраняет coverage **одного фиксированного θ**
на uniform coverage event. Для θ_n, меняющегося со временем, это не так:
прошлый interval может честно покрывать иной parameter. Primary §6 отдельно
разбирает этот риск. Пустое пересечение не устанавливает censorship cause;
это model/estimand warning. Restart/change detection требует своего design,
не authority продлить старое protected evidence.

### 14.4. R03-11 — error budget относится к объявленной семье claims

Anytime-valid sequence покрывает время внутри своего statistical question.
Она не выдаёт бесплатно новые сравнения при каждой смене epoch/endpoint,
перезапуске приложения, выборе другого workload/model или новой tuning variant.

**F4:** K независимых null experiments с false-rejection probability δ имеют
probability хотя бы одной ошибки `1−(1−δ)^K`, большеδ при K>1. Без independence
произведение не разрешено, но union bound остаётся доступным. Это ручная модель,
не подсчитанный риск нашего VPN. Per-incident δ может быть честной отдельной
целью; её нельзя подписывать как lifetime/family δ. Вопрос FWER/FDR/среднего
регрета/стоимости ложной смены — явное решение, не универсальный запрет resets.

Один простой bounded scientific design для **family any-error**:
channels/contrasts i, episodes r и variants k получают заранее weights
`w_i,v_r,u_k≥0`, суммы каждого≤1. Выделить `δ_irk = δ w_i v_r u_k`.
Пусть в каждом реально начатом эпизоде conditional probability event E_irk
непокрытия в любом его sample time ≤δ_irk, given admission history. Тогда

`P(∪_irk E_irk) ≤ Σ_irk P(E_irk) ≤ Σ_irk δ_irk ≤ δ`.

Независимость между comparisons/эпохами не нужна; conditional validity каждого
restart нужна. Для счётного r≥1 вариант `v_r=1/[r(r+1)]` суммируется в 1
телескопически. Это **иллюстрация учета**, не подобранная продуктовая allocation
и не maximum-power method. Бесконечный tiny budget теряет мощность; численная
underflow/неподдержаннаяδ должны дать unsupported, не ноль или новый полныйδ.

После data-dependent restart нельзя автоматически считать старые selected
samples свежими conditional-valid observations. Переименование network epoch
не создаёт независимость/model fit, а просмотр того же trace под многими names
не создаёт новый statistical budget. Если измерения не позволяют обосновать
conditional restart, честный вывод — неизвестная validity; правило weights
не исправляет невалидные E_irk. End-user safety monitoring от этого не зависит.

**F5, число сторон:** для two-sided family budget δ_irk в Theorem4 boundary
crossing probability задают δ_irk/2. Нельзя передать полныйδ в each side и
заявить 1−δ. Конкретная library может уже выполнять деление внутри API; прежде
caller надо сверить semantics, чтобы не путать alpha_tuning с coverage_alpha.

### 14.5. Ready reuse: actual caller и честная граница чтения

Existing pinned [confseq5ffe733ca2447a2e28c2c91f3b00086173f2ab2c](https://github.com/gostevehoward/confseq/tree/5ffe733ca2447a2e28c2c91f3b00086173f2ab2c)
переиспользован; MIT LICENSE/README copied в собственный cycle005 с сохранением
baseline. Новым read стал exact
[demo/ate_demo.py](https://raw.githubusercontent.com/gostevehoward/confseq/5ffe733ca2447a2e28c2c91f3b00086173f2ab2c/demo/ate_demo.py):
`estimate_average_treatment_effect → form_predictions(lagged cumulative means)`
→ bounded-support/propensity adjusted Xt → cumulative St/Vt →
`boundaries.gamma_exponential_mixture_bound(Vt,coverage_alpha/2,...)`
→ DataFrame interval. У demo есть отдельный alpha_opt, sample-count array и
known propensity; он не понимает our epoch/custody/roster/delayed observation.
Его `__main__`/test_ate не выполнялись и CSV не загружался.

**Reference/future offline adapt**, не import runtime. README сообщает C++14+
Boost headers, Python и partial R interface, early-stage/not-stable API; эти
слова относятся к exact captured README, не оценка всех нынешних releases.
Numerical boundary core/ABI/Windows/Android packaging в этом срезе не audited.
Direct raw/demo/archive/primary-HTML downloads истекли; errors сохранены. Primary
v9 и demo прочитаны web renderer, bytes полного core не выдуманы. Это ограничение
reuse proof, не утверждение математической/implementation ошибки upstream.

Выбор будущего метода: fixed/stationary bounded question может начать с уже
обоснованного time-union baseline THEORY17.5; empirical-Bernstein полезен при
доказанных adapted/predictable inputs и масштабе, exact demo показывает side
accounting. Простые bounds могут быть широкими. Никакой новый solver/dependency
не нужен для ручных F1…F5, и никакого numerical выигрыша здесь не измерено.

<a id="146-самокритика-future-cases-и-передача"></a>

### 14.6. Контрпримеры и будущая приёмка

Оппонент: completion order **может** быть допустим при обоснованном observation
model/conditional mean; F1 не универсальный запрет. Contiguous prefix —
достаточный conservative postprocessing, не единственный/оптимальный estimator.
Он не чинит ошибочный support/fit и может сделать inference практически немой.
Global FWER может быть излишне строгим для непрерывного сервиса; per-incident
decision-loss цель допустима, если не переименована в global coverage. None of
these choices меняет hard trust/protection semantics. Это opposing self-review,
независимое научное принятие ещё требуется.

| Future case | Предложенный критерий, NOT RUN |
|---|---|
| S01 | F1 fast-success/slow-failure не подаётся как несмещённый completed stream |
| S02 | Roster/start order и completion order различаются; gap Sequence не fake missing unit |
| S03 | F2 arbitrary finite K читает уже valid C_K; K0/pending не invented result |
| S04 | Later disclosed outcome не обучает predictor для раннего prefix sample задним числом |
| S05 | Never-return first task/capped roster → bounded inconclusive без queue growth/reset/slot release |
| S06 | Fixed follow-up observable noncompletion отличается от missing observer/user cancel |
| S07 | F3 time-average μ_t не подписана current/future path quality |
| S08 | Running intersection используется только для совместимого estimand/model |
| S09 | F4 reset/variant/channel family и declared error objective зафиксированы |
| S10 | Conditional restart validity проверяется отдельно от sum weights; нет replay как fresh data |
| S11 | F5 one/two-sided coverage и alpha_opt caller имеют разные semantics |
| S12 | Unsupported tinyδ/underflow/неизвестная model validity → explicit unknown, no reset fullδ |

**ADP-004/006/008:** reuse identities/outcomes, не смешивать coverage join и
scientific sampler; schema для statistical evaluator — отдельный design.
**TH-02/04/06:** bounded roster/follow-up/actual custody; no completion-order
winner filter; cost/power/latency tradeoff и comparison family явно.
**AND-07/11/16:** pending Binder/radio/family/epoch не IID certificate; late
receipt не новый protected permit; device/field acceptance остаётся открыта.

> Proposed THEORY insertion: при запоздалых исходах stochastic model включает
> порядок раскрытия/потери. Anytime validity не исправляет отбор completed-only.
> Уже valid sequence можно читать на раскрытом непрерывном префиксе, сохраняя
> исходную filtration/predictions и корректный prefix estimand; это может стоить
> мощности/ожидания. Для nonstationary bounds надо назвать estimand: историческая
> average conditional mean не текущее качество. Error budget относится к явной
> семье comparisons/эпох/variants; lifecycle reset не даёт global statistical
> guarantee бесплатно. Учет multiple claims не заменяет conditional validity
> каждого анализа и не является полномочием safety controller.

Scientific claims R03-10/11 — READY, не ACCEPTED. Все 12 execution cases NOT RUN.

## 15. R03-006 — решение с горизонтом, дрейфом и ценой перехода

2026-10-26 · **READY_FOR_REVIEW**. R03-12 — условный design candidate
для optional выбора keep / measure / switch suggestion / unknown. Это не
controller, SDK или вычислительный эксперимент. Формулы — собственные условные
выводы; литературные guarantees ограничены своими моделями.

### 15.1. Actual baseline и точка возможного применения

`shared/adaptive/preference.go:Prefer` использует network-class порядок:
mobile HY2/QUIC→VLESS/REALITY; wifi/ethernet WS→HY2/QUIC→REALITY, затем первый
enabled/live-verified descriptor. Unknown/no match/current match имеют разные
reasons. API прямо говорит: нет measured benefit/hysteresis/incident budget.
`Rank` также не прогнозирует useful completion текущей задачи.

Windows: `netwatch.go` после protected report и `verifySessionProtection`,
затем accepted healthy/recovered publication вызывает
`tryPreferredChannelOnNetworkChange → preferredChannelForNetworkChange`
(`transportpref.go:pickDefaultChannel`) → `switchChannelCoreFor(want,current,report)`.
Current-session/report guards и обычный Manager/probe/rollback остаются владельцами
исполнения. Это автоматическая preference после подтверждения текущего пути;
не mandatory recovery и не пользовательский SelectChannel. Class hint сам по
себе не содержит измеренного выигрыша или цены disruption.

**Предлагаемая узкая точка:** будущий optional advisor между выбором `want`
и platform reservation/switch. Он может предложить сохранить current/проверить
ещё/допустить bounded trial; не вмешивается в Start без current, ручной выбор,
hard stop/revoke, fail-closed recovery или порядок failover baseline.
ADP-005 сейчас только unconnected background-suppression prototype, не место
скрыто добавить вторую production policy. Требуется отдельный owner design.

В Android `ADAPTER-MATRIX` различает local physical selection/elapsed facts,
retained RPC custody и будущие AND-07 protected-task receipts. Полного producer
этих rates/transition risks пока нет. OS VALIDATED/полоса/health score не заменят
требуемые входы. Ни одна платформа не объявлена уже подключённой к этому advisor.

Для 23 выбранных локальных входов сохранены хеши до/копии/после. Точные
исходники и версия River указаны в cycle-006 manifests. Чтение исходника
не подтверждает компиляцию или работу runtime.

### 15.2. Кандидат: консервативный выбор для двух путей и одного измерения

Рассматриваются current A и **один** candidate B, предложенный существующим
deterministic baseline. Objective задан владельцем задачи: cost rate j_p(s)
в loss-units/second; меньший лучше. Integral rate и transition cost выражены
в одних loss-units. Energy/bytes/privacy/slots могут остаться отдельными caps;
складывать ms, joules и bytes без принятой функции обмена нельзя.

В probabilistic варианте j/C/G — **conditional expected losses/gains** при
одном joint context/model, а не гарантированный результат каждого пользователя.
Conditional-on-success сравнивает обе potential policies на той же event/model
stratum; простая выборка только удачных switches этого не устанавливает.
Pathwise bounds, если когда-либо доказаны, должны быть отдельным явным видом
envelope. Нельзя незаметно сменить mean certificate на hard delivery promise.

Кандидат не обучает модель. Он получает owned snapshot с идентичностью и
проверяемыми интервалами, делает конечную арифметику, возвращает suggestion
и reasons. Unknown поле не ноль. Все outputs требуют fresh platform gate.

| Слой входа | Минимально нужное | Если отсутствует |
|---|---|---|
| Purpose / applicability | Optional optimization, current A, baseline B, manual/quiet policy, task/loss contract и workload/load/warmth/family/route scope | Not-applicable/unknown; ordinary platform policy продолжает работать |
| Hard/current snapshot | Existing Scope/EvidenceIdentity/clock domain, current required-proof status, authority bound, owner/reservation/epoch/application/image refs | Advisor не выдаёт keep-as-permission; unknown/platform review, hard controller действует самостоятельно |
| Rate evidence | Для каждого пути valid interval [l,u] для объявленного weighted mean, generation/window observation ages, statistical/model coverage и selection/censoring assumptions | Нельзя подставить C2 score/HTTP4KiB latency/OS bandwidth вместо j |
| Drift / bias | Допустимый local rate-change envelope L_p≥0, mean observation age d_p, delivery age upper a_p, observer/counterfactual bias bound b_p≥0, model revision/validity | Unknown model, не L=0 по отсутствию alarm или малой variance |
| Horizon / transition | H∈[Hmin,Hmax] и successful-publication latency τ∈[τmin,τmax] в seconds, successful transition extra-cost range [Cmin,Cmax] | Нет универсального H/τ; context timeout не actual upper bound |
| Whole-trial outcomes | Upper failure-to-apply/usefully-publish probability pmax и failure-gain bounds, либо уже validated complete trial-value envelope | Applied-only gain не объявляется выгодой всей попытки; unknown conservatively p∈[0,1] |
| Optional budgets | Existing admission projection, current occupied owners, attempt/churn/energy/exposure caps; причины запрета/dwell от platform | No new reservation/counter/refund/таймер; старый owner не отпускается по expiry |
| Measurement opportunity | Не более одного q: exact allowed capability, total cost/delay/actual-return envelope, bounded joint outcome/world cells, terminal rules | Gray region остаётся unknown; не бесконечный probe/search |

Collection maxima задаются и валидируются owner contract до обхода. Безопасный
первый design ограничен 2paths/1measurement/1transition; конечные scenario/outcome
arrays имеют явные structural caps, unknown/overflow отвергаются без truncation.
Численные cap values ещё не утверждены и не выдаются за измеренный optimum.

### 15.3. Локализация среднего и старение: размерности и вывод

Пусть generation anchor g_p, наблюдения в валидном declared window имеют
неотрицательные веса w_i с суммой 1, а `d_p=Σ_i w_i(g_p−s_i)` — их средний возраст
относительно anchor. **При допущении** `|j_p(v)−j_p(u)|≤L_p|v−u|` на нужном
интервале weighted population mean отличается от j_p(g_p) не более L_p d_p:
это следует из triangle inequality и суммы весов. Интервал [l_p,u_p] обязан
сначала действительно покрывать этот mean; произвольный postselected ADWIN
window не создаёт такой confidence interval автоматически.

К now=t при `t−g_p≤a_p` и bias bound b_p, для u≥0 получаем

`j_p(t+u) ∈ [l_p−L_p(d_p+a_p+u)−b_p, u_p+L_p(d_p+a_p+u)+b_p]`.

Здесь L имеет units loss/second², d/a/u — seconds, b и l/u — loss/second.
Scope/load/intervention совместимость обязательна: это не вывод causal effect
нового пути из текущего health. R03-004/005 selection/disclosure/error-family
условия сохраняются. Если bound probabilistic, вся следующая математика
условна на соответствующем simultaneous validity event, не безусловная гарантия.

Определим `g−=l_A−u_B−L_A(d_A+a_A)−L_B(d_B+a_B)−b_A−b_B`,
`g+=u_A−l_B+L_A(d_A+a_A)+L_B(d_B+a_B)+b_A+b_B`, `Λ=L_A+L_B`.
Тогда applied-path advantage rate `j_A(t+u)−j_B(t+u)` лежит между
`g−−Λu` и `g++Λu`. Для bounded jumps можно отдельно вычесть/прибавить **заданный**
variation allowance; одно обнаружение change point не оценивает его величину.
Без finite envelope историческое среднее не прогноз current/future rate.

### 15.4. Break-even после задержки, а не только C/H

Для успешного перехода полезная экспозиция B начинается после τ. Пусть C —
**extra** transition loss относительно keeping A на [0,τ]; он включает disruption/
warm-up/observer work в том же objective и не counted повторно в rates после τ.
Для H>τ:

`G_success = ∫_τ^H [j_A(t+u)−j_B(t+u)]du − C`;
`F−(H,τ)=(H−τ)g−−(Λ/2)(H²−τ²)−Cmax`;
`F+(H,τ)=(H−τ)g++(Λ/2)(H²−τ²)−Cmin`.

Integral/extra cost показывают, что именно сравнивается с keep. C не равен
только перезапуску процесса, а τ не равен consumer timeout. Если Hmax≤τmin,
нет полезной B экспозиции до horizon. Если Hmin≤τmax, timely publication не
гарантировано; это unknown feasibility, не доказанная физическая невозможность.
Консервативный первый design требует Hmin>τmax; более общий case нужен отдельно.

При таком rectangle **конечное** вычисление extrema достаточно, без solver:
для lower bound `f(s)=g_lower*s−Λ*s²/2`, где g_lower=g−,
`S−=min{f(Hmin),f(Hmax)}−max_{τ∈[τmin,τmax]}f(τ)−Cmax`.
Последний max проверяет endpoints и clipped stationary point g−/Λ при Λ>0.
Для upper bound `h(s)=g_upper*s+Λ*s²/2`, где g_upper=g+,
`S+=max{h(Hmin),h(Hmax)}−min_{τ∈[τmin,τmax]}h(τ)−Cmin`;
min проверяет endpoints и clipped −g+/Λ. Λ=0 обрабатывается линейно, без деления.
Это own derivation по concavity/convexity, не выполненная numerical routine.

**B1, latency:** g=1,C=4,τ=2,H=5,Λ=0. Если забыть τ, H·g−C=1;
правильная successful-case gain=(5−2)−4=−1. Все units условные, не VPN preset.

**B2, longer horizon не всегда помогает:** g=1,Λ=.1,τ=1,C=1.5.
F−(5,1)=1.3; F−(20,1)=−2.45. При allowed path j_A=1,j_B(u)=.1u
вторая формула точна: более длинное обязательство захватывает разворот выигрыша.
Нельзя всегда подставлять максимальный H или только minimum H. Если u_p есть
прошлый **average** без L/b/correct observation ages, обе цифры не имеют основания.

### 15.5. Applied gain не complete trial value

Eligibility/live-verified/probe-gate не обещают успешного применения. Допустим
S−…S+ действительно bound conditional mean successful-case gain, а все failure/
timeout/late-unpublishable branches имеют conditional mean gain в [−Fmax,−Fmin]
относительно keep, 0≤Fmin≤Fmax. p∈[0,pmax] относится к той же full assigned trial,
horizon/population/scope. Missing result входит в model, не выкинут из denominator.

Law of total expectation даёт conservative whole-trial interval:

`T−=min_{p∈{0,pmax}}[(1−p)S−−p Fmax]`;
`T+=max_{p∈{0,pmax}}[(1−p)S+−p Fmin]`.

Independence failure и path performance **не требуется**: bounds должны быть
conditional на каждом исходе, в одном joint model. Если fail может дать и другой
gain, нужен общий outcome envelope вместо отрицательного-cost частного случая.
Если pmax неизвестен, диапазон [0,1], не p=0. Ресурсы за horizon и pending
retirement всё равно сохраняют отдельный custody/accounting contract.

**B3:** S−=1.3,Fmax=4. При pmax=.1 T−=.77; при .5 T−=−1.35.
Это не калибровка probabilities. Для S−>0 достаточный break-even
`pmax < S−/(S−+Fmax)`; margin добавляется до сравнения. Качественный applied
канал без trial reliability envelope ещё не основание whole-trial superiority.

### 15.6. Полный конечный decision rule

1. **Hard authority вне advisor.** Missing/unsafe/current mismatch → unknown/
   platform review. Не писать keep как разрешение сохранять опасную сессию.
2. **Applicability/feasibility.** Optional automatic branch, current A и baseline
   B, same task/evidence/model, clock/expiry, existing admission/custody/quiet/dwell.
   Ничего не резервировать. Unknown critical input → explicit unknown.
3. **Whole-trial certificate.** Вычислить T−/T+ по принятому model, с finite
   units/numeric validation. `T−>margin` → switch suggestion; `T+≤margin` → keep
   для данного candidate/question/horizon. Equality/uncertainty reasons различны.
4. **Gray region.** Только если q допустим, своевременен, bounded и есть joint
   outcome map, сравнить keep с одним contingent measure-then-act plan. Терминал
   plan предлагает switch только после обновлённого T−>margin и fresh gates;
   остальные исходы keep/unknown, не fake success.
5. **Measurement objective явно minimax regret**, не expected utility без prior.
   В finite joint cells w oracle comparison `O(w)=max(0,G_switch_now(w))`
   имеет тот же horizon/cost/feasible-action set. Для plan q взять certified
   upper bound `R_q^U=max_w max_{y∈Support(q,w)}[O^U(w)−G_q^L(w,y)]`;
   для keep lower bound `R_keep^L=max_w max(0,G_switch_now^L(w))`.
   Только `R_q^U+measurement_margin<R_keep^L` и полные caps → measure suggestion.
   Это sufficient comparison **в объявленном enclosing model**, не universal
   minimax optimality; отсутствующий model или нестрогое сравнение → unknown.
6. Owner заново проверяет Expected identity/model/clock/current gates и actual
   reservations **после** advice; затем обычное исполнение/settlement. Changed
   epoch/model, late report и expiry не получают новую authority по suggestion.

Outputs: action/reason, A/B/q reference, Expected scoped inputs, loss/model/
horizon refs, numeric bound **с явным guarantee scope**, validity upper bound,
RequiresPlatformGate=true. Keep означает отсутствие optional transition, measure
не network permission, switch означает предложение owned bounded trial.
Startup/manual/hard recovery не ждут этого расчёта. Model alarm не создаёт WFP/
TUN/DNS/OS epoch changes и не сбрасывает retry/statistical budgets.

**B4, measurement может и не помочь:** два joint worlds имеют G_now=+6/−6.
Keep worst regret6. Perfect q ценой 1; после его delay good switch gain5 даёт
plan net4, bad outcome keep даёт−1: regrets2/1, worst2<6. Если разрешён q-timeout,
сохраняющий оба мира, good-world timeout+keep даёт−1 и regret7: worst-case q
теперь хуже keep. Нельзя исключить censored outcome ради measure. Это показывает
и пригодность bounded plan, и его консервативный предел. Нужны сведения о q,
а не больше названий bandit algorithms; подробный probability/risk design —
следующий самостоятельный вопрос, а не implicit change objective здесь.

### 15.7. Change detection и bandits: что действительно переносится

| Подход / primary scope | Пригодное | Что не переносится в VPN без дополнительных данных |
|---|---|---|
| Bifet–Gavaldà ADWIN draft2006-10-17 §3, отдельная SDM2007 publication | Adaptive window/mean-change hint; bounded independent observations в draft setup; Theorem3.1 false-alarm bound сформулирован **на шаг**, не lifetime guarantee | Нет alarm не доказывает L=0; изменение measurement mix/observer/shared origin может менять mean без path degradation; elapsed detection bound не следует из sample-clock |
| Garivier–Moulines arXiv0805.3415v1,2008-05-22 §1–3/Theorem7 | Discounted/sliding-window UCB и явная bias/old-data цена | Piecewise-stationary reward model, bounded/independent noise и tuned window/discount; их regret включает breakpoints и gap assumptions. Missing/delayed/censored/expensive switches не исчезают от названия SW-UCB |
| Wu et al., Conservative Bandits ICML2016 §2/3/Theorem2 | Baseline-relative performance budget и separate exploration cost | Они также рассматривают adversarial variant; не отвергать blanket словом drift. Но expected/high-probability cumulative reward constraint **не** fail-closed safety/trust/instant task SLA, и не весь наш lifecycle |
| Dekel–Ding–Koren–Peres arXiv1310.2997v2,2013-11-19/STOC2014 | Fundamental information/switch-cost tradeoff; worst-case lower bound | Benchmark best **fixed** action, oblivious [0,1] loss sequence и explicit switch cost. Ни динамический oracle без цены, ни реальная РФ-сеть не получают их rate автоматически |
| R03-006 finite-horizon advisor | Explicit age/drift/τ/horizon/failure cost, deterministic bounded suggestions и abstain | Требует новых качественных producer inputs. Без них это design returning unknown, не proven improvement production |

**B5, adversarial/nonstationary boundary:** после одинаковой истории возможен
скачок качества перед actual switch publication. Отсутствие detector alarm
не даёт finite future slope, и certificate со взятым по умолчанию L=0 ложен.
Даже full-information выбор instant best может проиграть по total cost:
при чередующихся A/B losses0/1 и unit switching fee chasing winner стоит почти
по1 каждый шаг, fixed path в среднем1/2. Это own counterexample, не доказательство
общего linear regret. Lower bounds papers не превращены в product predictions.

### 15.8. Готовый source reuse: River ADWIN, exact pin/caller/цена

Public GitHub currentOid закреплён как **b674b758518341bc5c343ca113f34903ba55020a**.
`pyproject.toml` River0.26.1, Python>=3.11, NumPy/SciPy/Narwhals; Rust/PyO3/
maturin module river._river_rust. Internal Cargo crate0.1.0 — не Python version.
LICENSE BSD-3-Clause, copyright2020 river developers; notice/dependency review
нужны при дальнейшем code adaptation, новых dependencies сейчас нет.

Actual chain: `river/drift/adwin.py:ADWIN.update(x)` при предыдущем alarm делает
`_reset`, затем helper.update; `rust_src/pyo3_bindings.rs:RsAdaptiveWindowing`
зарегистрирован в drift module и вызывает
`rust_src/adwin.rs:AdaptiveWindowing.update → insert_element → detect_change`
→ sample tick modulo clock, bucket cuts → evaluate_cut(mean difference,
variance,width,delta) → delete old bucket/boolean drift → Python drift_detected.
Wrapper defaults clock32/grace10/max_buckets5/min_window5 — source defaults,
не time/energy/false-alarm/Snowden presets. Parameters/input финичность/диапазоны
должен валидировать adapter; выбранный source не назван safe input boundary.

Buckets сжимаются/растут при stream length; это не hard constant-memory cap.
Не получено runtime WCET/RSS/Android wakeup estimate. Ready **reference / future
offline adapt** observable change hint, затем если доказана польза — изолированный
bounded adapter/port review. Direct import Python/Rust ML framework в Go/Wails/
Kotlin/JNI не обоснован. Из source не копируется второй controller. Прочитаны
wrapper/actual Rust core/bindings/LICENSE/manifests, ничего не запускалось.

### 15.9. Оппонент, перенос и конечная приёмка NOT_RUN

Сильнейший objection: чтобы получить полезные S/T bounds, нужны достаточно
честные local rate/causal bias/drift/transition-outcome inputs; реальная сеть
может дать лишь unknown. R03-006 не устраняет этот information limit. pmax/Fmax
нельзя оценить только на успешных applied trials, L нельзя доказать no-alarm.
Низкое worst-case regret measurement часто отклонит q с возможным timeout;
другая допустимая risk/expected-regret цель требует явного нового contract.

Interval conservatism/длинный horizon могут отвергать реально полезные switches;
это цена выбранного guarantee, а не optimality. Корректная более простая fixed
preference может быть дешевле при отсутствии instrumentation. Улучшение должно
появиться в paired useful-task/false-switch/churn/resource outcomes, а не в
красивой формуле или source library. Small VPS/Android energy реальные costs
не измерены, неизвестные поля не заполнены. Это opposing self-review, не ACCEPTED.

| Future case | Предлагаемый criterion |
|---|---|
| P01 | Hard revoke/unsafe/unknown current authority не превращён в keep permission и не ждёт optional optimizer |
| P02 | Same-class/manual/startup/not-applicable branches сохраняют scope и явный owner intent |
| P03 | Window mean→anchor→now→future localization использует generation ages/bias/L, не CompletedAt или no-alarmL0 |
| P04 | B1 latency penalty предотвращает ложный positive gain |
| P05 | B2 H-range extrema проверяют оба horizon endpoints и допустимые stationary points; Λ0/unknown/overflow различны |
| P06 | Applied-only certificate не выдан за whole-trial guarantee; B3 pmax/cost sensitivity сохраняется |
| P07 | Censored/not-published/failure/rollback учитываются в complete trial population и costs; pending owner не released |
| P08 | B4 q-timeout включён в joint support и может изменить measure→unknown |
| P09 | Measurement/transition не начинает работу без actual admission; stale Expected отбрасывается |
| P10 | Drift alarm меняет optional-model validity, не trust/WFP/epoch/retry/error budgets |
| P11 | Sample-count detector clock не подписан wall-time SLO; varying sampling/observer mix видимы |
| P12 | Structural caps/model-support/units validity отвергают malformed/unknown без truncation/fake probabilities |
| P13 | Baseline и advisor сравниваются при same offered load/workload/authority и assigned denominator |
| P14 | Measured useful completion/recovery/churn/disruption/CPU/energy и planner cost сравниваются отдельно; отсутствие эффекта допустимо |

**ADP:** предложить отдельный review item после extraction/adapter acceptance,
не править Prefer/ADP-005 скрыто. **Windows:** узкий future consumer в automatic
network preference; current required proof и существующий switch owner сохраняются.
**Android AND-07/11/16:** сначала accepted measurement/task/outcome producer,
elapsed/JNI/retained custody и radio/cost bounds; OS hints/батарея не rate evidence.
R02 даёт threat/change mechanisms, R04 — cost/time/interference, R05 — hard gates,
R06 — task/loss/horizon; результаты за них не выдуманы. Full experiment/device/
field execution не разрешён и все 14 cases **NOT_RUN**.

> Предлагаемое уточнение adaptation chapter: optional switch
> сравнивается по полезной экспозиции после actual transition delay и полной цене
> попытки на объявленном horizon. Statistical window, delivery age, drift/bias и
> failure-to-apply uncertainty дают разные поправки. Большой H не всегда помогает
> при дрейфе. Applied-path gain не whole-trial value. При отсутствии enclosing
> model допустим unknown с сохранением ordinary platform policy. Detector alarm,
> отсутствие alarm и bandit regret не создают safety authority или resource budget.

R03-12 готов к независимому review; это один кандидат, не «лучшая policy».
Следующий существенный вопрос: как собрать пригодный full-transition risk input
из staged/частично наблюдаемых outcomes, не перемножая неверные marginal rates.

## 16. R03-007 — полный риск попытки из этапов и частичных исходов

2026-10-26 · **READY_FOR_REVIEW**.
Продолжает конкретный advisor006: откуда взять пригодные pmax/Fmax, а не ещё
один score. Новые предложения **R03-13** (coherent conditional trial risk) и
**R03-14** (deadline success не mean resource-cost bound). Никакого исполнения.

### 16.1. Actual outcomes не образуют один success bit

Windows `switchChannelCoreFor`: current/intent guard → prepare/clock retention
→ selector-only SwitchChannel либо только при ErrSelectorSwitchUnavailable
structural Reload → error/verified rollback либо current accepted publication.
Это ветвящийся protocol, не единый независимый набор component tests.
`finishObservedSwitch` различает current apply, verified old-path recovery,
discard и failure; `(false,false)` и breaker Closed не единый task result.

API1 `SettleFailover` при no OwnerReturned возвращает AwaitReturn. Discard
сбрасывает control failures success-shaped способом, но **не** измеряет network
success. Applied/rollback с current publication различны; MustKeepResourceReservation
зависит отдельно от ResourcesRetired. Допустимый source contract сочетает
OwnerReturned/PublishedCurrent с ещё не retired resources; source branch не
объявлен выполненным runtime сценарием.

Android `WorkerRpcLane.takeCompleted` переносит фактический result в retained
delivery и очищает pending; `submit` требует одновременно pending==null **и**
delivery==null, `acknowledge` привязан к exact Call. Один pending==null поэтому
не означает свободную lane или завершение остальных владельцев. Existing IDs/
custody следует reuse, а не создавать статистический параллельный lifecycle.

Три разных estimands для **одной заранее объявленной assigned когорты**:

| Estimand | Что нужно доказать наблюдением | Чего недостаточно |
|---|---|---|
| A(D): candidate accepted/published by deadline | Parent accepted current apply receipt до D по исходному scope/clock | nil error, worker attempted, prepared config или rollback old path |
| U(D): useful user task completed by deadline | Отдельно определённый task observer/goal; cancellation semantics заранее | Protected probe success или A(D) сами по себе не пользовательская работа |
| R(D): actual resource return/retirement by deadline | Existing exact owner/retirement facts, включая retained delivery | End span, observation timeout, pending==null, A(D) либо U(D) |

Rollback может сохранить рабочий путь, не применив B; U при этом ещё требует
своего наблюдателя. Unpublishable/cancelled не означает censorship cause.
Для pmax006 выбирается именно соответствующее trial goal, не control failure
counter или доля «успешных» current reports. Все private/raw user logs не читались.

### 16.2. R03-13 — три разных способа композиции

Для объявленных stage-success events E1…Em верна chain rule:

`P(E1∩…∩Em)=∏_i q_i`, где
`q_i=P(E_i | E1∩…∩E_(i−1), same cohort/context/policy)`.

**Independence не требуется**, потому что вероятности условные. При zero-mass
prefix дальнейшая conditional probability не определена; не подставлять «100%
из 0». Prefix success уже 0, либо uncertainty сохраняется явно. Когда q_i имеют
одновременно valid bounds [l_i,u_i], monotonicity даёт success interval
`[∏l_i,∏u_i]`, failure upper `1−∏l_i`. Это conditional **model** interval, а не
confidence из одних конечных counts. Validity event отдельных bounds объединяется
union budget Σδ_i, не произведением «доверий».

**C1, finite cohort identity:** N=100, stage1 pass80/reject20, stage2 reached80,
pass72/fail8. Conditional empirical rates .8 и .9 дают .72=72/N. В общем при
полном nested roster `n_(i+1)=passes_i` произведение `∏n_(i+1)/n_i`
телескопируется в final_success/N. Это полезный accounting check, **не новая
информация/не дополнительная statistical power** по сравнению с final counter.
Если использовать 72/N как stage2 conditional rate, ещё умножить на.8 — получится
.576, ошибка denominator. Ни одна из этих цифр не measured Snowden probability.

Два других вида входов требуют другой операции:

- **Unconditional first-failure attribution:** если каждый trial получает ровно
  один mutually exclusive terminal first-failure reason, общая failure mass
  равна сумме этих masses. Local admission reject может входить в trial failure
  для A(D), оставаясь не network failure. Missing терминал не исключать.
- **Overlapping symptom/component events:** одинаковый trial может иметь DNS/
  HTTPS/observer errors сразу. Сумма — лишь union upper bound; multiplication
  marginals требует дополнительных assumptions. Для двух failure events с
  probabilities f1,f2 без них union лежит между max(f1,f2) и min(1,f1+f2).

**C2:** два component-success marginals .9/.9 дают conjunction от.8 до.9;
.81 — значение при independence, не установленное значение. При полном common
outage conjunction=.9 и conditional second-pass given first-pass=1.
[NIST series model](https://www.itl.nist.gov/div898/handbook/apr/section1/apr182.htm)
явно требует component independence/first failure для произведения marginal
reliabilities. Наш conditional chain — другой elementary probability аргумент,
не применение этого physical lifetime model к VPN.

Для ветвей B_b с полным mutually exclusive partition:
`P(goal)=Σ_b P(B_b) P(goal|B_b)`. Внутри каждой ветви можно применять conditional
chain. Selector-only success, unavailable→Reload, verified rollback и cancellation
нельзя перемножить как независимые обязательные этапы. Retry loops требуют
bounded unfolding по existing attempt budget/receipts, не бесконечного graph.
Branch mix при новой policy тоже может измениться; historical mixture не
counterfactual probability без R03-004 условий.

**C3, retries:** три попытки с одинаковым hidden common outage probability.2
могут все проигрывать с probability.2, не .2³. Если вместо marginals действительно
доказано `P(fail_k | all prior fails, full current history)≤r_k`, successive
conditioning даёт `P(all fail)≤∏r_k` без independence. Это не разрешение ещё
одного retry/сброса reservation; реальные adaptive histories/контексты обязаны
соответствовать assumptions, а не выборке только вернувшихся запросов.

### 16.3. Данные для usable risk envelope

Proposed **analysis contract**, не новый implemented SDK/logger:

| Поле/инвариант | Назначение |
|---|---|
| Goal/Population/Policy/StageSchema refs | Одна assigned unit, current action/version, declared A/U/R goal и horizon; trial/RPC/task не смешиваются |
| Existing scoped identity | Intent/attempt/observation/session/network/application/image/cutover и clock domain из owner; not raw credentials/profile/traffic |
| Branch/step lineage | Конечный known branch schema, prefix reached/first terminal reason; replay/late duplicate не новый trial |
| Cohort conservation | Для reached stage: reached=passed+failed+unknown; nested successor=passed только если таков actual protocol, иначе explicit gate/branch |
| Follow-up / unknown | Parent deadline outcome, late/missing observer, cancellation/authority loss, not-reached и local rejection раздельны |
| BoundKind / ModelRef | Finite-population identification, statistical CI и model/predictive bound — разные типы; confidence/error family и validity/model age явно |
| Resources / cost | Actual held/returned/retired states, cost horizon и tail assumptions; p(A) не заменяет p(R) или Fmax |
| Bounded storage / privacy | Caps units/steps/outcomes/retention и incomplete-count witness; opaque refs, без raw user logs/OTLP export по умолчанию |

Если одни и те же данных не хватает для conservation, builder возвращает
incomplete/unknown, а не «достраивает» этап из generic Ok. Не требуется записывать
полную packet/application history. Оценка каждого q_i относится к фактически
достигнутому prefix и своему stratum; same function name/SDK version не доказывают
invariance после смены transport/load/network или новой policy.

**C4, missing:** из 100 assigned известно 80 goal success,10 non-success,10 unknown.
Finite success fraction находится в [.8,.9], failure fraction [.1,.2]. Это не
future pmax=.2 и не 1−δ confidence interval. Для будущей вероятности нужны
sampling/stability/model assumptions. Если receipt отсутствует из-за owner stop,
точный goal может быть known0 для A(D), но U(D)/R(D) оставаться unknown.
Если user изменил intent, original estimand/competing event сохраняются по
предварительному contract, не скрываются post-treatment exclusion.

**Сильная более простая альтернатива:** когда final goal наблюдаем для полной
когорты, сначала оценивать full-trial rate напрямую. Stage product полезен для
provenance/локализации пробела, но несколько маленьких reached cohorts и split
δ могут дать более широкие bounds. Нельзя требовать сложный event-tree evaluator
ради уже достаточного final outcome и дешевизны маленького VPN проекта.

### 16.4. Даже zero failures не даёт pmax0

При **fixed n**, полных independent Bernoulli trials с одной failure probability
p, observed0failures, one-sided binomial inversion даёт
`p_upper=1−δ^(1/n)` для confidence1−δ. Вывод вручную: вероятность увидеть 0
failures равна (1−p)^n; при p=p_upper онаδ. Это частный случай exact-binomial
подхода [NIST](https://www.itl.nist.gov/div898/handbook/prc/section2/prc241.htm),
не выполненный solver/Dataplot command. n=0 → no information, не division/zero risk.

В 006 при S−>0 и Fmax≥0 условный risk threshold `r=S−/(S−+Fmax)`.
Даже в этом простом zero-failure model, чтобы p_upper<r, требуется
`n>log(δ)/log(1−r)` при 0<r<1. Это planning inequality, не назначенное число
live trials. Она показывает измерительную цену высокой цели; requests/time/
energy/admission caps проверяет owner/R04, испытания этим срезом не разрешены.

Adaptive repeated looks/stopping, drift, correlated retries, outcome-dependent
missingness или несколько смешанных cohorts делают простой binomial plug-in
необоснованным. Нужен valid sequential/conditional analysis, не reset fullδ.
Для переноса прошлого p на будущий horizon нужен justified stability/variation
envelope; если он отсутствует, n большим не лечит неизвестный future drift.
p/S bounds должны быть совместно valid на объявленном error event; no independence
confidence levels из числа разных producer refs.

### 16.5. R03-14 — deadline probability и хвост cost/retirement различны

**C5, own heavy-tail counterexample:** T принимает2^k time-units с probability
2^(−k), k≥1. Масса суммируется в1. Для D=2^m вероятность T>D равна 2^(−m),
сколь угодно мала при большом m, но `E[T]=Σ_k 2^k·2^(−k)=∞`.
При positive held-resource cost rate r expected cost rE[T] также не ограничен.
Это математическая модель, не измеренное распределение Go/Binder/VPS.

Поэтому высокий P(return≤D), даже подтверждённый, не даёт mean/WCET cleanup
bound/Fmax006. Реальный process/container/FD/Job/resource cap может ограничить
amount, но duration и её tail остаются отдельными. Нельзя молча принять
exponential distribution/constant hazard по форме backoff или одному timeout.

Finite-horizon cost `E[min(T,H)]` может быть bounded при том же infinite E[T].
Это позволяет честно сравнить cost **только доH**, если objective такой. Но
OutstandingCustody заH остаётся настоящей: её не refund/reset/release ради
следующей попытки; future occupancy эффект R03-08 / REPORT§13 остаётся.
Для total-cost claim нужен настоящий tail/actual-retirement bound либо явный
residual-risk/неполный горизонт. Safety gates не заменяются probabilistic SLA.

### 16.6. Ready reuse и transfer cost

**Reuse first:** existing AttemptReceipt/AttemptOutcome, PublishedCurrent,
OwnerReturned/ResourcesRetired и Observation stages/censoring из API1. Current
source уже различает control settlement и evidence, но не предоставляет
готовый calibrated risk estimator/cohort collector. Producer/adapter ownership
остаётся у платформ, данные/версионирование обсуждаются с ADP owner. Рисковый
агрегатор не становится вторым budget/transition owner.

**Reference:** NIST series/complex-system models и exact-binomial interval
документация задают полезные assumptions; physical nonrepairable components и
IID trials не объявлены нашим lifecycle. Dataplot/R examples не выполнялись;
code/ABI/license выбранного numerical solver не inspected, dependency не нужна
для finite ручной арифметики. Реальная библиотека для inference выбирается после
dataset/model contract; не устанавливать solver для отсутствующих данных.

**Optional vocabulary reference, не telemetry внедрение:**
[OpenTelemetry Tracing API, SetStatus/End](https://opentelemetry.io/docs/specs/otel/trace/api/)
различает default Unset/Ok/Error, а End не завершает child spans. Сам стандарт
не определяет наши A/U/R outcomes и не аутентифицирует owner facts. `Ok` может
подавить error status; поэтому заменять typed outcomes одним span status нельзя.
API read/capture date2026-10-26, не pinned implementation audit. OTel SDK/exporter
не добавлен, data export не разрешён и storage/network overhead не оценён.

13local source/doc captures, NIST2 successful captures; OTel filesystem request
timeout, официальный текст прочитан web. Exact URLs/UTC/hashes/errors в cycle007.
Для 006 upstream River license/pin/code review сохранён; новым 007 внешнего code
не копирует. No tests/fixtures/build/simulation/native/crypto/security scans/live.

### 16.7. Оппонент и конечные будущие случаи NOT_RUN

Главный objection: корректная chain rule не даёт probabilities из отсутствующих
данных. Conditional stage law может измениться именно из-за того, что новая
policy достигает другого prefix; полный causal/model transport не доказан
агрегированием. Для многих целей прямой full-cohort outcome лучше/дешевле.
Heavy-tail example показывает логический предел, не причину паниковать о реальном
runtime: фактический хвост может быть bounded, но его здесь не измеряли.

| Case | Future criterion |
|---|---|
| R01 | A(D),U(D),R(D) и interval kind не смешиваются; rollback не applied B/не fake user success |
| R02 | C1 conditional denominators telescoping совпадают с полным final count; wrong denominator rejected |
| R03 | Conditional chain, disjoint first-failure sum и overlapping symptoms имеют разные algebra/contracts |
| R04 | C2 common cause не превращён в independence/product; нулевая reached population не 100% |
| R05 | Selector/Reload/rollback/cancel branches partition complete; retry attempts не новые incident units |
| R06 | C3 conditional retry bound не подменён marginal, no extra retry/refund permission |
| R07 | C4 unknown branch сохраняет bounds; parent deadline/observer loss/user intent имеют точные semantics |
| R08 | Breaker reset/failure counter не statistical denominator; retained delivery не free slot |
| R09 | Zero-failure p_upper/n planning используется лишь при declared valid sampling/δ/model; drift не скрыт |
| R10 | C5 small deadline-failure probability не выдана за finite mean/WCET/cost tail |
| R11 | Cost horizon H и outstanding custody послеH явно показываются; no synthetic retirement |
| R12 | OTel End/Ok не owner/resource/task proof; caps/privacy/lost-record completeness и no export enforced |

**ADP-004/006/008:** отдельный coherent RiskEnvelope producer/review request,
не считать failure arithmetic estimator. **Windows:** actual branch/receipt/
publication/custody lineage, без bool flattening. **Android07/11/16:** retained
RPC delivery/ack/retirement и useful-task observation отдельно; no fabricated
production measurement. **R04:** real amount/duration/tail/cost producer,
**R05:** hard gate/receipt semantics, **R06:** A/U goal/competing intent/horizon.
Ничего за другие роли не реализовано/не запущено, их планы не закрыты.

> Предлагаемое уточнение: risk полной попытки относится к одной
> assigned когорте/goal/clock и complete branch schema. Conditional stage pass
> probabilities можно перемножать по chain rule без independence; произвольные
> marginal reliability rates — нельзя. Control counter reset не успех опыта.
> Parent publication, useful completion и actual retirement имеют разные outcomes.
> Confidence от past trials не future drift guarantee. Deadline-success probability
> не mean cost/retirement bound; хвост за horizon и custody остаются явно открыты.

R03-13/14 READY, не ACCEPTED. Двенадцать будущих проверочных сценариев NOT_RUN.

## 17. R03-008 — producer observations → interval или abstain

Source-срез от 26 октября 2026 года.
**R03-15 / R03-16 READY_FOR_REVIEW.** Это научный proposal, source/manual evidence;
никаких изменений estimator/producer/product или выполнения. Passport —
[handoff008](handoffs/R03-008.md); primary/source provenance — SOURCES§10 и cycle-008.

### 17.1. Что уже есть и чего нельзя получить из existing API

Scoped source capture17 файлов сделан 11:13…11:14Z; before/copy/after совпали.
`shared/adaptive/evidence_types.go:Observation/EvidenceReceipt` уже дают exact
identity, start-order Sequence, state/origin/censoring, partial follow-up, bounded
payload, pressure и independent custody. `ComposeEvidence` решает coverage/latest
decisive status, **не** оценивает likelihood, среднее, causal effect или forecast.
`PlatformAccepted` — доверенная проекция caller, библиотека не аутентифицирует
источник. `MaxReceipts=64` ограничивает snapshot, не полный dataset/lifetime.

Windows `appProtectedProbe.Run` выделяет observation ID перед работой; finalizer
сохраняет original start/finish и current intent/epoch/owner/session checks.
`fetchProbeHTTPRequest` фиксирует bounded HTTP response, stage, status, bytes,
elapsed; `checkHTTPSVia` проверяет статус/формат IP. Это факты конкретного workload,
не наблюдение успешного звонка/доступа к рабочему ресурсу/скорости bulk transfer.
`channelHealth.observeInEpoch` хранит OK/FAIL после own filters, включая пропуск
measurement-only failures. Это полезный policy input, но **не полный assigned
cohort** для новой confidence procedure. Наличие фильтра здесь не объявляется
дефектом existing health policy; статистическое переиспользование меняет задачу.

Android `WorkerRpcLane` имеет pending→retained delivery→exact acknowledge.
Транспорт результата/его доставка не создают useful-task sample. По рассмотренному
adapter map full Android measurement producer отсутствует. Ни одна из платформ
пока не обеспечивает полный нижеописанный contract; это owner design request.

### 17.2. Три цели, три разных результата

Предлагается **одна** чистая процедура `BoundRegisteredLoss` с tagged результатом:

| TargetKind | Что ограничено | Чего результат не утверждает |
|---|---|---|
| finite_cohort | Средний реализованный bounded loss конкретных зарегистрированных единиц, включая неизвестные outcomes | Не probability следующего опыта |
| historical_conditional_mean | Среднее conditional expectations этих единиц относительно объявленного analysis filtration | Не marginal iid mean, current rate или causal contrast |
| horizon_model | Только отдельный accepted bridge из historical interval в rates/risk/cost на horizon006 | Bridge не выводится из маленькой variance/no alarm/свежего HTTP200 |

Отсутствие bridge сохраняет полезный historical result, но возвращает
`abstain_for_decision: target_transport_unknown`. Нельзя переименовать historical
upper bound в `pmax` следующего switching trial. Finite-cohort interval не
confidence interval; statistical miscoverage α и operational failure risk p —
разные величины. Вектор CPU/bytes/joules/outage отдельно, без придуманных весов.

### 17.3. R03-15 — минимальный вход и bounded algorithm

**Producer-owned inputs, proposal, не новый API в shared коде:**

1. `GoalRef/UnitRef/PolicyRef/ProtocolRef/PopulationRef`: точное событие/loss,
   задача и D, включение admission/cancel/reject, normalization/support, условия
   probe-on/off, warm/cold и owner execution policy. Для clipped latency/cost
   результат относится именно к clipped loss, не total tail.
2. Existing exact evidence identity/clock/scope и platform acceptance source.
   `RegistrationRef`, pre-work ordered unit IDs и completeness witness. Дедупликация
   по operation/unit, а не количеству callbacks/HTTP stages/endpoints.
3. Конечные owner caps K streams, N units/stream, pending slots, retained bytes,
   query-work cap и lifetime. Нет чисел по умолчанию, выданных за measured budget.
   Переполнение → freeze/abstain, неизвестные attempts не вытесняются как успехи.
4. Для каждой mature unit i — interval `[a_i(s),b_i(s)]⊆[0,1]` на snapshot s.
   Полностью честный task outcome даёт point; честный partial follow-up даёт
   interval по определению goal; неинформативный/неверифицируемый даёт `[0,1]`.
   Timeout-duration не completed RTT. Y_i — именно declared loss этой unit.
5. **Отдельный** error envelope Γ_n(s) с producer/model provenance: на каждом
   используемом snapshot истинная сумма S_n удовлетворяет
   `Σa_i−Γ_n ≤ S_n ≤ Σb_i+Γ_n`. Например, не более k неверных bounded labels
   дают Γ_n=k; если k неизвестно, разрешён conservative Γ_n=n, не 0.
   Это предпосылка с указанным владельцем, не оценка из числа красивых ответов.
6. Для statistical mode — `AnalysisOrderRef/FiltrationRef`, bounded Y_i,
   alpha-family/stream allocation и mature-prefix identity. Минимальный достаточный
   producer вариант — неперекрывающиеся заранее зарегистрированные episodes:
   follow-up unit i завершён до назначения i+1. Телеметрия может отсутствовать;
   underlying task loss определён к концу follow-up. Analysis filtration включает
   past latent losses и информацию, доступную до следующего назначения. Это
   определяет estimand явно; latent filtration нельзя выдать за пользовательский
   available-state forecast. Существующий overlapping pipeline сам такой witness
   не даёт. Без корректной альтернативной filtration — только finite_cohort.
7. Для horizon mode — отдельные bridge refs с bounded age/selection/observer bias,
   drift/valid horizon, task transfer, trial failure/cost и remaining custody.
   Их проверяют platform/model owners; unknown остаётся unknown.

**Порядок:**

- Проверить schema/identity/goal/support/order/completeness/caps. Malformed/unknown
  provenance не улучшает interval. Hard safety/current mismatch немедленно
  возвращается owner gate; statistical score не разрешает трафик.
- Для complete registered prefix n>0 вычислить finite envelope
  `I_fin(n,s)=[max(0,(Σa_i−Γ_n)/n), min(1,(Σb_i+Γ_n)/n)]`.
  n=0→unknown. Недостающий registered result остаётся interval, а не удалением
  denominator. Если неизвестно само число/состав units — completeness_unknown.
- При validated statistical contract вычислить radius r_n ниже и
  `I_hist(n,s)=[max(0,(Σa_i−Γ_n)/n−r_n), min(1,(Σb_i+Γ_n)/n+r_n)]`.
  Неверный/пустой interval — contract/model failure, не confident success.
- Вернуть bound kind, exact target/prefix/clock/model refs, uncertainty decomposition,
  cap/completeness status и why-not-horizon. Только полный совместимый bridge
  допускает передачу envelope в advisor006. Дальнейшие admission/switch/cleanup
  остаются у existing platform owners.

Хранение proposal ограничено `O(KN)` metadata с bounded identifiers и pending
intervals; один прямой пересчёт `O(KN)`, либо проверяемые достаточные aggregates
для закрытых intervals. Это алгоритмическая оценка по выбранным caps, **не**
измеренный RAM/CPU/energy/WCET. Позднее refinement с тем же n допустимо только
при valid envelope для каждого используемого snapshot; оно не регистрирует новую
unit, не меняет goal задним числом и не возвращает resource reservation.

### 17.4. Почему historical bound anytime-valid и что это не лечит

Ниже собственный простой вывод; это conservative baseline, не заявление новой
концентрационной теоремы или оптимальной ширины. Пусть для последовательности
Y_i∈[0,1], adapted к объявленному G_i, `μ_i=E[Y_i|G_(i−1)]`,
`A_n=Σμ_i`, `S_n=ΣY_i`. Conditional bounded-MGF даёт
`E[exp(λ(Y_i−μ_i))|G_(i−1)]≤exp(λ²/8)`; итерация и Chernoff дают
`P(|S_n−A_n|≥n r)≤2 exp(−2nr²)`.

Для каждого из заранее объявленных K streams выделено α_j>0, `Σα_j≤α_total`.
Положить `α_(j,n)=α_j/[n(n+1)]` и

`r_(j,n)=sqrt(log(2 n(n+1)/α_j)/(2n))`.

Поскольку `Σ_n 1/[n(n+1)]=1`, union bound одновременно по n/streams даёт
coverage не ниже 1−α_total для **среднего A_n/n**. Независимость streams не нужна.
На этом event подстановка любого pathwise-valid interval envelope из 17.3
сохраняет coverage при любом числе чтений/поздних refinements. Missingness может
зависеть от Y: она уже оплачена шириной intervals. Даже отсутствие complete
prefix telemetry не требует притворяться MCAR. Сам underlying mature prefix и
analysis order всё равно должны быть определены; arbitrary finish-order samples
этим выводом не разрешены.

Целевой A_n/n меняется с n. Поэтому **running intersection предыдущих I_hist
нельзя применять** без отдельного constant-target доказательства. Data-dependent
выбор победителя среди simultaneously covered registered streams допустим для их
объявленных targets. Новый post-hoc subgroup/workload/candidate не получает эту
защиту автоматически; retrospective вырезание «хорошего периода» меняет семью.
Рестарты/family budget — R03-11; здесь используется заранее конечный K.

Узкий radius не лечит selection bias, неверный label, разные task populations,
unknown drift или interference. При настоящем MCAR complete-case inference может
иметь более сильную модель; MAR/IPW требует дополнительных measured variables,
positivity/known propensities и подходящей concentration. Здесь такие предпосылки
не подразумеваются и никаких inverse weights из частоты наблюдений не создаётся.

### 17.5. R03-16 — граница недостоверного наблюдения и ready reuse

**Невозможность:** если accepted observer сообщает только outcome probe P, а
target T не связан с P проверяемой моделью, два мира с одинаковым потоком P могут
иметь противоположные T. Тогда history P не идентифицирует риск T. Ни подпись
ответа, ни число endpoints, ни sequential confidence не устраняют этот разрыв.
Γ=0 допустимо для честного измерения P по заявленной модели и всё ещё не означает
нулевой bias P→T. Неизвестная доля ложных P также не становится small ε после
многих одинаковых ответов. Полный недоверенный observer позволяет Γ=n.

Primary сравнение (version-specific, reference, не own theorem):

- [Waudby-Smith/Ramdas arXiv2010.09686v7,25 августа 2022](https://arxiv.org/html/2010.09686v7),
  §2/Proposition1/Theorem2/3: bounded[0,1], **constant conditional mean**, predictable
  bets. «Не iid» не означает unknown changing mean. Journal DOI
  [10.1093/jrsssb/qkad009](https://doi.org/10.1093/jrsssb/qkad009),2024 issue,
  не дата v7. Howardv9 Theorem4 допускает changing average conditional expectation;
  это другой estimand, уже рассмотренный 005, не новый прогноз.
- [Wang/Ramdas AISTATS2023, Lemma3/Theorem1](https://proceedings.mlr.press/v206/wang23p/wang23p.pdf):
  supplied TV contamination radius around one clean P и known clean moment bound.
  После Lemma3 разрешены history-adapted conditional Q_i в том же TV ball;
  произвольный per-history corruption/drift этим не покрывается. §4.1 отличает
  TV/distribution contamination от offline replacement. Готовая robust formula
  не оценивает отсутствующие ε/clean P/moment из наших counters.

**MIT confseq exact source**
`5ffe733ca2447a2e28c2c91f3b00086173f2ab2c`, setup0.0.11:
`predmix_hoeffding_cs→predmix_hoeffding_lower_cs→predmix_lower_cs`, две стороны
получают alpha/2; weighted cumulative center/λ²/8 margin, optional running
intersection. EmpBern→`lambda_predmix_eb`, lagged mean/variance arrays. Source
defaults/fake observations — регуляризация betting strategy, **не physical priors**
Snowden. Такой wrapper не реализует 17.3: input validation/censoring/Γ/goal/order/
bridge отсутствуют, а changing target и weighted/unweighted mean требуют нового
доказательства. Reference/offline comparison; не готовый Go/Kotlin adapter.
NumPy/SciPy/multiprocess + package build/pybind11 dependencies и batch arrays дают
лишнюю цену для client runtime. Numerical core/ABI/parity не проверялись.

**Author robust code** `HJ-Wg/catoni-confseq`
`381d7fb17d2af5f4cc78d422c92900f2b1d5d1c4`, untagged source pin, **GPL-3.0**.
Actual caller `demo.py→RCS_Generator(eps,moment)→observe→find_zero_decreasing`,
затем get_CS; demo не выполнялся. Source хранит growing data/λ/interval arrays,
root evaluation суммирует всю историю: не bounded online state для клиента.
Читать формулы/сравнить можно; прямое копирование требует license review.

**Узкое source расхождение, не empirical coverage finding:**
`robustconfseq.py:95` нормализует numeric side2 в string'2'; `:115` выбирает
alpha/2 по сравнению с numeric2. Для default string'2' и normalized numeric2
ветка split не выполняется. `observe:165…175` использует effective alpha в каждом
tail root. Теорема 1 использует log(2/α). Прочитанный caller этого split не добавляет.
Доступное per-tail доказательство здесь даёт union budget2α, **не** объявленный
α; это не доказательство фактического miscoverage>α и не результат запуска.
Брать repository как nominal two-sided α implementation нельзя без repair и
отдельной numerical/model проверки. Product/upstream patch не сделан/не отправлен.

### 17.6. Ручные opposing examples

Все числа — конечные логические модели, не measurements/priors проекта.

**U1 interval-valued/corruption floor.** Четыре capped losses имеют intervals
`[.2,.2],[.4,.6],[0,1],[.3,.3]`. Finite mean∈[.225,.525]. При supplied aggregate
label-error budget Γ=.2 interval расширяется до[.175,.575]; statistical radius
добавляется отдельно. Взять midpoint неизвестного loss и назвать его sample mean
нельзя. Увеличение n с тем же missing/error fraction не устраняет этот floor.

**U2 selection despite valid arm-specific inference.** Good/bad strata имеют
losses A=.1/.9, B=0/.8. A наблюдался 90/10 раз→mean.18, B10/90→.72; выбор по этим
средним предпочитает A, хотя B лучше в обоих strata. Balanced target means A.5,
B.4. Узкая CS respective sampled populations не создаёт target causal contrast.

**U3 shared-factor pseudo replication.** Один fair Z∈{0,1} и Y_i=Z для всех i.
Marginal mean=.5; после первого исхода conditional means остальных равны Z.
Наш target `A_n/n=(.5+(n−1)Z)/n`, а не.5. Считать повторные correlated samples
iid Bernoulli measurements одной fixed .5 frequency ошибочно. Новые endpoint names
не добавляют независимых network factors.

**U4 dishonest/surrogate success.** Observer каждый раз возвращает probe loss0.
Мир W0: target работает; W1: probe распознаётся/whitelisted, target заблокирован.
Histories/receipt signature одинаковы, target losses0/1. Никакой estimator только
этого потока не даёт полезный универсальный target bound; нужен иной producer или
содержательный verified transport assumption. Это модель, не найденная атака на
конкретный protected probe/сервер.

**U5 probe вмешивается в сравнение.** В условных единицах A имеет capacity10,
но concurrent measurement отбирает 9; B имеет capacity6 без этой нагрузки. Оценка
A1 versus B6 не отвечает задаче no-probe A10 versus B6. Учёт consumed bytes
полезен, но сам по себе не восстанавливает counterfactual interference effect.
ProtocolRef/episode definition либо causal model нужны отдельно.

**U6 invalid intersection.** Корректный interval для μ1=0 равен[0,.1], для
μ2=.5 — [.4,.6]. Их intersection пуст. Каждый target покрыт своим interval;
constant-parameter postprocessing не переносится на changing μ_n. Пример не
утверждает, что такой output уже генерирует наш/внешний код.

**U7 exact ready-source branch.** Разбор two allowed side inputs2 и'2' приводит
к string'2' до проверки effective alpha; обе проверки `side==2` на split false.
Это ручной branch trace указанного pin. Repair возможен, но не сделан; тестовое
выполнение/эмпирическая оценка coverage/остаточная безопасность не доказаны.

### 17.7. Cases NOT_RUN, противоположный вариант и следующий срез

| Case | Будущая проверка | Ожидаемая граница |
|---|---|---|
| M01 | n0/unknown cap/completeness/gap/duplicate unit | unknown либо validation error, не успешный empty cohort |
| M02 | Полные bounded point labels | Finite bounds сходятся в observed mean, CS radius отдельно |
| M03 | Outcome-dependent missingness | Denominator сохранён, interval width, без MCAR imputation |
| M04 | Right-censored capped latency | Valid follow-up interval, не RTT completion |
| M05 | Supplied Γ versus unknown Γ | Widen by Γ; unknown→conservative/trivial, не 0 |
| M06 | Unknown total cost support | No clipped→total conversion, cost certificate absent |
| M07 | Late refinement fixed n/out-of-order delivery | Same unit; only pathwise-valid bounds refine, не extra sample |
| M08 | Overlapping units без analysis-order model | Finite envelope only; statistical mode abstains |
| M09 | Shared latent factor U3 | Historical conditional target; не iid/marginal confidence |
| M10 | Repeated queries/winner among K registered streams | Общий заранее выделенный alpha budget, без independence |
| M11 | Post-hoc subgroup/new candidate/history reuse | Existing simultaneous coverage не наследуется |
| M12 | Unknown drift/target task transfer | Historical result можно сохранить; advisor horizon unknown |
| M13 | Probe-on versus probe-off/workload/family changes | Protocol/goal mismatch, не silently merge |
| M14 | Changing mean plus running intersection | Не применять constant-target intersection |
| M15 | External confseq weight/order/α normalization parity | Separate proposed wrapper proof/numerical check, сейчас NOT_RUN |
| M16 | Robust-code numeric/string two-sided α | Branch regression и calibrated coverage отдельны; сейчас source-only |
| M17 | Own cohort cap/retention overflow | Freeze/abstain без стирания pending/возврата custody |
| M18 | Safety signal на фоне narrow successful interval | Platform review; statistical success не скрывает safety |

**Сильная альтернатива:** отказаться от probabilistic mode до появления meaningful
producer и использовать bounded finite intervals/typed reasons. Это часто дешевле
и честнее сложного CS wrapper. Формальный алгоритм 17.3 полезен как contract;
ненулевые physical drift/bias/corruption caps он не изобретает. По настоящей
стационарной полной выборке может быть выгоднее repaired/pinned mature CS; это
отдельная numerical/adapter/model работа, не перенос всего Python research stack.

Windows адресат — owned observations/optional preference seam006, не замена
hard recovery. Android — сначала useful-task/outcome/cost producers. ADP — pure
bound kind/model/applicability review, не расширение unconnected ADP005 по умолчанию.
Source/authority, physical budget и task definition задаются отдельными контрактами.
Открытый вопрос: pre-work registration
и exploration allowance при дешёвом ложном успехе — как feedback не должен
сам себе открывать новые попытки/кредит. Это не повтор chain-rule/tail007.

## 18. R03-009 — регистрация попыток и exploration allowance против дешёвого ложного успеха

> **Есть поправка — NEEDS_REVISION.** Математические утверждения R03-17 в §18.1–18.3 и их пересказ ниже нельзя использовать как установленную теорию. См. [точные ошибки и контрпримеры](../reviews/NINE-2026-10-27-R03-17-ERRATUM.md) и [ограниченно принятую исправленную модель](../reviews/NINE-2026-10-27-R03-010-A.md).

26 октября 2026. Baseline: advisor R03-12…16. Вопрос: feedback не должен
сам себе открывать новые попытки или кредит. Runtime-проверок нет.

### 18.1 R03-17 — трёхслойный контракт: registration → bound → identification

**Слой 1 — регистрация (pre-work ledger).** До наблюдения исходов поколения
фиксируется append-only запись: `gen_id, attempt_list[], allowance W, estimand,
error target`. Попытка = (кандидат, decision-point, cost budget). Попытка,
начатая вне ledger, не становится claim этого поколения — только гипотезой для
следующего. Расширение W самим процессом по ходу данных запрещено; новое
поколение = новый W и новая error-цель, с явным sequential carryover-учётом
(R03-10/11 остаются в силе).

**Слой 2 — границы.** Per-claim anytime-valid границы (R03-15, Howard/confseq)
валидны при optional stopping каждого отдельного claim, но **не** при адаптивном
выборе того, *какие* claims репортить: репортируемый набор требует коррекции на
его размер. Worst case при отсутствии знаний о зависимости:
FWER ≤ 1-(1-α)^W. Зависимости (общие сетевые факторы, один и тот же канал)
могут только сузить union-границу при явной модели зависимости — без модели
зависимости union остаётся честной. Это прямой аналог того, что в комбинаторной
теории переобучения Воронцова в границу входит число рассмотренных гипотез, а
связность/расслоение семейства уточняют её сверху.

**Слой 3 — идентификация.** Без ledger наблюдаемы только успехи; доля ложных
среди reported неидентифицируема (ascertainment bias): одни и те же reported
успехи совместимы с любым числом скрытых попыток ≥ reported. Регистрация — это
**предположение идентификации**, а не статистическая обработка: никакой вывод по
reported-only данным не восстанавливает ненаблюдаемое W. Это расширяет
R03-16 (target/surrogate nonidentifiability) на механизм порождения выборки.

### 18.2 Контрпример: самопополняющийся бюджет

Пусть каждый «успех» добавляет +1 попытку (типичное продуктовое правило
«проверим ещё раз, раз получилось»). Per-look ложный успех с вероятностью α>0.
Число look'ов тогда случайное, растёт с числом (в т.ч. ложных) успехов —
ветвящийся процесс с положительным дрейфом: ожидаемое число ложных успехов
неограничено, family error не контролируется никаким per-claim anytime bound,
потому что число попыток — функция данных (adaptive expansion; ср. Dwork et al.
2015: адаптивный выбор запросов к данным разрушает валидность без добавочного
бюджета/шума). Фиксированный до данных W обрывает рекурсию: попыток ≤ W, граница
конечна, α-бюджет назначен заранее. Русская классика подтверждает сторону
«априорности»: SPRT Вальда держит ошибки I/II рода только при фиксированных
границах A/B, заданных до испытаний; у Ширяева оптимальные правила остановки
определены относительно априорно фиксированного класса решающих правил — смена
класса по ходу наблюдений выводит из-под гарантий.

### 18.3 Границы применимости и честный unknown

- Граница 1-(1-α)^W консервативна при сильной зависимости кандидатов; уточнение
  требует модели связности семейства (аналог effect связности Воронцова) — без
  неё не подставлять «эффективное W» ad hoc.
- Ledger не защищает от коррумпированных/задержанных исходов внутри попытки —
  это слой R03-15 (corruption envelope), здесь только регистрация факта попытки.
- Safety/admission/custody — у платформ: ledger меняет доказательную силу
  заявлений advisor'а, не права применения policy.
- Unknown: реальное распределение α (доля быстрых ложных успехов) в поле не
  измерено; подставлять неё оценку из reported-only выборки нельзя (слой 3).

### 18.4 Минимальный producer-контракт

`register(gen_id, attempts, W, estimand, α)` → append-only store →
`record(attempt_id, outcome|abstain, timestamps, cost)` →
`bound(gen_id, claim_id)` возвращает anytime-интервал c union-поправкой по
позиции claim в ledger. Producer владеет ledger'ом; consumer (adaptive policy)
читает только `bound()` и `remaining(gen_id)`. Никаких вероятностей из логов.

### 18.5 NOT_RUN

Реализация ledger в shared/adaptive; симуляция ветвящегося бюджета (подтверждение
дрейфа числом); поле: фактическое α; влияние зависимых кандидатов на ширину
union-границы. Все исполнения NOT_RUN в plan-only.
