# Метод исследования, источники и эксперименты

[Оглавление](../README.md) · [Термины](../GLOSSARY.md) · [Качество измерительного корпуса](MEASUREMENT-CORPUS-AND-COVERAGE.md)

## Краткий вывод

Сила вывода ограничена вопросом, источником и условиями наблюдения.
Контракт API, прослеженный вызов, результат лаборатории и поле подтверждают
разное. Различия ответов или похожий порог не устанавливают одну причину;
успешная малая проба не доказывает весь VPN-путь.

**Когда применять.** При проверке утверждения, выборе доказательства,
сопоставлении сообщений о блокировке и подготовке будущего сравнения.
Метод применим и к уже собранным материалам без новых измерений.

**Граница знания.** Реестр ниже отражает прочитанные материалы 21–27 октября 2026 года.
Он не заявляет полного чтения всех репозиториев или воспроизведения работ.
Списки опытов — условия будущей приёмки, не поручение запустить их сейчас.
Статусы и уровни определены в [общем словаре](../GLOSSARY.md);
детальные статистические условия — в [главе интервалов](../adaptation/INTERVALS-AND-POLICY-EVALUATION.md).

В этой главе: [паспорт утверждения](#source-6), [проектирование опыта](#source-10),
[источники S01–S27](#source-11), [источники D01–D29](#source-13-12),
[сопоставимость порогов](#measurement-comparability-2026-10-27).

<!-- source-unit: 6 -->
<a id="source-6"></a>
## 6. Как проверять новые факты (метод)

1. **Паспорт утверждения:** формулировка, источник/версия/дата, страна/ASN,
   измерение или дизайн, альтернативное объяснение, что его опровергнет.
   Документация подтверждает заявленный контракт; исходник — прочитанную
   реализацию при её предпосылках. Вызов ещё нужно проследить до выбранного
   потребителя. Unit покрывает выполненные случаи, live — проверенный путь
   и период. Наличие ссылки либо скачанного файла не означает полного чтения.
2. **Сначала вопрос, затем проверка:** не запускать live из наличия команды
   в документе. Офлайн — fixtures/анализ; стенд — отдельная изоляция;
   сеть владельца/сервер — после соответствующего запроса и доступной инфраструктуры.
3. **Локализация:** client→edge→origin→exit→target и обратное направление.
   cf-ray/Server — подсказки, не криптографическое доказательство origin;
   нужны nonce/идентификатор запроса и сопоставимые серверные данные.
4. **Причинность:** paired A/B, один фактор, чередование порядка, одинаковая
   полезная нагрузка; отмечать IP-family, DNS/reuse/cache, server load, время.
   Действие может менять и сеть: блокировка/TTL/остаточное состояние создают carryover.
5. **Приватность:** синтетические payload; публичные key fingerprints/config
   identity, не значения секретов и не хеши слабых паролей. Raw captures могут
   содержать адреса/имена/credentials — локальное ограниченное хранение и redaction.
6. **Вывод:** observation отдельно от inferred mechanism; явные unknown,
   failed/skipped runs, размер выборки и границы применимости. Возраст может снизить пригодность сведений для изменяющейся задачи,
   но не делает прежнее наблюдение ложным. Датированный факт о неизменяемом
   исходнике не устаревает так же, как доступность адреса. План экспериментов — §10.

---


<!-- /source-unit: 6 -->

<!-- source-unit: 10 -->
<a id="source-10"></a>
## 10. Программа экспериментов и критерии знания

Это спецификация будущих проверок. **В V2-109 они не запускались.**
Реализация и статус — PLAN §5.3; живые измерения требуют доступной
инфраструктуры и соответствующего запроса, публикация OONI не включается автоматически.


<!-- /source-unit: 10 -->

<!-- source-unit: 10.1 -->
<a id="source-10-1"></a>
### 10.1. Минимальный паспорт опыта

- Гипотеза и конкурирующие объяснения, контролируемый фактор и прогноз.
- Client/core/server/tool versions, config identity, capability set, topology,
  network epoch/ISP/family, время и ограничения capture. Не секреты.
- Workload, random seed/порядок A/B, fault schedule, warmup/cache/reuse,
  time/bytes/concurrency budget, abort policy и исключения.
- Метрики до запуска, точность/размер выборки по значимому эффекту и вариативности;
  нет универсального «трёх успешных запросов достаточно».
- Исходные синтетические наблюдения, failures/cancellations/timeouts, способ агрегации,
  доверительный интервал, полный знаменатель попыток и воспроизводимая команда.


<!-- /source-unit: 10.1 -->

<!-- source-unit: 10.2 -->
<a id="source-10-2"></a>
### 10.2. Матрица опытов

| ID / вопрос | Контроль и вмешательство | Измерение / критерий | Уровень, граница |
|---|---|---|---|
| E-01 Safety/lifecycle | Connect/Stop/error/crash/reboot/update; IPv4/IPv6/UDP/OS DNS; старый helper, зависший Start/Close | Не возникает разрешённый protected direct; intent и generation сохраняются; ошибка cleanup видима, время/ресурсы ограничены | Pure models → process tests → отдельная Windows VM. Unit не доказывает WFP |
| E-02 Freeze по объёму или времени? | Собственный endpoint, payload 1/8/16/32/64/256 KiB и выше по budget; разные pacing/reuse/новые carriers, направление, mux on/off | Completion и распределения bytes/time до остановки. Проверить конкурентов: idle timeout, server error, shared bandwidth | Controlled lab с известными fault rules → отдельные field blocks. Ни один размер не объявлять магическим |
| E-03 Лимит сессий / mux | Один exit/нагрузка, concurrency 1/2/4/8, фиксированный aggregate rate; reuse и mux как отдельные факторы | Handshakes/sec, queues, p95 TTFB, goodput, resets и живучесть существующих потоков | Стенд; фразы «mux помогает» без workload недостаточно |
| E-04 Active probing / TLS-in-TLS | Свой сервер и cover, корректный/неверный auth, replay, malformed/truncated input, разные паузы и веб-нагрузки | Ответы/время/размеры и benign-corpus confusion matrix с base-rate анализом; correct client проходит | Lab capture. Конечный корпус не доказывает неотличимость и атрибуцию проб цензору |
| E-05 Loss/MTU/idle/семантика | Loss/burst/reorder/RTT/rate, MTU blackhole, family, sleep/wake/network change; один параметр за раз | Goodput, latency соседнего потока, завершение TCP, реальный UDP/DNS и reconnect; CPU/RSS/handles | Изолированный Linux network lab + Windows VM. Не менять сеть рабочего хоста |
| E-06 Измеритель или путь? | Падает только echo/DNS/target, TLS expired, challenge, общий egress, clock jump; healthy alternative targets | Safety не PASS по недостатку данных; endpoint outage не роняет все каналы; classifier abstains; budgets соблюдены | Fixtures + local servers. Нет принудительного «whitelist» из x509/443/timeout |
| E-07 Независимость и distribution | Отключить общий origin/ASN/carrier/resolver/publisher, испортить signed update/pin, version rollback; unknown FD | Выбран работоспособный другой dependency path либо BLOCKED; no silent trust fallback; fresh provenance | Модели/стенд; реальный второй origin/ASN — только после OFF-09 |
| E-08 Польза адаптации / кандидат транспорта | Одинаковые topology/config/workload/fault schedule: bare pinned core, fixed channel, adaptive; затем один кандидат | Paired connect/recovery/TTFB/goodput, false switches/hour, probe bytes/hour, CPU/RSS, success rate; safety неизменна | Lab → оплаченный live → ограниченный canary. Данные сравниваемых продуктов отдельно от overhead baseline |

Toxiproxy полезен для TCP latency/reset/timeout/ограничения bytes [S21], но
`limit_data` **закрывает** соединение — это не точная модель silent drop после
порога. Для silence/UDP/packet loss нужен другой известный fault mechanism
в изолированном стенде. Сначала проверить, что инъекция действительно сработала;
успешный тест с неработавшим fault injector не считается прохождением сценария.


<!-- /source-unit: 10.2 -->

<!-- source-unit: 10.3 -->
<a id="source-10-3"></a>
<a id="103-статистика-и-stop-criteria"></a>
### 10.3. Статистика и критерии остановки

Сравнивать внутри блоков ISP×epoch×day×family×workload, чередовать A/B и
сохранять неуспешные серии. Повторения на одном middlebox могут зависеть от
общего состояния: считать их независимыми Bernoulli trials без основания нельзя.
Если к deadline операция не завершена, для длительности это правое
цензурирование при соответствующей модели; наблюдение не является нулевой
задержкой. Для цели «успех до D» исход уже определён как неуспех.
Тайм-аут из-за сбоя измерителя требует отдельной классификации. Скорость
только завершённых быстрых сессий может скрыть долю отказов.

Для uptime/failover учитывать длительность outages и полезные завершённые
операции. Percentile по маленькому n неустойчив; показывать n/распределение,
paired differences и интервалы (например блочное восстановление выборки при обоснованной схеме зависимости
и достаточном числе сопоставимых блоков). Для нуля отказов в заранее фиксированных n независимых Bernoulli-испытаниях
с одной вероятностью отказа p точная односторонняя 95%-я граница равна
`1 − 0.05^(1/n)`; приближение 3/n полезно при достаточно большом n.
Это не гарантия для произвольной адаптивной остановки, зависимых серий или
меняющегося p и не доказательство абсолютной безопасности.

Перед опытом выбрать primary metric, практически значимый effect и допуски
остальных метрик на baseline. Принять кандидат, если эффект устойчив в
отложенных блоках, safety не нарушена, resource/latency budgets выдержаны.
Остановить опыт при leak, trust failure, потере контроля нагрузки или saturation
стенда; такой run сохраняется как failure/invalid с причиной. Если эффекта нет,
не подбирать ещё десятки fingerprint до случайного PASS — пересмотреть гипотезу.


<!-- /source-unit: 10.3 -->

<!-- source-unit: 11 -->
<a id="source-11"></a>
## 11. Источники, версии и срок применимости

Проверены по доступному содержимому **2026-10-21**. RFC — контракт, upstream
docs/release — намерение/реализация автора, paper — исследование своей выборки,
issue — первичное полевое сообщение, а не репрезентативная статистика.
Проверка ограничена доступными публикациями, abstracts/страницами авторов,
относящимися к выводам разделами PDF и спецификаций; не заявляется репликация
экспериментов или независимый полный аудит исходников внешних проектов.

| ID | Первичный источник / период | Что обосновывает / граница |
|---|---|---|
| [S01] | Xue et al., IMC 2022, TSPU | In-path/stateful и распределённые измерения РФ того периода; не нынешний AND/ML/probing алгоритм |
| [S02] | Wu et al., USENIX 2023; измерения 2021–2023, Китай | Fully encrypted traffic имеет пассивные признаки; не перенос ready-made правил на РФ |
| [S03] | Xue et al., USENIX 2024 | Вложенные TLS handshake/bursts и пределы padding/mux; не гарантия XHTTP-защиты |
| [S04] | Cloudflare, 2025-06-26, данные с 9 июня | Ограничения своих сервисов из РФ; не вся сеть и не универсальный byte threshold |
| [S05] | OONI, interpreting data, страница в срезе 21 октября | Anomaly/confirmed/false positive; не включён публичный сбор данных |
| [S06] | RFC 9001, 2021, §§5.2/8 | QUIC Initial secrets и ограничения 0-RTT; не расшифровка application data |
| [S07] | RFC 9849, март 2026, §§10.1–10.4 | ECH threat model, anonymity set и downgrade resistance; IP/traffic correlation не исчезают |
| [S08] | net4people #490, 2025-06-27 | TCP byte-volume field report; вариативность и нет универсального счётчика |
| [S09] | net4people #546, 2025-11-14 | Гипотеза TLS session policing, неоднородные ответы; автор отмечает ограниченность данных |
| [S10] | net4people #417, 2024-11-07 | Сигнатура CF ECH в описанных сетях; это не текущий замер |
| [S11] | Hysteria2 protocol / документация в срезе 21 октября | HTTP/3 masquerade, auth, rate negotiation, Salamander; новых полей нет автоматически в нашем pin |
| [S12] | RFC 9114, 2022 | HTTP/3 streams; ограничения общего congestion/path остаются |
| [S13] | TCP Brutal upstream README | Rate-based TCP module, default caveat; не comparative speed evidence |
| [S14] | NaiveProxy README / root LICENSE | Chromium stack и padding protocol; BSD-style root, аудит всех third-party licenses впереди |
| [S15] | NaiveProxy v147.0.7727.49-3, commit c507fd6, prerelease | Preambles как конкретный development reference; не selected production version |
| [S16] | Xray maintainer discussion #4113, с 2024-12-05 | XHTTP modes/up-down separation; не parser support sing-box 1.13.19 |
| [S17] | Tor support report, январь 2026 | Неоднородность блокировок и distribution; не рейтинг пропускной способности |
| [S18] | Wails et al., USENIX 2025, UPGen | Research о генерации разных протоколов; не доказательство текущей РФ-устойчивости |
| [S19] | Microsoft, Azure domain fronting, 2023–2024 | Same-subscription exception; не произвольный фронтинг чужими доменами |
| [S20] | TUF spec 1.0.37, 2026-10-08 | Trust roles/thresholds/rollback/freeze, отдельная работа с clock; не наша compliance |
| [S21] | Shopify Toxiproxy README / MIT LICENSE | TCP fault injection; semantics toxic не тождественны DPI/UDP |
| [S22] | ESnet iperf3 docs 3.22 / BSD-3-Clause | Controlled bandwidth tool; официальные платформы Linux/FreeBSD/macOS, не VPN-proof |
| [S23] | RFC 9000, 2021, §9 | Path validation/migration в QUIC; не новый exit и не обход IP-блока |
| [S24] | Zohaib et al., USENIX 2025, Китай 2024+ | Измеренный QUIC Initial/SNI censor; не разрешение overload/DoS experiments |
| [S25] | sing-box TLS docs + локальный module cache 1.13.19 | Новые docs vs pinned Options: fragment/SPKI есть, spoof/XHTTP отсутствуют; поддержка доставки отдельно |
| [S26] | Microsoft WFP transaction API | Атомарная операция не означает persistent policy или доказанную OS-защиту |
| [S27] | Cheburcheck whitelist KB, страница в срезе 21 октября | SNI exceptions/диагностический опыт; mobile allowlist отделён, insecure пример не переносится в продукт |

Версия TUF в[S20]закреплена именно на 1.0.37. Прежняя
[плавающая ссылка latest](https://theupdateframework.github.io/specification/latest/)
сохранена для прослеживаемости, но не доказывает содержание названной версии.

**Источники прежней редакции:** Habr/dev.to/новости/внешний AI-обзор сохранены
как история поиска и генератор гипотез, но сами по себе не подтверждают скрытый
алгоритм ТСПУ. Юридические номера/санкции и точные современные доли deployment
вне перепроверенной технической выборки; не повышать их статус по соседству с RFC.
Если ссылка перестала открываться, сохранять DOI/tag/дату и искать первичную
копию; не считать 404 опровержением статьи. У IMC landing page в этой проверке
был 404, PDF с DOI доступен. Изменение источника → новая датированная сверка.

[S01]: https://censoredplanet.org/papers/tspu-imc22.pdf
[S02]: https://gfw.report/publications/usenixsecurity23/en/
[S03]: https://www.usenix.org/conference/usenixsecurity24/presentation/xue-fingerprinting
[S04]: https://blog.cloudflare.com/russian-internet-users-are-unable-to-access-the-open-internet
[S05]: https://ooni.org/support/interpreting-ooni-data/
[S06]: https://www.rfc-editor.org/rfc/rfc9001.html
[S07]: https://www.rfc-editor.org/rfc/rfc9849.html
[S08]: https://github.com/net4people/bbs/issues/490
[S09]: https://github.com/net4people/bbs/issues/546
[S10]: https://github.com/net4people/bbs/issues/417
[S11]: https://github.com/HyNetworks/hysteria/blob/master/PROTOCOL.md
[S12]: https://www.rfc-editor.org/rfc/rfc9114.html
[S13]: https://github.com/HyNetworks/tcp-brutal
[S14]: https://github.com/klzgrad/naiveproxy/blob/master/README.md
[S15]: https://github.com/klzgrad/naiveproxy/releases/tag/v147.0.7727.49-3
[S16]: https://github.com/XTLS/Xray-core/discussions/4113
[S17]: https://forum.torproject.org/t/tor-project-user-support-report-for-january-2026/21221
[S18]: https://www.usenix.org/conference/usenixsecurity25/presentation/wails
[S19]: https://learn.microsoft.com/en-gb/answers/questions/1461527/take-action-to-stop-domain-fronting-on-your-applic
[S20]: https://theupdateframework.github.io/specification/v1.0.37/
[S21]: https://github.com/Shopify/toxiproxy
[S22]: https://software.es.net/iperf/
[S23]: https://www.rfc-editor.org/rfc/rfc9000.html
[S24]: https://gfw.report/publications/usenixsecurity25/en/
[S25]: https://sing-box.sagernet.org/configuration/shared/tls/
[S26]: https://learn.microsoft.com/en-us/windows/win32/api/fwpmu/nf-fwpmu-fwpmtransactionbegin0
[S27]: https://cheburcheck.ru/kb/whitelist

---


<!-- /source-unit: 11 -->

<!-- source-unit: 13.7 -->
<a id="source-13-7"></a>
### 13.7. Качество измерения и независимость

Три URL одного origin/CDN/resolver/измерителя не являются тремя независимыми
голосами. Модель M01 задавала доступность общего узла 0.98, а трёх путей —
по 0.95 условно на доступность узла, с независимостью именно этих условных
отказов. Тогда `0.98 × (1 − 0.05³) = 0.9798775`.
Ошибочное предположение безусловной независимости трёх полных путей даёт
`1 − (1 − 0.98 × 0.95)³ = 0.999671491`.
Это ручная модель, не оценка VPS. [Граф общих отказов](../foundations/THREAT-MODEL.md#source-7-3)
содержит основной вывод; редактируемые веса score не становятся вероятностями.

Записывать cold/warm DNS/TLS/transport, размер входа, elapsed stage/bytes/completion,
network/owner/epoch и endpoint dependency; cache hit и keepalive отдельно от cold dial.
Проверять small и bounded bulk **по одному уже установленному соединению**, если
гипотеза касается объёма одной connection; повторная загрузка через новый socket
не проверяет тот же механизм. CDN cache/range/HTTP compression/connection pooling
меняют опыт и фиксируются, чтобы обычный cache miss не объявлялся ТСПУ.

Тайм-аут включать в результат/долю неуспеха; percentile только успешных запросов
может скрыть провалы. Не усреднять p99 разных окон/устройств: нужны counts и
совместимые merged histograms либо отдельные strata (M07). Медленный closed-loop
генератор может перестать посылать новые запросы во время stall и недосчитать
плохую задержку; фиксировать offered rate/пропущенные интервалы/исходный schedule.
Prometheus histogram semantics можно заимствовать без обязательного Prometheus/Grafana
на маленьком VPS. Session IDs/URL query/user app inventory не metrics labels. [D17]

Ноль ошибок в 100 независимых Bernoulli опытах ещё допускает≈2.95% failure probability
при односторонней 95% границе; в 1000 —≈0.299% (M06). Для зависимых ISP/time samples
эти iid-границы неприменимы. Safety доказывается инвариантами и scoped негативными
опытами; finite успешные probes не дают абсолютного «утечек нет».

РФ: использовать матрицу §12.4 и актуальные страты оператора/AS/времени; отдельные
mobile allowlist, UDP block, IP-prefix loss и volume cutoff. OONI2024 фиксирует
operator-specific reachability anomalies для cloud providers; это основание
измерять **точный** gateway IP, не blacklist каждого немецкого DC навсегда.
2026 USENIX talk о VPN failures — мотиватор независимой проверки, не новое наше
испытание и не сравнительный рейтинг провайдеров. [D24] [D25]


<!-- /source-unit: 13.7 -->

<!-- source-unit: 13.12 -->
<a id="source-13-12"></a>
<a id="reliability-sources"></a>
### 13.12. Первичные источники и границы

Сверка:2026-10-23. RFC — normative/experimental в своей области; mutable docs —
материал, требующий version gate. Список source files с 28 captured blobs и 5 LICENSE
хранится в evidence; root LICENSE не заменяет transitive SBOM. Китайские fingerprints
и выступление 2026 не являются измерением российского ТСПУ на нашем канале.

| ID | Первичный источник | Область применения |
|---|---|---|
| [D01] | RFC8899 | Datagram PLPMTUD/blackhole; не обход запрета UDP |
| [D02] | RFC9000 §14 |1200-byte UDP minimum/PMTU и ограничения внешней IP fragmentation |
| [D03] | RFC9002 | QUIC RTT/loss/PTO/congestion, отдельный transport recovery |
| [D04] | RFC9221 | Unreliable DATAGRAM и общий congestion context |
| [D05] | quic-go optimizations | Socket buffers/GSO/PMTUD; exact fork source имеет приоритет |
| [D06] | Hysteria performance | Окна/CPU/buffer/priority tradeoffs, не наш preset |
| [D07] | Hysteria2 protocol | Rate negotiation/Brutal-vs-adaptive semantics |
| [D08] | RFC8290, Experimental | FQ-CoDel в контролируемой очереди |
| [D09] | RFC8305 | Dual-stack Happy Eyeballs, в том числе NAT64/VPN constraints |
| [D10] | RFC2308 | Negative DNS caching, не произвольная трактовка timeout |
| [D11] | RFC8767 | Recursive serve-stale, не снятие trust expiry |
| [D12] | RFC8446 §8 | TLS1.3 0-RTT replay, не WS early-data |
| [D13] | Google SRE cascading failures | Retry amplification/global budgets/typed failures |
| [D14] | AWS backoff/jitter,2015/update2023 | Распределение retries; чужая OCC simulation не наш network benchmark |
| [D15] | Envoy adaptive concurrency | Алгоритмический reference, не новый обязательный proxy daemon |
| [D16] | Dean/Barroso, The Tail at Scale,2013 | Tail-latency мотивация; в этой выборке landing/abstract |
| [D17] | Prometheus histograms/summaries | Aggregation/quantiles/counts, не обязательный metrics stack |
| [D18] | Android regular network updates | Radio wakeups/batching, не VPN liveness proof |
| [D19] | ConnectivityManager.createSocketKeepalive | API29 IPsec NAT-T scope |
| [D20] | RFC4787 | NAT behavioural requirements; не measured ISP timeout |
| [D21] | Microsoft WFP Object Management | Dynamic/static/persistent/boot lifetimes |
| [D22] | Microsoft ALE Reauthorization | Existing-flow/policy-change semantics |
| [D23] | Go time monotonic docs | Suspend/serialization/clock semantics; exact target OS нужно проверить |
| [D24] | OONI Russia report2024 | Cloud-provider reachability по отдельным AS/периодам |
| [D25] | Ensafi, USENIX Security26 talk | Практические VPN failures, не новая наша evaluation |
| [D26] | Go GC guide | Soft GOMEMLIMIT/GC tradeoffs/non-Go allocations |
| [D27] | Linux PSI | CPU/memory/IO contention наблюдения |
| [D28] | systemd.resource-control | Cgroup resource controls, version-specific |
| [D29] | RFC9520,2023 | Обновление DNS failure caching; retry-storm protection |

[D01]: https://www.rfc-editor.org/rfc/rfc8899.html
[D02]: https://www.rfc-editor.org/rfc/rfc9000.html
[D03]: https://www.rfc-editor.org/rfc/rfc9002.html
[D04]: https://www.rfc-editor.org/rfc/rfc9221.html
[D05]: https://quic-go.net/docs/quic/optimizations/
[D06]: https://v2.hysteria.network/docs/advanced/Performance/
[D07]: https://v2.hysteria.network/docs/developers/Protocol/
[D08]: https://datatracker.ietf.org/doc/html/rfc8290
[D09]: https://www.rfc-editor.org/rfc/rfc8305.html
[D10]: https://www.rfc-editor.org/rfc/rfc2308.html
[D11]: https://www.rfc-editor.org/rfc/rfc8767.html
[D12]: https://www.rfc-editor.org/rfc/rfc8446.html
[D13]: https://sre.google/sre-book/addressing-cascading-failures/
[D14]: https://aws.amazon.com/blogs/architecture/exponential-backoff-and-jitter/
[D15]: https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/adaptive_concurrency_filter.html
[D16]: https://research.google/pubs/the-tail-at-scale/
[D17]: https://prometheus.io/docs/practices/histograms/
[D18]: https://developer.android.com/develop/connectivity/minimize-effect-regular-updates
[D19]: https://developer.android.com/reference/android/net/ConnectivityManager#createSocketKeepalive(android.net.Network,%20android.net.IpSecManager.UdpEncapsulationSocket,%20java.net.InetAddress,%20java.net.InetAddress,%20java.util.concurrent.Executor,%20android.net.SocketKeepalive.Callback)
[D20]: https://www.rfc-editor.org/rfc/rfc4787.html
[D21]: https://learn.microsoft.com/en-us/windows/win32/fwp/object-management
[D22]: https://learn.microsoft.com/en-us/windows/win32/fwp/ale-re-authorization
[D23]: https://pkg.go.dev/time#hdr-Monotonic_Clocks
[D24]: https://ooni.org/post/2024-russia-report
[D25]: https://www.usenix.org/conference/usenixsecurity26/presentation/ensafi
[D26]: https://go.dev/doc/gc-guide
[D27]: https://docs.kernel.org/accounting/psi.html
[D28]: https://www.freedesktop.org/software/systemd/man/latest/systemd.resource-control.html
[D29]: https://www.rfc-editor.org/rfc/rfc9520.html

---


<!-- /source-unit: 13.12 -->

<!-- source-unit: 15.1 -->
<a id="source-15-1"></a>
### 15.1. Что завершено и где проходит граница доказательства

Это исторический итог документального разбора 24 октября 2026 года,
а не утверждение о закрытии всех появившихся позднее вопросов. В него
входят исходная теория обхода, TH-00…08, повседневные/рабочие сценарии COM-00…19,
Android AND-00…18/A-FR-01…18, надёжность RQ-01…20 и уточнения TR-01…06.
Полный crosswalk **92 нормативных ID**, их реальных статусов, выбранного подхода
и будущей приёмки: `.artifacts/research/2026-10-24-research-completion/audit/coverage.json`.
Исторические F/E/FR/OFF сохранены отдельным маршрутом, а не объявлены выполненными.
Отчёт и воспроизводимая проверка документов/источников:
research completion — `docs/evidence/2026-10-24-research-completion.md`.

Для перечисленного тогда набора вопросов были подготовлены ответ по
исходникам/API либо решение о переиспользовании, адаптации, отложении или
отказе с будущей проверкой. Это достаточность для перехода к названным этапам,
а не доказательство полноты науки или всех поздних ограничений. Непроверенные сборка, OS enforcement, телефон, реальные маршруты,
удобство и скорость остаются в плане реализации. Их нельзя получить ещё одним
общим поиском статей. Новое исследование открывается при изменении требуемого
сценария, pin/API, угрозы или security advisory, которое отменяет принятое решение.

Старые §§1–6 сохраняют авторский текст и датированные наблюдения. Текущая трактовка
старых open questions — таблицы ниже и §§7–14. Отдельный gate требует actual
binary/module/source inputs; отсутствие новых данных не означает ноль, успех или
отсутствие блокировки. Свойство библиотеки, design contract и поведение продукта
имеют разный уровень доказательства.

**Цель «лучший» превращается в проверяемую многокритериальную задачу:** сначала
безопасность и корректность владения, затем useful completion/recovery/failure
tails, затем latency/goodput при ограниченных CPU/RSS/FD/bytes/energy. Нельзя
одновременно максимизировать все показатели для любых противников и сетей.
Точный endpoint/ISP/время и бюджет определяют допустимое решение. Отключённый
физический путь или запрещённый destination не восстанавливается одной сменой
SNI. FR-012/E-08 — будущие сравнительные доказательства, без обещания превосходства.


<!-- /source-unit: 15.1 -->

<a id="measurement-comparability-2026-10-27"></a>
## Сопоставимость порогов и контрольного объекта

**Сверено по первичным публикациям 2026-10-27.** Это уточнение E-02/E-03/E-06,
а не новая измеренная сигнатура цензора. Исторические источники подтверждают
свои сообщения и границы, не одинаковость скрытого механизма.

| Источник / дата публикации | Что сообщает источник | Что нельзя достраивать |
|---|---|---|
| [Cloudflare, 2025-06-26](https://blog.cloudflare.com/russian-internet-users-are-unable-to-access-the-open-internet/) | Ограничения собственных сервисов из РФ с 9 июня, включая TCP/TLS и QUIC; анализ указывает на первые 16 KB. В объяснении фигурируют 10–14 пакетов, Radar-инструмент видит аномалии в первых десяти | CDN не является заведомо устойчивым контролем. Окно измерителя не порог цензора; это не универсальная калибровка wire bytes/packets |
| [net4people #490, 2025-06-27](https://github.com/net4people/bbs/issues/490#issue-3183910059) | Исходный текст: около 15–20 KB server→client на TCP-соединение; отдельный upd5: 25 пакетов для TCP/UDP в любом направлении | Дата открытия issue не дата каждой правки. Дата upd5, правила счёта ACK/retransmissions и переносимость порога здесь не установлены |
| [net4people #546, 2025-11-14](https://github.com/net4people/bbs/issues/546#issue-3627041588) | Автор отделяет случай от #490; сообщает о помощи смены порта, сочетания no-flow+mux, затем 22 ноября об ограниченности эффекта | Не доказан общий connection-count алгоритм. Browser и tunnel не автоматически matched workloads; одновременные изменения flow/mux не выделяют эффект каждого |
| [OONI Web Connectivity, доступ 2026-10-27](https://ooni.org/nettest/web-connectivity/) | Методическая страница сравнивает пользовательский путь с контролем; отдельно предупреждает о GeoDNS, геозависимом содержимом и обычных отказах сайтов | Разные IP/ответы не всегда цензура. Страница не является аудитом текущего OONI-кода и не доказывает причину наших stalls |

Тело issue изменяемо; перепроверены доступные текст и указанные обновления,
не полная история всех комментариев. Дата обновления веб-страницы Cloudflare
не считается датой нового эксперимента. Перекрёстные ссылки между публикациями
не превращают их в независимые повторные измерения одного счётчика.

### Как типизировать порог

Для каждой попытки отдельно нужны application bytes, уникальные транспортные
payload bytes, пакеты на проводе, направление, elapsed time и стадия. ACK,
retransmissions, TLS overhead и capture/offload effects оговариваются.
Число вызовов `write()` и размер HTTP chunk не задают число пакетов на проводе.
Если capture видит только начало, конец окна не объявляется точкой остановки.

Конкурирующие объяснения: ограничение объёма, количества пакетов, времени,
числа соединений; PMTU/потери, серверный timeout, перегрузка, cache/ответ
приложения и отказ измерителя. Похожая форма графика не выбирает причину.
Даже одинаковый URL на двух точках может вести к разным CDN edges или разному
объекту. Для контролируемую передачу данных нужен синтетический объект с известными length/hash,
фиксированными HTTP status/encoding и фактически выбранным endpoint.
Hash здесь относится к тестовому объекту, не к пользовательскому содержимому.

### Ограниченная программа вместо рейтинга протоколов

Это **предложения пилотов**, не универсальные размер выборкиs или runtime presets.
Сначала отдельное разрешение, свои endpoints и известные fault controls.
Внутри блока фиксируются client/server pins, ISP/time/family, адрес, SNI/ALPN,
trust и aggregate workload; порядок попыток чередуется, carryover учитывается.

| Приоритет / gate | Конечный пилот | Какое различие проверяет |
|---|---|---|
| 1 / E-06 | Четыре controlled варианта: одинаковый объект, корректный GeoDNS, геозависимый объект, отказ приложения; по три парных повтора | Не выдаётся ли несовпадение контроля за вмешательство |
| 2 / E-02 | Два проверенных wire-сегментационных профиля × два pacing × два направления × три повтора: 24 соединения | Сохраняется ли точка остановки в единицах bytes, packets или time; одного порога не постулировать |
| 3 / E-03 | Concurrency 1/2/4 × mux off/on × три повтора: 18 серий; неизменный flow, одинаковые aggregate bytes/rate | Эффект числа carrier-соединений отдельно от изменения flow; mux overhead остаётся частью treatment |
| 4 / E-02/E-06 | Один endpoint/service на трёх согласованных портах, по три повтора | Разделение connect/TLS/data-stage outcome; вывод только об этих портах и окне |

Предварительный ceiling пилота: 30 секунд и 128 KiB application payload на
соединение, а для concurrency опыта на всю серию; отдельные конечные limits
для control/handshake/wire overhead задаются до запуска. Это выбранный budget,
не вывод из источников. Если он не достигает нужного режима, результат
**неопределённый результат**, не «блокировки нет». Нельзя расширять budget после просмотра
данных без фиксации нового опыта. Safety/trust failure, потеря контроля нагрузки
или невосстановившийся control прекращают серию. Три повтора не дают оценки
общероссийского failure rate; подтверждающий этап требует заранее выбранной
точности и независимых отложенных ISP/day/endpoint blocks (§10.3).

Все пилоты **NOT_RUN**. Нет запросов к исследуемым VPN endpoints, изменений
системной сети или автоматической публикации OONI. Краткий A/DoH/HTTPS probe
не заменяется bulk-измерителем по умолчанию; дополнительная нагрузка должна
быть обоснована решением, которое она способна изменить.

## Применение без нового исследования

К каждому существенному тезису достаточно сначала сопоставить уже сохранённые
источник, версию, прочитанный участок, предпосылки и опровергающий пример.
Если источник подтверждает только часть, вывод сужается. Если основания
не хватает, тезис остаётся открытым: количество ссылок не заменяет поддержку.
Противоречие классифицируется как различие версий, различие предпосылок,
исправленная ошибка либо действительно нерешённый вопрос.

Для [ПК](../../planning/knowledge-application/PC-PLAN.md) метод связывает
PC-03/07/09; для [Android](../../planning/knowledge-application/ANDROID-PLAN.md) —
ANDR-03/08 и device/полевые условия. Из методики берутся паспорт, исходы,
знаменатели и правила интерпретации, а не новый обязательный стек мониторинга.
Чужие инструменты и данные переиспользуются с версией, лицензией, известными
ограничениями инъекции и приватностью. Цена дополнительной пробы должна
оправдываться решением, которое её результат способен изменить.

Перечисленные лабораторные, native/device/live и comparative-опыты здесь
**NOT_RUN**. Статическая редактура не изменяет их статус и не запускает
новый сбор источников.
