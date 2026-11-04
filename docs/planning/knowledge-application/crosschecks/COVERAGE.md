# Границы переноса знаний в разработку

[Планы ПК и Android](../README.md) · [Согласованные ограничения](../RECONCILIATION.md)

Сопоставить знание с кодом — значит назвать конкретный вывод, его предпосылки,
потребителя и проверяемое изменение. Родственная тема или заполненная строка
реестра этого не доказывает. Структура корпуса, source-review, математический
вывод и работоспособность приложения остаются разными уровнями.

Предметные замечания ниже сохранены из ограниченной сверки 28 октября 2026 года.
Она не была независимой научной проверкой всех аргументов корпуса. Этот текст
не повышает её статус и не объявляет каждый исторический finding текущим дефектом.

<a id="cov-01-mapping-объясняет-другой-technical-unit"></a>
## COV-01. Родственный дефект не заменяет точный вывод

В [TLS-контрактах](../../../theory/transports/COMPOSITION-AND-TLS-CONTRACTS.md)
рассмотрен путь `requested MinVersion → uTLS preset → mutualVersion`.
Его область — конкретный Chrome/non-real-ECH caller, а не автоматически
REALITY/kTLS, STD или любой внешний version gate.

Это отличается от TLS-less VLESS→ShadowTLS. Исправление конфиденциальности
одной композиции не закрывает вопрос requested TLS floor в другой. Наличие
uTLS/Chrome в renderer не устанавливает, что продукт запросил minimum1.3.
Структурные TLS bounds Android также не аттестуют native artifact.

Для переноса нужны backend/preset/version-check caller, фактический requested
floor и two-sided compatibility. До этого вывод — compatibility/review gate,
а не безусловное требование менять ядро. Сохраняются certificate/Finished и
другие проверки; отсутствие одного guard нельзя расширять до отсутствия всех.

Связь: [NETWORK N03](../sections/NETWORK.md#n03), TH-01/05, OFF-04, AND-01/06.

<a id="cov-02-смешанное-administration-требует-точного-substantive-crosswalk"></a>
## COV-02. Содержание нельзя терять из-за прежнего назначения файла

Служебный документ мог содержать полезный контрпример. При очистке сохраняется
сам контрпример и его источник; распределение заданий и дедлайны не становятся
требованиями продукта.

| Содержательный вывод | Где он применяется |
|---|---|
| Receive window не ограничивает всю RAM; PTO maximum не задаёт minimum; singleflight не доказывает завершение старых поколений | [Производительность](../sections/PERFORMANCE.md), F15/F04/F06/F07, RQ-03/05, TR-04/06 |
| CCA-трилемма требует собственных предпосылок; старая ошибочная формула не восстанавливается как модель | [Ресурсы и транспорт](../../../theory/performance/TRANSPORT-AND-RESOURCES.md) |
| Artifact verification, OS signing, installed/executing image и adoption — разные факты | [Release evidence](../../../theory/architecture/RELEASE-EVIDENCE-BOUNDARIES.md), R05-004/005/006, TH-03, AND-14 |
| Offline status, multiple-repository agreement и old-client compatibility имеют отдельные пределы | [Доверие](../sections/TRUST.md), K8/K4/K2, [непрерывность доверия](../../../theory/architecture/TRUST-CONTINUITY-AND-AGREEMENT.md) |
| Reading depth, исправление R03-17 и source/model/field evidence нельзя смешивать | [Метод](../../../theory/methodology/RESEARCH-METHOD.md), ADAPTATION F17/F11, [поправка R03-17](../../../theory/research/reviews/NINE-2026-10-27-R03-17-ERRATUM.md) |

<a id="cov-03-восемь-duplicate-пар-имеют-разные-link-contexts"></a>
## COV-03. Одинаковый текст не всегда означает одинаковое основание

Перед объединением заметок проверяются дата, scope, source pin, целевая версия
и разрешённые относительные ссылки. Один абзац из архива и новой главы может
ссылаться на разные evidence или рецензии.

Равный hash не переносит ACCEPTED на соседнее утверждение. Нормализованный
body fingerprint, например после `trimEnd()+LF`, не равен byte identity:
original source hash и normalized snapshot hash должны различаться явно.

Архив авторского сборника нужен для сверки, тематическая глава — для текущего
чтения. [Карта исходных разделов](../../../theory/CONTENTS.json) хранит их
соответствие; исторические hashes не аттестуют нынешний код.

<a id="cov-04-late-reader-derivatives-и-r28-reader-index"></a>
## COV-04. Поздняя сводка не создаёт нового научного решения

[Ограничения применимости](../../../theory/research/cycles/2026-10-28-contracts/APPLICABILITY-LIMITS.md)
и [границы знания](../../../theory/research/LIMITATIONS.md) связывают уже имеющиеся
выводы. Они не превращают pending в ACCEPTED и не добавляют доказательство
самим появлением файла. Для производного вывода нужен его исходный claim/scope.

[Материалы по контрактам](../../../theory/research/cycles/2026-10-28-contracts/README.md)
уже доступны через читательский каталог. Прежнее отсутствие их маршрута —
исправленная навигационная проблема, а не оставшийся научный дефект.
[REVIEW-01](../../../theory/research/cycles/2026-10-28-contracts/REVIEW-01.md)
действует только на названные в нём scoped claims.

<a id="cov-05-общая-family-summary-не-покрывает-каждое-distinct-утверждение"></a>
## COV-05. Принцип покрытия полей не требует внедрения любого примера

[Границы probe](../../../theory/observability/PROBE-PROOF-BOUNDARIES.md) разделяют
fresh signed nonce, доверие signer как измерителю и частичное HMAC coverage
TWAMP. Для принципа нужны covered fields, scope и request correlation.
TWAMP может остаться reference-only контрпримером: из ссылки не следует
внедрение нового протокола, signer или сервиса.

Отдельная задача диагностики — egress comparison не равен confirmed leak.
Совпадение или различие удалённых IP интерпретируется с topology/path premises.
Existing unavailable/context guards и safety precedence сохраняются;
unknown не становится healthy и не разрешает direct fallback.

Связь: [NETWORK](../sections/NETWORK.md), N07, TH-02, OFF-05, FR-003a, AND-07/A-FR-06.

<a id="cov-06-устранённые-ранние-пропуски-не-должны-остаться-ложными-finding"></a>
## COV-06. Deferred reference не является обязательной новой функцией

[Padding и окно наблюдения](../../../theory/transports/PADDING-AND-OBSERVATION-WINDOW.md)
нужны для оценки schedule/stop/restart/horizon и признаков трафика.
Сохранение этого контракта не требует нового padder, ML runtime или зависимости.

Mux/padding остаются opt-in и требуют обоих концов, lifecycle, resource budget
и FR-012 comparative gate. Ранний пропуск учёта, который уже исправлен,
нельзя выдавать за действующий дефект реализации. Связь: TH-05, E-04/E-08,
COM-18; собственная оценка смысла этих материалов не заменяется счётчиком units.

<a id="состояние-покрытия"></a>
<a id="проверенные-отрицательные-результаты"></a>
<a id="минимальная-последовательность-применения"></a>
<a id="достаточная-запись-mapping-и-предел-проверки"></a>

## Достаточная запись решения

Для предлагаемого изменения явно указываются:

1. Source claim, точная версия, предпосылки и исключённый контрпример.
2. Actual consumer/caller и применимый режим платформы.
3. Existing requirement ID и решение: сохранить, изменить, проверить, отложить
   или отклонить с конкретной причиной.
4. Совместимость, зависимости, reuse/licence/cost и безопасный возврат.
5. Будущая конечная приёмка и ещё не полученные доказательства.

Не каждый источник требует нового кода. Reference/defer/reject допустимы,
когда сохраняют конкретный смысл. Нельзя заменять непрочитанное шаблонной
причиной для всей папки или считать число строк доказательством полноты.
