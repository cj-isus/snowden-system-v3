# V2-100 — проверенный откат Reload

Дата: 2026-10-20. Windows, Go 1.26.7, sing-box 1.13.19.
Локальное продолжение OFF-03, без VPS/admin/системной сети.

## Воспроизведённые дефекты

До изменения production кода выполнено:

`go test ./backend/core -run 'TestManagerReload(RestoresVerified|StartupRollback)' -count=1 -timeout=60s`

Оба новых regression — FAIL, exit=1:

- неуспешный probe кандидата останавливал прежний работоспособный путь;
- после внутреннего startup rollback Manager возвращал running, а старый
  probe имел только один вызов (начальный Start), без свежего подтверждения.

Facade дополнительно переводил любую Reload error в failCore. Автоматика могла
объявить кандидатов исчерпанными и остановить только что подтверждённый rollback.

## Реализованный контракт

- Отказ до замены instance — ErrReloadRejected. Нет нового канала и нет
  выдуманного сетевого FAIL для отклонённой конфигурации.
- Для rollback используются сохранённые applied bytes. Старый probe требуется
  повторно как после startup failure, так и после отказа probe нового канала.
- VerifiedRollbackError содержит свежий report и признак Restarted. Наличие
  обычного текстового label ErrSwitchRolledBack без report недостаточно для
  подтверждения пути на границе facade.
- Отмена и confirmed leak запрещают восстановление. Противоречивый report
  AllPassed=true + LeakConfirmed=true не принимается Start/Reload/Switch.
- Ошибки закрытия/запуска восстановленного engine сохраняют error; неполный
  cleanup остаётся sticky. Паника rollback не стирает исходную причину отказа.
- После подтверждённого restart прежнего конфига facade меняет snapshot identity
  и collector из restored config. Старый observer не управляет новой сессией;
  скоростные счётчики не наследуют прежнюю базу. Для rejected/seamless прежний
  lifetime сохраняется.
- Автоматика различает committed/verified rollback/rejected/failure/cancellation.
  Fresh rollback завершает outage, не получает BLOCKED от лимита кандидатов.
  Финальная публикация защищена operation generation и lifecycle lock.

Изменены Engine/Manager, общий путь failover и сообщения network preference.
Новых зависимостей, raw config, секретов и синтетического production bridge нет.

## Проверки

35 новых cases в `windows/backend/core/reload_test.go` и
`windows/switch_outcome_test.go`. Команда из `windows`:

```powershell
go test ./backend/core . -run 'TestManagerReload|TestReload|TestConfirmedLeak|TestStopDuringReload|TestChannel|TestAutomaticSwitch|TestLateAutomatic' -count=3 -timeout=120s -json
```

**105/105 новых leaf-case runs PASS**, failures=0. Regex также включает
существующие Reload regressions; они не прибавляются к количеству новых cases.

Матрица включает pass/fail/nil/leak/cancel/panic старого probe, startup rollback,
candidate Close failure, old constructor/Start/cleanup failure, поздний PASS
после Stop, отказ до замены, stale watchdog/fatal outcome, auto candidate limit
и старый результат после новой операции. Синхронизация — channels, без sleep.

Отдельный regression использует настоящий локальный sing-box для перехода
candidate → прежние bytes; probe и OS policy synthetic. Facade tests проверяют
контракт публикации на настоящем локальном Manager. Сетевой egress этим не
измеряется; проверка lifetime collector и действующего Clash API — разные
сценарии (последний ранее покрыт TestMetricsSessionMatchesAuthenticatedEngine).

Финальная полная последовательность:

`.tools/task/task.exe --taskfile Taskfile.yml backend:check`

Обычный Go набор → `with_utls,with_gvisor,with_quic,with_clash_api` → `go vet`:
**PASS, exit=0, 61.83 s**. Это финальный прогон после сохранения причины panic.
Промежуточный полный прогон тоже прошёл (114.63 s); его время не используется
как доказательство последующего изменения. Формат изменённых Go файлов проверен.

Артефакты (ignored):

- `.artifacts/tests/rollback-focused.jsonl` — повторные regressions;
- `.artifacts/tests/rollback-backend.txt` — полный сохранённый stdout/stderr;
- `.artifacts/evidence/2026-10-20-rollback-task.json` — команда, время, exit;
- `.artifacts/evidence/2026-10-20-verified-rollback.json` — hashes, сравнение
  файлов с V2-099 и итоги проверок.

## Границы

Hard bound стороннего Start/Close и процессная изоляция остаются OFF-02/06.
Подтверждённая утечка не даёт права пробовать direct fallback. Неопределённость
upstream early Close сохраняет консервативное требование завершить процесс.
Все prepare/observer fault cases, native DPI/WFP/OS/live и release gates не закрыты.
Исторический неповторившийся runner fail V2-099 имеет неустановленную причину;
этот срез не объявляет его исправленным. Текущий полный вывод сохранён.
UI и EXE не обновлялись. Запуск: TESTING.md; статусы: PLAN §5.1, STATE.
