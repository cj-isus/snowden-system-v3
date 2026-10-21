# 06-LIVE-TESTS — Live-проверки на сервере и машине (снимок 2026-09-20)

## Серверные live-проверки (выполнены 2026-09-20)

```bash
bash configs/verify-server.sh
```

Результат (после фикса listener regex):

```
PASS  service     sing-box active
PASS  config      sing-box check ok
PASS  route       route.final="final": "direct"
PASS  listener    tcp/443 слушает
PASS  listener    tcp/8443 слушает
PASS  listener    tcp/9443 слушает
PASS  listener    tcp/8445 слушает
PASS  listener    udp/8444 слушает
WARN  firewall    в ufw нет правил: 9443/tcp (возможно, порт открыт иначе)
PASS  /ip         https://vpn.example.com/ip → 200
PASS  /range      Range-запрос → 206
ИТОГ: инварианты выполнены (WARN=1)
```

## /range развёрнут (live 2026-09-20, V2-078)

```nginx
location = /range {
    alias /var/www/snowden/range.bin;
    add_header Accept-Ranges bytes always;
    default_type application/octet-stream;
}
```

- /var/www/snowden/range.bin: 65536 bytes (65536 x 0x00)
- backup: /etc/nginx/conf.d/snowden-vpn.conf.bak.<timestamp>-range
- live verify: HTTP/2 206, content-range: bytes 0-8191/65536

## Tunnel live-проверки (через SOCKS 127.0.0.1:1080)

```bash
# egress
curl -sS --proxy socks5h://127.0.0.1:1080 https://api.ipify.org
# → 203.0.113.10 (VPS)

# chatgpt trace (метод V2-041 — проверка WARP-маршрута)
curl -sS --proxy socks5h://127.0.0.1:1080 https://chatgpt.com/cdn-cgi/trace
# → warp=on, ip=104.28.219.140 (WARP egress), h=chatgpt.com

# chatgpt main page (curl получает 403 challenge — браузер проходит сам)
curl -sS -o /dev/null -w '%{http_code}' --proxy socks5h://127.0.0.1:1080 https://chatgpt.com/
# → 403 (ожидаемо: JS challenge, V2-041)

# google
curl -sS -o /dev/null -w '%{http_code}' --proxy socks5h://127.0.0.1:1080 https://www.google.com/generate_204
# → 204
```

## UI build verification

```text
npm run build (vue-tsc --noEmit + vite): ok
81 modules, dist/index.html 295.12 kB
wails generate module: ChannelIndependenceInfo в KnownStructs
wails build (production): snowden-system.exe 36.0 MB
```

## Что НЕ проверено live

| Что | Требуется | Причина |
|---|---|---|
| EnableTUN с V2-080 fix | owner нажать в приложении | elevated-процесс нужен админ |
| TSPU-freeze волна | owner/reальная сеть | нужна живая волна деградации |
| channel-a-shadowtls | owner/целевая сеть | configured, не автопереключается |
| channel-a-reality после TFO fix | owner | tfo-go убран, нужен пересбор + live |
| D2/D3 UI карточки | owner/реальная волна | нужны реальные отказы |
| A4 watchdog failover | owner/реальная волна | нужен реальный отказ канала при живом туннеле |
| channel B | owner/закупка VPS-B | B1-VPS-1 blocker |