# ADR-002 — восстановление после publisher fast-forward

2026-10-22, V2-121…123. Контракт реализуется в [исходниках](../tasks/TH-03-EPOCHS.md),
**unverified, новый EXE не собран**. V2-123 добавляет explicit fresh epoch enrollment.
Установленный клиент и старые strict версии автоматически не получили эту функцию.
Нормативный статус/очередь — PLAN §5.3; этот ADR объясняет решение и границы.

## Дефект текущей модели

`MetadataVersion` — uint64. Профильный publisher способен подписать очень большой
допустимый counter; обычный recovery permit требует строго больший target.
При MaxUint64 законного следующего counter нет. Простая смена publisher/root не
исправляет этот application floor: TUF metadata counters имеют другую область.
Удалять checkpoint, очищать pin history или считать повреждённый файл нулём нельзя.

## Выбранный протокол

1. Сравнивать принятую profile identity как `(epoch, counter, content hash)`.
   Legacy history — epoch0. Counter строго растёт в одной epoch; exact replay
   сохраняет прежнюю identity, equivocation отвергается. Epoch растёт строго на1,
   никогда не берётся из одного входящего profile или local recovery signature.
2. Epoch change разрешает отдельный public **profile epoch grant**. Он должен
   быть точным target заранее enrolled TUF lineage и авторизоваться offline
   top-level targets threshold≥2 по различному public material. Online profiles,
   timestamp/snapshot и local single recovery key сами его не разрешают.
3. Предпочтённый reserved target — `trust/profile-epoch.json`, непосредственно
   под top-level targets. Обычные profiles/updates сохраняют свои delegations.
   V2-122 product policy разрешает только этот optional direct target; прежние
   strict клиенты его отвергнут. SDK workflow не заменять своей проверкой подписей.
4. Grant связывает schema/purpose/from_epoch/to_epoch, new profile public key,
   профильный namespace, protocol minimum, issued/expires и уникальную identity.
   Он не назначает update/recovery/root keys, не сбрасывает update floor и не
   разрешает произвольные targets. Scope только profile recovery.
5. Клиент требует оба private proof: offline epoch grant и точный target нового
   profile в разрешённом namespace. Затем — подпись новой profile authority,
   issuer/expiry/clock/credential/render gates и explicit epoch-recovery intent
   с подтверждением target SHA256. Документ grant сам по себе не artifact PASS.
6. Сохранять прежние time/issued floors, retired channel/publisher IDs и pin
   revocations. Старые IDs/сертификаты не воскресают в новой epoch. Counter может
   начаться с1 только после epoch grant; `(old epoch, MaxUint64)` остаётся частью
   локальной истории. Общая папка/транспорт/адрес не превращают канал в новый ID.
7. Новый accepted checkpoint сохраняет epoch/grant digest/lineage/source identity
   и прошлую границу. Public policy keys, exact envelope, checkpoint и vault
   меняются прежней four-target transaction после retirement. Source/vault/time
   fences и blocked-on-failure остаются; дополнительный необъявленный файл не нужен.
8. Verified TUF prefix сохраняется независимо от отказа application recovery;
   metadata freshness не становится TTL существующей VPN-сессии. Root rotation
   без специального grant не увеличивает application epoch автоматически.
9. Первая установка в текущую ненулевую epoch — отдельный `epoch_enroll` intent,
   независимо approved root + точный offline grant/profile + target SHA/confirm.
   Требуются отсутствующие version/envelope файлы и отсутствие delivery authority;
   decimal0/corruption/lost accepted history не clean source. Schema3 хранит
   immutable реальный origin (epoch/counter/content/grant/anchor/seed authority),
   без придуманных boundaries. Seed profile keys остаются retired, исключая
   только exact delivered signer; update/recovery/foreign roles не продвигаются.
10. Retention budget64 ограничивает число переходов устройства после origin,
    не глобальный epoch ID; старые schema2 histories начинаются с epoch0.
    Последующая смена origin/пропуск/переполнение uint64 или бюджета запрещены.

## Compatibility и stop conditions

- V2-122 source добавил envelope/checkpoint schema2, tuple consumers, proof kind/
  path, offline-role provenance, issuer packaging и explicit API. UI/native
  bindings/rollout и full publisher пока не завершены. V2-123 source добавляет
  schema3 origin/fresh enrollment и общую owned копию checkpoint, без проверки.
  Частичный переход, в котором один consumer ещё сравнивает только counter,
  запрещён к активации. Source inventory не заменяет compile/runtime acceptance.
- Вначале доставить поддерживающий клиент по действующему update trust path;
  только затем активировать grant/new schema. Старый strict delivery policy
  отвергнет reserved top-level target, старый profile decoder — новую schema.
  Это ожидаемый отказ, не повод для direct/legacy fallback.
- Unenrolled legacy client не получает защиту TUF/epoch только из нового файла.
  Нужен явный bootstrap из независимого trusted root, сохранённая source history
  и отдельно предусмотренная миграция. Удаление повреждённого checkpoint не миграция.
- Отсутствующий/повреждённый source, чужая lineage, rollback/skip epoch, истёкший
  grant, смешанные target bytes, изменённая authority/vault или исчерпанный storage
  budget → отказ с сохранением protection/history. Unknown/corrupt epoch ≠ epoch0.
- Компрометация offline threshold, потеря всех root custodians, forged authenticated
  time и локальный admin — другие угрозы. Один такой grant их не устраняет.
- Update epoch/floor и полноценный installer recovery проектируются отдельно;
  profile grant не разрешает снижение версии EXE. Реальная независимость людей/
  ключевых хранилищ не доказывается числом разных public keys в metadata.

## Приёмка, когда владелец разрешит тестирование

Counter MaxUint64→authorised next epoch1; отказ без offline proof и при single
profile/recovery key; exact replay/equivocation/old epoch/skip/expired grant;
сохранение revoked/retired/time history; source/prefix/crash/four-target rollback;
полный inventory consumers/old-client upgrade/native blocked path. Команды не
выполнялись, fixtures не добавлялись. Пока эти gates не пройдены, production
recovery и весь TH-03 остаются непроверенными.

Fresh epoch enrollment дополнительно требует проверки действительно отсутствующих
files vs decimal0/corruption, current epoch>64, source replacement после retirement,
foreign/retired roles, origin immutability/deep-copy и реальной истории следующих
переходов. Максимальный номер epoch не обещает неограниченную retention.
