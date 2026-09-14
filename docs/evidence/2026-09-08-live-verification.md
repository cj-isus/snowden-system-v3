# Evidence — live-verification 2026-09-08

Дистиллят сырых логов live-проверки (оригиналы были в неигнорируемом временном
каталоге и удалены после очистки). Файл не содержит секретов: только метки,
состояния и публичные факты. Хеши секретов — в docs/INFRASTRUCTURE.md.

## Канал A: VLESS+WS+TLS через Cloudflare — live-verified

Источник: клиентский лог встроенного sing-box (SOCKS-режим, рабочий конфиг из
тестовых секретов), сессия 18:46–19:08 local.

```text
inbound/mixed[socks-in]: tcp server started at 127.0.0.1:1080
outbound/vless[vless-ws-tls]: outbound connection to api.ipify.org:443   ← egress-check #1
outbound/vless[vless-ws-tls]: outbound connection to ifconfig.me:443     ← egress-check #2
VPN running, state: running                                              ← probe pass → running
```

Дополнительные независимые факты той же сессии:

- curl через SOCKS :1080 вернул egress = IP VPS (203.0.113.10), тогда как
  прямой запрос с той же машины даёт другой IP → граница измерения подтверждена
  (трафик действительно идёт через туннель, не мимо).
- DNS-check через туннель — pass (разрешение целевых доменов внутри сессии).
- Server-side: sing-box journal содержит `[user-1] inbound connection` от
  нашего клиента; nginx access.log — 101 для WS-upgrade; 400 от sing-box ранее
  был ответом на невалидный base64 early-data в тестовых запросах (не баг nginx).

## Fail-closed валидатор — подтверждён

Источник: лог попытки запуска с шаблонным (незаполненным) HY2-outbound:

```text
Failed to load config: validate config: placeholder or secret-shaped value in outbound "hysteria2"
```

Движок отказался стартовать с placeholder-конфигом — ключевой контракт §1(1)
PLAN.md работает.

## Канал C: HY2 (UDP) — configured, не проверяем из сети агента

- Секреты клиента/сервера совпадают (сверка по хешам), TLS с certificate
  pinning проходит до QUIC-handshake.
- QUIC-handshake не получает ответа; контрольный QUIC к 8.8.8.8:443 из этой же
  сети тоже молчит → сеть агента фильтрует UDP. Это ограничение среды
  проверки, не доказанный дефект канала.
- Server-side: nft counter видел UDP-пакеты канала (8 pkts), sing-box hy2
  inbound на debug-уровне не логировал handshake → пакеты терялись до демона
  либо глушились на пути; из чистой сети требуется повтор.
- Статус в PLAN.md: configured (не live-verified). Проверить с пользовательской
  машины (домашняя/мобильная сеть) перед включением в production selector.

## Очистка

После фиксации evidence удалены: временные конфиги с секретами, клиентские
бинарники и логи (каталог .probe-tmp). Git-игнор покрывал каталог на весь
период тестирования.
