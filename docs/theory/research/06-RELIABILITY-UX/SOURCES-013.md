# R06-013 — источники и provenance

Дата чтения: **2026-10-27**. Все public HTTPS-чтения —
литературный/source review, не VPN live test. Реальные пользовательские данные,
логи, профили, vault и telemetry не читались.

Artifacts: `.artifacts/research/theory-06/2026-10-27-recovery-pilot-res9/`.

## 1. Локальный baseline

| ID | Источник / точное место | Статус | Использование и граница |
|---|---|---|---|
| L2 | `research/reviews/NINE-2026-10-27-R03-17-ERRATUM.md`, §§1–5 | FULL_TEXT_READ | Запрет ошибочной product-form FWER и ложного infinite-expectation claim; регистрация не исправляет selection/censoring |
| L3 | `06-RELIABILITY-UX/REPORT.md:1240–1416`, R06-004 | FULL_TEXT_READ (selected complete sections) | Bounded journal/export и K(y); evidence authority не достигает task outcome |
| L4 | `06-RELIABILITY-UX/REPORT.md:242–338`, R06-01 | FULL_TEXT_READ (selected complete section) | Cohort maturity, P/F/U missing-outcome bounds, privacy limitation; уточнено новой outcome taxonomy |
| L5 | `06-RELIABILITY-UX/REPORT.md:2027–2082`, R06-011; `handoffs/R06-011.md` | FULL_TEXT_READ | Baseline task_attempt; source ошибочной union-границы и недоказанного platform task producer; не принят как аксиома |
| L6 | `observability/DNS-AND-DIAGNOSTICS.md:190–216`, OP-05 | FULL_TEXT_READ (selected complete section) | Typed producer allowlist, immutable preview/share, no auto upload, limits deletion claims |
| L7 | `methodology/CLAIM-FORMAT.md`; `research/README.md` | FULL_TEXT_READ | Паспорт, уровни evidence, plan-only границы |

## 2. Методические первоисточники

### S1 — ICH E9(R1), estimand и missingness

- Организация: International Council for Harmonisation (ICH).
- Название: **ICH E9(R1) Addendum on Estimands and Sensitivity Analysis in
  Clinical Trials to the Guideline on Statistical Principles for Clinical
  Trials**.
- Версия/дата: Step 4, 20 November 2019; файл опубликован 3 December 2019
  (имя официального файла `E9-R1_Step4_Guideline_2019_1203.pdf`).
- URL: <https://database.ich.org/sites/default/files/E9-R1_Step4_Guideline_2019_1203.pdf>
- Статус: **FULL_TEXT_READ** локально из официального PDF; PDF SHA-256
  `f7471f411f1c87ee76783d5b2b9faaeca31d01d530213f71b50d136b39e3b0d9`;
  extracted text SHA-256
  `47f2980f8160dfc3c914c33193e2c8d7369c2433fe13425dae45e07bff3ac6bd`.
- Конкретные места:
  - pp.2–3 / extracted lines134–150: discontinuation/switch = intercurrent
    event; withdrawal/administrative censoring = missing-data problem;
  - A.3.1, pp.5–6 / lines235–302: definition and explicit distinction from
    missing data;
  - A.3.3, p.9 / lines427–456: condition, population, variable, intercurrent-
    event handling, population-level summary;
  - A.5.1, pp.15–16 / lines689–728: align estimator to estimand, state
    assumptions and sensitivity analysis.
- Применимость: **method vocabulary/reference only**. Клинические treatment
  effects, randomisation и regulatory claims не переносятся на VPN. Пакет
  адаптирует лишь требование заранее определить target/outcomes и отделить
  событие, меняющее смысл исхода, от ненаблюдённого исхода.

### S2 — Google SRE Workbook, user-centered SLI

- Организация: Google.
- Название: **Implementing SLOs**, Site Reliability Engineering Workbook.
- Дата: online workbook chapter; страница не показывает отдельную дату главы.
- URL: <https://sre.google/workbook/implementing-slos/>
- Статус: **FULL_TEXT_READ** через публичную HTML-страницу, 2026-10-27.
- Разделы/факты: “What do you want to measure?” / “Choose appropriate SLIs” —
  сначала определяется service outcome, значимый пользователю, независимо от
  способа измерения; ratio = good events / total observed events in declared
  population/window; health checks and component metrics are proxies and may
  miss user-impacting outcomes.
- Применимость: поддерживает уровень task outcome и разделение specification /
  observing implementation. Не доказывает, что текущий Snowden journal способен
  наблюдать task completion и не задаёт horizon/selection model нашего pilot.

### S3 — NIST/SEMATECH e-Handbook, censoring vocabulary

- Организация: NIST.
- Название: **8.1.3.1. Censoring**, NIST/SEMATECH e-Handbook of Statistical Methods.
- URL: <https://www.itl.nist.gov/div898/handbook/apr/section1/apr131.htm>
- Дата страницы: не указана.
- Статус: **FULL_PAGE_READ**, 2026-10-27; это учебный handbook, **не первичная
  методологическая публикация**.
- Использование: только базовый vocabulary right/administrative censoring;
  формулы/гарантии из страницы не переносились. Основное distinction основано на S1.

### S4 — OWASP Logging Cheat Sheet, minimisation/export

- Организация: OWASP Foundation.
- Название: **Logging Cheat Sheet**.
- URL: <https://cheatsheetseries.owasp.org/cheatsheets/Logging_Cheat_Sheet.html>
- Дата публикации страницы: не указана; прочитано 2026-10-27.
- Статус: **FULL_TEXT_READ**.
- Конкретные разделы: “Purpose”, “Which events to log”, “Data to exclude”,
  “Where to record event data”, “Protection”, “Disposal of logs”.
- Факты: intended purpose guides what/when/how much; session IDs/tokens/secrets/
  sensitive PII should not be directly recorded; mask/remove/pseudonymise where
  appropriate; imports/exports and access require controls; retention applies to
  extracts/copies.
- Применимость: security guidance, не доказательство compliance. Пакет выбирает
  более строгий closed typed schema до записи; OWASP не валидирует конкретную
  реализацию Snowden.

### S5 — 152-ФЗ, попытка русскоязычного первоисточника

- Документ: Федеральный закон РФ №152-ФЗ от 27.07.2006 «О персональных данных».
- Запрошенная страница: Consultant.ru, статья 5:
  <https://www.consultant.ru/document/cons_doc_LAW_61801/7424e5d3a243c597d0ac09b26c19e84afbcaaaf6/>
- Статус: **FETCH_INCOMPLETE**. Fetch вернул оглавление/метаданные, но не текст
  пунктов статьи; дата точной действующей редакции страницы не была показана
  (перечень изменений включал акт 26.07.2026 №265-ФЗ).
- Решение: нормы статьи не цитируются и compliance claim не делается. Этот пакет
  не является юридическим заключением. Русскоязычный primary не подменён
  англоязычным пересказом.

## 3. Exact OSS source — OONI Probe CLI

### Pin/provenance

- Проект: `ooni/probe-cli`, tag `v3.31.0`.
- Commit: `1605e00481e2b98488d62cfc2f7ce34ff345eb70`.
- Release timestamp:2026-10-22T13:05:42Z.
- [Репозиторий](https://github.com/ooni/probe-cli).
- Корневой `LICENSE` прочитан/сохранён целиком: GPL-3.0, SHA-256
  `3972dc9744f6499f0f9b2dbf76696f2ae7ad8af9b23dde66d6af86c9dfb36986`.
- Архив исходников: SHA-256
  `18f5c7b38926fed38015782bfb3bcedcd00b7f6aa8bf062639099e21bfe250ef`.
- SOURCE_FULL_RELEVANT_PATHS_READ выбранных путей exact-tag дерева.
  Проект не собирался/не запускался, сетевые эксперименты не исполнялись.

### Checked callers

| File/symbol | Exact source fact | Boundary |
|---|---|---|
| `internal/model/measurement.go:71–101` `Measurement` | Versioned local measurement structure, local ID/input/start time/test keys | Payload is far broader/more sensitive than proposed recovery record |
| same `:207–247`, `ScrubMeasurement` | Rewrites known current IP in serialized top-level/TestKeys; sets ProbeIP default | Specific replacement is not general privacy-by-schema; other identifiers may remain |
| `cmd/ooniprobe/internal/nettests/nettests.go:160–167` | Creates network submitter only when `Sharing.UploadResults` | Actual main CLI gate separating local save from upload |
| `internal/oonirun/experiment.go:192–216` | Saver `Enabled: !NoJSON`; submitter `Enabled: !NoCollector` | Separate local record and network submit controls |
| `internal/oonirun/submitter.go:21–64` | `Enabled=false` → no-op stub; true → real submitter | Reusable architectural pattern only |
| `internal/probeservices/submit.go:12–40` | JSON is POSTed to `/api/v1/submit_measurement` | Explicit network side effect; not used by proposed pilot |
| `cmd/ooniprobe/internal/cli/upload/upload.go:9–16` | `upload` command logs “not implemented” | Negative result: **not** a ready manual-export caller |

### Reuse/license decision

Решение — REFERENCE/ADAPT без переноса кода: брать различие
`local saver / submit gate / explicit network side effect`. Готовые OONI
inputs/test keys и сетевые эксперименты не становятся частью Snowden.
Его IP scrub не доказывает нашу приватность; список разрешённых полей producer в Snowden более узок.
Возможный последующий перенос кода требует отдельного обоснования
совместимости и полного состава лицензий, а не запрета или разрешения
только по названию GPL-3.0. OONI не доказывает измеренное восстановление задачи.

## 4. Own artifacts manifest

| Artifact | SHA-256 / note |
|---|---|
| `ICH_E9_R1_2019.pdf` | `f7471f411f1c87ee76783d5b2b9faaeca31d01d530213f71b50d136b39e3b0d9` |
| `ICH_E9_R1_2019.txt` | `47f2980f8160dfc3c914c33193e2c8d7369c2433fe13425dae45e07bff3ac6bd` |
| `ooni-v3.31.0.tar.gz` | `18f5c7b38926fed38015782bfb3bcedcd00b7f6aa8bf062639099e21bfe250ef` |
| captured `measurement.go` | `9208f1867cfd2e364dcca706fd8eee1e60e3165b7fbb804f942bcdfad759bcc0` |
| captured `submit.go` | `7bcb17c408e1dc885a20d79ce59815b67faf90016f948d2530686f0b6d0db951` |
| captured `oonirun/submitter.go` | `e403fdcf9bc78bbb51f5b2a8bacc5baa15f424fc82101e5f89d942fef4a342f8` |
| captured CLI `upload.go` | `8e6630dc62175871ad945f3f7ec796d43489022402a4d080e863762f9a1e39d0` |

Метаданные GitHub API, публичные документы и архив исходников получались
по HTTPS с проверкой сертификатов. Установка зависимостей, тесты, сборка,
симуляции, native/device/user измерения и телеметрия не выполнялись.
