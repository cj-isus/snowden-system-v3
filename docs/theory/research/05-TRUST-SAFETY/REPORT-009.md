# R05-009 — Android Key Attestation, операция с ключом и подтверждение пользователя

27 октября 2026. **READY_FOR_REVIEW**, не ACCEPTED и не native/product PASS.

Задание: [NINE-WINDOW, редакция 2, §6](../NINE-KNOWLEDGE.md).
Источники и области чтения: [SOURCES-009](SOURCES-009.md).
Паспорт передачи: [R05-009](handoffs/R05-009.md).

## 1. Пробел и проверенный baseline

R05-004 отделяет доверенный выпуск/установку/исполнение от adoption.
R05-005 отделяет допустимый as-of статус от globally latest revocation.
Эти различия сохраняются. R05-008, однако, недостаточно точно описывает
BiometricPrompt как «confirmation подписан keystore-ключом»: локальная
аутентификация, криптографическая операция и подтверждение текста — разные
события. Строка о hardware Keystore также не доказывает, что конкретный ключ
продукта hardware-backed. Здесь эти предпосылки проверены заново, а не приняты
как аксиомы; R05-008 и чужие документы не исправлялись.

**Новый срез:** определить, какой объект удостоверяется на каждом шаге, кто
проверяет evidence, какое enforcement действительно требуется и почему успех
одного шага не заменяет следующий.

### Реальные исходники Snowden доступны в этой рабочей папке

Это `D:/SNOWDEN-SYSTEM-V3`, не документационная копия Obsidian.

- `android/app/build.gradle.kts:10–18`: compile/target SDK 36, min SDK 29;
  Java toolchain 17 (:21–29). Manifest/source — не compiled APK evidence.
- `ProfileStoreCrypto.kt:21–33`: AndroidKeyStore **AES-256/GCM**; purposes
  ENCRYPT|DECRYPT, randomized IV; `setUserAuthenticationRequired(false)`.
  Комментарий :29–30 прямо связывает отсутствие per-use биометрии с
  unattended VPN recovery. Это намеренное требование, не новый дефект.
- `seal/open` (:39–68) пользуются JCA Cipher. В `createEnrollmentKey` нет
  attestation challenge, StrongBox request, KeyInfo inspection или signing key.
- `EncryptedProfileStore.kt:96–102`: ключ создаётся только при первом
  enrollment, иначе берётся existing key; `load:129–144` не пересоздаёт ключ;
  `paths:149–154` требует CE/unlocked user и `noBackupFilesDir`.
- Scoped search `android/**/*.kt, *.kts` по Attestation/BiometricPrompt/
  ConfirmationPrompt/KeyInfo/setIsStrongBoxBacked не обнаружил этих consumers;
  единственный фактический user-auth builder — ProfileStoreCrypto:31.
  Отрицательный поиск не является доказательством отсутствия APIs в будущих
  artifacts/зависимостях либо всех возможных Android-потребителях.

Адресные исходники сохранены с hashes в собственных artifacts. Crypto/KeyStore
методы не вызывались; реальные ключи, цепочки, профили и устройства не читались.

## 2. R05-009-A — четыре разных объекта evidence

### 2.1 Матрица свойств

| Evidence | Что можно вывести при выполненных предпосылках | Чего вывести нельзя |
|---|---|---|
| Проверенная цепочка Key Attestation, выбранные roots/status, ожидаемый challenge и extension | Связь конкретного public key с удостоверенной конфигурацией и authorization lists; challenge включён в аттестацию. Разрешённая давность зависит от verifier/request policy | Свежая user authentication/presence; подпись конкретного запроса; владение приватным ключом предъявителем; выполнение recovery/adoption; latest revocation |
| Успешная операция auth-bound ключом + проверенный результат операции | Операция выполнена ключом согласно проверенным ограничениям и enforcement. Для per-operation — токен авторизует её operation challenge | Пользователь видел/одобрил смысл подписанных bytes; полномочие изменить issuer/root/профиль; remote freshness без request binding |
| TRUSTED_USER_PRESENCE_REQUIRED + операция при доверенном enforcement | Условие physical presence выполнено в security domain, защищающем ключ | Идентичность пользователя, подтверждение текста, наличие универсального канала presence между TEE и StrongBox |
| Protected Confirmation + confirmation-bound signing key + проверенная подпись exact CBOR | Подтверждён показанный **promptText**; подписанный blob также связывает extraData. Высокая уверенность зависит от Trusted UI/KeyMint и проверенной key policy | Пользователь видел extraData; identity без отдельной authentication; конфиденциальность UI/сообщения; свежесть replayed signed blob |

Это логический crosswalk спецификаций [D1–D6, A1–A4], не замеры/OEM guarantees.

### 2.2 Attestation: challenge не является событием присутствия

`KeyGenParameterSpec.Builder.setAttestationChallenge` (API24; AOSP framework
99b01a65…, :1574–1608) кладёт supplied bytes в сертификат при создании **пары**
ключей. Назначение challenge — связать создание с определённым запросом.
Не-null пустой challenge разрешён API, но не создаёт защиту от replay.
`getCertificateChain` возвращает уже имеющуюся цепочку, а не обновляет сведения
о текущем присутствии пользователя или runtime-state.

Схема `KeyDescription` содержит `attestationChallenge`, `attestationSecurityLevel`,
`keyMintSecurityLevel`, `softwareEnforced`, `hardwareEnforced`. Уровень Android
API сам не доказывает TEE/StrongBox данного ключа. Политика проверяет уровни и
конкретные tags в нужном authorization list, а не один boolean «attestation OK».
Software-enforced metadata требует доверия к платформе; hardware-enforced
metadata имеет иной источник/enforcement. Verified Boot fields — тоже
аттестованные сведения, не бессрочный контроль процесса после генерации.

**Контрпример A1:** headless приложение создаёт ключ с authRequired=false и
свежим verifier challenge. Genuine hardware attestation проходит, пользователь
ничего не подтверждал. Следовательно, даже свежая аттестация не implies presence.

**Контрпример A2:** предъявитель пересылает genuine цепочку другого устройства.
Цепочка подлинная, но собственной операции этим ключом он ещё не доказал.
Proof of possession нужен отдельно. Request challenge уменьшает replay, но
сам по себе полностью не устраняет relay; это прямо оговорено pinned README.

### 2.3 Authentication per-operation и timeout-based — не одно событие

AOSP `HardwareAuthToken.aidl` (interfaces **5688f7eb…**, :23–84) описывает HAT:
secure user/authenticator IDs, authentication type, timestamp и MAC. HAT
challenge — значение для авторизации операции, **не** attestationChallenge.
MAC предназначен для secure components; обычный remote verifier не получает
от этого сертификат о человеческом действии и не знает shared HAT secret.

`IKeyMintOperation.aidl:81–146`: при per-operation auth требуется валидный HAT,
совпадающие SID/type и **challenge из BeginResult**. Это enforcement внутри
KeyMint. В timeout-based режиме auth может быть разрешена недавним credential
unlock для нескольких операций, а не новой биометрией на каждую подпись.
Timestamp HAT имеет согласованный secure-environment clock domain (обычно
после boot), а не автоматически UTC/global revocation time.

Framework API30 `setUserAuthenticationParameters(timeout=0, type)` означает
per-use. `KeyStore2ParameterUtils.java:326–339` переводит `0` в **отсутствие**
AUTH_TIMEOUT; отсутствие authRequired даёт NO_AUTH_REQUIRED. Поэтому нельзя
проверять attestation как `authTimeout==0` без учёта различия absent/present и
USER_AUTH_TYPE/NO_AUTH_REQUIRED. Депрецированный API29 duration=-1 отображается
в per-use BIOMETRIC_STRONG; новые API30 combinations проверяются отдельно.

`BiometricPrompt.authenticate(CryptoObject, …)` даёт контекст операции, но
callback `onAuthenticationSucceeded` не является подписанным transaction receipt.
Значимая дистанционная гарантия появляется лишь после проверки результата
операции attested ключом и его policy; если auth requirement software-enforced
или ключ заменён unbound ключом, remote assurance weaker/не установлена.
Framework CryptoObject-интеграция требует strong biometric; runtime API29
имеет отдельные ограничения на device-credential сочетания, поэтому новые
API30+ режимы не переносятся на min29 автоматически [D7].

**Контрпример A3:** пользователь разблокировал устройство; в течение разрешённого
AUTH_TIMEOUT процесс подписал несколько разных сообщений. Подписи возможны без
новой authentication для каждого сообщения; тем более они не доказывают, что
пользователь видел каждое сообщение.

### 2.4 Physical presence не переносится между security domains по названию API

`Tag.aidl:422–467` устанавливает отдельное optional условие TUP. Присутствие
подтверждается сигналом, неподделываемым normal-world Android: например,
аппаратной кнопкой StrongBox либо эксклюзивно контролируемым TEE fingerprint.
Спецификация прямо говорит: presence в TEE не доказывает presence в StrongBox;
универсальный механизм переноса аналогичный HAT не определён. Password input
из normal world не является таким proof при compromised Android.

**Применимость:** `setUserPresenceRequired(true)` (API28) — заявка на конкретную
capability реализации, не обещание «любой PIN/любой телефон = hardware presence».
Нет основания автоматически включать этот tag как продуктовый default.

## 3. R05-009-B — подтверждённый текст, nonce и current authority проверяются отдельно

### 3.1 Protected Confirmation связывает именно подтверждённые bytes

Официальная русская инструкция [D3] и `IKeyMintOperation.finish:201–214`
согласованы: signing key с TRUSTED_CONFIRMATION_REQUIRED может подписать только
данные, согласованные с confirmation token; mismatch → NO_USER_CONFIRMATION.
Trusted UI подтверждает текст даже при compromised normal-world/kernel **в
названной модели доверия**. Компрометация самой TEE/KeyMint/Trusted UI выходит
за эту модель.

Разрешённые purposes/digest/padding также входят в key policy. Официальная
Protected Confirmation инструкция требует, в частности, для RSA signing key
отсутствия PURPOSE_ENCRYPT/PURPOSE_DECRYPT; одной отметки tag508 недостаточно.
Отсутствие проверки этих дополнительных constraints не следует маскировать
словом «hardware»: verifier caller должен задавать их явно [D3].

`onConfirmed(dataThatWasConfirmed)` возвращает **CBOR blob** с promptText и
extraData. Подписывается этот exact blob, а не реконструированная строка и не
произвольный hash другого payload. `extraData` включены в подпись, но **не
показаны пользователю**. Relying party должна проверить соответствие между
отображённым смыслом promptText и скрытым operation/request context.

KeyGenParameterSpec (:1442–1462) различает:

- confirmation: некоторый человек с physical possession одобрил сообщение;
- authentication: допустимый пользователь аутентифицировался;
- оба условия можно потребовать одновременно, если выбранная capability
  действительно поддерживает сочетание; `ConfirmationPrompt` сам не делает
  пользователя authenticated.

Подтверждение не создаёт конфиденциальности. Не помещать реальные ключи,
секреты, полный профиль либо чувствительный URL в Trusted UI/extraData/логи.

### 3.2 Ограниченный contract для будущего критического действия

Обозначения: `Ksig` — отдельный signing key; `B` — полученный CBOR confirmation
blob; `R` — exact request/purpose/target/lineage/intent; `n` — nonce отдельной
pending request; `E` — принятый набор roots/status/clock-policy.

В пределах выбранной модели sufficient acceptance **этого слоя**:

`KeyEvidence(Ksig, E, requiredEnforcement)`

`∧ VerifySignature(Ksig.public, exact B)`

`∧ DecodeBounded(B) = (promptText, extraData)`

`∧ DisplayedMeaning(promptText) = ApprovedAction(R)`

`∧ HiddenContext(extraData) binds R and n`

`∧ PendingUnused(n, intent) ∧ WithinChosenRequestLifetime`

`∧ CurrentAuthority(R) ∧ ActualOutcomeRecordedSeparately`.

Это logical obligations, не новый production format/собственная криптосхема.
Переиспользуются Android Protected Confirmation, стандартная подпись, CBOR
и существующая platform/recovery authority. Конкретная грамматика promptText,
локаль и интервал request validity должны быть заданы владельцем; их нельзя
выдумать и объявить уже реализованными.

Nonce для операции отличается от challenge, удостоверяющего enrollment ключа.
Требуется replay-регистр/consumption на уровне relying party; crypto operation
одноразовость handle не гарантирует однократную доставку бизнес-запроса.
При потерянном ответе отсутствие callback не превращает completed operation в
неисполненную: отдельно сохраняется outcome unknown и сверяется исполнение.

**Контрпример B1:** на экране «Добавить резервный узел», extraData кодирует
«Сбросить trust history». Подпись защищает оба поля, но пользователь подтвердил
только первый смысл. Подпись без semantic crosscheck не разрешает второе действие.

**Контрпример B2:** старый genuine signed B отправлен повторно после успешного
действия. Подпись и confirmation policy валидны, но nonce уже consumed;
повторное принятие запрещено contract. Аттестация signing key это не обнаружит.

**Контрпример B3:** человек прямо сейчас одобрил действие с устаревшим или уже
отозванным issuer/root. Fresh consent не делает issuer полномочным. Ответ о
TUF/recovery authority задаётся платформенным trust contract, не этим ключом.

### 3.3 Offline verification даёт as-of, не latest revocation

[D1] описывает Google status JSON по serial с REVOKED/SUSPENDED и
Cache-Control как указание частоты обновления. Здесь endpoint со статусами
**не запрашивался**, реальные цепочки не проверялись. Используются только
описание протокола и исходники candidate.

Если verifier имеет доверенный snapshot `E` на `t0`, при отключённой сети
неразличимы два мира: после t0 сертификат не отозван либо отозван удалённо,
но локальные bytes/clock одинаковы. Значит, `LatestNotRevoked(now)` не выводится
из offline signature verification. Может быть установлен лишь
`AcceptableAsOf(E, chosen policy)` с named root/status lineage, purpose и clock.
Наличие новой подписи operation не обновляет статус signing/attestation roots.

Не смешивать certificate-validity и request-freshness: [D1] отдельно допускает
expired factory attest chains при соблюдении своей policy, но требует validity
для RKP. Автоматически отвергать все expired или принимать все expired — не
точное воспроизведение Android-policy. Новая root family с февраля 2026 и
RKP требуют поддерживаемых anchors/algorithm/path handling; сертификаты из
страницы не использованы как enrollment input продукта.

## 4. R05-009-C — ready verifier есть, но generic Success не равен requested assurance

### 4.1 Exact candidate и исходники

**Android Key Attestation Verifier**, `android/keyattestation`, commit
**34652d3df68b8d6887a6670b00eb0856090dae6d**, публичный HEAD, зафиксирован
read-only `git ls-remote` и source archive; не claimed release/artifact.
Последний обнаруженный tag v0.4 указывает на другой commit 621149c4…;
`RELEASE.md` называет pre-v1/test release. Совпадение HEAD с v0.4 не заявляется.
LICENSE **Apache-2.0** прочитан целиком. AOSP source snippets — Apache-2.0
по headers; лицензия dependency closure отдельно не audited.

Фактический путь:

`Verifier.verify(chain, challengeChecker, …)` → `KeyAttestationCertPath` →
`CertPathValidator("KeyAttestation")` + `RevocationChecker(supplied serial set)` →
`KeyDescription.parseFrom` → optional challenge-check → constraints/app-id →
`VerificationResult.Success(publicKey, securityLevel, boot state, …)`.

### 4.2 Проверенные reuse-seams и ограничения

| Seam | Source-факт | Обязательная consumer policy |
|---|---|---|
| Challenge | `verify:183–187`, `internalVerify:310–315`: checker nullable, при null не исполняется | Для enrollment/request assurance явно required checker, expected nonce/context и anti-replay; `Success` без этого — narrower fact |
| Root/status/time | Constructor :151–156 принимает три разных providers; path-validator :251–254 берёт их | Inputs должны иметь собственные provenance/freshness/error states; `emptySet()` не обозначать «fresh status: none revoked» |
| Boot/enforcement | Defaults требуют generated hardware origin, consistent nonsoftware levels, non-null RootOfTrust; не требуют VERIFIED+deviceLocked | Если такой claim заявлен — explicit constraints на VERIFIED/locked и конкретные auth/confirmation tags |
| Confirmation flags | `Extension.kt:375–379` парсит noAuth/userAuth/timeout/TUP/confirmation; generic Success (:46–54) их не возвращает | Проверять в `additionalConstraints` через trusted KeyDescription; одного Success недостаточно |
| App identity | `expectedAttestationApplicationId=null` разрешён; даже STRICT с expectedId=null возвращает true | Явные expected package/signature/version/lineage inputs, не identity из того же недоверенного запроса |
| Chain extension | Path-validator :238–254 отвергает append после TARGET; :317–340 проверяет extension только на TARGET и не поддерживает ATTEST_KEY-chain exception | Не заявлять parser универсальным для всякой Android-цепочки; supported profile explicit; existing anti-extension guards **есть** |
| Status projection | `GoogleRevocationList.kt:82–88` фильтрует только REVOKED; SUSPENDED отбрасывается | При policy «оба nonnormal statuses deny/unknown» нужен non-lossy status adapter; runtime-impact/реальное наличие строки SUSPENDED не установлены |
| Resource limits | InputLimits package/signature caps есть; maxCborPreallocationSize — optimization, не max-input bound | Bounded certificate/count/CBOR/request ingress до provider/parser, без объявления полной RAM-bound библиотеки |

Критический opposing check: parser на leaf не считается самостоятельным bypass
из-за existing path-validator guards. Верификатор сначала проверяет цепочку и
запрещает appended leaf/extension; утверждение «в leaf парсят, значит уязвимость»
**не делается**. Ограничение ATTEST_KEY chains прямо отмечено TODO, а не
интерпретируется как доказанный security defect.

### 4.3 Диагностический CLI — контрпример переносимости claims

`VerifierCli.kt:61–69` конструирует verifier с
`revokedSerialsSource={emptySet()}` и вызывает `verify(certs)` **без challenge**.
Success печатает «Verification Successful!». Это корректный narrower результат
выбранного caller, не remote transaction/presence/status assurance.

**Контрпример C1:** цепочка криптографически валидна и удовлетворяет generic
constraints, но принадлежит старому challenge; в актуальном внешнем status её
сертификат помечен revoked. CLI его не узнал, поскольку caller не подал этот
input. Дополнительный предел — он не требовал confirmation flag/операционной
подписи. Никаких пользовательских цепочек/CLI запусков для вывода не потребовалось.

`getRevocationStatusFromWeb` делает URL I/O и не задаёт явные connect/read
timeouts в прочитанном function body (:48–68). Это candidate deployment-cost,
не доказательство бесконечного зависания (external provider defaults unknown).
В research-коде отсутствует whole-proof status age policy; она у relying party.

### 4.4 Цена готового переиспользования

Build manifest candidate: Kotlin/JVM2.2.0, **JDK21**, BouncyCastle1.78.1,
Guava33.5.0-jre, protobuf4.28.3, Gson2.11.0, CBOR0.9, coroutines1.10.2;
roots-generated source делается из repository roots.json при build. Никакие
зависимости не резолвились/не устанавливались и build не запускался.

Snowden Android toolchain17 — это **не drop-in Android-library recommendation**.
Официальный advice проверять на independent trusted verifier и source price
поддерживают server-side candidate. Такой сервер нельзя делать обязательным
bootstrap prerequisite автономного VPN; deployment/policy — отдельное решение.
Потери доступности/latency/размер/RAM/cost не измерены.

Собственные status/chain providers, constraints, separate PoP/confirmation
signature+CBOR checking и nonce state — consumer seam, не конкурирующий
ASN.1/crypto engine. LogHook способен получать полную input chain/key description;
production integration по умолчанию redacted/no sensitive raw logs. Не собирать
IMEI/serial/uniqueId «ради безопасности», если они не требуются scoped purpose.

## 5. Применение к нашей системе без изменения продукта

### 5.1 Симметричный checkpoint не становится attestable из добавления challenge

AOSP KeyGenParameterSpec :1598–1601 и API reference [D6]: AES/HMAC не имеют
public-key certificate; попытка использовать non-null attestationChallenge для
такого symmetric-key generation отвергается. Attestation отдельного EC/RSA
signing key удостоверяет **его**, не существующий checkpoint AES.

**Контрпример C2:** два устройства показывают genuine EC attestation и подпись
«AES hardware-backed». На одном checkpoint создан в hardware, на другом в
software, а доверенный EC ключ одинаково способен подписать приложенные bytes.
Без дополнительного trusted measurement/binding эти сведения о AES не следуют
из EC сертификата. Подпись приложения не превращает его утверждение в измерение
hardware enforcement другого ключа.

KeyInfo — useful **локальное наблюдение**, не remote attestation. `getSecurityLevel`
добавлен API31 [D5], несмотря на tutorial-фразу о target API29+; для min29 нужны
runtime API guard и более узкий `isInsideSecurityHardware` (API23) в API29–30,
без угадывания exact TEE-vs-StrongBox. TargetSDK36 не поднимает runtime API.
В inspected Snowden source KeyInfo не используется: фактический security level
этого AES ключа **UNKNOWN**, не установлен и не назван software-backed.

### 5.2 Не ломать unattended recovery ради proof пользователя

Предлагаемое раздельное назначение, **PROPOSED**:

- checkpoint AES остаётся storage-ключом current contract; read/load/normal
  automatic recovery не требуют нового физического prompt;
- отдельный signing key и явно требуемая capability применяется только к
  согласованным критическим manual actions (например, смена trust-enrollment),
  когда product owner считает дополнительную assurance необходимой;
- если capability отсутствует, возвращать supported/unsupported/unknown/error
  честно. Не подменять strong claim обычным диалогом/quiet fallback; вариант
  другой approved recovery policy остаётся у владельца, не универсальный запрет;
- presence/confirmation/attestation **не** даёт права снять protection, очистить
  floors, принять expired record, совершить rollback или доверять чужому issuer.

Android Protected Confirmation API28+ опциональна. Pinned AOSP
`ConfirmationPrompt.java:232–335` проверяет accessibility state; доступность
может исчезнуть между `isSupported` и `presentPrompt`, последний возвращает
error/throws. Не требовать пользователю отключить accessibility ради работы VPN;
support matrix и отдельный доступный recovery маршрут нуждаются в решении owner.
Гарантии разных маршрутов должны оставаться разными, без одинаковой надписи PASS.

Расположение independent verifier — не доказательство его независимости:
оператор, rooting authority, deployment, root/status delivery и failure domain
описываются отдельно. Источник current TUF authority остаётся предметом R05-010,
не определяется наличием «серверного сертификата» или fresh consent.

## 6. Оппонирующая самопроверка

1. Fresh unpredictable attestation challenge может установить narrow issuance
   relation; поэтому здесь не утверждается «attestation никогда ничего не
   говорит о времени». Отвергается только вывод о presence/current operation
   без отдельного evidence.
2. Authentication и confirmation могут сочетаться; запрещено не сочетание,
   а автоматическое выведение одного из другого.
3. Hardware-backed ключ снижает extractability, но компрометированный процесс
   может пользоваться разрешённым ключом. Неэкспортируемость не защищает
   plaintext/application state, не доказывает выполнение задачи.
4. Candidate имеет реальные origin/security/chain guards. Наличие optional
   checks — integration contract, не blanket-обвинение в небезопасной библиотеке.
5. SUSPENDED-filter finding условен: требуются выбранная policy и реальная status
   строка. Нет заявления об exploited bypass/размере вреда/затронутом продукте.
6. Current AES-key hardware level неизвестен. Отсутствие explicit StrongBox
   request не равно доказательству software storage; ОС может выбрать TEE.
7. Неэкспортируемость не доказывает universal невозможность credential migration:
   есть другие purpose-scoped enrollment/import mechanisms; они не разработаны
   здесь и не требуют менять существующий ключ автоматически.

## 7. Конечная будущая приёмка — все NOT_RUN

Ни fixtures, ни operation/token/certificate генерация в этом окне не выполнялись.

| Case | Критерий future review/execution |
|---|---|
| F01 | AES builder+attestation challenge: documented rejection; отдельная EC attestation не маркирует AES attestable |
| F02 | Genuine fresh attestation authRequired=false: key evidence success, user-presence claim absent |
| F03 | Нет/пустой/wrong/stale challenge в consumer: required-policy rejection/unknown; generic CLI narrower verdict |
| F04 | HAT wrong SID/type/operation challenge: per-use operation запрещена; HAT не читается из production устройства |
| F05 | Auth timeout допускает несколько операций: не маркировать каждую как новую authentication/consent |
| F06 | Trusted presence TEE vs StrongBox/normal-world PIN: поддержка и guarantees проверяются раздельно |
| F07 | Confirmation-required signing exact CBOR: mismatch bytes не получает подпись; callback без policy не принимается |
| F08 | promptText≠extraData action / неверная локаль: semantic rejection даже при valid signature |
| F09 | Duplicate/expired/out-of-context operation nonce: reject; lost response сохраняет unknown outcome |
| F10 | Root/status wrong source, stale/offline или SUSPENDED: named-policy result, не latest-good |
| F11 | Factory expiry vs RKP expiry/new root family: accurate platform profile, no universal accept/reject shortcut |
| F12 | Extended chain / ATTEST_KEY supported matrix: first-trusted-extension guards, no unreviewed workaround |
| F13 | minAPI29/30 vs API31+ KeyInfo; StrongBox unavailable: typed narrower capability, не compile-only success |
| F14 | Accessibility change/isSupported race/cancel: no false confirmation, без давления отключить a11y |
| F15 | Нет source drift, bounds на request/chain/CBOR; malformed input/IO timeout: bounded typed error |
| F16 | После manual consent issuer уже недопустим: current authority still rejects; consent не очищает floors |

До explicit native/crypto/device permission эти cases только программа,
а не acceptance выполненного продукта. Телефоны/OEM/latency/radio/RAM не измерены.

## 8. Предлагаемая адресная вставка (не интегрирована)

Для architecture/SAFETY-AND-TRUST рядом с custody/consent; Android-главе — ссылка:

> Android Key Attestation удостоверяет конкретный public key и scoped
> authorization metadata, а не свежее присутствие пользователя. Authentication
> per-operation, trusted physical presence и Protected Confirmation имеют
> разные enforcement и evidence. Protected Confirmation связывает подпись с
> показанным promptText и exact CBOR; hidden extraData не считаются увиденными
> пользователем. Operation nonce/current authority/status-as-of проверяются
> отдельно. Симметричный AES checkpoint не получает attestation от отдельного
> signing key; его фактический hardware level требует своего observation.
> Отсутствие per-use authentication у storage-ключа сохраняет unattended
> recovery и не заменяет ручное подтверждение отдельного критического действия.

Это предлагаемое уточнение, не принятое изменение общего контракта.

## 9. FUP-6-01 / R05-009-D — потери status projection и предел результата

Дополнение от **27 октября 2026**: прочитан complete scoped путь и
сохранено сравнение шести файлов с pinned archive — `fup-6-01/inputs.json`.
Состояние: **PARTIAL_AT_CUTOFF**; source-наблюдения D доступны для review,
законченный consumer-adapter contract не заявлен. A/B/C сохранены.

### 9.1 End-to-end source trace на том же pin

Candidate commit **34652d3df68b8d6887a6670b00eb0856090dae6d**.

1. `getGoogleRevocationStatusFromWeb` (`GoogleRevocationList.kt:35–38`)
   вызывает HTTP helper. Он требует HTTP200 (:60–64), затем передаёт stream
   в `parseAttestationStatus` (:65). Соединение отключается в finally (:66–68).
   Реальный HTTP-запрос к status endpoint этим исследованием не выполнялся.
2. `StatusEntry` содержит только `status: String` (:71), `StatusFile` — map
   serial→entry (:73). Опубликованная схема Google перечисляет REVOKED и
   SUSPENDED; candidate не использует enum/явную schema validation в этом пути.
3. `parseAttestationStatus:82–88` десериализует через Gson и возвращает только
   keys тех entries, у которых `status == "REVOKED"`. `SUSPENDED` и иные
   успешно прочитанные строковые значения отфильтровываются. `expires`, reason,
   comment и происхождение/возраст HTTP-ответа в возвращаемом set отсутствуют.
4. `Verifier.internalVerify:249–254` создаёт PKIXParameters, отдельно ставит
   date из instantSource и добавляет `RevocationChecker(revokedSerialsSource())`.
   Значение date не содержит время получения/возраст status snapshot.
5. Path-validator :126–145 вызывает basic checker и дополнительные checkers.
   `RevocationChecker.kt:42–54` сравнивает serial с supplied set; совпадение
   даёт `CertPathValidatorException` с reason REVOKED. Невхождение означает
   только отсутствие этого serial в **переданном** set.
6. `Verifier.kt:268–284` переводит path exception в PathValidationFailure;
   при прочих успешных проверках возвращает Success (:349–357). Success не
   содержит status source identity, received-at, schema-validity или as-of.

### 9.2 Матрица сохранения и потери различий

| Исходный случай | Поведение traced path | Что известно на выходе |
|---|---|---|
| REVOKED | Serial остаётся в set; при проверке соответствующего сертификата membership вызывает path failure | Deny по supplied revocation set; причина/возраст записи не перенесены |
| SUSPENDED | Строка распарсена, затем исключена filter | Для membership checker не отличается от отсутствующего serial; generic Success возможен при прочих выполненных constraints |
| Иное строковое значение status | Если Gson прочитал его как String, точное сравнение с REVOKED ложно | Не сохраняется как отдельное UnknownStatus; не объявляется автоматически валидным значением схемы Google |
| Serial отсутствует в успешно полученном snapshot | Нет membership | Лишь NotListed в данном snapshot. При правильных scope/completeness/freshness предпосылках это полезное отрицательное evidence; не globally-latest статус |
| HTTP не 200 / ошибка загрузки | Helper бросает исключение, не возвращает emptySet | Этот путь не превращает сбой в Success автоматически; caller должен корректно сохранить failure |
| Ошибка разбора, реально выброшенная парсером | До возврата set выполнение прекращается исключением | Не штатный emptySet fallback. Поведение всех malformed/null/schema случаев не проверено исполнением |
| Устаревший, но корректно десериализованный snapshot | Возвращается обычный Set<String>, без timestamp/age | От свежего snapshot с тем же set не отличается; certificate evaluation date не исправляет потерю давности |
| Caller сам передал emptySet, как VerifierCli:65 | Checker не находит ни один serial | Нельзя отличить «проверенный список пуст» от «статус не проверялся». CLI — подтверждённый narrower caller, не доказанная ошибка продукта |

**Оговорка о fail-closed:** IOException загрузки и ошибки Gson не превращаются
в VerificationResult.PathValidationFailure тем catch, который окружает
`certPathValidator.validate`: status source вызывается **раньше**, при создании
PKIXParameters. Внешний `verify` (:191–199) ловит CertificateException, а не
произвольную IOException/parser exception. Для штатной ошибки загрузки/разбора
полезный вывод — выполнение не возвращает Success; ошибка может выйти caller'у
исключением, а не одним из перечисленных VerificationResult. `verifyAsync`
также не добавляет универсальный status-error mapping в прочитанном пути.

Поэтому утверждение «ошибки загрузки библиотека теряет и считает good» **не
подтверждено**. Потеря возникла бы при дополнительном consumer fallback
`error → emptySet`; такого production consumer Snowden не обнаружено и здесь
не предполагается. Не все несоответствия JSON Schema обязаны бросать parse error:
строковый unknown status уже попадает в отдельную подтверждённую ветку filter.
Полный malformed-input/Gson audit остаётся не выполненным.

### 9.3 Сильнейший контрпример — без crypto/fixture execution

Пусть genuine chain удовлетворяет остальных accepted constraints, а supplied
успешно десериализованный status snapshot содержит SUSPENDED для проверяемого
serial и не содержит для него REVOKED. Candidate projection выбрасывает запись,
membership checker её не видит, остальные checks могут дать generic Success.

Если action policy требует отказа или unknown при SUSPENDED, generic chain
Success **недостаточен для полного разрешения операции**. Он не доказывает
ложность криптографической цепочки: недостаточен именно набор учтённых status
предпосылок. Это условный source/model контрпример, не живой сертификат,
измеренный bypass, затронутый Snowden runtime либо severity claim.

Точно так же две копии status snapshot с одинаковым set и разным допустимым
возрастом неразличимы для Set<String>-контракта. Корректная подпись новой
операции и проверка certificate validity не восстанавливают выброшенное age.

### 9.4 Consumer obligations: что нельзя потерять при будущей интеграции

Из полученных source-фактов следуют требования к review конкретного consumer,
**не реализация нового status-протокола**:

- сохранить отдельно status source failure, schema/unsupported status и
  successfully obtained snapshot; ошибка не превращается в empty list;
- до проекции в revocation set сохранить REVOKED/SUSPENDED/unknown и применить
  явно выбранную action policy; serial-not-listed не равен snapshot-unavailable;
- иметь provenance/scope snapshot и происхождение его временных данных;
  время локального чтения старой копии не обновляет as-of автоматически;
- поддержать выбранную freshness/clock policy и вернуть stale/unknown при
  недостаточных inputs; не подставлять неизвестную давность нулём;
- сохранить результат chain validation отдельно от status-policy disposition
  и permission операции; причины unknown/error должны оставаться доступными
  caller'у, без утечки содержимого сертификатов в пользовательские логи.

Законченная структура adapter/error model, обработка Cache-Control и full
Gson/schema semantics **не завершены** и не объявляются проверенными. Внешний момент публикации/самые свежие отзывы не
исследовались. Unattended recovery checkpoint не меняется; это обязательства
для нового scoped manual acceptance, а не приказ остановить существующий VPN.

### 9.5 Остаток и доказательство

- Полный consumer mapping ошибок/неизвестных схем/давности — незавершённый gate.
- Native/crypto/JSON fixtures не создавались; все старые F01–F16 — NOT_RUN.
- Любая будущая проверка SUSPENDED/revoked/malformed/stale должна использовать
  разрешённый искусственный стенд, не реальные пользовательские сертификаты.
- Шесть source files совпали с сохранённым exact-pin archive до срока;
  remote refresh/whole-library audit не выполнялись. SHA-256 — identity source,
  не доказательство правильности криптографического результата.
- Это исторический остаток 27 октября; его ограниченное продолжение
  от 28 октября приведено в §10.

<a id="10-новая-задача-28-октября--завершение-source-model-остатка-fup-6-01"></a>
## 10. Уточнение source-model контракта FUP-6-01 от 28 октября

Дополнение от **28 октября 2026** продолжает оставшийся вопрос.
Исторический срез 27 октября, §9 и его **PARTIAL_AT_CUTOFF** сохраняются
отдельно; позднее дополнение не завершает их задним числом.

Научный результат этого дополнения: **R05-009-D, уточнённая область
READY_FOR_REVIEW** — exact parser/source matrix + законченный ограниченный
consumer contract в форме логических требований. Реализация, runtime,
полнота всех malformed-input случаев и криптографическая приёмка — NOT_RUN.

### 10.1 Новый проверенный вход: Gson2.11.0 на exact release pin

Verifier manifest содержит `com.google.code.gson:gson:2.11.0`. В этой партии
прочитаны нужные исходники **google/gson**, tag `gson-parent-2.11.0`:
peeled commit **828a97be0f8d58108b140b77df8dc76b657f4a87**, annotated tag object
**3b7c2ad8ce8dd471449ccbaee6d22f64c9816a38**. Код не исполнялся, class-файлы
не исследовались; relation release→source по опубликованному tag не является
независимой аттестацией Maven artifact. LICENSE Apache-2.0 прочитан.

Точный путь: `GoogleRevocationList.parseAttestationStatus(InputStream)` →
`new Gson().fromJson(Reader, StatusFile.class)` → reflection adapters для DTO
и map → `filterValues(status == "REVOKED")` → supplied Set<String>.

Источник G1–G6 в SOURCES §8; ниже source/model при обычном field-reflection
DTO без custom adapter, @JvmRecord, shrinker transformations или ограничений
рефлексии. Если runtime запрещает allocation/field access, результатом может
быть ошибка; source-presence не обещает удачного разбора на всех JVM/Android.

| Случай | Exact source consequence | Ограничение вывода |
|---|---|---|
| JSON `status: null` либо отсутствующее поле status в непустом объекте entry | STRING adapter допускает null; field-reflection не выполняет Kotlin primary constructor validation и не проверяет required JSON fields. При успешном DTO allocation reference может остаться null; точное сравнение с REVOKED не создаёт schema rejection | Kotlin `String` declaration не JSON schema. Наличие generated artifact/reflect-access/runtime checks не attested |
| JSON `status` — boolean или число | TypeAdapters.STRING:458–472 превращает boolean в текст; JsonReader.nextString:890–921 допускает numeric tokens. Полученные "true"/числовые строки не равны REVOKED и исключаются | Это не значение enum опубликованной схемы; strict syntax mode сам не устраняет coercion этого adapter |
| JSON `status` — object/array | STRING adapter вызывает nextString, для такой формы возникает unexpected-token failure; Gson оборачивает IllegalStateException/IO в JsonSyntaxException | Ошибка не эквивалент emptySet; конкретные все вложенные формы не исполнялись |
| Root пустой/JSON null | Gson.fromJson возвращает null; candidate затем обращается к `.entries`, не оформляет schema-validated empty list | Возможен exception до set. Нельзя объявить успешную пустоту; точный compiled exception type не установлен |
| `entries` отсутствует/null | Нет обязательной field validation; Map adapter принимает null; обращение candidate к map не создаёт корректный snapshot | При обычном reflected DTO ожидается failure, не достоверное "нет отзывов". Не обещается один конкретный exception class |
| Entry для serial равен null | Map adapter может сохранить null; candidate обращается к status entry | Отдельный malformed-state/failure, не подтверждённый NotListed |
| `entries` задан массивом пар | MapTypeAdapterFactory:182–194 умеет читать array-of-pairs, хотя опубликованная схема требует object | Успешная десериализация не schema conformance; explicit type guard обязателен |
| Повторённый serial в map | MapTypeAdapterFactory:188–203 проверяет replaced-value!=null и отвергает обычный duplicate с предыдущим non-null значением | Это существующий guard: нельзя писать «Gson всегда last-wins для map». Если previous=null, этот check отдельно duplicate не фиксирует |
| Повторено DTO поле status или entries | Reflective adapter:509–517 читает каждое известное имя и снова пишет поле (:265–278); отдельного duplicate-field guard здесь нет | Последующее значение способно заменить предыдущее, например REVOKED на SUSPENDED. При допущенных ordinary reflection условиях; runtime case NOT_RUN |
| Unknown extra field либо optional reason/expires/comment | Reflective adapter:513–514 skipValue для неописанных DTO fields | Известные optional metadata и запрещённые schema additionalProperties не различаются; их отсутствие/тип после DTO projection уже нельзя восстановить |
| JSON-синтаксис вне strict RFC8259 | new Gson() не задаёт explicit Strictness; fromJson меняет LEGACY_STRICT на LENIENT для compatibility | Strictness.STRICT — доступный reuse, но не full schema/type/duplicate/enum validation |
| Несколько top-level values после успешно разобранного non-null root | Reader overload invokes assertFullConsumption; при остатке бросается exception | Existing full-consumption guard есть. Нельзя переносить отсутствие такого guard из JsonReader overload на actual Reader caller |

**Почему Kotlin non-null не sufficient:** ReflectiveTypeAdapterFactory
обычным JVM-классам назначает FieldReflectionAdapter; ConstructorConstructor
ищет no-args constructor и при его отсутствии может использовать Unsafe
(DEFAULT_USE_JDK_UNSAFE=true). Primary Kotlin constructor с null-check не
является источником required-field validation этого пути. Подтверждается
только traced source rule при успешном reflection construction, не JCA/JVM
runtime behaviour каждого устройства. Даже запрет Unsafe не делает malformed
snapshot valid: если создание невозможно — ошибка должна остаться ошибкой.

**Уточнение относительно §9:** «ошибку загрузки не превращают в good» остаётся
верно для выброшенной ошибки; но не каждое schema violation является parse
exception. Поэтому comment fetcher «fail-closed if cannot parse» нельзя
расширять до «schema-invalid statuses всегда запрещены». Это разные predicates.

### 10.2 Невосстановимая потеря информации — ограниченный логический вывод

Определим опубликованный status document `D` и candidate projection:

`P(D) = { serial | D.entries[serial].status == REVOKED }`.

Пусть D0 — valid document, где serial s отсутствует; D1 — valid document,
где s помечен SUSPENDED и других отличий в REVOKED set нет. Тогда:

`P(D0) = P(D1)`.

Policy, различающая `NotListed(s)` и `Suspended(s)`, даёт разные dispositions.
Значит, не существует функции только от `P(D)`, которая восстановит обе
правильные dispositions для всех таких D. Это **точное отрицательное
утверждение о lossiness**, а не утверждение, что подпись chain недействительна
либо весь verifier небезопасен. Нужен исходный status/document evidence до P.

Аналогично одинаковый set может быть получен из допустимо свежего и старого
snapshot. Если age/provenance не вход функции, freshness policy по одному set
не вычисляется. Error не обязан теряться: source может throw. Потеря возникает
при consumer-преобразовании `(failed acquisition) → emptySet`, которое нельзя
принимать как корректный status contract.

### 10.3 Разделить пять результатов, а не добавить ещё один boolean

Предлагаемый **логический** контракт для одного bounded verification request,
не production wire format и не новый механизм подписи:

1. **Acquisition**: успешно полученное/повторно валидированное representation
   либо `Unavailable(stage/reason)`, `Canceled`, `ResourceLimit`, `InvalidDocument`.
   Stage: connect/read/HTTP/decode/schema/persist. Классификация сохраняет
   фактическую границу ошибки; если exception wrapper не позволяет разделить
   read IO и syntax, вернуть честный `ReadOrDecodeFailure`, не придуманный диагноз.
2. **SnapshotEvidence**: source identity, выбранные scope/profile, точные
   representation identity/validator, способ доставки/revalidation и time evidence.
   Body hash идентифицирует bytes, но не доверяет источнику и не доказывает полноту.
3. **StatusOf(subject)**: REVOKED / SUSPENDED / UnknownValue(raw identifier,
   safely bounded) / NotListed / SnapshotUnavailable. `NotListed` разрешается
   только в validated snapshot нужного scope; не при отсутствии snapshot.
4. **FreshnessDisposition**: AcceptableAsOf(policy/time inputs) / Stale /
   UnknownAge / RevalidationRequired. Возраст информации не обнуляется при
   чтении из диска. Отдельно сохранить acquisition failure даже если policy
   разрешает пользоваться прежним, ещё приемлемым snapshot.
5. **Decision**: ChainResult и StatusPolicyResult отдельно; итог разрешения
   операции зависит также от key/purpose/intent/current authority/presence.
   `NotListedAsOf` не называется globally latest NotRevoked и не означает
   автоматически OperationPermit.

Требования, а не свободная новая модель полномочий:

- JSON syntax и schema проверяются **до** DTO lossiness: object root/object
  entries, unique member names, bounded UTF-8 document, required String
  status exact enum, canonical serial representation, schema-defined optional
  fields. Unknown status/schema version → typed unsupported/unknown/invalid
  according to named policy, а не default normal. Строгий JSON parser готовый;
  собственный ASN.1/cipher/verifier не предлагается.
- Canonical serial matching должно соответствовать опубликованным lowercase
  hex keys и checker serial formatting. Не нормализовать неоднозначные duplicate
  aliases скрыто и не создавать synthetic serial identity.
- REVOKED и SUSPENDED сохраняются раздельно. Policy может deny оба с разными
  reasons либо считать SUSPENDED неразрешимым; **SUSPENDED не переименовывается
  в REVOKED** ради прохождения Set<String> adapter без разъяснения.
- Для permit требуются принимаемые known statuses всех subjects, входящих в
  обещание. Unknown/excluded/malformed не превращаются в NotListed. Partial
  чтение списка не certificate-good evidence; нельзя возвращать partial set
  как полный snapshot. Ранее отдельно accepted deny не исчезает от source error;
  правила его supersession определяются той же source/action policy, не вечным
  самодельным union всех исторических SUSPENDED.
- Trust anchor исключается из `CertPath.certificates`; checker применяется к
  certificates этого пути. Fresh root acceptance/retirement — отдельный gate
  trustAnchorsSource. Нельзя расширить факт membership проверки path на полную
  проверку latest status самого anchor по названию PKIXRevocationChecker.
- Failure handling не прячет cancel/timeout/crash под успех. Для async caller
  exception/future failure/cancellation — отдельный исход; они не одновременно
  typed VerificationResult.Success. Остальные unexpected provider errors
  сохраняются opaque failure с redacted reason, а не маскируются normal status.

Эти predicates — минимальные **семантические обязательства выбранного claim**,
не минимальное число production полей. Конкретный интерфейс может пользоваться
существующим object/context, если evidence не теряется.

### 10.4 Давность: HTTP freshness, observation time и время статуса — разные оси

Новые primary references: **RFC9111 HTTP Caching** (июнь 2022),
**RFC8259 JSON** (декабрь 2017); точные секции/чтение SOURCES §8.
Официальная D1 говорит применять HTTP Cache-Control к обновлению status list;
она не задаёт instant global revocation или signed per-record status time.

1. **HTTP200** может прийти через cache. Получение байтов сейчас означает
   local observation сейчас, не создание snapshot сейчас.
2. Для cache-reference RFC9111 §4.2.3 определяет age, включая время по пути:

   `apparent_age = max(0, response_time − Date)`;

   `corrected_age_value = Age + (response_time − request_time)`;

   `corrected_initial_age = max(apparent_age, corrected_age_value)`;

   `current_age = corrected_initial_age + (now − response_time)`.

   Это RFC cache estimate при указанной clock/header модели, **не наша
   криптографическая upper-bound гарантия**. RFC при отсутствии Age использует 0
   в вычислении; это не разрешение подставлять 0 для неизвестной as-of policy
   или забыть остальные terms/clock trust.
3. `max-age`/HTTP Expires задают freshness lifetime; срок уже расходуется в
   transit/cache. «Дать max-age заново от момента чтения» ошибочно. Unqualified
   `no-cache` требует подходящей validation; qualified form имеет отдельную
   header-field семантику (§5.2.2.4). `must-revalidate` не допускает обычного stale
   reuse после expiration. Здесь нет утверждения, что Google именно выставил
   такие directives: реальные response headers **не запрашивались**.
4. Свежая корректная **304 Not Modified** может обновить metadata уже
   сохранённого matching representation (§4.3.4), не обязана менять body bytes.
   Поэтому утверждение «возраст можно обновить только полным body download»
   тоже неверно. Revalidation связывается с нужным URL/scope/validator, не любым
  304 рядом со старым файлом. Original retrieval и latest successful validation
   остаются разными фактами; failed attempt не обновляет ни один из них.
5. JSON entry `expires` в D1 означает дату expiration **сертификата**, useful
   для очистки списка. Это не nextUpdate/TTL status snapshot и не основание
   принимать весь список до этой даты. Если source не публикует authenticated
   status-as-of, не выдумывать `thisUpdate` из этого поля.
6. Acceptable HTTP freshness означает acceptable response reuse **по выбранной
   policy**, не знание о всех непубликованных компрометациях. При неизвестных
   source publication guarantees даже свежий response не даёт true global status.

Минимальный time model для consumer:

- отдельно записывать request start, received-at, original retrieval / successful
  revalidation и clock domain/continuity;
- сохранять необходимые header/cache evidence, observation-vs-status assurance,
  scope и policy revision; не считать wall clock из приложения доверенным
  независимым time input по одному названию поля;
- refresh/revalidation deadline derive из известного HTTP evidence и explicit
  action max-age policy; при insufficient trust/clock → UnknownAge или bounded
  permitted knowledge, не unlimited normal status;
- disk restore/reboot без accepted clock continuity не обновляет давность;
  failed refresh не reset snapshot-age. Допустимое использование старого
  snapshot решает explicit policy с retain error, а не generic catch fallback.

Формально для negative permission требуется принятый producer bound C(t) на
давность знания и policy budget Δ: `C(t) ≤ Δ`. Если такого bound/clock/source
assumption нет, predicate неизвестен. Числа Δ/ClockDrift здесь не назначены;
RFC current_age не объявляется C(t) без дополнительной обоснованной модели.

### 10.5 Boundary cases: ручные контрпримеры и finite future review

| ID | Ручной scenario / ожидаемый consumer contract | Execution |
|---|---|---|
| D01 | Пустой valid entries object vs failed read→emptySet: первое может стать NotListedAsOf, второе Unavailable; не смешивать | NOT_RUN |
| D02 | SUSPENDED vs absent serial: разная status disposition, одинаковая candidate P(D); preserve map до projection | NOT_RUN |
| D03 | Missing/null/boolean/numeric status: schema invalid/unsupported, не «нет отзыва»; Kotlin non-null не достаточен | NOT_RUN |
| D04 | Repeated status REVOKED→SUSPENDED или повтор entries: duplicate-object fields rejected consumer; existing Gson map guard не all-layer proof | NOT_RUN |
| D05 | Entries array-of-pairs или unknown extra schema field: strict schema boundary до accepted snapshot | NOT_RUN |
| D06 | HTTP/IO/parser exception/cancel: typed acquisition failure, не reuse-as-fresh; opaque wrapped IO/decode разрешён как honest unknown | NOT_RUN |
| D07 | Корректно принятый cache response Age>0: max-age не начинается заново; actual headers/source trust должны быть inputs | NOT_RUN |
| D08 | Matching successful304 vs unrelated304 vs failed refresh: только первый update cache metadata по scope/validators | NOT_RUN |
| D09 | Serial expires далеко в будущем, snapshot устарел: certificate expiry не status TTL | NOT_RUN |
| D10 | Disk/reboot/rollback часов: отсутствие accepted continuity→UnknownAge; restore не новая observation/validation | NOT_RUN |
| D11 | Chain Success + stale/unknown status: crypto result сохранён, новой status-dependent операции permit нет | NOT_RUN |
| D12 | Valid path but root retired by root-source policy: trust-anchor gate отдельно, не implicit «checker проверил все корни» | NOT_RUN |

Готово на уровне **source/spec/model contract**, а не реализации/выполнения D01–D12.
Не создавались fixtures, JSON parser execution, certificates, HAT, device/network
probes. Исторический PARTIAL_AT_CUTOFF от 27 октября сохраняется отдельно
от результата дополнения 28 октября.

### 10.6 Итог и остаток

Закрыт прежний содержательный gate: что теряет конкретный parser/projection,
как error проходит caller boundary и какие error/provenance/as-of сведения
consumer обязан сохранять. Потери SUSPENDED/unknown/freshness **подтверждены
source**, штатные HTTP error guards признаны существующими, blanket bypass
не заявлен. Ошибочная аксиома «nullable/schema-invalid данные всегда приводят
к parse exception» не используется.

Остаток — независимое принятие руководителем scoped R05-009-D; выбор конкретной
action policy/supported platform и отдельное разрешение на implementation/
runtime проверки. Full parser/provider resource bound, real HTTP headers/latest
status, release-artifact attestation, текущая совместимость на устройствах и
измерение стоимости **не установлены**; отсутствие данных не заполнено нулём.


