# R02-011 — early-data caller sources

26 октября2026. `.artifacts/research/theory-02/2026-10-26-r02-011/`
(local-inputs-sha256.txt — хэши всех процитированных локальных файлов).

| Source pin | Symbols и boundary |
|---|---|
| sing-quic v0.6.4-0.20260803041914-d83826c306d7 (go.mod, module cache) | quic.go:65-85 CreateTransport STD-ветка, Dial:77 quic.DialEarly; hysteria2/client.go:73-85 quic.Config без early-data, :240-243 auth POST RoundTrip, :187-292 offerNew |
| quic-go v0.59.0-sing-box-mod.4 (go.mod, module cache) | client.go:65 DialEarly «0-RTT» doc; internal/handshake/crypto_setup.go:270-298 QUICStoreSession/QUICResumeSession — ticket-зависимый путь ранних данных |
| core sing-box v1.13.19 `b5ebaa1f…` (go.mod, module cache) | protocol/hysteria2/outbound.go:47 tls.NewClient; protocol/shadowtls/outbound.go:57-69 v3-ветвление WithSessionIDGenerator vs DefaultTLSHandshakeFunc; common/tls/utls_client.go:67 SetSessionIDGenerator; common/tls/std_client.go — ClientSessionCache отсутствует (negative grep) |
| sing-shadowtls v0.2.1 (go.mod, module cache; upstream commit af3279d7… из010) | tls_wrapper.go:23-46 DefaultTLSHandshakeFunc — точный список скопированных полей; VerifyConnection/ClientSessionCache/GetClientCertificate/ECH/KeyLogWriter не переносятся |
| Local Go1.26.7 source reference | crypto/tls/handshake_client.go:358 `ClientSessionCache == nil → return` — stdlib gate резюмирования. Reference, не attestation production toolchain |
| Snowden source | windows/backend/render/render.go:775-817 (HY2 tls без utls/кэша), :712-724 (shadowtls utls chrome); Taskfile.yml:6 PRODUCTION_TAGS with_utls |

Primary protocol/практика (web, без raw text equality):

- [RFC 8446](https://www.rfc-editor.org/rfc/rfc8446.html) §8, Appendix E.5 —
  0-RTT replay: single-use server anti-replay, ограничение масштабирования,
  прикладная идемпотентность.
- [RFC 9000](https://www.rfc-editor.org/rfc/rfc9000.html) §8.1 — QUIC address
  validation/anti-amplification limit 3× для Initial-replay поверхности.
- [Cloudflare: Introducing Zero Round Trip Time Resumption (0-RTT)](https://blog.cloudflare.com/introducing-0-rtt/)
  — ранние данные уязвимы к replay by design.
- [Trail of Bits: What Application Developers Need To Know About TLS Early Data](https://blog.trailofbits.com/2019/03/25/what-application-developers-need-to-know-about-tls-early-data-0rtt/)
  — прикладные контрмеры, одноразовые токены.
- [Finkel/Foltz… Replay Attacks on Zero Round Trip Time (IACR eprint 2017/082)](https://eprint.iacr.org/2017/082.pdf)
  — формализация replay-уязвимости early data.
- [Cisco CXSecurity tlsplayback PoC](https://github.com/CiscoCXSecurity/tlsplayback)
  — воспроизводимые replay-атаки против TLS1.3 библиотек.
- Qrator Labs (Habr): [TLS 1.3 enabled, and why you should do the same](https://habr.com/en/companies/qrator/articles/449610/)
  — русскоязычная команда Qrator о развёртывании TLS1.3; сам Ru-текст про 0-RTT
  replay у Qrator отдельной статьёй не найден — прямой русскоязычный primary по
  0-RTT replay отсутствует, used as adjacent reference.

Negative search: ClientSessionCache в windows/backend и common/tls sing-box —
не найден; consumer exporter/кэша в Snowden backend/corebridge — не найден
(наследует009/010). Scoped search, не deployed/runtime proof. Никакого
crypto/TLS/QUIC исполнения; remote byte equality не заявлена. Go/uTLS BSD-3-Clause,
core/sing/ST GPL-3.0-or-later. Все будущие execution cases **NOT_RUN**.
