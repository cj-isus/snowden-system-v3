# Проверки Snowden System

Этот документ описывает запуск и границы тестов. Статусы работ — в PLAN §5.1;
результаты конкретного прогона — в docs/evidence/. Зелёный локальный набор
не подтверждает доступность VPS, отсутствие блокировок или WFP в реальной ОС.

## Быстрый запуск

Из корня проекта, PowerShell:

```powershell
# Быстрые unit/component проверки интерфейса
npm run test:ui

# Полная UI проверка: типы приложения и тестов → unit → production build → Chromium
npm run check:ui

# Go: обычный набор → production tags → vet; без live-tag
npm run test:backend
```

Для чистой установки сначала `npm --prefix windows/frontend ci`, затем из
`windows/frontend` — `npx playwright install chromium`. Браузер устанавливается
один раз; запуск тестов не скачивает его и не использует пользовательский профиль.
Runtime зависимости Vue/Vite массово не обновляются. Тестовые зависимости
зафиксированы в manifest и lockfile; проверенная среда — Node 22.21.1, Go 1.26.7.
Корневой package.json задаёт локальную границу для Node resolver: инструменты
не должны зависеть от постороннего `D:/package.json`.

Дополнительные команды из `windows/frontend`:

| Команда | Назначение |
|---|---|
| `npm run test:watch` | Повторять затронутые unit/component тесты при изменениях |
| `npm run test:types` | Проверить типы production кода, fixtures и тестовых конфигов |
| `npm run test:coverage` | Покрытие выбранного критического UI контура |
| `npm run test:browser` | Проверить текущие source и **уже собранный** dist |
| `npx playwright test -g "470"` | Узкий responsive regression |
| `npx playwright show-report ../../.artifacts/tests/browser-report` | Открыть отчёт |

Перед `test:browser` после изменения приложения нужен `npm run build`.
`check:ui` делает это автоматически. Проверки не используют кэш для PASS.

## Что проверяется

| Риск | Исполняемая проверка |
|---|---|
| Старый snapshot затирает backend event, ответ после dispose | `tests/unit/backendState.test.ts` |
| Повторный Start/Switch, Stop во время долгого Start, ложный success после Stop | тот же файл; fake API с управляемыми Promise |
| Перекрывающиеся опросы, публикация после unmount, ошибка источника | `polling.test.ts`, fake timers и deferred responses |
| Меню и табы расходятся, теряется активный подраздел | `navigation.test.ts`, один registry всех 10 страниц |
| Поиск после пустой выдачи, клавиатура, focus trap/return | `commandPalette.test.ts`, `navigation.test.ts` |
| Go nil slice, null/false/true, недоступный Wails, event cleanup | `api.test.ts` |
| Два запуска диагностики или устаревший список результатов | `diagnostics.test.ts` |
| «Нет результата» ошибочно показано как PASS/FAIL | `testResult.test.ts` |
| Секрет без подтверждения, повторное сохранение, изменённый preview импорта | `forms.test.ts`; только synthetic значения |
| Смена адаптера/сброс счётчиков создаёт ложный пик трафика | `trafficHistory.test.ts`; реальные timestamp samples, ограниченная история |
| Любая страница ломает layout, icon control без имени, JS exception | `tests/browser/pages.spec.ts`: 10 страниц × 6 размеров |
| Production принимает `?mock=1` | отдельный браузерный сценарий против production dist |
| Фильтр каналов и Connect/Disconnect не проходят через UI/API | браузерные сценарии с явной demo-плашкой |
| Ошибка очистки теряет owner или снимает защиту | `windows/protection_test.go`: fake proxy/policy, настоящий локальный engine |
| Close error/panic, частичный Start, cleanup panic и ошибочный повторный Stop | `backend/core/lifecycle_faults_test.go`, fault injection на границе Box/Proxy |
| Stop на фазах Snapshot/Start/SetProxy/probe/reload/switch, конкурентная очистка | тот же файл; каналы синхронизации вместо sleep |
| Upstream early Close скрывает результат, App повторно снимает защиту | `TestUnconfirmedEngineCleanupCannotReleaseProtection`: реальный loopback bind conflict, fake WFP policy |
| Reload восстановил JSON, но прежний путь не подтверждён | `backend/core/reload_test.go`: старый probe, отказ/nil/отмена/panic/leak, ошибки restart/cleanup |
| Рабочий rollback ошибочно получает BLOCKED или поздний observer меняет новую сессию | `windows/switch_outcome_test.go`: rejected/verified/failed outcomes, Stop, новая сессия, lifetime metrics |
| Старый PID/timer снимает защиту, повреждённый marker теряется при миграции | `killswitch_lease_test.go`, `killswitch_windows_test.go`, `killswitch_retirement_test.go`: retained intent, rearm guard, explicit cleanup/retry, retired flags |
| Callback Reload держит mutex или поздний prepare затирает новую сессию | `backend/core/prepare_test.go`: panic/nil/cancel, reentry, Stop/Start, замена engine, concurrent/nested Reload, validation-before-options |
| Panic/nil diagnostic оставляет busy, leak становится PASS, late result меняет новый owner | `probe_observation_test.go`: shared guard, immutable factual reports, cancellation, manual/watch/network integration, fast reprobe |

Существующий Go набор дополнительно покрывает Manager/Engine, probe/preflight,
selector/reload/panic/cancel, retry/failover, классификацию, renderer/metadata,
vault, onboarding и update facade. Файлы `windows/onboarding_test.go` и
`updateapp_test.go` лежат в facade package: строка `[no test files]` у backend
подпакета не означает, что пользовательский сценарий вообще не тестируется.
Полнота этих контрактов остаётся отдельной работой OFF-02…06.

Для повторения локальной матрицы lifecycle из `windows`:

```powershell
$cases = 'TestEngine(CleanupFailure|FailedStart|ReloadDoesNotCreate|StartFailureWith)|TestManager(ReloadCleanupFailure|RestorePanic|PrimaryPanic|StopAcross|StopDuringSwitch|ConcurrentStop)|TestUnconfirmedEngineCleanup|TestFailedCleanupRetainsOwnership'
go test ./backend/core . -run $cases -count=10 -timeout=120s
```

Это 18 сценариев, включая subtests. При неопределённой очистке engine
повторный Stop обязан остаться ошибкой. Для sing-box 1.13.19 повторный Close
не повторяет cleanup; автоматический перезапуск поверх такого owner запрещён.
Проверки не доказывают hard bound зависшего Close; это отдельный OFF-02.

Матрица отката и результатов переключения из `windows`:

```powershell
go test ./backend/core . -run 'TestManagerReload|TestReload|TestConfirmedLeak|TestStopDuringReload|TestChannel|TestAutomaticSwitch|TestLateAutomatic' -count=3 -timeout=120s
```

Она включает 35 новых cases V2-100 и прежние Reload regressions. Fake probes
проверяют lifecycle contract; отдельный реальный loopback engine подтверждает
применение прежних bytes. Наличие VerifiedRollbackError само по себе не заменяет
полноту protected probe или live evidence. Ошибка/утечка/отмена отката не PASS.

## Изоляция и доказательства

Матрица explicit binding migration из `windows`:

```powershell
go test . ./backend/metadata ./tools/metadatatool -run '^TestBinding' -count=5 -timeout=120s -json
```

V2-108: 67 leaf ×5=335 PASS,0fail/skip. Pure migration проверяет подпись,
повышение version, explicit validity, полные ID назначения, duplicate channel/
role, immutable bound/revoked channels, alias isolation и все четыре role sets.
Expired authentic source допустим; output expiry должен быть в будущем.
Plan JSON имеет bounded size/depth, exact lowercase ASCII properties, запрещает
unknown/trailing/duplicate properties и case-folding/Unicode aliases.

CLI entry проходит subprocess этого test binary с generated private key;
production Wails EXE не запускается. Synthetic inputs/output используют t.TempDir.
Проверяются caps/boundaries, matching signer/consistent Ed25519 key, missing/
regular-file inputs, существующий output и aliases к source/plan/key. Hard-link
publication не заменяет существующий файл; отсутствие stages проверено после
обычных отказов. Crash/kill/power-loss/ACL/reparse/unsupported filesystem gate
не выполнен; на unsupported hard links CLI обязан вернуть ошибку.

App fixture сначала доказывает ErrAmbiguousKind у старого профиля; после explicit
IDs Preview/Apply/checked bytes/render выбирают два разных UUID одного kind и
сохраняют прежние slots. Missing ID/type mismatch отвергаются до записи всех
четырёх файлов. Истинное наличие/тип IDs проверяет staged client, publisher не
открывает vault. Pin-файлы здесь не доставляются, network/engine не запускаются.
Usage и format: evidence/2026-10-21-binding-migration.md.

Матрица общей channel schema из `windows`:

```powershell
go test . ./backend/metadata ./backend/render -run '^TestProfileSchema|^TestSecretIDLegacy' -count=5 -timeout=120s -json
```

V2-107: 75 новых leaf +2 legacy controls, 77×5=385 PASS,0fail/skip. До правок
9 known-field cases отвергались strict signed decoder. Проверяются общая
signed/embedded validation, подпись каждого extension и каждого FD field,
старые canonical bytes, nil/false/true mux, Clone и изоляция runtime-поправок.
Embedded decoder тоже отвергает unknown fields и trailing JSON. Неподдержанная
настройка не игнорируется: bandwidth только HY2, mux только WS/REALITY.

Общий fixture выполняет sign → Seal/Open → signature/Validate → temporary
metadata/vault → LoadDescriptors → RenderFrom для WS, двух HY2, REALITY и
ShadowTLS. Actual `option.Options.UnmarshalJSONContext(include.Context(...))`
проверяет каждый tuned config pinned sing-box. Box.New/Start не вызываются.
Отдельный App тест проходит Preview/Apply/four-file commit и checked bytes;
шесть invalid-extension сценариев сравнивают bytes всех файлов после отказа.
HY2 pin PEM предоставляется отдельно synthetic fixture: доставка/ротация pins
не доказаны. Endpoint/ASN independence и реальный network path не измеряются.
Старый `TestRenderAllChannelsPassSingboxParse` проверял лишь проектный
`config.Parse`; его название само по себе не было native parser evidence.
Legacy signatures совместимы, старые strict клиенты новые optional fields
отвергнут. Полная legacy binding migration и native/live gates остаются открыты.

Матрица KDF admission/codec/file input из `windows`:

```powershell
go test . ./backend/onboarding -run '^TestKDF' -count=5 -timeout=120s -json
```

V2-106: 79 leaf ×5=395 PASS,0fail/skip. Hostile N/r/p, checked overflow,
shared Seal/Open admission и error/panic/short-key используют private injected
KDF, не выделяют гигабайты и не подменяют production global. Каналы задают
конкурентность; 3 s guard обнаруживает зависание теста, не измеряет latency SLO.
Real scrypt выполняется только с штатными export defaults: fixture от старого
Seal открывается новым Open, новый Seal — независимым старым decoder, wrong
password/ciphertext отвергаются. Отдельный constant-key test проверяет GCM AAD.
Размеры header/payload/gzip, oversized whitespace, checksum, trailing/unknown
JSON, предсказание encoded output и regular-file input проверяются отдельно.
Все данные synthetic; файл читается из t.TempDir, системный диалог не вызывается.

Оценка активной памяти `128*r*(N+p+2)` взята из pinned x/crypto v0.51.0.
Per-call 96 MiB/2^21 N*r*p; aggregate 128 MiB/2^22 и две derivations, без очереди.
Это не hard process RSS/GC-retained heap, принудительная отмена scrypt, rate limit
или защита нескольких процессов; параллельность всего API вне KDF не ограничена
этим gate. Стандартный SNOB1 export совместим; более дорогие нестандартные
params/слишком большие password/salt и unknown fields теперь отвергаются.
Safe pre-fix overlay подтвердил 3 FAIL до compute; реальный hostile KDF не
запускался. Exact commands/source hashes: evidence/2026-10-21-kdf-admission.md.

Матрица strict import/transaction из `windows`:

```powershell
go test . ./backend/filetxn ./backend/metadata ./backend/render ./backend/secretvault -run '^(TestImport|TestFileTransaction)' -count=5 -timeout=120s -json
```

V2-105: 52 leaf ×5=260 PASS,0fail/skip. Preview/Apply проверяются на bad signature,
keys/schema/floor/protocol/refs/slots; до отказа сравниваются реальные bytes всех
четырёх temporary files. Есть успешный checked-bytes import, повторная проверка
подписи после Validate, concurrent versions и stale downgrade. Filetxn matrix
инъецирует error/panic каждого write, journal publish/commit-point rename,
rollback failure, missing targets и corrupt backups/manifest; fresh recovery
восстанавливает все файлы. App recovery проверен напрямую новым App/Manager,
полный Wails startup/crash/reboot не запускался. Компонентный write fault и App
commit rejection — разные уровни proof, не один live end-to-end эксперимент.

Journal содержит только public metadata/keys и DPAPI bytes; destinations задаёт
caller, не manifest. Все backups проверяются до restore. Незавершённый journal
сохраняет barrier. Serialize — координация cooperating callers в одном процессе,
не межпроцессный lock. Fsync файлов не доказывает Windows power-loss durability.
Staged directory cleanup после аварий, native filesystem/ACL и power-loss gate
остаются открыты. Срез V2-105 не включал KDF admission; его отдельная приёмка
V2-106 описана выше.

Матрица SecretID из `windows`:

```powershell
go test . ./backend/render ./backend/metadata ./backend/secretvault -run '^TestSecretID' -count=5 -timeout=120s -json
```

V2-104: 39 leaf cases ×5 =195 PASS. Два HY2 с разными passwords/obfs, WS,
REALITY и ShadowTLS проходят sign → encrypted bundle → verify → import vault →
LoadDescriptors → RenderFrom → strict sing-box parse. Используются synthetic
slots, временный DPAPI vault и самоподписанный test pin; network/engine не
запускаются. Есть missing/type mismatch/partial bindings, signature tamper,
старые canonical bytes, invalid batch/save fault, concurrent imports на одном
Manager и ambiguous AddSave. Проверяется byte identity vault при отказах.
Batch vault не доказывает транзакционность keys/envelope/floor/vault, устойчивость
к crash/power loss, multi-process writes или полную transport/FD/probe schema.

Матрица App probe из `windows`:

```powershell
go test . -run '^TestAppProbe' -count=5 -timeout=120s -json
```

42 leaf cases, 210 PASS в V2-103. Используются existing localRunningApp,
реальный loopback engine и injected Probe/fakeProtection. Synthetic `--tun`
в leak-тестах выбирает fake policy; сам engine не содержит TUN. Manual Stop
отменяет non-cooperative result до его публикации. Watchdog получает synthetic
ticks, fast reprobe — gap=0 через private worker; production интервалы сохранены.
Сетевые запросы default Probe не запускаются, пользовательский vault не читается.
Panic payload не должен попадать в error/log/view. Нормализация не заменяет
schema/independence protected evidence или live проверку отсутствия утечек.

Матрица Manager prepare из `windows`:

```powershell
go test ./backend/core -run '^TestReloadPrepare' -count=5 -timeout=120s -json
```

24 leaf cases, 120 PASS в V2-102. `faultBox`/`faultProxy` переиспользованы;
JSON берётся из существующего валидного `protectedFixture`, чтобы ошибка
валидации не подменяла проверку callback. Конкурентность задают каналы,
Stop завершается до освобождения заблокированного prepare. Пять секунд в
`awaitLifecycle` — предохранитель теста, не measured production latency SLO.
Произвольный callback должен вернуть управление; его принудительная отмена
не реализована, он не получает lifecycle ownership и не блокирует Manager.Stop.

Матрица retirement/migration из `windows`:

```powershell
go test . -run 'TestRetiredHelper|TestStartupRetainsEveryProtectionMarker|TestArmStrict|TestExplicitProtectionRecovery|TestLegacyRemoval|TestMissingLegacyRule|TestStrictMarkerRoundTrip' -count=5 -timeout=120s
```

64 cases включают strict/legacy/corrupt/unknown/mixed marker, отсутствие файла,
ошибку записи/apply/WFP cleanup/netsh/удаления marker и повторный Disconnect.
OS-вызовы заменены узкими injected boundary; marker находится в `t.TempDir()`.
CLI classifier проверяется на валидных/неполных/ошибочных аргументах, раннее
расположение guard в production main — через AST. Это **не** subprocess проверка
поставляемого Wails EXE и не остановка уже работающего старого helper.

- Браузерные тесты поднимают два собственных процесса на loopback 5374/5375.
  Чужой процесс на этих портах означает ошибку, а не повторное использование.
  Внешние HTTP запросы браузера блокируются; Wails/SSH/VPS там отсутствуют.
- Dev fixtures подгружаются динамически только при `import.meta.env.DEV` и
  явном `?mock=1`. Production проверяется без synthetic bridge.
- Обычные Go тесты используют локальные fixtures, loopback и fake policy.
  Нельзя добавлять `-tags live` в обычный runner или вызывать настоящий WFP
  из unit tests. Fake policy доказывает порядок вызовов, не работу Windows.
- Trace и скриншот при ошибке, HTML browser report и coverage находятся в
  ignored `.artifacts/tests/`. Скриншоты содержат исключительно demo-данные.
- Coverage scope явно задан в `vitest.config.ts`: lifecycle store, polling,
  навигация, диагностика, история трафика и навигационные компоненты. Пороги
  80% lines/statements, 75% branches, 70% functions относятся к этому scope,
  **не ко всему проекту**. Процент не заменяет негативные сценарии и review.
- Размеры 1366×768, 1180×780, 940×600, 752×480, 627×400, 470×300 проверяют
  CSS layout. Последние три моделируют уменьшение рабочего пространства;
  это не измерение нативного DPI WebView2 на Windows при 125/150/200%.

## Как добавлять полезный regression

1. Назвать нарушаемое пользовательское или safety свойство.
2. Воспроизвести отрицательный исход: ошибка/отмена/старый ответ/недоступность.
3. Управлять конкурентностью через каналы Go или deferred Promise, без
   ожиданий «на всякий случай». Fake timers — для таймеров, не для engine I/O.
4. Проверить наблюдаемый результат и ближайшую регрессию. Не проверять только
   наличие функции, строки кода или snapshot всего DOM.
5. Для backend выбрать реальные Manager/Engine там, где важна интеграция,
   и fake OS boundary. Для UI монтировать настоящий компонент с fake API.
6. После изменения входов повторить затронутые проверки и сохранить команду,
   область, результат и причины непроведённых OS/live проверок.

Инструменты: [Vitest](https://vitest.dev/guide/),
[Vue Test Utils](https://test-utils.vuejs.org/guide/),
[Playwright webServer](https://playwright.dev/docs/test-webserver).
Свой test runner и второй UI framework не добавлялись.
