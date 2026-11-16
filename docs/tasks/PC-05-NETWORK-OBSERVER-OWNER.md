# PC-05 — original owner цикла наблюдения за сетью

2026-11-05, V2-216, Windows **source/unverified**. Продолжение
[DNS task recovery](PC-05-DNS-OBSERVER-RECOVERY.md), полный
[PC00…09](../planning/knowledge-application/PC-PLAN.md).

## Пробел

Poller запускался без отдельного done/fault owner. Closed stop channel означал
только запрос остановки, но не возврат original collector/event stack. DNS task
мог оставаться формально доступным после отказа своего producer. Простая замена
по timeout могла бы создать параллельные polling loops или оставить новую DNS
задачу на уже завершившемся producer.

## Изменения в исходниках

1. `network_observer_lifecycle.go` владеет original poller: App.ctx/cancel,
   task-entry/done, running/stopping/fault и observed. Wrapper охватывает прежний
   poller loop, включая collector и event delivery; panic/unexpected App cancel
   отзывает running/observed до cancel и finalization. Done закрывается после
   возврата старого stack/finalizer. Explicit shutdown отличается от local fault.
2. DNS task получает immutable pointer своего poller. Existing DNS lease теперь
   берёт poller→DNS guards, поэтому отказ producer закрывает non-safety positive
   publication ещё до lifecycle cleanup. Нет I/O, Engine/Manager/cancel/log/epoch
   callback под этими leases; clock/Manager checks по-прежнему раньше них.
3. Poller fault помечает только actual linked current DNS task, отзывает pending/
   running, применяет существующий exact-runtime negative fence и отменяет App
   operation вне locks. DNS cancellation выполняется даже после caught panic в
   negative fence, её отдельный outcome сохраняется. Existing DNS finalizer и
   failCore/stopFailure владеют actual engine cleanup. Никаких synthetic up/down,
   network epoch, channel FAIL, нового forwarding grant или direct fallback.
4. Start joins terminal poller, затем terminal DNS, вне operationMu с одним общим
   cooperative wait budget. Actual Manager.Stop и capture-helper retirement
   остаются обязательны до reconciliation. Failed DNS при healthy poller меняет
   только DNS; failed producer требует обоих original done до одной новой пары.
   CAS повторяет actual App/watcher/source/DNS/stop/context. Final healthy-source
   check под watcher lock исключает пропуск dependency failure при замене DNS.
5. Successors наследуют App.ctx. Failed entry wait сохраняет именно созданные
   owner objects; timeout не разрешает следующую замену. Late producer failure
   повторно отменяет actual linked candidate, чтобы не оставить его на мёртвом
   source. Хранится максимум один confirmed predecessor каждого вида; unfinished
   original остаётся current. Повторный startup не создаёт новый poller.
6. При замене producer локальный delivered baseline сбрасывается в unknown.
   Это **не** OS event/epoch или Internet proof. Новый loop заново получает sample
   и проходит прежний debounce; unknown/TUN/all-link-down semantics V2-214 сохранены.
   Task entry не равен sample: observed появляется только после actual delivered
   event. Обычные channel-health writes до него запрещены, старый epoch history
   не оживает по одному restart. Независимые unsafe reports сохраняют stop path,
   но не записывают channel-health judgment. Required fresh Manager/probe gates
   не заменяются entry/observed flags.
7. Shutdown отзывает/cancel обе задачи до ожидания. Один общий timer ждёт их
   original done; на timeout повторно читаются те же completion channels.
   Unconfirmed owner сохраняется, журнал говорит об отсутствии receipt, не о
   доказанном завершении. Closed watcher/App cancellation/closing запрещают замену.
8. Оба task переиспользуют общий bounded entry/join helper. Poller entry не ждёт
   operationMu; terminal join под этим mutex запрещён. События не держат poller
   lease через MIB, epoch callback, журнал или DNS enqueue. Guards вокруг event
   delivery — value checks, не atomic transaction с Stop/kernel.

## Что этим не доказано

Running означает original task entry без terminal fault, не eventual progress.
Зависший collector не объявляется завершившимся по таймауту и не заменяется.
Legacy net.Interfaces/Addrs/MIB/UDP lookup ещё не получил общего cancelable I/O
owner; socket Close uncertainty V2-214 не стала подтверждённым cleanup. Done
задачи не доказывает освобождение каждого kernel socket/handle. Нет hard syscall/
callback/mutex/cancel/OS/RSS deadline. Task-entry wait отдельно от общего shutdown
wait; это не измеренный общий startup bound.

Baseline restart/first real event могут отменить текущий Start по существующему
epoch guard; наблюдаемый network gap не выдаёт обещания seamless resume. TTL
bootstrap hint остаётся address hint с отдельной TLS identity, не path proof.
Интеграция first-event/Start, ограниченного automatic recovery, suspend/resume,
actual IPv4/IPv6/UDP/TCP/physical path остаётся OPEN. Далее source — cancellation
и original socket-close custody collector, затем согласованное восстановление.

Source7 Go/4 docs, gofmt/manual source/caller/diff/byte-SHA inventory:
`.artifacts/memory/poller-owner-source-evidence.json`. Tests/fixtures не
добавлялись/не менялись/не запускались. Compile/build/vet/typecheck/parser/check/
doctor/install/generation/SDK/task/collector/worker/product/native/browser/admin/
live/network/server/owner-values/other-role/chat/subagent execution не было.
Нужны fixtures для initial entry/pending, producer/DNS races, failed fence/cancel,
partial Stop/cleanup, timeout/late done, unknown/first sample, unsafe/health guards,
intent/profile/Close/shutdown и retention; legacy private constructors менялись,
fixtures намеренно отложены. Compile/runtime/OS/resource приёмка не выполнена.

IPC9/engine Job ceiling8 прежние; native acceptance const=false, TUN unavailable,
schema2 unsupported. Полный PC00…09 и прежний FR/TH/OFF/COM/RQ/TR/TH00…08,
SDK/W3b/interprocess/durability/UX/release/comparative остаются OPEN.
