# windows/frontend — UI (Vue 3 + TS, целевой хост Wails)

Активный composition root — `src/App.vue`: topbar + левое меню продуктовых
разделов + общий заголовок + контекстные вкладки выбранного раздела + активный
View + ToastHost + палитра поиска (Ctrl K).

```text
src/
  main.ts                 — bootstrap; unhandledrejection → console
  App.vue                 — оболочка и раздельное состояние section/active tab
  style.css               — command-center токены, скроллбары, фокус, общие примитивы
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
  components/             — Icon, NavRail (6 продуктовых разделов), ContextTabs
                            (подпункты выбранного раздела), SectionHeader,
                            TopBar, HintBox, ToastHost, TestCard, SecretCard,
                            ProbeReportView
  views/                  — Dashboard (состояние/маршрут/probe/факты среды),
                            ChannelsPage (единая таблица с фильтрами),
                            AutoProtect (сторож: состояние/правила/инциденты),
                            Network (сетевые факты ОС + NetGuard), Statistics (счётчики адаптера),
                            Tests, Secrets, Channels (карточки — легаси-раздел), NetGuard, Logs,
                            Settings (профиль доставки + секреты + о защите)
```

## Навигационная иерархия (V2-081)

Левое меню и горизонтальные вкладки не дублируют друг друга. Меню выбирает
продуктовый раздел, вкладки — его внутреннее представление. Каждый View
принадлежит ровно одному разделу: `dashboard→Главная`,
`network/autoprotect/statistics/netguard→Состояние`, `channels→Каналы`,
`tests→Проверки`, `logs→Журнал`, `settings/secrets→Настройки`. `App.vue` хранит
`section` отдельно от `active`, а `sectionForView` гарантирует правильную
подсветку меню при вложенной и поисковой навигации. Панель вкладок не
рендерится для разделов с единственным экраном: одиночная вкладка не создаёт
выбора и не должна дублировать структуру.

## Адаптивные режимы (V2-084)

- `>1180px`: полный левый rail, двухколоночная Главная;
- `821–1180px`: компактный rail с иконками;
- `≤820px`: нижняя навигация с подписями и одноколоночный workspace;
- `≤980px`: hero Главной становится вертикальным, power-control — компактным
  горизонтальным блоком;
- `≤620px`: мобильные размеры контролов и уплотнённые facts.

Проверочные viewport: 1440×900, 1024×720, 768×700, 480×780. Breakpoint должен
оцениваться по реальному окну Wails, а не только по широкому browser-preview.

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
  кеш 5с), GetRuntimeInfo (версия текущего EXE + Go build info), GetDeliveryProfile
  (metadata-стор), GetFailoverStatus (failoverPolicy). NetworkFacts разделяет
  прямой egress без proxy и egress через SOCKS5h; ISP приходит из live HTTP-факта.

Фасад `api/backend.ts` вызывает реальные биндинги (`wailsjs/go/main/App`);
вне Wails (браузерный предпросмотр) — честный build phase. Сборка exe:
`wails build` из `windows/` → `build/bin/snowden-system.exe`; хранилище:
`build/bin/secrets/vault.v1.json` (gitignored, DPAPI, atomic write).
