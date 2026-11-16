# PC-05 — завершение DNS-задачи и повторный Start

2026-11-05, V2-215, Windows **source/unverified**. Полный
[PC00…09](../planning/knowledge-application/PC-PLAN.md), продолжение
[DNS handoff](PC-03-WORKER-DNS-HANDOFF.md) и
[observer availability](PC-03-OBSERVER-AVAILABILITY.md).

## Исправляемый пробел

Ранее original DNS task сохранял terminal fault, но Start его не учитывал.
После повторного подключения можно было опубликовать running/health при уже
завершившемся обработчике. Cleanup по captured cc не связывался с конкретным
DNS owner: во время нового Manager.Start root ещё может хранить прежний cc.
Простое ожидание task.done внутри operationMu создало бы взаимное ожидание с
его finalizer, которому нужен тот же mutex.

## Source-изменения

1. `dns_network_handoff.go`: original task имеет отдельные task-entry receipt,
   running lease и explicit stopping flag. Pending допускается до task entry,
   но не после fault/stop/cancel. Terminal exit отзывает lease и pending под
   owner.mu **до** cancel и ожидания lifecycle mutex. Panic и неожиданная
   отмена App context — local observer failure; explicit shutdown отличается.
   Fault старого объекта не сбрасывается. Done закрывается после finalizer,
   даже при его caught panic; это завершение задачи, не engine cleanup proof.
2. `dns_observer_lifecycle.go`: ordinary Start ограниченно ждёт terminal task
   **до** operationMu. После join повторяет intent/App lifetime guard. Timeout
   не разрешает замену. Initial task entry можно ждать под operationMu: этот
   receipt выставляется до любых lifecycle действий task, а не в finalizer.
3. `corebridge.go`: действующий lifecycle подтверждает previous.Manager.Stop
   и RetireCaptureHelpers до reconciliation. Проверяются те же runtime manager/
   engine, actual App/netWatcher owner и незакрытый stop channel. Только после
   original task.done разрешён один successor на App.ctx; второй poller не
   создаётся. Failed entry wait сохраняет именно нового owner, без rollback
   или следующей замены. NetWatcher хранит максимум одного confirmed predecessor.
4. Runtime actor получает exact DNS owner вместе с manager/engine. Lease
   проверяется до публикации нового actor и повторно после Manager.Start перед
   cc/metrics. Ни IPC, ни Manager.Stop/Start, ни metrics parse не выполняются
   под DNS lease. Confirmed Stop очищает runtime lease; error сохраняет owner.
5. Terminal failure снимает CoreReady/ProbeRunning/native cache только у того
   runtime actor, который связан с этим DNS owner, и отменяет захваченную App
   operation вне value locks. Local cleanup_unconfirmed подавляет автоматический
   retry; existing failCore/stopFailure выполняет реальную очистку после получения
   operationMu. Новый local error участвует в localRetirementFailure, поэтому
   проблема task/cleanup не запускает перебор каналов и не пишет channel FAIL.
6. `observer_publication.go`: non-safety publication, включая health/Started/
   Applied/retained Running, требует lease actual runtime DNS owner. Короткий
   guard сериализуется с terminal revocation; Manager/clock checks происходят
   **до** lease. Health/policy/UI value commit под ним; event delivery после
   освобождения. Negative safety publication не зависит от доступности observer:
   независимая опасность по-прежнему может запустить остановку.
7. `protection.go`: новые App operations наследуют исходный App.ctx через прежний
   network scope, без Background fallback при отсутствующем/отменённом App lifetime.
   Engine lifetime и actual retirement остаются отдельными; временный operation
   context не превращается в новый lifetime committed worker.
8. `stopNetWatch` отзывает lease до cancel, затем ждёт тот же done. Closed watcher/
   App cancellation/closing запрещают successor. Ошибки DNS IPC, cleanup и retry
   используют существующие пути; новое решение о Ready, transport,
   профиле, TUN/WFP или допуске пользовательского forwarding не создаётся.

## Границы и открытая приёмка

Это cooperative source lifecycle. Task entry не доказывает обработку OS events,
engine/network health, native admission или eventual completion. Двухсекундные
ожидания не ограничивают произвольные mutex/syscall/cancel/cleanup. Failure после
pre-join может потребовать следующего явного Start; незавершённый finalizer не
обходится. Зависший **живой** task не объявляется terminal по одному timeout.
Original poller ещё не имеет отдельного done/fault reconciliation; это следующий
source P0. Parent-context cancellation/event delivery и kernel forwarding не
образуют atomic transaction с publication lease.

Нужны fixtures для nil/canceled App lifetime, missing/stopped watcher, task entry,
panic/cancel/partial finalizer, failed Manager/helper cleanup, pre/post join races,
Stop/Close/profile cutover/intent supersession, stale lease/unsafe observation,
same-owner restart/retained rollback и bounded predecessor retention. Существующие
fixtures не изменялись; прежние nil-App/runtime без DNS lease больше не моделируют
production Start. Проверки требуют отдельного разрешения владельца.

Только source read/edit, gofmt6, manual diff/byte-SHA inventory:
`.artifacts/memory/dns-owner-source-evidence.json`. Tests/fixtures/build/compile/
vet/typecheck/parser/check/doctor/install/generation/SDK/worker/product/native/
browser/admin/live/network/server/owner-values/другие роли/чаты/субагенты не
запускались. Никакого compile/runtime/OS/resource PASS. IPC9/engine Job ceiling8
прежние; native acceptance const=false, TUN unavailable, schema2 unsupported.
Весь PC00…09 и прежние FR/TH/OFF/COM/RQ/TR/TH00…08, SDK/W3b/UX/release/
comparative остаются OPEN. Далее source — original poller completion/stop owner,
затем resume/actual family-path integration в рамках прежнего protection gate.
