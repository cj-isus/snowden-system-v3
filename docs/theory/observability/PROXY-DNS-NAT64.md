<a id="proxy-dns-nat64-2026-10-28"></a>
# Корпоративный proxy и NAT64: следовать фактическим socket-hop

[Оглавление](../README.md) · [DNS/ECH](../transports/ECH-DNS-BOOTSTRAP.md) · [Решения review](../research/reviews/2026-10-28-DEEP-REVIEW.md)

**2026-10-28, DR27-13.** Public source: sing-box1.13.19
`b5ebaa1fc0f2b94256180b95468e73ef53caa27d`, sing0.8.13
`7c349dacf402256d3a7029746073b05d2ead584a`. Это описание capabilities/callers,
не подтверждение corporate access, пользовательских credentials или actual
Windows/Android wiring. Новая SVCB-процедура разобрана отдельно по ссылке выше.

## 1. Proxy address и tunnel target обслуживают разные участки

N — сеть клиента, P — explicit proxy, E — tunnel endpoint, I — inner TLS
identity, R — resolver. Для одного proxy:

| Объект | Где нужен подходящий путь | Кто делает адресный lookup |
|---|---|---|
| R | Клиент к фактическому DNS transport | Собственная bootstrap-процедура R |
| P | N→P | При имени обычно клиентская resolver-цепочка |
| Named E внутри CONNECT/SOCKS | Proxy side→E | Proxy side, если имя сохранено |
| Numeric E внутри CONNECT | Proxy side→этот address | Адресный lookup E не требуется |
| I | TLS внутри полученного tunnel | Reference identity сама не требует DNS lookup |
| SOCKS UDP relay B | Отдельный N→B UDP socket | Клиентская нижняя цепочка, если B именованный |

Клиент IPv6-only может открыть socket к P6, передать `CONNECT E4:port`, а P
открыть IPv4 socket к E4. Это не требует клиентского IPv4 socket к E.
Обратный случай: недостижимый P4 literal остановит путь, даже если E имеет IPv6.

В pinned HTTP `Client.DialContext` сначала dial к `serverAddr=P`, затем E
записывается в authority CONNECT. SOCKS5 кодирует address type E в request.
[HTTP](https://github.com/SagerNet/sing/blob/7c349dacf402256d3a7029746073b05d2ead584a/protocol/http/client.go),
[SOCKS](https://github.com/SagerNet/sing/blob/7c349dacf402256d3a7029746073b05d2ead584a/protocol/socks/client.go),
[RFC1928 §§3–6](https://www.rfc-editor.org/rfc/rfc1928.html#section-3).

## 2. Placement resolver может изменить сам CONNECT request

Обычный `dialer.New` с Detour без explicit DomainResolver сохраняет named E.
Если указать `DomainResolver.Server` над detour, `NewWithOptions` добавляет
resolve wrapper: `router.Lookup(E)→DialSerial(addresses)` передаёт дальше
numeric destination. Клиентский lookup добавляет зависимость и может изменить
применимость proxy ACL. Это не обязательно direct DNS: resolver transport
может иметь собственный detour.
[Constructor](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/common/dialer/dialer.go),
[resolver](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/common/dialer/resolve.go),
[numeric dial](https://github.com/SagerNet/sing/blob/7c349dacf402256d3a7029746073b05d2ead584a/common/network/multi.go).

NewDetour и NewDefault — разные ветки. Socket controls верхнего endpoint
не копируются автоматически в leaf dialer другого outbound. Protect/bind
принадлежат реальному нижнему socket; DNS transport имеет свои options.
Поэтому bind N→P сам по себе не задаёт DNS realm для разрешения P.
[Detour](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/common/dialer/detour.go),
[leaf controls](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/common/dialer/default.go),
[DNS dialer](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/dns/transport_dialer.go).

## 3. Translation относится к тому socket, который проходит через переводчик

NAT64 переводит IP/transport tuple, а не CONNECT authority, SOCKS DST.ADDR,
SNI или WS Host. Client Pref64 не обязан быть маршрутизируемым для P→E.
Глобальная замена E4 на client-side synthesized E6 способна сломать исправный
proxy path. [RFC6146](https://www.rfc-editor.org/rfc/rfc6146.html#section-3.6.1).

DNS64 обслуживает AAAA resolution; numeric literal не проходит такую процедуру.
CLAT может обслужить IPv4 sockets отдельно. Это разные механизмы.
[RFC6147 §§5.1/5.3.3](https://www.rfc-editor.org/rfc/rfc6147.html#section-5.1),
[RFC6877 §5.2](https://www.rfc-editor.org/rfc/rfc6877.html#section-5.2).

**Дополнительный SOCKS-hop:** после успешных TCP/auth/UDP ASSOCIATE клиент
открывает UDP socket к `response.Bind`. P6 может вернуть B4, недоступный N
без IPv4/CLAT/synthesis. TCP proof до P6 не доказывает этот UDP путь.
BND.ADDR ответа CONNECT не требует такого второго socket, а BND.ADDR ответа
UDP ASSOCIATE его задаёт; association также зависит от управляющего TCP.
[RFC1928 §§6–7](https://www.rfc-editor.org/rfc/rfc1928.html#section-6),
[actual client](https://github.com/SagerNet/sing/blob/7c349dacf402256d3a7029746073b05d2ead584a/protocol/socks/client.go).

Pref64 discovery не является reachability proof. RFC7050 запрещает использовать
адреса самого `ipv4only.arpa` для connectivity checks. Неудача DNS discovery
не исключает prefix, полученный через RA; RFC8781 задаёт scope/lifetime и
рекомендации выбора источника. Private corporate P требует подходящего
translation realm: well-known `64:ff9b::/96` не предназначен для non-global IPv4.
[RFC7050 §§3.2.1/5.1](https://www.rfc-editor.org/rfc/rfc7050.html#section-3.2.1),
[RFC8781 §5.1](https://www.rfc-editor.org/rfc/rfc8781.html#section-5.1),
[RFC6052 §3.1](https://www.rfc-editor.org/rfc/rfc6052.html#section-3.1).

## 4. Общий auth/interop cut не исправляется сменой внутреннего транспорта

Pinned HTTP connector имеет preemptive Basic и произвольные headers, но
не полноценный NTLM/Negotiate challenge loop. При407 caller получает
обобщённое `authentication required`. По RFC407 возможны отсутствующие,
неполные или неверные credentials; из этой строки нельзя определить причину.
[RFC9110 §11.7](https://www.rfc-editor.org/rfc/rfc9110.html#section-11.7).

**Конкретное source-расхождение:** HTTP client допускает только status200,
тогда как [RFC9110 §9.3.6](https://www.rfc-editor.org/rfc/rfc9110.html#section-9.3.6)
переводит любой успешный2xx CONNECT в tunnel mode. Иной допустимый2xx будет
отвергнут этим pin до inner handshake. Частота таких ответов и применимость
к текущей поставке неизвестны.

Порядок HTTP-цепочки: `socket P→optional TLS(P)→proxy auth/CONNECT→TLS(I)→inner`.
Inner TLS не защищает уже состоявшийся proxy-auth exchange. Ordinary CONNECT
не реализует CONNECT-UDP; HY2 открывает outer UDP даже для прикладного TCP,
а VLESS может нести свой inner UDP поверх TCP. Совместимость резервов нужно
проверять на carrier и каждом socket-hop, не только на типе задачи пользователя.

## 5. Handoff и finite cases

Адресаты TH-02/03/07/08, COM и platform owners. Reuse существующих dialers,
resolver realms, auth-capability reporting и owner cancellation. Согласование
corporate policy/identity остаётся условием сценария; новый auth stack или
изменение маршрутов в этом исследовании не реализованы.

Cases **NOT_RUN**: P-name/P-literal при v6-only с/без CLAT; E-name/E4 внутри
CONNECT; explicit resolver над detour; stale Pref64 другого realm; P6/B4 UDP
relay; unsupported/incomplete auth;200/иной2xx; TLS(P)/TLS(I) раздельные отказы;
Stop во время handshake; общий P/NAT64 outage у двух E; successful small
handshake при failed useful task. При неизвестной причине сохраняется unknown,
а не «неверный пароль», DPI или независимость двух внешних адресов.
