# R02-010 — exporter/caller sources

26 октября2026. `.artifacts/research/theory-02/2026-10-26-r02-010/`.

| Source pin | Symbols и boundary |
|---|---|
| core1.13.19 `b5ebaa1fc0f2b94256180b95468e73ef53caa27d` | [common/tls/utls_client.go](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/common/tls/utls_client.go) Client/NewUTLSClient/ConnectionState literal; std_client.go Client/NewSTDClient; client.go handshake; protocol/shadowtls/outbound.go and option/tls.go |
| sing0.8.13/module hash из006 | common/tls/config.go Config/Conn/ClientHandshake; ConnectionState alias standard Go TLS. API has no separate exporter capability |
| MetaCubeX/uTLS1.8.4 `cf49b0864331e156689feec6363ef9bf518a5ac7` | [conn.go](https://github.com/MetaCubeX/utls/blob/cf49b0864331e156689feec6363ef9bf518a5ac7/conn.go) native state ekm; common.go ExportKeyingMaterial; u_conn.go UClient/uLoadSession; handshake_client.go cache nil guard |
| ShadowTLS0.2.1 `af3279d7446641d5dd2e66cada24ce4597e98d35` | [tls_wrapper.go](https://github.com/SagerNet/sing-shadowtls/blob/af3279d7446641d5dd2e66cada24ce4597e98d35/tls_wrapper.go) helper copies subset into new uTLS config, return error only |
| Local Go1.26.7 source reference | crypto/tls/common.go private ekm+method; handshake_client.go cache nil guard. VERSION/source/license captured. No toolchain execution, no production provenance/equality claim |

Negative search: windows/backend and android/corebridge/src/main for exporter/cache
consumers, excluding tests/runtime/secrets. No exporter consumer found; early_data
hits are WS fields. This is scoped code search, not complete deployed/runtime proof.

Primary protocol references:

- [RFC9266](https://www.ietf.org/rfc/rfc9266.txt), July2022, §§2/4/4.1–4.2:
  ready channel-binding profile, scope/cardinality and prohibition on key misuse.
  Первоначальный RFCEditor request429 и alternate network failure сменились успешным
  чтением official IETF text/search. Own paraphrased note сохранён, не raw text equality.
- [RFC8446 §7.5](https://www.rfc-editor.org/rfc/rfc8446.html#section-7.5), August2018:
  regular and early exporter separation; §§2.2–2.3/8 for resumption/early data.

No numerical RF measurements/implementation benchmark. Pins/manifests/cache hashes
в LOCAL/INDEX/REVIEW; raw re-acquisition/equality в010 не выполнялась. Source caches
не выдаются за installed/executing binary. Go/uTLS BSD-3-Clause, core/sing/ST GPL-3.0-or-later.
Все будущие execution/crypto/native/live cases **NOT_RUN**.
