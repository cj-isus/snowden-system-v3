# R28-Z02 — SOURCES: scope, pin, caller и предел чтения

28.10.2026. Этот реестр относится только к R28-Z02, не присваивает прежним reports independent acceptance.

## 1. Доступность и provenance

COMMON/TASK/named methodology и B/C/D R02-014 доступны. Read-only source
captured у OONI публично доступен по exact revision; project consumers
Windows/adaptive не читались (вопрос — published data, не product API).
**SOURCE_NOT_AVAILABLE не требуется для всего пакета**, но backend
classifier revision и exact Android source revision **UNKNOWN/NOT_DISCLOSED**.

Private captures только `.artifacts/research/r28-contracts/Z02/`.
`baseline-copies.json` помечает3 raw-records и прочие baseline copies как
**previous public acquisition**, не свежие remote measurements.
`*.meta.json`: URLs, acquisition UTC, HTTP/effective URL, SHA256,
TLS verification enabled. `ACQUIRED_NOT_YET_READ` — immutable acquisition
этап; окончательный reading-depth — этот реестр/reading-ledger.
Никаких `-k`, account endpoints, probes, installs и запуска OONI.
`fetch_public.py` — HTTP чтение публикаций/source, не measurement runner.

## 2. Baseline — только нужные места

- `docs/theory/methodology/MEASUREMENT-CORPUS-AND-COVERAGE.md`:
  **SECTIONS_READ §§1–4**, baseline proposed methods, не аксиомы.
- `research/02-CENSORSHIP/REPORT-014.md`: **SECTIONS_READ §8.1–8.7**,
  selected matrix/claims через целевой поиск, без повторного обзора операторов.
- `research/02-CENSORSHIP/SOURCES-014.md`: **SECTIONS_READ S16–S19/S21–S25**,
  pin availability и precise read-depth; старые ссылки не новое acquisition.
- Applicable root/docs rules и COMMON/Z02 TASK прочитаны. Scoped AGENTS в
  cycles-пути не обнаружены при targeted find; карту проекта не регенерировали.

## 3. P1 — фиксированные3 архивные records и отдельный API-summary

Producer OONI Probe. Публикация records — archival OONI API; не paper.
Дата повторного **локального чтения28.10.2026**, remote acquisition прежней
сессии27.10.2026, указанная в copied receipts. **DATA_FIELDS_READ**:
metadata/version/annotations, queries, TCP/TLS/network_events, requests,
failure/status/body-presence/tags, control fields/headers. HTML response body
не является самостоятельно разобранной статьёй и не скачивался массово.

1. [AS12668 record16.09.2024](https://api.ooni.io/api/v1/raw_measurement?measurement_uid=20240916045716.646385_RU_webconnectivity_1cf1f2dd6e536360),
   `ooni-raw.raw`; engine3.22.0/revision97f1df33611097fdcd8e0cc53fe096c0c7727487;
   SHA256 `0ad7bacc87e98a490a6456c5b3bc273b2742180aa733a72ee1984e6ab76ba149`.
2. [AS8402 record31.01.2026 20:33:10](https://api.ooni.io/api/v1/raw_measurement?measurement_uid=20260131203410.740128_RU_webconnectivity_d10431598d631e23),
   `ooni-jan8402.raw`; engine3.26.0/revisione9b4dc98c04d347f0ee5468c2585e32d94812b73;
   SHA256 `b75507e5fe70ef19d29e9ef6db9311ca04342413e695983b49c80786cbb8204e`.
3. [AS8615 record31.01.2026 20:21:18](https://api.ooni.io/api/v1/raw_measurement?measurement_uid=20260131202120.671695_RU_webconnectivity_35145dae4a7e440f),
   `ooni-jan8615.raw`; Android app5.3.0/engine3.28.0/revision empty,
   network_type=wifi;
   SHA256 `522ada3e0904798ca53cc4f0944fcc732f6c693a37148ed04aa1d97e2a2dd45f`.

Hashes copied raw matching initial receipts verified28.10; **saved bytes
identity**, не binary attestation/truth. Не тянули новый набор records.

[Сохранённый summary запроса](https://api.ooni.io/api/v1/measurements?probe_cc=RU&since=2026-01-01&until=2026-02-01&test_name=web_connectivity&domain=explorer.ooni.org&limit=5),
`ooni-jan2026.raw`, DATA_FIELDS_READ только3/previous context flags relevant
S18/S19; page5rows не representative population. API backend_revision
не disclosure; `anomaly`, `confirmed`, `failure`, `scores`,
verification_status не подменяют raw outcomes. No fresh summary retrieval.

Artifact `block-inventory.json` — локальная статическая извлечённая сводка:
3records, declared timestamps, IP sets/intersections, fields presence и controls;
это не четвёртое measurement/no classifier execution. У всех local/control
DNS **нет IP intersection**; слова consistent основаны на другом критерии.

## 4. P2 — exact source OONI e9b4dc98… под S18

Организация: **OONI/probe-cli contributors**; commit
`e9b4dc98c04d347f0ee5468c2585e32d94812b73` указан в raw P1. Date recorded in
annotations2025-06-12T15:43:40Z; не независимая whole-build attestation.
Чтение/acquisition28.10.2026, immutable URLs, **не latest main**.

**FULL_TEXT_READ** каждого короткого code file ниже, кроме отмеченных
SECTIONS_READ. Build constraints у файлов в scoped trace не найдены как
ограничивающий tag; source присутствие не executable artifact proof.

- [webconnectivity.go](https://github.com/ooni/probe-cli/blob/e9b4dc98c04d347f0ee5468c2585e32d94812b73/internal/experiment/webconnectivity/webconnectivity.go)
  `sourcecaller.raw`: Run149–168 localDNS/control URL vs IP list;
  189–229 independent TCP/TLS/HTTP calls;235–245 exact TCP map comparison;
  251–272 local SummaryKeys. Fresh hash equals baseline `fup-wc326.raw`:
  `0d93b452533223f0f9449f8f424a51b5039c9a2e9b9504c58cd8cdf62afb91b4`.
- [summary.go](https://github.com/ooni/probe-cli/blob/e9b4dc98c04d347f0ee5468c2585e32d94812b73/internal/experiment/webconnectivity/summary.go)
  `summary-recorded-pin.raw`:118–129 HTTPS shortcut/control-unreachable;
  131–147 consistent NXDOMAIN;199–268 HTTP failure/control/DNS override.
  Fresh hash equals prior summary:
  `26327ff540f3b93eb1152a962df14e2ff162c78bdf6bc84a9cebaabb5f23776d`.
- [httpget.go](https://github.com/ooni/probe-cli/blob/e9b4dc98c04d347f0ee5468c2585e32d94812b73/internal/experiment/webconnectivity/httpget.go)
  `httpget-current-pin.raw`:35–69 local host/IP snapshot→Getter;
  hash equals prior file, filename «current-pin» здесь means recorded pin,
  не current Android/main.
- [urlgetter/configurer.go](https://github.com/ooni/probe-cli/blob/e9b4dc98c04d347f0ee5468c2585e32d94812b73/internal/experiment/urlgetter/configurer.go)
  `configurer.raw`:38–99 configuration/Saver/DNSCache/TLS/proxy state;
  snapshot map51–70.
- [urlgetter/getter.go](https://github.com/ooni/probe-cli/blob/e9b4dc98c04d347f0ee5468c2585e32d94812b73/internal/experiment/urlgetter/getter.go)
  **SECTIONS_READ 43–81/128–147**, copied prior `fup-getter326.raw`;
  Runner return-error and Saver events independent fields; last request first.
- [urlgetter/runner.go](https://github.com/ooni/probe-cli/blob/e9b4dc98c04d347f0ee5468c2585e32d94812b73/internal/experiment/urlgetter/runner.go)
  `runner.raw`:35–50 dispatch;63–103 httpClient/CopyContext whole body then
  optional status failure;114–127 TCP/TLS close operations.
- [legacy/netx/http.go](https://github.com/ooni/probe-cli/blob/e9b4dc98c04d347f0ee5468c2585e32d94812b73/internal/legacy/netx/http.go)
  `netxhttp.raw`:16–36 config Dialer/TLS;42–43 actual saver wrapper default cap;
  52–61 HTTP3 flag selects transport.
- [legacy/netx/dialer.go](https://github.com/ooni/probe-cli/blob/e9b4dc98c04d347f0ee5468c2585e32d94812b73/internal/legacy/netx/dialer.go)
  `netxdialer.raw`:14–27 NewResolver→DNSDialer/observers/proxy.
- [legacy/netx/tls.go](https://github.com/ooni/probe-cli/blob/e9b4dc98c04d347f0ee5468c2585e32d94812b73/internal/legacy/netx/tls.go)
  `netxtls.raw`:13–22 config.Dialer/handshaker/TLSConfig.
- [legacy/netx/resolver.go](https://github.com/ooni/probe-cli/blob/e9b4dc98c04d347f0ee5468c2585e32d94812b73/internal/legacy/netx/resolver.go)
  `netxresolver.raw`:13–24 wrapper order/cache.
- [netxlite/resolvercache.go](https://github.com/ooni/probe-cli/blob/e9b4dc98c04d347f0ee5468c2585e32d94812b73/internal/netxlite/resolvercache.go)
  `resolvercache.raw`:34–43 readOnly cache;64–83 lookup returns static hit;
  no TTL/age field/gate in this selected implementation. Missing caller
  freshness elsewhere не объявлено отсутствующим на всем product.
- [webconnectivity/dnsanalysis.go](https://github.com/ooni/probe-cli/blob/e9b4dc98c04d347f0ee5468c2585e32d94812b73/internal/experiment/webconnectivity/dnsanalysis.go)
  `dnsanalysis.raw`:63–90 common nonzero-AS sufficient for consistent;
  92–105 fallback IP intersection. Это новая verified semantic дельта.
- [webconnectivity/control.go](https://github.com/ooni/probe-cli/blob/e9b4dc98c04d347f0ee5468c2585e32d94812b73/internal/experiment/webconnectivity/control.go)
  `control.raw`:23–37 TH call;40–49 AS lookup locally.
- [model/th.go](https://github.com/ooni/probe-cli/blob/e9b4dc98c04d347f0ee5468c2585e32d94812b73/internal/model/th.go)
  `thmodel.raw`:9–19 separate HTTP URL/TCP IPs;39–46 HTTP result lacks endpoint/
  operation timestamps;88–96 TH maps. Helper runtime/version unknown.
- [legacy/tracex/http.go](https://github.com/ooni/probe-cli/blob/e9b4dc98c04d347f0ee5468c2585e32d94812b73/internal/legacy/tracex/http.go)
  `tracehttp.raw`:63–115 status/headers before snapshot read, body assignment
  after successful read, error early return;126–130 default131072.
  Positive snapshot success/Runner tail error — conditional manual trace,
  **не** reproduced incident/theoretical all responses blocked.
- [legacy/tracex/archival.go](https://github.com/ooni/probe-cli/blob/e9b4dc98c04d347f0ee5468c2585e32d94812b73/internal/legacy/tracex/archival.go)
  **SECTIONS_READ 43–127**, `archival.raw`:NewTCPConnectList outcome/time;
  NewRequestList last-first order and constructor not filling
  address/transaction_id/t0;no connection join inferred from timestamps.
- [model/archival.go](https://github.com/ooni/probe-cli/blob/e9b4dc98c04d347f0ee5468c2585e32d94812b73/internal/model/archival.go)
  **SECTIONS_READ 264–309**, `archivalmodel.raw`:optional fields exist but
  old constructor no assigned address/t0/transactionID. Modern schema cannot
  reconstruct absent historical data.
- [model/measurement.go](https://github.com/ooni/probe-cli/blob/e9b4dc98c04d347f0ee5468c2585e32d94812b73/internal/model/measurement.go)
  **SECTIONS_READ 49–54/82–174**, `measurement.raw`:UTC formatting,
  per-measurement and per-set start, nonserialized begin and local summary
  interface. URI response-time is not start-time by convention alone.

**Licence:** root [probe-cli LICENSE](https://github.com/ooni/probe-cli/blob/e9b4dc98c04d347f0ee5468c2585e32d94812b73/LICENSE),
`probe-license.raw`: **SECTIONS_READ header GPL version3**; scope does not
resolve GPL-or-later wording of every source/dependency. Reference-only,
нет переноса кода/installation/full compliance approval.

## 5. P3 — OONI primary: temporal DNS mapping, не transplanted classifier РФ

- Авторы: **Arturo Filastò, Maria Xynou, Mehul Gulati (OONI)**.
- «Collateral Damage of IP-Based Blocking During LALIGA Football Streaming
  in Spain: Evidence from OONI Measurements»; публикация **30.06.2026**.
- URL https://ooni.org/post/2026-laliga-collateral/.
- Чтение28.10.2026: **SECTIONS_READ** Methods/Identifying blocked IPs/
  Identifying affected domains/Temporal coupling/Limitations, extracts
  `laliga.txt:75–110,204–212,259–266`. Other headline/list/search не evidence
  для текущих выводов. Images/whole dataset/notebook NOT_RUN/NOT_READ.
- Испания01.01–01.06.2026, control Frankfurt; IP×probe-AS×hour, TH>5/<1%
  и local>=2/>90% — source **эвристики**, не наш confidence theorem и не RF classifier.
- DNS-scans4/day с04.05.2026; ранние periods use first later scan;
  partial means at least one IP not included in blocked set, may be untested.
  No quantitative headline reproduced here (разные totals/denominator в baseline).
- Source gist notebook https://gist.github.com/hellais/a43e2816178e4c310f5e19dec02d1954
  **LINK_ONLY**, не исполнен/не прочитан код/публичные IP lists не скачивались.
- Используем изменение вывода: mere currentDNS→historical IP join не measured
  website reachability; missing endpoints/temporal contamination требует unknown,
  а не automatic lower-bound truth.

## 6. P4 — OONI primary: test-path/submit-path/clock uncertainty

- Авторы: **Luis Díaz, Arturo Filastò / OONI**.
- «From Heuristics to Anonymous Credentials: Assessing OONI's Approach to
  Bad Measurements»; **06.07.2026**.
- URL https://ooni.org/post/2026-faulty-measurements/.
- Чтение28.10.2026 **SECTIONS_READ** Approach & Methodology, IP geolocation,
  timestamp inconsistencies, Probe OS/version metadata, new heuristics/human
  review (`faulty.txt:48–67,159–160,173–209,215–269`). Таблицы трафика/проценты
  просмотрены только как контекст, не воспроизводятся и не оценены измерением.
- Check-in logs **19–21.03.2025**; timestamp analysis **02.03–01.04.2025**,
  не07.2026 data; clientGeoIP vs collector egress, delayed upload vs bad clock,
  legitimate fork/dev environment объяснения. Нет каждого user identity.
- Unknown software string/metadata mismatch не автоматический fraud/нет
  измерения. Sender credential controls не ground-truth proof DNS/TLS.
- Credential protocol ни source reviewed здесь, ни внедрён; используется
  только ограничение inference provenance, не feature proposal.

## 7. P5 — OONI primary: доступность dashboard ≠ upload/no network block

- **Maria Xynou / OONI**, «Russia blocked OONI Explorer, a large open dataset
  on Internet censorship», **25.09.2024** + un-dated update про02.09.2025/
  январь2026.
- https://ooni.org/post/2024-russia-blocked-ooni-explorer/.
- Чтение28.10.2026 **SECTIONS_READ** update/Methods/Conclusion
  `publication2024.txt:19–22,45–64,90–97`; main site/dataset apps/upload
  separated by author. Endpoint graphs/images не оценивались anew.
- Это historical cited study, not new RF measurements; no extrapolation
  to current access. Нельзя prove app transmission every ISP по stable global counts.

## 8. P6 — exact schema reference/latest-read snapshot, не исторический engine

Organisation: **OONI/The Tor Project/spec contributors**.
Снимок, полученный из public current branch и **закреплённый до чтения**:
**8a68ed35b1dfeaeec737c0f1e8985865828d75d7**, authored22.10.2026
11:39:55+02:00 / committed15:09:55+05:30; metadata из public commit page.
Это modern **reference для нормализации**, не mapped historical source3.22/3.26/3.28.

- [df-000-base.md](https://github.com/ooni/spec/blob/8a68ed35b1dfeaeec737c0f1e8985865828d75d7/data-formats/df-000-base.md):
  **SECTIONS_READ 41–156,173–231**; per-input/per-run timestamp, runtime,
  optional backend_version/bucket_date и network_type. Базовые examples
  не current server addresses и не fill for missing fields.
- [df-005-tcpconnect.md](https://github.com/ooni/spec/blob/8a68ed35b1dfeaeec737c0f1e8985865828d75d7/data-formats/df-005-tcpconnect.md):
  **FULL_TEXT_READ**; ip/port/status,t,optional IDs unknown if missing;
  text describes blocked as string but example false. Actual historic
  ComputeTCPBlocking bool pointer — source priority, inconsistency explicit.
- [df-001-httpt.md](https://github.com/ooni/spec/blob/8a68ed35b1dfeaeec737c0f1e8985865828d75d7/data-formats/df-001-httpt.md):
  **SECTIONS_READ transaction/request/response schema** via targeted search,
  not whole examples. Metadata transaction/transport/address missing cannot inferred.
- [ts-017-web-connectivity.md](https://github.com/ooni/spec/blob/8a68ed35b1dfeaeec737c0f1e8985865828d75d7/nettests/ts-017-web-connectivity.md):
  **SECTIONS_READ local processing/accessibility+limitations 579–650**,
  no attempt reproduce spec algorithm in whole. HTTPS server-side block
  caveat is useful counterexample; spec current snapshot not whole historical parity.
- [spec LICENSE](https://github.com/ooni/spec/blob/8a68ed35b1dfeaeec737c0f1e8985865828d75d7/LICENSE):
  **FULL_TEXT_READ 27lines BSD3 conditions** (notice/disclaimer/no endorsement).
  Data CC licence/engineGPL licences remain separate.
- data-formats README acquired, **NOT_READ** beyond discovery; no attributed
  semantic claim from it. Directory/meta commit capture for pinned names,
  **DISCOVERY_METADATA**, not field/source proof.

## 9. P7 — OONI developer opposite example HTTPS success versus page denial

- **bassosimone**, «webconnectivity: server-side blocking limitations»,
  https://github.com/ooni/probe/issues/2661.
- Создано **30.01.2024**; точная дата edits текста неизвестна.
- Чтение28.10.2026 **SECTIONS_READ тело issue125–132**; описывает v0.5
  case и server-side CAPTCHA, **не** historical S18 actual incident.
  Historic source P2 supports same generic HTTPS shortcut independently.
- Linked test PR code не запущен/не audited. Issue counts/models не
  эмпирическое proof current РФ. Один developer statement insufficient by itself,
  opposing use supported by exact-source guard in P2.

## 10. P8 — primary RFC references: только нужные sections

### RFC2330

**V. Paxson, G. Almes, J. Mahdavi, M. Mathis**, «Framework for IP Performance
Metrics», **May1998**, Informational, не protocol mandatory guarantee.
https://www.rfc-editor.org/rfc/rfc2330.txt.
**SECTIONS_READ** §§9.2 temporal composition;10.1 clock offset/synchronization;
11/11.1 singleton/sample/statistics;13/14 packet-type+addresses,
`rfc2330.txt:688–777,995–1152,1596–1656`.
Ссылаемся на contrast single observation vs sample/biased selection и time
claim need assumptions, не копируем Poisson commands/generator code.

### RFC9111

**R. Fielding, M. Nottingham, J. Reschke (editors)**, «HTTP Caching»,
**June2022**, Standards Track/STD98.
https://www.rfc-editor.org/rfc/rfc9111.txt.
**SECTIONS_READ §§4.2.3,5.1**, calculating Age/date/current age and difference
of cached message from current connection,719–782/1056–1078. Datetime+Age
не TH operation timestamp/clock attestation.

### RFC1035

**P. Mockapetris**, «Domain Names — Implementation and Specification»,
**November1987**.
https://www.rfc-editor.org/rfc/rfc1035.txt.
**SECTIONS_READ §3.2.1 TTL598–605**: cache permission0 means current transaction.
Не принимаем historical SOA/minimum/signedness clauses как latest whole-DNS spec;
TTL scope only. Null raw TTL не numeric0/immutable interval.

## 11. P9 — OONI data licence, exact pin и цена reuse

Organisation: **OONI**; root copyright2020; licence repository commit
**ce329e1d6a2a0d0922d27557123e07e463b7c1e9**,
public authored/committed **14.03.2024 11:05:46+01:00**.
Получен public branch directory, затем immutable raw links; не «мне кажется CC».

- [data/LICENSE.md](https://github.com/ooni/license/blob/ce329e1d6a2a0d0922d27557123e07e463b7c1e9/data/LICENSE.md),
  **FULL_TEXT_READ**: CC BY-NC-SA4.0 объявлена для work.
- [CC4.0-BY-NC-SA.md](https://github.com/ooni/license/blob/ce329e1d6a2a0d0922d27557123e07e463b7c1e9/data/CC4.0-BY-NC-SA.md),
  **FULL_TEXT_READ legal text**; §§2/3/4/8 reference:
  NC reproduce/share/adapt/database rights, attribution/modification indication,
  SA for adapted shares, exceptions preserved, no endorsement. Это source licence
  analysis, не professional legal opinion для всего проекта и не permission
  product commercial ingestion. Monetary savings не измерены.
- [Readme.md](https://github.com/ooni/license/blob/ce329e1d6a2a0d0922d27557123e07e463b7c1e9/data/Readme.md),
  **FULL_TEXT_READ** короткой annotation; not stronger grant than LICENSE.

## 12. Ограниченный самостоятельный вывод/model, не новый external dataset

Deterministic finite-D formula `s/N≤theta≤(s+u)/N` в REPORT §6 выводится
непосредственно перебором двух краёв неизвестных cells **в тексте**;
счёт data/probe simulation отсутствует. Предпосылки:known finite D,N>0,
верные stage classifications и same target cells. Не уверенностьα/independence,
не claim сэмплирования всех сетей РФ. CommonError/source misclassification
могут отменить premise — показано opposing worlds.

## 13. Paired C02 read-only

C02 HANDOFF был уже готов на review-stage **09:14:35Z**, прочитаны
§1/общий scope/claimA и decision-use limits. Он разделяет попытку/credentials/
validation/user traffic; **paired compatibility input**, не acceptance
source ECH claims B/C или независимый сетевой witness.
Не ждали, не polling neighbors, не сообщения и не cross-write.

## 14. Неудачные ссылки и явно непрочитанное

- Historical guessed `internal/legacy/netx/httptransport.go`:404;
  исправлен по каталогу того же commit → `http.go`. Одинаковая ошибка не повторялась.
- `internal/experiment/urlgetter/config.go`:404; config defaults inferred
  **не** из отсутствия файла: read Runner/Configurer/helper actual used fields;
  unspecified unrelated configs не audited.
- Все source/spec dirs/commit pages используются metadata/path discovery,
  не blanket full repo review. Latest raw not used for historical engine.
- Backend code/current API/classifier snapshot отсутствуют в three-record
  inputs; не делали API replay, no substitutes current main.
- No external measurements/certificates/accounts/user logs; при public raw
  archive используем3 already-scoped records, не harvesting dashboard.
- Whole papers/notebooks/figures datasets не объявлены FULL_TEXT_READ только
  потому, что URL/publisher известен. Новое число опубликованных источников
  не proof прибавки эмпирического покрытия.

## 15. Read-depth и сохранность

Глубина чтения указана явно; прежние A/B/C/D сохраняют свои области.
Source manifests/hashes подтверждают полученные bytes и provenance,
не достоверность сетевых утверждений.
Comparison fresh source vs previous captures проверяет3files (caller,summary,
HTTPGet), не всё upstream closure. Документационные link-target проверки
не build/native/field PASS. Полная source/tree licence compliance NOT_REVIEWED.
