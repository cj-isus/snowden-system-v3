# R28-Z07 — источники, версия и границы чтения

Срез и дата чтения: **28 октября 2026**. Source availability:
**SOURCE_AVAILABLE** в D:/SNOWDEN-SYSTEM-V3, не docs-only Obsidian-копия.
«Current source» ниже относится к этому историческому срезу.

## 1. Scope и шкала чтения

`SECTIONS_READ` означает прочитанные названные функции/параграфы, а не весь
большой файл/спецификацию. `FULL_TEXT_READ` — целый небольшой файл или лицензия.
Тестовый исходник только читался; его expectations не execution/evidence PASS.
При source/caller выводе фиксируется current manifest и точный commit.

Captures только `.artifacts/research/r28-contracts/Z07/`:
`upstream/`, `local/`, `inputs-start.json`, `inputs-extra.json`, review records.
SHA256 — provenance bytes, не криптографическая аттестация origin, не deployed
EXE/SDK/native proof. Ни SDK, ни генератор test keys, ни реальные состояния,
vault/профили/пользовательские логи не исполнялись/не читались.

## 2. Нормативные источники

| ID | Организация / документ / дата | Прочитанная область и URL | Depth / дата чтения |
|---|---|---|---|
| S1 | The Update Framework Authors, **TUF Specification1.0.37**; Last modified **2026-10-08** (header документа) | [Official version](https://theupdateframework.github.io/specification/v1.0.37/), §§4.2.1/4.3 keyID/signature/threshold,5.1–5.6 workflow,6.1 key migration,2.3 POUF | SECTIONS_READ,28.10.2026 ~08:56–09:21Z. Root persistence/final expiry/reset ordering; new timestamp acceptance; `keys have been rotated` не exhaustive predicate overlap/threshold-only |
| S2 | The Update Framework Authors, **TUF1.0.19**; Last modified **2021-05-27** | [Historical version](https://theupdateframework.github.io/specification/v1.0.19/),5.3.11 | SECTIONS_READ targeted paragraph28.10~09:08Z; historical boundary legacy source. Не current normative contract |

Raw HTML и преобразованный readable text S1 сохранены в own artifacts. Выводы
по fetched primary paragraphs, а не по snippets. Ссылка на Mercury внутри S1 —
**LINK_ONLY** в этой партии, paper findings/metadata не перенесены.
Официальный русский перевод **этих exact sections** не получен; не заявляется,
что смена locale в URL создаёт прочитанный перевод. Текст исследования написан
по-русски на основании exact primary en-US spec/source; популяризацию не
подставляю вместо проверенного нормативного документа.

## 3. Reference 1 — go-tuf2.4.2

Организация: The Update Framework Authors. Version **v2.4.2**, commit
**f5edbde31e5507f46db2069402dc38903fe6d9d4**.
Current `windows/go.mod:13` закрепляет v2.4.2; project Go1.26.

[Запись происхождения модуля](https://proxy.golang.org/github.com/theupdateframework/go-tuf/v2/@v/v2.4.2.info)
получена 28.10.2026 около 08:56Z: Origin.URL/ref/hash и Time
2026-05-19T08:44:09Z. Это запись публикации, не идентичность локального бинарного файла.

Все source URLs ниже addressed immutable commit, raw bytes captured own upstream.

| ID | Exact source | Read-depth / decisive functions |
|---|---|---|
| G1 | [trustedmetadata.go](https://github.com/theupdateframework/go-tuf/blob/f5edbde31e5507f46db2069402dc38903fe6d9d4/metadata/trustedmetadata/trustedmetadata.go) | SECTIONS_READ58–168,179–218: UpdateRoot old+new root threshold/N+1/assignment; UpdateTimestamp signature/root/version/reference/final-expiry; cache-not-yet-loaded comment; read28.10~08:58–09:08Z |
| G2 | [updater.go](https://github.com/theupdateframework/go-tuf/blob/f5edbde31e5507f46db2069402dc38903fe6d9d4/metadata/updater/updater.go) | SECTIONS_READ New/onlineRefresh,loadRoot487–521,loadTimestamp307–351,loadSnapshot355–413,persistMetadata591–644; signature rejection versus full workflow stage; read28.10~08:58–09:08Z, addressed recheck09:31Z |
| G3 | [metadata.go](https://github.com/theupdateframework/go-tuf/blob/f5edbde31e5507f46db2069402dc38903fe6d9d4/metadata/metadata.go) | SECTIONS_READ246–423 VerifyDelegate,428–447 expiry, targeted FromBytes/duplicate-signature guard navigation. New-role signatures selected by keyID then actual material fingerprint dedup; not naive signature-array count |
| G4 | [keys.go](https://github.com/theupdateframework/go-tuf/blob/f5edbde31e5507f46db2069402dc38903fe6d9d4/metadata/keys.go) | SECTIONS_READ46–133 ToPublicKey/KeyFromPublicKey/ID. Key.ID = canonical-key SHA256; arbitrary alias not independent key |
| G5 | [go.mod](https://github.com/theupdateframework/go-tuf/blob/f5edbde31e5507f46db2069402dc38903fe6d9d4/go.mod), [LICENSE](https://github.com/theupdateframework/go-tuf/blob/f5edbde31e5507f46db2069402dc38903fe6d9d4/LICENSE) | SECTIONS_READ Go≥1.25.0/manifest; FULL_TEXT_READ Apache-2.0 LICENSE on28.10~09:31Z. Не transitive licences audit |

Нет нового SDK fetch/install/build. Публичный source retrieval не разрешение
вызвать Updater/VerifyDelegate; от source availability не выводится runtime.
В названных root→timestamp functions отдельная cache deletion5.3.11 не найдена;
это bounded finding по workflow, не отсутствие всех возможных library hooks.

## 4. Reference 2 — python-tuf7.0.1 и paired-key recipe

Организация: The Update Framework Authors. Version **v7.0.1**, annotated tag
**c162c6113b90e26f6bffc81c59310384d68dcdd2**, commit
**49eb1da48c58cf0cbb0f2e490f7ad39fbd8c1f15**.
Fresh GitHub [ref](https://api.github.com/repos/theupdateframework/python-tuf/git/ref/tags/v7.0.1)
и [tag object](https://api.github.com/repos/theupdateframework/python-tuf/git/tags/c162c6113b90e26f6bffc81c59310384d68dcdd2)
получены 28.10~09:00Z: target commit и tagger date
**2026-09-01T12:02:05Z**. GitHub metadata не собственная crypto-verification tag.

| ID | Exact pinned file | Прочитанное / depth |
|---|---|---|
| P1 | [updater.py](https://github.com/theupdateframework/python-tuf/blob/49eb1da48c58cf0cbb0f2e490f7ad39fbd8c1f15/tuf/ngclient/updater.py) | SECTIONS_READ150–177 refresh,325–467 persistence/root/cache loading. Root history/symlink path; old metadata re-verification, no universal cache reset in named stage |
| P2 | [trusted_metadata_set.py](https://github.com/theupdateframework/python-tuf/blob/49eb1da48c58cf0cbb0f2e490f7ad39fbd8c1f15/tuf/ngclient/_internal/trusted_metadata_set.py) | SECTIONS_READ178–286 root/timestamp;288–350 snapshot start;454–493 root/load verification; no application authority from intermediate state |
| P3 | [test_updater_top_level_update.py](https://github.com/theupdateframework/python-tuf/blob/49eb1da48c58cf0cbb0f2e490f7ad39fbd8c1f15/tests/test_updater_top_level_update.py) | SECTIONS_READ73–113 helper creates new Updater;410–435 timestamp recovery;491–532 snapshot recovery/expected result. Source tests **NOT_RUN**, не independent replication |
| P4 | [repository_simulator.py](https://github.com/theupdateframework/python-tuf/blob/49eb1da48c58cf0cbb0f2e490f7ad39fbd8c1f15/tests/repository_simulator.py) | SECTIONS_READ161–196 rotate_keys/publish_root;253–318 fetch_metadata/update_timestamp. Clears old role keyIDs/signers; generates new threshold set; fetch clears signatures and signs only current keys |
| P5 | [tuf/api/_payload.py](https://github.com/theupdateframework/python-tuf/blob/49eb1da48c58cf0cbb0f2e490f7ad39fbd8c1f15/tuf/api/_payload.py), [metadata.py](https://github.com/theupdateframework/python-tuf/blob/49eb1da48c58cf0cbb0f2e490f7ad39fbd8c1f15/tuf/api/metadata.py) | SECTIONS_READ payload429–504 VerificationResult/verify_delegate; targeted root double-result API; metadata parse/delegate forwarding. Distinct correct keys в модели, no Go/Python alias-equivalence assertion |
| P6 | [pyproject.toml](https://github.com/theupdateframework/python-tuf/blob/49eb1da48c58cf0cbb0f2e490f7ad39fbd8c1f15/pyproject.toml), [LICENSE](https://github.com/theupdateframework/python-tuf/blob/49eb1da48c58cf0cbb0f2e490f7ad39fbd8c1f15/LICENSE), [LICENSE-MIT](https://github.com/theupdateframework/python-tuf/blob/49eb1da48c58cf0cbb0f2e490f7ad39fbd8c1f15/LICENSE-MIT) | SECTIONS_READ pyproject1–56 Python≥3.10,Apache-2.0 OR MIT; FULL_TEXT_READ licence files. No Python runtime/deps change |

Source/helper reading28.10~08:56–09:21Z. CryptoSigner.generate_ed25519 в test
только читался, не исполнялся; никаких новых ключей или signed fixtures.
Full-disjoint recipe не general evidence по overlap/dual signatures/reused
material. Root/file persistence source не hardware durability.

## 5. Opposing historical reference — legacy go-tuf0.7.0

Exact commit **6ad7fe593e4042db3544c4b0fedbe66bac371c42**:
[client/client.go](https://github.com/theupdateframework/go-tuf/blob/6ad7fe593e4042db3544c4b0fedbe66bac371c42/client/client.go),
[LICENSE](https://github.com/theupdateframework/go-tuf/blob/6ad7fe593e4042db3544c4b0fedbe66bac371c42/LICENSE).
Version0.7.0 label из named baseline; fresh legacy tag lookup не выполнялся,
поэтому новый version→commit receipt для legacy **не** заявляется.

SECTIONS_READ219–36528.10~09:00Z: root/cache/final expiry, predicate
`countDeleted(oldIDs,newIDs) >= oldThreshold`; role table T-only→timestamp,
S→timestamp+snapshot, targets→snapshot+targets. Source explicitly cites old1.0.19
и ongoing PR e50151…; table не уравнена с современной 1.0.37. FULL_TEXT_READ
BSD-3-Clause licence. Два counterexamples REPORT§3.3 — manual set models,
не исполнение legacy или утверждение actual compromised keys.

## 6. Current source/caller и фактическая доступность

Ниже существующие public project source в current checkout. Минимальная цепочка
привязана к `inputs-start.json/inputs-extra.json`; whole tree не audited.

| File | Exact scope |
|---|---|
| windows/go.mod | Go1.26,:13 go-tuf2.4.2 |
| delivery/policy.go | :19–101 keyIdentity/roleKeys/rootPolicy; Ed25519 only; material map ignores threshold in output, validates it separately |
| delivery/retained_inputs.go | FULL_TEXT_READ17–99; original-bytes equality; all intermediate root material diff OR |
| delivery/retained_state.go | FULL_TEXT_READ16–117; reset clears both caches; exact old timestamp/snapshot→continuation; proof gates separate |
| delivery/retained_policy.go | FULL_TEXT_READ13–51: timestamp version/snapshot ref/signed content continuity even when old seed did not load in SDK |
| delivery/client.go | SECTIONS_READ118–218/finalPolicy221–270: isolated full updater/progress/error/rotation ceiling/target proof |
| delivery/model.go | SECTIONS_READ54–154: State/cache/schema/Digest; no explicit acceptance-context field, protocol may exclude ambiguous images |
| windows/delivery_state.go | SECTIONS_READ41–119: retainDeliveryProgress→WithPreparedTransaction→existing fixed files/source fence |
| windows/update_delivery.go | SECTIONS_READ65–124: possible retention before deliveryErr; no new update proof on rejected tail |
| windows/updateapp.go | Relevant ApplyUpdate early-error consumer selected; not whole update/native code |
| filetxn/filetxn.go | SECTIONS_READ73–185/188–213: journal-domain/native writer/target pinning/BeforeCommit/checkTargets/barrier/retirement. Actual IO/durability not executed |
| secretvault/identity.go | SECTIONS_READ138–190: WithPreparedTransaction final same-source fence after staging. Real vault/credentials not read |
| filetxn/serialize.go | Targeted ownership-navigation, not full concurrent writer audit |
| metadata/delivery.go | Existing binding/proof source-navigation, not full application policy audit |

Prefix `delivery/...` здесь `windows/backend/delivery/...`; аналогично
filetxn/secretvault/metadata относятся к windows/backend/. All links/captures
в own local index имеют exact full paths.

Selected source/rule captures16 files: before/copy/after SHA256 agreed at each
capture, mid16/16 unchanged09:12:47Z, pre-handoff16/16 unchanged09:19:09Z.
Later actual comparison в final review, не atomic repository snapshot.
На 09:37:40Z filetxn.go изменился; targeted diff09:38:19Z показал только
package comment о WithRead/bare loaders, Commit/Recover body unchanged.
Обновлённый capture `local/windows__backend__filetxn__filetxn-review.go` и
`drift-filetxn.json` сохраняются отдельно, старые bytes остаются baseline.
Implementation WithRead не audited; новая фраза комментария не runtime proof.
Особенно важное drift-baseline: current filetxn и WithPreparedTransaction
изменены относительно 27oct текста; старый native/same-Manager-only description
не использовался как current proof.

## 7. Связанные научные материалы и paired review

- `architecture/TRUST-CONTINUITY-AND-AGREEMENT.md`§§1–3 и
  `reviews/2026-10-27-R05-001-PROGRESS.md` — SECTIONS_READ baseline root/reset/
  h_pre/h_post/paired recipe; accepted S-only **не** расширен на все rotations.
- `05-TRUST-SAFETY/REPORT-010.md` targeted§§2.2–2.5,4.4,7, SOURCE/MODEL baseline,
  не reread всей истории, не automatic ACCEPTED новых claims.
- C05/HANDOFF и REPORT §4 — SECTIONS_READ: opposing atomic-image argument и
  compromised-quorum distinction. Сопоставление выводов — не primary evidence
  и не ACCEPTED; независимо полученные G1/G3 и предпосылки модели сохраняются.

## 8. Исключённые знания и ограничения

Не установлены реальное существование old dual-signed envelope/compromised keys,
согласованность всех current writers, native/kernel/FS durability/power loss,
actual successful timestamp retention/application admission, applied
ResetAllowed для every partial/threshold-only rotation, transitive licence/build
closure, general unknown envelope corpus. Нет network/status endpoints probes.

10future cases **NOT_RUN**. SDK/tests/fixtures/crypto/native/device/live execution отсутствует. Listed HTTPS source requests получены успешно; TLS verification не отключалась.
