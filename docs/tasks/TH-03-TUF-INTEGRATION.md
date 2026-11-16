# TH-03 — включение проверки доставки в App

2026-10-21, V2-120. **Изменены исходники; компиляция и поведение не проверены.**
По запрету владельца тесты не добавлялись/менялись/запускались; build/vet/harness,
module resolution, native/worker/live и сборка EXE не выполнялись. Выполнены
редактирование, чтение исходников и gofmt. Все TH-00…08 остаются в объёме.

Основа: [адаптер V2-119](TH-03-TUF.md), [ADR-001](../decisions/001-tuf-delivery.md).

## Состояние и полномочия

- `TrustPolicy.delivery` содержит owned Root/Cache/ClockFloor из адаптера.
  Публичный файл ограничен 6MiB, отдельное delivery state — 5MiB; local keys≤256.
  Schema1 не получает delivery authority. Null/aliased/unknown fields отвергаются.
- `AcceptedProfileState.delivery_anchor_sha256` сохраняет initial anchor.
  Несовпадение, отсутствие root state или accepted checkpoint блокирует загрузку
  enrolled policy через общий Store. Runtime checkpoint writer не меняет marker.
  Это обнаружение рассогласования файлов, не защита от локального владельца,
  способного одновременно переписать оба файла. Потеря отдельных cache floors,
  ACL/reparse/interprocess/power-loss остаются отдельными native gates.
- `TrustPolicy.Scoped` копирует delivery buffers. Authority digest содержит local
  roles/retirement, initial anchor и SHA256 текущего root; comments/cache/time в
  него не входят. Full encoded source fence включает cache/time/comments/schema.
  Обычное metadata refresh не инвалидирует recovery permit, смена root — да:
  после такой смены issuer должен получить новый public recovery context/permit.
- TrustClockFloor учитывает последнее локальное delivery observation при новом
  импорте, update и построении profile snapshot. Repository expiry не становится
  session TTL: уже принятый профиль запускается по exact accepted identity и
  собственным profile/pin/certificate deadlines, без network refresh на Start.
  Local time по-прежнему не authenticated UTC.

## Импорт и enrollment

Отдельные App API `PreviewDeliveryEnrollment` / `ApplyDeliveryEnrollment`
принимают out-of-band root SHA256; Apply дополнительно требует confirm.
Root берётся из `package.metadata["root.json"]`, exact hash сравнивает адаптер.
Входящий пакет не может сам одобрить свой root или заменить уже enrolled anchor.
Для enrolled установки rotation проходит только через upstream root chain.

Optional `BundlePayload.delivery` содержит `target_path`, `profile_bytes` и
`package`. `profile_bytes` и metadata values — base64 byte arrays. Старое поле
`envelope` сохраняется; сравнение допускает только whitespace/HTML escaping
стандартного outer JSON encoder; нормализованные bytes не передаются verifier.
Import декодирует, проверяет и записывает именно исходные profile bytes.
Общий container/KDF bound 256KiB не увеличен; большие пакеты пока не переносятся
этим API. Старые strict clients отвергают новый optional field.

Для enrolled import/recovery обязательны одновременно private TUF proof точных
bytes/purpose/path и прежние local profile/recovery signatures, versions,
retained pin/channel/revocation/time history, credential/render gates. Recovery
не получает root/update grants, не сбрасывает floors. Обычный Store acceptance
без proof разрешает только уже принятую exact profile identity; замена профиля
на диске не становится новой доставкой только из-за валидной local подписи.

Initial enrollment записывается только после полного target/application gate,
в прежнем prepare→retire→source fence→four-target commit. Поздний отказ оставляет
profile transition blocked как раньше. Preview не сохраняет trust/files/vault.
Действующий root, certificates или owner keys агентом не назначались.

## Проверенный prefix при отказе

V2-191 [W2 role progress](TH-03-ROLE-PROGRESS.md): возвращаемый prefix строится
из успешных SDK per-role returns с exact-byte/continuity/effective-time/link
eligibility. Intermediate pointer после rejected expiry/version не означает
новое принятие. Прежняя accepted expired history сохраняется отдельно; полный
SDK/snapshot/target proof всё ещё обязателен. Источник сохранения и actual durable
outcome ниже не заменяются этим source-only изменением.

Commit-запрос enrolled установки сохраняет возвращённый адаптером проверенный
prefix **до** более поздних artifact/application gates. Отсутствие target proof
остаётся отказом импорта/update. При ошибке persistence текущая операция не
принимает artifact. Initial failed enrollment не создаёт новую authority.

`retainDeliveryProgress` сверяет source digest, exact public policy/checkpoint
и encrypted vault snapshot; не вызывает lifecycle/App под journal/vault lock.
`filetxn.File.Keep` оставляет остальные три target как есть, включая отсутствие
старого envelope при recovery. Все четыре backup/recovery target и старый
manifest format сохраняются; startup rollback восстанавливает полный прежний
набор. Не появляется необъявленный пятый файл или второй import journal.
Это source design; crash/interprocess/durability поведение не подтверждено.

## Обновление EXE

При enrolled authority рядом с `update.json` обязателен `delivery.json`:
объект с `target_path` внутри `updates/` и `package` с metadata map. Sidecar не
может enroll root. Его bounded strict decoder, SDK verifier и local update role
используются общим App path; Check не пишет state, Apply сохраняет prefix.
Повторный exact policy/checkpoint/expiry/version/signature gate удерживает
operation/journal serialization на время filesystem swap; callback не вызывает
App/lifecycle. Изменившаяся authority требует новой подготовки.

Manifest≤64KiB: duplicate/alias/null/unknown/trailing JSON отвергается. Payload —
regular file, portable basename, positive declared size≤1GiB, bounded hashing.
Добавлены per-App installer mutex, exclusive temporary staging, staged hash
до stop и непосредственно перед rename. Restart использует существующие
`modeSwitchArgs` / bounded `--await-previous`, а не PowerShell/PID polling.
Успешный Start процесса не объявляется readiness новой версии.

## Остаток и следующий срез

- Issuer package builder/plan, root epoch/application counter migration и recovery
  product UI/нативная генерация Wails bindings. Новые API ещё не выведены в UI.
- Enrolled ExportOnboarding сейчас явно отказывает: старый export потерял бы
  proof. Нужен свежий issuer package; молчаливой legacy downgrade нет.
- Module graph/SumDB/SBOM/conformance, compile/tagged/native acceptance и rollout.
- Полный installer lifecycle: concurrent Connect/Disconnect/quit, cross-process
  staging/ACL/reparse, locked EXE, atomic restore/startup health/floor/canary,
  crash/power-loss и воспроизводимый EXE/NSIS. Новый AND-gate их не закрывает.
- Context/resource bounds не дают hard deadline для OS I/O, CPU/RSS или suspend.
  Потеря durable state и publisher fast-forward не лечатся сбросом старой истории.

V2-121 добавил [public package assembler/API](TH-03-ISSUER.md) в исходники,
без full TUF metadata authoring/signing. Epoch recovery — ADR-002, ещё не код.
Следующий срез: реализовать epoch contract, затем full publisher/UI/rollout.
Тестирование/сборки и OS/live gates остаются отложены по указанию владельца.
