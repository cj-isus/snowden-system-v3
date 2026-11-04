# R28-Z09 — HANDOFF

## Claim

**R28-Z09-A — corrected fixed-cohort maturity contract.**

Protected task completion rate не искажается pending/early outcomes, если:

1. единица — pre-outcome registered task-recovery episode; retries/stages не
   создают новые units;
2. enrollment roster/N фиксируется независимо от outcomes;
3. horizon `D(class)` задан до outcome;
4. analytic maturity определяется только `t0+D≤cut_time` в сравнимом clock scope;
5. ранний success сохраняется, но не получает меньший denominator; ранний
   not-complete/partial остаётся provisional;
6. timing/follow-up unknown остаются в N;
7. task/protection/system axes разделены; P называется endpoint-protected, а не
   whole-operation protected.

Author status: **READY_FOR_REVIEW**.

## Scoped recommendation

- **A / cohort+maturity:** `ACCEPT_CORRECTED_FOR_INDEPENDENT_REVIEW` для fixed
  registered cohort и endpoint-protected system ITT при явных producer assumptions.
- **B/privacy/OSS/pilot:** `DEFER/ADAPT`; не принимать по аналогии. Current journal
  не complete registry/task producer; OONI reference-only; user pilot NOT_RUN.
- Это предложение для отдельного review, не научный ACCEPTED и не product/native/live PASS.

## Formulas

Fixed roster at cut:

`N=P+U+X+Q+F+C+M+R`.

- System endpoint-protected ITT:
  `I_sys=[P/N,(P+X+M+R)/N]`.
- Task resolved any route:
  `I_task=[(P+U+X)/N,(P+U+X+C+M+R)/N]`.

At final comparable cut `R=0`. These are deterministic identification hulls,
not confidence intervals or selection correction. Unknown registry completeness
for intended factual attempts can force `[0,1]`.

## Strongest opposing cases checked

1. **Early denominator:** 1 early P + 1 pending → mature-only reports 1; fixed N
   correctly gives `[1/2,1]` until second matures.
2. **Early not-complete:** response at +10, success at +40<D → early F would be
   false; only D or absorbing contract may assign F/Q terminally.
3. **Off→on→complete:** точечный protection sample рядом с ручной отметкой не
   покрывает фактический endpoint interval; без producer coverage → X. Известный
   unprotected route → U. Whole-operation protection тем более не следует.
4. **Clock reset:** incomparable elapsed scopes → M_timing, not exclusion/success.
5. **User stop:** known non-success for system ITT, unknown latent any-route task.
6. **Retries/late success:** one episode/denominator; late success does not rewrite
   timely outcome.

## Source/caller and reuse

- Current project bytes available and hashed; repository commit anchor
  `413621d…`, but files untracked, so hashes identify reviewed source.
- `SessionJournal`: process-local ring128, typed facts, GPL-3.0-or-later;
  adapt vocabulary only, reject as registry/task producer.
- `ControllerReport/Store/MainActivity`: frozen exact displayed text + manual
  `ACTION_SEND`; reuse/adapt manual aggregate export, not durable storage/proof.
- OONI Probe CLI v3.31.0 commit `1605e004…`, GPL-3.0: separate save/submit gates;
  `upload` CLI is unimplemented. Reference/adapt only, no code copy.
- Jiang–Ding v6 §2.1: worst-case binary missing bounds without MAR; does not
  provide registry completeness/protection bridge.
- Paired C03/C06 HANDOFFs became available only at final review. Both support
  fixed roster/outcome-independent maturity; C06's interval-coverage objection
  tightened P so a point sample without producer coverage becomes X. Agreement
  is corroboration, not primary evidence; paired files were not modified.

## Privacy/cost

Minimal record: closed enums, random local IDs, clock/horizon class, provisional
observations, task-at-D, protection-at-completion, system/user-stop and resource
axis. No domains/raw URLs/IP/SNI/SSID/package/account/workplace inventory,
content, credentials, stable device ID or raw errors. Local counts/bounds default;
manual immutable export only. Durable registry adds storage/migration/tombstone/
clock/privacy cost beyond current ring.

## Platform consequence

Windows/Android owners would need an explicit registration owner, fixed roster,
durable IDs through cut, clock-scope/bridge, task receipt producer and protection
bridge. Existing lifecycle/journal fields alone do not satisfy those contracts.
No platform status changes from this research.

## Unknown / future gates (NOT_RUN)

Numerical D/R, durable registry owner, lawful deletion/tombstones, strict
whole-operation protection bridge, task-level caller, all-user selection,
sampling/comparison inference, user burden/effect, legal compliance, synthetic
schema and device/user pilot.

## Work stages completed

Source/pin/caller; main argument; six opposing traces; reuse/licence/cost;
continuation interval bounds/triviality; scoped handoff. Paired C03/C06 HANDOFFs
were not available at review stage; no waiting/cross-write.
