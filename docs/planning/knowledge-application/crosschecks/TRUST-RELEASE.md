# Trust/release: готовность применения накопленных знаний

Срез delivery/custody от 28 октября. Последующая реализация и аттестация
артефактов не подтверждаются этим разбором; текущая очередь и отдельные gates —
в [плане ПК](../PC-PLAN.md) и [плане Android](../ANDROID-PLAN.md).

## 1. Состояние и приоритет

**Следующий полезный hardening — приёмка происхождения retained TUF progress,
затем результаты commit/cleanup и межплатформенная parity.** Новая общая
классификация trust-result либо ещё один verifier не заменяют эти работы.

Windows уже имеет source-only gates для точных policy/checkpoint/profile bytes,
активных ролей, pins/revocations, preview/intent/deadlines, vault source CAS,
fixed-file transaction и install image custody, включая **V2-183** urgent
pin-policy source и common prefix outcome. Их нельзя вновь назначать
«отсутствующими» по историческим отчётам. Эти реализации **не прошли текущую
SDK/native/durability/release приёмку**. Публичная установка schema2 App/helper
по-прежнему запрещена `checkUpdateInstallSupport`.

Android имеет интерфейс `ProfileTrustDriver` и coordinator/storage guards, но
**production driver и accepted-only runtime source не подключены**. `shared/trust/`
и `docs/trust/` в проверенном checkout отсутствуют. FIVE-STREAMS/TRUST — предложение
извлечения API, не доказательство уже опубликованного SDK или активированного owner.

Existing requirements: **FR-008, FR-011, TH-01/03/08, OFF-04/06**; Android
**AND-04-S4b, AND-13/14, T01…T14 CHANGE-REQUEST-TRUST-001**. Их статусы здесь не
изменяются. Самое раннее безопасное разбиение: одна platform change в existing
`delivery.retainedState`, один outcome/cleanup integration slice и отдельно
portable contract parity. Initial install, update, profile import, new-recipient
export и recovery не получают единого взаимозаменяемого permission.

## 2. Знание → текущий seam → дельта → решение

| Семья | Полезное знание и источник | Текущий код/ID | Дельта и disposition |
|---|---|---|---|
| **TR1: persistable progress** | SDK mutation не equals accepted predecessor; incoming expired T1000 не следует сохранить только из pointer. Full Refresh error не отменяет valid earlier step. [R05-010 §§2/7](../../../theory/research/05-TRUST-SAFETY/REPORT-010.md), frozen C05 §5 | `windows/backend/delivery/client.go:150–173`; `retained_state.go:39–67`; `retained_inputs.go:27–55`; `windows/delivery_state.go:41–138`; OP-03/TH-03, V2-169 | **Change required после scoped review:** classify prior/new timestamp before NextState retention. Existing exact-byte binding/continuity/Store fences сохраняются. Actual disk lockout/permit bypass не доказаны |
| **TR2: coherent evidence/gate** | Typed result и hash не дают coherent current witness. Late old result/new deny/time expiry требуют final gate. [R28-Z06 §§5/6](../../../theory/research/cycles/2026-10-28-contracts/Z06/REPORT.md) | `metadata/trust_snapshot.go:39–60`; `onboarding_import_source.go:29–132`; `update_delivery.go:149–190,224–264`; `onboarding_import.go:308–408`; V2-179…182, TH-01-PUBLIC-READERS | **Already source-implemented для рассмотренных import/export/update gates; acceptance открыта.** Late urgent pin-policy capture тоже внедряется; не дублировать witness-модуль/lock-owner |
| **TR3: denied authority/history** | Fresh consent/root snapshot не снимают previously accepted bans/roles/floors; cache expiry не deletion authority. [Trust continuity §§1–3](../../../theory/architecture/TRUST-CONTINUITY-AND-AGREEMENT.md), R05-005 §6 | `metadata/trust_policy.go:171–213`; `accepted_profile.go:327–446`; `pin_policy_checkpoint.go:16–135`; `runtime_checkpoint.go:63–88`; `profile_recovery.go:163–224`; TH-01-PIN-2, FR-008/011 | **Reuse существующих алгоритмов.** Material-overlap/reset normative scope требует review; runtime checkpoint immutable identity уже guarded, новое универсальное retirement store не нужно |
| **TR4: export/recipient compatibility** | Independent root/policy approval, exact installed profile, cumulative bans и реальный secret ID; clean enrollment не rollback repair старого получателя. [SAFETY-AND-TRUST §16.6](../../../theory/architecture/SAFETY-AND-TRUST.md), R05-004/005 | `onboarding_export_proof.go:67–214,217–294`; `onboarding_export.go:172–309`; `metadata/legacy_export.go:12–40`; `secretvault/identity.go:71–95`; TH-03-EXPORT, COM-13, AND-13 | **Already source-implemented + compatibility acceptance.** Legacy export rejects unsupported inherited bans/epoch/delivery; enrolled rebase берёт private SDK root и перепроверяет. Older enrolled client/full root chain support **defer**, не fresh-root reset |
| **TR5: transaction/cleanup outcome** | Completed state change и cleanup/final-read failure могут сосуществовать; error не означает «ничего не изменилось». [R05-004 §6](../../../theory/research/05-TRUST-SAFETY/R05-004.md), R05-010 §4 | `filetxn/filetxn.go:74–186`; `journal_targets.go:64–79`; `journal_io.go:137–144`; `secretvault/identity.go:183–212`; late `retainDeliveryProgressWithOutcome:125–138`; V2-173…177, TH-01-IMPORT-JOURNAL | **Change required в callers/diagnostics, не transaction rewrite:** публиковать completed/prefixRetained независимо от CleanupError; no-op и stable Digest не commit witnesses. Export/urgent pin prefix outcome late-fixed; GUI/owner recovery и durability NOT_RUN |
| **TR6: release/install/executing/adopted** | Approved bytes/OS signature/staged file/process/image/current code/readiness — разные факты. Self-check не предотвращает первое выполнение candidate-controlled кода. [RELEASE-EVIDENCE-BOUNDARIES §§2–4](../../../theory/architecture/RELEASE-EVIDENCE-BOUNDARIES.md), R05-004 §§1/4/6 | `updateapp.go:149–324`; `update_pair.go:178–295`; `update_image.go:25–32`; `update_accepted_image.go:18–71`; `core/update_code_acceptance.go:59–171`; AND-14/TH-08 | **Preserve unsupported schema2 gate.** Native source wiring не повод enable. Legacy cmd.Start+Process.Release не adopted receipt; initial missing pair receipt не release-integrity proof. Staged hashes/leases существующие, full OS-loader/whole closure **needs acceptance** |
| **TR7: time/status as-of** | capturedAt не status knowledge time; authenticated latest неизвестен offline; fixed SDK RefTime не fresh execution time. [RELEASE-EVIDENCE-BOUNDARIES §1](../../../theory/architecture/RELEASE-EVIDENCE-BOUNDARIES.md), R05-005 §§2/3/6 | `metadata/clock.go:38–109`; `delivery/model.go:375–379`; `update_delivery.go:170–190`; `onboarding_export.go:268–304`; `ProfileImportCoordinator.kt:263–288`; RQ-18/TH-03 | **Reuse локальных observed-clock/deadline guards; no authenticated UTC claim.** State/callback deadlines не продлеваются очередью/KDF/clone. Google/OCSP status adapters отдельно; в current core не интегрированы, новый web verifier не приоритет |
| **TR8: portable SDK boundary** | Exact bytes/application policy/root/progress/history semantics переносимы; OS I/O/custody/JNI/enforcement нет. DisableLocalCache не diskless full Updater. [SAFETY-AND-TRUST §16.3](../../../theory/architecture/SAFETY-AND-TRUST.md), FIVE-STREAMS/TRUST | `delivery.PrepareArtifacts`, `metadata.prepareAcceptedProfile`; `ProfileTrustDriver.kt:121–165`; CHANGE-REQUEST-TRUST-001/T01…14; AND-04-S4b | **Contract/extraction first; current implementation отсутствует.** T07 wording про error/expiry progress нуждается в provenance qualifier; не копировать Windows workspace/native locks в portable API и не писать второй AAR |
| **TR9: multirepo/agreement** | Одинаковые bytes/hash не permission одной purpose; mapping priority fallback не AND, heterogeneous hash compatibility not equivalence. [Trust continuity §§4–7](../../../theory/architecture/TRUST-CONTINUITY-AND-AGREEMENT.md) | В inspected profile/update path один full updater + local role policy; multirepo new consumer не найден; TH-03/08 | **Defer deployment/new SDK scope.** При будущем явном use case определить equality/required approver constraints/source mapping authority, не импортировать решение «threshold всё защищает» сейчас |

## 3. Точные текущие apply/export/update paths

### 3.1 Profile import и explicit recovery — Windows

`ApplyPublicProfileSource(token, approvedTarget, confirm)` consumes pending token
once (`onboarding_source.go:208–238`) → exact preview/expiry/mode binding →
`importPreparedProfilePayload` → `captureProfileImportSource` из coherent
TrustSnapshot → owned delivery verification **вне** journal/native scope →
separate eligible prefix retention → `bundleAuthorityInput.prepare` под source
scope → descriptors/credentials → retire previous session → synced fixed4
stage → `sourceFenceInScope` + vault final CAS в `Store.BeforeCommit` → commit →
separate lifecycle restart/publication.

Действующие guards:

- `onboarding_public.go:58–89`: normal import не назначает новый root/recovery
  authority; enroll/recover/epoch_* имеют разные explicit approvals.
- `onboarding_authority.go:71–249`: incoming bundle keys не promote retired/
  foreign role; exact recovery permit/grant/source identity обязательны.
- `profile_recovery.go:163–224`: intact checkpoint, current RoleRecovery,
  target/source hash и версии; exact replay может быть idempotent, но не новая
  authority. Old envelope для такого recovery не требуется — это допустимая
  положительная ветка, не обход corrupted checkpoint.
- `accepted_profile.go:373–442`: cumulative ChannelID/certificate bans, запрет
  resurrection и downgraded signed-pin identity; новый ID не снимает global
  certificate ban. Runtime-only writer меняет лишь TimeFloor по observed bound.
- `onboarding_import.go:305–408`: source/time/proof/preview deadline повторяются
  после credential work и после staging; Store writer/vault final fence общие.

**Не предлагать заново:** просто добавить final policy CAS или запрет hash-only
acceptance — они уже есть в данном source cut. Actual cooperative writers,
filesystem/ABI/ACL/power-loss order и postcommit UI readiness ещё не доказаны.
Source fence перед filesystem mutation не доказывает корректность каждого OS
syscall или исполнение новой VPN session.

### 3.2 Export и чистый получатель — Windows

`ExportPublicProfileSource` consumes export token → installed profile/current
active signer proof → required cumulative pin policy и fresh delivery package →
source-side acceptance → `rebaseExportProfile` из **NextState.Root** → full
verification rebased package → record actual renderer-requested SecretIDs →
Seal вне App/journal/vault/native locks → final public source/encrypted vault/
clock/proof/deadline comparison → return ciphertext.

`prepareEnrolledExport:68–73` запрещает recovery/secret-bearing publisher package;
`:92–102` связывает incoming profile с installed canonical identity/current
signer. `rebaseExportProfile:217–264` — новый recipient enrollment, не очищение
истории arbitrary old client. `CheckLegacyExport` переиспользует actual fresh-
acceptance algorithm и rejects, если signed envelope не переносит cumulative
bans; private local floor не выдаётся как подписанная policy.

Compatibility-проверка нужна, а не копирование vault:

1. Existing nonempty recipient не превращается в Empty при missing key/profile.
2. Schema1/enrolled/global-policy/epoch clients различаются; strict old decoder
   должен честно unsupported, без downgrade stripping новых constraints.
3. Missing/ambiguous explicit SecretID не kind-first fallback; future disabled
   channel provisioning отдельно.
4. Root и policy digest от approved issuer/custodian независимы от encrypted
   inbound bundle. Password/AES integrity не authenticates root selection.
5. Source retained prefix может commit без выдачи transfer. Late read/Seal error
   не причина откатывать legitimately accepted history.

### 3.3 Update и future pair release — Windows

`CheckUpdate` — point-in-time preview. `ApplyUpdate` повторяет full prepare,
version/install-support/payload hash, удерживает restart intent, staged payload
снова проверяется; `withVerifiedUpdateModeLocked` под current TrustSnapshot
сравнивает полную policy/checkpoint, purpose floor и fresh delivery proof **перед**
отдельным filesystem callback. Основание immutable-source уже есть; не строить
параллельный generic witness framework ради пяти названий.

Для schema2 `applyVerifiedPair`/recovery/readiness уже source-wired, но public
`checkUpdateInstallSupport` возвращает **errUpdatePairInstallerPending**. Gate
сохранять до native/release acceptance. В том пути есть original native roots/
payload handles, pending transaction, NEW companion/current-code acceptance,
prospective installed-pair lease и GUI transfer; source ownership сложнее
«одна подпись = install succeeded». Не закрывать OFF-06/TH-08 по одному pointer.

Legacy `launchUpdatedApplication` использует cmd.Start/Process.Release; честно
пишет «готовность ещё не подтверждена», но return text «обновление применено»
не должен означать adopted/readiness. Это migration/interface acceptance slice,
не повод force-enable pair installation. `checkAcceptedInstalledPair` при
receipt=nil возвращает initial-install состояние, **не** подтверждённый release.

OS signing gate и delivery policy не одно. В этой crosscheck actual full
Authenticode/CRL/image DLL enforcement/first-execution coverage не audited;
невозможно заявить отсутствие/полноту по одному search result или имени helper.
Explicit platform release policy должна определить, какие каналы first execution
контролирует доверенный installer, а какие имеют only app-start verification.

### 3.4 Android: имеющиеся fences не substitutes production trust

`ProfileTrustDriver.inspect/verifyForApply` — synchronous/offline interface
без production реализации; `SnowdenApplication.kt:76–77` это прямо оставляет
unavailable. `ProfileCheckpointGuard` проверяет shape/source/owned docs,
**не** TUF/canonical signatures. `ProfileImportCoordinator.applyReviewed` уже:

- повторно загружает current store и matches preview source;
- удерживает input до actual SDK return;
- записывает separate deliveryProgress, accepted doc не alias;
- снова проверяет clock/authority deadline перед gate admission;
- cancel после admission = TOO_LATE, store outcome отдельно;
- uncertainty удерживает ownership, не заменяет executor/создаёт Empty.

Это хорошие existing scaffolding gates, но T01…14 ещё **requested**. Критический
вход — accepted SDK version и correctness output relationship, затем Android
native/whole libbox/JNI/renderer, затем real import/Connect. Не добавить «пустой
успешный driver» или локальную Kotlin canonicalization ради видимой функции.

## 4. Порядок минимального hardening

### H1 — eligibility retained progress раньше нового SDK/external status work

**P0, Windows profile/security; TH-03/OP-03/FR-008/011, V2-169 continuation.**

Дельта остаётся на exact seam:
`PrepareArtifacts:150–173` получает SDK pointers и вызывает `retainedState`
**до refreshErr**, `retainedState:39–51` bind/continuity помещает timestamp в
NextState. Saved go-tuf2.4.2 `UpdateTimestamp:151–157` assigns перед expiry failure.
`State.Validate` shape и оригинальная byte equality не доказательство final
acceptance. Retention callers действительно пытаются persist prefix, но
subsequent `deliveryErr` aborts artifact — **это риск ошибочного restriction
floor/lockout, не доказанный signature/expiry permit bypass**.

Минимальный plan change: принять narrow eligibility criterion для **timestamp**:
exact prior-accepted predecessor H либо newly applicable final condition F,
плюс existing guards G. [R05-010 §7](../../../theory/research/05-TRUST-SAFETY/REPORT-010.md)
даёт candidate `G ∧ (H ∨ F)` на precise pointer/order pin. SDK RefTime должен
быть same captured instance time; наш observed-now не подставляется вместо него.
Это **proposed source-derived criterion**, не опубликованный SDK step receipt.
Для snapshot postassignment есть ещё final link/version guard — timestamp F
на него автоматически не переносить. Пока происхождение неизвестно: retain
допустимый previous cut, offending input отдельно, no new accepted cache.

Сохранить positive cases:

- previously accepted B теперь expired → historical retention, не новое permission;
- new valid timestamp, затем snapshot/targets/persistence error → eligible prefix;
- valid sequential root/online reset progress → separate continuation;
- old+B не объявить discarded целиком из-за общего Refresh error.

Самый дешёвый sufficient implementation **не** replay второго полного verifier,
не запрет любого progress при ошибке и не новое journal поле по умолчанию.
Требуется scoped review classification/phase requirements перед кодом.

### H2 — commit outcome и retirement fault до user-visible release claims

**P0/P1, Windows lifecycle/storage + UI; TH-01-IMPORT-JOURNAL/OFF-04/06, COM-13/14.**

Source filetxn commit point — removal recovery barrier, `completed=true`.
Postcommit resource Close/debris failures видны через Check/CleanupError и
не отменяют returned completed commit. Поэтому consumers обязаны различить:

- no-op (same state, coherent fence checked, no commit callback);
- prefix committed / profile or policy committed;
- commit unknown / recovery pending;
- cleanup/retirement unconfirmed;
- new process launch requested / adopted / protected-path proven.

Late source **уже** добавляет `retainDeliveryProgressWithOutcome` и переводит
export/urgent-pin prefix flags на actual callback outcome. Stable `TrustPolicy.Digest`
исключает delivery freshness/cache и поэтому не свидетельствует о commit.
Не менять Digest ради диагностики: use existing outcome helper. Current import/
update call sites надо сопоставить с outcome/cleanup result, а не заставлять
Store ошибкой «опровергать» completed operation.

План: подключить `CleanupError`/Check к соответствующему operation result/UI и
блокированию нового staging, сохранив public API compatibility там, где ещё
нужен старый string preview. Ошибка after-commit не вызывает автоматический
повтор импорта/новую передачу/запуск старого binary. Wire изменение отдельное
решение владельца, не новая универсальная state machine в SDK.

### H3 — finished coherent capture/reader parity, не duplicate locks

**P1, Windows; TH-01-PUBLIC-READERS V2-179…183.**

Freeze перечисление critical callers: initial capture, SDK-outside-lock,
prefix/no-op fence, readonly preview, final writer BeforeCommit, recovery,
post-KDF export, current-code/floor check, post-install receipt. На каждом
проверить полную source identity/presence/schema/checkpoint/intent/time.

Late pin-policy code уже switched на `captureProfileImportSource`, released
journal до SDK, uses source.withCurrentSource и staged final fence; это
**already source change**, не следующий P0 rewrite. Same-current guard tests,
cooperative reader/writer interprocess races, existing-only lock absence,
postcommit source-read/Close errors и wrong journal binding — acceptance gates.

Одна native lock identity/owned objects должны покрывать all writers; local
Serialize сам не interprocess exclusion. Не держать SDK/DPAPI/native slow
callbacks под App/native-reader locks ради «атомарности». Полная malicious
state+backup rollback detection не доказана shared lock/readback.

### H4 — один portable contract, parity и Android support matrix

**P0 AND-04-S4b / TH-03 extraction owner после отдельной авторизации.**

Переносимое ядро можно описать existing pure bounded-byte helpers:
policy/role/profile/epoch/history/canonical source checks, eligible progress,
private proof kind/path/artifact/source/deadline. Platform storage/lock/native
reader/writer/DPAPI/Keystore/JNI/network/floor/image custody — explicit ports.

T07 CHANGE-REQUEST сейчас говорит progress даже при cancellation/expiry;
уточнение: только **eligible predecessor** по provenance, не все pointers
после error. T11 source+progress+accepted generation отношение должно быть
доказано shared semantics, не только внешним Kotlin shape check. T10 deadline
не preparation TTL и не аппаратно удостоверенное время. T14 тот же whole libbox,
не второй AAR/Go runtime.

Два совместимых варианта FS scope: принять explicit private-workspace adapter
на платформе либо обоснованный complete lower-layer workflow/storage-clock seam.
Настройка DisableLocalCache+Fetcher не diskless доказательство. Не обещать
одновременно strict «нет FS вообще» и unchanged full updater с workspace.
Изменить requirement owner-requested, не silently ослабить driver contract.

Golden parity должна сравнивать owned original bytes/history/error/progress,
roles/epochs uint64, mixed schema/absent/numeric legacy floors, profile candidate
zero active channels, snapshot links, and negative approval/cancel outcomes.
Whole adapted AAR/compiler graph/ABI16KiB/package identity отдельно от pure
Go correctness. Не activate import/Connect без production driver+source.

### H5 — двухустановочная совместимость и release acceptance

**P1, Windows/Android release owners; TH-08/AND-13/14.**

Сначала paired/current-code native gates и actual signed build evidence,
затем expose installable schema2/публичный release UI. До этого migration
матрица: old unpaired executable, helper-pinned build (singleEXE update запрещён),
new pair, clean initial recipient, enrolled old root, epoch legacy/profile.
Package installation state и current process state не alias. Recovery только
current-authorized forward/special explicit policy, no rollback floors reset.

Windows: verify final bytes→original staged objects→installed pair→prospective
process/root/current-code→GUI receipt→source/intent→service/readiness; DLL/OS
loading trust scope документировать отдельно. Android: signed delivered APK set,
SDK-dependent signing lineage, installer policy/session/splits/install outcome,
current process incarnation, whole approved AAR и accepted source; APK signing
не common TUF acceptance. Host apksig reuse из корпуса только для соответствующей
release toolchain, не all-purpose phone verifier.

## 5. Минимальные входные/выходные контракты

| Seam | Входы | Выходы / гарантируемая область | Что platform сохраняет отдельно |
|---|---|---|---|
| Pure delivery prepare | Owned exact State/Package/Request set, explicit fixed time/limits | Error + classified eligible Progress; полный ProofFor каждого request лишь после full success | Storage/CAS/reset phase/durable result; actual clock assurance |
| Application acceptance | Current active role keys, retained profile/epoch/pin/history, exact envelope/proof/purpose, approval/context | Prepared next policy/checkpoint с exact source/deadline binding | Physical user action, OS signing/install/worker/TUN readiness |
| Prefix commit | Same current source, eligible prefix, fixed targets/Keep и vault CAS | no-op / committed / unknown + original error; artifact permission absent | Postcommit CleanupError/Check, current barrier/resources |
| Import apply | Frozen approval/source/current state + credentials, lifecycle intent; final time after staging | Profile/policy commit result и отдельно launch/retirement result | Current engine config/generation, protected-path proof |
| Export | Approved installed profile+fresh package+recipient mode/root/policy reference, actual selected credentials | Только ciphertext после final source/vault/time gate; prefix outcome отдельно | Recipient independent approval/revalidation; не cloning source history |
| Update apply | Exact signed manifest/target proof/final payload bytes/current source/floor/purpose | Stage/install/activation distinct outcomes; unsupported schema2 preserved | Windows image/Job/loader/GUI/floor durability; installer failure coverage |
| Future Android driver | Borrowed bounded bytes до synchronous return, explicit source/time/approval | Owned eligible progress/accepted output/rejection/deadline, без fake ready | Kotlin coordinator/store/JNI/native support и clock continuity |

Термин SDK «успешно» не должен означать одновременно proof, persisted progress,
secret write, installation и защищённую сессию. Это не требует новых перечислений
на каждом шаге: существующий result/context/owner может уже нести нужную область.

## 6. Сильные opposing guards и случаи будущей приёмки — NOT_RUN

| Case | Какой guard/дельта проверяется | Отрицательный и положительный исход |
|---|---|---|
| A1 | Incoming signed expired T1000 vs previously accepted B expired | Reject retention C; retain B as history. Ни один не свежий artifact permit |
| A2 | Fresh T11, target snapshot/hash/persist затем failed | Eligible prefix retained/error, proof absent; blanket drop-all-error нарушает positive case |
| A3 | Sequential root change S→S′→S или overlapping signatures | Existing intermediate scan/reset continuation recognised; не endpoint equality/не «all rotation fixed» без scope |
| A4 | New authoritative deny после preview/KDF/staging | Full source/final time check rejects stale apply/export/update; timestamps не auto-renew |
| A5 | Current profile normal vs missing envelope recovery | Ordinary import denies corrupt missing anchor; legitimate intact-checkpoint recovery works via correct permit/grant |
| A6 | Late commit cleanup/Close failure | Completed operation не retry, Check/CleanupError barrier visible; unknown commit не complete |
| A7 | Prefix no-op vs cache-only commit без stable Digest change | actual outcome helper distinguishes, old digest heuristic rejected |
| A8 | Wrong journal binding/vault mutation/second writer | WithPreparedTransaction final check after full stage denies; no independent lock bypass |
| A9 | Export old root changed extra roles/omitted cumulative bans/ambiguous kind | private actual SDK root+rebase+ban/reference guards; clean recipient not existing install repair |
| A10 | Pair schema/helper-pinned build legacy update request | Public support gate stops before mutation, even if all source callbacks exist |
| A11 | Correct installed hashes but new process/loader/GUI admission fails | Installed not adopted/not protected; old mapped App remains sticky constrained |
| A12 | Full local backup rollback + genuine consent | SOURCE/HISTORY_UNPROVEN under chosen adversary model, не magic independent anchor from AEAD/hash/QR |
| A13 | Android driver shape/CAS проходит, signature/progress metadata не authorised | Production SDK rejects; outer Kotlin guards не verifier |
| A14 | Authority deadline crossed between Android admission/store return | Named admission/commit/revalidation rule; source-only code не promise instant all-OS atomicity |
| A15 | Correct OS/store signer via another install channel | Scope first execution explicitly limited; app self-check не prevention независимо от канала |

Plan-only позволяет закрыть лишь **документированный mapping/proposed order**,
не эти product claims. Без разрешённой unit/native/VM/device/crypto приёмки нельзя
объявить compile/ABI/SDK parity, durable root/reset progress, crash-safe disk/registry,
interprocess writer exclusion, same-image execution/GUI adoption, cold repair,
two-device transfer, hardware KeyStore support, exact OS signing/enforcement или
protected VPN egress. Historical local PASS и нынешний source hash не заменяют
актуальный result на изменённых входах.

## 7. Готовое reuse, совместимость и цена

Использовать уже собранные компоненты, не новые acquisitions:

- go-tuf **v2.4.2 / f5edbde31e5507f46db2069402dc38903fe6d9d4**, Apache-2.0 по
  сохранённым LICENSE/materials: full workflow/role/check APIs остаются crypto
  implementation. Новая classification/extraction зависит от точного
  postassignment/RefTime порядка; version upgrade повторно reviews этот seam.
- Current filetxn/metadata/secretvault/ObservedClock/Manager source owners:
  reuse fixed4 journal, bounded original handles, source snapshots, role/history
  helpers. Это platform/native properties, не автоматическая portable licence/
  correctness аттестация. Android source GPL-3.0-or-later, licence closure shared
  extraction/distribution requires existing publisher inventory and future review.
- Saved Android apksig **61f88cbe…**, Apache-2.0 — host release verification
  candidate; Result.isVerified/SDK-range/lineage semantics, не signature-only
  root approval. Google keyattestation/Gson/Jackson/networknt материалы корпуса
  относятся к optional attestation/status tooling, **не** текущему signed
  profile/update verifier. Не добавлять JVM network status dependency в core
  ради прошлой научной темы.
- OS/PackageInstaller/JCA/Keystore/Win32 API — existing platform seams, не
  новый общий solver, TUF fork или install protocol. Не вводить MSIX/Store/
  multirepo/transparency deployment как бесплатный hardening без выбранного сценария.

Стоимость здесь не измерена: copy peak/CAS lock latency/DPAPI/KDF/fsync/JNI/build
size/CPU/native handles/certification/rollout. Cheapest first slice — narrowed
eligibility и existing final fences/outcome propagation. Additional database,
shared global-clock/logger/env, synthetic new signer, opaque SDK filesystem cache
или broad reset API увеличивают scope и не доказательства безопасности.

## 8. Late applicability и границы покрытия

Crosscheck основан на **67 явно прочитанных frozen units**, а не всём corpus.
Последний source applicability cut **2026-10-28T11:13:41Z**: поздняя запись
ANDROID-PLAN лишь фиксирует уже отмеченный AND-02-S5/ADR-062; trust driver и
AND-04-S4b по-прежнему не реализованы. Новая запись не меняет приведённый
trust/release порядок и не служит native acceptance.
Unit IDs, source hashes и dispositions перечислены в private coverage/Z06.json;
other units/unreviewed disclosed. Knowledge principal ownership остаётся у C05/
платформ; эту карту не считать повторным общим backlog или назначением новых ролей.

Current source меняется параллельно. Source cuts initial10:38Z/extra10:46Z и
позднее10:53Z показывают:

- `pin_policy_import.go` перешёл от bare initial reads/held SDK journal к
  coherent source capture, unlocked SDK, final shared/writer gate;
- `delivery_state.go` добавил initial WithRead fence **даже для no-op** и
  отдельный `retainDeliveryProgressWithOutcome`;
- export prefix helper переиспользует outcome result вместо собственной
  callback bookkeeping; failure не теряет confirmed committed flag;
- import-source comment уточняет urgent denial без old envelope. Behaviour
  не выведено из одного комментария;
- owner STATE/PLAN на позднем cut уже обозначают **V2-183** и следующий
  public selected-file intake original-receipt срез. Android STATE/PLAN перешли
  к **AND-02-S5/ADR-062** callback value guards; AND-04-S4b production driver
  по-прежнему открыт. Эти lifecycle изменения не reviewed здесь как whole
  native-реализация. Предложения повторить V2-183 источник/outcome устарели.

Эти late source changes адресно прочитаны; остальной изменившийся theory corpus
не объявлен заново reviewed. late-corpus.json10:21Z содержит Z01 STATUS и Z08
SOURCES/STATUS, **не** новые trust principal C05 units. Наблюдались также поздние
изменения R05-009/C05/Z06/прочих research files вне исходного inventory. Их hashes
и список требуют руководительской reconciliation before итоговое corpus-complete;
данная crosscheck сохраняет frozen знания и не редактирует inventory.

Дальнейшие source/native/SDK gates выше остаются открытыми. Source drift после
финального cut требует узкой повторной applicability проверки, а не объявления
этого планирования временем-неограниченным product PASS.
