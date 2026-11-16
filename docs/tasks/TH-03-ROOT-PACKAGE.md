# TH-03 — цепочка прежних корней внутри нового whole package

Продолжение V2-133: [approved maintenance + stream backup export](TH-03-MAINTENANCE.md),
также source-only; restore и native/DR acceptance ещё открыты.

2026-10-23, V2-132. **Source-only/unverified.** Тесты не создавались/менялись/
запускались; compile/build/vet/check/module/gen/CLI/HTTP/native/live не выполнялись.
Реальный publisher store/owner keys/cert/vault/config не читались. Только Windows
source/docs; Android/harness сохранены. Основа — [retention](TH-03-RETENTION.md).

## Проблема и существующие границы

PC downloader скачивает весь public profile JSON, а не отдельные TUF roots по
HTTP. Поэтому root route V2-131 сам по себе не устраняет разрыв цепочки в package.
Добавление roots меняет client JSON и manifest inventory; менять bytes уже
принятого выпуска под его прежним URL нельзя.

Новая explicit issuer операция готовит **отдельный candidate** вне publisher store.
Публикация остаётся за обычным `promote`, клиент — за общим import/SDK/application/
epoch/pin/history/credential/lifecycle gate. Новая операция не назначает роли,
не сбрасывает установленные корни и не запускает server/VPN.

## Source contract существующего repository CLI

Это описание реализованного dispatcher, **команда не запускалась**:

```text
repository supplement-roots <candidate.json> <approved-candidate-sha256> <application-policy.json> <approved-policy-sha256> <approved-retained-state-sha256> <first-client-root-version> <publisher-directory> <new-candidate.json>
```

Нужны exact independent approvals input candidate, scoped application policy и
исходного private publisher checkpoint. Store должен существовать, journal/aliases
должны быть согласованы. Входы/output находятся вне store; output обязан
отсутствовать, запись — existing new-only PublishNewChecked. Stable writer lease
удерживается до завершения подготовки. Это может временно давать HTTP busy/503
другому cooperating process; никакого availability/SLO обещания из исходников.

## Общий retained verifier вместо второго механизма доверия

`publisher_prepare.go` выделяет прежний promote pipeline: independent role policy,
existing retained SDK origin либо approved first enrollment, bounded full release
verification, forward root staging и отдельный verified-prefix commit. Promote и
supplement вызывают один и тот же helper. Supplement требует existing state и
`retained`; initial approval клиента этим не заменяется.

Approved old state сравнивается **до** собственного prefix advance. Затем финальные
gates привязаны к exact перечитанному state после этой собственной транзакции.
Сохранение SDK prefix отдельно означает, что поздний отказ подготовки не откатывает
уже аутентифицированные root/cache/time данные. Incomplete first enrollment
по-прежнему не сохраняется. Ошибка после prefix retention сообщает об этом явно.

## Откуда берётся цепочка

Версия первого клиентского корня — canonical positive int64. После проверки
кандидата выбирается диапазон до текущего private SDK root включительно. Разница
должна быть **строго меньше MaxRootRotations16**, как требует текущий SDK adapter.
Большая дистанция отказывается с необходимостью отдельного staged recovery,
автоматического reenroll нет.

Каждый root читается только через private retained version/SHA index V2-131:
bounded owned file, exact digest и version. Неизвестный/утраченный root не
назначается из public directory, input candidate или unsigned current pointer.
Отсутствующая история до начала retained index не выдумывается. Конфликт имеющегося
candidate numeric root с принятым root той же версии — отказ.

## Пересборка whole release

`release_supplement_layout.go` добавляет numeric roots в common metadata set через
existing bounded NewPackage. Пересобирает public onboarding transport и update
sidecar через existing codecs, затем common layout/links/inventory/manifest.

Exact signed `profile_bytes`, update target bytes, epoch grant, target paths,
application signer hints и profile identity сохраняются. Секреты не участвуют.
Client name по kind/target остаётся прежним **внутри другого release namespace**;
manifest/source metadata/file hashes и release ID пересчитываются. Если фактические
bytes не изменились, эта операция сама по себе не обещает другой ID.
Старые immutable objects/current aliases не переписываются.

Исходный candidate и дополненный candidate проходят полный SDK/local-role gate
от retained current state. Дополненный candidate также проходит тот же production
gate от временного baseline с выбранным **известным** прежним root. Такой baseline
не имеет cache и существует только внутри подготовки issuer candidate; он не
сохраняется, не передаётся клиенту и не становится publisher authority. Expired
исторический root допустим только если SDK принимает переход к свежему final root;
expiry текущего root/profile/grant/metadata не ослабляется.

Это root-path precondition, **не** доказательство совместимости с реальными
старым EXE, cached timestamp/snapshot floors, role/pin/revocation history и
credential state. Их клиентские проверки сохраняются и могут отказать.

## Финальная граница

Перед output и ещё раз после synced staging перед new-only link повторяются:
exact source controls, external candidate/policy bytes/approval, indexed root
bytes, output path guard, ObservedClock и все private artifact proofs/context.
Writer lease снимается после операции. Report разделяет candidate hash/release ID,
root range, prefix retention, requires promotion/client acceptance и отсутствие
server activation. Ошибка снятия lease после записи сообщает, что output уже есть.

Все прежние budgets package/metadata/client JSON/candidate/file count остаются.
Переполнение от добавленных roots — отказ без обхода лимитов. Context90s кооперативен;
несколько полных verifier passes увеличивают offline authoring work, hard CPU/RSS/
GC/FS interruption/Windows ACL/reparse/link/durability не доказаны.

## Остаток всего плана

Issuer source теперь может подготовить metadata-complete пакет для известного
прежнего корня в пределах SDK bound. Реальные CLI/crypto/SDK/old-client import,
client/update matrix, native/OS/bindings/module/rollout/release ещё не проверены.
Никакой тест-код не добавлен и никакие проверки не запущены.

Следом — explicit source-approved maintenance/backup, сохраняющий current,
retained archives/roots/shared objects и in-flight reader ownership. Global signed
revocation/private pin-history portability и все TH-00…08 обязательства сохраняются.
