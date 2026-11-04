# R03-010 — источники и области фактического чтения

Дата чтения: **2026-10-27**. **READY_FOR_REVIEW**, не ACCEPTED.
URLs, хэши и timestamps последних записей — `acquisition-index.json` в
`.artifacts/research/theory-03/2026-10-27-ledger-reuse-res3/`. Время записи
файла не считается точным временем retrieval; диапазон команд получения
сохранён в исходном tool transcript.

`FULL_TEXT_PARTIAL_READ` означает: полный текст получен, **цитируемые
разделы/доказательства действительно прочитаны**, остальная статья не объявлена
прочитанной. `FULL_TEXT_READ` применяется только к целиком прочитанному
короткому LICENSE либо соответствующему файлу. Если текст
сохранялся раньше, provenance отмечен отдельно; link/index не называется
полным чтением. Никакие install/demo commands источников не выполнялись.

## P1. Confidence sequences и changing historical target

- **Авторы:** Steven R. Howard, Aaditya Ramdas, Jon McAuliffe, Jasjeet Sekhon.
- **Название:** *Time-uniform, nonparametric, nonasymptotic confidence sequences*.
- **Версия/дата:** arXiv:1810.08240v9, метаданные PDF «6 Aug 2022»;
  титульная дата PDF «August 9, 2022». Journal citation: Annals of Statistics
  49(2), 1055–1080 (2021), указан exact confseq README; это не дата v9.
- **Источник:** https://arxiv.org/pdf/1810.08240v9 ; journal DOI
  https://doi.org/10.1214/20-AOS1991 (journal host full text не получил).
- **Read status:** FULL_TEXT_PARTIAL_READ для (1), §1.1 (PDF pp.1–3), §2
  (pp.4–7), §4.1/Theorem 4 (pp.13–14), §6/Lemma 3 (pp.18–20),
  Appendix A.8 (p.35). Получен 48-page PDF; остальные разделы не объявлены
  полностью reviewed.
- **Подтверждает:** anytime событие, random-time substitution, отдельные
  one/two-side уровни; empirical Bernstein для **средней условных ожиданий**,
  без обязательного common mean; predictable predictors и exact MGF proof.
- **Не подтверждает:** любой post-selection pipeline, arbitrary finish-order
  sample, current rate/future availability, отсутствие цензурирования.
- **Capture:** `howard-1810.08240v9.pdf`, `...pdf.utf8.txt` (PyMuPDF text,
  пометки PDF pages). `howard-v9.txt` через pdftotext имеет непригодную для
  dedicated Read кодировку; интерпретация велась по UTF-8 extraction.

## P2. FDR/mFDR и conditional super-uniformity

- **Авторы:** Aaditya Ramdas, Tijana Zrnic, Martin J. Wainwright, Michael I. Jordan.
- **Название:** *SAFFRON: an Adaptive Algorithm for Online Control of the False
  Discovery Rate*.
- **Публикация:** PMLR 80:4286–4294, ICML 2018. Официальный index содержит
  `citation_publication_date=2018/07/03`.
- **Источники:** https://proceedings.mlr.press/v80/ramdas18a.html ;
  https://proceedings.mlr.press/v80/ramdas18a/ramdas18a.pdf .
- **Read status:** FULL_TEXT_PARTIAL_READ для §1, §2.1–2.3, Proposition 1,
  Theorem 1 и §3.1–3.2 proof (PDF pp.1–7); весь 9-page PDF получен,
  experiments pp.8–9 не использованы как own evidence.
- **Подтверждает:** (3)/(4) conditional super-uniformity; Theorem 1(a,b)
  mFDR, (c,d) дополнительные independence + monotonicity для FDR;
  α-wealth после discovery имеет специальную FDR модель, не общий FWER.
- **Граница:** не переносим synchronous rejection/candidacy history на
  pending callbacks без новой filtration модели. Не используем SAFFRON
  как второй reference runtime.
- **Captures:** `saffron-pmlr80-ramdas18a.pdf`, UTF-8 текст,
  `saffron-official-index.html`.

## P3. Различные критерии и original alpha-investing

- **Авторы:** Adel Javanmard, Andrea Montanari.
- **Название:** *On Online Control of False Discovery Rate*.
- **Версия:** actual downloaded PDF arXiv:1502.06197v2, metadata
  «4 Mar 2015», титульная дата «March 5, 2015». Непинованный URL при
  acquisition дал именно эту версию; не приписывается latest journal revision.
- **Источник:** https://arxiv.org/pdf/1502.06197 .
- **Read status:** FULL_TEXT_PARTIAL_READ для §1.1 (PDF pp.4–5), §3 (p.9);
  31-page PDF получен, другие main theorems и все appendices целиком
  не audited. Для ключевого proof A1/A2 он не нужен.
- **Подтверждает:** FWER/FDR/mFDR definitions, stabilized mFDR convention,
  различие original alpha-investing vs FDR; нельзя обещать FDR из имени метода.
- **Не подтверждает:** general algorithms из будущих revisions или runtime
  implementation проекта.
- **Capture:** `javanmard-montanari-1502.06197.pdf`, UTF-8 text.

## P4. Constant conditional mean и predictable-Hoeffding reference

- **Авторы:** Ian Waudby-Smith, Aaditya Ramdas.
- **Название:** *Estimating means of bounded random variables by betting*.
- **Версия/дата:** arXiv:2010.09686v7, 25 Aug 2022. Journal edition не
  подменяет прочитанный preprint.
- **Источник:** https://arxiv.org/html/2010.09686v7 .
- **Read status:** FULL_TEXT_PARTIAL_READ для §2, (1)–(7), Theorem 1 и proof,
  §3.1, Proposition 1, (8)–(12), §3.2/Theorem 2 assumptions. Читался **полный
  prior HTML capture**, приобретённый исходным R03 автором 2026-10-26T11:19:44Z.
  Fresh remote equality статьи в этой поставке не заявлена. SHA-256 старого
  `betting-v7.html` совпадает с `cycle-008/paper-inputs.json` перед копированием.
- **Captures:** copied `betting-v7.html`; `betting-v7-mathtext.txt` извлечён
  с заменой MathML на actual `alttext`, чтобы сохранять формулы.
- **Подтверждает:** [0,1]-bounded observations, постоянное conditional mean
  относительно filtration, predictable λ; proof CS inversion/Ville.
- **Не подтверждает:** отсутствие drift, weighted→unweighted interpretation,
  numerical validity любого confseq call при новых параметрах.

## C1. Один journal reuse: pinned SagerNet/bbolt (source scope)

- **Проект:** https://github.com/SagerNet/bbolt .
- **Exact version:** `v0.0.0-20231014093535-ea5cb2fe9f0a`;
  manifest `windows/go.mod:102`, `sing-box@v1.13.19/go.mod:29`;
  cache `.info Time=2023-10-14T09:35:35Z`.
- **Источник bytes:** local Go module cache
  `C:/Users/Пользо/go/pkg/mod/github.com/sagernet/bbolt@...`.
  Fresh GitHub commit API 403; полного remote byte equality нет.
  Версия pinned в manifest не является доказательством production artifact.
- **Лицензия:** FULL_TEXT_READ LICENSE, MIT, Ben Johnson (2013).
- **Прочитанные symbols:** doc.go; db.go Update:825–856, View:859–887,
  Batch:890–928; NoSync declaration:45–55; tx.go OnCommit/Commit:129–223,
  write/writeMeta fsync guards через статический поиск; bucket.go
  Put:310–342, Delete:345–373, Sequence/SetSequence/NextSequence:375–416;
  internal/common/types.go IgnoreNoSync (OpenBSD special case).
- **Actual caller:** `sing-box@v1.13.19/experimental/cachefile/cache.go`
  Start/Open:107–135, cleanup of unknown buckets:141–159,
  wrappers:175–202, destructive resetDB:205–214.
- **Reuse disposition:** low-level transactional primitive — условный
  кандидат; **cachefile wrapper/file — REJECT для complete ledger**.
  Нет утверждения о existing statistical ledger consumer или working
  Windows/Android deployment.
- **Proof boundaries:** библиотека позволяет Put overwrite/Delete/SetSequence;
  append-only/monotonic/error accountant enforcement требует trusted owner.
  Callback Batch может повториться; real action не включается в DB transaction.
  Срез copy→commit→probe не объявлен OS atomic или exactly-once.

## C2. Одна statistical reference: confseq

- **Проект/commit:**
  https://github.com/gostevehoward/confseq/tree/5ffe733ca2447a2e28c2c91f3b00086173f2ab2c .
- **Версия:** setup.py `__version__='0.0.11'`.
- **Лицензия:** FULL_TEXT_READ MIT, Copyright (c) 2021 confseq developers.
- **Secure fresh captures:** exact raw URLs для
  `src/confseq/predmix.py`, `LICENSE`, `setup.py`, `README.md` при этом commit.
- **API/source reviewed:** predmix_hoeffding_cs:172–220 →
  predmix_hoeffding_lower_cs:121–169 → predmix_lower_cs:10–69;
  fixed_n vs default λ path; α/2 для обеих сторон;
  predmix_hoeffding_ci:296–311/ci_seq:314–330 отличается tuning зависимостью.
- **Fresh predmix SHA-256:**
  `70456e9a3531582a0d0e146ba9ed6c8d643667025f5f4b85864d41dbd9036fdb`;
  совпал с prior R03 cycle-008 capture **для этого файла**, не whole repository.
- **Disposition:** OFFLINE_REFERENCE; не dependency import и не полноценный
  numerical/ABI/C++ audit. Даже корректная формула зависит от input
  support/order/mean/filtration; package не предоставляет missing producer model.
- **Существующие consumer search:** `confseq|AlphaSpending|FamilyError|FWER`
  в shared/adaptive, windows/backend, windows/scoring.go с исключением tests
  не дал статистического Go-consumer. Это bounded negative search, не полный
  поиск всего проекта, Android или deployed executable.

## L1. Реальный проектный baseline и границы read-only source

- Прочитаны: NINE-WINDOW редакция 2 (общие правила, §3, Follow-ups),
  авторский erratum; README scientific method; CLAIM-FORMAT; MAP;
  REPORT §§14/17/18; SOURCES адресные pins; shared/adaptive/evidence_types.go
  (Observation/censoring/custody и MaxReceipts) и фрагменты budget_types.go
  (resource reservation/retirement).
- `STATUS-010.md` создан отдельно; прочие общие files только read-only.
- 15 scoped local inputs скопированы в own artifacts с SHA-256,
  `local-source-manifest.json`; capture последовательный, не атомарный.
- Сверка 13:47:31Z и source cut 13:59:45Z: drift среди этих 15 inputs
  отсутствовал. Полнота P4 old HTML также сверена с прежним SHA-256.
- На финальном cut 14:04:43Z изменился **только протокол**: руководитель
  добавил FUP-7-01 и FUP-2-01 другим исследователям. Раздел Follow-ups прочитан
  адресно; поручения исследователю 3 нет, срок/слот R03-010 не изменились.
  Остальные 14 scoped source/baseline inputs совпали. Первоначальный capture
  протокола сохранён; новая запись не подменяет старый hash.
  Даже no source drift не product/runtime proof.
- Историческое STATE родителя не является собственным ownership. Bind,
  platform code/manifest update, LESSONS/общие STATE/REVIEW-QUEUE не выполнялись.

## Неудачные/непригодные retrieval: их нельзя превращать в источники

| Адрес / кандидат | Фактический результат | Доказательная область |
|---|---|---|
| arxiv HTML1810.08240v9 / abs1810.08240 | WebFetch timeout; **позже PDF получен** | Не был proof до чтения PDF |
| arxiv HTML1705.01058v4 | 404 | FETCH_FAILED |
| arxiv PDF1705.01058 | Полученная работа — астрофизика OGLE, не online FDR | REJECT_WRONG_SOURCE; сохранён для учёта ошибки, не цитируется математически |
| PMLR v119/ramdas20a.html | 404 | FETCH_FAILED; неподтверждённый URL исключён |
| ProjectEuclid journal 20-AOS1991.full | «Request unsuccessful», actual full text недоступен | FETCH_FAILED для host; не ABSTRACT_ONLY |
| MathNet dan27730 / person15047 | WebFetch malformed encoding; secure curl 403; автора/предмет не установили | FETCH_FAILED/UNUSABLE; не было доказательства sequential theorem |
| MCCME catalogue / redirected old.mccme.ru/free-books | redirect, WebFetch timeout, secure curl timeout (partial HTML17804 bytes) | FETCH_FAILED/PARTIAL, не полный русский primary |
| GitHub SagerNet/bbolt commit API ea5cb2fe9f0a | 403 | FETCH_FAILED для fresh commit metadata; local manifest/cache version available |

Русские источники предпочтительны, но неполученная глава Ширяева/Вальда/
Воронцова не заменяет доказательства. Библиографические упоминания из §18
в этом пакете **не использованы** как подтверждённые теоремы. Проверенные
англоязычные primary не переведены в fictitious ru-RU URLs.

## Ручная математика и NOT_RUN

REPORT §§3.1–3.5 и §4 — собственные elementary event/conditional expectation/
geometric-series derivations с перечисленными предпосылками; не математические
запуски, не наука о measured network rates. Section 6.2 прямо сопоставляет
source с P4 Proposition 1. 16 finite future cases в REPORT §8 — **NOT_RUN**.
Не выполнялись симуляции, crypto, statistics APIs, unit tests, benchmarking,
package imports/installations, native/device/live/system-network действия.
Python/pdftotext применялись только для копирования/хэширования/чтения публичных
источников; научные вычисления и библиотечные методы не исполнялись.
