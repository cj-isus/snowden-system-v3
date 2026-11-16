# TH-03 / OFF-06 — адаптер TUF-доставки

2026-10-21, V2-119: исходники адаптера, unverified. Последующая исходная
[App/state integration V2-120](TH-03-TUF-INTEGRATION.md) также не проверена.
Тесты не добавлялись/менялись/запускались; build/vet/harness/experiments/native/
worker/live, package initialization и CLI не выполнялись. Gofmt не acceptance.
Все9 этапов остаются задачей. Решение и следующий контракт: [ADR-001](../decisions/001-tuf-delivery.md).

Актуализация V2-191: [KA-01/W2](TH-03-ROLE-PROGRESS.md) отделяет successful SDK
per-role returns от intermediate pointers и от допустимого сохранения. Новые
expiry/version/link failures не сохраняют отклонённую роль; old accepted expired bytes
сохраняются. Pinned private updater имеет storage + observation delta, финальные
SDK/snapshot checks остаются. Это source/unverified, не SDK/runtime приёмка.
Старые пункты ниже описывают V2-119, включая прежний временный cache; current
SDK memory/worker границы — STATE и TH-01-SDK-STORAGE/TH-01-SDK-WORKER.

## Добавлено

- `windows/backend/delivery/model.go`: bounded public State/Package, kind/path
  Request, неприсваиваемые из входного JSON Prepared proof/Progress. State и
  metadata buffers owned; bounds проверяются до копирования. Duplicate JSON,
  неподдержанные role/path/spec family, пустые/oversized inputs отвергаются.
- `policy.go`: initial root fingerprint approval + upstream self-threshold
  verification; static product threshold/role isolation; проверка каждого root
  цепочки, не только последнего. Не caller-supplied profile key bootstrap.
- `client.go`: обычный go-tuf Updater Refresh/GetTargetInfo/VerifyLengthHashes,
  injected offline package fetcher, отдельный временный cache для operation.
  Нет default HTTP fallback, global logger mutation, UnsafeLocalMode или подмены
  RefTime. Context checks и bounded work не являются hard OS/CPU/RSS guarantee.
- Verified prefix возвращается отдельно от artifact proof при позднем отказе.
  Original library-persisted bytes сохраняются, root берётся из подтверждённой
  версии цепочки. При max16 rotations proof не выдаётся до следующего прохода.
- Artifact proof требует expected kind/path/bytes и дополнительный effective-time
  gate, bound SHA256/length и expires. Это историческое подтверждение доставки,
  не running-session/installation verdict. Caller ещё обязан commit source fence.
- Public metadata хранится в JSON как base64 byte arrays: форматирование outer
  JSON не должно менять bytes, по которым timestamp/snapshot подписали hash/length.
- go-tuf/v2.4.2 закреплён в go.mod; добавлены новые checksum records в go.sum без
  смены прежних selected versions. Полное module resolution и SumDB verification
  tooling не выполнены; новый module graph может требовать доведения.
- 13 selected upstream source/license/manifest files сохранены и сопоставлены
  с Git blob IDs tag tree. Это evidence чтения/происхождения source, не наших
  tests. Источники в ADR; research snapshots — `.artifacts/research/go-tuf-v2.4.2`.

## Входные ограничения (не измеренные SLO)

| Область | Лимит в исходниках |
|---|---|
| Metadata record | 512KiB, timestamp fetch64KiB |
| Package | 64 records, 8MiB decoded, 12MiB encoded JSON |
| Retained state | root +5 known cached roles, 5MiB encoded state |
| Rotation/lookup | 16 roots/pass, 8 delegated visits, 64 fetches/8MiB |
| Keys/signatures | ≤64 keys/signatures, ≤32 keys в role |
| Opaque artifact | profile≤256KiB либо update manifest≤64KiB; EXE сюда не входит |

## Что не готово

Persistence/omission marker, explicit root enrollment и profile/update AND-gates
добавлены в App source V2-120. Recovery UI/Wails bindings, issuer package UX,
root/application epochs, durability/module/SBOM/native/live приёмка открыты.
Новые исходники не защищают установленный EXE одним наличием файлов. Owner root/key
не читался, не создавался и не назначался. Новый EXE не собран и не опубликован.

Следующий срез — issuer package tooling и явный epoch/recovery контракт. Без тестов
и изменения системной сети до нового указания владельца; остальные этапы в PLAN.
