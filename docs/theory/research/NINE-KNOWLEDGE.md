# ECH, измерения, ресурсы и доверие: карта подробных материалов

Девять предметных разборов с прямыми ссылками на аргументы, источники и принятые ограничения. Научный статус относится к названной области, а не ко всему пакету.

[Общая карта](../KNOWLEDGE-MAP.md) · [Все пакеты](CATALOG.md) · [Реестр решений](REVIEW-REGISTER.md)

## Предметы

| № | Вопрос | Пакет |
|---|---|---|
| 1 | [ECH/SVCB и согласованный bootstrap](#author-1) | R02-013 |
| 2 | [РФ: качество измерений и OONI labels](#author-2) | R02-014 |
| 3 | [Бюджет ошибок, физические кредиты и журнал](#author-3) | R03-010 |
| 4 | [Windows/HY2: UDP, очереди и владельцы ресурсов](#author-4) | R04-010 |
| 5 | [Android: Doze/FGS и цена keepalive](#author-5) | R04-011 |
| 6 | [Attestation, операция и полноценный status consumer](#author-6) | R05-009 |
| 7 | [TUF: Progress, retention и действующая authority](#author-7) | R05-010 |
| 8 | [VPN/MDM, UID capture и принадлежность app-rule](#author-8) | R06-012 |
| 9 | [Полезное восстановление задачи и приватный pilot](#author-9) | R06-013 |

<a id="author-1"></a>

## 1. ECH/SVCB и согласованный bootstrap

**Вопрос:** Какие имена, ECHConfig, адреса и fallback принадлежат одной попытке?

Нормативный ECH/GREASE/retry, Windows CDN/uTLS caller и область полевых наблюдений; D связывает ECHConfig с endpoint.

**Область принятия:** A/B/C ожидают рецензии; исторический D — PARTIAL_AT_CUTOFF.

[REPORT](02-CENSORSHIP/REPORT-013.md) · [SOURCES](02-CENSORSHIP/SOURCES-013.md) · [Handoff](02-CENSORSHIP/handoffs/R02-013.md)

Связанные тематические тексты: [transports/ECH-DNS-BOOTSTRAP.md](../transports/ECH-DNS-BOOTSTRAP.md) · [transports/COMPOSITION-AND-TLS-CONTRACTS.md](../transports/COMPOSITION-AND-TLS-CONTRACTS.md). Связь по вопросу не переносит чужое ACCEPTED на весь пакет.

Разделы REPORT: 1. Пробел и что не принимается за аксиому; 2. Нормативная база — номер, дата, статус; 3. Принятие ECH, GREASE и retry/fallback; 4. Property matrix: наблюдатель и пределы ECH; 5. R02-013-B — actual Windows CDN/uTLS caller; 6. R02-013-C — доказательства фильтрации и их область; 7. Сильнейшие противоположные примеры; 8. Reuse, цена и адресаты (без реализации); 9. Конечные будущие cases — ВСЕ NOT_RUN; 10. Предлагаемая вставка в канон — только предложение; 11. Проверка и предел сдачи; 12. FUP-1-01 — связь выбранного ECHConfig с endpoint.

<a id="author-2"></a>

## 2. РФ: качество измерений и OONI labels

**Вопрос:** Что действительно установлено числом, raw outcome и classifier label?

Lineage процентов/счётчиков, неполная ISP/access matrix, различие raw test_keys, engine и агрегатора; D разбирает версии и слой интерпретации.

**Область принятия:** A принят только в области числовых источников; B/C/D READY_FOR_REVIEW.

[REPORT](02-CENSORSHIP/REPORT-014.md) · [SOURCES](02-CENSORSHIP/SOURCES-014.md) · [Handoff](02-CENSORSHIP/handoffs/R02-014.md) · [Сохранённая рецензия](reviews/NINE-2026-10-27-R02-014-A.md)

Связанные тематические тексты: [methodology/MEASUREMENT-CORPUS-AND-COVERAGE.md](../methodology/MEASUREMENT-CORPUS-AND-COVERAGE.md). Связь по вопросу не переносит чужое ACCEPTED на весь пакет.

Разделы REPORT: 1. Вопрос, baseline и предел результата; 2. R02-014-A — lineage и область числа важнее количества цитат; 3. R02-014-B — матрица наблюдений с незаполненными ячейками; 4. R02-014-C — область источника и наблюдаемость ограничивают диагноз; 5. Применение к системе без изменения продукта; 6. Адресные предложения руководителю (без записи в общие файлы); 7. Конечные непроведённые cases / остаток; 8. FUP-2-01 / R02-014-D — сопоставимость raw outcomes и summary labels OONI.

<a id="author-3"></a>

## 3. Бюджет ошибок, физические кредиты и журнал

**Вопрос:** Когда композиция ошибок валидна и что журнал действительно удостоверяет?

Разделены FWER/FDR, countable alpha spending, credit model и pre-work producer witness; reference confseq/bbolt не заменяет producer assumptions.

**Область принятия:** Выбранная математика A принята; остальные A/B/C и применимость libraries/producer не приняты. Прежний R03-17 имеет отдельный NEEDS_REVISION.

[REPORT](03-ADAPTATION/REPORT-010.md) · [SOURCES](03-ADAPTATION/SOURCES-010.md) · [Handoff](03-ADAPTATION/handoffs/R03-010.md) · [Сохранённая рецензия](reviews/NINE-2026-10-27-R03-010-A.md)

Связанные тематические тексты: [adaptation/INTERVALS-AND-POLICY-EVALUATION.md](../adaptation/INTERVALS-AND-POLICY-EVALUATION.md). Связь по вопросу не переносит чужое ACCEPTED на весь пакет.

Разделы REPORT: 1. Что именно проверяется; 2. Словарь: четыре уровня, которые нельзя смешивать; 3. R03-010-A: композиция по фиксированному и счётному семейству; 4. R03-010-B: кредитная модель без ложного «положительного дрейфа»; 5. R03-010-C: минимальный producer-контракт; 6. Reuse только после доказательства; 7. Проверенные источники, русский язык и доказательные ограничения; 8. Конечная будущая программа — всё NOT_RUN; 9. Предлагаемый фрагмент теории (не внесён в общие главы).

<a id="author-4"></a>

## 4. Windows/HY2: UDP, очереди и владельцы ресурсов

**Вопрос:** Какой именно размер/holder ограничен очередью и когда ресурс освобождён?

Scalar Salamander/basicConn путь, scoped sendQueue bound, copies до Add, retiring generations и close/drain/reap; корректирует прежние RAM/PTO/singleflight обобщения.

**Область принятия:** READY_FOR_REVIEW. Нет global RAM/latency/performance proof или принятия всех correction proposals.

[REPORT](04-PERFORMANCE/REPORT-010.md) · [SOURCES](04-PERFORMANCE/SOURCES-010.md) · [Handoff](04-PERFORMANCE/handoffs/R04-010.md)

Связанные тематические тексты: [performance/QUIC-ACCOUNTING-AND-LIMITS.md](../performance/QUIC-ACCOUNTING-AND-LIMITS.md). Связь по вопросу не переносит чужое ACCEPTED на весь пакет.

Разделы REPORT: 1. Вопрос, baseline и область; 2. R04-010-A — фактический scalar Windows путь, не GSO; 3. R04-010-B — одна корректная граница очереди, не глобальная RAM; 4. Адресный пересмотр прежнего R04-008; 5. R04-010-C — singleflight, graceful drain и настоящий Windows owner; 6. R04-009: что не переносится в новую теорию; 7. Reuse, цена и адресаты; 8. Конечная будущая программа — все NOT_RUN; 9. Предлагаемый фрагмент для главы (не интегрирован автором).

<a id="author-5"></a>

## 5. Android: Doze/FGS и цена keepalive

**Вопрос:** Какие power states влияют на доступность, clocks и фактическую работу таймера?

Ordinary Doze/App Standby, foreground UID и LPS различаются; nominal HY2 10s не равен radio/battery rate. OSS PauseManager caller не доказывает остановку HY2 таймера.

**Область принятия:** READY_FOR_REVIEW, поздняя сдача. Source acquisition до cutoff заявлен автором; device/energy программа NOT_RUN.

[REPORT](04-PERFORMANCE/REPORT-011.md) · [SOURCES](04-PERFORMANCE/SOURCES-011.md) · [Handoff](04-PERFORMANCE/handoffs/R04-011.md)

Связанные тематические тексты: [performance/QUIC-ACCOUNTING-AND-LIMITS.md](../performance/QUIC-ACCOUNTING-AND-LIMITS.md) · [android/PLATFORM-AND-UX.md](../android/PLATFORM-AND-UX.md). Связь по вопросу не переносит чужое ACCEPTED на весь пакет.

Разделы REPORT: 1. Пробел и точная область; 2. Фактическая Android-интеграция Snowden; 3. R04-011-A — матрица power-режимов, не единый «фон»; 4. R04-011-B — exact source модель 10s HY2 keepalive; 5. R04-011-C — upstream pause caller is not HY2 timer proof; 6. Энергетический вывод без выдуманных чисел; 7. Reuse / adapt / reject; 8. Конечная device-программа — все случаи NOT_RUN; 9. Выводы для руководителя; 10. Предлагаемая вставка в общую теорию (руководитель решает); 11. Остаток знания.

<a id="author-6"></a>

## 6. Attestation, операция и полноценный status consumer

**Вопрос:** Как сохранить различия статуса, ошибки, давности и разрешения операции?

A/B/C разделяют key attestation, user authentication, Protected Confirmation и verifier assurance. D прослеживает status projection; новое §10 от 28 октября уточняет Gson и пять результатов consumer.

**Область принятия:** A/B/C pending. D исторически PARTIAL_AT_CUTOFF; новая ограниченная задача завершена 28.10 06:56:36Z как READY_FOR_REVIEW, не ACCEPTED.

[REPORT](05-TRUST-SAFETY/REPORT-009.md) · [SOURCES](05-TRUST-SAFETY/SOURCES-009.md) · [Handoff](05-TRUST-SAFETY/handoffs/R05-009.md)

Связанные тематические тексты: [architecture/RELEASE-EVIDENCE-BOUNDARIES.md](../architecture/RELEASE-EVIDENCE-BOUNDARIES.md) · [architecture/SAFETY-AND-TRUST.md](../architecture/SAFETY-AND-TRUST.md). Связь по вопросу не переносит чужое ACCEPTED на весь пакет.

Разделы REPORT: 1. Пробел и проверенный baseline; 2. R05-009-A — четыре разных объекта evidence; 3. R05-009-B — подтверждённый текст, nonce и current authority проверяются отдельно; 4. R05-009-C — ready verifier есть, но generic Success не равен requested assurance; 5. Применение к нашей системе без изменения продукта; 6. Оппонирующая самопроверка; 7. Конечная будущая приёмка — все NOT_RUN; 8. Предлагаемая адресная вставка (не интегрирована); 9. FUP-6-01 / R05-009-D — потери status projection и предел результата; 10. Новая задача 28 октября / завершение source-model остатка FUP-6-01.

<a id="author-7"></a>

## 7. TUF: Progress, retention и действующая authority

**Вопрос:** Когда pointer, retained bytes и accepted metadata — разные события?

Нормативный predecessor и SDK mutation, offline latest, recovery alternatives; D ищет witness для сохранения timestamp. Новый marker/DB не следует автоматически из необходимости различимости.

**Область принятия:** Принят только условный timestamp→returned Progress A; остальные части не приняты. D — PARTIAL_AT_CUTOFF по упаковке.

[REPORT](05-TRUST-SAFETY/REPORT-010.md) · [SOURCES](05-TRUST-SAFETY/SOURCES-010.md) · [Handoff](05-TRUST-SAFETY/handoffs/R05-010.md) · [Сохранённая рецензия](reviews/NINE-2026-10-27-R05-010-A.md)

Связанные тематические тексты: [architecture/TRUST-CONTINUITY-AND-AGREEMENT.md](../architecture/TRUST-CONTINUITY-AND-AGREEMENT.md) · [architecture/RELEASE-EVIDENCE-BOUNDARIES.md](../architecture/RELEASE-EVIDENCE-BOUNDARIES.md). Связь по вопросу не переносит чужое ACCEPTED на весь пакет.

Разделы REPORT: 1. Область, метод и новая дельта; 2. R05-010-A — persistable predecessor не равен указателю TrustedMetadata; 3. R05-010-B — два ограничения знания offline latest; 4. R05-010-C — bounded recovery contract и готовые механизмы; 5. Цена, falsifiers и конечная приёмка — NOT_RUN; 6. Предлагаемая адресная интеграция руководителю; 7. FUP-7-02 / R05-010-D — основание retention конкретного timestamp.

<a id="author-8"></a>

## 8. VPN/MDM, UID capture и принадлежность app-rule

**Вопрос:** Кто управляет захватом/lockdown и что означает найденный socket UID?

A/B/C: Android authority sets, Windows CSP/WFP и наблюдаемость. Новый D: shared UID, видимость PackageManager, SDK sandbox, mapper API и package churn; capture не доказывает exact app-rule match.

**Область принятия:** READY_FOR_REVIEW. D рассматривается отдельно. Существующие owner/parity blockers и default protection не объявлены обойдёнными.

[REPORT](06-RELIABILITY-UX/REPORT-012.md) · [SOURCES](06-RELIABILITY-UX/SOURCES-012.md) · [Handoff](06-RELIABILITY-UX/handoffs/R06-012.md)

Связанные тематические тексты: [android/UID-ROUTING-AND-OWNERSHIP.md](../android/UID-ROUTING-AND-OWNERSHIP.md). Связь по вопросу не переносит чужое ACCEPTED на весь пакет.

Разделы REPORT: 1. Вопрос и метод; 2. Поправки к прежней матрице; 3. R06-012-A — Android: три разных полномочия, не один список исключений; 4. R06-012-B — Windows: профиль MDM, арбитраж WFP и реальные сокеты; 5. R06-012-C — объяснимость ограничивается полномочиями наблюдателя; 6. Reuse, цена и границы; 7. Конечная будущая приёмка — всё NOT_RUN; 8. Предлагаемая вставка и остаток; 9. R06-012-D — UID closure: захват VPN не равен принадлежности app-rule.

<a id="author-9"></a>

## 9. Полезное восстановление задачи и приватный pilot

**Вопрос:** Как задать maturity/horizon/cohort до вычисления recovery rate?

Два estimand, P/U/X/Q/F/C/M исходы, локальная privacy record и будущая программа; OONI reference берётся как boundary, не готовая метрика VPN.

**Область принятия:** A NEEDS_REVISION по horizon/maturity/denominator/protection scope. B/OSS/пилот ожидают отдельного рассмотрения.

[REPORT](06-RELIABILITY-UX/REPORT-013.md) · [SOURCES](06-RELIABILITY-UX/SOURCES-013.md) · [Handoff](06-RELIABILITY-UX/handoffs/R06-013.md) · [Сохранённая рецензия](reviews/NINE-2026-10-27-R06-013-A.md)

Связанные тематические тексты: [adaptation/INTERVALS-AND-POLICY-EVALUATION.md](../adaptation/INTERVALS-AND-POLICY-EVALUATION.md) · [observability/PROBE-PROOF-BOUNDARIES.md](../observability/PROBE-PROOF-BOUNDARIES.md). Связь по вопросу не переносит чужое ACCEPTED на весь пакет.

Разделы REPORT: 1. Пробел и проверка baseline; 2. R06-013-A — два estimand вместо одного двусмысленного recovery rate; 3. R06-013-B — минимальный privacy-preserving local record; 4. Будущая локальная программа — дизайн, не запуск; 5. OSS reference: OONI Probe CLI v3.31.0 — adapt boundary, не код; 6. Цена, оппонирование и предел знания; 7. Предлагаемая вставка руководителю.

## Два времени и два вида проверки

PARTIAL_AT_CUTOFF 27 октября сохраняется даже при отдельной сдаче новой задачи 28 октября. DR27/Obsidian review использовал public source и документационный срез; NINE содержит свои source/caller trace и prerequisites. Одинаковая тема не делает pins, producer inputs и reviewed consumers одинаковыми. Новая общая навигация не является повторным независимым source review или product/runtime acceptance.
