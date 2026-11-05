# R02-013 — ECH/SVCB: bootstrap, границы приватности и доказательства фильтрации

27 октября 2026. **READY_FOR_REVIEW**. Исследование: чтение спецификаций,
full-text статьи FOCI и выбранных исходников. TLS/HPKE/DNS/QUIC execution
и live-измерения не выполнялись; пользовательские логи не использовались.

## 1. Пробел и что не принимается за аксиому

Baseline: [транспорты](../../transports/CENSORSHIP-MECHANISMS.md),
[DNS/диагностика](../../observability/DNS-AND-DIAGNOSTICS.md),
паспорта [R02-010](handoffs/R02-010.md) и [R02-011](handoffs/R02-011.md).
Они помогают найти вопрос, но их claims и прежние источники не становятся
принятыми вследствие повторной ссылки.

Проверяемый вопрос: какую именно приватность получает TLS/CDN-клиент от ECH,
какой bootstrap и retry нужны для этого свойства, и что можно утверждать
о фильтрации по имеющимся измерениям? В каноне номер/месяц RFC и тезис о
Cloudflare-фильтре присутствуют, но без полного property/caller crosswalk.
Нужно различить стандартизацию, опубликованные DNS-параметры, предложение
ECH, его принятие и фактический доступ к сервису.

## 2. Нормативная база — номер, дата, статус

Метаданные сверены по RFC Editor и IETF Datatracker; тексты получены напрямую
по HTTPS с проверкой сертификата. Подробный реестр чтения — [SOURCES-013](SOURCES-013.md).

| Документ | Название / авторы | Дата, статус | Точное значение |
|---|---|---|---|
| RFC 9849 | *TLS Encrypted Client Hello*, E. Rescorla, K. Oku, N. Sullivan, C. A. Wood | March 2026; IETF stream, Standards Track, **Proposed Standard** | ECHConfig, inner/outer ClientHello, принятие, rejection/retry, модель приватности |
| RFC 9848 | *Bootstrapping TLS Encrypted ClientHello with DNS Service Bindings*, B. Schwartz, M. Bishop, E. Nygren | March 2026; IETF, Standards Track, **Proposed Standard** | DNS SvcParam `ech`, endpoint binding, запрет части fallback |
| RFC 9460 | *Service Binding and Parameter Specification via the DNS (SVCB and HTTPS Resource Records)*, B. Schwartz, M. Bishop, E. Nygren | November 2023; IETF, Standards Track, **Proposed Standard** | SVCB RR type **64**, HTTPS RR type **65**, AliasMode/ServiceMode, mandatory keys, selection |
| RFC 8484 | *DNS Queries over HTTPS (DoH)*, P. Hoffman, P. McManus | October 2018; Standards Track | §§8–10: транспортная защита до resolver не равна DNSSEC integrity |
| RFC 9180 | *Hybrid Public Key Encryption*, R. Barnes, K. Bhargavan, B. Lipp, C. A. Wood | February 2022; Informational (IRTF) | §§9.1/9.7: HPKE Base не источник DNS-authenticity, не общий replay/fallback протокол |

**R02-013-A (нормативный вывод):** утверждение baseline «RFC 9849, март 2026»
подтверждено официальным документом, а не номером RFC или поисковой выдачей.
Однако весь клиентский контракт требует **RFC 9849 + RFC 9848 + RFC 9460**;
`ech` был только зарезервирован под номером 5 в первоначальном RFC 9460,
его семантику закрепляет RFC 9848 §3/§9. Proposed Standard не означает
Internet Standard (STD), включение в продукт или измеренную доступность.

### 2.1 Получение ECHConfig

1. Для HTTPS origin на 443 запрос — **HTTPS RR** по origin hostname;
   для другого порта — port-prefix `_port._https.name` (RFC 9460 §9.1).
   Обычный HTTP(S)-клиент не заменяет этот query на QTYPE=SVCB: §9 прямо
   запрещает SVCB queries/responses для схем http/https.
2. CNAME и AliasMode (`SvcPriority=0`) направляют resolution; в AliasMode
   SvcParams игнорируются. ServiceMode (`SvcPriority>0`) связывает
   `TargetName` и параметры **одного** alternative endpoint. Не склеивать
   `ech` из одного RR с port/address другого (RFC 9460 §§2.4/3).
3. `ech` — **ECHConfigList с length prefix**, wire bytes; в zone presentation —
   base64 без escape sequences (RFC 9848 §3). Формат публичный, это не секрет
   и не подпись издателя. TLS stack отбрасывает неподдерживаемые версии,
   алгоритмы и mandatory ECH-config extensions (RFC 9849 §§4/6.1).
4. Публикатор должен согласовать все адреса TargetName: соответствующие серверы
   либо умеют дешифровать данной ключевой конфигурацией, либо авторитетны для
   public_name, чтобы выполнить rejection recovery (RFC 9848 §4).
5. До окончания SVCB/HTTPS resolution ECH-capable клиент **MUST NOT** отправлять
   TLS ClientHello (RFC 9848 §5.3). Разрешённая предварительная работа с DNS
   и транспортом не разрешает преждевременно послать чувствительный SNI.

### 2.2 Доверие, приватность и доступность bootstrap — разные свойства

| Путь получения | Что защищено | Чего этот факт не доказывает |
|---|---|---|
| Plain DNS | Сам формат RR; дальнейшая TLS certificate verification остаётся отдельной | Конфиденциальность query, origin-authenticity ECHConfig, отсутствие strip/substitution |
| DoH/DoT с проверенной идентичностью выбранного resolver | Защита транзакции client↔resolver от локального on-path изменения/чтения | Честность resolver, DNSSEC-validation всей цепочки, отсутствие upstream filtering, доступность resolver |
| DNSSEC с валидацией и hard fail | Аутентичность DNS RRset/отрицательного ответа в заданной цепочке | Сокрытие имени query, отсутствие blocking/drop, честная текущая reachability |
| Предварительно предоставленный ECHConfig | Возможность начать ECH без текущего HTTPS lookup | Доверие к доставке, актуальность ключа и соответствие endpoint; эти предпосылки должен задать потребитель |

RFC 9849 §10.2 прямо допускает ECHConfig через неаутентифицированный DNS.
Это **совместимость**, не обещание защиты от DNS-attacker: он может подставить
свой ECH key и прочитать inner ClientHello либо удалить `ech`. TLS certificate
verification для origin всё ещё защищает иной уровень; нельзя считать её
ретроспективной защитой уже отправленного handshake-имени.
RFC 8484 §9 столь же прямо отделяет DoH transport security от DNSSEC.

**Контрпример:** злоумышленник меняет и A, и ECHConfig в plain DNS. Клиент
шифрует inner к ключу злоумышленника. Даже если затем TLS-сертификат origin
не проходит проверку и application data не отправлены, handshake privacy
уже нарушена. «TLS в конце всё проверит» — не достаточный bootstrap-контракт.

## 3. Принятие ECH, GREASE и retry/fallback

### 3.1 Минимальные состояния, не сводимые к одному boolean

Это предложенная **описательная** матрица для producers, не новый протокол:

| Состояние | Основание | Допустимый вывод |
|---|---|---|
| `not_configured` | Нет политики/потребителя реального ECH | Система не обещает шифровать inner SNI |
| `bootstrap_unknown` / `bootstrap_failed` | Нет результата HTTPS/config или запрос завершился ошибкой | Нет доказательства фильтрации; при соответствующей downgrade-resistant policy — ошибка, не silent fallback |
| `grease_only` | Dummy ECH extension без реального config/inner encryption | Проверяется переносимость extension, **не** сокрытие настоящего SNI |
| `real_ech_offered` | Выбран совместимый ECHConfig, создан ciphertext inner | Предложение свойства, ещё не end-to-end acceptance |
| `accepted_origin_tls` | Acceptance confirmation и успешное origin TLS authentication | Handshake privacy в модели RFC; не работа приложения/VPN-egress |
| `rejected_public_authenticated` | Rejection завершился handshake с проверенным public_name | Управляющий результат для retry; **не** successful origin connection |
| `rejected_unauthenticated` / `network_failure` | Public-name authentication или handshake не завершились | Ошибка; не доверенный сигнал отключить ECH |

RFC 9849 §6.1.4: acceptance определяется 8-byte подтверждением в ServerHello
либо HRR extension; весь TLS handshake и origin authentication остаются нужны.
Не считать частично разобранный ServerHello достаточным permit.

### 3.2 GREASE не скрывает реальный SNI

RFC 9849 §6.2: dummy ciphertext-like extension для не-ECH подключения;
такой клиент не предлагает реальный ECH для требований §6.1. Его настоящий
SNI может оставаться открытым. Полученные retry_configs нельзя сохранять
при GREASE (§6.2.1). GREASE — средство против ossification, не обещание
невидимости: §10.10.4 оставляет вне минимальной indistinguishability-модели
config_id, outer SNI, TLS-version/client-auth/HRR distinctions, знание
ECHConfigList и наблюдение многих соединений.

**Контрпример:** сканер видит extension 0xfe0d в uTLS/chrome и сообщает
«SNI зашифрован». В GREASE-случае hostname находится в обычном SNI;
пассивный читатель узнаёт его. Положительный extension-only test — ошибочный
privacy-test, но полезный отдельный compatibility-test.

### 3.3 Два разных вида fallback

- **DNS/endpoint fallback:** RFC 9848 §5.1: когда resolution успешно и **все**
  alternative endpoints имеют `ech`, ECH-capable SVCB-optional клиент **MUST**
  перейти к SVCB-reliant поведению. Generic fallback к authority endpoint без
  этих параметров не разрешён. Смешанный RRset с ECH и без него допускает
  selective-denial downgrade и **NOT RECOMMENDED** (§8).
- **Аутентифицированный ECH rejection:** RFC 9849 §6.1.6–7: завершить handshake
  с проверенным **public_name**, не посылать application data, закрыть с
  `ech_required`. При совместимых retry_configs — новая transport connection
  с ними; при перечисленных в RFC сигналах безопасного отключения — новый
  handshake с ECH disabled. Это не тот же путь, что generic timeout-fallback.
- **Unauthenticated failure:** RFC 9849 §6.1.6/§8.1.1: failure не является
  secure disable signal; unencrypted ClientHello fallback запрещён.
- Retry-state должен быть привязан к ECHConfig source и ограничен scope/lifetime
  (§6.1.8). Для немедленного retry нельзя безусловно подменить адреса свежим
  DNS-query: они могут относиться к иному public_name/config (§6.1.6).

«Direct connection» в RFC 9848 — прямое к authority endpoint без SVCB,
**не** проектный direct egress мимо VPN. Ни один нормативный retry не разрешает
снять OS protection, обойти подписанную destination policy или replay
пользовательской бизнес-операции.

## 4. Property matrix: наблюдатель и пределы ECH

| Свойство | Защищено при named assumptions | Остаётся видно / не обещано | Источник |
|---|---|---|---|
| Inner SNI и чувствительные inner extensions | HPKE к доверенному ECH recipient, совместимые peers, принятие | Outer SNI/public_name, endpoint IP, ECH extension metadata | RFC 9849 §§1/5/10.1/10.5 |
| Origin identity | Обычная успешная TLS verification по origin | public_name certificate не origin certificate authority | §§6.1.5/6.1.7; RFC 9460 §9.1 |
| Anonymity set | Совместная ECHConfig и согласованное внешнее поведение нескольких origin | k=1 и уникальные configs убирают полезную анонимность; timing/volume/popularity inference остаётся | RFC 9849 §10.1; RFC 9848 §8 |
| Неразличимость real/GREASE | Ограниченная модель §10.10.4 | Публичный стабильный outer SNI и известный IP-класс дают классификатору признаки | RFC 9849 §10.10.4; FOCI §4.2/5 |
| Forward secrecy application traffic | Отдельные TLS1.3 предпосылки и обмен ключами | HPKE inner ClientHello не forward-secret при компрометации ECH recipient key | RFC 9849 §10.10.5; RFC 9180 §9.7.4 |
| Защита от downgrade | Доверенный bootstrap/disable signal и корректная policy | DNS strip, mixed endpoint lists, managed policy и доверенный MITM имеют отдельные области | RFC 9849 §§8.1.2/10.2; RFC 9848 §§5.1/8 |
| Сокрытие других каналов идентичности | Отдельная защита DNS/cert-validation запросов | Plain DNS, OCSP/AIA/CRL, IP/timing и прежняя non-ECH history могут раскрыть имя | RFC 9849 §§1/10.7 |
| Доступность через фильтр | Не обещается криптографией | Drop IP/outer-SNI/extension, блок encrypted resolver | RFC 9848 §8; FOCI §§3.3/5/6.1 |

Split Mode дополнительно требует authenticated client-facing↔backend channel
и невозможности корреляции этих участков противником (RFC 9849 §10.1).
Cloudflare website/CDN documentation описывает termination на Cloudflare;
его свойство не следует автоматически переносить на иной split deployment.

## 5. R02-013-B — actual Windows CDN/uTLS caller

Проверено **исходное дерево**, не только документационная копия Obsidian.
`windows/go.mod`: Go 1.26, sing-box **v1.13.19**, MetaCubeX/uTLS **v1.8.4**,
sing **v0.8.13**. Build recipe `windows/tools/package.ps1:74` и Taskfile:6
используют `with_utls`; EXE/AAR provenance и исполнение **NOT_RUN**.
Sing-box `ech.go` имеет `//go:build go1.24`.

### 5.1 Точный путь, а не отрицательный grep по всему проекту

1. `windows/corebridge.go:131–154`: enabled VLESS+WS channels →
   `core.PreflightResolver.Resolve`; `WithDialOverrides` (:182–183).
2. `windows/backend/core/preflight.go:79–189`: lookup адресов через resolver/
   provider JSON `type=A`, результат — IP. Это не fetch ECHConfig и не
   ServiceMode endpoint selection. Plain UDP-ветка и DoH-ветка не имеют
   автоматически одного privacy/authority scope.
3. `windows/backend/render/render.go:737–761`: server может быть IP,
   но `tls.server_name` и WS Host остаются hostname; включены uTLS/chrome,
   **поле `tls.ech` здесь не создаётся**. WS early data не TLS ECH/0-RTT.
4. sing-box `protocol/vless/outbound.go:57–74` → NewClientWithOptions →
   `common/tls/client.go:64–69`: UTLS.Enabled выбирает NewUTLSClient.
5. `common/tls/utls_client.go:145–279`: `parseECHClientConfig` вызывается
   только при non-nil ECH options и Enabled (:259–266). Current renderer
   этот условный путь не активирует.
6. `transport/v2raywebsocket/client.go:36–42,76–93`: TLS NewDialer →
   handshake → HTTP Upgrade. Удалённые immutable bytes этого файла и ech.go
   совпали с local module cache; см. remote-equality.json.

### 5.2 Реальный GREASE и потеря ECH observability

- `uTLSClientHelloID("chrome")` → `HelloChrome_Auto` (sing-box :310);
  uTLS `u_common.go:615` связывает Auto с **HelloChrome_133**.
- `u_parrots.go:884–956`: профиль содержит **BoringGREASEECH()** (:953),
  а `u_ech.go:296–305` создаёт GREASE extension. Это source-состав профиля,
  не packet capture. Не утверждается, что каждый реальный negotiated handshake
  дойдёт до этой ветки без внешних ошибок.
- При nil real ECHConfigList `u_parrots.go:2851–2857` сохраняет обычный
  server_name в SNI; замена на public_name условна наличием real config.
- `uTLS/common.go:305–308` и `conn.go:1718` имеют ECHAccepted. Но
  sing-box `utlsConnWrapper.ConnectionState:89–105` формирует новый stdlib
  ConnectionState и **не присваивает ECHAccepted**. Для bool остаётся zero-value.
  Это projection-gap данного adapter: false здесь не устанавливает rejection
  или absence real ECH. Native state существует, STD path — другой путь.

**Короткий вывод B:** для рассмотренного Windows VLESS+WS/CDN renderer нет
включённого real-ECH consumer, зато выбранный uTLS fingerprint содержит GREASE.
Библиотечная ECH capability не равна включённому приватному handshake, и
adapter-level ECHAccepted не является полным observation. Ни утечка runtime,
ни отсутствие ECH во всех других consumers/Android этим не доказаны.

### 5.3 Готовый ECH path: почему не обещаем drop-in

sing-box `common/tls/ech.go:27–55` принимает явный PEM `ECH CONFIGS` либо
возвращает ECHClientConfig с DNSRouter. `fetchAndHandshake:130–179` делает
HTTPS query, берёт **первый** ech из Answer, сохраняет RR TTL/config и
создаёт TLS Conn. Сам этот wrapper не выбирает `SvcPriority/TargetName`,
не реализует AliasMode traversal/port-prefix mapping в данной функции.
Нижний DNS transport/router может вернуть уже развёрнутый Answer; его
полный closure не audited. Поэтому это **требование scoped review** выбранного
resolver/deployment, не blanket claim «sing-box не поддерживает SVCB».

`common/tls/client.go:117–134` умеет one retry по `*crypto/tls.ECHRejectionError`
с nonempty RetryConfigList. В uTLS определён **другой named type**
`*utls.ECHRejectionError` (`utls/ech.go:487–493`, возврат в
handshake_client_tls13.go:170–172), без As/Unwrap метода в этом определении.
Состав перечисленных интерфейсов не устанавливает преобразование ошибки.
Значит, при таком dynamic type нельзя предполагать срабатывание stdlib
errors.As-ветки: это **условная source-граница** для future uTLS+ECH consumer,
не установленное падение current renderer. Ни retry-loop, ни wire-semantics
не исполнялись. Пустая stdlib RetryConfigList возвращает error в данной
NewDialer-ветке, а не включает silent non-ECH fallback.

Для Android прочитан только security-gate
`NativeGraphSecurity.kt:143–146`: допускает `ech.enabled/config`, запрещает
implicit query_server_name/file scope. Это parser/policy capability, не
actual generated ECH profile, ECH acceptance или whole-AAR proof. Полный
Android source/caller аудит не входит в этот узкий Windows-срез.

## 6. R02-013-C — доказательства фильтрации и их область

### 6.1 Full-text академический источник

N. Niere, F. Lange, N. Heitmann, J. Somorovsky, Paderborn University,
*Encrypted Client Hello (ECH) in Censorship Circumvention*, **FOCI 2025(2),
pp.64–73**, CC BY 4.0. Прочитаны весь paper, методика, ограничения и Appendix A;
он рассматривает тогдашний draft, не реализацию финального RFC 2026.

- **§3.3, p.67:** сравнения ECH/TLS/QUIC handshakes; контролируемый endpoint
  в Германии и Cloudflare endpoints внутри/вне advertised ranges; варианты
  с/без ECH и outer-SNI; изменения framing. Опубликованы ссылки на pcaps,
  но pcaps/repository payloads **не скачивались и не перепроверялись**.
- **§5, p.69:** измерения **марта 2025**; блокировка ECH-трафика на исследованном
  пути требовала extension **и** `cloudflare-ech.com` в outer SNI **и**
  адреса в advertised Cloudflare ranges. Контроль на **1500 дополнительных
  server IP** уточняет третье условие, не 1500 российских ISP/vantage points.
- **§6.1, p.70; Appendix A Table4, p.73:** одна российская vantage point —
  **Москва, AS50867, vendor Serverwala, ISP HOSTKEY B.V.** Тип конечного
  домашнего/мобильного доступа не установлен: это арендованная серверная точка,
  не репрезентативная выборка MTS/МегаФон/T2/Билайн.
- В §5 не встретили censoring двух исследованных DNS providers на российском
  пути. Это не свидетельство отсутствия DNS/DoH-фильтрации в РФ вообще.
- TCP segmentation или TLS record fragmentation по отдельности оказались
  недостаточными, вместе помогали **в этой постановке**. Source не доказывает
  нынешний успех нашего record_fragment, UDP/QUIC или любое fragmentation.

**Внутренние оговорки paper:** основной текст и ref48 указывают November2024,
а Related Work p.71 содержит «May 2024» со ссылкой на тот же issue417;
это расхождение не разрешено отдельным первичным майским evidence.
Срок начала этой Cloudflare-signature нельзя датировать маем только по этой
строке. Figure3 даёт 7 non-CF successes, текст §4.1 — 6; эта статистика не
используется здесь для deployment-rate. Уверенность авторов в переносе на
другие сети (§6.1) — их интерпретация, не дополнительная измеренная выборка.

### 6.2 Полевой отчёт и русскоязычные сообщения

| Источник / дата | Что реально прочитано | Что поддерживает | Чего не поддерживает |
|---|---|---|---|
| wkrp, net4people/bbs#417, opened **07.11.2024**, title onset **05.11.2024** | Body + доступные comments HTML, последний видимый 25.08.2025 | Сообщение о нескольких сетях; pairwise SNI/ECH controls; TCP/QUIC; 21.07.2025 автор добавляет IP-condition со ссылкой на FOCI | Новые измерения каждой упомянутой сети, raw pcap proof автора здесь, нынешнюю nationwide rule |
| wkrp 08.11.2024, цитаты NTC | Firefox/Chrome observations, Yota явно только в цитате про Chromium130/Linux | Историческое сообщение о timeout/fallback в конкретных конфигурациях | Прямо прочитанную NTC-первичку, сегодняшнее браузерное поведение |
| KershawChang 12.02.2025, comment #417 | Уточнение про Firefox pref и DoH/native HTTPS-resolution race | Причина, почему ранний пересказ fallback требует осторожности | Source-audit Firefox этим пакетом и blanket verdict «все браузеры нарушают RFC» |
| ЦМУ ССОП, statement 07.11.2024 | Прямые cmu.gov.ru и portal.noc.gov.ru FETCH_FAILED; русская цитата в #417 | **Сообщение**, что ведомство заявляло ограничения TLS ECH посредством ТСПУ | Успешное независимое чтение оригинала, алгоритм/операторы/threshold |
| Интерфакс, **02.04.2025 18:22**, «РКН рекомендует отказаться…» | Полный article text, secondary цитата ведомства | Наличие публичного заявления об ограничении ECH | Измерительную методику или подпись к списку всех сетей |
| Cloudflare official ECH doc, updated **17.04.2026** | Основной текст Overview/How/Enable/Enterprise | Публичный common name cloudflare-ech.com; shared/CDN topology; Free zones default setting как заявление поставщика | Что конкретная зона/наш канал реально включены; свежую доступность из РФ |

Независимость этих источников ограничена: comment 21.07.2025 **пересказывает
FOCI**, а Interfax 02.04.2025 и русская quote #417 относятся к заявлениям
ведомства. Их нельзя посчитать четырьмя независимыми измерениями одной сигнатуры.

**Короткий вывод C:** подтверждён published measurement-result о составном
Cloudflare ECH-фильтре на одном названном российском пути марта 2025; ранний
two-predicate пересказ baseline неполон без destination-IP condition.
Универсальная/текущая ECH-блокировка, механизм на пути владельца и packet drop
в нашей сегодняшней среде — **UNKNOWN**. Даже timeout загрузки документа в
этом исследовании не классифицировался как ТСПУ.

## 7. Сильнейшие противоположные примеры

1. RFC существует, Cloudflare поддерживает ECH, fingerprint имеет 0xfe0d,
   но current renderer не получает real ECHConfig: GREASE, plain origin SNI.
2. ECH принят, но DNS query открыто или anonymity set=1: имя выводится по
   иному каналу, acceptance не гарантирует practical anonymity.
3. Исторический CF signature не сработал на другом endpoint: это не отсутствие
   фильтрации вообще; возможна отдельная IP/UDP/volume-filter policy.
4. HTTPS query timeout при живом A-answer: неверно автоматически отправлять
   non-ECH ClientHello после promise/privacy-reliant resolution.
5. Outer authentication разрешает retry, но не origin-success. Наш запрет
   прямого безопасного traffic fallback от этого не меняется.
6. uTLS adapter ECHAccepted=false при native true: ложный отрицательный verdict
   projection; рядом dummy extension даёт ложный положительный verdict sniffing.
7. Один valid ech Answer найден без связанного endpoint: наличие key bytes не
   гарантирует joint service/config correctness или свежесть.

## 8. Reuse, цена и адресаты (без реализации)

**Reuse:** уже транзитивные sing-box v1.13.19 (GPL-3.0-or-later, name restriction
в LICENSE) и uTLS v1.8.4 (BSD-3-Clause). Корневые licenses прочитаны; это не
проверка legal closure всех компонентов. Собственный cipher/HPKE не нужен.

**Adapt только после выбора продуктового сценария:** typed evidence
`config_source/endpoint_binding/bootstrap_protection/offered_kind/accepted_kind/
retry_auth/outcome`; origin-name verification отдельно от public_name; retain
scope/generation по existing observation/lifecycle контрактам. Не расширять
отрицательным boolean полномочия quarantine или platform permit.

**Стоимость:** HTTPS query и cache ownership; extra retry transport/latency;
DNS resolver становится дополнительной availability/privacy dependency;
key rotation и TTL; endpoint-bound caching; conditional uTLS/stdlib retry-type
и projection compatibility. Static config не требует recurring HTTPS query,
но требует trusted delivery, rotation и recovery-policy. Числа RTT/CPU/RAM/
вероятности и размер anonymity set текущего канала не измерены.

**Адресаты:** TH-02 — observational scope/unknown; TH-03 — bootstrap/config
source и endpoint binding; TH-05 — exact TLS capability/compatibility.
Для Android: только route к существующим владельцам наблюдений/profile
admission; AND-ID и реализация не закрываются этим пакетом.

## 9. Конечные будущие cases — ВСЕ NOT_RUN

| Case | Проверить при отдельном разрешении | Отрицательный/противоположный контроль |
|---|---|---|
| E01 | real ECHConfig + supported endpoint → acceptance + origin auth | GREASE без config; unsupported ECH versions |
| E02 | HTTPS alias/service resolution с binding ключа/port/TargetName | params от разных RR, AliasMode ech, duplicate keys, port-prefix |
| E03 | authenticated resolver/DNSSEC policy при HTTPS failure | A успех + HTTPS timeout; strip ech; bogus record |
| E04 | все endpoints ech → отсутствие generic non-ECH fallback | mixed RRset и selective denial как downgrade branch |
| E05 | public-name authenticated rejection с совместимым retry_configs | wrong cert/public_name, timeout, неподдерживаемая/пустая list |
| E06 | ровно bounded transport retry на actual STD/uTLS caller | dynamic utls.ECHRejectionError vs stdlib type |
| E07 | native accepted state сохранён observation adapter | ECHAccepted omitted; HandshakeComplete-only verdict |
| E08 | сеть/profile/config source сменились → scope/lifetime корректны | stale retry config, новый DNS endpoint с другим public_name |
| E09 | контрольная matrix CF-IP × outer-SNI × ECH/GREASE × TCP/QUIC | same endpoint without ECH; non-CF endpoint; фиксированный payload/размер |
| E10 | private SNI не выходит через DNS/cert-validation вне защищённого пути | plain bootstrap, AIA/OCSP, IP singleton anonymity set |

E09 — только проект будущего matched comparison, не текущий test/scan; никакие
цели/операторы не опрашивались. Pcap artifacts FOCI не являются нашими замерами.

## 10. Предлагаемая вставка в канон — только предложение

Тематическая связь: ECH anti-knowledge и observability bootstrap.

> ECH определён RFC9849 (March2026, Proposed Standard), DNS-bootstrap —
> RFC9848, Service Binding — RFC9460. Real-ECH proposal, GREASE-extension,
> acceptance и useful/protected connection — разные состояния. После успешного
> resolution с ech на всех endpoints generic non-SVCB fallback запрещён;
> unauthenticated timeout не разрешает отключить ECH. FOCI2025 устанавливает
> для московского AS50867 пути марта 2025 составное условие Cloudflare-фильтра
> (extension + outer SNI + destination Cloudflare range), а не current nationwide
> ECH-block. Current Windows WS/CDN renderer не задаёт ech; его chrome-profile
> содержит GREASE. uTLS adapter state не переносит ECHAccepted, поэтому false
> без capability/projection metadata не является negative privacy evidence.

<a id="11-проверка-и-предел-сдачи"></a>
## 11. Проверка и пределы выводов

SOURCE_REVIEW ограничен перечисленными captures. Два immutable remote source
файла sing-box совпали с local cache SHA-256; это не attestation module closure,
сборки, EXE/AAR или сети. Финальная адресная drift/document-сверка и время
зафиксированы в собственном STATUS и artifacts manifest. Peer/lead review
отдельны от авторской самопроверки; **ACCEPTED не присваивается**.

## 12. FUP-1-01 — связь выбранного ECHConfig с endpoint

**R02-013-D — ограниченный source-результат, PARTIAL_AT_CUTOFF.**
Capture: **2026-10-27T14:49:39.978416Z**. Раздел сохраняет уже прочитанный
материал и явно незавершённый объём; последующей source acquisition или
current-source проверки не было. A/B/C этим дополнением не пересматриваются.

### 12.1 Короткий ответ

**В рассмотренной цепочке автоматическая привязка ECHConfig к выбранному
по HTTPS/SVCB service endpoint не установлена; source показывает другую
последовательность: сначала dial к переданному destination, затем HTTPS-query
и извлечение первого `ech` из DNS Answer на уже открытом `net.Conn`.**

Нижний DNSRouter возвращает DNS-сообщение, а не согласованную пару
«выбранный ServiceMode RR + endpoint». Это не означает ошибку любой DNSRouter
функции: API Exchange здесь выполняет обмен DNS-сообщениями, а ответственность
за выбор service endpoint должна быть у вызывающего клиента. Развёртывание
с одним согласованным ключом на всех возможных endpoint может работать без
универсального Service Binding selector; это отдельная предпосылка.

### 12.2 Необходимый closure — что именно передаётся

Пины остаются sing-box **v1.13.19**, sing **v0.8.13**. Главные новые inputs
сохранены в `fup-1-01/inputs.json` (14 файлов). Номер строки ниже относится к
прочитанным source-версиям, не к нынешнему runtime.

| Слой | Прочитанное основание | Значение для binding |
|---|---|---|
| Конкретная реализация интерфейса | `box.go:178–187` создаёт `dns.NewRouter` и регистрирует его как `adapter.DNSRouter` | Здесь не неизвестный абстрактный resolver: основной Box использует рассмотренный Router |
| API | `adapter/dns.go:17–39`: Exchange возвращает `*dns.Msg`; Lookup — `[]netip.Addr`; DNSQueryOptions не содержит выбранного service endpoint | API не возвращает совместный RR/TargetName/port/ECHConfig результат |
| Адресный caller | `common/dialer/resolve.go:94–111`: domain → Router.Lookup → DialParallel/DialSerial; literal → внутренний dialer | IP выбирается по адресному lookup, не по ECH HTTPS-result |
| Адресный lookup | `dns/client.go:336–378,422–448` делает A/AAAA и возвращает адреса | Это не HTTPS ServiceMode selection; port не получен из этих ответов |
| Конечный socket caller | `common/dialer/default.go:246–270` требует разрешённый адрес, передаёт его TCP/UDP dialer | Устанавливается запрошенный endpoint. Его реальная достижимость здесь не измерялась |
| TLS/WS caller | `transport/v2raywebsocket/client.go:36–42,76–93`, `common/tls/client.go:117–122` | `DialContext(destination)` возвращает conn **раньше**, чем вызывается `aTLS.ClientHandshake` и ECH-query |
| ECH query | `common/tls/ech.go:130–150`: query_server_name либо TLS ServerName, Fqdn, QTYPE=HTTPS | Автоматического формирования port-prefix из destination.Port в этой функции нет; query override может быть явно задан потребителем |
| Router.Exchange | `dns/router.go:210–324`: правила/transport, Client.Exchange, reverse mapping A/AAAA, return response | Выбирается DNS transport/действие, а не сервисный HTTPS endpoint |
| DNS Client.Exchange | `dns/client.go:121–334`: cache либо transport.Exchange, padding/hints/TTL processing, return response | Сообщение не обязательно побайтно неизменно: TTL и hints могут меняться. Но это не сортировка ServiceMode по priority и не выбор TargetName/port |
| Конкретные wire transports | DoH `dns/transport/https.go:177–226`: Pack/HTTP/Unpack; DoT `dns/transport/tls.go:103–145`: WriteMessage/ReadMessage | Возвращается разобранный DNS Msg. Наличие transport не создаёт application-level service selection |
| ECH consumer | `common/tls/ech.go:157–179`: первый `*HTTPS` с ech в Answer, SetECHConfigList, затем Client(conn) | На этом шаге selected config применяется к **прежнему conn**; TargetName/port из RR не передаются обратно dialer |

Указание `DNSRouter` в Box и путь DoH/DoT — source analysis библиотеки.
Это **не** утверждение, что current Snowden ECH bootstrap реально использует
DoH: renderer `render.go:570–587` задаёт свои bootstrap/tunnel DNS rules,
а current VLESS+WS TLS branch по §5 вообще не включает ECH.

### 12.3 CNAME, AliasMode и ServiceMode нельзя считать одним механизмом

- Обычный recursive resolver может вернуть в Answer CNAME-цепочку и конечный
  HTTPS RR. Wrapper тогда способен найти `ech` среди возвращённых HTTPS.
  Из этого не следует, что сам wrapper прошёл цепочку и проверил её владельцев.
- RFC9460 §2.4.2 требует отдельно следовать HTTPS AliasMode, игнорируя его
  SvcParams; AliasMode не является CNAME для A/AAAA. Рассмотренный ECH wrapper
  сам не делает повторный query к AliasMode TargetName.
- Для холодного ECH cache ответ только с AliasMode и без terminal HTTPS ech
  приводит в этом wrapper к `no ECH config found in DNS records`. Это
  условная source-ветка, не результат реального DNS-запроса. SVCB-aware
  upstream может прислать дополнительные records, но wrapper читает только
  Answer, не Additional как готовый service plan.
- RFC9460 §§2.4.1/2.4.3/3/8: RRset не упорядочен порядком передачи; нужны
  совместимость mandatory parameters, режим записи и приоритеты. Wrapper
  выбирает первый ech без такого отбора. Это не доказывает, что любой
  возвращённый DNS Answer некорректен: он может быть правильным DNS-ответом,
  которому ещё требуется application interpretation.
- `TargetName="."` в ServiceMode означает owner name этой записи
  (RFC9460 §2.5.2); это не универсальный синоним исходного origin.
  Проверка TLS origin identity остаётся по исходному service name.

### 12.4 Бумажная иллюстрация уже прослеженного порядка вызовов

Это не DNS fixture, не новый профиль и не исполнение. Пусть caller без detour
уже соединился с endpoint **A:443**, а HTTPS RRset для `origin.example`
содержит две совместимые записи:

| Порядок в Answer | SvcPriority | TargetName / port | ECHConfig |
|---|---|---|---|
| Первая | 2 | `b.example:8443` | B-config |
| Вторая | 1 | `a.example:443` | A-config |

A и B — условные разные серверы; у каждого свой config/private key.
Рассмотренные DNS transport/Router могут вернуть обе записи в таком порядке:
priority не задаёт wire order. Wrapper берёт **B-config**, но socket остаётся
**A:443**. В этой трассе source не устанавливает ни принадлежность адреса A
к TargetName записи B, ни совпадение port, ни владение A ключом B-config.

Так показано отсутствие **общей гарантии binding** у рассмотренной композиции,
а не гарантированная утечка или успешный exploit. Что произойдёт при TLS —
rejection, допустимый authenticated retry или ошибка — зависит от сертификатов,
ключей и поведения A. Эти предпосылки не измерены. Изменение порядка Answer
может поменять выбранный config без изменения уже выбранного endpoint.

**Сильнейший противоположный случай:** A и B действительно разделяют нужный
ECH key/config, либо все адреса выбранного TargetName имеют соответствующие
ключи/полномочия public_name для recovery. Это совместимо с RFC9848 §4;
соединение может успешно работать. Следовательно, «берётся первый ech» не
равно «все ECH-подключения заведомо ломаются».

### 12.5 Условие использования и обновления для потребителя

Из ранее прочитанных RFC9460 §§2.4/3/5.1/8/9.1 и RFC9848 §§3–5 следует
проверять не изолированный ECHConfig, а **согласованное решение**:

`origin + RR source/owner + selected ServiceMode + TargetName + port +
совместимый ALPN + адресный набор + ECHConfigList + scope/freshness`.

Два разных допустимых проектных контракта не надо смешивать:

1. **Service Binding consumer:** сам разрешает цепочку, отбирает совместимую
   запись и применяет её endpoint/port/ECH совместно. Если транспорт был открыт
   заранее, повторное использование возможно лишь при согласованности с
   выбранным RR (RFC9460 §5.1), не просто при живом TCP.
2. **Fixed endpoint с отдельно предоставленным ECHConfig:** endpoint задаёт
   конфигурация продукта; согласованность ключа/public_name с ним обеспечивает
   выбранный deployment/доставка. Такой путь не следует рекламировать как
   полную обработку произвольных SVCB/HTTPS ServiceMode-записей.

Для refresh нужна сохраняемая связь с источником решения, а не только
подмена key bytes. Рассмотренный ECHClientConfig хранит config/TTL/update time,
но не выбранную ServiceMode запись с endpoint/port. Срок cache сам по себе
не устанавливает правильное соответствие при multi-CDN или смене адреса.
В этой партии не доказаны все переходы refresh/retry после смены generation;
они остаются **открытыми** и не считаются проверенными.

### 12.6 Что закрыто и что осталось

- **Закрыто на source-уровне:** необходимый exchange/caller closure подтверждает
  возврат DNS Msg, а не service-endpoint tuple; транспорт для TLS открывается
  до ECH-query; wrapper применяет config к прежнему conn.
- **Не доказано:** binding произвольного ответа со многими endpoints;
  current deployed ECH failure, полная совместимость всех DNS transports,
  корректность всех cache/refresh/retry transitions и фактический endpoint
  после ОС/NAT/proxy обработки.
- Актуальность нынешних source hashes не проверена. Capture
  **14:49:39.978416Z** описывает только тогда сохранённые 14 inputs;
  прочитанные после этой capture части `sing/common/network/multi.go` и
  `dns/transport/tcp.go` не получили собственного нового byte-capture.
- Предыдущие hashes/drift 13:58Z не переносятся как доказательство актуальности
  новой closure. Для FUP после обнаружения cutoff выполнялась только упаковка
  собственных документов и сохранность прежних префиксов.

Статус D — source-результат, доступный для отдельной рецензии; объём FUP в целом
зафиксирован как **PARTIAL_AT_CUTOFF**. Новая эмпирика и полная приемка не
выдуманы ради формального «закрытия» вопроса.
