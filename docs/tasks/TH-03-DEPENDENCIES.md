# TH-03 — зависимости и принадлежность конфигурации

2026-10-21, V2-113: implementation, **unverified**. Это часть TH-03,
полная цель TH-00…08 сохраняется. Тестирование и сборки отложены владельцем.

Продолжение expiry/lifetime в исходниках — [V2-117](TH-03-EXPIRY.md), также
unverified. Ни этот graph, ни local clock не доказывают независимость или UTC.

## Контракт и исходники

- `metadata/dependencies.go`: optional signed `dependencies` содержит публичные
  псевдонимы publisher, provider:account и recovery_publisher. Это утверждения
  владельца, не проверка физической инфраструктуры/доступности/прав аккаунта.
  Strict bounded decode отклоняет неизвестные/дублирующиеся поля и null.
- В FD добавлены exit/exit_asn/resolver/provider:account. Пустое/unknown остаётся
  неизвестным. WS больше не создаёт carrier=cloudflare. Новые поля scalar;
  ограничения идентификаторов и максимум64 channels ограничивают объём работы,
  а не доказывают пригодность для такого числа живых каналов.
- `render/snapshot.go`: один owned snapshot каналов, process split и provenance.
  Источник — embedded configuration либо verified metadata signature; для
  последнего сохраняются version, canonical envelope hash, expiry и полный
  SHA256 реально проверенного Ed25519 public key. Подпись доказывает ключ,
  не независимость издателя. Реальные ключи владельца не читались.
- RenderFrom использует только явно переданные split/profile options. Поздний
  UI load не меняет его route rules. SplitDirect() остаётся синхронизированным
  legacy configured cache, не источником render и не подтверждением applied
  session. App возвращает applied split при работающем Manager, иначе configured.
- `ClientConfig.Clone` копирует raw, maps, lists и вложенные channel values.
  Protected probe и rollback получают отдельные снимки. Low-level RenderFrom
  callers должны передавать split context явно; старые tests пока не менялись.
- `render/dependency_graph.go`: origin/ASN/carrier/exit/ASN/resolver/accounts,
  publisher/signing key, actual configured access target, local profile artifact,
  measurement endpoints и unverified recovery alternative. Namespaces находят
  общие actors между разными ролями, включая origin одного и exit другого.
- RankHint — известные различия / полный вес релевантных ролей. Unknown
  приносит0, общие actors не получают бонус как «разные» в другой роли.
  Общий origin/access target ограничивает hint; declared weights — политика,
  не калиброванные вероятности. Recovery alternative не повышает hint.
- Failover использует provenance применённого cc, не очередного UI load.
  Graph влияет на порядок, не на eligible set или protected probe gate.
  UI показывает заполненность, общие/неизвестные роли и «есть различия»,
  не объявляет каналы независимыми/безопасными по одному числу.

## Совместимость и остаток

Новые optional поля размещены после старых; omitzero должен сохранять legacy
canonical omission. Это проектное намерение, сейчас не проверенный результат.
Строгие старые clients отвергнут новые поля; rollout требует согласованных версий.
Graph не переносит owner assertions в live observations и не считает число
доказанных независимых путей. У одного accepted profile общий signer/artifact
остаётся общим отказом доставки; offline recovery label не создаёт второй путь.

ExpiresAt пока provenance, не enforcement срока уже работающей session.
Clock rollback/expiry, trust roles/offline recovery и полная controller atomicity
остаются открыты. Далее приоритет P0 TH-01: изоляция noncooperative Start/Close;
таймаут вокруг embedded вызова не считается принудительным завершением.

Новые tests не добавлялись/менялись/запускались; build/vet/harness/experiments,
native/live/admin не выполнялись. Только исходники, чтение и gofmt.
