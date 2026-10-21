# Windows — Go/Wails

## Фактическая структура

- Go module — этот каталог; `go.mod` закрепляет sing-box, Wails и Go version.
- `main.go` — запуск Wails; `app.go` — UI facade; `corebridge.go` — связка с ядром.
- `backend/core/` — Manager/Engine/Probe и proxy; `backend/render/` — descriptors
  → runtime config; `backend/config/` — строгая валидация; `backend/metadata/`,
  `secretvault/`, `onboarding/`, `update/` — границы данных и доверия.
- `controller.go`, `failover.go`, `netwatch.go`, `autoretry.go` — решения и события;
  применять изменения engine через Manager. Controller не второй engine owner.
- Ядро embedded; отдельного production `sing-box.exe` сейчас нет. Новая служба —
  целевое решение из OFF-06, не предположение для текущего кода.

## Инварианты

- Не захватывать `a.mu` повторно через `pushState`/`appendLog`; читать locked API.
  Для нескольких mutex фиксировать порядок владения в изменяемом участке.
- Контекст операции и engine lifetime различаются. Stop обязан отменять старые
  операции; stale generation не публикует running. Таймаут вокруг вызова сам
  по себе не доказывает прерываемость `box.Start/Close`.
- Prepare без глобальных side effects; applied snapshot соответствует реальному
  engine, controller и metrics session. Без структурной совместимости — Reload.
- Panic не возвращает успех. Switch/rollback/error должны иметь единый outcome.
- Ошибка не снимает strict-барьер как побочный эффект cleanup; см. FR-009.
- Secret refs, schema, migration и подписи проверяются до commit. Никаких
  synthetic credentials в production config; никаких реальных в fixtures.
- При генерации bindings использовать Wails; `frontend/wailsjs` не править вручную.
- `gofmt` только затронутых Go-файлов; не переносить backend packages ради формы.

## Проверки, когда запрошены

- Теги поставки: `with_utls,with_gvisor,with_quic,with_clash_api`, источник —
  `tools/package.ps1`. Не переносить `with_awg/with_wireguard` из старых записей.
- Узкий regression сначала; полный обычный/tagged/vet набор по blast radius.
  Команды из корня: `node scripts/harness/project.mjs check --scope backend`;
  runner сам выбирает cwd каждой команды.
- Race требует CGO/C compiler. Отказ инструмента — blocked, не PASS.
- `-tags live`, admin/WFP/TUN/NetGuard и реальный профиль не входят в обычные
  проверки. Не запускать их на рабочей сети как «безопасный smoke».
