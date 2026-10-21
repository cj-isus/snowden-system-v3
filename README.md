# Snowden System

Адаптивный VPN-клиент для Windows: Go, Wails, Vue и embedded sing-box.
Разрабатывается с проверяемыми контрактами защиты, доставки подписанных
профилей, переключения каналов и восстановления после ошибок.

## Состояние

Проект находится в разработке. Локальные проверки и исторические сетевые
проверки описаны отдельно; актуальная доступность VPS не подтверждена.
Production TUN, аварийное восстановление Windows и сравнительные измерения
ещё имеют открытые критерии приёмки. Подробности — в
[плане](docs/PLAN.md) и [текущем состоянии](docs/STATE.md).

Публичная копия содержит шаблоны адресов и обезличенные примеры. Рабочие
профили, ключи, сертификаты, vault, логи и готовые EXE в неё не входят.
Исторические записи о live-проверках не относятся к адресам-заглушкам.

## Навигация

- [Карта кода](docs/MAP.md) — точки входа и владельцы компонентов.
- [План и критерии приёмки](docs/PLAN.md) — требования, backlog, решения.
- [Теория и методика экспериментов](docs/CENSORSHIP-THEORY.md).
- [Проверки](docs/TESTING.md) и [датированные результаты](docs/evidence/).
- [UI](docs/UI-DESIGN.md), [серверные шаблоны](configs/README.md).
- [Рабочие правила](AGENTS.md), [инструменты проекта](docs/HARNESS.md).

## Локальная разработка

Требуются Windows, Go 1.26, Node.js/npm, Wails v2 и WebView2.
Версии зависимостей закреплены в `windows/go.mod` и frontend lockfile.

```powershell
git clone https://github.com/cj-isus/snowden-system-v3.git
cd snowden-system-v3
npm --prefix windows/frontend ci
node scripts/harness/project.mjs doctor
```

Статический doctor не включает VPN и не меняет системную сеть.
Для просмотра интерфейса с демонстрационными данными:

```powershell
npm --prefix windows/frontend run dev -- --host 127.0.0.1
# Открыть напечатанный localhost URL с параметром ?mock=1
```

Для запрошенных локальных проверок:

```powershell
npm run test:ui
npm --prefix windows/frontend run build
cd windows
go test ./... -count=1 -timeout=120s
go test -tags with_utls,with_gvisor,with_quic,with_clash_api ./... -count=1 -timeout=120s
go vet ./...
```

Browser-тесты требуют установленного Playwright Chromium; команды и границы
изолированных OS/live-проверок — в [TESTING](docs/TESTING.md).
Обычный запуск на реальном профиле требует его отдельного provisioning;
публичные адреса `example.com` и TEST-NET не являются рабочими серверами.

Сборка EXE/NSIS описана в `windows/tools/package.ps1`; Wintun и NSIS
подготавливаются отдельно. Успех сборки сам по себе не подтверждает VPN-путь.
