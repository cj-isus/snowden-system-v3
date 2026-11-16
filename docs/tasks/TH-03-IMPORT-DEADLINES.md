# TH-03 — общий срок превью и подтверждения импорта

Статус: V2-142, 2026-10-24, **реализовано в исходниках, unverified**.
Windows/PC only. Тесты не добавлялись/не менялись/не запускались; build,
typecheck, bindings generation, SDK/crypto/CLI/browser/native/live не выполнялись.
Форматирование девяти Go-файлов завершилось с exit0; это не проверка поведения.

## Общий контракт

`bundleDelivery.validUntil` повторяет существующие проверки полного private
batch и возвращает минимальный срок profile proof, epoch proof/body и pin-policy
proof/body. Только для первого enrollment добавляется срок bootstrap root;
сохранённый старый root не запрещает обычную проверяемую forward rotation.
`profileImportDeadline` добавляет подписанный профиль и recovery permit.
Отсутствующий необязательный proof не создаёт фиктивный срок или authority.

`OnboardingPreview.valid_until` передаёт этот минимум во всех режимах импорта.
Старое `delivery_valid_until` сохранено; для delivery оно получает тот же минимум.
Export учитывает исходный и portable proof, сохраняя исходные signed bytes,
role/epoch/history проверки. Существующие final proof gates уже отвергали
просроченные grant/policy: изменение исправляет неполный срок превью и lifetime
подтверждения, а не заменяет эти gates новым JSON-полем.

## Подтверждение и отмена

Source preview передаёт собственный child context в общий SDK-путь. Backend
one-use token получает min(две минуты, полный valid_until); отсутствующий или
неверный срок отклоняется. Apply переносит срок из private App intent, не из
публичного JSON. Проверка повторяется после ожидания operation mutex, подготовки
SDK/retained prefix и в общем source fence после retirement и synced staging.
Export повторяет private token deadline перед credentials и в final fence после
KDF перед выдачей переноса. Поздняя ошибка не откатывает уже отдельно сохранённый
аутентифицированный prefix: pending obligation остаётся blocked.

Прямые legacy APIs не получают выдуманную server-stored preview nonce. Они
повторяют все обычные signed/source/time проверки; UI ограничивает собственное
превью, а backend source-token TTL применяется только к существующему private
подтверждению. Сохранение уже зашифрованного переноса не продлевает его подписи;
получатель повторяет проверку.

## Интерфейс

Общий `usePreviewExpiry` используется public profile, pin policy и encrypted
import. Он принадлежит одной generation, очищается при unmount, не запускается
после disposal и отключает подтверждение после срока. Новый запрос сбрасывает
старый timer до await; устаревший callback не отменяет successor. Recovery approval
и export password очищаются при истечении. Невалидный backend срок вызывает
понятную ошибку. Таймер — UI feedback, не доказательство clock/suspend safety;
решение о commit принимает backend с повторной проверкой.

## Незакрытая приёмка и следующий срез

Нужны разрешённые владельцем compile/SDK/crypto/clock/queued-wait/staged-disk/
cancellation/race/UI/unmount/old-binding проверки. Ни один критерий не объявлен
PASS. Текущий execution ban сохраняется.

V2-143 [future HY2 Next activation](TH-03-PIN-ACTIVATION.md) добавлен в исходники
через owned timer/generation/clock и fresh обычный Start/probe, unverified.
Cold minimum-policy/custody/private history, P0 и все TH-00…08, module/bindings/
native/OS/crash/DR/resources/old-client/rollout/release/comparative остаются открытыми.

Исходники: `windows/onboarding_{deadline,delivery,authority,import,public,source,
export_proof,export}.go`, `onboarding.go`; frontend `composables/previewExpiry.ts`,
`api/{contract,backend}.ts`, `components/{PublicProfileSource,PinPolicySource,
ProfileTransfer}.vue`.
