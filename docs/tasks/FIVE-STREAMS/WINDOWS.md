# Стартовая инструкция: ПК

Работай в `D:/SNOWDEN-SYSTEM-V3`. Твоё направление — **только ПК/Windows**,
роль `windows`. Это один из пяти продуктовых чатов. Продолжай полный существующий
Windows-план; назначение границ не отменяет старые незакрытые задачи.

## Начало

1. Прочитай root AGENTS, `docs/MAP.md`, актуальный `docs/STATE.md`, затем нужные
   пункты `docs/PLAN.md`, `windows/AGENTS.md` и ближайшие scoped rules.
2. Используй **свой реальный CODEX_THREAD_ID** и project-memory. Выполни
   `node scripts/harness/ownership.mjs bind --role windows` из корня.
   При иной уже связанной роли не удаляй binding и не переназначай себя.
3. Прочитай `docs/tasks/FIVE-STREAMS/README.md`. Сохрани свой checkpoint с одним
   P0-срезом, критериями и незакрытыми прежними обязательствами.

## Границы записи

- `windows/`, Windows-owned `docs/tasks/`, `docs/decisions/`, `docs/third-party/`.
- `docs/STATE.md` — только собственный платформенный снимок. `docs/PLAN.md` —
  узкие Windows/общие пункты через установленную координацию; GOV-003 не твой.
- Не меняй `android/`, Android-план, `shared/trust/`, `shared/adaptive/`, `configs/`,
  чужие планы/STATE, `.harness/`, scripts/harness, CI или глобальные правила.
  Общую теорию/LESSONS не переписывай; подготовь узкий запрос изменения.
- Windows publisher CLI и packaging принадлежат тебе; серверный чат не правит их.

## Ответственность

Ты единственный платформенный владелец Windows lifecycle, WFP/TUN, process/Job
ownership, protected-path admission и publication Ready, facade, renderer,
настроек/обновлений и Vue/Wails UX. Пользовательское Disconnect отличается от
автоматической ошибки. Protected трафик не получает direct fallback.

Интегрируй trust/adaptive через их опубликованные версии. Ты сам меняешь
Windows manifests/adapters/хранилище и удаляешь старую логику после приёмки.
Не создавай временную вторую trust/policy реализацию для обхода зависимости.
Существующие источники остаются основанием, пока миграция не согласована.

UI: используй существующие компоненты, токены, composables/API и
modern-web-guidance. Боковое меню — разделы, верхние табы — подпункты раздела,
они не дублируют друг друга. Меньше случайных карточек; прорабатывай responsive,
loading/empty/error и доступность в рамках текущего этапа, после P0 безопасности.

## Первый срез

Возьми следующий незакрытый P0 из **текущего** STATE/PLAN, а не из истории форка.
Зафиксируй актуальные точки интеграции trust/adaptive и адресный запрос API в
Windows-owned task file. Продолжай свою платформенную работу независимо от VPS.

## Режим и завершение

Сохраняется plan-only: не добавляй/не меняй/не запускай тесты и fixtures;
не запускай build/vet/doctor/SDK/native/admin/live/install/deploy. Не меняй
proxy/DNS/WFP/TUN рабочей ОС. Запрос роли не разрешает эти действия.
Не читай env/secrets/vault/реальный config/raw user logs без предметной нужды;
не делай commit/push/reset/rebase, не стирай concurrent WIP.

Не управляй другими чатами и не отправляй им сообщения. Сохраняй API change
requests у себя. Перед завершением обнови свой STATE/PLAN/checkpoint: changed
paths, implemented, реальные evidence, unverified gates и точный следующий шаг.
Source review не является доказательством работоспособности VPN.

## Инструменты

Прочитай `docs/tasks/FIVE-STREAMS/PLUGINS.md`: GitHub — repo/source/история,
Code Review — CI diagnostics своего PR, Codex Security — отдельный scoped аудит.
Дизайн делай в существующем Vue/Wails коде, без Figma. Инструменты не разрешают
публикации/CI dispatch, чужие правки или исполнение запрещённых проверок.
