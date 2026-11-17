# KA-01/W4 — диагностика до допуска пользовательского трафика

V2-193…206, актуализация 2026-11-04. Owner: `windows`.
**В реализации, partial source/unverified**.
Полный PC-00…09, W4 и TH-00…08 сохраняются. Это не завершённая замена текущего lifecycle.

## Текущее состояние — не готово к сборке поставки

В actual newSingBox установлен закрытый ingress barrier. V2-194 подключает в
исходниках private worker probe и одноразовый exact-subject permit к Manager.
Новый путь ещё не компилировался и не выполнялся. **TUN остаётся unavailable**:
V2-200 подключает original-helper scoped observation к production Establish;
V2-202 соединяет original native source с worker, V2-203 отделяет informational
OS control от required native/actual-phase. V2-204 добавляет expiry-bound
prepared issuer; V2-205 реализует final native consume/platform-Manager source
за закрытым compiled acceptance gate. Normal helper Establish и native ingress
не включаются до будущей приёмки. Worker явно отказывает TUN activation по одному
outbound report. Нельзя объявлять
Ready по library Start, открывать по bool/report или собирать EXE как проверенную
рабочую версию. Тесты/сборки по указанию владельца пока запрещены.

## Основание

До W4 Manager записывал proxy до lifecycle probe. TUN Establish сначала выдавал
general LUID permit, затем проверял внешний путь; unrelated user flows в этот
период уже получали разрешение. Сам mixed inbound тоже был доступен до verdict.
Configured/live label, process running и egress probe — разные уровни допуска.

Pinned первичные источники:

- [sing-box box.go](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/box.go):
  actual Box создаёт inbound через registry с переданным Router; lifecycle/crypto
  не требуется копировать в новый runtime.
- [adapter/router.go](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/adapter/router.go)
  и [mixed/inbound.go](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/protocol/mixed/inbound.go):
  TCP/packet handlers и их Ex variants; mixed делегирует через переданный router.
- [tun/inbound.go](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/protocol/tun/inbound.go):
  PrepareConnection/redirect используют PreMatch; TCP/packet Ex присваивают inbound
  tag/type. Source trace не доказывает native Windows packet interception.
- [sing v0.8.13 service/context.go](https://github.com/SagerNet/sing/blob/v0.8.13/service/context.go):
  ContextWith меняет registry, ExtendContext копирует её. Поэтому adapter внедрён
  в cloned context, не в чужой service registry.
- [adapter/outbound.go](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/adapter/outbound.go),
  [group/selector.go](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/protocol/group/selector.go)
  и [adapter/inbound.go](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/adapter/inbound.go):
  actual outbound dialer/registry, selector lifecycle/mutation и SDK metadata
  context. Source чтение не подтверждает ABI, DNS/TLS integration или VPN-путь.

## Сделано в исходниках

1. `core/forwarding_admission.go`: один private admission на actual box, default
   closed. Existing registry/decoders/constructors переиспользуются. Поддержанные
   mixed/TUN получают guarded Router; другая inbound family отклоняется без
   unguarded fallback и при raw config validation, и при native Create.
2. Legacy и Ex TCP/packet routing проверяют context/admission и fixed source
   tag/type. Отказ закрывает принятое соединение и передаёт Close/error outcome
   caller callback. PreMatch также закрыт; native bypass не включается при delegate.
   Box.Close отзывает ingress до native cleanup. Никакого public enable/proof
   constructor, нового inbound port/password или trust verifier не добавлено.
3. `newSingBox` клонирует service context и оборачивает настоящий stock registry.
   Box.Start/Close, original worker/Job, pin/TLS/config/source/lifetime guards и
   selected SDK versions остаются прежними. Compile-time interface declarations
   добавлены в source; компиляция их **не выполнялась**.
4. RenderFrom переиспользует полный bounded parseSet: duplicate IDs, protocol,
   endpoint/credential-role shape до первого secret lookup. parseSet возвращает
   owned nested copies и повторяет validation clone; Get/GetByID callback не
   получает alias исходного caller descriptors. getOrErr повторяет общий gate
   перед доступом к vault. Signed bytes/history/schema не меняются. Это shape/
   ownership gate; сам по себе он не доказывает криптографическое разрешение
   endpoint, работоспособность транспорта или freshness. Прежние authenticated
   profile source и protocol TLS/REALITY/HY2 checks остаются обязательными.
5. Manager.Start теперь engine→full lifecycle probe/lifetime/owner checks→proxy
   write/readback→повторные context/engine/lifetime checks. Snapshot/cleanup owner
   W3a прежний. Эта перестановка отдельно не закрывает forwarding gate.
6. Native TUN Establish больше не выдаёт general permit до внешней проверки.
   Пока отсутствует original-helper scoped admission, он проверяет current worker/
   config и base lockdown, затем возвращает unavailable без fake lease/HTTP.
   General LUID permit implementation сохранён для будущего final commit, но из
   предварительного пути вызов удалён. Его фактическая поддержка не заявлена.

## Следующее обязательное продолжение

### W4b. Original-worker diagnostic path — V2-194 source/unverified

- Existing original Job/pipe получает protocol2 и bounded `probe`/`admit`, reply
  до64KiB. Exact command properties, nested probe shape, sequence, fixed proxy
  selector, normalized raw SHA и current tag/epoch проверяются с обеих сторон.
  Нового listener/password, regrant worker ресурсов или embedded fallback нет.
- App newAppProbe использует NewEngineProbe; limited HTTPS/DoH идут через actual
  guarded selector outbound. Разрешён только TCP к exact плановым HTTPS origins,
  redirect/env proxy отсутствуют; normal TLS проверяет HTTP origins. Current SDK
  constructor services и per-dial metadata сохраняются вместе с request context/
  deadline. Route `engine_outbound` отделён от `tunnel_socks` и `os_control`.
  Это проверка выбранного outbound, **не доказательство user ingress, routing
  rules, native DNS/UDP/IPv6 или credential endpoint realm**.
- Parent заранее резервирует прежний diagnostic budget; child отдельно ограничен
  actual work, временем и bounded report. OS-control семантика SOCKS сохранена;
  same-EXE OS control не становится TUN proof и не заменяет W4c.
- Только успешный actual worker report выдаёт random32-byte one-use token на5s.
  Exact subject/tag/epoch связывает его с box; parent хранит private permit вместе
  с original worker instance. Повторный/неудачный parent probe сбрасывает старый
  permit, включая budget rejection. Worker сжигает pending token до результата;
  пустой token подтверждает только уже открытый тот же subject, не открывает новый.
- Manager Start, Reload, восстановление прежнего конфига и selector outcome
  применяют private permit после complete lifecycle report и повторных context/
  actual engine/lifetime gates. После RPC freshness проверяется снова. Unknown
  reply/transport/subject ведёт к original retirement; cleanup uncertainty
  сохраняет owner, не retry/fallback/Ready.
- Обёрнут **actual stock outbound selector**, включая native SDK callers. Пока
  admission open, другой tag отвергается до mutation. Existing App failover
  обрабатывает ErrSelectorSwitchUnavailable через owned Reload/полное retirement
  и fresh probe; не переносит старые admitted streams на кандидат. До допуска
  всякая смена увеличивает epoch/стирает pending proof; epoch до/после probe
  связывается с issuance под тем же mutex. Close permanently revokes, повторяя
  atomic false под mutex против concurrent consume. Безопасная seamless custody
  и native in-flight/retirement ещё требуют приёмки; ускорение не измерено.
- RuntimeIdentity дополнен actual admission epoch/state; hello/start/current/
  switch replies проверяют subject. Изменение private protocol требует нового
  matching EXE; old worker/fixture compatibility пока не проверена и не исправлена
  запуском. Worker/compiled adapter доверены; это не hostile worker attestation.

### W4c. TUN scoped diagnostic actor и lifecycle

**V2-195 — только helper-side source, пока не подключён к native lifecycle.**

- Новый fixed purpose в существующем `snowden-native-probe.exe`, без другого
  executable/dependency/service/driver. Прежний DNS protocol1 оставлен прежним.
  TUN-mode принимает ровно один fixed argument; request идёт через private stdin,
  не command line/process audit: nonce, normalized graph SHA, tag, epoch и timeout.
- `netprobe/tun_protocol.go` переиспользует existing strict nativeObject: exact
  required properties, unknown/duplicate/alias/null/trailing rejection; framing
  до allocation, request1KiB/message8KiB/body4KiB, canonical base64 body. Типизированные
  ready→go→result→release→released сообщения — только данные. Их decode/constructor
  не доказывает original actor, route, intent, trust или право открыть TUN.
- Windows helper создаёт один non-inheritable TCP4 socket и ставит
  SO_EXCLUSIVEADDRUSE **до wildcard bind**, проверяет фактический socket option и
  assigned port через native readback. Ready публикуется после bind, Connect
  возможен только после exact matching go. Сокет удерживается тем же one-shot
  process, включая partial acquisition, HTTP failure/EOF/TLS duplicate close;
  другой socket/port/retry не создаются. Raw socket/WSA/file wrapper остаются
  process-owned, не освобождаются/финализируются до original process retirement.
- Один connect к `1.1.1.1:443`, затем standard Go FileConn duplicate + TLS1.2+
  с normal certificate/IP verification, fixed HTTPS GET. Нет DNS/env proxy/
  redirect/decompression/другого dial/пользовательского target. Response headers
  ограничены16KiB, body4KiB+1; parent должен сам разобрать exact bounded response,
  HTTP200 и единственный IPv4 ip field, сопоставить expected egress. Helper не
  выдаёт `protected=true`/permit. URL теперь общий existing renderer↔netprobe.
- Весь TUN-mode имеет startup request watchdog5s и operation deadline500…8000ms;
  watchdog exits3 при stalled anonymous pipe/native Connect. Это не hard Windows
  syscall termination bound. `released`, zero exit или совпавший IP отдельно
  **не** подтверждают cleanup/route; нужен checked original Job/I/O retirement.
- Read-only source findings: обычный AppID/пакетный SID не идентифицирует original
  process; generic DuplicateHandle socket path не используется. FileConn — только
  same-process стандартный Go путь. При source review request убран из argv,
  исключая graph-digest/nonce в process command-line audit; dial one-use — atomic.
  Ни один из этих выводов не подтверждён native execution.

**Следующее обязательное соединение, без которого TUN остаётся unavailable:**

1. Parent переиспользует protected helper lane, held pinned image и original
   suspended process/Job/pipe; resource limits и matching compiled helper pin.
   Данные ready никогда не принимаются от чужого listener/process/decoded marker.
2. Original **PROCESS** handle дублируется в SAME original engine process через
   retained target handle, не через PID reopen/имя. Worker scope содержит actual
   native handle/lifetime, bound port, exact raw/tag/epoch/nonce/deadline. Socket
   между процессами через generic DuplicateHandle не передаётся.
3. В default-closed TUN Router/PreMatch допускается только TCP tuple reserved
   source port→fixed1.1.1.1:443, пока actual original helper process alive и current
   subject не изменился. WFP diagnostic filter одновременно требует helper AppID,
   exact LUID, TCP/ports/remoteIP. Ordinary user ingress/general permit закрыты.
   Проверка смерти actor/port reuse/check→route/in-flight/crash остаётся OS gate;
   одиночный user-mode liveness check не объявляется атомарной kernel guarantee.
4. После bounded response сначала revoke diagnostic ingress+WFP, затем release
   и full original retirement. Unknown outcome сохраняет исходных owners и
   strict barrier. Только полный fresh native proof вместе с actual outbound
   probe и Manager source/intent/lifetime может открыть general forwarding.
   Same-EXE OS-control не используется как положительный TUN verdict; issuance
   одноразового5s permit должно следовать за полной native проверкой, не истечь
   во время неё и не обновляться по одному успешному bool/report.

**V2-196 — original-handle/ingress часть в исходниках; полный parent FSM/WFP ещё
не подключён. Общий TUN допуск по-прежнему явно отклоняется.**

- Engine private begin/end переиспользуют только original protected helper lane
  с held pinned image/actual process validation. Scope и native output receipt
  сохраняются до DuplicateHandle; actual helper Job проверяется как one-process/
  прежний128MiB budget по native query/accounting без переустановки лимитов.
  Query проверяет включённые flags/active limit/memory и peaks; incidental
  affinity/priority/workingset поля без соответствующих flags не становятся
  ложным обязательным нулём. Это source исправление по Win32 contract, не native PASS.
- Дублируется original **process object** в retained original engine target,
  только SYNCHRONIZE|PROCESS_QUERY_LIMITED_INFORMATION, non-inheritable. Parent
  никогда не закрывает target-process numeric value в своей таблице, не делает
  PID reopen/socket DuplicateHandle. Unknown duplicate/control/panic result
  требует retirement того же original engine Job; failed retirement сохраняет
  scope/target owner. Stale end другого scope не останавливает новый worker.
- Private engine IPC теперь3: exact scope-open/close commands, nested required
  properties/no aliases/duplicates/null/unknown, raw/tag/epoch/nonce binding и
  обычная original pipe sequence. Scope nonce запрещён в hello/start/current/
  switch/probe/admit replies. Legacy worker2 отклоняется; это не public API или
  готовая миграция fixtures. Parent/worker — matching same EXE, helper purpose1
  прежний. Доверие compiled parent/worker, не hostile-worker attestation.
- Receiver retains native handle до native validation; width/pseudo/inheritance,
  actual process ID/creation identity/zero-time wait проверяются через original
  object. PID/creation marker используются как факты для сравнения, не поиск или
  clock/trust authority. Failed Close sticky, без retry→proof; permanent box
  revoke предшествует SDK Close. Original worker retirement — cleanup backstop.
- Closed TUN ingress/PreMatch допускает только reserved TCP4 port→1.1.1.1:443
  при живом original actor, current epoch и коротком monotonic deadline. Fixed
  единственный TUN inbound tag/type, no FQDN/UDP/packet/mixed/second TCP connection/
  direct-route bypass; empty/ambiguous TUN tag не получает scope.
  One retained connection wrapper/context обеспечивает cancel + checked once
  Close до actor-handle close; expiry timer отзывает поток и сохраняет receipts.
  Close проверяет deadline до/после native cleanup, не полагаясь на успевший timer.
  Selector mutation и issuance запрещены, пока scope/unknown cleanup удерживаются.
- Actual pinned TUN source передаёт Network в PreMatch, но оставляет empty в
  TCP RouteConnectionEx. Source adapter дополняет его только в typed TCP path;
  packet handlers не получают такого допущения. Это исправлено чтением исходника,
  не воспроизведением. Normal admitted routing сохраняет прежний путь.
- Original actor wait в user mode, check→route, socket/port reuse/death, API/ABI/
  Job/IOCP/once-close/timer/concurrency/SDK semantics ещё требуют native приёмки.
  V2-197 ниже добавляет capture/WFP source. Private native proof, separated
  same-image OS control и final general permit/Manager commit остаются
  **следующей обязательной работой**. Ни scope ACK, ни HTTP observation не Ready.

### V2-197 — original capture и узкий WFP, source/unverified

- `Engine.ObserveNativeTUN` использует прежнюю reserved protected lane, held
  pinned image, original suspended process/Job и его pipes. Весь fixed HTTP/IPC
  payload резервируется до child; нет новой очереди, bare-exec/PID fallback,
  argv с nonce/subject или нового EXE/dependency. Image/process и actual intent
  guard проверяются до Resume. Это переиспользуемая primitive, пока **не вызвана
  production `Establish`**.
- Parent выполняет ready → original engine scope-open → concrete WFP acquire →
  go → result → ingress/filter revoke → release/released → точный EOF → original
  zero exit + полное Job/I-O/image retirement. Один finite exchange сохраняет
  свой `ioDone` до завершения; timeout poisons protocol, trailing byte запрещён.
  После retirement повторно разбирается HTTP200/single IPv4 и проверяется тот же
  worker/config/tag/epoch. Возвращается только HTTP-факт, не forwarding permit.
- Partial scope/filter owner сохраняется **до acquisition** в прежней lane.
  Cleanup не использует canceled operation context, отзывает оба разрешения,
  сначала закрывая ingress; original helper/socket/image не освобождается при
  неопределённости любого отзыва. Process-only retirement не может обойти этот
  record. Original scope-close и original Job retirement имеют раздельные receipts;
  новый worker/повторный Close не заменяют отсутствующее доказательство.
- Private platform adapter допускает ровно nonpersistent soft IPv4 permit:
  helper AppID ≠ engine bypass AppID, observed TUN LUID, TCP, exclusive source
  port, remote `1.1.1.1:443`. Полный expected owned-filter shape, тот же live actor/
  source/epoch/LUID и отсутствие general20/21 проверяются до go и после result.
  General TUN/user ingress не открываются.
- Mutable WFP session output/partial owner и fixed backing storage удерживаются
  до native acquisition. Dynamic-session Close0 — собственный BFE cleanup receipt,
  не повторная key-based adoption и не доказательство неизменности чужой policy.
  Unknown native Close sticky: global owner и protected lane остаются занятыми.
  [Microsoft описывает удаление dynamic objects при завершении original session](https://learn.microsoft.com/en-us/windows/win32/fwp/object-management).
- Чтение pinned `wf` выявило source-дефекты: `fromValue0` возвращает uint8/uint32
  вместо ожидаемых IPProto/ConditionFlag; private FWPM_FILTER0 располагает оба
  члена C union последовательно и затем читает смещённый EffectiveWeight.
  Root filter reads переведены на Windows-owned native reader с правильным
  16-byte union, явным amd64 ABI fence, typed conditions и requested/effective
  weight equality. Enum ограничен своим provider, 32 filters/8 conditions/page16;
  partial enum/page/session сохраняются до checked cleanup. Отсутствующий owned
  general filter больше не удаляется по одному stable key: foreign collision
  должен отказать на Add. Это source исправление, не native reproduction.
- [FWPM_FILTER0](https://learn.microsoft.com/en-us/windows/win32/api/fwpmtypes/ns-fwpmtypes-fwpm_filter0),
  [SESSION0](https://learn.microsoft.com/en-us/windows/win32/api/fwpmtypes/ns-fwpmtypes-fwpm_session0),
  [enum template](https://learn.microsoft.com/en-us/windows/win32/api/fwpmtypes/ns-fwpmtypes-fwpm_filter_enum_template0),
  [enum destruction](https://learn.microsoft.com/en-us/windows/win32/api/fwpmu/nf-fwpmu-fwpmfilterdestroyenumhandle0),
  [void allocation free](https://learn.microsoft.com/en-us/windows/win32/api/fwpmu/nf-fwpmu-fwpmfreememory0)
  и [Microsoft SDK fwptypes.h](https://github.com/microsoft/win32metadata/blob/main/generation/WinSDK/RecompiledIdlHeaders/shared/fwptypes.h)
  сверены как source/API, без исполнения. Cache dependency не изменён; native SDK
  structs не проверены компилятором или Windows вызовами.

### V2-198 — original persistent writer и BFE transactions, source/unverified

- Общий `nativeWFPSessionOwner` переиспользуется diagnostic/read/persistent
  owners. WFP actions выделены в отдельный компонент; platform seams
  Arm/BlockDirect/Release сохранены. Persistent writer удерживает **ту же original
  non-dynamic session** через успешные Arm/grant/restrict. Close происходит после
  explicit Disconnect, actual policy removal и подтверждения отсутствия объектов.
  Unknown Close sticky; другой owner/session его не заменяет.
- Base provider/subLayer/eight rules, обе general permits, restrictive revoke и
  explicit removal проходят через отдельные bounded BFE read/write transactions.
  Каждый native input и output filter ID сохранён до API, batch <=40 records.
  Все удаления получают owned identities или fixed object facts из **той же
  transaction**, а не предыдущего snapshot. Add не удаляет foreign same-key
  objects; повреждённый base не заменяется на retry. Matching general pair
  сохраняется, partial/mismatching pair требует сначала BlockDirect.
- Begin/Commit/Abort имеют отдельные original receipts. Error/panic/cancel после
  попытки grant Commit сохраняет pending grant и запрещает новые Arm/grant.
  Разрешён original-session restrictive cleanup; pending снимается только после
  committed revoke и fresh absence check. Abort/NO_TXN закрывают только вопрос
  активной transaction, не превращают unknown Commit в успешный grant.
- Reader создаёт/читает enum в одной explicit read transaction; writer reuse
  читает собственную active transaction. Checked page/enum/transaction cleanup
  и последняя проверка context предшествуют выдаче фактов. Identity-only mode
  используется только для restrictive/explicit cleanup повреждённых OWNED rules,
  не для validation. Dynamic owner должен завершиться до Disconnect.
- Provider GUID/persistence/disabled state/отсутствие service binding и subLayer
  GUID/provider/persistence/weight проверяются вместе с filters. AppID allocation
  теперь принадлежит тому же native resource record, включая error outputs;
  библиотечные `wf.New`/mutations/enumerations/AppID больше не вызываются этим
  Windows WFP path. Dependency остаётся источником публичных типов/constants;
  dependency/cache/версии не менялись.
- Декодер читает только active-width member C union. Неиспользованные upper
  bytes и вторая половина rawContext union не обязаны быть нулевыми; TypeID,
  фактическое значение и meaningful flags продолжают сравниваться строго.
  Это source correction, без runtime воспроизведения.

Первичные API contracts:

- [BFE object/transaction management](https://learn.microsoft.com/en-us/windows/win32/fwp/object-management)
  и [Begin](https://learn.microsoft.com/en-us/windows/win32/api/fwpmu/nf-fwpmu-fwpmtransactionbegin0),
  [Commit](https://learn.microsoft.com/en-us/windows/win32/api/fwpmu/nf-fwpmu-fwpmtransactioncommit0),
  [Abort](https://learn.microsoft.com/en-us/windows/win32/api/fwpmu/nf-fwpmu-fwpmtransactionabort0).
- [Enum snapshot](https://learn.microsoft.com/en-us/windows/win32/api/fwpmu/nf-fwpmu-fwpmfiltercreateenumhandle0):
  создание и чтение в explicit transaction для согласованного набора.
- [Provider](https://learn.microsoft.com/en-us/windows/win32/api/fwpmtypes/ns-fwpmtypes-fwpm_provider0),
  [subLayer](https://learn.microsoft.com/en-us/windows/win32/api/fwpmtypes/ns-fwpmtypes-fwpm_sublayer0),
  [AppID output ownership](https://learn.microsoft.com/en-us/windows/win32/api/fwpmu/nf-fwpmu-fwpmgetappidfromfilename0)
  и [filter deletion/session restrictions](https://learn.microsoft.com/en-us/windows/win32/api/fwpmu/nf-fwpmu-fwpmfilterdeletebykey0).

Это native transaction **contract в исходниках**, не доказанная atomicity/OS
durability. BFE serializes database transactions; отдельная platform lease между
разными App instances/пользователями/службой пока не доказана. `txnWaitTimeout=1s`
ограничивает ожидание lock, не длительность всей native операции: OS RPC/native
hard deadline, forced termination, suspend/clock, ACL и interprocess приёмка OPEN.
Persistent intent marker/storage и другие legacy backends этим срезом не закрыты.
Gofmt/source/API/manual diff only; tests/build/SDK/helper/native не выполнялись.

### V2-200 — preparation в actual lifecycle, source/unverified

- Существующий `nativeTunnelBoundary` получает actual App, а `Establish` вызывает
  прежний `ObserveNativeTUN` только с compiled helper pin и concrete native WFP
  adapter. Dev missing pin/injected components/invalid expected IPv4/stale
  owner/intent/clock/epoch/admitted worker отказывают до запуска helper.
- `nativeTUNLifecycleScope` нужен потому, что Start/Reload candidate ещё не
  опубликован в `runtime_.cc`; config OwnerGeneration может относиться к старой
  операции. Существующий `GuardRunningOwner` для этой фазы непригоден. Новый
  `GuardLifecycleOwner` — short value-only read guard active Manager owner/
  generation/engine/starting-reloading, без authority, IPC или Engine methods.
  Порядок runtime→Manager→App→epoch→clock; native work выполняется после release.
- Private scope связывает original Manager/engine, candidate identity+owned
  clone, intent/profile/update restriction/trust clock/pin/network epoch,
  exact instance/config/tag/admission epoch/closed phase и observed LUID.
  Helper callback повторяет source/interface facts; concrete adapter проверяет
  именно live diagnostic rule, поэтому base-only проверка внутрь callback не
  помещена. Это не новая policy/verifier/storage реализация.
- HTTP fact opaque, expected IPv4 сравнивается семантически после полного
  original Job/I/O/image retirement. Никакие публичные report/IP/bool не создают
  эту prepared lease. Lease не хранится как running proof, не выдаёт token и
  не принимает false→true без будущего explicit final phase.
- Source review: прежний `verifyWFPPolicyContext(false,0)` допускает general
  rules20/21. Для pre-admission этого недостаточно. Новый explicit closed
  verifier переиспользует тот же native bounded reader и exact verifier в strict
  режиме, отвергая general/diagnostic/unknown owned filters до helper и после
  retirement. Legacy base-observation callers сохраняют прежний контракт.
- Original ownership остаётся в core protected lane при ошибке. Finite
  `CleanupUnconfirmed` переносит typed helper/worker custody uncertainty через
  report seam без raw native errors/paths/handles. Manager сохраняет
  `ErrHelperCleanup`, прекращает candidate churn/rollback, проводит cleanup
  прежними владельцами; failed cleanup не теряет original owner. Aggregate
  admission/verified rollback reject flag. Новых очередей/таймеров/UI-network API нет.
- Product report называет только узкий helper/expected IPv4 и отсутствие общего
  допуска. Same-image TUN OS control всё ещё unavailable, private worker
  activation явно reject TUN; **ни одного general permit не добавлено**.
- Дополнительный source trace: scoped TCP пока делегируется stock Router и его
  rules. Binding config/selector и ожидаемый IP не доказывают actual selected
  outbound именно этого native flow (например, process split-direct). До joint
  proof нужно привязать реальный delegated flow/upstream retirement к protected
  выбранному outbound. Prepared observation этого не утверждает; product copy
  явно отделяет IPv4-ответ от неподтверждённого выбора выходного канала.

Source8 Go/4 docs; gofmt4+3+5+1 exit0 и manual source diff. Tests/fixtures/build/
typecheck/parser/SDK/helper/native/admin/live/browser не выполнялись. Нужна
приёмка active owner/cancel/Stop/reload/rollback/clock/epoch/update/partial cleanup,
original native ABI/WFP/TUN/Job/socket/inflight/resources и final admission.
Evidence: `.artifacts/memory/tun-join-source-evidence.json`.

### V2-201 — selected native-flow и original I/O custody, source/unverified

- Только fixed native TCP scope получает captured actual selected outbound из
  original box. Tag/type/TCP capability сверяются с original guarded selector и
  нормализованным графом, который уже прошёл protectedRuntimeConfig. Новая
  whitelist/trust policy не создаётся. Fresh минимальная InboundContext не несёт
  detour/sniff/process/network-fallback overrides. Конструкторские SDK services
  и operation cancellation сохраняются через существующий workerOutboundContext.
- Scoped PreMatch возвращает обычную stack preparation без Router/rules; scoped
  TCP DialContext вызывает captured adapter. Поэтому process split-direct или
  другое ambient правило не перенаправляет этот diagnostic. Admitted ordinary
  traffic использует прежний stock Router; UDP/mixed/second flow не получают
  диагностическое разрешение, direct fallback отсутствует.
- `nativeTUNFlow` владеет original ingress и upstream сразу после Dial return,
  включая nonnil+error и поздний возврат после cancel. Исходные once-close
  wrappers не дают unwrap/replaceable shortcut обойти receipt; после closing
  новые Read/Write отклоняются. In-flight I/O всё ещё требует завершения.
  Initial SetDeadline/handshake сериализованы с original Close: pinned gVisor
  может создавать lazy endpoint из этих методов, это не просто value setter.
- Переиспользован pinned sing0.8.13 `bufio.CopyConn`: его task.Group.Run ждёт обе
  copy tasks после cleanup. Обычный SDK NewConnectionEx/onClose сам по себе не
  служит таким receipt: route.ConnectionManager вызывает callback до последнего
  common.Close. Здесь done наступает после join, original closes и возврата
  единственного callback; panic/unknown close остаются restrictive.
- Scope deactivation/expiry проходит под admission.mu; ожидание flow/callback и
  actor Close — вне этого lock, сериализовано original cleanupMu. После failed
  cleanup тот же scope/flow/actor остаётся владельцем. Timeout ограничивает done
  wait, не произвольный native Close; некооперативный Dial/copy/Close/callback
  требует retirement исходного worker Job, а не запуска преемника.
- Private scope-close ACK теперь требует bound actual adapter, обмена в обе
  стороны на обеих сторонах, done/cleanup, актуальных actor/epoch/deadline и
  unchanged subject/tag. No-flow/one-way/queued callback/partial close — не ACK.
  Даже этот receipt не forwarding authority: parent ещё проверяет bounded
  helper HTTP200/expected IPv4 после полного original Job/I/O/image retirement.
- Worker IPC и internal argument повышены до4: V3 ACK не принимается за новый
  I/O receipt. Same-EXE production factory/dispatch используют одну константу;
  отдельный native helper protocol1, pair update protocol и bindings не менялись.
- Product copy отражает только scoped IPv4/selected output и закрытый общий
  трафик. Same-image OS control/consumeLocalForwarding всё ещё reject TUN.

Source6 Go/4 docs; captured gofmt exit0/manual diff/source identities only. Нет
tests/fixtures/build/typecheck/parser/SDK/helper/native/browser/admin/live.
Не доказаны runtime routing, half-close/early handshake compatibility, callback/
timer/Stop races, whole-operation deadlines, underlying transport tasks/kernel
state/ABI/OS/ресурсы или улучшение скорости. Изменённый private protocol и старые
fixtures требуют будущей приёмки после разрешения; тесты сейчас не изменены.
Evidence: `.artifacts/memory/tun-outbound-source-evidence.json`.

### V2-202 — private parent/worker native handoff, source/unverified

- После exact selected-flow/I/O scope-close worker хранит один completed receipt
  subject/tag/epoch/nonce с local TTL30s. Он ещё не confirmed и не является
  permission: worker сам не проверял plaintext HTTP и full helper retirement.
  Worker open/selector change/close/new claiming worker probe инвалидируют прежний факт.
- `ObserveNativeTUN` создаёт private opaque source только после прежних full
  helper/filter/Job/I/O/image retirement, bounded HTTP reparse и same-runtime
  gate. Копии observation разделяют один burn owner; Runtime/Egress getters,
  public ProbeReport, nonce или bool не создают source. Default/forged value
  не принимается native-aware probe constructor как обычный probe fallback.
- Parent claim связывает original engine/process instance/scope-close receipt,
  subject/tag/epoch, origin cancellation channel, незавершённую операцию и
  semantic expected IPv4. Burn предшествует outcome; budget rejection/неверный
  subject/другой origin/replay/expiry тоже расходуют попытку. Lock order Engine→
  source; callbacks/native work не идут под source lock. Контекст без Done
  может дать observation-факт, но не lifecycle handoff.
- Private `tun_native_confirm` сообщает исходному worker только canonical IPv4
  и exact completed nonce. Worker сверяет свой actual flow receipt, closed phase,
  epoch/subject/tag/freshness, не продлевает TTL. RPC ACK проверяется до/после
  original process/context; uncertainty сохраняет original retirement owner.
- Следующий claiming worker probe забирает подтверждение до verdict, даже если
  позднее провалится. Wrong nonempty nonce/expectation или stale не принимаются.
  Parent держит один original Engine lock через confirm и consuming probe: другой
  RPC не встаёт между ними. Plain/background probe во время active scope или
  fresh pending native handoff получает unavailable без расходования чужого
  receipt и без сетевых запросов; это устраняет найденный source interleaving.
  Missing source/nonce никогда не выдаёт positive native result. После outbound checks повторяются subject/tag/epoch/context и
  freshness, затем появляется отдельный required `Native TUN flow` result.
  Никакие nonce, process handles или digest не копируются в product report.
- Native-aware factory получает только opaque source из actual root lifecycle;
  public/custom report не подменяет его. Core cleanup fault проходит через finite
  CleanupUnconfirmed → existing Manager no-churn gate без raw error в UI.
- IPC5/argument5 отделяют новый confirmation/probe contract от старых ACK.
  Native helper protocol1, pair update protocol и generated bindings не менялись.
  Root report разделяет native flow и повторную проверку closed WFP/intent;
  использован прежний presenter/словарь, без новых cards/UI-network API/store/dependencies.

**Ограничение:** same-image OS control всё ещё required/unavailable в TUN;
consumeLocalForwarding явно reject TUN. Новый receipt только diagnostic fact,
не подтверждение всей WFP-защиты, не Ready и не platform grant. Fake policy
сама не выдаёт forwarding authority. Нельзя снять этот guard только по новому
report/nonce/наличию receipt или сделать AllowDirectEgress=true как обход.
Следующий шаг — required/informational contract, включая periodic admission
без нового native helper и без refresh старого native proof, затем compiled
platform final commit с fresh App/Manager/WFP/ingress phase и no partial success.

Source9 Go/1 TS label/4 docs; captured gofmt exit0/manual source diff only. Нет tests/
fixtures/build/typecheck/parser/SDK/helper/native/browser/admin/live. TTL/replay/
concurrency/ABA/clock/Stop/reload/old-worker/fixtures/serialization/native/OS и
финальная приёмка остаются OPEN. Evidence: `.artifacts/memory/tun-commit-source-evidence.json`.

### V2-203 — required/informational и admitted periodic, source/unverified

- `ProbeResult.requirement` — только finite metadata, не authority. Missing/empty
  остаётся required. Informational допустим только для exact same-image OS
  comparison с поддерживаемой схемой, `Passed=false/status=unavailable`.
  Неверная роль не понижает native/HTTPS/expected/DNS/safety check: required
  unavailable. UI mapper также фиксирует rejected role как required до изменения
  статуса: повторная нормализация не превращает её обратно в informational.
  Отчёт из одних информационных шагов не успешен.
- Shared value normalization проходит worker bounds, private `probePassed`,
  root copied report/summary и оба failure classifiers. Independent leak,
  protection/custody/trust/stale flags не снимаются. HTTPS/expected/consistency/
  resolver checks остаются required; HTTP IP relation не local-safety proof.
- OS request/общий request-byte budget сохранены, `AllowDirectEgress` не
  используется как обход. В TUN информационный запрос получает child deadline
  в половину общего timeout, чтобы не ждать специально до общего deadline.
  Caller cancellation/общий deadline по-прежнему делают отчёт неуспешным.
  Это исходная квота, не измеренная latency или гарантия прерывания транспорта.
- Private `workerTUNProbeBinding` создаётся только original worker под
  Engine→admission locks после actual phase/subject/tag/epoch. Для closed lifecycle
  он требует fresh consumed original native receipt и повторяет freshness/closed
  phase после outbound checks. Public/standalone TUN plan без него unavailable.
- Для admitted periodic берётся actual matching gate.open/subject/tag/epoch,
  повторно сверяемый после checks. Это named `TUN admission`, а не новый native
  flow; receipt/TTL не переиспользуются и не обновляются. Нет нового helper,
  issue/consume/parent permit. Plain unadmitted/busy/invalid handoff не стартует
  outbound I/O. Existing root `verifySessionProtection`/WFP Observe остаётся
  самостоятельным после-probe gate; worker report не подменяет WFP.
- Initial TUN ещё получает required `TUN activation unavailable`; ordinary
  issuer не выдаёт TUN token, consume всё ещё reject TUN. Этот явный остаток
  нельзя убрать только по report/role/phase label. Нужен expiry-bound original
  native+outbound issuer и final compiled platform/Manager phase commit.
- IPC6/argument6 запрещает принимать старый aggregate как новый role contract.
  Helper protocol1/pair update/bindings не менялись. Additive optional role
  переносится Go→existing UI mapper/presenter; есть текстовая пометка, semantic
  list и прежние токены (modern-web-guidance accessibility), без новых cards,
  store/API/dependencies. Unknown/malformed roles/metadata не дают UI success.

Source12 Go/3 TS/1 Vue/4 docs; gofmt/manual diff/source SHA only. Нет tests/fixtures/
build/typecheck/parser/SDK/helper/native/browser/admin/live. Protocol/legacy fields,
serialization/roles/deadlines/TTL/epoch/replay/cancel/concurrency/Stop/actual OS/
native/visual/resource/release приёмка OPEN. Evidence:
`.artifacts/memory/probe-requirement-source-evidence.json`.

### V2-204 — original native prepared issuer и one-emission IPC, source/unverified

- Только actual worker после завершённых native+outbound checks вызывает issuer
  своего private TUN binding. Original native receipt помечается spent до любого
  исхода, включая cancel/validation/entropy failure. Копия binding не выдаёт новый
  token. `until=min(original native.until, now+5s)` задаётся до entropy, повторно
  проверяется до записи. Confirmation/emission не продлевают original deadline.
- `workerTUNPreparedPermit` — отдельный actual slot с exact subject/tag/epoch,
  native pointer/nonce, local deadline и one-way emission bit. Ordinary issue/
  consume не открывают его. Emission после probe повторяет actual worker subject/
  phase/epoch/freshness/slot identity, burn предшествует outcome. Неподтверждённый
  process/pipe результат по-прежнему ведёт к original owned retirement.
- IPC7 `native_admission_token` допустим только в exact native-claiming probe ACK:
  canonical token, matching subject/tag/epoch, closed phase, no ordinary token/
  ScopeNonce и непустой report. Universal exchange gate отвергает поле на hello,
  control/error/non-native reply. Parent дополнительно требует свой original
  opaque source/process/instance/scope nonce/context и actual required verdict.
  Успешный unadmitted TUN ответ без отдельного token не принимается.
- GUI хранит prepared lane отдельно от ordinary `forwardingPermit`. Token и
  source не попадают в public ProbeReport/UI. Parent добавляет required
  `TUN activation unavailable`, сохраняя negative/custody/trust/stale flags.
  Manager поэтому не проходит общий допуск; consume TUN всё ещё отвергает.
  Final consume именно этого prepared record и compiled WFP/Manager commit ещё
  не реализованы. Нельзя заменить их field/report/bool или ordinary admit.
- Central parent/gate revocation покрывает original lifecycle/retirement/selector/
  scope/new claiming probe/ordinary consumption failure. Plain/background checks
  не стирают live native preparation и не делают I/O. GUI busyUntil — только
  bounded no-steal window; он не является worker deadline или native freshness.
  Periodic admitted path по-прежнему не выдаёт/не обновляет native token.

Source10 Go/4 docs; captured gofmt/manual diff/SHA only, без tests/fixtures/build/
typecheck/parser/SDK/worker/helper/native/browser/admin/live. Protocol7 compatibility,
entropy/cancel/deadline/TTL/replay/slot ABA/one-emission/Stop/reload/concurrency,
actual native/OS/resources/SDK/durability/release остаются OPEN. Нет новых timers,
helper owners/network requests/dependencies или generated edits. Evidence:
`.artifacts/memory/native-issuer-source-evidence.json`.

### V2-205 — guarded final native consume/platform/Manager join, source/unverified

- Compiled `nativeTUNForwardingAccepted=false` сохраняет закрытую поставку.
  Runtime settings/JSON/linker override/public report не меняют const. Пока gate
  false, normal Establish не запускает native helper, Manager не принимает
  prepared-only verdict и worker не включает TUN. Future gate/source changes
  требуют свежего evidence именно для изменённых входов, не старого source hash.
- `NativeForwardingProbe` возвращает opaque `NativeTUNAdmission`, созданный
  только actual original Engine после exact native ACK. Manager создаёт private
  commit context для своей active owner/generation/engine/origin Done. Zero/
  copied-stale receipt, public/interface success, чужой контекст или generic
  report не заменяют actual private prepared source и required worker facts.
- Pending eligibility допускает ровно один exact value-only activation step,
  без скрытых observations/comparison/attempts. Independent protection/custody/
  trust/leak/stale flags не очищаются. Pending phase сама по себе не синтезирует
  ProtectionUnavailable. Обычный `probePassed` всё ещё отвергает pending; positive
  final report создаётся только после receipt/current runtime/lifetime checks.
- IPC8 `admit_native` принимает отдельный token+original nonce/subject/tag/epoch,
  не ordinary token. Worker берёт exact original slot, сохраняет validity до
  удаления и сжигает slot до любого исхода; повторяет emitted/native+5s expiry/
  context/actual graph/closed phase/accepted build. Ordinary admit TUN запрещён.
  Parent burns обе lanes перед RPC; uncertainty сохраняет original retirement.
- Root использует one-shot shared slot вместо копии mutex в value probe.
  Только concrete compiled strict policy, свой App/candidate/source/pin/clock/
  trust/intent/network/Manager/LUID проходят precheck. Explicit phases сравнивают
  полный original RuntimeIdentity, меняя только ожидаемый ForwardingAdmitted.
  IPC/native callbacks никогда не выполняются под runtime/Manager/App/epoch locks.
- Existing native writer переиспользован: exact previously observed LUID и
  strict closed-policy read внутри той же write transaction. Затем повторяются
  WFP-ready/worker-admitted checks до/после external work. Attempt отмечается
  до вызова writer. Error/unknown/panic вызывает restrictive cleanup того же
  owner с отдельным uncanceled deadline/full closed-policy read; Release не
  вызывается. Cleanup panic/failure возвращает typed custody uncertainty.
- Start/select/Reload/rollback используют общий prepared eligibility и commit;
  LastProbeReport/verified rollback получают final report. Late cancellation
  между финальными checks и Manager.finish/Ready остаётся следующим source
  аудитом; broad native/OS/concurrency/resource приёмка не заменяется этим кодом.

Source12 Go/4 docs; captured gofmt/manual source diff/SHA only. Нет tests/fixtures/
build/typecheck/parser/SDK/helper/worker/native/browser/admin/live. IPC8/receipt/
context/roles/TTL/replay/entropy/panic/compensation/Stop/reload/clock/phase/LUID ABA,
native ABI/WFP durability/interprocess/resources/SDK/release/comparative и полный
PC-00…09/old9 остаются OPEN. Evidence:
`.artifacts/memory/native-commit-source-evidence.json`.

### V2-206 — terminal decision, own cancel и Ready context, source/unverified

- Просмотр текущих исходников подтвердил порядок: finish вызывал own cancel до
  state publication; observer release (LIFO defer) закрывал child ctx ещё раньше;
  Switch отменял ctx до successful/verified rollback return. Поэтому нельзя
  просто добавить ctx.Err после прежних self-cancel и считать его caller cancel.
  Race запуском не воспроизводилась — тестирование запрещено.
- Common `finishLifecycle` повторяет lifetime текущего applied probe вне Manager
  locks (включая restored-old constraints). Actual active owner, ordinary/trust
  cancellation, deadline с delayed timer delivery и registered Stop проверяются
  перед positive terminal decision. Stop записывает свой intent bit под тем же
  Manager.mu; own cancel/release выполняются после зафиксированного outcome.
- При late fault original active operation остаётся владельцем до возврата
  restrictive cleanup вне Manager lock. Последний positive report становится
  stale/failed, independent flags сохраняются, cleanup uncertainty typed. Только
  затем terminal StateError освобождает ожидающий Stop; replacement не получает
  владение ресурсами по одному timeout. Это не OS/syscall hard-bound proof.
- Invalid terminal outcome снимает owned verified rollback marker/report из
  ошибки, сохраняя failure causes и independent joined cleanup/trust branches.
  Успешный rollback остаётся verified только при принятом terminal decision.
  Root cancel/superseded error branches также снимают stale rollback marker,
  сохраняя independent cleanup/trust причины вместо замены всего на Canceled.
  Panic details теперь используют тип, не arbitrary callback payload.
- Root Start/selected/rollback Ready получает original live App ctx; normally
  retired Manager child ctx не используется как publication authority. Context/
  deadline проверяются до и внутри существующего runtime→Manager→App→epoch→clock
  value commit. Contextless lifecycle Ready отвергается. Health/recovery/unsafe
  evidence продолжают прежний scoped маршрут; network/native I/O под publication
  locks не добавлены. Все три actual lifecycle publication call sites обновлены.
  Отдельный rejected-without-mutation маршрут также проверяет actual retained
  owner/session/old evidence epoch/current context/trust перед running. Он не
  retag-ит исторический report и не обновляет health/probe time/HEALTHY.

Source5 Go/4 docs; gofmt/manual diff/SHA only, без tests/fixtures/build/typecheck/
parser/SDK/helper/native/browser/admin/live. Caller cancel vs normal retirement,
deadline/Stop/trust/rollback/panic/cleanup/source-owner/concurrency/publication/
UI/native/OS/resources приёмка OPEN. Native acceptance const=false, IPC8 и весь
PC-00…09/old9 сохранены. Evidence:
`.artifacts/memory/terminal-outcome-source-evidence.json`.

**Остаток W4c:** согласованная приёмка terminal cancel/Stop/Ready и полного
Windows/WFP/процессного пути before
release gate. `ForwardingAdmitted` phase и native proof не являются доказательством
неизмеренных native DNS/UDP/IPv6 или безотказности. V2-198 переносит прежние SDK calls в original
native owner; native ABI/resources/cleanup/atomicity, межпроцессная platform lease
и marker/storage остаются gates. **TUN unavailable**, all9
OPEN, public schema2 unsupported. Tests/fixtures/build/typecheck/parser/SDK/
helper/native/admin/live/EXE не выполнялись.

Дополнительные первичные источники:

- [Microsoft DuplicateHandle](https://learn.microsoft.com/en-us/windows/win32/api/handleapi/nf-handleapi-duplicatehandle):
  original object и target table, explicit rights/noninheritance. Source read не
  подтверждает actual native ownership, Close failure или process death ordering.
- [Microsoft GetProcessTimes](https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-getprocesstimes):
  query rights/creation identity. Exit time живого процесса не используется.
- [Microsoft Job basic limits](https://learn.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-jobobject_basic_limit_information)
  и [extended limits](https://learn.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-jobobject_extended_limit_information):
  применимость fields определяется flags; лимит committed memory не равен RSS.
- Pinned local `sing-box1.13.19 protocol/tun/inbound.go`, `sing0.8.13 metadata/addr.go`
  и `x/sys0.44.0 windows` API/amd64 types прочитаны только как source.

Первичные источники и границы:

- [Microsoft SO_EXCLUSIVEADDRUSE](https://learn.microsoft.com/en-us/windows/win32/winsock/so-exclusiveaddruse):
  option до bind, wildcard reservation semantics. Это не original-process proof.
- [Microsoft socket handles](https://learn.microsoft.com/en-us/windows/win32/winsock/socket-handles-2):
  generic DuplicateHandle не является поддержанным socket-transfer контрактом.
- [Go1.26 file_windows.go](https://github.com/golang/go/blob/go1.26.0/src/net/file_windows.go)
  и [FileConn](https://github.com/golang/go/blob/go1.26.0/src/net/file.go):
  library socket duplication/copy semantics. Проверка pinned source API не
  подтверждает реальное поведение provider/IOCP/Close/port custody на Windows.
- [WFP condition identifiers](https://learn.microsoft.com/en-us/windows/win32/fwp/filtering-condition-identifiers-)
  и [AppContainer model](https://learn.microsoft.com/en-us/windows/win32/secauthz/implementing-an-appcontainer):
  AppID/package SID — разные признаки; они сами по себе не original process handle.
  Новый AppContainer/profile/token/dependency в проект не добавлен.

- До general TUN/LUID permit нужен узкий diagnostic actor, привязанный к actual
  original helper process/image/Job, не имени curl/PID/listener/public token alone.
  На user ingress до proof не должны проходить unrelated TCP/UDP/ICMP flows.
- Start/switch/Reload/rollback требуют revoke и fresh exact proof; нельзя открыть
  кандидат по старому report. Сам atomic flag не отзывает уже переданный upstream
  flow и не устраняет гонку check→route; actual in-flight retirement/epoch/old
  admitted streams и selector carry lifetime надо решить в этом же контракте.
- Не оставлять native direct-route bypass за пределами последующего revoke.
  Если seamless нельзя сделать с доказанной границей, он не получает молчаливый
  успех: нужен существующий owned reload/retirement путь, затем полный fresh gate.
- Только после узкого OS-path verdict и final admitted subject выдавать общий
  permit; failure/cancel сохраняют strict barrier, explicit Disconnect отдельный.

### W4d. Приёмка после разрешения

Все OPEN: zero user payload до допуска; known/unknown inbound и injectable/UoT/
PreMatch/redirect/native protocol paths; source/factory registry isolation;
duplicate/malformed endpoint/roles и zero secret calls; original worker probe
subject/request/reply/expiry/single use/replay/partial/overflow; unexpected listener;
current default/old worker/Stop/cancel/switch/reload/rollback; native TCP/UDP/ICMP/
IPv4/IPv6/DNS/route/ACL/interprocess/crash/resources/old-client и actual UI Ready.
Source no-op/shape/hashes/gofmt не являются native proof.

## Evidence / границы

V2-196 `.artifacts/memory/tun-scope-source-evidence.json`:11 Go/3 docs,
original7/new4 source delta/before10 и native/pinned TUN source receipts;
gofmt11+1+1+1+2 (до прерывания) и current1 exit0 only. Scope begin/end ещё не вызваны production parent FSM;
WFP filter и native proof не реализованы, TUN unavailable. Нет tests/fixtures/
build/parser/SDK/helper/native/admin/live. Все прежние обязательства сохранены.

V2-195 `.artifacts/memory/tun-actor-source-evidence.json`:6 Go/3 docs,
original2/new4 diff, before5, pinned Go/x-sys/wf primary source receipts;
gofmt6+2+3 exit0 only. Source-side codec/helper mode и public endpoint alias —
реализация части W4c. Parent/engine-handle/ingress/WFP/native-proof/final activation
ещё не подключены; production TUN по-прежнему явно unavailable. Protocol/socket/
TLS/IOCP/alias/port lifetime/failed connect/watchdog/kill/Job/old-helper/build/native
и все остальные gates открыты. Никакого native/helper/SDK/parser/test исполнения.

Исторический V2-193 `.artifacts/memory/forwarding-admission-source-evidence.json`:6 Go,
original5/new1 diff, docs, pinned upstream source receipts. Gofmt6 + current2
captured exit0 only. Raw web cache misses, direct 404 для неверных route.go/
connection.go и TUN download timeout не отмечены как успешное чтение; correct
router/mixed/service source и cached pinned TUN page/tag read указаны отдельно.

V2-194 `.artifacts/memory/forwarding-worker-source-evidence.json`:13 Go/3 docs,
original11/new2 source diff и original14 before copies, retained config/render/TUN
source identity, pinned outbound/selector/router/mixed/service reads. Gofmt13+2+2
exit0 сохранён как форматирование; ручное source чтение исправило epoch race во
время пробы, revoke/consume interleaving, потерю SDK services при dial и сохранение
старого parent permit при budget rejection. Это source findings, не воспроизведение
или доказательство исправления запуском. Read-only diff/SHA не implementation PASS.

Нет tests/fixtures/build/parser/vet/typecheck/SDK/crypto/native/admin/live/OS/
browser/install или настройки сети. Owner values не читались, другие роли/chats/
shared/configs/harness/root tools не изменялись. W3b/NETGUARD-001, SDK native/parity,
durability/restart/OS/DR/resources/old-client/service/release/comparative/all9 gates
по-прежнему открыты; schema2 public update unsupported. W4 не завершён.

<a id="pc-v280-prematch"></a>

## V2-280 — PreMatch не создаёт native destination в обход guarded handlers

2026-11-06. PC-01/W4c, source/unverified; полный PC00…09/старые9 активен.

### Подтверждённая по исходникам причина

Прежний admissionRouter передавал admitted flow в SDK PreMatch с последним false
и комментировал это как отключение native direct-route bypass. В закреплённом
sing-box1.13.19 route/route.go PreMatch такой flag запрещает один bypass outcome,
но explicit RuleActionBypass с outbound и RuleActionRoute при ненулевом context
по-прежнему выбирают DirectRouteOutbound.NewDirectRouteConnection. Для ICMP без
выбранного правила он вызывается у default outbound. Такая destination живёт
вне наших TCP/packet handlers. Это source defect; native воспроизведения нет.

### Реализация в существующих компонентах

1. admissionRouter.PreMatch не вызывает SDK PreMatch вообще и не создаёт native
   destination. Только явные TCP/UDP families после existing admission check
   получают zero destination: дальнейший проход через стек/typed router. Перед
   допуском narrow native diagnostic по-прежнему только captured TCP scope.
2. Existing actual SDK RouteConnectionEx/RoutePacketConnectionEx/routing rules
   сохранены. Отложено только предварительное matching; ordinary routing policy
   не копируется в свой router. Existing native diagnostic продолжает идти через
   свой captured selected outbound, а не ambient split/direct rule.
3. ICMP/unknown/empty network в PreMatch отклоняются: нет защищённого owned native
   handler для них. Это временная недоступность требуемой native family, не её
   реализация/приёмка; полный protocol scope остаётся в W4d/PC-01. Нельзя вернуть
   native bypass ради успешного ping или открыть compiled TUN acceptance gate.
4. scopedTCP отвергает nonempty non-TCP metadata даже после общего допуска;
   allowedPacket отвергает nonempty non-UDP. Empty family в actual typed handler
   допускается по прежнему контракту SDK и затем нормализуется его кодом.
5. Nil packet input возвращает отрицательный результат без вызова nil.Close;
   Ex вызывает supplied callback с отказом и не делегирует в SDK. Не добавлены
   новый transport/registry/owner/setter/permission, lifecycle authority прежняя.

### Подтверждение и ограничения

Manual2 source diffs, typed handlers/nil/callback/admitted/closed/diagnostic paths;
local pinned source route PreMatch/route TCP+packet, adapter.Router и TUN inbound
PrepareConnection с network metadata.4 exact source SHA сохранены отдельно; cache
не изменялся. gofmt2 exit0.6 existing originals/task71rev650 archive; исходный
общий PLAN и эта task сохраняют byte prefix, shared/Android/чужие области прежние.
100 acceptance OPEN/56 checks NOT_RUN exact/no waiver.

Нет tests/fixtures/build/compile/vet/typecheck/product parser/check/doctor/install/
gen/SDK/helper/crypto/native/browser/admin/live/network исполнения. Source byte
учёт и форматирование не доказывают реальные TUN/Windows/WFP/protocol свойства.
PreMatch zero destination не доказывает отзыв уже делегированных ordinary SDK
flows, I/O/upstream/socket/callback/Close retirement или отсутствие check→route
гонки. Следующий обязательный срез — original per-box ordinary TCP/UDP ingress
ownership/revoke с сохранением actual SDK policy; upstream callback/legacy API
return не подменять native/physical cleanup, unknown owner не забывать.

OPEN: защищённые ICMP/другие native families, DNS/IP-family/split-policy semantics,
Go/native resource limits/performance, route-matching timing/compatibility,
root/browser/whole-chain control, ordinary pre-reservation Stop wait, helper/WFP
final traffic admission и весь PC00…09/old9/SDK/ABI/OS/UX/packaging/release/comparative.
All gatesfalse/ranBeforeJob/GUI6R1U1/IPC9/Job8/TUN unavailable/schema2 unsupported.

Урок: параметр supportBypass не является универсальным запретом native route.
Нужно проследить actual outbound constructor и возвращаемый resource; unsafe
preparation обходится through-stack path с policy в настоящих typed handlers.

<a id="pc-v281-ingress"></a>

## V2-281 — Учёт исходных TCP/UDP-потоков и две фазы закрытия SDK

2026-11-06. PC-01/W4c, source/unverified; полный PC00…09/старые9 активен.

### Почему прежнего флага недостаточно

После общего допуска actual SDK Router получал обычные соединения напрямую.
open=false запрещал последующие проверки, но не отменял уже переданные ingress
и не учитывал их Close/I/O/callback. Закреплённый legacy route API может вернуться
по отмене box context до окончания callback. Нельзя считать этот return очисткой.
Existing nativeTUNFlow решает отдельную captured diagnostic TCP-задачу и не
подходит вместо обычной routing policy для всех пользовательских соединений.

### Компоненты и последовательность

- forwarding_routes.go: original per-box receipt, registration/revoke, route/SDK/
  external callback/raw-I/O/Close/task fences и local join. forwarding_connections.go:
  typed TCP/packet guard и сохранение handshake. Existing admissionRouter остаётся
  входом; настоящий SDK Ex Router продолжает применять routing rules.
- Под gate.mu регистрируется original epoch/subject/selector/ingress/context до
  SDK или AfterFunc. Revocation под тем же mutex закрывает регистрацию и снимает
  snapshot exact records. Все original contexts отменяются до ожидания. Проверка
  непосредственно перед delegation и перед raw calls дополняет это владение;
  это не обещание атомарной остановки уже вошедшего OS вызова.
- Initial budget4096 route receipts включает live/pending/failed и detour entries.
  Healthy complete удаляются; failed/unknown сохраняются. Превышение не даёт
  bypass. Это новый фиксированный safety limit, не измеренный предел нагрузки,
  hard byte/RSS quota или доказательство максимальной скорости.
- Каждая запись хранит raw ingress и ровно один original Close attempt/actual
  return. Повторный или вложенный Close во время первой попытки возвращает
  pending, не ждёт себя и не получает новое доказательство через ErrClosed.
- Один standard context.AfterFunc вызывает original close task; stop/замена
  callback не используются. Отдельно учитываются raw call entry/return, Close
  stack/actual return, SDK callback, external callback и delegation frame. Panic/
  Goexit не дают success: local fault отзывает gate, original record удерживается;
  raw I/O/delegation panic повторно выдаётся как конечный local fault без payload.
- SDK callback только фиксирует свой result и отменяет context. External callback
  доставляет original close task после confirmed raw Close, завершения вошедших
  raw calls и SDK callback. Это избегает recursive Close/callback self-join и
  преждевременного освобождения ingress. При unknown этапе callback не имитируется;
  receipt остаётся pending. Callback thread/timing изменены и требуют приёмки.
- TCP/UDP wrappers не раскрывают Upstream/replaceable raw interface. Handshake
  success/failure, peer-aware TCP/packet handshake, TCP early hints и additional
  read-deadline hint делегируются оригиналу. Deadline/handshake сериализованы
  с Close отдельным control mutex; value/gate mutex не держатся на SDK/native IO.
  UDP local rejection освобождает только ещё не переданный writer буфер. После
  actual WritePacket никакой повторной догадки о Release не делается.
- Первое local Close/callback/IO custody failure seals box gate и отменяет его
  original routes одним sweep. Обычная возвращённая transport error сама по себе
  не становится custody fault/отрицательным channel-health фактом.
- Actual admissionBox.Close теперь сначала seals/cancels, затем пытается existing
  bootstrap Close и original SDK Box.Close и лишь после этого drains receipt-
  obligations одним existing5s budget с final same-done re-read. Иначе SDK callback
  мог ждать того самого Box.Close, которое блокировалось ожиданием callback.
  Timeout не отменяет Close и не выдаёт success; exact owners остаются. Legacy
  API использует тот же Ex path и bounded post-callback drain вместо чужого return.

- Existing worker probe проверяет local route fault до diagnostic work и в
  deferred negative normalizer. Actual original epoch/subject сохраняются даже
  при cancellation/failed HTTP/token issuance; cleanup_unconfirmed и независимый
  trust fault идут через прежний finite report. Permit отсутствует, IPC прежний;
  fault в route не становится channel FAIL. Нет нового push-monitor: родитель
  получает факт через existing probe, actual latency/IPC/native остаются OPEN.

### Допустимое подтверждение

Manual4 final source diffs, all4 entry points, native diagnostic separation,
registration/close/callback reentrancy/value-control lock order, partial cancel/
missing callback/panic/Goexit/normal return и UDP ownership. gofmt3+1+2+2 exit0,
только4 touched Go files.8 snapshots:6 существовавших файлов +2 absent paths;
Initial7 manifest сохранён до необходимого8-го snapshot engine_probe, снятого
перед его изменением; task71rev652 архивирован.7 pinned SDK source SHA +Go context.go/VERSION snapshot:
local Go1.26.7 AfterFunc запускает callback отдельно; stop не был join receipt.
SDK/cache/runtime source не изменялись. Byte accounting не проверка выполнения.

Нет tests/fixtures/build/compile/vet/typecheck/product parser/check/doctor/install/
gen/SDK/helper/crypto/native/browser/admin/live/network исполнения.100 acceptance
OPEN/56 checks NOT_RUN без waiver; ни один новый runtime/native PASS не заявлен.

### Незакрытое

1. Rejection до registration, включая cap exhaustion/old negative close paths,
   пока не получает новый original receipt: original inbound/root ownership
   нужно проследить и связать отдельно. Не объявлять его подтверждённым.
2. SDK/inbound/root могут держать raw alias и закрыть его раньше нашего owner.
   ErrClosed не повышается до proof; возможен conservative false failure. Нужна
   ранняя custody/inbound integration и native race acceptance, не retry Close.
3. Local done доказывает только предусмотренные code fences, не retirement
   скрытого upstream socket/SDK goroutine/kernel I/O/whole Job. SDK Box.Close и
   original process/Job ownership остаются обязательными частями общего teardown.
4. Generic wrapper может убрать copy/splice optimizations и меняет callback
   scheduling; callback/handshake, UDP loss/buffers, detours, throughput/CPU/RSS/
   concurrency/cap/resource ceilings не проверены.5s ограничивает observation,
   не native Close/SDK Box.Close/control mutex/callback или полное время Stop.
5. Main native family/WFP/whole-chain/root/browser control, ordinary Stop до
   terminal reservation, SDK/ABI/OS/UX/packaging/release/comparative и весь PC00…09/
   старые9 сохраняются. TUN unavailable/schema2 unsupported, all gatesfalse/
   ranBeforeJob/GUI6R1U1/IPC9/Job8 прежние. Не включать путь по одному local receipt.

Практический урок: сначала закрыть регистрацию и отменить все original tasks,
затем закрыть их SDK owner и лишь после этого ждать callbacks, зависящие от него.
Callback, raw Close и фактический upstream retirement остаются разными фактами.

<a id="pc-v282-rejection"></a>

## V2-282 — исходное владение отказом до допуска

**Режим:** PC-01 source/unverified; весь PC-00…09 и прежние девять этапов активны.
Работа возобновлена после возврата диска D:. Запрет тестирования сохранён.

### Почему требуется отдельный учёт

В pinned sing-box1.13.19 listener запускает NewConnectionEx отдельно для каждого
accepted TCP socket. Listener.Close закрывает tcpListener/udpConn/systemProxy,
но в просмотренном пути не соединяет handler tasks и не закрывает каждый уже
принятый socket. Mixed inbound передаёт в listener собственный handler во время
конструктора: wrapper возвращённого Inbound сам по себе не перехватит handshake.
Поэтому ordinary route receipts из V2-281 не покрывали отрицательные Router пути.

### Изменения исходников

- All4 legacy/Ex TCP/UDP отрицательных entrypoints используют один rejectIngress.
  Original raw/callback/done публикуются под gate.mu до первого Close/callback.
  Nil socket и отсутствующий callback не создают фиктивной resource acquisition.
- Separate initial4096 budget включает pending/failed rejection records. Original
  caller выполняет Close/callback синхронно, без новой per-rejection goroutine.
  Actual entry/return и original result отделены от отрицательного route outcome;
  только подтверждённый успех cleanup удаляет запись. Panic/Goexit/error сохраняют
  original owner, отзывают gate через существующий finite local-fault путь.
- Callback выполняется после confirmed original Close либо known no-resource.
  При unknown Close callback не имитируется. Это изменение timing/поведения требует
  будущей SDK/native приёмки; local done не доказывает physical socket retirement.
- Только прямые private forwardingTCPConnection/forwardingPacketConnection aliases
  получают existing original Close receipt. Same-box bounded acyclic chain checked
  до ожидания. Один original attempt, independent existing5s wait и final same-fence
  reread; concurrent pending/ErrClosed не становятся новой cleanup authority.
  Ordinary raw-wrapper close использует тот же путь. Opaque SDK wrapper не
  разворачивается через Upstream/Cast: он может владеть дополнительным Close work.
- Rejection cohort seals/snapshots только после попытки original SDK Box.Close;
  ordinary и rejection drains используют один cleanup deadline. Late arrival не
  может добавить забытый receipt после final snapshot. Сам SDK/native Close этим
  не становится interruptible или ограниченным по полному времени.
- Late/full record admission вызывает private negative-only worker capability:
  token создаётся только внутри validated serveEngineWorker и несётся original
  Engine lifetime, не caller operation context. Он завершает только child exit4,
  не выдаёт Ready/permit и недоступен через JSON/App setter. Local nonworker harness
  получает finite panic, а не право завершить GUI. Parent stopIsolated уже игнорирует
  ACK как proof и освобождает original owner только после Process exit/IO/whole Job/
  handle reap. Exit не выполняет deferred cleanup; parent reap обязателен.

### Что просмотрено и сохранено

Manual5 final source diffs: forwarding_admission, forwarding_routes, новый
forwarding_rejections, engine, worker_child. Просмотрены all4 negative entrypoints,
known-alias Close arbitration, callback/panic/Goexit/late seal/full budget, value/
control lock order и private child→Engine lifetime→admission capability. Existing
NewIsolatedEngine App factory и stop/retire parent path не менялись.

gofmt5+5 exit0: original batch и повторное форматирование exact5 touched files
после возвращения диска.9 snapshots:8 existing +1 absent; task71rev655 архивирован.
10 неизменённых local SDK/Go source/version identities связывают просмотр с pinned
входами. Source evidence: .artifacts/memory/bootstrap-v282-rejection-source-evidence.json.
Это просмотр исходников и byte accounting, не выполнение приложения/SDK.

Нет tests/fixtures/build/compile/vet/typecheck/product parser/check/doctor/install/
gen/SDK/helper/crypto/native/browser/admin/live/network исполнения.100 acceptance
OPEN/56 checks NOT_RUN; goal/tasks0…70/constraints/decisions сохранены, без waiver.

### Остаток

1. Pre-router accepted sockets и handshake handlers пока не покрыты новым Router
   receipt. Требуется ранняя original inbound/root custody, не wrapper поверх
   уже сконструированного SDK Inbound. Opaque SDK/raw aliases также отдельный gap.
2. Два initial4096 budgets не задают total fd/goroutine/byte/RSS/CPU/throughput
   предел. Синхронный Close/callback и SDK Box.Close могут ждать дольше5s; budget
   ограничивает observation/drain, не уже выполняющийся OS вызов/полный Stop.
3. Child exit4 при late/full — намеренный fail-stop; поведение при легитимной
   нагрузке, ложных отказах и recovery не проверено. Local harness panic не
   является complete embedded production ownership model; App fallback отсутствует.
4. Local done/Close result не доказывают opaque upstream/SDK goroutine/kernel
   cleanup/whole Job. Parent original process retirement остаётся обязательным.
5. Native family/WFP/whole-chain/root/browser control, ordinary Stop, SDK/ABI/OS/
   UX/release/comparative и весь PC/old9 OPEN. All gatesfalse/ranBeforeJob/
   GUI6R1U1/IPC9/Job8/TUN unavailable/schema2 unsupported сохранены.

Практический урок: отрицательный Router outcome тоже приобретает исходный socket/
callback; его нельзя считать отсутствием ресурса или забывать после Box.Close.

<a id="pc-v283-accept"></a>

## V2-283 — исходный mixed TCP socket до Router

**Режим:** PC-01 source/unverified. Весь PC-00…09/прежние девять этапов активны;
запрет тестирования/сборок/исполнения приложения сохранён.

### Архитектура и переиспользование

Возвращённый SDK Inbound wrapper не перехватывает handler, который pinned mixed
constructor уже передал собственному listener. Новый Windows ownedMixedInbound
поэтому использует public listener.ListenTCP для единственного actual bind и
свой original Accept task. Unchanged SDK mixed.NewInbound создаёт processor,
его собственный listener не запускается. SOCKS/HTTP/auth/UoT/UDP handlers не
переписаны; actual routing по-прежнему получает guarded admissionRouter.

Pinned mixed.Start при nil TLS выполняет только listener.Start. В текущем
normalized config.Inbound нет inbound TLS и set_system_proxy; последним управляет
существующий App/Manager. Private SDK options с этими возможностями отвергаются
до resource creation, без молчаливого пропуска их lifecycle. Будущая поддержка
новых inbound options требует отдельной интеграции/приёмки, не data flag.

### Исходное владение

- После actual Accept raw socket входит в per-box input map до чтения адресов,
  metadata/logging, запуска goroutine и SDK handler. Original listener Start/
  native Listen attempt, Accept task и Close имеют собственные сохранённые
  состояния/actual results. Pending Listen не заменяется и не повторяется.
- Initial4096 inputs включают active/pending/failed. При input pressure новый
  socket проходит existing bounded rejection lane без новой handler goroutine.
  Synchronous Close создаёт backpressure в original Accept task. Это уточнение
  первоначального pre-checkpoint: один input cap не требует убивать healthy
  worker; лишь untrackable rejection/full rejection pool или post-seal input
  используют прежний private worker-only fail-stop и original parent reaper.
- Temporary net.Error Accept повторяется в том же retained task с задержкой
  5ms→1s; context/Close прекращают retry. Новый listener не создаётся, tight
  error loop отсутствует. Permanent Accept error/panic/Goexit — sticky local
  fault; actual Accept return отдельно от native listener Close.
- TCP ingress wrapper считает actual Read/Write/address/control calls, исходный
  native Close и handler frame. Deadline/handshake control сериализованы с Close,
  value/gate mutex не удерживаются при raw IO/SDK/callback. Original AfterFunc
  close task не stop/replace; fields проверяются перед success-only removal.
- No Upstream/raw-copy escape. Read/write handshake hints, TCP/packet peer-aware
  handshake и additional deadline hint делегируются original socket; negative
  typed outcomes не выдают forwarding authority. Panic не подменяет partial IO.
- Known mixed wrapper Close в ordinary/rejection paths использует original
  closeWait/result, без второго native Close или ErrClosed-as-proof. External
  opaque aliases остаются отдельным gap; неизвестный wrapper не разворачивается.
- SDK mixed может вызвать callback(nil) после handler return при уже delegated
  потоке. Этот notification не закрывает raw socket и не подтверждает cleanup.
  Для injected onClose external delivery ждёт actual raw Close/entered IO/
  handler/recorded SDK callback. Отказ до SDK синтезирует только отрицательный
  notification, поскольку SDK producer не был создан. Callback scheduling,
  detours и duplicate notifications требуют будущей SDK приёмки.
- Gate close/fault сначала отменяет все captured input contexts. Actual SDK
  Box.Close закрывает listener/processor, затем existing rejection seal закрывает
  и input registration. Exact roots drains делят прежний post-Box5s deadline с
  routes/rejections. Failed/pending owners сохраняются; no timer-as-cleanup.

### Допустимое подтверждение

Manual5 final source/diff и caller/lock review: forwarding_admission/routes/
rejections, новые forwarding_mixed/forwarding_ingress. Просмотрены constructor
option guard, actual listener/processor lifecycle, accepted socket до Go, cancel
до/после scheduling, partial Accept, callback-before-close, reject-before-handler,
native Close once/join, panic/Goexit и bounded pressure/retry. gofmt5+3+1 exit0,
только затронутые Go paths.9 snapshots7existing2absent/task71rev657 архивирован.
16 pinned local SDK/Go source/version/license identities сохранены неизменными;
SDK/cache/runtime source не редактировались. Processor используется как dependency,
не скопирован/не форкнут. Source evidence — bootstrap-v283-accept-source-evidence.json.

Нет tests/fixtures/build/compile/vet/typecheck/product parser/check/doctor/install/
gen/SDK/helper/crypto/native/browser/admin/live/network исполнения.100 acceptance
OPEN/56 checks NOT_RUN, без waiver; source/gofmt не compile/runtime/native proof.

### Незакрытое

1. SOCKS5 UDP associate создаёт native UDP socket до первого packet/Router,
   запускает TCP watcher и ждёт first packet. Этот UDP ресурс и sibling task ещё
   не получают собственного early receipt. HTTP transport pipes/extra tasks и
   UoT/SDK opaque upstream также не доказаны завершёнными по TCP input done.
2. Listener.ListenTCP внутренне выполняет native creation/logging до возврата
   public handle; adapter хранит attempt, но не делает эту SDK операцию атомарной,
   interruptible или hard bounded. Parent original Process/Job/IO/handle retirement
   остаётся обязательным. Native listener/box close/control может блокироваться.
3. Initial4096 inputs плюс отдельные routes/rejections — structural bookkeeping
   limits, не hard fd/goroutine/byte/RSS/CPU budget/измеренный throughput. Retry
   delay — scheduling, не cleanup fence. Wrapper может менять copy fast paths;
   capacity/latency/CPU/RSS, pressure/recovery и Windows socket semantics OPEN.
4. Inbound option ABI, auth/UoT/UDP/HTTP/CONNECT/upgrade/detours, callback timing,
   native races/SDK/OS и release acceptance не запускались. OnClose delivery
   изменён намеренно, а не объявлен эквивалентным по одному source review.
5. WFP/native families/whole chain/root-browser/ordinary Stop/SDK migration/ABI/
   UX/packaging/release/comparative/full PC00…09/old9 OPEN. All gatesfalse/
   ranBeforeJob/GUI6R1U1/IPC9/Job8/TUN unavailable/schema2 unsupported прежние.

Практический урок: local handshake socket нуждается в original owner ещё до
Router; разрешение выполнить handshake и право выпустить пользовательский трафик
различаются. Nil callback после delegation не доказывает освобождение socket.

<a id="pc-v284-udp"></a>

## V2-284 — UDP-сокет association до первого пакета

**Режим:** PC-01 source/unverified; весь PC-00…09/старые девять этапов активны.
Тесты, сборки, SDK/native/runtime и сеть не выполнялись.

### Точка интеграции

Pinned socks.HandleConnectionEx принимает public PacketListener. Stock mixed
constructor передаёт внутренний listener и не позволяет перехватить native UDP
acquisition снаружи. Поэтому detached stock mixed processor V2-283 заменён тонким
Windows dispatcher: прежние SDK SOCKS/HTTP framing, authenticator и UoT router
сохранены как зависимости, а PacketListener предоставляет ownedMixedInbound.
Mixed dispatch адаптирован из pinned source, attribution/SPDX сохранены в новом
файле; SDK/cache/manifest/dependencies не изменены. Это local glue, не новый SOCKS
или HTTP parser. Existing inbound TLS/system-proxy private-option guards прежние.

### Original acquisition и очистка

- registerMixedInput переносит exact TCP owner в private context value. SDK auth
  derivation сохраняет его; PacketListener проверяет same gate и actual live
  input map, closed/fault/sealed/current context. Владелец не создаётся из JSON.
- Под gate→input value mutex публикуется ровно один mixedPacket до native
  ListenPacket. Ошибка не снимает reservation и не разрешает повтор/замену.
  Один UDP attempt на original TCP input наследует existing initial4096 inputs;
  это не hard quota всех native ресурсов/SDK goroutines/bytes.
- Bind-called, original bind frame return/error, optional raw handle и local
  address сохраняются отдельно. Pending acquisition не считается no-resource.
  Pre-call canceled без handle, returned error без handle, partial known handle
  и failed frame — разные исходы. Context cancellation получает original
  AfterFunc close task; никакая попытка не stop/replace и не забывается по timeout.
- Один native Close после original bind fence. Даже после failed bind frame
  известный raw handle закрывается один раз; факт actual native Close return/
  error отделён от итоговой uncertainty acquisition. Неизвестные ресурсы не
  превращаются в success через повторный Close или ErrClosed.
- PacketConn wrapper считает ReadFrom/WriteTo/control entry/return; deadlines
  сериализованы с native Close, value/gate mutex не держатся во время IO. Raw
  unwrap/replaceable copy path не предоставляется. Panic не возвращает invented
  partial packet result. PacketWriter buffer ownership остаётся у SDK adapters.
- Root cancel теперь владеет UDP socket даже до первого packet/Router: ранняя
  ошибка SOCKS response/first read больше не полагается на неявный SDK Close.
  Successful TCP input drain после handler/raw calls ждёт exact UDP done. Failed/
  pending pair сохраняется в original input map; parent Process/Job reaper
  обязателен, local receipts не подтверждают whole-tree physical retirement.
- TCP addresses снимаются в counted original frame до SDK entry, UDP LocalAddr
  внутри original bind frame. После Close возвращаются captured metadata без
  новых raw calls: cancellation не подменяет обычный LocalAddr внезапным nil.
  Эти адреса не выдаются за native identity, interface lease или permit.
- Own dispatcher logs только finite handshake closed/rejected category. Pinned
  auth parser может включать supplied credentials в error; его raw payload не
  выводится этим logger. Это локальная граница, не audit всех SDK error consumers.

### Допустимое подтверждение

Manual4 source/diff/caller/lock review: forwarding_mixed, forwarding_ingress,
новые forwarding_mixed_processor и forwarding_udp. Просмотрены exact SDK mixed
orchestration, SOCKS PacketListener/UDP branch, AssociatePacketConn.Close и bufio
packet adapter; native bind до публикации wrapper, cancellation/partial result,
known raw после panic, no-resource distinction, pending/reentrant Close, IO/task
fences и paired input drain. gofmt4+1 exit0, только4 touched Go files.
8 snapshots6existing2absent/task71rev659 архивирован;20 pinned local SDK/Go/license
source identities unchanged. Evidence: bootstrap-v284-udp-source-evidence.json.

Нет tests/fixtures/build/compile/vet/typecheck/product parser/check/doctor/install/
gen/SDK/helper/crypto/native/browser/admin/live/network исполнения.100 acceptance
OPEN/56 checks NOT_RUN; full goal/tasks0…70/constraints/decisions сохранены, no waiver.
Source/gofmt не доказывают compile/runtime/native/protected-egress готовность.

### Остаток

1. SDK SOCKS UDP branch по-прежнему запускает TCP watcher. Counted Read/native
   Close не доказывают возврат его original goroutine frame; first-packet buffer,
   canceler/cached wrappers и opaque SDK tasks требуют отдельного разбора.
2. HTTP pipe/upgrade/transport tasks и UoT ownership также не выводятся из этих
   socket receipts. Handler/callback return не равен upstream retirement.
3. Public ListenPacket/LocalAddr/native Close/control/SDK Box.Close могут ждать
   или частично отказать. Existing5s ограничивает observation, не native work или
   полное время Stop. Parent original Process/Job/IO/handles всё ещё обязательны.
4. Wrappers могут менять native batch/copy paths; UDP loss/fragmentation/deadlines/
   buffers/throughput/CPU/RSS/pressure/recovery/ABI/OS не проверены. Initial caps
   не являются measured performance или hard whole-process resource bound.
5. WFP/native families/whole chain/root-browser/ordinary Stop/SDK migration/ABI/
   UX/packaging/release/comparative/full PC/old9 OPEN. All gatesfalse/ranBeforeJob/
   GUI6R1U1/IPC9/Job8/TUN unavailable/schema2 unsupported сохранены.

Практический урок: UDP association приобретает native socket до первого пакета
и Router. Original acquisition нужен в этот момент; пакет ещё не пришёл — не
значит, что ресурса нет. Known raw очищается даже при failed acquisition frame.

<a id="pc-v285-association"></a>

## V2-285 — исходный UDP watcher и первый buffer

**Режим:** PC-01 source/unverified; весь PC-00…09/старые девять этапов активны.
Абсолютный запрет тестирования, сборок, SDK/native/runtime/сети сохраняется.

### Почему требовался local orchestration seam

Pinned socks.HandleConnectionEx создаёт TCP watcher без доступного join receipt
и передаёт первый buffer в SDK CachedPacketConn. Нормальный raw Read/Close из
V2-284 не доказывал возврат этой original goroutine. Windows SOCKS5 orchestration
теперь адаптирован отдельно, с attribution/SPDX; SDK request/response/auth codecs,
authenticator, lazy CONNECT и UDP framing используются как зависимости. SOCKS4/
HTTP handlers не переписаны; policy всё ещё проходит guarded Router.

### Original association

- Typed openOwnedPacket возвращает прежний exact UDP owner. Под его value mutex
  association публикуется до response write, watcher Go, buffer allocation/read.
  Closing/pending original owner не принимает позднюю замену association.
- Watcher объявляется до Go и хранит собственные original Read result/normal
  return/done. Его завершение запрашивает cancellation прежних TCP/UDP owners;
  native Close не выполняется заново и whole association здесь не ожидается.
- Original UDP orchestration frame отдельно сообщает normal return/error либо
  panic/Goexit. Returned transport/protocol error не равен custody failure.
  Failed/unknown записи остаются; panic не выдаёт fabricated packet/IO result.
- UDP close task после original native Close/raw-call fences ждёт association
  frame, объявленный watcher и проверяемое освобождение первого buffer. Only
  completed normal cleanup может завершить paired input. Unstarted watcher не
  превращается в synthetic completed task; его отсутствие — отдельный факт.
- Packet wrapper Close может вызваться синхронно из Router во время active
  delegation. Он closes buffer и joins только original native TCP/UDP closeWait,
  не frame/watcher/whole association. Иначе возникал бы self-join cycle: frame
  ждёт Close, который ждёт возврата того же frame. Полная проверка — post-Box drain.

### Первый пакет и время жизни

- Allocation/read/copy/Release имеют отдельные entry/normal-return facts.
  Original buffer публикуется до первого UDP read; ошибочный first read не теряет
  его. Known memory очищается после original frame/watcher; unknown удерживается.
- Local first-packet cache не отдаёт buffer в opaque refcount/cache wrapper.
  ReadPacket проверяет полную destination capacity, actual copied size и возвращает
  ErrShortBuffer без объявления truncated packet успешным. Первый пакет остаётся
  целым до подходящего caller либо Close. Nil buffer — отрицательный argument path.
- Consume/Close сериализуют только bounded buffer memory operations. Release
  вызывается один раз; actual normal return отделён от attempt. После confirmed
  Release pointer очищается и больше не читается. Panic/Goexit/unknown не разрешают
  retry Release или повторное применение potentially pooled buffer.
- Native first-read deadline и reset проверяются. При поддержке deadlines на
  этом exact UDP socket используется existing SDK NewTimeoutPacketConn напрямую.
  Его просмотренный constructor не создаёт timer goroutine. Общий SDK fallback
  NewPacketConn/Instance.wait здесь не используется, сохраняя idle timeout API.
- Не предлагаемый client auth method не выбирается; rejection заканчивает
  negotiation после одного NoAcceptedMethods response. Username/password status
  и CONNECT/request framing используют SDK codecs; own errors не содержат supplied
  credentials. Эти semantic corrections требуют будущей protocol приёмки.

### Допустимое подтверждение

Manual4 source/diff/caller/lock review: forwarding_mixed_processor/forwarding_udp,
новые forwarding_socks5/forwarding_association. Просмотрены exact SDK SOCKS5
request/auth/UDP branch, cached buffer/refcounts/Release, timeout constructor versus
timer fallback и PacketConn interface. Cancel-before-Go, response/first-read error,
no watcher/no buffer, small destination, concurrent consume/Close, early Router
Close, partial cleanup, panic/Goexit и frame/native/whole-task wait graph разобраны
по исходникам. gofmt4+1+1 exit0, только4 touched Go files.8 snapshots6existing2absent/
task71rev661 архивирован;27 initial +1 final ABI source identity =28 pinned local
SDK/Go/license identities unchanged. Product/dependency files не исполнялись.
Evidence — bootstrap-v285-association-source-evidence.json.

Нет tests/fixtures/build/compile/vet/typecheck/product parser/check/doctor/install/
gen/SDK/helper/crypto/native/browser/admin/live/network исполнения.100 acceptance
OPEN/56 checks NOT_RUN; goal/tasks0…70/constraints/decisions сохранены без waiver.
Source/gofmt не доказывают compile/protocol/runtime/native/VPN готовность.

### Остаток

1. HTTP transport/upgrade pipes, UoT/mux и opaque upstream SDK workers/resources
   ещё требуют original ownership integration. Local association done не
   подтверждает whole SDK/kernel/Job retirement; parent original reaper обязателен.
2. TCP prefetch/optimistic payload/cache transfer и остальные SDK buffer writer
   paths не покрыты новым first-UDP-buffer contract. Не объявлять их проверенными
   по одному исправленному cache или числу зарегистрированных TCP sockets.
3. Native Read/Close/deadline/SDK могут ждать дольше existing5s observations.
   No timer-as-cleanup/hard total Stop bound. One first buffer/watcher per input и
   existing initial4096 input cap не задают hard RSS/CPU/bytes/throughput quota.
4. SOCKS/auth/deadline/UDP semantics, callback scheduling/copy paths, loss/buffers/
   pressure/recovery/ABI/OS/performance не проверены. Более строгая negotiation и
   новый cache являются source changes, не заявленной runtime эквивалентностью.
5. WFP/native families/whole chain/root-browser/ordinary Stop/SDK migration/ABI/
   UX/packaging/release/comparative/full PC/old9 OPEN. All gatesfalse/ranBeforeJob/
   GUI6R1U1/IPC9/Job8/TUN unavailable/schema2 unsupported прежние.

Практический урок: native socket Close, original watcher return и освобождение
первого buffer — разные обязательства. Close, вызванный из active delegation,
не должен ждать возврата собственной delegation frame.

<a id="pc-v286-prefetch"></a>

## V2-286 — остаток TCP parser buffer после SOCKS CONNECT

**Режим:** PC-01 source/unverified; весь PC-00…09/старые девять этапов активны.
Запрет tests/build/SDK/native/runtime/network сохранён.

### Изменение исходников

Pinned SOCKS CONNECT handler разбирает запрос через bufio.Reader, затем передаёт
исходный socket в LazyConn. Reader мог уже прочитать application bytes после
запроса; этот остаток не становился частью переданного stream. Новый Windows
mixedPrefetch удерживает только этот already-buffered remainder, без новой native
read попытки. Wire/auth codecs и SDK lazy handshake остаются прежними.

- Handshake Reader имеет explicit4096 capacity, совпадающую с просмотренным Go
  default. Prefix claim публикуется в exact original input до Peek/allocation/
  copy/Discard. Original parser frame — единственный Reader user в переходе;
  size/actual copy/discard checked. Capture error/panic/Goexit сохраняет owner.
- Проверяется private context→same input, closing/cancellation и отсутствие
  прежнего prefix. Даже empty remainder не обходит existing pending owner.
  При empty path нет новой buffer allocation/copy/wrapper/prefix receipt.
- SOCKS5 wraps original ingress до SDK NewLazyConn. Для SOCKS4 existing SDK handler
  остаётся: callback проверяет exact LazyConn/contained original input и дополняет
  его Conn до первой delegation, сохраняя object identity/handshake state.
  Неожиданная constructor shape не получает raw fallback. Это адресный адаптер
  текущего pinned constructor, не general SDK unwrap/Cast.
- Prefix Read выдаёт все captured bytes перед следующим raw read. Copy/offset
  memory serialized, original read entry/normal return учитываются отдельно;
  native path всё ещё идёт через прежний counted ingress. Panic не возвращает
  invented partial Read result. Deadline/handshake/peer hints сохраняются через
  original private connection; raw/copy fast-path escape не предоставляется.
- Prefix Close seals/cancels и joins original native Close fence. Он не ждёт
  собственный active Read/handler/whole prefix: вызов может идти из delegation.
  Root post-close cleanup после handler/raw fences отдельно ждёт original capture
  и вошедшие prefix reads, затем clears owned slice. Unknown/failed owner retained;
  повтор native Close/новый prefix не выдаёт cleanup proof.
- Clear относится к собственной byte slice. Он не доказывает немедленное GC/RSS
  освобождение, отсутствие других копий данных или secure memory erasure.

### Допустимое подтверждение

Manual4 final source/diff/caller/lock review: forwarding_mixed_processor/
forwarding_socks5/forwarding_ingress и новый forwarding_prefetch. Просмотрены pinned
LazyConn construction/handshake/Close delegation, Go Reader default/Peek/Discard
contracts, empty/nonempty/max prefix, zero/small Read, cancellation до/после claim,
parallel Read/Close, early native Close/handler self-join, partial capture/read/
clear/panic/Goexit и success-only input retirement. gofmt4+1+1+2 exit0, только4 Go.
8 snapshots7existing1absent/task71rev663 архивирован;30 pinned local SDK/Go/license
identities unchanged. Evidence: bootstrap-v286-prefetch-source-evidence.json.

Нет tests/fixtures/build/compile/vet/typecheck/product parser/check/doctor/install/
gen/SDK/helper/crypto/native/browser/admin/live/network исполнения.100 acceptance
OPEN/56 checks NOT_RUN; full goal/tasks0…70/constraints/decisions сохранены без waiver.
Source/gofmt не доказывают compile/protocol/runtime/native/VPN готовность.

### Остаток

1. HTTP CONNECT cache/upgrade/transport pipes, UoT/mux/opaque upstream SDK tasks
   ещё требуют original resource/task integration. Один SOCKS prefix не является
   исправлением всех SDK caches/buffer writers или whole SDK retirement proof.
2. SOCKS4 constructor mutation сделана до first delegation по текущему SDK source.
   ABI/SDK update, handshake-before-data order, optimistic/pipelined payload,
   chunking/partial/EOF, detours/cancellation/races/protocol acceptance не исполнены.
3. Initial4096 input cap и один prefix<=4096 на input — structural byte/count
   limits этой части, не hard whole-process RSS/CPU/throughput bound. Copy/cast
   optimizations/latency/pressure/recovery не измерены; speed improvement не заявлен.
4. Native Read/Close/SDK/control могут ждать дольше existing5s observations. Parent
   original Process/Job/IO/handle reaper остаётся обязательным; no timer-as-cleanup.
5. WFP/native families/whole chain/root-browser/ordinary Stop/SDK migration/ABI/
   UX/packaging/release/comparative/full PC/old9 OPEN. All gatesfalse/ranBeforeJob/
   GUI6R1U1/IPC9/Job8/TUN unavailable/schema2 unsupported прежние.

Практический урок: передача socket не передаёт байты, уже забранные parser Reader.
Остаток должен иметь явного владельца и lifetime; его наличие не даёт forwarding
authority и не разрешает заменить original native cleanup receipt.

<a id="pc-v287-uot"></a>

## V2-287 — UoT frame/input buffer и общий отказ до writer

**Режим:** PC-01 source/unverified; весь PC-00…09/старые девять этапов активны.
Запрет тестирования/сборок/SDK/native/runtime/network сохранён.

### Подтверждённый source gap

В pinned sing UoT Conn.WritePacket branch без vectorised writer временный frame
Release-д, а переданный input buffer не Release-д. Просмотренные SDK packet-copy
callers передают владение writer: success path не освобождает input повторно,
error path вызывает Leak, а не Release. Это source ownership defect этой ветки;
runtime leak/RSS/частота/выигрыш не измерялись. Current guarded ingress не даёт
native vector/raw escape, поэтому нельзя считать alternate vector branch общим
доказательством корректности.

### Локальная интеграция

- Windows mixed processor использует typed ownedUotRouter. SDK MagicAddress/
  LegacyMagicAddress, ReadRequest и AddrParser сохранены; new adapter создаёт
  только protocol/write state, не собственный socket/helper goroutine. Actual TCP
  resource остаётся у original input и ordinary route. Source attribution сохранён.
- Проверяется private same-input context; packet metadata получает NetworkUDP.
  Early allowedPacket — отрицательный фильтр до UoT request read, не token/permit.
  Ordinary packet registration после parse снова проверяет actual admission.
  Не-UoT TCP и обычные packet entrypoints используют прежний guarded Router.
- ReadPacket сериализует frame parsing, проверяет destination capacity до payload
  read. Partial header/body/short-buffer/EOF не разрешает следующий frame в
  неизвестной позиции. Нет попытки resync/retry целого TCP frame после partial IO.
- WritePacket публикует один original write record до buffer metadata/allocation/
  copy/Release/Write. Uint16 payload cap65535, FQDN/address bound и actual codec/
  copy lengths checked до wire IO. Oversize не превращается в truncated uint16.
- Input buffer никогда не передаётся в Write: bytes копируются в отдельно owned
  bounded frame, затем input получает одну фактическую Release attempt/normal
  return. Frame возвращается только после actual original Write return. При
  panic/Goexit/unknown он остаётся у original record, без retry/поиска нового owner.
- Nil-error short Write становится ErrShortWrite и poisons protocol lane. Known
  returned transport/serialization error отделён от custody uncertainty; он сам
  по себе не становится whole-channel health failure. No raw/vector unwrap.
- Close seals writer intake, attempts original wrapped ingress Close до ожидания
  original write.done; data read/write mutex не удерживается им. Pending/failed
  writer даёт cleanup_unconfirmed, exact record остаётся. Original Close attempt,
  return/error и bounded observation результата writer разделены. Wrapped Close/
  Write могут относиться к TCP или HTTP pipe; это API receipts, не kernel proof.
- Existing handshake-failure reporter сохраняется как узкий protocol API. Error
  callback не ждёт data-write mutex: writer может ждать Close/cancellation того
  самого callback. Общий forwarding контроль/owned ingress остаются обязательными.

### Отказ до delegation

При проверке callers найден дополнительный bare buffer.Release в negative
forwardingPacketConnection.WritePacket до actual writer. Scope расширен двумя
существующими Windows files, snapshots сняты до их изменения; initial7 manifest
сохранён, final9 manifest содержит7 existing +2 absent paths.

Этот путь и UoT negative writes используют общий rejectPacketBuffer через прежний
bounded rejectIngress. Original buffer owner публикуется до memory Release;
pending/failed сохраняется, actual normal result удаляет receipt. Nil — known
no-acquisition; normal rejection сохраняет original cause. После real delegation
outer wrapper не делает догадку о повторном Release. Новая goroutine/независимый
pool/controller не добавлены. Existing post-Box seal/cap worker-only fail-stop
действует и здесь; memory Release не объявлен native Close proof.

### Допустимое подтверждение

Manual5 final source/diff/caller/lock review: mixed_processor, new forwarding_uot/
forwarding_uot_writer, existing forwarding_connections/rejections. Просмотрены SDK
UoT router/conn/request/address/read waiter/vectorised writer и both packet-copy
caller paths. Разобраны nil/zero/max/oversize/codec error, partial/short Write,
reader capacity/desync, buffer consumption/error, original Write/Release panic/
Goexit, retained failed writer, native-before-writer Close order и error-callback
wait graph. gofmt3+2+1+1+3 exit0, только5 touched Go files.9 snapshots7existing2absent/
task71rev665 архивирован;34 initial +2 caller identities =36 pinned local sources
unchanged. Evidence: bootstrap-v287-uot-source-evidence.json.

Нет tests/fixtures/build/compile/vet/typecheck/product parser/check/doctor/install/
gen/SDK/helper/crypto/native/browser/admin/live/network исполнения.100 acceptance
OPEN/56 checks NOT_RUN; goal/tasks0…70/constraints/decisions сохранены без waiver.
Source/gofmt не доказывают compile/protocol/runtime/native/VPN готовность.

### Остаток

1. HTTP pre-Router pipe/tasks/CONNECT cache/upgrade/transport и mux/opaque SDK
   workers ещё не получают полного original retirement contract. UoT headers в
   таких SDK pipes не объявлены покрытыми native TCP input receipt. Parent original
   Process/Job/IO/handle reaper остаётся обязательным, all SDK/kernel proof OPEN.
2. New framing preserves v2/legacy source layout, но wire/ABI/handshake/callback/
   EOF/short IO/deadline/UDP loss/detour/race acceptance не исполнены. Native/SDK
   IO и reporting могут блокировать дольше existing5s observations.
3. New record/channel/frame copy и отказ от vector/raw path требуют measurement;
   speed/RSS/CPU/GC gain не заявлен. Uint16/header bounds, one writer/route и
   existing initial route/rejection caps не hard whole-process resource quota.
4. Actual Release/Write return — API-level fact. Refcounts/unpooled memory/other
   copies, kernel/opaque SDK borrow и physical retirement не выводятся из него.
   Остальные SDK buffer-writer branches не объявлены автоматически исправленными.
5. WFP/native families/whole chain/root-browser/ordinary Stop/SDK migration/ABI/
   UX/packaging/release/comparative/full PC/old9 OPEN. All gatesfalse/ranBeforeJob/
   GUI6R1U1/IPC9/Job8/TUN unavailable/schema2 unsupported прежние.

Практический урок: packet writer должен завершить владение input даже при отказе;
простой buffer Release не доказывает завершение Write, который мог заимствовать
другой frame. Сначала закрывается исходный ingress, затем наблюдается writer.

<a id="pc-v288-http-connect"></a>

## V2-288 — исходный HTTP CONNECT remainder во всём request loop

**Режим:** PC-01 source/unverified, весь PC-00…09 и прежние девять этапов активны.
Нет tests/build/compile/vet/typecheck/product checks/SDK/native/network исполнения.

### Причина и повторное использование

Pinned sing HTTP HandleConnectionEx после CONNECT переносит reader.Buffered()
через pooled buffer в NewCachedConn. После предыдущего обычного HTTP keep-alive
тот же SDK loop тоже может прочитать CONNECT. Поэтому interception только первого
метода оставил бы этот source path; он отвергнут до финальной фиксации.

В Windows уже есть original input prefix owner V2-286: bounded already-buffered
copy, capture/read/clear fences, no extra raw read, cancellation и explicit root
retirement. Он переиспользован; второй prefix pool/owner/controller не создан.

### Изменения

1. Mixed dispatch вызывает local HTTP orchestration. SDK ReadRequest,
   ParseBasicAuth, SourceAddress, original upstream metadata adapter и Router/UoT
   сохранены. Каждый request проходит тот же local outer loop; нет serialize/
   replay parsed request обратно в SDK и обхода при последующем CONNECT.
2. SDK предоставляет whole-loop API; его one-request HTTP/Upgrade helpers private.
   Их algorithms адаптированы в forwarding_http.go с source attribution. Ordinary
   HTTP proxy/Upgrade/redirect/compression/hop-header/Host-port/keep-alive behavior
   сохранён в исходниках, соответствие wire/runtime пока не проверено. Это не
   замена функций ограниченным CONNECT-only режимом.
3. CONNECT сначала сохраняет только already-buffered bytes через тот же exact
   mixedIngressConnection/private input context, затем пишет прежний 200 response.
   Nil-error short Write даёт ErrShortWrite, без delegation/retry partial response.
   До positive Router delegation сохраняется callback/source/destination/user;
   обычный guarded Router повторно проверяет admission.
4. Proxy auth challenge/realm, HTTP1.0/no-keepalive, body-length condition для
   retry, per-request auth user, source metadata и default ports следуют SDK.
   Rejection error не включает username/password/Proxy-Authorization. SourceAddress
   остаётся protocol metadata, не original native peer attestation.
5. Между request/auth iterations проверяется current context, включая полностью
   prefetched pipeline. Уже выданные callback/route/native ownership контракты
   не объявляются завершёнными этим циклом. Nil input rejected до acquisition.

### Разрешённый review и идентичность

Manual2 final source/diff/caller review: mixed_processor и new forwarding_http.
Сверены pinned SDK request/auth/source/destination, challenge и outer keep-alive
loop, CONNECT reply/delegation, original prefix capture/failure/drain, ordinary
HTTP/Upgrade algorithms и upstream metadata adapter. Просмотрены parse/auth retry,
HTTP1.0, next ordinary request и later CONNECT, zero/max buffered prefix, partial
response, cancellation, callback/error и absence of pooled cache/raw unwrap.

gofmt2+1 exit0 только2 touched Go files.6 original snapshots5existing1absent и
task71rev668 сохранены;38 local pinned source identities без изменения. Evidence:
bootstrap-v288-http-connect-source-evidence.json.100 acceptance OPEN/56 checks
NOT_RUN; весь goal/constraints/decisions/checks/tasks0…70 без waiver.

### Открытые границы

- Ordinary/Upgrade pipes создаются до Router, используют прежние helper tasks,
  CopyConn и http.Transport. Оригинальные acquisition/IO/Close/body/callback/task
  return receipts и bounded shutdown ещё не интегрированы. Их source перенос не
  считается исправлением ownership; parent original Process/Job reaper обязателен.
- HTTP parser/header/body/auth-retry quotas и whole-process limits пока не
  доказаны.4096 — bound сохранённого reader remainder, не total request/header
  limit. Участок не обещает full HTTP/native retirement или hard5s deadline.
- Request/response strings и SDK/Go copies вне prefix не получают автоматически
  memory-retirement proof. Clear своего prefix не GC/RSS/secure wipe.
- Protocol/ABI/keep-alive/Upgrade/redirect/body/error/deadline/race/performance и
  реальные native/SDK/WFP/root-browser/OS/UX/release/comparative acceptance OPEN.
  Speed/resource superiority не измерена; all gatesfalse/ranBeforeJob/GUI6R1U1/
  IPC9/Job8/TUN unavailable/schema2 unsupported; full PC/old9 остаются открытыми.

Урок: перенести только первый request недостаточно, если SDK продолжает читать
остальные в своём цикле. Owner boundary должен покрывать каждую смену протокола.

<a id="pc-v289-http-pipes"></a>

## V2-289 — original HTTP pipe pair до конструктора и Router

**Режим:** PC-01 source/unverified, весь PC-00…09 и прежние девять этапов активны.
Абсолютный запрет tests/build/compile/vet/typecheck/product checks/SDK/native/network.

### Дефект и повторное использование

Ordinary HTTP DialContext и Upgrade создавали SDK pipe pair до Router и запускали
handler без input-owned acquisition/return record. Router владел только вторым
концом; ignored common.Close не являлся результатом original pair retirement.

Переиспользованы private mixed input context/registration, lifetime cancellation,
existing route/rejection gate, normal-result-only removal и mixedOriginalDone.
Новый pipe owner — узкий resource receipt в backend/core, не второй controller,
не новый transport, не independent admission и не разрешение native forwarding.

### Реализация

1. openMixedHTTPPipe проверяет exact retained input под gate→input locks. Original
   pair помещается в input map до pipe.Pipe, metadata calls, goroutine и Router.
   Initial32 receipts/input включают pending/failed; pressure rejected до ресурсов.
   Закрытый input/gate и sealed cohort не допускают поздний pair.
2. Constructor return и acquisition-frame return различаются. Известные original
   endpoints сохраняются и получают одну Close attempt даже при последующем сбое
   metadata/frame. Unknown constructor/frame/panic/Goexit не считается no-resource
   и не разрешает replacement; receipt остаётся и faults gate.
3. Handler start объявляется под тем же mutex, который seals Close. Отмена между
   constructor и Go либо known-skips handler, либо оставляет original declared
   task с return fence. Failed original frame не фабрикует отсутствующий SDK callback.
4. Оба private net.Conn ends считают actual Read/Write/deadline calls и outcomes.
   Addresses captured once; нет embedding, Upstream, ReadWaiter/vector/raw escape.
   Pinned pipe waiter при error после получения bytes может пропустить writer ack;
   эта ветка не раскрывается SDK casts. Скорость/copy cost не измерены.
5. Один retained AfterFunc task закрывает оба raw ends до ожидания IO/handler/
   callback. End.Close только cancels и observes original raw Close results через
   existing5s wait/final re-read, не ждёт собственный handler/IO/callback и не
   делает raw Close повторно. Timeout не подтверждает physical cleanup.
   Fresh original Close facts читаются под owner mutex после channel observation;
   если исходный результат всё ещё неизвестен/ошибочен, failure sticky и faults
   gate даже если внешний Transport проигнорирует error. Поздний original return
   до этой записи можно принять; replacement Close не используется как proof.
6. Callback имеет original return fence, cancel без synchronous join. Ordinary
   innerErr Store остаётся тем же callback consumer; Upgrade не теряет flow
   completion. Transport/serialization errors отличаются от custody uncertainty.
7. Original input cleanup сначала seals/cancels весь pair cohort, затем закрывает
   свой TCP и ждёт original handler. После этого ждёт exact pair receipts. Новый
   pair после snapshot не создаётся. Successful pair удаляется только после своих
   fences; failed/pending остаётся и блокирует positive lifecycle gate.

### Разрешённое подтверждение

Manual4 final source/diff/caller/lock review: forwarding_http, forwarding_ingress,
new forwarding_http_pipe и forwarding_http_pipe_io. Просмотрены cancellation до/
во время constructor/start, known partial resources, pressure, late DialContext,
both raw closes, in-flight IO, callback reentrancy, handler fault/Goexit, missing
callback, input seal/deletion races и no callback/Close self-join wait graph.

Pinned SDK pipe/pipe_wait/network casts, callback signature/TypedValue.Store и
CopyConn/task.Group рассмотрены по локальным исходникам. Group.Run после cleanup
ждёт taskContext completion от returned task bodies; прежняя гипотеза, что сам
normal Group return может произойти без этого ожидания, не подтверждена. Это
конкретный source порядок, не native/runtime/whole-SDK/API fault proof.

gofmt4+3+1 exit0 только4 touched Go files.8 snapshots6existing2absent и task71rev671
сохранены;41 initial +typed.go =42 local source identities без изменения. Evidence:
bootstrap-v289-http-pipes-source-evidence.json.100 acceptance OPEN/56 NOT_RUN;
goal/constraints/decisions/checks/tasks0…70 сохранены без waiver. Нет compile/
protocol/race/native/VPN acceptance; source/gofmt не PASS этих уровней.

### Открытые границы

- Transport dial/read/write/idle tasks, request/response body ownership, deadline
  timers и SDK/Go buffers вне counted pair calls ещё требуют отдельных original
  receipts. Input/pipe/Router/API return не заменяет whole SDK/kernel cleanup.
- Pipe constructor и raw Close source memory-only; неизвестный результат всё
  равно retained. Никакого hard OS5s bound: observation может истечь раньше
  original handler/IO/Close. Parent Process/Job/IO/handle reaper обязателен.
- 32 pairs/input + existing input/route caps — structural receipt budgets, не
  hard bytes/RSS/CPU quota. HTTP header/body/auth retry quotas и performance/
  pressure acceptance OPEN; extra task/wrappers/copy cost не измерен.
- Ordinary/Upgrade/CONNECT/body/deadline/EOF/cancel/race/ABI/SDK/native/WFP/
  root-browser/OS/UX/release/comparative/full PC/old9 OPEN. All gatesfalse/
  ranBeforeJob/GUI6R1U1/IPC9/Job8/TUN unavailable/schema2 unsupported.

Урок: Close, вызванный handler или callback, не должен join-ить тот же handler.
Сначала original endpoints получают Close, отдельно наблюдаются actual task/IO returns.

<a id="pc-v290-http-body"></a>

## V2-290 — исходные HTTP Body Read/Close и ранние ошибки заголовков

**Режим:** PC-01 source/unverified, полный PC-00…09/прежние девять этапов активны.
Tests/build/compile/vet/typecheck/product checks/SDK/native/network не исполнялись.

### Подтверждённые source gaps

Go Response.Write и Request.Write могут вернуть header-write error до вызова
transferWriter.writeBody, который закрывает BodyCloser. Один response.Write не
гарантировал вызов Body.Close на раннем выходе. Client.Do может закрывать request
body асинхронно; игнорирование повторного Close не даёт original outcome.

Go body.Close может дочитывать source под body mutex. Close до остановки original
socket может ждать Read/drain, удерживая выход из request helper. CloseIdleConnections
не прерывает active connections; Transport отдельно использует WithoutCancel для
dial context, сохраняя values. Ни cancel, ни этот void API не доказывают возврат
Transport goroutines. Эти факты сверены с локальным Go source, не runtime.

### Реализация и повторное использование

- Original mixed input содержит bounded64 body receipts, включая pending/failed.
  Request body ordinary HTTP/Upgrade registered до SDK handoff; response body —
  сразу после actual Do return до header/body use. Существующие exact input/gate,
  gate→input locks, original rejection lane и mixedOriginalDone переиспользованы.
- Wrapper считает actual Read returns, EOF и remaining content length без total
  byte-counter overflow. Нет raw/Upstream/WriterTo escape. Positive reads после
  closing/fault/cancel rejected. Panic/Goexit/invalid n не выдают partial success.
- Один original raw Close с started/finished/returned/error; SDK repeats и явный
  defer observing того же результата, без second raw attempt/ErrClosed-as-proof.
  Read и Close работают вне value mutex; успешное удаление только после original
  Close и actual outstanding Read returns. Failed/unknown остаётся, faults gate.
- Explicit helper defers закрывают used body на early-header/invalid-URL/Do/write/
  pipe-error exits; cleanup error joins original result. Request cancellation тоже
  deferred, не только ручные ветки. Nil/NoBody сохраняются без resource acquisition.
- Incomplete/active-read body Close cancels exact input до raw.Close, прерывая
  потенциальный source drain. Known EOF/content-length complete не отменяет
  успешную keep-alive session. Root cleanup seals bodies и pipes, закрывает native
  TCP, затем вызывает body cleanup до handler join и ждёт exact read/Close receipts.
- Response+error от Do может означать already-closed body (Go redirect error path).
  При таком captured unknown prior Close raw pointer сохраняется до parent reaper;
  wrapper не повторяет Close и не создаёт closeWait/done evidence. Если его нельзя
  зарегистрировать, existing child-only untracked-resource fail-stop сохраняет
  parent Process/Job responsibility. Current ErrUseLastResponse branch в Go source
  возвращает unclosed response с nil error; redirect behavior не менялся.
- Scope — used ordinary/Upgrade bodies. CONNECT parser body view не дочитывается
  через Close после tunnel handoff: payload остаётся у original TCP/prefix owner.
  Authentication/header parsing не объявлены закрытыми этим новым body wrapper.

### Разрешённое подтверждение

Manual3 final source/diff/caller/lock review: forwarding_http, forwarding_ingress
и new forwarding_http_body. Разобраны early header/Do/invalid URL/pipe failures,
nil/NoBody/HEAD, content length/chunked EOF, active Read versus Close/drain, SDK
repeat Close, cancellation, late body publication/pressure, prior-Do unknown state,
panic/Goexit/invalid n, failed retention и root native-before-body-before-handler order.

gofmt3+2 exit0 только3 touched Go files.7 original snapshots6existing1absent и
task71rev674 сохранены;46 local SDK/Go identities unchanged. Evidence:
bootstrap-v290-http-body-source-evidence.json.100 acceptance OPEN/56 NOT_RUN;
goal/constraints/decisions/checks/tasks0…70 без waiver. Это не compile/wire/race/
runtime/SDK/native/VPN proof; все прежние native gates закрыты.

### Остаток

1. Transport original dial/read/write/idle tasks, caller return versus asynchronous
   internal work, late dial cancellation и deadline timer retirement ещё OPEN.
   Новый body receipt не заменяет Transport/SDK/kernel receipt. Void CloseIdle API
   и done тела не считаются whole-chain cleanup; parent Process/Job reaper обязателен.
2. Initial64 records/input — count budget, не body/header/RSS/CPU limit. Body size/
   header/auth budgets, chunk/trailer/Expect100/early response/101 writable body,
   compression/redirect/EOF/keep-alive/race/performance acceptance не исполнены.
3. Неполное тело намеренно завершает input, а не пытается продолжить неизвестную
   HTTP границу. Native Close/body drain могут блокировать дольше5s observations;
   hard deadline, скорость или экономия памяти не заявляются.
4. Full PC00…09/old9/native/WFP/root-browser/mux/SDK/ABI/OS/UX/release/comparative
   OPEN; all gatesfalse/ranBeforeJob/GUI6R1U1/IPC9/Job8/TUN unavailable/schema2 unsupported.

Урок: Write(headers+body) может вернуть ошибку до ветки, закрывающей body. Явный
cleanup обязан наблюдать исходный Close, сохраняя асинхронные Read и unknown owner.

<a id="pc-v291-http-dial"></a>

## V2-291 — original request scope вместо detached DialContext lifetime

**Режим:** PC-01 source/unverified; весь PC-00…09 и прежние девять этапов активны.
Tests/build/compile/vet/typecheck/product checks/SDK/native/network не исполнялись.

### Подтверждённый источник проблемы

Go Transport создаёт dialCtx через WithCancel(WithoutCancel(requestCtx)): source
values сохраняются, cancellation/deadline исходного запроса — нет. Прежний local
DialContext передавал этот detached ctx в openMixedHTTPPipe. Client.Do cancel
мог завершить интерес к запросу, не отменив original pipe lifetime через этот ctx.
CloseIdleConnections закрывает idle connections, не является join всех goroutines.

### Изменения и reuse

1. Existing input/gate→input registration, original pipe owner, cancellation и
   mixedOriginalDone переиспользованы. Scope — resource record для одного HTTP
   request, не controller/transport/permit. Он опубликован до Client/Transport/Do.
   Source context/auth/source metadata захватываются до SDK detachment; request
   и его pipe owners используют одну cancellable lifetime.
2. Каждый admitted DialContext получает original frame record до address parse/
   pipe acquisition/handler Go. SDK ctx checked на входе и перед return, но не
   является lifetime authority. Original pipe сохраняется даже при error/late
   result. После scope seal/cancel результат не публикуется, cancels exact pair.
   Поздний invocation после seal rejected до ресурсов/Go; нового permit не выдаёт.
3. Existing private pipe ends возвращаются без unwrap/replacement. Return end —
   не borrower ACK. Гонка cancel сразу после последней проверки не разрешает IO:
   сам end/ordinary Router сохраняет собственные cancellation/admission gates.
4. Close seals call intake, cancels request/pipe lifetime до observations. Shared
   existing5s context и final original done re-read используются для каждого
   original dial frame и pair. Callback только existing innerErr.Store; он не
   вызывает Close scope, который его ждёт. Root сначала cancels scopes, затем
   seals pipes/bodies и closes TCP; original scope join после handler retirement.
5. Original Close outcome сохраняется, повторный вызов observes same result.
   Pending/failed/panic/Goexit/invariant fault остаются, faults gate. Concurrent
   observer timeout sticky; final Close не возвращает nil при уже recorded fault.
   Normal negative dial error не объявлен whole-channel health failure. Known
   partial pair отменяется/наблюдается, unknown frame не считается no-resource.
6. Initial32 scopes/input и32 admitted frames/scope, включая original failed
   records, используют existing pipe budget. Pressure rejected до acquisition;
   late detached SDK call не создаёт забытый pair после завершённого scope.
   Новой явной goroutine на scope/dial не добавлено; сам Go Transport по-прежнему
   запускает свои внутренние tasks. Его lifetime отдельно не принят.

### Разрешённое подтверждение

Manual3 final source/diff/caller/lock review: forwarding_http, forwarding_ingress
и new forwarding_http_dial. Просмотрены before/during/after cancel, detached SDK
ctx, source/auth preservation, original call before parse/constructor, late/partial
pipe, entry/return checks, deadline final reads, pressure, Close overlap/sticky
fault, panic/Goexit, seal/snapshot races и callback/Close wait graph.

Локальный Go Transport/context source подтверждает WithoutCancel lifetime и
CloseIdle boundary. Поведение SDK/wire/race/native не запускалось. gofmt3+1 exit0
только3 touched Go files.7 snapshots6existing1absent/task71rev677 и46 pinned local
source identities сохранены. Evidence: bootstrap-v291-http-dial-source-evidence.json.
100 acceptance OPEN/56 NOT_RUN; goal/constraints/decisions/checks/tasks0…70 без waiver.

### Остаток

- Scope joins только наши original DialContext calls и pipe receipts. Transport
  dialConnFor continuation/borrower adoption, readLoop/writeLoop/idle/TLS/timers и
  их actual goroutine returns ещё не получают полного original retirement API.
  Done scope/API return/CloseIdle не считаются whole Transport/SDK/kernel cleanup.
- SDK-only ctx cancellation checked на входе/выходе, не добавляет forced termination
  неизвестного constructor. Root/request cancel реально связан с original pipe
  context; raw calls/tasks могут пережить observation timeout. Parent original
  Process/Job/IO/handle reaper обязателен, hard5s bound не заявлен.
- Context values из source остаются; внутренние attempt-only SDK context values
  не получают отдельную integration acceptance. Поведение source deadline/auth/
  retries/TLS/redirect/early body/cancel/race/pressure/limits/perf не проверено.
- 32/32 receipt counts не byte/RSS/CPU quota. HTTP header/auth budgets, mux/opaque
  SDK/native/WFP/root-browser/ABI/OS/UX/release/comparative/full PC/old9 OPEN.
  All gatesfalse/ranBeforeJob/GUI6R1U1/IPC9/Job8/TUN unavailable/schema2 unsupported.

Урок: SDK context с теми же values может иметь другую cancellation lifetime.
Original request owner должен удерживать отдельную явную связь с создаваемыми ресурсами.

<a id="pc-v292-http-headers"></a>

## V2-292 — bounded HTTP header parsing и честная Transport граница

**Режим:** PC-01 source/unverified, полный PC-00…09/прежние девять этапов активны.
Нет tests/build/compile/vet/typecheck/product checks/SDK/native/network исполнения.

### Source audit: внутренние Transport tasks

В Go source readLoop/writeLoop, writeLoopDone, idle/TLS timers и continuation
dialConnFor принадлежат private persistConn/Transport implementation. Публичный
CloseIdleConnections закрывает idle connections, не даёт общего join этих tasks.
Conn.Close, httptrace notifications и наши local done receipts не заменяют actual
goroutine/worker retirement. Этот gate остаётся OPEN, original Process/Job/IO/
handle reaper обязателен. Копия Go Transport или отключение существующих функций
не использованы для искусственного закрытия этого gate.

Следующий доступный source gap — request readRequest вызывает ReadMIMEHeader с
MaxInt64 memory/header limits, custom proxy не использует Server.MaxHeaderBytes.
Response Transport при нулевом MaxResponseHeaderBytes использует10MiB default.

### Реализация и reuse

- Reuse прежнего single4096 bufio.Reader, SDK H.ReadRequest и original exact input.
  Новый source reader регулирует bytes только вокруг HTTP parse. При begin128KiB
  уменьшаются на уже Buffered bytes; каждый actual source Read ограничивается
  remaining. Конец budget даёт finite error до дополнительного raw Read.
- Budget обновляется для каждого ordinary/auth retry request в том же loop.
  Считаются request line и MIME headers до завершения parse. Body/CONNECT/Upgrade
  получают тот же Reader после снятия header budget, с сохранённым prefix.
  Нет вложенных Readers на каждый request, Reset, повторного parse/serialize,
  copy pooled-cache или ограничения tunnel/body payload величиной128KiB.
- Source bound к тому же private mixedIngressConnection/context. Raw Read всё
  ещё проходит original input IO receipt. Budget mutex защищает values, отпускается
  до socket IO; phase transition не разрешён при pending read. Concurrent/invalid
  source operation, panic/Goexit/invalid n poisons input и faults gate. Ordinary
  oversized header — rejection этого входа, не synthetic whole-channel failure.
- Для response используется штатный Transport.MaxResponseHeaderBytes128KiB,
  без собственного response parser. Existing keep-alive/auth/CONNECT/Upgrade,
  source metadata, body/dial/pipe owners и protected Router сохраняются в коде.
  SOCKS parsing использует тот же Reader с неактивным HTTP budget; handoff по-
  прежнему возвращает original connection/prefix, не новый raw/native interface.

### Разрешённое подтверждение

Manual3 final source/diff/caller/lock review: forwarding_mixed_processor,
forwarding_http и new forwarding_http_limits. Просмотрены initial Peek/readahead,
zero/exact/over budget, buffered bytes credit, line/header parsing, auth retry/
next keep-alive request, body read after clear, CONNECT prefix, SOCKS handoff,
nil/private-context mismatch, partial read/EOF/cancel, phase/read fault и lock graph.

Локальные Go request/textproto/Transport sources подтверждают parser limits и
штатный response field/default. gofmt3 exit0 только3 touched Go files;7 snapshots
6existing1absent/task71rev680 и48 local SDK/Go identities unchanged. Evidence:
bootstrap-v292-http-headers-source-evidence.json.100 acceptance OPEN/56 NOT_RUN;
goal/constraints/decisions/checks/tasks0…70 сохранены без waiver. Это не compile/
protocol/race/runtime/SDK/native/VPN proof.

### Остаток и критерии будущей приёмки

1. 128KiB — initial per-message byte policy, не exact heap/RSS allocation bound,
   body size, parsed field count или cumulative/auth-attempt quota. Реальные
   large cookies/authorization/URI/header interoperability ещё не проверены.
2. Slow-header/handshake deadlines, auth retry/time/rate budgets и concurrent
   process-wide admission требуют отдельной реализации/приёмки. Byte cap не
   является таймером или hard bound операции; source reader добавляет mutex
   work, его стоимость и memory/speed benefit не измерялись.
3. Header limit не активен на tunnel/body; остальные SDK/frame/Trailers/Expect100/
   early response/101/redirect/EOF/DPI/native/race/pressure/perf cases OPEN.
4. Transport private tasks/timers/borrower/whole worker-native retirement остаются
   отдельными gates. Parent Process/Job source и socket Close не объявлены OS proof.
   Full PC00…09/old9/WFP/root-browser/SDK/ABI/OS/UX/release/comparative OPEN;
   all gatesfalse/ranBeforeJob/GUI6R1U1/IPC9/Job8/TUN unavailable/schema2 unsupported.

Урок: маленький bufio buffer не ограничивает строку/набор заголовков — parser
может накапливать их за много fills. Budget должен учитывать уже buffered bytes
и сниматься перед чтением тела, сохраняя один исходный Reader.

<a id="pc-v293-handshake-time"></a>

## V2-293 — original phase timers и bounded HTTP auth

**Режим:** PC-01 source/unverified, весь PC-00…09 и прежние девять этапов активны.
Нет tests/build/compile/vet/typecheck/product checks/SDK/native/network исполнения.

### Реализация

1. Original input retains timer receipt до time.AfterFunc construction. Initial
   limit8 pending/failed budgets/input; no forgotten timer on early failure.
   Initial first-byte Peek получает30s budget. Каждый HTTP header и407/CONNECT200
   control write получает phase budget с минимумом из30s/supplied/context deadline.
   Native socket deadline не перезаписывается: его предыдущий deadline нельзя
   надёжно прочитать/угадать у opaque caller connection.
2. Release ставит disarmed flag под mutex до Stop. Callback до cancel claims action
   под тем же mutex. Если expiry уже claimed, phase end ждёт original callback/
   cancel return и не передаёт соединение дальше как своевременное. CompletedAt
   фиксируется до retirement observation, чтобы delayed join не менял время
   наблюдаемого завершения parse/write. Истёкший срок также проверяется при finish,
   даже если Go ещё не успел запустить timer callback.
3. Exactly one Timer.Stop attempt, no Reset/replacement. Stop(true) — known
   no-callback fact, не fabricated callbackDone. Stop(false) по Go contract не
   ждёт функцию: code observes original callbackDone/returned и action outcome.
   Constructor/Stop/callback panic/Goexit/unknown retained, fault local gate; actual
   TCP/IO cleanup остаётся у прежнего input owner. Callback cancel API return
   не считается socket/worker retirement.
4. Input cleanup seals/disarms all phase records до native/handler waits, attempts
   original TCP close, затем joins original timer record. Concurrent parser/root
   Close observes cached original result, не повторяет Stop и не заменяет owner.
   Shared existing5s/final same-done observation — не hard operation bound. Callback
   не вызывает Close своего timer; root cancellation запускает отдельный existing
   cleanup task, callback и parser не делают self-join.
5. HTTP consecutive failed auth budget4 и shared30s header/challenge window.
   Следующая неудача не продлевает deadline. Fourth407 ставит Connection:close;
   verified auth resets counter/window. Initial budget также включает keep-alive
   ожидание следующего header. No credential-bearing errors. Ordinary body/tunnel/
   Upgrade payload не ограничены завершившимся header budget; CONNECT handoff
   только после original timer retirement, short Write по-прежнему rejected.

### Principal context lifetime

В прежнем loop ctx каждый раз оборачивался auth.ContextWithUser(ctx, username),
создавая цепочку всех прошлых requests. Теперь каждая итерация начинает с original
base context, retaining original input/cancel/source values, и добавляет один
current user layer. Scope/dial/route по-прежнему получают verified current user.

Go parseBasicAuth делает strings.Cut decoded user:password string. Для principal
context сохраняется strings.Clone(username), чтобы substring не удерживал backing
string с password. Содержимое/регистр username не меняются. Это не secure wipe,
не proof освобождения GC/RSS и не удаление прочих SDK/request/credential copies.

### Разрешённое подтверждение

Manual5 final source/diff/caller/lock/wait-graph review: mixed_processor, HTTP,
HTTP limits, original ingress и new handshake_budget. Разобраны before/during/
after timer install/expiry/release, Stop true/false и pending callback, constructor
late return, callback action claim, parser/root Close overlap, original cancel
outcome versus native Close, buffered IO, context/source deadlines и handoff.
Auth missing/bad/correct/reset/fourth challenge, shared deadline/pipeline, flat
ctx user and decoded credential substring source проверены по исходникам.

Go time/sleep.go документирует AfterFunc Stop semantics; SDK auth context и Go
parseBasicAuth/strings.Clone inspected locally. gofmt5+2 exit0 только5 touched Go
files.9 snapshots8existing1absent/task71rev683 сохранены;50 initial +clone.go =51
local source identities unchanged. Evidence: bootstrap-v293-handshake-time-source-evidence.json.
100 acceptance OPEN/56 NOT_RUN; goal/constraints/decisions/checks/tasks0…70 без waiver.

Bookkeeping: empty recall fallback после медленного Git lookup не использован как
потеря требований; полное CLI status683 и собственный original ledger подтвердили
продолжение. Compact JSON читать через parse/selected fields, не TotalCount lines.

### Открытые границы

- 30s/4/8 — initial policies, не measured optimum. Таймер scheduling, cancellation,
  Stop/callback races, context deadline/clock jumps и native socket retirement не
  исполнялись. Long idle keep-alive/interactive auth/header clients могут получить
  early close; interoperability и UX acceptance OPEN. Никакого hard30s/5s shutdown,
  CPU/RSS/GC bound или superiority claim.
- Initial Peek budget не закрывает последующие SOCKS4/5 command/auth parsing и
  UDP association semantics. Global auth rate/admission, cumulative bytes/attempt
  budgets across connections, read/write/time policy других фаз ещё OPEN.
- Transport private loops/TLS/idle/timers/whole SDK/native retirement требуют
  отдельной приёмки; original parent Process/Job/IO/handle reaper обязателен.
  Source timer Stop/callback return не заменяет его и WFP/root-browser/OS proof.
- Full PC00…09/old9/SDK/ABI/OS/UX/release/comparative/perf OPEN; all gatesfalse/
  ranBeforeJob/GUI6R1U1/IPC9/Job8/TUN unavailable/schema2 unsupported.

Урок: Timer.Stop(false) не joins callback. Сначала нужно отозвать его authority,
отдельно дождаться original action/return и только затем делать handoff.

<a id="pc-v294-socks-phases"></a>

## V2-294 — сроки согласования SOCKS4/5 и служебные ответы

**Режим:** PC-01 source/unverified; PC-00…09 и прежние девять этапов активны.
Tests/build/compile/vet/product/SDK/native/network не выполнялись.

### Реализация

- SOCKS4 orchestration вынесен из opaque HandleConnectionEx в небольшой platform
  method: SDK ReadRequest/authenticator/LazyConn остаются прежними. Original
  input-owned30s/earlier-context timer охватывает parse и negative reply, затем
  полностью retire до original prefix capture и route delegation. Нет parse/replay,
  нового транспорта, второго reader или socket deadline overwrite.
- SOCKS5 negotiation выделен в synchronous helper: version/methods/auth/command
  и rejection replies используют один30s срок. Каждое поле/ответ не продлевает
  окно. Verified current user и original input values остаются в context.
  Timer callback retirement подтверждается до CONNECT prefix/LazyConn handoff.
- UDP acquisition и success reply получают отдельные retained owners с тем же
  supplied negotiation deadline. Original UDP reservation остаётся до native bind;
  partial/late handle не теряется. Reply timer завершается до watcher, first
  packet и существующего UDPTimeout lifetime. Timer cancel не доказывает возврат
  native bind/socket и не заменяет parent Process/Job/IO/handle retirement.
- SDK response codecs возвращают common.Error(writer.Write), теряя короткий count
  при nil error. Private single-write adapter возвращает io.ErrShortWrite без retry;
  invalid count — custody fault. Он применяется к explicit SOCKS4/5 control replies,
  не к непрозрачным поздним LazyConn writes или потоку payload.
- SOCKS4 explicit rejection задаёт Destination.Addr=IPv4Unspecified. SDK пишет
  Addr.AsSlice verbatim: invalid zero Addr даёт nil, а не четыре нулевых байта.
  Теперь этот отказ содержит требуемые8 bytes, finite error не содержит username.

### Границы байтов — повторный разбор закреплённого codec

Первичная гипотеза о неограниченной SOCKS4 NUL-string опровергнута исходником:
readString уже rejects length>255. SOCKS5 methods/user/password/domain имеют
one-byte length; address serializer читает fixed IPv4/IPv6 или bounded FQDN.
Новые parsers/дублирующий byte limit не добавлены. Успешные wire requests имеют
конечный размер (SOCKS4a до520, SOCKS5 с auth до1032 bytes); это не heap/RSS,
global admission, cumulative attempts/bytes или CPU bound. Reader может заранее
буферизовать payload в прежних4096 bytes; original prefix ownership сохранён.

### Разрешённое подтверждение и остаток

Manual4 final source/diff/caller review: dispatch, SOCKS4, SOCKS5, budget writer;
разобраны unsupported/auth refusal, short/partial reply, timed-out parse/bind/write,
existing timer release/action fences, cancellation между фазами и оригинальные
UDP/frame/prefix owners. Pinned protocol/auth/adapter/serializer/LazyConn и Go
netip.AsSlice inspected locally;55 source identities сохранены. gofmt4 exit0,
ещё1-file formatting перед read-only diff.8 snapshots7existing1absent/task71rev686.
Evidence: bootstrap-v294-socks-phases-source-evidence.json.
Goal/constraints/decisions/checks/tasks0…70 exact;100 OPEN/56 NOT_RUN/no waiver.

Deferred SDK LazyConn success/failure reply остаётся отдельным OPEN seam: он вне
завершённого negotiation timer и может терять short count; SOCKS4 failure с invalid
Addr требует отдельного consumer fix. Не менять это посредством lifetime-wide
timer, отключения LazyConn или раннего ложного CONNECT success. Global auth/rate/
resource budgets, wire/clock/race/native/SDK/WFP/root-browser/OS/UX/release/perf,
full PC/old9 OPEN.30s — initial policy, не measured optimum или hard shutdown bound.
All gatesfalse/ranBeforeJob/GUI6R1U1/IPC9/Job8/TUN unavailable/schema2 unsupported.

Получен attributed server handoff07.11: direct WS stock probe при Happ ON не
является Windows app/protected-egress приёмкой. Активный атомарный срез завершён
без server/network исполнения; совместимость и конкретные VPS requirements нужно
сверить отдельно по текущим public receipts и PC-owned trust/native consumers.

<a id="pc-v296-socks-reply"></a>

## V2-296 — исходный отложенный ответ SOCKS4/5

**Режим:** PC-01 source/unverified, полный PC-00…09/old9 активен.
Нет tests/build/compile/vet/product/SDK/native/network исполнения.

### Реализация и границы

1. Private mixedSOCKSConnection регистрируется в exact input до SDK route handoff.
   Допускается один owner/input и только original TCP или original retained prefix.
   Нет embedding/Upstream/vector/copy bypass; replaceability=false. Native TCP,
   payload IO и prefix остаются у прежних owners, новая socket/task не создаётся.
2. First report/IO claims one original reply before peer metadata, timer, response
   allocation или write. Explicit ConnHandshakeSuccess использует passed peer bind;
   negative response — прежний pinned ReplyCodeForError. Peer pointer сохраняется
   для original frame custody, но его native Close по-прежнему принадлежит SDK.
3. Initial30s/earlier-context budget относится только к metadata/encode/control
   write. Timer/callback retire до последующего payload Read/Write. Reply codec
   прежний; single-write adapter rejects short nil-error/invalid count. No retry,
   replacement reply, second success/refusal или lifetime-wide handshake timer.
4. Concurrent/repeated callers observe original done + returned/outcome. Unknown
   observation sticky; original first call тоже возвращает cleanup failure, если
   другой observer уже отметил uncertainty. Success/error не перезаписывается
   второй попыткой. После известного short/transport/refusal payload не допускается.
   Нормальный завершённый отказ отличен от неизвестного resource cleanup.
5. Root snapshot seals constructor и future response start, затем native TCP Close
   и original timer retirement предшествуют reply-frame join/IO/handler waits.
   Reply cancel/Close не joins свой frame; value mutex не удерживается при native/
   peer/timer API, cancel или wait. Not-started+sealed — no-acquisition факт, не
   fabricated response done. Pending/failed owner остаётся attached без Close retry.
6. SOCKS4 bind всегда IPv4/four bytes; failure и unknown/IPv6 bind используют
   IPv4Unspecified. Ответ имеет ровно8 bytes. SDK default invalid Addr либо IPv6
   Addr.AsSlice иначе дал бы4 либо20 bytes. SOCKS5 сохраняет bounded SDK serializer.
7. Original completed reply fast path observes done без создания context/timer
   на каждом payload read/write. Это source allocation avoidance, не измеренный
   throughput/latency/CPU/RSS результат.

### Почему сохраняется lazy first-IO readiness

Pinned SDK DNS hijack вызывает HandleStreamDNSRequest/binary.Read без отдельного
ConnHandshakeSuccess; local UoT prepare читает request до обычной packet routing.
Оба существующих consumer опираются на lazy first Read/Write. Их readiness fallback
сохранён; нет unconditional success сразу после parse/перед route selection.
Actual network dial по-прежнему вызывает explicit ReportConnHandshakeSuccess.
Readiness SOCKS не является самостоятельным proof remote dial/protected egress.
Нельзя устранить этот seam ранним общим success, отключением DNS/UoT или блокировкой
всех first-IO до несуществующего для этих services peer report.

### Разрешённое подтверждение

Manual5 final source/diff/caller/lock-wait review: socks4/5, ingress, prefetch и
new socks_reply. Разобраны first/concurrent positive/negative/short/partial reply,
phase end перед payload, current context/Close race, snapshot/late registration,
original timer/root close joins, peer metadata fault и no-unwrap. Pinned LazyConn,
network handshake reporters, route dial/DNS, DNS handler и UoT inspected locally.
gofmt5 exit0;9 snapshots8existing1absent/task71rev690 +61 local source SHA.
Evidence: bootstrap-v296-socks-reply-source-evidence.json.100 OPEN/56 NOT_RUN;
goal/constraints/decisions/checks/tasks0…70 exact/no waiver.

### Остаток и новый конкретный seam

- Scheduler/clock/race/interoperability/performance/native/SDK/OS не исполнялись;
  source/gofmt не доказывают compile/wire/whole cleanup или hard30s/5s bound.
- DNS HandleStreamDNSRequest после parsed message запускает go router.Exchange,
  response buffer/Write вне общего public join. Existing raw IO receipts не
  подтверждают ожидание/возврат этой исходной query goroutine. Per-request DNS
  count/bytes/retained task+message+buffer custody и global auth/admission rates
  остаются отдельной обязательной работой; этот slice их не закрывает.
- Whole SDK private Transport/DNS/outbound tasks и peer native ownership требуют
  отдельной приёмки; original parent Process/Job/IO/handle reaper обязателен.
- Full PC/old9/SDK/ABI/WFP/root-browser/OS/UX/release/perf OPEN; gatesfalse/
  ranBeforeJob/GUI6R1U1/IPC9/Job8/TUN unavailable/schema2 unsupported.

Получен новый public server-compatibility.json: actual WS path!=/ws и origin
Cloudflare Origin CA; stock direct TLS PASS использовал explicit public certificate.
Это конкретные PC-03 contract gaps, не evidence системного TLS renderer или
native Snowden PASS. После фиксации этого среза требуется узкое обновление
PC-VPS requirements/плана, signed descriptor path/TLS authority, без runtime patch.

<a id="pc-v300-dns-tcp"></a>

## V2-300 — исходные запросы TCP DNS и ограниченный конвейер

**Статус: source/unverified.** Реализован узкий PC-01 срез, не закрыт весь DNS,
W4c или PC-00…09.100 OPEN/56 NOT_RUN; tests/fixtures/build/vet/product parser/
SDK/native/network не запускались. После возвращения диска восстановлен тот же
session699 и исходный незавершённый объём; нового допуска исполнения нет.

### Граница существующей политики

Normalized config квалифицируется только при первом точном правиле port=[53],
action=hijack-dns без protocol/outbound/IP/domain/process selectors и без DNS
question rules. В dispatch сверяется первое реальное SDK rule/action/Match.
Нет дополнительного поиска правил или выбора транспорта. Сохраняются original
adapter.DNSRouter из того же service context, DNSQueryOptions и protected final;
копия query metadata получает пустой Destination, как pinned SDK DNS handler.

Путь применяется лишь к exact private mixedIngressConnection/prefetch/SOCKS
root с тем же gate/input/context. SOCKS lazy first Read/Write, HTTP CONNECT
prefetch, общий route guard, generation/subject/selector и original Close
остаются прежними. Обычный HTTP net.Pipe, TUN, UDP, detour и другие layouts идут
через прежний SDK и остаются отдельным незакрытым объёмом. Это не доказательство
общей эквивалентности SDK matching, native egress или полной DNS-изоляции.

### Исходные владельцы и начальные квоты

- Scope прикрепляется к original route до первого чтения заголовка. Запрос
  регистрируется в local/global maps до make/Unpack/go. Сохраняются исходные
  read frame, request, Exchange/copy/pack/write task, byte buffers и timer owner.
- Не более4 запросов на stream,128 на box; обычные owned wire charges8MiB.
  Каждый запрос резервирует request bytes +65536 response bytes +2-byte prefix.
  Дополнительный uncompressed packing storage резервируется отдельно,16MiB
  на box, до response.Copy и make. Uncompressed response ограничен1MiB.
- При заполнении запись ждёт освобождения capacity вне value locks, в том же
  retained10s read/query window (или более раннем parent deadline). Давление
  ресурсов не становится channel-health сигналом и не разрешает direct fallback.
  Четыре query timers + header + первый SOCKS reply укладываются в прежний
  input ceiling8; это не whole-process или глобальная rate-квота.
- Перед Unpack отсекаются невозможные header section counts по минимальной
  wire длине question/RR. Это ограничивает данный malformed input, но не является
  общей верхней границей decoded heap, CPU, SDK caches или RSS.

### Кодек и запись ответа

Reused miekg/dns1.1.72: Len с Compress=false измеряется на shallow view без
изменения borrowed SDK message. PackBuffer требует uncompressedLen+1, даже если
фактический wire ответ сжат. После резервирования выполняется отдельная Copy:
PackBuffer меняет OPT extended-RCODE; исходный SDK/cache object не изменяется
и не очищается. Секции и итоговый TCP message проверяются на uint16 предел.

Полный length-prefixed ответ сериализуется одним original conn.Write под
отдельным write mutex. Короткая запись — ошибка, повтор/склейка не выдаются за
успех. Raw Close не захватывает этот mutex перед прерыванием IO. Неожиданная
отдельная allocation из PackBuffer даёт custody fault и сохраняет owner/charges.
Лимит expanded1MiB может отказать большому wire-valid ответу; совместимость и
подбор квот ещё не измерены. Выигрыш памяти/скорости не заявляется.

### Завершение и порядок блокировок

Route seals future scope/queries, затем закрывает original raw connection;
только после raw close receipt ожидает DNS read frame и каждый retained query.
Лимит observation использует прежние5s. Неизвестный frame/query/timer outcome
sticky: owner/charges сохраняются, route fault запрещает безопасного преемника;
поздний возврат не обнуляет failed. Известный отрицательный protocol/IO outcome
может завершиться и освободить quota, но не допускает payload после ошибки.

Value lock order: gate → scope → query; constructor gate → route. Timer APIs,
DNSRouter.Exchange, native IO, cancel и joins выполняются вне value locks.
Только write mutex охватывает цельную запись; raw close его не ждёт. Query
retirement ожидает read и task-controlled frames, затем original timer; очищает
только собственные byte buffers/references и публикует done перед удалением
записи. Ошибка observer не снимается поздним footer.

Эти receipts относятся к контролируемым read/query/retirement frames. Возврат
Exchange не подтверждает выход внутренних SDK transport/cache workers; query
footer не доказывает literal whole-Go-goroutine/Process/Job/IO/native retirement.
Original parent/helper/worker reapers остаются обязательными и неизменными.

### Источники и остаток

Manual5 final source/diff/caller/lock review: engine, admission, route, DNS scope,
DNS query. Pinned sing-box1.13.19 stream handler/matcher/service/DNSRouter/cache
и miekg/dns codec прочитаны;18 immutable source identities сохранены.
Observed gofmt5 exit0 — только форматирование.9 snapshots7existing2absent,
task71rev698; evidence bootstrap-v300-dns-tcp-source-evidence.json.
Goal/constraints/decisions/checks/tasks0…70 сохраняются без waiver;
100 OPEN/56 NOT_RUN, все bootstrap/bridge/native/chain/relaunch/parent/helper
gatesfalse, ranBeforeJob sticky, GUI6/R1/U1/IPC9/Job8, TUN unavailable,
schema2 unsupported. Android/shared/configs/harness/другие чаты не изменялись.

Следующий source объём: UDP DNS original packet/query/response/buffer custody
и global admission; остальные TCP layouts/SDK hidden resources и global rates.
Clock/cancel/race/short-write/pipeline/interoperability/native/OS/resource/perf/
release/PC-00…09 и все прежние девять TH этапов остаются открытыми.

<a id="pc-v301-dns-udp"></a>

## V2-301 — UDP DNS, исходный таймер и фиксированный SOCKS peer

**Статус: source/unverified, 2026-11-08.** Продолжен PC-01, полный PC-00…09 и
девять TH этапов активны.100 OPEN/56 NOT_RUN; tests/fixtures/build/vet/typecheck/
SDK/parser/native/network не запускались. Original formatter session12851 после
прерывания наблюдался до exit0; повторный formatter вместо ожидания не запускался.

### Общий TCP/UDP путь и прежняя политика

В admissionRouter оба UDP entry points используют новый dispatchPacket после
прежних allowedPacket/registerRoute checks. Локальная DNS orchestration действует
только при том же qualified первом port53/hijack-dns и его реальном SDK Match.
Источник — exact private mixedAssociationConnection или ownedUotConnection с тем
же original mixed input/context/gate. Legacy UoT без известного port53 назначения,
TUN, detour и другие rule layouts остаются SDK-owned и отдельным открытым объёмом.

Переиспользованы forwardingDNSScope/Query из V2-300, original adapter.DNSRouter,
query metadata с пустым Destination и прежний protected final. Не появились второй
resolver, selector, правило маршрутизации, direct fallback или authority setter.
TCP framing/prefix и raw-close-first порядок сохранены source-only.

### Буферы и ограничения

- TCP/UDP делят исходные global128 records/8MiB base wire/16MiB extra packing;
  на scope не более4 записей, включая незавершённые/ожидающие чтение. UDP record
  регистрируется до allocation/read/Unpack/go и резервирует65535 input bytes,
  65536 response scratch и2048 head/tail room. Это начальные консервативные квоты,
  не гарантия128 одновременно активных UDP clients или справедливой очереди.
- buf.With создаёт unmanaged views поверх собственных retained bytes. SDK Release
  таких views не возвращает backing storage в pool. После original read/query/
  pack/write/timer frames очищаются только собственные bytes/references; неизвестный
  outcome сохраняет record/charges. Borrowed DNS/cache messages не очищаются.
- После bounded count sanity/Unpack прежний DNSRouter.Exchange получает запрос.
  Response.Copy выполняется после initial expansion reservation, до изменения
  OPT/truncation/pack. Использованы miekg/dns EDNS size logic и Truncate: минимум512
  без большего EDNS UDPSize. Если TSIG/question-only message после codec остаётся
  больше лимита, ответ отклоняется, а не отправляется oversized datagram.
- UncompressedLen+1 для PackBuffer и отдельный1024-byte head/tail room учтены
  до make. Проверяются alias исходного packing storage и uint16 wire предел.
  Первоначальный1MiB expanded cap остаётся; decoded heap/codec maps/SDK caches/
  CPU/RSS этим не ограничены. Исходный first-packet buffer ceiling не увеличен.

### Исходный UDP activity timer

Один forwardingDNSIdle принадлежит scope до NewTimer и до watcher Go. Watcher
единолично читает timer.C, Reset/Stop того же timer; activity меняет только deadline
под value lock. Как в pinned DNS handler, activity обновляется после datagram read
и успешного Exchange до pack/write. Решение expiry и touch используют один lock:
старый snapshot не может отменить уже продлённый deadline. Late activity не
воскрешает истёкший scope. Срок10s/ранний route cancellation не переписывает
native socket deadlines и не создаёт скрытый SDK timeout worker.

UDP query retained10s operation budget начинается после datagram, не во время
idle read. Он включает decode/Exchange/pack/write; TCP window прежний. Capacity
ожидается вне value locks под original activity lifetime. Старый association
UDPTimeout wrapper сохранён. Hard elapsed-time/cancel/clock/scheduler behavior
не принимался, долгие/нетипичные exchanges и quota tuning требуют приёмки.

Original raw Close идёт до scope frame, original idle install/watcher/Stop и query
joins. Все наблюдения делят прежний5s cleanup context. Timeout/constructor/watcher/
Stop uncertainty sticky; поздний footer не снимает fault. Stop(false) у channel
timer не выдаётся за возврат несуществующего callback. Речь о controlled frames,
не literal whole-Go-goroutine/SDK transport/cache/Process/Job/native retirement.

### Адрес исходного SOCKS UDP клиента

Pinned SDK serverPacketConn меняет remoteAddr на каждом read и использует это
поле для параллельного WriteBuffer. Сам write mutex DNS не защищает от другого
read/отправителя. Исправлена orchestration, исходник SDK не редактировался:

1. Первый packet читается прежним SDK codec под original association owner.
2. До router delegation сохраняется фактический native peer; его IP сверяется
   с captured TCP peer, endpoint фиксируется в original mixedPacket.
3. Для дальнейших reads/writes переиспользован SDK NewBindPacketConn с immutable
   endpoint на том же native socket и прежний SOCKS framing/UDPTimeout.
4. Foreign endpoint packets отбрасываются в исходном ReadFrom без refresh activity/
   native deadline/нового socket; payload очищается. Native short nil-error writes
   становятся io.ErrShortWrite, невозможные counts — custody fault. Retry нет.

Это затрагивает все локальные native SOCKS UDP associations, не только DNS.
Смена UDP source port требует новой association; NAT/port-rebind/client interop
остаётся открытой приёмкой. Peer pinning не аутентифицирует локальный процесс или
первый packet. Request destination DNS сохраняется отдельно для каждого ответа.
UoT использует прежний owned writer/frame/release, native peer pin к нему не применяется.

### Подтверждение и оставшийся объём

Manual7 source/diff/caller/ownership/lock review: admission, shared scope/query,
packet orchestration, idle timer, native UDP и SOCKS association.26 immutable
reviewed source SHA, исходные snapshots11/9existing/2absent и task71rev700 сохранены.
Gofmt batches6+7+1 exit0; это не compile, race, wire, native или performance proof.
Evidence: bootstrap-v301-dns-udp-source-evidence.json. Goal/constraints/decisions/
checks/tasks0…70 без waiver;100 OPEN/56 NOT_RUN/all gatesfalse/ranBeforeJob/
GUI6/R1/U1/IPC9/Job8/TUN unavailable/schema2 unsupported.

Other TCP/UDP rule layouts, SDK hidden resources/caches, глобальные ingress/auth
rates и fairness/idle-read quota pressure требуют продолжения. Lifecycle/clock/
race/EDNS/TSIG/short-write/foreign-peer/client-rebind/native/WFP/OS/UX/release/perf/
полный PC-00…09 и все девять TH этапов не закрыты. Android/shared/configs/harness/
другие чаты/сервер/OS/dependencies не изменялись. Получены новые public server
handoffs7–8 ноября; их адресное PC-00/03 обновление требуется отдельным срезом,
без переноса stock/SFA/Happ результатов на native Snowden или снятия запрета tests.

<a id="pc-v303-mixed-rates"></a>

## V2-303 — темп новых mixed-соединений и неудачных HTTP auth retries

**Статус: source/unverified, 2026-11-08.** Продолжен PC-01; полный PC-00…09 и
девять TH этапов активны.100 OPEN/56 NOT_RUN. Tests/fixtures/build/vet/typecheck/
SDK/parser/native/network/installation не выполнялись.

### Что переиспользовано

В windows/go.mod уже закреплён golang.org/x/time v0.11.0. Прочитаны его rate.go,
go.mod и BSD license. AllowN использует существующий token bucket с собственным
value mutex, без скрытой worker goroutine/таймера. Новая Windows orchestration
переиспользует его, не реализует ещё один token bucket. В go.mod тот же exact
version перенесён из indirect в direct из-за нового import; версии остальных
зависимостей и go.sum не менялись, tidy/download/install не запускались.

Отсутствующий аналог — общий бюджет запуска mixed handler и повторных failed
HTTP auth challenges: прежние ограничения4096 root owners,4 auth failures и30s
окно ограничивали число/длительность, но не скорость повторного создания.
Добавлен узкий forwarding_mixed_rates.go в прежнем core package. Существующие
owned listener/input/rejection/handshake/callback APIs и package boundaries сохранены.

### Начальные два бюджета на один box

| Бюджет | Значения | Где расходуется |
| --- | --- | --- |
| Новые mixed input turns |256 tokens/s, burst512 | Перед native Accept либо при SDK injection до root context/AfterFunc/handler allocation |
| Failed HTTP auth keepalive retries |64 tokens/s, burst128 | Только если ошибочная auth попытка могла бы продолжиться на том же соединении |

Это начальные параметры source policy, не измеренный оптимум и не строгий предел
успешных соединений в любую wall-clock секунду. При pre-Accept gate один native
Accept task может держать уже выданный turn во время ожидания клиента. Дополнительно
существуют kernel backlog/непринятые TCP connections; несколько listeners могут
иметь по одному такому ожидающему turn. Счётчики общие для одного admission/box,
не для всех процессов/engine generations/машины. Новый box получает свежие buckets
только по прежнему lifecycle, без ослабления original retirement gates.

### Native accept и SDK injection

Original accept task ждёт budget до original.Accept. При нехватке использует
прежний waitMixedAcceptRetry с channel timer примерно3.9ms; новый socket/root/
AfterFunc/handler Go в этом ожидании не создаётся. Нет rate.Wait/Reserve, отдельной
очереди sleepers или нового callback worker. Тот же task наблюдается через
прежний acceptDone; closing/context cancellation и listener Close остаются
original native механизмами. Таймер ожидания не выдаётся за native cleanup proof.

После Accept общий registerAcceptedMixedInput по-прежнему прикрепляет raw к gate
до metadata, scheduling или socket API. Он приватен; native loop уже израсходовал
один turn. SDK NewConnectionEx входит через registerMixedInput: его raw acquired
раньше внешним caller, поэтому denied turn передаётся прежнему bounded synchronous
rejection lane до Close/callback. Нет нового root/AfterFunc, per-rejection Go,
неучтённого raw Close или повторной native попытки. При действительной потере
resource custody прежний worker fail-stop/parent reaper остаётся обязательным.

Limiter не выдаёт forwarding authority: gate open/epoch/subject/selector checks,
private diagnostic receipt и current route IO checks неизменны. Отсутствующие
buckets отвергаются при private mixed construction, а не означают unlimited mode.
Обычная нехватка tokens не вызывает routeFault или смену канала в этом limiter.
Фактический failed diagnostic под нагрузкой всё равно требует отдельной корректной
классификации; узкий source slice не доказывает это по всей App цепочке.

### Повторная HTTP аутентификация

После неверного/отсутствующего Basic auth запрос сохраняет прежний407 response,
realm и конечные credential-free ошибки. Keepalive retry возможен только при
старых request/body/четырёх попытках/общем30s условиях и доступном global retry
token. При exhaustion ответ помечается Connection: close, затем original handler
заканчивается через прежнюю owner/Close цепочку. Это ограничение дополнительных
попыток; первая проверка в новом connection ограничивается intake budget.

Успешные credentials, обычные authenticated HTTP requests и payload уже работающих
потоков не расходуют retry bucket. SOCKS5 имеет прежнюю одну auth negotiation на
input и общий intake budget; новая auth policy/изменение SDK Verify не добавлены.
Не создаются credential/username/address-keyed cache, eviction/reset обход или
приоритет на основании Proxy-Authorization/X-Forwarded-For/IP. Для retry проверяются
actual registered original input/context/closing и gate custody. Limiter AllowN
выполняется вне gate/input value locks; сами gates не вызывают callback/OS/joins
под новым value mutex.

### Границы и следующий шаг

Оба бюджета могут задержать законный новый клиент при перегрузке. Per-peer/process
справедливость, защищённая доля private probe, starvation, backlog, CPU/heap/RSS,
total app request rates и подстройка по измерениям пока не реализованы/не приняты.
Local loopback IP/username не идентифицирует доверенный процесс, поэтому фальшивого
priority bypass нет. Native listener/cancel/clock/HTTP407/client reconnect/SDK
injection/race/performance поведение не исполнялось; hard time/resource bounds
и превосходство над другими клиентами не заявляются.

Manual5 final source/diff/caller/lock review плюс manifest: admission constructor,
mixed intake, original input, HTTP auth и new rate adapter.12 immutable reviewed
SHA,10 snapshots9existing1absent/task71rev705; gofmt5+1, no tests/build.
Evidence: bootstrap-v303-mixed-rates-source-evidence.json. Full original goal/
constraints/decisions/checks/tasks0…70 без waiver,100 OPEN/56 NOT_RUN; all bootstrap/
bridge/native/chain/relaunch/parent/helper gates=false, ranBeforeJob sticky,
GUI6/R1/U1/IPC9/Job8/TUN unavailable/schema2 unsupported.

Следующий PC01 source: remaining DNS/ordinary HTTP/SDK resource seams и actual
original-helper/WFP admission integration; generic timing/fairness нельзя выдать
за native acceptance. PC03 accepted signed provision/app-owned entry и full
PC00…09/old9/OS/UX/release остаются. V301 UDP peer/query custody и V302 dated server
facts сохранены. Android/shared/configs/harness/чужие чаты/сервер/OS не изменялись.

<a id="pc-v357-native-admission"></a>

## V2-357 — original capture владеет ресурсом до начала Acquire

2026-11-09. PC-01 W4c. **Partial source, unverified**.

**Source findings:** capture.filter присваивался после policy.Acquire; scope —
после beginTUNScope. При Goexit/другом отсутствии возврата partial result из defer
не попадал в caller assignment. Platform global lease/e.tunScope уже могли владеть
ресурсом, а helper capture видел nil. Sequential revoke мог не дойти до filter
после Goexit в ingress cleanup. Session close не различал unresolved Open с nonzero
output, diagnostic Add не имел return marker. Два scope fallback использовали
cleanupCall, зависящий от nonnil panic payload. Нативная гонка не воспроизводилась.

### Изменения

- Reuse nativeImagePreparation pattern: optional NativeTUNPolicyPreparation /
  NativeTUNLeasePreparation. Production NewLease — только literal allocation,
  без global claim, query/session/filter или callback. Original capture сохраняет
  lease перед in-place Acquire. Старый NativeTUNPolicy.Acquire API сохранён;
  production не переключается на legacy путь после ошибки нового Acquire.
- filterStarted/filterResolved — private negative custody metadata. Unreturned
  legacy call или nil/nil без owner удерживает original helper/socket/image.
  Обычный legacy nil/error разрешает отсутствие ресурса только по trusted policy
  contract: любой claimed resource должен вернуться partial owner. Pure allocator
  normal nil return означает отсутствие acquisition; это не native proof.
- beginTUNScopeOwned проверяет exact protectedHelper.tun/Engine/original empty
  capture и публикует scope в него до первого duplicate/open RPC. Existing private
  beginTUNScope signature сохранён для legacy callers, actual production вызывает
  owned variant. Не присваивается поздний result поверх ранее удержанного owner.
- Reuse lifecycleCleanupStep: ingress и filter revocations зарегистрированы
  до callbacks, исполняются в прежнем порядке. Panic/Goexit на одном шаге не
  отменяет попытку следующего; error не снимает custody. Cleanup timeout cancel
  также deferred. Только original successful scope/Job и filter Close очищают
  соответствующие ссылки; drained sequence — не native retirement authority.
- Два fallback retire scoped engine используют тот же normal-return witness
  вместо cleanupCall. Nil panic payload не превращается в scope.retired=true.
  Остальные shared cleanupCall consumers — отдельное следующее P0 продолжение.
- Lease Acquire one-use; claimed owner публикуется до native work. Pure/busy
  lease Close не закрывает чужой global session и не оставляет ложный busy hold
  после подтверждённого no-acquisition. Actual owned partial lease удерживается.
- Native Add получает return marker; неизвестный Add не допускает session Close.
  Shared session Close отвергает unreturned Open до обращения к numeric handle.
  Closed/closing session не открывается заново; failed Open не становится usable
  diagnostic Add. Sticky original Close behavior и reader custody сохранены.
- Existing actor/intent/worker/image/current runtime checks, six diagnostic rule
  conditions, bounded transcript, protocol и final commit unchanged. Compiled
  nativeTUNForwardingAccepted остаётся false. New interfaces/private metadata
  не являются новой Wails командой, bootstrap receipt или разрешением forwarding.

### Доказательство и предел

Manual четыре original Go diffs; touched-only gofmt batches4+3, оба exit0.
4Go4 owned docs/8 snapshots8 existing/12 initial pinned+4 additional reviewed
inputs (2 inherited pins). Source/FS/hash/checkpoint only. Tests/fixtures/build/
vet/parser/SDK/generation/browser/native/WFP/TUN/network/реальные данные не запускались.
New interfaces/type compatibility и fixture expectations не проверены компиляцией.

Trusted allocation/legacy partial-owner contracts не attestation hostile policy.
No force-cancel, hard OS deadline, proven Job/socket/image/WFP retirement, kernel
containment, atomic Stop/permit, crash/power loss или whole-session proof.
Unreturned native calls могут оставить ограниченный original owner заблокированным;
новый owner не подменяет отсутствие доказательства. Registry/SDK/installer/wholeUX/
W3b/NETGUARD/adaptive/IPv4/IPv6/UDP/OS/release gates остаются OPEN.
100OPEN56NOT_RUN/all gatesfalse/TUN unavailable/schema2 unsupported;55% rough only.
FullPC00…09/old FR/TH/OFF/COM/RQ/TR/TH-00…08 сохраняются.

**Далее:** PC-01 shared cleanupCall normal-return witness и его оставшиеся
consumers; после source work native/Windows приёмка всё ещё требует разрешения.


<a id="pc-v358-cleanup-witness"></a>

## V2-358 — обычный возврат callback и отрицательный terminal потока

2026-11-10. PC-01. **Partial source, unverified**.

**Source findings:** cleanupCall, validateLifecycleSource и completion guard
опирались на nonnil recover payload. nil payload не отличался от успешного
callback. В native TCP closeStreams/run последовательные вызовы могли пропустить
upstream/callback/terminal при Goexit в предыдущем шаге. В lifetimeRemaining
до callback устанавливалась положительная legacy секунда: при nil-payload panic
она могла остаться без ошибки. Runtime/compat configuration не проверялись.

### Изменения

- Existing cleanupCall получил факт обычного возврата. Normal nil/error остаются
  теми же результатами, identity обычной ошибки не меняется. При non-return
  создаётся finite error с типом payload, без его значения. Helper не останавливает
  Goexit и не принуждает native I/O завершиться; для последовательности нужны
  заранее зарегистрированные cleanup steps и собственные original owners.
- validateLifecycleSource и guardLifecycleCompletion переиспользуют общий helper
  вместо двух слабых recover handlers. Context before/after source, downstream-only
  guard под Manager.mu, normal failed-acquisition release, отсутствие нового
  platform lease у explicit legacy/noop probe сохранены. Negative default guard
  во время unwind не является acquired/released lease receipt.
- Native flow сохраняет original onClose/cancel callbacks. Через existing
  lifecycleCleanupStep до исполнения зарегистрированы cancel, stream cleanup,
  SDK callback и terminal publication. closeStreams аналогично регистрирует
  original ingress/upstream snapshots; один callback не подменяет второй поток.
- Failed terminal закрывает done с fault и typed ErrNativeScopeUnavailable:
  ожидание получает отрицательный результат, не зависает на пропущенной публикации.
  Это завершение frame, не доказанное завершение native resource. retire и proven
  отвергают fault; scope close/native receipt по-прежнему требует actual retire /
  context/actor/epoch/deadline/byte facts. onClose получает только finite failure,
  normal path по-прежнему исполняет callbacks один раз в прежнем порядке.
- Смежный lifetime.go включён до правки через дополнительный snapshot и checkpoint.
  Initial seven-file before.json сохранён; before-extended.json добавляет ровно один
  original source. Query callback использует тот же helper и свой return flag:
  interrupted query даёт typed trust_unavailable, а не положительную legacy секунду.
  Ноль с ошибкой — unknown/error convention, не измеренная нулевая длительность.
  Normal error identity/nonpositive query rejection и explicit non-Lifetime
  legacy fallback сохранены. Existing operation/running lifetime Goexit guards
  остаются original owners; helper не превращает Goexit в обычный return.

### Прослеженные потребители

| Потребитель | Что удерживает отрицательный результат |
|---|---|
| Engine Close | cleanupErr/original box, без повторного native Close как proof |
| Mixed listener Close | original listener/accept tasks и fault outcome |
| Proxy restore | ошибка не очищает proxyActive/ownership как successful restore |
| Proxy applied validation | failure не становится положительным readback |
| Lifecycle source/completion/release | исходный terminal/source/lease contract, без Ready из exception |
| Fault signal | best-effort wake, его return никогда не разрешает forwarding |
| Native TCP wait/retire/proven/scope | faulted done не выпускает actor/native receipt |
| Lifetime query/watchers | query interruption restricts trust, no positive default |

Проверены десять исходных cleanupCall callsites; четыре flow calls заменены
существующей sequence, добавлены два lifecycle и один lifetime caller. Текущих
callers девять (definition не считается). Message text не служит authority;
ordinary error/sentinel matching сохраняется. Новых RPC/DTO/deps/горутины нет.

### Доказательство и предел

Manual четыре original Go diffs.4Go4 owned docs/8 snapshots8 existing (7initial+
1 before expansion edit),15 initial+2 lifetime observer inputs. gofmt3+1+1exit0.
Read-only diff/FS/hash/checkpoint only; tests/fixtures/build/vet/parser/SDK/
generation/browser/native/WFP/TUN/network/реальные данные не запускались.
Normal-return logic не доказывает semantic correctness SDK callback, physical
I-O/Job/socket/lock retirement, forced cancellation, hard deadline или absence
native side effects. Compatibility panic payload/runtime modes не исполнялись.
No kernel/Windows/wholeUX/SDK/installer/recovery/release acceptance.
100OPEN56NOT_RUN/all gatesfalse/TUN unavailable/schema2 unsupported;55% rough only.
FullPC00…09/old FR/TH/OFF/COM/RQ/TR/TH-00…08 сохраняются.

**Далее:** source audit PC-01 по требованиям текущего плана: отделить оставшиеся
implementation gaps от native/Windows acceptance, не объявлять gate завершённым
по обычному callback return или отсутствию обнаруженной ошибки.


<a id="pc-v383-wfp-call-return"></a>

## V2-383 — normal native return and original WFP cleanup custody

2026-11-10, **partial source/unverified**, PC-01/W4c.

Source review confirmed that App and core already implement the joint commit.
The defect is in its original WFP call/cleanup bookkeeping, not a missing API.
Begin/Commit/Abort lacked distinct native-return records; reader queries lacked
an unreturned-call barrier. Sequential cleanup could skip Abort on Goexit, and
recording failed grant after cleanup could lose it when cleanup did not return.

- Retain Begin/Commit/Abort ordinary-return facts on the original transaction;
  query/Free/Destroy facts on the original reader; Add/Delete and Close facts on
  the original session. A pure graph predicate checks these exact records before
  another transaction, native cleanup, read, write slot or session reuse.
- Keep fixed-object query GUID input in the original reader. No new session or
  alternate numeric handle repairs an unknown output. A failed native call that
  actually returned is distinct from a call whose return was never observed.
- Register reader Free, enum Destroy and transaction Abort as separate deferred
  steps before executing callbacks. Later steps can do their negative graph check
  during Goexit, but cannot issue another syscall through an unreturned original.
  Returned failed Destroy retains its enum; returned failed Abort can only retry
  on the same original session under the existing restrictive contract.
- Record a failed/unknown committed grant before fallible cleanup and again in
  the finalizer. Cleanup panic/Goexit cannot erase pendingGrant/failedGrant and
  permit a new positive writer. Keep the first failed batch until a fresh
  restrictive transaction on the original owner confirms general permits absent.
- Read-close and native restriction wrappers require ordinary return; nil-payload
  panic or Goexit cannot become a successful cleanup receipt. Finite errors keep
  payload values out of messages. Native status return is not commit authority.
- Existing exact filter policy, C ABI structs, App/core commit, explicit Release
  and compiled forwarding gate are unchanged. Original owner allocation remains
  only under owner == nil; an unknown original owner is never replaced.

5Go5docs/10 existing snapshots; manual5 source diffs;10named0cached;
gofmt5+1exit0. Own receipts: .artifacts/memory/bootstrap-v383-w4c-.
Previous percentage-only answer was no implementation progress; this continuation
resumes original pre-checkpoint943 and completes source/document evidence.
No tests/fixtures/build/vet/types/parser/SDK/native/registry/WFP/TUN/browser/network/generation. Source return records do not prove native interruption, deadlines, cleanup, filter behavior, ABI, compiler compatibility or protected egress. nativeTUNForwardingAccepted remains false; TUN unavailable and schema2 installer unsupported. Root NETGUARD-001/legacy/migration/fullPC00..09/old9/native/OS/release remain OPEN;100OPEN56NOT_RUN.

<a id="pc-v506-parent-permit-operation"></a>

## V2-506 — ordinary parent permit / original lifecycle operation

Ordinary parent permit теперь сохраняет exact original process pointer и
metadata-only view исходной lifecycle-операции. Producer получает original Run
context отдельно от временного diagnostic reservation: обычный release в Run
отменяет diagnostic child, поэтому этот child не может быть identity допуска.
App value wrapper и Manager probe→commit сохраняют original Done. Перед публикацией
проверяются original sources/process; без nonnil Done наблюдение возвращает факты,
но не создаёт parent handoff.

Private admit выделен в компонент; он сначала сжигает оба parent permits, затем
проверяет same original Done/current old+caller sources/deadlines и exact process
до и после RPC. Повторный/fresh context не переносит старый допуск на новую
операцию. Actual RPC context, worker token expiry/one-use consume, original
retirement и native no-steal сохранены. Native forwarding gate остаётся false.

Outer Engine/RPC context callbacks остаются отдельным открытым вопросом; incoming
snapshot выполняется под Engine lock после burn. Это не приёмка Windows, Stop,
partial output, compiler/private fixtures, kernel, ресурсов или скорости.
2Go4docs/5existing1new/manual2diff1new/gofmt2+1+1exit0/12reviewed0SDK;
q87s95/261cache190verbatim59owned unchanged. No tests/build/SDK/product/native/
admin/live/network/newGit/otherroles/chats. FullPC00-09/all9/72tasks/100OPEN56 product
NOT_RUN+2 historic Git checks; TUN unavailable/schema2 unsupported/rootNETGUARD001 OPEN.

<a id="pc-v508-parent-admit-snapshot"></a>

## V2-508 — ordinary admit metadata outside Engine lock

Обычный admit сначала под коротким Engine lock сжигает обе parent-квитанции
и сохраняет exact original permit в revocable forwardingConsume. Чтение incoming
Done/Deadline/source values выполняется после unlock. Общий сброс отзывает также
эту попытку: Stop, новый probe или повторный admit не оставляют её действующей.

После snapshot повторный lock проверяет ту же попытку, исходную операцию,
current old+caller sources/deadlines, exact process и конфигурацию до RPC.
Отложенная очистка убирает только свой original permit и выполняется после
освобождения второго lock; ошибка/panic/Goexit не очищает квитанцию преемника.
После RPC проверки используют fixed original/caller metadata без повторного
вызова исходных context getters. Actual RPC получает прежний real ctx.

Это source/unverified. RPC/native/context callbacks под outer Engine lock
остаются открытым объёмом; нет доказательства полного Stop, прерываемости,
Windows cleanup, скорости или ресурсов. Native forwarding gate остаётся false.
3Go4docs/all7existing/manual3diff/gofmt3exit0/12reviewed0SDK;
q87s95/261cache190verbatim59owned unchanged. No tests/build/SDK/product/native/
admin/live/network/newGit/otherroles/chats. FullPC00-09/all9/72tasks/100OPEN56 product
NOT_RUN+2 historic Git checks; TUN unavailable/schema2 unsupported/rootNETGUARD001 OPEN.

<a id="pc-v509-admit-rpc-context"></a>

## V2-509 — ordinary admit RPC context / final cancellation fence

Для ordinary admit RPC добавлен отдельный контекст: real stdlib child
готовится вне Engine lock, Deadline фиксируется как min(original deadline,
control budget), а core source pointers сохраняются из исходной операции.
Done/Err и cancellation-key lookup обслуживает stdlib child; metadata-only view
не используется как RPC parent. Это сохраняет отмену и не сбрасывает бюджет
после ожидания Engine lock. Истёкший бюджет до request не резервирует protocol ID.

RPC остаётся сериализованным у исходного process/pipe owner. После его возврата
lock освобождается, отмена RPC child вызывается один раз и должна нормально
вернуться, затем под новым lock проверяются исходные consume pointer, операция,
process/config и live usable state. Обычная отмена короткого RPC child не отменяет
lifecycle operation. Deferred cleanup после последней проверки и повторный
parent unlink исключены из успешного пути; преемник не очищается старой попыткой.

Scope — текущие IPC getters, source/unverified. Unknown Value keys могут
делегировать к caller; custom constructor/native callbacks/другие Engine paths
и полный Stop ещё открыты. Нет доказательства compiler/Windows cleanup,
прерываемости, скорости или ресурсов. Go1.26.7 context source просмотрен офлайн.
2Go4docs/5existing1new/manual2diff1new/gofmt2+1+1exit0/13project2stdlib0SDK;
q87s95/261cache190verbatim59owned unchanged. No tests/build/SDK/product/native/
admin/live/network/newGit/otherroles/chats. FullPC00-09/all9/72tasks/100OPEN56 product
NOT_RUN+2 historic Git checks; TUN unavailable/schema2 unsupported/rootNETGUARD001 OPEN.

<a id="pc-v510-probe-rpc-context"></a>

## V2-510 — ordinary/native probe RPC context

Оба private probe wrappers готовят RPC-контекст до Engine lock через общий
компонент. Для него сохраняется snapshot фактического diagnostic ctx: более
короткий deadline и дополнительные negative sources не теряются. Исходная
lifecycle operation остаётся отдельной identity для parent/native допуска.
Request settings/timeout проверяются существующим валидатором; native nonce
добавляется только после исходного claim.

Ошибки подготовки сохраняют ordinary native-no-steal либо native clear/burn;
отмена RPC child выполняется после Engine unlock. Locked producer принимает
prepared context и отклоняет nil. Общая фабрика сверяет Done/deadline metadata
с реальным parent; при уже ограничивающем earlier/equal deadline использует
WithCancel без дублирующего таймера. Deadline не продлевается, source binding
не обновляется, metadata-only view не становится RPC parent.

Source/unverified: generic private fixtures/custom parent/unknown Value/native
callbacks/другие Engine entry paths ещё открыты. Нет доказательства полного
Stop, compiler/Windows cleanup, прерываемости, скорости или ресурсов.
2Go4docs/all6existing/manual2diff/gofmt2exit0/14project2stdlib0SDK;
q87s95/261cache190verbatim59owned unchanged. No tests/build/SDK/product/native/
admin/live/network/newGit/otherroles/chats. FullPC00-09/all9/72tasks/100OPEN56 product
NOT_RUN+2 historic Git checks; TUN unavailable/schema2 unsupported/rootNETGUARD001 OPEN.

<a id="pc-v511-current-runtime-context"></a>

## V2-511 — CurrentRuntime original observation после RPC cleanup

CurrentRuntime готовит original metadata и общий RPC child до Engine lock.
Typed helper выполняет current request через прежние protocol/I-O/process
границы с exact workerStateReplyScope. После unlock отмена child вызывается
один раз и должна нормально вернуться; затем проверяются original sources,
absolute budget, тот же process/instance, live usable state и последний command ID.

State scope теперь удерживает исходный immutable raw header для всех state
actions. Final fence сравнивает именно этот header/length, без нового parse/hash
или копии; исходный ограниченный буфер остаётся жив до конца проверки. Start
correlation сохранена. Смена команды/конфига/процесса отзывает старое наблюдение:
возвращается empty identity, преемник не очищается. Это read-only факт, не permit,
forwarding authority или доказательство kernel health.

Source/unverified. SwitchDefaultContext, legacy/direct current helpers и Start
ещё не переведены; native calls/unknown Value/custom parent/hard Stop открыты.
Нет доказательства compiler/Windows cleanup, прерываемости, скорости или ресурсов.
2Go4docs/all6existing/manual2diff/gofmt2exit0/13project2stdlib0SDK;
q87s95/261cache190verbatim59owned unchanged. No tests/build/SDK/product/native/
admin/live/network/newGit/otherroles/chats. FullPC00-09/all9/72tasks/100OPEN56 product
NOT_RUN+2 historic Git checks; TUN unavailable/schema2 unsupported/rootNETGUARD001 OPEN.

<a id="pc-v512-switch-rpc-context"></a>

## V2-512 — isolated switch RPC / original mutation acceptance

SwitchDefaultContext выделен в selector-компонент. Короткий value lock
сохраняет исходные isolated process/instance/raw header до чтения контекста.
RPC готовится вне Engine lock; фактическое переключение требует того же origin
и сохраняет старые request scope/ID/digest/targetRaw publication/reaper gates.
Embedded worker/local selector и его rollback сохранены отдельной веткой.

Отмена RPC вызывается один раз вне lock, после неё проверяются original source,
budget, process/instance и exact published target graph. Обычные status queries
не считаются новой selection. Поздний отказ или abnormal cancel использует
original reaper только при том же процессе и graph; преемник не очищается.
Fallible reap не повторяется abnormal finalizer после уже обработанного отказа.

Новый private unconfirmed-switch marker различает обычный preflight reject
и возможное уже завершённое изменение. Manager выбирает StateError и прежний
owned proxy restore даже при live process, не подтверждая старый канал по одному
Status=running. Marker сам не доказывает reconciliation/cleanup.

Source/unverified. Start/legacy current/native locks/unknown Value/custom parent
и hard Stop остаются открытыми. Нет доказательства compiler/Windows cleanup,
прерываемости, скорости или ресурсов.
3Go4docs/6existing1new/manual3diff1new/gofmt3+1exit0/13project2stdlib0SDK;
q87s95/261cache190verbatim59owned unchanged. No tests/build/SDK/product/native/
admin/live/network/newGit/otherroles/chats. FullPC00-09/all9/72tasks/100OPEN56 product
NOT_RUN+2 historic Git checks; TUN unavailable/schema2 unsupported/rootNETGUARD001 OPEN.

<a id="pc-v513-isolated-start-context"></a>

## V2-513 — isolated Start / revocable pre-native reservation

Public isolated Start выделен в отдельный компонент. После original context
snapshot короткий value lock регистрирует exact pre-native attempt. RPC child
и protected config normalization/legacy file validation готовятся вне Engine lock.
Stop/close отзывает регистрацию даже без процесса; отложенная очистка убирает
только свой slot. Каждая попытка удерживает свой исходный raw.

Prepared start проверяет тот же slot/raw и отсутствие другого owner, публикует
прошедший прежний валидатор target и запускает прежний worker protocol. Exact
partial process сохраняется на attempt до native acquisition. Перед fallible
error reap отмечается факт попытки: abnormal finalizer не повторяет неизвестный
результат. Separate own-slot defer выполняется и при Goexit; физический unknown
owner остаётся у Engine/Job, а не исчезает вместе с metadata.

Hello/start stage caps сохранены внутри общего absolute sum budget. RPC cancel
вне lock вызывается один раз до final original source/budget/process/target fence.
Это не lifetime worker: embedded start сохраняет background runCtx и detach.
Reload остаётся отдельным непереведённым путём; native/file/SDK interruption,
unknown Value/custom parent и hard Stop ещё открыты.

Source/unverified; нет доказательства compiler/Windows cleanup, скорости или ресурсов.
3Go4docs/6existing1new/manual3diff1new/gofmt3+1exit0/13project2stdlib0SDK;
q87s95/261cache190verbatim59owned unchanged. No tests/build/SDK/product/native/
admin/live/network/newGit/otherroles/chats. FullPC00-09/all9/72tasks/100OPEN56 product
NOT_RUN+2 historic Git checks; TUN unavailable/schema2 unsupported/rootNETGUARD001 OPEN.

<a id="pc-v514-isolated-reload-context"></a>

## V2-514 — isolated Reload / validated replacement and rollback

Isolated Reload выделен в компонент с exact original process/config reservation
на всё время validation→old close→candidate→возможный rollback. Общий negative
origin переиспользуется со Switch. RPC и полная проверка кандидата готовятся вне
Engine lock. Stop отзывает parent; Start и Switch отклоняют незавершённый lifecycle,
включая промежуток без процесса. Resource-only close сохраняет свой reload parent.

В зарегистрированной попытке ErrReloadRejected требует untouched current original, не busy/changed
owner. Факт начала замены фиксируется до old close и возвращается отдельно;
решение об откате опирается на него и actual no-process/no-cleanup-fault/current
slot/source/last graph. Для rollback проверяются сохранённые старые rendered bytes
вне lock; gates повторяются перед новым child. Неизвестный candidate owner не
заменяется. Candidate/rollback используют прежние start records, partial owner,
Job/I-O reaper и запрет повторного unknown cleanup.

Cancel RPC выполняется один раз вне lock до final child/graph/source/budget fence.
Old-config-restored marker требует actual accepted rollback worker; Manager
по-прежнему делает fresh protected probe/commit. Embedded lifetime/rollback
сохранены. Raw isolated start bypass отклоняется; starter принимает prepared ctx
и current nonnil child. Общий бюджет составлен из прежних stage ceilings.

Source/unverified. Native/SDK interruption, unknown Value/custom parent, legacy
controls и hard Stop открыты. Нет доказательства compiler/Windows cleanup,
прерываемости, скорости или ресурсов.
6Go4docs/8existing2new/manual6diff2new/gofmt6+1+1+1exit0/13project2stdlib0SDK;
q87s95/261cache190verbatim59owned unchanged. No tests/build/SDK/product/native/
admin/live/network/newGit/otherroles/chats. FullPC00-09/all9/72tasks/100OPEN56 product
NOT_RUN+2 historic Git checks; TUN unavailable/schema2 unsupported/rootNETGUARD001 OPEN.

<a id="pc-v515-dns-control-context"></a>

## V2-515 — service-DNS parent command / original process slot

Parent service-DNS pause/resume выделен из child-side DNS в свой компонент.
Проверка expected instance/digest/selector/TUN/epoch сохранена; exact original
process/instance/raw удерживается private pending slot самого процесса. Конкурирующая
DNS-команда не обгоняет старую; новый worker этот slot не наследует. Подготовка
RPC и cancel выполняются вне Engine mutex, actual pipe operation сериализована.

Факт возможной отправки отмечается до request; прежний no-ID source-canceled
результат сохраняет no-churn поведение. После cancel повторяются original source,
fixed deadline, slot, graph и live checks. Ошибка очищает только прежний process/
graph без нового lifecycle/native owner. Reap отмечается до fallible вызова,
own-slot discard выполняется отдельным defer даже при abnormal cleanup.
Существующий protocol ID отличает свой DNS request от новой non-current команды:
новая команда отзывает exact DNS slot до exchange, current/read-only сохраняет.
Stop/resource close и общий reaper отзывают slot до fallible callbacks. Старое
DNS-завершение не очищает новый owner даже при прежнем process/config.

Isolated CurrentDefault использует существующий CurrentRuntime; embedded selector
и child-side admitted-DNS/finite fault/wire не изменены. Новая forwarding/native/
path-health authority не появляется. Source/unverified: native/SDK/custom context/
unknown Value, hard Stop, concurrency/OS/resources/speed остаются открыты.
4Go4docs/7existing1new/manual4diff1new/gofmt4+2+1+3exit0/15project2stdlib0SDK;
q87s95/261cache190verbatim59owned unchanged. No tests/build/SDK/product/native/
admin/live/network/newGit/otherroles/chats. FullPC00-09/all9/72tasks/100OPEN56 product
NOT_RUN+2 historic Git checks; TUN unavailable/schema2 unsupported/rootNETGUARD001 OPEN.

<a id="pc-v516-native-consume-context"></a>

## V2-516 — parent native consume / capture before context

CommitPreparedNative выделен в parent-компонент. Он захватывает исходные
permit/process/instance/raw и отзывает обе родительские линии до чтения контекста.
Private consume-slot принадлежит исходному процессу. Подготовка RPC и cancel
выполняются вне Engine mutex; Manager проверяется вне lock после подготовки и
после cancel. До выдачи opaque admission повторяются source/deadline/slot/graph/
live/permit checks. Строка nativeTUNForwardingAccepted=false сохранена; закрытый
путь после burn не вызывает context getters. Prepared helper также проверяет gate.

Свой protocol ID сохраняет consume-slot; новая non-current команда отзывает его
до exchange. Shared clear/Stop/resource close/reaper отзывают slot до fallible
работы. Read-only current не меняет его. Факт возможной отправки и попытка reap
фиксируются до вызовов; no-ID source cancellation не очищает незатронутый worker
и не возвращает burned token. Own-slot discard выполняется отдельно даже при
abnormal cleanup, не затрагивая нового владельца.

App WFP compensation, NativeTUNAdmission, Manager fresh runtime/activation и wire
сохранены. Source/unverified: native begin/end scopes, unknown Value/custom ctx,
SDK/native interruption, concurrency/OS/latency/resources остаются открыты.
4Go4docs/7existing1new/manual4diff1new/gofmt4exit0/16project2stdlib0SDK;
q87s95/261cache190verbatim59owned unchanged. No tests/build/SDK/product/native/
admin/live/network/newGit/otherroles/chats. FullPC00-09/all9/72tasks/100OPEN56 product
NOT_RUN+2 historic Git checks; TUN unavailable/schema2 unsupported/rootNETGUARD001 OPEN.

<a id="pc-v517-tun-open-context"></a>

## V2-517 — native TUN open / original partial target custody

Открытие TUN scope выделено в компонент. Private pending slot исходного процесса
фиксирует process/instance/raw/helper/image/capture до context getters. Caller
по-прежнему удерживает protectedHelper lane. Подготовка RPC и один cancel вне
Engine; actual image/current reply/target duplication/open сериализованы через
prepared context. Общий логический бюджет состоит из двух прежних control caps
и сохраняет более ранний deadline родителя; native preemption не доказан.

Частичный scope сохраняется в Engine/capture до DuplicateHandle. Remote handle
не закрывается в родительской таблице: при отказе используется исходный target Job.
После cancel повторяются original source/deadline/slot/graph/scope/live/helper
checks. Success снимает свой slot под тем же final lock; failure сохраняет partial
scope и ограничивает reap прежним graph/owner. Отдельный metadata discard работает
при abnormal cleanup. Новые non-current команды/shared clear/Stop отзывают slot;
свой open ID/current read сохраняют его. Switch и probe учитывают scoped owner.

Факт retirementAttempted хранится на исходном scope и фиксируется реальным reaper
до fallible work. Automatic end/revoke не повторяет неизвестную попытку; retired
по-прежнему требует actual Job retirement, closed — отдельный scope-close ACK.
Подготовка контекста end RPC остаётся следующим незавершённым срезом. Native/SDK/
unknown Value/custom ctx/compiler/concurrency/Windows/latency/resources OPEN.
6Go4docs/9existing1new/manual6diff1new/gofmt6+1exit0/20project2stdlib0SDK;
q87s95/261cache190verbatim59owned unchanged. No tests/build/SDK/product/native/
admin/live/network/newGit/otherroles/chats. FullPC00-09/all9/72tasks/100OPEN56 product
NOT_RUN+2 historic Git checks; TUN unavailable/schema2 unsupported/rootNETGUARD001 OPEN.

<a id="pc-v518-tun-close-context"></a>

## V2-518 — native TUN close / original cleanup receipt

Закрытие TUN scope выделено в parent-компонент. Exact scope/process/instance/raw
фиксируются до context getters, старые parent permits отзываются до публикации
closing slot. Проверяются old subject и отсутствие duplicate/unknown retirement/
нового lifecycle owner. Подготовка RPC и один cancel вне Engine; actual command
использует prepared context и прежний control cap/ранний parent deadline.

Только валидный нормальный ответ исходного worker сохраняет closed как факт
scope cleanup и private ACK. Он хранится до outer cancel независимо от поздней
ошибки операции; это не Job retirement, current success или admission. Nil result
операции требует final original source/deadline/graph/slot/ACK/live fence после
cancel. Success снимает own slot под тем же lock. Newer non-current/shared clear/
Stop/reaper отзывают попытку; own ID/current read сохраняют её.

Failure очищает только unclosed/unretired исходный scope без нового lifecycle
owner; persistent retirementAttempted исключает unknown retry. Отдельный own-slot
discard работает при abnormal cleanup. Late open rejection учитывает closing owner.
Существующий retireCapture consumer не изменён: он использует только реальный
closed/retired факт. Positive observation по-прежнему требует original foreground
scope, helper/filter/Job cleanup и fresh runtime/worker confirmation. Child proof/
wire/TTL/WFP/activation сохранены; native/SDK/custom context/OS приёмка OPEN.
5Go4docs/8existing1new/manual5diff1new/gofmt5exit0/16project2stdlib0SDK;
q87s95/261cache190verbatim59owned unchanged. No tests/build/SDK/product/native/
admin/live/network/newGit/otherroles/chats. FullPC00-09/all9/72tasks/100OPEN56 product
NOT_RUN+2 historic Git checks; TUN unavailable/schema2 unsupported/rootNETGUARD001 OPEN.

<a id="pc-v519-worker-context-contracts"></a>

## V2-519 — parent worker / RPC and metadata types

Фактические parent request/exchange/current-switch/source-before-ID теперь
принимают workerRPCContext. Обход production callers показал, что normal paths
уже передавали prepared child. Nil child/process проверяются до getters/ID.
Exchange создаёт реальный stdlib child от typed parent; исходный frame/I-O/
cancel/correlation/final-fence body сохранён побайтно.

Fixed requestStop создаёт только stdlib Background/deadline, не читая контекст
пользователя. Под Engine это отдельный negative cleanup путь с прежним close cap
и ancestor exception; reply не заменяет original Job retirement. Native observation
хранит только nativeTUNOperation snapshot; claim/confirm/preparation разделяют
typed metadata и prepared RPC. Nil metadata проверяется до getters, native probe
передаёт исходный snapshot без возврата к context.Context. Burn/identity/TTL/ACK
не изменены; новая authority или activation не появляется.

Source/unverified. Internal signature/private fixture compatibility не проверена
компилятором, fixtures не менялись. Unknown Value/custom parent/native/SDK/hard
Stop и полная runtime/Windows/resources приёмка остаются OPEN.
7Go4docs/11existing0new/manual7diff0new/gofmt7exit0/20project2stdlib0SDK;
q87s95/261cache190verbatim59owned unchanged. No tests/build/SDK/product/native/
admin/live/network/newGit/otherroles/chats. FullPC00-09/all9/72tasks/100OPEN56 product
NOT_RUN+2 historic Git checks; TUN unavailable/schema2 unsupported/rootNETGUARD001 OPEN.

<a id="pc-v520-probe-publication"></a>

## V2-520 — original probe / publish after parent cleanup

Parent probe раньше сохранял permit внутри Engine до deferred RPC cancel
и release диагностического бюджета. Теперь private publication slot исходного
worker удерживает process/instance/raw/lifecycle/ID и permit-кандидаты до обычного
возврата обоих cleanup. Native-кандидат сохраняет прежний bounded no-steal, но не
виден как native preparation/consumption. Shared clear/Stop/reaper/new non-current
request отзывают stage; own ID/current read сохраняют его.

RPC удерживает точный diagnostic parent metadata отдельно от real stdlib child.
После cancel проверяется ещё живой diagnostic Done/source/budget/owner. Затем
Run выполняет release один раз вне Engine. Final check учитывает original lifecycle,
fixed deadline, frozen negative sources, exact slot/graph/live/expiry, сохраняя
различие с намеренной отменой собственного diagnostic child. Только после этого
публикуются parent permits и снимается own slot под тем же lock.

Abnormal/error удаляет только own stage и burns native source; stale metadata не
очищает successor. Existing actual IPC failure reaper сохранён, no-ID source cancel
не очищает незатронутый worker. Diagnostic pool и local worker/flow/wire/TTL/WFP/
activation не менялись. Source/unverified: compiler/private fixtures/concurrency/
cancel-release/outer App-Manager/native/Windows/resources приёмка OPEN.
5Go4docs/8existing1new/manual5diff1new/gofmt5+2exit0/22project2stdlib0SDK;
q87s95/261cache190verbatim59owned unchanged. No tests/build/SDK/product/native/
admin/live/network/newGit/otherroles/chats. FullPC00-09/all9/72tasks/100OPEN56 product
NOT_RUN+2 historic Git checks; TUN unavailable/schema2 unsupported/rootNETGUARD001 OPEN.

<a id="pc-v521-root-probe-publication"></a>

## V2-521 — App native preparation / publish after report finalizer

App раньше сохранял native lease до deferred finalizer отчёта, проверки доверия
и исходного subject. Новый private record одного Run резервируется до callbacks,
удерживает validated lease как stage и публикует его только после обычного возврата
finalizer. Exact source-bound context/subject сохраняется; исходный runtime →
Manager → App → epoch value lease удерживается до записи root slot. Под mutex slot
нет context/trust/native/Engine/I-O callbacks; existing subject guard переиспользован.

BeforeChange/Stop clear и take отзывают pending record. Late completion не может
вернуть отозванную попытку; discard сверяет exact pending/candidate pointer и не
трогает successor. Независимый abnormal discard охватывает report formatting и
release исходного subject. Stale/trust/unsafe/no-native result удаляет own stage.
Ошибка final identity сохраняет original report scope и restrictive path.

Periodic/non-TUN не резервируют slot. Root lease не является forwarding admission,
ошибка диагностики не становится PASS. Core permits/worker/wire/TTL/diagnostic pool/
Manager/lifetime/Job reaper/WFP/native commit body сохранены; compiled native gate
false. Source/unverified: compiler/private fixtures/finalizer/concurrent invalidation/
context callbacks/lock release/native ABI/Windows/latency/resources приёмка OPEN.
3Go4docs/6existing1new/manual3diff1new/gofmt3exit0/16project2stdlib0SDK;
q87s95/261cache190verbatim59owned unchanged. No tests/fixtures/build/SDK/product/
native/admin/live/VPN/network changes/new commit-push/otherroles/chats. Authorized
Git ref and collaborator read-only refresh only. FullPC00-09/all9/72tasks/100OPEN56
product NOT_RUN+2 historic Git; TUN unavailable/schema2 unsupported/rootNETGUARD001 OPEN.

<a id="pc-v522-terminal-trust-budget"></a>

## V2-522 — срок trust observation перед terminal handoff

После join operationLifetime код раньше отбрасывал duration последней проверки. Source/native proxy readback и source lease release могли завершиться уже после её срока.

Новый private `lifecycleTrustBudget` фиксирует monotonic начало реального callback. После source/proxy readback выполняется новый sample вне locks. Затем неизменный бюджет проверяется после guard/release и до/после резервирования running-watch перед active/done handoff. Истёкший budget сохраняет original owner/report invalidation/cleanup. Только exact новый observer, который ещё не запускался, отменяется и удаляется без фиктивного done.

`SessionValidity.Remaining` включает секундный потолок повторного наблюдения за часами: утрата актуальности — `trust_unavailable`, не утверждение об истечении конкретного сертификата. Legacy non-Lifetime не получает искусственный TTL. Worker/WFP/Job и тела samplers не менялись. Context callbacks, short-duration/concurrency/compiler/private fixtures/native/OS и фактическое разрешение трафика остаются непроверенными.

Source/unverified: 3 Go + 4 docs; manual diff и gofmt3 exit0. Тесты/сборки/native/сеть не запускались; полный PC-00…09/TH-00…08 и Windows-приёмка остаются OPEN.

<a id="pc-v523-terminal-context"></a>

## V2-523 — terminal completion без original ctx getters под lock

`terminalContextCause` и App terminal guard раньше вызывали методы исходного Context под Manager/App/epoch locks. Новый private completion удерживает original Done/deadline/core source pointers и уже известный Cause, подготовленные вне locks. Metadata переиспользует existing nativeTUNOperation только для negative checks, никогда как RPC/SDK context или parent.

Optional `LifecycleCompletionPreparationProbe` строит App downstream guard заранее. Closure сохраняет exact original intent/operation/path, без raw ctx; после захвата App/epoch сравнивает state/atomics. Core повторяет metadata check после guard/release и до/после watch reservation; original operation/cleanup/task receipts сохраняются. Legacy сигнатура оставлена, но arbitrary Value fallback отсутствует: custom keys требуют preparation refinement.

Late Done/source revocation запрещает completion; точная классификация позднего Cause, private fixtures/compiler, custom adapter callbacks и остальные App subject/native context paths остаются OPEN. Worker/Job/WFP/samplers/native false gate не менялись.

Source/unverified: 5 Go + 4 docs; просмотр diff, gofmt5+1 exit0. Тесты/сборки/native/сеть не запускались. Полный PC-00…09/TH-00…08 и Windows-приёмка OPEN.

<a id="pc-v524-app-publication-fence"></a>

## V2-524 — App publication без original ctx getters под lock

Opaque `core.OperationFence` переиспользует исходные Done/deadline/core source pointers. Внешний Err проверяется при capture; `Current` работает только с metadata. Тип не реализует Context, не владеет отменой и не заменяет реальный RPC/native parent.

Private publication capture сохраняет копию исходных App scope values и intent/operation/path pointers до runtime/Manager/App/epoch locks. Subject.lockCurrent, retained/started/applied state, original TUN scope и metrics parent проверяют эти сохранённые источники. Revocation после ожидания locks также запрещает публикацию; successor не подставляется. Existing release/clock/config/epoch/physical cleanup сохраняются. Nonlifecycle report path не получает нового требования к ctx.

TUN сохраняет original operation fence при создании scope до helper/native acquisition; новый caller не обновляет его. Metrics перестал вызывать parent.Err под publication locks. Другие nativeProbeScope paths, metrics interface equality, custom context/reentrancy/compiler/private fixtures и Windows/runtime приёмка остаются OPEN. Worker/Manager/WFP/samplers/native false gate не менялись.

Source/unverified: 6 Go + 4 docs, включая 2 новых компонента; diff просмотрен, gofmt6+1 exit0. Тесты/сборки/native/сеть не запускались. Полный PC-00…09/TH-00…08 и Windows-приёмка OPEN.

<a id="pc-v525-native-observation-fence"></a>

## V2-525 — optional native observation / continuity и context identity

Native scope раньше вызывал ctx.Err под value locks и проверял только номер эпохи; netepoch source подтверждает, что down не увеличивает номер. Теперь caller/action publication подготавливается вне locks, а scope сохраняет original path до native work и проверяет path identity/current/down/power вместе с прежними owner/config/clock checks. Foreground fence повторяется до draft/final write.

Срок выполненного факта отделён от жизни foreground action: после own cancel Background-read по-прежнему проверяет original intent/path/owner/TTL. Никакой новый probe/forwarding authority или mandatory health outcome из cache не возникает.

Metrics/adapter parent используют общий exact identity helper: reflect.Value.Comparable проверяет само значение, включая вложенные interfaces, прежде чем выполнять equality. Неустановленная identity делает optional observation unavailable; Done/deadline не используются как замена. Adapter parent также повторяет сохранённый fence после App.mu. Query actor/context factories и остальные App lifetime getters под locks остаются OPEN.

Source/unverified: 5 Go + 4 docs; diff просмотрен, gofmt5 exit0. Тесты/сборки/native/сеть не запускались. Полный PC-00…09/TH-00…08 и Windows-приёмка OPEN.

<a id="pc-v526-app-lifetime"></a>

## V2-526 — original App lifetime вне value-lock callbacks

`App.ctxFence` фиксирует исходный root lifetime. Wails startup резервирует оригинальную подготовку и читает Context вне App.mu до ресурсов; после проверки closing/current публикует metadata. Ready/final startup predicates и прямые root checks lifecycle/retry/DNS/power/restart/protection/update/UI/diagnostic/tray/proxy recovery используют общий value-only helper. DNS/netwatch берут App snapshot до registry locks, без обратного захвата App.mu.

Проверка всех NewApp-конструкторов обнаружила три headless multi-assignment initializer. Их restricted App получает original lifetime отдельно, после регистрации прежнего cleanup и до native work; installedImageChanged установлен до callbacks. Wails startup, vault/network preparation и code acceptance этим не разрешаются. Уже связанный App не перепривязывается к другому контексту.

Живой fence не означает readiness/trust/retirement/forwarding; zero fence не имеет fallback на raw getters. Real ctx остаётся настоящим SDK/network parent. Отмена startup не запускает acquisitions; иная недоступность original context даёт конечную sticky startup failure. Child context factories/cancel, caller ctx getters и adapter owner mutex ещё требуют разбора. Private fixtures с прямым присваиванием ctx требуют отдельной разрешённой миграции; их не меняли.

Source/unverified: 26 Go + 4 docs, один новый helper; diff просмотрен, gofmt25+1+5 exit0. Тесты/сборки/native/сеть не запускались. Полный PC-00…09/TH-00…08 и Windows-приёмка OPEN.

<a id="pc-v527-context-value-locks"></a>

## V2-527 — staging/adapter context work вне value locks

Staging сначала сохраняет original App parent/lifetime/registration/path. Context getters и создание real value wrappers выполняются вне App/epoch locks; перед возвратом проверяются исходные source/path/down/power/lifetime. Уже привязанный старый путь не заменяется новым. Этот этап не создаёт cancel child, timer, goroutine, native owner или forwarding authority.

Adapter registry теперь защищает только exact owner slot. Исходный parent сохраняется до constructor/query; mutex освобождён на время real WithTimeout, actual native-query frame и original cancel retirement. Slot остаётся занят до обычного возврата исходных frame/cancel; pending/abnormal не дают replacement. Final own-pointer/parent-fence/deadline gate не вызывает original getters и не стирает successor. Actual query и retirement body сохранены; это не OS-quiescence/RSS/latency proof.

Foreground beginOperation, bindUpdateOperationLocked, proxy context factories и callbacks отмены под App.mu остаются следующим объёмом.

Source/unverified: 2 Go + 4 docs; diff просмотрен, gofmt2+1 exit0. Тесты/сборки/native/сеть не запускались. Полный PC-00…09/TH-00…08 и Windows-приёмка OPEN.

<a id="pc-v528-operation-cancel"></a>

## V2-528 — отмена исходной операции вне App value locks

`appOperationCancellation` хранит exact original cancel и OperationSource, CAS начала, отдельные ordinary-return witness и unwind notification. Request сначала отзывает negative source; повторный/reentrant request не ждёт исходный callback и не доказывает его возврат. Join под существующим operationMu ждёт исходный frame; panic/Goexit unwind не становится успехом. Новых goroutine/timer/Context parent/native receipt нет.

Foreground/update/proxy регистрируют record вместе с существующим CancelFunc; занятый или несогласованный слот нельзя перезаписать. EndOperation вызывает cancel/join вне App.mu и очищает обе записи только по exact original pointer после обычного возврата. Unknown/pending/failed оставляет original slot. Context constructors/getters/WithOperationSources пока выполняются под App.mu — это следующий обязательный объём.

Stop legacy/UI/restart сохраняют original cancellation при регистрации и немедленно отзывают original intent source под App.mu. Actual cancel вызывается после Unlock до ожидания operationMu; после serializer выполняется join исходного record до cleanup/direct Release. Запрос не берёт cancellation нового владельца. StopRequest остаётся comparable. Profile сохраняет record, выполняет original join через существующий appCleanupStep вне App.mu до state event/native retirement; unknown/failed ведёт в reconciliation и не допускает профиль. Only outer endOperation освобождает его слот.

Proxy prepare/confirm раньше делали только defer cancel; теперь общий endOperation включает освобождение original slot. Body-return flag сбрасывается на время actual retirement и восстанавливается только после обычного возврата; panic/Goexit cancellation не подтверждает завершение исходного proxy request.

Это proof ordinary return только App-owned callback, не quiescence всех ancestor/raw-epoch cancellation, native actors, sockets или ОС. Никакой hard latency/RSS guarantee; pending callback может остаться pending. Compiler/private fixtures/concurrency/reentrancy/OS/performance acceptance OPEN. Initial V528 snapshots/pre1401 сохранены; после разрешённой Git-доставки продолжение заново привязано к own revision1404/pre1405 при неизменных source baselines.

Source/unverified: 8 Go + 4 docs; вручную просмотрены 7 diff и 1 новый helper, gofmt8 exit0. Тесты/сборки/native/сеть не запускались. Полный PC-00…09/TH-00…08 и Windows-приёмка OPEN.

<a id="pc-v529-operation-context"></a>

## V2-529 — actual foreground/update/proxy factories вне value locks

Новый immutable `appOperationPreparation` сохраняет original App parent/lifetime/registration/path и fixed mode/proxy/handoff/recovery inputs. Existing App operation/cancellation slot резервируется до любого real constructor/getter. OperationMu/updateMu остаются исходными сериализаторами; App/epoch/owner value locks не удерживаются во время WithCancel, Value/WithValue/WithOperationSources и fence capture. Fence metadata не подменяет actual Context parent.

Existing cancellation owner теперь различает constructor attempt, ordinary factory return и unwind; real partial cancel публикуется сразу после WithCancel до следующих getters под коротким private copy mutex. Pending request отзывает original negative source; без child return он не выдумывает cancel/retirement. Join ждёт исходный factory frame, затем actual original cancel. Unknown/abnormal constructor/cancel сохраняет original slot. Отказ до входа в constructor может освободить слот только после известного обычного возврата без созданного child. Failed build использует существующий appCleanupStep; Goexit продолжает unwind исходного caller.

Все четыре пути — foreground, ordinary/recovery update, committed update и local proxy recovery — строят контекст через один компонент после Unlock. Final short App→epoch gate проверяет exact owner/registration/lifetime, fixed mode constraints и captured path/current/down/power вместе с prepared publication fence. Delayed callback не может получить новый intent/path. Foreground требует прежние protection/watchdog/profile/proxy/image условия; update сохраняет отдельный stopped/recovery contract; committed update — exact handoff; proxy — actual local owner.current.

Epoch binding не вызывает constructor/getter/cancel под mutex: он регистрирует notification того же original owner только для captured current path и свободного слота. Events могут убрать notification, но сохраняют owner до обычного возврата его actual cancel; release очищает только exact own pointer. Legacy raw bindOperation оставлен для прежних private fixtures, product callers больше нет; owned App slot он не перезаписывает. Ancestor/stdlib propagation не является доказательством общей quiescence. Стандартный WithCancel сохраняет свои AfterFunc/watcher semantics; нового явного исполнителя, native actor или forwarding authority нет.

Следующий дефект: boolean beginOperation ещё сворачивает detailed preparation failure в false; исходные Start/Switch/probe outcomes должны отличать cleanup uncertainty от обычного stale admission и публиковать конечный результат, сохраняя original registration. Это не помечено закрытым. Compiler/private fixtures/concurrency/reentrancy/resources/native/OS/speed acceptance OPEN.

Source/unverified: 9 Go + 4 docs; вручную просмотрены 8 diff и 1 новый helper, gofmt9 exit0. Тесты/сборки/native/сеть не запускались. Полный PC-00…09/TH-00…08 и Windows-приёмка OPEN.

<a id="pc-v530-preparation-outcome"></a>

## V2-530 — preparation error и original lifecycle/UI outcome

Error-returning beginOperationContext заменяет boolean на всех пяти product-путях: Start, Switch, protected probe, mandatory protection verification и optional native diagnostic. Старый private boolean adapter сохранён для совместимости, product callers у него нет. Ordinary cancellation/stale admission и local cleanup failure больше не сводятся к одному false/общему сообщению.

Original appLifecycleOutcome хранит finite preparation custody failure. Даже обычный возврат тела не разрешает early success; отзыв/negative cleanup/publish выполняются только по исходной регистрации через существующий outcome и оригинальный cleanup cohort. Последующий возврат Stop не стирает независимую uncertainty. Существующие ordinary errors/verified rollback без этого marker не переклассифицируются. Direct Release, новая authority, новый actor или synthetic retirement receipt не добавлены.

Benign rejected Start публикует конечный error только для exact original current starting intent с прежними desired/profile/closing gates. Готовность и старые positive UI projections снимаются; message строится существующим finite presentation helper. Native work, новый retry и retirement receipt не создаются, protection intent не меняется. Stale successor не затрагивается; UI callback защищён исходным outcome.

Ошибка подготовки mandatory protection до начала Observe получает отдельный local fault с исходной registration. После serializer caller через existing outcome проверяет тот же runtime cc/Manager и source, затем выполняет только original negative cleanup. Она не создаёт ProbeReport, MarkRestrictionObserved, фиктивный WFP результат или health evidence. Реальный post-Observe protectionFault/marker сохранён. Optional native diagnostic возвращает finite cleanup class без изменения mandatory health/intent.

Proxy prepare/confirm при custody failure до native callback оставляют original request/Manager unconfirmed: return flag не выставляется, replacement не разрешён. UI использует существующее сообщение о неподтверждённом завершении. Общие Start snapshot/command gates отклоняют retained slot в stopped/error с прежним cleanup_unconfirmed reason. Public schema/bindings не меняются.

Report-scope исходник дополнительно заморожен до любых правок при own pre1411, с совпадением inherited bytes; initial before/reviewed manifests сохранены. Следующий дефект: Stop/restart actual cancelOriginal вызывается до установки finishStopOutcome; callback panic/Goexit требует собственного оригинального outcome до callback, без Release/потери регистрации. Compiler/private fixtures/concurrency/reentrancy/native/OS/performance OPEN.

Source/unverified: 9 Go + 4 docs; вручную просмотрены 9 diff, gofmt9 exit0. Тесты/сборки/native/сеть не запускались. Полный PC-00…09/TH-00…08 и Windows-приёмка OPEN.

<a id="pc-v531-stop-preparation"></a>

## V2-531 — original Stop preparation outcome до actual cancel

Legacy stopCoreFor, UI Stop и beginRestartHandoff используют общий acquireStopRequest. Original direct deferred outcome установлен до actual cancelOriginal и до ожидания operationMu. Source/record остаются исходными: notification до serializer, original join после acquisition, без lookup cancellation нового владельца. Ordinary join custody error остаётся unconfirmed; panic/Goexit не обходят исходную negative publication и не создают handoff/Release receipt. Goexit по-прежнему продолжает unwind исходного caller, не становится Go/RPC возвратом.

Acquired/transferred mutex facts различаются. Только normal nil-result передаёт operationMu caller; acquired failure/unwind освобождает его после failed publication, до acquisition чужой mutex не освобождается. Legacy/UI ставят свой unlock только после успеха; restart сохраняет прежний отдельный transfer в actual handoff token. Новый executor/native owner/context/timer не добавлен.

Existing Stop failure publisher вынесен без изменения body: original source revoke, exact sameLocked gate, finite error/cleanup_unconfirmed и readiness=false, retry cancellation и защищённый event step. Этот publisher не вызывает native cleanup, protection Release и не очищает operation slot. Старый serialized finishStopOutcome и полный native Stop cleanup/Release body сохранены; stale registration не публикует поверх successor.

UI использует существующий finite unconfirmed receipt: ordinary helper error может вернуть реальный outer RPC result, но не подтверждает inner cancel return; actual record сохраняет собственный failed/pending факт. Unknown callback/wait может остаться pending, без hard latency/OS quiescence доказательства. Следующий дефект — normal update/proxy slot в stopped/error сейчас попадает в cleanup_unconfirmed; admission/UI должны различать busy и witnessed failed return по passive owner metadata, без actual Context getters или timeout assumptions.

Source/unverified: 4 Go + 4 docs; вручную просмотрены 4 diff, gofmt4 exit0. Тесты/сборки/native/сеть не запускались. Полный PC-00…09/TH-00…08 и Windows-приёмка OPEN.

<a id="pc-v532-operation-slot"></a>

## V2-532 — busy operation и witnessed failed return

KnownFailure использует только initialized original record pointers, nonblocking observations закрытых prepared/done notification и atomic ordinary-return flags. Оба notification проверяются независимо: отказ cancel не скрывается за ещё pending factory. Open notification/source revoke не объявляются ошибкой, timeout или гарантией будущего завершения. Context getters, originalCancel copy mutex, callbacks, waits и новые источники/owners не используются.

OperationSlotBlockLocked различает empty, invalid tuple/known failed original return и ordinary occupied record. Любой занятый слот всегда запрещает replacement, даже если callback уже вернулся и original end ещё не очистил запись. Возможная гонка наблюдения влияет только на denial reason, не на разрешение. Start command/snapshot и actual context reservation используют тот же gate; initial/final lifetime/mode/intent/path/owner gates сохраняются.

Busy получает отдельную finite typed error, не превращается в cleanup failure. Независимый watchdog fault сохраняет приоритет/причину. Proxy UI возвращает finite busy message отдельно от unconfirmed custody. Existing lifecycle_busy hint обобщён до операции приложения (update/proxy/connect), next step — дождаться её завершения. Новых API/schema/bindings/layout/style/polling изменений нет; одна строка статического TS label заморожена до edits по inherited hash, initial manifests сохранены.

Классификация не очищает/отменяет/отзывает слот и не вызывает native cleanup/Release или retry; она не доказательство VPN health, OS retirement или quiescence. Original constructor/cancel/epoch/Stop logic сохранены. Следующий дефект — startup_unavailable всё ещё форматирует unknown a.startupErr через %w под App.mu; это требует finite safe rejection и private retained cause, не объявлено закрытым.

Source/unverified: 4 Go + 1 static TS label + 4 docs; вручную просмотрены 5 diff, gofmt4 exit0. Тесты/сборки/typecheck/native/сеть не запускались. Полный PC-00…09/TH-00…08 и Windows-приёмка OPEN.

<a id="pc-v533-startup-rejection"></a>

## V2-533 — startup rejection без unknown error callbacks под App.mu

Actual startRejectionLocked раньше делал fmt.Errorf(%w, a.startupErr) под App.mu. Установленный Go fmt source подтверждает вызов Formatter.Format или error.Error при таком форматировании. Теперь public rejection имеет собственный fixed Error text и Unwrap только в static core.ErrEngineCleanupFailed; private original cause не хранится в public projection и не достигается через public Is/As/Unwrap. Sticky first-writer App.startupErr сохраняется для внутреннего контроля.

App startup-failure state и public error используют один fixed message; прежний текст состояния сохранён. Остальные profile-transition/closing admission errors — fixed sentinels, fmt исключён из этого value-lock path. Snapshot reason, порядок отказов, intent/source/readiness/slot/mode gates не меняются. Negative category не является доказательством существования/retirement ресурсов или успешного перезапуска.

Одна существующая TS startup_unavailable hint теперь описывает прерванную подготовку и перезапуск после проверки остановки VPN, не обещает восстановить sticky abnormal startup повторным импортом профиля в том же экземпляре. Public schema/bindings/layout/style/polling не менялись. Actual startup finalizer/native cleanup/source revocation/private first-cause behavior сохранены, кроме переиспользования fixed message.

При чтении следующего пути найден actual alias: ensureWatchdog делает ctx:=a.ctx, ctx.Err и WithCancel(ctx) под runtime_.mu+App.mu, до reservation исходного task. Это не закрыто: нужен original pending owner до callbacks, real constructor/cancel вне value locks и сохранение actual started/done/failed return без replacement. Compiler/private fixtures/callback/Goexit/reentrancy/native/OS/performance OPEN.

Source/unverified: 2 Go + 1 static TS label + 4 docs; вручную просмотрены 3 diff, gofmt2 exit0. Тесты/сборки/typecheck/native/сеть не запускались. Полный PC-00…09/TH-00…08 и Windows-приёмка OPEN.

<a id="pc-v534-watchdog-context"></a>

## V2-534 — original watchdog context ownership вне value locks

ensureWatchdog раньше делал ctx:=a.ctx, ctx.Err и context.WithCancel(ctx) под runtime_.mu+App.mu до reservation task. Теперь под коротким runtime→App lock захватываются исходный parent/lifetime fence и резервируется единственный original watchdog с cancellation record, started/done. Actual constructor/getters/cancel выполняются после unlock. Final runtime→App→task gate проверяет именно исходного владельца, App/child fences, source и отрицательные lifecycle flags; metadata не становится родителем Context. Только успешный original creator запускает существующий run; contextReady не означает task entry.

Partial actual cancel публикуется до следующих getters. Preparation unwind/ordinary return и cancel unwind/ordinary return — разные факты. Если отмена пришла до получения partial cancel, общий storeCancel выполняет сохранённый запрос сразу после публикации, вне copy mutex; CAS сохраняет единственного владельца actual callback. App/watchdog после публикации проверяют отрицательные источники до следующих parent getters. Duplicate request, timeout или close(done) без обычного возврата не разрешают replacement/cleanup success.

Creator failure сохраняет original slot, ставит negative fault/blocked до cleanup и закрывает original failed-frame done только после registered stages. Оно не вызывает synchronously fault cleanup через уже удерживаемый creator operationMu; это failed preparation, не running-loop receipt. Guard/loop используют исходные immutable fences/source, без Context.Err под value locks; реальный цикл получает построенный child Context.

Run finalizer заранее регистрирует context-retirement и original fault-propagation stages через существующий appCleanupStep. Panic/Goexit actual callback не пропускает следующие deferred stages; blocked ставится до ожиданий, fault merge выполняется до original done. joinWatchdog дополнительно требует contextReady и ordinary-return facts preparation/cancel. watchdog_fault/terminal/native Stop bodies и status consumers сохранены; новая Engine/OS/health/forwarding authority не создаётся.

Исходный pre1423 и все original before/initial review/late promotion receipts сохранены через разрешённую Git-публикацию1424–1426; продолжение использует новый собственный CAS baseline. Gofmt3+3 относится к исходным четырём неизменившимся source files, не к повторному запуску format/test. No compiler/runtime proof.

Следующий подтверждённый source-дефект: startNetWatch/recoverServiceDNSObserver вызывают newNetworkPollerOwner и newServiceDNSHandoff под netWatch.mu до pending owner reservation; guards/recovery также вызывают Context.Err под value locks. Нужен original pending-owner протокол без обхода actual producer/native-power retirement, с bounded confirmed predecessor и recovery budget. Compiler/private fixtures/custom-parent callbacks/Goexit/reentrancy/native/OS/performance OPEN.

Source/unverified: 4 Go + 4 docs, один новый helper; вручную просмотрены 3 diff и новый файл, gofmt3+3 exit0. Тесты/сборки/native/сеть не запускались. Полный PC-00…09/TH-00…08 и Windows-приёмка OPEN.

<a id="pc-v535-network-context"></a>

## V2-535 — original network/DNS owners до real context callbacks

Initial startNetWatch и reconcileServiceDNSAfterRetirement раньше создавали actual WithCancel под netWatch.mu до публикации исходного владельца. Теперь newNetworkPollerOwner/newServiceDNSHandoff выделяют только passive task/cancellation/source/channels. Actual App parent/fence захватываются под App.mu до registry lock; исходная пара резервируется, затем real children, partial cancel и child fences готовятся вне value locks. Caller interest остаётся negative fence, не родителем. Final App→watcher→poller→DNS lease сверяет именно исходную пару, parent/child/interest/source и lifecycle denial flags до запуска существующих циклов. ready означает публикацию child context; started по-прежнему относится только к actual loop entry.

DNS task source и cancellation source — один исходный negative owner: dependency revocation также останавливает дальнейшую подготовку и сохраняет ранний запрос отмены. Переиспользуется appOperationCancellation: исходный partial child и factory/cancel unwind отделены от ordinary return, ранний запрос отмены не теряется. Ошибка до запуска удерживает original owners и закрывает original failed-setup done только после registered cancellation stages и receipt publication. preparationOnly/preparationReturned явно отличаются от bodyReturned; незапущенный loop не получает фиктивный return/native receipt. Panic/Goexit не пропускает последующие зарегистрированные stages, blocked callback сохраняет pending original owner. Creator не вызывает running fault cleanup с повторным захватом уже удерживаемого operationMu. DNS-only setup failure не отменяет исходный живой poller.

Actual running finalizers используют original cancellation join вместо повторного raw cancel как подтверждения завершения. Guard/terminal classification/event capture/current/submit/loop-entry/epoch/power/native registration gates читают исходные fences/source под value locks; real DNS loop/I/O по-прежнему получают actual child Context. Disconnected power recovery готовит caller fence заранее, а errors.Is/localRetirementFailure выполняет после копирования исходных fault values и unlock.

Actual prior runtime identity, original done/cleanupKnown/native power-handle retirement gates, callback joining/unpin, один bounded confirmed predecessor, тот же helper retirement и observer-only recovery budget сохранены. 17 original regions/file bodies сравнились без изменений. Исходники не доказывают native/OS path, quiescence всех stdlib ancestors, hard latency, RSS или успешный VPN. Private fixtures, создававшие context-ready task непосредственно constructor, требуют будущей миграции; тестовые файлы не менялись.

Следующий подтверждённый source-дефект: background_probe_owner.go создаёт real WithCancel до original task reservation, делает lifetime.Err под bgReg.mu и ctx.Err под task mutex; attempt admission имеет такой же callback под owner.mu, finalizer использует raw duplicate cancel return. Следующая правка должна сохранить original serial diagnostic attempt, actual connection close/release и bounded existing protocol. Compiler/private fixtures/Goexit/reentrancy/native/OS/performance OPEN.

Source/unverified: 11 Go + 4 docs, один новый helper; вручную просмотрены 10 diff и новый файл, gofmt11+1 exit0. Тесты/сборки/native/сеть не запускались. Полный PC-00…09/TH-00…08 и Windows-приёмка OPEN.

<a id="pc-v536-background-context"></a>

## V2-536 — background task до actual context и отмена вне Once

startBgProbes раньше строил actual WithCancel до reservation task, проверял lifetime.Err под bgReg.mu; available/finish/attempt admission вызывали ctx.Err под task.mu. Теперь исходный passive background owner резервируется первым. Actual child готовится вне App/bgReg/task locks через существующий механизм владения; final App→registry→task gate проверяет того же владельца/stop channel, исходные lifetime/child/source и lifecycle denial flags до запуска единственного прежнего run. Prepared/started registry values не считаются loop/native/diagnostic entry.

Прежний network context factory извлечён в app_task_context.go, producer frame receipt — в app_producer_retirement.go; существующий networkProducerReceipt оставлен alias. Factories/negative fences/partial cancellation/ordinary-return факты, network pair и actual native/power/cleanup/joins сохранены после нормализации имён. Компоненты используются poller, DNS и background task; новый scheduler, engine, SDK или реестр отмены не создаётся.

Background run finalizer ждёт original cancellation.join и публикует body/cleanup return до done. Неудачная original setup имеет preparationOnly/preparationReturned, а не фиктивный bodyReturned; original done закрывается после partial child cleanup и receipt publication. Known cleaned unstarted setup можно подтвердить отдельно от running loop; unknown constructor/cancel и retained attempt не дают successful retirement/replacement. Blocked callback остаётся pending, без rescue goroutine или обещания hard latency.

cancelBackgroundProbes выставляет stopping и закрывает passive stop через Once, затем вызывает actual cancellation вне Once/value locks. Установленный sync.Once source показывает вызов callback под внутренним mutex: прежний actual cancel там мог повторно войти в ту же отмену. Теперь duplicate notification не ждёт original callback и не подменяет его completion; actual finalizer join остаётся отдельным.

Round сначала копирует original registration/source-state/activeID/path/epoch/control под короткими App→epoch locks. Value/WithValue/child fence готовятся вне locks; old/unknown binding не подменяется новым путём, после getters сверяется тот же subject. Первоначально revoked intent остаётся тем же optional scope через source-state equality; это не разрешение Engine/protected path. Actual serial attempt, diagnostic admission, Dial, Close, watch, publication и release сохранены, кроме одного passive owner gate. Бюджеты/стартовая пауза/последовательный loop/weak-health semantics не менялись.

Обычная ошибка optional background setup даёт fixed предупреждение и не добавляет mandatory VPN readiness gate. Actual panic/Goexit всего исходного startup frame по-прежнему обрабатывается его original outcome; произвольный зависший callback не считается завершённым. 24 original regions/extracted bodies сравнились без изменений. Compiler/private fixture migration/reentrancy/Goexit/native/OS/performance OPEN.

Следующий подтверждённый source-дефект: prepareManualDiagnostic в secret_operation.go вызывает real scopeContext/WithTimeout/AdmitDiagnostic/getters/cancel/release под App.mu до original task reservation; currentError и historical diagnosticScope.currentLocked обращаются к actual ctx/parent под value locks. ownedDialer строит WithCancel под task.mu до dial reservation. Требуется original task/dial-before-callback protocol с сохранением actual budget/socket/watch/Close/release, credential/source/path gates и max32 bound. GUI diagnostic code только прочитан; никаких вызовов или чтения значений vault/config/log не выполнялось.

Source/unverified: 12 Go + 4 docs, три новых helper; вручную просмотрены 9 diff и 3 новых файла, gofmt12 exit0. Тесты/сборки/native/сеть не запускались. Полный PC-00…09/TH-00…08 и Windows-приёмка OPEN.

<a id="pc-v537-manual-context"></a>

## V2-537 — GUI diagnostic task/Dial до context и budget callbacks

prepareManualDiagnostic раньше выполнял scopeContext/Value, WithTimeout, AdmitDiagnostic, cleanup cancel/release и final getters под App.mu до reservation. Теперь сначала под App.mu сохраняются exact original task, App parent/lifetime, registration/path/context view и только identity vault pointer. Actual parent/path wrapper, bounded child и admission создаются вне App/task locks; real partial cancel и возвращённый budget записываются до следующих getters. Final App gate сверяет того же task/vault/request/path/view, original fences/deadline и публикует actual ctx plus immutable metadata через atomic readiness. Новый watcher/executor/SDK не добавлен; запускается только прежний original watcher после publication.

Общий appTaskContext поддерживает реальный bounded timeout и прежнюю cancel-only ветку; nonpositive bounded input не превращается в unbounded fallback. До публикации task/dial захватываются максимум два original negative источника request/parent-operation, которые проверяются до последующих getters. Это не заменитель Context parent и не positive authority. Initial accepted review и late source/format receipts сохранены.

Admission entry и ordinary return различаются. Failed original setup удерживает partial child/lease и допускает очистку только exact own slot после обычного возврата setup, actual cancel и original release. При неизвестном setup/admission/cancel cleanup unused budget не освобождается; task/done остаются pending. Known cleaned unstarted setup имеет preparationOnly receipt, не fake RPC/body/watch return, и не публикует наблюдение.

currentError читает sealed original fences/source после atomic ready, сохраняя cleanup/dial-limit/closing/request/path denial. Historical cache и final result/adaptive history используют original parent fence и фактически захваченный deadline вне mutex; child cancellation не обновляет subject и не делает старое наблюдение новым. Public Wails/schema, credential/source/path/current-view gates и distinction между observation и protected-path proof сохранены.

ownedDialer резервирует каждый original bounded slot до Err/Value/Deadline/WithCancel и actual native/library callback. Исходный run владеет preparation, callback, partial/late socket и cancel join. Не начатый callback получает preparation-only receipt без фиктивного returned; actual Conn capture/late rejection/Close/error shape и max32 bound сохранены. Pending child замечается sealing/cancellation, новый слот не выделяется вместо неизвестного.

Task/dial cancel, watch join и socket/Close stages регистрируются до callbacks; panic/Goexit не пропускает последующие зарегистрированные stages. Original setup/RPC освобождает budget единожды вне locks только после known retirement; interrupted/reentrant release не переигрывается. Blocked callback остаётся owned/pending, без rescue goroutine и hard latency/ancestor-quiescence/RSS promise. 21 original regions сравнились без изменений, включая native Conn Close, raw callback result handling, GUI protocol code и original completion/credential fences.

Далее — core AdmitDiagnostic release: сейчас sync.Once включает actual cancel и lane.active decrement, без отдельного original ordinary-return witness для duplicate/reentrant callers. Core pool в этом срезе не менялся: protected lane, optional active≤2, rolling requests/bytes и отсутствие refund при ordinary error/cancel сохранены. Compiler/private fixture migration/reentrancy/Goexit/native/OS/performance OPEN. Никакая GUI-диагностика не выполнялась; значения vault/runtime/config/log не читались.

Source/unverified: 8 Go + 4 docs, два новых helper; вручную просмотрены 6 diff и 2 новых файла, gofmt8+3 exit0. GUI-диагностика, тесты/сборки/native/сеть не запускались. Полный PC-00…09/TH-00…08 и Windows-приёмка OPEN.
