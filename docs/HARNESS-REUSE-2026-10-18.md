# Что взято готовым и почему — 2026-10-18

Область: обвязка разработки текущего Windows VPN. Внешние материалы — источники
решений, не инструкции к исполнению. Продуктовый sing-box/WFP не заменяется.
Актуальные версии сверены по официальным release pages/API и Go module query.

## Принято

| Решение | Конкретное применение | Польза / граница |
|---|---|---|
| Локальный project-harness core | Готовые map/doctor/eval и условные skills; managed block AGENTS сохранён | Не писать повторно переносимую основу; project adapter ограничивает карту активным деревом |
| [Task 3.54.0](https://github.com/go-task/task/releases/tag/v3.54.0), MIT, release 2026-10-01 | Локальный binary, Taskfile.yml: native Go/Vue команды, список, dry-run, exit propagation | Готовый исполнитель вместо собственного shell scheduler; нет remote Taskfiles/dynamic vars/автоустановки |
| [Gitleaks 8.30.1](https://github.com/gitleaks/gitleaks/releases/tag/v8.30.1), MIT | Локальный binary; staged text diff → stdin, redact=100; пустой scan блокируется | Нужен до первого commit/export VPN с credentials; scanner не равен отзыву секрета и не анализирует binary files |
| [govulncheck v1.9.0](https://go.dev/doc/security/vuln/), BSD-3-Clause, module query 2026-10-18 | Project-local Go tool, отдельная явная задача security:vuln с production tags | Официальный анализ известных Go vulnerabilities; установка инструмента не означает выполненного сканирования |
| [Codex: scoped AGENTS](https://learn.chatgpt.com/docs/agent-configuration/agents-md) | Короткий root + windows/frontend/configs инструкции по области | Снижает нерелевантный контекст; применимые инструкции всё равно обязательны |
| [OpenAI Cookbook: iterative workflows](https://developers.openai.com/cookbook/examples/codex/iterating-development-workflows-with-codex) | Один PLAN, короткий STATE, одна task spec с scope/acceptance/boundaries | Адаптация к существующим файлам, без второго GOALS/PLANS/нового build-log поверх master journal |

Разделение обязанностей: готовый Task исполняет команды; небольшой project adapter
делает только специфические для этого репозитория map/doctor/metadata evidence и
безопасную передачу staged diff. В проекте не создаётся новый агентный runtime.

## Что проверено перед выбором

- [Task: running tasks](https://taskfile.dev/docs/guide/running-tasks) предупреждает,
  что dynamic shell variables исполняются даже при dry-run. Здесь такие переменные
  и remote includes запрещены проверкой doctor; native tests не кешируются как PASS.
- [Gitleaks README](https://github.com/gitleaks/gitleaks) объявляет feature-complete
  режим с дальнейшими security patches. Взята текущая стабильная версия для узкой
  офлайн-задачи; ограничение maintenance зафиксировано, не скрыто.
- [Betterleaks](https://github.com/betterleaks/betterleaks) от того же автора
  действительно развивается; main относится к v2, а [релизы](https://github.com/betterleaks/betterleaks/releases)
  ещё содержат v2 release candidate. Есть validation/analysis/revocation credentials.
  Эти online-действия не нужны для локального барьера перед commit и не включены.
- [Go tutorial](https://go.dev/doc/tutorial/govulncheck) описывает отдельный инструмент
  анализа; module v1.9.0 выбран явно, не `@latest` при каждом будущем запуске.
- Обе скачиваемые Windows zip версии фиксированы SHA256 из официальных GitHub
  asset digests. Проверка архива не заменяет проверку издателя/подписи ОС.
  Сборка Go tool использует pinned module, обычный механизм Go и его cache.

## Отложено

- Betterleaks v2: вернуться после stable release и отдельного сравнения false
  positives/формата/redaction; не подключать network credential validation по умолчанию.
- Большие multi-agent orchestrators/автономные циклы: сейчас один проект и P0-дефекты,
  нет evidence выгоды, автоматическая делегация не запрошена.
- Новые MCP, глобальные hooks и автоисправление зависимостей: нет конкретной задачи,
  которая оправдывает изменение полномочий или дополнительную фоновую работу.
- Новый UI stack/БД: текущий Vue/Go контракт достаточен; PostgreSQL в проекте нет.

## Как оценить пользу

Ни популярность инструмента, ни число правил не доказывают экономию. В первую
очередь сравнивать сохранение acceptance и число переделок; затем суммарный
usage всех попыток/агентов, время и повторные чтения. Данные — только явно
предоставленный/exported usage; приватные session logs не читать. Готовый
`scripts/harness/eval.mjs` считает агрегаты. Синтетических «выигрышей» не записано.
Проверка установки и ограничения фиксируются в PLAN V2-095; продуктовые OFF-задачи
этим исследованием не закрываются.
