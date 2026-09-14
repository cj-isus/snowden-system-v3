# Инфраструктура snowden.system — серверы, секреты, процедуры

> Обновлено: 2026-09-08. Файл документирует фактическое состояние, проверенное
> live. Секреты в этом файле НЕ печатаются — только расположения и хеши.

## 1. Сервер

| Параметр | Значение |
|---|---|
| IP | 203.0.113.10 (публичный, известен цензору — см. риски) |
| Провайдер | AdminVPS, AS57043, Amsterdam NL |
| OS | Ubuntu 24.04.4 LTS, kernel 6.8, KVM, 1 vCPU/2GB |
| SSH | root@203.0.113.10:22, ключ `~/.ssh/vps_key` (ed25519, установлен 2026-09-09; парольная аутентификация — резерв, пароль в DPAPI-хранилище) |
| hostname | kopilot.com |

## 2. Сервисы на сервере

```text
nginx  :443/tcp  — TLS-терминация (Cloudflare origin cert, 15 лет),
                   location /ws → proxy_pass http://127.0.0.1:8443
                   (proxy_http_version 1.1, Upgrade/Connection headers,
                    Sec-WebSocket-Protocol НЕ переопределяется — идёт как есть)
sing-box 1.13.19:
  vless-in    127.0.0.1:8443/tcp  (ws, path /ws, early_data header)
  hy2-in      0.0.0.0:8444/udp   (salamander obfs, tls cert /etc/ssl/vpn.example.com.crt)
  outbounds: direct, block; route.final = direct  ← КРИТИЧНО: не block!
  log level: info
firewall (nft/ufw): 22/tcp, 80/tcp, 443/tcp, 8443/udp(legacy), 8444/udp
```

Важно: порт 8443 занят vless-in (TCP, localhost). HY2 слушает 8444/UDP.
Старый inventory.yaml ошибочно указывал HY2 на 8443.

## 3. Домены и Cloudflare

```text
vpn.example.com    — zone active (CF), A @ → 203.0.113.10 proxied
legacy-zone.dpdns.org — zone active (CF), A @ → 203.0.113.10 proxied
SSL mode: full; edge TLS 1.3; оба домена отвечают 200 с edge
```

Оба домена — один VPS и один аккаунт CF = общий failure domain. Независимый
канал B требует отдельного VPS/ASN (Phase B1).

## 4. Секреты (расположения, не значения)

| Секрет | Файл | Примечание |
|---|---|---|
| VLESS UUID | secrets/channels/vless_uuid | тестовый; ротация после acceptance |
| HY2 password | secrets/channels/hy2_password | тестовый |
| HY2 obfs password | secrets/channels/hy2_obfs_password | тестовый |
| VPS SSH key | путь в vps.env (VPS_SSH_KEY_PATH) | |
| CF API token | vps.env (gitignored) + DPAPI-хранилище | Проверен live 2026-09-09: `/user/tokens/verify` → ACTIVE (sha256-префикс 0000000000000000, id 00000000000000000000000000000000). История утечки в git (§7/§8) — решение об отзыве/ротации за owner |
| DPAPI-хранилище секретов | `%AppData%\snowden-system\secrets\vault.v1.json` (+ `hy2_pin.pem` рядом) | 5 слотов verify=ok 2026-09-09; П0 2026-09-09: прежний путь `build/bin/secrets` стирался ребилдом — путь сменён на AppData (migrateLegacyVault переносит старый файл); наполнение — `windows/tools/vaulttool fill` (значения только файл→DPAPI, никогда не печатаются) |
| Server certs | /etc/ssl/snowden*.crt|.key на VPS; публичный PEM самоподписанный — вытянуть при необходимости: `openssl s_client -connect <host>:<hy2-port> | openssl x509` | публичная часть — не секрет |
| CF origin cert/key | /etc/sing-box/certs/cloudflare-origin*.pem | для nginx |
| Metadata signing | см. inventory-secrets.env | Phase B2 |

Правила:
1. Значения не печатаются в чат/логи/diff. Сверка client↔server — по
   `sha256(value)` (пример в журнале V2-005 процесса 2026-09-08).
2. Рендер runtime-конфига: `client.local.json` (gitignored) из шаблонов +
   секретов. Шаблоны в configs/singbox/ содержат только плейсхолдеры.
3. Ротация тестовых секретов после acceptance: scripts/rotate-channel-secrets.sh
   (обновить серверный конфиг + локальные файлы + client.local.json).

## 5. Пиннинг HY2 сертификата

Клиентский outbound hysteria2.tls:
```json
{ "enabled": true, "server_name": "vpn.example.com", "certificate": "<PEM>" }
```
`insecure` запрещён. При ротации серверного сертификата обновить PEM в рендере.
Текущий отпечаток: sha256 10:6D:5C:00:FF:ED:95:FC:03:0F:56:BE:43:50:DD:7B:D2:05:83:16:23:B9:90:ED:87:92:2D:4A:83:9E:3F:07

## 6. Известные ограничения окружения агента

- Агент сидит за внешним VPN (HAP): egress 138.124.67.78 (не равен локальному ISP);
- UDP в этом окружении фильтруется полностью (подтверждено повторно 2026-09-09:
  QUIC к 8.8.8.8:443 и UDP к 203.0.113.10:8444 оба молчат) → HY2
  проверяется только на устройстве owner'а в сети без UDP-фильтра;
- TUN требует elevation → admin smoke отдельно;
- Выводы о «работе из РФ» из этого окружения не делаются.

## 7. Runbook: быстрая серверная диагностика

```bash
# 1) жив ли sing-box и что слушает
ssh vps 'systemctl is-active sing-box; ss -tlnp | grep 443; ss -ulnp | grep 8444'
# 2) что в логах (blocked/error = сервер режет трафик)
ssh vps 'journalctl -u sing-box -n 50 | grep -iE "error|blocked"'
# 3) конфиг валиден и final=direct
ssh vps 'sing-box check -c /etc/sing-box/config.json && grep -o "\"final\": \"[a-z]*\"" /etc/sing-box/config.json'
# 4) nginx проксирует WS (ожидаём 101 от sing-box, не 400)
ssh vps 'tail -5 /var/log/nginx/access.log | grep /ws'
# 5) сквозной тест с клиента
curl --proxy socks5h://127.0.0.1:1080 https://api.ipify.org   # → 203.0.113.10
```

Правило: любое изменение серверного конфига = backup + `sing-box check` +
restart + post-verify. Без этого не выполнять.

## 8. Гигиена

- docs/cloudflare-ip-restriction.md содержит реальный API-токен → удалить файл,
  токен отозвать (P0, сделано 2026-09-08: файл удалён, отзыв — на owner).
- отрендеренные конфиги с секретами во временных каталогах → удалять после
  сессии (каталог был в .gitignore; 2026-09-09 — удалён, evidence дистиллирован
  в docs/evidence/).
