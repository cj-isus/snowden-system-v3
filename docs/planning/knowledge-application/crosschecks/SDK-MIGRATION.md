# Общий trust SDK: минимальный состав и безопасная миграция

Предлагаемая граница SDK относится к source-срезу 28 октября,
не к полному inventory текущих реализаций. Порядок извлечения и подключения —
в [плане ПК](../PC-PLAN.md) и [плане Android](../ANDROID-PLAN.md).

## 1. Рекомендуемая граница

**Выделять существующий verifier по функциям, а не переносить Windows packages
целиком.** Минимальный общий слой — bounded codecs подписанных документов,
проверка ролей/истории/эпох/pin-policy, подготовка допустимого TUF progress и
application candidate на одном source cut. Storage, ключи устройства, часы ОС,
установка, исполнение, VPN lifecycle и наблюдения остаются платформам.

Это **frozen interface proposal v1 для согласования**, не опубликованный или
реализованный SDK. API surface и инварианты ниже фиксируют минимальную передачу;
реализация full TUF без ambient I/O и migration acceptance пока открыты.

Current source даёт основу, но не runtime-приёмку:

- Windows использует `go-tuf/v2 v2.4.2` ([manifest](../../../../windows/go.mod):13),
  `delivery.PrepareArtifacts`, opaque `Prepared`/`Progress` и value-only
  `prepareAcceptedProfile`. Статический source cut включает OS-bound loaders.
- Android имеет `ProfileTrustDriver.inspect/verifyForApply`, coordinator/store/
  checkpoint fences, но **production driver отсутствует**; `SnowdenApplication`
  его не создаёт (source — `android/app/src/main/kotlin/org/snowden/android/app/SnowdenApplication.kt`:76).
- На срезе 28 октября `shared/adaptive` существует как отдельный чистый Go module/API1.
  Наличие этой policy не доказывает готовность переносимого trust entrypoint.
  Извлечение и интеграция SDK требуют фактического владельца и согласованных
  контрактов; историческое предложение — `docs/tasks/FIVE-STREAMS/TRUST.md` —
  само по себе не разрешает создать роль или код.

### Минимальный граф зависимостей

```text
platform capture / explicit approval / owned bytes
                  ↓
portable schema + application-policy + delivery-workflow
                  ↓
owned Progress / opaque Candidate / typed Rejection
                  ↓
platform CAS + custody + coherent commit + fresh application gate
                  ↓
accepted-profile projection → platform renderer / worker / protected proof
                  ↓
platform accepted observations → existing adaptive API1 → recommendation
```

Trust не импортирует Wails/Kotlin/Windows/native socket/Keystore, `adaptive` или
Manager. Adaptive не импортирует trust/verifier; платформенный adapter передаёт
только нужную projection и ограничительный deadline. Observed health не
подменяет криптографические полномочия, `PlatformAccepted` в adaptive —
корреляционная owner projection, не подпись.

## 2. Знания, нынешняя реализация и disposition

| Семейство знания / сохранённый источник | Current code и план | Разница / решение |
|---|---|---|
| Exact original bytes, отдельный application canonical digest. [OP-03](../../../theory/architecture/SAFETY-AND-TRUST.md#source-16-3), R05-010§2, archive§16.3 | onboarding/delivery.go — `windows/backend/onboarding/delivery.go`:15–37; [metadata.go](../../../../windows/backend/metadata/metadata.go):137–162; TH-03-TUF-INTEGRATION, WIN-SDK-001-T, AND-04-S4b T01/T05/T08 | **Already source-implemented / reuse.** ProfileBytes/TUF metadata base64 byte arrays остаются original; Envelope.Canonical — отдельный исторический wire contract. Не заменять JCS/сортировкой/пересериализацией Kotlin |
| Принятый predecessor ≠ rejected SDK pointer ≠ durable commit ≠ artifact permit. Accepted [R05-001 review](../../../theory/research/reviews/2026-10-27-R05-001-PROGRESS.md)§3, R05-010§7 | `windows/backend/delivery/retained_state.go`:16–79, `windows/backend/delivery/model.go`:314–414; TH-03/FR-008/011, T07/T11 | **Change required before shared freeze.** Current pointer-to-original binding не доказательство final-step acceptance. Сохранять valid predecessor при позднем отказе, но не новый offending T/S; criterion/receipt provenance решить на существующем seam, не фильтровать все errors/progress |
| ResetAllowed, surviving quorum и post-reset acceptance. [Continuity](../../../theory/architecture/TRUST-CONTINUITY-AND-AGREEMENT.md)§1–3, R28-Z07§3–5 | `windows/backend/delivery/retained_inputs.go`:61–99, material-set OR across intermediate roots; retainedState reset+continuation; TH-03, AND T05/T07 | **Adapt explicit contract.** Threshold-only/material diff/root version не один смысл. Current exact-byte continuation/continuity guards сохраняются. Выбрать поддерживаемые rotations; новое поле требуется лишь если существующие coherent images не различают необходимые histories |
| Profile/update/recovery полномочия, original issuer/target/source. [R05-004](../../../theory/research/05-TRUST-SAFETY/R05-004.md)§1–2, TH-03-TRUST | `windows/backend/metadata/trust_policy.go`:154–214; `windows/backend/metadata/profile_recovery.go`:138–231; `windows/onboarding_authority.go`:71–210 | **Reuse value policy, extraction needed.** Schema1 даёт только profile-role, retired key не promote/revive; explicit recovery/epoch/enroll ветви не объединять bool. Store I/O отделить от решения |
| Epoch/counter/equivocation и same-source batch. R05-002§9.2, R05-007§2–5 | `windows/backend/metadata/epoch_grant.go`:120–204; `windows/backend/metadata/accepted_profile.go`:263–445; T08/T11, TH-03-EPOCHS | **Already source-implemented / reuse.** Profile+grant proofs должны иметь один source/next cut; UInt64 epoch/counter нельзя свести к Kotlin Long/Double. Application history сохраняется вне TUF role reset |
| Независимые root+policy approvals, latest-known custody, не global latest. OP-06, R05-010§3–4 | `windows/onboarding_bootstrap.go`:19–59; `windows/onboarding_public.go`:14–94; TH-03/07/COM-13 | **Current Windows guard / compatibility gap Android.** Android ProfileImportApproval имеет root/target, но отдельный independently approved first pin-policy digest не представлен. T02/T04 и authorityFence недостаточны, если approval для baseline не получено отдельно. Нужен Android contract change, не inbound auto-approval |
| Clock/reference/purpose, exclusive authority deadline, старые flows. R05-002§9.6; Release boundaries§1 | SessionValidity — `windows/backend/metadata/validity.go`:18–61; ProfileClock — `android/profile/src/main/kotlin/org/snowden/android/profile/ProfileClock.kt`:81–103; preparedRuntime authority vs preparation TTL | **Reuse; no deadline unification.** TUF fixed start не authenticated UTC и не срок runtime. Не продлевать ранее выданную validity при extraction/reconnect; platform wall/elapsed mapping вне SDK |
| Coherent cut/CAS/custody/actual-return и неизвестный commit. R05-002§9.2/9.5/9.7 | `windows/delivery_state.go`:41–119; EncryptedProfileStore — `android/profile/src/main/kotlin/org/snowden/android/profile/EncryptedProfileStore.kt`:55–145; T01/T11/T12 | **Keep platform-owned.** Existing four-target transaction и один encrypted checkpoint предпочтительнее новой БД. Ошибка/timeout не значит no write; decrypted/Loaded не accepted. Keys/enrollment/history не reset автоматически |
| Verified artifact ≠ OS signer/staged/installed/executing/adopted. [Release evidence](../../../theory/architecture/RELEASE-EVIDENCE-BOUNDARIES.md)§2–4; R05-004§3–6 | Windows backend/update native pair/accepted-image/floor, App apply; Android PackageManager/installer/worker owners; TH-08/OFF-06/AND-14 | **Keep native and install policy platform-owned.** Portable manifest verification не запускает EXE/APK, не аттестует whole process/DLL/splits и не даёт VPN Ready |
| Revocation acquisition/schema/status/freshness/chain/operation раздельны. R05-009/C05 status contracts | Current shared verification driver отсутствует; будущие Android status/provider и Windows trust/provider gate; AND-04/14 | **Defer optional adapter, needs review.** Не включать Google/Sigstore/PKI engine как обязательный минимум SDK. ChainResult/statusAsOf не permit. Frozen R05-009 snapshot SHA conflict указан в coverage; детальные claims не становятся требованиями по mismatched corpus |
| Metadata multirepo agreement не mapping authority/custom/governance. R05-006/007, continuity§4–6 | Current ordinary delivery single-repo, retained root/caches; no production multirepo caller established | **Defer.** Не новая mandatory subsystem. Если выбран required multi-approval, отдельно explicit artifact identity/all required descriptors/whole mapping authority; plain Hashes.Equal не protocol-wide agreement |
| Existing eligibility/evidence/budgets. adaptive API — `docs/adaptive/API.md` | shared/adaptive Eligible/Rank/Prefer/ComposeEvidence/PlanFailover/SettleFailover/retry/background; ADP-002…006 | **Reuse, do not duplicate.** Trust предоставляет authority/restriction outputs; platform передаёт existing adaptive DTO. Нет второго scorer, schedule, observer, health-history DB или общего “trust confidence” |

## 3. Извлечение по компонентам с сохранением клиента

### 3.1. Что переносимо, а что смешано с platform I/O

| Current component | Portable candidate | Что обязательно остаётся снаружи |
|---|---|---|
| `metadata/metadata.go`, `channel.go`, `bindings.go`, `profile_version.go`, `pins.go` | Legacy strict decode, field order/canonical bytes, key ID/shape, descriptor/pin semantic checks | Store/UserConfigDir/filetxn и Windows split_direct process application. Android не получает право выполнять Windows policy по распознанному полю |
| `accepted_profile.go`, epoch/pin-policy/recovery files | `prepareAcceptedProfile` уже value-only; pure advance/history/role/pin projection можно извлечь с captured old envelope/state | `LoadAcceptedProfile`, `Store.Prepare*`, file presence/custody source capture и save. Новая pure функция не должна повторно открывать source |
| `trust_policy.go`, `delivery.go` | Role-scoped key maps, semantic authority identity и private proof checks | Reader/Write/Seed/migration/native leases. Comments/cache/time сохраняются в full source fence, хотя semantic Digest их исключает |
| `delivery/client.go` и retained/policy files | Package/request bounds, exact owned bytes, allowed predecessor policy, same-batch opaque proofs | Current full Updater вызывает filesystem internally; Windows workspace/native resource owners не переносимы. Нужен отдельный approved workflow seam |
| `onboarding/*` | Public-profile/sidecar/capsule/transfer codecs и bounded KDF API при отдельной фазе | URI/file/network fetch, owned SOCKS, DPAPI, user confirmation, import operation/session retirement. Initial minimum не обязан переносить encrypted credential transfer |
| `update/update.go`, `codec.go` | Exact schema1/2 manifest Canonical/strict codec, signature/hash descriptors | `.exe` filename/platform constraints, native helper pair, floor registry/FS, staging, CreateProcess, installer readiness. Не менять Windows manifest на generic APK schema под видом portability |
| `metadata/runtime_profile.go` | Owned captured value projection части `ProfileSnapshot.projectRuntimeProfile`; immutable runtime authority/pin restrictions | Loader’s legacy-save side effects, read lock, platform renderer/schema/core capability, TUN/route/OS enforcement |
| `secretvault/*` | Только opaque credential references и validated format contract при реально нужном сценарии | DPAPI/Keystore/enrollment/ciphertext storage/custody/FD/ACL/transaction. Реальные vault не portable SDK state |

**Shared/trust — один обоснованный Go module**, не каталог-копия Windows.
Минимально достаточны package boundaries `metadata/policy`, `delivery workflow` и
`facade contracts`; конкретные имена каталогов должны следовать принятому
extraction decision. Общая root-package не импортирует Windows module ради aliases.

### 3.2. Последовательность и владельцы

| Этап / приоритет | Независимый deliverable | Principal owner / existing plan | Зависимости и outcome |
|---|---|---|---|
| M0 / prerequisite | Принять scope/role и source/API freeze; inventory licences/module cut; Android T01…14/Windows WIN-SDK-001 согласование | Harness-owner только регистрация после решения владельца; trust owner API; GOV-003/FIVE-STREAMS, TH-03, AND-04-S4b | Сейчас trust role отсутствует. Не bind/создание SDK из planning. Frozen proposal не actual API publication |
| M1 / P0 trust semantics | На existing Windows seam отделить допустимый progress/new rejected pointer, context/reset и typed error-with-progress | Windows owner, TH-03-TUF/OFF-06, R05-001/010 scoped review; trust owner согласует контракт | Не ждать общего SDK, чтобы исправлять narrowly accepted boundary. Поведение/receipts должны быть выбранным новым baseline для parity, не перенос known defect |
| M2 / P0 extraction | Перенос strict codecs/value-only policy без I/O; Windows thin adapters используют тот же implementation | Trust owner + Windows caller migration, TH-03-TRUST/EPOCHS/PIN-POLICY, WIN-SDK-001-T | No new schema/crypto/DB. Old API forwarding сохраняется до compatibility acceptance; не две независимо применяемые authorities |
| M3 / P0 blocker | Один complete TUF orchestration seam с explicit time/storage/original bytes/progress | Trust owner, OP-03/TH-03-TUF; Android T05/T06 | Варианты только два уже сохранённых: supported full-updater Storage/Clock hook **если реально принят**, либо separately reviewed complete lower-layer workflow. Hooks currently absent; lower workflow нельзя сократить. Пока unresolved — SDK verification capability unavailable |
| M4 / P0 adapter | Windows isolated workspace adapter/CAS/progress commit и Android synchronous driver mapping одного API | Windows/Android owners, OFF-04/06, AND-04-S4b T01…12 | Windows stays existing owner; Android sections/enrollment/generation/cancellation preserved. Bounds и root+pin baseline approval gap resolve before switch |
| M5 / P0 runtime/release compatibility | Whole-libbox Go/JNI inventory и accepted-only renderer/source; Windows publisher/CLI import paths/version compatibility | Android/native/release owner AND-01/03/04/14; Windows release/publisher TH-08/TH-03-ISSUER/PUBLISHER/RELEASE | Один approved AAR/runtime, no Windows module import/second Go AAR. SDK/core compiler graph must include go-tuf minimum1.25 and project/shared1.26; source-lock Go null/stock artifact incompatible |
| M6 / gate | Разрешённая acceptance: same-source parity, negative/malformed/schema/cancel/rotation/runtime/native/durability отдельно | SDK/trust owner pure gates; platform owner OS/device/live; TH-03/FR-008/011/AND-04/14 | Всё NOT_RUN сейчас. Сначала semantic/codec/adapter scope, затем OS/FFI, затем product protected path — build недостаточен |
| M7 / после acceptance | Удаление заменённой pure реализации и compatibility shims в выбранном supported client range | Windows/Android owners | Только когда actual consumers/publisher recipes migrated и regression scope accepted; native adapters остаются платформенными |

M1/M2 можно планировать отдельно от Android whole-AAR: не менять весь клиент
ради первого переносимого codec. M3/M5 не обходить mock/default driver или
DisableLocalCache. Общая программа safety/platform owners продолжает иметь
собственные P0; новая dependency не закрывает их OPEN gates.

### 3.3. Current migration opposing guard: методы Go и live source drift

`metadata.Envelope`/`TrustPolicy` можно сохранять в compatibility wrappers после
переноса pure definition/методов. Но целиком перенести `Store` или его methods
через alias нельзя: Store API содержит loaders/save/filetxn и относится к Windows
owner. Pure acceptance сейчас непубличный `prepareAcceptedProfile`, а
`PrepareProfileRecovery`/epoch/enrollment всё ещё открывают Store. Дешёвый extraction
— сначала value-only functions с borrowed/owned captured old-source inputs;
Windows Store methods остаются тонкими захватчиками/commit wrappers. Нельзя
поменять alias type и добавить новые методы на non-local type как будто миграция
готова; source/API forwarding design должен учитывать methods и compile scope.

Current retention review добавил существенный platform seam:
`retainDeliveryProgress` получает coherent `filetxn.WithRead` source-fence **до**
no-op equality branch; `retainDeliveryProgressWithOutcome` отдельно отмечает
успешный commit callback, не приравнивая неизменный policy digest/отсутствие cache
change к записи. Эти wrappers не входят в portable SDK. В migration acceptance
различать `no_write`, `callback_completed`, `uncertain`, а не выводить effect по
сравнению семантических digests. Даже callback_completed не сам по себе native
power-loss proof. Вводить второй outcome registry не нужно: reuse existing seam.
Source cut до/после этого изменения сохранён отдельно в private evidence.

Чистое extraction допускает механическое использование прежних stdlib
`crypto/ed25519`, `crypto/x509`, `encoding/json` и pinned TUF; это не новая
криптосхема. Однако `Manifest.Validate` содержит Windows `.exe` target policy,
а `Envelope.ValidateVersion` — подписанное Windows `split_direct` grammar.
Эти проверки нельзя выбросить для сборки на Android: codec сохраняет исходный
wire, adapter либо применяет совместимую платформенную policy, либо возвращает
Unsupported. Новый Android target format был бы отдельной schema migration,
не silently generic SDK capability.

На current WIN-SDK-001 V2-184 публичный untrusted file intake использует
original bounded read owner и checked Close; default stored-source readers
сохраняют trusted custody. Это platform capability, не proof SDK acceptance
и не перенос network/file picker в portable layer. Input/output lifetime profile
остаётся внешним adapter gate.

## 4. Frozen interface proposal v1

**Статус: design-only, not implemented.** Имена ниже — proposed portable facade,
не существующие Go functions. API version1 независим от signed schema1/2,
Android checkpoint format1 и IPC/native revisions. Version/capability фиксируются
consumer по accepted source/artifact, не берутся из непроверенного input.

### 4.1. Поверхность вызовов

```go
// Proposed facade; not current source and not permission to implement now.
type VerifierV1 interface {
    Describe() CapabilitiesV1
    Inspect(InspectionInputV1) InspectionResultV1
    VerifyForApply(ApplyInputV1) WorkResultV1
    ProjectAccepted(RuntimeInputV1) RuntimeResultV1
}
```

- **Inspect:** bounded parse/review на captured source, не accepted state, не
  durable progress, no writes/enrollment/credential retrieval. Результат public
  review fingerprint/fence без secret-bearing diagnostics.
- **VerifyForApply:** fresh re-evaluation exact input/source/review/approval/time;
  результаты Progress и Candidate раздельны, оба owned. Rejection/Cancellation
  могут сосуществовать с допустимым existing-root progress. Initial failed
  enrollment не создаёт prefix-only authority.
- **ProjectAccepted:** revalidates captured accepted identity/current applicable
  policy и выдаёт data/constraints; не читает Store, не создаёт legacy anchor,
  не renew grant, не генерирует защищённый graph по raw/preview/opaque loaded bytes.
  Windows/Android actual runtime gate остаётся обязательным.
- **Describe:** неподдельная frozen local capability/schema/source profile. Отсутствие
  complete TUF mode или required producer даёт Unsupported/Unavailable, не success.

Непубличные constructors candidates/proofs; getters возвращают copies.
Для FFI native consumer держит bounded verifier-owned work handle до explicit
release в том же runtime; serialized labels/hash/boolean никогда не создают
новый private proof. Native release/cancellation — explicit owner lifecycle,
а не timer, освобождающий borrowed input. Implementation/ABI handle protocol
не предполагается готовым и требует M5.

### 4.2. Минимальный набор входов

| Вход | Что обязательно сохранить | Чем не заменять |
|---|---|---|
| `APIProfile` | Version/capabilities, accepted implementation/source revision, declared limits | Version из downloaded profile не включит неподдерживаемый native seam |
| `OwnedInput` | Original public bundle/profile/target bytes, package records; bounded codec; raw target digest+length отдельно от canonical content identity | Kotlin marshal/RawMessage-normalization не TUF exact target bytes |
| `CapturedSource` | Original trust/application checkpoints, prior accepted envelope/presence, stable enrollment, retained lineage/cache/history/pins/bans, storage expected revision | Decrypted AES/Loaded/path presence не acceptance; semantic policy.Digest не full source fingerprint |
| `Purpose` | profile/enroll/recover/epoch_recover/epoch_enroll; update_manifest verification отдельный purpose | confirm bool не автоматически root/update/recovery permission |
| `IndependentApproval` | Mode и approved exact root/first pin-policy/target digests **по нужной purpose**, provenance/as-of scope вне SDK trust invention | Digest из того же incoming bundle не independent reference |
| `ReviewFence` | Exact source/policy/target/signer/mode/grant/pin baseline/revision, exclusive preparation validity | Локальный review UUID не криптографическое разрешение |
| `TimeInput` | Explicit fixed workflow reference и effective local observation/floor с domain/continuity; before-apply recheck platform | Ambient time.Now/global override/elapsed≈UTC; signed metadata time не trusted current clock |
| `CancellationView` | Cooperative request и lifetime input/current до actual return | Context timeout не actual retirement/rollback или permission освобождать workspace |

Ключи profile/update/recovery и TUF threshold authorities проверяются внутри
существующего принятого trust-policy. Scope string/ref — binding, не authority.
Runtime inputs возвращают актуальные applied policy/source facts отдельно от
передачи измерений adaptive; trust не повторяет ComposeEvidence.

### 4.3. Field-level freeze и адаптерная граница

Ниже fixed **proposal schema API1**, не новые signed fields существующего профиля.
Семантические имена предлагаются для согласования; конкретный Go/JNI codec ещё
не реализован. Unknown обязательного поля не трактуется wildcard/default.

| Proposed value | Frozen minimum fields / инвариант |
|---|---|
| `CapabilitiesV1` | `api_version=1`, supported operation purposes/signed schema versions, workflow mode (`unavailable`, `approved_private_workspace`, `approved_explicit_workflow`), exact limits, accepted implementation revision. Это local implementation capability, не data от mirror |
| `SourceV1` | Owned `trust_policy_bytes`, `application_state_bytes`, `prior_profile_bytes` с явной presence, retained delivery bytes/cache по текущей schema; `source_revision`/`enrollment_ref` у platform. Внешний SDK не читает Store и не расшифровывает product vault |
| `IdentityV1` | Раздельно raw target SHA256+length, current canonical profile identity, semantic authority SHA, full-source SHA/receipt и storage revision. Для каждого используется его existing encoder/identity rule; не одна новая универсальная digest схема |
| `InspectionInputV1` | Capabilities/profile, Purpose, bounded OriginalInput, SourceV1, TimeInput и CancellationView. Independent root/policy approval required только по соответствующей операции; inspect их показывает, не создаёт |
| `ApplyInputV1` | Те же original input/source/profile плюс returned ReviewFence и explicitly supplied approvals. Context/review не восстанавливать из UI/JSON label после source change |
| `ReviewFenceV1` | Полный existing preview binding: source trust+input/container/target/signer/mode/current profile/epoch grant/delivery anchor/pin-policy/current-policy/first baseline/exclusive validity. Any changed component требует нового review, а не смены одного ref |
| `ProgressV1` | Opaque owned permitted next delivery state, SourceDigest/expected source, classification (`prior_accepted`, `accepted_predecessor`, `authorized_reset_continuation`), stage/reason, reset context evidence. Representation classification new **proposal**, должен быть обоснован current checks; не произвольный enum от caller |
| `CandidateV1` | Private verified result, mode/purpose/exact target/source/next-image binding, required policy target, accepted application fields, applicable exclusive authority bounds. Не public constructible `verified=true` и не generated claim of store commit |
| `FailureV1` | Fixed code, actual known stage либо `unknown/read_or_decode`, bounded field/index, cancellation state; progress independent optional. Never arbitrary exception/credential-bearing text |
| `RuntimeInputV1` | Captured accepted source/current policy+intent ref, specific explicit time; no raw unaccepted container. Platform core/network/application capability остаётся external gate |
| `RuntimeResultV1` | Owned public descriptors/restrictions, exact accepted identity/current policy and separate applicable deadlines; original signed public/application history remains unchanged. No Ready/native permission |

Native bridge не обязан передавать Go structs через JSON. Frozen semantic values
можно передать bounded arrays/decimal integers и native-owned opaque result
handle в **том же** whole runtime, но handle ownership/ABI/error/release profile
фиксируется M5. Source/identity string из Kotlin не mint новый verified proof;
только результат доверенного SDK implementation/accepted driver, его exact checks
и platform gates дают допустимую передачу. Допустимость любой injected реализации
VerifierV1 определяется local owner, не интерфейсом/constructor именем.

`uint64` epoch/counter → полный unsigned range decimal/bytes без Float/Long loss;
время → declared domain, explicit known/unknown и seconds+nanos либо range-checked
integer unit. Time conversion не renew validity. Прогресс меняет только Delivery
часть trust state; Android outer `generation+1/+2` задаёт его store/coordinator,
не portable verifier как владелец транзакции. Windows original encoded checkpoint
и Android container1 кодируются **их** adapters, не общим storage форматером.

### 4.4. Результаты и ошибки

`WorkResultV1` содержит **типизированный итог операции**, а не только Go error:

- `Progress`: optional допустимый predecessor и original metadata, source digest,
  accepted-step/context/reset provenance, expected source, owned next trust state;
  **только ограничения/история**, не profile/EXE permit.
- `Candidate`: optional opaque application verification result; original exact
  profile/manifest identity, canonical identity, next application/trust image,
  same-batch proof bindings, purpose и applicable authority deadlines.
- `Failure`: bounded code/stage/field/index либо honest combined/unknown stage;
  новые rejected records не сохраняются как active accepted floor; prior progress
  при позднем failure не теряется. Никакие credentials/issuer raw logs/URL токены
  или exception strings не выходят в UI/лог.
- `Restrictions/Requirements`: pending policy/context/source/unsupported/custody
  prerequisites. Отсутствие измерения/receipt не заменяется числом0.

Не включать `Ready`, `InstallSucceeded`, `ResourcesRetired`, `LatestWorldwide`
или новый `PlatformAccepted=true` в trust output. Это факты отдельных владельцев.
Output `AsOf` и эксклюзивный deadline не attest current OS clock. Unknown commit
реального store создаёт платформенный Uncertain/barrier; SDK его не угадывает.

Для verification-only update result в первую версию входит **существующий
Windows manifest schema1/2**, не generic APK installer API. APK set/signing/
versionCode/PackageInstaller policy остаются Android-owned до отдельного
existing AND-14 requirement; manifest schema не расширять silently.

### 4.5. Time/progress contract без ложного per-step обещания

v1 default для Inspect/VerifyForApply — synchronous **per-return preparation**:
owned snapshots, без ambient I/O или durable effects. Он не обещает сохранение
volatile verified root при hard kill до return. Это несовместимо с обещанием
«каждый root уже durable до следующего шага», если нет explicit barrier.

Если обязательный workflow требует strict per-step durability, M3 должен
публиковать explicit supported step/continuation или full-updater Storage/Clock
adapter с **platform-owned acceptance receipt**. Нельзя просто добавить callback
и объявить его fsync/native-proof: old/new cut, source identity, actual return,
uncertainty и continued workflow gates должны быть согласованы. Existing journal
может дать носитель, новый wire signed protocol/DB не нужны. Полное поддерживаемое
решение этого gate остаётся открытым, что не мешает M2 extraction pure codecs.

### 4.6. Receipt no-op/commit и alias migration

Portable `Progress` не включает утверждение о выполненной записи. Current Windows
`retainDeliveryProgressWithOutcome` и Android `ProfileCacheWrite` должны оставаться
разными platform adapters с явным mapping: `NoWrite`, `Committed`, `Conflict`,
`RejectedBeforeMutation`, `Uncertain`. `Committed` scoped к договору конкретного
store; он не является attest power-loss/nonrollbackable storage. Re-read/digest
совпадение не fabricates committed outcome, а later application rejection не
удаляет уже подтверждённый predecessor. Если no-op source fence отказывает,
это failure, не новая запись по одному совпавшему semantic digest.

Repository/module/API rename при extraction не новые enrollment. Adapter
сохраняет stable initial anchor/current rotated root/accepted history и observed
source schema/presence; alias/path/private cache directory не назначают новую
trust identity. Нельзя объявить old-client compatible только по сохранённым
именам методов: payload/schema, complete delegated workflow, bounds и supported
purpose из M0/M6 фиксируются перед cutover. Read-only preview никогда не принимает
новую persisted phase, а resumed/old SDK handle нельзя rebind к новой source
revision вместо повторной проверки.

## 5. Compatibility и old clients

### 5.1. Сохраняемые wire invariants

1. **Envelope schema1/2:** Go historical Canonical field order и omitted fields
   неизменны; profile KeyID первые16hex SHA256 public key не заменяются полным
   digest/TUF keyID. UInt64 epoch/counter передавать FFI decimal strings либо
   проверенными unsigned bytes, не Kotlin Long/Double. Source storage generation
   — отдельный signed64 transaction ID.
2. **TrustPolicy schema1/2:** schema1 только profile-role; schema2 не downgrade
   flat key map. Historical retired keys нужны только exact installed-history
   verification, не новой active authority. Сортировка storage encoding может
   быть согласована, но full observed source fence включает исходную schema,
   comments/time/cache/presence; semantic Digest этого не заменяет.
3. **Accepted state schema1/2/3:** epoch namespace/history/origin/pin-policy/
   bans/last-recovery и ClockFloor сохраняются; storage decode migration не root
   enrollment и не сброс historical knowledge.
4. **SNOB1 и public bundle schema1:** encrypted transfer/credentials отдельная
   ветка, optional delivery/epoch/recovery/raw profile bytes не выбрасываются.
   SNOB2 pin-policy wrapper — отдельный поддерживаемый codec. Old strict reader
   вправе reject новые fields — нужна versioned rollout compatibility, не
   «backward compatible потому что optional».
5. **Update manifest schema1/2:** omitted Target/ActivationProtocol/helper не
   меняют schema1 canonical bytes; parsed validation/file policy не приравнивать
   к whole final payload identity. Final signing/hash/staging/native receipts
   идут тем же platform pipeline.
6. **Android checkpoint1:** public/trust/application/credential sections остаются
   owned original bytes. Progress при existing root сохраняет public/app/credentials
   exact; accepted image после progress имеет source+2 generation, без него+1,
   enrollment тот же. Crypto/keys creation+CАS не common DB transaction.

### 5.2. Caps конфликтуют не по названию, а по фактическому пути

| Path | Current cap | Решение v1 |
|---|---|---|
| Windows legacy Envelope reader | 1MiB,≤64channels | Legacy reader cap сохраняется; это не значит TUF target принимает1MiB |
| Delivery profile/manifest/epoch/pin target | 256KiB /64KiB /16KiB /768KiB | Exact purpose caps до clone/parse, не silent truncate |
| Delivery package decoded/encoded/state | 8MiB /12MiB /5MiB | Верхний полный batch/output bound отдельно от transport/container; accepted cache не silently evict для fitting |
| Windows onboarding core | 256KiB; public extended profile с pin-policy больше; protected wrapper≤2MiB | Existing codec determines actual admission; «всё256KiB» неверно для каждого wrapper |
| Android import/public bundle | 256KiB по текущему T-request | Новые larger public pin-policy wrappers **не** совместимы автоматически. Версионировать capability: phase1 поддерживает intersection либо Android owner явно меняет accepted input caps/schema with gates |
| Android trust/app/credential/plain checkpoint | 6MiB /2MiB /256KiB /9MiB | Overall transient copy peak JVM+JNI+Go+workspace выше single limit; required measurements NOT_RUN |

Не увеличить все caps общей константой «SDK max»; producer и receiver выбирают
совместимый negotiated profile **до** обработки, не по непроверенному input.
Нормативные limits и actual measured RAM разные свойства.

### 5.3. Миграционный guard против ложной parity

Старый `retainedState` может отражать rejected intermediate pointer. «Новый SDK
делает ровно то же» не достаточная acceptance. Parity сравнивает approved semantic
baseline: unchanged valid-input bytes и разрешённые transitions; malformed/unsafe
ветки обязаны иметь отдельный **intended divergence**. Evidence acceptance по
source не тест новой реализации. Не удалять old clients/adapters до actual
schema/publisher/JNI compatibility; не применять одновременно две разные authority
реализации. Shadow comparison будущего этапа не выполняет effects и не пишет
вторую trust историю.

## 6. Что остаётся у владельцев платформ

**Windows:** existing App operation/intents/serialization, original file/object
readers, vault prepared transaction/source fences, root+cache commit, runtime
retirement при urgent pin-policy, accepted image/Job/native companion/readiness,
HKCU floors, installer path/ACL/reparse, WFP/TUN/proxy/DNS и protected probe.
Publisher CLI подписывает/выдаёт по своей authority/custody, не через Android SDK
как администратор устройства. Миграция его pure codecs отдельна от размещения
privileged issuer keys.

**Android:** приём URI/Binder/provider; CE/noBackup/Keystore и CAS хранилища;
проверка wall/elapsed/boot в ProfileClock; отмена coordinator и время жизни владельца;
копирование входов/выходов JNI; совместимость полного AAR и native revision;
PackageManager, подписант, текущий профиль/DPC; одобренный renderer и однократный
RuntimeSourceProvider; epoch worker/сети/приложения; fd/protect/bind/lockdown и
защищённый выход. Расшифрование AES, локальный конструктор или согласие пользователя
не заменяют native trust proof.

**Adaptive:** existing candidate/rank/preference/evidence/budget operations.
Не переносить туда reset policy, signing/current authority или storage. Platform
bridge связывает trust restrictions/deadline с existing accepted observation
identity; SDK authority result не HTTP/DNS/tunnel measurement и не health label.

## 7. Future migration acceptance — все NOT_RUN

| Gate | Требуемая проверка / opposing current guard |
|---|---|
| A1 codec/wire | Exact schema1/2 canonical bytes, raw delivered profile/metadata transport; unknown/null/duplicate/alias/trailing reject; publisher uses same canonical implementation |
| A2 authority | Ordinary profile не promote update/recovery/root; retired key не revive; malformed/absent store не Empty. Initial independent root+pin baseline checked, Android approval gap согласован |
| A3 progress | Rejected new expired T/S pointer не accepted floor; old accepted expired predecessor retained; valid timestamp + bad later snapshot yields progress/no candidate; error не теряет progress |
| A4 reset | Supported rotation table (disjoint/overlap/threshold/dual/reuse/S→S′→S) с exact accepted-context provenance; current continuation/continuity guards preserve; post-reset T5 reacceptance не повторно reset |
| A5 batch | Profile+epoch grant+pin-policy private proofs same source/next image; unknown/mixed proof/forged public hash labels rejected; default/zero channel projection различить raw Envelope requiring≥1 records |
| A6 time | Fixed workflow time/application admission/runtime authority/preparation freshness distinct; ms truncation never extends expiry; old carrier constraints retained until full retirement; no reconnect renewal |
| A7 storage | Windows4targets и Android1encrypted image, original source/custody/full CAS; committed/uncertain/conflict/cleanup distinguished. Explicit promise per-step/per-return; no malicious whole-blob rollback guarantee from AEAD alone |
| A8 ownership | Cooperative cancel не releases inputs/workspace/FFI handles before actual return; late cleanup strips artifact proof but preserves permitted prefix; no callbacks under inappropriate App/journal/vault locks |
| A9 Android/native | Whole-libbox SDK package integration, public capability inventory, exact compiler/module/ABI/R8/16KiB/copies; stock/marker-only or separate Go runtime unsupported. Driver T01…14 exact mapping |
| A10 runtime/protection | Only accepted projection→actual renderer; current source/pin/grant/context verified before one-shot native; full protected proof separate from SDK acceptance/build |
| A11 update | Exact final signed EXE/APK bytes, intended source/candidate identity, OS signer/set, staging/install/execute/adopt and rollback floor separate; first candidate initialization controlled by prior trusted owner |
| A12 cutover | Valid supported old-client/publisher bytes preserved; intended safer divergence explicitly reviewed; one active authority implementation; actual tests/build/native/device/live commands require separate authorization |

Current guards — важный opposing baseline: opaque `Prepared` constructors,
CheckDeliveryProof, source-fenced snapshots, same-batch Epoch proofs, role-scoped
maps, pending pin-policy obligation, original input ownership/CAS и Android
strict no-default driver. Не выбрасывать их ради «общего DTO» или одного bool.
Сейчас acceptance таблица не исполняется и не создаёт fixtures.

## 8. Остаток и границы предложения

- Не решено: поддерживаемый full TUF Storage/Clock seam или complete lower-layer
  workflow; strict per-step promise versus chosen per-return unit; полный
  ResetAllowed для всех overlapping policies. Эти gates не маскируются names/API.
- Не опубликованы: role trust/SDK, production Android driver/accepted renderer,
  whole compatible AAR/artifact/compiler proof и platform parity/durability.
- Frozen inventory включает конфликт source hash `R05-009`; у пяти прочитанных
  units bodySha256 совпал, но whole source snapshot отличается. Они отмечены
  `needs_review` в private coverage: scoped body identity сохранена, whole-source
  frozen consistency не доказана. Непросмотренные late Z01/Z08 units и оставшийся
  corpus перечислены отдельно; crosscheck не заявляет full corpus coverage и
  не создаёт статус primary owners.
- Current source crosscheck зафиксирован отдельными selected hashes. Последний
  прочитанный WIN-SDK-001 request — V2-184/public-input changes; файл позже вновь
  отличается от reviewed snapshot. Более поздний хвост **late/unreviewed**,
  руководитель сопоставляет его с M0/M4/M6. Параллельный writer не остановлен;
  repeated polling не даёт frozen whole-repository identity. Остальные selected
  inputs совпали на final comparison, атомарность всего дерева не заявляется.
- Новая БД, новая криптосхема, automatic telemetry/status service, multirepo,
  обязательная биометрия каждого reconnect, второй adaptive engine и импорт всего
  Windows клиента в Android — **не входят в минимальный SDK**.
- No current performance/native/OS/runtime advantage claimed. Лицензия нового
  extracted project module должна быть явно закреплена после corresponding-source
  audit; Android-origin GPL-3.0-or-later не переписывает лицензии прочих packages.
  Saved go-tuf2.4.2 Apache-2.0; candidate Sigstore/Google status не выбран как новая
  dependency. Native signing/PKI platform licences и полного graph SBOM — отдельные
  gates TH-08/AND-14, не inferred from library README.

## 9. Основные источники применения

- FIVE-STREAMS/TRUST — `docs/tasks/FIVE-STREAMS/TRUST.md`,
  WIN-SDK-001 — `docs/tasks/CHANGE-REQUEST-WINDOWS-SDK-001.md`,
  Android T01…14 — `docs/android/CHANGE-REQUEST-TRUST-001.md`.
- [SAFETY-AND-TRUST](../../../theory/architecture/SAFETY-AND-TRUST.md)§16.3/16.6,
  [Trust continuity](../../../theory/architecture/TRUST-CONTINUITY-AND-AGREEMENT.md)§1–3,
  [Release boundaries](../../../theory/architecture/RELEASE-EVIDENCE-BOUNDARIES.md)§1–4.
- [R05-001 review](../../../theory/research/reviews/2026-10-27-R05-001-PROGRESS.md),
  [R05-002](../../../theory/research/05-TRUST-SAFETY/REPORT.md)§9,
  [R05-010](../../../theory/research/05-TRUST-SAFETY/REPORT-010.md)§2/7,
  [R28-Z07](../../../theory/research/cycles/2026-10-28-contracts/Z07/REPORT.md)§3–5.
- Adaptive API1 — `docs/adaptive/API.md`, ADP-002…006,
  [PLAN](../../../PLAN.md) TH-03/TH-08/OFF-04/06,
  ANDROID-PLAN — `docs/ANDROID-PLAN.md` AND-04-S4b/AND-13/14.

Это ограниченное сопоставление выбранных материалов и исходников 28 октября.
Совпадение отдельных тел R05-009 не устанавливает идентичность полного снимка;
поздние материалы данным разбором не прочитаны и не считаются отсутствующими
или автоматически подтверждёнными. Общее сопоставление и актуальные этапы
сохранены в двух планах платформ. Предложение границы SDK не меняет научный
статус исходных утверждений и не доказывает его реализацию.

<a id="минимальный-dependency-graph"></a>
<a id="3-component-level-extraction-без-замены-клиента"></a>
