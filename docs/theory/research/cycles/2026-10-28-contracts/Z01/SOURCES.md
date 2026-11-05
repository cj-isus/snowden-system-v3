# R28-Z01 — источники ECH и границы проверки

Дата исходного чтения — **28 октября 2026 года**. Различаются FULL_TEXT_READ,
SECTIONS_READ и SOURCE_READ. Публичное получение текста не было пробой VPN;
тесты, сборка, crypto/native/live не выполнялись.

## Краткий вывод

Реестр поддерживает разбор связи Service Binding, endpoint, ECH-параметров
и повторов в названных исходниках. Он не удостоверяет установленный артефакт
или работающий ECH-путь Snowden. Полные предпосылки и рекомендации —
в [аргументе](REPORT.md) и [сводке применения](HANDOFF.md).

## 1. Нормативные первоисточники

### S1 — RFC 9848

B. Schwartz, M. Bishop, E. Nygren. *Bootstrapping TLS Encrypted ClientHello
with DNS Service Bindings*. RFC9848, March2026, IETF Standards Track,
Proposed Standard. <https://www.rfc-editor.org/rfc/rfc9848.txt>.

В прежнем основании FULL_TEXT_READ; в R28 повторно **SECTIONS_READ** §§3–6, §8.
Исходное получение 08:59Z, `rfc9848.txt`.
Использованы принадлежность `ech` альтернативному endpoint, ключ либо
полномочия P для адресов TargetName, ожидание SVCB до ClientHello,
all-ech SVCB-reliant, смешанные списки и согласованность Alt-Svc.
Это нормативные условия, не доказательство их реализации клиентом.

### S2 — RFC 9849

E. Rescorla, K. Oku, N. Sullivan, C. A. Wood. *TLS Encrypted Client Hello*.
RFC9849, March2026, IETF Standards Track, Proposed Standard.
<https://www.rfc-editor.org/rfc/rfc9849.txt>.

**SECTIONS_READ:** §§4.1–5.2,6.1.4–6.1.8,6.2,8.1,10.1–10.4.
Исходное получение 08:59Z, `rfc9849.txt`.
Использованы разные идентичности O/P, аутентифицированное отклонение,
предупреждение о выборе адреса при retry, привязка сохраняемого состояния
к источнику, смена сети и tracking. Прочитанные разделы не равны всему аудиту TLS.

### S3 — RFC 9460

B. Schwartz, M. Bishop, E. Nygren. *Service Binding and Parameter Specification
via the DNS (SVCB and HTTPS Resource Records)*. RFC9460, November2023,
IETF Standards Track, Proposed Standard.
<https://www.rfc-editor.org/rfc/rfc9460.txt>.

**SECTIONS_READ:** §§2.3–3.2,5.1–5.2,7.1–8,9.1–9.4,12–13.
Исходное получение 08:59Z, `rfc9460.txt`.
Использованы целостность RR, priority/mandatory, согласованность preconnect,
различие address hints и A/AAAA, TargetName/port/ALPN, прокси и разделение кеша
по сетевому контексту. Готовой реализации selector документ не предоставляет.

## 2. Манифест и вызывающие пути на дату среза

| Вход | Версия / область | Прочитанный предмет и предел |
|---|---|---|
| `windows/go.mod`, `go.sum` | Go1.26, sing-box1.13.19, uTLS1.8.4, sing0.8.13, miekg/dns1.1.72 | SOURCE_READ выбора версий; не происхождение собранного артефакта |
| Snowden `corebridge.go` | renderAndPrepareContext:106–184 | SOURCE_READ preflight→override и владение |
| `backend/core/preflight.go` | Resolve:79–149, A/JSON DoH:151–189, cache:231–267 | SOURCE_READ first-success IP и host cache на 7 дней |
| `backend/render/render.go` | DNS:565–587, VLESS/WS:732–774, tunnelHostnames:1012–1021 | SOURCE_READ отдельного bootstrap DNS; настоящий ECH в renderer отсутствовал |
| `backend/core/engine.go` и worker | newSingBox:404–414, worker NewEngine | SOURCE_READ создания embedded/worker Box; без runtime |
| sing-box `box.go` | Менеджеры и регистрация dns.Router | SOURCE_READ конкретного DNSRouter |
| `common/tls/ech.go` | parse:27–55, ECHClientConfig/fetch/cache/Clone:109–189 | SOURCE_READ состояния config и границы обновления |
| `common/tls/client.go` | New:52–69, dial/retry:117–134 | SOURCE_READ socket до handshake и typed retry stdlib |
| STD/uTLS clients | ConfigList setters, Client/Clone, NewClient ECH | SOURCE_READ проекции состояния и клонирования |
| VLESS/WS | NewOutbound/Dial:46–99,151–170; WS dial:36–108, early conn:136–238 | SOURCE_READ порядка app data после TLS/Upgrade и времени жизни объектов |
| DNS router/client/transports | Exchange/Reset, cache/Lookup, DoH/DoT/TCP | SOURCE_READ различий Msg/address API и reset |
| Network manager | ResetNetwork:457–484, notifications | SOURCE_READ DNS/connection reset без найденного сброса TLS ECH wrapper |
| Dialer resolve/default | Domain→A/AAAA→address, socket dial | SOURCE_READ выбора endpoint отдельно от HTTPS RR |
| uTLS `u_handshake_client.go` | O, разбор E, внешнее P, handshake | SOURCE_READ переходов идентичности настоящего ECH |
| `handshake_client*.go` | Acceptance/rejection, certificate/Finished | SOURCE_READ проверки и ошибок выбранных ветвей |
| `u_tls_extensions.go` | SNI:201–208 | SOURCE_READ сохранения O при настоящем ECH |
| uTLS ECH/parser/connection | public_name, native ECHAccepted, тип ошибки | SOURCE_READ; без исполнения продукта |
| miekg/dns `svcb.go` | HTTPS/SVCB, mandatory/alpn/port/hints/ech | SOURCE_READ готовых parser structs; смысловая проверка у caller |
| Go1.26.7 crypto/tls/errors | ECH contract/type и errors.As | SOURCE_READ справочной реализации, не аттестация compiler EXE |

Начальная копия: `.artifacts/research/r28-contracts/Z01/source-initial.json`,
37 файлов около 08:57Z. Добавления — `source-extra.json` (19),
`source-composition.json` (3) и выбранные paired/API-входы.
Копирование последовательное, не атомарный снимок всего репозитория.

## 3. Сопоставление байтов

В исходном срезе 09:18Z прочитаны immutable raw-пути:

- MetaCubeX/uTLS `cf49b0864331e156689feec6363ef9bf518a5ac7`:
  `u_tls_extensions.go`, `handshake_client.go`.
- SagerNet/sing-box `b5ebaa1fc0f2b94256180b95468e73ef53caa27d`:
  `common/tls/ech.go`, `route/network.go`.

Четыре пары совпали с module cache; записи — `remote-equality.json`.
Это идентичность выбранных байтов, не всего модуля, зависимостей или артефакта.

## 4. Переиспользование и лицензии

- sing-box root LICENSE: SOURCE_READ, GPL-3.0-or-later и условие о
  названии/ассоциации. Соответствие всей поставки не установлено.
- MetaCubeX/uTLS и miekg/dns: root LICENSE, SOURCE_READ, BSD-3-Clause.
- Go standard library: сохранён BSD-style LICENSE; полный состав поставки
  за пределами этого разбора.
- RFC: уведомления IETF Trust; это спецификации, не готовый программный компонент.

Брать существующие parser/TLS/exchange primitives и адаптировать нужный контекст;
новые собственные HPKE/ECH/DNSSEC реализации не обоснованы.

## 5. Сопоставление с C02

C02/HANDOFF стал доступен при исходном review и прочитан адресно;
копия вместе с `ech_shared.go` записана в `source-review-extra.json`.

- C02-B: DNS/CloseAll/mux reset не означает очистку ECH wrapper.
- C02-C: uTLS default rejection verifier может проверять O вместо P
  в названной ветви; это нельзя скрывать общей фразой «P аутентифицирован».
- C02-A: фиксированный endpoint с общим одобренным ключом — положительный
  контроль; отсутствие универсальной SVCB-семантики не делает любой ECH небезопасным.

Z01 проследил названный source-путь отдельно. Совпадение моделей не является
независимым полевым доказательством или автоматическим научным принятием.

## 6. Статус чтения и получения

RFC9848/9849/9460 — SECTIONS_READ официального RFC Editor TXT;
четыре критичных immutable raw-файла — SOURCE_READ;
module cache — SOURCE_READ выбранных symbols, не весь репозиторий.
Внешние статьи и полевые сообщения для этого узкого вывода не использовались.
Неудачного получения, от которого зависел бы финальный аргумент, не осталось.
Частные endpoints, профили, логи, vault, credentials и DNS/TLS-пробы не использовались.

## 7. Предел доказательства

Ручные контрпримеры — модели, не воспроизведённые отказы.
Настоящий ECH не входил в renderer исходного среза; серьёзность и частота
ошибки развёрнутого продукта не установлены. Все будущие случаи REPORT §8 —
NOT_RUN: тесты, fixtures, сборка, симуляции, benchmarks, crypto/native/device/live,
административные и серверные действия, изменения сети не выполнялись.
Применение требует платформенного решения и собственной приёмки.

<a id="r28-z01--источники-и-provenance"></a>
<a id="2-actual-manifestsource-pins"></a>
<a id="3-independent-byte-equality"></a>
<a id="4-reuselicences"></a>
<a id="5-paired-independent-input"></a>
<a id="6-readfetch-status"></a>
<a id="7-evidence-boundaries-and-future-not_run"></a>
