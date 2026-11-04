# Каталог выводов и очередь научной рецензии

Навигационный срез на **28 октября 2026**. [Полный реестр решений](REVIEW-REGISTER.md) задаёт принятую область; эта очередь показывает пересмотр, принятые claims и открытые вопросы, не вынося новых вердиктов.

[Каталог аргументов](CATALOG.md) · [Рецензии и поправки](REVIEW-REGISTER.md) · [Границы доказательств](LIMITATIONS.md)

## Требует пересмотра до использования

| Claim / пакет | Решение | Принятая область или причина пересмотра | Граница |
|---|---|---|---|
| [R03-17](reviews/NINE-2026-10-27-R03-17-ERRATUM.md) / [R03-009](03-ADAPTATION/handoffs/R03-009.md) | **NEEDS_REVISION**<br>Самокоррекция, не независимая внешняя рецензия | Прежняя product-formula при неизвестной зависимости и тезис о бесконечном ожидании требуют пересмотра. | Без независимости error events применим union bound, а не 1−(1−α)^W. +1 credit за успех не доказывает бесконечное ожидание. |
| [R06-013-A](reviews/NINE-2026-10-27-R06-013-A.md) / [R06-013](06-RELIABILITY-UX/handoffs/R06-013.md) | **NEEDS_REVISION** | Пересмотр maturity/horizon/denominator и protection scope; раннее «не завершено» не terminal failure к D. | Условная алгебра не исправляет категории/cohort. B/OSS/пилот отдельно не приняты. |

Авторские исходники сохраняют историю. При использовании R03-009 и R06-013 сначала читать соответствующую поправку/рецензию; прежнее READY не отменяет решения. 

## Можно читать как отдельно принятые выводы

| Claim / пакет | Решение | Принятая область или причина пересмотра | Граница |
|---|---|---|---|
| [R04-06](reviews/HB01-R04-06.md) / [R04-002](04-PERFORMANCE/handoffs/R04-002.md) | **ACCEPTED** | Source-факты и условный interleaving fragment lifetime/cache eviction HY2. | Не воспроизведение race/crash, частота, severity, native-эффект или исправление. |
| [R02-10](reviews/2H-HB01-R02-10.md) / [R02-004](02-CENSORSHIP/handoffs/R02-004.md) | **ACCEPTED** | Рассмотренная TLS-less VLESS→ShadowTLS3 композиция не получает data confidentiality от donor TLS/HMAC. | Не deployed profile, реальная утечка, расшифровка inner HTTPS или исправление. |
| [R04-10](reviews/2H-HB02-R04-10.md) / [R04-004](04-PERFORMANCE/handoffs/R04-004.md) | **ACCEPTED** | Method-set mismatch выбирает base adapter; рассмотренный normal batch dispatch не обновляет ackRate. | Не compile/native-проверка, реальная скорость, pacing/SRTT или отсутствие иных feedback paths. |
| [R02-15](reviews/4H-HB01-R02-15.md) / [R02-007](02-CENSORSHIP/handoffs/R02-007.md) | **ACCEPTED** | Фиксированный HRR Random в one-shot seed: условное повторение first outer MAC predicate. | Не гарантия relay delivery, эффект на inner/target/application, exploit или severity. |
| [R02-014-A](reviews/NINE-2026-10-27-R02-014-A.md) / [R02-014](02-CENSORSHIP/handoffs/R02-014.md) | **ACCEPTED** | Область числовых источников: у 95/99,5 не идентифицирована методика; 469−258 не доказывает 211 новых уникальных блокировок. | Чрезмерные числовые обобщения — NEEDS_REVISION в этой области. Не измерение сети, опровержение существования любых испытаний или приёмка B/C/D. |
| [R03-010-A](reviews/NINE-2026-10-27-R03-010-A.md) / [R03-010](03-ADAPTATION/handoffs/R03-010.md) | **ACCEPTED** | Выбранная математическая часть: union/tower и счётная alpha-композиция при A1/A2, post-selection контрпример. | Не весь A, B/C, numerical/library/producer применимость, FDR или текущая сеть. |
| [R05-010-A](reviews/NINE-2026-10-27-R05-010-A.md) / [R05-010](05-TRUST-SAFETY/handoffs/R05-010.md) | **ACCEPTED** | Только условный timestamp→returned Progress trace и retention call перед возвратом исходной ошибки. | Не успешная запись на диск, artifact proof/update authority, lockout/bypass, severity или остальные A/B/C/D. |

## Историческое CONFIRMED и отдельное public-source ACCEPTED

| Claim / пакет | Решение | Принятая область или причина пересмотра | Граница |
|---|---|---|---|
| [R02-01](reviews/EVE-2026-10-27-R02-01.md) / [R02-001](02-CENSORSHIP/handoffs/R02-001.md) | **CONFIRMED** | Vision присутствует в dependency closure; прежний отрицательный grep только по sing-box был слишком узким. | В раннем документе отдельное ACCEPTED не записано. Позднейшая [public-source рецензия](reviews/2026-10-27-R02-01-VISION.md) сохранена отдельно. Не runtime activation или готовность обеих сторон. |

## Открытая очередь по областям

Это список оставшихся научных вопросов по сохранённым итогам, **не новые задания, сроки или product backlog**. Полный охват старых и новых пакетов — в [каталоге](CATALOG.md).

| Область | Где читать | Что остаётся отдельно от принятого |
|---|---|---|
| ECH/SVCB, bootstrap и фильтрация | [R02-013](02-CENSORSHIP/handoffs/R02-013.md) | A/B/C ожидают рецензии; D PARTIAL_AT_CUTOFF, caller/endpoint/source-freshness границы |
| РФ: качество наблюдений и OONI | [R02-014](02-CENSORSHIP/handoffs/R02-014.md) | B/C/D не покрыты числовым решением A; версия/текущий network outcome неизвестны |
| Последовательная статистика и журнал | [R03-010](03-ADAPTATION/handoffs/R03-010.md) | Producer/numerical/library применимость, B/C и непринятые части A |
| Windows UDP, очереди и RAM/PTO | [R04-010](04-PERFORMANCE/handoffs/R04-010.md) | Целостная resource/OS композиция, независимая source-рецензия; реальные эффекты не измерены |
| Android Doze/FGS и энергия | [R04-011](04-PERFORMANCE/handoffs/R04-011.md) | Поздняя сдача; source-модель не device/energy proof |
| Attestation и подтверждение операции | [R05-009](05-TRUST-SAFETY/handoffs/R05-009.md) | A/B/C pending; исторический D — PARTIAL_AT_CUTOFF, позднее дополнение — READY_FOR_REVIEW |
| Update-progress и durable authority | [R05-010](05-TRUST-SAFETY/handoffs/R05-010.md) | Успешная запись/полный consumer contract не доказаны; D PARTIAL_AT_CUTOFF |
| VPN/MDM и platform authority | [R06-012](06-RELIABILITY-UX/handoffs/R06-012.md) | Source/API и actual artifacts/OEM/OS execution различаются; позднее дополнение остаётся pending и рассматривается отдельно |
| Useful-task recovery и privacy | [R06-013](06-RELIABILITY-UX/handoffs/R06-013.md) | A NEEDS_REVISION; B/OSS/пилот не приняты |
| Прежние нерассмотренные claims | [R02](CATALOG.md#r02), [R03](CATALOG.md#r03), [R04](CATALOG.md#r04), [R05](CATALOG.md#r05), [R06](CATALOG.md#r06) | Не исчезают из очереди при появлении нового цикла; scoped принятие не распространяется на соседние claims |

## Правила учёта

1. Авторский READY_FOR_REVIEW — готовность передачи, отдельная рецензия — научное решение.
2. Частичное ACCEPTED относится к названному claim, версии и предпосылкам. Непокрытые части сохраняют свой статус.
3. PARTIAL_AT_CUTOFF и поздняя упаковка не превращаются в полное своевременное завершение из-за наличия файлов.
4. Группировка пакетов и тематических глав не меняет научный статус и не переносит решения в implemented/native/live.

## Пробелы, добавленные DR27 и поздними продолжениями

- R04-14: compatibility/semantic matrix; DR27-10: reset overlap/reuse и минимальность — [смешанные решения](reviews/2026-10-28-DEEP-REVIEW.md).
- DR27-16: partial current-RF search и покрытие корпуса — [методическая заметка](../methodology/MEASUREMENT-CORPUS-AND-COVERAGE.md).
- R05-009-D: позднее source/model дополнение ждёт независимого review; прежняя область PARTIAL_AT_CUTOFF сохраняется.
- R06-012-D: UID/sandbox/app-rule counterexample ждёт review; не доказан обход работающего BLOCK.
- Взаимосвязи и scope поздних тематических пакетов — [карта материалов](NINE-KNOWLEDGE.md). 
