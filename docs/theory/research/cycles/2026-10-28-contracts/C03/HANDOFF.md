# R28-C03 — HANDOFF

28 октября 2026.
**READY_FOR_REVIEW; claims PROPOSED.** Научное принятие и интеграция требуют отдельного scoped решения.

Основной аргумент: [REPORT](REPORT.md). Pins/read depth: [SOURCES](SOURCES.md).

## Короткий ответ

Для обоснованного адаптивного выбора нужны четыре отдельные вещи:

1. Право выполнить действие в текущем состоянии защиты и сети.
2. Полный учёт исходной когорты попыток, включая поздние/отклонённые/неполные.
3. Статистическая гарантия именно для выбранных loss, порядка и процедуры.
4. Обоснованная связь исторического результата с целевой будущей работой.

В просмотренном Windows/API1 пути есть сильные current/physical guards, но нет
producer всех четырёх частей. Отсутствие prospective bridge оставляет future
target unknown/[0,1]; это не повод ослабить защиту или остановить обычную recovery.

## R28-C03-A — current projection не complete cohort

**Новая дельта:** прослежен actual путь
`onNetPathChanged → netChangeReprobe → appProtectedProbe.Run → AdmitDiagnostic /
HTTP → verifySessionProtection / publishObservation → optional preference →
switchChannelCoreFor`.

Latest-value publisher намеренно не даёт старому ordinary report регрессировать
текущее состояние. Статистический учёт той же исходной когорты сохраняет его
как original unit outcome. `Manager.LastProbeReport` — один report, не журнал;
Run ID не учитывает события, ушедшие по busy/no-session веткам до Run.

**Предложение:** bounded typed read-model `EvidenceForDecision/WhyUnknown` с
отдельными CurrentGate, Roster, LossClaim, CoverageMode, Kernel/Envelope/Transform,
Bridge и PhysicalFeasibility. Не передавать весь roster в каждый вызов, если
producer уже обеспечивает достаточный frozen summary с проверяемой provenance.

**Конечное правило:** при выполненных current/physical gates и совместно
обоснованных target intervals сравнить full-policy losses в одинаковых единицах:

- `L_A−U_B>m` → SuggestCandidate(B), с Expected и обязательным platform recheck.
- `U_A−L_B≤m` → CandidateDoesNotClearMargin для этой пары/горизонта.
- Иначе → Abstain; missing assumption остаётся явной WhyUnknown.

B включает переход/probe/rollback/failure costs по объявленной цели. Это
условное сравнение ожидаемых losses, не гарантия отдельного будущего исхода.

**Сильные opposing cases:** slow failure/fast success, скрытое событие до Run,
смена owner/epoch, одинаковая история с противоположным будущим. Existing
current gates реально прочитаны; claim не объявляет уязвимостью нынешнюю
deterministic preference. Automatic netWatcher пропускает polling при
`tunRequested()`: этот caller review не доказывает TUN/Android coverage.

## R28-C03-B — envelope должна соответствовать kernel и postprocessing

**Новая дельта:** baseline aggregate Γ ограничивает обычные sums, а selected
confseq caller использует weighted center. Один current Γ_n не переносится
на weighted error без дополнительного условия.

**K1:** true losses(1,0), reported(0,1), веса u>v>0. Final ordinary error=0,
но weighted error=u−v. Поэтому current Γ2=0 не достаточна для такого adapter.

Для nonincreasing weights и jointly valid envelope каждого prefix достаточно

`κ_n = w_n Γ_n + Σ_(k<n)(w_k−w_(k+1))Γ_k`.

Это ручной summation-by-parts вывод; predictable kernel/filtration проверяются
отдельно. Не новая CS и не upstream defect.

**Более простой scoped кандидат:** frozen до данных `fixed_n=Nopt>0`, `N=None`,
`running_intersection=False`, true-loss constant-conditional-mean model. Тогда
w постоянен, weighted mean обычный, достаточно current Γ_n для отдельного query:

`w=sqrt(8 log(2/α)/Nopt)`, `r_n=log(2/α)/(n w)+w/8`.

Robust interval расширяет `[Σa/n,Σb/n]` на `Γ_n/n+r_n`. Цена: radius стремится
к w/8, не к нулю. Повторное `fixed_n=len(x)` после каждого просмотра не одна
заранее выбранная CS. Другая историческая модель требует отдельного proof binding.

**K2:** correction должна предшествовать running intersection и относиться к
одному target. При α=2e^(-2), Nopt64, w1/2, истинных losses0, первых64 reported1
и следующих64 reported0, Γ_k=min(k,64). Raw lower на64=7/8; raw running lower
на128 минус current Γ128/128 остаётся≥3/8 и ложно исключает µ0. Corrected prefix
bounds перед пересечением не делают этой ошибки. Для меняющихся θ_k простое
пересечение разных targets вообще не обосновано.

**Recommendation:** ADAPT mathematical reference только с явными
KernelRef/EnvelopeKind/TransformRef и порядком postprocessing. Wrapper не реализован,
численная устойчивость/parity не проверены. Midpoint missing labels не адаптер.

## Что reuse, а что отложить

| Объект | Решение / цена |
|---|---|
| Existing owners/API1 | ADAPT exact identity/current coverage/Expected/settlement; bounded owner summary. Полный probabilistic producer DEFER до модели/cohort/bridge |
| confseq0.0.11 | OFFLINE_REFERENCE, MIT, commit `5ffe733ca2447a2e28c2c91f3b00086173f2ab2c`; выбранные Python functions/лицензия/setup прочитаны, пакет не установлен |
| SagerNet/bbolt | CONDITIONAL_REUSE DB primitive; MIT, pseudo `v0.0.0-20231014093535-ea5cb2fe9f0a`, full commit `ea5cb2fe9f0afaec4827c0591954db0e11ba548c`. Selected fresh source hashes совпали с cache |
| sing-box CacheFilev1.13.19 | REJECT как lifetime statistical ledger: cleanup/whole-file reset. GPL-3.0-or-later wrapper не становится MIT из-за low-level зависимости |

Single writer не исправляет stale arithmetic вне transaction. `Batch` может
повторить callback; `OnCommit` не атомизирует внешний effect. При выбранной
persistence reserve+record делаются DB-only, затем successful commit, owner
recheck/unique consumption, effect, original outcome/custody update. Crash между
commit и effect остаётся not-started/unknown. Не добавлять storage I/O в nested
publication critical section. Для closed process-local cohort новая БД не нужна.

## Пройденные stages и связанные углубления

1. Exact manifest/pin и actual producer→caller lineage — source-only завершено.
2. Minimum typed contract и finite rule — сформулированы, assumptions видны.
3. Opposing traces — delay/hidden attempts/stale proof/source reset/future risk,
   stale budget/Batch/custody/duplicate и K1/K2 разобраны вручную.
4. API1 projection, reuse, licences и costs — адресно разобраны.

Weakest bridge: дополнительные одинаковые HTTP passes не идентифицируют future
useful-task risk. Нужны нужные task outcomes/full-policy target и отдельная
prospective guarantee. Если future mean failure loss µ уже обоснован и N известно,
то `µ≤P(any failure)≤min(1,Nµ)`; temporal bridge этим не заменяется.

Selected claims: data-dependent subset/threshold внутри уже jointly covered
семьи не требует нового α. Новый target/procedure/workload — другой claim и
требует валидности относительно actual pre-admission history, включая pending
feedback. Physical credits, α_stat, label β и bridge η не взаимозаменяемы.

## Доказательства, ограничения, следующий scoped шаг

- 23project и8module captures, before/copy/after provenance; финальная hash-сверка
  09:29:35Z:31/31 unchanged. Не atomic snapshot всего дерева/бинарника.
- Z03/Z09 HANDOFF отсутствовали при review-stage check09:23:06–07Z.
  **NOT_AVAILABLE_AT_CHECK**; paired independent producer/cohort review открыт.
  Два этапных availability checks не polling/ожидание соседей.
- D1/K1/K2/O11 — ручные конечные конструкции; source/spec/model/field разделены.
  Исполняемые cases не создавались, F1…F10 **NOT_RUN**. Native/device/live,
  VPS, скорость/энергия и production reliability **NOT_EVALUATED**.

Предложенная последовательность после отдельного разрешения на следующий срез:
owner выбирает одну наблюдаемую task/goal → задаёт closed bounded roster и branch
losses → обеспечивает summary/model/family/kernel binding → отдельно обосновывает
target bridge → получает независимый review → только затем разрешённая реализация
и приёмка.
