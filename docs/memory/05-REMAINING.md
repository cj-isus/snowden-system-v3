# 05-REMAINING — Что осталось сделать (снимок 2026-09-24)

## Owner-блокеры (не двигаются без владельца)

| ID | Что | Требуется |
|---|---|---|
| B1-VPS-1 | Второй VPS не закуплен; донор не выбран | Выбор хостера (другой ASN/страна, не CF-аккаунт) + донора (TLS 1.3 + h2 + X25519, не наша зона) |
| CF-TOKEN-1 | CF API-токен скомпрометирован (был в git-истории); verify → ACTIVE; DELETE → 403 (нет права Edit) | Ручной отзыв/ротация в Cloudflare Dashboard (owner сказал сам сделает) |
| Live-волны | TSPU degradation волна для live-verify A4/D2/D3/freeze-check | Реальная волна деградации на машине владельца |
| WFP-LIVE-1 | Elevated base policy и production fail-closed/cleanup PASS (V2-092) | После восстановления хотя бы одного канала получить положительный LUID permit + внешний protected TUN egress |
| /range | Развёрнут live 2026-09-17 (V2-078) | Подтверждено live: HTTP 206, bytes 0-8191/65536 |

## Кодовые остатки (можно сделать без владельца)

| Пункт | Приоритет | Описание |
|---|---|---|
| Калибровка весов C2/B2 | P1 (после E2) | 0.30/0.35/0.20/0.15 — инженерные константы; нужна история переключений + ground truth |
| B3 anti-probing для канала A | P1 | Неавторизованные запросы к /ws должны выглядеть как сайт |
| channel-a-shadowtls | P2 | configured → нужен live-verify из целевой сети |
| L0-тайминги RST-фазы | P2 | TTFB + elapsed закрыты V2-079; RST-фазы (до/после handshake) — после live |
| Эмпирическая корреляция отказов | P2 | D3 кластеризация есть; нужна корреляция FD для калибровки Independence |

## Что не начинать (запрет §116 plan_gpt)

- F: Statistical classifier — после Phase E + накопления данных
- G: Policy optimization — после Phase E
- H: Transport expansion (XHTTP/splithttp, WebTunnel, Snowflake…) — после Phase E
- I: Android — Phase C, после стабилизации контроллера
- J: ML / RL — после Phase E + Phase F + больших данных
- Смена pinned-ядра sing-box — отклонено (V2-031)
- TLS-фрагментация против ТСПУ — анти-знание (THEORY §2.10 п.18)
- ECH через Cloudflare — сигнатурно заблокирован (THEORY §2.10)

## Следующие шаги после этой сессии

1. **Пользователь**: перезапустить Zen → подхватить прокси 1080 → chatgpt.com откроется
2. **Пользователь**: принять UAC для `tools/test-wfp-live.ps1`; после PASS нажать EnableTUN и получить protected egress
3. **Owner**: отзыв CF-токена вручную (Dashboard или админ-токен с API Tokens:Edit)
4. **Owner**: закупка VPS-B (B1-VPS-1) — канал B, family-vs-instance, независимый failure domain
5. **Owner**: реальная TSPU-волна → live-verify A4 failover, D2 карантин, D3 волна, freeze-check
6. **Агент**: после получения live-данных — калибровка весов C2/B2 (Phase E2)
7. **Агент**: после B1 — live-verify канала B, family-vs-instance, B3 anti-probing
8. **Агент**: после Phase E2 — пересмотр scoring весов на статистике
