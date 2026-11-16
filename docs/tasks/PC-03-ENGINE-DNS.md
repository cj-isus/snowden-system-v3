# PC-03 — DNS движка: service bootstrap и пользовательские запросы

2026-11-04, V2-209, Windows **source/unverified**. Часть полного
[PC-00…09](../planning/knowledge-application/PC-PLAN.md). Продолжение
[bootstrap/cache](PC-03-BOOTSTRAP.md), без разрешения на тестирование.

## Источник дефектов

Прежний renderer выбирал direct `bootstrap-dns` правилом `domain` из всех
descriptor.Hostname, затем catch-all `tunnel-dns`. Это смешивало адрес соединения
с TLS SNI/Host/REALITY donor и пользовательским вопросом с таким же именем.
Пустой список убирал условие direct-правила. В закреплённом ядре DNS action
участвует в IsValid, а пустой matcher не создаёт дополнительного ограничения;
ссылки на сервер сами по себе не подтверждают безопасную область правила.

Default DNS cache не был разделён по transport. Source `dns/client.go` использует
общий question key без independent_cache и question+transport tag с ним. Поэтому
разделения только правил недостаточно для самостоятельных bootstrap/user ролей.

В route первым стоял `protocol=dns`, но не было обязательного sniff перед ним.
Неопознанный TCP/UDP port53 мог дойти до process/LAN-direct правил. Отдельное
TUN `route_exclude_address=172.16.0.0/12` обходило engine rules вообще.
Исходный строгий parser проверял DNS references, но принимал direct/default-local/
незащищённый final, неподдержанные action и отсутствие самого DNS-контракта.

## Решение в существующих компонентах

- В `config` добавлены ровно поддерживаемые поля DNS final/cache/typed transport,
  TLS peer identity, outbound DomainResolveOptions и route port. Одна
  `validateDNS` вызывается прежним Config.Validate до выдачи normalized runtime
  bytes и original-worker загрузки. Нового DNS runtime/controller или второго verifier нет.
- `render/configureDNS` работает с фактическими Outbounds после IP-подстановок.
  Только domain в реальном `outbound.Server` получает explicit
  `domain_resolver={server:bootstrap-dns,strategy:prefer_ipv4,disable_cache:true}`.
  Hostname/SNI/donor сами по себе не создают direct DNS dependency. Для IP-only
  набора bootstrap transport отсутствует; для mixed набора добавляется ровно один.
- Пользовательский `dns.final=tunnel-dns` — typed TLS к literal8.8.8.8:853 через
  protected selector proxy, с `tls.enabled=true/server_name=dns.google`.
  Bootstrap — typed UDP literal1.1.1.1:53 через direct. Глобальных DNS rules нет,
  включая правила для имён серверов. Вопрос пользователя с именем endpoint/donor
  следует тому же protected final. Ошибка защищённого resolver не выбирает direct.
- DNS independent_cache=true, configured cache_capacity=1024; bootstrap dialer
  disable_cache=true ограничивает и reads, и writes в проверенных исходниках.
  Bootstrap answer не становится защищённым cached answer. В pinned client
  меньшая capacity поднимается до1024; число1024 — настройка API, не доказанный
  предел RSS/суммарной памяти/числа конкурентных запросов. Cache namespace роли
  не доказывает независимость сетевых провайдеров.
- Первая route — точный `port:[53]/action:hijack-dns`, без зависимости от sniff,
  process/IP/domain matcher. Далее сохранено `protocol:dns` для распознанных
  запросов на других портах, затем осознанные process/LAN правила. Это обычные
  DNS поверх TCP/UDP53, не перехват всего DoH/DoT/DoQ или arbitrary encrypted DNS.
- Blind TUN prefix exclusion удалён из renderer. Строгий контракт отвергает
  TUN без automatic+strict routing или с route exclusions; deliberate LAN/process
  exceptions остаются внутри engine после port53 capture. Это ещё не аттестация
  фактических Windows routes, WFP, connected-route precedence или чужого VPN.

DNS service address, service TLS identity, transport family и фактический
пользовательский маршрут остаются разными фактами. Текущие DNS transports
используют IPv4 literals; AAAA answer не доказывает IPv6 route. SOCKS режим
охватывает только трафик, действительно переданный в него. Native admission
gate остаётся закрытым; packet ingress до принятого gate по-прежнему запрещён.

## Совместимость и отказ

Подписанная profile/channel schema, credentials/pins, selector и actual native
receipt/IPC8 не менялись. Runtime DNS пересобирается из принятого профиля одним
renderer. Неподдержанный legacy raw DNS, question-based rules, direct/default-local
final, неверные resolver roles/references/cache/capture/TUN flags дают явный
ErrDNSContract до замены движка; нет автоматического insecure migration/fallback.
Legacy `address` поле оставлено для понятного отказа старому raw input и
опускается в новом typed JSON. Existing default-channel/probe/trust/Manager
ownership и rejected-update/retained-session outcomes сохраняют свои gates.

Старые fixtures и raw-config API ожидания требуют миграции после разрешения
на тестирование. Полный config.Parse/sing-box parse/new Box не запускался.
Это не текущая binary compatibility или успешный Start.

## Опора на источники

Исследованы локальные исходники **sing-box v1.13.19**, закреплённые в windows/go.mod:

- option/outbound.go, common/dialer/dialer.go и resolve.go: explicit transport
  попадает в DNSQueryOptions для ServerIsDomain, а Lookup с ним обходит DNS rules.
- option/dns.go, dns/router.go, transport_manager.go: typed transports и explicit
  final; DNS transports создаются раньше outbounds в box.go. Отсутствующий final
  не подменяется успешным local fallback.
- dns/client.go: cache key, disable-cache обеих операций и capacity floor1024.
- option/rule.go, option/rule_dns.go, route/rule/rule_abstract.go и route/route.go:
  port matcher и различие sniff/classification/capture.
- protocol/vless/outbound.go, protocol/hysteria2/outbound.go и dns/transport/tls.go:
  actual server/dialer и TLS identity construction.

Официальный [контракт Google DNS-over-TLS](https://developers.google.com/speed/public-dns/docs/dns-over-tls)
задаёт authentication name dns.google, проверку сертификата и отказ при
неподтверждённой identity. Страница прочитана 2026-11-04, указанный update2024-09-03.
Это чтение публичной документации; provider/VPS/port/path не проверялись.

## Evidence и открытый объём

4 Go/4 docs, оригиналы8 путей (3new/absent), source/gofmt/manual diff/bytes-SHA:
`.artifacts/memory/engine-dns-source-evidence.json`. Ни tests/fixtures/build/
compile/vet/typecheck/parser/check/SDK/helper/product/native/browser/admin/live,
ни DNS/network/config execution не выполнялись; реальная ОС не менялась.

Fixed DoH probe не измеряет native port53→hijack→user final. Отдельны full codec/
engine parser/API fixtures, default/donor/empty/IP-only/mixed/cancel/Reload/worker
cases, actual UDP/TCP53 с LAN/split, PKI/provider availability, Windows route/WFP/
DNS settings и другие приложения. Native ABI/resources/stdlib cache/ConnPool/
goroutine/Close/hard time/RSS/durability/concurrency остаются unverified.

Далее PC-03: active-route address refresh/reconnect и явная family/path coverage;
cold-restart cache identity и hard optional I/O остаются открытыми. Весь PC00…09/
old9/W3b/SDK/UX/interprocess/release/comparative сохраняется. Compiled native
acceptance const=false, TUN unavailable, schema2 install unsupported.

<a id="pc-v313-sdk-dns"></a>

## V2-313 — перевод bootstrap DNS в закреплённый SDK

**Source/unverified, 2026-11-08.** Продолжается весь PC00…09/old9;
100 OPEN/56 NOT_RUN, без waiver.1 Go/4 docs. Tests/fixtures/build/typecheck,
SDK/options parser/check/start, native/browser/admin/network не выполнялись.

### Независимо установленная source-причина

Внешний отчёт о stock1.13.19 дал гипотезу; его payload PASS/exit IP не использованы
как собственное доказательство работы нашего приложения. Реальные профили,
config/vault/secrets/логи и VPS не читались/не менялись, чужой чат не открывался.

В windows/go.mod закреплён sing-box1.13.19. Renderer создаёт plain
Outbound{Type:direct,Tag:direct}; normalized DNS-контракт требует typed literal
UDP bootstrap-dns с Detour:direct. Проверены cached primary sources этой версии:

- dns/transport/udp.go: NewUDP→NewRemoteDialer; Start(StartStateStart) вызывает
  InitializeDetour. Это позже JSON/options decoding.
- dns/transport_dialer.go: typed remote DNS без LegacyDefaultDialer использует
  dialer.NewWithOptions. Literal public IPv4 не требует recursive resolver.
- common/dialer/dialer.go: непустой Detour создаёт NewDetour; пустой — NewDefault,
  а не NewDefaultOutbound и не выбор protected selector/route.final.
- common/dialer/detour.go: init отвергает DirectDialer.IsEmpty; точный diagnostic
  error уже есть в SDK. protocol/direct/outbound.go вычисляет IsEmpty по dialer
  options, не tag. Plain direct renderer попадает в этот случай.
- option/dns.go: явный type:udp получает typed registry options; legacy address
  upgrade/default-outbound ветка не является данным путём.

Следовательно старый source contract мог пройти options parse и отказать на
более позднем Start. Ни наш parser, ни SDK parser или Start здесь не запускались.
Это source вывод о ветке отказа, не измеренный текущий отказ/доступность сети.

### Узкая поправка на существующей границе

sdkRuntimeConfiguration уже отделяет normalized config от SDK view, раньше
удаляя только BootstrapHints. Теперь после прежнего strict config.Parse:

1. Plain direct определяется reflect.DeepEqual с полным пустым config.Outbound
   кроме Type/Tag. Любые custom fields, включая будущие ненулевые поля, не дают
   право на переписывание: их detour сохраняется для прежней SDK обработки.
2. DNS struct и servers slice копируются перед изменением. Только точный
   bootstrap-dns/Detour:direct получает пустой detour в SDK JSON. Protected resolver,
   TLS peer, proxy detour, final/cache/rules, service domain_resolver и route capture
   не изменяются. IP-only набор не получает bootstrap resolver.
3. Typed literal UDP использует штатные DNS socket defaults закреплённого SDK.
   Это translation роли service bootstrap, не автоматический fallback после
   сетевой ошибки. Socket/UDP-fragment defaults direct outbound и DNS dialer
   различаются в SDK; полной поведенческой эквивалентности не заявляется.
4. Возвращаемый cfg остаётся исходным strict normalized object, включая explicit
   bootstrap detour и hints. SDK view не сохраняется в e.rawJSON/IPC/rollback и
   не становится новым subject. ConfigurationSubject/RuntimeConfigurationDigest
   продолжают использовать normalized bytes. Trust signatures/accepted profile/
   worker instance/selector/admission epoch не пересоздаются этим переводом.

Оба потребителя — validatedRuntimeOptions и newSingBox — уже используют один
sdkRuntimeConfiguration; отдельного parser/renderer/fallback нет. New box сохраняет
cfg для original bootstrapWorker и admission. Existing endpointDNSRouter с exact
transport/domain resolver и private original path/flight сохраняется. Вопросы
пользователя, включая имя сервера, по-прежнему не выбирают bootstrap через rules.
Прямой fallback защищённого трафика/пользовательского DNS не добавлен.

### Evidence и открытый объём

Manual source/caller/error/copy/subject review и touched gofmt1+1exit0;
5 existing snapshots/task71rev728/13 immutable reviewed+7 cached SDK SHA.
Evidence: .artifacts/memory/bootstrap-v313-sdk-dns-source-evidence.json.
Cached source read — не SDK execution/ABI/native/network приёмка.

OPEN: compile/options/start/fixtures/custom-direct/empty/IP-only/mixed/domain/
protected-final/refusal/Reload/rollback/worker/source-identity compatibility;
actual DNS/TLS/UDP response and fragmentation/socket routing, providers, WFP/TUN,
foreign VPN/IPv6/cache/resource/time/performance. No hard native cancellation,
literal reachability, route attestation, public payload PASS or release claim.

Следующий source seam: original DNS transport/ConnPool socket lifetime при
bootstrap worker pause/resume и path retirement. Свежая generation/подсказка не
доказывает новый underlying UDP socket; старый SDK owner не должен заменяться
без подтверждённого retirement. Прежний общий fullPC00…09/old9/PC04all8/
PC01W3bW4c/PC03signed-native/SDK/release/comparative остаётся OPEN.
Все bootstrap/bridge/native/chain/relaunch/parent/helper gates=false,
ranBeforeJob sticky, GUI6/R1/U1/IPC9/Job8, TUN unavailable/schema2 unsupported.
Android/shared/configs/harness/dependencies/Git/ОС/сервер не изменялись.

