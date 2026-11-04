# R02-011 — early-data API в actual HY2 caller и граница ShadowTLS conversion

26 октября 2026. Чтения — local module cache exact go.mod pins и локальный
Go source reference; crypto/TLS/network execution не выполнялись.
Статусы R02-009/010 и решения по ним этим срезом не меняются.

## 1. R02-20 — HY2-пароль не экспонирован TLS 0-RTT в рассмотренной композиции

Вопрос STATE: actual HY2/QUIC early-data/authentication/replay caller, не перенося
вывод TCP VLESS на QUIC. Полный traced chain:

1. core1.13.19 `protocol/hysteria2/outbound.go:47` → `tls.NewClient` (STD по
   умолчанию; Snowden рендерит HY2 без `utls` — `render.go:776` только
   server_name+certificate).
2. sing-quic `v0.6.4-0.20260803041914` `hysteria2/client.go:73-85`: `quic.Config`
   без early-data полей; ALPN h3. Аутентификация — **post-handshake** HTTP/3 POST:
   `client.go:240-243` кладёт `AuthRequest{Auth: password, Rx}` в header и делает
   `http3Transport.RoundTrip` c handshakeTimeout.
3. `qtls.CreateTransport` (`quic.go:65-85`): sing-box STDClientConfig не реализует
   `qtls.Config` (нет Dial/DialEarly/CreateTransport методов) → STD-ветка строит
   `http3.Transport`, чей Dial вызывает **`quic.DialEarly`** (`quic.go:77`).
4. Но имя API ≠ включённый ранний data. quic-go `v0.59.0-sing-box-mod.4`
   `internal/handshake/crypto_setup.go:279-298`: событие `QUICResumeSession`
   (выбор session ticket) приходит от stdlib crypto/tls; ранние данные разрешаются
   только если ticket найден и his Extra согласован.
5. stdlib gate: Go1.26.7 `crypto/tls/handshake_client.go:358` —
   `if ... ClientSessionCache == nil { return }`: без кэша loadSession не
   выполняется, резюмирование и 0-RTT невозможны. sing-box `std_client.go`
   не задаёт ClientSessionCache (negative grep по common/tls и
   protocol/hysteria2); Snowden render не содержит опций кэша.

**Вывод (bounded negative result):** в рассмотренной композиции (core1.13.19 +
sing-quic0.6.4 + quic0.59.0-mod.4 + Go1.26.7 + Snowden render) аутентификационный
POST HY2 всегда уходит как обычные 1-RTT данные; replay-поверхность TLS early data
на этом слое не возникает. Наличие `DialEarly` в caller — не свидетельство 0-RTT.

Сильнейшая альтернатива отвергнута по source: «DialEarly значит 0-RTT» ложно —
gate на session cache стоит в stdlib, ниже transport-обёрток.

## 2. Что остаётся из replay-поверхности

- **QUIC Initial replay/amplification** (RFC9000 §8.1): повтор Initial-датаграмм
  создаёт незавершаемые connection attempts; стоимость сервера ограничена
  anti-amplification limit (≤3×). Salamander обфусцирует датаграммы, но replay
  байтов валиден и без знания obfs-пароля — handshake не завершится. Это
  серверная admission-поверхность, не зависит от клиентской композиции.
- **Replay-cache scope/multi-instance** из STATE растворяется: без 0-RTT нет и
  server-side anti-replay обязательств. Вопрос становится **условным**: любой
  будущий consumer, включающий ClientSessionCache для ускорения реконнекта HY2,
  активирует требование single-use server anti-replay и общий кэш между
  инстансами (RFC8446 §8/Appendix E.5; Cloudflare/Trail of Bits практика; PoC
  tlsplayback). Multi-instance без общего кэша получит дубликаты early data —
  классический race из прикладных replay-атак.

## 3. R02-21 — ShadowTLS v3 lossy conversion реальна, но не достигается в composed caller

Кандидат из STATE: «ShadowTLS default TLS config conversion не переносит
VerifyConnection». Проверка по exact caller:

- sing-shadowtls `v0.2.1` `tls_wrapper.go:23-46` (`DefaultTLSHandshakeFunc`)
  действительно копирует только подмножество полей crypto/tls.Config в utls.Config:
  Rand, Time, VerifyPeerCertificate, RootCAs, NextProtos, ServerName,
  InsecureSkipVerify, CipherSuites, Min/MaxVersion, CurvePreferences,
  SessionTicketsDisabled, Renegotiation. **Теряются:** `VerifyConnection`,
  `ClientSessionCache`, `GetClientCertificate`, `Certificates`, ECH-поля,
  KeyLogWriter. Подтверждено как source-факт.
- Но sing-box `protocol/shadowtls/outbound.go:57-69` для **version 3** сначала
  пробует `tlsConfig.(WithSessionIDGenerator)`: UTLSClientConfig реализует
  `SetSessionIDGenerator` (`utls_client.go:67`) → используется полный
  `tls.ClientHandshake` с uTLS-fingerprint. `DefaultTLSHandshakeFunc` — fallback
  только для конфигов **без** session-id-generator (STD).
- Snowden рендерит shadowtls-звено с `utls enabled + fingerprint chrome`
  (`render.go:717-721`), production tags содержат `with_utls` (Taskfile:6) →
  фактический путь — UTLS-ветка. Lossy-конвертация **не достигается**.

**Вывод:** потери VerifyConnection в текущей композиции нет; это скрытая граница
совместимости, а не дефект Snowden. Если будущий consumer выключит uTLS на v3
звене (STD config), любой donor-pinning VerifyConnection callback будет тихо
проигнорирован — handshake откатится к RootCAs/InsecureSkipVerify без ошибки.

## 4. Reuse и цена

- Ничего не менять в текущем рендере HY2/shadowtls; оба свойства держатся на
  **отсутствии** опций (нет session cache, есть utls) — это хрупкая инвариантность:
  зафиксировать в design-note, не в коде.
- Запретительная рекомендация: не включать ClientSessionCache на HY2 outbound без
  серверного anti-replay дизайна (единый replay-store на все инстансы + admission
  budget). Выигрыш — один RTT на реконнект; цена — replay-поверхность на
  аутентификацию и admission.
- Для ShadowTLS v3: конфигурация без utls — недопустимый путь для будущих
  donor-authorization callbacks; проверять реализацию WithSessionIDGenerator.

## 5. Конечная будущая приёмка — NOT_RUN

1. Native QUIC capture: подтвердить отсутствие 0-RTT пакетов на реальном
   HY2-коннекте Snowden (wire-level).
2. Включить экспериментальный ClientSessionCache → повторить capture →
   зафиксировать replay-дубликаты на двух серверных инстансах.
3. Salamander-on/off Initial replay против сервера: замер amplification-budget
   потребления (server-side admission cost).
4. STD-ветка ShadowTLS v3 с VerifyConnection callback: runtime-подтверждение
   тихой потери (при будущем переиспользовании).
Все исполнения в plan-only окне NOT_RUN; неизвестные (фактическая частота replay
Initial в поле, доля доноров с TLS<1.3) не подставлены нулём.
