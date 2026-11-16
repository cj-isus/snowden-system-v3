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
