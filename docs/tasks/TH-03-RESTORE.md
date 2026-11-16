# TH-03 — approved checkpoint restore и ремонт без замены target authority

2026-10-23, V2-134. **Source-only/unverified.** Тесты не создавались/менялись/
запускались; compile/build/vet/check/module/gen/CLI/HTTP/native/live не выполнялись.
Реальные archives/store/owner keys/cert/vault/config не читались и не восстанавливались.
Только Windows source/docs. Предыдущий срез — [backup/maintenance](TH-03-MAINTENANCE.md).

## Independent approvals до создания target

Source contract существующего dispatcher, **команда не выполнялась**:

```text
repository restore <backup.zip> <approved-backup-sha256> <application-policy.json> <approved-policy-sha256> <approved-archived-state-sha256> <approved-anchor-sha256> <new|resume|approved-existing-target-state-sha256> <publisher-directory>
```

Нужны SHA всего архива, exact private checkpoint внутри него, retained enrollment
anchor и отдельный scoped application policy JSON/SHA. Значения из самого архива
не заменяют эти approvals. Policy должна иметь existing schema2 scoped roles без
delivery cache; backup state/manifest authority и lineage должны совпасть.
Archive и policy находятся вне target store. Проверка precedes target creation.

Это восстановление ранее принятого checkpoint по independent owner approval,
не новое issuer artifact acceptance. Оно не доказывает происхождение root chain
до начала сохранённого root index и не обнаруживает более позднее **утраченное**
состояние, о котором больше нет независимых данных. Cold recovery требует
актуального independently retained state approval, а не выбора «любого старого ZIP».

## Bounded ZIP reader без generic extraction

`publisher_backup_read.go` держит opened archive identity, whole SHA и bounded
entry readers. До `archive/zip` parser проверяет EOCD22 и весь bounded central
directory: non-ZIP64/no-comment/single-disk, count≤16385, directory≤16MiB,
name≤400, extra≤128. Header counts/lengths должны точно покрывать central directory.
Предел archive320MiB; только Store entries, regular mode, допустимые flags,
nonempty equal compressed/uncompressed size. Overlapping bodies запрещены.

Каждый output path — portable relative allowlist: exact private/state, public
aliases и accepted release/metadata/targets namespaces. Нет symlink/absolute/
UNC/device/ADS/traversal/case alias/private lock/journal/vault paths. Bodies
ограничены declared size и прежним file cap, проходят CRC, exact length и SHA.
Reader не распаковывает ZIP в выбранную архивом файловую систему.

Проверены **исходники установленного Go archive/zip** для EOCD/layout/reader limits,
не выполнен запуск библиотеки или совместимости ZIP writer/reader. Другие ZIP
форматы намеренно не становятся supported backup format автоматически.

## Checkpoint-derived inventory

Manifest strict shape/record count/total data bounds повторяются; entry set должен
совпасть с manifest. Private state bytes должны иметь exact independently approved
SHA, проходить existing decoder/alias/root self-threshold/lineage checks.
ObservedClock принимает retained SDK/serving floors, rejects uncertain/future
claims. Historical expiry не продлевается и не мешает копии для восстановления.

`decodeAcceptedHead` и `checkpointInventory` выделены из прежних read/inventory
helpers. Через bounded object reader они выводят exact frames/files/shared paths
из **private** current/history heads, сверяют retained root versions/hashes и
последовательные dual thresholds. Derived file list и digest должны точно совпасть
с archive manifest; extra/missing/unaccepted file — отказ до target creation.
Обычные disk readers продолжают source fence и полный immutable read.

Cache/floors/history копируются как approved at-rest bytes. Не вызывается Unsafe
reference time, не подменяется source SDK state parsed incoming root-ом. Клиент
после restore продолжает собственный SDK/application/epoch/pins/history gate.

## New и resume

`new` требует atomically absent dedicated target folder: existing folder не
принимается даже если кажется пустой. Затем existing publisher layout/writer lease,
все4 state/current/root/timestamp отсутствуют. Точный `private/restore-intent.json`
фиксирует approved ZIP/state/policy/anchor и hash target path. Это staging receipt,
не authority для serving/нового signer.

До commit копируются только проверенные public objects new-only. State/aliases
ещё отсутствуют; HTTP reader не имеет принятого publication. Прежний source store
не трогается. Public logical admission переиспользуется по record lengths без
сборки всего архива в memory map; лимиты не отключаются для restore.

`resume` требует существующего layout/stable writer lease, те же independent
approvals и exact первоначальный intent, по-прежнему все4 control files отсутствуют.
Byte-identical staged objects допускают replay. При незавершённом journal сначала
нужен отдельный existing `recover`; автоматического rollback нет. После commit
повторный resume не сбрасывает authority — нужен обычный approved repair/status.
Ошибки до записи intent требуют отдельного осмотра оставшегося нового folder.

## Existing repair сохраняет более новый target

Вместо new/resume передаётся independent SHA **текущего target state**. Capture
может видеть отсутствующие/несогласованные aliases, но state обязан корректно
декодироваться и иметь ту же approved policy/lineage. Missing/corrupt authority
не получает silent replacement из ZIP.

Expected set строится из target current/history/root index. Backup предоставляет
совпадающие known objects; intact более новые target objects, которых нет в старом
backup, могут остаться источником. Conflicting content или неизвестные missing
objects — отказ. New-only write не перезаписывает повреждённые existing immutable
bytes; force/quarantine этой процедурой не вводятся.

State в journal остаётся **Keep byte-for-byte**. Поэтому более новые root/cache/
serving/clock floors/history не заменяются архивными. Aliases выводятся из target
head, а не слепо берутся из backup. Разные role policies требуют отдельного
authorization workflow, не adoption старой policy при ремонте.

## Активация и rollback boundary

Writer lease удерживается на staging и fixed4 transaction, не запускается listener.
BeforeCommit после synced journal staging повторяет source controls/intent/policy,
полный local immutable inventory, whole archive SHA/opened identity, clock/context.
Cold replica сохраняет exact approved state bytes; repair — существующий state.
Journal/recovery использует прежние4 targets, не arbitrary paths из ZIP.

Failure до commit оставляет append-only staging без новой authority; ошибка после
commit явно говорит committed. Resume/recover не удаляют writer lock по PID/age
и не превращают missing state в bootstrap. Остатки не удаляются рекурсивно.

Report отделяет local restore/mode/state SHA, **только** publication time validity,
client acceptance required и external reachability not observed. Он не доказывает
работу VPN/server, реальную совместимость клиента или отсутствие newer unseen state.

## Остаток всего плана

V2-145 PC source review после [first policy baseline](TH-03-BOOTSTRAP-POLICY.md):
`verifyPublisherBackup` уже требует independent exact archived state SHA + root
anchor + scoped application-policy SHA, не берёт authority из ZIP manifest.
New/resume требует absent authority и exact original restore intent; existing
repair берёт target checkpoint byte-for-byte. Полный retained policy marker/
clock/history внутри state SHA не заменяется старым backup; pending target/applied
policy mismatch далее блокируется serving gate `checkPublicationPinPolicy`.
Дополнительный дублирующий approval/ledger не добавлен. Это чтение исходников,
не выполненный restore/crypto/DR: latest-known external custody и утраченная
невидимая история по-прежнему требуют независимых данных владельца.

Corrupt immutable quarantine, cross-policy/source migration, latest-known cold
approval custody, shared storage migration, physical space/ACL/reparse/hardlink/
rename/handle/journal crash/reboot/power-loss, cancellation and90s cooperative I/O,
SDK/crypto/backup roundtrip/native DR/module/old-client/bindings/rollout/release
остаются открытыми. Реальное восстановление/rollback не выполнялось.
Далее — global signed revocation/private pin-history portability и следующие
P0/TH-02/04…08 source задачи. Все9 этапов и comparative gates сохраняются.
