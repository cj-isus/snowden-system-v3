# SOURCES R02-013 — проверенные спецификации, source/caller и поле

Дата чтения: **27 октября 2026 UTC**. Captures:
`.artifacts/research/theory-02/2026-10-27-ech-probing-res1/`.
HTTPS-downloads выполнялись с обычной проверкой TLS; `-k`/verify-disable
не использовались. Source и LICENSE не исполнялись. Source review и hashes
не являются product PASS.

## 1. Правило статусов чтения

- **FULL_TEXT_READ** — текст названного документа и относящиеся к выводу
  методы/ограничения прочитаны. В references найденная ссылка сама остаётся
  LINK_ONLY, пока отдельная работа не прочитана.
- **SECTIONS_READ** — получен полный файл, но научный вывод использует только
  явно прочитанные главы. Не объявляется чтение всех его references/appendices.
- **SOURCE_READ** — прочитан перечисленный source-path/symbol; наличие
  другого файла в capture-list не означает полный аудит его функций.
- **PARTIAL_FETCH / FETCH_FAILED / LINK_ONLY** — не основание переносить
  утверждение как достоверный первичный факт.
- Прочитанная secondary quote остаётся secondary. Чтение русского цитирования
  не равнозначно независимому чтению оригинала или сетевому измерению.

## 2. Нормативные документы

### P01 — ECH

**E. Rescorla, K. Oku, N. Sullivan, C. A. Wood.** *TLS Encrypted Client Hello*.
**RFC 9849, March 2026**, DOI 10.17487/RFC9849.
Stream IETF, Standards Track, **Proposed Standard**, не отдельный STD.

- Primary: <https://www.rfc-editor.org/rfc/rfc9849.txt>.
- Metadata: <https://www.rfc-editor.org/info/rfc9849>,
  <https://datatracker.ietf.org/doc/rfc9849/>.
- **SECTIONS_READ:** заголовок/статус и §§1–10.12; ключевые §§3.2/4/5,
  6.1.4–8, 6.2, 8.1.1–2, 10.1–7, 10.10.4–5.
- Read 2026-10-27 около 13:21–13:23Z; metadata до 13:26Z.
- Captures: `rfc9849.txt`, `ietf-rfc9849.html`, `info-rfc9849.html` и
  производные readable text (не альтернативный канонический источник).
- Поддерживает: номер/дата, real-vs-GREASE, public_name authenticated rejection,
  ограничения anonymity/forward secrecy и отсутствие availability guarantee.
- Не поддерживает: deployment конкретного канала, сегодняшнюю фильтрацию РФ,
  полноту формального доказательства цитируемой ECH-Analysis paper (LINK_ONLY).
- RFC Editor info WebFetch сначала timed out; curl с TLS verification получил
  официальный текст/metadata. Timeout не классифицирован как блокировка.

### P02 — DNS bootstrap ECH

**B. Schwartz, M. Bishop, E. Nygren.** *Bootstrapping TLS Encrypted ClientHello
with DNS Service Bindings*. **RFC 9848, March 2026**, DOI 10.17487/RFC9848.
Stream IETF, Standards Track, **Proposed Standard**.

- Primary: <https://www.rfc-editor.org/rfc/rfc9848.txt>.
- Metadata: <https://datatracker.ietf.org/doc/rfc9848/>.
- **FULL_TEXT_READ** (включая §§3–9 и references как реестр ссылок, не прочитанные
  отдельно работы). Read 2026-10-27 около 13:22–13:23Z; metadata 13:39Z.
- Capture `rfc9848.txt`, `ietf-rfc9848.html`.
- §3 — binary ECHConfigList/Base64 presentation, `ech` SvcParam;
  §4 — endpoint consistency; §5.1 — all-ech endpoints ⇒ SVCB-reliant;
  §5.3 — no ClientHello before resolution; §6 — Alt-Svc;
  §8 — mixed-list downgrade/anonymity/DNS disclosure; §9 — SvcParamKey=5.

### P03 — SVCB / HTTPS resource records

**B. Schwartz, M. Bishop, E. Nygren.** *Service Binding and Parameter
Specification via the DNS (SVCB and HTTPS Resource Records)*.
**RFC 9460, November 2023**, DOI 10.17487/RFC9460.
IETF stream, Standards Track, **Proposed Standard**.

- Primary: <https://www.rfc-editor.org/rfc/rfc9460.txt>.
- Metadata: <https://datatracker.ietf.org/doc/rfc9460/>.
- **SECTIONS_READ:** header/status, §§1–5, 7.3–9.4, 12–14.
  Остальные section/appendix contents не названы FULL_TEXT_READ.
- Read 2026-10-27 около 13:21–13:23Z; metadata 13:39Z.
- Capture `rfc9460.txt`, `ietf-rfc9460.html`.
- Конкретные пределы: §2.4 AliasMode/ServiceMode; §3.1 failure policy;
  §3.2 proxy privacy; §7.3 IP hints; §8 mandatory keys;
  §9 HTTPS QTYPE/port-prefix/origin identity; §12 cache/network partition.
- Type64=SVCB, Type65=HTTPS. В initial registry RFC9460 key5=`ech` только
  RESERVED; нынешнюю ECH семантику переносим из P02, не угадываем из этой строки.

### P04 — DoH: транспорт до resolver ≠ DNSSEC

**P. Hoffman, P. McManus.** *DNS Queries over HTTPS (DoH)*.
**RFC 8484, October 2018**, Standards Track.

- <https://www.rfc-editor.org/rfc/rfc8484.txt>.
- **SECTIONS_READ:** §§8–10, особенно §9 response integrity и §10
  resolver-selection/recursive dependency.
- Read 2026-10-27 около 13:46–13:47Z; capture `rfc8484.txt`.
- Не используется для заявления DNSSEC/nonfiltering текущего проекта.

### P05 — HPKE: криптографический primitive и non-goals

**R. Barnes, K. Bhargavan, B. Lipp, C. Wood.** *Hybrid Public Key Encryption*.
**RFC 9180, February 2022**, **Informational, IRTF/CFRG**, не Standards Track.

- <https://www.rfc-editor.org/rfc/rfc9180.txt>.
- **SECTIONS_READ:** header/status, §§9.1, 9.7.1–9.7.6, частично §9.8.
- Read 2026-10-27 около 13:46–13:47Z; capture `rfc9180.txt`.
- Поддерживает: отсутствие recipient-compromise forward secrecy;
  app embedding задаёт downgrade/replay/length свойства; Base sender
  authentication не переносится из других HPKE modes.

## 3. Полная научная статья о фильтрации

### F01 — FOCI 2025: методика + российский scope

**Niklas Niere, Felix Lange, Nico Heitmann, Juraj Somorovsky**,
Paderborn University. *Encrypted Client Hello (ECH) in Censorship
Circumvention*. **Free and Open Communications on the Internet 2025(2),
pp.64–73**. Published year 2025/issue2 подтверждены proceedings/PDF;
точный календарный день публикации этой страницы не установлен.
**Лицензия CC BY 4.0** — proceedings и PDF p.64.

- Proceedings: <https://petsymposium.org/foci/2025/foci-2025-0016.php>.
- Full PDF: <https://petsymposium.org/foci/2025/foci-2025-0016.pdf>.
- **FULL_TEXT_READ:** все 10 PDF-страниц: текст, методика, выводы,
  Limitations, References и Appendix A; refs не прочитаны вместо цитируемых papers.
- Read 2026-10-27 около 13:36–13:39Z после получения 13:30Z.
- Captures: `foci2025-ech.html`, `.pdf`, `foci2025-ech-utf8.txt`,
  `foci2025-ech-readable.txt`. Извлечение `pdftotext -enc UTF-8 -layout`;
  formfeed заменён маркером страницы только в readable derivative.
  Первоначальная default-encoding extract не использована для цитирования.

**Ключевые locators:**

| Страница / раздел | Результат и ограничение |
|---|---|
| p.67 §3.3 | Сравнения TLS/QUIC ECH, outer SNI, CF/non-CF destination; controlled German endpoint; DNS-providers только Firefox defaults |
| p.69 §5 | **March2025**: CF-range destination + cloudflare-ech.com + ECH-extension; 1500 дополнительных destination IP — не vantage points |
| p.70 §6.1 | По одной точке China/Russia/Iran; generalizability confidence автора не расширяет выборку |
| p.73 Appendix A Table4 | Москва, **AS50867**, Serverwala, HOSTKEY B.V. (private); это серверная точка, не named consumer mobile ISP |
| p.69 §5, p.70 §6 | TCP segments и TLS records по отдельности недостаточны, вместе помогали в исследованной постановке; не current product proof |
| p.71 Related Work vs pp.64/69, ref48 | «May2024» против November2024/issue417; отметили, не согласовали догадкой |
| p.68 Fig3 vs p.67 §4.1 | 7 vs 6 non-CF handshake successes; deployment-rate не выводился из противоречивой цифры |

**Ссылки, но не прочитанные artifacts:**
<https://github.com/UPB-SysSec/EchCensorshipResults> — **LINK_ONLY**;
pcap payloads не скачивались, packet-level самостоятельное подтверждение
не заявлено. Эти публикации — внешний эмпирический result, не наш experiment.

## 4. Поле, русский материал и provider deployment

### F02 — net4people/bbs #417

**wkrp.** *Blocking of Cloudflare ECH in Russia, 2024-11-05*.
Issue opened **07.11.2024**; title обозначает onset **05.11.2024**.

- <https://github.com/net4people/bbs/issues/417>.
- **FULL_TEXT_READ для полученной HTML страницы:** body и отображённые
  комментарии, последний видимый 25.08.2025. Hidden duplicate comment и
  возможное непоказанное содержимое не считается прочитанным.
- Read 2026-10-27 около 13:28–13:29Z.
- Capture `issue417.html`, readable text и derived embedded properties.
- Body: pairwise outer SNI/ECH controls, TCP/QUIC сообщения о нескольких
  сетях, но имена/AS всех сетей там не приведены.
- **21.07.2025 comment wkrp:** дополнение CF-IP condition **со ссылкой на F01**;
  это не второе независимое измерение той же paper-выборки.
- **08.11.2024 wkrp:** tentative Firefox/Chrome fallback notes, русский
  NTC Chromium130/Linux/Yota excerpt — вторичный для NTC.
- **12.02.2025 KershawChang:** Firefox fallback preference/default и ожидание
  native HTTPS resolution vs DoH — источник оговорки, не Firefox source audit.
- Browser policies/команды из comments не выполнялись и не предлагались как
  автоматическое снятие проектной защиты.
- GitHub REST issue/comments requests: HTTP403; initial HTML timeout,
  `?plain=1` HTML retrieval succeeded. Failed attempts не становятся source proof.

### F03 — русскоязычная secondary цитата РКН

**Интерфакс.** *РКН рекомендует отказаться от обходящего блокировки расширения
сервиса Cloudflare*. **02.04.2025, 18:22** (timezone в article не указан).

- <https://www.interfax.ru/digital/1017951>.
- **FULL_TEXT_READ для article body**, read 2026-10-27 около 13:36Z.
- Capture `interfax-ech.html`, readable text.
- Поддерживает факт публичной цитаты ведомства и упоминание statement
  07.11.2024. Не содержит методики/AS/packet-level описания фильтра.
- Статус **SECONDARY**, не подмена официальной первички.
- Полный article не переносится в REPORT; локальный capture для provenance,
  в научном тексте только необходимые краткие paraphrases.

### F04 — ЦМУ ССОП, прямая официальная первичка

**ЦМУ ССОП.** *Рекомендуем отказаться от CDN-сервиса CloudFlare*, **07.11.2024**.

- <https://cmu.gov.ru/ru/news/2024/11/07/рекомендуем-отказаться-от-cdn-сервиса-cloudflare/>.
- Paper ref38 использует альтернативный host
  <https://portal.noc.gov.ru/ru/news/2024/11/07/рекомендуем-отказаться-от-cdn-сервиса-cloudflare/>.
- **FETCH_FAILED**: direct hosts timeout (13:30/13:39Z). Original не прочитан.
- Русская secondary quote читается в F02/F03, но не объявляется независимой
  official-source verification. Никакое юридическое заключение об универсальном
  запрете ECH не выводится из quotation.

### D01 — Cloudflare: текущая публичная документация, не runtime зоны

**Cloudflare.** *ECH Protocol*, last updated **17.04.2026**.

- <https://developers.cloudflare.com/ssl/edge-certificates/ech/>.
- **FULL_TEXT_READ для main article** Overview/What/How/Enable/Enterprise;
  ссылки и navigation не аудит других pages.
- Read 2026-10-27 около 13:28Z; capture `cf-ech.html`, readable text.
- `public_name=cloudflare-ech.com`, supplier termination и deployment
  selection; Free-zone default как statement поставщика.
- Рекомендации страницы по DNS-filtering не считаются normative override
  RFC hard-fail policy и не выполнялись. Этот текст не доказывает фактический
  ECH-state конкретной зоны или доступность сервиса в РФ.

### R01 — попытка русского технического перевода спецификации

- <https://www.protokols.ru/WP/rfc9460/>.
- **PARTIAL_FETCH / FETCH_FAILED**: 16384 bytes, затем timeout (13:26Z).
  `ru-rfc9460.html` — частичный raw capture, не FULL_TEXT_READ.
- Автор/дата перевода не установлены из полного текста. Нормативные выводы
  проверены по оригиналу P03; нельзя назвать перевод источником этих выводов.

### R02 — ранний русскоязычный технический report-кандидат

- Habr <https://habr.com/ru/news/856342/> (находится в F02).
- **PARTIAL_FETCH / FETCH_FAILED**: 64543/117107 bytes, timeout 13:43Z;
  amp URL <https://habr.com/ru/amp/news/856342/> — HTTP404 13:48Z.
- Отдельное авторство/дату primary-статьи не установили. Текст, цитируемый в
  F02, читается как secondary quotation. Он не превращён в новый независимый
  источник количественных оценок сайтов/операторов.

## 5. Source: версии, callers и границы

Actual workspace: **D:\SNOWDEN-SYSTEM-V3**, не Obsidian docs-only snapshot.
Текущий manifest windows/go.mod: Go1.26, sing-box v1.13.19,
MetaCubeX/uTLS v1.8.4, sing v0.8.13.

| Input / pin | Прочитанные symbols / строки | Что доказано и что нет |
|---|---|---|
| windows/corebridge.go | renderAndPrepareContext:131–154/182–183 | Caller preflight→IP override для enabled VLESS+WS; не runtime сетевого запроса |
| windows/backend/core/preflight.go | Resolve:79–149; udpResolve:151–165; dohResolve:168–189 | Результат IP; provider JSON type=A; не ECHConfig/SVCB service binding |
| windows/backend/render/render.go | VLESS branch:737–761 | TLS server_name+utls/chrome, WS Host, без ech в этом branch |
| Taskfile.yml:6; windows/tools/package.ps1:74 | tags/build recipe | Требуется with_utls; не attestation существующего бинарника |
| sing-box v1.13.19, common/tls/client.go | NewClientWithOptions:52–69; defaultDialer:117–134 | uTLS/STD branch; stdlib ECHRejectionError one-retry guard |
| sing-box protocol/vless/outbound.go | NewOutbound:57–74 | Fact: renderer TLS options передаются TLS-конструктору, transport использует config |
| sing-box transport/v2raywebsocket/client.go | NewClient:36–42; dialContext:76–108 | TLS.NewDialer перед HTTP upgrade; не самостоятельный browser-DNS ECH consumer |
| sing-box common/tls/utls_client.go | :60–83,89–105,145–279,310 | Native ECHConfig capability, conditional ECH.Enabled, state literal **без ECHAccepted** |
| sing-box common/tls/std_client.go | ECH list :69–74, NewSTDClient :201–208 | Отдельная STD branch с conditional ECH.Enabled |
| sing-box common/tls/ech.go | `//go:build go1.24`; parse config:27–55; ECHClientConfig:109–189 | Static PEM или HTTPS-query wrapper; первый ech Answer; TTL; endpoint-selection не реализован данной функцией |
| sing-box dns/client.go | client Exchange:135–257/270+ | Router/transport/cache существуют; full DNS closure/authoritative validation не audited |
| uTLS v1.8.4 u_common.go | :615 | HelloChrome_Auto=HelloChrome_133 |
| uTLS u_parrots.go | Chrome133:884–956; SNI:2851–2857 | GREASE в fingerprint; public_name substitution условен real config list |
| uTLS u_ech.go | BoringGREASEECH:296–305 | Dummy ECH scheme, не приватный SNI |
| uTLS common.go / conn.go | :305–308 / :1718 | Native ECHAccepted field существует/заполняется |
| uTLS handshake_client_tls13.go / ech.go | :106–172 / :481–493 | Rejected ECH возвращает собственный named utls.ECHRejectionError; нет As/Unwrap в его определении |
| sing v0.8.13 common/tls/config.go | :53–65 | Compat ClientHandshake vs direct Config.Client dispatch; error propagation без показанного type conversion |
| Android NativeGraphSecurity.kt | :143–146 | Gate explicit ech.enabled/config; implicit query/file не разрешены. Только parser/source, не активный профиль или AAR |
| Local Go VERSION/crypto/tls/ech.go | VERSION go1.26.7; error :477–489 | stdlib named ECHRejectionError reference; не production binary toolchain proof |

### Независимая byte-сверка двух critical файлов

Commit для raw paths взят из ранее заявленного pin, но **независимо проверены
только эти bytes**, не весь commit/module происхождения:
`b5ebaa1fc0f2b94256180b95468e73ef53caa27d`.

- <https://raw.githubusercontent.com/SagerNet/sing-box/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/common/tls/ech.go>.
- <https://raw.githubusercontent.com/SagerNet/sing-box/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/transport/v2raywebsocket/client.go>.

Fetch 2026-10-27 около 13:41Z; SHA-256 remote/cache **совпали 2/2**.
Exact digests: `remote-equality.json`. Другие selected cache files не имеют
нового remote equality proof. go.mod/go.sum подтверждают selection pin,
не отсутствие подмены всего module cache, compiled provenance или live use.

### Licenses

- sing-box root LICENSE прочитан: **GPL-3.0-or-later** плюс запрет имени/
  implied association без consent. Это root notice, не full legal closure.
- uTLS root LICENSE прочитан: **BSD-3-Clause**.
- RFC тексты: IETF Trust legal notices; code-components Revised BSD notice.
- FOCI article: **CC BY 4.0**, paper/proceedings.

## 6. Capture/provenance и drift

- `source-inputs-initial.json`: 20 selected paths, captured
  **2026-10-27T13:43:40.112920Z**.
- `source-inputs-extra.json`: 9 extra paths; `source-inputs-boundary.json`: 2
  captures (u_ech повторён для boundary + Android gate).
- Финальная сверка `source-drift-final.json`: **31 capture-records / 30 unique
  paths, changed=0**, около **13:53:10Z**; sequential, не atomic tree snapshot.
- Эти manifests — own read-only inputs; старые source artifacts соседей не
  перезаписаны. Исторические parent snapshots не использованы как собственная
  actual session provenance.
- Первые GitHub REST failures, partially fetched RU pages и timeout официального
  host сохранены в собственном retrieval ledger; отсутствие content не скрыто.
- Никаких tests/fixtures/build/vet/doctor/crypto/bench/native/device/live/admin/
  install/deploy/OS-network changes, raw пользовательских logs или credentials.

Результат автора максимум **READY_FOR_REVIEW**. Число RFC, страниц, hashes
и источников само не переводит hypotheses/source claims в ACCEPTED.

## 7. FUP-1-01 — provenance дополнительного source/caller среза

Capture: **2026-10-27T14:49:39.978416Z**. Эта секция сохраняет результат
предыдущей source-партии; последующих source acquisitions, внешних запросов
или current-source checks не было. Актуальность дерева на более позднюю дату
этим capture не подтверждается.

### 7.1 Новые inputs, необходимые для ответа о binding

Организация/авторы исходников: **SagerNet / contributors**, выбранная версия
**sing-box v1.13.19**; дополнительный адресный caller — **sing v0.8.13**.
Версия из ранее прочитанного manifest, даты публикации каждого source-файла
не устанавливались. **SOURCE_READ** означает названные функции, не полный
построчный аудит каждого файла. Нового immutable remote equality для FUP нет.

Основной публичный источник:
<https://github.com/SagerNet/sing-box/tree/v1.13.19>.
Следующие пути читаются относительно этого module root:

| Source / локатор | Роль в выводе | Capture в `fup-1-01/` |
|---|---|---|
| `box.go:178–187` | Реальный Box создаёт dns.Router и регистрирует adapter.DNSRouter | `input-00-box.go` |
| `adapter/dns.go:17–39` | Exchange возвращает DNS Msg; Lookup — addresses; service tuple отсутствует в API | `input-01-dns.go` |
| `dns/router.go:210–324` | DNS rules/transport → client.Exchange → response; это не SvcPriority ranking | `input-02-router.go` |
| `dns/client.go:121–334,336–378,422–448` | Cache/transport/TTL/hints; адресный A/AAAA lookup отдельно от HTTPS | `input-03-client.go` |
| `dns/transport/https.go:158–226` | Конкретный DoH путь Pack/HTTP/Unpack возвращает Msg | `input-04-https.go` |
| `dns/transport/tls.go:103–145` | Конкретный DoT путь возвращает ReadMessage result | `input-05-tls.go` |
| `common/tls/ech.go:130–179` | QueryServerName/ServerName, QTYPE HTTPS, первый ech Answer, Client прежнего conn | `input-06-ech.go` |
| `common/tls/client.go:117–134` | Dial destination раньше ClientHandshake/ECH-query | `input-07-client.go` |
| `common/dialer/resolve.go:94–111` | Domain lookup возвращает адреса для прежнего destination | `input-08-resolve.go` |
| `common/dialer/default.go:246–270` | Default socket dial требует уже разрешённый адрес | `input-09-default.go` |
| `common/dialer/dialer.go:39–135` | Выбор Default/Detour и добавление ResolveDialer; условия, не все возможные deployments | `input-10-dialer.go` |
| `transport/v2raywebsocket/client.go:36–42,76–108` | TLS NewDialer получает c.serverAddr до HTTP Upgrade | `input-11-client.go` |
| Snowden `windows/backend/render/render.go:570–587,737–761` | DNS rules и не включающий ECH CDN-renderer; не выдумывать активный DoH-ECH consumer | `input-12-render.go` |
| Snowden `windows/go.mod` | Выбор версий; не build attestation | `input-13-go.mod` |

Все 14 source inputs перечислены с SHA-256 в `fup-1-01/inputs.json`.
Это последовательный датированный capture, не атомарный снимок репозитория.

Дополнительно в предыдущей партии были прочитаны:

- <https://github.com/SagerNet/sing/tree/v0.8.13/common/network/multi.go>,
  `DialSerial:16–31`, `DialParallel:49–126`: адресный перебор сохраняет
  `destination.Port`, а не получает новый port из HTTPS RR.
- sing-box `dns/transport/tcp.go:103–120`, `ReadMessage`: DNS-wire decode
  в Msg; это не service selection.

Для этих двух дополнительных файлов отдельные byte-captures до закрытия
партии **не сохранены**. Их locators — след прочитанного кода, не новый
SHA-256/current provenance.
Главный вывод о порядке dial→query и возвращаемом API уже поддержан 14 captures.

### 7.2 Повторно применённые нормативные основания

Используются уже полученные P01/P02/P03, без новых сетевых запросов:

- **RFC9460 §§2.4.1–3, 2.5.2, 3, 5.1, 8, 9.1:** wire order RRset не
  service priority; CNAME/AliasMode и ServiceMode различны; параметры одного
  endpoint совместны; reuse открытого conn требует consistency с RR.
- **RFC9848 §§3–5:** ech относится к конкретному alternative endpoint;
  publisher согласует адреса TargetName с key/public_name authority;
  no premature ClientHello и all-ech SVCB-reliant правила сохраняются.
- **RFC9849 §6.1.8:** связь persisted ECH-state с источником bootstrap.

Ручной A/B-случай REPORT §12 — иллюстрация уже прочитанного порядка вызовов,
не опубликованные DNS records, не fixture и не исполненный эксперимент.
Актуальность текущих исходников не проверялась. Полный refresh/retry
контракт при смене source/generation остаётся **не завершён**.

### 7.3 Сохранность и статус

Перед дополнением сохранены прежние REPORT-013, SOURCES-013 и handoff в
`fup-1-01/baseline-*`. Проверяется только сохранность собственных документов
и captures, без нового чтения текущего source-дерева.

- A/B/C: прежняя сдача **READY_FOR_REVIEW**, без само-принятия.
- D: ограниченный source/caller результат, переданный для рассмотрения.
- FUP в целом: **PARTIAL_AT_CUTOFF**; не объявляется завершённая ECH
  реализация или полная проверка произвольного Service Binding deployment.
