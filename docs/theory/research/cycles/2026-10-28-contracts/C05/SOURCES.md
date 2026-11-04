# R28-C05 — SOURCES / provenance / read depth

Дата: 28 октября 2026.
Корень проекта `D:/SNOWDEN-SYSTEM-V3`. Captures только
`D:/SNOWDEN-SYSTEM-V3/.artifacts/research/r28-contracts/C05/`.
SHA256 здесь удостоверяет идентичность прочитанного source, не правильность
криптографии/исполнения. Raw source fetch не обращается к status/device/VPS endpoint.

## 1. Baseline / управление областью

| ID | Прочитанный input | Read depth / значение |
|---|---|---|
| B0 | C05/TASK и COMMON нового цикла | FULL_TEXT_READ при начале нового назначения; дальнейшие выводы в рамках четырёх stages. Старые окна не возобновлялись |
| B1 | `docs/theory/research/05-TRUST-SAFETY/REPORT-009.md` | SECTIONS_READ: §10.1–10.4, scoped verdict/остаток; §9 только граница предыдущего partial. Не принят автоматически |
| B2 | `docs/theory/research/05-TRUST-SAFETY/REPORT-010.md` | SECTIONS_READ: current retention trace, scope/reset ordering, §7/F/H/G и contrasting outcomes |
| B3 | `docs/theory/architecture/TRUST-CONTINUITY-AND-AGREEMENT.md` | SECTIONS_READ: §§1–3 (history/coherent cut/root ordering/overlap). TAP/hash-agreement за пределами этого пакета |
| B4 | `docs/theory/architecture/RELEASE-EVIDENCE-BOUNDARIES.md` | SECTIONS_READ: время status evidence, historical CRL coverage, source/native boundary; не whole release audit |
| B5 | Z06/Z07 TASK | SECTIONS_READ программы/strongest question; HANDOFF на09:10:09Z отсутствуют, не SOURCE_NOT_AVAILABLE для наших upstream inputs |

Baseline/current copies и hashes: `local-preimage.json` (09:01:57–58Z),
`local-supplement.json` (09:18Z). Paths в этих manifests относятся к project root.
TASK/COMMON/shared chapters/old reports/продукт не редактировались.

## 2. Primary documents

| ID | Источник / version | Прочитанная область и граница |
|---|---|---|
| P1 | [Android key-attestation guide](https://developer.android.com/privacy-and-security/security-key-attestation), retrieved28.10 | SECTIONS_READ: verification roles, status schema/serial/enum/expiry cleanup, root/path distinction. Дата страницы не runtime evidence. Actual status URL НЕ запрашивался; capture `public/google-attestation-doc.html` |
| P2 | [TUF spec1.0.37](https://theupdateframework.github.io/specification/v1.0.37/), version header Last modified2026-10-08 | SECTIONS_READ: §§5.1–5.5, особенно5.3.7–11/5.4 ordering/expiry/discard. Не весь conformance audit; capture `public/tuf-spec-1.0.37.html` |
| P3 | [RFC9111](https://www.rfc-editor.org/rfc/rfc9111.html), June2022 | SECTIONS_READ: §4.2.3 age formula и §4.3.4 representation selection/304; raw capture `public/rfc9111.txt`. Не origin status publication/time guarantee |
| P4 | [JavaSE21 CertPathValidator](https://docs.oracle.com/en/java/javase/21/docs/api/java.base/java/security/cert/CertPathValidator.html#getInstance(java.lang.String)) | SECTIONS_READ: provider architecture, getInstance(String), getInstance(String,Provider), getProvider. Algorithm-only выбирает первый подходящий provider; actual registered providers не читались |
| P5 | [Android KeyPermanentlyInvalidatedException](https://developer.android.com/reference/android/security/keystore/KeyPermanentlyInvalidatedException), API23 | SECTIONS_READ: class definition/invalidation semantics, не runtime/device proof |
| P6 | [Android BiometricPrompt.CryptoObject](https://developer.android.com/reference/android/hardware/biometrics/BiometricPrompt.CryptoObject), API28 | SECTIONS_READ: class summary, auth-per-use против time-based keys, связанные API. Не совместимость конкретного authenticator |
| P7 | [Windows NCryptSignHash](https://learn.microsoft.com/en-us/windows/win32/api/ncrypt/nf-ncrypt-ncryptsignhash) | SECTIONS_READ: key handle, silent flag failure, return value и platform requirements; actual KSP/key не опрашивались |
| P8 | [Windows CryptProtectData](https://learn.microsoft.com/en-us/windows/win32/api/dpapi/nf-dpapi-cryptprotectdata) | SECTIONS_READ: user/computer context и exceptions, UI_FORBIDDEN, NULL prompt, success/failure. Не hardware-custody certification |

P1/P2/P3 raw captures получены с09:04:10Z/08:59:08Z/09:04:10Z batches;
P4–P8 прочитаны через primary web tool (09:01–09:14Z), URL/sections выше.
Точный byte hash динамических P4–P8 страниц не заявлен. Для офлайн-проверки
они не подменяются скопированными пересказами. Не целые API/reference сайты.

## 3. Google verifier exact source

Pin **`34652d3df68b8d6887a6670b00eb0856090dae6d`**.
[Pinned tree](https://github.com/android/keyattestation/tree/34652d3df68b8d6887a6670b00eb0856090dae6d).
Captured source commit, не published binary/release identity.

| ID | File / выбранные строки | Read depth |
|---|---|---|
| G1 | `src/main/kotlin/GoogleRevocationList.kt`:31–87 | SECTIONS_READ всей рабочей логики fetch/parse/DTO/projection; exact raw повторно получен08:57:20–23Z |
| G2 | `src/main/kotlin/Verifier.kt`:45–67,149–284,285–359 | SECTIONS_READ result/constructor/verify/async/internalVerify и gates до Success; exact raw получен09:01:36–42Z, archive hash совпал |
| G3 | `provider/RevocationChecker.kt`:class/init/check | SECTIONS_READ всей логики checker; fresh raw получен09:04Z |
| G4 | `provider/KeyAttestationCertPath.kt`:34–90 | SECTIONS_READ constructor/getCertificates/serialNumbers из имеющегося pinned archive; остальная provisioning parser logic не audited |
| G5 | `provider/KeyAttestationCertPathValidator.kt`:54–155 | SECTIONS_READ parameter/date/provider/checker loop; BasicChecker/factory/RKP validator целиком не reviewed |
| G6 | `provider/KeyAttestationProvider.kt`:28–60 | SECTIONS_READ registration/service resolution; fresh raw получен09:04Z |
| G7 | `build.gradle.kts`:repositories/dependencies/java toolchain | SECTIONS_READ: Gson2.11.0, Kotlin2.2.0, Java21; fresh capture09:01Z. Test tasks не запускались |
| G8 | LICENSE at pin | SECTIONS_READ license identity и redistribution conditions; fresh capture08:59Z, Apache-2.0 |

Fresh raw URLs имеют основу
`https://raw.githubusercontent.com/android/keyattestation/34652d3df68b8d6887a6670b00eb0856090dae6d/`.
G4/G5 archive:
`.artifacts/research/theory-05/2026-10-27-keystore-attestation-res6/verifier-source/`.
G2 свежий hash совпал с этим архивом; архив не объявляется independently verified
published binary. G4/G5 дополнительно получены по exact raw pin09:22:40–44Z;
их hashes совпали с archive (`source-fetch-07.json`).

## 4. Gson2.11.0 exact source

Tag `gson-parent-2.11.0` → tag object
`3b7c2ad8ce8dd471449ccbaee6d22f64c9816a38` → commit
**`828a97be0f8d58108b140b77df8dc76b657f4a87`**.
Tag/ref и target metadata повторно получены09:13:52–09:14:23Z:
`public/gson-tag.json`, `public/gson-tag-target.json`.
[Pinned tree](https://github.com/google/gson/tree/828a97be0f8d58108b140b77df8dc76b657f4a87).

| ID | File under `gson/src/main/java/com/google/gson/` | SECTIONS_READ |
|---|---|---|
| J1 | `internal/bind/ReflectiveTypeAdapterFactory.java` | Normal/record selection;244–279 field assignment;498–564 read/accumulator/DTO duplicates |
| J2 | `internal/bind/MapTypeAdapterFactory.java` | 172–209 map/array-of-pairs/null/replaced-value duplicate guard |
| J3 | `internal/bind/TypeAdapters.java` | 458–478 STRING coercion/null handling |
| J4 | `stream/JsonReader.java` | 890–925 nextString numeric/string tokens и exception branch |
| J5 | `internal/ConstructorConstructor.java` | 105–176 construction selection;412–445 Unsafe path/failure. Actual allocation не исполнялось |
| J6 | `Gson.java` | Defaults155–161; Reader overload1259–1265;1345–1381 strictness/catches/EOF;1466–1475 full-consumption guard |
| J7 | `GsonBuilder.java` | 552–568 setStrictness, available since2.11.0; SECTIONS_READ по fresh pin09:22Z, hash совпал с archive; не целый builder audit |
| J8 | LICENSE | License identity/redistribution, Apache-2.0; exact fresh capture08:59Z |

J1–J6 fresh captures получены08:57–09:13Z. Для J1,J3–J6 свежие hashes совпали
с адресно читанным архивом; J2 read напрямую, `archive-identity.json` фиксирует
проведённые сравнения (не blanket проверку всего archive).
Raw URL base:
`https://raw.githubusercontent.com/google/gson/828a97be0f8d58108b140b77df8dc76b657f4a87/`.
SDK compilation/JAR/reflection/R8/desugaring и schema fixtures — NOT_RUN.

## 5. TUF exact source и current consumer

SDK tag **v2.4.2** → commit
**`f5edbde31e5507f46db2069402dc38903fe6d9d4`**; fresh tag metadata09:13Z
`public/tuf-tag.json`. Apache-2.0 LICENSE повторно получен08:59Z.
[Pinned tree](https://github.com/theupdateframework/go-tuf/tree/f5edbde31e5507f46db2069402dc38903fe6d9d4).

| ID | Source | Read depth / для какого вывода |
|---|---|---|
| T1 | `metadata/trustedmetadata/trustedmetadata.go`:43–56,61–95,103–173,178–261 | SECTIONS_READ fixed RefTime, root checks/workflow guard, assignment timestamp до final expiry; snapshot post-assignment expiry/version checks; raw capture08:57Z, дополнительное чтение09:23Z |
| T2 | `metadata/updater/updater.go`:loadTimestamp311–353,loadRoot487–526 | SECTIONS_READ cache load/remote error/step persistence/root chain; current artifact hash совпал с прежним pinned source |
| T3 | `metadata/metadata.go`:VerifyDelegate248–425, IsExpired | SECTIONS_READ role keys/quorum и выбор конкретных подписей, distinct public-material contribution, expiry equality. Dual-signed model требует только этой логики, не генерации подписей |
| T4 | `go.mod` | SECTIONS_READ module/go1.25.0/dependencies; no fetch/install/build |

Raw URL base:
`https://raw.githubusercontent.com/theupdateframework/go-tuf/f5edbde31e5507f46db2069402dc38903fe6d9d4/`.
Назначение/persistence pointer не названо общим receipt. Полная безопасность SDK
или соответствие всем версиям spec не заявлены.

Current project files прочитаны статически и скопированы с hashes:

| File | SECTIONS_READ / bound claim |
|---|---|
| `windows/go.mod` | go1.26, declared go-tuf/v2 v2.4.2; replace/toolchain поиск. Declared source не installed/runtime module proof |
| `windows/backend/delivery/client.go` | packageFetcher, PrepareArtifacts и finalPolicy; refresh → retention → error; success proof creation. No network fetcher |
| `windows/backend/delivery/retained_inputs.go` | Вся рабочая логика matchesMetadata/boundMetadata/retainedRoot; ambiguity guard, every intermediate root и material-set reset predicate |
| `windows/backend/delivery/retained_state.go` | Вся рабочая логика next/root/reset/cache и completeRetainedState; progress отдельно от artifact proof |
| `windows/backend/delivery/retained_policy.go` | Timestamp/snapshot continuity floors, не expiry receipt |
| `windows/backend/delivery/policy.go` | keyIdentity/roleKeys/rootPolicy; guardMetadataShape/metadataReferences. Accepted signature не выводится из shape guard |
| `windows/backend/delivery/model.go` | State.Validate/Digest, Prepared.Check, Progress getters; sections122–160,375–420 |
| `windows/backend/metadata/delivery.go` | CheckDeliveryBinding/Proof, exact source/next source; не защита от attacker, переписывающего оба файла |
| `windows/update_delivery.go` | prepareVerifiedUpdate и повторный withVerifiedUpdateModeLocked до apply callback; error branch и purpose-specific gates |
| `windows/delivery_state.go` | retainDeliveryProgress до transaction invocation; guard/CAS/vault-owner seam. Actual Commit и все filetxn internals не исполнялись/не audited |
| `windows/backend/secretvault/crypto_windows.go` | DPAPI wrappers/source flags, не real vault/key/certificate |

Математические histories/quorum и countertrace описаны только текстом в REPORT.
Они не являются измеренными attacker traces, созданными fixtures или доказанным
успешным изменением состояния устройства.

## 6. Captures, неудачи и точность заявлений

- `source-fetch-01.json`…`source-fetch-07.json`: exact URLs, source acquisition
  begin/end UTC, success/failure и SHA256. Последний raw fetch09:22:44Z;
  дополнительное чтение уже полученного spec/SDK09:23:16–17Z, до cutoff.
- `archive-identity.json`: восемь адресных hash comparisons, все same=true;
  ещё три совпадения в `source-fetch-07.json`; не проверка всего репозитория/всех зависимостей.
- `local-preimage.json`, `local-supplement.json`: исходные source/doc inputs,
  копии в `local/`. `preservation-review.json` от09:25Z: все15 current inputs
  и их copies совпали с зафиксированными hashes.
- `doc-review.json` от09:30Z: все8 локальных links существуют, все26 recorded
  source hashes совпали; `public-index.json` перечисляет raw captures с hashes.
  Это статическая сверка собственных документов/source, не product tests.
- `paired-availability.json`: одно наблюдение отсутствия Z06/Z07 HANDOFF.
- TUF1.0.37 через web tool сначала **FETCH_FAILED** (Internal Error,08:58Z),
  затем bounded native public fetch **succeeded**08:59:08–13Z; использована
  полученная версия с проверенным header. Одинаковый failing запрос не повторялся.
- Несколько навигационных запросов по предполагаемым именам вернули no file
  (`delivery_progress.go`, `state.go/types.go`, старый SOURCES-001); реальные
  пути найдены адресным rg. Эти имена не стали фиктивными source evidence.
- Большие combined outputs несколько раз обрезались. Критические code scopes
  дочитаны адресно; не объявляется FULL_TEXT_READ всего файла по наличию capture.
- Никаких endpoint/status/SDK/native/crypto/test/build/admin/live/production
  результатов. Все future acceptance V1–V8 — **NOT_RUN**.
