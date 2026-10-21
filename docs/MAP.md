# MAP — активное дерево Snowden System

> Генерируется: `node scripts/harness/project.mjs map`. Не редактировать вручную.

## Быстрый вход

- Продолжение: [STATE](STATE.md) → активный пункт [PLAN](PLAN.md) → исходники.
- Правила: корневой AGENTS + ближайший scoped AGENTS; справочник docs/AGENTS.md.
- Подтверждённые уроки: поиск по docs/LESSONS.md; аудит: docs/OFFLINE-AUDIT-2026-10-18.md.
- История не загружается по умолчанию: old_snowden/, docs/archive/, docs/memory/, SESSION-STATE.

## Стек / источники команд

- Go 1.26, embedded sing-box/Wails: windows/go.mod + windows/tools/package.ps1.
- Vue ^3.5.13, TypeScript ~5.6.2, Vite: windows/frontend/package.json.
- Production tags: `with_utls,with_gvisor,with_quic,with_clash_api`.
- Корневой package.json — граница Node tooling и команды тестов; зависимости в windows/frontend. PostgreSQL нет; Android не начат.
- Команды проекта: Taskfile.yml (готовый Task); pinned инструменты: .harness/tools.lock.json.

## Где искать

| Область | Реальные входы |
|---|---|
| Windows facade / lifecycle | `windows/main.go`, `windows/app.go`, `windows/corebridge.go`, `windows/backend/core/manager.go`, `windows/backend/core/engine.go` |
| Защита / доказательства пути | `windows/killswitch_windows.go`, `windows/wfp_killswitch_windows.go`, `windows/backend/core/probe.go`, `windows/diagprobe.go` |
| Выбор / восстановление | `windows/controller.go`, `windows/failover.go`, `windows/netwatch.go`, `windows/autoretry.go`, `windows/scoring.go`, `windows/chanhealth.go` |
| Профили / доверие | `windows/backend/render/`, `windows/backend/config/`, `windows/backend/metadata/`, `windows/backend/secretvault/`, `windows/onboarding.go`, `windows/updateapp.go` |
| Vue presentation / API / state | `windows/frontend/src/App.vue`, `windows/frontend/src/components/`, `windows/frontend/src/composables/`, `windows/frontend/src/composables/navigation.ts`, `windows/frontend/src/composables/polling.ts`, `windows/frontend/src/api/contract.ts`, `windows/frontend/src/api/backend.ts`, `windows/frontend/src/views/`, `windows/frontend/src/style.css` |
| Локальные UI регрессии / дизайн страниц | `windows/frontend/tests/unit/`, `windows/frontend/tests/browser/`, `windows/frontend/vitest.config.ts`, `windows/frontend/playwright.config.ts`, `docs/TESTING.md`, `docs/UI-DESIGN.md` |
| Сервер / поставка | `configs/README.md`, `configs/singbox/`, `configs/deploy-vps.sh`, `windows/tools/package.ps1`, `windows/build/windows/installer.nsi` |

## Владение и reuse

- Текущее: UI → App → Manager → Engine. Наблюдатели передают решения через Manager.
- Целевые ProtectionController/EngineSession — planned FR-009/010, не существующий API.
- UI: views/composition → существующие composables → src/api/backend.ts; contract.ts — типы.
- Компоненты: NavRail/ContextTabs/TopBar/SectionHeader/ProbeReportView/HintBox; сначала проверить их API.
- Wails bindings генерируются; runtime config строится backend/render, не копией assets/configs.

## Работа с обвязкой

```powershell
node scripts/harness/project.mjs doctor
node scripts/harness/project.mjs next
node scripts/harness/project.mjs check --scope harness
```

`check` без `--run` ничего не выполняет. Scope frontend/backend — по запросу проверки.
Doctor — статический, не подтверждает VPN, приложение, VPS или сборку.
Release/admin/live не запускаются этим runner. Полный порядок — docs/HARNESS.md.

## Данные и артефакты

- Public descriptors: windows/backend/render/descriptors.json; server templates: configs/singbox/.
- Не индексировать значения секретов, env, vault, raw logs. MAP читает только перечисленные публичные манифесты.
- .artifacts/harness/last-<scope>.json — локальные итоги; docs/evidence/ — только redacted acceptance.
- Source digest — фильтр устаревания, не доказательство совпадения всей ОС/сети/инструментария.
