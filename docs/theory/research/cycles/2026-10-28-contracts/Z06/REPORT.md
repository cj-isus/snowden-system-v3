# R28-Z06 — Five-result revocation contract: достаточно ли классификации для operation gate?

28 октября 2026. ID **R28-Z06**.
Статус: **READY_FOR_REVIEW**. Все execution/native/crypto cases **NOT_RUN**.
Источники и область чтения: [SOURCES](SOURCES.md).

## 1. Короткий ответ и граница независимости

**Пять результатов достаточны как словарь различий, но не как самостоятельное
доказательство согласованного допуска операции.** Нужны отношения между ними:
один immutable evidence bundle, точная identity subjects/policy/provider,
повторная оценка времени и accepted history на gate, запрет late completion
после смены head/intent и явная граница самого действия.

Рекомендуемая disposition к baseline R05-009-D:

- **source/parser/lossiness часть — поддержать в её условной области**;
- **five-result taxonomy — сохранить** без требования создать шестой тип;
- **утверждение достаточности до operation gate — NEEDS_REVISION**, если оно
  означает, что раздельно correct типы автоматически дают coherent execution;
- расширенное прочтение «current authority проверяется на самом gate под
  согласованными inputs» **не опровергнуто**: оно уже запрещает strongest trace.

Эта сессия участвовала в написании baseline R05-009-D. Поэтому результат —
повторная source-oriented самопроверка с новыми primary captures/opposing cases,
**не внешняя независимая рецензия**. C05 прочитан только адресно как чужой draft;
независимый ACCEPTED/NEEDS_REVISION у руководителя этим не устанавливается.

## 2. Что перепроверено, а что unavailable

Канон и источники доступны в D:/SNOWDEN-SYSTEM-V3. Actual upstream
**android/keyattestation**, commit **34652d3df68b8d6887a6670b00eb0856090dae6d**,
и **Gson2.11.0**, commit **828a97be0f8d58108b140b77df8dc76b657f4a87**,
повторно получены по immutable raw URLs. У10 критических файлов fresh bytes
совпали с прежними captured source; это новая source byte equality, не runtime
proof. Verifier manifest объявляет Gson2.11.0/Kotlin2.2/JDK21. Maven artifacts/
собранный EXE/APK, actual JCA provider/device/config **не аттестованы**.

В scoped Android `.kt/.kts` search нет consumer этого attestation-status verifier;
ProfileStoreCrypto — отдельный AES/GCM storage helper. Его действие не следует
из proposed five-result contract. Не выдумывается production path, в котором
новая классификация уже применяется. Source example below — upstream caller,
operation sequence — логический противник proposed consumer.

## 3. R28-Z06-A — exact parser→DTO→projection→checker, safeguards сохранены

### 3.1 Фактический caller

`getGoogleRevocationStatusFromWeb` → `getRevocationStatusFromWeb` → HTTP200
проверка → `parseAttestationStatus(InputStream)` → `new Gson().fromJson(Reader,
StatusFile.class)` → `entries.filterValues(status == REVOKED).keys` → callback
`() -> Set<String>` в `Verifier.internalVerify` → `RevocationChecker` в PKIX
parameters → path checker → `PathValidationFailure` либо generic `Success`.

Точный status source error возникает **до** try/catch вокруг path-validator.
Внешний verify ловит CertificateException, но не каждый IO/JSON runtime exception.
Поэтому thrown acquisition error прерывает проверку, а не становится success;
полного typed status-error result этот API не гарантирует. Проекция old/custom
caller error→emptySet является отдельной возможной ошибкой, не найденным guard
в inspected helper.

### 3.2 Guard crosswalk (V1/G1–G5)

| Boundary | Подтверждённый safeguard / scope | Нельзя выводить |
|---|---|---|
| HTTP | !=200 → IOException; disconnect finally | HTTP200 сам удостоверяет freshness/schema или correct final source после redirect |
| Decode root | Gson пустой/null root может вернуть null; subsequent entries access может failure | Kotlin non-null type валидирует JSON required fields |
| String field | STRING допускает null/boolean coercion; nextString numeric token | Strict syntax автоматически проверяет enum String и обязательность |
| Map | array-of-pairs допустим; duplicate key с previous non-null бросает exception | «Любой duplicate принят» либо «все повторённые DTO members запрещены» |
| DTO fields | Reflection field read/write повторно применяет known member, unknown skip | Schema additionalProperties/duplicate validation уже выполнена |
| EOF | Reader overload вызывает assertFullConsumption при non-null decoded root | JsonReader overload и Reader обладают одним и тем же EOF promise |
| REVOKED | entry сохраняется в set; subject membership → path exception | SUSPENDED/unknown/age сохраняются после filter |
| Chain | Подписи/называемые path constraints есть; appended target/extension отвергаются | Generic Success означает PoP/presence/current operation permit |
| Anchor | trust anchor dropLast из path certificates, separate trustAnchorsSource | Path membership checker проверил current root retirement целиком |

**Условные malformed source выводы:** regular Kotlin DTO с constructor args без
custom type adapter обычно получает field-reflection/Unsafe construction; null
required reference не обязательно бросает на этапе decode. Если reflection/
allocation запрещены actual runtime, это failure, не обещание silent acceptance.
Compiled DTO/JVM/R8/desugaring не выполнялись.

### 3.3 Самая сильная простая lossiness модель

Для valid status documents определим `P(D) = {s | status_D(s)=REVOKED}`.
D0 не содержит s; D1 содержит SUSPENDED(s), остальных REVOKED различий нет.
Тогда P(D0)=P(D1), хотя selected policy может дать разные outcomes.
Функция только от P(D) не восстановит discarded status. Для этого вывода не
нужны null/малформированные данные, unsafe exploit либо подозрение о Gson.

Five-result классификация исправляет именно этот предел **если** StatusOf
происходит из полного проверенного D до P и сохраняет происхождение; из уже
returned set создать честный StatusOf(SUSPENDED) невозможно. Acquisition failure
не принадлежит domain valid documents, поэтому не добавляется к NotListed.

## 4. R28-Z06-B — точный strongest opposing input sequence

### 4.1 Новая дельта: правильные типы ещё не связанные доказательства

Обозначения, только model:

- E — immutable evidence candidate: source/scope/representation/validation data;
- H — **локально принятый head** status history и named supersession policy;
- Q — current root/key/action policy revision;
- I — operation intent/session incarnation;
- S — точный subject set chain+selected trust anchors;
- t — gate time с named clock/continuity; не arbitrary global clock.

Baseline перечисляет Acquisition, SnapshotEvidence, StatusOf, FreshnessDisposition,
Decision. Он также говорит сохранять current authority/accepted deny. Но нет
явно проверяемого равенства **какое решение выведено из какого E/H/Q/I** и
линейной точки, в которой elapsed-time/новый accepted deny применяются к действию.
Надпись «immutable» в описании snapshot не запрещает caller брать каждый
result из отдельной mutable cache cell; Set<T> в Kotlin не deep immutable.

### 4.2 Trace T1: корректный callback прошлого snapshot после нового deny

| Шаг | Событие | Сохранившиеся значения |
|---|---|---|
| 1 | В request I принят supported snapshot E0; s NotListed; допустимая давность, Q0 | Acquisition(E0), StatusOf(s,E0), Freshness(E0,t0), generic chain C0 корректны |
| 2 | Async chain verification по E0 ещё работает; snapshot bytes не повреждены | E0 truthful as-of proof, не magic current status |
| 3 | Другой refresh завершён и **принят E1**, где s REVOKED; H становится H1 | Accepted deny принадлежит current history |
| 4 | Следующий refresh даёт read/HTTP error; wrapper честно сохраняет Unavailable | Error не отменяет E1; старый E0 может оставаться архивом |
| 5 | Поздний callback C0/E0 возвращает Success. Decision берёт cached NotListed и старую Freshness либо ставит latest Acquisition(error) рядом с E0 | Все labels могут быть правдиво типизированы, но evidence join несогласован |
| 6 | Caller отправляет новое status-dependent действие без проверки H1/Q/I и gate time | Новый accepted deny фактически проигнорирован |

**Результат:** классификация не обеспечивает permit. Это countermodel для
недостаточно определённого caller, **не воспроизведённый bypass Snowden/Google**.
E0 chain authenticity не ложна: ложен перенос её narrow result в current permit.

**Сильнейшая противоположная защита:** если baseline-фразу «current authority»
понимать как gate-атомарное сравнение H/Q/I плюс статусный S(E) и пересчёт
freshness, шаг 6 невозможен. Тогда trace не опровергает полную logical conjunction,
а показывает недостающее operational definition её predicates. Вердикт
NEEDS_REVISION scoped sufficiency, не отбрасывание entire five-result разделения.

### 4.3 Trace T2: отсутствие нового deny, но разрешение устарело в очереди

At t0 E0 допустим по выбранному Δ. Verification/очередь задержались до t1,
когда freshness predicate уже false, H/Q/I не менялись. Caller кэширует
`FreshnessDisposition.AcceptableAsOf` как бессрочный bool и выполняет действие.
Все equality E/H/Q/I есть, **одного head/CAS check недостаточно**. Gate должен
проверить named age bound/deadline **в t1**, не повторять acquisition ради формы.
Если defined policy разрешает grandfathered continuation grant, это другой
scope, не новое «свежее» разрешение.

### 4.4 Trace T3: данные чекера изменились, identity snapshot осталась прежней

RevocationChecker хранит supplied Set<String> reference. Custom source может
возвращать mutable backing set; Kotlin read-only interface не запрещает mutation
через другой alias. После вычисления digest/metadata E0 writer меняет набор,
checker читает новые membership values, Decision ссылается на старый E0.
Snapshot hash не fences underlying borrowed bytes/collections. Нужны owned
immutable copy или эквивалентная custody, не название `val` или Set.
Подтверждается API source+Kotlin collection semantics; custom caller exploit
не исследовался, данный сценарий не приписан default fetcher как факт.

## 5. Минимальная поправка: один witness bundle и gate, не шестая система

Требования ниже **PROPOSED**, ни новой БД/схемы каталогов, ни production fields.

### 5.1 Связанный witness

Логически W содержит references/identity:

`W = (E, S, H_at_evaluation, Q, I, chain_binding, clock_domain, expiry_policy)`.

Все five results либо производные от W, либо отказ **этого же attempt**.
Они не соединяются как independently latest values. Вариант допускаемого cache
fallback сохраняет одновременно latest-attempt error и выбранный **старый E**;
его статус, область и возраст не заменяются свежестью failed attempt.

Жёсткие bounds на request/subject set/ingress требуются до allocation. Подробность
retained resource scope — в §8; число типов не RAM-cap.

### 5.2 Gate admission и истина source

Достаточный условный gate на **новое** действие:

`Permit(W,t) ⇒ SameSubjectAndSourceScope(W)`

`∧ ImmutableEvidenceUsed(W)`

`∧ AcceptedHeadCompatible(H_now,H_W,Q_now)`

`∧ CurrentIntent(I_now,I_W)`

`∧ FreshnessStillValid(E,t,clockPolicy)`

`∧ StatusPolicyPermits(S,E,H_now,Q_now)`

`∧ ChainAndActionConstraints(W,Q_now)`.

AcceptedHeadCompatible не требует всегда identical E: policy может явно
разрешать старый still-acceptable snapshot, **не отменяющий новый accepted deny**.
Переход REVOKED/SUSPENDED→released определяется source-specific supersession,
а не вечным union, не hash order или новым capturedAt.

При неопределённом H/reset/custody — fail/unknown для нового permit.
«Недоступен snapshot» не говорит удалить старую историю; это input failure.
Действие принято gate-ом и действие физически исполнено — разные evidence.

### 5.3 Linearization и pending external effect

Одного `AtomicReference.compareAndSet` достаточно только для atomic смены
ссылки в памяти (JDK21 API), не для неизменности её reachable data, файловой
durability и тем более HTTP/OS action. Возможный ready pattern:
owned immutable head + monotonic local incarnation + existing owner mutex/CAS.

**Порядок policy:** если revoke принят до atomic admission — старое W reject.
Если admission случился первым, дальнейшая отмена/resume/side effect определяется
explicit in-flight rule платформы. Нельзя обещать instant global revocation
между admission и irreversible effect без authority, контролирующей этот effect.
Нельзя совершать внешний irreversible effect внутри updateAndGet callback:
JDK API допускает повторное выполнение функции при CAS contention и требует
side-effect-free callback. Atomic receipt/admission не actual outcome.

**ABA/restore:** revision/incarnation не повторяется для нового local authority
state, даже если bytes/Set/ETag вновь те же. Restore процесса/root lineage может
инвалидировать старое W. Без independent accepted history невозможно отличить
полный rollback state+checkpoint от чистого старого snapshot; это unknown,
не изобретённая защита новым counter.

## 6. Связанные fallback/retry/cache questions (TASK continuation выполнен)

### T4 — типизированный error снова схлопнулся при fallback

Attempt a1 закончился SourceUnavailable. Старый E0 ещё в cache.
«last successful result» UI/decision helper возвращает Success(E0) и забывает
ошибку a1. Запрет error→emptySet не помогает: error→oldSuccess — другая lossy
проекция. Correct contract сохраняет selectedEvidence=E0, latestAttempt=a1/failure,
old-as-of и gate-permission; допустимая политика cache reuse может permit E0,
но не report fresh successful acquisition a1.

### T5 — cache restore с новым receivedAt не новая authority

Disk E0 восстановлен после reboot; counter/elapsed origin старого boot не
continuously accepted. Новый capturedAt не даёт новый validation time.
Freshness→UnknownAge или explicit accepted durable-clock model.
Если старый deny тоже восстановлен без independently current head, exact
recovery possible state неизвестен. Новый user presence не authenticates
freshness/head. Existing checkpoint code reject uncertain state — useful guard,
не full solution rollback adversary.

### T6 — opaque ETag принят за монотонный revision

RFC9110 §8.8.3: ETag opaque representation validator. Lexicographic/numeric sort
по tag не определяет новизну head и не заменяет status supersession order.
Повтор body hash/ETag тоже не proves no newer accepted deny.
Conditional304 подходит только matching response по URL/method/Vary/validators
(RFC9111 §§4/4.1/4.3.4). Actual endpoint headers **UNKNOWN**; basic inspected
fetcher принимает только HTTP200 и не устанавливает conditional validators.
Правильный cache layer может дать reused200; фактический provider cache unknown.

### T7 — partial semantic snapshot при valid JSON

Valid syntactic document может быть semantically incomplete из-за неверного
source/scope/parser schema, selection без всех required subjects либо фактически
partial transfer. Parser EOF guard не source completeness certificate.
RFC9111 §3.3 разрешает хранить incomplete200, но требует маркировки и не позволяет
его использовать как complete response. Selected subject absence даёт NotListed
лишь при доказанной применимости complete source profile. Полный «current list
всех keys мира» Google JSON тоже не обещает; historical source coverage отдельна.

### T8 — root и certificate status не универсально один serial

У candidate root excluded from CertPath checkers. Расширенный consumer должен
определить subject scope issuer/key/root family; global decimal serial не identity.
Google map — конкретный published profile. Fresh root retirement inputs отдельны;
current root policy не из наличия Google-root fingerprint в старой chain.

## 7. HTTP freshness: подтверждённые части и требуемая precision

RFC9111 §4.2.3 определяет estimated HTTP response age:

`current_age = max(max(0,response_time−Date), Age+response_delay) + resident_time`.

Это conservative cache calculation при RFC assumptions, не безусловный
криптографический upper bound на knowledge age. Δ/clock bounds здесь UNKNOWN,
не 0. `expires` внутри status JSON — certificate expiration date, не status TTL.

Freshness вычисляется при selected gate t, не раз и навсегда at parsing. Successful
matching304 может обновить response headers/validation evidence, не requires
новых bytes; failed request не freshens old E. Unqualified no-cache и
must-revalidate obey their semantics; qualified no-cache имеет narrower header
scope. RFC9111 §6 не автоматически определяет application admission/recovery:
HTTP cache policy и permits приложения необходимо сопоставить explicit.

Из currentAge alone не следуют newly published/unpublished revocations, historical
absence или право particular action. Именно это сохраняет fifth Decision layer.

## 8. R28-Z06-C — готовое reuse: guards, licence и bounded cost

### 8.1 Малый вариант без второго JSON stack

**Gson2.11.0**, exact pin828a97be…, Apache-2.0. Ready primitives:
`JsonReader.setStrictness(STRICT)`, token lookahead, explicit object/scalar read,
required fields/duplicate-name tracking и full-document consumption в scoped
TypeAdapter/ingress validation. Не пользоваться default String adapter для
strict typed status (он coercive), не читать complete tree перед duplicate check.

Это reuse parser, а не новая crypto/protocol implementation. Но полный custom
TypeAdapter отсутствует в reviewed candidate; его correctness требует отдельной
разрешённой finite matrix. Сложность небольшая fixed schema, цена — поддерживать
schema revision/enum/UTF8/field bounds и ясные errors. Готовую end-to-end
schema-valid функцию не заявляем без реализации/acceptance.

### 8.2 Готовый универсальный schema вариант — условно/defer

**networknt/json-schema-validator1.5.9**,
commit **270df62471e6a0c564dc13ae6cf2bf4116916f82**, Apache-2.0.
Manifest Java8, Jackson databind/yaml2.18.3, itu1.14.0, slf4j2.0.17;
optional regex engines не нужны simple canonical-serial regex.
Ready API `JsonSchemaFactory.getInstance(VersionFlag.V7,customizer)` /
`getSchema` / `JsonSchema.validate(JsonNode)`; errors non-empty не allow.
Default typeLoose=false/nullableKeywordEnabled=false; required/type/enum
combined, одна required не rejects null. Explicit local reviewed schema,
no runtime remote `$ref`; MapSchemaLoader плюс terminating
`DisallowSchemaLoader` после whitelisted local refs. Default loader fallback
может открыть classpath/URI; empty local lookup сам не prohibits network.

**Jackson-core2.18.3**, commit
**cc34e8d5b75d4c911c16f2bc87d6e13e42eafddd**, Apache-2.0:
STRICT_DUPLICATE_DETECTION explicit enable (default=false) **до** object tree;
`StreamReadConstraints` document/name/string/number/depth/token bounds; byte-stream
size cap вне parser также. Ready wrapper через JsonFactory builder; nextToken EOF
проверяется явно. API constructors/read mode/actual enforcement reviewed source,
not run. Parser defaults не sufficient resource caps.

**Strong opposing case для schema reuse:** pinned
AdditionalPropertiesValidator:116–119 **skip всех pname с `#`**.
Объект root с required valid entries плюс unknown `#shadow` может пройти этот
keyword, хотя Google schema root additionalProperties=false. Другие explicit
scope/schema/ingress guards могут reject; blanket «generic schema validate всегда
точно закрывает Google profile» этим source не установлен. PropertyNamesValidator
внутри entries перебирает все имена — existing counter-guard для invalid serial,
но не заменяет запрет unknown root/entry properties, где такого propertyNames нет.
Следовательно full-library schema guarantee **DEFER**, или отдельный reviewed
whitelist ingress guard before validation; не чинить upstream в этом задании.

Цена: второй Jackson tree/validator stack и error data vs reuse existing Gson;
classpath/remote loader controls; node/string/regex/validation-message allocations.
LICENSE не correctness/security audit, dependency closure не audited. Rooted
Key Attestation verifier сам JDK21/Kotlin2.2 — не drop-in Android Java17 promise.
Рекомендуемый выбор для одного fixed status profile: existing strict streaming
Gson + small, отдельно принимаемый adapter; generic validator нужен лишь при
реальном требовании many-schema reuse. Не вводить новую зависимость сейчас.

### 8.3 Bounded ресурсы — требуемые параметры, не найденные числа

B — max ingress bytes, N — max serial entries, L — max names/strings,
D — max depth, A — bounded concurrent attempts, R — bounded retained snapshots.
Два caps для streaming duplicate detection/body ownership:
`O(B + N·L)` active memory under bounded parser/duplicate sets;
`O(A·(B+N·L) + R·B)` для авторской модели ограниченного числа generations.
Это не фактический JVM heap/RSS bound: headers, decoder, exception messages,
validator node overhead, source queues/thread stack и pool retention отдельно.
При unidentified A/R/cancellation actual return global bound **UNKNOWN**.

Schema maxProperties, настроенный лишь после complete tree parse, не защищает
раннюю allocation. Content-Length только claimed size; bounded decoding нужен
по **фактически прочитанным** bytes с пределом compressed expansion при таком
transport. Error data не обязаны включать весь source. В ready Jackson docs
default document/token limits unlimited; name/string defaults велики. Значения
B/N/L/A/R владелец задаёт из purpose, не из отсутствия измерения.

Read-timeout отдельно не overall deadline (trickle data продлевает операцию).
Google helper не задаёт own timeouts; JDK docs говорят 0 infinite, но actual
custom provider defaults неизвестны. После caller cancellation старый future
может закончиться; revoked attempt generation fencing обязателен независимо
от promise cancel. Никаких размеров/rate/benchmarks во время работы не измерялось.

## 9. Применение платформе и что уже работает в source

Android EncryptedProfileStore:63–87 требует expected revision/generation,
сохраняет enrollment/floor; :111–123 retained uncertainty, :129–144 pending-stage/
missing-key errors не превращает в empty store. Existing model owner/custody
паттерн пригоден для reuse. Это **не** готовый storage для Google status list,
не proof rollback protection real disk и не permission автоматически расширить
checkpoint state. Source только адресный read-only; facts captured.

Новая status-dependent manual operation может использовать существующий
platform owner/state fence; pure classification layer не владеет OS action.
Windows/provider native/image scopes как RELEASE-EVIDENCE-BOUNDARIES требуют
separate inputs/receipt; этот пакет не обещает WinVerifyTrust/native checks.
Unattended VPN recovery не ломается от optional attestation source failure.
Различать продолжение уже принятого bounded режима и новое operation grant.

Model supplied head/intent подтверждение требуется **в точке admission**;
не нужно считать local counter authoritative anti-rollback при полном откате
всех store/процесса. Source status head ordering либо serial-specific deny
supersession задаётся существующей platform policy; opaque HTTP ETag её не даёт.

## 10. Пройденные стадии, scoped рекомендации и future cases

| Stage TASK | Сделано / evidence |
|---|---|
| Source/pin/caller | Fresh exact-commit source bytes, manifests, guards; actual integrated Snowden attestation consumer не найден в scoped search |
| Основной аргумент | Taxonomy vs related witness distinction; lossiness theorem сохранён, «типы достаточны» опровергнуты model T1/T2 |
| Opposing cases | Existing current-authority guard и grandfathered policy не игнорируются; T1–T8 отделены от runtime findings |
| HTTP/schema | RFC9110/9111/8259 + current official RU schema; endpoint headers UNKNOWN, latest status не query |
| Reuse/cost | Gson small adapter vs networknt/Jackson exact pins/licenses; additionalProperties `#` qualifier и local-schema loader guard; resource premises |
| Продолжение | Callback/cache error-to-oldSuccess, restore/ABA, time drift, partial response и gate cancellation рассмотрены |
| Paired input | C05 TASK и текущий REPORT §§2–3 адресно прочитаны; aligned subject/request qualifier, no agreement-as-proof; finished HANDOFF availability отдельно в seal |

Предлагаемые verdicts: **R28-Z06-A — поддержать scoped source/model**;
**R28-Z06-B — NEEDS_REVISION sufficiency claim с relational/gate requirements**;
**R28-Z06-C — REUSE_GSON_PRIMITIVES / DEFER_GENERIC_SCHEMA_GUARANTEE**, не
автоматический перевод исходного пакета в ACCEPTED.

Все future cases ниже **NOT_RUN**, fixtures не создавались:

- F1: valid absent/SUSPENDED same revoked set, сохранять policy distinction.
- F2: null/coercion/duplicate DTO vs duplicate map, EOF guard и parser configuration.
- F3: older E0 verification pending → accepted E1 deny → error → late E0 callback.
- F4: freshness expires during queue while E/H/Q/I unchanged.
- F5: shared mutable set изменён после snapshot/chain evaluation.
- F6: error-to-oldSuccess cache fallback retains old as-of and latest failure.
- F7: revalidation wrong URL/Vary/validator vs matching304, actual headers pinned.
- F8: process reboot/ABA/cache restore with head identity/clock continuity unknown.
- F9: generic schema root `#shadow` and explicit ingress whitelist counterguard.
- F10: bounded actual bytes/tokens/error retention vs maxProperties after tree allocation.
- F11: root provider preference/retirement отдельно, no magical whole-chain latest state.
- F12: admission CAS before external action with simultaneous deny; define in-flight
  outcome and cancel/uncertainty, не double effect from retry function.

Завершён scoped source/model разбор. Не установлены actual endpoints/status/
certificates, current production caller, whole closure/JVM artifacts/device
compatibility, global RSS/CPU/time guarantee, доступность сервера и live VPN path.
