# TH-03 — публичные пакеты издателя

2026-10-22, V2-121. **Исходники добавлены, компиляция и поведение не проверены.**
Tests не добавлялись/менялись/запускались; build/vet/harness checks, dependency
resolution/install, worker/native/live не выполнялись. Издательские CLI,
подпись/генерация ключей, реальный vault и сертификаты владельца не запускались
и не читались. Форматирование gofmt не подтверждает работоспособность.

Предыдущий контракт: [App integration](TH-03-TUF-INTEGRATION.md).

## Реализация

Общий `windows/tools/internal/deliverypack` вызывается существующими командами:

```text
metadatatool delivery-pack public-plan.json APPROVED_ROOT_SHA256 new-profile.json
updatetool delivery-pack public-plan.json APPROVED_ROOT_SHA256 delivery.json
```

Это описание будущего вызова владельцем; команды агентом не выполнялись.
Hash — полный lowercase SHA256 точных root bytes из отдельного доверенного канала.
Он передаётся отдельным аргументом, не извлекается как одобрение из плана.

План содержит ровно шесть полей. Пример структуры, не готовый подписанный пакет:

```json
{
  "schema": 1,
  "kind": "profile",
  "target_path": "profiles/main.json",
  "artifact": "artifacts/channels.json",
  "authority": "public/application-keys.json",
  "metadata": {
    "root.json": "metadata/root.json",
    "timestamp.json": "metadata/timestamp.json",
    "12.snapshot.json": "metadata/12.snapshot.json",
    "7.targets.json": "metadata/7.targets.json",
    "21.profiles.json": "metadata/21.profiles.json"
  }
}
```

Для update: `kind=update_manifest`, target внутри `updates/`, artifact — исходный
`update.json`; metadata map содержит необходимые versions/updates delegate.
Числа имён берутся из настоящих signed links, не из примера. Дополнительные
intermediate roots указываются явно; wildcard/recursive discovery нет.

## Границы файлов и полномочий

- Plan≤32KiB; только явные относительные `.json` пути из каталога плана:
  portable ASCII, ≤240 bytes/12 components, без traversal/drive/URL/backslash.
  Видимые symlink components отвергаются. Windows reparse/ACL/interprocess и
  подмена родительского каталога не объявляются доказанно защищёнными.
- SDK bounds сохранены: metadata≤512KiB, map≤64 records/8MiB; root +5 retained
  roles≤5MiB, encoded package≤12MiB. Application authority file≤6MiB.
- `delivery.NewPackage` владеет original metadata bytes; SDK `Enroll/Prepare`
  выполняет прежний workflow/threshold/purpose/length/hash/expiry gate. Нет HTTP,
  unsafe clock/local mode, автоматического подписания или CLI private key input.
- Authority file дополнительно проверяет application signature отдельно от TUF:
  profile keys не разрешают code updates. В public profile переносится только
  действующий signer public key, без update/recovery grants и comments.
- `publisherfiles.Read/PublishNew` переиспользует прежний bounded/new-output
  алгоритм metadatatool. Lstat/SameFile, stage→Sync→Close→hard link; существующий
  destination не заменяется. Parent-directory durability и native gates открыты.
- В stdout — только hashes/bytes/purpose/root version/expiry/signer и
  `requires_client_acceptance=true`. Это issuer snapshot, не история текущего
  устройства. Payload EXE этим subcommand не читается и не подтверждается.

## Результаты для клиента

Profile output — публичный `onboarding.BundlePayload` с пустыми secrets, exact
base64 profile bytes и TUF package; общий bound256KiB сохранён. CreatedAt берётся
из signed IssuedAt; форматирование не меняет bytes для verifier. Update output —
sidecar `delivery.json`; исходный `update.json` и EXE остаются отдельными файлами.

`DecodePublicProfile/EncodePublicProfile` не являются криптографической приёмкой.
Они ограничивают container, отвергают duplicate/alias/null/unknown/trailing JSON
и credential slots. Публичная передача не заменяет зашифрованный secret transfer.

Новые App API `PreviewPublicProfile` / `ApplyPublicProfile` принимают request:

| Mode | Дополнительное подтверждение |
|---|---|
| `profile` | Не меняет root/recovery authority |
| `enroll` | Отдельный `approved_root_sha256`, без recovery target |
| `recover` | Отдельный `approved_target_sha256` при Apply, без нового root |

Apply всегда требует confirm; unknown mode/лишнее назначение authority отвергается.
На чистой установке (нет accepted version и envelope) explicit root enrollment
разрешает только signer точного delivered profile: один incoming public key.
Seed profile keys сохраняются retired; update/recovery roles не меняются и не
продвигаются, retired signer не возрождается. Уже принятая история исключает
такой bootstrap: смена её publisher по-прежнему требует recovery authority.
Все режимы идут через общий `importPreparedProfilePayload`: device checkpoint,
local roles, retained pins/revocations, SDK proof, credential/render gates,
retire/source fence/four-target commit. На устройстве credentials должны быть
уже заполнены либо перенесены прежним encrypted flow. Preview не пишет state.
New Wails bindings и UI **ещё не сгенерированы/не подключены**.

App и updatetool теперь используют один `backend/update.DecodeStrict` вместо
расходящихся парсеров. Application signing остаётся отдельным AND gate;
`delivery-pack` принимает уже signed TUF inputs. V2-124 добавляет в те же CLI
[подготовку ролей и раздельное подписание](TH-03-PUBLISHER.md), source-only/unverified.

## Остаток

- V2-124 source: role authoring, per-custodian signing, threshold assembly,
  snapshot/timestamp exact hash links, refresh/dual-threshold root rotation и
  new-only file publication. V2-125 добавляет [coherent candidate/layout/manifest](TH-03-RELEASE.md)
  в source; state journal/promotion/restore automation и reproducibility открыты.
  Delivery-pack по-прежнему требует finalized signed public inputs; отдельная
  подпись role не полная приёмка repository или установленного клиента.
- Builder проверяет от явно одобренного issuer root. Он не доказывает chain от
  старого client anchor, сохранённые rollback floors, local publisher enrollment,
  pin continuity, credentials, staged EXE или фактический VPN-путь.
- Public recovery-plan generation/упаковка permit ещё не автоматизирована;
  существующий `recovery-sign` и общий client recovery gate сохраняются.
- Root/profile epoch recovery — [V2-122/123 source](TH-03-EPOCHS.md), unverified;
  optional `epoch_grant` в plan, SDK batch и public `epoch_recover` добавлены.
  Fresh nonzero-epoch bootstrap добавлен отдельным `epoch_enroll` с schema3 origin,
  independent root/target approval и clean-source gate, пока source-only.
  Enrolled legacy export/UI/bindings/rollout,
  module/SBOM/compile/conformance/OS/release/comparative gates открыты.

Ни действующий root, ни real publisher keys агентом не выпускались/не назначались.
Все девять этапов PLAN §5.3 остаются задачей; это один source slice TH-03.

V2-122 расширяет прежний six-field plan optional седьмым `epoch_grant` только
для profile. Grant bytes сохраняются exact/base64, не как RawMessage. Report
добавляет string tuple, canonical profile content SHA и raw grant SHA;
raw artifact SHA остаётся отдельным. Source implementation, CLI не запускались.
