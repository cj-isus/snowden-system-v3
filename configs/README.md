# configs/ — серверные конфигурации и деплой

> Создан 2026-09-17 (V2-076) как честная замена «висячей» ссылке на несуществующий
> `configs/README.md` из AGENTS §1/§7. Источник правды по требованиям — `docs/PLAN.md`
> (FR-005/FR-006), по фактическому состоянию сервера — живой сервер + журнал PLAN §6.

## Что здесь лежит

| Файл | Роль | Границы |
|---|---|---|
| `singbox/server-combined.json` | Шаблон подстановки секретов для **базового** контура: `vless-in` (WS, localhost) + `hy2-in` (UDP, salamander + TLS) + `route.final=direct` | **НЕ воспроизводит прод целиком.** На сервере дополнительно развёрнуты (journal V2-044/V2-051/V2-041): REALITY-инбаунд 8443/tcp, ShadowTLS 8445/tcp, WS переехал на 9443/localhost, `multiplex` в `vless-in`, WARP-endpoint и правило маршрутизации AI-доменов. Шаблон — минимальный валидный baseline, а не «копия сервера» |
| `deploy-vps.sh` | Подстановка секретов из `secrets/channels/` (gitignored) → scp → `sing-box check` + проверка `route.final` | Рестарт ОСОЗНАННО не автоматизирован (не рвать живой туннель) |
| `verify-server.sh` | **Post-deploy инварианты** (V2-076): сервис active, конфиг валиден ядром, `route.final == direct`, обязательные листенеры, ufw, `/ip` = 200, `/range` = 200/206 | Только чтение (is-active/check/ss/grep/curl). Секреты не читает и не печатает |

## Порядок работы с сервером

```bash
bash configs/deploy-vps.sh          # подстановка + деплой + config-level проверки
ssh <vps> 'systemctl restart sing-box'
bash configs/verify-server.sh      # инварианты живого сервера (FAIL → exit 1)
```

Правило (FR-006): любое изменение серверного конфига = **backup + `sing-box check` +
restart + post-verify**. `verify-server.sh` — тот самый post-verify, переведённый из
ручного runbook в код (урок §3.2 CENSORSHIP-THEORY: `route.final=block` ронял трафик
при внешне «подключённом» клиенте).

## Секреты

Реальные значения — только в gitignored-файлах (`secrets/channels/*`, `vps.env`),
в этом каталоге их нет и быть не должно. Сверка клиент↔сервер — по `sha256`
значения, не по значению (PLAN §7).