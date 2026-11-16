# TH-03 — отдельное превью и применение отзывов на ПК

2026-10-24, V2-140. **Реализация в исходниках, без проверки запуском.**
Тесты не добавлялись, не менялись и не выполнялись. Сборка, typecheck,
генерация bindings, SDK/криптография, CLI, браузер, HTTP/native/live не запускались.
Все девять этапов §5.3 и обязательства P0/OS/release/comparative сохраняются.

Основа: [приёмник V2-138](TH-03-PIN-URGENT.md),
[отдельная публикация и fetch V2-139](TH-03-PIN-EMERGENCY.md).

## Один владелец ожидающего пакета

`beginPublicSource` переиспользует существующий `App.publicProfiles`: один
admission, один request ID/cancel и один public-only pending draft для profile
import/export либо policy network/file preview. Новый App-level mutex/store
или второй параллельный pending buffer не вводятся.

Новый запрос после завершения предыдущего заменяет старое превью. Cancel
обнуляет только совпадающий request ID и pending; он не освобождает busy до
фактического возврата владельца. Поздний cleanup не может обнулить successor.
Approved apply остаётся отдельной операцией, которую Cancel не откатывает.

Pending policy хранит exact capsule string и отдельную копию scalar
`PinPolicyPreview`/approval. В UI уходят только версия, отпечатки, сроки и token;
тело капсулы, credentials и application authority не передаются через UI.
Nonce создаётся прежним CSPRNG helper, UI request ID служит только корреляции.
Одноразовое подтверждение поглощается и при ошибке/expiry.

## Получение из источника или файла

- `PreviewPinPolicySource` требует reserved policy target, использует V2-139
  bounded fetch. Protected owner/session/generation/own-EXE split fence
  повторяется до/после download и после private SDK preview.
- `PreviewPinPolicyFile` выбирает публичный `.json` в native dialog в том же
  admission. Отмена возвращает null без ошибки/сохранения. Explicit regular
  `Lstat`/opened `Stat`/`SameFile`, size и `LimitReader(max+1)` ограничивают чтение.
  Путь не сохраняется как authority; принимается exact полученный byte snapshot.
- Содержимое проходит общий already-enrolled V2-138 preview. Source child
  context распространяется на SDK; cancellation/closing проверяется после
  ожидания operation mutex и до публикации pending draft.
- Общий source context45s и SDK30s — cooperative. Native file dialog/open/read,
  journal mutex и враждебные filesystem races не объявляются жёстко прерываемыми
  или доказанно bounded по wall-clock; это отдельные native/OS gates.

## Применение и срок намерения

Preview deadline равен минимуму двух минут и private policy/proof ValidUntil.
UI показывает срок, держит один owned timer, запрещает истёкшее подтверждение
и отменяет pending при expiry. Backend остаётся единственным authority gate.

`ApplyPinPolicySource` получает только token + explicit confirm. Network/file
не перечитываются. Общий receiver повторяет source trust/checkpoint/capsule/
policy approval, private SDK/monotonic gates и fixed4 lifecycle V2-138.
Deadline проверяется после ожидания operation mutex и в final fence до и после
synced staging. Expiry не может превратить старое подтверждение в новый импорт.

Если после подтверждения уже получен новый authenticated prefix, он по-прежнему
сохраняется отдельно после retirement даже при поздней ошибке/expiry финального
apply. Это не применение policy body: pending restriction оставляет старый
runtime blocked. Не заявляется откат отдельно сохранённой authority/запретов.

## Компонент и контракт

Отдельный `PinPolicySource.vue` включён в существующий `ProfileTransfer.vue`
рядом с public profile flow, внутри прежнего контейнера, без новых menu/tab
дубликатов и UI framework. Компактная форма, строки сведений и раскрываемые
отпечатки используют существующие tokens/buttons.

Состояния: unavailable, idle, loading, canceled, error, read-only preview,
expired, applying, committed. Смена source/route/consent и unmount отменяют
own pending и отбрасывают поздний результат по generation/request ID.
Loading admission удерживается до возврата своего promise; новый запрос не
перекрывает старый, а поздний finally не завершает successor.
Для уже подтверждённого apply unmount не сообщает об откате и не снимает защиту.

API маппит строковые uint64 версии без numeric precision loss, обязательные
hash/boolean/time fields, request ID/route/release/package identities и deadline.
File cancel — null, не synthetic success. Optional methods разрешаются только
из generated namespace, без `window.go` bypass; старый EXE/preview показывает
unavailable. Generated files не редактировались, normal generation пока запрещена.

Explicit checkbox подтверждает показанный список и возможную остановку сессии.
Success означает только `committed` того же policy/capsule/version; готовность
VPN показывается отдельно. Error поглощает старое подтверждение и требует нового
превью; unsafe/старый/direct fallback не предлагается.

Форма имеет явные labels, текстовые errors после попытки, ARIA/focus, live status,
перенос длинных hashes и одноколоночную компоновку на узком окне. Из локального
modern-web guide `required-field-feedback` взят принцип своевременной текстовой
и ARIA-обратной связи; новые browser APIs/зависимости не добавлялись. Это source
решение, не визуальная/native/accessibility приёмка.

## Следующие обязательства

1. V2-141 [проверенные локальные сведения](TH-03-PROFILE-FACTS.md) добавлены
   в исходники вместо raw signed-only counts: read-only inspection, nullable
   effective eligibility/global bans/pending/unknown и обновление Settings;
   поведение unverified, health из eligibility не выводится.
2. V2-142 [common deadlines](TH-03-IMPORT-DEADLINES.md), private confirmation
   lifetime и общий UI expiry timer добавлены source-only. V2-143
   [future Next wakeup](TH-03-PIN-ACTIVATION.md) тоже source-only;
   strict incoming acceptance/restriction-only сохраняются.
3. Cold minimum-policy/custody/private history, SDK/codec/modules/old-client/
   resources/OS/crash/DR/native WebView2/bindings/rollout/release/comparative и
   TH-02/04…08. Изменённые исходники после V2-110R не имеют execution evidence.

Исходники: `windows/{onboarding_source,pin_policy_source,pin_policy_import,
onboarding_dialogs}.go`; frontend `api/{contract,backend}.ts`,
`components/{PinPolicySource,ProfileTransfer}.vue`.
