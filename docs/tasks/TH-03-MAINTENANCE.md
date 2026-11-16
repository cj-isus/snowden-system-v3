# TH-03 — approved maintenance и потоковый publisher backup

Restore source продолжен в [V2-134](TH-03-RESTORE.md): independent archive/state/
anchor/policy approvals, new/resume и repair с сохранением target authority.
Ниже сохранён срез V2-133; native/DR/restore execution acceptance ещё открыта.

2026-10-23, V2-133. **Source-only/unverified.** Тесты не создавались/менялись/
запускались; compile/build/vet/check/module/gen/CLI/HTTP/native/live не выполнялись.
Реальный store, owner keys/cert/vault/config не читались; фактической очистки,
архивации или удаления agent не выполнял. Только Windows source/docs, Android и
обвязка сохранены. Основа: [retention](TH-03-RETENTION.md), [root package](TH-03-ROOT-PACKAGE.md).

## Один inventory от существующего private checkpoint

`publisher_inventory.go` заново выводит protected set из exact private state,
retained current/history heads и root index. Для каждого accepted head читается
полный known-hash candidate/inventory; отдельно проверяются все indexed roots.
Файлы из обхода public directory не становятся trust/backup authority.

Protected set включает exact state/aliases, current и до16 retained archives,
release frames, client packages, version/hash shared paths, все128 indexed roots.
Expired retained head продолжает защищаться: архивирование/ремонт не продлевают
его expiry. Дубликат path с другими bytes/case — отказ. Sorted path/length/SHA
inventory получает общий digest; затем повторяется source fence.

Поддерживается enrolled checkpoint без Published: state/root prefix сохраняются,
missing authority/journal/corrupt source не превращаются в пустое хранилище.

## План и explicit применение — разные source operations

Source dispatcher существующего CLI, **не выполненные команды**:

```text
repository maintenance-plan <approved-state-sha256> <publisher-directory> <new-plan.json>
repository maintenance-apply <plan.json> <approved-plan-sha256> <publisher-directory>
repository backup <approved-state-sha256> <publisher-directory> <new-backup.zip>
```

Plan требует independent exact state approval. Он записывается new-only вне store,
содержит state SHA, hash canonical store path, protected inventory SHA, created/
expires и exact file path/length/SHA + directory list. Lifetime≤10min, до512 files,
до512 directories, до64MiB data за batch, JSON≤1MiB. Это предложение для просмотра,
сам `maintenance-plan` ничего не удаляет. Store path hash — не physical volume ID.

Apply требует independent SHA всего plan и снова проверяет state/path/protected
inventory. Strict parser отвергает unknown/duplicate/null fields, case aliases,
unsorted/oversized records, неканонические paths/hashes и нарушенный deadline.
ObservedClock/floor, context и source approval не заменяются фактом наличия plan.
Clock checkpoint обычного serving может сделать plan stale; нужен новый plan,
автоматического игнорирования нового состояния или остановки сервера нет.

## Разрешённое удаление

Единственный namespace — exact canonical `public/releases/<retired-id>/...`.
Retained current/archive ID защищает **весь** каталог, даже лишние файлы в нём.
Private paths, shared `public/metadata`/`targets`, root/aliases и все прочие namespaces
не допускаются. В retired release разрешены известные metadata/targets/clients/
candidates JSON и exact approved `.publisher-stage-*` regular leftovers.

При применении каждого file сверяются размер/hash, открытая regular identity и
повторный path/identity gate перед `os.Remove`. Нет recursive remove. Empty orphan
files тоже имеют exact empty SHA. Directories обходятся children-first и удаляются
только пустыми; nonempty остаются, включая объекты вне текущего batch. Отсутствующий
объект учитывается как already_absent, не как новые освобождённые bytes.

Source/input fence повторяется каждые16 объектов и в конце; journal/time/context
проверяются также перед изменением. Cooperating writer lease удерживается на весь
apply. Protected inventory и state не переписываются. HTTP reader в существующем
коде копирует owned bytes до освобождения lease и отправляет их из памяти; cleanup
не добавляет чтение диска в response lifetime. Native concurrency не проверена.

Это не all-or-nothing cleanup: при error/cancel/crash ранее удалённые retired
objects не восстанавливаются. Error явно сообщает partial completion; тот же plan
можно повторить при неизменных source/protection/time или построить новый. Authority
journal не расширяется deletion target-ами. Logical removed bytes — не доказанная
физически освобождённая ёмкость.

## Обход и budgets

`publisher_walk.go` переиспользован storage admission и maintenance: ReadDir128,
depth32, context между entries, regular/identity/reparse gates. Admission сохраняет
прежние256MiB/8192 files/2048 dirs; maintenance inspection допускает2× эти пределы,
чтобы bounded cleanup был возможен после превышения обычного admission. Larger/
unsupported/corrupt source отказывается, не становится permission для слепого sweep.

`publisherfiles.ReadSnapshot` расширяет существующий reader для owned file identity
и optional empty orphan; обычный Read по-прежнему требует nonempty. После чтения
проверяются размер/identity и close error. Existing output path переиспользует
synced new-only link, теперь также с bounded streaming producer.

## Backup export

Точный state approval и тот же protected inventory задают архив. Попадают state,
имеющиеся aliases, принятые release frames/packages/shared paths и indexed roots.
Signing keys, vault, writer.lock, journal и discovered/unaccepted leftovers не
копируются. Исходный external application policy JSON нужен отдельно; его SHA
сохраняется, report оставляет `external_role_policy_required=true`.

ZIP Store без сжатия пишется потоком в private temporary file вне store, затем
Sync/Close/final source+time+context gate и new-only link. В каждом entry только
owned relative path и exact accepted bytes; entry metadata фиксирована. Общий
manifest содержит source state/authority/store/inventory hashes и file records.
Рамки: data≤288MiB, manifest≤4MiB, archive output≤320MiB; SHA всего потока идёт в
report. Одновременно не собирается весь ZIP в []byte, но hard RSS/GC/CPU/FS duration
из этого не следует. Context90s кооперативен, включая повторные source gates.

Архив не зашифрован и содержит private authority checkpoint и topology принятых
public профилей. File mode0600 не доказательство Windows ACL. Backup SHA/inventory
не self-authorize восстановление; независимое approval и restore protocol нужны
отдельно. Historical expired state можно копировать, это не readiness/renewal.
Result разделяет export, independent policy, future restore acceptance и keys.

## Открытые границы полного плана

Restore verifier/staging/activation/rollback пока не подключены. Нужна отдельная
explicit процедура с independent archive/policy approvals, проверкой origin,
retained floors и защитой existing target; backup file не новый trust bootstrap.
Не удаляются общие старые объекты/неизвестные namespaces; полный storage migration
при их заполнении остаётся отдельным решением. Копирование signing keys и реальный
внешний DR/restore exercise также не выполнены.

Native ACL/reparse/hardlink/rename/open-handle deletion, interprocess/hostile writer,
crash/reboot/power-loss, physical quota, resource bounds, CLI/SDK/crypto/backup
roundtrip, old-client/bindings/module/rollout/release/comparative acceptance открыты.
Global signed revocation/private pin-history portability, TH-01/02/04…08 и все
прежние P0 safety/lifecycle gates сохраняются. Тест-код/запуск пока запрещены.
