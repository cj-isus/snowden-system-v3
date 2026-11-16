# TH-03 — pin policy в PC import/export и ограниченном переносе

2026-10-23, V2-136. **Source-only/unverified.** Продолжение
[SDK/metadata foundation](TH-03-PIN-POLICY.md). Тесты не добавлялись/менялись/
запускались; build/vet/typecheck/check/module/gen/SDK/crypto/product CLI/native/live
не выполнялись. Реальные profiles/certificates/keys/vault/publisher/config не читались.

Продолжение issuer/publisher и same-epoch reference — [V2-137](TH-03-PIN-PUBLISHER.md).
Ниже сохранена граница V2-136; текущий остаток указан в STATE и новом срезе.

## Два независимых бюджета

`BundlePayload.PinPolicy` переносит original public bytes в base64-поле
`pin_policy`. Public profile имеет верхнюю границу `256KiB + base64(768KiB) + 128`
bytes; часть без policy по-прежнему проходит прежние payload и JSON256KiB limits.
Без adjunct сохраняется и прежний raw input256KiB. HTTPS downloader/common public
entrypoint используют новый общий public cap, все routing/redirect/time gates прежние.

При encrypted transfer `Seal` сначала отделяет owned policy bytes. Credential
core остаётся **SNOB1**, с прежними256KiB plaintext/transport/gzip и KDF limits.
Только при наличии policy он оборачивается в **SNOB2**: base64url JSON с exact
`schema:2`, `protected:SNOB1…`, `pin_policy:base64…`; outer cap2MiB. Внешний adjunct
не зашифрован и не становится authority от близости к ciphertext.

Open проверяет outer bounds/shape до KDF: без null/unknown/duplicates/case aliases/
nested SNOB2; один inner SNOB1. Внутреннее plaintext поле pin_policy запрещено
включая null/case aliases; нельзя получить второй источник policy. Policy требует
delivery package. Legacy SNOB1 сохраняет исходный размер **до TrimSpace**.
File load/save используют общую проверку bounded framing без расшифровки; это не
подтверждение секретов или подписи. Реального Seal/Open/сохранения не выполнялось.

## Одна App authority и persistence цепочка

`prepareBundleDelivery` запрашивает profile + optional epoch + policy в одном
SDK batch. Common `check` повторяет exact profile/grant и privately verified
policy со всем SourceDigest/final NextState; нельзя обойти policy через recovery
или fresh root approval. Обычный signer/epoch/enrollment выбор сохраняется.

После успешной обычной/fresh-epoch/recovery подготовки `ApplyProfile` проверяет
identity **уже авторизованного** acceptance, накладывает monotonic policy и снова
проверяет pin/ID continuity этого exact delivered profile. Helper не назначает
signer/epoch/root, не пишет файлы и не запускает engine. На новом устройстве это
cumulative public policy, не копия private local ledger.

Изменение policy SHA запускает существующий `retireProfileSession`, даже когда
сам профиль/ключи/секреты прежние. Затем тот же fixed4 commit: keys, envelope,
checkpoint, vault; error/finish следует existing blocked transition. Это общий
profile import, **не** ещё отсутствующий urgent policy-only приём.

Общий public-source/clock/cancel/profile/grant/policy/recovery fence выделен и
повторяется после retirement и через existing `filetxn.BeforeCommit` **после**
полного synced staging до journal publication. Callback не изменяет targets и
не берёт App mutex; внутри vault callback читает только public state. Native
locking/interprocess/crash/reboot/durability остаются непроверенными.

## Export, preview и frontend boundary

Enrolled export проверяет новый snapshot также против private source acceptance,
не записывая pin history на исходном ПК; затем rebases только verified root и
deep-copies public policy. Fresh-origin proofs и final expiry checks сохраняются.
Export deadline учитывает snapshot expiry; фактические credential IDs выбирает
прежний renderer/vault path. SNOB2 содержит те же выбранные зашифрованные slots.

Preview содержит policy version **строкой uint64**, SHA, expiry, authenticated
required flag и current retained policy SHA. Downloaded preview/apply comparison
включает эти identities, не теряет изменение policy при той же profile version.
Frontend API/contract сохраняют optional old-build unknown, проверяют policy
version без Number conversion и допускают SNOB1/SNOB2 encrypted output. Новые
native bindings не генерировались/не редактировались вручную; visual UI facts,
old-client/codec/binding/runtime приёмка не выполнены.

## Следующий необходимый срез

1. Issuer authoring и delivery-pack: bounded pin-policy path, проверка schema/
   application constraints/expiry, original target bytes, report/inventory.
2. Repository draft/release/candidate/promotion/retained prefix, root-package и
   publisher HTTP/archive/backup/restore: один exact policy artifact на все
   profile targets; согласовать bounded batch, не ослаблять partial-proof gate.
3. Urgent policy-only receiver: отдельный exact proof/preview/source fence,
   сохранение bans при неприемлемом profile, unsafe session/pending operations
   retirement до fixed4 commit, выбор разрешённого failover и blocked errors.
4. Bootstrap minimum-policy/custody/unseen history, deployment/client migration,
   UI facts/normal binding generation и вся отложенная verification.

**Не публиковать новый TUF target до полного issuer/publisher/client пути.**
Никакой network/deploy/admin/OS операции этот срез не выполнил. Все девять TH,
P0 safety, TH02/04…08, DR/resources/native/release/comparative gates остаются.

## Исходники

`windows/backend/onboarding/{onboarding,pin_transfer,codec,public_profile,source}.go`,
`windows/backend/metadata/pin_policy_checkpoint.go`,
`windows/onboarding{,_delivery,_authority,_public,_import,_dialogs,_export_proof}.go`,
`windows/frontend/src/api/{backend,contract}.ts`.
