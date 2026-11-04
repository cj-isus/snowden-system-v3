# R05-010 — источники, версии и пределы чтения

Пакет **R05-010**, дата среза **27 октября 2026**. Все часы ниже — UTC.
«Current source» означает исходники этого среза, не актуальную версию продукта.

## 1. Как понимать реестр

`FULL_TEXT_READ` ниже означает чтение **полного текста указанного раздела/файла
по заявленной области**, а не автоматически всей длинной спецификации. Для
огромного документа всегда указаны прочитанные sections, прочие не audited.
Search snippets и библиографические каталоги в доказательства не включены.
`FETCH_FAILED` не означает отсутствие документа или отрицательный source-факт.

Artifact root:
`.artifacts/research/theory-05/2026-10-27-server-floor-contract-res7/`.
`remote/` — полученные public texts, `local/` — selected project-source copies.
Capture hashes — воспроизводимость bytes, не подписи источника и не binary
attestation. Реальные saved states, SDK execution и пользовательские vault не
входят в корпус.

## 2. Первичные спецификации и фактически полученный русский источник

| ID | Организация / автор; название; публикация | URL / exact sections | Чтение и статус |
|---|---|---|---|
| S1 | The Update Framework Authors, **TUF Specification1.0.36**; Last modified **2026-08-05** (header сам документ) | [versioned spec](https://theupdateframework.github.io/specification/v1.0.36/), §§2.1.1,5.1–5.6,6.1 | FULL_TEXT_READ выбранных нормативных разделов,27.10.2026 ~13:18–13:33; raw HTML248581bytes. Root persistence before final expiry; incoming expired T/S discard; role-key rotation reset; withholding/freeze. Полный conformance audit не заявлен |
| S2 | The Update Framework Authors, **TUF Specification1.0.37**; Last modified **2026-10-08** | [versioned spec](https://theupdateframework.github.io/specification/v1.0.37/), §§5.3–5.6 | FULL_TEXT_READ critical corresponding paragraphs,27.10.2026 ~13:33. Порядок persist/root expiry и discard T/S совпадает в прочитанных passages; остальные изменения editions не audited |
| S3 | B. Laurie, E. Messeri, R. Stradling / IETF, **Certificate Transparency Version2.0**, RFC9162, **December2021**, Experimental | [RFC Editor TXT](https://www.rfc-editor.org/rfc/rfc9162.txt), §§2.1.4,4.9–4.11,11/11.3 | FULL_TEXT_READ указанных разделов,27.10.2026 ~13:24–13:27; header прочитан. Append-only proof between heads; STH timestamp/tree_size/root_hash; gossip не определяется этим RFC. Не TUF implementation и не новое авторизационное полномочие |
| S4 | H. Birkholz, D. Thaler, M. Richardson, N. Smith, W. Pan / IETF, **Remote ATtestation procedureS (RATS) Architecture**, RFC9334, **January2023**, Informational | [RFC Editor TXT](https://www.rfc-editor.org/rfc/rfc9334.txt), §10.1–10.4 с introductory caveats §10 | FULL_TEXT_READ раздела10,27.10.2026 ~13:26–13:27. Trusted clocks/nonces/epoch IDs; свежая подпись не делает старые actual claim values свежими; изменение policy после подписи возможно |
| S5 | Microsoft, **Функция FlushFileBuffers (fileapi.h)**; page metadata `ms.date=2018-12-05` (не дата каждого исправления перевода) | [фактически ru-RU](https://learn.microsoft.com/ru-ru/windows/win32/api/fileapi/nf-fileapi-flushfilebuffers), Parameters/Return value/Remarks | FULL_TEXT_READ названных разделов,27.10.2026 ~13:37–13:39. HTML `lang=ru-ru` и русское тело прочитаны; это не предположение по замене locale в URL. WriteFile может буферизовать, FlushFileBuffers пишет buffered data. Не аттестация Go/FS durability и не просьба flush/admin сейчас |

Для TUF state/progress не обнаружен **прочитанный и подтверждённый** русский
официальный перевод этих точных редакций. Непроверенный перевод не цитируется
ради языковой квоты; report сформулирован по-русски с primary en-US spec/source.
S5 — действительное официальное русское API reference в узкой storage-области.

## 3. Exact upstream source: go-tuf v2.4.2

Manifest проекта: `windows/go.mod:13`, module go1.26.
Pinned commit: **f5edbde31e5507f46db2069402dc38903fe6d9d4**.
Издатель: The Update Framework Authors. Copyright header2024 — **не** дата release.
Дата публикации release/commit не устанавливалась новым успешным GitHub API
response; capture/read date —27.10.2026,13:18–13:33.

| ID | Immutable URL / exact область | Статус |
|---|---|---|
| U1 | [metadata/trustedmetadata/trustedmetadata.go](https://github.com/theupdateframework/go-tuf/blob/f5edbde31e5507f46db2069402dc38903fe6d9d4/metadata/trustedmetadata/trustedmetadata.go), :27–361 | FULL_TEXT_READ файл. UpdateRoot61–94; UpdateTimestamp103–168; UpdateSnapshot179–259; UpdateDelegatedTargets268–335; loadTrustedRoot340–360. Получен raw по immutable path |
| U2 | [metadata/updater/updater.go](https://github.com/theupdateframework/go-tuf/blob/f5edbde31e5507f46db2069402dc38903fe6d9d4/metadata/updater/updater.go), :76–209,307–522,590–656,669–689 | FULL_TEXT_READ указанных функций: constructor/Refresh/order/cache/read/persist, metadata access, UnsafeSetRefTime. Остальные target download functions прочитаны только для API-навигации, не full download audit |
| U3 | [metadata/config/config.go](https://github.com/theupdateframework/go-tuf/blob/f5edbde31e5507f46db2069402dc38903fe6d9d4/metadata/config/config.go), весь файл | FULL_TEXT_READ. Initial root explicit; fetcher/cache/unsafe options; MaxRootRotations256; no standard client floor/checkpoint API в этой конфигурации |
| U4 | [metadata/errors.go](https://github.com/theupdateframework/go-tuf/blob/f5edbde31e5507f46db2069402dc38903fe6d9d4/metadata/errors.go), :28–203 | FULL_TEXT_READ selected error hierarchy. ErrExpiredMetadata119–126 и ErrBadVersionNumber75–84 входят в ErrRepository; lower mutation+error может продолжаться для локального cache |
| U5 | [metadata/metadata.go](https://github.com/theupdateframework/go-tuf/blob/f5edbde31e5507f46db2069402dc38903fe6d9d4/metadata/metadata.go), :428–459 | FULL_TEXT_READ expiry/link API fragment. IsExpired использует `referenceTime.After(Expires)`. Для trace выбран Expires **строго раньше** RefTime; equality boundary не используется как доказательство |
| U6 | [go.mod](https://github.com/theupdateframework/go-tuf/blob/f5edbde31e5507f46db2069402dc38903fe6d9d4/go.mod), [LICENSE](https://github.com/theupdateframework/go-tuf/blob/f5edbde31e5507f46db2069402dc38903fe6d9d4/LICENSE) | FULL_TEXT_READ. Go≥1.25.0; Apache-2.0. Это license/minimum toolchain, не runtime correctness |

Fresh raw U1/U2/U3/U6 побайтно совпали с historical R05-001 captures на том же
commit. U1 отдельно получен через raw tag path `v2.4.2`; равен immutable U1
SHA256 **2adcfa54cb1a578930393849b478bbb4a158e2f4eefe93bf8e0582edff5343b4**.
GitHub API fresh tag lookup HTTP403, поэтому новую API tag attestation не заявляем.
Имеющийся historical `R05-001/upstream/go-tuf-tag.json` был прочитан и показывает
тот же tag→commit, но это старый receipt. Binary/Go module-cache fetch/build не
проводились; remote source captures не выдаются за executable dependency closure.

### Build/API условия

В U1/U2 критические файлы не gated платформенным build-tag. `Updater.New`
при включённом cache работает с directory/file API; `UnsafeSetRefTime` самим
source объявлен test-only. Actual Windows caller использует cfg.Fetcher=bounded
packageFetcher, isolated workspace, UnsafeLocalMode=false/DisableLocalCache=false.
Source чтение не является разрешением выполнить constructor/Refresh.

## 4. Current project source и реальная граница поиска

`SOURCE_AVAILABLE` для нижеследующего списка: это текущий checkout
**D:\SNOWDEN-SYSTEM-V3**, не присланная документационная Obsidian-копия.
Исходники доступны и прочитаны непосредственно; выполняющие операции программы
не запускались. Невозможно из доступности source вывести deployed correctness.

| Input | Exact caller / прочитанный scope |
|---|---|
| windows/go.mod | :3 go1.26, :13 go-tuf2.4.2. Полный dependency lock не дампился |
| windows/backend/delivery/client.go | packageFetcher:21–62; Prepare→PrepareArtifacts:68–218; finalPolicy:221–270 |
| windows/backend/delivery/model.go | State/cache bounds:54–155; Progress accessors:382–421; package/request shape навигация |
| windows/backend/delivery/policy.go | rootPolicy69–101; guardMetadataShape169–225; metadataReferences231–241; Enroll245–261 |
| windows/backend/delivery/retained_state.go | весь файл16–117: current pointer/byte binding, reset/cache, complete proof checks |
| windows/backend/delivery/retained_inputs.go | весь файл: matchesMetadata17–23; boundMetadata27–55; retainedRoot61–99 |
| windows/backend/delivery/retained_policy.go | весь файл: timestamp/snapshot continuity, не final expiry admission |
| windows/update_delivery.go | prepareVerifiedUpdate36–135; read/clock/SDK/progress/retain/error/proof; source final proof path навигация |
| windows/updateapp.go | ApplyUpdate156–200: retain=true call177, error до применения payload |
| windows/delivery_state.go | весь файл: retainDeliveryProgress41–119; source/binding/anchor/time/vault/fence/fixed-filetxn |
| windows/onboarding_import.go | :19–23 importCommitFn/importTargets, root enrollment boundary навигация; прочие import implementation не audited |
| windows/backend/metadata/delivery.go | весь файл: CheckDeliveryBinding16–29, CheckDeliveryProof34–51 |
| windows/backend/metadata/trust_policy.go | validate53–77; Decode/Encode/purpose identity по relevant scope; no generic final TUF expiry admission in shape gate |
| windows/backend/metadata/clock.go | :15–109: local wall/steady observation and floor, не authenticated UTC |
| windows/backend/secretvault/identity.go | :128–227: PrepareImportSnapshot(nil)/WithPreparedSnapshot owner+same-Manager source fence. Прочитан только source; реальный vault/keys не читались |
| windows/backend/filetxn/filetxn.go | весь source: Commit67–136, Recover139–208, validate210–235, writeSync261–268, atomicWrite271–285 |

Три source manifests `local-inputs-start.json`, `local-inputs-extra.json`,
`local-inputs-followup.json`:17 selected rows =16 product files+assignment doc.
Copy/before/after comparison — адресный последовательный capture, не атомарный
snapshot repository. Assignment doc изменился добавлением FUP-7-01; исходный
revision2 capture сохранён, updated follow-up copy сохранён отдельно.
Финальный drift seal записывается в `final-review.json`; его отсутствие не PASS.

**Не проверено:** user files/current saved checkpoints, kernel/FS/TPM/Keystore
behavior, whole Android SDK/AAR, все product callers/stores, signer provenance
для symbolic T1000, successful retention на ОС, фактическое использование candidate.

## 5. Baseline и follow-up (контекст, не доказательства реализации)

- R05-001 REPORT§§3–4: source mutation/permitted progress и role-reset; old
  cache-only описание не соответствует current retained_state inputs.
- R05-002 REPORT§9 и handoff: durable cut/unknown commit/per-return versus
  per-step; не наследуется обещание конкретной storage реализации.
- R05-005§§2–4,6–8: as-of/status/time ограничения; R05-006/007: authority/mapping
  и offline lineage. Они READY, не ACCEPTED в этом пакете.
- R05-008§2: «fresh consent + server floor» — слишком универсальная формула;
  уточнена independent surviving history/time/authority contract в REPORT-010.
  Native/backup claims R05-008 не повторяются и не подтверждаются этим исследованием.
- Уточнение end-to-end границ приведено в REPORT-010 §2.5; само задание
  не является доказательством реализации.

## 6. Failed retrieval / исключённые заявления

| Источник / попытка | Итог |
|---|---|
| WebFetch TUF1.0.36 | FETCH_FAILED timeout; позднее verified curl получил HTML, использован именно он |
| GitHub API ref tags/v2.4.2 | FETCH_FAILED HTTP403; historical tag JSON обозначен как historical; actual tag raw отдельно сравнен с commit |
| WebFetch RFC9162 HTML | FETCH_FAILED timeout; позднее RFC Editor TXT получен и relevant sections прочитаны |
| USENIX ATC17 Kuppusamy paper page | FETCH_FAILED timeout. Full paper/abstract **не прочитан**, paper finding/библиографические подробности не перенесены; ссылка внутри TUF лишь LINK_ONLY |

TLS verification не отключалась. Никаких исследований через unsigned зеркало
или guessed paper metadata. Все исполнение/fixtures/SDK/crypto/native/live/
admin/device/restore cases — **NOT_RUN**.

## 7. FUP-7-02 / R05-010-D — критерий timestamp retention

Source-reading партия: **27 октября 2026, 14:46–14:49Z**.
Нового внешнего fetch не было: переиспользованы проверенные immutable captures
на том же commit. Свежесть current inputs отдельно отражена в
`fup-7-02/review.json`; capture не объявляется remote-latest кодом.

### Точно прочитанное для D

| Источник / статус | Область и что он даёт |
|---|---|
| U1, go-tuf2.4.2 immutable trustedmetadata.go, FULL_TEXT_READ relevant functions | :27–55 New/public state; :61–94 root update; :97–168 UpdateTimestamp/checkFinalTimestamp. Assignment151 после type/signature/version/reference; единственный post-assignment final check — expiry. Timestamp/RefTime публичны |
| U2, тот же updater.go, FULL_TEXT_READ relevant functions | New77–104; onlineRefresh130–148; loadTimestamp307–351; persistence591–644; loadLocalMetadata669–672; GetTrustedMetadataSet680–682; UnsafeSetRefTime687–689. loadTimestamp/checkFinalTimestamp/persistMetadata непубличны; нового per-step receipt в этих API нет |
| U3 config.go, FULL_TEXT_READ constructor/config boundary | Fetcher/cache/unsafe settings, без accepted-step callback. Cache-disabled persistence no-op не receipt |
| U4 errors.go, FULL_TEXT_READ28–127 | ErrExpiredMetadata и ErrEqualVersionNumber входят в ErrRepository; local invalid/expired cache может оставить pointer перед remote attempt |
| U5 metadata.go, FULL_TEXT_READ428–447 | IsExpired: referenceTime.After(Expires); D выбирает strict Expires.After(RefTime), equality не используется как fresh acceptance |
| S1/S2 TUF§5.4, READ ранее, critical source сличен для D | New expired timestamp discard; new same-version timestamp normal discard не новый acceptance; persistence только после final acceptance |
| windows/backend/delivery/client.go:118–173, FULL_TEXT_READ relevant path | Isolated workspace/config/current fetcher; Refresh→state inspection→retention before error. RefTime не подменяется production UnsafeSetRefTime |
| retained_inputs.go:17–55; retained_state.go:16–79; retained_policy.go:13–28, FULL_TEXT_READ | Original byte binding не receipt; previous/context/continuity; reset && identical previous timestamp приводит к continuation, не old-floor resurrection |
| workspace.go:31–60 и workspace_windows.go:38–114,145–179, FULL_TEXT_READ | Ограниченное custody/seed before SDK call; pre-Refresh read equality; это не proof later SDK internal read/persist |
| cache_reader.go, FULL_TEXT_READ | Existing bounded cache read API, не receipt SDK verification |
| [Рецензия руководителя](../reviews/NINE-2026-10-27-R05-010-A.md), FULL_TEXT_READ | Принят только timestamp→returned-Progress conditional A; whole A/B/C и новая D не ACCEPTED |

Bounded negative search: список public methods/полей в U1/U2/U3 и current
caller (не все packages/extensions репозитория). Не найден публичный outcome
per-stage receipt/callback, который current caller может прямо получить.
Лог `Updated timestamp` стоит до checkFinalTimestamp — не принятый receipt.
Полный source spelling/порядок приведён в REPORT§7; нельзя переносить вывод на
snapshot (там есть дополнительный final version predicate).

### Provenance и файлы D

`fup-7-02/before/` сохраняет полные ранее сданные REPORT/SOURCES/handoff:
новое дополнение append-only, hashes/prefix preservation в final review D.
`fup-7-02/inputs-start.json` —8 selected product-source snapshots и SHA256
ранее полученных5 upstream files. Fresh SDK/network/latest attestation не
объявляется. Новые D1–D6 **NOT_RUN**, нет fixtures/SDK/state/OS выполнения.

Собственное логическое основание D: точный source-order плюс observed
SDK-owned state, known prior acceptance/context и strict final predicate.
Это **derived certificate**, не публичный upstream receipt, не криптография,
не допуск артефакта и не platform durability proof.

### Закрытие после cutoff — 28 октября2026

Итоговый `fup-7-02/review.json` оформлен **28 октября 2026 после исследовательского
среза**. Сдача D и source-drift проверка до исходного cutoff **не подтверждены**;
связанные excerpts о snapshot стадиях не включены в D.

`fup-7-02/review.json` фиксирует только post-cutoff own output hashes, local-file
links и сохранность baseline prefixes. Он **не** проверяет нынешние product
source bytes и не заменяет original input snapshots либо signature/runtime
proof. Новых источников/SDK/fixtures/реальных states нет; D1–D6 NOT_RUN.
