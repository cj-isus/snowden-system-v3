# TH-03 — подготовка и раздельное подписание TUF metadata

2026-10-22, V2-124. **Реализовано в исходниках, не проверено.** Tests не
добавлялись/менялись/запускались; build/vet/harness/module tooling/native/live и
новые CLI не выполнялись. Ключи/root/certificates/vault владельца не читались
и не выпускались. Gofmt/чтение кода не подтверждают компиляцию или поведение.

Входы: [issuer package](TH-03-ISSUER.md), [ADR-001](../decisions/001-tuf-delivery.md),
[profile epochs](TH-03-EPOCHS.md). Все девять этапов PLAN §5.3 открыты в своих gates.

## Компонент и повторное использование

`windows/tools/internal/repository` вызывается подкомандой `repository` обоих
существующих CLI: metadatatool/updatetool. Новый daemon/service не добавляется.
Go-tuf2.4.2 Metadata API создаёт/подписывает/проверяет роли; Sigstore signer/verifier
и cjson — те же зависимости и версии, которые использует выбранный SDK.
Client acceptance остаётся полным Updater workflow, а не publisher shape check.

`delivery.CheckPublicationMetadata` разделяет product shape/bounds с authoring;
unsigned draft разрешён только здесь. Runtime `guardMetadata` по-прежнему требует
подписи; draft validator не выдаёт private target proof/права установки.
`Request.Validate` разделяет purpose/path/size policy. Bounded JSON/path/read и
new-output helpers общие с delivery-pack через `publisherfiles`.

Основание: [официальный пример v2.4.2](https://github.com/theupdateframework/go-tuf/blob/v2.4.2/examples/repository/basic_repository.go)
и закреплённые `metadata/{metadata,types,keys}.go`; source API inspected, CLI не
запускались. Использованы API/паттерны, example не исполнялся. В отличие от примера,
команда не собирает полный threshold набор приватных ключей в одном процессе.

## Операции — описание будущих вызовов

```text
metadatatool repository keygen new-public.json new-private.key
metadatatool repository draft plan.json PARENT_SHA_OR_bootstrap PREVIOUS_SHA_OR_first new-draft.json
metadatatool repository sign root draft.json APPROVED_DRAFT_SHA @own-private.key new-partial.json
metadatatool repository assemble assembly.json APPROVED_DRAFT_SHA PARENT_SHA_OR_bootstrap new-signed.json
```

Эти команды агентом не выполнялись. Все outputs — новые файлы, существующий
destination не заменяется. Parent directory должен существовать. Не менять
ключи/сроки/права установленного клиента только из-за наличия новой команды.

### 1. Отдельное хранение ключей

Один keygen создаёт ровно одну Ed25519 пару. Public file — SDK TUF key object;
`keyid` вычисляет SDK. Private hex64 bytes пишется в отдельный new-only file0600
до public output. При отказе public записи уже сохранённый private file остаётся,
ошибка сообщает об этом; rollback не стирает ключ. Sign принимает только explicit
`@file`, bounded read1KiB и согласованные seed/public halves, не raw key в argv.

Public plan содержит только 32-byte lowercase public hex. Все хранители подписывают
один independently approved unsigned draft на своих устройствах; assembly читает
только публичные partial files. Hash key file, SDK keyid, unsigned draft hash и
final signed root hash имеют разные смыслы. OOB enrollment клиента требует SHA256
именно **final root bytes**, а не public key file/draft/ключевой fingerprint.

Один API/число разных ключей не доказывают реальную независимость хранителей.
0600/clear buffers не доказательство Windows ACL, HSM/DPAPI, secure erase или
защиты от локального admin; автоматической настройки custody устройства нет.

### 2. Девять обязательных свойств public draft plan

| Поле | Контракт |
|---|---|
| `schema` | 1 |
| `role` | root, targets, profiles, updates, snapshot, timestamp |
| `version` | positive int64; новый выпуск ровно previous+1, без overflow |
| `expires` | явный UTC expiry, должен быть в будущем локального времени |
| `parent` | relative public JSON: root для top roles, targets для delegates; empty только initial root1 |
| `previous` | exact previous signed role; empty только version1 с explicit `first` |
| `roles` | root: ровно root/targets/snapshot/timestamp; targets: ровно profiles/updates; прочие `{}` |
| `targets` | target path → relative artifact JSON; root/snapshot/timestamp `{}` |
| `metadata` | snapshot: targets.json/profiles.json/updates.json → finalized files; timestamp: snapshot.json; прочие `{}` |

Role definition — ровно `threshold` и `public` (массив distinct canonical public
hex, max32). Product policy сохраняет root/targets threshold≥2, update delegation≥2,
отдельные profiles/updates, disjoint offline/online root roles и terminating flat
delegations. Root consistent_snapshot=true. Top-level artifact только optional
`trust/profile-epoch.json` с валидным typed grant; чужих authority slots нет.

Plan≤32KiB, exact fields/duplicate/null/alias rejection; relative JSON paths по
прежнему explicit/ASCII/≤240 bytes/12 components, без recursive discovery/URL.
Metadata≤512KiB, max256 artifacts в role и суммарный artifact-read budget8MiB;
per-purpose profile256KiB/update64KiB/grant16KiB bounds сохраняются. Эти ceilings
не замер CPU/RSS/FS syscall времени и не жёсткий OS deadline.

Parent/previous hashes передаются отдельно от плана. Parent root self-threshold
проверяется SDK; approved targets parent является отдельным точным public input,
не самодоказанной цепочкой до установленного root. Полная chain/purpose/freshness/
hash проверка всё равно обязательна в delivery-pack и на клиенте.
Previous должен быть finalized/подписан и проверяем под выбранным parent; при
delegated/top-role key rotation нужен overlap, затем вывод старых keys отдельным
выпуском. Publisher не знает floors каждого установленного клиента.

### 3. Общая цепочка

1. Подготовить/подписать application profile или update manifest прежними CLI.
2. Выпустить root1; либо next root с точным predecessor и version+1. Новый root
   требует одновременно прежний root threshold и собственный новый threshold.
   Истёкший старый root может авторизовать next root; новый должен быть fresh.
3. Targets authoring задаёт ровно две delegations и optional offline epoch grant.
4. Profiles/updates связывают target paths с SHA256/length точных artifact bytes
   через SDK TargetFile.FromBytes, без повторной сериализации файлов.
5. Snapshot связывает finalized targets/profiles/updates по version/length/SHA256;
   signatures каждого linked role проверяются SDK через соответствующий parent.
6. Timestamp связывает finalized snapshot. Срок child/parent проверяется при
   подготовке. Каждый custodian независимо подписывает точный approved draft.
7. Assembly сравнивает signed canonical payload, требует ровно одну подпись от
   каждого partial, отвергает чужой/повторный ID, неверную подпись/другой документ;
   затем SDK проверяет полный role threshold. Порядок signatures сортируется.
8. Уже существующий delivery-pack выполняет полный SDK workflow и application
   signature AND gate и публикует публичный client package новым файлом.

Ни sign, ни assembly одного role не означают полной проверки repository или
client checkpoint/credentials/EXE/VPN. JSON report явно различает
`role_threshold_checked` и `requires_repository_acceptance=true`.

## Запись и оставшаяся работа

PublishNewChecked: stage→Sync→Close→fresh clock/expiry/source-value gate→new-only
hard link. Draft/sign/assembly повторяют срок собственного role и доступного
parent; обычный delivery-pack — private artifact/grant proof + grant TTL и context
после staging. Source bytes owned; actual metadata validity целого выпуска
оценивается полным SDK. Существующие destinations и клиентские файлы не меняются.

V2-125 добавляет [owned release candidate/inventory/layout/manifest](TH-03-RELEASE.md)
в source, unverified. V2-126/127 [promotion](TH-03-PROMOTION.md) добавляет Windows
store/floors/lease, prefix-only + four-target journals, explicit local promote/
policy authorization/recover и после staging source/time gates. Нет remote
upload/deploy, private-key backups/restore automation или измеренной reproducibility.
Не публиковать timestamp раньше готовности всех versioned metadata/target files.
Сборка/подписи каждого role не заменяют coherent publication/rollback gates.

Новые commands не запускались. V2-128 добавляет [cooperating reader/status/alias
repair и loopback HTTP source](TH-03-SERVING.md); network/CLI/native не запускались.
Client selection/download/export/recovery UX, архив roots/old-client compatibility,
publisher time/storage lifecycle/generated bindings/rollout остаются. Module graph/SBOM/licenses,
crypto/interoperability/old-client, Windows ACL/reparse/interprocess/FS links/
crash/power-loss/clock/lifecycle/installer/release/comparative gates открыты.
Реальные custody/подписание и deployment требуют отдельных действий владельца.
