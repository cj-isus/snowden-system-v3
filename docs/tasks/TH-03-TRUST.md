# TH-03 — полномочия ключей и офлайн-восстановление профиля

2026-10-21, V2-118: **source implementation, unverified**. Compilation также
не проверялась. Tests не добавлялись/менялись/запускались, build/vet/harness,
experiments/native/worker/live не выполнялись. Все TH-00…08 остаются в объёме.

Продолжение выбора библиотеки и bounded verifier — [V2-119](TH-03-TUF.md).
Это ещё не подключение TUF к App, root persistence/rollout остаются открытыми.

## Основание и решение

Раньше `trusted_keys.json` означал одинаковые полномочия для channels и EXE.
Onboarding объединял присланные ключи с установленными до проверки envelope:
подтверждение переноса профиля одновременно расширяло code-signing authority.
Комментарий `update-signing` не был ограничением криптографического verifier.

Разделение полномочий и отдельный корень — требования зрелой модели
[TUF 1.0.37, §2.1](https://theupdateframework.github.io/specification/latest/#roles-and-pki).
Здесь реализован **локальный role policy существующего envelope-протокола**,
не TUF root/client и не новый криптоалгоритм. Используются прежние Ed25519/JSON
и стандартные Go crypto primitives. Полный TUF workflow/threshold root rotation
остаётся отдельным решением OFF-06/COM-14; unsafe local refresh не включался.
Сохранённый go-tuf HEAD требует Go1.27, проект закрепляет Go1.26; библиотека
и зависимости этим срезом не устанавливались/обновлялись.

## Явные полномочия

| Роль | Разрешение | Не даёт |
|---|---|---|
| profile | Подписывать профили существующего доверенного publisher | Заменять EXE или выдавать recovery-разрешения |
| update | Подписывать приложение для существующего update verifier | Подписывать профили и менять recovery authority |
| recovery | Разрешить точную замену profile publisher через отдельное подтверждение | Менять update/recovery/root authority, сбрасывать историю |

`metadata/trust_policy.go`: schema2, одна роль на ключ; bounded public reader
≤128KiB/256 records, regular/SameFile, strict property/duplicate/null/ID/role
checks. Retired records сохраняются. Публичные карты возвращаются собственными
копиями. Digest authority не включает comments, сериализация сохраняет их.

Schema1 читается только как **profile authority**; даже `comment=update-signing`
не разрешает EXE. Новый update требует отдельного явно provisioned update-ключа.
SaveKeys и legacy CLI helper не могут сплющить schema2 обратно в общую карту.
Это сознательное изменение совместимости: старые strict clients schema2 не
понимают; нужен согласованный rollout, новый EXE здесь не собирался.

Обычный import допускает initial profile enrollment только на пустой установке
без accepted history/envelope/ключей. На существующей установке присланные
ключи обязаны уже иметь active profile-role. Нет объединения arbitrary incoming
key с code/recovery authority, нет автоматического bootstrap после потери файла.
Export переносит только active profile-ключи и сообщает об этом; роли update/
recovery на новом устройстве нужно provision отдельно. Это не полный trust backup.

## Recovery-разрешение

`metadata/profile_recovery.go`: domain-separated Ed25519 permit содержит:

- исходные version, canonical profile SHA256 и authority SHA256;
- точные target version/hash и public key нового profile publisher;
- issued/expires, ID уже установленного recovery-ключа и signature.

Новая версия строго выше исходной. Часы и retained floor проверяются; новый
envelope отдельно подписан целевым profile-ключом, hash/version/key привязаны
к permit. Recovery-ключ должен быть provisioned **до** отказа/компрометации.
Присланный вместе с bundle recovery-ключ такого права не получает.

При применении все прежние profile-ключи переходят в retired history, новый
ключ получает только profile-role. Update/recovery records сохраняются.
Уже retired либо другой роли ключ не переиспользуется. Старый installed envelope
проверяется историческими profile-ключами только как якорь accepted hash/history;
новые профили старым ключом не принимаются.

Accepted checkpoint сохраняет `last_recovery_sha256`; exact committed replay
того же permit/target не создаёт новых полномочий. Последующая новая версия,
иная исходная authority либо другой target требуют актуального разрешения.
Retired channel IDs, глобальные pin revocations, version/hash и clock floors
не сбрасываются. Recovery может восстановить missing/corrupt channels-файл при
целом accepted checkpoint; потерянный/повреждённый checkpoint не превращается в0.

## App и issuer

- `onboarding_authority.go` выделяет решение о полномочиях из transaction flow.
  Ordinary и recovery intent различаются. Обычный импорт с permit отвергается.
- `PreviewProfileRecovery` показывает signer/full SHA256 fingerprint, target,
  issuer ID и retired publishers. `ApplyProfileRecovery` требует explicit confirm
  и target SHA256 из превью, повторно выполняет подготовку. `GetProfileRecoveryContext`
  возвращает только публичные source bindings/floor/доступные recovery IDs.
- Bundle optional `recovery` ограничен общим container budget и отдельными16KiB.
  Старые bundles без поля сохраняют формат. Новое поле старый strict reader
  отвергнет. Пятый import target не добавлялся; permit identity в прежнем version.txt.
- CLI `metadatatool keygen ... [profile|recovery]` и `updatetool keygen ...`
  используют общий `tools/internal/publisherkeys`: сохраняют другие роли,
  private output только O_EXCL, Sync до public publication; observed source
  change отказывает, незавершённый private output сохраняется с ошибкой.
  Provisioning — offline при остановленном приложении, не hot trust editing.
- `metadatatool recovery-sign <permit.json> <@offline-recovery.key> <new-output.json>`
  читает bounded unsigned plan, применяет domain signature, публикует только новый
  output существующим publisher helper. CLI не запускался, ключи не создавались.
- UpdateApp и updatetool выбирают только RoleUpdate; metadata runtime — active
  RoleProfile. Ошибка authority/signature в runtime — typed trust fault без
  channel-health FAIL и бессмысленного network retry/wakeup.
- Новые backend methods/preview fields ещё не соединены с recovery UI. Wails
  generated bindings вручную не менялись; generation/native/UI gates открыты.
  Не представлять API как законченный пользовательский recovery-сценарий.

## Подготовка, retirement и commit

При чтении обнаружены противоположные порядки App→vault у обычных bindings и
vault→App у прежнего import retirement. Фактический deadlock не воспроизводился.
В исходниках последовательность изменена:

1. Под App operationMu + journal serialization готовится authority/checkpoint и
   owned vault snapshot; все refs/значения валидируются до lifecycle mutation.
2. Journal и vault locks освобождаются перед `retireProfileSession` и App calls.
3. После retirement journal reacquire проверяет source policy, schema/comments
   и checkpoint. Vault `WithPreparedSnapshot` сравнивает private encrypted source
   digest под своим mutex. Изменившийся источник отвергается, не перезаписывается.
4. Часы/срок profile и permit повторно проверяются после retirement; floor
   обновляется в staged checkpoint. Только затем прежний four-target commit.
5. Vault/journal освобождаются до error publication/finish/нового fenced Start.
   Ошибка после retirement оставляет transition blocked, protection не снимается.

Export использует тот же owned vault snapshot, сериализован с profile import;
App mutex не удерживается при DPAPI/KDF/file I/O. Логи вызываются после release
journal. Это source-level исправление порядка, не concurrency/Windows proof.

## Что остаётся

- Root enrollment/rotation, multiple-signature thresholds, timestamp/snapshot/
  targets workflow, authenticated freshness и политика client/root epochs при
  compromised-publisher fast-forward ещё не реализованы. Один offline recovery
  key — самостоятельная граница компрометации, не threshold/quorum система.
- Corrupt/lost checkpoint, потеря recovery private key, local admin/ACL/reparse,
  interprocess races и power-loss требуют отдельных процедур/приёмки. Сброс
  version/time/revocation history не добавлен как универсальный выход.
- Recovery UI, Wails generation, issuer plan/bundle UX, old-client migration,
  actual owner enrollment/rotation, protected new-session path не проверены.
- Private snapshot fence действует в одном Manager/process. Private outputs,
  directory durability, malicious path substitution и concurrent CLI не получили
  доказанную Windows atomicity. Hash/stage file — не OS gate.
- Все compile/unit/concurrency/native/release/comparative gates открыты.
  Tests пока запрещены владельцем; старые PASS не относятся к V2-118.

Следующий срез: выбрать совместимый release go-tuf/ADR для root/threshold/freshness
и root epoch, завершить issuer/recovery product contract. Не заменять библиотеки
своей реализацией полного TUF. TH-02/04 и TH-05…08 остаются в общей очереди.
