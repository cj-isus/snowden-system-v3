# R02-004 — первоисточники и границы

26 октября 2026. Captures:
`.artifacts/research/theory-02/2026-10-26-r02-004/`.

## Локальная цепочка

- `windows/go.mod`: sing-box1.13.19, sing-shadowtls0.2.1,
  sing-vmess0.2.8-0.20250909125414-3aed155119a1, sing-shadowsocks2 0.2.1.
- `windows/backend/render/render.go:692–731,816–840`: композиция и добавление
  в selector; статическое чтение, никакого renderer/Parse execution.
- `windows/backend/config/config.go:409–440,579–613`: admission исключение и
  неверное обоснование encryption. Реальные configs/profiles не читались.
- `android/.../configuration/NativeGraphSecurity.kt:17–30`: отдельный current subset;
  ShadowTLS не принят, TLS обязателен. `NativeGraphDefinitions.kt`: graph edges.
- `android/native/source-lock.json`: pinned core; наличие/работа AAR не проверялись.
- `docs/CENSORSHIP-THEORY.md`: новизна относительно сохранённого сборника.

## Actual pinned source

| Pin | Файлы / наблюдение |
|---|---|
| **sing-shadowtls v0.2.1**, `af3279d7446641d5dd2e66cada24ce4597e98d35`, tag/commit2026-05-17 | [client.go](https://github.com/SagerNet/sing-shadowtls/blob/af3279d7446641d5dd2e66cada24ce4597e98d35/client.go), [v3_conn.go](https://github.com/SagerNet/sing-shadowtls/blob/af3279d7446641d5dd2e66cada24ce4597e98d35/v3_conn.go), [v3_client.go](https://github.com/SagerNet/sing-shadowtls/blob/af3279d7446641d5dd2e66cada24ce4597e98d35/v3_client.go): три raw GitHub API captures совпали с actual cache SHA-256. v3_constrat.go, service.go/v3_server.go сохранены из cache |
| sing-box v1.13.19, `b5ebaa1fc0f2b94256180b95468e73ef53caa27d` | [ShadowTLS outbound](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/protocol/shadowtls/outbound.go), [VLESS outbound](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/protocol/vless/outbound.go), common/dialer/detour.go, protocol/shadowsocks/outbound.go. Сохранены cache copies, не новый full core audit |
| sing-vmess `3aed155119a174c9290a4099841049f0cb275e02` | [vless/client.go](https://github.com/SagerNet/sing-vmess/blob/3aed155119a174c9290a4099841049f0cb275e02/vless/client.go), [vless/protocol.go](https://github.com/SagerNet/sing-vmess/blob/3aed155119a174c9290a4099841049f0cb275e02/vless/protocol.go): no-flow wrapper и request encoding |
| sing-shadowsocks2 v0.2.1 | Existing dependency/cipher registry прочитаны из module cache; reuse candidate, не полный crypto audit. SS2022 support не native/live доказательство |

Exploratory old ShadowTLS pseudo-version fcd445d3… исключена из evidence о
выбранной версии. `shadowtls-pin.json` явно различает actual и exploratory.
Go module/replacements/dependency resolution не исполнялись; selection установлен
по manifests, actual cache и public tag metadata. Отдельный upstream raw fetch
protocol-v3-en.md timed out; такого capture в index не обещается.

## Протокол и reuse

- [ShadowTLS v3 design, immutable wiki revision e3075638…, 2023-02-12](https://github.com/ihciah/shadow-tls/wiki/V3-Protocol/e30756384998f064e5b51e0ffd016286622777b2):
  encryption/forward security явно вне целей; independently supports traced code.
- [Оригинальный авторский README](https://github.com/ihciah/shadow-tls): требует
  encrypted proxy при использовании; mutable context, не implementation pin.
- [SIP022 AEAD-2022](https://shadowsocks.org/doc/sip022.html): AEAD confidentiality,
  PSK и отсутствие forward secrecy. Counterclaim против автоматического наследования
  полной PFS от donor handshake. Это protocol candidate, не обещание RF обхода.
- [Официальная структура ShadowTLS outbound](https://sing-box.sagernet.org/configuration/outbound/shadowtls/):
  TLS field обязателен, но структура не устанавливает data secrecy; текущая docs
  не подменяет exact source.

Go component license texts — GPL-3.0-or-later. Reuse остаётся внутри уже имеющегося
core при совместимых платформах/сервере. Dependency installation/import/copy в product
не выполнялись. Полные hashes и manual drift review — INDEX.json и REVIEW.json.
