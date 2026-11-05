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

Принятые области собраны один раз в [основном реестре](REVIEW-REGISTER.md#принятые-ранние-выводы).
Эта очередь хранит только ограничения применения и непокрытые части;
полный текст рецензии остаётся основанием каждого решения.

## Историческое CONFIRMED и отдельное public-source ACCEPTED

Обе области R02-01 сохранены отдельно в
[реестре](REVIEW-REGISTER.md#vision-tuf-и-углублённые-решения): ранний CONFIRMED
не переименован в ACCEPTED и не заменяет более позднюю публичную рецензию.

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
