<a id="interval-policy-evaluation-2026-10-28"></a>
# Что можно вывести из неполных наблюдений и как сравнивать полные политики

[Оглавление](../README.md) · [Политика и evidence](POLICY-AND-EVIDENCE.md) · [Решения review](../research/reviews/2026-10-28-DEEP-REVIEW.md)

**2026-10-28, DR27-08/09.** Ниже проверены математические утверждения при
явных предпосылках, а не измерены failure rates VPN. Формулы не назначаются
production scoring и не предполагают, что нужные producer guarantees уже есть.
R03-008 и R03-004/006/007 использованы как авторские основания.

## 1. Пропавшее наблюдение остаётся в знаменателе

Пусть заранее задан cohort из n>0 попыток с определённым loss
`Y_i∈[0,1]`. Для каждого полученного/неполного результата есть интервал
`[a_i,b_i]`; для совершенно неизвестного результата — `[0,1]`.
Cancellation/rejection либо имеют заранее определённый loss, либо не относятся
к этой целевой величине: нельзя менять это после просмотра исхода.

Если **одновременно для всех используемых prefixes и snapshots** известно

$$
\sum_{i=1}^n a_i-\Gamma_n\le S_n=\sum_{i=1}^nY_i
\le\sum_{i=1}^n b_i+\Gamma_n,
$$

то реализованное среднее ограничено

$$
I_{fin}(n)=\left[
\max\left(0,\frac{\sum a_i-\Gamma_n}{n}\right),
\min\left(1,\frac{\sum b_i+\Gamma_n}{n}\right)
\right].
$$

Эта оболочка не требует независимости missingness от результата.
Γ имеет единицы **суммарного loss**, а не вероятность ошибочности отчёта.
Без содержательного ограничения corruption Γ нельзя заменить нулём.

Для binary losses: u пропусков, f единиц среди полученных labels, не более k
ошибочных полученных labels дают sharp interval hull

$$
\left[\frac{\max(0,f-k)}n,\frac{\min(n,f+u+k)}n\right].
$$

Пример n=10,u=2,f=2,k=1 даёт `[0.1,0.5]`. Концы достигаются допустимыми
заполнениями missing и исправлением labels; это аналитическая конструкция.
Для interval-valued общего случая валидная оболочка не обязательно самая узкая.
Первичный контекст worst-case missing bounds:
[Jiang–Ding, v6, §2.1](https://arxiv.org/html/1610.01198v6).

## 2. Anytime historical interval имеет конкретную цель

Выберем filtration `G_i`, к которой Y_i adapted, и обозначим

$$
\mu_i=E[Y_i\mid G_{i-1}],\qquad
\theta_n=\frac1n\sum_{i=1}^n\mu_i.
$$

Для заранее объявленных streams j выделяются `α_j>0`,
`Σ_j α_j≤α_stat`. Conditional Hoeffding bound для bounded adapted Y даёт

$$
P\!\left(\left|S_n/n-\theta_n\right|\ge r\right)
\le2e^{-2nr^2},\qquad
r_{j,n}=\sqrt{\frac{\log(2n(n+1)/\alpha_j)}{2n}}.
$$

Union bound по j,n с `Σ_{n≥1}1/[n(n+1)]=1` обеспечивает общее событие
покрытия. Расширение концов `I_fin` на `±r_{j,n}` с clipping в `[0,1]`
даёт historical interval для **θ_n**, включая выбор момента остановки.
Самостоятельное доказательство использует
`E[exp(λ(Y_i−μ_i)) | G_(i−1)]≤exp(λ²/8)` и итерацию условных ожиданий.
Это консервативная union-bound конструкция, не самая эффективная известная CS.
Контекст более сильных методов:
[Howard et al., v9, §§2/4.1/6](https://arxiv.org/html/1810.08240v9).

Если исходная envelope/completeness гарантия держится лишь на **совместном**
событии вероятности≥1−β, итоговое miscoverage≤`α_stat+β`.
Независимость двух событий не нужна; pointwise99% каждого snapshot недостаточно.
Сто snapshots могут иметь ровно один ложный, случайно размещённый среди них:
каждый точечный сертификат прав с99%, но выбор ложного после просмотра
не имеет99%-гарантии. Late refinements одного фиксированного n допустимы
при совместной валидности; пересечение intervals разных θ_n — другая задача.

## 3. Какие похожие бюджеты не взаимозаменяемы

| Известное ограничение | Что разрешено вывести | Что не следует |
|---|---|---|
| Не более k испорченных labels за полный N | Для prefix n безопасно `Γ_n=min(k,n)` | Corruption каждого prefix≤`kn/N` |
| `TV(P,Q)≤ε`, bounded loss | Разность соответствующих ожиданий≤ε | Pathwise ошибка каждой суммы≤nε |
| Полны receipts зарегистрированного cohort | Результат для этого cohort | До регистрации или вне регистра не было иных attempts |
| Envelope корректна для отдельного snapshot | Его pointwise-ограничение | Совместная корректность всех поздних snapshots/selected stopping time |
| Мал sampling radius | Точность названного estimand при выполненных assumptions | Мал systematic bias или пригодность будущего прогноза |

Prefix-контрпример: N100,k10, все истинные losses0, первые десять labels
подменены1. Общая corruption10%, но на prefix10 она100%. Пропорциональный Γ1
даёт ложный `I_fin=[0.9,1]`; корректный Γ10 даёт `[0,1]`.
TV-contamination и offline label replacement имеют разные модели:
[Wang–Ramdas, v2, §§2–4](https://arxiv.org/html/2301.09573v2).
Из этой работы нельзя приписывать универсальную теорему о невозможности
неизвестного ε во всех моделях.

Если учтены n единиц с `L≤S_recorded≤U` и дополнительно неучтённых единиц
того же cohort не более H, полный **реализованный** mean лежит в
`[L/(n+H),(U+H)/(n+H)]`. При неизвестном H универсальная замкнутая оболочка —
`[0,1]`. Подпись receipts и отсутствие gaps у выданных IDs не доказывают полноту
всех фактических attempts. Это требует архитектурной точки регистрации.

## 4. Валидная historical цель может быть неверным forecast

Пусть одна случайная величина `Z~Bernoulli(1/2)` задаёт все `Y_i=Z`.
Для filtration, раскрывающей предыдущие истинные outcomes,
`μ_1=1/2`, `μ_i=Z` при i≥2. При n128:

`θ_128 = 1/256`, если Z0; `θ_128 = 255/256`, если Z1.

Historical intervals вокруг наблюдённых0 или1 могут честно покрывать эти цели
и одновременно **никогда не покрывать marginal mean1/2**. Если controller
ещё не получил delayed feedback, его available-state forecast также может
оставаться1/2. Ни формула, ни журнал не разрешают менять filtration в названии
результата. Зависимость допускается ценой иной точной цели, а не превращает
128 callbacks одного общего события в128 независимых полевых испытаний.

Переход к future target q требует отдельной гарантии `|q−θ_n|≤B`.
Тогда historical interval расширяется на±B. Если bridge гарантирован лишь
с совместной вероятностью1−η, budget становится `α_stat+β+η`.
Два мира с одинаковым прошлым и противоположным следующим outcome показывают,
почему unknown B не означает0. Mean future loss и риск хотя бы одного
срыва за горизонт тоже не одна целевая величина.

## 5. Полную policy можно сравнить без восстановления скрытого DPI-state

На изолированную экспериментальную единицу — **домен общей зависимости** —
назначается целая политика Z: baseline0 или adaptive1, с известным
`0<p_i=P(Z_i=1)<1`. Adaptive фиксирован по версии, initial-memory recipe и
правилу обучения, но продолжает адаптироваться внутри эпизода.
Заранее задаются offered workload, стартовые условия, горизонт H и полный
результат G_i. При consistency, random assignment, отсутствии interference
между единицами и наблюдаемом полном outcome:

$$
\widehat\Delta_H=\frac1N\sum_i\left(
\frac{Z_iG_i}{p_i}-\frac{(1-Z_i)G_i}{1-p_i}\right)
$$

несмещённо оценивает finite-horizon эффект **назначения** полной политики.
Admission failures, waits, retries, измерительная нагрузка и реакция сети
являются частью G_i, а не основанием выкинуть попытку из знаменателя.
Для этого ITT-эффекта не нужны внутренние action propensities и Markov-модель.
Нужны доказанные assignment/completeness/isolation assumptions, не только поля
`EpisodeID` и `InterferenceDomainID` в schema.

Расчётное ядро — N,Z,p,G. Для проверяемого смысла дополнительно фиксируются
policy/runtime identity, eligible roster до assignment, workload/deadlines,
outcome observation, полный cost до H и `outstanding custody at H`.
Detailed stage lineage полезна, но не математически обязательна при уже
достоверном полном G. [Aronow–Samii, §4](https://arxiv.org/html/1305.6156v3).

## 6. Почему marginal propensities не спасают общий rollout

Два клиента делят bottleneck. Logging design допускает `00` с вероятностью1/2,
`10` и `01` — по1/4, а `11` никогда. Outcomes в двух мирах:

| Назначение | Мир M+ | Мир M− |
|---|---|---|
| 00 | (1/2,1/2) | (1/2,1/2) |
| 10 | (1,1/2) | (1,1/2) |
| 01 | (1/2,1) | (1/2,1) |
| 11, отсутствует в logging support | (1,1) | (0,0) |

Каждый marginal propensity положителен и известен; все наблюдаемые logs имеют
одинаковое распределение. Но эффект rollout11 против00 равен+1/2 либо−1/2.
Даже знак не идентифицирован. Cluster-robust стандартная ошибка не создаёт
отсутствующий exposure. Нужна подходящая единица/совместное назначение и
positivity нужного exposure. Это ручная модель, не характеристика текущего VPS.

Sequential IPS может учитывать действие→реакцию среды, если environment kernel
условно на историю и действие одинаков при behavior/target policy, верен
assignment mechanism и есть поддержка нужных траекторий. Отношение вероятностей
включает все управляемые решения: не только channel, но waiting/probe/termination
schedule, если они меняются. Фактическая длительность работы — уже outcome.
[Jiang–Li, v3, §§3–5](https://arxiv.org/html/1511.03722v3),
[Kallus–Uehara, v3](https://arxiv.org/html/1908.08526v3).

Known propensities — сильное достаточное условие, но не универсально необходимое:
estimated propensity возможна при дополнительных модельных условиях. DR не
создаёт support и не устраняет скрытое confounding. Delayed receipt можно
соединить с исходной задачей для итогового G, но нельзя задним числом включить
его в историю, доступную прежнему решению.

Switchback требует обоснованного carryover. В
[Bojinov–Simchi-Levi–Zhao, v4, §4.3](https://arxiv.org/html/2009.00148v4)
недооценённый лаг может сместить estimator для заявленного estimand, даже
когда exact test sharp null сохраняет validity. Network epoch и фиксированная
пауза не доказывают забывание состояния middlebox или памяти learner.

## 7. Конечный handoff

Адресаты TH-02/04/06 и существующий adaptive owner. Reuse: детерминированная
baseline, scoped outcomes и известные статистические конструкции; новый
controller/ML runtime не выбран. Сбор приватных пользовательских logs не
следует из этих формул. Цена — полное учётное событие, жизненный цикл outcome,
scope гарантии и возможный abstain вместо привлекательной точечной оценки.

Future cases, все **NOT_RUN**: binary missing/corrupt bounds; corruption,
сосредоточенная в prefix; pointwise против joint snapshots; незарегистрированные
attempts; общий Z; разные historical prefixes; два мира future; rollout11 без
support; randomized isolated whole-policy эпизод с полным G; delayed feedback
и residual resources за H. Эти проверки сначала относятся к конечной модели,
а не являются полевым evidence превосходства адаптации.
