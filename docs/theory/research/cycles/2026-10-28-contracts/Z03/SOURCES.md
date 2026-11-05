# R28-Z03 — SOURCES: фактическое чтение, pin/caller и ограничения

Все даты чтения — 28 октября 2026. Источники продукта доступны в D:/SNOWDEN-SYSTEM-V3; unavailable старый Obsidian snapshot не описывает фактический source этого материала. Результат максимум READY_FOR_REVIEW.

Private captures только `.artifacts/research/r28-contracts/Z03/`:
`initial-inputs.json` (36 paths), `additional-inputs.json` (7), named raw sources,
`remote-byte-equality.json`. Захват sequential, не атомарный снимок.
Чтение исходников не запускало их функции. Для timeline используются реальные
UTC checks и metadata captures; mtime не точное время acquisition.

## 1. Общие инструкции и принятая математика

| ID | Источник / read scope | Что разрешено выводить |
|---|---|---|
| B2 | [NINE-2026-10-27-R03-010-A](../../../reviews/NINE-2026-10-27-R03-010-A.md), FULL_TEXT_READ | ACCEPTED для sum union/A1/A2/countable composition/postselection/min-example/covered subset; не bbolt/producer/FDR/whole package |
| B3 | [R03-010 REPORT](../../../03-ADAPTATION/REPORT-010.md), SECTIONS_READ §§5–6 + ранее доступное доказательство §3 | Предложенный registration/ledger baseline не current implementation |
| B4 | [INTERVALS-AND-POLICY-EVALUATION](../../../../adaptation/INTERVALS-AND-POLICY-EVALUATION.md), SECTIONS_READ §§1–5 и границы§6–7 | Joint envelope, finite denominator, unknown hidden H, α_stat+β accounting; ITT включает failures/retries/cost |


B2 — существующее решение руководителя, не самостоятельная новая рецензия
этого исполнителя. R28-Z03 предлагает только scope решения по новым claims.

## 2. L1 — current producer→publication→health→ranking

Источник: **локальные project source files**, snapshot hashes в own manifests.
Current build/executables не проверялись. Go module `windows/go.mod:1–8,102`:
`github.com/snowden-system/windows`, go1.26, sing-box1.13.19, bbolt pseudo-version.

| Файл / symbols и строки на captured cut | Read status / доказательная область |
|---|---|
| windows/probe_observation.go:69,84–170 | SECTIONS_READ: atomic ObservationID до factory/I/O, defer stamps/cancel/panic/stale; no pre-I/O durable write в этом теле |
| windows/probe_observation.go:183–214,297–360 | SECTIONS_READ: currentSubject guards, manual caller publication/drop; не полный App audit |
| windows/observer_publication.go:1–177 | FULL_TEXT_READ: operation/runtime/owner/epoch/clock guard, ordering exception unsafe, in-memory write, phase-specific health update |
| windows/chanhealth.go:35–229 | SECTIONS_READ: state/verdict, filters, result aggregation, events-ring, epoch reset; no statement о всех других storage paths |
| windows/scoring.go:77–99,128–228 | SECTIONS_READ: health state consumer, score/rank, recent epoch; это не p/CI model |
| windows/failover.go:43–159,420–528 | SECTIONS_READ: volatile policy/retry counters, exact generation tokens, candidate ranking and accepted switch flows |
| windows/observer_failover.go:1–114 | FULL_TEXT_READ: scoped attempt owns outcome; cancel/stale discard отдельно от failed candidate; no statistical alpha field |
| windows/corebridge.go:601–808 | SECTIONS_READ: fixed fast-reprobeCount2, cancel/current drops, initial-failure+recovered vs confirmed health semantics |
| windows/backend/core/probe.go:41–91,124–321 | SECTIONS_READ: Report clone, validation, resource admission, fixed child slots, wg.Wait, result aggregation, HTTP transport construction |
| windows/backend/core/probe_dns.go:1–88 | FULL_TEXT_READ: sequential DoH fallback, returned attempt array, stops on success, no unknown attempts fabrication |
| windows/backend/core/probe_http_observation.go:132–214 | SECTIONS_READ: bounded body, httptrace stage updates, client.Do; не physical send counter |
| windows/backend/core/diagnostic_budget.go:1–94 | FULL_TEXT_READ: process pool, reserved lanes/concurrency/recent resource totals, defer caller release. In-memory/resource, not statistical accountant |
| windows/backend/core/manager.go:69–111,154–193 | SECTIONS_READ: lastReport clone/current owner generation/op.done; остальные repeated rememberProbe callers точечный поиск, не полный lifecycle review |
| windows/bgprobe.go:207–237 | SECTIONS_READ: admission отказ не channel fail; QUIC not attempted; background success separate from health.FAIL |
| windows/app.go:102–191,942–991 | SECTIONS_READ: queue1024, tail500, buffered writer/drop/error behavior, no File.Sync в persistLogLines. К реальным пользовательским log bytes не обращались |
| shared/adaptive/evidence_types.go:128–237 | SECTIONS_READ: observation/censoring/custody metadata, PlatformAccepted projection, snapshot fields |
| shared/adaptive/evidence_validation.go:148–191; evidence.go:1–94 | SECTIONS_READ: duplicate IDs/sequences within snapshot, current coverage, retained cancelled not replacement. Это не completeness witness |
| shared/adaptive/budget_types.go:81–138 | SECTIONS_READ: attempt receipt/outcome and separate owner/resource retirement; schedule generation не globally counted attempts |
| windows/backend/filetxn/bounded_write.go:1–111 | FULL_TEXT_READ: existing original-operation retention on uncertain publication/Close; no hard waiting deadline; **не bbolt storage implementation** |

Прочитанный chain не имеет declared CS/alpha statistic. Поэтому отсутствие
statistical guarantee — не finding сломанного product promise, а запрет
усилить operational observation до нового research cohort без producer adapter.
Никакие комментарии «доказано live» из source не признаны новым PASS.

## 3. U1 — SagerNet/bbolt exact pinned implementation

- Организация/проект: SagerNet fork bbolt, исходная библиотека Ben Johnson.
- Pin: `v0.0.0-20231014093535-ea5cb2fe9f0a`; cache `.info` Time
  2023-10-14T09:35:35Z. `windows/go.mod:102`, go.sum:210–211;
  upstream `sing-box@v1.13.19/go.mod:29` тот же dependency.
- Module declares Go1.20 (local go.mod прочитан), current caller module Go1.26.
- Источник: https://github.com/SagerNet/bbolt/tree/ea5cb2fe9f0a ; actual selected
  raw URL в `remote-byte-equality.json`.
- **6/6 exact selected files** freshly retrieved по HTTPS и совпали с local cache:
  db.go, tx.go, bucket.go, bolt_windows.go, internal/common/meta.go, LICENSE.
  Whole module/closure/build attestation не заявлены. Короткий commit ref взят
  из manifest pseudo-version, не guessed main branch.
- LICENSE FULL_TEXT_READ: MIT, Ben Johnson2013, notice preservation.

| Symbols | Scope/source pointers |
|---|---|
| DB.Open/file lock | db.go:165–226 (mode/default/options), Timeout только flock, не transaction context |
| Begin/readers/writer | db.go:694–783 (read-only transaction lifetime; one writer mutex) |
| Update/View | db.go:825–887 (callback+Commit, rollback on callback error) |
| Batch | db.go:890–928,961–989 (callback can repeat, trySolo/retry) |
| Tx.Commit/OnCommit | tx.go:129–223 (dirty pages/meta write, close, commit callbacks); SECTIONS_READ |
| write/writeMeta | tx.go:440–445,467–484 (NoSync guards; WriteAt precedes meta Sync error) |
| Meta selection/checksum | db.go:1043–1063; internal/common/meta.go:24–65 (highest valid local txid, fallback previous; FNV64a checksum) |
| Put/Delete/Sequence | bucket.go:310–416 (overwrite/Delete/SetSequence allowed; no immutable log API) |
| Windows flock/mmap/fdatasync | bolt_windows.go:1–119 FULL_TEXT_READ (sentinel byte lock, file.Sync, mmap) |
| Ownership notes | doc.go:1–40 FULL_TEXT_READ (borrowed slices expire with tx; source claims не device durability measurement) |

Fresh selected source bytes равны captured/current local input. Это доказательство
именно bytes, не факта настройки NoSync, вызова Sync или safe filesystem у владельца.

## 4. U2 — реальный upstream bbolt consumer

SagerNet/sing-box `v1.13.19` local module cache, `experimental/cachefile/cache.go`:
SECTIONS_READ107–214: Open, deletion unknown buckets, wrappers и destructive
resetDB. Manifest прочитан. Не выдаётся за current statistical consumer или
доказательство, что такая DB вообще включена в runtime Snowden.

**Disposition:** cachefile wrapper/file REJECT для complete scientific ledger;
raw bbolt conditional REUSE, smallest owner adapter отдельно от cache cleanup.
Это полезная reuse цена/граница, не patch upstream/product.

## 5. U3 — Go reference: cancellation, retry и sync chain

- Local reference VERSION: **go1.26.7**, build metadata time2026-08-18.
  VERSION прочитан. Это reference source, не compiler attestation всего binary.
- LICENSE FULL_TEXT_READ: BSD-3-Clause, Go Authors2009.
- `src/context/context.go:227–239`: CancelFunc не ждёт work stop.
- `src/os/file_posix.go:160–170` → `internal/poll/fd_fsync_windows.go:9–15`
  → `syscall/syscall_windows.go:755–756` → FlushFileBuffers.
- `src/net/http/transport.go:818–862`: concrete retry conditions (reused
  connections, nothingWrittenError, replayable request, certain read errors).
- `src/net/http/request.go:1534–1548`: replayable methods/GetBody/idempotency header.
- Read status: SECTIONS_READ, никакие Go/package function не исполнялись.

No inference: source retry path существует, но это не claim, что конкретная
DoH/egress HTTP unit выполнена повторно. Full physical wire cost unknown.
NoBody retry и no-write evidence не превращают POST автоматически в безопасный
повтор business action. Our probe targets/code не user business operations.

## 6. P1/P2 — реальные официальные русские Win32 primary

### P1. Microsoft Learn: FlushFileBuffers

URL: https://learn.microsoft.com/ru-ru/windows/win32/api/fileapi/nf-fileapi-flushfilebuffers .
Microsoft; metadata ms.date2018-12-05, updated_at2026-01-07,
перевод MT ru-RU, чтение 28.10.2026. FULL_TEXT_READ основного reference-тела:
parameters/return/remarks/requirements прочитаны через WebFetch
и сохранённую HTTPS-страницу.

Подтверждает: flushed file buffers to device/channel, error returns/GetLastError,
GENERIC_WRITE, performance cost. Не exactly-once probe, не universal hardware
power-loss recovery, не completeness outside DB. Сначала secure curl timeout,
повторный successful retrieval после прочтения WebFetch сохранён; TLS check не
отключался. Capture `flushfilebuffers-ru.html` + text extraction.

### P2. Microsoft Learn: LockFileEx

URL: https://learn.microsoft.com/ru-ru/windows/win32/api/fileapi/nf-fileapi-lockfileex .
Microsoft; ms.date2018-12-05, updated_at2026-01-07, ru-RU MT.
**FULL_TEXT_READ reference body**, параметры/flags/returns/comments.

Подтверждает: byte-range lock, mapped view I/O не blocked; handle/process-exit
unlock (время release зависит от ресурсов), FAIL_IMMEDIATELY и OVERLAPPED semantics.
bbolt sentinel range сериализует cooperating Open, не все mmap writers.
Capture `lockfileex-ru.html` + text extraction. Actual functions не вызывались.

Русские страницы действительно прочитаны, не выведены простой заменой locale.
MT перевод не заменяет source traced call/условия; ссылки не публикуют user data.

## 7. P3/P4 — границы log proofs и logical retry

### P3. RFC9162

B. Laurie, E. Messeri, R. Stradling. *Certificate Transparency Version 2.0*.
December2021; IETF **Experimental**, не новый стандарт протокола нашего ledger.
https://www.rfc-editor.org/rfc/rfc9162.txt .
**SECTIONS_READ:** header/status, §§2.1.3/2.1.4, §11/11.2/11.3.
Полный text получен; остальные части не объявлены reviewed. Inclusion proves
leaf in named tree; consistency binds earlier prefix and newer head. §11.3
comparing different advertised views нужен для misbehaving-log detection.
Это контекст аргумента, не proof raw producer completeness.

### P4. RFC9110

R. Fielding, M. Nottingham, J. Reschke (eds.), *HTTP Semantics*, June2022,
STD97/Standards Track. https://www.rfc-editor.org/rfc/rfc9110.txt .
**SECTIONS_READ:** header и §9.2.2. Intended idempotent effect не исключает
повторных logs/side effects; retry possible при response loss even if original
succeeded. Не собственный network test и не гарантия всех requests safe.

HTML WebFetch по RFC9162/9110 timeout; alternate official HTTPS TXT obtained.
Ни FETCH_FAILED, ни ссылка на главу не приняты за прочитанную спецификацию.

## 8. Модель, peer review и provenance limits

- REPORT §§4–5: ручной conditional argument/failure traces; используются accepted
  theorem и interval formulas со scope. Дополнение §4.1 — causal-registration
  induction и hull при неизвестном started-only membership (c>0 confirmed,
  u possible units); вывод приведён явно, empty cohort отдельно. Это не новый
  статистический алгоритм/теорема ради новизны и не execution модели.
  Новые distribution/field rates не заданы.
- Связанный C03 HANDOFF отсутствовал на срез 09:13:49Z.
  Независимая парная сверка с ним не заявляется.
- Tests/fixtures/code/simulations/crypto/profiling/native/OS network **NOT_RUN**.
  SHA-256/source copies/text extraction — разрешённая статическая provenance,
  не statistical execution. Никакие real logs, actual DB/vault/runtime-config
  files или `.env` не открывались.
- Scoped drift на 09:20:20Z: исходные 36 inputs без изменений. Финальный
  cut 09:31:39Z: все 43 initial+additional inputs совпали, записано
  `submission-source-cut.json`. Это именованный scope, не freshness всего
  репозитория и не source runtime PASS.

