# TH-03 — повторная проверка перед продвижением выпуска

2026-10-23, V2-126/127. **Исходники добавлены, компиляция/runtime не проверены.**
Tests не добавлялись/менялись/запускались; build/vet/harness checks/module/CLI/
native/live не выполнялись. Реальные owner keys/certs/vault/config не читались.
Роль текущего чата привязана к Windows реальным session ID; Android и код
обвязки не менялись. Предыдущий срез — [candidate](TH-03-RELEASE.md).

## Граница реализации

`candidate.go`/`promotion_verify.go` дают private full-verification result;
V2-127 подключает его к отдельному Windows publisher store и исходникам команд
`promote`, `authorize-policy`, `recover` в существующих CLIs. Команды **не запускались**.
Это локальная подготовка public JSON repository; server activation/deploy,
проверенная криптографическая интеграция и работающий VPN этим не доказаны.

Исходная цель — все9 этапов PLAN §5.3 для ПК/Windows; все актуальные acceptance
остаются открыты. Исторические PASS не доказывают новые исходники.

## Прочитанный candidate не становится authority

- Reader ограничивает encoded bytes≤12MiB и клонирует input до hash/parse:
  approval и разбор относятся к одной owned копии. Внешний candidate SHA
  подтверждает выбранные bytes, не право назначить root.
- Exact mandatory fields проверяются отдельно для candidate/manifest/file/
  artifact objects. Duplicate/null/alias/unknown/missing fields отвергаются.
  Files требуют base64 strings, не альтернативные byte arrays; decoded file
  budget≤8MiB/max256, packages≤32 и source metadata≤64 остаются.
- Sorted exact file records, length/hash, case aliases, source mapping и
  derived client paths сверяются до применения. Missing/extra bytes выявляются
  через повторное построение полного layout/manifest и byte comparison.
- Source names не выбирают control destinations: восстановленный inventory
  проходит full rebuild, затем writer выводит paths внутри public layout.
  Absolute/escaping/device/reserved/trailing-dot paths запрещены; четыре
  control targets фиксированы отдельно и не берутся из journal/candidate.
- Самостоятельный release ID/self-hash — content identity, не подпись. Decoder
  не выдаёт private artifact proof. Issuer candidate gate также отказывает
  decoded объекту без первоначальных issuer Publication результатов.

## Отдельный полный verifier

`verifyRelease` получает либо retained `delivery.State`, либо отдельно approved
initial root SHA. При наличии retained state попытка повторного enrollment
отвергается. Root/hashes, заявленные manifest, не подменяют этот выбор authority.
Cache/version/clock floors переходят через штатный PrepareArtifacts workflow.
Current root alias связывается с **точными trusted root bytes из SDK**, не только
с номером версии: retained verifier может не читать присланный root той же версии.
Иначе подставной self-signed root с копиями legitimate online public keys мог бы
пройти отдельную layout проверку, пока SDK использует настоящий retained root.

Application policy передаётся отдельным trusted input, не из transfer key hints.
Она валидируется и копируется. Writer читает bounded external file по отдельному
approved SHA; требует schema2 scoped keys без delivery cache. Candidate и policy
должны находиться вне publisher store. До/после staging повторяются exact bytes/
absence/source/proof/time gates; policy digest остаётся в retained state.

Публичные transfer containers заново декодируются существующими codecs;
проверяются original artifact bytes/common metadata/purpose. Для profile
разрешён ровно его independent-policy signer, без дополнительных unsigned keys,
comments/recovery/device hints; created_at соответствует signed issued_at.
Credentials/новые права update/recovery/root из transfer не принимаются.

`deliverypack.CheckApplicationArtifact` общий для первоначального assembler и
повторного reader: profile/update role, signature, candidate schema/time/epoch
namespace и released_at gates. Это не проверка checkpoint/pins/credentials
каждого установленного клиента — они остаются в исходном App import path.

## Epoch grant — по каждому transfer

Release inventory может иметь один общий offline grant, но intent переносится
конкретным public profile package. Verifier запрашивает profile+grant одним SDK
batch только для этого transfer. Обычный соседний профиль не наследует grant
из предыдущего элемента/sort order. Оба proof имеют exact source/final state;
проверяются grant purpose/namespace/new public key/profile epoch/time.

Grant expiry, private proof expiry, bootstrap root/profile/current role TTL
повторно ограничивают candidate validity. Rebuilt manifest не может объявить
более слабые сроки. После полной проверки сохраняется private sealed result;
fresh check повторяет SDK proof/локальные роли/profile/grant перед будущей записью.

## Verified prefix отдельно от принятого выпуска

`promotionProgress` возвращает source delivery digest и latest verified SDK
state даже при позднем role/artifact/inventory/cancellation отказе. Field
`accepted` появляется только после всех gates/полного byte rebuild. При первом
неполном enrollment результат не выдаёт state для частичного назначения root.

V2-127 writer:

1. Сверяет exact source delivery digest, anchor/clock floor и четыре control
   файла под writer lease; чужой/stale progress не применяется.
2. На существующем enrollment отдельно commits latest authenticated prefix,
   включая поздний local-role/artifact/cancellation отказ. Public aliases/current
   остаются Keep, включая отсутствие; первый неполный enrollment не сохраняется.
3. После prefix transaction перечитывает source. Следующий publication journal
   содержит уже advanced prefix в backup, поэтому rollback не возвращает прежний
   root/cache/floor. Ошибка после retention прямо сообщает об этом отдельно от PASS.
4. Повторяет gates после Sync/Close staging всего backup/new set через optional
   `filetxn.Store.BeforeCommit`, до публикации recovery barrier. Старые callers
   с nil callback сохраняют прежний путь. Callback не изменяет targets.

Наличие commit source не доказывает directory durability/power-loss behavior.

## Хранилище, inventory и команды — V2-127

Dedicated local drive folder, без UNC/device namespace/alternate streams;
существующие reparse components отклоняются до/после directory creation:

```text
publisher/
  private/writer.lock                 stable Windows byte-range lease
  private/state.json                  authority hash + SDK state + published head
  private/promotion-journal/           fixed-path filetxn recovery barrier
  public/current.json                 canonical pointer to one release
  public/metadata/root.json           mutable alias
  public/metadata/timestamp.json      mutable activation hint, written last
  public/metadata/<version>.<role>.json
  public/targets/<namespace>/<hash>.<name>.json
  public/releases/<release-id>/candidates/<candidate-sha256>.json
  public/releases/<release-id>/<exact candidate files>
```

`private` нельзя отдавать HTTP server. Current/root/timestamp сверяются с retained
published head до операции; missing/corrupt state при existing aliases — отказ,
не reenrollment. State bounded≤5MiB+32KiB, exact known fields/SDK state schema;
source snapshots сохраняют разницу отсутствия/содержимого. Published head может
быть старее retained SDK root после prefix-only retention — это ожидаемо.

Immutable files добавляются new-only: существующие допустимы только byte-identical.
Исходный outer candidate JSON хранится по своему approved byte hash: один manifest
release ID не делает whitespace/map-order двух frames byte-identical и не должен
создавать ложный immutable conflict; actual signed/file bytes всё равно exact.
Version/hash namespace общий для TUF; client packages, чьи имена могут повторяться
между выпусками, остаются только внутри `releases/<id>`. Полный release view и все
shared immutable paths готовы и перечитаны до four-target state/current/root/
timestamp commit. Неактивированные leftovers не удаляются автоматически;
conflicting orphan требует явного owner repair/new store, не overwrite history.
Глобальная disk quota/GC ещё не реализованы. Context60s cooperative, не hard I/O/RSS bound.

Source CLI contracts, **не команды, которые уже выполнялись**:

```text
repository promote <candidate.json> <approved-candidate-sha256> <application-policy.json> <approved-policy-sha256> <approved-first-root-sha256|retained> <publisher-directory>
repository authorize-policy <application-policy.json> <approved-new-policy-sha256> <approved-current-policy-sha256> <publisher-directory>
repository recover <publisher-directory>
```

На existing state `promote` допускает только literal `retained` и прежнюю policy.
Её смена возможна отдельно через `authorize-policy`: exact old digest + независимо
approved new public scoped policy; SDK history/floors/current aliases сохраняются.
Это не миграция ключей клиента и не перенос TUF authority из policy. Удаление
retained state или обычный candidate не разрешают reset/root enrollment.

`recover` под той же lease восстанавливает четыре фиксированных targets из journal,
затем перечитывает их согласованность. Автоматического recovery при promote нет.
Без journal команда не подтверждает происхождение всех immutable files или health.

Windows lease использует уже закреплённый [x/sys v0.44.0 LockFileEx](https://github.com/golang/sys/blob/v0.44.0/windows/syscall_windows.go#L184).
Конкурирующий cooperating writer получает немедленный отказ, stable lock file
не удаляется/не «протухает» по PID/возрасту. На других OS store fail-closed;
остальные authoring commands от него не зависят. Native поведение не проверено.

## Следующий срез и отложенная приёмка

Для live HTTP atomic view нужны cooperating reader/serving layout или отдельный
promotion protocol: четыре локальные записи сами этого не доказывают. V2-128
добавляет [cooperating read/HTTP/status/alias repair](TH-03-SERVING.md)
и opt-in loopback command; всё unverified, команды/сеть не запускались. Следом —
client transfer selection/download/export/recovery UX, archive/root compatibility,
generated bindings/rollout и publisher time/storage lifecycle.
VPS/deploy не запускаются. EXE delivery/installer — TH-08, не этот JSON candidate.
ACL не назначается mode0700 на Windows; path checks не устраняют hostile rename/
hardlink/TOCTOU. Windows ACL/reparse/link/interprocess/crash/reboot/power-loss,
crypto/module graph/SBOM/license/compatibility и owner custody gates открыты.

Когда владелец разрешит testing: codec ambiguity/bounds/source mutation,
mixed ordinary/epoch transfer order, local-role mismatch/retired key, expired/
skip/mixed proofs, same-version root substitution, same-source replay/rollback, prefix after late failure,
first enrollment failure, rebuilt orphan/missing/altered files, explicit policy
rotation/source replacement, prefix-only→promotion rollback, after-staging fence
и native journal/interprocess/crash/power-loss/old-client rollout.
Тесты не создавались/менялись/запускались, сборки и новые CLI тоже не запускались.
