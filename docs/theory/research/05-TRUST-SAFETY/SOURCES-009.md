# R05-009 — источники, pins и границы прочтения

Дата чтения: **27 октября 2026**. [REPORT-009](REPORT-009.md),
[краткий разбор](handoffs/R05-009.md). Source retrieval/review: примерно
13:17–13:44Z; точный срез и manifests — в artifacts. Crypto execution нет.

## 1. Метод и реальные пути

Канон этого checkout: `D:/SNOWDEN-SYSTEM-V3/docs/theory/`; реальные исходники
Android здесь присутствуют. Утверждения о другом Obsidian snapshot не перенесены
на этот checkout. Исторические R05-004/005/008 прочитаны как baseline,
не как принятые аксиомы.

Собственный каталог:
`.artifacts/research/theory-05/2026-10-27-keystore-attestation-res6/`.

- `local-inputs.json` и `local-inputs/` — адресные source copies пяти файлов,
  hashes/time. Не атомарный snapshot всего Android дерева.
- `capture-manifest.json` — SHA-256 сохранённых source/doc captures, не
  attestation SDK/устройства, не product PASS.
- `verifier-source/` — текстовые исходники точного candidate commit; build,
  tests, certificate fixtures, генераторы и crypto providers не запускались.
- `*.html/*.txt` — официальные публичные страницы. PEM-примеры официальных
  roots исключены из readable captures после получения; не парсились,
  не верифицировались и не стали trust inputs продукта. Public source archive
  может включать upstream test-data; они не использованы как certificate cases.
- AOSP `*.base64` декодированы как транспортный формат исходника в `*.aidl`;
  это не криптографическая операция/декодирование ключа или сертификата.

`FULL_TEXT_READ` ниже относится к тексту статьи/короткого исходника, без
сертификатных примеров. `SECTIONS_READ` означает только точно названные секции:
нельзя переносить эту отметку на весь сайт/репозиторий. Publication date, если
страница его не даёт, **UNKNOWN**; Last updated не переименовывается в дату
первой публикации.

## 2. Официальные источники Android: русский текст действительно прочитан

| ID | Организация, название / URL | Дата документа | Реальная область чтения, время | Использование |
|---|---|---|---|---|
| D1 | Android Developers / Google, [Verify hardware-backed key pairs with key attestation — русская локаль](https://developer.android.com/privacy-and-security/security-key-attestation?hl=ru); [en-US](https://developer.android.com/privacy-and-security/security-key-attestation?hl=en) | First publication UNKNOWN; Last updated **2026-07-09 UTC** | FULL_TEXT_READ RU и EN, примерно 13:17–13:44Z; PEM omitted. Разделы Before you begin; retrieve/verify; roots; verification libraries; status; factory expiry; revocation policy | Independent verifier, chain+expected extensions, root/status inputs, first trusted extension, RKP/factory profile, offline предел |
| D2 | Android Developers / Google, [Android Keystore system — русская локаль](https://developer.android.com/privacy-and-security/keystore?hl=ru) | First publication UNKNOWN; Last updated **2026-10-24 UTC** | SECTIONS_READ article body RU, 13:18–13:44Z; Security features/nonextractability; StrongBox; authorizations; user authentication; selected JCA examples, secure import (не все примерные code blocks) | Nonexportability ≠ no malicious use; hardware level per-key; authentication/time modes; costs StrongBox |
| D3 | Android Developers / Google, [Android Protected Confirmation — русская локаль](https://developer.android.com/privacy-and-security/security-android-protected-confirmation?hl=ru) | First publication UNKNOWN; Last updated **2026-10-24 UTC** | FULL_TEXT_READ RU, 13:18–13:28Z; relying-party steps; onConfirmed; promptText/extraData; replay; signing-only RSA restrictions | Exact confirmed CBOR, hidden extraData не witnessed, confirmation ≠ authentication/confidentiality |
| D4 | AOSP, [Key and ID attestation — RU](https://source.android.com/docs/security/features/keystore/attestation?hl=ru); [en-US](https://source.android.com/docs/security/features/keystore/attestation?hl=en) | First publication UNKNOWN; Last updated **2026-09-30 UTC** | SECTIONS_READ 13:17–13:33Z: schema/version panels; KeyDescription fields; security levels; authorization-list fields NO_AUTH/USER_AUTH/AUTH_TIMEOUT/TUP/confirmation; RootOfTrust. Не full ID-attestation implementation audit | attestation/KeyMint levels, software/hardware lists; tag507 vs508; challenge-generation scope. RU machine-translation anomalies сверены EN/AIDL |
| D5 | Android Developers, [KeyInfo](https://developer.android.com/reference/android/security/keystore/KeyInfo) | Last updated **2026-08-03 UTC**; original publication UNKNOWN | SECTIONS_READ 13:25–13:36Z: getSecurityLevel (API31), isInsideSecurityHardware (API23), isUserAuthenticationRequirementEnforcedBySecureHardware, isUserConfirmationRequired | SDK guard: min29 ≠ runtime31; local KeyInfo ≠ remote certificate |
| D6 | Android Developers, [KeyGenParameterSpec.Builder](https://developer.android.com/reference/android/security/keystore/KeyGenParameterSpec.Builder) | Last updated **2026-08-03 UTC**; original publication UNKNOWN | SECTIONS_READ 13:33–13:36Z: setAttestationChallenge(API24), setUserAuthenticationParameters(API30), setUserConfirmationRequired/setUserPresenceRequired(API28) | Symmetric AES/HMAC challenge rejection; actual API levels; auth/presence/confirmation distinctions |
| D7 | Android Developers, [BiometricPrompt](https://developer.android.com/reference/android/hardware/biometrics/BiometricPrompt) | Last updated **2026-08-28 UTC**; original publication UNKNOWN | SECTIONS_READ 13:33–13:44Z: authenticate(CryptoObject,CancellationSignal,Executor,AuthenticationCallback), API28; strong biometrics; API29 credential restriction/cancel | Callback не transaction proof; only bound key operation yields crypto evidence; minAPI differences |
| D8 | AOSP, [Authentication — русский текст](https://source.android.com/docs/security/features/authentication?hl=ru) | Last updated **2026-09-30 UTC**; first publication UNKNOWN | FULL_TEXT_READ 13:22–13:25Z; enrollment, auth flow, HAT format, boot shared secret | HAT защищает auth для KeyMint, не remote presence receipt; clocks/boot/SID |
| D9 | AOSP, [Android Protected Confirmation — русский текст](https://source.android.com/docs/security/features/protected-confirmation?hl=ru) | Last updated **2026-09-30 UTC**; first publication UNKNOWN | FULL_TEXT_READ короткой обзорной страницы 13:22–13:25Z; scope ConfirmationUI+KeyMint в TEE | Trusted UI threat boundary, secure token → signing enforcement |

Страницы получены через HTTPS curl **с проверкой TLS**. WebFetch D1 дал
content-blocked, D2/D3 — timeout; это не было выдано за успешное чтение.
Прямая загрузка curl затем дала непустой официальный HTML и реальный русский
article text. Никаких сертификатных проверок отключением TLS или запросов к
аккаунтам/устройствам пользователя.

Официальная D1 определяет status JSON, не утверждает instant publication:
новый отзыв обычно в течение нескольких дней, возможна задержка.
`https://android.googleapis.com/attestation/status` и `/root` в этом срезе
**LINK_ONLY / NOT_FETCHED**, не queried latest status/anchors. Отсутствие
запроса не заменено `emptySet` знанием. Использовалось описание endpoint,
а не реальные сертификаты/данные о статусе конкретных ключей.

## 3. Exact AOSP source references (не device artifact)

Framework tag `android-16.0.0_r1` resolved read-only `git ls-remote`:
annotated tag `2e7c3f260cbaef2052cdf2c1c84d8442549e0063`, peeled commit
**99b01a65cc4c104933788b3143285ab6bae65827**.

Hardware/interfaces tag `android-16.0.0_r1`: tag
`98f077657c3ce04f613efee891bc1ea7316008ef`, peeled commit
**5688f7eb1e117ed26e642a695de300b7683acb87**.
Tag date/first publication не установлены, source чтение 27.10.2026;
не используется rolling `android17-release` из current doc как будто это
Android16 source pin. Изначально guessed GitHub mirror hardware/interfaces
дал repository-not-found; исправлено на официальный googlesource.

| ID | Exact path / источник | Прочитанная область |
|---|---|---|
| A1 | [KeyGenParameterSpec.java](https://github.com/aosp-mirror/platform_frameworks_base/blob/99b01a65cc4c104933788b3143285ab6bae65827/keystore/java/android/security/keystore/KeyGenParameterSpec.java) | SECTIONS_READ :1410–1609; auth invalidation; confirmation separate; timeout=0; TUP; asymmetric challenge vs AES/HMAC rejection |
| A2 | [KeyStore2ParameterUtils.java](https://github.com/aosp-mirror/platform_frameworks_base/blob/99b01a65cc4c104933788b3143285ab6bae65827/keystore/java/android/security/keystore2/KeyStore2ParameterUtils.java) | SECTIONS_READ :306–351 addUserAuthArgs; NO_AUTH_REQUIRED; absent AUTH_TIMEOUT for0; confirmation/presence tags |
| A3 | [HardwareAuthToken.aidl](https://android.googlesource.com/platform/hardware/interfaces/+/5688f7eb1e117ed26e642a695de300b7683acb87/security/keymint/aidl/android/hardware/security/keymint/HardwareAuthToken.aidl) | FULL_TEXT_READ :1–85; HAT challenge/types/SIDs/clock/MAC; не execution |
| A4 | [IKeyMintOperation.aidl](https://android.googlesource.com/platform/hardware/interfaces/+/5688f7eb1e117ed26e642a695de300b7683acb87/security/keymint/aidl/android/hardware/security/keymint/IKeyMintOperation.aidl) | FULL_TEXT_READ :1–283; per-operation auth; timeout clocks; finish confirmation token checked against signed data; abort/outcome |
| A5 | [Tag.aidl](https://android.googlesource.com/platform/hardware/interfaces/+/5688f7eb1e117ed26e642a695de300b7683acb87/security/keymint/aidl/android/hardware/security/keymint/Tag.aidl) | SECTIONS_READ :335–489 USER_SECURE_ID, NO_AUTH, USER_AUTH_TYPE, AUTH_TIMEOUT, trusted presence/confirmation; 708 challenge declaration found, not full tag audit |
| A6 | [ConfirmationPrompt.java](https://github.com/aosp-mirror/platform_frameworks_base/blob/99b01a65cc4c104933788b3143285ab6bae65827/core/java/android/security/ConfirmationPrompt.java) | SECTIONS_READ :43–145 prompt/callback bytes; :232–335 accessibility/isSupported/present race/throws |
| A7 | [AndroidKeyStoreKeyGeneratorSpi.java](https://github.com/aosp-mirror/platform_frameworks_base/blob/99b01a65cc4c104933788b3143285ab6bae65827/keystore/java/android/security/keystore2/AndroidKeyStoreKeyGeneratorSpi.java) | SECTIONS_READ/targeted symbol search init/generate/flags/security-level, calls addUserAuthArgs; not whole implementation/native assurance |
| A8 | [AndroidKeyStoreKeyPairGeneratorSpi.java](https://github.com/aosp-mirror/platform_frameworks_base/blob/99b01a65cc4c104933788b3143285ab6bae65827/keystore/java/android/security/keystore2/AndroidKeyStoreKeyPairGeneratorSpi.java) | SECTIONS_READ/targeted symbol search generateKeyPair, challenge arguments, addUserAuthArgs caller; not whole crypto-generation audit |

AOSP headers sampled in прочитанных files specify Apache-2.0.
References копируются только в собственные artifacts, не в продукт.
Первый wrong-path ConfirmationPrompt `keystore/java` дал404; исправлено на
`core/java`. Этот404 не считается source verification.

## 4. Exact parser/verifier candidate: Android keyattestation

Организация Android/Google, **Android Key Attestation Verifier**.
[Репозиторий](https://github.com/android/keyattestation), exact reviewed commit
**34652d3df68b8d6887a6670b00eb0856090dae6d**.

- HEAD resolved read-only `git ls-remote`, 13:19Z примерно.
- Source tar downloaded from codeload with exact commit, 13:20Z.
- [README at pin](https://github.com/android/keyattestation/blob/34652d3df68b8d6887a6670b00eb0856090dae6d/README.md)
  FULL_TEXT_READ. Main README re-fetched at pin, bytes equal on comparison.
- [LICENSE at pin](https://github.com/android/keyattestation/blob/34652d3df68b8d6887a6670b00eb0856090dae6d/LICENSE)
  **FULL_TEXT_READ Apache License2.0**, including clauses4/7/8. Dependency licenses
  не audited; compatibility с whole product distribution не certified.
- Последний обнаруженный version tag **v0.4 = 621149c4ebaeeea844d6f43098687c73c4d370a3**,
  отличный от reviewed HEAD. [RELEASE.md at pin](https://github.com/android/keyattestation/blob/34652d3df68b8d6887a6670b00eb0856090dae6d/RELEASE.md)
  FULL_TEXT_READ: test/pre-beta release. Commit date UNKNOWN; не claimed stable
  frozen API/installed dependency.

Каждый относительный source path ниже находится под ссылкой
[exact source tree](https://github.com/android/keyattestation/tree/34652d3df68b8d6887a6670b00eb0856090dae6d).

| Source | Область прочтения / точное следствие |
|---|---|
| `src/main/kotlin/Verifier.kt` | FULL_TEXT_READ :1–371; verify optional challenge, source providers, result model, parsing/constraints/appid. Success не сообщает operation/presence, не вся key policy экспортируется |
| `ConstraintConfig.kt` | SECTIONS_READ :1–240 и245–434; generated origin/nonsoftware/default RootOfTrust; STRICT(expected=null) не requires identity; additionalConstraints seam |
| `Extension.kt` | SECTIONS_READ :62–159, keyDescription parse/type :173–250, AuthorizationList:361–432 и :558–564 inspected; keys/tags NULL/true; not full ASN.1 engine or dependency audit |
| `provider/KeyAttestationCertPath.kt` | FULL_TEXT_READ :1–201; fixed supplied chain profile; no generic automatic path discovery |
| `provider/KeyAttestationCertPathValidator.kt` | FULL_TEXT_READ :1–344; rejects extra target/extension, checks chain signature; factory/RKP validity branch; ATTEST_KEY TODO exists |
| `provider/RevocationChecker.kt` | FULL_TEXT_READ :1–56; supplied serial set is checked; omission/empty set is caller information limit |
| `GoogleRevocationList.kt` | FULL_TEXT_READ :1–88; HTTP fetch, exceptions, filter REVOKED-only; SUSPENDED information lost in this projection; no explicit timeout set in this body |
| `VerifierCli.kt` | SECTIONS_READ :20–95 executable caller; revoked source emptySet and omitted challenge; no operation proof. CLI not executed |
| `build.gradle.kts` | FULL_TEXT_READ :1–144; JVM/JDK21/deps, generated roots task, snapshot version fallback. No resolution/build |
| `README.md` / `RELEASE.md` / LICENSE | FULL_TEXT_READ: intended verifier use; replay/relay caveat; root/status providers; test-release and licence |

**Caveat**: full source repository, dependency closure, numerical/crypto engine,
tests/security correctness не audited. Test files/генераторы не запускались;
внешняя официальная рекомендация «well-tested» отмечается как claim автора,
не как выполненная этим исследователем независимая приёмка.

## 5. Negative searches и ошибки доступа

- Source consumer search — только Android `.kt/.kts`, не реальные profile/vault,
  не deployed runtime. Actual authRequired=false подтверждён файлом, hardware
  level остаётся **UNKNOWN**.
- Initial guessed `google/android-key-attestation` API запрос дал403;
  correct `android/keyattestation` найден по ссылке официальной D1 и pinned.
  Ошибка не названа отсутствием verifier.
- WebFetch отказ/таймаут → официальный HTTPS curl, **не `-k`**; TLS policy не
  ослаблялась. Никаких user-account credential inputs.
- Google status/roots service не query; отсутствует claim «latest revocation
  checked». Сертификаты/key операции не выполнялись.
- PKI/native/device/live/crypto future cases — **NOT_RUN**. Самопроверка файлов
  и хэшей — статическая review-документация, не успешность cryptographic verifier.

Ни внешняя TLS-загрузка, ни чтение spec не являются proof операции; не создают
runtime/device evidence. Полученные временные snapshots источников ограничены
датами в этой таблице и exact pins, а не будущими состояниями сайтов.

## 6. Итоговая статическая сверка

На **2026-10-27T13:51:09Z** пять scoped Android source inputs совпали с
сохранёнными hashes; relative local paths восьми ссылок пакета существовали.
Это не полный anchor/external-link/crypto test. Metadata review и hashes
сохранены в `review-ready.json` / `output-seal.json` artifacts пакета.

Предлагаемые выводы R05-009-A/B/C в REPORT не повышают статус никакого старого
пакета. Общие STATE/REPORT/SOURCES/индексы/планы и чужие файлы read-only.

<a id="7-fup-6-01--status-projection-scoped-повторное-чтение-до-cutoff"></a>
## 7. FUP-6-01 — проверка status projection

Дополнение от **27 октября 2026**: source/schema партия примерно
14:45–14:50Z. **PARTIAL_AT_CUTOFF**: законченный consumer-adapter/error/freshness
contract не заявлен. A/B/C и их источники этим уточнением не отменены.

Новых внешних источников и запросов не требовалось: использованы ранее
captured официальные D1 RU/EN и source archive exact commit
**34652d3df68b8d6887a6670b00eb0856090dae6d**.

| Ранее captured источник | Что повторно прочитано до срока |
|---|---|
| D1 Certificate revocation status list / schema | EN capture :200–255, пример :257–272, revocation publication policy :289–316; REVOKED/SUSPENDED, Cache-Control, optional expires. Endpoint NOT_FETCHED |
| `GoogleRevocationList.kt` | FULL_TEXT_READ :1–88: HTTP200 guard, disconnect finally, DTO String status, REVOKED-only filter; schema validation/freshness metadata не передаются |
| `Verifier.kt` | SECTIONS_READ :44–68 и140–371: Set<String> callback; status-source invocation перед validate catch; external catch CertificateException; Success без status provenance/as-of |
| `provider/RevocationChecker.kt` | FULL_TEXT_READ :1–56: serial membership и exception REVOKED |
| `provider/KeyAttestationCertPathValidator.kt` | SECTIONS_READ :44–268: basic/checkers path; exception propagation into path failure |
| `provider/KeyAttestationCertPath.kt` | SECTIONS_READ :50–84: chain representation и serial formatting |
| `VerifierCli.kt` | SECTIONS_READ :60–89: explicit emptySet status provider, no challenge; narrower result |
| README | SECTIONS_READ :145–173: roots/status providers, свежесть списка заявлена в тексте, не в Set<String> type |

`fup-6-01/inputs.json`: **6/6** участвующих source files совпали с прежним
pinned tar archive при побайтном сравнении; это локальная identity comparison,
не новая независимая remote re-acquisition. `fup-6-01/baseline-docs/` сохраняет
пакет до добавления D. Никаких JSON/сертификатных fixtures или Gson/PKIX execution.
Malformed/null/wrong-type semantics Gson не исследованы полностью и не заявлены
как проверенные. Условный unknown-string путь ограничен случаем, в котором
строка успешно десериализована; это не assertion об актуальном JSON endpoint.

До срока подтверждены:

- SUSPENDED/успешно прочитанные unknown strings исключаются проекцией;
- отсутствие serial в supplied set не различает nonlisted, discarded status и
  отсутствие проверки у caller, который сам подал пустой set;
- exception HTTP/parse не становится штатным emptySet success; универсального
  типизированного status-error результата в traced Verifier catch нет;
- возраст и происхождение snapshot не представлены Set<String>, а certificate
  evaluation date не замена snapshot age.

Полный non-lossy consumer contract и проверка schema/parser corner cases
остались незавершёнными. Результат D — bounded source/model clarification,
не ACCEPTED/bypass/native PASS. Latest status/реальные цепочки не запрашивались.

<a id="8-новая-ограниченная-задача-28-октября-gson-и-http-cache-contract"></a>

## 8. Дополнение 28 октября: Gson и HTTP cache contract

Дополнение от **28 октября 2026** относится к остатку FUP-6-01.
Исторический §7 и его **PARTIAL_AT_CUTOFF** сохраняются отдельно.

Новый каталог evidence:
`renewal-2026-10-28/` внутри `.artifacts/research/theory-05/2026-10-27-keystore-attestation-res6/`.

### 8.1 Source identity и область нового чтения

Candidate verifier pin не меняется: **34652d3df68b8d6887a6670b00eb0856090dae6d**.
Его `build.gradle.kts:32` фиксирует **Gson2.11.0**. Новая партия не подменила
этот pin latest branch/другой версией parser и не изменяет dependency manifests.

**google/gson**, published tag `gson-parent-2.11.0`:

- annotated tag object: **3b7c2ad8ce8dd471449ccbaee6d22f64c9816a38**;
- peeled commit: **828a97be0f8d58108b140b77df8dc76b657f4a87**;
- resolve read-only `git ls-remote`, exact codeload source fetched
  **28.10.2026, примерно06:44–06:45Z**;
- release publication/commit calendar date: **UNKNOWN** в этом срезе;
- [LICENSE](https://github.com/google/gson/blob/828a97be0f8d58108b140b77df8dc76b657f4a87/LICENSE)
  **FULL_TEXT_READ Apache-2.0** (2026-10-28); Gson dependency closure licences
  не audited, Maven-byte identity/compiled artifact не проверены.

Sources G1–G6 ниже опубликованы Google/Gson maintainers, exact source branch
commit указан в каждой ссылке. Чтение **SECTIONS_READ**, 28.10.2026 примерно
06:44–06:48Z. Manifest and source captures/hash registry: `gson-inputs.json`.

| ID | Exact source / прочитанная область | Вывод, который источник поддерживает |
|---|---|---|
| G1 | [Gson.java](https://github.com/google/gson/blob/828a97be0f8d58108b140b77df8dc76b657f4a87/gson/src/main/java/com/google/gson/Gson.java):155–161 defaults, :1259–1265 Reader overload, :1345–1381 read, :1466–1475 full consumption | new Gson default implicit strictness/Unsafe; legacy lenient read; EOF/null semantics; IO→JsonSyntax wrapper; Reader caller имеет full-consumption guard при non-null root |
| G2 | [ReflectiveTypeAdapterFactory.java](https://github.com/google/gson/blob/828a97be0f8d58108b140b77df8dc76b657f4a87/gson/src/main/java/com/google/gson/internal/bind/ReflectiveTypeAdapterFactory.java):148–161 normal vs Record; :244–279 field write; :498–564 read loop/accumulator | Normal reflected DTO не invokes Kotlin primary-constructor required checks; null reference write разрешён; unknown fields skip; duplicate known DTO fields читаются повторно |
| G3 | [TypeAdapters.java](https://github.com/google/gson/blob/828a97be0f8d58108b140b77df8dc76b657f4a87/gson/src/main/java/com/google/gson/internal/bind/TypeAdapters.java):458–478 String adapter; [JsonReader.java](https://github.com/google/gson/blob/828a97be0f8d58108b140b77df8dc76b657f4a87/gson/src/main/java/com/google/gson/stream/JsonReader.java):890–921 nextString | STRING допускает null/boolean coercion; nextString принимает numeric tokens; object/array не String |
| G4 | [MapTypeAdapterFactory.java](https://github.com/google/gson/blob/828a97be0f8d58108b140b77df8dc76b657f4a87/gson/src/main/java/com/google/gson/internal/bind/MapTypeAdapterFactory.java):172–209 read | Null map; array-of-pairs support; ordinary duplicate-key guard есть при non-null previous value, не эквивалент duplicate detection всех object fields |
| G5 | [ConstructorConstructor.java](https://github.com/google/gson/blob/828a97be0f8d58108b140b77df8dc76b657f4a87/gson/src/main/java/com/google/gson/internal/ConstructorConstructor.java):105–176 selection, :412–445 Unsafe branch | No-args/instanceCreator/Unsafe options; allocation not universal runtime promise. UnsafeAllocator captured как navigation, не full mechanism audit |
| G6 | [GsonBuilder.java](https://github.com/google/gson/blob/828a97be0f8d58108b140b77df8dc76b657f4a87/gson/src/main/java/com/google/gson/GsonBuilder.java):552–568 strictness method | Strictness.STRICT available since2.11.0; настройка syntax не заменяет schema/enum/required/type/duplicate validation |

Не выполнялись Kotlin compilation/Java/Gson calls, parser tests, fixtures,
reflection allocation или arbitrary malformed-input probing. Точные null
exception classes на compiled candidate/Android JVM не аттестованы. Source
matrix conditional на actual ordinary DTO field adapter и разрешённой allocation;
если runtime блокирует reflect/Unsafe — honest failure, не normal status.

### 8.2 Новые нормативные primary sources

| ID | Автор/организация, название, дата и URL | Реальная область прочтения / дата | Применение и предел |
|---|---|---|---|
| H1 | R. Fielding, M. Nottingham, J. Reschke (eds), IETF **RFC9111: HTTP Caching**, **июнь2022**, [официальный текст](https://www.rfc-editor.org/rfc/rfc9111.txt) | **SECTIONS_READ**, 28.10.2026 примерно06:45–06:48Z: §4.2.1 freshness lifetime, §4.2.3 age calculation, §4.2.4 stale reuse, §4.3.4 matching304 validation, §5.2.2.2 must-revalidate и §5.2.2.4 unqualified/qualified no-cache | Receipt time не snapshot origin time; Age+Date+transit/residence; successful matching304 способен refresh cache metadata; cache freshness не instant global knowledge |
| H2 | T. Bray (ed), IETF **RFC8259: The JavaScript Object Notation (JSON) Data Interchange Format**, **декабрь2017**, [официальный текст](https://www.rfc-editor.org/rfc/rfc8259.txt) | **SECTIONS_READ**, 28.10.2026 примерно06:45–06:48Z: §4 Objects/unique names, §8.1 UTF-8, §9 parser limits located | Duplicate names SHOULD unique в RFC; stricter duplicate rejection — explicit application policy, не выдуманный MUST стандарта. Parser acceptance не равен Google status JSON schema |

English originals прочитаны как primary; русский перевод этих RFC **не искался
и не заявлен прочитанным**. Они дополняют ранее реально прочитанную официальную
русскую D1, где JSON status REVOKED/SUSPENDED и Cache-Control documented.
Не копировать сами RFC целиком в REPORT; captures — в ignored own artifacts,
производные фрагменты/paraphrase соблюдают предел источника.

### 8.3 Проверенные предпосылки и честные неизвестные

- D1 status schema взята из сохранённой официальной страницы (2026-07-09
  Last updated). Нового запроса к `/attestation/status` или `/root` нет.
- HTTP Age/Cache-Control/Date конкретного Google endpoint **UNKNOWN**: RFC
  определяет semantics, не доказательство фактического header deployment.
- JSON `expires` — certificate expiration date по D1, не nextUpdate статуса.
- Default parser can coerce/skip/overwrite DTO values при ordinary reflection;
  это source факт, не executed bypass against a real certificate.
- Existing source guards (HTTP200/throw; map duplicate detection; Reader EOF
  consumption; path signature/constraints) сохранены в анализе. Нельзя
  заменить их blanket фразой «библиотека fail-open».
- Trust root excluded from CertPath certificate list отдельно; status checker
  path membership не доказательство latest root retirement. Named trust-anchor
  provider/accepted roots policy остаётся самостоятельным gate.
- Закончена bounded source/spec/model версия consumer contract REPORT §10:
  Acquisition, SnapshotEvidence, StatusOf, FreshnessDisposition, Decision.
  Это не код/protocol/бенчмарк, не current native protection и не ACCEPTED.
- Прежние A/B/C/частичный D и PARTIAL_AT_CUTOFF сохранены как текстовый префикс;
  новый результат относится к новой задаче28октября, не задним числом старого окна.
- Итоговый cut **2026-10-28T06:56:36Z**:10/10 Gson source/manifests hashes
  unchanged; прежние3 авторских документа prefix-exact,8 локальных relative
  paths без отсутствующих targets. `review-ready.json` и финальный
  `output-seal.json` — own artifacts. Не full anchors/external URLs/crypto test.



