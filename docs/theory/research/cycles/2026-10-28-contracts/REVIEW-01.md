# R28 — первая независимая рецензия

28 октября 2026.
Область — выбранные source/model выводы из готовых пакетов. Приём всей работы
по числу файлов, совпадению ответов моделей или авторскому READY не проводился.

[Очередь остальных выводов](APPLICABILITY-LIMITS.md) · Состояние цикла — `docs/archive/theory-organization-2026-10-28/r28/STATE.md`.

## Решения и границы

| Решение руководителя | Авторский материал | Принятая область |
|---|---|---|
| **R28-REV-01: ACCEPTED — algebra/source scope** | [C03, §6.3](C03/REPORT.md) | Различие обычной и взвешенной оболочки, формула для монотонных весов, порядок поправки и пересечения. Не вся предлагаемая CS/decision policy |
| **R28-REV-02: ACCEPTED — two-function source scope** | [Z03-A](Z03/HANDOFF.md), [C03-A](C03/HANDOFF.md) | Ранний ID и защищённая latest-value публикация сами не предоставляют полный учёт статистической когорты. Не отрицание всех producer во всём проекте |
| **R28-REV-03: ACCEPTED — pinned projection scope** | [C05-A](C05/HANDOFF.md), [Z06-A](Z06/HANDOFF.md) | В выбранном Google candidate отображение валидного SUSPENDED и отсутствующего serial в один membership-результат. Не доказанный дефект интегрированного VPN |
| **R28-REV-04: ACCEPTED — operational refinement** | [Z06-B](Z06/HANDOFF.md), [C05-A](C05/HANDOFF.md) | Связность evidence и актуальность допуска непосредственно при admission. Контрпример не опровергает полный исходный контракт с current authority и сохранением accepted deny |

Остальные части этих пакетов и все claims C02/C04 остаются в очереди.
Ни один новый продуктовый PASS, реализованный адаптер или native/live результат
этими решениями не объявлен. Авторские отчёты и их статусы не переписаны.

## 1. Взвешенная оболочка и порядок операций

### Что независимо прочитано

[confseq predmix.py, commit 5ffe733…](https://github.com/gostevehoward/confseq/blob/5ffe733ca2447a2e28c2c91f3b00086173f2ab2c/src/confseq/predmix.py):
`predmix_lower_cs` 40–64, Hoeffding 141–157, two-sided 189–205 и CI wrapper
276–291. В выбранном коде центр взвешен; running intersection выполняется над
уже вычисленными bounds. Fixed tuning даёт постоянный вес, two-sided делит α.
Это чтение исходника, не исполнение Python.

Прочитаны C03 REPORT §6.3 и исходная
[aggregate envelope](../../../adaptation/INTERVALS-AND-POLICY-EVALUATION.md)
§1. Точный свежий primary capture совпал с авторским по SHA256.

### Независимый ручной вывод

Пусть Y=(1,0), a=b=(0,1), веса u>v>0. Обычные суммы совпадают,
поэтому Γ₂=0 совместима с Γ₁=1. Взвешенные суммы отличаются на u−v.
Следовательно, одна конечная aggregate Γ₂ не ограничивает этот weighted error.
Это не пример ошибки библиотеки: расширенный corruption contract ей не передан.

Для Dₖ=Σᵢ≤ₖ(Yᵢ−aᵢ) и w₁≥…≥wₙ≥0:

$$
\sum_{i=1}^{n}w_i(Y_i-a_i)
=w_nD_n+\sum_{k=1}^{n-1}(w_k-w_{k+1})D_k.
$$

При совместных Dₖ≥−Γₖ все коэффициенты неотрицательны, поэтому достаточно

$$
\kappa_n=w_n\Gamma_n+
\sum_{k=1}^{n-1}(w_k-w_{k+1})\Gamma_k.
$$

Для верхней стороны аналогично. Монотонность весов и одновременность prefix
оболочек существенны. Pathwise алгебра не доказывает predictable sampling model.
Если сама оболочка вероятностна с ошибкой β, β требуется отдельно учесть в
совместном покрытии; её нельзя спрятать внутри α статистического kernel.

Проверен K2 вручную. При α=2e⁻², Nopt=64 получаем w=1/2,
r₆₄=2/(64·1/2)+(1/2)/8=1/8. После 64 ложных единиц raw lower=7/8.
На n=128 running lower сохраняет ≥7/8; позднее вычитание Γ₁₂₈/128=1/2
оставляет ≥3/8 при истинном µ=0. Поправленные bounds каждого prefix этого
ложного исключения не дают: observed mean−Γₖ/k=0 для всех этих prefixes.

**Принято:** K1, формула κ и K2 как конечные алгебраические аргументы;
исправление каждого prefix предшествует пересечению для одного и того же target.
Это не новая самостоятельная теорема о полевом риске VPN.

**Не принято этим review:** actual conditional-mean model, completeness,
filtration, численная устойчивость, реализация wrapper, оптимальность fixed
tuning и prospective bridge. Обоснованность вероятностного покрытия требует
этих отдельных предпосылок. Для разных θₖ простое пересечение targets не следует.

## 2. Current projection и полная когорта — разные потребители

Непосредственно прочитаны текущие:

- `windows/probe_observation.go`, `appProtectedProbe.Run`, 84–145;
- `windows/observer_publication.go`, `publishObservationLocked`, 41–129.

Оба файла скопированы с before/after hash-сверкой в private review artifacts.
Первый выдаёт atomic sequence до основного выполнения и дополняет report
identity/time/staleness. Второй намеренно отклоняет ряд stale/ordinary/неподходящих
для current owner/epoch публикаций и фильтрует health update.

Из этих двух функций не следует complete durable roster: current projection
решает задачу публикации актуального состояния, а не сохранения каждой заранее
определённой statistical unit. Это ограничение конкретного seam, не результат
поиска «во всём проекте ничего нет» и не новая уязвимость.

**Сильный guard сохранён:** unsafe report не отбрасывается только из-за меньшего
ObservationID; current owner, epoch, validity и intent действительно проверяются.
Предложение сохранить outcome в иной accounting projection не разрешает
ослабить эти guards или публиковать старый успех как текущий.

**Принято:** C03-A/Z03-A в указанной области. Полный предлагаемый producer
protocol, bbolt durability/ACK/rollback, формулы started-only и actual scoring
consumer этой рецензией не приняты: их отдельная source/model проверка впереди.

## 3. Валидный SUSPENDED теряется до membership checker

Независимо прочитаны по точному pin `34652d3df68b8d6887a6670b00eb0856090dae6d`:

- [GoogleRevocationList.kt](https://github.com/android/keyattestation/blob/34652d3df68b8d6887a6670b00eb0856090dae6d/src/main/kotlin/GoogleRevocationList.kt), 44–82;
- [RevocationChecker.kt](https://github.com/android/keyattestation/blob/34652d3df68b8d6887a6670b00eb0856090dae6d/src/main/kotlin/provider/RevocationChecker.kt), 28–49.

Первый оставляет только REVOKED serials; второй проверяет membership переданного
набора. Свежие raw captures совпали с авторскими. Опубликованный
[Android schema](https://developer.android.com/privacy-and-security/security-key-attestation)
содержит оба допустимых статуса REVOKED и SUSPENDED.

Поэтому валидный SUSPENDED и NotListed дают одинаковое отсутствие serial в этом
наборе. Для этого вывода не нужны malformed JSON, Gson coercion, runtime
reflection или предположение, что HTTP error возвращает пустой набор: код на
неподходящий HTTP ответ бросает исключение, и этот guard сохранён.

**Принято:** lossy projection выбранного candidate и необходимость сохранить
семантику статуса, если наш claim зависит от SUSPENDED. Это не требование считать
его REVOKED и не доказательство текущего product bypass. Policy может иметь
разные решения для разных статусов; chain verification не заменяет эту policy.
Malformed parser cases, весь Gson/JCA/provider и alternative schema stack pending.

## 4. Поздний успех и новая отрицательная информация

Независимо сопоставлены Z06 HANDOFF и исходный
[R05-009-D, §10.3](../../05-TRUST-SAFETY/REPORT-009.md): последний уже требует
current authority и прямо сохраняет ранее accepted deny при source error.

Ручной порядок событий: начата проверка E₀ → принят E₁ deny → новый refresh
ошибается → приходит поздний E₀ success → caller пытается допустить операцию.
Отдельные labels E₀ могут оставаться истинными относительно прошлого времени,
но из этого не следует current operation permit.

**Принято как уточнение реализации контракта:** результаты относятся к одному
owned evidence; admission сверяет его с текущими subject/scope/intent/policy и
accepted history, проверяет возраст именно в момент допуска. Reference equality
или CAS сами не доказывают deep immutability/свежесть/долговечность. Если deny
пришёл после admission, дальнейшее поведение задаёт отдельная in-flight policy;
мгновенный отзыв уже совершённого эффекта здесь не обещан.

**Ограничение возражения:** E₀ trace запрещён сильным прочтением исходных current
authority/accepted-deny требований. Поэтому он не основание отвергать весь D
или приписывать ему доказанный logical bypass. Уточняется operational binding,
а не добавляется обязательная новая БД, ещё один статус или собственная криптосхема.

Z06 явно сообщил, что ранее писал baseline D. Его пакет — полезная повторная
source/model самопроверка, а не независимый голос по числу моделей. Решение выше
основано на прочитанном исходном контракте, exact source и ручном контрпримере.

## Provenance и остаток

Private evidence: `.artifacts/research/r28-contracts/review-2026-10-28/`.
`source-provenance.json`: девять local inputs stable during copy, три fresh
primary файла совпали с author captures. Hash не является доказательством
правильности всего snapshot/бинарника. Read depth ограничен названными sections.

Все контрпримеры разобраны в тексте вручную; никакие tests/fixtures/simulations,
библиотечные/криптографические вызовы, native/live или чтение реальных статусов,
ключей, profiles, vault и пользовательских логов не выполнялись. 

C02 backend identity/first-write, C04 cancellation/reap и Z01/Z04/Z05/Z07/Z08/Z09
требуют отдельных scoped решений. Появление файла не означает READY,
READY не означает ACCEPTED.
