# Frontend — Vue 3 / TypeScript / Wails

- Composition root — `src/App.vue`; меню — `NavRail`, подразделы — `ContextTabs`.
  Табы — подпункты выбранного меню; не дублировать две одинаковые навигации.
- Перед новым компонентом посмотреть `src/components`, views и composables.
  Views организуют сценарий; reusable presentation — в components; общий state
  и lifecycle — в существующих composables, вызовы backend — в `src/api/backend.ts`.
- Контракт UI — `src/api/contract.ts`; источник runtime-фактов — Go. Изменение
  API сначала согласовать с backend; `wailsjs` генерируется, не патчится вручную.
- Empty/loading/error/blocked/unavailable — отдельные состояния. Отсутствие
  измерения не превращать в 0; browser preview не является работающим VPN.
- Dev fixtures отдельно от production. `?mock=1` и скриншот нельзя использовать
  как evidence доступности каналов; отсутствие backend не маскировать mock.
- Асинхронный ответ привязан к session generation, устаревший результат не
  перезаписывает свежий. Таймеры/подписки имеют владельца и cleanup; без наложения
  polling и повторных action, долгий вызов не блокирует доступ к отключению.
- Состояние защиты, причина ошибки, действие восстановления важнее декора.
  Списки/сводки объединять в строки/таблицы; не увеличивать число карточек без
  новой пользовательской задачи. Стиль референса не оправдывает плохой контраст.
- Общие стили/токены — `src/style.css`, локальные — scoped. Не добавлять новый
  UI framework, router/store или каталог shared/lib без обнаруженной потребности.
- Для responsive-приёмки проверять минимум Wails 940×600, обычное 1180×780,
  1366×768 и scaling 125/150/200%; keyboard, focus, доступные имена controls.
  Скрытый label должен иметь доступное имя; visual mock не доказывает native UX.
- Проверки по запросу: Vitest + Vue Test Utils, vue-tsc и Vite; вызов из корня:
  `node scripts/harness/project.mjs check --scope frontend`.
  Ожидаемые команды сначала посмотреть без `--run`.
  `npm run check:ui` дополнительно запускает Playwright на всех страницах.
  Сценарии, команды и границы — docs/TESTING.md; дизайн — docs/UI-DESIGN.md.
- Registry меню/страниц — `composables/navigation.ts`; poller — `polling.ts`.
  Отложенные ответы проверять deferred Promise, таймеры — fake timers.
  Полностью skipped диагностика не означает PASS.
- AUD-018 локально исправлен: явный PostCSS config и корневая package boundary.
  Внешний `D:/package.json` не менять; это не вход проекта.
- PostgreSQL в этом проекте не обнаружен; SQL-часть переносимого skill не
  загружать и не добавлять БД без отдельного требования.
