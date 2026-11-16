# KA-01/W3a — собственные записи системного прокси

V2-192, 2026-10-30. Owner: `windows`. **Partial source, unverified**.
W3 целиком не закрыт: process-local receipt не разрешает восстановление после
падения процесса. [Внешний UserMode/SYSTEM migration request](CHANGE-REQUEST-NETGUARD-001.md).
Все девять TH-этапов и предыдущие W1/W2 native gates сохраняются.

## Найденные дефекты

- Snapshot подменял ошибки/отсутствие значения нулями и пустыми строками.
  Тип DWORD/QWORD не различался, SZ/EXPAND_SZ и missing/empty при Restore терялись.
- Manager назначал cleanup после Snapshot; частичное native acquisition не имело
  зарегистрированного cleanup. Snapshot мог разрешить обратную запись даже без
  собственного SetProxy — в том числе после отказа Engine до любых proxy writes.
- Restore перезаписывал текущий чужой tuple старой копией. PAC delete и WinINET
  notification errors игнорировались. Original key Close не учитывался.
- Startup/manual reset считал fixed port/dead listener доказательством владения.
  PAC при этом не был частью полного сравнения; другой VPN мог иметь те же bytes.
- NetGuard UI повышал task ready/running, скрытую SYSTEM задачу и listener до
  защиты/работающего auto-heal. Отсутствие log трактовалось как DoH disabled.

Это вывод по исходникам. Системные настройки, процессы, пользовательские логи и
реестр этой машины не читались/не менялись инструментами разработки.

## Локальный owner и receipt

`ProxyManager` остаётся владельцем Snapshot/SetProxy/Restore; общий интерфейс
Manager/noopProxy и wire DTO не меняются. В `proxy_registry_windows.go` вынесен
ограниченный reader/writer, новый параллельный proxy service не создаётся.

1. Snapshot под process-local mutex резервирует одного original writer и registry
   handle. Cleanup зарегистрирован Manager **до** вызова Snapshot. Read failure
   сохраняет частичный owner до Close; Snapshot без собственных write receipts
   разрешает только закрыть handle, не писать исходные настройки обратно.
2. Читаются presence/type/raw bytes трёх фиксированных значений. Не настоящий miss,
   wrong type, >16KiB/value, malformed/noncanonical UTF-16 или недоступность — error,
   не disabled/default. Поддержанные canonical empty SZ и missing различаются;
   EXPAND_SZ не раскрывается и возвращается тем же типом. QWORD не обрезается до
   DWORD. Неподдержанные представления отклоняются, не нормализуются.
3. Bounded double observation проверяет весь tuple. До каждого собственного
   write нужен exact current receipt, после — exact full readback. Address/PAC
   перед Enable, enable последним. No-op не получает нового write receipt.
4. Перед native write отмечается pending. Nil return + exact readback подтверждают
   новую запись. Write error + unchanged observed tuple не дают нового разрешения.
   Unknown/changed readback, panic или ошибка чтения после попытки сохраняют
   pending и original owner; повторная запись/автоматический repair запрещены.
5. Restore меняет только подтверждённый current tuple, по шагам обратного порядка;
   каждая partial restore тоже получает readback receipt. Чужой tuple не затирается.
   Уже наблюдаемое exact original state позволяет завершить без повторных writes.
   Обязательные WinINET notifications проверяются отдельно от registry readback.
6. Owner снимается только после confirmed original handle Close. Ошибка Close
   sticky: старый handle не заменяется и второй Close не выдаётся за доказательство.
   Read-only observer также удерживает original query handle при Close uncertainty;
   новый reader/writer не подменяет его. Максимум один writer и один query owner
   в этом процессе — source domain, не измеренный OS resource bound.

## Границы гарантии

Mutex — только внутри процесса. Registry API не даёт здесь atomic tuple CAS:
чужая программа/MDM может записать между read/write, вернуть те же bytes или
пересоздать key. Double observation/readback обнаруживают часть конфликтов,
но не доказывают отсутствие гонки/полной native identity. Нет interprocess lease,
durable crash receipt, authenticated prior owner/process death, power-loss ordering
или доказанной прерываемости registry/WinINET calls. Нового обещания fail-closed
защиты через proxy нет; WFP/TUN и различие manual disconnect/error остаются прежними.

`HealStaleSystemProxy` сохраняет legacy API, но возвращает unavailable без
сохранённого original receipt и ничего не меняет. Startup heuristic вызов удалён.
Обычный Stop использует process-local owner выше. После аварии старые proxy values
могут остаться: неподтверждённый сброс не выдаётся за исправление. Persistent
recovery, безопасный общий startup/manual/UserMode writer и миграция старых
scripts/tasks ещё обязательны; W3b остаётся открытым.

## Наблюдения и UI

NetGuard использует общий bounded typed reader. Ошибка proxy read/Close приходит
в существующий error channel, poller снимает старые data. Отсутствие log — лишь
отсутствие evidence, не installation/DNS verdict; другие ошибки log возвращаются.
Наблюдаются все три известные task names, включая NetEvent. Stale ownership не
выводится из port; старые boolean DTO поля не получают выдуманных новых фактов.

На двух существующих Vue страницах убраны защиты по task/listener, автоматические
обещания и кнопка неподдержанного repair. Reuse PageHeading/HintBox/poller, без
новой страницы/framework/CSS/generated changes. Local port — только доступность,
task — только task, log — история. Empty/error не означают отсутствие сбоев.
Modern Web Guidance `accessibility`, §8: polite status updates и отдельное error
announcement; screen-reader/native acceptance пока не выполнялась.

## Evidence и оставшиеся gates

`.artifacts/memory/proxy-receipt-source-evidence.json`: exact6 Go +2 Vue, docs,
original source diff, retained inputs и captured gofmt. Первая попытка replace
через delete/add была отклонена patch parser; до записи hash исходного файла
совпадал с baseline, нового файла не было. Изменение выполнено затем отдельно.
Это ошибка инструмента редактирования, не product/runtime failure.

Primary API source: [x/sys v0.44.0 registry/value.go](https://github.com/golang/sys/blob/v0.44.0/windows/registry/value.go),
[RegQueryValueExW](https://learn.microsoft.com/en-us/windows/win32/api/winreg/nf-winreg-regqueryvalueexw),
[RegSetValueExW](https://learn.microsoft.com/en-us/windows/win32/api/winreg/nf-winreg-regsetvalueexw).
Typed reads могут отличать miss/type errors; native raw strings не гарантируют
терминацию. Поэтому reader ограничивает bytes и требует воспроизводимого формата.

Tests/fixtures/build/parser/vet/typecheck/SDK/native/admin/live/OS/worker/install/
network действия не выполнялись. Нет browser/visual/screen-reader acceptance.
Нужны разрешённые future cases: missing/empty/EXPAND_SZ/wrong type/oversize/read
error, Snapshot failure without writes, partial native write/readback/notify/Close,
foreign change on each step, original restored externally, parallel managers,
Stop/error barriers, mutex/thread/native interprocess/key-replacement/crash/reboot,
stale callback/UI unavailable и migration старого external script/installed task.
Full W3/W4, durability/SDK parity/OS/old-client/DR/resources/service/release/
comparative/all9 gates открыты; schema2 update всё ещё unsupported.

<a id="pc-v304-proxy-validation"></a>

## V2-304 — повторная проверка применённого proxy перед допуском и Running

**Source/unverified, 2026-11-08.** Прежний W3a остаётся process-local. Весь
PC-00…09 и девять TH этапов активны;100 OPEN/56 NOT_RUN. Tests/fixtures/build/
vet/typecheck/product parser/SDK/native/registry/WinINET/network не выполнялись.

### Конкретный пробел

V2-192 проверял readback каждой собственной записи, но между ним и финальным
Running выполняются WinINET notification, source/worker checks и control RPC.
Внешняя программа могла изменить proxy в этом интервале без повторного чтения
исходным writer. Display ProxySnapshot/loopback port не подтверждает оригинальное
состояние и не подходит для этого решения.

WFP исходники отдельно прослежены от original helper/capture/prepare до App native
commit/worker consume/Manager completion. Эта цепочка уже соединена в source;
её native acceptancefalse намеренно сохранён. Создавать ещё один commit path или
включать константу вместо отложенной Windows/WFP/Process/Job приёмки нельзя.

### Исходный proxy owner и applied phase

В существующем ProxyManager добавлен ValidateAppliedProxy. Он использует тот же
retained registry handle, process-local proxyOwner и bounded typed double reader
трёх фиксированных values: presence/type/raw bytes сверяются с pm.current.
Новый query handle, display snapshot, строковое сравнение порта, writer или
автоматическое восстановление чужого tuple не создаются.

Private applied flag означает только успешно завершённый SetProxy/notification:

1. До нового SetProxy фаза снимается; partial write/ошибка уведомления не могут
   оставить старый положительный результат.
2. После полного existing write/readback sequence и успешного WinINET notify
   фаза появляется. Отдельный readback выполняется потребителем до допуска.
3. При validation фаза снимается до native read; только actual exact readback
   возвращает её. Read error/panic/conflict оставляют её снятой; последующие
   совпавшие bytes не воскрешают фазу без нового явного SetProxy.
4. Restore снимает applied до восстановления/закрытия. Original own-write,
   pending/uncertain/Close receipts сохраняют прежнюю отдельную семантику.

SetProxy, который обнаружил уже совпадающий target и не писал values, по-прежнему
не получает pm.writes или права restore-write. Applied/readback не утверждает,
что чужие идентичные bytes были записаны приложением. Ошибка проверки не даёт
права перезаписать внешний tuple; старый Restore отказывает при конфликте.

### Интеграция в существующий lifecycle

- Новый proxy_receipt.go содержит optional systemProxyValidation refinement и
  Manager wrapper. Базовый systemProxy interface не изменён; existing explicit
  custom/noop adapters сохраняют контракт. Native ProxyManager предоставляет
  проверку; skip для TUN noop/custom не выдаётся за native proxy proof.
- Manager commitEngineForwarding сначала проверяет applied original proxy. При
  ошибке прежний markTerminalFailureLocked снимает AllPassed, помечает report
  stale; positive forwarding не запрашивается. Original error/cleanup пути остаются.
- Общий finishLifecycle повторяет readback перед финальным state decision.
  Статические caller sites Start/select/Reload/rollback просмотрены: validation
  выполняется вне Manager/Engine locks. После IO finalizer снова проверяет context/
  deadline/lifetime/source/active operation/Stop и под value lease публикует state.
- Отказ после уже выполненного commit идёт в существующий restrictive cleanup:
  protection→original engine retirement→only-owned proxy Restore. Ни сам readback,
  ни сообщение об ошибке не снимают strict WFP policy и не дают direct fallback.
  Неуспешная очистка удерживает прежнего владельца и остаётся отдельным error.
- Finite terminal message теперь учитывает proxy/owner/source invalidation;
  actual proxy values, PAC contents и raw registry bytes не выводятся.

### Границы гарантии и остаток

Это две ограниченные observations, не атомарный tuple CAS/registry transaction
или lease против внешней программы. Чужая запись после финального чтения,
same-byte ABA, key replacement/MDM, фактическое WinINET consumer refresh, mutable
settings во время уже Running, periodic observer, interprocess generation и
authenticated crash receipt этим не закрыты. Открытие/чтение/notify/закрытие могут
блокировать ОС; context проверяется до/после wrapper, hard native bound не заявлен.

Существующий process-local owner/Manager operation не заменяет durable W3b,
общий startup/manual/UserMode recovery или native W4c. HealStaleSystemProxy остаётся
unavailable; matching port/byte tuple не создаёт права исправлять чужие настройки.
Existing imported configs/private profiles/runtime registry не читались и не менялись.

Manual4 final source/diff/caller/lock review: native proxy, validation wrapper,
forwarding commit и terminal finalizer.11 immutable reviewed SHA;8 original
snapshots7existing1absent/task71rev707; gofmt4+2 exit0. Evidence:
bootstrap-v304-proxy-validation-source-evidence.json. Source/formatting/byte
identity не доказывают compile/Windows/interop/resource/runtime correctness.

Goal/constraints/decisions/checks/tasks0…70 без waiver;100 OPEN/56 NOT_RUN.
Bootstrap/bridge/native/chain/relaunch/parent/helper gatesfalse, ranBeforeJob
sticky, GUI6/R1/U1/IPC9/Job8/TUN unavailable/schema2 unsupported. V301 UDP custody,
V303 rate limits и V302 dated server baseline сохранены. Android/shared/configs/
harness/чужие чаты/сервер/OS/dependencies не изменялись.

Далее независимый PC04 source объём: Главная/Проверки/Локальное восстановление,
actual scope/freshness/unknown/error и безопасный следующий шаг через existing
components/composables/API. Без новых выдуманных fields или ручной генерации Wails
bindings. Loading/empty/blocked/keyboard/responsive source прорабатывается отдельно;
browser/WebView2/native/UX/release/perf и весь PC00…09/old9 остаются открытыми.

<a id="pc-v343-proxy-key"></a>

## V2-343 — исходный HKEY и запрет повторного native Close

**Source/unverified, 2026-11-09.** PC-01/06, полный PC-00…09 и прежние девять
этапов active. V342 — progress.100 OPEN/56 NOT_RUN, без waiver.3 Go/4 docs;
7 snapshots:6 existing/1 new, original task71/revision819 сохранена.

### Выбор следующего P0

W4c прослежен от original actor/filter/capture через prepared issuer к platform
native commit, private worker consume и Manager outcome. Цепочка уже есть в
исходниках за compiled nativeTUNForwardingAccepted=false. Это не основание
подменять её вторым путём, открывать константу или выдавать source за Windows
приёмку. Gate сохранён false; TUN unavailable/schema2 unsupported.

Следующий конкретный source defect относится к PC-01 proxy custody:

- Snapshot назначал proxyOwner/key лишь после registry.OpenKey. В закреплённом
  x/sys v0.44.0 OpenKey native output хранится локально; при error wrapper
  возвращает0. Частичный/abnormal выход не сохранялся в original ProxyManager.
- Restore помечал Close failure только после returned Key.Close error. Panic/
  Goexit/unreturned Close не получали pre-call marker, допуская повторный Close
  и registry I/O через alias с неизвестным native outcome.
- ReadSystemProxySnapshot защищал только returned closeErr. Abnormal Close мог
  оставить closeErr=nil, и следующая observation перезаписала бы original key.
  При Close error populated display tuple также оставался в named return.

Это вывод по исходникам, не observed leak/handle reuse или исполненный failure.
Реестр рабочей машины, PAC, текущий proxy и чужие процессы не читались/не менялись.

### Реализация и reuse

1. New proxy_registry_owner_windows.go — общий original HKEY primitive для двух
   существующих lanes, не новый proxy service. Адаптирован уже имеющийся
   autostart_registry_windows.go original-output/one-Close pattern; переиспользован
   nativelease.ValidHandleValue, дополнительно исключены predefined HKEY forms.
   Representation check не доказывает actual object type/authority/key identity.
2. Original containing owner резервируется до native Open. UTF16 path backing и
   native output handle принадлежат этому object до RegOpenKeyEx. Open started/
   returned/error/unknown не восстанавливаются из later key lookup. Невалидный
   output не даёт read/write или Close authority; pointer остаётся restrictive.
3. closeStarted/sticky error ставятся до RegCloseKey; closeReturned/result и
   closed относятся к той же попытке. Panic/Goexit/невернувшийся/returned failed
   Close не заменяются повтором. No acquisition и normal failed zero-output
   Open разрешают known no-handle cleanup; valid nonzero original output хранится
   до собственного Close. Не добавлены timeout worker или forced native bound.
4. ProxyManager хранит native owner и borrowed registry.Key alias. Alias выдаётся
   только после usable original Open; ready проверяет exact native pointer/state/
   handle. Snapshot failure сохраняет partial owner до существующего Manager
   cleanup. Restore снимает applied до дальнейших проверок; native Close started
   или неизвестный original owner запрещают registry read/write через alias.
   Не повторяются restore writes после неизвестного Close.
5. Presence/type/raw bytes, bounded double read, per-write readback, no-op без
   own-write authority, conflict refusal, PAC preservation и notify semantics
   сохранены. Чужой tuple не перезаписывается, а port/listener не становится
   recovery receipt. Mutex order proxyOwnerMu→ProxyManager.mu сохранён.
6. ReadSystemProxySnapshot использует новый retained original owner до Open.
   Deferred cleanup выполняется на error/abnormal read и проверяет тот же native
   Close outcome; unknown native owner не заменяется следующим reader/writer.
   Любая ошибка возвращает empty display tuple вместе с error. Known retired
   handle допускает новый query; display projection не получает restore authority.
   Единственный production consumer netguardProxyState проверяет error до fields.

### Просмотр и границы

Manual: два original diffs и один новый owner, Snapshot reserve/error/partial key,
normal failed Open/zero output, unusable/pseudo HKEY, original Close error/panic/
Goexit, repeat Restore before alias I/O, read-only cleanup, caller and lock order.
14 immutable reviewed inputs;2 cached dependency source SHA: pinned registry
OpenKey и generated RegOpenKeyEx/RegCloseKey wrappers. Read-only source lookup,
без запуска этих API. Touched-only gofmt3+3+1, все explicit exit0.

Не добавлялись/не запускались tests/fixtures/build/vet/parser/typecheck/SDK/product
CLI/browser/Windows registry/WinINET/native/admin/network. Нет actual handle/
FFI/cancellation/deadline/resource correctness proof. Source/formatting/SHA не
устанавливают compile или native acceptance. Android/shared/configs/harness,
зависимости/generated/чужие чаты и действующие OS настройки не изменялись.

OPEN: key path replacement/registry view/native ABI/handle lifecycle and sharing,
interprocess CAS/ABA/MDM/foreign writers, WinINET consumer refresh, data read/write
native frames/resource bounds, authenticated durable W3b/crash recovery/UserMode
migration and full W4c/OS/release/comparison. Original source fixes do not supply
those receipts. HealStaleSystemProxy остаётся unavailable.100OPEN56NOT_RUN, all
bootstrap/bridge/native/chain/relaunch/parent/helper gates=false; TUN unavailable,
schema2 unsupported.55% — прежняя грубая оценка, не замер после этого изменения.

**Далее:** PC-01 W3b durable original-write/recovery contract и оставшиеся
read/write/notify ownership seams через существующий Manager/ProxyManager,
без неавторизованного repair чужого proxy или открытия W4c gate; полный PC-plan,
Windows SDK/adaptive integration и непроверенные metrics/UX/release сохраняются.


<a id="pc-v344-proxy-phases"></a>

## V2-344 — исходная попытка записи и уведомления WinINET

**Source/unverified, 2026-11-09.** PC-01/06; полный PC-00…09 и прежние девять
этапов active. V343 — progress;100 OPEN/56 NOT_RUN, без waiver.4 Go/4 docs;
8 snapshots:6 existing/2 new, original task71/revision822 сохранена.

### Что ограничивает восстановление после crash

Сверен текущий ownership.json: root tools/netguard не назначен Windows, остаётся
review-required; NETGUARD-001 требует согласования общего original-write receipt,
interprocess owner и installed-old/mixed migration. Его текст — не разрешение на
внешний writer. Root scripts/tasks, другие чаты и правила владения не менялись.
Долговременный recovery format не введён односторонне. Нынешние записи остаются
process-local; HealStaleSystemProxy unavailable. Это обязательный остаток W3b,
а не отмена части цели и не будто уже исправленное восстановление после аварии.

### Конкретный source-пробел внутри ProxyManager

В V343 pending сохранял факт неизвестной записи, но before/target/readback и
точный callback outcome были локальными переменными. Уведомление WinINET вообще
не имело original attempt в ProxyManager. После abnormal Find/Call applied был
false, однако обычный Restore мог снова начать registry writes/notification.
Ещё DeleteValue ErrNotExist превращался в nil: foreign delete между read и call
мог получить наше own-write подтверждение. Это вывод из исходников, не observed
failure этой Windows или native acceptance.

### Изменения и reuse

- proxy_write_windows.go удерживает только последнюю попытку в existing writer:
  fixed index, before/target/observed typed tuples, adapter callback/readback
  started/returned/error и finite outcome. Record публикуется до callback; raw
  slices — immutable private bounded reader/value data, не произвольный caller
  buffer. Нет истории-цепочки, endpoint cache, нового service/worker или таймера.
- SetProxy/Restore продолжают прежние typed setters, full-tuple double-read,
  readback и no-op ordering. Callback unknown запрещает даже readback; readback
  error/abnormal/несовпадение остаются restrictive. Normal callback error с exact
  before readback остаётся observed unchanged, не создаёт новую write authority
  и не доказывает отсутствие внешнего ABA. Only nil callback+exact target readback
  подтверждают запись; существующий pending/applied contract сохранён.
- Общий phaseErrorLocked вызывается из ready/write/Restore/notify. Pending или
  unknown write/readback/notification нельзя обойти новым совпавшим tuple,
  повторным callback или original handle Close. Same-writer I/O блокируется до
  known outcome; это не новый глобальный запрет независимого read-only display.
- proxy_notify_windows.go удерживает original lookup и два fixed InternetSetOptionW
  calls39/37 с before-call/returned/result markers. Shared lazy procedure не
  вызывает WinAPI на чтении source. Сохранён старый порядок и оба returned FALSE
  результата, включая finite fallback при ERROR_SUCCESS. Lookup failure до calls
  и ordinary returned API error отличаются от panic/Goexit/unreturned attempt.
  Нормальный error разрешает прежнюю conflict-checked restore sequence; unknown
  не даёт начать новые writes/notify/Close или заменить попытку. Applied требует
  завершённого успешного notify; это не доказательство WinINET consumer refresh.
- writeProxyRegistryValue получает fixed-index/16KiB/type/DWORD/canonical UTF16
  guard до typed setter. Missing-at-delete больше не нормализуется в success:
  effectful pre-read ожидал present value, другой writer мог удалить его первым.
  Такой normal error не получает нашего delete receipt. Real no-op уже обнаружен
  перед callback и по-прежнему не создаёт own-write authority.
- Original HKEY/one Close из V343, presence/type/raw bytes, foreign tuple refusal,
  PAC/EXPAND_SZ и protection semantics сохранены. Новые raw tuples могут содержать
  PAC credentials: они private, не JSON/логи/durable files. References снимаются
  только после successful known Restore/Close; secure wipe/GC/RSS не заявлены.
  Windows public API/DTO/bindings и другие consumers не менялись.

### Просмотр и ограничения

Manual2 original diff+2 new helper: write no-op/normal failure/readback/error/
unknown; lookup/call39/call37 success/FALSE/panic/Goexit; deferred Manager cleanup,
phase gates до alias I/O, repeated mutation refusal, raced missing delete,
private raw data lifetime.12 immutable reviewed SHA+1 cached x/sys value.go:
existing setters/delete wrappers source inspected, не исполнены. Touched gofmt
4+4+4 с explicit exit0, последний после всех source правок.

Слой receipt — **Go adapter callback inputs/results**. Внутренние x/sys encoded
buffers, literal native stack/ABI retirement, cooperative cancellation и actual
WinINET consumer refresh этим не аттестуются. New raw FFI не добавлен. Не были
добавлены/запущены tests/fixtures/build/vet/parser/typecheck/SDK/product CLI/browser/
registry/WinINET/native/admin/network. Реальные settings, logs, profiles/env/vault/
secrets не читались. Source/formatting/SHA не устанавливают runtime correctness.

OPEN: authenticated durable W3b original-write journal, crash/reboot/process-death
proof, interprocess lock/CAS/ABA/MDM/foreign writer conflicts, external UserMode/
SYSTEM migration/coordination, native query/write/notify frames/resources/FFI/
ABI/deadlines/sharing, full W4c/OS/release/comparative. Unknown phases may keep
proxy settings restrictive until safe recovery/process restart; guaranteed
availability/speed не заявляются.100OPEN56NOT_RUN/all gates=false/TUN unavailable/
schema2 unsupported/fullPC00…09/old9 active;55% остаётся грубой оценкой.

**Далее:** Windows recovery candidate/owner evidence и оставшиеся read/notification
retirement seams по PC-01 с сохранением NETGUARD-001 boundary; затем полная SDK/
adaptive/P0/UX/release матрица. Не включать automatic crash repair по process-local
record, port, missing listener, тому же tuple или успешному WinINET return.


<a id="pc-v345-proxy-read"></a>

## V2-345 — удержание исходного чтения реестра

**Source/unverified, 2026-11-09.** PC-01/06/fullPC00…09/old9 active; V344 — progress.
4 Go/4 docs;8 snapshots7existing1new/task71rev825.100 OPEN/56 NOT_RUN, без waiver.

**Пробел.** Query buffer/type/length/error и двойной snapshot были локальными.
Unknown read мог потерять эти данные и разрешить Close исходного HKEY. Finding
по source, не наблюдённый failure Windows. Actual settings не читались.

- New proxy_read_windows.go удерживается original proxyRegistryKeyOwner до
  подготовки имени/буфера/native query. Запись содержит borrowed original key,
  2×3 fixed query slots, UTF16 names, buffers≤16KiB, native type/length/error,
  started/returned и обе observations. RegQueryValueEx writes в эти поля; caller
  KeepAlive сохраняет original backing. Повтор run или смена owner/key не даёт
  нового query. Ни borrowed key, ни совпавшие bytes не дают Close authority.
- Existing bounded typed checks вынесены в pure decodeProxyRegistryValue без
  изменения DWORD/SZ/EXPAND_SZ/canonical UTF16/presence правил. Missing означает
  actual FILE_NOT_FOUND; more-data/type/length/read errors не дают default0 или
  accepted prefix. Две exact observations остаются проверкой, не atomic CAS.
- Snapshot, ValidateAppliedProxy, before-write, after-write readback, Restore и
  read-only projection используют один native owner read path. Старые helper-local
  GetValue/state paths удалены. Native owner ready/Close отказывают при unknown
  original read frame; новый read не заменяет его. Normal returned query/shape/
  conflict error позволяет known cleanup; abnormal/unreturned query сохраняет
  record/buffers/HKEY. Existing write/notify phase barriers из V344 сохранены.
- После known Close references отпускаются, не обещается secure erase. Данные
  private/no logs/JSON/durable files. Нет новой goroutine/таймера/повторного query
  при more-data, proxy writer/repair authority или публичного API/binding change.

**Просмотр:** manual3 original diffs+1 new, all6 production call sites, missing/
oversize/type/UTF16/conflict, normal error/abnormal callback, replay/owner/Close
order.10 reviewed SHA+1 cached x/sys v0.44.0 generated query wrapper. gofmt4+4+1
explicit exit0. Tests/fixtures/build/parser/types/SDK/registry/WinINET/native/
admin/browser/network не добавлялись/не запускались; реестр/logs/env/vault не читались.

**OPEN:** actual native query/buffer/FFI/ABI/handle/cancellation/deadline/sharing/
resource/RSS correctness, key identity/replacement/CAS/ABA/MDM/interprocess writer,
durable authenticated W3b/crash/process-death/UserMode protocol and installed-old
NETGUARD migration. Source/SHA/gofmt не закрывают runtime/OS/W4c/release/comparative.
All gates=false/TUN unavailable/schema2 unsupported; fullPC00…09/old9/100OPEN56NOT_RUN.
55% — прежняя грубая оценка, не измерение. Не вводилось автоматическое crash repair.

**Далее:** Windows recovery evidence/contract и полный PC-01/SDK/adaptive/UX/release
план с сохранением NETGUARD-001 ownership boundary; исходные открытые obligations
и непроверенная metrics/proxy/native приёмка сохраняются.


<a id="pc-v346-proxy-conflict"></a>

## V2-346 — known foreign conflict отдельно от неизвестной очистки

2026-11-09. PC-01/04/06. **Partial source, unverified**.

**Дефект по исходникам:** обычный чужой tuple конфликт оставлял original HKEY
и process-local owner занятыми. Manager оборачивал это в cleanup failure даже
при нормально вернувшихся callback/readback. Простое подавление conflict error
дало бы обратный дефект: следующий кандидат мог взять новый Snapshot и заново
переписать внешние настройки. Нужны отдельные resource и retry outcomes.

**Реализация:**

- ProxyManager удерживает sticky conflict original snapshot. Snapshot/read/
  validation/pre-write conflict сохраняется; later matching bytes не снимают
  уже наблюдавшийся конфликт. Fresh Snapshot и retirement сбрасывают applied.
- При известном конфликте, без pending write или unknown read/notification/
  Close, выполняется только Close original HKEY. Новых registry writes/WinINET
  notification в этой ветке нет. Private receipt содержит exact ProxyManager,
  original native owner и outcome; public Restore возвращает conflict error.
- Manager принимает исключение только под value lease original proxy/native/
  confirmed native Close/zero alias/cleared raw references и одноразовым consume.
  Порядок locks: proxyOwnerMu → ProxyManager.mu → Manager.mu, без I/O/events/
  callback под коротким lease. Другие cleanup errors не подавляются. Fake/noop/
  display tuple/error/same port не могут создать такой native retirement receipt.
- Manager sticky barrier запрещает Start/select/Reload и forwarding validation
  этого экземпляра; lifecycle return сохраняет conflict даже при исходном другом
  probe/network cause и отзывает joined positive rollback. Stop остаётся доступен.
- App хранит отдельный negative retry latch. Candidate loops, timer/network/
  certificate wakeup, startup resume и positive DNS retain учитывают конфликт.
  Predecessor Stop проверяется до замены Manager; импортированный профиль может
  commit, но не запускает VPN автоматически и не оставляет UI в starting.
- Только принятый public Start после обычных busy/closing/source checks снимает
  App latch. Это не отменяет cleanup/trust/native gates. Если conflict впервые
  обнаружен при retirement predecessor или сохраняется другая ошибка cleanup,
  этот запуск всё ещё отказывает; нужен завершённый cleanup и новый явный intent.
- Existing scalar BlockedReason + finite label/text/log объясняют proxy_conflict.
  Error/tuple/адреса/PAC не выводятся. cleanup_unconfirmed имеет приоритет; ручной
  Disconnect и его existing original WFP Release gate не превращены в autoretry.

**Просмотр:** manual16 original source diffs+1 new component; exact receipt/
one-use lease/lock order, original native read/write/notify/Close conditions,
candidate/rollback/Stop/replacement/trust/pin/DNS/profile/resume callers и два
TS API label/type изменения.13 unchanged reviewed sources; gofmt13+2 explicit0.
15Go2TS4docs/21snap20existing1new. Tests/fixtures/build/parser/types/SDK/browser/
registry/WinINET/native/admin/network не добавлялись/не запускались; actual
registry/env/vault/profile/log data не читались и не менялись.

**OPEN:** source reasoning не доказывает actual native Close/ABI/FFI/cancellation/
deadlines/registry sharing/RSS/UX/module acceptance. Tuple reads не atomic CAS,
не различают внешний ABA/одинаковые bytes/key replacement/MDM/interprocess writers.
До обнаружения конфликта могли состояться собственные записи; при unknown outcome
ветка known preservation не применяется. Private reference release не secure erase.
Process-local receipt не разрешает crash/startup recovery и не удостоверяет Engine,
WFP, whole-process cleanup или защищённый egress. Durable authenticated W3b/UserMode
protocol и installed-old NETGUARD migration по NETGUARD-001 остаются отдельно OPEN.
All100OPEN56NOT_RUN/gates=false/TUN unavailable/schema2 unsupported/fullPC00…09/old9;
55% — прежняя грубая оценка, не измерение готовности.

**Далее:** полный PC-01 recovery/native и PC-04 action/presentation matrix; SDK/
adaptive/OS/release/comparative и исходные незакрытые обязательства сохраняются.


<a id="pc-v367-proxy-writer"></a>

## V2-367 — удерживаемый writer lease для Windows-прокси

2026-11-10, **partial source/unverified**, PC-01 W3b. V366 — progress;
полный PC-00…09/прежние девять этапов сохраняются. W4c compiled gate всё ещё
false; его source callback не превращён в native acceptance.

### Зачем этот компонент

Process-local proxyOwnerMu координировал только один процесс. Persistent
original-write protocol требует также native serialization согласованных
Windows clients. Существующий filetxn journal writer уже имеет original
parent/file owner, bounded path/ACL/NTFS/FileID checks и fail-immediate
LockFileEx. Его callback lifetime был короче ProxyManager Snapshot→Restore.

Новый WindowsWriterLease переиспользует этот domain/acquireWriter/checkWriter/
release path; native lock/ACL code не копировался и не менялся. Object хранится
до Acquire. Partial/unknown acquisition, validation и Close не подменяются
новым object/path/PID cleanup. Нормальная contention отличается от unknown
retirement; Retired — отдельный TryLock value check, без native I/O.

### Интеграция с существующим владельцем

- Fixed namespace — process-primary-token LocalAppData/snowden-system/
  proxy-recovery/session; lock suffix создаёт existing filetxn. APPDATA/PATH,
  relative fallback и user-supplied path не используются. KnownFolderPath —
  existing x/sys wrapper, а не новый shell/COM процесс.
- ProxyManager уже зарегистрирован до native path lookup; lease регистрируется
  до directory/file/lock acquisition, затем валидируется до HKCU Open.
- readyLocked проверяет original lease перед SetProxy/prewrite/readback phase,
  ValidateAppliedProxy и WinINET notification; Restore проверяет его перед
  восстановлением. Порядок native retirement: original HKEY → writer lease.
  Неизвестный Close сохраняет весь ProxyManager/domain и запрещает replacement.
- Partial Snapshot без HKEY закрывает только известные собственные ресурсы.
  Known conflict по-прежнему не пишет registry/WinINET. Private opaque
  foreign-preservation receipt содержит original lease и принимает исключение
  cleanup только при том же object и его confirmed retirement вместе с HKEY.
  Read-only UI snapshot, matching bytes или port не дают эту квитанцию.
- Local ErrProxyWriterBusy не выдаётся за registry conflict. Он и proxy state
  uncertainty прекращают перебор transport candidates, finite lifecycle text
  объясняет локальный ресурс. Existing bounded fail-closed retry может позже
  повторить попытку; permanent busy latch и новая retry policy не добавлялись.

Resource serialization остаётся в original proxy/lease domains; Manager/App/
epoch publication locks не приобретаются внутри new filesystem owner. Noop/
injected proxy contract и native forwarding admission не заменены новым API.
Public Wails/frontend bindings и UI не менялись.

### Что ещё не реализовано

Это **same-storage-domain cooperative coordination**, не весь W3b. Другой
profile/root/relocation, legacy/external scripts, MDM и uncooperative writers
могут не пользоваться этим namespace. Сам file lock не удостоверяет HKCU
owner, process death, registry ABA/CAS, историю writes или право crash restore.
Нет authenticated durable receipt/DPAPI sealing, write-intent-before-mutation,
readback commit/tombstone, recovery/migration или startup/manual repair.
HealStaleSystemProxy продолжает возвращать ErrProxyReceiptUnavailable.

Future receipt store должен работать под retained original lease без recursive
WithWriter/Store acquisition того же domain. Persistent tuples могут содержать
PAC credentials: их нельзя логировать/публиковать/сохранять plaintext. Нужны
fixed purpose/schema/SID/owner/session/generation bindings, bounded encrypted
storage и state-before-write/readback/conflict/partial outcome. Возможность
взять освобождённый file lock не является доказательством смерти процесса.

NETGUARD-001 остаётся запросом: root tools/установленные copies/tasks/DNS/DoH
не менялись. Единый protocol и migration требуют своего owner scope; наличие
Windows foundation не разрешает исправлять чужие installed artifacts.

### Доказательство и ограничения

7Go5docs/12 originals(10existing/2new); manual5original diffs+2new files,
16named/1cached source unchanged, touched gofmt7+1+3 exit0. Tests/fixtures/build/
vet/types/product parser/SDK execution/browser/native/registry/admin/network/
real env/log/vault/other roles/chats/subagents не выполнялись/не менялись.
Реальных LocalAppData/lock/registry значений agent tools не открывали.

Actual Win32/ACL/path/reparse/hardlink/FileID/share/LockFileEx/Close, profile
redirection/mixed clients/long native call/abnormal unwind/cancellation/RSS/
whole-process retirement и runtime acceptance не доказаны. KnownFolderPath
wrapper/allocator также не получил отдельного original native heap proof.
Source/gofmt не дают forced deadline, filesystem durability, W3b recovery,
protection/egress/W4c или release proof.100OPEN56NOT_RUN/fullPC00…09/old9,
TUN gatefalse/schema2 unsupported/native/SDK/OS/wholeUX/release OPEN.

Следующий срез — actual authenticated bounded receipt/state machine под этим
Windows owner, с сохранением внешней NETGUARD migration boundary.

<a id="pc-v368-proxy-journal"></a>

## V2-368 — private DPAPI journal перед proxy mutation

2026-11-10, **partial source/unverified**, PC-01 W3b. V367 predecessor сохранён;
предыдущий короткий ответ о процентах — status-only, не progress. Этот срез меняет
production source. Полный PC-00…09 и прежние девять этапов остаются active.

### Формат и конфиденциальность

- Новый proxy_receipt_codec_windows.go: fixed binary purpose/schema, phase,
  prepared index, confirmed-written mask, generation, primary SID (8…68 bytes),
  process PID/creation FILETIME, random16 nonce; original/current/target tuples.
  Exact presence/type/raw bytes, existing canonical registry validator, exact
  EOF; bounded cursor до allocation, максимум192KiB plaintext/256KiB ciphertext.
  Generation overflow отвергается, new nonce отделяет новую session generation.
  Незатронутый written bit не разрешает отличный от original confirmed current.
- proxy_receipt_crypto_windows.go использует existing x/sys typed current-user
  CryptProtectData/UnprotectData, UI_FORBIDDEN, fixed public purpose entropy.
  Purpose не secret. Original input/blobs/output retained до API; native output
  size проверяется до Go copy, original LocalFree один раз с проверкой результата.
  Unknown API/free или partial output при API failure удерживает allocation и
  весь исходный owner; replacement/повторный free/crypto fallback отсутствуют.
- proxy_receipt_store_windows.go: fixed private receipt.v1.bin рядом с original
  storage namespace; existing trusted BoundedReadSlot/BoundedWriteSlot, без
  recursive WithWriter. На диск идут DPAPI bytes + fixed outer header; tuple/PAC
  не попадают в DTO/log/JSON/публичные файлы. Decrypted decode owns bounded copies;
  temporary plaintext очищается лишь после confirmed crypto retirement. Это не
  secure-zeroization или hard-RSS proof; native allocator может transiently
  выделить больше проверяемого Go output limit.

### Подключённые production переходы

1. Store удерживается до SID/process/random/file/crypto calls, под original lease
   и существующими proxyOwnerMu→ProxyManager.mu. Actual primary SID через existing
   bounded fileacl.CurrentUser; process creation через original process pseudo
   handle/GetProcessTimes, без SID.String/native conversion allocation.
2. Prior receipt отсутствует только при known missing read + original retirement.
   Existing file принимается только после DPAPI, exact codec, same SID и finished
   closed/foreign phase. Corrupt/unresolved active/prepared не удаляется,
   не tombstone-ится и не усыновляется новым snapshot. Failed load с known-retired
   resources закрывает пустой owner/lease, сохраняя старый файл; unknown — retain.
3. Snapshot сохраняет initial active record до SetProxy. Каждая apply/restore
   mutation сохраняет prepared original/current/target/index перед setter.
   После disk I/O повторяются lease validation и полный tuple read, чтобы не
   раздувать окно между fence и setter. Это не atomic registry CAS/ABA proof.
4. Known pre-setter отказ отменяет только prepared intent; foreign tuple не
   становится our current. Если native read не retired — pending сохраняется.
   Успешный setter получает exact whole-tuple readback и encrypted active
   confirmation до продвижения pm.current/writes. Error+exact before подтверждает
   unchanged без новой write authority. Unknown setter/readback/persist retains.
5. Restore использует тот же журнал для своих per-value writes. Final closed
   (original observed/restored либо no writes) или foreign_preserved сохраняется
   до original HKEY Close, затем lease Close. Foreign record хранит last own
   history, не удостоверяет actual foreign tuple. Opaque conflict receipt требует
   тот же closed/non-pending/non-active store и прежние original HKEY/lease facts.
6. ErrProxyReceiptUnavailable имеет finite presentation без raw data; candidate
   loop использует прежнюю state-uncertain classification. Ordinary bounded
   retry не заменён новым permanent latch. HealStaleSystemProxy остался stub.

### Что это не закрывает

DPAPI integrity/current-user sealing не доказывает свежесть, anti-rollback,
process death, current HKCU ownership или защиту от malicious same-user/admin.
Finished file разрешает новый capture, не restoration authority. Storage lease
не координирует другой profile/redirection/legacy/MDM/uncooperative writer.
Process-primary SID vs effective HKCU/impersonation, KnownFolderPath hidden free,
native DPAPI/LocalFree/ACL/FS/FlushFileBuffers/power-loss/abnormal-exit/Win32 memory
и latency/cancellation proof остаются отдельными gates. Synchronous calls не
получили forced deadline. Root tools/installed tasks/NETGUARD-001 не менялись.
Process-death observation, freshness/rollback policy, startup/manual recovery и
legacy UserMode/SYSTEM interoperability/migration ещё требуют реализации; полный
W3b не закрыт, healer не включён. W4c gatefalse/TUN unavailable/schema2 unsupported.

### Доказательство и следующий шаг

Только manual source/original diff и touched gofmt6+2+2+4+1exit0;
8Go5docs/13snap10existing3new/manual5diff+3new/15named3cached. Own receipts:
.artifacts/memory/bootstrap-v368-proxy-receipt-source-evidence.json,
manual-source-review.json/final-identities.json с тем же prefix.
Не запускались tests/fixtures edits/build/vet/typecheck/product parsers/SDK/
native/registry/browser/network; это не functional/native acceptance.
Следующий actual W3b source gap: retained original prior-process observation
(PID+creation, limited handle, no guessed PID-death), scoped admission для
unresolved journal; recovery остаётся закрытым до полного owner/freshness
протокола.100OPEN56NOT_RUN/fullPC00…09/old9/native/SDK/OS/release OPEN.

<a id="pc-v369-proxy-process"></a>

## V2-369 — наблюдение original prior process без PID-эвристики

2026-11-10, **partial source/unverified**, PC-01 W3b. V368 — progress; этот срез
подключает новый native source component к реальному unresolved journal intake.

### Production flow и ownership

1. Только после DPAPI, exact codec и same primary SID store сохраняет private
   previous receipt, затем original proxyReceiptProcess до native acquisition.
   Ни raw UI PID, ни loopback port, ни process scan не являются input authority.
2. Один OpenProcess с PROCESS_QUERY_LIMITED_INFORMATION|SYNCHRONIZE и inherit=false.
   Нет terminate/write/debug privilege/duplicate/helper/polling/переоткрытия PID.
   Original handle проверяется existing nativelease.ValidHandleValue; это
   representation check, не attestation процесса. GetProcessId и creation FILETIME
   сравниваются с decoded receipt до дальнейших conclusions.
3. Creation mismatch — отдельный reused-PID refusal; состояние replacement не
   выдаётся за old owner death. Same-creation process требует original TOKEN_QUERY
   token и bounded fileacl.UserSID, exact equality с actual current primary user.
   Current process SID ранее уже сопоставлен с receipt SID. Helpers reused,
   SID string/native allocation conversion и новая identity implementation нет.
4. Один WaitForSingleObject(original handle,0): timeout — observed running;
   signaled — повторный GetProcessTimes на том же held handle, same creation и
   nonzero exit time. Только эта ветка даёт observed-exited classification.
   GetExitCodeProcess/STILL_ACTIVE не используются. Unexpected wait/query/native
   errors остаются unresolved, без восстановления настроек.
5. Ordinary returned query/open error с known outputs закрывает original token,
   затем original process через existing spawnprep.CloseOnce на исходных slots.
   Ни failure OpenProcess (включая missing/access denied), ни zero/partial output
   не интерпретируются как смерть. Unknown call/partial contradictory output/
   Close удерживает object/store/lease; callback payload не логируется, второй
   Close или замена original observer не разрешены. Retired — отдельный value
   receipt; ordinary observation failure не равна resource uncertainty.
6. Store.open/resourcesRetired/write/ready учитывают original prior observer.
   Previous record не становится new active session; write запрещён при previous.
   Known-retired отказ позволяет закрыть пустой ProxyManager/lease без tombstone
   чужого unresolved file. Unknown retirement удерживает прежний pending barrier.
7. Common finite errors и existing lifecycle text различают observed running,
   creation mismatch, unavailable observation и exited/recovery-required. Нет
   raw SID/PID/tuple/public DTO. Cleanup uncertainty сохраняет более строгую
   прежнюю presentation/classification; candidate/retry policy не подменена.

### Границы выводов и actual remaining work

Это source наблюдения, не native acceptance и не процессная capability из PID.
Running — момент вызова, не обещание дальнейшей жизни. Полностью исчезнувший/
недоступный для OpenProcess prior process остаётся unresolved; отсутствие handle
не удостоверяет death. PID+creation не решают boot/clock/freshness/rollback model
и malicious same-user/admin threat. Signaled same-subject handle подтверждает
только наблюдение process exit, не retirement Engine/WFP/Job/всех иных ресурсов,
не current HKCU ownership и не свежесть сохранённого original/current tuple.

Все four refusal branches сохраняют file и запрещают fresh apply/repair. Actual
recovery admission, independent freshness/local rollback policy, immutable
journal-subject revalidation, per-value partial restore/recovery record и legacy
UserMode/SYSTEM/installed tasks interoperability всё ещё требуют реализации.
HealStaleSystemProxy не менялся и остаётся unavailable. Root NETGUARD-001/other
roles/chats untouched; full PC-00…09 и прежние девять этапов не сокращены.

### Доказательство и следующий шаг

Только manual source и original3 Go diffs+1new, touched gofmt4+2exit0;
4Go5docs/9snap8existing1new/16named3cached. Own source-evidence/manual-source-review/
final-identities с prefix .artifacts/memory/bootstrap-v369-proxy-process-.
Tests/fixtures edits/build/vet/types/product parser/SDK/native/registry/browser/
network не выполнялись. Win32 access/token/exit/Close/abnormal paths/latency/
forced deadline/physical resources не проверены; bounded state не hard OS bound.
Далее actual scoped recovery code и freshness/local rollback contract; не включать
healer только из-за exit observation.100OPEN56NOT_RUN/fullPC00…09/old9/gatesfalse/
TUN unavailable/schema2 unsupported/native/SDK/OS/W3b/W4c/release OPEN.

<a id="pc-v370-proxy-subject"></a>

## V2-370 — original immutable recovery subject и local consistency

2026-11-10, **partial source/unverified**, PC-01 W3b. V369 — progress;
полный PC-00…09 и прежние девять этапов сохраняются.

### Reuse существующего reader

BoundedReadSlot.ReadSnapshot возвращает bytes + os.FileInfo из тех же двух Stat
и original bounded read; значения выходят только после checked original Close.
Read(path,limit) сохранил два результата и делегирует тому же пути. Внутренняя
readSnapshotLocked вызывается лишь под original slot mutex, cleanup один раз
в caller; normal-return witness отличает ordinary missing/error от panic/Goexit.
Failed/unknown Close обнуляет оба результата и сохраняет прежний poisoned slot.

Windows FileInfo — existing private windowsReadInfo, eager original-handle
VolumeSerial/FileIndex, size/mtime; compare только existing SameWindowsFile,
без os.SameFile с неподдерживаемым типом, повторного Stat(path) или новой Win32
identity реализации. Portable/selected paths сохраняют свой existing policy;
snapshot не превращает selected input в trusted store. Это общий utility source
change, не подтверждённая регрессией совместимость всех его consumers.

### Реальный intake и неизменный subject

1. Proxy receipt store теперь получает actual original WindowsWriterLease из
   ProxyManager. Native Validate перед original file read, до crypto/write и
   снова перед ciphertext publication; это не recursive WithWriter.
2. После exact DPAPI/codec/SID unfinished receipt создаётся private subject:
   same store, lease, reader/writer slots и original prior-process observer,
   fixed path, original same-read FileInfo, complete ciphertext SHA256,
   canonical plaintext SHA256, exact nonce/generation и previous-record pointer.
   Digest не trust proof сам по себе: authentication предшествует capture.
3. Process observation использует прежний original query handle. Лишь после
   known observer retirement subject выполняет revalidation: original pointer/
   subject checks, lease Validate, same retained reader slot ReadSnapshot,
   original FileID/size/mtime/full cipher digest, canonical decoded subject
   re-encoding/hash и ещё lease Validate. Неизвестный original process/Close
   не прикрывается следующим file I/O или положительным snapshot.
4. Same name с другим original FileID, изменённые bytes/nonce/generation/tuple,
   switched store/slots/lease/observer, missing/unreadable source или abnormal
   verification — failure. Result/failure sticky; позже matching bytes не
   сбрасывают original subject. Process refusal не возвращается как актуальная
   классификация, если его исходная квитанция уже не подтверждена.
5. subjectConflict — отдельный logical hold. Даже при known-retired reader/
   process resources empty-owner finish сохраняет original lease/store/subject;
   не пишет tombstone и не разрешает fresh capture/replacement. Это deliberate
   admission hold, не утверждение о native зависании или утечке. Existing
   cleanup uncertainty остаётся более строгой причиной; автоматического сброса
   latch нет. Новый finite lifecycle text не раскрывает SID/PID/digests/tuple.

### Незакрытые свойства и следующий actual срез

Same-file/hash recheck удостоверяет лишь unchanged локальное наблюдение между
этими вызовами. Это **не durable antirollback floor, global/latest freshness,
atomic filesystem/registry CAS или crash-recovery permission**. FileInfo не
держит native custody после Close; FileID reuse/ABA, clock/boot, same-user/admin,
trusted legacy/MDM и power-loss/restore образов не решаются одним SHA256/DPAPI.
Initial absent/finished-record intake и own active journal writes сохраняют
existing cooperative-domain assumptions; новый subject относится именно к
unresolved recovery intake. Не объявлять весь persistent source protocol готовым.

Scoped recovery admission policy, durable rollback model, per-value partial
recovery/finalization и legacy UserMode/SYSTEM/installed-task migration требуют
реализации. HealStaleSystemProxy остаётся unavailable. Root NETGUARD-001/other
roles не менялись; W4c false/TUN unavailable/schema2 unsupported.

Только manual5 original Go diffs+1new,15named0cached/touched gofmt6+2+2exit0;
6Go5docs/11snap10existing1new. Own source-evidence/manual-source-review/final-
identities с prefix .artifacts/memory/bootstrap-v370-proxy-subject-.
Initial patch context был отклонён атомарно до изменений и исправлен; не тест.
Tests/fixtures edits/build/vet/typecheck/product parser/SDK/native/registry/
browser/network не выполнялись. Reader/consumer compatibility, Win32/native/
FS/ACL/sharing/Close/abnormal/latency/RSS/power-loss не проверены.
Далее actual recovery state machine и explicit admission/freshness policy,
сохранить fail-closed до наличия authority; не включать healer по local subject.
100OPEN56NOT_RUN/fullPC00…09/old9/native/SDK/OS/W3b/W4c/release OPEN.

<a id="pc-v371-proxy-anchor"></a>

## V2-371 — independent registry anchor и двухфазная запись журнала

2026-11-10, **partial source/unverified**, PC-01 W3b. V370 — progress;
full PC-00…09/old9 и все прежние acceptance obligations сохранены.

### Контракт независимой записи

- Fixed private HKCU Software/SnowdenSystem/ProxyRecovery, value AnchorV1.
  Только hash/nonce/generation/phase/SID/path/counter, без endpoint/credential/PAC
  tuple. Anchor DPAPI имеет отдельную fixed purpose entropy/magic, plaintext
  limit512/cipher4096; existing retained crypto/checked one LocalFree reused.
  Receipt zero-value crypto purpose/192KiB+256KiB limits сохранены.
- Binary codec exact-length/EOF, bounded SID8…68, path digest, monotonic local
  sequence, committed/target subjects. Odd sequence только pending, even только
  committed; counter overflow rejected. Preparation/commit нельзя пропустить,
  reset sequence/change SID/path/committed member через ordinary writer запрещён.
  New session nonce/gen1 разрешён только после finished prior phase; own session
  gen+1/active→prepared→active и active→closed/foreign сохраняют порядок.
- Original fixed-purpose HKEY retained до Open/Create/query/Set/flush. Existing
  bounded proxyRegistryQuery record reused: two fixed4KiB queries, exact REG_BINARY,
  no allocating getValue retry. Create output/disposition in original owner;
  only newly-created disposition accepted for both-absent enrollment. No delete,
  truncation, replacement of unknown callback, helper or network I/O.

### Production integration

1. Store opens anchor under same retained Windows lease before file read. File
   DPAPI/codec/SID/path/nonce/generation/phase/cipher must match committed anchor.
   Older valid receipt with newer retained anchor rejected. Existing receipt
   without anchor requires explicit migration; existing key without valid value,
   missing file with retained anchor, corrupt/foreign path/SID are not fresh state.
   Existing pending pair is recovery-required and is not silently normalized.
2. Initial both-absent allows only this fresh store to create an anchor before
   its first initial active receipt, hence before any system-proxy mutation.
   A concurrently opened-existing key cannot overwrite another namespace's state.
   Both-absent is an explicit local enrollment assumption, not proof against
   deletion/restore of the complete user profile or hive.
3. Before each actual receipt update, same original reader checks prior file
   FileID/size/mtime/full cipher digest against committed state. After anchor
   preparation it checks again before replacement. No silent overwrite of a
   changed private receipt; existing reader/lease/known retirement gates reused.
4. Anchor prepare: checked state transition, original current-value byte fence,
   purpose-separated encryption, original SetValueEx, exact readback, checked
   RegFlushKey, repeated readback. Then bounded encrypted file publication and
   known-retired reader capture of exact published bytes/identity. Then checked
   anchor commit+readback+flush. Only afterward store.record/fileInfo/current
   advance. Apply/partial restore/tombstone use this same writer.
5. Failure after preparation leaves explicit old+target pending obligation.
   Unknown native/file/crypto/Close retains original graph, no second write/Close
   used to erase uncertainty. Known mismatched source is a separate logical hold.
   operationsReturned replaces misleading resourcesRetired: transient reader/
   crypto/process calls may have returned while original anchor HKEY stays held.
6. Unresolved subject binds actual anchor pointer/sequence/full anchor ciphertext,
   in addition to prior file/slots/lease/observer; original anchor checks surround
   source revalidation. None of these observations grants registry repair.
7. Final receipt + anchor commit precedes checked anchor HKEY Close, then the
   existing system-proxy HKEY Close and storage lease Close. Store.closed implies
   known original anchor closure; existing opaque foreign receipt still requires
   this same closed store/retired native system HKEY/lease. Failed read-only intake
   closes known resources without changing old anchor/file; conflict hold remains.
8. Finite UI reasons distinguish unsupported legacy migration, pending write and
   missing/inconsistent anchor; no SID/path/digest/tuple details in DTO/log text.
   Existing candidate/ordinary bounded retry and stronger cleanup classification
   retained. Root scripts/tasks/NETGUARD-001 and other owners not modified.

### Что ещё не принято и не реализовано

Source protocol detects an old/missing file relative to an independently retained
anchor. It is **not TPM/remote monotonic storage or protection against a correlated
rollback/deletion of both file and hive**, malicious same-user/admin or arbitrary
registry/FS aliases. Registry ACL/default inheritance/link/rename/effective HKCU
vs primary SID and native process/OS enforcement remain separate gates. No claim
that every accepted blob is globally latest, or that read fences are atomic CAS.

RegFlushKey is synchronous and potentially expensive. No forced deadline/SLO,
startup/Stop performance, power-loss or physical storage ordering proof exists.
The implementation requests checked flush/readback, not a proved hardware or
cross-resource transaction. Cache/NTFS/registry/hive/clock/boot acceptance OPEN.

Actual remaining code: pending-anchor/file reconciliation, explicit legacy
enrollment/migration, per-value partial crash/startup/manual recovery and external
UserMode/SYSTEM interoperability. Interrupted first enrollment can leave an empty
private key; it is deliberately not reset on restart. Existing V368…370 receipt
without anchor is unsupported until migration, not silently accepted as current.
HealStaleSystemProxy remains unavailable; no automatic system repair enabled.

Only manual6 existing Go diffs+3new,14named5cached/touched gofmt9+2+4exit0;
9Go5docs/14snap11existing3new. Own source-evidence/manual-source-review/final-
identities prefix .artifacts/memory/bootstrap-v371-proxy-anchor-.
No tests/fixtures/build/vet/types/product parser/SDK/native/registry/browser/network
execution. Compile/runtime/native/crypto/ACL/FS/flush/Close/latency/resource bounds
and full W3b/W4c/release unverified. Next actual pending reconciliation/recovery
protocol under same original lease, without treating anchor/process facts as
sufficient permission to change Internet Settings.100OPEN56NOT_RUN/fullPC00…09/
old9/gatesfalse/TUN unavailable/schema2 unsupported/native/SDK/OS/release OPEN.

<a id="pc-v372-proxy-reconcile"></a>

## V2-372 — pending pair: завершение metadata для exact target

2026-11-10, **partial source/unverified**, PC-01 W3b; V371 — progress.

- Actual admitAnchoredFile вызывает one-attempt proxyAnchorReconciliation после
  file DPAPI/codec/SID и anchor SID/path проверки. Original object retained до I/O;
  bound store/lease/anchor/reader/path, original sequence/cipher/committed+target,
  full file subject/cipher/canonical record digest и same-read FileInfo.
- Exact target branch проверяет original bindings, lease, current anchor bytes,
  trusted bounded file read + original FileID/size/mtime/hash/canonical subject;
  затем повторяет anchor/lease checks. Existing commitAnchor/publish завершает
  только прежний prepared→committed шаг (counter+1, checked readback/flush).
  После commit вновь проверяются source и exact committed anchor. Файл не
  переписывается, Internet Settings HKEY не открывается этим компонентом.
- Только после completed target metadata admission продолжает обычный путь:
  unfinished record всё ещё проходит prior-process refusal/restore gates,
  finished record разрешает fresh native capture, не восстановление settings.
  Metadata completion не создаёт proxyRestoreReceipt или forwarding permission.
- Committed old file — отдельный refusal: наличие old bytes не доказывает,
  что target никогда не публиковалась. Pending pair не отменяется и не теряется.
  Neither/missing known/source mismatch удерживает original logical owner/lease.
  Pending first enrollment + observed missing file — отдельный finite refusal,
  без reset/re-enrollment/delete. Existing empty key без валидного anchor payload
  по-прежнему unsupported; текущий срез не закрывает interrupted setup целиком.
- returned/committed/completed разделены. Store pending/operationsReturned
  учитывают original reconciliation return; panic/Goexit/unknown native/Close
  не прикрываются новым object или matching bytes. Ordinary old/initial-missing
  refusal закрывает known read-only resources, сохраняя данные перехода.
- Новые finite UI reasons не выводят raw PID/SID/path/hash/PAC. Source-only:
  manual4 existing Go diffs+1new/16named0cached/gofmt5+1+1exit0;5Go5docs/10snap.
  Own source-evidence/manual-source-review/final-identities prefix:
  .artifacts/memory/bootstrap-v372-proxy-reconcile-.

**Остаток:** old/missing pending resolution с явной authority, empty-key/legacy
migration, per-value crash/startup/manual proxy repair и UserMode/SYSTEM protocol.
HealStaleSystemProxy остаётся unavailable. Факты anchor/file/process не заменяют
restore authority; correlated hive/profile rollback и native ACL/FS/flush/Close/
latency/OS/physical resources не приняты. Tests/fixtures/build/vet/types/product
parser/SDK/native/registry/browser/network не выполнялись. FullPC00…09/old9/
100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported/release OPEN.

<a id="pc-v373-proxy-stage"></a>

## V2-373 — parent-bound encrypted staging intent и journal replay

2026-11-10, **partial source/unverified**, PC-01 W3b; V372 — progress.

### Запись

- Fixed sibling receipt.v1.stage.bin, encrypted under a distinct fixed DPAPI
  purpose. Closed internal domain enum keeps existing receipt/anchor purposes
  and bounds; stage plain278528/cipher294912 bytes. Exact bounded binary codec:
  SID/path, parent even counter/committed nonce-generation-phase-hash, original
  parent FileID/volume/size/last-write (or explicit absence), exact target cipher.
  Existing immutable reader snapshots/MatchesWindowsFileID reused, no new native
  file identity implementation. Canonical absent fields, lengths/EOF, counter
  headroom and phase/gen transitions checked before trusting the intent.
- Normal store.write persists/reads back staging before anchor prepare. Before
  stage overwrite, it requires a known completed predecessor matching current
  committed anchor/counter/SID/path, and repeated original stage identity/hash
  checks. Orphan/unknown intent is preserved. No delete, unbounded backup history
  or recursive WithWriter. Main source still checked before preparation/replacement.
- Existing bounded reader/writer slots and checked DPAPI/free retained for stage
  lifetime. Success requires known original retirement; unreturned call/Close
  retains stage/store/lease. Store.ready/pending/operationsReturned account for
  stage and original replay return. New finite UI text makes no false claim that
  a failed operation left every file unchanged; no raw config/PAC/SID/hash output.

### Реальный intake/replay

| Наблюдение | Source action |
|---|---|
| Exact canonical target + pending anchor | Existing V372 target metadata completion remains; no redundant rewrite |
| Stage parent matches pending anchor, main old/missing | Recreate/publish exact staged target, then finish anchor commit |
| Stage parent matches committed anchor, preparation not recorded | Replay recorded private intent through existing prepare/file/commit |
| Committed anchor matches stage target, main missing/known parent | Restore journal from committed encrypted backup, without lowering counter |
| Both canonical and anchor absent, valid parent-zero stage intent | Initial private journal enrollment/replay; active old record still hits normal gates |
| Completed matching stage + current canonical target | Keep as completed predecessor; no replay |
| Unknown source, wrong identity/SID/path/counter/hash, corrupt data | Refuse/preserve; no blind overwrite/reset |

One retained replayer binds original store/lease/reader/writer/anchor/stage and
before state/cipher. Source and target receipt MAC/SID/canonical data, stage
record/hash and old file identity are checked before and after native steps.
Missing destination uses original WriteNew, never truncation if a file appears.
Known exact target is reused. Actual publication/readback/anchor commit and
final file/stage/lease checks precede completed. At most one replay per intake;
no new helper, process, polling, network or replacement of uncertain ownership.

### Ограничения и остаток

Only the private journal is replayed. No Internet Settings key/value or process
death heuristic is used by this component; unfinished records still pass existing
prior-process/restore refusals. Replay does not create proxyRestoreReceipt or
forwarding authority. No-write record retirement and per-value crash/startup/
manual system repair remain actual code gaps. Empty anchor key without valid
payload, unanchored legacy receipt and mixed old writers still need migration.
Without valid stage the earlier old/missing refusals remain. Corrupt/unrelated
canonical data is not overwritten using a vaguely matching backup.

File/anchor/stage observations are not atomic OS CAS or global freshness. Full
profile/hive/stage correlated rollback/deletion, malicious same-user/admin, ACL/
links/Win32/FS/DPAPI/Close/power-loss and latency/RSS remain unverified. Older
writers that ignore staging can leave a stale sidecar; it is refused, not silently
deleted. Stage addition increases encrypted disk/crypto work; no speed/SLO claim.
Root NETGUARD-001/other roles untouched; HealStale unavailable.

Only manual6 existing Go diffs+3new/16named0cached and touched
gofmt9+1+1+1+1+2exit0;9Go5docs/14snap11existing3new. Own source-evidence/manual-
source-review/final-identities prefix .artifacts/memory/bootstrap-v373-proxy-stage-.
Two alignment-context patches were rejected atomically and corrected; not tests.
No tests/fixtures/build/vet/types/product parser/SDK/native/registry/browser/network.
FullPC00…09/old9/100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported/
native/OS/W3b/W4c/release OPEN. Next no-write metadata retirement and actual scoped
system recovery, retaining all migration/native/legacy obligations.

<a id="pc-v374-proxy-no-write"></a>

## V2-374 — private retirement при отсутствии незавершённых own writes

2026-11-10, **partial source/unverified**, PC-01 W3b. V373 implemented/finalized915;
previous user-facing percentage response was status-only, not development progress.
Current source was revalidated and this turn adds the actual retirement path.

### Admission and publication

- Only a valid authenticated anchored record in active phase, index255/written0,
  exact original=current=target, generation headroom and committed anchor with
  room for two increments is eligible. Prepared/written0 is explicitly excluded:
  its setter may have run without durable confirmation. Written0 does not mean
  that no setter was ever attempted; it means no confirmed write is outstanding
  and the durable record has no pending setter intent.
- Original retirement owner is stored before I/O. It binds actual store, lease,
  reader/writer/anchor/stage, immutable source file identity/bytes/plain digest,
  initial anchor/counter/cipher, old stage presence/identity/hash, old record and
  independently generated fresh session identity. It cannot be replaced after
  unknown return. Normal open/write/ready/finish lifetime accounts for its return.
- Extracted the existing stage/anchor/file publisher rather than adding another
  crypto/native writer. Ordinary calls retain opened-session admission. The other
  caller must present the exact attempted original retirement owner and exact
  no-write→closed target. Generic pending state does not grant this entry.
- Target preserves old SID/nonce/PID/creation and every original/current/target
  value byte, increments generation once and sets closed. Exclusive cooperative
  lease plus canonical file/source, stage and original anchor checks surround
  publication. Existing prepared stage, known readback, anchor commit/flush are
  reused. Exact borrowed input and original source owner remain retained on error.
- Post-publication verification checks original output FileID/bytes/plain digest,
  same session/generation, anchor counter+2 and committed target, matching staged
  parent/file/target and repeated stage/anchor/lease checks. Only then can open
  permit the existing fresh snapshot. The old record is never adopted into the
  new session identity; the fresh nonce/PID remain untouched throughout.

### Scope and remaining work

This component never opens/queries/writes Internet Settings or issues a public
restore receipt. It does not observe the prior process or infer its death from
the free lease. It closes only private metadata when no pending setter or own
confirmed write requires restoration. Current OS proxy may have changed externally;
the subsequent ordinary Snapshot must capture it afresh. Lease protection covers
cooperating writers in this storage domain, not old clients/other VPNs/MDM.

Error, source conflict or unknown publication keeps original owner/input/lease
and denies successor. A crash during this metadata transition uses V373 staged
replay; source/target corruption and unrelated files are still preserved/refused.
Prepared and written records keep the existing unresolved prior-process/source
gates. Per-value crash/startup/manual system recovery remains unfinished, as do
empty anchor key enrollment, unanchored legacy and mixed-writer migration.
HealStale remains unavailable. Full correlated hive/profile/stage rollback and
native ACL/FS/DPAPI/Close/flush/power-loss/latency/resources/OS proof remain OPEN.

4Go5docs/9snap7existing2new; manual2existing Go diffs+2new,16named0cached,
touched gofmt4+3+1exit0. Own receipts prefix
.artifacts/memory/bootstrap-v374-proxy-no-write-. No tests/fixtures/build/vet/types/
product parser/SDK/native/registry/browser/network. Root NETGUARD-001 and other
roles unchanged; fullPC00…09/old9/100OPEN56NOT_RUN/gatesfalse/TUN unavailable/
schema2 unsupported/release OPEN.

<a id="pc-v375-proxy-recovery"></a>

## V2-375 — original-exit scoped per-value system recovery

2026-11-10, **partial source/unverified**, PC-01 W3b; V374 — source progress.

### Admission, ownership and persistence

- Actual open now offers a private recovery transaction only after the original
  observer verified PID+creation, primary SID, signaled original process handle,
  nonzero exit time and checked token/process Close. Its retained authenticated
  file subject is revalidated. Missing/unopenable/reused PID, running/unknown
  lifetime, free storage lease or matching port remain insufficient. No new
  native process API, retry, helper or process termination is introduced.
- Reserve one recovery owner before key I/O; bind exact ProxyManager/global owner,
  store/lease/reader/writer/anchor/stage, immutable initial subject/process proof,
  current receipt/file and separate fresh session. Store lifetime tracks unknown
  operations. Original existing registry HKEY/read/setter/notification/Close owners
  are reused. Three indexed setter slots and one notification are retained, never
  replaced after unknown return. Generation reserves8 and anchor16 increments.
- Shared private publisher requires the exact attempted recovery owner, retained
  next record and same session/original values, consecutive generation and
  monotonic written mask. Source and post-publication file identity/bytes/plain
  digest, staged parent/target, anchor sequence/hash and lease checks repeat.
  Exact writer input and intended/output records survive uncertain publication.
- Nonce/SID/original tuple remain the old session. Before any new native setter,
  prepared metadata records the current recovery PID/creation. A crash must not
  attribute a new pending setter only to the already exited predecessor. This is
  a writer handoff within recovery, not adoption into the fresh normal session;
  fresh nonce/PID record stays untouched for subsequent ordinary Snapshot.

### Actual system settings path

| Observed whole tuple | Action |
|---|---|
| Prepared current | Resolve pending metadata without granting a new confirmed write |
| Prepared target | Record confirmed target/written bit, then restore owned values |
| Active current | Restore only differing fields whose written bit is recorded |
| Active original | Skip setters; finish original-state confirmation/notification |
| Supported foreign tuple or completed double-read conflict | Preserve every current value; cancel only pending authority/history, then publish foreign outcome |
| Unknown query/setter/readback/source/Close | Retain original owner/intent; deny successor |

Restoration order is enable→PAC→server. Each real setter requires persisted
prepared intent, fresh file/stage/anchor/lease checks and whole-tuple fence after
journal I/O; exact presence/type/bytes use the existing setter. Native successful
return plus exact full target readback precedes active confirmation. Failed setter
with exact unchanged readback cancels only that intent and retains the unfinished
recovery with its ordinary error. Other ambiguous outcomes keep prepared intent.
Foreign tuple is never adopted as our current/source or overwritten. Cancelling
pending authority on foreign observation does not assert that no past setter ran.

Final original observation and existing WinINET notification precede closed
metadata, final source check and checked original HKEY Close. Only completed
restoration allows fresh ordinary Snapshot. Foreign outcome stops this Start;
cleanup carries the actual recovery HKEY Close witness and preserves the existing
opaque foreign receipt flow. Unused fresh key reservation remains retained; no
borrowed/unopened numeric alias fabricates original-handle retirement.

### Remaining limits

Automatic recovery is deliberately unavailable when the prior process cannot be
opened/observed, including many fully gone processes. No death is inferred from
absence. Explicit manual takeover and resuming a failed live recovery owner are
not implemented; ordinary write/notification errors can retain the transaction
until that path exists. Restarted recovery uses the persisted responsible writer
but still needs original-exit admission. HealStale API remains unavailable until
broader migration/admission is complete. Empty anchor-key enrollment, unanchored
legacy, old writers/UserMode/SYSTEM/root NETGUARD-001 integration remain OPEN.

No atomic HKCU CAS, global antirollback, protection from uncooperative same-user/
admin/MDM or correlated hive/profile/stage rollback is claimed. Filesystem/ACL/
links/DPAPI/Close/flush/power loss/resource/latency and OS/forwarding/release need
their own future acceptance. Additional synchronous journal/flush work has no
measured performance claim. TUN and schema2 gates remain false/unsupported.

9Go5docs/14snap11existing3new; manual6 existing Go diffs+3new/18named0cached,
touched gofmt9+3+1exit0. Source-only receipts prefix
.artifacts/memory/bootstrap-v375-proxy-recovery-. Two guessed read filenames were
absent; enumerated actual native files before using them. No product execution,
tests/fixtures/build/vet/types/parser/SDK/native/registry/browser/network. Other
roles unchanged/fullPC00…09/old9/100OPEN56NOT_RUN/gatesfalse/release OPEN.

<a id="pc-v376-proxy-consent"></a>

## V2-376 — core manual takeover с одноразовым source-bound подтверждением

2026-11-10, **partial core source/unverified**, PC-01 W3b; V375 — source progress.

### API и source lifetime

- ProxyManager.PrepareProxyRecovery(ctx) reuses the exact existing global/PM
  locks, Snapshot reservation, primary-user storage namespace and original
  intake. Automatic system recovery is disabled for this preparation; private
  journal reconciliation/no-write retirement may still complete. Preview never
  opens or mutates Internet Settings. A current active snapshot, observed running
  prior process, nonretired native owner, source conflict or unknown Close refuses.
- One retained private offer owns original PM/store/subject, immutable raw/file/
  plaintext receipt, stage presence/identity/hash, anchor state/hash, original
  process observation, previous semantic failure and never-attempted automatic
  candidate. Fresh32B random token is compared only after exact length/bounded
  hex decoding, using constant-time bytes. Fixed scope is recorded-values restore;
  the wire DTO omits PAC/registry bytes, paths, SID/PID and file hashes.
- Confirmation checks monotonic2min start deadline before and after source I/O.
  Queue/verification wait does not extend it. The retained deadline is private;
  its UTC wire string is presentation only. Correct-token confirmation has one
  attempt even after expiry/source failure. Wrong token/scope cannot substitute
  another offer. Only an exact revalidated original source permits consumption.
- Known semantic intake refusals may be superseded by this explicit grant after
  source validation; the original error is retained. Unknown native completion,
  reader/writer/lease failure or source conflict is never cleared. A never-started
  automatic candidate can be replaced only as pure bookkeeping and remains
  retained in the offer. No acquired original resource is discarded or reopened.

### Actual confirmation/cancellation behavior

Consent is an explicit cooperative writer takeover, not process death. A
known-retired observation of an unopenable process or reused PID may be allowed;
an observed running original is refused. The existing per-value transaction,
source/stage/anchor/lease fences, original key/setter/readback/notification owners
and current-writer PID persistence are reused. New normal session stays separate.
The same PM.pending phase fence surrounds manual and automatic execution.

Caller context is checked before new key/setter/notification intents through the
existing contextError, including its operation-source/deadline checks. Cancellation
does not discard readback/durable finalization of an already returned setter.
Start TTL authorizes one transaction; it is not a hard OS deadline or an abort
mechanism for synchronous calls. Once accepted, a failed/unknown transaction is
retained and cannot be cancelled/replayed as an unused offer.

Component result is returned only after final metadata, original recovery HKEY
Close, anchor/fresh reservation closure and original storage lease retirement.
Outcome/count concern recorded proxy recovery only; Engine/WFP/TUN/whole cleanup
or protected forwarding is not attested. No automatic reconnect is added.

Unused offer cancellation is allowed after expiry and never changes proxy values.
Ordinary Stop can also mark it finished only after exact original component
cleanup; an unused token cannot indefinitely block a later Snapshot after that
known retirement. Preparation errors retire newly reserved/unused resources only
after ordinary return and existing guards; uncertain original frames remain held.
There is no new expiry timer/goroutine; expiry alone does not close a lease.

### Remaining integration and limits

These are core APIs, not Wails/App commands or a delivered UI action. App/Manager
must serialize their lifecycle, bind real current operation context, preserve
their existing native/protection owners and use finite errors. The NetGuard view
was inspected only; no frontend/generated bindings were edited or executed. UI
must keep a token per actual preview, cancel unused interest on close/navigation,
prevent stale/replayed confirmation and distinguish component result from VPN
health. App/Manager/UI integration is the next concrete slice.

Resuming an already attempted failed live recovery remains unsupported, including
known ordinary setter/notification failures; unknown callbacks must never retry.
Empty anchor key, unanchored legacy, mixed old writers/UserMode/SYSTEM/root
NETGUARD-001, correlated rollback and native/OS/durability/latency/resource/release
acceptance remain. HealStale legacy API stays unavailable. Renderer-token
possession is not independent human/process identity or protection from a malicious
same-user/admin; cooperative storage/source fences are not atomic HKCU CAS.

7Go5docs/12snap10existing2new; manual5 existing Go diffs+2new/18named0cached,
touched gofmt7+4+3exit0. Own prefix
.artifacts/memory/bootstrap-v376-proxy-consent-. No tests/fixtures/build/vet/types/
product parser/SDK/native/registry/browser/network; other roles untouched.
FullPC00…09/old9/100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported/
native/OS/release OPEN.

<a id="pc-v377-proxy-ui"></a>

## V2-377 — actual Manager/App/NetGuard manual recovery wiring

2026-11-10, **partial source/unverified**, PC-01 W3b; V376 — source progress.

### Core lifecycle and actual runtime

- Stable preview/confirmation/result DTOs moved into common core API. Optional
  native interface retains a private value-only original resource-graph check;
  arbitrary public bool/noop adapters cannot acquire manual authority. Manager
  reserves its existing original lifecycle operation for actual callbacks,
  increments generation, registers proxyActive before partial acquisition and
  never holds Manager.mu over native work. UI wait between callbacks does not
  occupy an active lifecycle slot. Stop can cancel/wait the actual callback.
- Only stopped/error Manager with no active sampler or running/unknown Engine
  enters this workflow. Actual body-return witness is required; an abnormal
  original frame retains its active owner. Component retirement clears only
  proxyActive/session after the exact native graph is known retired. Engine,
  WFP/protection/forwarding and readiness are not inferred or released.
- Stop's stopped fast path now requires !proxyActive. SetEngine/SetProbe/
  SetSystemProxy also require no active operation and no held proxy. Original
  Restore marks its matching manual session finished, so a later normal Start
  cannot revive old pending interest through a reused Manager.
- App can install a profileless Manager in the actual runtime when both Manager
  and Engine are absent; it creates no Engine/config/profile/connection. Existing
  Stop, quit and replacement retain/retire that exact actor. An orphan Engine
  is not overwritten. Native/core failures cannot replace an original owner.
- Directly exposed integration correction: replacement captures the previous
  proxy outcome before Stop and refuses a foreign conflict newly discovered by
  this Stop. Historical completed foreign outcome is not a new conflict. All
  Stop cleanup errors and existing explicit-vs-automatic App intent gates remain.
  Original corebridge bytes were captured from unchanged reviewed input before
  that narrow edit; source-scope extension receipt is retained.

### App request/callback custody

- One private App request is reserved before operationMu queue waits. RequestID,
  exact intent registration/source, actual Manager/Engine and local atomic
  maintenance lifetime are checked at dispatch. Same-context source/deadline
  contract is reused without fabricating physical path/Engine authority. Current
  operation cancellation closes future native intents; already entered callbacks
  retain their original readback/finalization.
- Early Cancel revokes local interest before waiting and retains one request-ID
  tombstone for the sole pending UI producer. A delayed Prepare cannot resurrect
  it. Duplicate request identity/confirmation and stale intent are refused.
  Negative cleanup always targets the originally captured Manager, never a newer
  runtime. Finite App errors do not serialize raw backend/PAC/path data.
- Entered Prepare/Confirm frames have original done channels and ordinary-return
  flags. Cancel holding operationMu may wait only entered predecessors' remaining
  App.mu-only defers; it never waits a queued successor that needs operationMu.
  Abnormal return remains unconfirmed. Later queued calls cannot be replaced
  while their actual original promise/frame is outstanding.
- A refused preview never triggers implicit Restore/settings mutation. Unknown
  source/Close/callback remains owned. Negative cleanup of a failed empty-token
  preparation can only acknowledge actual retired graph, not restore values.
  Foreign result sets denial-only conflict/retry fences; existing App error,
  protectionDesired, watchdog and whole-cleanup flags remain untouched.

### UI and binding delivery boundary

NetGuard reuses existing DialogFrame and design tokens; no parallel dialog or
card grid is added. Typed facade validates schema/request/intent/scope/token/time/
enum/count before publishing results, validates request interest before RPC and
resolves exports only from generated App namespace. No raw window.go bypass or
manual wailsjs edit. Missing exports/mock/browser keep action unavailable.

The component explains scope, recorded count, unconfirmed prior process, explicit
acknowledgement and remaining client-clock time; backend monotonic TTL remains
authoritative. Existing Escape/focus/close behavior and a safe first Close action
are reused. Local modern-web-guidance platform-controls-dismiss-dialog informed
withdraw-interest behavior, not a new browser API. Layout uses wrapping/minmax,
existing tokens and a narrow dialog breakpoint; no visual/keyboard/scaling claim
is made without execution.

One module-level original UI owner outlives route disposal. Pending preparation,
confirmation and cancellation promises cannot be replaced before actual return.
Close/navigation/intent change withdraws interest and requests cancellation;
late results cannot reopen a dialog. Expiry timer belongs to that owner and is
cleared on close/confirmation/disposal. A failed negative cleanup keeps the hold;
explicit checking may re-address the same original scope without retrying an
unknown native setter/Close. Cancel after dispatched confirmation does not claim
that no restoration ran. Component result never becomes VPN health or auto-start.

All routes are source-connected, but normal Wails generation is still prohibited:
the current generated namespace/executable has not acquired these exports. The
existing namespace therefore presents this feature as unavailable until an
authorized normal generation/build. No source work here proves runtime delivery.

### Remaining gates and evidence

Retained failed live recovery resume for known ordinary errors is still missing;
unknown callbacks must never retry. Empty anchor key, unanchored legacy, mixed
old writers/UserMode/SYSTEM/root NETGUARD-001, correlated rollback and full
native/OS/durability/latency/resource/UI/release acceptance remain. HealStale legacy
API stays unavailable. FullPC00…09/old9 remain the objective; TUN/schema2 admission
is not enabled. Interface callbacks, code generation, build, native or VPN path
have not been executed.

6Go5UI5docs/16snap12existing4new; manual7 existing source diffs+4new,19named0cached;
touched gofmt5+2+1+3+1exit0. Own receipts prefix
.artifacts/memory/bootstrap-v377-proxy-ui-. No tests/fixtures/build/vet/types/
product parser/SDK/native/registry/browser/network/generation. Other roles unchanged;
100OPEN56NOT_RUN/gatesfalse/release OPEN.

<a id="pc-v378-proxy-resume"></a>

## V2-378 — retained recovery continuation core

2026-11-10, **partial source/unverified**, PC-01 W3b. Предыдущий ход дал
статусную оценку без изменений; продолжение возвращено к незавершённому V378.

- Retained owner distinguishes a returned failed setter with complete unchanged
  tuple readback, a returned known notification failure, and cancellation before
  Open/setter/notification. Source/anchor/stage/publication/query/Close uncertainty
  remains sticky and cannot be reclassified as a retryable pause.
- Preparation proves exact paused owner/current record/cipher/FileID, original
  HKEY/lease/reader/writer, stage/anchor, cumulative steps and original callback
  slots. Includes the retained notification predecessor and prior grant digest.
  No temporary clearing of failure flags is used to obtain readonly permission.
- Fresh random one-shot grant has the same2min monotonic start TTL, repeated
  source/context/expiry proof immediately before consumption, and explicit resume
  fields in the common preview DTO. It grants no old-process death inference.
- Resume reuses the same native key and journal chain. Only normally returned
  unchanged failed callbacks are retired, with one bounded previous slot/value,
  notification and grant. Unknown calls are never replaced. No automatic retry.
- Cumulative generation/anchor/step counts never reset. Each grant admits at most
  eight metadata transitions, with uint64 headroom checked before approval. A
  cancellation-before-setter resolves that retained prepared intent as not run;
  later matching bytes cannot be attributed to an absent setter.
- Unused resume-grant cancellation retires only that grant. Original HKEY/lease,
  pending transaction and cleanup barrier remain. Known final journal/readback/
  HKEY Close and actual component retirement still gate success. Confirmed setter
  accounting survives continuation; original source and old writer identity remain.

Manager/App/UI do not yet offer resume: their existing session/request/pending
guards must distinguish unused-grant retirement from full resource retirement.
Current generated bindings/EXE are not updated. This source result proves neither
compilation nor native/OS correctness, bounded latency or VPN readiness.

8Go5docs/13snap11existing2new; manual6 source diffs+2new,15named0cached;
gofmt8exit0 reconfirmed after final source edits. Earlier formatting output was
clipped; the explicit final8-file exit0 is the current formatting receipt.
Own receipts: .artifacts/memory/bootstrap-v378-proxy-resume-.
No tests/fixtures/build/vet/types/parser/SDK/native/registry/browser/network/generation. Manager/App/UI resume integration still open; current generated/executable feature unavailable. Unknown native/readback/publication/Close/source conflict cannot resume. Empty-key/legacy/mixed writers/NETGUARD-001/native/OS/durability/resource/UI/release remain. HealStale unavailable; fullPC00..09/old9/100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported.

<a id="pc-v379-proxy-continuation"></a>

## V2-379 — continuation through actual Manager/App/NetGuard

2026-11-10, **partial source/unverified**, PC-01 W3b; V378 core is source progress.

- Manager adds explicit continuation preparation on its same retained session,
  existing lifecycle slot and native proxy component. The private proposal gate
  accepts only a known paused owner with no outstanding unused/unknown grant.
  Positive confirmation still requires the fresh current token/generation. An
  empty negative token addresses only the exact Manager's internally retained
  grant after a refused preview, never a Restore or a successor runtime.
- App new preparation names the exact previous request. It requires ordinary
  return of all original Prepare/Confirm/Cancel frames, binds a fresh current
  intent, and retains the previous actual Manager/Engine without creating or
  substituting a runtime actor. Native custody remains in that Manager. A
  single in-flight cancellation fence prevents a duplicate's return from
  masquerading as completion of another queued cancellation.
- Cancellation now returns finite component-scope released/transaction_held/
  blocked facts. Retiring an unused resume grant keeps the original HKEY/lease/
  transaction hold. No branch promotes Engine/WFP/protection or whole cleanup.
- A new proposal can be refused before App registration. The negative request
  carries only its new and exact previous IDs; App tombstones the new ID before
  addressing the named original, returning its actual owner ID. UI retains both
  IDs until that ordinary reply and cannot start another proposal on an unknown
  reply. There is no linked request history or arbitrary fallback owner.
- Existing facade validates resume/prior-process/reason relationships, request
  IDs, fixed scope, expiry, finite cancellation outcomes and the actual selected
  owner ID. Missing/older/malformed replies cannot grant release or continuation.
  All calls still use optional generated namespace only, with no wailsjs edits.
- NetGuard reuses the existing modal and tokens. The source reason and new
  acknowledgment distinguish continuation. Local pending promises/expiry/close/
  intent/navigation rules remain. During pending phases focus moves to the
  persistent Close control when the initiating button disappears; Escape/focus
  behavior is reused from DialogFrame. Local modern-web-guidance dismiss-dialog
  informed withdraw-interest behavior; no browser/scaling/keyboard proof is made.

3Go4UI5docs/12 snapshots, all existing; manual7 source diffs;12named0cached.
Touched gofmt3exit0. Own receipts: .artifacts/memory/bootstrap-v379-proxy-continuation-.
The cancellation argument/result DTO changed, so normal Wails regeneration and
matching backend/frontend delivery are required when execution is authorized.
Current generated files and EXE have not been updated; the feature stays unavailable.
No tests/fixtures/build/vet/types/product parser/SDK/native/registry/browser/network/generation. Existing generated bindings/EXE were not regenerated; optional feature remains unavailable there. Source/gofmt does not prove IPC/compiler/native/OS/UI/latency/resource/durability/release behavior. Empty-key enrollment, unanchored legacy, mixed old writers/UserMode/SYSTEM/root NETGUARD-001, correlated rollback, P0 W4c and fullPC00..09/old9 remain. HealStale unavailable/100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported.

<a id="pc-v380-proxy-enrollment"></a>

## V2-380 — initial empty anchor-value enrollment

2026-11-10, **partial source/unverified**, PC-01 W3b; V379 is source progress.

The CreateKey→first SetValue crash window formerly left an existing key without
AnchorV1; intake refused before reading the staged initial publication. Source
now distinguishes that known absence from a missing key and from corrupt data.

| Original anchor/source | New source behavior |
|---|---|
| Missing key+missing canonical, valid initial stage | Same replay, now additionally requires generation1/active/written0/no prepared intent and original=current=target |
| Existing original HKEY, value twice missing; same initial stage; canonical missing or exact staged target | Private metadata-only enrollment/replay through that retained HKEY |
| Existing empty value without eligible initial stage | Refuse migration; no arbitrary reset or fresh ordinary overwrite |
| Unanchored legacy/active-written/prepared or nonmatching canonical | Refuse; no restoration or adoption of settings |
| Loaded valid independent anchor | Existing sequence/prepare/commit/reconciliation contracts remain |

- Two bounded original-HKEY queries classify emptyValue. The key is never retagged
  missingKey, reopened or recreated. A private capability is the exact retained
  stage replayer, bound to original HKEY numeric handle, store/lease/reader/writer,
  stage digest/parent-zero intent, decoded initial no-write target and exact source.
  Eligibility is checked again by the anchor publisher before and after DPAPI.
- Initial replay repeats known absence around source/stage/lease reads. Existing
  file must match the exact encrypted target; no foreign bytes are overwritten.
  Existing Set/query/flush/requery and canonical publication/checked commit remain
  the actual completion path. Only then is emptyValue cleared. Unknown native or
  publication result retains original inputs/resources and cannot be retried.
- Anchor encoding/DPAPI now complete before CreateKey. Their error no longer
  creates an empty key. The final exact source query runs after encryption,
  directly before SetValue through the same original HKEY.
- This is private metadata recovery. The existing no-write retirement precedes a
  later fresh Snapshot; the old tuple is not adopted as that Snapshot and gains
  no Internet Settings setter/notification permission or process-death inference.

4Go5docs/9 snapshots8existing1new; manual3 source diffs+1new;12named0cached;
touched gofmt4+2exit0. Own receipts: .artifacts/memory/bootstrap-v380-proxy-enrollment-.
No native call, file codec, product parser or crypto code was executed by this work.
Unanchored legacy and old UserMode/SYSTEM/root writers still require separate
source migration/integration; an absent independent history cannot authorize their
settings repair. Current generated bindings/EXE were not regenerated or rebuilt.
No tests/fixtures/build/vet/types/product parser/SDK/native/registry/browser/network/generation. Initial metadata replay does not grant system-settings restoration, process-death proof, VPN readiness or adoption of historical settings. Missing initial stage, unanchored legacy, mixed old writers/UserMode/SYSTEM/root NETGUARD-001 remain; no correlated hive/profile/stage rollback defense, atomic OS CAS, key-path/ACL/alias or native durability/resource/latency proof. FullPC00..09/old9/W4c/native/OS/release remain; HealStale unavailable/100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported.

<a id="pc-v381-proxy-writers"></a>

## V2-381 — running applied-proxy observation and external-writer boundary

2026-11-10, **partial source/unverified**, PC-01 W3b; V380 is source progress.

Read-only source inspection confirms root NetGuard UserMode still disables proxy
by loopback/dead-listener inference and SYSTEM changes physical DNS/DoH. Ownership
and NETGUARD-001 require separate root-scope review; no root/installed task/script
was changed or run. This platform fix does not close that migration request.

- Existing running lifetime sampling checked local trust after terminal applied-
  proxy validation, leaving no ongoing native tuple observation. Native sessions
  now reserve exactly one proxy child within the original lifetime owner. Explicit
  custom/noop adapters do not gain native ownership or fabricated proxy proof.
- The child serially calls the existing native ProxyManager validation, through
  its original registry HKEY/current tuple and writer lease. Manager/watch/
  generation/Engine/native-owner bindings surround the callback; no Manager lock
  spans native work. A known conflict remains deny-only after cancellation. No
  registry alias, loopback inference, parallel read queue or automatic setter.
- Initial serial insertion into the trust sampler was rejected during source
  review because an uninterruptible registry call could stall trust expiry. The
  pre-scope/intermediate diff and explicit design amendment remain retained.
  Pure trust sampling stays independent. One child uses the same cancellation
  and a sequential timer; its normal/abnormal result is read only after actual
  done. Existing protection-before-engine-stop, Engine.Stop and proxy cleanup
  are attempted before waiting for the child. Parent done includes child return.
- A late original child fault is retained through the parent join and matching
  Manager error state/signal. Cancel or timeout cannot replace callback return,
  retire an HKEY or admit a successor observer. Trust-callback abnormal return is
  distinguished from local observer/cleanup failure. Pending native ownership
  remains with the original registry/file components.
- ErrProxyObservationUnavailable is local session failure, not channel health or
  retirement evidence. App preserves actual trust faults in joined reports, but
  a proxy fault alone no longer fabricates TrustUnavailable/certificate expiry.
  Finite proxy text/report and existing protection_retained/proxy_conflict/
  cleanup_unconfirmed precedence preserve explicit action and block channel churn.
  No protection release, direct fallback, forwarding admission or automatic reset.

6Go6docs/12 snapshots11existing1new; manual5 source diffs+1new;12named0cached;
touched gofmt6+2exit0. Own receipts: .artifacts/memory/bootstrap-v381-proxy-writers-.
One-second child scheduling is not a native deadline/detection-latency or CPU/RSS
measurement. External changes between observations/ABA and SYSTEM DNS remain
outside this observation; root migration and native acceptance are still open.
No tests/fixtures/build/vet/types/parser/SDK/native/registry/COM/browser/network/generation. Root tools/netguard and installed scripts/tasks/DNS untouched; NETGUARD-001 remains review-required. Source monitoring does not neutralize external writers or prove DNS, no leak, atomic OS CAS/ABA, native detection deadline, cleanup, throughput, CPU/RSS/goroutine bounds or release. One-second scheduling is an implementation target only. Initial/unanchored legacy/migration/W4c/fullPC00..09/old9/native/OS remain; HealStale unavailable/100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported.

<a id="pc-v382-proxy-expiry"></a>

## V2-382 — early negative fault during active lifecycle/child join

2026-11-10, **partial source/unverified**, PC-01 follow-up to V381.

Source review found a V381 cancellation/join gap. When Reload/Switch already owns
Manager.active, the old sampler cannot take over cleanup. Waiting for its complete
done before that active operation acts could leave the old engine alive behind a
blocked proxy native read. Ordinary cancellation also ended pure trust polling
before the old deadline while the native child remained held.

- Reserve one private fault channel and one immutable early cause on the original
  watch. The original parent publishes only a real negative finding; readers wait
  for channel close before reading it. Final cause may still accumulate late faults.
- Active retireRunningLifetime wakes on that fault and attempts its existing
  protection-before-engine-stop/Engine.Stop/proxy cleanup before waiting whole
  done. Original active owner is checked again; there is no alternate cleaner.
- During cancelled join the same original parent keeps immutable deny-only trust
  checks until the child returns or an actual fault requests cleanup. No positive
  interest is renewed. The actual app LifetimeRemaining reads original validity/
  observed clock; this does not invoke profile I/O or recapture a new session.
- Join has its own ordinary-return guard and deferred original-child wait. A
  panic/Goexit in a local check still publishes a finite negative cause and waits
  the native child before the parent done may close. Early fault never releases
  a writer/HKEY slot, resets unknown state or authorizes a replacement observer.
- Stop still attempts its existing cleanup before joining and preserves explicit
  Disconnect semantics. Late negative child cause remains part of final retirement.

2Go5docs/7 existing snapshots; manual2 source diffs;8named0cached;gofmt2+1exit0.
Own receipts: .artifacts/memory/bootstrap-v382-proxy-expiry-.
No tests/fixtures/build/vet/types/parser/SDK/native/registry/COM/browser/network/generation. Source cleanup ordering is not a native deadline or proof of WFP/engine/native close, responsiveness, detection latency or bounded resources. Root scripts/tasks/SYSTEM DNS remain unchanged/review-required NETGUARD-001. Legacy/migration/W4c/fullPC00..09/old9/native/OS/release remain; HealStale unavailable/100OPEN56NOT_RUN/gatesfalse/TUN unavailable/schema2 unsupported.
