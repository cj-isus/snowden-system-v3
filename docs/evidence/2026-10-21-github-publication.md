# Публикация проекта в GitHub — 2026-10-21

## Область

Владелец явно запросил обновить `cj-isus/snowden-system-v3` и удалить
`cj-isus/snowden_system_vpn`, `cj-isus/dead-project`, `cj-isus/Snowden_system`.
Остальные репозитории, локальные архивы и рабочие credentials сохраняются.

## Подготовка

- Рабочий корень не имеет первого коммита; origin уже указывает на нужный URL.
- Remote main до публикации: `753500e87bc07a804d27bd54022cedda935d027c`.
- Существующий `.tmp/clean-export` имеет staged изменения и не используется
  для новой публикации. Отдельный clone сохраняет remote history.
- `.gitignore` дополнен исключениями локального архива, логов, credentials,
  сертификатов, runtime data, зависимостей и двоичных результатов сборки.
- В изолированном snapshot известные инфраструктурные адреса и конкретные
  legacy credential values заменяются плейсхолдерами; actual vault не читается.
  Исходники рабочего приложения остаются прежними.
- README описывает реальное состояние, структуру и локальный запуск.

## Приёмка

Публикация выполняется после отдельного scan/review выбранного snapshot.
Gitleaks 8.30.1 получен из закреплённого официального release, SHA256 совпал
с `.harness/tools.lock.json`. Исторический scan исходного remote выявил два
generic-api-key match в статических fake ShadowTLS fixtures. Текущий snapshot
даёт те же два match и один в синтетическом `SecretIDs` mapping, который
содержит идентификатор слота, а не credential. Проверены fake-store context
и несовпадение test credential с actual owner ShadowTLS credential.
Gitleaks exit=1 сохранён как есть: три reviewed findings, ноль необъяснённых;
глобальный allowlist/baseline не добавлен. Scanner не доказывает отсутствие
любого возможного секрета.

## Результат

- Push в `main` выполнен: `dae1f3c2fefd1d723f803dbceb8910bbfe58202b`.
  GitHub API подтвердил commit, parent `753500e…` и tree
  `e64a010602678484ab276e3903b0b89f3a53542c`. История сохранена, force не применялся.
- Remote recursive tree: 323 файла, `truncated=false`; нет путей secrets,
  local archive, node_modules, logs, .tmp/.tools/.artifacts и приватных
  расширений. Включены UI, backend, tests, теория V2-109 и исходный архив теории.
- Перед commit проверены source SHA256 всех 323 выбранных files и точный staged
  file set. В snapshot изменены 30 files при обезличивании; рабочие application
  sources/credentials не переписаны. `dist/.gitkeep` сохраняет go:embed boundary.
- Harness: `node scripts/harness/project.mjs check --scope harness --run` —
  PASS, 1.7s, sourceBefore=sourceAfter. Собственные publication files проходят
  `git diff --cached --check`; общий diff выявляет прежние whitespace в архиве
  и generated bindings, которые в этом срезе не переформатируются.
- Вход через git credential manager работал; устаревший gh keyring token
  не использовался для push. Секреты авторизации не выводились/не сохранялись.
- Ни один из трёх старых репозиториев агентом **не удалён**. GitHub запросил
  sudo-mode verification. После отдельного согласия на три удаления владелец
  отозвал дальнейшее выполнение: «забей, я сам». Удаление передано владельцу;
  оно не отмечается PASS и автоматически не продолжается.

Продуктовые тесты, сборка EXE, live VPN и административные проверки
в этом срезе не запускаются. Прежние результаты V2-108 и документационная
работа V2-109 не являются подтверждением GitHub-публикации.
