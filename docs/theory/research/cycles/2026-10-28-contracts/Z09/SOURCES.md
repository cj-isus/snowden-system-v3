# R28-Z09 — SOURCES

Read/capture date: **2026-10-28**. Public source reading and source-only project
inspection; no product execution, real logs, telemetry or user data.

## 1. Assignment and baseline

| ID | Source | Read depth | Used for / boundary |
|---|---|---|---|
| B1 | `cycles/2026-10-28-contracts/COMMON.md` | FULL_TEXT_READ | Window, rights, work stages, evidence rules; organisational only |
| B2 | `cycles/2026-10-28-contracts/Z09/TASK.md` | FULL_TEXT_READ | R28-Z09 scope and acceptance |
| B3 | `research/reviews/NINE-2026-10-27-R06-013-A.md` | FULL_TEXT_READ | Exact NEEDS_REVISION and counterexamples; review did not reverify external sources |
| B4 | `research/06-RELIABILITY-UX/REPORT-013.md`, §§2–5 | SECTIONS_READ | Original unit/horizon/outcomes/privacy/OONI proposal; not accepted axiom |
| B5 | `adaptation/INTERVALS-AND-POLICY-EVALUATION.md`, §§1–7 | FULL_TEXT_READ | Fixed cohort, missing interval, completeness, historical target/bridge, ITT policy vocabulary |
| B6 | C03/C06 paired HANDOFF | NOT_AVAILABLE at checks 08:57,09:14,09:27 UTC | Did not wait/poll; no independent paired claim imported |

## 2. Primary/method sources

### S1 — Jiang and Ding, partial identification with missing outcomes

- Zhichao Jiang, Peng Ding.
- *Using Missing Types to Improve Partial Identification with Application to a
  Study of HIV Prevalence in Malawi*.
- arXiv `1610.01198v6`; published 2016-10-04; v6 updated 2018-09-11.
- URL: <https://arxiv.org/abs/1610.01198> / official PDF
  <https://arxiv.org/pdf/1610.01198v6>.
- Read depth: **SECTIONS_READ**, introduction + §2.1 (PDF pp.2–4 / extracted
  lines54–176). Official arXiv API metadata and PDF captured.
- Relevant fact: without MAR assumption, binary missing outcome contributes an
  unidentified term bounded by its extreme 0/1 values; resulting worst-case
  bounds do not assume a missing mechanism but may be too wide. The paper does
  not establish our registry completeness, maturity clock or protection bridge.
- Capture hashes:
  - metadata XML `3e1d4348570fecefd87f14ac709f5e619d336a0c5675d2d4e3e45af43fc19637`;
  - PDF `67e5a45c170d30b1317f14ac709f5e619d336a0c5675d2d4e3e45af43fc19637`;
  - extracted text `23c8deb2cab5214a39e83d3ba0c507e3e68f6a932f000046f9892ec2ea8dc209`.
- First HTML WebFetch attempt: **FETCH_FAILED (timeout)**. Method changed once to
  official API/PDF; failing fetch not repeated.

### S2 — ICH E9(R1), estimand/intercurrent/missing vocabulary

- International Council for Harmonisation (ICH).
- *E9(R1) Addendum on Estimands and Sensitivity Analysis in Clinical Trials*.
- Step 4, 20 Nov 2019; official file `E9-R1_Step4_Guideline_2019_1203.pdf`.
- URL: <https://database.ich.org/sites/default/files/E9-R1_Step4_Guideline_2019_1203.pdf>.
- Read depth: prior FULL_TEXT_READ capture; this cycle **SECTIONS_RE-READ**:
  pp.2–3 lines134–150; A.3.1 pp.5–6 lines235–302; A.3.3 p.9 lines427–460.
- Used only as method vocabulary: define target/population/variable/handling of
  intercurrent events/summary; distinguish event changing interpretation from
  missing/censoring. Clinical treatment/regulatory claims are not transferred.
- PDF SHA-256 `f7471f411f1c87ee76783d5b2b9faaeca31d01d530213f71b50d136b39e3b0d9`;
  extracted text `47f2980f8160dfc3c914c33193e2c8d7369c2433fe13425dae45e07bff3ac6bd`.

## 3. Current Snowden source/caller

Repository HEAD anchor at review: `413621d567d80b815721fe417c300f3423a8ad95`.
The reviewed files were untracked in this working tree; **their exact SHA-256,
not the commit, identifies reviewed bytes**. Android app manifest:
`versionName=0.1.0-dev`, compile/targetSdk36, minSdk29.

All source headers: SPDX `GPL-3.0-or-later`.

| File / symbols | Exact source fact | Reuse boundary |
|---|---|---|
| `android/vpn/.../SessionJournal.kt:10–112` | Process-local ring128; command/state/validated worker observations; elapsed+wall; evicted/recordingLimited; no disk/background/raw exception/addresses/config/credentials/worker identity | ADAPT typed bounded facts/clock vocabulary. REJECT as complete fixed-cohort registry/task producer/VPN proof |
| `ControllerReport.kt:17–90` | Typed allowlist, frozen capture fields, event/count/unknown age, explicit not-VPN-proof text, 32768 ASCII cap | REUSE aggregate/export shape; not task endpoint |
| `ControllerReportStore.kt:13–95` | One memory-only frame, exact displayed revision for share, 10min access expiry, clear drops references only | ADAPT immutable manual export; not durable cohort storage or wipe guarantee |
| `ControllerReportScreen.kt:51–138` | User opens frozen history, chooses fields/window/rows, previews and presses share | Existing explicit local UI boundary; no consent/task episode caller |
| `MainActivity.kt:110–121` | Foreground/focus guard; `textForShare`; `ACTION_SEND`; dispatch explicitly not delivery/VPN proof | REUSE exact-byte manual share boundary |
| `android/app/build.gradle.kts:7–25` | App `0.1.0-dev`, compile/target36, min29, GPL header | Version context only; no build performed |

Hashes: `.artifacts/research/r28-contracts/Z09/project-source-sha256.txt`.

## 4. OSS reference — OONI Probe CLI

- Project: `ooni/probe-cli`.
- Tag: `v3.31.0`; commit
  `1605e00481e2b98488d62cfc2f7ce34ff345eb70`.
- License: root **GPL-3.0**, FULL_TEXT_RE-READ header/current capture.
- Repository: <https://github.com/ooni/probe-cli>.
- Read depth: **SOURCE_RELEVANT_PATHS_READ**, exact tag captures:
  - `internal/oonirun/experiment.go:192–216`: local saver and submitter gates;
  - `internal/oonirun/submitter.go:21–64`: disabled → stub, enabled → real submitter;
  - `cmd/ooniprobe/internal/nettests/nettests.go:155–167`: submitter only when
    `Sharing.UploadResults`;
  - `cmd/ooniprobe/internal/cli/upload/upload.go:9–16`: upload command not implemented.
- Decision: REFERENCE/ADAPT separation of local save/network submit only. No code
  copy (GPL and mismatched broad measurement model); no ready manual export,
  cohort maturity or task/protection bridge.
- Hashes: `.artifacts/research/r28-contracts/Z09/ooni-v3.31.0-sha256.txt`.

## 5. Own model/provenance boundaries

- Formulas/traces T1–T6 are manual deterministic arguments, not simulations.
- `I_sys`/`I_task` are identification hulls conditional on fixed complete
  registered roster, correct category assignment and bridge assumptions; not CI,
  FWER/FDR, causal effect or all-user estimate.
- Current source proves APIs/fields/callers in named bytes only, not runtime,
  device, native path, user consent or product acceptance.
- Plan-only boundaries observed: no test/build/fixture/simulation/benchmark,
  native/device/live/network endpoint, secrets/logs/profiles or product changes.
