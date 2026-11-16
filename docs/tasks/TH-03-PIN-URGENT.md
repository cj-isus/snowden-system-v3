# TH-03 — policy-only PC receiver и runtime restrictions

2026-10-24, V2-138 (начат 2026-10-23). **Source-only/unverified.** Продолжение
[issuer/publisher](TH-03-PIN-PUBLISHER.md). Тесты не добавлялись/менялись/
запускались; compile/build/vet/typecheck/check/module/gen/SDK/crypto/product CLI/
HTTP/browser/native/live не выполнялись. Реальные owner profiles/certificates/
keys/vault/publisher/backup/runtime config не читались. Только PC source/docs.

## Durable obligation в существующем checkpoint

`delivery.State.PinPolicyTargetSHA256` читает target SHA из **retained** top-level
SDK cache. Это observation защищённого локального состояния, не новая проверка
подписи и не proof для входящего artifact. `CheckAppliedPinPolicy` сравнивает его
с applied marker. Новый/удалённый target при несовпадении marker блокирует runtime
с `pin_policy_pending`; известные bans не сбрасываются. Для legacy без наблюдения
target и без marker сохраняется прежний путь; unseen history не объявляется известной.

После **успешного** отдельного prefix commit новый target hash переживает отказ
последующего policy commit: его rollback возвращает old marker рядом с уже новым
cache, и runtime остаётся blocked. Дополнительного файла/пятого target нет.
Физическая durability/power loss/ACL/interprocess и невозможность записать сам
prefix остаются отдельными OS/FR-009 gates; не обещается сохранение невыполненной
записи. До успешного prefix commit действует in-process retirement/error ownership.

Expiry snapshot не снимает уже применённые bans. Fresh SDK/shape/time/lineage
проверка по-прежнему нужна для следующего принятия. Retained hash не позволяет
обходить её parsed JSON/boolean/unsigned hint. Сверен existing adapter cache flow;
upstream v2.4.2 source не найден в локальном module cache, download/tooling не
запускались. SDK cache persistence contract и фактическая совместимость не проверены.

## Exact installed runtime profile

`Store.LoadRuntimeProfile` проверяет active profile signer, validity/clock/version
и при global policy — exact ранее committed tuple/content SHA. Он не принимает
новый профиль, не меняет epoch/origin/history и не переписывает signed bytes.
`PrepareAcceptance`/`ApplyVersion` сохраняют строгий incoming-profile контракт.
Legacy/bootstrap использует прежний strict acceptance/unchanged-write guard.

Owned runtime projection только уменьшает разрешения:

- signed revocations сохраняют прежний skip; retained retired IDs выключены;
- при global policy legacy HY2 без signed pins выключен;
- local/global certificate bans добавляются только к runtime-копии Revoked для
  Current/Next, уже подписанных в профиле; новые leaf/hostname/endpoint не добавляются;
- если Next уже допустим, остаётся его исходный сертификат; если разрешённого
  active pin нет, канал выключен. Other eligible channels не отбрасываются;
- signed-disabled не включается; validation evidence не повышается, времени
  или подписи нового профиля projection не придумывает.

Reasons `channel_revoked`, `signed_pin_required`, `certificate_filtered`,
`certificate_unavailable`, `revoked_certificate_unavailable` — local restriction
facts, не результаты network probe. Snapshot содержит policy SHA/version и reasons;
signature provenance относится к исходному профилю, ограничения — отдельный overlay.

Core не читает manual legacy PEM для выключенных каналов. Нет selectable при
applied policy — typed trust error, включая expiry между snapshot и render.
Clock checkpoint running config дополнительно сверяет policy SHA; старый config
не освежает proof после pending/смены marker. Safe startup/failover по-прежнему
проходит fresh renderer/Manager/protected probe, не объявляется network PASS.
Автоматическое пробуждение к будущему Next.NotBefore ещё требует отдельного TH-04 среза.

## Общие import/export transitions

Если SDK prefix объявляет unapplied policy, common profile import сначала
освобождает journal lock и retires old session/operations, затем отдельно сохраняет
prefix, даже если artifact gate отказал. Один transition ID повторно не retire-ится;
ранние ошибки/panic после retirement завершаются blocked, без старой сессии.

Enrolled export тоже retires при таком наблюдении и отказывает до применения policy;
он не назначает bans как побочный эффект переноса. Подтверждённый prefix/error
учитываются отдельно. `retainDeliveryProgress` повторяет весь public-source/checkpoint
fence через existing BeforeCommit после synced staging, под прежним vault snapshot
и fixed4 ownership. App mutex в callback не захватывается.

## Bounded public capsule и App API

Capsule exact schema1/purpose `snowden-system/pin-policy/v1`/policy/package, без
profile/secret/key/root-enrollment approval. Общий cap — encoded metadata12MiB +
base64(policy768KiB) +512; policy wire string ограничена **до** decode, package
использует прежние8MiB/64 records/12MiB encoded bounds. Null/unknown/duplicates/
case aliases/numeric byte-array alternative не допускаются. Encoder/parser не proof.

`PreviewPinPolicy` read-only; `ApplyPinPolicy` требует confirm и exact approval
SHA public trust, whole checkpoint, whole capsule и raw policy. Требуется уже
enrolled lineage и intact accepted checkpoint; bootstrap/root approval не выводится
из capsule. Version/root version возвращаются строками, validity — min private
proof/snapshot. Preview не заявляет, что старый profile или runtime работает.

Один policy-only SDK target batch независим от old envelope expiry/corruption/
наличия файла. При policy change/pending transition: retire вне journal lock →
separate prefix commit → fresh private policy proof + monotonic Apply → fixed4
commit только нового checkpoint, envelope/keys/vault Keep. Credential values не
загружаются/не расшифровываются; encrypted snapshot нужен для прежнего vault fence.
Whole source/clock/cancel/proof повторяются после synced staging. Error оставляет
blocked; success планирует fresh Start с новым projection и probe. `Committed`
не означает connected/live; postcommit failure отдельно сообщает, что policy уже
сохранена. Bans сохраняются даже при непригодном старом профиле;
такой профиль не продлевается и не чинится самоподписью.

## Следующий обязательный срез

1. V2-139 [policy-only emergency publication](TH-03-PIN-EMERGENCY.md) добавлена
   в исходники issuer/publisher/candidate/head/supplementation и bounded fetch.
   Ordinary full release сохраняет gate profiles/updates. Обе ветки unverified;
   App-owned downloaded draft/file/UI и rollout остаются открытыми.
2. V2-140 [source/file/cancel/approval UI](TH-03-PIN-UI.md) добавлены в исходники,
   unverified. V2-141 [read-only authenticated facts](TH-03-PROFILE-FACTS.md)
   заменяют raw signed-only counts source-only; native/UI приёмка открыта.
3. V2-142 [common deadlines](TH-03-IMPORT-DEADLINES.md) и V2-143
   [future Next wakeup](TH-03-PIN-ACTIVATION.md) добавлены source-only.
   Bootstrap minimum-policy/custody/private history; SDK/modules/codec/runtime/race/OS/crash/DR/old-client/resources/
   rollout/release/comparative gates и все TH-00…08 остаются открытыми.

Ни SDK, ни capsule encode/decode, ни новый App method не выполнялись. Не разворачивать
target/EXE до полной цепочки и разрешённой проверки; native bindings не правились вручную.

## Исходники

`windows/backend/delivery/pin_policy.go`;
`windows/backend/metadata/{pin_policy_binding,runtime_profile,clock}.go`;
`windows/backend/render/{snapshot,render}.go`;
`windows/backend/onboarding/policy_capsule.go`;
`windows/{pin_policy_import,delivery_state,onboarding_import,onboarding_export,corebridge,trust_lifetime}.go`.
