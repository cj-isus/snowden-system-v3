# Каталог исследовательских пакетов

Пакеты сгруппированы по направлениям R02–R06. Каждая строка связывает краткий вывод с точными REPORT/SOURCES и применимым решением; новый раздел внутри накопительного REPORT не создаёт пакет и не меняет научный статус автоматически.

[Общая карта](../KNOWLEDGE-MAP.md) · [ECH, измерения и доверие: содержание](NINE-KNOWLEDGE.md) · [Полный реестр решений](REVIEW-REGISTER.md) · [JSON](CATALOG.json)

## Как читать

Краткий вывод → точные аргумент и источники → применимая рецензия и её область. Накопительный REPORT может содержать несколько пакетов; Решение относится к названному вопросу и предпосылкам; оно не принимает весь пакет и не проверяет исполнение продукта. [Исходная карта разделов](../CONTENTS.json) сохранена отдельно.

Хеши `sourceObservations` в JSON относятся к датированным исходным captures,
а не к текущей редакции читательских страниц. Исторические счётчики пакетов
не заменяют рецензию; поздние циклы перечислены отдельно.

<a id="r02"></a>

## R02 — Блокировки и противник

| Пакет и вопрос | Отчёт / источники | Решения и продолжения |
|---|---|---|
| [R02-001](02-CENSORSHIP/handoffs/R02-001.md) — контрмеры и границы применимости | [Аргумент](02-CENSORSHIP/REPORT.md) · [Источники](02-CENSORSHIP/SOURCES.md) | **Public-source ACCEPTED; EVE CONFIRMED отдельно**<br>[R02-01: CONFIRMED](reviews/EVE-2026-10-27-R02-01.md); [Дополнительная рецензия](reviews/2026-10-27-R02-01-VISION.md) |
| [R02-002](02-CENSORSHIP/handoffs/R02-002.md) — совместное наблюдение, префикс и условия формирования трафика | [Аргумент](02-CENSORSHIP/REPORT-002.md) · [Источники](02-CENSORSHIP/SOURCES-002.md) | **Ожидает рецензии** |
| [R02-003](02-CENSORSHIP/handoffs/R02-003.md) — готовые машины и непереносимые гарантии | [Аргумент](02-CENSORSHIP/REPORT-003.md) · [Источники](02-CENSORSHIP/SOURCES-003.md) | **Ожидает рецензии** |
| [R02-004](02-CENSORSHIP/handoffs/R02-004.md) — VLESS → ShadowTLS без шифрования полезных данных | [Аргумент](02-CENSORSHIP/REPORT-004.md) · [Источники](02-CENSORSHIP/SOURCES-004.md) | **Частично принято**<br>[R02-10: ACCEPTED](reviews/2H-HB01-R02-10.md) |
| [R02-005](02-CENSORSHIP/handoffs/R02-005.md) — свойства шифрующей замены по участкам пути | [Аргумент](02-CENSORSHIP/REPORT-005.md) · [Источники](02-CENSORSHIP/SOURCES-005.md) | **Есть дополнительное решение в указанной области**<br>[Дополнительная рецензия](reviews/2026-10-28-DEEP-REVIEW.md) |
| [R02-006](02-CENSORSHIP/handoffs/R02-006.md) — варианты ввода-вывода меняют семантику протокола | [Аргумент](02-CENSORSHIP/REPORT-006.md) · [Источники](02-CENSORSHIP/SOURCES-006.md) | **Ожидает рецензии** |
| [R02-007](02-CENSORSHIP/handoffs/R02-007.md) — свежесть HRR и нижняя граница версии TLS | [Аргумент](02-CENSORSHIP/REPORT-007.md) · [Источники](02-CENSORSHIP/SOURCES-007.md) | **Есть дополнительное решение в указанной области**<br>[R02-15: ACCEPTED](reviews/4H-HB01-R02-15.md); [Дополнительная рецензия](reviews/2026-10-28-DEEP-REVIEW.md) |
| [R02-008](02-CENSORSHIP/handoffs/R02-008.md) — граница первой записи и защита от активных проб | [Аргумент](02-CENSORSHIP/REPORT-008.md) · [Источники](02-CENSORSHIP/SOURCES-008.md) | **Ожидает рецензии** |
| [R02-009](02-CENSORSHIP/handoffs/R02-009.md) — проверки свежести и область защиты от повторов | [Аргумент](02-CENSORSHIP/REPORT-009.md) · [Источники](02-CENSORSHIP/SOURCES-009.md) | **Ожидает рецензии** |
| [R02-010](02-CENSORSHIP/handoffs/R02-010.md) — доступность готовой привязки к сессии | [Аргумент](02-CENSORSHIP/REPORT-010.md) · [Источники](02-CENSORSHIP/SOURCES-010.md) | **Есть дополнительное решение в указанной области**<br>[Дополнительная рецензия](reviews/2026-10-28-DEEP-REVIEW.md) |
| [R02-011](02-CENSORSHIP/handoffs/R02-011.md) — ранние данные HY2 и потеря полей при адаптации ShadowTLS | [Аргумент](02-CENSORSHIP/REPORT-011.md) · [Источники](02-CENSORSHIP/SOURCES-011.md) | **Ожидает рецензии** |
| [R02-012](02-CENSORSHIP/handoffs/R02-012.md) — домены отказа при ограничениях IP, ASN и списках разрешённого | [Аргумент](02-CENSORSHIP/REPORT-012.md) · [Источники](02-CENSORSHIP/SOURCES-012.md) | **Количественные расширения ограничены R02-014-A; прочее не принято целиком** |
| [R02-013](02-CENSORSHIP/handoffs/R02-013.md) — ECH/SVCB: спецификация, начальное разрешение и область фильтрации | [Аргумент](02-CENSORSHIP/REPORT-013.md) · [Источники](02-CENSORSHIP/SOURCES-013.md) | **Ожидает рецензии**<br>A/B/C ожидают рецензии; D — PARTIAL_AT_CUTOFF. |
| [R02-014](02-CENSORSHIP/handoffs/R02-014.md) — качество доказательств сетевых ограничений РФ | [Аргумент](02-CENSORSHIP/REPORT-014.md) · [Источники](02-CENSORSHIP/SOURCES-014.md) | **Частично принято**<br>[R02-014-A: ACCEPTED](reviews/NINE-2026-10-27-R02-014-A.md)<br>A принят в числовой source-области; B/C/D ожидают рецензии. |

<a id="r03"></a>

## R03 — Адаптация и статистика

| Пакет и вопрос | Отчёт / источники | Решения и продолжения |
|---|---|---|
| [R03-001](03-ADAPTATION/handoffs/R03-001.md) — наблюдения и допустимость адаптивного решения | [Аргумент](03-ADAPTATION/REPORT.md) · [Источники](03-ADAPTATION/SOURCES.md) | **Ожидает рецензии** |
| [R03-002](03-ADAPTATION/handoffs/R03-002.md) — время доказательства и выполнимость измерений | [Аргумент](03-ADAPTATION/REPORT.md) · [Источники](03-ADAPTATION/SOURCES.md) | **Ожидает рецензии** |
| [R03-003](03-ADAPTATION/handoffs/R03-003.md) — минимально полезные измерения для конкретного решения | [Аргумент](03-ADAPTATION/REPORT.md) · [Источники](03-ADAPTATION/SOURCES.md) | **Ожидает рецензии** |
| [R03-004](03-ADAPTATION/handoffs/R03-004.md) — идентифицируемое сравнение адаптивных политик | [Аргумент](03-ADAPTATION/REPORT.md) · [Источники](03-ADAPTATION/SOURCES.md) | **Есть дополнительное решение в указанной области**<br>[Дополнительная рецензия](reviews/2026-10-28-DEEP-REVIEW.md) |
| [R03-005](03-ADAPTATION/handoffs/R03-005.md) — задержанные исходы и статистические сбросы | [Аргумент](03-ADAPTATION/REPORT.md) · [Источники](03-ADAPTATION/SOURCES.md) | **Ожидает рецензии** |
| [R03-006](03-ADAPTATION/handoffs/R03-006.md) — необязательное решение при дрейфе и дорогом переходе | [Аргумент](03-ADAPTATION/REPORT.md) · [Источники](03-ADAPTATION/SOURCES.md) | **Ожидает рецензии** |
| [R03-007](03-ADAPTATION/handoffs/R03-007.md) — риск полной попытки и остаточные ресурсы | [Аргумент](03-ADAPTATION/REPORT.md) · [Источники](03-ADAPTATION/SOURCES.md) | **Ожидает рецензии** |
| [R03-008](03-ADAPTATION/handoffs/R03-008.md) — наблюдения с пропусками и ложными результатами | [Аргумент](03-ADAPTATION/REPORT.md) · [Источники](03-ADAPTATION/SOURCES.md) | **Есть дополнительное решение в указанной области**<br>[Дополнительная рецензия](reviews/2026-10-28-DEEP-REVIEW.md) |
| [R03-009](03-ADAPTATION/handoffs/R03-009.md) — регистрация попыток и бюджет исследования вариантов | [Аргумент](03-ADAPTATION/REPORT.md) · [Источники](03-ADAPTATION/SOURCES.md) | **Есть NEEDS_REVISION**<br>[R03-17: NEEDS_REVISION](reviews/NINE-2026-10-27-R03-17-ERRATUM.md) |
| [R03-010](03-ADAPTATION/handoffs/R03-010.md) — последовательная статистика и журнал попыток | [Аргумент](03-ADAPTATION/REPORT-010.md) · [Источники](03-ADAPTATION/SOURCES-010.md) | **Частично принято**<br>[R03-010-A: ACCEPTED](reviews/NINE-2026-10-27-R03-010-A.md)<br>Принята выбранная математическая часть A; остальные A/B/C ожидают рецензии. |

<a id="r04"></a>

## R04 — Производительность, ресурсы и энергия

| Пакет и вопрос | Отчёт / источники | Решения и продолжения |
|---|---|---|
| [R04-001](04-PERFORMANCE/handoffs/R04-001.md) — HY2: задержки и ресурсные ограничения | [Аргумент](04-PERFORMANCE/REPORT.md) · [Источники](04-PERFORMANCE/SOURCES.md) | **Ожидает рецензии** |
| [R04-002](04-PERFORMANCE/handoffs/R04-002.md) — конкуренция TCP/UDP и ограничение перегрузки | [Аргумент](04-PERFORMANCE/REPORT.md) · [Источники](04-PERFORMANCE/SOURCES.md) | **Есть дополнительное решение в указанной области**<br>[R04-06: ACCEPTED](reviews/HB01-R04-06.md); [Дополнительная рецензия](reviews/2026-10-28-DEEP-REVIEW.md) |
| [R04-003](04-PERFORMANCE/handoffs/R04-003.md) — часы сна, поддержание соединения и энергия | [Аргумент](04-PERFORMANCE/REPORT.md) · [Источники](04-PERFORMANCE/SOURCES.md) | **Есть дополнительное решение в указанной области**<br>[Дополнительная рецензия](reviews/2026-10-28-DEEP-REVIEW.md) |
| [R04-004](04-PERFORMANCE/handoffs/R04-004.md) — часы HY2/BBR и обратная связь Brutal | [Аргумент](04-PERFORMANCE/REPORT.md) · [Источники](04-PERFORMANCE/SOURCES.md) | **Частично принято**<br>[R04-10: ACCEPTED](reviews/2H-HB02-R04-10.md) |
| [R04-005](04-PERFORMANCE/handoffs/R04-005.md) — очереди, временные метки и обратная связь PMTU | [Аргумент](04-PERFORMANCE/REPORT.md) · [Источники](04-PERFORMANCE/SOURCES.md) | **Ожидает рецензии** |
| [R04-006](04-PERFORMANCE/handoffs/R04-006.md) — BBR: учёт flightBefore/flightAfter | [Аргумент](04-PERFORMANCE/REPORT.md) · [Источники](04-PERFORMANCE/SOURCES.md) | **Есть дополнительное решение в указанной области**<br>[Дополнительная рецензия](reviews/2026-10-28-DEEP-REVIEW.md) |
| [R04-007](04-PERFORMANCE/handoffs/R04-007.md) — кандидат исправления Brutal и границы переноса обратных вызовов | [Аргумент](04-PERFORMANCE/REPORT.md) · [Источники](04-PERFORMANCE/SOURCES.md) | **R04-14 NEEDS_REVISION; см. DR27**<br>[Дополнительная рецензия](reviews/2026-10-28-DEEP-REVIEW.md) |
| [R04-008](04-PERFORMANCE/handoffs/R04-008.md) — удержание истории отправок при плохой связи | [Аргумент](04-PERFORMANCE/REPORT.md) · [Источники](04-PERFORMANCE/SOURCES.md) | **Общий предел 20 MB ∧ 30 s не обоснован; см. адресное ограничение** |
| [R04-009](04-PERFORMANCE/handoffs/R04-009.md) — заявленная трилемма CCA и допуск при перегрузке | [Аргумент](04-PERFORMANCE/REPORT.md) · [Источники](04-PERFORMANCE/SOURCES.md) | **Трилемма и общий вывод о допуске и RAM не установлены** |
| [R04-010](04-PERFORMANCE/handoffs/R04-010.md) — Windows/HY2: пакетный путь и ограничения ресурсов | [Аргумент](04-PERFORMANCE/REPORT-010.md) · [Источники](04-PERFORMANCE/SOURCES-010.md) | **Ожидает рецензии**<br>Аргумент по исходникам и модели ожидает независимой рецензии. |
| [R04-011](04-PERFORMANCE/handoffs/R04-011.md) — Android: Doze/FGS и энергия QUIC | [Аргумент](04-PERFORMANCE/REPORT-011.md) · [Источники](04-PERFORMANCE/SOURCES-011.md) | **Ожидает рецензии**<br>Сохраняется ограничение поздней сдачи: время получения источников независимо не подтверждено. |

<a id="r05"></a>

## R05 — Доверие и безопасность

| Пакет и вопрос | Отчёт / источники | Решения и продолжения |
|---|---|---|
| [R05-001](05-TRUST-SAFETY/handoffs/R05-001.md) — доверие, история и право на применение | [Аргумент](05-TRUST-SAFETY/REPORT.md) · [Источники](05-TRUST-SAFETY/SOURCES.md) | **Есть дополнительное решение в указанной области**<br>[Дополнительная рецензия](reviews/2026-10-27-R05-001-PROGRESS.md) |
| [R05-002](05-TRUST-SAFETY/handoffs/R05-002.md) — минимальный устойчиво сохраняемый контекст доверия | [Аргумент](05-TRUST-SAFETY/REPORT.md) · [Источники](05-TRUST-SAFETY/SOURCES.md) | **Частное решение и незакрытый пересмотр**<br>[Дополнительная рецензия](reviews/2026-10-28-DEEP-REVIEW.md) |
| [R05-003](05-TRUST-SAFETY/handoffs/R05-003.md) — происхождение, полномочия и принятие результата различаются | [Аргумент](05-TRUST-SAFETY/REPORT.md) · [Источники](05-TRUST-SAFETY/SOURCES.md) | **Ожидает рецензии** |
| [R05-004](05-TRUST-SAFETY/handoffs/R05-004.md) — установка и исполнение требуют разных доказательств | [Аргумент](05-TRUST-SAFETY/REPORT.md) · [Источники](05-TRUST-SAFETY/SOURCES.md) · [Контракт](05-TRUST-SAFETY/R05-004.md) | **Ожидает рецензии** |
| [R05-005](05-TRUST-SAFETY/handoffs/R05-005.md) — офлайн-проверка не подтверждает актуальные глобальные полномочия | [Аргумент](05-TRUST-SAFETY/REPORT.md) · [Источники](05-TRUST-SAFETY/SOURCES.md) · [Контракт](05-TRUST-SAFETY/R05-005.md) | **Ожидает рецензии** |
| [R05-006](05-TRUST-SAFETY/handoffs/R05-006.md) — готовый поиск разрешения и его ограничения | [Аргумент](05-TRUST-SAFETY/REPORT.md) · [Источники](05-TRUST-SAFETY/SOURCES.md) · [Контракт](05-TRUST-SAFETY/R05-006.md) | **Ожидает рецензии** |
| [R05-007](05-TRUST-SAFETY/handoffs/R05-007.md) — начальное доверие, переход и согласование целевого файла | [Аргумент](05-TRUST-SAFETY/REPORT.md) · [Источники](05-TRUST-SAFETY/SOURCES.md) · [Контракт](05-TRUST-SAFETY/R05-007.md) | **Есть дополнительное решение в указанной области**<br>[Дополнительная рецензия](reviews/2026-10-28-DEEP-REVIEW.md) |
| [R05-008](05-TRUST-SAFETY/handoffs/R05-008.md) — системные средства резервного копирования и восстановления | [Аргумент](05-TRUST-SAFETY/REPORT.md) · [Источники](05-TRUST-SAFETY/SOURCES.md) · [Контракт](05-TRUST-SAFETY/R05-008.md) | **Ожидает рецензии** |
| [R05-009](05-TRUST-SAFETY/handoffs/R05-009.md) — Key Attestation, операция и подтверждение | [Аргумент](05-TRUST-SAFETY/REPORT-009.md) · [Источники](05-TRUST-SAFETY/SOURCES-009.md) | **Ожидает рецензии**<br>A/B/C pending; исторический D — PARTIAL_AT_CUTOFF, позднее дополнение — READY_FOR_REVIEW. |
| [R05-010](05-TRUST-SAFETY/handoffs/R05-010.md) — TUF: timestamp, Progress и сохранение истории | [Аргумент](05-TRUST-SAFETY/REPORT-010.md) · [Источники](05-TRUST-SAFETY/SOURCES-010.md) | **Частично принято**<br>[R05-010-A: ACCEPTED](reviews/NINE-2026-10-27-R05-010-A.md)<br>Только timestamp trace A принят; остальные части ожидают рецензии, D — PARTIAL_AT_CUTOFF. |

<a id="r06"></a>

## R06 — Надёжность и повседневный UX

| Пакет и вопрос | Отчёт / источники | Решения и продолжения |
|---|---|---|
| [R06-001](06-RELIABILITY-UX/handoffs/R06-001.md) — первый цикл: надёжность сквозной услуги | [Аргумент](06-RELIABILITY-UX/REPORT.md) · [Источники](06-RELIABILITY-UX/SOURCES.md) | **Ожидает рецензии** |
| [R06-002](06-RELIABILITY-UX/handoffs/R06-002.md) — восстановление во времени при составном отказе | [Аргумент](06-RELIABILITY-UX/REPORT.md) · [Источники](06-RELIABILITY-UX/SOURCES.md) | **Ожидает рецензии** |
| [R06-003](06-RELIABILITY-UX/handoffs/R06-003.md) — что сохраняется при переключении рабочего соединения | [Аргумент](06-RELIABILITY-UX/REPORT.md) · [Источники](06-RELIABILITY-UX/SOURCES.md) | **Ожидает рецензии** |
| [R06-004](06-RELIABILITY-UX/handoffs/R06-004.md) — границы причинного диагноза по приватной истории | [Аргумент](06-RELIABILITY-UX/REPORT.md) · [Источники](06-RELIABILITY-UX/SOURCES.md) | **Ожидает рецензии** |
| [R06-005](06-RELIABILITY-UX/handoffs/R06-005.md) — восстановление, диагностика и рабочие сценарии | [Аргумент](06-RELIABILITY-UX/REPORT.md) · [Источники](06-RELIABILITY-UX/SOURCES.md) | **Сводка выводов; не отдельное решение** |
| [R06-006](06-RELIABILITY-UX/handoffs/R06-006.md) — прокси с аутентификацией, строгая защита и важная сессия | [Аргумент](06-RELIABILITY-UX/REPORT.md) · [Источники](06-RELIABILITY-UX/SOURCES.md) | **Есть дополнительное решение в указанной области**<br>[Дополнительная рецензия](reviews/2026-10-28-DEEP-REVIEW.md) |
| [R06-007](06-RELIABILITY-UX/handoffs/R06-007.md) — IPv6-only/NAT64: начальное соединение и достижимое восстановление | [Аргумент](06-RELIABILITY-UX/REPORT.md) · [Источники](06-RELIABILITY-UX/SOURCES.md) | **Есть дополнительное решение в указанной области**<br>[Дополнительная рецензия](reviews/2026-10-28-DEEP-REVIEW.md) |
| [R06-008](06-RELIABILITY-UX/handoffs/R06-008.md) — прокси и NAT64: где фактически устанавливается соединение | [Аргумент](06-RELIABILITY-UX/REPORT.md) · [Источники](06-RELIABILITY-UX/SOURCES.md) | **Есть дополнительное решение в указанной области**<br>[Дополнительная рецензия](reviews/2026-10-28-DEEP-REVIEW.md) |
| [R06-009](06-RELIABILITY-UX/handoffs/R06-009.md) — корпоративные сети, DNS и восстановление | [Аргумент](06-RELIABILITY-UX/REPORT.md) · [Источники](06-RELIABILITY-UX/SOURCES.md) | **Сводка выводов; не отдельное решение** |
| [R06-010](06-RELIABILITY-UX/handoffs/R06-010.md) — рабочие сети: split DNS/NRPT/PAC, LAN и Android | [Аргумент](06-RELIABILITY-UX/REPORT.md) · [Источники](06-RELIABILITY-UX/SOURCES.md) | **Ожидает рецензии** |
| [R06-011](06-RELIABILITY-UX/handoffs/R06-011.md) — измерение восстановления полезной задачи | [Аргумент](06-RELIABILITY-UX/REPORT.md) · [Источники](06-RELIABILITY-UX/SOURCES.md) | **Ожидает рецензии** |
| [R06-012](06-RELIABILITY-UX/handoffs/R06-012.md) — VPN/MDM и полномочия | [Аргумент](06-RELIABILITY-UX/REPORT-012.md) · [Источники](06-RELIABILITY-UX/SOURCES-012.md) | **Ожидает рецензии**<br>A/B/C pending; позднее дополнение D — READY_FOR_REVIEW. |
| [R06-013](06-RELIABILITY-UX/handoffs/R06-013.md) — полезное восстановление и локальная оценка с сохранением приватности | [Аргумент](06-RELIABILITY-UX/REPORT-013.md) · [Источники](06-RELIABILITY-UX/SOURCES-013.md) | **Есть NEEDS_REVISION**<br>[R06-013-A: NEEDS_REVISION](reviews/NINE-2026-10-27-R06-013-A.md)<br>A — NEEDS_REVISION; B/OSS/пилот ожидают рецензии. |
