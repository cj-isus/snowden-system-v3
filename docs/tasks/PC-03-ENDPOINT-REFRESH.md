# PC-03 — имя сервера, краткий seed и повторное разрешение адреса

2026-11-04, V2-210, Windows **source/unverified**. Часть полного
[PC-00…09](../planning/knowledge-application/PC-PLAN.md), после
[bootstrap/cache](PC-03-BOOTSTRAP.md) и [engine DNS](PC-03-ENGINE-DNS.md).
Тестирование, сборки и исполнение продукта запрещены владельцем.

## Дефект и границы

Root resolver сохранял TTL, но `prepareBootstrap` сводил результат к строке IP,
а renderer записывал этот IP в постоянный `outbound.Server`. Срок исчезал при
передаче; все новые подключения и rollback продолжали использовать тот же IP.
Одного возвращения domain + stock UDP недостаточно: в сети может работать только
один из существующих DoH способов. Это возможность по архитектуре, не результат
сетевой проверки и не доказательство независимости используемых провайдеров.

## Изменение существующих границ

1. Root передаёт original `PreflightAddress`, не теряя Host/ObservedAt/Until.
   Profile/operation/network/clock guards до передачи сохранены. В renderer
   `Server` остаётся исходным именем. TLS SNI прежний, WS Host — исходная
   authority (нестандартный порт включён). Содержимое signed profile не меняется.
2. Typed Config содержит до8 `bootstrap_hints` по exact outbound tag. Только
   protected domain-service с explicit bootstrap resolver может иметь hint;
   canonical public IPv4, исходное окно >0/<=5min. Истёкший hint в сохранённом
   rollback не делает сам конфиг невалидным, но не используется. Renderer
   исключает expired hints; worker повторяет freshness на использовании.
   Старые nonempty timeless override API теперь явно отвергаются до secrets.
3. Full runtime bytes, IPC subject и rollback сохраняют metadata. Общий
   `sdkRuntimeConfiguration` делает отдельный shallow SDK view без единственного
   поля hints перед обоими stock parsers. Config gate выполняется до SDK boundary;
   digest не вычисляется по урезанному SDK view. Изменение hint меняет subject.
4. Existing outbound registry декорирует только actual VLESS/HY2 constructor
   DNSRouter в клоне service registry. Проверяются actual Server и typed options,
   exact original bootstrap transport object/strategy/no-cache. Чужие запросы и
   user DNS используют original router; новых question-based DNS rules нет.
   Borrowed decorator не владеет Start/Close оригинального DNSRouter.
5. Original worker diagnostic и original native diagnostic flow получают
   private admission-pointer key. Только до открытия ingress и один раз можно
   использовать initial seed с исходным UTC interval + process steady ceiling.
   Обычный data ingress не получает key; expired/missing seed требует resolution.
   Это адресная подсказка до TLS/probe, не trust proof или permission.
6. Последующие actual service lookups повторно используют **тот же**
   `PreflightResolver`, wire codec и original network wrappers. Worker получает
   отдельный original per-box work domain, actual full runtime SHA namespace и
   service tag/type/name endpoint digest. Это не verified profile SHA или
   наблюдённый network epoch: worker epoch0/no CachePath, persistent cache нет.
   App resolver сохраняет прежний original process domain и cache contract.
   Selector-default rewrite синхронизирует source SHA только на existing closed/
   quiescent boundary до публикации новых raw bytes; active flight не переименуется.
7. Один active flight, до16 callers суммарно, включая followers уже завершённого
   flight. Одновременные запросы того же exact service scope делят active result;
   разные scopes/переполнение возвращают local busy. После flight кеша нет.
   Six-strategy I/O/4s timeout/10 connection slots/2 bodies остаются существующими.
   Cooperative overall flight deadline6s включает stock fallback; leader caller
   cancellation завершает общий flight, followers могут получить отказ.
8. Только ordinary inability получить свежую IPv4 hint допускает original
   non-caching DNSRouter fallback с его family behavior. Scope/cancel/clock/busy/
   uncertain Close не становятся fallback success. Stock Lookup не возвращает
   TTL receipt: его результат делится только между текущими callers в пределах
   processing deadline, не сохраняется как DNS cache. До16 адресов копируются;
   это не ограничение внутренних allocations/горутин SDK.
9. Worker использует existing ObservedClock для TTL continuity, не для выдачи
   trust authority. Краткие local failures сохраняются bounded counter/last error;
   probe снимает stamp до работы и передаёт typed negative report **без RPC error**,
   потому что existing worker_child удаляет report при ошибке команды. Local
   busy/clock/cancel не записывается как channel health failure. Cleanup fact
   отдельно выставляет CleanupUnconfirmed. Нет нового positive grant API.
10. Close сначала отменяет per-box producer и отзывает admission, затем ждёт
    original flight, cancellation callback и всех borrowers до Box.Close.
    Cooperative wait2s не прерывает arbitrary syscall. Неопределённость сохраняет
    actual domain/fault, irreversibly закрывает admission/clear pending/native
    receipt/deactivate scope; original Box/Job остаётся владельцем retirement.
    Нет successor поверх uncertain owner, recursive Close из resolver или I/O
    под App/admission/value mutex. Здоровые established соединения по DNS TTL
    не закрываются, фонового refresh/reconnect timer нет.

## Что просмотрено в закреплённых исходниках

Локальный `sing-box v1.13.19`: `adapter/dns.go`, `adapter/outbound/registry.go`,
`option/{outbound,vless,hysteria2}.go`, `box.go`,
`common/dialer/{dialer,resolve}.go`, `protocol/{vless,hysteria2}/outbound.go`,
`transport/v2raywebsocket/client.go`, DNS router/client/transports.
Registry передаёт pointer typed options; service DNS transports/router создаются
до outbounds. ResolveDialer захватывает constructor DNSRouter и вызывает Lookup
для domain на новом dial; WS сохраняет request context. `sing v0.8.13`
`service/context.go`: ContextWith изменяет registry, ExtendContext клонирует его.
Поздние service registrations не являются обещанием динамического clone sync;
поддерживаемый constructor/dialer path разобран по этой pinned версии.
Зависимости/версии/лицензии/SDK не изменялись и не выполнялись.

## Доказательства и незакрытый объём

Изменены16 Go/4 docs; gofmt, ручной source diff, byte/SHA inventory.
Evidence: `.artifacts/memory/endpoint-refresh-source-evidence.json`.
Это **не compile, SDK parser, runtime, DNS, native, OS, speed или safety PASS**.
Tests/fixtures не добавлялись/не изменялись/не запускались; build/vet/typecheck/
doctor/product/SDK/helper/browser/admin/live/DNS/server не выполнялись.

Открыты: current API fixtures/SDK acceptance; initial/expired/replay/rollback/
clock-jump/cancel/Close race; реальные reconnect и service-address rotation;
IPv4/IPv6/family fairness/UDP/TCP/path scope; TUN bootstrap route/capture/WFP и
coexistence; stdlib pools/tasks/allocations, synchronous I/O/Close hard bounds,
RSS/latency и длительный нагрузочный режим. TLS service identity всё ещё должно
подтвердить actual TLS, а network epoch не выводится из DNS answer.
Через сериализацию нет доказанного monotonic/cold-restart continuity; worker
не объявляет это проверенным. Initial seed одного адреса не является RRset или
перебором всех reachable CDN edges. Повторный lookup не гарантирует другой IP.

Native acceptance const=false, TUN unavailable, schema2 install unsupported.
Все PC-00…09 и прежние FR/TH/OFF/COM/RQ/TR, TH-00…08, W3b/SDK/OS/UX/release/
comparative gates сохраняются. Следующий source-срез — family/path contract и
stale-operation handoff, без исполнения проверок.
