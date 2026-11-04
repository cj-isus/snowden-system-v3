# R05-001 — источники и точные входы

Проверка: 2026-10-25. Публичные первоисточники и source-only анализ.
Ссылки — данные исследования, не инструкции на запуск/установку.

## Первоисточники

| ID | Версия / источник | Использование и граница |
|---|---|---|
| P1 | [TUF 1.0.36](https://theupdateframework.github.io/specification/v1.0.36/), 2026-08-05, §5.1/5.3/5.4/5.5 | Normative workflow: fixed time, root progression, dependent cache invalidation, final metadata acceptance. Не спецификация установки приложения/OS durability |
| P2 | [TUF 1.0.37](https://theupdateframework.github.io/specification/v1.0.37/), 2026-10-08 | Проверены именно относящиеся к срезу пункты. HTML получен прямым публичным HTTP; web open этой версии вернул internal error. Это не полная проверка diff всех версий |
| P3 | [go-tuf TrustedMetadata](https://github.com/theupdateframework/go-tuf/blob/f5edbde31e5507f46db2069402dc38903fe6d9d4/metadata/trustedmetadata/trustedmetadata.go#L103), v2.4.2, commit `f5edbde31e5507f46db2069402dc38903fe6d9d4` | UpdateTimestamp103…168, UpdateSnapshot179…259: assignment до final check, сравнения versions. Не самостоятельный storage/workflow контракт |
| P4 | [go-tuf Updater](https://github.com/theupdateframework/go-tuf/blob/f5edbde31e5507f46db2069402dc38903fe6d9d4/metadata/updater/updater.go#L307), тот же pin | onlineRefresh, loadTimestamp307…352, loadSnapshot355…413, loadRoot485…524, persistence. Ручное чтение, не conformance run |
| P5 | [go-tuf go.mod](https://github.com/theupdateframework/go-tuf/blob/f5edbde31e5507f46db2069402dc38903fe6d9d4/go.mod), [LICENSE](https://github.com/theupdateframework/go-tuf/blob/f5edbde31e5507f46db2069402dc38903fe6d9d4/LICENSE) | Go≥1.25.0 в upstream; Apache-2.0. Локальный Windows module закрепляет Go1.26 и v2.4.2. Android Go/JNI/whole-AAR compatibility этим не подтверждается |
| P6 | [python-tuf Updater](https://github.com/theupdateframework/python-tuf/blob/49eb1da48c58cf0cbb0f2e490f7ad39fbd8c1f15/tuf/ngclient/updater.py#L381), [TrustedMetadataSet](https://github.com/theupdateframework/python-tuf/blob/49eb1da48c58cf0cbb0f2e490f7ad39fbd8c1f15/tuf/ngclient/_internal/trusted_metadata_set.py#L216), [release7.0.1](https://github.com/theupdateframework/python-tuf/releases/tag/v7.0.1) | Commit `49eb1da48c58cf0cbb0f2e490f7ad39fbd8c1f15`, release2026-09-01. `pyproject.toml`: Apache-2.0 OR MIT, Python≥3.10; обе лицензии захвачены. Reference partial-error/persistence разделения; не готовое решение B1 и не новая зависимость |
| P7 | [legacy go-tuf0.7.0 client](https://github.com/theupdateframework/go-tuf/blob/6ad7fe593e4042db3544c4b0fedbe66bac371c42/client/client.go#L224), [LICENSE](https://github.com/theupdateframework/go-tuf/blob/6ad7fe593e4042db3544c4b0fedbe66bac371c42/LICENSE) | Commit `6ad7fe593e4042db3544c4b0fedbe66bac371c42`, BSD-3-Clause. KeyInfo comparison и invalidation335…361 — reference старого draft, не wholesale backport/замена v2 |
| P8 | [Android AtomicFile](https://developer.android.com/reference/android/util/AtomicFile) | API требует внешней синхронизации. Reference, готовый API не заменяет текущий encrypted CAS/recovery. Код не заимствован |
| P9 | [Microsoft FlushFileBuffers](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-flushfilebuffers), [MoveFileExW](https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-movefileexw) | Различные native flush/move guarantees; не сертификат `os.Rename` или конкретного диска. Reference; native вызовов не было |

## Фиксация upstream

Корень captures: `../../../../.artifacts/research/theory-05/R05-001/upstream/`.

- `go-tuf-f5edbde/`: trustedmetadata.go, updater.go, config.go, go.mod, LICENSE;
  SHA256/размеры — `inventory.json`. `go-tuf-tag.json` и annotated tag фиксируют
  разрешение v2.4.2 в полный commit выше.
- `python-tuf-49eb1da/`: updater.py, trusted_metadata_set.py, pyproject.toml,
  LICENSE и LICENSE-MIT; hashes в inventory. Release, tag и annotated tag
  сохранены отдельно. Анализ ограничен указанными файлами.
- `go-tuf-legacy/`: client.go и LICENSE; исходный ref v0.7.0 захвачен. Старые
  комментарии ссылаются на draft: таблица invalidation не переносится без review.
- Версионные TUF HTML и Android/Microsoft API HTML сохранены. Общий перечень
  capture checksums — `upstream-inputs.json` рядом с local-inputs.json.
- Hashing относится только к фиксации публичного source/документов; реальные
  подписи, ключи, профили и crypto verifier не исполнялись.

## Локальные source pointers

Все пути ниже относительны корню репозитория. Exact bytes, размеры, дата
захвата и SHA256 27 файлов — в `local-inputs.json`; копии под `local/`.
Незахваченные platform PLAN использованы только как указатель существующих IDs,
а не доказательство статуса реализации.

| Область | Прочитанные ключевые символы |
|---|---|
| Общая теория | `docs/CENSORSHIP-THEORY.md`: §3/7/12.1–3/5, §13.9, §15.3, OP-03/06, AD-01/06; новые выводы не интегрировались |
| Выбранный workflow | `docs/decisions/001-tuf-delivery.md`; `windows/go.mod`; `windows/backend/delivery/{client,model,policy}.go`: packageFetcher, PrepareArtifacts, retainedState, Prepared/Progress, guardMetadataShape |
| Retention | `windows/delivery_state.go`: retainDeliveryProgress и source fence; `windows/backend/metadata/delivery.go`: CheckDeliveryBinding/Proof |
| Transaction/time | `windows/backend/filetxn/filetxn.go`: Commit/Recover/atomicWrite; `windows/onboarding_deadline.go`; metadata `{clock,runtime_profile,validity}.go` |
| Android verifier request | `docs/android/CHANGE-REQUEST-TRUST-001.md`: T01…14; `android/profile/.../ProfileTrustDriver.kt`: ProfileTrustWork, inspect/verifyForApply; production implementation отсутствует по самому контракту |
| Android apply/storage | `ProfileImportCoordinator.kt`: applyReviewed; `ProfileCheckpointGuard.kt`: acceptsShape; `EncryptedProfileStore.kt`: write/load; `ProfileClock.kt`; `ProfileStoreCrypto.kt` |
| Android architecture | `android/decisions/006-profile-storage.md`, 007 ingress-clock, 008 import-coordinator, 015 runtime-expiry, 017 journal — последний также содержит блок TUF seam; `android/vpn/.../ConnectionState.kt`: RuntimeSourceProvider |

## Reuse/adapt/reference/reject

| Кандидат | Решение |
|---|---|
| Pinned go-tuf2.4.2 crypto/workflow | **Reuse условно** через согласованный seam. Не считать наличие full Updater доказательством A/B семантики или memory-only совместимости |
| Python-tuf7.0.1 | **Reference** для сравнения порядка мутации/ошибки/записи. Не переносить runtime и не объявлять B1 исправленным сменой языка |
| Legacy KeyInfo/invalidation | **Reference/adapt principle**, после сверки актуальной спецификации. BSD notice при переносе кода обязателен; в продукт код не копировался |
| Копировать все intermediate SDK pointers | **Reject как общий алгоритм retention**: не различает новый rejected объект и ранее принятый cached intermediate |
| Глобальный max счётчиков / root change сбрасывает всё | **Reject оба правила**: теряют соответственно recovery или rollback protection |
| Новый crypto/canonicalizer/второй AAR | **Reject**: научный вопрос решается уточнением существующей границы доверия |

## Drift и проверка пакета

Исследование относится к захваченным inputs. Последняя сверка текущих файлов
и замечания приводятся в handoff. Любой изменённый source требует адресной
перепроверки затронутого вывода. Собственная ручная проверка не является
независимой рецензией, тестированием либо принятием руководителем.

При финальном просмотре из 27 inputs изменился только
`android/decisions/017-bounded-session-journal.md`: добавлен маршрут к ADR-032
про preview/share controller journal. Diff просмотрен вручную; TUF seam block
не изменился, выводы A/B не затронуты. Snapshot сохранён исходным;
checksum расхождения записан в `drift.json`. Это не проверка нового journal кода.

---

<a id="sources-r05-002"></a>
## R05-002 — дополнительные источники и границы использования

2026-10-25. Предыдущий source register и immutable R05-001 captures сохранены.
Ниже — основания нового раздела REPORT §9, а не пересмотренный status A/B.

| ID | Exact первоисточник | Использование / не доказывает |
|---|---|---|
| Q1 | [TUF1.0.37 §5.3](https://theupdateframework.github.io/specification/v1.0.37/#update-root), 2026-10-08; дополнительно публично просмотрен [1.0.36 §5](https://theupdateframework.github.io/specification/v1.0.36/#detailed-client-workflow) | Root progression, persistence/reset ordering и metadata acceptance. Версионный HTML1.0.37 из предыдущего capture скопирован без изменения; полного межверсионного conformance исследования нет |
| Q2 | [go-tuf2.4.2 Updater](https://github.com/theupdateframework/go-tuf/blob/f5edbde31e5507f46db2069402dc38903fe6d9d4/metadata/updater/updater.go#L590), [TrustedMetadata](https://github.com/theupdateframework/go-tuf/blob/f5edbde31e5507f46db2069402dc38903fe6d9d4/metadata/trustedmetadata/trustedmetadata.go#L60) | Commit `f5edbde31e5507f46db2069402dc38903fe6d9d4`, Apache-2.0. Existing exact source replay: persistMetadata590…635, loadRoot/onlineRefresh, UpdateRoot/Timestamp/Snapshot. Workspace write/close/rename ≠ platform durable receipt; memory-only/pure-step contract не выбран |
| Q3 | [legacy go-tuf0.7.0](https://github.com/theupdateframework/go-tuf/blob/6ad7fe593e4042db3544c4b0fedbe66bac371c42/client/client.go#L224); compare [Python7.0.1](https://github.com/theupdateframework/python-tuf/blob/49eb1da48c58cf0cbb0f2e490f7ad39fbd8c1f15/tuf/ngclient/updater.py#L381) | BSD-3-Clause legacy comparison/invalidation335…361 с комментарием о старом draft; Python Apache-2.0 OR MIT. Эти implementations не уравниваются с одним normative reset predicate и не становятся новым SDK |
| Q4 | [bbolt1.5.0 tx.go](https://github.com/etcd-io/bbolt/blob/e7a8b2dd498494a3766ba24dd94d3509e5588485/tx.go#L170), [db.go](https://github.com/etcd-io/bbolt/blob/e7a8b2dd498494a3766ba24dd94d3509e5588485/db.go#L1140), [go.mod](https://github.com/etcd-io/bbolt/blob/e7a8b2dd498494a3766ba24dd94d3509e5588485/go.mod), [LICENSE](https://github.com/etcd-io/bbolt/blob/e7a8b2dd498494a3766ba24dd94d3509e5588485/LICENSE) | Release2026-06-21, commit `e7a8b2dd498494a3766ba24dd94d3509e5588485`, MIT. Tx.Commit170…294→write520→writeMeta595, handlers после close; NoSync и recovery meta selection. Go1.25.0/toolchain1.25.11 declared; API read, не сборка/подтверждение Android/Windows compatibility |
| Q5 | [Izraelevitz, Mendes, Scott, TR999](https://www.cs.rochester.edu/u/scott/papers/2016_Izraelevitz_TR999.pdf), University of Rochester, July2016 | Full-system-crash NVM model и durable vs buffered durable linearizability, определения3/4 (§2, PDF pages4–5). Терминологический фон; его ISA/NVM results не переносятся автоматически на файловую систему или наш SDK. Собственные различимость/K/API proofs сформулированы отдельно |
| Q6 | [SQLite atomic commit](https://sqlite.org/atomiccommit.html), [synchronous](https://sqlite.org/pragma.html#pragma_synchronous), [copyright](https://sqlite.org/copyright.html) | Reference journal/commit и различие durability режимов. WAL NORMAL против WAL FULL; rollback режим отдельно. Deliverable code/docs — public domain по заявлению upstream, build scripts могут иметь иные условия. Binary/driver/VFS version не выбраны, dependency не вводится |
| Q7 | [python-tuf7.0.1 reference test source](https://github.com/theupdateframework/python-tuf/blob/49eb1da48c58cf0cbb0f2e490f7ad39fbd8c1f15/tests/test_updater_top_level_update.py#L410) | Только прочитанный исходник: timestamp recovery410, snapshot491 (paired rotation), targets599. Это сильное counterevidence к обобщению S-only ветки. Код не исполнялся; тесты/fixtures проекта не создавались |

### Captures и provenance R05-002

Корень: `../../../../.artifacts/research/theory-05/R05-002/`.

- `baseline/REPORT-R05-001.md`, `SOURCES-R05-001.md`, `handoff-R05-001.md` —
  сохранённые предыдущие документы до append. Старый handoff в docs не менялся.
- `local-inputs.json`, `local/`: 16 адресных документов/consumer/storage source.
  Own START/depth question, THEORY/ADR-001, Android T01…14/ADR006/008/015,
  EncryptedProfileStore/ProfileClock/ProfileCheckpointGuard/ProfileImportCoordinator/
  ProfileTrustDriver interface; Windows filetxn, delivery_state и import deadline.
- `reference-r05-001/`: byte copies versioned spec и двух exact upstream Go
  source из прежнего собственного capture. Исходные upstream refs/licenses
  остаются в immutable R05-001 inventory, повторной загрузкой pins не менялись.
  Отдельно скачан pinned Python test **source**, сохранён как `.txt` для чтения,
  без исполнения или подключения к какому-либо runner.
- `upstream/bbolt/`: tx.go, db.go, README.md, go.mod и LICENSE; release/ref/tag
  metadata в корне R05-002 разрешают tag в полный commit.
- Author PDF Q5 и официальные SQLite HTML сохранены; retrieval/source hashes —
  `external-inputs.json`. Hashing — provenance публичных документов, не crypto run.
- Product SDK, `shared/trust` implementation, реальные states/vault/Keystore/
  профили/env/logs не открывались. Нет вывода об их текущей корректности.

### Actual caller / branch / consumer нового среза

| Путь и символ | Наблюдаемая ветка | Использование |
|---|---|---|
| `windows/delivery_state.go:retainDeliveryProgress` → `filetxn.Store.Commit` | Source fence, existing four-target set, Keep остальных файлов, journal transaction | PC mapping логических progress и application cuts; не доказательство root/reset семантики product SDK |
| `windows/backend/filetxn/filetxn.go:Commit/Recover/atomicWrite` | Backup/stage, BeforeCommit, публикация barrier, replace, снятие barrier; failure recovery | Rollback возвращает transaction start; нужны отдельные semantic/durability gates |
| Android `ProfileImportCoordinator.applyReviewed` → `store.write(progress)` → fresh clock/source → `store.write(replacement)` | Progress revision используется для следующего CAS; profileCommitAdmitted предшествует actual write | Scope intent linearization отличается от storage completion/adopt |
| Android `EncryptedProfileStore.write/load` | mutationAdmitted/commitCompleted, file/dir sync, typed Uncertain; alias-only != Empty | Разные persistence domains и барьер неизвестного эффекта |
| Android `ProfileTrustDriver` interface / request T07/T09/T10/T11 | Pure synchronous return, output progress, explicit time, minimum applicable bounds | API feasibility/purpose questions; production implementation не читалась |
| Android `ProfileClock.observe/currentEpochMs/authorityDeadlineElapsedMs` | Local wall/elapsed continuity, floor, short observation freshness | Не authenticated UTC bound; не переносить elapsed permit через reboot |
| bbolt pinned `Tx.Commit`, `write`, `writeMeta` | См. Q4 и REPORT §9.5/9.7 | Caller/branch для reference API boundary; не код продукта |

### Reuse decisions и unresolved choices

Сначала существующие platform stores; только потом — обоснованная смена storage.
bbolt — reference/условный кандидат, не установлен. Mmap/borrowed byte lifetime,
single writer/locking, bounded DB growth и whole-Go integration требуют gates.
Нельзя выполнять README recipe с отдельным gomobile AAR как часть исследования.
SQLite — reference транзакционной семантики, не обещание Android default FULL
или сохранности credentials в обычной DB/WAL. Для production понадобятся
конкретные platform binary/driver pins и подтверждённый режим.

Неразрешённые вопросы руководителю: точный ResetAllowed при intermediate
rotation/threshold changes; требование per-root durable barrier против per-return
progress; purpose применимость TUF expiry после принятия профиля; local storage
adversary/independent recovery anchor и требуемая automatic liveness.
Готовый paired-key publisher recipe может сократить state/interop scope;
это отдельный вариант с quorum/cache/key-reuse условиями, а не доказательство,
что всё исследованное recovery уже работает в существующем SDK.

Final relevant drift и самостоятельная оппонирующая проверка — в handoff R05-002
и artifacts `opposing-review.md`/`drift.json`. Это не независимое научное ACCEPTED.

На последней provenance сверке все 16 локальных inputs совпали со снимками;
`drift.json` содержит пустой drift. `preservation.json` подтверждает исходные
R05-001 REPORT/SOURCES как неизменённые prefix и прежний handoff checksum.
16 external captures включают source/лицензию/метаданные tag/документы, а не
16 независимых оснований. Тестовый source Q7 только читался и не исполнялся.

<a id="sources-r05-003"></a>

## R05-003 — release provenance / offline transparency

Получено публичным web/source чтением в текущем срезе; после deadline только
упаковка уже полученного материала. Утверждения относятся к указанному pin,
не к любой будущей версии. Код/зависимости не копировались в продукт.

| ID | Первоисточник | Использование / ограничение |
|---|---|---|
| U1 | [SLSA v1.2 verification](https://slsa.dev/spec/v1.2/verifying-artifacts) | Consumer expectations и provenance policy. Утверждённая v1.2, не ранее найденный rc2. Документ/reference; Community Specification License1.0, не лицензия всех возможных инструментов |
| U2 | [SLSA v1.2 provenance](https://slsa.dev/spec/v1.2/build-provenance) | Predicate v1, builder trust boundary, untrusted externalParameters, completeness зависит от builder. Не proof production pipeline |
| U3 | [Sigstore bundle](https://docs.sigstore.dev/about/bundle/) | Format0.3.2 по документации; historical signing time, informational message digest. Страница предупреждает, что описание не исчерпывает formal protobuf schema; совместимость всех bundles не проверена |
| U4 | [sigstore-go signed_entity.go](https://github.com/sigstore/sigstore-go/blob/22d3691c7b8e0c5530fae3c05577690bfef5cd00/pkg/verify/signed_entity.go) | NewVerifier/Validate; PolicyConfig/defaults; artifact/identity options; predicate omission; Verify/observer time. Не готовая SLSA policy и не полный wrapper/network audit |
| U5 | [sigstore-go tlog.go](https://github.com/sigstore/sigstore-go/blob/22d3691c7b8e0c5530fae3c05577690bfef5cd00/pkg/verify/tlog.go) | cap/duplicate guards; unique key IDs; V1 promise/proof, V2 proof; timestamp binding. Не независимость операторов и не consensus/gossip proof |
| U6 | [release v1.3.0](https://github.com/sigstore/sigstore-go/releases/tag/v1.3.0), [commit](https://github.com/sigstore/sigstore-go/commit/22d3691c7b8e0c5530fae3c05577690bfef5cd00), [go.mod](https://github.com/sigstore/sigstore-go/blob/22d3691c7b8e0c5530fae3c05577690bfef5cd00/go.mod), [LICENSE](https://github.com/sigstore/sigstore-go/blob/22d3691c7b8e0c5530fae3c05577690bfef5cd00/LICENSE) | Published2026-07-30T14:53:02Z; Go1.25.8; Apache-2.0. Full commit40hex; transitive licenses/whole Android ABI/runtime/resources не проверены |

Captures: `.artifacts/research/theory-05/R05-003/upstream/`. `web-*` — сохранённые
текстовые результаты web tool с source line labels/метаданными. Это не raw Git
blob archive. `sigstore-go-pin-verified.json` — действительная запись pin.
`sigstore-go-pin.json` с null commit и `sigstore-go-ref.json` — failed fetch
evidence; использовать их как pin нельзя. GitHub API ref timeout и read-only
git ls-remote DNS failure сохранены в пояснении; OS сеть не перенастраивалась.
Несуществующий pkg/verify/policy.go вернул404; policy изучена в signed_entity.go.

`web-primary-specs.txt` и `web-verification-docs.txt` включают web-rendered docs;
main README/verification examples служили ориентацией. Старый Go minimum из
main docs не подменяет pinned go.mod. Reference snippets не исполнялись.

Local closing inputs: windows/updateapp.go, windows/update_readiness.go,
windows/update_pair.go, docs/ANDROID-PLAN.md. Hashes/copies — closing-inputs.json
и local-closing/. Это post-read closing capture, не initial snapshot; полной
доказанной drift-free истории этих четырёх inputs нет. SDK source, saved states,
credentials, raw user logs не читались. Общая THEORY не изменялась.

NOT_RUN: Sigstore/TUF криптография, build/tests/fixtures, installer/image/native
signing, Android device, network/runtime/performance и реальные обновления.
Exact Authenticode/APK signing semantics, policy-update/revocation и complete
offline wrapper dependency closure остаются адресными последующими вопросами.

<a id="sources-r05-004"></a>

## R05-004 — OS signing, installation, execution

Проверено2026-10-26, новый source-only срез. Публичные данные не команды на запуск.

| ID | Exact primary / version / caller | Использование и предел |
|---|---|---|
| V1 | [Microsoft PE signing](https://learn.microsoft.com/en-us/windows/win32/secbp/understanding-pe-signatures), updated2025-07-31; [PE format](https://learn.microsoft.com/en-us/windows/win32/debug/pe-format) | SIP/Authenticode hash exclusions против flat digest. Не Windows native validation |
| V2 | [WinVerifyTrust](https://learn.microsoft.com/en-us/windows/win32/api/wintrust/nf-wintrust-winverifytrust), [FILE_INFO](https://learn.microsoft.com/en-us/windows/win32/api/wintrust/ns-wintrust-wintrust_file_info), [DATA](https://learn.microsoft.com/en-us/windows/win32/api/wintrust/ns-wintrust-wintrust_data) | Zero-only success, hFile+path, cache-only vs no-revocation. OS API reference, DLL version не снята; полного adapter caller продукта не заявлено |
| V3 | [CreateFile](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-createfilew), [FILE_ID_INFO](https://learn.microsoft.com/en-us/windows/win32/api/winbase/ns-winbase-file_id_info), [CreateProcess](https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-createprocessw), [QueryFullProcessImageName](https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-queryfullprocessimagenamew) | Sharing/object/name/initialization scope. Условия support для API+actual filesystem ещё native gate; FILE_ID_INFO страница имеет ограничительную minimum-client запись, поддержка всех клиентов не выведена |
| V4 | [DLL search order](https://learn.microsoft.com/en-us/windows/win32/dlls/dynamic-link-library-search-order), [DLL security](https://learn.microsoft.com/en-us/windows/win32/dlls/dynamic-link-library-security) | Dependencies/search closure, primitive reuse. Code signing основного EXE не whole-process code inventory |
| V5 | [MSIX signing](https://learn.microsoft.com/en-us/windows/msix/package/signing-package-overview), [update constraints](https://learn.microsoft.com/en-us/windows/msix/app-package-updates), [update settings](https://learn.microsoft.com/en-us/windows/msix/app-installer/update-settings) | Conditional non-Store runtime integrity Windows2004+, family/signing identity, downgrade override. No migration/installer/runtime proof; цены услуг не использовались |
| V6 | [APK v2](https://source.android.com/docs/security/features/apksigning/v2), updated2026-09-30; [v3](https://source.android.com/docs/security/features/apksigning/v3), [v3.1](https://source.android.com/docs/security/features/apksigning/v3-1) | Protected sections/anti-stripping/rotation by OS range. Новая [v3.2](https://source.android.com/docs/security/features/apksigning/v3-2) требует Android17; поддержка её старому pin не приписана |
| V7 | [apksig tag android-16.0.0_r2](https://android.googlesource.com/platform/tools/apksig/+/refs/tags/android-16.0.0_r2), tag2025-06-10; commit `61f88cbe6f81b4536d1ef124daf3caa4ad014afd`; [ApkVerifier](https://android.googlesource.com/platform/tools/apksig/+/61f88cbe6f81b4536d1ef124daf3caa4ad014afd/src/main/java/com/android/apksig/ApkVerifier.java), [LICENSE](https://android.googlesource.com/platform/tools/apksig/+/61f88cbe6f81b4536d1ef124daf3caa4ad014afd/LICENSE), [build.gradle](https://android.googlesource.com/platform/tools/apksig/+/61f88cbe6f81b4536d1ef124daf3caa4ad014afd/build.gradle) | Apache2; public verify150→private183→containsErrors606→setVerified611; Builder3470 null/range/defaults. Root README: host usage; Java1.8 declaration and generic KMS deps not whole-client compatibility. Source fetched, verifier не исполнялся |
| V8 | [frameworks/base android-16.0.0_r2](https://android.googlesource.com/platform/frameworks/base/+/refs/tags/android-16.0.0_r2), tag2025-06-10, commit `99b01a65cc4c104933788b3143285ab6bae65827`; [PackageManager](https://android.googlesource.com/platform/frameworks/base/+/99b01a65cc4c104933788b3143285ab6bae65827/core/java/android/content/pm/PackageManager.java#11132), [SigningInfo](https://android.googlesource.com/platform/frameworks/base/+/99b01a65cc4c104933788b3143285ab6bae65827/core/java/android/content/pm/SigningInfo.java#136), [API](https://developer.android.com/reference/android/content/pm/SigningInfo), updated2026-08-03 | Apache2 headers; historical vs current/set identity; API definitions/implementation, не complete package-manager service audit. Methods bodies may be base stubs; поведение service не выведено из stub exception |
| V9 | [PackageInstaller.Session](https://developer.android.com/reference/android/content/pm/PackageInstaller.Session), [SessionParams](https://developer.android.com/reference/kotlin/android/content/pm/PackageInstaller.SessionParams), [PackageInstaller](https://developer.android.com/reference/android/content/pm/PackageInstaller) | API21 session/seal/async status, API34 dontKillApp/update ownership, permissions/default guards. Callback lost/history native/device не проверены |
| V10 | [Play signing](https://developer.android.com/studio/publish/app-signing), [bundle FAQ](https://developer.android.com/guide/app-bundle/faq), [splits](https://developer.android.com/guide/app-bundle/app-bundle-format), [versioning](https://developer.android.com/studio/publish/versioning) | Upload/app keys, final APK transformation, set coverage и installed-relative versionCode. Store policy/restrictions выбирает владелец; никакой публикации |
| V11 | [Android DCL risks](https://developer.android.com/privacy-and-security/risks/dynamic-code-loading), updated2024-09-24; [Android14 DCL changes](https://developer.android.com/about/versions/14/behavior-changes-14) | Closure beyond package; read-only requirement не объявлен полным запретом DCL. No app feature/code changes |

Captures: `.artifacts/research/theory-05/R05-004/`. Raw base64-decoded Gitiles source
и metadata fetch records в upstream; apksig/context pins различны. Browser open
некоторых exact Gitiles file URLs дал inaccessible; direct public format=TEXT
для пяти основных файлов дал успешные byte captures. Android.bp/CLI caller
дальше не загрузились (timeouts), Windows signature-settings doc — network error
дважды. Никаких «исходник проверен» для отсутствующих файлов.

Web rendered records сохранены отдельно; их hashes не Git blob hashes.
Local inputs до финализации: update_image.go/update_accepted_image.go/update_pair.go/
update_readiness.go и ANDROID-PLAN.md; snapshots/hashes в local-inputs.json.
Только platform facade, SDK implementation и реальные user states не читались.
Окончательные preservation/drift — в handoff/artifacts. Все runtime проверки NOT_RUN.

<a id="sources-r05-005"></a>

## R05-005 — отрицательное знание и офлайн отзыв

Primary чтение2026-10-26. Версионные RFC/reference; никаких crypto/provider runs.

| ID | Primary / дата / exact section | Применение и предел |
|---|---|---|
| W1 | [RFC6960](https://www.rfc-editor.org/rfc/rfc6960.html), June2013, §2.2/2.4/3.2/4.2.2.1/4.2.2.2 | Good/revoked/unknown, CertID/responder authority, times, optional nextUpdate. Это base protocol, не гарантия произвольной библиотеки |
| W2 | [RFC5019](https://www.rfc-editor.org/rfc/rfc5019.html), September2007, §2.2.4/4/6 | Cache/preproduction profile требует nextUpdate; accurate time/nonces. Отличается от general RFC6960; IETF Trust terms, code implementation не заимствована |
| W3 | [RFC5280](https://www.rfc-editor.org/rfc/rfc5280.html), May2008, §5.1.2.5/5.2.3/5.2.4/5.3.1/5.3.2 | Issuer+scope numbering, delta/base matching, hold/remove and invalidityDate. Не полный PKIX conformance audit/свой CRL merger |
| W4 | [CertGetCertificateChain](https://learn.microsoft.com/en-us/windows/win32/api/wincrypt/nf-wincrypt-certgetcertificatechain), pTime/flags/return; [CERT_TRUST_STATUS](https://learn.microsoft.com/en-us/windows/win32/api/wincrypt/ns-wincrypt-cert_trust_status) | Current root/trust/revocation despite historical time; cache-only dimensions, typed unknown/stale. OS API reference; реальный current store/DLL/provider не читался |
| W5 | [CertVerifyCertificateChainPolicy](https://learn.microsoft.com/en-us/windows/win32/api/wincrypt/nf-wincrypt-certverifycertificatechainpolicy), return value | BOOL completion плюс dwError; не правило WinVerifyTrust. Product caller defect не заявлен |
| W6 | [Uptane Standard2.1.0](https://uptane.org/docs/2.1.0/standard/uptane-standard), §5.4/5.4.4.2 | Secure time, two-repository matching, full vs partial. Exact doc version, не current latest implementation pin. License implementation не исследована; site footer All Rights Reserved → только reference, без заимствования текста/кода |
| W7 | THEORY architecture §16.6 и R05-002/004; TUF1.0.36/37 captures R05-001 | Существующий baseline limits/latest-known/explicit purposes и pinned TUF references не новая находка. Product SDK не читался |

Caller boundaries: OS CertGetCertificateChain→context trust status, отдельный
CertVerifyCertificateChainPolicy→dwError; это документированный API contract,
не выполненный product caller. Android SigningInfo/PackageManager pin из V8
использован только для contrast с app/PKI scope; не заявлен Android OCSP adapter.

Captures в `.artifacts/research/theory-05/R05-005/upstream/`: web-rendered source
records, не raw RFC archive. Их hashes — capture provenance. Baseline relevant
THEORY §16.6 прочитан; повтор прежнего impossibility claim явно отделён от новых
уточнений. Собственных local product inputs нет; ранее собранные R05-004 facade
inputs не выданы за fresh runtime proof. Все10 будущих cases NOT_RUN.

<a id="sources-r05-006"></a>

## R05-006 — TAP4 / pinned multirepo

Primary public чтение2026-10-26, никаких runs/crypto. Готовое не значит approved.

| ID | Exact primary / version / caller | Scope |
|---|---|---|
| X1 | [multirepo.go](https://github.com/theupdateframework/go-tuf/blob/f5edbde31e5507f46db2069402dc38903fe6d9d4/metadata/multirepo/multirepo.go), go-tuf2.4.2, commit `f5edbde31e5507f46db2069402dc38903fe6d9d4` | NewConfig77/New102/init122/Refresh172/GetTargetInfo221/Download314/EnsurePaths346. Scope guards, lookup, IO; не продуктовый SDK |
| X2 | [metadata.go Equal](https://github.com/theupdateframework/go-tuf/blob/f5edbde31e5507f46db2069402dc38903fe6d9d4/metadata/metadata.go#L466), тот же pin | Length/Hashes.Equal, custom не входит; внутренний Hashes.Equal audit не выполнен |
| X3 | [TAP4](https://github.com/theupdateframework/taps/blob/master/tap4.md), v1 Accepted, Last-Modified24-Jun-2020, TUF-Version1.0.0 | Датированный design reference с master, immutable doc commit не закреплён; implementation анализируется по X1 pin |
| X4 | R05-001 captured pinned go.mod/LICENSE; X1 Apache2 header | Apache-2.0, Go≥1.25.0; API/whole-Go Android/device/native/IO compatibility не доказана |

Captures `.artifacts/research/theory-05/R05-006/upstream/`:4 web-rendered records,
не raw Git blob archive. Пакет назван metadata/multirepo, первоначальное поисковое
multirepoupdater не являлось доказательством имени API. Versioned pkg.go.dev open
не удался; exact raw pinned code доступен и прочитан. No dependency installation.
Публичный upstream caller chain в X1 рассмотрен; актуальное использование этого
API продуктом не утверждается. Opposing review и границы — §последний R05-006.md.
10 future cases NOT_RUN. Shared theory/product plans/SDK не изменены.

Финальная source партия10:00:39Z→10:01:49Z вернулась до deadline: TAP4 copyright
Public Domain и old-client ignore compatibility подтверждены. types.go получен
как capture, но внутренний Hashes.Equal audit не завершён и не объявлен
выполненным. Сохранение после10:03:26Z не включало новые source requests.

<a id="sources-r05-007"></a>

## R05-007 — map governance, migration и exact hash compatibility

Публичные источники/dated consumers прочитаны2026-10-26 в новом4h окне,
первый UTC10:54:29Z; основные raw/refetch batches11:19…11:25Z. Вызовы до/после
ограничивались UTC checks. Никакой verifier/crypto/client IO не исполнялся.

| ID | Exact primary / date / caller | Что доказывает / граница |
|---|---|---|
| Y1 | [TAP4 pinned](https://github.com/theupdateframework/taps/blob/6096e30ca04eb4a79e1d23a2aa9ac5ad64a6d63b/tap4.md), v1 Accepted, Last-Modified24-Jun-2020; commit6096e30ca04eb4a79e1d23a2aa9ac5ad64a6d63b, committer2023-03-22T16:34:23Z | Adopter assignment/mapping and custom exemption/legacy behavior. Exact pin теперь установлен через public API; Public Domain. Не готовое signed-map bootstrap/lifecycle implementation |
| Y2 | [multirepo.go](https://github.com/theupdateframework/go-tuf/blob/f5edbde31e5507f46db2069402dc38903fe6d9d4/metadata/multirepo/multirepo.go), go-tuf2.4.2, commitf5edbde31e5507f46db2069402dc38903fe6d9d4 | Raw lines NewConfig84/New112/init134, metadataDir150/config.New167, GetTargetInfo241, threshold297. Guard presence и alias/cache coupling; не доказательство отсутствия platform stable store |
| Y3 | [metadata.go](https://github.com/theupdateframework/go-tuf/blob/f5edbde31e5507f46db2069402dc38903fe6d9d4/metadata/metadata.go#L488), [Hashes.Equal852](https://github.com/theupdateframework/go-tuf/blob/f5edbde31e5507f46db2069402dc38903fe6d9d4/metadata/metadata.go#L852), VerifyLengthHashes471→verifyHashes880; [updater](https://github.com/theupdateframework/go-tuf/blob/f5edbde31e5507f46db2069402dc38903fe6d9d4/metadata/updater/updater.go) DownloadTarget213→Verify257/cached297 | Exact common-overlap match, no custom, all descriptor hashes/unknown algorithm rejection. Ручной symbolic liveness trace, не exploit/crypto execution. Updater raw captured ранее вR05-001, здесь адресно прочитан; новый metadata raw захвачен |
| Y4 | [TUF1.0.36 §2.1.1/§5.3/§6.1](https://theupdateframework.github.io/specification/v1.0.36/#key-management-and-migration), modified2026-08-05; [config.go pinned](https://github.com/theupdateframework/go-tuf/blob/f5edbde31e5507f46db2069402dc38903fe6d9d4/metadata/config/config.go#L64) | OOB trusted root, old+new sequential rotation, threshold-compromise boundary; default MaxRootRotations256. Не global current knowledge и не unbounded catch-up одним вызовом; spec newer-version conformance не заявлена |
| Y5 | [Go filepath.Match API](https://pkg.go.dev/path/filepath#Match), retrieved2026-10-26 | OS-specific escaping/Separator semantics; byte-identical map не гарантирует одинаковый parser outcome. Actual Windows/Android paths/native runtime не проверены; версия toolchain support должна быть закреплена платформой |
| Y6 | Dated Windows onboarding_public.go14…96, onboarding.go OnboardingPreview; update_image.go; Android PLAN min29/AND-04/14; THEORY16.6 | Existing independent-approval/preview/source identities и planned platform gates; mappings implementation в SDK не читалась и не объявлена отсутствующей |

go-tuf Apache-2.0, Go≥1.25.0 по immutable R05-001 go.mod/LICENSE captures;
TAP4 Public Domain. Примитивы reuse/adapt, не новая dependency/отдельный Android
AAR. Primary official Go API — reference, source не копировался в продукт.

Artifacts `.artifacts/research/theory-05/R05-007/`: raw public refetch metadata/
multirepo/types/config, exact TAP4, GitHub pin/tree metadata и web records.
config refetch SHA совпал с R05-001: web line labels удаляют blank lines и не
заменяют raw source line references. Некоторые early bundled web/API attempts
дали connection failure; native public HTTP затем дал valid pin/source. Failed
records не считаются доказательством содержимого. Git/network settings не менялись.

Source hashes/manifest фиксируют bytes, не runtime behavior. Local-inputs.json
содержит4 initial snapshots; onboarding.go — supplemental capture после чтения
type seam. No claim целостного atomic checkout пяти файлов. Final relevant drift/
preservation и самостоятельный opposing review в artifacts/handoff. Все13 cases
NOT_RUN, новые тесты/fixtures не созданы.

<a id="sources-r05-008"></a>
## Z. R05-008 — native backup/recovery sources

Дата среза: **26 октября 2026**.

| Источник | Что фиксирует |
|---|---|
| [ms-PKI-DPAPIMasterKeys — схема AD (ru-RU)](https://learn.microsoft.com/ru-ru/windows/win32/adschema/a-mspkidpapimasterkeys) | Доменное хранение DPAPI master keys — официальный русскоязычный документ Microsoft |
| [MS-BKRP — Backup Key Restoration Protocol](https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-bkrp/7cbbd712-dcf9-49ce-a9a4-611a62bfba7e) | Протокол восстановления DPAPI-секретов через домен; применим только при домене |
| [MS Support: DPAPI недоступен после admin password reset](https://support.microsoft.com/sl-si/help/3038562/cannot-access-dpapi-data-after-an-administrator-resets-your-password-o) | Подтверждение границы: сброс пароля администратором делает blob невосстановимым |
| [CryptProtectData (Win32 API)](https://learn.microsoft.com/en-us/windows/win32/api/dpapi/nf-dpapi-cryptprotectdata) | Базовый контракт DPAPI (en-US canonical; ru-RU перевод страницы API не пинован) |
| [Android Keystore / KeyMint](https://developer.android.com/privacy-and-security/security-crypto), [NIST SP 800-63B-style user-presence идеи через setUserPresenceRequired](https://developer.android.com/reference/android/security/keystore/KeyGenParameterSpec.Builder) | Неэкспортируемость hardware-ключей, invalidation, presence/authentication флаги (canonical en-US) |
| [Auto Backup / резервное копирование данных](https://developer.android.com/identity/data/backup) | Правила переноса app-data; ключи keystore бэкапом не переносятся |
| Snowden source (dated, read-only) | windows/app.go + delivery_state.go (DPAPI vault), sysutil_windows.go:80-84 vaultDir; android AndroidManifest.xml:4 allowBackup=false; ProfileStoreCrypto.kt:24-31 KeyGenParameterSpec, setUserAuthenticationRequired(false) |

Remote byte equality не заявляется; SDK/runtime не читались и не исполнялись.
Доменные сценарии — граница теории, не предпосылка Snowden. Все future cases
NOT_RUN.
