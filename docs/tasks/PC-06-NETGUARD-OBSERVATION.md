# PC-06 — локальные наблюдения NETGUARD и владение ресурсами

<a id="pc-v360-com-observer"></a>

## V2-360 — Task Scheduler COM collector

2026-11-10, Windows owner, **source/unverified**. Предыдущий V359 — progress:
изменены source/docs/checkpoint; [PC-01 audit](PC-01-SOURCE-AUDIT.md) и bounded
log/UI сохранены. Полная цель — PC-00…09 и прежние FR/TH/OFF/COM/RQ/TR/TH-00…08.
Ни один общий gate этой сдачей не закрывается;100 acceptance OPEN/56 NOT_RUN.

### Какие дефекты были в исходниках

В прежнем comGetTask результат CoInitializeEx игнорировался; CoUninitialize
всегда вызывался, даже после failed init. Goroutine не закреплялась на OS thread,
хотя apartment/interfaces и cleanup привязаны к исходному потоку. Для каждой из
трёх задач отдельно создавался Schedule.Service и выполнялся Connect/GetFolder.
Результат Connect терялся; ошибка property VARIANT.Clear не проверялась.
Generic Value/formatting принимали неожиданные типы и могли превращать ошибку
даты в число. Ошибка без HRESULT отображалась как invented unknown:0x00000000.

Источники сверки: закреплённый go-ole v1.3.0 в go.mod/go.sum, его com.go,
variant.go, oleutil/oleutil.go, IUnknown/IDispatch Windows sources и
variant_date_amd64.go. Только чтение source: cache/версии/зависимости не менялись,
COM/SDK вызовы не выполнялись. Existing original-apartment pattern из
backend/core/elevated_gui_windows.go переиспользован по принципу владения;
launch/elevation code не менялся и не выполнялся.

### Текущий source-контракт

1. **Один исходный collector.** Production netguardAllTasks получает одну
   синхронную COM operation. TryLock исключает параллельную работу этого домена;
   незавершённый исходный owner не заменяется новым observer. Не более3 имён,
   каждое1…128 bytes без NUL, проверяются до копирования/выделения rows.
2. **Исходный OS thread.** Owner сохраняется до LockOSThread/CoInitializeEx.
   S_OK и S_FALSE дают ровно один initialization, который балансируется после
   освобождения собственных references. В go-ole1.3.0 S_FALSE представлен
   OleError(1). Остальная init error прекращает работу; чужой apartment не
   uninitialize. Никаких retry с другим COM mode.
3. **Явная graph ресурсов.** Original CreateObject/QI outputs записываются
   в service/dispatch; результаты Connect/GetFolder/GetTask/GetProperty — в
   original VARIANT slots. До нового вызова прежний slot должен быть свободен.
   В группе используется один service/root folder, а не три отдельных setup.
4. **Заимствование без двойного Release.** ToIDispatch не AddRef. Borrowed
   folder/task освобождаются только через Clear владеющего VARIANT. У service
   и отдельного QI reference — по одному original Release. Результат Connect
   очищается до GetFolder. Обычный GetTask error также не теряет возвращённый
   VARIANT, если он есть.
5. **Каждый Clear имеет исход.** Перед вызовом фиксируется попытка; только nil
   error от настоящего возврата разрешает обнулить/reuse slot. Failed или
   interrupted Clear сохраняет original graph и запрещает повтор/замену.
   Cleanup идёт property→task→folder→reply→QI→service→own Uninitialize→unlock.
   После неизвестного native phase родительские releases не предполагаются
   безопасными. Known failure без неизвестного ресурса может завершить owner.
6. **Конечные данные.** State/LastTaskResult допускают только VT_I4/VT_UI4;
   State проверяется до int32 conversion. LastRunTime/NextRunTime требуют
   конечный VT_DATE и успешный actual converter; explicit DATE0 даёт пустую
   дату. Ошибка конвертера не принимает его time.Now fallback. Arbitrary
   BSTR/array/object Value/string conversion не выполняется. Это не новая
   проверенная политика timezone/locale или доказательство истории задач.
7. **Публикация после cleanup.** Обычные per-task GetTask HRESULT сохраняют
   not-found/no-access только для фактической ошибки GetTask. Setup/property/
   lifecycle failure не получает такой смысл. Unknown без HRESULT остаётся
   unknown, без synthetic code0. При failed final retirement весь пакет из
   трёх строк становится unknown; готовые частичные rows не выдают successful
   cleanup. Source seam comGetTask сохранён на том же original owner.

### Что означает ограничение

Ограничены число owned collector/graph, имена и число собственных property
operations. Нет новых goroutine/timeout replacement/repair. Если оригинальный
COM вызов не вернётся, этот observer не создаст следующий collector; из другого
вызова возвращается unknown. Retained reference не доказывает, что OS thread
или native object физически живы либо что native release произошёл.

Source не ограничивает время native RPC, внутренние COM/server threads,
native VARIANT/BSTR allocation до проверки типа, RSS/GC, process-wide observers
или межпроцессную работу. Memory cap, ускорение3×, native leak freedom и
жёсткий deadline не заявлены. Переиспользование service уменьшает source setup
calls для одной группы; количественного замера скорости нет.

Чтение задач и код0 не подтверждают protection, исправность VPN/DNS, успешное
восстановление или право писать proxy/DNS/WFP/TUN. Root scripts/tasks и
installed copies не менялись. Durable W3b/NETGUARD migration всё ещё отсутствует
и требует согласованного owner scope; [NETGUARD-001](CHANGE-REQUEST-NETGUARD-001.md)
не является разрешением или реализацией recovery.

### Состояние работы

2 Go source и4 документа,6 snapshots(4existing/2new), manual original1 diff
и новый файл. Только touched gofmt:2 файла, затем1 после уточнения комментариев,
оба exit0.12 reviewed source и8 cached dependency inputs сверены по SHA256.
Тесты/fixtures/build/vet/types/parser/generation/browser/COM/native/admin/network
и реальные пользовательские env/log/vault не добавлялись/не исполнялись.
Source/gofmt не доказывают компиляцию, threading/VARIANT/COM ABI, native resource
retirement, календарную семантику, OS recovery, whole UX или готовность выпуска.
W4c compiled gatefalse, TUN unavailable/schema2 unsupported сохраняются.

### Следующий source-разрыв

В core.LoopbackAlive остаётся DialTimeout с ignored conn.Close, без fixed
original admission/retirement и фактического App lifetime. Сначала найти
существующих original socket/probe owners и все consumers, затем выделить
отдельный срез: timeout не освобождает owner; неизвестный dial/Close не даёт
replacement. Port reachability не превращать в proxy ownership/VPN proof.
Полный PC-00…09/старые девять этапов остаются active, без waiver.

<a id="pc-v361-loopback-owner"></a>

## V2-361 — исходный loopback socket и явные исходы UI

2026-11-10, **source/unverified**, PC-06/04. V360 — progress: COM collector
source/docs/checkpoint сохранены. Полный PC-00…09 и прежние девять этапов active;
100 acceptance OPEN/56 checks NOT_RUN, без waiver.

### Дефект и переиспользование

core.LoopbackAlive использовал raw net.DialTimeout и игнорировал conn.Close.
NetGuardStatus подключался к1080 повторно через proxy read и SOCKS read. Bool
false объединял отказ, timeout и неизвестность. Помимо NETGUARD, реальный
потребитель этих booleans найден в NetworkView; правка одного экрана оставляла
там ложный «ответ не получен» при неизвестном состоянии.

Новый Windows-only loopback_observation.go переиспользует preflightNetwork/
preflightConn: original socket резервируется до dial, Close outcome sticky.
Общий lifecycleCleanupStep регистрирует original cancel и close независимо;
аномальное завершение cancellation не пропускает cleanup известных sockets.
ManualDiagnosticTask уже существует, но предназначен для явных команд и требует
vault/intent/контекста диагностики/своего watcher. Автоматическое read-only
loopback-наблюдение не создаёт для этого вторую manual session.

### Контракт исходной операции

- Один синхронный process-local observer/lane. Не более2 input ports1…65535,
  duplicate ports объединяются до native acquisition. Original owner записан
  до создания child timer/dial. Fixed counts/budget — resource policy, не mock
  измерений. Новый goroutine/watcher/reaper/cache не добавлен.
- Один requested300ms deadline на всю группу, ограниченный исходным parent;
  tcp4 и literal127.0.0.1 не добавляют hostname lookup/IPv6/Happ route checks.
  Фактическое время возврата/OS internals этим не ограничены жёстко.
- Actual preflight dial/deadline frame должен вернуться. Unknown acquisition
  сохраняет graph; он не объявляется failed-no-resource и не закрывается
  предположительно. Сигнал cancel не равен retirement. Known returned sockets
  закрывает тот же original network owner; actual Close error не игнорируется.
- Положительный reachable выходит только после known original cleanup и
  отрицательной проверки исходного parent/deadline. Failed/unknown cleanup
  даёт unavailable и удерживает original owner; следующий observer не заменяет
  его новым. Сохранён обычный finite refused/timeout/canceled/busy/unavailable.
- Для Windows refusal используется existing x/sys0.44 WSAECONNREFUSED10061.
  Generic syscall.ECONNREFUSED в Windows — другой APPLICATION_ERROR; текст
  ошибки и guessed numeric port absence не участвуют в классификации.

### App, совместимость и UI

NetGuardStatus захватывает реальный a.ctx до COM/registry/log ожиданий, не
перепривязывает поздний результат к новому lifetime и не использует Background
как App fallback. Registry decoder больше не запускает отдельный dial. Один
batch обслуживает обе строки, а final App fence заменяет stale known outcomes
на canceled, сохраняя busy/unavailable как исходную неизвестность. App.mu не
держится во время I/O. Это negative display fence, не native/traffic authority.

Legacy standalone LoopbackAlive(port) сохранён для совместимости на том же
bounded domain и возвращает true только для accepted reachable. Его explicit
Background scope не используется App; source caller search не нашёл другого
production App consumer этого legacy bool seam. False не доказывает отсутствие
listener. HealStaleSystemProxy всё ещё unavailable; recovery не добавлено.

Optional schema1 DTO содержит только bounded port/outcome values. Strict mapper
отклоняет malformed/duplicate/unsupported facts; legacy backend остаётся unknown.
Общий fmtLoopbackObservation в existing labels.ts используется в NETGUARD и
NetworkView. Оба экрана не трактуют legacy false как measured failure; отказ,
timeout, cancel, busy и unavailable показаны отдельно. Existing rows/tokens/
semantic controls сохранены, новых карточек и live-region spam нет. Applied
modern-web-guidance: существующий cached accessibility guide, passive text для
полученных наблюдений; это не visual/browser/a11y acceptance.

### Оставшиеся границы

Речь только о локальном IPv4 TCP connect. Не доказаны proxy/SOCKS protocol,
process owner, VPN path/egress, IPv6/DNS/UDP, physical Close, отсутствие native
leak или фактическая свежесть в момент IPC доставки. Coherent registry/port
snapshot и measurement timestamps не заявлены. Стандартный dialer/OS могут
иметь внутренние задачи/буферы; RSS, wholeAPI/rate/interprocess, native latency
и crash recovery не получают hard bound из этого source. Legacy booleans
сохранены, но два actual UI consumers используют новый observation contract.

Root tasks/scripts, DNS/proxy/WFP/TUN settings, generated bindings, Android/
shared/configs/harness/другие чаты и реальные env/log/vault/server не менялись
и не запускались. Durable W3b/external NETGUARD migration остаётся отсутствующей
owner-dependent реализацией. W4c gatefalse/TUN unavailable/schema2 unsupported.

### Результат и следующий шаг

3Go3TS2Vue4docs. Initial10 snapshots сохранены; NetworkView/labels.ts были
добавлены двумя отдельными original snapshots и expanded checkpoint до edits.
Всего12 originals(11existing/1new); manual7 original source diffs+new file,
17 named inputs unchanged и один cached WSA source pin, сделанный при review,
а не до первых source edits. Touched gofmt batches3+1+1 exit0. Никаких tests/
fixtures/build/vet/types/parser/SDK calls/generation/browser/native/admin/network
или реальных log/env/vault reads. Compile/runtime/native/wholeUX не проверены.

Следующий actual source-срез PC-04/06: usePolling уже имеет pauseWhenHidden, но
NETGUARD/Network passive callers его не запрашивают. Сначала caller/manual-flow
audit, затем existing visible-interest API; не приостанавливать engine/protection
и не выдавать UI stop/visibility/timeout за original backend retirement.

<a id="pc-v363-adapter-facts"></a>

## V2-363 — счётчики адаптера отдельно от HTTP-фактов

2026-11-10, **source/unverified**, PC-03/06. V362 — progress, visible/source
interest сохранён. FullPC00…09/прежние девять этапов active,100 OPEN/56 NOT_RUN.

### Реальные разрывы

- GetNetStats вызывал snapshotFacts→collectFacts: счётчики адаптера запускали
  также external egress HTTP, SSID и общий cache wait. PowerShell-парсер
  оставлял отсутствующие/невалидные counters нулями с nil error.
- snapshotFacts регистрировал deferred Unlock, отпускал mutex и вызывал
  collectFacts. Abnormal unwind мог разбалансировать владение mutex. Pending
  collection возвращала старое/пустое значение без отдельного unavailable.
- Единственный health reader ISP брал кеш без epoch/path/age проверки.
  ISP здесь — organization из HTTPS-ответа, не идентификация физического ISP.

### Локальный native producer

GetNetStats теперь вызывает отдельный observeAdapterFacts с actual App parent.
Один original process-local query owner публикуется до context/native calls;
requested700ms budget не разрешает replacement при unknown query/cancel.
GetNetStats не вызывает HTTP, shell/helper, SSID или egress cache. Normal native
query error даёт unavailable, без guessed counters0. Numeric0 из принятого
MIB row остаётся реальным сообщённым counter; UI не называет0 link speed
подтверждённой пропускной способностью.

Переиспользован existing localRouteSnapshot through private row-capture seam:
fp/class/counters берутся из одного validated MibIfRow2. Old wrapper по-прежнему
исключает PPP/tunnel/свой TUN для netwatch; explicit read-only stats purpose
может читать virtual interface. Никаких OS route/DNS/WFP/TUN writes. Primary
x/sys0.44 types прочитаны из cache, модули/версии не менялись.

Route scope — локальный выбор Windows к9.9.9.9 или2620:fe::fe. Это не
доказательство единственного default0/0 маршрута, physical interface или пути
всего трафика. UI теперь показывает именно эту привязку, а не выдаёт её за
универсальный default. API read не делает HTTP-пробу. Нет atomic whole-OS,
physical-native-retirement, interprocess, RSS/latency или performance proof.

### Identity, история и совместимость

Optional sourceSchema1/native_route_adapter/LUID:index/routeTarget дополняют
старые поля. Frontend строг к native schema/identity/target; malformed/unsupported
native data не downgrade в legacy. Отсутствующий schema у старого backend
остаётся legacy с неизвестной identity, без заявления native continuity.
History reset учитывает identity в дополнение к alias/counter/time rollback.
От одинакового имени другого адаптера не продолжается Native окно. Intervals
по-прежнему renderer receipt times, не измерение wire/VPN throughput.

NetworkFacts LocalIP допускает IPv4/IPv6 выбранного маршрута и передаёт route
target. Native LUID/source fields не являются WFP/session/forwarding authority.
Generated Wails bindings не патчились/не генерировались; приёмка API OPEN.

### SSID и процессы

Убран hiddenExec/PATH и PowerShell из этого producer. SSID report сохраняется
через fixed SystemDirectory/netsh.exe, existing optional helper lane и original
Job/image/output retirement; protected helper lane не расходуется. Requested
SSID context2s, output16KiB. Новый helper wrapper только в Windows-owned core,
не Wails arbitrary-command endpoint. Alias не вставляется в команду.

SSID report сопоставляется с Name/Имя выбранного alias; отсутствие match даёт
unknown вместо чужой сети. Legacy UTF-8/UTF-16LE/cp1251 decode/locale и raw SSID
семантика не получили runtime/native WLAN proof. Unknown locale/encoding,
multiple interfaces, reparse/image/helper timeout/native cleanup — OPEN.

### Cache/publication и cohort label

Cache reservation/publication используют короткие собственные mutex frames;
native/SSID/HTTP выполняются вне cache/App locks. Actual App/path pointer/mode/
epoch захватываются до collection, stale path не заменяется поздним. Busy даёт
error, без nil-success с пустым cache. Interrupted original collection остаётся
retained; public abnormal handler не превращается в nil success. TTL и
CheckedAt идут от начала collection, не обновляют возраст старого измерения
после медленного возврата. Cache ограничен одной записью, без нового store.

cachedFactsISP проверяет epoch/TTL/path/canceled parent/closing/error/active
collection без App lock под health/cache/epoch locks. Это historical organization
label, не physical ISP/trust/health verdict. Неподтверждённая метка остаётся пустой.

### Обязательный остаток и следующий срез

egressFacts/tunnelEgressFacts **по-прежнему** используют Background, старые
HTTP workers, unchecked Body.Close и transport lifetime. Source cache frame
не является resource owner/retirement receipt. Эта сдача не закрывает whole
facts/resource bound, PC-03 или PC-06. Нужен отдельный original HTTP/socket/
response/transport worker domain с actual App/path/route/intent, admission
budget, overflow/redirect/DNS policy и наблюдаемым cleanup. До этого нельзя
заявлять leak freedom, реальную freshness/независимость путей или DNS proof.

8Go3TS2Vue4docs,17 originals(16existing/1new), manual12original source diffs+
new file;13 named и1 cached dependency input unchanged. Touched gofmt
batches8+3+1+1 exit0. Tests/fixtures/build/vet/types/parser/SDK calls/generation/
browser/native/admin/network/real env/log/vault не исполнялись/не менялись.
Native ABI/Windows/SSID/route/cache concurrency/GUI/runtime/recovery/release
не проверены. CSS/cards/layout неизменны. W4c gatefalse/TUN unavailable/
schema2 unsupported/100OPEN56NOT_RUN/fullPC00…09/old9 сохраняются.

<a id="pc-v364-egress-owner"></a>

## V2-364 — владение необязательными HTTP-фактами

2026-11-10, **source/unverified**, PC-03/06. V363 завершён как source-срез;
процентный status-only ответ — no progress, затем реализация возобновлена.
Полная цель PC-00…09 и прежние девять этапов сохранены.

### Что изменено

В collectFacts удалены две unowned parallel goroutines, Background requests,
local http.Transport и unchecked Body.Close. Новый Windows-core producer
ObserveEgressHTTPS принимает только два existing fixed targets и compiled
loopback mixed address. Wails arbitrary URL/command API не добавлен.

- Один process-local original owner резервируется до context/budget/dial.
  Existing AdmitDiagnostic/DiagnosticBackground ограничивает optional work,
  rolling reservations не возвращаются. Protected lane не расходуется.
- Два запроса последовательны. Existing preflightNetwork/preflightConn/body
  дают reserve-before-dial, original sticky socket Close и один body Close.
  Переиспользован pinned sing0.8.13 ClientHandshake5 на уже owned socket;
  x/net proxy context workers и новый SOCKS codec не добавлены.
- Standard synchronous TLS/HTTP1 codecs используются без Transport, pool,
  redirect, retry или decompression. SNI/обычная certificate verification
  остаются. Requested batch6s/request3s, header16KiB, TLS-plaintext32KiB,
  body4096/64 плюс overflow byte, HTTP200/identity encoding only.
- Один cancellation watcher зарегистрирован и joinable. Original intent/
  control/parent cancellation, stop request и periodic negative fence закрывают
  известные sockets. Actual caller frame, watcher/timer, socket/body/cancel и
  budget release должны нормально вернуться. Unknown original удерживается;
  timeout/TTL не разрешает replacement или refund активного слота.
- http.ReadResponse decoder сам не владеет отдельным OS socket. Его обычный
  Close error отвергает response после подтверждённого original socket Close,
  но не отравляет optional lane навсегда. Abnormal decoder Close, failed native
  Close или неизвестный acquisition сохраняют original owner. Повторное
  net.ErrClosed не подменяет исходный native receipt.

App capture сохраняет original parent/path/mode/intent registration/control
source и, для SOCKS, существующий appliedProbeSubject с Manager owner/generation.
Новая сессия не усыновляет старый cache/ответ. Starting/switching/stopping и
blocked/profile/update transitions не начинают collection. Никаких новых
operation/session/health/protection grants. OS control отключён при TUN/strict,
его отказ не запускает direct fallback. SOCKS request без исходной current
applied session не отправляется; API показывает неизвестный результат.

Stop через existing cleanup sequence вызывает RetireEgressHTTPS с existing
terminalProducerTimeout до Manager/helper/protection handoff. Он сигналит и
join-ит именно захваченный original task. Timeout/unknown остаётся ошибкой
cleanup и не разрешает WFP Release. Новый successor не входит до original
final metadata/done frame. Cleanup Background context не используется для I/O.

Endpoint IP разбирается через netip, zone/private/loopback/non-global отвергаются.
JSON country/org ограничены и не принимают control characters; origin — actual
ipapi JSON URL, вымышленный /ip fallback удалён. DNSViaTunnel сохранён только
для совместимости как legacy SOCKS HTTPS outcome; DNS/TUN им не доказаны.
GetNetStats, SSID optional helper и frontend V363 не менялись.

### Границы доказательства и остаток

OS-control использует tcp4/net.Dialer и системный resolver. Его внутренние
native DNS/Win32/Go tasks не получают full retirement proof из возврата Dial;
нужен отдельный original-resolution owner или процессный boundary. Certificate
API internals, kernel/wire/RSS/absolute latency, resource hard bound, endpoint
semantics и Windows behaviour не проверены. Запрошенные deadlines — policy,
не измеренный SLO. TLS plaintext cap не является лимитом wire bytes или RSS.
SOCKS handshake/HTTPS не удостоверяют независимость маршрутов, local process
identity, реальный DNS или protected TUN. W4c gate по-прежнему false.

Следующее: source ownership OS-resolution, затем typed per-route outcome/age
без stale/unknown→false и возврат к actual W4c/W3b implementation gaps PC-01.
Durable W3b, native TUN и schema2 installer не объявлены реализованными.

4Go4docs/8 originals(6existing/2new); initial scope расширен перед изменением
stop_cleanup.go, original snapshot сохранён. Manual2original diffs+2new,
17named/3cached unchanged, touched gofmt3+2+3+2+1+1 exit0. Tests/fixtures/build/
vet/types/parser/SDK execution/generation/browser/native/admin/network/real
env/log/vault/other roles/chats/subagents не выполнялись/не менялись.
100 OPEN/56 NOT_RUN/fullPC00…09/old9/native/SDK/OS/wholeUX/W3b/release OPEN.

<a id="pc-v365-os-dns"></a>

## V2-365 — исходный владелец системного разрешения адреса

2026-11-10, **source/unverified**, PC-03/06. V364 — progress;
полная цель PC-00…09 и прежние девять этапов сохраняются.

### Основание и переиспользование

В cached Go src/net/lookup_windows.go lookupIP запускает GetAddrInfoW в отдельной
горутине и при ctx.Done разрешает native worker завершиться позднее. Поэтому
возврат net.Dialer с canceled не доказывал возврат original OS resolution.
Existing helper/PreflightResolver были рассмотрены: новый system helper не
нужен, а fixed bootstrap resolver изменил бы смысл OS-control observation.

Использованы существующие x/sys0.44 wrappers GetAddrInfoW/FreeAddrInfoW и
WSAStartup/WSACleanup, структуры AddrinfoW/RawSockaddrInet4 и existing
publicProbeIPv4 gate. Versions/dependencies, native helper roles и SDK не менялись.

### Реализация

- Platform-local egressAddressQuery регистрируется в original HTTP owner до
  native acquisition. Повторный fetch не может заменить original query.
- Capabilities проверяются Find. Синхронный GetAddrInfoW получает fixed
  ipapi.co/443, AF_INET/SOCK_STREAM/IPPROTO_TCP. Отдельная Go goroutine или
  net.Resolver не запускаются; при timeout caller остаётся владельцем native
  frame до фактического возврата. Stale/canceled ответ не начинает TCP.
- Удерживаются original result pointer и ровно одна successful Winsock
  reference. Failed startup не балансируется чужим WSACleanup. FreeAddrInfoW
  и WSACleanup идут после known socket/body close в existing cleanup sequence.
  Void Free принимается только после обычного возврата, pointer не теряется
  заранее; failed/unknown cleanup не повторяется и не даёт replacement/refund.
- Native list scan ограничен32 records, output8 unique IPv4. Cycles, unexpected
  family/type/protocol/length/null/port и non-public address отвергаются.
  Existing PublicIPv4 gate — не полный IANA special-purpose registry.
- Dial boundary независимо проверяет count/canonical literal/public IPv4.
  До8 sequential address attempts делят оставшийся исходный request deadline;
  timeout/failure с подтверждённым original close может попробовать следующий
  захваченный address. Unknown cleanup/scope/busy прерывают перебор. После
  successful connect deadline возвращается к исходному request deadline,
  без нового срока и без повторного DNS lookup.8+1SOCKS < existing10 slots.
- NonWindows producer честно unavailable и не включает implicit Go resolver.
  SOCKS hostname остаётся в existing synchronous remote CONNECT codec;
  контрольный адрес не заменяет SOCKS/защищённый путь после отказа.

Existing captured App/path/mode/intent/control/applied session, optional budget,
HTTP header/plaintext/body+1 bounds, actual origin/IP parser, joined watcher,
original Stop done fence и запрет прямого fallback сохранены. Public API/DTO,
frontend/native counters/SSID/protection admission не менялись.

### Ограничения

Синхронный GetAddrInfoW не получает принудительной отмены. Он может превысить
requested3s/6s; Stop имеет существующий bounded join и при неизвестном возврате
оставляет cleanup error/protection. Это ограничение availability/SLO остаётся
явным. Original native frame не заменяется таймером, но hard bound на системный
DNS provider, Win32 внутренние tasks, kernel/RSS/latency и атомарность всей ОС
не доказан. Async native cancellation/process-boundary evolution требует
своего контракта и приёмки; не объявлен уже реализованным.

Windows resolver может использовать hosts/cache/политику ОС. Результат не
доказывает wire DNS, TTL, независимый физический маршрут, protected DNS/TUN
или реальную скорость. Native ABI, pointer/allocator behaviour, balanced WSA
reference, long lookup/late result/Stop races/address selection и nonWindows
build не проверены. Нет нового DNS store/cache или fallback-provider.

Далее — typed per-route outcome/measurement age, чтобы skipped/budget/busy/
unavailable не выглядели измеренным сетевым отказом; затем actual PC-01
W4c/W3b implementation gaps с сохранёнными native gates.

3Go4docs/7 originals(5existing/2new), manual1original diff+2new source files,
14named/3cached unchanged; touched gofmt3+2 exit0. Tests/fixtures/build/vet/
types/product parser/SDK execution/generation/browser/native/admin/network/
real env/log/vault/other roles/chats/subagents не выполнялись/не менялись.
100OPEN56NOT_RUN/fullPC00…09/old9/native/SDK/OS/wholeUX/W4c/W3b/release OPEN;
TUN gatefalse/schema2 unsupported сохраняются.

<a id="pc-v366-egress-status"></a>

## V2-366 — отдельные исходы и возраст HTTP-наблюдений

2026-11-10, **source/unverified**, PC-03/04/06. V365 — progress;
полный PC-00…09 и прежние девять этапов остаются активными.

### Контракт и producer

NetworkFacts получил optional egress/schema1. Два фиксированных observation
points os_control/tunnel_socks имеют outcome, skip reason, attempted, stage,
HTTP status, startedAt/completedAt и nullable elapsedMicros/ageMicros.
Новый core/egress_status.go содержит display-only value contract и helpers;
это не общий SDK, новый health consumer или forwarding authority.

- Outcome различает ok/skipped/busy/budget/canceled/timeout/unavailable/
  invalid_response/http_status/cleanup_unconfirmed. Skip reason ограничен
  policy/no_session/no_intent/not_requested. Отсутствие попытки не получает
  synthetic0 duration или failed network verdict.
- Attempted означает вход в original target frame, не фактический пакет или
  достижение сервера. Stage фиксирует local DNS/connect/proxy/TLS/request/
  headers/body/complete. Raw provider error/URL/credential в metadata не попадают.
- Core response — только bounded HTTP bytes. Existing main IP/JSON parser
  единственный назначает ok, иначе invalid_response; legacy DNSViaTunnel
  сохраняется ради wire compatibility, но не является DNS/TUN proof.
- Context/budget admission, existing busy owner и known retained failed owner
  получают разные outcomes. Общая отмена/expiry не публикует raw IP bytes;
  unknown original cleanup остаётся отдельным outcome и сохраняет owner/budget.
  У не начатого запроса не появляется invalid_response от другого target.

Original native DNS/WSA/result/socket/body/watch/Stop/budget cleanup и captured
App/path/mode/intent/control/applied session не заменены metadata. Никакого
direct fallback, нового endpoint, процесса, таймера или grant не добавлено.
Endpoint http_status не объясняет DPI и не означает отказ VPN.

### Время и cache

Start/end берутся из самой synchronous attempt, duration — monotonic Sub.
Их frame включает resolution/handshake/HTTP work, не является RTT или native
retirement duration. Clock timestamps предназначены для отображения; frontend
не сравнивает их порядок как health/freshness gate и не доверяет client clock.

AtReply создаёт value copy с monotonic age исхода на момент ответа backend.
GetNetworkFacts копирует nested group перед добавлением age: shared cache и
immutable elapsed pointers не меняются; cached read не освежает started/end
или общий collection-start TTL. No-attempt timing null, genuine observed0
не теряется. Existing infoFacts receipt/interest/pending owners неизменны.

### Facade и два существующих экрана

Strict decoder проверяет schema, fixed routes, allowlisted states/reasons/stages,
boolean attempt, UTC timestamps, safe nonnegative durations/age, HTTP range и
stage/outcome relations. Unsupported/malformed present format не downgrade в
legacy positives: соответствующие IP/geo/org и bool скрыты, local fields остаются.
Raw IP parsing остаётся у Go; frontend проверяет bounded display string/metadata.
Старый отсутствующий schema совместим, но подробности/время не выдумываются,
legacy false отображается как unknown вместо measured failure.

Existing api/labels используется в Network/Dashboard. IP/status/time находятся
в прежних rows, повторная отдельная SOCKS status row удалена. Age подписан
«при передаче», не обновляется новым timer и не получает guessed validity.
Нет новых cards/navigation/CSS. Reused modern-web-guidance accessibility guide:
automatic observation updates не создают дополнительные noisy live regions.

### Доказательство и остаток

4Go3TS2Vue4docs/13 originals(12existing/1new), manual8original diffs+new file;
16 named unchanged/0 cached dependency inputs. Touched gofmt4+1 exit0.
Tests/fixtures/build/vet/typecheck/product parser/SDK execution/generation/
browser/native/admin/network/real env/log/vault/other roles/chats/subagents
не выполнялись/не менялись. Visual/responsive/assistive-tech и runtime acceptance
не доказаны. Нативные Wails bindings не генерировались/не патчились.

Наблюдение остаётся historical metadata at reply. Оно не доказывает текущее
состояние после следующей смены сети/сессии, независимость маршрутов, DNS,
отсутствие утечек, W4c/TUN admission, throughput или завершённый PC-03/04/06.
Source/current-context display после последующих событий остаётся отдельным
PC-04 обязательством. Win32 lookup может превысить requested deadline;
forced native cancellation/SLO, native ABI/OS/resources/release ещё OPEN.

Следующий активный срез — actual PC-01 W4c/W3b implementation gaps; не
продолжать бесконечно расширять passive facts вместо protection/lifecycle.
100OPEN56NOT_RUN/fullPC00…09/old9/native/SDK/OS/wholeUX/W4c/W3b/release OPEN,
TUN gatefalse/schema2 unsupported сохраняются.
