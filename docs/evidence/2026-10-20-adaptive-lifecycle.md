# V2-097 — адаптивный VPN: документация и первый safety/lifecycle срез

Дата: 2026-10-20. Работа начата 19 октября; текущая приёмка — 20 октября.
Область: OFF-01/02/03 и непосредственно связанные OFF-05/06/07/UI contracts.
Нормативные требования/статусы — PLAN FR-009…012, §5.1. Этот файл — evidence.

## Основание документации

Исходный archive/plan_gpt.md §0 задаёт NetworkFacts → observations → classifier
→ ChannelHealth → controller → Manager. Сопоставлены его network epoch,
probe levels, failure domains, recovery/canary/budget и разделение egress/path.
PLAN FR-012 закрепляет safety-first порядок, измеримые connect/recovery/TTFB/
throughput/false-switch/probe-cost метрики и условия сравнения. Значений
baseline нет; мировой уникальности или превосходства не заявляется.

CENSORSHIP-THEORY §0 отделяет сетевую аномалию от доказанного вмешательства,
ограничивает перенос результатов GFW на другие сети/периоды, исключает
универсальную «неотличимость» и выводы о длинных потоках по маленькому echo.
Проверенные первоисточники: исследование GFW Report/USENIX 2023, методика OONI,
Microsoft WFP transaction API. Ссылки — непосредственно в теории. Измерений
нынешних блокировок в сети владельца не выполнялось.

## Реализованный срез

- App владеет desired intent, StopReason, operation mutex/cancel/generation.
  Disconnect освобождает policy после teardown; failure/quit/restart — нет.
  Первый TUN Connect вызывает arm до render/preflight. Если arm не удался,
  ошибка означает отсутствие подтверждённой защиты, не «всё защищено».
- Strict WFP не имеет expiry-helper. Legacy helper dispatch идёт в main до
  Wails/SingleInstance; expiry/startup не удаляют WFP и recovery marker.
  Маркер, в том числе старый/повреждённый, — намерение, не effective-policy PASS.
- При повторном arm существующий барьер проверяется и сохраняется; отсутствует
  прежний unconditional remove/reapply. При автоматическом teardown отзываются
  TUN permits. Частичный/повреждённый набор требует явного recovery, не тихого
  удаления. Полная транзакционная установка/repair и OS enforcement ещё не приняты.
- Ручной active RunProbe не захватывает a.mu повторно через pushState; его
  отменённый результат не публикуется, positive leak завершает unsafe session.
- Switch/Reload panic возвращают error с cleanup errors. Даже same-tag switch
  требует probe; отменённый PASS не commit. Engine context остаётся отдельным
  от operation context; cancellation во время Start достигает runCtx, а после
  успеха callback отключается. Это cooperative cancellation, не hard Close bound.
- Prepare не перезаписывает active metrics и не усекает log. Один endpoint/
  secret соответствует raw engine config, временный port-check listener закрыт.
  Seamless возможен только при selector-only difference; иначе Reload. После
  reload сбрасывается metrics collector, после selector switch он сохраняется.
- Failed switch использует общий finalizer/retry. Старый watchdog не может
  остановить/переключить другую session. Ownership handle сохраняется при
  ошибке teardown. Update не меняет EXE после Stop failure.
- WFP drift-check включает conditions/AppID/LUID, веса, flags, owned sublayer.
  Эти unit shape checks не исполняют WFP arbitration на Windows.
- В UI доступен явный Stop в error/start/reload через существующий lifecycle API.
  В текстах SOCKS scope и retained intent отделены от доказанной OS защиты.
- Vite css.postcss настроен локально (plain CSS, plugins=[]); внешний D:/package.json
  не изменялся. Зависимости/framework/bindings не менялись.

## Выполненные проверки

Cwd Go: D:/SNOWDEN-SYSTEM-V3/windows. Cwd frontend: windows/frontend.
Go 1.26.7 windows/amd64; CGO_ENABLED=0. Production tags взяты из tools/package.ps1.

| Команда / область | Результат |
|---|---|
| `go test ./... -count=1 -timeout=120s` | PASS, без live/admin tags |
| `go test -tags with_utls,with_gvisor,with_quic,with_clash_api ./... -count=1 -timeout=120s` | PASS, включая authenticated clash metrics на реальном локальном engine |
| `go vet ./...` | PASS |
| `npm run build` | PASS: vue-tsc + Vite 6.4.3, 86 modules, single-file dist |
| `go build -tags with_utls,with_gvisor,with_quic,with_clash_api -ldflags '-H=windowsgui' -o ../.artifacts/build/snowden-system-offline-2026-10-20.exe .` | PASS, проверочный EXE с текущим dist |
| Два EXE helper-процесса, `--killswitch-expire --killswitch-seconds=1`, пустой изолированный AppData | Оба exit=0 в пределах 5s, 3407ms суммарно, createdFiles=0; не заходят в GUI |
| `node scripts/harness/project.mjs doctor` | PASS: 53 pass, 2 warn, 0 error, 2 info; предупреждения — нет git baseline и Gitleaks. Не VPN-проверка |
| Race / WFP admin / protected live / Wails NSIS installer | NOT RUN: CGO=0; нет Windows VM/live запроса и VPS; вне текущего среза |

Адресные regressions: active probe, Stop cancellation, stop reason matrix с fake
policy, cleanup error при manager=nil, stale Start/watchdog/switch, manual leak,
retry intent/generation, panic switch/reload, canceled PASS, same-tag probe,
metrics port/auth, structural config guard, WFP drift, strict marker startup.
Использованы synthetic secrets, fake system proxy/policy, локальные listeners.
Они не подтверждают работу реального VPN-пути.

Первый полный прогон после введения intent выявил fixtures netwatch без
protectionDesired и зависание assertion под mutex в TestBackoffCounterStillGrows.
Fixtures и assertion исправлены; окончательные прогоны выполнены заново после
последних изменений. Предыдущие PASS не переносились на изменённые исходники.

Fingerprint изменённых исходников, manifests, embedded dist и проверочного EXE:
`.artifacts/evidence/2026-10-20-adaptive-lifecycle.json`. Это snapshot текущего
дерева без git revision; не доказательство clean-checkout reproducibility.
EXE не заменяет существующую установленную сборку и не является release/installer.

## Не закрыто

- Все OS-фазы Stop, зависшие third-party Start/Close, legacy PID identity/migration,
  crash/reboot и реальная WFP arbitration/IPv4/IPv6/TCP/UDP/DNS в изолированной VM.
- Полный prepare/rollback/observer outcome matrix и observer evidence generation.
- OFF-04 signed schema/SecretID/transactional import/KDF; OFF-05 endpoint health,
  actual engine/OS DNS, общий diagnostic budget, калибровка; OFF-06 verified staging,
  signing roles, concurrent Apply и минимизация privileged GUI.
- OFF-07 clean baseline/Wails/NSIS/race; OFF-08 production mock/polling/responsive;
  OFF-09 server IaC и независимая инфраструктура после оплаты/явного запроса.

Системный proxy/DNS/WFP/TUN рабочей машины не изменялся. SSH/CF/deploy не
выполнялись, реальный vault/runtime-config/user logs не читались. Скорость,
безотказность на целевых сетях и экономия токенов не измерены.
