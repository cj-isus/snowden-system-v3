# R28-Z01 — источники и provenance

Чтение: 28 октября 2026. Статусы FULL_TEXT_READ/SECTIONS_READ/SOURCE_READ разделяются. Public source fetch — не VPN probe; tests/build/crypto/native/live NOT_RUN.

## 1. Нормативные первоисточники

### S1 — RFC 9848

B. Schwartz, M. Bishop, E. Nygren. *Bootstrapping TLS Encrypted ClientHello
with DNS Service Bindings*. RFC 9848, March 2026, IETF Standards Track,
Proposed Standard. <https://www.rfc-editor.org/rfc/rfc9848.txt>.

**FULL_TEXT_READ ранее baseline; для R28 повторно SECTIONS_READ:** §§3–6, §8.
Получен заново 08:59Z, capture `rfc9848.txt`.
Использование: `ech` принадлежит alternative endpoint; все адреса TargetName
должны иметь key либо authority P; ClientHello ждёт SVCB resolution;
all-ech SVCB-reliant; mixed-list downgrade; Alt-Svc consistency.

### S2 — RFC 9849

E. Rescorla, K. Oku, N. Sullivan, C. A. Wood. *TLS Encrypted Client Hello*.
RFC 9849, March 2026, IETF Standards Track, Proposed Standard.
<https://www.rfc-editor.org/rfc/rfc9849.txt>.

**SECTIONS_READ:** §§4.1–5.2, 6.1.4–6.1.8, 6.2, 8.1, 10.1–10.4.
Получен заново 08:59Z, capture `rfc9849.txt`.
Использование: O/P identities, authenticated rejection, retry-address warning,
source-bound persisted state, network-change mitigation and tracking.

### S3 — RFC 9460

B. Schwartz, M. Bishop, E. Nygren. *Service Binding and Parameter Specification
via the DNS (SVCB and HTTPS Resource Records)*. RFC 9460, November 2023,
IETF Standards Track, Proposed Standard.
<https://www.rfc-editor.org/rfc/rfc9460.txt>.

**SECTIONS_READ:** §§2.3–3.2, 5.1–5.2, 7.1–8, 9.1–9.4, 12–13.
Получен заново 08:59Z, capture `rfc9460.txt`.
Использование: RR atomicity/priority/mandatory; preconnect consistency; address
hints vs A/AAAA; TargetName/port/ALPN; proxy behavior; cache/network partition.

## 2. Actual manifest/source pins

| Input | Pin / area | Read depth and exact purpose |
|---|---|---|
| `windows/go.mod`, `go.sum` | Go1.26; sing-box1.13.19; uTLS1.8.4; sing0.8.13; miekg/dns1.1.72 | SOURCE_READ version selection; not compiled artifact provenance |
| Snowden `corebridge.go` | renderAndPrepareContext:106–184 | SOURCE_READ preflight→override ownership |
| Snowden `backend/core/preflight.go` | Resolve:79–149, A/JSON DoH:151–189, cache:231–267 | SOURCE_READ first-success IP and 7-day host cache |
| Snowden `backend/render/render.go` | DNS:565–587, VLESS/WS:732–774, tunnelHostnames:1012–1021 | SOURCE_READ separate bootstrap DNS and current no-ECH renderer |
| Snowden `backend/core/engine.go` / worker | newSingBox:404–414; worker NewEngine route | SOURCE_READ actual embedded/worker box construction; no runtime |
| sing-box `box.go` | managers and dns.Router registration | SOURCE_READ concrete DNSRouter implementation |
| sing-box `common/tls/ech.go` | parse:27–55; ECHClientConfig/fetch/cache/Clone:109–189 | SOURCE_READ config state and refresh gap |
| sing-box `common/tls/client.go` | New client:52–69; dial/retry:117–134 | SOURCE_READ dial-before-handshake and stdlib typed retry |
| sing-box STD/uTLS clients | ConfigList setters, Client/Clone, NewClient ECH branch | SOURCE_READ state projection and cloning |
| sing-box VLESS/WS | VLESS NewOutbound/Dial:46–99,151–170; WS client dial:36–108; early conn:136–238 | SOURCE_READ app data only after TLS/upgrade, object lifecycle |
| sing-box DNS router/client/transports | Router Exchange/Reset; Client Exchange/cache/Lookup; DoH/DoT/TCP transport | SOURCE_READ Msg vs address API and reset behavior |
| sing-box network manager | ResetNetwork:457–484, notifications | SOURCE_READ DNS/connection reset but no TLS ECH reset |
| sing-box dialer resolve/default | Domain→A/AAAA→address, socket dial | SOURCE_READ endpoint selection separately from HTTPS RR |
| uTLS `u_handshake_client.go` | O initial state, E parsing, outer P assignment, TLS handshake | SOURCE_READ real ECH identity transitions |
| uTLS `handshake_client*.go` | acceptance, rejection, certificate/Finished guards | SOURCE_READ authenticated/error path |
| uTLS `u_tls_extensions.go` | SNI guard :201–208 | SOURCE_READ config O preserved under real ECH |
| uTLS ECH/parser/connection | public_name config, native ECHAccepted, error type | SOURCE_READ no product execution |
| miekg/dns `svcb.go` | HTTPS/SVCB structs, mandatory/alpn/port/hints/ech | SOURCE_READ ready parser primitives; semantic validation remains caller responsibility |
| local Go1.26.7 crypto/tls/errors | stdlib ECH contract/type and errors.As semantics | SOURCE_READ reference; whole EXE compiler attestation NOT_RUN |

Initial source capture: `.artifacts/research/r28-contracts/Z01/source-initial.json`,
37 files at about 08:57Z. Extra captures: `source-extra.json` (19 files),
`source-composition.json` (3 files), and selected paired/API captures. Serial
copies, not atomic repository snapshots.

## 3. Independent byte equality

Immutable raw URLs read 09:18Z:

- MetaCubeX/uTLS commit `cf49b0864331e156689feec6363ef9bf518a5ac7`:
  `u_tls_extensions.go`, `handshake_client.go`.
- SagerNet/sing-box commit `b5ebaa1fc0f2b94256180b95468e73ef53caa27d`:
  `common/tls/ech.go`, `route/network.go`.

Remote/local module cache SHA-256 equality: **4/4**, exact records in
`remote-equality.json`. This proves named bytes only, not full module/artifact
attestation or dependency integrity.

## 4. Reuse/licences

- sing-box root LICENSE: **GPL-3.0-or-later** plus name/association restriction;
  SOURCE_READ. Whole-product compliance not determined.
- MetaCubeX/uTLS root LICENSE: **BSD-3-Clause**; SOURCE_READ.
- miekg/dns root LICENSE: **BSD-3-Clause**; SOURCE_READ.
- Go standard library LICENSE: BSD-style; captured; complete distribution
  obligations outside this research scope.
- RFCs: IETF Trust legal notices; specifications, not drop-in software.

Recommendation is reuse/adapt existing parser/TLS/exchange primitives; no own
HPKE/ECH/DNSSEC implementation.

## 5. Paired independent input

C02/HANDOFF became available during review and was read once, not polled.
Captured in `source-review-extra.json` with sing-box `ech_shared.go`.
Relevant strongest claims:

- R28-C02-B: DNS/CloseAll/mux reset does not imply ECH wrapper reset;
  stale config and handover need separate admission.
- R28-C02-C: uTLS default rejection verifier may verify O rather than P.
- R28-C02-A: fixed endpoint/shared key is strong positive control; missing
  generic SVCB semantics is not blanket unsafe.

Z01 independently traced source and agrees within named scope. Model agreement
is not independent field evidence or scientific acceptance.

## 6. Read/fetch status

- RFC 9848/9849/9460: **SECTIONS_READ**, official RFC Editor TXT, successful.
- GitHub immutable raw critical source: **SOURCE_READ**, successful 4/4.
- Module cache source: **SOURCE_READ** selected symbols; full repository not audited.
- No external papers/field reports required by this narrow contract package.
- No failed source fetch relevant to final argument.
- No private endpoints, profiles, logs, vault, credentials, DNS or TLS probes.

## 7. Evidence boundaries and future NOT_RUN

No tests/fixtures/build/vet/doctor/simulation/benchmarks/crypto execution/security
scans/native/device/live/admin/install/VPS/SSH/deploy/OS network. Handwritten
counterexamples are models, not reproduced failures. Current Windows renderer
does not include ECH; deployed severity/frequency unknown.

Future cases in REPORT §8 are all NOT_RUN. Platform consequence requires owner
review; this author does not modify product source/plans/dependencies/roles.
