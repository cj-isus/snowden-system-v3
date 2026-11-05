# R02-011 — early-data API в actual HY2 caller и граница ShadowTLS conversion

26 октября 2026. Чтения — local module cache exact go.mod pins и локальный
Go source reference; crypto/TLS/network execution не выполнялись.
Статусы R02-009/010 и решения по ним этим срезом не меняются.

**Краткий вывод.** Имя `DialEarly` не доказывает передачу ранних данных.
В прочитанной композиции отсутствие `ClientSessionCache` закрывает найденный
путь возобновления; обратный вывод «добавлен кеш ⇒ включён 0-RTT» неверен.
Для ShadowTLS потеря полей в fallback-адаптере реальна, но рассмотренный
вызывающий путь v3 + uTLS её не использует.

**Применимость и предел.** Это датированный разбор исходников, а не захват
трафика установленного клиента. Go1.26.7 служит справочным исходником,
не доказательством версии сборки. Ни глобальный replay-store, ни автоматическое
изменение действующих профилей из результата не следуют.

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
   только если выбран подходящий билет; дополнительно проверяются его
   `EarlyData` и сохранённые транспортные параметры `Extra`.
5. stdlib gate: Go1.26.7 `crypto/tls/handshake_client.go:358` —
   `if ... ClientSessionCache == nil { return }`: без кэша loadSession не
   выполняется, резюмирование и 0-RTT невозможны. sing-box `std_client.go`
   не задаёт ClientSessionCache (negative grep по common/tls и
   protocol/hysteria2); Snowden render не содержит опций кэша.

**Вывод (bounded negative result):** в рассмотренной композиции (core1.13.19 +
sing-quic0.6.4 + quic0.59.0-mod.4 + Snowden render, при применимости прочитанного
Go1.26.7 reference) найденный путь не предоставляет HY2-auth передачу как
TLS 0-RTT. Это ограниченный отрицательный вывод об этой композиции; он
не устанавливает фактический wire trace всех сборок. Наличие `DialEarly`
в вызывающем коде — не свидетельство 0-RTT.

Сильнейшая альтернатива отвергнута по source: «DialEarly значит 0-RTT» ложно —
gate на session cache стоит в stdlib, ниже transport-обёрток.

## 2. Что остаётся из replay-поверхности

- **Повтор QUIC Initial и усиление трафика** (RFC9000 §8.1) — отдельная
  серверная поверхность. До проверки адреса ограничение 3× относится
  к отправленным байтам относительно полученных, а не к общему CPU, памяти
  или числу незавершённых попыток. Повтор ранее наблюдённых Salamander-байтов
  не требует создания новой правильной обфускации; это ещё не завершённое
  рукопожатие и не повтор прикладного действия.
- **Будущее возобновление и 0-RTT нужно различать.** Ненулевой
  `ClientSessionCache` позволяет искать билеты, но не доказывает, что пригодный
  билет найден, `EarlyData` разрешён обеими сторонами и запрос реально
  отправлен до окончания рукопожатия. Эти дополнительные условия видны
  в уже прочитанной ветви `QUICResumeSession`.
- **Область защиты от повторов задаётся выбранной семантикой.** Если будущая
  композиция действительно допускает ранние данные, отдельно определить,
  какие запросы допустимы, кто и в какой области отвергает повтор и что
  происходит при отказе или перезапуске. Ссылки на RFC8446 §8/Appendix E.5,
  Cloudflare/Trail of Bits и tlsplayback описывают риск; они не доказывают,
  что Snowden обязан выбрать один общий межсерверный кеш. Отказ от 0-RTT
  также не требует такого кеша. Прежняя рекомендация общей БД была сильнее
  обоснования. Другие replay/admission поверхности от отсутствия 0-RTT
  не исчезают.

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

- Сохранить найденные предпосылки как датированное ограничение совместимости:
  отсутствие кеша в рассмотренном HY2 и выбранная ветвь uTLS в ShadowTLS.
  Это не указание навсегда запретить кеширование или исправить чужой рендер.
- Перед ускорением возобновления отдельно выбрать политику 0-RTT и допустимость
  повтора auth/прикладного запроса. Цена зависит от этого выбора: хранение
  билетов, время жизни и изоляция контекстов, возможное состояние защиты
  от повторов и допуск серверных попыток. Экономия одного RTT не измерена
  и не обещается для простого добавления `ClientSessionCache`.
- Для STD-адаптера ShadowTLS нельзя предполагать перенос `VerifyConnection`
  или других потерянных полей. Применимость зависит от фактического
  `WithSessionIDGenerator`, проверяющего и полного вызывающего пути.
- Основной разбор — [композиция TLS](../../transports/COMPOSITION-AND-TLS-CONTRACTS.md).
  Переиспользование относится к PC-02/03/06 [плана ПК](../../../planning/knowledge-application/PC-PLAN.md)
  и ANDR-00/02/03 [плана Android](../../../planning/knowledge-application/ANDROID-PLAN.md).
  Собственные артефакты, состав лицензий и поддержанная пара клиента/сервера
  проверяются владельцами; этот срез не разрешает новую зависимость или БД.

## 5. Конечная будущая приёмка — NOT_RUN

1. Native QUIC capture: подтвердить отсутствие 0-RTT пакетов на реальном
   HY2-коннекте Snowden (wire-level).
2. Для отдельно выбранного будущего сценария различить: кеш без пригодного
   билета; возобновление без 0-RTT; разрешённые и отклонённые ранние данные.
   Дубликаты на двух инстансах — условный случай при общей области билетов
   и допуске 0-RTT, не заранее установленный результат.
3. Salamander-on/off Initial replay против сервера: замер amplification-budget
   потребления (server-side admission cost).
4. STD-ветка ShadowTLS v3 с VerifyConnection callback: runtime-подтверждение
   тихой потери (при будущем переиспользовании).
Все исполнения в plan-only окне NOT_RUN; неизвестные (фактическая частота replay
Initial в поле, доля доноров с TLS<1.3) не подставлены нулём.
