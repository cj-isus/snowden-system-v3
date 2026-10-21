# V2-101 — retirement legacy helper и явная миграция

Дата: 2026-10-20. Scope: OFF-02, FR-009, Windows client core.

## Исходный дефект и решение

Поиск потребителей показал: apply/start/renew legacy lease оставались только
в тестах, production использует strict WFP. При этом stop/renew запускали
`taskkill /PID` из неподтверждённого marker, startup автоматически удалял netsh
правила, а неполные helper-флаги могли пройти в обычный GUI lifecycle.

До исправления `TestRetiredHelperHasNoProcessTerminationOrLeaseStart`:
`go test . -run '^TestRetiredHelperHasNoProcessTerminationOrLeaseStart$' -count=1 -timeout=60s`
завершился exit=1: три legacy entrypoint и два literal taskkill. Это source
regression; реального завершения процесса или изменения firewall не выполняли.

Вместо нового identity-протокола для неиспользуемого режима удалены start,
renew, timer expiry, PID termination и возможность создания netsh policy.
Новый binary принимает любые старые helper-флаги как немедленный возврат из
main до Wails/update/SingleInstance. Это не отзывает код уже запущенного EXE.

## Поведение

- Startup сообщает о сохранённой защите и не меняет marker или правила.
- Legacy/повреждённый/неизвестный marker, strict+ненулевой helper PID и ошибка
  чтения дают actionable recovery error до WFP mutation. PID не исполняется.
- Новый strict marker сохраняется до apply; при ошибке apply остаётся intent.
  Существующий strict marker не переписывается; enforcement всё равно требует
  успешного WFP apply и последующей приёмки защищённого пути.
- Только explicit Disconnect после engine cleanup очищает WFP и семь точных
  legacy имён. Оба backend пробуются при ошибке одного; errors.Join сохраняет
  причины. Marker удаляется после полного успеха; повторная попытка поддержана.
- Ошибка запуска netsh и общий текст «не найдено» не считаются отсутствием правил.

## Выполненные проверки

Из `windows`:

```powershell
go test . -run 'TestRetiredHelper|TestStartupRetainsEveryProtectionMarker|TestArmStrict|TestExplicitProtectionRecovery|TestLegacyRemoval|TestMissingLegacyRule|TestStrictMarkerRoundTrip' -count=5 -timeout=120s -json
```

64 leaf cases обновлённой матрицы, 320 PASS, 0 FAIL/skip; exit=0, 14.09 s.
JSON: `.artifacts/tests/legacy-focused.jsonl`; запись команды/времени:
`.artifacts/evidence/2026-10-20-legacy-focused.json`.

Полный `.tools/task/task.exe --taskfile Taskfile.yml backend:check`:
обычные Go → `with_utls,with_gvisor,with_quic,with_clash_api` → vet,
exit=0, 81.92 s. Вывод `.artifacts/tests/legacy-backend.txt`, время/результат:
`.artifacts/evidence/2026-10-20-legacy-task.json`. Старые lease tests заменены
новым контрактом; соседние protection/lifecycle/rollback regressions проходят.
`gofmt -l` семи изменённых Go-файлов пуст.

Source hashes/изменённые файлы и результат harness находятся в
`.artifacts/evidence/2026-10-20-legacy-retirement.json`.

## Границы результата

Только `t.TempDir`, injected WFP/runner и pure CLI classifier. AST проверяет,
что main начинает с возврата для retired flags; это не запуск production EXE.
System proxy/DNS/WFP/TUN, VPS, SSH и Cloudflare здесь не менялись.

Already-running helper из прежнего EXE, migration/crash/reboot в elevated VM,
hard bound non-cooperative Start/Close и upstream early cleanup остаются в
OFF-02/06. Новая версия не может обещать отмену уже запущенного старого кода.
UI/EXE/NSIS здесь не менялись/не пересобирались, race при CGO=0 не запускался.
V2-099 runner anomaly остаётся в OFF-07 с неизвестной причиной.
