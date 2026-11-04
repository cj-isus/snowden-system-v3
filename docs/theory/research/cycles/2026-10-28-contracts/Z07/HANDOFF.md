<a id="r28-z07--конечный-handoff-исследователя-7"></a>
# R28-Z07 — краткие выводы и ограничения

Срез исходников: 28 октября 2026.

**Авторский статус: READY_FOR_REVIEW.** Не scientific ACCEPTED и не product PASS.
Source availability: **SOURCE_AVAILABLE**.

## 1. Главный ответ

Нужно различать **три независимых вопроса**:

1. **Разрешено ли invalidate прежний T/S cache?** Это ResetAllowed из verified
   root lineage и выбранной transition semantics. TUF1.0.37§5.3.11 говорит
   “keys have been rotated”, но не задаёт полный boolean predicate для
   overlap/add/remove/threshold-only/reuse. Рост root version и изменение одного
   threshold сами по себе не доказанное право забыть историю.
2. **Проходит ли конкретный старый envelope новый quorum?** Решает actual
   verified signature set, не число изменённых keys. При
   `2-of-{a,b,c}→2-of-{b,c,d}` old signatures a,b недостаточны, b,c достаточны.
   Disjoint key sets также не исключают dual-signed old+future-key envelope.
3. **Исключён ли достаточный compromised new quorum?** Это ещё один threat
   predicate. Даже Q(oldEnvelope)=false не запрещает attacker выпустить новое
   сообщение, если он по-прежнему контролирует достаточные b,c private keys.

Удаление cache само не отзывает ключи и не гарантирует recovery-liveness.

<a id="2-claims-и-предлагаемый-scoped-verdict"></a>
## 2. Выводы и границы доказательств

| Claim | Новая дельта и evidence | Граница предлагаемого вывода |
|---|---|---|
| **R28-Z07-A** | Матрица всех TASK-переходов; separate ResetAllowed/Q/P/D; exact current material-diff OR versus legacy revoked-count. Для old-only signatures ограниченный criterion exclusion: `|K_old ∩ K_new| < q_new`; новые/dual signatures отменяют эту предпосылку | Рассмотреть принятие только source/model distinction и concrete counterexamples. Universal ResetAllowed для partial/threshold/reuse остаётся explicit unresolved policy |
| **R28-Z07-B** | Full current updater→continuation→confirmed hypothetical empty-cache cut→fresh old T5 reacceptance→snapshot failure→новый floor100→T6 ref1 rejection. Opposing Python paired-key recipe подтверждён по actual helper, не просто по названию теста | Рассмотреть conditional h_post/liveness trace. Реальная запись, эксплуатационный lockout, frequency/severity, arbitrary artifact bypass не доказаны |
| **R28-Z07-C** | Минимальное semantic obligation: accepted context относительно reset/barrier; ambiguous `(R8,T5)` h_pre/h_post; atomic root+empty-cache image может исключить h_pre без field/БД. S→S′→S path memory и expired-prefix cross-call варианты | Рассмотреть необходимую различимость относительно promised recovery branches. Не принимать обязательность нового поля и достаточность конкретного OS journal без separate native/all-writer/readers evidence |

Все рекомендации — предложения. Приёмка прежних R05-001/R05-010 claims не
распространяется на A/B/C нового пакета автоматически.

## 3. Проверенные сильные противоположные случаи / existing guards

- **Overlap:** old b,c envelope переживает key change; old a,b нет. Нельзя
  доказать replay-exclusion одним root/key-set diff.
- **Threshold-only:** ↑q может исключить old-cache подписи в SDK, но current
  wrapper `reset=false` сохраняет `timestampContinuity(previous,new)` и
  отвергает понижение timestamp/snapshot reference. Это existing guard, не
  automatic floor forgetting и не факт runtime.
- **Paired full replacement:** Python helper очищает и role keyIDs, и signers,
  создаёт fresh threshold set; старый envelope с **только** old signatures
  отвергается. Recipe не покрывает dual signatures/overlap/material reuse.
- **S→S′→S:** current retainedRoot OR по intermediate roots уже видит изменение
  внутри одного call. Endpoint-only контрпример **не** опровергает этот guard.
  Across calls возможный pending basis зависит от допустимых atomic images.
- **First continuation:** current reset+exact previous-byte equality выдаёт
  continuation, не artifact proof. Fresh pass после completed reset — другой
  acceptance context, в котором ещё-valid old T5 может быть принят снова.
- **Same-version resig/reformat:** если old T5 уже в SDK, incoming same-version
  возвращает equal-version **до assignment**; old pointer сохраняется, поэтому
  simple reserialization не доказанный обход первого continuation. Report§4.5
  содержит это опровержение, не sensational byte-bypass claim.
- **Crash:** незавершённая запись без subsequent admission не нарушение TUF.
  Actual commit/barrier/uncertain owner необходимо проверить перед новым
  acceptance; existing source guards не объявляются отсутствующими.

## 4. Exact source/reuse и цена

- **TUF1.0.37**, Last modified2026-10-08, relevant§§4.2.1/4.3/5.1–5.6/6.1.
- **go-tuf2.4.2**, commit
  `f5edbde31e5507f46db2069402dc38903fe6d9d4`, current Windows manifest.
  `UpdateRoot/UpdateTimestamp/VerifyDelegate`, full `loadRoot/loadTimestamp`
  source; Apache-2.0, upstream Go≥1.25.0. Reuse crypto, adapt scoped result/
  reset-context contract, не новая реализация signature verifier.
- **python-tuf7.0.1**, commit
  `49eb1da48c58cf0cbb0f2e490f7ad39fbd8c1f15`, fresh tag receipt;
  `test_new_snapshot_fast_forward_recovery` + `rotate_keys` + fetch signer path.
  Apache-2.0 OR MIT, Python≥3.10; reference, не новый runtime dependency.
- **Legacy0.7.0 exact commit6ad7fe…**: BSD-3-Clause, old/draft reset predicate.
  Не drop-in correction contemporary SDK.
- Existing `retainedRoot/retainedState`, `WithPreparedTransaction`, fixed
  journal/source-CAS — conditional reuse. Реальная OS durability/reader cohesion
  и неизменность всех writers не доказаны; field/БД не назначаются теорией.

Цена описана семантически: original-byte/path/provenance retention, bounded
continuation, source review при смене SDK, possible availability loss и custody/
barrier acceptance. CPU/RAM/latency/runtime/token/billing не измерены и не нули.
Никаких patches/imports/install/dependency upgrade в этой работе.

<a id="5-work-stages-и-продолжение-внутри-task"></a>
## 5. Область анализа

| Область | Результат |
|---|---|
| Source/pin/caller | Current project availability, exact Go/Python reference pins/spec, public-source captures |
| Основной аргумент/matrix | Disjoint/S-only/overlap/add/remove/threshold↑↓/aliases/dual signatures/reuse/S→S′→S |
| Trace/recovery recipe | Current continuation guard + separate h_post replay, Python opposing recipe |
| Минимальный witness | Context/phase/barrier вместо unconditional new field; explicit atomic-image alternative |
| Следующий существенный вопрос | Completed deletion не excludes replay; crash исследован с admission/commit, не сам по себе |
| Opposing/reuse/cost | Existing threshold/equal-version/root-path/transaction guards, source/API/licence и границы |
| Связанный разбор | C05 HANDOFF и REPORT §4 прочитаны: atomic-image alternative и compromised-quorum distinction. Согласие авторских выводов не primary proof и не scientific acceptance |

## 6. Документы, evidence и остаток

- [REPORT](REPORT.md) — матрица/вывод, complete trace, counterarguments,
  current-source guards, reuse/cost,10finite future cases.
- [SOURCES](SOURCES.md) — exact URLs/sections/read depth/publication pins,
  source availability и drift, licensing, excluded evidence.
- Captures: `.artifacts/research/r28-contracts/Z07/`.
  `inputs-start/extra` — sequential hashes; `drift-filetxn.json` — возникший
  comment-only drift сохранён отдельно. `review.json` — итог собственных docs/
  selected-source bytes; не SDK/native test.

Осталось **UNKNOWN/NOT_RUN**: применённый ResetAllowed для every partial
rotation, actual signed corpus и compromised keys, real OS durability/power-loss,
all-readers/writers invariant, успешный persisted replay/lockout, whole SDK
conformance, действующий platform admission. Все F1–F10 NOT_RUN.

