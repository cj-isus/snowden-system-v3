# Доверие, доставка обновлений и восстановление: применение теории

## 1. Состояние и порядок работ

Сначала исправить границу допустимого TUF progress и закончить покрытие original I/O/source/outcome границ. Затем согласовать переносимый verifier и подключить Android. Завершение установщика и восстановление после потери истории — самостоятельные результаты. Новые механизмы аттестации, transparency и нескольких репозиториев не являются зависимостями первого работоспособного клиента.

Основание — накопленный корпус R05-001…010, архитектурные дополнения, рецензии и R28 C05/Z06/Z07. Ниже различаются **существующий source**, **предлагаемое изменение**, **решение, требующее review**, и **будущая проверка NOT_RUN**. Научное ACCEPTED не означает успешную запись, пригодность SDK, работающий APK/EXE или защищённый сетевой путь. Полный побайтовый реестр корпуса и чтений находится в coverage C05 — `.artifacts/planning/knowledge-application/2026-10-28/coverage/C05.json`.

### Что уже есть

| Компонент | Подтверждённая исходниками граница | Открытый объём |
|---|---|---|
| Windows metadata/delivery | Роли profile/update/recovery; существующие envelope/epoch/pin/history checks; pinned go-tuf v2.4.2; private proof отдельно от progress; exact original bytes, supplemental continuity, path-aware online-key reset и полный batch перед artifact proof | Допустимость **нового** intermediate при ошибке; opaque SDK I/O; per-return/per-step durability; совместимость и OS приёмка |
| Windows storage, V2-170…183 | Original bounded I/O, общий native writer/read domain, fixed4 journal, prepared vault CAS, целостные ProfileSnapshot/TrustSnapshot; runtime writer меняет только допустимый TimeFloor. V2-180 export, V2-181 updater, V2-182 import/recovery и V2-183 urgent pin-policy уже используют новые границы | Public file intake receipts; полное покрытие readers/writers; old clients, interprocess, retirement, crash/power-loss и реальные outcomes |
| Windows updater | Signed manifest, точные payload hashes, version/purpose gates, application/helper pair, installed/recovery image leases, current-source recheck через callback установки | Public schema2 **не поддержана для выпуска**, пока не пройдены SDK/native/compatibility/durability/GUI/OS gates. Initial install без pair receipt не даёт release-integrity proof |
| Android profile | EncryptedProfileStore: CE/no-backup, AES-GCM Keystore, CAS, uncertain outcome; ProfileClock: local wall/elapsed continuity; ProfileImportCoordinator: preview/apply/cancel, progress до application commit, свежий exclusive deadline | Production ProfileTrustDriver, accepted-only runtime source/renderer и app composition отсутствуют; контракт T01…14 requested, не принят |
| Общие модули | `shared/adaptive` существует; рекомендации оставляют admission платформе. В просмотренном registry четыре роли: harness/windows/android/adaptive | `shared/trust` и зарегистрированного trust owner нет. Сначала определить ответственного и принять границы; отсутствие роли не разрешает автоматически создавать пятый слой/владельца |

Кодовые основания: TrustSnapshot — `windows/backend/metadata/trust_snapshot.go`, ProfileSnapshot — `windows/backend/metadata/profile_snapshot.go`, runtime checkpoint — `windows/backend/metadata/runtime_checkpoint.go`, updater source gate — `windows/update_delivery.go`, import source V2-182 — `windows/onboarding_import_source.go`, [vault transaction](../../../../windows/backend/secretvault/identity.go), Android coordinator — `android/profile/src/main/kotlin/org/snowden/android/profile/ProfileImportCoordinator.kt`, composition boundary — `android/app/src/main/kotlin/org/snowden/android/app/SnowdenApplication.kt`.

Исходники читались последовательными срезами 28.10.2026, не атомарным snapshot репозитория. Частные manifests сохраняют SHA256: initial source cut — `.artifacts/planning/knowledge-application/2026-10-28/captures/C05/current-source-cut.json`, дополнительный срез — `.artifacts/planning/knowledge-application/2026-10-28/captures/C05/current-source-extra.json`, финальный drift review — `.artifacts/planning/knowledge-application/2026-10-28/captures/C05/final-review.json`. При последующих изменениях действуют новые исходники, а не номер V2 в этой таблице.

Поздняя [R28 REVIEW-01](../../../theory/research/cycles/2026-10-28-contracts/REVIEW-01.md) учтена отдельно от frozen корпуса: **REV-03** принимает только lossy SUSPENDED projection выбранного Google candidate; **REV-04** — operational binding/current admission как уточнение уже сильного исходного контракта. Они не принимают весь parser/provider, остальные R28 claims или интегрированный VPN. R05-010-D и общий ResetAllowed остаются review gates. [Общие ограничения применения](../../../theory/research/cycles/2026-10-28-contracts/APPLICABILITY-LIMITS.md) сохраняют native/device/live и source-to-artifact границы.

<a id="families"></a>
## 2. Знания → существующие компоненты → решение

<a id="k1"></a>
### K1. Допустимый predecessor отдельно от application acceptance

**Основание:** [R05 REPORT §§3–5,9](../../../theory/research/05-TRUST-SAFETY/REPORT.md), [независимая рецензия A/B](../../../theory/research/reviews/2026-10-27-R05-001-PROGRESS.md), [R05-010 §§2,7](../../../theory/research/05-TRUST-SAFETY/REPORT-010.md), [рецензия timestamp trace](../../../theory/research/reviews/NINE-2026-10-27-R05-010-A.md).

Различать получение bytes, intermediate SDK state, **допустимый progress P**, принятие приложения A, сохранение D и текущий platform admission L. Ошибка позднего шага не отменяет ранее принятый P. Указатель `TrustedMetadata.Timestamp/Snapshot` сам не доказывает завершение workflow: pinned lower layer присваивает объект до последней проверки. Ранее принятый, теперь expired B сохраняет rollback history; новый rejected expired C не становится таким B. Root имеет отдельный порядок последовательного dual-quorum/N+1 принятия до проверки срока конечного root.

**Код:** retainedState — `windows/backend/delivery/retained_state.go`, boundMetadata/retainedRoot — `windows/backend/delivery/retained_inputs.go`, PrepareArtifacts — `windows/backend/delivery/client.go`, retainDeliveryProgress — `windows/delivery_state.go`. В updater retention вызывается до `deliveryErr`; publisher verifyRelease — `windows/tools/internal/repository/promotion_verify.go` тоже принимает `NextState` до проверки tail error. Android coordinator уже записывает разрешённый existing-root progress перед application outcome.

**Решение — CHANGE REQUIRED, P0:** восстановить per-role eligibility принятого progress. Для нового timestamp текущие exact-byte/continuity gates не доказывают final expiry eligibility; это подтверждённый условный source trace, **не доказанная durable запись/lockout или bypass обновления**. Старое описание «retention читает только workspace cache» устарело после V2-169.

Кандидат R05-010-D/R28-C05: при сохранённых source/root/reset/cancel guards G разрешать timestamp по `G ∧ (H ∨ F)`: H — exact ранее принятый predecessor в применимом context; F — достаточное прохождение final timestamp/root freshness относительно **того же неизменного SDK RefTime**. Строгое `Expires.After(RefTime)` — достаточный консервативный вариант, не публичный upstream receipt. Сам D ещё требует принятия scoped contract. `now` приложения не подставлять вместо RefTime. Для snapshot дополнительно важны timestamp link/version, length/hash и отдельный финальный шаг: timestamp-критерий автоматически на него не переносится.

**Противовес:** boundMetadata уже отвергает неоднозначные originals, supplemental floors сохраняются даже при skipped seed load, completeRetainedState/finalPolicy и `deliveryErr` запрещают artifact proof. Сохранять эти проверки. Не применять патч «любой error → выбросить весь progress» или «любой signed pointer → сохранить». Связи плана: **TH-03, FR-008/011, OFF-04/06, WIN-SDK-001-T, AND-04/T05–07,T11**.

<a id="k2"></a>
### K2. Reset разрешённой истории, старые подписи и recovery — разные условия

**Основание:** [R05-002](../../../theory/research/05-TRUST-SAFETY/REPORT.md), [continuity supplement](../../../theory/architecture/TRUST-CONTINUITY-AND-AGREEMENT.md), [R28-Z07 §§3–5](../../../theory/research/cycles/2026-10-28-contracts/Z07/REPORT.md).

Отдельно определить: (1) ResetAllowed по authenticated root lineage/принятой policy; (2) достаточен ли quorum **конкретного старого envelope** под новым root; (3) исключён ли достаточный compromised new quorum; (4) завершён ли platform barrier. TUF §5.3.11 сбрасывает оба T/S cache при рассматриваемой online-role rotation, но не application epochs, bans, clock, enrollment или custody. Полная нормативная функция для partial addition/removal/threshold-only/reuse не установлена корпусом; current material-set diff — adopter policy, не универсальная цитата спецификации.

Порядок существенен: normative reset следует после final-root freshness, а допустимый root prefix может сохраняться раньше, включая expired intermediate. Current adapter способен сформировать root+empty-cache cut при tail error; это требует отдельного принятия restrictive/continuation semantics и не объявляется completed §5.3.11 только по наличию нового root. Carry, invalidate и последующее повторное принятие cache должны соответствовать одной выбранной recovery policy.

**REUSE:** current `retainedRoot` учитывает каждый intermediate link, поэтому S→S′→S в одном вызове уже не теряется. Threshold-only diff не даёт reset, а `timestampContinuity` сохраняет floor независимо от загрузки старого cache в SDK. Exact old-byte continuation защищает первый проход. Равноверсионное переформатирование само не обходит его: SDK equal-version branch сохраняет старый pointer.

**ADAPT / REVIEW:** после подтверждённого reset свежий проход вправе заново принять ещё-valid old timestamp, если его подписи удовлетворяют новым authority. Atomic deletion не исключает h_post replay. При old-only signatures достаточное условие исключения — `|Kold∩Knew| < qnew`; при dual signatures/reused material его предпосылки нарушаются. После compromise отдельно исключить `|Cpriv∩Knew|≥qnew`. Число удалённых keys относительно старого threshold — неподходящая замена.

Минимальный первый supported recovery recipe: явная парная замена T/S с исключением старого достаточного quorum, свежие новые metadata и сохранение application history. Существующие publisher draft — `windows/tools/internal/repository/draft.go`/custody — `windows/tools/internal/repository/custody.go` уже умеют задавать role keys и проверять parent approval/N+1; recipe/ограничения добавлять в этот pipeline, не второй publisher. Python7.0.1 — reference recipe, не новая runtime dependency. Partial rotations оставить unsupported/review-required до принятия точной semantics.

Если existing atomic root+empty-T/S image и reader/admission protocol исключают h_pre, новое поле не требуется. Если обещана continuation с carried pre-reset cache через expired root, нужно сохранить basis либо сузить обещание. Это информационное обязательство, не автоматическое требование пятого файла/БД. **TH-03/COM-14, OFF-06, AND-04/T05–07; S1/S5 ниже.**

<a id="k3"></a>
### K3. Один текущий источник при async admission

**Основание:** [R05-009 §§9–10](../../../theory/research/05-TRUST-SAFETY/REPORT-009.md), [R28-Z06 §§4–8](../../../theory/research/cycles/2026-10-28-contracts/Z06/REPORT.md), [R28-C05 §§3,5](../../../theory/research/cycles/2026-10-28-contracts/C05/REPORT.md).

Даже пять корректных результатов — acquisition/evidence/status/freshness/decision — недостаточны без их общей привязки. Late callback по E0 не должен разрешить действие после принятого E1 deny. При неизменной revision разрешение также может истечь в очереди. Read-only interface/`val` не означает глубокую неизменяемость Set/bytes. Failed refresh не стирает принятый deny и не делает старый success свежим; latest attempt и выбранное ещё-допустимое evidence сохраняются отдельно.

**Уже SOURCE IMPLEMENTED, расширять покрытие:** V2-179/181/182/183 snapshots/final source fences, prepared vault CAS и Android commit/cancel/current-clock gate. Не создавать отдельный WitnessController, второй revocation store или ещё один scheduler. После долгой verification и staging проверять current head, exact source, purpose/intent, применимый clock/deadline; необратимый внешний effect не помещать в retriable CAS callback. После admission pending effect учитывает platform policy, а не обещание мгновенного глобального revoke.

**Проверенный late-source delta:** прежние отдельные initial loads pin_policy_import — `windows/pin_policy_import.go` заменены в **V2-183** на `captureProfileImportSource`, scoped Prepare/Apply/preview и final in-scope fence. SDK вынесен из profile scopes; общий retention helper проверяет coherent source даже на no-op. `retainDeliveryProgressWithOutcome` отличает actual successful commit callback от cache digest/no-op и сохраняет его при поздней ошибке. Это source implementation, не native durability proof; повторять этот patch не нужно.

**Следующий открытый участок:** [onboarding_dialogs](../../../../windows/onboarding_dialogs.go), `selectPinPolicyFile/readOnboardingTransport`: bounded regular-file/size/format проверки уже есть, но используются bare os.Open/defer Close. Выбрать existing original bounded read seam с checked retirement и receipt для public input; не переносить private trusted-store ACL/path policy на пользовательский выбранный файл. Проверка подписи/approval после чтения остаётся обязательной. Затем закончить реестр readers/writers и source→final-effect closure. **OFF-04/06, TH-03, WIN-SDK-001-T, AND-04/T03,T11–13, AND-07/M01–06.**

<a id="k4"></a>
### K4. Время, status и граница offline знания

**Основание:** [R05-005](../../../theory/research/05-TRUST-SAFETY/R05-005.md), [R05-010 §3–4](../../../theory/research/05-TRUST-SAFETY/REPORT-010.md), [release boundaries](../../../theory/architecture/RELEASE-EVIDENCE-BOUNDARIES.md).

Scope включает subject/issuer/root/purpose и status profile. `NotListedAsOf` не означает globally latest NotRevoked. OCSP good не доказывает все свойства сертификата; historical CRL может не содержать уже expired revoked certificate. Не строить собственный CRL merger, общий max serial/version или вечный union SUSPENDED. Base OCSP и RFC5019 имеют разные требования к nextUpdate. Capture/receivedAt, thisUpdate/producedAt/nextUpdate, TSA genTime и evaluation time различаются. Explicit Windows pTime не замораживает текущие root/status stores.

**REUSE:** Windows ObservedClock — `windows/backend/metadata/clock.go` и Android ProfileClock — `android/profile/src/main/kotlin/org/snowden/android/profile/ProfileClock.kt` уже отвергают discontinuity/overflow/stale observations и не объявляют local floor доверенным UTC. Floor — нижняя граница истории, не верхняя граница реального времени. После reboot прежний elapsed receipt не восстанавливать.

**ADAPT contract:** до `min(deadlines)` определить purpose: TUF delivery, принятие профиля, разрешение канала, уже принятая running session, установка code и recovery имеют разные authority. Не переносить expiry всей delivery-chain автоматически на уже принятую data plane; failed refresh сам по себе её не отзывает. При этом уже выданные Android authority deadlines не удлинять новым толкованием: T10/ADR-015 пересматриваются только отдельной совместимой версией. Preparation TTL не signed lifetime; reconnect не renewal. **TH-03, FR-008/011, AND-04/T09–10, AND-10/11, COM-08/14.**

<a id="k5"></a>
### K5. Custody и восстановление утраченной истории

**Основание:** полезная часть [R05-008](../../../theory/research/05-TRUST-SAFETY/R05-008.md) с исправлениями [R05-009 §§5–6](../../../theory/research/05-TRUST-SAFETY/REPORT-009.md) и [R05-010 §§3–4](../../../theory/research/05-TRUST-SAFETY/REPORT-010.md).

DPAPI/Keystore/AEAD защищают custody/целостность в своём scope; не назначают issuer, freshness, antirollback после restore или hardware durability. Потеря key/history не превращается в fresh enrollment. **REJECT как безусловные требования R05-008:** всегда hardware-backed Keystore; BiometricPrompt как подпись показанного смысла; безусловная невосстановимость DPAPI после любого account/password reset; обязательные fresh consent и server-signed floor как единственный recovery.

**REUSE existing:** Android store — `android/profile/src/main/kotlin/org/snowden/android/profile/EncryptedProfileStore.kt` различает Empty, key missing, alias-only incomplete enrollment, pending/uncertain transaction; Loaded остаётся unverified. ProfileStoreCrypto — `android/profile/src/main/kotlin/org/snowden/android/profile/ProfileStoreCrypto.kt` сознательно использует `authRequired=false` для unattended recovery. Не создавать новый alias на ошибке чтения, не удалять stage/историю по timeout. Отсутствие StrongBox request не доказывает software key; AES security level здесь UNKNOWN.

**ADAPT existing recovery:** Windows ProfileRecoveryPermit — `windows/backend/metadata/profile_recovery.go` уже связывает exact source/target/role/validity и не разрешает снизить floor, восстановить retired pins или enroll update/root. Он восстанавливает envelope при целом checkpoint, не всю потерянную историю. Для расширенного restore нужен переживший отказ независимо доверенный anchor принятой истории: существующая custodian backup procedure, отдельная сохранённая запись, подтверждённый offline канал или выбранный hardware/server anchor. **Новый always-online floor server не обязателен.** Старый signed floor плюс свежее согласие не доказывают latest revocation. При недостаточном основании — явный recovery/blocked с границей as-of. **TH-03/07, COM-13, OFF-04/06, AND-04/13.**

<a id="k6"></a>
### K6. Проверка key attestation и осмысленное согласие — опциональный продукт

**Основание:** [R05-009 §§2–4,9–10](../../../theory/research/05-TRUST-SAFETY/REPORT-009.md), [Z06](../../../theory/research/cycles/2026-10-28-contracts/Z06/REPORT.md).

Key attestation связывает key/configuration/challenge при выбранной chain policy. Она не доказывает possession предъявителем, fresh user authentication, смысл действия или фактический commit. Per-operation auth/HAT, trusted physical presence и Protected Confirmation — разные свойства. В confirmation подписывается exact CBOR с promptText/extraData; пользователь видел promptText, не скрытый extraData. Независимый attestable EC key не аттестует существующий AES checkpoint key.

**DEFER интеграцию** до конкретной потребности в ручном критическом действии. На текущем Android нет production consumer Google verifier/status checker; candidate findings не называются уязвимостью нашего VPN. Если фича выбрана: обязательный request challenge, replay/current-authority/purpose gate, exact prompt binding, supported device/a11y path и отдельная семантика отказа. Никакой biometric/confirmation prompt на каждом reconnect.

**Условный ADAPT готового:** Android keyattestation commit `34652d3df68b8d6887a6670b00eb0856090dae6d` — Apache-2.0, pre-v1 source candidate, не стабильный установленный артефакт. Optional challenge/app-id defaults и generic Success уже требуют explicit consumer constraints; existing chain anti-extension guards сохраняются. Google status projection отбрасывает SUSPENDED; nullable/unknown/duplicate JSON нельзя схлопывать в normal/emptySet. Использовать non-lossy typed status до Set adapter. Предпочесть узкий bounded adapter над выбранным parser; второй JSON stack/schema framework не обязателен. Candidate JDK21/Kotlin2.2 не drop-in в app Java17. **AND-04/13/14 — отдельное owner решение; core SDK не зависит от этой фичи.**

<a id="k7"></a>
### K7. Durability и исход операции — обязанности платформы

**Основание:** [R05 REPORT §9](../../../theory/research/05-TRUST-SAFETY/REPORT.md), [Z07 §5](../../../theory/research/cycles/2026-10-28-contracts/Z07/REPORT.md), [Safety supplement](../../../theory/architecture/SAFETY-AND-TRUST.md).

Pure evaluate, результат commit и разрешение использовать результат — разные границы. Single pure return не обещает durable-before-next-root-step при hard kill. Если выбран per-step guarantee, нужен explicit external barrier/continuation/storage port; если per-return — честно задать crash contract. Уточнить это до SDK orchestration, не после Android JNI.

**REUSE existing storage:** [fixed4 Store](../../../../windows/backend/filetxn/filetxn.go), bounded original readers/writers, `WithRead/WithWriter`, `WithPreparedTransaction`; Android encrypted checkpoint и coordinator. New fields/DB не следуют из information-limit argument. bbolt/SQLite — reference semantics, не основание миграции нынешних двух небольших stores.

Неопределённый commit может иметь effect: удержать исходного владельца/barrier до reconciliation, не повторять как no-effect и не объявлять все pointers потерянными по одному error. Completed commit и ошибка позднего retirement/cleanup сохраняют разные смыслы. Crash до записи сам по себе не доказывает TUF violation: анализ включает следующий admission. Cooperative lock не защищает от old writer, не соблюдающего protocol, trusted-owner malware/admin/kernel; scope поддержки фиксируется явно. **OFF-04/06, FR-011, WIN-SDK-001-T, AND-04/11.**

<a id="k8"></a>
### K8. Release bytes → установленный объект → исполняемый образ → readiness

**Основание:** [R05-004](../../../theory/research/05-TRUST-SAFETY/R05-004.md), [release supplement](../../../theory/architecture/RELEASE-EVIDENCE-BOUNDARIES.md), [R05 REPORT §10](../../../theory/research/05-TRUST-SAFETY/REPORT.md).

Подпись TUF/application manifest, exact final payload, OS channel authority, custody, installed binding, executing incarnation, current intent и readiness не взаимозаменяемы. Flat PE digest считать после финальной подписи/timestamp: Authenticode SIP исключает некоторые bytes. WinVerifyTrust успешен только при LONG==0; CertVerifyCertificateChainPolicy требует и успешного BOOL, и dwError==0. Cache-only/revocation flags и evaluation context задать явно **если выбран такой канал проверки**. FileID/path/MEM_IMAGE не означают immutable approved code; DLL initialization может начаться до возврата LoadLibrary.

**SOURCE REUSE, завершить gate:** Windows pair/native/recovery leases уже существуют — pair payload — `windows/backend/update/pair.go`, native image — `windows/backend/core/native_image_windows.go`, accepted startup — `windows/update_accepted_image.go`. Не писать installer заново. Их scope ограничен выбранными файлами/FS/ACL/architecture; не обещать whole-process attestation всех DLL/плагинов или защиту от trusted-owner/admin/kernel. Самопроверка кандидата не предотвращает его первое исполнение из другого разрешённого OS channel; first-install/channel policy должна иметь независимую границу.

**Android ALREADY PLANNED, AND-14:** final APK/split set/package/versionCode/current signer или допустимая lineage — не AAB/upload certificate и не installer-reported checksum. OS PackageInstaller уже проверяет package signatures; custom ZIP/ASN.1 verifier в приложении не нужен. Выбрать Store или local install contract, exact write→seal custody, reconcile lost callback/pending session, installed identity и отдельно running process incarnation; API34 inherited split/dontKillApp учитывать в support matrix. Host-side apksig — reference/tool candidate, не автоматическая client dependency.

Recovery code должен быть текущим разрешённым forward fix либо blocked; rollback binary не откатывает trust floor/accepted deny. Обновление не должно выдавать Ready до platform admission/egress proof. **OFF-06/07, TH-01/03/08, COM-14, AND-01/04/14/15.**

<a id="k9"></a>
### K9. Provenance/transparency — защита от выбранной угрозы

**Основание:** [R05 REPORT §10 и SOURCES R05-003](../../../theory/research/05-TRUST-SAFETY/REPORT.md), [handoff R05-003](../../../theory/research/05-TRUST-SAFETY/handoffs/R05-003.md).

TUF уже защищает от подмены mirror в заявленном scope. SLSA/provenance добавляет проверяемые build expectations, но не назначает artifact authority и не заменяет install/adoption. Identity/issuer/builder/repository/ref/workflow/subject digest должны приходить из независимо принятой policy. V1 SET promise не inclusion, inclusion не consistency/latest; уникальные log key IDs не доказывают независимых операторов.

**DEFER обязательный client verifier; OPTIONAL issuer-first reuse:** sigstore-go v1.3.0 `22d3691c7b8e0c5530fae3c05577690bfef5cd00`, Apache-2.0/Go≥1.25.8, SLSA1.2 как reference. Проверка на стороне publisher дешевле по клиентским dependencies/roots/status/IO, но не независимая защита от того же скомпрометированного issuer. Если нужна именно такая защита, принимать отдельную независимую policy/approval boundary, без обещания её из второй подписи того же владельца. **COM-14, OFF-06/07, AND-14.**

<a id="k10"></a>
### K10. Несколько репозиториев и governance

**Основание:** [R05-006](../../../theory/research/05-TRUST-SAFETY/R05-006.md), [R05-007](../../../theory/research/05-TRUST-SAFETY/R05-007.md), [continuity supplement §§4–7](../../../theory/architecture/TRUST-CONTINUITY-AND-AGREEMENT.md).

**DEFER deployment:** pinned go-tuf `metadata/multirepo`/TAP4 — готовая conditional reuse точка, но сейчас независимые custodians/operational policy не выбраны. Число VPS, стран, keys или repo labels не число независимых authority. Priority/fallback mapping не AND; mapping policy не авторизует себя. Rename/remove/readd сохраняют stable lineage/history; cache alias не новая root lineage. Старый TAP4-ignorant client может игнорировать map: minimum-version поле не заставляет его выполнять новую policy.

Готовые unique-repository/conflict/threshold guards уже существуют. При этом `Hashes.Equal` common-overlap compatibility не транзитивна: sha256-only A, sha256+sha512 B, sha512-only C; representative order может менять quorum. Download проверяет все hashes выбранного descriptor, но не автоматически совокупность constraints всех counted approvers. `custom` не входит в agreement. Если критические факты можно поместить в один exact signed manifest, это дешевле общего consensus layer. Нельзя заявлять произвольный payload bypass или требовать ещё одну БД из этого примера. **TH-03/07, COM-13/14, AND-04/13/14; отдельное согласование до rollout.**

<a id="k11"></a>
### K11. Граница доверия с safety, адаптацией и UX

**Основание:** [SAFETY-AND-TRUST §§3,7,13,15](../../../theory/architecture/SAFETY-AND-TRUST.md), [ранняя threat model](../../../theory/research/05-TRUST-SAFETY/REPORT.md), [историческая UX-матрица](../../../theory/ux/COMPETITOR-FEATURES.md).

Hard trust/protection admissibility предшествует rank, retry и статистике. Unknown clock/store/producer не тратится как разрешение рискованной попытки. Probe fail ≠ подтверждённая leak ≠ недоступность транспорта; порт/HTTP200/DNS success не Ready и не доказательство DPI. Dynamic WFP cleanup не persistent crash barrier; уже существующие TCP/one-way UDP flows требуют platform-native проверки. Эти знания продолжают **FR-009/010, TH-01/02/04, ADP-002…006, AND-02/03/07/11**, не новую trust реализацию WFP/сcheduler.

Сохранять one lifecycle owner, exact SecretID, pin/hostname/EKU validation, explicit approved temporary direct grants вместо общего fallback, подписанную policy и provenance failure domains. Поля транспорта должны пройти schema→render→core→server совместимость; неизвестные зависимости не независимость. Старые «WS значит Cloudflare», обязательные фиксированные probe endpoints/quorum, таймеры и универсальные mux presets не становятся требованиями из архива. V2-113 уже убрал carrier inference, текущие measurement/compatibility decisions принадлежат своим owners.

**REJECT как доказательства:** заявления старой UX-матрицы «у всех конкурентов только socket», «аналогов нет», «подтверждённо сильнее рынка» и универсальная актуальность её transport claims. В этом этапе новых рыночных проверок нет. Сохранить полезные сценарии, уже отражённые **COM-01…19/ANDROID-PLAN**: безопасный onboarding/transfer, offline error action, срок/as-of, revoked/unknown/committed/pending/Ready, тихая работа и понятный recovery. Не делать raw экспорт эквивалентным нашим гарантиям; не обещать server revoke по локальному удалению профиля. Проектирование страниц отдельно от trust SDK.

<a id="stages"></a>
## 3. Поэтапная разработка

Обозначения S0…S7 — части этого предложения, **не новые статусы PLAN**. Ответственные принимают изменения в существующих FR/TH/OFF/AND/COM пунктах. Фактическую native/live приёмку разрешает владелец отдельно.

| Этап | Результат и конкретные изменения | Ответственный / зависимости | Проверяемый выход, сейчас NOT_RUN |
|---|---|---|---|
| **S0. Зафиксировать contract** | Свести WIN-SDK-001-T и T01…14/M01…06 в одну versioned semantics matrix: progress eligibility, source identity, purpose/time, исход commit, raw byte ownership, per-return/per-step guarantee. Разрешить спорные K1-D/K2 branches review; выбрать поддержанные recovery recipes | Windows + Android; общий verifier maintainer назначается существующим owner через FIVE-STREAMS/registry procedure, если extraction выбран. No self-registration | Принятый API/version/pins/licence/extraction map, список consumers и unsupported branches; никакой зависимости от «готового trust SDK», которого нет |
| **S1. Исправить допускаемый progress** — P0 | Узко изменить retained_state/inputs/policy seam, чтобы новый offending timestamp не становился accepted predecessor; сохранять старый B и законный новый P при позднем error. Отдельно описать snapshot/root/targets gates. Уточнить T07/Driver KDoc, не обещать retention любой mutation. Проверить shared caller chain updater/import/pin/publisher | Windows current delivery owner; scoped normative contract S0. Перенос portable части — после S3, не блокирует локальный P0 | Таблица exact incoming/prior receipt → eligible progress/proof/error; input parity и future cases A1–A3. Нового artifact proof при error нет |
| **S2. Закончить I/O/source closure** — P0 | V2-182 import/recovery и V2-183 urgent pin-policy/actual prefix outcome **уже source implemented**. Следующий patch — public file intake checked Close/original receipt в onboarding_dialogs на existing seam с отдельной public-input policy. Инвентаризировать всех реальных readers/writers journal/vault и отметить старые неподдержанные tools/clients. Не держать native source locks через SDK/App/lifecycle | Windows, OFF-04/06/TH-03; existing V2-170…183. Независим от общего SDK, Android и оплаченного VPS | Capture/prepare/final fence/commit outcome одной source incarnation; CAS/freshness до effect; uncertainty сохраняет original owner. Cache-only commit/no-op/late failure различимы. Native guarantees остаются отдельными A4–A6 |
| **S3. Переносимый verifier** | Извлечь existing canonical/role/epoch/pin/acceptance/delivery policy по карте реальных consumers, сохранив upstream crypto. Удаление старой реализации — после parity. Выбрать поддерживаемый bounded full-updater storage/clock seam либо отдельно reviewed **полный** lower workflow; не config-only diskless | Назначенный maintainer; S0 и семантика S1. Windows/Android сами адаптируют callers/manifest/packaging в своей зоне | Pure bounded API без ambient FS/network/clock/logger, immutable/owned outputs, typed errors, private proof, original-byte/state relationship. Если нет допустимого seam, SDK остаётся blocker, не signature-only replacement |
| **S4. Реальный Android consumer** | Реализовать ProfileTrustDriver inspect/verifyForApply и accepted-only renderer/provider в существующем profile/corebridge/whole libbox. Подключить coordinator/app composition, повторную проверку каждого decrypted/recovered checkpoint. M01…06 извлекать из того же accepted source, если формат их поддерживает; отсутствие отличать от empty/default endpoints | Android, AND-04/07/11; принятый S3 API, whole-AAR ABI/capabilities. Платформенные CE/Keystore/clock/CAS остаются здесь | Не fake Import/Connect; один Go runtime/AAR, intact input/owned output lifetime, uint64 без signed truncation, denial/recovery/zero-channel semantics. A7, затем device gates |
| **S5. Поставка и восстановление code** | Завершить существующий Windows pair pipeline/native admission/old-client/initial install contract; Android AND-14 — конкретный канал и final APK/split/signing lineage matrix. Проверять точные bytes после signing, current purpose до install/recovery/adoption; сохранять trust floor при forward repair | Windows OFF-06/07, Android AND-14; каждый свой release path. Не требует optional attestation/multirepo | A8–A9 по native/device, no readiness из install callback. SLSA/SBOM issuer-side — отдельный optional подпункт K9 |
| **S6. Restore и повседневное управление** | Развить existing issuer/transfer/backup/recovery: independently approved root+policy baseline в одном понятном review record, recipient/purpose/as-of, device credentials и revoke, неполный backup и lost-history outcomes. Не помещать admin/signing keys на устройство | Windows/Android + infra owner в COM-13/TH-03/07/AND-13; S3 и выбранная surviving-history basis. Реальный server provisioning отдельно | A10: old valid backup не выдаётся за latest; lost key/history blocked/recovery; imported profile не запускает произвольные команды. Автономный valid cache остаётся возможным |
| **S7. Только выбранные усиления** | K6 manual attestation/confirmation; K9 independent provenance; K10 independently governed repositories. Для каждого — отдельный threat/use case, cheapest sufficient mechanism и эксплуатационный владелец | Соответствующий owner после S0…S6 и доказанной необходимости | A11–A12, отдельные лицензии/dependency/runtime/resource gates. До этого deferred, не скрытая зависимость выпуска |

Практический первый пакет разработчику: **S1 + S2 как отдельные изменения**, затем принятие S0/S3. Их можно ревьюить независимо. Android интеграция ждёт контракт, а Windows custody/lifecycle P0 продолжается без ожидания. S5 Windows и Android выполняются независимо на своих платформах; S6 не ждёт S7.

<a id="contracts"></a>
## 4. Минимальные контракты между компонентами

Ни один пункт сам по себе не требует нового persisted поля или нового public DTO. Существующие private objects/atomic images допустимы, если обеспечивают обязательство.

| Граница | Вход / обязательство | Выход / запрещённое расширение |
|---|---|---|
| Review/approval | Exact original container и authority, named mode/purpose, independently approved anchor/target, source enrollment/history incarnation | Preview с полным fence; embedded digest/UUID/parse не capability. Применение заново проверяет approval и source |
| Verification | Bounded immutable borrowed bytes до synchronous return; owned previous accepted state; явное time observation/clock domain; cancel semantics | Typed accepted/rejected + **отдельный eligible P**, exact owned originals, роли/версии/сроки/причина. No ambient IO/OS side effect; error не empty; panic не частичный success |
| Progress | Source identity и per-role completed acceptance либо exact applicable prior receipt; root/reset basis; original byte relationship | Restriction/history progress не разрешает profile/render/install/Ready. Initial enrollment не сохраняется как prefix-only. В Android existing P→A relationship/generation +2/+1/initial1 проверяет Driver, CAS лишь внешний слой |
| Commit | Ожидаемый coherent source; текущая authority/intent/time; staged same-source vault/records; один original writer | Completed, rejected/no-effect либо uncertain с retained owner/recovery barrier; completed + cleanup fault различимы. Application rollback не стирает отдельно подтверждённый P |
| Runtime/measurement source | Тот же accepted head, original graph/SecretID/pins, applicable deadlines и поддержанная measurement policy | Только accepted-only config/permit; service/family/native-outbound/remote-egress/user-work scope различимы; без default target/direct fallback |
| Platform admission | Fresh current source/purpose/intent + actual worker/network/apps/OS protection/clock/resource state после ожиданий | Платформа одна выполняет переход и подтверждает readiness; portable proof/score/HTTP200/storage checksum не дают это право |
| Recovery | Сохранившаяся разрешённая history/authority basis, exact transition/recipient/as-of и ясный loss scope | Exact восстановление в выбранном scope либо blocked; fresh consent не отменяет signed roles/floors/deny, retired material не оживает из старого backup |
| Release | Final payload set + manifest/roles/version, выбранная OS channel/signing policy и custody через staging/seal/launch | Installed/executing/adopted facts разделены. Не принимать checksum самого installer или self-check кандидата как независимое first-execution approval |

Размеры Android input256KiB/public256KiB/trust6MiB/application2MiB/credentials256KiB/plaintext9MiB и existing Windows metadata/package caps — начальные source bounds, **не измеренные peak RSS/latency SLO**. До allocations учитывать JVM/native/Go/encoding copies, cert/root path, history/count/depth/work/cancel/cleanup. При исчерпании bounds — typed limit/blocked, не silent history eviction или снижение quorum. JNI cancellation не освобождает borrowed buffers до actual return; внешний timeout не подтверждает прекращение native I/O.

<a id="reuse"></a>
## 5. Совместимость, готовые компоненты и цена

| Компонент / pin из сохранённых источников | Решение | Совместимость, licence и цена |
|---|---|---|
| go-tuf2.4.2 `f5edbde31e5507f46db2069402dc38903fe6d9d4` | REUSE crypto/workflow, ADAPT только описанные seams | Apache-2.0; Go≥1.25, Windows go.mod1.26. Никакого массового upgrade на HEAD. Module resolution/transitive closure/whole Android graph ещё не приняты |
| Existing Windows metadata/delivery/filetxn/vault/update | REUSE/EXTRACT по S1–S3 | Извлекать pure часть, не импортировать Windows module в Android. Нет установленной общей root LICENSE для нового module: maintainer фиксирует rights/notices/combined distribution до публикации; Android source GPL-3.0-or-later |
| Python-tuf7.0.1 `49eb1da48c58cf0cbb0f2e490f7ad39fbd8c1f15` | REFERENCE paired recipe/result API | Apache-2.0 OR MIT, Python≥3.10. Не добавлять Python runtime; прочитанные upstream tests не исполнены у нас |
| Legacy go-tuf0.7.0 `6ad7fe593e4042db3544c4b0fedbe66bac371c42` | REFERENCE, не port reset predicate | BSD-3-Clause; старый draft/role table не современный контракт |
| Google keyattestation `34652d…`, Gson2.11.0 | DEFER feature; условный narrow adapter | Apache-2.0 по saved sources. Candidate не app-Java17 drop-in; default parsing/Success нельзя считать полной policy. Не пересобирать AES custody ради attestation |
| Jackson-core2.18.3 `cc34e8d5b75d4c911c16f2bc87d6e13e42eafddd`, NetworkNT1.5.9 `270df62471e6a0c564dc13ae6cf2bf4116916f82` | DEFER второй parser/schema stack | Apache-2.0. Jackson duplicate detection не default, document/token caps нужно включать; NetworkNT AdditionalProperties имеет `#` exception, ordinary schema loader требует explicit deny network fallback. Их наличие не универсальная JSON-schema assurance и не причина включать две зависимости |
| sigstore-go1.3.0 `22d3691…`, SLSA1.2 | OPTIONAL issuer-side, client deferred | Apache-2.0, Go≥1.25.8. Цена roots/status/expectations/log IO и offline bundle; не считать её нулевой. Нужна независимая policy для независимой угрозы |
| apksig android-16.0.0_r2 `61f88cbe6f81b4536d1ef124daf3caa4ad014afd` | REFERENCE/host-side candidate | Apache-2.0; host tool и whole client compatibility различаются. Android17/v3.2 не приписывать этому pin; platform signer/history API привязывать к поддержанному OS range |
| go-tuf metadata/multirepo + TAP4 `6096e30ca04eb4a79e1d23a2aa9ac5ad64a6d63b` | DEFER deployment | Код Apache-2.0, TAP Public Domain; уже тот же Go pin. Операционная независимость custodians, mapping migration и offline availability дороже добавления import |
| bbolt1.5.0 / SQLite | REFERENCE, REJECT migration без storage gap | MIT / Public Domain по сохранённому корпусу. Borrowed mmap bytes/locks/Sync/WAL NORMAL-vs-FULL semantics не переносят OS guarantees; plaintext secrets в новую БД недопустимы |
| MSIX / Uptane-class architecture | DEFER без выбранного канала/угрозы | Новый distribution/identity/OS/операционный scope. Не замена завершению существующего pair updater |

**Compatibility gate:** legacy trust schema1 остаётся profile-only, не code authority. Schema2/новые optional critical fields старые strict decoders могут отвергать; старый permissive/TAP4-ignorant client способен игнорировать новую policy. Capability/min-version в новом manifest не действует на уже старый binary. Нужен реальный supported-client/channel rollout и отказ от неподдержанной ветки, без silent downgrade/fresh enrollment/fallback. Сохранять exact original signed bytes и uint64 epochs/counters: Kotlin Long/Double не сужают пространство. Native bridge/AAR/protocol версии и marker/source hashes не доказывают наличие работающего API.

Цена S1/S2 — bounded comparisons/копии, дополнительные source/clock gates и иногда честный retry/recovery; это может снизить доступность на неопределённых входах, сохраняя смысл защиты. Цена S3/S4 — extraction/parity и combined toolchain/FFI. CPU/RAM/IO/battery, длительность и стоимость эксплуатации **не измерены**; чисел выгоды и нулей вместо missing data нет.

<a id="acceptance"></a>
## 6. Конечная будущая приёмка — всё NOT_RUN

Здесь перечислены требования к проверке после отдельного разрешения. Это не созданные fixtures и не результаты запуска. Static source review данного документа не закрывает эти gates.

| ID / слой | Конечные случаи | Условие приёмки |
|---|---|---|
| A1 — unit/SDK, K1 | Old accepted expired B; rejected new expired C; invalid-signature C оставляет B; valid new timestamp + later snapshot/target/app/cancel/cleanup error | Сохраняется только eligible predecessor; original bytes/source/context точны; artifact proof отсутствует при error; свой RefTime не подменён app completion time |
| A2 — unit/SDK, K1/K2 | Snapshot expiry/version/link mismatch; root expired intermediate/fresh final; max rotation continuation; S→S′→S; ambiguous raw serializations | Per-role semantics и full batch links; невозможно забыть applicable history из-за failed local read; no blanket pointer retention |
| A3 — publisher + SDK | S-only replacement; paired disjoint old-only signatures; overlap; threshold-only; material reuse/dual signatures; confirmed reset→fresh old T5→tail fail→new T6 ref1 | ResetAllowed/exact-envelope Q/compromised quorum/D разные; current continuation сохраняется; выбранная supported recipe имеет объявленные предпосылки. Не обещать recovery всех rotations |
| A4 — source/unit concurrency | E0 callback после accepted E1 deny; expiry без revision change; source/vault/envelope changes во время SDK/staging; readonly Set с mutable backing | Последний admission отвергает stale/expired/mixed source; существующий final native/writer gate сохранён; no side effect внутри retriable CAS |
| A5 — Windows native/durability | Actual concurrent new/old writer/readers; storage fault/disk full/native Close/move uncertainty; crash до/после root/reset/P/application commit; cold recovery | Подтверждён actual commit point/outcome, readers не принимают mixed cut; uncertain owner/barrier не теряются; application rollback не возвращает состояние до отдельно committed P |
| A6 — Android storage/device | Alias-only/file-only/pending stage/key invalidation; old authenticated encrypted blob; process death при rename/fsync; CE locked; reboot/elapsed discontinuity | Missing/uncertain не Empty; recovery не снижает floors; state заново semantic verified; AES не считается freshness/rollback proof |
| A7 — Android SDK/JNI/ABI | Inspect≠accept, five modes/independent approvals, zero-channel revoke, uint64 границы, progress→accepted relationship, late native return/cancel, 9MiB copy peak | Один accepted whole libbox artifact и Go runtime; no ambient FS/network/global mutation; buffers живы до return; production provider существует; marker-only artifact не допускается |
| A8 — Windows release/native/OS | Tamper after hash/signing; main/helper mismatch; initial install/no receipt; image replacement/ACL/reparse/old protocol; forward repair/current-code versus install purpose; readiness after restart | Exact approved final set и выбранный pre-execution channel; supported FS/OS/architecture scope, installed/executing/current-intent binding; no unsupported schema2 rollout |
| A9 — Android release/device | Wrong signer/lineage/OS range/versionCode; split/AAB mix; write→seal change; denied/low storage; lost async callback; installed new package with old process | Final approved APK set, OS-authorized installation, reconcile pending outcome и новая process/session admission; update callback не Ready |
| A10 — recovery/UX/native | Wrong recipient/purpose; same-channel substituted root+policy digest; old valid backup omitted revoke; lost checkpoint/key; no surviving anchor; valid offline adopted profile при failed refresh | Explicit independently sourced approval/as-of; честный blocked/recovery при нехватке основания; не вводится обязательная биометрия/онлайн-сервер и не отменяется valid adopted authority одним refresh error |
| A11 — optional attestation | Missing challenge/app identity, generic Success без needed flags, exact prompt vs extraData, revoked/suspended/unknown/malformed/duplicate status, late E0 after deny, unavailable source + still-valid old evidence | Claims не шире chain/key/status/confirmation evidence; acquisition error не normal; supported manual fallback/a11y policy; отсутствие production feature остаётся deferred |
| A12 — optional governance/provenance | Wrong expected builder/workflow/subject, V1 promise без inclusion, same operator multiple log IDs; sha256/sha512 representative permutations/custom mismatch; map rename/remove/readd; old map-ignorant client | Independently adopted expectations, exact agreement и lineage; no weak fallback для обязательного strong approval; новая policy не объявляется исполняемой старым клиентом |

**Live gate отдельно:** только после оплаченной и явно предоставленной инфраструктуры — protected DNS/egress/leak coverage, смена сети, revoke/cutover/update recovery под реальным трафиком. Даже успешные A1…A12 сами не доказывают работу при российских блокировках, пропускную способность, battery SLO или превосходство над другими VPN.

<a id="limits"></a>
## 7. Что исключить из ближайшего плана

- Собственный криптоалгоритм/canonicalizer, signature-only «TUF», UnsafeLocalMode, global clock/chdir/logger override, второй Go runtime и незаметное ослабление Android diskless требования.
- Новую БД, fifth journal target, reset-phase field, обязательный server-floor протокол или «универсальный witness» только из модели неразличимости. Сначала проверить existing atomic images и поддержанные positive branches.
- Переписывание V2-178…183 как будто snapshots/CAS/pin-policy closure ещё нет; перенос старых R05 source claims на изменившийся код; закрытие runtime статуса по source/gofmt/хешу.
- Reset всех floors после root rotation, неудачной расшифровки или потери файла; error→empty/old-as-fresh; недопустимый progress и, с другой стороны, потерю законного predecessor при любом error.
- Обязательную аттестацию/биометрию для reconnect, собственный ASN.1/CRL merger, загрузку больших schema/provenance/multirepo stacks до конкретной потребности.
- Обещание глобально latest offline revoke, total whole-process attestation, мгновенного отзыва уже admitted внешнего effect или рыночного превосходства из исторической UX-таблицы.

Остаток, требующий решения: scoped acceptance R05-010-D и R28 reset/status claims; выбор per-return/per-step persistence и supported recovery subset; ownership/licence общего module; accepted SDK/version и Android artifact; actual native durability/old-client/release/device/live приёмка. Это конечные зависимости предложенной разработки, а не повод продолжать бесконечный survey.
