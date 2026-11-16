# TH-03 — доверенный архив, корни, часы и лимит записи

Whole-package delivery продолжена в [V2-132](TH-03-ROOT-PACKAGE.md): explicit
issuer supplementation готовит новый candidate; runtime compatibility не заявлена.
Ниже сохранён срез V2-131, его «ещё нет supplementation» уточнён продолжением.

2026-10-23, V2-131. **Source-only/unverified.** Тесты не создавались, не менялись
и не запускались. Build/vet/check runner/module/generator/CLI/HTTP/native/live
не выполнялись. Реальный publisher store, owner keys/cert/vault/config не читались.
Только Windows source/docs; Android и обвязка сохранены. Основа:
[promotion](TH-03-PROMOTION.md), [serving](TH-03-SERVING.md), [export](TH-03-EXPORT.md).

## Два bounded индекса в существующем private checkpoint

`publisherState` schema1 принимает optional `history`, `roots`, `serving_floor`.
Strict parser ограничивает размеры/формы, запрещает null и неизвестные поля;
старый checkpoint без этих полей читается без назначения истории из public disk.
Общий предел state — delivery5MiB +128KiB. Старый publisher decoder отвергнет
новые поля: обратная совместимость publisher EXE/backup требует отдельной приёмки.
Формат public current, client package и подписи не изменён.

- `history`: текущий head и до16 предыдущих **принятых** публикаций. Promotion
  переносит прежний private head в начало bounded списка и исключает дубликат
  нового release ID; policy/prefix/clock операции список не изменяют.
- `roots`: до128 version/SHA256 записей, строго последовательных. Последняя
  совпадает с exact retained SDK root. Никакого eviction корневой истории:
  исчерпание ёмкости требует явной миграции и блокирует новое продвижение.
- `serving_floor`: сохранённое наблюдение локальных часов. Эффективный порог —
  max этого значения и SDK `Delivery.ClockFloor`; переход не может его снизить.

Корневой индекс начинается с root, который эта установка действительно знает.
Отсутствующую историю до него восстановлением файлов из public folder не назначаем.
Чистая установка с approved root v10 не доказывает происхождение v1…v9.

## Forward root continuity и отдельное сохранение SDK prefix

Verifier держит private exact root до SDK и owned candidate records. После SDK
`trustedRootFiles` идёт **вперёд** от этого корня: consecutive version, подпись
предыдущим threshold и новым self threshold; в конце exact byte equality с
SDK `NextState.Root`. Результат не выводится из self-signature найденного файла
или из обратного обхода от incoming root. Цепочка ограничена `MaxRootRotations`.

Новые root files пишутся new-only в `public/metadata/<N>.root.json`; конфликт
одной версии с другими bytes — отказ. Root index сохраняется вместе с verified
SDK prefix отдельной транзакцией до принятия release. При позднем artifact/role
отказе существующая установка сохраняет этот prefix; incomplete first enrollment
не сохраняется. Immutable leftovers без принятого индекса не дают HTTP authority.

Общий commit gate запрещает сокращать/заменять прежний root index, снижать
эффективный порог времени и менять archive без promotion. Существующий журнал
по-прежнему имеет ровно state/current/root/timestamp; новые authority files и
второй commit protocol не создаются. Policy/repair/recovery сохраняют эти данные.

## HTTP: прежние поколения и отдельный root lookup

`/releases/<id>/...` выбирает current либо exact private archived head. Для архива
перечитывается полный bounded accepted candidate/inventory, затем source fence.
Чужой discovered frame не принимается по self-hash. Archive view принадлежит
запросу; cache остаётся один, только current. Две admissions по-прежнему охватывают
read и response. Writer lease освобождается до отправки ответа.

Valid архив отдаёт прежние bytes без подмены current. Не retained либо expired
release возвращает410; клиент может целиком перевыбрать current. Архив не
продлевает profile/metadata/grant deadline и не отменяет клиентские version,
revocation, pins, credentials и epoch gates. Удаление head из списка после16
поколений закрывает route; **дисковые файлы не удаляются**.

Новый узкий `/metadata/<N>.root.json` разрешает только canonical положительный
int64 и root, чей digest находится в private index. Нет listing/unversioned alias/
произвольного metadata lookup. Missing next root —404, повреждение/неопределённость —
503 без fallback. Перед ответом root bytes/version/SHA сверяются с индексом.
Исторический root можно получить после его expiry: собственный SDK клиента
обязан проверить последовательные thresholds и freshness конечной цепочки.
Просроченный profile или target этим исключением обслуживаться не начинает.

## Durable local serving clock

Handler использует существующий `ObservedClock`. Первый допустимый read и затем
наблюдение не чаще раза в `ClockCheckpointInterval` (1min) сохраняют floor через
тот же four-target journal со всеми aliases Keep. До commit и после последнего I/O
повторяются source/context/time gates. Ошибка записи/журнала/release lease означает
отказ ответа, не stale cache. После собственной записи источник перечитывается.

Status остаётся read-only. `serve` preflight теперь может сохранить clock floor
до открытия listener. Ошибка последующего HTTP write не откатывает наблюдение и
не принимает другую публикацию. Наблюдение может сохраниться даже при последующем
404 выбранного файла. В status показаны retained counts, floor и admission policy;
эти counts не доказательство целостности всех архивов/корней.

Это **локальное время**, не authenticated UTC, secure monotonic hardware или
доказательство elapsed offline time. Сохраняются допуск MaxClockSkew5min и
checkpoint interval; откат всего защищённого backup/hostile state writer этим не
устранён. Никакого обещания точной real-time revocation после offline периода.

## Admission перед append-only записью

`publisher_storage.go` ограничивает public logical data256MiB,8192 файлов,
2048 каталогов, depth32 и сохраняет16MiB/4-file staging reserve. Обход делает
`ReadDir(128)`, проверяет context между объектами, regular paths и прежние
Windows reparse gates. Считаются также abandoned stages/releases и неизвестные
объекты: они не исчезают из бюджета из-за отсутствия в accepted index.

До root staging и до release staging учитываются все новые unique paths/bytes/
каталоги; byte-identical replay не считается новой копией. Existing immutable
conflict затем отдельно отклоняет `writeImmutable`. Cooperating writer lease
сериализует операции. Отказ admission не разрешает удалить активный выпуск или
обойти сохранение доверия; требуется явное обслуживание/восстановление места.

Это не physical disk quota/reservation: private journal/state backups, allocation
units, внешние процессы, hardlinks, hostile rename/reparse races, blocking FS I/O,
RSS/GC/CPU и crash/power-loss остаются OS gates. GC/удаление не реализованы и не
выполнялись; bounded approved maintenance нужен следующим срезом.

## Что остаётся в полном плане

Root HTTP route и архив **не доказывают old-client compatibility**. PC downloader
сейчас берёт whole public package; он не подгружает корни через новый route.
Issuer/candidate пакету ещё нужна согласованная bounded supplementation из
retained chain без изменения immutable принятого package под прежним именем.
SDK per-refresh rotation ceiling16 остаётся, даже при общем index128.

Дальше: bounded client root-chain delivery/старые package версии и явное
source-approved maintenance/backup, не затрагивающее current/archive/root/shared
объекты и in-flight readers. Global signed revocation/private pin-history
portability, bindings/module/crypto/SDK/native/OS/rollout/release/comparative gates,
TH-01/02/04…08 остаются открытыми. Приёмку сформулируем по этим инвариантам после
разрешения владельца; тест-код и запуск сейчас запрещены. Готовность VPN/VPS и
превосходство скорости не заявляются.
