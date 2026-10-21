# Проектные соглашения — справочник по необходимости

Актуализировано 2026-10-18, V2-095. Автоматическая точка входа — корневой
AGENTS.md и ближайшие scoped AGENTS. Этот файл не требует полного повторного
чтения на каждую правку. Нормативные решения/статусы — PLAN.md; журнал §6
append-only. Старые SESSION-STATE.md/memory не переопределяют текущие правила.

## 1. Маршрут чтения

| Задача | Минимальный контекст |
|---|---|
| Продолжение | STATE → активный OFF-пункт PLAN → связанный код/уроки |
| UI | frontend/AGENTS → App/компонент/composable/API и потребители |
| Lifecycle, WFP, cancellation | windows/AGENTS → PLAN FR-009/010 → затронутый поток |
| Профиль, подпись, секреты, update | PLAN FR-008/011 → соответствующий backend package |
| Теория/классификация | раздел CENSORSHIP-THEORY + failclass/scoring |
| Сервер | configs/AGENTS + configs/README + INFRASTRUCTURE с датой evidence |
| Настройка среды | HARNESS → .harness/project.json → project.mjs |

Не читать подряд весь архив, журнал и все memory-файлы. Карта — указатель, не
доказательство отсутствия файла. Сначала фактические манифесты/потребители.

## 2. Источники и границы

- PLAN.md — единственный backlog, требования, решения, нормативные критерии.
- STATE.md — компактный заменяемый снимок, не второй реестр статусов.
- MAP.md — генерируемая навигация по активному дереву; обновляется командой.
- LESSONS.md — подтверждённая причина/рабочее правило и ссылка на evidence.
- tasks/<ID>.md — спецификация одного сложного среза, кодовые точки и ограничения;
  не копия таблицы статусов. Рутинная правка не требует нового файла.
- evidence/ — датированные результаты с областью и ограничениями. archive/,
  concept/, old_snowden/, старые snapshots — история по запросу.
- Код показывает фактическое поведение, а не отменяет контракт: расхождение
  записать как дефект. Исторический live PASS не является текущим health-check.

## 3. Работа с изменением

1. Проверить git status и точку входа. Пока нет первого commit, diff недостаточен:
   учитывать новые/untracked файлы и собственный список изменений.
2. Выбрать один срез OFF-пункта: цель, граница, приёмка. Для нового поведения:
   ID, дата, приоритет, owner, причина, verification, статус в PLAN до реализации.
3. Найти существующую реализацию/API/variants. Изменять поток данных целиком,
   сохраняя consumers; не вводить альтернативного владельца состояния.
4. Ограничивать область изменения; не форматировать/переписывать соседние файлы.
   Не менять зависимости и глобальные настройки без предметной причины.
5. Выполнять запрошенную проверку минимально достаточного уровня. План команды
   не означает запуск. После изменения входов старый результат не переиспользовать.
6. Обновить PLAN-пункт и STATE; подтверждённый урок — LESSONS. Journal только
   дополнить; исправление старой записи — новой датированной записью.
7. Завершение: что изменено, реальная проверка, непроверенное, следующий шаг.

## 4. Safety и секреты

- Protected selector не содержит direct. Явные private/split/bootstrap исключения
  ограничены политикой; не делать общий fallback из служебного исключения.
- Error/BLOCKED, shutdown engine и пользовательский Disconnect — разные события.
  FR-009 требует strict-барьер между retry; текущие дефекты — AUD-001/002.
- Protected probe доказывает соответствующий путь/egress, а не лишь открытый порт.
  UI не считает TCP/handshake/browser mock успешным VPN.
- Категория описывает наблюдение; timeout/RST не доказывают DPI. Memory/score/
  экспертный confidence ранжируют кандидатов, не создают evidence.
- Секреты, реальный runtime config, vault и auth env не читаются массово; значения
  не выводятся в diagnostics, чат, snapshots, stdout или source artifacts.
  Разрешённый reveal — отдельное явное действие, без логирования.
- Реальное provisioning не копировать в fixtures. Сверка — безопасные metadata/
  fingerprint. .gitignore не отзывает утёкший credential.
- Raw captures/evals/локальные результаты — ignored .artifacts/ и evals/runs/;
  в docs только необходимое redacted evidence. Нет доказанного отзыва токена —
  не писать «ротация выполнена».

## 5. Проверки и полномочия

Runner без --run показывает команды. Doctor статический: не запускает приложение,
native tests, package install или внешние запросы. Запрос настройки обвязки
разрешает её статическую диагностику. Тесты/сборки продукта добавляются и
запускаются в рамках запроса пользователя на проверку.

| Область | При запрошенной проверке |
|---|---|
| Harness/docs | node syntax + doctor + структура/ссылки/состояние |
| Go | узкий regression, обычные/tagged tests и vet по blast radius |
| Vue | vue-tsc → Vite; mock screenshot не подменяет native-проверку |
| Bindings | Wails generation с tags; frontend typecheck; generated не патчить |
| Race | только с CGO/C compiler; unavailable отдельно |
| Release | tagged frontend/Wails/NSIS + revision/hash; build-only отдельно |
| OS/live | отдельное разрешение/изолированный стенд, актуальное evidence |

Теги поставки: with_utls,with_gvisor,with_quic,with_clash_api, источник
windows/tools/package.ps1. Не использовать старые with_awg/with_wireguard.
npm ci, npm audit fix, Wails install, SSH, deploy и admin checks не являются
частью doctor. Проверять существование prerequisites до запуска.

## 6. Развитие архитектуры

Текущие слои и целевые владельцы — PLAN §4/§4.2. Реальные API:
Manager.Start/Stop/Reload/SwitchChannel; Engine.Start/Stop/Reload/SwitchDefault;
ProtectedProbe.Run. Нет production ReloadVPN, Engine.Close, Telegram runtime
и Android module в этом дереве. Android — будущая фаза. PostgreSQL не введён.
Новую Windows-службу/IPC выделять после contracts/regressions, не разносить
файлы во время исправления дефекта. Правила старого ядра не являются его API.

Длинные процедуры harness/оценки — HARNESS. Установленные skills применять
по задаче, не по наличию. Изменения моделей/permissions/MCP в эту настройку
не входят. Предыдущая редакция справочника сохранена в archive/AGENTS-pre-harness-2026-10-18.md.
