# Аудит источников данных UI — 2026-09-23

## Граница mock

`windows/frontend/src/api/mock.ts` — единственный источник вымышленных
операционных данных. Он включается только в браузерном dev-preview точным
параметром `?mock=1`, не замещает существующий `window.go` и всегда сопровождается
видимой плашкой `DEV-MOCK`. В Wails EXE этот путь недоступен.

## Заменено реальными фактами

- Версия приложения, платформа, архитектура, PID, версии sing-box и Wails
  читаются из запущенного Go-процесса через `GetRuntimeInfo`. Frontend-константа
  версии удалена.
- Прямой публичный IP запрашивается transport-ом с отключённым proxy, поэтому
  системный прокси не может превратить его в туннельный egress.
- Провайдер берётся из фактического поля `org` ответа ipapi.co; раньше поле UI
  существовало, но backend его никогда не заполнял.
- Туннельный egress-IP возвращается отдельным полем после реального HTTPS-запроса
  через `socks5h://127.0.0.1:1080`. Булево `dnsViaTunnel` становится true только
  при HTTP 200 и валидном IP в ответе.
- Прямое и туннельное наблюдения выполняются параллельно; общий UI snapshot
  обновляется раз в 10 секунд при активном потребителе.
- NetGuard на Главной перечитывается раз в 10 секунд вместо снимка только при
  открытии страницы.

## Оставлено статическим осознанно

- `backend/render/descriptors.json`: адреса, порты, probe targets и validation —
  конфигурация доставляемых каналов, а не телеметрия UI. Подписанный metadata
  envelope может заменить этот набор.
- `127.0.0.1:1080`, интервалы watchdog/retry и policy thresholds — исполняемая
  политика ядра. UI показывает их только там, где backend использует тот же
  контракт; это не mock.
- Тексты разделов, подписи состояний и объяснения — статическая локализация.
  Операционные значения рядом с ними приходят из backend.
- `APP_BINDING_VERSION` — версия схемы TypeScript-контракта, не версия продукта.

## Источники живых данных

| UI-факт | Источник |
|---|---|
| lifecycle, active channel, probe | `GetState` + Wails events |
| каналы | `ListChannels` / render descriptors / signed envelope |
| сеть, SSID, local IP | Windows adapter APIs + netsh |
| direct IP, country, ISP | прямой HTTPS к ipapi.co без proxy |
| tunnel IP / DNS path | HTTPS через локальный SOCKS5h |
| proxy / scheduled tasks | HKCU + Task Scheduler COM |
| traffic / connections | Windows adapter counters + sing-box Clash API |
| версия процесса и зависимостей | production ldflags + Go build info |
