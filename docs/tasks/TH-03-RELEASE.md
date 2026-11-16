# TH-03 — согласованный публичный кандидат выпуска

2026-10-22, V2-125. **Исходники добавлены, не проверены.** Tests не создавались,
не менялись и не запускались; build/vet/harness checks/module/new CLI/native/live
не выполнялись. Реальные owner keys/certificates/vault/runtime config не читались.
Область этого чата — PC/Windows; Android code/plan/chat не менялись.

Предыдущий срез: [publisher authoring/custody](TH-03-PUBLISHER.md). Все9 этапов
и OS/module/rollout/release/comparative acceptance остаются открыты.

## Что добавлено

```text
metadatatool repository release public-release-plan.json APPROVED_ISSUER_ROOT_SHA new-candidate.json
updatetool repository release public-release-plan.json APPROVED_ISSUER_ROOT_SHA new-candidate.json
```

Описание будущих вызовов, команды агентом не выполнялись. Оба CLI используют
один `repository` package. Пример public плана — структура, не готовый выпуск:

```json
{
  "schema": 1,
  "packages": [
    { "kind": "profile", "plan": "plans/profile.json" },
    { "kind": "update_manifest", "plan": "plans/update.json" }
  ]
}
```

Ровно schema/packages; каждый entry — ровно kind/plan. Пути relative public JSON
и правила/границы прежнего issuer. Каждый nested plan задаёт свои explicit
artifact/authority/metadata paths по [issuer контракту](TH-03-ISSUER.md).

## Один набор исходных metadata

- Новый opaque `deliverypack.Publication` переиспользует прежние full SDK
  workflow + application signature/role gates. Getter возвращает owned copies;
  private proof не создаётся из JSON и не выдаётся как публичная authority.
- Все nested packages требуют одного approved issuer root и **byte-identical
  полного metadata map**, включая имена; одинаковой версии или signer недостаточно.
  Verified current root generation также должна совпадать.
- Каждый application target указан один раз. Один public epoch grant из profile
  package может разделяться несколькими target, только при точном совпадении bytes.
  Дубли application targets, разные grant/root/snapshot sets → отказ.
- Candidate не меняет клиентское доверие/профиль/политику и не пишет серверные
  каталоги. Его создание не означает, что клиент примет пакет или VPN заработает.

## Проверка всего inventory

Помимо full Updater проверки каждого выбранного package, builder проверяет
целый current graph: root→timestamp→snapshot→targets→profiles/updates.
Подписи проверяются штатным VerifyDelegate; каждый metadata link содержит
положительную version/length/hash и проверяет exact finalized bytes.
Timestamp указывает только snapshot; snapshot — ровно три target roles.

Каждый root self-threshold проверяется; предоставленные root versions образуют
непрерывную цепочку с old→new threshold. Latest совпадает с root SDK proof.
Конфликт bytes одной версии, имя не совпадает с signed version, устаревший
неиспользуемый nonroot metadata или чужой набор source records → отказ.
Истёкшие промежуточные roots допустимы; current roles должны быть fresh.

Все targets из current top-level/profiles/updates должны иметь artifact; extra
artifact тоже запрещён. Проверяются purpose/path/size и signed length/hashes.
Таким образом кандидат не молча публикует часть подписанного JSON inventory.
Это не проверка исторических client anchors каждого пользователя.

## Layout и manifest

| Derived path | Содержимое |
|---|---|
| `metadata/N.root.json` | Exact versioned roots |
| `metadata/root.json` | Latest root alias; hash отдельно от issuer bootstrap root |
| `metadata/N.snapshot.json`, `N.targets.json`, `N.profiles.json`, `N.updates.json` | Exact finalized current metadata |
| `metadata/timestamp.json` | Current timestamp |
| `targets/<namespace>/<hash>.<basename>.json` | Consistent-snapshot artifact aliases |
| `clients/<derived-sha256>.json` | Готовые public profile packages/update sidecars |

Pinned SDK `updater.go:232–249` выбирает первый доступный hash из map при remote
naming. Layout поэтому включает **все** объявленные поддержанные sha256/sha512
aliases, без предположений о порядке map. Hash algorithms/lengths сверяются;
case-aliased Windows paths и разные bytes на одном derived path отвергаются.
Отдельной своей проверки TUF threshold/hash algorithms вместо SDK нет.

Candidate JSON содержит schema1, release_id, manifest и files как base64 bytes.
Manifest хранит exact file list/path/length/SHA256, application target→client file,
source metadata name→derived path, issuer/published root hashes, root generation
и validity ceiling. Original bootstrap root source сохраняется через versioned
path, даже когда root.json alias уже указывает на более новый root.

Release ID — hash canonical JSON manifest, **не подпись/новая authority**.
Списки сортируются, timestamps manifest нормализуются в UTC; local observation
не входит в identity. Воспроизводимость ещё не измерялась. Candidate SHA в stdout
хеширует весь encoded output и отличается от release ID/ключевого fingerprint.

## Свежесть и ограничения ресурсов

После Sync/Close перед new-only link builder повторяет context/private proofs,
grant TTL и application profile validity. Delivery-pack теперь дополнительно
ограничивает expiry исходным approved bootstrap root: будущая первая установка
начинает проверку именно с него, даже если issuer Updater уже обновил current root.
Это ограничение выдаваемого пакета, не TTL уже принятой VPN-сессии.

Max32 packages, max256 derived files, суммарно files≤8MiB, encoded candidate≤12MiB;
cumulative metadata/artifact/client-package input budget16MiB. Per-purpose bounds
не сняты, public profile container≤256KiB. Release operation context60s, nested
SDK operation≤30s. Необязательные HTTP/remote fallback, EXE download или install
не добавлены. Это cooperative limits/расчёт buffers, не доказанный RSS/CPU/OS deadline.

**EXE payloads, secrets и server configs не входят в этот кандидат.** Signed
update manifest/sidecar не заменяет проверку/поставку бинарного файла (TH-08).
Report явно оставляет requires_promotion/client_acceptance=true,
includes_payload_executables=false. Ни настоящий выпуск, ни deployment не выполнены.

## Следующий срез

V2-126/127: [strict reread, retained-trust verifier и Windows publisher store](TH-03-PROMOTION.md)
добавлены в source, unverified. `promote`/`authorize-policy`/`recover` подключены
к существующим CLI в исходниках, **не запускались**. Используются independent
policy/root approvals, SDK/application AND gate, отдельный prefix commit,
four-target journal и after-staging source/time fence. Self-hash не authority.

Существующий filetxn допускает до8 fixed targets; подходит для фиксированного
publisher state/current pointer/root/timestamp journal, а не journal-chosen paths
для произвольного inventory. Immutable files должны быть готовы до timestamp
promotion. Windows byte-range writer lease реализована в source, native не
проверена. V2-128 добавляет [cooperating reader/status/repair/loopback source](TH-03-SERVING.md),
unverified. Client reselection/archive/root compatibility, crash/reboot/power-loss/
ACL/reparse/restore и реальные custody/server/EXE/UI rollout gates сохраняются.
