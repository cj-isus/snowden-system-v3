# TH-01 / OFF-06 — local metadata storage SDK

2026-10-28, V2-187. Owner: `windows`. **Source implemented, unverified**.
Продолжение [public readers](TH-01-PUBLIC-READERS.md) и
[original workspace](TH-01-UPDATE-PAIR.md). Решение —
[ADR-001](../decisions/001-tuf-delivery.md#v2-187-local-storage-seam-2026-10-28--source-only).

## Точный источник и обнаруженный gap

go.mod по-прежнему закрепляет go-tuf/v2 v2.4.2. Go proxy version-info Origin и
GitHub tag ref независимо указывают commit
`f5edbde31e5507f46db2069402dc38903fe6d9d4`; SHA256 tag/commit-specific updater.go
совпали: `61653cc910879edd28c6cdd7e28bcc7bacacd28c0a022e0f3ff62b9f16c10dc2`.
Это source provenance, не проверка полного module zip против go.sum.

В [config](https://github.com/theupdateframework/go-tuf/blob/f5edbde31e5507f46db2069402dc38903fe6d9d4/metadata/config/config.go)
нет storage interface. [Updater](https://github.com/theupdateframework/go-tuf/blob/f5edbde31e5507f46db2069402dc38903fe6d9d4/metadata/updater/updater.go)
использует EnsurePathsExist, os.ReadFile, CreateTemp/Write/Close/Rename/readback
для metadata, WriteFile/ReadFile для target cache. Некоторые Close/short-write
outcomes не наблюдаются, а local read errors могут трактоваться как cache miss.
Checked outer workspace seed/cleanup не владеет всеми этими original handles.
DisableLocalCache не устраняет local reads. Это подтверждённые source calls,
не воспроизведённый runtime/OS defect.

## Узкая адаптация готового workflow

`delivery/internal/tufupdater/updater.go` — полный pinned исходник с одним required
Storage и изменёнными local I/O boundaries. LICENSE/NOTICE byte-for-byte из того
же commit; original header + prominent change note + ORIGIN сохранены. Raw diff
в `.artifacts/memory/sdk-io-upstream/updater-local.patch` не меняет TUF workflow
bodies: последовательность root/timestamp/snapshot/targets, dual thresholds,
delegated walk, version/expiry/hash checks. Metadata/trustedmetadata/config/crypto
остаются imports исходного pinned module. Нет local reimplementation криптографии,
второй policy, упрощённого verifier или ветки fallback к старому updater.

EnsurePaths/ReadMetadata/WriteMetadata/ReadTarget/WriteTarget обязательны; default
disk storage отсутствует. PersistMetadata сохраняет write/readback comparison.
Product по-прежнему задаёт UnsafeLocalMode=false, DisableLocalCache=false,
package-only Fetcher и прежние root/delegation/byte limits. UnsafeSetRefTime не
вызывается. Target cache явно unsupported: existing complete-batch acceptance
проверяет supplied artifact bytes/length/hash/role, SDK target download не нужен.

## Владение байтами и ограничения

`sdk_memory.go` содержит private metadata-only cache:

- Только virtual metadata/target namespace identifiers; они не передаются OS.
  Exact clean parent/name, только root.json +5 существующих cache roles. Нет
  arbitrary paths, re-encoding, hidden disk/network fallback или root enrollment.
- Cache seed и каждое SDK read/write владеют independent bytes. Bounds применяются
  до clone, заменённый owned buffer очищается, caller/SDK aliases не возвращаются.
- До6 файлов, до512 KiB каждый, до3 MiB live raw cache values. До256 charge steps
  и до52 MiB charged clone work при текущих MaxRecords/MaxPackageBytes/MaxStateBytes.
  Present read расходует lookup step и copy step. Это не aggregate RSS/GC/CPU/
  whole-SDK allocation bound и не hard deadline.
- Только настоящий miss разрешённого имени возвращает fs.ErrNotExist без sticky
  fault. Namespace/limit/context/target-cache нарушения сохраняют первый error;
  дальнейший refresh/fallback не может его снять. Нет reset/retry обхода ошибки
  в пределах этого verification owner.
- Adapter наблюдает error после New, Refresh, каждого GetTargetInfo и перед
  выдачей complete proof. При позднем отказе допустим только прежний verified
  restriction-only prefix; private artifact proof не выдаётся. Existing raw-prefix/
  online-key-reset/rollback/equivocation/full-batch/source/clock gates прежние.

После retirement adapter ещё раз проверяет context. Поздняя отмена или legacy
retirement failure снимает complete artifact proofs из возвращаемого Progress;
только уже проверенный restriction-only prefix может остаться вместе с error.

openWorkspace сохраняет process-wide TryLock/retained owner и прежний
cacheReaderRetired barrier, но создаёт memory storage вместо native temp tree.
Legacy platform records/cleanup code остаются для quarantine/compatibility и не
создаются активным SDK path. Удерживаемый original owner не отбрасывается даже
ради нового memory path. Close освобождает только owned public buffers, а старые
uncertain native/cache-reader receipts по-прежнему блокируют replacement/proof.
Удаление legacy исходников требует отдельной разрешённой parity/compatibility
приёмки; второй trust workflow через них не вызывается.

## Границы доказательства и следующий шаг

Evidence: `.artifacts/memory/sdk-io-source-evidence.json`, exact5 Go sources плюс
ORIGIN/LICENSE/NOTICE hashes, upstream commit/tag/version metadata и exact source
diff. Gofmt5 + refinement3 + final comment1 captured exit0; manual ownership/
namespace/raw-byte/call-site/protocol-delta review. No tests/fixtures/build/compile/
vet/typecheck/parser/check/module install/SDK/crypto/native/admin/live/owner values.
Existing malformed-cache/missing/rollback/intermediate-rotation/online-key-reset/
delegation/full-batch/late-error/cancel/parity/API/resource scenarios не исполнялись.

Это устранение explicit local metadata-cache file calls в активном source path,
не whole-SDK I/O attestation, benchmark, гарантия без OOM или native acceptance.
Оригинальные retained profile storage writers/readers, fixed4 journal/Keep/CAS/
source fences, WFP/TUN/session/lifecycle и installation custody прежние. Directory
power-loss/restart, orphan/late recovery/source coverage, module resolution/API/
ABI/CPU/RSS/deadline/OS/DR/old-client/service/release/comparative и все9 gates
остаются открыты; public schema2 install unsupported.

V2-188: [existing SDK check worker resources](TH-01-SDK-RESOURCES.md) закрывает
source defect commitBytes=0/no CPU cap только у acceptance/recovery-check consumer.
Initial512MiB committed Job memory/20 user-CPU-seconds, exact Set/Query/terminate
action/child readiness/post-exit accounting используют прежний original owner.
Engine/GUI policies прежние; full profile/pin/export SDK process bridge ещё нужен.
Gofmt8+1/source-only, native/parity/budget compatibility/OS/all9 не проверены.
