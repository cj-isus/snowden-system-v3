# R28-Z01 — передача руководителю

28 октября 2026.

status: READY_FOR_REVIEW

## 1. Scoped verdict

Предлагаю **ACCEPT R28-Z01-A только как source/model finding**:

> В рассмотренном sing-box v1.13.19 future-ECH caller нет доказанного
> endpoint-bound Service Binding plan. Socket выбирается отдельно, затем
> ECH wrapper берёт первый `ech` из DNS Answer и хранит config/TTL/time без
> TargetName/port/address/source/network epoch. TLS guards предотвращают silent
> unauthenticated application success, но не доказывают общее происхождение
> ECHConfig и endpoint. Network reset очищает DNS/соединения, не wrapper ECH cache.

Не принимать это как deployed vulnerability, current ECH failure, plaintext
application leak, censorship, native regression или product PASS. Current
Windows VLESS/WS renderer real ECH не включает.

## 2. Exact evidence

- Manifest: sing-box1.13.19, uTLS1.8.4, sing0.8.13, miekg/dns1.1.72.
- Actual chain: Snowden preflight IP override → renderer O/Host → sing-box VLESS
  dialer/WS → socket destination → ECH HTTPS query → first ech config → TLS.
- RFC9848/9849/9460: service attempt связывает O, selected RR, T, port, ALPN,
  address set, E, P, source/freshness; preconnect нельзя превратить в
  ClientHello до consistency result.
- Remote/cache source equality: four critical immutable files, 4/4.
- Full trace and locators: [REPORT](REPORT.md), source read-status:
  [SOURCES](SOURCES.md).

## 3. Strongest counterexample that survives guards

G1: wrapper cached E_A with TTL, endpoint pool A. Network handover resets DNS
cache/connections, but leaves E_A. G2 address lookup selects pool B. B still
has old ECH key E_A and valid origin O certificate (rollout overlap/shared key).
ECH acceptance, origin certificate, CertificateVerify and Finished all pass.

Thus cryptography correctly authenticates a usable endpoint, yet no existing
state proves that E_A and B were chosen from the same ServiceMode source or
network epoch. This is provenance/freshness debt, not authentication bypass.
If B lacks key, guards fail/recover instead; that opposing case does not close
source binding.

## 4. Guard crosswalk

| Guard | Stops | Does not prove |
|---|---|---|
| ECH acceptance confirmation | False acceptance of arbitrary inner transcript | DNS source/endpoint provenance |
| Origin O certificate + signature + Finished | Unauthenticated origin/application TLS | Correct ServiceMode RR or network epoch |
| Rejected outer P authentication | Adoption of unauthenticated retry config | P is checked correctly in uTLS current path; endpoint binding |
| Error before WS Upgrade | WS/VLESS data on TLS failure | End-to-end VPN confidentiality after CDN terminator |
| DNS cache reset/connection close | Reuse of stale DNS message/socket | Clearing separate ECH list/TTL state |
| One bounded stdlib retry | Infinite immediate retry loop | uTLS named-error parity or safe new address selection |

## 5. Ready reuse / licence / cost

- Reuse miekg/dns1.1.72 parser structs (BSD-3), adapt semantic selector.
- Reuse sing-box DNS/TLS transport and cache primitives (GPL3-or-later + name
  restriction), but do not call them full SVCB binding.
- Prefer standard/native ECH verifier/error path where compatibility permits;
  defer real uTLS ECH until P verification, errors and observation parity proven
  (uTLS BSD-3).
- Reject custom HPKE/ECH/DNSSEC implementation.

Required cost: immutable attempt plan/source ID/network epoch; bounded alias and
priority selection; cancellation on handover; one shared timeout/retry budget;
STD/uTLS parity; extra DNS/latency/energy/privacy. No numeric costs measured.

## 6. Platform consequence

Windows/Android platform owner should own endpoint selection, config-source
identity, epoch/cancellation and enforcement. Portable policy may consume typed
facts but must not invent binding from hostname, TTL, HandshakeComplete or
ECHAccepted=false. Fixed endpoint ECH is a separate contract: signed/owned E
must explicitly authorize that endpoint/public-name set.

## 7. Open / NOT_RUN

- Executed STD/uTLS ECH with P-only/O-only/multi-SAN certificates.
- Multi-CDN IPv4/IPv6 handover and expired/no-ech response behavior.
- Native cancellation of late DNS/TLS results and product policy ECH-required.
- Actual server key overlap, performance, energy and field filtering.
- Compiled EXE/AAR parity and whole-product licence compliance.

All NOT_RUN. No source/product changes, tests, profiles or live probes.

## 8. Work stages

1. Source/pin/caller — completed.
2. Endpoint/identity/retry attempt trace — completed.
3. Opposing guards and surviving counterexamples — completed.
4. Stale cache/handover continuation — completed.
5. Reuse/licence/cost and handoff — completed.

Задача исчерпана в scoped source/model области; автор останавливается после
финальной статической сверки своих четырёх файлов.
