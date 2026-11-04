# R28-Z01 — endpoint-bound ECH bootstrap на реальном caller

28 октября 2026. **Статус автора:** READY_FOR_REVIEW; без исполнения протокола и продукта.

## 1. Вопрос и короткий ответ

**Вопрос:** где источник/клиент связывает ECHConfig с service identity,
выбранным адресом и retry/fallback, и какой counterexample переживает guards?

**Короткий ответ (R28-Z01-A):** RFC 9460/9848 связывают `ech` с **одной
выбранной ServiceMode-записью** (`TargetName`, `port`, `alpn`, address set и
прочие SvcParams); TLS затем связывает inner identity с исходным service name,
а rejected outer — с `ECHConfig.public_name`. В actual future-ECH wrapper
sing-box v1.13.19 эта полная попытка не материализована: socket открывается по
отдельному `destination`, `ECHClientConfig` позднее запрашивает HTTPS RR и
берёт первый `ech` из Answer, сохраняя только config/TTL/time. Endpoint/port,
RR/source identity, address pool и network epoch рядом с config не сохраняются.

**Сильнейший counterexample, переживающий существующие guards:** после смены
сети DNSRouter очищает A/AAAA/HTTPS cache и закрывает соединения, но отдельный
ECH cache в TLS wrapper остаётся до TTL; новая попытка может получить новый
адрес пула B и применить старый config A. Если B продолжает иметь старый
ECH private key и сертификат исходного origin (обычный overlap/rollout), HPKE,
ECH acceptance, origin certificate, CertificateVerify и Finished **все могут
успешно пройти**. Поэтому guards не доказывают, что attempt построен из одного
ServiceMode source/network epoch. Это не plaintext leak и не auth bypass;
это разрыв provenance/freshness, позволяющий stale/mixed plan выглядеть
полностью успешным. Отдельно, если ключ больше не поддерживается, guards дают
ошибку/retry, но не восстанавливают endpoint provenance.

Текущий Windows renderer ECH не включает; вывод относится к готовому future
consumer/library path и не является дефектом работающего канала.

## 2. Входы, пины и actual composition

Повторно прочитан manifest `windows/go.mod`: Go 1.26, sing-box **v1.13.19**,
uTLS **v1.8.4**, sing **v0.8.13**, miekg/dns **v1.1.72**. Module cache
доступен. Windows production tags включают `with_utls`; EXE/AAR attestation
и execution NOT_RUN.

Actual product chain, source-only:

1. `corebridge.renderAndPrepareContext` создаёт `PreflightResolver`, для VLESS+WS
   сохраняет первый допустимый IPv4 override и передаёт его renderer.
2. `PreflightResolver` параллельно спрашивает несколько plain-UDP resolver и
   provider-specific JSON DoH `type=A`; первый успешный IP побеждает. Если
   все стратегии не дали результат, используется host→IP cache с TTL **7 дней**.
3. Renderer превращает этот IP в `outbound.server`, но сохраняет origin hostname
   как TLS `server_name` и WS Host; текущий branch задаёт uTLS/chrome и **не
   задаёт `ech`**.
4. Другая DNS subsystem внутри sing-box имеет bootstrap rule для всех tunnel
   hostnames через direct UDP `1.1.1.1`; основной tunnel DNS идёт через proxy.
   Если в будущем к TLS branch добавить implicit ECH query, HTTPS RR этого
   tunnel hostname попадёт под отдельный bootstrap-dns route, не под тот resolver,
   который выбрал preflight IP.
5. `box.New` создаёт/регистрирует `dns.Router`; VLESS outbound создаёт dialer,
   TLS config и WS transport. WS `dialContext` вызывает TLS dialer с уже
   выбранным `serverAddr`; `defaultDialer` сначала делает TCP dial, потом
   `ClientHandshake`.
6. Лишь в `ECHClientConfig.ClientHandshake` выполняется HTTPS query; wrapper
   берёт первый `ech` из Answer и применяет список к уже открытому conn.

Таким образом, даже корректные по отдельности A/AAAA и HTTPS ответы не являются
одним согласованным Service Binding решением. В текущем продукте это пока
не активируется (ECH отсутствует), но именно этот seam должен быть адаптирован,
а не объявлен готовым по наличию поля `ech`.

## 3. Нормативная модель одной попытки

### 3.1 Immutable attempt plan

Одна ECH-попытка должна владеть согласованным plan:

`service identity O + config source S + selected ServiceMode record R +
TargetName T + port + compatible transport/ALPN + address candidates A4/A6 +
ECHConfigList E + public_name P + freshness/network epoch G + retry budget`.

Основания:

- RFC 9460 §§2.4/3: ServiceMode связывает TargetName и SvcParams; сортировка
  по SvcPriority, обязательная совместимость mandatory keys, A/AAAA относятся
  к выбранному TargetName. RRset wire order не является priority selection.
- RFC 9460 §5.1: optimistic connection допустим, но ClientHello нельзя послать,
  пока не известно, что соединение **consistent** с выбранной записью.
- RFC 9848 §3: `ech` конфигурация относится к alternative endpoint; §4 требует,
  чтобы все его TargetName addresses имели key либо были authoritative для P;
  §5.3 запрещает ClientHello до завершения SVCB resolution.
- RFC 9849 §§6.1.6–6.1.8: retry config можно переносить только в новый transport,
  consistent с предыдущей конфигурацией; адреса нового DNS query могут относиться
  к другому ECH server/public_name; persisted state связывается с source S.

Это не требует, чтобы resolver сам возвращал новый сложный API: selector может
быть отдельным pure-layer поверх стандартного DNS Msg. Но consumer обязан
сохранить выбранный RR и endpoint вместе, а не передавать TLS лишь `[]byte E`.

### 3.2 Transition matrix

| Переход | Обязательные guards | Что разрешено | Что остаётся unknown |
|---|---|---|---|
| DNS response → selected plan | CNAME/Alias loop limit; mode; mandatory/self-consistency; priority; ALPN/transport; port; T/A/E из одного R; trust/freshness | Создать immutable plan и, при нужном privacy policy, разрешить preconnect без ClientHello | Reachability и censorship |
| plan → TCP/QUIC socket | Address принадлежит plan.A либо preconnect совместим с R | Открыть transport в общем deadline | TLS/ECH acceptance |
| socket → real ECH ClientHello | E поддержан; T/port/ALPN/socket consistent; origin O не заменён T/P | Отправить inner O, outer P | Application/VPN success |
| ECH accepted | acceptance confirmation + origin O certificate/signature/Finished | Дальше application protocol по negotiated ALPN | Protected egress/task success |
| ECH rejected | outer certificate должен проверять P, no app data/client cert/ticket adoption | Typed rejection result; новые configs доступны только в scope старого plan | Работает ли retry endpoint |
| retry with configs | Один bounded retry; config source/old address set/epoch preserved; новый DNS query не смешан молча | Новый transport consistent с previous plan | Fresh server rollout consistency |
| authenticated disable | Только после валидного outer P outcome по RFC/application policy | Новый transport without ECH, если профиль это разрешает | ECH-required policy может запретить |
| network/profile handover | Отменить attempt; новый G требует re-resolution либо explicit fixed-endpoint proof | Новый immutable plan | Late results старого G не применяются |

## 4. Guards: что они действительно останавливают

### 4.1 Сильный безопасный путь

При real-ECH acceptance uTLS проверяет 8-byte confirmation, переключает transcript
на inner ClientHello, затем проверяет server certificate, CertificateVerify и
Finished; application data не становится разрешённым из одного ECH bit.
Origin name хранится в config ServerName и восстанавливается при acceptance.
Если endpoint B имеет корректный ECH key и origin certificate, эти guards
справедливо допускают соединение независимо от физического IP.

Этот положительный случай важен: отсутствие generic SVCB selector не означает,
что fixed endpoint/shared-key deployment небезопасен. Его контракт просто иной:
endpoint и E доставлены одной внешней авторитетной конфигурацией, а не выбраны
из произвольного HTTPS RRset.

### 4.2 Mismatch без общего ключа

Если socket ведёт на B, а E только для A, сервер не расшифрует inner. Тогда:

- rejection branch должен аутентифицировать P, запрещает application data и
  client certificate, возвращает typed `ECHRejectionError`;
- malformed/no certificate/Finished failures останавливают попытку;
- core stdlib dialer делает не больше одного retry с nonempty RetryConfigList.

Это предотвращает silent application success с неаутентифицированным B, но
не показывает, что выбранный E принадлежал этому endpoint. Safety guard
работает через fail/recovery, provenance plan остаётся недоказанным.

### 4.3 uTLS public-name и retry границы

uTLS real-ECH outer SNI заменяется на P, но default rejection verifier в
`handshake_client.go` использует `config.ServerName`/explicit verify name.
SNI-extension guard при real ECH не переписывает config.ServerName внешним P.
При O≠P это отличается от RFC-required verification P:

- P-only certificate может быть ошибочно отвергнут, хотя recovery peer корректен;
- O-only certificate может удовлетворить name predicate O (при валидной chain/time),
  хотя это не доказательство public_name P.

Остальные CertificateVerify/Finished/error guards сохраняются, поэтому это не
готовый bypass claim. Более того, `client.go` retry recognizer ждёт stdlib
`*crypto/tls.ECHRejectionError`, а uTLS возвращает свой named type без найденного
`As/Unwrap`. Значит, исправить только type conversion недостаточно: сначала
нужно доказать корректный public-name verifier и сохранить source-bound plan.

### 4.4 Первый application write

WS transport выполняет TLS handshake до HTTP Upgrade. Если TLS возвращает error,
WS header/early-data не отправляются. Это сильный guard. После успешного TLS
WS early data может включать VLESS/user payload в HTTP header, поэтому permit
после TLS нельзя трактовать шире доказанных CDN/transport confidentiality rules.
Этот вопрос подтверждает paired C02 handoff и не расширяется здесь.

## 5. Counterexample, переживающий guards

### 5.1 Stale E across network handover

Начало: сеть G1, endpoint pool A, HTTPS RR даёт E_A с TTL. Wrapper хранит
`ECHConfigList=E_A,lastTTL,lastUpdate`, но не S/R/T/address set/network epoch.

Handover G1→G2:

1. `NetworkManager.ResetNetwork` закрывает connectionManager, уведомляет VLESS
   outbound (`WS.Close`, mux reset), затем `Router.ResetNetwork` очищает DNS cache
   и reset DNS transports.
2. ECHClientConfig не зарегистрирован как network listener и не имеет Clear/Reset;
   его `lastTTL/lastUpdate/E_A` остаются в outbound TLS object.
3. Новый address lookup в G2 выбирает B (иная geolocation/IPv4-vs-IPv6 pool).
4. Пока local E_A TTL не истёк, `fetchAndHandshake` не делает новый HTTPS query
   и использует E_A для нового socket B.
5. Если B всё ещё принимает E_A и имеет origin O certificate (overlap rollout,
   shared key), HPKE/acceptance/origin TLS/Finished проходят. Нет ошибки, которая
   могла бы сигнализировать смешение G1 source и G2 address pool.

Это переживает существующие криптографические guards именно потому, что каждый
component валиден, но совместное происхождение plan не доказано. Последствия:
не auth bypass, а нарушение требуемого source/epoch binding и потенциальный
tracking/retention scope из RFC 9849 §6.1.8/§10.3. Если B не принимает E_A,
guards дают failure/retry — availability symptom, не causal diagnosis.

### 5.2 Expired config + NOERROR without ech

После TTL wrapper выполняет HTTPS query, но перед query не очищает старый list.
При RcodeSuccess/Answer без нового `ech` цикл ничего не устанавливает, после чего
`len(ECHConfigList)>0`, и старый E используется. DNS cache может корректно
содержать новый NODATA-like state; wrapper всё равно сохраняет прежний key.
Transport error или non-success Rcode guard останавливают handshake — это
сильный opposing guard — но успешный ответ без `ech` проходит эту развилку.

### 5.3 Retry configs и адреса

После authenticated rejection stdlib branch подменяет только E bytes и повторяет
`dialContext(ctx,destination,false)`. Для domain destination новый resolveDialer
lookup может выбрать новый address set; previous plan/address source не хранится.
RFC 9849 §6.1.6 предупреждает, что адреса из нового DNS query могут относиться
к другому ECH server/public_name. Для literal endpoint адрес сохраняется —
положительный bounded случай. Current Snowden override literal уменьшает один
риск, но ECH branch сейчас не включён и сам IP мог прийти из отдельного 7-day
preflight cache.

## 6. Два независимых bootstrap-потока Snowden

В current composition VLESS+WS IP выбирает PreflightResolver: шесть параллельных
стратегий (plain UDP и JSON DoH), первый public IPv4, при полном отказе — 7-day
host→IP cache. Renderer сохраняет O для SNI/Host, server становится literal IP.

Sing-box DNS config отдельно направляет tunnel hostname в direct `bootstrap-dns`
(1.1.1.1) и остальной DNS через proxy. Future implicit ECH HTTPS query по O
попадёт в этот второй resolver path, а socket address уже выбран первым.
Ни hostname, ни TLS origin certificate не доказывают, что ответы двух resolvers
относятся к одному CDN pool/ServiceMode source. TLS origin verification защищает
O, но не E/T/port provenance. Это source composition risk, не утверждение, что
сейчас утечка существует: current renderer не задаёт ECH.Enabled.

## 7. Ready reuse, лицензии и цена

### 7.1 Reuse/adapt/defer

| Компонент | Решение | Причина / предел |
|---|---|---|
| miekg/dns v1.1.72 (`HTTPS/SVCB`, `SVCBECHConfig`, mandatory/alpn/port/hints) | **Reuse parser, adapt selector** | BSD-3-Clause; structs сохраняют связность RR, но library прямо возлагает semantic mandatory validation на consumer |
| sing-box DNSRouter/transports/cache | **Reuse exchange/transport, not service selection proof** | GPL-3.0-or-later + name restriction в root LICENSE; возвращают Msg/addresses, не endpoint-bound plan |
| sing-box TLS ECH wrapper | **Adapt only behind fixed-endpoint or new plan API** | Готовы config decode/TTL/handshake; нет source/endpoint/epoch state и reset |
| Go stdlib TLS ECH | **Prefer for normative rejection/error path when feasible** | BSD-style Go LICENSE; stdlib error/public-name semantics ближе к RFC, но fingerprint/reuse compatibility требует platform review |
| uTLS ECH | **Defer production real-ECH until public-name/retry parity proven** | BSD-3-Clause; fingerprint/GREASE полезны, но identified verifier/type/projection gaps требуют adaptation |
| Собственный HPKE/ECH/DNSSEC | **Reject** | Уже есть зрелые primitives; новый crypto/parser увеличит supply-chain/audit cost |

Paired C02 согласуется с Z01 по cache/handover и P-vs-O guard. Это независимое
source/model согласие, не два independent measurements и не научное принятие.

### 7.2 Минимальный API-контракт

Предложение platform owner, не реализация:

```
type ECHAttemptPlan struct {
  OriginName, RRSourceID, TargetName, PublicName string
  Port uint16
  ALPN []string
  Addresses []netip.Addr
  ECHConfigList []byte
  DNSExpiresAt time.Time
  NetworkEpoch, ProfileEpoch uint64
  ECHRequired bool
}
```

Selector получает DNS Msg и возвращает immutable plan либо typed
`NoCompatibleServiceBinding`; dialer принимает plan, а не независимые
`destination + []byte E`. Retry получает `previousPlan` и typed rejection result;
может заменить E, но не source/address/network epoch молча. Network handover
отменяет plan и очищает ECH cache; late результат старой generation не применим.
Fixed endpoint mode использует отдельный constructor с signed/owned config source,
чтобы не имитировать SVCB semantics.

Не обязателен именно новый public Go struct: эквивалентный immutable internal
object допустим. Требование — semantic ownership, а не имя API.

### 7.3 Стоимость

- хранение RR/source/epoch/address set и bounded alias/priority selection;
- синхронность preconnect: ClientHello ждёт завершения SVCB/ECH resolution;
- сброс cache на network/profile change и cancellation late results;
- отдельный retry transport и общий deadline/byte/retry budget;
- parity STD/uTLS для P verification, typed errors и ECHAccepted projection;
- multi-CDN/geolocation: больше DNS запросов и риск latency/energy/privacy;
- migration: current non-ECH profile не должен внезапно стать ECH-required.

Численные RAM/latency/energy costs не измерялись. Поле/профиль пользователя,
серверы и DNS не опрашивались.

## 8. Property/transition matrix и конечные NOT_RUN cases

| Case | Expected guard/outcome | Counterexample |
|---|---|---|
| one fixed endpoint + signed E + shared key | ECH accepted, origin O verified | Wrong key → typed rejection/error; fixed contract не SVCB claim |
| two ServiceMode RR with E_A/E_B | Select one compatible RR atomically | First-ech + independent address picks cross A/B |
| A/AAAA arrives before HTTPS | TCP preconnect only, no ClientHello until consistency | WS/TLS handshake before SVCB result leaks/fails privacy |
| IPv4 A key valid; IPv6 B key invalid | Family/address belong to same selected plan; fail B without causal censorship label | Generic shared E assumes both pools consistent |
| network handover before TTL | Cancel/clear plan/E; re-resolve under G2 | Existing DNS reset leaves E_A, new B accepted due key overlap |
| expired E; fresh NOERROR no ech | Clear E, follow profile fallback/required policy | Existing nonempty list survives and is reused |
| authenticated rejection P + retry E2 | Verify P, bind E2 to previous plan, one bounded retry | uTLS verifies O or stdlib type guard misses uTLS error |
| retry + fresh DNS changed public_name | Refuse implicit merge; build new plan | Current domain retry may choose new address with previous E2 |
| GREASE only | No real-ECH success/retry config adoption | Extension/HandshakeComplete reported as privacy success |
| successful TLS then WS/VLESS first write | Permit only named CDN/transport disclosure contract | Treat TLS success as end-to-end VPN confidentiality |

Все случаи NOT_RUN. Они конечны, не являются fixtures/tests и не запускают
crypto/network/simulation.

## 9. Scoped verdict и открытое

**Предлагаемое решение:** принять R28-Z01-A как source/model finding в области
пинов и caller'ов: generic implicit ECH wrapper **не доказывает endpoint-bound
Service Binding plan**; существующие TLS guards предотвращают silent
unauthenticated application success, но не доказывают общий source/network epoch.
Network reset и TTL не закрывают stale/mixed-plan counterexample.

Не принимать как установленное: deployed vulnerability, current ECH failure,
plaintext application leak, censorship, availability regression или whole-artifact
behavior. Current renderer real ECH не включает. Open: executed STD/uTLS parity,
actual server key overlap, final platform API shape, product policy ECH-required,
native cancellation and performance.
