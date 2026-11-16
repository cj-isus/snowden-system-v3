# TH-01 / TH-02 — владение системным helper для TUN-наблюдения

Статус: V2-146, 2026-10-24, **исходники, unverified**, только PC/Windows.
Тесты не добавлялись/не менялись/не запускались. Build/compile/vet/check/module/
generation/SDK/product CLI/helper-процессы/native/live не выполнялись. `gofmt -w` пяти Go-файлов
завершился exit0; это не Windows/Job/WFP/protocol evidence.

## Почему это предшествует native-family наблюдениям

Чтение `wfpBaseRuleSpecs`, `spawnEngineProcess` и `tunnelBoundary` показывает:
parent и worker используют один EXE, permit задаётся по его AppID. Поэтому сырой
UDP/TCP DNS из parent сам по себе не подтверждает защищённый native маршрут;
наличие интерфейса с нужным именем тоже не даёт такое полномочие. Это source
вывод, не обнаруженная live-утечка или реализованная служба FR-009…011.

Существующий внешний IPv4 gate запускает SystemDirectory/curl.exe с другим
AppID. До V2-146 он был обычным `exec.CommandContext` с direct-child cancel и
WaitDelay: дерево процесса и его committed memory не имели отдельного Job owner.
Для дальнейшего native helper сначала нужен тот же owned lifecycle, что у worker.

## Общее Win32-владение

Прежний `spawnEngineProcess` стал wrapper существующего общего `spawnOwnedProcess`:
exact absolute executable/argv, CREATE_SUSPENDED/NO_WINDOW, allowlist трёх stdin/
stdout/NUL-stderr handles, unnamed non-inheritable Job и assignment до Resume.
Partial/suspended owner возвращается вызывающему даже при ошибке/panic; context
проверяется перед созданием и до Resume. Engine по-прежнему получает ceiling8
процессов для netsh fallback; новая memory policy или environment ему не назначены.

`CaptureProtectedHelper` — отдельный один protected slot через TryLock, без
очереди, Wails endpoint, shell/PATH lookup, saved PID/OpenProcess/taskkill.
Это не API произвольного запуска команд из профиля; текущий caller выбирает
только абсолютный системный curl. Optional диагностика не может занимать этот
reserved owner: для неё сначала нужна отдельная полоса с собственным бюджетом.

V2-147: [native protocol](TH-02-NATIVE-PROTOCOL.md) добавил отдельную optional
lane с тем же owner/reaper и отдельный Go-helper. Это source foundation;
App sender/image pin enforcement/scope lease ещё не подключены, caller budget
и полный native acceptance по-прежнему обязательны.

V2-148 [image owner](TH-02-NATIVE-IMAGE.md): source image validation/retained handles
и mandatory scoped native wrapper добавлены. Отсутствует actual App session lease/
sender на V2-148; compile/Win32/WFP/resource и полный release acceptance не выполнены.

V2-149 [manual App scope/action](TH-02-NATIVE-SCOPE.md): source caller с полной
before/after привязкой, Stop/epoch context и отдельными bounded family facts.
Mandatory probe/health и reserved helper не обновляются optional наблюдением.
Bindings/UI/native/OS/pair updater/rollback приёмка всё ещё открыта.

V2-150 [Stop ownership](TH-01-STOP-OWNERSHIP.md): source App lifecycle barrier
retire обе helper lanes после Manager.Stop и до Release/replacement; busy/uncertain
resource сохраняется, old queued Stop fenced по ID регистрации. Local cleanup
не запускает channel churn. Это source contract, Win32/Stop/WFP proof отсутствует.

Helper получает active-process limit1, KILL_ON_JOB_CLOSE и128MiB JobMemoryLimit
до Resume. По [Microsoft](https://learn.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-jobobject_extended_limit_information)
этот лимит относится к committed virtual memory Job, а не RSS, CPU, kernel memory
или общему потреблению компьютера. Значение — начальный непроверенный ceiling,
не результат замера. CreateProcess/loader до assignment и Win32 calls остаются
отдельными OS/resource gates; не заявляется жёсткий общий time bound.

## Окружение и результат

Абсолютный application name задаётся отдельно от корректно составленного argv;
число/длина аргументов, NUL и итоговая UTF16 command line ограничены. Helper
получает минимальный Unicode env block только с SystemRoot/WINDIR и cwd системного
каталога. Пользовательские CURL/SSL/proxy/keylog/HOME/PATH/profile variables не
наследуются. Системное окружение пользователя не меняется. Основание Win32
environment/command-line contract — [CreateProcessW](https://learn.microsoft.com/en-us/windows/desktop/api/processthreadsapi/nf-processthreadsapi-createprocessw);
новая конфигурация helper не объявлена совместимой без native приёмки.

stdin закрывается, stderr не публикуется; stdout читается одной goroutine до
Max+1, API cap64KiB, текущий caller cap4KiB. Ошибка/overflow/cancel/panic не выдаёт
частичные bytes как успех. Нужны root Wait, exit code0, pipe completion и полное
retirement через прежний original-handle/Job reaper. Только после empty Job и
закрытия owned handles возвращаются bytes. Deadline повторяется после cleanup.

Неопределённый terminate/reap/close/panic сохраняет точный owner; следующий вызов
может сначала завершить только его, а не создать замену. Cleanup panic имеет
typed uncertainty; admission освобождается, неизвестный owner не теряется.
Completion flags не зависят только от nonnil recover payload. Ни raw child
panic/error/argv, ни stderr не становятся пользовательским диагностическим текстом.

## Интеграция существующего external TUN gate

Системный curl теперь использует этот slot, проверяется отличие его AppID от
parent/worker bypass AppID. Busy/uncertain cleanup не обходятся bare exec или
повторным helper. Shared protected admission учитывает3×(4KiB+1) максимального
stdout. Сохраняются caller/attempt deadlines, -q первым аргументом, HTTPS/IPv4,
no proxy/no redirects, curl time/size flags; write-out использует буквальные
escape sequences libcurl, не raw command-line newline.

После успешного capture всё ещё обязательны status200 trailer, ровно одно ip=
поле и expected-IP equality. Before/after worker/config/selector/LUID/WFP gates
остаются в прежнем `nativeTunnelBoundary`; Job exit не заменяет маршрут или
сетевую эпоху. Обычный periodic Observe не начал запускать внешние helpers.
Fixed Cloudflare external trace dependency и её outage/quorum gate не устранены.

## Что остаётся

Native DNS/UDP/TCP/IPv6 helper sender/result добавлен source-only в V2-147,
но App его не вызывает. Нужны optional diagnostic admission, pinned-image lock,
обоснованный non-exempt/socket-filter execution boundary и полный before/after
intent/config/owner/LUID/epoch/clock lease; protocol guard не заменяет их.
Raw parent socket или успешный DoH не становятся protected/native-family PASS.

Nested Job/ACL/image/path/handle inheritance, WFP arbitration, memory-pressure/
timeout/cancel/power loss/kill/reboot, stock curl/minimal environment/x86/x64,
module/compile/race/runtime/native/DR/resources/bindings/rollout/release/comparative
gates не выполнены. Это не sandbox разрешений или выполненный переход к службе.
Все TH-00…08 и оставшиеся P0 сохраняются; testing ban не менялся.

Исходники: core `{worker_process,worker_host_windows,worker_host_other,
helper_process}.go`; App `wfp_killswitch_windows.go`. Новые зависимости, generated
и tests, Android/harness не менялись. Никакой helper фактически не запускался.

<a id="pc-v229-helper-reader"></a>

## V2-229 — common helper output и original I/O/source lifetime

2026-11-05, Windows **source/unverified**. Полный PC00…09/old9 и прежние
обязательства сохраняются. Запрет tests/fixtures/build/product/native execution
не менялся. Развивает original I/O record V228, не создаёт второй process owner.

### Найдено в исходниках

`captureOwnedHelper` использовал растущий io.ReadAll(LimitReader), отдельный
buffered result и ioDone. Source проверялся до запуска, read/exit wait слушал
только ctx.Done. Отмена могла выбрать другую select-ветку и потерять completed
read cause; goroutine могла начать чтение после отзыва источника. keepImage
validation и panic finalizer могли затереть typed cause. Native DNS декодировал
bytes без отдельного immediately-before-decode original-scope guard.
Это source findings; runtime воспроизведение не выполнялось.

### Компоненты и ресурсная граница

- `helper_output.go` выделяет только общий `ownedPipeIO{done, err}` из V228.
  Typed TUN message и обычные output bytes остаются в своих records. Exact
  original header прикреплён к process.captureIO и тому же ioDone; one current
  record, без карты/очереди/новой goroutine. Normal worker protocol не изменён.
- `helperOutputRead` получает один buffer maxOutput+1 **до** child/reader creation.
  Существующий предел 1…64KiB сохраняется. Stdlib ReadFull используется с pure
  helperScopeReader: source/deadline checks до/после каждого фактического Read,
  без App callback. Dynamic ReadAll growth больше нет; output alias имеет cap=n.
- Empty/короткий/exact-max output принимается только при EOF и последующих
  process gates. Max+1 наблюдённых bytes — ErrHelperLimit, prefix не выдаётся
  как success. Нулевой прогресс blocking pipe без error — incomplete вместо
  busy loop. Ошибки syscall не печатают raw pipe paths; конечный local cause
  объединяется с cancellation. При panic bytes не выдаются.
- Одно исходное чтение пишет data/error и закрывает done. Caller читает fields
  только после close; прежний результат не заменяется до actual I/O return.
  Existing read+exit wait слушает intent/operation/dependency passive channels
  и ctx. Отзыв сам по себе не доказывает process/Job/pipe/image retirement.
- Capture сохраняет original process локально. На ошибке он unusable; прежний
  independent reaper закрывает pipes, ждёт original waiter/ioDone/empty Job и
  actual handles. После retirement attempt добавляется completed original
  read/overflow/panic cause. Unfinished cause остаётся unknown; local cleanup
  uncertainty сохраняет original owner и запрещает unsafe replacement.
- Output не покидает функцию до прежних zero exit/actual Job+I/O/image gates;
  updater keepImage и bounded verifier resource policy сохранены. Более поздний
  failure обнуляет provisional bytes. Earlier image/guard/panic/cancel cause
  сохраняется через join; finite image marker не затирает actual typed error.
- Native DNS rechecks original scope до DecodeNativeReply и после него; parse
  error и after-scope error объединяются, cancellation не подменяет independent
  local fault. Reply complete и scoped publication обязательны; native DNS не
  становится новым protected/health/whole-VPN PASS.

### Evidence и остаток

Source5Go/4 собственных docs. Exact-file gofmt5+1 exit0, manual source/caller/
lock/async ownership/diff и byte-SHA inventory:
`.artifacts/memory/helper-reader-source-evidence.json`. Исходные bytes и точный
прежний composite task71 сохранены; журнал только дополнен. Это **не runtime PASS**.

Нет tests/fixture edits/run, compile/build/vet/typecheck/product parser/check/
doctor/install/generation/SDK/helper/product/crypto/native/browser/admin/live/
DNS/server/OS-settings/owner-values/other-role/chat/subagent execution.
Все full PC/old9 acceptance остаются OPEN.

OPEN: legacy API/fixtures; empty/exact-max/max+1/partial output/EOF/error/no progress;
original source/ctx/deadline/delayed delivery/read+exit select races; mixed late
limit/panic/cancel/guard/image errors; keepImage/verifier behavior; actual original
Job/I/O/image/handle cleanup; native DNS parse/publication; resources/OS/real path.
Fixed logical buffer не доказывает allocator overhead/total heap/RSS/child/kernel
bounds или скорость. Уже вошедший pipe syscall требует original reaper; нет
atomic cancel/Read/OS hard-time guarantee. Helper limits, minimal environment,
native gate=false/private IPC9/Job8/TUN unavailable/schema2 unsupported прежние.

Следующее: updater GUI bounded exchange (`update_gui.go`) имеет отдельные result/
done и ctx-only wait, без source checks внутри goroutine. Его readiness/committed
lifetime/child-side path нельзя объявить закрытыми этой common reader правкой.
Remaining native writers/consumers и весь PC00…09/old9/SDK/W3b/interprocess/
durability/UX/release/comparative scope остаются активны.