# R05-010 — TUF progress и recovery при откате доверенной истории

Срез исходников и спецификаций: **27 октября 2026**. Авторский статус —
**READY_FOR_REVIEW**, не ACCEPTED. Описания «current» ниже относятся к этому
историческому срезу, а не к последующим версиям продукта.

## 1. Область, метод и новая дельта

Проверены только спецификации, pinned upstream source и адресные публичные
исходники проекта. SDK, криптография, реальные checkpoints/профили/vault,
запись доверенного состояния и native-действия **не исполнялись/не читались**.
Примеры ниже символические, не fixtures и не воспроизведённые ошибки продукта.

Baseline: R05-001-A/B, R05-002, R05-005/006/007/008. Их READY-статус не даёт
права наследовать утверждения как аксиомы. Новая дельта:

1. Матрица **persistable predecessor** по §§5.3–5.6 TUF и проверка текущего
   wrapper, отличающегося от старого описания «только workspace cache».
2. Два разных ограничения знания: **потеря ранее наблюдённой истории** при
   полном rollback и **неизвестность новых внешних событий** даже без rollback.
3. Bounded recovery contract: exact TUF API, независимые основания истории и
   времени; серверный anchor — один вариант, не единственно допустимый.

### Доказательная база

- TUF **1.0.36**, Last modified **5 August 2026**, §§2.1.1, 5.1–5.6, 6.1;
  соседняя **1.0.37**, Last modified **8 October 2026**, critical paragraphs
  сверены отдельно. Это сравнение двух текстов, не conformance-аттестация SDK.
- go-tuf **v2.4.2**, commit
  `f5edbde31e5507f46db2069402dc38903fe6d9d4` — current `windows/go.mod:13`.
  Fresh HTTPS source по immutable commit; `trustedmetadata.go` по tag-path
  v2.4.2 совпал побайтно. API tag lookup HTTP403 не выдан за успех.
- Current project source: `windows/backend/delivery/` и узкая цепочка retention
  в `windows/update_delivery.go`, `windows/delivery_state.go`, metadata binding.
- RFC9162, Experimental, December2021: consistency/inclusion/STH limits.
  RFC9334, Informational, January2023: freshness, timestamps/nonces/epoch IDs.

Точные URL, прочитанные разделы и статусы получения — [SOURCES-010](SOURCES-010.md).
Own captures: `.artifacts/research/theory-05/2026-10-27-server-floor-contract-res7/`.

## 2. R05-010-A — persistable predecessor не равен указателю TrustedMetadata

### 2.1. Три разных состояния

- **SDK mutation**: поле `TrustedMetadata` изменено в памяти. Это факт исполнения
  алгоритма по source, а не receipt workflow acceptance.
- **Accepted progress**: конкретный шаг выбранного TUF workflow разрешил принять
  exact input и сделать его предшественником следующей операции.
- **Durable cut**: платформа подтвердила согласованную запись такого progress с
  lineage/cache/reset provenance и нужной storage-моделью. Это не свойство имени
  класса Updater или одного `os.Rename`.

Факт подписи не заменяет проверки версии, связей, срока и полномочия роли.
Без указания provenance слово «intermediate» двусмысленно: оно может означать
ранее принятый локальный cache или **новый** объект, который сейчас отвергнут.

### 2.2. Нормативная матрица

| Кандидат / точка остановки | Разрешённое сохранение | Чего не выводить |
|---|---|---|
| Root N+1 не проходит подписи старого и нового quorum либо версию N+1 | Сохранить прежний root; отвергнуть offending candidate | Самоподпись кандидата не authority |
| Root N+1 прошёл old+new threshold и последовательность; следующий root/роль недоступны | TUF5.3.7–8: принять и записать verified root **до** дальнейшего прохода | Полный target/profile ещё не принят |
| Подписанная последовательная root chain; конечный root expired | TUF5.3.8 предшествует5.3.10: достигнутый root progress уже сохранён; update cycle abort | Expired final root не допускает загрузку нового timestamp/target |
| Новые timestamp/snapshot ключи после допустимой rotation | TUF5.3.11: dependent timestamp **и** snapshot history инвалидируется в указанной области | Не сбрасывать application epoch/ban/history; новый номер root без key rotation недостаточен |
| Новый timestamp: подписи/version/reference прошли, expiry не прошёл | TUF5.4.4: discard новый timestamp;5.4.5 persistence не достигается | Мутация `trusted.Timestamp` не создаёт persistable floor |
| Новый timestamp принят; snapshot/target/application затем отказали | Timestamp — допустимый predecessor; последующий отказ его не отменяет | Progress не является proof приложения |
| Ранее принятый timestamp теперь expired | Сохранение исторического floor допустимо; его действие как live metadata прекратилось | Нельзя считать его новым acceptance или permit |
| Новый snapshot не совпал с timestamp version, либо expiry не прошёл | TUF5.5.4/6: discard candidate; сохранить допустимый предыдущий progress | `trusted.Snapshot` не receipt финальных checks |
| Новый snapshot принят; targets/артефакт позднее отказали | Допустим snapshot predecessor со связями и target-reference floors | Не публиковать частичный artifact proof |
| Новые targets неверной version/expiry | TUF5.6.4–6: не принимать offending targets | Все полученные package records не verified cache |

Важный порядок:5.3.11 стоит **после** проверки expiry конечного root5.3.10.
Если crash произошёл между root persistence и завершением workflow, нужно знать
область ещё обязательного reset; нельзя выводить её из одного error string или
смешивать нормативный момент удаления с ограничительным pending-обязательством.
Root lineage и application историю этот recovery reset не обнуляет.

### 2.3. Pinned API подтверждает различие

`TrustedMetadata.UpdateRoot` (`trustedmetadata.go:61–94`) проверяет old quorum,
N+1, new quorum, затем назначает Root; expiry deferred до UpdateTimestamp.

`UpdateTimestamp` (`103–168`) проверяет подпись/root/version/reference, затем
**назначает Timestamp:151**, лишь потом `checkFinalTimestamp:155`. Expiry error
возвращается после assignment. `UpdateSnapshot(..., isTrusted)` (`179–259`)
аналогично назначает Snapshot:238 перед final version/expiry checks:242.
`isTrusted=true` относится к ранее проверенному cache и пропускает timestamp
length/hash сравнение, не является режимом для произвольного inbound input.

Full `Updater.loadTimestamp:307–351` и `loadSnapshot:355–413` сохраняют **новые**
bytes только после успешного final update. Root loop:487–521 сохраняет очередной
root после UpdateRoot и до следующего шага. `GetTrustedMetadataSet:680–682`
возвращает shallow struct copy с указателями, не список успешных workflow receipts.

`persistMetadata:591–644`: temp write → Close → Rename → readback. В прочитанной
функции **нет File.Sync/directory flush**. Поэтому нельзя обещать power-loss
совместимость или per-root platform durability только по этой реализации.
`DisableLocalCache=true` делает её no-op; это также не per-step durable contract.
Это чтение storage path, не crash-test и не утверждение о каждом OS/filesystem.

### 2.4. Current source изменился относительно R05-001

R05-001 REPORT:52–55 описывал retention из workspace cache, а не указателей.
Этот baseline **не соответствует выбранным current bytes**:

- `retainedState:39–51` вызывает `boundMetadata` для `trusted.Timestamp`.
- `boundMetadata:27–55` рассматривает previous cache и records из
  `fetcher.delivered`, связывая exact bytes по parsed-object equality.
- `timestampContinuity:13–28` проверяет version/reference/content continuity,
  **не final expiry**. `snapshotContinuity:31–51` также не final acceptance.
- `State.Validate/model.go:122–142` → `guardMetadataShape/policy.go:169–225`
  проверяет bounds/shape, положительную version и ненулевой Expires, **не**
  «новый объект прошёл expiry». Это разумно для old historical cache, но не
  достаточно для классификации нового offending record.

#### Символический opposing trace

Предпосылки: исходный enrolled root свежий; previous timestamp T10 и snapshot
reference S20; корректно подписанный **новый** T1000 указывает S20 и expired
**строго до** `TrustedMetadata.RefTime`. Exact package bytes не двусмысленны;
product shape, budgets и role policy соблюдены; подпись предполагается существующей,
не генерируется и не подделывается. Snapshot/targets presence не требуется для
этого раннего отказа. Attack signer/quorum или origin такого T1000 не установлен.

1. Full updater загружает T10, получает T1000 через packageFetcher.
2. `UpdateTimestamp` назначает `trusted.Timestamp=T1000`, final expiry error.
   `loadTimestamp` не сохраняет T1000 в SDK cache и `Refresh` завершается ошибкой.
3. Project `PrepareArtifacts:150–157` всё равно вызывает retainedState.
   `boundMetadata` выбирает доставленные T1000 bytes; continuity допускает
  1000>10 и20≥20; shape допускает ненулевой прошлый Expires.
4. Returned `Progress.NextState()` может содержать T1000, proofs отсутствуют;
   затем `PrepareArtifacts:171–172` возвращает refresh error.
5. `update_delivery.go:87–103` при `retain=true` пытается сохранить progress
   **до** возврата deliveryErr. `retainDeliveryProgress:41–119` проверяет
   source/binding/anchor/time и передаёт policy существующему committer.

**Точный вывод:** есть current-source путь от нового rejected expired timestamp
к returned progress и попытке platform retention. Ранний тезис «этот wrapper
никогда не переносит pointer mutation» больше неприменим. Не доказаны успешный
commit на ОС, штатная достижимость problematic signed input, frequency/severity
или actual persisted lockout. Не объявляется отсутствие всех иных guards:
source/custody/clock/binding и отсутствие artifact proofs здесь сохранены.

Сильнейший контраргумент: lower layer **намеренно** держит expired intermediate
для rollback-check следующего candidate. Это объясняет его внутреннее состояние,
но **не меняет** TUF5.4.4 «discard new expired timestamp» в выбранном full workflow.
Если нужен stricter observation-only denial floor, это должен быть иной явно
утверждённый restriction contract с retention/recovery ценой, не подмена термина
«accepted TUF progress». Хранить offending input для bounded диагностики можно
отдельно от active role cache, не создавая trust этим фактом.

### 2.5. FUP-7-01: end-to-end границы и сильнейший противоположный случай

Для trace из §2.4 различаем **три события**, а не называем всё словом «accepted»:

| Событие | Достижимый путь / guards | Установленная область |
|---|---|---|
| SDK pointer и returned Progress | root-policy, source Validate/Clone, ctx, requests/budgets, strict package/metadata shape; `updater.New`; TUF signature/version/reference; `boundMetadata` exact parsed equality и отсутствие неоднозначных original bytes; continuity; `State.Validate` | При описанных символических предпосылках T1000 доходит до returned `Progress.NextState`; это **source-вывод**, без исполнения |
| Возможное сохранение bytes | `ApplyUpdate/updateapp.go:156–197` при актуальном intent и operationMu вызывает `prepareVerifiedUpdate(...,true)`; directory/policy/state/clock/read/decode gates; наличие vault, RecoverImport; SourceDigest/CheckDeliveryBinding; anchor/time; EncodeTrustPolicy; prepared snapshot owner/source; повторный fence; fixed four-target filetxn | Есть **условная попытка записи** new policy bytes; actual disk commit/readback/power-loss durability не установлены |
| Признание metadata действующими / допуск артефакта | После retention `prepareVerifiedUpdate:102–103` возвращает исходный deliveryErr; `ApplyUpdate:178–196` прекращает применение; proof absent. Повторный TUF refresh и finalPolicy не снимаются наличием retained bytes | В данном вызове **нет нового допуска**; сохранённое условное T1000 — не действующий target/profile permit |

Дополнительные guards возможной записи прослежены до implementation:

- `retainDeliveryProgress` отказывает при nil/mismatched policy/state, missing
  enrolled delivery/vault, несовпадении source digest, потере anchor или
  понижении ClockFloor, недопустимой policy schema/shape.
- `PrepareImportSnapshot(nil)` не импортирует credentials; загрузка/ownership
  прежнего encrypted vault всё равно может отказать. Никакого реального vault
  исследование не читает. `WithPreparedSnapshot/identity.go:139–153` проверяет
  owner и source-hash same-Manager snapshot; это не межпроцессный OS-CAS.
- Перед commit и в `BeforeCommit` fence повторно загружает **в product source**
  policy/state и сравнивает exact source; операция может быть прервана конфликтом.
- `importTargets/onboarding_import.go:21–23`: фиксированы keys/envelope/version/
  vault. Только keys policy получает Data; прочие `Keep=true`.
- `filetxn.Store.Commit/filetxn.go:67–136`: validate fixed paths/unique names/
  size/Keep shape; existing journal check; directory/staging; read originals;
  `writeSync` каждого backup/new/manifest; BeforeCommit; publish journal;
  atomicWrite только non-Keep; removal recovery barrier — source commit point.
  Любой error может вызвать Recover либо оставить uncertainty/barrier.
  Наличие `File.Sync` на этом **отдельном** platform path не переносится назад
  на updater.persistMetadata, где Sync отсутствует; directory/OS semantics и
  отсутствие malicious restore всех backups этим чтением не доказаны.
- Ненулевой injected committer — отдельная доверенная граница; concrete native
  ветка `committer==nil → tx.Commit` рассмотрена, иной callback не аттестован.

После **условно успешной** записи T1000 следующий честный updater может загрузить
его как **expired historical intermediate** (ErrExpiredMetadata является
ErrRepository; upstream `loadTimestamp:316–325` допускает продолжение). Однако
пришедший T11<T1000 будет отвергнут version-check:133–139; новый T1001 должен
пройти final expiry и все downstream gates. Таким образом, возможное последствие
trace — дополнительный denial/lockout при подходящем signer/input, **не bypass**
срока или выполнение старого артефакта. Уже adopted data plane не исследовался:
ему нельзя приписывать остановку только по отказу update-подготовки.

**Сильнейший противоположный случай:** исходный T10 ранее accepted и сейчас
валиден; следующий inbound T1000 имеет invalid signature. Upstream
`UpdateTimestamp:125–127` возвращает error **до** assignment:151, поэтому pointer
остаётся T10. boundMetadata выбирает original previous T10, continuity сохраняет
его, State.Validate проходит; returned progress может обновить ClockFloor,
сохранив T10. Retention predecessor при ошибке нового record здесь правильна,
и blanket «не сохранять никакой progress после error» ломает это свойство.
Аналогично valid root successor или timestamp, принятый до отказа следующей
роли, остаётся разрешённым predecessor; разрешается predecessor, не offending
record. Контрпример не требует генерировать bogus signature/fixtures сейчас.

**Граница R05-010-A:** доказан условный returned-progress trace и прослежена
условная попытка записи. Не заявлены actual persisted offending metadata,
воспроизведённый lockout, runtime security bypass либо blanket regression.

## 3. R05-010-B — два ограничения знания offline latest

### 3.1. Определения и модель противника

- `LatestKnown(C)`: наибольшая **по разрешённой lineage/role семантике**, а не
  произвольному integer max, подтверждённая клиентом позиция.
- `LatestPublished(t)`: позиция указанной authority/репозитория в момент t;
  требует определённого порядка публикаций. При fork «latest» неоднозначен.
- `Admissible(C,t,purpose)`: действие допускается доступными доказательствами
  данной цели/срока; не эквивалентно знанию глобального latest.

Предполагаются честный verifier и неповреждённые primitives подписи/hash.
Противник может воспроизвести ранее допустимые signed bytes и откатить **весь**
co-rollbackable client bundle: state, software checkpoints, local log heads,
clock floor, receipts. Он не обязан подделывать подпись. Независимого внешнего
input и surviving non-rollbackable anchor нет. Рассматриваем гарантию различения
истории, не способность атакующего произвольно менять код verifier.

### 3.2. Утрата собственной истории: доказательство неразличимости

Пусть после legitimately accepted cut C0 клиент ранее принял C1>C0.
Сравним истории:

- H0: клиент дошёл только до C0; offline input — ранее допустимый package P0.
- H1: клиент дошёл до C1, затем весь перечисленный bundle восстановлен до C0;
  offline input — **тот же** P0.

Наблюдаемый вход verifier `(trusted baseline, C0, local receipts/log heads,
input P0, доступная time-модель)` совпадает. Для deterministic алгоритма итог
совпадает; для randomized алгоритма совпадает распределение при одинаковом
распределении свежих случайных битов. Значит, алгоритм не может одновременно
подтвердить «отката не было» в H0 и гарантированно выявить rollback в H1.
Всегда BLOCKED сохраняет safety ценой отсутствия обещанной recovery-liveness,
но не становится способом **узнать** историю.

Это информационный аргумент. `hash-chain`, AEAD/MAC/signature checkpoint,
копия checkpoint в том же backup и locally stored monotonic integer не помогают:
в H1 восстановлен весь ранее корректный префикс вместе с доказательствами.

### 3.3. Неизвестное внешнее событие: сильнее, чем проблема backup

Даже если C0 нельзя локально откатить и сохранились **честные часы**, два мира
могут отличаться только publication C1/revocation **после** выдачи P0. Выбираем
t внутри ещё действующего validity interval P0; клиент offline, external input
одинаков. В H0 ничего нового не опубликовано, в H1 policy изменена. Клиент не
знает, какой мир действителен. Secure time может исключить expiry/freeze за
пределами lease, но не мгновенный отзыв **внутри** ещё valid lease.

TUF6.1 прямо допускает withholding new roots: freeze ограничен expiry последнего
доступного root, не утверждением «ничего новее не существует». Missing N+1/404
или остановка по MaxRootRotations также не signed proof latest. Это особенно
видно у project packageFetcher: отсутствие файла пакета имитирует404; package
может быть bounded incomplete без нового доверенного утверждения полноты.

### 3.4. Старый signed floor + fresh consent — контрпример

Пусть recovery artifact F0 действительно был подписан authority K0. Позднее
допустимый C1 сменил/отозвал K0 либо поднял policy floor. Восстанавливаются F0,
C0 и все локальные receipts; пользователь **сейчас** честно подтверждает restore.

Подпись доказывает действие K0 над F0; consent доказывает текущую пользовательскую
авторизацию выбора. Ни один факт не доказывает, что K0 **сейчас** полномочен, F0
не superseded и scope имеет право снять C1-ограничения. Даже fresh nonce response
от прежнего key holder не доказывает его текущую полномочность без отдельной
принятой authority chain. Нельзя превратить recovery новой учётки в «clean install»
и обещать сохранение прежних bans/floors, если сама прежняя история потеряна.

## 4. R05-010-C — bounded recovery contract и готовые механизмы

### 4.1. Что TUF предоставляет вместо магической server-floor позиции

Стандартные средства: OOB trusted root, последовательная old+new threshold root
rotation, signed timestamp/snapshot/targets и target length/hash binding,
expiry и role-local rollback checks. **Отдельного универсального wire-типа
«подписанная серверная floor-позиция клиента» в reviewed workflow/API нет.**
Это ограниченный вывод о перечисленных файлах API/spec, не поиск всех возможных
extensions и не заявление, что никакое приложение не может иметь checkpoint.

Application facts можно переносить **внутри existing signed target payload**
под нужной уже принятой delegation; `GetTargetInfo` и `VerifyLengthHashes`
связывают exact bytes, но смысл lineage/purpose/floor/lease задаёт application.
Подпись/большой version от online timestamp роли не дают ей право устанавливать
recovery governance или обнулять application bans. Не требуется свой cipher,
nonce-signature protocol или новый центральный сервис.

### 4.2. Минимальный логический contract, не новый DTO

1. **До операции**: определить purpose (catch-up / repair / new-recipient enrollment /
   resume), adversary/storage модель, trusted lineage и требуемую давность знания.
   Не брать recovery authority из восстановленного candidate.
2. **Recovered inputs**: owned exact bytes; limits; independent surviving history
   basis если требуется сохранение прежних floors. Если basis утрачен, назвать
   результат `HISTORY_UNPROVEN`, не «не было новых требований».
3. **TUF progression**: проверять каждый permitted successor готовым upstream;
   полученный error не авторизует offending record. Root/cache reset provenance
   и predecessor commit отделить от нового profile acceptance.
4. **Checkpoint comparison**: связывать independent anchor с enrolled lineage и
   exact position/digest/accepted generation. Он должен переживать **ту же**
   rollback-модель, против которой заявлена защита; compare-only, если anchor
   нельзя достоверно обновить. Сам по себе anchor не всегда хранит весь metadata.
5. **Time/freshness**: выбранный источник определяет, насколько свежее знание
   обосновано. При UTC interval `[L,U]` conservative expiry admission требует
   `U < expires`; один lower-bound ClockFloor не даёт upper bound. Если обновление
   использует fixed reference time TUF5.1, runtime admission после медленного
   commit повторно проверяет нужный срок. Не использовать UnsafeSetRefTime в
   production как способ «оживить» старый объект.
6. **Commit**: source-CAS, exact old/new cut, reset obligations; confirmed durable
   receipt либо explicit uncertainty. Artifact failure не откатывает уже допустимый
   predecessor. Coherence/antirollback/availability — разные свойства storage.
7. **После commit**: admission по текущим policy/purpose/owner/clock, отдельные OS/
   VPN readiness gates; ни persisted checkpoint, ни TUF target proof не Ready.

При неизвестном commit возможны несколько cuts. Разрешаем только действие,
чей gate проходит во **всех** остающихся cuts, либо ограниченное reconcile/blocked.
Не выбирать наиболее удобный cut по score или подтверждению пользователя.

### 4.3. Альтернативы anchor: сервер не обязателен

| Механизм / доверенная предпосылка | Что добавляет | Что не добавляет / цена |
|---|---|---|
| Сохранившийся отдельно custody checkpoint (иной носитель/оператор вне rollback scope), доставленный offline | Нижнюю известную позицию/историю **as-of** этого checkpoint; можно исключить restore ниже неё | Не latest после даты checkpoint; носитель может быть потерян/устареть. Обычный второй файл в том же backup не подходит |
| Hardware non-rollbackable register/counter, связанный с lineage/digest и transaction model | Можно обнаружить локальную позицию ниже пережившего counter | Не доказывает внешнюю текущую policy; platform availability/atomic binding/wear/reprovisioning не установлены. Реальный TPM/Keystore не проверен |
| Доверенный surviving UTC source + existing TUF expiry | Ограничивает приём expired evidence и максимальную автономную lease по выбранной policy | Не выявляет rollback между ещё valid cut и не сообщает latest. Secure time не history anchor |
| Своевременный контакт с независимо принятой recovery authority, включая offline доставку ею свежего approved package | Новую подтверждённую **as-of** позицию и permitted successor chain | Доступность/custodian/privacy стоимость; одна stale signature/URL не достаточны. Это может быть сервер, но может быть управляемый офлайн канал |
| Transparency/witness как отдельно доверенный surviving head | RFC9162 consistency доказывает append-only **между предъявленными heads**; inclusion — наличие leaf в данном tree | Log не authorizes policy. Старый valid head/consistency не latest; gossip в RFC9162 не определён; same-backup head откатывается. CT reference не ready TUF-integrated implementation |
| Явно ограниченное offline resume ранее adopted lease | Может сохранить полезную доступность, если выбранная application policy это обещает и срок/история доказаны | Не new acceptance, не instant revocation; произвольное продление запрещено |
| Independently authorized OOB repair после root-threshold compromise | TUF2.1.1 предусматривает OOB замену root keys при threshold compromise | Нельзя доверять new self-signed root или fresh consent без внешнего основания; если verifier/OS захвачен, безопасность на устройстве не доказана |

RFC9334§10 различает trustworthy time, nonce и epoch ID; одновременно прямо
предупреждает, что state может измениться сразу после evidence и что actual
claim values могли быть старыми **до подписи**. Здесь используется архитектурное
ограничение freshness, не перенос RATS wire-протокола в VPN.

### 4.4. Exact API и boundary адаптации

| Ready API | Узкая пригодность / граница |
|---|---|
| `config.New(remoteURL, rootBytes)` → `updater.New(cfg)` | Trusted root — явный input, не discovery latest; constructor пишет initial root при включённом cache. Поэтому изолированный workspace и проверенная lineage обязательны |
| `cfg.Fetcher`, length/rotation/delegation limits | Existing bounded package transport; no live fetch требуется от научного пакета; no cache-disable trick для durable guarantees |
| `Updater.Refresh()` | root→timestamp→snapshot→targets. Partial failure требует классификации progress, не dump всех pointers |
| `GetTrustedMetadataSet()` | Shallow inspected state; не accepted-step ledger и не raw-byte receipt |
| `GetTargetInfo(path)` + `TargetFiles.VerifyLengthHashes(bytes)` / `DownloadTarget` | Target identity under accepted metadata; application recovery/policy semantics и custody отдельно |
| `TrustedMetadata.UpdateRoot`, `UpdateTimestamp`, `UpdateSnapshot` | Lower primitives годятся как reference/substep seam **только** при сохранении full workflow checks и provenance; не готовая замена полного updater |
| `delivery.PrepareArtifacts` → `Progress.SourceDigest/NextState/ProofFor` | Current project source gate; returned NextState classification требует review R05-010-A; ProofFor absent до полного успеха |
| `retainDeliveryProgress` → existing four-target committer | Отдельный platform persistence seam; source-fence guards существуют, OS durability/rollback scope не доказаны |

go-tuf v2.4.2: upstream Go≥1.25.0, Apache-2.0 (LICENSE прочитан), Windows manifest
Go1.26. Никаких imports/SDK patches/dependency upgrades не предлагается выполнять
этим исследованием. Android/whole-AAR compatibility и trust-provider consumer
этим пакетом не проверены; спецификационный contract применим отдельно от binary.

## 5. Цена, falsifiers и конечная приёмка — NOT_RUN

| Case | Требуемое различие |
|---|---|
| P1 | Wrong old/new root quorum либо not N+1: candidate discarded, previous progress неизменен |
| P2 | Valid root chain, final expired root: разрешённый root predecessor сохраняется, new target запрещён |
| P3 | Новый signed expired T1000: SDK intermediate и accepted-step progress различены; current returned Progress path отдельно воспроизвести только после разрешения |
| P4 | Valid new timestamp, затем wrong snapshot hash/version/expiry: timestamp retained, offending snapshot не становится accepted cache |
| P5 | Ранее accepted cache теперь expired: историческая информация не очищается и не даёт runtime permit |
| P6 | Online-key rotation и crash на root/reset boundary: область cache invalidation восстановима; application history не сбрасывается |
| P7 | Cancel/cleanup/write failure: подтверждённый predecessor не равен artifact proof; uncertain commit не выдаётся как nothing happened |
| P8 | Restore C0 со всеми software anchors после C1: typed history-unproven, no promise local rollback detection |
| P9 | Тот же restore с surviving bound anchor C1: rollback ниже anchor обнаруживается при доказанной binding/custody model |
| P10 | Честные часы, P0 ещё valid, внешний C1 withheld: acceptable-as-of и global-latest различены |
| P11 | Старый signed F0 + fresh consent либо свежая подпись старыми полномочиями: new authority не выводится |
| P12 | Old log head + valid inclusion/consistency: append-only proof не latest/recovery authorization |
| P13 | Two divergent lineage/same-version content: typed conflict, не global max/hash winner |
| P14 | Long commit перешёл expiry: checkpoint retention и fresh runtime admission различены |

Все случаи — **NOT_RUN**. Это не план разрешённого исполнения сейчас.

Цена: bounded original-byte/provenance storage, review каждого step, independent
custody и ограниченный offline horizon, reconciliation при uncertainty. CPU/RAM/
fsync latency/стоимость администратора не измерены. Вечно BLOCKED — допустимое
отрицательное решение безопасности, но не положительный recovery SLA.

Falsifiers: доказанный non-rollbackable anchor меняет часть3.2; новое доверенное
external evidence меняет множество миров в3.3; доказанная compiler/OS/platform
storage модель усиливает durability claims; другое explicit restriction-only
retention contract может изменить оценку current wrapper, но не нормативный
текст выбранной TUF версии. Ни один из этих inputs здесь не подменён догадкой.

<a id="6-предлагаемая-адресная-интеграция-руководителю"></a>
## 6. Применение выводов

1. R05-010-A: уточнить OP-03/TH-03/TUF-progress, запретить наследование раннего
   cache-only утверждения для нынешнего wrapper. Адресный source trace не
   превращать в blanket product defect или автоматический patch.
2. R05-010-B/C: заменить универсальное «fresh consent + server floor достаточно»
   из R05-008 на surviving history basis + independently authorized recovery,
   purpose/time/admission checks и honest as-of ограничения. Сервер необязателен.
3. Per-step versus per-return durability и reset ordering должны быть явным
   выбранным контрактом до платформенной реализации. Не вводить новые TH-ID,
   новые signed protocol или пятую fixed journal запись только из этой модели.

Авторский статус — READY_FOR_REVIEW; независимая приёмка определяется
отдельной рецензией, не этим предложением.

## 7. FUP-7-02 / R05-010-D — основание retention конкретного timestamp

Рецензия приняла **только** условный timestamp → returned-Progress source/model
срез A; это не приёмка нового D или всего A/B/C. Здесь рассматривается
теоретическое исправление одного seam, без нового verifier/patch/SDK-run.

### 7.1. Какой receipt действительно доступен

В просмотренных pinned `metadata/updater/updater.go`, `config/config.go` и
`trustedmetadata/trustedmetadata.go` **нет публичного per-timestamp-step receipt**
или callback после final check: `loadTimestamp`, `checkFinalTimestamp` и
`persistMetadata` непубличны. `Refresh()` возвращает общую ошибку;
`GetTrustedMetadataSet()` отдаёт `Timestamp` и `RefTime`, но не stage outcome.
Это bounded negative source result по названным файлам, не поиск всех возможных
расширений go-tuf. Настройки Fetcher дают fetch-delivery, не acceptance;
лог `Updated timestamp` на151–152 также стоит **до** final check и не receipt.

Lower `TrustedMetadata.UpdateTimestamp` публично возвращает object+error своего
вызова, но current full updater этот локальный result наружу не передаёт.
Использовать lower вызов вместо полного workflow — другая orchestration и
compatibility работа, не рекомендация строить собственный verifier.

**Доступное дополнительное основание для выбранного pin:** post-call timestamp
связать с original bytes и проверить final predicate относительно **того же
SDK RefTime**, а прежний predecessor идентифицировать по prior-acceptance
provenance. Это не формальный upstream receipt и не generic правило для всех
ролей/версий; это narrow source-derived certificate.

### 7.2. Строгое условие и его обоснование

Обозначим:

- `B` — exact bytes **ранее разрешённого** timestamp predecessor в выбранной
  lineage/cache context. Наличие файла/State.Validate само не устанавливает
  prior acceptance. После lawful online-key reset старый B нельзя воскресить
  как действующий role cache; отдельно сохранённая audit history — иной scope.
- `C` — original bytes, однозначно связанные с `S.Timestamp` после данного
  full-updater вызова (`boundMetadata`); `S` — наблюдённый SDK state, `t0=S.RefTime`.
- `H(C)` — C в точности B, есть independently justified prior-acceptance
  provenance и domain/reset semantics разрешают его retention.
- `F(C)` — для ненулевого `S.Timestamp` верны строгие
  `S.Timestamp.Signed.Expires.After(t0)` и
  `S.Root.Signed.Expires.After(t0)`. RefTime известен, неизменён и относится к
  этому updater. Он не объявляется authenticated UTC/current runtime time.
- `G(C)` — обязательные guards exact-byte/source/shape/continuity/root-lineage/
  reset/owner/cancellation пройдены; binding не двусмыслен, root/context не stale.

Для непаниковавшего, изолированного current full workflow, без внешней мутации
shallow SDK pointers/RefTime, допустимое **restriction-only retention** имеет
достаточное условие:

`G(C) ∧ (H(C) ∨ F(C))`.

Операция сохраняет только допустимую историю/ограничения, **не** артефакт или
runtime permit. Это достаточный criterion выбранного seam, не доказанный минимум
числа полей и не unconditional production API. Нельзя исполнять эту формулу без
её provenance/ownership предпосылок.

**Почему F отделяет rejected incoming timestamp:**

1. Новый full updater начинает с `Timestamp=nil` (New/NewTrustedMetadata).
2. Единственное присваивание Timestamp в рассматриваемом workflow —
   `UpdateTimestamp:151`, после проверки типа, root freshness, подписи,
   timestamp/reference rollback условий:109–146.
3. После assignment у timestamp осталась **одна** final проверка:
   `checkFinalTimestamp:163–168` → `IsExpired(RefTime)`. У snapshot это иначе:
   есть ещё final version-link predicate, перенос одного F запрещён.
4. При F final timestamp/root predicates выполнены. В полном workflow их
   успешность не отменяется более поздним snapshot/targets/persist error.
   Проверяется exact retained object, а не version/Expires произвольного input.
5. Публичный `IsExpired` использует `RefTime.After(Expires)` и допускает equality;
   TUF§5.4.4 требует Expires **выше** fixed start time. Поэтому здесь выбрано
   строгое `After(t0)`: equality не объявляется accepted-new по этому criterion.

Следовательно, **новый C, expired до t0, не проходит F**, несмотря на valid
signature, высокий version и pointer assignment. Если C не H, его не выбирают
для нового accepted-cache retention; допустимый B сохраняется отдельно.

Для C=previous B и F=false prior H сохраняет именно historical predecessor —
это не permission считать expired B актуальными метаданными. Свежий совпавший
pointer может быть результатом local cache load: F доказывает прохождение нужных
final условий в этом workflow, но **не** приём нового incoming равноверсионного
объекта. Происхождение candidate и назначение retention всё равно обязательны.

### 7.3. Маленькая матрица observed facts → retention / unknown

Все строки предполагают G и отсутствие недекларированной pointer/RefTime mutation.

| Наблюдённые факты | Что разрешено утверждать / сохранить |
|---|---|
| C=B, prior acceptance/context известны, reset не инвалидировал B; current new record отклонён | Historical H: сохранить B независимо от expiry сейчас. Не записывать новый rejected input |
| C новый, exact binding есть, F=true; Refresh позже упал на snapshot/targets | Final timestamp conditions подтверждены source-derived F; возможен restriction-only retention C без artifact proof и без требования Refresh==nil |
| C новый, F=true; timestamp final check прошёл, но SDK persist завершился I/O error | Пройденный final predicate не отменён. Возможен отдельный platform retention через свой receipt/CAS; успешный SDK write/durable commit **unknown** |
| C новый, F=false и H=false | Не выбирать C как accepted timestamp progress; сохранить допустимый B, если он есть. Это не превращает rejection в data-plane Disconnect |
| C=B, same-version remote input; full updater вернул equal-version normal branch | Нового acceptance incoming input нет; pointer остаётся B. Применяется H/F к B, не к delivered равноверсионным bytes |
| Post-cache bytes=C, before-cache отсутствовал либо достоверно отличался; directory owned/unmodified, cache enabled, C exact-bound | Положительное свидетельство выполнения verified-step write, поскольку SDK пишет новый timestamp только после успешного UpdateTimestamp. Не power-loss/durable platform receipt; при чтении/identity uncertainty — unknown |
| Файл cache просто присутствует/равен C, но before/prior provenance неизвестны, либо C=B | Файл мог быть seeded старым cache; **это не receipt нового шага**. Без H/F/другого verified receipt classification unknown |
| timestamp key/reset context изменился, C совпадает с pre-rotation B | Не воскресить старый role floor из H без разрешённого нового context. Current `retainedState:43–44` уже возвращает rotation continuation; root/reset progress — отдельный результат |
| Pointer nil, original bytes ambiguous, SDK owner/RefTime нарушен или classification не доказана | Нет новой принятой timestamp позиции; permitted previous cut сохранить по его provenance, новый C — unknown |

Post-cache чтение — **альтернативный положительный witness**, а не необходимое
условие: persist может отказать после verification; прежний файл мог не
измениться; после error чтение может быть невозможным. Выбранный F использует
уже доступный state API и не требует новой filesystem receipt подсистемы.
Существующий `readCache` — bounded reader; full updater сам читает cache
opaque `os.ReadFile`. Pre-Refresh byte equality в `openWorkspace` не доказывает,
что later SDK read succeeded и что new accepted timestamp persisted.

### 7.4. Контрпример ошибочному критерию и цена

**Неверно:** «`Refresh` вернул error — значит timestamp нельзя сохранить».
H0: новый T11 прошёл UpdateTimestamp final check и persistence, но snapshot
hash неверен. H1: новый T11 expired и остался pointer. Оба `Refresh!=nil`;
в H0 predecessor T11 допускается, в H1 новый T11 отвергнут. При разных
источниках error может содержать дополнительные hints, но только общий bool
их не различает. F разделяет эти случаи на выбранном pin; H сохраняет old B.

Ещё более тонкая ветка: B ранее accepted, **теперь expired**; remote прислал
тот же version (может даже иной Expires). Local `UpdateTimestamp(B)` держит
expired pointer, а remote same-version branch возвращает ErrEqualVersionNumber
**до** assignment/new expiry check. `loadTimestamp` превращает эту ошибку в nil
на337–340, но дальнейший `UpdateSnapshot` отказывает по expiry B. Поэтому даже
private `loadTimestamp()==nil` не всегда означает «принят новый fresh timestamp»;
нужны H/F и происхождение. F(B)=false; H(B) разрешает historical retention,
не refreshed permit. Равноверсионный rejected candidate не освежает B.

Цена narrow adaptation: ещё одно value-only strict deadline comparison по
SDK RefTime + provenance-развилка и существующие guards, **без** новой crypto
или всего replay verifier. Однако доказательство привязано к точному assignment/
final-check порядку v2.4.2: обновление SDK требует повторного review; новые
post-assignment guards могли бы сделать F недостаточным. Shallow objects не
safe concurrent: read-only ownership и immutable captured time обязательны.
Артефакт/native допуск и durable commit по-прежнему отдельны. Никаких замеров
или обещания нулевой дополнительной цены CPU/RAM.

### 7.5. Что новое и что осталось NOT_RUN

Crosswalk к baseline: §2.3 уже показывает assignment/final-order, §2.5 —
потребителя и отсутствие нового artifact permit, §4.4 — отсутствие step ledger
в GetTrustedMetadataSet. **Новая ограниченная дельта D** — доступный derived F
с prior H/G, равноверсионная normal branch и persist-error distinction;
не новый обзор TUF и не повтор утверждения A.

Будущие случаи D1–D6 (все **NOT_RUN**):

1. Fresh new timestamp + late snapshot failure → F и retention predecessor.
2. Expired new timestamp + SDK pointer → F=false, offending C не accepted.
3. Expired previously accepted B + same-version incoming → H, не new acceptance.
4. Fresh final timestamp + persist/write/readback failure → F, disk unknown.
5. Missing/stale/ambiguous cache и invalidated root-key context → honest unknown/
   continuation, не resurrect old floor.
6. RefTime mutation/concurrent pointer update/SDK semantic drift → criterion
   unsupported до нового review, не silently successful retention.

D — **READY_FOR_REVIEW**, не принят автоматически вслед за A.
