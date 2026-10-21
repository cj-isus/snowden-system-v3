# V2-099 — cleanup ownership и матрица Stop

Дата: 2026-10-20. Windows, Go 1.26.7; sing-box **1.13.19** из go.mod.
Продолжение OFF-02/03 по запросу владельца. VPS/admin/SSH/системная сеть
не использовались; проверяются fake policy/proxy и локальный loopback.

## Найденные дефекты и исправления

1. Engine.Stop терял Box даже при Close error. Следующий Stop возвращал nil,
   что позволяло App объявить очистку успешной и перейти к Release policy.
   Теперь ошибка и owner сохраняются; Status=error, запуск замены запрещён.
2. Частичный Box.Start не фиксировал owner до вызова и терял ошибку cleanup.
   Owner присваивается до Start; ошибки операции и cleanup объединяются.
3. Panic в cleanup могла прервать обработку исходной panic, пропустить
   восстановление proxy и оставить manager.active. Каждый шаг cleanup имеет
   отдельную границу recover; finalizer завершает операцию при этих отказах.
4. Reload воспринимал любой статус кроме stopped как running. Неуспешно
   очищенный кандидат теперь сохраняет error и запрещает запуск rollback.
5. После отмены на фазе SetProxy мог запускаться probe, а switch выполнял
   rollback probe после Stop. Добавлены проверки отмены до следующих шагов;
   late PASS не принимается. App сбрасывает CoreReady/ProbeRunning при ошибке Stop.

Production изменения: `backend/core/{cleanup,engine,manager}.go`,
`windows/corebridge.go`. Точки fault injection локальны экземпляру Engine;
глобальных mutable hooks и новой службы нет.

## Что означает неуспешная очистка

Проверены исходники установленной зависимости:
`$GOMODCACHE/github.com/sagernet/sing-box@v1.13.19/box.go`, Start и Close.
Close закрывает done перед дочерними сервисами; повтор возвращает os.ErrClosed.
Start при ошибке сам вызывает Close, но не возвращает его результат.

| Случай | Поведение |
|---|---|
| Close успешен | Owner снят, обычный повторный Stop идемпотентен |
| Engine закрыт, proxy Restore отказал/паниковал | Proxy ownership сохранён; повторный Restore разрешён |
| Close error/panic или недоступный результат early Close | Sticky ErrEngineCleanupFailed; owner сохранён; новый engine и Release запрещены |

Это консервативная граница: даже ошибка старта из-за занятого порта может
потребовать завершения процесса клиента, если upstream скрыл итог очистки.
Отсутствие доказательства не объявляется успешным освобождением ресурсов.
Прозрачное автоматическое восстановление требует процесса-владельца с
подтверждённым завершением либо проверяемого upstream cleanup contract.
Hard bound для зависшего стороннего Close этим изменением **не реализован**.

## Регрессии и выполненные команды

`windows/backend/core/lifecycle_faults_test.go`: Close error/panic и повтор,
partial Start error/panic, успешный cleanup с повторным Start, отказ старого
Close на Reload, отказ cleanup кандидата, вторичная cleanup panic, proxy
recovery, Stop на Snapshot/Start/SetProxy/probe/reload/switch и concurrent Stop.
`windows/protection_test.go`: реальный loopback bind conflict + fake policy
проверяет всю цепочку Engine → Manager → App и запрет Release после двух Stop.
Прежний proxy recovery дополнен проверкой stale CoreReady/ProbeRunning.

17 новых сценариев и один усиленный прежний. Повторение **18 × 10: 180 PASS,
0 failures**. Команда с точным regex находится в TESTING.md. Детерминированные
точки синхронизации — channels; sleep для порядка операций не используется.

Финальные самостоятельные прогоны:

- `go test ./... -count=1 -timeout=120s -json` — PASS, 6 packages с тестами;
- `go test -tags with_utls,with_gvisor,with_quic,with_clash_api ./... -count=1 -timeout=120s -json` — PASS, 6 packages с тестами;
- `go vet ./...` — PASS;
- `.tools/task/task.exe --taskfile Taskfile.yml backend:check` — PASS,
  точный повтор ordinary → tagged → vet; exit=0, 78.46 s, stdout сохранён;
- `gofmt -l` по шести изменённым Go файлам — пусто;
- `node scripts/harness/project.mjs check --scope harness --run` — PASS,
  2.69 s, bounded source before=after.

**Неполное evidence отдельного прогона:** runner `npm run test:backend`,
начатый в 06:59:27 UTC, вернул exit=201, outputBytes=37333. Его сессия вывода
стала недоступна до чтения; runner не сохранял raw output. Имя теста и причина
этого сбоя не установлены. Последующие обычный/tagged JSON прогоны прошли;
отдельный точный Task replay также прошёл, stdout и exit сохранены ниже.
Причину нельзя приписать приложению, инфраструктуре или flaky test без данных.

## Артефакты и границы

- `.artifacts/tests/lifecycle-faults-repeat.jsonl` — повторная матрица;
- `.artifacts/tests/cleanup-full-unit.jsonl`, `cleanup-full-tagged.jsonl` —
  полные структурированные результаты;
- `.artifacts/tests/cleanup-task-backend.txt` и
  `.artifacts/evidence/2026-10-20-cleanup-task.json` — точный Task replay;
- `.artifacts/evidence/2026-10-20-cleanup-ownership.json` — source hashes,
  итоги, версии и запись неполного failed-run evidence;
- `.artifacts/evidence/2026-10-20-ui-tests-backend.json` — сохранённый предыдущий
  V2-098 backend run до замены файла last-backend.

UI не менялся; старые 47 unit/component и 10 browser PASS здесь не переиздаются
как новый прогон. EXE/NSIS, race при CGO=0, native WebView2 DPI, WFP/OS и VPS
не проверялись. OFF-02/03 остаются partial: isolation/hard-bound, legacy PID,
verified Reload rollback и полный observer outcome matrix открыты.
