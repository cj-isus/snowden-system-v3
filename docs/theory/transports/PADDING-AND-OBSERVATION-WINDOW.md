<a id="padding-observation-window-2026-10-28"></a>
# Padding: остановка, перезапуск и граница наблюдения детектора

[Оглавление](../README.md) · [Модель противника](../foundations/THREAT-MODEL.md) · [Решения review](../research/reviews/2026-10-28-DEEP-REVIEW.md)

**2026-10-28, DR27-04.** Это исследование метода оценки и опубликованных
результатов других систем. Website fingerprinting внутри Tor и классификация
proxy/benign являются разными задачами; новый детектор российских сетей не
обучался и его точность не измерялась.

## 1. Политика stop/restart является частью защиты

Авторская версия **WFDefProxy v1 (2021-11-24)** сравнивает симуляцию, знающую
конец загрузки, с реальной soft-stop реализацией. У Tamaraw поздний старт и
промежуточные остановки уменьшали padding. У FRONT повторный старт порождал
новый front-padding schedule и мог увеличить расход. Направление изменения
стоимости поэтому зависит от механизма, не только от длительности prefix.
[Первичный текст, §§4/6](https://arxiv.org/html/2111.12629v1).
Идентичность этой версии поздней TIFS-публикации здесь не утверждается.

В той же работе data overhead усредняется по traces, а time overhead
заканчивается на последнем **настоящем** packet. Эти метрики не описывают
автоматически hard session quota и tail-cover после useful completion.
Новая граница не обязательно ухудшает защиту: reported FRONT реализация при
сопоставимом среднем overhead могла давать меньший TPR DF; вклад stop/restart
отдельным causal intervention не изолирован.

Поэтому единица контракта:

`schedule × stop rule × restart rule × quota/reset scope × observation horizon`.

Source prefix-limit сам по себе не доказывает remote distinguishability.
Например, pinned Naive Variant1 имеет переход к WriteNoPadding после8 frames,
но обнаружимость этой границы по внешнему trace требует отдельного корпуса.
[Pinned writer](https://github.com/klzgrad/naiveproxy/blob/d04efe1ea4477c2db24a7d0e2f068b3bee931e45/src/net/tools/naive/naive_padding_socket.cc).

## 2. Конечный cover budget не гарантирует скрытия объёма

В упрощённой lossless padding-only модели без compression/постороннего трафика
`W=U+P`, `0≤P≤C`. Если `sup(U0)+C < inf(U1)`, итоговые объёмы двух классов
имеют непересекающиеся диапазоны при любом выборе такого P.
Это условная нижняя граница модели, а не найденный сетевой classifier.
Wire bytes, application bytes, внутренние records и direction counts нельзя
смешивать при проверке неравенства.

С другой стороны, исчерпание бюджета не обязано быть видимо наблюдателю.
Потребуются реально наблюдаемые timing/size признаки и проверка ошибки на
benign corpus; внутренний timestamp `padding exhausted` не даётся атакующему
как будто он известен на проводе.

## 3. Окно модели может спрятать сигнал только от самой оценки

В официальном артефакте *Ephemeral Network-Layer Fingerprinting Defenses*,
PoPETs2026(1), commit `31ff58466742c69b738191bc66789064bac849c2`, DF-config
использует directions и `trace_len:10000`; `PadOrCutTrace` отрезает дальнейшие
элементы. Dataset branch задаёт цель `page_label`.
[Config](https://github.com/maybenot-io/popets-2026.1-ephemeral-defs-paper-artifacts/blob/31ff58466742c69b738191bc66789064bac849c2/def-ml/config/model/df-orig.yaml),
[features](https://github.com/maybenot-io/popets-2026.1-ephemeral-defs-paper-artifacts/blob/31ff58466742c69b738191bc66789064bac849c2/def-ml/def_ml/trace/features.py),
[target](https://github.com/maybenot-io/popets-2026.1-ephemeral-defs-paper-artifacts/blob/31ff58466742c69b738191bc66789064bac849c2/def-ml/config/dataset/bigenough.yaml).

Если cover вытесняет информативный фрагмент за cutoff, эта модель его теряет,
хотя наблюдатель полного соединения продолжает видеть. Source устанавливает
ограничение входа; он **не** устанавливает долю опубликованного эффекта,
объясняемую этим механизмом. Другие модели статьи не получают такой verdict
автоматически; полный PDF этой работы в исследовании не был прочитан.

Для late inner handshake нужно сравнить first-N и заранее заданное полное
внешнее окно с idle/restart/tail. Administrative capture end не выдаётся за
естественный конец соединения. Производные одного trace не должны попадать
одновременно в train и test как независимые samples.

## 4. Open world не означает будущий день и незнакомый транспорт

[RegulaTor v3, §§4.2–4.3](https://arxiv.org/html/2012.06609v3) оценивает website
fingerprinting, а перенос параметров защиты между datasets не тождественен
замороженному proxy-classifier на будущих днях/новых carriers.
В PoPETs-артефакте rotating dataset folds сами не устанавливают forward-time split.

Даже обычное преобразование base rate в PPV предполагает пригодность условных
TPR/FPR к новой популяции. Изменение дня, transport или benign mixture может
изменить эти величины; переставить только prior π недостаточно.
Для двухступенчатого AND-каскада gate G→detector D:

`TPR_all=P(G|Y=1)P(D+|G,Y=1)`;
`FPR_all=P(G|Y=0)P(D+|G,Y=0)`.

Это условное разложение не требует независимости стадий. FPR после отбора
gate не является FPR всего входящего потока. Отдельные endpoint-level и
flow-level решения также имеют разные знаменатели.

## 5. Следующий ограниченный опыт — NOT_RUN

Для E-04 сначала определить proxy/benign estimand, уровень carrier/endpoint,
старт/окно наблюдения и доступные атакующему признаки. Сравнить cold, warm,
late-stream, sparse/idle и bulk при одном согласованном transport profile.
Фиксировать stop/restart, extra bytes и tail-cover отдельно от useful latency.

Обучение/threshold замораживаются до future-day test; один transport полностью
удерживается вне train/validation. Первый-N и полнооконный representations
сравниваются на тех же допустимых исходных traces. Failed/censored runs
сохраняются; внутренние oracle events не используются как wire features.
Sample size выбирается по необходимой точности FPR и реальным independent
blocks, а не по числу удобных успешных traces. Всё перечисленное **NOT_RUN**.

Reuse evaluation-подходов возможен после замены цели/корпуса/splits и проверки
licenses/data provenance. Новые padding presets, dependency или обещание
неотличимости из этого обзора не следуют. Приоритетный открытый вопрос —
late handshake на тёплом carrier после окончания prefix против обычной
поздней активности браузера в новых network/day/endpoint blocks.
