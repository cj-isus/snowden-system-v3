# R28-Z03 — HANDOFF: регистрация до раскрытия и полнота cohort

28 октября 2026.

- status: READY_FOR_REVIEW
- source_availability: AVAILABLE

## Главный итог

Ранний sequence ID, current-publication guard, bbolt ACID и self-hash не
гарантируют complete pre-outcome cohort. Достаточный условный протокол:
controlled dispatch после подтверждённой durable регистрации/debit,
уникальная operation identity, сохранённые pending/cancelled/unpublishable
membership и recovery continuity. Результат sampling и ресурсная retirement
живут на разных осях. Статистическую A1-validity и полноту до регистрации
нужно доказать отдельно; журнал их не создаёт.

## R28-Z03-A — actual caller и предел existing projection

**Вывод source/model:** `appProtectedProbe.Run → publishObservationLocked →
chanHealth → scoredCandidates` — operational health/ranking seam, не
complete durable attempt cohort.

**Evidence:** `probe_observation.go:84–128` выдаёт ObservationID до factory/Run,
но atomic счётчик в памяти; `observer_publication.go:41–121` — subject/
clock/epoch/order gates и filtered writes; `corebridge.go:617–647,723–779`
cancel/retry выбрасывают часть return-путей из health; `chanhealth.go:225–229`
epoch reset забывает aggregate; `scoring.go:201–228` читает epoch projection.
Async human logs (`app.go:107–139,171–191,968–991`) не ACK перед измерением.

**Сильнейший opposing case/guard:** ProbeReport реально содержит bounded returned
DoH attempts; core.ProtectedProbe ждёт wg.Wait перед release. Unsafe current
report не теряется только из-за newer ID. Поэтому blanket «все результаты
теряются» или «cancel сразу освобождает весь owner» отвергнут.

**Рекомендуемый scoped verdict:** принять только отсутствие доказанного
complete-cohort свойства **этого** усиленного consumer contract. Не product
bug/утечка и не основание убрать safety gates. Current C2 score не статистический
p-value; typed observations и deterministic baseline сохраняются.

## R28-Z03-B — custody/disclosure/recovery contract

**Новая дельта:** после failed registration ACK возможен persisted debit;
после dispatched marker и crash execution может быть unknown. Это разные
cuts, которые одна строка «cancelled/not_started» не разрешает.
Инвариант в REPORT §4.1: для каждого controlled start существует retained r
с `commit(r) ≺ ack(r) ≺ authorize(start) ≺ first_relevant_disclosure`.
Это causal order, не один сравнимый timestamp. Полнота admitted cohort не
доказывает полноту actually-started membership: confirmed c>0 units со
суммой loss∈[L,U] и u возможно стартовавших units дают finite started-only
hull `[L/(c+u),(U+u)/(c+u)]` при заявленной membership/support модели.
c=0 допускает empty cohort/undefined mean. Это идентификация, не CS;
admitted/ITT cohort имеет иное заранее заданное определение исхода.

**Условный protocol:** immutable Family/Cohort/Operation/Attempt IDs +
reference данных, доступных до admission → atomic record+reservation →
acknowledged storage → platform safety/current gate → persist dispatch
marker → actual owner start → returned/mature/retired witnesses отдельно.
Cancel/stale не удаляют unit. Duplicate delivery — тот же ID; измерительный
retry — новая child attempt либо часть заранее определённой anytime strategy.
Unconfirmed writer запрещает слепой повтор action/новый бюджет, reconciliation
проверяет первоначальный ID и owner. Mandatory safety action не ждёт
scientific storage: optional inference тогда incomplete/abstain.

**Сильнейший контрпример:** две uniform true-null p доступны, обе честно
зарегистрированы до reveal; выбирается min и оплачивается только выбранная
проверка δ. FWER=1−(1−δ)^2>δ. Полнота+chronology есть, conditional A1/A2
не следуют. Напротив, выбрать поднабор уже simultaneously covered семьи
можно без новой FWER цены — это accepted baseline theorem, не повторная
модель этого автора.

**Reset/recovery:** process/health epoch ≠ новая lifetime statistical семья.
Rollback log/head/accountant до старого prefix одинаков в двух мирах (после
prefix дополнительных attempts не было / были и пропали). Self-consistent
hash/signature не различат; использовать independent retained continuity
witness, корректно narrower scope либо abstain. Центральный сервер не
объявлен единственным допустимым witness; новый signing protocol не придуман.

**Рекомендуемый scoped verdict:** conditional protocol + information limit,
не установленные guarantees current producer. Accepted alpha rule применима
при настоящем conditional validity и conserved family expenditure, не по
наличию ACK/hash. Unknown/missing/censored stays in defined denominator.

## R28-Z03-C — ready reuse/licence/cost

**SagerNet/bbolt** `v0.0.0-20231014093535-ea5cb2fe9f0a` в current
windows/go.mod:102 и sing-box/go.mod:29. MIT Ben Johnson2013 прочитан;
шесть critical source/LICENSE файлов получены по pinned raw ref и совпали
с local cache. API: DB.Update/Commit, Put/NextSequence/View/Batch.

**Evidence/counterexample:** Put overwrites, Delete/SetSequence возможны;
Batch вызывает fn неоднократно; Tx.writeMeta сначала WriteAt, затем Sync,
ошибка Sync не доказывает отсутствия write. LockFileEx sentinel и one-writer
mutex сериализуют cooperating access, не producer chronology/omitted events.
NoSync/open timeout/reader lifetime не hard RAM/latency или anti-rollback proof.
Actual sing-box cachefile удаляет неизвестные buckets/reset'ит DB — **REJECT**
его wrapper/file как complete ledger. Raw transactional API — conditional
REUSE/ADAPT, не новая storage platform по умолчанию.

**Минимальный reuse seam:** register/debit до optional dispatch, owned
unconfirmed writer slot, bounded membership/outcomes snapshot, stable IDs и
restored family accountant. Existing `filetxn.BoundedWriteSlot` проверен как
original-owner-retirement pattern, не как turnkey bbolt adapter.

**Цена:** sync/serial admission wait, concurrency/retention/ACL/recovery proof,
copy View values, separate projections, privacy. Нет measured CPU/RAM/energy,
WCET или hard storage cap. Windows/Android owners реализуют только в отдельном
разрешённом этапе; здесь никакой code/dependency changes.

## Источники и область доказательства

[REPORT.md](REPORT.md): аргумент, exact trace, event states, invariants,
crash/cancel/retry/duplicate cuts, оба продолжения TASK, opposing guards.
[SOURCES.md](SOURCES.md): scoped current inputs, pin/API/LICENSE,
**действительно прочитанные русские** MS Learn FlushFileBuffers/LockFileEx,
RFC9162 inclusion/consistency против source completeness,
RFC9110 idempotency против physical repeated sends.

Вместо новой CS использована scoped приёмка
[NINE-R03-010-A](../../../reviews/NINE-2026-10-27-R03-010-A.md). Именно A1/A2,
не весь old R03-010, bbolt implementation или actual producer assumptions.
Private captures: `.artifacts/research/r28-contracts/Z03/` (36 initial +7
additional inputs; remote equality6; scoped final drift отдельно).

## Пройденные stages / остаток

TASK1–4: пройдены. Продолжения «failure before measurement» и
«reset/recovery lineage/error budget» пройдены. Opposing/reuse/licence/cost и
handoff выполнены. Paired C03 HANDOFF отсутствовал при review-stage09:13:49Z:
не ждал его, independent paired review остаётся pending.

12 future cases C01–C12 в REPORT описаны только словами — **NOT_RUN**.
Непроверенные device/filesystem durability, actual resource return/physical
send count, conditional measurement model, storage caps и product adapter
остаются unknown. Source+manual model не product/native/live proof.

Tests, fixtures, simulations, crypto и native/device/live проверки **NOT_RUN**.
