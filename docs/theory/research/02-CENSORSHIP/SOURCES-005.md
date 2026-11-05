# R02-005 — sources, callers, pins

26 октября 2026; текущая дата получения, не дата всех источников.
Capture namespace `.artifacts/research/theory-02/2026-10-26-r02-005/`.

## Actual sources

| Компонент / exact pin | Прочитанные callers/symbols |
|---|---|
| sing-box1.13.19 `b5ebaa1fc0f2b94256180b95468e73ef53caa27d` | [SS outbound](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/protocol/shadowsocks/outbound.go): NewOutbound→CreateMethod→DialEarlyConn, UoT/mux; [SS inbound](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/protocol/shadowsocks/inbound.go): NewServiceWithPassword from different server library |
| тот же core | [VLESS outbound](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/protocol/vless/outbound.go): tlsDialer→DialEarlyConn; protocol/vless/inbound.go TLS server; protocol/shadowtls/outbound.go ignores supplied destination for its own dial; inbound/option shadowtls strict/server composition |
| тот же core | [TLS client](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/common/tls/client.go): handshake before return; std_client.go/utls_client.go reference name, certificates, ALPN, versions. ECH retry observed, not audited end-to-end |
| **sing-shadowsocks2 v0.2.1** `1f9f20777fd1aedc3aeaebbc08fad00a2e2f8c40` | [method.go](https://github.com/SagerNet/sing-shadowsocks2/blob/1f9f20777fd1aedc3aeaebbc08fad00a2e2f8c40/shadowaead_2022/method.go): exact key length, writeRequest/readResponse, request-salt check; protocol.go KDF; internal/shadowio writer/reader AEAD; cipher/method_registry.go and [method_none.go](https://github.com/SagerNet/sing-shadowsocks2/blob/1f9f20777fd1aedc3aeaebbc08fad00a2e2f8c40/cipher/method_none.go) |
| **server sing-shadowsocks v0.2.8** `e0612494bafdd1429e9632bc52fd278585d28690` | [service.go](https://github.com/SagerNet/sing-shadowsocks/blob/e0612494bafdd1429e9632bc52fd278585d28690/shadowaead_2022/service.go): NewService local replay filter, newConnection/writeResponse, long-key handling; service_multi.go scoped constructor/key handling; protocol.go matching KDF. Полный multi-user audit не сделан |
| sing v0.8.13, actual manifest | `common/replay/simple.go`: in-memory map/mutex per instance, timeout60s from caller. Scope — process object; serverless/global persistence не существует в прочитанной реализации |
| ShadowTLS v0.2.1 `af3279d7446641d5dd2e66cada24ce4597e98d35` | Сохранённый 004 exact raw/cache data writer, actual core caller и option. В005 не повторяется finding004 как новый |

Client method и server service raw GitHub contents при exact tags совпали с cache
SHA-256. `PINS.json`, `INDEX.json`, `LOCAL.json`, `REVIEW.json` фиксируют область.
Наличие capture не означает exhaustive review всех строк. GitHub tag metadata
проверена текущим чтением; module/build/tools execution не выполнялось.

Локальные inputs: windows/go.mod, backend/render/render.go, config/config.go;
android/native/source-lock.json и NativeGraphSecurity.kt; тематические transports/
SAFETY-AND-TRUST; собственный 004. Секреты, реальные profiles/runtime/logs не читались.
Тексты upstream licenses GPL-3.0-or-later сохранены; notices иных транзитивных
компонентов не заменяются одной лицензией server/client Go packages.

## Primary protocol material

- [SIP022 AEAD-2022](https://shadowsocks.org/doc/sip022.html), §§1–3.1.5:
  encryption, PSK, no forward secrecy, header/response and bounded replay.
  Mutable official specification; scoped source подтверждает конкретные branches.
- [RFC8446](https://www.rfc-editor.org/rfc/rfc8446.html), August2018,
  §§2.3/4.2.9/8: data-stage vs handshake, PSK/0-RTT limitations.
  Не утверждает, что наш VLESS DialEarlyConn включает TLS0-RTT.
- [RFC9525](https://www.rfc-editor.org/rfc/rfc9525.html#section-6), November2023:
  service reference identity. Domain/source authorization профиля остаётся отдельным.
- [Автор ShadowTLS v3](https://github.com/ihciah/shadow-tls/blob/master/docs/protocol-v3-en.md),
  документ updated2023-02-11: strict TLS1.3 и handshake/data stage. Mutable reading,
  pinned implementation conclusions привязаны к Go source, а не master README.

Численные field результаты из этих документов не заимствуются. Manual path coverage,
cross-Service replay witness и цена framing — вывод R02 при явных assumptions.
Future tests/fixtures/build/crypto/simulation/native/live — **NOT_RUN**.
