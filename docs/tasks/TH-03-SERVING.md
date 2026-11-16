# TH-03 — чтение, локальный статус и восстановление aliases

2026-10-23, V2-128. **Source-only, unverified.** Тесты не создавались/менялись/
запускались; compile/build/vet/harness/module/CLI/native/live не выполнялись.
HTTP listener даже на loopback не запускался. Owner keys/certs/vault/config не
читались. Только Windows source/docs; Android/harness source/state сохранены.
Предыдущая граница — [publisher store/promotion](TH-03-PROMOTION.md).

Продолжение V2-131: [bounded accepted archive/root index, durable serving floor
и storage admission](TH-03-RETENTION.md). Ниже сохранён контракт V2-128;
ограничения «нет архива/clock checkpoint» в текущих исходниках уточнены V2-131.
Это по-прежнему source-only/unverified; listener/CLI не запускались.

## Проверенный локальный head, а не доверие из public directory

`publisher_view.go` читает retained private publisher state под той же Windows
writer lease, что promote/recover. Reader не создаёт directories/lock file и не
выполняет recovery; missing store/lock — отказ. Общая capture сохраняет exact
bytes/absence четырёх control files, но **не принимает** aliases автоматически.
Обычный read отдельно требует согласованности aliases с retained head.

Candidate берётся только из derived `public/releases/<release-id>/candidates/
<retained-candidate-sha256>.json`; внешний запрос не выбирает этот путь. Known
head hash пришёл из локально принятой publication, не из self-hash найденного
файла. Strict bounded decoder проверяет manifest/inventory/source mapping,
release ID/root/timestamp/valid-until сверяются с head. Полный release view и
shared immutable version/hash paths перечитываются через existing staged gate.

Это read proof относительно защищённого checkpoint. Повторного enrollment или
нового SDK/application acceptance тут нет; оно было обязательным при promotion.
Клиент заново выполняет собственный SDK/application/pins/history/credential gate.
Private-directory tampering не решается self-hash: ACL/hostile path race gates
по-прежнему открыты. При prefix advance retained SDK root может быть новее
published root — reader сохраняет этот контракт и не подменяет старый head.

## Явный local status

Source contract существующих CLIs, **не выполненная команда**:

```text
repository status <publisher-directory>
```

JSON разделяет `state`, `local_integrity`, `can_serve_current`,
`client_acceptance=required`, `external_reachability=not_observed`.
Unavailable/inspection/recovery/aliases/source/inventory/clock/expiry не zero
health и не VPN FAIL. Timeout чтения — inspection_incomplete, не corruption.
Положительный local статус означает только согласованные checkpoint/inventory
и local time gate; exit0 сам по себе не проверка готовности сервера/VPN.

При валидном private state status может дать его exact SHA и public publication
head даже при несогласованных aliases. Key tables/cache/owner paths не выводятся.
Отдельный reader status действительно перечитывает disk inventory; cache HTTP
может ещё содержать аутентичные owned bytes уже удалённого immutable disk file.
Это разные наблюдения, не proof работы запущенного сервера или внешнего пути.

## Явное восстановление трёх aliases

```text
repository repair-aliases <approved-retained-state-sha256> <publisher-directory>
```

Команда требует valid existing private state/published head, exact independent
approval его bytes и intact previously accepted immutable release view. Новый
candidate/root/policy из аргумента не принимается. Missing/corrupt private state,
unfinished journal или отсутствующий immutable inventory не чинятся назначением
нового trust; broader restore требует отдельного owner repair/backup protocol.

Fixed four-target journal сохраняет state **Keep**, восстанавливая только
canonical current/root/timestamp aliases из retained head/view. Поэтому SDK
cache/root/clock floor, policy, prefix и schema/history bytes не сбрасываются.
BeforeCommit повторяет source bytes/absence и full immutable read после synced
backup staging. Staged read теперь повторяет gate также после последнего I/O:
отмена/expiry во время последнего файла не пропускается.

Истёкшую publication разрешено восстановить как историческое содержимое; она
останется неготовой к раздаче до fresh time-valid publication. Result явно
оставляет `trust_changed=false`, `requires_fresh_time_check=true`. Recovery
journal — отдельная команда `recover`; repair не запускает её автоматически.

## HTTP view одного выпуска

`NewPublisherHandler` не открывает сокет/файлы при создании. Handler принимает
только GET/HEAD по известным routes, без FileServer/listing/redirect/fallback:

```text
/current.json
/releases/<current-release-id>/metadata/<exact inventory file>.json
/releases/<current-release-id>/targets/<namespace>/<hash>.<name>.json
/releases/<current-release-id>/clients/<derived package hash>.json
```

`private`, journal, candidate frames, arbitrary paths, percent aliases, traversal,
query downloads и directory indexes не exposed. Metadata/targets routes также
могут иметь только точный путь из bounded accepted inventory. В ответах JSON,
nosniff/CSP/no-store, exact Content-Length/ETag/X-Snowden-Release; HEAD без body.
Внешнему caller возвращаются fixed codes, не OS errors/paths/keys.

Lease удерживается только на capture/owned copy/source fence, **не на HTTP write**.
Затем response использует одну owned immutable view. Cache mutex защищает только
pointer, не disk/HTTP I/O. Cache связывается с canonical current head bytes,
удерживает только decoded public file data≤8MiB, не outer frame≤12MiB. Journal/
control/time failure не получает fallback на старый cache. Cached bytes ранее
прошли полный disk inventory check; новый head требует нового complete read.

Две nonqueued admissions охватывают read и весь response lifetime; busy получает
503/Retry-After. Это bounds owned view admission, **не** hard RSS/GC/CPU/TCP count
или deadline синхронного filesystem I/O. Старые in-flight owned views могут
завершить ответ после concurrent promotion, сохраняя прежние exact bytes.
Повторный lookup нового запроса не подставляет новый выпуск под старый URL:
при смене release ID — 410 release_retired с current release header.

Клиент должен заново выбрать current и повторить свою целую SDK/artifact операцию,
сохраняя authenticated prefix. **Automatic downloader/reselection ещё не подключён.**
Single public profile/update transfer уже содержит свой полный metadata package;
SDK acceptance пользователя остаётся в исходном App import path.

Legacy unversioned `/metadata/*` и архив retired releases/старых root цепочек
handler пока не раздаёт. Поэтому это **не drop-in универсальный TUF HTTP server**
и не доказанная old-client compatibility. Durable root/archive retention,
in-flight retirement policy и bounded reconnect/reselection остаются явными gates.

## Opt-in loopback command и часы

```text
repository serve <publisher-directory> <literal-loopback-address:port>
```

Source dispatcher вызывает listener только по этой явной команде: literal
loopback, canonical port1..65535, без hostname/zone/public binding/port discovery.
До bind нужен accepted time-valid local view; startup не создаёт store. Исходники
задают header/read/write/idle timeouts и Ctrl+C→bounded Shutdown/Close. Native
shutdown/noncooperative I/O/TCP admission/cleanup не проверены, TLS/reverse proxy/
authentication/deploy решения не выбираются автоматически.

Handler и promotion повторно используют `metadata.ObservedClock`: wall/steady
continuity + retained floor; deadline повторяется перед headers/commit. Status
наблюдает continuity своей операции. Это local time evidence, не authenticated
UTC и не доказательство offline elapsed time после restart/reboot. Publisher
requests не обновляют retained floor автоматически; durable time checkpointing
и rollback-after-restart сохраняются отдельной открытой границей TH-03.

## Остаток полного плана

V2-129 добавляет [bounded transfer selection + common App preview/apply + UI source](TH-03-TRANSFER.md),
unverified. Normal Wails generation отложена, UI capability сейчас unavailable;
issuer-proof export, root/archive/client compatibility и publisher time/storage
lifecycle остаются. Generated bindings вручную не править.
Native ACL/reparse/hardlink/lease/journal/crash/power-loss, crypto/module/license/
interoperability/rollout/release и весь TH-00…08 acceptance остаются открытыми.
Ни local HTTP source, ни status/read/repair не подтверждают VPS/VPN/speed superiority.
