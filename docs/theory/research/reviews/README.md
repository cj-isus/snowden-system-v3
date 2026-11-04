# Реестр сохранённых рецензий и поправок

Историческая группа ранних/NINE решений: **10 документов** — 7 ограниченных ACCEPTED, 2 NEEDS_REVISION, 1 CONFIRMED без отдельного формального ACCEPTED. Теперь это часть [объединённого реестра 13 документов](../REVIEW-REGISTER.md). Ниже сохранены прежние решения; отдельное public-source ACCEPTED R02-01 и DR27 приведены в новом разделе.

[Каталог пакетов](../CATALOG.md) · [Очередь](../REVIEW-QUEUE.md) · [Границы доказательств](../LIMITATIONS.md)

## Сначала исправления и ограниченные подтверждения

| Claim / пакет | Решение | Принятая область или причина пересмотра | Граница |
|---|---|---|---|
| [R02-01](EVE-2026-10-27-R02-01.md) / [R02-001](../02-CENSORSHIP/handoffs/R02-001.md) | **CONFIRMED** | Vision присутствует в dependency closure; прежний отрицательный grep только по sing-box был слишком узким. | В этом раннем документе отдельное ACCEPTED не записано; позднее public-source решение сохранено отдельно ниже. Не runtime activation или готовность обеих сторон. |
| [R03-17](NINE-2026-10-27-R03-17-ERRATUM.md) / [R03-009](../03-ADAPTATION/handoffs/R03-009.md) | **NEEDS_REVISION**<br>Самокоррекция, не независимая внешняя рецензия | Прежняя product-formula при неизвестной зависимости и тезис о бесконечном ожидании требуют пересмотра. | Без независимости error events применим union bound, а не 1−(1−α)^W. +1 credit за успех не доказывает бесконечное ожидание. |
| [R06-013-A](NINE-2026-10-27-R06-013-A.md) / [R06-013](../06-RELIABILITY-UX/handoffs/R06-013.md) | **NEEDS_REVISION** | Пересмотр maturity/horizon/denominator и protection scope; раннее «не завершено» не terminal failure к D. | Условная алгебра не исправляет категории/cohort. B/OSS/пилот отдельно не приняты. |

## Принятые области

| Claim / пакет | Решение | Принятая область или причина пересмотра | Граница |
|---|---|---|---|
| [R04-06](HB01-R04-06.md) / [R04-002](../04-PERFORMANCE/handoffs/R04-002.md) | **ACCEPTED** | Source-факты и условный interleaving fragment lifetime/cache eviction HY2. | Не воспроизведение race/crash, частота, severity, native-эффект или исправление. |
| [R02-10](2H-HB01-R02-10.md) / [R02-004](../02-CENSORSHIP/handoffs/R02-004.md) | **ACCEPTED** | Рассмотренная TLS-less VLESS→ShadowTLS3 композиция не получает data confidentiality от donor TLS/HMAC. | Не deployed profile, реальная утечка, расшифровка inner HTTPS или исправление. |
| [R04-10](2H-HB02-R04-10.md) / [R04-004](../04-PERFORMANCE/handoffs/R04-004.md) | **ACCEPTED** | Method-set mismatch выбирает base adapter; рассмотренный normal batch dispatch не обновляет ackRate. | Не compile/native-проверка, реальная скорость, pacing/SRTT или отсутствие иных feedback paths. |
| [R02-15](4H-HB01-R02-15.md) / [R02-007](../02-CENSORSHIP/handoffs/R02-007.md) | **ACCEPTED** | Фиксированный HRR Random в one-shot seed: условное повторение first outer MAC predicate. | Не гарантия relay delivery, эффект на inner/target/application, exploit или severity. |
| [R02-014-A](NINE-2026-10-27-R02-014-A.md) / [R02-014](../02-CENSORSHIP/handoffs/R02-014.md) | **ACCEPTED** | Область числовых источников: у 95/99,5 не идентифицирована методика; 469−258 не доказывает 211 новых уникальных блокировок. | Чрезмерные числовые обобщения — NEEDS_REVISION в этой области. Не измерение сети, опровержение существования любых испытаний или приёмка B/C/D. |
| [R03-010-A](NINE-2026-10-27-R03-010-A.md) / [R03-010](../03-ADAPTATION/handoffs/R03-010.md) | **ACCEPTED** | Выбранная математическая часть: union/tower и счётная alpha-композиция при A1/A2, post-selection контрпример. | Не весь A, B/C, numerical/library/producer применимость, FDR или текущая сеть. |
| [R05-010-A](NINE-2026-10-27-R05-010-A.md) / [R05-010](../05-TRUST-SAFETY/handoffs/R05-010.md) | **ACCEPTED** | Только условный timestamp→returned Progress trace и retention call перед возвратом исходной ошибки. | Не успешная запись на диск, artifact proof/update authority, lockout/bypass, severity или остальные A/B/C/D. |

Исходные рецензии сохранены без изменений. Самокоррекция R03-17 не становится независимой экспертизой; CONFIRMED по R02-01 не переименован в формальное ACCEPTED. Ни одно решение не подтверждает весь REPORT, исправление продукта или live VPN-путь.

## Дополнительно перенесено из Obsidian

- [Vision и REALITY keygen: public upstream closure](2026-10-27-R02-01-VISION.md): R02-01 ACCEPTED только public upstream source. Ранний EVE CONFIRMED сохранён отдельно; включение flow и two-sided native compatibility не приняты.
- [TUF progress и разрешённая ротация](2026-10-27-R05-001-PROGRESS.md): R05-001-A/B ACCEPTED в named spec/public-source/model области; snapshot-only full-key-replacement trace не распространяется на все rotations или текущий адаптер.
- [DR27: 16 направлений, scoped и смешанные решения](2026-10-28-DEEP-REVIEW.md): 15 полных отчётов, частичный DR27-16; R04-14 и расширения DR27-10 требуют пересмотра. Два дополнительных opponent review не завершены.
