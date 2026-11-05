# R28-Z07 — ResetAllowed, surviving quorum и повторное принятие timestamp

Срез **28 октября 2026**. **READY_FOR_REVIEW**, не ACCEPTED.
Source availability: **SOURCE_AVAILABLE** в D:/SNOWDEN-SYSTEM-V3.
Описания «current» относятся к исходникам этого среза.

## 1. Вопрос и точная дельта

Какие изменения role keys/thresholds разрешают reset и когда старый timestamp
остаётся допустимым, порождая другую recovery-историю?

Baseline уже подтверждает S-only full-replacement trace, сохранение допустимого
predecessor и различие mutation/acceptance/durable cut. Новая дельта этого пакета:

- точная матрица overlap, keyID/material, threshold-only, dual signatures и
  S→S′→S, а не отождествление всех вариантов со словом rotation;
- различие **права cache reset** и **исключения повторного приёма старого envelope**;
- actual current wrapper guard и следующий fresh-pass replay, который этот
  guard сам по себе не исключает;
- минимальное информационное обязательство reset/admission provenance,
  включая continuation через expired intermediate roots.

Никакие подписи, SDK, tests/fixtures, native filesystem, реальные state/vault/
профили не создавались и не исполнялись. Числа/keys ниже — символические examples.
Нет live exploit/severity, доказанной записи или product conformance verdict.

## 2. Primary pin/caller и ограничения нормы

### 2.1. Точная спецификация

Получен HTTPS текст **TUF1.0.37**, Last modified **8 October2026**.
Прочитаны §§4.2.1/4.3 (keyID/threshold), 5.1–5.6 (workflow), 6.1 (migration).
§5.3.11 буквально требует:

> If the timestamp and / or snapshot keys have been rotated, then delete the
> trusted timestamp and snapshot metadata files.

Эта стадия находится **после**5.3.10 final-root expiry check. Intermediate
roots, прошедшие old+new root quorum и N+1, принимаются/сохраняются раньше,
на 5.3.7–8. Timestamp после reset проверяется по 5.4: подпись новой authority,
версии относительно действующей trusted history, expiry, затем persistence.

**Норма не даёт исчерпывающей boolean-функции** для key addition/overlap/
threshold-only/key-material aliases или возврата S→S′→S. Поэтому нельзя
представить свой `keysChanged` как буквальный стандартный ResetAllowed для
этих случаев. Для полной замены S с отзывом прежнего quorum основание принято
baseline review; остальное требует явно выбранной adopter semantics/POUF.
Threshold-only не назван здесь отдельным разрешением обнулить version history.

### 2.2. Проверенные references

1. **go-tuf v2.4.2**, commit
   `f5edbde31e5507f46db2069402dc38903fe6d9d4`.
   `windows/go.mod:13` закрепляет этот pin; свежий proxy module-info подтверждает
   tag/ref/commit (Time2026-05-19T08:44:09Z). Read source: `UpdateRoot`,
   `UpdateTimestamp`, `VerifyDelegate`, `loadRoot/loadTimestamp`, persistence.
2. **python-tuf7.0.1**, commit
   `49eb1da48c58cf0cbb0f2e490f7ad39fbd8c1f15`.
   Получен tag→annotated-tag→commit (tagger date2026-09-01T12:02:05Z).
   Read updater/trusted set/API payload плюс exact recovery test и simulator.
3. Дополнительный opposing reference — **legacy go-tuf0.7.0**,
   commit `6ad7fe593e4042db3544c4b0fedbe66bac371c42`.
   Его reset predicate явно привязан к старой v1.0.19 и ongoing draft PR;
   не копируется как реализация современной 1.0.37.

Два современных references имеют одинаково важный scope: root updates перед
local timestamp load; lower-set комментарий «timestamp/snapshot cannot yet be
loaded» **не** означает отсутствия прежнего cache на диске. В прочитанных full
Go/Python workflow нет отдельного cache-deletion шага 5.3.11. Повторная подпись
старого cache может быть отвергнута **или** остаться достаточной — это решает
конкретный envelope/новые authority, а не название функции `rotate_keys`.

## 3. R28-Z07-A — reset и exact-envelope quorum различаются

### 3.1. Определения

Для role r и root R:

- `K_r(R)` — identities keys, авторизованных role r; учитываются scheme,
  public material и корректный signature keyID binding, не только строковый label.
- `q_r(R)` — требуемый положительный threshold.
- `V_r(M,R)` — множество **действительно проверяемых** уникальных contributions
  в конкретном signed envelope M под role r из R. Недостаточно считать строки
  signatures. В Go pin duplicates material считаются один раз по PKIX fingerprint;
  Python result использует verified keyIDs своего API, что не переносится как
  identical alias semantics вне well-formed distinct-key области examples.
- `Q_r(M,R) ≡ |V_r(M,R)| ≥ q_r(R)` — envelope удовлетворяет текущему quorum.
- `A(chain,policy)` — **разрешение reset** из verified root lineage и принятой
  transition policy. Это не результат Q и не факт наличия файлов.
- `P(M,cut)` — completed acceptance M **после** соответствующего reset-barrier,
  а не сохранённый pre-reset record и не mutated pointer.

Необходимо различать два вывода:

1. Verified authorized rotation может разрешить invalidate old active T/S cache.
2. Из этого **не следует**, что после очистки `Q_timestamp(old,Rnew)=false`.

При `Q=true`, свежих final root/record, правильном формате и остальных gates
TUF может **заново** принять старый M в пустом новом context. Подпись не хранит
локальный факт «до/после reset». Новый acceptance создаёт новые applicable floors,
даже если signed payload и версия те же, что раньше.

### 3.2. Матрица переходов

Все root transitions предполагают valid old+new root quorum/N+1, final root
freshness и отсутствие иных schema/budget отказов. Ключи a,b,c,d различны,
подписи зафиксированы symbolically; cryptography не исполнялась.

| Переход role authority | Нормативный reset / выбранный scope | Допустимость exact old record и counterexample |
|---|---|---|
| S-only disjoint full replacement, T неизменён | Baseline принятого 5.3.11: удалить оба T/S caches | Old timestamp signed T всё ещё Q=true; S-replacement не отзывает T. После completed reset timestamp может вернуться; old snapshot S может не пройти новую S′ подпись |
| T и S disjoint replacement, old envelope содержит только old signatures | Разрешённая полноценная key rotation; оба caches reset | Old timestamp Q=false. Это конкретная гарантия paired recipe, не всех paired list diffs |
| `2-of-{a,b,c}→2-of-{b,c,d}` | Spec не определяет detail reset predicate; current material-diff policy рассматривает как reset | Old M signed a,b →1/2, rejected; M signed b,c →2/2, acceptable. Одинаковый diff — противоположные Q |
| Add d: `2-of-{a,b,c}→2-of-{a,b,c,d}` | Не приписывать норме auto-reset по addition. Current wrapper material-diff=true | Old M signed a,b проходит 2/2; изменение множества не убирает старый quorum |
| Remove a: `2-of-{a,b,c}→2-of-{b,c}` | Уменьшение набора требует explicit rotation policy, не inference по integer diff | M signed a,b отвергнут; b,c проходит. «Отозвано меньше threshold» не отвечает за every-record Q |
| Threshold ↑: `2-of-{a,b,c}→3-of-{a,b,c}` | Ключи не изменены; отдельного reset-права в5.3.11 не установлено; current wrapper reset=false | M signed a,b отвергнут; a,b,c проходит. Даже если old cache не загрузится, forgetting version требует отдельного contract |
| Threshold ↓: `3-of-{a,b,c}→2-of-{a,b,c}` | Threshold-only не самостоятельный reset; current wrapper reset=false | Каждое old envelope, удовлетворявшее 3 distinct keys, проходит 2. Меняется assurance, не выпускается право rollback |
| Новый root version/expiry/root-role keys, T/S authority неизменна | Нет online-role reset только от роста root version/root-only change | Старые T/S по-прежнему сравниваются с прежними floors; final root freshness отдельно |
| KeyID relabel с тем же key material/scheme | Произвольный alias не считать spec-valid rotation: §4.3 требует correct computed ID; current policy дополнительно ограничивает Ed25519/fields | Same material не новая независимая authority. Old envelope без нового matching signature keyID может не пройти; с supported matching contributions может пройти. Alias acceptance не доказана для проекта |
| Material reuse после удаления/re-add | Named keyID/name diff не доказывает новую secret custody; нужен valid canonical ID/scheme | Подпись старым материалом остаётся математически проверяемой при возврате той же authority; compromise не исцеляется переименованием |
| Disjoint IDs/material sets, но old envelope уже dual-signed old+future-authorized keys | Key-set disjoint не значит Q=false для **всех** signatures envelope | Дополнительные валидные new-key signatures удовлетворяют new quorum; old root ранее мог игнорировать их. Никакой подделки не требуется |
| `S→S′→S` в verified цепочке | Полный путь имел изменения даже если endpoints совпали; ResetAllowed зависит от принятой transition semantics | Финальный S снова допускает старые S signatures. Endpoints equality не отсутствие rotation; OR path-diff не nonreplay guarantee |

**Точное достаточное условие исключения старого timestamp replay** в конечном
указанном наборе retained envelopes H:

`∀ M ∈ H: |V_timestamp(M,Rnew)| < q_timestamp(Rnew)`.

Это **условие signature rejection**, не разрешение reset и не гарантия future
liveness. Альтернатива rejection — strict expiry/application deny, но expiry
не стирает previously accepted rollback history и может оставить recovery
недоступным. Для general неизвестного корпуса H условие не доказано конечной
проверкой нескольких известных envelopes. Нельзя обещать reject future inputs,
если signer продолжает иметь возможность их выпускать.

**Полезный ограниченный set-bound:** при фиксированном correct ID/scheme,
различных keys и условии «old envelopes содержат подписи **только** K_old»
максимальный новый quorum равен `|K_old ∩ K_new|`. Поэтому

`|K_old ∩ K_new| < q_new`

достаточно, чтобы **все old-only envelopes** были rejected по новому quorum.
Если intersection≥q_new и old authority была достижима, можно выбрать подписи
intersection и дополнительные old keys до q_old; существует old-accepted
envelope с достаточным new quorum. Это модель доступности необходимых signers,
не утверждение о реальном наличии подписанного документа. Future/dual signatures,
alias/scheme distinctions или reused material снимают old-only premise.
Число **удалённых** keys сравнивать со старым q для этого свойства неверно:
решают surviving eligible contributions и новый q.

### 3.3. Почему два готовых predicates не равны этому условию

**Current wrapper** `retainedRoot/retained_inputs.go:80–91`:

`reset = OR_per_intermediate_root( materialSet_before(T)!=after(T)
                               OR materialSet_before(S)!=after(S) )`.

`roleKeys` принимает threshold для shape validation, но возвращает только
material identity map. Поэтому threshold-only изменение **не** влияет на
boolean; keyID alias без material change также не даёт нового material-set diff.
Правильная сторона — сравнение по каждому verified intermediate root, так что
S→S′→S **внутри одного вызова** не исчезает в endpoint equality.

**Threshold-only opposing guard:** если ↑q исключил old-cache подписи,
bare SDK может продолжить download без загрузки старого timestamp в memory.
Но current wrapper сохраняет `reset=false` и выполняет
`timestampContinuity(previous.Cache,Tnew)` независимо от успешности seed-load.
Уменьшение timestamp version либо snapshot reference будет rejected по
retained_policy.go:21–23. Поэтому отсутствие in-memory old quorum не становится
автоматическим forgetting floor в current source. При full final success с
низкой версией этот retention error всё равно предотвращает выдачу artifact
proof на client.go:156–157. Это existing guard, не deployed тест.

Граница — любой material addition/removal вызывает reset, даже когда прежний
timestamp envelope сохранил достаточный quorum. Это adopter predicate, не
воспроизведённая полная нормативная семантика всех rotations.

**Legacy reference** `client.go:325–359` сравнивает
`|old keyIDs \ new keyIDs| ≥ old_threshold` и удаляет разные файлы по role table.
Это не `Q(oldEnvelope,Rnew)=false`. Два opposing cases:

- Old `2-of-{a,b,c,d}` → new `2-of-{c,d,e,f}`: revoked a,b=2, predicate true,
  но old M signed c,d всё ещё valid2/2.
- Old `3-of-{a,b,c}` → new `3-of-{b,c,d}`: revoked a=1<3, predicate false;
  любое envelope, подписанное **только old set**, имеет максимум 2 new
  contributions и rejected3/3. Additional future-key signatures меняют вывод.

Поэтому legacy threshold-revocation code не готовая замена current policy;
его cache-deletion table (T-only удаляет только timestamp) также не буквальная
современная 5.3.11 (оба caches). Source прямо ссылается на ongoing draft.

### 3.4. Сохранившийся compromised quorum — третья независимая проверка

Связанный разбор C05/HANDOFF (claim B) и REPORT §4 также разделяет reset и
устранение compromised quorum. Это сопоставление авторских выводов, **не**
независимая приёмка R28-Z07; source G3 и явно заданная модель quorum остаются
первичными основаниями.

Пусть `C_priv` — identities приватных keys, которыми противник действительно
может подписывать сейчас (включая копии reused material); `q_new` и `K_new` —
новая role policy. В fixed valid-ID/scheme distinct-key области:

`|C_priv ∩ K_new| ≥ q_new`

означает возможность получить достаточные **новые** signatures. Это иной
предикат, чем Q(oldEnvelope,Rnew): exact old record может Q=false (например,
он раньше signed a,b, а после overlap b,c были скомпрометированы), но противник
может выпустить **другой** fresh record signed b,c. Reset cache не отнимает
у него это право и не доказывает восстановление trust после компрометации.
Обратный случай: dual-signed старый M ещё Q=true, хотя все private keys
оператором сохранены честными — возможен replay/liveness trace без утечки ключей.

Для bounded recovery после compromise нужны раздельно:

1. surviving offline/root authority и допустимый transition/reset;
2. exclusion достаточного **compromised** new quorum в принятой threat-модели;
3. условия new metadata availability/freshness/accepted context;
4. barrier/commit before последующего admission.

Known compromise release/ban policy не обнуляется dependent T/S-cache reset.
Key reuse не автоматически запрещается этой моделью: если отсутствует accepted
compromise input, нет основания объявлять every reused key compromised. Если
compromise известно, тот же материал с новым keyID/label не лечит его; нужны
реально иные approved authority или scoped deny/recovery. Получение новой root
chain само не доказывает уничтожение старых private copies. Реальные keys и
их состояние не читались, виновность signers не утверждается.

## 4. R28-Z07-B — end-to-end reset, fresh-pass replay и opposing recipe

### 4.1. Однозначные предпосылки trace

Root R7: timestamp `2-of-{a,b,c}`, snapshot authority S. T5 version5 указывает
snapshot100, подписан **b,c**, ещё fresh. Исходный accepted prefix может быть
**timestamp-only**: cache snapshot отсутствует после ранее неуспешного tail.
Verified R8: timestamp `2-of-{b,c,d}`, snapshot authority S′, final root fresh.
New intended T6 version6 указывает snapshot1 под S′.
Format/strict JSON/budgets/signatures/expiry/source fences предполагаются valid;
T5 signatures достаточны под обоими root. Ни одного real signed input не создаём.

### 4.2. Current adapter guard действительно существует

1. `PrepareArtifacts/client.go:118–150` создаёт isolated workspace из previous
   cache, full `updater.New`/Refresh. `loadRoot` в pinned Go:487–521 проверяет
   R8 old+new quorum/N+1 и делает persistence до timestamp загрузки.
2. В bare updater old T5 загружается через loadTimestamp:307–328; Q под R8=true.
   Intended new T6 с snapshot reference1 отвергается относительно 100 на
   UpdateTimestamp:141–145. Это cache pre-reset source trace.
3. Current `retainedRoot` видит T/S material change. `retainedState:30–35` строит
   next(R8, empty T/S), затем exact-bound pointer T5 сравнивается с old bytes.
   Guard `reset && bytes.Equal(stamp,previous.Cache[T])` на 43–44 возвращает
   **rotation continuation**, не artifact proof. Это существенный существующий
   guard; bare updater observation не blanket product defect.
4. `prepareVerifiedUpdate/update_delivery.go:87–103` может вызвать
   `retainDeliveryProgress` до возврата error. Source/custody/anchor/time/fences
   и fixed journal могут прервать запись. Предполагая **подтверждённый** commit
   next(R8,empty T/S), новый consumer начинает новый bounded pass; actual disk
   commit/kernel durability этим исследованием **не установлены**.

### 4.3. Продолжение внутри TASK: deletion ещё не исключает replay

5. Fresh pass стартует именно с committed `(R8,empty T/S)`. Missing next root
   завершает available chain, root fresh. Adversarial delivery возвращает
   **exact old T5** (подписи b,c, ещё valid). `Timestamp=nil` → нет старого
   reference100 floor. Q=true → T5 final check проходит; full updater сохраняет
   timestamp **до** snapshot шага.
6. Snapshot100 может быть unavailable или иметь только старую S подпись; tail
   отказ не отменяет уже принятого T5. Current wrapper previous timestamp пуст,
   root chain не менялась → reset=false; equality-continuation guard не срабатывает.
   Timestamp T5 становится допустимым **post-reset predecessor**, а не hidden
   resurrection pre-reset bytes.
7. Предполагая подтверждённую platform запись этого predecessor, next pass
   T6→snapshot1 получает 1<100 и rejected. Восстановление не гарантировано одной
   очисткой cache; создана другая допустимая история h_post.

Это не новый target/EXE acceptance. При tail failure proofs не публикуются;
source finalPolicy/completeRetainedState и consumer error gates остаются.
Это conditional liveness trace с fresh old signed metadata; signature forgery,
network exploitation и persisted lockout **не доказаны**.

### 4.4. Opposing recipe: Python paired replacement

`test_new_snapshot_fast_forward_recovery` test source:491–522 меняет S **и T**,
а `RepositorySimulator.rotate_keys:166–173` очищает role keyIDs и signers и
создаёт threshold новых distinct signers. Fetch:283–285 подписывает только
текущими signers. Это конкретный **old-only-signatures** recipe: old T5 Q=false
under new T′, поэтому local old timestamp не загружается; new T с ref1
проходит. `_load_timestamp:427–447` ловит rejected old cache, downloads new T,
пишет его только после successful `update_timestamp`.

**Чтение tests не runtime PASS.** Их helper исключает overlap/extra future
signatures/material reuse по конструктивным предпосылкам; simple «две role
lists изменились» этот эффект не воспроизводит. Full current Python source
также не содержит отдельного универсального reset stage; paired recipe
успешен по signature rejection, а не доказанной реализации every rotation.

### 4.5. Exact byte equality — не semantic old-record prohibition

Current guard сравнивает exact serialized bytes, но здесь есть важный
**противоположный guard**: если initial pass уже загрузил previous T5, incoming
same-version T5′ возвращает ErrEqualVersionNumber **до assignment** на
UpdateTimestamp:136–139. Поэтому простой resig/reformat равной версии не
меняет SDK pointer. boundMetadata либо выберет original previous T5 и
continuation сработает, либо отвергнет неоднозначные originals. Утверждение
«пересериализуем равную version и обойдём первый continuation» **не доказано**
и этим source опровергнуто при successful old-cache load.

После completed reset fresh-pass previous timestamp пуст — это другой context.
Если тот же signed payload приходит с иным допустимым набором signatures/JSON
formatting, original bytes могут измениться при сохранённом version/reference100.
SDK по-прежнему обязан проверить подписи, shape и final expiry; boundMetadata
выбирает только соответствующий SDK parsed object, без ambiguous originals.
При выполненных предпосылках это **новый valid envelope** со старой semantic
position; old-byte comparison с отсутствующим previous cache ничего не запрещает.
Даже exact прежние bytes уже могут пройти эту ветку по §4.3, форматирование
не необходимо и не является механизмом обхода.

Следовательно, нельзя использовать bytes.Equal как общий anti-replay запрет
на старую semantic position. Однако нельзя и выкинуть все equality/version
guards: они отделяют correct existing prefix от offending progress. Product
equivocation/immutable-signed-content rule и workflow acceptance требуют
раздельного scope. Это model boundary, не утверждение deployed bypass.

## 5. R28-Z07-C — continuity witness и crash только вместе с admission

### 5.1. Минимум относится к различимым историям

Требуемые знания consumer:

1. Verified **enrolled lineage** и exact path роли, а не inbound self-signed root.
2. Какая adopted ResetAllowed policy разрешила изменение каких T/S cache
   contexts; threshold/material/ID distinctions и scope application history.
3. Root-prefix progress, final-root time gate, reset **pending/completed** и
   accepted-cache provenance **после** reset. Это semantic witness, не обязательное
   новое DTO-поле/БД. Оно может быть implicit в допустимых atomic images.
4. Source identity/CAS плюс результат commit/barrier before дальнейшего приема;
   temporary workspace write и platform durable cut — разные носители.

Обоснование necessity: один serialized `(R8,T5,empty snapshot)` совместим с
h_pre «root сохранён, old T5 carried до completed reset» и h_post «reset completed,
тот же T5 уже accepted снова». На T6→ref1 h_pre надо сначала выполнить pending
reset, h_post должен соблюдать заново принятый 100 floor. Одинаковый representation
и разный обязательный ответ означают: сохранить различие, исключить одну историю
protocol'ом либо честно не обещать автоматическую recovery-ветку.
Не требуется новая signature/floor-server protocol.

### 5.2. Existing atomic image может исключить h_pre

Если storage **реально гарантирует** единственный доступный commit
`(R7,T5,S100) → (R8,empty T/S)` и readers/admission блокированы до его завершения,
а T5 может появиться только через subsequent verified acceptance, ambiguous
h_pre-image исключена. После этого `(R8,T5,empty snapshot)` означает h_post.
Новое basis field не нужно **для этой ограниченной ветки**.

Атомарный неправильный image `(R8,carried pre-reset T5)` проблему не решает.
Переписывание root/cache при artifact rollback не должно отменять previously
confirmed predecessor cut. Existing `filetxn` — reuse candidate for coherent
image, не математический proof hardware antirollback или OS power-loss safety.

### 5.3. S→S′→S и continuation через expired root

- Для whole call `R7(S)→R8(S′)→R9(S)` current retainedRoot выполняет OR **по всем
  intermediate links**, так что endpoint equality не теряет history-change.
  Но final S принимает old S signatures; reset≠signature exclusion.
- Для раздельных вызовов `R7(S)→R8(expired,S′)`, затем later `R9(fresh,S′)`:
  root persistence предшествует final-root expiry, reset5.3.11 после него.
  Если saved cut содержит R8 **и carried old T/S**, caller должен сохранить
  pending rotation basis across passes. Сравнение только R8/R9 равно S′/S′
  и пропускает предыдущую смену относительно cache.
- **Существующий противовес current adapter:** first retainedState может
  committed root+empty-cache образ даже при tail error, тем самым исключив
  carried-cache ambiguity. Это ограничительная policy до normative reset
  completion, а не автоматический proof всей 5.3.11. Она сохраняет root lineage,
  не выдаёт new artifact; OS receipt/semantics требуют own acceptance.

Поэтому «нужен новый pending field» не unconditional theorem. Если existing
atomic images запрещают ambiguous state, достаточно этого invariant. Если
обещано сохранять pre-reset cache across expired root prefix, требуется basis/
transition obligation, обеспеченное existing journal или явным state contract.

### 5.4. Crash — только с subsequent admission

| Crash/cut | Последующее поведение, необходимое для contract | Что нельзя заключить |
|---|---|---|
| Verify R8 выполнен, запись ещё не подтверждена; процесс остановлен | При restart исходный committed cut/recovery uncertainty; следующий live admission до reconcile не начинается | Сам crash не proof violation TUF; volatile checks могли не быть unit acceptance |
| Root successor записан, reset ещё pending | Не использовать carried old role cache как post-reset predecessor; завершить authorized reset либо continuation/blocked | Root version alone не receipt reset completion |
| Confirmed atomic R8+empty T/S | Следующий new pass может принять valid old T5, если Q=true; это h_post | Очистка cache не anti-replay privilege |
| T5 post-reset accepted/persisted, snapshot отказал | T5 floor остаётся; tail error не разрешает ещё раз очистить history без новой authorized transition | Nil artifact proof не означает отсутствие разрешённого predecessor |
| Uncertain journal move/write retirement | Existing barrier/fault и source-CAS запрещают second owner/admission до reconcile | Не assume успешный/неуспешный native write по одному error |

Current `filetxn.Store.Commit` изменён относительно 27 октября: journal-domain/
native writer/target pinning, BeforeCommit after staged records, checkTargets,
commit point на удалении recovery barrier, retirement/fault semantics.
`retainDeliveryProgress` использует **WithPreparedTransaction**, не прежний
same-Manager-only snapshot callback. Были прочитаны эти exact current portions;
полный kernel/FS/concurrent writer audit здесь **не выполнен**.

На 09:37:40Z обнаружен собственный input drift filetxn.go. Targeted diff09:38
показал **только вводный комментарий**: explicit coherent readers WithRead и
отсутствие locking у bare loaders. Commit/Recover body не изменён относительно
нашего capture. Новый raw capture/version hash сохранён отдельно, прежний не
перезаписан. Это не проверка implementation WithRead: лишь reinforces условие
«atomic image требует reader/admission protocol», которое не следует из writer
lock автоматически. All-writer/all-reader guarantee остаётся UNKNOWN.

`PrepareArtifacts` proofs появляются только after full lookup/hash/complete
retained state; при tail/continuation error return содержит prefix, не proof.
`prepareVerifiedUpdate` возвращает deliveryErr после possible retention;
не дошедшая до proof ветка не разрешает install. Это source guard, не actual
OS/VPN acceptance. Current trust boundary не объявляется сломанным по crash.

## 6. Scoped reuse, licence и цена

| Ready component | Решение | Причина / цена / предел |
|---|---|---|
| Existing Go2.4.2 `Root.VerifyDelegate`, `TrustedMetadata.UpdateRoot/UpdateTimestamp`, full Updater | **REUSE** crypto и explicit workflow | Old/new root quorum и exact metadata checks уже готовы; не собственный verifier. Apache-2.0, Go≥1.25.0 (manifest проекта 1.26); stage receipt/custody/platform checks отдельны |
| Current `retainedRoot/roleKeys` + retainedState continuation | **ADAPT contract**, без patch | Path-aware material diff полезен; threshold-only semantics и distinction A/Q/P explicit. OR path не equivalent old-record rejection; byte equality только narrow guard |
| Existing four-target journal/WithPreparedTransaction | **CONDITIONAL REUSE** | Coherent root+cache image/CAS/retirement уже отдельный seam; для bounded fresh-root reset может исключить h_pre без fifth file/DB. Kernel/native durability и malformed-state coverage NOT_RUN |
| Python7.0.1 verification result API + paired test recipe | **REFERENCE** | `Root.get_verification_result/verify_delegate` и `VerificationResult` показывают конкретные verified contributions. Apache-2.0 OR MIT, Python≥3.10; не добавлять Python runtime/second crypto stack ради reference |
| Legacy0.7.0 revoked-count reset | **REFERENCE / NOT DROP-IN** | BSD-3-Clause; old threshold and draft-specific cache table не contemporary norm/exact-envelope test. Blind port может слишком широко/узко invalidate |

Минимальный consumer result должен отвечать: какой authenticated transition
разрешил invalidate, какое accepted context действует для cache, completed ли
barrier до new admission, и какие previously accepted/newly observed records
ещё Q-valid. Это semantic obligations; representation может быть нынешним
atomic cut/journal или отдельным field **только если выбранные branches иначе
неразличимы**. Не предлагается дополнительная БД или центральный floor server.

Стоимость: bounded root-path/original-byte/provenance review, quarantine при
uncertainty, более узкий supported recovery recipe, quorum/signature validation
при chosen migration. Фактические CPU/RAM/IO/delay/billing не измерены и не
поставлены 0. Для packet preparation применимы существующие bounds. Root catch-up
limit не доказательство latest. Old valid metadata и независимые часы не
обеспечивают offline knowledge последнего revoke — прежний предел R05-010-B
не переоткрыт и не отменён.

## 7. Opposing review и recommended scoped verdict

- **Против любого key-set diff как универсального reset права:** spec phrase
  не исчерпывает addition/threshold semantics; current adopter predicate не
  нормативное определение. Recommendation: contract уточнить, не auto-code fix.
- **Против «paired keys исключают h_post»:** overlap b,c и dual signatures
  сохраняют Q. Recipe нужен на **конкретные retained envelopes/старые signers**.
- **Против «S-only blocked значит SDK всегда неправильный»:** T rotation с
  old-only signatures предотвращает cache load; timestamp expired предотвращает
  full acceptance; current wrapper имеет continuation. Это scoped liveness case.
- **Против «atomicity достаточно»:** неправильный root+carried-cache image не
  различает истории; atomic image с empty context действительно может исключить
  h_pre и не требует нового field.
- **Против «после reset можно забыть все floor»:** application history сохраняется,
  а post-reset legitimate T5 заново создал applicable metadata floor.
- **Против «root endpoint equality достаточно»:** whole-path S→S′→S; current OR
  already handles single-call variant, cross-call basis зависит от image contract.

Границы предлагаемых выводов:

1. **R28-Z07-A: рассмотреть ACCEPTED только source/model distinction A/Q и
   конкретные матричные examples.** Spec detail ResetAllowed overlap/threshold
   оставить explicit unresolved adopter policy, не blanket normative claim.
2. **R28-Z07-B: рассмотреть ACCEPTED conditional h_post trace и opposing Python
   recipe.** Actual disk replay/lockout/server exposure не принимать без execution.
3. **R28-Z07-C: рассмотреть ACCEPTED necessary semantic witness относительно
   обещанных recovery branches.** Достаточность конкретного OS journal/API и
   обязательность нового field **не установлены**.

Авторский статус каждого — **READY_FOR_REVIEW**, не scientific ACCEPTED.
Применение к Windows/Android trust (TH-03, FR-008/011, Android trust admission)
остаётся предложением в названной области, не продуктовой приёмкой.

## 8. Work stages, remaining unknown и finite future cases

| Work stage TASK | Выполненное |
|---|---|
| 1: source/pin/caller | TUF1.0.37; Go2.4.2; Python7.0.1 + recipe; legacy opposing reference; selected current-source captures |
| 2: semantic matrix | Disjoint/S-only/overlap/add/remove/threshold↑↓/aliases/dual signatures/material reuse/S→S′→S; A/Q/P/D различены |
| 3: updater/recovery trace | Current continuation и fresh-pass h_post; Python helper opposing trace; predicate scopes проверены |
| 4: minimal witness/reuse | Atomic image versus carried-cache ambiguity; existing transaction path; chosen recipe and cost/licence |
| Продолжение: deletion/replay | Post-reset Q-valid old timestamp и retaken floor100 исследован, не повтор first finding |
| Продолжение: crash+admission | Pending/completed barrier и prefix/proof/native receipt различены; не crash-alone attack |

Future cases **NOT_RUN** (текст, не исполняемые fixtures):

- F1: disjoint T/S full replacement с old-only signatures versus dual-signed
  timestamp при одинаковых key sets.
- F2: overlap2-of-{a,b,c}→2-of-{b,c,d}: a,b signature set versus b,c.
- F3: add/remove keys с surviving quorum; verify explicit chosen reset policy.
- F4: threshold-only increase/decrease с достаточными/недостаточными signatures,
  no implicit deletion privilege.
- F5: computed keyID/scheme/material reuse и duplicate contributions без
  выдуманной alias independence.
- F6: whole-call S→S′→S versus cross-call expired-prefix continuation.
- F7: completed reset→replay fresh T5→snapshot tail failure→T6 ref1 rejection;
  distinguish source prediction from real disk receipt.
- F8: resig/reserialized same-payload envelope versus exact byte-equality guard;
  preserve complete signed constraints and ambiguity refusal.
- F9: crash before barrier/after root/after reset/after T acceptance и фактический
  последующий admission; no progress-only artifact permit.
- F10: atomic current image excludes h_pre; carried-cache image needs additional
  witness or restricts supported continuation.

Связанный C05 HANDOFF/REPORT §4 рассмотрен в §3.4: atomic-image opposing case
и distinction compromised quorum. Согласие выводов в этой области — **не**
независимая приёмка A/B/C и не замена public source. Итоговые source hashes,
drift и ссылки сохранены в captures; это static documentation review,
не tests/SDK/runtime.
