# DNS, наблюдения и диагностика

[Оглавление теории](../README.md).

Углубление 2026-10-28: [что доказывает probe](PROBE-PROOF-BOUNDARIES.md),
[proxy/DNS/NAT64 и фактический socket-hop](PROXY-DNS-NAT64.md),
[quality/time/coverage корпуса](../methodology/MEASUREMENT-CORPUS-AND-COVERAGE.md).

Разделы перенесены из авторского сборника с сохранением исходной нумерации, дат и границ доказательства. Исторический статус не является текущим product PASS.

- [§13.5 — 13.5. DNS, адреса и кэши — ускорение с контекстом](#source-13-5)
- [§16 — 16. Новые исходники: достоверность наблюдений, журнал и portable trust](#source-16)
- [§16.1 — 16.1. OP-01 — успешный DNS ответ и свежесть имеют разные значения](#source-16-1)
- [§16.2 — 16.2. OP-02 — bootstrap resolver всё ещё отдельный путь](#source-16-2)
- [§16.4 — 16.4. OP-04 — журнал контроллера не полный аудит VPN](#source-16-4)
- [§16.5 — 16.5. OP-05 — privacy до записи и bounded support export](#source-16-5)
- [Дополнение 2026-10-27: SVCB/HTTPS, proxy и NAT64](#service-binding-observations-2026-10-27)

---

<!-- source-unit: 13.5 -->
<a id="source-13-5"></a>
### 13.5. DNS, адреса и кэши — ускорение с контекстом

Happy Eyeballs полезен для частично сломанного IPv6/IPv4: ограниченные пересекающиеся
попытки к **разрешённым** адресам с освобождением проигравших sockets. Он не означает
соревнование «VPN против direct» и не подтверждает работоспособность приложения.
Resolver/transport race должны учитывать физическую сеть, NAT64/DNS64, policy,
credential/trust generation и единый dial budget. Успешный TCP до gateway не
авторизует новый profile и не заменяет authenticated/protected probe. [D09]

Ключ cache/coalescing — не один hostname: нужны query type, resolver/realm,
путь/назначение bootstrap-vs-protected, policy generation и необходимые privacy
options. В M09 одинаковое имя из work/travel realms даёт разные ответы.
Отдельные кэши transport в sing-box помогают разделять DNS servers, но не доказывают
изоляцию всех наших scopes. Смена namespace инвалидирует прежние ответы; DNS server
order может быть значимым, route/address set order — обычно нет.

Negative caching NXDOMAIN/NODATA отличается от SERVFAIL/timeout. TTL/refresh/backoff
не должны закреплять transient failure на весь день и вызывать query storm после
истечения. Более новый RFC9520 обновляет RFC2308: resolution failures, включая
DNSSEC validation failures, тоже требуют bounded failure caching; рекомендация
«никогда не кэшировать SERVFAIL» была бы ошибкой. Это не право выдавать timeout
за NXDOMAIN или переносить failure cache между resolver realms. [D29]
Область RFC — кэширующий/валидирующий DNS resolver; retry-cache внешнего probe или
клиент без DNSSEC validation не становится таким resolver по одному таймеру.
Применять требования через фактический DNS transport/cache API и его semantics.
Serve-stale из RFC8767 повышает доступность recursive DNS при отказе
authoritative, но не разрешает бессрочное `disable_expire`, reuse чужого network realm,
отключение проверки сертификата или принятие отозванного ключа. DNS TTL и trust expiry
имеют разные полномочия. Новый gateway IP получает проверку обычного имени/пина. [D10][D11]

`x/sync/singleflight` уже в module graph: готовое подавление одинаковых in-flight
вычислений, BSD-3-Clause. Оно не ограничивает количество **разных** ключей, не имеет
автоматической отмены leader по timeout waiter; `Forget` допускает новую работу до
завершения старой. Нужны существующие admission/owned result/context fences и
bounded keys; использовать один экземпляр на правильный namespace.

TLS resumption может сократить startup; caches ticket/token привязаны к authority/
policy и ограничены по жизни/памяти. 0-RTT допускает replay: trust mutation, выдача
credential, платёж/отзыв/одноразовый Apply не отправляются как replay-safe early data.
WS `max_early_data` в renderer — другой механизм, его нельзя называть TLS0-RTT.
Pool reuse не продлевает срок сертификата/профиля и не переживает urgent revoke. [D12]


<!-- /source-unit: 13.5 -->

<!-- source-unit: 16 -->
<a id="source-16"></a>
<a id="observation-proof-research"></a>
## 16. Новые исходники: достоверность наблюдений, журнал и portable trust

2026-10-24, продолжение по новому запросу владельца после §15. Основание для
переоткрытия: PC V2-144/145 — отдельное подтверждение первого policy baseline и
резервные wire DoH probes; Android AND-08-S1/AND-12-S1 — app/Compose и журнал,
а также уточнённый контракт portable verifier. Старое завершение desk research
остаётся датированным результатом, а новые выводы привязаны к этим изменениям.

OP-01…06 ниже — labels уточнений существующих TH/RQ/COM/AND, не ещё один backlog.
Source reads и стандарты не доказывают реальные запросы, SDK/crypto, leak/скорость,
устройства или удобство. Артефакты/снимки/проверка:
evidence — `docs/evidence/2026-10-24-observation-proof.md`.


<!-- /source-unit: 16 -->

<!-- source-unit: 16.1 -->
<a id="source-16-1"></a>
### 16.1. OP-01 — успешный DNS ответ и свежесть имеют разные значения

PC ProtectedProbe теперь проверяет bounded wire A-answer через HTTPS/SOCKS и
последовательно пробует до3 разрешённых измерителей. Успех относится к одному
DoH exchange, а не к системному DNS, рекурсии до authoritative server сейчас или
общей доступности имён. Query `example.com.` не содержит OPT; отсутствие EDE в
таком ответе не свидетельствует об отсутствии stale/filtering/recovery.

**Новый вывод из стандартов:** HTTP cache age и DNS TTL связаны; `no-cache` не
гарантирует новый authoritative lookup. Serve-stale может законно вернуть старую
запись во время отказа authoritative. EDE может сопровождать даже NOERROR; код
остаётся диагностикой resolver, не доказанным механизмом блокировки. DoH защищает
транзакцию до выбранного resolver, но не делает его утверждения независимой
истиной. EDE EXTRA-TEXT не предназначен для machine classification и может
содержать приватные сведения.

**Решение: adapt существующий typed observation.** Сейчас не добавлять DNSSEC
validator, random per-query name или обязательный EDE. Если нужна именно
проверка cache freshness — отдельный согласованный controlled authoritative
endpoint и бюджет; не приписывать её нынешнему probe. При будущем DNS caching
учитывать Age/TTL; если ответ используется только как observation, отсутствие
полей Age/EDE не объявлять найденной ошибкой кеширования продукта.

**Gate TH-02/RQ-07/08/14, AND-07:** HTTP200+cached/stale DNS, NOERROR+EDE,
unknown/multiple EDE, no OPT, SERVFAIL/negative cache, unavailable authoritative
при живом resolver. Проверять scope/freshness labels и отсутствие ложного
quarantine; не запускать новые authoritative queries по умолчанию.
Первичка: [RFC8484 §5.1](https://www.rfc-editor.org/rfc/rfc8484.html#section-5.1),
[RFC8767](https://www.rfc-editor.org/rfc/rfc8767.html),
[RFC8914 §§2/3/6](https://www.rfc-editor.org/rfc/rfc8914.html).


<!-- /source-unit: 16.1 -->

<!-- source-unit: 16.2 -->
<a id="source-16-2"></a>
### 16.2. OP-02 — bootstrap resolver всё ещё отдельный путь

V2-145 перевёл **protected measurement**, но не весь DNS проекта на wire DoH.
В scoped source `corebridge.go` создаёт PreflightResolver для enabled VLESS+WS
hostname до рендера; тот сохраняет UDP стратегии, provider JSON APIs и disk cache
на7 дней. Caller использует IP override, при неудаче продолжает hostname dial.
Это реально найденный call site, но его исполнение/правила WFP здесь не проверены.
Нельзя переносить свойства SOCKS/no-redirect/no-env-proxy protected fetch на этот
`http.Client` с default transport. Фактический proxy/redirect зависит от среды.

В cache entry есть `host/IP/At`; cacheGet проверяет age только сверху и ParseIP.
Следовательно, future At не отвергается самим этим условием; нет явной связи с
network/profile/resolver epoch. Это source gap срока/namespace, а не доказанный
эксплойт или direct leak. Cache TTL7d — локальная retention policy, не DNS RR TTL.
Plain bootstrap lookup может раскрыть gateway hostname; TLS identity сохраняет
другую границу доверия, но не отменяет privacy/availability стоимость lookup.

`publicProbeIPv4 = To4 && IsGlobalUnicast && !IsPrivate` не является полной
IANA globally-reachable policy. Shared100.64/10, documentation и benchmarking
имеют особые назначения; в IANA встречаются и globally-reachable специальные
исключения. Не запрещать всё special-purpose или весь192.0.0/24 одним правилом.
Кроме того, parseable response-address и допустимый dial destination — разные
роли; protected DNS probe не соединяется с каждым возвращённым A.

**Решение: adapt существующий PreflightResolver, не создавать третий resolver.**
Разделить authorized gateway bootstrap, user DNS и measurement. Для bootstrap
требуются owned deadlines/admission, bounded cache read, clock discontinuity/
future timestamp rejection, network/profile/identity provenance, explicit
transport/redirect/proxy policy. IP hint не расширяет подписанные hostname/pin/
destination полномочия. Роль-зависимая address policy получает versioned IANA
snapshot с exact-prefix exception precedence; не обещает actual routability.

**Gate TH-03/RQ-08/09/12/15, COM-07/15, AND-04/05:** poisoned/future/oversized
cache, смена profile/network/host/pins, proxy env/redirect, DNS64/IPv4 literal,
special-purpose samples и stale hint→fresh TLS/owner/protected proof. До измерений
не называть источники независимыми или гарантированно доступными из РФ.
[Go1.26 IP predicate](https://pkg.go.dev/net@go1.26.0#IP.IsGlobalUnicast),
[IANA registry](https://www.iana.org/assignments/iana-ipv4-special-registry/).


<!-- /source-unit: 16.2 -->

<!-- source-unit: 16.4 -->
<a id="source-16-4"></a>
### 16.4. OP-04 — журнал контроллера не полный аудит VPN

Android SessionJournal уже ограничивает ring128, typed enums, process-local
sequence, parent registration wall/elapsed; дедуплицирует state pairs, сообщает
evictions/exhaustion и не пишет raw config/UID/IP/exception/disk. Это подходящий
existing reuse, а не повод добавлять logging SDK/daemon. Command admission не
равен выполнению команды; state publication не kernel/worker/egress receipt.

**Adapt UI contract:** показывать scope parent/process, границу retained window,
evicted/recordingLimited и отсутствие durable history. Clear не отменяет VPN,
trust floors или уже удержанные snapshots; process restart не доказательство
отсутствия прежней сессии. StateFlow snapshot сохраняет текущую выборку, а не
обязан доставить каждому медленному observer каждое intermediate состояние.
Если потребуется полный forensic audit, это отдельный threat/storage contract,
не расширение retention slider без пересмотра privacy и ресурсов.

**Gate COM-07/12, AND-11/12:** burst>128, slow collector, Clear/rotate/process
death/worker adoption, repeated state, clock jump и held old snapshot; UI не
рисует отсутствующее событие/длительность/полный outage history. Проверять lock
tail/RSS/copies под нагрузкой:128 refs — source cap, не измеренная memory SLO.


<!-- /source-unit: 16.4 -->

<!-- source-unit: 16.5 -->
<a id="source-16-5"></a>
### 16.5. OP-05 — privacy до записи и bounded support export

Новые typed observations/journal не означают, что всё приложение перестало
собирать адреса: legacy preflight всё ещё вызывает Logf→App.appendLog с host/IP/
ошибкой. Нужно inventory producers/collectors всех уровней, включая Go/libbox,
HTTP/provider errors, Binder/UI, crash reports и logcat. Здесь прочитан только
исходный поток; реальные пользовательские логи не открывались.

**Adapt allowlist на producer boundary:** typed stage/code/counters/scope/age,
без raw headers/body/URI/token/profile/cert/UID/app inventory. Regex после записи
не удаляет прежнюю копию. Для support export — explicit user action, immutable
review snapshot, видимый состав/объём/time window/dropped count; preview и share
одних bytes. По умолчанию локально, без automatic upload. Нет необходимости
хешировать low-entropy hostname как «анонимный» stable identifier.

Очистка видимой истории, удаление persisted export и cryptographic erasure —
разные операции. Установить честный контракт GC/старых snapshots/выданных URI,
не обещать уничтожение уже отправленного файла. Bounded ring/bytes/queue/format
encoding; logging failure не должен давать direct или останавливать lifecycle.
Версионная schema + synthetic canaries для запретных полей, CR/LF/oversize и
raw exception path входят в будущую разрешённую проверку, не в реальные secrets.

**Gate COM-07/12/13/15, TH-08, AND-08/11/12/14/A-FR-17:** source→UI→export/logcat
inventory, slow storage/share failure/Cancel, old snapshot/Clear и отсутствие
secret fields во всех output sinks. Crash/third-party SDK — отдельный binary
inventory. [OWASP Logging](https://cheatsheetseries.owasp.org/cheatsheets/Logging_Cheat_Sheet.html),
[Android log disclosure](https://developer.android.com/privacy-and-security/risks/log-info-disclosure).


<!-- /source-unit: 16.5 -->

<a id="service-binding-observations-2026-10-27"></a>
## SVCB/HTTPS: дополнительный lookup меняет область наблюдения

2026-10-27. Нормы **RFC 9460**, не утверждение об уже реализованной поддержке.
Полная связка ECH и fallback разобрана в
[отдельной тематической заметке](../transports/ECH-DNS-BOOTSTRAP.md).

1. **A-answer не проверяет HTTPS RR.** HTTP-клиент может получить адрес, но не
   параметры нужного ServiceMode endpoint. Query type, исходный service name,
   TargetName и выбранная запись имеют разные роли. Неудачу HTTPS lookup нельзя
   заменять успешным A lookup в отчёте о ECH bootstrap. Это следствие
   [RFC 9460 §§2–3, 9](https://www.rfc-editor.org/rfc/rfc9460.html#section-3),
   не причина добавлять этот lookup в каждую mandatory VPN-пробу.
2. **Proxy не автоматически покрывает новый DNS-запрос.** При named HTTP CONNECT
   или SOCKS клиент может не делать локальных A/AAAA. Самостоятельный SVCB lookup
   способен раскрыть имя дополнительной стороне. §3.2 требует подходящих privacy
   свойств отдельной процедуры; если их обеспечить нельзя, SVCB-optional клиент
   **MUST** отключить SVCB resolution, SVCB-reliant считает конфигурацию невалидной.
   Это не разрешение direct DNS fallback. При использовании SVCB с named
   destination клиент **MUST** передать прокси финальные TargetName и port;
   прежнее имя для TLS-аутентификации при этом не подменяется.
   [RFC 9460 §3.2](https://www.rfc-editor.org/rfc/rfc9460.html#section-3.2).
3. **Address hints не заменяют обычную адресную процедуру.** При уже доступных
   A/AAAA для TargetName клиенту **SHOULD** игнорировать `ipv4hint/ipv6hint`;
   иначе выполнить соответствующий lookup и использовать его адреса для будущих
   соединений. Сам hint не является проверкой достижимости или новым trust pin.
   [RFC 9460 §7.3](https://www.rfc-editor.org/rfc/rfc9460.html#section-7.3).
4. **Resolver DNS64 и NAT64-клиент имеют разные полномочия.** DNS64 resolver
   **MUST NOT** синтезировать адреса внутри SVCB IP hints: это нарушило бы DNSSEC
   записи. Но NAT64-клиент при одном `ipv4hint` может синтезировать адрес локально
   по RFC 7050 либо дождаться AAAA. Запрещена мутация подписанной DNS-записи,
   а не любая локальная трансляция адреса.
   [§4.2.1](https://www.rfc-editor.org/rfc/rfc9460.html#section-4.2.1),
   [§7.3](https://www.rfc-editor.org/rfc/rfc9460.html#section-7.3).

Предложение для **TH-02/03, E-06**: при отдельном ECH/SVCB срезе фиксировать
record type, bootstrap/protected/proxy realm, DNS outcome, выбранный endpoint
и TLS authentication scope. Сохранять producer allowlist: реальные имена,
полные RRsets и пользовательские запросы не становятся обязательными log fields.
Legacy typed A-probe остаётся свидетельством своей узкой проверки, не ошибкой
только из-за отсутствия в нём HTTPS RR.

Future cases: A успешен/HTTPS timeout; private named proxy без безопасного
SVCB resolver; DNS64 Additional AAAA против неизменённого hint; stale hint и
новый адрес; смена realm во время lookup. **NOT_RUN**. API, native routing,
ресурсы и фактическая ECH-поддержка Windows/Android здесь не установлены.
