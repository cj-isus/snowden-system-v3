# R28-C02 — источники, pins и прочитанная область

Дата чтения: **28.10.2026**. Загруженный целый файл не равен FULL_TEXT_READ: ниже указан фактически прочитанный scope.

Private public-source captures и metadata: `.artifacts/research/r28-contracts/C02/`. В docs нет реальных endpoints/profiles/credentials/raw user logs. Captures не являются результатами VPN probes.

## S1. Нормативные первоисточники — SECTIONS_READ

| Источник | Статус / дата публикации | Прочитано и для какого вывода |
|---|---|---|
| [RFC9848](https://www.rfc-editor.org/rfc/rfc9848.html), *Bootstrapping TLS Encrypted ClientHello with DNS Service Bindings* | IETF Standards Track, March 2026 | §§3–6,8: endpoint/config compatibility, ECH/SVCB-reliant rule, CH ordering, Alt-Svc, mixed-RR/DNS disclosure. Нормы отделены от предложенной strict policy |
| [RFC9849](https://www.rfc-editor.org/rfc/rfc9849.html), *TLS Encrypted Client Hello* | IETF Standards Track, March 2026 | §§6.1,6.1.4–6.2.1; адресно §4.1 по config_id. Offer/acceptance/rejection, public-name scope, retry address consistency, source-bound persistence, GREASE и unsupported configs |
| [RFC9460](https://www.rfc-editor.org/rfc/rfc9460.html), *Service Binding and Parameter Specification via the DNS* | IETF Standards Track, November 2023 | §§2.3–3.1,5.1–5.2,8,9.1,10.4.4. Names, Alias/ServiceMode, mandatory compatibility, protected-resolution failures, optimistic connection consistency, independent A/AAAA/HTTPS lookups |
| [RFC9525](https://www.rfc-editor.org/rfc/rfc9525.html), *Service Identity in TLS* | IETF Standards Track, November 2023 | §6.1.1, соседний verification overview: reference identity не заимствуется из произвольного intermediate/предъявленного peer имени |

HTML прочитан через web tool; TXT тех же четырёх RFC получены с официального RFC Editor. `RFC-FETCH.json` содержит URL, SHA256 и длину, captures `rfc*.txt` сохранены. Это не full-text review всего корпуса RFC. Текущие errata отдельно не проверялись; опубликованные спецификации не названы текущей field поддержкой.

## S2. Actual manifest и source caller Windows — SECTIONS_READ

Пути ниже относительно `D:/SNOWDEN-SYSTEM-V3/`. Hashes и время snapshots: `LOCAL-INPUTS.json`, `LOCAL-INPUTS-2.json`; это идентичность файлов на записанный UTC, не EXE attestation.

| Файл | Прочитанная область |
|---|---|
| `windows/go.mod`, `windows/go.sum` | Версии/модульные checksum lines: Go1.26, sing-box1.13.19, uTLS1.8.4, sing0.8.13, miekg/dns1.1.72, x/net0.54.0; dependency updates не выполнялись |
| `windows/tools/package.ps1` | Строки build recipe/tag `with_utls,with_gvisor,with_quic,with_clash_api`, особенно :74; сборка NOT_RUN |
| `windows/corebridge.go` | :106–208: renderAndPrepareContext, existing snapshot consumer, VLESS/WS preflight, cached flag discarded, fallback hostname, dial overrides. Функция pinForChannel и содержимое vault не исследованы |
| `windows/backend/core/preflight.go` | Resolver structs/strategies, Resolve, UDP/JSON DoH parsing, cacheGet/cachePut/public-IP gate. Сетевые calls не выполнялись; resolve-cache.json не читался |
| `windows/backend/render/render.go` | :570–586 scoped bootstrap DNS; :737–763 VLESS/WS TLS/Host/IP override. Настоящие descriptors и сгенерированный runtime config не читались |
| `windows/netwatch.go` | Адресные :293–367 и :441–483, вызовы current session/probe/verification и network reevaluation; не full native lifecycle audit |
| `windows/failover.go` | :193–296, guard/current operation, candidate render, selector-only vs Reload, сохранение старых streams в комментарии и Validity.Constrain; не доказательство OS retirement |
| `windows/backend/render/fdmodel.go`, `dependency_graph.go` | Effective dependency fields, Comparison/Shared/Different/Unknown/Coverage, known denominator и rank calculation. Никакого live ASN/provider discovery |
| `windows/backend/netprobe/dns.go` | Фиксированное имя example.com, A query, bounded Encode/Decode signatures и comments; не generic HTTPS/SVCB parser |
| `windows/backend/core/probe_dns.go` | checkDNS: protected SOCKS wire DoH step, конечные per-source budgets, observation scope. Этот consumer не preflight JSON resolver |

Ни code execution, ни native/admin/lifecycle calls из этих файлов не выполнялись. Встреченные комментарии о доступности протоколов в РФ не приняты за измерения.

## S3. sing-box — primary pinned source, SECTIONS_READ

**v1.13.19 → `b5ebaa1fc0f2b94256180b95468e73ef53caa27d`**. Публичный [tag ref](https://api.github.com/repos/SagerNet/sing-box/git/ref/tags/v1.13.19) повторно получен; object type commit. `SagerNet-sing-box-tag.json` — capture. Local cache версия/время `.info` также прочитаны; go.sum проверен статически, `go mod verify` NOT_RUN.

| Immutable primary source | Прочитано |
|---|---|
| [common/tls/ech.go](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/common/tls/ech.go) | :1–55 и :109–189: config parse/DNS wrapper, TTL, successful negative refresh, handshake/Clone |
| [common/tls/client.go](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/common/tls/client.go) | :35–79, :93–141: backend choice, dial before handshake, one typed stdlib retry, Close and recursive retry false |
| [common/tls/utls_client.go](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/common/tls/utls_client.go) | :32–108, :145–186, :253–278, chrome mapping: state projection, config clone, chosen reference name, ECH gate |
| [common/tls/std_client.go](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/common/tls/std_client.go) | :48–127, ECH option gate: standard TLS object, source ownership, PKI/pin/DisableSNI distinction. Подробный pin audit не повторялся |
| [common/tls/ech_shared.go](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/common/tls/ech_shared.go) | :11–15 только ECHCapableConfig interface. Key generation не выполнялась |
| [protocol/vless/outbound.go](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/protocol/vless/outbound.go) | :44–82, :136–170: outbound/tls/transport constructors, InterfaceUpdated, no-mux TCP DialEarlyConn. Позднее получен remote byte-equal capture в EARLY-DATA-INDEX |
| [transport/v2raywebsocket/client.go](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/transport/v2raywebsocket/client.go) | :36–124: TLS dialer → Upgrade, Host/ALPN и lazy WS early-data branch |
| [common/dialer/resolve.go](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/common/dialer/resolve.go) | :82–116: domain Lookup vs literal, DialParallel/Serial |
| [adapter/dns.go](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/adapter/dns.go) | :17–59: Exchange/Lookup и QueryOptions, отсутствие typed service-plan output |
| [dns/router.go](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/dns/router.go) | :210–251 и :440–460: Exchange boundary, ClearCache и ResetNetwork; это не полный audit всех DNS transports |
| [route/network.go](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/route/network.go), [route/router.go](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/route/router.go) | :457–483 и :226–228 соответственно: противоположный CloseAll/InterfaceUpdated/DNS reset guard. Native callback достижимость не установлена. RESET-INDEX, remote equality |
| [transport/v2raywebsocket/conn.go](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/transport/v2raywebsocket/conn.go) | :157–240 EarlyWebsocketConn writeRequest/Write/WriteBuffer: префикс content в Base64 header/path, остаток после Upgrade; EARLY-DATA-INDEX, remote equality |
| [LICENSE](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/LICENSE) | Grant: GPL version3 or later; whole-product licence compliance не оценена |

У immutable public files, перечисленных в `UPSTREAM-INDEX.json` и `UPSTREAM-INDEX-2.json`, SHA256 совпал с соответствующим local cache. Это не claim equality всего module/artifact и не доказательство выбора этой source ветки установленным процессом.

## S4. MetaCubeX/uTLS — primary pinned source, SECTIONS_READ

**v1.8.4 → `cf49b0864331e156689feec6363ef9bf518a5ac7`**. [Повторно полученный tag ref](https://api.github.com/repos/MetaCubeX/utls/git/ref/tags/v1.8.4), capture `MetaCubeX-utls-tag.json`.

| Immutable primary source | Прочитано / отношение к opposing trace |
|---|---|
| [common.go](https://github.com/MetaCubeX/utls/blob/cf49b0864331e156689feec6363ef9bf518a5ac7/common.go) | ECH fields/comments :852–890, native ECHAccepted и config clone fields — capability/contract, не runtime observation |
| [u_parrots.go](https://github.com/MetaCubeX/utls/blob/cf49b0864331e156689feec6363ef9bf518a5ac7/u_parrots.go) | :2770–2810, :2851–2857, найденные Chrome GREASE lines; real public SNI source и preset context |
| [u_common.go](https://github.com/MetaCubeX/utls/blob/cf49b0864331e156689feec6363ef9bf518a5ac7/u_common.go) | :615,646 Auto→Chrome133; RESET-INDEX, remote equality |
| [u_tls_extensions.go](https://github.com/MetaCubeX/utls/blob/cf49b0864331e156689feec6363ef9bf518a5ac7/u_tls_extensions.go) | SNIExtension :123–137, :201–207: real-ECH guard не копирует outer P в config.ServerName |
| [u_conn.go](https://github.com/MetaCubeX/utls/blob/cf49b0864331e156689feec6363ef9bf518a5ac7/u_conn.go) | :106–170, :263–275, :550–590: build, config/SNI assignments, real ECH context and inner construction |
| [u_handshake_client.go](https://github.com/MetaCubeX/utls/blob/cf49b0864331e156689feec6363ef9bf518a5ac7/u_handshake_client.go) | :386–405, :469–500, :534–566: UConn handshake, outer c.serverName, context transfer to TLS1.3 |
| [handshake_client.go](https://github.com/MetaCubeX/utls/blob/cf49b0864331e156689feec6363ef9bf518a5ac7/handshake_client.go) | real config parse, outer-name assignments, :1161–1219 rejection/normal verifier, :1250–1263 callback guards. Адресный rg assignments по production files; это не security scan |
| [handshake_client_tls13.go](https://github.com/MetaCubeX/utls/blob/cf49b0864331e156689feec6363ef9bf518a5ac7/handshake_client_tls13.go) | :99–132 acceptance; :135–178 handshake sequence/rejection error до completed; retry extension extraction locations. Никакого crypto execution |
| [ech.go](https://github.com/MetaCubeX/utls/blob/cf49b0864331e156689feec6363ef9bf518a5ac7/ech.go) | :481–493 separate ECHRejectionError/Error, targeted searches отсутствия As/Unwrap в этом type definition, ConfigID/public name parse references |
| [LICENSE](https://github.com/MetaCubeX/utls/blob/cf49b0864331e156689feec6363ef9bf518a5ac7/LICENSE) | BSD-3-Clause conditions/disclaimer прочитаны |

Публичные bytes файлов, названных в upstream/RESET indices, сверены с local cache. O/P-only certificate cases — **ручные source models**, не generated certificates, handshake tests или exploit results.

## S5. Готовые DNS и TLS primitives

- **miekg/dns v1.1.72**, actual go.mod/go.sum; [svcb.go](https://github.com/miekg/dns/blob/v1.1.72/svcb.go) :218–243, :273–277, :521–530, :650–658 — SVCB/HTTPS/Mandatory/Port/ECH structs. [LICENSE](https://github.com/miekg/dns/blob/v1.1.72/LICENSE): BSD-3-Clause. SECTIONS_READ. Raw tag-version source/лицензия совпали с local bytes; `DNS-INDEX.json`. Tag не назван immutable commit; отдельный git commit pin не получен. Codec не объявлен service selector.
- **sing v0.8.13**, `common/tls/config.go:53–70`: ConfigCompat ClientHandshake dispatch; go.sum `yVoXnx9nPxfjlwD4Tp+Wd9zuW2tfiSVrcRDBZNbKRCw=`. SECTIONS_READ local source. Public [versioned source](https://github.com/SagerNet/sing/blob/v0.8.13/common/tls/config.go) — ссылка, remote bytes в этом пакете не загружались. Licence local GPL-3.0-or-later grant; remote equality/commit NOT_ESTABLISHED.
- **Go source reference1.26.7**: `C:/Program Files/Go/VERSION`, `src/crypto/tls/handshake_client.go:1127–1159,1191–1203`, `common.go` ECHAccepted/API fields; BSD-3-Clause LICENSE. SECTIONS_READ. Собственные byte copies в `go-reference/`, SHA256 `REFERENCE-INPUTS.json`. Это локальный публичный SDK source, не доказательство source/bin equality или используемого runtime toolchain. [Официальный TLS API](https://pkg.go.dev/crypto/tls) в этом пакете LINK_ONLY; текущие docs latest не подменяют source reference.
- **sing-vmess v0.2.8-0.20250909125414-3aed155119a1**, actual go.mod: [commit](https://github.com/SagerNet/sing-vmess/commit/3aed155119a174c9290a4099841049f0cb275e02) подтверждён public GitHub API по 12-hex prefix, capture `sing-vmess-commit.json`. [vless/client.go](https://github.com/SagerNet/sing-vmess/blob/3aed155119a174c9290a4099841049f0cb275e02/vless/client.go) :53–62,137–158; [vless/protocol.go](https://github.com/SagerNet/sing-vmess/blob/3aed155119a174c9290a4099841049f0cb275e02/vless/protocol.go) :124–168. SECTIONS_READ, `WriteRequest(...,payload)` и downstream buffer. [LICENSE](https://github.com/SagerNet/sing-vmess/blob/3aed155119a174c9290a4099841049f0cb275e02/LICENSE) grant GPL-3.0-or-later. Три файла remote/local equal; EARLY-DATA-INDEX. No-mux TCP flow scope, не все multiplex/UDP ветки.

## S6. Baseline / governance — не независимые первоисточники

- Root и `docs/AGENTS.md`, `windows/AGENTS.md`; current TASK/COMMON и CLAIM-FORMAT. Правила прав/паспорта, не evidence runtime.
- `docs/MAP.md` — навигационные строки/структура; `docs/LESSONS.md` — targeted ECH/TLS/resolver/ownership excerpts. Из них не выводится выполненная native приёмка.
- `transports/ECH-DNS-BOOTSTRAP.md` §§2–5,7; `COMPOSITION-AND-TLS-CONTRACTS.md` §§2–4: baseline source/model, не новая независимая проверка.
- `research/02-CENSORSHIP/REPORT-013.md` §§5,12 — actual caller baseline и partial endpoint closure; основные source transitions повторно прочитаны в нынешних pins.
- `REPORT-014.md` §§5,7,8.3–8.7 — ограничения field/classifier/control/lineage; новые OONI records/API в этом пакете не загружались, raw records не читались.
- Paired Z01/Z02 TASK — scope только; HANDOFF при адресном чтении 08:53:53Z и конечном review09:14:54Z не было. Не приписано independent agreement/review, ожидания/сообщений нет.
- Native memory `MEMORY.md:171–172` использована для role/evidence orientation, проверена по текущему TASK. Это не source научных claims.

<a id="s7-ошибки-приобретения--отрицательные-результаты"></a>
## S7. Полнота источников

- RFC и перечисленные upstream fetch завершились; файлы FETCHED не названы прочитанными целиком. Число remote/local equal files сверяется по именованным indices, не по всему дереву; поздние EARLY-DATA/RESET captures учитываются отдельно.
- Индексы фиксируют 18 primary files FETCHED/equal и ещё 2 DNS captures.
  Область чтения и сравнения ограничена перечисленными inputs.
- SOURCE_NOT_AVAILABLE для полного deployed artifact/profile/server identity: эти данные не доступны в разрешённой области и не добывались через секреты/live. Tests/build/native/live/crypto/field execution — **NOT_RUN**.

## S8. Проверка в конце

`REVIEW-DRAFT.json`, 09:14:21Z:28 named remote/local source records, несовпадений0;15 matrix rows; missing local links0;18 local/baseline inputs без drift. Последующий FINAL-REVIEW фиксирует конечные doc hashes и повторную статическую сверку. Числа относятся к этой области, не ко всему репозиторию/артефакту. Документ не присваивает scientific ACCEPTED или product PASS.
