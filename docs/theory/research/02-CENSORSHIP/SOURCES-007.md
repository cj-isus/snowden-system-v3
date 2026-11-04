# R02-007 — HRR, policy caller и provenance

26 октября2026. Captures `.artifacts/research/theory-02/2026-10-26-r02-007/`.

| Pin / source | Symbols и предел |
|---|---|
| **sing-shadowtls0.2.1** `af3279d7446641d5dd2e66cada24ce4597e98d35` | [service.go](https://github.com/SagerNet/sing-shadowtls/blob/af3279d7446641d5dd2e66cada24ce4597e98d35/service.go): first server frame→extractServerRandom→C/S HMAC, copyByFrameUntilHMACMatches→handler; [v3_client.go](https://github.com/SagerNet/sing-shadowtls/blob/af3279d7446641d5dd2e66cada24ce4597e98d35/v3_client.go): one-shot readHMAC seed including HRR; v3_server.go extract/random/versions/HMAC switch; client.go strict flag |
| **sing-box1.13.19** `b5ebaa1fc0f2b94256180b95468e73ef53caa27d` | [ShadowTLS outbound](https://github.com/SagerNet/sing-box/blob/b5ebaa1fc0f2b94256180b95468e73ef53caa27d/protocol/shadowtls/outbound.go), inbound.go, option/shadowtls.go: separate server strict and absent client option; common/tls/utls_client.go sets config versions/ALPN |
| **MetaCubeX/uTLS1.8.4** `cf49b0864331e156689feec6363ef9bf518a5ac7` | [u_conn.go](https://github.com/MetaCubeX/utls/blob/cf49b0864331e156689feec6363ef9bf518a5ac7/u_conn.go): ApplyPreset/SetTLSVers/ApplyConfig; [u_parrots.go](https://github.com/MetaCubeX/utls/blob/cf49b0864331e156689feec6363ef9bf518a5ac7/u_parrots.go): Chrome133 supported versions/key shares; u_common.go Auto alias; handshake_client_tls13.go actual HRR handling; common.go constant; u_tls_extensions.go supported_versions assignment |
| **sing0.8.13**, module hash from006 | common/task/task.go Group.Run: completion/error aggregation. Не предполагается, что default Group отменяет successful switch как ошибку |

HRR source изучен на фактической pinned dependency, а не старой pseudo-version.
Current v3_client.go повторно совпал с actual raw004. Service raw request в этом
срезе timed out; cache byte equality к upstream для него **не заявлена**. PINS/INDEX
хранят hashes и эту границу. Package module h1 — lock provenance, не выполненный
go mod verify. Никакой TLS/crypto execution или unit fixture не создавались.

Primary references:

- [RFC8446 §4.1.3](https://www.rfc-editor.org/rfc/rfc8446.html#section-4.1.3), August2018:
  HRR использует ServerHello layout и фиксированный SHA256-defined Random.
- [RFC8446 §4.1.4](https://www.rfc-editor.org/rfc/rfc8446.html#section-4.1.4):
  HRR/retry protocol branches. Это не proof свежести отдельного ShadowTLS HMAC.
- [Авторский ShadowTLS v3 design](https://github.com/ihciah/shadow-tls/blob/master/docs/protocol-v3-en.md),
  metadata updated2023-02-11: strict TLS1.3/non-strict TLS1.2 и разделение stages.
  Mutable reference, не implementation pin и не исследование современных сетей РФ.

Ключевые условия/контраргументы: наблюдённый legitimate HRR transcript, одинаковый
PSK, delivery/replay gate scope, no forced-HRR/live assumption, outer handler≠inner
acceptance, SS/TLS independent protections, no HTTPS decryption. Для min-version
trace — Chrome133/non-ECH branch; не обобщать на ECH/STD/все presets.

Лицензии scoped: Go core/ShadowTLS GPL-3.0-or-later, uTLS BSD-3-Clause. Public code
captured как исследовательский material, не аттестация product binary.
