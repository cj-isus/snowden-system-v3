# windows/frontend — UI (Vue 3 + TS, целевой хост Wails)

Активный composition root — `src/App.vue`: topbar + боковой слайдер меню
( concept docs/concept ) + активный View + ToastHost + палитра поиска (Ctrl K).

```text
src/
  main.ts                 — bootstrap; unhandledrejection → console
  App.vue                 — оболочка: TopBar + NavRail + активный View + палитра/уведомления
  style.css               — дизайн-токены concept (тёмная тема), скроллбары, фокус, общие примитивы
  api/
    contract.ts           — ЕДИНСТВЕННЫЙ источник типов UI↔backend (v1) + факты infobindings
    backend.ts            — фасад; build phase → unavailable + честные ошибки
    labels.ts             — RU-подписи состояний (1:1 со states) и форматтеры
  composables/
    backendState.ts       — стор фактов (AppState, channels, tests, busy)
    lifecycle.ts          — start/stop/probe/openLogs
    secretsStore.ts       — список/сохранение/удаление/проверка/тест/раскрытие секретов
    diagnostics.ts        — список тестов + запуск
    toast.ts, uiLog.ts    — уведомления и локальный журнал (free of secrets)
  components/             — Icon, NavRail (слайдер меню), TopBar (бренд/поиск/окно),
                            HintBox, ToastHost, TestCard, SecretCard, ProbeReportView
  views/                  — Dashboard (главная: hero/канал/карта/проверка/каналы/автозащита/
                            события/нижняя сетка/о защите), ChannelsPage (таблица с фильтрами),
                            AutoProtect (сторож: состояние/правила/инциденты),
                            Network (сетевые факты ОС + NetGuard), Statistics (счётчики адаптера),
                            Tests, Secrets, Channels (карточки — легаси-раздел), NetGuard, Logs,
                            Settings (профиль доставки + секреты + о защите)
```

## Правила (обязательные)

1. **Только факты.** Статусы из `api/contract.ts` (lifecycle, validation, probe).
   Отсутствие данных — «нет данных», не ноль и не выдумка (AGENTS.md §3.4/§3.6).
2. **Секреты.** UI показывает только метаданные: fingerprint (SHA256-префикс),
   статус проверки, число раскрытий. Значение копируется в буфер по явному
   действию, на экране — маскировано (§2.4).
3. **Хинты.** Каждый раздел содержит пояснения на русском: что означает статус,
   что проверяет probe, ограничения среды (UDP/TUN), правило компрометации.
4. **Контракт.** Новое поле UI начинается с `api/contract.ts` (и потом app.go);
   `wailsjs/` не редактируется руками.
5. **Сборка/проверка:** `npm run build` (vue-tsc --noEmit + vite build).

## Статус backend

`windows/app.go` реализует контракт §4.1 частично:
- секреты — полностью (DPAPI, fingerprints, verify, reveal-аудит, seed 5 слотов,
  живой TestSecret по виду секрета: канальные — сверка хешей с сервером по SSH,
  ssh-key — реальный вход, cf-token — /user/tokens/verify, custom — skip);
- тесты — полностью (UDP-availability, HY2 reachability, secrets present, elevation, core-wired);
- lifecycle Start/Stop/Probe — честно отклоняется до подключения ядра (fail-closed);
- инфо-факты (infobindings.go): GetNetworkFacts/GetNetStats (PowerShell + HTTP-наблюдение,
  кеш 5с), GetDeliveryProfile (metadata-стор), GetFailoverStatus (failoverPolicy).

Фасад `api/backend.ts` вызывает реальные биндинги (`wailsjs/go/main/App`);
вне Wails (браузерный предпросмотр) — честный build phase. Сборка exe:
`wails build` из `windows/` → `build/bin/snowden-system.exe`; хранилище:
`build/bin/secrets/vault.v1.json` (gitignored, DPAPI, atomic write).
