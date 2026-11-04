# R02-008 — source и контекст

26 октября2026. `.artifacts/research/theory-02/2026-10-26-r02-008/`.

- sing-shadowtls0.2.1 `af3279d7446641d5dd2e66cada24ce4597e98d35`:
  [v3_server.go](https://github.com/SagerNet/sing-shadowtls/blob/af3279d7446641d5dd2e66cada24ce4597e98d35/v3_server.go)
  extractFrame/extractServerName/verifyClientHello; [service.go](https://github.com/SagerNet/sing-shadowtls/blob/af3279d7446641d5dd2e66cada24ce4597e98d35/service.go)
  case3 parse-error before fallback, failed-auth donor relay. Cache captures;
  raw service equality не установлена, как отмечено007.
- sing0.8.13 `common/bufio/only.go` ReadOnlyConn оборачивает переданный Reader;
  вторые records из настоящего socket туда не добавляются автоматически.
- sing-box1.13.19 `b5ebaa1fc0f2b94256180b95468e73ef53caa27d`:
  protocol/shadowtls/inbound.go caller/error path. Точный wire close не исследован.
- Локальный **Go1.26.7** source, VERSION time2026-08-18:
  `src/crypto/tls/conn.go` readHandshakeBytes/readHandshake;
  `handshake_server.go` readClientHello/GetConfigForClient. Источники установки
  прочитаны и сохранены без запуска go/toolchain; не production artifact provenance.
  Primary upstream reference paths: [conn.go](https://github.com/golang/go/blob/go1.26.7/src/crypto/tls/conn.go),
  [handshake_server.go](https://github.com/golang/go/blob/go1.26.7/src/crypto/tls/handshake_server.go).
  Remote byte equality для локальной установки не заявлена.
- [RFC8446 §5.1](https://www.rfc-editor.org/rfc/rfc8446.html#section-5.1), August2018:
  fragmented/coalesced handshake records; AppendixC отдельно требует проверки
  multi-record ClientHello. Standards allowance не означает every donor acceptance.

LOCAL/INDEX/REVIEW фиксируют inputs, hashes, ограничения и counterarguments.
Go BSD-3-Clause / Go implementation scopes GPL-3.0-or-later; notices сохранены.
Никаких tests/fixtures/TLS/crypto execution/scans/native/live или proxy/system changes.
