# TH-01 — TUN/WFP gate в lifecycle

2026-10-21, V2-116: implementation, **unverified**. Полная цель TH-00…08
сохраняется. Tests не добавлялись/менялись/запускались; build/vet/harness/
native/live/worker/системная сеть не выполнялись. Только исходники и gofmt.

## Исправляемый путь

До среза App вызывал permit/external TUN gate только после Manager.Start.
Reload, seamless switch и verified rollback ограничивались SOCKS probe.
Имя найденного интерфейса не связывалось с отвечающим worker/config/selector,
а внешний curl использовал PATH, detached loop и неограниченный CombinedOutput.

## Реализация

- `core.LifecycleProbe`: BeforeEngineChange и RunLifecycle. Manager вызывает
  их в Start/Reload/restore/selector rollback и cleanup; все lifecycle probes,
  включая оба rollback пути, используют расширенный gate. Periodic Run не
  добавляет внешний curl. Одна engine ownership цепочка Manager → Engine.
- Полная подготовка Reload включает pinned sing-box option validation до
  изменения ownership/policy. Rejected до замены сохраняет старый runtime;
  если permits уже отозваны, прежний путь требует полного нового lifecycle gate.
  При rejection без замены Restarted=false, а не фиктивный новый процесс.
- ProtectionUnavailable — отдельная uncertainty, не подтверждённая утечка.
  Она запрещает candidate PASS. Вернуть прежний путь можно только после отзыва
  текущих permits и свежего полного старого gate. Подтверждённый leak запрещает
  rollback; неудачная/отменённая очистка оставляет protection и uncertain owner.
- `render.ClientConfig.TUNName` берётся из реально создаваемого inbound, Clone
  переносит scalar. App проверяет совпадение с process mode; probe mode следует
  за snapshot. Константы interface name/external endpoint имеют одного владельца.
- `core.Engine.CurrentRuntime`: исходный owned worker отвечает о текущем
  selector; process instance, TUN name и digest owned config сравниваются с cc.
  Digest — typed JSON value normalization ≤2MiB, без config.Parse/file validation
  под Engine.mu. Он приватный, не выводится в UI/логи; это не remote attestation.
- `tunnel_boundary.go`: lease связывает Engine pointer/instance/config/selector
  с конкретным наблюдаемым LUID. До/после external IPv4 flow и после protected
  probe повторяются worker/config/LUID/policy checks. Lease не переиспользуется
  как новое наблюдение при смене lifecycle/intent/epoch. Факт владения kernel
  TUN handle этим не доказывается; local malicious admin не исключается.
- BeforeChange снимает только owned TUN permits и проверяет сохранённый base
  lockdown. Cleanup продолжает engine/proxy retirement при ошибке guard; обе
  IP families очищаются, отсутствие permits проверяется повторно. Чужой provider
  со столкнувшимся TUN filter ID отвергается, его filter не удаляется.
- Установка permit принимает только exact ожидаемый LUID. Уже корректный набор
  не переустанавливается при seamless проверке. Неоднозначный/down интерфейс и
  изменение LUID отвергаются; ожидание прерывается caller context.
- У native boundary есть явный private injection seam. При custom probeFactory
  без explicit TUN boundary вызывается отказ до native действий. Это не bypass
  production protection. Старым TUN unit fixtures потребуется адаптация позже;
  сейчас tests не правились и не запускались.
- Periodic Observe проверяет текущий worker/config/selector/LUID/policy без
  нового external HTTP и не объявляет старое внешнее наблюдение свежим.

## Внешний IPv4 probe и ограничения

SystemDirectory/curl.exe, скрытый process, первый аргумент -q, proxy/config
выключены, только HTTPS/IPv4. Shared protected admission резервирует3 попытки
и 3×4KiB output, caller deadline и per-attempt context4s; curl max-time3s,
connect2s. V2-146 [owned helper](TH-01-NATIVE-HELPER.md) заменяет прежний
direct-child/WaitDelay на suspended Job limit1/128MiB committed memory,
bounded Max+1 pipe и original-handle complete reap; busy/uncertain owner blocked.
stdout>4KiB отвергается, stderr не публикуется; minimal system env/cwd.
Требуются успешный process exit, отдельный curl HTTP status200 trailer и ровно
один валидный ip, равный expected через net.IP.Equal. Error page, duplicate ip,
overflow/cancel/timeout не становятся egress proof.

Семантика -q/http_code/max-filesize сверена с [официальной справкой curl](https://curl.se/docs/manpage.html).
До curl8.4 max-filesize не ограничивает заранее неизвестный размер; поэтому
есть собственный writer/deadline. Установленная Windows версия curl не запускалась
и не проверялась. Новый Job source contract не является выполненной native
приёмкой или доказательством общего kernel/driver time bound.

1.1.1.1 trace остаётся фиксированной общей зависимостью Cloudflare, отмеченной
в dependency graph как conditional TUN measurement. Это не независимый канал
и не текущая проверка доступности. Endpoint outage/quorum/замена measurement
path остаются TH-02/03/04; ошибки измерителя не равны quarantine всех каналов.
IPv4 echo не доказывает native DNS/UDP/IPv6/bulk, endpoint TLS не TUN handle owner.

## Остаток

Compilation/schema fixtures, Windows nested jobs/worker/query/LUID timing,
WFP reauthorization/established flows, route/driver cleanup, policy corruption/
explicit Disconnect, crash/reboot/ACL/interprocess, late cancellation и реальный
external traffic остаются непроверенными gates. Callbacks/kernel calls не получили
общего hard timeout или отдельной службы. Исторические PASS их не покрывают.

Далее TH-03 clock/profile/cert expiry и trust recovery; TH-02/04 измерители,
bounded actions/quorum/backoff/wakeup. TH-05…08 и все9 этапов остаются активными.

<a id="pc-v228-native-reader"></a>

## V2-228 — original actor, TUN exchange и late local cause

2026-11-05, Windows **source/unverified**. Полная цель PC00…09 и прежние девять
этапов сохраняются. Tests/fixtures/build/native/product execution запрещены.

### Source findings

`exchangeTUNHelper` проверял context перед запуском I/O goroutine, затем decoder/
writer выполнялись без fresh guard; ожидание слушало только ctx.Done, а два
раздельных result/done позволяли потерять уже завершённую ошибку при cancellation.
`NativeTUNActor.Descriptor` не удерживал creation context и использовал supplied
ctx. Перед DuplicateHandle были native queries без нового source check у mutation.
`helperScope` заменял actual guard error поздним context error. Это source trace,
не воспроизведённые runtime defects.

### Изменения и ownership

- Existing TUN exchange теперь один `nativeTUNExchange` с done/message/error.
  Он прикреплён к **тому же** engineProcess/ioDone. Одна прежняя I/O goroutine
  публикует fields до close(done); потребитель читает их только после close.
  Result channel и отдельное второе ожидание I/O больше не нужны.
- Перед заменой ioDone требуется завершение original I/O. При unknown original
  помечается unusable, возвращается local cleanup error; никакой новой команды,
  процесса или забывания старого record. Один current record, без списка/карты.
- Внутри goroutine source/deadline проверяется до write codec и перед началом
  decoder/EOF read, затем после него. Exchange и ожидание original process.done
  слушают уже существующие intent/operation/dependency revoke channels.
  Отзыв не является process/Job/pipe/resource retirement receipt.
- После early cancellation record остаётся у того же original process. Existing
  reaper закрывает pipes и ждёт ioDone. Finalizer добавляет **завершённый** original
  codec/panic error к earlier failure даже после потери интереса caller. Unknown
  не становится отсутствием fault или успехом. Ошибка после forced pipe closure
  не приписывается каналу/оператору как новая causal health evidence.
- Actor хранит original bounded capture ctx, Descriptor сначала проверяет его,
  затем private adapter переносит exact bound OperationSources в supplied ctx.
  Более новый/foreign binding отрицательно отклоняется; original scope не
  подменяется. Child deadline не длиннее original until; cancel в defer.
- Confirmation и consuming worker probe наследуют ту же original operation
  binding перед Engine lock/IPC. Public observation/report/descriptor fields
  не создают её. Existing one-use native receipt/native permit gates сохраняются.
- Generic unbound original context сохраняет существующий contract; original
  own ctx проверяется, source identity из воздуха не создаётся. Это не обещание
  мгновенной передачи generic cancellation в произвольный background caller.
- Source rechecked после readonly current IPC до scope reservation, после
  original transfer и непосредственно перед DuplicateHandle после native queries.
  Remote handle сохраняется прямо в original receipt даже при partial API error;
  parent не закрывает его в своей handle table. Original target Job остаётся
  единственным владельцем неизвестного remote outcome.
- Existing negative scope/filter/Job/image cleanup использует независимый budget
  и не получает positive source precondition. helperScope/panic finalizers join
  earlier local cause с later cancellation, не затирая её.
- Descriptor и начальная/последняя CurrentRuntime проверки сохраняют actual
  engine/cancellation cause вместе с finite scope error, не теряя typed custody.

### Evidence и незакрытые gates

Source7Go/4 собственных docs; exact-file gofmt exit0 и manual source/caller/lock/
async ownership/diff/byte-SHA review. Evidence:
`.artifacts/memory/native-reader-source-evidence.json`. Original task/document
bytes и точный прежний composite task71 сохранены; журнал только дополнен.

Нет tests/fixture edits/run, compile/build/vet/typecheck/product parser/check/
doctor/install/generation/SDK/helper/product/crypto/native/browser/admin/live/
DNS/server/OS-settings/owner-values/other-role/chat/subagent execution.
Все acceptance остаются OPEN; исходники не объявлены runtime PASS.

OPEN: legacy API/fixtures; actor with generic/current/foreign/revoked contexts;
caller/original deadline, delayed timer and passive revoke races; cancellation
before goroutine/write/partial write/read/EOF/exit; malformed/trailing frames;
panic/late mixed local error publication; ioDone replacement and exact reaper
retirement; partial duplication/native grant/revoke; WFP/OS/routes/driver/real
protected path; count/bytes/RSS/CPU/latency/performance. No new monitor/source
watcher or I/O goroutine, но standard child context timer может существовать.
Checks перед codec/API не дают atomic write, syscall/driver interruption или
hard mutex/OS time bound. Native acceptance=false, IPC9/Job8/TUN unavailable/
schema2 install unsupported прежние. Далее common helper async reader и native
DNS, остальные native writers; полный PC/old9/SDK/W3b/interprocess/durability/
UX/release/comparative scope не завершён.

<a id="pc-v486-tun-connection-controls"></a>
## V2-486 — явное владение TCP deadline/handshake

2026-11-16, **PC-01 source/unverified**. Новый компонент
`windows/backend/core/tun_connection.go` выделен из существующего tun_flow;
он сохраняет двух original stream owners, existing Conn: constructors и net.Conn API.

### Основание и изменение

Раньше embedded net.Conn напрямую предоставлял SetReadDeadline/SetWriteDeadline.
Они обходили mutex/seal, которыми уже защищались SetDeadline, handshake и Close.
В закреплённом sing-tun gLazyConn все три deadline setter вызывают ленивый
HandshakeContext. Это подтверждает пробел интерфейса, но не воспроизведение гонки:
нынешний sing CopyConn явно half-deadline setters не вызывает.

1. Conn стал именованным полем. Явные LocalAddr/RemoteAddr сохраняют delegation;
   все net.Conn методы присутствуют, добавлен compile-time source assertion.
2. Все три setter вызывают соответствующий original native метод под общим mutex
   после проверки closing. Обычная возвращённая ошибка не заменяется и не означает
   неизвестный результат управления.
3. Setter/handshake устанавливают guard normal return. Panic/Goexit публикует
   closing и sticky controlFault до mutex unlock; исходный unwind не перехватывается
   с успешным результатом. Новые операции после seal отклоняются.
4. Original Close остаётся once с предварительной отрицательной квитанцией.
   Его нормальный успех не стирает предыдущий controlFault. Flow proof дополнительно
   проверяет оба original fault наряду с bound/cleanup/counters.
5. Read/Write не держат close mutex во время блокирующего I/O. Их join и cleanup
   остаются обязанностью существующего flow owner. Upstream/replaceability и
   half-close API не добавлены; SDK не получает нового пути раскрыть raw owner.

### W4c и оставшиеся границы

appProtectedProbe.CommitNativeForwarding и Manager private context уже составляют
цепочку commit в source. Prepared boundary сам permission не устанавливает;
его комментарий приведён в соответствие этому коду. Compiled native acceptance
остаётся false, TUN недоступен. Никакая публичная HTTP/IP сводка не заменяет
original helper/Job/IO/image/WFP evidence.

Metadata LocalAddr/RemoteAddr всё ещё делегируются исходному соединению; ownership
их callback здесь не закрыт. Native control, handshake или Close могут блокировать
либо reenter; mutex не доказывает preemption или предел wall time. Ранее начатый
control может завершиться до получения Close mutex. Native/OS retirement требует
собственных original receipts и original Job, не только завершающего Go-frame.

### Доказательство и остаток

Только source:3Go4docs/6existing1new; ручной просмотр трёх diff, gofmt3 exit0.
18 reviewed input hashes и4 cached SDK source hashes сохранены; q87/s95 unchanged,
261 cached originals/190 verbatim mirrors/59 owned mirrors не изменены.
Нет tests/fixtures/build/vet/typecheck/product/SDK/helper/native/admin/live/network,
новой публикации, Android/shared/configs/harness/других чатов или субагентов.

Compile/API/fixtures, panic/Goexit/close/handshake/reentry/cancellation/соперничество,
address metadata custody, WFP/Job/OS/path/resources/release/comparative acceptance
открыты. Полный PC-00…09/прежние девять TH и100 критериев остаются OPEN;
72ПК-задачи и56 product NOT_RUN+2 прежние Git-only checks сохранены.
Следующий source-вопрос — native actor Descriptor/metadata и original retirement.

<a id="pc-v487-tun-actor-read"></a>
## V2-487 — NativeTUNActor и original Descriptor read

2026-11-16, **PC-01 source/unverified**. Actor и публичный value descriptor
выделены из native_tun_capture в `windows/backend/core/native_tun_actor.go`.
Существующий ObserveNativeTUN конструирует тот же actor; WFP использует прежний API.

### Исходное поведение и изменение

Раньше Descriptor удерживал actor.mu через CurrentRuntime IPC, image.ValidateProcess,
ancestorScope и image.Executable. Capture.revoke сначала вызывал deactivate,
ожидающий тот же mutex, и только затем регистрировал ingress/filter revocations.
Такое удержание подтверждается исходниками; runtime-deadlock не воспроизводился.

1. Один embedded read slot закрепляется до первого callback. Он держит original
   caller/derived context, cancel и конечную phase. Параллельный/reentrant Descriptor
   возвращает unavailable+busy вместо ожидания. Новых worker/timer/queue/owner нет;
   прежний derived context/deadline используется с тем же исходным пределом.
2. IPC/image/ancestor/path callbacks и original cancel выполняются вне actor.mu.
   Actor source fields immutable после публикации; mutable phase payload пишет
   только принятый читатель. Cleanup читает pending/fault, не mutable payload.
3. Внешний guard считает чтение вернувшимся только после query и его cancel defer.
   Panic/Goexit сохраняет исходный unwind, закрывает actor и оставляет unknown read
   с исходными ссылками. Его нельзя повторить или заменить новым успешным запросом.
4. Executable вызывается до последних проверок источника. Перед возвратом descriptor
   повторены active/original context/deadline/helper checks. Поздний normal callback
   после deactivation не публикует успешный descriptor; его собственное завершение
   при этом может разрешить последующий отрицательный cleanup того же owner.
5. Deactivate ставит seal под коротким metadata mutex. Revoke заранее регистрирует
   descriptor-retirement, filter и ingress steps, фактический порядок — ingress,
   filter, descriptor. Unknown ingress всё равно даёт попытку следующим отрицательным
   шагам. Pending/unknown descriptor не позволяет helper lane освободить socket/image.
6. Частичный capture без опубликованного actor не имеет Descriptor borrower;
   nil actor receipt допустим только в этом случае. Scope/filter cleanup сохраняется.

### Границы результата

Original Engine/WFP lease/native mutexes и syscalls могут блокировать свои операции;
снятие actor.mu не доказывает прерываемость native, hard wall time или OS retirement.
Возврат original context cancel не равен join всех timer/AfterFunc/goroutine frames.
Source contextError сохраняет прежний contract; произвольные сторонние реализации
context/callback и внутренние не возвращённые provider resources этим не ограничены.
Обычная возвращённая ошибка не превращается в unknown custody; её normal query
retirement не выдаёт forwarding authority. Unknown read остаётся отрицательным.

Ручной просмотр2 source diff и gofmt2 exit0;2Go4docs/5existing1new.
16 reviewed source hashes сохранены, vendor q87/s95/261 cached originals/
190 verbatim mirrors/59 owned mirrors неизменны. No tests/fixtures/build/vet/types/
product/SDK/helper/browser/native/admin/live/network/newGit или чужих областей.

Compile/API/fixtures, late return/cancel/panic/Goexit/reentry/concurrency, original
WFP/Job/image/actor custody, OS/path/resources/release/comparative acceptance OPEN.
Native compiled gate=false, TUN unavailable/schema2 unsupported. Полный PC-00…09,
прежние TH-00…08,72ПК-задачи,100OPEN/56 product NOT_RUN+2 Git-only checks сохранены.
Следующий source-вопрос PC-01 — исходные WFP lease callbacks и original session
retirement в Acquire/Validate/Close, включая неизвестный Add/Close outcome.

<a id="pc-v488-wfp-operation-scope"></a>
## V2-488 — original operation у diagnostic WFP Open/Add

2026-11-16, **PC-01 source/unverified**. Сохранён существующий учёт native session,
reader, transaction и Add/Close outcomes. Source-пробел был в контексте операции:
native diagnostic Open/Add не принимали ctx, а более ранние lease/Descriptor checks
не подтверждают scope после Find/UTF16/filter preparation непосредственно перед Call.
Случай позднего grant в runtime не воспроизводился.

### Изменение

1. Новый Windows-local wfpOperationScope переиспользует childbootstrap.CheckScope
   с value-only core.OperationSourcesCurrent. Он проверяет ctx.Err, действительный
   deadline и исходные operation/path/closing/blocked sources. Новых timer/context/
   worker/native callbacks helper не создаёт; generic unbound ctx сохраняется.
2. Actual diagnostic session открывается через openSessionContext(ctx). Original
   native owner проверяет scope до подготовки и непосредственно перед Open.
   openCode/openReturned/handle сохраняются до повторной проверки scope после Call.
3. Private nativeDiagnosticWFP.open теперь требует ctx. Его единственный production
   caller обновлён. Проверки проходят до подготовки, перед Add и после записи
   addReturned/writePending/filterID. Fixed rule/ABI/native procedure не изменены.
4. Поздний native success возвращает local unavailable; original outputs и
   pointer backing stores остаются в том же owner. Existing negative revoke/Close
   очищает исходную dynamic session, не подменяя её новым handle или same-key rule.
5. Acquire повторяет scope после ожидания lease mutex перед global claim и перед
   opened=true. Validate проверяет scope в начале и после final original Descriptor.
   Существующие проверки AppID/LUID/port/actor/filter/base policy сохранены.
6. Negative Close/Free/Abort не зависят от того, жив ли исходный diagnostic ctx.
   Их original callsReturned/unknown-result requirements не ослаблены. Generic
   openSession/openSessionMode сохраняют Background semantics и прежний private API.

### Остаток и доказательство

Cancel может произойти после допуска уже начатого native Call: source-check не
делает cancellation и BFE write атомарными и не прерывает syscall. После позднего
известного успеха нужен original negative cleanup; unknown outcomes по-прежнему
сохраняют владельца. Lease mutex продолжает сериализовать mutable reader/session;
его blocking и остальные Engine/native locks здесь не закрыты.

Остальные generic WFP policy/query/transaction source paths не приняты этим срезом.
Public NativeTUNPolicy/lease API не изменён; private Add signature изменилась,
tests/fixtures не правились, совместимость и compile остаются непроверенными.

Ручной просмотр3 source diff и gofmt3 exit0;3Go4docs/6existing1new;
14 reviewed source hashes,0 cached SDK inputs. q87/s95/261 cached originals/
190 verbatim mirrors/59 owned mirrors неизменны. No tests/build/vet/types/product/
SDK/helper/browser/native/admin/live/network/newGit/чужих областей или чатов.
Полный PC-00…09/TH-00…08/72ПК-задачи/100OPEN/56 product NOT_RUN+2 Git-only checks
сохранены. Native gate=false, TUN unavailable/schema2 unsupported.
Далее — source audit контекстов positive WFP transaction/grant и отделения
independent negative cleanup; fixture/native/OS/path/resource/release acceptance OPEN.

<a id="pc-v489-wfp-batch-source"></a>
## V2-489 — WFP original batch/transaction/write source

2026-11-16, **PC-01 source/unverified**. Существующий writer уже сохранял native
inputs/returned results и первый failed grant. Исходный batch не хранил operation
ctx, native mutation не проверял value-only authority, Commit проверял ctx.Err.
Более ранняя проверка не доказывает scope после native preparation/ожиданий.
Runtime-сбой позднего grant не воспроизводился.

### Изменение

1. Existing batch хранит неизменяемый original operation ctx до Begin. Transaction
   получает original batch/backlink до Find/Call. Early Begin failure не принимает
   предыдущую transaction; legacy/read-only begin API сохраняет nil-batch path.
2. Each existing fixed40 write record хранит original batch до подготовки inputs.
   nextWrite/callWrite требуют точных owner/batch/transaction/backlink и writable
   active phase. Старый write нельзя перенести на новый ctx/transaction.
3. Positive Arm/Grant переиспользуют real deadline/value-source wfpOperationScope
   перед Begin, резервированием write, Find/мутацией и Commit. После actual syscall
   returned flags/IDs/code/commitConfirmed сохраняются до late-error projection.
4. Caller Commit ctx может сократить операцию, но дополнительно проверяется именно
   original batch ctx. Свежий caller не оживляет старую operation authority.
5. Writer повторяет scope после входа, после verification и original cleanup.
   Late failure доходит до existing retainFailedGrant/finalizer: first failed batch
   остаётся исходным, pending не снимается новым read/совпадением stable key.
6. Restrict/Release используют собственный ctx.Err-only contract на новых границах.
   Значения revoked grant context их не блокируют; собственная отмена independent
   cleanup ctx по-прежнему приводит к отказу. Negative native Free/Abort/Close и
   callsReturned/unknown Commit guards сохранены; automatic Release не добавлен.

### Границы

Scope-check не делает native Call и cancellation атомарными. После допуска Call
может завершиться поздно; original result сохраняется и требует negative cleanup.
Unknown result нельзя заменить новой session/transaction. Writer mutex/Native/OS
blocking и hard time bound не доказаны. Context root хранится в current/first failed
batch, не в растущей истории; объём произвольного context graph/RSS не измерен.

Existing applyWFPBase по старому API использует Background: lifecycle/intent этой
точки не объявлены закрытыми. Generic legacy/read APIs сохранены; новые private
batch pointer invariants могут требовать адаптации fixtures после разрешения tests.

Ручной просмотр3 source diff и gofmt3 exit0;3Go4docs/7existing0new;
13 reviewed source hashes,0 cached SDK inputs. q87/s95/261 cached originals/
190 verbatim mirrors/59 owned mirrors неизменны. No tests/build/vet/types/product/
SDK/helper/browser/native/admin/live/network/newGit/чужих областей или чатов.
Native gate=false/TUN unavailable/schema2 unsupported. Полный PC-00…09/TH-00…08/
72ПК-задачи/100OPEN/56 product NOT_RUN+2 Git-only checks сохранены.
Далее — source audit Arm/BlockDirect/Release caller lifetime и independent cleanup;
fixture/compile/native/OS/path/resource/release/comparative acceptance OPEN.

<a id="pc-v490-protection-arm-scope"></a>
## V2-490 — original intent для App Arm

2026-11-16, **PC-01 source/unverified**. Actual App Arm раньше переходил через
strict policy к marker/applyWFPBase с Background. Срыв network context при этом
не отменяет желание удержать базовый барьер; явный Disconnect отзывает исходный
intent. Эти два случая сохранены отдельно от допуска пользовательского forwarding.

### Изменение

1. protection_arm.go извлекает именно original appOperationScope и его intent source,
   operation source, App lifetime/closing и captured policy. Issuer проверяет exact
   registration и protectionDesired под App.mu. Numeric intent не заменяет pointer.
2. Все3 App callers перешли на общий helper. Новый5s standard context берётся от
   App lifetime; real deadline/source проверки используют существующий CheckScope.
   Physical network cancel не делает защитный intent недействительным. Завершение
   operation, successor/Disconnect source revocation или App closing отклоняют scope.
3. protectionPolicy API сохранён; optional ArmContext поддерживает strict policy.
   Compiled strict path переносит контекст в marker и applyWFPBaseContext. Injected
   legacy Arm проверяется до/после вызова; внутреннюю native conformance не выдумываем.
4. Marker получает проверки до read/directory/write/apply и после apply. Уже начатая
   запись может завершиться поздно; marker остаётся recovery intent. Автоматически
   он не удаляется; queued explicit Disconnect идёт после caller-owned operationMu.
5. WFP helper проверяет private Arm source без App.mu. General Grant и diagnostic
   Acquire/Validate/Add явно отвергают Arm-tag context. Base protection scope не
   заменяет actual network/native observation или final forwarding source.
6. Raw legacy Arm/base/marker signatures сохранены. Actual strict App calls используют
   scoped путь. Manual Release commit после Manager/helper cleanup и independent
   BlockDirect остаются прежними; новый automatic Release не добавлен.

### Остаток и доказательство

Новый context/timer — один на сериализованный Arm action.5s — operation policy,
не измеренный SLO/не forced syscall timeout. Cancel return не является timer/OS/
native thread join. Произвольный context graph, native/file blocking и полный RSS
не ограничены этим срезом. Marker read size/ACL/interprocess/atomic write/fsync/
power-loss и cooperative native cancellation требуют самостоятельной приёмки.

Source audit обнаружил legacy osRunner.Run: raw exec.Command(netsh)/CombinedOutput
не имеет original process/Job/output budget. Он не исправлен здесь; следующий
приоритет — его owned bounded replacement с сохранением семи точных rule names
и explicit manual release authorization. Публичные конфиги/серверы не затрагиваются.

Ручной просмотр8 source diff и gofmt8 exit0;8Go4docs/11existing1new;
19 reviewed source hashes,0 cached SDK inputs. q87/s95/261 cached originals/
190 verbatim mirrors/59 owned mirrors неизменны. No tests/build/vet/types/product/
SDK/helper/browser/native/admin/live/network/newGit/чужих областей или чатов.
FullPC00-09/TH-00…08/72ПК-задачи/100OPEN/56 product NOT_RUN+2 Git-only checks
сохранены. Native gate=false/TUN unavailable/schema2 unsupported. Private fixtures/
compile/intent/timeout/panic/Goexit/reentry/concurrency/native/OS/path/release OPEN.

<a id="pc-v491-owned-legacy-firewall"></a>
## V2-491 — original command owner для legacy netsh

2026-11-16, **PC-01 source/unverified**. Raw exec.Command(name)/CombinedOutput
заменён новым legacy_firewall_windows runner и существующим protected helper owner.
Actual production caller — disarmKillSwitch в уже committed manual Release после
Manager/helper cleanup. Новая automatic Release или публичная команда не добавлена.

### Выполнение и результат

1. Разрешены только name=netsh и пять точных args удаления одного из семи известных
   rule names. Исполняются заново созданные canonical args, не mutable caller slice.
   Native GetSystemDirectory задаёт absolute netsh.exe; shell/PATH не используются.
2. Один command ctx5s и existing protected lane/Job1/128MiB. Output reader заранее
   выделяет4096+1 bytes и требует EOF; overflow/timeout остаются ошибками. Independent
   original reap и unknown-output/handle guards сохранены;5s не forced syscall limit.
3. CaptureProtectedCommand возвращает typed code/output только после original
   I/O effects/Wait/empty Job/handle/preparation/source checks и full cleanup.
   Nil capture error означает факт capture/retirement, не успех команды: ExitCode
   проверяется отдельно. Частичный результат при ошибке не возвращается.
4. Default CaptureProtectedHelper по-прежнему требует zero exit; command-result
   режим нельзя смешать с pinned-image/guard/update-verifier options.

### Stdout и stderr

Source audit выявил existing stderr sink. Добавлен command-only PrepareCombined:
single parent output pipe, distinct inherited stdout/stderr duplicates одного
original outputChild. Native slot ownership/three-handle allowlist/once-close
сохранены; unused stderr File закрывается как original empty slot. Второго reader
или неучтённого handle copy нет. Обычный Prepare и Detached public API прежние;
private mode signatures изменились. Engine/verifier/GUI combined mode отвергается.

Три дополнительных Go-targets заморожены с совпадением predecessor named hashes
до правок; initial receipts сохранены, extension checkpoint1284 precedes edits.
Дополнительные consumers включают original File.Close negative-empty behavior.

### Ошибки и отсутствие правил

Только completed capture с ExitCode1 и exact whole known message даёт exact absent
sentinel. Переиспользуется pure netguard UTF8/UTF16/1251 decoding, pinned866 и
полные UTF16 code units для no-BOM случая. Odd BOM UTF16, mixed/unknown/empty/
oversized output не доказывают отсутствие. Набор message/locale/code вариантов
консервативный; универсальная Windows locale conformance не заявлена.

Все actual failures typed/redacted, native causes сохраняются для errors.Is/As,
arbitrary stdout не попадает в UI/logs. Их нельзя принять за missing rule старой
строковой эвристикой. KillSwitchRunner и injected legacy text compatibility
сохранены как seam, не native proof. Marker удаляется только после успеха both WFP
и всех legacy outcomes; unknown retirement сохраняет original owner и marker.

### Доказательство и остаток

Ручной просмотр8 source diff; gofmt5+4 exit0 (8 unique paths),8Go4docs/10existing2new.
18 reviewed source inputs (15 initial+3 supplement),1 cached x/text source.
q87/s95/261 cached originals/190 verbatim/59 owned mirrors неизменны, новых deps нет.
Нет tests/fixtures/build/vet/types/product/SDK/helper/browser/native/admin/live/
network/newGit/чужих областей/чатов. Никакой netsh фактически не запускался.
5s/128MiB/output budget не доказывают hard total Release latency/RSS/native
preemption; actual platform/locale/stdio/Job/fixture/compile acceptance OPEN.
Далее marker bound/ACL/interprocess/atomic persistence/removal identity/durability.
FullPC00-09/TH-00…08/72ПК-задачи/100OPEN/56 product NOT_RUN+2 Git-only checks
сохранены; native gatefalse/TUN unavailable/schema2 unsupported.

<a id="pc-v492-protection-marker-custody"></a>
## V2-492 — bounded marker и исходный handle на весь ручной Disconnect

**Статус:** source/unverified; PC-01/TH-01. Никакие product gates не закрыты.

### Фактический дефект и переиспользование

Ранее marker path читался через unbounded os.ReadFile, json.Unmarshal молча
принимал duplicate/case aliases, os.WriteFile мог заменить существующее
содержимое, а os.Remove удалял имя после долгого cleanup без custody исходного
объекта. Исследованы существующие filetxn read/write/journal locks/deletion
и childbootstrap filelease; новый независимый файловый стек не создаётся.

### Arm и формат

- Постоянные BoundedReadSlot/BoundedWriteSlot с пределом1024bytes; Windows
  trusted-store ACL, regular/single-link/reparse/path checks остаются native API.
- Token-first decoder допускает только exact policy/applied_at/helper_pid;
  duplicate/alias/unknown/null/trailing/invalid UTF8 отвергаются. Policy strict-wfp,
  корректный nonzero timestamp, optional zero helper_pid. PID/time не дают
  authority на процесс, expiry, автоматическую очистку или enforcement.
- Existing WriteNew создаёт bounded temporary, flushes original bytes,
  non-replace rename, published file flush и checked original Close. Readback
  проверяет точные bytes; после apply проверяются original ID и bytes.
- Original App operation scope проверяется до/после I/O/apply. Поздний marker
  остаётся intent; отменённый Arm не получает WFP grant. Ни синхронное файловое
  I/O, ни flush/rename не объявлены hard deadline или power-loss durable.

### Сериализация и явное снятие

Local TryLock предотвращает очередь повторных операций. Existing WithWriter
держит оригинальные ancestor/lock-file handles и fail-immediate byte-range lock
на весь read/Arm либо manual Release. Новый original WindowsRemovalSlot заранее
удерживает nativeJournalTargets и nativeFileDeletion; prepare открывает именно
исходный marker с запретом write/delete sharing. Этот handle остаётся живым через
removeWFP и все7 legacy outcomes. Только их обычный успех разрешает commit
disposition на этом же handle. Затем original Close и native absence через тот
же held parent; successor не удаляется по имени/hash/повторному snapshot.

При initial native absence обе cleanup ветви всё равно выполняются; появившийся
marker даёт ошибку финальной проверки без его удаления. Recovery до16MiB проверяет
только metadata/ACL/identity, содержимое не читает и не декодирует: malformed,
empty и превышающие Arm1KiB markers допускаются для явного Disconnect. Больше
16MiB/untrusted/readonly/reparse/hardlink/sharing конфликт не снимаются этим путём.
Обычная ошибка cleanup не выставляет disposition; unknown callback/native/Close
удерживает исходных владельцев и запрещает их замену. Atomic pending/uncertain
поддерживает консервативную видимость intent/custody даже при отсутствии имени.

### Аварийный callback и совместимость

Source review выявил recover-only defer в WithWriter: runtime.Goexit не даёт
panic payload, но callback не вернулся. Callback-return flag теперь устанавливает
тот же domain fault до release; исходный lock/owner остаётся удержанным.
Обычные callback результаты и public API прежние. Journal deletion использует
тот же prepare/commit; expected digest и initial absence поведение сохранены,
добавлена parent validation перед absence probe. Совместимость потребителей
и partial/native output поведения не подтверждена исполнением.

### Доказательство и остаток

5Go4docs/7existing2new;5 вручную просмотренных source diffs, gofmt5+1+1 exit0
(5 unique files).18 reviewed inputs/0cached SDK. Supplemental journal_lock.go
совпадает с initial reviewed hash, замороженным до precheckpoint1289; отдельные
original snapshot/initial list и reviewcheckpoint1290 предшествуют этой правке.
q87/s95/261 cached originals/190 verbatim/59 owned mirrors неизменны.

Тесты/fixtures не добавлялись и не менялись; tests/build/vet/typecheck/product/
harness checks/native/admin/live/network не запускались; commit/push/release
не выполнялись. Эта блокировка координирует операции
новых cooperating builds; она не доказывает full multi-process VPN/session lease,
совместимость старых builds, службу FR-009…011, OS/power-loss durability, native
preemption, fixture/compile/production acceptance. Полный PC00-09/old9/72 tasks/
100OPEN/56 product NOT_RUN+2 исторических Git-only checks сохранён; gatefalse,
TUN unavailable/schema2 unsupported/rootNETGUARD001 OPEN.

Далее PC-01: negative enforcement/resource custody после неуспешного Arm,
Restrict и ручного Release; проверить фактических native WFP owners и reset paths
без нового контроллера и без открытия native TUN gate.

<a id="pc-v493-restrict-barrier-proof"></a>
## V2-493 — complete barrier proof после restrictive commit

**Статус:** source/unverified; PC-01/TH-01, не native/OS приёмка.

### Найденные исходные дефекты

1. verifyWFPPolicyAction(Restrict) подтверждал только отсутствие20/21. Пустая
   policy или отсутствие обязательных blocking filters не препятствовали успеху.
2. pendingGrant/first failedGrant очищались в основном теле writer до deferred
   reader/transaction cleanup и final caller scope. Поздняя ошибка могла потерять
   исходную запись о неудачном Grant, хотя Restrict не вернул успех целиком.

### Реализация

Original batch/action/transaction/backlink обязательны. До commit остаётся
identity-only подтверждение удаления20/21. После commitConfirmed исходной
transaction вызывается existing verifyWFPScopeContext на той же original session:
valid persistent provider/sublayer ownership/weight, exact8 base filters/AppID/
shape и отсутствие general/diagnostic/unknown owned permits. Missing/corrupt base
возвращает ErrHelperCleanup с исходной причиной. Это локальный дефект защиты,
а не сигнал менять каналы. Read-only postcommit transaction не заменяет исходный
batch.transaction receipt, который выбирает фазу проверки.

Полная base проверка намеренно не перенесена до restrictive commit: при её
ошибке Abort вернул бы удаляемые permits. Сначала допускается commit их удаления,
потом отдельно подтверждается полный барьер. Если его нет, удаление остаётся
committed, но защищённое состояние не подтверждено и base не создаётся молча.

Только Restrict с complete body, полным postcommit proof, обычным original
reader/transaction cleanup и актуальным independent negative caller scope
очищает pendingGrant/first failedGrant. Поздняя ошибка/panic/Goexit сохраняет
исходный failed receipt; новые Arm/Grant не обходят pendingGrant gate.
Revoked value sources старого Grant не блокируют independent negative cleanup.

### Границы и следующий шаг

Manual Release reset/Close/owner-unpublication ordering этим срезом не изменён
и не принят. Strict base proof отвергает также active diagnostic filter: нужно
проследить фактический Stop/helper/Manager cleanup и свежую финальную Restrict
проверку, а не ослаблять правило до принятия неизвестных permits. Original native
owners/syscalls/capture/marker contracts и unavailable TUN gate сохранены.

2Go4docs/all6existing, manual2 source diffs/0new, gofmt2 exit0,14 reviewed inputs/
0cached SDK;859 predecessor named hashes0drift. q87/s95/261 cached originals/
190 verbatim/59 owned mirrors неизменны. Тесты/fixtures не добавлялись/не менялись;
tests/build/vet/typecheck/harness checks/product/native/admin/live/network не
запускались; commit/push/release не выполнялись. Compiled/native/OS/cleanup/SLO
acceptance OPEN; fullPC00-09/old9/72tasks/100OPEN56 product NOT_RUN+2 Git-only
checks сохранены. TUN unavailable/schema2 unsupported/rootNETGUARD001 OPEN.

<a id="pc-v494-stop-revoke-finalize"></a>
## V2-494 — original Release finalization и phased Stop

**Статус:** source/unverified; PC-01/TH-01, не compile/native/OS приёмка.

### Исходные проблемы

- Manual WFP Release очищал pendingGrant/failedGrant, закрывал session и убирал
  global owner в основном теле, до deferred reader/transaction cleanup/scope.
- Stop вызывал полный BlockDirect до original Manager/helpers cleanup. После
  усиления V493 это могло отвергать ещё живой diagnostic filter; финальной
  свежей проверки после фактического actor cleanup не было.

### Отрицательные фазы на том же владельце

Private wfpPolicyRevoke добавлен к existing WFP writer/negative ctx: exact20/21
removal и identity-only absence до/после commit. Он не добавляет permits, не
принимает diagnostic filter за proof, не очищает pendingGrant/first failedGrant
и не подтверждает полный барьер. Nil Revoke относится только к этой фазе;
positive Arm/Grant по-прежнему блокируются pendingGrant. Restrict сохраняет
полный strict postcommit base proof V493. Отдельного policy owner нет.

Optional protectionRetirementPolicy имеет RevokeDirect; actual strict method
остаётся Windows-only. Public protectionPolicy/Wails/App API не расширен.

### Порядок Stop

Под существующим operationMu захватывается original policy. Все negative
callbacks регистрируются заранее. Фактическая последовательность:

1. stopping projection;
2. early Revoke при требуемой защите, включая explicit Disconnect;
3. original facts и metrics requests;
4. original Manager;
5. original App capture helpers;
6. final full Restrict для retained protection либо final Revoke для Disconnect;
7. sequence.finish и только затем existing App manual Release commit.

Panic/Goexit не превращают незавершённый callback в успех и сохраняют остальные
negative defers. Блокирующий native callback не прерывается искусственно. Более
поздний успех не стирает раннюю ошибку. Manual Disconnect может удалять damaged
owned base: prerequisite полного здорового барьера для recovery не добавлен.
Injected nonstaged policy сохраняет manual contract; для retained protection
BlockDirect выполняется early/final. Call-count/fixture совместимость не проверена.

### Original Release outcome

Body подтверждает commit и полное отсутствие owned policy на original session.
Deferred finalizer завершает original reader/transaction cleanup и negative
caller scope. Затем проверяет same global owner, выполняет original guarded
Close и требует closed/handle0/callsReturned до reset/unpublication. Ошибка или
невернувшийся Close удерживает owner/history; неизвестный handle не закрывается
повторно и не заменяется новым. Если caller отменён уже после known native Close,
API возвращает отмену, но подтверждённые retired resources не воскрешаются;
explicit retry не блокируется искусственным закрытым owner. Это не успешная
API приёмка и не разрешение заменить unknown native owner.

### Доказательство и остаток

5Go4docs/all9existing, manual5 source diffs/0new, gofmt5 exit0,14 reviewed inputs/
0cached SDK;859 predecessor named hashes0drift. q87/s95/261 cached originals/
190 verbatim/59 owned mirrors неизменны. Tests/fixtures не добавлялись/не менялись;
tests/build/vet/typecheck/harness checks/product/native/admin/live/network не
запускались; commit/push/release не выполнялись. Системный firewall не изменялся.

Private enum/legacy callback counts, Stop/abnormal reentry/diagnostic/native/OS
acceptance OPEN. Native RPC/total Stop latency, established-flow reevaluation,
power-loss или continuous barrier против noncooperating writer не доказаны.
Diagnostic WFP lease ещё держит mutex через actor/native callbacks; следующий
срез — original per-lease pending/callback custody без новой policy и без открытия
native gate. FullPC00-09/old9/72tasks/100OPEN56 product NOT_RUN+2 Git-only checks
сохранены; TUN unavailable/schema2 unsupported/rootNETGUARD001 OPEN.

<a id="pc-v495-diagnostic-lease-calls"></a>
## V2-495 — один original diagnostic call, callback вне metadata mutex

**Статус:** source/unverified; PC-01/TH-01, не compile/concurrency/native приёмка.

### Дефект и существующие владельцы

Acquire/Validate/Close удерживали lease.mu через actor.Descriptor IPC, image/
interface queries и Windows WFP calls. Это задерживало Close и допускало
reentrant wait на том же lease. Исследованы actor original read slot,
NativeTUNCapture owner-before-Acquire/revoke и existing core.OperationSource.
Lease/native/global owners и fixed six-condition one-flow rule сохранены.

### Одно исходное действие

New private wfp_tun_lease_operation_windows.go хранит one admitted call с
исходными ctx/deadline/stage и permanently revocable OperationSource. Original
pointer/pending/fault/flags защищаются metadata mutex; native payload принадлежит
только admitted caller. Context wrapping, actor/image/interface/native и final
context fence выполняются вне lease.mu. Metadata global claim и Close identity
commit оба используют lease→global order, без внешних callbacks под locks.

Concurrent/reentrant вызов возвращает Busy+Cleanup без очереди. Close сначала
seals и отзывает original source; pending operation не разрешает native Close.
При её обычном завершении можно выполнить следующий Close; watchers, background
wait, detached callbacks, timers и goroutines не добавлены. Opened публикуется
только в normal Acquire finalization после свежего scope/deadline/seal check.

Direct deferred finishCall требует normal body и final fence return. Panic/Goexit
в callback либо final context check удерживает exact call/fault/ctx/source/stage
и seals lease. Normal call completion не равен native cleanup: original native
outputs остаются в том же lease; existing lower Open/Add/Close guards запрещают
закрытие неизвестного handle или его замену. Compatibility Acquire сохраняет
partial owner при claimed либо unresolved original call.

### Source и отрицательная очистка

Optional call key только сужает existing wfpOperationScope, включая native
positive session Open/filter Add. Старый/nested ключ отвергается, а не заменяется
новым source. Никакой forwarding/Ready authority не создаётся. Independent
Close/Free/Abort/Revoke/Restrict/Release остаются negative cleanup.

Actual dynamic-session Close идёт вне mutex; только normal callback result,
same global owner и native closed/handle0/callsReturned разрешают metadata unclaim.
Pure allocation/busy refusal не закрывает foreign owner. Unknown callback/session
retirement сохраняется, positive operations после Close seal не принимаются.

Last scope check не атомарен с последующим syscall: ранее admitted native call
может гоняться с seal, продолжаться или завершиться после него. Outputs остаются
в original pending owner. IPC/RPC preemption, hard Close deadline или мгновенная
остановка native queries не заявлены. Query/image paths ещё могут продолжать
bounded work до normal return/deadline; далее их scope routing нужно согласовать
с independent negative cleanup, не возвращая истёкшему Grant право veto cleanup.

### Доказательство и остаток

3Go4docs/6existing1new/manual3 source diffs1new/gofmt3 exit0/10 reviewed inputs/
0cached SDK;859 prior named hashes0drift. q87/s95/261 cached originals/190 verbatim/
59 owned mirrors неизменны. Private helper/Acquire-attempt/Validate panic/busy/
callback-count/fixture/concurrency/native/OS acceptance OPEN. Tests/fixtures не
добавлялись/не менялись; tests/build/vet/typecheck/harness checks/product/native/
admin/live/network не запускались; commit/push/release не выполнялись.
FullPC00-09/old9/72tasks/100OPEN56 product NOT_RUN+2 Git-only checks сохранены;
TUN unavailable/schema2 unsupported/rootNETGUARD001 OPEN.

<a id="pc-v496-wfp-query-source"></a>
## V2-496 — original query source, readonly Begin и cleanup polarity

**Статус:** source/unverified; PC-01/TH-01, не compile/ABI/native приёмка.

### Найденные пути

EnumCreate/EnumRead, provider/sublayer и AppID запросы в основном проверяли
ctx.Err(). Отзыв lease source мог не останавливать следующие queries. Readonly
Begin не получал explicit read ctx; standalone reader открывал session через
Background. Простое применение positive gate к любому чтению нарушило бы
independent negative Revoke/Restrict/Release при отозванном Grant context.

### Original batch и scope

Native session сохраняет original queryBatch; batch — original session backlink.
Они устанавливаются с original transaction до native Begin и сохраняются при
отдельных readonly snapshots. QueryScope проверяет эти backlinks и original
queryActive. Fence активируется только при однократном bind и закрывается в
defer исходного writer после всей финализации, включая panic/Goexit. Старый
batch не остаётся открытым для query admission и не bind повторно.

Polarity берётся из actual original batch.action, не из ctx flag. Для positive/
unbound readers действуют original App/lease sources и real deadline. Для actual
negative batch проверяются caller и original negative ctx.Err-only scopes;
отозванные Grant value sources не veto cleanup. Query fence не выдаёт Add,
forwarding, Ready или authority на замену native owner. Positive mutator gates
не ослаблены. Циклы/query receipts остаются bounded на том же владельце.

### Native границы и результат

- Actual readonly Begin хранит readContext в original transaction; checks после
  Find/перед native Begin и после recorded return. Поздний Begin остаётся в
  original receipt и может быть independently aborted. Оба production readonly
  consumers обновлены; legacy private wrappers сохранены без новой приёмки.
- EnumCreate/каждый EnumRead/object/AppID проверяют source перед вызовом и после
  записи native outputs, затем final scope после original cleanup. Scope error
  не теряет исходную allocation/page/enum и не превращается в отсутствие policy.
- Original Free/Destroy/Abort/Close выполняются независимо от source и прежними
  owner slots/return guards. Rules/partial object/AppID facts обнуляются при error.
- Standalone read session получает caller ctx; после known original Close
  проверяется actual positive scope. Новый Background не продлевает старый запрос.

### Доказательство и границы

6Go4docs/all10existing, manual6 source diffs/0new, gofmt6 exit0,11 reviewed inputs/
0cached SDK;860 prior named hashes0drift. q87/s95/261 cached originals/190 verbatim/
59 owned mirrors неизменны. Native ABI payload structs/API и bounds32 rules/
16 pointers/8 conditions не изменены; новые поля — Go ownership receipts.

Last check не атомарен с syscall: уже admitted call может завершиться после
source revoke. Hard RPC preemption/time/RSS/continuous policy не доказаны;
negative scope намеренно остаётся ctx.Err-only. Private queryObject signature,
error types/readonly wrappers/fixture/ABI/concurrency/native/OS acceptance OPEN.
Tests/fixtures не добавлялись/не менялись; tests/build/vet/typecheck/harness checks/
product/native/admin/live/network не запускались; commit/push/release не выполнялись.
Далее PC-01: native condition/AppID decoder bounds и partial-error propagation
без замены original native output owners или открытия native gate.
FullPC00-09/old9/72tasks/100OPEN56 product NOT_RUN+2 Git-only checks сохранены;
TUN unavailable/schema2 unsupported/rootNETGUARD001 OPEN.

<a id="pc-v497-wfp-appid-codec"></a>
## V2-497 — pure decoder и единый AppID/native filename codec

**Статус:** source/unverified; PC-01/TH-01, не compile/Unicode/ABI приёмка.

### Подтверждённый source issue и отклонённая гипотеза

Blob decoder ограничивал AppID65522bytes в UTF16, а producer/query guard сравнивал
len(string) с32760: это bytes UTF8/WTF8, не UTF16 units. Pure rule/condition decode
находился в I/O reader; GetAppID output представлялся искусственным condition.

Исходники pinned x/sys делегируют UTF16 conversion в syscall. Фактический Go source
установленной системы сохраняет unpaired surrogates через WTF8. Первоначальная
гипотеза о replacement-normalization до правки отвергнута; сообщение уточнено.
Урок: проверять реальную toolchain implementation, не навязывать utf8.ValidString
или surrogate rejection Windows identity. Проверка исходников не равна runtime
проверке либо гарантии поведения другого компилятора/версии поставки.

### Общий codec и сохранённая семантика

- New pure wfp_policy_decode_windows.go отделяет native value decoding от I/O.
  Owned-rule decoder и scalar union prefix перенесены без изменения текста
  после EOL normalization; active uint8/uint16/uint32 widths, pointer members,
  flags/raw-context и unused padding semantics сохранены.
- Существующий project cap65522bytes равен32761 UTF16 units включая NUL. Text
  prebound98280bytes ограничивает input conversion; authoritative limit проверяет
  actual UTF16 units. Это project limit, не обещание полного Windows max-path.
- Encoder требует непустой text, platform no-NUL conversion и exact text→units→text.
  Decoder требует non-null data, even size4..65522, один terminal NUL без внутренних
  NUL и exact units→text→units включая NUL. WTF8 не запрещён; lossy/noncanonical
  text отклоняется без сторонней Unicode normalization.
- Persistent/diagnostic AppID producers и native filename query применяют codec.
  Input byte bound проверяется до filepath parsing. Buffers остаются в original
  write/native owner до Add; native query allocation остаётся в original reader.
- GetAppID output читается как actual byte blob; image/codec/standard causes
  сохраняются. Existing final scope/Free error обнуляет outputs. Identity-only
  explicit recovery повреждённой owned policy не требует condition decode.

### Доказательство и пределы

4Go4docs/7existing1new, manual4 source diffs1new/gofmt4+1 exit0,9 reviewed project
inputs и4 local source inputs (pinned wf/xsys и установленный Go). Source transfer
receipt подтверждает неизменный owned-rule decoder/scalar prefix; product code
не исполнялся.860 prior named hashes0drift; q87/s95/261 cached originals/190 verbatim/
59 owned mirrors неизменны, deps/версии/installed SDK не менялись.

Raw pointer accessibility/ABI/lifetime требуют реальных native outputs; header
bounds не делают parser безопасным для произвольных адресов. Codec allocations,
native RPC/RSS/performance, Unicode/WTF8/toolchain/fixture/native/OS acceptance
не проверены. Native structs/APIs/owner/Free/query source и closed gate прежние.
Tests/fixtures не добавлялись/не менялись; tests/build/vet/typecheck/harness checks/
product/SDK/native/admin/live/network не запускались; commit/push/release не выполнялись.
Далее PC-01: источники AppID и original executable/image custody в base barrier
и diagnostic lease, без нового path-based grant или открытия native gate.
FullPC00-09/old9/72tasks/100OPEN56 product NOT_RUN+2 Git-only checks сохранены;
TUN unavailable/schema2 unsupported/rootNETGUARD001 OPEN.

<a id="pc-v498-wfp-application-image"></a>
## V2-498 — оригинальный exe/image owner при WFP AppID (2026-11-17)

### Реализация в исходниках

- Новый core.ApplicationImageLease компонует existing verifierApplicationLease;
  original current exe/ancestors, ACL/NTFS/FileID/PE/process-path checks остаются
  прежними. Pure allocation до Prepare; TryLock, atomic retirement до Close,
  sticky abnormal fault, пустой Path при ошибке; partial owner не заменяется.
- Native WFP session сохраняет exact image owner до подготовки. AppID query
  окружён Path validation и original query source checks. Base Arm перенесён
  внутрь original writer и отвергает пустой/несовпадающий expected current path.
- Root verify использует один original session/image для AppID и policy read.
  Diagnostic engine AppID берётся под удерживаемым current App image; helper
  остаётся под существующим original actor/image/process owner. Причины ошибок
  AppID/Free/cleanup в diagnostic возвращаются вместе с domain error.
- Writer освобождает image после original transaction/query finalizers, до unlock;
  pending native query/write/transaction удерживает его. Full-session deny-delete
  намеренно не добавлен: он мешал бы следующему owned update rename. Reader,
  diagnostic и manual Release закрывают native session перед image retirement.
- Negative writes выполняются до image cleanup. Только exact verified original
  negative batch получает value-blind context внутри image inspector: original
  Done/Err/Deadline сохранены; wrapper не передаётся Add/policy routing. Main
  query fences проверяют неизменённые caller и original batch.
- Final source fence следует после image Close. Deferred failed-grant retention
  и восстановление временно очищенного Restrict predecessor выполняются также
  при abnormal cleanup/Goexit до writer unlock. Нельзя принять grant после late
  image/source error или заменить image после неизвестного Close.

### Доказательство и остаток

7 Go +4 docs,9 existing/2new; вручную прочитаны7 final source diff, включая2new.
Touched-only gofmt7+1+1 exit0;16 project inputs frozen,0 local SDK reads;
q87/s95/261 cached originals/190 verbatim/59 owned mirrors unchanged.
Evidence: `.artifacts/memory/bootstrap-v498-appid-custody-`:
`final-source-diffs.json`, `formatting{,-accepted,-guard}.json`,
`manual-source-review.json`, `source-evidence.json`, `final-identities.json`.
Это source/manual evidence, не Go execution, compile или Windows acceptance.
Current disk lease не доказывает trusted release/hash/kernel loaded sections,
не защищает от trusted owner/admin/kernel и старого writable mapped view.
Общий image+WFP atomic snapshot, native call preemption/time/RSS/throughput,
Unicode/compiler/private fixtures, Win32 ABI, crash/Stop/update/OS ещё OPEN.
Image path/caps/ACL/native-close guards не расширялись; новые API Win32 и
зависимости не добавлены. Persistent policy/file lifetime различаются; следующий
gate заново удерживает и проверяет current disk image. Full PC-00…09/all9/
72tasks/100OPEN56 product NOT_RUN+2 historical Git checks сохранены.
No tests/fixtures/build/vet/types/harness/SDK/product/native/admin/live/network/
new commit/push/other-role/chat execution. TUN unavailable/schema2 unsupported/
root NETGUARD-001 OPEN. Далее original helper descriptor/image custody при
deactivation и финальной диагностической проверке.

<a id="pc-v499-actor-final-source"></a>
## V2-499 — финальный scope дескриптора и original actor revocation (2026-11-17)

В finalizer Descriptor оставались caller/original contextError callbacks под
actor.mu. Вынесены в original finishDescriptor: исходное чтение pending до
normal body/cancel/final source fence. Внутри mutex только metadata, atomic
revocation, time/deadline и passive original process done-channel observation;
engineProcess.exited не вызывает host.Wait/native API. Abnormal body/cancel/
final fence сохраняет exact contexts/cancel/stage/pending/fault, отзывает актёра,
обнуляет descriptor и возвращает typed scope/cleanup error. Нормальный cancel
не подменяет исходный caller; actual derived deadline сохранён для value commit.

Actor Revoked — только one-way negative fence. deactivate выставляет его до
mutex и обоих existing negative cleanup. Per-stage query checks проверяют этот
же актёр/caller/original operation до IPC/image/ancestor/path работы и после.
Original diagnostic WFP call.current теперь также читает atomic revoke исходного
actor; false не заменяет Descriptor/runtime/image/process proof. Нового source
owner, таймера, watcher, goroutine или API Win32 нет. Negative Free/Abort/Close
не зависят от этого positive fence; root по-прежнему пытается оба ingress/filter
revocation до descriptor-retired gate и не освобождает helper/socket/image при
pending/fault. Поздний native result остаётся у original native/lease owner.

2 Go +4 docs, все6 existing; вручную прочитаны2 source diff; gofmt2 exit0.
9 project inputs frozen,0 SDK reads; q87/s95/261cache190verbatim59owned unchanged.
Evidence: `.artifacts/memory/bootstrap-v499-actor-custody-` с source-diffs,
formatting, manual-source-review, source-evidence и final-identities JSON.
Это source/manual evidence: tests/fixtures/build/vet/types/harness/SDK/product/
native/admin/live/network/new commit/push/other-role/chat execution не было.
Callback/native preemption, last-check→syscall atomicity, compiler/private
concurrency fixtures/ABI/OS/Stop/update/crash/RSS/speed остаются OPEN. Full
PC-00…09/all9/72tasks/100OPEN56 product NOT_RUN+2 historic Git checks сохранены;
TUN unavailable/schema2 unsupported/root NETGUARD-001 OPEN. Далее final native
observation publication после original helper/filter/ingress retirement.

<a id="pc-v500-observation-operation"></a>
## V2-500 — original operation metadata native observation (2026-11-17)

Trace подтвердил existing ordering: dual ingress/filter revoke → helper release/
EOF/zero exit → original Job/I-O/image retirement → HTTP reparse/runtime check.
Новый дефект premature publication не заявляется. Downstream claimLocked и
source busy/preparation checks читали произвольный original operation context
под Engine/source mutex. Новый private nativeTUNOperation заранее сохраняет
exact original Done channel/deadline и тот же private core source binding.
Getters используют passive channel/clock/value/atomic pointers, не вызывая
original context; original reference остаётся удерживаемой. Snapshot raw/source
checks выполняются вне locks, abnormal capture не возвращает пригодный view.

Producer создаёт view source operation после actual helper retirement;
probe — view claiming operation до Engine lock. Actual RPC ctx и source binding/
таймауты прежние: view никогда не parent для context constructors и не RPC ctx.
Это исключает добавление скрытого cancellation observer через новую обёртку.
Err — conservative canceled/deadline predicate; custom Err/Cause и посторонние
Value keys в metadata view не поддерживаются. Новый source/health cache или
positive authority не создаются; прежние receipt lifetime/Done identity/one-use
checks обязательны. Known preparation error сохраняет parent permit clearing
(проверены pure pointer assignments) и source burn; abnormal path burns после
Engine unlock. Public API/descriptor/worker/native acceptance не расширены.

3 Go +4 docs,6existing/1new; manual3 source diff1new, gofmt3+1 exit0,
13 project inputs/0 SDK reads; q87/s95/261cache190verbatim59owned unchanged.
Evidence: `.artifacts/memory/bootstrap-v500-observation-retirement-`:
accepted-source-diffs, formatting{,-accepted}, manual-source-review,
source-evidence, final-identities JSON. Missing errors import исправлен при
ручном review; компиляция/Go execution не проводились. Actual RPC ctx/IPC под
Engine lock и legacy private/direct fixtures не покрыты этим узким исправлением;
их callback/OS/cancellation/concurrency/resource acceptance остаётся OPEN.
No tests/fixtures/build/vet/types/harness/SDK/product/native/admin/live/network/
new commit/push/other-role/chat execution. Full PC-00…09/all9/72tasks/100OPEN56
product NOT_RUN+2 historical Git checks; TUN unavailable/schema2 unsupported/
root NETGUARD-001 OPEN. Далее native preparation/commit context gates.

<a id="pc-v501-native-preparation-scope"></a>
## V2-501 — parent native preparation scope/consume (2026-11-17)

Три production parent paths используют V500 metadata view до Engine.mu:
nativePreparationCurrent, ValidateNativeForwardingContext и CommitPreparedNative.
Done/deadline/core source pointers исходные. Actual Manager commit key/current/
owner/generation проверяются original ctx вне Engine.mu; RPC получает тот же
actual ctx. View не parent для новых contexts и не заменяет authority/RPC.

Final consume захватывает exact original permit под Engine.mu и снимает оба
parent slots до nativePermitPreparationLocked. Defer unlock теперь до проверок.
Read-only helper использует текущий slot; consume — свой captured original,
без принятия позднего совпадающего permit. Проверен busyLocked: slot equality
не требуется, original process/source/instance/deadline обязательны. Snapshot
failure в Commit всё равно проходит parent burn. Acceptance gate false сохранён.

Worker draft пересмотрен по фактическому caller: prepareForwarding выполняется
в runLocalWorkerProbe под outer Engine.mu. Перенос только inner context checks
не устраняет внешний lock и может сдвинуть callbacks перед one-use burn. Эти
правки не приняты. Own native_tun_permit.go восстановлен byte-identically только
после сравнения с exact expected own substitutions; worker consume original
body сохранён. Worker request stdlib timeout context подтверждён по serve source,
но generic callback/entropy/fixture/OS приёмка из этого не следует.

Actual1 Go +4 docs;6 existing snapshots,0new; manual1 source diff+1 unchanged
source, initial gofmt2/final1 exit0;10 project inputs/0SDK; q87/s95 unchanged.
Evidence: `.artifacts/memory/bootstrap-v501-native-commit-scope-` source-diffs,
formatting{,-final}, manual-source-review, source-evidence, final-identities JSON.
Source/manual only: tests/fixtures/build/vet/types/harness/SDK/product/native/
admin/live/network/new commit/push/other-role/chat execution не было. Actual
Engine/RPC/context/selector/worker entropy/abnormal pre-lock ordering/private
fixtures/compiler/ABI/OS/Stop/concurrency/resources/performance остаются OPEN.
FullPC00-09/all9/72tasks/100OPEN56 product NOT_RUN+2 historical Git checks;
TUN unavailable/schema2 unsupported/root NETGUARD-001 OPEN. Далее worker token
preparation/emission/consume с outer-lock и one-use burn ordering.

<a id="pc-v502-worker-native-entropy"></a>
## V2-502 — worker native preparation и admission mutex (2026-11-17)

Actual caller runLocalWorkerProbe удерживает original Engine.mu. Внутренний
prepareForwarding дополнительно держал admission.mu во время context callbacks,
rand.Read и создания token. Теперь short initial section снимает old slots,
burns exact native.prepared до любого callback, проверяет original native fact,
запоминает min(control budget, receipt expiry) и gate subject/selector/receipt.
Вне admission.mu выполняются metadata snapshot, entropy и pure candidate
allocation. Ошибка/abnormal completion не публикует candidate и не снимает burn.

Final section проверяет те же binding/admission/native pointers, subject/selector/
epoch, original gate subject/selector/nativeReceipt, closed/open/scope/epoch,
vacant ordinary/native slots и zero pendingUntil, prepared/confirmed IPv4 fact,
original nonce receipt, source и fixed deadlines. Публикация только после этих
value checks; поздний другой slot не перезаписывается, ожидание не продлевает
expiry. Candidate не открывает ingress: emission и compiled final consume ещё
отдельны, nativeTUNForwardingAccepted=false. View не context parent/RPC ctx.

Negative close/fault теперь может захватить admission.mu во время entropy и
seal исходный gate. Проверен actual sealRoutesLocked → revokeRegistrationLocked:
open=false и closed=true устанавливаются под mutex; ранняя lost-update гипотеза
отвергнута, дублирующий revival-флаг не добавлен. Outer Engine.mu остаётся:
general Engine/RPC/selector/callback/entropy preemption этим не доказаны.

1 Go +4 docs,all5 existing; manual1 source diff, gofmt1 exit0;
11 project inputs/0SDK; q87/s95/261cache190verbatim59owned unchanged.
Evidence: `.artifacts/memory/bootstrap-v502-worker-native-preparation-`
source-diffs/formatting/manual-source-review/source-evidence/final-identities JSON.
Source/manual only: tests/fixtures/build/vet/types/harness/SDK/product/native/
admin/live/network/new commit/push/otherroles/chats не запускались. Private
concurrency fixtures/compiler/ABI/OS/Stop/entropy/native time/RSS/speed OPEN.
FullPC00-09/all9/72tasks/100OPEN56 product NOT_RUN+2 historic Git checks;
TUN unavailable/schema2 unsupported/root NETGUARD-001 OPEN. Далее ordinary
issuance/native emission/consume и их one-use/context/slot ownership.

<a id="pc-v503-forwarding-issuance"></a>
## V2-503 — original ordinary issuance и общий pending slot (2026-11-17)

Existing ordinary issue держал admission.mu во время rand.Read/encoding и
создавал pendingUntil после ожидания. Метод с прежней private API выделен в
forwarding_issue.go. Original forwardingIssue — только metadata попытки;
initial lock снимает old slots, проверяет epoch/closed/scope/subject/selector,
сохраняет existing admitted branch без entropy/new token/deadline. Для нового
issue exact private pointer и fixed control expiry публикуются до entropy.

Rand/encoding выполняются без admission.mu. Final lock требует тот же attempt,
epoch/closed/open/scope/gate subject/selector/nativeReceipt, пустые slots и fixed
deadline. Shared clearPendingLocked отзывает attempt также при новой попытке,
consume, selector/probe/close/route-fault/TUN cleanup. Deferred completion/error/
panic/Goexit cleanup снимает только свой pointer, не successor или published
token. Native permit current/final candidate дополнительно требуют no ordinary
issuer, чтобы late native result не перезаписал новое ordinary preparation.

Attempt не authority/report/native resource receipt. Actual consume/private
worker API/wire прежние; caller runLocalWorkerProbe сохраняет outer Engine.mu.
Issue не получил context parameter: general Engine/RPC cancellation/selector/
entropy preemption и measured resource/speed bounds этим не доказаны. Fixed
expiry до ожидания может только сократить старое окно; новых timer/watcher/queue
нет. TUN acceptance gate false и separate native emission/final consume сохранены.

3 Go +4 docs,6existing/1new; manual3 source diff1new, gofmt3 exit0;
10 project inputs/0SDK; q87/s95/261cache190verbatim59owned unchanged.
Evidence: `.artifacts/memory/bootstrap-v503-forwarding-issuance-`
source-diffs/formatting/manual-source-review/source-evidence/final-identities JSON.
Source/manual only: tests/fixtures/build/vet/types/harness/SDK/product/native/
admin/live/network/new commit/push/otherroles/chats не запускались. Compiler/
private concurrency fixtures/ABI/OS/Stop/resources/performance остаются OPEN.
FullPC00-09/all9/72tasks/100OPEN56 product NOT_RUN+2 historical Git checks;
TUN unavailable/schema2 unsupported/root NETGUARD-001 OPEN. Далее native token
emission/consume, burn-before-callback и original slot/source custody.

<a id="pc-v504-native-token-handoff"></a>
## V2-504 — original native token emission/consume (2026-11-17)

Emit под admission.mu сначала burns original emitted flag и проверяет candidate.
Metadata snapshot выполняется без admission.mu; final section требует того же
nativePending pointer, emitted/current/source/deadlines. Failed/abnormal finalizer
снимает только matching original candidate, не новый issuer/consumer.

Consume выделен из native_tun_commit.go в native_tun_consume.go. Initial section
проверяет original candidate и снимает prepared slot до любого context callback,
сохраняя compiled gate/emitted token/subject/selector/epoch/nonce. Eligible gated
attempt хранится в exact private workerTUNConsumption с original permit/receipt;
metadata snapshot идёт без admission.mu. Final section требует тот же record,
vacant candidate slot, same gate receipt, all fixed source/state/deadline checks
до open. Post-open metadata source check сохраняет исходное fail-closed поведение.

Shared clearPendingLocked отзывает также consumer. Его own-only finalizer не
удаляет successor/ordinary issuer; ordinary/native publication/current predicates
требуют consumer vacancy. Fixed matchesLocked отделён от actual slot identity:
положительное value match само по себе не authority, caller обязан удерживать
exact nativePending или consumption record; stale pendingUntil также rejected.

Constant nativeTUNForwardingAccepted=false подтверждён по actual source, public
worker API/RPC/Manager/WFP/process/image custody не расширены. Metadata component
comment уточнён: worker сохраняет outer Engine operation lock, исправление
admission/source mutex не доказывает general Engine/RPC callbacks/preemption.
View никогда не RPC ctx/context parent; новых timer/watcher/worker/dependency нет.

6 Go +4 docs,9existing/1new; manual6 source diff1new, gofmt5+1 exit0;
12 project inputs/0SDK; q87/s95/261cache190verbatim59owned unchanged.
Evidence: `.artifacts/memory/bootstrap-v504-native-token-handoff-`
source-diffs/formatting{,-metadata}/manual-source-review/source-evidence/final-identities JSON.
Before target-extension comment accepted own review checkpoint1328 with original
frozen source SHA. Source/manual only: tests/fixtures/build/vet/types/harness/
SDK/product/native/admin/live/network/new commit/push/otherroles/chats не было.
Last-check/open atomicity, outer Engine/RPC/selector/context/native cancellation,
private fixtures/compiler/ABI/OS/Stop/RSS/speed остаются OPEN. FullPC00-09/all9/
72tasks/100OPEN56 product NOT_RUN+2 historical Git checks; TUN unavailable/schema2
unsupported/root NETGUARD-001 OPEN. Далее worker report/ACK/permit handoff при
late source/cleanup/output failure и сохранении original owner.

<a id="pc-v505-worker-prewrite-permit"></a>
## V2-505 — current worker permit после JSON encoding (2026-11-17)

Parent exchange уже проверяет original wire version/ID/action/reply shape,
source/ancestor/owned I-O completion и передаёт failed original process в reaper.
Эти механизмы сохранены. Child после post-state query сериализовал token reply
без повторного чтения actual private candidate. writeWorkerFrameGuarded теперь
использует тот же encoder/length check, затем optional synchronous owner guard
до первого header/body byte; старый writeWorkerFrame передаёт nil guard.

Child включает guard только для ok probe reply с fresh token. Read-only
workerReplyPermitCurrent повторяет finite report/token correlation и actual
private gate: cleanup/closed/open/scope/epoch/subject/selector, ordinary pending
token/deadline/vacant slots либо exact emitted native candidate/nonce/receipt/
deadline. SDK/selector/native/context callbacks под этими value locks нет;
candidate не меняется. Нельзя сделать authority только строкой token/report.
Guard failure не пишет header и идёт в existing child output-failure exit2;
parent сохраняет original process/pipes/Job до собственной retirement приёмки.
Negative typed restrictions/no-token frames/default codec/wire не изменены.

Это не command-context freshness: worker normal execution context уже отменён
перед post-state. Проверяется actual candidate, а не придумывается новый timeout.
Guard→write не atomic, revoke/expiry может произойти позже, OS write может
блокировать/вернуться поздно. Parent source/ancestor/read/consume gates остаются
обязательными; ACK не cleanup/health/Manager proof. Existing length check не
доказывает peak encoding RSS/прерываемость. Новых timer/watcher/worker/API нет.

3 Go +4 docs,6existing/1new; manual3 source diff1new, gofmt3 exit0;
12 project inputs/0SDK; q87/s95/261cache190verbatim59owned unchanged.
Evidence: `.artifacts/memory/bootstrap-v505-worker-reply-source-`
source-diffs/formatting/manual-source-review/source-evidence/final-identities JSON.
Source/manual only: tests/fixtures/build/vet/types/harness/SDK/product/native/
admin/live/network/new commit/push/otherroles/chats не запускались. Compiler/
private partial-output/concurrency fixtures/ABI/OS/Stop/kernel/RSS/speed OPEN.
FullPC00-09/all9/72tasks/100OPEN56 product NOT_RUN+2 historic Git checks;
TUN unavailable/schema2 unsupported/root NETGUARD-001 OPEN. Далее partial output/
original reply retirement и parent permit source/outcome continuity.
