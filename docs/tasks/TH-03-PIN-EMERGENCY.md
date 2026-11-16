# TH-03 — аварийная публикация списка отзывов для ПК

2026-10-24, V2-139. **Исходники изменены, поведение не проверено запуском.**
Тесты, сборка, CLI, SDK/криптография, сеть и native-действия не выполнялись.
Полная задача TH-00…08 и существующие P0/OS/release/comparative gates сохраняются.

Продолжение [приёмника и runtime-ограничений](TH-03-PIN-URGENT.md) и
[обычной публикации](TH-03-PIN-PUBLISHER.md).

## Зачем отдельный режим

Обычный выпуск требует пригодности всех объявленных application artifacts.
Когда новый глобальный запрет делает старый профиль недопустимым, это условие
может помешать доставить сам запрет. Аварийный выпуск явно содержит только
`trust/pin-history.json` и его публичную капсулу. Старые profile/update payloads
не загружаются и не получают application acceptance.

Это не восстановление профиля, root enrollment или назначение нового signer.
Установленный приёмник V2-138 требует уже enrolled lineage и intact checkpoint;
непригодный старый профиль после применения запрета сам собой не исправляется.

## Явное назначение и совместимость

- `purpose: "pin_policy"` — необязательное поле release plan, manifest и
  `PublicationHead`. Отсутствие сохраняет прежний обычный режим и сериализацию.
- Явные null/empty/unknown/alias и смешанный режим отклоняются. В аварийном
  плане/manifest разрешён ровно один artifact указанного kind и reserved path.
- Manifest hash включает purpose; private checkpoint/current/history содержат
  то же назначение. Проверка accepted candidate сравнивает его с private head.
- Старые strict decoders такой новый выпуск отклонят. Перед публикацией нужна
  миграция клиентов; выпуск target/EXE сейчас не разрешён и не выполнялся.

Форма отдельного release plan (пример контракта, не выполненный сценарий):

```json
{
  "schema": 1,
  "packages": [{ "kind": "pin_policy", "plan": "policy-package.json" }],
  "purpose": "pin_policy"
}
```

`policy-package.json` использует существующие шесть обязательных полей:
`schema:1`, `kind:"pin_policy"`, `target_path:"trust/pin-history.json"`,
явные public `.json` пути `artifact`/`authority` и metadata map с `root.json`.
Adjunct `pin_policy`/`epoch_grant` в таком плане запрещены: policy уже основной
artifact. Authority остаётся независимо одобренной scoped policy издателя;
новая application signing authority из капсулы не появляется.

## Issuer и promotion

1. Существующий issuer admission читает bounded public metadata и policy artifact
   ≤768 KiB, проверяет независимо одобренный root и один policy-only SDK batch.
2. `PreparePinPolicy`/`Retain` дают private proof и проверку срока/namespace/
   protocol/структуры. Issuer без device history не обещает continuity конкретного ПК.
3. Прежний `EncodePinPolicyCapsule` создаёт публичный пакет без профиля/секретов.
   В исходниках metadatatool добавлено действие `pin-policy-pack`, оно не запускалось.
4. Candidate verifier сверяет raw target hash, exact capsule bytes и общую metadata
   snapshot; один target не добавляется повторно в SDK batch.
5. Publisher использует прежний retained baseline, private proof, cumulative
   `CheckPinPolicyContinuity` и prefix retention. Application payload/signature
   gate пропускается только для явно отдельного policy kind; обычный выпуск
   сохраняет полный gate всех declared artifacts.
6. Новый layout имеет только policy payload/hash aliases и капсулу. Все прежние
   metadata links/signatures/root chain/freshness всё ещё проверяются, включая
   profiles/updates role metadata; их payloads отсутствуют. Просроченную metadata
   нужно перевыпустить через прежние подписи, режим не снимает expiry.
7. Сохраняются прежние admission: 32 inputs, 256 files, 16 MiB cumulative work,
   8 MiB decoded candidate files, 12 MiB outer candidate и пределы metadata.
   Удвоение metadata в капсуле/инвентаре учитывается существующими бюджетами;
   большой пакет может быть отклонён, лимиты не расширены.

После staging повторяются прежние source/clock/context/private-proof gates.
Publisher фиксирует state/current/root/timestamp прежней транзакцией из четырёх
файлов; дополнительные mutable targets и implicit reset не вводятся.

## История, supplementation, restore и выдача

- Root supplementation пересобирает капсулу с исходным policy body и обновлённым
  metadata package, сохраняет purpose и требует прежнюю повторную private проверку.
- Accepted-head decoder проверяет purpose против private checkpoint. Shared
  protected inventory/backup/restore используют этот decoder и прежние exact
  stage bytes; прошлые accepted objects остаются защищёнными даже после expiry.
  Реальные backup/restore и crash/DR не выполнялись и не доказаны.
- Исправлен отдельный пробел выдачи: после успешного отдельного prefix commit
  новый retained top-target SHA может ещё не совпадать с applied marker. Такое
  pending состояние запрещает выдавать прежний current/archives, включая cache;
  status сообщает `pin_policy_pending`, current получает unavailable, архив —410.
  Numeric indexed roots остаются доступны для sequential trust recovery.
- Как и раньше, source/permission проверяется при выборе HTTP response. Уже
  начавшаяся передача и межпроцессные/OS races не объявляются атомарно прерванными.

## Клиентская загрузка

`FetchPublicProfile` и новый `FetchPinPolicy` используют один bounded HTTP путь:
явный protected/local/bootstrap intent, owner SOCKS для protected, без redirects,
environment proxy, decompression и direct fallback; общий таймаут30s, максимум
две выборки current при410. Policy target фиксирован существующим Request gate.

Profile download на emergency head возвращает отдельный `ErrPinPolicyPublication`,
а не пытается загрузить отсутствующий profile. Policy download требует emergency
purpose, получает deterministic policy client path и проверяет форму капсулы.
Unsigned head остаётся routing hint; разрешение применить даёт только SDK и
локальный приёмник, а не успешная загрузка или release ID.

## Следующий срез и границы

V2-140 [App-owned source/file draft и UI](TH-03-PIN-UI.md) добавлены в исходники:
общий admission/cancel, exact preview/one-use approval и deadline final fence,
отдельное действие в существующем переносе профиля. Поведение unverified;
refetch/direct retry не добавлены. Native bindings генерировать обычным способом
только после разрешения исполнения; старые сборки показывают unavailable.

Открыты effective UI facts вместо raw signed counts, common grant/policy deadlines,
future Next activation wakeup, cold minimum-policy/custody/private history,
SDK/module/codec/compatibility/native/OS/crash/DR/resources/old-client/rollout/
release/comparative и остальные этапы §5.3. Форматирование Go не доказывает
типовую совместимость, подписи, работу сервера, доставку или VPN-путь.
