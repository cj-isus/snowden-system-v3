# R03-010 — последовательный бюджет ошибок и доказательная граница журнала попыток

**27 октября 2026 · READY_FOR_REVIEW.** Срез источников и ручное
доказательство относятся к этой дате.

Пакет исправляет математическую основу R03 REPORT §18, не переписывая его.
[Авторская поправка](../reviews/NINE-2026-10-27-R03-17-ERRATUM.md) — обязательный
baseline, а не независимо принятая аксиома. §§14/17 остаются авторскими
предложениями с собственными assumptions. Никакого исполнения моделей,
статистики, crypto, тестов, сборки, сетевых проб VPN или пользовательских данных.
Публичное HTTPS-чтение научных источников и статическая обработка PDF не являются
статистическим экспериментом. Все результаты ниже — ручная математика либо
source-факты, не полевые оценки и не продуктовый PASS.

## 1. Что именно проверяется

Три ограниченных вывода:

- **R03-010-A:** достаточные условия счётного предсказуемого распределения
  бюджета FWER при адаптивном выборе, случайном числе проверок и задержках;
- **R03-010-B:** точная модель кредитов «+b за успех» и различие ожидания,
  жёсткого физического лимита и статистической гарантии;
- **R03-010-C:** минимальный producer-контракт и две границы reuse:
  транзакционный bbolt не создаёт complete ledger автоматически; confseq
  предоставляет конкретный conditional-mean CS, не гарантию произвольного advisor.

Имена локальны пакету, глобальные диапазоны R03-18… не занимаются.

## 2. Словарь: четыре уровня, которые нельзя смешивать

Пусть при одном фиксированном распределении данных P, соответствующем допустимому
сценарию, R_t — число отвергнутых нулей до момента t, V_t — число ошибочно
отвергнутых истинных нулей. При необходимости гарантия должна выполняться для
каждого P из заранее определённого модельного класса, а не только одного P.

| Величина | Определение / гарантия | Не означает |
|---|---|---|
| FWER(t) | P(V_t ≥ 1) | Частоту фактических отказов VPN |
| Lifetime FWER | P(существует хотя бы одна ложная декларация во всей объявленной семье) | Покрытие произвольных новых постфактум вопросов |
| FDR(t) | E[V_t / max(R_t,1)] | P(V_t≥1), realized FDP или долю ошибок среди опубликованного поднабора |
| mFDR | Отношение ожиданий E[V_t]/E[R_t], либо версия с объявленным стабилизатором | E[V_t/max(R_t,1)]; конвенции разных статей нельзя незаметно смешивать |
| Per-look level | P(ошибка в одном заранее определённом анализе) ≤ α | P(ошибка когда-либо при многократном просмотре) ≤ α |
| Per-claim anytime level | P(хотя бы одна ошибка за все разрешённые моменты анализа одного claim) ≤ α_j | Бюджет для других claims/restarts/models |
| Confidence sequence (CS) | P(для всех n: θ_n∈C_n) ≥ 1−α_j | Текущую/future вероятность успешного пути, если θ_n — историческая средняя |

Определения проверены по Javanmard–Montanari [P3, §1.1, PDF pp.4–5] и Ramdas
et al. [P2, §1–2, PDF pp.1–3]. CS и random-time свойство — Howard et al.
[P1, (1), §1.1, Lemma 3, PDF pp.1,3,19]. Полные библиографические и read-status
паспорта — [SOURCES-010](SOURCES-010.md).

**FWER ⇒ FDR** для той же семьи и того же времени, поскольку
V_t/max(R_t,1) ≤ 1{V_t≥1}. Обратное неверно: если отвергнуты 100 ложных нулей
правильно и один истинный нуль ошибочно с вероятностью один, FDR=1/101,
но FWER=1. Это логический сценарий, не измерение.

**Публикация поднабора:** при simultaneous coverage всех уже определённых
claims произвольный выбор их поднабора не увеличивает событие «хотя бы одна
ошибка». FWER/coverage сохраняются. Однако FDR полного набора не даёт FDR
выборочно опубликованного: в предыдущем примере оставить только ложную
декларацию — опубликованный FDP равен один. Новая цель, новый subgroup или
иная подгонка модели не являются поднабором прежних покрытых утверждений.
Поэтому blanket-тезис §18 «выбор, какие claims репортить, всегда требует новой
поправки» неверен без уточнения вида гарантии и семьи.

## 3. R03-010-A: композиция по фиксированному и счётному семейству

### 3.1 Произвольная зависимость: корректная конечная граница

Для фиксированного W обозначим E_j событие ошибки j-го истинного нуля,
P(E_j)≤α_j. Тогда pathwise 1_{∪E_j}≤Σ1_{E_j}, и

\[
 P\!\left(\bigcup_{j=1}^{W}E_j\right)
 \le \min\!\left(1,\sum_{j=1}^{W}P(E_j)\right)
 \le \min\!\left(1,\sum_{j=1}^{W}\alpha_j\right).
\]

Независимость не нужна. При равных уровнях получаем min(1,Wα).
Только при независимых **событиях ошибок**:

\[
 P(\cup E_j)=1-\prod_j(1-P(E_j))\le 1-(1-\alpha)^W.
\]

Два несовместимых события на U~Uniform[0,1], E_1={U≤.05},
E_2={.05<U≤.10}, имеют FWER=.10 > .0975. Совпадающие E_j дают .05.
Это независимо воспроизведённый контрпример к §18.1/18.3, а не опровергающий
пример к исправленной сумме. Сравнение с аналогиями Воронцова для этого
элементарного доказательства не требуется и не используется.

### 3.2 Точная теорема для адаптивных запусков

Нумеровать **admission**, а не время возврата результата: j=1,2,… — счётный
набор возможных запусков. Если запуск не произошёл, его ошибка пуста и
расход равен нулю. В асинхронном процессе \(\mathcal H_j^-\) — информация,
действительно доступная **до admission j**, включая прошлые результаты,
partial callbacks, решения scheduler и зарегистрированную конфигурацию.
Она не обязана содержать финальные исходы всех предыдущих запусков.

Достаточные предпосылки, для каждого допустимого P:

1. A_j∈{0,1} — решение запустить проверку; α_j∈[0,1], цель H_j, модель, масштаб,
   стороны и процедура выбраны по \(\mathcal H_j^-\). При A_j=0 берём α_j=0;
   уровень α_j=0 означает отсутствие разрешённой декларации, а не порог
   численного сравнения, полученный underflow.
2. E_j — событие **хотя бы одной ложной декларации** данной проверки в любой
   разрешённый момент, включая повторный просмотр и её own stopping. Условие:

\[
 \mathbb E_P[\mathbf1_{E_j}\mid\mathcal H_j^-]\le A_j\alpha_j.
 \tag{A1}
\]

   Формулировка сразу про ошибку устраняет скрытую предпосылку о измеримости
   случайно выбранного истинного нуля. Для обычного fixed-parameter model
   можно проверить условную super-uniformity выбранного true-null test;
   если truth/target зависят от будущего, отдельно определить event и модель.
3. Расход **всей семьи**, не только reported/completed/null-known subset:

\[
 \sum_{j\ge1}A_j\alpha_j\le\delta\quad\text{почти наверное}.
 \tag{A2}
\]

4. Утверждение относится к объявленным процедурам и семье. Новое название
   epoch не создаёт бесплатную независимую процедуру, model validity или δ.

**Доказательство.** Для каждого конечного m, по неравенству индикаторов,
башне условных ожиданий и (A1):

\[
 \begin{aligned}
 P(\cup_{j\le m}E_j)
 &\le\sum_{j\le m}\mathbb E[\mathbf1_{E_j}]\\
 &=\sum_{j\le m}\mathbb E[\mathbb E(\mathbf1_{E_j}\mid\mathcal H_j^-)]\\
 &\le\mathbb E[\sum_{j\le m}A_j\alpha_j]\le\delta.
 \end{aligned}
\]

По непрерывности вероятности снизу (эквивалентно монотонной сходимости для
неотрицательных сумм) получаем P(∃j:E_j)≤δ. Конечное или бесконечное
ожидание числа попыток в доказательстве не участвует. Отсутствие независимости
между потоками допустимо **при выполненном A1**, а не вместо A1.

Например α_j=δ/[j(j+1)] даёт Σα_j=δ, поскольку 1/[j(j+1)]=1/j−1/(j+1).
Можно выбирать α_j адаптивно из остатка при сохранении A1/A2. Заранее
фиксированный W — один из достаточных способов учёта, **не необходимость**.
Эта теорема — элементарное доказательство пакета; не новая научная теорема и
не утверждение, что поля current API автоматически удовлетворяют A1.

### 3.3 Что именно должна дать per-claim статистика

Достаточный механизм A1 — неотрицательный test-supermartingale M_{j,n} с
M_{j,0}=1, условно на фактическую admission history, относительно допустимой
фильтрации внутри j. Для истинного нуля:

\[
 E[M_{j,n}\mid\mathcal G_{j,n-1}]\le M_{j,n-1},\quad
 \mathcal H_j^-\subseteq\mathcal G_{j,0}.
\]

Отклонять при M_{j,n}≥1/α_j для α_j>0. Условная версия неравенства Вилля
даёт A1. Её короткое обоснование: τ — первое пересечение порога, для каждого
конечного n неотрицательность и optional sampling для bounded τ∧n дают

\[
 \alpha_j^{-1}P(\tau\le n\mid\mathcal H_j^-)
 \le E[M_{j,\tau\wedge n}\mid\mathcal H_j^-]\le1.
\]

Здесь случайный порог 1/α_j измерим при admission; α_j=0 — деклараций нет.
Переход n→∞ по возрастающим событиям даёт A1 без предположения E[τ]<∞.
Предсказуемые ставки λ_{j,n}, выбранные до X_{j,n}, могут зависеть от прошлого,
**если** условное MGF/mean предположение верно относительно всей реально
используемой фильтрации. Простая marginal validity для фиксированного j
не гарантирует conditional validity после data-dependent выбора j.

Источники: Waudby-Smith–Ramdas [P4, §2, (3)–(7), Theorem 1]; Howard et al.
[P1, §2, §6, A.8] показывают supermartingale/CS механизм и его предпосылки.
Доказательство счётной композиции выше не требует конкретной библиотеки.

### 3.4 Контрпример: post-selection + ровно δ расхода всё ещё недостаточно

Заранее доступны два истинных нуля с независимыми p_1,p_2~Uniform[0,1].
Посмотреть оба, выбрать меньший, объявить только одну «новую» проверку с α=δ.
Учёт показывает единственную строку и расход δ, но вероятность отказа:

\[
 P(\min(p_1,p_2)\le\delta)=1-(1-\delta)^2>\delta.
\]

Выбранный p не conditionally super-uniform после выбора; A1 нарушено.
Хэшированная запись, сделанная после просмотра, это не лечит. Альтернативы:
заранее покрыть оба claims общей simultaneous процедурой; отделить discovery
от новых confirmatory observations с обоснованной условной моделью;
либо применить отдельно доказанный selective-inference метод.
«Новые» измерения на том же сетевом состоянии не создают независимость сами.

### 3.5 Задержки: registration order, reveal order и возвращённый кредит

- Admission α_j возможен при незавершённых ранних проверках; их статистические
  расходы остаются зарезервированными. Завершение, timeout или отсутствие
  rejection не дают автоматического **нового** расходного бюджета.
- Возможный возврат неиспользованного reservation должен иметь отдельное
  доказательство: конкретная проверка не использовала данные, не делала
  деклараций и pathwise не сможет сделать их позже, либо новая композиционная
  схема специально доказывает recycled budget. «Cancel отправлен» не такой receipt.
- Допустимый conservative вариант внутри claim: frozen predictors в исходном
  unit order, анализ только полностью раскрытого непрерывного префикса K(t).
  На simultaneous событии покрытия всех n можно читать C_{K(t)} даже при
  outcome-dependent времени чтения. Но это **сохранение уже валидной CS**,
  не исправление finish-order выборки и не unbiasedness среднего при K(t).
- Если Y=1 раскрывается быстро, Y=0 медленно, completed-only поток biased.
  Пустое место roster не заменяется нулём; большие IDs не дают extra samples.
  Последний известный outcome не должен ретроспективно менять predictor,
  объявленный предсказуемым для раннего sample.
- Если методы используют промежуточные данные других pending проверок,
  модель A1 должна учитывать эту историю. Наивное перенесение synchronous
  online-FDR theorem на asynchronous callbacks не установлено этим пакетом.

Следствия проверены ручным event-аргументом и сопоставлены с R03 §14.2,
Howard [P1, Lemma 3, §6]. Никаких generated/delayed traces не исполнялось.

### 3.6 Alpha-investing/FDR — альтернативная цель, не бесплатный FWER

В прочитанном SAFFRON [P2, Theorem 1, §2.2–3.2] mFDR-результат дан при
conditional super-uniformity относительно rejection/candidacy history;
дополнительный FDR-результат требует независимости null p-values от остальных
и монотонности выбора thresholds. Нельзя убрать последние условия, потому
что «общие сетевые факторы учтены в DependencyRef».

Original alpha-investing не даёт произвольный FDR/FWER по факту пополнения
wealth. В Javanmard–Montanari [P3, §3] показано различие mFDR/FDR.
FDR(t)≤δ для каждого фиксированного t отдельно также не доказывает FDR(τ)≤δ
при произвольном optional stopping τ. Ручной контрпример с накопительными
rejections: два несовместимых события A,B имеют вероятности .03. На t=1
только в A отвергнут один истинный нуль; на t=2 в A добавляется одно правильное
отклонение ложного нуля, а в B отвергается один истинный нуль. В остальных
сценариях отклонений нет. Тогда FDR(1)=.03 и FDR(2)=.03/2+.03=.045≤.05.
Но stopping time τ=1 при первом rejection, иначе τ=2 даёт FDR(τ)=.06>.05.
Это контрпример к логическому переносу fixed-time гарантии, не утверждение,
что SAFFRON создаёт такую траекторию или что его отдельные stopping-time
расширения невозможны. Для нашей задачи отсутствуют producer p-values с
доказанной A1; здесь выбирается прозрачный conservative spending,
а SAFFRON — источник границ применимости, **не второй reuse runtime**.

## 4. R03-010-B: кредитная модель без ложного «положительного дрейфа»

Пусть W≥1 — целое начальное число физических кредитов. Попытка тратит один;
успех S_t∈{0,1} возвращает b≥0 целых кредитов. Пока A_t>0:

\[
 A_0=W,\qquad A_{t+1}=A_t-1+bS_t.
\]

Сначала берём независимые S_t~Bernoulli(q), 0≤q≤1. q — вероятность
операционного события, **не уровень статистической ошибки α**. В специально
объявленном all-null toy scenario можно положить q=α, если успех определён
как ложное отклонение independent exact-level теста.

### 4.1 Один возвращаемый кредит

При b=1 и q<1 завершение — W-й неуспех. Каждый waiting time до неуспеха имеет
E[G]=Σ_{k≥0}q^k=1/(1−q). Сумма W таких ожиданий даёт

\[
 E[T]=W/(1-q),\qquad E[\sum_{t<T}S_t]=E[T]-W=Wq/(1-q).
\]

Дрейф q−1<0. При q=1 работа бесконечна. При 0<q<1 support T не ограничен:
для каждого k>W событие достаточно длинной серии успехов имеет положительную
вероятность. Поэтому finite expectation ≠ hard timeout/bytes/RAM bound.

**Важное дополнительное уточнение:** в independent all-null версии, когда
каждый успех — ложная декларация с вероятностью α, отсутствие хотя бы одного
успеха означает, что первые W попыток закончились неуспехом. Следовательно

\[
 \operatorname{FWER}=1-(1-\alpha)^W.
\]

Здесь формула верна **для этой конкретной механики**, а не как worst-case
union bound. При W=1 lifetime FWER=α несмотря на случайное число попыток и
Σ_{t<T}α>α на некоторых траекториях. Это показывает: теорема A1/A2 —
достаточная, не необходимая; blanket-тезис «успешное пополнение никогда не
совместимо с контролем ошибок» тоже неверен. Перенос toy-результата на реальные
успехи, зависимость, selection, другой reward или changing null не разрешён.

### 4.2 Несколько возвращаемых кредитов

Для b≥2 независимая модель эквивалентна обработке W корней дерева, где один
кредит порождает b детей с вероятностью q, иначе ноль. Среднее число детей bq.
При bq<1 expected total work получается суммой по поколениям:

\[
 E[T]=\sum_{g\ge0}W(bq)^g=W/(1-bq).
\]

При bq=1 и b≥2 распределение offspring недегеративно; expected total work
бесконечен, хотя extinction происходит почти наверное. При bq>1 возможна
положительная вероятность неокончания. Проверка extinction без ссылки на
непрочитанную теорему: для **одного корня** вероятность extinction к поколению
g равна итерации f^{(g)}(0), f(s)=1−q+qs^b; это возрастающая ограниченная
последовательность, предел — наименьший fixed point на [0,1]. Для W независимых
корней соответствующая вероятность — [f^{(g)}(0)]^W. При bq=1 для s<1 функция
f(s)−s положительна: её производная bq·s^{b−1}−1=s^{b−1}−1<0, а значение
в 1 равно нулю. Поэтому единственный fixed point — 1. При bq>1, f(0)>0
для q<1, а f(s)−s<0 сразу слева от 1; существует fixed point <1.
При q=1,b≥2 ноль — наименьший fixed point, неокончание детерминировано.
Extinction/survival результаты не нужны для нашей product-границы;
b=1,q=1 — отдельный дегеративный случай выше.

Это ручная вероятностная модель, не доказательство наблюдаемого поведения
Snowden. Если outcomes dependent, reward нефиксирован или q неизвестно,
эти exact ожидания неприменимы. Физическая безопасность должна иметь отдельно
введённые hard caps admissions, bytes, pending owners, времени/retention и
actual resource return receipts. Ни α-spending, ни среднее T их не заменяют.

## 5. R03-010-C: минимальный producer-контракт

Не новый публичный API и не реализованный ledger. Требуются следующие семантики,
а не обязательный заранее фиксированный список W:

| Область | Минимальное содержание | Отказ от inference |
|---|---|---|
| Семья и цель | FamilyID, null/estimand, population/unit definition, support, model/filtration, error objective, sides и процедура/version | unknown target/model; per-look процедура названа anytime |
| Admission | Уникальный operation ID, admitted ordinal, выбранные до данных α_j и конфигурация, reference на использованную историю; statistical reservation | post-hoc registration, reused unit под новым ID |
| Ordering | Отдельные admission/start/reveal/record/publication orders; local monotonic times + clock domain; frozen predictor refs | сортировка по finish time объявлена исходным порядком |
| Completeness | Один контролируемый execution entrypoint для claimed population; roster rejected/admitted/not-started/started/pending/returned/missing/cancelled; witness его coverage | unknown launch path или gap в roster |
| Outcomes | task semantics; partial/missing/censored/abandoned раздельно; fixed follow-up origin/horizon при rate; late result привязан к той же unit | timeout автоматически превращён в completed RTT/task failure |
| Cost | planned/reserved/actual counts отдельно, единицы, unknown значения; cap admissions/pending/bytes/retention; resource retirement receipt | deadline/send-cancel объявлен resource release |
| Error budget | All admissions charged по одному семейному accountant, включая параллельные clients/epochs; reserved/spent, rejects/underflow/overflow | перезапуск БД/эпохи пополняет δ; unsupported tiny α становится 0 |
| Publication | Snapshot ID, roster prefix/completeness flag, опубликованные claims и их original family refs | selective FDR полного набора перенесён на published subset |

Current source здесь **доступен**, не SOURCE_NOT_AVAILABLE. Статически прочитаны
`shared/adaptive/evidence_types.go` и фрагменты `budget_types.go`, `windows/go.mod`.
Observation уже имеет Sequence, planned/admitted/started/finished timestamps,
censoring и custody; MaxReceipts=64 ограничивает snapshot, не lifetime.
PlatformAccepted — caller projection, не аутентификация producer. Полей
статистического accountant/условной p-value validity в этих двух типовых
контрактах не установлено. Это узкая карта прочитанных seams, не полный аудит
Windows/Android и не отрицательное доказательство отсутствия любого ledger.

**Hash-chain:** помогает обнаружить изменение связанных записей относительно
независимо удержанного head. Сам не доказывает, что (a) были записаны все
запуски, (b) запись существовала до просмотра outcome, (c) outcome верен,
(d) head не откатился вместе с журналом. Нужен trusted execution gate и
scope/completeness witness; timestamp собственного producer не внешний witness.
Сохранённый журнал даже при полной честности не раскрывает truth H_j сам.

### 5.1 Отказ до статистики

Если известно только, что зарегистрированы N бинарных исходов и наблюдены S
успехов, а M исходов неизвестны, конечная realized success доля находится в
[S/N,(S+M)/N] при известном denominator и корректных labels. Это
**идентификационный интервал**, не CS, не risk следующей попытки. Когда
denominator неизвестен, даже этот finite-cohort результат не устанавливается.
Если labels corrupted, нужен отдельный error envelope из §17 baseline,
а не invented α. Честный default — typed observations + abstain до A1.

## 6. Reuse только после доказательства

### 6.1 Единственный journal-кандидат: уже pinned SagerNet/bbolt

Версия в **windows/go.mod:102** и upstream sing-box/go.mod:29:
`v0.0.0-20231014093535-ea5cb2fe9f0a`, module cache timestamp
2023-10-14T09:35:35Z. LICENSE прочитан: MIT, Ben Johnson (2013).
Функции прочитаны по exact cache source [C1]; свежая внешняя побайтная
проверка полного bbolt pin не выполнена (GitHub API 403), что явно ограничивает
provenance, но не скрывает available source.

- DB.Update (db.go:825–856): fn внутри одного write transaction, ошибка fn
  откатывает; Commit error возвращается caller.
- Bucket.NextSequence (bucket.go:399–415), Put (:310–342), DB.View/Cursor:
  хватит для atomic admission record + budget reservation в одном writer.
- **Put перезаписывает** существующий key; NextSequence существует рядом с
  SetSequence/Delete. Следовательно bbolt не append-only по API: owner должен
  enforce unique immutable operation IDs и закрыть произвольные мутации.
- Tx.Commit (:134–223) пишет dirty pages, meta и лишь затем commit handlers.
  `NoSync` исключает fsync и помечен unsafe; successful call без долговечного
  storage contract нельзя превратить в unconditional power-loss proof.
- DB.Batch (:890–928) может вызывать callback несколько раз: запуск probe или
  другой внешний side effect **внутри callback** запрещён предлагаемым
  контрактом. Отдельные шаги: commit reservation → caller получает success →
  platform admission gate → старт. Не применять OnCommit для обещания atomic
  транзакции вместе с ОС-операцией: crash между commit и стартом оставляет
  not-started/missing запись; это нормальный отдельный outcome.
- Byte slices из DB действуют только в течение transaction [doc.go:36–38].
  Frozen export должен копировать bytes до закрытия View; long-lived readers
  и mmap не дают автоматический bound RSS/retention.

**Реальный upstream caller**: `sing-box v1.13.19/experimental/cachefile/cache.go`:
Open (:107–135), Batch очистки buckets (:141–159), View/Batch/Update
wrappers (:175–202), resetDB (:205–214). Wrapper удаляет неизвестные buckets
и может удалить/пересоздать БД после corruption. Поэтому брать *его файл*
для научного ledger нельзя. Только отдельно owned storage на том же low-level
примитиве, если platform owner выберет такую архитектуру; не новый service,
не зависимость, не write в cachefile. Current ledger consumer не обнаружен
в прочитанном caller — он хранит cache, не историю статистики.

Стоимость: отдельные lifecycle/ACL/quota/retention/backup/restore contracts,
serial write latency, copy для snapshots, риск deadlock/долгих View, потеря
complete cohort при deletion. Windows Go library пригодна как source-level
кандидат; Android имеет другой platform owner и native/ABI/lifecycle gate,
перенос бbolt в Android этим результатом не утверждён. При уже пригодном
platform journal лучше расширить его, чем внедрять новую storage сущность.

### 6.2 Единственная statistical reference: confseq, exact commit

`gostevehoward/confseq@5ffe733ca2447a2e28c2c91f3b00086173f2ab2c`, setup.py
`0.0.11`, LICENSE MIT (2021 confseq developers). Новая secure remote копия
predmix.py совпала SHA-256 с prior captured файлом; LICENSE/README/setup
заново прочитаны. Read scope — selected Python path; numerical/C++/ABI audit
и runtime installation отсутствуют [C2].

API и actual path:
`predmix_hoeffding_cs(x, alpha, truncation, running_intersection=False, N=None,
fixed_n=None)` → lower_cs с alpha/2 + `1−lower_cs(1−x,alpha/2)`
(predmix.py:172–220) → `predmix_hoeffding_lower_cs` (:121–169) →
`predmix_lower_cs` (:10–69). Последняя использует weighted cumulative center,
margin `(log(1/α)+Σλ²/8)/Σλ`. Input x в [0,1], N=None; не imputed unknowns.

**Проверка математического target.** Waudby-Smith–Ramdas [P4, §2, §3]
предполагают постоянное conditional mean μ. Для предсказуемых λ_i при этой
модели process

\[
 M_n=\exp\{\sum_{i\le n}\lambda_i(X_i-\mu)-\sum_{i\le n}\lambda_i^2/8\}
\]

является неотрицательным supermartingale по conditional Hoeffding MGF.
Ville даёт lower boundary, upper через 1−x с α/2 сохраняет общий level α.
Выбор библиотекой семьи/уровня риска не следует из numerical formula:
это отдельная проверка. Для нашей source-проверки требуются
0<alpha<1, положительные конечные λ_i и Σλ_i>0; fixed_n, если задан,
положителен и выбран до использования анализируемых данных. Tiny α,
непредставимый численно, должен дать unsupported, не новый нулевой риск.
Input validation/order/context/family budget — обязанности wrapper, не Python API.

При меняющемся μ_i тот же weighted expression, если применимо conditional
MGF, относится к **weighted historical mean** Σλ_i μ_i/Σλ_i, не к unweighted
average и не current μ_n. Howard [P1, Theorem 4] предлагает иной конкретный
unweighted average-conditional-mean результат; подмена API одним названием
«confidence sequence» недопустима. Running intersection годится для одного
fixed target, не moving target.

`fixed_n` — tuning n_opt, выбранный до анализируемых данных, не обязательный
финальный размер. `predmix_hoeffding_ci` (:296–311) сам ставит fixed_n=len(x).
Последовательное многократное использование этого **CI-wrapper** с новым
len(x) не наследует одну CS, настроенную заранее. Это граница caller-
семантики, не finding actual miscoverage upstream. Для proposed comparison
выбирать CS path; fixed_n/truncation/alpha/procedure freeze до новой unit.

Не устанавливать Python stack на клиент ради этого результата: setup требует
numpy/scipy/multiprocess и прочие пакеты, package build включает C++/pybind11;
README exact pin помечает API early-stage/not stable. Reference для будущей
offline parity, не ready Go/Kotlin adapter, не численно проверенная safety
процедура, не источник неизвестных drift/cost/Γ и pmax.

## 7. Проверенные источники, русский язык и доказательные ограничения

Все формулы выводятся в этом пакете вручную и привязаны к определениям;
источники нужны для проверки модели, а не вместо доказательства.

- P1: полный документ получен (`FULL_TEXT_PARTIAL_READ`), прочитаны **цитируемые разделы** (1), §2, §4.1,
  §6, A.8; v9 metadata «6 Aug 2022» и титульная дата «August 9, 2022» различны.
  Journal 2021 нельзя выдавать за дату этого PDF.
- P2: официальные PDF/index, прочитаны §1–3.2/Theorem 1; FDR против mFDR
  assumptions не упущены.
- P3: PDF прочитан по определениям §1.1 и alpha-investing §3, не весь 31-page
  proof appendices.
- P4: prior full HTML capture прочитан по §2–3, definitions, Theorem 1 и
  predictable-Hoeffding path; свежая внешняя equality paper не выполнена.

Попытки получить русские primary MathNet/MCCME не дали пригодного текста:
403/encoding/unavailable/timeouts. Поэтому Ширяев, Вальд, Воронцов из §18
**не используются как будто прочитанные доказательства**. Отсутствие русского
первичного текста — ограничение этой поставки, а не основание придумывать
теоремы/номера страниц. Референсы и failed retrieval перечислены в SOURCES.
Ошибочно запрошенный arXiv 1705.01058 оказался астрофизикой и исключён.

## 8. Конечная будущая программа — всё NOT_RUN

| Case | Проверяемая предпосылка / ожидаемое поведение |
|---|---|
| T01 | Два несовместимых E_j: sum bound, не independence product |
| T02 | Adaptive α_j при pathwise accountant ≤δ; pending charges не теряются |
| T03 | Выбор min из двух inspected p без общего покрытия → A1 отказ |
| T04 | CS two-sided α/2 + fixed tuning; CI-wrapper не переименован anytime |
| T05 | Fast-success/slow-failure delivery не sorted-completed sample |
| T06 | Frozen predictor/reveal prefix; поздний callback не новый sample |
| T07 | b=1/q<1 finite mean, без заявления hard cap; q=1 отдельный случай |
| T08 | Операционный success и false rejection разные labels; no α→q substitution |
| T09 | Crash после reservation commit до запуска → not-started/missing, не success/refund |
| T10 | DB.Batch повторяет callback; нет внешнего I/O внутри него |
| T11 | Put duplicate/Delete/SetSequence/reset → отказ append-only/completeness certificate |
| T12 | Corruption или dropped roster → unknown; δ не пополняется сбросом БД |
| T13 | Storage cap/disk-full/unsupported tiny α → no new admission, typed error |
| T14 | Copied View bytes, privacy-bounded export, missing/censoring retained |
| T15 | Moving conditional mean → weighted/unweighted targets не смешаны |
| T16 | Snapshot утратил denominator или application outcome → finite/probabilistic abstain |

Это перечень будущих unit/native/data/model проверок для владельцев, не
реализованные fixtures и не выполненные эксперименты. Здесь только ручные
примеры и source inspection. Новые product требования/статусы не создавались.

## 9. Предлагаемый фрагмент теории (не внесён в общие главы)

> Statistical budget, physical budget и полномочия перехода независимы.
> Unknown number of admissions допускает FWER-контроль посредством
> предсказуемого alpha-spending с pathwise суммой ≤δ и conditional anytime
> validity при реальном адаптивном выборе. Журнал может хранить эту композицию,
> но не создаёт её assumptions: полнота запуска, порядок доступа к данным,
> смысл outcome и неизменность семьи требуют producer witnesses. Hash-chain
> не удостоверяет omitted attempts или registration-before-observation.
> При отсутствии корректной модели сохранять typed observations/finite bounds
> и abstain; не заменять неизвестные cost/drift/Γ нулём. CS для прошлого не
> разрешает switch и не доказывает будущую работоспособность пути.

Адресаты: ADP-004/006/008 и TH-02/04/06 — только согласование контрактов;
Windows/Android владеют producers, retirement и admission. Пакет исследователя 9
может использовать §§2–5.1 как математическую базу для denominator/selection,
не общий reported-only rate и не старое произведение §18. Записи в чужие
файлы, REPORT.md/STATE.md/REVIEW-QUEUE, platform source и registry не делались.
