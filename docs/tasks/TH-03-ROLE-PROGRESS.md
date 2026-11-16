# KA-01/W2 — допустимый прогресс отдельных TUF-ролей

V2-191, 2026-10-30. Owner: `windows`. **Source implemented, unverified**.
[KA-01](../planning/knowledge-application/PLAN.md#ka-01-независимые-p0-платформ),
TH-01/TH-03, OFF-06. Полная задача TH-00…08 и прежние native gates сохраняются.

## Дефект и основание

`retainedState` использовал `GetTrustedMetadataSet` как свидетельство завершённой
проверки каждой роли. В pinned go-tuf2.4.2 timestamp/snapshot могут попасть в набор
до финального отказа по expiry/version. Наличие pointer не отличает успешный return
от такого промежуточного состояния. Старый `State.Cache` при этом уже содержит
ранее принятую историю, которую нельзя стирать только из-за её текущей давности.

Первичный источник: [trustedmetadata.go, fixed commit](https://github.com/theupdateframework/go-tuf/blob/f5edbde31e5507f46db2069402dc38903fe6d9d4/metadata/trustedmetadata/trustedmetadata.go).
`UpdateTimestamp`/`UpdateSnapshot` присваивают объект перед final checks; targets
присваивается после проверок. Cached `UpdateSnapshot(..., true)` пропускает byte
length/hash check. Это чтение исходников, не воспроизведение атаки или SDK execution.
Неверная первая попытка raw-path дала cache miss; правильные commit/tag source и
прежний локальный exact-source snapshot прочитаны отдельно.

## Учёт успешных возвратов без второго verifier

В существующем Windows-private `internal/tufupdater` добавлен `progress.go`:

- Каждый оригинальный SDK per-role call обёрнут наблюдением его result/error.
  Только nil error обновляет отдельный `SuccessfulMetadataSet`. Исходные result,
  error, аргументы, порядок, retries и SDK policy возвращаются без подмены.
- Учёт выполняется до следующего cache write. Поэтому поздний write/lookup/target
  failure не теряет уже допустимый prefix. Root начинается с успешного SDK New;
  последовательные root rotations по-прежнему проверяет исходная библиотека.
- Тип отличается от SDK TrustedMetadata: случайная передача intermediate set в
  retainedState больше не соответствует его сигнатуре. Getter копирует map ролей;
  parsed pointers считаются неизменяемыми в pinned SDK workflow. Это внутренний
  контракт доверенного кода, не attestation от hostile caller/mutation.
- Нет callback, second verifier, новых wire fields, clock override, сетевого или
  disk fallback. `GetTrustedMetadataSet`, исходный Refresh/GetTargetInfo, snapshot
  verifier и криптография остаются в финальной artifact chain. Delta расширяет
  прежнюю storage-only адаптацию наблюдением; provenance уточнён в ORIGIN.md.

## Допуск к сохранению — отдельно от успешного return

`retainedState` начинает с owned clone прежнего accepted State и effective clock.
Каждая новая запись проходит прежнюю exact original-byte binding/continuity и
следующие gates до присваивания cache entry:

| Роль | Условия нового сохранения | При отказе |
|---|---|---|
| root | Original SDK success, последовательная цепочка, root policy/exact bytes | Прежняя root history; принятые промежуточные expired roots допустимы |
| timestamp | Success receipt, действующий root и timestamp по effective now, прежние version/reference floors/equal-version immutability | Прежний timestamp и его rollback history |
| snapshot | Success receipt, допущенный timestamp, expiry, exact timestamp version/length/hash link, прежняя snapshot continuity | Сохранённый допустимый root/timestamp prefix и прежний snapshot |
| targets/profiles/updates | Success receipt, допущенный snapshot, expiry и exact snapshot version/length/hash link | Прежний entry; ранее допущенные роли остаются |

Отсутствующий receipt не удаляет cache. Старые accepted expired bytes остаются
историей, не становятся fresh proof. Исключение — прежний официальный online-key
reset timestamp/snapshot при SDK-проверенной root rotation; old seed не повышается
до нового prefix по совпадению pointer, продолжение требует bounded fresh pass.
Новый rejected expired/mismatched объект не повышает rollback floor следующего
вызова. Принятый ранее expired предшественник сохраняет свой прежний floor.

Для cached `isTrusted` snapshot точный byte link проверяется до нового сохранения.
Прежний `completeRetainedState` также оставлен: новые проверки его не заменяют.
FinalPolicy/complete batch/full SDK set/source fences по-прежнему нужны до proof.
Prefix не разрешает Connect, import, install, serving или target publication.

## Сохранённые границы

Формат State/schema1, AnchorSHA, old cache, clock floor, SDK versions/manifests,
private Prepared, compiled worker adapter/HMAC/retirement, storage sticky fault,
fixed4/Keep/no-op/CAS/source/encrypted-vault fences и lifecycle не менялись.
Success receipt не является доказательством durable commit: all4-target actual
outcome, postcommit quarantine, original writer/native handles остаются отдельными.
Никакие SDK/shared/Android/harness чужие модули или правила не менялись.

## Evidence и открытая приёмка

`.artifacts/memory/role-prefix-source-evidence.json`: exact5 Go hashes, six docs,
original4/new1 source diff и delta pinned updater относительно upstream. Gofmt
только changed Go sources; manual source trace, source identity readback.
Tests/fixtures не добавлялись/менялись/запускались; build/compile/parser/vet/
typecheck/check/SDK/crypto/native/admin/live/install/network не выполнялись.

Открыты rejected-new/retained-old expired timestamp, equality/rollback/mismatched
snapshot, cached same-version changed hash, lawful partial prefix, root reset/
overlap/ceiling, late storage/context/retirement failure, batch authority, SDK
parity/API/ABI и actual fixed4/Keep/no-op/CAS/source outcomes. Отдельно остаются
native durability/crash/restart/old-client/OS/DR/resources/service/release/
comparative/full9 gates. Schema2 public install остаётся unsupported.
Следующий Windows source-срез — W3 own-write proxy receipts/readback/conflict;
W4 probe-only admission остаётся после него. W1 native acceptance тоже открыта.
