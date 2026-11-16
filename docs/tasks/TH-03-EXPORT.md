# TH-03 — свежий issuer proof и зашифрованный перенос enrolled профиля

Publisher retention продолжен в [V2-131](TH-03-RETENTION.md); перенос private
pin history и дополнение whole package старыми root chains остаются открытыми.

2026-10-23, V2-130. **Source-only/unverified.** Тесты не создавались/менялись/
запускались; compile/build/vet/harness/module/Wails generation/CLI/HTTP/listener/
native/live не выполнялись. Agent не читал owner keys/certs/vault/runtime config
и не создавал реальный transfer. Только ПК source/docs; Android/harness и их state
не менялись. До этого — [public source/import](TH-03-TRANSFER.md).

## Что переносится и откуда полномочия

`PublicProfileSourceRequest.purpose` явно разделяет import (default) и export.
Export получает public package по прежнему bounded маршруту, но не запускает
root/epoch import на исходном устройстве. Такой пакет может содержать current
epoch grant, нужный **чистому** получателю; обычный import всё ещё отвергает grant
без отдельного intent. Import/export nonce взаимозаменяемыми не становятся.

Source export требует intact enrolled policy/checkpoint/envelope. Fresh publisher
package должен иметь тот же signed canonical profile/tuple, что установлен;
иная wire whitespace допустима лишь при той же аутентичной canonical identity.
Installed raw bytes одновременно сохраняются в source fence. Только active local
profile signer, exact one-key public hint, никаких recovery permit/extra role
keys/device hints/secret slots. Новый профиль сначала нужно принять, не экспортировать
как уже установленный. Signature/version/clock/target/pin/history checks выполняются
через существующие metadata/store APIs; SDK проверяет profile и при необходимости
offline epoch grant одним batch. Grant.ToEpoch/public/namespace должны соответствовать
текущему profile epoch/signer. Renewed legitimate grant не переписывает source history.

Public capture/check и SDK workflow не захватывают vault. Operation/journal guards
удерживаются только на snapshot/fence и store acceptance, не на network/SDK
workflow/scrypt. Preview не сохраняет prefix или новые authority.

## Root для чистого получателя

Source retained SDK может проигнорировать присланный root той же версии. Поэтому
нельзя вычислить SHA исходного input root.json и объявить его доверенным bootstrap:
его дополнительные signing roles могли вообще не участвовать в source verification.

`rebaseExportProfile` берёт **точные verified SDK NextState.Root bytes**, проверяет
их fresh enrollment, создаёт owned package с этим root.json и удаляет старые
numeric root records. Original profile target bytes/metadata role bytes/epoch grant
сохраняются; source local role table не превращается в publisher policy. Rebased
package повторно проходит whole SDK profile/grant gate от нового initial root.

SHA текущего SDK root показывается как `destination_root_sha256`, отдельно от
source installation anchor. Это независимая информация с уже доверенного устройства,
которую пользователь сверяет на принимающем ПК; внутри ciphertext она не становится
собственным root approval. Никакой root/role source не меняется из-за rebase.

Перенос предназначен для **новой установки**. Root chain для произвольного older
enrolled client не восстанавливается и его anchor не сбрасывается; ordinary SDK
может отказать без нужных intermediate roots. Archive/old-client compatibility
остаётся открытой. У получателя current nonzero epoch создаёт actual EpochOrigin
по существующему clean-source contract, не фиктивные старые epochs. Source private
checkpoint/retired IDs/pin history не клонируются как подписанная authority.
Собственные source history/pins/epoch/origin остаются неизменными; global signed
revocation coverage/межустановочная непрерывность остаются отдельными gates.

## Подтверждение и prefix retention

One public-only draft хранит exact original downloaded bytes/intent, scalar
preview approval, source policy digest, full accepted checkpoint digest и
destination root SHA. Deadline подтверждения — min2min/profile/portable SDK/grant
lifetime. Returned preview не alias retained approval scalar fields.

`ExportPublicProfileSource(token, passphrase)` consumes именно export nonce once;
минимальная/максимальная длина фразы проверяется до consumption. Затем повторяются
source capture/SDK/application/store acceptance и destination-root comparison.
Смена policy/checkpoint/profile после preview требует нового подтверждения.

На explicit export authenticated SDK prefix удерживается прежним four-target
Keep transaction даже при позднем artifact/export отказе. Это не экспорт или PASS
профиля. Source повторно читается после собственного prefix commit; именно этот
advanced source участвует в последующих fences. Legacy export, который потерял
бы delivery proof, по-прежнему отвергается старым API.

## Секреты: только необходимые и только в ciphertext

После public verification берётся owned encrypted vault snapshot. Recording
resolver запускает existing `render.ValidateCredentials` для selectable channels:
он использует те же CredentialRefs/SecretIDs/kind aliases. `ExportReference`
возвращает реальный stored ID/kind; missing explicit ID не fallback, ambiguous
legacy kind не «первый слот». Слоты deduplicate/sort по actual IDs. Unrelated
vault values и секреты planned/disabled/revoked/unavailable channels не экспортируются;
после их отдельного включения может потребоваться дополнительное provisioning.

DPAPI reads и existing scrypt+AES-GCM Seal выполняются **вне App/journal/vault locks**.
После Seal повторяются encrypted vault source hash, exact public source/checkpoint/
profile bytes, clock/proof/profile/grant expiry и cancellation. Callback не берёт
App/runtime locks и не делает lifecycle вызовов. Stale ciphertext не выдаётся.
Snapshot admission/encryption limits прежние; это не forced KDF cancellation или
hard RSS/CPU/Windows multi-process CAS proof. Go string physical zeroization не
обещается; codec очищает свои byte buffers, exporter drops temporary slot refs.

Output содержит только encrypted SNOB1 transport и public summary: source profile
identity, destination mode/root/target hashes, actual exported slot count. Ни
plaintext slot, ни passphrase не выдаются UI/logs. Password не сохраняется в draft.
Источник profile/epoch/roles/history не изменяется, manager не перезапускается;
только independently authenticated delivery prefix может сохраниться.

## Приём и сохранение

`ProtectedProfileTransferRequest` объединяет bounded encrypted transport/passphrase
и explicit profile/enroll/recover/epoch_recover/epoch_enroll approvals. Новые
Preview/Apply wrappers используют existing encrypted Open и common App import,
без отдельного engine/role/history алгоритма. First enrollment требует отдельный
SHA256 root.json; epoch first enrollment/recovery — также exact preview target SHA.
Existing nonempty/corrupt source не становится clean device/reset permission.

Source UI показывает назначение, независимые root/target fingerprints и нужный
first-install mode. Recipient form связывает preview с exact transport/passphrase/
mode/root; изменение этих полей инвалидирует approval. Legacy normal import
сохраняет existing API. New methods идут через optional generated capabilities;
**bindings всё ещё не генерировались, новый путь сейчас unavailable**, не обходится
raw window.go/manual generated edits. Browser/native/keyboard/DPI/type acceptance
не выполнялась, прежние tests не подтверждают новые source.

`SaveProtectedProfileTransfer` использует existing owner file dialog и принимает
только bounded SNOB1 text. Необычный destination file/symlink отвергается basic
regular-file gate. Dialog error теперь не маскируется отменой, old SaveOnboardingFile
переиспользует helper. Это не Windows reparse/hardlink/ACL/atomic power-loss guarantee.
Saving после expiry не продлевает authority: получатель заново проверяет profile,
delivery/grant time/roles/history и credentials перед применением.

## Остаток полного плана

Normal Wails generation, compiler/crypto/SDK/codec/source-race/vault/KDF/GUI/native
приёмка; actual two-device epoch0/current-epoch transfer, root substitution,
retired/ambiguous/unrelated secrets, clock/prefix/corruption/expiry/cancel/oversize
сценарии — когда владелец разрешит testing. Сейчас тесты не создавались и не запускались.
Source checkpoint clone/private pin history не объявлены переносимым protocol.
V2-144 [first policy baseline](TH-03-BOOTSTRAP-POLICY.md): enrolled export требует
policy в publisher package и несёт destination policy SHA в preview/result.
Получатель подтверждает exact SHA отдельно рядом с root; export не назначает
ему approval и не доказывает unseen history/latest. Source-only, gates открыты.
Далее — bounded root/archive compatibility и publisher durable time/storage
lifecycle; P0/TH-02/04 и все9 TH stages/rollout/OS/release/comparative gates открыты.

## V2-180 — согласованное чтение и legacy parity источника

2026-10-28, source implemented/unverified. [PUBLIC-READERS](TH-01-PUBLIC-READERS.md#v2-180--coherent-export-source-и-финальный-барьер)
описывает common source component: existing-only native shared capture, полный
decoded policy/checkpoint и exact raw envelope fence. Initial enrolled capture,
read-only acceptance и post-prefix reload подключены; final vault writer использует
checkInScope без recursive lock. SDK/DPAPI/KDF не держат native reader scope.
Committed prefix outcome сохраняется при позднем source read/Close fault и
сообщается независимо от stable authority digest/cache-only изменения.

Legacy export теперь также проверяет active signature, clock/version/history,
переносимость retained bans и final public/encrypted-vault source перед выдачей.
Чистый destination rederived через existing acceptance algorithm; недостающие в
signed profile cumulative bans приводят к отказу. Local checkpoint не принимается
просмотром/экспортом. Enrolled/global policy/epoch не могут перейти в legacy.
Raw legacy JSON в transport может нормализоваться кодировщиком; canonical signature
сохраняет прежний контракт. Exact TUF bytes остаются в отдельном ProfileBytes.

Общий resolver переносит только actual credential IDs selectable channels; unrelated
vault slots больше не попадают и в legacy bundle. Изменение совместимости требует
приёмки: missing/ambiguous secrets и future-channel provisioning, старые helper/API,
native locks, fresh-time/final-race/expiry/retirement/two-device cases пока не проверены.
Final gofmt7/source hashes в export-snapshot-source-evidence.json; tests/fixtures,
build/SDK/DPAPI/native/UI/live не выполнялись. Existing bindings/generation/OS/release/
all9 gates открыты; public schema2 install unsupported.

V2-180a: exact raw envelope capture теперь использует metadata.ReadEnvelopeBytes,
тот же stored reader/writer gate, что LoadEnvelope; отдельный App reader не обходит
poisoned I/O. Final inventory расширен до7 Go paths. После финального legacy gate
нет App/log callbacks с новым ожиданием locks. Только source/gofmt, без runtime proof.

V2-181 уточняет committed outcome: successful retainDeliveryProgress может быть
no-op без transaction. Export отмечает commit только когда реально вызван
successful commit callback; late reload/Close сохраняет этот результат. Source/
gofmt4+subset1 в update-source-evidence.json; runtime/compatibility не проверены.
