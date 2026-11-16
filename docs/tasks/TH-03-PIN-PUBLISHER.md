# TH-03 — pin policy в issuer, release и retained publisher

2026-10-23, V2-137. **Source-only/unverified.** Продолжение
[PC transfer/App](TH-03-PIN-TRANSFER.md). Тесты не добавлялись/менялись/запускались;
compile/build/vet/typecheck/check/module/gen/SDK/crypto/product CLI/HTTP/native/live
не выполнялись. Реальные owner profiles/certificates/keys/vault/publisher/backup/
runtime config не читались. Только Windows source/docs, gofmt изменённых файлов.

Продолжение urgent PC receiver/runtime — [V2-138](TH-03-PIN-URGENT.md).
Ниже сохранена граница V2-137; emergency publisher остаётся следующим срезом.

## Общие metadata-границы

Из существующего Apply выделены `CheckPinPolicyContinuity` и private-proof
`VerifiedPinPolicy.Retain`. Первый проверяет **значения**, не назначает authority;
второй требует свежий exact SDK proof из той же lineage. Нельзя удалить marker,
понизить version/IAT, убрать bans или заменить raw target при той же version.
`RetainedPinPolicy.CheckEnvelope` — общий global certificate/ID constraint для
App, issuer и publisher; подпись, время профиля, private device history, secrets
и lifecycle по-прежнему проверяются отдельными прежними границами.

`CheckArtifact` связывает policy с profile **или update** по одному SourceDigest
и всему final SDK state; application role check остаётся обязательным. На один
artifact максимум три запроса: profile + optional epoch + policy; update + policy
не требует увеличения лимита. Partial batch по-прежнему возвращает только prefix.

## Issuer и authoring

Existing delivery-pack plan допускает optional public relative `.json` поле
`pin_policy`. Читаются original bytes≤768KiB; reserved offline top-level target
получает прежние threshold≥2 signatures через существующий draft/custody flow.
Новых ключей, transport dependencies, каталогов secrets и signing roles нет.

Assembly проверяет complete SDK batch, snapshot shape/freshness и profile bans.
Public profile получает original adjunct, update package сохраняет прежний wire
format; optional update-plan policy даёт общий artifact для полного release без
profile inputs. Publication getter возвращает owned copy policy bytes.

Report: version строкой uint64, SHA, raw byte count, обязательность **для profile**;
deadline включает policy snapshot/SDK expiry. Raw policy участвует в прежнем
cumulative release work budget16MiB, даже если update transport её не содержит.
Private final Check повторяет policy binding/freshness и profile constraints;
epoch grant больше не завершает Check раньше проверки policy.

## Exact release/candidate inventory

Release собирает один `trust/pin-history.json`; разные bytes между packages
отвергаются. Каждый profile transfer обязан нести exact common policy bytes.
Candidate lookup получает reserved raw target по top-level metadata SHA лишь как
**непроверенный указатель**, затем обычный SDK подтверждает authority каждого
profile/update batch. Parsed JSON, candidate self-hash и unsigned hints не proof.

Layout различает epoch/pin reserved kinds, сохраняет exact target bytes и все
допустимые hash aliases. Полный rebuilt manifest/file inventory/release identity
должен совпасть с independently approved candidate. Root supplementation
сохраняет adjunct и повторяет current/known-old-root проверки с retained bans.
Budgets8MiB files/12MiB candidate/32 inputs/256 files не расширялись; исчерпание
приводит к отказу, не к eviction или ослаблению checks. RSS/время не измерены.

## Publisher continuity, serving и backup

Optional strict `publisherState.pin_policy` содержит cumulative retained marker;
его выдаёт private verification результата полного выпуска. После первого
принятия последующие releases не могут убрать target/marker, bans или понизить
историю. Source/prefix/role-policy/serving-time операции сохраняют marker точно;
без promotion его изменение запрещено. Existing fixed4 journal и post-staging
source/time/validity/input/inventory fences сохраняются, общий state budget прежний.
Publisher clock floor включает accepted policy IAT.

Accepted-view decoder извлекает только historical shape/SHA из **точного private
head-bound candidate**, не назначает authority по найденным файлам. HTTP/current
или archive view с другим policy SHA получает410; cache хранит policy identity.
Даже ещё не истёкший старый profile package не должен дать новому устройству
обойти latest accepted bans. Numeric privately indexed root route остаётся
доступным для последовательной client root continuity. Уже скопированный/начатый
HTTP response не становится отзывным; клиент повторяет собственную acceptance.

Status показывает retained version/SHA и `publication_policy_superseded`, не
объявляет native/client/external health. Старые heads/objects остаются protected:
shared checkpoint-derived inventory, backup и independently approved restore
принимают новый strict marker. Repair existing target сохраняет его точные bytes,
не заменяет новым/старым backup marker. Cold restore всё ещё требует independent
latest-known custody; unseen lost history не доказана. Реальный backup/DR не выполнен.

## Исправленная стыковка текущей эпохи

Portable issuer package содержит current epoch grant для нового устройства.
Раньше обычный App import отвергал любой такой grant до проверки неизменного
доверия. Теперь допускается **reference**, если установка уже enrolled, retained
epoch>0 совпадает с profile/grant, namespace прежний и public key уже активен
для текущего signer. Same-batch grant/profile proof обязателен. Обычные signature,
counter/identity/history/pin checks не пропускаются. Epoch/origin/last transition
marker и роли не меняются; initial/next epoch/recovery требуют прежнего явного intent.
Preview/approval bind SHA реально вложенного grant, не только старого origin.
Это исправление по исходникам, не воспроизведённая runtime-проверка.

## Следующие обязательства

1. **Urgent policy-only publisher/receiver**, независимо от приемлемости профиля:
   existing full-release gate сейчас требует допустимые application artifacts.
   Нужен bounded public capsule/политика публикации без unsafe profile, exact
   private proof/source/preview, durable SDK prefix и cumulative bans.
2. До fixed4 client commit остановить unsafe current/pending session. Retained
   signed profile может стать непригодным: разобраться с existing metadata/load/
   render boundaries, чтобы явно блокировать revoked channels и разрешать только
   доказанно безопасный failover без изменения signed bytes/direct fallback.
3. Bootstrap minimum-policy/custody, policy/reference deadlines и UI facts,
   normal native binding generation; compiler/SDK/codec/race/crash/OS/DR/old-client/
   resource/rollout/release/comparative acceptance отложены.

**Новый target/EXE не разворачивать до завершения urgent/client migration и
разрешённой проверки полной цепочки.** Все TH-00…08/P0/TH02/04…08 сохраняются.

## Основные исходники

`windows/backend/metadata/{pin_policy_continuity,pin_policy_checkpoint,pin_policy,accepted_profile}.go`;
`windows/tools/internal/deliverypack/{plan,assemble,publication,run}.go`;
`windows/tools/internal/repository/{draft,release_pin_policy,release,release_layout,candidate,release_supplement_layout,promotion_verify,promotion_store,publisher_history,publisher_prepare,promotion,release_supplement,publisher_view,publisher_http}.go`;
`windows/{onboarding_authority,onboarding_import}.go`.
