# R28-Z09 — источники и пределы переноса

Дата исходного чтения и сохранения — **28 октября 2026 года**.
Рассматривались публичные источники и выбранные исходники проекта.
Исполнение продукта, реальные логи, телеметрия и пользовательские данные
не использовались.

## Краткий вывод

Статья о пропущенных исходах поддерживает общий приём границ 0/1,
ICH — словарь целевой величины и событий. Ни один источник не предоставляет
готовый producer задачи, полноту реестра или связь с защищённым VPN-путём.
Их перенос ограничен явными предпосылками, а не тематическим сходством.

## 1. Предыдущее основание

| ID | Материал | Глубина | Область и предел |
|---|---|---|---|
| B3 | `research/reviews/NINE-2026-10-27-R06-013-A.md` | FULL_TEXT_READ | Точное NEEDS_REVISION и контрпримеры. Та рецензия не перепроверяла внешние источники |
| B4 | `research/06-RELIABILITY-UX/REPORT-013.md`, §§2–5 | SECTIONS_READ | Исходные единица/горизонт/исходы/privacy/OONI, не принятая аксиома |
| B5 | `adaptation/INTERVALS-AND-POLICY-EVALUATION.md`, §§1–7 | FULL_TEXT_READ | Fixed cohort, пропуски, полнота, историческая цель/связь с будущим, ITT-словарь |
| B6 | C03/C06 HANDOFF | На ранних проверках 08:57/09:14/09:27 UTC недоступны | Финальное ограниченное сопоставление описано в REPORT §9. Ранняя недоступность не означает отсутствие позднего материала |

Организационные COMMON/TASK задавали границы назначения, а не доказательство
научного вывода; их прочтение не заменяет источники ниже.

## 2. Методические первоисточники

### S1 — Jiang и Ding: частичная идентификация при пропусках

- Zhichao Jiang, Peng Ding.
- *Using Missing Types to Improve Partial Identification with Application to a
  Study of HIV Prevalence in Malawi*.
- arXiv `1610.01198v6`: первоначальная публикация 2016-10-04,
  обновление v6 —2018-09-11.
- [Метаданные](https://arxiv.org/abs/1610.01198) ·
  [точная PDF-версия](https://arxiv.org/pdf/1610.01198v6).
- SECTIONS_READ: introduction и §2.1, PDF pp.2–4, извлечённые строки 54–176.
  В исходном срезе сохранены официальный arXiv API XML и PDF.

Используемый принцип: без MAR неизвестный бинарный исход допускает крайние 0/1,
что даёт worst-case диапазон, часто слишком широкий. Статья не устанавливает
наши полноту регистрации, зрелость часов или подтверждение защиты VPN.
Формулы REPORT — отдельная детерминированная модель фиксированной когорты,
а не клиническая или выборочная оценка из статьи.

SHA-256 сохранённых файлов:

- `papers/arxiv-1610.01198.xml`:
  `3e1d4348570fecefd87f20738e4ece71c61d250db9fc8a27758e8b3a087ee076`.
- `papers/arxiv-1610.01198v6.pdf`:
  `67e5a45c170d30b1317f14ac709f5e619d336a0c5675d2d4e3e45af43fc19637`.
- `papers/arxiv-1610.01198v6.txt`:
  `23c8deb2cab5214a39e83d3ba0c507e3e68f6a932f000046f9892ec2ea8dc209`.

Редакционная сверка 4 ноября выявила ошибку переписывания XML-хеша:
выше указан фактический хеш имеющегося файла. PDF и извлечённый текст совпали
с прежним реестром. Сверка файлов не является новым получением источника
или доказательством корректности его метода.

Первый запрос HTML в исходном срезе завершился timeout (FETCH_FAILED),
после чего использованы официальный API/PDF. Непрочитанный ответ не считается источником.

### S2 — ICH E9(R1): целевая величина и события

- International Council for Harmonisation (ICH),
  *E9(R1) Addendum on Estimands and Sensitivity Analysis in Clinical Trials*.
- Step4,20Nov2019; `E9-R1_Step4_Guideline_2019_1203.pdf`.
- [Официальный документ](https://database.ich.org/sites/default/files/E9-R1_Step4_Guideline_2019_1203.pdf).
- Прежнее FULL_TEXT_READ; в этом цикле SECTIONS_RE-READ:
  pp.2–3 строки 134–150; A.3.1 pp.5–6 строки 235–302; A.3.3 p.9 строки 427–460.
- Только методический словарь: target/population/variable, обращение с
  intercurrent events и summary; различие события, меняющего смысл результата,
  и missing/censoring. Клинические и регуляторные выводы не переносятся.
- PDF SHA-256:
  `f7471f411f1c87ee76783d5b2b9faaeca31d01d530213f71b50d136b39e3b0d9`;
  извлечённый текст:
  `47f2980f8160dfc3c914c33193e2c8d7369c2433fe13425dae45e07bff3ac6bd`.

## 3. Исходники Snowden на дату среза

HEAD anchor: `413621d567d80b815721fe417c300f3423a8ad95`.
Рассмотренные файлы были untracked: идентичность задают их SHA-256,
не этот commit. Контекст приложения: `versionName=0.1.0-dev`,
compile/targetSdk36, minSdk29. Прочитанные source headers —
SPDX `GPL-3.0-or-later`. Это не текущая аттестация артефакта.

| Файл / symbols | Подтверждённый факт исходника | Граница переиспользования |
|---|---|---|
| `android/vpn/.../SessionJournal.kt:10–112` | Process-local ring128; command/state/validated worker observations, elapsed+wall, evicted/recordingLimited; нет disk/background/raw exceptions/addresses/config/credentials/worker identity | Адаптировать ограниченные typed facts и часы. Не считать полным реестром, producer задачи или доказательством VPN |
| `ControllerReport.kt:17–90` | Typed allowlist, frozen capture fields, counts/unknown age, явное not-VPN-proof, ASCII cap32768 | Ориентир агрегирования/экспорта, не task endpoint |
| `ControllerReportStore.kt:13–95` | Одна memory-only frame, точная показанная share revision, срок доступа 10min; Clear удаляет ссылки | Ручной неизменяемый экспорт, не durable store и не физическое стирание |
| `ControllerReportScreen.kt:51–138` | Открытие frozen history, выбор fields/window/rows, preview и явное share | Существующая локальная UI-граница; согласие на task episodes из неё не следует |
| `MainActivity.kt:110–121` | Foreground/focus guard, `textForShare`, `ACTION_SEND`; dispatch не доказательство доставки/VPN | Граница ручной отправки точного текста |
| `android/app/build.gradle.kts:7–25` | `0.1.0-dev`, compile/target36, min29, GPL header | Контекст версии; без сборки |

Хеши: `.artifacts/research/r28-contracts/Z09/project-source-sha256.txt`.

## 4. OONI Probe CLI как готовый ориентир

- `ooni/probe-cli`, tag `v3.31.0`,
  `1605e00481e2b98488d62cfc2f7ce34ff345eb70`.
- Корневой LICENSE: GPL-3.0; FULL_TEXT_RE-READ заголовка и сохранённой копии.
- [Репозиторий](https://github.com/ooni/probe-cli).
- SOURCE_RELEVANT_PATHS_READ по точному tag:
  `internal/oonirun/experiment.go:192–216` — saver/submitter;
  `internal/oonirun/submitter.go:21–64` — stub либо real submitter;
  `cmd/ooniprobe/internal/nettests/nettests.go:155–167` —
  включение только при `Sharing.UploadResults`;
  `cmd/ooniprobe/internal/cli/upload/upload.go:9–16` — команда не реализована.

Брать только разделение local save/network submit. Код не копировался:
GPL и более широкая чувствительная измерительная модель требуют отдельного
рассмотрения. Готовые maturity, ручной экспорт и task/protection bridge
этим API не предоставляются.
Хеши: `.artifacts/research/r28-contracts/Z09/ooni-v3.31.0-sha256.txt`.

## 5. Границы собственной модели

- T1–T6 и формулы — ручные детерминированные аргументы, не симуляции.
- `I_sys`/`I_task` условны на фиксированный состав, корректные категории
  и предпосылки подтверждений; это не CI/FWER/FDR, причинный эффект или
  оценка всех пользователей.
- Названные source bytes подтверждают поля/API/callers, не runtime,
  устройство, native-путь, согласие пользователя или продуктовую приёмку.
- Тесты, сборка, fixtures, simulation/benchmark, native/device/live,
  endpoints, secrets/logs/profiles в этом срезе не выполнялись/не использовались.

<!-- Исторически ошибочный XML-хеш, исключён из действующей идентичности:
3e1d4348570fecefd87f14ac709f5e619d336a0c5675d2d4e3e45af43fc19637
Сохранён только для трассировки редакционной поправки; фактический хеш указан в S1. -->

<a id="r28-z09--sources"></a>
<a id="1-assignment-and-baseline"></a>
<a id="2-primarymethod-sources"></a>
<a id="s1--jiang-and-ding-partial-identification-with-missing-outcomes"></a>
<a id="s2--ich-e9r1-estimandintercurrentmissing-vocabulary"></a>
<a id="3-current-snowden-sourcecaller"></a>
<a id="4-oss-reference--ooni-probe-cli"></a>
<a id="5-own-modelprovenance-boundaries"></a>
