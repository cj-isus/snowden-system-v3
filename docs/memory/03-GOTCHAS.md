# 03-GOTCHAS — Ловушки, уроки, готчи (снимок 2026-09-20)

## PowerShell

1. Select-String -Recurse НЕ существует → Get-ChildItem -Recurse | Select-String
2. Get-ChildItem -Include БЕЗ -Recurse тихо возвращает пустой набор
   (ложный «dead code», ложный «нет вызовов» — подтверждено)
3. Оператор ?? может не поддерживаться → if/else
4. Set-Content -Append НЕ существует → Add-Content
5. Переменная $PID read-only → не перезаписывать
6. cmd /c с кириллицей и %{...} в curl ломается → временный .sh + Git Bash
7. WSL bash не установлен → использовать C:\Program Files\Git\bin\bash.exe
8. Select-String выводит "2" при бинарном содержимом в файле → bash grep
9. git add -A -n показывает "warning: adding embedded git repository" для old_snowden
10. $ErrorActionPreference Continue + *> redirect → все ошибки в лог

## Git Bash / bash.exe

1. bash -lc с экранированием кавычек внутри PowerShell → ломается
   (использовать временный .sh файл или герметичный here-doc в Git Bash)
2. Set-Content с UTF8 BOM → bash видит BOM в первой строке → error
   (использовать [IO.File]::WriteAllText с UTF8Encoding($false))
3. %{} и $vars внутри curl в PowerShell → ломаются
4. here-doc <<'REMOTE' с вложенными кавычками → экранировать одинарные ' как '"'"'
5. shellcheck отсутствует в среде → bash -n только

## Go / Wails

1. go test в sanitized-копии пишет в общий %AppData% app.log
   (NewApp() инициализирует logging в реальный путь — дефект гигиены)
2. Windows time.Since может вернуть 0 ns (sub-tick = 0 мкс)
   → строгие >0 ассерты только для многотиковых интервалов
3. Wails SingleInstanceLock: второй процесс получает ERROR_ALREADY_EXISTS
   → FindWindowW → SendMessage → os.Exit(0) МОЛЧА (без логов, окна, crash-лога)
4. ShellExecuteW «runas» возвращается СРАЗУ — старый процесс ещё жив
   → mutex race при переключении режима (V2-080 fix: awaitSingleInstanceReleased)
5. wails generate module -tags "..." — создаёт wailsjs bindings
   (вручную файлы НЕ править)
6. wails build -tags "with_utls,with_gvisor,with_quic,with_clash_api"
   -ldflags "-X main.runningVersion=..." — production build
7. NSIS: installer.nsi обязателен UTF-8 BOM (кириллица), иначе ANSI → mojibake
8. sing-box config.Parse ≠ sing-box runtime parse (strict types, DisallowUnknownFields)
9. sing-box render: int-форма для badoption.Duration отвергается живым парсером
   (нужна duration-строка "60s", не 60)
10. tfo-go НЕ поддерживает TCP Fast Open на Windows
    (tcp_fast_open: true ронял каждое соединение REALITY — V2-080)

## Диагностика на живом сервере

1. SSH read-only: systemctl is-active sing-box; ss -tlnp; ss -ulnp;
   nginx -T | grep location; sing-box check -c config
2. Серверные изменения: backup + sing-box check + restart + post-verify
3. live curl: /ip, /range (Range bytes=0-8191 → 206)
4. engine.log: tail для inbound/outbound пути каждого соединения
5. Логи расхождение клиент/сервер локализует отказ за минуты
6. ss вывод содержит CRLF через SSH (Git Bash) — tr -d '\r'
7. Метод V2-041: chatgpt.com/cdn-cgi/trace через туннель → warp=on/ip=WARP

## Важные факты про сервер (live-verified 2026-09-20)

IP=203.0.113.10, AdminVPS AS57043 NL
nginx: listen 443 ssl http2, /ws → proxy_pass 9443, /range → range.bin 65536B
sing-box inbounds: vless-in localhost:9443(mux), hy2-in 0.0.0.0:8444,
  vless-reality-in 0.0.0.0:8443, shadowtls-in 0.0.0.0:8445, vless-st-in localhost:9444
route.final=direct
/ip → 200, /range → 206 (bytes 0-8191/65536)
ufw: 443/tcp, 8443/tcp, 8444/udp, 8445/tcp (9443 не в ufw — loopback)

## Секреты (расположения, НЕ значения)

vps.env: VPS_HOST, VPS_PORT, VPS_USER, VPS_SSH_KEY_PATH, CF_API_TOKEN, CF_ACCOUNT_ID
secrets/channels/: vless_uuid, hy2_password, hy2_obfs_password, shadowtls_password, vless_uuid_d, hy2_pin.pem
Vault: %AppData%/snowden-system/secrets/vault.v1.json
CF token: ACTIVE, DELETE → 403 (нет права API Tokens:Edit) — owner отзовёт вручную
REALITY: .tmp/reality.env (если существует)

## Git / GitHub

remote: https://github.com/cj-isus/snowden-system-v3 (origin добавлен)
GitHub: public, main @ 753500e (6 commits sanitized)
Локально: ветка main, 0 коммитов, всё untracked
Публикация: scripts/make-clean-export.sh --check (leak-scan + build/test в копии)
Push: только через sanitized export (leak-scan clean + build ok)

## Что осталось

1. B1-VPS-1: закупка второго VPS (другой ASN/страна) + донор REALITY → канал B
2. CF-TOKEN-1: отзыв токена вручную (owner сказал сам сделаю)
3. Live-волны: TSPU degradation для live-verify A4/D2/D3/freeze
4. Калибровка весов C2/B2: Phase E2 (история переключений)
5. Zen restart: пользователь должен перезапустить Zen для подхвата прокси 1080
6. TUN crash: тест EnableTUN с новой сборкой после V2-080 fix
7. L0-тайминги на живом /range: запустить после /range + реальной сети
8. channel-a-shadowtls: configured → нужен live-verify из целевой сети
9. Файлы с PLAN-V3 ссылками в комментариях: 25 строк в 19 файлах