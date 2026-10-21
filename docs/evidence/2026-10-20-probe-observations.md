# V2-103 — общий обработчик probe и фактические результаты

Дата: 2026-10-20. Scope: OFF-03/FR-010, Windows App lifecycle.

## Воспроизведение

`go test . -run '^TestAppProbe' -count=1 -timeout=60s -json`
до исправления: exit=1, пять leaf FAIL, два PASS. Сохранено в
`.artifacts/tests/probe-before.jsonl`. Factory/Run/nil-probe panic уходили наружу
и оставляли ProbeRunning=true; leak при AllPassed=true отображался как PASS;
поздний отменённый результат сбрасывал busy-флаг новой reservation.

## Изменение

- `probe_observation.go` — отдельная граница App diagnostics: guarded factory/Run,
  normalization, manual runner и UI mapping; consumers остались в corebridge/netwatch.
- Factory и probe inputs захватываются при создании: ExpectedEgress/targets не
  подхватывают позднюю мутацию, factory не заменяется следующей сессией. Run
  проверяет cancellation до factory и перед фактическим выполнением.
- Panic/nil получают nonnil structured failure. В diagnostics сохраняется тип
  panic, произвольный payload не попадает в error/log/view.
- Leak flag или явный leak step запрещает PASS и имеет видимый failing step.
  False/unavailable/skipped/unknown step не считается успешным; failed aggregate
  без отрицательного шага получает честное «путь не подтверждён». Исходные report
  и result slice не меняются; повторная нормализация не добавляет дубликаты.
- Manual RunProbe снимает busy только для своего config/manager/operation.
  Отмена или смена owner отклоняет результат до публикации. Только успешный
  report обновляет время успешного probe. Обычная ошибка диагностики оставляет
  прежний путь; confirmed leak останавливает engine с сохранённой защитой.
- Watchdog/network/fast-reprobe используют тот же guard. Fast-reprobe повторно
  проверяет session после Run; cancelled watcher выходит до принятия решения.
  Watcher принимает synthetic ticks через private worker, fast-reprobe — gap;
  production интервалы не изменены, новых runtime flags/global hooks нет.

## Проверки

Из `windows`:

```powershell
go test . -run '^TestAppProbe' -count=5 -timeout=120s -json
```

42 leaf cases ×5 =210 PASS,0 FAIL/skip, exit=0,23.02 s. Матрица включает
factory/Run/nil faults, context, source immutability, contradictory facts,
retained protection при leak, cancellation/Disconnect, new-owner busy,
late fast-reprobe и настоящие manual/watch/network entrypoints.
JSON `.artifacts/tests/probe-focused.jsonl`; точная команда/время:
`.artifacts/evidence/2026-10-20-probe-focused.json`.

Полный `.tools/task/task.exe --taskfile Taskfile.yml backend:check`:
ordinary → `with_utls,with_gvisor,with_quic,with_clash_api` → vet,
exit=0,65.28 s. Вывод `.artifacts/tests/probe-backend.txt`; record
`.artifacts/evidence/2026-10-20-probe-task.json`.

`gofmt -l` четырёх затронутых Go-файлов пуст. Итоговые hashes/checks:
`.artifacts/evidence/2026-10-20-probe-observations.json`.

## Границы

Новые тесты используют injected probes/fake policy, loopback mixed engine,
`t.TempDir`/временный APPDATA, synthetic ticks и синхронизацию каналами. Synthetic
`--tun` выбирает fakeProtection; реального TUN/WFP нет. Внешний DNS/HTTPS,
VPS/SSH/Cloudflare, пользовательский vault и системная сеть не использовались.

Нормализация проверяет согласованность существующего trusted Probe contract,
не полноту/независимость реальных измерителей. Полная network-epoch/decision
atomicity остальных watchdog/network публикаций и facade prepare/preflight
остаются в OFF-03/05. Также открыты process isolation/upstream early cleanup,
production EXE/old-helper migration, native Windows/DPI/live и comparative gate.
UI не менялся, EXE/NSIS не пересобирались, race при CGO=0 не запускался.
Историческая runner anomaly V2-099 не объявлена исправленной.
