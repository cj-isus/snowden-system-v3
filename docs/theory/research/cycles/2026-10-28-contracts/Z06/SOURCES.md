# R28-Z06 — источники статуса, проверки схемы и допуска

**Чтение исходного среза — 28 октября 2026 года.**
[Аргумент](REPORT.md) · [Сводка](HANDOFF.md).
Публичные копии: `.artifacts/research/r28-contracts/Z06/`.

## Краткий вывод

Источники показывают конкретные потери информации при проекции статуса,
существующие guards parser/HTTP и границы атомарных примитивов. Они не
предоставляют готовые текущие полномочия операции, установленный provider
или проверенный Android-адаптер. FULL_TEXT_READ означает целиком прочитанный
небольшой файл; SECTIONS_READ — только указанную часть; FETCH_FAILED —
неполученный источник, а не отрицательный факт о нём.

## 1. Основание и независимость

SECTIONS_READ: R05-009 REPORT §10:545–779 и SOURCES §8;
FULL_TEXT_READ: `architecture/RELEASE-EVIDENCE-BOUNDARIES.md`.
Нужные verifier/Gson-копии и канонические документы были доступны.
В адресно проверенных Android .kt/.kts consumer этого Google status verifier
не найден. Это не доказательство отсутствия любого consumer во всех
зависимостях, артефактах и runtime.

Первоначальная D и повторная проверка имеют общее авторство. Новое совпадение
байтов и собственный контрпример не создают внешней независимой рецензии.
Поздние C05/lead решения учитываются только в их названной области.
Publication date — UNKNOWN, когда источник её не сообщает;
Last updated и время чтения не подменяют дату создания.
Загруженный код не исполнялся.

## 2. Реестр источников

### V — Google/Android Key Attestation Verifier

Commit `34652d3df68b8d6887a6670b00eb0856090dae6d`,
[точное дерево](https://github.com/android/keyattestation/tree/34652d3df68b8d6887a6670b00eb0856090dae6d).
Исходное получение около 08:56Z, чтение 08:57–09:10Z; publication date UNKNOWN.
Замороженный release, Maven bytes и установленная зависимость не подтверждены.

| Файл | Прочитанное и предел |
|---|---|
| `src/main/kotlin/GoogleRevocationList.kt` | FULL_TEXT_READ1–88: HTTP200/throw/finally; String DTO→REVOKED set; нет status age и собственного timeout |
| `src/main/kotlin/Verifier.kt` | SECTIONS_READ140–371, result44–68: три providers; status callback до validate catch; общий Success не разрешает операцию |
| `provider/RevocationChecker.kt` | FULL_TEXT_READ1–56: membership supplied set по переданной ссылке; нет собственной immutable copy и времени статуса |
| `provider/KeyAttestationCertPath.kt` | SECTIONS_READ50–84: исключение anchor через dropLast, формат serial hex |
| `provider/KeyAttestationCertPathValidator.kt` | SECTIONS_READ118–268: extra-extension guard и последовательность signature/checker; не приёмка всего PKI engine |
| `build.gradle.kts` | Зависимости/toolchain31–52: Gson2.11.0, Kotlin2.2, JDK21 |

Десять критичных файлов (5V+5G) совпали с прежними копиями;
`source-identity-comparison.json`. Это идентичность файлов, не correctness,
атомарность всего дерева, аттестация release или native-приёмка.

### G — Gson2.11.0

Tag gson-parent-2.11.0: object `3b7c2ad8ce8dd471449ccbaee6d22f64c9816a38`,
peeled commit `828a97be0f8d58108b140b77df8dc76b657f4a87`;
tag повторно разрешён 09:05Z.
[Точное дерево](https://github.com/google/gson/tree/828a97be0f8d58108b140b77df8dc76b657f4a87).
Получение около 08:56/09:00Z, SECTIONS_READ08:57–09:09Z;
publication date UNKNOWN.

| Файл под `gson/src/main/java/com/google/gson/` | Прочитанная область |
|---|---|
| G1 `Gson.java` |155–161 defaults;1259–1265 Reader;1345–1381 parse;1466–1475 consumption: nullable/lenient/IO и Reader guard |
| G2 `internal/bind/ReflectiveTypeAdapterFactory.java` |148–161,265–279,498–565: повторное присваивание DTO fields, unknown skip/null references; нет проверки required по schema |
| G3 `internal/bind/TypeAdapters.java`, `stream/JsonReader.java` |STRING458–478,nextString890–921: boolean/numeric coercion; синтаксис не равен typed enum |
| G4 `internal/bind/MapTypeAdapterFactory.java` |172–209: array-pairs и duplicate guard при non-null предыдущем значении; не все дубликаты объектов |
| G5 `internal/ConstructorConstructor.java`, `GsonBuilder.java` |126–176 constructor fallback;552–568 strictness: reflection/Unsafe условны; строгий parser не является готовым TypeAdapter |

[LICENSE](https://github.com/google/gson/blob/828a97be0f8d58108b140b77df8dc76b657f4a87/LICENSE)
прочитан целиком: Apache-2.0. Java/reflection/parser/R8 — NOT_RUN.
Выводы о null условны на обычный DTO adapter и успешное выделение объекта;
точное поведение каждой собранной JVM этим не установлено.

### H — нормативные HTTP и JSON

| Источник | SECTIONS_READ исходного среза 08:58–09:13Z | Предел |
|---|---|---|
| R.Fielding/M.Nottingham/J.Reschke eds, RFC9111 HTTP Caching, June2022, [текст](https://www.rfc-editor.org/rfc/rfc9111.txt) |§3.3/3.4 incomplete;§4/4.1 URI/method/Vary;§4.2.1/3 age;§4.3.4 304;§5.2.2.2/4;§6 app cache | HTTP freshness не разрешает действие и не устанавливает глобальный latest. Фактические headers неизвестны |
| T.Bray ed, RFC8259 JSON, Dec2017, [текст](https://www.rfc-editor.org/rfc/rfc8259.txt) |§4 дубликаты имён;§8.1 UTF8;§9 parser limits | SHOULD unique не является автоматически исполняемым MUST всех parsers; app schema отдельно |
| R.Fielding/M.Nottingham/J.Reschke eds, RFC9110 HTTP Semantics, June2022, [текст](https://www.rfc-editor.org/rfc/rfc9110.txt) |§8.8.3/1/2 opaque ETag, получение 09:09Z | ETag не задаёт монотонный status-floor и порядок статусов |

Прочитаны английские оригиналы. В REPORT — пересказ и собственный вывод,
не полный текст RFC; наличие русского первичного перевода не заявлялось.

### D — русская официальная схема Android status

Google/Android Developers, *Verify hardware-backed key pairs with key attestation*,
[русская страница](https://developer.android.com/privacy-and-security/security-key-attestation?hl=ru).
Получение 09:00Z; фактически русский текст, SECTIONS_READ09:01–09:08Z:
Certificate status schema, Cache-Control, expires, factory-validity, publication.
Last updated2026-07-09UTC; первоначальная публикация UNKNOWN.

Использованы REVOKED/SUSPENDED, required status, канонический lowercase serial,
закрытые properties, expires как дата сертификата, задержка публикации.
Публичные root PEM examples удалены из копии и не разбирались/не проверялись.
Реальные `/attestation/status` и `/root` — NOT_FETCHED/LINK_ONLY.
Сертификаты устройства, accounts, latest status и живые HTTP headers не проверены.

### J — Jackson-core2.18.3

Tag object `d04c20d7ed00f37127e690288f9923e97f6176c7`,
peeled commit `cc34e8d5b75d4c911c16f2bc87d6e13e42eafddd`;
[точное дерево](https://github.com/FasterXML/jackson-core/tree/cc34e8d5b75d4c911c16f2bc87d6e13e42eafddd).
Получение 09:00–09:08Z; publication UNKNOWN; установленного runtime не проверяли.

| Файл | Прочитанный предмет |
|---|---|
| `StreamReadFeature.java`, `JsonParser.java` |315–375 и определения: явный duplicate flag по умолчанию false; сопоставлены версии privacy comments |
| `json/DupDetector.java`, `JsonReadContext.java` |FULL_TEXT_READ DupDetector1–111; context242–253: проверка имён до map/tree, накладные расходы HashSet |
| `StreamReadConstraints.java` |39–95 defaults;140–190 builder;483–571 checks: document/token по умолчанию не ограничены, важны единицы конкретных limits |
| `JsonFactory.java`, `TSFBuilder.java` |Parser refs, builder194–227,350–362: duplicate/read constraints |
| `UTF8StreamJsonParser.java`, `ReaderBasedJsonParser.java`, `ParserBase.java` |UTF8_loadMore206–242;Reader248;Base271/1361: достижимость duplicate/depth guards. Общая память/buffering не аттестованы |

[LICENSE](https://github.com/FasterXML/jackson-core/blob/cc34e8d5b75d4c911c16f2bc87d6e13e42eafddd/LICENSE)
прочитан целиком: Apache-2.0. Комментарий об overhead20–30% — оценка автора
исходника, не наш benchmark и не численный прогноз для Snowden.
Parser tests и JSON fixtures не исполнялись.

### N — NetworkNT json-schema-validator1.5.9

Tag1.5.9 → `270df62471e6a0c564dc13ae6cf2bf4116916f82`,
[точное дерево](https://github.com/networknt/json-schema-validator/tree/270df62471e6a0c564dc13ae6cf2bf4116916f82).
Получение 08:57–08:58Z, чтение 09:00–09:13Z; publication UNKNOWN.
POM: Java8/Jackson2.18.3/itu1.14/slf4j2.0.17; без установки/сборки.

| Файл | Прочитанная область и предел |
|---|---|
| `README.md` |SECTIONS_READ235–298/config: typeLoose и получение schema; пример использования не приёмка |
| `TypeValidator.java`, `utils/JsonNodeUtil.java` |FULL_TEXT_READ type1–70/utils68–120: проверка node type и условие typeLoose |
| `RequiredValidator.java`, `EnumValidator.java` |FULL_TEXT_READ required1–97/enum1–124: missing/null различаются; enum точен без явного ослабления |
| `AdditionalPropertiesValidator.java` |35–183: skip при startsWith("#"); ограничение этой ветви, не исполненная атака |
| `PropertyNamesValidator.java` |FULL_TEXT_READ1–68: все имена; противовес для serial-map, не замена root/entry whitelist |
| `JsonSchema.java`, `JsonSchemaFactory.java` |599–662 aggregate result; getInstance/constructor/loading: набор ошибок не разрешает операцию |
| `DisallowSchemaLoader`, `DefaultSchemaLoader`, `SchemaLoaders` |FULL_TEXT_READ: локальная разрешённая map затем окончательный deny; обычный lookup может продолжиться сетевым loader |
| `pom.xml`, `LICENSE` |Manifest70–163; полный Apache-2.0 LICENSE, не весь transitive licence/security audit |

Три исходника AdditionalProperties/Disallow/POM совпали с архивом.
Получение canonical draft07 `.txt` и альтернативного `.html` вернуло 403:
FETCH_FAILED, полного чтения спецификации не заявлено.
Использованы опубликованная Google schema и выбранные validator sources;
общая гарантия произвольной схемы остаётся отложенной.

### A — атомарные ссылки и границы вызывающего кода

| Источник | SECTIONS_READ и предел |
|---|---|
| Oracle JDK21 [AtomicReference](https://docs.oracle.com/en/java/javase/21/docs/api/java.base/java/util/concurrent/atomic/AtomicReference.html) |CAS502–529/updateAndGet638–666: атомарна ссылка; функция обновления может повторяться и не должна включать необратимый эффект. Не file durability/network action. Получение 09:01Z |
| JetBrains [Kotlin collections](https://kotlinlang.org/docs/collections-overview.html) |Read-only/mutable/val: вид не делает содержимое immutable. Получение 09:03Z, publication UNKNOWN |
| Oracle JDK21 [URLConnection](https://docs.oracle.com/en/java/javase/21/docs/api/java.base/java/net/URLConnection.html), [HttpURLConnection](https://docs.oracle.com/en/java/javase/21/docs/api/java.base/java/net/HttpURLConnection.html) |Timeout0 означает бесконечное ожидание в названном контракте; оговорки реализации и redirect defaults. Получение 09:09Z; фактический custom provider неизвестен |
| Oracle JDK21 [CertPathValidator](https://docs.oracle.com/en/java/javase/21/docs/api/java.base/java/security/cert/CertPathValidator.html) |Выбор getInstance по algorithm/provider preference и explicit-provider overload. Получение 09:13Z; реальный provider не исполнялся |

Проектный `EncryptedProfileStore.kt`, SECTIONS_READ63–144: expected revision,
generation/floor/uncertainty/pending-stage, копия 09:09Z. Это существующий
образец исходников, не доказанный disk commit или current revocation gate.
Реальные ключи/профили не читались. ProfileStoreCrypto сохранён только
для навигации, полного аудита его исходников здесь не было.

## 3. Сопоставление и предел доказательства

Из C05 REPORT адресно прочитаны draft §§2–3 в 09:13Z: same-request inputs,
root/provider/current-generation. Готового HANDOFF в тот момент не было.
Поздние изменения draft не покрываются этим срезом автоматически;
согласие авторов не заменяет первичный источник.

T1–T8 — логические последовательности, не fixtures и не полевой/native опыт.
Ресурсные формулы — условная модель выделений, не heap/RSS guarantee.
Реальные certs, Google status endpoint, vault, runtime-логи не использовались;
тесты, сборка, crypto, device/live и серверные действия не выполнялись.

Реестры acquisition/reuse/additional/parser-path/http, source identity и
baseline inputs сохраняют даты чтения, байты, пути и ошибки.
SHA256 именует сохранённые входы, не их правильность и не атомарный снимок
всего репозитория. Практический перенос и цена — в HANDOFF.

<a id="r28-z06--источники-и-scope-прочтения"></a>
<a id="1-baseline-availability-independence"></a>
<a id="2-source-registry"></a>
<a id="g--google-gson2110"></a>
<a id="h--normative-httpjson-originals"></a>
<a id="d--official-russian-android-status-schema"></a>
<a id="j--fasterxml-jackson-core2183"></a>
<a id="a--atomicreferencecaller-primitives"></a>
<a id="3-paired-scope-и-финальный-review"></a>
