# Адаптивная политика и качество доказательств

[Оглавление теории](../README.md).

Углубление 2026-10-28: [joint guarantees, missing outcomes, historical estimand
и causal whole-policy comparison](INTERVALS-AND-POLICY-EVALUATION.md).
Условия формул не означают наличие нужных producer/bridge inputs в продукте.

Разделы перенесены из авторского сборника с сохранением исходной нумерации, дат и границ доказательства. Исторический статус не является текущим product PASS.

- [§8 — 8. Адаптация при неполных и меняющихся данных](#source-8)
- [§8.1 — 8.1. Состояние и наблюдения](#source-8-1)
- [§8.2 — 8.2. Правило решения и защитные инварианты](#source-8-2)
- [§8.3 — 8.3. Колебания, бюджет и обучение](#source-8-3)
- [§13.6 — 13.6. Управление восстановлением без самоперегрузки](#source-13-6)
- [§17 — 17. Адаптация под неопределённостью: доказательства, причинность и цена решения](#source-17)
- [§17.1 — 17.1. AD-01 — пригодность определяется набором доказательств, не числом health](#source-17-1)
- [§17.2 — 17.2. AD-02 — текущие четыре score-компонента дают меньше информации, чем кажется](#source-17-2)
- [§17.3 — 17.3. AD-03 — диагностировать нужно и сам измеритель](#source-17-3)
- [§17.4 — 17.4. AD-04 — сначала определить измеряемую цель и missing outcomes](#source-17-4)
- [§17.5 — 17.5. AD-05 — повторный просмотр результатов требует отдельной статистики](#source-17-5)
- [§17.6 — 17.6. AD-06 — безопасная адаптация: область допустимого раньше оптимизации](#source-17-6)
- [§17.7 — 17.7. Что готового действительно использовать](#source-17-7)
- [§17.8 — 17.8. Структурированный следующий этап](#source-17-8)

---

<!-- source-unit: 8 -->
<a id="source-8"></a>
## 8. Адаптация при неполных и меняющихся данных


<!-- /source-unit: 8 -->

<!-- source-unit: 8.1 -->
<a id="source-8-1"></a>
### 8.1. Состояние и наблюдения

Скрытое состояние включает перегрузку, отказ сервера, политику сети, остаточный
блок, неисправность target и клиента. Наблюдение одно и то же (timeout) может
порождаться каждым из них. Следовательно, controller решает задачу частичной
наблюдаемости; на первом этапе достаточно детерминированных правил с `unknown`.
POMDP/bandit/RL — возможные модели исследования, не необходимость внедрения ML.

Минимальное observation: mono time + wall time, network epoch, channel/config
identity, attempt/generation, target и фактический route/IP-family, stage
DNS/dial/TLS/auth/data, bytes up/down, deadline/completion/error form,
provenance и ограничения. Не писать browsing history, payload или credentials.
Структурное событие сильнее regex; отсутствие capture не заполнять «SNI-block».

Раздельные оси: **Safety**, TransportHealth, EgressHealth, EndpointHealth и
InfrastructureHealth. HTTP challenge обычно даёт сведения о site policy,
certificate error — о trust/config/time; ни одно не следует автоматически
лечить перебором портов. Пример альтернативных причин — OONI [S05].


<!-- /source-unit: 8.1 -->

<!-- source-unit: 8.2 -->
<a id="source-8-2"></a>
### 8.2. Правило решения и защитные инварианты

```text
validated candidates + свежие наблюдения + budget + protection intent
→ stay / bounded reprobe / diagnose / switch / recover / blocked
→ один Manager, generation/epoch check → apply → fresh protected gate → publish
```

Safety — жёсткое ограничение, а не штраф в utility. После него сравнивать
доступность, p95 recovery/TTFB, goodput и цену. Профиль с меньшим RTT, но без
удостоверенного protected пути не выигрывает score.

- Подтверждённая утечка или недостоверный барьер → safe error немедленно.
- Один liveness timeout → ограниченное подтверждение, не новый «диагноз DPI».
- Только endpoint outage → другой независимый измеритель; не гонять все транспорты.
- Общий origin недоступен → приоритет другого origin, а не всех его протоколов.
- Все пути непроверены/недоступны → BLOCKED и bounded retry без снятия защиты.
- Пользовательский Disconnect отличается от ошибки, crash и закрытия GUI.

Смена транспорта обычно не мигрирует существующий пользовательский TCP-поток.
«Seamless» разрешено писать только для фактически сохранённых потоков;
reconnect новых потоков и verified rollback — отдельные результаты.


<!-- /source-unit: 8.2 -->

<!-- source-unit: 8.3 -->
<a id="source-8-3"></a>
### 8.3. Колебания, бюджет и обучение

Cooldown, hysteresis, min dwell и backoff с jitter ограничивают колебания,
но их величины должны измеряться. Reset на новом epoch не должен стирать
protection intent. Старые observations не могут commit после смены сети/сессии.
Репутацию канала ослаблять с возрастом, различать срок действия профиля и TTL
наблюдения; не сбрасывать безопасность из-за устаревшей статистики.

Один общий budget: максимальные concurrent probes, bytes/окно, attempts,
CPU/time, server rate, батарея/стоимость сети; maintenance probes проходят тот
же limiter. Отмена UI не означает, что запущенная работа перестала потреблять
ресурсы. Неподчиняющийся engine Start/Close требует process boundary, не ещё
одного goroutine timeout. Стенд должен проверять boundedness и накопление работы.

Исследование нового кандидата — только из валидного доверенного набора, без
protected user payload до gate. Не сканировать порты/домены массово и не
переобучаться на собственных probes. Сначала replay фиксированного fault schedule,
затем shadow recommendations, canary и rollback. ML допускается при выигрыше
на отложенных сетях/днях и стоимости ошибок, с fallback к объяснимой политике.


<!-- /source-unit: 8.3 -->

<!-- source-unit: 13.6 -->
<a id="source-13-6"></a>
### 13.6. Управление восстановлением без самоперегрузки

Три уровня по3 попытки дают до27 нижних работ вместо3 у единственного retry owner
(M02). У QUIC уже есть PTO/loss recovery; внешний controller, HTTP client и UI
не должны независимо повторять одно действие. Передавать remaining end-to-end
deadline, retry classification и spent budget; auth/revocation/permission/permanent
schema errors не лечатся бесконечным reconnect. Backoff с jitter разносит fleet storm,
но не заменяет caps active operations/bytes/attempts/cleanup. [D03][D13][D14]

Fixed-window «3 в минуту» может пропустить6 на границе окна; token bucket допускает
burst+refill и не тождествен strict rolling bound (M03). В проекте rolling budgets
уже есть — сохранить их смысл. Budget rejection означает local admission result,
не failure удалённого канала. Не возвращать budget после каждого early error:
неудавшийся dial тоже расходует CPU/радио/криптографию.

Сохранять отдельные lanes safety/recovery и optional diagnostics/update/UI history;
добавить предел ожидающих, размер request/result и возможность coalescing без
потери Stop/revoke. «Timeout наблюдателя» не освобождает зависший native owner.
При resource pressure сначала убрать optional/bulk work, а не превратить отсутствие
измерения в quarantine и ещё10 параллельных проверок. Envoy adaptive concurrency —
готовый алгоритмический ориентир для latency/queue feedback; его HTTP minRTT windows
не копируются как доказательство DPI или разрешение direct. [D15]

Сравнивать **выигрыш после цены перехода**. В M08 улучшение35→30ms на горизонте50
действий не окупает500ms переключения;70→30ms окупает при тех же условных входах.
Реальный horizon неизвестен, а старые TCP/UDP sessions могут оборваться. Нужны
hysteresis/minimum dwell/uncertainty и ручной quiet mode; security revoke сильнее
удобства. Recovery от реального отказа не должен ждать декоративного cooldown.


<!-- /source-unit: 13.6 -->

<!-- source-unit: 17 -->
<a id="source-17"></a>
<a id="adaptive-evidence-depth"></a>
## 17. Адаптация под неопределённостью: доказательства, причинность и цена решения

2026-10-25. Это углубление алгоритма TH-02/04/06, RQ-11…15/18/20 и AND-07/11/16,
а не ещё один список транспортов. Прочитаны current PC V2-146/Android AND-12-S2
snapshots, scoped controller/scoring/health/probe/admission/failover source и
первичные алгоритмы. AD-01…06 — traceability labels уточнений существующих работ.
Новая реализация controller/SDK, полевые результаты и оптимальность не заявлены.
Evidence/снимки — `docs/evidence/2026-10-25-adaptive-evidence.md`.


<!-- /source-unit: 17 -->

<!-- source-unit: 17.1 -->
<a id="source-17-1"></a>
### 17.1. AD-01 — пригодность определяется набором доказательств, не числом health

Для каждого результата нужен контекст:

`(owner, intent, profile+policy identity, physical path/epoch, required scope,
measurement endpoint/dependency, workload+family, method, start/finish/expiry)`.

Это **целевой evidence contract**, не описание уже существующего полного DTO.
Его следует составить из имеющихся IDs/receipts, не вводить второго session owner.
Значения `unknown/not-supported/not-measured/stale` отличаются от отрицательного
результата. Успех HTTPS/IPv4 не отвечает за IPv6/UDP/голосовую сессию; более новый
узкий тест не сильнее старого широкого только из-за timestamp.

Формально evidence сравнивается по частичному порядку: `e1` может заменить `e2`
только при совместимом контексте, достаточном scope и пригодном сроке. Для required
capabilities `R` нужна совместимая совокупность receipts, а не `max(healthScore)`.
Если используются интервалы пригодности `I_k`, непустое пересечение `∩I_k` и
одинаковые identity/epoch — необходимые условия композиции. Они не доказывают
отсутствие невидимого изменения физической сети; эта неопределённость остаётся.
Нельзя соединять новый DNS receipt с прежними egress/owner фактами другого cutover.

Разделить **допуск к безопасной проверке** и **допуск пользовательского трафика**.
Candidate trial может пройти обычный Start/check только под закрытой защитой;
user-ready появляется после свежего полного gate. Статистическая уверенность,
legacy `live-verified` label или прохождение cooldown этот gate не заменяют.
Freshness detector не продлевает signed expiry/authority/OS permission.

**Gate TH-02/04/RQ-13…15/18, AND-05/07/09:** несовместимые receipts/частичные
families/late publication, физическое изменение между tests, old label+new intent,
timer-only refresh. Для каждого user-facing READY указать actual capability scope
и источник самого слабого обязательного факта; no evidence — не synthetic success.


<!-- /source-unit: 17.1 -->

<!-- source-unit: 17.2 -->
<a id="source-17-2"></a>
### 17.2. AD-02 — текущие четыре score-компонента дают меньше информации, чем кажется

По exact `scoring.go` при текущих трёх verdict и обычных входах:

| Verdict | H | F | E | Точный score до сортировки |
|---|---:|---:|---:|---|
| healthy |1|1|1|`0.65 + 0.35·I`|
| no_data |0.5|0|0|`0.15 + 0.35·I`|
| failing |0|0|1|`0.15 + 0.35·I`|

`healthBucket` сортируется **раньше** score. Поэтому внутри каждого current bucket
H/F/E постоянны и C2-порядок определяется I, затем D2/stable tie-break. Это
алгебраическое следствие source, не результат benchmark. Три свидетельства
health/freshness/evidence не независимы: они производны того же verdict.
Настройка их весов сама по себе не добавит информации к текущему C2-ranking.

Ограничение вывода существенно: итоговый выбор не равен одному C2. В failover
сначала фильтруются enabled/legacy live-verified/represented candidates, затем
применяются C2, `orderCandidatesForFailure` и recent-attempt ordering; fresh
switch/protection gates идут отдельно. Нельзя объявить весь controller функцией I
или найденный score-collapse автоматически runtime дефектом.

**Решение: сохранить deterministic baseline**, не подбирать новые «оптимальные»
веса на тех же фактах. Если нужен прирост, добавить действительно новую
source-backed информацию: current workload completion, cost/failure-domain
provenance, measured switch disruption, interval uncertainty. Переход от scalar
independence к dependency graph не делает неизвестные связи независимыми.

**Gate TH-02/04/06/RQ-14/18/20, AND-07/16:** documented candidate-order trace по
всем стадиям, ablation против текущего порядка, разные failure domains/unknown,
неравные sample counts и старые факты. Не заявлять learned probability для score
или улучшение от изменения весов без отдельной matched оценки.


<!-- /source-unit: 17.2 -->

<!-- source-unit: 17.3 -->
<a id="source-17-3"></a>
### 17.3. AD-03 — диагностировать нужно и сам измеритель

Lifeguard показывает проблему detector local slowness: здоровый peer может
ошибочно считаться отказавшим. Exact memberlist `awareness.go` реализует
saturating local score и `timeout·(score+1)`; `suspicion.go` дедуплицирует
подтверждения по sender identity и уменьшает soft suspicion timeout с накоплением
подтверждений. Это кластерный протокол, не готовый VPN detector.

**Adapt principle, не cluster engine:** разделить local admission rejection,
local queue/worker starvation, transport setup, resolver/measurement endpoint,
destination service и protection/authority outcome. Таймаут не устанавливает
место проблемы. Новые индикаторы local pressure должны опираться на реальное
admission/wait/actual-return наблюдение, а не новый timer/daemon ради fake CPU score.
Существующие reserved DiagnosticProtected/optional lanes — основа.

При local pressure сокращать optional work и признавать measurement uncertainty.
Можно bounded отложить **soft** suspicion/optional retry; нельзя увеличить trust
expiry, срок пользовательского grant или продолжать READY на протухшем proof.
Если обязательный gate не удаётся обновить, итог определяется protection policy,
а не убеждением «вероятно, завис только детектор».

Envoy различает local-origin и external/transaction errors, а outlier statistics
требуют минимальных host/request volumes. Но его timeout/reset local-origin
означает источник **на стороне proxy**, не доказанную локальную CPU неисправность.
Memberlist unique senders также не означают независимость failure domains.
Нельзя механически превратить3 CDN URLs или3 протокола одного VPS в3 свидетеля.
Envoy panic может выбрать all hosts либо no hosts по policy; поведение,
разрешающее не прошедший safety кандидат, в наш protected controller не переносится.

**Gate TH-02/04/06/RQ-11/12/14/19, AND-02/07/11:** local stall+healthy network,
общий measurement outage, shared resolver/CDN, hard trust revoke при overload,
недостаточный sample volume. Проверять false quarantine вместе с missed faults
и recovery tails: уменьшение ложных тревог ценой бессрочного молчания не победа.
[Lifeguard paper](https://arxiv.org/html/1707.00788v2),
[memberlist pin](https://github.com/hashicorp/memberlist/tree/5cd2c4b0fb09caea5de03aae1adb9c36374ef3e0),
[Envoy outlier](https://www.envoyproxy.io/docs/envoy/v1.38.2/intro/arch_overview/upstream/outlier),
[panic policy](https://www.envoyproxy.io/docs/envoy/v1.38.2/intro/arch_overview/upstream/load_balancing/panic_threshold).


<!-- /source-unit: 17.3 -->

<!-- source-unit: 17.4 -->
<a id="source-17-4"></a>
### 17.4. AD-04 — сначала определить измеряемую цель и missing outcomes

Для работы пользователю важнее `P(useful task complete within D | context)`, чем
RTT среди удачных probes. Context включает family/workload/destination/network,
cold/warm состояние и offered load. Deadline D задаётся до сравнения и не меняется
для выигравшего кандидата. Five fast successes out of100 и95 умеренно быстрых
успехов out of100 демонстрируют разницу success-conditioned latency и task success;
это иллюстрация метрик, не измерение продукта.

Хранить разные исходы: planned/admitted/started/completed/failed/timed-out/
user-cancelled/superseded/budget-rejected/not-attempted. Не кодировать последние
четыре как remote-channel failure. Для операционной оценки учитывать end-to-end
время ожидания/восстановления; для причинного анализа отдельно показывать
network stage и local wait. Намеренное исключение aborted samples может создать
survivorship bias. Статистику с timeout нельзя молча трактовать как полный RTT.

История детерминированного winner selection не содержит outcomes невыбранных
каналов; offline оценка иной политики без достаточного coverage неидентифицируема.
Если когда-нибудь понадобятся bandits/off-policy методы, сначала требуется
контракт допустимого exploration и selection probabilities. Из текущего журнала
их не восстановить выдуманным propensity=1. На этапе baseline ML не нужен.

Для будущих paired trials фиксировать workload/deadline/cold-warm/epoch, порядок
A/B и block/ISP/day; не считать100 packets одного outage100 независимыми опытами.
Active probes сами нагружают путь. Envoy отдельно предупреждает, что health-check
traffic искажает minRTT sampling; нельзя оценивать пользовательский cost только
по health probes. Bounded randomized scheduling помогает против синхронизации,
но jitter не гарантирует независимость, репрезентативность или скрытность.

Нельзя ради online сравнения открыть двух lifecycle owners. Если candidate нельзя
проверить изолированно без смены active path, эксперимент проводится в разрешённом
controlled window/лаборатории; quiet mode и safety contract сохраняются.
**Gate TH-02/06/RQ-11/16/18/20, AND-07/11/16:** unequal cancellation/timeout,
непроверенные arms, cold-vs-warm bias, overload от probes, delayed/short/bulk
workload и разные deadline. До paired evidence — descriptive observation,
не причинный вывод «кандидат лучше».
[IPPM methods RFC7799](https://www.rfc-editor.org/rfc/rfc7799.html),
[reactive paths/sampling RFC7312](https://www.rfc-editor.org/rfc/rfc7312.html),
[Envoy concurrency guidance](https://www.envoyproxy.io/docs/envoy/v1.38.2/configuration/http/http_filters/adaptive_concurrency_filter).


<!-- /source-unit: 17.4 -->

<!-- source-unit: 17.5 -->
<a id="source-17-5"></a>
### 17.5. AD-05 — повторный просмотр результатов требует отдельной статистики

Fixed-sample95% interval не становится общей95% гарантией, если после каждого
запроса смотреть на него и останавливаться на выгодном результате. Time-uniform
confidence sequences позволяют контролировать ошибку при произвольном stopping,
но тоже требуют названных model/martingale assumptions; произвольная сеть с
меняющейся политикой цензора не становится iid от названия алгоритма.
Первичный результат: [Howard et al., v5](https://arxiv.org/html/1810.08240v5).

**Простой собственный вывод для будущей методики, не выполненный experiment:**
пусть для заранее названных вариантов/strata `i` есть iid bounded outcomes
`X_i,k∈[0,1]` с фиксированным mean `μ_i`; нет selection/censoring bias. Выбрать
`w_i>0, Σw_i≤1` и общий error budget `α∈(0,1)`. Для **каждого sample count n≥1**:

```
δ_i,n = α·w_i / (n·(n+1))
b_i,n = sqrt(log(2/δ_i,n) / (2n))
L_i,n = max(0, mean_i,n − b_i,n)
U_i,n = min(1, mean_i,n + b_i,n)
```

Fixed-n Hoeffding bound ограничивает ошибку на каждом n величиной δ_i,n.
Так как `Σ_n 1/(n(n+1))=1`, union bound даёт:

`P(существуют i,n: μ_i вне [L_i,n,U_i,n]) ≤ Σ_i,n δ_i,n ≤ α`.

Независимость **между событиями просмотра** для union bound не нужна. Но iid/
fixed-mean и корректно собранные observations нужны для использованного
fixed-n bound. Трюк не исправляет biased sampling, коррелированные bursts,
изменение μ, необъявленные новые strata или reset n/α после неудобного результата.
Для более слабых assumptions нужен отдельно выбранный валидный martingale bound,
а не тот же интервал с надписью robust. Это консервативная простая baseline-методика;
modern mixture/empirical-Bernstein boundaries могут быть уже при своих условиях.

**Решение: offline reference/candidate**, не новая production dependency и не
gate безопасности. Author package confseq имеет MIT license; exact README/
LICENSE закреплены, вычислительное ядро и текущие Python/C++ ABI не проверялись.
Перед численным использованием нужны отдельные formula/golden/tail/underflow
и assumption checks. Нельзя заявлять95% uptime/«утечек нет» по этому apparatus.
**Gate TH-02/06/RQ-16/20/FR-012, AND-16:** repeated looks, multiple variants,
partial sampling, drift/change of strata и остановка/перезапуск анализа; явно
указать population/estimand/error budget. Safety proof не p-value.


<!-- /source-unit: 17.5 -->

<!-- source-unit: 17.6 -->
<a id="source-17-6"></a>
### 17.6. AD-06 — безопасная адаптация: область допустимого раньше оптимизации

Ввести в целевой contract два различных решения:

1. **Hard safety/lifecycle:** revocation, permission/owner uncertainty, invalid
   authority, protection failure → существующие retire/block/recovery gates.
   Никакой score/confidence/quiet/local-health timeout не покупает обход.
2. **Optional optimization:** только среди допустимых trials, при пригодных
   observations, доступном budget и допустимой цене disruption. Стабильный
   работающий путь не обязан меняться ради маленького неопределённого gain.

Если cost `J` измерен в заранее заданных сопоставимых единицах на задачу,
консервативный proposed switch criterion имеет вид:

`UCB(J_new) + C_switch/H + margin < LCB(J_current)`.

`C_switch` — cost переключения в тех же суммарных единицах; `H` — горизонт задач.
Unknown H/disruption не заполняются удобным числом; сравнение может остаться
inconclusive. Criterion не применяется к срочному safety recovery и не обещает
сохранить существующий TCP/UDP поток. Для повседневных сценариев сначала нужны
измеримые task SLO, не новая универсальная формула utility.

Целевой порядок, собирающий существующие components:

```
event → validate current owner/intent/authority/protection facts
      → hard failure? retain closed barrier; ordinary bounded retirement/recovery
      → classify observation scope + measurement/local-health uncertainty
      → current required proof valid? keep current unless justified optional change
      → eligible trial set → deterministic baseline order + bounded policy budget
      → one owned transition → complete fresh gates → atomic scoped publication
```

Это псевдокод contract, не реализованный ProtectionController/дублирующий actor.
Hysteresis/dwell ограничивают optional churn, но не блокируют Stop/revoke. Все
гейты повторяются после ожиданий/retirement; проигравший/отменённый trial остаётся
ресурсом до actual return. Измерительные consumers не продлевают право публикации.

**Теоретический предел:** можно построить два будущих мира с одинаковыми
наблюдениями до t, где после t ломается соответственно A или B. Causal policy
с одинаковой историей не может заранее выбрать oracle-победителя в обоих.
Следовательно, «всегда самый быстрый/безотказный» не гарантируется алгоритмом
при произвольном неизвестном будущем. Сравнимые требования — safety invariants,
полезное completion/recovery/churn/resource outcomes в объявленном threat/workload
scope и цена ошибок. Это собственный indistinguishable-history аргумент,
не доказательство оптимальности нашей реализации.

**Gate TH-01/02/04/06/08/RQ-11…20, AND-02/07/10/11/16:** alternating path quality,
expensive transitions, quiet+revoke, no eligible candidate, late/blocked returns,
current valid path+inconclusive benefit и genuine outage. Сравнивать baseline и
новую policy по одним scenarios, включая failures/missed faults/false switches;
никакого улучшения скорости до evidence.


<!-- /source-unit: 17.6 -->

<!-- source-unit: 17.7 -->
<a id="source-17-7"></a>
### 17.7. Что готового действительно использовать

| Источник / pin / license | Выбор и ограничение |
|---|---|
| memberlist `5cd2c4b0fb09caea5de03aae1adb9c36374ef3e0`, awareness/suspicion/LICENSE/go.mod, MPL-2.0/Go1.25 | **Adapt algorithmic principle**, без gossip cluster/metrics dependency в VPN. Local self-health и soft suspicion; identity dedup не statistical independence, hard safety сроки не растягивать |
| Envoy v1.38.2 `f387231af8dd7274e37c5ae2cc797cb09a948818`, outlier proto/implementation/LICENSE, Apache-2.0 | **Reference/adapt contracts** origin classification, sample adequacy, detected/enforced distinction. HTTP/cluster defaults и panic policy не VPN presets; C++ sidecar не добавлять |
| confseq `5ffe733ca2447a2e28c2c91f3b00086173f2ab2c`, README/LICENSE, MIT | **Offline candidate/reference** для будущих numerical checks, не embedded runtime. Сейчас проверены только назначение/license docs; statistical ядро/ABI не audited и package не установлен |
| Existing scoring/health/DiagnosticProtected/owner receipts | **Reuse deterministic baseline and lanes**; сначала trace/measurement contract и отрицательные cases, затем один bounded improvement. Новые readiness fields проходят обычный DTO/schema/old-client gate |

9 upstream files/Git blobs и15 локальных snapshots сохранены;3 source licenses
не доказывают лицензию всего будущего binary graph. Все новые deps/копирование кода
требуют actual composition/license/security gate. Ни одна dependency не добавлена.


<!-- /source-unit: 17.7 -->

<!-- source-unit: 17.8 -->
<a id="source-17-8"></a>
### 17.8. Структурированный следующий этап

| Порядок | Выход существующих владельцев | Будущая приёмка |
|---|---|---|
| 1 / P0 TH-01/03/04, AND-04/06 | Current safety/trust/accepted producer и source blockers | Existing OS/SDK/native gates; algorithm research их не снимает |
| 2 / TH-02/04, AND-07 | Scope/receipt join и typed outcome/origin/local-wait contract AD-01/03/04 | Local stall/endpoint outage/stale joins/partial families/no false cause |
| 3 / TH-04/06, AND-07/11 | Exact baseline trace AD-02, separate hard vs soft decisions AD-06 | Churn/recovery/unknown/actual-return budgets; preserve working path when benefit uncertain |
| 4 / TH-06/FR-012, AND-16 | Predeclared estimand/paired blocks/analysis-error budget AD-04/05 | Counts/cancellations/all variants, repeated looks, drift/holds; no iid assumption hidden |
| 5 / разрешённый comparative gate | Один выбранный improvement против baseline | Useful completions/recovery tails/disruption/resources/energy и full failures, затем rollout decision |

Current plan-only сохраняется: в этом срезе только первичка/source/manual algebra
и документы. Product/harness tests/build, statistical simulation, native/device/
VPS/live и системная сеть не выполнялись. Детальные неисполненные cases — evidence.


<!-- /source-unit: 17.8 -->
