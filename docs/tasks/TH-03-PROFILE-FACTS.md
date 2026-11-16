# TH-03 — проверенные локальные сведения о профиле ПК

2026-10-24, V2-141. **Реализовано в исходниках, не проверено исполнением.**
Тесты не добавлялись/не менялись/не выполнялись; сборка, typecheck, bindings
generation, SDK/crypto, CLI, браузер, native и live не запускались.
Полная цель TH-00…08 и P0/OS/DR/release/comparative gates остаются открытыми.

## Исправленный дефект

Прежний `GetDeliveryProfile` загружал envelope, но не проверял его подпись,
активную роль signer, version/expiry/checkpoint/global policy. Он выставлял
`present=true` по наличию файла и считал все channels кроме signed revocations.
Settings делал из этого зелёное «Установлено», а mapper заменял отсутствующие
значения нулями. Это не подтверждало приёмку или effective eligibility.

## Общий gate, отдельный режим чтения

`Store.InspectRuntimeProfile` переиспользует runtime authentication/projection
через один private loader. Обычный `LoadRuntimeProfile` сохраняет прежний
разрешённый legacy bootstrap/checkpoint writer. Inspect не вызывает Save,
не принимает новый профиль и не записывает version/history/time floor.
Legacy signature-only view помечается `Checkpointed=false`; она не выдаётся
за принятый профиль с известным разрешённым составом.

Гейты сохраняют active signer/signature, profile version+expiry/local clock floor,
pending target/applied policy marker и exact installed content identity для
глобальной policy. Incoming import остаётся строгим, signed bytes не меняются.
ObservedClock используется как прежнее process-local наблюдение времени;
оно не записывает accepted checkpoint и не является authenticated UTC.

Runtime projection и retained counters теперь используют общие unique unions
локальных/global certificate и channel bans. Проверка retired ID не зависит
только от предварительного flatten в state.Retired: global retired IDs тоже
непосредственно ограничивают projection. Это только denial, без нового enable,
certificate/endpoint, live evidence или ослабления импорта.

`DescriptorsFromRuntimeProfile` — общий pure structural gate для render snapshot
и информационного чтения. Он отдаёт owned copies, не читает vault/PEM, не
запускает engine и не меняет configured split. `render.Selectable` считает
metadata-разрешение войти в selector; credentials, сборочные/runtime capability,
protected probe и работоспособность сети в эту цифру не входят.

## Снимок App

`GetDeliveryProfile`/DTO перенесены из общего infobindings в существующий Go main
package, файл `delivery_profile.go`. Внешнее имя метода и legacy поля сохранены.
Clock handle берётся до import-journal lock; информационное чтение не берёт
operation/session/engine ownership и не делает HTTP или попытку подключения.

Прежний journal сериализует cooperating writers в одном процессе. Whole encoded trust policy и
checkpoint representation повторно сверяются перед возвратом; signature identity
и checkpoint tuple/hash view сравниваются с исходной authority. При source change
весь результат заменяется unavailable, а не смешиваются старые/new counters.
Это не доказательство защиты от hostile writer/reparse/hardlink/power-loss или
атомарности ответа относительно всех OS процессов — native gates сохраняются.

`factsSchema=1` различает:

- `accepted`: текущий принятый профиль прошёл локальные проверки, eligible >0;
- `restricted`: профиль принят, разрешённых для выбора каналов сейчас0;
- `legacy_unanchored`: signature/version действительны, приёмка не зафиксирована;
- `policy_pending`: retained top-target и applied marker не совпадают;
- `embedded`: внешнего профиля нет, это не proof работающего встроенного пути;
- `unavailable`: чтение/подпись/версия/expiry/source/clock не подтверждены.

Nullable counters показывают active profile keys, declared/eligible/restricted
channels и unique retained certificate/channel bans. Restricted включает и
removed signed revocations, и disabled projection без повторного учёта. Последние
retained counts — сведения о
локально сохранённых запретах, не proof отсутствия unseen private history.
Pending/rejected/legacy состояния не получают known eligibility или synthetic0.
Current identity выдаётся только после authentication; retained identity отдельно
называет последнюю checkpointed версию. Строковая `epoch:counter` и строковая
policy version сохраняют uint64 precision; старые numeric поля не authority.

`PinPolicyState` различает applied, pending, not_observed, unknown. Сохранённый
список продолжает ограничивать после expiry; «не получен» не означает отсутствия
глобальных запретов у издателя. Repository freshness не продлевается этим чтением.

## UI и обновление

Mapper проверяет schema/status/nullable counters/tuple/hash/time/boolean и их
согласованность. Старый binding без factsSchema даёт unknown и пустые факты,
без переноса прежних raw counts/«подпись проверена» claims. Браузерный preview
не маскируется production facts; dev mock и generated файлы не менялись.

Settings использует прежний контейнер/строки. Показывает статус, версии,
разрешение выбора, сохранённые ограничения, applied/pending policy и local
checked time; зелёный профиль не называется работающим VPN. Loading скрывает
старый snapshot, errors/unknown не превращаются во встроенный fallback.
Ручной refresh, generation/unmount guards и `changed` от policy/profile apply
и export обновляют справку в том числе после частично сохранённого prefix/error.
Поздний ответ не перезаписывает более свежий запрос.

Длинные значения и header controls переносятся в существующей компоновке;
status/labels остаются текстовыми. Browser/native/DPI/keyboard/refresh race
и доступность пока не проверялись, визуальное улучшение не заявляется.
Локальный guide css-layout использован для обычного flex-wrap и min-width:0;
ограниченно поддерживаемые новые варианты wrap и зависимости не добавлялись.

## Следующий срез

V2-142 [общие сроки](TH-03-IMPORT-DEADLINES.md) и private intent lifetime добавлены
в исходники, unverified. V2-143 [future Next wakeup](TH-03-PIN-ACTIVATION.md)
добавлен через owned timer/generation/clock и fresh Start/probe, также unverified.
Cold minimum-policy/custody/private history и остальные TH/P0/native/SDK/DR/
resources/old-client/bindings/rollout/release/comparative остаются открытыми.

Исходники: metadata/runtime_profile, render/snapshot, delivery_profile,
infobindings; frontend api/contract/backend, SettingsView и change signals
ProfileTransfer/PublicProfileSource/PinPolicySource. Форматирование Go —
не compile, crypto, runtime, GUI или VPN evidence.
