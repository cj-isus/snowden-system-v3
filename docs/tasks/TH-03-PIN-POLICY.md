# TH-03 — общий отзыв HY2-сертификатов и каналов

2026-10-23, V2-135. **Source-only/unverified.** Реализованы SDK/metadata-границы,
не законченная доставка или rollout. Тесты не добавлялись/менялись/запускались;
build/vet/check/module/gen/product CLI/crypto/native/live не выполнялись. Реальные
профили, сертификаты, ключи, vault и publisher store не читались.

Продолжение App/transport source — [V2-136](TH-03-PIN-TRANSFER.md); перечисленные
ниже App/codec пробелы описывают границу V2-135, не отменяют новые исходники.

## Authority и complete batch

Зарезервирован `trust/pin-history.json`: прямой top-level TUF target с прежним
offline threshold≥2, отдельно от online timestamp/snapshot и profile delegation.
Root/targets/delegation проверки не заменяются новым application signing key.
Top-level допускает только два прежних/новых reserved target: epoch и pin history.

SDK `PrepareArtifacts` допускает до трёх запросов: profile + optional epoch grant
+ pin policy. После полного Refresh наличие pin target в authenticated targets
делает его обязательным для profile batch; отсутствие bytes возвращает verified
prefix без artifact proofs. Partial batch по-прежнему не даёт subset proofs.
`Prepared.RequiresPinPolicy` — приватно зафиксированный факт, не bundle hint.

`VerifiedPinPolicy` владеет bytes до проверки proof и parse. `CheckProfile`
сверяет exact profile target/bytes, одинаковый SourceDigest и весь final NextState
обоих proofs. `ValidUntil` — minimum snapshot и SDK deadlines. Отдельное urgent
policy-only принятие должно стать самостоятельной lifecycle-операцией, а не
ослаблением complete-batch правила.

## Формат и неизменяемая история

Snapshot schema1/purpose `snowden-system/pin-history/v1`, namespace `profiles/`,
minimum_protocol1; version — uint64>0, issued_at/expires_at проверяются с existing
clock skew. Строгие exact поля, без null/unknown/duplicates/aliases/trailing JSON.
До clone/parse действует 768KiB; sorted unique lowercase leaf DER SHA256≤8192,
portable retired ChannelID≤1024. Это отзыв leaf fingerprints и IDs, не CA/SPKI
или application signing keys; для смены других trust roles остаётся epoch/recovery.

Accepted checkpoint сохраняет version/raw-target SHA/time/global lists в optional
`pin_policy`. Monotonic merge запрещает version/IAT rollback, удаление старых
запретов и другой raw target под той же version. Expiry не отменяет уже принятый
отзыв; новый snapshot всё равно требует свежего приватного SDK proof.

Retired IDs объединяются с private local history. Global certificate bans действуют
между IDs; duplicate local bans удаляются только после сохранения в non-removable
global set, все per-channel continuity markers остаются. Общий budget8192 и
checkpoint1MiB не расширены. При переполнении отказ, без eviction старых запретов.
Clock floor включает policy issued_at, observed time использует прежний minute
checkpoint interval. Обычный runtime writer не может назначать/менять/снимать
policy authority; epoch/origin/recovery advance и deep Clone сохраняют marker.

После marker enabled HY2 без signed leaf pins не допускается. Возврат revoked
fingerprint запрещён, кроме explicit signed `HY2Pins.Revoked`, который уже
исключается renderer. Retired ID нельзя вернуть через другой profile/transport.

## Исправленный дефект continuity

`accepted_profile.go` сравнивал Transport с `hysteria2`, хотя channel schema
задаёт Protocol=`hysteria2`, Transport=`quic`. Это могло отклонять корректный
повторный профиль с retained pin marker. Условие исправлено по реальному
`channel.go:ValidateChannel`. Это вывод из исходников, не воспроизведённый тест.

## Обязательное продолжение до rollout

1. Common App import/export и public/sealed transports: policy bytes, same-batch
   proof, exact preview/apply/source/time fences, fresh enrollment и existing
   history. `MaxBundleSize` сейчас **256KiB**, policy bound768KiB: нельзя просто
   расширить secrets/KDF ingress; нужен отдельный bounded public policy adjunct
   или явно версионированный общий формат с суммарным admission budget.
2. Issuer/repository/publisher: authoring/signature collection, exact target
   inventory, release candidate/source/promotion/root-package/backup/restore;
   full profile+update+epoch+policy batches требуют отдельного согласования bounds.
3. Urgent policy-only acceptance: до fixed4 commit остановить/инвалидировать
   unsafe Manager и pending old operation; после ошибки не возвращать unsafe
   session или direct fallback. Проверить весь installed profile и допустимый
   failover. `Apply` сейчас только готовит owned value, файлов не пишет.
4. V2-144 [PC bootstrap baseline](TH-03-BOOTSTRAP-POLICY.md) требует независимый
   exact policy SHA рядом с root при public/encrypted первой TUF-установке;
   cumulative public запреты сохраняются без клонирования private ledger.
   Source-only; unseen/pre-policy history, latest-known/offline custody и
   legacy unenrolled migration при полной потере истории остаются границами.
5. PC UI: policy version как строка, facts/deadlines/unknown; обычная генерация
   bindings и tests/build/native/old-client/DR/release приёмка после разрешения.

**Новый target не публиковать до завершения клиентской и publisher цепочки.**
App/issuer codecs policy передают в исходниках V2-136…144, без execution proof. Старые клиенты могут отвергнуть
новый top-level target/checkpoint; обновление клиента и migration precede rollout.
SDK foundation не доказывает реальное применение/остановку потока или скорость.
Все TH-00…08 и оставшиеся P0/ОС/DR/comparative gates сохраняются.

## Исходники

- `windows/backend/delivery/{model,policy,client}.go`.
- `windows/backend/metadata/{pin_policy,pin_policy_checkpoint,accepted_profile,clock,epoch_enrollment}.go`.
- Предыдущий срез: [approved publisher restore](TH-03-RESTORE.md).

## V2-183 — согласованный urgent receiver

2026-10-28, source implemented/unverified. [PUBLIC-READERS](TH-01-PUBLIC-READERS.md)
фиксирует coherent trust/checkpoint capture, frozen expected source после own
prefix, read-scoped cumulative denial acceptance и fresh preview gate. Vault и
старый envelope не требуются для public preview; SDK работает вне profile locks.
Final checkpoint-only fixed4 commit повторяет public+encrypted-vault source и
proof/clock/deadline внутри существующего writer без recursive reader.

Retire-before-obligation, private enrolled SDK proof и cumulative bans/ChannelID
continuity прежние; no source/package/approval flag даёт signing/enrollment authority.
Общий prefix helper проверяет coherent source и на no-op, а successful commit
callback отличает cache-only persistence от отсутствия новой transaction. Поздняя
ошибка не скрывает отдельно сохранённый prefix. Новый target всё ещё не публикуется.

Gofmt4+comment1/source hashes в pin-source-evidence.json; tests/fixtures/build/
SDK/DPAPI/native/live не выполнялись. Field/OS/runtime/old-client/DR/release/
comparative/all9 приёмка открыта, schema2 install unsupported. Далее public file
intake receipts без подмены trusted-store policy, directory durability/SDK I/O.
