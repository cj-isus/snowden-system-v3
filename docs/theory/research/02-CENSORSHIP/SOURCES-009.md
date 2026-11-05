# R02-009 — source/caller и качество evidence

26 октября 2026. `.artifacts/research/theory-02/2026-10-26-r02-009/`.

| Pin | Relevant source |
|---|---|
| sing-shadowtls0.2.1 `af3279d7446641d5dd2e66cada24ce4597e98d35` | [service.go](https://github.com/SagerNet/sing-shadowtls/blob/af3279d7446641d5dd2e66cada24ce4597e98d35/service.go) C/S HMAC, relay completion and handler; v3_server.go first-match decode; v3_client.go old model reference. Новый HRR finding не заявлен |
| sing0.8.13/module hash из 006 | common/task/task.go Group.Run; common/exceptions/error.go IsClosedOrCanceled/IsClosed; common/bufio/cache.go NewCachedConn/Read |
| sing-vmess `3aed155119a174c9290a4099841049f0cb275e02` | [vless/service.go](https://github.com/SagerNet/sing-vmess/blob/3aed155119a174c9290a4099841049f0cb275e02/vless/service.go) ReadRequest→UUID/flow→handler, TCP/no-flow scope |
| sing-shadowsocks0.2.8 `e0612494bafdd1429e9632bc52fd278585d28690` | [shadowaead_2022/service.go](https://github.com/SagerNet/sing-shadowsocks/blob/e0612494bafdd1429e9632bc52fd278585d28690/shadowaead_2022/service.go) NewService replay.NewSimple60s and validation sequence before handler |
| sing-box1.13.19 `b5ebaa1fc0f2b94256180b95468e73ef53caa27d` | protocol/vless/inbound.go TLS handshake before service; outbound.go tlsDialer before DialEarlyConn; common/tls/client.go handshake caller |

Source provenance: actual module-cache copies captured, SHA-256 in INDEX. Public
proxy module zip retrieval timed out; browser exact-source read failed. Поэтому
не заявляется новая remote equality; old004/005 raw matches остаются историческими.
Значения пользовательских config/PSK/UUID/logs не читались. source branch ≠ active binary.

Primary standards: [RFC8446](https://www.rfc-editor.org/rfc/rfc8446.html), August2018,
§§2.2–2.3/8: fresh handshake/resumption/early data distinction. RFC9266 fetch дал 429,
alternate primary fetch network failure; exporter conclusions в009 не делаются.
Новых numerical field результатов/современных RF claims нет.

Семь локальных source/author/review inputs — LOCAL.json. Independent lead decision
R02-10 прочитан как baseline, не как принятие R02-15/18. GPL-3.0-or-later notices
предыдущих pinned packages сохраняются; нет нового product reuse/import.
Manual source/opposition only; execution cases **NOT_RUN**.
