<a id="trust-continuity-agreement-2026-10-28"></a>
# Непрерывность доверия: различимость истории и точный смысл согласия

[Оглавление](../README.md) · [TUF progress/reset](SAFETY-AND-TRUST.md#tuf-progress-and-reset-2026-10-27) · [Решения review](../research/reviews/2026-10-28-DEEP-REVIEW.md)

**2026-10-28, DR27-10/11.** Основания: R05-002/006/007, TUF1.0.37,
TAP4 и public go-tuf2.4.2 `f5edbde31e5507f46db2069402dc38903fe6d9d4`.
Это conditional model/source conclusions. Текущие platform stores и SDK
в данном docs-срезе не доступны, поэтому дефект приложения не установлен.

## 1. Минимальное состояние определяется обещанными положительными ветками

Если `E(h)` — сохраняемое представление истории h, а `F(h,x)` — обязательный
ответ на следующий вход x, то

`E(h1)=E(h2)` и `F(h1,x)≠F(h2,x)`

означают невозможность восстановителю выдать оба правильных ответа только
по E,x. Выходы: сохранить различие, исключить одну историю протоколом или
отказаться от соответствующей automatic recovery ветки. Это необходимое
информационное условие, не доказательство минимального числа bytes/полей.
Вечный BLOCKED удовлетворил бы одной safety-цели, но не полезному recovery.

**Проверенный пример h_pre/h_post:** fresh R8 сменил snapshot keys S→S′,
сохранив timestamp signer T; exact t5 указывает snapshot100 и ещё валиден.

| История | Как появился `(R8,t5,нет snapshot)` | Следующий t6→snapshot1 |
|---|---|---|
| h_pre | t5 принят под R7; R8 сохранён; crash до завершения разрешённого reset | Завершить pending reset, затем t6 может пройти остальные gates |
| h_post | Reset уже завершён; тот же t5 заново принят под R8; snapshot не получен | Floor100 уже принят заново; t6→1 отвергается без нового разрешённого перехода |

Timestamp сохраняется до получения snapshot, а его стандартный формат не
сообщает локальный момент принятия относительно reset. Его корректная подпись
T под R8 не различает эти две истории. История всей root chain тоже сама
не сообщает, был ли затем принят t5. Полный existing journal/CAS receipt,
напротив, может уже содержать нужное различие.
[TUF §§4.6/5.3–5.5](https://theupdateframework.github.io/specification/v1.0.37/#detailed-client-workflow).

## 2. Из нижней границы не следует обязательное новое поле или новая БД

Atomic protocol `(R7,t5)→(R8,∅timestamp,∅snapshot)` может исключить h_pre:
после него появившийся t5 относится к последующему принятию. Согласованное
image/journal может заменить отдельный `basis` field. Но атомарная запись
**неправильного** образа `(R8,старый active t5)` проблему не решает.

Для intermediate expired roots сложнее: root prefix сохраняется до final-root
freshness check, а reset выполняется позже. В цепочке
`R7→R8(expired,S→S′)→новый вызов→R9(fresh,S′)` сравнение лишь R8/R9 теряет
изменение относительно старого cache. Поддерживаемая continuation-ветка должна
сохранять причинное основание — в данных или эквивалентном протоколе.

**Уточнение прежнего durability-аргумента:** crash сразу после verify, до
незавершённой записи, ещё не опровергает порядок TUF. Значимый counterexample —
verifier **продолжил следующий workflow step**, не выполнив требуемый durable
barrier принятого prefix и не оставив эквивалентного authorized replay obligation.
Pure whole-call prepare может иметь более узкую единицу acceptance и не
объявлять внутренние проверки завершёнными durable steps.
Простой сохранённый inbound bundle не является разрешением его replay.

В pinned go-tuf `loadRoot` вызывает `UpdateRoot`, затем `persistMetadata`
для каждого нового root; temp/write/close/rename/readback не удостоверяют
power-loss durability. `Fetcher` выдаёт входные bytes, а не acceptance receipt.
Lower-level `UpdateRoot` — существующий step seam, но обёртка должна сохранить
полный workflow. [Source](https://github.com/theupdateframework/go-tuf/blob/f5edbde31e5507f46db2069402dc38903fe6d9d4/metadata/updater/updater.go).

## 3. Paired keys — условие на старые подписи, а не на два изменившихся списка

Python reference recovery заменяет T и S; helper очищает старые signers и
создаёт новые. Тогда старый t5 не проходит новый quorum и конкретная h_post
с его повторным принятием исключена.
[Test](https://github.com/theupdateframework/python-tuf/blob/49eb1da48c58cf0cbb0f2e490f7ad39fbd8c1f15/tests/test_updater_top_level_update.py),
[helper](https://github.com/theupdateframework/python-tuf/blob/49eb1da48c58cf0cbb0f2e490f7ad39fbd8c1f15/tests/repository_simulator.py).

Однако переход `2-of-{a,b,c}→2-of-{b,c,d}` даёт разные результаты:
старый record с a,b не проходит, с b,c проходит. Даже непересекающиеся key IDs
не гарантируют исключения exact старого envelope, если в нём заранее имеются
достаточные подписи новых keys. Нужно доказывать отсутствие достаточного нового
quorum для каждого восстанавливаемого старого record. Expiry cache сама не
отменяет ранее принятый rollback floor.

Фраза TUF “keys have been rotated” не задаёт исчерпывающий ResetAllowed для
overlap, threshold-only, S→S′→S или reuse key material. Эти варианты остаются
открытыми контрактами; нельзя объявлять нормативным любой key-set diff.
Восстановление также зависит от availability нужных подписанных inputs и IO.

Отдельная точная boundary: spec требует `expires > fixedStart`, тогда как
четыре pinned `IsExpired` используют `referenceTime.After(expires)` и на
равенстве возвращают false. Это source-level различие строгой границы;
дополнительные app gates могли бы его ужесточать и здесь не проверены.
[IsExpired](https://github.com/theupdateframework/go-tuf/blob/f5edbde31e5507f46db2069402dc38903fe6d9d4/metadata/metadata.go).

## 4. Согласие по bytes не назначает новую authority

[TAP4 v1](https://github.com/theupdateframework/taps/blob/6096e30ca04eb4a79e1d23a2aa9ac5ad64a6d63b/tap4.md)
задаёт mapping-local threshold для length/hash и **исключает custom** из
обязательного agreement. Map предполагается доверенной adopter-настройкой.
Корректный hash N отвечает «какие bytes», но не «кто разрешил использовать N
как новую policy». Approvals `activate/P` и `preview/Q` нельзя складывать
по одинаковому digest. Для joint P AND N нужны отдельные выполненные predicates.

Противовес: example `BootstrapTUF` уже доставляет map и child roots как
проверяемые TUF targets. Если действующая governance назначила этот authority,
механизм может быть подходящим. Но authority, способная менять весь map/anchors,
остаётся общей зависимостью. Разные repository names не доказывают независимых
custodians. [Actual example](https://github.com/theupdateframework/go-tuf/blob/f5edbde31e5507f46db2069402dc38903fe6d9d4/examples/multirepo/client/client_example.go).

## 5. Какие guards действительно есть у multirepo

`GetTargetInfo` создаёт новые groups для каждой mapping, исключает повтор
repoName, обходит все repositories строки и проверяет конфликт всех
threshold-групп перед возвратом. **Смешивание counts разных mappings и ранний
возврат первой группы не найдены.** Это важные опровержения чрезмерных claims.

Но после успеха одной mapping поиск не продолжает следующие. Mappings —
priority/fallback, не AND всех совпадающих строк. В pinned example строгая
`*.pub` threshold2 non-terminating строка имеет последующий `*` threshold1.
При успешных Refresh, но несовпадающих descriptors, fallback может дать
односторонний результат. Ветвь «repo offline→fallback» через этот main не
эквивалентна: общий Refresh вернёт ошибку до поиска.
[Implementation](https://github.com/theupdateframework/go-tuf/blob/f5edbde31e5507f46db2069402dc38903fe6d9d4/metadata/multirepo/multirepo.go),
[fixture map](https://github.com/theupdateframework/go-tuf/blob/f5edbde31e5507f46db2069402dc38903fe6d9d4/examples/multirepo/repository/targets/562fc7cffe872542a430342995998546e3949dc3acbe7d37668dc76d657032ff.map.json).

## 6. Hash compatibility не является отношением эквивалентности

Pinned `TargetFiles.Equal` проверяет length и `Hashes.Equal`. Последнее требует
совпадения всех **общих** algorithms и наличия хотя бы одного общего; одинаковый
полный набор hashes не требуется. Для одного path/length:

`A={sha256:a}`, `B={sha256:a,sha512:b}`, `C={sha512:b}`.

A совместим с B, B с C, A не совместим с C. При representative grouping
порядок B,A,C может дать одну группу3, A,B,C — две группы с недобором,
A,C,B — две пересекающиеся threshold2-группы и conflict error.
[Equality и verifier](https://github.com/theupdateframework/go-tuf/blob/f5edbde31e5507f46db2069402dc38903fe6d9d4/metadata/metadata.go).

**Более сильный условный пример:** A подписывает `sha256=H256(F)`, B — тот же
sha256 плюс `sha512=b′`, где b′ не соответствует F. При A-first metadata
совместимы, threshold2 достигнут, а download проверяет descriptor A и может
вернуть F. Все constraints B не выполнены. При B-first тот же F отвергается.
`VerifyLengthHashes` действительно проверяет все hashes **переданного**
descriptor; multirepo не передаёт ему объединение constraints approvers.

Это опровергает обещание «returned bytes выполнили все signed constraints
каждого counted approver». Это **не** доказательство arbitrary payload вопреки
честному byte approval: A действительно одобрил F. Совместимость TAP4 для
разнородных hash-наборов и конкретный application exploit не объявлены доказанными.

## 7. Решение и конечные cases

Reuse existing stores/crypto/mapping guards. Сначала уточнить требуемую
семантику agreement: общий выбранный алгоритм, полное equality либо проверка
всех требуемых descriptors — это разные политики с ценой совместимости.
Freshness, физическая независимость, availability и применение manifest
проверяются отдельно от byte agreement. Новая БД или multirepo deployment
не выбраны. Адресаты TH-03/08 и существующий trust owner.

Future cases **NOT_RUN**: обе h_pre/h_post плюс допустимый successor; atomic
cuts и промежуточный expired root; partial quorums/dual-signed cache/reuse;
P committed при отказе A; точное expiry равенство; раздельные mapping counts;
два threshold candidates; три порядка A/B/C; extra hash mismatch у counted
approver; разные purpose/custom; strict→weak mapping fallback; alias rename
при сохранении repository history. Source/math review не заменяет SDK/native
durability, signing или реализацию этих tests.
