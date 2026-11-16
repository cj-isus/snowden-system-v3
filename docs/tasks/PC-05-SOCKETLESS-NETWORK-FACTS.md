# PC-05 — локальный снимок маршрута без UDP-сокета

2026-11-05, V2-217, Windows **source/unverified**. Продолжение
[original poller owner](PC-05-NETWORK-OBSERVER-OWNER.md), полный
[PC00…09](../planning/knowledge-application/PC-PLAN.md).

## Почему изменён механизм

Прежний observer открывал UDP socket к literal ради LocalAddr. Caller cancellation
не передавалась в collector; результат Close терял original handle при ошибке.
Перебор Addrs всех адаптеров создавал неоднозначность при одинаковом IP, а класс
читался отдельным MIB call уже при доставке события. Самому сборщику нужны только
локальные route/source/adapter facts, поэтому network connection здесь удалено.

## Реализация по компонентам

- `network_route_windows.go`: сначала прежний AF_UNSPEC adapter snapshot.
  Непустой снимок до256 IP-адаптеров без up non-loopback links даёт explicit local
  down; virtual/IPv6/наш TUN учитываются. Ошибка/пустота/limit/cancel — unknown.
- Lookup IPv4 preferred, затем IPv6 только после failed family route lookup.
  `GetBestInterfaceEx` задаёт nonzero interface; `GetBestRoute2` получает маршрут
  и best source на этом interface. Source-family/LUID/index/loopback проверяются.
  `GetIfEntry2Ex` подтверждает тот же LUID/index/up и даёт alias/type/media.
  `GetUnicastIpAddressEntry` сверяет exact source/LUID/index, preferred DAD,
  SkipAsSource=false и ненулевой valid lifetime. В конце best interface читается
  повторно. Returned malformed/forbidden/stale tuple не пробует другую family.
- Переиспользованы закреплённые `x/sys v0.44.0` structures/wrappers и существующий
  system `iphlpapi.dll` object. Для отсутствующего в wrapper GetBestRoute2 — exact
  seven-argument readonly call с pinned structures, keepalive и return-status
  check; thread LastError не используется как результат. Наличие API проверяется
  до wrapper calls. Нет самописного C struct layout, новых deps или proc spawn.
- Own TUN alias сравнивается без учёта регистра; PPP/loopback/known tunnel и
  unusable/non-global/link-local/mapped source дают unknown. Duplicate IP больше
  не выбирает первый adapter: interface и source связаны native route LUID.
- Poller, Start preference и post-probe preference передают исходный context.
  Один cooperative sample budget700ms охватывает весь сбор, checkpoints идут
  до/после native stages и перед выдачей значения. Нет detached timeout goroutine,
  нового socket/Close owner или Background fallback. Уже выполняющийся Windows
  call не объявляется прерванным; original poller.done по-прежнему ждёт stack.
- Fingerprint включает LUID и captured class; equality замечает смену adapter
  identity. `netClassOf` теперь value-only: MIB в fire/preference больше не
  читается повторно. Lifecycle/poller/DNS/health/Stop guards V2-214…216 сохранены.
- Non-Windows stub возвращает unknown, без socket/system resolver fallback.
  Private Go hook/collector/preference signatures теперь принимают context;
  API/fixtures требуют отдельной приёмки, Wails/worker schema не менялись.

## Источники и границы

[GetBestInterfaceEx](https://learn.microsoft.com/en-us/windows/win32/api/iphlpapi/nf-iphlpapi-getbestinterfaceex),
[GetBestRoute2](https://learn.microsoft.com/en-us/windows/win32/api/netioapi/nf-netioapi-getbestroute2),
[GetUnicastIpAddressEntry](https://learn.microsoft.com/en-us/windows/win32/api/netioapi/nf-netioapi-getunicastipaddressentry)
прочитаны как primary API documentation. Pin и actual structures/uintptr calling
convention просмотрены в локальном cache, dependency installation не запускалась.

Это local table observation для тех же lookup literals `9.9.9.9`/`2620:fe::fe`,
не проверка их доступности, DNS/UDP connectivity, общего default route ко всем
серверам, physical underlay, protected egress или kernel ownership. Ethernet-shaped
TAP/virtual route не становится physical proof. LUID здесь не WFP admission token.
Individual native reads не атомарны; repeated index не исключает все route/address/
compartment/driver races. Preferred-only source selection консервативна; deprecated
source возвращает unknown. Проверки соответствия API/Win64 ABI пока не выполнены.

700ms — budget выдачи результата и cooperative checkpoints, не hard syscall/cancel/
CPU/RSS bound. Cap256 ограничивает обработку, но не внутреннюю allocation native
enumeration. Удаление collector socket закрывает его потерю при Close в этом source
path; остальные sockets/handles SDK/probe/helper этим не подтверждены и не изменены.

Source7 Go/4 docs/gofmt/manual caller/diff/byte-SHA inventory:
`.artifacts/memory/route-observer-source-evidence.json`. Tests/fixtures не
добавлялись/не менялись/не запускались. Compile/build/vet/typecheck/product parser/
check/doctor/install/gen/SDK/task/collector/worker/product/crypto/native/browser/
admin/live/network/server/owner-values/other-role/chat/subagent execution не было;
только чтение public primary docs снаружи. Нет compile/runtime/API/OS/performance PASS.

Открыты direct ABI/source-address layout, loader availability, Windows families/
duplicate IP/LUID/index reuse/virtual/PPP/DAD/lifetime/route change/Stop/cancel/
budget/legacy hook fixtures, actual collector progress/resources и весь native
family/path scope. Далее source — first-real-event/Start/ограниченное automatic
recovery/resume. IPC9/Job8 прежние, native acceptance const=false, TUN unavailable,
schema2 unsupported; полный PC00…09 и прежний backlog остаются OPEN.
