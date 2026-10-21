# 07-PROCESS-RULES — Правила процесса (снимок 2026-09-20)

> Историческая редакция. Актуальны корневой AGENTS.md, scoped AGENTS,
> docs/AGENTS.md и PLAN V2-095. Перечень чтения/команд ниже не запускать как workflow.

## Порядок работы (AGENTS §1)

1. Прочитать docs/PLAN.md (§1 принципы, §3 контракты FR, §5 фазы, §6 журнал)
2. Прочитать docs/AGENTS.md (правила)
3. Прочитать docs/CENSORSHIP-THEORY.md (теория)
4. Прочитать docs/INFRASTRUCTURE.md (факты сервера)
5. Прочитать docs/B1-REALITY-RESEARCH.md (канал B)
6. Прочитать configs/README.md (серверные конфиги)
7. Прочитать docs/memory/*.md (снимок состояния)
8. git status --short --branch (проверить чужие изменения)
9. Проверить mtime файлов перед правкой (параллельная сессия!)
10. Проверить PLAN.md на занятость V2-номеров перед присвоением

## Запреты

- НЕ делать commit, push, reset, rebase, deploy без явной просьбы
- НЕ запускать SSH или команды на VPS без явного разрешения
- НЕ делать npm audit fix или массовое обновление dependencies
- НЕ удалять/откатывать/перезаписывать чужие изменения
- НЕ запускать production scripts без понимания последствий
- НЕ делать массовое reformatting чужих файлов
- НЕ запускать приложение при подключённом туннеле без согласования
- НЕ печатать значения секретов

## Порядок создания новой фичи

1. Запись в PLAN.md §6 (ID, дата, статус, формулировка, причина, способ проверки)
2. Код
3. Тесты (unit — без сети; live — с тегом live)
4. go test ./... -count=1 && go vet ./... && gofmt чисто
5. Frontend build (если UI затронут): npm run build
6. Wails bindings генерация (если новый биндинг App): wails generate module -tags "..."
7. Обновить PLAN §5 (фазы) если нужно
8. Обновить docs/CENSORSHIP-THEORY.md если есть теоретическая часть
9. Журнал: обновить статус записи с evidence (имена тестов, результаты)

## Порядок деплоя на сервер

1. bash configs/deploy-vps.sh (подстановка секретов + деплой + config-level проверки)
2. ssh $VPS 'systemctl restart sing-box'
3. bash configs/verify-server.sh (инварианты живого сервера)
4. curl --proxy socks5h://127.0.0.1:1080 https://api.ipify.org → VPS IP

## Риск-классификация проверок

Документационная правка: diff + ссылки достаточно
Код: минимальный релевантный набор тестов, расширяя по blast radius:
  - Windows backend: go test ./... && go vet ./... && go build -tags "..." ./...
  - Frontend: npm run build (vue-tsc + vite)
  - Cloudflare: node --check configs/cloudflare/worker.js
  - Android: flutter analyze && flutter test && flutter build apk

## Acceptance checklist перед «готово»

- git diff не содержит чужих/случайных изменений
- runtime config валиден и без placeholders
- protected selector не содержит direct и несуществующих тегов
- Start/Reload/Stop cleanup проверен
- relevant unit tests, typecheck/build и vet пройдены
- unavailable data явно показана как unavailable
- secrets отсутствуют в diff, логах и diagnostics
- platform-specific live blockers отдельно указаны
- документация синхронизирована с фактическим поведением

## Приоритеты плана исправлений (снимок)

P0: порт канала (V2-068 ✅) + FAIL кандидату (V2-075 ✅) + failing<no_data (V2-075 ✅)
P1: freeze-тишина (V2-071 ✅) + TUN control (V2-071 ✅) + self-hosted probe (V2-072 ✅)
P1: ASN+clamp (V2-073 ✅) + UI independence (V2-074 ✅) + server invariants (V2-076 ✅)
P2: preference probe-gate (V2-077 ✅) + L0-тайминги (V2-079 ✅) + TFO fix (V2-080 ✅)
P2: mutex race fix (V2-080 ✅) + EOF hint (V2-077 ✅)
Остаток: калибровка C2/B2 (owner), live-волны (owner), B1-VPS-1 (owner), CF-TOKEN-1 (owner)
