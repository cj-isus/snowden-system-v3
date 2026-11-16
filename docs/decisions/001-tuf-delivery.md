# ADR-001: go-tuf для доставки, отдельные application gates

2026-10-21, V2-119. Решение о библиотеке принято для исходников адаптера.
**Не production acceptance:** module graph/build/runtime не проверены.
[V2-120](../tasks/TH-03-TUF-INTEGRATION.md) добавил App/state integration в исходники;
новый EXE не собран. Тестирование остаётся запрещённым владельцем.

## Выбор

Использовать целиком client workflow `github.com/theupdateframework/go-tuf/v2`
**v2.4.2**, tag commit `f5edbde31e5507f46db2069402dc38903fe6d9d4`.
[go.mod выбранного tag](https://github.com/theupdateframework/go-tuf/blob/v2.4.2/go.mod)
объявляет Go1.25.0; проект — Go1.26. Это совместимые minimum declarations,
а не доказанная совместимость всего module graph или Windows build.

Причины: готовые root dual-signature/threshold, timestamp/snapshot/targets,
rollback/freeze/hash/length и delegation checks вместо своей реализации TUF.
В [v2.4.2](https://github.com/theupdateframework/go-tuf/releases/tag/v2.4.2)
исправлен подсчёт duplicate public material в thresholds. Предыдущие
[threshold](https://github.com/theupdateframework/go-tuf/security/advisories/GHSA-fphv-w9fq-2525)
и [malformed-response](https://github.com/theupdateframework/go-tuf/security/advisories/GHSA-846p-jg2w-w324)
advisories указывают patched≥2.3.1. Это не исчерпывающий vulnerability/SBOM gate.

Apache-2.0: [license/notice](../third-party/go-tuf-v2.4.2/NOTICE), полный LICENSE
сохранён рядом. Проверка лицензий всего нового transitive graph ещё открыта.
go.mod закреплён; go.sum дополнен checksum records. SumDB lookup из окружения
не удался; own-module hashes сверены с официальным cli/cli go.sum, остальные
новые records взяты из выбранного upstream go.sum. Go tooling verification и
полное dependency resolution не запускались, модули в cache не устанавливались.

## Текущий компонент

`windows/backend/delivery` использует полный upstream workflow; V2-187 ниже
добавляет narrow local-storage seam, не собственный TUF verifier.
Один Updater на operation; thread-unsafe объект между goroutines не разделяется.
Фиктивный URL `snowden.invalid` — namespace in-memory fetcher, не endpoint.
`UnsafeLocalMode=false`; test-only `UnsafeSetRefTime` не вызывается. Библиотека
получает metadata из локального пакета через штатный workflow и private cache.
Никаких HTTP fallback/retry, скачивания EXE, VPN/Manager/policy mutation здесь нет.

Дополнительный product profile поверх TUF:

- Ed25519 public keys, spec family1.0.x, consistent snapshot; root и top-level
  targets thresholds≥2 по различному public material; онлайн timestamp/snapshot
  не используют offline root/targets keys. Это не доказательство независимости
  людей, машин, провайдеров или процедур хранения ключей.
- Только две immediate terminating delegations: `profiles/*` и `updates/*`.
  Profile/update keys различаются; update delegate threshold≥2. Top-level target
  shadowing, arbitrary role filenames, nested/succinct delegations отвергаются.
- Extra local effective-time expiry gate может только ужесточать upstream wall
  reference. Local clock/floor не становится authenticated UTC.
- Root threshold/resource policy проверяется и на intermediate roots; исчерпание
 16-rotation pass возвращает continuation, не artifact proof. Caller должен
  сохранить проверенный prefix и выполнить следующий ограниченный проход.
- Target proof привязан к kind/path/точным bytes/сроку; metadata signing не заменяет
  application envelope/manifest schema, локальные роли, version/pin history и
  lifecycle gates. Проверенный prefix на последующем отказе не является artifact PASS.

Metadata/cache — byte arrays, JSON base64. RawMessage внутри outer JSON изменил бы
whitespace при marshal и сломал signed snapshot length/hash links. Пакет не
пересериализует original metadata перед передачей verifier/persistence.

### V2-187: local storage seam, 2026-10-28 — source-only

В v2.4.2 `UpdaterConfig` не даёт storage hook, а updater напрямую выполняет local
metadata read/temp/write/rename/readback. DisableLocalCache не отключает local
reads. Поэтому private SDK temp workspace с checked seed/cleanup не даёт receipt
для всех внутренних файловых handles.

Выбрана локальная адаптация **одного полного upstream updater.go** под
`windows/backend/delivery/internal/tufupdater/`: explicit required Storage,
изменены только local I/O boundaries. Root/timestamp/snapshot/targets workflow,
delegated walk и все threshold/signature/version/expiry/hash decisions остаются
upstream source; metadata/trustedmetadata/config/crypto берутся из прежнего pinned
модуля. В product path нет второго verifier, OS-file fallback или ослабления роли.
go.mod/go.sum не меняются. Provenance/license/change note —
[ORIGIN](../../windows/backend/delivery/internal/tufupdater/ORIGIN.md).

Product Storage — bounded owned bytes в памяти: root +5 прежних cache roles,
точные bytes/owned clones, live/work/step bounds до allocation, genuine cache miss
отдельно от sticky budget/path/context fault, target cache unsupported. Adapter
проверяет sticky error после каждого mutating SDK entrypoint и до полного proof,
потому что SDK может проглотить local read failure. Persist write/readback остаётся.
Scratch cache не durable authority: retained state/clock/floors/fixed4 imports и
private complete-batch proof прежние. Global owner/legacy I/O quarantine barriers
сохраняются; новые native SDK workspaces в активном source path не создаются.

Основание выбора — отсутствие hook в точном pinned source, сохранение готового
TUF workflow и устранение explicit metadata-cache filesystem acquisition из
этого пути. Это не доказанный выигрыш скорости/RSS, whole-SDK I/O attestation,
compile/native/parity acceptance или production promotion. Tests/SDK/build/module
install не запускались. Protocol-policy diff, cache/rollback/intermediate rotation/
online-key reset/delegation/full-batch/error/cancel/API и resource/OS/DR/release
gates открыты; см. [TH-01-SDK-STORAGE](../tasks/TH-01-SDK-STORAGE.md).

## Контракт интеграции (исходники V2-120, приёмка открыта)

1. Хранить retained delivery state внутри существующего scoped `trusted_keys.json`
   с bounded reader/clone/authority digest; увеличить отдельный public state budget
   осмысленно. В accepted profile checkpoint сохранить anchor marker, чтобы потеря
   или omission delivery state не включала legacy fallback.
2. Bootstrap — отдельное owner intent с out-of-band root SHA256 и точным current
   profile target. V2-144 [PC policy baseline](../tasks/TH-03-BOOTSTRAP-POLICY.md)
   дополнительно требует независимо подтверждённый SHA точного cumulative policy
   snapshot, полный private proof и retained bans. Known cut не доказывает global
   latest/unseen history; обычный import не получает reset. Source-only/unverified.
   Присланный root в обычном bundle не повышает полномочия.
   Root rotation — только штатная последовательная dual-threshold цепочка.
3. Провести state/proof через общий profile import prepare→retire→source fence→
   four-target commit; сохранять verified root prefix и metadata floors также
   при отказе следующего repository step. Не оставлять cache как единственную
   durable authority и не добавлять необъявленный пятый journal target.
4. Для enrolled updates нужен AND-gate TUF exact manifest target + действующая
   local update authority + payload/staging/version/owner lifecycle gates.
   Backend verifier сам не устанавливает приложение и не подтверждает запуск.
5. Freshness доставки не равна сроку уже принятой VPN-сессии. Не выполнять network
   refresh на каждый Start и не объявлять здоровый data plane плохим из-за outage
   distribution mirror. Применять signed profile/pin TTL по отдельному контракту.
6. Root epoch/application profile counter migration — отдельный контракт.
   TUF metadata counters не сбрасывают наш `MetadataVersion`/update floor.
   Compromised-publisher fast-forward и lost checkpoint recovery остаются открыты;
   нельзя скрыто обнулить старую историю при смене root.

V2-121: [public assembler/API](../tasks/TH-03-ISSUER.md) добавлены в source;
V2-122/123: [profile epochs/origin](../tasks/TH-03-EPOCHS.md) реализуются в source.
V2-124: [publisher role authoring и detached custody](../tasks/TH-03-PUBLISHER.md)
используют те же pinned Metadata API/Sign/VerifyDelegate, cjson0.11.0 и
Sigstore1.10.6, уже входившие в dependency tree SDK. Они объявлены direct imports;
версии/checksum records не обновлялись, module tooling не запускался.
V2-125 добавляет [coherent metadata candidate/layout/manifest](../tasks/TH-03-RELEASE.md),
через тот же issuer verifier. Publication state/journal/promotion/restore, физическая custody и весь
runtime/module/SBOM/OS/rollout gate остаются открыты. Всё source-only/unverified.

## Отложенная приёмка

Compile/ordinary/tagged/vet, compatible module graph/SBOM, API fixtures,
root rotation/freeze/equal-version/mix-and-match/budget/cancel, byte persistence,
Windows ACL/reparse/interprocess/power loss, old-client rollout, Wails/product UX,
reproducible EXE/canary и comparative gates не выполнялись. Upstream в source
объявляет SPECIFICATION_VERSION1.0.31; conformance нашего профиля к актуальной
спецификации1.0.37 не заявляется. Все TH-00…08 сохраняются.

## V2-189 — product worker transport (source-only)

Product Prepare consumers use the existing core original process/Job/image/reaper
through one statically bound adapter; delivery owns the private invocation codec
and reconstitutes Prepared only after this adapter has confirmed full retirement.
There is one actual pinned PrepareArtifacts verifier in the child. Existing
bounded update-check Jobs execute it inline, without nested process/CPU regrant.
No GUI SDK fallback. Publisher CLI keeps the synchronous signature-checked API.

The compiled adapter is explicitly part of the proof-issuing trusted code base.
It receives the original private pipe request and could fabricate replies if its
code were replaced: per-call HMAC is invocation binding, not process attestation.
Do not expose PrepareWithTrustedWorker to plugins/runtime/network-selected hooks.
The current production binding is core.exchangeDeliveryWorker only. This is not
a public serialized-proof import or a new root approval mechanism. Native code
identity/adapter-boundary/package/API compatibility must be accepted before
publication; malicious trusted-owner/Go code and loaded sections are not attested.

Current on-disk App/ancestor identity, ACL, deny-write-delete and PE reader reuse
the existing native custody; no unapproved self-hash pass claims loaded-code
identity. Dedicated no-queue SDK lane shares the Stop retirement barrier. Parent
legacy SDK workspace/cache-reader uncertainty still forbids a new invocation.
Initial Job ceilings remain1/512MiB committed/20userCPU, not measured RSS/rate/SLO.

EnrollmentSource is only explicit-OOB/shape/freshness-checked pending verifier
input. Initial root signature verification remains mandatory in existing SDK New
before any Progress; first-enrollment final expiry/source gates remain in callers.
Signed roles/history/floors/clock/full batch/fixed4/source fences are not changed.
Source/gofmt14+4+3 only. No tests/build/SDK/native; IPC/parity/per-role prefix/
authority/ABI/resources/durability/OS/release/all9 gates open, schema2 unsupported.
