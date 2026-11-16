# TH-01 — владение worker-процессом ядра

2026-10-21, V2-114: implementation, **unverified**. Отдельный P0-срез OFF-02,
не закрытие TH-01. Тестирование, сборки и native/admin запуски отложены владельцем.

## Решение

Application → Manager → Engine сохраняется. `NewIsolatedEngine` в corebridge
использует internal worker из того же EXE; sing-box остаётся встроенным в EXE.
`NewEngine` исполняет embedded код только внутри worker/local harness. Ошибка
изоляции не переключает приложение обратно на in-process запуск. Это граница
отказа, не sandbox разрешений, не служба и не новый внешний транспорт.

`main` обрабатывает internal mode до Wails/single-instance/update housekeeping/
таймеров/legacy helper. Неверный internal mode завершается без запуска GUI.
Worker использует унаследованный token, сам не повышает права и не управляет
system proxy/WFP/UI. Обычный console запуск без pipe stdin/stdout отвергается;
защита от злонамеренного локального владельца/admin этим не заявляется.

## Процесс и протокол

V2-146 [native helper owner](TH-01-NATIVE-HELPER.md) переиспользует существующий
host/Job reaper через общий suspended spawn. Worker сохраняет8-process ceiling и
прежнее окружение; системный curl имеет отдельный reserved slot, limit1/128MiB
committed-memory quota, минимальное окружение и complete reap до выдачи bytes.
Source-only/unverified; это не отдельная служба или native-family proof.

- Windows host использует уже закреплённый x/sys, без новых dependencies.
  У CreateProcess задан точный текущий executable, CREATE_SUSPENDED/NO_WINDOW.
  До ResumeThread процесс включается в unnamed non-inheritable Job Object:
  KILL_ON_JOB_CLOSE, active-process ceiling8, без breakaway. Ошибка assignment
  сохраняет suspended owner для завершения; sing-box ещё не запускался.
- Исходные process/thread handles принадлежат parent с CreateProcess. PID-file,
  OpenProcess по сохранённому PID и taskkill не используются. Падение parent
  должно закрыть единственный job handle; Windows-поведение пока не проверено.
- Три унаследованных handle явно перечислены через attribute allowlist:
  private anonymous stdin/stdout pipes и NUL stderr. Job/прочие handles не
  передаются. Конфиг не попадает в command line, environment или новый temp file.
  Существующий configured engine log сохраняется; parent не копирует child stderr.
  Limit учитывает helper tree: pinned sing-tun winipcfg/netsh.go запускает
  netsh.exe в Windows DNS fallback. Первоначальный limit1 исправлен по исходникам
  до любых запусков; поиск только по *windows* filename этот build-tag file упустил.
- Bounded length-prefixed JSON, version1, последовательные request IDs, точные
  top-level property names без duplicate/case aliases/null. Raw config ≤2MiB,
  reply ≤4KiB; unknown action/frame/version/ID отклоняются. Start/switch/current/
  stop — весь протокол. Произвольные child error/panic/config strings не
  передаются; наружу выходят фиксированные коды и bounded selector tags.
- Timeout/cancel/desync инвалидирует pipes protocol. Не повторять команду с
  неизвестным исходом на том же worker: завершить принадлежащий job и подтвердить
  root exit, I/O completion и ActiveProcesses=0. Ответ Close сам по себе не proof.
  Неуверенная очистка сохраняет owner/ErrEngineCleanupFailed и запрещает замену.
  Частично закрытые handles повторно закрываются после сохранённого drained proof.

## Бюджеты и интеграция

Начальные source ceilings: hello5s, Start20s, control5s, graceful Close2s,
независимый reap4s. Operation context может прервать ожидание раньше. Успешный
Start отделён от operation lifetime; последующий cancel не убивает здоровый
worker. При Stop/reload старый owner должен быть retired до нового процесса.
Селектор меняется в worker и в parent raw snapshot; ошибка sync уничтожает
неопределённый runtime вместо сохранения расходящихся конфигов.

Manager передаёт switch operation context и сохраняет protected probe/rollback
gates; перед успехом проверяет и engine status. State не выдаёт running при
замеченном exit/uncertain owner. App отклоняет report мёртвого Manager; сторож
останавливает с сохранением protection и без выдуманного channel-health FAIL.
Публикация/реакция на exit через текущий watchdog, не обещание мгновенного события.

В этом же срезе закрыта обнаруженная snapshot-смесь: automatic eligibility,
ranking и provenance берутся из applied cc.Descriptors/selector/profile, а не
позднего LoadDescriptors. Unknown applied set — abstain, не all-channels-failed.

## Что остаётся открытым

- Ни compilation, ни protocol/process integration не проверялись. Tests не
  добавлялись/правились/запускались; gofmt не является функциональной приёмкой.
- Native Windows startup/inherited handles/nested jobs, noncooperative Start/
  Close, unexpected exit, cancellation/Stop/reload/rollback и resource reclamation.
  Поддержка x/sys API сверена только чтением закреплённых локальных исходников.
- Process exit не доказывает cleanup routes/adapter/driver/WFP/DNS/proxy. Эти
  OS/crash/reboot/IPv4/IPv6/UDP/DNS gates остаются отдельными; protection release
  по-прежнему требует успешного существующего Stop, automatic failure её не снимает.
- Budgets ограничивают ожидание third-party работы после создания process owner,
  не длительность произвольного kernel syscall/CreateProcess/proxy/WFP и не
  общий GUI SLO. При kernel/driver failure owner остаётся uncertain, а не success.
- Full facade/controller decision atomicity, signed update/worker lifetime при
  замене EXE, service OFF-06, native packaging/release и весь TH-00…08 scope.
  Worker memory/CPU/startup overhead и пригодность ceiling8 не измерены; будущий
  внешний транспорт потребует отдельного resource/security решения.

V2-190 [W1 payload admission](TH-01-PAYLOAD-CONFIDENTIALITY.md) adds the shared
strict protected-config gate before Engine worker/factory startup and before a
Reload can close its old owner. Applied bytes are the owned normalized validated
graph; Reload does not re-read a later caller slice. Raw ShadowTLS nodes are not
admitted, regardless of selector alias/extra inner-TLS flag. Descriptor and secret
lookup gates apply the same current capability restriction. Source/gofmt7+1+1
only; lifecycle/normalized identity/legacy fixture/API/native/OS gates remain
unverified. Proxy cleanup W3 and probe-only/TUN W4 are still separate open work.

<a id="pc-v235-state-replies"></a>

## V2-235 — original startup/selector snapshots и Stop reply

2026-11-05, PC-01 source/unverified; полный PC00…09/old9 сохранён.

### Прежние пробелы

После V234 hello/start/current/switch/stop ещё разбирались после caller wait.
Switch проверял только формат Previous; неверный, но допустимый тег мог стать
rollback target. Target config/digest вычислялись после изменения child; parent
повторял config.Parse, который проверяет legacy certificate files под Engine.mu.
Это добавляло зависимость от позднего состояния диска в принятую live-сессию.

### Source implementation

- Новый `worker_state.go` содержит parent-only immutable snapshot: exact process,
  next request ID/action/selector/target, original digest/default и bounded target
  bytes/digest. Existing runtimeConfiguration читает только owned JSON; selector
  candidate и old default проверяются до IPC. Не-кандидат допускает только
  selector_unavailable, не successful ACK с придуманным digest.
- Неэкспортируемый workerRequest.replyState не сериализуется encoding/json и
  отсутствует в strict wire allowlist. Actual parent p.exchange требует original
  process/ID/action match перед обоими pipes; Start также требует исходный owned
  Raw slice. Child decoded request этот private scope не получает. Existing worker
  invocation/version9 и JSON properties не менялись; omission/compatibility OPEN.
- Dispatcher до original done теперь разбирает все known protocol actions.
  Hello ready и Stop ok/close_failed имеют empty shapes. Startup должен быть
  closed с exact original digest, current и switch Previous — original default.
  Actual changed switch не может ACK admitted; noop и valid nonfatal
  selector_unavailable имеют свои правила. Unexpected action/payload rejected.
- Parent после ACK публикует exact prepared target bytes, повторяет current ctx/
  original process checks и actual normalized digest. Неизвестная/отменённая
  мутация уходит в independent original reap. Malformed scope, dead/unusable owner,
  sequence exhaustion и failed original preparation не разрешают replacement.
  Early canceled healthy state command до подготовки не использует protocol ID.
- Parent preparation и child/local synchronization делят typed selector marshal.
  Normalize source просмотрен: только urltest→selector alias; Default не меняет
  это преобразование. Hash prepared typed bytes сопоставляется с normalized worker
  digest и parent publication. Target bytes ограничены maxWorkerConfigBytes.
- Selector rewrite использует уже принятый owned graph с value-only decoding.
  Повторных certificate-file reads через config.Parse под Engine.mu здесь нет.
  Initial protected W1 admission/validation сохранён; existing generic JSON
  fallback остаётся для local legacy graphs и не выдаёт forwarding authority.
- Stop response failure — ошибка команды; actual root/Job/I/O/handles решают
  cleanup. Close ACK или close_failed не подменяют original retirement receipt.
  Scope/digest/Previous не становятся health/native/Manager/source permission.

Scope хранит existing immutable owned start bytes и один prepared typed target;
serialization/parser allocation, total memory, lock wait/CPU latency не измерены.
Predicate не прерывает entered syscall или config decoder. Concurrent source
revocation не становится atomic native authorization благодаря этим checks.
No new network/OS operation/goroutine/watcher/timer/resource budget introduced.

### Evidence / OPEN

Source5Go/4docs; exact-file gofmt5+1+3 exit0, manual source/type/caller/lock/
Normalize/JSON-field/next-ID/reply/publication/reap/diff review и named byte/SHA.
Evidence `.artifacts/memory/worker-state-source-evidence.json`; exact original9
snapshots (new source has empty before) и prior task71 rev470 сохранены отдельно.
PLAN/этот task append-only byte prefixes retained; full goal/constraints/tasks0…70/
acceptance/decisions/old checks unchanged, all acceptance OPEN.

Не добавлялись/не менялись/не запускались tests/fixtures; нет compile/build/vet/
typecheck/product parser/check/doctor/install/generation/SDK/helper/product/
crypto/native/browser/admin/live/network/owner-values/other-role/chat/subagents.
OPEN: compilation/private API/fixtures; JSON omission/IPC9/foreign-stale scope/
raw alias/sequence exhaustion; before-after digest and Previous/target/noop/
unavailable/rollback, malformed hello/start/stop, context/source loss around
preparation/write/publication; independent reap/panic/race/Job/resources/OS.
Native Release/launch atomicity, scalar async provenance, полный PC00…09/old9/
SDK/W3b/interprocess/durability/UX/release/comparative OPEN. Native=false/IPC9/
Job8/TUN unavailable/schema2 unsupported; no runtime/safety/performance proof.
