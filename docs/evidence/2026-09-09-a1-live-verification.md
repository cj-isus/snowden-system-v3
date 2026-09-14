# Evidence: A1.2 / A1.3 / A1.4 live-verification — 2026-09-09

> Правило проекта: значения секретов не печатаются нигде. Сверка — по
> SHA256-префиксам и лог-фактам.

## Контекст

Владелец сообщил ошибку UI: `рендер конфига: channel channel-a-vless: secret
"vless-uuid" unavailable: значение ещё не задано` — и попросил закрыть A1.2
(TUN admin smoke), A1.3 (HY2 live-verify) и A1.4 (selector reload) самому,
прямо запросив то, что не получится.

## 1. P0: хранилище стиралось ребилдом (найдено и исправлено)

- Причина ошибки владельца: vault жил в `build/bin/secrets/`, `wails build`
  очищает каталог — все 5 слотов оказались пустыми.
- Исправление: vault+logs переехали в `%AppData%\snowden-system\`;
  `migrateLegacyVault()` переносит старый файл один раз, не перезаписывая.
- Залито заново 5/5 `verify=ok`; канальные fingerprints совпадают с сервером:
  uuid `f210a06c2e077d59`, hy2 `a9383b3b87bfeae0`, obfs `236124c16317a5b7`.

## 2. A1.3: HY2 live-verified (ложь «UDP заблокирован» вскрыта)

- Pin-сертификат сервера получен по SSH с VPS (`/etc/ssl/vpn.example.com.crt`,
  self-signed `CN=vpn.example.com`, notAfter 2036-08-24) и установлен в
  AppData secrets; это публичный материал.
- `SelectChannel("channel-a-hy2")`: render → двойная валидация → Reload →
  обязательный probe → **6/6 PASS**; в логах sing-box соединения идут через
  `outbound/hysteria2[channel-a-hy2]` (4 записи), egress = ожидаемый
  `203.0.113.10`.
- Вывод: UDP/QUIC до VPS доступен из этой сети; прежний вывод «UDP фильтрован»
  был следствием сырых UDP-сокетов (не показатель для QUIC-сессий).
- Дескриптор `channel-a-hy2` → `validation_status: live-verified`,
  `validated_at: 2026-09-09`.

## 3. A1.4: SelectChannel (переключение каналов)

- Контракт: рендер с `WithDefaultChannel` → строгий парсер + sing-box →
  `Manager.Reload` → probe; неудача = fail-closed (Manager откатывает/останавливает).
- Contract-тесты: `TestRenderSelectorDefaultHonored` (default читается из
  собранного конфига), `TestRenderSelectorDefaultUnknownFallsBackToFirst`.
- Live-тест: START (default=vless) → SELECT → активен `channel-a-hy2` →
  post-select probe 6/6.

## 4. A1.2: TUN admin smoke (PASS через request_elevation)

- `wintun.dll` 0.14.1 (amd64, официальный билд, PE-заголовок проверен):
  прямые загрузки рвались сетью — доставлен через VPS-релей (base64 по SSH).
- Тег `with_gvisor` обязателен для TUN-стека (поймано на живом запуске).
- Рендер: `WithTUN()` → inbound `tun` c `interface_name: snowden0`,
  `auto_route`+`strict_route`, gvisor; строгий парсер принимает.
- Elevated-прогон: `inbound/tun[tun-in]: started at snowden0`, адаптер
  `snowden0` в состоянии «Подключён» (netsh), sing-box start 0.27s,
  probe **6/6** на старте и после, чистый Stop.

## 5. Владелец запустил exe (финальная проверка owner-сценария)

- Владелец нажал «Подключить» в UI; процесс жив, `127.0.0.1:1080` слушается.
- Независимая проверка из вне приложения (curl через SOCKS):
  `api.ipify.org` → `203.0.113.10` (VPS), `ifconfig.me/all.json` → `ip_addr:
  203.0.113.10`; direct без прокси → `192.0.2.44` (домашний IP).
- Вывод: туннель, поднятый владельцем из UI, работает end-to-end;
  активный канал — vless (default), селектор обслуживает трафик.

## 6. Владелец включил TUN из UI (V2-018, пользовательский прогон)

- Владелец нажал «Включить TUN» в UI; процесс перезапущен БЕЗ `--tun`
  (проверено по CommandLine) — значит переключение прошло через
  DisableTUN-ветку или UAC был отклонён/не завершён: режим процесса — SOCKS.
- При этом туннель работает корректно: SOCKS-порт слушается, egress через
  туннель `203.0.113.10` ≠ direct `192.0.2.44`, системный прокси-реестр
  выставлен (`socks=127.0.0.1:1080`, ProxyEnable=1) — это путь SOCKS-режима.
- Диагноз UX: после elevated-перезапуска приложение запускается в stopped —
  владельцу нужно ещё раз нажать «Подключить», и это неочевидно. **Исправлено
  сразу:** переключение режима передаёт `--auto-connect` новому процессу,
  который восстанавливает состояние (startup → go startCore). Юнит-тест
  парсинга флага добавлен; exe пересобран 17:13.
- Адаптер snowden0 не активен — ожидаемо для SOCKS-режима этого процесса.

## 7. TUN-переключатель в UI (V2-018)

- Контракт: EnableTUN = Stop → перезапуск exe с `--tun` (UAC через
  ShellExecuteW runas) → Quit; DisableTUN — то же без elevation.
  Манифест остаётся asInvoker (минимальные привилегии: SOCKS-режим
  администратора не требует).
- Флаг `--tun` включает render WithTUN (интерфейс `snowden0`, auto_route +
  strict_route, gvisor) и отключает системный прокси — маршрутизацию делает
  адаптер на IP-уровне, реестр не затрагивается.
- Юнит-тесты: парсинг флага, quoteArg (экранирование аргументов), guard
  «режим уже установлен», связка флаг→рендер (`--tun` ⇒ tun-inbound).
- Live (elevated): `--tun` → `inbound/tun[tun-in]: started at snowden0`,
  адаптер активен, probe 6/6; exe пересобран 16:49.

## 7. Гигиена

- Временные файлы значений (fill-спека, root-pw, cf_token) удалены;
  тестовые бинарники и livetun_out.txt удалены; wintun-архивы на VPS удалены.
- Build-теги зафиксированы: `with_utls,with_gvisor,with_quic`.
- exe пересобран (16:18) с wintun.dll рядом; vault в AppData больше не зависит
  от ребилдов.
