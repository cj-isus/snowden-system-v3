# TH-01 / OFF-06 — SDK worker для продуктовой доставки

V2-189. Owner: `windows`. **Source implemented, unverified**.
Продолжение [storage](TH-01-SDK-STORAGE.md), [resource limits](TH-01-SDK-RESOURCES.md)
и [ADR-001](../decisions/001-tuf-delivery.md). Общая цель TH-00…08 сохраняется.

## Продуктовые точки входа

`onboarding_delivery`, `pin_policy_import`, `update_delivery` используют один
`core.PrepareDeliveryArtifacts`. Он вызывает прежний `delivery.PrepareArtifacts`
в отдельном процессе того же приложения. Publisher CLI сохраняет свой прежний
синхронный API; алгоритм pinned updater/crypto/role policy здесь не переписан.

Уже ограниченный acceptance/recovery-check worker выполняет SDK внутри своего
фактического Job: Ready определяется native Query, не аргументом/context flag.
Вложенный процесс и свежий CPU budget не создаются. Обычный GUI использует
отдельный worker; ошибка Query/spawn/cleanup не разрешает in-process fallback.

Обнаружен дополнительный GUI SDK call в `delivery.Enroll`: trustedmetadata.New
проверял initial root signature ещё до batch. Новый `EnrollmentSource` создаёт
только pending input после explicit OOB SHA, shape и expiry checks. Это не trust
grant/Prepared/persisted enrollment. Прежний updater.New в worker по-прежнему
обязательно вызывает trustedmetadata.New до создания Progress. Initial root
expiry повторяется перед commit через прежний bundle deadline. CLI `Enroll`
сохраняет signature-checked API. Проверка подписи не удалена и не заменена SHA.

## IPC и private proof

- Один canonical JSON request, до20MiB, frame header44 bytes и точный EOF.
  Owned State/Package/requests/now, свежие nonce и отдельный 32-byte random key.
  В argv только фиксированный режим и SHA256 всего request; key передаётся pipe.
- Один reply до6MiB. HMAC с domain separator связывает exact request SHA и reply
  bytes; MAC проверяется до JSON decode. Closed schema/canonical round-trip
  запрещают лишние/повторные/case-alias/alternate fields и trailing command.
- Пакет delivery остаётся единственным создателем private Prepared. Decoder
  сверяет source, anchor, exact clock floor, полный ordered batch, kind/path/
  artifact SHA/root version/deadline и общий authenticated pin-policy-required fact.
  Artifact bytes берутся из собственного запроса, не из произвольного reply.
- SDK logical failure может вернуть restriction-only NextState, без artifact
  grants. Closed bitmask сохраняет объединённые ErrTrust/ErrBudget/cancel/deadline
  classes без raw SDK error strings. Поздняя отмена после подтверждённого
  retirement снимает все artifact proofs, сохраняя только уже проверенный prefix.
- Ошибка transport/exit/resource/image/retirement не создаёт Progress. Неизвестный
  оригинальный Job/I/O/image owner удерживается; replacement до cleanup запрещён.
  Новый worker не обходит parent workspaceMu/legacy cache-reader quarantine.

**Граница доверия:** `PrepareWithTrustedWorker` принимает только статически
подключённый `core.exchangeDeliveryWorker`. Этот adapter входит в trusted code
base, выдающий proof; runtime/plugin/user/network callback недопустим. Он имеет
доступ к request pipe и при подмене кода мог бы сфабриковать ответ. HMAC защищает
binding одного вызова/ответа, но не аттестует злонамеренный adapter, kernel loaded
sections или native retirement. JSON/status/MAC сами по себе не доказательство
TUF или завершения процесса. В source найден один production binding; runtime
registration/setter и public ParsePrepared/digest constructor не добавлены.

## Native owner и lifecycle

Существующий suspended CreateProcess/Job/pipe/reaper переиспользуется. Отдельная
TryLock lane без очереди отделена от protected/optional diagnostics и входит в
общий Stop helper cleanup barrier. Initial1 process/512MiB committed memory/
20 user-CPU-seconds, exact Set/Query/termination action и accounting перед Resume
и после zero exit — прежний V188 contract. Capture30s, independent image/reap
budgets4s не доказывают точный kernel/handle deadline или total GUI+worker RSS.

Native image lease переиспользует original ancestors/ACL/FileID/deny-write-delete/
PE reader/checked Close. Держится текущий on-disk App, без обязательного network
helper. Без отдельного approved first-run image hash пересчёт собственного файла
не даёт loaded-section attestation; такой лишний полный проход не добавлен.
Parent и suspended child image paths проверяются через исходные process handles.
Это cooperative filesystem custody, не code-signing/hostile-owner protection.

Pure SDK worker имеет только SystemRoot/WINDIR/SystemDirectory cwd, без APPDATA/
TEMP/proxy/keylog inheritance. Dispatcher идёт до recovery/GUI/engine: отдельный
точный pipe+Job mode доступен и recovery companion, без repair/store/vault/GUI/
network-producer effects. Invalid flags terminal. Repair/GUI entrypoints и права
не расширены. Engine8, diagnostics128MiB и GUI lifetime Job policies прежние.

## Сохранённые контракты

Signed envelopes/pins/channel identity, role thresholds, exact retained inputs,
root rotation/online-key-reset/history/floors, full batch, effective clock, source
snapshot/final native fence, fixed4 journal/Keep/vault CAS, retirement перед
restrictive prefix и postcommit/quarantine outcomes остаются в прежних callers.
Никаких новых transport protocols, direct fallback, shared SDK/API миграций,
изменений dependencies, Wails bindings или Android/harness/configs здесь нет.

## Evidence и незакрытое

`.artifacts/memory/sdk-bridge-source-evidence.json`: exact15 Go hashes/sizes,
original9/new6 source diff, source caller trace. Gofmt14 + refinement4 + taxonomy3
captured exit0; manual review только. Tests/fixtures/build/compile/vet/typecheck/
parser/check/module install/SDK/crypto/native/admin/live/owner data не выполнялись.

Открыты native ABI/IPC round-trip/old callers/nested Jobs/admission/CPU overshoot/
OOM/error/cancel/Stop/partial creation/checked Close/orphans/ABI parity и trusted
adapter/code identity acceptance. Parent parsers/copies/app signatures/KDF и общий
GUI+worker CPU/RSS/rate остаются отдельными ресурсными gates. Все существующие
SDK per-role prefix eligibility/parity, directory-powerloss/restart/source coverage,
old-client/OS/DR/service/release/comparative/TH-00…08 gates остаются открытыми.
Public schema2 install **unsupported**. Source progress не означает native PASS.
