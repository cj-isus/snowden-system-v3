# R28-C03 — SOURCES

Дата чтения: 28 октября 2026. Read depths ниже относятся только к реально прочитанной области; наличие capture не означает FULL_TEXT_READ. Execution отсутствует.

## 1. Baseline и научный статус

| ID | Источник / read depth | Что используется |
|---|---|---|
| B1 | [INTERVALS-AND-POLICY-EVALUATION](../../../../adaptation/INTERVALS-AND-POLICY-EVALUATION.md),28октября2026; **SECTIONS_READ §§1–6** | finite/historical/target различия, Γ/TV/prefix/shared-envelope бюджеты, complete-policy estimand/support. Формулы не объявляются новым результатом C03 |
| B2 | [REPORT-010](../../../03-ADAPTATION/REPORT-010.md),27октября; **SECTIONS_READ §§3.2,3.4–3.5,5,6** | A1/A2, disclosure versus admission, minimal producer obligations, selected bbolt/confseq path |
| B3 | [Ограниченный review010-A](../../../reviews/NINE-2026-10-27-R03-010-A.md); **FULL_TEXT_READ** | ACCEPTED только перечисленная математическая область; не real producer/код/библиотеки/field |
| B4 | [Erratum R03-17](../../../reviews/NINE-2026-10-27-R03-17-ERRATUM.md); **FULL_TEXT_READ** | Не использовать independence product для arbitrary dependence и старое универсальное infinite-credit утверждение |

## 2. Actual project seam / source-only

В `.artifacts/research/r28-contracts/C03/local-inputs.json`23repo inputs:
before/copy/after SHA256 и capture UTC28октября. Это addressed non-atomic cuts,
не commit/artifact/runtime identity. Старые versions26/27октября не выданы за current.

| ID | Файлы / depth | Реально прослеженная область |
|---|---|---|
| L1 | windows/netwatch.go **SECTIONS_READ** run/fire/onNetPathChanged/netWatchProbeStart/netChangeReprobe/tryPreferredChannelOnNetworkChange; transportpref.go pickDefaultChannel/preferredChannelForNetworkChange | Debounce/TUN skip→async event→busy/current filtering→protected probe→verify/publication→class hint→existing switch executor |
| L2 | windows/probe_observation.go **SECTIONS_READ** Run/currentProbeSubject; core/probe.go Run/ProbeReport; core/probe_http_observation.go types/fetchProbeHTTPRequest | Pre-work observation ID, original start/finish, bounded HTTP/result branches, no useful-app or finite-cohort semantics inferred |
| L3 | windows/backend/core/diagnostic_budget.go **FULL_TEXT_READ** | Nonrefunded request/body reservation before I/O, separate protected/optional lanes; no α/roster/outcome/custody proof |
| L4 | windows/observer_publication.go **SECTIONS_READ** through current/owner/clock/health commit; core/manager.go LastProbeReport/rememberProbe | Latest publication order/unsafe precedence/current guards, single retained historical report, not complete cohort |
| L5 | windows/failover.go **SECTIONS_READ** switchChannelCoreFor/finishChannelSwitchError and policy counter; shared/adaptive/budget_types.go receipt/settlement | Expected/current owner gate and Manager switch/Reload/rollback publication; reset/return/retirement not probability |
| L6 | shared/adaptive/evidence_types.go/evidence.go/evidence_validation.go/evidence_time.go **SECTIONS_READ**, preference.go **FULL_TEXT_READ** | Exact identity/clock/coverage, state/censor/custody, latest decisive status, no family/filtration/bridge/accountant fields |
| L7 | Windows go.mod **TARGETED_LINES** | Go1.26; sing-boxv1.13.19; SagerNet/bbolt pseudo-version pinned |

Bounded search `ComposeEvidence(` / `PlanAutomaticBackground(` / `adaptive.Prefer`
и shared module path в Windows/shared non-test Go sources обнаружил определения
SDK и existing Windows preference caller, но не production Windows caller SDK.
Это не аудит всех платформ/бинарников. Ни source not-found, ни runtime result не
выводятся из отсутствия match.

## 3. Готовые implementations и primary semantics

### C1. SagerNet/bbolt

- Exact manifest version: `v0.0.0-20231014093535-ea5cb2fe9f0a`.
- Fresh primary metadata: [official Go proxy .info](https://proxy.golang.org/github.com/sagernet/bbolt/@v/v0.0.0-20231014093535-ea5cb2fe9f0a.info),
  capture28октября; full commit **ea5cb2fe9f0afaec4827c0591954db0e11ba548c**,
  source version time2023-10-14T09:35:35Z.
- Selected source: [db.go](https://github.com/SagerNet/bbolt/blob/ea5cb2fe9f0afaec4827c0591954db0e11ba548c/db.go),
  [bucket.go](https://github.com/SagerNet/bbolt/blob/ea5cb2fe9f0afaec4827c0591954db0e11ba548c/bucket.go),
  tx.go/doc.go from exact local module cache.
  **SECTIONS_READ:** Update/View/Batch/batch.run retry, NextSequence/SetSequence, Commit/OnCommit,
  byte lifetime; **FULL_TEXT_READ LICENSE MIT,2013BenJohnson**.
- New bounded provenance: fresh db.go/bucket.go/LICENSE each equal local captured
  SHA256 (`remote-cache-comparison.json`). Это не byte audit всего upstream/бинарника;
  tx.go/doc.go остались exact-version local source, не fresh remote equality.
- Actual upstream caller: sing-box **v1.13.19**, cachefile/cache.go Start/view/batch/
  update/resetDB **SECTIONS_READ**; module cache capture. Cache wrapper top-level
  cleanup/reset semantics непригодны как complete statistical ledger. Sing-box
  LICENSE **HEADER_READ GPL-3.0-or-later**, не MIT low-level bbolt.
- Execution/install/DB open/benchmark/durability test: **NOT_RUN**.

### C2. confseq

- [Exact commit5ffe733ca2447a2e28c2c91f3b00086173f2ab2c](https://github.com/gostevehoward/confseq/tree/5ffe733ca2447a2e28c2c91f3b00086173f2ab2c),
  setup.py version **0.0.11**, fresh predmix.py/LICENSE/setup.py captures.
- **SECTIONS_READ:** predmix_hoeffding_cs→lower_cs→generic weighted accumulation,
  deterministic/default versus fixed_n λ and α/2 split; running_intersection
  maximum after raw lower/clipping; CI-wrapper fixed_n=len(x), CI-seq wrapper.
  No numerical/C++/ABI audit. REPORT6.3 K1/κ, constant-kernel expression и K2 —
  own algebra/manual counterexamples under declared conditions, not an upstream
  claim that API handles missing/corrupted labels. K2 uses no computed run.
- Predmix SHA256 `70456e9a3531582a0d0e146ba9ed6c8d643667025f5f4b85864d41dbd9036fdb`.
- LICENSE **FULL_TEXT_READ MIT,2021confseq developers**; setup **SECTIONS_READ**
  skbuild/package requirements. Runtime dependency/import not introduced.
- [Waudby-Smith/Ramdas arXiv2010.09686v7](https://arxiv.org/html/2010.09686v7),
 25августа2022, **SECTIONS_READ §2/§3.1/Proposition1**: constant conditional mean,
 predictable bets/bounded input. Journal2024 issue — другой publication date.
- Reuse disposition: **OFFLINE_REFERENCE**, не missingness/registration/bridge
  producer и не ready Windows/Android implementation.

### P3. Clock semantics

[Go time@go1.26.0, Monotonic Clocks](https://pkg.go.dev/time@go1.26.0#hdr-Monotonic_Clocks),
**SECTIONS_READ**: UTC strips monotonic reading. Initially unversioned docs opened
go1.27.3; для вывода выбрана explicit go1.26.0. Manifest Go1.26 не подтверждает
версию реально запущенного executable. Current library duration/clock semantics —
reference к source call, не доказательство native runtime clock continuity.

## 4. Ручные выводы и недоступный paired input

K1/κ, K2, D1 и O11 получены ручной алгеброй/конечными конструкциями REPORT,
не заимствованными network measurements или новыми внешними теоремами. Их
предположения и отсутствие execution указаны рядом. Baseline proofs не
переатрибутируются C03; independent scientific review ещё нужен.

Z03/Z09 HANDOFF отсутствовали на раннем check08:56:41Z и финальном review-stage
check09:23:06–07Z. `paired-inputs.json` фиксирует вторую адресную проверку.
**NOT_AVAILABLE_AT_CHECK**, не SOURCE_READ/поддержка C03. Чужие draft reports
не подставлены вместо готового handoff, ожидания/опроса в цикле не было.

## 5. Артефакты и граница проверки

Local23 captures, module-cache8, fresh external selected files/metadata и hashes
хранятся только в разрешённом C03 artifacts namespace. Тайминг — UTC в manifests.
Read-depth source register ограничивает attribution; захваченные большие файлы
полностью прочитанными не объявляются. Primary URLs не исполнялись; README/build/
example instructions не выполнялись.

`final-input-drift.json`,09:29:35Z: current hashes всех23repo inputs и8module
inputs совпали с captured hashes; сами capture bytes тоже неизменны. В эту
область входят read-only TASK/COMMON и baseline. Это сравнение выбранных файлов,
не полная atomic tree identity и не проверка runtime. Own final doc hashes
записываются отдельно в `final-deliverables.json` после упаковки четырёх файлов.
