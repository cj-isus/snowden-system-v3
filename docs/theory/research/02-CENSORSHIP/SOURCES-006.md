# R02-006 — exact source, negative caller и ограничения capture

26 октября 2026; `.artifacts/research/theory-02/2026-10-26-r02-006/`.

- **sing-shadowtls v0.2.1**, commit `af3279d7446641d5dd2e66cada24ce4597e98d35`:
  [v3_conn.go](https://github.com/SagerNet/sing-shadowtls/blob/af3279d7446641d5dd2e66cada24ce4597e98d35/v3_conn.go)
  write117–134, WriteBuffer136–150, WriteVectorised152–167, verifyApplicationData;
  Write98–115 chunk/error loop. Current cache equality к actual raw004 повторно checked.
  client.go creates HMAC state; constants confirm header5/tag4.
- **sing v0.8.13**, date2026-08-10, module hash
  `h1:yVoXnx9nPxfjlwD4Tp+Wd9zuW2tfiSVrcRDBZNbKRCw=` from current windows/go.sum:
  [copy_direct.go](https://github.com/SagerNet/sing/blob/v0.8.13/common/bufio/copy_direct.go),
  [vectorised.go](https://github.com/SagerNet/sing/blob/v0.8.13/common/bufio/vectorised.go),
  copy.go/copy_session.go/conn.go/wait.go; network/counter.go/direct.go/early.go.
  Full tag SHA API timed out: **не придуман exact commit**; pinned version/cache/
  module manifest используются с этой границей provenance.
- **sing-vmess** `3aed155119a174c9290a4099841049f0cb275e02`:
  [vless/client.go](https://github.com/SagerNet/sing-vmess/blob/3aed155119a174c9290a4099841049f0cb275e02/vless/client.go),
  requestWritten/WriterReplaceable/ExtendedConn/Upstream. No-flow branch only.
- **sing-box1.13.19** `b5ebaa1fc0f2b94256180b95468e73ef53caa27d`:
  protocol/vless/outbound.go and common/mux/client.go: no multiplex on zero options.
  Тот же caller прочитан в005; наличие branch не actual runtime trace.
- **sing-shadowsocks0.2.8** `e0612494bafdd1429e9632bc52fd278585d28690`:
  [shadowaead/aead.go](https://github.com/SagerNet/sing-shadowsocks/blob/e0612494bafdd1429e9632bc52fd278585d28690/shadowaead/aead.go),
  Writer.WriteVectorised gathers encrypted data and calls upstream.Write;
  shadowaead_2022/service.go serverConn caller. Это negative evidence против
  автоматического переноса ShadowTLS vectorized дефекта на SS replacement.
- **sing-shadowsocks2 v0.2.1** `1f9f20777fd1aedc3aeaebbc08fad00a2e2f8c40`:
  internal/shadowio/writer.go: client scalar/WriteBuffer; no automatic pass-through
  upstream.WriteVectorised branch в прочитанном codec.

Official contracts:

- [hash.Hash](https://pkg.go.dev/hash#Hash): Sum не изменяет state; current cached
  Go docs reference, **не доказательство версии установленного toolchain**.
- [io.Writer](https://pkg.go.dev/io#Writer): byte count и nonnil error for short write.
- [RFC8446 §5.1–5.2](https://www.rfc-editor.org/rfc/rfc8446.html#section-5.2), August2018:
  TLS1.3 record size bounds; это протокол camouflage reference, не то же самое,
  что собственный custom ShadowTLS parser acceptance.

Raw Go1.26 hash/io source fetches failed: zero-byte files не usable source.
sing-tag.json содержит unusable null после API failure, а не commit evidence.
INDEX отмечает эти failures, не считает их прочитанными исходниками. Source licenses
implementation scopes GPL-3.0-or-later; notices preserved. Module execution, crypto
calculation, tests, fixtures, native/live и system changes отсутствуют.
