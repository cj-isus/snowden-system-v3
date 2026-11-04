# R02-008 — ClientHello boundary и цена anti-probing fallback

26 октября2026 · **READY_FOR_REVIEW**.
Последний bounded вопрос текущего окна. [Источники](SOURCES-008.md).
Source/manual analysis; никакого выполнения TLS/fixtures/сканирования.

## R02-17 — прозрачность для обычного клиента зависит от record boundary

ShadowTLS0.2.1/af3279d7… server NewConnection сначала extractFrame читает **один
TLS record**, затем extractServerName передаёт только эти bytes в Go TLS server
через bytes.Reader/ReadOnlyConn. При error возвращается до verifyClientHello
и до donor fallback. Этот caller ограничивает parser одним record.

В локально доступном reference source Go1.26.7 readClientHello сначала вызывает
readHandshake; тот получает полный размер handshake message и читает до4+n bytes
через readRecord. Только после полного parse вызывается GetConfigForClient.
Если ClientHello разделён между двумя TLS records, input bytes.Reader содержит
только первый; продолжение отсутствует и callback не получает полный ClientHello.
Это не поломка Go TLS parser: caller передал ему усечённый stream.

Ручной контрпример обещанию одинакового ordinary-client fallback:

| Ordinary TLS input без ShadowTLS credentials | Прочитанный path |
|---|---|
| Полный корректный ClientHello в одном record | SNI parsed; auth fails; donor relay возможен |
| Тот же handshake message в двух допустимых records, первый неполный | Первый record прочитан; local parser требует продолжение; error до donor relay |

TLS1.3 разрешает такую fragmentation при соблюдении record/key-change правил.
[RFC8446 §5.1](https://www.rfc-editor.org/rfc/rfc8446.html#section-5.1).
**Это не TCP segmentation**: io.ReadFull собирает record через несколько TCP reads.
Изменяется именно TLS record representation, а не место границы TCP packet.

### Ограничения и альтернативные объяснения

- Нужен donor/control endpoint, который принимает выбранное корректное разбиение.
  Firewall, TLS terminator и обычный сайт тоже могут его отвергать. Поэтому такая
  разница — candidate probe surface, не уникальный detector и не измеренная accuracy.
- Конкретный wire FIN/RST/alert и тайминги не измерены; source устанавливает
  отсутствие donor relay в этой ветке, а не точный пакет закрытия.
- Go1.26.7 взят из доступных исходников установки (VERSION сохранён), **не из
  аттестованного production binary**. Его parser reference поддерживает вывод;
  фактический build/toolchain/core/server остаются отдельным gate.
- PSK не нужен обычному неавторизованному клиенту для проверки fallback. Перенос
  вывода на успешный authorized fragmented transcript требует отдельно проверить
  MAC canonicalization; такой compatibility PASS не заявлен.

## Цена защищённого fallback

Первая allocation ограничена16-bit record length, но per-record bound не задаёт
общий бюджет множества соединений. При Q параллельных inputs bytes порядка Q·L
сохраняются до parse. verifyClientHello при отсутствии совпадения перебирает U users
и хеширует handshake; работа масштабируется с Q/U/L. Это размерностная модель,
не CPU benchmark, не заявленная DoS эксплуатация и не проверка всей инфраструктуры.

После неуспешной auth допустимый ordinary handshake открывает donor connection;
это внешние sockets/трафик/время до доказательства клиента. Wildcard/custom SNI
меняет выбранного donor; текущий deployed режим не читался. Внешние LB/firewall/
rate limits могут ограничивать расходы — их отсутствие не предполагается.

**Adapt:** переиспользовать зрелый TLS parser, обеспечив bounded stream/message
reassembly и сохранение исходного wire prefix для fallback; auth transcript должен
быть однозначен и совместим с клиентом. Existing CachedConn — seam для prefix,
не готовое доказательство правильной reassembly. Не писать конкурирующий TLS parser.

Нужны три явные области budgets: одно сообщение, одно соединение и общий процесс/
donor pool, включая deadlines/FD/CPU/bytes. Прямолинейный отказ только «странным»
inputs сам может оставить fingerprint; полная прозрачность и неограниченный relay
тоже не бесплатны. Матched comparison должен учитывать ответы donor под теми же
лимитами. Server-side donor fallback не равен клиентскому direct fallback VPN.

**Reuse:** Go parser/существующий prefix relay — adapt/reference; single-record
assumption как доказательство indistinguishability — reject claim. Go parser BSD-3-Clause,
ShadowTLS/core/sing implementation GPL-3.0-or-later; notices в captures. Новые
dependencies/product patches не вводились.

## Применимость и finite приёмка — NOT_RUN

Windows: это server/transport compatibility при future encrypted replacement,
не текущий online health. Android ShadowTLS graph пока не принимает; общий core
pin не создаёт server/native поддержку. Передача R05/R06 — resource/failure scope,
R03 — не превращать observed close в подтверждённое DPI. Общие планы не менялись.

После отдельного разрешения: (1) один/two/many TLS records при том же полном CH;
(2) разные TCP segmentation при неизменных records; (3) coalescing и early EOF;
(4) donor controls с теми же SNI/options/budgets; (5) authorized MAC transcript;
(6) finite concurrency/slow input/bad-auth и fallback FD/bytes/deadline limits.
Сейчас source result и границы сохранены; native/live/field superiority не заявлены.
