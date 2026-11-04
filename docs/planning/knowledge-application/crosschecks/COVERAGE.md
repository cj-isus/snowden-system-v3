# Полнота корпуса и применимость решений

**Срез: 28 октября 2026. Вердикт независимого crosscheck: PARTIAL по содержательной полноте применения.** Структурный реестр согласован: **2 714 уникальных units**, и для каждого существует primary-строка. Но заполненные строки не доказывают правильность всех решений. Независимо прочитаны **148 выбранных units**; остальные **2 566** проверены структурно, без самостоятельной верификации всех аргументов и callers.

## Состояние покрытия

| Контур | Проверенный результат | Граница доказательства |
|---|---|---|
| Frozen inventory | 287 исходных файлов / 2 698 units; late — 3 версии файлов / 16 новых units | Это census, не научная приёмка |
| Назначение | A00: 448; C02: 660; C03: 363; C04: 378; C05: 438; C06: 427. Все IDs назначены ровно один раз | Тематический owner не разрешение на продуктовые изменения |
| Primary coverage | На конечном срезе есть строки для всех 2 714 IDs | Наличие disposition и ссылки не доказывает соответствие конкретному знанию |
| Исходный сборник | 319 169 байт; общий SHA-256 совпал. Проверены хэши 98 исходных диапазонов и наличие целевых chapters/anchors | `sourceStart/sourceEnd` — символьные, не байтовые offsets. Это сохранность, не истинность всех разделов |
| Snapshot provenance | Все 2 714 нормализованных body hashes согласованы; 15 whole-file snapshots отличаются от исходного source hash | `prepare.mjs` применяет `trimEnd()+LF`. Нужны отдельные original/snapshot hashes; полное byte equality для этих 15 файлов неверно |
| Архивирование | Все 80 перенесённых путей найдены после применения карты relocations | Перенос не означает допустимость исключения смешанного scientific содержания |
| Duplicate candidates | Прочитаны тела 40 равных normalized-body пар; у 8 различаются разрешённые относительные ссылки; 10 требуют проверки heading/link context | Равенство body не равно тождеству evidence и не разрешает удаление |
| Late derivatives | `APPLICABILITY-LIMITS.md` и `LIMITATIONS.md` не входят в frozen inventory | Это производные ограничения существующих claims; требуется lineage, а не автоматический новый scientific verdict |
| Каталог | 58 legacy handoffs и связанные report/source targets доступны. Все 42 R28 REPORT/SOURCES/HANDOFF существуют и входят в inventory | CATALOG.json ещё не имеет явного R28 cycle entry: reader-index gap, не потеря R28 корпуса |

Исходный сборник: [архив](../../../theory/archive/CENSORSHIP-THEORY-before-split-2026-10-26.md), SHA-256 `a70114d7bef534c348e0e0c5296721cab7d781f2fe7057979406235564c37ad7`. Машинная [карта 98 разделов](../../../theory/CONTENTS.json) сохранена отдельно от научных решений.

Точные IDs, source cuts и структурные списки находятся в `.artifacts/planning/knowledge-application/2026-10-28/coverage/Z02.json` и `captures/Z02/`. Это private evidence, не дополнительный редактируемый master теории.

## COV-01. Mapping объясняет другой technical unit

**Unit `K-b67482e9fa7456`**, [COMPOSITION-AND-TLS-CONTRACTS §3](../../../theory/transports/COMPOSITION-AND-TLS-CONTRACTS.md), frozen lines 68–96: `requested MinVersion → uTLS preset → mutualVersion`, claim R02-16. Его область ограничена Chrome/non-real-ECH, без REALITY/kTLS и дополнительного внешнего version gate.

В C02 coverage unit имеет `change_required`, family `N02`, reason о **TLS-less VLESS→ShadowTLS** и code refs на render:692/metadata:187. Это другой вопрос. [NETWORK N03](../sections/NETWORK.md#n03) уже упоминает TLS minimum, но конкретная строка туда не связана и не объясняет достаточный caller/compatibility gate.

**Минимальная поправка:** связать unit с N03/TLS-floor subfamily, назвать backend, preset, version-check caller и фактически запрошенный floor продукта. До проверки активного consumer решение — **needs review / compatibility gate**, а не безусловное требование менять ядро.

Прочитанные seams: `windows/backend/render/render.go:666,720,754` содержит uTLS/Chrome; эти строки сами по себе не устанавливают requested minimum 1.3. Android `NativeGraphSecurity.kt:104–117` принимает структурные TLS bounds, но это не аттестация AAR. Existing IDs: TH-01/TH-05/OFF-04, AND-01/AND-06.

**Opposing guard:** requested floor может отсутствовать; STD, иной preset, real ECH или внешний strict gate имеют другую область. Сохраняются certificate/Finished и другие проверки. Исправление текущей ShadowTLS eligibility не закрывает отдельное знание о requested TLS floor.

## COV-02. Смешанное administration требует точного substantive crosswalk

TASK/STATUS/WINDOW не обязаны становиться новыми requirements. Однако их source facts, counterexamples и ограничения нельзя исключать только по имени файла. Формулировка «substantive источники рассматриваются отдельно» без ID/family не доказывает, что они действительно сохранены.

| Unit / frozen scope | Содержимое, требующее сохранения | Достаточная связь |
|---|---|---|
| `K-4ed76a49b04234`, NINE-WINDOW §4:158–170 | Receive window не whole RAM; PTO maximum не minimum; singleflight не cleanup generations; CCA-трилемма — гипотеза | PERFORMANCE F15/F04/F06/F07; RQ-03/RQ-05/TR-04/TR-06. Не восстанавливать старые ошибочные формулы |
| `K-bd9ea941e6100a`, WINDOW-2026-10-26-FINAL:2–28 | Artifact/OS/adoption, offline status, multirepo guards и old-client limits; READY и будущие проверки | R05-004/005/006 → TRUST K8/K4/K2; TH-03/AND-14. Исключение execution применимо только к research scope |
| `K-0ce6ca26f2662b`, NINE quality:89–115 | Reading depth, исправление R03-17, source/model/field distinctions | RESEARCH-METHOD и ADAPTATION F17/F11 как evidence governance, не новая функция продукта |

Текущие A00 rows этих трёх units имеют `administrative`, section «Структура знаний». Исключение может быть правильным, **если** указаны связанные substantive units и retained limitations. Сейчас такой точной связи в проверенных строках нет.

[REVIEW-01](../../../theory/research/cycles/2026-10-28-contracts/REVIEW-01.md) корректно ограничивает четыре accepted scopes. Его mapping уже появился в root families; это положительный результат, не основание распространить ACCEPTED на соседние pending claims.

## COV-03. Восемь duplicate-пар имеют разные link contexts

Все 40 кандидатных bodies действительно совпадают. Ниже — не «восемь разных научных результатов», а пары, которые нельзя сворачивать без проверки источников относительных ссылок.

| Candidate → base | Что различается |
|---|---|
| `K-22b79b0fee2f86` → `K-639fcdc354ce94` | LIBRARY-FORMAT в архиве и каноне: reading guide/claim format/review queue |
| `K-c20fc9598f55c0` → `K-975f20c25bb3b2` | STATE «Завершённые срезы»: ссылки на R02-009/010 и области verdicts |
| `K-6eb6ad666f6945` → `K-9000ada62ccf8f` | STATE «Предыдущие результаты»: review R02-10 и исторический namespace |
| `K-bdc07b6bf2ecba` → `K-d03b212bccb70e` | Root STATE: смешанные accepted/pending ссылки и временной контекст |
| `K-3bd830696c1299` → `K-1ca14a91a4c195` | DEPTH-CYCLE source context |
| `K-ac76adc5062aa9` → `K-828ee5ce3a5e67` | TEN-HOUR-WINDOW link и историческая область |
| `K-cff8dfcfca930d` → `K-a3837bdfd1aea7` | HB01 review/handoff links и предел принятия |
| `K-b4684375943d53` → `K-5d0510fd5330b3` | MIGRATION/CONTENTS links к исходной карте |

Ещё две пары различаются heading: README/KNOWLEDGE-MAP и STATE «Предыдущие датированные записи»/«Исторический снимок». Итого **10 context-sensitive кандидатов**; body mismatch не найден.

**Минимум для сворачивания:** `duplicate_of_unit_id`, исходный и canonical target, историческая дата/роль, resolved-link crosswalk и retained scope. Архив можно убрать из читательской навигации; удалять его provenance или делать одинаковый scientific verdict по равному SHA нельзя.

## COV-04. Late reader derivatives и R28 reader index

[APPLICABILITY-LIMITS](../../../theory/research/cycles/2026-10-28-contracts/APPLICABILITY-LIMITS.md) и [LIMITATIONS](../../../theory/research/LIMITATIONS.md) созданы после frozen census. Оба текста прочитаны как производные summaries: первый содержит шесть ограничений переноса, второй сохраняет explicit references к прежним reports/reviews и подчёркивает отсутствие новых научных решений. Наличие ссылок полезно, но оно ещё не является unit-ID lineage в issued inventory.

**Минимум:** дать derivative lineage к существующим unit IDs, source hash/date и canonical refs. Не считать производный summary новым accepted claim и не увеличивать denominator по числу его headings без согласованного правила identity.

CATALOG.json содержит legacy 58 handoffs и NINE/DR27 cycles, но не явный R28 cycle slot. R28 bodies уже есть в inventory и reader/review register. Дополнение index — задача навигации и provenance, не повторное научное принятие.

**Не утрата:** 80 organization/obsolete путей разрешены через relocation map; отсутствующих source files после этой сверки нет. Изменения reader-links требуют current-source cut, не новой теории.

## COV-05. Общая family-summary не покрывает каждое distinct утверждение

**Unit `K-4492a002f13283`**, [PROBE-PROOF-BOUNDARIES §1](../../../theory/observability/PROBE-PROOF-BOUNDARIES.md), frozen lines 11–31: fresh signed nonce, доверие signer как измерителю и частичное HMAC coverage TWAMP.

Его row `change_required / N07` объясняет current public-IP → LeakConfirmed. Это полезный related principle, но не disposition всех specific references внутри unit.

**Минимум:** сохранить различие между применимым принципом field coverage и reference-only counterexample неиспользуемого TWAMP. Для принципа нужен scope/covered fields/request correlation; для TWAMP — «reference, не выбран, нового кода не требует» и точная ссылка. Внедрение TWAMP/нового signer не следует из этого unit.

Actual caller: `windows/backend/core/probe.go:284–290,332–363` действительно переводит scalar egress comparison в leak flag. Там также есть unavailable/context guards. `windows/failure_observation.go:48–65` сохраняет safety precedence, а observer проверяет current intent/session/epoch. N07 может требовать уточнения semantics после owner review; это не повод удалять hard stop или объявлять все reference-параграфы requirements. Existing IDs: TH-02/OFF-05/FR-003a, AND-07/A-FR-06.

## COV-06. Устранённые ранние пропуски не должны остаться ложными finding

Семь units [Padding/observation window](../../../theory/transports/PADDING-AND-OBSERVATION-WINDOW.md) изначально попали в A00, хотя это scientific topic schedule/stop/restart/evaluation. На конечном срезе A00 **все семь** получили индивидуальные reference/defer причины, TH-05/E-04/E-08/FR-012. Ранний rowless gap устранён.

Текущий renderer включает mux+padding только по opt-in (`render.go:684–688,764–770`). TH-05 — planned candidate, TH-06 — comparative gate, COM-18 — optional privacy. Для этих знаний достаточно сохранить evaluation contract; новый padder, ML runtime или dependency не обязательны.

**Нельзя считать этот ранний gap оставшимся** лишь потому, что прежний capture показывал пустую строку. Аналогично placeholder C06/A00 coverage уже заполнена; финальный census не выводится из раннего состояния работающего автора.

## Проверенные отрицательные результаты

- Хэш исходного сборника и все 98 source ranges согласованы с CONTENTS; chapter/anchor targets найдены. Сохранность не доказывает правильность всех исторических тезисов.
- Initial units охватывают все непустые не-heading строки frozen sources; heading сохранён в context. В late 29 строках вне новых ranges пропуска нет: их body точно соответствует initial units; crosswalk сохранён.
- Коллизии assignments, повторные unit IDs и body hash failures не найдены.
- В checked ADAPTATION F17 и PERFORMANCE F15 исправления R03-17/R04-008/009 сохранены; их старые ошибки не назначены новыми обязательными функциями.
- NETWORK сохраняет defer real ECH и не объявляет conditional library finding текущей неисправностью renderer без real ECH.
- **В проверенных families не установлена безусловная promotion pending → requirement.** COV-01/05 создают риск такой ошибки при интеграции. Непрочитанные families не объявлены «чистыми».

## Минимальная последовательность применения

| Приоритет / ответственный | Достаточная дельта | Совместимость и зависимости | Будущий критерий — NOT_RUN |
|---|---|---|---|
| P0 перед final plan / руководитель | Исправить specific family edges COV-01/05 и mixed-unit crosswalk COV-02 | Existing unit/claim/caller/TH/AND/ADP; без новых продуктовых ролей | Каждый change имеет свой source claim, scope, caller, existing plan и acceptance gate |
| P0 mapping integrity / maintainer | Original/snapshot/body hashes, normalized ranges, relocations | Сохранить issued IDs и frozen bytes | Нет byte-equality обещания для 15 formatted snapshots |
| P1 reader integrity / maintainer | Context-aware duplicates, R28 catalog и derivative lineage | Исторические links/dates/review limits | Восемь link-different пар проверены до удаления исходного контекста |
| P1 Windows owner после review | Egress comparison отдельно от confirmed leak | Producer/classifier/observer/UI меняются согласованно; hard stop/ownership сохраняются | Confirmed leak требует named premise/witness; unknown не healthy |
| P1/P2 TLS owner | Requested-floor/backend/preset matrix как compatibility gate | Chrome/STD/real ECH и Android native — отдельные области | Active requested-floor consumer и сильный guard установлены до выбора fix |
| P2 после safety / network-performance owner | Padding evaluation contract как deferred reference | Opt-in mux, оба конца, lifecycle и FR-012 | Stop/restart/horizon/splits/benign corpus/resources соответствуют выбранной цели |

Старая клиентская совместимость требует сохранения публичных контрактов и разных unsupported/unknown/outcome состояний. Родственное знание не даёт основания массово заменить schema, ядро или FFI. Дешёвое достаточное решение — уточнить mapping и existing seam, а не создать ещё один DTO/controller/БД.

## Достаточная запись mapping и предел проверки

`unit_id · sourceSha256 · snapshotSha256 · bodySha256 · range/context · knowledge/prerequisites · substantive family/duplicate-of · caller/source cut · existing plan ID · disposition · review scope · opposing guard · compatibility/deps/cost · future acceptance NOT_RUN · missing reason`.

Не каждый unit требует нового кода: source facts, counterexamples, limitations и provenance могут получить reference/defer/reject/administrative. Нужны **конкретная причина и связанный substantive ID**, а не один шаблон для всей папки.

Независимо прочитаны 148 выбранных units. 2 566 остальных не объявлены substantively verified этим crosscheck. Структурная проверка охватила весь issued inventory и late delta; full semantic coverage proof не получен. Итог — **PARTIAL**, с завершённым точным списком проверенных проблем и явно открытым остальным корпусом.

На проверке завершения source cuts изменились `docs/PLAN.md` и `docs/ANDROID-PLAN.md`: Windows — до V2-185 selected-output write, Android — до AND-02-S7/AND-05-S4 admission refinements. Это параллельная работа владельцев, не изменение crosscheck. Названные выше TH-02/TH-05/TH-06, AND-06/AND-07/AND-18 остаются отдельными открытыми gates; новые lifecycle refinements целиком здесь не audited. Прочитанные probe/render/config/classifier seams на final check не изменились. Старый frozen corpus и current source cuts сохранены раздельно.

Не стоит компенсировать coverage gap новым survey, веб-поиском, обязательной телеметрией, БД/ML/SDK ролью или автоматическим восстановлением ошибочных исторических моделей. Source/model/READY не product/native/live PASS. Изменения кода, планов владельцев, deps/roles/bindings, tests/build/probes/crypto/native/VPS/deploy не выполнялись.
